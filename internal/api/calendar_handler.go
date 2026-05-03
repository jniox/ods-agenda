package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orbus-digital/agenda/internal/auth"
	"github.com/orbus-digital/agenda/internal/domain"
	"github.com/orbus-digital/agenda/internal/events"
)

// CalendarHandler handles calendar REST endpoints.
type CalendarHandler struct {
	repo     domain.CalendarRepository
	producer events.Producer
}

// NewCalendarHandler creates a new CalendarHandler.
func NewCalendarHandler(repo domain.CalendarRepository, producer events.Producer) *CalendarHandler {
	return &CalendarHandler{repo: repo, producer: producer}
}

type createCalendarRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

type calendarResponse struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Color       string     `json:"color"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	IsDefault   bool       `json:"is_default"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

func toCalendarResponse(c *domain.Calendar) calendarResponse {
	return calendarResponse{
		ID:          c.ID,
		TenantID:    c.TenantID,
		Name:        c.Name,
		Description: c.Description,
		Color:       c.Color,
		OwnerID:     c.OwnerID,
		IsDefault:   c.IsDefault,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
		DeletedAt:   c.DeletedAt,
	}
}

// Create handles POST /api/v1/calendars.
func (h *CalendarHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())

	var req createCalendarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", "")
		return
	}

	cal, err := domain.NewCalendar(tenantID, userID, req.Name, req.Description, req.Color)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	if err := h.repo.Create(r.Context(), cal); err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toCalendarResponse(cal))
}

// List handles GET /api/v1/calendars.
func (h *CalendarHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())

	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit > 200 {
		limit = 200
	}

	var ownerID *uuid.UUID
	if ownerStr := r.URL.Query().Get("owner_id"); ownerStr != "" {
		id, err := uuid.Parse(ownerStr)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid owner_id", "owner_id")
			return
		}
		ownerID = &id
	}

	calendars, total, err := h.repo.List(r.Context(), tenantID, ownerID, limit, offset)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	data := make([]calendarResponse, len(calendars))
	for i, c := range calendars {
		data[i] = toCalendarResponse(c)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   data,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetByID handles GET /api/v1/calendars/:id.
func (h *CalendarHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid calendar id", "id")
		return
	}

	cal, err := h.repo.GetByID(r.Context(), tenantID, id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCalendarResponse(cal))
}

// Update handles PUT /api/v1/calendars/:id.
func (h *CalendarHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid calendar id", "id")
		return
	}

	cal, err := h.repo.GetByID(r.Context(), tenantID, id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-CAL-005: only owner can update
	if cal.OwnerID != userID {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "only the calendar owner can update", "")
		return
	}

	var req createCalendarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", "")
		return
	}

	if req.Name != "" {
		cal.Name = req.Name
	}
	cal.Description = req.Description
	if req.Color != "" {
		cal.Color = req.Color
	}

	if err := cal.Validate(); err != nil {
		handleDomainError(w, err)
		return
	}

	if err := h.repo.Update(r.Context(), cal); err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toCalendarResponse(cal))
}

// Delete handles DELETE /api/v1/calendars/:id.
func (h *CalendarHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := auth.TenantIDFromContext(r.Context())
	userID, _ := auth.UserIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid calendar id", "id")
		return
	}

	cal, err := h.repo.GetByID(r.Context(), tenantID, id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	// BR-CAL-005: only owner can delete
	if cal.OwnerID != userID {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "only the calendar owner can delete", "")
		return
	}

	// BR-CAL-002: cannot delete default calendar
	if cal.IsDefault {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "cannot delete default calendar", "")
		return
	}

	if err := h.repo.SoftDelete(r.Context(), tenantID, id); err != nil {
		handleDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseUUIDParam(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID")
	}
	return &id, nil
}
