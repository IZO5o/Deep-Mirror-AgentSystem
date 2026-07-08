package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-web-base/agent"
	"agent-web-base/shared"
	"agent-web-base/shared/log"
	"agent-web-base/vo"
)

type Server struct {
	db                    *gorm.DB
	agents                AgentProvider
	mediaDir              string
	asrClient             ASRClient
	audioExtractor        AudioExtractor
	transcriptionQueue    chan string
	traceBeforeCreateHook func(*AgentDecisionTrace) error
}

type AgentProvider interface {
	Get(rawType string) (agent.AgentType, agent.Runner, error)
}

func NewServer(db *gorm.DB, agents AgentProvider) *Server {
	return &Server{
		db:                 db,
		agents:             agents,
		mediaDir:           filepath.Join(shared.GetWorkspaceDir(), ".agent-web-base", "media"),
		audioExtractor:     NewFFmpegAudioExtractor("ffmpeg"),
		transcriptionQueue: make(chan string, 100),
	}
}

func (s *Server) ConfigureTranscription(mediaDir string, asrClient ASRClient, audioExtractor AudioExtractor) {
	if mediaDir != "" {
		s.mediaDir = mediaDir
	}
	if asrClient != nil {
		s.asrClient = asrClient
	}
	if audioExtractor != nil {
		s.audioExtractor = audioExtractor
	}
	if s.transcriptionQueue == nil {
		s.transcriptionQueue = make(chan string, 100)
	}
}

func (s *Server) ResolveAgentType(rawType string) (agent.AgentType, error) {
	if s.agents == nil {
		return "", fmt.Errorf("agent provider is nil")
	}
	agentType, _, err := s.agents.Get(rawType)
	return agentType, err
}

func (s *Server) CreateConversation(req vo.CreateConversationReq) (vo.ConversationVO, error) {
	conv := Conversation{
		ConversationID: uuid.New().String(),
		UserID:         req.UserID,
		Title:          req.Title,
		CreatedAt:      time.Now().Unix(),
	}
	if err := s.db.Create(&conv).Error; err != nil {
		return vo.ConversationVO{}, err
	}
	return vo.ConversationVO{
		ConversationID: conv.ConversationID,
		UserID:         conv.UserID,
		Title:          conv.Title,
		CreatedAt:      conv.CreatedAt,
	}, nil
}

func (s *Server) ListConversations(userID string) ([]vo.ConversationVO, error) {
	var convs []Conversation
	query := s.db.Order("created_at desc")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&convs).Error; err != nil {
		return nil, err
	}

	result := make([]vo.ConversationVO, 0, len(convs))
	for _, conv := range convs {
		result = append(result, vo.ConversationVO{
			ConversationID: conv.ConversationID,
			UserID:         conv.UserID,
			Title:          conv.Title,
			CreatedAt:      conv.CreatedAt,
		})
	}
	return result, nil
}

func (s *Server) RenameConversation(conversationID string, title string) (vo.ConversationVO, error) {
	if err := s.db.Model(&Conversation{}).
		Where("conversation_id = ?", conversationID).
		Update("title", title).Error; err != nil {
		return vo.ConversationVO{}, err
	}

	var conv Conversation
	if err := s.db.First(&conv, "conversation_id = ?", conversationID).Error; err != nil {
		return vo.ConversationVO{}, err
	}

	return vo.ConversationVO{
		ConversationID: conv.ConversationID,
		UserID:         conv.UserID,
		Title:          conv.Title,
		CreatedAt:      conv.CreatedAt,
	}, nil
}

func (s *Server) DeleteConversation(conversationID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", conversationID).
			Delete(&ChatMessage{}).Error; err != nil {
			return err
		}

		return tx.Where("conversation_id = ?", conversationID).
			Delete(&Conversation{}).Error
	})
}

func (s *Server) ListMessages(conversationID string) ([]vo.ChatMessageVO, error) {
	var msgs []ChatMessage
	if err := s.db.Where("conversation_id = ?", conversationID).
		Order("created_at asc").Find(&msgs).Error; err != nil {
		return nil, err
	}

	result := make([]vo.ChatMessageVO, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, vo.ChatMessageVO{
			MessageID:       msg.MessageID,
			ConversationID:  msg.ConversationID,
			ParentMessageID: msg.ParentMessageID,
			AgentType:       normalizeAgentType(msg.AgentType),
			Query:           msg.Query,
			Response:        msg.Response,
			Model:           msg.Model,
			CreatedAt:       msg.CreatedAt,
			Rounds:          parseRounds(msg.Rounds),
		})
	}
	return result, nil
}

// CreateMessage 验证会话、构建历史、保存消息记录，并启动 agent 流式执行。
func (s *Server) CreateMessage(ctx context.Context, conversationID string, req vo.CreateMessageReq, voCh chan<- vo.SSEMessageVO) error {
	if s.agents == nil {
		return fmt.Errorf("agent provider is nil")
	}
	agentType, runner, err := s.agents.Get(req.AgentType)
	if err != nil {
		return err
	}

	// 验证会话存在
	var conv Conversation
	if err := s.db.Where("conversation_id = ?", conversationID).First(&conv).Error; err != nil {
		return err
	}

	// 从历史消息构建 history
	var historyMsgs []ChatMessage
	if err := s.db.Where("conversation_id = ?", conversationID).
		Order("created_at asc").Find(&historyMsgs).Error; err != nil {
		return err
	}
	history := buildHistory(historyMsgs, req.ParentMessageID)

	msgID := uuid.New().String()
	createdAt := time.Now().Unix()

	eventCh := make(chan agent.MessageVO, 64)
	confirmCh := make(chan agent.ConfirmationAction, 1)
	defer func() {
		close(eventCh)
		close(confirmCh)
	}()

	go func() {
		for e := range eventCh {
			if e.Type == agent.MessageTypeToolConfirm {
				confirmCh <- agent.ConfirmReject
			}
			voCh <- toSSEMessage(msgID, agentType, e)
		}
	}()

	result, runErr := runner.RunStreamingWithHistory(ctx, history, req.Query, eventCh, confirmCh)
	if runErr != nil {
		log.Warnf("run streaming error: %v", runErr)
	}

	roundsJSON, _ := json.Marshal(result.Rounds)
	usageJSON, _ := json.Marshal(result.Usage)
	s.db.Create(&ChatMessage{
		MessageID:       msgID,
		UserID:          req.UserID,
		ConversationID:  conversationID,
		ParentMessageID: req.ParentMessageID,
		AgentType:       string(agentType),
		Query:           req.Query,
		Response:        result.Response,
		Rounds:          string(roundsJSON),
		Usage:           string(usageJSON),
		Model:           runner.Model(),
		CreatedAt:       createdAt,
	})

	return nil
}

func toSSEMessage(msgID string, agentType agent.AgentType, e agent.MessageVO) vo.SSEMessageVO {
	msg := vo.SSEMessageVO{MessageID: msgID, AgentType: string(agentType), Event: e.Type}
	switch e.Type {
	case agent.MessageTypeReasoning:
		msg.ReasoningContent = e.ReasoningContent
	case agent.MessageTypeContent, agent.MessageTypeError:
		msg.Content = e.Content
	case agent.MessageTypeToolCall:
		if e.ToolCall != nil {
			msg.ToolCall = &e.ToolCall.Name
			msg.ToolArguments = &e.ToolCall.Arguments
		}
	case agent.MessageTypeToolResult:
		if e.ToolResult != nil {
			msg.ToolCall = &e.ToolResult.Name
			msg.ToolResult = &e.ToolResult.Result
		}
	case agent.MessageTypePolicy:
		if e.Policy != nil {
			policy := &vo.PolicyEventVO{
				Name:    e.Policy.Name,
				Running: e.Policy.Running,
			}
			if e.Policy.Error != nil {
				policy.Error = e.Policy.Error.Error()
			}
			msg.Policy = policy
		}
	case agent.MessageTypeMemory:
		if e.Memory != nil {
			memory := &vo.MemoryEventVO{
				Running: e.Memory.Running,
			}
			if e.Memory.Error != nil {
				memory.Error = e.Memory.Error.Error()
			}
			msg.Memory = memory
		}
	case agent.MessageTypeToolConfirm:
		if e.ToolConfirmationRequest != nil {
			msg.ToolConfirmation = &vo.ToolConfirmationEventVO{
				ToolName:  e.ToolConfirmationRequest.ToolName,
				Arguments: e.ToolConfirmationRequest.Arguments,
				Action:    "auto_reject",
			}
		}
	}
	return msg
}

func normalizeAgentType(agentType string) string {
	if agentType == "" {
		return string(agent.AgentTypeAssistant)
	}
	return agentType
}

// parseRounds 将存储的 rounds JSON 转换为前端友好的 RoundMessageVO 列表。
func parseRounds(roundsJSON string) []vo.RoundMessageVO {
	if roundsJSON == "" {
		return nil
	}
	var msgs []shared.OpenAIMessage
	if err := json.Unmarshal([]byte(roundsJSON), &msgs); err != nil {
		return nil
	}

	result := make([]vo.RoundMessageVO, 0, len(msgs))
	for _, m := range msgs {
		switch {
		case m.OfUser != nil:
			// user 消息不需要展示
			continue

		case m.OfAssistant != nil:
			a := m.OfAssistant
			rv := vo.RoundMessageVO{Role: "assistant"}
			if len(a.ToolCalls) > 0 {
				for _, tc := range a.ToolCalls {
					if tc.OfFunction != nil {
						rv.ToolCalls = append(rv.ToolCalls, vo.ToolCallVO{
							ID:        tc.OfFunction.ID,
							Name:      tc.OfFunction.Function.Name,
							Arguments: tc.OfFunction.Function.Arguments,
						})
					}
				}
				result = append(result, rv)
			}

		case m.OfTool != nil:
			t := m.OfTool
			result = append(result, vo.RoundMessageVO{
				Role:    "tool",
				ToolID:  t.ToolCallID,
				Content: t.Content.OfString.Value,
			})
		}
	}
	return result
}
