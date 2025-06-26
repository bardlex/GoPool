package bitcoin

import (
	"testing"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// BenchmarkCreateCoinbaseTransaction benchmarks the coinbase transaction creation hot path
func BenchmarkCreateCoinbaseTransaction(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_, _, _, err := CreateCoinbaseTransaction(
			100,
			5000000000,
			"12345678",
			"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			&chaincfg.MainNetParams,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCalculateMerkleRoot benchmarks merkle root calculation for different block sizes
func BenchmarkCalculateMerkleRoot(b *testing.B) {
	tests := []struct {
		name  string
		count int
	}{
		{"Small_Block_100_Tx", 100},
		{"Medium_Block_1000_Tx", 1000},
		{"Large_Block_4000_Tx", 4000},
		{"Massive_Block_8000_Tx", 8000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create test transaction hashes
			txHashes := make([]chainhash.Hash, tt.count)
			for i := 0; i < tt.count; i++ {
				// Create unique hash for each transaction
				hash := chainhash.DoubleHashH([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
				txHashes[i] = hash
			}

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = CalculateMerkleRoot(txHashes)
			}
		})
	}
}

// BenchmarkGetMerkleBranch benchmarks merkle branch generation for different block sizes
func BenchmarkGetMerkleBranch(b *testing.B) {
	tests := []struct {
		name  string
		count int
	}{
		{"Small_Block_100_Tx", 100},
		{"Medium_Block_1000_Tx", 1000},
		{"Large_Block_4000_Tx", 4000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create test transaction hashes
			txHashes := make([]chainhash.Hash, tt.count)
			for i := 0; i < tt.count; i++ {
				hash := chainhash.DoubleHashH([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
				txHashes[i] = hash
			}

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = GetMerkleBranch(txHashes, 0) // Branch for coinbase (index 0)
			}
		})
	}
}

// BenchmarkDifficultyToTarget benchmarks target calculation from difficulty
func BenchmarkDifficultyToTarget(b *testing.B) {
	tests := []struct {
		name       string
		difficulty float64
	}{
		{"Difficulty_1", 1.0},
		{"Difficulty_1000", 1000.0},
		{"Difficulty_1M", 1000000.0},
		{"Difficulty_Fractional", 0.5},
		{"Difficulty_Very_High", 50000000000.0}, // Realistic high difficulty
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_ = DifficultyToTarget(tt.difficulty)
			}
		})
	}
}

// BenchmarkHashMeetsTarget benchmarks hash vs target comparison
func BenchmarkHashMeetsTarget(b *testing.B) {
	// Create test hash
	testHash := chainhash.Hash{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}

	// Create test target
	target := []byte{
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = HashMeetsTarget(testHash, target)
	}
}

// BenchmarkReconstructBlock benchmarks block reconstruction from components
func BenchmarkReconstructBlock(b *testing.B) {
	// Create a mock template
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

	// Create a coinbase transaction
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

	b.ReportAllocs()

	for b.Loop() {
		_, _, err := ReconstructBlock(template, coinbaseTx, "87654321", "12345678", "abcdef00")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidateShare benchmarks share validation performance
func BenchmarkValidateShare(b *testing.B) {
	// Create a mock template
	template := &btcjson.GetBlockTemplateResult{
		Version:      1,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Bits:         "1d00ffff",
		Height:       100,
		Target:       "00000000FFFF0000000000000000000000000000000000000000000000000000",
		Transactions: []btcjson.GetBlockTemplateResultTx{},
	}

	// Create a coinbase transaction
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

	b.ReportAllocs()

	for b.Loop() {
		// Note: This will likely fail validation due to random nonce, but we're benchmarking the process
		_ = ValidateShare("job_1", "87654321", "12345678", "abcdef00", 1.0, template, coinbaseTx)
	}
}

// BenchmarkCryptoService benchmarks the service wrapper vs direct function calls
func BenchmarkCryptoService(b *testing.B) {
	service := NewCryptoService(&chaincfg.MainNetParams)

	b.Run("Service_CreateCoinbaseTransaction", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, _, _, err := service.CreateCoinbaseTransaction(
				100,
				5000000000,
				"12345678",
				"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				nil,
			)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Service_DifficultyToTarget", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = service.DifficultyToTarget(1000.0)
		}
	})
}

// BenchmarkSyncPoolEfficiency compares performance with and without pools
func BenchmarkSyncPoolEfficiency(b *testing.B) {
	b.Run("With_Pools", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = DifficultyToTarget(1000.0)
		}
	})

	// This would be a comparison against a version without pools
	// but since we've already optimized everything, we'll just test the current version
	b.Run("Pool_Buffer_Reuse", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			buf := getBuffer()
			putBuffer(buf)
		}
	})

	b.Run("Pool_BigInt_Reuse", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			bi := getBigInt()
			putBigInt(bi)
		}
	})
}
