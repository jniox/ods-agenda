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

// AttendeeHandler handles attendee REST endpoints.
type AttendeeHandler struct {
	attendeeRepo domain.AttendeeRepository
	eventRepo    domain.EventRepository
	producer     events.Producer
}

// NewAttendeeHandler creates a new AttendeeHandler.
func NewAttendeeHandler(ar domain.AttendeeRepository, er domain.EventRepository, p events.Producer) *AttendeeHandler {
	return &AttendeeHandler{attendeeRepo: ar, eventRepo: er, producer: p}
}

type addAttendeeRequest struct {
	Email  string `json:"email"`
	UserID string `json:"user_id,omitempty"`
	Role   string `json:"role"`
}

type respondRequest struct {
	Status string `json:"status"`
}

// AddAttendee handles POST /api/v1/events/:event_id/attendees.
func (h *AttendeeHandler) AddAttendee(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())
	eventID, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid event_id", "event_id")
		return
	}

	// Check event exists
	ev, err := h.eventRepo.GetByID(r.Context(), tenantID, eventID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-EVT-002: only creator or organizer can add attendees
	if ev.CreatedBy != userID {
		// Check if user is an organizer
		attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, eventID)
		isOrganizer := false
		for _, a := range attendees {
			if a.UserID == userID && a.Role == domain.AttendeeRoleOrganizer {
				isOrganizer = true
				break
			}
		}
		if !isOrganizer {
			writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "only the event creator or organizer can add attendees", "")
			return
		}
	}

	// Check attendee count (BR-ATT-006)
	existingAttendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, eventID)
	if len(existingAttendees) >= 50 {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "attendees: maximum 50 allowed", "attendees")
		return
	}

	var req addAttendeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", "")
		return
	}

	role := domain.AttendeeRole(req.Role)
	if !role.IsValid() {
		role = domain.AttendeeRoleRequired
	}

	var attUserID uuid.UUID
	if req.UserID != "" {
		attUserID, _ = uuid.Parse(req.UserID)
	}

	att, err := domain.NewAttendee(tenantID, eventID, attUserID, req.Email, role)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	if err := h.attendeeRepo.Create(r.Context(), att); err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toAttendeeResponse(att))
}

// Respond handles PUT /api/v1/events/:event_id/attendees/:attendee_id/respond.
func (h *AttendeeHandler) Respond(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())

	attendeeID, err := uuid.Parse(chi.URLParam(r, "attendee_id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid attendee_id", "attendee_id")
		return
	}

	att, err := h.attendeeRepo.GetByID(r.Context(), tenantID, attendeeID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-ATT-003: only the attendee themselves can respond
	if att.UserID != userID {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "only the attendee can respond to an invitation", "")
		return
	}

	// BR-ATT-005: external attendees cannot respond
	if att.UserID == uuid.Nil {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "external attendees cannot respond via API", "")
		return
	}

	var req respondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", "")
		return
	}

	status := domain.AttendeeStatus(req.Status)
	if !status.IsValid() || status == domain.AttendeeStatusPending {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be accepted, declined, or tentative", "status")
		return
	}

	if err := att.Respond(status); err != nil {
		handleDomainError(w, err)
		return
	}

	if err := h.attendeeRepo.UpdateStatus(r.Context(), att); err != nil {
		handleDomainError(w, err)
		return
	}

	// Emit CloudEvent
	h.producer.Publish(r.Context(), tenantID, events.TypeAttendeeResponded, "/agenda/attendees", map[string]interface{}{
		"attendee_id": att.ID.String(),
		"event_id":    att.EventID.String(),
		"user_id":     att.UserID.String(),
		"email":       att.Email,
		"status":      string(att.Status),
		"role":        string(att.Role),
	})

	writeJSON(w, http.StatusOK, toAttendeeResponse(att))
}

// RemoveAttendee handles DELETE /api/v1/events/:event_id/attendees/:attendee_id.
func (h *AttendeeHandler) RemoveAttendee(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())

	eventID, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid event_id", "event_id")
		return
	}

	attendeeID, err := uuid.Parse(chi.URLParam(r, "attendee_id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid attendee_id", "attendee_id")
		return
	}

	ev, err := h.eventRepo.GetByID(r.Context(), tenantID, eventID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	att, err := h.attendeeRepo.GetByID(r.Context(), tenantID, attendeeID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-ATT-004: only creator, organizer, or the attendee themselves
	canRemove := false
	if ev.CreatedBy == userID || att.UserID == userID {
		canRemove = true
	}
	if !canRemove {
		attendees, _ := h.attendeeRepo.ListByEvent(r.Context(), tenantID, eventID)
		for _, a := range attendees {
			if a.UserID == userID && a.Role == domain.AttendeeRoleOrganizer {
				canRemove = true
				break
			}
		}
	}

	if !canRemove {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "insufficient permissions to remove attendee", "")
		return
	}

	if err := h.attendeeRepo.Delete(r.Context(), tenantID, attendeeID); err != nil {
		handleDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Availability holds the availability handler.
type AvailabilityHandler struct {
	repo domain.AvailabilityRepository
}

// NewAvailabilityHandler creates a new AvailabilityHandler.
func NewAvailabilityHandler(repo domain.AvailabilityRepository) *AvailabilityHandler {
	return &AvailabilityHandler{repo: repo}
}

type availabilityRequest struct {
	UserIDs  []string `json:"user_ids"`
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Timezone string   `json:"timezone"`
}

// CheckAvailability handles POST /api/v1/availability.
func (h *AvailabilityHandler) CheckAvailability(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())

	var req availabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", "")
		return
	}

	// BR-AVL-001: max 10 user_ids
	if len(req.UserIDs) > 10 {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "user_ids: maximum 10 allowed", "user_ids")
		return
	}
	if len(req.UserIDs) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "user_ids: at least one required", "user_ids")
		return
	}

	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid start time format", "start")
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid end time format", "end")
		return
	}

	// BR-AVL-002: max 30-day range
	if end.Sub(start) > 30*24*time.Hour {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "time range: maximum 30 days allowed", "")
		return
	}

	var userIDs []uuid.UUID
	for _, s := range req.UserIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid user_id: "+s, "user_ids")
			return
		}
		userIDs = append(userIDs, id)
	}

	slots, err := h.repo.GetBusySlots(r.Context(), tenantID, userIDs, start, end)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// Convert to string-keyed map for JSON
	availability := make(map[string]interface{})
	for uid, busy := range slots {
		availability[uid.String()] = map[string]interface{}{
			"busy": busy,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"availability": availability,
	})
}
