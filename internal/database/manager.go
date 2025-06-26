// Package database provides unified database management for GOMP mining pool.
// It coordinates operations across PostgreSQL, Redis, and InfluxDB databases.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bardlex/gomp/internal/database/influx"
	"github.com/bardlex/gomp/internal/database/postgres"
	"github.com/bardlex/gomp/internal/database/redis"
	"github.com/bardlex/gomp/pkg/circuit"
	"github.com/bardlex/gomp/pkg/errors"
	"github.com/bardlex/gomp/pkg/retry"
)

// Manager coordinates all database operations across PostgreSQL, Redis, and InfluxDB
type Manager struct {
	Postgres *postgres.Client
	Redis    *redis.Client
	Influx   *influx.Client

	// Repositories
	Users   *postgres.UserRepository
	Workers *postgres.WorkerRepository
	Shares  *postgres.ShareRepository
	Blocks  *postgres.BlockRepository

	// Error handling
	circuitBreaker *circuit.Breaker
	retryConfig    *retry.Config
}

// Config holds configuration for all database systems
type Config struct {
	Postgres *postgres.Config
	Redis    *redis.Config
	Influx   *influx.Config
}

// NewManager creates a new database manager with all connections
func NewManager(cfg *Config) (*Manager, error) {
	// Initialize PostgreSQL
	pgClient, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeDatabase, "postgres_connection", 
			"failed to connect to PostgreSQL database")
	}

	// Initialize Redis
	redisClient, err := redis.NewClient(cfg.Redis)
	if err != nil {
		if closeErr := pgClient.Close(); closeErr != nil {
			// Wrap both errors
			origErr := errors.Wrap(err, errors.ErrorTypeDatabase, "redis_connection", 
				"failed to connect to Redis database")
			closeErr = errors.Wrap(closeErr, errors.ErrorTypeDatabase, "postgres_cleanup", 
				"failed to close PostgreSQL connection during error cleanup")
			return nil, errors.New(errors.ErrorTypeDatabase, "connection_failure", 
				"multiple database connection failures").
				WithContext("redis_error", origErr.Error()).
				WithContext("postgres_cleanup_error", closeErr.Error())
		}
		return nil, errors.Wrap(err, errors.ErrorTypeDatabase, "redis_connection", 
			"failed to connect to Redis database")
	}

	// Initialize InfluxDB
	influxClient, err := influx.NewClient(cfg.Influx)
	if err != nil {
		var closeErrs []error
		if closeErr := pgClient.Close(); closeErr != nil {
			closeErrs = append(closeErrs, closeErr)
		}
		if closeErr := redisClient.Close(); closeErr != nil {
			closeErrs = append(closeErrs, closeErr)
		}
		
		origErr := errors.Wrap(err, errors.ErrorTypeDatabase, "influx_connection", 
			"failed to connect to InfluxDB database")
		
		if len(closeErrs) > 0 {
			return nil, origErr.WithContext("cleanup_errors", fmt.Sprintf("%v", closeErrs))
		}
		return nil, origErr
	}

	// Configure error handling
	cbConfig := &circuit.Config{
		MaxFailures:     3,
		SuccessRequired: 2,
		Timeout:         30 * time.Second,
		ResetTimeout:    60 * time.Second,
	}

	// Create repositories
	users := postgres.NewUserRepository(pgClient.DB())
	workers := postgres.NewWorkerRepository(pgClient.DB())
	shares := postgres.NewShareRepository(pgClient.DB())
	blocks := postgres.NewBlockRepository(pgClient.DB())

	return &Manager{
		Postgres:       pgClient,
		Redis:          redisClient,
		Influx:         influxClient,
		Users:          users,
		Workers:        workers,
		Shares:         shares,
		Blocks:         blocks,
		circuitBreaker: circuit.New(cbConfig),
		retryConfig:    retry.DatabaseConfig(),
	}, nil
}

// Close closes all database connections
func (m *Manager) Close() error {
	var errs []error

	if err := m.Postgres.Close(); err != nil {
		errs = append(errs, fmt.Errorf("PostgreSQL close error: %w", err))
	}

	if err := m.Redis.Close(); err != nil {
		errs = append(errs, fmt.Errorf("redis close error: %w", err))
	}

	m.Influx.Close()

	if len(errs) > 0 {
		return fmt.Errorf("database close errors: %v", errs)
	}

	return nil
}

// Health checks the health of all database connections
func (m *Manager) Health(ctx context.Context) error {
	if err := m.Postgres.Health(ctx); err != nil {
		return fmt.Errorf("PostgreSQL health check failed: %w", err)
	}

	if err := m.Redis.Health(ctx); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	if err := m.Influx.Health(ctx); err != nil {
		return fmt.Errorf("InfluxDB health check failed: %w", err)
	}

	return nil
}

// High-level operations that coordinate across multiple databases

// RecordShare records a share across all relevant databases
func (m *Manager) RecordShare(ctx context.Context, share *postgres.Share) error {
	return m.circuitBreaker.Execute(ctx, func() error {
		return retry.Do(ctx, m.retryConfig, func() error {
			// Store in PostgreSQL for persistence (critical operation)
			if err := m.Shares.CreateShare(ctx, share); err != nil {
				return errors.Wrap(err, errors.ErrorTypeDatabase, "record_share", 
					"failed to store share in PostgreSQL").
					WithContext("user_id", share.UserID).
					WithContext("worker_id", share.WorkerID).
					WithContext("share_difficulty", share.Difficulty)
			}

			// Record metrics in InfluxDB (best effort, don't retry on failure)
			m.Influx.WriteShareMetric(
				share.UserID,
				share.WorkerID,
				share.Difficulty,
				share.NetworkDifficulty,
				share.IsValid,
				share.IsBlockCandidate,
			)

			// Update hashrate in Redis (best effort, don't fail on error)
			hashrate := share.Difficulty * 4294967296 / 600 // Approximate hashrate
			if err := m.Redis.SetHashrate(ctx, share.UserID, share.WorkerID, hashrate, 10*time.Minute); err != nil {
				// Create a non-retryable error for Redis failures to avoid blocking
				redisErr := errors.Wrap(err, errors.ErrorTypeDatabase, "redis_hashrate_update", 
					"failed to update hashrate in Redis (non-critical)")
				redisErr.Retryable = false
				// Log but don't fail the operation
				fmt.Printf("Warning: %v\n", redisErr)
			}

			return nil
		})
	})
}

// RecordBlock records a found block across all databases
func (m *Manager) RecordBlock(ctx context.Context, block *postgres.Block) error {
	return m.circuitBreaker.Execute(ctx, func() error {
		return retry.Do(ctx, m.retryConfig, func() error {
			// Store in PostgreSQL (critical operation)
			if err := m.Blocks.CreateBlock(ctx, block); err != nil {
				return errors.Wrap(err, errors.ErrorTypeDatabase, "record_block", 
					"failed to store block in PostgreSQL").
					WithContext("block_hash", block.Hash).
					WithContext("block_height", block.Height).
					WithContext("user_id", block.UserID).
					WithContext("worker_id", block.WorkerID)
			}

			// Record metrics in InfluxDB (best effort)
			m.Influx.WriteBlockMetric(
				block.Height,
				block.Hash,
				block.UserID,
				block.WorkerID,
				block.Difficulty,
				block.Reward,
				block.Status,
			)

			// Cache block info in Redis for quick access (best effort)
			blockKey := fmt.Sprintf("block:%d", block.Height)
			if err := m.Redis.SetCache(ctx, blockKey, block, 24*time.Hour); err != nil {
				redisErr := errors.Wrap(err, errors.ErrorTypeDatabase, "redis_block_cache", 
					"failed to cache block in Redis (non-critical)")
				redisErr.Retryable = false
				fmt.Printf("Warning: %v\n", redisErr)
			}

			return nil
		})
	})
}

// GetUserWithStats retrieves user data with aggregated statistics
func (m *Manager) GetUserWithStats(ctx context.Context, address string) (*UserWithStats, error) {
	// Get user from PostgreSQL
	user, err := m.Users.GetUserByAddress(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get current hashrate from Redis
	hashrate, err := m.Redis.GetAverageHashrate(ctx, user.ID, 0, 10*time.Minute)
	if err != nil {
		hashrate = 0 // Default to 0 if not available
	}

	// Get share statistics from InfluxDB
	shareStats, err := m.Influx.GetShareStats(ctx, user.ID, 24*time.Hour)
	if err != nil {
		shareStats = &influx.ShareStats{} // Default empty stats
	}

	return &UserWithStats{
		User:       user,
		Hashrate:   hashrate,
		ShareStats: shareStats,
	}, nil
}

// GetPoolStats retrieves comprehensive pool statistics
func (m *Manager) GetPoolStats(ctx context.Context) (*PoolStats, error) {
	// Get current pool hashrate from InfluxDB
	poolHashrate, err := m.Influx.GetPoolHashrate(ctx, 10*time.Minute)
	if err != nil {
		poolHashrate = 0
	}

	// Get recent blocks from PostgreSQL
	recentBlocks, err := m.Blocks.GetRecentBlocks(ctx, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent blocks: %w", err)
	}

	// Get active connections from Redis (if tracked)
	activeConnections, _ := m.Redis.GetCounter(ctx, "active_connections")

	return &PoolStats{
		TotalHashrate:     poolHashrate,
		ActiveConnections: activeConnections,
		RecentBlocks:      recentBlocks,
		LastUpdated:       time.Now(),
	}, nil
}

// StartPeriodicTasks starts background tasks for database maintenance
func (m *Manager) StartPeriodicTasks(ctx context.Context) {
	// Refresh PostgreSQL materialized views every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := m.Postgres.DB().ExecContext(ctx, "SELECT refresh_user_stats()"); err != nil {
					fmt.Printf("Warning: failed to refresh user stats: %v\n", err)
				}
			}
		}
	}()

	// Flush InfluxDB writes every 10 seconds
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.Influx.Flush()
			}
		}
	}()

	// Write pool statistics every minute
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats, err := m.GetPoolStats(ctx)
				if err != nil {
					fmt.Printf("Warning: failed to get pool stats: %v\n", err)
					continue
				}

				m.Influx.WritePoolStatsMetric(
					stats.TotalHashrate,
					stats.ActiveConnections,
					0,       // active users - would need to be calculated
					0, 0, 0, // share counts - would need to be calculated
					0, 0, // network stats - would come from Bitcoin Core
				)
			}
		}
	}()
}

// Data structures

// UserWithStats combines user data with real-time statistics
type UserWithStats struct {
	User       *postgres.User
	Hashrate   float64
	ShareStats *influx.ShareStats
}

// PoolStats represents comprehensive pool statistics
type PoolStats struct {
	TotalHashrate     float64
	ActiveConnections int64
	RecentBlocks      []*postgres.Block
	LastUpdated       time.Time
}
