package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// POST /interviews/:interview_id/memory-candidates
func (s *Server) generateMemoryCandidates(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.GenerateMemoryCandidates(c.Request.Context(), interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/memory-candidates
func (s *Server) listMemoryCandidates(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.ListMemoryCandidates(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /memory-candidates
func (s *Server) listMemoryCandidatesGlobal(c *gin.Context) {
	userID := strings.TrimSpace(c.Query("user_id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, vo.Err(400, "user_id is required"))
		return
	}

	result, err := s.ListMemoryCandidatesByQuery(MemoryCandidateQuery{
		UserID:        userID,
		Status:        c.Query("status"),
		SourceRefType: c.Query("source_ref_type"),
		CompanyName:   c.Query("company_name"),
		JobTitle:      c.Query("job_title"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /coaching-sessions/:session_id/memory-candidates
func (s *Server) generateMemoryCandidatesFromCoachingSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	result, err := s.GenerateMemoryCandidatesFromCoachingSession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /mock-interviews/:mock_id/memory-candidates
func (s *Server) generateMemoryCandidatesFromMockInterview(c *gin.Context) {
	mockID := c.Param("mock_id")

	result, err := s.GenerateMemoryCandidatesFromMockInterview(c.Request.Context(), mockID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /memory-candidates/:candidate_id/accept
func (s *Server) acceptMemoryCandidate(c *gin.Context) {
	candidateID := c.Param("candidate_id")

	result, err := s.AcceptMemoryCandidate(candidateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// POST /memory-candidates/:candidate_id/reject
func (s *Server) rejectMemoryCandidate(c *gin.Context) {
	candidateID := c.Param("candidate_id")

	result, err := s.RejectMemoryCandidate(candidateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /memory-items
func (s *Server) listMemoryItems(c *gin.Context) {
	userID := c.Query("user_id")

	result, err := s.ListMemoryItems(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
