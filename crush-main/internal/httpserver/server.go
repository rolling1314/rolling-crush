package httpserver

import (
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

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

// FileNode represents a file or folder in the file tree
type FileNode struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"` // "file" or "folder"
	Path     string     `json:"path"`
	Content  string     `json:"content,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

var idCounter = 0

// Start starts the HTTP server with Gin
func (s *Server) Start() error {
	// CORS middleware for development
	s.engine.Use(corsMiddleware())

	// Health check endpoint
	s.engine.GET("/health", s.handleHealth)

	// API endpoints group
	apiGroup := s.engine.Group("/api")
	{
		// Authentication endpoints
		authGroup := apiGroup.Group("/auth")
		{
			authGroup.POST("/login", s.handleLogin)
			authGroup.GET("/verify", auth.GinAuthMiddleware(), s.handleVerify)
		}

		// File tree endpoint (requires authentication)
		apiGroup.GET("/files", auth.GinAuthMiddleware(), s.handleGetFiles)
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

// handleGetFiles handles file tree requests
func (s *Server) handleGetFiles(c *gin.Context) {
	// Get path from query parameter, default to current directory
	targetPath := c.DefaultQuery("path", ".")

	// Reset ID counter
	idCounter = 0

	// Get absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid path: " + err.Error(),
		})
		return
	}

	// Build file tree
	fileTree, err := buildFileTree(absPath, absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to build file tree: " + err.Error(),
		})
		return
	}

	slog.Info("File tree generated", "path", absPath, "user", c.GetString("username"))
	c.JSON(http.StatusOK, fileTree)
}

// generateID generates a unique ID for file nodes
func generateID() string {
	idCounter++
	return strconv.Itoa(idCounter)
}

// buildFileTree recursively builds the file tree structure
func buildFileTree(path string, rootPath string) (*FileNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Calculate relative path
	relativePath, err := filepath.Rel(rootPath, path)
	if err != nil {
		relativePath = path
	}
	if relativePath == "." {
		relativePath = ""
	}

	node := &FileNode{
		ID:   generateID(),
		Name: info.Name(),
		Path: "/" + filepath.ToSlash(relativePath),
	}

	if info.IsDir() {
		node.Type = "folder"

		// Read folder contents
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}

		node.Children = []FileNode{}
		for _, file := range files {
			// Skip hidden files and common ignore patterns
			if shouldIgnoreFile(file.Name()) {
				continue
			}

			childPath := filepath.Join(path, file.Name())
			childNode, err := buildFileTree(childPath, rootPath)
			if err != nil {
				slog.Warn("Failed to read file", "path", childPath, "error", err)
				continue // Skip files that cannot be read
			}
			node.Children = append(node.Children, *childNode)
		}
	} else {
		node.Type = "file"

		// Read file content (limit size to avoid memory issues)
		if info.Size() < 1024*1024 { // Only read files smaller than 1MB
			content, err := ioutil.ReadFile(path)
			if err == nil {
				node.Content = string(content)
			}
		}
	}

	return node, nil
}

// shouldIgnoreFile checks if a file should be ignored
func shouldIgnoreFile(name string) bool {
	ignorePatterns := []string{
		".git",
		".DS_Store",
		"node_modules",
		".idea",
		".vscode",
		"__pycache__",
		".pytest_cache",
		".pyc",
		".pyo",
		".env",
		".env.local",
	}

	for _, pattern := range ignorePatterns {
		if name == pattern {
			return true
		}
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}

	return false
}
