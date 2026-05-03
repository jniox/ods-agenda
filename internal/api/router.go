package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/orbus-digital/agenda/internal/auth"
	"github.com/orbus-digital/agenda/internal/domain"
	"github.com/orbus-digital/agenda/internal/events"
	"github.com/orbus-digital/agenda/internal/repository"
)

// NewRouter creates the Chi router with all endpoints registered.
func NewRouter(db *repository.DB, producer events.Producer, jwtSecret string) chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Repositories
	calendarRepo := repository.NewCalendarRepo(db)
	eventRepo := repository.NewEventRepo(db)
	attendeeRepo := repository.NewAttendeeRepo(db)
	availabilityRepo := repository.NewAvailabilityRepo(db)

	// Handlers
	healthHandler := NewHealthHandler(db)
	calendarHandler := NewCalendarHandler(calendarRepo, producer)
	eventHandler := NewEventHandler(eventRepo, calendarRepo, attendeeRepo, producer)
	attendeeHandler := NewAttendeeHandler(attendeeRepo, eventRepo, producer)
	availabilityHandler := NewAvailabilityHandler(availabilityRepo)

	// Auth middleware
	jwtMw := auth.NewJWTMiddleware(jwtSecret)

	// Public endpoints
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// Protected endpoints
	r.Group(func(r chi.Router) {
		r.Use(jwtMw.Handler)

		// Calendars
		r.Route("/api/v1/calendars", func(r chi.Router) {
			r.Post("/", calendarHandler.Create)
			r.Get("/", calendarHandler.List)
			r.Get("/{id}", calendarHandler.GetByID)
			r.Put("/{id}", calendarHandler.Update)
			r.Delete("/{id}", calendarHandler.Delete)

			// Events under calendar
			r.Post("/{calendar_id}/events", eventHandler.Create)
			r.Get("/{calendar_id}/events", eventHandler.ListByCalendar)
		})

		// Events (cross-calendar)
		r.Route("/api/v1/events", func(r chi.Router) {
			r.Get("/", eventHandler.ListByTenant)
			r.Get("/{id}", eventHandler.GetByID)
			r.Put("/{id}", eventHandler.Update)
			r.Delete("/{id}", eventHandler.Cancel)

			// Attendees under event
			r.Post("/{event_id}/attendees", attendeeHandler.AddAttendee)
			r.Put("/{event_id}/attendees/{attendee_id}/respond", attendeeHandler.Respond)
			r.Delete("/{event_id}/attendees/{attendee_id}", attendeeHandler.RemoveAttendee)
		})

		// Availability
		r.Post("/api/v1/availability", availabilityHandler.CheckAvailability)
	})

	return r
}

// Verify that repository types implement the domain interfaces at compile time.
var _ domain.CalendarRepository = (*repository.CalendarRepo)(nil)
var _ domain.EventRepository = (*repository.EventRepo)(nil)
var _ domain.AttendeeRepository = (*repository.AttendeeRepo)(nil)
var _ domain.AvailabilityRepository = (*repository.AvailabilityRepo)(nil)
