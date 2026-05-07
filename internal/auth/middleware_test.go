package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-for-agenda"

func makeHMACToken(t *testing.T, tenantID, userID string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"sub":       userID,
		"exp":       exp.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	tokenStr := makeHMACToken(t, tenantID.String(), userID.String(), time.Now().Add(1*time.Hour))

	mw := NewJWTMiddleware(testSecret)
	var gotTenantID, gotUserID uuid.UUID
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenantID, _ = TenantIDFromContext(r.Context())
		gotUserID, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenantID, gotTenantID)
	assert.Equal(t, userID, gotUserID)
}

func TestJWTMiddleware_MissingHeader(t *testing.T) {
	mw := NewJWTMiddleware(testSecret)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	mw := NewJWTMiddleware(testSecret)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	tokenStr := makeHMACToken(t, uuid.New().String(), uuid.New().String(), time.Now().Add(-1*time.Hour))

	mw := NewJWTMiddleware(testSecret)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTMiddleware_MissingTenantID(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": uuid.New().String(),
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	mw := NewJWTMiddleware(testSecret)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTMiddleware_MissingSub(t *testing.T) {
	claims := jwt.MapClaims{
		"tenant_id": uuid.New().String(),
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	mw := NewJWTMiddleware(testSecret)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestContextHelpers_NoValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := TenantIDFromContext(req.Context())
	assert.False(t, ok)
	_, ok = UserIDFromContext(req.Context())
	assert.False(t, ok)
}

// --- RS256 Tests ---

func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func makeRS256Token(t *testing.T, privateKey *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	s, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return s
}

func TestJWTMiddlewareRSA_ValidToken(t *testing.T) {
	key := generateTestRSAKey(t)
	tenantID := uuid.New()
	userID := uuid.New()
	issuer := "https://oid.staging.orbusdigital.com"

	tokenStr := makeRS256Token(t, key, "test-kid-1", jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"sub":       userID.String(),
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
		"iss":       issuer,
	})

	mw := NewJWTMiddlewareRSA(&key.PublicKey, issuer)
	var gotTenantID, gotUserID uuid.UUID
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenantID, _ = TenantIDFromContext(r.Context())
		gotUserID, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenantID, gotTenantID)
	assert.Equal(t, userID, gotUserID)
}

func TestJWTMiddlewareRSA_RejectsHS256(t *testing.T) {
	key := generateTestRSAKey(t)
	issuer := "https://oid.staging.orbusdigital.com"

	// Try to use HS256 token against RS256 middleware
	hmacToken := makeHMACToken(t, uuid.New().String(), uuid.New().String(), time.Now().Add(1*time.Hour))

	mw := NewJWTMiddlewareRSA(&key.PublicKey, issuer)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+hmacToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTMiddlewareRSA_InvalidIssuer(t *testing.T) {
	key := generateTestRSAKey(t)
	issuer := "https://oid.staging.orbusdigital.com"

	tokenStr := makeRS256Token(t, key, "test-kid-1", jwt.MapClaims{
		"tenant_id": uuid.New().String(),
		"sub":       uuid.New().String(),
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
		"iss":       "https://evil.example.com",
	})

	mw := NewJWTMiddlewareRSA(&key.PublicKey, issuer)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTMiddlewareRSA_ExpiredToken(t *testing.T) {
	key := generateTestRSAKey(t)
	issuer := "https://oid.staging.orbusdigital.com"

	tokenStr := makeRS256Token(t, key, "test-kid-1", jwt.MapClaims{
		"tenant_id": uuid.New().String(),
		"sub":       uuid.New().String(),
		"exp":       time.Now().Add(-1 * time.Hour).Unix(),
		"iss":       issuer,
	})

	mw := NewJWTMiddlewareRSA(&key.PublicKey, issuer)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- JWKS Provider Tests ---

func TestJWKSProvider_FetchAndCache(t *testing.T) {
	key := generateTestRSAKey(t)

	// Create a test JWKS server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jwks := JWKSResponse{
			Keys: []JWKSKey{
				{
					Kty: "RSA",
					Kid: "test-kid-1",
					Use: "sig",
					Alg: "RS256",
					N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
					E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	provider := NewJWKSProvider(srv.URL, 5*time.Minute)

	pubKey, err := provider.GetKey(t.Context(), "test-kid-1")
	require.NoError(t, err)
	require.NotNil(t, pubKey)
	assert.Equal(t, key.PublicKey.N.Cmp(pubKey.N), 0)
	assert.Equal(t, key.PublicKey.E, pubKey.E)
}

func TestJWKSProvider_UnknownKid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := JWKSResponse{Keys: []JWKSKey{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	provider := NewJWKSProvider(srv.URL, 5*time.Minute)

	_, err := provider.GetKey(t.Context(), "unknown-kid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestJWKSProvider_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	provider := NewJWKSProvider(srv.URL, 5*time.Minute)

	_, err := provider.GetKey(t.Context(), "test-kid-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch JWKS")
}

func TestJWKSMiddleware_EndToEnd(t *testing.T) {
	key := generateTestRSAKey(t)

	// Create a test JWKS server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := JWKSResponse{
			Keys: []JWKSKey{
				{
					Kty: "RSA",
					Kid: "e2e-kid",
					Use: "sig",
					Alg: "RS256",
					N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
					E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	issuer := srv.URL
	provider := NewJWKSProvider(issuer, 5*time.Minute)
	mw := NewJWTMiddlewareJWKS(provider, issuer)

	tenantID := uuid.New()
	userID := uuid.New()
	tokenStr := makeRS256Token(t, key, "e2e-kid", jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"sub":       userID.String(),
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
		"iss":       issuer,
	})

	var gotTenantID, gotUserID uuid.UUID
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenantID, _ = TenantIDFromContext(r.Context())
		gotUserID, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenantID, gotTenantID)
	assert.Equal(t, userID, gotUserID)
}

func TestJWKSMiddleware_RejectsHMACTokens(t *testing.T) {
	key := generateTestRSAKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := JWKSResponse{
			Keys: []JWKSKey{
				{
					Kty: "RSA",
					Kid: "hmac-test-kid",
					Use: "sig",
					Alg: "RS256",
					N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
					E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	provider := NewJWKSProvider(srv.URL, 5*time.Minute)
	mw := NewJWTMiddlewareJWKS(provider, srv.URL)

	hmacToken := makeHMACToken(t, uuid.New().String(), uuid.New().String(), time.Now().Add(1*time.Hour))

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler with HMAC token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+hmacToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
