// Package bitcoin provides Bitcoin protocol operations for mining pool functionality.
// It includes cryptographic primitives for block construction, validation, and
// difficulty calculations, as well as RPC communication with Bitcoin Core.
package bitcoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// Pool optimizations for high-performance mining operations.
// These pools reduce garbage collection pressure in hot paths by reusing allocations.
var (
	// bufferPool provides reusable byte buffers for serialization operations.
	// This is critical for block reconstruction and transaction serialization hot paths.
	bufferPool = sync.Pool{
		New: func() any {
			// Pre-allocate 1MB buffer - typical for Bitcoin blocks
			return bytes.NewBuffer(make([]byte, 0, 1024*1024))
		},
	}

	// hashSlicePool provides reusable slices for transaction hash operations.
	// Used heavily in merkle tree calculations and branch generation.
	hashSlicePool = sync.Pool{
		New: func() any {
			// Pre-allocate for typical block size (~2000-4000 transactions)
			return make([]chainhash.Hash, 0, 4000)
		},
	}

	// byteSlicePool provides reusable byte slices for various operations.
	// Used for target calculations and hash comparisons.
	byteSlicePool = sync.Pool{
		New: func() any {
			// 32-byte slices for hash operations
			return make([]byte, 32)
		},
	}

	// bigIntPool provides reusable big.Int instances for difficulty calculations.
	// Reduces allocations in DifficultyToTarget hot path.
	bigIntPool = sync.Pool{
		New: func() any {
			return new(big.Int)
		},
	}

	// bigFloatPool provides reusable big.Float instances for precise difficulty calculations.
	bigFloatPool = sync.Pool{
		New: func() any {
			return new(big.Float)
		},
	}
)

// getBuffer retrieves a reusable buffer from the pool.
// Remember to call putBuffer() when done to return it to the pool.
func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putBuffer returns a buffer to the pool for reuse.
func putBuffer(buf *bytes.Buffer) {
	// Prevent memory leaks by limiting buffer size
	if buf.Cap() < 10*1024*1024 { // 10MB limit
		bufferPool.Put(buf)
	}
}

// getHashSlice retrieves a reusable hash slice from the pool.
// Remember to call putHashSlice() when done.
func getHashSlice() []chainhash.Hash {
	slice := hashSlicePool.Get().([]chainhash.Hash)
	return slice[:0] // Reset length but keep capacity
}

// putHashSlice returns a hash slice to the pool for reuse.
func putHashSlice(slice []chainhash.Hash) {
	// Prevent memory leaks by limiting slice size
	if cap(slice) < 10000 {
		hashSlicePool.Put(slice)
	}
}

// getByteSlice retrieves a reusable 32-byte slice from the pool.
func getByteSlice() []byte {
	return byteSlicePool.Get().([]byte)
}

// putByteSlice returns a byte slice to the pool for reuse.
func putByteSlice(slice []byte) {
	byteSlicePool.Put(slice)
}

// getBigInt retrieves a reusable big.Int from the pool.
func getBigInt() *big.Int {
	bi := bigIntPool.Get().(*big.Int)
	bi.SetInt64(0) // Reset to zero
	return bi
}

// putBigInt returns a big.Int to the pool for reuse.
func putBigInt(bi *big.Int) {
	bigIntPool.Put(bi)
}

// getBigFloat retrieves a reusable big.Float from the pool.
func getBigFloat() *big.Float {
	bf := bigFloatPool.Get().(*big.Float)
	bf.SetFloat64(0) // Reset to zero
	return bf
}

// putBigFloat returns a big.Float to the pool for reuse.
func putBigFloat(bf *big.Float) {
	bigFloatPool.Put(bf)
}

// CreateCoinbaseTransaction creates a BIP 34 compliant coinbase transaction for mining.
// It generates the coinbase transaction with block height, pool signature, and split
// points for Stratum mining protocol.
//
// Parameters:
//   - blockHeight: The height of the block (required for BIP 34)
//   - coinbaseValue: The total coinbase reward in satoshis
//   - extraNonce1: The first part of the extra nonce for mining
//   - poolAddress: The Bitcoin address to receive mining rewards
//   - chainParams: The Bitcoin network parameters (mainnet, testnet, etc.)
//
// Returns:
//   - *wire.MsgTx: The complete coinbase transaction
//   - string: Coinb1 - first part of serialized coinbase (before extra nonce)
//   - string: Coinb2 - second part of serialized coinbase (after extra nonce)
//   - error: Any error encountered during transaction creation
func CreateCoinbaseTransaction(blockHeight int64, coinbaseValue int64, extraNonce1 string, poolAddress string, chainParams *chaincfg.Params) (*wire.MsgTx, string, string, error) {
	// Create coinbase transaction
	coinbaseTx := wire.NewMsgTx(wire.TxVersion)

	// Coinbase input (previous output is null hash with index 0xffffffff)
	coinbaseInput := &wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
		},
		Sequence: 0xffffffff,
	}

	// Create coinbase script with block height (BIP 34)
	heightScript, err := txscript.NewScriptBuilder().AddInt64(blockHeight).Script()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create height script: %w", err)
	}

	// Add pool signature/identifier
	poolSig := []byte("/GOMP/")

	// Calculate split point - after height and pool sig, before extra nonce
	splitPoint := len(heightScript) + len(poolSig)

	// Create coinb1 (everything before extra nonce)
	coinb1Script := append(heightScript, poolSig...)

	// Extra nonce placeholder (8 bytes: 4 ExtraNonce1 + 4 ExtraNonce2)
	extraNoncePlaceholder := make([]byte, 8)
	if extraNonce1 != "" {
		extraNonce1Bytes, err := hex.DecodeString(extraNonce1)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to decode extra nonce 1: %w", err)
		}
		copy(extraNoncePlaceholder[:len(extraNonce1Bytes)], extraNonce1Bytes)
	}

	// Complete script for transaction creation
	fullScript := append(coinb1Script, extraNoncePlaceholder...)
	coinbaseInput.SignatureScript = fullScript
	coinbaseTx.AddTxIn(coinbaseInput)

	// Create output to pool address
	if poolAddress == "" {
		poolAddress = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" // Default to genesis address
	}

	poolAddr, err := btcutil.DecodeAddress(poolAddress, chainParams)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to decode pool address: %w", err)
	}

	pkScript, err := txscript.PayToAddrScript(poolAddr)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create output script: %w", err)
	}

	coinbaseOutput := &wire.TxOut{
		Value:    coinbaseValue,
		PkScript: pkScript,
	}
	coinbaseTx.AddTxOut(coinbaseOutput)

	// Serialize transaction to create coinb1/coinb2 split (using pool for efficiency)
	buf := getBuffer()
	defer putBuffer(buf)

	if err := coinbaseTx.Serialize(buf); err != nil {
		return nil, "", "", fmt.Errorf("failed to serialize coinbase: %w", err)
	}

	coinbaseBytes := buf.Bytes()

	// Calculate precise split point in serialized transaction
	// Structure: version(4) + input_count(1) + prev_hash(32) + prev_index(4) + script_length(varint) + script + sequence(4) + output_count(1) + outputs + locktime(4)
	scriptLengthStart := 4 + 1 + 32 + 4
	scriptLength := len(fullScript)

	// Find actual script start (after script length varint)
	scriptStart := scriptLengthStart + 1 // Assuming script length fits in 1 byte
	if scriptLength >= 253 {
		scriptStart = scriptLengthStart + 3 // 3-byte varint
	}

	actualSplitPoint := scriptStart + splitPoint

	// Validate split point
	if actualSplitPoint >= len(coinbaseBytes)-8 {
		return nil, "", "", fmt.Errorf("invalid split point calculation")
	}

	coinb1 := hex.EncodeToString(coinbaseBytes[:actualSplitPoint])
	coinb2 := hex.EncodeToString(coinbaseBytes[actualSplitPoint+8:]) // Skip 8 bytes for extra nonce

	return coinbaseTx, coinb1, coinb2, nil
}

// CalculateMerkleRoot calculates the Bitcoin merkle root from a list of transaction hashes.
// It implements the standard Bitcoin merkle tree algorithm used in block headers.
// For odd numbers of transactions, the last transaction is duplicated.
//
// Parameters:
//   - txHashes: Slice of transaction hashes to build the merkle tree from
//
// Returns:
//   - chainhash.Hash: The calculated merkle root hash
func CalculateMerkleRoot(txHashes []chainhash.Hash) chainhash.Hash {
	if len(txHashes) == 0 {
		return chainhash.Hash{}
	}

	if len(txHashes) == 1 {
		return txHashes[0]
	}

	// Build merkle tree level by level (using pools for efficiency)
	currentLevel := getHashSlice()
	defer putHashSlice(currentLevel)
	currentLevel = append(currentLevel, txHashes...)

	for len(currentLevel) > 1 {
		nextLevel := getHashSlice()

		for i := 0; i < len(currentLevel); i += 2 {
			var left, right chainhash.Hash
			left = currentLevel[i]

			if i+1 < len(currentLevel) {
				right = currentLevel[i+1]
			} else {
				right = left // Duplicate if odd number
			}

			// Concatenate and double SHA256
			concat := append(left[:], right[:]...)
			hash1 := sha256.Sum256(concat)
			hash2 := sha256.Sum256(hash1[:])

			newHash, _ := chainhash.NewHash(hash2[:])
			nextLevel = append(nextLevel, *newHash)
		}

		// Return old level to pool and use new level
		putHashSlice(currentLevel)
		currentLevel = nextLevel
	}

	result := currentLevel[0]
	return result
}

// GetMerkleBranch calculates the merkle branch (authentication path) for a transaction.
// This is used in the Stratum mining protocol to allow miners to verify their
// shares without downloading the entire block.
//
// Parameters:
//   - txHashes: Complete list of transaction hashes in the block
//   - txIndex: Index of the transaction to generate the branch for (typically 0 for coinbase)
//
// Returns:
//   - []chainhash.Hash: The merkle branch hashes needed to verify the transaction
func GetMerkleBranch(txHashes []chainhash.Hash, txIndex int) []chainhash.Hash {
	if len(txHashes) <= 1 || txIndex >= len(txHashes) {
		return []chainhash.Hash{}
	}

	// Build branch using pools for efficiency
	currentLevel := getHashSlice()
	defer putHashSlice(currentLevel)
	currentLevel = append(currentLevel, txHashes...)
	currentIndex := txIndex

	// Use a regular slice for branch return value
	var branch []chainhash.Hash

	for len(currentLevel) > 1 {
		// Find sibling
		var siblingIndex int
		if currentIndex%2 == 0 {
			siblingIndex = currentIndex + 1
		} else {
			siblingIndex = currentIndex - 1
		}

		// Add sibling to branch if it exists
		if siblingIndex < len(currentLevel) {
			branch = append(branch, currentLevel[siblingIndex])
		}

		// Build next level
		nextLevel := getHashSlice()
		for i := 0; i < len(currentLevel); i += 2 {
			var left, right chainhash.Hash
			left = currentLevel[i]

			if i+1 < len(currentLevel) {
				right = currentLevel[i+1]
			} else {
				right = left
			}

			concat := append(left[:], right[:]...)
			hash1 := sha256.Sum256(concat)
			hash2 := sha256.Sum256(hash1[:])

			newHash, _ := chainhash.NewHash(hash2[:])
			nextLevel = append(nextLevel, *newHash)
		}

		// Return old level to pool and use new level
		putHashSlice(currentLevel)
		currentLevel = nextLevel
		currentIndex /= 2
	}

	return branch
}

// ReconstructBlock reconstructs a complete Bitcoin block from mining share components.
// This is used to convert a successful mining share back into a complete block
// for submission to the Bitcoin network.
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
func ReconstructBlock(template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx, _, ntime, nonce string) (*wire.MsgBlock, string, error) {
	// Parse ntime and nonce
	ntimeInt, err := parseHexUint32(ntime)
	if err != nil {
		return nil, "", fmt.Errorf("invalid ntime: %w", err)
	}

	nonceInt, err := parseHexUint32(nonce)
	if err != nil {
		return nil, "", fmt.Errorf("invalid nonce: %w", err)
	}

	// Create block header
	prevHash, err := chainhash.NewHashFromStr(template.PreviousHash)
	if err != nil {
		return nil, "", fmt.Errorf("invalid previous block hash: %w", err)
	}

	// Build transaction list starting with coinbase
	transactions := []*wire.MsgTx{coinbaseTx}

	// Add template transactions
	for _, tx := range template.Transactions {
		txBytes, err := hex.DecodeString(tx.Data)
		if err != nil {
			return nil, "", fmt.Errorf("invalid transaction data: %w", err)
		}

		msgTx := &wire.MsgTx{}
		if err := msgTx.Deserialize(bytes.NewReader(txBytes)); err != nil {
			return nil, "", fmt.Errorf("failed to deserialize transaction: %w", err)
		}

		transactions = append(transactions, msgTx)
	}

	// Calculate merkle root
	txHashes := make([]chainhash.Hash, len(transactions))
	for i, tx := range transactions {
		txHashes[i] = tx.TxHash()
	}
	merkleRoot := CalculateMerkleRoot(txHashes)

	// Parse bits
	bits, err := parseHexUint32(template.Bits)
	if err != nil {
		return nil, "", fmt.Errorf("invalid bits: %w", err)
	}

	// Create block header
	header := &wire.BlockHeader{
		Version:    template.Version,
		PrevBlock:  *prevHash,
		MerkleRoot: merkleRoot,
		Timestamp:  time.Unix(int64(ntimeInt), 0),
		Bits:       bits,
		Nonce:      nonceInt,
	}

	// Create complete block
	block := &wire.MsgBlock{
		Header:       *header,
		Transactions: transactions,
	}

	// Serialize block to hex (using pool for efficiency)
	buf := getBuffer()
	defer putBuffer(buf)

	if err := block.Serialize(buf); err != nil {
		return nil, "", fmt.Errorf("failed to serialize block: %w", err)
	}

	blockHex := hex.EncodeToString(buf.Bytes())
	return block, blockHex, nil
}

// ValidateShare validates that a mining share meets the required difficulty target.
// This is used to verify that miners have performed the required proof-of-work
// for their submitted shares.
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
func ValidateShare(_, extraNonce2, ntime, nonce string, difficulty float64, template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx) error {
	// Reconstruct block header for validation
	block, _, err := ReconstructBlock(template, coinbaseTx, extraNonce2, ntime, nonce)
	if err != nil {
		return fmt.Errorf("failed to reconstruct block: %w", err)
	}

	// Validate block header (using pool for efficiency)
	headerBytes := make([]byte, 80)
	buf := getBuffer()
	defer putBuffer(buf)

	if err := block.Header.Serialize(buf); err != nil {
		return fmt.Errorf("failed to serialize header: %w", err)
	}
	copy(headerBytes, buf.Bytes())

	// Calculate hash
	hash := chainhash.DoubleHashH(headerBytes)

	// Convert difficulty to target
	target := DifficultyToTarget(difficulty)

	// Check if hash meets difficulty target
	if !HashMeetsTarget(hash, target) {
		return fmt.Errorf("share does not meet difficulty target")
	}

	return nil
}

// IsBlockCandidate determines if a valid share also meets the network difficulty,
// making it a potential block solution that should be submitted to Bitcoin Core.
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
func IsBlockCandidate(_, extraNonce2, ntime, nonce string, template *btcjson.GetBlockTemplateResult, coinbaseTx *wire.MsgTx) (bool, error) {
	// Reconstruct block header
	block, _, err := ReconstructBlock(template, coinbaseTx, extraNonce2, ntime, nonce)
	if err != nil {
		return false, fmt.Errorf("failed to reconstruct block: %w", err)
	}

	// Calculate hash (using pool for efficiency)
	headerBytes := make([]byte, 80)
	buf := getBuffer()
	defer putBuffer(buf)

	if err := block.Header.Serialize(buf); err != nil {
		return false, fmt.Errorf("failed to serialize header: %w", err)
	}
	copy(headerBytes, buf.Bytes())

	hash := chainhash.DoubleHashH(headerBytes)

	// Parse network target from template
	networkTarget, err := parseHexTarget(template.Target)
	if err != nil {
		return false, fmt.Errorf("invalid network target: %w", err)
	}

	// Check if hash meets network difficulty
	return HashMeetsTarget(hash, networkTarget), nil
}

// DifficultyToTarget converts a mining difficulty value to a target hash threshold.
// The target represents the maximum hash value that satisfies the given difficulty.
// Lower targets (higher difficulty) require more computational work.
//
// This implementation uses proper big integer arithmetic for precise calculations,
// following Bitcoin's standard difficulty-to-target conversion algorithm.
//
// Parameters:
//   - difficulty: The mining difficulty as a floating-point number
//
// Returns:
//   - []byte: The 32-byte target threshold in big-endian format
func DifficultyToTarget(difficulty float64) []byte {
	// Validate difficulty input
	if difficulty <= 0 {
		// Return maximum target for invalid/zero difficulty
		return []byte{
			0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		}
	}

	// Bitcoin's maximum target (difficulty 1) as a big integer
	// This is 0x00000000FFFF0000000000000000000000000000000000000000000000000000
	maxTargetBytes := []byte{
		0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	// Convert to big integer (using pools for efficiency)
	maxTarget := getBigInt()
	defer putBigInt(maxTarget)
	maxTarget.SetBytes(maxTargetBytes)

	// Use big.Float for precise division with fractional difficulties
	difficultyFloat := getBigFloat()
	defer putBigFloat(difficultyFloat)
	difficultyFloat.SetFloat64(difficulty)

	maxTargetFloat := getBigFloat()
	defer putBigFloat(maxTargetFloat)
	maxTargetFloat.SetInt(maxTarget)

	// Calculate target = maxTarget / difficulty
	targetFloat := getBigFloat()
	defer putBigFloat(targetFloat)
	targetFloat.Quo(maxTargetFloat, difficultyFloat)

	// Convert back to big.Int
	target := getBigInt()
	defer putBigInt(target)
	targetFloat.Int(target)

	// Convert back to 32-byte array
	targetBytes := target.Bytes()
	result := make([]byte, 32)

	// Copy target bytes to result (right-aligned for big-endian)
	if len(targetBytes) <= 32 {
		copy(result[32-len(targetBytes):], targetBytes)
	} else {
		// Target is too large, return maximum target
		copy(result, maxTargetBytes)
	}

	return result
}

// HashMeetsTarget determines if a given hash satisfies the difficulty target.
// This implements the Bitcoin proof-of-work validation by comparing the hash
// value against the target threshold.
//
// Parameters:
//   - hash: The hash to validate (typically a block header hash)
//   - target: The difficulty target as a 32-byte threshold
//
// Returns:
//   - bool: True if the hash is less than or equal to the target
func HashMeetsTarget(hash chainhash.Hash, target []byte) bool {
	// Convert hash to bytes (reverse for little-endian comparison, using pool)
	hashBytes := getByteSlice()
	defer putByteSlice(hashBytes)

	for i := range 32 {
		hashBytes[i] = hash[31-i]
	}

	// Compare hash with target
	for i := range 32 {
		if hashBytes[i] < target[i] {
			return true
		}
		if hashBytes[i] > target[i] {
			return false
		}
	}
	return true
}

// parseHexUint32 parses a hex string to uint32 with input validation.
func parseHexUint32(hexStr string) (uint32, error) {
	// Validate hex string length (must be exactly 8 hex characters for 4 bytes)
	if len(hexStr) != 8 {
		return 0, fmt.Errorf("invalid hex string length: expected 8 characters, got %d", len(hexStr))
	}

	// Validate hex string contains only valid hex characters
	for i, c := range hexStr {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return 0, fmt.Errorf("invalid hex character '%c' at position %d", c, i)
		}
	}

	val, err := hex.DecodeString(hexStr)
	if err != nil {
		return 0, fmt.Errorf("failed to decode hex string: %w", err)
	}
	if len(val) != 4 {
		return 0, fmt.Errorf("expected 4 bytes, got %d", len(val))
	}

	// Convert little-endian bytes to uint32
	return uint32(val[0]) | uint32(val[1])<<8 | uint32(val[2])<<16 | uint32(val[3])<<24, nil
}

// parseHexTarget parses a hex target string to bytes with validation.
func parseHexTarget(targetStr string) ([]byte, error) {
	// Validate input length (must be even number of hex characters)
	if len(targetStr) == 0 {
		return nil, fmt.Errorf("target string cannot be empty")
	}
	if len(targetStr)%2 != 0 {
		return nil, fmt.Errorf("target string must have even length, got %d", len(targetStr))
	}
	if len(targetStr) > 64 {
		return nil, fmt.Errorf("target string too long: maximum 64 hex characters (32 bytes), got %d", len(targetStr))
	}

	// Validate hex string contains only valid hex characters
	for i, c := range targetStr {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return nil, fmt.Errorf("invalid hex character '%c' at position %d", c, i)
		}
	}

	target, err := hex.DecodeString(targetStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex target: %w", err)
	}

	// Ensure target is exactly 32 bytes
	if len(target) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(target):], target)
		target = padded
	} else if len(target) > 32 {
		return nil, fmt.Errorf("target too large: maximum 32 bytes, got %d", len(target))
	}

	return target, nil
}
