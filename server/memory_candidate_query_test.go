package server

import (
	"net/http"
	"testing"

	"agent-web-base/vo"
)

func TestListMemoryCandidatesByQueryFiltersAcrossSources(t *testing.T) {
	s := newTestServer(t)
	reviewInterview := createMemoryCandidateQueryInterview(t, s, "user_mem", "Acme", "Backend Engineer")
	coachingInterview := createMemoryCandidateQueryInterview(t, s, "user_mem", "Acme", "Backend Engineer")
	mockInterview := createMemoryCandidateQueryInterview(t, s, "user_mem", "Beta", "Platform Engineer")
	otherUserInterview := createMemoryCandidateQueryInterview(t, s, "other_user", "Acme", "Backend Engineer")

	seedMemoryCandidateForQuery(t, s, "candidate_review_pending", reviewInterview.InterviewID, "user_mem", MemoryCandidateStatusPending, MemorySourceReviewReport)
	seedMemoryCandidateForQuery(t, s, "candidate_coaching_pending", coachingInterview.InterviewID, "user_mem", MemoryCandidateStatusPending, MemorySourceCoachingSession)
	seedMemoryCandidateForQuery(t, s, "candidate_coaching_accepted", coachingInterview.InterviewID, "user_mem", MemoryCandidateStatusAccepted, MemorySourceCoachingSession)
	seedMemoryCandidateForQuery(t, s, "candidate_mock_pending", mockInterview.InterviewID, "user_mem", MemoryCandidateStatusPending, MemorySourceMockInterview)
	seedMemoryCandidateForQuery(t, s, "candidate_other_user_pending", otherUserInterview.InterviewID, "other_user", MemoryCandidateStatusPending, MemorySourceCoachingSession)

	candidates, err := s.ListMemoryCandidatesByQuery(MemoryCandidateQuery{
		UserID:        "user_mem",
		Status:        MemoryCandidateStatusPending,
		SourceRefType: MemorySourceCoachingSession,
		CompanyName:   "Acme",
		JobTitle:      "Backend Engineer",
	})
	if err != nil {
		t.Fatalf("ListMemoryCandidatesByQuery() error = %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1; candidates=%#v", len(candidates), candidates)
	}
	if candidates[0].CandidateID != "candidate_coaching_pending" {
		t.Fatalf("candidate_id = %q, want candidate_coaching_pending", candidates[0].CandidateID)
	}
}

func TestGlobalMemoryCandidatesControllerRequiresUserID(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)

	rec := performJSONRequest(router, http.MethodGet, "/api/memory-candidates?status=pending", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestGlobalMemoryCandidatesControllerReturnsFilteredCandidates(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)
	interview := createMemoryCandidateQueryInterview(t, s, "user_mem", "Acme", "Backend Engineer")
	seedMemoryCandidateForQuery(t, s, "candidate_mock_pending", interview.InterviewID, "user_mem", MemoryCandidateStatusPending, MemorySourceMockInterview)
	seedMemoryCandidateForQuery(t, s, "candidate_coaching_pending", interview.InterviewID, "user_mem", MemoryCandidateStatusPending, MemorySourceCoachingSession)

	rec := performJSONRequest(router, http.MethodGet, "/api/memory-candidates?user_id=user_mem&status=pending&source_ref_type=mock_interview", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var candidates []vo.MemoryCandidateVO
	decodeOKData(t, rec, &candidates)
	if len(candidates) != 1 {
		t.Fatalf("candidates length = %d, want 1; candidates=%#v", len(candidates), candidates)
	}
	if candidates[0].CandidateID != "candidate_mock_pending" {
		t.Fatalf("candidate_id = %q, want candidate_mock_pending", candidates[0].CandidateID)
	}
}

func createMemoryCandidateQueryInterview(t *testing.T, s *Server, userID string, companyName string, jobTitle string) vo.InterviewSessionVO {
	t.Helper()

	session, err := s.CreateInterview(vo.CreateInterviewReq{
		UserID:         userID,
		CompanyName:    companyName,
		JobTitle:       jobTitle,
		InterviewRound: "technical",
		InterviewType:  "backend",
	})
	if err != nil {
		t.Fatalf("CreateInterview() error = %v", err)
	}
	return session
}

func seedMemoryCandidateForQuery(t *testing.T, s *Server, candidateID string, interviewID string, userID string, status string, sourceRefType string) {
	t.Helper()

	if err := s.db.Create(&MemoryCandidate{
		CandidateID:    candidateID,
		UserID:         userID,
		InterviewID:    interviewID,
		MemoryType:     MemoryTypeUserWeakness,
		SubjectKey:     "redis-consistency",
		Content:        "Needs stronger Redis consistency explanation.",
		Evidence:       "Seeded from test.",
		Confidence:     MemoryConfidenceHigh,
		Status:         status,
		Source:         sourceRefType,
		SourceRefType:  sourceRefType,
		SourceRefID:    candidateID + "_source",
		RawAgentOutput: "{}",
		CreatedAt:      100,
		UpdatedAt:      100,
	}).Error; err != nil {
		t.Fatalf("seed memory candidate: %v", err)
	}
}
