package bitcoin

import (
	"context"
	"errors"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// MockRPCClient provides a mock implementation of RPCInterface for testing.
type MockRPCClient struct {
	// Control mock behavior
	ShouldError bool
	ErrorMsg    string

	// Mock data
	BlockTemplate *btcjson.GetBlockTemplateResult
	BlockCount    int64
	BestBlockHash string
}

// NewMockRPCClient creates a new mock RPC client for testing.
func NewMockRPCClient() *MockRPCClient {
	return &MockRPCClient{
		BlockTemplate: &btcjson.GetBlockTemplateResult{
			Version:       1,
			PreviousHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:          "1d00ffff",
			Height:        100,
			Target:        "00000000FFFF0000000000000000000000000000000000000000000000000000",
			CoinbaseValue: func() *int64 { v := int64(5000000000); return &v }(),
			Transactions:  []btcjson.GetBlockTemplateResultTx{},
		},
		BlockCount:    100,
		BestBlockHash: "0000000000000000000000000000000000000000000000000000000000000001",
	}
}

// GetBlockTemplate returns a mock block template.
func (m *MockRPCClient) GetBlockTemplate(_ context.Context) (*btcjson.GetBlockTemplateResult, error) {
	if m.ShouldError {
		return nil, errors.New(m.ErrorMsg)
	}
	return m.BlockTemplate, nil
}

// GetBlockCount returns mock block count.
func (m *MockRPCClient) GetBlockCount(_ context.Context) (int64, error) {
	if m.ShouldError {
		return 0, errors.New(m.ErrorMsg)
	}
	return m.BlockCount, nil
}

// GetBestBlockHash returns mock best block hash.
func (m *MockRPCClient) GetBestBlockHash(_ context.Context) (string, error) {
	if m.ShouldError {
		return "", errors.New(m.ErrorMsg)
	}
	return m.BestBlockHash, nil
}

// GetBlock returns mock block data.
func (m *MockRPCClient) GetBlock(_ context.Context, hash string) (*btcjson.GetBlockVerboseResult, error) {
	if m.ShouldError {
		return nil, errors.New(m.ErrorMsg)
	}
	return &btcjson.GetBlockVerboseResult{Hash: hash}, nil
}

// GetNetworkInfo returns mock network info.
func (m *MockRPCClient) GetNetworkInfo(_ context.Context) (*btcjson.GetNetworkInfoResult, error) {
	if m.ShouldError {
		return nil, errors.New(m.ErrorMsg)
	}
	return &btcjson.GetNetworkInfoResult{Version: 220000}, nil
}

// GetDifficulty returns mock difficulty.
func (m *MockRPCClient) GetDifficulty(_ context.Context) (float64, error) {
	if m.ShouldError {
		return 0, errors.New(m.ErrorMsg)
	}
	return 1.0, nil
}

// GetMiningInfo returns mock mining info.
func (m *MockRPCClient) GetMiningInfo(_ context.Context) (*btcjson.GetMiningInfoResult, error) {
	if m.ShouldError {
		return nil, errors.New(m.ErrorMsg)
	}
	return &btcjson.GetMiningInfoResult{Difficulty: 1.0}, nil
}

// GetBlockchainInfo returns mock blockchain info.
func (m *MockRPCClient) GetBlockchainInfo(_ context.Context) (*btcjson.GetBlockChainInfoResult, error) {
	if m.ShouldError {
		return nil, errors.New(m.ErrorMsg)
	}
	return &btcjson.GetBlockChainInfoResult{Chain: "main"}, nil
}

// SubmitBlock simulates block submission.
func (m *MockRPCClient) SubmitBlock(_ context.Context, _ string) error {
	if m.ShouldError {
		return errors.New(m.ErrorMsg)
	}
	return nil
}

// ValidateAddress simulates address validation.
func (m *MockRPCClient) ValidateAddress(_ context.Context, address string) (bool, error) {
	if m.ShouldError {
		return false, errors.New(m.ErrorMsg)
	}
	return address == "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", nil
}

// Ping simulates connection test.
func (m *MockRPCClient) Ping(_ context.Context) error {
	if m.ShouldError {
		return errors.New(m.ErrorMsg)
	}
	return nil
}

// CreateCoinbaseTransaction delegates to the package function.
func (m *MockRPCClient) CreateCoinbaseTransaction(_ context.Context, blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string) (*wire.MsgTx, string, string, error) {
	if m.ShouldError {
		return nil, "", "", errors.New(m.ErrorMsg)
	}
	return CreateCoinbaseTransaction(blockHeight, coinbaseValue, extraNonce1, poolAddress, &chaincfg.MainNetParams)
}

// CalculateMerkleRoot delegates to the package function.
func (m *MockRPCClient) CalculateMerkleRoot(_ context.Context, txHashes []string) (string, error) {
	if m.ShouldError {
		return "", errors.New(m.ErrorMsg)
	}
	if len(txHashes) == 0 {
		return "", errors.New("no transactions provided")
	}

	// Convert string hashes to chainhash.Hash
	hashes := make([]chainhash.Hash, len(txHashes))
	for i, hashStr := range txHashes {
		hash, err := chainhash.NewHashFromStr(hashStr)
		if err != nil {
			return "", err
		}
		hashes[i] = *hash
	}

	merkleRoot := CalculateMerkleRoot(hashes)
	return merkleRoot.String(), nil
}

// GetMerkleBranch delegates to the package function.
func (m *MockRPCClient) GetMerkleBranch(_ context.Context, txHashes []string) ([]string, error) {
	if m.ShouldError {
		return nil, errors.New(m.ErrorMsg)
	}
	if len(txHashes) <= 1 {
		return []string{}, nil
	}

	// Convert string hashes to chainhash.Hash
	hashes := make([]chainhash.Hash, len(txHashes))
	for i, hashStr := range txHashes {
		hash, err := chainhash.NewHashFromStr(hashStr)
		if err != nil {
			return nil, err
		}
		hashes[i] = *hash
	}

	branch := GetMerkleBranch(hashes, 0)
	branchStrs := make([]string, len(branch))
	for i, hash := range branch {
		branchStrs[i] = hash.String()
	}

	return branchStrs, nil
}

// Close simulates client shutdown.
func (m *MockRPCClient) Close() {
	// Nothing to do for mock
}

// Compile-time interface compliance check
var _ RPCInterface = (*MockRPCClient)(nil)
