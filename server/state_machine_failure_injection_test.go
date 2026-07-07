package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestFailureInjectionCoachingSessionAgentRunFailureTrace(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	runners[agent.AgentTypeSecondRoundCoach].taskErr = errors.New("model unavailable")
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = "partial coaching output"

	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "正式回答", SubmitMode: CoachingSubmitModeFormalAnswer}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = nil, want run error")
	}

	updated, err := s.GetCoachingSession(session.Session.SessionID)
	if err != nil {
		t.Fatalf("GetCoachingSession() error = %v", err)
	}
	if updated.Session.Status != CoachingSessionStatusFailed || !strings.Contains(updated.Session.ErrorMessage, "model unavailable") {
		t.Fatalf("session = %#v, want failed with model error", updated.Session)
	}
	if len(updated.Attempts) != 0 {
		t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
	}
	assertNoPracticeStates(t, s, "user_001")
	if len(updated.Turns) != 3 || updated.Turns[2].TurnType != CoachingTurnTypeError || updated.Turns[2].RawAgentOutput != "partial coaching output" {
		t.Fatalf("turns = %#v, want user + error turn with raw output", updated.Turns)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
		Status:     AgentDecisionTraceStatusFailed,
	})
	assertTraceBase(t, trace, AgentDecisionTraceStatusFailed, agent.AgentTypeSecondRoundCoach, AgentTraceSourceCoachingSession, session.Session.SessionID, AgentTraceStepCoachingSessionTurn)
	if trace.RawAgentOutput != "partial coaching output" || !strings.Contains(trace.ErrorMessage, "model unavailable") {
		t.Fatalf("trace missing raw/error: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "recorded failed coaching_session_turn", "updated coaching_session failed")
}

func TestFailureInjectionCoachingSessionParseFailureTrace(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = "not json"

	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "正式回答", SubmitMode: CoachingSubmitModeFormalAnswer}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = nil, want parse error")
	}
	updated, err := s.GetCoachingSession(session.Session.SessionID)
	if err != nil {
		t.Fatalf("GetCoachingSession() error = %v", err)
	}
	if updated.Session.Status != CoachingSessionStatusFailed || len(updated.Attempts) != 0 {
		t.Fatalf("session/attempts = %#v/%#v, want failed and no attempts", updated.Session, updated.Attempts)
	}
	assertNoPracticeStates(t, s, "user_001")

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
		Status:     AgentDecisionTraceStatusFailed,
	})
	if trace.RawAgentOutput != "not json" || !strings.Contains(trace.ErrorMessage, "parse coaching session JSON") {
		t.Fatalf("trace missing raw/error: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "recorded failed coaching_session_turn", "updated coaching_session failed")
}

func TestFailureInjectionCoachingPracticeStateUpdateRollbackTrace(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 88, "回答达标。", CoachingNextActionPromptNext, false)
	if err := s.db.Exec("CREATE TRIGGER fail_practice_insert BEFORE INSERT ON practice_states BEGIN SELECT RAISE(FAIL, 'injected practice failure'); END").Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "正式回答", SubmitMode: CoachingSubmitModeFormalAnswer}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = nil, want practice update error")
	}
	updated, err := s.GetCoachingSession(session.Session.SessionID)
	if err != nil {
		t.Fatalf("GetCoachingSession() error = %v", err)
	}
	if len(updated.Turns) != 1 || len(updated.Attempts) != 0 {
		t.Fatalf("turns/attempts = %d/%d, want rollback to start-only and no attempts", len(updated.Turns), len(updated.Attempts))
	}
	if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer || updated.Session.CurrentTaskID != taskID {
		t.Fatalf("session = %#v, want unchanged waiting current task", updated.Session)
	}
	var task CoachingTask
	if err := s.db.First(&task, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != CoachingTaskStatusInProgress {
		t.Fatalf("task status = %q, want %q", task.Status, CoachingTaskStatusInProgress)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
		Status:     AgentDecisionTraceStatusFailed,
	})
	assertTraceContainsAction(t, trace, "failed to persist coaching_session_turn")
	if !strings.Contains(trace.ParsedDecision, CoachingInputTypeFormalAnswer) || !strings.Contains(trace.ErrorMessage, "injected practice failure") {
		t.Fatalf("trace missing parsed/error details: %#v", trace)
	}
}

func TestFailureInjectionMockTurnAgentRunFailureTrace(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	runners[agent.AgentTypeMockInterviewer].taskErr = errors.New("model unavailable")
	runners[agent.AgentTypeMockInterviewer].taskResponse = "partial mock output"

	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "answer"}); err == nil {
		t.Fatalf("SubmitMockTurn() error = nil, want run error")
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.Status != MockInterviewStatusFailed || got.RawAgentOutput != "partial mock output" {
		t.Fatalf("mock = %#v, want failed with raw output", got)
	}
	assertNoPracticeStates(t, s, "user_001")

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusFailed,
	})
	if trace.RawAgentOutput != "partial mock output" || !strings.Contains(trace.ErrorMessage, "model unavailable") {
		t.Fatalf("trace missing raw/error: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "recorded failed mock_turns", "updated mock status failed")
}

func TestFailureInjectionMockTurnParseFailureTrace(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{sampleMockStartJSON(), "not json"}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}

	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "bad"}); err == nil {
		t.Fatalf("SubmitMockTurn() error = nil, want parse error")
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.Status != MockInterviewStatusFailed || got.RawAgentOutput != "not json" {
		t.Fatalf("mock = %#v, want failed with raw output", got)
	}
	assertNoPracticeStates(t, s, "user_001")

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusFailed,
	})
	if trace.RawAgentOutput != "not json" || !strings.Contains(trace.ErrorMessage, "parse mock turn JSON") {
		t.Fatalf("trace missing raw/error: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "recorded failed mock_turns", "updated mock status failed")
}

func TestFailureInjectionMockPracticeStateUpdateRollbackTrace(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSONWithTopics("next question", 72, []string{"Redis 缓存一致性"}),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if err := s.db.Exec("CREATE TRIGGER fail_practice_insert BEFORE INSERT ON practice_states BEGIN SELECT RAISE(FAIL, 'injected practice failure'); END").Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "answer"}); err == nil {
		t.Fatalf("SubmitMockTurn() error = nil, want practice update error")
	}
	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("turns length = %d, want opening only after rollback", len(turns))
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 0 || got.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock = %#v, want unchanged waiting state", got)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusFailed,
	})
	assertTraceContainsAction(t, trace, "failed to persist mock_turn")
	if !strings.Contains(trace.ParsedDecision, mockInputTypeFormalAnswer) || !strings.Contains(trace.ErrorMessage, "injected practice failure") {
		t.Fatalf("trace missing parsed/error details: %#v", trace)
	}
}

func TestFailureInjectionSessionMemoryCandidateParseFailureTrace(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	beforeItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}
	runners[agent.AgentTypeMemoryCurator].taskResponse = "not json"

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err == nil {
		t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = nil, want parse error")
	}
	if count := countMemoryCandidatesBySourceRef(t, s, MemorySourceCoachingSession, completed.Session.SessionID); count != 0 {
		t.Fatalf("source candidates count = %d, want 0", count)
	}
	afterItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(afterItems) != len(beforeItems) {
		t.Fatalf("memory_items changed from %d to %d", len(beforeItems), len(afterItems))
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMemoryCandidateGeneration,
		SourceID:   completed.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionMemoryCandidates,
		Status:     AgentDecisionTraceStatusFailed,
	})
	if trace.RawAgentOutput != "not json" || !strings.Contains(trace.ErrorMessage, "parse memory curator JSON") {
		t.Fatalf("trace missing raw/error: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "did not create memory_candidates")
}

func TestFailureInjectionMockMemoryCandidateParseFailureTrace(t *testing.T) {
	s, runners, completed := createCompletedMockInterviewForMemoryCandidates(t)
	beforeItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}
	runners[agent.AgentTypeMemoryCurator].taskResponse = "not json"

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), completed.MockID); err == nil {
		t.Fatalf("GenerateMemoryCandidatesFromMockInterview() error = nil, want parse error")
	}
	if count := countMemoryCandidatesBySourceRef(t, s, MemorySourceMockInterview, completed.MockID); count != 0 {
		t.Fatalf("source candidates count = %d, want 0", count)
	}
	afterItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(afterItems) != len(beforeItems) {
		t.Fatalf("memory_items changed from %d to %d", len(beforeItems), len(afterItems))
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMemoryCandidateGeneration,
		SourceID:   completed.MockID,
		StepName:   AgentTraceStepMockInterviewMemoryCandidates,
		Status:     AgentDecisionTraceStatusFailed,
	})
	if trace.RawAgentOutput != "not json" || !strings.Contains(trace.ErrorMessage, "parse memory curator JSON") {
		t.Fatalf("trace missing raw/error: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "did not create memory_candidates")
}
