package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

const (
	TopicAgendaEvents = "agenda.events"

	TypeEventCreated      = "agenda.event.created"
	TypeEventUpdated      = "agenda.event.updated"
	TypeEventCancelled    = "agenda.event.cancelled"
	TypeAttendeeResponded = "agenda.attendee.responded"
)

// CloudEvent represents a CloudEvents v1.0 envelope.
type CloudEvent struct {
	SpecVersion     string      `json:"specversion"`
	ID              string      `json:"id"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	Time            string      `json:"time"`
	DataContentType string      `json:"datacontenttype"`
	TenantID        string      `json:"tenantid"`
	Data            interface{} `json:"data"`
}

// Producer publishes CloudEvents to Redpanda/Kafka.
type Producer interface {
	Publish(ctx context.Context, tenantID uuid.UUID, eventType, source string, data interface{}) error
	Close() error
}

// KafkaProducer implements Producer using kafka-go.
type KafkaProducer struct {
	writer *kafka.Writer
}

// NewKafkaProducer creates a new Kafka/Redpanda producer.
func NewKafkaProducer(brokers []string) *KafkaProducer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        TopicAgendaEvents,
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Lz4,
	}
	return &KafkaProducer{writer: w}
}

// Publish sends a CloudEvent to the configured topic.
func (p *KafkaProducer) Publish(ctx context.Context, tenantID uuid.UUID, eventType, source string, data interface{}) error {
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
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(tenantID.String()),
		Value: payload,
	})
}

// Close shuts down the producer.
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

// NoopProducer is a no-op producer for testing.
type NoopProducer struct {
	Published []CloudEvent
}

// Publish records the event without sending it.
func (p *NoopProducer) Publish(_ context.Context, tenantID uuid.UUID, eventType, source string, data interface{}) error {
	p.Published = append(p.Published, CloudEvent{
		SpecVersion:     "1.0",
		ID:              uuid.New().String(),
		Type:            eventType,
		Source:          source,
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		TenantID:        tenantID.String(),
		Data:            data,
	})
	return nil
}

// Close is a no-op.
func (p *NoopProducer) Close() error { return nil }
