package bitcoin

import (
	"testing"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestNewCryptoService(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

	if service == nil {
		t.Error("NewCryptoService() returned nil")
		return
	}

	if service.chainParams != &chaincfg.MainNetParams {
		t.Error("NewCryptoService() did not set chainParams correctly")
	}
}

func TestCryptoService_CreateCoinbaseTransaction(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

	tx, coinb1, coinb2, err := service.CreateCoinbaseTransaction(
		100,
		5000000000,
		"12345678",
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		nil, // Use service default chain params
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
}

func TestCryptoService_CreateCoinbaseTransactionWithCustomParams(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

	// Test with custom chain params (testnet)
	tx, _, _, err := service.CreateCoinbaseTransaction(
		100,
		5000000000,
		"12345678",
		"tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx", // testnet address
		&chaincfg.TestNet3Params,
	)

	if err != nil {
		t.Errorf("CreateCoinbaseTransaction() with custom params unexpected error: %v", err)
		return
	}

	if tx == nil {
		t.Error("CreateCoinbaseTransaction() with custom params returned nil transaction")
	}
}

func TestCryptoService_DifficultyToTarget(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

	tests := []struct {
		name       string
		difficulty float64
		wantLen    int
	}{
		{"difficulty 1", 1.0, 32},
		{"difficulty 2", 2.0, 32},
		{"high difficulty", 1000.0, 32},
		{"zero difficulty", 0.0, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := service.DifficultyToTarget(tt.difficulty)

			if len(target) != tt.wantLen {
				t.Errorf("DifficultyToTarget() returned target length %d, want %d", len(target), tt.wantLen)
			}
		})
	}
}

func TestCryptoService_HashMeetsTarget(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

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
			got := service.HashMeetsTarget(tt.hash, tt.target)
			if got != tt.want {
				t.Errorf("HashMeetsTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCryptoService_ReconstructBlock(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

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

	block, blockHex, err := service.ReconstructBlock(template, coinbaseTx, "87654321", "12345678", "abcdef00")

	if err != nil {
		t.Errorf("ReconstructBlock() unexpected error: %v", err)
		return
	}

	if block == nil {
		t.Error("ReconstructBlock() returned nil block")
	}

	if blockHex == "" {
		t.Error("ReconstructBlock() returned empty block hex")
	}
}

func TestCryptoService_ValidateShare(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

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

	// This should fail since random nonce unlikely to meet difficulty
	err := service.ValidateShare("job_1", "87654321", "12345678", "abcdef00", 1.0, template, coinbaseTx)

	if err == nil {
		t.Error("ValidateShare() expected error for random nonce, got nil")
	}
}

func TestCryptoService_IsBlockCandidate(t *testing.T) {
	service := NewCryptoService(&chaincfg.MainNetParams)

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

	isBlock, err := service.IsBlockCandidate("job_1", "87654321", "12345678", "abcdef00", template, coinbaseTx)

	if err != nil {
		t.Errorf("IsBlockCandidate() unexpected error: %v", err)
		return
	}

	// Random nonce should not meet network difficulty
	if isBlock {
		t.Error("IsBlockCandidate() should return false for random nonce")
	}
}

// Test that CryptoService implements CryptoInterface
func TestCryptoService_ImplementsInterface(_ *testing.T) {
	var _ CryptoInterface = (*CryptoService)(nil)
}
