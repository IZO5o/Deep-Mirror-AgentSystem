package agent

type ConfirmationAction int

const (
	ConfirmAllow ConfirmationAction = iota
	ConfirmReject
	ConfirmAlwaysAllow
)

const (
	MessageTypeReasoning   = "reasoning"
	MessageTypeContent     = "content"
	MessageTypeToolCall    = "tool_call"
	MessageTypeToolResult  = "tool_result"
	MessageTypeError       = "error"
	MessageTypePolicy      = "policy"
	MessageTypeMemory      = "memory"
	MessageTypeToolConfirm = "tool_confirm"
)

// MessageVO 用于流式展示当前模型流式输出或者状态
type MessageVO struct {
	Type string `json:"type"`

	ReasoningContent        *string             `json:"reasoning_content,omitempty"`
	Content                 *string             `json:"content,omitempty"`
	ToolCall                *ToolCallVO         `json:"tool,omitempty"`
	ToolResult              *ToolResultVO       `json:"tool_result,omitempty"`
	Policy                  *PolicyVO           `json:"policy,omitempty"`
	Memory                  *MemoryVO           `json:"memory,omitempty"`
	ToolConfirmationRequest *ToolConfirmationVO `json:"tool_confirmation_request,omitempty"`
}

// PolicyVO 策略执行状态
type PolicyVO struct {
	Name    string `json:"name"`    // 策略名称
	Running bool   `json:"running"` // 是否正在执行
	Error   error  `json:"error"`
}

// MemoryVO 记忆更新状态
type MemoryVO struct {
	Running bool  `json:"running"` // 是否正在执行
	Error   error `json:"error"`
}

type ToolCallVO struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolResultVO struct {
	Name   string `json:"name"`
	Result string `json:"result"`
}

type ToolConfirmationVO struct {
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
}
