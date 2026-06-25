package server

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type practiceStateUpdateToolInput struct {
	UserID     string
	Topics     []string
	Score      int
	Feedback   string
	SourceType string
	SourceID   string
}

// runPracticeStateUpdateToolTx is an internal business tool controlled by services.
// It is not exposed to the LLM; callers decide when it is allowed to run.
func (s *Server) runPracticeStateUpdateToolTx(tx *gorm.DB, input practiceStateUpdateToolInput) error {
	userID := strings.TrimSpace(input.UserID)
	topics := uniqueNonEmptyStrings(input.Topics)
	if userID == "" || len(topics) == 0 {
		return nil
	}

	now := time.Now().Unix()
	score := clampScore(input.Score)
	for _, topic := range topics {
		var state PracticeState
		err := tx.First(&state, "user_id = ? AND topic = ?", userID, topic).Error
		switch {
		case err == nil:
			state.Dimension = inferPracticeDimension(topic)
			state.MasteryScore = smoothMasteryScore(state.MasteryScore, score)
			state.AttemptCount++
			state.LastScore = score
			state.LastFeedback = input.Feedback
			state.LastPracticedAt = now
			state.SourceType = input.SourceType
			state.SourceID = input.SourceID
			state.UpdatedAt = now
			if err := tx.Save(&state).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			state = PracticeState{
				StateID:         uuid.New().String(),
				UserID:          userID,
				Topic:           topic,
				Dimension:       inferPracticeDimension(topic),
				MasteryScore:    score,
				AttemptCount:    1,
				LastScore:       score,
				LastFeedback:    input.Feedback,
				LastPracticedAt: now,
				SourceType:      input.SourceType,
				SourceID:        input.SourceID,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := tx.Create(&state).Error; err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}
