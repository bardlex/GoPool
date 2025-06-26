// Package messaging provides Kafka-based inter-service communication for GOMP mining pool.
// It handles job distribution, share processing, and statistics messaging.
package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	"github.com/bardlex/gomp/pkg/circuit"
	"github.com/bardlex/gomp/pkg/errors"
	"github.com/bardlex/gomp/pkg/retry"
)

// KafkaClient wraps kafka-go with protobuf support and connection pooling
type KafkaClient struct {
	brokers        []string
	logger         *slog.Logger
	writers        map[string]*kafka.Writer
	readers        map[string]*kafka.Reader
	writersMu      sync.RWMutex
	readersMu      sync.RWMutex
	circuitBreaker *circuit.Breaker
	retryConfig    *retry.Config
}

// NewKafkaClient creates a new Kafka client
func NewKafkaClient(brokers []string, logger *slog.Logger) *KafkaClient {
	// Configure circuit breaker for Kafka operations
	cbConfig := &circuit.Config{
		MaxFailures:     5,
		SuccessRequired: 3,
		Timeout:         15 * time.Second,
		ResetTimeout:    60 * time.Second,
	}

	return &KafkaClient{
		brokers:        brokers,
		logger:         logger,
		writers:        make(map[string]*kafka.Writer),
		readers:        make(map[string]*kafka.Reader),
		circuitBreaker: circuit.New(cbConfig),
		retryConfig:    retry.NetworkConfig(),
	}
}

// GetProducer gets or creates a Kafka producer for a topic (with connection pooling)
func (k *KafkaClient) GetProducer(topic string) *kafka.Writer {
	k.writersMu.RLock()
	if writer, exists := k.writers[topic]; exists {
		k.writersMu.RUnlock()
		return writer
	}
	k.writersMu.RUnlock()

	k.writersMu.Lock()
	defer k.writersMu.Unlock()

	// Double-check after acquiring write lock
	if writer, exists := k.writers[topic]; exists {
		return writer
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(k.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		Compression:  kafka.Snappy,
	}

	k.writers[topic] = writer
	k.logger.Info("created Kafka producer", "topic", topic)
	return writer
}

// GetConsumer gets or creates a Kafka consumer for a topic and group
func (k *KafkaClient) GetConsumer(topic, groupID string) *kafka.Reader {
	key := fmt.Sprintf("%s-%s", topic, groupID)

	k.readersMu.RLock()
	if reader, exists := k.readers[key]; exists {
		k.readersMu.RUnlock()
		return reader
	}
	k.readersMu.RUnlock()

	k.readersMu.Lock()
	defer k.readersMu.Unlock()

	// Double-check after acquiring write lock
	if reader, exists := k.readers[key]; exists {
		return reader
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.brokers,
		Topic:       topic,
		GroupID:     groupID,
		StartOffset: kafka.LastOffset,
		MinBytes:    1,
		MaxBytes:    10e6, // 10MB
		MaxWait:     1 * time.Second,
	})

	k.readers[key] = reader
	k.logger.Info("created Kafka consumer", "topic", topic, "group_id", groupID)
	return reader
}

// PublishProto publishes a protobuf message to Kafka
func (k *KafkaClient) PublishProto(ctx context.Context, topic, key string, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "protobuf_marshal", 
			"failed to marshal protobuf message").
			WithContext("topic", topic).
			WithContext("key", key)
	}

	return k.circuitBreaker.Execute(ctx, func() error {
		return retry.Do(ctx, k.retryConfig, func() error {
			writer := k.GetProducer(topic)
			kafkaMsg := kafka.Message{
				Key:   []byte(key),
				Value: data,
				Time:  time.Now(),
			}

			if err := writer.WriteMessages(ctx, kafkaMsg); err != nil {
				return errors.Wrap(err, errors.ErrorTypeKafka, "publish_message", 
					"failed to publish message to Kafka").
					WithContext("topic", topic).
					WithContext("key", key).
					WithContext("message_size", len(data))
			}

			k.logger.Debug("published message", "topic", topic, "key", key, "size", len(data))
			return nil
		})
	})
}

// PublishJSON publishes a JSON message to Kafka
func (k *KafkaClient) PublishJSON(ctx context.Context, topic, key string, data []byte) error {
	return k.circuitBreaker.Execute(ctx, func() error {
		return retry.Do(ctx, k.retryConfig, func() error {
			writer := k.GetProducer(topic)
			kafkaMsg := kafka.Message{
				Key:   []byte(key),
				Value: data,
				Time:  time.Now(),
			}

			if err := writer.WriteMessages(ctx, kafkaMsg); err != nil {
				return errors.Wrap(err, errors.ErrorTypeKafka, "publish_json", 
					"failed to publish JSON message to Kafka").
					WithContext("topic", topic).
					WithContext("key", key).
					WithContext("message_size", len(data))
			}

			k.logger.Debug("published JSON message", "topic", topic, "key", key, "size", len(data))
			return nil
		})
	})
}

// ConsumeProto consumes and unmarshals protobuf messages from Kafka
func (k *KafkaClient) ConsumeProto(ctx context.Context, reader *kafka.Reader, msg proto.Message) (string, error) {
	return circuit.ExecuteWithResult(ctx, k.circuitBreaker, func() (string, error) {
		return retry.DoWithResult(ctx, k.retryConfig, func() (string, error) {
			kafkaMsg, err := reader.ReadMessage(ctx)
			if err != nil {
				return "", errors.Wrap(err, errors.ErrorTypeKafka, "read_message", 
					"failed to read message from Kafka")
			}

			if err := proto.Unmarshal(kafkaMsg.Value, msg); err != nil {
				return "", errors.Wrap(err, errors.ErrorTypeValidation, "protobuf_unmarshal", 
					"failed to unmarshal protobuf message").
					WithContext("topic", kafkaMsg.Topic).
					WithContext("message_size", len(kafkaMsg.Value))
			}

			key := string(kafkaMsg.Key)
			k.logger.Debug("consumed message", "topic", kafkaMsg.Topic, "key", key, "size", len(kafkaMsg.Value))
			return key, nil
		})
	})
}

// MessageHandler defines the interface for handling Kafka messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, key string, msg proto.Message) error
}

// StartConsumer starts a consumer loop for a topic
func (k *KafkaClient) StartConsumer(ctx context.Context, topic, groupID string, msgFactory func() proto.Message, handler MessageHandler) error {
	reader := k.GetConsumer(topic, groupID)
	defer func() {
		if err := reader.Close(); err != nil {
			k.logger.Error("failed to close Kafka reader", "error", err)
		}
	}()

	k.logger.Info("starting consumer", "topic", topic, "group_id", groupID)

	for {
		select {
		case <-ctx.Done():
			k.logger.Info("consumer stopping", "topic", topic)
			return ctx.Err()
		default:
		}

		msg := msgFactory()
		key, err := k.ConsumeProto(ctx, reader, msg)
		if err != nil {
			k.logger.Error("failed to consume message", "topic", topic, "error", err)
			continue
		}

		if err := handler.HandleMessage(ctx, key, msg); err != nil {
			k.logger.Error("failed to handle message", "topic", topic, "key", key, "error", err)
		}
	}
}

// Close closes all producers and consumers
func (k *KafkaClient) Close() error {
	k.writersMu.Lock()
	defer k.writersMu.Unlock()

	k.readersMu.Lock()
	defer k.readersMu.Unlock()

	var lastErr error

	// Close all writers
	for topic, writer := range k.writers {
		if err := writer.Close(); err != nil {
			k.logger.Error("failed to close producer", "topic", topic, "error", err)
			lastErr = err
		}
	}

	// Close all readers
	for key, reader := range k.readers {
		if err := reader.Close(); err != nil {
			k.logger.Error("failed to close consumer", "key", key, "error", err)
			lastErr = err
		}
	}

	k.writers = make(map[string]*kafka.Writer)
	k.readers = make(map[string]*kafka.Reader)
	return lastErr
}
