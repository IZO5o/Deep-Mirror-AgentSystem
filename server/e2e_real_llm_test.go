//go:build real_llm

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"agent-web-base/agent"
	ctxengine "agent-web-base/agent/context"
	"agent-web-base/shared"
	"agent-web-base/vo"
)

func TestRealLLMFullInterviewFlow(t *testing.T) {
	s := newRealLLME2EServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	userID := "u_real_e2e"
	transcript := `面试官：请介绍一下你做的 Agent 项目。
候选人：我做了一个面试复盘 Agent，可以根据面试转写文本生成结构化复盘报告，并根据长期记忆辅助二面准备。
面试官：如果 Agent 调用工具失败，你怎么处理？
候选人：我会进行重试，但目前还没有完整设计幂等、审计日志和人工确认。
面试官：Redis 缓存和数据库不一致怎么办？
候选人：我知道可以先更新数据库再删除缓存，也可以做延迟双删，但细节还不够熟悉。
面试官：你这个项目里的长期记忆怎么避免污染？
候选人：真实面试复盘生成候选记忆，需要用户确认后才写入长期记忆；模拟面试只更新练习状态。`

	session, err := s.CreateInterview(vo.CreateInterviewReq{
		UserID:         userID,
		CompanyName:    "字节跳动",
		JobTitle:       "后端开发实习",
		InterviewRound: "first_round",
		InterviewType:  "technical",
	})
	if err != nil {
		t.Fatalf("CreateInterview() error = %v", err)
	}
	if session.Status != InterviewStatusCreated {
		t.Fatalf("created status = %q, want %q", session.Status, InterviewStatusCreated)
	}

	if _, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:     userID,
		Content:    transcript,
		SourceType: TranscriptSourceTypeManualText,
		Language:   "zh",
	}); err != nil {
		t.Fatalf("UpsertInterviewTranscript() error = %v", err)
	}
	afterTranscript, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() after transcript error = %v", err)
	}
	if afterTranscript.Status != InterviewStatusReadyForReview {
		t.Fatalf("transcript status = %q, want %q", afterTranscript.Status, InterviewStatusReadyForReview)
	}

	report, err := s.TriggerInterviewReview(ctx, session.InterviewID)
	if err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v; raw=%s", err, report.RawAgentOutput)
	}
	afterReview, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() after review error = %v", err)
	}
	if afterReview.Status != InterviewStatusReviewed {
		t.Fatalf("reviewed status = %q, want %q", afterReview.Status, InterviewStatusReviewed)
	}
	if report.Status != InterviewReviewStatusGenerated {
		t.Fatalf("review status = %q, want %q; raw=%s", report.Status, InterviewReviewStatusGenerated, report.RawAgentOutput)
	}
	if report.OverallSummary == "" {
		t.Fatalf("overall_summary is empty; raw=%s", report.RawAgentOutput)
	}
	questions, err := s.ListInterviewQuestions(session.InterviewID)
	if err != nil {
		t.Fatalf("ListInterviewQuestions() error = %v", err)
	}
	if len(questions) < 2 {
		t.Fatalf("questions length = %d, want >= 2; raw=%s", len(questions), report.RawAgentOutput)
	}

	candidates, err := s.GenerateMemoryCandidates(ctx, session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}
	if len(candidates) == 0 {
		t.Fatalf("memory candidates length = 0, want >= 1")
	}
	if !hasPendingMemoryCandidate(candidates) {
		t.Fatalf("memory candidates have no pending item: %#v", candidates)
	}
	firstCandidate := firstNonEmptyMemoryCandidate(candidates)
	if firstCandidate.CandidateID == "" {
		t.Fatalf("memory candidates have no non-empty content item: %#v", candidates)
	}

	item, err := s.AcceptMemoryCandidate(firstCandidate.CandidateID)
	if err != nil {
		t.Fatalf("AcceptMemoryCandidate() error = %v", err)
	}
	if item.Status != MemoryItemStatusActive {
		t.Fatalf("memory item status = %q, want %q", item.Status, MemoryItemStatusActive)
	}
	itemsAfterFirstAccept, err := s.ListMemoryItems(userID)
	if err != nil {
		t.Fatalf("ListMemoryItems() after first accept error = %v", err)
	}
	if len(itemsAfterFirstAccept) == 0 {
		t.Fatalf("memory_items length = 0, want >= 1")
	}
	if _, err := s.AcceptMemoryCandidate(firstCandidate.CandidateID); err != nil {
		t.Fatalf("second AcceptMemoryCandidate() error = %v", err)
	}
	itemsAfterSecondAccept, err := s.ListMemoryItems(userID)
	if err != nil {
		t.Fatalf("ListMemoryItems() after second accept error = %v", err)
	}
	if len(itemsAfterSecondAccept) != len(itemsAfterFirstAccept) {
		t.Fatalf("memory_items count changed after repeated accept from %d to %d", len(itemsAfterFirstAccept), len(itemsAfterSecondAccept))
	}

	plan, err := s.GenerateCoachingPlan(ctx, session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        userID,
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v; raw=%s", err, plan.RawAgentOutput)
	}
	if plan.Status != CoachingPlanStatusGenerated {
		t.Fatalf("coaching plan status = %q, want %q; raw=%s", plan.Status, CoachingPlanStatusGenerated, plan.RawAgentOutput)
	}
	if plan.OverallStrategy == "" {
		t.Fatalf("overall_strategy is empty; raw=%s", plan.RawAgentOutput)
	}
	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("coaching tasks length = 0, want >= 1; raw=%s", plan.RawAgentOutput)
	}

	mock, err := s.StartMockInterview(ctx, session.InterviewID, vo.StartMockInterviewReq{
		UserID:      userID,
		PlanID:      plan.PlanID,
		TargetRound: "second_round",
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v; raw=%s", err, mock.RawAgentOutput)
	}
	if mock.Status != MockInterviewStatusInProgress {
		t.Fatalf("mock status = %q, want %q; raw=%s", mock.Status, MockInterviewStatusInProgress, mock.RawAgentOutput)
	}
	if mock.FirstQuestion == "" {
		t.Fatalf("first_question is empty; raw=%s", mock.RawAgentOutput)
	}

	turn, err := s.SubmitMockTurn(ctx, mock.MockID, vo.SubmitMockTurnReq{
		Answer: "我会先说明项目目标和核心链路，然后补充工具调用失败时的重试、幂等保护、审计日志和人工确认机制。",
	})
	if err != nil {
		updatedMock, _ := s.GetMockInterview(mock.MockID)
		t.Fatalf("SubmitMockTurn() error = %v; raw=%s", err, updatedMock.RawAgentOutput)
	}
	if turn.Feedback == "" {
		t.Fatalf("turn feedback is empty; raw=%s", turn.RawAgentOutput)
	}
	if turn.NextQuestion == "" {
		t.Fatalf("turn next_question is empty; raw=%s", turn.RawAgentOutput)
	}
	if len(turn.TopicTags) == 0 && turn.Score <= 0 {
		t.Fatalf("turn has no topic_tags and non-positive score: score=%d raw=%s", turn.Score, turn.RawAgentOutput)
	}

	practiceStates, err := s.ListPracticeStates(userID, "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(practiceStates) == 0 {
		t.Fatalf("practice_states length = 0, want >= 1")
	}
	if !hasPracticeProgress(practiceStates) {
		t.Fatalf("practice_states have no progress: %#v", practiceStates)
	}

	completed, err := s.CompleteMockInterview(ctx, mock.MockID)
	if err != nil {
		updatedMock, _ := s.GetMockInterview(mock.MockID)
		t.Fatalf("CompleteMockInterview() error = %v; raw=%s", err, updatedMock.RawAgentOutput)
	}
	if completed.Status != MockInterviewStatusCompleted {
		t.Fatalf("completed status = %q, want %q; raw=%s", completed.Status, MockInterviewStatusCompleted, completed.RawAgentOutput)
	}
	if completed.FinalSummary == "" {
		t.Fatalf("final_summary is empty; raw=%s", completed.RawAgentOutput)
	}
}

func newRealLLME2EServer(t *testing.T) *Server {
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
	t.Logf("real LLM model=%s base_url=%s", model.Model, model.BaseURL)

	db, err := InitDB(filepath.Join(t.TempDir(), "real-e2e.db"))
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
	if _, ok := os.LookupEnv("OPENAI_API_KEY"); !ok {
		t.Fatalf("OPENAI_API_KEY disappeared after config load")
	}
	return NewServer(db, registry)
}

func hasPendingMemoryCandidate(candidates []vo.MemoryCandidateVO) bool {
	for _, candidate := range candidates {
		if candidate.Status == MemoryCandidateStatusPending {
			return true
		}
	}
	return false
}

func firstNonEmptyMemoryCandidate(candidates []vo.MemoryCandidateVO) vo.MemoryCandidateVO {
	for _, candidate := range candidates {
		if candidate.CandidateID != "" && candidate.Content != "" {
			return candidate
		}
	}
	return vo.MemoryCandidateVO{}
}

func hasPracticeProgress(states []vo.PracticeStateVO) bool {
	for _, state := range states {
		if state.MasteryScore > 0 || state.AttemptCount > 0 {
			return true
		}
	}
	return false
}
