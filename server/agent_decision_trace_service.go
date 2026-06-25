package server

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"agent-web-base/shared/log"
	"agent-web-base/vo"
)

const (
	AgentDecisionTraceStatusSucceeded = "succeeded"
	AgentDecisionTraceStatusFailed    = "failed"

	AgentTraceSourceInterviewReview           = "interview_review"
	AgentTraceSourceCoachingPlan              = "coaching_plan"
	AgentTraceSourceCoachingSession           = "coaching_session"
	AgentTraceSourceMockInterview             = "mock_interview"
	AgentTraceSourceMemoryCandidateGeneration = "memory_candidate_generation"

	AgentTraceStepCoachingPlanGenerate            = "coaching_plan_generate"
	AgentTraceStepCoachingSessionTurn             = "coaching_session_turn"
	AgentTraceStepMockStart                       = "mock_start"
	AgentTraceStepMockTurn                        = "mock_turn"
	AgentTraceStepCoachingSessionMemoryCandidates = "coaching_session_memory_candidates"
	AgentTraceStepMockInterviewMemoryCandidates   = "mock_interview_memory_candidates"
	agentDecisionTraceDefaultLimit                = 50
	agentDecisionTraceMaxLimit                    = 200
	agentDecisionTraceMaxStringLength             = 20000
)

type AgentDecisionTraceInput struct {
	UserID                  string
	InterviewID             string
	AgentType               string
	SourceType              string
	SourceID                string
	StepName                string
	SelectedContextSnapshot string
	InputSnapshot           string
	RawAgentOutput          string
	ParsedDecision          string
	ServiceActions          string
	Status                  string
	ErrorMessage            string
}

type AgentDecisionTraceQuery struct {
	UserID      string
	InterviewID string
	SourceType  string
	SourceID    string
	AgentType   string
	StepName    string
	Status      string
	Limit       int
}

func (s *Server) recordAgentDecisionTrace(input AgentDecisionTraceInput) {
	if err := s.saveAgentDecisionTrace(input); err != nil {
		log.Warnf("save agent decision trace failed: %v", err)
	}
}

func (s *Server) saveAgentDecisionTrace(input AgentDecisionTraceInput) error {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = AgentDecisionTraceStatusSucceeded
	}
	now := time.Now().Unix()
	trace := AgentDecisionTrace{
		TraceID:                 uuid.New().String(),
		UserID:                  input.UserID,
		InterviewID:             input.InterviewID,
		AgentType:               input.AgentType,
		SourceType:              input.SourceType,
		SourceID:                input.SourceID,
		StepName:                input.StepName,
		SelectedContextSnapshot: trimTraceString(input.SelectedContextSnapshot),
		InputSnapshot:           trimTraceString(input.InputSnapshot),
		RawAgentOutput:          trimTraceString(input.RawAgentOutput),
		ParsedDecision:          trimTraceString(input.ParsedDecision),
		ServiceActions:          trimTraceString(input.ServiceActions),
		Status:                  status,
		ErrorMessage:            trimTraceString(input.ErrorMessage),
		CreatedAt:               now,
	}
	return s.db.Create(&trace).Error
}

func (s *Server) ListAgentDecisionTraces(query AgentDecisionTraceQuery) ([]vo.AgentDecisionTraceVO, error) {
	limit := normalizeAgentDecisionTraceLimit(query.Limit)
	dbq := s.db.Order("created_at desc").Limit(limit)
	if strings.TrimSpace(query.UserID) != "" {
		dbq = dbq.Where("user_id = ?", strings.TrimSpace(query.UserID))
	}
	if strings.TrimSpace(query.InterviewID) != "" {
		dbq = dbq.Where("interview_id = ?", strings.TrimSpace(query.InterviewID))
	}
	if strings.TrimSpace(query.SourceType) != "" {
		dbq = dbq.Where("source_type = ?", strings.TrimSpace(query.SourceType))
	}
	if strings.TrimSpace(query.SourceID) != "" {
		dbq = dbq.Where("source_id = ?", strings.TrimSpace(query.SourceID))
	}
	if strings.TrimSpace(query.AgentType) != "" {
		dbq = dbq.Where("agent_type = ?", strings.TrimSpace(query.AgentType))
	}
	if strings.TrimSpace(query.StepName) != "" {
		dbq = dbq.Where("step_name = ?", strings.TrimSpace(query.StepName))
	}
	if strings.TrimSpace(query.Status) != "" {
		dbq = dbq.Where("status = ?", strings.TrimSpace(query.Status))
	}

	var traces []AgentDecisionTrace
	if err := dbq.Find(&traces).Error; err != nil {
		return nil, err
	}
	return toAgentDecisionTraceVOs(traces), nil
}

func normalizeAgentDecisionTraceLimit(limit int) int {
	if limit <= 0 {
		return agentDecisionTraceDefaultLimit
	}
	if limit > agentDecisionTraceMaxLimit {
		return agentDecisionTraceMaxLimit
	}
	return limit
}

func marshalTraceJSON(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return trimTraceString(string(data))
}

func buildSelectedContextTraceSnapshot(selection MemorySelectionResult) string {
	return marshalTraceJSON(map[string]any{
		"debug_summary":            selection.DebugSummary,
		"selected_memory_items":    toSelectedMemoryItemVOs(selection.MemoryItems),
		"selected_practice_states": toSelectedPracticeStateVOs(selection.PracticeStates),
	})
}

func traceErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func trimTraceString(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= agentDecisionTraceMaxStringLength {
		return value
	}
	return value[:agentDecisionTraceMaxStringLength]
}

func parseTraceLimit(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return limit
}

func toAgentDecisionTraceVOs(traces []AgentDecisionTrace) []vo.AgentDecisionTraceVO {
	result := make([]vo.AgentDecisionTraceVO, 0, len(traces))
	for _, trace := range traces {
		result = append(result, toAgentDecisionTraceVO(trace))
	}
	return result
}

func toAgentDecisionTraceVO(trace AgentDecisionTrace) vo.AgentDecisionTraceVO {
	return vo.AgentDecisionTraceVO{
		TraceID:                 trace.TraceID,
		UserID:                  trace.UserID,
		InterviewID:             trace.InterviewID,
		AgentType:               trace.AgentType,
		SourceType:              trace.SourceType,
		SourceID:                trace.SourceID,
		StepName:                trace.StepName,
		SelectedContextSnapshot: trace.SelectedContextSnapshot,
		InputSnapshot:           trace.InputSnapshot,
		RawAgentOutput:          trace.RawAgentOutput,
		ParsedDecision:          trace.ParsedDecision,
		ServiceActions:          trace.ServiceActions,
		Status:                  trace.Status,
		ErrorMessage:            trace.ErrorMessage,
		CreatedAt:               trace.CreatedAt,
	}
}
