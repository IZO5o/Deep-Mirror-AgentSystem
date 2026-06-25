package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"agent-web-base/vo"
)

const (
	PracticeStateSourceMockTurn            = "mock_turn"
	PracticeStateSourceCoachingTaskAttempt = "coaching_task_attempt"

	PracticeDimensionBackendKnowledge = "backend_knowledge"
	PracticeDimensionAgentProject     = "agent_project"
	PracticeDimensionCommunication    = "communication"
	PracticeDimensionSystemDesign     = "system_design"
	PracticeDimensionGeneral          = "general"
)

func (s *Server) updatePracticeStatesFromMockTurnTx(tx *gorm.DB, turn MockTurn) error {
	return s.runPracticeStateUpdateToolTx(tx, practiceStateUpdateToolInput{
		UserID:     turn.UserID,
		Topics:     unmarshalStringSlice(turn.TopicTags),
		Score:      turn.Score,
		Feedback:   turn.Feedback,
		SourceType: PracticeStateSourceMockTurn,
		SourceID:   turn.TurnID,
	})
}

func (s *Server) ListPracticeStates(userID string, topic string, dimension string) ([]vo.PracticeStateVO, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	query := s.db.Where("user_id = ?", userID)
	if topic != "" {
		query = query.Where("topic = ?", topic)
	}
	if dimension != "" {
		query = query.Where("dimension = ?", dimension)
	}

	var states []PracticeState
	if err := query.Order("updated_at desc").Find(&states).Error; err != nil {
		return nil, err
	}

	result := make([]vo.PracticeStateVO, 0, len(states))
	for _, state := range states {
		result = append(result, toPracticeStateVO(state))
	}
	return result, nil
}

func (s *Server) GetPracticeState(stateID string) (vo.PracticeStateVO, error) {
	var state PracticeState
	if err := s.db.First(&state, "state_id = ?", stateID).Error; err != nil {
		return vo.PracticeStateVO{}, err
	}
	return toPracticeStateVO(state), nil
}

func (s *Server) loadPracticeStatesForPrompt(userID string, limit int) ([]PracticeState, error) {
	if limit <= 0 {
		limit = 20
	}
	var states []PracticeState
	if err := s.db.Where("user_id = ?", userID).
		Order("updated_at desc").
		Limit(limit).
		Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func smoothMasteryScore(oldScore int, newScore int) int {
	return int(float64(clampScore(oldScore))*0.7 + float64(clampScore(newScore))*0.3 + 0.5)
}

func inferPracticeDimension(topic string) string {
	topicLower := strings.ToLower(topic)
	switch {
	case containsAny(topicLower, []string{"redis", "mysql", "缓存", "数据库", "并发", "索引"}):
		return PracticeDimensionBackendKnowledge
	case containsAny(topicLower, []string{"agent", "工具调用", "rag", "记忆", "prompt"}):
		return PracticeDimensionAgentProject
	case containsAny(topicLower, []string{"项目表达", "表达", "沟通", "叙述"}):
		return PracticeDimensionCommunication
	case containsAny(topicLower, []string{"系统设计", "架构", "高并发", "扩展性"}):
		return PracticeDimensionSystemDesign
	default:
		return PracticeDimensionGeneral
	}
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func practiceStatesJSON(states []PracticeState) string {
	payload := make([]map[string]any, 0, len(states))
	for _, state := range states {
		payload = append(payload, map[string]any{
			"state_id":          state.StateID,
			"topic":             state.Topic,
			"dimension":         state.Dimension,
			"mastery_score":     state.MasteryScore,
			"attempt_count":     state.AttemptCount,
			"last_score":        state.LastScore,
			"last_feedback":     state.LastFeedback,
			"last_practiced_at": state.LastPracticedAt,
			"source_type":       state.SourceType,
			"source_id":         state.SourceID,
		})
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func toPracticeStateVO(state PracticeState) vo.PracticeStateVO {
	return vo.PracticeStateVO{
		StateID:         state.StateID,
		UserID:          state.UserID,
		Topic:           state.Topic,
		Dimension:       state.Dimension,
		MasteryScore:    state.MasteryScore,
		AttemptCount:    state.AttemptCount,
		LastScore:       state.LastScore,
		LastFeedback:    state.LastFeedback,
		LastPracticedAt: state.LastPracticedAt,
		SourceType:      state.SourceType,
		SourceID:        state.SourceID,
		CreatedAt:       state.CreatedAt,
		UpdatedAt:       state.UpdatedAt,
	}
}
