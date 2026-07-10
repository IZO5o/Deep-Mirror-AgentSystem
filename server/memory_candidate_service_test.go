package server

import (
	"context"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestGenerateMemoryCandidatesRequiresReviewedInterview(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createTestInterview(t, s, "user_001")

	if _, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID); err == nil {
		t.Fatalf("GenerateMemoryCandidates() error = nil, want error")
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != 0 {
		t.Fatalf("memory curator calls = %d, want 0", runners[agent.AgentTypeMemoryCurator].taskCalls)
	}
}

func TestGenerateMemoryCandidatesCreatesPendingCandidates(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("Redis consistency weakness")

	candidates, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates length = %d, want 2", len(candidates))
	}
	if candidates[0].Status != MemoryCandidateStatusPending {
		t.Fatalf("candidate status = %q, want %q", candidates[0].Status, MemoryCandidateStatusPending)
	}
	if candidates[0].MemoryType != MemoryTypeUserWeakness {
		t.Fatalf("memory_type = %q, want %q", candidates[0].MemoryType, MemoryTypeUserWeakness)
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != 1 {
		t.Fatalf("memory curator calls = %d, want 1", runners[agent.AgentTypeMemoryCurator].taskCalls)
	}
}

func TestGenerateMemoryCandidatesReplacesOnlyPendingCandidates(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)

	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("first weakness")
	first, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("first GenerateMemoryCandidates() error = %v", err)
	}
	if _, err := s.AcceptMemoryCandidate(first[0].CandidateID); err != nil {
		t.Fatalf("AcceptMemoryCandidate() error = %v", err)
	}
	if _, err := s.RejectMemoryCandidate(first[1].CandidateID); err != nil {
		t.Fatalf("RejectMemoryCandidate() error = %v", err)
	}

	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("second weakness")
	if _, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID); err != nil {
		t.Fatalf("second GenerateMemoryCandidates() error = %v", err)
	}

	all, err := s.ListMemoryCandidates(session.InterviewID)
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	statusCounts := map[string]int{}
	for _, candidate := range all {
		statusCounts[candidate.Status]++
	}
	if statusCounts[MemoryCandidateStatusAccepted] != 1 {
		t.Fatalf("accepted count = %d, want 1", statusCounts[MemoryCandidateStatusAccepted])
	}
	if statusCounts[MemoryCandidateStatusRejected] != 1 {
		t.Fatalf("rejected count = %d, want 1", statusCounts[MemoryCandidateStatusRejected])
	}
	if statusCounts[MemoryCandidateStatusPending] != 2 {
		t.Fatalf("pending count = %d, want 2", statusCounts[MemoryCandidateStatusPending])
	}
}

func TestGenerateMemoryCandidatesParseFailureDoesNotWriteCandidates(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = "not json"

	if _, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID); err == nil {
		t.Fatalf("GenerateMemoryCandidates() error = nil, want error")
	}

	candidates, err := s.ListMemoryCandidates(session.InterviewID)
	if err != nil {
		t.Fatalf("ListMemoryCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates length = %d, want 0", len(candidates))
	}
}

func TestParseMemoryCuratorOutputAcceptsBareCandidateArray(t *testing.T) {
	parsed, err := parseMemoryCuratorOutput(`[
  {
    "memory_type": "user_weakness",
    "subject_key": "user:user_001",
    "content": "Needs deeper architecture detail.",
    "evidence": "Mock feedback asked for more implementation depth.",
    "confidence": "high",
    "source": "mock_interview"
  }
]`)
	if err != nil {
		t.Fatalf("parseMemoryCuratorOutput() error = %v", err)
	}
	if len(parsed.Candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1", len(parsed.Candidates))
	}
	if parsed.Candidates[0].Source != MemorySourceMockInterview {
		t.Fatalf("source = %q, want %q", parsed.Candidates[0].Source, MemorySourceMockInterview)
	}
}

func TestGenerateMemoryCandidatesFiltersPrivateInterviewerSignals(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = `{
  "candidates": [
    {
      "memory_type": "interviewer_focus",
      "subject_key": "interviewer:bad",
      "content": "面试官是女性，外貌很年轻。",
      "evidence": "私人属性",
      "confidence": "high",
      "source": "agent_generated"
    },
    {
      "memory_type": "interviewer_focus",
      "subject_key": "interviewer:ok",
      "content": "面试官重视异常处理和幂等设计。",
      "evidence": "多次追问失败恢复。",
      "confidence": "high",
      "source": "interview_question"
    }
  ]
}`

	candidates, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1", len(candidates))
	}
	if candidates[0].SubjectKey != "interviewer:"+session.InterviewID+":professional_focus" {
		t.Fatalf("subject_key = %q, want normalized professional_focus key", candidates[0].SubjectKey)
	}
}

func TestAcceptMemoryCandidateIsIdempotent(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("Redis consistency weakness")
	candidates, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}

	first, err := s.AcceptMemoryCandidate(candidates[0].CandidateID)
	if err != nil {
		t.Fatalf("first AcceptMemoryCandidate() error = %v", err)
	}
	second, err := s.AcceptMemoryCandidate(candidates[0].CandidateID)
	if err != nil {
		t.Fatalf("second AcceptMemoryCandidate() error = %v", err)
	}
	if second.MemoryID != first.MemoryID {
		t.Fatalf("memory_id = %q, want %q", second.MemoryID, first.MemoryID)
	}

	items, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
}

func TestRejectMemoryCandidateIsIdempotentAndDoesNotCreateItem(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("Redis consistency weakness")
	candidates, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}

	first, err := s.RejectMemoryCandidate(candidates[0].CandidateID)
	if err != nil {
		t.Fatalf("first RejectMemoryCandidate() error = %v", err)
	}
	second, err := s.RejectMemoryCandidate(candidates[0].CandidateID)
	if err != nil {
		t.Fatalf("second RejectMemoryCandidate() error = %v", err)
	}
	if second.CandidateID != first.CandidateID {
		t.Fatalf("candidate_id = %q, want %q", second.CandidateID, first.CandidateID)
	}
	if second.Status != MemoryCandidateStatusRejected {
		t.Fatalf("status = %q, want %q", second.Status, MemoryCandidateStatusRejected)
	}

	items, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items length = %d, want 0", len(items))
	}
}

func TestRejectAcceptedMemoryCandidateFails(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("Redis consistency weakness")
	candidates, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}
	if _, err := s.AcceptMemoryCandidate(candidates[0].CandidateID); err != nil {
		t.Fatalf("AcceptMemoryCandidate() error = %v", err)
	}

	if _, err := s.RejectMemoryCandidate(candidates[0].CandidateID); err == nil {
		t.Fatalf("RejectMemoryCandidate() error = nil, want error")
	}
}

func TestGenerateMemoryCandidatesFromCompletedCoachingSession(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	beforeItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "coaching stable weakness")

	candidates, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.InterviewID != completed.Session.InterviewID || candidate.UserID != completed.Session.UserID {
		t.Fatalf("candidate owner = %#v, want session interview/user", candidate)
	}
	if candidate.Status != MemoryCandidateStatusPending || candidate.Source != MemorySourceCoachingSession {
		t.Fatalf("candidate status/source = %s/%s", candidate.Status, candidate.Source)
	}
	if candidate.SourceRefType != MemorySourceCoachingSession || candidate.SourceRefID != completed.Session.SessionID {
		t.Fatalf("candidate source_ref = %s/%s, want coaching session", candidate.SourceRefType, candidate.SourceRefID)
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls+1 {
		t.Fatalf("memory curator calls = %d, want %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls+1)
	}
	afterItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(afterItems) != len(beforeItems) {
		t.Fatalf("memory items length changed from %d to %d before accept", len(beforeItems), len(afterItems))
	}

	item, err := s.AcceptMemoryCandidate(candidate.CandidateID)
	if err != nil {
		t.Fatalf("AcceptMemoryCandidate() error = %v", err)
	}
	if item.SourceCandidateID != candidate.CandidateID {
		t.Fatalf("source_candidate_id = %q, want %q", item.SourceCandidateID, candidate.CandidateID)
	}
}

func TestGenerateMemoryCandidatesFromNonCompletedCoachingSessionFails(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), session.Session.SessionID); err == nil {
		t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = nil, want error")
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls {
		t.Fatalf("memory curator calls = %d, want unchanged %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls)
	}
}

func TestGenerateMemoryCandidatesFromCoachingSessionIsIdempotent(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "coaching stable weakness")

	first, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID)
	if err != nil {
		t.Fatalf("first GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}
	second, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID)
	if err != nil {
		t.Fatalf("second GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].CandidateID != second[0].CandidateID {
		t.Fatalf("idempotent candidates first=%#v second=%#v", first, second)
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls+1 {
		t.Fatalf("memory curator calls = %d, want %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls+1)
	}
}

func TestGenerateMemoryCandidatesFromCoachingSessionCreatesMemoryEvents(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "coaching stable weakness")

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err != nil {
		t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}

	events := loadMemoryEventsBySource(t, s, MemorySourceCoachingSession, completed.Session.SessionID)
	if len(events) != 2 {
		t.Fatalf("events length = %d, want 2; events=%#v", len(events), events)
	}
	for _, event := range events {
		if event.UserID != completed.Session.UserID || event.SourceType != MemorySourceCoachingSession || event.SourceID != completed.Session.SessionID {
			t.Fatalf("event ownership/source = %#v, want completed coaching session", event)
		}
		if !strings.Contains(event.Observation, "completed coaching task") || event.ScoreTrend == "" {
			t.Fatalf("event missing factual observation/score trend: %#v", event)
		}
	}
}

func TestGenerateMemoryCandidatesFromCoachingSessionEventsAreIdempotent(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "coaching stable weakness")

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err != nil {
		t.Fatalf("first GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}
	firstEvents := loadMemoryEventsBySource(t, s, MemorySourceCoachingSession, completed.Session.SessionID)

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err != nil {
		t.Fatalf("second GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}
	secondEvents := loadMemoryEventsBySource(t, s, MemorySourceCoachingSession, completed.Session.SessionID)

	if len(firstEvents) != len(secondEvents) {
		t.Fatalf("event count changed from %d to %d", len(firstEvents), len(secondEvents))
	}
	for i := range firstEvents {
		if firstEvents[i].EventID != secondEvents[i].EventID {
			t.Fatalf("event[%d] id changed from %q to %q", i, firstEvents[i].EventID, secondEvents[i].EventID)
		}
	}
}

func TestGenerateMemoryCandidatesFromCoachingSessionBackfillsMissingEventsForExistingCandidates(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "coaching stable weakness")

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err != nil {
		t.Fatalf("first GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}
	if err := s.db.Where("source_type = ? AND source_id = ?", MemorySourceCoachingSession, completed.Session.SessionID).
		Delete(&MemoryEvent{}).Error; err != nil {
		t.Fatalf("delete memory events: %v", err)
	}
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err != nil {
		t.Fatalf("second GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}
	events := loadMemoryEventsBySource(t, s, MemorySourceCoachingSession, completed.Session.SessionID)
	if len(events) != 2 {
		t.Fatalf("events length = %d, want 2; events=%#v", len(events), events)
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls {
		t.Fatalf("memory curator calls = %d, want unchanged %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls)
	}
}

func TestGenerateMemoryCandidatesFromCompletedMockInterview(t *testing.T) {
	s, runners, mock := createCompletedMockInterviewForMemoryCandidates(t)
	beforeItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "mock stable weakness")

	candidates, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.InterviewID != mock.InterviewID || candidate.UserID != mock.UserID {
		t.Fatalf("candidate owner = %#v, want mock interview/user", candidate)
	}
	if candidate.Status != MemoryCandidateStatusPending || candidate.Source != MemorySourceMockInterview {
		t.Fatalf("candidate status/source = %s/%s", candidate.Status, candidate.Source)
	}
	if candidate.SourceRefType != MemorySourceMockInterview || candidate.SourceRefID != mock.MockID {
		t.Fatalf("candidate source_ref = %s/%s, want mock interview", candidate.SourceRefType, candidate.SourceRefID)
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls+1 {
		t.Fatalf("memory curator calls = %d, want %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls+1)
	}
	afterItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(afterItems) != len(beforeItems) {
		t.Fatalf("memory items length changed from %d to %d before accept", len(beforeItems), len(afterItems))
	}
}

func TestGenerateMemoryCandidatesFromNonCompletedMockInterviewFails(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err == nil {
		t.Fatalf("GenerateMemoryCandidatesFromMockInterview() error = nil, want error")
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls {
		t.Fatalf("memory curator calls = %d, want unchanged %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls)
	}
}

func TestGenerateMemoryCandidatesFromMockInterviewIsIdempotent(t *testing.T) {
	s, runners, mock := createCompletedMockInterviewForMemoryCandidates(t)
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "mock stable weakness")

	first, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID)
	if err != nil {
		t.Fatalf("first GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}
	second, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID)
	if err != nil {
		t.Fatalf("second GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].CandidateID != second[0].CandidateID {
		t.Fatalf("idempotent candidates first=%#v second=%#v", first, second)
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls+1 {
		t.Fatalf("memory curator calls = %d, want %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls+1)
	}
}

func TestGenerateMemoryCandidatesFromMockInterviewCreatesMemoryEvents(t *testing.T) {
	s, runners, mock := createCompletedMockInterviewForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "mock stable weakness")

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err != nil {
		t.Fatalf("GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}

	events := loadMemoryEventsBySource(t, s, MemorySourceMockInterview, mock.MockID)
	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1; events=%#v", len(events), events)
	}
	event := events[0]
	if event.UserID != mock.UserID || event.SourceType != MemorySourceMockInterview || event.SourceID != mock.MockID {
		t.Fatalf("event ownership/source = %#v, want completed mock interview", event)
	}
	if !strings.Contains(event.Observation, "completed mock interview topic") || event.ScoreTrend == "" {
		t.Fatalf("event missing factual observation/score trend: %#v", event)
	}
}

func TestGenerateMemoryCandidatesFromMockInterviewEventsAreIdempotent(t *testing.T) {
	s, runners, mock := createCompletedMockInterviewForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "mock stable weakness")

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err != nil {
		t.Fatalf("first GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}
	firstEvents := loadMemoryEventsBySource(t, s, MemorySourceMockInterview, mock.MockID)

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err != nil {
		t.Fatalf("second GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}
	secondEvents := loadMemoryEventsBySource(t, s, MemorySourceMockInterview, mock.MockID)

	if len(firstEvents) != len(secondEvents) {
		t.Fatalf("event count changed from %d to %d", len(firstEvents), len(secondEvents))
	}
	for i := range firstEvents {
		if firstEvents[i].EventID != secondEvents[i].EventID {
			t.Fatalf("event[%d] id changed from %q to %q", i, firstEvents[i].EventID, secondEvents[i].EventID)
		}
	}
}

func TestGenerateMemoryCandidatesFromMockInterviewBackfillsMissingEventsForExistingCandidates(t *testing.T) {
	s, runners, mock := createCompletedMockInterviewForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "mock stable weakness")

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err != nil {
		t.Fatalf("first GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}
	if err := s.db.Where("source_type = ? AND source_id = ?", MemorySourceMockInterview, mock.MockID).
		Delete(&MemoryEvent{}).Error; err != nil {
		t.Fatalf("delete memory events: %v", err)
	}
	beforeCalls := runners[agent.AgentTypeMemoryCurator].taskCalls

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err != nil {
		t.Fatalf("second GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}
	events := loadMemoryEventsBySource(t, s, MemorySourceMockInterview, mock.MockID)
	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1; events=%#v", len(events), events)
	}
	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeCalls {
		t.Fatalf("memory curator calls = %d, want unchanged %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeCalls)
	}
}

func TestGenerateMemoryCandidatesFromSessionParseFailureDoesNotWriteCandidates(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = "not json"

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err == nil {
		t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = nil, want parse error")
	}
	count := countMemoryCandidatesBySourceRef(t, s, MemorySourceCoachingSession, completed.Session.SessionID)
	if count != 0 {
		t.Fatalf("source candidates count = %d, want 0", count)
	}
}

func TestGenerateMemoryCandidatesFromSessionParseFailureDoesNotWriteEvents(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = "not json"

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err == nil {
		t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = nil, want parse error")
	}
	count := countMemoryEventsBySource(t, s, MemorySourceCoachingSession, completed.Session.SessionID)
	if count != 0 {
		t.Fatalf("source events count = %d, want 0", count)
	}
}

func TestGenerateMemoryCandidatesFromMockParseFailureDoesNotWriteEvents(t *testing.T) {
	s, runners, mock := createCompletedMockInterviewForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = "not json"

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err == nil {
		t.Fatalf("GenerateMemoryCandidatesFromMockInterview() error = nil, want parse error")
	}
	count := countMemoryEventsBySource(t, s, MemorySourceMockInterview, mock.MockID)
	if count != 0 {
		t.Fatalf("source events count = %d, want 0", count)
	}
}

func TestGenerateMemoryCandidatesFromCoachingSessionDoesNotWriteMemoryItemsBeforeAccept(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "coaching stable weakness")
	beforeItems, err := s.ListMemoryItems(completed.Session.UserID)
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}

	if _, err := s.GenerateMemoryCandidatesFromCoachingSession(context.Background(), completed.Session.SessionID); err != nil {
		t.Fatalf("GenerateMemoryCandidatesFromCoachingSession() error = %v", err)
	}

	afterItems, err := s.ListMemoryItems(completed.Session.UserID)
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(afterItems) != len(beforeItems) {
		t.Fatalf("memory items length changed from %d to %d before explicit accept", len(beforeItems), len(afterItems))
	}
}

func TestGenerateMemoryCandidatesFromMockInterviewDoesNotWriteMemoryItemsBeforeAccept(t *testing.T) {
	s, runners, mock := createCompletedMockInterviewForMemoryCandidates(t)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "mock stable weakness")
	beforeItems, err := s.ListMemoryItems(mock.UserID)
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}

	if _, err := s.GenerateMemoryCandidatesFromMockInterview(context.Background(), mock.MockID); err != nil {
		t.Fatalf("GenerateMemoryCandidatesFromMockInterview() error = %v", err)
	}

	afterItems, err := s.ListMemoryItems(mock.UserID)
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(afterItems) != len(beforeItems) {
		t.Fatalf("memory items length changed from %d to %d before explicit accept", len(beforeItems), len(afterItems))
	}
}

func createMemoryReadyInterview(t *testing.T, s *Server, runners map[agent.AgentType]*fakeRunner) vo.InterviewSessionVO {
	t.Helper()

	session := createReviewedTestInterview(t, s, "user_001")
	runners[agent.AgentTypeReview].taskResponse = sampleReviewJSON("review summary", "review question")
	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v", err)
	}
	return session
}

func createCompletedCoachingSessionForMemoryCandidates(t *testing.T) (*Server, map[agent.AgentType]*fakeRunner, vo.CoachingSessionDetailVO) {
	t.Helper()
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	runners[agent.AgentTypeSecondRoundCoach].taskResponses = []string{
		sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 88, "第一项达标。", CoachingNextActionPromptNext, false),
		sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 90, "全部达标。", CoachingNextActionCompletePlan, false),
	}
	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "first answer", SubmitMode: CoachingSubmitModeFormalAnswer}); err != nil {
		t.Fatalf("first SubmitCoachingSessionTurn() error = %v", err)
	}
	completed, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "second answer", SubmitMode: CoachingSubmitModeFormalAnswer})
	if err != nil {
		t.Fatalf("second SubmitCoachingSessionTurn() error = %v", err)
	}
	if completed.Session.Status != CoachingSessionStatusCompleted {
		t.Fatalf("session status = %q, want completed", completed.Session.Status)
	}
	return s, runners, completed
}

func createCompletedMockInterviewForMemoryCandidates(t *testing.T) (*Server, map[agent.AgentType]*fakeRunner, vo.MockInterviewVO) {
	t.Helper()
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockCompleteTurnJSON(),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "answer"}); err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	completed, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if completed.Status != MockInterviewStatusCompleted {
		t.Fatalf("mock status = %q, want completed", completed.Status)
	}
	return s, runners, completed
}

func countMemoryCandidatesBySourceRef(t *testing.T, s *Server, sourceRefType string, sourceRefID string) int64 {
	t.Helper()
	var count int64
	if err := s.db.Model(&MemoryCandidate{}).
		Where("source_ref_type = ? AND source_ref_id = ?", sourceRefType, sourceRefID).
		Count(&count).Error; err != nil {
		t.Fatalf("count memory candidates: %v", err)
	}
	return count
}

func loadMemoryEventsBySource(t *testing.T, s *Server, sourceType string, sourceID string) []MemoryEvent {
	t.Helper()
	var events []MemoryEvent
	if err := s.db.Where("source_type = ? AND source_id = ?", sourceType, sourceID).
		Order("topic asc, created_at asc").
		Find(&events).Error; err != nil {
		t.Fatalf("load memory events: %v", err)
	}
	return events
}

func countMemoryEventsBySource(t *testing.T, s *Server, sourceType string, sourceID string) int64 {
	t.Helper()
	var count int64
	if err := s.db.Model(&MemoryEvent{}).
		Where("source_type = ? AND source_id = ?", sourceType, sourceID).
		Count(&count).Error; err != nil {
		t.Fatalf("count memory events: %v", err)
	}
	return count
}

func sampleSourceMemoryCandidateJSON(source string, content string) string {
	return `{
  "candidates": [
    {
      "memory_type": "user_weakness",
      "subject_key": "user:user_001",
      "content": "` + content + `",
      "evidence": "来自` + strings.ReplaceAll(source, "_", " ") + `完成后的稳定表现。",
      "confidence": "high",
      "source": "` + source + `"
    }
  ]
}`
}

func sampleMemoryCandidateJSON(weakness string) string {
	return `{
  "candidates": [
    {
      "memory_type": "user_weakness",
      "subject_key": "user:user_001",
      "content": "` + weakness + `",
      "evidence": "来自第 1 个问题。",
      "confidence": "high",
      "source": "interview_question"
    },
    {
      "memory_type": "preparation_tip",
      "subject_key": "user:user_001",
      "content": "准备 Redis、MySQL 和 Go 并发的项目追问。",
      "evidence": "复盘报告建议加强工程细节。",
      "confidence": "medium",
      "source": "review_report"
    }
  ]
}`
}
