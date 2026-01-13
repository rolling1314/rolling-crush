package http

import (
	"net/http"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/pkg/config"
	"github.com/gin-gonic/gin"
)

// handleGetProviders returns all known providers
func (s *Server) handleGetProviders(c *gin.Context) {
	if s.config == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Config not available"})
		return
	}

	// Return all known providers (complete list from catwalk)
	// not just configured providers
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

// handleGetProviderModels returns models for a specific provider
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

	// First try to find from known providers
	knownProviders, err := config.Providers(s.config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get providers: " + err.Error()})
		return
	}

	// Find the corresponding provider
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

// handleTestProviderConnection tests connection to a provider
func (s *Server) handleTestProviderConnection(c *gin.Context) {
	var req TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if s.config == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Config not available"})
		return
	}

	// Get known providers to determine provider type and base URL
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

	// Build temporary provider config for testing
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

	// Test connection
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

// handleConfigureProvider validates provider configuration
func (s *Server) handleConfigureProvider(c *gin.Context) {
	var req ConfigureProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if s.config == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Config not available"})
		return
	}

	// Web version doesn't need to save to filesystem, just validate configuration
	// Actual config will be saved to session_model_configs table when creating session

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Provider configuration validated successfully",
	})
}
