package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	ContextKeyTenantID contextKey = "tenant_id"
	ContextKeyUserID   contextKey = "user_id"
)

// TenantIDFromContext extracts tenant_id from the request context.
func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ContextKeyTenantID).(uuid.UUID)
	return v, ok
}

// UserIDFromContext extracts user_id (sub) from the request context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(uuid.UUID)
	return v, ok
}

// JWTMiddleware validates Bearer tokens and injects tenant_id + user_id into context.
// For development/testing, it accepts tokens signed with the provided HMAC secret.
// In production, this would validate against the OID JWKS endpoint.
type JWTMiddleware struct {
	secret []byte
}

// NewJWTMiddleware creates a JWT auth middleware with an HMAC secret.
func NewJWTMiddleware(secret string) *JWTMiddleware {
	return &JWTMiddleware{secret: []byte(secret)}
}

// Handler returns an http.Handler middleware that validates JWT tokens.
func (m *JWTMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid authorization header format")
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.secret, nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token claims")
			return
		}

		tenantIDStr, _ := claims["tenant_id"].(string)
		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil || tenantID == uuid.Nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid tenant_id claim")
			return
		}

		subStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(subStr)
		if err != nil || userID == uuid.Nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid sub claim")
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyTenantID, tenantID)
		ctx = context.WithValue(ctx, ContextKeyUserID, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}`))
}
