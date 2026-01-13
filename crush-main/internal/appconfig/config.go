// Package appconfig provides application configuration management.
package appconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration.
type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Sandbox  SandboxConfig  `yaml:"sandbox"`
	Storage  StorageConfig  `yaml:"storage"`
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
	globalConfig *Config
	configMutex  sync.RWMutex
	configOnce   sync.Once
)

// Load loads the configuration from the YAML file.
// It returns the configuration for the specified environment (development or production).
func Load(configPath string, env string) (*Config, error) {
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
	var configs map[string]Config
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Get the environment-specific config
	config, ok := configs[env]
	if !ok {
		return nil, fmt.Errorf("environment '%s' not found in config file", env)
	}

	// Override with environment variables if they exist
	overrideWithEnv(&config)

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

// overrideWithEnv overrides config values with environment variables if they exist.
func overrideWithEnv(config *Config) {
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
}

// GetGlobal returns the global configuration instance.
// It loads the configuration on first call.
func GetGlobal() *Config {
	configOnce.Do(func() {
		env := getEnv("APP_ENV", "development")
		config, err := Load("", env)
		if err != nil {
			// If config file doesn't exist, use defaults
			config = getDefaultConfig()
		}
		globalConfig = config
	})

	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig
}

// SetGlobal sets the global configuration instance.
func SetGlobal(config *Config) {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfig = config
}

// getDefaultConfig returns a default configuration.
func getDefaultConfig() *Config {
	return &Config{
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
	}
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
