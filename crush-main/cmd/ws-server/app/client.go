package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/permission"
	storeredis "github.com/rolling1314/rolling-crush/infra/redis"
	"github.com/rolling1314/rolling-crush/infra/storage"
	"github.com/rolling1314/rolling-crush/internal/agent"
)

// WSImageAttachment represents an image attached to a message
type WSImageAttachment struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Filename string `json:"filename"`
}

// HandleClientDisconnect handles WebSocket disconnection
// Instead of cancelling the agent, we mark the session as disconnected so messages
// continue to be buffered in Redis for later retrieval
func (app *WSApp) HandleClientDisconnect() {
	fmt.Println("=== HandleClientDisconnect called ===")
	slog.Info("WebSocket client disconnected", "sessionID", app.currentSessionID)

	// Mark session as disconnected but DON'T cancel the agent
	// The agent will continue running and messages will be buffered in Redis
	if app.currentSessionID != "" {
		app.connectedSessions.Set(app.currentSessionID, false)

		// Update Redis connection status
		if app.RedisStream != nil {
			ctx := context.Background()
			if err := app.RedisStream.SetConnectionStatus(ctx, app.currentSessionID, false); err != nil {
				slog.Warn("Failed to update Redis connection status", "error", err)
			}
		}

		fmt.Printf("Session %s marked as disconnected, agent continues running\n", app.currentSessionID)
		slog.Info("Session marked as disconnected, agent continues running", "sessionID", app.currentSessionID)
	}

	// Clear the current session ID so new connections start fresh
	app.currentSessionID = ""
	fmt.Println("Current session ID cleared")
}

// HandleClientMessage processes messages from the WebSocket client
func (app *WSApp) HandleClientMessage(rawMsg []byte) {
	fmt.Println("=== HandleClientMessage called ===")
	fmt.Println("Raw message:", string(rawMsg))

	type ClientMsg struct {
		Type            string              `json:"type"`
		Content         string              `json:"content"`
		SessionID       string              `json:"sessionID"`  // Optional: if frontend sends it (camelCase)
		SessionIDSnake  string              `json:"session_id"` // Optional: for permission_response (snake_case)
		ID              string              `json:"id"`
		ToolCallID      string              `json:"tool_call_id"`
		Granted         bool                `json:"granted"`
		Denied          bool                `json:"denied"`
		AllowForSession bool                `json:"allow_for_session"` // Allow this tool for the entire session
		ToolName        string              `json:"tool_name"`         // Tool name for allowlist
		Action          string              `json:"action"`            // Action for allowlist
		Path            string              `json:"path"`              // Path for allowlist
		Images          []WSImageAttachment `json:"images"`            // Image attachments
		LastMsgID       string              `json:"lastMsgId"`         // For reconnection - last received Redis stream message ID
	}

	var msg ClientMsg
	if err := json.Unmarshal(rawMsg, &msg); err != nil {
		slog.Error("Failed to unmarshal client message", "error", err)
		return
	}

	fmt.Println("Parsed message type:", msg.Type, "content:", msg.Content, "sessionID:", msg.SessionID)

	// Handle reconnection request - client wants to resume receiving messages
	if msg.Type == "reconnect" {
		app.handleReconnection(msg.SessionID, msg.LastMsgID)
		return
	}

	// Handle permission responses
	if msg.Type == "permission_response" {
		// Get session ID from snake_case field (from permission_response)
		sessionID := msg.SessionIDSnake
		if sessionID == "" {
			sessionID = msg.SessionID // Fallback to camelCase
		}
		if sessionID == "" {
			sessionID = app.currentSessionID // Fallback to current session
		}
		app.handlePermissionResponse(msg.ID, msg.ToolCallID, sessionID, msg.Granted, msg.Denied, msg.AllowForSession, msg.ToolName, msg.Action, msg.Path)
		return
	}

	// Handle cancel requests - 取消当前会话的 agent 请求
	if msg.Type == "cancel" {
		app.handleCancelRequest(msg.SessionID)
		return
	}

	// Use existing session or create new one
	sessionID := app.resolveSessionID(msg.SessionID)
	if sessionID == "" {
		return
	}

	// Mark session as connected
	app.markSessionConnected(sessionID)

	fmt.Println("Final sessionID:", sessionID)
	slog.Info("Received message from client", "content", msg.Content, "sessionID", sessionID)

	// Ensure AgentCoordinator is initialized
	if !app.ensureAgentInitialized() {
		return
	}

	// Fetch image attachments if any
	attachments := app.processImageAttachments(msg.Images)

	// Run the agent via worker pool for bounded concurrency
	if err := app.runAgentViaPool(sessionID, msg.Content, attachments); err != nil {
		slog.Error("[GOROUTINE] Failed to submit agent task",
			"session_id", sessionID,
			"error", err,
		)
		// Send error message to client
		app.sendErrorToClient(sessionID, "系统繁忙，请稍后重试 (503)")
		return
	}
}

// handlePermissionResponse handles permission grant/deny responses
func (app *WSApp) handlePermissionResponse(id, toolCallID, sessionID string, granted, denied, allowForSession bool, toolName, action, path string) {
	ctx := context.Background()
	permissionChan := app.Permissions.Subscribe(ctx)

	permissionReq := permission.PermissionRequest{
		ID:         id,
		ToolCallID: toolCallID,
		SessionID:  sessionID,
		ToolName:   toolName,
		Action:     action,
		Path:       path,
	}

	// Check if this is a resumed permission request (tool call in awaiting_permission status)
	// If so, handle it specially to re-run the original task
	if app.db != nil && toolCallID != "" {
		toolCall, err := app.db.GetToolCall(ctx, toolCallID)
		if err == nil && toolCall.Status == "awaiting_permission" {
			slog.Info("[GOROUTINE] Handling resumed permission response",
				"tool_call_id", toolCallID,
				"session_id", sessionID,
				"granted", granted || allowForSession,
			)
			app.handleResumedPermissionResponse(ctx, toolCallID, sessionID, granted || allowForSession, toolName, action, path)
			// Clean up subscription
			go func() {
				<-permissionChan
			}()
			return
		}
	}

	if allowForSession {
		// Allow for session: grant now and add to session allowlist
		slog.Info("Permission granted for session by client",
			"tool_call_id", toolCallID,
			"session_id", sessionID,
			"tool_name", toolName,
			"action", action,
		)
		app.Permissions.GrantForSession(permissionReq)
	} else if granted {
		slog.Info("Permission granted by client", "tool_call_id", toolCallID, "session_id", sessionID)
		app.Permissions.Grant(permissionReq)
	} else if denied {
		slog.Info("Permission denied by client", "tool_call_id", toolCallID, "session_id", sessionID)
		app.Permissions.Deny(permissionReq)
	}

	// Also update Redis permission status directly to ensure it's updated
	if app.RedisStream != nil {
		status := "denied"
		if granted || allowForSession {
			status = "granted"
		}
		if err := app.RedisStream.UpdatePermissionStatus(ctx, sessionID, toolCallID, status); err != nil {
			slog.Warn("Failed to update permission status in Redis", "error", err, "session_id", sessionID, "tool_call_id", toolCallID)
		} else {
			slog.Info("Permission status updated in Redis", "session_id", sessionID, "tool_call_id", toolCallID, "status", status)
		}
	}

	// Clean up subscription
	go func() {
		<-permissionChan
	}()
}

// handleCancelRequest handles agent cancellation requests
func (app *WSApp) handleCancelRequest(sessionID string) {
	if sessionID == "" {
		sessionID = app.currentSessionID
	}
	if sessionID != "" && app.AgentCoordinator != nil {
		fmt.Printf("[CANCEL] Cancelling agent request for session: %s\n", sessionID)
		slog.Info("Cancelling agent request", "sessionID", sessionID)
		app.AgentCoordinator.Cancel(sessionID)
	}
}

// resolveSessionID resolves the session ID from the message or creates a new session
func (app *WSApp) resolveSessionID(msgSessionID string) string {
	sessionID := msgSessionID
	fmt.Println("Processing message, sessionID from message:", sessionID)

	if sessionID == "" {
		fmt.Println("No sessionID in message, checking currentSessionID:", app.currentSessionID)
		if app.currentSessionID == "" {
			fmt.Println("Creating new session...")
			sess, err := app.Sessions.Create(context.Background(), "", "Web Session")
			if err != nil {
				slog.Error("Failed to create session", "error", err)
				return ""
			}
			app.currentSessionID = sess.ID
			fmt.Println("Created session with ID:", sess.ID)
		}
		sessionID = app.currentSessionID
	} else {
		app.currentSessionID = sessionID
	}

	return sessionID
}

// markSessionConnected marks the session as connected in both local state and Redis
func (app *WSApp) markSessionConnected(sessionID string) {
	app.connectedSessions.Set(sessionID, true)
	if app.RedisStream != nil {
		ctx := context.Background()
		if err := app.RedisStream.SetConnectionStatus(ctx, sessionID, true); err != nil {
			slog.Warn("Failed to update Redis connection status", "error", err)
		}
	}
}

// ensureAgentInitialized ensures the AgentCoordinator is initialized
func (app *WSApp) ensureAgentInitialized() bool {
	if app.AgentCoordinator == nil {
		fmt.Println("AgentCoordinator is nil, attempting to initialize...")
		slog.Warn("AgentCoordinator not initialized, attempting to initialize now")
		if err := app.InitCoderAgent(context.Background()); err != nil {
			fmt.Println("Failed to initialize AgentCoordinator:", err)
			slog.Error("Failed to initialize AgentCoordinator", "error", err)
			return false
		}
		fmt.Println("AgentCoordinator initialized successfully")
	} else {
		fmt.Println("AgentCoordinator already initialized")
	}
	return true
}

// processImageAttachments processes image attachments from the message
func (app *WSApp) processImageAttachments(images []WSImageAttachment) []message.Attachment {
	var attachments []message.Attachment
	fmt.Println("=== 开始检查图片附件 ===")
	fmt.Printf("收到的消息中包含图片数量: %d\n", len(images))

	if len(images) == 0 {
		fmt.Println("  - 没有图片附件")
		return attachments
	}

	fmt.Printf("Processing %d image attachments\n", len(images))
	minioClient := storage.GetMinIOClient()

	for i, img := range images {
		fmt.Printf("\n[图片 %d/%d] 开始处理\n", i+1, len(images))
		fmt.Printf("  - URL: %s\n", img.URL)
		fmt.Printf("  - Filename: %s\n", img.Filename)
		fmt.Printf("  - MimeType: %s\n", img.MimeType)
		fmt.Printf("Fetching image: %s\n", img.URL)

		var imageData []byte
		var mimeType string
		var err error

		// Check if it's a MinIO URL and fetch accordingly
		if minioClient != nil && minioClient.IsMinIOURL(img.URL) {
			fmt.Println("  - 检测到 MinIO URL，从 MinIO 获取图片")
			imageData, mimeType, err = minioClient.GetFile(context.Background(), img.URL)
		} else {
			// Fetch from external URL
			fmt.Println("  - 检测到外部 URL，开始下载图片")
			imageData, mimeType, err = wsFetchImageFromURL(img.URL)
		}

		if err != nil {
			fmt.Printf("  ❌ Failed to fetch image %s: %v\n", img.URL, err)
			slog.Error("Failed to fetch image", "url", img.URL, "error", err)
			continue
		}
		fmt.Printf("  ✅ 图片下载成功！大小: %d bytes, MIME类型: %s\n", len(imageData), mimeType)

		// Use provided mime type if available
		if img.MimeType != "" {
			fmt.Printf("  - 使用客户端提供的 MIME 类型: %s\n", img.MimeType)
			mimeType = img.MimeType
		}

		filename := img.Filename
		if filename == "" {
			// Extract filename from URL
			parts := strings.Split(img.URL, "/")
			filename = parts[len(parts)-1]
			fmt.Printf("  - 从 URL 提取文件名: %s\n", filename)
		} else {
			fmt.Printf("  - 使用客户端提供的文件名: %s\n", filename)
		}

		attachments = append(attachments, message.Attachment{
			FilePath: img.URL,
			FileName: filename,
			MimeType: mimeType,
			Content:  imageData,
		})
		fmt.Printf("  ✅ Image attachment added: %s (%s, %d bytes)\n", filename, mimeType, len(imageData))
		fmt.Printf("[图片 %d/%d] 处理完成\n", i+1, len(images))
	}

	fmt.Printf("\n=== 图片处理完成，共添加 %d 个附件 ===\n\n", len(attachments))
	return attachments
}

// runAgentViaPool submits an agent task to the worker pool for execution.
// Returns an error if the pool is full or shutting down.
// This method provides bounded concurrency control.
func (app *WSApp) runAgentViaPool(sessionID, content string, attachments []message.Attachment) error {
	if app.AgentWorkerPool == nil {
		// Fall back to direct execution if pool not initialized
		slog.Warn("[GOROUTINE] Worker pool not available, falling back to direct execution")
		app.runAgentAsync(sessionID, content, attachments)
		return nil
	}

	task := agent.AgentTask{
		SessionID:   sessionID,
		Prompt:      content,
		Attachments: attachments,
		ResultChan:  make(chan agent.AgentTaskResult, 1),
	}

	if err := app.AgentWorkerPool.Submit(context.Background(), task); err != nil {
		slog.Error("[GOROUTINE] Failed to submit task to worker pool",
			"session_id", sessionID,
			"error", err,
		)
		return err
	}

	slog.Info("[GOROUTINE] Task submitted to worker pool",
		"session_id", sessionID,
		"pool_stats", app.AgentWorkerPool.Stats(),
	)
	return nil
}

// runAgentAsync runs the agent asynchronously (fallback when worker pool is not available)
// Note: This uses the same lifecycle pattern as the worker pool for consistency
func (app *WSApp) runAgentAsync(sessionID, content string, attachments []message.Attachment) {
	fmt.Println("\n=== About to call AgentCoordinator.Run in goroutine ===")
	fmt.Printf("准备传递的附件数量: %d\n", len(attachments))
	for i, att := range attachments {
		fmt.Printf("  [附件 %d] FileName: %s, MimeType: %s, Size: %d bytes\n",
			i+1, att.FileName, att.MimeType, len(att.Content))
	}

	go func() {
		fmt.Printf("\n[GOROUTINE] 🚀 Session Agent Goroutine 创建 | sessionID=%s\n", sessionID)
		defer fmt.Printf("[GOROUTINE] 🛑 Session Agent Goroutine 退出 | sessionID=%s\n", sessionID)

		ctx := context.Background()

		// === LIFECYCLE: Task Start ===
		slog.Info("[LIFECYCLE] Agent task started (async)", "session_id", sessionID)
		if app.RedisStream != nil {
			if err := app.RedisStream.SetSessionRunningStatus(ctx, sessionID, storeredis.SessionStatusRunning); err != nil {
				slog.Warn("Failed to set session running status", "error", err, "session_id", sessionID)
			}
			if err := app.RedisStream.SetActiveGeneration(ctx, sessionID, true); err != nil {
				slog.Warn("Failed to mark generation as active", "error", err)
			}
		}
		app.sendSessionStatusUpdate(sessionID, storeredis.SessionStatusRunning)

		// === Execute Agent ===
		_, err := app.AgentCoordinator.Run(ctx, sessionID, content, attachments...)

		// === LIFECYCLE: Task Complete ===
		var finalStatus storeredis.SessionRunningStatus
		var reason string
		if err != nil {
			if ctx.Err() == context.Canceled {
				finalStatus = storeredis.SessionStatusCancelled
				reason = "cancelled"
			} else {
				finalStatus = storeredis.SessionStatusError
				reason = "error"
			}
		} else {
			finalStatus = storeredis.SessionStatusCompleted
			reason = "completed"
		}

		slog.Info("[LIFECYCLE] Agent task completed (async)", "session_id", sessionID, "reason", reason, "error", err)

		if app.RedisStream != nil {
			if setErr := app.RedisStream.SetSessionRunningStatus(ctx, sessionID, finalStatus); setErr != nil {
				slog.Warn("Failed to set session completed status", "error", setErr, "session_id", sessionID)
			}
			if setErr := app.RedisStream.SetActiveGeneration(ctx, sessionID, false); setErr != nil {
				slog.Warn("Failed to mark generation as complete", "error", setErr)
			}
			if pubErr := app.RedisStream.PublishMessage(ctx, sessionID, "generation_complete", map[string]interface{}{
				"session_id": sessionID,
				"status":     string(finalStatus),
				"error":      err != nil,
			}); pubErr != nil {
				slog.Warn("Failed to publish generation complete event", "error", pubErr)
			}
		}
		app.sendSessionStatusUpdate(sessionID, finalStatus)

		if err != nil {
			slog.Error("Agent run error", "error", err)
		}
	}()
	fmt.Println("Goroutine started, HandleClientMessage returning")
}

// handleReconnection handles client reconnection and sends missed messages
func (app *WSApp) handleReconnection(sessionID string, lastMsgID string) {
	fmt.Printf("=== handleReconnection called for session %s, lastMsgID: %s ===\n", sessionID, lastMsgID)
	slog.Info("Handling reconnection", "sessionID", sessionID, "lastMsgID", lastMsgID)

	if sessionID == "" {
		slog.Warn("Reconnection request without session ID")
		return
	}

	// Mark session as connected
	app.currentSessionID = sessionID
	app.connectedSessions.Set(sessionID, true)

	if app.RedisStream == nil {
		slog.Warn("Redis stream service not available, cannot replay messages")
		return
	}

	ctx := context.Background()

	// Update Redis connection status
	if err := app.RedisStream.SetConnectionStatus(ctx, sessionID, true); err != nil {
		slog.Warn("Failed to update Redis connection status", "error", err)
	}

	// Read missed messages from Redis stream
	messages, newLastID, err := app.RedisStream.ReadMessages(ctx, sessionID, lastMsgID, 0)
	if err != nil {
		slog.Error("Failed to read missed messages from Redis", "error", err)
		return
	}

	fmt.Printf("Found %d missed messages for session %s\n", len(messages), sessionID)
	slog.Info("Replaying missed messages", "sessionID", sessionID, "count", len(messages))

	// Send missed messages to the client
	for _, msg := range messages {
		// Skip permission-related messages during replay - they are managed separately
		// via pending permissions state (not in stream anymore, but skip for backwards compatibility)
		if msg.Type == "permission_request" || msg.Type == "permission_notification" {
			slog.Debug("Skipping permission message during replay", "type", msg.Type, "streamId", msg.ID)
			continue
		}

		// Skip tool_call_update messages during replay - we'll send the latest state separately
		// This prevents showing outdated tool status (e.g., pending when it's already completed)
		if msg.Type == "tool_call_update" {
			slog.Debug("Skipping tool_call_update during replay, will send latest state", "streamId", msg.ID)
			continue
		}

		// Handle stream_delta messages - send them directly without wrapping
		if msg.Type == "stream_delta" {
			var deltaPayload map[string]interface{}
			if err := json.Unmarshal(msg.Payload, &deltaPayload); err != nil {
				slog.Warn("Failed to unmarshal stream_delta payload", "error", err)
				continue
			}
			// Add replay metadata and stream ID
			deltaPayload["Type"] = "stream_delta"
			deltaPayload["_replay"] = true
			deltaPayload["_streamId"] = msg.ID
			app.WSServer.SendToSession(sessionID, deltaPayload)
			continue
		}

		var payload interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			slog.Warn("Failed to unmarshal message payload", "error", err)
			continue
		}

		// Send the message with its original type
		app.WSServer.SendToSession(sessionID, map[string]interface{}{
			"_replay":    true,
			"_streamId":  msg.ID,
			"_type":      msg.Type,
			"_timestamp": msg.Timestamp,
			"_payload":   payload,
		})
	}

	// Send latest tool call states from Redis (real-time status)
	if app.RedisCmd != nil {
		toolCallStates, err := app.RedisCmd.GetSessionToolCallStates(ctx, sessionID)
		if err != nil {
			slog.Warn("Failed to get tool call states", "error", err)
		} else if len(toolCallStates) > 0 {
			slog.Info("Sending latest tool call states on reconnection", "sessionID", sessionID, "count", len(toolCallStates))
			for _, state := range toolCallStates {
				toolCallMsg := map[string]interface{}{
					"Type":       "tool_call_update",
					"id":         state.ID,
					"session_id": state.SessionID,
					"message_id": state.MessageID,
					"name":       state.Name,
					"input":      state.Input,
					"status":     state.Status,
				}
				app.WSServer.SendToSession(sessionID, toolCallMsg)

				// If tool is not pending, it means permission was already granted/denied
				// Send permission_notification to clear the permission card in frontend
				if state.Status != "pending" {
					granted := state.Status == "running" || state.Status == "completed"
					denied := state.Status == "error" || state.Status == "cancelled"
					permNotif := map[string]interface{}{
						"Type":         "permission_notification",
						"tool_call_id": state.ID,
						"granted":      granted,
						"denied":       denied,
					}
					app.WSServer.SendToSession(sessionID, permNotif)
				}
			}
		}
	}

	// Send any pending permissions that are still waiting for user response
	pendingPerms, err := app.RedisStream.GetAllPendingPermissions(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to get pending permissions", "error", err)
	} else if len(pendingPerms) > 0 {
		slog.Info("Sending pending permissions on reconnection", "sessionID", sessionID, "count", len(pendingPerms))
		for _, perm := range pendingPerms {
			permMsg := map[string]interface{}{
				"Type":         "permission_request",
				"id":           perm.ID,
				"session_id":   perm.SessionID,
				"tool_call_id": perm.ToolCallID,
				"tool_name":    perm.ToolName,
				"description":  perm.Description,
				"action":       perm.Action,
				"params":       perm.Params,
				"path":         perm.Path,
			}
			app.WSServer.SendToSession(sessionID, permMsg)
		}
	}

	// Check for awaiting_permission tool calls from database (suspended tasks from previous session)
	app.checkAndSendAwaitingPermissionToolCalls(ctx, sessionID)

	// Update last read ID
	if newLastID != "" {
		if err := app.RedisStream.SetLastReadID(ctx, sessionID, newLastID); err != nil {
			slog.Warn("Failed to update last read ID", "error", err)
		}
	}

	// Check session running status from Redis
	sessionStatus, err := app.RedisStream.GetSessionRunningStatus(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to check session running status", "error", err)
	}

	// Check if generation is still active (for backward compatibility)
	isActive, err := app.RedisStream.IsGenerationActive(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to check generation status", "error", err)
	}

	// Determine if session is running based on status or active generation
	isRunning := sessionStatus == storeredis.SessionStatusRunning || isActive

	// Notify client about reconnection status including session running status
	app.WSServer.SendToSession(sessionID, map[string]interface{}{
		"Type":              "reconnection_status",
		"session_id":        sessionID,
		"messages_replayed": len(messages),
		"generation_active": isActive,
		"session_status":    string(sessionStatus),
		"is_running":        isRunning,
		"last_stream_id":    newLastID,
	})

	// Send current session info including context_window
	app.sendSessionUpdate(ctx, sessionID)

	fmt.Printf("Reconnection complete for session %s\n", sessionID)
}

// sendSessionUpdate sends the current session info to the client via WebSocket
// This ensures the client has the latest session data including context_window
func (app *WSApp) sendSessionUpdate(ctx context.Context, sessionID string) {
	// Get session from database
	sess, err := app.Sessions.Get(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to get session for update", "sessionID", sessionID, "error", err)
		return
	}

	// Get context window for this session
	contextWindow := app.getSessionContextWindow(ctx, sessionID)

	slog.Info("Sending session update on connect", "sessionID", sessionID, "context_window", contextWindow, "cost", sess.Cost)

	// Send session update to client
	sessionMsg := map[string]interface{}{
		"Type":              "session_update",
		"id":                sessionID,
		"project_id":        sess.ProjectID,
		"title":             sess.Title,
		"message_count":     sess.MessageCount,
		"prompt_tokens":     sess.PromptTokens,
		"completion_tokens": sess.CompletionTokens,
		"cost":              sess.Cost,
		"context_window":    contextWindow,
		"created_at":        sess.CreatedAt,
		"updated_at":        sess.UpdatedAt,
	}

	app.WSServer.SendToSession(sessionID, sessionMsg)
}

// wsFetchImageFromURL fetches an image from an external URL
func wsFetchImageFromURL(url string) ([]byte, string, error) {
	fmt.Printf("    → 开始 HTTP GET 请求: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("    ❌ HTTP 请求失败: %v\n", err)
		return nil, "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("    → HTTP 状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("    ❌ HTTP 状态码错误: %d\n", resp.StatusCode)
		return nil, "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	fmt.Println("    → 开始读取响应数据...")
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("    ❌ 读取数据失败: %v\n", err)
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}
	fmt.Printf("    → 读取完成，数据大小: %d bytes\n", len(data))

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
		fmt.Printf("    → 自动检测 MIME 类型: %s\n", mimeType)
	} else {
		fmt.Printf("    → 从响应头获取 MIME 类型: %s\n", mimeType)
	}

	return data, mimeType, nil
}

// checkAndSendAwaitingPermissionToolCalls checks the database for tool calls
// that are awaiting permission and sends them to the client
func (app *WSApp) checkAndSendAwaitingPermissionToolCalls(ctx context.Context, sessionID string) {
	if app.db == nil {
		slog.Debug("Database not available, skipping awaiting permission check")
		return
	}

	// Query awaiting_permission tool calls from database
	toolCalls, err := app.db.ListAwaitingPermissionToolCalls(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to list awaiting permission tool calls", "sessionID", sessionID, "error", err)
		return
	}

	if len(toolCalls) == 0 {
		slog.Debug("No awaiting permission tool calls found", "sessionID", sessionID)
		return
	}

	slog.Info("[GOROUTINE] Found awaiting permission tool calls on reconnect",
		"sessionID", sessionID,
		"count", len(toolCalls),
	)

	// Send each awaiting permission tool call as a permission request to the client
	for _, tc := range toolCalls {
		permMsg := map[string]interface{}{
			"Type":            "permission_request",
			"id":              tc.ID,
			"session_id":      tc.SessionID,
			"tool_call_id":    tc.ID,
			"tool_name":       tc.Name,
			"description":     fmt.Sprintf("Tool %s requires permission (resumed from previous session)", tc.Name),
			"action":          tc.PermissionAction.String,
			"path":            tc.PermissionPath.String,
			"original_prompt": tc.OriginalPrompt.String,
			"_resumed":        true, // Mark as resumed for frontend
		}

		// Parse input if available
		if tc.Input.Valid && tc.Input.String != "" {
			var params interface{}
			if err := json.Unmarshal([]byte(tc.Input.String), &params); err == nil {
				permMsg["params"] = params
			}
		}

		app.WSServer.SendToSession(sessionID, permMsg)
		slog.Info("[GOROUTINE] Sent awaiting permission request to client",
			"sessionID", sessionID,
			"toolCallID", tc.ID,
			"toolName", tc.Name,
		)
	}
}

// handleResumedPermissionResponse handles permission response for a resumed (previously timed out) tool call
// It updates the database and re-submits the original task to the agent
func (app *WSApp) handleResumedPermissionResponse(ctx context.Context, toolCallID, sessionID string, granted bool, toolName, action, path string) {
	if app.db == nil {
		slog.Warn("Database not available, cannot handle resumed permission response")
		return
	}

	// Get the tool call from database
	toolCall, err := app.db.GetToolCall(ctx, toolCallID)
	if err != nil {
		slog.Warn("Failed to get tool call for resumed permission", "toolCallID", toolCallID, "error", err)
		return
	}

	// Verify it's in awaiting_permission status
	if toolCall.Status != "awaiting_permission" {
		slog.Debug("Tool call not in awaiting_permission status, ignoring",
			"toolCallID", toolCallID,
			"status", toolCall.Status,
		)
		return
	}

	if granted {
		slog.Info("[GOROUTINE] Resumed permission granted, re-submitting task",
			"sessionID", sessionID,
			"toolCallID", toolCallID,
			"toolName", toolCall.Name,
		)

		// Update tool call status to running
		if err := app.db.UpdateToolCallPermissionGranted(ctx, toolCallID); err != nil {
			slog.Error("Failed to update tool call permission granted", "error", err)
		}

		// Add to session allowlist so the re-run will pass permission check
		if app.RedisStream != nil {
			// Grant for session via the permission service which handles allowlist properly
			permReq := permission.PermissionRequest{
				ID:         toolCallID,
				SessionID:  sessionID,
				ToolCallID: toolCallID,
				ToolName:   toolName,
				Action:     action,
				Path:       path,
			}
			app.Permissions.GrantForSession(permReq)
		}

		// Re-submit the original task to the agent via worker pool
		if toolCall.OriginalPrompt.Valid && toolCall.OriginalPrompt.String != "" {
			slog.Info("[GOROUTINE] Re-running agent with original prompt",
				"sessionID", sessionID,
				"prompt_length", len(toolCall.OriginalPrompt.String),
			)
			// Run agent via worker pool with the original prompt
			if err := app.runAgentViaPool(sessionID, toolCall.OriginalPrompt.String, nil); err != nil {
				slog.Error("[GOROUTINE] Failed to re-submit resumed task",
					"session_id", sessionID,
					"error", err,
				)
				app.sendErrorToClient(sessionID, "系统繁忙，无法恢复任务 (503)")
			}
		} else {
			slog.Warn("No original prompt found for resumed task, cannot re-run",
				"sessionID", sessionID,
				"toolCallID", toolCallID,
			)
		}
	} else {
		slog.Info("[GOROUTINE] Resumed permission denied",
			"sessionID", sessionID,
			"toolCallID", toolCallID,
		)

		// Update tool call status to cancelled
		if err := app.db.CancelToolCall(ctx, toolCallID); err != nil {
			slog.Error("Failed to cancel tool call for denied permission", "error", err)
		}
	}
}

// sendErrorToClient sends an error message to the client via WebSocket
func (app *WSApp) sendErrorToClient(sessionID, errorMessage string) {
	app.WSServer.SendToSession(sessionID, map[string]interface{}{
		"Type":       "error",
		"session_id": sessionID,
		"error":      errorMessage,
	})
}
