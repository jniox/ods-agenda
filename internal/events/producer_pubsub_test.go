package events

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubsubProducer_Publish(t *testing.T) {
	emulatorHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("PUBSUB_EMULATOR_HOST not set — skipping Pub/Sub emulator test")
	}

	ctx := context.Background()
	projectID := "test-project"

	// Setup: create topic + subscription via admin client
	adminClient, err := pubsub.NewClient(ctx, projectID)
	require.NoError(t, err)
	defer adminClient.Close()

	topicID := "test-topic-" + uuid.New().String()[:8]
	topic, err := adminClient.CreateTopic(ctx, topicID)
	require.NoError(t, err)
	defer topic.Delete(ctx) //nolint:errcheck

	sub, err := adminClient.CreateSubscription(ctx, topicID+"-sub", pubsub.SubscriptionConfig{
		Topic:                 topic,
		EnableMessageOrdering: true,
	})
	require.NoError(t, err)
	defer sub.Delete(ctx) //nolint:errcheck

	// Create producer under test
	producer, err := NewPubsubProducer(ctx, projectID, topicID)
	require.NoError(t, err)
	defer producer.Close()

	// Publish a test event
	tenantID := uuid.New()
	eventType := TypeEventCreated
	source := "/agenda/events"
	eventData := map[string]string{"event_id": uuid.New().String(), "title": "Test Meeting"}

	err = producer.Publish(ctx, tenantID, eventType, source, eventData)
	require.NoError(t, err)

	// Give the async publish time to complete
	time.Sleep(500 * time.Millisecond)

	// Pull message from subscription
	recvCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var receivedMsg *pubsub.Message
	err = sub.Receive(recvCtx, func(_ context.Context, msg *pubsub.Message) {
		receivedMsg = msg
		msg.Ack()
		cancel()
	})
	// Receive returns when context is cancelled — not an error
	if err != nil && err != context.Canceled {
		require.NoError(t, err)
	}

	require.NotNil(t, receivedMsg, "expected to receive a message from subscription")

	// Assert Pub/Sub message attributes match CloudEvents spec (AC-003)
	assert.Equal(t, "1.0", receivedMsg.Attributes["ce-specversion"])
	assert.Equal(t, eventType, receivedMsg.Attributes["ce-type"])
	assert.Equal(t, source, receivedMsg.Attributes["ce-source"])
	assert.Equal(t, "application/json", receivedMsg.Attributes["ce-datacontenttype"])
	assert.Equal(t, tenantID.String(), receivedMsg.Attributes["tenant_id"])
	assert.NotEmpty(t, receivedMsg.Attributes["ce-id"])
	assert.NotEmpty(t, receivedMsg.Attributes["ce-time"])

	// Assert ordering key is tenant_id
	assert.Equal(t, tenantID.String(), receivedMsg.OrderingKey)

	// Assert body is valid CloudEvent JSON (AC-002)
	var ce CloudEvent
	err = json.Unmarshal(receivedMsg.Data, &ce)
	require.NoError(t, err)
	assert.Equal(t, "1.0", ce.SpecVersion)
	assert.Equal(t, eventType, ce.Type)
	assert.Equal(t, source, ce.Source)
	assert.Equal(t, tenantID.String(), ce.TenantID)
	assert.Equal(t, "application/json", ce.DataContentType)
	assert.NotEmpty(t, ce.ID)
	assert.NotEmpty(t, ce.Time)
}

func TestPubsubProducer_PublishFireAndForget(t *testing.T) {
	emulatorHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("PUBSUB_EMULATOR_HOST not set — skipping Pub/Sub emulator test")
	}

	ctx := context.Background()
	projectID := "test-project"

	// Create producer pointing to a non-existent topic
	// The Publish call should not return an error (fire-and-forget, AC-004)
	producer, err := NewPubsubProducer(ctx, projectID, "nonexistent-topic")
	require.NoError(t, err)
	defer producer.Close()

	tenantID := uuid.New()
	err = producer.Publish(ctx, tenantID, TypeEventCreated, "/agenda/events", map[string]string{"key": "val"})
	assert.NoError(t, err, "Publish should not propagate errors (fire-and-forget)")
}

func TestPubsubProducer_Close(t *testing.T) {
	emulatorHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("PUBSUB_EMULATOR_HOST not set — skipping Pub/Sub emulator test")
	}

	ctx := context.Background()
	producer, err := NewPubsubProducer(ctx, "test-project", "any-topic")
	require.NoError(t, err)

	err = producer.Close()
	assert.NoError(t, err, "Close should flush and dispose cleanly (AC-006)")
}
