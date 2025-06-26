package main

import (
	"context"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/wire"
	"github.com/bardlex/gomp/internal/bitcoin"
	"github.com/bardlex/gomp/internal/config"
	"github.com/bardlex/gomp/internal/messaging"
	"github.com/bardlex/gomp/pkg/log"
)

func TestNewBlockSubmitter(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-blocksubmit",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	
	// Create mock Bitcoin client (simplified for test)
	bitcoinClient, err := bitcoin.NewRPCClient("localhost", 8332, "test", "test")
	if err != nil {
		// Expected to fail in test environment without Bitcoin Core
		t.Logf("Expected Bitcoin client creation to fail in test: %v", err)
	}
	
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	bs := NewBlockSubmitter(cfg, logger, bitcoinClient, kafkaClient)

	if bs == nil {
		t.Fatal("NewBlockSubmitter() returned nil")
	}

	if bs.cfg != cfg {
		t.Error("NewBlockSubmitter() did not set config correctly")
	}

	if bs.logger == nil {
		t.Error("NewBlockSubmitter() did not set logger correctly")
	}

	if bs.kafkaClient != kafkaClient {
		t.Error("NewBlockSubmitter() did not set kafkaClient correctly")
	}

	if bs.done == nil {
		t.Error("NewBlockSubmitter() did not initialize done channel")
	}
}

func TestBlockSubmitter_Shutdown(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-blocksubmit",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	// Create a minimal submitter for testing shutdown
	bs := &BlockSubmitter{
		cfg:         cfg,
		logger:      logger.WithComponent("blocksubmit"),
		kafkaClient: kafkaClient,
		done:        make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := bs.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() unexpected error: %v", err)
	}

	// Verify that done channel is closed
	select {
	case <-bs.done:
		// Channel is closed, this is expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown() did not close done channel")
	}
}

func TestBlockSubmitter_submitBlock(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-blocksubmit",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)

	// Use a mock Bitcoin client
	mockBitcoin := &testMockBitcoinClient{shouldError: false}

	bs := &BlockSubmitter{
		cfg:           cfg,
		logger:        logger.WithComponent("blocksubmit"),
		bitcoinClient: mockBitcoin,
		kafkaClient:   kafkaClient,
		done:          make(chan struct{}),
	}

	block := &SolvedBlock{
		BlockHash:    "test_hash",
		BlockHex:     "deadbeef",
		BlockHeight:  100,
		Difficulty:   1.0,
		MinerAddress: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		WorkerName:   "test_worker",
		FoundAt:      time.Now(),
	}

	ctx := context.Background()
	bs.submitBlock(ctx, block)

	// submitBlock doesn't return an error, it logs internally
	// Test error case
	mockBitcoin.shouldError = true
	bs.submitBlock(ctx, block)
	// No error to check since submitBlock handles errors internally
}

// testMockBitcoinClient provides a minimal mock for testing block submission
type testMockBitcoinClient struct {
	shouldError bool
}

func (m *testMockBitcoinClient) SubmitBlock(_ context.Context, _ string) error {
	if m.shouldError {
		return &testSubmitError{msg: "mock submission error"}
	}
	return nil
}

func (m *testMockBitcoinClient) Ping(_ context.Context) error {
	return nil
}

func (m *testMockBitcoinClient) Close() {
	// Nothing to do for mock
}

// Implement other required interface methods as no-ops for testing
func (m *testMockBitcoinClient) GetBlockTemplate(_ context.Context) (*btcjson.GetBlockTemplateResult, error) {
	return nil, nil
}

func (m *testMockBitcoinClient) GetBlockCount(_ context.Context) (int64, error) {
	return 0, nil
}

func (m *testMockBitcoinClient) GetBestBlockHash(_ context.Context) (string, error) {
	return "", nil
}

func (m *testMockBitcoinClient) GetBlock(_ context.Context, hash string) (*btcjson.GetBlockVerboseResult, error) {
	return nil, nil
}

func (m *testMockBitcoinClient) GetNetworkInfo(_ context.Context) (*btcjson.GetNetworkInfoResult, error) {
	return nil, nil
}

func (m *testMockBitcoinClient) GetDifficulty(_ context.Context) (float64, error) {
	return 0, nil
}

func (m *testMockBitcoinClient) GetMiningInfo(_ context.Context) (*btcjson.GetMiningInfoResult, error) {
	return nil, nil
}

func (m *testMockBitcoinClient) GetBlockchainInfo(_ context.Context) (*btcjson.GetBlockChainInfoResult, error) {
	return nil, nil
}

func (m *testMockBitcoinClient) ValidateAddress(_ context.Context, address string) (bool, error) {
	return false, nil
}

func (m *testMockBitcoinClient) CreateCoinbaseTransaction(_ context.Context, blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string) (*wire.MsgTx, string, string, error) {
	return nil, "", "", nil
}

func (m *testMockBitcoinClient) CalculateMerkleRoot(_ context.Context, txHashes []string) (string, error) {
	return "", nil
}

func (m *testMockBitcoinClient) GetMerkleBranch(_ context.Context, txHashes []string) ([]string, error) {
	return nil, nil
}

// testSubmitError implements error interface for testing
type testSubmitError struct {
	msg string
}

func (e *testSubmitError) Error() string {
	return e.msg
}

// Compile-time interface check
var _ bitcoin.RPCInterface = (*testMockBitcoinClient)(nil)