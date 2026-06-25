package server

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

func TestUploadInterviewMediaControllerCreatesQueuedJob(t *testing.T) {
	s := newTestServer(t)
	s.ConfigureTranscription(filepath.Join(t.TempDir(), "media"), nil, nil)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	rec := performMultipartUpload(router, "/api/interviews/"+session.InterviewID+"/media", map[string]string{
		"user_id":  "user_001",
		"language": "zh",
	}, "file", "interview.mp3", "audio/mpeg", []byte("fake audio"))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var result vo.UploadInterviewMediaVO
	decodeOKData(t, rec, &result)
	if result.MediaFile.Status != MediaFileStatusUploaded {
		t.Fatalf("media status = %q, want %q", result.MediaFile.Status, MediaFileStatusUploaded)
	}
	if result.MediaFile.MediaType != MediaTypeAudio {
		t.Fatalf("media type = %q, want %q", result.MediaFile.MediaType, MediaTypeAudio)
	}
	if result.TranscriptionJob.Status != TranscriptionJobStatusQueued {
		t.Fatalf("job status = %q, want %q", result.TranscriptionJob.Status, TranscriptionJobStatusQueued)
	}
	if result.TranscriptionJob.Language != "zh" {
		t.Fatalf("job language = %q, want zh", result.TranscriptionJob.Language)
	}
}

func TestUploadInterviewMediaControllerRejectsMissingFile(t *testing.T) {
	s := newTestServer(t)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("user_id", "user_001"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/interviews/"+session.InterviewID+"/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUploadInterviewMediaControllerRejectsUnsupportedExtension(t *testing.T) {
	s := newTestServer(t)
	s.ConfigureTranscription(filepath.Join(t.TempDir(), "media"), nil, nil)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	rec := performMultipartUpload(router, "/api/interviews/"+session.InterviewID+"/media", map[string]string{
		"user_id": "user_001",
	}, "file", "notes.txt", "text/plain", []byte("not media"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestProcessTranscriptionJobSuccessWritesTranscriptAndUpdatesSession(t *testing.T) {
	s := newTestServer(t)
	fakeASR := &fakeASRClient{text: "面试官：请介绍项目。候选人：我做了 Agent 系统。"}
	s.ConfigureTranscription(filepath.Join(t.TempDir(), "media"), fakeASR, nil)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	result := uploadTestMedia(t, router, session.InterviewID, "interview.wav", "audio/wav")

	if err := s.ProcessTranscriptionJob(context.Background(), result.TranscriptionJob.JobID); err != nil {
		t.Fatalf("ProcessTranscriptionJob() error = %v", err)
	}

	transcript, err := s.GetInterviewTranscript(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterviewTranscript() error = %v", err)
	}
	if transcript.SourceType != TranscriptSourceTypeASRAudio {
		t.Fatalf("source_type = %q, want %q", transcript.SourceType, TranscriptSourceTypeASRAudio)
	}
	if transcript.Content != fakeASR.text {
		t.Fatalf("content = %q, want %q", transcript.Content, fakeASR.text)
	}

	updated, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if updated.Status != InterviewStatusReadyForReview {
		t.Fatalf("status = %q, want %q", updated.Status, InterviewStatusReadyForReview)
	}

	job, err := s.GetTranscriptionJob(result.TranscriptionJob.JobID)
	if err != nil {
		t.Fatalf("GetTranscriptionJob() error = %v", err)
	}
	if job.Status != TranscriptionJobStatusSucceeded {
		t.Fatalf("job status = %q, want %q", job.Status, TranscriptionJobStatusSucceeded)
	}
	if job.TranscriptID != transcript.TranscriptID {
		t.Fatalf("job transcript_id = %q, want %q", job.TranscriptID, transcript.TranscriptID)
	}
}

func TestProcessTranscriptionJobFailureRecordsError(t *testing.T) {
	s := newTestServer(t)
	s.ConfigureTranscription(filepath.Join(t.TempDir(), "media"), &fakeASRClient{err: errors.New("asr down")}, nil)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	result := uploadTestMedia(t, router, session.InterviewID, "interview.mp3", "audio/mpeg")

	if err := s.ProcessTranscriptionJob(context.Background(), result.TranscriptionJob.JobID); err == nil {
		t.Fatalf("ProcessTranscriptionJob() error = nil, want error")
	}

	job, err := s.GetTranscriptionJob(result.TranscriptionJob.JobID)
	if err != nil {
		t.Fatalf("GetTranscriptionJob() error = %v", err)
	}
	if job.Status != TranscriptionJobStatusFailed {
		t.Fatalf("job status = %q, want %q", job.Status, TranscriptionJobStatusFailed)
	}
	if !strings.Contains(job.ErrorMessage, "asr down") {
		t.Fatalf("job error_message = %q, want asr down", job.ErrorMessage)
	}

	updated, err := s.GetInterview(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterview() error = %v", err)
	}
	if updated.Status != InterviewStatusCreated {
		t.Fatalf("status = %q, want %q", updated.Status, InterviewStatusCreated)
	}
}

func TestProcessTranscriptionJobVideoExtractsAudioBeforeASR(t *testing.T) {
	s := newTestServer(t)
	fakeASR := &fakeASRClient{text: "video transcript"}
	fakeExtractor := &fakeAudioExtractor{}
	s.ConfigureTranscription(filepath.Join(t.TempDir(), "media"), fakeASR, fakeExtractor)
	router := NewRouter(s)
	session := createTestInterview(t, s, "user_001")

	result := uploadTestMedia(t, router, session.InterviewID, "screen.mp4", "video/mp4")

	if err := s.ProcessTranscriptionJob(context.Background(), result.TranscriptionJob.JobID); err != nil {
		t.Fatalf("ProcessTranscriptionJob() error = %v", err)
	}
	if fakeExtractor.calls != 1 {
		t.Fatalf("extractor calls = %d, want 1", fakeExtractor.calls)
	}
	if !strings.HasSuffix(fakeASR.lastAudioPath, ".mp3") {
		t.Fatalf("asr audio path = %q, want extracted mp3", fakeASR.lastAudioPath)
	}

	transcript, err := s.GetInterviewTranscript(session.InterviewID)
	if err != nil {
		t.Fatalf("GetInterviewTranscript() error = %v", err)
	}
	if transcript.SourceType != TranscriptSourceTypeASRVideo {
		t.Fatalf("source_type = %q, want %q", transcript.SourceType, TranscriptSourceTypeASRVideo)
	}
}

func uploadTestMedia(t *testing.T, router *gin.Engine, interviewID string, filename string, contentType string) vo.UploadInterviewMediaVO {
	t.Helper()

	rec := performMultipartUpload(router, "/api/interviews/"+interviewID+"/media", map[string]string{
		"user_id":  "user_001",
		"language": "zh",
	}, "file", filename, contentType, []byte("fake media bytes"))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var result vo.UploadInterviewMediaVO
	decodeOKData(t, rec, &result)
	return result
}

func performMultipartUpload(handler http.Handler, target string, fields map[string]string, fileField string, filename string, contentType string, data []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="`+fileField+`"; filename="`+filename+`"`)
	partHeader.Set("Content-Type", contentType)
	part, _ := writer.CreatePart(partHeader)
	_, _ = part.Write(data)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type fakeASRClient struct {
	text          string
	err           error
	lastAudioPath string
}

func (c *fakeASRClient) Provider() string {
	return "fake"
}

func (c *fakeASRClient) Model() string {
	return "fake-asr"
}

func (c *fakeASRClient) Transcribe(_ context.Context, req ASRRequest) (ASRResult, error) {
	c.lastAudioPath = req.AudioPath
	if c.err != nil {
		return ASRResult{}, c.err
	}
	return ASRResult{Text: c.text}, nil
}

type fakeAudioExtractor struct {
	calls int
}

func (e *fakeAudioExtractor) ExtractAudio(_ context.Context, _ string, outputPath string) error {
	e.calls++
	return os.WriteFile(outputPath, []byte("fake extracted audio"), 0644)
}
