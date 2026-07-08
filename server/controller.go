package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/shared/log"
	"agent-web-base/vo"
)

func NewRouter(s *Server) *gin.Engine {
	g := gin.Default()

	api := g.Group("/api")
	api.POST("/conversation", s.createConversation)
	api.GET("/conversation", s.listConversations)
	api.PATCH("/conversation/:conversation_id", s.renameConversation)
	api.DELETE("/conversation/:conversation_id", s.deleteConversation)
	api.POST("/conversation/:conversation_id/message", s.createMessage)
	api.GET("/conversation/:conversation_id/message", s.listMessages)
	api.POST("/interviews", s.createInterview)
	api.GET("/interviews", s.listInterviews)
	api.GET("/interviews/:interview_id", s.getInterview)
	api.GET("/interviews/:interview_id/detail", s.getInterviewDetail)
	api.POST("/interviews/:interview_id/media", s.uploadInterviewMedia)
	api.GET("/interviews/:interview_id/media", s.listInterviewMedia)
	api.PUT("/interviews/:interview_id/transcript", s.upsertInterviewTranscript)
	api.GET("/interviews/:interview_id/transcript", s.getInterviewTranscript)
	api.GET("/transcription-jobs/:job_id", s.getTranscriptionJob)
	api.POST("/interviews/:interview_id/review", s.triggerInterviewReview)
	api.GET("/interviews/:interview_id/review", s.getInterviewReview)
	api.GET("/interviews/:interview_id/questions", s.listInterviewQuestions)
	api.GET("/interviews/:interview_id/transcript-segments", s.listTranscriptSegments)
	api.GET("/interviews/:interview_id/selected-context", s.getSelectedContextDebug)
	api.POST("/interviews/:interview_id/memory-candidates", s.generateMemoryCandidates)
	api.GET("/interviews/:interview_id/memory-candidates", s.listMemoryCandidates)
	api.GET("/memory-candidates", s.listMemoryCandidatesGlobal)
	api.POST("/memory-candidates/:candidate_id/accept", s.acceptMemoryCandidate)
	api.POST("/memory-candidates/:candidate_id/reject", s.rejectMemoryCandidate)
	api.GET("/memory-items", s.listMemoryItems)
	api.POST("/interviews/:interview_id/coaching-plan", s.generateCoachingPlan)
	api.GET("/interviews/:interview_id/coaching-plan", s.getCoachingPlan)
	api.GET("/coaching-plans/:plan_id/tasks", s.listCoachingTasks)
	api.POST("/coaching-plans/:plan_id/sessions", s.startOrResumeCoachingSession)
	api.GET("/coaching-sessions/:session_id", s.getCoachingSession)
	api.POST("/coaching-sessions/:session_id/turns", s.submitCoachingSessionTurn)
	api.POST("/coaching-sessions/:session_id/resume", s.resumeFailedCoachingSession)
	api.POST("/coaching-sessions/:session_id/memory-candidates", s.generateMemoryCandidatesFromCoachingSession)
	api.POST("/coaching-sessions/:session_id/pause", s.pauseCoachingSession)
	api.POST("/coaching-sessions/:session_id/cancel", s.cancelCoachingSession)
	api.PATCH("/coaching-tasks/:task_id", s.updateCoachingTask)
	api.POST("/interviews/:interview_id/mock-interviews", s.startMockInterview)
	api.POST("/mock-interviews/:mock_id/turns", s.submitMockTurn)
	api.POST("/mock-interviews/:mock_id/resume", s.resumeFailedMockInterview)
	api.GET("/mock-interviews/:mock_id", s.getMockInterview)
	api.GET("/mock-interviews/:mock_id/turns", s.listMockTurns)
	api.POST("/mock-interviews/:mock_id/complete", s.completeMockInterview)
	api.POST("/mock-interviews/:mock_id/cancel", s.cancelMockInterview)
	api.POST("/mock-interviews/:mock_id/memory-candidates", s.generateMemoryCandidatesFromMockInterview)
	api.POST("/practice-goals", s.createPracticeGoal)
	api.GET("/practice-goals", s.listPracticeGoals)
	api.GET("/practice-goals/:goal_id", s.getPracticeGoal)
	api.PATCH("/practice-goals/:goal_id", s.updatePracticeGoal)
	api.POST("/practice-goals/:goal_id/archive", s.archivePracticeGoal)
	api.POST("/practice-goals/:goal_id/coaching-plan", s.generatePracticeGoalCoachingPlan)
	api.POST("/practice-goals/:goal_id/mock", s.startPracticeGoalMockInterview)
	api.GET("/practice-states", s.listPracticeStates)
	api.GET("/practice-states/:state_id", s.getPracticeState)
	api.GET("/agent-decision-traces", s.listAgentDecisionTraces)
	api.GET("/agent-evaluations", s.listAgentEvaluations)
	api.GET("/dashboard-summary", s.getDashboardSummary)

	return g
}

// POST /conversation
func (s *Server) createConversation(c *gin.Context) {
	var req vo.CreateConversationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.CreateConversation(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /conversation
func (s *Server) listConversations(c *gin.Context) {
	userID := c.Query("user_id")

	result, err := s.ListConversations(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// PATCH /conversation/:conversation_id
func (s *Server) renameConversation(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	var req vo.UpdateConversationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.RenameConversation(conversationID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// DELETE /conversation/:conversation_id
func (s *Server) deleteConversation(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	if err := s.DeleteConversation(conversationID); err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(map[string]any{"conversation_id": conversationID}))
}

// GET /conversation/:conversation_id/message
func (s *Server) listMessages(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	result, err := s.ListMessages(conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /conversation/:conversation_id/message
// 创建新消息并 SSE 流式输出 agent 响应
func (s *Server) createMessage(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	var req vo.CreateMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}
	if _, err := s.ResolveAgentType(req.AgentType); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	eventCh := make(chan vo.SSEMessageVO, 64)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	go func() {
		defer close(eventCh)
		if err := s.CreateMessage(c.Request.Context(), conversationID, req, eventCh); err != nil {
			errMsg := err.Error()
			eventCh <- vo.SSEMessageVO{Event: vo.SSETypeError, Content: &errMsg}
			return
		}
	}()

	for {
		select {
		case <-c.Request.Context().Done():
			log.Warn("Server is shutting down. Exiting...")
			return
		case e, ok := <-eventCh:
			if !ok {
				return
			}
			c.SSEvent("message", e)
			c.Writer.Flush()
		}
	}
}
