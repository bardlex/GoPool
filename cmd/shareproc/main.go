// Package main implements shareproc service for GOMP mining pool.
// This service validates mining shares and identifies block candidates.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bardlex/gomp/internal/config"
	"github.com/bardlex/gomp/internal/messaging"
	"github.com/bardlex/gomp/internal/validation"
	"github.com/bardlex/gomp/pkg/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	logger.Info("starting shareproc",
		"version", cfg.Version,
		"worker_pool_size", cfg.WorkerPoolSize,
	)

	// Create Kafka client
	kafkaClient := messaging.NewKafkaClient(
		cfg.KafkaBrokers,
		logger.Logger,
	)

	// Create the share processor
	processor := NewShareProcessor(cfg, logger, kafkaClient)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the processor
	go func() {
		if err := processor.Start(ctx); err != nil {
			logger.WithError(err).Error("share processor failed")
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("shutdown signal received")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := processor.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("shutdown failed")
		os.Exit(1)
	}

	logger.Info("shareproc stopped")
}

// ShareProcessor processes and validates mining shares
type ShareProcessor struct {
	cfg         *config.Config
	logger      *log.Logger
	validator   *validation.ShareValidator
	kafkaClient *messaging.KafkaClient

	// Job cache for validation
	jobCache map[string]*validation.MiningJob

	// Worker pool
	shareQueue chan *ShareSubmission
	done       chan struct{}
}

// ShareSubmission represents a share submission for processing
type ShareSubmission struct {
	Share      *validation.Share
	SessionID  string
	RemoteAddr string
	ResultChan chan *ShareResult
}

// ShareResult represents the result of share processing
type ShareResult struct {
	ShareID          string
	Status           ShareStatus
	ErrorMessage     string
	IsBlockCandidate bool
}

// ShareStatus represents the validation status of a share
type ShareStatus int

const (
	// ShareStatusPending indicates a share is awaiting validation
	ShareStatusPending ShareStatus = iota
	// ShareStatusValid indicates a share passed validation
	ShareStatusValid
	// ShareStatusInvalid indicates a share failed validation
	ShareStatusInvalid
	// ShareStatusStale indicates a share was submitted for an old job
	ShareStatusStale
	// ShareStatusDuplicate indicates a share was already submitted
	ShareStatusDuplicate
	// ShareStatusBlockCandidate indicates a share meets the network difficulty
	ShareStatusBlockCandidate
)

// NewShareProcessor creates a new share processor
func NewShareProcessor(cfg *config.Config, logger *log.Logger, kafkaClient *messaging.KafkaClient) *ShareProcessor {
	validator := validation.NewShareValidator(
		cfg.MinDifficulty,
		cfg.MaxDifficulty,
		30*time.Second, // max time skew
	)

	return &ShareProcessor{
		cfg:         cfg,
		logger:      logger.WithComponent("shareproc"),
		validator:   validator,
		kafkaClient: kafkaClient,
		jobCache:    make(map[string]*validation.MiningJob),
		shareQueue:  make(chan *ShareSubmission, cfg.WorkerPoolSize*10),
		done:        make(chan struct{}),
	}
}

// Start starts the share processor
func (sp *ShareProcessor) Start(ctx context.Context) error {
	sp.logger.Info("share processor starting")

	// Start worker pool
	for i := 0; i < sp.cfg.WorkerPoolSize; i++ {
		go sp.worker(ctx, i)
	}

	// Start Kafka consumer for share submissions
	go sp.startShareConsumer(ctx)

	// Wait for shutdown
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sp.done:
		return nil
	}
}

// Shutdown gracefully shuts down the share processor
func (sp *ShareProcessor) Shutdown(_ context.Context) error {
	sp.logger.Info("shutting down share processor")
	close(sp.done)
	return nil
}

// worker processes shares from the queue
func (sp *ShareProcessor) worker(ctx context.Context, workerID int) {
	logger := sp.logger.WithFields("worker_id", workerID)
	logger.Info("worker started")

	defer logger.Info("worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-sp.done:
			return
		case submission := <-sp.shareQueue:
			result := sp.processShare(ctx, submission)

			// Send result back
			select {
			case submission.ResultChan <- result:
			case <-ctx.Done():
				return
			case <-sp.done:
				return
			}
		}
	}
}

// processShare processes a single share submission
func (sp *ShareProcessor) processShare(ctx context.Context, submission *ShareSubmission) *ShareResult {
	share := submission.Share
	logger := sp.logger.WithShare(share.ID, share.Difficulty)

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		logger.LogDuration("share_processing", duration.Nanoseconds())
	}()

	// Look up job template
	jobTemplate, exists := sp.jobCache[share.JobID]
	if !exists {
		logger.Error("job not found in cache", "job_id", share.JobID)
		return &ShareResult{
			ShareID:      share.ID,
			Status:       ShareStatusStale,
			ErrorMessage: "Job not found",
		}
	}

	// Validate the share
	if err := sp.validator.ValidateShare(share, jobTemplate); err != nil {
		logger.WithError(err).Info("share validation failed")
		return &ShareResult{
			ShareID:      share.ID,
			Status:       ShareStatusInvalid,
			ErrorMessage: err.Error(),
		}
	}

	// Check if it's a block candidate
	isBlockCandidate := sp.validator.IsBlockCandidate(share)

	status := ShareStatusValid
	if isBlockCandidate {
		status = ShareStatusBlockCandidate
		logger.LogBlockFound(
			"", // block hash would be calculated
			share.BlockHeight,
			share.MinerAddress,
			share.WorkerName,
			share.Difficulty,
		)
	}

	logger.LogShareSubmission(
		share.MinerAddress,
		share.WorkerName,
		share.JobID,
		share.Difficulty,
		sp.statusToString(status),
	)

	result := &ShareResult{
		ShareID:          share.ID,
		Status:           status,
		IsBlockCandidate: isBlockCandidate,
	}

	// Publish share validation result
	validationResult := &messaging.ShareValidationResult{
		ShareID:          share.ID,
		JobID:            share.JobID,
		Status:           sp.statusToString(status),
		IsBlockCandidate: isBlockCandidate,
		ProcessedAt:      time.Now(),
		ProcessingTimeMs: float64(time.Since(startTime).Nanoseconds()) / 1e6,
	}

	if resultData, err := json.Marshal(validationResult); err == nil {
		if err := sp.kafkaClient.PublishJSON(ctx, messaging.TopicShareResults, share.ID, resultData); err != nil {
			sp.logger.Error("failed to publish share result", "error", err, "share_id", share.ID)
		}
	}

	// If it's a block candidate, publish to block candidates topic
	if isBlockCandidate {
		blockCandidate := &messaging.BlockCandidateMessage{
			ShareID:      share.ID,
			JobID:        share.JobID,
			BlockHeight:  share.BlockHeight,
			Difficulty:   share.Difficulty,
			MinerAddress: share.MinerAddress,
			WorkerName:   share.WorkerName,
			ExtraNonce2:  share.ExtraNonce2,
			Ntime:        share.Ntime,
			Nonce:        share.Nonce,
			FoundAt:      time.Now(),
			// TODO: Calculate actual block hash and hex
		}

		if candidateData, err := json.Marshal(blockCandidate); err == nil {
			if err := sp.kafkaClient.PublishJSON(ctx, messaging.TopicBlockCandidates, share.ID, candidateData); err != nil {
				sp.logger.Error("failed to publish block candidate", "error", err, "share_id", share.ID)
			}
		}
	}

	return result
}

// statusToString converts ShareStatus to string
func (sp *ShareProcessor) statusToString(status ShareStatus) string {
	switch status {
	case ShareStatusPending:
		return "pending"
	case ShareStatusValid:
		return "valid"
	case ShareStatusInvalid:
		return "invalid"
	case ShareStatusStale:
		return "stale"
	case ShareStatusDuplicate:
		return "duplicate"
	case ShareStatusBlockCandidate:
		return "block_candidate"
	default:
		return "unknown"
	}
}

// AddJob adds a job to the cache for validation
func (sp *ShareProcessor) AddJob(job *validation.MiningJob) {
	sp.jobCache[job.JobID] = job
	sp.logger.Info("job added to cache", "job_id", job.JobID, "block_height", job.BlockHeight)
}

// RemoveJob removes a job from the cache
func (sp *ShareProcessor) RemoveJob(jobID string) {
	delete(sp.jobCache, jobID)
	sp.logger.Info("job removed from cache", "job_id", jobID)
}

// ProcessShare processes a share submission (public interface)
func (sp *ShareProcessor) ProcessShare(ctx context.Context, share *validation.Share, sessionID, remoteAddr string) (*ShareResult, error) {
	resultChan := make(chan *ShareResult, 1)

	submission := &ShareSubmission{
		Share:      share,
		SessionID:  sessionID,
		RemoteAddr: remoteAddr,
		ResultChan: resultChan,
	}

	// Submit to queue
	select {
	case sp.shareQueue <- submission:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-sp.done:
		return nil, fmt.Errorf("processor shutting down")
	default:
		return nil, fmt.Errorf("share queue full")
	}

	// Wait for result
	select {
	case result := <-resultChan:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-sp.done:
		return nil, fmt.Errorf("processor shutting down")
	}
}

// startShareConsumer starts consuming share submissions from Kafka
func (sp *ShareProcessor) startShareConsumer(ctx context.Context) {
	reader := sp.kafkaClient.GetConsumer(messaging.TopicShares, "shareproc-group")
	defer func() {
		if err := reader.Close(); err != nil {
			sp.logger.Error("failed to close Kafka reader", "error", err)
		}
	}()

	sp.logger.Info("started share consumer", "topic", messaging.TopicShares)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sp.done:
			return
		default:
		}

		// Read message from Kafka
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			sp.logger.WithError(err).Error("failed to read message from Kafka")
			continue
		}

		// Parse share message
		var shareMsg messaging.ShareMessage
		if err := json.Unmarshal(msg.Value, &shareMsg); err != nil {
			sp.logger.WithError(err).Error("failed to unmarshal share message")
			continue
		}

		// Convert to validation.Share
		share := &validation.Share{
			ID:           shareMsg.ShareID,
			JobID:        shareMsg.JobID,
			MinerAddress: shareMsg.MinerAddress,
			WorkerName:   shareMsg.WorkerName,
			ExtraNonce2:  shareMsg.ExtraNonce2,
			Ntime:        shareMsg.Ntime,
			Nonce:        shareMsg.Nonce,
			Difficulty:   shareMsg.Difficulty,
			BlockHeight:  shareMsg.BlockHeight,
			SubmittedAt:  shareMsg.SubmittedAt,
		}

		// Create submission for processing
		resultChan := make(chan *ShareResult, 1)
		submission := &ShareSubmission{
			Share:      share,
			SessionID:  shareMsg.SessionID,
			RemoteAddr: shareMsg.RemoteAddr,
			ResultChan: resultChan,
		}

		// Submit to processing queue
		select {
		case sp.shareQueue <- submission:
			// Wait for result (non-blocking for Kafka consumer)
			go func() {
				select {
				case <-resultChan:
					// Result processed and published via Kafka
				case <-time.After(30 * time.Second):
					sp.logger.Error("share processing timeout", "share_id", share.ID)
				}
			}()
		default:
			sp.logger.Error("share queue full, dropping share", "share_id", share.ID)
		}
	}
}
