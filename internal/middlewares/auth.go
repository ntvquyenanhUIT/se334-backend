package middlewares

import (
	"HAB/internal/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	userContextKey     = "userID"
	usernameContextKey = "username"
	emailContextKey    = "email"
)

// AuthMiddleware creates a middleware that enforces authentication.
// It validates the access token from the cookie and sets the userID in the context.
func AuthMiddleware(tokenService *services.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("access_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		claims, err := tokenService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Set(userContextKey, claims.UserID)
		c.Set(usernameContextKey, claims.Username)
		c.Set(emailContextKey, claims.Email)
		c.Next()
	}
}

// OptionalAuthMiddleware creates a middleware that checks for authentication but doesn't enforce it.
// If a valid token is present, it sets the userID in the context. Otherwise, it continues without it.
func OptionalAuthMiddleware(tokenService *services.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("access_token")
		if err != nil || strings.TrimSpace(tokenString) == "" {
			c.Next()
			return
		}

		claims, err := tokenService.ValidateToken(tokenString)
		if err == nil && claims != nil {
			c.Set(userContextKey, claims.UserID)
		}

		c.Next()
	}
}
