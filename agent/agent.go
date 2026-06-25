package agent

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/openai/openai-go/v3"

	"agent-web-base/agent/tool"
	"agent-web-base/shared"

	ctxengine "agent-web-base/agent/context"
)

type ToolConfirmConfig struct {
	RequireConfirmTools map[tool.AgentTool]bool
}

type Agent struct {
	mu               sync.Mutex
	confirmConfig    ToolConfirmConfig
	alwaysAllowTools map[tool.AgentTool]bool
	model            string
	client           openai.Client
	contextEngine    *ctxengine.Engine
	nativeTools      map[tool.AgentTool]tool.Tool
	mcpClients       map[string]*McpClient
}

func NewAgent(modelConf shared.ModelConfig, systemPrompt string, confirmConfig ToolConfirmConfig, tools []tool.Tool, mcpClients []*McpClient, contextEngine *ctxengine.Engine) *Agent {
	a := Agent{
		confirmConfig:    confirmConfig,
		alwaysAllowTools: make(map[tool.AgentTool]bool),
		model:            modelConf.Model,
		client:           shared.NewLLMClient(modelConf),
		contextEngine:    contextEngine,
		nativeTools:      make(map[tool.AgentTool]tool.Tool),
		mcpClients:       make(map[string]*McpClient),
	}

	a.contextEngine.Init(systemPrompt, ctxengine.TokenBudget{ContextWindow: modelConf.ContextWindow})

	for _, t := range tools {
		a.nativeTools[t.ToolName()] = t
	}
	for _, mcpClient := range mcpClients {
		a.mcpClients[mcpClient.Name()] = mcpClient
	}

	return &a
}

func (a *Agent) Model() string {
	return a.model
}

func (a *Agent) findTool(toolName string) (tool.Tool, bool) {
	t, ok := a.nativeTools[toolName]
	if ok {
		return t, true
	}
	for _, mcpClient := range a.mcpClients {
		for _, t := range mcpClient.GetTools() {
			if t.ToolName() == toolName {
				return t, true
			}
		}
	}
	return nil, false
}

func (a *Agent) buildTools() []openai.ChatCompletionToolUnionParam {
	tools := make([]openai.ChatCompletionToolUnionParam, 0)
	// 集成 mcp tools
	for _, t := range a.nativeTools {
		tools = append(tools, t.Info())
	}
	// 集成 mcp tools
	for _, mcpClient := range a.mcpClients {
		for _, t := range mcpClient.GetTools() {
			tools = append(tools, t.Info())
		}
	}
	return tools
}

func (a *Agent) ResetSession() {
	a.contextEngine.Reset()
}

type RunResult struct {
	Response string
	Rounds   []shared.OpenAIMessage
	Usage    openai.CompletionUsage
}

// RunStreaming 和 Run 基本逻辑一致，但是使用流式请求，并且通过 channel 实现流式输出
func (a *Agent) RunStreaming(ctx context.Context, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	_, err := a.runStreaming(ctx, query, viewCh, confirmCh, false)
	return err
}

func (a *Agent) RunStreamingWithHistory(ctx context.Context, history []shared.OpenAIMessage, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction) (RunResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.contextEngine.Reset()
	a.contextEngine.SetMessages(history)
	return a.runStreaming(ctx, query, viewCh, confirmCh, false)
}

func (a *Agent) RunTask(ctx context.Context, query string) (RunResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.contextEngine.Reset()
	viewCh := make(chan MessageVO, 64)
	confirmCh := make(chan ConfirmationAction, 1)
	defer close(viewCh)
	defer close(confirmCh)
	go func() {
		for e := range viewCh {
			if e.Type == MessageTypeToolConfirm {
				confirmCh <- ConfirmReject
			}
		}
	}()
	return a.runStreaming(ctx, query, viewCh, confirmCh, true)
}

func (a *Agent) runStreaming(ctx context.Context, query string, viewCh chan MessageVO, confirmCh chan ConfirmationAction, skipPoliciesAndMemory bool) (RunResult, error) {
	a.contextEngine.SetPolicyEventHook(func(policyName string, running bool, err error) {
		viewCh <- MessageVO{
			Type: MessageTypePolicy,
			Policy: &PolicyVO{
				Name:    policyName,
				Running: running,
				Error:   err,
			},
		}
	})
	a.contextEngine.SetMemoryEventHook(func(running bool, err error) {
		viewCh <- MessageVO{
			Type: MessageTypeMemory,
			Memory: &MemoryVO{
				Running: running,
				Error:   err,
			},
		}
	})
	defer a.contextEngine.SetPolicyEventHook(nil)
	defer a.contextEngine.SetMemoryEventHook(nil)

	draft := a.contextEngine.StartTurn(openai.UserMessage(query))
	defer a.contextEngine.AbortTurn(draft)

	// 为本轮次创建新的消息链。草稿消息在 commit 前不会污染上下文。
	messages := a.contextEngine.BuildRequestMessages()
	messages = append(messages, draft.NewMessages...)
	var usage openai.CompletionUsage
	var finalResponse string
	for {
		params := openai.ChatCompletionNewParams{
			Model:         a.model,
			Messages:      messages,
			Tools:         a.buildTools(),
			StreamOptions: openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)},
		}

		log.Printf("calling llm model %s...", a.model)
		stream := a.client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}
		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			if len(chunk.Choices) > 0 {
				deltaRaw := chunk.Choices[0].Delta
				// 不同厂商会把推理内容放在 reasoning_content、reasoning 或 thinking 字段里。
				delta, err := parseDeltaWithReasoning(deltaRaw.RawJSON())
				if err != nil {
					log.Printf("parse delta failed, raw=%s, err=%v", deltaRaw.RawJSON(), err)
					continue
				}
				if reasoningContent := delta.ReasoningText(); reasoningContent != "" {
					viewCh <- MessageVO{
						Type:             MessageTypeReasoning,
						ReasoningContent: &reasoningContent,
					}
				}
				if delta.Content != "" {
					content := delta.Content
					viewCh <- MessageVO{
						Type:    MessageTypeContent,
						Content: &content,
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			viewCh <- MessageVO{
				Type:    MessageTypeError,
				Content: shared.Ptr(err.Error()),
			}
			return RunResult{Rounds: draft.NewMessages, Usage: usage}, err
		}
		if len(acc.Choices) == 0 {
			log.Printf("no choices returned, resp: %v", acc)
			return RunResult{Rounds: draft.NewMessages, Usage: usage}, nil
		}
		usage = acc.Usage
		message := acc.Choices[0].Message
		// 拼接 assistant message 到整体消息链中
		assistantMsg := message.ToParam()
		messages = append(messages, assistantMsg)
		draft.NewMessages = append(draft.NewMessages, assistantMsg)

		// tool loop 结束，可以返回结果
		if len(message.ToolCalls) == 0 {
			finalResponse = message.Content
			break
		}

		for _, toolCall := range message.ToolCalls {
			t, ok := a.findTool(toolCall.Function.Name)
			if !ok {
				viewCh <- MessageVO{
					Type:    MessageTypeError,
					Content: shared.Ptr("tool not found"),
				}
				toolMsg := openai.ToolMessage("tool not found", toolCall.ID)
				messages = append(messages, toolMsg)
				draft.NewMessages = append(draft.NewMessages, toolMsg)
				continue
			}

			toolName := t.ToolName()
			needConfirm := a.confirmConfig.RequireConfirmTools[toolName] && !a.alwaysAllowTools[toolName]

			if needConfirm {
				confirmReq := ToolConfirmationVO{
					ToolName:  toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				}
				viewCh <- MessageVO{
					Type:                    MessageTypeToolConfirm,
					ToolConfirmationRequest: &confirmReq,
				}

				select {
				case <-ctx.Done():
					return RunResult{Response: finalResponse, Rounds: draft.NewMessages, Usage: usage}, nil
				case action := <-confirmCh:
					switch action {
					case ConfirmReject:
						toolMsg := openai.ToolMessage("user rejected tool call", toolCall.ID)
						messages = append(messages, toolMsg)
						draft.NewMessages = append(draft.NewMessages, toolMsg)
						continue
					case ConfirmAlwaysAllow:
						a.alwaysAllowTools[toolName] = true
					case ConfirmAllow:
					}
				}
			}

			toolResult, err := t.Execute(ctx, toolCall.Function.Arguments)
			if err != nil {
				toolResult = err.Error()
				viewCh <- MessageVO{
					Type:    MessageTypeError,
					Content: &toolResult,
				}
			}

			viewCh <- MessageVO{
				Type: MessageTypeToolCall,
				ToolCall: &ToolCallVO{
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			}
			viewCh <- MessageVO{
				Type: MessageTypeToolResult,
				ToolResult: &ToolResultVO{
					Name:   toolCall.Function.Name,
					Result: toolResult,
				},
			}

			log.Printf("tool call %s, arguments %s, error: %v", toolCall.Function.Name, toolCall.Function.Arguments, err)
			toolMsg := openai.ToolMessage(toolResult, toolCall.ID)
			messages = append(messages, toolMsg)
			draft.NewMessages = append(draft.NewMessages, toolMsg)
		}

		// 每次循环后检查 context 是否被取消（用户按 ESC）
		select {
		case <-ctx.Done():
			// 用户按 ESC，保存消息但不执行 policies/memory
			_ = a.contextEngine.CommitTurn(ctx, draft, ctxengine.Usage{PromptTokens: int(usage.TotalTokens)}, true)
			return RunResult{Response: finalResponse, Rounds: draft.NewMessages, Usage: usage}, nil
		default:
		}

	}

	err := a.contextEngine.CommitTurn(ctx, draft, ctxengine.Usage{PromptTokens: int(usage.TotalTokens)}, skipPoliciesAndMemory)
	return RunResult{Response: finalResponse, Rounds: draft.NewMessages, Usage: usage}, err
}

type deltaWithReasoning struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
	Reasoning        string `json:"reasoning"`
	Thinking         string `json:"thinking"`
}

func parseDeltaWithReasoning(rawJSON string) (deltaWithReasoning, error) {
	delta := deltaWithReasoning{}
	err := json.Unmarshal([]byte(rawJSON), &delta)
	return delta, err
}

func (d deltaWithReasoning) ReasoningText() string {
	switch {
	case d.ReasoningContent != "":
		return d.ReasoningContent
	case d.Reasoning != "":
		return d.Reasoning
	default:
		return d.Thinking
	}
}
