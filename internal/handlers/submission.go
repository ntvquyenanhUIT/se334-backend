package handlers

import (
	"HAB/internal/logger"
	"HAB/internal/models"
	"HAB/internal/repositories"
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type SubmissionHandler struct {
	codeRepo repositories.CodeRepository
	redis    *redis.Client
}

// NewSubmissionHandler creates a new submission handler
func NewSubmissionHandler(codeRepo repositories.CodeRepository, redis *redis.Client) *SubmissionHandler {
	return &SubmissionHandler{
		codeRepo: codeRepo,
		redis:    redis,
	}
}

// CreateSubmission handles the submission creation request
func (h *SubmissionHandler) CreateSubmission(c *gin.Context) {
	var req models.SubmissionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := req.ValidateRequest(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	submission := models.Submission{
		UserID:     req.UserID,
		ProblemID:  req.ProblemID,
		LanguageID: req.LanguageID,
		SourceCode: req.SourceCode,
		Status:     models.StatusProcessing,
	}

	if err := h.codeRepo.CreateSubmission(context.Background(), &submission); err != nil {
		logger.Log.Error("Failed to create submission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process submission"})
		return
	}

	err := h.redis.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "code_submissions",
		ID:     "*", // Auto-generate ID
		Values: map[string]interface{}{
			"submission_id": submission.ID,
		},
	}).Err()

	if err != nil {
		logger.Log.Error("Failed to add submission to Redis stream",
			zap.Int("submission_id", submission.ID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue submission"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":       "Submission queued for processing",
		"submission_id": submission.ID,
	})
}

func (h *SubmissionHandler) GetSubmission(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	// Get submission details from database
	submission, err := h.codeRepo.GetSubmissionByID(context.Background(), id)
	if err != nil {
		logger.Log.Error("Failed to get submission",
			zap.Int("submission_id", id),
			zap.Error(err))

		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve submission details"})
		return
	}

	// Prepare simplified response with just the key information
	response := gin.H{
		"status":      submission.Status,
		"source_code": submission.SourceCode,
	}

	// Add wrong test case info if status is wrong answer
	if submission.Status == models.StatusWrongAnswer && submission.WrongTestcase != nil {
		response["wrong_testcase"] = *submission.WrongTestcase
		response["expected_output"] = *submission.ExpectedOutput
	}

	// Include program output for any non-successful submission
	if submission.ProgramOutput != nil &&
		(submission.Status == models.StatusWrongAnswer ||
			submission.Status == models.StatusCompilationError) {
		response["program_output"] = *submission.ProgramOutput
	}

	c.JSON(http.StatusOK, response)
}

func (h *SubmissionHandler) GetUserSubmissions(c *gin.Context) {
	userIDStr := c.Query("user_id")
	problemIDStr := c.Query("problem_id")

	if userIDStr == "" || problemIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Both user_id and problem_id query parameters are required"})
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	problemID, err := strconv.Atoi(problemIDStr)
	if err != nil || problemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid problem ID"})
		return
	}

	submissions, err := h.codeRepo.GetSubmissionsByUserAndProblem(context.Background(), userID, problemID)
	if err != nil {
		logger.Log.Error("Failed to get user submissions",
			zap.Int("user_id", userID),
			zap.Int("problem_id", problemID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve submission history"})
		return
	}

	// Format submission times and add language names
	for i := range submissions {
		// Format time as "Jan 2, 2006 at 3:04 PM"
		submissions[i].FormattedTime = submissions[i].SubmittedAt.Format("Jan 2, 2006 at 3:04 PM")

		// Map language ID to language name
		submissions[i].LanguageName = getLanguageName(submissions[i].LanguageID)
	}

	c.JSON(http.StatusOK, gin.H{
		"submissions": submissions,
		"count":       len(submissions),
	})
}

// Helper function to map language IDs to names
func getLanguageName(languageID int) string {
	switch languageID {
	case 1:
		return "Python"
	case 2:
		return "Go"
	default:
		return "Unknown"
	}
}

func (h *SubmissionHandler) RegisterRoutes(router *gin.Engine) {
	submissionGroup := router.Group("/submissions")
	{
		submissionGroup.POST("", h.CreateSubmission)
		submissionGroup.GET("/:id", h.GetSubmission)
		submissionGroup.GET("", h.GetUserSubmissions)
	}
}
