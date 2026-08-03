# Security Agent Memory — Agenda Service

## Last Review
- Date: 2026-05-07
- Commit: 87b605d
- Round: R6 (post-fix audit)
- Status: concerns
- Severity: MEDIUM
- OWASP Score: 9/10
- Staging: UNBLOCKED (0 critical/high findings)

## Fixes Confirmed in R3
1. **R2-003 FIXED** — RequestLogger middleware added (logging.go). Logs request_id, method, path, status, duration, tenant_id.
2. **R2-005 FIXED** — SET LOCAL now uses parameterized set_config($1) in db.go:44.
3. **R2-009 FIXED** — CORS policy added in router.go with staging origins whitelist.

## Fixes Confirmed in R6 (commits e0f9a2e + 87b605d)
1. **SEC-001 RESOLVED** — RS256/JWKS validation live in main.go:70 (NewJWTMiddlewareJWKS). jwks.go adds JWKSProvider with 5min cache, RWMutex, kid validation, algorithm pinning. NewJWTMiddleware (HMAC) retained as test-only helper.
2. **SEC-005-headers RESOLVED** — SecurityHeaders middleware in router.go:30: X-Content-Type-Options, X-Frame-Options, HSTS, CSP.
3. **SEC-007 RESOLVED** — Token-bucket RateLimiter per-IP (10 req/s, burst 20) in router.go:40.
4. **SEC-005-cors RESOLVED** — CORS origins from CORS_ALLOWED_ORIGINS env var via CORSOriginsFromEnv().

## Open Findings (R6)
1. **SEC-011 MEDIUM** — Rate limiter trusts X-Forwarded-For without proxy validation (router.go:108). IP spoofing bypass possible.
2. **SEC-009 MEDIUM** — No govulncheck/gosec in CI (ci.yml:64).
3. **SEC-003 LOW** — Real dev DB credentials in .env.example:2.
4. **SEC-002 LOW** — Hardcoded CI password ci.yml:18.
5. **SEC-006 LOW** — .gitignore missing *.pem, *.key, *.cert, .env.* variants.
6. **SEC-008 LOW** — writeError() string concatenation (middleware.go:139). Constants only, low risk.
7. **SEC-010 LOW** — No RemoteAddr in RequestLogger (logging.go:32).
8. **SEC-013 LOW** — JWKS HTTP client missing response body size limit (jwks.go:87).

## R6 Report
- JSON: /home/jniox_orbusdigital_com/dev/ops/reviews/agenda/security.json
- HTML: https://reports.dev.orbusdigital.com/review-security/20260507-48ec2bfef411.html

## Architecture Notes (current)
- Stack: Go 1.25, Chi router, pgx v5.9.2, zerolog v1.35.1, kafka-go v0.4.51
- Auth: RS256/JWKS via OID_ISSUER_URL (NewJWTMiddlewareJWKS, main.go:70)
- JWKS: JWKSProvider with 5-min cache, RWMutex, fallback on OID downtime — jwks.go
- Security headers: X-Content-Type-Options, X-Frame-Options, HSTS, CSP — router.go:30
- Rate limiting: token-bucket per-IP 10 req/s burst 20 — router.go:40
- CORS: CORS_ALLOWED_ORIGINS env var, fallback *.staging.orbusdigital.com — router.go:122
- RLS: Correctly implemented on all 3 tables — migrations/001_init.sql:83-90
- Multi-tenancy: WithTenantTx uses parameterized set_config() — db.go:44
- MaxBytesReader(1MB) on all handlers
- HTTP server timeouts: ReadTimeout/WriteTimeout 15s, IdleTimeout 60s
- No XML parsing, no HTML rendering
- All direct dependencies current, no known CVEs at review date
