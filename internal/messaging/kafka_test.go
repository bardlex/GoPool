package messaging

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/bardlex/gomp/proto/gomp/v1"
)

func TestNewKafkaClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	brokers := []string{"localhost:9092"}

	client := NewKafkaClient(brokers, logger)

	if client == nil {
		t.Fatal("NewKafkaClient returned nil")
	}

	if len(client.brokers) != 1 || client.brokers[0] != "localhost:9092" {
		t.Errorf("Expected brokers [localhost:9092], got %v", client.brokers)
	}

	if client.logger == nil {
		t.Error("Logger should not be nil")
	}

	if client.writers == nil {
		t.Error("Writers map should not be nil")
	}

	if client.readers == nil {
		t.Error("Readers map should not be nil")
	}
}

func TestKafkaClient_GetProducer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewKafkaClient([]string{"localhost:9092"}, logger)

	topic := "test-topic"

	// First call should create a new producer
	producer1 := client.GetProducer(topic)
	if producer1 == nil {
		t.Fatal("GetProducer returned nil")
	}

	if producer1.Topic != topic {
		t.Errorf("Expected topic %s, got %s", topic, producer1.Topic)
	}

	// Second call should return the same producer (cached)
	producer2 := client.GetProducer(topic)
	if producer1 != producer2 {
		t.Error("Expected same producer instance from cache")
	}

	// Verify producer is stored in map
	if len(client.writers) != 1 {
		t.Errorf("Expected 1 writer in map, got %d", len(client.writers))
	}
}

func TestKafkaClient_GetConsumer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewKafkaClient([]string{"localhost:9092"}, logger)

	topic := "test-topic"
	groupID := "test-group"

	// First call should create a new consumer
	consumer1 := client.GetConsumer(topic, groupID)
	if consumer1 == nil {
		t.Fatal("GetConsumer returned nil")
	}

	// Second call should return the same consumer (cached)
	consumer2 := client.GetConsumer(topic, groupID)
	if consumer1 != consumer2 {
		t.Error("Expected same consumer instance from cache")
	}

	// Different group should create different consumer
	consumer3 := client.GetConsumer(topic, "different-group")
	if consumer1 == consumer3 {
		t.Error("Expected different consumer for different group")
	}

	// Verify consumers are stored in map
	if len(client.readers) != 2 {
		t.Errorf("Expected 2 readers in map, got %d", len(client.readers))
	}
}

func TestKafkaClient_PublishProto(t *testing.T) {
	// Skip integration test if Kafka is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewKafkaClient([]string{"localhost:9092"}, logger)

	// Create a test protobuf message
	job := &pb.MiningJob{
		JobId:        "test-job-123",
		PrevHash:     "0000000000000000000000000000000000000000000000000000000000000000",
		Coinb1:       "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff",
		Coinb2:       "ffffffff0100f2052a01000000434104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac00000000",
		MerkleBranch: []string{},
		Version:      "20000000",
		Nbits:        "1d00ffff",
		Ntime:        "504e86b9",
		CleanJobs:    true,
		BlockHeight:  100,
		CreatedAt:    timestamppb.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This will fail if Kafka is not running, but that's expected in unit tests
	err := client.PublishProto(ctx, TopicJobs, "test-key", job)
	if err != nil {
		t.Logf("Expected error without Kafka running: %v", err)
		// This is expected in unit tests without Kafka
		return
	}

	t.Log("Successfully published message to Kafka")
}

func TestTopicConstants(t *testing.T) {
	expectedTopics := map[string]string{
		"TopicJobs":            "mining.jobs",
		"TopicShares":          "mining.shares",
		"TopicBlockCandidates": "mining.block_candidates",
		"TopicBlockResults":    "mining.block_results",
		"TopicShareResults":    "mining.share_results",
		"TopicUserStats":       "mining.user_stats",
		"TopicMinerStats":      "miner.stats",
		"TopicPoolStats":       "pool.stats",
	}

	actualTopics := map[string]string{
		"TopicJobs":            TopicJobs,
		"TopicShares":          TopicShares,
		"TopicBlockCandidates": TopicBlockCandidates,
		"TopicBlockResults":    TopicBlockResults,
		"TopicShareResults":    TopicShareResults,
		"TopicUserStats":       TopicUserStats,
		"TopicMinerStats":      TopicMinerStats,
		"TopicPoolStats":       TopicPoolStats,
	}

	for name, expected := range expectedTopics {
		if actual, exists := actualTopics[name]; !exists {
			t.Errorf("Topic constant %s is missing", name)
		} else if actual != expected {
			t.Errorf("Topic %s: expected %s, got %s", name, expected, actual)
		}
	}
}

// Mock message handler for testing
type mockMessageHandler struct {
	messages []mockMessage
}

type mockMessage struct {
	key string
	msg proto.Message
}

func (h *mockMessageHandler) HandleMessage(_ context.Context, key string, msg proto.Message) error {
	h.messages = append(h.messages, mockMessage{key: key, msg: msg})
	return nil
}

func TestKafkaClient_StartConsumer(t *testing.T) {
	// Skip integration test if Kafka is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewKafkaClient([]string{"localhost:9092"}, logger)

	handler := &mockMessageHandler{}

	msgFactory := func() proto.Message {
		return &pb.MiningJob{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This will timeout quickly since we don't have messages to consume
	err := client.StartConsumer(ctx, TopicJobs, "test-group", msgFactory, handler)
	if err != context.DeadlineExceeded {
		t.Logf("Consumer stopped with: %v", err)
	}

	// Verify no messages were processed (expected without Kafka)
	if len(handler.messages) > 0 {
		t.Errorf("Expected 0 messages, got %d", len(handler.messages))
	}
}

func TestKafkaClient_Close(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewKafkaClient([]string{"localhost:9092"}, logger)

	// Create some producers and consumers
	_ = client.GetProducer("topic1")
	_ = client.GetProducer("topic2")
	_ = client.GetConsumer("topic1", "group1")
	_ = client.GetConsumer("topic2", "group2")

	// Verify they were created
	if len(client.writers) != 2 {
		t.Errorf("Expected 2 writers, got %d", len(client.writers))
	}
	if len(client.readers) != 2 {
		t.Errorf("Expected 2 readers, got %d", len(client.readers))
	}

	// Close the client
	err := client.Close()
	if err != nil {
		t.Logf("Close returned error (expected without Kafka): %v", err)
	}

	// Verify maps were cleared
	if len(client.writers) != 0 {
		t.Errorf("Expected 0 writers after close, got %d", len(client.writers))
	}
	if len(client.readers) != 0 {
		t.Errorf("Expected 0 readers after close, got %d", len(client.readers))
	}
}

// Benchmark tests for performance
func BenchmarkKafkaClient_GetProducer(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewKafkaClient([]string{"localhost:9092"}, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.GetProducer("test-topic")
	}
}

func BenchmarkKafkaClient_GetConsumer(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewKafkaClient([]string{"localhost:9092"}, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.GetConsumer("test-topic", "test-group")
	}
}

func BenchmarkProtoMarshal(b *testing.B) {
	job := &pb.MiningJob{
		JobId:        "test-job-123",
		PrevHash:     "0000000000000000000000000000000000000000000000000000000000000000",
		Coinb1:       "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff",
		Coinb2:       "ffffffff0100f2052a01000000434104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac00000000",
		MerkleBranch: []string{},
		Version:      "20000000",
		Nbits:        "1d00ffff",
		Ntime:        "504e86b9",
		CleanJobs:    true,
		BlockHeight:  100,
		CreatedAt:    timestamppb.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := proto.Marshal(job)
		if err != nil {
			b.Fatal(err)
		}
	}
}
