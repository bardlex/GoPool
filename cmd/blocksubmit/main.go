// Package main implements blocksubmit service for GOMP mining pool.
// This service handles the ultra-low latency submission of solved blocks to Bitcoin Core.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bardlex/gomp/internal/bitcoin"
	"github.com/bardlex/gomp/internal/config"
	"github.com/bardlex/gomp/internal/messaging"
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
	logger.Info("starting blocksubmit",
		"version", cfg.Version,
		"bitcoin_host", cfg.BitcoinRPCHost,
		"bitcoin_port", cfg.BitcoinRPCPort,
	)

	// Create Bitcoin client
	bitcoinClient, err := bitcoin.NewRPCClient(
		cfg.BitcoinRPCHost,
		cfg.BitcoinRPCPort,
		cfg.BitcoinRPCUser,
		cfg.BitcoinRPCPassword,
	)
	if err != nil {
		logger.WithError(err).Error("failed to create Bitcoin RPC client")
		os.Exit(1)
	}
	defer bitcoinClient.Close()

	// Test Bitcoin connection with context
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := bitcoinClient.Ping(pingCtx); err != nil {
		logger.WithError(err).Error("failed to connect to Bitcoin Core")
		os.Exit(1)
	}
	logger.Info("connected to Bitcoin Core")

	// Create Kafka client
	kafkaClient := messaging.NewKafkaClient(
		cfg.KafkaBrokers,
		logger.Logger,
	)

	// Create the block submitter
	submitter := NewBlockSubmitter(cfg, logger, bitcoinClient, kafkaClient)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the submitter
	go func() {
		if err := submitter.Start(ctx); err != nil {
			logger.WithError(err).Error("block submitter failed")
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("shutdown signal received")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := submitter.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("shutdown failed")
		os.Exit(1)
	}

	logger.Info("blocksubmit stopped")
}

// BlockSubmitter handles submission of solved blocks to Bitcoin Core
type BlockSubmitter struct {
	cfg           *config.Config
	logger        *log.Logger
	bitcoinClient bitcoin.RPCInterface
	kafkaClient   *messaging.KafkaClient

	// Block queue for submission
	blockQueue chan *SolvedBlock
	done       chan struct{}
}

// SolvedBlock represents a solved block ready for submission
type SolvedBlock struct {
	BlockHash    string
	BlockHex     string
	BlockHeight  int64
	Difficulty   float64
	MinerAddress string
	WorkerName   string
	FoundAt      time.Time
}

// NewBlockSubmitter creates a new block submitter
func NewBlockSubmitter(cfg *config.Config, logger *log.Logger, bitcoinClient bitcoin.RPCInterface, kafkaClient *messaging.KafkaClient) *BlockSubmitter {
	return &BlockSubmitter{
		cfg:           cfg,
		logger:        logger.WithComponent("blocksubmit"),
		bitcoinClient: bitcoinClient,
		kafkaClient:   kafkaClient,
		blockQueue:    make(chan *SolvedBlock, 100), // Buffer for blocks
		done:          make(chan struct{}),
	}
}

// Start starts the block submitter
func (bs *BlockSubmitter) Start(ctx context.Context) error {
	bs.logger.Info("block submitter starting")

	// Start the submission worker
	go bs.submissionWorker(ctx)

	// Start Kafka consumer for block candidates
	go bs.startBlockCandidateConsumer(ctx)

	// Wait for shutdown
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-bs.done:
		return nil
	}
}

// Shutdown gracefully shuts down the block submitter
func (bs *BlockSubmitter) Shutdown(_ context.Context) error {
	bs.logger.Info("shutting down block submitter")
	close(bs.done)
	return nil
}

// submissionWorker processes blocks from the queue and submits them
func (bs *BlockSubmitter) submissionWorker(ctx context.Context) {
	bs.logger.Info("submission worker started")
	defer bs.logger.Info("submission worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-bs.done:
			return
		case block := <-bs.blockQueue:
			bs.submitBlock(ctx, block)
		}
	}
}

// submitBlock submits a solved block to Bitcoin Core
func (bs *BlockSubmitter) submitBlock(ctx context.Context, block *SolvedBlock) {
	logger := bs.logger.WithFields(
		"block_hash", block.BlockHash,
		"block_height", block.BlockHeight,
		"miner_address", block.MinerAddress,
		"worker_name", block.WorkerName,
	)

	startTime := time.Now()

	logger.Info("submitting block to Bitcoin Core")

	// Submit the block
	err := bs.bitcoinClient.SubmitBlock(ctx, block.BlockHex)

	submissionDuration := time.Since(startTime)
	logger.LogDuration("block_submission", submissionDuration.Nanoseconds())

	if err != nil {
		logger.WithError(err).Error("failed to submit block")

		// TODO: Handle different types of submission errors
		// - duplicate: block already submitted
		// - inconclusive: submission status unclear
		// - rejected: block rejected by network

		return
	}

	logger.Info("block submitted successfully",
		"submission_duration_ms", float64(submissionDuration.Nanoseconds())/1e6,
	)

	// Log the successful block find
	logger.LogBlockFound(
		block.BlockHash,
		block.BlockHeight,
		block.MinerAddress,
		block.WorkerName,
		block.Difficulty,
	)

	// Publish block submission result to Kafka
	status := "accepted"
	if err != nil {
		status = "rejected"
	}

	result := &messaging.BlockSubmissionResult{
		ShareID:        block.BlockHash, // TODO: Use actual share ID
		BlockHash:      block.BlockHash,
		BlockHeight:    block.BlockHeight,
		Status:         status,
		SubmissionTime: time.Now(),
		LatencyMs:      float64(submissionDuration.Nanoseconds()) / 1e6,
	}

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	if resultData, err := json.Marshal(result); err == nil {
		if err := bs.kafkaClient.PublishJSON(context.Background(), messaging.TopicBlockResults, block.BlockHash, resultData); err != nil {
			bs.logger.Error("failed to publish block result", "error", err, "block_hash", block.BlockHash)
		}
	}
}

// SubmitBlock adds a block to the submission queue (public interface)
func (bs *BlockSubmitter) SubmitBlock(block *SolvedBlock) error {
	select {
	case bs.blockQueue <- block:
		return nil
	case <-bs.done:
		return fmt.Errorf("submitter shutting down")
	default:
		return fmt.Errorf("block queue full")
	}
}

// GetQueueLength returns the current queue length
func (bs *BlockSubmitter) GetQueueLength() int {
	return len(bs.blockQueue)
}

// GetStats returns submission statistics
func (bs *BlockSubmitter) GetStats() *SubmissionStats {
	// TODO: Implement proper statistics tracking
	return &SubmissionStats{
		QueueLength: bs.GetQueueLength(),
	}
}

// SubmissionStats represents block submission statistics
type SubmissionStats struct {
	QueueLength      int
	TotalSubmitted   int64
	TotalAccepted    int64
	TotalRejected    int64
	AverageLatencyMs float64
	LastSubmissionAt time.Time
}

// startBlockCandidateConsumer starts consuming block candidates from Kafka
func (bs *BlockSubmitter) startBlockCandidateConsumer(ctx context.Context) {
	reader := bs.kafkaClient.GetConsumer(messaging.TopicBlockCandidates, "blocksubmit-group")
	defer func() {
		if err := reader.Close(); err != nil {
			bs.logger.Error("failed to close Kafka reader", "error", err)
		}
	}()

	bs.logger.Info("started block candidate consumer", "topic", messaging.TopicBlockCandidates)

	for {
		select {
		case <-ctx.Done():
			return
		case <-bs.done:
			return
		default:
		}

		// Read message from Kafka
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			bs.logger.WithError(err).Error("failed to read message from Kafka")
			continue
		}

		// Parse block candidate message
		var candidate messaging.BlockCandidateMessage
		if err := json.Unmarshal(msg.Value, &candidate); err != nil {
			bs.logger.WithError(err).Error("failed to unmarshal block candidate message")
			continue
		}

		// Convert to SolvedBlock
		solvedBlock := &SolvedBlock{
			BlockHash:    candidate.BlockHash,
			BlockHex:     candidate.BlockHex,
			BlockHeight:  candidate.BlockHeight,
			Difficulty:   candidate.Difficulty,
			MinerAddress: candidate.MinerAddress,
			WorkerName:   candidate.WorkerName,
			FoundAt:      candidate.FoundAt,
		}

		// Submit to processing queue (non-blocking)
		select {
		case bs.blockQueue <- solvedBlock:
			bs.logger.Info("block candidate queued for submission",
				"block_hash", candidate.BlockHash,
				"block_height", candidate.BlockHeight,
				"miner", candidate.MinerAddress,
			)
		default:
			bs.logger.Error("block queue full, dropping block candidate",
				"block_hash", candidate.BlockHash,
				"block_height", candidate.BlockHeight,
			)
		}
	}
}
