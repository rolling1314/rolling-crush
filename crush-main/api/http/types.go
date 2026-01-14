package http

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the response for login/register operations
type LoginResponse struct {
	Success bool      `json:"success"`
	Token   string    `json:"token,omitempty"`
	Message string    `json:"message,omitempty"`
	User    *UserInfo `json:"user,omitempty"`
}

// UserInfo represents user information in responses
type UserInfo struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ProjectRequest represents a project create/update request
type ProjectRequest struct {
	Name             string  `json:"name" binding:"required"`
	Description      string  `json:"description"`
	ExternalIP       string  `json:"external_ip"`
	FrontendPort     int32   `json:"frontend_port"`
	WorkspacePath    string  `json:"workspace_path"`
	ContainerName    *string `json:"container_name,omitempty"`
	WorkdirPath      *string `json:"workdir_path,omitempty"`
	DbHost           *string `json:"db_host,omitempty"`
	DbPort           *int32  `json:"db_port,omitempty"`
	DbUser           *string `json:"db_user,omitempty"`
	DbPassword       *string `json:"db_password,omitempty"`
	DbName           *string `json:"db_name,omitempty"`
	BackendPort      *int32  `json:"backend_port,omitempty"`
	FrontendCommand  *string `json:"frontend_command,omitempty"`
	FrontendLanguage *string `json:"frontend_language,omitempty"`
	BackendCommand   *string `json:"backend_command,omitempty"`
	BackendLanguage  *string `json:"backend_language,omitempty"`
	NeedDatabase     bool    `json:"need_database"`
}

// ProjectResponse represents a project in API responses
type ProjectResponse struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	ExternalIP       string  `json:"external_ip"`
	FrontendPort     int32   `json:"frontend_port"`
	WorkspacePath    string  `json:"workspace_path"`
	ContainerName    *string `json:"container_name,omitempty"`
	WorkdirPath      *string `json:"workdir_path,omitempty"`
	DbHost           *string `json:"db_host,omitempty"`
	DbPort           *int32  `json:"db_port,omitempty"`
	DbUser           *string `json:"db_user,omitempty"`
	DbPassword       *string `json:"db_password,omitempty"`
	DbName           *string `json:"db_name,omitempty"`
	BackendPort      *int32  `json:"backend_port,omitempty"`
	FrontendCommand  *string `json:"frontend_command,omitempty"`
	FrontendLanguage *string `json:"frontend_language,omitempty"`
	BackendCommand   *string `json:"backend_command,omitempty"`
	BackendLanguage  *string `json:"backend_language,omitempty"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

// SessionResponse represents a session in API responses
type SessionResponse struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"project_id"`
	Title            string  `json:"title"`
	MessageCount     int64   `json:"message_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	ContextWindow    int64   `json:"context_window"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

// SessionModelConfig represents model configuration for a session
type SessionModelConfig struct {
	Provider        string   `json:"provider" binding:"required"`
	Model           string   `json:"model" binding:"required"`
	BaseURL         string   `json:"base_url"`
	APIKey          string   `json:"api_key"`
	MaxTokens       *int64   `json:"max_tokens"`
	Temperature     *float64 `json:"temperature"`
	TopP            *float64 `json:"top_p"`
	ReasoningEffort string   `json:"reasoning_effort"`
	Think           bool     `json:"think"`
}

// CreateSessionRequest represents a request to create a new session
// Note: model_config is now optional. If not provided or is_auto is true,
// the system will use the auto model configuration from config.yaml
type CreateSessionRequest struct {
	ProjectID   string              `json:"project_id" binding:"required"`
	Title       string              `json:"title" binding:"required"`
	ModelConfig *SessionModelConfig `json:"model_config"`
	IsAuto      bool                `json:"is_auto"` // If true, use auto model config
}

// SessionConfigResponse represents the model configuration for a session
type SessionConfigResponse struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	APIKey          string   `json:"api_key"` // Masked for security
	BaseURL         string   `json:"base_url,omitempty"`
	MaxTokens       *int64   `json:"max_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
}

// UpdateSessionConfigRequest represents the request to update session model configuration
type UpdateSessionConfigRequest struct {
	Provider        string   `json:"provider" binding:"required"`
	Model           string   `json:"model" binding:"required"`
	APIKey          string   `json:"api_key"` // Optional - only update if provided
	BaseURL         string   `json:"base_url"`
	MaxTokens       *int64   `json:"max_tokens"`
	Temperature     *float64 `json:"temperature"`
	TopP            *float64 `json:"top_p"`
	ReasoningEffort string   `json:"reasoning_effort"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// ProviderInfo represents provider information in API responses
type ProviderInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Type    string `json:"type"`
}

// ModelInfo represents model information in API responses
type ModelInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	DefaultMaxTokens int64  `json:"default_max_tokens"`
}

// TestConnectionRequest represents a request to test provider connection
type TestConnectionRequest struct {
	Provider string `json:"provider" binding:"required"`
	Model    string `json:"model" binding:"required"`
	APIKey   string `json:"api_key" binding:"required"`
	BaseURL  string `json:"base_url"`
}

// ConfigureProviderRequest represents a request to configure a provider
type ConfigureProviderRequest struct {
	Provider        string `json:"provider" binding:"required"`
	Model           string `json:"model" binding:"required"`
	APIKey          string `json:"api_key" binding:"required"`
	BaseURL         string `json:"base_url"`
	MaxTokens       *int64 `json:"max_tokens"`
	ReasoningEffort string `json:"reasoning_effort"`
	SetAsDefault    bool   `json:"set_as_default"` // Reserved but not used
}
