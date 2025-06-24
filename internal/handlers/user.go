package handlers

import (
	"HAB/internal/logger"
	"HAB/internal/models"
	"HAB/internal/repositories"
	"HAB/internal/services"
	"HAB/internal/utils"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthHandler struct {
	userRepo     repositories.UserRepository
	tokenService *services.TokenService
}

func NewAuthHandler(userRepo repositories.UserRepository, tokenService *services.TokenService) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		tokenService: tokenService,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := h.userRepo.CreateUser(context.Background(), &req); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
			return
		}
		logger.Log.Error("Failed to create user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	user, err := h.userRepo.GetUserByEmail(context.Background(), req.Email)
	if err != nil || !utils.CheckPasswordHash(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid credentials"})
		return
	}

	accessToken, refreshToken, err := h.tokenService.GenerateTokens(user.ID)
	if err != nil {
		logger.Log.Error("Failed to generate tokens", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log in"})
		return
	}

	// Store refresh token in DB
	refreshExpiresAt := time.Now().Add(time.Hour * 24 * 14)
	if err := h.userRepo.StoreRefreshToken(context.Background(), user.ID, refreshToken, refreshExpiresAt); err != nil {
		logger.Log.Error("Failed to store refresh token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log in"})
		return
	}

	c.SetCookie("access_token", accessToken, 3600, "/", "", false, true)
	c.SetCookie("refresh_token", refreshToken, 3600*24*14, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// still processing request even user has not logged in (is not important as button does not show for not logged-in user)
func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err == nil && refreshToken != "" {
		if err := h.userRepo.RevokeToken(context.Background(), refreshToken); err != nil {
			logger.Log.Warn("Failed to revoke token on logout", zap.Error(err))
		}
	}

	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) Verify(c *gin.Context) {
	// 1. Check for access token
	accessToken, err := c.Cookie("access_token")
	if err == nil {
		claims, err := h.tokenService.ValidateToken(accessToken)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"is_authenticated": true, "user_id": claims.UserID})
			return
		}
	}

	// 2. If access token is invalid/missing, check for refresh token
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"is_authenticated": false, "error": "Authorization required"})
		return
	}

	// 3. Validate refresh token from DB
	dbToken, err := h.userRepo.GetRefreshToken(context.Background(), refreshToken)
	if err != nil || dbToken.IsRevoked || time.Now().After(dbToken.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"is_authenticated": false, "error": "Invalid session"})
		return
	}

	// 4. Validate refresh token signature
	claims, err := h.tokenService.ValidateToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"is_authenticated": false, "error": "Invalid token"})
		return
	}

	// 5. Issue new access token
	newAccessToken, _, err := h.tokenService.GenerateTokens(claims.UserID)
	if err != nil {
		logger.Log.Error("Failed to generate new access token during verify", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"is_authenticated": false, "error": "Could not refresh session"})
		return
	}

	c.SetCookie("access_token", newAccessToken, 3600, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"is_authenticated": true, "user_id": claims.UserID})
}

func (h *AuthHandler) RegisterRoutes(router *gin.Engine) {
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)
		authGroup.POST("/logout", h.Logout)
		authGroup.GET("/verify", h.Verify)
	}
}
