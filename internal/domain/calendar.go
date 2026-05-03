package domain

import (
	"time"

	"github.com/google/uuid"
)

const DefaultCalendarColor = "#3B82F6"

// Calendar represents a user's calendar within a tenant.
type Calendar struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	Color       string
	OwnerID     uuid.UUID
	IsDefault   bool
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewCalendar(tenantID, ownerID uuid.UUID, name, description, color string) (*Calendar, error) {
	if tenantID == uuid.Nil {
		return nil, NewValidationError("tenant_id", "is required")
	}
	if ownerID == uuid.Nil {
		return nil, NewValidationError("owner_id", "is required")
	}
	if name == "" {
		return nil, NewValidationError("name", "is required")
	}
	if color == "" {
		color = DefaultCalendarColor
	}

	now := time.Now().UTC()
	return &Calendar{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        name,
		Description: description,
		Color:       color,
		OwnerID:     ownerID,
		IsDefault:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (c *Calendar) Validate() error {
	if c.TenantID == uuid.Nil {
		return NewValidationError("tenant_id", "is required")
	}
	if c.OwnerID == uuid.Nil {
		return NewValidationError("owner_id", "is required")
	}
	if c.Name == "" {
		return NewValidationError("name", "is required")
	}
	return nil
}
