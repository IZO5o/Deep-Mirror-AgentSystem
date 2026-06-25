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

func TestGenerateCoachingPlanRequiresReviewedInterview(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createTestInterview(t, s, "user_001")

	if _, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	}); err == nil {
		t.Fatalf("GenerateCoachingPlan() error = nil, want error")
	}
	if runners[agent.AgentTypeSecondRoundCoach].taskCalls != 0 {
		t.Fatalf("second round coach calls = %d, want 0", runners[agent.AgentTypeSecondRoundCoach].taskCalls)
	}
}

func TestGenerateCoachingPlanWithoutMemoryItems(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createReviewedTestInterview(t, s, "user_001")
	runners[agent.AgentTypeReview].taskResponse = sampleReviewJSON("review summary", "review question")
	if _, err := s.TriggerInterviewReview(context.Background(), session.InterviewID); err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v", err)
	}
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingPlanJSON("strategy without memory", "memory insufficient")

	plan, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}
	if plan.Status != CoachingPlanStatusGenerated {
		t.Fatalf("plan status = %q, want %q", plan.Status, CoachingPlanStatusGenerated)
	}
	if plan.OverallStrategy != "strategy without memory" {
		t.Fatalf("overall_strategy = %q, want %q", plan.OverallStrategy, "strategy without memory")
	}
	if !strings.Contains(runners[agent.AgentTypeSecondRoundCoach].taskQueries[0], "There are no selected relevant active memory_items") {
		t.Fatalf("coaching prompt did not include no-memory note")
	}
}

func TestGenerateCoachingPlanIncludesReviewQuestionsAndMemoryItems(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, memoryID := createCoachingReadyInterview(t, s, runners)
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "unrelated-memory",
		UserID:     "user_001",
		MemoryType: MemoryTypeCompanyProfile,
		SubjectKey: "company:Unrelated",
		Content:    "Unrelated frontend company context should not enter coaching prompt.",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
		CreatedAt:  999,
		UpdatedAt:  999,
	})
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = strings.ReplaceAll(
		sampleCoachingPlanJSON("strategy with memory", "focus with memory"),
		"MEMORY_ID_PLACEHOLDER",
		memoryID,
	)

	plan, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "technical_round",
		RemainingDays: 3,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}
	if plan.TargetRound != "technical_round" {
		t.Fatalf("target_round = %q, want %q", plan.TargetRound, "technical_round")
	}
	if plan.RemainingDays != 3 {
		t.Fatalf("remaining_days = %d, want 3", plan.RemainingDays)
	}

	prompt := runners[agent.AgentTypeSecondRoundCoach].taskQueries[0]
	for _, want := range []string{"review summary", "review question", "Selected memory_items", "selection_reason", "score", memoryID, "Redis consistency weakness"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("coaching prompt missing %q", want)
		}
	}
	if strings.Contains(prompt, "Unrelated frontend company context") {
		t.Fatalf("coaching prompt included unrelated memory")
	}

	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks length = %d, want 2", len(tasks))
	}
	if tasks[0].Status != CoachingTaskStatusTodo {
		t.Fatalf("task status = %q, want %q", tasks[0].Status, CoachingTaskStatusTodo)
	}
	if len(tasks[0].RelatedMemoryIDs) != 1 || tasks[0].RelatedMemoryIDs[0] != memoryID {
		t.Fatalf("related_memory_ids = %#v, want [%q]", tasks[0].RelatedMemoryIDs, memoryID)
	}
}

func TestGenerateCoachingPlanLimitsSelectedMemoryItems(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createMemoryReadyInterview(t, s, runners)
	for i := 0; i < defaultMemorySelectionLimit+4; i++ {
		seedMemoryItem(t, s, MemoryItem{
			MemoryID:   fmt.Sprintf("weakness-%02d", i),
			UserID:     "user_001",
			MemoryType: MemoryTypeUserWeakness,
			SubjectKey: "user:user_001",
			Content:    fmt.Sprintf("Selected weakness marker %02d for Redis and Go preparation", i),
			Confidence: MemoryConfidenceHigh,
			Status:     MemoryItemStatusActive,
			CreatedAt:  int64(i + 1),
			UpdatedAt:  int64(i + 1),
		})
	}
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleSingleTaskCoachingPlanJSON("limited strategy")

	if _, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	}); err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}

	prompt := runners[agent.AgentTypeSecondRoundCoach].taskQueries[0]
	if count := strings.Count(prompt, "Selected weakness marker"); count != defaultMemorySelectionLimit {
		t.Fatalf("selected memory marker count = %d, want %d", count, defaultMemorySelectionLimit)
	}
}

func TestGenerateCoachingPlanRepeatUpdatesPlanAndRebuildsTasks(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createCoachingReadyInterview(t, s, runners)

	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingPlanJSON("strategy one", "focus one")
	first, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("first GenerateCoachingPlan() error = %v", err)
	}

	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleSingleTaskCoachingPlanJSON("strategy two")
	second, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "hr_round",
		RemainingDays: 1,
	})
	if err != nil {
		t.Fatalf("second GenerateCoachingPlan() error = %v", err)
	}
	if second.PlanID != first.PlanID {
		t.Fatalf("plan_id = %q, want existing %q", second.PlanID, first.PlanID)
	}
	if second.OverallStrategy != "strategy two" {
		t.Fatalf("overall_strategy = %q, want %q", second.OverallStrategy, "strategy two")
	}

	tasks, err := s.ListCoachingTasks(second.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks length = %d, want 1", len(tasks))
	}
}

func TestGenerateCoachingPlanParseFailureSavesFailedPlan(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = "not json"

	if _, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	}); err == nil {
		t.Fatalf("GenerateCoachingPlan() error = nil, want error")
	}

	plan, err := s.GetCoachingPlan(session.InterviewID)
	if err != nil {
		t.Fatalf("GetCoachingPlan() error = %v", err)
	}
	if plan.Status != CoachingPlanStatusFailed {
		t.Fatalf("plan status = %q, want %q", plan.Status, CoachingPlanStatusFailed)
	}
	if plan.RawAgentOutput != "not json" {
		t.Fatalf("raw_agent_output = %q, want %q", plan.RawAgentOutput, "not json")
	}

	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks length = %d, want 0", len(tasks))
	}
}

func TestGenerateCoachingPlanAgentFailureSavesFailedPlan(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = "partial output"
	runners[agent.AgentTypeSecondRoundCoach].taskErr = errors.New("model unavailable")

	if _, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	}); err == nil {
		t.Fatalf("GenerateCoachingPlan() error = nil, want error")
	}

	plan, err := s.GetCoachingPlan(session.InterviewID)
	if err != nil {
		t.Fatalf("GetCoachingPlan() error = %v", err)
	}
	if plan.Status != CoachingPlanStatusFailed {
		t.Fatalf("plan status = %q, want %q", plan.Status, CoachingPlanStatusFailed)
	}
	if plan.RawAgentOutput != "partial output" {
		t.Fatalf("raw_agent_output = %q, want %q", plan.RawAgentOutput, "partial output")
	}
}

func TestUpdateCoachingTaskStatus(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleSingleTaskCoachingPlanJSON("strategy")
	plan, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}
	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		t.Fatalf("ListCoachingTasks() error = %v", err)
	}

	updated, err := s.UpdateCoachingTask(tasks[0].TaskID, vo.UpdateCoachingTaskReq{Status: CoachingTaskStatusDone})
	if err != nil {
		t.Fatalf("UpdateCoachingTask() error = %v", err)
	}
	if updated.Status != CoachingTaskStatusDone {
		t.Fatalf("task status = %q, want %q", updated.Status, CoachingTaskStatusDone)
	}

	if _, err := s.UpdateCoachingTask(tasks[0].TaskID, vo.UpdateCoachingTaskReq{Status: "blocked"}); err == nil {
		t.Fatalf("UpdateCoachingTask() invalid status error = nil, want error")
	}
}

func TestGenerateCoachingPlanDoesNotWriteMemoryItems(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createCoachingReadyInterview(t, s, runners)
	before, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingPlanJSON("strategy", "focus")

	if _, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	}); err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}

	after, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("memory item count changed from %d to %d", len(before), len(after))
	}
}

func createCoachingReadyInterview(t *testing.T, s *Server, runners map[agent.AgentType]*fakeRunner) (vo.InterviewSessionVO, string) {
	t.Helper()

	session := createMemoryReadyInterview(t, s, runners)
	runners[agent.AgentTypeMemoryCurator].taskResponse = sampleMemoryCandidateJSON("Redis consistency weakness")
	candidates, err := s.GenerateMemoryCandidates(context.Background(), session.InterviewID)
	if err != nil {
		t.Fatalf("GenerateMemoryCandidates() error = %v", err)
	}
	item, err := s.AcceptMemoryCandidate(candidates[0].CandidateID)
	if err != nil {
		t.Fatalf("AcceptMemoryCandidate() error = %v", err)
	}
	return session, item.MemoryID
}

func sampleCoachingPlanJSON(strategy string, focus string) string {
	return `{
  "overall_strategy": "` + strategy + `",
  "focus_summary": "` + focus + `",
  "tasks": [
    {
      "sequence": 1,
      "day_index": 1,
      "task_type": "weakness_fix",
      "title": "补齐 Redis 缓存一致性回答",
      "description": "准备延迟双删、binlog 订阅、最终一致性和异常补偿的回答。",
      "related_memory_ids": ["MEMORY_ID_PLACEHOLDER"],
      "priority": "high"
    },
    {
      "sequence": 2,
      "day_index": 2,
      "task_type": "project_answer_polish",
      "title": "打磨项目深挖回答",
      "description": "准备架构、故障恢复和指标收益。",
      "related_memory_ids": [],
      "priority": "medium"
    }
  ]
}`
}

func sampleSingleTaskCoachingPlanJSON(strategy string) string {
	return `{
  "overall_strategy": "` + strategy + `",
  "focus_summary": "single task focus",
  "tasks": [
    {
      "sequence": 1,
      "day_index": 1,
      "task_type": "company_preparation",
      "title": "准备公司业务问题",
      "description": "结合岗位要求准备问题清单。",
      "related_memory_ids": [],
      "priority": "low"
    }
  ]
}`
}
