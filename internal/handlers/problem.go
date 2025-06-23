package handlers

import (
	"HAB/internal/logger"
	"HAB/internal/repositories"
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ProblemHandler struct {
	problemRepo repositories.ProblemRepository
}

// NewProblemHandler creates a new problem handler
func NewProblemHandler(problemRepo repositories.ProblemRepository) *ProblemHandler {
	return &ProblemHandler{
		problemRepo: problemRepo,
	}
}

// GetProblems returns a list of all problems with minimal information
func (h *ProblemHandler) GetProblems(c *gin.Context) {
	problems, err := h.problemRepo.GetProblems(context.Background())
	if err != nil {
		logger.Log.Error("Failed to get problems", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve problems"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"problems": problems,
	})
}

// GetProblemByID returns detailed information about a specific problem
func (h *ProblemHandler) GetProblemByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid problem ID"})
		return
	}

	problem, err := h.problemRepo.GetProblemByID(context.Background(), id)
	if err != nil {
		logger.Log.Error("Failed to get problem",
			zap.Int("problem_id", id),
			zap.Error(err))

		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Problem not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve problem details"})
		return
	}

	c.JSON(http.StatusOK, problem)
}

// RegisterRoutes registers the problem handler routes
func (h *ProblemHandler) RegisterRoutes(router *gin.Engine) {
	problemGroup := router.Group("/problems")
	{
		problemGroup.GET("", h.GetProblems)
		problemGroup.GET("/:id", h.GetProblemByID)
	}
}
