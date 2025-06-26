package bitcoin

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"

	"github.com/bardlex/gomp/pkg/circuit"
	"github.com/bardlex/gomp/pkg/errors"
	"github.com/bardlex/gomp/pkg/retry"
)

// RPCClient provides a high-level interface to Bitcoin Core's JSON-RPC API.
// It wraps btcd's RPC client with additional mining-specific functionality
// and type conversions for seamless integration with mining pool operations.
type RPCClient struct {
	client         *rpcclient.Client
	circuitBreaker *circuit.Breaker
	retryConfig    *retry.Config
}

// NewRPCClient creates a new Bitcoin Core RPC client using btcd's robust
// RPC implementation. It configures the client for HTTP-only communication
// with TLS disabled, which is typical for local Bitcoin Core deployments.
//
// Parameters:
//   - host: Bitcoin Core hostname or IP address
//   - port: Bitcoin Core RPC port (typically 8332 for mainnet)
//   - username: RPC authentication username
//   - password: RPC authentication password
//
// Returns:
//   - *RPCClient: Configured RPC client ready for use
//   - error: Any error encountered during client creation
func NewRPCClient(host string, port int, username, password string) (*RPCClient, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("%s:%d", host, port),
		User:         username,
		Pass:         password,
		HTTPPostMode: true, // Use HTTP POST mode for Bitcoin Core compatibility
		DisableTLS:   true, // Bitcoin Core typically doesn't use TLS
	}

	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeBitcoin, "rpc_client_creation", 
			"failed to create Bitcoin RPC client").
			WithContext("host", host).
			WithContext("port", port)
	}

	// Configure circuit breaker for Bitcoin RPC
	cbConfig := &circuit.Config{
		MaxFailures:     3,
		SuccessRequired: 2,
		Timeout:         10 * time.Second,
		ResetTimeout:    30 * time.Second,
	}

	return &RPCClient{
		client:         client,
		circuitBreaker: circuit.New(cbConfig),
		retryConfig:    retry.NetworkConfig(),
	}, nil
}

// Close gracefully shuts down the RPC client and releases any resources.
// This should be called when the client is no longer needed.
func (c *RPCClient) Close() {
	c.client.Shutdown()
}

// GetBlockTemplate retrieves a block template from Bitcoin Core for mining.
// The template contains all transactions, difficulty target, and other data
// needed to construct a new block.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - *btcjson.GetBlockTemplateResult: The mining template with transactions and metadata
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetBlockTemplate(ctx context.Context) (*btcjson.GetBlockTemplateResult, error) {
	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (*btcjson.GetBlockTemplateResult, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (*btcjson.GetBlockTemplateResult, error) {
			req := &btcjson.TemplateRequest{
				Mode:         "template",
				Capabilities: []string{"coinbasetxn", "workid", "coinbase/append"},
				Rules:        []string{"segwit"},
			}

			template, err := c.client.GetBlockTemplateAsync(req).Receive()
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrorTypeBitcoin, "get_block_template", 
					"failed to retrieve block template from Bitcoin Core")
			}

			return template, nil
		})
	})
}

// GetBlockCount gets the current block count.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - int64: Current block height
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetBlockCount(ctx context.Context) (int64, error) {
	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (int64, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (int64, error) {
			count, err := c.client.GetBlockCountAsync().Receive()
			if err != nil {
				return 0, errors.Wrap(err, errors.ErrorTypeBitcoin, "get_block_count", 
					"failed to retrieve current block height")
			}
			return count, nil
		})
	})
}

// GetBestBlockHash gets the hash of the best block.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - string: Hash of the current best block
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetBestBlockHash(ctx context.Context) (string, error) {
	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (string, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (string, error) {
			hash, err := c.client.GetBestBlockHashAsync().Receive()
			if err != nil {
				return "", errors.Wrap(err, errors.ErrorTypeBitcoin, "get_best_block_hash", 
					"failed to retrieve best block hash")
			}
			return hash.String(), nil
		})
	})
}

// GetBlock gets block information by hash.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//   - hash: Block hash to retrieve
//
// Returns:
//   - *btcjson.GetBlockVerboseResult: Detailed block information
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetBlock(ctx context.Context, hash string) (*btcjson.GetBlockVerboseResult, error) {
	blockHash, err := chainhash.NewHashFromStr(hash)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeValidation, "hash_parsing", 
			"failed to parse block hash").
			WithContext("hash", hash)
	}

	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (*btcjson.GetBlockVerboseResult, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (*btcjson.GetBlockVerboseResult, error) {
			block, err := c.client.GetBlockVerboseAsync(blockHash).Receive()
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrorTypeBitcoin, "get_block", 
					"failed to retrieve block information").
					WithContext("block_hash", hash)
			}
			return block, nil
		})
	})
}

// GetNetworkInfo gets network information.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - *btcjson.GetNetworkInfoResult: Network status and configuration
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetNetworkInfo(ctx context.Context) (*btcjson.GetNetworkInfoResult, error) {
	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (*btcjson.GetNetworkInfoResult, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (*btcjson.GetNetworkInfoResult, error) {
			info, err := c.client.GetNetworkInfoAsync().Receive()
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrorTypeBitcoin, "get_network_info", 
					"failed to retrieve network information")
			}
			return info, nil
		})
	})
}

// SubmitBlock submits a solved block to the Bitcoin network.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//   - blockHex: Hexadecimal representation of the complete block
//
// Returns:
//   - error: Any error from Bitcoin Core or network
func (c *RPCClient) SubmitBlock(ctx context.Context, blockHex string) error {
	// Validate block hex first (no retry needed for validation)
	blockBytes, err := hex.DecodeString(blockHex)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "block_validation", 
			"invalid block hex encoding").
			WithContext("block_hex_length", len(blockHex))
	}

	block := &wire.MsgBlock{}
	if err := block.Deserialize(bytes.NewReader(blockBytes)); err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "block_deserialization", 
			"failed to deserialize block data").
			WithContext("block_size", len(blockBytes))
	}

	// Use circuit breaker but minimal retry for block submission (time-critical)
	submitConfig := &retry.Config{
		MaxAttempts: 2,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond,
		Multiplier:  1.5,
		Jitter:      false,
	}

	return c.circuitBreaker.Execute(ctx, func() error {
		return retry.Do(ctx, submitConfig, func() error {
			err := c.client.SubmitBlockAsync(btcutil.NewBlock(block), nil).Receive()
			if err != nil {
				return errors.Wrap(err, errors.ErrorTypeBitcoin, "submit_block", 
					"failed to submit block to Bitcoin Core").
					WithContext("block_hash", block.BlockHash().String())
			}
			return nil
		})
	})
}

// ValidateAddress validates a Bitcoin address.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//   - address: Bitcoin address to validate
//
// Returns:
//   - bool: True if the address is valid
//   - error: Any error during validation
func (c *RPCClient) ValidateAddress(ctx context.Context, address string) (bool, error) {
	addr, err := btcutil.DecodeAddress(address, &chaincfg.MainNetParams)
	if err != nil {
		// If we can't decode it, it's invalid (but not an error)
		return false, nil
	}

	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (bool, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (bool, error) {
			result, err := c.client.ValidateAddressAsync(addr).Receive()
			if err != nil {
				return false, errors.Wrap(err, errors.ErrorTypeBitcoin, "validate_address", 
					"failed to validate Bitcoin address").
					WithContext("address", address)
			}
			return result.IsValid, nil
		})
	})
}

// Ping tests the connection to Bitcoin Core.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - error: Any connection error
func (c *RPCClient) Ping(ctx context.Context) error {
	return c.circuitBreaker.Execute(ctx, func() error {
		return retry.Do(ctx, c.retryConfig, func() error {
			err := c.client.PingAsync().Receive()
			if err != nil {
				return errors.Wrap(err, errors.ErrorTypeNetwork, "ping", 
					"Bitcoin Core connectivity check failed")
			}
			return nil
		})
	})
}

// GetDifficulty gets the current network difficulty.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - float64: Current network difficulty
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetDifficulty(ctx context.Context) (float64, error) {
	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (float64, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (float64, error) {
			difficulty, err := c.client.GetDifficultyAsync().Receive()
			if err != nil {
				return 0, errors.Wrap(err, errors.ErrorTypeBitcoin, "get_difficulty", 
					"failed to retrieve network difficulty")
			}
			return difficulty, nil
		})
	})
}

// GetMiningInfo gets mining-related information.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - *btcjson.GetMiningInfoResult: Mining statistics and status
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetMiningInfo(ctx context.Context) (*btcjson.GetMiningInfoResult, error) {
	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (*btcjson.GetMiningInfoResult, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (*btcjson.GetMiningInfoResult, error) {
			info, err := c.client.GetMiningInfoAsync().Receive()
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrorTypeBitcoin, "get_mining_info", 
					"failed to retrieve mining information")
			}
			return info, nil
		})
	})
}

// GetBlockchainInfo gets blockchain information.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//
// Returns:
//   - *btcjson.GetBlockChainInfoResult: Blockchain status and statistics
//   - error: Any error from Bitcoin Core
func (c *RPCClient) GetBlockchainInfo(ctx context.Context) (*btcjson.GetBlockChainInfoResult, error) {
	return circuit.ExecuteWithResult(ctx, c.circuitBreaker, func() (*btcjson.GetBlockChainInfoResult, error) {
		return retry.DoWithResult(ctx, c.retryConfig, func() (*btcjson.GetBlockChainInfoResult, error) {
			info, err := c.client.GetBlockChainInfoAsync().Receive()
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrorTypeBitcoin, "get_blockchain_info", 
					"failed to retrieve blockchain information")
			}
			return info, nil
		})
	})
}

// Mining operations using pure crypto functions from crypto.go

// CreateCoinbaseTransaction creates a coinbase transaction using the crypto package.
// This method does not require Bitcoin Core connectivity as it uses pure cryptographic functions.
//
// Parameters:
//   - ctx: Context for cancellation (not used but maintained for API consistency)
//   - blockHeight: The height of the block (required for BIP 34)
//   - coinbaseValue: The total coinbase reward in satoshis
//   - extraNonce1: The first part of the extra nonce for mining
//   - poolAddress: The Bitcoin address to receive mining rewards
//
// Returns:
//   - *wire.MsgTx: The complete coinbase transaction
//   - string: Coinb1 - first part of serialized coinbase
//   - string: Coinb2 - second part of serialized coinbase
//   - error: Any error encountered during transaction creation
func (c *RPCClient) CreateCoinbaseTransaction(_ context.Context, blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string) (*wire.MsgTx, string, string, error) {
	return CreateCoinbaseTransaction(blockHeight, coinbaseValue, extraNonce1, poolAddress, &chaincfg.MainNetParams)
}

// CalculateMerkleRoot calculates merkle root from transaction hashes using crypto functions.
// This method does not require Bitcoin Core connectivity.
//
// Parameters:
//   - ctx: Context for cancellation (not used but maintained for API consistency)
//   - txHashes: Slice of transaction hash strings
//
// Returns:
//   - string: The calculated merkle root as a hex string
//   - error: Any error encountered during calculation
func (c *RPCClient) CalculateMerkleRoot(_ context.Context, txHashes []string) (string, error) {
	if len(txHashes) == 0 {
		return "", fmt.Errorf("no transactions provided")
	}

	// Convert string hashes to chainhash.Hash
	hashes := make([]chainhash.Hash, len(txHashes))
	for i, hashStr := range txHashes {
		hash, err := chainhash.NewHashFromStr(hashStr)
		if err != nil {
			return "", fmt.Errorf("invalid hash %s: %w", hashStr, err)
		}
		hashes[i] = *hash
	}

	// Calculate merkle root
	merkleRoot := CalculateMerkleRoot(hashes)
	return merkleRoot.String(), nil
}

// GetMerkleBranch calculates the merkle branch for coinbase (index 0) using crypto functions.
// This method does not require Bitcoin Core connectivity.
//
// Parameters:
//   - ctx: Context for cancellation (not used but maintained for API consistency)
//   - txHashes: Complete list of transaction hash strings in the block
//
// Returns:
//   - []string: The merkle branch hashes needed to verify the coinbase transaction
//   - error: Any error encountered during calculation
func (c *RPCClient) GetMerkleBranch(_ context.Context, txHashes []string) ([]string, error) {
	if len(txHashes) <= 1 {
		return []string{}, nil
	}

	// Convert string hashes to chainhash.Hash
	hashes := make([]chainhash.Hash, len(txHashes))
	for i, hashStr := range txHashes {
		hash, err := chainhash.NewHashFromStr(hashStr)
		if err != nil {
			return nil, fmt.Errorf("invalid hash %s: %w", hashStr, err)
		}
		hashes[i] = *hash
	}

	// Get merkle branch for coinbase (index 0)
	branch := GetMerkleBranch(hashes, 0)

	// Convert back to strings
	branchStrs := make([]string, len(branch))
	for i, hash := range branch {
		branchStrs[i] = hash.String()
	}

	return branchStrs, nil
}
