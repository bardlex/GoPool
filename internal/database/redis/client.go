// Package redis provides Redis client and caching operations for GOMP mining pool.
// It handles session management, hashrate caching, and temporary data storage.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps Redis operations for the mining pool
type Client struct {
	rdb *redis.Client
}

// Config holds Redis connection configuration
type Config struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewClient creates a new Redis client
func NewClient(cfg *Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Health checks Redis connectivity
func (c *Client) Health(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Session management

// SetSession stores a session with expiration
func (c *Client) SetSession(ctx context.Context, sessionID string, data any, expiration time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := fmt.Sprintf("session:%s", sessionID)
	if err := c.rdb.Set(ctx, key, jsonData, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set session: %w", err)
	}

	return nil
}

// GetSession retrieves session data
func (c *Client) GetSession(ctx context.Context, sessionID string, dest any) error {
	key := fmt.Sprintf("session:%s", sessionID)
	jsonData, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("session not found")
		}
		return fmt.Errorf("failed to get session: %w", err)
	}

	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return nil
}

// DeleteSession removes a session
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// ExtendSession extends session expiration
func (c *Client) ExtendSession(ctx context.Context, sessionID string, expiration time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	if err := c.rdb.Expire(ctx, key, expiration).Err(); err != nil {
		return fmt.Errorf("failed to extend session: %w", err)
	}
	return nil
}

// Job management

// SetCurrentJob stores the current mining job
func (c *Client) SetCurrentJob(ctx context.Context, jobData any) error {
	jsonData, err := json.Marshal(jobData)
	if err != nil {
		return fmt.Errorf("failed to marshal job data: %w", err)
	}

	if err := c.rdb.Set(ctx, "current_job", jsonData, 0).Err(); err != nil {
		return fmt.Errorf("failed to set current job: %w", err)
	}

	return nil
}

// GetCurrentJob retrieves the current mining job
func (c *Client) GetCurrentJob(ctx context.Context, dest any) error {
	jsonData, err := c.rdb.Get(ctx, "current_job").Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("no current job")
		}
		return fmt.Errorf("failed to get current job: %w", err)
	}

	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return nil
}

// SetJobTemplate stores a job template with expiration
func (c *Client) SetJobTemplate(ctx context.Context, jobID string, jobData any, expiration time.Duration) error {
	jsonData, err := json.Marshal(jobData)
	if err != nil {
		return fmt.Errorf("failed to marshal job template: %w", err)
	}

	key := fmt.Sprintf("job:%s", jobID)
	if err := c.rdb.Set(ctx, key, jsonData, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set job template: %w", err)
	}

	return nil
}

// GetJobTemplate retrieves a job template
func (c *Client) GetJobTemplate(ctx context.Context, jobID string, dest any) error {
	key := fmt.Sprintf("job:%s", jobID)
	jsonData, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("job template not found")
		}
		return fmt.Errorf("failed to get job template: %w", err)
	}

	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal job template: %w", err)
	}

	return nil
}

// Statistics and counters

// IncrementCounter increments a counter with expiration
func (c *Client) IncrementCounter(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	pipe := c.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, expiration)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}

	return incrCmd.Val(), nil
}

// GetCounter retrieves a counter value
func (c *Client) GetCounter(ctx context.Context, key string) (int64, error) {
	val, err := c.rdb.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get counter: %w", err)
	}
	return val, nil
}

// SetHashrate stores hashrate data for a user/worker
func (c *Client) SetHashrate(ctx context.Context, userID, workerID int64, hashrate float64, window time.Duration) error {
	key := fmt.Sprintf("hashrate:%d:%d", userID, workerID)
	timestamp := time.Now().Unix()

	// Store as sorted set with timestamp as score
	member := &redis.Z{
		Score:  float64(timestamp),
		Member: hashrate,
	}

	pipe := c.rdb.Pipeline()
	pipe.ZAdd(ctx, key, *member)
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", timestamp-int64(window.Seconds())))
	pipe.Expire(ctx, key, window*2) // Keep data a bit longer than window

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to set hashrate: %w", err)
	}

	return nil
}

// GetAverageHashrate calculates average hashrate over a time window
func (c *Client) GetAverageHashrate(ctx context.Context, userID, workerID int64, window time.Duration) (float64, error) {
	key := fmt.Sprintf("hashrate:%d:%d", userID, workerID)
	minScore := time.Now().Add(-window).Unix()

	// Get all hashrate values in the time window
	values, err := c.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", minScore),
		Max: "+inf",
	}).Result()

	if err != nil {
		return 0, fmt.Errorf("failed to get hashrate values: %w", err)
	}

	if len(values) == 0 {
		return 0, nil
	}

	// Calculate average
	var total float64
	for _, val := range values {
		if hashrate, err := strconv.ParseFloat(val, 64); err == nil {
			total += hashrate
		}
	}

	return total / float64(len(values)), nil
}

// Rate limiting

// CheckRateLimit checks if an action is rate limited
func (c *Client) CheckRateLimit(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	pipe := c.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}

	return incrCmd.Val() <= limit, nil
}

// Caching

// SetCache stores data in cache with expiration
func (c *Client) SetCache(ctx context.Context, key string, data any, expiration time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	cacheKey := fmt.Sprintf("cache:%s", key)
	if err := c.rdb.Set(ctx, cacheKey, jsonData, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	return nil
}

// GetCache retrieves data from cache
func (c *Client) GetCache(ctx context.Context, key string, dest any) error {
	cacheKey := fmt.Sprintf("cache:%s", key)
	jsonData, err := c.rdb.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("cache miss")
		}
		return fmt.Errorf("failed to get cache: %w", err)
	}

	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return nil
}

// DeleteCache removes data from cache
func (c *Client) DeleteCache(ctx context.Context, key string) error {
	cacheKey := fmt.Sprintf("cache:%s", key)
	if err := c.rdb.Del(ctx, cacheKey).Err(); err != nil {
		return fmt.Errorf("failed to delete cache: %w", err)
	}
	return nil
}
