package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWKSKey represents a single key from a JWKS endpoint.
type JWKSKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKSResponse represents the JSON Web Key Set response from the OID JWKS endpoint.
type JWKSResponse struct {
	Keys []JWKSKey `json:"keys"`
}

// JWKSProvider fetches and caches JWKS keys from the OID issuer.
type JWKSProvider struct {
	issuerURL    string
	httpClient   *http.Client
	mu           sync.RWMutex
	keys         map[string]*rsa.PublicKey
	lastFetch    time.Time
	refreshEvery time.Duration
}

// NewJWKSProvider creates a new JWKS key provider.
func NewJWKSProvider(issuerURL string, refreshEvery time.Duration) *JWKSProvider {
	return &JWKSProvider{
		issuerURL:    issuerURL,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		keys:         make(map[string]*rsa.PublicKey),
		refreshEvery: refreshEvery,
	}
}

// SetHTTPClient sets a custom HTTP client (useful for testing).
func (p *JWKSProvider) SetHTTPClient(client *http.Client) {
	p.httpClient = client
}

// GetKey returns the RSA public key for the given key ID.
// It refreshes the cache if needed or if the key ID is unknown.
func (p *JWKSProvider) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	p.mu.RLock()
	key, ok := p.keys[kid]
	needsRefresh := time.Since(p.lastFetch) > p.refreshEvery
	p.mu.RUnlock()

	if ok && !needsRefresh {
		return key, nil
	}

	// Refresh keys
	if err := p.refresh(ctx); err != nil {
		// If refresh fails but we have a cached key, return it
		if ok {
			return key, nil
		}
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	p.mu.RLock()
	key, ok = p.keys[kid]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key ID %q not found in JWKS", kid)
	}
	return key, nil
}

func (p *JWKSProvider) refresh(ctx context.Context) error {
	url := p.issuerURL + "/.well-known/jwks.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKSResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pubKey, err := parseRSAPublicKey(k)
		if err != nil {
			continue
		}
		newKeys[k.Kid] = pubKey
	}

	p.mu.Lock()
	p.keys = newKeys
	p.lastFetch = time.Now()
	p.mu.Unlock()

	return nil
}

func parseRSAPublicKey(k JWKSKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
