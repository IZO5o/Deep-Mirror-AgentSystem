package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestTriggerInterviewReview_Success(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createReviewedTestInterview(t, s, "user_001")
	runners[agent.AgentTypeReview].taskResponse = sampleReviewJSON("summary one", "question one")

	report, err := s.TriggerInterviewReview(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v", err)
	}
	if report.Status != InterviewReviewStatusGenerated {
		t.Fatalf("report status = %q, want %q", report.Status, InterviewReviewStatusGenerated)
	}
	if report.OverallSummary != "summary one" {
		t.Fatalf("overall_summary = %q, want %q", report.OverallSummary, "summary one")
	}
	if len(report.Strengths) != 1 || report.Strengths[0] != "clear project context" {
		t.Fatalf("strengths = %#v, want one expected strength", report.Strengths)
	}
	if runners[agent.AgentTypeReview].taskCalls != 1 {
		t.Fatalf("review task calls = %d, want 1", runners[agent.AgentTypeReview].taskCalls)
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("questions length = %d, want 1", len(questions))
	}
	if questions[0].Question != "question one" {
		t.Fatalf("question = %q, want %q", questions[0].Question, "question one")
	}
	if len(questions[0].TopicTags) != 2 {
		t.Fatalf("topic_tags = %#v, want 2 tags", questions[0].TopicTags)
	}

	updated, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if updated.Status != InterviewStatusReviewed {
		t.Fatalf("session status = %q, want %q", updated.Status, InterviewStatusReviewed)
	}
}

func TestTriggerInterviewReview_RequiresTranscript(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createTestInterview(t, s, "user_001")

	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err == nil {
		t.Fatalf("TriggerInterviewReview() error = nil, want error")
	}
	if runners[agent.AgentTypeReview].taskCalls != 0 {
		t.Fatalf("review task calls = %d, want 0", runners[agent.AgentTypeReview].taskCalls)
	}
}

func TestTriggerInterviewReview_ReplacesQuestionsOnRepeat(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createReviewedTestInterview(t, s, "user_001")

	runners[agent.AgentTypeReview].taskResponse = sampleReviewJSON("summary one", "question one")
	first, err := s.TriggerInterviewReview(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("first TriggerInterviewReview() error = %v", err)
	}

	runners[agent.AgentTypeReview].taskResponse = sampleReviewJSON("summary two", "question two")
	second, err := s.TriggerInterviewReview(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("second TriggerInterviewReview() error = %v", err)
	}
	if second.ReportID != first.ReportID {
		t.Fatalf("report_id = %q, want existing %q", second.ReportID, first.ReportID)
	}
	if second.OverallSummary != "summary two" {
		t.Fatalf("overall_summary = %q, want %q", second.OverallSummary, "summary two")
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("questions length = %d, want 1", len(questions))
	}
	if questions[0].Question != "question two" {
		t.Fatalf("question = %q, want %q", questions[0].Question, "question two")
	}
}

func TestTriggerInterviewReview_ParseFailureSavesRawOutput(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createReviewedTestInterview(t, s, "user_001")
	runners[agent.AgentTypeReview].taskResponse = "not json"

	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err == nil {
		t.Fatalf("TriggerInterviewReview() error = nil, want error")
	}

	report, err := s.GetInterviewReview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterviewReview() error = %v", err)
	}
	if report.Status != InterviewReviewStatusFailed {
		t.Fatalf("report status = %q, want %q", report.Status, InterviewReviewStatusFailed)
	}
	if report.RawAgentOutput != "not json" {
		t.Fatalf("raw_agent_output = %q, want %q", report.RawAgentOutput, "not json")
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) != 0 {
		t.Fatalf("questions length = %d, want 0", len(questions))
	}
}

func TestTriggerInterviewReview_AgentFailureSavesFailedReport(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createReviewedTestInterview(t, s, "user_001")
	runners[agent.AgentTypeReview].taskResponse = "partial output"
	runners[agent.AgentTypeReview].taskErr = errors.New("model unavailable")

	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err == nil {
		t.Fatalf("TriggerInterviewReview() error = nil, want error")
	}

	report, err := s.GetInterviewReview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterviewReview() error = %v", err)
	}
	if report.Status != InterviewReviewStatusFailed {
		t.Fatalf("report status = %q, want %q", report.Status, InterviewReviewStatusFailed)
	}
	if report.RawAgentOutput != "partial output" {
		t.Fatalf("raw_agent_output = %q, want %q", report.RawAgentOutput, "partial output")
	}
}

func TestTriggerInterviewReview_LongTranscriptUsesSegmentedReview(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, transcript := createLongTranscriptInterview(t, s, "user_001")
	segments := splitTranscriptIntoSegments(transcript)
	runners[agent.AgentTypeReview].taskResponses = append(segmentReviewResponses(len(segments)), sampleReviewJSON("merged summary", "merged final question"))

	report, err := s.TriggerInterviewReview(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v", err)
	}
	if report.Status != InterviewReviewStatusGenerated {
		t.Fatalf("report status = %q, want %q", report.Status, InterviewReviewStatusGenerated)
	}
	if runners[agent.AgentTypeReview].taskCalls != len(segments)+1 {
		t.Fatalf("review task calls = %d, want %d", runners[agent.AgentTypeReview].taskCalls, len(segments)+1)
	}
	for i, query := range runners[agent.AgentTypeReview].taskQueries {
		if strings.Contains(query, transcript.Content) {
			t.Fatalf("task query %d unexpectedly contains full transcript", i)
		}
	}

	var storedSegments []TranscriptSegment
	if err := s.db.Where("interview_id = ?", session.InterviewID).
		Order("sequence asc").
		Find(&storedSegments).Error; err != nil {
		t.Fatalf("load transcript segments: %v", err)
	}
	if len(storedSegments) != len(segments) {
		t.Fatalf("stored segments length = %d, want %d", len(storedSegments), len(segments))
	}
	for _, segment := range storedSegments {
		if segment.Status != TranscriptSegmentStatusExtracted {
			t.Fatalf("segment %d status = %q, want %q", segment.Sequence, segment.Status, TranscriptSegmentStatusExtracted)
		}
		if segment.Summary == "" {
			t.Fatalf("segment %d summary is empty", segment.Sequence)
		}
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("questions length = %d, want 1", len(questions))
	}
	if questions[0].Question != "merged final question" {
		t.Fatalf("question = %q, want final merge question", questions[0].Question)
	}

	updated, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if updated.Status != InterviewStatusReviewed {
		t.Fatalf("session status = %q, want %q", updated.Status, InterviewStatusReviewed)
	}
}

func TestExtractSegmentReview_ParseFailureRetriesWithStrictPrompt(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, transcript := createLongTranscriptInterview(t, s, "user_001")
	segment := splitTranscriptIntoSegments(transcript)[0]
	if err := s.db.Create(&segment).Error; err != nil {
		t.Fatalf("seed segment: %v", err)
	}
	runner := runners[agent.AgentTypeReview]
	runner.taskResponses = []string{"not json", segmentReviewResponses(1)[0]}

	saved, err := s.extractSegmentReview(context.Background(), runner, InterviewSession{
		InterviewID:    session.InterviewID,
		UserID:         session.UserID,
		CompanyName:    session.CompanyName,
		JobTitle:       session.JobTitle,
		InterviewRound: session.InterviewRound,
		InterviewType:  session.InterviewType,
		Status:         session.Status,
		OccurredAt:     session.OccurredAt,
		CreatedAt:      session.CreatedAt,
		UpdatedAt:      session.UpdatedAt,
	}, transcript, segment)
	if err != nil {
		t.Fatalf("extractSegmentReview() error = %v", err)
	}
	if runner.taskCalls != 2 {
		t.Fatalf("task calls = %d, want 2", runner.taskCalls)
	}
	if saved.Status != TranscriptSegmentStatusExtracted {
		t.Fatalf("segment status = %q, want %q", saved.Status, TranscriptSegmentStatusExtracted)
	}
	if saved.Summary != "segment 1 summary" {
		t.Fatalf("summary = %q, want retry summary", saved.Summary)
	}
	retryPrompt := runner.taskQueries[1]
	for _, want := range []string{"Retry segment extraction", "compact STRICT JSON", "question_candidates must contain at most 3 items"} {
		if !strings.Contains(retryPrompt, want) {
			t.Fatalf("retry prompt missing %q", want)
		}
	}
}

func TestExtractSegmentReview_ParseFailureAfterRetryMarksFailed(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createLongTranscriptInterview(t, s, "user_001")
	runners[agent.AgentTypeReview].taskResponses = []string{"not json", "still not json"}

	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err == nil {
		t.Fatalf("TriggerInterviewReview() error = nil, want error")
	}
	if runners[agent.AgentTypeReview].taskCalls != 2 {
		t.Fatalf("task calls = %d, want 2", runners[agent.AgentTypeReview].taskCalls)
	}

	var failedSegments []TranscriptSegment
	if err := s.db.Where("interview_id = ? AND status = ?", session.InterviewID, TranscriptSegmentStatusFailed).
		Find(&failedSegments).Error; err != nil {
		t.Fatalf("load failed segments: %v", err)
	}
	if len(failedSegments) != 1 {
		t.Fatalf("failed segments length = %d, want 1", len(failedSegments))
	}
	if failedSegments[0].RawAgentOutput != "still not json" {
		t.Fatalf("raw_agent_output = %q, want retry output", failedSegments[0].RawAgentOutput)
	}
	if failedSegments[0].ErrorMessage == "" {
		t.Fatalf("error_message is empty")
	}

	report, err := s.GetInterviewReview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterviewReview() error = %v", err)
	}
	if report.Status != InterviewReviewStatusFailed {
		t.Fatalf("report status = %q, want %q", report.Status, InterviewReviewStatusFailed)
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) != 0 {
		t.Fatalf("questions length = %d, want 0", len(questions))
	}
}

func TestTriggerInterviewReview_LongTranscriptSegmentFailureSavesFailedReport(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createLongTranscriptInterview(t, s, "user_001")
	runners[agent.AgentTypeReview].taskResponse = "not json"

	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err == nil {
		t.Fatalf("TriggerInterviewReview() error = nil, want error")
	}

	report, err := s.GetInterviewReview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterviewReview() error = %v", err)
	}
	if report.Status != InterviewReviewStatusFailed {
		t.Fatalf("report status = %q, want %q", report.Status, InterviewReviewStatusFailed)
	}

	var failedSegments []TranscriptSegment
	if err := s.db.Where("interview_id = ? AND status = ?", session.InterviewID, TranscriptSegmentStatusFailed).
		Find(&failedSegments).Error; err != nil {
		t.Fatalf("load failed segments: %v", err)
	}
	if len(failedSegments) != 1 {
		t.Fatalf("failed segments length = %d, want 1", len(failedSegments))
	}
	if failedSegments[0].RawAgentOutput != "not json" {
		t.Fatalf("failed segment raw output = %q, want not json", failedSegments[0].RawAgentOutput)
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) != 0 {
		t.Fatalf("questions length = %d, want 0", len(questions))
	}

	updated, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if updated.Status == InterviewStatusReviewed {
		t.Fatalf("session status = %q, should not be reviewed after segment failure", updated.Status)
	}
}

func TestTriggerInterviewReview_RepeatLongTranscriptRebuildsSegments(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, transcript := createLongTranscriptInterview(t, s, "user_001")
	segments := splitTranscriptIntoSegments(transcript)

	runners[agent.AgentTypeReview].taskResponses = append(segmentReviewResponses(len(segments)), sampleReviewJSON("first summary", "first final question"))
	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err != nil {
		t.Fatalf("first TriggerInterviewReview() error = %v", err)
	}

	var firstSegments []TranscriptSegment
	if err := s.db.Where("interview_id = ?", session.InterviewID).
		Order("sequence asc").
		Find(&firstSegments).Error; err != nil {
		t.Fatalf("load first segments: %v", err)
	}
	if len(firstSegments) != len(segments) {
		t.Fatalf("first segments length = %d, want %d", len(firstSegments), len(segments))
	}

	runners[agent.AgentTypeReview].taskResponses = append(segmentReviewResponses(len(segments)), sampleReviewJSON("second summary", "second final question"))
	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err != nil {
		t.Fatalf("second TriggerInterviewReview() error = %v", err)
	}

	var secondSegments []TranscriptSegment
	if err := s.db.Where("interview_id = ?", session.InterviewID).
		Order("sequence asc").
		Find(&secondSegments).Error; err != nil {
		t.Fatalf("load second segments: %v", err)
	}
	if len(secondSegments) != len(firstSegments) {
		t.Fatalf("second segments length = %d, want %d", len(secondSegments), len(firstSegments))
	}
	if len(secondSegments) > 0 && secondSegments[0].SegmentID == firstSegments[0].SegmentID {
		t.Fatalf("segment_id was not rebuilt on repeat review")
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("questions length = %d, want 1", len(questions))
	}
	if questions[0].Question != "second final question" {
		t.Fatalf("question = %q, want second final question", questions[0].Question)
	}
}

func createReviewedTestInterview(t *testing.T, s *Server, userID string) vo.InterviewSessionVO {
	t.Helper()

	session := createTestInterview(t, s, userID)
	if _, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:  userID,
		Content: "面试官：请介绍你的 Go 项目。候选人：我做了一个多 Agent 面试复盘系统。",
	}); err != nil {
		t.Fatalf("UpsertInterviewTranscript() error = %v", err)
	}
	return session
}

func createLongTranscriptInterview(t *testing.T, s *Server, userID string) (vo.InterviewSessionVO, InterviewTranscript) {
	t.Helper()

	session := createTestInterview(t, s, userID)
	if _, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:  userID,
		Content: buildLongTranscriptContent(),
	}); err != nil {
		t.Fatalf("UpsertInterviewTranscript() error = %v", err)
	}

	var transcript InterviewTranscript
	if err := s.db.First(&transcript, "interview_id = ?", session.InterviewID).Error; err != nil {
		t.Fatalf("load transcript: %v", err)
	}
	return session, transcript
}

func segmentReviewResponses(count int) []string {
	responses := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		responses = append(responses, fmt.Sprintf(`{
  "segment_summary": "segment %d summary",
  "speaker_role_notes": [
    {
      "speaker_label": "面试官",
      "normalized_role": "interviewer",
      "reason": "explicit speaker label",
      "evidence_text": "面试官：请解释项目"
    }
  ],
  "question_candidates": [
    {
      "local_sequence": 1,
      "question": "segment %d question",
      "answer": "segment %d answer",
      "topic_tags": ["Go", "Agent"],
      "difficulty": "medium",
      "answer_quality": "medium",
      "weakness_summary": "segment %d weakness",
      "improvement_suggestion": "segment %d suggestion",
      "evidence_text": "segment %d evidence"
    }
  ],
  "key_evidence": ["segment %d evidence"],
  "uncertain_parts": []
}`, i, i, i, i, i, i, i))
	}
	return responses
}

func sampleReviewJSON(summary string, question string) string {
	return `{
  "overall_summary": "` + summary + `",
  "strengths": ["clear project context"],
  "weaknesses": ["missing tradeoff discussion"],
  "follow_up_risks": ["Go concurrency details"],
  "suggested_preparation": ["prepare channel and goroutine examples"],
  "questions": [
    {
      "sequence": 1,
      "question": "` + question + `",
      "answer": "I built a multi-agent interview review system.",
      "topic_tags": ["Go concurrency", "project experience"],
      "difficulty": "medium",
      "answer_quality": "weak",
      "weakness_summary": "The answer lacks implementation details.",
      "improvement_suggestion": "Explain architecture, tradeoffs, and metrics.",
      "evidence_text": "我做了一个多 Agent 面试复盘系统"
    }
  ]
}`
}
