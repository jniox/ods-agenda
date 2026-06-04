package api

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// RequestIDMiddleware sets the X-Request-Id response header.
// It reads the request ID from the Chi context (set by middleware.RequestID),
// falls back to the X-Request-Id request header, and generates a UUID as last resort.
// This ensures the response always carries the same request ID used for logging.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		if reqID == "" {
			reqID = r.Header.Get("X-Request-Id")
		}
		if reqID == "" {
			reqID = uuid.New().String()
		}
		w.Header().Set("X-Request-Id", reqID)
		next.ServeHTTP(w, r)
	})
}

// NotFoundHandler returns an http.Handler that writes a JSON 404 response.
func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error: ErrorDetail{Code: "NOT_FOUND", Message: "Route not found"},
		})
	})
}

// MethodNotAllowedHandler returns an http.Handler that writes a JSON 405 response.
func MethodNotAllowedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error: ErrorDetail{Code: "METHOD_NOT_ALLOWED", Message: "Method not allowed"},
		})
	})
}
