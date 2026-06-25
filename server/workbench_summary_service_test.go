package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"agent-web-base/vo"
)

func TestGetDashboardSummaryAggregatesUserWorkbenchState(t *testing.T) {
	s := newTestServer(t)
	now := time.Now().Unix()

	interview := InterviewSession{
		InterviewID:    "interview_dash_1",
		UserID:         "user_dash",
		CompanyName:    "ByteDance",
		JobTitle:       "Agent Engineer",
		InterviewRound: "second_round",
		InterviewType:  "technical",
		Status:         "ready_for_review",
		OccurredAt:     now - 120,
		CreatedAt:      now - 100,
		UpdatedAt:      now - 90,
	}
	if err := s.db.Create(&interview).Error; err != nil {
		t.Fatalf("seed interview: %v", err)
	}
	if err := s.db.Create(&MemoryCandidate{
		CandidateID:   "candidate_dash_pending",
		UserID:        "user_dash",
		InterviewID:   interview.InterviewID,
		MemoryType:    "weakness",
		SubjectKey:    "user:weakness",
		Content:       "Needs tighter failure recovery explanation.",
		Evidence:      "Missed dirty-write rollback detail.",
		Confidence:    "high",
		Status:        MemoryCandidateStatusPending,
		Source:        "review_report",
		SourceRefType: AgentTraceSourceInterviewReview,
		SourceRefID:   interview.InterviewID,
		CreatedAt:     now - 80,
		UpdatedAt:     now - 80,
	}).Error; err != nil {
		t.Fatalf("seed pending candidate: %v", err)
	}
	if err := s.db.Create(&MemoryCandidate{
		CandidateID: "candidate_dash_accepted",
		UserID:      "user_dash",
		InterviewID: interview.InterviewID,
		Status:      "accepted",
		CreatedAt:   now - 79,
		UpdatedAt:   now - 79,
	}).Error; err != nil {
		t.Fatalf("seed accepted candidate: %v", err)
	}
	if err := s.db.Create(&CoachingSession{
		SessionID:        "session_dash_1",
		UserID:           "user_dash",
		InterviewID:      interview.InterviewID,
		CoachingPlanID:   "plan_dash_1",
		CurrentTaskID:    "task_dash_1",
		Status:           CoachingSessionStatusWaitingUserAnswer,
		ProgressSummary:  "current task 1",
		LastAgentMessage: "answer the next question",
		LastActiveAt:     now - 60,
		CreatedAt:        now - 70,
		UpdatedAt:        now - 60,
	}).Error; err != nil {
		t.Fatalf("seed coaching session: %v", err)
	}
	if err := s.db.Create(&MockInterview{
		MockID:        "mock_dash_1",
		UserID:        "user_dash",
		InterviewID:   interview.InterviewID,
		Status:        MockInterviewStatusWaitingAnswer,
		CurrentTurn:   1,
		CurrentTopic:  "failure recovery",
		OverallGoal:   "practice project deep dive",
		FirstQuestion: "How do you recover failed agent tool calls?",
		CreatedAt:     now - 50,
		UpdatedAt:     now - 40,
	}).Error; err != nil {
		t.Fatalf("seed mock: %v", err)
	}
	if err := s.db.Create(&PracticeState{
		StateID:         "practice_dash_1",
		UserID:          "user_dash",
		Topic:           "failure recovery",
		Dimension:       PracticeDimensionAgentProject,
		MasteryScore:    63,
		AttemptCount:    2,
		LastScore:       66,
		LastFeedback:    "Needs clearer rollback boundary.",
		LastPracticedAt: now - 30,
		SourceType:      PracticeStateSourceMockTurn,
		SourceID:        "mock_turn_dash_1",
		CreatedAt:       now - 120,
		UpdatedAt:       now - 30,
	}).Error; err != nil {
		t.Fatalf("seed practice state: %v", err)
	}
	if err := s.saveAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:         "user_dash",
		InterviewID:    interview.InterviewID,
		AgentType:      "second_round_coach",
		SourceType:     AgentTraceSourceCoachingSession,
		SourceID:       "session_dash_1",
		StepName:       AgentTraceStepCoachingSessionTurn,
		InputSnapshot:  `{"user_input":"answer"}`,
		Status:         AgentDecisionTraceStatusFailed,
		ErrorMessage:   "parse failed",
		ServiceActions: "failed before updating coaching_session; did not create memory_items",
	}); err != nil {
		t.Fatalf("seed trace: %v", err)
	}

	summary, err := s.GetDashboardSummary("user_dash")
	if err != nil {
		t.Fatalf("GetDashboardSummary() error = %v", err)
	}
	if summary.PendingMemoryCandidateCount != 1 {
		t.Fatalf("pending count = %d, want 1", summary.PendingMemoryCandidateCount)
	}
	if len(summary.RecentPendingCandidates) != 1 || summary.RecentPendingCandidates[0].CandidateID != "candidate_dash_pending" {
		t.Fatalf("recent pending candidates = %#v, want candidate_dash_pending", summary.RecentPendingCandidates)
	}
	if len(summary.RecentInterviews) != 1 || summary.RecentInterviews[0].InterviewID != interview.InterviewID {
		t.Fatalf("recent interviews = %#v, want seeded interview", summary.RecentInterviews)
	}
	if len(summary.ActiveCoachingSessions) != 1 || summary.ActiveCoachingSessions[0].SessionID != "session_dash_1" {
		t.Fatalf("active coaching sessions = %#v, want session_dash_1", summary.ActiveCoachingSessions)
	}
	if len(summary.ActiveMockInterviews) != 1 || summary.ActiveMockInterviews[0].MockID != "mock_dash_1" {
		t.Fatalf("active mocks = %#v, want mock_dash_1", summary.ActiveMockInterviews)
	}
	if summary.PracticeStateSummary.TotalStates != 1 || summary.PracticeStateSummary.AverageMasteryScore != 63 {
		t.Fatalf("practice summary = %#v, want one state average 63", summary.PracticeStateSummary)
	}
	if summary.PracticeStateSummary.WeakStateCount != 1 || summary.PracticeStateSummary.RecentAttemptCount != 2 {
		t.Fatalf("practice weak/recent summary = %#v, want weak=1 recent_attempt_count=2", summary.PracticeStateSummary)
	}
	if len(summary.RecentFailedTraces) != 1 || summary.RecentFailedTraces[0].Status != AgentDecisionTraceStatusFailed {
		t.Fatalf("recent failed traces = %#v, want one failed trace", summary.RecentFailedTraces)
	}
	if summary.EvaluationSummary.TotalTraces != 1 || summary.EvaluationSummary.FailedTraces != 1 {
		t.Fatalf("evaluation summary = %#v, want one failed trace", summary.EvaluationSummary)
	}
}

func TestGetDashboardSummaryRequiresUserID(t *testing.T) {
	s := newTestServer(t)

	_, err := s.GetDashboardSummary(" ")
	if err == nil || !strings.Contains(err.Error(), "user_id is required") {
		t.Fatalf("GetDashboardSummary() error = %v, want user_id required", err)
	}
}

func TestDashboardSummaryControllerRequiresUserID(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)

	rec := performJSONRequest(router, http.MethodGet, "/api/dashboard-summary", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user_id is required") {
		t.Fatalf("body = %s, want user_id required error", rec.Body.String())
	}
}

func TestDashboardSummaryControllerReturnsWrappedData(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)
	now := time.Now().Unix()

	interview := InterviewSession{
		InterviewID:    "interview_dash_controller_1",
		UserID:         "user_dash_controller",
		CompanyName:    "Acme",
		JobTitle:       "Backend Engineer",
		InterviewRound: "first_round",
		InterviewType:  "technical",
		Status:         InterviewStatusCreated,
		CreatedAt:      now - 20,
		UpdatedAt:      now - 10,
	}
	if err := s.db.Create(&interview).Error; err != nil {
		t.Fatalf("seed interview: %v", err)
	}

	rec := performJSONRequest(router, http.MethodGet, "/api/dashboard-summary?user_id=user_dash_controller", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var summary vo.DashboardSummaryVO
	decodeOKData(t, rec, &summary)
	if len(summary.RecentInterviews) != 1 || summary.RecentInterviews[0].InterviewID != interview.InterviewID {
		t.Fatalf("recent interviews = %#v, want seeded interview", summary.RecentInterviews)
	}
}
