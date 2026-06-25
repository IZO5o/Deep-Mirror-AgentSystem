package server

import (
	"fmt"
	"strings"
	"time"

	"agent-web-base/vo"
)

const (
	workbenchDashboardRecentLimit          = 5
	workbenchDashboardTraceLimit           = 5
	workbenchDashboardEvaluationTraceLimit = 50
	workbenchDashboardWeakMasteryThreshold = 70
	workbenchDashboardRecentWindowSeconds  = 30 * 24 * 60 * 60
)

func (s *Server) GetDashboardSummary(userID string) (vo.DashboardSummaryVO, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return vo.DashboardSummaryVO{}, fmt.Errorf("user_id is required")
	}

	interviews, err := s.dashboardRecentInterviews(userID)
	if err != nil {
		return vo.DashboardSummaryVO{}, err
	}
	pendingCandidates, pendingCount, err := s.dashboardPendingCandidates(userID)
	if err != nil {
		return vo.DashboardSummaryVO{}, err
	}
	coachingSessions, err := s.dashboardActiveCoachingSessions(userID)
	if err != nil {
		return vo.DashboardSummaryVO{}, err
	}
	mockInterviews, err := s.dashboardActiveMockInterviews(userID)
	if err != nil {
		return vo.DashboardSummaryVO{}, err
	}
	practiceSummary, err := s.dashboardPracticeStateSummary(userID)
	if err != nil {
		return vo.DashboardSummaryVO{}, err
	}
	failedTraces, err := s.ListAgentDecisionTraces(AgentDecisionTraceQuery{
		UserID: userID,
		Status: AgentDecisionTraceStatusFailed,
		Limit:  workbenchDashboardTraceLimit,
	})
	if err != nil {
		return vo.DashboardSummaryVO{}, err
	}
	evaluation, err := s.EvaluateAgentDecisionTraces(AgentDecisionTraceQuery{
		UserID: userID,
		Limit:  workbenchDashboardEvaluationTraceLimit,
	})
	if err != nil {
		return vo.DashboardSummaryVO{}, err
	}

	return vo.DashboardSummaryVO{
		RecentInterviews:            interviews,
		PendingMemoryCandidateCount: pendingCount,
		RecentPendingCandidates:     pendingCandidates,
		ActiveCoachingSessions:      coachingSessions,
		ActiveMockInterviews:        mockInterviews,
		PracticeStateSummary:        practiceSummary,
		RecentFailedTraces:          failedTraces,
		EvaluationSummary: vo.DashboardEvaluationSummaryVO{
			TotalTraces:  evaluation.TotalTraces,
			PassedTraces: evaluation.PassedTraces,
			FailedTraces: evaluation.FailedTraces,
		},
	}, nil
}

func (s *Server) dashboardRecentInterviews(userID string) ([]vo.InterviewSessionVO, error) {
	var rows []InterviewSession
	if err := s.db.Where("user_id = ?", userID).
		Order("updated_at desc, created_at desc").
		Limit(workbenchDashboardRecentLimit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]vo.InterviewSessionVO, 0, len(rows))
	for _, row := range rows {
		result = append(result, toInterviewSessionVO(row))
	}
	return result, nil
}

func (s *Server) dashboardPendingCandidates(userID string) ([]vo.MemoryCandidateVO, int, error) {
	var count int64
	if err := s.db.Model(&MemoryCandidate{}).
		Where("user_id = ? AND status = ?", userID, MemoryCandidateStatusPending).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var rows []MemoryCandidate
	if err := s.db.Where("user_id = ? AND status = ?", userID, MemoryCandidateStatusPending).
		Order("updated_at desc, created_at desc").
		Limit(workbenchDashboardRecentLimit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return toMemoryCandidateVOs(rows), int(count), nil
}

func (s *Server) dashboardActiveCoachingSessions(userID string) ([]vo.CoachingSessionVO, error) {
	var rows []CoachingSession
	if err := s.db.Where("user_id = ? AND status IN ?", userID, activeCoachingSessionStatuses()).
		Order("updated_at desc, last_active_at desc, created_at desc").
		Limit(workbenchDashboardRecentLimit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]vo.CoachingSessionVO, 0, len(rows))
	for _, row := range rows {
		result = append(result, toCoachingSessionVO(row))
	}
	return result, nil
}

func (s *Server) dashboardActiveMockInterviews(userID string) ([]vo.MockInterviewVO, error) {
	var rows []MockInterview
	if err := s.db.Where("user_id = ? AND status IN ?", userID, activeMockInterviewStatuses()).
		Order("updated_at desc, created_at desc").
		Limit(workbenchDashboardRecentLimit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]vo.MockInterviewVO, 0, len(rows))
	for _, row := range rows {
		result = append(result, toMockInterviewVO(row))
	}
	return result, nil
}

func (s *Server) dashboardPracticeStateSummary(userID string) (vo.PracticeStateSummaryVO, error) {
	var states []PracticeState
	if err := s.db.Where("user_id = ?", userID).Find(&states).Error; err != nil {
		return vo.PracticeStateSummaryVO{}, err
	}

	summary := vo.PracticeStateSummaryVO{TotalStates: len(states)}
	if len(states) == 0 {
		return summary, nil
	}

	var totalMastery int
	recentSince := time.Now().Unix() - workbenchDashboardRecentWindowSeconds
	for _, state := range states {
		totalMastery += state.MasteryScore
		if state.MasteryScore <= workbenchDashboardWeakMasteryThreshold {
			summary.WeakStateCount++
		}
		if state.LastPracticedAt >= recentSince {
			summary.RecentAttemptCount += state.AttemptCount
		}
	}
	summary.AverageMasteryScore = totalMastery / len(states)
	return summary, nil
}

func activeMockInterviewStatuses() []string {
	return []string{
		MockInterviewStatusCreated,
		MockInterviewStatusInProgress,
		MockInterviewStatusWaitingAnswer,
		MockInterviewStatusEvaluating,
		MockInterviewStatusAskingFollowup,
		MockInterviewStatusSwitchingTopic,
	}
}
