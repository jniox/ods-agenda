package events

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProducer_Kafka(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Backend:      "kafka",
		KafkaBrokers: []string{"localhost:9092"},
	}
	p, err := NewProducer(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	defer p.Close()

	_, ok := p.(*KafkaProducer)
	assert.True(t, ok, "expected KafkaProducer when Backend=kafka")
}

func TestNewProducer_EmptyDefaultsToKafka(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Backend:      "",
		KafkaBrokers: []string{"localhost:9092"},
	}
	p, err := NewProducer(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	defer p.Close()

	_, ok := p.(*KafkaProducer)
	assert.True(t, ok, "expected KafkaProducer when Backend is empty")
}

func TestNewProducer_UnknownBackend(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Backend: "rabbitmq",
	}
	p, err := NewProducer(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "unknown event bus backend")
}

func TestNewProducer_Pubsub(t *testing.T) {
	if os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		t.Skip("PUBSUB_EMULATOR_HOST not set — skipping Pub/Sub factory test")
	}

	ctx := context.Background()
	cfg := Config{
		Backend:      "pubsub",
		GCPProjectID: "test-project",
		PubsubTopic:  "factory-test-topic",
	}
	p, err := NewProducer(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	defer p.Close()

	_, ok := p.(*PubsubProducer)
	assert.True(t, ok, "expected PubsubProducer when Backend=pubsub")
}
