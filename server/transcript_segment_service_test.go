package server

import (
	"strings"
	"testing"
)

func TestSplitTranscriptIntoSegmentsLongText(t *testing.T) {
	transcript := InterviewTranscript{
		TranscriptID: "transcript-1",
		InterviewID:  "interview-1",
		UserID:       "user-1",
		Content:      buildLongTranscriptContent(),
	}

	segments := splitTranscriptIntoSegments(transcript)
	if len(segments) < 2 {
		t.Fatalf("segments length = %d, want at least 2", len(segments))
	}

	previousEnd := 0
	for i, segment := range segments {
		wantSequence := i + 1
		if segment.Sequence != wantSequence {
			t.Fatalf("segment %d sequence = %d, want %d", i, segment.Sequence, wantSequence)
		}
		if segment.StartOffset < 0 || segment.EndOffset <= segment.StartOffset {
			t.Fatalf("segment %d offsets invalid: start=%d end=%d", i, segment.StartOffset, segment.EndOffset)
		}
		if i > 0 && segment.StartOffset < previousEnd-segmentOverlapChars {
			t.Fatalf("segment %d start_offset = %d, previous end = %d", i, segment.StartOffset, previousEnd)
		}
		if strings.TrimSpace(segment.Content) == "" {
			t.Fatalf("segment %d content is empty", i)
		}
		if segment.CharCount != len([]rune(segment.Content)) {
			t.Fatalf("segment %d char_count = %d, want %d", i, segment.CharCount, len([]rune(segment.Content)))
		}
		previousEnd = segment.EndOffset
	}
}

func TestShouldUseSegmentedReview(t *testing.T) {
	shortContent := strings.Repeat("短文本", 100)
	if shouldUseSegmentedReview(shortContent) {
		t.Fatalf("short transcript triggered segmented review")
	}

	longContent := strings.Repeat("长文本", longTranscriptThresholdChars)
	if !shouldUseSegmentedReview(longContent) {
		t.Fatalf("long transcript did not trigger segmented review")
	}
}

func TestBuildSegmentReviewPromptIncludesOutputLimits(t *testing.T) {
	prompt := buildSegmentReviewPrompt(
		InterviewSession{CompanyName: "Acme", JobTitle: "Backend Engineer", InterviewRound: "first_round", InterviewType: "technical"},
		InterviewTranscript{Language: "zh"},
		TranscriptSegment{Sequence: 1, Content: "面试官：请介绍项目。候选人：我做了 Agent 项目。"},
	)

	for _, want := range []string{
		"question_candidates must contain at most 5 items",
		"Every evidence_text field must be 200 Chinese characters or fewer",
		"segment_summary must be 300 Chinese characters or fewer",
		"Do not return Markdown",
		"Do not wrap the JSON in code fences",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func buildLongTranscriptContent() string {
	var builder strings.Builder
	for i := 0; i < 90; i++ {
		builder.WriteString("面试官：请解释你在 Agent 项目里的服务编排、数据库事务、异步任务和错误处理设计。\n")
		builder.WriteString("候选人：我会先描述 interview transcript、review report、memory candidate、coaching plan 和 mock interview 的边界，然后说明实现细节、取舍、测试覆盖和失败处理。\n")
	}
	return builder.String()
}
