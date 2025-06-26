// Package validation provides share validation logic for the GOMP mining pool.
// It handles validating mining shares against difficulty targets and detecting block candidates.
package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"time"
)

// ShareValidator handles validation of mining shares
type ShareValidator struct {
	minDifficulty float64
	maxDifficulty float64
	maxTimeSkew   time.Duration
}

// NewShareValidator creates a new share validator
func NewShareValidator(minDiff, maxDiff float64, maxTimeSkew time.Duration) *ShareValidator {
	return &ShareValidator{
		minDifficulty: minDiff,
		maxDifficulty: maxDiff,
		maxTimeSkew:   maxTimeSkew,
	}
}

// ValidateShare performs comprehensive validation of a mining share
func (v *ShareValidator) ValidateShare(share *Share, jobTemplate *MiningJob) error {
	// Basic field validation
	if err := v.validateBasicFields(share); err != nil {
		return fmt.Errorf("basic validation failed: %w", err)
	}

	// Job validation
	if err := v.validateJob(share, jobTemplate); err != nil {
		return fmt.Errorf("job validation failed: %w", err)
	}

	// Time validation
	if err := v.validateTime(share, jobTemplate); err != nil {
		return fmt.Errorf("time validation failed: %w", err)
	}

	// Difficulty validation
	if err := v.validateDifficulty(share); err != nil {
		return fmt.Errorf("difficulty validation failed: %w", err)
	}

	// Cryptographic validation
	if err := v.validateProofOfWork(share, jobTemplate); err != nil {
		return fmt.Errorf("proof of work validation failed: %w", err)
	}

	return nil
}

// validateBasicFields checks that all required fields are present and valid
func (v *ShareValidator) validateBasicFields(share *Share) error {
	if share.ID == "" {
		return fmt.Errorf("share ID is required")
	}

	if share.JobID == "" {
		return fmt.Errorf("job ID is required")
	}

	if share.MinerAddress == "" {
		return fmt.Errorf("miner address is required")
	}

	if share.WorkerName == "" {
		return fmt.Errorf("worker name is required")
	}

	if share.ExtraNonce2 == "" {
		return fmt.Errorf("extra nonce2 is required")
	}

	if share.Ntime == "" {
		return fmt.Errorf("ntime is required")
	}

	if share.Nonce == "" {
		return fmt.Errorf("nonce is required")
	}

	// Validate hex encoding
	if !isValidHex(share.ExtraNonce2) {
		return fmt.Errorf("extra nonce2 is not valid hex")
	}

	if !isValidHex(share.Ntime) {
		return fmt.Errorf("ntime is not valid hex")
	}

	if !isValidHex(share.Nonce) {
		return fmt.Errorf("nonce is not valid hex")
	}

	return nil
}

// validateJob checks that the share references a valid job
func (v *ShareValidator) validateJob(share *Share, jobTemplate *MiningJob) error {
	if jobTemplate == nil {
		return fmt.Errorf("job template not found")
	}

	if share.JobID != jobTemplate.JobID {
		return fmt.Errorf("job ID mismatch")
	}

	// Check if job has expired
	if jobTemplate.ExpiresAt != nil {
		if time.Now().After(*jobTemplate.ExpiresAt) {
			return fmt.Errorf("job has expired")
		}
	}

	return nil
}

// validateTime checks that the timestamp is within acceptable bounds
func (v *ShareValidator) validateTime(share *Share, jobTemplate *MiningJob) error {
	// Parse ntime from hex
	ntimeInt, err := strconv.ParseInt(share.Ntime, 16, 64)
	if err != nil {
		return fmt.Errorf("invalid ntime format: %w", err)
	}

	shareTime := time.Unix(ntimeInt, 0)
	now := time.Now()

	// Check if time is too far in the future
	if shareTime.After(now.Add(v.maxTimeSkew)) {
		return fmt.Errorf("share time too far in future")
	}

	// Check if time is too far in the past (relative to job creation)
	jobTime := jobTemplate.CreatedAt
	if shareTime.Before(jobTime.Add(-v.maxTimeSkew)) {
		return fmt.Errorf("share time too far in past")
	}

	return nil
}

// validateDifficulty checks that the share difficulty is within acceptable bounds
func (v *ShareValidator) validateDifficulty(share *Share) error {
	if share.Difficulty < v.minDifficulty {
		return fmt.Errorf("difficulty too low: %f < %f", share.Difficulty, v.minDifficulty)
	}

	if share.Difficulty > v.maxDifficulty {
		return fmt.Errorf("difficulty too high: %f > %f", share.Difficulty, v.maxDifficulty)
	}

	return nil
}

// validateProofOfWork performs the cryptographic validation of the share
func (v *ShareValidator) validateProofOfWork(share *Share, jobTemplate *MiningJob) error {
	// Reconstruct the block header
	blockHeader, err := v.reconstructBlockHeader(share, jobTemplate)
	if err != nil {
		return fmt.Errorf("failed to reconstruct block header: %w", err)
	}

	// Calculate the hash
	hash := sha256.Sum256(blockHeader)
	hash = sha256.Sum256(hash[:]) // Double SHA256

	// Convert hash to big integer (reverse byte order for little-endian)
	hashBytes := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hashBytes[i] = hash[31-i]
	}
	hashInt := new(big.Int).SetBytes(hashBytes)

	// Calculate target from difficulty
	target := v.calculateTarget(share.Difficulty)

	// Check if hash meets the target
	if hashInt.Cmp(target) > 0 {
		return fmt.Errorf("hash does not meet difficulty target")
	}

	return nil
}

// reconstructBlockHeader rebuilds the 80-byte block header from share and job data
func (v *ShareValidator) reconstructBlockHeader(share *Share, jobTemplate *MiningJob) ([]byte, error) {
	// This is a simplified version - in production, you'd need to:
	// 1. Reconstruct the coinbase transaction with ExtraNonce1 + ExtraNonce2
	// 2. Calculate the merkle root using the coinbase and merkle branch
	// 3. Assemble the complete 80-byte header

	// For now, return a placeholder that demonstrates the structure
	header := make([]byte, 80)

	// Version (4 bytes)
	versionBytes, err := hex.DecodeString(jobTemplate.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}
	copy(header[0:4], versionBytes)

	// Previous hash (32 bytes)
	prevHashBytes, err := hex.DecodeString(jobTemplate.PrevHash)
	if err != nil {
		return nil, fmt.Errorf("invalid prev hash: %w", err)
	}
	copy(header[4:36], prevHashBytes)

	// Merkle root (32 bytes) - would be calculated from coinbase + merkle branch
	// For now, using zeros as placeholder
	// copy(header[36:68], merkleRoot)

	// Time (4 bytes)
	ntimeBytes, err := hex.DecodeString(share.Ntime)
	if err != nil {
		return nil, fmt.Errorf("invalid ntime: %w", err)
	}
	copy(header[68:72], ntimeBytes)

	// Bits (4 bytes)
	nbitsBytes, err := hex.DecodeString(jobTemplate.Nbits)
	if err != nil {
		return nil, fmt.Errorf("invalid nbits: %w", err)
	}
	copy(header[72:76], nbitsBytes)

	// Nonce (4 bytes)
	nonceBytes, err := hex.DecodeString(share.Nonce)
	if err != nil {
		return nil, fmt.Errorf("invalid nonce: %w", err)
	}
	copy(header[76:80], nonceBytes)

	return header, nil
}

// calculateTarget converts difficulty to target value
func (v *ShareValidator) calculateTarget(difficulty float64) *big.Int {
	// Bitcoin's maximum target (difficulty 1)
	maxTarget := new(big.Int)
	maxTarget.SetString("00000000FFFF0000000000000000000000000000000000000000000000000000", 16)

	// Target = maxTarget / difficulty
	diffInt := new(big.Int)
	diffInt.SetInt64(int64(difficulty))

	target := new(big.Int).Div(maxTarget, diffInt)
	return target
}

// isValidHex checks if a string is valid hexadecimal
func isValidHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// IsBlockCandidate checks if a share meets the network difficulty target
func (v *ShareValidator) IsBlockCandidate(share *Share) bool {
	return share.Difficulty >= share.NetworkDifficulty
}
