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

func TestRealLLMGenerateCoachingPlan(t *testing.T) {
	_ = godotenv.Load("../.env")
	appConf, err := shared.LoadAppConfig("../config.json")
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if appConf.LLMProviders.FrontModel.ApiKey == "" {
		t.Fatalf("OPENAI_API_KEY is empty")
	}

	db, err := InitDB(filepath.Join(t.TempDir(), "real-llm.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	registry, err := agent.NewAgentRegistry(agent.AgentTypeAssistant, agent.DefaultAgentProfiles(), func(profile agent.AgentProfile) agent.Runner {
		return agent.NewAgent(
			appConf.LLMProviders.FrontModel,
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

	s := NewServer(db, registry)
	now := time.Now().Unix()
	userID := "real_llm_step5_user"
	interviewID := "real-llm-interview"
	memoryID := "real-llm-memory"

	if err := db.Create(&InterviewSession{
		InterviewID:    interviewID,
		UserID:         userID,
		CompanyName:    "Acme AI",
		JobTitle:       "Backend Engineer",
		InterviewRound: "first_round",
		InterviewType:  "technical",
		Status:         InterviewStatusReviewed,
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("seed interview: %v", err)
	}
	if err := db.Create(&InterviewReviewReport{
		ReportID:             "real-llm-report",
		InterviewID:          interviewID,
		UserID:               userID,
		OverallSummary:       "候选人项目表达清晰，但 Redis 缓存一致性和工具调用异常补偿回答不够完整。",
		Strengths:            `["项目背景表达清楚"]`,
		Weaknesses:           `["Redis 缓存一致性方案不完整","Agent 工具调用失败补偿不足"]`,
		FollowUpRisks:        `["二面可能继续追问缓存删除失败、最终一致性和异常恢复"]`,
		SuggestedPreparation: `["准备延迟双删、binlog 订阅、重试和补偿方案","打磨 Agent 工具调用失败兜底"]`,
		Status:               InterviewReviewStatusGenerated,
		CreatedAt:            now,
		UpdatedAt:            now,
	}).Error; err != nil {
		t.Fatalf("seed review report: %v", err)
	}
	if err := db.Create(&InterviewQuestion{
		QuestionID:            "real-llm-question",
		InterviewID:           interviewID,
		UserID:                userID,
		Sequence:              1,
		Question:              "如果删除缓存失败怎么办？",
		Answer:                "可以通过重试解决，但没有展开完整补偿方案。",
		TopicTags:             `["Redis","缓存一致性"]`,
		Difficulty:            "medium",
		AnswerQuality:         "weak",
		WeaknessSummary:       "回答缺少延迟双删、binlog 订阅和最终一致性边界。",
		ImprovementSuggestion: "结合项目准备缓存一致性和失败补偿的系统化回答。",
		EvidenceText:          "面试官追问缓存删除失败时，候选人只提到重试。",
		CreatedAt:             now,
		UpdatedAt:             now,
	}).Error; err != nil {
		t.Fatalf("seed question: %v", err)
	}
	if err := db.Create(&MemoryItem{
		MemoryID:          memoryID,
		UserID:            userID,
		MemoryType:        MemoryTypeUserWeakness,
		SubjectKey:        "user:" + userID,
		Content:           "用户在 Redis 缓存一致性问题上缺少工程化补偿方案。",
		Evidence:          "来自一面缓存一致性追问。",
		Confidence:        MemoryConfidenceHigh,
		SourceCandidateID: "real-llm-candidate",
		SourceInterviewID: interviewID,
		Status:            MemoryItemStatusActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("seed memory item: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plan, err := s.GenerateCoachingPlan(ctx, interviewID, vo.GenerateCoachingPlanReq{
		UserID:        userID,
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}
	if plan.Status != CoachingPlanStatusGenerated {
		t.Fatalf("plan status = %q, want %q", plan.Status, CoachingPlanStatusGenerated)
	}
	if plan.OverallStrategy == "" || plan.FocusSummary == "" {
		t.Fatalf("plan content is empty: overall_strategy=%q focus_summary=%q", plan.OverallStrategy, plan.FocusSummary)
	}

	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("tasks length = 0, want > 0; raw=%s", plan.RawAgentOutput)
	}
	if _, ok := os.LookupEnv("OPENAI_API_KEY"); !ok {
		t.Fatalf("OPENAI_API_KEY disappeared during test")
	}
}
