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

func NewSubmissionHandler(codeRepo repositories.CodeRepository, redis *redis.Client) *SubmissionHandler {
	return &SubmissionHandler{
		codeRepo: codeRepo,
		redis:    redis,
	}
}

func (h *SubmissionHandler) CreateSubmission(c *gin.Context) {
	var req models.SubmissionRequest

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := req.ValidateRequest(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	submission := models.Submission{
		UserID:     userID.(int),
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
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idStr := c.Param("id")
	submissionID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	submission, err := h.codeRepo.GetSubmissionByID(context.Background(), submissionID, userID.(int))
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "access denied") {
			// This is not a server error, but a client-side error (e.g., wrong ID or permission issue).
			// Logged as Info for auditing purposes.
			logger.Log.Info("User attempted to access a non-existent or forbidden submission",
				zap.Int("submission_id", submissionID),
				zap.Any("user_id", userID),
				zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found or access denied"})
			return
		}

		// For other types of errors, it's a potential server issue.
		logger.Log.Error("Failed to get submission",
			zap.Int("submission_id", submissionID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve submission details"})
		return
	}

	response := gin.H{
		"status":      submission.Status,
		"source_code": submission.SourceCode,
	}

	if submission.Status == models.StatusWrongAnswer && submission.WrongTestcase != nil {
		response["wrong_testcase"] = *submission.WrongTestcase
		response["expected_output"] = *submission.ExpectedOutput
	}

	if submission.ProgramOutput != nil &&
		(submission.Status == models.StatusWrongAnswer ||
			submission.Status == models.StatusCompilationError) {
		response["program_output"] = *submission.ProgramOutput
	}

	c.JSON(http.StatusOK, response)
}

func (h *SubmissionHandler) GetUserSubmissions(c *gin.Context) {
	problemIDStr := c.Query("problem_id")

	if problemIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "problem_id query parameter is required"})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	problemID, err := strconv.Atoi(problemIDStr)
	if err != nil || problemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid problem ID"})
		return
	}

	submissions, err := h.codeRepo.GetSubmissionsByUserAndProblem(context.Background(), userID.(int), problemID)
	if err != nil {
		logger.Log.Error("Failed to get user submissions",
			zap.Int("user_id", userID.(int)),
			zap.Int("problem_id", problemID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve submission history"})
		return
	}

	for i := range submissions {
		submissions[i].FormattedTime = submissions[i].SubmittedAt.Format("02/01/2006 3:04PM")

		submissions[i].LanguageName = getLanguageName(submissions[i].LanguageID)
	}

	c.JSON(http.StatusOK, gin.H{
		"submissions": submissions,
		"count":       len(submissions),
	})
}

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

func (h *SubmissionHandler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	submissionGroup := router.Group("/submissions")
	submissionGroup.Use(authMiddleware)
	{
		submissionGroup.POST("", h.CreateSubmission)
		submissionGroup.GET("/:id", h.GetSubmission)
		submissionGroup.GET("", h.GetUserSubmissions)
	}
}
