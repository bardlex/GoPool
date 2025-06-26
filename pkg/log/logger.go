// Package log provides structured logging utilities for the GOMP mining pool.
// It wraps the standard library's slog package with additional convenience methods.
package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog.Logger with additional context and convenience methods
type Logger struct {
	*slog.Logger
	service string
	version string
}

// New creates a new logger with the specified configuration
func New(service, version, level, format string) *Logger {
	var handler slog.Handler

	// Parse log level
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Create handler based on format
	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: logLevel == slog.LevelDebug,
	}

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	// Create base logger with service context
	baseLogger := slog.New(handler).With(
		"service", service,
		"version", version,
	)

	return &Logger{
		Logger:  baseLogger,
		service: service,
		version: version,
	}
}

// WithContext returns a logger with additional context fields
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract common context values if they exist
	logger := l.Logger

	// Add request ID if available
	if reqID := ctx.Value("request_id"); reqID != nil {
		logger = logger.With("request_id", reqID)
	}

	// Add trace ID if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		logger = logger.With("trace_id", traceID)
	}

	return &Logger{
		Logger:  logger,
		service: l.service,
		version: l.version,
	}
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields ...any) *Logger {
	return &Logger{
		Logger:  l.With(fields...),
		service: l.service,
		version: l.version,
	}
}

// WithComponent returns a logger with a component field
func (l *Logger) WithComponent(component string) *Logger {
	return l.WithFields("component", component)
}

// WithMiner returns a logger with miner-specific fields
func (l *Logger) WithMiner(address, worker string) *Logger {
	return l.WithFields("miner_address", address, "worker_name", worker)
}

// WithJob returns a logger with job-specific fields
func (l *Logger) WithJob(jobID string, blockHeight int64) *Logger {
	return l.WithFields("job_id", jobID, "block_height", blockHeight)
}

// WithShare returns a logger with share-specific fields
func (l *Logger) WithShare(shareID string, difficulty float64) *Logger {
	return l.WithFields("share_id", shareID, "difficulty", difficulty)
}

// WithError returns a logger with error context
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithFields("error", err.Error())
}

// Performance logging helpers

// LogDuration logs the duration of an operation
func (l *Logger) LogDuration(operation string, duration int64) {
	l.Info("operation completed",
		"operation", operation,
		"duration_ns", duration,
		"duration_ms", float64(duration)/1e6,
	)
}

// LogThroughput logs throughput metrics
func (l *Logger) LogThroughput(operation string, count int64, duration int64) {
	throughput := float64(count) / (float64(duration) / 1e9) // ops per second
	l.Info("throughput metrics",
		"operation", operation,
		"count", count,
		"duration_ns", duration,
		"throughput_ops_sec", throughput,
	)
}

// Connection logging helpers

// LogConnection logs connection events
func (l *Logger) LogConnection(event, remoteAddr string) {
	l.Info("connection event",
		"event", event,
		"remote_addr", remoteAddr,
	)
}

// LogStratumMessage logs Stratum protocol messages (debug level)
func (l *Logger) LogStratumMessage(direction, message string) {
	l.Debug("stratum message",
		"direction", direction,
		"message", message,
	)
}

// Mining-specific logging helpers

// LogShareSubmission logs share submissions
func (l *Logger) LogShareSubmission(minerAddr, workerName, jobID string, difficulty float64, status string) {
	l.Info("share submission",
		"miner_address", minerAddr,
		"worker_name", workerName,
		"job_id", jobID,
		"difficulty", difficulty,
		"status", status,
	)
}

// LogBlockFound logs when a block is found
func (l *Logger) LogBlockFound(blockHash string, blockHeight int64, minerAddr, workerName string, difficulty float64) {
	l.Info("block found",
		"block_hash", blockHash,
		"block_height", blockHeight,
		"miner_address", minerAddr,
		"worker_name", workerName,
		"difficulty", difficulty,
	)
}

// LogJobDistribution logs job distribution
func (l *Logger) LogJobDistribution(jobID string, blockHeight int64, cleanJobs bool, minerCount int) {
	l.Info("job distributed",
		"job_id", jobID,
		"block_height", blockHeight,
		"clean_jobs", cleanJobs,
		"miner_count", minerCount,
	)
}
