package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/rolling1314/rolling-crush/auth"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/project"
	"github.com/rolling1314/rolling-crush/domain/session"
	"github.com/rolling1314/rolling-crush/domain/user"
	"github.com/rolling1314/rolling-crush/infra/postgres"
	"github.com/rolling1314/rolling-crush/infra/sandbox"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

// Server represents the HTTP server
type Server struct {
	port           string
	engine         *gin.Engine
	userService    user.Service
	projectService project.Service
	sessionService session.Service
	messageService message.Service
	db             *postgres.Queries
	config         *config.Config
	sandboxClient  *sandbox.Client
}

// New creates a new HTTP server instance
func New(port string, userService user.Service, projectService project.Service, sessionService session.Service, messageService message.Service, queries *postgres.Queries, cfg *config.Config) *Server {
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

// Start initializes routes and starts the HTTP server
func (s *Server) Start() error {
	s.engine.Use(corsMiddleware())

	// Health check
	s.engine.GET("/health", s.handleHealth)

	// GitHub OAuth callback (must be at root level to match GitHub OAuth app configuration)
	s.engine.GET("/auth/github/callback", s.handleGitHubCallback)

	// API routes
	apiGroup := s.engine.Group("/api")
	{
		// Auth routes
		authGroup := apiGroup.Group("/auth")
		{
			authGroup.POST("/register", s.handleRegister)
			authGroup.POST("/login", s.handleLogin)
			authGroup.GET("/verify", auth.GinAuthMiddleware(), s.handleVerify)
			// GitHub OAuth routes
			authGroup.GET("/github", s.handleGitHubLogin)
			authGroup.GET("/github/callback", s.handleGitHubCallback) // Also keep this for consistency
		}

		// Project routes
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

		// Session routes
		sessionGroup := apiGroup.Group("/sessions")
		sessionGroup.Use(auth.GinAuthMiddleware())
		{
			sessionGroup.POST("", s.handleCreateSession)
			sessionGroup.GET("/:id/messages", s.handleGetSessionMessages)
			sessionGroup.GET("/:id/config", s.handleGetSessionConfig)
			sessionGroup.PUT("/:id/config", s.handleUpdateSessionConfig)
			sessionGroup.DELETE("/:id", s.handleDeleteSession)
		}

		// Provider routes
		apiGroup.GET("/providers", auth.GinAuthMiddleware(), s.handleGetProviders)
		apiGroup.GET("/providers/:provider/models", auth.GinAuthMiddleware(), s.handleGetProviderModels)
		apiGroup.POST("/providers/test-connection", auth.GinAuthMiddleware(), s.handleTestProviderConnection)
		apiGroup.POST("/providers/configure", auth.GinAuthMiddleware(), s.handleConfigureProvider)

		// Auto model config endpoint
		apiGroup.GET("/auto-model", auth.GinAuthMiddleware(), s.handleGetAutoModel)

		// File routes
		apiGroup.GET("/files", auth.GinAuthMiddleware(), s.handleGetFiles)

		// Image upload route
		apiGroup.POST("/upload", auth.GinAuthMiddleware(), s.handleUploadImage)
	}

	slog.Info("HTTP server starting", "port", s.port)
	return s.engine.Run(":" + s.port)
}

// getSessionContextWindow helper
func (s *Server) getSessionContextWindow(ctx context.Context, sessionID string) int64 {
	configJSON, err := s.db.GetSessionConfigJSON(ctx, sessionID)
	if err != nil || configJSON == "" || configJSON == "{}" {
		return 0
	}

	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configData); err != nil {
		return 0
	}

	if models, ok := configData["models"].(map[string]interface{}); ok {
		if largeModel, ok := models["large"].(map[string]interface{}); ok {
			provider, _ := largeModel["provider"].(string)
			modelID, _ := largeModel["model"].(string)

			if provider != "" && modelID != "" {
				modelInfo := s.config.GetModel(provider, modelID)
				if modelInfo != nil {
					return int64(modelInfo.ContextWindow)
				}
			}
		}
	}
	return 0
}
