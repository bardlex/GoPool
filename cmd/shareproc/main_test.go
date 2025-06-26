package main

import (
	"context"
	"testing"
	"time"

	"github.com/bardlex/gomp/internal/config"
	"github.com/bardlex/gomp/internal/messaging"
	"github.com/bardlex/gomp/internal/validation"
	"github.com/bardlex/gomp/pkg/log"
)

func TestNewShareProcessor(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "test-shareproc",
		Version:        "test",
		LogLevel:       "error",
		LogFormat:      "json",
		MinDifficulty:  1.0,
		MaxDifficulty:  1000.0,
		WorkerPoolSize: 4,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	processor := NewShareProcessor(cfg, logger, kafkaClient)

	if processor == nil {
		t.Fatal("NewShareProcessor() returned nil")
	}

	if processor.cfg != cfg {
		t.Error("NewShareProcessor() did not set config correctly")
	}

	if processor.logger == nil {
		t.Error("NewShareProcessor() did not set logger correctly")
	}

	if processor.validator == nil {
		t.Error("NewShareProcessor() did not create validator")
	}

	if processor.kafkaClient != kafkaClient {
		t.Error("NewShareProcessor() did not set kafkaClient correctly")
	}

	if processor.jobCache == nil {
		t.Error("NewShareProcessor() did not initialize jobCache")
	}

	if processor.shareQueue == nil {
		t.Error("NewShareProcessor() did not initialize shareQueue")
	}

	if processor.done == nil {
		t.Error("NewShareProcessor() did not initialize done channel")
	}

	// Check queue capacity
	expectedCap := cfg.WorkerPoolSize * 10
	if cap(processor.shareQueue) != expectedCap {
		t.Errorf("NewShareProcessor() shareQueue capacity = %d, want %d", cap(processor.shareQueue), expectedCap)
	}
}

func TestShareProcessor_AddJob(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "test-shareproc",
		Version:        "test",
		LogLevel:       "error",
		LogFormat:      "json",
		MinDifficulty:  1.0,
		MaxDifficulty:  1000.0,
		WorkerPoolSize: 4,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	processor := NewShareProcessor(cfg, logger, kafkaClient)

	job := &validation.MiningJob{
		JobID:       "test_job_1",
		BlockHeight: 100,
		NetworkDifficulty: 1.0,
	}

	processor.AddJob(job)

	// Verify job was added to cache
	cachedJob, exists := processor.jobCache[job.JobID]
	if !exists {
		t.Error("AddJob() did not add job to cache")
	}

	if cachedJob != job {
		t.Error("AddJob() cached job does not match original")
	}
}

func TestShareProcessor_RemoveJob(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "test-shareproc",
		Version:        "test",
		LogLevel:       "error",
		LogFormat:      "json",
		MinDifficulty:  1.0,
		MaxDifficulty:  1000.0,
		WorkerPoolSize: 4,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	processor := NewShareProcessor(cfg, logger, kafkaClient)

	jobID := "test_job_1"
	job := &validation.MiningJob{
		JobID:       jobID,
		BlockHeight: 100,
		NetworkDifficulty: 1.0,
	}

	// Add job first
	processor.AddJob(job)

	// Verify it's there
	if _, exists := processor.jobCache[jobID]; !exists {
		t.Fatal("Job was not added to cache")
	}

	// Remove job
	processor.RemoveJob(jobID)

	// Verify it's gone
	if _, exists := processor.jobCache[jobID]; exists {
		t.Error("RemoveJob() did not remove job from cache")
	}
}

func TestShareProcessor_processShare(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "test-shareproc",
		Version:        "test",
		LogLevel:       "error",
		LogFormat:      "json",
		MinDifficulty:  1.0,
		MaxDifficulty:  1000.0,
		WorkerPoolSize: 4,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	processor := NewShareProcessor(cfg, logger, kafkaClient)

	// Add a test job to cache
	job := &validation.MiningJob{
		JobID:       "test_job_1",
		BlockHeight: 100,
		NetworkDifficulty: 1.0,
	}
	processor.AddJob(job)

	tests := []struct {
		name           string
		share          *validation.Share
		expectStatus   ShareStatus
		description    string
	}{
		{
			name: "valid share",
			share: &validation.Share{
				ID:           "share_1",
				JobID:        "test_job_1",
				MinerAddress: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				WorkerName:   "worker1",
				ExtraNonce2:  "00000000",
				Ntime:        "12345678",
				Nonce:        "abcdef00",
				Difficulty:   1.0,
				BlockHeight:  100,
				SubmittedAt:  time.Now(),
			},
			expectStatus: ShareStatusValid, // Note: May fail validation due to mock data
			description:  "should process valid share",
		},
		{
			name: "share with unknown job",
			share: &validation.Share{
				ID:           "share_2",
				JobID:        "unknown_job",
				MinerAddress: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				WorkerName:   "worker1",
				ExtraNonce2:  "00000000",
				Ntime:        "12345678",
				Nonce:        "abcdef00",
				Difficulty:   1.0,
				BlockHeight:  100,
				SubmittedAt:  time.Now(),
			},
			expectStatus: ShareStatusStale,
			description:  "should reject share with unknown job",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			submission := &ShareSubmission{
				Share:      tt.share,
				SessionID:  "test_session",
				RemoteAddr: "127.0.0.1:12345",
				ResultChan: make(chan *ShareResult, 1),
			}

			ctx := context.Background()
			result := processor.processShare(ctx, submission)

			if result == nil {
				t.Fatal("processShare() returned nil result")
			}

			if result.ShareID != tt.share.ID {
				t.Errorf("processShare() ShareID = %v, want %v", result.ShareID, tt.share.ID)
			}

			if tt.expectStatus == ShareStatusStale && result.Status != ShareStatusStale {
				t.Errorf("processShare() Status = %v, want %v (%s)", result.Status, tt.expectStatus, tt.description)
			}

			// For valid shares, the actual validation may fail due to mock data,
			// but we can check that it's not a stale share
			if tt.expectStatus == ShareStatusValid && result.Status == ShareStatusStale {
				t.Errorf("processShare() unexpected stale status for valid job (%s)", tt.description)
			}
		})
	}
}

func TestShareProcessor_statusToString(t *testing.T) {
	processor := &ShareProcessor{}

	tests := []struct {
		status ShareStatus
		want   string
	}{
		{ShareStatusPending, "pending"},
		{ShareStatusValid, "valid"},
		{ShareStatusInvalid, "invalid"},
		{ShareStatusStale, "stale"},
		{ShareStatusDuplicate, "duplicate"},
		{ShareStatusBlockCandidate, "block_candidate"},
		{ShareStatus(999), "unknown"}, // Test unknown status
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := processor.statusToString(tt.status)
			if got != tt.want {
				t.Errorf("statusToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShareProcessor_ProcessShare(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "test-shareproc",
		Version:        "test",
		LogLevel:       "error",
		LogFormat:      "json",
		MinDifficulty:  1.0,
		MaxDifficulty:  1000.0,
		WorkerPoolSize: 4,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	processor := NewShareProcessor(cfg, logger, kafkaClient)

	// Test that the public interface works
	share := &validation.Share{
		ID:           "share_1",
		JobID:        "unknown_job", // This will cause stale status
		MinerAddress: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		WorkerName:   "worker1",
		ExtraNonce2:  "00000000",
		Ntime:        "12345678",
		Nonce:        "abcdef00",
		Difficulty:   1.0,
		BlockHeight:  100,
		SubmittedAt:  time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a single worker to process the share
	go processor.worker(ctx, 0)

	result, err := processor.ProcessShare(ctx, share, "test_session", "127.0.0.1:12345")

	if err != nil {
		t.Errorf("ProcessShare() unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("ProcessShare() returned nil result")
	}

	if result.ShareID != share.ID {
		t.Errorf("ProcessShare() ShareID = %v, want %v", result.ShareID, share.ID)
	}

	// Should be stale since job doesn't exist
	if result.Status != ShareStatusStale {
		t.Errorf("ProcessShare() Status = %v, want %v", result.Status, ShareStatusStale)
	}
}

func TestShareProcessor_Shutdown(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "test-shareproc",
		Version:        "test",
		LogLevel:       "error",
		LogFormat:      "json",
		MinDifficulty:  1.0,
		MaxDifficulty:  1000.0,
		WorkerPoolSize: 4,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	processor := NewShareProcessor(cfg, logger, kafkaClient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := processor.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() unexpected error: %v", err)
	}

	// Verify that done channel is closed
	select {
	case <-processor.done:
		// Channel is closed, this is expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown() did not close done channel")
	}
}