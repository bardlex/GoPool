// Package postgres provides PostgreSQL database client and operations for GOMP mining pool.
// It handles persistent storage of users, workers, shares, blocks, and other mining data.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// PostgreSQL driver for database/sql
	_ "github.com/lib/pq"
)

// Client wraps PostgreSQL database operations
type Client struct {
	db *sql.DB
}

// Config holds PostgreSQL connection configuration
type Config struct {
	Host         string
	Port         int
	Database     string
	User         string
	Password     string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

// NewClient creates a new PostgreSQL client
func NewClient(cfg *Config) (*Client, error) {
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.Password, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Client{db: db}, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	return c.db.Close()
}

// Health checks database connectivity
func (c *Client) Health(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// BeginTx starts a new transaction
func (c *Client) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, nil)
}

// DB returns the underlying sql.DB for advanced operations
func (c *Client) DB() *sql.DB {
	return c.db
}
