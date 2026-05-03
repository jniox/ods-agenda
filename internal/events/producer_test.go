package events

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopProducer_Publish(t *testing.T) {
	p := &NoopProducer{}
	tenantID := uuid.New()
	data := map[string]string{"event_id": uuid.New().String()}

	err := p.Publish(context.Background(), tenantID, TypeEventCreated, "/agenda/events", data)
	require.NoError(t, err)
	require.Len(t, p.Published, 1)

	ce := p.Published[0]
	assert.Equal(t, "1.0", ce.SpecVersion)
	assert.Equal(t, TypeEventCreated, ce.Type)
	assert.Equal(t, "/agenda/events", ce.Source)
	assert.Equal(t, tenantID.String(), ce.TenantID)
	assert.Equal(t, "application/json", ce.DataContentType)
}

func TestNoopProducer_Close(t *testing.T) {
	p := &NoopProducer{}
	assert.NoError(t, p.Close())
}

func TestCloudEventConstants(t *testing.T) {
	assert.Equal(t, "agenda.events", TopicAgendaEvents)
	assert.Equal(t, "ods.agenda.event.created", TypeEventCreated)
	assert.Equal(t, "ods.agenda.event.updated", TypeEventUpdated)
	assert.Equal(t, "ods.agenda.event.cancelled", TypeEventCancelled)
	assert.Equal(t, "ods.agenda.attendee.responded", TypeAttendeeResponded)
	assert.Equal(t, "ods.agenda.calendar.created", TypeCalendarCreated)
	assert.Equal(t, "ods.agenda.calendar.updated", TypeCalendarUpdated)
	assert.Equal(t, "ods.agenda.calendar.deleted", TypeCalendarDeleted)
}
