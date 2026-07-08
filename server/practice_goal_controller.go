package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"agent-web-base/vo"
)

func writePracticeGoalError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, vo.Err(http.StatusNotFound, err.Error()))
		return
	}
	message := err.Error()
	if strings.Contains(message, "required") ||
		strings.Contains(message, "unsupported practice goal status") ||
		strings.Contains(message, "practice goal user_id mismatch") ||
		strings.Contains(message, "practice goal status must be") {
		c.JSON(http.StatusBadRequest, vo.Err(http.StatusBadRequest, message))
		return
	}
	c.JSON(http.StatusInternalServerError, vo.Err(http.StatusInternalServerError, message))
}

func (s *Server) createPracticeGoal(c *gin.Context) {
	var req vo.CreatePracticeGoalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(http.StatusBadRequest, err.Error()))
		return
	}
	result, err := s.CreatePracticeGoal(req)
	if err != nil {
		writePracticeGoalError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}

func (s *Server) listPracticeGoals(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, vo.Err(http.StatusBadRequest, "user_id is required"))
		return
	}
	result, err := s.ListPracticeGoals(userID, c.Query("status"))
	if err != nil {
		writePracticeGoalError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}

func (s *Server) getPracticeGoal(c *gin.Context) {
	result, err := s.GetPracticeGoal(c.Param("goal_id"))
	if err != nil {
		writePracticeGoalError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}

func (s *Server) updatePracticeGoal(c *gin.Context) {
	var req vo.UpdatePracticeGoalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(http.StatusBadRequest, err.Error()))
		return
	}
	result, err := s.UpdatePracticeGoal(c.Param("goal_id"), req)
	if err != nil {
		writePracticeGoalError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}

func (s *Server) archivePracticeGoal(c *gin.Context) {
	result, err := s.ArchivePracticeGoal(c.Param("goal_id"))
	if err != nil {
		writePracticeGoalError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}

func (s *Server) generatePracticeGoalCoachingPlan(c *gin.Context) {
	var req vo.GeneratePracticeGoalCoachingPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(http.StatusBadRequest, err.Error()))
		return
	}
	result, err := s.GeneratePracticeGoalCoachingPlan(c.Request.Context(), c.Param("goal_id"), req)
	if err != nil {
		writePracticeGoalError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}

func (s *Server) startPracticeGoalMockInterview(c *gin.Context) {
	var req vo.StartPracticeGoalMockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(http.StatusBadRequest, err.Error()))
		return
	}
	result, err := s.StartPracticeGoalMockInterview(c.Request.Context(), c.Param("goal_id"), req)
	if err != nil {
		writePracticeGoalError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.OK(result))
}
