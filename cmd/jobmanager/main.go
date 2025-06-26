// Package main implements jobmanager service for GOMP mining pool.
// This service creates mining jobs from Bitcoin Core block templates and distributes them via Kafka.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcjson"
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
	logger.Info("starting jobmanager",
		"version", cfg.Version,
		"bitcoin_host", cfg.BitcoinRPCHost,
		"bitcoin_port", cfg.BitcoinRPCPort,
	)

	// Create Bitcoin client
	// Create Bitcoin RPC client with both basic and mining operations
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
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	if err := bitcoinClient.Ping(pingCtx); err != nil {
		logger.WithError(err).Error("failed to connect to Bitcoin Core")
		os.Exit(1)
	}
	logger.Info("connected to Bitcoin Core")

	// Create Kafka client
	kafkaClient := messaging.NewKafkaClient(
		cfg.KafkaBrokers,
		logger.Logger, // Convert to slog.Logger
	)

	// Create the job manager
	jobManager := NewJobManager(cfg, logger, bitcoinClient, kafkaClient)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the job manager
	go func() {
		if err := jobManager.Start(ctx); err != nil {
			logger.WithError(err).Error("job manager failed")
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("shutdown signal received")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := jobManager.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("shutdown failed")
		os.Exit(1)
	}

	logger.Info("jobmanager stopped")
}

// JobManager manages mining job creation and distribution
type JobManager struct {
	cfg           *config.Config
	logger        *log.Logger
	bitcoinClient bitcoin.RPCInterface
	kafkaClient   *messaging.KafkaClient

	// Current job state
	currentTemplate *btcjson.GetBlockTemplateResult
	currentJobID    string
	jobCounter      int64

	// Channels
	done chan struct{}
}

// NewJobManager creates a new job manager
func NewJobManager(cfg *config.Config, logger *log.Logger, bitcoinClient bitcoin.RPCInterface, kafkaClient *messaging.KafkaClient) *JobManager {
	return &JobManager{
		cfg:           cfg,
		logger:        logger.WithComponent("jobmanager"),
		bitcoinClient: bitcoinClient,
		kafkaClient:   kafkaClient,
		done:          make(chan struct{}),
	}
}

// Start starts the job manager
func (jm *JobManager) Start(ctx context.Context) error {
	jm.logger.Info("job manager starting")

	// Start the main loop
	ticker := time.NewTicker(5 * time.Second) // Check for new blocks every 5 seconds
	defer ticker.Stop()

	// Create initial job
	if err := jm.createNewJob(ctx); err != nil {
		jm.logger.WithError(err).Error("failed to create initial job")
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-jm.done:
			return nil
		case <-ticker.C:
			if err := jm.checkForNewBlock(ctx); err != nil {
				jm.logger.WithError(err).Error("failed to check for new block")
			}
		}
	}
}

// Shutdown gracefully shuts down the job manager
func (jm *JobManager) Shutdown(_ context.Context) error {
	jm.logger.Info("shutting down job manager")
	close(jm.done)
	return nil
}

// checkForNewBlock checks if there's a new block and creates a new job if needed
func (jm *JobManager) checkForNewBlock(ctx context.Context) error {
	// Get current best block hash with timeout
	hashCtx, hashCancel := context.WithTimeout(ctx, 5*time.Second)
	defer hashCancel()
	bestHash, err := jm.bitcoinClient.GetBestBlockHash(hashCtx)
	if err != nil {
		return fmt.Errorf("failed to get best block hash: %w", err)
	}

	// Check if we need a new template
	needNewTemplate := false

	if jm.currentTemplate == nil {
		needNewTemplate = true
	} else if jm.currentTemplate.PreviousHash != bestHash {
		needNewTemplate = true
		jm.logger.Info("new block detected",
			"old_prev_hash", jm.currentTemplate.PreviousHash,
			"new_prev_hash", bestHash,
		)
	}

	if needNewTemplate {
		return jm.createNewJob(ctx)
	}

	return nil
}

// createNewJob creates a new mining job
func (jm *JobManager) createNewJob(ctx context.Context) error {
	// Get block template from Bitcoin Core with timeout
	templateCtx, templateCancel := context.WithTimeout(ctx, 10*time.Second)
	defer templateCancel()
	template, err := jm.bitcoinClient.GetBlockTemplate(templateCtx)
	if err != nil {
		return fmt.Errorf("failed to get block template: %w", err)
	}

	// Generate job ID
	jm.jobCounter++
	jobID := fmt.Sprintf("job_%d", jm.jobCounter)

	// Store current template
	jm.currentTemplate = template
	jm.currentJobID = jobID

	// Create mining job
	job := jm.createMiningJob(template, jobID)

	jm.logger.LogJobDistribution(
		jobID,
		template.Height,
		true, // clean_jobs = true for new blocks
		0,    // TODO: track connected miners
	)

	// Publish job to Kafka for stratumd services
	jobMessage := &messaging.JobMessage{
		JobID:        jobID,
		PrevHash:     template.PreviousHash,
		Coinb1:       job.Coinb1,
		Coinb2:       job.Coinb2,
		MerkleBranch: job.MerkleBranch,
		Version:      job.Version,
		NBits:        job.NBits,
		NTime:        job.NTime,
		CleanJobs:    job.CleanJobs,
		BlockHeight:  template.Height,
		Difficulty:   1.0, // TODO: Calculate actual difficulty from target
		CreatedAt:    time.Now(),
	}

	// Serialize and publish to Kafka
	jobData, err := json.Marshal(jobMessage)
	if err != nil {
		jm.logger.WithError(err).Error("failed to marshal job message")
		return err
	}

	if err := jm.kafkaClient.PublishJSON(ctx, messaging.TopicJobs, jobID, jobData); err != nil {
		jm.logger.WithError(err).Error("failed to publish job to Kafka")
		return err
	}

	jm.logger.Info("new job created and published",
		"job_id", jobID,
		"block_height", template.Height,
		"prev_hash", template.PreviousHash,
		"difficulty", template.Target,
		"coinbase_value", *template.CoinbaseValue,
	)

	return nil
}

// createMiningJob converts a block template to a mining job
func (jm *JobManager) createMiningJob(template *btcjson.GetBlockTemplateResult, jobID string) *MiningJob {
	// Create coinbase transaction
	coinb1, coinb2 := jm.createCoinbaseTransaction(template)

	// Calculate merkle branch
	merkleBranch := jm.calculateMerkleBranch(template.Transactions)

	return &MiningJob{
		JobID:        jobID,
		PrevHash:     template.PreviousHash,
		Coinb1:       coinb1,
		Coinb2:       coinb2,
		MerkleBranch: merkleBranch,
		Version:      fmt.Sprintf("%08x", template.Version),
		NBits:        template.Bits,
		NTime:        fmt.Sprintf("%08x", template.CurTime),
		CleanJobs:    true,
		BlockHeight:  template.Height,
	}
}

// MiningJob represents a mining job (simplified version)
type MiningJob struct {
	JobID        string
	PrevHash     string
	Coinb1       string
	Coinb2       string
	MerkleBranch []string
	Version      string
	NBits        string
	NTime        string
	CleanJobs    bool
	BlockHeight  int64
}

// createCoinbaseTransaction creates the coinbase transaction parts using mining client
func (jm *JobManager) createCoinbaseTransaction(template *btcjson.GetBlockTemplateResult) (string, string) {
	// Use the mining client to create proper coinbase transaction (doesn't need context for crypto operations)
	ctx := context.Background()
	_, coinb1, coinb2, err := jm.bitcoinClient.CreateCoinbaseTransaction(
		ctx,
		template.Height,
		*template.CoinbaseValue,
		"", // ExtraNonce1 will be filled by stratum
		"", // Use default pool address for now
	)
	
	if err != nil {
		jm.logger.WithError(err).Error("failed to create coinbase transaction, using fallback")
		// Fallback to simple implementation
		coinb1 = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff"
		coinb2 = "ffffffff01" + fmt.Sprintf("%016x", *template.CoinbaseValue) + "1976a914" + "0000000000000000000000000000000000000000" + "88ac00000000"
	}

	return coinb1, coinb2
}

// calculateMerkleBranch calculates the merkle branch for the coinbase transaction
func (jm *JobManager) calculateMerkleBranch(transactions []btcjson.GetBlockTemplateResultTx) []string {
	// Build transaction hash list (coinbase will be first)
	txHashes := make([]string, len(transactions)+1)
	
	// Coinbase hash will be calculated later, use placeholder for now
	txHashes[0] = "0000000000000000000000000000000000000000000000000000000000000000"
	
	// Add template transaction hashes
	for i, tx := range transactions {
		txHashes[i+1] = tx.Hash
	}
	
	// Use btcd wrapper to calculate merkle branch (doesn't need context for crypto operations)
	ctx := context.Background()
	branch, err := jm.bitcoinClient.GetMerkleBranch(ctx, txHashes)
	if err != nil {
		jm.logger.WithError(err).Error("failed to calculate merkle branch")
		return []string{} // Return empty branch on error
	}
	
	return branch
}


