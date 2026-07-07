package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestApplyPersistentStateUpdateMergeAndReplace(t *testing.T) {
	now := int64(1710000000)
	current := `{"focus":"Redis","weaknesses":["clarity"],"updated_at":1700000000}`

	merged, err := applyPersistentStateUpdate(current, PersistentStateUpdate{
		UpdateMode: PersistentStateUpdateModeMerge,
		Fields: map[string]any{
			"focus":       "Kafka",
			"next_action": "practice STAR",
			"updated_at":  int64(1),
			"weaknesses":  []any{"depth", "examples"},
		},
	}, now)
	if err != nil {
		t.Fatalf("merge applyPersistentStateUpdate() error = %v", err)
	}

	var mergedState map[string]any
	if err := json.Unmarshal([]byte(merged), &mergedState); err != nil {
		t.Fatalf("merge state is not JSON: %v", err)
	}
	if got := mergedState["focus"]; got != "Kafka" {
		t.Fatalf("merged focus = %#v, want Kafka", got)
	}
	if got := mergedState["next_action"]; got != "practice STAR" {
		t.Fatalf("merged next_action = %#v, want practice STAR", got)
	}
	if got := int64(mergedState["updated_at"].(float64)); got != now {
		t.Fatalf("merged updated_at = %d, want %d", got, now)
	}
	if got, want := mergedState["weaknesses"], []any{"depth", "examples"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged weaknesses = %#v, want %#v", got, want)
	}

	replaced, err := applyPersistentStateUpdate(current, PersistentStateUpdate{
		UpdateMode: PersistentStateUpdateModeReplace,
		Fields: map[string]any{
			"summary": "stronger close",
		},
	}, now+1)
	if err != nil {
		t.Fatalf("replace applyPersistentStateUpdate() error = %v", err)
	}

	var replacedState map[string]any
	if err := json.Unmarshal([]byte(replaced), &replacedState); err != nil {
		t.Fatalf("replace state is not JSON: %v", err)
	}
	if _, ok := replacedState["focus"]; ok {
		t.Fatalf("replace kept previous focus: %#v", replacedState)
	}
	if got := replacedState["summary"]; got != "stronger close" {
		t.Fatalf("replace summary = %#v, want stronger close", got)
	}
	if got := int64(replacedState["updated_at"].(float64)); got != now+1 {
		t.Fatalf("replace updated_at = %d, want %d", got, now+1)
	}
}

func TestBuildPersistentStatePromptSection(t *testing.T) {
	coachingRaw := `{"updated_at":1710000000,"weaknesses":["结构","例子"],"focus":"系统设计","attempts":2}`
	coaching := buildPersistentStatePromptSection("coaching", coachingRaw)

	wantOrder := []string{
		"=== 持续辅导状态（上轮决策）===",
		"- attempts: 2",
		"- focus: 系统设计",
		"- weaknesses: [结构, 例子]",
	}
	lastIndex := -1
	for _, want := range wantOrder {
		idx := strings.Index(coaching, want)
		if idx < 0 {
			t.Fatalf("coaching prompt missing %q in:\n%s", want, coaching)
		}
		if idx <= lastIndex {
			t.Fatalf("coaching prompt order wrong for %q in:\n%s", want, coaching)
		}
		lastIndex = idx
	}
	if strings.Contains(coaching, "updated_at") {
		t.Fatalf("coaching prompt should exclude updated_at:\n%s", coaching)
	}

	mock := buildPersistentStatePromptSection("mock", "")
	for _, want := range []string{
		"=== 持续模拟面试状态（上轮决策）===",
		"暂无持续状态",
		"首次进入该 mock",
	} {
		if !strings.Contains(mock, want) {
			t.Fatalf("mock empty prompt missing %q in:\n%s", want, mock)
		}
	}
}

func TestApplyPersistentStateUpdateRejectsOversizedModelState(t *testing.T) {
	oversized := strings.Repeat("x", maxPersistentStateStringBytes+1)
	_, err := applyPersistentStateUpdate("", PersistentStateUpdate{
		UpdateMode: PersistentStateUpdateModeMerge,
		Fields: map[string]any{
			"active_focus": oversized,
		},
	}, 200)
	if err == nil {
		t.Fatalf("applyPersistentStateUpdate() error = nil, want oversized field error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want size limit message", err)
	}
}
