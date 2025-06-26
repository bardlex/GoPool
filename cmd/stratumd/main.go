// Package main implements stratumd service for GOMP mining pool.
// This service provides the Stratum V1 protocol server for miner connections.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := log.New(cfg.ServiceName, cfg.Version, cfg.LogLevel, cfg.LogFormat)
	logger.Info("starting stratumd",
		"version", cfg.Version,
		"listen_addr", cfg.ListenAddr,
		"listen_port", cfg.ListenPort,
	)

	// Create Kafka client
	kafkaClient := messaging.NewKafkaClient(
		cfg.KafkaBrokers,
		logger.Logger,
	)

	// Create database manager
	dbConfig := &database.Config{
		Postgres: &postgres.Config{
			Host:         "localhost",
			Port:         5432,
			Database:     "gomp",
			User:         "gomp",
			Password:     "gomp",
			SSLMode:      "disable",
			MaxOpenConns: 25,
			MaxIdleConns: 5,
			MaxLifetime:  5 * time.Minute,
		},
		Redis: &redis.Config{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           0,
			PoolSize:     10,
			MinIdleConns: 2,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Influx: &influx.Config{
			URL:    cfg.InfluxURL,
			Token:  cfg.InfluxToken,
			Org:    cfg.InfluxOrg,
			Bucket: cfg.InfluxBucket,
		},
	}

	dbManager, err := database.NewManager(dbConfig)
	if err != nil {
		logger.WithError(err).Error("failed to create database manager")
		os.Exit(1)
	}

	// Create the server
	server := NewStratumServer(cfg, logger, kafkaClient, dbManager)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the server
	go func() {
		if err := server.Start(ctx); err != nil {
			logger.WithError(err).Error("server failed")
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("shutdown signal received")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("shutdown failed")
		os.Exit(1)
	}

	logger.Info("stratumd stopped")
}

// StratumServer represents the main Stratum server
type StratumServer struct {
	cfg         *config.Config
	logger      *log.Logger
	listener    net.Listener
	sessions    map[string]*stratum.Session
	mu          sync.RWMutex
	wg          sync.WaitGroup
	kafkaClient *messaging.KafkaClient
	dbManager   *database.Manager
	currentJob  *messaging.JobMessage
	jobMu       sync.RWMutex
}

// NewStratumServer creates a new Stratum server
func NewStratumServer(cfg *config.Config, logger *log.Logger, kafkaClient *messaging.KafkaClient, dbManager *database.Manager) *StratumServer {
	return &StratumServer{
		cfg:         cfg,
		logger:      logger.WithComponent("server"),
		sessions:    make(map[string]*stratum.Session),
		kafkaClient: kafkaClient,
		dbManager:   dbManager,
	}
}

// Start starts the Stratum server
func (s *StratumServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.ListenAddr, s.cfg.ListenPort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	s.logger.Info("server listening", "address", addr)

	// Start Kafka job consumer
	s.wg.Add(1)
	go s.startJobConsumer(ctx)

	// Accept connections
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				s.logger.WithError(err).Error("failed to accept connection")
				continue
			}
		}

		// Handle the connection
		s.wg.Add(1)
		go s.handleConnection(ctx, conn)
	}
}

// handleConnection handles a new client connection
func (s *StratumServer) handleConnection(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		if err := conn.Close(); err != nil {
			s.logger.Error("failed to close connection", "error", err)
		}
	}()

	// Generate session ID
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())

	// Create session
	session := stratum.NewSession(
		sessionID,
		conn,
		s.logger,
		s.cfg.ReadTimeout,
		s.cfg.WriteTimeout,
	)

	// Register session
	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	// Cleanup session on exit
	defer func() {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
	}()

	// Create message handler
	handler := NewMessageHandler(s.cfg, s.logger, s)

	// Start session
	if err := session.Start(ctx, handler); err != nil {
		if err != context.Canceled {
			s.logger.WithError(err).Error("session failed")
		}
	}
}

// Shutdown gracefully shuts down the server
func (s *StratumServer) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")

	// Close listener
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error("failed to close listener", "error", err)
		}
	}

	// Close all sessions
	s.mu.RLock()
	for _, session := range s.sessions {
		session.Close()
	}
	s.mu.RUnlock()

	// Close Kafka client
	if s.kafkaClient != nil {
		if err := s.kafkaClient.Close(); err != nil {
			s.logger.WithError(err).Error("failed to close Kafka client")
		}
	}

	// Close database manager
	if s.dbManager != nil {
		if err := s.dbManager.Close(); err != nil {
			s.logger.WithError(err).Error("failed to close database manager")
		}
	}

	// Wait for all connections to close
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("all connections closed")
		return nil
	case <-ctx.Done():
		s.logger.Warn("shutdown timeout exceeded")
		return ctx.Err()
	}
}

// MessageHandler implements the stratum.MessageHandler interface
type MessageHandler struct {
	cfg    *config.Config
	logger *log.Logger
	server *StratumServer
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(cfg *config.Config, logger *log.Logger, server *StratumServer) *MessageHandler {
	return &MessageHandler{
		cfg:    cfg,
		logger: logger.WithComponent("handler"),
		server: server,
	}
}

// HandleMessage handles incoming Stratum messages
func (h *MessageHandler) HandleMessage(ctx context.Context, session *stratum.Session, msg *stratum.Message) error {
	if msg.IsRequest() {
		return h.handleRequest(ctx, session, msg)
	}

	// Ignore responses and notifications from clients
	h.logger.Debug("ignoring non-request message", "method", msg.Method)
	return nil
}

// handleRequest handles request messages
func (h *MessageHandler) handleRequest(ctx context.Context, session *stratum.Session, msg *stratum.Message) error {
	switch msg.Method {
	case "mining.subscribe":
		return h.handleSubscribe(ctx, session, msg)
	case "mining.authorize":
		return h.handleAuthorize(ctx, session, msg)
	case "mining.submit":
		return h.handleSubmit(ctx, session, msg)
	default:
		h.logger.Warn("unknown method", "method", msg.Method)
		return session.SendError(msg.ID, stratum.ErrorMethodNotFound, "Method not found")
	}
}

// handleSubscribe handles mining.subscribe requests
func (h *MessageHandler) handleSubscribe(_ context.Context, session *stratum.Session, msg *stratum.Message) error {
	req, err := stratum.ParseSubscribeRequest(msg.Params)
	if err != nil {
		h.logger.WithError(err).Error("invalid subscribe request")
		return session.SendError(msg.ID, stratum.ErrorInvalidParams, "Invalid parameters")
	}

	h.logger.Info("miner subscribed",
		"user_agent", req.UserAgent,
		"session_id", req.SessionID,
	)

	// Generate ExtraNonce1 (unique per session)
	extraNonce1 := fmt.Sprintf("%08x", time.Now().Unix())
	session.SetExtraNonce1(extraNonce1)
	session.SetSubscribed(true)

	// Send response
	response := stratum.SubscribeResponse{
		Subscriptions: [][]string{
			{"mining.notify", session.ID()},
		},
		ExtraNonce1:     extraNonce1,
		ExtraNonce2Size: 4, // 4 bytes for ExtraNonce2
	}

	return session.SendResponse(msg.ID, []interface{}{
		response.Subscriptions,
		response.ExtraNonce1,
		response.ExtraNonce2Size,
	})
}

// handleAuthorize handles mining.authorize requests
func (h *MessageHandler) handleAuthorize(_ context.Context, session *stratum.Session, msg *stratum.Message) error {
	if !session.IsSubscribed() {
		return session.SendError(msg.ID, stratum.ErrorNotSubscribed, "Not subscribed")
	}

	req, err := stratum.ParseAuthorizeRequest(msg.Params)
	if err != nil {
		h.logger.WithError(err).Error("invalid authorize request")
		return session.SendError(msg.ID, stratum.ErrorInvalidParams, "Invalid parameters")
	}

	// Parse username (format: address.worker_name)
	minerAddr := req.Username
	workerName := "default"

	// Simple validation - in production, you'd validate the Bitcoin address
	if len(minerAddr) < 26 {
		return session.SendError(msg.ID, stratum.ErrorUnauthorized, "Invalid address")
	}

	session.SetUsername(minerAddr)
	session.SetWorkerName(workerName)
	session.SetAuthorized(true)

	h.logger.Info("miner authorized",
		"miner_address", minerAddr,
		"worker_name", workerName,
	)

	// Send initial difficulty
	if err := session.SendNotification("mining.set_difficulty", []interface{}{h.cfg.MinDifficulty}); err != nil {
		h.logger.WithError(err).Error("failed to send difficulty")
	}
	session.SetDifficulty(h.cfg.MinDifficulty)

	return session.SendResponse(msg.ID, true)
}

// handleSubmit handles mining.submit requests
func (h *MessageHandler) handleSubmit(_ context.Context, session *stratum.Session, msg *stratum.Message) error {
	if !session.IsAuthorized() {
		return session.SendError(msg.ID, stratum.ErrorUnauthorized, "Not authorized")
	}

	req, err := stratum.ParseSubmitRequest(msg.Params)
	if err != nil {
		h.logger.WithError(err).Error("invalid submit request")
		return session.SendError(msg.ID, stratum.ErrorInvalidParams, "Invalid parameters")
	}

	// Record share for vardiff
	session.RecordShare()

	h.logger.LogShareSubmission(
		session.Username(),
		session.WorkerName(),
		req.JobID,
		session.Difficulty(),
		"pending",
	)

	// Publish share to Kafka for validation
	if err := h.server.publishShare(session, req); err != nil {
		h.logger.WithError(err).Error("failed to publish share")
		// Continue processing even if Kafka publish fails
	}

	// Check if difficulty should be adjusted
	if shouldAdjust, newDiff := session.ShouldAdjustDifficulty(); shouldAdjust {
		// Apply bounds
		if newDiff < h.cfg.MinDifficulty {
			newDiff = h.cfg.MinDifficulty
		} else if newDiff > h.cfg.MaxDifficulty {
			newDiff = h.cfg.MaxDifficulty
		}

		if newDiff != session.Difficulty() {
			h.logger.Info("adjusting difficulty",
				"old_difficulty", session.Difficulty(),
				"new_difficulty", newDiff,
			)

			session.SetDifficulty(newDiff)
			if err := session.SendNotification("mining.set_difficulty", []interface{}{newDiff}); err != nil {
				h.logger.WithError(err).Error("failed to send difficulty adjustment")
			}
		}
	}

	return session.SendResponse(msg.ID, true)
}

// startJobConsumer starts consuming jobs from Kafka
func (s *StratumServer) startJobConsumer(ctx context.Context) {
	defer s.wg.Done()

	reader := s.kafkaClient.GetConsumer(messaging.TopicJobs, s.cfg.KafkaGroupID)
	defer func() {
		if err := reader.Close(); err != nil {
			s.logger.Error("failed to close Kafka reader", "error", err)
		}
	}()

	s.logger.Info("started job consumer", "topic", messaging.TopicJobs)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("job consumer stopping")
			return
		default:
		}

		kafkaMsg, err := reader.ReadMessage(ctx)
		if err != nil {
			s.logger.WithError(err).Error("failed to read job message")
			continue
		}

		var jobMsg messaging.JobMessage
		if err := json.Unmarshal(kafkaMsg.Value, &jobMsg); err != nil {
			s.logger.WithError(err).Error("failed to unmarshal job message")
			continue
		}

		s.logger.Info("received new job", "job_id", jobMsg.JobID, "block_height", jobMsg.BlockHeight)

		// Update current job
		s.jobMu.Lock()
		s.currentJob = &jobMsg
		s.jobMu.Unlock()

		// Send job to all connected miners
		s.broadcastJob(&jobMsg)
	}
}

// broadcastJob sends a new job to all connected miners
func (s *StratumServer) broadcastJob(job *messaging.JobMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.sessions) == 0 {
		return
	}

	s.logger.Info("broadcasting job to miners", "job_id", job.JobID, "miner_count", len(s.sessions))

	for _, session := range s.sessions {
		if session.IsSubscribed() {
			go s.sendJobToSession(session, job)
		}
	}
}

// sendJobToSession sends a job to a specific session
func (s *StratumServer) sendJobToSession(session *stratum.Session, job *messaging.JobMessage) {
	params := []interface{}{
		job.JobID,
		job.PrevHash,
		job.Coinb1,
		job.Coinb2,
		job.MerkleBranch,
		job.Version,
		job.NBits,
		job.NTime,
		job.CleanJobs,
	}

	if err := session.SendNotification("mining.notify", params); err != nil {
		s.logger.WithError(err).Error("failed to send job to session", "session_id", session.ID())
	}
}

// publishShare publishes a share to Kafka for validation
func (s *StratumServer) publishShare(session *stratum.Session, req *stratum.SubmitRequest) error {
	// Generate unique share ID
	shareID := s.generateShareID()

	// Get current job
	s.jobMu.RLock()
	currentJob := s.currentJob
	s.jobMu.RUnlock()

	if currentJob == nil {
		return fmt.Errorf("no current job available")
	}

	// Create share message
	shareMsg := &messaging.ShareMessage{
		ShareID:      shareID,
		JobID:        req.JobID,
		UserID:       0, // TODO: Get from database
		WorkerID:     0, // TODO: Get from database
		MinerAddress: session.Username(),
		WorkerName:   session.WorkerName(),
		ExtraNonce2:  req.ExtraNonce2,
		Ntime:        req.NTime,
		Nonce:        req.Nonce,
		Difficulty:   session.Difficulty(),
		BlockHeight:  currentJob.BlockHeight,
		SessionID:    session.ID(),
		RemoteAddr:   session.RemoteAddr(),
		SubmittedAt:  time.Now(),
	}

	// Serialize and publish
	data, err := json.Marshal(shareMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal share message: %w", err)
	}

	return s.kafkaClient.PublishJSON(context.Background(), messaging.TopicShares, shareID, data)
}

// generateShareID generates a unique share ID
func (s *StratumServer) generateShareID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("share_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
