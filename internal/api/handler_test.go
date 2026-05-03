package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/orbus-digital/agenda/internal/auth"
	"github.com/orbus-digital/agenda/internal/domain"
	"github.com/orbus-digital/agenda/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-for-agenda"

func makeToken(t *testing.T, tenantID, userID uuid.UUID) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"sub":       userID.String(),
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

func setupTestRouter() (chi.Router, *mockCalendarRepo, *mockEventRepo, *mockAttendeeRepo, *mockAvailabilityRepo, *events.NoopProducer) {
	calRepo := newMockCalendarRepo()
	evRepo := newMockEventRepo()
	attRepo := newMockAttendeeRepo()
	availRepo := newMockAvailabilityRepo()
	producer := &events.NoopProducer{}

	calHandler := NewCalendarHandler(calRepo, producer)
	evHandler := NewEventHandler(evRepo, calRepo, attRepo, producer)
	attHandler := NewAttendeeHandler(attRepo, evRepo, producer)
	availHandler := NewAvailabilityHandler(availRepo)

	jwtMw := auth.NewJWTMiddleware(testSecret)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(jwtMw.Handler)

		r.Route("/api/v1/calendars", func(r chi.Router) {
			r.Post("/", calHandler.Create)
			r.Get("/", calHandler.List)
			r.Get("/{id}", calHandler.GetByID)
			r.Put("/{id}", calHandler.Update)
			r.Delete("/{id}", calHandler.Delete)
			r.Post("/{calendar_id}/events", evHandler.Create)
			r.Get("/{calendar_id}/events", evHandler.ListByCalendar)
		})

		r.Route("/api/v1/events", func(r chi.Router) {
			r.Get("/", evHandler.ListByTenant)
			r.Get("/{id}", evHandler.GetByID)
			r.Put("/{id}", evHandler.Update)
			r.Delete("/{id}", evHandler.Cancel)
			r.Post("/{event_id}/attendees", attHandler.AddAttendee)
			r.Put("/{event_id}/attendees/{attendee_id}/respond", attHandler.Respond)
			r.Delete("/{event_id}/attendees/{attendee_id}", attHandler.RemoveAttendee)
		})

		r.Post("/api/v1/availability", availHandler.CheckAvailability)
	})

	return r, calRepo, evRepo, attRepo, availRepo, producer
}

func doRequest(r chi.Router, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// --- Calendar Tests ---

func TestCreateCalendar_Success(t *testing.T) {
	r, _, _, _, _, producer := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()
	token := makeToken(t, tenantID, userID)

	rec := doRequest(r, http.MethodPost, "/api/v1/calendars", map[string]string{
		"name":        "Work",
		"description": "Work calendar",
		"color":       "#10B981",
	}, token)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp calendarResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Work", resp.Name)
	assert.Equal(t, "#10B981", resp.Color)
	assert.Equal(t, tenantID, resp.TenantID)
	assert.Equal(t, userID, resp.OwnerID)

	// Verify calendar.created CloudEvent emitted
	found := false
	for _, ce := range producer.Published {
		if ce.Type == events.TypeCalendarCreated {
			found = true
		}
	}
	assert.True(t, found, "expected ods.agenda.calendar.created CloudEvent")
}

func TestCreateCalendar_MissingName(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	token := makeToken(t, uuid.New(), uuid.New())

	rec := doRequest(r, http.MethodPost, "/api/v1/calendars", map[string]string{
		"description": "No name",
	}, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateCalendar_DuplicateName(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()
	token := makeToken(t, tenantID, userID)

	doRequest(r, http.MethodPost, "/api/v1/calendars", map[string]string{"name": "Work"}, token)
	rec := doRequest(r, http.MethodPost, "/api/v1/calendars", map[string]string{"name": "Work"}, token)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateCalendar_Unauthorized(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	rec := doRequest(r, http.MethodPost, "/api/v1/calendars", map[string]string{"name": "Work"}, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListCalendars_Success(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()
	token := makeToken(t, tenantID, userID)

	doRequest(r, http.MethodPost, "/api/v1/calendars", map[string]string{"name": "Cal1"}, token)
	doRequest(r, http.MethodPost, "/api/v1/calendars", map[string]string{"name": "Cal2"}, token)

	rec := doRequest(r, http.MethodGet, "/api/v1/calendars", nil, token)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp PaginatedResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
}

func TestGetCalendar_NotFound(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	token := makeToken(t, uuid.New(), uuid.New())
	rec := doRequest(r, http.MethodGet, "/api/v1/calendars/"+uuid.New().String(), nil, token)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateCalendar_NotOwner(t *testing.T) {
	r, calRepo, _, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	ownerID := uuid.New()
	otherUser := uuid.New()

	cal, _ := domain.NewCalendar(tenantID, ownerID, "Work", "", "")
	calRepo.Create(nil, cal)

	token := makeToken(t, tenantID, otherUser)
	rec := doRequest(r, http.MethodPut, "/api/v1/calendars/"+cal.ID.String(), map[string]string{"name": "Updated"}, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDeleteCalendar_DefaultForbidden(t *testing.T) {
	r, calRepo, _, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	ownerID := uuid.New()

	cal, _ := domain.NewCalendar(tenantID, ownerID, "Default", "", "")
	cal.IsDefault = true
	calRepo.Create(nil, cal)

	token := makeToken(t, tenantID, ownerID)
	rec := doRequest(r, http.MethodDelete, "/api/v1/calendars/"+cal.ID.String(), nil, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDeleteCalendar_Success(t *testing.T) {
	r, calRepo, _, _, _, producer := setupTestRouter()
	tenantID := uuid.New()
	ownerID := uuid.New()

	cal, _ := domain.NewCalendar(tenantID, ownerID, "Work", "", "")
	calRepo.Create(nil, cal)

	token := makeToken(t, tenantID, ownerID)
	rec := doRequest(r, http.MethodDelete, "/api/v1/calendars/"+cal.ID.String(), nil, token)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify calendar.deleted CloudEvent emitted
	found := false
	for _, ce := range producer.Published {
		if ce.Type == events.TypeCalendarDeleted {
			found = true
		}
	}
	assert.True(t, found, "expected ods.agenda.calendar.deleted CloudEvent")
}

// --- Event Tests ---

func TestCreateEvent_Success(t *testing.T) {
	r, calRepo, _, _, _, producer := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()

	cal, _ := domain.NewCalendar(tenantID, userID, "Work", "", "")
	calRepo.Create(nil, cal)

	token := makeToken(t, tenantID, userID)
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)

	rec := doRequest(r, http.MethodPost, "/api/v1/calendars/"+cal.ID.String()+"/events", map[string]interface{}{
		"title":      "Meeting",
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
		"timezone":   "UTC",
	}, token)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp eventResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Meeting", resp.Title)
	assert.Equal(t, domain.EventStatusConfirmed, resp.Status)
	assert.True(t, len(producer.Published) > 0)
	assert.Equal(t, events.TypeEventCreated, producer.Published[0].Type)
}

func TestCreateEvent_InvalidTimezone(t *testing.T) {
	r, calRepo, _, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()

	cal, _ := domain.NewCalendar(tenantID, userID, "Work", "", "")
	calRepo.Create(nil, cal)

	token := makeToken(t, tenantID, userID)
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)

	rec := doRequest(r, http.MethodPost, "/api/v1/calendars/"+cal.ID.String()+"/events", map[string]interface{}{
		"title":      "Meeting",
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
		"timezone":   "Invalid/Timezone",
	}, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateEvent_EndBeforeStart(t *testing.T) {
	r, calRepo, _, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()

	cal, _ := domain.NewCalendar(tenantID, userID, "Work", "", "")
	calRepo.Create(nil, cal)

	token := makeToken(t, tenantID, userID)
	start := time.Now().Add(2 * time.Hour)
	end := time.Now().Add(1 * time.Hour)

	rec := doRequest(r, http.MethodPost, "/api/v1/calendars/"+cal.ID.String()+"/events", map[string]interface{}{
		"title":      "Meeting",
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
	}, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateEvent_CalendarNotFound(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	token := makeToken(t, uuid.New(), uuid.New())
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)

	rec := doRequest(r, http.MethodPost, "/api/v1/calendars/"+uuid.New().String()+"/events", map[string]interface{}{
		"title":      "Meeting",
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
	}, token)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreateEvent_TooManyAttendees(t *testing.T) {
	r, calRepo, _, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()

	cal, _ := domain.NewCalendar(tenantID, userID, "Work", "", "")
	calRepo.Create(nil, cal)

	token := makeToken(t, tenantID, userID)
	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)

	attendees := make([]map[string]string, 51)
	for i := range attendees {
		attendees[i] = map[string]string{"email": "user" + uuid.New().String()[:8] + "@test.com", "role": "required"}
	}

	rec := doRequest(r, http.MethodPost, "/api/v1/calendars/"+cal.ID.String()+"/events", map[string]interface{}{
		"title":      "Meeting",
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
		"attendees":  attendees,
	}, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetEvent_Success(t *testing.T) {
	r, _, evRepo, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	ev, _ := domain.NewEvent(tenantID, uuid.New(), userID, "Meeting", "", "", start, end, false, "UTC", "")
	evRepo.Create(nil, ev)

	token := makeToken(t, tenantID, userID)
	rec := doRequest(r, http.MethodGet, "/api/v1/events/"+ev.ID.String(), nil, token)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdateEvent_NotCreatorForbidden(t *testing.T) {
	r, _, evRepo, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	creatorID := uuid.New()
	otherUser := uuid.New()

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	ev, _ := domain.NewEvent(tenantID, uuid.New(), creatorID, "Meeting", "", "", start, end, false, "UTC", "")
	evRepo.Create(nil, ev)

	token := makeToken(t, tenantID, otherUser)
	rec := doRequest(r, http.MethodPut, "/api/v1/events/"+ev.ID.String(), map[string]interface{}{
		"title": "Updated",
	}, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCancelEvent_Success(t *testing.T) {
	r, _, evRepo, _, _, producer := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	ev, _ := domain.NewEvent(tenantID, uuid.New(), userID, "Meeting", "", "", start, end, false, "UTC", "")
	evRepo.Create(nil, ev)

	token := makeToken(t, tenantID, userID)
	rec := doRequest(r, http.MethodDelete, "/api/v1/events/"+ev.ID.String(), nil, token)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify CloudEvent emitted
	found := false
	for _, ce := range producer.Published {
		if ce.Type == events.TypeEventCancelled {
			found = true
		}
	}
	assert.True(t, found, "expected ods.agenda.event.cancelled CloudEvent")
}

// --- Attendee Tests ---

func TestAddAttendee_Success(t *testing.T) {
	r, _, evRepo, _, _, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	ev, _ := domain.NewEvent(tenantID, uuid.New(), userID, "Meeting", "", "", start, end, false, "UTC", "")
	evRepo.Create(nil, ev)

	token := makeToken(t, tenantID, userID)
	rec := doRequest(r, http.MethodPost, "/api/v1/events/"+ev.ID.String()+"/attendees", map[string]interface{}{
		"email": "new@test.com",
		"role":  "required",
	}, token)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestRespondAttendee_Success(t *testing.T) {
	r, _, evRepo, attRepo, _, producer := setupTestRouter()
	tenantID := uuid.New()
	creatorID := uuid.New()
	attendeeUserID := uuid.New()

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	ev, _ := domain.NewEvent(tenantID, uuid.New(), creatorID, "Meeting", "", "", start, end, false, "UTC", "")
	evRepo.Create(nil, ev)

	att, _ := domain.NewAttendee(tenantID, ev.ID, attendeeUserID, "att@test.com", domain.AttendeeRoleRequired)
	attRepo.Create(nil, att)

	token := makeToken(t, tenantID, attendeeUserID)
	rec := doRequest(r, http.MethodPut, "/api/v1/events/"+ev.ID.String()+"/attendees/"+att.ID.String()+"/respond", map[string]string{
		"status": "accepted",
	}, token)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Check CloudEvent
	found := false
	for _, ce := range producer.Published {
		if ce.Type == events.TypeAttendeeResponded {
			found = true
		}
	}
	assert.True(t, found)
}

func TestRespondAttendee_NotSelfForbidden(t *testing.T) {
	r, _, evRepo, attRepo, _, _ := setupTestRouter()
	tenantID := uuid.New()
	creatorID := uuid.New()
	attendeeUserID := uuid.New()
	otherUser := uuid.New()

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	ev, _ := domain.NewEvent(tenantID, uuid.New(), creatorID, "Meeting", "", "", start, end, false, "UTC", "")
	evRepo.Create(nil, ev)

	att, _ := domain.NewAttendee(tenantID, ev.ID, attendeeUserID, "att@test.com", domain.AttendeeRoleRequired)
	attRepo.Create(nil, att)

	token := makeToken(t, tenantID, otherUser)
	rec := doRequest(r, http.MethodPut, "/api/v1/events/"+ev.ID.String()+"/attendees/"+att.ID.String()+"/respond", map[string]string{
		"status": "accepted",
	}, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRemoveAttendee_Success(t *testing.T) {
	r, _, evRepo, attRepo, _, _ := setupTestRouter()
	tenantID := uuid.New()
	creatorID := uuid.New()

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	ev, _ := domain.NewEvent(tenantID, uuid.New(), creatorID, "Meeting", "", "", start, end, false, "UTC", "")
	evRepo.Create(nil, ev)

	att, _ := domain.NewAttendee(tenantID, ev.ID, uuid.New(), "att@test.com", domain.AttendeeRoleRequired)
	attRepo.Create(nil, att)

	token := makeToken(t, tenantID, creatorID)
	rec := doRequest(r, http.MethodDelete, "/api/v1/events/"+ev.ID.String()+"/attendees/"+att.ID.String(), nil, token)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- Availability Tests ---

func TestAvailability_Success(t *testing.T) {
	r, _, _, _, availRepo, _ := setupTestRouter()
	tenantID := uuid.New()
	userID := uuid.New()
	user1 := uuid.New()

	availRepo.slots[user1] = []domain.BusySlot{
		{Start: time.Now().Add(2 * time.Hour), End: time.Now().Add(3 * time.Hour), EventID: uuid.New(), Title: "Busy"},
	}

	token := makeToken(t, tenantID, userID)
	rec := doRequest(r, http.MethodPost, "/api/v1/availability", map[string]interface{}{
		"user_ids": []string{user1.String()},
		"start":    time.Now().Format(time.RFC3339),
		"end":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}, token)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAvailability_TooManyUsers(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	token := makeToken(t, uuid.New(), uuid.New())

	userIDs := make([]string, 11)
	for i := range userIDs {
		userIDs[i] = uuid.New().String()
	}

	rec := doRequest(r, http.MethodPost, "/api/v1/availability", map[string]interface{}{
		"user_ids": userIDs,
		"start":    time.Now().Format(time.RFC3339),
		"end":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAvailability_RangeTooLarge(t *testing.T) {
	r, _, _, _, _, _ := setupTestRouter()
	token := makeToken(t, uuid.New(), uuid.New())

	rec := doRequest(r, http.MethodPost, "/api/v1/availability", map[string]interface{}{
		"user_ids": []string{uuid.New().String()},
		"start":    time.Now().Format(time.RFC3339),
		"end":      time.Now().Add(31 * 24 * time.Hour).Format(time.RFC3339),
	}, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Tenant Isolation Test ---

func TestTenantIsolation_CrossTenant(t *testing.T) {
	r, calRepo, _, _, _, _ := setupTestRouter()
	tenant1 := uuid.New()
	tenant2 := uuid.New()
	owner := uuid.New()

	cal, _ := domain.NewCalendar(tenant1, owner, "Private", "", "")
	calRepo.Create(nil, cal)

	// Tenant2 user tries to access tenant1's calendar
	token := makeToken(t, tenant2, uuid.New())
	rec := doRequest(r, http.MethodGet, "/api/v1/calendars/"+cal.ID.String(), nil, token)
	assert.Equal(t, http.StatusNotFound, rec.Code, "tenant isolation must hide cross-tenant resources")
}
