package app

import (
	"context"
	"fmt"
	"log/slog"

	storeredis "github.com/rolling1314/rolling-crush/infra/redis"
	"github.com/rolling1314/rolling-crush/internal/agent"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

func (app *WSApp) InitCoderAgent(ctx context.Context) error {
	fmt.Println("=== InitCoderAgent called ===")

	// Ensure agent configuration exists (for Web mode)
	if app.config.Agents == nil {
		app.config.Agents = make(map[string]config.Agent)
	}

	coderAgentCfg, ok := app.config.Agents[config.AgentCoder]
	if !ok || coderAgentCfg.ID == "" {
		fmt.Println("No coder agent config found, creating default config")
		// Create a default coder agent config for Web mode
		coderAgentCfg = config.Agent{
			ID:    config.AgentCoder,
			Name:  "Coder",
			Model: config.SelectedModelTypeLarge,
			AllowedTools: []string{
				"agent",
				"agentic_fetch",
				"bash",
				"job_output",
				"job_kill",
				"download",
				"edit",
				"multi_edit",
				"fetch",
				"glob",
				"grep",
				"ls",
				"sourcegraph",
				"view",
				"write",
				"diagnostics",
				"references",
			},
		}
		app.config.Agents[config.AgentCoder] = coderAgentCfg
		fmt.Println("Default coder agent config created")
	}

	var err error
	fmt.Println("Creating coordinator with dbReader:", app.db != nil)

	// Get Redis command service for real-time tool call state updates
	var redisCmd *storeredis.CommandService
	if storeredis.GetClient() != nil {
		redisCmd = storeredis.GetGlobalCommandService()
	}

	app.AgentCoordinator, err = agent.NewCoordinator(
		ctx,
		app.config,
		app.Sessions,
		app.Messages,
		app.ToolCalls,
		redisCmd,
		app.Permissions,
		app.History,
		app.LSPClients,
		app.db, // Pass DB queries as DBReader for session config loading
	)
	if err != nil {
		fmt.Println("Failed to create coordinator:", err)
		slog.Error("Failed to create coder agent", "err", err)
		return err
	}
	fmt.Println("Coordinator created successfully")
	return nil
}

func (app *WSApp) UpdateAgentModel(ctx context.Context) error {
	return app.AgentCoordinator.UpdateModels(ctx)
}
