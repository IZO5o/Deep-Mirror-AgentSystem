package server

import (
	"net/http"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestMemoryCandidateControllerFlow(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("controller weakness")

	generateRec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+session.InterviewID+"/memory-candidates", "")
	if generateRec.Code != http.StatusOK {
		t.Fatalf("generate status = %d, want %d; body=%s", generateRec.Code, http.StatusOK, generateRec.Body.String())
	}
	var generated []vo.MemoryCandidateVO
	decodeOKData(t, generateRec, &generated)
	if len(generated) != 2 {
		t.Fatalf("generated length = %d, want 2", len(generated))
	}

	listRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+session.InterviewID+"/memory-candidates", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var listed []vo.MemoryCandidateVO
	decodeOKData(t, listRec, &listed)
	if len(listed) != 2 {
		t.Fatalf("listed length = %d, want 2", len(listed))
	}

	acceptRec := performJSONRequest(router, http.MethodPost, "/api/memory-candidates/"+generated[0].CandidateID+"/accept", "")
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept status = %d, want %d; body=%s", acceptRec.Code, http.StatusOK, acceptRec.Body.String())
	}
	var item vo.MemoryItemVO
	decodeOKData(t, acceptRec, &item)
	if item.SourceCandidateID != generated[0].CandidateID {
		t.Fatalf("source_candidate_id = %q, want %q", item.SourceCandidateID, generated[0].CandidateID)
	}

	rejectRec := performJSONRequest(router, http.MethodPost, "/api/memory-candidates/"+generated[1].CandidateID+"/reject", "")
	if rejectRec.Code != http.StatusOK {
		t.Fatalf("reject status = %d, want %d; body=%s", rejectRec.Code, http.StatusOK, rejectRec.Body.String())
	}
	var rejected vo.MemoryCandidateVO
	decodeOKData(t, rejectRec, &rejected)
	if rejected.Status != MemoryCandidateStatusRejected {
		t.Fatalf("rejected status = %q, want %q", rejected.Status, MemoryCandidateStatusRejected)
	}

	itemsRec := performJSONRequest(router, http.MethodGet, "/api/memory-items?user_id=user_001", "")
	if itemsRec.Code != http.StatusOK {
		t.Fatalf("items status = %d, want %d; body=%s", itemsRec.Code, http.StatusOK, itemsRec.Body.String())
	}
	var items []vo.MemoryItemVO
	decodeOKData(t, itemsRec, &items)
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	if items[0].MemoryID != item.MemoryID {
		t.Fatalf("memory_id = %q, want %q", items[0].MemoryID, item.MemoryID)
	}
}

func TestMemoryCandidateControllerGenerateBeforeReviewedReturnsError(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	rec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+session.InterviewID+"/memory-candidates", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestMemoryCandidateControllerGenerateFromCoachingSession(t *testing.T) {
	s, runners, completed := createCompletedCoachingSessionForMemoryCandidates(t)
	router := NewRouter(s)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceCoachingSession, "controller coaching weakness")

	rec := performJSONRequest(router, http.MethodPost, "/api/coaching-sessions/"+completed.Session.SessionID+"/memory-candidates", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var candidates []vo.MemoryCandidateVO
	decodeOKData(t, rec, &candidates)
	if len(candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1", len(candidates))
	}
	if candidates[0].SourceRefType != MemorySourceCoachingSession || candidates[0].SourceRefID != completed.Session.SessionID {
		t.Fatalf("source_ref = %s/%s, want coaching session", candidates[0].SourceRefType, candidates[0].SourceRefID)
	}
}

func TestMemoryCandidateControllerGenerateFromMockInterview(t *testing.T) {
	s, runners, completed := createCompletedMockInterviewForMemoryCandidates(t)
	router := NewRouter(s)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleSourceMemoryCandidateJSON(MemorySourceMockInterview, "controller mock weakness")

	rec := performJSONRequest(router, http.MethodPost, "/api/mock-interviews/"+completed.MockID+"/memory-candidates", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var candidates []vo.MemoryCandidateVO
	decodeOKData(t, rec, &candidates)
	if len(candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1", len(candidates))
	}
	if candidates[0].SourceRefType != MemorySourceMockInterview || candidates[0].SourceRefID != completed.MockID {
		t.Fatalf("source_ref = %s/%s, want mock interview", candidates[0].SourceRefType, candidates[0].SourceRefID)
	}
}
