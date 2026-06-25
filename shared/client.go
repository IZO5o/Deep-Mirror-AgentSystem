package shared

import (
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// 创建一个兼容OpenAI SDK的LLM客户端，方便后续替换其他模型提供商的SDK
func NewLLMClient(modelConf ModelConfig) openai.Client {
	client := openai.NewClient(
		option.WithBaseURL(modelConf.BaseURL),
		option.WithAPIKey(modelConf.ApiKey),
		option.WithHeader("X-Title", "AgentWebBase"),
		option.WithHeader("HTTP-Referer", "https://github.com/agent-web-base"),
	)
	return client
}
