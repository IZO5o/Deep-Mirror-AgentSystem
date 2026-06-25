package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// POST /interviews/:interview_id/coaching-plan
func (s *Server) generateCoachingPlan(c *gin.Context) {
	interviewID := c.Param("interview_id")

	var req vo.GenerateCoachingPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.GenerateCoachingPlan(c.Request.Context(), interviewID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/coaching-plan
func (s *Server) getCoachingPlan(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.GetCoachingPlan(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /coaching-plans/:plan_id/tasks
func (s *Server) listCoachingTasks(c *gin.Context) {
	planID := c.Param("plan_id")

	result, err := s.ListCoachingTasks(planID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /coaching-plans/:plan_id/sessions
func (s *Server) startOrResumeCoachingSession(c *gin.Context) {
	planID := c.Param("plan_id")
	userID := c.Query("user_id")

	result, err := s.StartOrResumeCoachingSession(planID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /coaching-sessions/:session_id
func (s *Server) getCoachingSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	result, err := s.GetCoachingSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /coaching-sessions/:session_id/turns
func (s *Server) submitCoachingSessionTurn(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req vo.SubmitCoachingSessionTurnReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.SubmitCoachingSessionTurn(c.Request.Context(), sessionID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /coaching-sessions/:session_id/pause
func (s *Server) pauseCoachingSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	result, err := s.PauseCoachingSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /coaching-sessions/:session_id/cancel
func (s *Server) cancelCoachingSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	result, err := s.CancelCoachingSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// PATCH /coaching-tasks/:task_id
func (s *Server) updateCoachingTask(c *gin.Context) {
	taskID := c.Param("task_id")

	var req vo.UpdateCoachingTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.UpdateCoachingTask(taskID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
