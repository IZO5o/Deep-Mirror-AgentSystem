package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// POST /interviews
func (s *Server) createInterview(c *gin.Context) {
	var req vo.CreateInterviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.CreateInterview(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews
func (s *Server) listInterviews(c *gin.Context) {
	userID := c.Query("user_id")

	result, err := s.ListInterviews(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id
func (s *Server) getInterview(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.GetInterview(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// PUT /interviews/:interview_id/transcript
func (s *Server) upsertInterviewTranscript(c *gin.Context) {
	interviewID := c.Param("interview_id")

	var req vo.UpsertInterviewTranscriptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.UpsertInterviewTranscript(interviewID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/transcript
func (s *Server) getInterviewTranscript(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.GetInterviewTranscript(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
