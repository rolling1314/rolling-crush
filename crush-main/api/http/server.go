package http

import (
	"log/slog"

	"github.com/charmbracelet/crush/auth"
	"github.com/charmbracelet/crush/pkg/config"
	"github.com/charmbracelet/crush/store/postgres"
	"github.com/charmbracelet/crush/domain/message"
	"github.com/charmbracelet/crush/domain/project"
	"github.com/charmbracelet/crush/sandbox"
	"github.com/charmbracelet/crush/domain/session"
	"github.com/charmbracelet/crush/domain/user"
	"github.com/gin-gonic/gin"
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
			authGroup.GET("/github/callback", s.handleGitHubCallback)
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

		// File routes
		apiGroup.GET("/files", auth.GinAuthMiddleware(), s.handleGetFiles)

		// Image upload route
		apiGroup.POST("/upload", auth.GinAuthMiddleware(), s.handleUploadImage)
	}

	slog.Info("HTTP server starting", "port", s.port)
	return s.engine.Run(":" + s.port)
}
