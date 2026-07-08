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
	CoachingPlanStatusGenerated = "generated"
	CoachingPlanStatusFailed    = "failed"

	CoachingPlanSourceInterview    = "interview"
	CoachingPlanSourcePracticeGoal = "practice_goal"

	CoachingTaskStatusTodo          = "todo"
	CoachingTaskStatusDone          = "done"
	CoachingTaskStatusInProgress    = "in_progress"
	CoachingTaskStatusNeedsRevision = "needs_revision"
	CoachingTaskStatusSkipped       = "skipped"

	CoachingTaskPriorityHigh   = "high"
	CoachingTaskPriorityMedium = "medium"
	CoachingTaskPriorityLow    = "low"
)

type coachingPlanOutput struct {
	OverallStrategy string               `json:"overall_strategy"`
	FocusSummary    string               `json:"focus_summary"`
	Tasks           []coachingTaskOutput `json:"tasks"`
}

type coachingTaskOutput struct {
	Sequence         int      `json:"sequence"`
	DayIndex         int      `json:"day_index"`
	TaskType         string   `json:"task_type"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	RelatedMemoryIDs []string `json:"related_memory_ids"`
	Priority         string   `json:"priority"`
}

func (s *Server) GenerateCoachingPlan(ctx context.Context, interviewID string, req vo.GenerateCoachingPlanReq) (vo.CoachingPlanVO, error) {
	if s.agents == nil {
		return vo.CoachingPlanVO{}, fmt.Errorf("agent provider is nil")
	}
	if req.TargetRound == "" {
		req.TargetRound = "second_round"
	}
	if req.RemainingDays <= 0 {
		req.RemainingDays = 1
	}

	input, err := s.loadCoachingInput(interviewID, req)
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}
	if input.session.Status != InterviewStatusReviewed {
		return vo.CoachingPlanVO{}, fmt.Errorf("interview status must be %q before generating coaching plan", InterviewStatusReviewed)
	}
	if input.session.UserID != req.UserID {
		return vo.CoachingPlanVO{}, fmt.Errorf("interview user_id mismatch")
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeSecondRoundCoach))
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}

	prompt := buildCoachingPlanPrompt(input, req)
	inputSnapshot := marshalTraceJSON(map[string]any{
		"interview_id":     input.session.InterviewID,
		"user_id":          input.session.UserID,
		"company_name":     input.session.CompanyName,
		"job_title":        input.session.JobTitle,
		"target_round":     req.TargetRound,
		"remaining_days":   req.RemainingDays,
		"question_count":   len(input.questions),
		"prompt_length":    len(prompt),
		"review_status":    input.report.Status,
		"interview_status": input.session.Status,
	})
	selectedContextSnapshot := buildSelectedContextTraceSnapshot(input.selection)

	result, err := runner.RunTask(ctx, prompt)
	if err != nil {
		log.Warnf("second round coach agent failed for interview %s: %v", interviewID, err)
		report, saveErr := s.upsertFailedCoachingPlan(input.session, req, result.Response, err)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  input.session.UserID,
			InterviewID:             input.session.InterviewID,
			AgentType:               string(agent.AgentTypeSecondRoundCoach),
			SourceType:              AgentTraceSourceCoachingPlan,
			SourceID:                report.PlanID,
			StepName:                AgentTraceStepCoachingPlanGenerate,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ServiceActions:          marshalTraceJSON([]string{"upserted failed coaching_plan"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		if saveErr != nil {
			return vo.CoachingPlanVO{}, fmt.Errorf("second round coach failed: %v; save failed plan: %w", err, saveErr)
		}
		return report, fmt.Errorf("second round coach failed: %w", err)
	}

	parsed, err := parseCoachingPlanOutput(result.Response)
	if err != nil {
		log.Warnf("parse coaching plan output failed for interview %s: %v, raw=%s", interviewID, err, result.Response)
		report, saveErr := s.upsertFailedCoachingPlan(input.session, req, result.Response, err)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  input.session.UserID,
			InterviewID:             input.session.InterviewID,
			AgentType:               string(agent.AgentTypeSecondRoundCoach),
			SourceType:              AgentTraceSourceCoachingPlan,
			SourceID:                report.PlanID,
			StepName:                AgentTraceStepCoachingPlanGenerate,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ServiceActions:          marshalTraceJSON([]string{"upserted failed coaching_plan"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		if saveErr != nil {
			return vo.CoachingPlanVO{}, fmt.Errorf("parse coaching plan failed: %v; save failed plan: %w", err, saveErr)
		}
		return report, err
	}

	plan, err := s.saveSuccessfulCoachingPlan(input.session, req, result.Response, parsed)
	if err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  input.session.UserID,
			InterviewID:             input.session.InterviewID,
			AgentType:               string(agent.AgentTypeSecondRoundCoach),
			SourceType:              AgentTraceSourceCoachingPlan,
			SourceID:                input.session.InterviewID,
			StepName:                AgentTraceStepCoachingPlanGenerate,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ParsedDecision:          marshalTraceJSON(parsed),
			ServiceActions:          marshalTraceJSON([]string{"failed to persist coaching_plan"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		return vo.CoachingPlanVO{}, err
	}
	s.recordAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:                  input.session.UserID,
		InterviewID:             input.session.InterviewID,
		AgentType:               string(agent.AgentTypeSecondRoundCoach),
		SourceType:              AgentTraceSourceCoachingPlan,
		SourceID:                plan.PlanID,
		StepName:                AgentTraceStepCoachingPlanGenerate,
		SelectedContextSnapshot: selectedContextSnapshot,
		InputSnapshot:           inputSnapshot,
		RawAgentOutput:          result.Response,
		ParsedDecision:          marshalTraceJSON(parsed),
		ServiceActions: marshalTraceJSON([]string{
			"created coaching_plan",
			fmt.Sprintf("created coaching_tasks: %d", len(parsed.Tasks)),
		}),
		Status: AgentDecisionTraceStatusSucceeded,
	})
	return plan, nil
}

func (s *Server) GeneratePracticeGoalCoachingPlan(ctx context.Context, goalID string, req vo.GeneratePracticeGoalCoachingPlanReq) (vo.CoachingPlanVO, error) {
	if s.agents == nil {
		return vo.CoachingPlanVO{}, fmt.Errorf("agent provider is nil")
	}
	req.UserID = strings.TrimSpace(req.UserID)
	input, err := s.loadPracticeGoalCoachingInput(goalID, req)
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}
	goal := *input.practiceGoal
	if strings.TrimSpace(req.TargetRound) == "" {
		req.TargetRound = goal.TargetRound
	}
	if req.RemainingDays <= 0 {
		req.RemainingDays = goal.RemainingDays
	}
	if req.RemainingDays <= 0 {
		req.RemainingDays = 1
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeSecondRoundCoach))
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}

	planReq := vo.GenerateCoachingPlanReq{UserID: req.UserID, TargetRound: req.TargetRound, RemainingDays: req.RemainingDays}
	prompt := buildCoachingPlanPrompt(input, planReq)
	inputSnapshot := marshalTraceJSON(map[string]any{
		"practice_goal_id": goal.GoalID,
		"user_id":          goal.UserID,
		"company_name":     goal.CompanyName,
		"job_title":        goal.JobTitle,
		"target_round":     req.TargetRound,
		"remaining_days":   req.RemainingDays,
		"prompt_length":    len(prompt),
		"source_type":      CoachingPlanSourcePracticeGoal,
	})
	selectedContextSnapshot := buildSelectedContextTraceSnapshot(input.selection)

	result, err := runner.RunTask(ctx, prompt)
	if err != nil {
		plan, saveErr := s.upsertFailedPracticeGoalCoachingPlan(goal, req, result.Response, err)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  goal.UserID,
			AgentType:               string(agent.AgentTypeSecondRoundCoach),
			SourceType:              AgentTraceSourceCoachingPlan,
			SourceID:                plan.PlanID,
			StepName:                AgentTraceStepCoachingPlanGenerate,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ServiceActions:          marshalTraceJSON([]string{"upserted failed practice_goal coaching_plan"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		if saveErr != nil {
			return vo.CoachingPlanVO{}, fmt.Errorf("second round coach failed: %v; save failed practice goal plan: %w", err, saveErr)
		}
		return plan, fmt.Errorf("second round coach failed: %w", err)
	}

	parsed, err := parseCoachingPlanOutput(result.Response)
	if err != nil {
		plan, saveErr := s.upsertFailedPracticeGoalCoachingPlan(goal, req, result.Response, err)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:                  goal.UserID,
			AgentType:               string(agent.AgentTypeSecondRoundCoach),
			SourceType:              AgentTraceSourceCoachingPlan,
			SourceID:                plan.PlanID,
			StepName:                AgentTraceStepCoachingPlanGenerate,
			SelectedContextSnapshot: selectedContextSnapshot,
			InputSnapshot:           inputSnapshot,
			RawAgentOutput:          result.Response,
			ServiceActions:          marshalTraceJSON([]string{"upserted failed practice_goal coaching_plan"}),
			Status:                  AgentDecisionTraceStatusFailed,
			ErrorMessage:            traceErrorMessage(err),
		})
		if saveErr != nil {
			return vo.CoachingPlanVO{}, fmt.Errorf("parse coaching plan failed: %v; save failed practice goal plan: %w", err, saveErr)
		}
		return plan, err
	}

	plan, err := s.saveSuccessfulPracticeGoalCoachingPlan(goal, req, result.Response, parsed)
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}
	s.recordAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:                  goal.UserID,
		AgentType:               string(agent.AgentTypeSecondRoundCoach),
		SourceType:              AgentTraceSourceCoachingPlan,
		SourceID:                plan.PlanID,
		StepName:                AgentTraceStepCoachingPlanGenerate,
		SelectedContextSnapshot: selectedContextSnapshot,
		InputSnapshot:           inputSnapshot,
		RawAgentOutput:          result.Response,
		ParsedDecision:          marshalTraceJSON(parsed),
		ServiceActions: marshalTraceJSON([]string{
			"created practice_goal coaching_plan",
			fmt.Sprintf("created coaching_tasks: %d", len(parsed.Tasks)),
		}),
		Status: AgentDecisionTraceStatusSucceeded,
	})
	return plan, nil
}

func (s *Server) GetCoachingPlan(interviewID string) (vo.CoachingPlanVO, error) {
	var plan CoachingPlan
	if err := s.db.First(&plan, "interview_id = ?", interviewID).Error; err != nil {
		return vo.CoachingPlanVO{}, err
	}
	return toCoachingPlanVO(plan), nil
}

func (s *Server) ListCoachingTasks(planID string) ([]vo.CoachingTaskVO, error) {
	var tasks []CoachingTask
	if err := s.db.Where("plan_id = ?", planID).
		Order("sequence asc").
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	result := make([]vo.CoachingTaskVO, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, toCoachingTaskVO(task))
	}
	return result, nil
}

func (s *Server) UpdateCoachingTask(taskID string, req vo.UpdateCoachingTaskReq) (vo.CoachingTaskVO, error) {
	if !validCoachingTaskStatus(req.Status) {
		return vo.CoachingTaskVO{}, fmt.Errorf("unsupported coaching task status %q", req.Status)
	}

	var task CoachingTask
	if err := s.db.First(&task, "task_id = ?", taskID).Error; err != nil {
		return vo.CoachingTaskVO{}, err
	}
	task.Status = req.Status
	task.UpdatedAt = time.Now().Unix()
	if err := s.db.Save(&task).Error; err != nil {
		return vo.CoachingTaskVO{}, err
	}
	return toCoachingTaskVO(task), nil
}

type coachingInput struct {
	sourceType   string
	session      InterviewSession
	practiceGoal *PracticeGoal
	report       *InterviewReviewReport
	questions    []InterviewQuestion
	selection    MemorySelectionResult
}

func (s *Server) loadCoachingInput(interviewID string, req vo.GenerateCoachingPlanReq) (coachingInput, error) {
	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return coachingInput{}, err
	}

	var report InterviewReviewReport
	if err := s.db.First(&report, "interview_id = ?", interviewID).Error; err != nil {
		return coachingInput{}, err
	}
	if report.Status != InterviewReviewStatusGenerated {
		return coachingInput{}, fmt.Errorf("review report must be %q before generating coaching plan", InterviewReviewStatusGenerated)
	}

	var questions []InterviewQuestion
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("sequence asc").
		Find(&questions).Error; err != nil {
		return coachingInput{}, err
	}

	selection, err := s.SelectMemoriesForCoaching(MemorySelectionRequest{
		UserID:              req.UserID,
		CompanyName:         session.CompanyName,
		JobTitle:            session.JobTitle,
		TargetRound:         req.TargetRound,
		CurrentTask:         MemorySelectorTaskCoachingPlan,
		LimitMemoryItems:    defaultMemorySelectionLimit,
		LimitPracticeStates: defaultPracticeStateSelectionLimit,
	})
	if err != nil {
		return coachingInput{}, err
	}

	return coachingInput{
		sourceType: CoachingPlanSourceInterview,
		session:    session,
		report:     &report,
		questions:  questions,
		selection:  selection,
	}, nil
}

func (s *Server) loadPracticeGoalCoachingInput(goalID string, req vo.GeneratePracticeGoalCoachingPlanReq) (coachingInput, error) {
	goal, err := firstPracticeGoalByID(s.db, goalID)
	if err != nil {
		return coachingInput{}, err
	}
	if goal.UserID != strings.TrimSpace(req.UserID) {
		return coachingInput{}, fmt.Errorf("practice goal user_id mismatch")
	}
	if goal.Status != PracticeGoalStatusActive {
		return coachingInput{}, fmt.Errorf("practice goal status must be %q", PracticeGoalStatusActive)
	}
	targetRound := normalizeDefault(strings.TrimSpace(req.TargetRound), goal.TargetRound)
	selection, err := s.SelectMemoriesForCoaching(MemorySelectionRequest{
		UserID:              goal.UserID,
		CompanyName:         goal.CompanyName,
		JobTitle:            goal.JobTitle,
		TargetRound:         targetRound,
		CurrentTask:         strings.Join(unmarshalStringSlice(goal.FocusTopics), " "),
		LimitMemoryItems:    defaultMemorySelectionLimit,
		LimitPracticeStates: defaultPracticeStateSelectionLimit,
	})
	if err != nil {
		return coachingInput{}, err
	}
	return coachingInput{
		sourceType:   CoachingPlanSourcePracticeGoal,
		practiceGoal: &goal,
		selection:    selection,
	}, nil
}

func coachingMemoryTypes() []string {
	return []string{
		MemoryTypeUserWeakness,
		MemoryTypeUserStrength,
		MemoryTypeCompanyProfile,
		MemoryTypeJobProfile,
		MemoryTypeInterviewerFocus,
		MemoryTypeQuestionPattern,
		MemoryTypePreparationTip,
	}
}

func (s *Server) saveSuccessfulCoachingPlan(session InterviewSession, req vo.GenerateCoachingPlanReq, rawOutput string, parsed coachingPlanOutput) (vo.CoachingPlanVO, error) {
	now := time.Now().Unix()
	var savedPlan CoachingPlan
	err := s.db.Transaction(func(tx *gorm.DB) error {
		plan, err := upsertCoachingPlan(tx, session, req, now, func(plan *CoachingPlan) {
			plan.OverallStrategy = parsed.OverallStrategy
			plan.FocusSummary = parsed.FocusSummary
			plan.RawAgentOutput = rawOutput
			plan.Status = CoachingPlanStatusGenerated
		})
		if err != nil {
			return err
		}
		savedPlan = plan

		if err := tx.Where("plan_id = ?", plan.PlanID).Delete(&CoachingTask{}).Error; err != nil {
			return err
		}

		for i, t := range parsed.Tasks {
			sequence := t.Sequence
			if sequence <= 0 {
				sequence = i + 1
			}
			dayIndex := t.DayIndex
			if dayIndex <= 0 {
				dayIndex = 1
			}
			task := CoachingTask{
				TaskID:           uuid.New().String(),
				PlanID:           plan.PlanID,
				UserID:           session.UserID,
				InterviewID:      session.InterviewID,
				SourceType:       CoachingPlanSourceInterview,
				Sequence:         sequence,
				DayIndex:         dayIndex,
				TaskType:         normalizeDefault(t.TaskType, "weakness_fix"),
				Title:            t.Title,
				Description:      t.Description,
				RelatedMemoryIDs: marshalStringSlice(t.RelatedMemoryIDs),
				Priority:         normalizeCoachingPriority(t.Priority),
				Status:           CoachingTaskStatusTodo,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}
	return toCoachingPlanVO(savedPlan), nil
}

func (s *Server) upsertFailedCoachingPlan(session InterviewSession, req vo.GenerateCoachingPlanReq, rawOutput string, cause error) (vo.CoachingPlanVO, error) {
	now := time.Now().Unix()
	var savedPlan CoachingPlan
	err := s.db.Transaction(func(tx *gorm.DB) error {
		plan, err := upsertCoachingPlan(tx, session, req, now, func(plan *CoachingPlan) {
			plan.RawAgentOutput = rawOutput
			plan.Status = CoachingPlanStatusFailed
			if cause != nil {
				plan.OverallStrategy = cause.Error()
			}
		})
		if err != nil {
			return err
		}
		savedPlan = plan
		if err := tx.Where("plan_id = ?", plan.PlanID).Delete(&CoachingTask{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}
	return toCoachingPlanVO(savedPlan), nil
}

func (s *Server) saveSuccessfulPracticeGoalCoachingPlan(goal PracticeGoal, req vo.GeneratePracticeGoalCoachingPlanReq, rawOutput string, parsed coachingPlanOutput) (vo.CoachingPlanVO, error) {
	now := time.Now().Unix()
	var savedPlan CoachingPlan
	err := s.db.Transaction(func(tx *gorm.DB) error {
		plan, err := upsertPracticeGoalCoachingPlan(tx, goal, req, now, func(plan *CoachingPlan) {
			plan.OverallStrategy = parsed.OverallStrategy
			plan.FocusSummary = parsed.FocusSummary
			plan.RawAgentOutput = rawOutput
			plan.Status = CoachingPlanStatusGenerated
		})
		if err != nil {
			return err
		}
		savedPlan = plan
		if err := tx.Where("plan_id = ?", plan.PlanID).Delete(&CoachingTask{}).Error; err != nil {
			return err
		}
		for i, t := range parsed.Tasks {
			sequence := t.Sequence
			if sequence <= 0 {
				sequence = i + 1
			}
			dayIndex := t.DayIndex
			if dayIndex <= 0 {
				dayIndex = 1
			}
			task := CoachingTask{
				TaskID:           uuid.New().String(),
				PlanID:           plan.PlanID,
				UserID:           goal.UserID,
				InterviewID:      "",
				PracticeGoalID:   goal.GoalID,
				SourceType:       CoachingPlanSourcePracticeGoal,
				Sequence:         sequence,
				DayIndex:         dayIndex,
				TaskType:         normalizeDefault(t.TaskType, "weakness_fix"),
				Title:            t.Title,
				Description:      t.Description,
				RelatedMemoryIDs: marshalStringSlice(t.RelatedMemoryIDs),
				Priority:         normalizeCoachingPriority(t.Priority),
				Status:           CoachingTaskStatusTodo,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}
	return toCoachingPlanVO(savedPlan), nil
}

func (s *Server) upsertFailedPracticeGoalCoachingPlan(goal PracticeGoal, req vo.GeneratePracticeGoalCoachingPlanReq, rawOutput string, cause error) (vo.CoachingPlanVO, error) {
	now := time.Now().Unix()
	var savedPlan CoachingPlan
	err := s.db.Transaction(func(tx *gorm.DB) error {
		plan, err := upsertPracticeGoalCoachingPlan(tx, goal, req, now, func(plan *CoachingPlan) {
			plan.RawAgentOutput = rawOutput
			plan.Status = CoachingPlanStatusFailed
			if cause != nil {
				plan.OverallStrategy = cause.Error()
			}
		})
		if err != nil {
			return err
		}
		savedPlan = plan
		if err := tx.Where("plan_id = ?", plan.PlanID).Delete(&CoachingTask{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return vo.CoachingPlanVO{}, err
	}
	return toCoachingPlanVO(savedPlan), nil
}

func upsertPracticeGoalCoachingPlan(tx *gorm.DB, goal PracticeGoal, req vo.GeneratePracticeGoalCoachingPlanReq, now int64, mutate func(*CoachingPlan)) (CoachingPlan, error) {
	var plan CoachingPlan
	err := tx.First(&plan, "practice_goal_id = ? AND user_id = ?", goal.GoalID, goal.UserID).Error
	switch {
	case err == nil:
		plan.InterviewID = ""
		plan.PracticeGoalID = goal.GoalID
		plan.SourceType = CoachingPlanSourcePracticeGoal
		plan.UserID = goal.UserID
		plan.TargetRound = normalizeDefault(strings.TrimSpace(req.TargetRound), goal.TargetRound)
		plan.RemainingDays = normalizedPracticeGoalRemainingDays(req.RemainingDays)
		plan.CompanyName = goal.CompanyName
		plan.JobTitle = goal.JobTitle
		plan.UpdatedAt = now
		mutate(&plan)
		if err := tx.Save(&plan).Error; err != nil {
			return CoachingPlan{}, err
		}
		return plan, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		plan = CoachingPlan{
			PlanID:         uuid.New().String(),
			UserID:         goal.UserID,
			InterviewID:    "",
			PracticeGoalID: goal.GoalID,
			SourceType:     CoachingPlanSourcePracticeGoal,
			TargetRound:    normalizeDefault(strings.TrimSpace(req.TargetRound), goal.TargetRound),
			RemainingDays:  normalizedPracticeGoalRemainingDays(req.RemainingDays),
			CompanyName:    goal.CompanyName,
			JobTitle:       goal.JobTitle,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		mutate(&plan)
		if err := tx.Create(&plan).Error; err != nil {
			return CoachingPlan{}, err
		}
		return plan, nil
	default:
		return CoachingPlan{}, err
	}
}

func upsertCoachingPlan(tx *gorm.DB, session InterviewSession, req vo.GenerateCoachingPlanReq, now int64, mutate func(*CoachingPlan)) (CoachingPlan, error) {
	var plan CoachingPlan
	err := tx.First(&plan, "interview_id = ?", session.InterviewID).Error
	switch {
	case err == nil:
		plan.UserID = req.UserID
		plan.TargetRound = req.TargetRound
		plan.RemainingDays = req.RemainingDays
		plan.CompanyName = session.CompanyName
		plan.JobTitle = session.JobTitle
		plan.SourceType = CoachingPlanSourceInterview
		plan.UpdatedAt = now
		mutate(&plan)
		if err := tx.Save(&plan).Error; err != nil {
			return CoachingPlan{}, err
		}
		return plan, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		plan = CoachingPlan{
			PlanID:        uuid.New().String(),
			UserID:        req.UserID,
			InterviewID:   session.InterviewID,
			SourceType:    CoachingPlanSourceInterview,
			TargetRound:   req.TargetRound,
			RemainingDays: req.RemainingDays,
			CompanyName:   session.CompanyName,
			JobTitle:      session.JobTitle,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		mutate(&plan)
		if err := tx.Create(&plan).Error; err != nil {
			return CoachingPlan{}, err
		}
		return plan, nil
	default:
		return CoachingPlan{}, err
	}
}

func buildCoachingPlanPrompt(input coachingInput, req vo.GenerateCoachingPlanReq) string {
	questionsPayload := make([]map[string]any, 0, len(input.questions))
	for _, q := range input.questions {
		questionsPayload = append(questionsPayload, map[string]any{
			"sequence":               q.Sequence,
			"question":               q.Question,
			"answer_quality":         q.AnswerQuality,
			"topic_tags":             unmarshalStringSlice(q.TopicTags),
			"weakness_summary":       q.WeaknessSummary,
			"improvement_suggestion": q.ImprovementSuggestion,
		})
	}
	questionsJSON, _ := json.Marshal(questionsPayload)

	memoryNote := ""
	if len(input.selection.MemoryItems) == 0 {
		memoryNote = "There are no selected relevant active memory_items. In focus_summary, explicitly state that long-term memory is insufficient and the plan is mainly based on available fallback context."
	}
	contextSection := buildCoachingPlanContextSection(input)
	sourceUserID := input.session.UserID
	sourceInterviewID := input.session.InterviewID
	companyName := input.session.CompanyName
	jobTitle := input.session.JobTitle
	interviewRound := input.session.InterviewRound
	interviewType := input.session.InterviewType
	if input.practiceGoal != nil {
		sourceUserID = input.practiceGoal.UserID
		companyName = input.practiceGoal.CompanyName
		jobTitle = input.practiceGoal.JobTitle
		interviewRound = input.practiceGoal.TargetRound
		interviewType = "practice_goal"
	}

	return fmt.Sprintf(`Generate a one-time preparation plan for the next interview round.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.

Do not write or update long-term memory.
Do not start a mock interview conversation.
Only generate a concrete preparation plan and task list.
%s

JSON schema:
{
  "overall_strategy": "string",
  "focus_summary": "string",
  "tasks": [
    {
      "sequence": 1,
      "day_index": 1,
      "task_type": "knowledge_review|project_answer_polish|mock_question|weakness_fix|company_preparation",
      "title": "string",
      "description": "string",
      "related_memory_ids": ["string"],
      "priority": "high|medium|low"
    }
  ]
}

Interview session:
- user_id: %s
- interview_id: %s
- context_source: %s
- company_name: %s
- job_title: %s
- interview_round: %s
- interview_type: %s

Target:
- target_round: %s
- remaining_days: %d

%s

Structured questions:
%s

Selected memory_items:
%s

Selected practice_states:
%s`,
		memoryNote,
		sourceUserID,
		sourceInterviewID,
		input.sourceType,
		companyName,
		jobTitle,
		interviewRound,
		interviewType,
		req.TargetRound,
		req.RemainingDays,
		contextSection,
		string(questionsJSON),
		selectedMemoriesJSON(input.selection.MemoryItems),
		selectedPracticeStatesJSON(input.selection.PracticeStates),
	)
}

func buildCoachingPlanContextSection(input coachingInput) string {
	if input.practiceGoal != nil {
		return fmt.Sprintf(`Practice goal cold start context:
- practice_goal_id: %s
- company_name: %s
- job_title: %s
- target_round: %s
- remaining_days: %d
- job_description: %s
- self_reported_focus_topics: %s`,
			input.practiceGoal.GoalID,
			input.practiceGoal.CompanyName,
			input.practiceGoal.JobTitle,
			input.practiceGoal.TargetRound,
			input.practiceGoal.RemainingDays,
			input.practiceGoal.JobDescription,
			strings.Join(unmarshalStringSlice(input.practiceGoal.FocusTopics), ", "),
		)
	}
	if input.report == nil {
		return "Fallback context: no review report or practice goal is available."
	}
	return fmt.Sprintf(`Review report:
- overall_summary: %s
- strengths: %s
- weaknesses: %s
- follow_up_risks: %s
- suggested_preparation: %s`,
		input.report.OverallSummary,
		input.report.Strengths,
		input.report.Weaknesses,
		input.report.FollowUpRisks,
		input.report.SuggestedPreparation,
	)
}

func parseCoachingPlanOutput(raw string) (coachingPlanOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed coachingPlanOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return coachingPlanOutput{}, fmt.Errorf("parse coaching plan JSON: %w", err)
	}
	if parsed.Tasks == nil {
		parsed.Tasks = []coachingTaskOutput{}
	}
	return parsed, nil
}

func normalizeCoachingPriority(priority string) string {
	switch priority {
	case CoachingTaskPriorityHigh, CoachingTaskPriorityMedium, CoachingTaskPriorityLow:
		return priority
	default:
		return CoachingTaskPriorityMedium
	}
}

func validCoachingTaskStatus(status string) bool {
	switch status {
	case CoachingTaskStatusTodo,
		CoachingTaskStatusInProgress,
		CoachingTaskStatusNeedsRevision,
		CoachingTaskStatusDone,
		CoachingTaskStatusSkipped:
		return true
	default:
		return false
	}
}

func toCoachingPlanVO(plan CoachingPlan) vo.CoachingPlanVO {
	return vo.CoachingPlanVO{
		PlanID:          plan.PlanID,
		UserID:          plan.UserID,
		InterviewID:     plan.InterviewID,
		PracticeGoalID:  plan.PracticeGoalID,
		SourceType:      normalizeDefault(plan.SourceType, CoachingPlanSourceInterview),
		TargetRound:     plan.TargetRound,
		RemainingDays:   plan.RemainingDays,
		CompanyName:     plan.CompanyName,
		JobTitle:        plan.JobTitle,
		OverallStrategy: plan.OverallStrategy,
		FocusSummary:    plan.FocusSummary,
		RawAgentOutput:  plan.RawAgentOutput,
		Status:          plan.Status,
		CreatedAt:       plan.CreatedAt,
		UpdatedAt:       plan.UpdatedAt,
	}
}

func toCoachingTaskVO(task CoachingTask) vo.CoachingTaskVO {
	return vo.CoachingTaskVO{
		TaskID:           task.TaskID,
		PlanID:           task.PlanID,
		UserID:           task.UserID,
		InterviewID:      task.InterviewID,
		PracticeGoalID:   task.PracticeGoalID,
		SourceType:       normalizeDefault(task.SourceType, CoachingPlanSourceInterview),
		Sequence:         task.Sequence,
		DayIndex:         task.DayIndex,
		TaskType:         task.TaskType,
		Title:            task.Title,
		Description:      task.Description,
		RelatedMemoryIDs: unmarshalStringSlice(task.RelatedMemoryIDs),
		Priority:         task.Priority,
		Status:           task.Status,
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
	}
}
