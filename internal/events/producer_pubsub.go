package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// PubsubProducer implements Producer using Google Cloud Pub/Sub (v1 GA client).
// Migration to v2 deferred until pubsub/v2 exits beta.
type PubsubProducer struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

// NewPubsubProducer creates a new Cloud Pub/Sub producer.
func NewPubsubProducer(ctx context.Context, projectID, topicID string) (*PubsubProducer, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub client: %w", err)
	}
	topic := client.Topic(topicID)
	topic.EnableMessageOrdering = true
	return &PubsubProducer{client: client, topic: topic}, nil
}

// Publish sends a CloudEvent to the configured Pub/Sub topic.
// Failures are logged at ERROR level but not propagated (fire-and-forget).
func (p *PubsubProducer) Publish(ctx context.Context, tenantID uuid.UUID, eventType, source string, data interface{}) error {
	ce := CloudEvent{
		SpecVersion:     "1.0",
		ID:              uuid.New().String(),
		Type:            eventType,
		Source:          source,
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		TenantID:        tenantID.String(),
		Data:            data,
	}

	payload, err := json.Marshal(ce)
	if err != nil {
		log.Error().
			Err(err).
			Str("tenant_id", tenantID.String()).
			Str("event_type", eventType).
			Msg("pubsub: failed to marshal CloudEvent")
		return nil // fire-and-forget
	}

	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

	result := p.topic.Publish(publishCtx, &pubsub.Message{
		Data: payload,
		Attributes: map[string]string{
			"ce-specversion":     "1.0",
			"ce-id":              ce.ID,
			"ce-type":            eventType,
			"ce-source":          source,
			"ce-time":            ce.Time,
			"ce-datacontenttype": "application/json",
			"tenant_id":          tenantID.String(),
		},
		OrderingKey: tenantID.String(),
	})

	// Wait for ack asynchronously — fire-and-forget
	go func() {
		defer cancel()
		if _, err := result.Get(publishCtx); err != nil {
			log.Error().
				Err(err).
				Str("tenant_id", tenantID.String()).
				Str("event_type", eventType).
				Msg("pubsub publish failed")
		}
	}()

	return nil
}

// Close flushes pending publishes and disposes the Pub/Sub client.
func (p *PubsubProducer) Close() error {
	p.topic.Stop()
	return p.client.Close()
}
