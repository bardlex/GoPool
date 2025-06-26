package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/bardlex/gomp/internal/config"
	"github.com/bardlex/gomp/internal/database"
	"github.com/bardlex/gomp/internal/database/influx"
	"github.com/bardlex/gomp/internal/database/postgres"
	"github.com/bardlex/gomp/internal/database/redis"
	"github.com/bardlex/gomp/internal/messaging"
	"github.com/bardlex/gomp/internal/stratum"
	"github.com/bardlex/gomp/pkg/log"
)

func TestNewStratumServer(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-stratumd",
		Version:     "test",
		LogLevel:    "error", // Reduce log noise in tests
		LogFormat:   "json",
		ListenAddr:  "127.0.0.1",
		ListenPort:  0, // Use random port for tests
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	kafkaClient := messaging.NewKafkaClient([]string{"localhost:9092"}, logger.Logger)
	
	// Create minimal database config for testing
	dbConfig := &database.Config{
		Postgres: &postgres.Config{
			Host:         "localhost",
			Port:         5432,
			Database:     "test",
			User:         "test",
			Password:     "test",
			SSLMode:      "disable",
			MaxOpenConns: 1,
			MaxIdleConns: 1,
			MaxLifetime:  time.Minute,
		},
		Redis: &redis.Config{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           1, // Use test DB
			PoolSize:     1,
			MinIdleConns: 1,
			MaxRetries:   1,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
		Influx: &influx.Config{
			URL:    "http://localhost:8086",
			Token:  "test-token",
			Org:    "test-org",
			Bucket: "test-bucket",
		},
	}

	// Note: This will fail to connect to actual databases in test environment
	// We'll create a mock database manager for proper unit tests
	dbManager, _ := database.NewManager(dbConfig)

	server := NewStratumServer(cfg, logger, kafkaClient, dbManager)

	if server == nil {
		t.Fatal("NewStratumServer() returned nil")
	}

	if server.cfg != cfg {
		t.Error("NewStratumServer() did not set config correctly")
	}

	if server.logger == nil {
		t.Error("NewStratumServer() did not set logger correctly")
	}

	if server.kafkaClient != kafkaClient {
		t.Error("NewStratumServer() did not set kafkaClient correctly")
	}

	if server.sessions == nil {
		t.Error("NewStratumServer() did not initialize sessions map")
	}
}

func TestNewMessageHandler(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-stratumd",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	server := &StratumServer{} // Minimal server for testing

	handler := NewMessageHandler(cfg, logger, server)

	if handler == nil {
		t.Fatal("NewMessageHandler() returned nil")
	}

	if handler.cfg != cfg {
		t.Error("NewMessageHandler() did not set config correctly")
	}

	if handler.server != server {
		t.Error("NewMessageHandler() did not set server correctly")
	}
}

func TestMessageHandler_HandleMessage(t *testing.T) {
	cfg := &config.Config{
		ServiceName:   "test-stratumd",
		Version:       "test",
		LogLevel:      "error",
		LogFormat:     "json",
		MinDifficulty: 1.0,
		MaxDifficulty: 1000.0,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	server := &StratumServer{cfg: cfg}
	handler := NewMessageHandler(cfg, logger, server)

	tests := []struct {
		name    string
		message *stratum.Message
		wantErr bool
	}{
		{
			name: "valid request message",
			message: &stratum.Message{
				ID:     "1",
				Method: "mining.subscribe",
				Params: []interface{}{"test-miner/1.0"},
			},
			wantErr: false,
		},
		{
			name: "response message (should be ignored)",
			message: &stratum.Message{
				ID:     "1",
				Result: []byte(`true`),
			},
			wantErr: false,
		},
		{
			name: "notification message (should be ignored)",
			message: &stratum.Message{
				Method: "mining.notify",
				Params: []interface{}{"job1", "prev", "coinb1", "coinb2"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock session for testing
			mockConn := &mockConnection{}
			session := stratum.NewSession(
				"test-session",
				mockConn,
				logger,
				30*time.Second,
				30*time.Second,
			)

			ctx := context.Background()
			err := handler.HandleMessage(ctx, session, tt.message)

			if (err != nil) != tt.wantErr {
				t.Errorf("HandleMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMessageHandler_handleSubscribe(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "test-stratumd",
		Version:     "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	server := &StratumServer{cfg: cfg}
	handler := NewMessageHandler(cfg, logger, server)

	tests := []struct {
		name    string
		params  []interface{}
		wantErr bool
	}{
		{
			name:    "valid subscribe with user agent",
			params:  []interface{}{"test-miner/1.0"},
			wantErr: false,
		},
		{
			name:    "valid subscribe with user agent and session",
			params:  []interface{}{"test-miner/1.0", "session-id"},
			wantErr: false,
		},
		{
			name:    "invalid params - empty",
			params:  []interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &mockConnection{}
			session := stratum.NewSession(
				"test-session",
				mockConn,
				logger,
				30*time.Second,
				30*time.Second,
			)

			msg := &stratum.Message{
				ID:     "1",
				Method: "mining.subscribe",
				Params: tt.params,
			}

			ctx := context.Background()
			err := handler.handleSubscribe(ctx, session, msg)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleSubscribe() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check if session was properly subscribed for valid cases
			if !tt.wantErr && !session.IsSubscribed() {
				t.Error("handleSubscribe() did not set session as subscribed")
			}
		})
	}
}

func TestMessageHandler_handleAuthorize(t *testing.T) {
	cfg := &config.Config{
		ServiceName:   "test-stratumd",
		Version:       "test",
		LogLevel:      "error",
		LogFormat:     "json",
		MinDifficulty: 1.0,
	}

	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	server := &StratumServer{cfg: cfg}
	handler := NewMessageHandler(cfg, logger, server)

	tests := []struct {
		name        string
		subscribed  bool
		params      []interface{}
		wantErr     bool
		description string
	}{
		{
			name:        "valid authorize with subscribed session",
			subscribed:  true,
			params:      []interface{}{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "password"},
			wantErr:     false,
			description: "should authorize valid Bitcoin address",
		},
		{
			name:        "authorize without subscription",
			subscribed:  false,
			params:      []interface{}{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "password"},
			wantErr:     true,
			description: "should reject authorization without subscription",
		},
		{
			name:        "invalid address - too short",
			subscribed:  true,
			params:      []interface{}{"shortaddr", "password"},
			wantErr:     true,
			description: "should reject invalid Bitcoin address",
		},
		{
			name:        "invalid params - empty",
			subscribed:  true,
			params:      []interface{}{},
			wantErr:     true,
			description: "should reject empty parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &mockConnection{}
			session := stratum.NewSession(
				"test-session",
				mockConn,
				logger,
				30*time.Second,
				30*time.Second,
			)

			// Set subscription status
			session.SetSubscribed(tt.subscribed)

			msg := &stratum.Message{
				ID:     "1",
				Method: "mining.authorize",
				Params: tt.params,
			}

			ctx := context.Background()
			err := handler.handleAuthorize(ctx, session, msg)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleAuthorize() error = %v, wantErr %v (%s)", err, tt.wantErr, tt.description)
			}

			// Check authorization status for valid cases
			if !tt.wantErr && !session.IsAuthorized() {
				t.Error("handleAuthorize() did not set session as authorized")
			}
		})
	}
}

func TestGenerateShareID(t *testing.T) {
	server := &StratumServer{}

	// Generate multiple IDs to check uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := server.generateShareID()
		
		if id == "" {
			t.Error("generateShareID() returned empty string")
		}

		if ids[id] {
			t.Errorf("generateShareID() generated duplicate ID: %s", id)
		}
		ids[id] = true

		// Check that ID is reasonable length (hex string should be 32 chars for 16 bytes)
		if len(id) != 32 {
			t.Errorf("generateShareID() returned ID of unexpected length: %d (expected 32)", len(id))
		}
	}
}

// mockConnection implements net.Conn for testing
type mockConnection struct {
	readData  []byte
	writeData []byte
	closed    bool
}

func (m *mockConnection) Read(b []byte) (int, error) {
	if len(m.readData) == 0 {
		return 0, net.ErrClosed
	}
	n := copy(b, m.readData)
	m.readData = m.readData[n:]
	return n, nil
}

func (m *mockConnection) Write(b []byte) (int, error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConnection) Close() error {
	m.closed = true
	return nil
}

func (m *mockConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3333}
}

func (m *mockConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *mockConnection) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}