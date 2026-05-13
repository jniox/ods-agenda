package events

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const correlationIDKey contextKey = "correlationID"

// ContextWithCorrelationID returns a new context with the given correlation ID.
func ContextWithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from context.
// If not present, generates a new UUID.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok && id != "" {
		return id
	}
	return uuid.New().String()
}
