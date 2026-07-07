package context

import (
	"context"
	"testing"

	"github.com/openai/openai-go/v3"

	"agent-web-base/shared"
)

type fakeEngineMemory struct {
	updates int
}

func (m *fakeEngineMemory) String() string {
	return "memory"
}

func (m *fakeEngineMemory) Update(_ context.Context, _ []shared.OpenAIMessage) error {
	m.updates++
	return nil
}

type fakeEnginePolicy struct {
	applies int
}

func (p *fakeEnginePolicy) Name() string {
	return "fake"
}

func (p *fakeEnginePolicy) ShouldApply(context.Context, *Engine) bool {
	return true
}

func (p *fakeEnginePolicy) Apply(_ context.Context, e *Engine) (PolicyResult, error) {
	p.applies++
	return PolicyResult{Messages: e.messages, ContextTokens: e.contextTokens}, nil
}

func TestBuildRequestMessagesWithSystemContextAppendsContext(t *testing.T) {
	engine := NewContextEngine(nil, nil)
	engine.Init("base system", TokenBudget{})
	engine.SetMessages([]shared.OpenAIMessage{openai.UserMessage("previous")})

	got := engine.BuildRequestMessagesWithSystemContext("Company: Acme")

	if len(got) != 2 {
		t.Fatalf("message count = %d, want 2", len(got))
	}
	if got[0].OfSystem == nil {
		t.Fatalf("first message should be system")
	}
	systemContent := contentString(t, got[0])
	if want := "base system\n\n## Business Context\nCompany: Acme"; systemContent != want {
		t.Fatalf("system content = %q, want %q", systemContent, want)
	}
	if got[1].OfUser == nil {
		t.Fatalf("second message should be existing user history")
	}
	if got := contentString(t, got[1]); got != "previous" {
		t.Fatalf("history content = %q, want %q", got, "previous")
	}

	withoutContext := engine.BuildRequestMessagesWithSystemContext("")
	if got := contentString(t, withoutContext[0]); got != "base system" {
		t.Fatalf("system content without context = %q, want %q", got, "base system")
	}
}

func TestCommitTurnWithOptionsSeparatesPoliciesAndMemory(t *testing.T) {
	t.Run("policies only", func(t *testing.T) {
		mem := &fakeEngineMemory{}
		policy := &fakeEnginePolicy{}
		engine := NewContextEngine(mem, []Policy{policy})
		draft := engine.StartTurn(openai.UserMessage("hello"))

		err := engine.CommitTurnWithOptions(context.Background(), draft, Usage{}, CommitOptions{
			ApplyPolicies:     true,
			UpdateAgentMemory: false,
		})
		if err != nil {
			t.Fatalf("CommitTurnWithOptions() error = %v", err)
		}
		if policy.applies != 1 {
			t.Fatalf("policy applies = %d, want 1", policy.applies)
		}
		if mem.updates != 0 {
			t.Fatalf("memory updates = %d, want 0", mem.updates)
		}
	})

	t.Run("memory only", func(t *testing.T) {
		mem := &fakeEngineMemory{}
		policy := &fakeEnginePolicy{}
		engine := NewContextEngine(mem, []Policy{policy})
		draft := engine.StartTurn(openai.UserMessage("hello"))

		err := engine.CommitTurnWithOptions(context.Background(), draft, Usage{}, CommitOptions{
			ApplyPolicies:     false,
			UpdateAgentMemory: true,
		})
		if err != nil {
			t.Fatalf("CommitTurnWithOptions() error = %v", err)
		}
		if policy.applies != 0 {
			t.Fatalf("policy applies = %d, want 0", policy.applies)
		}
		if mem.updates != 1 {
			t.Fatalf("memory updates = %d, want 1", mem.updates)
		}
	})
}
