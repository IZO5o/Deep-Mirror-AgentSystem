package agent

import (
	"context"
	"fmt"
	"sync"

	"agent-web-base/shared"
)

type AgentType string

const (
	AgentTypeAssistant        AgentType = "assistant"
	AgentTypeReview           AgentType = "review"
	AgentTypeMemoryCurator    AgentType = "memory_curator"
	AgentTypeSecondRoundCoach AgentType = "second_round_coach"
	AgentTypeStudyPlanner     AgentType = "study_planner"
	AgentTypeMockInterviewer  AgentType = "mock_interviewer"
)

type AgentProfile struct {
	Type         AgentType
	Name         string
	Description  string
	SystemPrompt string
}

// RunOptions controls per-call context and commit behavior. Callers should
// start from DefaultRunOptions unless intentionally disabling policy or memory updates.
type RunOptions struct {
	SystemContext     string
	ApplyPolicies     bool
	UpdateAgentMemory bool
}

func DefaultRunOptions() RunOptions {
	return RunOptions{
		ApplyPolicies:     true,
		UpdateAgentMemory: true,
	}
}

type Runner interface {
	Model() string
	RunTask(ctx context.Context, query string) (RunResult, error)
	RunStreamingWithHistory(ctx context.Context, history []shared.OpenAIMessage, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction) (RunResult, error)
	RunStreamingWithContextHistory(ctx context.Context, options RunOptions, history []shared.OpenAIMessage, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction) (RunResult, error)
}

type AgentRegistry struct {
	defaultType AgentType
	profiles    map[AgentType]AgentProfile
	runners     map[AgentType]Runner
	runMu       sync.Mutex
}

func DefaultAgentProfiles() []AgentProfile {
	return []AgentProfile{
		{
			Type:         AgentTypeAssistant,
			Name:         "Assistant",
			Description:  "Default coding assistant for backward compatibility.",
			SystemPrompt: CodingAgentSystemPrompt,
		},
		{
			Type:         AgentTypeReview,
			Name:         "Interview Review Agent",
			Description:  "Analyzes interview transcripts, JD, and resume context.",
			SystemPrompt: InterviewReviewAgentSystemPrompt,
		},
		{
			Type:         AgentTypeMemoryCurator,
			Name:         "Memory Curator Agent",
			Description:  "Produces candidate long-term memories for user confirmation.",
			SystemPrompt: MemoryCuratorAgentSystemPrompt,
		},
		{
			Type:         AgentTypeSecondRoundCoach,
			Name:         "Second Round Coach Agent",
			Description:  "Creates second-round preparation plans and improved answers.",
			SystemPrompt: SecondRoundCoachAgentSystemPrompt,
		},
		{
			Type:         AgentTypeStudyPlanner,
			Name:         "Study Planner Agent",
			Description:  "Compatibility alias for second-round preparation planning.",
			SystemPrompt: SecondRoundCoachAgentSystemPrompt,
		},
		{
			Type:         AgentTypeMockInterviewer,
			Name:         "Mock Interviewer Agent",
			Description:  "Runs multi-turn mock interviews and follow-up questions.",
			SystemPrompt: MockInterviewerAgentSystemPrompt,
		},
	}
}

func NewAgentRegistry(defaultType AgentType, profiles []AgentProfile, factory func(AgentProfile) Runner) (*AgentRegistry, error) {
	if factory == nil {
		return nil, fmt.Errorf("agent factory is nil")
	}

	r := &AgentRegistry{
		defaultType: defaultType,
		profiles:    make(map[AgentType]AgentProfile, len(profiles)),
		runners:     make(map[AgentType]Runner, len(profiles)),
	}
	for _, profile := range profiles {
		if profile.Type == "" {
			return nil, fmt.Errorf("agent profile type is empty")
		}
		if _, exists := r.profiles[profile.Type]; exists {
			return nil, fmt.Errorf("duplicate agent type %q", profile.Type)
		}
		runner := factory(profile)
		if runner == nil {
			return nil, fmt.Errorf("agent factory returned nil for type %q", profile.Type)
		}
		r.profiles[profile.Type] = profile
		r.runners[profile.Type] = &serializedRunner{
			mu:     &r.runMu,
			runner: runner,
		}
	}
	if _, ok := r.profiles[defaultType]; !ok {
		return nil, fmt.Errorf("default agent type %q is not registered", defaultType)
	}
	return r, nil
}

func (r *AgentRegistry) Resolve(rawType string) (AgentType, AgentProfile, error) {
	agentType := AgentType(rawType)
	if agentType == "" {
		agentType = r.defaultType
	}
	profile, ok := r.profiles[agentType]
	if !ok {
		return "", AgentProfile{}, fmt.Errorf("unknown agent_type %q", rawType)
	}
	return agentType, profile, nil
}

func (r *AgentRegistry) Get(rawType string) (AgentType, Runner, error) {
	agentType, _, err := r.Resolve(rawType)
	if err != nil {
		return "", nil, err
	}
	return agentType, r.runners[agentType], nil
}

type serializedRunner struct {
	mu     *sync.Mutex
	runner Runner
}

func (r *serializedRunner) Model() string {
	return r.runner.Model()
}

func (r *serializedRunner) RunTask(ctx context.Context, query string) (RunResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runner.RunTask(ctx, query)
}

func (r *serializedRunner) RunStreamingWithHistory(ctx context.Context, history []shared.OpenAIMessage, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction) (RunResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runner.RunStreamingWithHistory(ctx, history, query, viewCh, confirmCh)
}

func (r *serializedRunner) RunStreamingWithContextHistory(ctx context.Context, options RunOptions, history []shared.OpenAIMessage, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction) (RunResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runner.RunStreamingWithContextHistory(ctx, options, history, query, viewCh, confirmCh)
}
