package handler

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rolling1314/rolling-crush/domain/project"
	"github.com/rolling1314/rolling-crush/infra/sandbox"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

// generateSubdomain generates a random 10-character alphanumeric subdomain
func generateSubdomain() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 10)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to a simple counter if crypto/rand fails
			b[i] = charset[i%len(charset)]
		} else {
			b[i] = charset[n.Int64()]
		}
	}
	return string(b)
}

// handleCreateProject handles project creation
func (s *Server) handleCreateProject(c *gin.Context) {
	userID := c.GetString("user_id")
	var req ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	slog.Info("Creating project", "name", req.Name, "backend_language", req.BackendLanguage, "need_database", req.NeedDatabase)

	// Call sandbox service to create container
	sandboxResp, err := s.sandboxClient.CreateProject(c.Request.Context(), sandbox.CreateProjectRequest{
		ProjectName:     req.Name,
		BackendLanguage: stringPtrToValue(req.BackendLanguage),
		NeedDatabase:    req.NeedDatabase,
	})
	if err != nil {
		slog.Error("Failed to create project container", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to create container: %v", err)})
		return
	}

	slog.Info("Container created",
		"container_id", sandboxResp.ContainerID,
		"container_name", sandboxResp.ContainerName,
		"frontend_port", sandboxResp.FrontendPort,
		"backend_port", sandboxResp.BackendPort,
		"workdir", sandboxResp.Workdir)

	// Set default values - use config's external_ip if not provided in request
	appCfg := config.GetGlobalAppConfig()
	externalIP := req.ExternalIP
	if externalIP == "" {
		externalIP = appCfg.Sandbox.ExternalIP
		if externalIP == "" {
			externalIP = "localhost"
		}
	}
	workspacePath := req.WorkspacePath
	if workspacePath == "" {
		workspacePath = "/workspace"
	}

	// Generate subdomain
	subdomain := generateSubdomain()
	domain := appCfg.Cloudflare.Domain
	if domain == "" {
		domain = "rollingcoding.com"
	}
	fullSubdomain := fmt.Sprintf("%s.%s", subdomain, domain)

	slog.Info("Generated subdomain", "subdomain", subdomain, "full_subdomain", fullSubdomain)

	// Configure domain in sandbox (nginx + vite)
	if s.sandboxClient != nil {
		_, err := s.sandboxClient.ConfigureDomain(c.Request.Context(), sandbox.ConfigureDomainRequest{
			ContainerID:  sandboxResp.ContainerID,
			Subdomain:    subdomain,
			FrontendPort: sandboxResp.FrontendPort,
			Domain:       domain,
		})
		if err != nil {
			slog.Warn("Failed to configure domain in sandbox", "error", err, "subdomain", fullSubdomain)
			// Don't fail the request, continue without domain configuration
		} else {
			slog.Info("Domain configured in sandbox", "subdomain", fullSubdomain)
		}
	}

	// Add DNS record to Cloudflare
	fmt.Printf("🔍 Checking Cloudflare: client_nil=%v, api_token_empty=%v, domain=%s\n",
		s.cloudflareClient == nil,
		appCfg.Cloudflare.APIToken == "",
		appCfg.Cloudflare.Domain)

	if s.cloudflareClient != nil && appCfg.Cloudflare.APIToken != "" {
		fmt.Printf("📤 Calling Cloudflare API: subdomain=%s, ip=%s\n", subdomain, externalIP)
		err := s.cloudflareClient.AddOrUpdateDNSRecord(c.Request.Context(), subdomain, externalIP)
		if err != nil {
			fmt.Printf("❌ Cloudflare DNS failed: %v\n", err)
			slog.Error("Failed to add DNS record to Cloudflare", "error", err, "subdomain", fullSubdomain, "ip", externalIP)
		} else {
			fmt.Printf("✅ Cloudflare DNS added: %s -> %s\n", fullSubdomain, externalIP)
			slog.Info("DNS record added to Cloudflare successfully", "subdomain", fullSubdomain, "ip", externalIP)
		}
	} else {
		fmt.Printf("⚠️ Skipping Cloudflare: client_nil=%v, api_token_empty=%v\n",
			s.cloudflareClient == nil, appCfg.Cloudflare.APIToken == "")
		slog.Warn("Skipping Cloudflare DNS configuration", "subdomain", fullSubdomain)
	}

	// Create project record
	proj, err := s.projectService.Create(
		c.Request.Context(),
		userID,
		req.Name,
		req.Description,
		externalIP,
		workspacePath,
		sandboxResp.FrontendPort,
	)
	if err != nil {
		slog.Error("Failed to create project in database", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Update project with container info
	// Store container ID (12-char short ID) in container_name field
	proj.ContainerName = sql.NullString{String: sandboxResp.ContainerID, Valid: true}
	// Working directory is /workspace
	proj.WorkdirPath = sql.NullString{String: sandboxResp.Workdir, Valid: true}
	// Store the subdomain
	proj.Subdomain = sql.NullString{String: fullSubdomain, Valid: true}

	if req.BackendLanguage != nil && *req.BackendLanguage != "" {
		proj.BackendLanguage = sql.NullString{String: *req.BackendLanguage, Valid: true}
		if sandboxResp.BackendPort != nil {
			proj.BackendPort = sql.NullInt32{Int32: *sandboxResp.BackendPort, Valid: true}
		}
	}
	proj.FrontendLanguage = sql.NullString{String: "vite", Valid: true}

	slog.Info("Updating project with container info",
		"container_id", sandboxResp.ContainerID,
		"workdir", sandboxResp.Workdir,
		"frontend_port", sandboxResp.FrontendPort,
		"backend_port", sandboxResp.BackendPort,
		"subdomain", fullSubdomain)

	// If database is needed, configure database connection info
	if req.NeedDatabase {
		proj.DbHost = sql.NullString{String: "localhost", Valid: true}
		proj.DbPort = sql.NullInt32{Int32: 5432, Valid: true}
		proj.DbUser = sql.NullString{String: "postgres", Valid: true}
		proj.DbPassword = sql.NullString{String: "postgres", Valid: true}
		proj.DbName = sql.NullString{String: req.Name, Valid: true}
	}

	// Save updated project info
	proj, err = s.projectService.Update(c.Request.Context(), proj)
	if err != nil {
		slog.Error("Failed to update project with container info", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	slog.Info("Project created successfully", "project_id", proj.ID, "subdomain", fullSubdomain)

	c.JSON(http.StatusOK, projectToResponse(proj))
}

// handleListProjects handles listing projects for a user
func (s *Server) handleListProjects(c *gin.Context) {
	userID := c.GetString("user_id")
	projects, err := s.projectService.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]ProjectResponse, len(projects))
	for i, proj := range projects {
		response[i] = projectToResponse(proj)
	}

	c.JSON(http.StatusOK, response)
}

// handleGetProject handles getting a single project by ID
func (s *Server) handleGetProject(c *gin.Context) {
	projectID := c.Param("id")
	proj, err := s.projectService.GetByID(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Project not found"})
		return
	}

	c.JSON(http.StatusOK, projectToResponse(proj))
}

// handleUpdateProject handles updating a project
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
		Subdomain:        ptrToNullString(req.Subdomain),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, projectToResponse(proj))
}

// handleDeleteProject handles project deletion
func (s *Server) handleDeleteProject(c *gin.Context) {
	projectID := c.Param("id")

	// First, get the project to find the container ID
	proj, err := s.projectService.GetByID(c.Request.Context(), projectID)
	if err != nil {
		slog.Error("Failed to get project for deletion", "error", err, "project_id", projectID)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Project not found"})
		return
	}

	// If project has a container, delete it from sandbox
	if proj.ContainerName.Valid && proj.ContainerName.String != "" {
		containerID := proj.ContainerName.String
		slog.Info("Deleting project container", "container_id", containerID, "project_id", projectID)

		_, err := s.sandboxClient.DeleteProject(c.Request.Context(), sandbox.DeleteProjectRequest{
			ContainerID: containerID,
		})
		if err != nil {
			// Log the error but continue with database deletion
			// Container might already be deleted or not exist
			slog.Warn("Failed to delete container from sandbox", "error", err, "container_id", containerID)
		} else {
			slog.Info("Container deleted successfully", "container_id", containerID)
		}
	}

	// Delete the project from database
	if err := s.projectService.Delete(c.Request.Context(), projectID); err != nil {
		slog.Error("Failed to delete project from database", "error", err, "project_id", projectID)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	slog.Info("Project deleted successfully", "project_id", projectID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleGetProjectSessions handles getting sessions for a project
func (s *Server) handleGetProjectSessions(c *gin.Context) {
	projectID := c.Param("id")
	sessions, err := s.sessionService.List(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]SessionResponse, len(sessions))
	for i, sess := range sessions {
		contextWindow := s.getSessionContextWindow(c.Request.Context(), sess.ID)
		
		// Debug: log session todos
		slog.Info("Session todos", "session_id", sess.ID, "todos_count", len(sess.Todos))
		
		// Convert session todos to response format
		var todos []TodoResponse
		for _, todo := range sess.Todos {
			todos = append(todos, TodoResponse{
				Content:    todo.Content,
				Status:     string(todo.Status),
				ActiveForm: todo.ActiveForm,
			})
		}
		
		response[i] = SessionResponse{
			ID:               sess.ID,
			ProjectID:        sess.ProjectID,
			Title:            sess.Title,
			MessageCount:     sess.MessageCount,
			PromptTokens:     sess.PromptTokens,
			CompletionTokens: sess.CompletionTokens,
			Cost:             sess.Cost,
			ContextWindow:    contextWindow,
			Todos:            todos,
			CreatedAt:        sess.CreatedAt,
			UpdatedAt:        sess.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// projectToResponse converts a project.Project to ProjectResponse
func projectToResponse(proj project.Project) ProjectResponse {
	return ProjectResponse{
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
		Subdomain:        nullStringToPtr(proj.Subdomain),
		CreatedAt:        proj.CreatedAt,
		UpdatedAt:        proj.UpdatedAt,
	}
}
