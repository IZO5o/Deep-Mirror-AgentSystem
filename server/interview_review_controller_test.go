package server

import (
	"net/http"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestInterviewReviewControllerFlow(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	runners[agent.AgentTypeReview].taskResponse = sampleReviewJSON("controller summary", "controller question")

	createRec := performJSONRequest(router, http.MethodPost, "/api/interviews", `{"user_id":"user_001","company_name":"Acme"}`)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusOK, createRec.Body.String())
	}
	var created vo.InterviewSessionVO
	decodeOKData(t, createRec, &created)

	transcriptRec := performJSONRequest(router, http.MethodPut, "/api/interviews/"+created.InterviewID+"/transcript", `{"user_id":"user_001","content":"面试官：讲讲项目。候选人：..."}`)
	if transcriptRec.Code != http.StatusOK {
		t.Fatalf("transcript status = %d, want %d; body=%s", transcriptRec.Code, http.StatusOK, transcriptRec.Body.String())
	}

	reviewRec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+created.InterviewID+"/review", "")
	if reviewRec.Code != http.StatusOK {
		t.Fatalf("review status = %d, want %d; body=%s", reviewRec.Code, http.StatusOK, reviewRec.Body.String())
	}
	var report vo.InterviewReviewReportVO
	decodeOKData(t, reviewRec, &report)
	if report.Status != InterviewReviewStatusGenerated {
		t.Fatalf("report status = %q, want %q", report.Status, InterviewReviewStatusGenerated)
	}

	getReviewRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+created.InterviewID+"/review", "")
	if getReviewRec.Code != http.StatusOK {
		t.Fatalf("get review status = %d, want %d; body=%s", getReviewRec.Code, http.StatusOK, getReviewRec.Body.String())
	}
	var gotReport vo.InterviewReviewReportVO
	decodeOKData(t, getReviewRec, &gotReport)
	if gotReport.ReportID != report.ReportID {
		t.Fatalf("report_id = %q, want %q", gotReport.ReportID, report.ReportID)
	}

	questionsRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+created.InterviewID+"/questions", "")
	if questionsRec.Code != http.StatusOK {
		t.Fatalf("questions status = %d, want %d; body=%s", questionsRec.Code, http.StatusOK, questionsRec.Body.String())
	}
	var questions []vo.InterviewQuestionVO
	decodeOKData(t, questionsRec, &questions)
	if len(questions) != 1 {
		t.Fatalf("questions length = %d, want 1", len(questions))
	}
	if questions[0].Question != "controller question" {
		t.Fatalf("question = %q, want %q", questions[0].Question, "controller question")
	}

	detailRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+created.InterviewID, "")
	var detail vo.InterviewSessionVO
	decodeOKData(t, detailRec, &detail)
	if detail.Status != InterviewStatusReviewed {
		t.Fatalf("interview status = %q, want %q", detail.Status, InterviewStatusReviewed)
	}
}

func TestInterviewReviewControllerWithoutTranscriptReturnsError(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	rec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+session.InterviewID+"/review", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestListTranscriptSegmentsController(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")
	longContent := strings.Repeat("segment visible text ", 40) + "TAIL_MARKER_SHOULD_NOT_RETURN"

	for _, segment := range []TranscriptSegment{
		{
			SegmentID:          "segment-2",
			InterviewID:        session.InterviewID,
			TranscriptID:       "transcript-1",
			UserID:             "user_001",
			Sequence:           2,
			StartOffset:        301,
			EndOffset:          620,
			Content:            "second segment",
			CharCount:          len([]rune("second segment")),
			Summary:            "second summary",
			QuestionCandidates: `[{"local_sequence":1,"question":"q2"}]`,
			KeyEvidence:        `["e2"]`,
			UncertainParts:     `[]`,
			Status:             TranscriptSegmentStatusFailed,
			ErrorMessage:       "parse failed",
			CreatedAt:          100,
			UpdatedAt:          110,
		},
		{
			SegmentID:          "segment-1",
			InterviewID:        session.InterviewID,
			TranscriptID:       "transcript-1",
			UserID:             "user_001",
			Sequence:           1,
			StartOffset:        0,
			EndOffset:          len([]rune(longContent)),
			Content:            longContent,
			CharCount:          len([]rune(longContent)),
			Summary:            "first summary",
			SpeakerRoleNotes:   `[{"speaker_label":"面试官","normalized_role":"interviewer"}]`,
			QuestionCandidates: `[{"local_sequence":1,"question":"q1"}]`,
			KeyEvidence:        `["e1"]`,
			UncertainParts:     `["u1"]`,
			Status:             TranscriptSegmentStatusExtracted,
			CreatedAt:          90,
			UpdatedAt:          100,
		},
	} {
		if err := s.db.Create(&segment).Error; err != nil {
			t.Fatalf("seed segment: %v", err)
		}
	}

	rec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+session.InterviewID+"/transcript-segments", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "TAIL_MARKER_SHOULD_NOT_RETURN") {
		t.Fatalf("response returned full segment content: %s", rec.Body.String())
	}
	var segments []vo.TranscriptSegmentVO
	decodeOKData(t, rec, &segments)
	if len(segments) != 2 {
		t.Fatalf("segments length = %d, want 2", len(segments))
	}
	if segments[0].Sequence != 1 || segments[1].Sequence != 2 {
		t.Fatalf("segments not ordered by sequence: %#v", segments)
	}
	if segments[0].Status != TranscriptSegmentStatusExtracted || segments[1].ErrorMessage != "parse failed" {
		t.Fatalf("unexpected segment debug fields: %#v", segments)
	}
	if len([]rune(segments[0].ContentPreview)) > 300 {
		t.Fatalf("content_preview length = %d, want <= 300", len([]rune(segments[0].ContentPreview)))
	}
}

func TestSelectedContextDebugController(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "memory-redis",
		UserID:     "user_001",
		MemoryType: MemoryTypeUserWeakness,
		SubjectKey: "user:user_001",
		Content:    "Redis consistency answer needs clearer tradeoffs.",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
		CreatedAt:  100,
		UpdatedAt:  110,
	})
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "memory-other-user",
		UserID:     "user_002",
		MemoryType: MemoryTypeUserWeakness,
		SubjectKey: "user:user_002",
		Content:    "Should not appear.",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
	})
	seedPracticeState(t, s, "user_001", "Redis 缓存一致性", PracticeDimensionBackendKnowledge, 42)

	rec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+session.InterviewID+"/selected-context?user_id=user_001&target_round=second_round&current_task=mock_start", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var debug vo.SelectedContextDebugVO
	decodeOKData(t, rec, &debug)
	if debug.CurrentTask != MemorySelectorTaskMockStart || debug.TargetRound != "second_round" {
		t.Fatalf("debug request fields = %#v", debug)
	}
	if len(debug.SelectedMemoryItems) == 0 || debug.SelectedMemoryItems[0].MemoryID != "memory-redis" {
		t.Fatalf("selected memories = %#v, want memory-redis first", debug.SelectedMemoryItems)
	}
	if debug.SelectedMemoryItems[0].Score <= 0 || debug.SelectedMemoryItems[0].SelectionReason == "" {
		t.Fatalf("selected memory missing score/reason: %#v", debug.SelectedMemoryItems[0])
	}
	for _, item := range debug.SelectedMemoryItems {
		if item.MemoryID == "memory-other-user" {
			t.Fatalf("unrelated memory appeared: %#v", debug.SelectedMemoryItems)
		}
	}
	if len(debug.SelectedPracticeStates) == 0 || debug.SelectedPracticeStates[0].Score <= 0 || debug.SelectedPracticeStates[0].SelectionReason == "" {
		t.Fatalf("selected practice states missing score/reason: %#v", debug.SelectedPracticeStates)
	}
}
