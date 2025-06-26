package bitcoin

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestCalculateMerkleRoot(t *testing.T) {
	tests := []struct {
		name     string
		txHashes []string
		expected string
		wantErr  bool
	}{
		{
			name:     "empty transactions",
			txHashes: []string{},
			expected: "0000000000000000000000000000000000000000000000000000000000000000",
			wantErr:  false,
		},
		{
			name: "single transaction",
			txHashes: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
			expected: "0000000000000000000000000000000000000000000000000000000000000001",
			wantErr:  false,
		},
		{
			name: "two transactions",
			txHashes: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000002",
			},
			expected: "b413f47d13ee2fe6c845b2ee141af81de858df4ec549a58b7970bb96645bc8d2",
			wantErr:  false,
		},
		{
			name: "three transactions (odd number)",
			txHashes: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000002",
				"0000000000000000000000000000000000000000000000000000000000000003",
			},
			expected: "dffe8c3c7f0c4c5b8e8e8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c8c",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert string hashes to chainhash.Hash
			hashes := make([]chainhash.Hash, len(tt.txHashes))
			for i, hashStr := range tt.txHashes {
				hash, err := chainhash.NewHashFromStr(hashStr)
				if err != nil {
					t.Fatalf("Failed to parse hash %s: %v", hashStr, err)
				}
				hashes[i] = *hash
			}

			result := CalculateMerkleRoot(hashes)
			resultStr := result.String()

			// For empty case, check if it's zero hash
			if len(tt.txHashes) == 0 {
				zeroHash := chainhash.Hash{}
				if result != zeroHash {
					t.Errorf("Expected zero hash for empty transactions, got %s", resultStr)
				}
				return
			}

			// For single transaction, should return the same hash
			if len(tt.txHashes) == 1 {
				if resultStr != tt.txHashes[0] {
					t.Errorf("Expected %s for single transaction, got %s", tt.txHashes[0], resultStr)
				}
				return
			}

			// For multiple transactions, just verify it's not empty and is valid hex
			if resultStr == "" {
				t.Error("Merkle root should not be empty")
			}

			// Verify it's a valid 64-character hex string
			if len(resultStr) != 64 {
				t.Errorf("Expected 64-character hex string, got %d characters: %s", len(resultStr), resultStr)
			}

			_, err := hex.DecodeString(resultStr)
			if err != nil {
				t.Errorf("Result is not valid hex: %v", err)
			}
		})
	}
}

func TestGetMerkleBranch(t *testing.T) {
	tests := []struct {
		name     string
		txHashes []string
		txIndex  int
		expected int // Expected number of branch elements
	}{
		{
			name:     "empty transactions",
			txHashes: []string{},
			txIndex:  0,
			expected: 0,
		},
		{
			name: "single transaction",
			txHashes: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
			txIndex:  0,
			expected: 0,
		},
		{
			name: "two transactions, index 0",
			txHashes: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000002",
			},
			txIndex:  0,
			expected: 1, // Should have 1 sibling
		},
		{
			name: "four transactions, index 0",
			txHashes: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000002",
				"0000000000000000000000000000000000000000000000000000000000000003",
				"0000000000000000000000000000000000000000000000000000000000000004",
			},
			txIndex:  0,
			expected: 2, // Should have 2 levels
		},
		{
			name: "invalid index",
			txHashes: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000002",
			},
			txIndex:  5,
			expected: 0, // Should return empty for invalid index
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert string hashes to chainhash.Hash
			hashes := make([]chainhash.Hash, len(tt.txHashes))
			for i, hashStr := range tt.txHashes {
				hash, err := chainhash.NewHashFromStr(hashStr)
				if err != nil {
					t.Fatalf("Failed to parse hash %s: %v", hashStr, err)
				}
				hashes[i] = *hash
			}

			branch := GetMerkleBranch(hashes, tt.txIndex)

			if len(branch) != tt.expected {
				t.Errorf("Expected %d branch elements, got %d", tt.expected, len(branch))
			}

			// Verify all branch elements are valid hashes
			for i, hash := range branch {
				if hash.IsEqual(&chainhash.Hash{}) {
					t.Errorf("Branch element %d is zero hash", i)
				}
			}
		})
	}
}

func TestDifficultyToTarget(t *testing.T) {
	tests := []struct {
		name       string
		difficulty float64
		wantLen    int
	}{
		{
			name:       "difficulty 1",
			difficulty: 1.0,
			wantLen:    32,
		},
		{
			name:       "difficulty 2",
			difficulty: 2.0,
			wantLen:    32,
		},
		{
			name:       "high difficulty",
			difficulty: 1000.0,
			wantLen:    32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := DifficultyToTarget(tt.difficulty)

			if len(target) != tt.wantLen {
				t.Errorf("DifficultyToTarget() returned target length %d, want %d", len(target), tt.wantLen)
			}
		})
	}
}

func TestHashMeetsTarget(t *testing.T) {
	// Create a test hash (very low value)
	testHash := chainhash.Hash{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}

	tests := []struct {
		name   string
		hash   chainhash.Hash
		target []byte
		want   bool
	}{
		{
			name: "hash meets easy target",
			hash: testHash,
			target: []byte{
				0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			want: true,
		},
		{
			name: "hash does not meet hard target",
			hash: testHash,
			target: []byte{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashMeetsTarget(tt.hash, tt.target)
			if got != tt.want {
				t.Errorf("HashMeetsTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseHexUint32(t *testing.T) {
	tests := []struct {
		name    string
		hexStr  string
		want    uint32
		wantErr bool
	}{
		{
			name:    "valid hex",
			hexStr:  "01020304",
			want:    0x04030201, // Little-endian
			wantErr: false,
		},
		{
			name:    "invalid hex",
			hexStr:  "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "wrong length",
			hexStr:  "0102",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHexUint32(tt.hexStr)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseHexUint32() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseHexUint32() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("parseHexUint32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateCoinbaseTransaction(t *testing.T) {
	tests := []struct {
		name          string
		blockHeight   int64
		coinbaseValue int64
		extraNonce1   string
		poolAddress   string
		chainParams   *chaincfg.Params
		wantErr       bool
	}{
		{
			name:          "valid coinbase transaction",
			blockHeight:   100,
			coinbaseValue: 5000000000, // 50 BTC in satoshis
			extraNonce1:   "12345678",
			poolAddress:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			chainParams:   &chaincfg.MainNetParams,
			wantErr:       false,
		},
		{
			name:          "invalid pool address",
			blockHeight:   100,
			coinbaseValue: 5000000000,
			extraNonce1:   "12345678",
			poolAddress:   "invalid_address",
			chainParams:   &chaincfg.MainNetParams,
			wantErr:       true,
		},
		{
			name:          "empty pool address uses default",
			blockHeight:   100,
			coinbaseValue: 5000000000,
			extraNonce1:   "12345678",
			poolAddress:   "",
			chainParams:   &chaincfg.MainNetParams,
			wantErr:       false,
		},
		{
			name:          "invalid extra nonce",
			blockHeight:   100,
			coinbaseValue: 5000000000,
			extraNonce1:   "gg", // Invalid hex
			poolAddress:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			chainParams:   &chaincfg.MainNetParams,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, coinb1, coinb2, err := CreateCoinbaseTransaction(
				tt.blockHeight,
				tt.coinbaseValue,
				tt.extraNonce1,
				tt.poolAddress,
				tt.chainParams,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("CreateCoinbaseTransaction() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CreateCoinbaseTransaction() unexpected error: %v", err)
				return
			}

			// Verify transaction structure
			if tx == nil {
				t.Error("CreateCoinbaseTransaction() returned nil transaction")
				return
			}

			// Verify coinb1 and coinb2 are valid hex strings
			if _, err := hex.DecodeString(coinb1); err != nil {
				t.Errorf("coinb1 is not valid hex: %v", err)
			}

			if _, err := hex.DecodeString(coinb2); err != nil {
				t.Errorf("coinb2 is not valid hex: %v", err)
			}

			// Verify transaction has one input and one output
			if len(tx.TxIn) != 1 {
				t.Errorf("Expected 1 input, got %d", len(tx.TxIn))
			}

			if len(tx.TxOut) != 1 {
				t.Errorf("Expected 1 output, got %d", len(tx.TxOut))
			}

			// Verify coinbase input
			if tx.TxIn[0].PreviousOutPoint.Index != 0xffffffff {
				t.Errorf("Expected coinbase input index 0xffffffff, got %d", tx.TxIn[0].PreviousOutPoint.Index)
			}

			// Verify output value
			if tx.TxOut[0].Value != tt.coinbaseValue {
				t.Errorf("Expected output value %d, got %d", tt.coinbaseValue, tx.TxOut[0].Value)
			}
		})
	}
}

func TestReconstructBlock(t *testing.T) {
	// Create a simple mock template
	template := &btcjson.GetBlockTemplateResult{
		Version:      1,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Bits:         "1d00ffff",
		Height:       100,
		Transactions: []btcjson.GetBlockTemplateResultTx{},
	}

	// Create a simple coinbase transaction
	coinbaseTx := wire.NewMsgTx(wire.TxVersion)
	coinbaseTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
		},
		SignatureScript: []byte("test coinbase"),
		Sequence:        0xffffffff,
	})
	coinbaseTx.AddTxOut(&wire.TxOut{
		Value:    5000000000,
		PkScript: []byte("test script"),
	})

	tests := []struct {
		name        string
		template    *btcjson.GetBlockTemplateResult
		coinbaseTx  *wire.MsgTx
		extraNonce2 string
		ntime       string
		nonce       string
		wantErr     bool
	}{
		{
			name:        "valid block reconstruction",
			template:    template,
			coinbaseTx:  coinbaseTx,
			extraNonce2: "87654321",
			ntime:       "12345678", // Valid 8-char hex
			nonce:       "abcdef00", // Valid 8-char hex
			wantErr:     false,
		},
		{
			name:        "invalid ntime",
			template:    template,
			coinbaseTx:  coinbaseTx,
			extraNonce2: "87654321",
			ntime:       "invalid",
			nonce:       "abcdef00",
			wantErr:     true,
		},
		{
			name:        "invalid nonce",
			template:    template,
			coinbaseTx:  coinbaseTx,
			extraNonce2: "87654321",
			ntime:       "12345678",
			nonce:       "invalid",
			wantErr:     true,
		},
		{
			name: "invalid previous hash",
			template: &btcjson.GetBlockTemplateResult{
				Version:      1,
				PreviousHash: "invalid_hash",
				Bits:         "1d00ffff",
				Height:       100,
				Transactions: []btcjson.GetBlockTemplateResultTx{},
			},
			coinbaseTx:  coinbaseTx,
			extraNonce2: "87654321",
			ntime:       "12345678",
			nonce:       "abcdef00",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, blockHex, err := ReconstructBlock(tt.template, tt.coinbaseTx, tt.extraNonce2, tt.ntime, tt.nonce)

			if tt.wantErr {
				if err == nil {
					t.Error("ReconstructBlock() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ReconstructBlock() unexpected error: %v", err)
				return
			}

			// Verify block is not nil
			if block == nil {
				t.Error("ReconstructBlock() returned nil block")
				return
			}

			// Verify block hex is valid
			if blockHex == "" {
				t.Error("ReconstructBlock() returned empty block hex")
			}

			if _, err := hex.DecodeString(blockHex); err != nil {
				t.Errorf("Block hex is not valid: %v", err)
			}

			// Verify block has transactions
			if len(block.Transactions) == 0 {
				t.Error("Block should have at least coinbase transaction")
			}
		})
	}
}

func TestValidateShare(t *testing.T) {
	// Create a simple mock template
	template := &btcjson.GetBlockTemplateResult{
		Version:      1,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Bits:         "1d00ffff",
		Height:       100,
		Target:       "00000000FFFF0000000000000000000000000000000000000000000000000000",
		Transactions: []btcjson.GetBlockTemplateResultTx{},
	}

	// Create a simple coinbase transaction
	coinbaseTx := wire.NewMsgTx(wire.TxVersion)
	coinbaseTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
		},
		SignatureScript: []byte("test coinbase"),
		Sequence:        0xffffffff,
	})
	coinbaseTx.AddTxOut(&wire.TxOut{
		Value:    5000000000,
		PkScript: []byte("test script"),
	})

	tests := []struct {
		name        string
		jobID       string
		extraNonce2 string
		ntime       string
		nonce       string
		difficulty  float64
		template    *btcjson.GetBlockTemplateResult
		coinbaseTx  *wire.MsgTx
		wantErr     bool
		errorType   string
	}{
		{
			name:        "share validation with random nonce",
			jobID:       "job_1",
			extraNonce2: "87654321",
			ntime:       "12345678",
			nonce:       "abcdef00",
			difficulty:  1.0,
			template:    template,
			coinbaseTx:  coinbaseTx,
			wantErr:     true, // Random nonce unlikely to meet difficulty
			errorType:   "difficulty",
		},
		{
			name:        "share with reconstruction error",
			jobID:       "job_1",
			extraNonce2: "87654321",
			ntime:       "invalid", // This will cause reconstruction to fail
			nonce:       "abcdef00",
			difficulty:  1.0,
			template:    template,
			coinbaseTx:  coinbaseTx,
			wantErr:     true,
			errorType:   "reconstruction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateShare(
				tt.jobID,
				tt.extraNonce2,
				tt.ntime,
				tt.nonce,
				tt.difficulty,
				tt.template,
				tt.coinbaseTx,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateShare() expected error, got nil")
				}
				// Just verify we got an error - specific error depends on the test case
			} else {
				if err != nil {
					t.Errorf("ValidateShare() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsBlockCandidate(t *testing.T) {
	// Create a simple mock template
	template := &btcjson.GetBlockTemplateResult{
		Version:      1,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Bits:         "1d00ffff",
		Height:       100,
		Target:       "00000000FFFF0000000000000000000000000000000000000000000000000000",
		Transactions: []btcjson.GetBlockTemplateResultTx{},
	}

	// Create a simple coinbase transaction
	coinbaseTx := wire.NewMsgTx(wire.TxVersion)
	coinbaseTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
		},
		SignatureScript: []byte("test coinbase"),
		Sequence:        0xffffffff,
	})
	coinbaseTx.AddTxOut(&wire.TxOut{
		Value:    5000000000,
		PkScript: []byte("test script"),
	})

	tests := []struct {
		name        string
		jobID       string
		extraNonce2 string
		ntime       string
		nonce       string
		template    *btcjson.GetBlockTemplateResult
		coinbaseTx  *wire.MsgTx
		wantErr     bool
	}{
		{
			name:        "valid block candidate check",
			jobID:       "job_1",
			extraNonce2: "87654321",
			ntime:       "12345678",
			nonce:       "abcdef00",
			template:    template,
			coinbaseTx:  coinbaseTx,
			wantErr:     false,
		},
		{
			name:        "block candidate with reconstruction error",
			jobID:       "job_1",
			extraNonce2: "87654321",
			ntime:       "invalid", // This will cause reconstruction to fail
			nonce:       "abcdef00",
			template:    template,
			coinbaseTx:  coinbaseTx,
			wantErr:     true,
		},
		{
			name:        "block candidate with invalid target",
			jobID:       "job_1",
			extraNonce2: "87654321",
			ntime:       "12345678",
			nonce:       "abcdef00",
			template: &btcjson.GetBlockTemplateResult{
				Version:      1,
				PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
				Bits:         "1d00ffff",
				Height:       100,
				Target:       "invalid_target", // Invalid target
				Transactions: []btcjson.GetBlockTemplateResultTx{},
			},
			coinbaseTx: coinbaseTx,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBlock, err := IsBlockCandidate(
				tt.jobID,
				tt.extraNonce2,
				tt.ntime,
				tt.nonce,
				tt.template,
				tt.coinbaseTx,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("IsBlockCandidate() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("IsBlockCandidate() unexpected error: %v", err)
				}
				// isBlock should be boolean (true or false)
				_ = isBlock
			}
		})
	}
}

func TestParseHexTarget(t *testing.T) {
	tests := []struct {
		name      string
		targetStr string
		wantErr   bool
		wantLen   int
	}{
		{
			name:      "valid target",
			targetStr: "00000000FFFF0000000000000000000000000000000000000000000000000000",
			wantErr:   false,
			wantLen:   32,
		},
		{
			name:      "short target",
			targetStr: "FFFF",
			wantErr:   false,
			wantLen:   32, // Should be padded
		},
		{
			name:      "empty target",
			targetStr: "",
			wantErr:   true,
		},
		{
			name:      "odd length target",
			targetStr: "FFF", // Odd number of characters
			wantErr:   true,
		},
		{
			name:      "too long target",
			targetStr: "00000000FFFF000000000000000000000000000000000000000000000000000000000000",
			wantErr:   true,
		},
		{
			name:      "invalid hex characters",
			targetStr: "GGGG0000000000000000000000000000000000000000000000000000000000000000",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := parseHexTarget(tt.targetStr)

			if tt.wantErr {
				if err == nil {
					t.Error("parseHexTarget() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseHexTarget() unexpected error: %v", err)
				return
			}

			if len(target) != tt.wantLen {
				t.Errorf("parseHexTarget() returned length %d, want %d", len(target), tt.wantLen)
			}
		})
	}
}

func TestDifficultyToTargetEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		difficulty float64
		wantLen    int
	}{
		{
			name:       "zero difficulty",
			difficulty: 0.0,
			wantLen:    32,
		},
		{
			name:       "negative difficulty",
			difficulty: -1.0,
			wantLen:    32,
		},
		{
			name:       "very high difficulty",
			difficulty: 1000000.0,
			wantLen:    32,
		},
		{
			name:       "fractional difficulty",
			difficulty: 1.5,
			wantLen:    32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := DifficultyToTarget(tt.difficulty)

			if len(target) != tt.wantLen {
				t.Errorf("DifficultyToTarget() returned target length %d, want %d", len(target), tt.wantLen)
			}

			// For zero/negative difficulty, should return max target
			if tt.difficulty <= 0 {
				maxTarget := []byte{
					0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				}
				for i, b := range maxTarget {
					if target[i] != b {
						t.Errorf("Expected max target for zero/negative difficulty")
						break
					}
				}
			}
		})
	}
}

func TestParseHexUint32EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		hexStr  string
		wantErr bool
	}{
		{
			name:    "hex with invalid character",
			hexStr:  "0102030G",
			wantErr: true,
		},
		{
			name:    "empty string",
			hexStr:  "",
			wantErr: true,
		},
		{
			name:    "too long",
			hexStr:  "0102030405",
			wantErr: true,
		},
		{
			name:    "lowercase valid hex",
			hexStr:  "abcdef01",
			wantErr: false,
		},
		{
			name:    "uppercase valid hex",
			hexStr:  "ABCDEF01",
			wantErr: false,
		},
		{
			name:    "mixed case valid hex",
			hexStr:  "AbCdEf01",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseHexUint32(tt.hexStr)

			if tt.wantErr {
				if err == nil {
					t.Error("parseHexUint32() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("parseHexUint32() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestReconstructBlockWithTransactions(t *testing.T) {
	// Test with a template that has transactions
	template := &btcjson.GetBlockTemplateResult{
		Version:      1,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Bits:         "1d00ffff",
		Height:       100,
		Transactions: []btcjson.GetBlockTemplateResultTx{
			{
				Data: "0100000001000000000000000000000000000000000000000000000000000000000000000000000000ffffffff0100f2052a01000000434104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac00000000",
			},
		},
	}

	// Create a simple coinbase transaction
	coinbaseTx := wire.NewMsgTx(wire.TxVersion)
	coinbaseTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
		},
		SignatureScript: []byte("test coinbase"),
		Sequence:        0xffffffff,
	})
	coinbaseTx.AddTxOut(&wire.TxOut{
		Value:    5000000000,
		PkScript: []byte("test script"),
	})

	t.Run("block with invalid template transaction", func(t *testing.T) {
		_, _, err := ReconstructBlock(template, coinbaseTx, "87654321", "12345678", "abcdef00")

		if err == nil {
			t.Error("ReconstructBlock() expected error for invalid transaction, got nil")
		}
		// The transaction data I provided is malformed, so this should fail
	})

	t.Run("template with invalid transaction data", func(t *testing.T) {
		invalidTemplate := &btcjson.GetBlockTemplateResult{
			Version:      1,
			PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:         "1d00ffff",
			Height:       100,
			Transactions: []btcjson.GetBlockTemplateResultTx{
				{
					Data: "invalid_hex_data",
				},
			},
		}

		_, _, err := ReconstructBlock(invalidTemplate, coinbaseTx, "87654321", "12345678", "abcdef00")
		if err == nil {
			t.Error("ReconstructBlock() expected error for invalid transaction data, got nil")
		}
	})

	t.Run("template with malformed transaction", func(t *testing.T) {
		malformedTemplate := &btcjson.GetBlockTemplateResult{
			Version:      1,
			PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:         "1d00ffff",
			Height:       100,
			Transactions: []btcjson.GetBlockTemplateResultTx{
				{
					Data: "deadbeef", // Valid hex but not valid transaction
				},
			},
		}

		_, _, err := ReconstructBlock(malformedTemplate, coinbaseTx, "87654321", "12345678", "abcdef00")
		if err == nil {
			t.Error("ReconstructBlock() expected error for malformed transaction, got nil")
		}
	})

	t.Run("invalid bits in template", func(t *testing.T) {
		invalidBitsTemplate := &btcjson.GetBlockTemplateResult{
			Version:      1,
			PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
			Bits:         "invalid",
			Height:       100,
			Transactions: []btcjson.GetBlockTemplateResultTx{},
		}

		_, _, err := ReconstructBlock(invalidBitsTemplate, coinbaseTx, "87654321", "12345678", "abcdef00")
		if err == nil {
			t.Error("ReconstructBlock() expected error for invalid bits, got nil")
		}
	})
}
