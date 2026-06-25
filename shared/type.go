package shared

import "github.com/openai/openai-go/v3"

// 给OpenAI SDK 消息定义一个别名，方便后续替换其他模型提供商的SDK
type OpenAIMessage = openai.ChatCompletionMessageParamUnion

// GetRoleName 从消息中获取角色名称（不依赖 GetRole()）
func GetRoleName(message OpenAIMessage) string {
	if message.OfSystem != nil {
		return "system"
	}
	if message.OfUser != nil {
		return "user"
	}
	if message.OfAssistant != nil {
		return "assistant"
	}
	if message.OfTool != nil {
		return "tool"
	}
	if message.OfDeveloper != nil {
		return "developer"
	}
	if message.OfFunction != nil {
		return "function"
	}
	return "unknown"
}
