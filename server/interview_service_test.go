package server

import (
	"testing"

	"agent-web-base/vo"
)

func TestCreateGetAndListInterviews(t *testing.T) {
	s := newTestServer(t)

	first, err := s.CreateInterview(vo.CreateInterviewReq{
		UserID:         "user_001",
		CompanyName:    "Acme",
		JobTitle:       "Backend Engineer",
		InterviewRound: "first_round",
		InterviewType:  "technical",
		OccurredAt:     1710000000,
	})
	if err != nil {
		t.Fatalf("CreateInterview() error = %v", err)
	}
	if first.Status != InterviewStatusCreated {
		t.Fatalf("status = %q, want %q", first.Status, InterviewStatusCreated)
	}
	if first.CreatedAt == 0 || first.UpdatedAt == 0 {
		t.Fatalf("timestamps were not populated: created_at=%d updated_at=%d", first.CreatedAt, first.UpdatedAt)
	}

	if _, err := s.CreateInterview(vo.CreateInterviewReq{
		UserID:      "user_002",
		CompanyName: "Other",
	}); err != nil {
		t.Fatalf("CreateInterview() second error = %v", err)
	}

	got, err := s.GetInterview(first.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if got.InterviewID != first.InterviewID {
		t.Fatalf("interview_id = %q, want %q", got.InterviewID, first.InterviewID)
	}
	if got.CompanyName != "Acme" {
		t.Fatalf("company_name = %q, want %q", got.CompanyName, "Acme")
	}

	list, err := s.ListInterviews("user_001")
	if err != nil {
		t.Fatalf("ListInterviews() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list length = %d, want 1", len(list))
	}
	if list[0].InterviewID != first.InterviewID {
		t.Fatalf("listed interview_id = %q, want %q", list[0].InterviewID, first.InterviewID)
	}
}

func TestUpsertInterviewTranscriptCreatesTranscriptAndUpdatesSession(t *testing.T) {
	s := newTestServer(t)
	session := createTestInterview(t, s, "user_001")

	transcript, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:  "user_001",
		Content: "面试官：请介绍项目。候选人：...",
	})
	if err != nil {
		t.Fatalf("UpsertInterviewTranscript() error = %v", err)
	}
	if transcript.InterviewID != session.InterviewID {
		t.Fatalf("interview_id = %q, want %q", transcript.InterviewID, session.InterviewID)
	}
	if transcript.SourceType != TranscriptSourceTypeManualText {
		t.Fatalf("source_type = %q, want %q", transcript.SourceType, TranscriptSourceTypeManualText)
	}
	if transcript.Language != DefaultTranscriptLanguage {
		t.Fatalf("language = %q, want %q", transcript.Language, DefaultTranscriptLanguage)
	}

	updated, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if updated.Status != InterviewStatusReadyForReview {
		t.Fatalf("status = %q, want %q", updated.Status, InterviewStatusReadyForReview)
	}
}

func TestUpsertInterviewTranscriptUpdatesExistingTranscript(t *testing.T) {
	s := newTestServer(t)
	session := createTestInterview(t, s, "user_001")

	first, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:   "user_001",
		Content:  "first content",
		Language: "en",
	})
	if err != nil {
		t.Fatalf("first UpsertInterviewTranscript() error = %v", err)
	}
	second, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:   "user_001",
		Content:  "second content",
		Language: "zh",
	})
	if err != nil {
		t.Fatalf("second UpsertInterviewTranscript() error = %v", err)
	}

	if second.TranscriptID != first.TranscriptID {
		t.Fatalf("transcript_id = %q, want %q", second.TranscriptID, first.TranscriptID)
	}
	if second.Content != "second content" {
		t.Fatalf("content = %q, want %q", second.Content, "second content")
	}

	var count int64
	if err := s.db.Model(&InterviewTranscript{}).
		Where("interview_id = ?", session.InterviewID).
		Count(&count).Error; err != nil {
		t.Fatalf("count transcripts: %v", err)
	}
	if count != 1 {
		t.Fatalf("transcript count = %d, want 1", count)
	}
}

func TestUpsertInterviewTranscriptRejectsMismatchedUser(t *testing.T) {
	s := newTestServer(t)
	session := createTestInterview(t, s, "user_001")

	err := func() error {
		_, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
			UserID:  "user_002",
			Content: "not allowed",
		})
		return err
	}()
	if err == nil {
		t.Fatalf("UpsertInterviewTranscript() error = nil, want error")
	}

	unchanged, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if unchanged.Status != InterviewStatusCreated {
		t.Fatalf("status = %q, want %q", unchanged.Status, InterviewStatusCreated)
	}
}

func TestGetInterviewTranscriptMissingReturnsError(t *testing.T) {
	s := newTestServer(t)
	session := createTestInterview(t, s, "user_001")

	if _, err := s.GetInterviewTranscript(session.InterviewID); err == nil {
		t.Fatalf("GetInterviewTranscript() error = nil, want error")
	}
}

func createTestInterview(t *testing.T, s *Server, userID string) vo.InterviewSessionVO {
	t.Helper()

	session, err := s.CreateInterview(vo.CreateInterviewReq{
		UserID:         userID,
		CompanyName:    "Acme",
		JobTitle:       "Backend Engineer",
		InterviewRound: "first_round",
		InterviewType:  "technical",
	})
	if err != nil {
		t.Fatalf("CreateInterview() error = %v", err)
	}
	return session
}
