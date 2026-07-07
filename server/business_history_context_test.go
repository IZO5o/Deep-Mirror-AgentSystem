package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"

	"agent-web-base/shared"
)

func TestProjectCoachingTurnsToMessagesUsesCompactMetadata(t *testing.T) {
	messages := projectCoachingTurnsToMessages([]CoachingSessionTurn{
		{
			TurnID:    "turn_id_should_not_appear",
			SessionID: "session_id_should_not_appear",
			Role:      CoachingTurnRoleUser,
			Content:   "  I would use idempotent retries.  ",
		},
		{
			TurnID:      "turn_id_should_not_appear",
			SessionID:   "session_id_should_not_appear",
			Role:        CoachingTurnRoleAssistant,
			TurnType:    CoachingTurnTypeFeedback,
			AgentAction: CoachingNextActionAskRetry,
			Score:       72,
			Feedback:    "Add failure compensation.",
			Content:     "Try again with retry boundaries.",
		},
	})

	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if role := shared.GetRoleName(messages[0]); role != "user" {
		t.Fatalf("first role = %q, want user", role)
	}
	if got := messageContentString(t, messages[0]); got != "I would use idempotent retries." {
		t.Fatalf("first content = %q", got)
	}
	if role := shared.GetRoleName(messages[1]); role != "assistant" {
		t.Fatalf("second role = %q, want assistant", role)
	}

	content := messageContentString(t, messages[1])
	if !strings.HasPrefix(content, "[meta type=feedback score=72 action=ask_retry]\n") {
		t.Fatalf("assistant content missing compact metadata prefix: %q", content)
	}
	for _, want := range []string{
		"type=feedback",
		"score=72",
		"action=ask_retry",
		"feedback: Add failure compensation.",
		"Try again with retry boundaries.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("assistant content missing %q: %q", want, content)
		}
	}
	for _, forbidden := range []string{"turn_id_should_not_appear", "session_id_should_not_appear", "turn_id", "session_id"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("assistant content contains forbidden DB JSON field/value %q: %q", forbidden, content)
		}
	}
}

func TestProjectMockTurnsToMessagesUsesCompactMetadata(t *testing.T) {
	messages := projectMockTurnsToMessages([]MockTurn{
		{
			TurnID:     "turn_id_should_not_appear",
			MockID:     "mock_id_should_not_appear",
			Role:       mockTurnRoleUser,
			Content:    "fallback content",
			UserAnswer: "  preferred answer  ",
		},
		{
			TurnID:         "turn_id_should_not_appear",
			MockID:         "mock_id_should_not_appear",
			Role:           mockTurnRoleAssistant,
			TurnType:       mockTurnTypeEvaluationFeedback,
			AgentAction:    mockNextActionAskFollowup,
			Score:          81,
			TopicTags:      `["cache design","consistency"]`,
			Feedback:       "Good tradeoff framing.",
			NextQuestion:   "How would you handle hotspots?",
			Content:        "Let's go deeper.",
			RawAgentOutput: "raw_agent_output_should_not_appear",
		},
	})

	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if role := shared.GetRoleName(messages[0]); role != "user" {
		t.Fatalf("first role = %q, want user", role)
	}
	if got := messageContentString(t, messages[0]); got != "preferred answer" {
		t.Fatalf("first content = %q", got)
	}
	if role := shared.GetRoleName(messages[1]); role != "assistant" {
		t.Fatalf("second role = %q, want assistant", role)
	}

	content := messageContentString(t, messages[1])
	if !strings.HasPrefix(content, "[meta type=evaluation_feedback score=81 action=ask_followup topic=\"cache design,consistency\"]\n") {
		t.Fatalf("assistant content missing compact metadata prefix: %q", content)
	}
	for _, want := range []string{
		"type=evaluation_feedback",
		"score=81",
		"action=ask_followup",
		`topic="cache design,consistency"`,
		"feedback: Good tradeoff framing.",
		"next_question: How would you handle hotspots?",
		"Let's go deeper.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("assistant content missing %q: %q", want, content)
		}
	}
	for _, forbidden := range []string{"mock_id_should_not_appear", "raw_agent_output_should_not_appear", "mock_id", "raw_agent_output"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("assistant content contains forbidden DB JSON field/value %q: %q", forbidden, content)
		}
	}

	duplicate := projectMockTurnsToMessages([]MockTurn{{
		Role:     mockTurnRoleAssistant,
		TurnType: mockTurnTypeEvaluationFeedback,
		Content:  "same feedback",
		Feedback: "same feedback",
	}})
	got := messageContentString(t, duplicate[0])
	if strings.Count(got, "same feedback") != 1 {
		t.Fatalf("duplicate feedback was not compacted: %q", got)
	}
}

func TestCompressBusinessHistoryForPromptSummarizesOlderMessages(t *testing.T) {
	input := make([]shared.OpenAIMessage, 0, 16)
	for i := 0; i < 16; i++ {
		input = append(input, openai.UserMessage(fmt.Sprintf("message %02d with deterministic content %s", i, strings.Repeat("extended detail about architecture tradeoffs, retries, compensation, monitoring, ownership, and follow-up practice notes. ", 8))))
	}

	result := compressBusinessHistoryForPrompt(input, BusinessHistoryCompressionConfig{
		MaxPromptTokens:        3000,
		KeepRecentMessages:     4,
		SummaryCharLimit:       3000,
		MinMessagesToSummarize: 12,
	})

	if !result.SummaryGenerated {
		t.Fatalf("SummaryGenerated = false, want true")
	}
	if result.OriginalMessageCount != 16 {
		t.Fatalf("OriginalMessageCount = %d, want 16", result.OriginalMessageCount)
	}
	if result.CompressedMessageCount != len(result.Messages) {
		t.Fatalf("CompressedMessageCount = %d, len(Messages) = %d", result.CompressedMessageCount, len(result.Messages))
	}
	if len(result.Messages) != 5 {
		t.Fatalf("compressed message count = %d, want summary + 4 recent", len(result.Messages))
	}
	if role := shared.GetRoleName(result.Messages[0]); role != "user" {
		t.Fatalf("summary role = %q, want user", role)
	}

	summary := messageContentString(t, result.Messages[0])
	if !strings.Contains(summary, "Older conversation summary") {
		t.Fatalf("summary missing title: %q", summary)
	}
	for _, want := range []string{"- user: message 00", "- user: message 11"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing older bullet %q: %q", want, summary)
		}
	}
	if strings.Contains(summary, "message 12") {
		t.Fatalf("summary contains recent message: %q", summary)
	}
	for i := 0; i < 4; i++ {
		got := messageContentString(t, result.Messages[i+1])
		if wantPrefix := fmt.Sprintf("message %02d with deterministic content", i+12); !strings.HasPrefix(got, wantPrefix) {
			t.Fatalf("recent message %d = %q, want prefix %q", i, got, wantPrefix)
		}
	}
	if result.OriginalTokenEstimate <= 0 || result.CompressedTokenEstimate <= 0 {
		t.Fatalf("token estimates must be positive: original=%d compressed=%d", result.OriginalTokenEstimate, result.CompressedTokenEstimate)
	}
	if result.CompressedTokenEstimate >= result.OriginalTokenEstimate {
		t.Fatalf("compressed token estimate = %d, want less than original %d", result.CompressedTokenEstimate, result.OriginalTokenEstimate)
	}
}

func TestCompressBusinessHistoryForPromptTruncatesWhenStillOverBudget(t *testing.T) {
	messages := []shared.OpenAIMessage{
		openai.UserMessage(strings.Repeat("older ", 200)),
		openai.AssistantMessage(strings.Repeat("also older ", 200)),
		openai.UserMessage(strings.Repeat("recent user ", 200)),
		openai.AssistantMessage(strings.Repeat("recent assistant ", 200)),
	}

	result := compressBusinessHistoryForPrompt(messages, BusinessHistoryCompressionConfig{
		MaxPromptTokens:        1,
		KeepRecentMessages:     2,
		SummaryCharLimit:       500,
		MinMessagesToSummarize: 2,
	})

	if !result.SummaryGenerated {
		t.Fatalf("SummaryGenerated = false, want true")
	}
	if !result.Truncated {
		t.Fatalf("Truncated = false, want true")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("message count = %d, want latest recent message after dropping summary", len(result.Messages))
	}
	if got := messageContentString(t, result.Messages[0]); !strings.HasPrefix(got, "recent assistant") {
		t.Fatalf("first remaining message = %q, want latest recent assistant", got)
	}
}

func TestCompressBusinessHistoryForPromptTruncatesOversizedRecentWindow(t *testing.T) {
	messages := []shared.OpenAIMessage{
		openai.UserMessage(strings.Repeat("first ", 200)),
		openai.UserMessage(strings.Repeat("second ", 200)),
		openai.UserMessage(strings.Repeat("third ", 200)),
	}

	result := compressBusinessHistoryForPrompt(messages, BusinessHistoryCompressionConfig{
		MaxPromptTokens:        1,
		KeepRecentMessages:     5,
		SummaryCharLimit:       200,
		MinMessagesToSummarize: 10,
	})

	if !result.Truncated {
		t.Fatalf("Truncated = false, want true")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("message count = %d, want 1 latest message", len(result.Messages))
	}
	if got := messageContentString(t, result.Messages[0]); !strings.HasPrefix(got, "third ") {
		t.Fatalf("remaining message = %q, want latest message", got)
	}
}

func TestCompressBusinessHistoryForPromptAvoidsIncreasingTokensUnderBudget(t *testing.T) {
	messages := []shared.OpenAIMessage{
		openai.UserMessage("a"),
		openai.AssistantMessage("b"),
		openai.UserMessage("c"),
	}

	result := compressBusinessHistoryForPrompt(messages, BusinessHistoryCompressionConfig{
		MaxPromptTokens:        10000,
		KeepRecentMessages:     1,
		SummaryCharLimit:       200,
		MinMessagesToSummarize: 2,
	})

	if result.SummaryGenerated {
		t.Fatalf("SummaryGenerated = true, want false when summary would increase under-budget history")
	}
	if len(result.Messages) != len(messages) {
		t.Fatalf("message count = %d, want original %d", len(result.Messages), len(messages))
	}
}

func TestBuildCoachingTurnStaticContextIncludesSelectedMemoryAndPractice(t *testing.T) {
	state := `{"weak_area":"cache invalidation"}`
	selection := MemorySelectionResult{
		MemoryItems: []SelectedMemoryItem{{
			MemoryItem: MemoryItem{
				MemoryID:   "memory-1",
				MemoryType: "weakness",
				SubjectKey: "system_design",
				Content:    "Needs stronger consistency tradeoff framing.",
			},
			Score:           91,
			SelectionReason: "matches current task",
		}},
		PracticeStates: []SelectedPracticeState{{
			PracticeState: PracticeState{
				StateID:      "state-1",
				Topic:        "cache design",
				Dimension:    "system_design",
				MasteryScore: 42,
				LastFeedback: state,
			},
			Score:           87,
			SelectionReason: "recent weak topic",
		}},
		DebugSummary: "selected 1 memory and 1 practice state",
	}

	context := buildCoachingTurnStaticContext(
		CoachingPlan{PlanID: "plan-1", TargetRound: "onsite", OverallStrategy: "Practice tradeoffs."},
		CoachingSession{SessionID: "session-1", ProgressSummary: "Started", AgentPersistentState: &state},
		CoachingTask{TaskID: "task-1", Title: "Cache consistency drill", Description: "Explain invalidation."},
		[]CoachingTask{{TaskID: "task-1", Title: "Cache consistency drill"}},
		selection,
	)

	for _, want := range []string{
		"Coaching plan",
		"Selected memory_items",
		"memory-1",
		"Needs stronger consistency tradeoff framing.",
		"Selected practice_states",
		"cache design",
		selection.DebugSummary,
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("static context missing %q: %s", want, context)
		}
	}
}

func TestBuildBusinessContextTraceSnapshotIncludesCompression(t *testing.T) {
	selection := MemorySelectionResult{
		MemoryItems: []SelectedMemoryItem{{
			MemoryItem: MemoryItem{
				MemoryID: "memory-1",
				Content:  "Practice concise tradeoffs.",
			},
		}},
		DebugSummary: "selected one memory",
	}

	snapshot := buildBusinessContextTraceSnapshot(selection, BusinessHistoryCompressionResult{
		OriginalMessageCount:    12,
		CompressedMessageCount:  5,
		OriginalTokenEstimate:   9000,
		CompressedTokenEstimate: 3000,
		SummaryGenerated:        true,
		Truncated:               false,
	})

	for _, want := range []string{
		"selected_memory_items",
		"history_compression",
		"original_message_count",
		"summary_generated",
	} {
		if !strings.Contains(snapshot, want) {
			t.Fatalf("trace snapshot missing %q: %s", want, snapshot)
		}
	}
}

func messageContentString(t *testing.T, message shared.OpenAIMessage) string {
	t.Helper()
	content := message.GetContent().AsAny()
	value, ok := content.(*string)
	if !ok {
		t.Fatalf("message content is not *string: %T", content)
	}
	return *value
}
