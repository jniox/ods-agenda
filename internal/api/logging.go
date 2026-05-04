package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/orbus-digital/agenda/internal/auth"
)

// RequestLogger is a middleware that logs incoming requests and responses with structured context.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := middleware.GetReqID(r.Context())

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		status := ww.Status()

		logger := log.Info()
		if status >= 500 {
			logger = log.Error()
		} else if status >= 400 {
			logger = log.Warn()
		}

		evt := logger.
			Str("request_id", reqID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", status).
			Dur("duration_ms", duration)

		// Add tenant_id if available (after auth middleware)
		if tenantID, ok := auth.TenantIDFromContext(r.Context()); ok {
			evt = evt.Str("tenant_id", tenantID.String())
		}

		evt.Msg("request")
	})
}
