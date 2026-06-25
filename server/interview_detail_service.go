package server

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"agent-web-base/vo"
)

func (s *Server) GetInterviewDetail(interviewID string, userID string) (vo.InterviewDetailVO, error) {
	interviewID = strings.TrimSpace(interviewID)
	if interviewID == "" {
		return vo.InterviewDetailVO{}, fmt.Errorf("interview_id is required")
	}

	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return vo.InterviewDetailVO{}, err
	}
	if userID != "" && session.UserID != userID {
		return vo.InterviewDetailVO{}, fmt.Errorf("interview user_id mismatch")
	}

	transcript, err := s.interviewDetailTranscript(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}
	mediaFiles, err := s.ListInterviewMedia(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}
	transcriptionJobs, err := s.interviewDetailTranscriptionJobs(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}
	reviewReport, err := s.interviewDetailReview(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}
	questions, err := s.ListInterviewQuestions(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}
	memoryCandidates, err := s.ListMemoryCandidates(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}
	coachingPlan, coachingTasks, err := s.interviewDetailCoaching(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}
	latestMock, err := s.interviewDetailLatestMock(interviewID)
	if err != nil {
		return vo.InterviewDetailVO{}, err
	}

	return vo.InterviewDetailVO{
		Interview:           toInterviewSessionVO(session),
		Transcript:          transcript,
		MediaFiles:          mediaFiles,
		TranscriptionJobs:   transcriptionJobs,
		ReviewReport:        reviewReport,
		Questions:           questions,
		MemoryCandidates:    memoryCandidates,
		CoachingPlan:        coachingPlan,
		CoachingTasks:       coachingTasks,
		LatestMockInterview: latestMock,
	}, nil
}

func (s *Server) interviewDetailTranscript(interviewID string) (*vo.InterviewTranscriptVO, error) {
	var transcript InterviewTranscript
	if err := s.db.First(&transcript, "interview_id = ?", interviewID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	result := toInterviewTranscriptVO(transcript)
	return &result, nil
}

func (s *Server) interviewDetailTranscriptionJobs(interviewID string) ([]vo.TranscriptionJobVO, error) {
	var jobs []TranscriptionJob
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("created_at desc").
		Find(&jobs).Error; err != nil {
		return nil, err
	}

	result := make([]vo.TranscriptionJobVO, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, toTranscriptionJobVO(job))
	}
	return result, nil
}

func (s *Server) interviewDetailReview(interviewID string) (*vo.InterviewReviewReportVO, error) {
	var report InterviewReviewReport
	if err := s.db.First(&report, "interview_id = ?", interviewID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	result := toInterviewReviewReportVO(report)
	return &result, nil
}

func (s *Server) interviewDetailCoaching(interviewID string) (*vo.CoachingPlanVO, []vo.CoachingTaskVO, error) {
	var plan CoachingPlan
	if err := s.db.First(&plan, "interview_id = ?", interviewID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, []vo.CoachingTaskVO{}, nil
		}
		return nil, nil, err
	}

	tasks, err := s.ListCoachingTasks(plan.PlanID)
	if err != nil {
		return nil, nil, err
	}
	result := toCoachingPlanVO(plan)
	return &result, tasks, nil
}

func (s *Server) interviewDetailLatestMock(interviewID string) (*vo.MockInterviewVO, error) {
	var mock MockInterview
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("created_at desc").
		First(&mock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	result := toMockInterviewVO(mock)
	return &result, nil
}
