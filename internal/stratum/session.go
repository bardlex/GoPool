package stratum

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/bardlex/gomp/pkg/log"
)

// Session represents a Stratum mining session
type Session struct {
	id     string
	conn   net.Conn
	logger *log.Logger

	// Session state
	subscribed  bool
	authorized  bool
	username    string
	workerName  string
	extraNonce1 string
	difficulty  float64

	// Vardiff tracking
	lastShareTime time.Time
	shareCount    int64
	vardiffWindow time.Duration
	vardiffTarget time.Duration

	// Connection management
	readTimeout  time.Duration
	writeTimeout time.Duration

	// Channels for communication
	outbound chan []byte
	done     chan struct{}

	// Synchronization
	mu sync.RWMutex
}

// NewSession creates a new Stratum session
func NewSession(id string, conn net.Conn, logger *log.Logger, readTimeout, writeTimeout time.Duration) *Session {
	return &Session{
		id:            id,
		conn:          conn,
		logger:        logger.WithFields("session_id", id, "remote_addr", conn.RemoteAddr().String()),
		difficulty:    1.0, // Default difficulty
		vardiffWindow: 90 * time.Second,
		vardiffTarget: 30 * time.Second,
		readTimeout:   readTimeout,
		writeTimeout:  writeTimeout,
		outbound:      make(chan []byte, 100), // Buffered channel for outbound messages
		done:          make(chan struct{}),
	}
}

// Start begins processing the session
func (s *Session) Start(ctx context.Context, handler MessageHandler) error {
	s.logger.LogConnection("connected", s.conn.RemoteAddr().String())

	// Start the write goroutine
	go s.writeLoop(ctx)

	// Start the read loop in the current goroutine
	return s.readLoop(ctx, handler)
}

// readLoop handles incoming messages from the client
func (s *Session) readLoop(ctx context.Context, handler MessageHandler) error {
	defer s.Close()

	scanner := bufio.NewScanner(s.conn)
	scanner.Buffer(make([]byte, 4096), 4096) // Set buffer size

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			return nil
		default:
		}

		// Set read deadline
		if err := s.conn.SetReadDeadline(time.Now().Add(s.readTimeout)); err != nil {
			s.logger.WithError(err).Error("failed to set read deadline")
			return err
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				s.logger.WithError(err).Error("scanner error")
				return err
			}
			// EOF - client disconnected
			s.logger.Info("client disconnected")
			return nil
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		s.logger.LogStratumMessage("received", string(line))

		// Parse the message
		msg, err := ParseMessage(line)
		if err != nil {
			s.logger.WithError(err).Error("failed to parse message")
			if sendErr := s.SendError(nil, ErrorParseError, "Parse error"); sendErr != nil {
				s.logger.WithError(sendErr).Error("failed to send parse error")
			}
			continue
		}

		// Handle the message
		if err := handler.HandleMessage(ctx, s, msg); err != nil {
			s.logger.WithError(err).Error("failed to handle message")
		}
	}
}

// writeLoop handles outbound messages to the client
func (s *Session) writeLoop(ctx context.Context) {
	defer func() {
		if err := s.conn.Close(); err != nil {
			s.logger.Error("failed to close connection", "error", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case data := <-s.outbound:
			if err := s.conn.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil {
				s.logger.WithError(err).Error("failed to set write deadline")
				return
			}

			// Add newline delimiter
			data = append(data, '\n')

			if _, err := s.conn.Write(data); err != nil {
				s.logger.WithError(err).Error("failed to write message")
				return
			}

			s.logger.LogStratumMessage("sent", string(data[:len(data)-1])) // Log without newline
		}
	}
}

// SendMessage sends a message to the client
func (s *Session) SendMessage(msg *Message) error {
	data, err := MarshalMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	select {
	case s.outbound <- data:
		return nil
	case <-s.done:
		return fmt.Errorf("session closed")
	default:
		return fmt.Errorf("outbound channel full")
	}
}

// SendResponse sends a response message
func (s *Session) SendResponse(id interface{}, result interface{}) error {
	return s.SendMessage(NewResponse(id, result))
}

// SendError sends an error response
func (s *Session) SendError(id interface{}, code int, message string) error {
	return s.SendMessage(NewErrorResponse(id, code, message))
}

// SendNotification sends a notification message
func (s *Session) SendNotification(method string, params []interface{}) error {
	return s.SendMessage(NewNotification(method, params))
}

// Close closes the session
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.done:
		return // Already closed
	default:
		close(s.done)
		s.logger.LogConnection("disconnected", s.conn.RemoteAddr().String())
	}
}

// Getters and setters with proper locking

// ID returns the unique session identifier.
func (s *Session) ID() string {
	return s.id
}

// RemoteAddr returns the remote address of the client connection.
func (s *Session) RemoteAddr() string {
	return s.conn.RemoteAddr().String()
}

// IsSubscribed returns whether the session has completed mining.subscribe.
func (s *Session) IsSubscribed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subscribed
}

// SetSubscribed sets the subscription status of the session.
func (s *Session) SetSubscribed(subscribed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribed = subscribed
}

// IsAuthorized returns whether the session has completed mining.authorize.
func (s *Session) IsAuthorized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authorized
}

// SetAuthorized sets the authorization status of the session.
func (s *Session) SetAuthorized(authorized bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorized = authorized
}

// Username returns the miner's username (Bitcoin address).
func (s *Session) Username() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username
}

// SetUsername sets the miner's username (Bitcoin address).
func (s *Session) SetUsername(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.username = username
}

// WorkerName returns the worker name for this session.
func (s *Session) WorkerName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workerName
}

// SetWorkerName sets the worker name for this session.
func (s *Session) SetWorkerName(workerName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workerName = workerName
}

// ExtraNonce1 returns the ExtraNonce1 value for this session.
func (s *Session) ExtraNonce1() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extraNonce1
}

// SetExtraNonce1 sets the ExtraNonce1 value for this session.
func (s *Session) SetExtraNonce1(extraNonce1 string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extraNonce1 = extraNonce1
}

// Difficulty returns the current difficulty target for this session.
func (s *Session) Difficulty() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.difficulty
}

// SetDifficulty sets the difficulty target for this session.
func (s *Session) SetDifficulty(difficulty float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.difficulty = difficulty
}

// RecordShare records a share submission for vardiff calculation
func (s *Session) RecordShare() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.lastShareTime = now
	s.shareCount++
}

// ShouldAdjustDifficulty checks if difficulty should be adjusted based on vardiff
func (s *Session) ShouldAdjustDifficulty() (bool, float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.shareCount == 0 {
		return false, s.difficulty
	}

	timeSinceLastShare := time.Since(s.lastShareTime)
	if timeSinceLastShare < s.vardiffWindow {
		return false, s.difficulty
	}

	// Calculate average time between shares
	avgShareTime := timeSinceLastShare / time.Duration(s.shareCount)

	// Adjust difficulty to target the desired share interval
	targetRatio := avgShareTime.Seconds() / s.vardiffTarget.Seconds()
	newDifficulty := s.difficulty * targetRatio

	// Apply bounds and hysteresis
	const minAdjustment = 0.1 // 10% minimum change
	if targetRatio > 1+minAdjustment || targetRatio < 1-minAdjustment {
		return true, newDifficulty
	}

	return false, s.difficulty
}

// MessageHandler interface for handling Stratum messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, session *Session, msg *Message) error
}
