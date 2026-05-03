package domain

import (
	"time"

	"github.com/google/uuid"
)

type EventStatus string

const (
	EventStatusConfirmed EventStatus = "confirmed"
	EventStatusTentative EventStatus = "tentative"
	EventStatusCancelled EventStatus = "cancelled"
)

func (s EventStatus) IsValid() bool {
	switch s {
	case EventStatusConfirmed, EventStatusTentative, EventStatusCancelled:
		return true
	}
	return false
}

// Event represents a calendar event within a tenant.
type Event struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	CalendarID     uuid.UUID
	Title          string
	Description    string
	Location       string
	StartTime      time.Time
	EndTime        time.Time
	AllDay         bool
	Timezone       string
	Status         EventStatus
	RecurrenceRule string
	CreatedBy      uuid.UUID
	DeletedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewEvent(tenantID, calendarID, createdBy uuid.UUID, title, description, location string, startTime, endTime time.Time, allDay bool, timezone, recurrenceRule string) (*Event, error) {
	if tenantID == uuid.Nil {
		return nil, NewValidationError("tenant_id", "is required")
	}
	if calendarID == uuid.Nil {
		return nil, NewValidationError("calendar_id", "is required")
	}
	if createdBy == uuid.Nil {
		return nil, NewValidationError("created_by", "is required")
	}
	if title == "" {
		return nil, NewValidationError("title", "is required")
	}
	if !endTime.After(startTime) {
		return nil, NewValidationError("end_time", "must be after start_time")
	}
	if timezone == "" {
		timezone = "UTC"
	}

	now := time.Now().UTC()
	return &Event{
		ID:             uuid.New(),
		TenantID:       tenantID,
		CalendarID:     calendarID,
		Title:          title,
		Description:    description,
		Location:       location,
		StartTime:      startTime,
		EndTime:        endTime,
		AllDay:         allDay,
		Timezone:       timezone,
		Status:         EventStatusConfirmed,
		RecurrenceRule: recurrenceRule,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (e *Event) Cancel() {
	e.Status = EventStatusCancelled
	now := time.Now().UTC()
	e.DeletedAt = &now
	e.UpdatedAt = now
}
