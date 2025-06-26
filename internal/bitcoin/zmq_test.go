package bitcoin

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewZMQNotifier(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid endpoint",
			endpoint: "tcp://localhost:28332",
			wantErr:  false,
		},
		{
			name:     "empty endpoint",
			endpoint: "",
			wantErr:  false, // ZMQ allows empty endpoint
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewZMQNotifier(tt.endpoint, logger)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewZMQNotifier() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewZMQNotifier() unexpected error: %v", err)
				return
			}

			if notifier == nil {
				t.Errorf("NewZMQNotifier() returned nil notifier")
				return
			}

			if notifier.endpoint != tt.endpoint {
				t.Errorf("NewZMQNotifier() endpoint = %v, want %v", notifier.endpoint, tt.endpoint)
			}

			// Clean up
			if err := notifier.Close(); err != nil {
				t.Errorf("Failed to close notifier: %v", err)
			}
		})
	}
}

func TestZMQNotifier_Subscribe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	notifier, err := NewZMQNotifier("tcp://localhost:28332", logger)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}
	defer func() { _ = notifier.Close() }()

	tests := []struct {
		name    string
		topic   string
		wantErr bool
	}{
		{
			name:    "valid topic",
			topic:   "hashblock",
			wantErr: false,
		},
		{
			name:    "another valid topic",
			topic:   "hashtx",
			wantErr: false,
		},
		{
			name:    "empty topic",
			topic:   "",
			wantErr: false, // ZMQ allows empty topic (subscribes to all)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := notifier.Subscribe(tt.topic)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Subscribe() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Subscribe() unexpected error: %v", err)
			}
		})
	}
}

func TestZMQNotifier_Connect(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid endpoint format",
			endpoint: "tcp://localhost:28332",
			wantErr:  false, // May fail to connect but format is valid
		},
		{
			name:     "invalid endpoint format",
			endpoint: "invalid://endpoint",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewZMQNotifier(tt.endpoint, logger)
			if err != nil {
				t.Fatalf("Failed to create notifier: %v", err)
			}
			defer func() { _ = notifier.Close() }()

			err = notifier.Connect()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Connect() expected error, got nil")
				}
				return
			}

			// Note: Connect may fail if no ZMQ server is running, but that's expected in tests
			// We're mainly testing that the method doesn't panic and handles errors gracefully
		})
	}
}

func TestZMQNotifier_Listen(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	notifier, err := NewZMQNotifier("tcp://localhost:28332", logger)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}
	defer func() { _ = notifier.Close() }()

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	messageReceived := false
	handler := func(_ string, _ []byte) error {
		messageReceived = true
		return nil
	}

	err = notifier.Listen(ctx, handler)

	// Should return context.Canceled error
	if err != context.Canceled {
		t.Errorf("Listen() with cancelled context should return context.Canceled, got %v", err)
	}

	if messageReceived {
		t.Errorf("Listen() should not have received messages with cancelled context")
	}
}

func TestZMQNotifier_ListenWithTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	notifier, err := NewZMQNotifier("tcp://localhost:28332", logger)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}
	defer func() { _ = notifier.Close() }()

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	messageReceived := false
	handler := func(_ string, _ []byte) error {
		messageReceived = true
		return nil
	}

	err = notifier.Listen(ctx, handler)

	// Should return context.DeadlineExceeded error
	if err != context.DeadlineExceeded {
		t.Errorf("Listen() with timeout should return context.DeadlineExceeded, got %v", err)
	}

	if messageReceived {
		t.Errorf("Listen() should not have received messages in timeout test")
	}
}

func TestNewBlockNotificationHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	handler := NewBlockNotificationHandler(logger)

	if handler == nil {
		t.Errorf("NewBlockNotificationHandler() returned nil")
		return
	}

	if handler.logger != logger {
		t.Errorf("NewBlockNotificationHandler() logger not set correctly")
	}
}

func TestBlockNotificationHandler_SetHandlers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewBlockNotificationHandler(logger)

	blockHandler := func(_ string) error {
		return nil
	}

	txHandler := func(_ string) error {
		return nil
	}

	handler.SetNewBlockHandler(blockHandler)
	handler.SetNewTxHandler(txHandler)

	// Test that handlers are set
	if handler.onNewBlock == nil {
		t.Errorf("SetNewBlockHandler() did not set handler")
	}

	if handler.onNewTx == nil {
		t.Errorf("SetNewTxHandler() did not set handler")
	}
}

func TestBlockNotificationHandler_HandleMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewBlockNotificationHandler(logger)

	blockCalled := false
	txCalled := false
	var receivedBlockHash, receivedTxHash string

	handler.SetNewBlockHandler(func(blockHash string) error {
		blockCalled = true
		receivedBlockHash = blockHash
		return nil
	})

	handler.SetNewTxHandler(func(txHash string) error {
		txCalled = true
		receivedTxHash = txHash
		return nil
	})

	tests := []struct {
		name        string
		topic       string
		data        []byte
		wantErr     bool
		expectBlock bool
		expectTx    bool
	}{
		{
			name:        "valid block hash",
			topic:       "hashblock",
			data:        make([]byte, 32), // 32 bytes of zeros
			wantErr:     false,
			expectBlock: true,
			expectTx:    false,
		},
		{
			name:        "valid tx hash",
			topic:       "hashtx",
			data:        make([]byte, 32), // 32 bytes of zeros
			wantErr:     false,
			expectBlock: false,
			expectTx:    true,
		},
		{
			name:        "invalid block hash length",
			topic:       "hashblock",
			data:        make([]byte, 16), // Wrong length
			wantErr:     true,
			expectBlock: false,
			expectTx:    false,
		},
		{
			name:        "invalid tx hash length",
			topic:       "hashtx",
			data:        make([]byte, 16), // Wrong length
			wantErr:     true,
			expectBlock: false,
			expectTx:    false,
		},
		{
			name:        "raw block",
			topic:       "rawblock",
			data:        []byte("block data"),
			wantErr:     false,
			expectBlock: false,
			expectTx:    false,
		},
		{
			name:        "raw tx",
			topic:       "rawtx",
			data:        []byte("tx data"),
			wantErr:     false,
			expectBlock: false,
			expectTx:    false,
		},
		{
			name:        "unknown topic",
			topic:       "unknown",
			data:        []byte("data"),
			wantErr:     false,
			expectBlock: false,
			expectTx:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset state
			blockCalled = false
			txCalled = false
			receivedBlockHash = ""
			receivedTxHash = ""

			err := handler.HandleMessage(tt.topic, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("HandleMessage() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("HandleMessage() unexpected error: %v", err)
				return
			}

			if blockCalled != tt.expectBlock {
				t.Errorf("HandleMessage() block handler called = %v, want %v", blockCalled, tt.expectBlock)
			}

			if txCalled != tt.expectTx {
				t.Errorf("HandleMessage() tx handler called = %v, want %v", txCalled, tt.expectTx)
			}

			if tt.expectBlock && receivedBlockHash == "" {
				t.Errorf("HandleMessage() block handler called but no hash received")
			}

			if tt.expectTx && receivedTxHash == "" {
				t.Errorf("HandleMessage() tx handler called but no hash received")
			}
		})
	}
}

func TestReverseHex(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "simple bytes",
			data: []byte{0x01, 0x02, 0x03, 0x04},
			want: "04030201",
		},
		{
			name: "empty bytes",
			data: []byte{},
			want: "",
		},
		{
			name: "single byte",
			data: []byte{0xff},
			want: "ff",
		},
		{
			name: "32 byte hash",
			data: make([]byte, 32),
			want: "0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reverseHex(tt.data)
			if got != tt.want {
				t.Errorf("reverseHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestZMQNotifier_Close(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	notifier, err := NewZMQNotifier("tcp://localhost:28332", logger)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	// Test closing
	err = notifier.Close()
	if err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}

	// Test closing again (should not panic)
	err = notifier.Close()
	if err != nil {
		t.Errorf("Close() second call unexpected error: %v", err)
	}
}

// Benchmark tests for performance
func BenchmarkReverseHex(b *testing.B) {
	data := make([]byte, 32)
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reverseHex(data)
	}
}

func BenchmarkBlockNotificationHandler_HandleMessage(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewBlockNotificationHandler(logger)

	handler.SetNewBlockHandler(func(_ string) error {
		return nil
	})

	data := make([]byte, 32)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.HandleMessage("hashblock", data)
	}
}
