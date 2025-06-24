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

func NewProblemHandler(problemRepo repositories.ProblemRepository) *ProblemHandler {
	return &ProblemHandler{
		problemRepo: problemRepo,
	}
}

func (h *ProblemHandler) GetProblems(c *gin.Context) {
	problems, err := h.problemRepo.GetProblems(context.Background())
	if err != nil {
		logger.Log.Error("Failed to get problems", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve problems"})
		return
	}

	if userID, exists := c.Get("userID"); exists {
		solvedMap, err := h.problemRepo.GetSolvedProblemIDs(context.Background(), userID.(int))
		if err != nil {
			logger.Log.Warn("Failed to get solved problem IDs", zap.Error(err))
		} else {
			for i := range problems {
				if _, solved := solvedMap[problems[i].ID]; solved {
					problems[i].IsSolved = true
				}
			}
		}
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

	if userID, exists := c.Get("userID"); exists {
		solvedMap, err := h.problemRepo.GetSolvedProblemIDs(context.Background(), userID.(int))
		if err != nil {
			logger.Log.Warn("Failed to get solved problem IDs for single problem", zap.Error(err))
		} else {
			if _, solved := solvedMap[problem.ID]; solved {
				problem.IsSolved = true
			}
		}
	}

	c.JSON(http.StatusOK, problem)
}

func (h *ProblemHandler) RegisterRoutes(router *gin.Engine, optionalAuthMiddleware gin.HandlerFunc) {
	problemGroup := router.Group("/problems")
	problemGroup.Use(optionalAuthMiddleware)
	{
		problemGroup.GET("", h.GetProblems)
		problemGroup.GET("/:id", h.GetProblemByID)
	}
}
