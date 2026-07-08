package server

import (
	"net/http"
	"testing"

	"agent-web-base/vo"
)

func TestPracticeGoalControllerCRUDFlow(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)

	createRec := performJSONRequest(router, http.MethodPost, "/api/practice-goals", `{"user_id":"user_001","company_name":"ByteDance","job_title":"Backend Engineer","focus_topics":["缓存一致性"],"remaining_days":3}`)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body=%s", createRec.Code, createRec.Body.String())
	}
	var goal vo.PracticeGoalVO
	decodeOKData(t, createRec, &goal)
	if goal.GoalID == "" || goal.Status != PracticeGoalStatusActive {
		t.Fatalf("goal = %#v, want active goal", goal)
	}

	listRec := performJSONRequest(router, http.MethodGet, "/api/practice-goals?user_id=user_001&status=active", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", listRec.Code, listRec.Body.String())
	}
	var goals []vo.PracticeGoalVO
	decodeOKData(t, listRec, &goals)
	if len(goals) != 1 || goals[0].GoalID != goal.GoalID {
		t.Fatalf("goals = %#v", goals)
	}

	getRec := performJSONRequest(router, http.MethodGet, "/api/practice-goals/"+goal.GoalID, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body=%s", getRec.Code, getRec.Body.String())
	}

	updateRec := performJSONRequest(router, http.MethodPatch, "/api/practice-goals/"+goal.GoalID, `{"job_title":"Senior Backend Engineer","focus_topics":["系统设计"]}`)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body=%s", updateRec.Code, updateRec.Body.String())
	}
	var updated vo.PracticeGoalVO
	decodeOKData(t, updateRec, &updated)
	if updated.JobTitle != "Senior Backend Engineer" || len(updated.FocusTopics) != 1 {
		t.Fatalf("updated = %#v", updated)
	}

	archiveRec := performJSONRequest(router, http.MethodPost, "/api/practice-goals/"+goal.GoalID+"/archive", "")
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("archive status = %d, want 200; body=%s", archiveRec.Code, archiveRec.Body.String())
	}
	var archived vo.PracticeGoalVO
	decodeOKData(t, archiveRec, &archived)
	if archived.Status != PracticeGoalStatusArchived {
		t.Fatalf("archived status = %q", archived.Status)
	}
}

func TestPracticeGoalControllerValidationAndNotFound(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)

	missingUser := performJSONRequest(router, http.MethodGet, "/api/practice-goals", "")
	if missingUser.Code != http.StatusBadRequest {
		t.Fatalf("missing user status = %d, want 400; body=%s", missingUser.Code, missingUser.Body.String())
	}

	invalidCreate := performJSONRequest(router, http.MethodPost, "/api/practice-goals", `{"user_id":"user_001"}`)
	if invalidCreate.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status = %d, want 400; body=%s", invalidCreate.Code, invalidCreate.Body.String())
	}

	notFound := performJSONRequest(router, http.MethodGet, "/api/practice-goals/missing_goal", "")
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d, want 404; body=%s", notFound.Code, notFound.Body.String())
	}
}
