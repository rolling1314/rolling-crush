package tools

import (
	"context"
)

type (
	sessionIDContextKey  string
	messageIDContextKey  string
	workingDirContextKey string
)

const (
	SessionIDContextKey  sessionIDContextKey  = "session_id"
	MessageIDContextKey  messageIDContextKey  = "message_id"
	WorkingDirContextKey workingDirContextKey = "working_dir"
)

func GetSessionFromContext(ctx context.Context) string {
	sessionID := ctx.Value(SessionIDContextKey)
	if sessionID == nil {
		return ""
	}
	s, ok := sessionID.(string)
	if !ok {
		return ""
	}
	return s
}

func GetMessageFromContext(ctx context.Context) string {
	messageID := ctx.Value(MessageIDContextKey)
	if messageID == nil {
		return ""
	}
	s, ok := messageID.(string)
	if !ok {
		return ""
	}
	return s
}

func GetWorkingDirFromContext(ctx context.Context) string {
	workingDir := ctx.Value(WorkingDirContextKey)
	if workingDir == nil {
		return ""
	}
	wd, ok := workingDir.(string)
	if !ok {
		return ""
	}
	return wd
}
