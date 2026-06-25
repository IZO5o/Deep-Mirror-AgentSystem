package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestSubmitMockTurnCreatesPracticeStates(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSONWithTopics("next question", 72, []string{"Redis 缓存一致性", "Agent 工具调用"}),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "answer"}); err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}

	states, err := s.ListPracticeStates("user_001", "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("states length = %d, want 2", len(states))
	}
	byTopic := practiceStatesByTopic(states)
	redis := byTopic["Redis 缓存一致性"]
	if redis.MasteryScore != 72 || redis.AttemptCount != 1 || redis.Dimension != PracticeDimensionBackendKnowledge {
		t.Fatalf("redis state = %#v, want score=72 attempts=1 dimension=%s", redis, PracticeDimensionBackendKnowledge)
	}
	agentState := byTopic["Agent 工具调用"]
	if agentState.MasteryScore != 72 || agentState.Dimension != PracticeDimensionAgentProject {
		t.Fatalf("agent state = %#v, want score=72 dimension=%s", agentState, PracticeDimensionAgentProject)
	}
}

func TestSubmitMockTurnUpdatesSamePracticeTopicWithSmoothing(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSONWithTopics("next question one", 70, []string{"Redis 缓存一致性"}),
		sampleMockTurnJSONWithTopics("next question two", 90, []string{"Redis 缓存一致性"}),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "first"}); err != nil {
		t.Fatalf("first SubmitMockTurn() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "second"}); err != nil {
		t.Fatalf("second SubmitMockTurn() error = %v", err)
	}

	states, err := s.ListPracticeStates("user_001", "Redis 缓存一致性", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("states length = %d, want 1", len(states))
	}
	state := states[0]
	if state.AttemptCount != 2 {
		t.Fatalf("attempt_count = %d, want 2", state.AttemptCount)
	}
	if state.MasteryScore != 76 {
		t.Fatalf("mastery_score = %d, want 76", state.MasteryScore)
	}
	if state.LastScore != 90 {
		t.Fatalf("last_score = %d, want 90", state.LastScore)
	}
}

func TestPracticeStateFiltersAndGet(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	backend := seedPracticeState(t, s, "user_001", "MySQL 索引", PracticeDimensionBackendKnowledge, 80)
	seedPracticeState(t, s, "user_001", "项目表达", PracticeDimensionCommunication, 60)
	seedPracticeState(t, s, "user_002", "MySQL 索引", PracticeDimensionBackendKnowledge, 30)

	states, err := s.ListPracticeStates("user_001", "", PracticeDimensionBackendKnowledge)
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 || states[0].StateID != backend.StateID {
		t.Fatalf("filtered states = %#v, want only backend state", states)
	}

	got, err := s.GetPracticeState(backend.StateID)
	if err != nil {
		t.Fatalf("GetPracticeState() error = %v", err)
	}
	if got.Topic != "MySQL 索引" || got.MasteryScore != 80 {
		t.Fatalf("practice state = %#v, want MySQL score 80", got)
	}
}

func TestCoachingPlanPromptIncludesPracticeStates(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, memoryID := createCoachingReadyInterview(t, s, runners)
	seedPracticeState(t, s, "user_001", "Redis 缓存一致性", PracticeDimensionBackendKnowledge, 45)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = strings.ReplaceAll(
		sampleCoachingPlanJSON("strategy", "focus"),
		"MEMORY_ID_PLACEHOLDER",
		memoryID,
	)

	if _, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	}); err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}

	prompt := runners[agent.AgentTypeSecondRoundCoach].taskQueries[0]
	for _, want := range []string{"Selected practice_states", "Redis 缓存一致性", `"mastery_score":45`, "selection_reason"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("coaching prompt missing %q", want)
		}
	}
}

func TestMockInterviewPromptIncludesPracticeStates(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	seedPracticeState(t, s, "user_001", "Agent 工具调用", PracticeDimensionAgentProject, 45)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()

	if _, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	}); err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}

	prompt := runners[agent.AgentTypeMockInterviewer].taskQueries[0]
	for _, want := range []string{"Selected practice_states", "Agent 工具调用", `"dimension":"agent_project"`, "selection_reason"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("mock prompt missing %q", want)
		}
	}
}

func TestMockTurnDoesNotCallMemoryCuratorWhenUpdatingPracticeStates(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	beforeMemoryCuratorCalls := runners[agent.AgentTypeMemoryCurator].taskCalls
	beforeMemoryItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSONWithTopics("next question", 75, []string{"Agent 工具调用"}),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "answer"}); err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}

	if runners[agent.AgentTypeMemoryCurator].taskCalls != beforeMemoryCuratorCalls {
		t.Fatalf("memory curator calls = %d, want unchanged %d", runners[agent.AgentTypeMemoryCurator].taskCalls, beforeMemoryCuratorCalls)
	}
	afterMemoryItems, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(afterMemoryItems) != len(beforeMemoryItems) {
		t.Fatalf("memory item count changed from %d to %d", len(beforeMemoryItems), len(afterMemoryItems))
	}
}

func practiceStatesByTopic(states []vo.PracticeStateVO) map[string]vo.PracticeStateVO {
	result := make(map[string]vo.PracticeStateVO, len(states))
	for _, state := range states {
		result[state.Topic] = state
	}
	return result
}

func seedPracticeState(t *testing.T, s *Server, userID string, topic string, dimension string, masteryScore int) PracticeState {
	t.Helper()
	now := int64(1700000000 + masteryScore)
	state := PracticeState{
		StateID:         fmt.Sprintf("state-%d-%d", len([]rune(topic)), masteryScore),
		UserID:          userID,
		Topic:           topic,
		Dimension:       dimension,
		MasteryScore:    masteryScore,
		AttemptCount:    1,
		LastScore:       masteryScore,
		LastFeedback:    "seed feedback",
		LastPracticedAt: now,
		SourceType:      PracticeStateSourceMockTurn,
		SourceID:        "seed-turn",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.db.Create(&state).Error; err != nil {
		t.Fatalf("seed practice state: %v", err)
	}
	return state
}

func sampleMockTurnJSONWithTopics(nextQuestion string, score int, topics []string) string {
	payload := map[string]any{
		"feedback":         "你的回答说明了功能流程，但缺少异常处理和状态流转。",
		"score":            score,
		"follow_up_reason": "根据练习状态继续追问薄弱点。",
		"topic_tags":       topics,
		"next_question":    nextQuestion,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}
