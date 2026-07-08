package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-web-base/vo"
)

const (
	PracticeGoalStatusActive    = "active"
	PracticeGoalStatusCompleted = "completed"
	PracticeGoalStatusArchived  = "archived"
)

func (s *Server) CreatePracticeGoal(req vo.CreatePracticeGoalReq) (vo.PracticeGoalVO, error) {
	userID := strings.TrimSpace(req.UserID)
	companyName := strings.TrimSpace(req.CompanyName)
	if userID == "" {
		return vo.PracticeGoalVO{}, fmt.Errorf("user_id is required")
	}
	if companyName == "" {
		return vo.PracticeGoalVO{}, fmt.Errorf("company_name is required")
	}
	now := time.Now().Unix()
	goal := PracticeGoal{
		GoalID:         uuid.New().String(),
		UserID:         userID,
		CompanyName:    companyName,
		JobTitle:       strings.TrimSpace(req.JobTitle),
		TargetRound:    normalizeDefault(strings.TrimSpace(req.TargetRound), "second_round"),
		JobDescription: strings.TrimSpace(req.JobDescription),
		FocusTopics:    marshalStringSlice(req.FocusTopics),
		RemainingDays:  normalizedPracticeGoalRemainingDays(req.RemainingDays),
		Status:         PracticeGoalStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.db.Create(&goal).Error; err != nil {
		return vo.PracticeGoalVO{}, err
	}
	return toPracticeGoalVO(goal), nil
}

func (s *Server) ListPracticeGoals(userID string, status string) ([]vo.PracticeGoalVO, error) {
	userID = strings.TrimSpace(userID)
	status = strings.TrimSpace(status)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	query := s.db.Where("user_id = ?", userID)
	if status != "" {
		if !validPracticeGoalStatus(status) {
			return nil, fmt.Errorf("unsupported practice goal status %q", status)
		}
		query = query.Where("status = ?", status)
	}

	var goals []PracticeGoal
	if err := query.Order("updated_at desc").Find(&goals).Error; err != nil {
		return nil, err
	}

	result := make([]vo.PracticeGoalVO, 0, len(goals))
	for _, goal := range goals {
		result = append(result, toPracticeGoalVO(goal))
	}
	return result, nil
}

func (s *Server) GetPracticeGoal(goalID string) (vo.PracticeGoalVO, error) {
	goal, err := firstPracticeGoalByID(s.db, goalID)
	if err != nil {
		return vo.PracticeGoalVO{}, err
	}
	return toPracticeGoalVO(goal), nil
}

func (s *Server) UpdatePracticeGoal(goalID string, req vo.UpdatePracticeGoalReq) (vo.PracticeGoalVO, error) {
	var updated PracticeGoal
	err := s.db.Transaction(func(tx *gorm.DB) error {
		goal, err := firstPracticeGoalByID(tx, goalID)
		if err != nil {
			return err
		}

		if strings.TrimSpace(req.CompanyName) != "" {
			goal.CompanyName = strings.TrimSpace(req.CompanyName)
		}
		if strings.TrimSpace(req.JobTitle) != "" {
			goal.JobTitle = strings.TrimSpace(req.JobTitle)
		}
		if strings.TrimSpace(req.TargetRound) != "" {
			goal.TargetRound = normalizeDefault(strings.TrimSpace(req.TargetRound), goal.TargetRound)
		}
		if strings.TrimSpace(req.JobDescription) != "" {
			goal.JobDescription = strings.TrimSpace(req.JobDescription)
		}
		if req.FocusTopics != nil {
			goal.FocusTopics = marshalStringSlice(req.FocusTopics)
		}
		if req.RemainingDays > 0 {
			goal.RemainingDays = req.RemainingDays
		}
		status := strings.TrimSpace(req.Status)
		if status != "" {
			if !validPracticeGoalStatus(status) {
				return fmt.Errorf("unsupported practice goal status %q", status)
			}
			goal.Status = status
		}
		goal.UpdatedAt = time.Now().Unix()

		if err := tx.Save(&goal).Error; err != nil {
			return err
		}
		updated = goal
		return nil
	})
	if err != nil {
		return vo.PracticeGoalVO{}, err
	}
	return toPracticeGoalVO(updated), nil
}

func (s *Server) ArchivePracticeGoal(goalID string) (vo.PracticeGoalVO, error) {
	return s.UpdatePracticeGoal(goalID, vo.UpdatePracticeGoalReq{Status: PracticeGoalStatusArchived})
}

func firstPracticeGoalByID(tx *gorm.DB, goalID string) (PracticeGoal, error) {
	var goal PracticeGoal
	if err := tx.First(&goal, "goal_id = ?", goalID).Error; err != nil {
		return PracticeGoal{}, err
	}
	return goal, nil
}

func toPracticeGoalVO(goal PracticeGoal) vo.PracticeGoalVO {
	return vo.PracticeGoalVO{
		GoalID:         goal.GoalID,
		UserID:         goal.UserID,
		InterviewID:    "",
		CompanyName:    goal.CompanyName,
		JobTitle:       goal.JobTitle,
		TargetRound:    goal.TargetRound,
		JobDescription: goal.JobDescription,
		FocusTopics:    unmarshalStringSlice(goal.FocusTopics),
		RemainingDays:  goal.RemainingDays,
		Status:         goal.Status,
		CreatedAt:      goal.CreatedAt,
		UpdatedAt:      goal.UpdatedAt,
	}
}

func normalizedPracticeGoalRemainingDays(days int) int {
	if days <= 0 {
		return 1
	}
	return days
}

func validPracticeGoalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PracticeGoalStatusActive, PracticeGoalStatusCompleted, PracticeGoalStatusArchived:
		return true
	default:
		return false
	}
}
