package storage

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/appconfig"
)

// NewClientFromConfig creates a storage client based on application configuration.
func NewClientFromConfig(cfg *appconfig.Config) (*MinIOClient, error) {
	storageCfg := cfg.Storage

	switch storageCfg.Type {
	case "minio":
		return NewMinIOClientFromConfig(storageCfg.MinIO)
	case "oss":
		// TODO: Implement OSS client
		slog.Warn("OSS storage type is not yet implemented, falling back to MinIO")
		return NewMinIOClientFromConfig(storageCfg.MinIO)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageCfg.Type)
	}
}

// NewMinIOClientFromConfig creates a MinIO client from config.
func NewMinIOClientFromConfig(cfg appconfig.MinIOConfig) (*MinIOClient, error) {
	minioCfg := MinIOConfig{
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKey,
		SecretAccessKey: cfg.SecretKey,
		BucketName:      cfg.Bucket,
		UseSSL:          cfg.UseSSL,
		PublicEndpoint:  cfg.PublicEndpoint,
	}
	return NewMinIOClient(minioCfg)
}

// InitGlobalClientFromConfig initializes the global storage client from app config.
func InitGlobalClientFromConfig(cfg *appconfig.Config) error {
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage client: %w", err)
	}
	globalMinIOClient = client
	slog.Info("Global storage client initialized from config", "type", cfg.Storage.Type)
	return nil
}
