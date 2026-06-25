//go:build real_step11

package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"agent-web-base/agent"
	ctxengine "agent-web-base/agent/context"
	"agent-web-base/shared"
	"agent-web-base/vo"
)

const step11InterviewID = "ae1deb1b-426b-4a03-8112-4342e7c5fc9f"

func TestStep11RealAudioInProcessValidation(t *testing.T) {
	s := newStep11RealServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	session, err := s.GetInterview(step11InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	userID := session.UserID

	transcript, err := s.GetInterviewTranscript(step11InterviewID)
	if err != nil {
		t.Fatalf("GetInterviewTranscript() error = %v", err)
	}
	t.Logf("STEP11_ENV db=agent-web-base.db interview_id=%s user_id=%s transcript_id=%s source_type=%s language=%s char_count=%d",
		step11InterviewID, userID, transcript.TranscriptID, transcript.SourceType, transcript.Language, len([]rune(transcript.Content)))
	t.Logf("STEP11_TRANSCRIPT_HEAD %s", step11ClipRunes(transcript.Content, 300))
	t.Logf("STEP11_TRANSCRIPT_TAIL %s", step11ClipTailRunes(transcript.Content, 300))
	t.Logf("STEP11_SEGMENTED_EXPECTED %v threshold_chars=%d", shouldUseSegmentedReview(transcript.Content), longTranscriptThresholdChars)

	report, err := s.TriggerInterviewReview(ctx, step11InterviewID)
	if err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v; status=%s raw=%s", err, report.Status, step11ClipRunes(report.RawAgentOutput, 800))
	}
	t.Logf("STEP11_REVIEW report_id=%s status=%s summary_non_empty=%v strengths=%d weaknesses=%d risks=%d prep=%d raw_non_empty=%v",
		report.ReportID, report.Status, strings.TrimSpace(report.OverallSummary) != "", len(report.Strengths), len(report.Weaknesses), len(report.FollowUpRisks), len(report.SuggestedPreparation), strings.TrimSpace(report.RawAgentOutput) != "")
	t.Logf("STEP11_REVIEW_SUMMARY %s", step11ClipRunes(report.OverallSummary, 300))

	var segments []TranscriptSegment
	if err := s.db.Where("interview_id = ?", step11InterviewID).Order("sequence asc").Find(&segments).Error; err != nil {
		t.Fatalf("query transcript_segments error = %v", err)
	}
	t.Logf("STEP11_SEGMENTS count=%d status_counts=%s", len(segments), step11SegmentStatusCounts(segments))

	questions, err := s.ListInterviewQuestions(step11InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	t.Logf("STEP11_QUESTIONS count=%d", len(questions))
	for i, q := range questions {
		if i >= 3 {
			break
		}
		t.Logf("STEP11_QUESTION_%d sequence=%d quality=%s tags=%v question=%s weakness=%s",
			i+1, q.Sequence, q.AnswerQuality, q.TopicTags, step11ClipRunes(q.Question, 180), step11ClipRunes(q.WeaknessSummary, 180))
	}

	candidates, err := s.GenerateMemoryCandidates(ctx, step11InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}
	t.Logf("STEP11_MEMORY_CANDIDATES count=%d", len(candidates))
	for i, c := range candidates {
		if i >= 3 {
			break
		}
		t.Logf("STEP11_MEMORY_CANDIDATE_%d id=%s type=%s subject=%s confidence=%s status=%s content=%s",
			i+1, c.CandidateID, c.MemoryType, c.SubjectKey, c.Confidence, c.Status, step11ClipRunes(c.Content, 180))
	}

	acceptedMemoryID := ""
	if candidate := step11FirstPendingCandidate(candidates); candidate.CandidateID != "" {
		item, err := s.AcceptMemoryCandidate(candidate.CandidateID)
		if err != nil {
			t.Fatalf("AcceptMemoryCandidate(%s) error = %v", candidate.CandidateID, err)
		}
		acceptedMemoryID = item.MemoryID
		t.Logf("STEP11_ACCEPTED_MEMORY memory_id=%s type=%s subject=%s status=%s", item.MemoryID, item.MemoryType, item.SubjectKey, item.Status)
	} else {
		t.Logf("STEP11_ACCEPTED_MEMORY skipped=no_pending_non_empty_candidate")
	}

	items, err := s.ListMemoryItems(userID)
	if err != nil {
		t.Fatalf("ListMemoryItems() error = %v", err)
	}
	t.Logf("STEP11_MEMORY_ITEMS active_count=%d accepted_memory_id=%s", len(items), acceptedMemoryID)

	plan, err := s.GenerateCoachingPlan(ctx, step11InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        userID,
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v; status=%s raw=%s", err, plan.Status, step11ClipRunes(plan.RawAgentOutput, 800))
	}
	t.Logf("STEP11_COACHING_PLAN plan_id=%s status=%s strategy_non_empty=%v focus_non_empty=%v raw_non_empty=%v strategy=%s",
		plan.PlanID, plan.Status, strings.TrimSpace(plan.OverallStrategy) != "", strings.TrimSpace(plan.FocusSummary) != "", strings.TrimSpace(plan.RawAgentOutput) != "", step11ClipRunes(plan.OverallStrategy, 240))

	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}
	t.Logf("STEP11_COACHING_TASKS count=%d", len(tasks))
	for i, task := range tasks {
		if i >= 3 {
			break
		}
		t.Logf("STEP11_COACHING_TASK_%d title=%s priority=%s status=%s", i+1, step11ClipRunes(task.Title, 160), task.Priority, task.Status)
	}

	mock, err := s.StartMockInterview(ctx, step11InterviewID, vo.StartMockInterviewReq{
		UserID:      userID,
		PlanID:      plan.PlanID,
		TargetRound: "second_round",
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v; status=%s raw=%s", err, mock.Status, step11ClipRunes(mock.RawAgentOutput, 800))
	}
	t.Logf("STEP11_MOCK_START mock_id=%s status=%s goal_non_empty=%v first_question=%s",
		mock.MockID, mock.Status, strings.TrimSpace(mock.OverallGoal) != "", step11ClipRunes(mock.FirstQuestion, 240))

	turn, err := s.SubmitMockTurn(ctx, mock.MockID, vo.SubmitMockTurnReq{
		Answer: "我会先概括项目背景，再说明关键技术方案、异常处理、状态管理和可观测性，最后结合业务结果说明收益。",
	})
	if err != nil {
		updatedMock, _ := s.GetMockInterview(mock.MockID)
		t.Fatalf("SubmitMockTurn() error = %v; raw=%s", err, step11ClipRunes(updatedMock.RawAgentOutput, 800))
	}
	t.Logf("STEP11_MOCK_TURN turn_id=%s index=%d score=%d tags=%v feedback=%s next_question=%s",
		turn.TurnID, turn.TurnIndex, turn.Score, turn.TopicTags, step11ClipRunes(turn.Feedback, 240), step11ClipRunes(turn.NextQuestion, 240))

	states, err := s.ListPracticeStates(userID, "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	t.Logf("STEP11_PRACTICE_STATES count=%d", len(states))
	for i, state := range states {
		if i >= 5 {
			break
		}
		t.Logf("STEP11_PRACTICE_STATE_%d topic=%s dimension=%s mastery=%d attempts=%d last_score=%d feedback=%s",
			i+1, state.Topic, state.Dimension, state.MasteryScore, state.AttemptCount, state.LastScore, step11ClipRunes(state.LastFeedback, 160))
	}

	completed, err := s.CompleteMockInterview(ctx, mock.MockID)
	if err != nil {
		updatedMock, _ := s.GetMockInterview(mock.MockID)
		t.Fatalf("CompleteMockInterview() error = %v; raw=%s", err, step11ClipRunes(updatedMock.RawAgentOutput, 800))
	}
	t.Logf("STEP11_MOCK_COMPLETE mock_id=%s status=%s final_summary=%s",
		completed.MockID, completed.Status, step11ClipRunes(completed.FinalSummary, 300))
}

func TestStep11ConstructedLongTranscriptSegmentedFlow(t *testing.T) {
	s := newStep11RealServer(t)

	transcriptBytes, err := os.ReadFile(filepath.Join("..", "testdata", "step11_long_interview_transcript.txt"))
	if err != nil {
		t.Fatalf("read constructed long transcript: %v", err)
	}
	transcript := strings.TrimSpace(string(transcriptBytes))
	charCount := len([]rune(transcript))
	if charCount <= longTranscriptThresholdChars {
		t.Fatalf("constructed transcript char_count=%d, want > %d", charCount, longTranscriptThresholdChars)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	userID := "u_real_segmented_step11"
	session, err := s.CreateInterview(vo.CreateInterviewReq{
		UserID:         userID,
		CompanyName:    "Step11 长文本分段验证公司",
		JobTitle:       "Backend Agent Engineer",
		InterviewRound: "first_round",
		InterviewType:  "technical",
	})
	if err != nil {
		t.Fatalf("CreateInterview() error = %v", err)
	}
	t.Logf("STEP11_LONG_CREATED interview_id=%s user_id=%s char_count=%d", session.InterviewID, userID, charCount)

	savedTranscript, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:     userID,
		Content:    transcript,
		SourceType: TranscriptSourceTypeManualText,
		Language:   "zh",
	})
	if err != nil {
		t.Fatalf("UpsertInterviewTranscript() error = %v", err)
	}
	t.Logf("STEP11_LONG_TRANSCRIPT transcript_id=%s source_type=%s char_count=%d segmented_expected=%v",
		savedTranscript.TranscriptID, savedTranscript.SourceType, charCount, shouldUseSegmentedReview(transcript))

	report, err := s.TriggerInterviewReview(ctx, session.InterviewID)
	if err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v; status=%s raw=%s", err, report.Status, step11ClipRunes(report.RawAgentOutput, 1000))
	}
	t.Logf("STEP11_LONG_REVIEW report_id=%s status=%s summary=%s strengths=%d weaknesses=%d risks=%d prep=%d",
		report.ReportID, report.Status, step11ClipRunes(report.OverallSummary, 300), len(report.Strengths), len(report.Weaknesses), len(report.FollowUpRisks), len(report.SuggestedPreparation))

	var segments []TranscriptSegment
	if err := s.db.Where("interview_id = ?", session.InterviewID).Order("sequence asc").Find(&segments).Error; err != nil {
		t.Fatalf("query transcript_segments error = %v", err)
	}
	if len(segments) < 2 {
		t.Fatalf("segments count=%d, want >=2", len(segments))
	}
	for _, segment := range segments {
		if segment.Status != TranscriptSegmentStatusExtracted {
			t.Fatalf("segment %d status=%s error=%s raw=%s", segment.Sequence, segment.Status, segment.ErrorMessage, step11ClipRunes(segment.RawAgentOutput, 500))
		}
	}
	t.Logf("STEP11_LONG_SEGMENTS count=%d status_counts=%s", len(segments), step11SegmentStatusCounts(segments))
	for i, segment := range segments {
		if i >= 5 {
			break
		}
		t.Logf("STEP11_LONG_SEGMENT_%d seq=%d offsets=%d-%d chars=%d status=%s summary=%s",
			i+1, segment.Sequence, segment.StartOffset, segment.EndOffset, segment.CharCount, segment.Status, step11ClipRunes(segment.Summary, 180))
	}

	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) == 0 {
		t.Fatalf("questions count=0; raw=%s", step11ClipRunes(report.RawAgentOutput, 1000))
	}
	t.Logf("STEP11_LONG_QUESTIONS count=%d", len(questions))
	for i, q := range questions {
		if i >= 3 {
			break
		}
		t.Logf("STEP11_LONG_QUESTION_%d sequence=%d quality=%s tags=%v question=%s weakness=%s",
			i+1, q.Sequence, q.AnswerQuality, q.TopicTags, step11ClipRunes(q.Question, 180), step11ClipRunes(q.WeaknessSummary, 180))
	}

	candidates, err := s.GenerateMemoryCandidates(ctx, session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}
	if len(candidates) == 0 {
		t.Fatalf("memory candidates count=0")
	}
	t.Logf("STEP11_LONG_MEMORY_CANDIDATES count=%d", len(candidates))
	for i, c := range candidates {
		if i >= 3 {
			break
		}
		t.Logf("STEP11_LONG_MEMORY_CANDIDATE_%d id=%s type=%s subject=%s confidence=%s status=%s content=%s",
			i+1, c.CandidateID, c.MemoryType, c.SubjectKey, c.Confidence, c.Status, step11ClipRunes(c.Content, 180))
	}

	candidate := step11FirstPendingCandidate(candidates)
	if candidate.CandidateID == "" {
		t.Fatalf("no pending non-empty memory candidate")
	}
	item, err := s.AcceptMemoryCandidate(candidate.CandidateID)
	if err != nil {
		t.Fatalf("AcceptMemoryCandidate(%s) error = %v", candidate.CandidateID, err)
	}
	t.Logf("STEP11_LONG_ACCEPTED_MEMORY memory_id=%s type=%s subject=%s status=%s", item.MemoryID, item.MemoryType, item.SubjectKey, item.Status)

	plan, err := s.GenerateCoachingPlan(ctx, session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        userID,
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v; status=%s raw=%s", err, plan.Status, step11ClipRunes(plan.RawAgentOutput, 1000))
	}
	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("coaching tasks count=0; raw=%s", step11ClipRunes(plan.RawAgentOutput, 1000))
	}
	t.Logf("STEP11_LONG_COACHING plan_id=%s status=%s tasks=%d strategy=%s", plan.PlanID, plan.Status, len(tasks), step11ClipRunes(plan.OverallStrategy, 240))

	mock, err := s.StartMockInterview(ctx, session.InterviewID, vo.StartMockInterviewReq{
		UserID:      userID,
		PlanID:      plan.PlanID,
		TargetRound: "second_round",
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v; status=%s raw=%s", err, mock.Status, step11ClipRunes(mock.RawAgentOutput, 1000))
	}
	t.Logf("STEP11_LONG_MOCK_START mock_id=%s status=%s first_question=%s", mock.MockID, mock.Status, step11ClipRunes(mock.FirstQuestion, 240))

	turn, err := s.SubmitMockTurn(ctx, mock.MockID, vo.SubmitMockTurnReq{
		Answer: "我会先说明这个项目从真实音频输入开始，通过异步转写、长文本分段复盘、候选记忆确认、二面计划和模拟面试形成闭环；同时补充事务、幂等、失败状态和可观测日志。",
	})
	if err != nil {
		updatedMock, _ := s.GetMockInterview(mock.MockID)
		t.Fatalf("SubmitMockTurn() error = %v; raw=%s", err, step11ClipRunes(updatedMock.RawAgentOutput, 1000))
	}
	t.Logf("STEP11_LONG_MOCK_TURN turn_id=%s index=%d score=%d tags=%v feedback=%s",
		turn.TurnID, turn.TurnIndex, turn.Score, turn.TopicTags, step11ClipRunes(turn.Feedback, 240))

	states, err := s.ListPracticeStates(userID, "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) == 0 {
		t.Fatalf("practice states count=0")
	}
	t.Logf("STEP11_LONG_PRACTICE_STATES count=%d", len(states))

	completed, err := s.CompleteMockInterview(ctx, mock.MockID)
	if err != nil {
		updatedMock, _ := s.GetMockInterview(mock.MockID)
		t.Fatalf("CompleteMockInterview() error = %v; raw=%s", err, step11ClipRunes(updatedMock.RawAgentOutput, 1000))
	}
	t.Logf("STEP11_LONG_MOCK_COMPLETE mock_id=%s status=%s final_summary=%s",
		completed.MockID, completed.Status, step11ClipRunes(completed.FinalSummary, 300))
}

func newStep11RealServer(t *testing.T) *Server {
	t.Helper()

	_ = godotenv.Load("../.env")
	appConf, err := shared.LoadAppConfig("../config.json")
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	model := appConf.LLMProviders.FrontModel
	if model.ApiKey == "" {
		t.Fatalf("OPENAI_API_KEY is empty")
	}
	if model.BaseURL == "" {
		t.Fatalf("OPENAI_BASE_URL is empty")
	}
	if model.Model == "" {
		t.Fatalf("OPENAI_MODEL is empty")
	}
	t.Logf("STEP11_MODEL base_url=%s model=%s", model.BaseURL, model.Model)

	db, err := InitDB(filepath.Join("..", "agent-web-base.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	registry, err := agent.NewAgentRegistry(agent.AgentTypeAssistant, agent.DefaultAgentProfiles(), func(profile agent.AgentProfile) agent.Runner {
		return agent.NewAgent(
			model,
			profile.SystemPrompt,
			agent.ToolConfirmConfig{},
			nil,
			nil,
			ctxengine.NewContextEngine(nil, nil),
		)
	})
	if err != nil {
		t.Fatalf("NewAgentRegistry() error = %v", err)
	}
	return NewServer(db, registry)
}

func step11FirstPendingCandidate(candidates []vo.MemoryCandidateVO) vo.MemoryCandidateVO {
	for _, candidate := range candidates {
		if candidate.Status == MemoryCandidateStatusPending && strings.TrimSpace(candidate.Content) != "" {
			return candidate
		}
	}
	return vo.MemoryCandidateVO{}
}

func step11SegmentStatusCounts(segments []TranscriptSegment) string {
	if len(segments) == 0 {
		return "{}"
	}
	counts := map[string]int{}
	for _, segment := range segments {
		counts[segment.Status]++
	}
	parts := make([]string, 0, len(counts))
	for status, count := range counts {
		parts = append(parts, status+":"+itoa(count))
	}
	return strings.Join(parts, ",")
}

func step11ClipRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func step11ClipTailRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[len(runes)-limit:])
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
