package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent_ValidInput(t *testing.T) {
	tenantID := uuid.New()
	calendarID := uuid.New()
	createdBy := uuid.New()
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(2 * time.Hour)

	ev, err := NewEvent(tenantID, calendarID, createdBy, "Meeting", "Standup", "Room A", start, end, false, "Europe/Paris", "")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, ev.ID)
	assert.Equal(t, tenantID, ev.TenantID)
	assert.Equal(t, calendarID, ev.CalendarID)
	assert.Equal(t, "Meeting", ev.Title)
	assert.Equal(t, EventStatusConfirmed, ev.Status)
	assert.Equal(t, "Europe/Paris", ev.Timezone)
	assert.False(t, ev.CreatedAt.IsZero())
}

func TestNewEvent_MissingTenantID(t *testing.T) {
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(2 * time.Hour)
	_, err := NewEvent(uuid.Nil, uuid.New(), uuid.New(), "Meeting", "", "", start, end, false, "UTC", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewEvent_MissingTitle(t *testing.T) {
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(2 * time.Hour)
	_, err := NewEvent(uuid.New(), uuid.New(), uuid.New(), "", "", "", start, end, false, "UTC", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewEvent_EndBeforeStart(t *testing.T) {
	start := time.Now().Add(2 * time.Hour)
	end := time.Now().Add(1 * time.Hour)
	_, err := NewEvent(uuid.New(), uuid.New(), uuid.New(), "Meeting", "", "", start, end, false, "UTC", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewEvent_DefaultTimezone(t *testing.T) {
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(2 * time.Hour)
	ev, err := NewEvent(uuid.New(), uuid.New(), uuid.New(), "Meeting", "", "", start, end, false, "", "")
	require.NoError(t, err)
	assert.Equal(t, "UTC", ev.Timezone)
}

func TestEvent_Cancel(t *testing.T) {
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(2 * time.Hour)
	ev, _ := NewEvent(uuid.New(), uuid.New(), uuid.New(), "Meeting", "", "", start, end, false, "UTC", "")
	ev.Cancel()
	assert.Equal(t, EventStatusCancelled, ev.Status)
	assert.NotNil(t, ev.DeletedAt)
}

func TestEventStatus_Valid(t *testing.T) {
	assert.True(t, EventStatusConfirmed.IsValid())
	assert.True(t, EventStatusTentative.IsValid())
	assert.True(t, EventStatusCancelled.IsValid())
	assert.False(t, EventStatus("invalid").IsValid())
}
