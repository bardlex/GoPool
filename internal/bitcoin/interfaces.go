// Package bitcoin provides interfaces for better testability and dependency injection.
// These interfaces define the contracts for Bitcoin operations, allowing for easy mocking
// and testing of components that depend on Bitcoin functionality.
package bitcoin

import (
	"context"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// RPCInterface defines the contract for Bitcoin Core RPC operations.
// This interface allows for easy mocking and testing of components that depend on Bitcoin RPC.
//
// All methods include context.Context for proper cancellation and timeout handling.
// Error handling follows Go conventions with wrapped errors containing context.
type RPCInterface interface {
	// Core RPC Operations

	// GetBlockTemplate retrieves a block template for mining from Bitcoin Core.
	GetBlockTemplate(ctx context.Context) (*btcjson.GetBlockTemplateResult, error)

	// GetBlockCount returns the current blockchain height.
	GetBlockCount(ctx context.Context) (int64, error)

	// GetBestBlockHash returns the hash of the current best block.
	GetBestBlockHash(ctx context.Context) (string, error)

	// GetBlock retrieves detailed information about a specific block.
	GetBlock(ctx context.Context, hash string) (*btcjson.GetBlockVerboseResult, error)

	// GetNetworkInfo returns Bitcoin network status and configuration.
	GetNetworkInfo(ctx context.Context) (*btcjson.GetNetworkInfoResult, error)

	// GetDifficulty returns the current network difficulty.
	GetDifficulty(ctx context.Context) (float64, error)

	// GetMiningInfo returns mining-related statistics and status.
	GetMiningInfo(ctx context.Context) (*btcjson.GetMiningInfoResult, error)

	// GetBlockchainInfo returns blockchain status and statistics.
	GetBlockchainInfo(ctx context.Context) (*btcjson.GetBlockChainInfoResult, error)

	// Network Operations

	// SubmitBlock submits a solved block to the Bitcoin network.
	SubmitBlock(ctx context.Context, blockHex string) error

	// ValidateAddress checks if a Bitcoin address is valid.
	ValidateAddress(ctx context.Context, address string) (bool, error)

	// Ping tests connectivity to Bitcoin Core.
	Ping(ctx context.Context) error

	// Mining Operations (using crypto package)

	// CreateCoinbaseTransaction creates a BIP 34 compliant coinbase transaction.
	CreateCoinbaseTransaction(ctx context.Context, blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string) (*wire.MsgTx, string, string, error)

	// CalculateMerkleRoot computes the merkle root from transaction hashes.
	CalculateMerkleRoot(ctx context.Context, txHashes []string) (string, error)

	// GetMerkleBranch calculates the merkle branch for coinbase verification.
	GetMerkleBranch(ctx context.Context, txHashes []string) ([]string, error)

	// Connection Management

	// Close gracefully shuts down the RPC client.
	Close()
}

// CryptoInterface defines the contract for Bitcoin cryptographic operations.
// These operations are pure functions that don't require network connectivity.
//
// This interface is useful for testing components that need Bitcoin cryptographic
// functionality without requiring a full RPC client.
type CryptoInterface interface {
	// Transaction Operations

	// CreateCoinbaseTransaction creates a BIP 34 compliant coinbase transaction.
	// Returns the transaction, coinb1 (first part), coinb2 (second part), and any error.
	CreateCoinbaseTransaction(blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string, chainParams *chaincfg.Params) (*wire.MsgTx, string, string, error)

	// Block Operations

	// ReconstructBlock rebuilds a complete Bitcoin block from mining components.
	ReconstructBlock(template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx, extraNonce2, ntime, nonce string) (*wire.MsgBlock, string, error)

	// Validation Operations

	// ValidateShare verifies that a mining share meets the required difficulty.
	ValidateShare(jobID, extraNonce2, ntime, nonce string, difficulty float64, template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx) error

	// IsBlockCandidate determines if a share meets network difficulty (is a valid block).
	IsBlockCandidate(jobID, extraNonce2, ntime, nonce string, template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx) (bool, error)

	// Difficulty and Target Operations

	// DifficultyToTarget converts mining difficulty to a target threshold.
	DifficultyToTarget(difficulty float64) []byte

	// HashMeetsTarget checks if a hash satisfies a difficulty target.
	HashMeetsTarget(hash chainhash.Hash, target []byte) bool
}

// ZMQInterface defines the contract for Bitcoin Core ZMQ notifications.
// This interface allows for mocking ZMQ functionality in tests.
type ZMQInterface interface {
	// Configuration

	// Subscribe adds a topic subscription for ZMQ notifications.
	Subscribe(topic string) error

	// Connect establishes connection to the ZMQ endpoint.
	Connect() error

	// Listening

	// Listen starts the ZMQ listener with a message handler function.
	// The handler function receives topic and data for each message.
	Listen(ctx context.Context, handler func(topic string, data []byte) error) error

	// Connection Management

	// Close gracefully shuts down the ZMQ connection.
	Close() error
}

// BlockNotificationInterface defines the contract for handling Bitcoin block notifications.
// This interface allows for custom block notification handling logic.
type BlockNotificationInterface interface {
	// Handler Configuration

	// SetNewBlockHandler sets the callback for new block notifications.
	SetNewBlockHandler(handler func(blockHash string) error)

	// SetNewTxHandler sets the callback for new transaction notifications.
	SetNewTxHandler(handler func(txHash string) error)

	// Message Processing

	// HandleMessage processes incoming ZMQ messages and routes them to appropriate handlers.
	HandleMessage(topic string, data []byte) error
}

// Compile-time interface compliance checks
var (
	_ RPCInterface               = (*RPCClient)(nil)
	_ CryptoInterface            = (*CryptoService)(nil)
	_ ZMQInterface               = (*ZMQNotifier)(nil)
	_ BlockNotificationInterface = (*BlockNotificationHandler)(nil)
)
