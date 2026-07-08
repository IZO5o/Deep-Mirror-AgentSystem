package server

import (
	"testing"

	"github.com/google/uuid"

	"agent-web-base/vo"
)

func TestCreatePracticeGoalDefaultsAndRoundTrip(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)

	goal, err := s.CreatePracticeGoal(vo.CreatePracticeGoalReq{
		UserID:         " user_001 ",
		CompanyName:    " ByteDance ",
		JobTitle:       "Backend Engineer",
		JobDescription: "负责高并发推荐服务",
		FocusTopics:    []string{"缓存一致性", "Redlock 争议点"},
	})
	if err != nil {
		t.Fatalf("CreatePracticeGoal() error = %v", err)
	}
	if goal.UserID != "user_001" || goal.CompanyName != "ByteDance" {
		t.Fatalf("goal identity = %#v, want trimmed user/company", goal)
	}
	if goal.TargetRound != "second_round" || goal.RemainingDays != 1 || goal.Status != PracticeGoalStatusActive {
		t.Fatalf("goal defaults = %#v, want second_round/1/active", goal)
	}
	if len(goal.FocusTopics) != 2 || goal.FocusTopics[1] != "Redlock 争议点" {
		t.Fatalf("focus topics = %#v", goal.FocusTopics)
	}
	if goal.InterviewID != "" {
		t.Fatalf("interview_id = %q, want empty for standalone goal", goal.InterviewID)
	}
}

func TestPracticeGoalCRUDValidationAndArchive(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	if _, err := s.CreatePracticeGoal(vo.CreatePracticeGoalReq{CompanyName: "ByteDance"}); err == nil {
		t.Fatalf("CreatePracticeGoal() missing user_id error = nil")
	}
	if _, err := s.CreatePracticeGoal(vo.CreatePracticeGoalReq{UserID: "user_001"}); err == nil {
		t.Fatalf("CreatePracticeGoal() missing company_name error = nil")
	}

	goal, err := s.CreatePracticeGoal(vo.CreatePracticeGoalReq{
		UserID:        "user_001",
		CompanyName:   "ByteDance",
		TargetRound:   "hr_round",
		FocusTopics:   []string{"项目复盘"},
		RemainingDays: 3,
	})
	if err != nil {
		t.Fatalf("CreatePracticeGoal() error = %v", err)
	}
	updated, err := s.UpdatePracticeGoal(goal.GoalID, vo.UpdatePracticeGoalReq{
		JobTitle:      "Senior Backend Engineer",
		FocusTopics:   []string{"项目复盘", "系统设计"},
		RemainingDays: 5,
	})
	if err != nil {
		t.Fatalf("UpdatePracticeGoal() error = %v", err)
	}
	if updated.JobTitle != "Senior Backend Engineer" || updated.RemainingDays != 5 || len(updated.FocusTopics) != 2 {
		t.Fatalf("updated = %#v", updated)
	}
	if _, err := s.UpdatePracticeGoal(goal.GoalID, vo.UpdatePracticeGoalReq{Status: "deleted"}); err == nil {
		t.Fatalf("UpdatePracticeGoal() invalid status error = nil")
	}
	goals, err := s.ListPracticeGoals("user_001", PracticeGoalStatusActive)
	if err != nil {
		t.Fatalf("ListPracticeGoals() error = %v", err)
	}
	if len(goals) != 1 || goals[0].GoalID != goal.GoalID {
		t.Fatalf("goals = %#v, want created goal", goals)
	}
	archived, err := s.ArchivePracticeGoal(goal.GoalID)
	if err != nil {
		t.Fatalf("ArchivePracticeGoal() error = %v", err)
	}
	if archived.Status != PracticeGoalStatusArchived {
		t.Fatalf("archived status = %q", archived.Status)
	}
}

func TestPracticeGoalPlansAllowBlankInterviewIDDuplicates(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	now := int64(123)
	first := CoachingPlan{
		PlanID:         uuid.NewString(),
		UserID:         "user_001",
		InterviewID:    "",
		PracticeGoalID: "goal_1",
		SourceType:     CoachingPlanSourcePracticeGoal,
		Status:         CoachingPlanStatusGenerated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	second := CoachingPlan{
		PlanID:         uuid.NewString(),
		UserID:         "user_001",
		InterviewID:    "",
		PracticeGoalID: "goal_2",
		SourceType:     CoachingPlanSourcePracticeGoal,
		Status:         CoachingPlanStatusGenerated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.db.Create(&first).Error; err != nil {
		t.Fatalf("create first blank-interview plan: %v", err)
	}
	if err := s.db.Create(&second).Error; err != nil {
		t.Fatalf("create second blank-interview plan: %v", err)
	}
}
