// Package storage provides object storage functionality using MinIO.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig holds the configuration for MinIO client.
type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
	PublicEndpoint  string // Optional: public endpoint for generating URLs (e.g., for docker/k8s environments)
}

// DefaultMinIOConfig returns a default MinIO configuration from app config.
func DefaultMinIOConfig() MinIOConfig {
	// 尝试从应用配置加载
	// 注意：需要在调用前初始化 appconfig
	// 如果配置未找到，回退到环境变量
	
	// 优先使用环境变量（保持向后兼容）
	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		return MinIOConfig{
			Endpoint:        endpoint,
			AccessKeyID:     getEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: getEnvOrDefault("MINIO_SECRET_KEY", "minioadmin123"),
			BucketName:      getEnvOrDefault("MINIO_BUCKET", "crush-images"),
			UseSSL:          getEnvOrDefault("MINIO_USE_SSL", "false") == "true",
			PublicEndpoint:  getEnvOrDefault("MINIO_PUBLIC_ENDPOINT", ""),
		}
	}
	
	// 使用默认配置
	return MinIOConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin123",
		BucketName:      "crush-images",
		UseSSL:          false,
		PublicEndpoint:  "",
	}
}

// NewMinIOConfigFromAppConfig creates MinIO config from application config.
func NewMinIOConfigFromAppConfig(cfg interface{}) MinIOConfig {
	// 接收 appconfig.Config 类型，避免循环依赖
	// 使用类型断言或反射获取配置值
	return DefaultMinIOConfig()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MinIOClient wraps the MinIO client with convenience methods.
type MinIOClient struct {
	client         *minio.Client
	bucketName     string
	publicEndpoint string
	useSSL         bool
}

// NewMinIOClient creates a new MinIO client with the given configuration.
func NewMinIOClient(cfg MinIOConfig) (*MinIOClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	mc := &MinIOClient{
		client:         client,
		bucketName:     cfg.BucketName,
		publicEndpoint: cfg.PublicEndpoint,
		useSSL:         cfg.UseSSL,
	}

	// Ensure the bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := mc.ensureBucket(ctx); err != nil {
		return nil, err
	}

	slog.Info("MinIO client initialized", "endpoint", cfg.Endpoint, "bucket", cfg.BucketName)
	return mc, nil
}

// ensureBucket creates the bucket if it doesn't exist.
func (m *MinIOClient) ensureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = m.client.MakeBucket(ctx, m.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		slog.Info("Created MinIO bucket", "bucket", m.bucketName)

		// Set bucket policy to allow public read access
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}]
		}`, m.bucketName)

		err = m.client.SetBucketPolicy(ctx, m.bucketName, policy)
		if err != nil {
			slog.Warn("Failed to set bucket policy for public read access", "error", err)
		}
	}

	return nil
}

// UploadResult contains the result of an upload operation.
type UploadResult struct {
	URL      string `json:"url"`
	ObjectID string `json:"object_id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// UploadFile uploads a file to MinIO and returns the result.
func (m *MinIOClient) UploadFile(ctx context.Context, filename string, data []byte, contentType string) (*UploadResult, error) {
	// Generate unique object ID
	objectID := uuid.New().String()
	ext := path.Ext(filename)
	objectName := objectID + ext

	reader := bytes.NewReader(data)
	size := int64(len(data))

	_, err := m.client.PutObject(ctx, m.bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	// Generate URL
	objectURL := m.getObjectURL(objectName)

	slog.Info("File uploaded to MinIO",
		"object_id", objectID,
		"filename", filename,
		"size", size,
		"content_type", contentType,
		"url", objectURL,
	)

	return &UploadResult{
		URL:      objectURL,
		ObjectID: objectID,
		Filename: filename,
		MimeType: contentType,
		Size:     size,
	}, nil
}

// getObjectURL generates the public URL for an object.
func (m *MinIOClient) getObjectURL(objectName string) string {
	endpoint := m.publicEndpoint
	if endpoint == "" {
		endpoint = m.client.EndpointURL().Host
	}

	scheme := "http"
	if m.useSSL {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", scheme, endpoint, m.bucketName, objectName)
}

// GetFile downloads a file from MinIO.
func (m *MinIOClient) GetFile(ctx context.Context, objectURL string) ([]byte, string, error) {
	// Extract object name from URL
	objectName, err := m.extractObjectName(objectURL)
	if err != nil {
		return nil, "", err
	}

	obj, err := m.client.GetObject(ctx, m.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	// Get object info for content type
	info, err := obj.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat object: %w", err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read object: %w", err)
	}

	return data, info.ContentType, nil
}

// extractObjectName extracts the object name from a MinIO URL.
func (m *MinIOClient) extractObjectName(objectURL string) (string, error) {
	parsed, err := url.Parse(objectURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// URL format: http://endpoint/bucket/objectName
	path := strings.TrimPrefix(parsed.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid object URL format: %s", objectURL)
	}

	return parts[1], nil
}

// IsMinIOURL checks if a URL points to the configured MinIO storage.
func (m *MinIOClient) IsMinIOURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	endpoint := m.publicEndpoint
	if endpoint == "" {
		endpoint = m.client.EndpointURL().Host
	}

	return strings.Contains(parsed.Host, endpoint) ||
		strings.Contains(urlStr, m.bucketName)
}

// ValidImageTypes returns the list of valid image MIME types.
func ValidImageTypes() []string {
	return []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
	}
}

// IsValidImageType checks if the content type is a valid image type.
func IsValidImageType(contentType string) bool {
	for _, valid := range ValidImageTypes() {
		if contentType == valid {
			return true
		}
	}
	return false
}

// Global MinIO client instance
var globalMinIOClient *MinIOClient

// InitGlobalMinIOClient initializes the global MinIO client.
func InitGlobalMinIOClient() error {
	cfg := DefaultMinIOConfig()
	client, err := NewMinIOClient(cfg)
	if err != nil {
		return err
	}
	globalMinIOClient = client
	return nil
}

// GetMinIOClient returns the global MinIO client.
func GetMinIOClient() *MinIOClient {
	return globalMinIOClient
}
