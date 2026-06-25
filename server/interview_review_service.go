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
	InterviewStatusReviewed = "reviewed"

	InterviewReviewStatusGenerated = "generated"
	InterviewReviewStatusFailed    = "failed"
)

type reviewAgentOutput struct {
	OverallSummary       string                 `json:"overall_summary"`
	Strengths            []string               `json:"strengths"`
	Weaknesses           []string               `json:"weaknesses"`
	FollowUpRisks        []string               `json:"follow_up_risks"`
	SuggestedPreparation []string               `json:"suggested_preparation"`
	Questions            []reviewQuestionOutput `json:"questions"`
}

type reviewQuestionOutput struct {
	Sequence              int      `json:"sequence"`
	Question              string   `json:"question"`
	Answer                string   `json:"answer"`
	TopicTags             []string `json:"topic_tags"`
	Difficulty            string   `json:"difficulty"`
	AnswerQuality         string   `json:"answer_quality"`
	WeaknessSummary       string   `json:"weakness_summary"`
	ImprovementSuggestion string   `json:"improvement_suggestion"`
	EvidenceText          string   `json:"evidence_text"`
}

func (s *Server) TriggerInterviewReview(ctx context.Context, interviewID string) (vo.InterviewReviewReportVO, error) {
	if s.agents == nil {
		return vo.InterviewReviewReportVO{}, fmt.Errorf("agent provider is nil")
	}

	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return vo.InterviewReviewReportVO{}, err
	}

	var transcript InterviewTranscript
	if err := s.db.First(&transcript, "interview_id = ?", interviewID).Error; err != nil {
		return vo.InterviewReviewReportVO{}, fmt.Errorf("interview transcript not found: %w", err)
	}

	_, runner, err := s.agents.Get(string(agent.AgentTypeReview))
	if err != nil {
		return vo.InterviewReviewReportVO{}, err
	}

	if shouldUseSegmentedReview(transcript.Content) {
		return s.runSegmentedInterviewReview(ctx, session, transcript, runner)
	}

	result, runErr := runner.RunTask(ctx, buildInterviewReviewPrompt(session, transcript))
	if runErr != nil {
		report, saveErr := s.upsertFailedInterviewReview(interviewID, session.UserID, result.Response, runErr)
		if saveErr != nil {
			return vo.InterviewReviewReportVO{}, fmt.Errorf("review agent failed: %v; save failed report: %w", runErr, saveErr)
		}
		return report, fmt.Errorf("review agent failed: %w", runErr)
	}

	parsed, parseErr := parseReviewAgentOutput(result.Response)
	if parseErr != nil {
		report, saveErr := s.upsertFailedInterviewReview(interviewID, session.UserID, result.Response, parseErr)
		if saveErr != nil {
			return vo.InterviewReviewReportVO{}, fmt.Errorf("parse review output failed: %v; save failed report: %w", parseErr, saveErr)
		}
		return report, parseErr
	}

	report, err := s.saveSuccessfulInterviewReview(interviewID, session.UserID, result.Response, parsed)
	if err != nil {
		return vo.InterviewReviewReportVO{}, err
	}
	return report, nil
}

func (s *Server) GetInterviewReview(interviewID string) (vo.InterviewReviewReportVO, error) {
	var report InterviewReviewReport
	if err := s.db.First(&report, "interview_id = ?", interviewID).Error; err != nil {
		return vo.InterviewReviewReportVO{}, err
	}
	return toInterviewReviewReportVO(report), nil
}

func (s *Server) ListInterviewQuestions(interviewID string) ([]vo.InterviewQuestionVO, error) {
	var questions []InterviewQuestion
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("sequence asc").
		Find(&questions).Error; err != nil {
		return nil, err
	}

	result := make([]vo.InterviewQuestionVO, 0, len(questions))
	for _, question := range questions {
		result = append(result, toInterviewQuestionVO(question))
	}
	return result, nil
}

func (s *Server) saveSuccessfulInterviewReview(interviewID string, userID string, rawOutput string, parsed reviewAgentOutput) (vo.InterviewReviewReportVO, error) {
	now := time.Now().Unix()
	var savedReport InterviewReviewReport
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("interview_id = ?", interviewID).Delete(&InterviewQuestion{}).Error; err != nil {
			return err
		}

		for i, q := range parsed.Questions {
			sequence := q.Sequence
			if sequence <= 0 {
				sequence = i + 1
			}
			question := InterviewQuestion{
				QuestionID:            uuid.New().String(),
				InterviewID:           interviewID,
				UserID:                userID,
				Sequence:              sequence,
				Question:              q.Question,
				Answer:                q.Answer,
				TopicTags:             marshalStringSlice(q.TopicTags),
				Difficulty:            normalizeDefault(q.Difficulty, "unknown"),
				AnswerQuality:         normalizeDefault(q.AnswerQuality, "unknown"),
				WeaknessSummary:       q.WeaknessSummary,
				ImprovementSuggestion: q.ImprovementSuggestion,
				EvidenceText:          q.EvidenceText,
				CreatedAt:             now,
				UpdatedAt:             now,
			}
			if err := tx.Create(&question).Error; err != nil {
				return err
			}
		}

		report, err := upsertInterviewReviewReport(tx, interviewID, userID, now, func(report *InterviewReviewReport) {
			report.OverallSummary = parsed.OverallSummary
			report.Strengths = marshalStringSlice(parsed.Strengths)
			report.Weaknesses = marshalStringSlice(parsed.Weaknesses)
			report.FollowUpRisks = marshalStringSlice(parsed.FollowUpRisks)
			report.SuggestedPreparation = marshalStringSlice(parsed.SuggestedPreparation)
			report.RawAgentOutput = rawOutput
			report.Status = InterviewReviewStatusGenerated
		})
		if err != nil {
			return err
		}
		savedReport = report

		return tx.Model(&InterviewSession{}).
			Where("interview_id = ?", interviewID).
			Updates(map[string]any{
				"status":     InterviewStatusReviewed,
				"updated_at": now,
			}).Error
	})
	if err != nil {
		return vo.InterviewReviewReportVO{}, err
	}
	return toInterviewReviewReportVO(savedReport), nil
}

func (s *Server) upsertFailedInterviewReview(interviewID string, userID string, rawOutput string, cause error) (vo.InterviewReviewReportVO, error) {
	now := time.Now().Unix()
	var savedReport InterviewReviewReport
	err := s.db.Transaction(func(tx *gorm.DB) error {
		report, err := upsertInterviewReviewReport(tx, interviewID, userID, now, func(report *InterviewReviewReport) {
			report.RawAgentOutput = rawOutput
			report.Status = InterviewReviewStatusFailed
			if cause != nil {
				report.OverallSummary = cause.Error()
			}
		})
		if err != nil {
			return err
		}
		savedReport = report
		return nil
	})
	if err != nil {
		return vo.InterviewReviewReportVO{}, err
	}
	return toInterviewReviewReportVO(savedReport), nil
}

func upsertInterviewReviewReport(tx *gorm.DB, interviewID string, userID string, now int64, mutate func(*InterviewReviewReport)) (InterviewReviewReport, error) {
	var report InterviewReviewReport
	err := tx.First(&report, "interview_id = ?", interviewID).Error
	switch {
	case err == nil:
		report.UserID = userID
		report.UpdatedAt = now
		mutate(&report)
		if err := tx.Save(&report).Error; err != nil {
			return InterviewReviewReport{}, err
		}
		return report, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		report = InterviewReviewReport{
			ReportID:    uuid.New().String(),
			InterviewID: interviewID,
			UserID:      userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		mutate(&report)
		if err := tx.Create(&report).Error; err != nil {
			return InterviewReviewReport{}, err
		}
		return report, nil
	default:
		return InterviewReviewReport{}, err
	}
}

func buildInterviewReviewPrompt(session InterviewSession, transcript InterviewTranscript) string {
	return fmt.Sprintf(`Analyze the following interview transcript and return STRICT JSON only.

Do not return Markdown.
Do not wrap the JSON in code fences.
Do not include explanations outside the JSON object.

JSON schema:
{
  "overall_summary": "string",
  "strengths": ["string"],
  "weaknesses": ["string"],
  "follow_up_risks": ["string"],
  "suggested_preparation": ["string"],
  "questions": [
    {
      "sequence": 1,
      "question": "string",
      "answer": "string",
      "topic_tags": ["string"],
      "difficulty": "easy|medium|hard|unknown",
      "answer_quality": "good|medium|weak|unknown",
      "weakness_summary": "string",
      "improvement_suggestion": "string",
      "evidence_text": "string"
    }
  ]
}

Use empty strings or empty arrays when information is missing.

Interview metadata:
- company_name: %s
- job_title: %s
- interview_round: %s
- interview_type: %s
- language: %s

Transcript:
%s`, session.CompanyName, session.JobTitle, session.InterviewRound, session.InterviewType, transcript.Language, transcript.Content)
}

func parseReviewAgentOutput(raw string) (reviewAgentOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed reviewAgentOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return reviewAgentOutput{}, fmt.Errorf("parse review agent JSON: %w", err)
	}
	if parsed.Questions == nil {
		parsed.Questions = []reviewQuestionOutput{}
	}
	return parsed, nil
}

func stripJSONFence(s string) string {
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	return s
}

func marshalStringSlice(values []string) string {
	if values == nil {
		values = []string{}
	}
	data, _ := json.Marshal(values)
	return string(data)
}

func unmarshalStringSlice(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}

func normalizeDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func toInterviewQuestionVO(question InterviewQuestion) vo.InterviewQuestionVO {
	return vo.InterviewQuestionVO{
		QuestionID:            question.QuestionID,
		InterviewID:           question.InterviewID,
		UserID:                question.UserID,
		Sequence:              question.Sequence,
		Question:              question.Question,
		Answer:                question.Answer,
		TopicTags:             unmarshalStringSlice(question.TopicTags),
		Difficulty:            question.Difficulty,
		AnswerQuality:         question.AnswerQuality,
		WeaknessSummary:       question.WeaknessSummary,
		ImprovementSuggestion: question.ImprovementSuggestion,
		EvidenceText:          question.EvidenceText,
		CreatedAt:             question.CreatedAt,
		UpdatedAt:             question.UpdatedAt,
	}
}

func toInterviewReviewReportVO(report InterviewReviewReport) vo.InterviewReviewReportVO {
	return vo.InterviewReviewReportVO{
		ReportID:             report.ReportID,
		InterviewID:          report.InterviewID,
		UserID:               report.UserID,
		OverallSummary:       report.OverallSummary,
		Strengths:            unmarshalStringSlice(report.Strengths),
		Weaknesses:           unmarshalStringSlice(report.Weaknesses),
		FollowUpRisks:        unmarshalStringSlice(report.FollowUpRisks),
		SuggestedPreparation: unmarshalStringSlice(report.SuggestedPreparation),
		RawAgentOutput:       report.RawAgentOutput,
		Status:               report.Status,
		CreatedAt:            report.CreatedAt,
		UpdatedAt:            report.UpdatedAt,
	}
}
