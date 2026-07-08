package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// POST /interviews/:interview_id/mock-interviews
func (s *Server) startMockInterview(c *gin.Context) {
	interviewID := c.Param("interview_id")

	var req vo.StartMockInterviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.StartMockInterview(c.Request.Context(), interviewID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /mock-interviews/:mock_id/turns
func (s *Server) submitMockTurn(c *gin.Context) {
	mockID := c.Param("mock_id")

	var req vo.SubmitMockTurnReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.SubmitMockTurn(c.Request.Context(), mockID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /mock-interviews/:mock_id
func (s *Server) getMockInterview(c *gin.Context) {
	mockID := c.Param("mock_id")

	result, err := s.GetMockInterview(mockID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /mock-interviews/:mock_id/turns
func (s *Server) listMockTurns(c *gin.Context) {
	mockID := c.Param("mock_id")

	result, err := s.ListMockTurns(mockID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /mock-interviews/:mock_id/complete
func (s *Server) completeMockInterview(c *gin.Context) {
	mockID := c.Param("mock_id")

	result, err := s.CompleteMockInterview(c.Request.Context(), mockID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /mock-interviews/:mock_id/resume
func (s *Server) resumeFailedMockInterview(c *gin.Context) {
	mockID := c.Param("mock_id")

	result, err := s.ResumeFailedMockInterview(c.Request.Context(), mockID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /mock-interviews/:mock_id/cancel
func (s *Server) cancelMockInterview(c *gin.Context) {
	mockID := c.Param("mock_id")

	result, err := s.CancelMockInterview(mockID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
