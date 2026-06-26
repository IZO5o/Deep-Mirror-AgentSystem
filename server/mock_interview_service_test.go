package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

func TestStartMockInterviewSuccess(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "unrelated-mock-memory",
		UserID:     "user_001",
		MemoryType: MemoryTypeCompanyProfile,
		SubjectKey: "company:Unrelated",
		Content:    "Unrelated company context should not enter mock prompt.",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
	})
	seedSelectorPracticeState(t, s, PracticeState{
		StateID:      "mock-low-practice",
		UserID:       "user_001",
		Topic:        "Redis consistency",
		Dimension:    PracticeDimensionBackendKnowledge,
		MasteryScore: 35,
		LastScore:    50,
		LastFeedback: "Need clearer Redis consistency tradeoffs.",
		AttemptCount: 2,
	})
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID:      "user_001",
		PlanID:      planID,
		TargetRound: "second_round",
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if mock.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock status = %q, want %q", mock.Status, MockInterviewStatusWaitingAnswer)
	}
	if mock.FirstQuestion == "" || mock.OverallGoal == "" {
		t.Fatalf("mock fields empty: first_question=%q overall_goal=%q", mock.FirstQuestion, mock.OverallGoal)
	}
	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 1 || turns[0].TurnType != mockTurnTypeOpeningQuestion || turns[0].Role != mockTurnRoleAssistant {
		t.Fatalf("opening turns = %#v, want one assistant opening question", turns)
	}

	prompt := runners[agent.AgentTypeMockInterviewer].taskQueries[0]
	for _, want := range []string{"review summary", "review question", "Selected memory_items", "Selected practice_states", "selection_reason", "score", "Redis consistency weakness", "mock-low-practice", planID, "补齐 Redis 缓存一致性回答"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("mock start prompt missing %q", want)
		}
	}
	if strings.Contains(prompt, "Unrelated company context") {
		t.Fatalf("mock start prompt included unrelated memory")
	}
}

func TestStartMockInterviewResumesActiveSession(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()

	first, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("first StartMockInterview() error = %v", err)
	}
	second, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("second StartMockInterview() error = %v", err)
	}
	if second.MockID != first.MockID {
		t.Fatalf("resumed mock_id = %q, want %q", second.MockID, first.MockID)
	}
	if runners[agent.AgentTypeMockInterviewer].taskCalls != 1 {
		t.Fatalf("mock interviewer calls = %d, want 1", runners[agent.AgentTypeMockInterviewer].taskCalls)
	}
}

func TestStartMockInterviewRequiresReviewedInterview(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session := createTestInterview(t, s, "user_001")

	if _, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
	}); err == nil {
		t.Fatalf("StartMockInterview() error = nil, want error")
	}
	if runners[agent.AgentTypeMockInterviewer].taskCalls != 0 {
		t.Fatalf("mock interviewer calls = %d, want 0", runners[agent.AgentTypeMockInterviewer].taskCalls)
	}
}

func TestStartMockInterviewRejectsWrongPlan(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, _ := createMockReadyInterview(t, s, runners)

	other := createTestInterview(t, s, "user_001")
	plan := CoachingPlan{
		PlanID:      "other-plan",
		UserID:      "user_001",
		InterviewID: other.InterviewID,
		Status:      CoachingPlanStatusGenerated,
	}
	if err := s.db.Create(&plan).Error; err != nil {
		t.Fatalf("seed other plan: %v", err)
	}

	if _, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: plan.PlanID,
	}); err == nil {
		t.Fatalf("StartMockInterview() error = nil, want error")
	}
}

func TestSubmitMockTurnUsesFirstQuestionAndThenNextQuestion(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	seedSelectorPracticeState(t, s, PracticeState{
		StateID:      "turn-low-practice",
		UserID:       "user_001",
		Topic:        "Agent 工具调用",
		Dimension:    PracticeDimensionAgentProject,
		MasteryScore: 30,
		LastScore:    55,
		LastFeedback: "Tool call recovery answer is weak.",
		AttemptCount: 3,
	})
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("next question one", 72),
		sampleMockTurnJSON("next question two", 130),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID:      "user_001",
		PlanID:      planID,
		TargetRound: "second_round",
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	firstTurn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "first answer"})
	if err != nil {
		t.Fatalf("first SubmitMockTurn() error = %v", err)
	}
	if firstTurn.InterviewerQuestion != mock.FirstQuestion {
		t.Fatalf("first interviewer_question = %q, want first_question %q", firstTurn.InterviewerQuestion, mock.FirstQuestion)
	}
	if firstTurn.NextQuestion != "next question one" {
		t.Fatalf("next_question = %q, want %q", firstTurn.NextQuestion, "next question one")
	}
	firstTurnPrompt := runners[agent.AgentTypeMockInterviewer].taskQueries[1]
	for _, want := range []string{"Selected memory_items", "Selected practice_states", "selection_reason", "turn-low-practice"} {
		if !strings.Contains(firstTurnPrompt, want) {
			t.Fatalf("mock turn prompt missing %q", want)
		}
	}

	secondTurn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "second answer"})
	if err != nil {
		t.Fatalf("second SubmitMockTurn() error = %v", err)
	}
	if secondTurn.InterviewerQuestion != "next question one" {
		t.Fatalf("second interviewer_question = %q, want %q", secondTurn.InterviewerQuestion, "next question one")
	}
	if secondTurn.Score != 100 {
		t.Fatalf("clamped score = %d, want 100", secondTurn.Score)
	}

	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 2 {
		t.Fatalf("current_turn = %d, want 2", got.CurrentTurn)
	}
}

func TestSubmitMockTurnOmittedSubmitModeDefaultsFormalAndUpdatesPractice(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("继续追问工具失败恢复。", 76),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "我会先介绍项目背景，再讲工具失败恢复。"})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.Score != 76 || turn.Feedback == "" {
		t.Fatalf("turn score/feedback = %d/%q, want formal scoring", turn.Score, turn.Feedback)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 1 {
		t.Fatalf("current_turn = %d, want 1 for omitted submit_mode formal answer", got.CurrentTurn)
	}
	states, err := s.ListPracticeStates("user_001", "Agent 工具调用", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 || states[0].AttemptCount != 1 || states[0].LastScore != 76 {
		t.Fatalf("practice states = %#v, want one updated state", states)
	}
}

func TestSubmitMockTurnChatModeOffRecordDoesNotScoreOrAdvance(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockChatOnlyJSON("可以，我先解释一下这题在看什么，然后我们回到原题。", "ask_explain", 91),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{
		Answer:     "这题能解释一下吗？",
		SubmitMode: "chat",
	})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.Content != "可以，我先解释一下这题在看什么，然后我们回到原题。" {
		t.Fatalf("content = %q, want visible message", turn.Content)
	}
	if turn.Score != 0 || turn.Feedback != "" {
		t.Fatalf("score/feedback = %d/%q, want cleared off-record metadata", turn.Score, turn.Feedback)
	}
	if turn.NextQuestion != mock.FirstQuestion {
		t.Fatalf("next_question = %q, want current question %q", turn.NextQuestion, mock.FirstQuestion)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 0 || got.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock = %#v, want current_turn unchanged and waiting", got)
	}
	assertNoPracticeStates(t, s, "user_001")
	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("turns length = %d, want opening/user/assistant", len(turns))
	}
	if turns[2].Score != 0 || turns[2].Feedback != "" {
		t.Fatalf("assistant score/feedback = %d/%q, want no formal scoring metadata", turns[2].Score, turns[2].Feedback)
	}
}

func TestSubmitMockTurnChatSmalltalkAndUnclearSkipPractice(t *testing.T) {
	for _, tc := range []struct {
		name       string
		intent     string
		visibleMsg string
	}{
		{name: "smalltalk", intent: "smalltalk", visibleMsg: "没问题，我们继续保持当前问题。"},
		{name: "unclear", intent: "unclear", visibleMsg: "我还不确定你的意思，请直接回答当前问题或说明想要提示。"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, runners := newTestServerWithFakeAgents(t)
			session, planID := createMockReadyInterview(t, s, runners)
			runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
				sampleMockStartJSON(),
				sampleMockChatOnlyJSON(tc.visibleMsg, tc.intent, 88),
			}
			mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
			if err != nil {
				t.Fatalf("StartMockInterview() error = %v", err)
			}
			turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{
				Answer:     "哈哈",
				SubmitMode: "chat",
			})
			if err != nil {
				t.Fatalf("SubmitMockTurn() error = %v", err)
			}
			if turn.Score != 0 || turn.Feedback != "" {
				t.Fatalf("score/feedback = %d/%q, want cleared", turn.Score, turn.Feedback)
			}
			got, err := s.GetMockInterview(mock.MockID)
			if err != nil {
				t.Fatalf("GetMockInterview() error = %v", err)
			}
			if got.CurrentTurn != 0 {
				t.Fatalf("current_turn = %d, want unchanged", got.CurrentTurn)
			}
			assertNoPracticeStates(t, s, "user_001")
		})
	}
}

func TestSubmitMockTurnFormalNonAnswerRecordAttemptDoesNotScoreOrUpdatePractice(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockFormalHintWithBadScoreJSON(),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{
		Answer:     "能给一点提示吗？",
		SubmitMode: "formal_answer",
	})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.Score != 0 || turn.Feedback != "" {
		t.Fatalf("score/feedback = %d/%q, want cleared for formal non-answer", turn.Score, turn.Feedback)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 0 {
		t.Fatalf("current_turn = %d, want unchanged", got.CurrentTurn)
	}
	assertNoPracticeStates(t, s, "user_001")
}

func TestSubmitMockTurnParseFailureDoesNotWriteTurn(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{sampleMockStartJSON(), "not json"}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}

	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "bad"}); err == nil {
		t.Fatalf("SubmitMockTurn() error = nil, want error")
	}

	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("turns length = %d, want 3", len(turns))
	}
	if turns[2].TurnType != mockTurnTypeError {
		t.Fatalf("last turn type = %q, want %q", turns[2].TurnType, mockTurnTypeError)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.Status != MockInterviewStatusFailed {
		t.Fatalf("mock status = %q, want %q", got.Status, MockInterviewStatusFailed)
	}
	if got.RawAgentOutput != "not json" {
		t.Fatalf("raw_agent_output = %q, want %q", got.RawAgentOutput, "not json")
	}
}

func TestSubmitMockTurnAgentFailureDoesNotWriteTurn(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	runners[agent.AgentTypeMockInterviewer].taskErr = errors.New("model unavailable")
	runners[agent.AgentTypeMockInterviewer].taskResponse = "partial"

	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "answer"}); err == nil {
		t.Fatalf("SubmitMockTurn() error = nil, want error")
	}
	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("turns length = %d, want 3", len(turns))
	}
	if turns[2].TurnType != mockTurnTypeError {
		t.Fatalf("last turn type = %q, want %q", turns[2].TurnType, mockTurnTypeError)
	}
}

func TestCompleteMockInterviewSuccess(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("next question", 80),
		sampleMockCompleteJSON(),
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

	completed, err := s.CompleteMockInterview(context.Background(), mock.MockID)
	if err != nil {
		t.Fatalf("CompleteMockInterview() error = %v", err)
	}
	if completed.Status != MockInterviewStatusCompleted {
		t.Fatalf("status = %q, want %q", completed.Status, MockInterviewStatusCompleted)
	}
	if completed.FinalSummary == "" {
		t.Fatalf("final_summary is empty")
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "after complete"}); err == nil {
		t.Fatalf("SubmitMockTurn() after complete error = nil, want error")
	}
}

func TestCompleteMockInterviewParseFailureKeepsInProgress(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{sampleMockStartJSON(), "not json"}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.CompleteMockInterview(context.Background(), mock.MockID); err == nil {
		t.Fatalf("CompleteMockInterview() error = nil, want error")
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.Status != MockInterviewStatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, MockInterviewStatusFailed)
	}
}

func TestMockInterviewDoesNotWriteMemoryItems(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	before, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() before error = %v", err)
	}
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSON("next question", 70),
		sampleMockCompleteJSON(),
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
	if _, err := s.CompleteMockInterview(context.Background(), mock.MockID); err != nil {
		t.Fatalf("CompleteMockInterview() error = %v", err)
	}
	after, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() after error = %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("memory item count changed from %d to %d", len(before), len(after))
	}
}

func TestSubmitMockTurnHintDoesNotUpdatePracticeState(t *testing.T) {
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
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "能给一个提示吗？"})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if turn.TurnType != mockTurnTypeHintRequest || turn.Role != mockTurnRoleAssistant {
		t.Fatalf("turn = %#v, want assistant hint turn", turn)
	}
	states, err := s.ListPracticeStates("user_001", "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("practice states length = %d, want 0", len(states))
	}
}

func TestSubmitMockTurnSwitchTopicAndComplete(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockSwitchTopicJSON(),
		sampleMockCompleteTurnJSON(),
	}

	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	switchTurn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "first"})
	if err != nil {
		t.Fatalf("switch SubmitMockTurn() error = %v", err)
	}
	if switchTurn.TurnType != mockTurnTypeTopicSwitch || switchTurn.AgentAction != mockNextActionSwitchTopic {
		t.Fatalf("switch turn = %#v", switchTurn)
	}
	completedTurn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "second"})
	if err != nil {
		t.Fatalf("complete SubmitMockTurn() error = %v", err)
	}
	if completedTurn.TurnType != mockTurnTypeClosingSummary {
		t.Fatalf("completed turn type = %q, want %q", completedTurn.TurnType, mockTurnTypeClosingSummary)
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.Status != MockInterviewStatusCompleted || got.FinalSummary == "" {
		t.Fatalf("mock = %#v, want completed with final summary", got)
	}
}

func TestCancelMockInterviewStopsFurtherTurns(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponse = sampleMockStartJSON()
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	cancelled, err := s.CancelMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("CancelMockInterview() error = %v", err)
	}
	if cancelled.Status != MockInterviewStatusCancelled {
		t.Fatalf("status = %q, want %q", cancelled.Status, MockInterviewStatusCancelled)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "after cancel"}); err == nil {
		t.Fatalf("SubmitMockTurn() after cancel error = nil, want error")
	}
}

func TestSubmitMockTurnPracticeStateFailureRollsBackTurns(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockTurnJSONWithTopics("next question", 72, []string{"Redis 缓存一致性"}),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{UserID: "user_001", PlanID: planID})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if err := s.db.Exec("DROP TABLE practice_states").Error; err != nil {
		t.Fatalf("drop practice_states: %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "answer"}); err == nil {
		t.Fatalf("SubmitMockTurn() error = nil, want practice state error")
	}
	turns, err := s.ListMockTurns(mock.MockID)
	if err != nil {
		t.Fatalf("ListMockTurns() error = %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("turns length = %d, want only opening turn after rollback", len(turns))
	}
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 0 || got.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock = %#v, want unchanged active state", got)
	}
}

func createMockReadyInterview(t *testing.T, s *Server, runners map[agent.AgentType]*fakeRunner) (vo.InterviewSessionVO, string) {
	t.Helper()

	session, _ := createCoachingReadyInterview(t, s, runners)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = strings.ReplaceAll(
		sampleCoachingPlanJSON("mock strategy", "mock focus"),
		"MEMORY_ID_PLACEHOLDER",
		"mock-memory",
	)
	plan, err := s.GenerateCoachingPlan(context.Background(), session.InterviewID, vo.GenerateCoachingPlanReq{
		UserID:        "user_001",
		TargetRound:   "second_round",
		RemainingDays: 2,
	})
	if err != nil {
		t.Fatalf("GenerateCoachingPlan() error = %v", err)
	}
	return session, plan.PlanID
}

func sampleMockStartJSON() string {
	return `{
  "overall_goal": "模拟后端二面，重点追问项目深度、Redis 和 Agent 工具调用异常处理。",
  "first_question": "请先介绍一下你最近做的 Agent 项目，重点说清楚它解决了什么具体问题。"
}`
}

func sampleMockTurnJSON(nextQuestion string, score int) string {
	return fmt.Sprintf(`{
  "input_type": "formal_answer",
  "agent_message": "%s",
  "score": %d,
  "feedback": "你的回答说明了功能流程，但缺少异常处理和状态流转。",
  "topic": "Agent 工具调用",
  "weakness_tags": ["Agent 工具调用", "异常处理", "项目深挖"],
  "next_action": "ask_followup",
  "should_update_practice_state": true,
  "practice_updates": [{"topic":"Agent 工具调用","score":%d,"feedback":"你的回答说明了功能流程，但缺少异常处理和状态流转。"}],
  "should_complete_mock": false,
  "follow_up_reason": "根据长期记忆，用户在工程边界表达上较弱，需要继续追问失败恢复。",
  "topic_tags": ["Agent 工具调用", "异常处理", "项目深挖"],
  "next_question": "%s"
}`, nextQuestion, score, score, nextQuestion)
}

func sampleMockChatOnlyJSON(visibleMessage string, userIntent string, score int) string {
	return fmt.Sprintf(`{
  "visible_message": "%s",
  "user_intent": "%s",
  "state_action": "chat_only",
  "confidence": 0.86,
  "needs_clarification": false,
  "input_type": "formal_answer",
  "agent_message": "兼容旧字段不应该覆盖 visible_message",
  "score": %d,
  "feedback": "这段反馈不应该保存。",
  "topic": "Agent 工具调用",
  "weakness_tags": ["Agent 工具调用"],
  "next_action": "ask_followup",
  "should_update_practice_state": true,
  "practice_updates": [{"topic":"Agent 工具调用","score":%d,"feedback":"不应写入练习状态。"}],
  "should_complete_mock": false,
  "follow_up_reason": "不应保存正式追问原因。",
  "topic_tags": ["Agent 工具调用"],
  "next_question": "不应该推进到这个问题"
}`, visibleMessage, userIntent, score, score)
}

func sampleMockFormalHintWithBadScoreJSON() string {
	return `{
  "visible_message": "可以，先按背景、方案、失败恢复三个层次组织回答。",
  "user_intent": "ask_hint",
  "state_action": "record_attempt",
  "confidence": 0.91,
  "needs_clarification": false,
  "input_type": "formal_answer",
  "agent_message": "这个旧字段不应作为正式追问推进。",
  "score": 93,
  "feedback": "这段评分不应该保存。",
  "topic": "Agent 工具调用",
  "weakness_tags": ["Agent 工具调用"],
  "next_action": "ask_followup",
  "should_update_practice_state": true,
  "practice_updates": [{"topic":"Agent 工具调用","score":93,"feedback":"不应写入练习状态。"}],
  "should_complete_mock": false,
  "follow_up_reason": "不应保存正式追问原因。",
  "topic_tags": ["Agent 工具调用"],
  "next_question": "不应该推进到这个问题"
}`
}

func sampleMockCompleteJSON() string {
	return `{
  "final_summary": "本次模拟中，用户项目背景表达清楚，但幂等、失败恢复和可观测性仍需补强。"
}`
}

func sampleMockHintJSON() string {
	return `{
  "input_type": "hint_request",
  "agent_message": "可以先从问题背景、你的方案、失败恢复三个层次回答。",
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

func sampleMockSwitchTopicJSON() string {
	return `{
  "input_type": "formal_answer",
  "agent_message": "我们换到 Redis 缓存一致性。你会如何处理缓存和数据库双写不一致？",
  "score": 68,
  "feedback": "项目描述有主线，但故障恢复仍不够具体。",
  "topic": "项目深挖",
  "weakness_tags": ["项目深挖"],
  "next_action": "switch_topic",
  "should_update_practice_state": true,
  "practice_updates": [{"topic":"项目深挖","score":68,"feedback":"项目描述有主线，但故障恢复仍不够具体。"}],
  "should_complete_mock": false,
  "follow_up_reason": "当前主题已覆盖，切换到弱项 Redis。"
}`
}

func sampleMockCompleteTurnJSON() string {
	return `{
  "input_type": "formal_answer",
  "agent_message": "本次模拟结束：你能覆盖核心方案，但需要补强一致性边界和监控指标。",
  "score": 82,
  "feedback": "Redis 一致性回答比项目深挖更完整。",
  "topic": "Redis 缓存一致性",
  "weakness_tags": ["Redis 缓存一致性"],
  "next_action": "complete",
  "should_update_practice_state": true,
  "practice_updates": [{"topic":"Redis 缓存一致性","score":82,"feedback":"Redis 一致性回答比项目深挖更完整。"}],
  "should_complete_mock": true,
  "follow_up_reason": "已覆盖本轮目标。"
}`
}
