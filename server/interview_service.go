package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-web-base/vo"
)

const (
	InterviewStatusCreated        = "created"
	InterviewStatusReadyForReview = "ready_for_review"

	TranscriptSourceTypeManualText = "manual_text"
	DefaultTranscriptLanguage      = "zh"
)

func (s *Server) CreateInterview(req vo.CreateInterviewReq) (vo.InterviewSessionVO, error) {
	now := time.Now().Unix()
	session := InterviewSession{
		InterviewID:    uuid.New().String(),
		UserID:         req.UserID,
		CompanyName:    req.CompanyName,
		JobTitle:       req.JobTitle,
		InterviewRound: req.InterviewRound,
		InterviewType:  req.InterviewType,
		Status:         InterviewStatusCreated,
		OccurredAt:     req.OccurredAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.db.Create(&session).Error; err != nil {
		return vo.InterviewSessionVO{}, err
	}
	return toInterviewSessionVO(session), nil
}

func (s *Server) GetInterview(interviewID string) (vo.InterviewSessionVO, error) {
	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return vo.InterviewSessionVO{}, err
	}
	return toInterviewSessionVO(session), nil
}

func (s *Server) ListInterviews(userID string) ([]vo.InterviewSessionVO, error) {
	var sessions []InterviewSession
	query := s.db.Order("created_at desc")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}

	result := make([]vo.InterviewSessionVO, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, toInterviewSessionVO(session))
	}
	return result, nil
}

func (s *Server) UpsertInterviewTranscript(interviewID string, req vo.UpsertInterviewTranscriptReq) (vo.InterviewTranscriptVO, error) {
	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = TranscriptSourceTypeManualText
	}
	if sourceType != TranscriptSourceTypeManualText {
		return vo.InterviewTranscriptVO{}, fmt.Errorf("unsupported transcript source_type %q", sourceType)
	}

	language := req.Language
	if language == "" {
		language = DefaultTranscriptLanguage
	}

	var result InterviewTranscript
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var session InterviewSession
		if err := tx.First(&session, "interview_id = ?", interviewID).Error; err != nil {
			return err
		}
		if session.UserID != req.UserID {
			return fmt.Errorf("interview user_id mismatch")
		}

		now := time.Now().Unix()
		var transcript InterviewTranscript
		err := tx.First(&transcript, "interview_id = ?", interviewID).Error
		switch {
		case err == nil:
			transcript.UserID = req.UserID
			transcript.SourceType = sourceType
			transcript.Content = req.Content
			transcript.Language = language
			transcript.UpdatedAt = now
			if err := tx.Save(&transcript).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			transcript = InterviewTranscript{
				TranscriptID: uuid.New().String(),
				InterviewID:  interviewID,
				UserID:       req.UserID,
				SourceType:   sourceType,
				Content:      req.Content,
				Language:     language,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := tx.Create(&transcript).Error; err != nil {
				return err
			}
		default:
			return err
		}

		if err := tx.Model(&InterviewSession{}).
			Where("interview_id = ?", interviewID).
			Updates(map[string]any{
				"status":     InterviewStatusReadyForReview,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		result = transcript
		return nil
	})
	if err != nil {
		return vo.InterviewTranscriptVO{}, err
	}
	return toInterviewTranscriptVO(result), nil
}

func (s *Server) GetInterviewTranscript(interviewID string) (vo.InterviewTranscriptVO, error) {
	var transcript InterviewTranscript
	if err := s.db.First(&transcript, "interview_id = ?", interviewID).Error; err != nil {
		return vo.InterviewTranscriptVO{}, err
	}
	return toInterviewTranscriptVO(transcript), nil
}

func toInterviewSessionVO(session InterviewSession) vo.InterviewSessionVO {
	return vo.InterviewSessionVO{
		InterviewID:    session.InterviewID,
		UserID:         session.UserID,
		CompanyName:    session.CompanyName,
		JobTitle:       session.JobTitle,
		InterviewRound: session.InterviewRound,
		InterviewType:  session.InterviewType,
		Status:         session.Status,
		OccurredAt:     session.OccurredAt,
		CreatedAt:      session.CreatedAt,
		UpdatedAt:      session.UpdatedAt,
	}
}

func toInterviewTranscriptVO(transcript InterviewTranscript) vo.InterviewTranscriptVO {
	return vo.InterviewTranscriptVO{
		TranscriptID: transcript.TranscriptID,
		InterviewID:  transcript.InterviewID,
		UserID:       transcript.UserID,
		SourceType:   transcript.SourceType,
		Content:      transcript.Content,
		Language:     transcript.Language,
		CreatedAt:    transcript.CreatedAt,
		UpdatedAt:    transcript.UpdatedAt,
	}
}
