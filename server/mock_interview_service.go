package server

import (
	"context"
	"encoding/json"
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
	MockInterviewStatusCreated        = "created"
	MockInterviewStatusInProgress     = "in_progress"
	MockInterviewStatusWaitingAnswer  = "waiting_answer"
	MockInterviewStatusEvaluating     = "evaluating_answer"
	MockInterviewStatusAskingFollowup = "asking_followup"
	MockInterviewStatusSwitchingTopic = "switching_topic"
	MockInterviewStatusCompleted      = "completed"
	MockInterviewStatusFailed         = "failed"
	MockInterviewStatusCancelled      = "cancelled"

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
)

type mockStartOutput struct {
	OverallGoal   string `json:"overall_goal"`
	FirstQuestion string `json:"first_question"`
}

type mockTurnOutput struct {
	InputType                 string               `json:"input_type"`
	AgentMessage              string               `json:"agent_message"`
	Score                     int                  `json:"score"`
	Feedback                  string               `json:"feedback"`
	Topic                     string               `json:"topic"`
	WeaknessTags              []string             `json:"weakness_tags"`
	NextAction                string               `json:"next_action"`
	ShouldUpdatePracticeState bool                 `json:"should_update_practice_state"`
	PracticeUpdates           []mockPracticeUpdate `json:"practice_updates"`
	ShouldCompleteMock        bool                 `json:"should_complete_mock"`
	FollowUpReason            string               `json:"follow_up_reason"`
	TopicTags                 []string             `json:"topic_tags"`
	NextQuestion              string               `json:"next_question"`
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
	session       InterviewSession
	report        InterviewReviewReport
	questions     []InterviewQuestion
	selection     MemorySelectionResult
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

	input, err := s.loadMockInput(interviewID, req.UserID, req.PlanID, req.TargetRound, MemorySelectorTaskMockStart)
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

func (s *Server) SubmitMockTurn(ctx context.Context, mockID string, req vo.SubmitMockTurnReq) (vo.MockTurnVO, error) {
	if s.agents == nil {
		return vo.MockTurnVO{}, fmt.Errorf("agent provider is nil")
	}

	var mock MockInterview
	if err := s.db.First(&mock, "mock_id = ?", mockID).Error; err != nil {
		return vo.MockTurnVO{}, err
	}
	if !isMockSubmittableStatus(mock.Status) {
		return vo.MockTurnVO{}, fmt.Errorf("mock interview status %q does not accept turns", mock.Status)
	}

	turns, err := s.loadMockTurns(mockID)
	if err != nil {
		return vo.MockTurnVO{}, err
	}
	input, err := s.loadMockInput(mock.InterviewID, mock.UserID, mock.PlanID, mock.TargetRound, MemorySelectorTaskMockTurn)
	if err != nil {
		return vo.MockTurnVO{}, err
	}

	currentQuestion := currentMockQuestion(mock, turns)

	_, runner, err := s.agents.Get(string(agent.AgentTypeMockInterviewer))
	if err != nil {
		return vo.MockTurnVO{}, err
	}

	prompt := buildMockTurnPrompt(input, mock, turns, currentQuestion, req.Answer)
	inputSnapshot := marshalTraceJSON(map[string]any{
		"mock_id":                 mock.MockID,
		"interview_id":            mock.InterviewID,
		"user_id":                 mock.UserID,
		"plan_id":                 mock.PlanID,
		"target_round":            mock.TargetRound,
		"mock_status":             mock.Status,
		"current_turn":            mock.CurrentTurn,
		"current_topic":           mock.CurrentTopic,
		"existing_turn_count":     len(turns),
		"current_question_length": len(currentQuestion),
		"answer_length":           len(req.Answer),
		"question_count":          len(input.questions),
		"prompt_length":           len(prompt),
	})
	selectedContextSnapshot := buildSelectedContextTraceSnapshot(input.selection)

	result, err := runner.RunTask(ctx, prompt)
	if err != nil {
		log.Warnf("mock interviewer turn failed for mock %s: %v", mockID, err)
		_ = s.failMockTurn(mock, len(turns), currentQuestion, req.Answer, fallbackRaw(result.Response, err), err.Error())
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

	parsed, err := parseMockTurnOutput(result.Response)
	if err != nil {
		log.Warnf("parse mock turn output failed for mock %s: %v, raw=%s", mockID, err, result.Response)
		_ = s.failMockTurn(mock, len(turns), currentQuestion, req.Answer, result.Response, err.Error())
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
	userTurn := MockTurn{
		TurnID:              uuid.New().String(),
		MockID:              mock.MockID,
		UserID:              mock.UserID,
		InterviewID:         mock.InterviewID,
		TurnIndex:           nextIndex,
		Role:                mockTurnRoleUser,
		TurnType:            userTurnType,
		Phase:               MockInterviewStatusEvaluating,
		Content:             req.Answer,
		InterviewerQuestion: currentQuestion,
		UserAnswer:          req.Answer,
		RawAgentOutput:      result.Response,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	nextIndex++

	created := []MockTurn{userTurn}
	updates := map[string]any{
		"raw_agent_output": result.Response,
		"updated_at":       now,
	}
	var responseTurn MockTurn

	if parsed.InputType == mockInputTypeCancel {
		cancelTurn := MockTurn{
			TurnID:         uuid.New().String(),
			MockID:         mock.MockID,
			UserID:         mock.UserID,
			InterviewID:    mock.InterviewID,
			TurnIndex:      nextIndex,
			Role:           mockTurnRoleAssistant,
			TurnType:       mockTurnTypeCancellationSummary,
			Phase:          MockInterviewStatusCancelled,
			AgentAction:    mockInputTypeCancel,
			Content:        parsed.AgentMessage,
			RawAgentOutput: result.Response,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		created = append(created, cancelTurn)
		updates["status"] = MockInterviewStatusCancelled
		updates["last_feedback"] = parsed.AgentMessage
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
			TurnIndex:           nextIndex,
			Role:                mockTurnRoleAssistant,
			TurnType:            assistantType,
			Phase:               MockInterviewStatusWaitingAnswer,
			AgentAction:         mockNextActionWaitForInput,
			Content:             parsed.AgentMessage,
			InterviewerQuestion: currentQuestion,
			NextQuestion:        currentQuestion,
			RawAgentOutput:      result.Response,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		created = append(created, assistantTurn)
		updates["status"] = MockInterviewStatusWaitingAnswer
		responseTurn = assistantTurn
	} else {
		topics := mockPracticeTopics(parsed)
		evaluationTurn := MockTurn{
			TurnID:              uuid.New().String(),
			MockID:              mock.MockID,
			UserID:              mock.UserID,
			InterviewID:         mock.InterviewID,
			TurnIndex:           nextIndex,
			Role:                mockTurnRoleAssistant,
			TurnType:            mockTurnTypeEvaluationFeedback,
			Phase:               MockInterviewStatusEvaluating,
			Content:             parsed.Feedback,
			InterviewerQuestion: currentQuestion,
			UserAnswer:          req.Answer,
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
			updates["final_summary"] = parsed.AgentMessage
		} else {
			updates["status"] = MockInterviewStatusWaitingAnswer
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
			return tx.Model(&MockInterview{}).
				Where("mock_id = ?", mockID).
				Updates(updates).Error
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
			ServiceActions:          marshalTraceJSON(mockTurnTraceActions(parsed, len(created))),
			Status:                  AgentDecisionTraceStatusSucceeded,
		})
		return toMockTurnVO(responseTurn), nil
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, turn := range created {
			if err := tx.Create(&turn).Error; err != nil {
				return err
			}
		}
		return tx.Model(&MockInterview{}).
			Where("mock_id = ?", mockID).
			Updates(updates).Error
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
		ServiceActions:          marshalTraceJSON(mockTurnTraceActions(parsed, len(created))),
		Status:                  AgentDecisionTraceStatusSucceeded,
	})
	return toMockTurnVO(responseTurn), nil
}

func buildMockActionTurn(mock MockInterview, turnIndex int, parsed mockTurnOutput, currentQuestion string, raw string, now int64) MockTurn {
	message := parsed.AgentMessage
	action := parsed.NextAction
	if action == "" {
		action = mockNextActionAskFollowup
	}
	turn := MockTurn{
		TurnID:              uuid.New().String(),
		MockID:              mock.MockID,
		UserID:              mock.UserID,
		InterviewID:         mock.InterviewID,
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

func mockTurnTraceActions(parsed mockTurnOutput, createdTurnCount int) []string {
	actions := []string{
		fmt.Sprintf("created mock_turns: %d", createdTurnCount),
		"updated mock status",
	}
	if parsed.InputType == mockInputTypeFormalAnswer {
		if parsed.ShouldUpdatePracticeState {
			actions = append(actions, "updated practice_states")
		} else {
			actions = append(actions, "skipped practice update")
		}
	}
	if parsed.InputType == mockInputTypeHintRequest || parsed.InputType == mockInputTypeExplanationRequest {
		actions = append(actions, "skipped practice update")
	}
	if parsed.ShouldCompleteMock || parsed.NextAction == mockNextActionComplete {
		actions = append(actions, "updated mock status completed")
	}
	return actions
}

func (s *Server) failMockTurn(mock MockInterview, existingTurnCount int, currentQuestion string, answer string, raw string, message string) error {
	now := time.Now().Unix()
	userTurn := MockTurn{
		TurnID:              uuid.New().String(),
		MockID:              mock.MockID,
		UserID:              mock.UserID,
		InterviewID:         mock.InterviewID,
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
	errorTurn := MockTurn{
		TurnID:         uuid.New().String(),
		MockID:         mock.MockID,
		UserID:         mock.UserID,
		InterviewID:    mock.InterviewID,
		TurnIndex:      existingTurnCount + 2,
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
		if err := tx.Create(&userTurn).Error; err != nil {
			return err
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
		TurnID:      uuid.New().String(),
		MockID:      mock.MockID,
		UserID:      mock.UserID,
		InterviewID: mock.InterviewID,
		TurnIndex:   s.nextMockTurnIndex(mockID),
		Role:        mockTurnRoleSystem,
		TurnType:    mockTurnTypeCancellationSummary,
		Phase:       MockInterviewStatusCancelled,
		AgentAction: mockInputTypeCancel,
		Content:     "Mock interview cancelled.",
		CreatedAt:   now,
		UpdatedAt:   now,
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
		session:       session,
		report:        report,
		questions:     questions,
		selection:     selection,
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

func activeMockStatuses() []string {
	return []string{
		MockInterviewStatusCreated,
		MockInterviewStatusInProgress,
		MockInterviewStatusWaitingAnswer,
		MockInterviewStatusEvaluating,
		MockInterviewStatusAskingFollowup,
		MockInterviewStatusSwitchingTopic,
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
  "first_question": "string"
}

Interview context:
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

Target round: %s`,
		mockSessionJSON(input.session),
		mockReportJSON(input.report),
		mockQuestionsJSON(input.questions),
		selectedMemoriesJSON(input.selection.MemoryItems),
		selectedPracticeStatesJSON(input.selection.PracticeStates),
		mockCoachingJSON(input.coachingPlan, input.coachingTasks),
		req.TargetRound,
	)
}

func buildMockTurnPrompt(input mockInput, mock MockInterview, turns []MockTurn, currentQuestion string, answer string) string {
	return fmt.Sprintf(`Continue a text-only mock interview as a deterministic state machine.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.

Do not write long-term memory.
Do not create coaching plans.
Classify the user input first:
- formal_answer: the candidate is answering the interviewer question.
- hint_request: the candidate asks for a hint; do not score or update practice state.
- explanation_request: the candidate asks for explanation; do not score or update practice state.
- cancel: the candidate wants to stop; do not score or update practice state.

JSON schema:
{
  "input_type": "formal_answer|hint_request|explanation_request|cancel",
  "agent_message": "next interviewer message, hint, explanation, or closing summary",
  "score": 72,
  "feedback": "string",
  "topic": "primary topic",
  "weakness_tags": ["string"],
  "next_action": "ask_followup|switch_topic|complete|wait_for_answer",
  "should_update_practice_state": true,
  "practice_updates": [{"topic":"string","score":72,"feedback":"string"}],
  "should_complete_mock": false,
  "follow_up_reason": "string"
}

For formal_answer, give concise feedback, score 0-100, include weakness_tags/practice_updates, then either ask a follow-up, switch topic, or complete.
For hint_request/explanation_request, keep the same current interviewer question active and set next_action to wait_for_answer.
For cancel, set input_type to cancel and should_complete_mock to false.

Mock interview:
%s

Context:
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
		mockInterviewJSON(mock),
		mockSessionJSON(input.session),
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

func parseMockTurnOutput(raw string) (mockTurnOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed mockTurnOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return mockTurnOutput{}, fmt.Errorf("parse mock turn JSON: %w", err)
	}
	return normalizeMockTurnOutput(parsed), nil
}

func normalizeMockTurnOutput(parsed mockTurnOutput) mockTurnOutput {
	if parsed.InputType == "" {
		parsed.InputType = mockInputTypeFormalAnswer
	}
	if parsed.NextAction == "" {
		parsed.NextAction = mockNextActionAskFollowup
	}
	if parsed.ShouldCompleteMock {
		parsed.NextAction = mockNextActionComplete
	}
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
	if parsed.NextQuestion == "" && (parsed.NextAction == mockNextActionAskFollowup || parsed.NextAction == mockNextActionSwitchTopic) {
		parsed.NextQuestion = parsed.AgentMessage
	}
	if parsed.Topic == "" && len(parsed.TopicTags) > 0 {
		parsed.Topic = parsed.TopicTags[0]
	}
	if parsed.InputType == mockInputTypeFormalAnswer && len(parsed.TopicTags) > 0 && len(parsed.PracticeUpdates) == 0 {
		parsed.ShouldUpdatePracticeState = true
	}
	return parsed
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
		"mock_id":        mock.MockID,
		"target_round":   mock.TargetRound,
		"status":         mock.Status,
		"overall_goal":   mock.OverallGoal,
		"first_question": mock.FirstQuestion,
		"current_turn":   mock.CurrentTurn,
		"current_topic":  mock.CurrentTopic,
		"last_feedback":  mock.LastFeedback,
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
		MockID:         mock.MockID,
		UserID:         mock.UserID,
		InterviewID:    mock.InterviewID,
		PlanID:         mock.PlanID,
		TargetRound:    mock.TargetRound,
		Status:         mock.Status,
		CurrentTurn:    mock.CurrentTurn,
		CurrentTopic:   mock.CurrentTopic,
		OverallGoal:    mock.OverallGoal,
		FirstQuestion:  mock.FirstQuestion,
		LastFeedback:   mock.LastFeedback,
		ErrorMessage:   mock.ErrorMessage,
		FinalSummary:   mock.FinalSummary,
		RawAgentOutput: mock.RawAgentOutput,
		CreatedAt:      mock.CreatedAt,
		UpdatedAt:      mock.UpdatedAt,
	}
}

func toMockTurnVO(turn MockTurn) vo.MockTurnVO {
	return vo.MockTurnVO{
		TurnID:              turn.TurnID,
		MockID:              turn.MockID,
		UserID:              turn.UserID,
		InterviewID:         turn.InterviewID,
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
		TopicTags:           unmarshalStringSlice(turn.TopicTags),
		NextQuestion:        turn.NextQuestion,
		RawAgentOutput:      turn.RawAgentOutput,
		ErrorMessage:        turn.ErrorMessage,
		CreatedAt:           turn.CreatedAt,
		UpdatedAt:           turn.UpdatedAt,
	}
}
