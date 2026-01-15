// Package redis provides Redis client for message streaming and caching.
package redis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

var (
	globalClient *Client
	clientOnce   sync.Once
	clientMutex  sync.RWMutex
)

// Client wraps the Redis client with additional functionality.
type Client struct {
	rdb          *redis.Client
	streamMaxLen int64
	streamTTL    time.Duration
}

// NewClient creates a new Redis client from the configuration.
func NewClient(cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	slog.Info("Redis connection established",
		"host", cfg.Host,
		"port", cfg.Port,
		"db", cfg.DB,
	)

	return &Client{
		rdb:          rdb,
		streamMaxLen: cfg.StreamMaxLen,
		streamTTL:    time.Duration(cfg.StreamTTL) * time.Second,
	}, nil
}

// InitGlobalClient initializes the global Redis client.
func InitGlobalClient() error {
	var initErr error
	clientOnce.Do(func() {
		cfg := config.GetGlobalAppConfig()
		client, err := NewClient(cfg.Redis)
		if err != nil {
			initErr = err
			return
		}
		globalClient = client
	})
	return initErr
}

// GetClient returns the global Redis client.
func GetClient() *Client {
	clientMutex.RLock()
	defer clientMutex.RUnlock()
	return globalClient
}

// SetClient sets the global Redis client (mainly for testing).
func SetClient(client *Client) {
	clientMutex.Lock()
	defer clientMutex.Unlock()
	globalClient = client
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Redis returns the underlying Redis client.
func (c *Client) Redis() *redis.Client {
	return c.rdb
}

// StreamMaxLen returns the configured maximum stream length.
func (c *Client) StreamMaxLen() int64 {
	return c.streamMaxLen
}

// StreamTTL returns the configured stream TTL.
func (c *Client) StreamTTL() time.Duration {
	return c.streamTTL
}
