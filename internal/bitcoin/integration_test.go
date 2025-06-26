package bitcoin

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestIntegration_MockRPCClient(t *testing.T) {
	ctx := context.Background()
	mock := NewMockRPCClient()

	t.Run("GetBlockTemplate", func(t *testing.T) {
		template, err := mock.GetBlockTemplate(ctx)
		if err != nil {
			t.Errorf("GetBlockTemplate() unexpected error: %v", err)
			return
		}
		if template == nil {
			t.Error("GetBlockTemplate() returned nil template")
			return
		}
		if template.Height != 100 {
			t.Errorf("Expected height 100, got %d", template.Height)
		}
	})

	t.Run("GetBlockCount", func(t *testing.T) {
		count, err := mock.GetBlockCount(ctx)
		if err != nil {
			t.Errorf("GetBlockCount() unexpected error: %v", err)
			return
		}
		if count != 100 {
			t.Errorf("Expected count 100, got %d", count)
		}
	})

	t.Run("CreateCoinbaseTransaction", func(t *testing.T) {
		tx, coinb1, coinb2, err := mock.CreateCoinbaseTransaction(
			ctx, 100, 5000000000, "12345678", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		)
		if err != nil {
			t.Errorf("CreateCoinbaseTransaction() unexpected error: %v", err)
			return
		}
		if tx == nil {
			t.Error("CreateCoinbaseTransaction() returned nil transaction")
		}
		if coinb1 == "" {
			t.Error("CreateCoinbaseTransaction() returned empty coinb1")
		}
		if coinb2 == "" {
			t.Error("CreateCoinbaseTransaction() returned empty coinb2")
		}
	})

	t.Run("CalculateMerkleRoot", func(t *testing.T) {
		txHashes := []string{
			"0000000000000000000000000000000000000000000000000000000000000001",
			"0000000000000000000000000000000000000000000000000000000000000002",
		}
		root, err := mock.CalculateMerkleRoot(ctx, txHashes)
		if err != nil {
			t.Errorf("CalculateMerkleRoot() unexpected error: %v", err)
			return
		}
		if root == "" {
			t.Error("CalculateMerkleRoot() returned empty root")
		}
		if len(root) != 64 {
			t.Errorf("Expected 64-character hex string, got %d characters", len(root))
		}
	})

	t.Run("Error handling", func(t *testing.T) {
		mock.ShouldError = true
		mock.ErrorMsg = "test error"

		_, err := mock.GetBlockTemplate(ctx)
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err.Error() != "test error" {
			t.Errorf("Expected 'test error', got '%s'", err.Error())
		}
	})
}

func TestIntegration_CryptoService_WithRealData(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

	t.Run("End-to-end coinbase transaction creation", func(t *testing.T) {
		// Create a coinbase transaction
		tx, coinb1, coinb2, err := service.CreateCoinbaseTransaction(
			100,                                  // block height
			5000000000,                           // 50 BTC reward
			"12345678",                           // extra nonce 1
			"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // genesis address
			nil,                                  // use service default params
		)

		if err != nil {
			t.Fatalf("CreateCoinbaseTransaction failed: %v", err)
		}

		// Verify transaction structure
		if len(tx.TxIn) != 1 {
			t.Errorf("Expected 1 input, got %d", len(tx.TxIn))
		}
		if len(tx.TxOut) != 1 {
			t.Errorf("Expected 1 output, got %d", len(tx.TxOut))
		}
		if tx.TxOut[0].Value != 5000000000 {
			t.Errorf("Expected output value 5000000000, got %d", tx.TxOut[0].Value)
		}

		// Verify coinbase splits are not empty
		if coinb1 == "" {
			t.Error("Coinb1 should not be empty")
		}
		if coinb2 == "" {
			t.Error("Coinb2 should not be empty")
		}
	})

	t.Run("Difficulty calculations", func(t *testing.T) {
		difficulties := []float64{1.0, 2.0, 1000.0, 0.5}

		for _, diff := range difficulties {
			target := service.DifficultyToTarget(diff)

			if len(target) != 32 {
				t.Errorf("Target should be 32 bytes, got %d", len(target))
			}

			// Higher difficulty should result in smaller target (more leading zeros)
			if diff > 1.0 {
				maxTarget := service.DifficultyToTarget(1.0)
				// Compare first few bytes to ensure target is smaller
				targetIsSmaller := false
				for i := 0; i < 8; i++ {
					if target[i] < maxTarget[i] {
						targetIsSmaller = true
						break
					} else if target[i] > maxTarget[i] {
						break
					}
				}
				if !targetIsSmaller {
					t.Errorf("Higher difficulty %f should produce smaller target than difficulty 1.0", diff)
				}
			}
		}
	})
}

// Test that MockRPCClient implements RPCInterface
func TestMockRPCClient_ImplementsInterface(_ *testing.T) {
	var _ RPCInterface = (*MockRPCClient)(nil)
}
