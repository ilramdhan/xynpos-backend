package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps go-redis with convenience methods for XynPOS use cases.
type Client struct {
	rdb *redis.Client
}

// Config holds Redis connection configuration.
type Config struct {
	URL      string
	Password string
	DB       int
}

// New creates a new Redis client.
func New(cfg Config) (*Client, error) {
	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		// Fallback: treat URL as addr
		opt = &redis.Options{
			Addr:     cfg.URL,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}

	if cfg.Password != "" {
		opt.Password = cfg.Password
	}
	opt.DB = cfg.DB

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Raw returns the underlying go-redis client for advanced usage.
func (c *Client) Raw() *redis.Client {
	return c.rdb
}

// ──────────────────────────────────────────────
// Basic Operations
// ──────────────────────────────────────────────

// Set stores a value with an optional TTL (0 = no expiry).
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := marshal(value)
	if err != nil {
		return fmt.Errorf("redis set %s: marshal: %w", key, err)
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// Get retrieves a value and unmarshals it into dest.
func (c *Client) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err // caller checks redis.Nil
	}
	return json.Unmarshal(data, dest)
}

// GetString retrieves a string value.
func (c *Client) GetString(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Delete removes one or more keys.
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists checks whether a key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

// Incr atomically increments a key by 1 and returns the new value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// IncrBy atomically increments a key by delta.
func (c *Client) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return c.rdb.IncrBy(ctx, key, delta).Result()
}

// Expire sets/resets the TTL on an existing key.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// SetNX sets a key only if it does not exist (atomic, useful for distributed locks).
func (c *Client) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	data, err := marshal(value)
	if err != nil {
		return false, fmt.Errorf("redis setnx %s: marshal: %w", key, err)
	}
	return c.rdb.SetNX(ctx, key, data, ttl).Result()
}

// TTL returns the remaining TTL of a key.
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, key).Result()
}

// ──────────────────────────────────────────────
// Hash Operations (useful for rate limiting counters per field)
// ──────────────────────────────────────────────

// HSet sets fields in a hash.
func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) error {
	return c.rdb.HSet(ctx, key, values...).Err()
}

// HGet gets a field from a hash.
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.rdb.HGet(ctx, key, field).Result()
}

// ──────────────────────────────────────────────
// Pub/Sub (for SSE notification channel)
// ──────────────────────────────────────────────

// Publish sends a message on a channel.
func (c *Client) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := marshal(message)
	if err != nil {
		return fmt.Errorf("redis publish %s: marshal: %w", channel, err)
	}
	return c.rdb.Publish(ctx, channel, data).Err()
}

// Subscribe subscribes to one or more channels. Returns a *redis.PubSub.
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, channels...)
}

// ──────────────────────────────────────────────
// Health
// ──────────────────────────────────────────────

// Ping verifies the Redis connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

// IsNil returns true if the error is a redis.Nil (key not found).
func IsNil(err error) bool {
	return err == redis.Nil
}

func marshal(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case string:
		return []byte(val), nil
	case []byte:
		return val, nil
	default:
		return json.Marshal(v)
	}
}
