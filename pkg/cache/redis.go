package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	applog "github.com/arixbit/ginblade/pkg/log"
)

const redisPingTimeout = 5 * time.Second

// ErrCacheMiss is returned by Get when a key does not exist in Redis.
var ErrCacheMiss = errors.New("cache: key not found")

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Client wraps a Redis client.
type Client struct {
	rdb *redis.Client
}

// NewTestClient wraps an existing Redis client. It is intended for tests.
func NewTestClient(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// NewClient creates and verifies a Redis client.
func NewClient(cfg RedisConfig) (*Client, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	defer cancel()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("connect to redis: %w", err)
	}

	applog.L().Info("redis connection successful", zap.String("addr", cfg.Addr), zap.Int("db", cfg.DB))
	return &Client{rdb: rdb}, nil
}

// Underlying returns the raw Redis client for advanced use.
func (c *Client) Underlying() *redis.Client {
	if c == nil {
		return nil
	}
	return c.rdb
}

// Ping verifies that Redis is reachable.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis client is not configured")
	}
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// Get retrieves a string value by key. Returns ErrCacheMiss when the key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if c == nil || c.rdb == nil {
		return "", fmt.Errorf("redis client is not configured")
	}
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", fmt.Errorf("redis get %q: %w", key, err)
	}
	return val, nil
}

// Set stores a string value with an optional TTL. Pass 0 for no expiration.
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis client is not configured")
	}
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}
