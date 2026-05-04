package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/orbus-digital/agenda/internal/domain"
)

// ErrorResponse represents the standard error response format.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail holds the error code, message, and optional field.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// PaginatedResponse wraps a list of items with pagination metadata.
type PaginatedResponse struct {
	Data   interface{} `json:"data"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErrorResponse(w http.ResponseWriter, status int, code, message, field string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{Code: code, Message: message, Field: field},
	})
}

func handleDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", ve.Error(), ve.Field)
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "resource not found", "")
		return
	}
	if errors.Is(err, domain.ErrForbidden) {
		writeErrorResponse(w, http.StatusForbidden, "FORBIDDEN", err.Error(), "")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		writeErrorResponse(w, http.StatusConflict, "CONFLICT", err.Error(), "")
		return
	}
	if errors.Is(err, domain.ErrUnauthorized) {
		writeErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), "")
		return
	}
	reqID := middleware.GetReqID(r.Context())
	log.Error().
		Str("request_id", reqID).
		Err(err).
		Msg("internal server error")
	writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", "")
}
