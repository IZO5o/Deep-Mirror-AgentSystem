package server

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-web-base/agent"
	"agent-web-base/vo"
)

const (
	longTranscriptThresholdChars = 12000
	segmentMaxChars              = 4000
	segmentOverlapChars          = 200

	TranscriptSegmentStatusPending   = "pending"
	TranscriptSegmentStatusExtracted = "extracted"
	TranscriptSegmentStatusFailed    = "failed"
)

type segmentReviewOutput struct {
	SegmentSummary     string                     `json:"segment_summary"`
	SpeakerRoleNotes   []segmentSpeakerRoleNote   `json:"speaker_role_notes"`
	QuestionCandidates []segmentQuestionCandidate `json:"question_candidates"`
	KeyEvidence        []string                   `json:"key_evidence"`
	UncertainParts     []string                   `json:"uncertain_parts"`
}

type segmentSpeakerRoleNote struct {
	SpeakerLabel   string `json:"speaker_label"`
	NormalizedRole string `json:"normalized_role"`
	Reason         string `json:"reason"`
	EvidenceText   string `json:"evidence_text"`
}

type segmentQuestionCandidate struct {
	LocalSequence         int      `json:"local_sequence"`
	Question              string   `json:"question"`
	Answer                string   `json:"answer"`
	TopicTags             []string `json:"topic_tags"`
	Difficulty            string   `json:"difficulty"`
	AnswerQuality         string   `json:"answer_quality"`
	WeaknessSummary       string   `json:"weakness_summary"`
	ImprovementSuggestion string   `json:"improvement_suggestion"`
	EvidenceText          string   `json:"evidence_text"`
}

func shouldUseSegmentedReview(content string) bool {
	return len([]rune(content)) > longTranscriptThresholdChars
}

func splitTranscriptIntoSegments(transcript InterviewTranscript) []TranscriptSegment {
	contentRunes := []rune(transcript.Content)
	if len(contentRunes) == 0 {
		return []TranscriptSegment{}
	}

	boundaries := transcriptSemanticBoundaries(contentRunes)
	now := time.Now().Unix()
	segments := make([]TranscriptSegment, 0, len(contentRunes)/segmentMaxChars+1)

	start := 0
	sequence := 1
	for start < len(contentRunes) {
		targetEnd := start + segmentMaxChars
		if targetEnd >= len(contentRunes) {
			targetEnd = len(contentRunes)
		} else if boundary := bestBoundaryBefore(boundaries, start, targetEnd); boundary > start {
			targetEnd = boundary
		}
		if targetEnd <= start {
			targetEnd = min(start+segmentMaxChars, len(contentRunes))
		}

		content := strings.TrimSpace(string(contentRunes[start:targetEnd]))
		if content != "" {
			segments = append(segments, TranscriptSegment{
				SegmentID:    uuid.New().String(),
				InterviewID:  transcript.InterviewID,
				TranscriptID: transcript.TranscriptID,
				UserID:       transcript.UserID,
				Sequence:     sequence,
				StartOffset:  start,
				EndOffset:    targetEnd,
				Content:      content,
				CharCount:    len([]rune(content)),
				Status:       TranscriptSegmentStatusPending,
				CreatedAt:    now,
				UpdatedAt:    now,
			})
			sequence++
		}

		if targetEnd >= len(contentRunes) {
			break
		}
		nextStart := targetEnd
		if targetEnd-start >= segmentMaxChars && segmentOverlapChars > 0 {
			nextStart = max(targetEnd-segmentOverlapChars, start+1)
		}
		start = nextStart
	}

	return segments
}

func transcriptSemanticBoundaries(contentRunes []rune) []int {
	content := string(contentRunes)
	pattern := regexp.MustCompile(`(?m)(\n+|面试官[:：]|候选人[:：]|Interviewer:|Candidate:|\[[0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?\]|[0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?)`)
	matches := pattern.FindAllStringIndex(content, -1)
	boundaries := make([]int, 0, len(matches))
	for _, match := range matches {
		if match[0] <= 0 {
			continue
		}
		boundaries = append(boundaries, len([]rune(content[:match[0]])))
	}
	sort.Ints(boundaries)
	return boundaries
}

func bestBoundaryBefore(boundaries []int, start int, targetEnd int) int {
	minBoundary := start + segmentMaxChars/2
	best := 0
	for _, boundary := range boundaries {
		if boundary <= minBoundary {
			continue
		}
		if boundary > targetEnd {
			break
		}
		best = boundary
	}
	return best
}

func buildSegmentReviewPrompt(session InterviewSession, transcript InterviewTranscript, segment TranscriptSegment) string {
	return fmt.Sprintf(`Analyze this transcript segment and return STRICT JSON only.

Do not return Markdown.
Do not wrap the JSON in code fences.
Do not include explanations outside the JSON object.
Do not invent content that is not supported by this segment.
Do not output oversized JSON just to cover every detail.
Be conservative about speaker roles. Only mark interviewer/candidate when explicit labels such as "面试官", "候选人", "Interviewer", or "Candidate" are present. If role evidence is unclear, use "unknown".

Output limits:
- segment_summary must be 300 Chinese characters or fewer.
- question_candidates must contain at most 5 items.
- key_evidence must contain at most 5 items.
- uncertain_parts must contain at most 5 items.
- speaker_role_notes should keep only the most important speaker labels and avoid long explanations.
- Every evidence_text field must be 200 Chinese characters or fewer.

JSON schema:
{
  "segment_summary": "string",
  "speaker_role_notes": [
    {
      "speaker_label": "string",
      "normalized_role": "interviewer|candidate|unknown",
      "reason": "string",
      "evidence_text": "string"
    }
  ],
  "question_candidates": [
    {
      "local_sequence": 1,
      "question": "string",
      "answer": "string",
      "topic_tags": ["string"],
      "difficulty": "easy|medium|hard|unknown",
      "answer_quality": "good|medium|weak|unknown",
      "weakness_summary": "string",
      "improvement_suggestion": "string",
      "evidence_text": "string"
    }
  ],
  "key_evidence": ["string"],
  "uncertain_parts": ["string"]
}

Use empty strings or empty arrays when information is missing.

Interview metadata:
- company_name: %s
- job_title: %s
- interview_round: %s
- interview_type: %s
- language: %s

Transcript segment:
- sequence: %d
- start_offset: %d
- end_offset: %d

Segment content:
%s`, session.CompanyName, session.JobTitle, session.InterviewRound, session.InterviewType, transcript.Language, segment.Sequence, segment.StartOffset, segment.EndOffset, segment.Content)
}

func buildSegmentReviewRetryPrompt(session InterviewSession, transcript InterviewTranscript, segment TranscriptSegment, previousOutput string, previousErr error) string {
	previousError := ""
	if previousErr != nil {
		previousError = previousErr.Error()
	}
	return fmt.Sprintf(`Retry segment extraction with a compact STRICT JSON response only.

The previous response could not be parsed as JSON.
Previous parse error: %s
Previous output preview:
%s

Rules for this retry:
- Return one valid JSON object only.
- Do not return Markdown.
- Do not wrap JSON in code fences.
- Do not include explanations outside JSON.
- Keep the response compact.
- segment_summary must be 180 Chinese characters or fewer.
- question_candidates must contain at most 3 items.
- key_evidence must contain at most 3 items.
- uncertain_parts must contain at most 3 items.
- speaker_role_notes should include only essential speaker labels.
- Every evidence_text field must be 120 Chinese characters or fewer.
- Use empty strings or empty arrays when information is missing.

Use this exact JSON schema:
{
  "segment_summary": "string",
  "speaker_role_notes": [
    {
      "speaker_label": "string",
      "normalized_role": "interviewer|candidate|unknown",
      "reason": "string",
      "evidence_text": "string"
    }
  ],
  "question_candidates": [
    {
      "local_sequence": 1,
      "question": "string",
      "answer": "string",
      "topic_tags": ["string"],
      "difficulty": "easy|medium|hard|unknown",
      "answer_quality": "good|medium|weak|unknown",
      "weakness_summary": "string",
      "improvement_suggestion": "string",
      "evidence_text": "string"
    }
  ],
  "key_evidence": ["string"],
  "uncertain_parts": ["string"]
}

Interview metadata:
- company_name: %s
- job_title: %s
- interview_round: %s
- interview_type: %s
- language: %s

Transcript segment:
- sequence: %d
- start_offset: %d
- end_offset: %d

Segment content:
%s`, previousError, clipRunes(previousOutput, 800), session.CompanyName, session.JobTitle, session.InterviewRound, session.InterviewType, transcript.Language, segment.Sequence, segment.StartOffset, segment.EndOffset, segment.Content)
}

func (s *Server) extractSegmentReview(ctx context.Context, runner agent.Runner, session InterviewSession, transcript InterviewTranscript, segment TranscriptSegment) (TranscriptSegment, error) {
	result, runErr := runner.RunTask(ctx, buildSegmentReviewPrompt(session, transcript, segment))
	if runErr != nil {
		_ = s.updateTranscriptSegmentFailed(segment.SegmentID, result.Response, runErr)
		return TranscriptSegment{}, fmt.Errorf("segment %d review agent failed: %w", segment.Sequence, runErr)
	}

	parsed, parseErr := parseSegmentReviewOutput(result.Response)
	if parseErr != nil {
		retryResult, retryRunErr := runner.RunTask(ctx, buildSegmentReviewRetryPrompt(session, transcript, segment, result.Response, parseErr))
		if retryRunErr != nil {
			_ = s.updateTranscriptSegmentFailed(segment.SegmentID, retryResult.Response, retryRunErr)
			return TranscriptSegment{}, fmt.Errorf("segment %d retry review agent failed after parse error: %w", segment.Sequence, retryRunErr)
		}
		retryParsed, retryParseErr := parseSegmentReviewOutput(retryResult.Response)
		if retryParseErr != nil {
			_ = s.updateTranscriptSegmentFailed(segment.SegmentID, retryResult.Response, retryParseErr)
			return TranscriptSegment{}, fmt.Errorf("parse segment %d review output failed after retry: %w", segment.Sequence, retryParseErr)
		}
		result = retryResult
		parsed = retryParsed
	}

	now := time.Now().Unix()
	updates := map[string]any{
		"summary":             parsed.SegmentSummary,
		"speaker_role_notes":  marshalJSON(parsed.SpeakerRoleNotes),
		"question_candidates": marshalJSON(parsed.QuestionCandidates),
		"key_evidence":        marshalStringSlice(parsed.KeyEvidence),
		"uncertain_parts":     marshalStringSlice(parsed.UncertainParts),
		"raw_agent_output":    result.Response,
		"status":              TranscriptSegmentStatusExtracted,
		"error_message":       "",
		"updated_at":          now,
	}
	if err := s.db.Model(&TranscriptSegment{}).
		Where("segment_id = ?", segment.SegmentID).
		Updates(updates).Error; err != nil {
		return TranscriptSegment{}, err
	}

	var saved TranscriptSegment
	if err := s.db.First(&saved, "segment_id = ?", segment.SegmentID).Error; err != nil {
		return TranscriptSegment{}, err
	}
	return saved, nil
}

func parseSegmentReviewOutput(raw string) (segmentReviewOutput, error) {
	cleaned := stripJSONFence(strings.TrimSpace(raw))
	var parsed segmentReviewOutput
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return segmentReviewOutput{}, fmt.Errorf("parse segment review JSON: %w", err)
	}
	if parsed.SpeakerRoleNotes == nil {
		parsed.SpeakerRoleNotes = []segmentSpeakerRoleNote{}
	}
	if parsed.QuestionCandidates == nil {
		parsed.QuestionCandidates = []segmentQuestionCandidate{}
	}
	if parsed.KeyEvidence == nil {
		parsed.KeyEvidence = []string{}
	}
	if parsed.UncertainParts == nil {
		parsed.UncertainParts = []string{}
	}
	return parsed, nil
}

func (s *Server) updateTranscriptSegmentFailed(segmentID string, rawOutput string, cause error) error {
	now := time.Now().Unix()
	errorMessage := ""
	if cause != nil {
		errorMessage = cause.Error()
	}
	return s.db.Model(&TranscriptSegment{}).
		Where("segment_id = ?", segmentID).
		Updates(map[string]any{
			"raw_agent_output": rawOutput,
			"status":           TranscriptSegmentStatusFailed,
			"error_message":    errorMessage,
			"updated_at":       now,
		}).Error
}

func (s *Server) ListTranscriptSegments(interviewID string) ([]vo.TranscriptSegmentVO, error) {
	var segments []TranscriptSegment
	if err := s.db.Where("interview_id = ?", interviewID).
		Order("sequence asc").
		Find(&segments).Error; err != nil {
		return nil, err
	}

	result := make([]vo.TranscriptSegmentVO, 0, len(segments))
	for _, segment := range segments {
		result = append(result, toTranscriptSegmentVO(segment))
	}
	return result, nil
}

func toTranscriptSegmentVO(segment TranscriptSegment) vo.TranscriptSegmentVO {
	return vo.TranscriptSegmentVO{
		SegmentID:          segment.SegmentID,
		InterviewID:        segment.InterviewID,
		TranscriptID:       segment.TranscriptID,
		UserID:             segment.UserID,
		Sequence:           segment.Sequence,
		StartOffset:        segment.StartOffset,
		EndOffset:          segment.EndOffset,
		CharCount:          segment.CharCount,
		ContentPreview:     clipRunes(segment.Content, 300),
		Summary:            segment.Summary,
		SpeakerRoleNotes:   unmarshalJSONAny(segment.SpeakerRoleNotes),
		QuestionCandidates: unmarshalJSONAny(segment.QuestionCandidates),
		KeyEvidence:        unmarshalJSONAny(segment.KeyEvidence),
		UncertainParts:     unmarshalJSONAny(segment.UncertainParts),
		Status:             segment.Status,
		ErrorMessage:       segment.ErrorMessage,
		CreatedAt:          segment.CreatedAt,
		UpdatedAt:          segment.UpdatedAt,
	}
}

func buildFinalSegmentedReviewPrompt(session InterviewSession, transcript InterviewTranscript, segments []TranscriptSegment) string {
	type finalSegmentInput struct {
		Sequence           int    `json:"sequence"`
		StartOffset        int    `json:"start_offset"`
		EndOffset          int    `json:"end_offset"`
		Summary            string `json:"summary"`
		SpeakerRoleNotes   any    `json:"speaker_role_notes"`
		QuestionCandidates any    `json:"question_candidates"`
		KeyEvidence        any    `json:"key_evidence"`
		UncertainParts     any    `json:"uncertain_parts"`
	}

	inputs := make([]finalSegmentInput, 0, len(segments))
	for _, segment := range segments {
		inputs = append(inputs, finalSegmentInput{
			Sequence:           segment.Sequence,
			StartOffset:        segment.StartOffset,
			EndOffset:          segment.EndOffset,
			Summary:            segment.Summary,
			SpeakerRoleNotes:   unmarshalJSONAny(segment.SpeakerRoleNotes),
			QuestionCandidates: unmarshalJSONAny(segment.QuestionCandidates),
			KeyEvidence:        unmarshalJSONAny(segment.KeyEvidence),
			UncertainParts:     unmarshalJSONAny(segment.UncertainParts),
		})
	}

	return fmt.Sprintf(`Merge segment-level interview review results and return STRICT JSON only.

Do not return Markdown.
Do not wrap the JSON in code fences.
Do not include explanations outside the JSON object.
Do not invent interview content that is not supported by segment summaries, question candidates, or evidence.
Deduplicate similar questions across segments.
Renumber final questions from 1.
Keep evidence_text concise and use the most relevant evidence snippet.

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
- transcript_id: %s
- transcript_char_count: %d

Segment extraction results JSON:
%s`, session.CompanyName, session.JobTitle, session.InterviewRound, session.InterviewType, transcript.Language, transcript.TranscriptID, len([]rune(transcript.Content)), marshalJSON(inputs))
}

func (s *Server) runSegmentedInterviewReview(ctx context.Context, session InterviewSession, transcript InterviewTranscript, runner agent.Runner) (vo.InterviewReviewReportVO, error) {
	segments := splitTranscriptIntoSegments(transcript)
	if len(segments) == 0 {
		return vo.InterviewReviewReportVO{}, fmt.Errorf("transcript has no segment content")
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("interview_id = ?", transcript.InterviewID).Delete(&TranscriptSegment{}).Error; err != nil {
			return err
		}
		for _, segment := range segments {
			if err := tx.Create(&segment).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return vo.InterviewReviewReportVO{}, err
	}

	extractedSegments := make([]TranscriptSegment, 0, len(segments))
	for _, segment := range segments {
		extracted, err := s.extractSegmentReview(ctx, runner, session, transcript, segment)
		if err != nil {
			report, saveErr := s.upsertFailedInterviewReview(transcript.InterviewID, session.UserID, "", err)
			if saveErr != nil {
				return vo.InterviewReviewReportVO{}, fmt.Errorf("segmented review failed: %v; save failed report: %w", err, saveErr)
			}
			return report, err
		}
		extractedSegments = append(extractedSegments, extracted)
	}

	finalPrompt := buildFinalSegmentedReviewPrompt(session, transcript, extractedSegments)
	result, runErr := runner.RunTask(ctx, finalPrompt)
	if runErr != nil {
		report, saveErr := s.upsertFailedInterviewReview(transcript.InterviewID, session.UserID, result.Response, runErr)
		if saveErr != nil {
			return vo.InterviewReviewReportVO{}, fmt.Errorf("final segmented review failed: %v; save failed report: %w", runErr, saveErr)
		}
		return report, fmt.Errorf("final segmented review failed: %w", runErr)
	}

	parsed, parseErr := parseReviewAgentOutput(result.Response)
	if parseErr != nil {
		report, saveErr := s.upsertFailedInterviewReview(transcript.InterviewID, session.UserID, result.Response, parseErr)
		if saveErr != nil {
			return vo.InterviewReviewReportVO{}, fmt.Errorf("parse final segmented review output failed: %v; save failed report: %w", parseErr, saveErr)
		}
		return report, parseErr
	}

	return s.saveSuccessfulInterviewReview(transcript.InterviewID, session.UserID, result.Response, parsed)
}

func marshalJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func unmarshalJSONAny(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return []any{}
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	return value
}

func clipRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if limit <= 0 || len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
