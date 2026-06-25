package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-web-base/shared/log"
)

func (s *Server) StartTranscriptionWorker(ctx context.Context) {
	if s.transcriptionQueue == nil {
		s.transcriptionQueue = make(chan string, 100)
	}
	s.enqueuePendingTranscriptionJobs()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case jobID := <-s.transcriptionQueue:
				if err := s.ProcessTranscriptionJob(ctx, jobID); err != nil {
					log.Warnf("process transcription job failed: job_id=%s err=%v", jobID, err)
				}
			}
		}
	}()
}

func (s *Server) enqueuePendingTranscriptionJobs() {
	var jobs []TranscriptionJob
	if err := s.db.Where("status = ?", TranscriptionJobStatusQueued).
		Order("created_at asc").
		Find(&jobs).Error; err != nil {
		log.Warnf("load queued transcription jobs failed: %v", err)
		return
	}
	for _, job := range jobs {
		s.enqueueTranscriptionJob(job.JobID)
	}
}

func (s *Server) ProcessTranscriptionJob(ctx context.Context, jobID string) error {
	var job TranscriptionJob
	if err := s.db.First(&job, "job_id = ?", jobID).Error; err != nil {
		return err
	}
	if job.Status != TranscriptionJobStatusQueued {
		return nil
	}

	var media MediaFile
	if err := s.db.First(&media, "media_id = ?", job.MediaID).Error; err != nil {
		return err
	}

	now := time.Now().Unix()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&TranscriptionJob{}).
			Where("job_id = ? AND status = ?", jobID, TranscriptionJobStatusQueued).
			Updates(map[string]any{
				"status":     TranscriptionJobStatusProcessing,
				"started_at": now,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&MediaFile{}).
			Where("media_id = ?", job.MediaID).
			Updates(map[string]any{
				"status":     MediaFileStatusProcessing,
				"updated_at": now,
			}).Error
	}); err != nil {
		return err
	}
	job.Status = TranscriptionJobStatusProcessing
	job.StartedAt = now

	audioPath := job.InputMediaPath
	if media.MediaType == MediaTypeVideo {
		extractedPath, err := s.extractAudioForJob(ctx, job, media)
		if err != nil {
			_ = s.markTranscriptionJobFailed(job, media, err)
			return err
		}
		audioPath = extractedPath
		job.ExtractedAudioPath = extractedPath
	}

	if s.asrClient == nil {
		err := fmt.Errorf("asr client is not configured")
		_ = s.markTranscriptionJobFailed(job, media, err)
		return err
	}

	asrResult, err := s.asrClient.Transcribe(ctx, ASRRequest{
		AudioPath: audioPath,
		Language:  job.Language,
	})
	if err != nil {
		_ = s.markTranscriptionJobFailed(job, media, err)
		return err
	}
	if strings.TrimSpace(asrResult.Text) == "" {
		err := fmt.Errorf("asr transcript text is empty")
		_ = s.markTranscriptionJobFailed(job, media, err)
		return err
	}

	if err := s.saveSuccessfulTranscription(job, media, asrResult.Text); err != nil {
		_ = s.markTranscriptionJobFailed(job, media, err)
		return err
	}
	return nil
}

func (s *Server) extractAudioForJob(ctx context.Context, job TranscriptionJob, media MediaFile) (string, error) {
	if s.audioExtractor == nil {
		return "", fmt.Errorf("audio extractor is not configured")
	}
	derivedDir := filepath.Join(s.mediaDir, "interviews", job.InterviewID, "derived")
	if err := os.MkdirAll(derivedDir, 0755); err != nil {
		return "", err
	}
	extractedPath := filepath.Join(derivedDir, job.MediaID+".mp3")
	if err := s.audioExtractor.ExtractAudio(ctx, media.StoragePath, extractedPath); err != nil {
		return "", err
	}

	now := time.Now().Unix()
	if err := s.db.Model(&TranscriptionJob{}).
		Where("job_id = ?", job.JobID).
		Updates(map[string]any{
			"extracted_audio_path": extractedPath,
			"updated_at":           now,
		}).Error; err != nil {
		return "", err
	}
	return extractedPath, nil
}

func (s *Server) saveSuccessfulTranscription(job TranscriptionJob, media MediaFile, text string) error {
	now := time.Now().Unix()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var session InterviewSession
		if err := tx.First(&session, "interview_id = ?", job.InterviewID).Error; err != nil {
			return err
		}
		if session.UserID != job.UserID {
			return fmt.Errorf("interview user_id mismatch")
		}
		if session.Status == InterviewStatusReviewed {
			return fmt.Errorf("reviewed interview cannot accept asr transcript")
		}

		transcript, err := upsertASRTranscript(tx, job, media, text, now)
		if err != nil {
			return err
		}

		if err := tx.Model(&InterviewSession{}).
			Where("interview_id = ?", job.InterviewID).
			Updates(map[string]any{
				"status":     InterviewStatusReadyForReview,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&MediaFile{}).
			Where("media_id = ?", media.MediaID).
			Updates(map[string]any{
				"status":        MediaFileStatusTranscribed,
				"error_message": "",
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&TranscriptionJob{}).
			Where("job_id = ?", job.JobID).
			Updates(map[string]any{
				"status":        TranscriptionJobStatusSucceeded,
				"transcript_id": transcript.TranscriptID,
				"error_message": "",
				"finished_at":   now,
				"updated_at":    now,
			}).Error
	})
}

func upsertASRTranscript(tx *gorm.DB, job TranscriptionJob, media MediaFile, text string, now int64) (InterviewTranscript, error) {
	sourceType := TranscriptSourceTypeASRAudio
	if media.MediaType == MediaTypeVideo {
		sourceType = TranscriptSourceTypeASRVideo
	}

	var transcript InterviewTranscript
	err := tx.First(&transcript, "interview_id = ?", job.InterviewID).Error
	switch {
	case err == nil:
		transcript.UserID = job.UserID
		transcript.SourceType = sourceType
		transcript.Content = text
		transcript.Language = normalizeDefault(job.Language, DefaultTranscriptLanguage)
		transcript.UpdatedAt = now
		if err := tx.Save(&transcript).Error; err != nil {
			return InterviewTranscript{}, err
		}
		return transcript, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		transcript = InterviewTranscript{
			TranscriptID: uuid.New().String(),
			InterviewID:  job.InterviewID,
			UserID:       job.UserID,
			SourceType:   sourceType,
			Content:      text,
			Language:     normalizeDefault(job.Language, DefaultTranscriptLanguage),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&transcript).Error; err != nil {
			return InterviewTranscript{}, err
		}
		return transcript, nil
	default:
		return InterviewTranscript{}, err
	}
}

func (s *Server) markTranscriptionJobFailed(job TranscriptionJob, media MediaFile, cause error) error {
	now := time.Now().Unix()
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&MediaFile{}).
			Where("media_id = ?", media.MediaID).
			Updates(map[string]any{
				"status":        MediaFileStatusFailed,
				"error_message": message,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&TranscriptionJob{}).
			Where("job_id = ?", job.JobID).
			Updates(map[string]any{
				"status":        TranscriptionJobStatusFailed,
				"error_message": message,
				"finished_at":   now,
				"updated_at":    now,
			}).Error
	})
}
