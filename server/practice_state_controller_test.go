package server

import (
	"net/http"
	"testing"

	"agent-web-base/vo"
)

func TestPracticeStateControllerListAndGet(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)
	backend := seedPracticeState(t, s, "user_001", "Redis 缓存一致性", PracticeDimensionBackendKnowledge, 70)
	seedPracticeState(t, s, "user_001", "项目表达", PracticeDimensionCommunication, 55)

	listRec := performJSONRequest(router, http.MethodGet, "/api/practice-states?user_id=user_001&dimension="+PracticeDimensionBackendKnowledge, "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var states []vo.PracticeStateVO
	decodeOKData(t, listRec, &states)
	if len(states) != 1 || states[0].StateID != backend.StateID {
		t.Fatalf("states = %#v, want backend state", states)
	}

	getRec := performJSONRequest(router, http.MethodGet, "/api/practice-states/"+backend.StateID, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var got vo.PracticeStateVO
	decodeOKData(t, getRec, &got)
	if got.Topic != "Redis 缓存一致性" || got.MasteryScore != 70 {
		t.Fatalf("state = %#v, want Redis score 70", got)
	}
}

func TestPracticeStateControllerMissingUserIDReturns400(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)

	rec := performJSONRequest(router, http.MethodGet, "/api/practice-states", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
