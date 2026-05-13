package events

import (
	"context"
	"fmt"
)

// Config holds event bus configuration.
type Config struct {
	Backend        string   // EVENT_BUS env: "pubsub" | "kafka" | ""
	GCPProjectID   string   // GCP_PROJECT_ID
	PubsubTopic    string   // PUBSUB_TOPIC (e.g. "agenda-events")
	PubsubTopicDLQ string   // PUBSUB_TOPIC_DLQ (e.g. "agenda-events-dlq")
	KafkaBrokers   []string // REDPANDA_BROKERS (legacy)
}

// NewProducer creates a Producer based on the configured backend.
// When Backend is "pubsub", a Cloud Pub/Sub producer is returned.
// When Backend is "kafka" or empty, a Kafka producer is returned (backward-compat).
func NewProducer(ctx context.Context, cfg Config) (Producer, error) {
	switch cfg.Backend {
	case "pubsub":
		return NewPubsubProducer(ctx, cfg.GCPProjectID, cfg.PubsubTopic)
	case "kafka", "":
		return NewKafkaProducer(cfg.KafkaBrokers), nil
	default:
		return nil, fmt.Errorf("unknown event bus backend: %s", cfg.Backend)
	}
}
