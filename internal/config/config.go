// Package config provides configuration management for the GOMP mining pool.
// It handles loading configuration from environment variables with sensible defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the global configuration for GOMP services
type Config struct {
	// Service identification
	ServiceName string
	Version     string
	Environment string

	// Network configuration
	ListenAddr string
	ListenPort int

	// Bitcoin Core connection
	BitcoinRPCHost     string
	BitcoinRPCPort     int
	BitcoinRPCUser     string
	BitcoinRPCPassword string
	BitcoinZMQAddr     string

	// Kafka configuration
	KafkaBrokers []string
	KafkaGroupID string

	// Database connections
	PostgresURL  string
	RedisURL     string
	InfluxURL    string
	InfluxToken  string
	InfluxOrg    string
	InfluxBucket string

	// Pool configuration
	PoolFeePercent    float64
	PPLNSWindowFactor float64
	MinDifficulty     float64
	MaxDifficulty     float64
	VardiffTarget     time.Duration
	VardiffRetarget   time.Duration

	// Performance tuning
	MaxConnections int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxMessageSize int
	BufferSize     int
	WorkerPoolSize int

	// Logging
	LogLevel  string
	LogFormat string
}

// Load loads configuration from environment variables with sensible defaults
func Load() (*Config, error) {
	cfg := &Config{
		// Service defaults
		ServiceName: getEnv("SERVICE_NAME", "gomp"),
		Version:     getEnv("VERSION", "dev"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// Network defaults
		ListenAddr: getEnv("LISTEN_ADDR", "0.0.0.0"),
		ListenPort: getEnvInt("LISTEN_PORT", 3333),

		// Bitcoin Core defaults
		BitcoinRPCHost:     getEnv("BITCOIN_RPC_HOST", "localhost"),
		BitcoinRPCPort:     getEnvInt("BITCOIN_RPC_PORT", 8332),
		BitcoinRPCUser:     getEnv("BITCOIN_RPC_USER", ""),
		BitcoinRPCPassword: getEnv("BITCOIN_RPC_PASSWORD", ""),
		BitcoinZMQAddr:     getEnv("BITCOIN_ZMQ_ADDR", "tcp://localhost:28332"),

		// Kafka defaults
		KafkaBrokers: getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "gomp"),

		// Database defaults
		PostgresURL:  getEnv("POSTGRES_URL", "postgres://gomp:gomp@localhost/gomp?sslmode=disable"),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379/0"),
		InfluxURL:    getEnv("INFLUX_URL", "http://localhost:8086"),
		InfluxToken:  getEnv("INFLUX_TOKEN", ""),
		InfluxOrg:    getEnv("INFLUX_ORG", "gomp"),
		InfluxBucket: getEnv("INFLUX_BUCKET", "mining"),

		// Pool defaults
		PoolFeePercent:    getEnvFloat("POOL_FEE_PERCENT", 1.0),
		PPLNSWindowFactor: getEnvFloat("PPLNS_WINDOW_FACTOR", 2.0),
		MinDifficulty:     getEnvFloat("MIN_DIFFICULTY", 1.0),
		MaxDifficulty:     getEnvFloat("MAX_DIFFICULTY", 1000000.0),
		VardiffTarget:     getEnvDuration("VARDIFF_TARGET", 30*time.Second),
		VardiffRetarget:   getEnvDuration("VARDIFF_RETARGET", 90*time.Second),

		// Performance defaults
		MaxConnections: getEnvInt("MAX_CONNECTIONS", 10000),
		ReadTimeout:    getEnvDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:   getEnvDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:    getEnvDuration("IDLE_TIMEOUT", 120*time.Second),
		MaxMessageSize: getEnvInt("MAX_MESSAGE_SIZE", 4096),
		BufferSize:     getEnvInt("BUFFER_SIZE", 8192),
		WorkerPoolSize: getEnvInt("WORKER_POOL_SIZE", 100),

		// Logging defaults
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// validate performs basic validation of configuration values
func (c *Config) validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("SERVICE_NAME cannot be empty")
	}

	if c.ListenPort <= 0 || c.ListenPort > 65535 {
		return fmt.Errorf("LISTEN_PORT must be between 1 and 65535")
	}

	if c.PoolFeePercent < 0 || c.PoolFeePercent > 100 {
		return fmt.Errorf("POOL_FEE_PERCENT must be between 0 and 100")
	}

	if c.PPLNSWindowFactor <= 0 {
		return fmt.Errorf("PPLNS_WINDOW_FACTOR must be positive")
	}

	if c.MinDifficulty <= 0 {
		return fmt.Errorf("MIN_DIFFICULTY must be positive")
	}

	if c.MaxDifficulty <= c.MinDifficulty {
		return fmt.Errorf("MAX_DIFFICULTY must be greater than MIN_DIFFICULTY")
	}

	return nil
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing
		// In production, might want more sophisticated parsing
		return []string{value}
	}
	return defaultValue
}
