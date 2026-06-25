package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestGetInterviewDetailRequiresInterviewID(t *testing.T) {
	s := newTestServer(t)

	_, err := s.GetInterviewDetail(" ", "user_detail")
	if err == nil {
		t.Fatalf("GetInterviewDetail() error = nil, want interview_id required error")
	}
	if !strings.Contains(err.Error(), "interview_id is required") {
		t.Fatalf("GetInterviewDetail() error = %q, want interview_id is required", err.Error())
	}
}

func TestGetInterviewDetailAggregatesReadOnlyState(t *testing.T) {
	s := newTestServer(t)
	now := int64(1700000000)
	interviewID := "interview-detail-1"
	userID := "user_detail"

	seedInterviewDetailState(t, s, interviewID, userID, now)

	detail, err := s.GetInterviewDetail(interviewID, userID)
	if err != nil {
		t.Fatalf("GetInterviewDetail() error = %v", err)
	}

	if detail.Interview.InterviewID != interviewID {
		t.Fatalf("interview_id = %q, want %q", detail.Interview.InterviewID, interviewID)
	}
	if detail.Transcript == nil || detail.Transcript.TranscriptID != "transcript-detail-1" {
		t.Fatalf("transcript = %#v, want seeded transcript", detail.Transcript)
	}
	if len(detail.MediaFiles) != 1 || detail.MediaFiles[0].MediaID != "media-detail-1" {
		t.Fatalf("media files = %#v, want seeded media", detail.MediaFiles)
	}
	if len(detail.TranscriptionJobs) != 1 || detail.TranscriptionJobs[0].JobID != "job-detail-1" {
		t.Fatalf("transcription jobs = %#v, want seeded job", detail.TranscriptionJobs)
	}
	if detail.ReviewReport == nil || detail.ReviewReport.ReportID != "report-detail-1" {
		t.Fatalf("review report = %#v, want seeded report", detail.ReviewReport)
	}
	if len(detail.Questions) != 1 || detail.Questions[0].QuestionID != "question-detail-1" {
		t.Fatalf("questions = %#v, want seeded question", detail.Questions)
	}
	if len(detail.MemoryCandidates) != 1 || detail.MemoryCandidates[0].CandidateID != "candidate-detail-1" {
		t.Fatalf("memory candidates = %#v, want seeded candidate", detail.MemoryCandidates)
	}
	if detail.CoachingPlan == nil || detail.CoachingPlan.PlanID != "plan-detail-1" {
		t.Fatalf("coaching plan = %#v, want seeded plan", detail.CoachingPlan)
	}
	if len(detail.CoachingTasks) != 1 || detail.CoachingTasks[0].TaskID != "task-detail-1" {
		t.Fatalf("coaching tasks = %#v, want seeded task", detail.CoachingTasks)
	}
	if detail.LatestMockInterview == nil || detail.LatestMockInterview.MockID != "mock-detail-new" {
		t.Fatalf("latest mock interview = %#v, want newest seeded mock", detail.LatestMockInterview)
	}

	var itemCount int64
	if err := s.db.Model(&MemoryItem{}).Where("source_interview_id = ?", interviewID).Count(&itemCount).Error; err != nil {
		t.Fatalf("count memory items: %v", err)
	}
	if itemCount != 0 {
		t.Fatalf("memory item count = %d, want 0", itemCount)
	}
}

func TestInterviewDetailControllerRejectsUserMismatch(t *testing.T) {
	s := newTestServer(t)
	seedInterviewDetailState(t, s, "interview-detail-2", "user_detail", 1700000100)
	router := NewRouter(s)

	rec := performJSONRequest(router, http.MethodGet, "/api/interviews/interview-detail-2/detail?user_id=other_user", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestInterviewDetailControllerReturnsWrappedData(t *testing.T) {
	s := newTestServer(t)
	seedInterviewDetailState(t, s, "interview-detail-3", "user_detail", 1700000200)
	router := NewRouter(s)

	rec := performJSONRequest(router, http.MethodGet, "/api/interviews/interview-detail-3/detail?user_id=user_detail", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var detail struct {
		Interview struct {
			InterviewID string `json:"interview_id"`
		} `json:"interview"`
		Transcript *struct {
			TranscriptID string `json:"transcript_id"`
		} `json:"transcript"`
		ReviewReport *struct {
			ReportID string `json:"report_id"`
		} `json:"review_report"`
		Questions []struct {
			QuestionID string `json:"question_id"`
		} `json:"questions"`
		MemoryCandidates []struct {
			CandidateID string `json:"candidate_id"`
		} `json:"memory_candidates"`
		CoachingPlan *struct {
			PlanID string `json:"plan_id"`
		} `json:"coaching_plan"`
		CoachingTasks []struct {
			TaskID string `json:"task_id"`
		} `json:"coaching_tasks"`
		LatestMockInterview *struct {
			MockID string `json:"mock_id"`
		} `json:"latest_mock_interview"`
	}
	decodeOKData(t, rec, &detail)

	if detail.Interview.InterviewID != "interview-detail-3" ||
		detail.Transcript == nil ||
		detail.ReviewReport == nil ||
		len(detail.Questions) != 1 ||
		len(detail.MemoryCandidates) != 1 ||
		detail.CoachingPlan == nil ||
		len(detail.CoachingTasks) != 1 ||
		detail.LatestMockInterview == nil {
		t.Fatalf("detail response missing aggregate data: %#v", detail)
	}
}

func seedInterviewDetailState(t *testing.T, s *Server, interviewID string, userID string, now int64) {
	t.Helper()

	records := []any{
		&InterviewSession{
			InterviewID:    interviewID,
			UserID:         userID,
			CompanyName:    "DetailCo",
			JobTitle:       "Backend Engineer",
			InterviewRound: "first_round",
			InterviewType:  "technical",
			Status:         InterviewStatusReviewed,
			OccurredAt:     now - 100,
			CreatedAt:      now - 90,
			UpdatedAt:      now - 80,
		},
		&InterviewTranscript{
			TranscriptID: "transcript-detail-1",
			InterviewID:  interviewID,
			UserID:       userID,
			SourceType:   TranscriptSourceTypeManualText,
			Content:      "面试转写",
			Language:     DefaultTranscriptLanguage,
			CreatedAt:    now - 70,
			UpdatedAt:    now - 60,
		},
		&MediaFile{
			MediaID:          "media-detail-1",
			InterviewID:      interviewID,
			UserID:           userID,
			OriginalFilename: "interview.mp3",
			StoredFilename:   "media-detail-1.mp3",
			StoragePath:      "/tmp/media-detail-1.mp3",
			ContentType:      "audio/mpeg",
			MediaType:        MediaTypeAudio,
			FileExt:          ".mp3",
			SizeBytes:        123,
			Status:           MediaFileStatusTranscribed,
			CreatedAt:        now - 59,
			UpdatedAt:        now - 58,
		},
		&TranscriptionJob{
			JobID:          "job-detail-1",
			MediaID:        "media-detail-1",
			InterviewID:    interviewID,
			UserID:         userID,
			Status:         TranscriptionJobStatusSucceeded,
			InputMediaPath: "/tmp/media-detail-1.mp3",
			ASRProvider:    DefaultASRProvider,
			ASRModel:       DefaultASRModel,
			Language:       DefaultTranscriptLanguage,
			TranscriptID:   "transcript-detail-1",
			CreatedAt:      now - 57,
			UpdatedAt:      now - 56,
		},
		&InterviewReviewReport{
			ReportID:             "report-detail-1",
			InterviewID:          interviewID,
			UserID:               userID,
			OverallSummary:       "summary",
			Strengths:            marshalStringSlice([]string{"clear"}),
			Weaknesses:           marshalStringSlice([]string{"depth"}),
			FollowUpRisks:        marshalStringSlice([]string{"system design"}),
			SuggestedPreparation: marshalStringSlice([]string{"practice"}),
			Status:               InterviewReviewStatusGenerated,
			CreatedAt:            now - 50,
			UpdatedAt:            now - 49,
		},
		&InterviewQuestion{
			QuestionID:            "question-detail-1",
			InterviewID:           interviewID,
			UserID:                userID,
			Sequence:              1,
			Question:              "讲讲项目",
			Answer:                "项目回答",
			TopicTags:             marshalStringSlice([]string{"project"}),
			Difficulty:            "medium",
			AnswerQuality:         "ok",
			WeaknessSummary:       "needs detail",
			ImprovementSuggestion: "add metrics",
			CreatedAt:             now - 48,
			UpdatedAt:             now - 47,
		},
		&MemoryCandidate{
			CandidateID:   "candidate-detail-1",
			UserID:        userID,
			InterviewID:   interviewID,
			MemoryType:    MemoryTypeUserWeakness,
			SubjectKey:    "system_design",
			Content:       "系统设计需要加强",
			Evidence:      "review",
			Confidence:    MemoryConfidenceHigh,
			Status:        MemoryCandidateStatusPending,
			Source:        MemorySourceReviewReport,
			SourceRefType: MemorySourceReviewReport,
			SourceRefID:   "report-detail-1",
			CreatedAt:     now - 46,
			UpdatedAt:     now - 45,
		},
		&CoachingPlan{
			PlanID:          "plan-detail-1",
			UserID:          userID,
			InterviewID:     interviewID,
			TargetRound:     "second_round",
			RemainingDays:   3,
			CompanyName:     "DetailCo",
			JobTitle:        "Backend Engineer",
			OverallStrategy: "strategy",
			FocusSummary:    "focus",
			Status:          CoachingPlanStatusGenerated,
			CreatedAt:       now - 44,
			UpdatedAt:       now - 43,
		},
		&CoachingTask{
			TaskID:      "task-detail-1",
			PlanID:      "plan-detail-1",
			UserID:      userID,
			InterviewID: interviewID,
			Sequence:    1,
			DayIndex:    1,
			TaskType:    "practice",
			Title:       "练习系统设计",
			Description: "补齐方案",
			Priority:    CoachingTaskPriorityHigh,
			Status:      CoachingTaskStatusTodo,
			CreatedAt:   now - 42,
			UpdatedAt:   now - 41,
		},
		&MockInterview{
			MockID:        "mock-detail-old",
			UserID:        userID,
			InterviewID:   interviewID,
			PlanID:        "plan-detail-1",
			TargetRound:   "second_round",
			Status:        MockInterviewStatusCompleted,
			CurrentTurn:   1,
			OverallGoal:   "old",
			FirstQuestion: "old question",
			CreatedAt:     now - 40,
			UpdatedAt:     now - 39,
		},
		&MockInterview{
			MockID:        "mock-detail-new",
			UserID:        userID,
			InterviewID:   interviewID,
			PlanID:        "plan-detail-1",
			TargetRound:   "second_round",
			Status:        MockInterviewStatusWaitingAnswer,
			CurrentTurn:   2,
			OverallGoal:   "new",
			FirstQuestion: "new question",
			CreatedAt:     now - 30,
			UpdatedAt:     now - 29,
		},
	}

	for _, record := range records {
		if err := s.db.Create(record).Error; err != nil {
			t.Fatalf("seed %T: %v", record, err)
		}
	}
}
