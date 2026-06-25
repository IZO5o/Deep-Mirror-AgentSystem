package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// GET /practice-states
func (s *Server) listPracticeStates(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, vo.Err(400, "user_id is required"))
		return
	}

	result, err := s.ListPracticeStates(userID, c.Query("topic"), c.Query("dimension"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /practice-states/:state_id
func (s *Server) getPracticeState(c *gin.Context) {
	stateID := c.Param("state_id")

	result, err := s.GetPracticeState(stateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
