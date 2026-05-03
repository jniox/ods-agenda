package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/orbus-digital/agenda/internal/domain"
)

// mockCalendarRepo is an in-memory mock for domain.CalendarRepository.
type mockCalendarRepo struct {
	mu        sync.Mutex
	calendars map[uuid.UUID]*domain.Calendar
}

func newMockCalendarRepo() *mockCalendarRepo {
	return &mockCalendarRepo{calendars: make(map[uuid.UUID]*domain.Calendar)}
}

func (m *mockCalendarRepo) Create(_ context.Context, cal *domain.Calendar) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Check unique constraint: tenant_id + owner_id + name
	for _, c := range m.calendars {
		if c.TenantID == cal.TenantID && c.OwnerID == cal.OwnerID && c.Name == cal.Name && c.DeletedAt == nil {
			return fmt.Errorf("%w: calendar name already exists for this owner", domain.ErrConflict)
		}
	}
	m.calendars[cal.ID] = cal
	return nil
}

func (m *mockCalendarRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Calendar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cal, ok := m.calendars[id]
	if !ok || cal.DeletedAt != nil || cal.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return cal, nil
}

func (m *mockCalendarRepo) List(_ context.Context, tenantID uuid.UUID, ownerID *uuid.UUID, limit, offset int) ([]*domain.Calendar, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Calendar
	for _, c := range m.calendars {
		if c.TenantID != tenantID || c.DeletedAt != nil {
			continue
		}
		if ownerID != nil && c.OwnerID != *ownerID {
			continue
		}
		result = append(result, c)
	}
	total := len(result)
	if offset >= len(result) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockCalendarRepo) Update(_ context.Context, cal *domain.Calendar) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.calendars[cal.ID]
	if !ok || existing.DeletedAt != nil {
		return domain.ErrNotFound
	}
	m.calendars[cal.ID] = cal
	return nil
}

func (m *mockCalendarRepo) SoftDelete(_ context.Context, tenantID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cal, ok := m.calendars[id]
	if !ok || cal.DeletedAt != nil || cal.TenantID != tenantID {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	cal.DeletedAt = &now
	return nil
}

// mockEventRepo is an in-memory mock for domain.EventRepository.
type mockEventRepo struct {
	mu     sync.Mutex
	events map[uuid.UUID]*domain.Event
}

func newMockEventRepo() *mockEventRepo {
	return &mockEventRepo{events: make(map[uuid.UUID]*domain.Event)}
}

func (m *mockEventRepo) Create(_ context.Context, ev *domain.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[ev.ID] = ev
	return nil
}

func (m *mockEventRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev, ok := m.events[id]
	if !ok || ev.DeletedAt != nil || ev.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return ev, nil
}

func (m *mockEventRepo) ListByCalendar(_ context.Context, tenantID, calendarID uuid.UUID, start, end time.Time, status *domain.EventStatus, limit, offset int) ([]*domain.Event, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Event
	for _, ev := range m.events {
		if ev.TenantID != tenantID || ev.CalendarID != calendarID || ev.DeletedAt != nil {
			continue
		}
		if ev.StartTime.After(end) || ev.EndTime.Before(start) {
			continue
		}
		if status != nil && ev.Status != *status {
			continue
		}
		result = append(result, ev)
	}
	total := len(result)
	if offset >= len(result) {
		return nil, total, nil
	}
	e := offset + limit
	if e > len(result) {
		e = len(result)
	}
	return result[offset:e], total, nil
}

func (m *mockEventRepo) ListByTenant(_ context.Context, tenantID uuid.UUID, calendarID, createdBy *uuid.UUID, start, end time.Time, status *domain.EventStatus, limit, offset int) ([]*domain.Event, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Event
	for _, ev := range m.events {
		if ev.TenantID != tenantID || ev.DeletedAt != nil {
			continue
		}
		if ev.StartTime.After(end) || ev.EndTime.Before(start) {
			continue
		}
		if calendarID != nil && ev.CalendarID != *calendarID {
			continue
		}
		if createdBy != nil && ev.CreatedBy != *createdBy {
			continue
		}
		if status != nil && ev.Status != *status {
			continue
		}
		result = append(result, ev)
	}
	total := len(result)
	if offset >= len(result) {
		return nil, total, nil
	}
	e := offset + limit
	if e > len(result) {
		e = len(result)
	}
	return result[offset:e], total, nil
}

func (m *mockEventRepo) Update(_ context.Context, ev *domain.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.events[ev.ID]
	if !ok || existing.DeletedAt != nil {
		return domain.ErrNotFound
	}
	m.events[ev.ID] = ev
	return nil
}

func (m *mockEventRepo) Cancel(_ context.Context, tenantID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev, ok := m.events[id]
	if !ok || ev.DeletedAt != nil || ev.TenantID != tenantID {
		return domain.ErrNotFound
	}
	ev.Cancel()
	return nil
}

// mockAttendeeRepo is an in-memory mock for domain.AttendeeRepository.
type mockAttendeeRepo struct {
	mu        sync.Mutex
	attendees map[uuid.UUID]*domain.Attendee
}

func newMockAttendeeRepo() *mockAttendeeRepo {
	return &mockAttendeeRepo{attendees: make(map[uuid.UUID]*domain.Attendee)}
}

func (m *mockAttendeeRepo) Create(_ context.Context, att *domain.Attendee) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.attendees {
		if a.EventID == att.EventID && a.Email == att.Email {
			return fmt.Errorf("%w: attendee already exists for this event", domain.ErrConflict)
		}
	}
	m.attendees[att.ID] = att
	return nil
}

func (m *mockAttendeeRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Attendee, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	att, ok := m.attendees[id]
	if !ok || att.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return att, nil
}

func (m *mockAttendeeRepo) ListByEvent(_ context.Context, tenantID, eventID uuid.UUID) ([]*domain.Attendee, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Attendee
	for _, a := range m.attendees {
		if a.TenantID == tenantID && a.EventID == eventID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockAttendeeRepo) UpdateStatus(_ context.Context, att *domain.Attendee) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.attendees[att.ID]
	if !ok {
		return domain.ErrNotFound
	}
	existing.Status = att.Status
	existing.RespondedAt = att.RespondedAt
	return nil
}

func (m *mockAttendeeRepo) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	att, ok := m.attendees[id]
	if !ok || att.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(m.attendees, id)
	return nil
}

// mockAvailabilityRepo is an in-memory mock for domain.AvailabilityRepository.
type mockAvailabilityRepo struct {
	slots map[uuid.UUID][]domain.BusySlot
}

func newMockAvailabilityRepo() *mockAvailabilityRepo {
	return &mockAvailabilityRepo{slots: make(map[uuid.UUID][]domain.BusySlot)}
}

func (m *mockAvailabilityRepo) GetBusySlots(_ context.Context, _ uuid.UUID, userIDs []uuid.UUID, _, _ time.Time) (map[uuid.UUID][]domain.BusySlot, error) {
	result := make(map[uuid.UUID][]domain.BusySlot)
	for _, uid := range userIDs {
		if slots, ok := m.slots[uid]; ok {
			result[uid] = slots
		} else {
			result[uid] = []domain.BusySlot{}
		}
	}
	return result, nil
}
