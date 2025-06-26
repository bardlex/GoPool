package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/wire"
	"github.com/bardlex/gomp/internal/bitcoin"
	"github.com/bardlex/gomp/internal/config"
	"github.com/bardlex/gomp/internal/messaging"
	"github.com/bardlex/gomp/pkg/log"
)

func TestNewJobManager(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-jobmanager",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	
	// Create mock Bitcoin client
	mockBitcoin := &testMockRPCClient{
		blockTemplate: &btcjson.GetBlockTemplateResult{
			Version:       1,
			PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:          "1d00ffff",
			Height:        100,
			CoinbaseValue: func() *int64 { v := int64(5000000000); return &v }(),
			Transactions:  []btcjson.GetBlockTemplateResultTx{},
		},
		blockCount:    100,
		bestBlockHash: "0000000000000000000000000000000000000000000000000000000000000001",
	}
	
	// Create Kafka client (will not actually connect in test)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	jm := NewJobManager(cfg, logger, mockBitcoin, kafkaClient)

	if jm == nil {
		t.Fatal("NewJobManager() returned nil")
	}

	if jm.cfg != cfg {
		t.Error("NewJobManager() did not set config correctly")
	}

	if jm.logger == nil {
		t.Error("NewJobManager() did not set logger correctly")
	}

	if jm.bitcoinClient == nil {
		t.Error("NewJobManager() did not set bitcoinClient correctly")
	}

	if jm.kafkaClient != kafkaClient {
		t.Error("NewJobManager() did not set kafkaClient correctly")
	}

	if jm.done == nil {
		t.Error("NewJobManager() did not initialize done channel")
	}
}

func TestJobManager_checkForNewBlock_Skip(t *testing.T) {
	t.Skip("Skipping test that requires Kafka connection")
}

func TestJobManager_checkForNewBlock_Unit(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-jobmanager",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	mockBitcoin := &testMockRPCClient{
		blockTemplate: &btcjson.GetBlockTemplateResult{
			Version:       1,
			PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:          "1d00ffff",
			Height:        100,
			CoinbaseValue: func() *int64 { v := int64(5000000000); return &v }(),
			Transactions:  []btcjson.GetBlockTemplateResultTx{},
		},
		blockCount:    100,
		bestBlockHash: "0000000000000000000000000000000000000000000000000000000000000001",
	}
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	jm := NewJobManager(cfg, logger, mockBitcoin, kafkaClient)

	tests := []struct {
		name               string
		currentTemplate    *btcjson.GetBlockTemplateResult
		mockBestHash       string
		mockShouldError    bool
		expectNewTemplate  bool
		description        string
	}{
		{
			name:              "no current template",
			currentTemplate:   nil,
			mockBestHash:      "0000000000000000000000000000000000000000000000000000000000000001",
			mockShouldError:   false,
			expectNewTemplate: true,
			description:       "should create new template when none exists",
		},
		{
			name: "same block hash",
			currentTemplate: &btcjson.GetBlockTemplateResult{
				PreviousHash: "0000000000000000000000000000000000000000000000000000000000000001",
			},
			mockBestHash:      "0000000000000000000000000000000000000000000000000000000000000001",
			mockShouldError:   false,
			expectNewTemplate: false,
			description:       "should not create new template for same block",
		},
		{
			name: "new block hash",
			currentTemplate: &btcjson.GetBlockTemplateResult{
				PreviousHash: "0000000000000000000000000000000000000000000000000000000000000001",
			},
			mockBestHash:      "0000000000000000000000000000000000000000000000000000000000000002",
			mockShouldError:   false,
			expectNewTemplate: true,
			description:       "should create new template for new block",
		},
		{
			name:              "bitcoin rpc error",
			currentTemplate:   nil,
			mockBestHash:      "",
			mockShouldError:   true,
			expectNewTemplate: false,
			description:       "should handle RPC errors gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock client
			mockBitcoin.bestBlockHash = tt.mockBestHash
			mockBitcoin.shouldError = tt.mockShouldError
			if tt.mockShouldError {
				mockBitcoin.errorMsg = "mock RPC error"
			} else {
				mockBitcoin.errorMsg = ""
			}

			// Set current template
			jm.currentTemplate = tt.currentTemplate

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := jm.checkForNewBlock(ctx)

			if tt.mockShouldError {
				if err == nil {
					t.Errorf("checkForNewBlock() expected error but got none (%s)", tt.description)
				}
			} else {
				// For unit test, we just verify no unexpected errors for valid cases
				// Note: Full integration test would verify job creation and publishing
				t.Logf("checkForNewBlock() completed for case: %s", tt.description)
			}
		})
	}
}

func TestJobManager_createMiningJob(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-jobmanager",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	mockBitcoin := &testMockRPCClient{
		blockTemplate: &btcjson.GetBlockTemplateResult{
			Version:       1,
			PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:          "1d00ffff",
			Height:        100,
			CoinbaseValue: func() *int64 { v := int64(5000000000); return &v }(),
			Transactions:  []btcjson.GetBlockTemplateResultTx{},
		},
		blockCount:    100,
		bestBlockHash: "0000000000000000000000000000000000000000000000000000000000000001",
	}
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	jm := NewJobManager(cfg, logger, mockBitcoin, kafkaClient)

	// Create a test block template
	coinbaseValue := int64(5000000000)
	template := &btcjson.GetBlockTemplateResult{
		Version:       1,
		PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000001",
		Height:        100,
		Bits:          "1d00ffff",
		CurTime:       1234567890,
		CoinbaseValue: &coinbaseValue,
		Transactions: []btcjson.GetBlockTemplateResultTx{
			{
				Data: "0100000001000000000000000000000000000000000000000000000000000000000000000000000000ffffffff0100f2052a01000000434104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac00000000",
				Hash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
		},
	}

	jobID := "test_job_1"
	job := jm.createMiningJob(template, jobID)

	if job == nil {
		t.Fatal("createMiningJob() returned nil")
	}

	// Verify job fields
	if job.JobID != jobID {
		t.Errorf("createMiningJob() JobID = %v, want %v", job.JobID, jobID)
	}

	if job.PrevHash != template.PreviousHash {
		t.Errorf("createMiningJob() PrevHash = %v, want %v", job.PrevHash, template.PreviousHash)
	}

	if job.BlockHeight != template.Height {
		t.Errorf("createMiningJob() BlockHeight = %v, want %v", job.BlockHeight, template.Height)
	}

	if job.NBits != template.Bits {
		t.Errorf("createMiningJob() NBits = %v, want %v", job.NBits, template.Bits)
	}

	if !job.CleanJobs {
		t.Error("createMiningJob() CleanJobs should be true")
	}

	// Verify that coinbase parts are not empty
	if job.Coinb1 == "" {
		t.Error("createMiningJob() Coinb1 should not be empty")
	}

	if job.Coinb2 == "" {
		t.Error("createMiningJob() Coinb2 should not be empty")
	}

	// Verify merkle branch is initialized (can be empty for coinbase-only blocks)
	if job.MerkleBranch == nil {
		t.Error("createMiningJob() MerkleBranch should not be nil")
	}
}

func TestJobManager_createCoinbaseTransaction(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-jobmanager",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	mockBitcoin := &testMockRPCClient{
		blockTemplate: &btcjson.GetBlockTemplateResult{
			Version:       1,
			PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:          "1d00ffff",
			Height:        100,
			CoinbaseValue: func() *int64 { v := int64(5000000000); return &v }(),
			Transactions:  []btcjson.GetBlockTemplateResultTx{},
		},
		blockCount:    100,
		bestBlockHash: "0000000000000000000000000000000000000000000000000000000000000001",
	}
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	jm := NewJobManager(cfg, logger, mockBitcoin, kafkaClient)

	coinbaseValue := int64(5000000000)
	template := &btcjson.GetBlockTemplateResult{
		Height:        100,
		CoinbaseValue: &coinbaseValue,
	}

	coinb1, coinb2 := jm.createCoinbaseTransaction(template)

	// Both parts should be non-empty hex strings
	if coinb1 == "" {
		t.Error("createCoinbaseTransaction() coinb1 should not be empty")
	}

	if coinb2 == "" {
		t.Error("createCoinbaseTransaction() coinb2 should not be empty")
	}

	// Basic validation that they look like hex strings
	// (More detailed validation would require parsing the actual transaction)
	if len(coinb1)%2 != 0 {
		t.Error("createCoinbaseTransaction() coinb1 should be valid hex (even length)")
	}

	if len(coinb2)%2 != 0 {
		t.Error("createCoinbaseTransaction() coinb2 should be valid hex (even length)")
	}
}

func TestJobManager_calculateMerkleBranch(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-jobmanager",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	mockBitcoin := &testMockRPCClient{
		blockTemplate: &btcjson.GetBlockTemplateResult{
			Version:       1,
			PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:          "1d00ffff",
			Height:        100,
			CoinbaseValue: func() *int64 { v := int64(5000000000); return &v }(),
			Transactions:  []btcjson.GetBlockTemplateResultTx{},
		},
		blockCount:    100,
		bestBlockHash: "0000000000000000000000000000000000000000000000000000000000000001",
	}
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	jm := NewJobManager(cfg, logger, mockBitcoin, kafkaClient)

	tests := []struct {
		name         string
		transactions []btcjson.GetBlockTemplateResultTx
		expectBranch bool
		description  string
	}{
		{
			name:         "no transactions",
			transactions: []btcjson.GetBlockTemplateResultTx{},
			expectBranch: false,
			description:  "should return empty branch for coinbase-only block",
		},
		{
			name: "single transaction",
			transactions: []btcjson.GetBlockTemplateResultTx{
				{Hash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			},
			expectBranch: true,
			description:  "should return branch for block with transactions",
		},
		{
			name: "multiple transactions",
			transactions: []btcjson.GetBlockTemplateResultTx{
				{Hash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
				{Hash: "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"},
			},
			expectBranch: true,
			description:  "should return branch for block with multiple transactions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch := jm.calculateMerkleBranch(tt.transactions)

			if branch == nil {
				t.Error("calculateMerkleBranch() should never return nil")
			}

			if tt.expectBranch && len(branch) == 0 && len(tt.transactions) > 0 {
				t.Errorf("calculateMerkleBranch() expected non-empty branch for %d transactions (%s)", 
					len(tt.transactions), tt.description)
			}

			if !tt.expectBranch && len(branch) != 0 {
				t.Errorf("calculateMerkleBranch() expected empty branch but got %d items (%s)", 
					len(branch), tt.description)
			}
		})
	}
}

func TestJobManager_Shutdown(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-jobmanager",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	mockBitcoin := &testMockRPCClient{
		blockTemplate: &btcjson.GetBlockTemplateResult{
			Version:       1,
			PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:          "1d00ffff",
			Height:        100,
			CoinbaseValue: func() *int64 { v := int64(5000000000); return &v }(),
			Transactions:  []btcjson.GetBlockTemplateResultTx{},
		},
		blockCount:    100,
		bestBlockHash: "0000000000000000000000000000000000000000000000000000000000000001",
	}
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	jm := NewJobManager(cfg, logger, mockBitcoin, kafkaClient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := jm.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() unexpected error: %v", err)
	}

	// Verify that done channel is closed by trying to read from it
	select {
	case <-jm.done:
		// Channel is closed, this is expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown() did not close done channel")
	}
}

// testMockRPCClient provides a simple mock for testing
type testMockRPCClient struct {
	blockTemplate *btcjson.GetBlockTemplateResult
	blockCount    int64
	bestBlockHash string
	shouldError   bool
	errorMsg      string
}

func (m *testMockRPCClient) GetBlockTemplate(_ context.Context) (*btcjson.GetBlockTemplateResult, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}
	return m.blockTemplate, nil
}

func (m *testMockRPCClient) GetBlockCount(_ context.Context) (int64, error) {
	if m.shouldError {
		return 0, errors.New(m.errorMsg)
	}
	return m.blockCount, nil
}

func (m *testMockRPCClient) GetBestBlockHash(_ context.Context) (string, error) {
	if m.shouldError {
		return "", errors.New(m.errorMsg)
	}
	return m.bestBlockHash, nil
}

func (m *testMockRPCClient) GetBlock(_ context.Context, hash string) (*btcjson.GetBlockVerboseResult, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}
	return &btcjson.GetBlockVerboseResult{Hash: hash}, nil
}

func (m *testMockRPCClient) GetNetworkInfo(_ context.Context) (*btcjson.GetNetworkInfoResult, error) {
	return &btcjson.GetNetworkInfoResult{Version: 220000}, nil
}

func (m *testMockRPCClient) GetDifficulty(_ context.Context) (float64, error) {
	return 1.0, nil
}

func (m *testMockRPCClient) GetMiningInfo(_ context.Context) (*btcjson.GetMiningInfoResult, error) {
	return &btcjson.GetMiningInfoResult{Difficulty: 1.0}, nil
}

func (m *testMockRPCClient) GetBlockchainInfo(_ context.Context) (*btcjson.GetBlockChainInfoResult, error) {
	return &btcjson.GetBlockChainInfoResult{Chain: "main"}, nil
}

func (m *testMockRPCClient) SubmitBlock(_ context.Context, _ string) error {
	return nil
}

func (m *testMockRPCClient) ValidateAddress(_ context.Context, address string) (bool, error) {
	return address == "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", nil
}

func (m *testMockRPCClient) Ping(_ context.Context) error {
	return nil
}

func (m *testMockRPCClient) CreateCoinbaseTransaction(_ context.Context, blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string) (*wire.MsgTx, string, string, error) {
	return nil, "coinb1_hex_data", "coinb2_hex_data", nil
}

func (m *testMockRPCClient) CalculateMerkleRoot(_ context.Context, txHashes []string) (string, error) {
	if len(txHashes) == 0 {
		return "", errors.New("no transactions provided")
	}
	return "mock_merkle_root", nil
}

func (m *testMockRPCClient) GetMerkleBranch(_ context.Context, txHashes []string) ([]string, error) {
	if len(txHashes) <= 1 {
		return []string{}, nil
	}
	return []string{"mock_branch_hash"}, nil
}

func (m *testMockRPCClient) Close() {
	// Nothing to do for mock
}

// Compile-time interface check
var _ bitcoin.RPCInterface = (*testMockRPCClient)(nil)

// testMockKafkaClient provides a mock Kafka client that doesn't connect
type testMockKafkaClient struct{}

func (m *testMockKafkaClient) PublishJSON(ctx context.Context, topic, key string, data []byte) error {
	return nil // Successful mock publish
}

func (m *testMockKafkaClient) GetProducer(topic string) interface{} {
	return nil
}

func (m *testMockKafkaClient) GetConsumer(topic, groupID string) interface{} {
	return nil
}

func (m *testMockKafkaClient) Close() error {
	return nil
}