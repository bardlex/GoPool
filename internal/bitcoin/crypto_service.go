// Package bitcoin provides a service implementation for Bitcoin cryptographic operations.
// This service implements the CryptoInterface and provides dependency injection capabilities.
package bitcoin

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// CryptoService provides Bitcoin cryptographic operations without network dependencies.
// It implements the CryptoInterface and can be easily mocked for testing.
//
// This service is stateless and thread-safe, making it suitable for concurrent use
// across multiple goroutines in a mining pool environment.
type CryptoService struct {
	// chainParams holds the Bitcoin network parameters (mainnet, testnet, etc.)
	chainParams *chaincfg.Params
}

// NewCryptoService creates a new CryptoService with the specified chain parameters.
//
// Parameters:
//   - chainParams: Bitcoin network parameters (use chaincfg.MainNetParams for mainnet)
//
// Returns:
//   - *CryptoService: A new crypto service instance
func NewCryptoService(chainParams *chaincfg.Params) *CryptoService {
	return &CryptoService{
		chainParams: chainParams,
	}
}

// CreateCoinbaseTransaction creates a BIP 34 compliant coinbase transaction.
// This method delegates to the package-level function for consistency.
//
// Parameters:
//   - blockHeight: The height of the block (required for BIP 34)
//   - coinbaseValue: The total coinbase reward in satoshis
//   - extraNonce1: The first part of the extra nonce for mining
//   - poolAddress: The Bitcoin address to receive mining rewards
//   - chainParams: The Bitcoin network parameters (if nil, uses service default)
//
// Returns:
//   - *wire.MsgTx: The complete coinbase transaction
//   - string: Coinb1 - first part of serialized coinbase
//   - string: Coinb2 - second part of serialized coinbase
//   - error: Any error encountered during transaction creation
func (cs *CryptoService) CreateCoinbaseTransaction(blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string, chainParams *chaincfg.Params) (*wire.MsgTx, string, string, error) {
	// Use provided chainParams or fall back to service default
	params := chainParams
	if params == nil {
		params = cs.chainParams
	}

	return CreateCoinbaseTransaction(blockHeight, coinbaseValue, extraNonce1, poolAddress, params)
}

// ReconstructBlock rebuilds a complete Bitcoin block from mining components.
// This method delegates to the package-level function for consistency.
//
// Parameters:
//   - template: The block template from Bitcoin Core
//   - coinbaseTx: The coinbase transaction with extra nonce filled in
//   - extraNonce2: The second part of the extra nonce from the miner
//   - ntime: The timestamp for the block header (hex encoded, little-endian)
//   - nonce: The nonce value that satisfied the proof-of-work (hex encoded, little-endian)
//
// Returns:
//   - *wire.MsgBlock: The complete reconstructed block
//   - string: The block serialized as hexadecimal
//   - error: Any error encountered during reconstruction
func (cs *CryptoService) ReconstructBlock(template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx, extraNonce2, ntime, nonce string) (*wire.MsgBlock, string, error) {
	return ReconstructBlock(template, coinbaseTx, extraNonce2, ntime, nonce)
}

// ValidateShare verifies that a mining share meets the required difficulty.
// This method delegates to the package-level function for consistency.
//
// Parameters:
//   - jobID: The job identifier for tracking purposes
//   - extraNonce2: The second part of the extra nonce from the miner
//   - ntime: The timestamp used in the block header
//   - nonce: The nonce value found by the miner
//   - difficulty: The target difficulty for this share
//   - template: The block template used for this job
//   - coinbaseTx: The coinbase transaction for this share
//
// Returns:
//   - error: Non-nil if the share does not meet the difficulty target
func (cs *CryptoService) ValidateShare(jobID, extraNonce2, ntime, nonce string, difficulty float64, template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx) error {
	return ValidateShare(jobID, extraNonce2, ntime, nonce, difficulty, template, coinbaseTx)
}

// IsBlockCandidate determines if a share meets network difficulty (is a valid block).
// This method delegates to the package-level function for consistency.
//
// Parameters:
//   - jobID: The job identifier for tracking purposes
//   - extraNonce2: The second part of the extra nonce from the miner
//   - ntime: The timestamp used in the block header
//   - nonce: The nonce value found by the miner
//   - template: The block template containing network target
//   - coinbaseTx: The coinbase transaction for this share
//
// Returns:
//   - bool: True if this share solves a block
//   - error: Any error encountered during validation
func (cs *CryptoService) IsBlockCandidate(jobID, extraNonce2, ntime, nonce string, template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx) (bool, error) {
	return IsBlockCandidate(jobID, extraNonce2, ntime, nonce, template, coinbaseTx)
}

// DifficultyToTarget converts mining difficulty to a target threshold.
// This method delegates to the package-level function for consistency.
//
// Parameters:
//   - difficulty: The mining difficulty as a floating-point number
//
// Returns:
//   - []byte: The 32-byte target threshold in big-endian format
func (cs *CryptoService) DifficultyToTarget(difficulty float64) []byte {
	return DifficultyToTarget(difficulty)
}

// HashMeetsTarget checks if a hash satisfies a difficulty target.
// This method delegates to the package-level function for consistency.
//
// Parameters:
//   - hash: The hash to validate (as a chainhash.Hash)
//   - target: The difficulty target as a 32-byte threshold
//
// Returns:
//   - bool: True if the hash is less than or equal to the target
func (cs *CryptoService) HashMeetsTarget(hash chainhash.Hash, target []byte) bool {
	return HashMeetsTarget(hash, target)
}

// Compile-time interface compliance check
var _ CryptoInterface = (*CryptoService)(nil)
