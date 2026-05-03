package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CalendarRepository defines the persistence interface for calendars.
type CalendarRepository interface {
	Create(ctx context.Context, cal *Calendar) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Calendar, error)
	List(ctx context.Context, tenantID uuid.UUID, ownerID *uuid.UUID, limit, offset int) ([]*Calendar, int, error)
	Update(ctx context.Context, cal *Calendar) error
	SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error
}

// EventRepository defines the persistence interface for events.
type EventRepository interface {
	Create(ctx context.Context, ev *Event) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Event, error)
	ListByCalendar(ctx context.Context, tenantID, calendarID uuid.UUID, start, end time.Time, status *EventStatus, limit, offset int) ([]*Event, int, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, calendarID, createdBy *uuid.UUID, start, end time.Time, status *EventStatus, limit, offset int) ([]*Event, int, error)
	Update(ctx context.Context, ev *Event) error
	Cancel(ctx context.Context, tenantID, id uuid.UUID) error
}

// AttendeeRepository defines the persistence interface for attendees.
type AttendeeRepository interface {
	Create(ctx context.Context, att *Attendee) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Attendee, error)
	ListByEvent(ctx context.Context, tenantID, eventID uuid.UUID) ([]*Attendee, error)
	UpdateStatus(ctx context.Context, att *Attendee) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// BusySlot represents a time slot where a user is busy.
type BusySlot struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	EventID uuid.UUID `json:"event_id"`
	Title   string    `json:"title"`
}

// AvailabilityRepository provides availability query capability.
type AvailabilityRepository interface {
	GetBusySlots(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID, start, end time.Time) (map[uuid.UUID][]BusySlot, error)
}
