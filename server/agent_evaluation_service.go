package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-web-base/vo"
)

func (s *Server) EvaluateAgentDecisionTraces(query AgentDecisionTraceQuery) (vo.AgentEvaluationReportVO, error) {
	traces, err := s.ListAgentDecisionTraces(query)
	if err != nil {
		return vo.AgentEvaluationReportVO{}, err
	}

	report := vo.AgentEvaluationReportVO{
		TotalTraces: len(traces),
		Results:     make([]vo.AgentEvaluationResultVO, 0, len(traces)),
	}
	for _, trace := range traces {
		result := evaluateAgentDecisionTrace(trace)
		if result.Passed {
			report.PassedTraces++
		} else {
			report.FailedTraces++
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func evaluateAgentDecisionTrace(trace vo.AgentDecisionTraceVO) vo.AgentEvaluationResultVO {
	checks := make([]vo.AgentEvaluationCheckVO, 0, 16)
	add := func(name string, passed bool, reason string) {
		checks = append(checks, vo.AgentEvaluationCheckVO{Name: name, Passed: passed, Reason: reason})
	}

	status := strings.TrimSpace(trace.Status)
	if status == AgentDecisionTraceStatusSucceeded {
		add("succeeded_raw_agent_output", strings.TrimSpace(trace.RawAgentOutput) != "", "succeeded trace should keep raw_agent_output")
		add("succeeded_parsed_decision", strings.TrimSpace(trace.ParsedDecision) != "", "succeeded trace should keep parsed_decision")
		add("succeeded_service_actions", strings.TrimSpace(trace.ServiceActions) != "", "succeeded trace should keep service_actions")
	} else if status == AgentDecisionTraceStatusFailed {
		add("failed_error_message", strings.TrimSpace(trace.ErrorMessage) != "", "failed trace should keep error_message")
		add("failed_debug_payload", strings.TrimSpace(trace.RawAgentOutput) != "" || strings.TrimSpace(trace.InputSnapshot) != "", "failed trace should keep raw_agent_output or input_snapshot")
	} else {
		add("known_trace_status", false, fmt.Sprintf("unknown trace status %q", trace.Status))
	}

	addJSONValidityChecks(add, trace)
	addSelectedContextChecks(add, trace)
	addServiceActionChecks(add, trace)
	addMemoryBoundaryCheck(add, trace)
	addFailedTraceActionCheck(add, trace)

	passedCount := 0
	for _, check := range checks {
		if check.Passed {
			passedCount++
		}
	}
	score := 100
	passed := true
	if len(checks) > 0 {
		score = passedCount * 100 / len(checks)
		passed = passedCount == len(checks)
	}
	return vo.AgentEvaluationResultVO{
		TraceID:     trace.TraceID,
		AgentType:   trace.AgentType,
		SourceType:  trace.SourceType,
		SourceID:    trace.SourceID,
		StepName:    trace.StepName,
		TraceStatus: trace.Status,
		Passed:      passed,
		Score:       score,
		Checks:      checks,
	}
}

func addJSONValidityChecks(add func(string, bool, string), trace vo.AgentDecisionTraceVO) {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "parsed_decision_json", value: trace.ParsedDecision},
		{name: "selected_context_snapshot_json", value: trace.SelectedContextSnapshot},
		{name: "input_snapshot_json", value: trace.InputSnapshot},
		{name: "service_actions_json", value: trace.ServiceActions},
	} {
		value := strings.TrimSpace(field.value)
		if value == "" {
			add(field.name, true, "empty field is allowed")
			continue
		}
		add(field.name, json.Valid([]byte(value)), "non-empty trace JSON field should be valid JSON")
	}
}

func addSelectedContextChecks(add func(string, bool, string), trace vo.AgentDecisionTraceVO) {
	required := selectedContextRequiredForStep(trace.StepName)
	snapshot := strings.TrimSpace(trace.SelectedContextSnapshot)
	if !required {
		add("selected_context_required", true, "selected context is not required for this step")
		return
	}
	if snapshot == "" {
		add("selected_context_required", false, "this step should keep selected_context_snapshot")
		return
	}
	add("selected_context_required", true, "selected_context_snapshot is present")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(snapshot), &parsed); err != nil {
		add("selected_context_schema", false, "selected_context_snapshot should be a JSON object")
		return
	}
	for _, key := range []string{"selected_memory_items", "selected_practice_states", "debug_summary"} {
		_, ok := parsed[key]
		add("selected_context_has_"+key, ok, "selected_context_snapshot should include "+key)
	}
}

func selectedContextRequiredForStep(stepName string) bool {
	switch stepName {
	case AgentTraceStepCoachingPlanGenerate, AgentTraceStepMockStart, AgentTraceStepMockTurn:
		return true
	default:
		return false
	}
}

func addServiceActionChecks(add func(string, bool, string), trace vo.AgentDecisionTraceVO) {
	actions := strings.ToLower(trace.ServiceActions)
	if strings.TrimSpace(actions) == "" {
		add("service_actions_semantics", trace.Status != AgentDecisionTraceStatusSucceeded, "service_actions semantics require actions to inspect")
		return
	}
	if trace.Status == AgentDecisionTraceStatusFailed {
		return
	}

	switch trace.StepName {
	case AgentTraceStepCoachingPlanGenerate:
		add("service_actions_coaching_plan", containsAll(actions, "created coaching_plan", "created coaching_tasks"), "coaching_plan_generate should create plan and tasks")
	case AgentTraceStepCoachingSessionTurn:
		addCoachingSessionTurnActionChecks(add, trace, actions)
	case AgentTraceStepMockStart:
		add("service_actions_mock_start", containsAll(actions, "created mock_interview", "opening mock_turn"), "mock_start should create mock_interview and opening mock_turn")
	case AgentTraceStepMockTurn:
		addMockTurnActionChecks(add, trace, actions)
	case AgentTraceStepCoachingSessionMemoryCandidates, AgentTraceStepMockInterviewMemoryCandidates:
		add("service_actions_memory_candidates", strings.Contains(actions, "generated memory_candidates"), "memory candidate generation should report generated memory_candidates")
		add("service_actions_memory_events", strings.Contains(actions, "generated memory_events"), "memory candidate generation should report generated memory_events")
	default:
		add("service_actions_known_step", true, "no step-specific service action rule")
	}
}

func addCoachingSessionTurnActionChecks(add func(string, bool, string), trace vo.AgentDecisionTraceVO, actions string) {
	inputType := parsedDecisionString(trace.ParsedDecision, "input_type")
	add("service_actions_coaching_turn", containsAll(actions, "coaching_session", "turn", "updated coaching_session"), "coaching_session_turn should record turn and update session")
	if inputType == CoachingInputTypeFormalAnswer {
		add("service_actions_coaching_formal_answer", containsAll(actions, "coaching_task_attempt", "practice_states"), "formal coaching answer should record attempt and update practice_states")
		return
	}
	add("service_actions_coaching_non_formal", !strings.Contains(actions, "practice_states"), "non-formal coaching input should not require practice_states")
}

func addMockTurnActionChecks(add func(string, bool, string), trace vo.AgentDecisionTraceVO, actions string) {
	add("service_actions_mock_turns", strings.Contains(actions, "mock_turns"), "mock_turn should create mock_turns")
	if parsedDecisionBool(trace.ParsedDecision, "should_update_practice_state") {
		add("service_actions_mock_practice_update", strings.Contains(actions, "updated practice_states"), "mock formal answer with should_update_practice_state=true should update practice_states")
		return
	}
	add("service_actions_mock_practice_optional", true, "practice_states is not required for hint/explanation/cancel or skipped updates")
}

func addMemoryBoundaryCheck(add func(string, bool, string), trace vo.AgentDecisionTraceVO) {
	actions := strings.ToLower(trace.ServiceActions)
	violated := strings.Contains(actions, "created memory_items") ||
		strings.Contains(actions, "updated memory_items") ||
		strings.Contains(actions, "upserted memory_items")
	add("memory_items_write_boundary", !violated, "service_actions must not directly create/update memory_items")
}

func addFailedTraceActionCheck(add func(string, bool, string), trace vo.AgentDecisionTraceVO) {
	if trace.Status != AgentDecisionTraceStatusFailed {
		return
	}
	actions := strings.ToLower(trace.ServiceActions)
	describesFailure := strings.Contains(actions, "failed") ||
		strings.Contains(actions, "did not create") ||
		strings.Contains(actions, "recorded failed") ||
		strings.Contains(actions, "updated failed")
	add("failed_service_actions_stage", describesFailure, "failed trace service_actions should identify the failed stage")
}

func parsedDecisionString(raw string, key string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return ""
	}
	value, _ := parsed[key].(string)
	return value
}

func parsedDecisionBool(raw string, key string) bool {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return false
	}
	value, _ := parsed[key].(bool)
	return value
}

func containsAll(value string, wants ...string) bool {
	for _, want := range wants {
		if !strings.Contains(value, strings.ToLower(want)) {
			return false
		}
	}
	return true
}
