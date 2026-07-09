package server

import (
	"fmt"
	"time"
)

const (
	TimePressureStyleNone     = "none"
	TimePressureStyleModerate = "moderate"
	TimePressureStyleStrict   = "strict"
	defaultWarnAtSeconds      = 300
)

type TimerResult struct {
	Active           bool   `json:"active"`
	TotalSeconds     int    `json:"total_seconds"`
	ElapsedSeconds   int    `json:"elapsed_seconds"`
	RemainingSeconds int    `json:"remaining_seconds"`
	WarnTriggered    bool   `json:"warn_triggered"`
	Expired          bool   `json:"expired"`
	PressureStyle    string `json:"pressure_style"`
	StartedAt        int64  `json:"started_at"`
}

func checkMockTimer(mock MockInterview, turns []MockTurn, now time.Time) TimerResult {
	question := latestTimedMockQuestion(turns)
	if question == nil || question.TimeLimitSeconds <= 0 {
		return TimerResult{Active: false}
	}
	warnAt := question.WarnAtSeconds
	if warnAt <= 0 {
		warnAt = defaultWarnAtSeconds
	}
	elapsed := int(now.Unix() - question.CreatedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	remaining := question.TimeLimitSeconds - elapsed
	return TimerResult{
		Active:           true,
		TotalSeconds:     question.TimeLimitSeconds,
		ElapsedSeconds:   elapsed,
		RemainingSeconds: remaining,
		WarnTriggered:    elapsed >= warnAt,
		Expired:          elapsed >= question.TimeLimitSeconds,
		PressureStyle:    normalizeTimePressureStyle(question.TimePressureStyle),
		StartedAt:        question.CreatedAt,
	}
}

func latestTimedMockQuestion(turns []MockTurn) *MockTurn {
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if turn.Role != mockTurnRoleAssistant {
			continue
		}
		if turn.TurnType == mockTurnTypeOpeningQuestion || turn.TurnType == mockTurnTypeFollowupQuestion || turn.TurnType == mockTurnTypeTopicSwitch {
			return &turn
		}
	}
	return nil
}

func normalizeTimePressureStyle(style string) string {
	switch style {
	case TimePressureStyleModerate, TimePressureStyleStrict:
		return style
	default:
		return TimePressureStyleNone
	}
}

func formatMockTimerPromptSection(result TimerResult) string {
	if !result.Active {
		return "=== 时间状态 ===\n本题不限时。"
	}
	remaining := result.RemainingSeconds
	if remaining < 0 {
		remaining = 0
	}
	section := fmt.Sprintf("=== 时间状态 ===\n本题限时 %s，已用 %s，剩余约 %s，压力模式：%s。",
		formatDurationCN(result.TotalSeconds),
		formatDurationCN(result.ElapsedSeconds),
		formatDurationCN(remaining),
		result.PressureStyle,
	)
	if result.WarnTriggered {
		section += "\n提醒阈值已触发：请减少铺垫，优先追问关键质量。"
	}
	if result.Expired {
		section += "\n本题已超时：如果用户仍未完成，请要求其简短总结或切换下一题。"
	}
	return section
}

func formatDurationCN(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	remain := seconds % 60
	if minutes == 0 {
		return fmt.Sprintf("%d 秒", remain)
	}
	if remain == 0 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	return fmt.Sprintf("%d 分 %d 秒", minutes, remain)
}
