package server

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestStartOrResumeCoachingSession_Idempotent(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)

	first, err := s.StartOrResumeCoachingSession(plan.PlanID, "user_001")
	if err != nil {
		t.Fatalf("StartOrResumeCoachingSession() error = %v", err)
	}
	if first.Session.Status != CoachingSessionStatusWaitingUserAnswer {
		t.Fatalf("session status = %q, want %q", first.Session.Status, CoachingSessionStatusWaitingUserAnswer)
	}
	if first.Session.CurrentTaskID == "" {
		t.Fatalf("current_task_id is empty")
	}
	if first.CurrentTask == nil || first.CurrentTask.Sequence != 1 {
		t.Fatalf("current task = %#v, want sequence 1", first.CurrentTask)
	}
	if len(first.Turns) != 1 || first.Turns[0].Role != CoachingTurnRoleAssistant || first.Turns[0].TurnType != CoachingTurnTypeStart {
		t.Fatalf("start turns = %#v, want one assistant start turn", first.Turns)
	}

	second, err := s.StartOrResumeCoachingSession(plan.PlanID, "user_001")
	if err != nil {
		t.Fatalf("second StartOrResumeCoachingSession() error = %v", err)
	}
	if second.Session.SessionID != first.Session.SessionID {
		t.Fatalf("session_id = %q, want existing %q", second.Session.SessionID, first.Session.SessionID)
	}
	var count int64
	if err := s.db.Model(&CoachingSession{}).
		Where("coaching_plan_id = ?", plan.PlanID).
		Count(&count).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("session count = %d, want 1", count)
	}
	if runners[agent.AgentTypeSecondRoundCoach].taskCalls != 1 {
		t.Fatalf("second_round_coach task calls = %d, want only plan generation call", runners[agent.AgentTypeSecondRoundCoach].taskCalls)
	}
}

func TestSubmitCoachingSessionTurn_FormalAnswerPassedAdvancesTask(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	firstTaskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 88, "回答达标，进入下一项。", CoachingNextActionPromptNext, false)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "我会从缓存一致性、失败补偿和监控告警三个层面回答。"})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if len(updated.Attempts) != 1 {
		t.Fatalf("attempts length = %d, want 1", len(updated.Attempts))
	}
	if !updated.Attempts[0].Passed || updated.Attempts[0].Score != 88 || updated.Attempts[0].AttemptIndex != 1 {
		t.Fatalf("attempt = %#v, want passed score 88 index 1", updated.Attempts[0])
	}
	states, err := s.ListPracticeStates("user_001", "补齐 Redis 缓存一致性回答", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("practice states length = %d, want 1", len(states))
	}
	if states[0].MasteryScore != 88 || states[0].LastScore != 88 || states[0].AttemptCount != 1 {
		t.Fatalf("practice state = %#v, want score=88 attempts=1", states[0])
	}
	if states[0].SourceType != PracticeStateSourceCoachingTaskAttempt || states[0].SourceID != updated.Attempts[0].AttemptID {
		t.Fatalf("practice state source = %s/%s, want %s/%s", states[0].SourceType, states[0].SourceID, PracticeStateSourceCoachingTaskAttempt, updated.Attempts[0].AttemptID)
	}
	if updated.Session.CurrentTaskID == "" || updated.Session.CurrentTaskID == firstTaskID {
		t.Fatalf("current_task_id = %q, want advanced from %q", updated.Session.CurrentTaskID, firstTaskID)
	}
	if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer {
		t.Fatalf("session status = %q, want waiting", updated.Session.Status)
	}

	var firstTask CoachingTask
	if err := s.db.First(&firstTask, "task_id = ?", firstTaskID).Error; err != nil {
		t.Fatalf("load first task: %v", err)
	}
	if firstTask.Status != CoachingTaskStatusDone {
		t.Fatalf("first task status = %q, want %q", firstTask.Status, CoachingTaskStatusDone)
	}
	if len(updated.Turns) != 3 || updated.Turns[1].Role != CoachingTurnRoleUser || updated.Turns[2].Role != CoachingTurnRoleAssistant {
		t.Fatalf("turns = %#v, want start/user/assistant", updated.Turns)
	}
	if !strings.Contains(runners[agent.AgentTypeSecondRoundCoach].taskQueries[len(runners[agent.AgentTypeSecondRoundCoach].taskQueries)-1], "Current task") {
		t.Fatalf("submit prompt missing current task context")
	}
}

func TestSubmitCoachingSessionTurn_FormalAnswerFailedNeedsRevision(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, false, false, 52, "缺少异常补偿和权衡。请重答。", CoachingNextActionAskRetry, false)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "我会用 Redis 做缓存。"})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if updated.Session.Status != CoachingSessionStatusNeedsRevision {
		t.Fatalf("session status = %q, want %q", updated.Session.Status, CoachingSessionStatusNeedsRevision)
	}
	if updated.Session.CurrentTaskID != taskID {
		t.Fatalf("current_task_id = %q, want same %q", updated.Session.CurrentTaskID, taskID)
	}
	if len(updated.Attempts) != 1 || updated.Attempts[0].Passed || updated.Attempts[0].Score != 52 {
		t.Fatalf("attempts = %#v, want one failed attempt", updated.Attempts)
	}
	states, err := s.ListPracticeStates("user_001", "补齐 Redis 缓存一致性回答", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 || states[0].MasteryScore != 52 || states[0].SourceType != PracticeStateSourceCoachingTaskAttempt {
		t.Fatalf("practice states = %#v, want failed formal answer state score 52", states)
	}

	var task CoachingTask
	if err := s.db.First(&task, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != CoachingTaskStatusNeedsRevision {
		t.Fatalf("task status = %q, want %q", task.Status, CoachingTaskStatusNeedsRevision)
	}
	if !strings.Contains(updated.Session.LastAgentMessage, "请重答") {
		t.Fatalf("last_agent_message = %q, want retry prompt", updated.Session.LastAgentMessage)
	}
}

func TestSubmitCoachingSessionTurn_HintAndExplanationDoNotCreateAttempt(t *testing.T) {
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

			updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "给我一点提示"})
			if err != nil {
				t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
			}
			if len(updated.Attempts) != 0 {
				t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
			}
			states, err := s.ListPracticeStates("user_001", "", "")
			if err != nil {
				t.Fatalf("ListPracticeStates() error = %v", err)
			}
			if len(states) != 0 {
				t.Fatalf("practice states length = %d, want 0", len(states))
			}
			if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer {
				t.Fatalf("session status = %q, want waiting", updated.Session.Status)
			}
			var task CoachingTask
			if err := s.db.First(&task, "task_id = ?", taskID).Error; err != nil {
				t.Fatalf("load task: %v", err)
			}
			if task.Status == CoachingTaskStatusDone {
				t.Fatalf("task status = done, want not done")
			}
			if updated.Turns[1].TurnType != tc.inputType {
				t.Fatalf("user turn type = %q, want %q", updated.Turns[1].TurnType, tc.inputType)
			}
		})
	}
}

func TestPauseCancelAndIllegalSubmit(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)

	paused, err := s.PauseCoachingSession(session.Session.SessionID)
	if err != nil {
		t.Fatalf("PauseCoachingSession() error = %v", err)
	}
	if paused.Session.Status != CoachingSessionStatusPaused {
		t.Fatalf("paused status = %q, want paused", paused.Session.Status)
	}
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 90, "ok", CoachingNextActionPromptNext, false)
	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "answer"}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() paused error = nil, want error")
	}

	resumed, err := s.StartOrResumeCoachingSession(plan.PlanID, "user_001")
	if err != nil {
		t.Fatalf("resume StartOrResumeCoachingSession() error = %v", err)
	}
	if resumed.Session.Status != CoachingSessionStatusWaitingUserAnswer {
		t.Fatalf("resumed status = %q, want waiting", resumed.Session.Status)
	}

	cancelled, err := s.CancelCoachingSession(session.Session.SessionID)
	if err != nil {
		t.Fatalf("CancelCoachingSession() error = %v", err)
	}
	if cancelled.Session.Status != CoachingSessionStatusCancelled {
		t.Fatalf("cancelled status = %q, want cancelled", cancelled.Session.Status)
	}
	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "answer"}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() cancelled error = nil, want error")
	}
}

func TestSubmitCoachingSessionTurn_CompletedSessionCannotContinue(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	if err := s.db.Model(&CoachingSession{}).
		Where("session_id = ?", session.Session.SessionID).
		Update("status", CoachingSessionStatusCompleted).Error; err != nil {
		t.Fatalf("mark completed: %v", err)
	}
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 90, "ok", CoachingNextActionCompletePlan, false)

	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "answer"}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() completed error = nil, want error")
	}
}

func TestSubmitCoachingSessionTurn_ParseFailureMarksSessionFailed(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = "not json"

	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "正式回答"}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = nil, want parse error")
	}
	updated, err := s.GetCoachingSession(session.Session.SessionID)
	if err != nil {
		t.Fatalf("GetCoachingSession() error = %v", err)
	}
	if updated.Session.Status != CoachingSessionStatusFailed {
		t.Fatalf("session status = %q, want failed", updated.Session.Status)
	}
	if updated.Session.ErrorMessage == "" {
		t.Fatalf("error_message is empty")
	}
	if len(updated.Attempts) != 0 {
		t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
	}
	states, err := s.ListPracticeStates("user_001", "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("practice states length = %d, want 0", len(states))
	}
	var task CoachingTask
	if err := s.db.First(&task, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status == CoachingTaskStatusDone {
		t.Fatalf("task status = done, want not done")
	}
	if len(updated.Turns) != 3 || updated.Turns[2].TurnType != CoachingTurnTypeError || updated.Turns[2].RawAgentOutput != "not json" {
		t.Fatalf("turns = %#v, want error turn with raw output", updated.Turns)
	}
}

func TestSubmitCoachingSessionTurn_SkipAndPauseDoNotUpdatePracticeState(t *testing.T) {
	for _, tc := range []struct {
		name      string
		inputType string
		action    string
		pause     bool
	}{
		{name: "skip", inputType: CoachingInputTypeSkipTask, action: CoachingNextActionPromptNext},
		{name: "pause", inputType: CoachingInputTypePause, action: CoachingNextActionPause, pause: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, runners, plan := createCoachingSessionReadyPlan(t)
			session := startTestCoachingSession(t, s, plan.PlanID)
			runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(tc.inputType, false, false, 0, "跳过或暂停。", tc.action, tc.pause)

			updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: tc.name})
			if err != nil {
				t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
			}
			if len(updated.Attempts) != 0 {
				t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
			}
			states, err := s.ListPracticeStates("user_001", "", "")
			if err != nil {
				t.Fatalf("ListPracticeStates() error = %v", err)
			}
			if len(states) != 0 {
				t.Fatalf("practice states length = %d, want 0", len(states))
			}
		})
	}
}

func TestSubmitCoachingSessionTurn_PracticeStateFailureRollsBackRound(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 88, "回答达标。", CoachingNextActionPromptNext, false)
	if err := s.db.Exec("DROP TABLE practice_states").Error; err != nil {
		t.Fatalf("drop practice_states: %v", err)
	}

	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{UserInput: "正式回答"}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = nil, want practice state error")
	}
	updated, err := s.GetCoachingSession(session.Session.SessionID)
	if err != nil {
		t.Fatalf("GetCoachingSession() error = %v", err)
	}
	if len(updated.Turns) != 1 {
		t.Fatalf("turns length = %d, want only start turn after rollback", len(updated.Turns))
	}
	if len(updated.Attempts) != 0 {
		t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
	}
	if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer || updated.Session.CurrentTaskID != taskID {
		t.Fatalf("session = %#v, want unchanged waiting current task", updated.Session)
	}
	var task CoachingTask
	if err := s.db.First(&task, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != CoachingTaskStatusInProgress {
		t.Fatalf("task status = %q, want %q", task.Status, CoachingTaskStatusInProgress)
	}
}

func createCoachingSessionReadyPlan(t *testing.T) (*Server, map[agent.AgentType]*fakeRunner, vo.CoachingPlanVO) {
	t.Helper()
	s, runners := newTestServerWithFakeAgents(t)
	session, memoryID := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = strings.ReplaceAll(sampleCoachingPlanJSON("session strategy", "session focus"), "MEMORY_ID_PLACEHOLDER", memoryID)
	plan, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}
	return s, runners, plan
}

func startTestCoachingSession(t *testing.T, s *Server, planID string) vo.CoachingSessionDetailVO {
	t.Helper()
	session, err := s.StartOrResumeCoachingSession(planID, "user_001")
	if err != nil {
		t.Fatalf("StartOrResumeCoachingSession() error = %v", err)
	}
	return session
}

func sampleCoachingSessionDecisionJSON(inputType string, passed bool, complete bool, score int, feedback string, nextAction string, pause bool) string {
	return `{
  "input_type": "` + inputType + `",
  "agent_message": "` + feedback + `",
  "score": ` + strconv.Itoa(score) + `,
  "passed": ` + boolString(passed) + `,
  "feedback": "` + feedback + `",
  "next_action": "` + nextAction + `",
  "should_complete_current_task": ` + boolString(complete) + `,
  "should_pause": ` + boolString(pause) + `
}`
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
