package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

func (s *Server) listAgentEvaluations(c *gin.Context) {
	result, err := s.EvaluateAgentDecisionTraces(AgentDecisionTraceQuery{
		UserID:      c.Query("user_id"),
		InterviewID: c.Query("interview_id"),
		SourceType:  c.Query("source_type"),
		SourceID:    c.Query("source_id"),
		AgentType:   c.Query("agent_type"),
		StepName:    c.Query("step_name"),
		Status:      c.Query("status"),
		Limit:       parseTraceLimit(c.Query("limit")),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}
