package server

import (
	"context"
	"net/http"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestMockInterviewControllerFlow(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("controller next question", 75),
		sampleMockCompleteJSON(),
	}

	startRec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+session.InterviewID+"/mock-interviews", `{"user_id":"user_001","plan_id":"`+planID+`","target_round":"second_round"}`)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start status = %d, want %d; body=%s", startRec.Code, http.StatusOK, startRec.Body.String())
	}
	var mock vo.MockInterviewVO
	decodeOKData(t, startRec, &mock)
	if mock.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock status = %q, want %q", mock.Status, MockInterviewStatusWaitingAnswer)
	}

	turnRec := performJSONRequest(router, http.MethodPost, "/api/mock-interviews/"+mock.MockID+"/turns", `{"answer":"我做了一个 Go 多 Agent 面试复盘系统。"}`)
	if turnRec.Code != http.StatusOK {
		t.Fatalf("turn status = %d, want %d; body=%s", turnRec.Code, http.StatusOK, turnRec.Body.String())
	}
	var turn vo.MockTurnVO
	decodeOKData(t, turnRec, &turn)
	if turn.NextQuestion != "controller next question" {
		t.Fatalf("next_question = %q, want %q", turn.NextQuestion, "controller next question")
	}

	getRec := performJSONRequest(router, http.MethodGet, "/api/mock-interviews/"+mock.MockID, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var gotMock vo.MockInterviewVO
	decodeOKData(t, getRec, &gotMock)
	if gotMock.CurrentTurn != 1 {
		t.Fatalf("current_turn = %d, want 1", gotMock.CurrentTurn)
	}

	turnsRec := performJSONRequest(router, http.MethodGet, "/api/mock-interviews/"+mock.MockID+"/turns", "")
	if turnsRec.Code != http.StatusOK {
		t.Fatalf("turns status = %d, want %d; body=%s", turnsRec.Code, http.StatusOK, turnsRec.Body.String())
	}
	var turns []vo.MockTurnVO
	decodeOKData(t, turnsRec, &turns)
	if len(turns) != 4 {
		t.Fatalf("turns length = %d, want 4", len(turns))
	}

	completeRec := performJSONRequest(router, http.MethodPost, "/api/mock-interviews/"+mock.MockID+"/complete", "")
	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status = %d, want %d; body=%s", completeRec.Code, http.StatusOK, completeRec.Body.String())
	}
	var completed vo.MockInterviewVO
	decodeOKData(t, completeRec, &completed)
	if completed.Status != MockInterviewStatusCompleted {
		t.Fatalf("completed status = %q, want %q", completed.Status, MockInterviewStatusCompleted)
	}
}

func TestMockInterviewControllerMissingAnswerReturns400(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}

	rec := performJSONRequest(router, http.MethodPost, "/api/mock-interviews/"+mock.MockID+"/turns", `{}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestMockInterviewControllerBeforeReviewedReturnsError(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	rec := performJSONRequest(router, http.MethodPost, "/api/interviews/"+session.InterviewID+"/mock-interviews", `{"user_id":"user_001","target_round":"second_round"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}
