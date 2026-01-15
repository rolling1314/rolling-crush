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
	"github.com/rolling1314/rolling-crush/infra/storage"
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
		Type       string              `json:"type"`
		Content    string              `json:"content"`
		SessionID  string              `json:"sessionID"` // Optional: if frontend sends it
		ID         string              `json:"id"`
		ToolCallID string              `json:"tool_call_id"`
		Granted    bool                `json:"granted"`
		Denied     bool                `json:"denied"`
		Images     []WSImageAttachment `json:"images"`    // Image attachments
		LastMsgID  string              `json:"lastMsgId"` // For reconnection - last received Redis stream message ID
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
		app.handlePermissionResponse(msg.ID, msg.ToolCallID, msg.Granted, msg.Denied)
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

	// Run the agent asynchronously
	app.runAgentAsync(sessionID, msg.Content, attachments)
}

// handlePermissionResponse handles permission grant/deny responses
func (app *WSApp) handlePermissionResponse(id, toolCallID string, granted, denied bool) {
	ctx := context.Background()
	permissionChan := app.Permissions.Subscribe(ctx)

	permissionReq := permission.PermissionRequest{
		ID:         id,
		ToolCallID: toolCallID,
	}

	if granted {
		slog.Info("Permission granted by client", "tool_call_id", toolCallID)
		app.Permissions.Grant(permissionReq)
	} else if denied {
		slog.Info("Permission denied by client", "tool_call_id", toolCallID)
		app.Permissions.Deny(permissionReq)
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

// runAgentAsync runs the agent asynchronously
func (app *WSApp) runAgentAsync(sessionID, content string, attachments []message.Attachment) {
	fmt.Println("\n=== About to call AgentCoordinator.Run in goroutine ===")
	fmt.Printf("准备传递的附件数量: %d\n", len(attachments))
	for i, att := range attachments {
		fmt.Printf("  [附件 %d] FileName: %s, MimeType: %s, Size: %d bytes\n",
			i+1, att.FileName, att.MimeType, len(att.Content))
	}

	go func() {
		fmt.Println("\n=== Inside goroutine, calling AgentCoordinator.Run ===")
		fmt.Printf("Goroutine 中的附件数量: %d\n", len(attachments))

		// Mark generation as active in Redis
		if app.RedisStream != nil {
			ctx := context.Background()
			if err := app.RedisStream.SetActiveGeneration(ctx, sessionID, true); err != nil {
				slog.Warn("Failed to mark generation as active", "error", err)
			}
		}

		_, err := app.AgentCoordinator.Run(context.Background(), sessionID, content, attachments...)

		// Mark generation as complete in Redis
		if app.RedisStream != nil {
			ctx := context.Background()
			if err := app.RedisStream.SetActiveGeneration(ctx, sessionID, false); err != nil {
				slog.Warn("Failed to mark generation as complete", "error", err)
			}

			// Publish generation complete event
			if err := app.RedisStream.PublishMessage(ctx, sessionID, "generation_complete", map[string]interface{}{
				"session_id": sessionID,
				"error":      err != nil,
			}); err != nil {
				slog.Warn("Failed to publish generation complete event", "error", err)
			}
		}

		// Send generation complete to WebSocket if connected
		isConnected, _ := app.connectedSessions.Get(sessionID)
		if isConnected {
			app.WSServer.SendToSession(sessionID, map[string]interface{}{
				"Type":       "generation_complete",
				"session_id": sessionID,
				"error":      err != nil,
			})
		}

		if err != nil {
			fmt.Println("Agent run error:", err)
			slog.Error("Agent run error", "error", err)
		} else {
			fmt.Println("Agent run completed successfully")
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

	// Update last read ID
	if newLastID != "" {
		if err := app.RedisStream.SetLastReadID(ctx, sessionID, newLastID); err != nil {
			slog.Warn("Failed to update last read ID", "error", err)
		}
	}

	// Check if generation is still active
	isActive, err := app.RedisStream.IsGenerationActive(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to check generation status", "error", err)
	} else {
		// Notify client about generation status
		app.WSServer.SendToSession(sessionID, map[string]interface{}{
			"Type":              "reconnection_status",
			"session_id":        sessionID,
			"messages_replayed": len(messages),
			"generation_active": isActive,
			"last_stream_id":    newLastID,
		})
	}

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
