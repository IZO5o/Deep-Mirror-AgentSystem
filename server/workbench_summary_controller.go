package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// GET /dashboard-summary
func (s *Server) getDashboardSummary(c *gin.Context) {
	userID := strings.TrimSpace(c.Query("user_id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, vo.Err(400, "user_id is required"))
		return
	}

	result, err := s.GetDashboardSummary(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
