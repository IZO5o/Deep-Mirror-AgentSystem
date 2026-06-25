//go:build real_hardening

package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"agent-web-base/agent"
	ctxengine "agent-web-base/agent/context"
	"agent-web-base/shared"
	"agent-web-base/vo"
)

func TestStep12ARealLongTranscriptSegmentedReviewHardening(t *testing.T) {
	s, runners := newStep12ARealServerWithCountingRunners(t)

	transcriptBytes, err := os.ReadFile(filepath.Join("..", "testdata", "step11_long_interview_transcript.txt"))
	if err != nil {
		t.Fatalf("read constructed long transcript: %v", err)
	}
	transcript := strings.TrimSpace(string(transcriptBytes))
	if len([]rune(transcript)) <= longTranscriptThresholdChars {
		t.Fatalf("transcript char_count=%d, want > %d", len([]rune(transcript)), longTranscriptThresholdChars)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	userID := "u_real_hardening_step12a"
	session, err := s.CreateInterview(vo.CreateInterviewReq{
		UserID:         userID,
		CompanyName:    "Step12A 长文本硬化验证公司",
		JobTitle:       "Backend Agent Engineer",
		InterviewRound: "first_round",
		InterviewType:  "technical",
	})
	if err != nil {
		t.Fatalf("CreateInterview() error = %v", err)
	}
	if _, err := s.UpsertInterviewTranscript(session.InterviewID, vo.UpsertInterviewTranscriptReq{
		UserID:     userID,
		Content:    transcript,
		SourceType: TranscriptSourceTypeManualText,
		Language:   "zh",
	}); err != nil {
		t.Fatalf("UpsertInterviewTranscript() error = %v", err)
	}

	report, err := s.TriggerInterviewReview(ctx, session.InterviewID)
	if err != nil {
		t.Fatalf("TriggerInterviewReview() error = %v; status=%s raw=%s", err, report.Status, step12AClipRunes(report.RawAgentOutput, 1000))
	}
	if report.Status != InterviewReviewStatusGenerated {
		t.Fatalf("report status=%s, want %s", report.Status, InterviewReviewStatusGenerated)
	}

	var segments []TranscriptSegment
	if err := s.db.Where("interview_id = ?", session.InterviewID).Order("sequence asc").Find(&segments).Error; err != nil {
		t.Fatalf("query transcript_segments error = %v", err)
	}
	if len(segments) < 2 {
		t.Fatalf("segments count=%d, want >=2", len(segments))
	}
	for _, segment := range segments {
		if segment.Status != TranscriptSegmentStatusExtracted {
			t.Fatalf("segment %d status=%s error=%s raw=%s", segment.Sequence, segment.Status, segment.ErrorMessage, step12AClipRunes(segment.RawAgentOutput, 500))
		}
	}

	reviewCalls := runners[agent.AgentTypeReview].taskCalls
	retryCount := reviewCalls - len(segments) - 1
	if retryCount < 0 {
		retryCount = 0
	}
	t.Logf("STEP12A_REAL_HARDENING segments=%d status_counts=%s review_task_calls=%d retry_count=%d segment_max_chars=%d segment_overlap_chars=%d",
		len(segments), step12ASegmentStatusCounts(segments), reviewCalls, retryCount, segmentMaxChars, segmentOverlapChars)
}

func newStep12ARealServerWithCountingRunners(t *testing.T) (*Server, map[agent.AgentType]*countingRunner) {
	t.Helper()

	_ = godotenv.Load("../.env")
	appConf, err := shared.LoadAppConfig("../config.json")
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	model := appConf.LLMProviders.FrontModel
	if model.ApiKey == "" || model.BaseURL == "" || model.Model == "" {
		t.Fatalf("real_hardening requires OPENAI_API_KEY, OPENAI_BASE_URL, and OPENAI_MODEL")
	}

	db, err := InitDB(filepath.Join(t.TempDir(), "step12a-real-hardening.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	runners := map[agent.AgentType]*countingRunner{}
	registry, err := agent.NewAgentRegistry(agent.AgentTypeAssistant, agent.DefaultAgentProfiles(), func(profile agent.AgentProfile) agent.Runner {
		inner := agent.NewAgent(
			model,
			profile.SystemPrompt,
			agent.ToolConfirmConfig{},
			nil,
			nil,
			ctxengine.NewContextEngine(nil, nil),
		)
		runner := &countingRunner{inner: inner}
		runners[profile.Type] = runner
		return runner
	})
	if err != nil {
		t.Fatalf("NewAgentRegistry() error = %v", err)
	}
	return NewServer(db, registry), runners
}

type countingRunner struct {
	inner     agent.Runner
	taskCalls int
}

func (r *countingRunner) Model() string {
	return r.inner.Model()
}

func (r *countingRunner) RunTask(ctx context.Context, query string) (agent.RunResult, error) {
	r.taskCalls++
	return r.inner.RunTask(ctx, query)
}

func (r *countingRunner) RunStreamingWithHistory(ctx context.Context, history []shared.OpenAIMessage, query string, ch chan agent.MessageVO, confirmCh chan agent.ConfirmationAction) (agent.RunResult, error) {
	return r.inner.RunStreamingWithHistory(ctx, history, query, ch, confirmCh)
}

func step12ASegmentStatusCounts(segments []TranscriptSegment) string {
	counts := map[string]int{}
	for _, segment := range segments {
		counts[segment.Status]++
	}
	parts := make([]string, 0, len(counts))
	for status, count := range counts {
		parts = append(parts, status+":"+step12AItoa(count))
	}
	return strings.Join(parts, ",")
}

func step12AClipRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func step12AItoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
