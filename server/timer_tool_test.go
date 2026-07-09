package server

import (
	"strings"
	"testing"
	"time"
)

func TestCheckMockTimerInactiveWithoutLimit(t *testing.T) {
	result := checkMockTimer(MockInterview{MockID: "mock_1"}, []MockTurn{
		{Role: mockTurnRoleAssistant, TurnType: mockTurnTypeOpeningQuestion, CreatedAt: 100},
	}, time.Unix(200, 0))
	if result.Active {
		t.Fatalf("Active = true, want false")
	}
}

func TestCheckMockTimerWarnAndExpired(t *testing.T) {
	turns := []MockTurn{{
		Role:              mockTurnRoleAssistant,
		TurnType:          mockTurnTypeOpeningQuestion,
		TimeLimitSeconds:  900,
		TimePressureStyle: TimePressureStyleModerate,
		WarnAtSeconds:     300,
		CreatedAt:         1000,
	}}
	result := checkMockTimer(MockInterview{MockID: "mock_1"}, turns, time.Unix(1901, 0))
	if !result.Active || !result.WarnTriggered || !result.Expired {
		t.Fatalf("timer result = %+v, want active warn expired", result)
	}
}

func TestFormatMockTimerPromptSection(t *testing.T) {
	text := formatMockTimerPromptSection(TimerResult{
		Active:           true,
		TotalSeconds:     900,
		ElapsedSeconds:   630,
		RemainingSeconds: 270,
		WarnTriggered:    true,
		PressureStyle:    TimePressureStyleModerate,
	})
	for _, want := range []string{"=== 时间状态 ===", "15 分钟", "4 分 30 秒", "提醒阈值已触发"} {
		if !strings.Contains(text, want) {
			t.Fatalf("timer prompt missing %q in %s", want, text)
		}
	}
}
