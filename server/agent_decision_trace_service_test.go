package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestAgentDecisionTraceCoachingPlanSuccess(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, memoryID := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = strings.ReplaceAll(
		sampleCoachingPlanJSON("trace strategy", "trace focus"),
		"MEMORY_ID_PLACEHOLDER",
		memoryID,
	)

	plan, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}

	traces := mustListAgentDecisionTraces(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingPlan,
		SourceID:   plan.PlanID,
		StepName:   AgentTraceStepCoachingPlanGenerate,
	})
	if len(traces) != 1 {
		t.Fatalf("trace count = %d, want 1", len(traces))
	}
	trace := traces[0]
	if trace.Status != AgentDecisionTraceStatusSucceeded || trace.AgentType != string(agent.AgentTypeSecondRoundCoach) {
		t.Fatalf("trace status/agent = %s/%s", trace.Status, trace.AgentType)
	}
	if !strings.Contains(trace.SelectedContextSnapshot, memoryID) || !strings.Contains(trace.SelectedContextSnapshot, "selection_reason") {
		t.Fatalf("selected context snapshot missing selected memory details: %s", trace.SelectedContextSnapshot)
	}
	if !strings.Contains(trace.RawAgentOutput, "trace strategy") || !strings.Contains(trace.ParsedDecision, "trace focus") {
		t.Fatalf("trace missing raw/parsed output: %#v", trace)
	}
	if !strings.Contains(trace.ServiceActions, "created coaching_plan") || !strings.Contains(trace.ServiceActions, "created coaching_tasks") {
		t.Fatalf("service_actions = %s", trace.ServiceActions)
	}
}

func TestAgentDecisionTraceCoachingSessionTurnSuccess(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 88, "trace feedback", CoachingNextActionPromptNext, false)

	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "trace answer", SubmitMode: CoachingSubmitModeFormalAnswer}); err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}

	traces := mustListAgentDecisionTraces(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
	})
	if len(traces) != 1 {
		t.Fatalf("trace count = %d, want 1", len(traces))
	}
	trace := traces[0]
	if trace.Status != AgentDecisionTraceStatusSucceeded || trace.SourceID != session.Session.SessionID {
		t.Fatalf("trace status/source = %s/%s", trace.Status, trace.SourceID)
	}
	for _, want := range []string{"recorded coaching_task_attempt", "updated practice_states", "updated coaching_session state"} {
		if !strings.Contains(trace.ServiceActions, want) {
			t.Fatalf("service_actions missing %q: %s", want, trace.ServiceActions)
		}
	}
	if !strings.Contains(trace.InputSnapshot, "recent_turn_count") || !strings.Contains(trace.ParsedDecision, "formal_answer") {
		t.Fatalf("trace missing input/parsed details: %#v", trace)
	}
}

func TestAgentDecisionTraceMockStartAndTurnSelectedContext(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("trace next question", 75),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "trace mock answer"}); err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}

	startTraces := mustListAgentDecisionTraces(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockStart,
	})
	if len(startTraces) != 1 {
		t.Fatalf("mock start trace count = %d, want 1", len(startTraces))
	}
	if !strings.Contains(startTraces[0].SelectedContextSnapshot, "selected_memory_items") || !strings.Contains(startTraces[0].ServiceActions, "created opening mock_turn") {
		t.Fatalf("mock start trace missing selected context/actions: %#v", startTraces[0])
	}

	turnTraces := mustListAgentDecisionTraces(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
	})
	if len(turnTraces) != 1 {
		t.Fatalf("mock turn trace count = %d, want 1", len(turnTraces))
	}
	if !strings.Contains(turnTraces[0].SelectedContextSnapshot, "selected_practice_states") || !strings.Contains(turnTraces[0].ServiceActions, "updated practice_states") {
		t.Fatalf("mock turn trace missing selected context/actions: %#v", turnTraces[0])
	}
}

func TestAgentDecisionTraceMemoryCandidateGenerationSuccess(t *testing.T) {
	t.Run("coaching session", func(t *testing.T) {
		s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
		runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "trace coaching candidate")

		if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err != nil {
			t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
		}

		traces := mustListAgentDecisionTraces(t, s, AgentDecisionTraceQuery{
			SourceType: AgentTraceSourceMemoryCandidateGeneration,
			SourceID:   completed.Session.SessionID,
			StepName:   AgentTraceStepCoachingSessionMemoryCandidates,
		})
		if len(traces) != 1 || !strings.Contains(traces[0].ServiceActions, "generated memory_candidates: 1") {
			t.Fatalf("coaching memory trace = %#v", traces)
		}
	})

	t.Run("mock interview", func(t *testing.T) {
		s, runners, completed := createCompletedMockInterviewForMemoryCandidates(t)
		runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "trace mock candidate")

		if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), completed.MockID); err != nil {
			t.Fatalf("GenerateMemoryCandidatesFromMockInterview() error = %v", err)
		}

		traces := mustListAgentDecisionTraces(t, s, AgentDecisionTraceQuery{
			SourceType: AgentTraceSourceMemoryCandidateGeneration,
			SourceID:   completed.MockID,
			StepName:   AgentTraceStepMockInterviewMemoryCandidates,
		})
		if len(traces) != 1 || !strings.Contains(traces[0].ServiceActions, "generated memory_candidates: 1") {
			t.Fatalf("mock memory trace = %#v", traces)
		}
	})
}

func TestAgentDecisionTraceParseFailure(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = "not json"

	if _, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	}); err == nil {
		t.Fatalf("GenerateCoachingPlan() error = nil, want parse error")
	}

	traces := mustListAgentDecisionTraces(t, s, AgentDecisionTraceQuery{
		InterviewID: session.InterviewID,
		StepName:    AgentTraceStepCoachingPlanGenerate,
		Status:      AgentDecisionTraceStatusFailed,
	})
	if len(traces) != 1 {
		t.Fatalf("failed trace count = %d, want 1", len(traces))
	}
	if traces[0].RawAgentOutput != "not json" || !strings.Contains(traces[0].ErrorMessage, "parse coaching plan JSON") {
		t.Fatalf("failed trace missing raw/error: %#v", traces[0])
	}
}

func TestAgentDecisionTraceControllerFilters(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session, memoryID := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = strings.ReplaceAll(
		sampleCoachingPlanJSON("api trace strategy", "api trace focus"),
		"MEMORY_ID_PLACEHOLDER",
		memoryID,
	)
	plan, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}

	rec := performJSONRequest(router, http.MethodGet, "/api/agent-decision-traces?source_type="+AgentTraceSourceCoachingPlan+"&source_id="+plan.PlanID+"&step_name="+AgentTraceStepCoachingPlanGenerate, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var traces []vo.AgentDecisionTraceVO
	decodeOKData(t, rec, &traces)
	if len(traces) != 1 || traces[0].SourceID != plan.PlanID || traces[0].StepName != AgentTraceStepCoachingPlanGenerate {
		t.Fatalf("traces = %#v", traces)
	}
}

func mustListAgentDecisionTraces(t *testing.T, s *Server, query AgentDecisionTraceQuery) []vo.AgentDecisionTraceVO {
	t.Helper()
	traces, err := s.ListAgentDecisionTraces(query)
	if err != nil {
		t.Fatalf("ListAgentDecisionTraces() error = %v", err)
	}
	return traces
}

func mustFindSingleTrace(t *testing.T, s *Server, query AgentDecisionTraceQuery) vo.AgentDecisionTraceVO {
	t.Helper()
	traces := mustListAgentDecisionTraces(t, s, query)
	if len(traces) != 1 {
		t.Fatalf("trace count = %d, want 1 for query %#v; traces=%#v", len(traces), query, traces)
	}
	return traces[0]
}

func assertTraceContainsAction(t *testing.T, trace vo.AgentDecisionTraceVO, actions ...string) {
	t.Helper()
	for _, action := range actions {
		if !strings.Contains(trace.ServiceActions, action) {
			t.Fatalf("trace %s service_actions missing %q: %s", trace.TraceID, action, trace.ServiceActions)
		}
	}
}

func assertTraceNotContainsAction(t *testing.T, trace vo.AgentDecisionTraceVO, action string) {
	t.Helper()
	if strings.Contains(trace.ServiceActions, action) {
		t.Fatalf("trace %s service_actions unexpectedly contains %q: %s", trace.TraceID, action, trace.ServiceActions)
	}
}

func assertTraceBase(t *testing.T, trace vo.AgentDecisionTraceVO, status string, agentType agent.AgentType, sourceType string, sourceID string, stepName string) {
	t.Helper()
	if trace.Status != status {
		t.Fatalf("trace status = %q, want %q; trace=%#v", trace.Status, status, trace)
	}
	if trace.AgentType != string(agentType) {
		t.Fatalf("trace agent_type = %q, want %q; trace=%#v", trace.AgentType, agentType, trace)
	}
	if trace.SourceType != sourceType || trace.SourceID != sourceID {
		t.Fatalf("trace source = %s/%s, want %s/%s; trace=%#v", trace.SourceType, trace.SourceID, sourceType, sourceID, trace)
	}
	if trace.StepName != stepName {
		t.Fatalf("trace step_name = %q, want %q; trace=%#v", trace.StepName, stepName, trace)
	}
}

func assertNoPracticeStates(t *testing.T, s *Server, userID string) {
	t.Helper()
	states, err := s.ListPracticeStates(userID, "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("practice states length = %d, want 0; states=%#v", len(states), states)
	}
}
