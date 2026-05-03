package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCalendar_ValidInput(t *testing.T) {
	tenantID := uuid.New()
	ownerID := uuid.New()

	cal, err := NewCalendar(tenantID, ownerID, "Work", "Work calendar", "#3B82F6")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, cal.ID)
	assert.Equal(t, tenantID, cal.TenantID)
	assert.Equal(t, ownerID, cal.OwnerID)
	assert.Equal(t, "Work", cal.Name)
	assert.Equal(t, "Work calendar", cal.Description)
	assert.Equal(t, "#3B82F6", cal.Color)
	assert.False(t, cal.IsDefault)
	assert.False(t, cal.CreatedAt.IsZero())
	assert.False(t, cal.UpdatedAt.IsZero())
	assert.Nil(t, cal.DeletedAt)
}

func TestNewCalendar_MissingTenantID(t *testing.T) {
	_, err := NewCalendar(uuid.Nil, uuid.New(), "Work", "", "#3B82F6")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewCalendar_MissingName(t *testing.T) {
	_, err := NewCalendar(uuid.New(), uuid.New(), "", "", "#3B82F6")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewCalendar_MissingOwnerID(t *testing.T) {
	_, err := NewCalendar(uuid.New(), uuid.Nil, "Work", "", "#3B82F6")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestNewCalendar_DefaultColor(t *testing.T) {
	cal, err := NewCalendar(uuid.New(), uuid.New(), "Work", "", "")
	require.NoError(t, err)
	assert.Equal(t, DefaultCalendarColor, cal.Color)
}

func TestCalendar_Validate(t *testing.T) {
	cal := &Calendar{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		OwnerID:  uuid.New(),
		Name:     "Valid",
	}
	assert.NoError(t, cal.Validate())

	cal.Name = ""
	assert.ErrorIs(t, cal.Validate(), ErrValidation)
}
