package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-web-base/agent"
	"agent-web-base/shared"
	"agent-web-base/vo"
)

func TestRenameConversation_UpdatesTitle(t *testing.T) {
	s := newTestServer(t)

	created, err := s.CreateConversation(vo.CreateConversationReq{
		UserID: "user_001",
		Title:  "Old Title",
	})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	updated, err := s.RenameConversation(created.ConversationID, "New Title")
	if err != nil {
		t.Fatalf("RenameConversation() error = %v", err)
	}

	if updated.Title != "New Title" {
		t.Fatalf("updated title = %q, want %q", updated.Title, "New Title")
	}

	var stored Conversation
	if err := s.db.First(&stored, "conversation_id = ?", created.ConversationID).Error; err != nil {
		t.Fatalf("load stored conversation: %v", err)
	}

	if stored.Title != "New Title" {
		t.Fatalf("stored title = %q, want %q", stored.Title, "New Title")
	}
}

func TestDeleteConversation_RemovesConversationAndMessages(t *testing.T) {
	s := newTestServer(t)

	created, err := s.CreateConversation(vo.CreateConversationReq{
		UserID: "user_001",
		Title:  "Delete Me",
	})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	if err := s.db.Create(&ChatMessage{
		MessageID:       "msg-1",
		UserID:          "user_001",
		ConversationID:  created.ConversationID,
		ParentMessageID: "",
		Query:           "hello",
		Response:        "world",
		Model:           "test-model",
		CreatedAt:       time.Now().Unix(),
	}).Error; err != nil {
		t.Fatalf("seed chat message: %v", err)
	}

	if err := s.DeleteConversation(created.ConversationID); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	var conversationCount int64
	if err := s.db.Model(&Conversation{}).
		Where("conversation_id = ?", created.ConversationID).
		Count(&conversationCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if conversationCount != 0 {
		t.Fatalf("conversation count = %d, want 0", conversationCount)
	}

	var messageCount int64
	if err := s.db.Model(&ChatMessage{}).
		Where("conversation_id = ?", created.ConversationID).
		Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("message count = %d, want 0", messageCount)
	}
}

func TestCreateMessage_DefaultAgentType(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)

	created, err := s.CreateConversation(vo.CreateConversationReq{
		UserID: "user_001",
		Title:  "Default Agent",
	})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	voCh := make(chan vo.SSEMessageVO, 8)
	err = s.CreateMessage(context.Background(), created.ConversationID, vo.CreateMessageReq{
		UserID: "user_001",
		Query:  "hello",
	}, voCh)
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	if runners[agent.AgentTypeAssistant].calls != 1 {
		t.Fatalf("assistant calls = %d, want 1", runners[agent.AgentTypeAssistant].calls)
	}

	var stored ChatMessage
	if err := s.db.First(&stored, "conversation_id = ?", created.ConversationID).Error; err != nil {
		t.Fatalf("load stored message: %v", err)
	}
	if stored.AgentType != string(agent.AgentTypeAssistant) {
		t.Fatalf("stored agent_type = %q, want %q", stored.AgentType, agent.AgentTypeAssistant)
	}

	msgs, err := s.ListMessages(created.ConversationID)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("message count = %d, want 1", len(msgs))
	}
	if msgs[0].AgentType != string(agent.AgentTypeAssistant) {
		t.Fatalf("VO agent_type = %q, want %q", msgs[0].AgentType, agent.AgentTypeAssistant)
	}
}

func TestCreateMessage_SpecifiedAgentType(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)

	created, err := s.CreateConversation(vo.CreateConversationReq{
		UserID: "user_001",
		Title:  "Review Agent",
	})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	voCh := make(chan vo.SSEMessageVO, 8)
	err = s.CreateMessage(context.Background(), created.ConversationID, vo.CreateMessageReq{
		UserID:    "user_001",
		Query:     "review this interview",
		AgentType: string(agent.AgentTypeReview),
	}, voCh)
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	if runners[agent.AgentTypeReview].calls != 1 {
		t.Fatalf("review calls = %d, want 1", runners[agent.AgentTypeReview].calls)
	}
	if runners[agent.AgentTypeAssistant].calls != 0 {
		t.Fatalf("assistant calls = %d, want 0", runners[agent.AgentTypeAssistant].calls)
	}

	var stored ChatMessage
	if err := s.db.First(&stored, "conversation_id = ?", created.ConversationID).Error; err != nil {
		t.Fatalf("load stored message: %v", err)
	}
	if stored.AgentType != string(agent.AgentTypeReview) {
		t.Fatalf("stored agent_type = %q, want %q", stored.AgentType, agent.AgentTypeReview)
	}
}

func TestCreateMessage_InvalidAgentType(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)

	created, err := s.CreateConversation(vo.CreateConversationReq{
		UserID: "user_001",
		Title:  "Invalid Agent",
	})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	voCh := make(chan vo.SSEMessageVO, 8)
	err = s.CreateMessage(context.Background(), created.ConversationID, vo.CreateMessageReq{
		UserID:    "user_001",
		Query:     "hello",
		AgentType: "unknown",
	}, voCh)
	if err == nil {
		t.Fatalf("CreateMessage() error = nil, want error")
	}

	var messageCount int64
	if err := s.db.Model(&ChatMessage{}).
		Where("conversation_id = ?", created.ConversationID).
		Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("message count = %d, want 0", messageCount)
	}
}

func TestCreateMessageController_InvalidAgentTypeReturns400(t *testing.T) {
	s, _ := newTestServerWithFakeAgents(t)
	router := NewRouter(s)

	body := `{"user_id":"user_001","query":"hello","agent_type":"unknown"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation/conv-1/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	return NewServer(db, nil)
}

func newTestServerWithFakeAgents(t *testing.T) (*Server, map[agent.AgentType]*fakeRunner) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	runners := make(map[agent.AgentType]*fakeRunner)
	registry, err := agent.NewAgentRegistry(agent.AgentTypeAssistant, agent.DefaultAgentProfiles(), func(profile agent.AgentProfile) agent.Runner {
		runner := &fakeRunner{agentType: profile.Type}
		runners[profile.Type] = runner
		return runner
	})
	if err != nil {
		t.Fatalf("NewAgentRegistry() error = %v", err)
	}

	return NewServer(db, registry), runners
}

type fakeRunner struct {
	agentType       agent.AgentType
	calls           int
	taskCalls       int
	queries         []string
	taskQueries     []string
	contextQueries  []string
	systemContexts  []string
	histories       [][]shared.OpenAIMessage
	runOptions      []agent.RunOptions
	taskResponse    string
	taskResponses   []string
	streamResponses []string
	taskErr         error
}

func (r *fakeRunner) Model() string {
	return "fake-model"
}

func (r *fakeRunner) RunTask(_ context.Context, query string) (agent.RunResult, error) {
	r.taskCalls++
	r.taskQueries = append(r.taskQueries, query)
	if r.taskErr != nil {
		return agent.RunResult{Response: r.taskResponse}, r.taskErr
	}
	if len(r.taskResponses) > 0 {
		response := r.taskResponses[0]
		r.taskResponses = r.taskResponses[1:]
		return agent.RunResult{Response: response}, nil
	}
	if r.taskResponse == "" {
		r.taskResponse = fmt.Sprintf(`{"overall_summary":"fake review from %s","strengths":[],"weaknesses":[],"follow_up_risks":[],"suggested_preparation":[],"questions":[]}`, r.agentType)
	}
	return agent.RunResult{Response: r.taskResponse}, nil
}

func (r *fakeRunner) RunStreamingWithHistory(_ context.Context, _ []shared.OpenAIMessage, query string, _ chan agent.MessageVO, _ chan agent.ConfirmationAction) (agent.RunResult, error) {
	return r.RunStreamingWithContextHistory(context.Background(), agent.DefaultRunOptions(), nil, query, nil, nil)
}

func (r *fakeRunner) RunStreamingWithContextHistory(_ context.Context, options agent.RunOptions, history []shared.OpenAIMessage, query string, _ chan agent.MessageVO, _ chan agent.ConfirmationAction) (agent.RunResult, error) {
	r.calls++
	r.queries = append(r.queries, query)
	r.contextQueries = append(r.contextQueries, query)
	r.systemContexts = append(r.systemContexts, options.SystemContext)
	r.histories = append(r.histories, history)
	r.runOptions = append(r.runOptions, options)
	if r.taskErr != nil {
		return agent.RunResult{Response: r.taskResponse}, r.taskErr
	}
	if len(r.streamResponses) > 0 {
		response := r.streamResponses[0]
		r.streamResponses = r.streamResponses[1:]
		return agent.RunResult{Response: response}, nil
	}
	if len(r.taskResponses) > 0 {
		response := r.taskResponses[0]
		r.taskResponses = r.taskResponses[1:]
		return agent.RunResult{Response: response}, nil
	}
	if r.taskResponse != "" {
		return agent.RunResult{Response: r.taskResponse}, nil
	}
	return agent.RunResult{Response: "fake response from " + string(r.agentType)}, nil
}
