package message

import "time"

// DeltaType represents the type of streaming delta content
type DeltaType string

const (
	// DeltaTypeText represents text content delta
	DeltaTypeText DeltaType = "text"
	// DeltaTypeReasoning represents reasoning/thinking content delta
	DeltaTypeReasoning DeltaType = "reasoning"
	// DeltaTypeToolCallInput represents tool call input delta
	DeltaTypeToolCallInput DeltaType = "tool_call_input"
	// DeltaTypeToolCall represents a new tool call being added
	DeltaTypeToolCall DeltaType = "tool_call"
	// DeltaTypeFinish represents the end of streaming for a message
	DeltaTypeFinish DeltaType = "finish"
	// DeltaTypeError represents an error notification (shown as toast, not stored in chat)
	DeltaTypeError DeltaType = "error"
)

// StreamDelta represents an incremental update to a message during streaming.
// Instead of sending the full message on every update, we send only the delta (change).
type StreamDelta struct {
	// MessageID is the ID of the message being updated
	MessageID string `json:"message_id"`
	// SessionID is the session this message belongs to
	SessionID string `json:"session_id"`
	// DeltaType indicates what kind of content is being streamed
	DeltaType DeltaType `json:"delta_type"`
	// Content is the incremental text content (for text, reasoning, tool_call_input)
	Content string `json:"content"`
	// ToolCallID is set when DeltaType is tool_call_input or tool_call
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolCallName is set when DeltaType is tool_call (new tool call)
	ToolCallName string `json:"tool_call_name,omitempty"`
	// FinishReason is set when DeltaType is finish
	FinishReason string `json:"finish_reason,omitempty"`
	// Timestamp when this delta was created
	Timestamp int64 `json:"timestamp"`
}

// NewTextDelta creates a new text content delta
func NewTextDelta(messageID, sessionID, content string) StreamDelta {
	return StreamDelta{
		MessageID: messageID,
		SessionID: sessionID,
		DeltaType: DeltaTypeText,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewReasoningDelta creates a new reasoning/thinking content delta
func NewReasoningDelta(messageID, sessionID, content string) StreamDelta {
	return StreamDelta{
		MessageID: messageID,
		SessionID: sessionID,
		DeltaType: DeltaTypeReasoning,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewToolCallInputDelta creates a new tool call input delta
func NewToolCallInputDelta(messageID, sessionID, toolCallID, content string) StreamDelta {
	return StreamDelta{
		MessageID:  messageID,
		SessionID:  sessionID,
		DeltaType:  DeltaTypeToolCallInput,
		Content:    content,
		ToolCallID: toolCallID,
		Timestamp:  time.Now().UnixMilli(),
	}
}

// NewToolCallDelta creates a delta for a new tool call being started
func NewToolCallDelta(messageID, sessionID, toolCallID, toolCallName string) StreamDelta {
	return StreamDelta{
		MessageID:    messageID,
		SessionID:    sessionID,
		DeltaType:    DeltaTypeToolCall,
		ToolCallID:   toolCallID,
		ToolCallName: toolCallName,
		Timestamp:    time.Now().UnixMilli(),
	}
}

// NewFinishDelta creates a delta indicating the message streaming is complete
func NewFinishDelta(messageID, sessionID, finishReason string) StreamDelta {
	return StreamDelta{
		MessageID:    messageID,
		SessionID:    sessionID,
		DeltaType:    DeltaTypeFinish,
		FinishReason: finishReason,
		Timestamp:    time.Now().UnixMilli(),
	}
}

// NewErrorDelta creates a delta for error notification (shown as toast in frontend)
func NewErrorDelta(sessionID, errorMessage string) StreamDelta {
	return StreamDelta{
		MessageID: "",
		SessionID: sessionID,
		DeltaType: DeltaTypeError,
		Content:   errorMessage,
		Timestamp: time.Now().UnixMilli(),
	}
}
