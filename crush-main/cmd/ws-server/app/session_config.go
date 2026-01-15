package app

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/rolling1314/rolling-crush/pkg/config"
)

// getSessionContextWindow retrieves the context window size for a session from its config
func (app *WSApp) getSessionContextWindow(ctx context.Context, sessionID string) int64 {
	// Debug: Check if app.config has providers loaded
	if app.config.Providers == nil {
		slog.Error("app.config.Providers is nil!", "session_id", sessionID)
		return 0
	}

	providerCount := 0
	for range app.config.Providers.Seq() {
		providerCount++
	}
	slog.Debug("app.config has providers", "session_id", sessionID, "provider_count", providerCount)

	configJSON, err := app.db.GetSessionConfigJSON(ctx, sessionID)
	slog.Info("getSessionContextWindow called", "session_id", sessionID, "config_json_length", len(configJSON), "error", err)

	if err != nil || configJSON == "" || configJSON == "{}" {
		slog.Warn("No session config found", "session_id", sessionID, "config_json", configJSON, "error", err)
		return 0
	}

	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configData); err != nil {
		slog.Error("Failed to parse session config JSON", "session_id", sessionID, "error", err)
		return 0
	}

	slog.Info("Parsed config data", "session_id", sessionID, "has_models", configData["models"] != nil, "has_providers", configData["providers"] != nil)

	if models, ok := configData["models"].(map[string]interface{}); ok {
		slog.Info("Found models in config", "session_id", sessionID, "models_keys", getMapKeys(models))

		if largeModel, ok := models["large"].(map[string]interface{}); ok {
			provider, _ := largeModel["provider"].(string)
			modelID, _ := largeModel["model"].(string)

			slog.Info("Found large model config", "session_id", sessionID, "provider", provider, "model", modelID)

			if provider != "" && modelID != "" {
				// First try from session config's providers section (if saved)
				if providers, ok := configData["providers"].(map[string]interface{}); ok {
					if providerData, ok := providers[provider].(map[string]interface{}); ok {
						if modelsData, ok := providerData["models"].([]interface{}); ok {
							for _, md := range modelsData {
								if modelData, ok := md.(map[string]interface{}); ok {
									if id, _ := modelData["id"].(string); id == modelID {
										if ctxWindow, ok := modelData["context_window"].(float64); ok && ctxWindow > 0 {
											slog.Info("✅ Found model info in session config providers", "session_id", sessionID, "provider", provider, "model", modelID, "context_window", int64(ctxWindow))
											return int64(ctxWindow)
										}
									}
								}
							}
						}
					}
				}

				// Second try from app.config.Providers
				if providerConfig, ok := app.config.Providers.Get(provider); ok {
					slog.Info("Provider found in config", "provider", provider, "model_count", len(providerConfig.Models))
					for _, m := range providerConfig.Models {
						if m.ID == modelID {
							slog.Info("✅ Found model info in app.config", "session_id", sessionID, "provider", provider, "model", modelID, "context_window", m.ContextWindow)
							return int64(m.ContextWindow)
						}
					}
				}

				// Fallback: try from knownProviders (catwalk providers)
				knownProviders, err := config.Providers(app.config)
				if err == nil {
					for _, p := range knownProviders {
						if string(p.ID) == provider {
							for _, m := range p.Models {
								if m.ID == modelID {
									slog.Info("✅ Found model info in knownProviders", "session_id", sessionID, "provider", provider, "model", modelID, "context_window", m.ContextWindow)
									return int64(m.ContextWindow)
								}
							}
							break
						}
					}
				}

				slog.Warn("❌ Model not found in config or knownProviders", "session_id", sessionID, "provider", provider, "model", modelID)
			} else {
				slog.Warn("Provider or model ID is empty", "session_id", sessionID, "provider", provider, "model", modelID)
			}
		} else {
			slog.Warn("No large model config found in models", "session_id", sessionID)
		}
	} else {
		slog.Warn("No models section in config", "session_id", sessionID)
	}

	return 0
}

// getMapKeys is a helper function to get map keys for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
