package server

import (
	"encoding/json"
	"strings"
)

const (
	DefenseRuleChatSubmitModeForcesChatOnly = "R1_chat_submit_mode_forces_chat_only"
	DefenseRuleStateActionWhitelist         = "R2_state_action_whitelist"
	DefenseRuleRecordAttemptScoreRange      = "R3_record_attempt_score_range"
	DefenseRuleSmalltalkUnclearChatOnly     = "R4_smalltalk_unclear_chat_only"
	DefenseRuleMemoryItemsWriteWarning      = "R5_memory_items_write_warning"
)

type DefenseRuleDecision struct {
	RuleID  string `json:"rule_id"`
	Applied bool   `json:"applied"`
	Reason  string `json:"reason"`
}

func marshalDefenseRuleDecisions(decisions []DefenseRuleDecision) string {
	data, _ := json.Marshal(decisions)
	return string(data)
}

func containsMemoryItemsWriteAction(text string) bool {
	normalized := strings.ToLower(text)
	return strings.Contains(normalized, "created memory_items") ||
		strings.Contains(normalized, "updated memory_items") ||
		strings.Contains(normalized, "upserted memory_items") ||
		strings.Contains(normalized, `"memory_items"`)
}
