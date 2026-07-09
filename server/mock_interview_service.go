package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-web-base/agent"
	"agent-web-base/shared/log"
	"agent-web-base/vo"
)

const (
	MockInterviewStatusCreated         = "created"
	MockInterviewStatusInProgress      = "in_progress"
	MockInterviewStatusWaitingAnswer   = "waiting_answer"
	MockInterviewStatusEvaluating      = "evaluating_answer"
	MockInterviewStatusAskingFollowup  = "asking_followup"
	MockInterviewStatusSwitchingTopic  = "switching_topic"
	MockInterviewStatusCompleted       = "completed"
	MockInterviewStatusFailed          = "failed"
	MockInterviewStatusRetriableFailed = "retriable_failed"
	MockInterviewStatusCancelled       = "cancelled"

	mockTurnRoleUser      = "user"
	mockTurnRoleAssistant = "assistant"
	mockTurnRoleSystem    = "system"

	mockTurnTypeOpeningQuestion     = "opening_question"
	mockTurnTypeUserAnswer          = "user_answer"
	mockTurnTypeEvaluationFeedback  = "evaluation_feedback"
	mockTurnTypeFollowupQuestion    = "followup_question"
	mockTurnTypeTopicSwitch         = "topic_switch"
	mockTurnTypeHintRequest         = "hint_request"
	mockTurnTypeExplanationRequest  = "explanation_request"
	mockTurnTypeClosingSummary      = "closing_summary"
	mockTurnTypeError               = "error"
	mockTurnTypeCancellationSummary = "cancellation_summary"

	mockInputTypeFormalAnswer       = "formal_answer"
	mockInputTypeHintRequest        = "hint_request"
	mockInputTypeExplanationRequest = "explanation_request"
	mockInputTypeCancel             = "cancel"

	mockNextActionAskFollowup  = "ask_followup"
	mockNextActionSwitchTopic  = "switch_topic"
	mockNextActionComplete     = "complete"
	mockNextActionWaitForInput = "wait_for_answer"

	mockSubmitModeChat         = "chat"
	mockSubmitModeFormalAnswer = "formal_answer"

	mockTurnTriggerSilenceTimeout = "silence_timeout"

	mockUserIntentAnswer     = "answer"
	mockUserIntentAskHint    = "ask_hint"
	mockUserIntentAskExplain = "ask_explain"
	mockUserIntentSmalltalk  = "smalltalk"
	mockUserIntentUnclear    = "unclear"
	mockUserIntentCancel     = "cancel"

	mockStateActionRecordAttempt = "record_attempt"
	mockStateActionChatOnly      = "chat_only"
	mockStateActionStayCurrent   = "stay_current"
	mockStateActionCancel        = "cancel"
)

type mockStartOutput struct {
	OverallGoal       string `json:"overall_goal"`
	FirstQuestion     string `json:"first_question"`
	TimeLimitSeconds  int    `json:"time_limit_seconds"`
	TimePressureStyle string `json:"time_pressure_style"`
	WarnAtSeconds     int    `json:"warn_at_seconds"`
}

type mockTurnOutput struct {
	InputType                 string                `json:"input_type"`
	AgentMessage              string                `json:"agent_message"`
	VisibleMessage            string                `json:"visible_message"`
	UserIntent                string                `json:"user_intent"`
	StateAction               string                `json:"state_action"`
	Confidence                float64               `json:"confidence"`
	NeedsClarification        bool                  `json:"needs_clarification"`
	SubmitMode                string                `json:"submit_mode"`
	Score                     int                   `json:"score"`
	Feedback                  string                `json:"feedback"`
	Topic                     string                `json:"topic"`
	WeaknessTags              []string              `json:"weakness_tags"`
	NextAction                string                `json:"next_action"`
	ShouldUpdatePracticeState bool                  `json:"should_update_practice_state"`
	PracticeUpdates           []mockPracticeUpdate  `json:"practice_updates"`
	ShouldCompleteMock        bool                  `json:"should_complete_mock"`
	FollowUpReason            string                `json:"follow_up_reason"`
	TopicTags                 []string              `json:"topic_tags"`
	NextQuestion              string                `json:"next_question"`
	PersistentStateUpdate     PersistentStateUpdate `json:"persistent_state_update"`
	DefenseRules              []DefenseRuleDecision `json:"defense_rules,omitempty"`
}

type mockCompleteOutput struct {
	FinalSummary string `json:"final_summary"`
}

type mockPracticeUpdate struct {
	Topic    string `json:"topic"`
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

type mockInput struct {
	sourceType    string
	session       InterviewSession
	practiceGoal  *PracticeGoal
	report        InterviewReviewReport
	questions     []InterviewQuestion
	selection     MemorySelectionResult
	questionBank  []QuestionBankQuestion
	coachingPlan  *CoachingPlan
	coachingTasks []CoachingTask
}

func (s *Server) StartMockInterview(ctx context.Context, interviewID string, req vo.StartMockInterviewReq) (vo.MockInterviewVO, error) {
	if s.agents == nil {
		return vo.MockInterviewVO{}, fmt.Errorf("agent provider is nil")
	}
	if req.TargetRound == "" {
		req.TargetRound = "second_round"
	}

	if active, ok, err := s.findActiveMockInterview(interviewID, req.UserID, req.PlanID, req.TargetRound); err != nil {
		return vo.MockInterviewVO{}, err
	} else if ok {
		return toMockInterviewVO(active), nil
	}

	input, err := s.loadMockInput(interviewID, req.UserID, req.PlanID, req.TargetRound, normalizeDefault(req.FocusTopic, MemorySelectorTaskMockStart))
	if err != nil {
		return vo.MockInterviewVO{}, err
	}
	if input.session.Status != InterviewStatusReviewed {
		return vo.MockInterviewVO{}, fmt.Errorf("interview status must be %q before starting mock interview", InterviewStatusReviewed)
	}
	if input.session.UserID != req.UserID {
		return vo.MockInterviewVO{}, fmt.Errorf("interview user_id mismatch")
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeMockInterviewer))
	if err != nil {
		return vo.MockInterviewVO{}, err
	}

	prompt := buildMockStartPrompt(input, req)
	inputSnapshot := marshalTraceJSON(map[string]any{
		"interview_id":     input.session.InterviewID,
		"user_id":          req.UserID,
		"plan_id":          req.PlanID,
		"target_round":     req.TargetRound,
		"question_count":   len(input.questions),
		"has_plan":         input.coachingPlan != nil,
		"task_count":       len(input.coachingTasks),
		"prompt_length":    len(prompt),
		"review_status":    input.report.Status,
		"interview_status": input.session.Status,
	})
	selectedContextSnapshot := buildSelectedContextTraceSnapshot(input.selection)

	result, err := runner.RunTask(ctx, prompt)
	now := time.Now().Unix()
	if err != nil {
		log.Warnf("mock interviewer start failed for interview %s: %v", interviewID, err)
		raw := fallbackRaw(result.Response, err)
		mock := MockInterview{
			MockID:         uuid.New().String(),
			UserID:         req.UserID,
			InterviewID:    interviewID,
			PlanID:         req.PlanID,
			TargetRound:    req.TargetRound,
			Status:         MockInterviewStatusFailed,
			RawAgentOutput: raw,
			ErrorMessage:   err.Error(),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if createErr := s.db.Create(&mock).Error; createErr != nil {
			return vo.MockInterviewVO{}, createErr
		}
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  input.session.UserID,
			InterviewID:             input.session.InterviewID,
			AgentType:               string(agent.AgentTypeMockInterviewer),
			SourceType:              AgentTraceSourceMockInterview,
			SourceID:                mock.MockID,
			StepName:                AgentTraceStepMockStart,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          raw,
			ServiceActions:          marshalTraceJSON([]string{"created failed mock_interview"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		return toMockInterviewVO(mock), fmt.Errorf("mock interviewer start failed: %w", err)
	}

	parsed, err := parseMockStartOutput(result.Response)
	if err != nil {
		log.Warnf("parse mock start output failed for interview %s: %v, raw=%s", interviewID, err, result.Response)
		mock := MockInterview{
			MockID:         uuid.New().String(),
			UserID:         req.UserID,
			InterviewID:    interviewID,
			PlanID:         req.PlanID,
			TargetRound:    req.TargetRound,
			Status:         MockInterviewStatusFailed,
			RawAgentOutput: result.Response,
			ErrorMessage:   err.Error(),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if createErr := s.db.Create(&mock).Error; createErr != nil {
			return vo.MockInterviewVO{}, createErr
		}
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  input.session.UserID,
			InterviewID:             input.session.InterviewID,
			AgentType:               string(agent.AgentTypeMockInterviewer),
			SourceType:              AgentTraceSourceMockInterview,
			SourceID:                mock.MockID,
			StepName:                AgentTraceStepMockStart,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ServiceActions:          marshalTraceJSON([]string{"created failed mock_interview"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		return toMockInterviewVO(mock), err
	}

	mock := MockInterview{
		MockID:         uuid.New().String(),
		UserID:         req.UserID,
		InterviewID:    interviewID,
		PlanID:         req.PlanID,
		TargetRound:    req.TargetRound,
		Status:         MockInterviewStatusWaitingAnswer,
		CurrentTurn:    0,
		CurrentTopic:   "opening",
		OverallGoal:    parsed.OverallGoal,
		FirstQuestion:  parsed.FirstQuestion,
		RawAgentOutput: result.Response,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	openingTurn := MockTurn{
		TurnID:              uuid.New().String(),
		MockID:              mock.MockID,
		UserID:              mock.UserID,
		InterviewID:         mock.InterviewID,
		TurnIndex:           1,
		Role:                mockTurnRoleAssistant,
		TurnType:            mockTurnTypeOpeningQuestion,
		Phase:               MockInterviewStatusWaitingAnswer,
		AgentAction:         mockNextActionWaitForInput,
		Content:             parsed.FirstQuestion,
		InterviewerQuestion: parsed.FirstQuestion,
		NextQuestion:        parsed.FirstQuestion,
		TimeLimitSeconds:    parsed.TimeLimitSeconds,
		TimePressureStyle:   parsed.TimePressureStyle,
		WarnAtSeconds:       parsed.WarnAtSeconds,
		RawAgentOutput:      result.Response,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&mock).Error; err != nil {
			return err
		}
		return tx.Create(&openingTurn).Error
	}); err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  input.session.UserID,
			InterviewID:             input.session.InterviewID,
			AgentType:               string(agent.AgentTypeMockInterviewer),
			SourceType:              AgentTraceSourceMockInterview,
			SourceID:                mock.MockID,
			StepName:                AgentTraceStepMockStart,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ParsedDecision:          marshalTraceJSON(parsed),
			ServiceActions:          marshalTraceJSON([]string{"failed to persist mock_start"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		return vo.MockInterviewVO{}, err
	}
	s.recordAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:                  input.session.UserID,
		InterviewID:             input.session.InterviewID,
		AgentType:               string(agent.AgentTypeMockInterviewer),
		SourceType:              AgentTraceSourceMockInterview,
		SourceID:                mock.MockID,
		StepName:                AgentTraceStepMockStart,
		SelectedContextSnapshot: selectedContextSnapshot,
		InputSnapshot:           inputSnapshot,
		RawAgentOutput:          result.Response,
		ParsedDecision:          marshalTraceJSON(parsed),
		ServiceActions:          marshalTraceJSON([]string{"created mock_interview", "created opening mock_turn"}),
		Status:                  AgentDecisionTraceStatusSucceeded,
	})
	return toMockInterviewVO(mock), nil
}

func (s *Server) StartPracticeGoalMockInterview(ctx context.Context, goalID string, req vo.StartPracticeGoalMockReq) (vo.MockInterviewVO, error) {
	if s.agents == nil {
		return vo.MockInterviewVO{}, fmt.Errorf("agent provider is nil")
	}
	req.UserID = strings.TrimSpace(req.UserID)

	input, err := s.loadPracticeGoalMockInput(goalID, req.UserID, req.PlanID, req.TargetRound, req.FocusTopic)
	if err != nil {
		return vo.MockInterviewVO{}, err
	}
	goal := *input.practiceGoal
	if strings.TrimSpace(req.TargetRound) == "" {
		req.TargetRound = goal.TargetRound
	}
	if strings.TrimSpace(req.TargetRound) == "" {
		req.TargetRound = "second_round"
	}
	if active, ok, err := s.findActivePracticeGoalMockInterview(goal.GoalID, req.UserID, req.PlanID, req.TargetRound); err != nil {
		return vo.MockInterviewVO{}, err
	} else if ok {
		return toMockInterviewVO(active), nil
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeMockInterviewer))
	if err != nil {
		return vo.MockInterviewVO{}, err
	}
	startReq := vo.StartMockInterviewReq{UserID: req.UserID, PlanID: req.PlanID, TargetRound: req.TargetRound, FocusTopic: req.FocusTopic}
	prompt := buildMockStartPrompt(input, startReq)
	result, err := runner.RunTask(ctx, prompt)
	now := time.Now().Unix()
	if err != nil {
		return vo.MockInterviewVO{}, fmt.Errorf("mock interviewer start failed: %w", err)
	}
	parsed, err := parseMockStartOutput(result.Response)
	if err != nil {
		return vo.MockInterviewVO{}, err
	}

	mock := MockInterview{
		MockID:         uuid.New().String(),
		UserID:         req.UserID,
		InterviewID:    "",
		PracticeGoalID: goal.GoalID,
		PlanID:         req.PlanID,
		TargetRound:    req.TargetRound,
		Status:         MockInterviewStatusWaitingAnswer,
		CurrentTurn:    0,
		CurrentTopic:   "opening",
		OverallGoal:    parsed.OverallGoal,
		FirstQuestion:  parsed.FirstQuestion,
		RawAgentOutput: result.Response,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	openingTurn := MockTurn{
		TurnID:              uuid.New().String(),
		MockID:              mock.MockID,
		UserID:              mock.UserID,
		InterviewID:         "",
		PracticeGoalID:      goal.GoalID,
		TurnIndex:           1,
		Role:                mockTurnRoleAssistant,
		TurnType:            mockTurnTypeOpeningQuestion,
		Phase:               MockInterviewStatusWaitingAnswer,
		AgentAction:         mockNextActionWaitForInput,
		Content:             parsed.FirstQuestion,
		InterviewerQuestion: parsed.FirstQuestion,
		NextQuestion:        parsed.FirstQuestion,
		TimeLimitSeconds:    parsed.TimeLimitSeconds,
		TimePressureStyle:   parsed.TimePressureStyle,
		WarnAtSeconds:       parsed.WarnAtSeconds,
		RawAgentOutput:      result.Response,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&mock).Error; err != nil {
			return err
		}
		return tx.Create(&openingTurn).Error
	}); err != nil {
		return vo.MockInterviewVO{}, err
	}
	s.recordAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:                  goal.UserID,
		AgentType:               string(agent.AgentTypeMockInterviewer),
		SourceType:              AgentTraceSourceMockInterview,
		SourceID:                mock.MockID,
		StepName:                AgentTraceStepMockStart,
		SelectedContextSnapshot: buildSelectedContextTraceSnapshot(input.selection),
		InputSnapshot: marshalTraceJSON(map[string]any{
			"practice_goal_id": goal.GoalID,
			"user_id":          req.UserID,
			"plan_id":          req.PlanID,
			"target_round":     req.TargetRound,
			"source_type":      CoachingPlanSourcePracticeGoal,
			"prompt_length":    len(prompt),
		}),
		RawAgentOutput: result.Response,
		ParsedDecision: marshalTraceJSON(parsed),
		ServiceActions: marshalTraceJSON([]string{"created practice_goal mock_interview", "created opening mock_turn"}),
		Status:         AgentDecisionTraceStatusSucceeded,
	})
	return toMockInterviewVO(mock), nil
}

func (s *Server) SubmitMockTurn(ctx context.Context, mockID string, req vo.SubmitMockTurnReq) (vo.MockTurnVO, error) {
	return s.submitMockTurnInternal(ctx, mockID, req, nil)
}

func (s *Server) submitMockTurnInternal(ctx context.Context, mockID string, req vo.SubmitMockTurnReq, existingUserTurn *MockTurn) (vo.MockTurnVO, error) {
	if s.agents == nil {
		return vo.MockTurnVO{}, fmt.Errorf("agent provider is nil")
	}

	var mock MockInterview
	if err := s.db.First(&mock, "mock_id = ?", mockID).Error; err != nil {
		return vo.MockTurnVO{}, err
	}
	if existingUserTurn == nil {
		if !isMockSubmittableStatus(mock.Status) {
			return vo.MockTurnVO{}, fmt.Errorf("mock interview status %q does not accept turns", mock.Status)
		}
		orphan, err := s.findLatestOrphanMockUserTurn(mockID)
		if err != nil {
			return vo.MockTurnVO{}, err
		}
		if orphan != nil {
			existingUserTurn = orphan
		}
	}

	turns, err := s.loadMockTurns(mockID)
	if err != nil {
		return vo.MockTurnVO{}, err
	}
	answer := strings.TrimSpace(req.Answer)
	isSilenceTimeout := strings.TrimSpace(req.Trigger) == mockTurnTriggerSilenceTimeout
	if existingUserTurn != nil {
		answer = existingUserTurn.UserAnswer
		if strings.TrimSpace(answer) == "" {
			answer = existingUserTurn.Content
		}
	} else {
		if answer == "" && !isSilenceTimeout {
			return vo.MockTurnVO{}, fmt.Errorf("answer is required")
		}
		if isSilenceTimeout {
			req.SubmitMode = mockSubmitModeChat
			answer = "（候选人超过提醒时间未输入，前端触发静默 check-in）"
		}
	}
	var input mockInput
	if mock.PracticeGoalID != "" {
		input, err = s.loadPracticeGoalMockInput(mock.PracticeGoalID, mock.UserID, mock.PlanID, mock.TargetRound, MemorySelectorTaskMockTurn)
	} else {
		input, err = s.loadMockInput(mock.InterviewID, mock.UserID, mock.PlanID, mock.TargetRound, MemorySelectorTaskMockTurn)
	}
	if err != nil {
		return vo.MockTurnVO{}, err
	}

	currentQuestion := currentMockQuestion(mock, turns)
	submitMode := normalizeMockSubmitMode(req.SubmitMode)

	_, runner, err := s.agents.Get(string(agent.AgentTypeMockInterviewer))
	if err != nil {
		return vo.MockTurnVO{}, err
	}

	timerResult := checkMockTimer(mock, turns, time.Now())
	staticContext := buildMockTurnInstructionContext() + "\n\n" + buildMockTurnStaticContext(input, mock, currentQuestion, timerResult)
	historyTurns := mockHistoryTurnsForPrompt(turns, existingUserTurn)
	historyMessages := projectMockTurnsToMessages(historyTurns)
	compression := compressBusinessHistoryForPrompt(historyMessages, BusinessHistoryCompressionConfig{})
	userMessage := buildMockTurnUserMessage(answer, submitMode, currentQuestion, strings.TrimSpace(req.Trigger))
	inputSnapshot := marshalTraceJSON(map[string]any{
		"mock_id":                   mock.MockID,
		"interview_id":              mock.InterviewID,
		"user_id":                   mock.UserID,
		"plan_id":                   mock.PlanID,
		"target_round":              mock.TargetRound,
		"mock_status":               mock.Status,
		"current_turn":              mock.CurrentTurn,
		"current_topic":             mock.CurrentTopic,
		"submit_mode":               submitMode,
		"history_turn_count":        len(turns),
		"history_message_count":     len(historyMessages),
		"compressed_message_count":  compression.CompressedMessageCount,
		"history_summary_generated": compression.SummaryGenerated,
		"history_truncated":         compression.Truncated,
		"trigger":                   strings.TrimSpace(req.Trigger),
		"current_question_length":   len(currentQuestion),
		"answer_length":             len(answer),
		"question_count":            len(input.questions),
		"static_context_length":     len(staticContext),
		"user_message_length":       len(userMessage),
	})
	selectedContextSnapshot := buildBusinessContextTraceSnapshot(input.selection, compression)

	viewCh := make(chan agent.MessageVO, 64)
	confirmCh := make(chan agent.ConfirmationAction, 1)
	drained := make(chan struct{})
	defer func() {
		close(viewCh)
		<-drained
		close(confirmCh)
	}()
	go func() {
		defer close(drained)
		for event := range viewCh {
			if event.Type == agent.MessageTypeToolConfirm {
				select {
				case confirmCh <- agent.ConfirmReject:
				default:
				}
			}
		}
	}()

	result, err := runner.RunStreamingWithContextHistory(ctx, agent.RunOptions{
		SystemContext:     staticContext,
		ApplyPolicies:     true,
		UpdateAgentMemory: false,
	}, compression.Messages, userMessage, viewCh, confirmCh)
	if err != nil {
		log.Warnf("mock interviewer turn failed for mock %s: %v", mockID, err)
		_ = s.failMockTurn(mock, len(turns), currentQuestion, answer, fallbackRaw(result.Response, err), err.Error(), existingUserTurn)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  mock.UserID,
			InterviewID:             mock.InterviewID,
			AgentType:               string(agent.AgentTypeMockInterviewer),
			SourceType:              AgentTraceSourceMockInterview,
			SourceID:                mock.MockID,
			StepName:                AgentTraceStepMockTurn,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          fallbackRaw(result.Response, err),
			ServiceActions:          marshalTraceJSON([]string{"recorded failed mock_turns", "updated mock status failed"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		return vo.MockTurnVO{}, fmt.Errorf("mock interviewer turn failed: %w", err)
	}

	parsed, err := parseMockTurnOutput(result.Response, submitMode)
	if err != nil {
		log.Warnf("parse mock turn output failed for mock %s: %v, raw=%s", mockID, err, result.Response)
		_ = s.failMockTurn(mock, len(turns), currentQuestion, answer, result.Response, err.Error(), existingUserTurn)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  mock.UserID,
			InterviewID:             mock.InterviewID,
			AgentType:               string(agent.AgentTypeMockInterviewer),
			SourceType:              AgentTraceSourceMockInterview,
			SourceID:                mock.MockID,
			StepName:                AgentTraceStepMockTurn,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ServiceActions:          marshalTraceJSON([]string{"recorded failed mock_turns", "updated mock status failed"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		return vo.MockTurnVO{}, err
	}

	now := time.Now().Unix()
	nextIndex := len(turns) + 1
	userTurnType := mockTurnTypeUserAnswer
	if parsed.InputType == mockInputTypeHintRequest {
		userTurnType = mockTurnTypeHintRequest
	}
	if parsed.InputType == mockInputTypeExplanationRequest {
		userTurnType = mockTurnTypeExplanationRequest
	}
	var userTurn MockTurn
	if existingUserTurn != nil {
		userTurn = *existingUserTurn
	} else {
		userTurn = MockTurn{
			TurnID:              uuid.New().String(),
			MockID:              mock.MockID,
			UserID:              mock.UserID,
			InterviewID:         mock.InterviewID,
			PracticeGoalID:      mock.PracticeGoalID,
			TurnIndex:           nextIndex,
			Role:                mockTurnRoleUser,
			TurnType:            userTurnType,
			Phase:               MockInterviewStatusEvaluating,
			Content:             answer,
			InterviewerQuestion: currentQuestion,
			UserAnswer:          answer,
			RawAgentOutput:      result.Response,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
	}
	nextIndex++

	created := []MockTurn{}
	if existingUserTurn == nil {
		created = append(created, userTurn)
	}
	updates := map[string]any{
		"raw_agent_output": result.Response,
		"error_message":    "",
		"updated_at":       now,
	}
	var responseTurn MockTurn

	if parsed.InputType == mockInputTypeCancel {
		cancelTurn := MockTurn{
			TurnID:         uuid.New().String(),
			MockID:         mock.MockID,
			UserID:         mock.UserID,
			InterviewID:    mock.InterviewID,
			PracticeGoalID: mock.PracticeGoalID,
			TurnIndex:      nextIndex,
			Role:           mockTurnRoleAssistant,
			TurnType:       mockTurnTypeCancellationSummary,
			Phase:          MockInterviewStatusCancelled,
			AgentAction:    mockInputTypeCancel,
			Content:        mockVisibleMessage(parsed),
			RawAgentOutput: result.Response,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		created = append(created, cancelTurn)
		updates["status"] = MockInterviewStatusCancelled
		updates["last_feedback"] = mockVisibleMessage(parsed)
		responseTurn = cancelTurn
	} else if parsed.InputType == mockInputTypeHintRequest || parsed.InputType == mockInputTypeExplanationRequest {
		assistantType := mockTurnTypeHintRequest
		if parsed.InputType == mockInputTypeExplanationRequest {
			assistantType = mockTurnTypeExplanationRequest
		}
		assistantTurn := MockTurn{
			TurnID:              uuid.New().String(),
			MockID:              mock.MockID,
			UserID:              mock.UserID,
			InterviewID:         mock.InterviewID,
			PracticeGoalID:      mock.PracticeGoalID,
			TurnIndex:           nextIndex,
			Role:                mockTurnRoleAssistant,
			TurnType:            assistantType,
			Phase:               MockInterviewStatusWaitingAnswer,
			AgentAction:         mockNextActionWaitForInput,
			Content:             mockVisibleMessage(parsed),
			InterviewerQuestion: currentQuestion,
			NextQuestion:        currentQuestion,
			RawAgentOutput:      result.Response,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		created = append(created, assistantTurn)
		updates["status"] = MockInterviewStatusWaitingAnswer
		responseTurn = assistantTurn
	} else if isFormalMockAttempt(parsed) {
		topics := mockPracticeTopics(parsed)
		evaluationTurn := MockTurn{
			TurnID:              uuid.New().String(),
			MockID:              mock.MockID,
			UserID:              mock.UserID,
			InterviewID:         mock.InterviewID,
			PracticeGoalID:      mock.PracticeGoalID,
			TurnIndex:           nextIndex,
			Role:                mockTurnRoleAssistant,
			TurnType:            mockTurnTypeEvaluationFeedback,
			Phase:               MockInterviewStatusEvaluating,
			Content:             parsed.Feedback,
			InterviewerQuestion: currentQuestion,
			UserAnswer:          answer,
			Feedback:            parsed.Feedback,
			Score:               clampScore(parsed.Score),
			FollowUpReason:      parsed.FollowUpReason,
			TopicTags:           marshalStringSlice(topics),
			RawAgentOutput:      result.Response,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		created = append(created, evaluationTurn)
		nextIndex++

		actionTurn := buildMockActionTurn(mock, nextIndex, parsed, currentQuestion, result.Response, now)
		created = append(created, actionTurn)
		responseTurn = actionTurn

		updates["current_turn"] = mock.CurrentTurn + 1
		updates["current_topic"] = parsed.Topic
		updates["last_feedback"] = parsed.Feedback
		if parsed.ShouldCompleteMock || parsed.NextAction == mockNextActionComplete {
			updates["status"] = MockInterviewStatusCompleted
			updates["final_summary"] = mockVisibleMessage(parsed)
		} else {
			updates["status"] = MockInterviewStatusWaitingAnswer
		}

		successTraceInput := AgentDecisionTraceInput{
			UserID:                  mock.UserID,
			InterviewID:             mock.InterviewID,
			AgentType:               string(agent.AgentTypeMockInterviewer),
			SourceType:              AgentTraceSourceMockInterview,
			SourceID:                mock.MockID,
			StepName:                AgentTraceStepMockTurn,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ParsedDecision:          marshalTraceJSON(parsed),
			ServiceActions:          marshalTraceJSON(mockTurnTraceActions(parsed, len(created))),
			Status:                  AgentDecisionTraceStatusSucceeded,
		}
		err = s.db.Transaction(func(tx *gorm.DB) error {
			for _, turn := range created {
				if err := tx.Create(&turn).Error; err != nil {
					return err
				}
			}
			if parsed.ShouldUpdatePracticeState {
				if err := s.updatePracticeStatesFromMockTurnTx(tx, evaluationTurn); err != nil {
					return err
				}
			}
			if err := mergeMockPersistentStateUpdateTx(tx, mockID, updates, parsed.PersistentStateUpdate, now); err != nil {
				return err
			}
			if err := tx.Model(&MockInterview{}).
				Where("mock_id = ?", mockID).
				Updates(updates).Error; err != nil {
				return err
			}
			return s.recordAgentDecisionTraceTx(tx, successTraceInput)
		})
		if err != nil {
			s.recordAgentDecisionTrace(AgentDecisionTraceInput{
				UserID:                  mock.UserID,
				InterviewID:             mock.InterviewID,
				AgentType:               string(agent.AgentTypeMockInterviewer),
				SourceType:              AgentTraceSourceMockInterview,
				SourceID:                mock.MockID,
				StepName:                AgentTraceStepMockTurn,
				SelectedContextSnapshot: selectedContextSnapshot,
				InputSnapshot:           inputSnapshot,
				RawAgentOutput:          result.Response,
				ParsedDecision:          marshalTraceJSON(parsed),
				ServiceActions:          marshalTraceJSON([]string{"failed to persist mock_turn"}),
				Status:                  AgentDecisionTraceStatusFailed,
				ErrorMessage:            traceErrorMessage(err),
			})
			return vo.MockTurnVO{}, err
		}
		return toMockTurnVO(responseTurn), nil
	} else {
		assistantTurn := buildMockOffRecordTurn(mock, nextIndex, parsed, currentQuestion, result.Response, now)
		created = append(created, assistantTurn)
		updates["status"] = MockInterviewStatusWaitingAnswer
		responseTurn = assistantTurn
	}

	successTraceInput := AgentDecisionTraceInput{
		UserID:                  mock.UserID,
		InterviewID:             mock.InterviewID,
		AgentType:               string(agent.AgentTypeMockInterviewer),
		SourceType:              AgentTraceSourceMockInterview,
		SourceID:                mock.MockID,
		StepName:                AgentTraceStepMockTurn,
		SelectedContextSnapshot: selectedContextSnapshot,
		InputSnapshot:           inputSnapshot,
		RawAgentOutput:          result.Response,
		ParsedDecision:          marshalTraceJSON(parsed),
		ServiceActions:          marshalTraceJSON(mockTurnTraceActions(parsed, len(created))),
		Status:                  AgentDecisionTraceStatusSucceeded,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, turn := range created {
			if err := tx.Create(&turn).Error; err != nil {
				return err
			}
		}
		if err := mergeMockPersistentStateUpdateTx(tx, mockID, updates, parsed.PersistentStateUpdate, now); err != nil {
			return err
		}
		if err := tx.Model(&MockInterview{}).
			Where("mock_id = ?", mockID).
			Updates(updates).Error; err != nil {
			return err
		}
		return s.recordAgentDecisionTraceTx(tx, successTraceInput)
	})
	if err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  mock.UserID,
			InterviewID:             mock.InterviewID,
			AgentType:               string(agent.AgentTypeMockInterviewer),
			SourceType:              AgentTraceSourceMockInterview,
			SourceID:                mock.MockID,
			StepName:                AgentTraceStepMockTurn,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ParsedDecision:          marshalTraceJSON(parsed),
			ServiceActions:          marshalTraceJSON([]string{"failed to persist mock_turn"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		return vo.MockTurnVO{}, err
	}
	return toMockTurnVO(responseTurn), nil
}

func mergeMockPersistentStateUpdateTx(tx *gorm.DB, mockID string, updates map[string]any, update PersistentStateUpdate, now int64) error {
	normalizedPersistentStateUpdate := normalizePersistentStateUpdate(update)
	if len(normalizedPersistentStateUpdate.Fields) == 0 {
		return nil
	}

	var currentMock MockInterview
	if err := tx.Select("agent_persistent_state").
		Where("mock_id = ?", mockID).
		First(&currentMock).Error; err != nil {
		return err
	}
	nextPersistentState, err := applyPersistentStateUpdate(persistentStateValue(currentMock.AgentPersistentState), normalizedPersistentStateUpdate, now)
	if err != nil {
		return err
	}
	updates["agent_persistent_state"] = persistentStatePtr(nextPersistentState)
	return nil
}

func buildMockActionTurn(mock MockInterview, turnIndex int, parsed mockTurnOutput, currentQuestion string, raw string, now int64) MockTurn {
	message := mockVisibleMessage(parsed)
	action := parsed.NextAction
	if action == "" {
		action = mockNextActionAskFollowup
	}
	turn := MockTurn{
		TurnID:              uuid.New().String(),
		MockID:              mock.MockID,
		UserID:              mock.UserID,
		InterviewID:         mock.InterviewID,
		PracticeGoalID:      mock.PracticeGoalID,
		TurnIndex:           turnIndex,
		Role:                mockTurnRoleAssistant,
		Phase:               MockInterviewStatusWaitingAnswer,
		AgentAction:         action,
		Content:             message,
		InterviewerQuestion: currentQuestion,
		Feedback:            parsed.Feedback,
		Score:               clampScore(parsed.Score),
		FollowUpReason:      parsed.FollowUpReason,
		TopicTags:           marshalStringSlice(mockPracticeTopics(parsed)),
		RawAgentOutput:      raw,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	switch {
	case parsed.ShouldCompleteMock || action == mockNextActionComplete:
		turn.TurnType = mockTurnTypeClosingSummary
		turn.Phase = MockInterviewStatusCompleted
	case action == mockNextActionSwitchTopic:
		turn.TurnType = mockTurnTypeTopicSwitch
		turn.NextQuestion = message
	default:
		turn.TurnType = mockTurnTypeFollowupQuestion
		turn.NextQuestion = message
	}
	return turn
}

func buildMockOffRecordTurn(mock MockInterview, turnIndex int, parsed mockTurnOutput, currentQuestion string, raw string, now int64) MockTurn {
	message := mockVisibleMessage(parsed)
	turnType := mockTurnTypeFollowupQuestion
	if parsed.UserIntent == mockUserIntentAskHint || parsed.InputType == mockInputTypeHintRequest {
		turnType = mockTurnTypeHintRequest
	} else if parsed.UserIntent == mockUserIntentAskExplain || parsed.InputType == mockInputTypeExplanationRequest {
		turnType = mockTurnTypeExplanationRequest
	}
	return MockTurn{
		TurnID:              uuid.New().String(),
		MockID:              mock.MockID,
		UserID:              mock.UserID,
		InterviewID:         mock.InterviewID,
		PracticeGoalID:      mock.PracticeGoalID,
		TurnIndex:           turnIndex,
		Role:                mockTurnRoleAssistant,
		TurnType:            turnType,
		Phase:               MockInterviewStatusWaitingAnswer,
		AgentAction:         parsed.StateAction,
		Content:             message,
		InterviewerQuestion: currentQuestion,
		NextQuestion:        currentQuestion,
		RawAgentOutput:      raw,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func mockTurnTraceActions(parsed mockTurnOutput, createdTurnCount int) []string {
	actions := []string{
		fmt.Sprintf("created mock_turns: %d", createdTurnCount),
		"updated mock status",
	}
	if isFormalMockAttempt(parsed) {
		if parsed.ShouldUpdatePracticeState {
			actions = append(actions, "updated practice_states")
		} else {
			actions = append(actions, "skipped practice update")
		}
	}
	if !isFormalMockAttempt(parsed) && parsed.InputType != mockInputTypeCancel {
		actions = append(actions, "skipped practice update")
	}
	if parsed.ShouldCompleteMock || parsed.NextAction == mockNextActionComplete {
		actions = append(actions, "updated mock status completed")
	}
	if len(normalizePersistentStateUpdate(parsed.PersistentStateUpdate).Fields) > 0 {
		actions = append(actions, "merged mock agent_persistent_state")
	}
	return actions
}

func mockHistoryTurnsForPrompt(turns []MockTurn, currentUserTurn *MockTurn) []MockTurn {
	history := make([]MockTurn, 0, len(turns))
	for _, turn := range turns {
		if currentUserTurn != nil && turn.TurnID == currentUserTurn.TurnID {
			continue
		}
		if turn.Role == mockTurnRoleSystem || turn.TurnType == mockTurnTypeError {
			continue
		}
		history = append(history, turn)
	}
	return history
}

func (s *Server) latestFailedMockUserTurn(mockID string) (MockTurn, error) {
	var turn MockTurn
	err := s.db.Where("mock_id = ? AND role = ?", mockID, mockTurnRoleUser).
		Order("turn_index desc, created_at desc").
		First(&turn).Error
	return turn, err
}

func (s *Server) findLatestOrphanMockUserTurn(mockID string) (*MockTurn, error) {
	var turn MockTurn
	err := s.db.Where("mock_id = ? AND role = ?", mockID, mockTurnRoleUser).
		Order("turn_index desc, created_at desc").
		First(&turn).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var laterCount int64
	if err := s.db.Model(&MockTurn{}).
		Where("mock_id = ? AND turn_index > ? AND role IN ?", mockID, turn.TurnIndex, []string{mockTurnRoleAssistant, mockTurnRoleSystem}).
		Count(&laterCount).Error; err != nil {
		return nil, err
	}
	if laterCount > 0 {
		return nil, nil
	}
	return &turn, nil
}

func (s *Server) failMockTurn(mock MockInterview, existingTurnCount int, currentQuestion string, answer string, raw string, message string, existingUserTurn ...*MockTurn) error {
	now := time.Now().Unix()
	var userTurn *MockTurn
	if len(existingUserTurn) > 0 && existingUserTurn[0] != nil {
		userTurn = existingUserTurn[0]
	} else {
		userTurn = &MockTurn{
			TurnID:              uuid.New().String(),
			MockID:              mock.MockID,
			UserID:              mock.UserID,
			InterviewID:         mock.InterviewID,
			PracticeGoalID:      mock.PracticeGoalID,
			TurnIndex:           existingTurnCount + 1,
			Role:                mockTurnRoleUser,
			TurnType:            mockTurnTypeUserAnswer,
			Phase:               MockInterviewStatusEvaluating,
			Content:             answer,
			InterviewerQuestion: currentQuestion,
			UserAnswer:          answer,
			RawAgentOutput:      raw,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
	}
	errorTurnIndex := existingTurnCount + 2
	if userTurn.TurnIndex >= errorTurnIndex {
		errorTurnIndex = userTurn.TurnIndex + 1
	}
	errorTurn := MockTurn{
		TurnID:         uuid.New().String(),
		MockID:         mock.MockID,
		UserID:         mock.UserID,
		InterviewID:    mock.InterviewID,
		PracticeGoalID: mock.PracticeGoalID,
		TurnIndex:      errorTurnIndex,
		Role:           mockTurnRoleSystem,
		TurnType:       mockTurnTypeError,
		Phase:          MockInterviewStatusFailed,
		Content:        message,
		ErrorMessage:   message,
		RawAgentOutput: raw,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if len(existingUserTurn) == 0 || existingUserTurn[0] == nil {
			if err := tx.Create(userTurn).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&errorTurn).Error; err != nil {
			return err
		}
		return tx.Model(&MockInterview{}).
			Where("mock_id = ?", mock.MockID).
			Updates(map[string]any{
				"status":           MockInterviewStatusFailed,
				"raw_agent_output": raw,
				"error_message":    message,
				"updated_at":       now,
			}).Error
	})
}

func (s *Server) ResumeFailedMockInterview(ctx context.Context, mockID string) (vo.MockTurnVO, error) {
	var mock MockInterview
	if err := s.db.First(&mock, "mock_id = ?", mockID).Error; err != nil {
		return vo.MockTurnVO{}, err
	}
	if mock.Status != MockInterviewStatusFailed {
		return vo.MockTurnVO{}, fmt.Errorf("mock interview status must be 'failed' to resume, got %q", mock.Status)
	}
	if mock.FailedRetryCount >= maxFailedSessionRetries {
		return vo.MockTurnVO{}, fmt.Errorf("retry limit reached (%d)", maxFailedSessionRetries)
	}
	userTurn, err := s.latestFailedMockUserTurn(mock.MockID)
	if err != nil {
		return vo.MockTurnVO{}, err
	}
	now := time.Now().Unix()
	if err := s.db.Model(&MockInterview{}).
		Where("mock_id = ?", mock.MockID).
		Updates(map[string]any{
			"status":             MockInterviewStatusRetriableFailed,
			"failed_retry_count": mock.FailedRetryCount + 1,
			"updated_at":         now,
		}).Error; err != nil {
		return vo.MockTurnVO{}, err
	}
	answer := userTurn.UserAnswer
	if strings.TrimSpace(answer) == "" {
		answer = userTurn.Content
	}
	return s.submitMockTurnInternal(ctx, mockID, vo.SubmitMockTurnReq{
		Answer:     answer,
		SubmitMode: mockSubmitModeFormalAnswer,
	}, &userTurn)
}

func (s *Server) GetMockInterview(mockID string) (vo.MockInterviewVO, error) {
	var mock MockInterview
	if err := s.db.First(&mock, "mock_id = ?", mockID).Error; err != nil {
		return vo.MockInterviewVO{}, err
	}
	return toMockInterviewVO(mock), nil
}

func (s *Server) ListMockTurns(mockID string) ([]vo.MockTurnVO, error) {
	turns, err := s.loadMockTurns(mockID)
	if err != nil {
		return nil, err
	}
	result := make([]vo.MockTurnVO, 0, len(turns))
	for _, turn := range turns {
		result = append(result, toMockTurnVO(turn))
	}
	return result, nil
}

func (s *Server) CancelMockInterview(mockID string) (vo.MockInterviewVO, error) {
	var mock MockInterview
	if err := s.db.First(&mock, "mock_id = ?", mockID).Error; err != nil {
		return vo.MockInterviewVO{}, err
	}
	if isMockTerminalStatus(mock.Status) {
		return toMockInterviewVO(mock), nil
	}
	now := time.Now().Unix()
	cancelTurn := MockTurn{
		TurnID:         uuid.New().String(),
		MockID:         mock.MockID,
		UserID:         mock.UserID,
		InterviewID:    mock.InterviewID,
		PracticeGoalID: mock.PracticeGoalID,
		TurnIndex:      s.nextMockTurnIndex(mockID),
		Role:           mockTurnRoleSystem,
		TurnType:       mockTurnTypeCancellationSummary,
		Phase:          MockInterviewStatusCancelled,
		AgentAction:    mockInputTypeCancel,
		Content:        "Mock interview cancelled.",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&cancelTurn).Error; err != nil {
			return err
		}
		return tx.Model(&MockInterview{}).
			Where("mock_id = ?", mockID).
			Updates(map[string]any{"status": MockInterviewStatusCancelled, "updated_at": now}).Error
	})
	if err != nil {
		return vo.MockInterviewVO{}, err
	}
	mock.Status = MockInterviewStatusCancelled
	mock.UpdatedAt = now
	return toMockInterviewVO(mock), nil
}

func (s *Server) CompleteMockInterview(ctx context.Context, mockID string) (vo.MockInterviewVO, error) {
	if s.agents == nil {
		return vo.MockInterviewVO{}, fmt.Errorf("agent provider is nil")
	}

	var mock MockInterview
	if err := s.db.First(&mock, "mock_id = ?", mockID).Error; err != nil {
		return vo.MockInterviewVO{}, err
	}
	if mock.Status == MockInterviewStatusCompleted {
		return toMockInterviewVO(mock), nil
	}
	if mock.Status == MockInterviewStatusCancelled || mock.Status == MockInterviewStatusFailed {
		return vo.MockInterviewVO{}, fmt.Errorf("mock interview status %q cannot be completed", mock.Status)
	}

	turns, err := s.loadMockTurns(mockID)
	if err != nil {
		return vo.MockInterviewVO{}, err
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeMockInterviewer))
	if err != nil {
		return vo.MockInterviewVO{}, err
	}

	result, err := runner.RunTask(ctx, buildMockCompletePrompt(mock, turns))
	if err != nil {
		log.Warnf("mock interviewer complete failed for mock %s: %v", mockID, err)
		_ = s.db.Model(&MockInterview{}).
			Where("mock_id = ?", mockID).
			Updates(map[string]any{
				"status":           MockInterviewStatusFailed,
				"raw_agent_output": fallbackRaw(result.Response, err),
				"error_message":    err.Error(),
				"updated_at":       time.Now().Unix(),
			}).Error
		return vo.MockInterviewVO{}, fmt.Errorf("mock interviewer complete failed: %w", err)
	}

	parsed, err := parseMockCompleteOutput(result.Response)
	if err != nil {
		log.Warnf("parse mock complete output failed for mock %s: %v, raw=%s", mockID, err, result.Response)
		_ = s.db.Model(&MockInterview{}).
			Where("mock_id = ?", mockID).
			Updates(map[string]any{
				"status":           MockInterviewStatusFailed,
				"raw_agent_output": result.Response,
				"error_message":    err.Error(),
				"updated_at":       time.Now().Unix(),
			}).Error
		return vo.MockInterviewVO{}, err
	}

	now := time.Now().Unix()
	closingTurn := MockTurn{
		TurnID:         uuid.New().String(),
		MockID:         mock.MockID,
		UserID:         mock.UserID,
		InterviewID:    mock.InterviewID,
		PracticeGoalID: mock.PracticeGoalID,
		TurnIndex:      len(turns) + 1,
		Role:           mockTurnRoleAssistant,
		TurnType:       mockTurnTypeClosingSummary,
		Phase:          MockInterviewStatusCompleted,
		AgentAction:    mockNextActionComplete,
		Content:        parsed.FinalSummary,
		RawAgentOutput: result.Response,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	mock.Status = MockInterviewStatusCompleted
	mock.FinalSummary = parsed.FinalSummary
	mock.RawAgentOutput = result.Response
	mock.UpdatedAt = now
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&closingTurn).Error; err != nil {
			return err
		}
		return tx.Save(&mock).Error
	}); err != nil {
		return vo.MockInterviewVO{}, err
	}
	return toMockInterviewVO(mock), nil
}

func (s *Server) loadMockInput(interviewID string, userID string, planID string, targetRound string, currentTask string) (mockInput, error) {
	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return mockInput{}, err
	}

	var report InterviewReviewReport
	if err := s.db.First(&report, "interview_id = ?", interviewID).Error; err != nil {
		return mockInput{}, err
	}
	if report.Status != InterviewReviewStatusGenerated {
		return mockInput{}, fmt.Errorf("review report must be %q before starting mock interview", InterviewReviewStatusGenerated)
	}

	var questions []InterviewQuestion
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("sequence asc").
		Find(&questions).Error; err != nil {
		return mockInput{}, err
	}

	selection, err := s.SelectMemoriesForMock(MemorySelectionRequest{
		UserID:              userID,
		CompanyName:         session.CompanyName,
		JobTitle:            session.JobTitle,
		TargetRound:         targetRound,
		CurrentTask:         currentTask,
		LimitMemoryItems:    defaultMemorySelectionLimit,
		LimitPracticeStates: defaultPracticeStateSelectionLimit,
	})
	if err != nil {
		return mockInput{}, err
	}
	questionBankResults, err := SearchQuestions(currentTask, "", session.CompanyName, 3)
	if err != nil {
		questionBankResults = nil
	}

	var plan *CoachingPlan
	var tasks []CoachingTask
	if planID != "" {
		var loadedPlan CoachingPlan
		if err := s.db.First(&loadedPlan, "plan_id = ?", planID).Error; err != nil {
			return mockInput{}, err
		}
		if loadedPlan.InterviewID != interviewID {
			return mockInput{}, fmt.Errorf("coaching plan does not belong to interview")
		}
		plan = &loadedPlan
		if err := s.db.Where("plan_id = ?", planID).
			Order("sequence asc").
			Find(&tasks).Error; err != nil {
			return mockInput{}, err
		}
	}

	return mockInput{
		sourceType:    CoachingPlanSourceInterview,
		session:       session,
		report:        report,
		questions:     questions,
		selection:     selection,
		questionBank:  questionBankResults,
		coachingPlan:  plan,
		coachingTasks: tasks,
	}, nil
}

func (s *Server) loadPracticeGoalMockInput(goalID string, userID string, planID string, targetRound string, currentTask string) (mockInput, error) {
	goal, err := firstPracticeGoalByID(s.db, goalID)
	if err != nil {
		return mockInput{}, err
	}
	if goal.UserID != strings.TrimSpace(userID) {
		return mockInput{}, fmt.Errorf("practice goal user_id mismatch")
	}
	if goal.Status != PracticeGoalStatusActive {
		return mockInput{}, fmt.Errorf("practice goal status must be %q", PracticeGoalStatusActive)
	}
	targetRound = normalizeDefault(strings.TrimSpace(targetRound), goal.TargetRound)
	selectionTask := strings.TrimSpace(currentTask)
	if selectionTask == "" {
		selectionTask = strings.Join(unmarshalStringSlice(goal.FocusTopics), " ")
	}
	questionBankResults, err := SearchQuestions(selectionTask, "", goal.CompanyName, 3)
	if err != nil {
		questionBankResults = nil
	}
	selection, err := s.SelectMemoriesForMock(MemorySelectionRequest{
		UserID:              goal.UserID,
		CompanyName:         goal.CompanyName,
		JobTitle:            goal.JobTitle,
		TargetRound:         targetRound,
		CurrentTask:         selectionTask,
		LimitMemoryItems:    defaultMemorySelectionLimit,
		LimitPracticeStates: defaultPracticeStateSelectionLimit,
	})
	if err != nil {
		return mockInput{}, err
	}

	var plan *CoachingPlan
	var tasks []CoachingTask
	if planID != "" {
		var loadedPlan CoachingPlan
		if err := s.db.First(&loadedPlan, "plan_id = ?", planID).Error; err != nil {
			return mockInput{}, err
		}
		if loadedPlan.PracticeGoalID != goal.GoalID || loadedPlan.UserID != goal.UserID {
			return mockInput{}, fmt.Errorf("coaching plan does not belong to practice goal")
		}
		plan = &loadedPlan
		if err := s.db.Where("plan_id = ?", planID).
			Order("sequence asc").
			Find(&tasks).Error; err != nil {
			return mockInput{}, err
		}
	}

	return mockInput{
		sourceType:    CoachingPlanSourcePracticeGoal,
		practiceGoal:  &goal,
		selection:     selection,
		questionBank:  questionBankResults,
		coachingPlan:  plan,
		coachingTasks: tasks,
	}, nil
}

func (s *Server) loadMockTurns(mockID string) ([]MockTurn, error) {
	var turns []MockTurn
	if err := s.db.Where("mock_id = ?", mockID).
		Order("turn_index asc, created_at asc").
		Find(&turns).Error; err != nil {
		return nil, err
	}
	return turns, nil
}

func (s *Server) findActiveMockInterview(interviewID string, userID string, planID string, targetRound string) (MockInterview, bool, error) {
	var mock MockInterview
	q := s.db.Where("interview_id = ? AND user_id = ? AND target_round = ? AND status IN ?",
		interviewID, userID, targetRound, activeMockStatuses())
	if planID == "" {
		q = q.Where("plan_id = ''")
	} else {
		q = q.Where("plan_id = ?", planID)
	}
	err := q.Order("updated_at desc").First(&mock).Error
	if err == nil {
		return mock, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return MockInterview{}, false, nil
	}
	return MockInterview{}, false, err
}

func (s *Server) findActivePracticeGoalMockInterview(goalID string, userID string, planID string, targetRound string) (MockInterview, bool, error) {
	var mock MockInterview
	q := s.db.Where("practice_goal_id = ? AND user_id = ? AND target_round = ? AND status IN ?",
		goalID, userID, targetRound, activeMockStatuses())
	if planID == "" {
		q = q.Where("plan_id = ''")
	} else {
		q = q.Where("plan_id = ?", planID)
	}
	err := q.Order("updated_at desc").First(&mock).Error
	if err == nil {
		return mock, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return MockInterview{}, false, nil
	}
	return MockInterview{}, false, err
}

func activeMockStatuses() []string {
	return []string{
		MockInterviewStatusCreated,
		MockInterviewStatusInProgress,
		MockInterviewStatusWaitingAnswer,
		MockInterviewStatusEvaluating,
		MockInterviewStatusAskingFollowup,
		MockInterviewStatusSwitchingTopic,
		MockInterviewStatusRetriableFailed,
	}
}

func isMockSubmittableStatus(status string) bool {
	for _, active := range activeMockStatuses() {
		if status == active {
			return true
		}
	}
	return false
}

func isMockTerminalStatus(status string) bool {
	return status == MockInterviewStatusCompleted ||
		status == MockInterviewStatusFailed ||
		status == MockInterviewStatusCancelled
}

func currentMockQuestion(mock MockInterview, turns []MockTurn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if turn.Role != mockTurnRoleAssistant {
			continue
		}
		switch turn.TurnType {
		case mockTurnTypeOpeningQuestion, mockTurnTypeFollowupQuestion, mockTurnTypeTopicSwitch:
			if strings.TrimSpace(turn.NextQuestion) != "" {
				return turn.NextQuestion
			}
			if strings.TrimSpace(turn.Content) != "" {
				return turn.Content
			}
			if strings.TrimSpace(turn.InterviewerQuestion) != "" {
				return turn.InterviewerQuestion
			}
		}
		if strings.TrimSpace(turn.NextQuestion) != "" {
			return turn.NextQuestion
		}
	}
	return mock.FirstQuestion
}

func (s *Server) nextMockTurnIndex(mockID string) int {
	var maxIndex int
	_ = s.db.Model(&MockTurn{}).
		Where("mock_id = ?", mockID).
		Select("COALESCE(MAX(turn_index), 0)").
		Scan(&maxIndex).Error
	return maxIndex + 1
}

func buildMockStartPrompt(input mockInput, req vo.StartMockInterviewReq) string {
	return fmt.Sprintf(`Start a text-only mock interview.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.

Do not write long-term memory.
Do not create coaching plans.
Act as the interviewer and produce the simulation goal plus the first question.

JSON schema:
{
  "overall_goal": "string",
  "first_question": "string",
  "time_limit_seconds": 900,
  "time_pressure_style": "none|moderate|strict",
  "warn_at_seconds": 300
}

Source context:
%s

Review report:
%s

Structured questions:
%s

Selected memory_items:
%s

Selected practice_states:
%s

Question bank candidates:
%s

Coaching plan and tasks:
%s

Target round: %s`,
		mockSourceContextJSON(input),
		mockReportJSON(input.report),
		mockQuestionsJSON(input.questions),
		selectedMemoriesJSON(input.selection.MemoryItems),
		selectedPracticeStatesJSON(input.selection.PracticeStates),
		formatQuestionBankPromptSection(input.questionBank),
		mockCoachingJSON(input.coachingPlan, input.coachingTasks),
		req.TargetRound,
	)
}

func buildMockTurnPrompt(input mockInput, mock MockInterview, turns []MockTurn, currentQuestion string, answer string, submitMode string) string {
	return fmt.Sprintf(`你是固定的 mock_interviewer Agent，正在继续一次文本模拟面试。

只返回严格 JSON。不要返回 Markdown、代码块或 JSON 外解释。
不要写入 memory_items。不要调用任何 tools。不要新增 Agent。不要创建 coaching plans。

本轮 submit_mode: %s

新的 JSON schema:
{
  "visible_message": "给用户看的中文回复",
  "user_intent": "answer|ask_hint|ask_explain|smalltalk|unclear|cancel",
  "state_action": "record_attempt|chat_only|stay_current|cancel",
  "confidence": 0.0,
  "needs_clarification": false,
  "score": 72,
  "feedback": "正式评分反馈；没有正式作答时为空字符串",
  "topic": "primary topic",
  "weakness_tags": ["string"],
  "practice_updates": [{"topic":"string","score":72,"feedback":"string"}],
  "input_type": "formal_answer|hint_request|explanation_request|cancel",
  "agent_message": "兼容旧字段；内容与 visible_message 一致",
  "next_action": "ask_followup|switch_topic|complete|wait_for_answer",
  "should_update_practice_state": true,
  "should_complete_mock": false,
  "follow_up_reason": "string",
  "next_question": "string"
}

规则:
- submit_mode=formal_answer 且用户确实在回答当前面试题时，使用 user_intent=answer + state_action=record_attempt，score 0-100，并给出具体 feedback 和 practice_updates。
- submit_mode=chat 表示场外提问、提示、解释、寒暄或不清楚表达；必须使用 state_action=chat_only/stay_current，不要使用 record_attempt，不要打分，不要写 feedback，不要写 practice_updates。
- ask_hint、ask_explain、smalltalk、unclear 都不是正式回答；保持当前 interviewer question active，next_action=wait_for_answer。
- 只有 cancel 才使用 state_action=cancel；取消时不打分、不更新 practice。
- visible_message 是用户会看到的回复，默认中文、简洁、直接。
- 兼容旧字段 input_type、agent_message、next_action，但新字段 user_intent 和 state_action 决定服务端行为。

Mock interview:
%s

Source context:
%s

Review report:
%s

Structured questions:
%s

Selected memory_items:
%s

Selected practice_states:
%s

Coaching plan and tasks:
%s

Previous turns:
%s

Current interviewer question:
%s

Candidate answer:
%s`,
		submitMode,
		mockInterviewJSON(mock),
		mockSourceContextJSON(input),
		mockReportJSON(input.report),
		mockQuestionsJSON(input.questions),
		selectedMemoriesJSON(input.selection.MemoryItems),
		selectedPracticeStatesJSON(input.selection.PracticeStates),
		mockCoachingJSON(input.coachingPlan, input.coachingTasks),
		mockTurnsJSON(turns),
		currentQuestion,
		answer,
	)
}

func buildMockCompletePrompt(mock MockInterview, turns []MockTurn) string {
	return fmt.Sprintf(`Summarize this text-only mock interview.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.

JSON schema:
{
  "final_summary": "string"
}

Mock interview:
%s

Turns:
%s`,
		mockInterviewJSON(mock),
		mockTurnsJSON(turns),
	)
}

func parseMockStartOutput(raw string) (mockStartOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed mockStartOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return mockStartOutput{}, fmt.Errorf("parse mock start JSON: %w", err)
	}
	return parsed, nil
}

func parseMockTurnOutput(raw string, submitMode string) (mockTurnOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed mockTurnOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return mockTurnOutput{}, fmt.Errorf("parse mock turn JSON: %w", err)
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &rawFields); err != nil {
		return mockTurnOutput{}, fmt.Errorf("parse mock turn JSON object: %w", err)
	}
	return normalizeMockTurnOutput(parsed, rawFields, submitMode), nil
}

func normalizeMockTurnOutput(parsed mockTurnOutput, rawFields map[string]json.RawMessage, submitMode string) mockTurnOutput {
	submitMode = normalizeMockSubmitMode(submitMode)
	_, hasUserIntent := rawFields["user_intent"]
	_, hasStateAction := rawFields["state_action"]
	legacyInputType := normalizeMockInputType(parsed.InputType)
	legacyNextAction := normalizeMockNextAction(parsed.NextAction)

	parsed.SubmitMode = submitMode
	parsed.InputType = legacyInputType
	parsed.NextAction = legacyNextAction
	parsed.UserIntent = normalizeMockUserIntent(parsed.UserIntent)
	parsed.StateAction = normalizeMockStateAction(parsed.StateAction)
	if strings.TrimSpace(parsed.VisibleMessage) == "" {
		parsed.VisibleMessage = strings.TrimSpace(parsed.AgentMessage)
	}
	if strings.TrimSpace(parsed.AgentMessage) == "" {
		parsed.AgentMessage = strings.TrimSpace(parsed.VisibleMessage)
	}
	if !hasUserIntent {
		parsed.UserIntent = legacyMockUserIntent(legacyInputType)
	}
	if !hasStateAction {
		parsed.StateAction = legacyMockStateAction(legacyInputType)
	}
	if submitMode == mockSubmitModeChat && parsed.StateAction == mockStateActionRecordAttempt {
		parsed.StateAction = mockStateActionChatOnly
		parsed.DefenseRules = append(parsed.DefenseRules, DefenseRuleDecision{
			RuleID:  DefenseRuleChatSubmitModeForcesChatOnly,
			Applied: true,
			Reason:  "submit_mode=chat forced state_action to chat_only",
		})
	}
	if parsed.StateAction == mockStateActionRecordAttempt && parsed.UserIntent != mockUserIntentAnswer {
		parsed.StateAction = mockStateActionChatOnly
		parsed.DefenseRules = append(parsed.DefenseRules, DefenseRuleDecision{
			RuleID:  DefenseRuleChatSubmitModeForcesChatOnly,
			Applied: true,
			Reason:  "record_attempt rejected for non-answer intent, forced chat_only",
		})
	}
	if parsed.UserIntent == mockUserIntentSmalltalk || parsed.UserIntent == mockUserIntentUnclear ||
		parsed.UserIntent == mockUserIntentAskHint || parsed.UserIntent == mockUserIntentAskExplain {
		if parsed.StateAction != mockStateActionCancel {
			parsed.StateAction = mockStateActionChatOnly
			if parsed.UserIntent == mockUserIntentSmalltalk || parsed.UserIntent == mockUserIntentUnclear {
				parsed.DefenseRules = append(parsed.DefenseRules, DefenseRuleDecision{
					RuleID:  DefenseRuleSmalltalkUnclearChatOnly,
					Applied: true,
					Reason:  fmt.Sprintf("user_intent=%s forced state_action to chat_only", parsed.UserIntent),
				})
			}
		}
	}
	parsed.InputType = mockInputTypeForIntent(parsed.UserIntent, legacyInputType)
	parsed.NextAction = mockNextActionForStateAction(parsed.StateAction, legacyNextAction)
	if parsed.ShouldCompleteMock {
		parsed.NextAction = mockNextActionComplete
	}
	if rawStateAction, ok := rawFields["state_action"]; ok {
		rawValue := strings.Trim(string(rawStateAction), `"`)
		if rawValue != "" && normalizeMockStateAction(rawValue) == mockStateActionChatOnly && rawValue != mockStateActionChatOnly {
			parsed.DefenseRules = append(parsed.DefenseRules, DefenseRuleDecision{
				RuleID:  DefenseRuleStateActionWhitelist,
				Applied: true,
				Reason:  fmt.Sprintf("unknown state_action %q fell back to chat_only", rawValue),
			})
		}
	}
	if parsed.Score < 0 || parsed.Score > 100 {
		parsed.DefenseRules = append(parsed.DefenseRules, DefenseRuleDecision{
			RuleID:  DefenseRuleRecordAttemptScoreRange,
			Applied: true,
			Reason:  fmt.Sprintf("score %d is outside valid range 0-100", parsed.Score),
		})
	}
	parsed.DefenseRules = append(parsed.DefenseRules, DefenseRuleDecision{
		RuleID:  DefenseRuleMemoryItemsWriteWarning,
		Applied: false,
		Reason:  "memory_items write boundary check is a passive warning; enforcement occurs at service_actions level",
	})
	if parsed.TopicTags == nil {
		parsed.TopicTags = []string{}
	}
	if len(parsed.TopicTags) == 0 && len(parsed.WeaknessTags) > 0 {
		parsed.TopicTags = parsed.WeaknessTags
	}
	if parsed.AgentMessage == "" {
		if parsed.NextQuestion != "" {
			parsed.AgentMessage = parsed.NextQuestion
		} else if parsed.Feedback != "" {
			parsed.AgentMessage = parsed.Feedback
		}
	}
	if strings.TrimSpace(parsed.VisibleMessage) == "" {
		parsed.VisibleMessage = strings.TrimSpace(parsed.AgentMessage)
	}
	if parsed.NextQuestion == "" && (parsed.NextAction == mockNextActionAskFollowup || parsed.NextAction == mockNextActionSwitchTopic) {
		parsed.NextQuestion = mockVisibleMessage(parsed)
	}
	if parsed.Topic == "" && len(parsed.TopicTags) > 0 {
		parsed.Topic = parsed.TopicTags[0]
	}
	if isFormalMockAttempt(parsed) && len(parsed.TopicTags) > 0 && len(parsed.PracticeUpdates) == 0 {
		parsed.ShouldUpdatePracticeState = true
	}
	if !isFormalMockAttempt(parsed) {
		parsed = clearMockFormalScoringMetadata(parsed)
	}
	return parsed
}

func normalizeMockSubmitMode(submitMode string) string {
	if strings.TrimSpace(submitMode) == mockSubmitModeChat {
		return mockSubmitModeChat
	}
	return mockSubmitModeFormalAnswer
}

func normalizeMockInputType(inputType string) string {
	switch strings.TrimSpace(inputType) {
	case mockInputTypeFormalAnswer,
		mockInputTypeHintRequest,
		mockInputTypeExplanationRequest,
		mockInputTypeCancel:
		return strings.TrimSpace(inputType)
	default:
		return mockInputTypeFormalAnswer
	}
}

func normalizeMockNextAction(nextAction string) string {
	switch strings.TrimSpace(nextAction) {
	case mockNextActionAskFollowup,
		mockNextActionSwitchTopic,
		mockNextActionComplete,
		mockNextActionWaitForInput:
		return strings.TrimSpace(nextAction)
	default:
		return mockNextActionAskFollowup
	}
}

func normalizeMockUserIntent(userIntent string) string {
	switch strings.TrimSpace(userIntent) {
	case mockUserIntentAnswer,
		mockUserIntentAskHint,
		mockUserIntentAskExplain,
		mockUserIntentSmalltalk,
		mockUserIntentUnclear,
		mockUserIntentCancel:
		return strings.TrimSpace(userIntent)
	default:
		return mockUserIntentUnclear
	}
}

func normalizeMockStateAction(stateAction string) string {
	switch strings.TrimSpace(stateAction) {
	case mockStateActionRecordAttempt,
		mockStateActionChatOnly,
		mockStateActionStayCurrent,
		mockStateActionCancel:
		return strings.TrimSpace(stateAction)
	default:
		return mockStateActionChatOnly
	}
}

func legacyMockUserIntent(inputType string) string {
	switch normalizeMockInputType(inputType) {
	case mockInputTypeHintRequest:
		return mockUserIntentAskHint
	case mockInputTypeExplanationRequest:
		return mockUserIntentAskExplain
	case mockInputTypeCancel:
		return mockUserIntentCancel
	default:
		return mockUserIntentAnswer
	}
}

func legacyMockStateAction(inputType string) string {
	switch normalizeMockInputType(inputType) {
	case mockInputTypeHintRequest, mockInputTypeExplanationRequest:
		return mockStateActionChatOnly
	case mockInputTypeCancel:
		return mockStateActionCancel
	default:
		return mockStateActionRecordAttempt
	}
}

func mockInputTypeForIntent(userIntent string, fallback string) string {
	switch normalizeMockUserIntent(userIntent) {
	case mockUserIntentAskHint:
		return mockInputTypeHintRequest
	case mockUserIntentAskExplain:
		return mockInputTypeExplanationRequest
	case mockUserIntentCancel:
		return mockInputTypeCancel
	case mockUserIntentAnswer:
		return mockInputTypeFormalAnswer
	default:
		if fallback == mockInputTypeHintRequest || fallback == mockInputTypeExplanationRequest || fallback == mockInputTypeCancel {
			return fallback
		}
		return mockInputTypeFormalAnswer
	}
}

func mockNextActionForStateAction(stateAction string, fallback string) string {
	switch normalizeMockStateAction(stateAction) {
	case mockStateActionRecordAttempt:
		return fallback
	case mockStateActionCancel, mockStateActionChatOnly, mockStateActionStayCurrent:
		return mockNextActionWaitForInput
	default:
		return mockNextActionWaitForInput
	}
}

func isFormalMockAttempt(parsed mockTurnOutput) bool {
	return parsed.SubmitMode == mockSubmitModeFormalAnswer &&
		parsed.UserIntent == mockUserIntentAnswer &&
		parsed.StateAction == mockStateActionRecordAttempt
}

func clearMockFormalScoringMetadata(parsed mockTurnOutput) mockTurnOutput {
	parsed.Score = 0
	parsed.Feedback = ""
	parsed.WeaknessTags = nil
	parsed.TopicTags = nil
	parsed.PracticeUpdates = nil
	parsed.ShouldUpdatePracticeState = false
	parsed.FollowUpReason = ""
	if parsed.StateAction != mockStateActionCancel {
		parsed.NextAction = mockNextActionWaitForInput
		parsed.NextQuestion = ""
		parsed.ShouldCompleteMock = false
	}
	return parsed
}

func mockVisibleMessage(parsed mockTurnOutput) string {
	if message := strings.TrimSpace(parsed.VisibleMessage); message != "" {
		return message
	}
	return strings.TrimSpace(parsed.AgentMessage)
}

func mockPracticeTopics(parsed mockTurnOutput) []string {
	seen := make(map[string]bool)
	var topics []string
	for _, update := range parsed.PracticeUpdates {
		topic := strings.TrimSpace(update.Topic)
		if topic != "" && !seen[topic] {
			topics = append(topics, topic)
			seen[topic] = true
		}
	}
	for _, topic := range parsed.TopicTags {
		topic = strings.TrimSpace(topic)
		if topic != "" && !seen[topic] {
			topics = append(topics, topic)
			seen[topic] = true
		}
	}
	if len(topics) == 0 && strings.TrimSpace(parsed.Topic) != "" {
		topics = append(topics, strings.TrimSpace(parsed.Topic))
	}
	return topics
}

func parseMockCompleteOutput(raw string) (mockCompleteOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed mockCompleteOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return mockCompleteOutput{}, fmt.Errorf("parse mock complete JSON: %w", err)
	}
	return parsed, nil
}

func mockSessionJSON(session InterviewSession) string {
	data, _ := json.Marshal(map[string]any{
		"user_id":         session.UserID,
		"interview_id":    session.InterviewID,
		"company_name":    session.CompanyName,
		"job_title":       session.JobTitle,
		"interview_round": session.InterviewRound,
		"interview_type":  session.InterviewType,
	})
	return string(data)
}

func mockSourceContextJSON(input mockInput) string {
	if input.practiceGoal != nil {
		return mockPracticeGoalJSON(*input.practiceGoal)
	}
	return mockSessionJSON(input.session)
}

func mockPracticeGoalJSON(goal PracticeGoal) string {
	data, _ := json.Marshal(map[string]any{
		"source_type":      CoachingPlanSourcePracticeGoal,
		"user_id":          goal.UserID,
		"practice_goal_id": goal.GoalID,
		"company_name":     goal.CompanyName,
		"job_title":        goal.JobTitle,
		"target_round":     goal.TargetRound,
		"remaining_days":   goal.RemainingDays,
		"job_description":  goal.JobDescription,
		"focus_topics":     unmarshalStringSlice(goal.FocusTopics),
	})
	return string(data)
}

func mockReportJSON(report InterviewReviewReport) string {
	data, _ := json.Marshal(map[string]any{
		"overall_summary":       report.OverallSummary,
		"strengths":             unmarshalStringSlice(report.Strengths),
		"weaknesses":            unmarshalStringSlice(report.Weaknesses),
		"follow_up_risks":       unmarshalStringSlice(report.FollowUpRisks),
		"suggested_preparation": unmarshalStringSlice(report.SuggestedPreparation),
	})
	return string(data)
}

func mockQuestionsJSON(questions []InterviewQuestion) string {
	payload := make([]map[string]any, 0, len(questions))
	for _, q := range questions {
		payload = append(payload, map[string]any{
			"sequence":               q.Sequence,
			"question":               q.Question,
			"answer_quality":         q.AnswerQuality,
			"topic_tags":             unmarshalStringSlice(q.TopicTags),
			"weakness_summary":       q.WeaknessSummary,
			"improvement_suggestion": q.ImprovementSuggestion,
		})
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func mockMemoriesJSON(memories []MemoryItem) string {
	payload := make([]map[string]any, 0, len(memories))
	for _, m := range memories {
		payload = append(payload, map[string]any{
			"memory_id":   m.MemoryID,
			"memory_type": m.MemoryType,
			"subject_key": m.SubjectKey,
			"content":     m.Content,
			"evidence":    m.Evidence,
			"confidence":  m.Confidence,
		})
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func mockCoachingJSON(plan *CoachingPlan, tasks []CoachingTask) string {
	if plan == nil {
		return "{}"
	}
	taskPayload := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		taskPayload = append(taskPayload, map[string]any{
			"sequence":           t.Sequence,
			"day_index":          t.DayIndex,
			"task_type":          t.TaskType,
			"title":              t.Title,
			"description":        t.Description,
			"related_memory_ids": unmarshalStringSlice(t.RelatedMemoryIDs),
			"priority":           t.Priority,
			"status":             t.Status,
		})
	}
	data, _ := json.Marshal(map[string]any{
		"plan_id":          plan.PlanID,
		"target_round":     plan.TargetRound,
		"remaining_days":   plan.RemainingDays,
		"overall_strategy": plan.OverallStrategy,
		"focus_summary":    plan.FocusSummary,
		"tasks":            taskPayload,
	})
	return string(data)
}

func mockInterviewJSON(mock MockInterview) string {
	data, _ := json.Marshal(map[string]any{
		"mock_id":          mock.MockID,
		"interview_id":     mock.InterviewID,
		"practice_goal_id": mock.PracticeGoalID,
		"target_round":     mock.TargetRound,
		"status":           mock.Status,
		"overall_goal":     mock.OverallGoal,
		"first_question":   mock.FirstQuestion,
		"current_turn":     mock.CurrentTurn,
		"current_topic":    mock.CurrentTopic,
		"last_feedback":    mock.LastFeedback,
	})
	return string(data)
}

func mockTurnsJSON(turns []MockTurn) string {
	payload := make([]map[string]any, 0, len(turns))
	for _, t := range turns {
		payload = append(payload, map[string]any{
			"turn_index":           t.TurnIndex,
			"role":                 t.Role,
			"turn_type":            t.TurnType,
			"phase":                t.Phase,
			"agent_action":         t.AgentAction,
			"content":              t.Content,
			"interviewer_question": t.InterviewerQuestion,
			"user_answer":          t.UserAnswer,
			"feedback":             t.Feedback,
			"score":                t.Score,
			"topic_tags":           unmarshalStringSlice(t.TopicTags),
			"next_question":        t.NextQuestion,
		})
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func fallbackRaw(raw string, err error) string {
	if raw != "" {
		return raw
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func toMockInterviewVO(mock MockInterview) vo.MockInterviewVO {
	return vo.MockInterviewVO{
		MockID:               mock.MockID,
		UserID:               mock.UserID,
		InterviewID:          mock.InterviewID,
		PracticeGoalID:       mock.PracticeGoalID,
		PlanID:               mock.PlanID,
		TargetRound:          mock.TargetRound,
		Status:               mock.Status,
		CurrentTurn:          mock.CurrentTurn,
		CurrentTopic:         mock.CurrentTopic,
		OverallGoal:          mock.OverallGoal,
		FirstQuestion:        mock.FirstQuestion,
		LastFeedback:         mock.LastFeedback,
		ErrorMessage:         mock.ErrorMessage,
		FinalSummary:         mock.FinalSummary,
		RawAgentOutput:       mock.RawAgentOutput,
		AgentPersistentState: decodePersistentStateForVO(mock.AgentPersistentState),
		FailedRetryCount:     mock.FailedRetryCount,
		CreatedAt:            mock.CreatedAt,
		UpdatedAt:            mock.UpdatedAt,
	}
}

func toMockTurnVO(turn MockTurn) vo.MockTurnVO {
	return vo.MockTurnVO{
		TurnID:              turn.TurnID,
		MockID:              turn.MockID,
		UserID:              turn.UserID,
		InterviewID:         turn.InterviewID,
		PracticeGoalID:      turn.PracticeGoalID,
		TurnIndex:           turn.TurnIndex,
		Role:                turn.Role,
		TurnType:            turn.TurnType,
		Phase:               turn.Phase,
		AgentAction:         turn.AgentAction,
		Content:             turn.Content,
		InterviewerQuestion: turn.InterviewerQuestion,
		UserAnswer:          turn.UserAnswer,
		Feedback:            turn.Feedback,
		Score:               turn.Score,
		FollowUpReason:      turn.FollowUpReason,
		TimeLimitSeconds:    turn.TimeLimitSeconds,
		TimePressureStyle:   turn.TimePressureStyle,
		WarnAtSeconds:       turn.WarnAtSeconds,
		TopicTags:           unmarshalStringSlice(turn.TopicTags),
		NextQuestion:        turn.NextQuestion,
		RawAgentOutput:      turn.RawAgentOutput,
		ErrorMessage:        turn.ErrorMessage,
		CreatedAt:           turn.CreatedAt,
		UpdatedAt:           turn.UpdatedAt,
	}
}
