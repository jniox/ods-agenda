package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orbus-digital/agenda/internal/auth"
	"github.com/orbus-digital/agenda/internal/domain"
	"github.com/orbus-digital/agenda/internal/events"
)

// EventHandler handles event REST endpoints.
type EventHandler struct {
	eventRepo    domain.EventRepository
	calendarRepo domain.CalendarRepository
	attendeeRepo domain.AttendeeRepository
	producer     events.Producer
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(er domain.EventRepository, cr domain.CalendarRepository, ar domain.AttendeeRepository, p events.Producer) *EventHandler {
	return &EventHandler{eventRepo: er, calendarRepo: cr, attendeeRepo: ar, producer: p}
}

type createEventRequest struct {
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Location       string                 `json:"location"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	AllDay         bool                   `json:"all_day"`
	Timezone       string                 `json:"timezone"`
	RecurrenceRule string                 `json:"recurrence_rule"`
	Attendees      []createAttendeeInput  `json:"attendees"`
}

type createAttendeeInput struct {
	Email  string    `json:"email"`
	UserID string    `json:"user_id,omitempty"`
	Role   string    `json:"role"`
}

type updateEventRequest struct {
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Location       string    `json:"location"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	AllDay         bool      `json:"all_day"`
	Timezone       string    `json:"timezone"`
	Status         string    `json:"status"`
}

type eventResponse struct {
	ID             uuid.UUID            `json:"id"`
	TenantID       uuid.UUID            `json:"tenant_id"`
	CalendarID     uuid.UUID            `json:"calendar_id"`
	Title          string               `json:"title"`
	Description    string               `json:"description"`
	Location       string               `json:"location"`
	StartTime      time.Time            `json:"start_time"`
	EndTime        time.Time            `json:"end_time"`
	AllDay         bool                 `json:"all_day"`
	Timezone       string               `json:"timezone"`
	Status         domain.EventStatus   `json:"status"`
	RecurrenceRule string               `json:"recurrence_rule"`
	CreatedBy      uuid.UUID            `json:"created_by"`
	Attendees      []attendeeResponse   `json:"attendees,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type attendeeResponse struct {
	ID          uuid.UUID              `json:"id"`
	TenantID    uuid.UUID              `json:"tenant_id,omitempty"`
	EventID     uuid.UUID              `json:"event_id"`
	UserID      *uuid.UUID             `json:"user_id,omitempty"`
	Email       string                 `json:"email"`
	Status      domain.AttendeeStatus  `json:"status"`
	Role        domain.AttendeeRole    `json:"role"`
	RespondedAt *time.Time             `json:"responded_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

func toEventResponse(ev *domain.Event, attendees []*domain.Attendee) eventResponse {
	resp := eventResponse{
		ID:             ev.ID,
		TenantID:       ev.TenantID,
		CalendarID:     ev.CalendarID,
		Title:          ev.Title,
		Description:    ev.Description,
		Location:       ev.Location,
		StartTime:      ev.StartTime,
		EndTime:        ev.EndTime,
		AllDay:         ev.AllDay,
		Timezone:       ev.Timezone,
		Status:         ev.Status,
		RecurrenceRule: ev.RecurrenceRule,
		CreatedBy:      ev.CreatedBy,
		CreatedAt:      ev.CreatedAt,
		UpdatedAt:      ev.UpdatedAt,
	}
	if attendees != nil {
		resp.Attendees = make([]attendeeResponse, len(attendees))
		for i, a := range attendees {
			resp.Attendees[i] = toAttendeeResponse(a)
		}
	}
	return resp
}

func toAttendeeResponse(a *domain.Attendee) attendeeResponse {
	resp := attendeeResponse{
		ID:          a.ID,
		EventID:     a.EventID,
		Email:       a.Email,
		Status:      a.Status,
		Role:        a.Role,
		RespondedAt: a.RespondedAt,
		CreatedAt:   a.CreatedAt,
	}
	if a.UserID != uuid.Nil {
		resp.UserID = &a.UserID
	}
	return resp
}

// isValidTimezone checks if tz is a valid IANA timezone.
func isValidTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}

// Create handles POST /api/v1/calendars/:calendar_id/events.
func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())
	calendarID, err := uuid.Parse(chi.URLParam(r, "calendar_id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid calendar_id", "calendar_id")
		return
	}

	// Verify calendar exists and belongs to tenant
	_, err = h.calendarRepo.GetByID(r.Context(), tenantID, calendarID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	var req createEventRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", "")
		return
	}

	// BR-EVT-005: validate timezone
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if !isValidTimezone(tz) {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timezone", "timezone")
		return
	}

	// BR-ATT-006: max 50 attendees
	if len(req.Attendees) > 50 {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "attendees: maximum 50 allowed", "attendees")
		return
	}

	ev, err := domain.NewEvent(tenantID, calendarID, userID, req.Title, req.Description, req.Location,
		req.StartTime, req.EndTime, req.AllDay, tz, req.RecurrenceRule)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	if err := h.eventRepo.Create(r.Context(), ev); err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-ATT-002: auto-add creator as organizer
	var attendees []*domain.Attendee
	creatorAtt, err := domain.NewAttendee(tenantID, ev.ID, userID, "", domain.AttendeeRoleOrganizer)
	if err == nil {
		creatorAtt.Status = domain.AttendeeStatusAccepted
		if createErr := h.attendeeRepo.Create(r.Context(), creatorAtt); createErr == nil {
			attendees = append(attendees, creatorAtt)
		}
	}

	// Add other attendees from request
	for _, ai := range req.Attendees {
		role := domain.AttendeeRole(ai.Role)
		if !role.IsValid() {
			role = domain.AttendeeRoleRequired
		}
		var attUserID uuid.UUID
		if ai.UserID != "" {
			attUserID, _ = uuid.Parse(ai.UserID)
		}
		att, err := domain.NewAttendee(tenantID, ev.ID, attUserID, ai.Email, role)
		if err != nil {
			continue
		}
		if err := h.attendeeRepo.Create(r.Context(), att); err != nil {
			continue
		}
		attendees = append(attendees, att)
	}

	// Emit CloudEvent
	h.producer.Publish(r.Context(), tenantID, events.TypeEventCreated, "/agenda/events", map[string]interface{}{
		"event_id":       ev.ID.String(),
		"calendar_id":    ev.CalendarID.String(),
		"title":          ev.Title,
		"start_time":     ev.StartTime.Format(time.RFC3339),
		"end_time":       ev.EndTime.Format(time.RFC3339),
		"timezone":       ev.Timezone,
		"created_by":     ev.CreatedBy.String(),
		"attendee_count": len(attendees),
	})

	writeJSON(w, http.StatusCreated, toEventResponse(ev, attendees))
}

// ListByCalendar handles GET /api/v1/calendars/:calendar_id/events.
func (h *EventHandler) ListByCalendar(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	calendarID, err := uuid.Parse(chi.URLParam(r, "calendar_id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid calendar_id", "calendar_id")
		return
	}

	start, end, ok := parseTimeRange(w, r)
	if !ok {
		return
	}

	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit > 500 {
		limit = 500
	}

	var status *domain.EventStatus
	if s := r.URL.Query().Get("status"); s != "" {
		es := domain.EventStatus(s)
		if !es.IsValid() {
			writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status", "status")
			return
		}
		status = &es
	}

	evs, total, err := h.eventRepo.ListByCalendar(r.Context(), tenantID, calendarID, start, end, status, limit, offset)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	data := make([]eventResponse, len(evs))
	for i, ev := range evs {
		attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, ev.ID)
		data[i] = toEventResponse(ev, attendees)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{Data: data, Total: total, Limit: limit, Offset: offset})
}

// ListByTenant handles GET /api/v1/events.
func (h *EventHandler) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())

	start, end, ok := parseTimeRange(w, r)
	if !ok {
		return
	}

	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit > 500 {
		limit = 500
	}

	var status *domain.EventStatus
	if s := r.URL.Query().Get("status"); s != "" {
		es := domain.EventStatus(s)
		if !es.IsValid() {
			writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status", "status")
			return
		}
		status = &es
	}

	calendarID, err := parseUUIDParam(r.URL.Query().Get("calendar_id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid calendar_id", "calendar_id")
		return
	}

	createdBy, err := parseUUIDParam(r.URL.Query().Get("created_by"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid created_by", "created_by")
		return
	}

	evs, total, err := h.eventRepo.ListByTenant(r.Context(), tenantID, calendarID, createdBy, start, end, status, limit, offset)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	data := make([]eventResponse, len(evs))
	for i, ev := range evs {
		attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, ev.ID)
		data[i] = toEventResponse(ev, attendees)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{Data: data, Total: total, Limit: limit, Offset: offset})
}

// GetByID handles GET /api/v1/events/:id.
func (h *EventHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid event id", "id")
		return
	}

	ev, err := h.eventRepo.GetByID(r.Context(), tenantID, id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, ev.ID)
	writeJSON(w, http.StatusOK, toEventResponse(ev, attendees))
}

// Update handles PUT /api/v1/events/:id.
func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid event id", "id")
		return
	}

	ev, err := h.eventRepo.GetByID(r.Context(), tenantID, id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-EVT-002: only creator or organizer can update
	if !h.canModifyEvent(r, tenantID, userID, ev) {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "only the event creator or organizer can update", "")
		return
	}

	var req updateEventRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", "")
		return
	}

	// Track changed fields for CloudEvent
	var changedFields []string
	if req.Title != "" && req.Title != ev.Title {
		changedFields = append(changedFields, "title")
		ev.Title = req.Title
	}
	if req.Description != ev.Description {
		changedFields = append(changedFields, "description")
		ev.Description = req.Description
	}
	if req.Location != ev.Location {
		changedFields = append(changedFields, "location")
		ev.Location = req.Location
	}
	if !req.StartTime.IsZero() && !req.StartTime.Equal(ev.StartTime) {
		changedFields = append(changedFields, "start_time")
		ev.StartTime = req.StartTime
	}
	if !req.EndTime.IsZero() && !req.EndTime.Equal(ev.EndTime) {
		changedFields = append(changedFields, "end_time")
		ev.EndTime = req.EndTime
	}
	if req.AllDay != ev.AllDay {
		changedFields = append(changedFields, "all_day")
		ev.AllDay = req.AllDay
	}
	if req.Timezone != "" && req.Timezone != ev.Timezone {
		if !isValidTimezone(req.Timezone) {
			writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid timezone", "timezone")
			return
		}
		changedFields = append(changedFields, "timezone")
		ev.Timezone = req.Timezone
	}
	if req.Status != "" && domain.EventStatus(req.Status) != ev.Status {
		s := domain.EventStatus(req.Status)
		if !s.IsValid() {
			writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status", "status")
			return
		}
		changedFields = append(changedFields, "status")
		ev.Status = s
	}

	// Validate end > start
	if !ev.EndTime.After(ev.StartTime) {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "end_time: must be after start_time", "end_time")
		return
	}

	if err := h.eventRepo.Update(r.Context(), ev); err != nil {
		handleDomainError(w, err)
		return
	}

	// Emit CloudEvent
	h.producer.Publish(r.Context(), tenantID, events.TypeEventUpdated, "/agenda/events", map[string]interface{}{
		"event_id":       ev.ID.String(),
		"calendar_id":    ev.CalendarID.String(),
		"title":          ev.Title,
		"start_time":     ev.StartTime.Format(time.RFC3339),
		"end_time":       ev.EndTime.Format(time.RFC3339),
		"timezone":       ev.Timezone,
		"updated_by":     userID.String(),
		"changed_fields": changedFields,
	})

	attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, ev.ID)
	writeJSON(w, http.StatusOK, toEventResponse(ev, attendees))
}

// Cancel handles DELETE /api/v1/events/:id.
func (h *EventHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid event id", "id")
		return
	}

	ev, err := h.eventRepo.GetByID(r.Context(), tenantID, id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-EVT-002: only creator or organizer can cancel
	if !h.canModifyEvent(r, tenantID, userID, ev) {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "only the event creator or organizer can cancel", "")
		return
	}

	// Get attendee emails for the CloudEvent
	attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, ev.ID)
	var emails []string
	for _, a := range attendees {
		emails = append(emails, a.Email)
	}

	if err := h.eventRepo.Cancel(r.Context(), tenantID, id); err != nil {
		handleDomainError(w, err)
		return
	}

	// Emit CloudEvent
	h.producer.Publish(r.Context(), tenantID, events.TypeEventCancelled, "/agenda/events", map[string]interface{}{
		"event_id":        ev.ID.String(),
		"calendar_id":     ev.CalendarID.String(),
		"title":           ev.Title,
		"cancelled_by":    userID.String(),
		"attendee_emails": emails,
	})

	w.WriteHeader(http.StatusNoContent)
}

// canModifyEvent checks if userID is the creator or an organizer of the event.
func (h *EventHandler) canModifyEvent(r *http.Request, tenantID, userID uuid.UUID, ev *domain.Event) bool {
	if ev.CreatedBy == userID {
		return true
	}
	attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, ev.ID)
	for _, a := range attendees {
		if a.UserID == userID && a.Role == domain.AttendeeRoleOrganizer {
			return true
		}
	}
	return false
}

func parseTimeRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	if startStr == "" || endStr == "" {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "start and end query params are required", "")
		return time.Time{}, time.Time{}, false
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid start time format (use RFC3339)", "start")
		return time.Time{}, time.Time{}, false
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid end time format (use RFC3339)", "end")
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}
