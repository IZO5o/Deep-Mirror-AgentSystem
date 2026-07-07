package agent

import (
	"context"
	"sync"
	"testing"

	"github.com/openai/openai-go/v3"

	"agent-web-base/shared"
)

type testRunner struct {
	options RunOptions
	history []shared.OpenAIMessage
	query   string
}

func (*testRunner) Model() string {
	return "test-model"
}

func (*testRunner) RunTask(context.Context, string) (RunResult, error) {
	return RunResult{}, nil
}

func (r *testRunner) RunStreamingWithHistory(ctx context.Context, history []shared.OpenAIMessage, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction) (RunResult, error) {
	return r.RunStreamingWithContextHistory(ctx, DefaultRunOptions(), history, query, viewCh, confirmCh)
}

func (r *testRunner) RunStreamingWithContextHistory(_ context.Context, options RunOptions, history []shared.OpenAIMessage, query string, _ chan MessageVO, _ chan ConfirmationAction) (RunResult, error) {
	r.options = options
	r.history = history
	r.query = query
	return RunResult{}, nil
}

func TestAgentRegistryResolveDefault(t *testing.T) {
	registry := newTestRegistry(t)

	agentType, profile, err := registry.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\") error = %v", err)
	}
	if agentType != AgentTypeAssistant {
		t.Fatalf("agentType = %q, want %q", agentType, AgentTypeAssistant)
	}
	if profile.SystemPrompt != CodingAgentSystemPrompt {
		t.Fatalf("default profile prompt was not the coding assistant prompt")
	}
}

func TestAgentRegistryResolveSupportedProfiles(t *testing.T) {
	registry := newTestRegistry(t)

	for _, want := range []AgentType{
		AgentTypeAssistant,
		AgentTypeReview,
		AgentTypeMemoryCurator,
		AgentTypeSecondRoundCoach,
		AgentTypeStudyPlanner,
		AgentTypeMockInterviewer,
	} {
		agentType, profile, err := registry.Resolve(string(want))
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", want, err)
		}
		if agentType != want {
			t.Fatalf("agentType = %q, want %q", agentType, want)
		}
		if profile.Type != want {
			t.Fatalf("profile.Type = %q, want %q", profile.Type, want)
		}
		if profile.SystemPrompt == "" {
			t.Fatalf("profile.SystemPrompt for %q is empty", want)
		}
	}
}

func TestAgentRegistryRejectsUnknownType(t *testing.T) {
	registry := newTestRegistry(t)

	if _, _, err := registry.Resolve("unknown"); err == nil {
		t.Fatalf("Resolve(\"unknown\") error = nil, want error")
	}
}

func TestDefaultRunOptionsEnablesPoliciesAndMemory(t *testing.T) {
	options := DefaultRunOptions()

	if !options.ApplyPolicies {
		t.Fatalf("ApplyPolicies = false, want true")
	}
	if !options.UpdateAgentMemory {
		t.Fatalf("UpdateAgentMemory = false, want true")
	}
	if options.SystemContext != "" {
		t.Fatalf("SystemContext = %q, want empty", options.SystemContext)
	}
}

func TestSerializedRunnerForwardsContextHistoryOptions(t *testing.T) {
	inner := &testRunner{}
	runner := &serializedRunner{
		mu:     &sync.Mutex{},
		runner: inner,
	}
	history := []shared.OpenAIMessage{openai.UserMessage("prior")}
	options := RunOptions{
		SystemContext:     "business context",
		ApplyPolicies:     true,
		UpdateAgentMemory: false,
	}
	viewCh := make(chan MessageVO, 1)
	confirmCh := make(chan ConfirmationAction, 1)

	_, err := runner.RunStreamingWithContextHistory(context.Background(), options, history, "query", viewCh, confirmCh)
	if err != nil {
		t.Fatalf("RunStreamingWithContextHistory() error = %v", err)
	}

	if inner.options != options {
		t.Fatalf("options = %+v, want %+v", inner.options, options)
	}
	if len(inner.history) != 1 || contentStringForRegistryTest(t, inner.history[0]) != "prior" {
		t.Fatalf("history was not forwarded: %+v", inner.history)
	}
	if inner.query != "query" {
		t.Fatalf("query = %q, want %q", inner.query, "query")
	}
}

func newTestRegistry(t *testing.T) *AgentRegistry {
	t.Helper()

	registry, err := NewAgentRegistry(AgentTypeAssistant, DefaultAgentProfiles(), func(AgentProfile) Runner {
		return &testRunner{}
	})
	if err != nil {
		t.Fatalf("NewAgentRegistry() error = %v", err)
	}
	return registry
}

func contentStringForRegistryTest(t *testing.T, msg shared.OpenAIMessage) string {
	t.Helper()
	v := msg.GetContent().AsAny()
	s, ok := v.(*string)
	if !ok {
		t.Fatalf("message content is not string: %T", v)
	}
	return *s
}
