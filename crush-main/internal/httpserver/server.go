package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/charmbracelet/crush/internal/auth"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server for handling authentication and API requests
type Server struct {
	port   string
	engine *gin.Engine
}

// New creates a new HTTP server with Gin framework
func New(port string) *Server {
	// Set Gin mode (can be set to gin.ReleaseMode in production)
	gin.SetMode(gin.DebugMode)

	engine := gin.Default()

	return &Server{
		port:   port,
		engine: engine,
	}
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the login response body
type LoginResponse struct {
	Success bool      `json:"success"`
	Token   string    `json:"token,omitempty"`
	Message string    `json:"message,omitempty"`
	User    *UserInfo `json:"user,omitempty"`
}

// UserInfo represents user information
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// Start starts the HTTP server with Gin
func (s *Server) Start() error {
	// CORS middleware for development
	s.engine.Use(corsMiddleware())

	// Health check endpoint
	s.engine.GET("/health", s.handleHealth)

	// Authentication endpoints group
	authGroup := s.engine.Group("/api/auth")
	{
		authGroup.POST("/login", s.handleLogin)
		authGroup.GET("/verify", auth.GinAuthMiddleware(), s.handleVerify)
	}

	slog.Info("HTTP server starting with Gin framework", "port", s.port)
	return s.engine.Run(":" + s.port)
}

// corsMiddleware returns a Gin middleware for CORS
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// handleLogin handles login requests
func (s *Server) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Validate credentials
	userStore := auth.GetUserStore()
	user, err := userStore.Authenticate(req.Username, req.Password)
	if err != nil {
		slog.Warn("Login failed", "username", req.Username, "error", err)
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		slog.Error("Failed to generate token", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to generate token",
		})
		return
	}

	slog.Info("User logged in successfully", "username", user.Username)

	// Return success response
	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		Message: "Login successful",
		User: &UserInfo{
			ID:       user.ID,
			Username: user.Username,
		},
	})
}

// handleVerify handles token verification requests
func (s *Server) handleVerify(c *gin.Context) {
	// If we reach here, the token is valid (validated by middleware)
	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "Token is valid",
	})
}
