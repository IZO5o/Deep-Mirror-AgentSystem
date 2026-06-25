package agent

import (
	"context"
	"testing"

	"agent-web-base/shared"
)

type testRunner struct{}

func (testRunner) Model() string {
	return "test-model"
}

func (testRunner) RunTask(context.Context, string) (RunResult, error) {
	return RunResult{}, nil
}

func (testRunner) RunStreamingWithHistory(context.Context, []shared.OpenAIMessage, string, chan MessageVO, chan ConfirmationAction) (RunResult, error) {
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

func newTestRegistry(t *testing.T) *AgentRegistry {
	t.Helper()

	registry, err := NewAgentRegistry(AgentTypeAssistant, DefaultAgentProfiles(), func(AgentProfile) Runner {
		return testRunner{}
	})
	if err != nil {
		t.Fatalf("NewAgentRegistry() error = %v", err)
	}
	return registry
}
