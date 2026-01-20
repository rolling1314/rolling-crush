package permission

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rolling1314/rolling-crush/internal/pkg/csync"
	"github.com/rolling1314/rolling-crush/internal/pubsub"
)

var (
	ErrorPermissionDenied  = errors.New("user denied permission")
	ErrorPermissionTimeout = errors.New("permission request timed out")
)

// AllowlistChecker is an interface for checking session-level tool allowlist.
// This is typically implemented by the Redis stream service.
type AllowlistChecker interface {
	IsToolAllowedInSession(ctx context.Context, sessionID, toolName, action, path string) (bool, error)
	AddToSessionAllowlist(ctx context.Context, sessionID string, entry AllowlistEntry) error
}

// AllowlistEntry represents an entry in the session tool allowlist.
type AllowlistEntry struct {
	ToolName string `json:"tool_name"`
	Action   string `json:"action"`
	Path     string `json:"path"`
	AddedAt  int64  `json:"added_at"`
}

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type PermissionNotification struct {
	SessionID  string `json:"session_id"`
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

// PermissionTimeoutCallback is called when a permission request times out.
// It allows the caller to persist the pending state (e.g., to database).
type PermissionTimeoutCallback func(req PermissionRequest, originalPrompt string)

type Service interface {
	pubsub.Suscriber[PermissionRequest]
	GrantPersistent(permission PermissionRequest)
	Grant(permission PermissionRequest)
	GrantForSession(permission PermissionRequest)
	Deny(permission PermissionRequest)
	Request(opts CreatePermissionRequest) bool
	// RequestWithTimeout requests permission with a timeout duration.
	// If timeout occurs, the onTimeout callback is called with the permission request.
	// Returns (granted, error) where error is ErrorPermissionTimeout on timeout,
	// ErrorPermissionDenied on denial, or nil on success.
	RequestWithTimeout(ctx context.Context, opts CreatePermissionRequest, timeout time.Duration, originalPrompt string, onTimeout PermissionTimeoutCallback) (bool, error)
	AutoApproveSession(sessionID string)
	SetSkipRequests(skip bool)
	SkipRequests() bool
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
	SetAllowlistChecker(checker AllowlistChecker)
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	notificationBroker    *pubsub.Broker[PermissionNotification]
	workingDir            string
	sessionPermissions    []PermissionRequest
	sessionPermissionsMu  sync.RWMutex
	pendingRequests       *csync.Map[string, chan bool]
	autoApproveSessions   map[string]bool
	autoApproveSessionsMu sync.RWMutex
	skip                  bool
	allowedTools          []string

	// Per-session request locks and active requests
	sessionRequestMu     *csync.Map[string, *sync.Mutex]
	sessionActiveRequest *csync.Map[string, *PermissionRequest]

	// Allowlist checker for session-level tool allowlist (Redis-backed)
	allowlistChecker   AllowlistChecker
	allowlistCheckerMu sync.RWMutex
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  permission.SessionID,
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	// Clear active request for this session
	if activeReq, ok := s.sessionActiveRequest.Get(permission.SessionID); ok && activeReq != nil && activeReq.ID == permission.ID {
		s.sessionActiveRequest.Del(permission.SessionID)
	}
}

func (s *permissionService) Grant(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  permission.SessionID,
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	// Clear active request for this session
	if activeReq, ok := s.sessionActiveRequest.Get(permission.SessionID); ok && activeReq != nil && activeReq.ID == permission.ID {
		s.sessionActiveRequest.Del(permission.SessionID)
	}
}

// GrantForSession grants permission and adds the tool to the session's allowlist.
// Future requests for the same tool+action combination will be auto-approved.
func (s *permissionService) GrantForSession(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  permission.SessionID,
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	// Add to in-memory session permissions (for backward compatibility)
	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	// Add to Redis allowlist for persistence
	s.allowlistCheckerMu.RLock()
	checker := s.allowlistChecker
	s.allowlistCheckerMu.RUnlock()

	if checker != nil {
		ctx := context.Background()
		entry := AllowlistEntry{
			ToolName: permission.ToolName,
			Action:   permission.Action,
			Path:     permission.Path,
		}
		if err := checker.AddToSessionAllowlist(ctx, permission.SessionID, entry); err != nil {
			slog.Warn("Failed to add tool to session allowlist",
				"error", err,
				"session_id", permission.SessionID,
				"tool_name", permission.ToolName,
				"action", permission.Action,
			)
		} else {
			slog.Info("Tool added to session allowlist",
				"session_id", permission.SessionID,
				"tool_name", permission.ToolName,
				"action", permission.Action,
				"path", permission.Path,
			)
		}
	}

	// Clear active request for this session
	if activeReq, ok := s.sessionActiveRequest.Get(permission.SessionID); ok && activeReq != nil && activeReq.ID == permission.ID {
		s.sessionActiveRequest.Del(permission.SessionID)
	}
}

func (s *permissionService) Deny(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  permission.SessionID,
		ToolCallID: permission.ToolCallID,
		Granted:    false,
		Denied:     true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- false
	}

	// Clear active request for this session
	if activeReq, ok := s.sessionActiveRequest.Get(permission.SessionID); ok && activeReq != nil && activeReq.ID == permission.ID {
		s.sessionActiveRequest.Del(permission.SessionID)
	}
}

func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	if s.skip {
		return true
	}

	// Note: Don't publish notification here - it will be sent via PermissionRequest event
	// The empty notification (granted=false, denied=false) was causing duplicate UI updates

	// Get or create per-session mutex
	sessionMu, _ := s.sessionRequestMu.Get(opts.SessionID)
	if sessionMu == nil {
		sessionMu = &sync.Mutex{}
		s.sessionRequestMu.Set(opts.SessionID, sessionMu)
	}
	sessionMu.Lock()
	defer sessionMu.Unlock()

	// Check if the tool/action combination is in the static allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true
	}

	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()

	if autoApprove {
		return true
	}

	fileInfo, err := os.Stat(opts.Path)
	dir := opts.Path
	if err == nil {
		if fileInfo.IsDir() {
			dir = opts.Path
		} else {
			dir = filepath.Dir(opts.Path)
		}
	}

	if dir == "." {
		dir = s.workingDir
	}

	// Check Redis session allowlist (if available)
	s.allowlistCheckerMu.RLock()
	checker := s.allowlistChecker
	s.allowlistCheckerMu.RUnlock()

	if checker != nil {
		ctx := context.Background()
		allowed, err := checker.IsToolAllowedInSession(ctx, opts.SessionID, opts.ToolName, opts.Action, dir)
		if err != nil {
			slog.Warn("Failed to check session allowlist",
				"error", err,
				"session_id", opts.SessionID,
				"tool_name", opts.ToolName,
			)
		} else if allowed {
			slog.Debug("Tool auto-approved from session allowlist",
				"session_id", opts.SessionID,
				"tool_name", opts.ToolName,
				"action", opts.Action,
			)
			return true
		}
	}

	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	// Check in-memory session permissions (for backward compatibility)
	s.sessionPermissionsMu.RLock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			s.sessionPermissionsMu.RUnlock()
			return true
		}
	}
	s.sessionPermissionsMu.RUnlock()

	// Set active request for this session
	s.sessionActiveRequest.Set(opts.SessionID, &permission)

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)

	return <-respCh
}

// RequestWithTimeout requests permission with a timeout.
// Returns (granted, error) where error is:
// - nil if granted
// - ErrorPermissionDenied if denied
// - ErrorPermissionTimeout if timeout occurs
// - ctx.Err() if context is cancelled
func (s *permissionService) RequestWithTimeout(ctx context.Context, opts CreatePermissionRequest, timeout time.Duration, originalPrompt string, onTimeout PermissionTimeoutCallback) (bool, error) {
	if s.skip {
		return true, nil
	}

	// Get or create per-session mutex
	sessionMu, _ := s.sessionRequestMu.Get(opts.SessionID)
	if sessionMu == nil {
		sessionMu = &sync.Mutex{}
		s.sessionRequestMu.Set(opts.SessionID, sessionMu)
	}
	sessionMu.Lock()
	defer sessionMu.Unlock()

	// Check if the tool/action combination is in the static allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true, nil
	}

	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()

	if autoApprove {
		return true, nil
	}

	fileInfo, err := os.Stat(opts.Path)
	dir := opts.Path
	if err == nil {
		if fileInfo.IsDir() {
			dir = opts.Path
		} else {
			dir = filepath.Dir(opts.Path)
		}
	}

	if dir == "." {
		dir = s.workingDir
	}

	// Check Redis session allowlist (if available)
	s.allowlistCheckerMu.RLock()
	checker := s.allowlistChecker
	s.allowlistCheckerMu.RUnlock()

	if checker != nil {
		allowed, err := checker.IsToolAllowedInSession(ctx, opts.SessionID, opts.ToolName, opts.Action, dir)
		if err != nil {
			slog.Warn("Failed to check session allowlist",
				"error", err,
				"session_id", opts.SessionID,
				"tool_name", opts.ToolName,
			)
		} else if allowed {
			slog.Debug("Tool auto-approved from session allowlist",
				"session_id", opts.SessionID,
				"tool_name", opts.ToolName,
				"action", opts.Action,
			)
			return true, nil
		}
	}

	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	// Check in-memory session permissions (for backward compatibility)
	s.sessionPermissionsMu.RLock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			s.sessionPermissionsMu.RUnlock()
			return true, nil
		}
	}
	s.sessionPermissionsMu.RUnlock()

	// Set active request for this session
	s.sessionActiveRequest.Set(opts.SessionID, &permission)

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer func() {
		s.pendingRequests.Del(permission.ID)
		// Clear active request
		if activeReq, ok := s.sessionActiveRequest.Get(permission.SessionID); ok && activeReq != nil && activeReq.ID == permission.ID {
			s.sessionActiveRequest.Del(permission.SessionID)
		}
	}()

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)

	slog.Info("[GOROUTINE] Permission request started with timeout",
		"permission_id", permission.ID,
		"session_id", opts.SessionID,
		"tool_name", opts.ToolName,
		"timeout", timeout,
	)

	// Wait with timeout
	select {
	case granted := <-respCh:
		if granted {
			slog.Info("[GOROUTINE] Permission granted",
				"permission_id", permission.ID,
				"session_id", opts.SessionID,
			)
			return true, nil
		}
		slog.Info("[GOROUTINE] Permission denied",
			"permission_id", permission.ID,
			"session_id", opts.SessionID,
		)
		return false, ErrorPermissionDenied

	case <-time.After(timeout):
		slog.Warn("[GOROUTINE] Permission request timed out",
			"permission_id", permission.ID,
			"session_id", opts.SessionID,
			"tool_name", opts.ToolName,
			"timeout", timeout,
		)
		// Call the timeout callback to persist state
		if onTimeout != nil {
			onTimeout(permission, originalPrompt)
		}
		return false, ErrorPermissionTimeout

	case <-ctx.Done():
		slog.Info("[GOROUTINE] Permission request cancelled",
			"permission_id", permission.ID,
			"session_id", opts.SessionID,
			"reason", ctx.Err(),
		)
		return false, ctx.Err()
	}
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions[sessionID] = true
	s.autoApproveSessionsMu.Unlock()
}

func (s *permissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification] {
	return s.notificationBroker.Subscribe(ctx)
}

func (s *permissionService) SetSkipRequests(skip bool) {
	s.skip = skip
}

func (s *permissionService) SkipRequests() bool {
	return s.skip
}

// SetAllowlistChecker sets the Redis-backed allowlist checker.
// This should be called after the Redis client is initialized.
func (s *permissionService) SetAllowlistChecker(checker AllowlistChecker) {
	s.allowlistCheckerMu.Lock()
	s.allowlistChecker = checker
	s.allowlistCheckerMu.Unlock()
	slog.Info("Allowlist checker set for permission service")
}

func NewPermissionService(workingDir string, skip bool, allowedTools []string) Service {
	return &permissionService{
		Broker:               pubsub.NewBroker[PermissionRequest](),
		notificationBroker:   pubsub.NewBroker[PermissionNotification](),
		workingDir:           workingDir,
		sessionPermissions:   make([]PermissionRequest, 0),
		autoApproveSessions:  make(map[string]bool),
		skip:                 skip,
		allowedTools:         allowedTools,
		pendingRequests:      csync.NewMap[string, chan bool](),
		sessionRequestMu:     csync.NewMap[string, *sync.Mutex](),
		sessionActiveRequest: csync.NewMap[string, *PermissionRequest](),
	}
}
