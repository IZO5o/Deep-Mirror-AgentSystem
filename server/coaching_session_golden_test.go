package server

import (
	"context"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestGoldenCoachingFormalAnswerPassedAdvancesTask(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	firstTaskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 88, "回答达标，进入下一项。", CoachingNextActionPromptNext, false)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "我会覆盖一致性、补偿和监控。", SubmitMode: CoachingSubmitModeFormalAnswer})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if len(updated.Turns) != 3 || updated.Turns[1].Role != CoachingTurnRoleUser || updated.Turns[2].TurnType != CoachingTurnTypeFeedback {
		t.Fatalf("turns = %#v, want start/user/feedback", updated.Turns)
	}
	if len(updated.Attempts) != 1 || !updated.Attempts[0].Passed || updated.Attempts[0].Score != 88 {
		t.Fatalf("attempts = %#v, want one passed score 88", updated.Attempts)
	}
	var firstTask CoachingTask
	if err := s.db.First(&firstTask, "task_id = ?", firstTaskID).Error; err != nil {
		t.Fatalf("load first task: %v", err)
	}
	if firstTask.Status != CoachingTaskStatusDone {
		t.Fatalf("first task status = %q, want done", firstTask.Status)
	}
	if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer || updated.Session.CurrentTaskID == "" || updated.Session.CurrentTaskID == firstTaskID {
		t.Fatalf("session = %#v, want advanced to next waiting task", updated.Session)
	}
	states, err := s.ListPracticeStates("user_001", "补齐 Redis 缓存一致性回答", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 || states[0].SourceID != updated.Attempts[0].AttemptID {
		t.Fatalf("practice states = %#v, want source attempt", states)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceBase(t, trace, AgentDecisionTraceStatusSucceeded, agent.AgentTypeSecondRoundCoach, AgentTraceSourceCoachingSession, session.Session.SessionID, AgentTraceStepCoachingSessionTurn)
	if trace.RawAgentOutput == "" || !strings.Contains(trace.ParsedDecision, CoachingInputTypeFormalAnswer) {
		t.Fatalf("trace missing raw/parsed decision: %#v", trace)
	}
	if !strings.Contains(trace.InputSnapshot, `"submit_mode":"formal_answer"`) {
		t.Fatalf("trace input snapshot missing submit_mode: %s", trace.InputSnapshot)
	}
	for _, want := range []string{`"submit_mode":"formal_answer"`, `"user_intent":"answer"`, `"state_action":"record_attempt"`, `"visible_message":"回答达标，进入下一项。"`} {
		if !strings.Contains(trace.ParsedDecision, want) {
			t.Fatalf("trace parsed decision missing %s: %s", want, trace.ParsedDecision)
		}
	}
	assertTraceContainsAction(t, trace, "recorded coaching_session user turn", "recorded coaching_task_attempt", "updated practice_states", "marked coaching_task done")
}

func TestGoldenCoachingFormalAnswerFailedNeedsRevision(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, false, false, 52, "缺少补偿路径，请重答。", CoachingNextActionAskRetry, false)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "我会用 Redis 做缓存。", SubmitMode: CoachingSubmitModeFormalAnswer})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if updated.Session.Status != CoachingSessionStatusNeedsRevision || updated.Session.CurrentTaskID != taskID {
		t.Fatalf("session = %#v, want needs_revision same task", updated.Session)
	}
	if len(updated.Attempts) != 1 || updated.Attempts[0].Passed {
		t.Fatalf("attempts = %#v, want one failed attempt", updated.Attempts)
	}
	states, err := s.ListPracticeStates("user_001", "补齐 Redis 缓存一致性回答", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 || states[0].MasteryScore != 52 {
		t.Fatalf("practice states = %#v, want score 52", states)
	}
	var task CoachingTask
	if err := s.db.First(&task, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != CoachingTaskStatusNeedsRevision {
		t.Fatalf("task status = %q, want needs_revision", task.Status)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceContainsAction(t, trace, "recorded coaching_task_attempt", "updated practice_states", "updated coaching_session state")
	assertTraceNotContainsAction(t, trace, "marked coaching_task done")
}

func TestGoldenCoachingHintAndExplanationDoNotMutatePractice(t *testing.T) {
	for _, tc := range []struct {
		name      string
		inputType string
	}{
		{name: "hint", inputType: CoachingInputTypeHintRequest},
		{name: "explanation", inputType: CoachingInputTypeExplanationRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, runners, plan := createCoachingSessionReadyPlan(t)
			session := startTestCoachingSession(t, s, plan.PlanID)
			taskID := session.Session.CurrentTaskID
			runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(tc.inputType, false, false, 0, "这是提示或解释。", CoachingNextActionContinue, false)

			updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "给我一点帮助", SubmitMode: CoachingSubmitModeChat})
			if err != nil {
				t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
			}
			if len(updated.Attempts) != 0 {
				t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
			}
			assertNoPracticeStates(t, s, "user_001")
			if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer || updated.Session.CurrentTaskID != taskID {
				t.Fatalf("session = %#v, want waiting same task", updated.Session)
			}

			trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
				SourceType: AgentTraceSourceCoachingSession,
				SourceID:   session.Session.SessionID,
				StepName:   AgentTraceStepCoachingSessionTurn,
				Status:     AgentDecisionTraceStatusSucceeded,
			})
			assertTraceContainsAction(t, trace, "recorded coaching_session user turn", "recorded coaching_session assistant turn", "updated coaching_session state")
			assertTraceNotContainsAction(t, trace, "recorded coaching_task_attempt")
			assertTraceNotContainsAction(t, trace, "updated practice_states")
		})
	}
}

func TestGoldenCoachingSkipTaskAdvancesWithoutPractice(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeSkipTask, false, false, 0, "跳过当前任务。", CoachingNextActionPromptNext, false)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "跳过这个任务", SubmitMode: CoachingSubmitModeChat})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if len(updated.Attempts) != 0 {
		t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
	}
	assertNoPracticeStates(t, s, "user_001")
	var skipped CoachingTask
	if err := s.db.First(&skipped, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("load skipped task: %v", err)
	}
	if skipped.Status != CoachingTaskStatusSkipped {
		t.Fatalf("task status = %q, want skipped", skipped.Status)
	}
	if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer || updated.Session.CurrentTaskID == taskID {
		t.Fatalf("session = %#v, want advanced without practice update", updated.Session)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceContainsAction(t, trace, "marked coaching_task skipped", "updated coaching_session state")
	assertTraceNotContainsAction(t, trace, "updated practice_states")
}
