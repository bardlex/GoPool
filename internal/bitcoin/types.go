// Package bitcoin provides Bitcoin protocol types and RPC structures.
package bitcoin

import (
	"encoding/json"
)

// RPCRequest represents a JSON-RPC 1.0 request to Bitcoin Core.
// It follows the JSON-RPC specification used by Bitcoin Core's RPC interface.
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []any `json:"params"`
}

// RPCResponse represents a JSON-RPC 1.0 response from Bitcoin Core.
// Contains either a result or an error, never both.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
}

// RPCError represents an error returned by Bitcoin Core's JSON-RPC interface.
// Includes both an error code and descriptive message.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Note: BlockTemplate types are now imported from btcjson.GetBlockTemplateResult
// This eliminates the need for custom types and reduces maintenance overhead.

// BlockInfo contains comprehensive information about a Bitcoin block.
// Used for block explorers and validation purposes.
type BlockInfo struct {
	Hash              string  `json:"hash"`
	Confirmations     int64   `json:"confirmations"`
	Size              int32   `json:"size"`
	StrippedSize      int32   `json:"strippedsize"`
	Weight            int32   `json:"weight"`
	Height            int64   `json:"height"`
	Version           int32   `json:"version"`
	VersionHex        string  `json:"versionHex"`
	MerkleRoot        string  `json:"merkleroot"`
	Time              int64   `json:"time"`
	MedianTime        int64   `json:"mediantime"`
	Nonce             uint32  `json:"nonce"`
	Bits              string  `json:"bits"`
	Difficulty        float64 `json:"difficulty"`
	ChainWork         string  `json:"chainwork"`
	PreviousBlockHash string  `json:"previousblockhash"`
	NextBlockHash     string  `json:"nextblockhash"`
}

// NetworkInfo contains information about the Bitcoin network state
// and connectivity. Used for monitoring and diagnostics.
type NetworkInfo struct {
	Version         int32     `json:"version"`
	SubVersion      string    `json:"subversion"`
	ProtocolVersion int32     `json:"protocolversion"`
	Connections     int32     `json:"connections"`
	NetworkActive   bool      `json:"networkactive"`
	Networks        []Network `json:"networks"`
	RelayFee        float64   `json:"relayfee"`
	IncrementalFee  float64   `json:"incrementalfee"`
}

// Network represents configuration and status information for
// a specific network interface (IPv4, IPv6, Tor, etc.).
type Network struct {
	Name                      string `json:"name"`
	Limited                   bool   `json:"limited"`
	Reachable                 bool   `json:"reachable"`
	Proxy                     string `json:"proxy"`
	ProxyRandomizeCredentials bool   `json:"proxy_randomize_credentials"`
}

// MiningInfo contains current Bitcoin network mining statistics
// including difficulty, hash rate estimates, and mining pool data.
type MiningInfo struct {
	Blocks           int64   `json:"blocks"`
	CurrentBlockSize int64   `json:"currentblocksize"`
	CurrentBlockTx   int64   `json:"currentblocktx"`
	Difficulty       float64 `json:"difficulty"`
	NetworkHashPS    float64 `json:"networkhashps"`
	PooledTx         int64   `json:"pooledtx"`
	Chain            string  `json:"chain"`
	Warnings         string  `json:"warnings"`
}

// BlockchainInfo contains comprehensive Bitcoin blockchain state information
// including chain tip, verification progress, and network consensus rules.
type BlockchainInfo struct {
	Chain                string                 `json:"chain"`
	Blocks               int64                  `json:"blocks"`
	Headers              int64                  `json:"headers"`
	BestBlockHash        string                 `json:"bestblockhash"`
	Difficulty           float64                `json:"difficulty"`
	MedianTime           int64                  `json:"mediantime"`
	VerificationProgress float64                `json:"verificationprogress"`
	InitialBlockDownload bool                   `json:"initialblockdownload"`
	ChainWork            string                 `json:"chainwork"`
	SizeOnDisk           int64                  `json:"size_on_disk"`
	Pruned               bool                   `json:"pruned"`
	Softforks            map[string]any `json:"softforks"`
	Warnings             string                 `json:"warnings"`
}

// AddressValidation contains the result of Bitcoin address validation,
// including whether the address is valid and its associated script information.
type AddressValidation struct {
	IsValid      bool   `json:"isvalid"`
	Address      string `json:"address,omitempty"`
	ScriptPubKey string `json:"scriptPubKey,omitempty"`
	IsScript     bool   `json:"isscript,omitempty"`
	IsWitness    bool   `json:"iswitness,omitempty"`
}
