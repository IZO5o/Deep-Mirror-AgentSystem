package server

import (
	"context"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestGoldenMockStartCreatesOpeningTurnAndTrace(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID, TargetRound: "second_round"})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if mock.Status != MockInterviewStatusWaitingAnswer || mock.FirstQuestion == "" || mock.OverallGoal == "" {
		t.Fatalf("mock = %#v, want waiting with first question", mock)
	}
	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 1 || turns[0].TurnType != mockTurnTypeOpeningQuestion || turns[0].Role != mockTurnRoleAssistant {
		t.Fatalf("turns = %#v, want one opening assistant turn", turns)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockStart,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceBase(t, trace, AgentDecisionTraceStatusSucceeded, agent.AgentTypeMockInterviewer, AgentTraceSourceMockInterview, mock.MockID, AgentTraceStepMockStart)
	if trace.SelectedContextSnapshot == "" || !strings.Contains(trace.SelectedContextSnapshot, "selected_memory_items") {
		t.Fatalf("selected context snapshot is empty or malformed: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "created mock_interview", "created opening mock_turn")
}

func TestGoldenMockFormalAnswerAskFollowupUpdatesPractice(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("继续追问工具失败恢复。", 75),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "我会先讲整体架构。"})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.TurnType != mockTurnTypeFollowupQuestion || turn.NextQuestion != "继续追问工具失败恢复。" {
		t.Fatalf("turn = %#v, want followup next question", turn)
	}
	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 4 {
		t.Fatalf("turns length = %d, want opening/user/evaluation/followup", len(turns))
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 1 || got.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock = %#v, want current_turn=1 waiting", got)
	}
	states, err := s.ListPracticeStates("user_001", "Agent 工具调用", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 || states[0].SourceType != PracticeStateSourceMockTurn {
		t.Fatalf("practice states = %#v, want mock_turn source", states)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	if trace.SelectedContextSnapshot == "" || trace.RawAgentOutput == "" || !strings.Contains(trace.ParsedDecision, mockNextActionAskFollowup) {
		t.Fatalf("trace missing selected/raw/parsed details: %#v", trace)
	}
	assertTraceContainsAction(t, trace, "created mock_turns: 3", "updated mock status", "updated practice_states")
}

func TestGoldenMockTurnTraceAndPromptIncludeSubmitDecisionFields(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("继续追问工具失败恢复。", 75),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "我会先讲整体架构。"}); err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}

	prompt := runners[agent.AgentTypeMockInterviewer].taskQueries[1]
	for _, want := range []string{
		"你是固定的 mock_interviewer Agent",
		"本轮 submit_mode: formal_answer",
		`"visible_message": "给用户看的中文回复"`,
		`"user_intent": "answer|ask_hint|ask_explain|smalltalk|unclear|cancel"`,
		`"state_action": "record_attempt|chat_only|stay_current|cancel"`,
		"不要写入 memory_items",
		"不要调用任何 tools",
		"不要新增 Agent",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("mock turn prompt missing %q\nprompt:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "Do not write long-term memory") {
		t.Fatalf("mock turn prompt should use Chinese memory boundary, got: %s", prompt)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	if !strings.Contains(trace.InputSnapshot, `"submit_mode":"formal_answer"`) {
		t.Fatalf("trace input snapshot missing submit_mode: %s", trace.InputSnapshot)
	}
	for _, want := range []string{
		`"submit_mode":"formal_answer"`,
		`"user_intent":"answer"`,
		`"state_action":"record_attempt"`,
		`"confidence"`,
		`"needs_clarification"`,
		`"visible_message":"继续追问工具失败恢复。"`,
	} {
		if !strings.Contains(trace.ParsedDecision, want) {
			t.Fatalf("trace parsed decision missing %q: %s", want, trace.ParsedDecision)
		}
	}
}

func TestGoldenMockFormalAnswerSwitchTopic(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockSwitchTopicJSON(),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "先讲项目。"})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.TurnType != mockTurnTypeTopicSwitch || turn.AgentAction != mockNextActionSwitchTopic {
		t.Fatalf("turn = %#v, want topic switch", turn)
	}
	if turn.NextQuestion == "" || !strings.Contains(turn.NextQuestion, "Redis") {
		t.Fatalf("next_question = %q, want Redis topic question", turn.NextQuestion)
	}
	states, err := s.ListPracticeStates("user_001", "项目深挖", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("practice states = %#v, want project deep-dive state", states)
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceContainsAction(t, trace, "created mock_turns: 3", "updated practice_states")
	if !strings.Contains(trace.ParsedDecision, mockNextActionSwitchTopic) {
		t.Fatalf("parsed_decision missing switch_topic: %s", trace.ParsedDecision)
	}
}

func TestGoldenMockFormalAnswerCompleteStopsFurtherSubmit(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockCompleteTurnJSON(),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "完整回答"})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.TurnType != mockTurnTypeClosingSummary {
		t.Fatalf("turn type = %q, want closing summary", turn.TurnType)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.Status != MockInterviewStatusCompleted || got.FinalSummary == "" {
		t.Fatalf("mock = %#v, want completed with final summary", got)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "after complete"}); err == nil {
		t.Fatalf("SubmitMockTurn() after complete error = nil, want error")
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceContainsAction(t, trace, "created mock_turns: 3", "updated practice_states", "updated mock status completed")
}

func TestGoldenMockHintRequestKeepsQuestionAndSkipsPractice(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockHintJSON(),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "能提示吗？"})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.TurnType != mockTurnTypeHintRequest || turn.NextQuestion != mock.FirstQuestion {
		t.Fatalf("turn = %#v, want hint with original question", turn)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 0 || got.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock = %#v, want waiting without counted turn", got)
	}
	assertNoPracticeStates(t, s, "user_001")

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceContainsAction(t, trace, "created mock_turns: 2", "skipped practice update")
	assertTraceNotContainsAction(t, trace, "updated practice_states")
}

func TestGoldenMockCancelStopsFurtherSubmit(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockCancelJSON(),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "取消"})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.TurnType != mockTurnTypeCancellationSummary {
		t.Fatalf("turn type = %q, want cancellation summary", turn.TurnType)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.Status != MockInterviewStatusCancelled {
		t.Fatalf("mock status = %q, want cancelled", got.Status)
	}
	assertNoPracticeStates(t, s, "user_001")
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "after cancel"}); err == nil {
		t.Fatalf("SubmitMockTurn() after cancel error = nil, want error")
	}

	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	assertTraceContainsAction(t, trace, "created mock_turns: 2", "updated mock status")
	assertTraceNotContainsAction(t, trace, "updated practice_states")
}

func sampleMockCancelJSON() string {
	return `{
  "input_type": "cancel",
  "agent_message": "本次模拟面试已取消。",
  "score": 0,
  "feedback": "",
  "topic": "",
  "weakness_tags": [],
  "next_action": "wait_for_answer",
  "should_update_practice_state": false,
  "practice_updates": [],
  "should_complete_mock": false,
  "follow_up_reason": ""
}`
}
