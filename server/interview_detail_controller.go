package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// GET /interviews/:interview_id/detail
func (s *Server) getInterviewDetail(c *gin.Context) {
	interviewID := c.Param("interview_id")
	userID := c.Query("user_id")

	result, err := s.GetInterviewDetail(interviewID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
