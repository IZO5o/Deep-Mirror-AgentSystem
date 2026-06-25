package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agent-web-base/vo"
)

// POST /interviews/:interview_id/media
func (s *Server) uploadInterviewMedia(c *gin.Context) {
	interviewID := c.Param("interview_id")
	userID := c.PostForm("user_id")
	language := c.PostForm("language")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
		return
	}

	result, err := s.UploadInterviewMedia(interviewID, userID, language, fileHeader)
	if err != nil {
		if isBadRequestError(err) {
			c.JSON(http.StatusBadRequest, vo.Err(400, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusAccepted, vo.OK(result))
}

// GET /transcription-jobs/:job_id
func (s *Server) getTranscriptionJob(c *gin.Context) {
	jobID := c.Param("job_id")

	result, err := s.GetTranscriptionJob(jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}

// GET /interviews/:interview_id/media
func (s *Server) listInterviewMedia(c *gin.Context) {
	interviewID := c.Param("interview_id")

	result, err := s.ListInterviewMedia(interviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, vo.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, vo.OK(result))
}
