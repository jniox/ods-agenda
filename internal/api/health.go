package api

import (
	"net/http"
	"time"

	"github.com/orbus-digital/agenda/internal/repository"
)

// HealthHandler handles /health and /ready endpoints.
type HealthHandler struct {
	db        *repository.DB
	startTime time.Time
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *repository.DB) *HealthHandler {
	return &HealthHandler{db: db, startTime: time.Now()}
}

// Health returns service liveness status.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	uptime := int(time.Since(h.startTime).Seconds())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"service":        "agenda",
		"version":        "1.0.0",
		"uptime_seconds": uptime,
	})
}

// Ready returns service readiness (checks DB).
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "not_ready",
			"reason": "database connection failed",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ready",
	})
}
