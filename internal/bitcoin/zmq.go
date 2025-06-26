package bitcoin

import (
	"context"
	"fmt"
	"log/slog"

	zmq "github.com/pebbe/zmq4"
)

// ZMQNotifier handles ZMQ notifications from Bitcoin Core
type ZMQNotifier struct {
	socket   *zmq.Socket
	endpoint string
	logger   *slog.Logger
}

// NewZMQNotifier creates a new ZMQ notifier
func NewZMQNotifier(endpoint string, logger *slog.Logger) (*ZMQNotifier, error) {
	socket, err := zmq.NewSocket(zmq.SUB)
	if err != nil {
		return nil, fmt.Errorf("failed to create ZMQ socket: %w", err)
	}

	return &ZMQNotifier{
		socket:   socket,
		endpoint: endpoint,
		logger:   logger,
	}, nil
}

// Subscribe subscribes to a specific topic
func (z *ZMQNotifier) Subscribe(topic string) error {
	if err := z.socket.SetSubscribe(topic); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}
	z.logger.Info("subscribed to ZMQ topic", "topic", topic)
	return nil
}

// Connect connects to the ZMQ endpoint
func (z *ZMQNotifier) Connect() error {
	if err := z.socket.Connect(z.endpoint); err != nil {
		return fmt.Errorf("failed to connect to ZMQ endpoint %s: %w", z.endpoint, err)
	}
	z.logger.Info("connected to ZMQ endpoint", "endpoint", z.endpoint)
	return nil
}

// Listen starts listening for ZMQ messages
func (z *ZMQNotifier) Listen(ctx context.Context, handler func(topic string, data []byte) error) error {
	z.logger.Info("starting ZMQ listener")

	for {
		select {
		case <-ctx.Done():
			z.logger.Info("ZMQ listener stopping")
			return ctx.Err()
		default:
		}

		// Receive message with timeout
		msg, err := z.socket.RecvMessageBytes(zmq.DONTWAIT)
		if err != nil {
			if err.Error() == "resource temporarily unavailable" {
				// No message available, continue
				continue
			}
			z.logger.Error("failed to receive ZMQ message", "error", err)
			continue
		}

		if len(msg) < 2 {
			z.logger.Warn("received malformed ZMQ message", "parts", len(msg))
			continue
		}

		topic := string(msg[0])
		data := msg[1]

		z.logger.Debug("received ZMQ message", "topic", topic, "size", len(data))

		if err := handler(topic, data); err != nil {
			z.logger.Error("failed to handle ZMQ message", "topic", topic, "error", err)
		}
	}
}

// Close closes the ZMQ socket
func (z *ZMQNotifier) Close() error {
	if z.socket != nil {
		return z.socket.Close()
	}
	return nil
}

// BlockNotificationHandler handles block-related ZMQ notifications
type BlockNotificationHandler struct {
	logger     *slog.Logger
	onNewBlock func(blockHash string) error
	onNewTx    func(txHash string) error
}

// NewBlockNotificationHandler creates a new block notification handler
func NewBlockNotificationHandler(logger *slog.Logger) *BlockNotificationHandler {
	return &BlockNotificationHandler{
		logger: logger,
	}
}

// SetNewBlockHandler sets the handler for new block notifications
func (h *BlockNotificationHandler) SetNewBlockHandler(handler func(blockHash string) error) {
	h.onNewBlock = handler
}

// SetNewTxHandler sets the handler for new transaction notifications
func (h *BlockNotificationHandler) SetNewTxHandler(handler func(txHash string) error) {
	h.onNewTx = handler
}

// HandleMessage handles a ZMQ message
func (h *BlockNotificationHandler) HandleMessage(topic string, data []byte) error {
	switch topic {
	case "hashblock":
		if len(data) != 32 {
			return fmt.Errorf("invalid block hash length: %d", len(data))
		}

		// Reverse bytes for proper endianness
		blockHash := reverseHex(data)
		h.logger.Info("new block notification", "hash", blockHash)

		if h.onNewBlock != nil {
			return h.onNewBlock(blockHash)
		}

	case "hashtx":
		if len(data) != 32 {
			return fmt.Errorf("invalid tx hash length: %d", len(data))
		}

		// Reverse bytes for proper endianness
		txHash := reverseHex(data)
		h.logger.Debug("new transaction notification", "hash", txHash)

		if h.onNewTx != nil {
			return h.onNewTx(txHash)
		}

	case "rawblock":
		h.logger.Info("raw block notification", "size", len(data))
		// Could parse the raw block here if needed

	case "rawtx":
		h.logger.Debug("raw transaction notification", "size", len(data))
		// Could parse the raw transaction here if needed

	default:
		h.logger.Warn("unknown ZMQ topic", "topic", topic)
	}

	return nil
}

// reverseHex reverses bytes and converts to hex string
func reverseHex(data []byte) string {
	reversed := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		reversed[i] = data[len(data)-1-i]
	}
	return fmt.Sprintf("%x", reversed)
}
