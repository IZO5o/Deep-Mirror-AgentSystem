package server

import (
	"net/http"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestAgentEvaluationCoachingPlanGeneratePasses(t *testing.T) {
	s := newTestServer(t)
	traceID := mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:                  "user_001",
		InterviewID:             "interview_001",
		AgentType:               string(agent.AgentTypeSecondRoundCoach),
		SourceType:              AgentTraceSourceCoachingPlan,
		SourceID:                "plan_001",
		StepName:                AgentTraceStepCoachingPlanGenerate,
		SelectedContextSnapshot: validSelectedContextSnapshot(),
		InputSnapshot:           `{"interview_id":"interview_001"}`,
		RawAgentOutput:          `{"plan":"raw"}`,
		ParsedDecision:          `{"tasks":[{"title":"Redis"}]}`,
		ServiceActions:          `["created coaching_plan","created coaching_tasks: 1"]`,
		Status:                  AgentDecisionTraceStatusSucceeded,
	})

	result := mustEvaluateSingleTrace(t, s, AgentDecisionTraceQuery{SourceID: "plan_001"})
	if result.TraceID != traceID || !result.Passed || result.Score != 100 {
		t.Fatalf("evaluation result = %#v, want pass score 100", result)
	}
}

func TestAgentEvaluationMockStartAndTurnSelectedContextPasses(t *testing.T) {
	s := newTestServer(t)
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:                  "user_001",
		InterviewID:             "interview_001",
		AgentType:               string(agent.AgentTypeMockInterviewer),
		SourceType:              AgentTraceSourceMockInterview,
		SourceID:                "mock_001",
		StepName:                AgentTraceStepMockStart,
		SelectedContextSnapshot: validSelectedContextSnapshot(),
		InputSnapshot:           `{"mock_id":"mock_001"}`,
		RawAgentOutput:          `{"opening_question":"讲项目"}`,
		ParsedDecision:          `{"opening_question":"讲项目"}`,
		ServiceActions:          `["created mock_interview","created opening mock_turn"]`,
		Status:                  AgentDecisionTraceStatusSucceeded,
	})
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:                  "user_001",
		InterviewID:             "interview_001",
		AgentType:               string(agent.AgentTypeMockInterviewer),
		SourceType:              AgentTraceSourceMockInterview,
		SourceID:                "mock_001",
		StepName:                AgentTraceStepMockTurn,
		SelectedContextSnapshot: validSelectedContextSnapshot(),
		InputSnapshot:           `{"mock_id":"mock_001","current_turn":0}`,
		RawAgentOutput:          `{"input_type":"formal_answer"}`,
		ParsedDecision:          `{"input_type":"formal_answer","should_update_practice_state":true}`,
		ServiceActions:          `["created mock_turns: 3","updated mock status","updated practice_states"]`,
		Status:                  AgentDecisionTraceStatusSucceeded,
	})

	report, err := s.EvaluateAgentDecisionTraces(AgentDecisionTraceQuery{SourceID: "mock_001", Limit: 10})
	if err != nil {
		t.Fatalf("EvaluateAgentDecisionTraces() error = %v", err)
	}
	if report.TotalTraces != 2 || report.PassedTraces != 2 || report.FailedTraces != 0 {
		t.Fatalf("report = %#v, want two passing traces", report)
	}
}

func TestAgentEvaluationCoachingSessionTurnAllowsEmptySelectedContext(t *testing.T) {
	s := newTestServer(t)
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:         "user_001",
		InterviewID:    "interview_001",
		AgentType:      string(agent.AgentTypeSecondRoundCoach),
		SourceType:     AgentTraceSourceCoachingSession,
		SourceID:       "session_001",
		StepName:       AgentTraceStepCoachingSessionTurn,
		InputSnapshot:  `{"session_id":"session_001"}`,
		RawAgentOutput: `{"input_type":"hint_request"}`,
		ParsedDecision: `{"input_type":"hint_request"}`,
		ServiceActions: `["recorded coaching_session user turn","recorded coaching_session assistant turn","updated coaching_session state"]`,
		Status:         AgentDecisionTraceStatusSucceeded,
	})

	result := mustEvaluateSingleTrace(t, s, AgentDecisionTraceQuery{SourceID: "session_001"})
	if !result.Passed {
		t.Fatalf("result = %#v, want pass because coaching_session_turn may omit selected context", result)
	}
}

func TestAgentEvaluationDetectsFailedTraceMissingErrorMessage(t *testing.T) {
	s := newTestServer(t)
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:         "user_001",
		InterviewID:    "interview_001",
		AgentType:      string(agent.AgentTypeSecondRoundCoach),
		SourceType:     AgentTraceSourceCoachingSession,
		SourceID:       "failed_missing_error",
		StepName:       AgentTraceStepCoachingSessionTurn,
		InputSnapshot:  `{"session_id":"failed_missing_error"}`,
		RawAgentOutput: "not json",
		ServiceActions: `["recorded failed coaching_session_turn","updated coaching_session failed"]`,
		Status:         AgentDecisionTraceStatusFailed,
	})

	result := mustEvaluateSingleTrace(t, s, AgentDecisionTraceQuery{SourceID: "failed_missing_error"})
	assertEvaluationCheck(t, result, "failed_error_message", false)
}

func TestAgentEvaluationDetectsInvalidParsedDecisionJSON(t *testing.T) {
	s := newTestServer(t)
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:                  "user_001",
		InterviewID:             "interview_001",
		AgentType:               string(agent.AgentTypeMockInterviewer),
		SourceType:              AgentTraceSourceMockInterview,
		SourceID:                "invalid_parsed",
		StepName:                AgentTraceStepMockTurn,
		SelectedContextSnapshot: validSelectedContextSnapshot(),
		InputSnapshot:           `{"mock_id":"invalid_parsed"}`,
		RawAgentOutput:          `{"input_type":"formal_answer"}`,
		ParsedDecision:          `{"input_type":"formal_answer"`,
		ServiceActions:          `["created mock_turns: 3","updated mock status"]`,
		Status:                  AgentDecisionTraceStatusSucceeded,
	})

	result := mustEvaluateSingleTrace(t, s, AgentDecisionTraceQuery{SourceID: "invalid_parsed"})
	assertEvaluationCheck(t, result, "parsed_decision_json", false)
}

func TestAgentEvaluationDetectsMissingServiceActionKeyword(t *testing.T) {
	s := newTestServer(t)
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:         "user_001",
		InterviewID:    "interview_001",
		AgentType:      string(agent.AgentTypeSecondRoundCoach),
		SourceType:     AgentTraceSourceCoachingSession,
		SourceID:       "missing_action",
		StepName:       AgentTraceStepCoachingSessionTurn,
		InputSnapshot:  `{"session_id":"missing_action"}`,
		RawAgentOutput: `{"input_type":"formal_answer"}`,
		ParsedDecision: `{"input_type":"formal_answer"}`,
		ServiceActions: `["recorded coaching_session user turn","updated coaching_session state"]`,
		Status:         AgentDecisionTraceStatusSucceeded,
	})

	result := mustEvaluateSingleTrace(t, s, AgentDecisionTraceQuery{SourceID: "missing_action"})
	assertEvaluationCheck(t, result, "service_actions_coaching_formal_answer", false)
}

func TestAgentEvaluationDetectsMemoryItemsBoundaryViolation(t *testing.T) {
	s := newTestServer(t)
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:         "user_001",
		InterviewID:    "interview_001",
		AgentType:      string(agent.AgentTypeMemoryCurator),
		SourceType:     AgentTraceSourceMemoryCandidateGeneration,
		SourceID:       "memory_boundary",
		StepName:       AgentTraceStepCoachingSessionMemoryCandidates,
		InputSnapshot:  `{"session_id":"memory_boundary"}`,
		RawAgentOutput: `{"candidates":[]}`,
		ParsedDecision: `{"candidates":[]}`,
		ServiceActions: `["generated memory_candidates: 1","generated memory_events: 1","created memory_items: 1"]`,
		Status:         AgentDecisionTraceStatusSucceeded,
	})

	result := mustEvaluateSingleTrace(t, s, AgentDecisionTraceQuery{SourceID: "memory_boundary"})
	assertEvaluationCheck(t, result, "memory_items_write_boundary", false)
}

func TestAgentEvaluationControllerFilters(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:                  "user_001",
		InterviewID:             "interview_001",
		AgentType:               string(agent.AgentTypeMockInterviewer),
		SourceType:              AgentTraceSourceMockInterview,
		SourceID:                "mock_filter",
		StepName:                AgentTraceStepMockStart,
		SelectedContextSnapshot: validSelectedContextSnapshot(),
		InputSnapshot:           `{"mock_id":"mock_filter"}`,
		RawAgentOutput:          `{"opening_question":"讲项目"}`,
		ParsedDecision:          `{"opening_question":"讲项目"}`,
		ServiceActions:          `["created mock_interview","created opening mock_turn"]`,
		Status:                  AgentDecisionTraceStatusSucceeded,
	})
	mustSaveEvaluationTrace(t, s, AgentDecisionTraceInput{
		UserID:         "user_001",
		InterviewID:    "interview_001",
		AgentType:      string(agent.AgentTypeSecondRoundCoach),
		SourceType:     AgentTraceSourceCoachingSession,
		SourceID:       "session_filter",
		StepName:       AgentTraceStepCoachingSessionTurn,
		InputSnapshot:  `{"session_id":"session_filter"}`,
		RawAgentOutput: `{"input_type":"hint_request"}`,
		ParsedDecision: `{"input_type":"hint_request"}`,
		ServiceActions: `["recorded coaching_session user turn","recorded coaching_session assistant turn","updated coaching_session state"]`,
		Status:         AgentDecisionTraceStatusSucceeded,
	})

	rec := performJSONRequest(router, http.MethodGet, "/api/agent-evaluations?source_type="+AgentTraceSourceMockInterview+"&source_id=mock_filter&step_name="+AgentTraceStepMockStart, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var report vo.AgentEvaluationReportVO
	decodeOKData(t, rec, &report)
	if report.TotalTraces != 1 || report.Results[0].SourceID != "mock_filter" || report.Results[0].StepName != AgentTraceStepMockStart {
		t.Fatalf("report = %#v, want filtered mock_start report", report)
	}
}

func mustSaveEvaluationTrace(t *testing.T, s *Server, input AgentDecisionTraceInput) string {
	t.Helper()
	if err := s.saveAgentDecisionTrace(input); err != nil {
		t.Fatalf("saveAgentDecisionTrace() error = %v", err)
	}
	traces, err := s.ListAgentDecisionTraces(AgentDecisionTraceQuery{SourceID: input.SourceID, StepName: input.StepName, Limit: 1})
	if err != nil {
		t.Fatalf("ListAgentDecisionTraces() error = %v", err)
	}
	if len(traces) != 1 {
		t.Fatalf("trace count = %d, want 1", len(traces))
	}
	return traces[0].TraceID
}

func mustEvaluateSingleTrace(t *testing.T, s *Server, query AgentDecisionTraceQuery) vo.AgentEvaluationResultVO {
	t.Helper()
	report, err := s.EvaluateAgentDecisionTraces(query)
	if err != nil {
		t.Fatalf("EvaluateAgentDecisionTraces() error = %v", err)
	}
	if report.TotalTraces != 1 || len(report.Results) != 1 {
		t.Fatalf("report = %#v, want single result", report)
	}
	return report.Results[0]
}

func assertEvaluationCheck(t *testing.T, result vo.AgentEvaluationResultVO, name string, passed bool) {
	t.Helper()
	for _, check := range result.Checks {
		if check.Name == name {
			if check.Passed != passed {
				t.Fatalf("check %s passed = %v, want %v; result=%#v", name, check.Passed, passed, result)
			}
			if !passed && result.Passed {
				t.Fatalf("result unexpectedly passed despite failed check %s: %#v", name, result)
			}
			return
		}
	}
	names := make([]string, 0, len(result.Checks))
	for _, check := range result.Checks {
		names = append(names, check.Name)
	}
	t.Fatalf("check %s not found; available=%s", name, strings.Join(names, ","))
}

func validSelectedContextSnapshot() string {
	return `{"debug_summary":"selected context ok","selected_memory_items":[],"selected_practice_states":[]}`
}
