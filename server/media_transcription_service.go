package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-web-base/shared/log"
	"agent-web-base/vo"
)

const (
	MediaTypeAudio = "audio"
	MediaTypeVideo = "video"

	MediaFileStatusUploaded    = "uploaded"
	MediaFileStatusProcessing  = "processing"
	MediaFileStatusTranscribed = "transcribed"
	MediaFileStatusFailed      = "failed"

	TranscriptionJobStatusQueued     = "queued"
	TranscriptionJobStatusProcessing = "processing"
	TranscriptionJobStatusSucceeded  = "succeeded"
	TranscriptionJobStatusFailed     = "failed"

	TranscriptSourceTypeASRAudio = "asr_audio"
	TranscriptSourceTypeASRVideo = "asr_video"

	DefaultASRProvider = "openai"
	DefaultASRModel    = "gpt-4o-transcribe"
)

var errBadRequest = errors.New("bad request")

func isBadRequestError(err error) bool {
	return errors.Is(err, errBadRequest)
}

func (s *Server) UploadInterviewMedia(interviewID string, userID string, language string, fileHeader *multipart.FileHeader) (vo.UploadInterviewMediaVO, error) {
	if strings.TrimSpace(userID) == "" {
		return vo.UploadInterviewMediaVO{}, fmt.Errorf("%w: user_id is required", errBadRequest)
	}
	if fileHeader == nil {
		return vo.UploadInterviewMediaVO{}, fmt.Errorf("%w: file is required", errBadRequest)
	}
	if language == "" {
		language = DefaultTranscriptLanguage
	}

	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return vo.UploadInterviewMediaVO{}, err
	}
	if session.UserID != userID {
		return vo.UploadInterviewMediaVO{}, fmt.Errorf("%w: interview user_id mismatch", errBadRequest)
	}
	if session.Status == InterviewStatusReviewed {
		return vo.UploadInterviewMediaVO{}, fmt.Errorf("%w: reviewed interview cannot accept a new media upload", errBadRequest)
	}

	mediaType, ext, err := detectMediaType(fileHeader)
	if err != nil {
		return vo.UploadInterviewMediaVO{}, err
	}

	mediaID := uuid.New().String()
	storedFilename := mediaID + ext
	storageDir := filepath.Join(s.mediaDir, "interviews", interviewID, "original")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return vo.UploadInterviewMediaVO{}, err
	}
	storagePath := filepath.Join(storageDir, storedFilename)
	if err := saveUploadedFile(fileHeader, storagePath); err != nil {
		return vo.UploadInterviewMediaVO{}, err
	}

	now := time.Now().Unix()
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(ext)
	}
	media := MediaFile{
		MediaID:          mediaID,
		InterviewID:      interviewID,
		UserID:           userID,
		OriginalFilename: filepath.Base(fileHeader.Filename),
		StoredFilename:   storedFilename,
		StoragePath:      storagePath,
		ContentType:      contentType,
		MediaType:        mediaType,
		FileExt:          ext,
		SizeBytes:        fileHeader.Size,
		Status:           MediaFileStatusUploaded,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	job := TranscriptionJob{
		JobID:          uuid.New().String(),
		MediaID:        mediaID,
		InterviewID:    interviewID,
		UserID:         userID,
		Status:         TranscriptionJobStatusQueued,
		InputMediaPath: storagePath,
		ASRProvider:    s.asrProvider(),
		ASRModel:       s.asrModel(),
		Language:       language,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&media).Error; err != nil {
			return err
		}
		return tx.Create(&job).Error
	}); err != nil {
		return vo.UploadInterviewMediaVO{}, err
	}

	s.enqueueTranscriptionJob(job.JobID)
	return vo.UploadInterviewMediaVO{
		MediaFile:        toMediaFileVO(media),
		TranscriptionJob: toTranscriptionJobVO(job),
	}, nil
}

func (s *Server) GetTranscriptionJob(jobID string) (vo.TranscriptionJobVO, error) {
	var job TranscriptionJob
	if err := s.db.First(&job, "job_id = ?", jobID).Error; err != nil {
		return vo.TranscriptionJobVO{}, err
	}
	return toTranscriptionJobVO(job), nil
}

func (s *Server) ListInterviewMedia(interviewID string) ([]vo.MediaFileVO, error) {
	var mediaFiles []MediaFile
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("created_at desc").
		Find(&mediaFiles).Error; err != nil {
		return nil, err
	}

	result := make([]vo.MediaFileVO, 0, len(mediaFiles))
	for _, mediaFile := range mediaFiles {
		result = append(result, toMediaFileVO(mediaFile))
	}
	return result, nil
}

func (s *Server) enqueueTranscriptionJob(jobID string) {
	if s.transcriptionQueue == nil {
		return
	}
	select {
	case s.transcriptionQueue <- jobID:
	default:
		log.Warnf("transcription queue is full; job remains queued: %s", jobID)
	}
}

func (s *Server) asrProvider() string {
	if s.asrClient == nil {
		return DefaultASRProvider
	}
	return s.asrClient.Provider()
}

func (s *Server) asrModel() string {
	if s.asrClient == nil {
		return DefaultASRModel
	}
	return s.asrClient.Model()
}

func detectMediaType(fileHeader *multipart.FileHeader) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	contentType := strings.ToLower(fileHeader.Header.Get("Content-Type"))
	audioExts := map[string]bool{
		".mp3":  true,
		".m4a":  true,
		".wav":  true,
		".webm": true,
		".mpeg": true,
		".mpga": true,
	}
	videoExts := map[string]bool{
		".mp4":  true,
		".mov":  true,
		".mkv":  true,
		".webm": true,
	}
	if strings.HasPrefix(contentType, "audio/") && audioExts[ext] {
		return MediaTypeAudio, ext, nil
	}
	if strings.HasPrefix(contentType, "video/") && videoExts[ext] {
		return MediaTypeVideo, ext, nil
	}
	if audioExts[ext] {
		return MediaTypeAudio, ext, nil
	}
	if videoExts[ext] {
		return MediaTypeVideo, ext, nil
	}
	return "", "", fmt.Errorf("%w: unsupported media file extension %q", errBadRequest, ext)
}

func saveUploadedFile(fileHeader *multipart.FileHeader, dst string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func toMediaFileVO(mediaFile MediaFile) vo.MediaFileVO {
	return vo.MediaFileVO{
		MediaID:          mediaFile.MediaID,
		InterviewID:      mediaFile.InterviewID,
		UserID:           mediaFile.UserID,
		OriginalFilename: mediaFile.OriginalFilename,
		StoredFilename:   mediaFile.StoredFilename,
		StoragePath:      mediaFile.StoragePath,
		ContentType:      mediaFile.ContentType,
		MediaType:        mediaFile.MediaType,
		FileExt:          mediaFile.FileExt,
		SizeBytes:        mediaFile.SizeBytes,
		Status:           mediaFile.Status,
		ErrorMessage:     mediaFile.ErrorMessage,
		CreatedAt:        mediaFile.CreatedAt,
		UpdatedAt:        mediaFile.UpdatedAt,
	}
}

func toTranscriptionJobVO(job TranscriptionJob) vo.TranscriptionJobVO {
	return vo.TranscriptionJobVO{
		JobID:              job.JobID,
		MediaID:            job.MediaID,
		InterviewID:        job.InterviewID,
		UserID:             job.UserID,
		Status:             job.Status,
		InputMediaPath:     job.InputMediaPath,
		ExtractedAudioPath: job.ExtractedAudioPath,
		ASRProvider:        job.ASRProvider,
		ASRModel:           job.ASRModel,
		Language:           job.Language,
		TranscriptID:       job.TranscriptID,
		ErrorMessage:       job.ErrorMessage,
		StartedAt:          job.StartedAt,
		FinishedAt:         job.FinishedAt,
		CreatedAt:          job.CreatedAt,
		UpdatedAt:          job.UpdatedAt,
	}
}
