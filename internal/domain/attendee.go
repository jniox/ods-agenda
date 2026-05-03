package domain

import (
	"time"

	"github.com/google/uuid"
)

type AttendeeStatus string

const (
	AttendeeStatusPending   AttendeeStatus = "pending"
	AttendeeStatusAccepted  AttendeeStatus = "accepted"
	AttendeeStatusDeclined  AttendeeStatus = "declined"
	AttendeeStatusTentative AttendeeStatus = "tentative"
)

func (s AttendeeStatus) IsValid() bool {
	switch s {
	case AttendeeStatusPending, AttendeeStatusAccepted, AttendeeStatusDeclined, AttendeeStatusTentative:
		return true
	}
	return false
}

type AttendeeRole string

const (
	AttendeeRoleOrganizer AttendeeRole = "organizer"
	AttendeeRoleRequired  AttendeeRole = "required"
	AttendeeRoleOptional  AttendeeRole = "optional"
)

func (r AttendeeRole) IsValid() bool {
	switch r {
	case AttendeeRoleOrganizer, AttendeeRoleRequired, AttendeeRoleOptional:
		return true
	}
	return false
}

// Attendee represents an event participant within a tenant.
type Attendee struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	EventID     uuid.UUID
	UserID      uuid.UUID
	Email       string
	Status      AttendeeStatus
	Role        AttendeeRole
	RespondedAt *time.Time
	CreatedAt   time.Time
}

func NewAttendee(tenantID, eventID, userID uuid.UUID, email string, role AttendeeRole) (*Attendee, error) {
	if tenantID == uuid.Nil {
		return nil, NewValidationError("tenant_id", "is required")
	}
	if eventID == uuid.Nil {
		return nil, NewValidationError("event_id", "is required")
	}
	if email == "" {
		return nil, NewValidationError("email", "is required")
	}
	if !role.IsValid() {
		return nil, NewValidationError("role", "must be organizer, required, or optional")
	}

	return &Attendee{
		ID:        uuid.New(),
		TenantID:  tenantID,
		EventID:   eventID,
		UserID:    userID,
		Email:     email,
		Status:    AttendeeStatusPending,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (a *Attendee) Respond(status AttendeeStatus) error {
	if !status.IsValid() {
		return NewValidationError("status", "must be pending, accepted, declined, or tentative")
	}
	a.Status = status
	now := time.Now().UTC()
	a.RespondedAt = &now
	return nil
}
