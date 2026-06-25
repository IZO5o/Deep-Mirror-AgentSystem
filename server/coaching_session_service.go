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
	"agent-web-base/vo"
)

const (
	CoachingSessionStatusCreated           = "created"
	CoachingSessionStatusInProgress        = "in_progress"
	CoachingSessionStatusWaitingUserAnswer = "waiting_user_answer"
	CoachingSessionStatusEvaluating        = "evaluating"
	CoachingSessionStatusNeedsRevision     = "needs_revision"
	CoachingSessionStatusTaskCompleted     = "task_completed"
	CoachingSessionStatusPaused            = "paused"
	CoachingSessionStatusCompleted         = "completed"
	CoachingSessionStatusFailed            = "failed"
	CoachingSessionStatusCancelled         = "cancelled"

	CoachingTurnRoleUser      = "user"
	CoachingTurnRoleAssistant = "assistant"
	CoachingTurnRoleSystem    = "system"

	CoachingTurnTypeStart              = "start"
	CoachingTurnTypePrompt             = "prompt"
	CoachingTurnTypeUserAnswer         = "user_answer"
	CoachingTurnTypeHintRequest        = "hint_request"
	CoachingTurnTypeExplanationRequest = "explanation_request"
	CoachingTurnTypeFeedback           = "feedback"
	CoachingTurnTypeStateTransition    = "state_transition"
	CoachingTurnTypeError              = "error"

	CoachingInputTypeFormalAnswer       = "formal_answer"
	CoachingInputTypeHintRequest        = "hint_request"
	CoachingInputTypeExplanationRequest = "explanation_request"
	CoachingInputTypeSkipTask           = "skip_task"
	CoachingInputTypePause              = "pause"

	CoachingNextActionAskRetry     = "ask_retry"
	CoachingNextActionPromptNext   = "prompt_next_task"
	CoachingNextActionContinue     = "continue_current_task"
	CoachingNextActionCompletePlan = "complete_plan"
	CoachingNextActionPause        = "pause"
)

type coachingSessionAgentOutput struct {
	InputType                 string `json:"input_type"`
	AgentMessage              string `json:"agent_message"`
	Score                     int    `json:"score"`
	Passed                    bool   `json:"passed"`
	Feedback                  string `json:"feedback"`
	NextAction                string `json:"next_action"`
	ShouldCompleteCurrentTask bool   `json:"should_complete_current_task"`
	ShouldPause               bool   `json:"should_pause"`
}

func (s *Server) StartOrResumeCoachingSession(planID string, userID string) (vo.CoachingSessionDetailVO, error) {
	var plan CoachingPlan
	if err := s.db.First(&plan, "plan_id = ?", planID).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	if plan.Status != CoachingPlanStatusGenerated {
		return vo.CoachingSessionDetailVO{}, fmt.Errorf("coaching plan status must be %q", CoachingPlanStatusGenerated)
	}
	if strings.TrimSpace(userID) == "" {
		userID = plan.UserID
	}
	if plan.UserID != userID {
		return vo.CoachingSessionDetailVO{}, fmt.Errorf("coaching plan user_id mismatch")
	}

	var existing CoachingSession
	err := s.db.Where("coaching_plan_id = ? AND user_id = ? AND status IN ?", planID, userID, activeCoachingSessionStatuses()).
		Order("updated_at desc, created_at desc").
		First(&existing).Error
	switch {
	case err == nil:
		if existing.Status == CoachingSessionStatusPaused {
			if err := s.resumePausedCoachingSession(existing, plan); err != nil {
				return vo.CoachingSessionDetailVO{}, err
			}
		}
		return s.GetCoachingSession(existing.SessionID)
	case errors.Is(err, gorm.ErrRecordNotFound):
	default:
		return vo.CoachingSessionDetailVO{}, err
	}

	now := time.Now().Unix()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		task, hasTask, err := firstRunnableCoachingTask(tx, planID)
		if err != nil {
			return err
		}

		session := CoachingSession{
			SessionID:       uuid.New().String(),
			UserID:          userID,
			InterviewID:     plan.InterviewID,
			CoachingPlanID:  plan.PlanID,
			Status:          CoachingSessionStatusCompleted,
			ProgressSummary: "coaching plan has no remaining tasks",
			StartedAt:       now,
			LastActiveAt:    now,
			CompletedAt:     now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if hasTask {
			session.CurrentTaskID = task.TaskID
			session.Status = CoachingSessionStatusWaitingUserAnswer
			session.CompletedAt = 0
			session.ProgressSummary = fmt.Sprintf("current task %d: %s", task.Sequence, task.Title)
			session.LastAgentMessage = buildCoachingTaskPromptMessage(task)
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		if hasTask && task.Status == CoachingTaskStatusTodo {
			if err := tx.Model(&CoachingTask{}).
				Where("task_id = ?", task.TaskID).
				Updates(map[string]any{"status": CoachingTaskStatusInProgress, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if hasTask {
			return tx.Create(&CoachingSessionTurn{
				TurnID:         uuid.New().String(),
				SessionID:      session.SessionID,
				CoachingPlanID: session.CoachingPlanID,
				CoachingTaskID: session.CurrentTaskID,
				Role:           CoachingTurnRoleAssistant,
				TurnType:       CoachingTurnTypeStart,
				Content:        session.LastAgentMessage,
				AgentAction:    "start_current_task",
				CreatedAt:      now,
			}).Error
		}
		return nil
	})
	if err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}

	var created CoachingSession
	if err := s.db.Where("coaching_plan_id = ? AND user_id = ?", planID, userID).
		Order("created_at desc").
		First(&created).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	return s.GetCoachingSession(created.SessionID)
}

func (s *Server) GetCoachingSession(sessionID string) (vo.CoachingSessionDetailVO, error) {
	var session CoachingSession
	if err := s.db.First(&session, "session_id = ?", sessionID).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}

	var tasks []CoachingTask
	if err := s.db.Where("plan_id = ?", session.CoachingPlanID).
		Order("sequence asc").
		Find(&tasks).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	taskVOs := make([]vo.CoachingTaskVO, 0, len(tasks))
	var currentTask *vo.CoachingTaskVO
	for _, task := range tasks {
		taskVO := toCoachingTaskVO(task)
		taskVOs = append(taskVOs, taskVO)
		if task.TaskID == session.CurrentTaskID {
			taskCopy := taskVO
			currentTask = &taskCopy
		}
	}

	var turns []CoachingSessionTurn
	if err := s.db.Where("session_id = ?", session.SessionID).
		Order("created_at asc").
		Find(&turns).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}

	var attempts []CoachingTaskAttempt
	if err := s.db.Where("session_id = ?", session.SessionID).
		Order("created_at asc, attempt_index asc").
		Find(&attempts).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}

	return vo.CoachingSessionDetailVO{
		Session:     toCoachingSessionVO(session),
		CurrentTask: currentTask,
		Tasks:       taskVOs,
		Turns:       toCoachingSessionTurnVOs(turns),
		Attempts:    toCoachingTaskAttemptVOs(attempts),
	}, nil
}

func (s *Server) SubmitCoachingSessionTurn(ctx context.Context, sessionID string, req vo.SubmitCoachingSessionTurnReq) (vo.CoachingSessionDetailVO, error) {
	if s.agents == nil {
		return vo.CoachingSessionDetailVO{}, fmt.Errorf("agent provider is nil")
	}
	userInput := strings.TrimSpace(req.UserInput)
	if userInput == "" {
		return vo.CoachingSessionDetailVO{}, fmt.Errorf("user_input is required")
	}

	var session CoachingSession
	if err := s.db.First(&session, "session_id = ?", sessionID).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	if err := validateCoachingSessionCanSubmit(session); err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}

	currentTask, err := s.ensureCoachingSessionCurrentTask(&session)
	if err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	if currentTask.TaskID == "" {
		return s.GetCoachingSession(session.SessionID)
	}

	now := time.Now().Unix()
	userTurn := CoachingSessionTurn{
		TurnID:         uuid.New().String(),
		SessionID:      session.SessionID,
		CoachingPlanID: session.CoachingPlanID,
		CoachingTaskID: currentTask.TaskID,
		Role:           CoachingTurnRoleUser,
		TurnType:       CoachingTurnTypeUserAnswer,
		Content:        userInput,
		CreatedAt:      now,
	}

	var plan CoachingPlan
	if err := s.db.First(&plan, "plan_id = ?", session.CoachingPlanID).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	tasks, err := s.loadCoachingTasks(session.CoachingPlanID)
	if err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	turns, err := s.loadRecentCoachingTurns(session.SessionID, 8)
	if err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeSecondRoundCoach))
	if err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	prompt := buildCoachingSessionTurnPrompt(plan, session, currentTask, tasks, turns, userInput)
	inputSnapshot := marshalTraceJSON(map[string]any{
		"session_id":        session.SessionID,
		"interview_id":      session.InterviewID,
		"coaching_plan_id":  session.CoachingPlanID,
		"current_task_id":   currentTask.TaskID,
		"session_status":    session.Status,
		"task_status":       currentTask.Status,
		"recent_turn_count": len(turns),
		"task_count":        len(tasks),
		"user_input_length": len(userInput),
		"prompt_length":     len(prompt),
	})
	result, runErr := runner.RunTask(ctx, prompt)
	if runErr != nil {
		if saveErr := s.failCoachingSessionAfterAgentError(session, currentTask, userTurn, result.Response, runErr); saveErr != nil {
			return vo.CoachingSessionDetailVO{}, fmt.Errorf("coaching session agent failed: %v; save failure: %w", runErr, saveErr)
		}
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         session.UserID,
			InterviewID:    session.InterviewID,
			AgentType:      string(agent.AgentTypeSecondRoundCoach),
			SourceType:     AgentTraceSourceCoachingSession,
			SourceID:       session.SessionID,
			StepName:       AgentTraceStepCoachingSessionTurn,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ServiceActions: marshalTraceJSON([]string{"recorded failed coaching_session_turn", "updated coaching_session failed"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(runErr),
		})
		return vo.CoachingSessionDetailVO{}, fmt.Errorf("coaching session agent failed: %w", runErr)
	}

	parsed, parseErr := parseCoachingSessionAgentOutput(result.Response)
	if parseErr != nil {
		if saveErr := s.failCoachingSessionAfterAgentError(session, currentTask, userTurn, result.Response, parseErr); saveErr != nil {
			return vo.CoachingSessionDetailVO{}, fmt.Errorf("parse coaching session output failed: %v; save failure: %w", parseErr, saveErr)
		}
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         session.UserID,
			InterviewID:    session.InterviewID,
			AgentType:      string(agent.AgentTypeSecondRoundCoach),
			SourceType:     AgentTraceSourceCoachingSession,
			SourceID:       session.SessionID,
			StepName:       AgentTraceStepCoachingSessionTurn,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ServiceActions: marshalTraceJSON([]string{"recorded failed coaching_session_turn", "updated coaching_session failed"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(parseErr),
		})
		return vo.CoachingSessionDetailVO{}, parseErr
	}
	if err := s.applyCoachingSessionAgentOutput(session, currentTask, userTurn, result.Response, parsed); err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         session.UserID,
			InterviewID:    session.InterviewID,
			AgentType:      string(agent.AgentTypeSecondRoundCoach),
			SourceType:     AgentTraceSourceCoachingSession,
			SourceID:       session.SessionID,
			StepName:       AgentTraceStepCoachingSessionTurn,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ParsedDecision: marshalTraceJSON(parsed),
			ServiceActions: marshalTraceJSON([]string{"failed to persist coaching_session_turn"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(err),
		})
		return vo.CoachingSessionDetailVO{}, err
	}
	s.recordAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:         session.UserID,
		InterviewID:    session.InterviewID,
		AgentType:      string(agent.AgentTypeSecondRoundCoach),
		SourceType:     AgentTraceSourceCoachingSession,
		SourceID:       session.SessionID,
		StepName:       AgentTraceStepCoachingSessionTurn,
		InputSnapshot:  inputSnapshot,
		RawAgentOutput: result.Response,
		ParsedDecision: marshalTraceJSON(parsed),
		ServiceActions: marshalTraceJSON(coachingSessionTraceActions(parsed)),
		Status:         AgentDecisionTraceStatusSucceeded,
	})
	return s.GetCoachingSession(session.SessionID)
}

func (s *Server) PauseCoachingSession(sessionID string) (vo.CoachingSessionDetailVO, error) {
	var session CoachingSession
	if err := s.db.First(&session, "session_id = ?", sessionID).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	if isTerminalCoachingSessionStatus(session.Status) {
		return vo.CoachingSessionDetailVO{}, fmt.Errorf("cannot pause terminal coaching session status %q", session.Status)
	}
	now := time.Now().Unix()
	if err := s.db.Model(&CoachingSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"status":         CoachingSessionStatusPaused,
			"last_active_at": now,
			"updated_at":     now,
		}).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	return s.GetCoachingSession(sessionID)
}

func (s *Server) CancelCoachingSession(sessionID string) (vo.CoachingSessionDetailVO, error) {
	var session CoachingSession
	if err := s.db.First(&session, "session_id = ?", sessionID).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	if session.Status == CoachingSessionStatusCompleted {
		return vo.CoachingSessionDetailVO{}, fmt.Errorf("cannot cancel completed coaching session")
	}
	now := time.Now().Unix()
	if err := s.db.Model(&CoachingSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"status":         CoachingSessionStatusCancelled,
			"last_active_at": now,
			"updated_at":     now,
		}).Error; err != nil {
		return vo.CoachingSessionDetailVO{}, err
	}
	return s.GetCoachingSession(sessionID)
}

func (s *Server) resumePausedCoachingSession(session CoachingSession, plan CoachingPlan) error {
	now := time.Now().Unix()
	message := session.LastAgentMessage
	if strings.TrimSpace(message) == "" && strings.TrimSpace(session.CurrentTaskID) != "" {
		var task CoachingTask
		if err := s.db.First(&task, "task_id = ?", session.CurrentTaskID).Error; err == nil {
			message = buildCoachingTaskPromptMessage(task)
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&CoachingSession{}).
			Where("session_id = ?", session.SessionID).
			Updates(map[string]any{
				"status":             CoachingSessionStatusWaitingUserAnswer,
				"last_agent_message": message,
				"last_active_at":     now,
				"updated_at":         now,
			}).Error; err != nil {
			return err
		}
		return tx.Create(&CoachingSessionTurn{
			TurnID:         uuid.New().String(),
			SessionID:      session.SessionID,
			CoachingPlanID: plan.PlanID,
			CoachingTaskID: session.CurrentTaskID,
			Role:           CoachingTurnRoleAssistant,
			TurnType:       CoachingTurnTypePrompt,
			Content:        message,
			AgentAction:    "resume_session",
			CreatedAt:      now,
		}).Error
	})
}

func (s *Server) ensureCoachingSessionCurrentTask(session *CoachingSession) (CoachingTask, error) {
	if strings.TrimSpace(session.CurrentTaskID) != "" {
		var task CoachingTask
		if err := s.db.First(&task, "task_id = ?", session.CurrentTaskID).Error; err != nil {
			return CoachingTask{}, err
		}
		if task.Status != CoachingTaskStatusDone && task.Status != CoachingTaskStatusSkipped {
			return task, nil
		}
	}
	task, hasTask, err := firstRunnableCoachingTask(s.db, session.CoachingPlanID)
	if err != nil {
		return CoachingTask{}, err
	}
	if !hasTask {
		now := time.Now().Unix()
		err := s.db.Model(&CoachingSession{}).
			Where("session_id = ?", session.SessionID).
			Updates(map[string]any{
				"status":           CoachingSessionStatusCompleted,
				"current_task_id":  "",
				"progress_summary": "all coaching tasks completed",
				"completed_at":     now,
				"last_active_at":   now,
				"updated_at":       now,
			}).Error
		return CoachingTask{}, err
	}
	now := time.Now().Unix()
	if err := s.db.Model(&CoachingSession{}).
		Where("session_id = ?", session.SessionID).
		Updates(map[string]any{
			"current_task_id":  task.TaskID,
			"status":           CoachingSessionStatusWaitingUserAnswer,
			"progress_summary": fmt.Sprintf("current task %d: %s", task.Sequence, task.Title),
			"updated_at":       now,
		}).Error; err != nil {
		return CoachingTask{}, err
	}
	if task.Status == CoachingTaskStatusTodo {
		if err := s.db.Model(&CoachingTask{}).
			Where("task_id = ?", task.TaskID).
			Updates(map[string]any{"status": CoachingTaskStatusInProgress, "updated_at": now}).Error; err != nil {
			return CoachingTask{}, err
		}
		task.Status = CoachingTaskStatusInProgress
	}
	session.CurrentTaskID = task.TaskID
	return task, nil
}

func (s *Server) applyCoachingSessionAgentOutput(session CoachingSession, task CoachingTask, userTurn CoachingSessionTurn, rawOutput string, parsed coachingSessionAgentOutput) error {
	now := time.Now().Unix()
	inputType := normalizeCoachingInputType(parsed.InputType)
	userTurn.TurnType = inputType
	assistantTurnType := CoachingTurnTypeFeedback
	if inputType == CoachingInputTypeHintRequest {
		assistantTurnType = CoachingTurnTypeHintRequest
	} else if inputType == CoachingInputTypeExplanationRequest {
		assistantTurnType = CoachingTurnTypeExplanationRequest
	} else if inputType == CoachingInputTypePause || parsed.ShouldPause {
		assistantTurnType = CoachingTurnTypeStateTransition
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&userTurn).Error; err != nil {
			return err
		}

		assistantTurn := CoachingSessionTurn{
			TurnID:         uuid.New().String(),
			SessionID:      session.SessionID,
			CoachingPlanID: session.CoachingPlanID,
			CoachingTaskID: task.TaskID,
			Role:           CoachingTurnRoleAssistant,
			TurnType:       assistantTurnType,
			Content:        parsed.AgentMessage,
			AgentAction:    parsed.NextAction,
			Score:          parsed.Score,
			Feedback:       parsed.Feedback,
			RawAgentOutput: rawOutput,
			CreatedAt:      now,
		}
		if err := tx.Create(&assistantTurn).Error; err != nil {
			return err
		}

		nextStatus := CoachingSessionStatusWaitingUserAnswer
		currentTaskID := task.TaskID
		progressSummary := fmt.Sprintf("current task %d: %s", task.Sequence, task.Title)
		completedAt := int64(0)

		if inputType == CoachingInputTypeFormalAnswer {
			attemptIndex, err := nextCoachingAttemptIndex(tx, session.SessionID, task.TaskID)
			if err != nil {
				return err
			}
			attempt := CoachingTaskAttempt{
				AttemptID:      uuid.New().String(),
				SessionID:      session.SessionID,
				CoachingTaskID: task.TaskID,
				UserAnswer:     userTurn.Content,
				Score:          parsed.Score,
				Feedback:       parsed.Feedback,
				Passed:         parsed.Passed,
				AttemptIndex:   attemptIndex,
				RawAgentOutput: rawOutput,
				CreatedAt:      now,
			}
			if err := tx.Create(&attempt).Error; err != nil {
				return err
			}
			if err := s.runPracticeStateUpdateToolTx(tx, practiceStateUpdateToolInput{
				UserID:     session.UserID,
				Topics:     coachingTaskPracticeTopics(task),
				Score:      parsed.Score,
				Feedback:   parsed.Feedback,
				SourceType: PracticeStateSourceCoachingTaskAttempt,
				SourceID:   attempt.AttemptID,
			}); err != nil {
				return err
			}
			if parsed.Passed && parsed.ShouldCompleteCurrentTask {
				if err := tx.Model(&CoachingTask{}).
					Where("task_id = ?", task.TaskID).
					Updates(map[string]any{"status": CoachingTaskStatusDone, "updated_at": now}).Error; err != nil {
					return err
				}
				nextTask, hasNext, err := firstRunnableCoachingTask(tx, session.CoachingPlanID)
				if err != nil {
					return err
				}
				if hasNext {
					currentTaskID = nextTask.TaskID
					nextStatus = CoachingSessionStatusWaitingUserAnswer
					progressSummary = fmt.Sprintf("completed task %d; next task %d: %s", task.Sequence, nextTask.Sequence, nextTask.Title)
					if nextTask.Status == CoachingTaskStatusTodo {
						if err := tx.Model(&CoachingTask{}).
							Where("task_id = ?", nextTask.TaskID).
							Updates(map[string]any{"status": CoachingTaskStatusInProgress, "updated_at": now}).Error; err != nil {
							return err
						}
					}
				} else {
					currentTaskID = ""
					nextStatus = CoachingSessionStatusCompleted
					progressSummary = "all coaching tasks completed"
					completedAt = now
				}
			} else {
				nextStatus = CoachingSessionStatusNeedsRevision
				if err := tx.Model(&CoachingTask{}).
					Where("task_id = ?", task.TaskID).
					Updates(map[string]any{"status": CoachingTaskStatusNeedsRevision, "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}

		if inputType == CoachingInputTypeSkipTask {
			if err := tx.Model(&CoachingTask{}).
				Where("task_id = ?", task.TaskID).
				Updates(map[string]any{"status": CoachingTaskStatusSkipped, "updated_at": now}).Error; err != nil {
				return err
			}
			nextTask, hasNext, err := firstRunnableCoachingTask(tx, session.CoachingPlanID)
			if err != nil {
				return err
			}
			if hasNext {
				currentTaskID = nextTask.TaskID
				progressSummary = fmt.Sprintf("skipped task %d; next task %d: %s", task.Sequence, nextTask.Sequence, nextTask.Title)
				if nextTask.Status == CoachingTaskStatusTodo {
					if err := tx.Model(&CoachingTask{}).
						Where("task_id = ?", nextTask.TaskID).
						Updates(map[string]any{"status": CoachingTaskStatusInProgress, "updated_at": now}).Error; err != nil {
						return err
					}
				}
			} else {
				currentTaskID = ""
				nextStatus = CoachingSessionStatusCompleted
				progressSummary = "all coaching tasks completed"
				completedAt = now
			}
		}

		if inputType == CoachingInputTypePause || parsed.ShouldPause {
			nextStatus = CoachingSessionStatusPaused
		}

		updates := map[string]any{
			"current_task_id":    currentTaskID,
			"status":             nextStatus,
			"progress_summary":   progressSummary,
			"last_agent_message": parsed.AgentMessage,
			"error_message":      "",
			"last_active_at":     now,
			"updated_at":         now,
		}
		if completedAt > 0 {
			updates["completed_at"] = completedAt
		}
		return tx.Model(&CoachingSession{}).
			Where("session_id = ?", session.SessionID).
			Updates(updates).Error
	})
}

func (s *Server) failCoachingSessionAfterAgentError(session CoachingSession, task CoachingTask, userTurn CoachingSessionTurn, rawOutput string, cause error) error {
	now := time.Now().Unix()
	errorMessage := ""
	if cause != nil {
		errorMessage = cause.Error()
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		userTurn.TurnType = CoachingTurnTypeUserAnswer
		if err := tx.Create(&userTurn).Error; err != nil {
			return err
		}
		if err := tx.Create(&CoachingSessionTurn{
			TurnID:         uuid.New().String(),
			SessionID:      session.SessionID,
			CoachingPlanID: session.CoachingPlanID,
			CoachingTaskID: task.TaskID,
			Role:           CoachingTurnRoleSystem,
			TurnType:       CoachingTurnTypeError,
			Content:        errorMessage,
			RawAgentOutput: rawOutput,
			ErrorMessage:   errorMessage,
			CreatedAt:      now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&CoachingSession{}).
			Where("session_id = ?", session.SessionID).
			Updates(map[string]any{
				"status":         CoachingSessionStatusFailed,
				"error_message":  errorMessage,
				"last_active_at": now,
				"updated_at":     now,
			}).Error
	})
}

func buildCoachingSessionTurnPrompt(plan CoachingPlan, session CoachingSession, currentTask CoachingTask, tasks []CoachingTask, turns []CoachingSessionTurn, userInput string) string {
	taskPayload := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		taskPayload = append(taskPayload, map[string]any{
			"task_id":     task.TaskID,
			"sequence":    task.Sequence,
			"task_type":   task.TaskType,
			"title":       task.Title,
			"description": task.Description,
			"priority":    task.Priority,
			"status":      task.Status,
		})
	}
	turnPayload := make([]map[string]any, 0, len(turns))
	for _, turn := range turns {
		turnPayload = append(turnPayload, map[string]any{
			"role":      turn.Role,
			"turn_type": turn.TurnType,
			"content":   turn.Content,
			"score":     turn.Score,
			"feedback":  turn.Feedback,
		})
	}
	tasksJSON, _ := json.Marshal(taskPayload)
	turnsJSON, _ := json.Marshal(turnPayload)

	return fmt.Sprintf(`You are the fixed second_round_coach agent running one step of a plan-level coaching session.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.
Do not write memory_items. Do not call tools. Do not change state directly; the server will persist state.

Classify the user input as one of:
- formal_answer
- hint_request
- explanation_request
- skip_task
- pause

JSON schema:
{
  "input_type": "formal_answer|hint_request|explanation_request|skip_task|pause",
  "agent_message": "string",
  "score": 0,
  "passed": false,
  "feedback": "string",
  "next_action": "ask_retry|prompt_next_task|continue_current_task|complete_plan|pause",
  "should_complete_current_task": false,
  "should_pause": false
}

Rules:
- For hint_request or explanation_request, do not mark the task complete.
- For formal_answer, score from 0 to 100 and provide concrete feedback.
- Set passed=true only when the answer satisfies the current task.
- Set should_complete_current_task=true only when this task should be marked done.
- Keep agent_message user-facing and concise.

Coaching plan:
- plan_id: %s
- interview_id: %s
- target_round: %s
- company_name: %s
- job_title: %s

Session:
- session_id: %s
- status: %s
- current_task_id: %s
- progress_summary: %s

Current task:
- task_id: %s
- sequence: %d
- type: %s
- title: %s
- description: %s
- priority: %s

All tasks JSON:
%s

Recent turns JSON:
%s

User input:
%s`,
		plan.PlanID,
		plan.InterviewID,
		plan.TargetRound,
		plan.CompanyName,
		plan.JobTitle,
		session.SessionID,
		session.Status,
		currentTask.TaskID,
		session.ProgressSummary,
		currentTask.TaskID,
		currentTask.Sequence,
		currentTask.TaskType,
		currentTask.Title,
		currentTask.Description,
		currentTask.Priority,
		string(tasksJSON),
		string(turnsJSON),
		userInput,
	)
}

func parseCoachingSessionAgentOutput(raw string) (coachingSessionAgentOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed coachingSessionAgentOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return coachingSessionAgentOutput{}, fmt.Errorf("parse coaching session JSON: %w", err)
	}
	parsed.InputType = normalizeCoachingInputType(parsed.InputType)
	parsed.NextAction = normalizeDefault(parsed.NextAction, CoachingNextActionContinue)
	return parsed, nil
}

func coachingSessionTraceActions(parsed coachingSessionAgentOutput) []string {
	actions := []string{
		"recorded coaching_session user turn",
		"recorded coaching_session assistant turn",
		"updated coaching_session state",
	}
	if parsed.InputType == CoachingInputTypeFormalAnswer {
		actions = append(actions, "recorded coaching_task_attempt", "updated practice_states")
		if parsed.Passed && parsed.ShouldCompleteCurrentTask {
			actions = append(actions, "marked coaching_task done")
		}
	}
	if parsed.InputType == CoachingInputTypeSkipTask {
		actions = append(actions, "marked coaching_task skipped")
	}
	if parsed.ShouldPause || parsed.InputType == CoachingInputTypePause {
		actions = append(actions, "paused coaching_session")
	}
	return actions
}

func firstRunnableCoachingTask(db *gorm.DB, planID string) (CoachingTask, bool, error) {
	var task CoachingTask
	err := db.Where("plan_id = ? AND status IN ?", planID, []string{
		CoachingTaskStatusInProgress,
		CoachingTaskStatusNeedsRevision,
		CoachingTaskStatusTodo,
	}).
		Order("case status when 'in_progress' then 0 when 'needs_revision' then 1 else 2 end, sequence asc").
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CoachingTask{}, false, nil
	}
	if err != nil {
		return CoachingTask{}, false, err
	}
	return task, true, nil
}

func nextCoachingAttemptIndex(tx *gorm.DB, sessionID string, taskID string) (int, error) {
	var count int64
	if err := tx.Model(&CoachingTaskAttempt{}).
		Where("session_id = ? AND coaching_task_id = ?", sessionID, taskID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

func (s *Server) loadCoachingTasks(planID string) ([]CoachingTask, error) {
	var tasks []CoachingTask
	if err := s.db.Where("plan_id = ?", planID).Order("sequence asc").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Server) loadRecentCoachingTurns(sessionID string, limit int) ([]CoachingSessionTurn, error) {
	if limit <= 0 {
		limit = 8
	}
	var turns []CoachingSessionTurn
	if err := s.db.Where("session_id = ?", sessionID).
		Order("created_at desc").
		Limit(limit).
		Find(&turns).Error; err != nil {
		return nil, err
	}
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
	return turns, nil
}

func buildCoachingTaskPromptMessage(task CoachingTask) string {
	return fmt.Sprintf("我们先练习：%s。%s 请直接给出一版正式回答；如果你需要，也可以先要求提示或解释。", task.Title, task.Description)
}

func coachingTaskPracticeTopics(task CoachingTask) []string {
	if topic := strings.TrimSpace(task.Title); topic != "" {
		return []string{topic}
	}
	if topic := strings.TrimSpace(task.TaskType); topic != "" {
		return []string{topic}
	}
	description := strings.TrimSpace(task.Description)
	if description == "" {
		return nil
	}
	runes := []rune(description)
	if len(runes) > 40 {
		description = string(runes[:40])
	}
	return []string{description}
}

func activeCoachingSessionStatuses() []string {
	return []string{
		CoachingSessionStatusCreated,
		CoachingSessionStatusInProgress,
		CoachingSessionStatusWaitingUserAnswer,
		CoachingSessionStatusEvaluating,
		CoachingSessionStatusNeedsRevision,
		CoachingSessionStatusTaskCompleted,
		CoachingSessionStatusPaused,
	}
}

func validateCoachingSessionCanSubmit(session CoachingSession) error {
	if isTerminalCoachingSessionStatus(session.Status) {
		return fmt.Errorf("cannot submit turn when coaching session status is %q", session.Status)
	}
	if session.Status == CoachingSessionStatusPaused {
		return fmt.Errorf("cannot submit turn when coaching session is paused; resume it first")
	}
	return nil
}

func isTerminalCoachingSessionStatus(status string) bool {
	switch status {
	case CoachingSessionStatusCompleted, CoachingSessionStatusFailed, CoachingSessionStatusCancelled:
		return true
	default:
		return false
	}
}

func normalizeCoachingInputType(inputType string) string {
	switch strings.TrimSpace(inputType) {
	case CoachingInputTypeFormalAnswer,
		CoachingInputTypeHintRequest,
		CoachingInputTypeExplanationRequest,
		CoachingInputTypeSkipTask,
		CoachingInputTypePause:
		return strings.TrimSpace(inputType)
	default:
		return CoachingInputTypeFormalAnswer
	}
}

func toCoachingSessionVO(session CoachingSession) vo.CoachingSessionVO {
	return vo.CoachingSessionVO{
		SessionID:        session.SessionID,
		UserID:           session.UserID,
		InterviewID:      session.InterviewID,
		CoachingPlanID:   session.CoachingPlanID,
		CurrentTaskID:    session.CurrentTaskID,
		Status:           session.Status,
		ProgressSummary:  session.ProgressSummary,
		LastAgentMessage: session.LastAgentMessage,
		ErrorMessage:     session.ErrorMessage,
		StartedAt:        session.StartedAt,
		LastActiveAt:     session.LastActiveAt,
		CompletedAt:      session.CompletedAt,
		CreatedAt:        session.CreatedAt,
		UpdatedAt:        session.UpdatedAt,
	}
}

func toCoachingSessionTurnVOs(turns []CoachingSessionTurn) []vo.CoachingSessionTurnVO {
	result := make([]vo.CoachingSessionTurnVO, 0, len(turns))
	for _, turn := range turns {
		result = append(result, vo.CoachingSessionTurnVO{
			TurnID:         turn.TurnID,
			SessionID:      turn.SessionID,
			CoachingPlanID: turn.CoachingPlanID,
			CoachingTaskID: turn.CoachingTaskID,
			Role:           turn.Role,
			TurnType:       turn.TurnType,
			Content:        turn.Content,
			AgentAction:    turn.AgentAction,
			Score:          turn.Score,
			Feedback:       turn.Feedback,
			RawAgentOutput: turn.RawAgentOutput,
			ErrorMessage:   turn.ErrorMessage,
			CreatedAt:      turn.CreatedAt,
		})
	}
	return result
}

func toCoachingTaskAttemptVOs(attempts []CoachingTaskAttempt) []vo.CoachingTaskAttemptVO {
	result := make([]vo.CoachingTaskAttemptVO, 0, len(attempts))
	for _, attempt := range attempts {
		result = append(result, vo.CoachingTaskAttemptVO{
			AttemptID:      attempt.AttemptID,
			SessionID:      attempt.SessionID,
			CoachingTaskID: attempt.CoachingTaskID,
			UserAnswer:     attempt.UserAnswer,
			Score:          attempt.Score,
			Feedback:       attempt.Feedback,
			Passed:         attempt.Passed,
			AttemptIndex:   attempt.AttemptIndex,
			RawAgentOutput: attempt.RawAgentOutput,
			ErrorMessage:   attempt.ErrorMessage,
			CreatedAt:      attempt.CreatedAt,
		})
	}
	return result
}
