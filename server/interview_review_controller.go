package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// POST /interviews/:interview_id/review
func (s *Server) triggerInterviewReview(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.TriggerInterviewReview(c.Request.Context(), interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/review
func (s *Server) getInterviewReview(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.GetInterviewReview(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/questions
func (s *Server) listInterviewQuestions(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.ListInterviewQuestions(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/transcript-segments
func (s *Server) listTranscriptSegments(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.ListTranscriptSegments(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/selected-context
func (s *Server) getSelectedContextDebug(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.GetSelectedContextDebug(
		interviewID,
		c.Query("user_id"),
		c.Query("target_round"),
		c.Query("current_task"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
