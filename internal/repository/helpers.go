package repository

import (
	"strings"
)

// isUniqueViolation checks if a PostgreSQL error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgx wraps PostgreSQL error code 23505 (unique_violation)
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "unique") ||
		strings.Contains(err.Error(), "duplicate key")
}
