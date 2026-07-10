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
	MemoryCandidateStatusPending  = "pending"
	MemoryCandidateStatusAccepted = "accepted"
	MemoryCandidateStatusRejected = "rejected"

	MemoryItemStatusActive   = "active"
	MemoryItemStatusArchived = "archived"

	MemoryTypeUserWeakness     = "user_weakness"
	MemoryTypeUserStrength     = "user_strength"
	MemoryTypeCompanyProfile   = "company_profile"
	MemoryTypeJobProfile       = "job_profile"
	MemoryTypeInterviewerFocus = "interviewer_focus"
	MemoryTypeQuestionPattern  = "question_pattern"
	MemoryTypePreparationTip   = "preparation_tip"

	MemoryConfidenceLow    = "low"
	MemoryConfidenceMedium = "medium"
	MemoryConfidenceHigh   = "high"

	MemorySourceReviewReport      = "review_report"
	MemorySourceInterviewQuestion = "interview_question"
	MemorySourceAgentGenerated    = "agent_generated"
	MemorySourceCoachingSession   = "coaching_session"
	MemorySourceMockInterview     = "mock_interview"
)

type memoryCuratorOutput struct {
	Candidates []memoryCandidateOutput `json:"candidates"`
}

type memoryCandidateOutput struct {
	MemoryType string `json:"memory_type"`
	SubjectKey string `json:"subject_key"`
	Content    string `json:"content"`
	Evidence   string `json:"evidence"`
	Confidence string `json:"confidence"`
	Source     string `json:"source"`
}

type memoryCandidateSourceRef struct {
	SourceRefType string
	SourceRefID   string
	ReplaceReview bool
}

type MemoryCandidateQuery struct {
	UserID        string
	Status        string
	SourceRefType string
	CompanyName   string
	JobTitle      string
}

type coachingSessionMemoryCandidateInput struct {
	session         InterviewSession
	report          *InterviewReviewReport
	questions       []InterviewQuestion
	plan            CoachingPlan
	tasks           []CoachingTask
	coachingSession CoachingSession
	turns           []CoachingSessionTurn
	attempts        []CoachingTaskAttempt
	practiceStates  []PracticeState
}

type mockInterviewMemoryCandidateInput struct {
	session        InterviewSession
	report         *InterviewReviewReport
	questions      []InterviewQuestion
	mock           MockInterview
	turns          []MockTurn
	plan           *CoachingPlan
	tasks          []CoachingTask
	practiceStates []PracticeState
}

func (s *Server) GenerateMemoryCandidates(ctx context.Context, interviewID string) ([]vo.MemoryCandidateVO, error) {
	if s.agents == nil {
		return nil, fmt.Errorf("agent provider is nil")
	}

	session, report, questions, err := s.loadMemoryCandidateInput(interviewID)
	if err != nil {
		return nil, err
	}
	if session.Status != InterviewStatusReviewed {
		return nil, fmt.Errorf("interview status must be %q before generating memory candidates", InterviewStatusReviewed)
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeMemoryCurator))
	if err != nil {
		return nil, err
	}

	result, err := runner.RunTask(ctx, buildMemoryCandidatePrompt(session, report, questions))
	if err != nil {
		log.Warnf("memory curator agent failed for interview %s: %v", interviewID, err)
		return nil, fmt.Errorf("memory curator agent failed: %w", err)
	}

	parsed, err := parseMemoryCuratorOutput(result.Response)
	if err != nil {
		log.Warnf("parse memory curator output failed for interview %s: %v, raw=%s", interviewID, err, result.Response)
		return nil, err
	}

	return s.saveMemoryCandidates(session, result.Response, parsed.Candidates, memoryCandidateSourceRef{ReplaceReview: true})
}

func (s *Server) GenerateMemoryCandidatesFromCoachingSession(ctx context.Context, sessionID string) ([]vo.MemoryCandidateVO, error) {
	if s.agents == nil {
		return nil, fmt.Errorf("agent provider is nil")
	}

	var coachingSession CoachingSession
	if err := s.db.First(&coachingSession, "session_id = ?", sessionID).Error; err != nil {
		return nil, err
	}
	if coachingSession.Status != CoachingSessionStatusCompleted {
		return nil, fmt.Errorf("coaching session status must be %q before generating memory candidates", CoachingSessionStatusCompleted)
	}

	existing, err := s.findExistingMemoryCandidatesForSource(MemorySourceCoachingSession, sessionID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		input, err := s.loadCoachingSessionMemoryCandidateInput(coachingSession)
		if err != nil {
			return nil, err
		}
		if _, err := s.generateMemoryEventsFromCoachingSession(input); err != nil {
			return nil, err
		}
		return toMemoryCandidateVOs(existing), nil
	}

	input, err := s.loadCoachingSessionMemoryCandidateInput(coachingSession)
	if err != nil {
		return nil, err
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeMemoryCurator))
	if err != nil {
		return nil, err
	}
	prompt := buildCoachingSessionMemoryCandidatePrompt(input)
	inputSnapshot := marshalTraceJSON(map[string]any{
		"session_id":       input.coachingSession.SessionID,
		"interview_id":     input.session.InterviewID,
		"user_id":          input.session.UserID,
		"coaching_plan_id": input.plan.PlanID,
		"session_status":   input.coachingSession.Status,
		"task_count":       len(input.tasks),
		"turn_count":       len(input.turns),
		"attempt_count":    len(input.attempts),
		"question_count":   len(input.questions),
		"practice_count":   len(input.practiceStates),
		"prompt_length":    len(prompt),
	})
	result, err := runner.RunTask(ctx, prompt)
	if err != nil {
		log.Warnf("memory curator agent failed for coaching session %s: %v", sessionID, err)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       sessionID,
			StepName:       AgentTraceStepCoachingSessionMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ServiceActions: marshalTraceJSON([]string{"did not create memory_candidates"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(err),
		})
		return nil, fmt.Errorf("memory curator agent failed: %w", err)
	}

	parsed, err := parseMemoryCuratorOutput(result.Response)
	if err != nil {
		log.Warnf("parse coaching session memory curator output failed for session %s: %v, raw=%s", sessionID, err, result.Response)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       sessionID,
			StepName:       AgentTraceStepCoachingSessionMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ServiceActions: marshalTraceJSON([]string{"did not create memory_candidates"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(err),
		})
		return nil, err
	}
	candidates, err := s.saveMemoryCandidates(input.session, result.Response, parsed.Candidates, memoryCandidateSourceRef{
		SourceRefType: MemorySourceCoachingSession,
		SourceRefID:   sessionID,
	})
	if err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       sessionID,
			StepName:       AgentTraceStepCoachingSessionMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ParsedDecision: marshalTraceJSON(parsed),
			ServiceActions: marshalTraceJSON([]string{"failed to persist memory_candidates"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(err),
		})
		return nil, err
	}
	events, err := s.generateMemoryEventsFromCoachingSession(input)
	if err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       sessionID,
			StepName:       AgentTraceStepCoachingSessionMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ParsedDecision: marshalTraceJSON(parsed),
			ServiceActions: marshalTraceJSON([]string{
				fmt.Sprintf("generated memory_candidates: %d", len(candidates)),
				"failed to generate memory_events after candidate persistence",
			}),
			Status:       AgentDecisionTraceStatusFailed,
			ErrorMessage: traceErrorMessage(err),
		})
		return nil, err
	}
	s.recordAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:         input.session.UserID,
		InterviewID:    input.session.InterviewID,
		AgentType:      string(agent.AgentTypeMemoryCurator),
		SourceType:     AgentTraceSourceMemoryCandidateGeneration,
		SourceID:       sessionID,
		StepName:       AgentTraceStepCoachingSessionMemoryCandidates,
		InputSnapshot:  inputSnapshot,
		RawAgentOutput: result.Response,
		ParsedDecision: marshalTraceJSON(parsed),
		ServiceActions: marshalTraceJSON([]string{
			fmt.Sprintf("generated memory_candidates: %d", len(candidates)),
			fmt.Sprintf("generated memory_events: %d", len(events)),
		}),
		Status: AgentDecisionTraceStatusSucceeded,
	})
	return candidates, nil
}

func (s *Server) GenerateMemoryCandidatesFromMockInterview(ctx context.Context, mockID string) ([]vo.MemoryCandidateVO, error) {
	if s.agents == nil {
		return nil, fmt.Errorf("agent provider is nil")
	}

	var mock MockInterview
	if err := s.db.First(&mock, "mock_id = ?", mockID).Error; err != nil {
		return nil, err
	}
	if mock.Status != MockInterviewStatusCompleted {
		return nil, fmt.Errorf("mock interview status must be %q before generating memory candidates", MockInterviewStatusCompleted)
	}

	existing, err := s.findExistingMemoryCandidatesForSource(MemorySourceMockInterview, mockID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		input, err := s.loadMockInterviewMemoryCandidateInput(mock)
		if err != nil {
			return nil, err
		}
		if _, err := s.generateMemoryEventsFromMockInterview(input); err != nil {
			return nil, err
		}
		return toMemoryCandidateVOs(existing), nil
	}

	input, err := s.loadMockInterviewMemoryCandidateInput(mock)
	if err != nil {
		return nil, err
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeMemoryCurator))
	if err != nil {
		return nil, err
	}
	prompt := buildMockInterviewMemoryCandidatePrompt(input)
	inputSnapshot := marshalTraceJSON(map[string]any{
		"mock_id":        input.mock.MockID,
		"interview_id":   input.session.InterviewID,
		"user_id":        input.session.UserID,
		"plan_id":        input.mock.PlanID,
		"mock_status":    input.mock.Status,
		"turn_count":     len(input.turns),
		"question_count": len(input.questions),
		"task_count":     len(input.tasks),
		"practice_count": len(input.practiceStates),
		"prompt_length":  len(prompt),
	})
	result, err := runner.RunTask(ctx, prompt)
	if err != nil {
		log.Warnf("memory curator agent failed for mock interview %s: %v", mockID, err)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       mockID,
			StepName:       AgentTraceStepMockInterviewMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ServiceActions: marshalTraceJSON([]string{"did not create memory_candidates"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(err),
		})
		return nil, fmt.Errorf("memory curator agent failed: %w", err)
	}

	parsed, err := parseMemoryCuratorOutput(result.Response)
	if err != nil {
		log.Warnf("parse mock interview memory curator output failed for mock %s: %v, raw=%s", mockID, err, result.Response)
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       mockID,
			StepName:       AgentTraceStepMockInterviewMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ServiceActions: marshalTraceJSON([]string{"did not create memory_candidates"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(err),
		})
		return nil, err
	}
	candidates, err := s.saveMemoryCandidates(input.session, result.Response, parsed.Candidates, memoryCandidateSourceRef{
		SourceRefType: MemorySourceMockInterview,
		SourceRefID:   mockID,
	})
	if err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       mockID,
			StepName:       AgentTraceStepMockInterviewMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ParsedDecision: marshalTraceJSON(parsed),
			ServiceActions: marshalTraceJSON([]string{"failed to persist memory_candidates"}),
			Status:         AgentDecisionTraceStatusFailed,
			ErrorMessage:   traceErrorMessage(err),
		})
		return nil, err
	}
	events, err := s.generateMemoryEventsFromMockInterview(input)
	if err != nil {
		s.recordAgentDecisionTrace(AgentDecisionTraceInput{
			UserID:         input.session.UserID,
			InterviewID:    input.session.InterviewID,
			AgentType:      string(agent.AgentTypeMemoryCurator),
			SourceType:     AgentTraceSourceMemoryCandidateGeneration,
			SourceID:       mockID,
			StepName:       AgentTraceStepMockInterviewMemoryCandidates,
			InputSnapshot:  inputSnapshot,
			RawAgentOutput: result.Response,
			ParsedDecision: marshalTraceJSON(parsed),
			ServiceActions: marshalTraceJSON([]string{
				fmt.Sprintf("generated memory_candidates: %d", len(candidates)),
				"failed to generate memory_events after candidate persistence",
			}),
			Status:       AgentDecisionTraceStatusFailed,
			ErrorMessage: traceErrorMessage(err),
		})
		return nil, err
	}
	s.recordAgentDecisionTrace(AgentDecisionTraceInput{
		UserID:         input.session.UserID,
		InterviewID:    input.session.InterviewID,
		AgentType:      string(agent.AgentTypeMemoryCurator),
		SourceType:     AgentTraceSourceMemoryCandidateGeneration,
		SourceID:       mockID,
		StepName:       AgentTraceStepMockInterviewMemoryCandidates,
		InputSnapshot:  inputSnapshot,
		RawAgentOutput: result.Response,
		ParsedDecision: marshalTraceJSON(parsed),
		ServiceActions: marshalTraceJSON([]string{
			fmt.Sprintf("generated memory_candidates: %d", len(candidates)),
			fmt.Sprintf("generated memory_events: %d", len(events)),
		}),
		Status: AgentDecisionTraceStatusSucceeded,
	})
	return candidates, nil
}

func (s *Server) ListMemoryCandidates(interviewID string) ([]vo.MemoryCandidateVO, error) {
	var candidates []MemoryCandidate
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("created_at asc").
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	return toMemoryCandidateVOs(candidates), nil
}

func (s *Server) ListMemoryCandidatesByQuery(query MemoryCandidateQuery) ([]vo.MemoryCandidateVO, error) {
	userID := strings.TrimSpace(query.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	db := s.db.Model(&MemoryCandidate{}).
		Select("memory_candidates.*").
		Joins("JOIN interview_sessions ON interview_sessions.interview_id = memory_candidates.interview_id").
		Where("memory_candidates.user_id = ?", userID)

	if status := strings.TrimSpace(query.Status); status != "" {
		db = db.Where("memory_candidates.status = ?", status)
	}
	if sourceRefType := strings.TrimSpace(query.SourceRefType); sourceRefType != "" {
		db = db.Where("memory_candidates.source_ref_type = ?", sourceRefType)
	}
	if companyName := strings.TrimSpace(query.CompanyName); companyName != "" {
		db = db.Where("interview_sessions.company_name = ?", companyName)
	}
	if jobTitle := strings.TrimSpace(query.JobTitle); jobTitle != "" {
		db = db.Where("interview_sessions.job_title = ?", jobTitle)
	}

	var candidates []MemoryCandidate
	if err := db.Order("memory_candidates.created_at asc").Find(&candidates).Error; err != nil {
		return nil, err
	}
	return toMemoryCandidateVOs(candidates), nil
}

func (s *Server) AcceptMemoryCandidate(candidateID string) (vo.MemoryItemVO, error) {
	var item MemoryItem
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var candidate MemoryCandidate
		if err := tx.First(&candidate, "candidate_id = ?", candidateID).Error; err != nil {
			return err
		}
		if candidate.Status == MemoryCandidateStatusRejected {
			return fmt.Errorf("cannot accept rejected memory candidate")
		}

		if err := tx.First(&item, "source_candidate_id = ?", candidateID).Error; err == nil {
			if candidate.Status != MemoryCandidateStatusAccepted {
				now := time.Now().Unix()
				candidate.Status = MemoryCandidateStatusAccepted
				candidate.UpdatedAt = now
				if err := tx.Save(&candidate).Error; err != nil {
					return err
				}
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := time.Now().Unix()
		candidate.Status = MemoryCandidateStatusAccepted
		candidate.UpdatedAt = now
		if err := tx.Save(&candidate).Error; err != nil {
			return err
		}

		item = MemoryItem{
			MemoryID:          uuid.New().String(),
			UserID:            candidate.UserID,
			MemoryType:        candidate.MemoryType,
			SubjectKey:        candidate.SubjectKey,
			Content:           candidate.Content,
			Evidence:          candidate.Evidence,
			Confidence:        candidate.Confidence,
			SourceCandidateID: candidate.CandidateID,
			SourceInterviewID: candidate.InterviewID,
			Status:            MemoryItemStatusActive,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		return tx.Create(&item).Error
	})
	if err != nil {
		return vo.MemoryItemVO{}, err
	}
	return toMemoryItemVO(item), nil
}

func (s *Server) RejectMemoryCandidate(candidateID string) (vo.MemoryCandidateVO, error) {
	var candidate MemoryCandidate
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&candidate, "candidate_id = ?", candidateID).Error; err != nil {
			return err
		}
		if candidate.Status == MemoryCandidateStatusAccepted {
			return fmt.Errorf("cannot reject accepted memory candidate")
		}
		if candidate.Status == MemoryCandidateStatusRejected {
			return nil
		}
		candidate.Status = MemoryCandidateStatusRejected
		candidate.UpdatedAt = time.Now().Unix()
		return tx.Save(&candidate).Error
	})
	if err != nil {
		return vo.MemoryCandidateVO{}, err
	}
	return toMemoryCandidateVO(candidate), nil
}

func (s *Server) ListMemoryItems(userID string) ([]vo.MemoryItemVO, error) {
	var items []MemoryItem
	query := s.db.Where("status = ?", MemoryItemStatusActive).Order("created_at desc")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]vo.MemoryItemVO, 0, len(items))
	for _, item := range items {
		result = append(result, toMemoryItemVO(item))
	}
	return result, nil
}

func (s *Server) loadMemoryCandidateInput(interviewID string) (InterviewSession, InterviewReviewReport, []InterviewQuestion, error) {
	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return InterviewSession{}, InterviewReviewReport{}, nil, err
	}

	var report InterviewReviewReport
	if err := s.db.First(&report, "interview_id = ?", interviewID).Error; err != nil {
		return InterviewSession{}, InterviewReviewReport{}, nil, err
	}
	if report.Status != InterviewReviewStatusGenerated {
		return InterviewSession{}, InterviewReviewReport{}, nil, fmt.Errorf("review report must be %q before generating memory candidates", InterviewReviewStatusGenerated)
	}

	var questions []InterviewQuestion
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("sequence asc").
		Find(&questions).Error; err != nil {
		return InterviewSession{}, InterviewReviewReport{}, nil, err
	}

	return session, report, questions, nil
}

func (s *Server) loadCoachingSessionMemoryCandidateInput(coachingSession CoachingSession) (coachingSessionMemoryCandidateInput, error) {
	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", coachingSession.InterviewID).Error; err != nil {
		return coachingSessionMemoryCandidateInput{}, err
	}
	var plan CoachingPlan
	if err := s.db.First(&plan, "plan_id = ?", coachingSession.CoachingPlanID).Error; err != nil {
		return coachingSessionMemoryCandidateInput{}, err
	}
	tasks, err := s.loadCoachingTasks(coachingSession.CoachingPlanID)
	if err != nil {
		return coachingSessionMemoryCandidateInput{}, err
	}
	var turns []CoachingSessionTurn
	if err := s.db.Where("session_id = ?", coachingSession.SessionID).Order("created_at asc").Find(&turns).Error; err != nil {
		return coachingSessionMemoryCandidateInput{}, err
	}
	var attempts []CoachingTaskAttempt
	if err := s.db.Where("session_id = ?", coachingSession.SessionID).Order("created_at asc, attempt_index asc").Find(&attempts).Error; err != nil {
		return coachingSessionMemoryCandidateInput{}, err
	}
	practiceStates, err := s.loadPracticeStatesForPrompt(coachingSession.UserID, defaultPracticeStateSelectionLimit)
	if err != nil {
		return coachingSessionMemoryCandidateInput{}, err
	}

	var report *InterviewReviewReport
	var loadedReport InterviewReviewReport
	if err := s.db.First(&loadedReport, "interview_id = ?", coachingSession.InterviewID).Error; err == nil {
		report = &loadedReport
	}
	var questions []InterviewQuestion
	_ = s.db.Where("interview_id = ?", coachingSession.InterviewID).Order("sequence asc").Find(&questions).Error

	return coachingSessionMemoryCandidateInput{
		session:         session,
		report:          report,
		questions:       questions,
		plan:            plan,
		tasks:           tasks,
		coachingSession: coachingSession,
		turns:           turns,
		attempts:        attempts,
		practiceStates:  practiceStates,
	}, nil
}

func (s *Server) loadMockInterviewMemoryCandidateInput(mock MockInterview) (mockInterviewMemoryCandidateInput, error) {
	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", mock.InterviewID).Error; err != nil {
		return mockInterviewMemoryCandidateInput{}, err
	}
	turns, err := s.loadMockTurns(mock.MockID)
	if err != nil {
		return mockInterviewMemoryCandidateInput{}, err
	}
	practiceStates, err := s.loadPracticeStatesForPrompt(mock.UserID, defaultPracticeStateSelectionLimit)
	if err != nil {
		return mockInterviewMemoryCandidateInput{}, err
	}

	var report *InterviewReviewReport
	var loadedReport InterviewReviewReport
	if err := s.db.First(&loadedReport, "interview_id = ?", mock.InterviewID).Error; err == nil {
		report = &loadedReport
	}
	var questions []InterviewQuestion
	_ = s.db.Where("interview_id = ?", mock.InterviewID).Order("sequence asc").Find(&questions).Error

	var plan *CoachingPlan
	var tasks []CoachingTask
	if strings.TrimSpace(mock.PlanID) != "" {
		var loadedPlan CoachingPlan
		if err := s.db.First(&loadedPlan, "plan_id = ?", mock.PlanID).Error; err == nil {
			plan = &loadedPlan
			_ = s.db.Where("plan_id = ?", mock.PlanID).Order("sequence asc").Find(&tasks).Error
		}
	}

	return mockInterviewMemoryCandidateInput{
		session:        session,
		report:         report,
		questions:      questions,
		mock:           mock,
		turns:          turns,
		plan:           plan,
		tasks:          tasks,
		practiceStates: practiceStates,
	}, nil
}

func (s *Server) findExistingMemoryCandidatesForSource(sourceRefType string, sourceRefID string) ([]MemoryCandidate, error) {
	var candidates []MemoryCandidate
	if err := s.db.Where("source_ref_type = ? AND source_ref_id = ? AND status IN ?",
		sourceRefType, sourceRefID, []string{MemoryCandidateStatusPending, MemoryCandidateStatusAccepted}).
		Order("created_at asc").
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	return candidates, nil
}

func (s *Server) generateMemoryEventsFromCoachingSession(input coachingSessionMemoryCandidateInput) ([]MemoryEvent, error) {
	attemptsByTask := make(map[string][]CoachingTaskAttempt)
	for _, attempt := range input.attempts {
		attemptsByTask[attempt.CoachingTaskID] = append(attemptsByTask[attempt.CoachingTaskID], attempt)
	}

	events := make([]MemoryEvent, 0, len(input.tasks))
	now := time.Now().Unix()
	for _, task := range input.tasks {
		topic := strings.TrimSpace(task.Title)
		if topic == "" {
			topic = strings.TrimSpace(task.TaskType)
		}
		if topic == "" {
			continue
		}

		taskAttempts := attemptsByTask[task.TaskID]
		latestScore := 0
		passed := false
		feedback := ""
		if len(taskAttempts) > 0 {
			latest := taskAttempts[len(taskAttempts)-1]
			latestScore = latest.Score
			passed = latest.Passed
			feedback = latest.Feedback
		}
		if feedback == "" {
			feedback = latestCoachingFeedbackForTask(input.turns, task.TaskID)
		}

		scoreTrend := buildScoreTrend(len(taskAttempts), latestScore, passed)
		observation := compactObservation([]string{
			"completed coaching task " + topic,
			scoreTrend,
			compactFeedback(feedback),
		})
		event, _, err := s.createMemoryEventIfMissing(MemoryEvent{
			EventID:     uuid.New().String(),
			UserID:      input.session.UserID,
			SourceType:  MemorySourceCoachingSession,
			SourceID:    input.coachingSession.SessionID,
			Topic:       topic,
			Observation: observation,
			ScoreTrend:  scoreTrend,
			CreatedAt:   now,
		})
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *Server) generateMemoryEventsFromMockInterview(input mockInterviewMemoryCandidateInput) ([]MemoryEvent, error) {
	topics := orderedUniqueMockTopics(input)
	if len(topics) == 0 {
		return nil, nil
	}

	latestScore := 0
	formalCount := 0
	feedback := strings.TrimSpace(input.mock.LastFeedback)
	for _, turn := range input.turns {
		if turn.Score > 0 {
			formalCount++
			latestScore = turn.Score
		}
		if strings.TrimSpace(turn.Feedback) != "" {
			feedback = turn.Feedback
		}
	}

	scoreTrend := buildMockScoreTrend(formalCount, latestScore)
	events := make([]MemoryEvent, 0, len(topics))
	now := time.Now().Unix()
	for _, topic := range topics {
		observation := compactObservation([]string{
			"completed mock interview topic " + topic,
			scoreTrend,
			compactFeedback(feedback),
		})
		event, _, err := s.createMemoryEventIfMissing(MemoryEvent{
			EventID:     uuid.New().String(),
			UserID:      input.session.UserID,
			SourceType:  MemorySourceMockInterview,
			SourceID:    input.mock.MockID,
			Topic:       topic,
			Observation: observation,
			ScoreTrend:  scoreTrend,
			CreatedAt:   now,
		})
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *Server) createMemoryEventIfMissing(event MemoryEvent) (MemoryEvent, bool, error) {
	topic := strings.TrimSpace(event.Topic)
	if topic == "" {
		return MemoryEvent{}, false, fmt.Errorf("memory event topic is required")
	}
	event.Topic = topic

	var existing MemoryEvent
	err := s.db.Where("user_id = ? AND source_type = ? AND source_id = ? AND topic = ?", event.UserID, event.SourceType, event.SourceID, event.Topic).
		First(&existing).Error
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return MemoryEvent{}, false, err
	}

	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = uuid.New().String()
	}
	if event.CreatedAt == 0 {
		event.CreatedAt = time.Now().Unix()
	}
	if err := s.db.Create(&event).Error; err != nil {
		return MemoryEvent{}, false, err
	}
	return event, true, nil
}

func latestCoachingFeedbackForTask(turns []CoachingSessionTurn, taskID string) string {
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if turn.CoachingTaskID == taskID && strings.TrimSpace(turn.Feedback) != "" {
			return turn.Feedback
		}
	}
	return ""
}

func orderedUniqueMockTopics(input mockInterviewMemoryCandidateInput) []string {
	seen := make(map[string]bool)
	topics := make([]string, 0)
	add := func(topic string) {
		topic = strings.TrimSpace(topic)
		if topic == "" || seen[topic] {
			return
		}
		seen[topic] = true
		topics = append(topics, topic)
	}

	for _, turn := range input.turns {
		for _, topic := range unmarshalStringSlice(turn.TopicTags) {
			add(topic)
		}
	}
	add(input.mock.CurrentTopic)
	return topics
}

func buildScoreTrend(attemptCount int, latestScore int, passed bool) string {
	if attemptCount == 0 {
		return "completed without scored attempt"
	}
	if passed {
		return fmt.Sprintf("passed after %d attempt(s), latest score %d", attemptCount, latestScore)
	}
	return fmt.Sprintf("not passed after %d attempt(s), latest score %d", attemptCount, latestScore)
}

func buildMockScoreTrend(formalCount int, latestScore int) string {
	if formalCount == 0 {
		return "completed without scored formal turn"
	}
	return fmt.Sprintf("completed after %d scored turn(s), latest score %d", formalCount, latestScore)
}

func compactObservation(parts []string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, " | ")
}

func compactFeedback(feedback string) string {
	feedback = strings.Join(strings.Fields(feedback), " ")
	if feedback == "" {
		return ""
	}
	const maxFeedbackLen = 180
	if len(feedback) > maxFeedbackLen {
		feedback = feedback[:maxFeedbackLen] + "..."
	}
	return "latest feedback: " + feedback
}

func (s *Server) saveMemoryCandidates(session InterviewSession, rawOutput string, candidates []memoryCandidateOutput, sourceRef memoryCandidateSourceRef) ([]vo.MemoryCandidateVO, error) {
	now := time.Now().Unix()
	saved := make([]MemoryCandidate, 0, len(candidates))
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if sourceRef.ReplaceReview {
			if err := tx.Where(
				"interview_id = ? AND status = ? AND (source_ref_type = '' OR source_ref_type IS NULL) AND (source_ref_id = '' OR source_ref_id IS NULL)",
				session.InterviewID,
				MemoryCandidateStatusPending,
			).
				Delete(&MemoryCandidate{}).Error; err != nil {
				return err
			}
		}

		for _, candidate := range candidates {
			normalized, ok := normalizeMemoryCandidate(session, rawOutput, candidate, sourceRef, now)
			if !ok {
				continue
			}
			if err := tx.Create(&normalized).Error; err != nil {
				return err
			}
			saved = append(saved, normalized)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return toMemoryCandidateVOs(saved), nil
}

func buildMemoryCandidatePrompt(session InterviewSession, report InterviewReviewReport, questions []InterviewQuestion) string {
	questionPayload := make([]map[string]any, 0, len(questions))
	for _, q := range questions {
		questionPayload = append(questionPayload, map[string]any{
			"sequence":               q.Sequence,
			"question":               q.Question,
			"answer":                 q.Answer,
			"topic_tags":             unmarshalStringSlice(q.TopicTags),
			"difficulty":             q.Difficulty,
			"answer_quality":         q.AnswerQuality,
			"weakness_summary":       q.WeaknessSummary,
			"improvement_suggestion": q.ImprovementSuggestion,
			"evidence_text":          q.EvidenceText,
		})
	}
	questionsJSON, _ := json.Marshal(questionPayload)

	return fmt.Sprintf(`Generate candidate long-term memories from this reviewed interview.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.

Allowed memory_type values:
- user_weakness
- user_strength
- company_profile
- job_profile
- interviewer_focus
- question_pattern
- preparation_tip

Allowed confidence values: low, medium, high.
Allowed source values: review_report, interview_question, agent_generated.

Privacy rules:
- Do not record interviewer age, gender, appearance, personality, private identity, family, marital status, or other private attributes.
- interviewer_focus may only describe professional focus, technical evaluation preferences, or follow-up style.
- If evidence contains private interviewer attributes, ignore that information.
- Do not claim any memory has been saved. These are pending candidates only.

JSON schema:
{
  "candidates": [
    {
      "memory_type": "user_weakness",
      "subject_key": "user:USER_ID",
      "content": "string",
      "evidence": "string",
      "confidence": "low|medium|high",
      "source": "review_report|interview_question|agent_generated"
    }
  ]
}

Subject key guidance:
- user-level memories: user:%s
- company profile: company:%s
- job profile: job:%s:%s
- interviewer_focus: interviewer:%s:professional_focus

Interview session:
- user_id: %s
- interview_id: %s
- company_name: %s
- job_title: %s
- interview_round: %s
- interview_type: %s

Review report:
- overall_summary: %s
- strengths: %s
- weaknesses: %s
- follow_up_risks: %s
- suggested_preparation: %s

Structured questions:
%s`,
		session.UserID,
		session.CompanyName,
		session.CompanyName,
		session.JobTitle,
		session.InterviewID,
		session.UserID,
		session.InterviewID,
		session.CompanyName,
		session.JobTitle,
		session.InterviewRound,
		session.InterviewType,
		report.OverallSummary,
		report.Strengths,
		report.Weaknesses,
		report.FollowUpRisks,
		report.SuggestedPreparation,
		string(questionsJSON),
	)
}

func buildCoachingSessionMemoryCandidatePrompt(input coachingSessionMemoryCandidateInput) string {
	payload, _ := json.Marshal(map[string]any{
		"interview": map[string]any{
			"user_id":         input.session.UserID,
			"interview_id":    input.session.InterviewID,
			"company_name":    input.session.CompanyName,
			"job_title":       input.session.JobTitle,
			"interview_round": input.session.InterviewRound,
			"interview_type":  input.session.InterviewType,
		},
		"review_report":          memoryCandidateReportPayload(input.report),
		"interview_questions":    memoryCandidateQuestionPayload(input.questions),
		"coaching_plan":          memoryCandidateCoachingPlanPayload(input.plan, input.tasks),
		"coaching_session":       memoryCandidateCoachingSessionPayload(input.coachingSession, input.turns, input.attempts),
		"recent_practice_states": memoryCandidatePracticeStatePayload(input.practiceStates),
	})

	return fmt.Sprintf(`Generate candidate long-term memories from this completed coaching session.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.

Only generate candidates for stable long-term observations. Do not convert a one-off score, temporary mistake, or momentary emotion into long-term memory.
Practice mastery details belong in practice_states; do not repeat practice-state logs as memory candidates.
Good candidates may describe stable user weaknesses, stable strengths, answer structure preferences, company/job follow-up patterns, or preparation tips.
Do not record private interviewer attributes. Do not claim anything has been saved to memory_items.

Allowed memory_type values:
- user_weakness
- user_strength
- company_profile
- job_profile
- interviewer_focus
- question_pattern
- preparation_tip

Allowed confidence values: low, medium, high.
Allowed source values: coaching_session.

JSON schema:
{
  "candidates": [
    {
      "memory_type": "user_weakness",
      "subject_key": "user:%s",
      "content": "string",
      "evidence": "string",
      "confidence": "low|medium|high",
      "source": "coaching_session"
    }
  ]
}

Subject key guidance:
- user-level memories: user:%s
- company profile: company:%s
- job profile: job:%s:%s
- interviewer_focus: interviewer:%s:professional_focus

Completed coaching session context JSON:
%s`,
		input.session.UserID,
		input.session.UserID,
		input.session.CompanyName,
		input.session.CompanyName,
		input.session.JobTitle,
		input.session.InterviewID,
		string(payload),
	)
}

func buildMockInterviewMemoryCandidatePrompt(input mockInterviewMemoryCandidateInput) string {
	payload, _ := json.Marshal(map[string]any{
		"interview": map[string]any{
			"user_id":         input.session.UserID,
			"interview_id":    input.session.InterviewID,
			"company_name":    input.session.CompanyName,
			"job_title":       input.session.JobTitle,
			"interview_round": input.session.InterviewRound,
			"interview_type":  input.session.InterviewType,
		},
		"review_report":          memoryCandidateReportPayload(input.report),
		"interview_questions":    memoryCandidateQuestionPayload(input.questions),
		"mock_interview":         memoryCandidateMockInterviewPayload(input.mock, input.turns),
		"coaching_plan":          memoryCandidateOptionalCoachingPayload(input.plan, input.tasks),
		"recent_practice_states": memoryCandidatePracticeStatePayload(input.practiceStates),
	})

	return fmt.Sprintf(`Generate candidate long-term memories from this completed mock interview.

Return STRICT JSON only. Do not return Markdown, code fences, or explanations outside JSON.

Only generate candidates for stable long-term observations. Do not convert a single low score, a one-off miss, or temporary hesitation into long-term memory.
Practice mastery details belong in practice_states; do not repeat mock turn logs as memory candidates.
Good candidates may describe stable user weaknesses, stable strengths, answer structure preferences, company/job follow-up patterns, or preparation tips.
Do not record private interviewer attributes. Do not claim anything has been saved to memory_items.

Allowed memory_type values:
- user_weakness
- user_strength
- company_profile
- job_profile
- interviewer_focus
- question_pattern
- preparation_tip

Allowed confidence values: low, medium, high.
Allowed source values: mock_interview.

JSON schema:
{
  "candidates": [
    {
      "memory_type": "user_weakness",
      "subject_key": "user:%s",
      "content": "string",
      "evidence": "string",
      "confidence": "low|medium|high",
      "source": "mock_interview"
    }
  ]
}

Subject key guidance:
- user-level memories: user:%s
- company profile: company:%s
- job profile: job:%s:%s
- interviewer_focus: interviewer:%s:professional_focus

Completed mock interview context JSON:
%s`,
		input.session.UserID,
		input.session.UserID,
		input.session.CompanyName,
		input.session.CompanyName,
		input.session.JobTitle,
		input.session.InterviewID,
		string(payload),
	)
}

func memoryCandidateReportPayload(report *InterviewReviewReport) any {
	if report == nil {
		return map[string]any{}
	}
	return map[string]any{
		"overall_summary":       report.OverallSummary,
		"strengths":             unmarshalStringSlice(report.Strengths),
		"weaknesses":            unmarshalStringSlice(report.Weaknesses),
		"follow_up_risks":       unmarshalStringSlice(report.FollowUpRisks),
		"suggested_preparation": unmarshalStringSlice(report.SuggestedPreparation),
	}
}

func memoryCandidateQuestionPayload(questions []InterviewQuestion) []map[string]any {
	payload := make([]map[string]any, 0, len(questions))
	for _, q := range questions {
		payload = append(payload, map[string]any{
			"sequence":               q.Sequence,
			"question":               q.Question,
			"answer":                 q.Answer,
			"topic_tags":             unmarshalStringSlice(q.TopicTags),
			"answer_quality":         q.AnswerQuality,
			"weakness_summary":       q.WeaknessSummary,
			"improvement_suggestion": q.ImprovementSuggestion,
		})
	}
	return payload
}

func memoryCandidateCoachingPlanPayload(plan CoachingPlan, tasks []CoachingTask) map[string]any {
	return map[string]any{
		"plan_id":          plan.PlanID,
		"target_round":     plan.TargetRound,
		"overall_strategy": plan.OverallStrategy,
		"focus_summary":    plan.FocusSummary,
		"tasks":            memoryCandidateCoachingTaskPayload(tasks),
	}
}

func memoryCandidateOptionalCoachingPayload(plan *CoachingPlan, tasks []CoachingTask) any {
	if plan == nil {
		return map[string]any{}
	}
	return memoryCandidateCoachingPlanPayload(*plan, tasks)
}

func memoryCandidateCoachingTaskPayload(tasks []CoachingTask) []map[string]any {
	payload := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		payload = append(payload, map[string]any{
			"sequence":    task.Sequence,
			"task_type":   task.TaskType,
			"title":       task.Title,
			"description": task.Description,
			"priority":    task.Priority,
			"status":      task.Status,
		})
	}
	return payload
}

func memoryCandidateCoachingSessionPayload(session CoachingSession, turns []CoachingSessionTurn, attempts []CoachingTaskAttempt) map[string]any {
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
	attemptPayload := make([]map[string]any, 0, len(attempts))
	for _, attempt := range attempts {
		attemptPayload = append(attemptPayload, map[string]any{
			"coaching_task_id": attempt.CoachingTaskID,
			"score":            attempt.Score,
			"feedback":         attempt.Feedback,
			"passed":           attempt.Passed,
			"attempt_index":    attempt.AttemptIndex,
		})
	}
	return map[string]any{
		"session_id":       session.SessionID,
		"status":           session.Status,
		"progress_summary": session.ProgressSummary,
		"turns":            turnPayload,
		"attempts":         attemptPayload,
	}
}

func memoryCandidateMockInterviewPayload(mock MockInterview, turns []MockTurn) map[string]any {
	turnPayload := make([]map[string]any, 0, len(turns))
	for _, turn := range turns {
		turnPayload = append(turnPayload, map[string]any{
			"turn_index":      turn.TurnIndex,
			"role":            turn.Role,
			"turn_type":       turn.TurnType,
			"content":         turn.Content,
			"user_answer":     turn.UserAnswer,
			"feedback":        turn.Feedback,
			"score":           turn.Score,
			"topic_tags":      unmarshalStringSlice(turn.TopicTags),
			"agent_action":    turn.AgentAction,
			"followup_reason": turn.FollowUpReason,
		})
	}
	return map[string]any{
		"mock_id":        mock.MockID,
		"target_round":   mock.TargetRound,
		"status":         mock.Status,
		"current_topic":  mock.CurrentTopic,
		"overall_goal":   mock.OverallGoal,
		"final_summary":  mock.FinalSummary,
		"last_feedback":  mock.LastFeedback,
		"current_turn":   mock.CurrentTurn,
		"structured_log": turnPayload,
	}
}

func memoryCandidatePracticeStatePayload(states []PracticeState) []map[string]any {
	payload := make([]map[string]any, 0, len(states))
	for _, state := range states {
		payload = append(payload, map[string]any{
			"topic":         state.Topic,
			"dimension":     state.Dimension,
			"mastery_score": state.MasteryScore,
			"attempt_count": state.AttemptCount,
			"last_score":    state.LastScore,
			"last_feedback": state.LastFeedback,
			"source_type":   state.SourceType,
		})
	}
	return payload
}

func parseMemoryCuratorOutput(raw string) (memoryCuratorOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed memoryCuratorOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		var candidates []memoryCandidateOutput
		if arrayErr := json.Unmarshal([]byte(cleaned), &candidates); arrayErr != nil {
			return memoryCuratorOutput{}, fmt.Errorf("parse memory curator JSON: %w", err)
		}
		parsed.Candidates = candidates
	}
	if parsed.Candidates == nil {
		parsed.Candidates = []memoryCandidateOutput{}
	}
	return parsed, nil
}

func normalizeMemoryCandidate(session InterviewSession, rawOutput string, candidate memoryCandidateOutput, sourceRef memoryCandidateSourceRef, now int64) (MemoryCandidate, bool) {
	memoryType := strings.TrimSpace(candidate.MemoryType)
	content := strings.TrimSpace(candidate.Content)
	evidence := strings.TrimSpace(candidate.Evidence)
	if !validMemoryType(memoryType) || content == "" {
		return MemoryCandidate{}, false
	}
	if memoryType == MemoryTypeInterviewerFocus && containsPrivateInterviewerSignal(content+" "+evidence+" "+candidate.SubjectKey) {
		return MemoryCandidate{}, false
	}

	confidence := strings.TrimSpace(candidate.Confidence)
	if !validMemoryConfidence(confidence) {
		confidence = MemoryConfidenceMedium
	}
	source := strings.TrimSpace(candidate.Source)
	if !validMemorySource(source) {
		source = MemorySourceAgentGenerated
	}
	subjectKey := strings.TrimSpace(candidate.SubjectKey)
	if subjectKey == "" {
		subjectKey = defaultSubjectKey(session, memoryType)
	}
	if memoryType == MemoryTypeInterviewerFocus && strings.HasPrefix(subjectKey, "interviewer:") {
		subjectKey = "interviewer:" + session.InterviewID + ":professional_focus"
	}

	return MemoryCandidate{
		CandidateID:    uuid.New().String(),
		UserID:         session.UserID,
		InterviewID:    session.InterviewID,
		MemoryType:     memoryType,
		SubjectKey:     subjectKey,
		Content:        content,
		Evidence:       evidence,
		Confidence:     confidence,
		Status:         MemoryCandidateStatusPending,
		Source:         source,
		SourceRefType:  sourceRef.SourceRefType,
		SourceRefID:    sourceRef.SourceRefID,
		RawAgentOutput: rawOutput,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, true
}

func defaultSubjectKey(session InterviewSession, memoryType string) string {
	switch memoryType {
	case MemoryTypeCompanyProfile:
		return "company:" + session.CompanyName
	case MemoryTypeJobProfile:
		return "job:" + session.CompanyName + ":" + session.JobTitle
	case MemoryTypeInterviewerFocus:
		return "interviewer:" + session.InterviewID + ":professional_focus"
	default:
		return "user:" + session.UserID
	}
}

func validMemoryType(memoryType string) bool {
	switch memoryType {
	case MemoryTypeUserWeakness,
		MemoryTypeUserStrength,
		MemoryTypeCompanyProfile,
		MemoryTypeJobProfile,
		MemoryTypeInterviewerFocus,
		MemoryTypeQuestionPattern,
		MemoryTypePreparationTip:
		return true
	default:
		return false
	}
}

func validMemoryConfidence(confidence string) bool {
	switch confidence {
	case MemoryConfidenceLow, MemoryConfidenceMedium, MemoryConfidenceHigh:
		return true
	default:
		return false
	}
}

func validMemorySource(source string) bool {
	switch source {
	case MemorySourceReviewReport,
		MemorySourceInterviewQuestion,
		MemorySourceAgentGenerated,
		MemorySourceCoachingSession,
		MemorySourceMockInterview:
		return true
	default:
		return false
	}
}

func containsPrivateInterviewerSignal(text string) bool {
	privateSignals := []string{
		"年龄", "性别", "男", "女", "外貌", "漂亮", "帅", "性格", "身份", "已婚", "未婚", "家庭", "私人身份",
		"age", "gender", "male", "female", "appearance", "personality", "identity", "married", "family", "private identity",
	}
	lower := strings.ToLower(text)
	for _, signal := range privateSignals {
		if strings.Contains(lower, strings.ToLower(signal)) {
			return true
		}
	}
	return false
}

func toMemoryCandidateVOs(candidates []MemoryCandidate) []vo.MemoryCandidateVO {
	result := make([]vo.MemoryCandidateVO, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, toMemoryCandidateVO(candidate))
	}
	return result
}

func toMemoryCandidateVO(candidate MemoryCandidate) vo.MemoryCandidateVO {
	return vo.MemoryCandidateVO{
		CandidateID:    candidate.CandidateID,
		UserID:         candidate.UserID,
		InterviewID:    candidate.InterviewID,
		MemoryType:     candidate.MemoryType,
		SubjectKey:     candidate.SubjectKey,
		Content:        candidate.Content,
		Evidence:       candidate.Evidence,
		Confidence:     candidate.Confidence,
		Status:         candidate.Status,
		Source:         candidate.Source,
		SourceRefType:  candidate.SourceRefType,
		SourceRefID:    candidate.SourceRefID,
		RawAgentOutput: candidate.RawAgentOutput,
		CreatedAt:      candidate.CreatedAt,
		UpdatedAt:      candidate.UpdatedAt,
	}
}

func toMemoryItemVO(item MemoryItem) vo.MemoryItemVO {
	return vo.MemoryItemVO{
		MemoryID:          item.MemoryID,
		UserID:            item.UserID,
		MemoryType:        item.MemoryType,
		SubjectKey:        item.SubjectKey,
		Content:           item.Content,
		Evidence:          item.Evidence,
		Confidence:        item.Confidence,
		SourceCandidateID: item.SourceCandidateID,
		SourceInterviewID: item.SourceInterviewID,
		Status:            item.Status,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}
