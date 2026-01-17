// Package appconfig provides application configuration management.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// AppConfig holds the complete application configuration.
type AppConfig struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Database  DatabaseConfig  `yaml:"database"`
	Redis     RedisConfig     `yaml:"redis"`
	Sandbox   SandboxConfig   `yaml:"sandbox"`
	Storage   StorageConfig   `yaml:"storage"`
	AutoModel AutoModelConfig `yaml:"auto_model"`
	Email     EmailConfig     `yaml:"email"`
}

// EmailConfig holds email SMTP settings.
type EmailConfig struct {
	SMTPHost    string `yaml:"smtp_host"`
	SMTPPort    string `yaml:"smtp_port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	FromAddress string `yaml:"from_address"`
	FromName    string `yaml:"from_name"`
	UseSSL      bool   `yaml:"use_ssl"`
	CodeExpire  int    `yaml:"code_expire"` // Verification code expire time in minutes
}

// ServerConfig holds server settings.
type ServerConfig struct {
	HTTPPort string `yaml:"http_port"`
	WSPort   string `yaml:"ws_port"`
	Debug    bool   `yaml:"debug"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret       string `yaml:"jwt_secret"`
	TokenExpireHour int    `yaml:"token_expire_hour"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	StreamMaxLen int64  `yaml:"stream_max_len"` // Maximum length of each session's stream
	StreamTTL    int    `yaml:"stream_ttl"`     // Stream expiration time in seconds
}

// AutoModelConfig holds the default "Auto" model configuration.
// When users select "Auto" model, this configuration is used.
type AutoModelConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	SSLMode      string `yaml:"sslmode"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// SandboxConfig holds sandbox service settings.
type SandboxConfig struct {
	BaseURL string `yaml:"base_url"`
	Timeout int    `yaml:"timeout"`
}

// StorageConfig holds object storage settings.
type StorageConfig struct {
	Type  string      `yaml:"type"` // "minio" or "oss"
	MinIO MinIOConfig `yaml:"minio"`
	OSS   OSSConfig   `yaml:"oss"`
}

// MinIOConfig holds MinIO-specific settings.
type MinIOConfig struct {
	Endpoint       string `yaml:"endpoint"`
	AccessKey      string `yaml:"access_key"`
	SecretKey      string `yaml:"secret_key"`
	Bucket         string `yaml:"bucket"`
	UseSSL         bool   `yaml:"use_ssl"`
	PublicEndpoint string `yaml:"public_endpoint"`
}

// OSSConfig holds Aliyun OSS-specific settings.
type OSSConfig struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Bucket          string `yaml:"bucket"`
	UseSSL          bool   `yaml:"use_ssl"`
}

var (
	globalAppConfig *AppConfig
	appConfigMutex  sync.RWMutex
	appConfigOnce   sync.Once
)

// LoadAppConfig loads the configuration from the YAML file.
// It returns the configuration for the specified environment (development or production).
func LoadAppConfig(configPath string, env string) (*AppConfig, error) {
	if env == "" {
		env = getEnv("APP_ENV", "development")
	}

	// If configPath is empty, try to find config.yaml in common locations
	if configPath == "" {
		configPath = findConfigFile()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the YAML file
	var configs map[string]AppConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Get the environment-specific config
	config, ok := configs[env]
	if !ok {
		return nil, fmt.Errorf("environment '%s' not found in config file", env)
	}

	// Override with environment variables if they exist
	overrideWithEnvApp(&config)

	return &config, nil
}

// findConfigFile searches for config.yaml in common locations.
func findConfigFile() string {
	// Try current directory first
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	// Try executable directory
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		configPath := filepath.Join(exeDir, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Default to current directory
	return "config.yaml"
}

// overrideWithEnvApp overrides config values with environment variables if they exist.
func overrideWithEnvApp(config *AppConfig) {
	// Email overrides
	if v := os.Getenv("EMAIL_SMTP_HOST"); v != "" {
		config.Email.SMTPHost = v
	}
	if v := os.Getenv("EMAIL_SMTP_PORT"); v != "" {
		config.Email.SMTPPort = v
	}
	if v := os.Getenv("EMAIL_USERNAME"); v != "" {
		config.Email.Username = v
	}
	if v := os.Getenv("EMAIL_PASSWORD"); v != "" {
		config.Email.Password = v
	}
	if v := os.Getenv("EMAIL_FROM_ADDRESS"); v != "" {
		config.Email.FromAddress = v
	}

	// Database overrides
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		config.Database.Host = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &config.Database.Port)
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		config.Database.User = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		config.Database.Password = v
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		config.Database.Database = v
	}
	if v := os.Getenv("POSTGRES_SSLMODE"); v != "" {
		config.Database.SSLMode = v
	}

	// Sandbox overrides
	if v := os.Getenv("SANDBOX_BASE_URL"); v != "" {
		config.Sandbox.BaseURL = v
	}

	// Storage overrides (MinIO)
	if v := os.Getenv("MINIO_ENDPOINT"); v != "" {
		config.Storage.MinIO.Endpoint = v
	}
	if v := os.Getenv("MINIO_ACCESS_KEY"); v != "" {
		config.Storage.MinIO.AccessKey = v
	}
	if v := os.Getenv("MINIO_SECRET_KEY"); v != "" {
		config.Storage.MinIO.SecretKey = v
	}
	if v := os.Getenv("MINIO_BUCKET"); v != "" {
		config.Storage.MinIO.Bucket = v
	}
	if v := os.Getenv("MINIO_PUBLIC_ENDPOINT"); v != "" {
		config.Storage.MinIO.PublicEndpoint = v
	}

	// Storage overrides (OSS)
	if v := os.Getenv("OSS_ENDPOINT"); v != "" {
		config.Storage.OSS.Endpoint = v
	}
	if v := os.Getenv("OSS_ACCESS_KEY_ID"); v != "" {
		config.Storage.OSS.AccessKeyID = v
	}
	if v := os.Getenv("OSS_ACCESS_KEY_SECRET"); v != "" {
		config.Storage.OSS.AccessKeySecret = v
	}
	if v := os.Getenv("OSS_BUCKET"); v != "" {
		config.Storage.OSS.Bucket = v
	}

	// Redis overrides
	if v := os.Getenv("REDIS_HOST"); v != "" {
		config.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &config.Redis.Port)
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		config.Redis.Password = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		fmt.Sscanf(v, "%d", &config.Redis.DB)
	}
}

// GetGlobalAppConfig returns the global application configuration instance.
// It loads the configuration on first call.
func GetGlobalAppConfig() *AppConfig {
	appConfigOnce.Do(func() {
		env := getEnv("APP_ENV", "development")
		config, err := LoadAppConfig("", env)
		if err != nil {
			// If config file doesn't exist, use defaults
			config = getDefaultAppConfig()
		}
		globalAppConfig = config
	})

	appConfigMutex.RLock()
	defer appConfigMutex.RUnlock()
	return globalAppConfig
}

// SetGlobalAppConfig sets the global configuration instance.
func SetGlobalAppConfig(config *AppConfig) {
	appConfigMutex.Lock()
	defer appConfigMutex.Unlock()
	globalAppConfig = config
}

// getDefaultAppConfig returns a default configuration.
func getDefaultAppConfig() *AppConfig {
	return &AppConfig{
		Server: ServerConfig{
			HTTPPort: "8001",
			WSPort:   "8002",
			Debug:    false,
		},
		Auth: AuthConfig{
			JWTSecret:       "crush-dev-jwt-secret-change-in-production-2024",
			TokenExpireHour: 24,
		},
		Database: DatabaseConfig{
			Host:         "localhost",
			Port:         5432,
			User:         "crush",
			Password:     "123456",
			Database:     "crush",
			SSLMode:      "disable",
			MaxOpenConns: 25,
			MaxIdleConns: 5,
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Password:     "123456",
			DB:           0,
			PoolSize:     10,
			StreamMaxLen: 1000,
			StreamTTL:    3600,
		},
		Sandbox: SandboxConfig{
			BaseURL: "http://localhost:8888",
			Timeout: 300,
		},
		Storage: StorageConfig{
			Type: "minio",
			MinIO: MinIOConfig{
				Endpoint:  "localhost:9000",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin123",
				Bucket:    "crush-images",
				UseSSL:    false,
			},
		},
		Email: EmailConfig{
			SMTPHost:    "smtp.163.com",
			SMTPPort:    "465",
			Username:    "",
			Password:    "",
			FromAddress: "",
			FromName:    "Crush",
			UseSSL:      true,
			CodeExpire:  5,
		},
	}
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
