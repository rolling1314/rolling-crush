package httpserver

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/config"
	"github.com/gin-gonic/gin"
)

// handleCreateSession handles session creation
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

		// 1. Create a temporary Config instance with DB storage enabled
		tempConfig := *s.config // Shallow copy of base config
		tempConfig.EnableDBStorage(sess.ID, s.db)
		fmt.Println("Enabled DB storage for session:", sess.ID)

		// 2. Set API Key following TUI logic (writes to database automatically)
		if req.ModelConfig.APIKey != "" {
			if err := tempConfig.SetProviderAPIKey(req.ModelConfig.Provider, req.ModelConfig.APIKey); err != nil {
				slog.Error("Failed to set provider API key", "error", err, "session_id", sess.ID)
			} else {
				slog.Info("Saved API key to database", "provider", req.ModelConfig.Provider, "session_id", sess.ID)
			}
		}

		// 3. Update preferred large model following TUI logic (writes to database automatically)
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

		// 4. Auto-set small model following TUI logic (writes to database automatically)
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

// handleGetSessionMessages handles getting messages for a session
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
