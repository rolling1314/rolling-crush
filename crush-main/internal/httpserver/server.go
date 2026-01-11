package httpserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/auth"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/project"
	"github.com/charmbracelet/crush/internal/sandbox"
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
	sandboxClient  *sandbox.Client
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
		sandboxClient:  sandbox.GetDefaultClient(),
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
	Name             string  `json:"name" binding:"required"`
	Description      string  `json:"description"`
	ExternalIP       string  `json:"external_ip" binding:"required"`
	FrontendPort     int32   `json:"frontend_port" binding:"required"`
	WorkspacePath    string  `json:"workspace_path" binding:"required"`
	ContainerName    *string `json:"container_name,omitempty"`
	WorkdirPath      *string `json:"workdir_path,omitempty"`
	DbHost           *string `json:"db_host,omitempty"`
	DbPort           *int32  `json:"db_port,omitempty"`
	DbUser           *string `json:"db_user,omitempty"`
	DbPassword       *string `json:"db_password,omitempty"`
	DbName           *string `json:"db_name,omitempty"`
	BackendPort      *int32  `json:"backend_port,omitempty"`
	FrontendCommand  *string `json:"frontend_command,omitempty"`
	FrontendLanguage *string `json:"frontend_language,omitempty"`
	BackendCommand   *string `json:"backend_command,omitempty"`
	BackendLanguage  *string `json:"backend_language,omitempty"`
}

type ProjectResponse struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	ExternalIP       string  `json:"external_ip"`
	FrontendPort     int32   `json:"frontend_port"`
	WorkspacePath    string  `json:"workspace_path"`
	ContainerName    *string `json:"container_name,omitempty"`
	WorkdirPath      *string `json:"workdir_path,omitempty"`
	DbHost           *string `json:"db_host,omitempty"`
	DbPort           *int32  `json:"db_port,omitempty"`
	DbUser           *string `json:"db_user,omitempty"`
	DbPassword       *string `json:"db_password,omitempty"`
	DbName           *string `json:"db_name,omitempty"`
	BackendPort      *int32  `json:"backend_port,omitempty"`
	FrontendCommand  *string `json:"frontend_command,omitempty"`
	FrontendLanguage *string `json:"frontend_language,omitempty"`
	BackendCommand   *string `json:"backend_command,omitempty"`
	BackendLanguage  *string `json:"backend_language,omitempty"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
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

// Helper function to convert sql.NullString to *string
func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// Helper function to convert *string to sql.NullString
func ptrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// Helper function to convert sql.NullInt32 to *int32
func nullInt32ToPtr(ni sql.NullInt32) *int32 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int32
}

// Helper function to convert *int32 to sql.NullInt32
func ptrToNullInt32(i *int32) sql.NullInt32 {
	if i == nil {
		return sql.NullInt32{Valid: false}
	}
	return sql.NullInt32{Int32: *i, Valid: true}
}

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
			sessionGroup.GET("/:id/config", s.handleGetSessionConfig)
			sessionGroup.PUT("/:id/config", s.handleUpdateSessionConfig)
			sessionGroup.DELETE("/:id", s.handleDeleteSession)
		}

		// Providers and models endpoints
		apiGroup.GET("/providers", auth.GinAuthMiddleware(), s.handleGetProviders)
		apiGroup.GET("/providers/:provider/models", auth.GinAuthMiddleware(), s.handleGetProviderModels)
		apiGroup.POST("/providers/test-connection", auth.GinAuthMiddleware(), s.handleTestProviderConnection)
		apiGroup.POST("/providers/configure", auth.GinAuthMiddleware(), s.handleConfigureProvider)

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

	proj, err := s.projectService.Create(c.Request.Context(), userID, req.Name, req.Description, req.ExternalIP, req.WorkspacePath, req.FrontendPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ProjectResponse{
		ID:               proj.ID,
		Name:             proj.Name,
		Description:      proj.Description.String,
		ExternalIP:       proj.ExternalIP,
		FrontendPort:     proj.FrontendPort,
		WorkspacePath:    proj.WorkspacePath,
		ContainerName:    nullStringToPtr(proj.ContainerName),
		WorkdirPath:      nullStringToPtr(proj.WorkdirPath),
		DbHost:           nullStringToPtr(proj.DbHost),
		DbPort:           nullInt32ToPtr(proj.DbPort),
		DbUser:           nullStringToPtr(proj.DbUser),
		DbPassword:       nullStringToPtr(proj.DbPassword),
		DbName:           nullStringToPtr(proj.DbName),
		BackendPort:      nullInt32ToPtr(proj.BackendPort),
		FrontendCommand:  nullStringToPtr(proj.FrontendCommand),
		FrontendLanguage: nullStringToPtr(proj.FrontendLanguage),
		BackendCommand:   nullStringToPtr(proj.BackendCommand),
		BackendLanguage:  nullStringToPtr(proj.BackendLanguage),
		CreatedAt:        proj.CreatedAt,
		UpdatedAt:        proj.UpdatedAt,
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
			ID:               proj.ID,
			Name:             proj.Name,
			Description:      proj.Description.String,
			ExternalIP:       proj.ExternalIP,
			FrontendPort:     proj.FrontendPort,
			WorkspacePath:    proj.WorkspacePath,
			ContainerName:    nullStringToPtr(proj.ContainerName),
			WorkdirPath:      nullStringToPtr(proj.WorkdirPath),
			DbHost:           nullStringToPtr(proj.DbHost),
			DbPort:           nullInt32ToPtr(proj.DbPort),
			DbUser:           nullStringToPtr(proj.DbUser),
			DbPassword:       nullStringToPtr(proj.DbPassword),
			DbName:           nullStringToPtr(proj.DbName),
			BackendPort:      nullInt32ToPtr(proj.BackendPort),
			FrontendCommand:  nullStringToPtr(proj.FrontendCommand),
			FrontendLanguage: nullStringToPtr(proj.FrontendLanguage),
			BackendCommand:   nullStringToPtr(proj.BackendCommand),
			BackendLanguage:  nullStringToPtr(proj.BackendLanguage),
			CreatedAt:        proj.CreatedAt,
			UpdatedAt:        proj.UpdatedAt,
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
		ID:               proj.ID,
		Name:             proj.Name,
		Description:      proj.Description.String,
		ExternalIP:       proj.ExternalIP,
		FrontendPort:     proj.FrontendPort,
		WorkspacePath:    proj.WorkspacePath,
		ContainerName:    nullStringToPtr(proj.ContainerName),
		WorkdirPath:      nullStringToPtr(proj.WorkdirPath),
		DbHost:           nullStringToPtr(proj.DbHost),
		DbPort:           nullInt32ToPtr(proj.DbPort),
		DbUser:           nullStringToPtr(proj.DbUser),
		DbPassword:       nullStringToPtr(proj.DbPassword),
		DbName:           nullStringToPtr(proj.DbName),
		BackendPort:      nullInt32ToPtr(proj.BackendPort),
		FrontendCommand:  nullStringToPtr(proj.FrontendCommand),
		FrontendLanguage: nullStringToPtr(proj.FrontendLanguage),
		BackendCommand:   nullStringToPtr(proj.BackendCommand),
		BackendLanguage:  nullStringToPtr(proj.BackendLanguage),
		CreatedAt:        proj.CreatedAt,
		UpdatedAt:        proj.UpdatedAt,
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
		ID:               projectID,
		Name:             req.Name,
		Description:      sql.NullString{String: req.Description, Valid: req.Description != ""},
		ExternalIP:       req.ExternalIP,
		FrontendPort:     req.FrontendPort,
		WorkspacePath:    req.WorkspacePath,
		ContainerName:    ptrToNullString(req.ContainerName),
		WorkdirPath:      ptrToNullString(req.WorkdirPath),
		DbHost:           ptrToNullString(req.DbHost),
		DbPort:           ptrToNullInt32(req.DbPort),
		DbUser:           ptrToNullString(req.DbUser),
		DbPassword:       ptrToNullString(req.DbPassword),
		DbName:           ptrToNullString(req.DbName),
		BackendPort:      ptrToNullInt32(req.BackendPort),
		FrontendCommand:  ptrToNullString(req.FrontendCommand),
		FrontendLanguage: ptrToNullString(req.FrontendLanguage),
		BackendCommand:   ptrToNullString(req.BackendCommand),
		BackendLanguage:  ptrToNullString(req.BackendLanguage),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ProjectResponse{
		ID:               proj.ID,
		Name:             proj.Name,
		Description:      proj.Description.String,
		ExternalIP:       proj.ExternalIP,
		FrontendPort:     proj.FrontendPort,
		WorkspacePath:    proj.WorkspacePath,
		ContainerName:    nullStringToPtr(proj.ContainerName),
		WorkdirPath:      nullStringToPtr(proj.WorkdirPath),
		DbHost:           nullStringToPtr(proj.DbHost),
		DbPort:           nullInt32ToPtr(proj.DbPort),
		DbUser:           nullStringToPtr(proj.DbUser),
		DbPassword:       nullStringToPtr(proj.DbPassword),
		DbName:           nullStringToPtr(proj.DbName),
		BackendPort:      nullInt32ToPtr(proj.BackendPort),
		FrontendCommand:  nullStringToPtr(proj.FrontendCommand),
		FrontendLanguage: nullStringToPtr(proj.FrontendLanguage),
		BackendCommand:   nullStringToPtr(proj.BackendCommand),
		BackendLanguage:  nullStringToPtr(proj.BackendLanguage),
		CreatedAt:        proj.CreatedAt,
		UpdatedAt:        proj.UpdatedAt,
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

	// Save model config using TUI's exact logic, writing to database instead of file
	fmt.Println("=== handleCreateSession: About to save model config ===")
	fmt.Println("req.ModelConfig:", req.ModelConfig)

	if req.ModelConfig != nil {
		fmt.Println("ModelConfig is not nil, proceeding with config save")
		fmt.Println("Provider:", req.ModelConfig.Provider, "Model:", req.ModelConfig.Model)

		// 1. 创建一个临时Config实例，启用数据库存储模式
		tempConfig := *s.config // 浅拷贝基础配置
		tempConfig.EnableDBStorage(sess.ID, s.db)
		fmt.Println("Enabled DB storage for session:", sess.ID)

		// 2. 按照TUI逻辑设置API Key（会自动写入数据库）
		if req.ModelConfig.APIKey != "" {
			if err := tempConfig.SetProviderAPIKey(req.ModelConfig.Provider, req.ModelConfig.APIKey); err != nil {
				slog.Error("Failed to set provider API key", "error", err, "session_id", sess.ID)
			} else {
				slog.Info("Saved API key to database", "provider", req.ModelConfig.Provider, "session_id", sess.ID)
			}
		}

		// 3. 按照TUI逻辑更新preferred large model（会自动写入数据库）
		largeModel := config.SelectedModel{
			Model:           req.ModelConfig.Model,
			Provider:        req.ModelConfig.Provider,
			ReasoningEffort: req.ModelConfig.ReasoningEffort,
		}
		if req.ModelConfig.MaxTokens != nil {
			largeModel.MaxTokens = *req.ModelConfig.MaxTokens
		}
		if err := tempConfig.UpdatePreferredModel(config.SelectedModelTypeLarge, largeModel); err != nil {
			slog.Error("Failed to update preferred large model", "error", err, "session_id", sess.ID)
		} else {
			slog.Info("Saved large model to database", "model", req.ModelConfig.Model, "session_id", sess.ID)
		}

		// 4. 按照TUI逻辑自动设置small model（会自动写入数据库）
		knownProviders, err := config.Providers(&tempConfig)
		if err == nil {
			var providerInfo *catwalk.Provider
			for _, p := range knownProviders {
				if string(p.ID) == req.ModelConfig.Provider {
					providerInfo = &p
					break
				}
			}

			if providerInfo != nil && providerInfo.DefaultSmallModelID != "" {
				smallModelInfo := tempConfig.GetModel(req.ModelConfig.Provider, providerInfo.DefaultSmallModelID)
				if smallModelInfo != nil {
					smallModel := config.SelectedModel{
						Model:           smallModelInfo.ID,
						Provider:        req.ModelConfig.Provider,
						ReasoningEffort: smallModelInfo.DefaultReasoningEffort,
						MaxTokens:       smallModelInfo.DefaultMaxTokens,
					}
					if err := tempConfig.UpdatePreferredModel(config.SelectedModelTypeSmall, smallModel); err != nil {
						slog.Error("Failed to update preferred small model", "error", err, "session_id", sess.ID)
					} else {
						slog.Info("Saved small model to database", "model", smallModelInfo.ID, "session_id", sess.ID)
					}
				}
			}
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

func (s *Server) handleTestProviderConnection(c *gin.Context) {
	type TestConnectionRequest struct {
		Provider string `json:"provider" binding:"required"`
		Model    string `json:"model" binding:"required"`
		APIKey   string `json:"api_key" binding:"required"`
		BaseURL  string `json:"base_url"`
	}

	var req TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if s.config == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Config not available"})
		return
	}

	// 获取 known providers 来确定 provider 的类型和 base URL
	knownProviders, err := config.Providers(s.config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get providers: " + err.Error()})
		return
	}

	var providerInfo *catwalk.Provider
	for _, p := range knownProviders {
		if string(p.ID) == req.Provider {
			providerInfo = &p
			break
		}
	}

	if providerInfo == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Provider not found"})
		return
	}

	// 构造临时的 provider config 用于测试
	providerConfig := config.ProviderConfig{
		ID:      req.Provider,
		Name:    providerInfo.Name,
		APIKey:  req.APIKey,
		Type:    providerInfo.Type,
		BaseURL: req.BaseURL,
	}
	if providerConfig.BaseURL == "" {
		providerConfig.BaseURL = providerInfo.APIEndpoint
	}

	// 测试连接
	if err := providerConfig.TestConnection(s.config.Resolver()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Connection successful",
	})
}

func (s *Server) handleConfigureProvider(c *gin.Context) {
	type ConfigureProviderRequest struct {
		Provider        string `json:"provider" binding:"required"`
		Model           string `json:"model" binding:"required"`
		APIKey          string `json:"api_key" binding:"required"`
		BaseURL         string `json:"base_url"`
		MaxTokens       *int64 `json:"max_tokens"`
		ReasoningEffort string `json:"reasoning_effort"`
		SetAsDefault    bool   `json:"set_as_default"` // 暂时保留参数但不使用
	}

	var req ConfigureProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if s.config == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Config not available"})
		return
	}

	// Web版本不需要保存到文件系统，只验证配置有效性即可
	// 实际的配置会在创建session时保存到session_model_configs表

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Provider configuration validated successfully",
	})
}

func (s *Server) handleGetFiles(c *gin.Context) {
	// 获取请求参数
	targetPath := c.DefaultQuery("path", ".")
	sessionID := c.Query("session_id")

	slog.Info("handleGetFiles request", "session_id", sessionID, "path", targetPath, "query", c.Request.URL.RawQuery)

	// session_id 是必需的
	if sessionID == "" {
		slog.Warn("Missing session_id parameter", "path", targetPath)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required. Usage: /api/files?session_id=xxx&path=/sandbox/project"})
		return
	}

	slog.Info("Fetching file tree from sandbox", "session_id", sessionID, "path", targetPath)

	// 通过沙箱客户端获取文件树
	resp, err := s.sandboxClient.GetFileTree(c.Request.Context(), sandbox.FileTreeRequest{
		SessionID: sessionID,
		Path:      targetPath,
	})

	if err != nil {
		slog.Error("Failed to get file tree from sandbox", "error", err, "session_id", sessionID)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to get file tree: %v", err)})
		return
	}

	// 返回文件树
	c.JSON(http.StatusOK, resp.Tree)
}

// SessionConfigResponse represents the model configuration for a session
type SessionConfigResponse struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	APIKey          string   `json:"api_key"` // Masked for security
	BaseURL         string   `json:"base_url,omitempty"`
	MaxTokens       *int64   `json:"max_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
}

// UpdateSessionConfigRequest represents the request to update session model configuration
type UpdateSessionConfigRequest struct {
	Provider        string   `json:"provider" binding:"required"`
	Model           string   `json:"model" binding:"required"`
	APIKey          string   `json:"api_key"` // Optional - only update if provided
	BaseURL         string   `json:"base_url"`
	MaxTokens       *int64   `json:"max_tokens"`
	Temperature     *float64 `json:"temperature"`
	TopP            *float64 `json:"top_p"`
	ReasoningEffort string   `json:"reasoning_effort"`
}

// handleGetSessionConfig returns the model configuration for a session
func (s *Server) handleGetSessionConfig(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required"})
		return
	}

	// Get session config JSON from database
	configJSON, err := s.db.GetSessionConfigJSON(c.Request.Context(), sessionID)
	if err != nil {
		slog.Error("Failed to get session config", "session_id", sessionID, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get session config"})
		return
	}

	// If no config found, return empty response
	if configJSON == "" || configJSON == "{}" {
		c.JSON(http.StatusOK, SessionConfigResponse{})
		return
	}

	// Parse the JSON to extract model config
	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configData); err != nil {
		slog.Error("Failed to parse session config JSON", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to parse config"})
		return
	}

	response := SessionConfigResponse{}

	// Extract large model config
	if models, ok := configData["models"].(map[string]interface{}); ok {
		if largeModel, ok := models["large"].(map[string]interface{}); ok {
			if provider, ok := largeModel["provider"].(string); ok {
				response.Provider = provider
			}
			if model, ok := largeModel["model"].(string); ok {
				response.Model = model
			}
			if maxTokens, ok := largeModel["max_tokens"].(float64); ok {
				tokens := int64(maxTokens)
				response.MaxTokens = &tokens
			}
			if reasoningEffort, ok := largeModel["reasoning_effort"].(string); ok {
				response.ReasoningEffort = reasoningEffort
			}
		}
	}

	// Extract provider API key (masked)
	if providers, ok := configData["providers"].(map[string]interface{}); ok {
		if providerConfig, ok := providers[response.Provider].(map[string]interface{}); ok {
			if apiKey, ok := providerConfig["api_key"].(string); ok {
				// Mask the API key for security (show only last 4 characters)
				if len(apiKey) > 4 {
					response.APIKey = "****" + apiKey[len(apiKey)-4:]
				} else {
					response.APIKey = "****"
				}
			}
			if baseURL, ok := providerConfig["base_url"].(string); ok {
				response.BaseURL = baseURL
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// handleUpdateSessionConfig updates the model configuration for a session
func (s *Server) handleUpdateSessionConfig(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required"})
		return
	}

	var req UpdateSessionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Create a temporary Config instance and enable DB storage
	tempConfig := *s.config // Shallow copy of base config
	tempConfig.EnableDBStorage(sessionID, s.db)

	// Set API Key using TUI logic
	if req.APIKey != "" {
		if err := tempConfig.SetProviderAPIKey(req.Provider, req.APIKey); err != nil {
			slog.Error("Failed to set provider API key", "error", err, "session_id", sessionID)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to set API key"})
			return
		}
		slog.Info("Updated API key in database", "provider", req.Provider, "session_id", sessionID)
	}

	// Update preferred large model using TUI logic
	largeModel := config.SelectedModel{
		Model:           req.Model,
		Provider:        req.Provider,
		ReasoningEffort: req.ReasoningEffort,
	}
	if req.MaxTokens != nil {
		largeModel.MaxTokens = *req.MaxTokens
	}
	if err := tempConfig.UpdatePreferredModel(config.SelectedModelTypeLarge, largeModel); err != nil {
		slog.Error("Failed to update preferred large model", "error", err, "session_id", sessionID)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to update model"})
		return
	}
	slog.Info("Updated large model in database", "model", req.Model, "session_id", sessionID)

	// Auto-set small model using TUI logic
	knownProviders, err := config.Providers(&tempConfig)
	if err == nil {
		var providerInfo *catwalk.Provider
		for _, p := range knownProviders {
			if string(p.ID) == req.Provider {
				providerInfo = &p
				break
			}
		}

		if providerInfo != nil && providerInfo.DefaultSmallModelID != "" {
			smallModelInfo := tempConfig.GetModel(req.Provider, providerInfo.DefaultSmallModelID)
			if smallModelInfo != nil {
				smallModel := config.SelectedModel{
					Model:           smallModelInfo.ID,
					Provider:        req.Provider,
					ReasoningEffort: smallModelInfo.DefaultReasoningEffort,
					MaxTokens:       smallModelInfo.DefaultMaxTokens,
				}
				if err := tempConfig.UpdatePreferredModel(config.SelectedModelTypeSmall, smallModel); err != nil {
					slog.Error("Failed to update preferred small model", "error", err, "session_id", sessionID)
				} else {
					slog.Info("Updated small model in database", "model", smallModelInfo.ID, "session_id", sessionID)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session configuration updated successfully"})
}

// handleDeleteSession deletes a session and all associated data
func (s *Server) handleDeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required"})
		return
	}

	ctx := c.Request.Context()

	// Delete session messages
	if err := s.db.DeleteSessionMessages(ctx, sessionID); err != nil {
		slog.Error("Failed to delete session messages", "session_id", sessionID, "error", err)
	}

	// Delete session files
	if err := s.db.DeleteSessionFiles(ctx, sessionID); err != nil {
		slog.Error("Failed to delete session files", "session_id", sessionID, "error", err)
	}

	// Delete session model config
	if err := s.db.DeleteSessionModelConfig(ctx, sessionID); err != nil {
		slog.Error("Failed to delete session model config", "session_id", sessionID, "error", err)
	}

	// Delete session
	if err := s.db.DeleteSession(ctx, sessionID); err != nil {
		slog.Error("Failed to delete session", "session_id", sessionID, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete session"})
		return
	}

	slog.Info("Session deleted successfully", "session_id", sessionID)
	c.JSON(http.StatusOK, gin.H{"message": "Session deleted successfully"})
}
