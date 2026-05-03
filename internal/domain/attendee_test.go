package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAttendee_ValidInput(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()
	userID := uuid.New()

	att, err := NewAttendee(tenantID, eventID, userID, "user@example.com", AttendeeRoleRequired)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, att.ID)
	assert.Equal(t, tenantID, att.TenantID)
	assert.Equal(t, eventID, att.EventID)
	assert.Equal(t, userID, att.UserID)
	assert.Equal(t, "user@example.com", att.Email)
	assert.Equal(t, AttendeeStatusPending, att.Status)
	assert.Equal(t, AttendeeRoleRequired, att.Role)
	assert.Nil(t, att.RespondedAt)
}

func TestNewAttendee_MissingTenantID(t *testing.T) {
	_, err := NewAttendee(uuid.Nil, uuid.New(), uuid.New(), "user@example.com", AttendeeRoleRequired)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewAttendee_MissingEventID(t *testing.T) {
	_, err := NewAttendee(uuid.New(), uuid.Nil, uuid.New(), "user@example.com", AttendeeRoleRequired)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewAttendee_MissingEmail(t *testing.T) {
	_, err := NewAttendee(uuid.New(), uuid.New(), uuid.New(), "", AttendeeRoleRequired)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewAttendee_InvalidRole(t *testing.T) {
	_, err := NewAttendee(uuid.New(), uuid.New(), uuid.New(), "user@example.com", AttendeeRole("invalid"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestAttendee_Respond(t *testing.T) {
	att, _ := NewAttendee(uuid.New(), uuid.New(), uuid.New(), "user@example.com", AttendeeRoleRequired)

	err := att.Respond(AttendeeStatusAccepted)
	require.NoError(t, err)
	assert.Equal(t, AttendeeStatusAccepted, att.Status)
	assert.NotNil(t, att.RespondedAt)
}

func TestAttendee_Respond_InvalidStatus(t *testing.T) {
	att, _ := NewAttendee(uuid.New(), uuid.New(), uuid.New(), "user@example.com", AttendeeRoleRequired)

	err := att.Respond(AttendeeStatus("invalid"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestAttendeeStatus_Valid(t *testing.T) {
	assert.True(t, AttendeeStatusPending.IsValid())
	assert.True(t, AttendeeStatusAccepted.IsValid())
	assert.True(t, AttendeeStatusDeclined.IsValid())
	assert.True(t, AttendeeStatusTentative.IsValid())
	assert.False(t, AttendeeStatus("invalid").IsValid())
}

func TestAttendeeRole_Valid(t *testing.T) {
	assert.True(t, AttendeeRoleOrganizer.IsValid())
	assert.True(t, AttendeeRoleRequired.IsValid())
	assert.True(t, AttendeeRoleOptional.IsValid())
	assert.False(t, AttendeeRole("invalid").IsValid())
}
