package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestCoachingPlanControllerFlow(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session, _ := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleSingleTaskCoachingPlanJSON("controller strategy")

	generateRec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+session.InterviewID+"/coaching-plan", `{"user_id":"user_001","target_round":"second_round","remaining_days":2}`)
	if generateRec.Code != http.StatusOK {
		t.Fatalf("generate status = %d, want %d; body=%s", generateRec.Code, http.StatusOK, generateRec.Body.String())
	}
	var plan vo.CoachingPlanVO
	decodeOKData(t, generateRec, &plan)
	if plan.Status != CoachingPlanStatusGenerated {
		t.Fatalf("plan status = %q, want %q", plan.Status, CoachingPlanStatusGenerated)
	}

	getRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+session.InterviewID+"/coaching-plan", "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var gotPlan vo.CoachingPlanVO
	decodeOKData(t, getRec, &gotPlan)
	if gotPlan.PlanID != plan.PlanID {
		t.Fatalf("plan_id = %q, want %q", gotPlan.PlanID, plan.PlanID)
	}

	tasksRec := performJSONRequest(router, http.MethodGet, "/api/coaching-plans/"+plan.PlanID+"/tasks", "")
	if tasksRec.Code != http.StatusOK {
		t.Fatalf("tasks status = %d, want %d; body=%s", tasksRec.Code, http.StatusOK, tasksRec.Body.String())
	}
	var tasks []vo.CoachingTaskVO
	decodeOKData(t, tasksRec, &tasks)
	if len(tasks) != 1 {
		t.Fatalf("tasks length = %d, want 1", len(tasks))
	}

	updateRec := performJSONRequest(router, http.MethodPatch, "/api/coaching-tasks/"+tasks[0].TaskID, `{"status":"done"}`)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}
	var updated vo.CoachingTaskVO
	decodeOKData(t, updateRec, &updated)
	if updated.Status != CoachingTaskStatusDone {
		t.Fatalf("task status = %q, want %q", updated.Status, CoachingTaskStatusDone)
	}
}

func TestCoachingPlanControllerBeforeReviewedReturnsError(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	rec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+session.InterviewID+"/coaching-plan", `{"user_id":"user_001","target_round":"second_round","remaining_days":2}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestCoachingTaskControllerInvalidStatusReturnsError(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session, _ := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleSingleTaskCoachingPlanJSON("controller strategy")
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

	rec := performJSONRequest(router, http.MethodPatch, "/api/coaching-tasks/"+tasks[0].TaskID, `{"status":"blocked"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestCoachingSessionControllerFlow(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	router := NewRouter(s)

	startRec := performJSONRequest(router, http.MethodPost, "/api/coaching-plans/"+plan.PlanID+"/sessions?user_id=user_001", "")
	if startRec.Code != http.StatusOK {
		t.Fatalf("start status = %d, want %d; body=%s", startRec.Code, http.StatusOK, startRec.Body.String())
	}
	var started vo.CoachingSessionDetailVO
	decodeOKData(t, startRec, &started)
	if started.Session.Status != CoachingSessionStatusWaitingUserAnswer {
		t.Fatalf("started status = %q, want waiting", started.Session.Status)
	}

	resumeRec := performJSONRequest(router, http.MethodPost, "/api/coaching-plans/"+plan.PlanID+"/sessions?user_id=user_001", "")
	if resumeRec.Code != http.StatusOK {
		t.Fatalf("resume status = %d, want %d; body=%s", resumeRec.Code, http.StatusOK, resumeRec.Body.String())
	}
	var resumed vo.CoachingSessionDetailVO
	decodeOKData(t, resumeRec, &resumed)
	if resumed.Session.SessionID != started.Session.SessionID {
		t.Fatalf("resumed session_id = %q, want %q", resumed.Session.SessionID, started.Session.SessionID)
	}

	getRec := performJSONRequest(router, http.MethodGet, "/api/coaching-sessions/"+started.Session.SessionID, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, false, false, 60, "需要补充异常处理。", CoachingNextActionAskRetry, false)
	turnRec := performJSONRequest(router, http.MethodPost, "/api/coaching-sessions/"+started.Session.SessionID+"/turns", `{"user_input":"我先讲缓存一致性。","submit_mode":"formal_answer"}`)
	if turnRec.Code != http.StatusOK {
		t.Fatalf("turn status = %d, want %d; body=%s", turnRec.Code, http.StatusOK, turnRec.Body.String())
	}
	var afterTurn vo.CoachingSessionDetailVO
	decodeOKData(t, turnRec, &afterTurn)
	if afterTurn.Session.Status != CoachingSessionStatusNeedsRevision || len(afterTurn.Attempts) != 1 {
		t.Fatalf("after turn = %#v, want needs_revision with attempt", afterTurn)
	}

	pauseRec := performJSONRequest(router, http.MethodPost, "/api/coaching-sessions/"+started.Session.SessionID+"/pause", "")
	if pauseRec.Code != http.StatusOK {
		t.Fatalf("pause status = %d, want %d; body=%s", pauseRec.Code, http.StatusOK, pauseRec.Body.String())
	}
	var paused vo.CoachingSessionDetailVO
	decodeOKData(t, pauseRec, &paused)
	if paused.Session.Status != CoachingSessionStatusPaused {
		t.Fatalf("paused status = %q, want paused", paused.Session.Status)
	}

	cancelRec := performJSONRequest(router, http.MethodPost, "/api/coaching-sessions/"+started.Session.SessionID+"/cancel", "")
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, want %d; body=%s", cancelRec.Code, http.StatusOK, cancelRec.Body.String())
	}
	var cancelled vo.CoachingSessionDetailVO
	decodeOKData(t, cancelRec, &cancelled)
	if cancelled.Session.Status != CoachingSessionStatusCancelled {
		t.Fatalf("cancelled status = %q, want cancelled", cancelled.Session.Status)
	}
}

func TestResumeFailedCoachingSessionControllerRoute(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	router := NewRouter(s)
	session := startTestCoachingSession(t, s, plan.PlanID)
	runner := runners[agent.AgentTypeSecondRoundCoach]
	runner.taskErr = errors.New("model unavailable")
	runner.taskResponse = "partial coaching output"
	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{
		UserInput:  "我会从缓存一致性和失败补偿回答。",
		SubmitMode: CoachingSubmitModeFormalAnswer,
	}); err == nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = nil, want model error")
	}

	runner.taskErr = nil
	runner.taskResponse = sampleCoachingSessionDecisionJSON(CoachingInputTypeFormalAnswer, true, true, 88, "恢复后回答达标。", CoachingNextActionPromptNext, false)
	resumeRec := performJSONRequest(router, http.MethodPost, "/api/coaching-sessions/"+session.Session.SessionID+"/resume", "")
	if resumeRec.Code != http.StatusOK {
		t.Fatalf("resume status = %d, want %d; body=%s", resumeRec.Code, http.StatusOK, resumeRec.Body.String())
	}
	var resumed vo.CoachingSessionDetailVO
	decodeOKData(t, resumeRec, &resumed)
	if resumed.Session.FailedRetryCount != 1 || resumed.Session.Status != CoachingSessionStatusWaitingUserAnswer {
		t.Fatalf("resumed session = %#v, want retry count 1 and waiting", resumed.Session)
	}
}
