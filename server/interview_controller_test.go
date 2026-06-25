package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-web-base/vo"
)

func TestInterviewControllerFlow(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)

	createBody := `{"user_id":"user_001","company_name":"Acme","job_title":"Backend Engineer","interview_round":"first_round","interview_type":"technical"}`
	createRec := performJSONRequest(router, http.MethodPost, "/api/interviews", createBody)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusOK, createRec.Body.String())
	}

	var created vo.InterviewSessionVO
	decodeOKData(t, createRec, &created)
	if created.InterviewID == "" {
		t.Fatalf("created interview_id is empty")
	}
	if created.Status != InterviewStatusCreated {
		t.Fatalf("created status = %q, want %q", created.Status, InterviewStatusCreated)
	}

	listRec := performJSONRequest(router, http.MethodGet, "/api/interviews?user_id=user_001", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var list []vo.InterviewSessionVO
	decodeOKData(t, listRec, &list)
	if len(list) != 1 {
		t.Fatalf("list length = %d, want 1", len(list))
	}

	detailRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+created.InterviewID, "")
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d; body=%s", detailRec.Code, http.StatusOK, detailRec.Body.String())
	}
	var detail vo.InterviewSessionVO
	decodeOKData(t, detailRec, &detail)
	if detail.InterviewID != created.InterviewID {
		t.Fatalf("detail interview_id = %q, want %q", detail.InterviewID, created.InterviewID)
	}

	transcriptBody := `{"user_id":"user_001","content":"面试官：请介绍项目。候选人：..."}`
	upsertRec := performJSONRequest(router, http.MethodPut, "/api/interviews/"+created.InterviewID+"/transcript", transcriptBody)
	if upsertRec.Code != http.StatusOK {
		t.Fatalf("upsert transcript status = %d, want %d; body=%s", upsertRec.Code, http.StatusOK, upsertRec.Body.String())
	}
	var transcript vo.InterviewTranscriptVO
	decodeOKData(t, upsertRec, &transcript)
	if transcript.SourceType != TranscriptSourceTypeManualText {
		t.Fatalf("source_type = %q, want %q", transcript.SourceType, TranscriptSourceTypeManualText)
	}
	if transcript.Language != DefaultTranscriptLanguage {
		t.Fatalf("language = %q, want %q", transcript.Language, DefaultTranscriptLanguage)
	}

	getTranscriptRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+created.InterviewID+"/transcript", "")
	if getTranscriptRec.Code != http.StatusOK {
		t.Fatalf("get transcript status = %d, want %d; body=%s", getTranscriptRec.Code, http.StatusOK, getTranscriptRec.Body.String())
	}
	var gotTranscript vo.InterviewTranscriptVO
	decodeOKData(t, getTranscriptRec, &gotTranscript)
	if gotTranscript.TranscriptID != transcript.TranscriptID {
		t.Fatalf("transcript_id = %q, want %q", gotTranscript.TranscriptID, transcript.TranscriptID)
	}

	updatedDetailRec := performJSONRequest(router, http.MethodGet, "/api/interviews/"+created.InterviewID, "")
	var updatedDetail vo.InterviewSessionVO
	decodeOKData(t, updatedDetailRec, &updatedDetail)
	if updatedDetail.Status != InterviewStatusReadyForReview {
		t.Fatalf("updated status = %q, want %q", updatedDetail.Status, InterviewStatusReadyForReview)
	}
}

func TestCreateInterviewControllerMissingRequiredFieldReturns400(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)

	rec := performJSONRequest(router, http.MethodPost, "/api/interviews", `{"user_id":"user_001"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpsertInterviewTranscriptControllerMissingContentReturns400(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)

	session := createTestInterview(t, s, "user_001")
	rec := performJSONRequest(router, http.MethodPut, "/api/interviews/"+session.InterviewID+"/transcript", `{"user_id":"user_001"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func performJSONRequest(handler http.Handler, method string, target string, body string) *httptest.ResponseRecorder {
	var requestBody *bytes.Reader
	if body == "" {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, target, requestBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeOKData(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()

	var response struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	if response.Code != 0 {
		t.Fatalf("response code = %d, want 0; msg=%s", response.Code, response.Msg)
	}
	if err := json.Unmarshal(response.Data, out); err != nil {
		t.Fatalf("decode data: %v; data=%s", err, string(response.Data))
	}
}
