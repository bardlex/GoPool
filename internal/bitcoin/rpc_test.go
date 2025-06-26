package bitcoin

import (
	"context"
	"testing"
	"time"
)

func TestNewRPCClient(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		username string
		password string
		wantErr  bool
	}{
		{
			name:     "valid connection parameters",
			host:     "localhost",
			port:     8332,
			username: "user",
			password: "pass",
			wantErr:  false, // Will fail to connect but client creation should succeed
		},
		{
			name:     "invalid port zero",
			host:     "localhost",
			port:     0,
			username: "user",
			password: "pass",
			wantErr:  false, // Client creation still succeeds, connection would fail later
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewRPCClient(tt.host, tt.port, tt.username, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Error("NewRPCClient() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewRPCClient() unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("NewRPCClient() returned nil client")
				return
			}

			// Test Close method
			client.Close()
		})
	}
}

func TestRPCClientMethodSignatures(t *testing.T) {
	// Test that RPC client methods exist and have correct signatures
	// These won't actually work without a Bitcoin Core instance, but we can test creation
	client, err := NewRPCClient("localhost", 8332, "user", "pass")
	if err != nil {
		t.Fatalf("Failed to create RPC client: %v", err)
	}
	defer client.Close()

	// Test that methods exist (will fail with connection error, but that's expected)
	t.Run("GetBlockTemplate method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetBlockTemplate(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("GetBlockCount method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetBlockCount(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("GetBestBlockHash method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetBestBlockHash(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("Ping method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = client.Ping(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("GetBlock method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetBlock(ctx, "0000000000000000000000000000000000000000000000000000000000000000")
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("GetNetworkInfo method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetNetworkInfo(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("GetDifficulty method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetDifficulty(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("GetMiningInfo method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetMiningInfo(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})

	t.Run("GetBlockchainInfo method exists", func(_ *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _ = client.GetBlockchainInfo(ctx)
		// Expect connection error, not method not found - any error is fine for this test
	})
}

func TestRPCClientCryptoMethods(t *testing.T) {
	client, err := NewRPCClient("localhost", 8332, "user", "pass")
	if err != nil {
		t.Fatalf("Failed to create RPC client: %v", err)
	}
	defer client.Close()

	t.Run("CreateCoinbaseTransaction", func(t *testing.T) {
		ctx := context.Background()
		tx, coinb1, coinb2, err := client.CreateCoinbaseTransaction(
			ctx,
			100,                                  // blockHeight
			5000000000,                           // coinbaseValue
			"12345678",                           // extraNonce1
			"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // poolAddress
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
		ctx := context.Background()
		txHashes := []string{
			"0000000000000000000000000000000000000000000000000000000000000001",
			"0000000000000000000000000000000000000000000000000000000000000002",
		}

		root, err := client.CalculateMerkleRoot(ctx, txHashes)
		if err != nil {
			t.Errorf("CalculateMerkleRoot() unexpected error: %v", err)
			return
		}

		if root == "" {
			t.Error("CalculateMerkleRoot() returned empty root")
		}

		if len(root) != 64 {
			t.Errorf("CalculateMerkleRoot() returned root of length %d, expected 64", len(root))
		}
	})

	t.Run("CalculateMerkleRoot empty input", func(t *testing.T) {
		ctx := context.Background()
		_, _ = client.CalculateMerkleRoot(ctx, []string{})
		if err == nil {
			t.Error("CalculateMerkleRoot() expected error for empty input, got nil")
		}
	})

	t.Run("CalculateMerkleRoot invalid hash", func(t *testing.T) {
		ctx := context.Background()
		txHashes := []string{"invalid_hash"}
		_, _ = client.CalculateMerkleRoot(ctx, txHashes)
		if err == nil {
			t.Error("CalculateMerkleRoot() expected error for invalid hash, got nil")
		}
	})

	t.Run("GetMerkleBranch", func(t *testing.T) {
		ctx := context.Background()
		txHashes := []string{
			"0000000000000000000000000000000000000000000000000000000000000001",
			"0000000000000000000000000000000000000000000000000000000000000002",
		}

		branch, err := client.GetMerkleBranch(ctx, txHashes)
		if err != nil {
			t.Errorf("GetMerkleBranch() unexpected error: %v", err)
			return
		}

		if len(branch) != 1 {
			t.Errorf("GetMerkleBranch() returned branch of length %d, expected 1", len(branch))
		}
	})

	t.Run("GetMerkleBranch single transaction", func(t *testing.T) {
		ctx := context.Background()
		txHashes := []string{
			"0000000000000000000000000000000000000000000000000000000000000001",
		}

		branch, err := client.GetMerkleBranch(ctx, txHashes)
		if err != nil {
			t.Errorf("GetMerkleBranch() unexpected error: %v", err)
			return
		}

		if len(branch) != 0 {
			t.Errorf("GetMerkleBranch() returned branch of length %d, expected 0 for single tx", len(branch))
		}
	})

	t.Run("GetMerkleBranch invalid hash", func(t *testing.T) {
		ctx := context.Background()
		txHashes := []string{"invalid_hash", "another_invalid"}
		_, _ = client.GetMerkleBranch(ctx, txHashes)
		if err == nil {
			t.Error("GetMerkleBranch() expected error for invalid hash, got nil")
		}
	})
}

func TestRPCClientValidateAddress(t *testing.T) {
	client, err := NewRPCClient("localhost", 8332, "user", "pass")
	if err != nil {
		t.Fatalf("Failed to create RPC client: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name    string
		address string
		want    bool
		wantErr bool
	}{
		{
			name:    "valid mainnet address",
			address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			want:    false, // Will return false without connection, but won't error
			wantErr: true,  // Expect connection error
		},
		{
			name:    "invalid address",
			address: "invalid_address",
			want:    false,
			wantErr: false, // Should return false for invalid address without connection error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			valid, err := client.ValidateAddress(ctx, tt.address)

			if tt.wantErr {
				// For valid addresses, we expect a connection error
				if err == nil {
					t.Error("ValidateAddress() expected connection error, got nil")
				}
			} else {
				// For invalid addresses, should return false without error
				if err != nil {
					t.Errorf("ValidateAddress() unexpected error: %v", err)
				}
				if valid != tt.want {
					t.Errorf("ValidateAddress() = %v, want %v", valid, tt.want)
				}
			}
		})
	}
}

func TestRPCClientSubmitBlock(t *testing.T) {
	client, err := NewRPCClient("localhost", 8332, "user", "pass")
	if err != nil {
		t.Fatalf("Failed to create RPC client: %v", err)
	}
	defer client.Close()

	t.Run("SubmitBlock with invalid hex", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = client.SubmitBlock(ctx, "invalid_hex")
		if err == nil {
			t.Error("SubmitBlock() expected error for invalid hex, got nil")
		}
	})

	t.Run("SubmitBlock with valid hex but invalid block", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = client.SubmitBlock(ctx, "deadbeef") // Valid hex but not a valid block
		if err == nil {
			t.Error("SubmitBlock() expected error for invalid block, got nil")
		}
	})
}
