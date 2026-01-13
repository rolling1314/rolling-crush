package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware is a middleware that validates JWT tokens (for standard http.HandlerFunc)
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}
		
		// Expected format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}
		
		token := parts[1]
		claims, err := ValidateToken(token)
		if err != nil {
			slog.Error("Token validation failed", "error", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
		
		slog.Info("User authenticated", "user_id", claims.UserID, "username", claims.Username)
		
		// Token is valid, proceed to the next handler
		next.ServeHTTP(w, r)
	}
}

// GinAuthMiddleware is a Gin middleware that validates JWT tokens
func GinAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}
		
		// Expected format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}
		
		token := parts[1]
		claims, err := ValidateToken(token)
		if err != nil {
			slog.Error("Token validation failed", "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}
		
		slog.Info("User authenticated", "user_id", claims.UserID, "username", claims.Username)
		
		// Store user info in context for use in handlers
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		
		// Token is valid, proceed to the next handler
		c.Next()
	}
}

