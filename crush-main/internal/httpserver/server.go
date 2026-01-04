package httpserver

import (
	"database/sql"
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/charmbracelet/crush/internal/auth"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/project"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/user"
	"github.com/gin-gonic/gin"
)

type Server struct {
	port           string
	engine         *gin.Engine
	userService    user.Service
	projectService project.Service
	sessionService session.Service
	messageService message.Service
	db             *db.Queries
	config         *config.Config
}

func New(port string, userService user.Service, projectService project.Service, sessionService session.Service, messageService message.Service, queries *db.Queries, cfg *config.Config) *Server {
	gin.SetMode(gin.DebugMode)
	engine := gin.Default()

	return &Server{
		port:           port,
		engine:         engine,
		userService:    userService,
		projectService: projectService,
		sessionService: sessionService,
		messageService: messageService,
		db:             queries,
		config:         cfg,
	}
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Success bool      `json:"success"`
	Token   string    `json:"token,omitempty"`
	Message string    `json:"message,omitempty"`
	User    *UserInfo `json:"user,omitempty"`
}

type UserInfo struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type ProjectRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	Host          string `json:"host" binding:"required"`
	Port          int32  `json:"port" binding:"required"`
	WorkspacePath string `json:"workspace_path" binding:"required"`
}

type ProjectResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Host          string `json:"host"`
	Port          int32  `json:"port"`
	WorkspacePath string `json:"workspace_path"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type SessionResponse struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"project_id"`
	Title            string  `json:"title"`
	MessageCount     int64   `json:"message_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

type SessionModelConfig struct {
	Provider        string   `json:"provider" binding:"required"`
	Model           string   `json:"model" binding:"required"`
	BaseURL         string   `json:"base_url"`
	APIKey          string   `json:"api_key"`
	MaxTokens       *int64   `json:"max_tokens"`
	Temperature     *float64 `json:"temperature"`
	TopP            *float64 `json:"top_p"`
	ReasoningEffort string   `json:"reasoning_effort"`
	Think           bool     `json:"think"`
}

type CreateSessionRequest struct {
	ProjectID   string              `json:"project_id" binding:"required"`
	Title       string              `json:"title" binding:"required"`
	ModelConfig *SessionModelConfig `json:"model_config" binding:"required"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type FileNode struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Path     string     `json:"path"`
	Content  string     `json:"content,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

var idCounter = 0

func (s *Server) Start() error {
	s.engine.Use(corsMiddleware())

	s.engine.GET("/health", s.handleHealth)

	apiGroup := s.engine.Group("/api")
	{
		authGroup := apiGroup.Group("/auth")
		{
			authGroup.POST("/register", s.handleRegister)
			authGroup.POST("/login", s.handleLogin)
			authGroup.GET("/verify", auth.GinAuthMiddleware(), s.handleVerify)
		}

		projectGroup := apiGroup.Group("/projects")
		projectGroup.Use(auth.GinAuthMiddleware())
		{
			projectGroup.POST("", s.handleCreateProject)
			projectGroup.GET("", s.handleListProjects)
			projectGroup.GET("/:id", s.handleGetProject)
			projectGroup.PUT("/:id", s.handleUpdateProject)
			projectGroup.DELETE("/:id", s.handleDeleteProject)
			projectGroup.GET("/:id/sessions", s.handleGetProjectSessions)
		}

		sessionGroup := apiGroup.Group("/sessions")
		sessionGroup.Use(auth.GinAuthMiddleware())
		{
			sessionGroup.POST("", s.handleCreateSession)
			sessionGroup.GET("/:id/messages", s.handleGetSessionMessages)
		}

		// Providers and models endpoints
		apiGroup.GET("/providers", auth.GinAuthMiddleware(), s.handleGetProviders)
		apiGroup.GET("/providers/:provider/models", auth.GinAuthMiddleware(), s.handleGetProviderModels)

		apiGroup.GET("/files", auth.GinAuthMiddleware(), s.handleGetFiles)
	}

	slog.Info("HTTP server starting", "port", s.port)
	return s.engine.Run(":" + s.port)
}

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

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (s *Server) handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	user, err := s.userService.Create(c.Request.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		User: &UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	user, err := s.userService.VerifyPassword(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		User: &UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

func (s *Server) handleVerify(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"valid": true})
}

func (s *Server) handleCreateProject(c *gin.Context) {
	userID := c.GetString("user_id")
	var req ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	proj, err := s.projectService.Create(c.Request.Context(), userID, req.Name, req.Description, req.Host, req.WorkspacePath, req.Port)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ProjectResponse{
		ID:            proj.ID,
		Name:          proj.Name,
		Description:   proj.Description.String,
		Host:          proj.Host,
		Port:          proj.Port,
		WorkspacePath: proj.WorkspacePath,
		CreatedAt:     proj.CreatedAt,
		UpdatedAt:     proj.UpdatedAt,
	})
}

func (s *Server) handleListProjects(c *gin.Context) {
	userID := c.GetString("user_id")
	projects, err := s.projectService.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]ProjectResponse, len(projects))
	for i, proj := range projects {
		response[i] = ProjectResponse{
			ID:            proj.ID,
			Name:          proj.Name,
			Description:   proj.Description.String,
			Host:          proj.Host,
			Port:          proj.Port,
			WorkspacePath: proj.WorkspacePath,
			CreatedAt:     proj.CreatedAt,
			UpdatedAt:     proj.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleGetProject(c *gin.Context) {
	projectID := c.Param("id")
	proj, err := s.projectService.GetByID(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Project not found"})
		return
	}

	c.JSON(http.StatusOK, ProjectResponse{
		ID:            proj.ID,
		Name:          proj.Name,
		Description:   proj.Description.String,
		Host:          proj.Host,
		Port:          proj.Port,
		WorkspacePath: proj.WorkspacePath,
		CreatedAt:     proj.CreatedAt,
		UpdatedAt:     proj.UpdatedAt,
	})
}

func (s *Server) handleUpdateProject(c *gin.Context) {
	projectID := c.Param("id")
	var req ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	proj, err := s.projectService.Update(c.Request.Context(), project.Project{
		ID:            projectID,
		Name:          req.Name,
		Description:   sql.NullString{String: req.Description, Valid: req.Description != ""},
		Host:          req.Host,
		Port:          req.Port,
		WorkspacePath: req.WorkspacePath,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ProjectResponse{
		ID:            proj.ID,
		Name:          proj.Name,
		Description:   proj.Description.String,
		Host:          proj.Host,
		Port:          proj.Port,
		WorkspacePath: proj.WorkspacePath,
		CreatedAt:     proj.CreatedAt,
		UpdatedAt:     proj.UpdatedAt,
	})
}

func (s *Server) handleDeleteProject(c *gin.Context) {
	projectID := c.Param("id")
	if err := s.projectService.Delete(c.Request.Context(), projectID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleGetProjectSessions(c *gin.Context) {
	projectID := c.Param("id")
	sessions, err := s.sessionService.List(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]SessionResponse, len(sessions))
	for i, sess := range sessions {
		response[i] = SessionResponse{
			ID:               sess.ID,
			ProjectID:        sess.ProjectID,
			Title:            sess.Title,
			MessageCount:     sess.MessageCount,
			PromptTokens:     sess.PromptTokens,
			CompletionTokens: sess.CompletionTokens,
			Cost:             sess.Cost,
			CreatedAt:        sess.CreatedAt,
			UpdatedAt:        sess.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleCreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	sess, err := s.sessionService.Create(c.Request.Context(), req.ProjectID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Save model config to database
	if req.ModelConfig != nil {
		config := db.SessionConfigParams{
			Provider:        req.ModelConfig.Provider,
			Model:           req.ModelConfig.Model,
			BaseURL:         req.ModelConfig.BaseURL,
			APIKey:          req.ModelConfig.APIKey,
			MaxTokens:       req.ModelConfig.MaxTokens,
			Temperature:     req.ModelConfig.Temperature,
			TopP:            req.ModelConfig.TopP,
			ReasoningEffort: req.ModelConfig.ReasoningEffort,
			Think:           req.ModelConfig.Think,
		}

		if err := s.db.CreateSessionModelConfig(c.Request.Context(), sess.ID, config); err != nil {
			slog.Error("Failed to save session model config", "error", err, "session_id", sess.ID)
			// Don't fail the request, just log the error
		} else {
			slog.Info("Saved model config for session", "session_id", sess.ID, "provider", config.Provider, "model", config.Model)
		}
	}

	c.JSON(http.StatusOK, SessionResponse{
		ID:               sess.ID,
		ProjectID:        sess.ProjectID,
		Title:            sess.Title,
		MessageCount:     sess.MessageCount,
		PromptTokens:     sess.PromptTokens,
		CompletionTokens: sess.CompletionTokens,
		Cost:             sess.Cost,
		CreatedAt:        sess.CreatedAt,
		UpdatedAt:        sess.UpdatedAt,
	})
}

func (s *Server) handleGetSessionMessages(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required"})
		return
	}

	messages, err := s.messageService.List(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (s *Server) handleGetProviders(c *gin.Context) {
	if s.config == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Config not available"})
		return
	}

	// 返回所有已知的 providers（从 catwalk 获取的完整列表）
	// 而不仅仅是已配置的 providers
	type ProviderInfo struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		Type    string `json:"type"`
	}

	// 先获取所有 known providers
	knownProviders, err := config.Providers(s.config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get providers: " + err.Error()})
		return
	}

	result := make([]ProviderInfo, 0, len(knownProviders))
	for _, p := range knownProviders {
		result = append(result, ProviderInfo{
			ID:      string(p.ID),
			Name:    p.Name,
			BaseURL: p.APIEndpoint,
			Type:    string(p.Type),
		})
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) handleGetProviderModels(c *gin.Context) {
	providerID := c.Param("provider")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "provider is required"})
		return
	}

	if s.config == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Config not available"})
		return
	}

	// 首先尝试从 known providers 中查找
	knownProviders, err := config.Providers(s.config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get providers: " + err.Error()})
		return
	}

	type ModelInfo struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		DefaultMaxTokens int64  `json:"default_max_tokens"`
	}

	// 查找对应的 provider
	for _, p := range knownProviders {
		if string(p.ID) == providerID {
			result := make([]ModelInfo, 0, len(p.Models))
			for _, m := range p.Models {
				result = append(result, ModelInfo{
					ID:               m.ID,
					Name:             m.Name,
					DefaultMaxTokens: m.DefaultMaxTokens,
				})
			}
			c.JSON(http.StatusOK, result)
			return
		}
	}

	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Provider not found"})
}

func (s *Server) handleGetFiles(c *gin.Context) {
	targetPath := c.DefaultQuery("path", ".")
	idCounter = 0

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Check if path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Path does not exist: " + absPath})
		return
	}

	fileTree, err := buildFileTree(absPath, absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, fileTree)
}

func generateID() string {
	idCounter++
	return strconv.Itoa(idCounter)
}

func buildFileTree(path string, rootPath string) (*FileNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

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
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}

		node.Children = []FileNode{}
		for _, file := range files {
			if shouldIgnoreFile(file.Name()) {
				continue
			}

			childPath := filepath.Join(path, file.Name())
			childNode, err := buildFileTree(childPath, rootPath)
			if err != nil {
				continue
			}
			node.Children = append(node.Children, *childNode)
		}
	} else {
		node.Type = "file"
		if info.Size() < 1024*1024 {
			content, err := ioutil.ReadFile(path)
			if err == nil {
				node.Content = string(content)
			}
		}
	}

	return node, nil
}

func shouldIgnoreFile(name string) bool {
	ignorePatterns := []string{
		".git", ".DS_Store", "node_modules", ".idea", ".vscode",
		"__pycache__", ".pytest_cache", ".pyc", ".pyo", ".env", ".env.local",
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
