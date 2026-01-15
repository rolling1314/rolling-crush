package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ToolCallResponse represents a tool call state response
type ToolCallResponse struct {
	ID           string `json:"id"`
	SessionID    string `json:"session_id"`
	MessageID    string `json:"message_id,omitempty"`
	Name         string `json:"name"`
	Input        string `json:"input,omitempty"`
	Status       string `json:"status"`
	Result       string `json:"result,omitempty"`
	IsError      bool   `json:"is_error"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	StartedAt    *int64 `json:"started_at,omitempty"`
	FinishedAt   *int64 `json:"finished_at,omitempty"`
}

// handleGetSessionToolCalls gets all tool calls for a session
func (s *Server) handleGetSessionToolCalls(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required"})
		return
	}

	toolCalls, err := s.toolCallService.ListBySession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Convert to response format
	responses := make([]ToolCallResponse, len(toolCalls))
	for i, tc := range toolCalls {
		responses[i] = ToolCallResponse{
			ID:           tc.ID,
			SessionID:    tc.SessionID,
			MessageID:    tc.MessageID,
			Name:         tc.Name,
			Input:        tc.Input,
			Status:       string(tc.Status),
			Result:       tc.Result,
			IsError:      tc.IsError,
			ErrorMessage: tc.ErrorMessage,
			CreatedAt:    tc.CreatedAt,
			UpdatedAt:    tc.UpdatedAt,
			StartedAt:    tc.StartedAt,
			FinishedAt:   tc.FinishedAt,
		}
	}

	c.JSON(http.StatusOK, responses)
}

// handleGetPendingToolCalls gets pending/running tool calls for a session
func (s *Server) handleGetPendingToolCalls(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required"})
		return
	}

	toolCalls, err := s.toolCallService.ListPending(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Convert to response format
	responses := make([]ToolCallResponse, len(toolCalls))
	for i, tc := range toolCalls {
		responses[i] = ToolCallResponse{
			ID:           tc.ID,
			SessionID:    tc.SessionID,
			MessageID:    tc.MessageID,
			Name:         tc.Name,
			Input:        tc.Input,
			Status:       string(tc.Status),
			Result:       tc.Result,
			IsError:      tc.IsError,
			ErrorMessage: tc.ErrorMessage,
			CreatedAt:    tc.CreatedAt,
			UpdatedAt:    tc.UpdatedAt,
			StartedAt:    tc.StartedAt,
			FinishedAt:   tc.FinishedAt,
		}
	}

	c.JSON(http.StatusOK, responses)
}

// handleGetToolCall gets a specific tool call by ID
func (s *Server) handleGetToolCall(c *gin.Context) {
	toolCallID := c.Param("toolCallId")
	if toolCallID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tool_call_id is required"})
		return
	}

	tc, err := s.toolCallService.Get(c.Request.Context(), toolCallID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Tool call not found"})
		return
	}

	response := ToolCallResponse{
		ID:           tc.ID,
		SessionID:    tc.SessionID,
		MessageID:    tc.MessageID,
		Name:         tc.Name,
		Input:        tc.Input,
		Status:       string(tc.Status),
		Result:       tc.Result,
		IsError:      tc.IsError,
		ErrorMessage: tc.ErrorMessage,
		CreatedAt:    tc.CreatedAt,
		UpdatedAt:    tc.UpdatedAt,
		StartedAt:    tc.StartedAt,
		FinishedAt:   tc.FinishedAt,
	}

	c.JSON(http.StatusOK, response)
}
