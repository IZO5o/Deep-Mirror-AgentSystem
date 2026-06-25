package vo

const (
	SSETypeError       = "error"
	SSETypeReasoning   = "reasoning"
	SSETypeContent     = "content"
	SSETypeToolCall    = "tool_call"
	SSETypeToolResult  = "tool_result"
	SSETypePolicy      = "policy"
	SSETypeMemory      = "memory"
	SSETypeToolConfirm = "tool_confirm"
)

type SSEMessageVO struct {
	MessageID        string                   `json:"message_id"`
	AgentType        string                   `json:"agent_type,omitempty"`
	Event            string                   `json:"event"`
	Content          *string                  `json:"content,omitempty"`
	ReasoningContent *string                  `json:"reasoning_content,omitempty"`
	ToolCall         *string                  `json:"tool_call,omitempty"`
	ToolArguments    *string                  `json:"tool_arguments,omitempty"`
	ToolResult       *string                  `json:"tool_result,omitempty"`
	Policy           *PolicyEventVO           `json:"policy,omitempty"`
	Memory           *MemoryEventVO           `json:"memory,omitempty"`
	ToolConfirmation *ToolConfirmationEventVO `json:"tool_confirmation,omitempty"`
}

type PolicyEventVO struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

type MemoryEventVO struct {
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

type ToolConfirmationEventVO struct {
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
	Action    string `json:"action"`
}
