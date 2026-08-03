# DevOps Agent Memory — agenda

## Last Deploy: 2026-05-21 (commit 2eba31a) — DEPLOYED_HEALTHY, no-traffic tag=rev-2eba31a
## Last Review: 2026-05-07 (commit 299bcd9) — PASS_WITH_NOTES (17/20 checks PASS, 0 FAIL, 3 WARN)

### All Prior Blockers RESOLVED
1. Non-root Docker user — FIXED: USER 1000 (nonroot), addgroup/adduser, --chown
2. .dockerignore missing — FIXED: excludes .git, tasks/, *.md, *_test.go
3. .env.example missing — FIXED: DATABASE_URL, OID_ISSUER_URL, REDPANDA_BROKERS, PORT, LOG_LEVEL, CORS_ALLOWED_ORIGINS
4. Go version mismatch — FIXED: go.mod 1.25.0, Dockerfile golang:1.25-alpine, CI go-version: "1.25"
5. No CORS — FIXED: CORS_ALLOWED_ORIGINS env var, defaults to *.staging.orbusdigital.com
6. SQL injection — FIXED: parameterized queries ($1,$2) throughout
7. HS256 JWT — FIXED: RS256/JWKS via OID_ISSUER_URL (commit e0f9a2e)
8. Security headers missing — FIXED: HSTS, CSP, X-Frame-Options (commit 87b605d)
9. Rate limiting missing — FIXED: token bucket 10 req/s burst 20 (commit 87b605d)
10. REDPANDA_BROKERS required at startup — FIXED: NoopProducer fallback (commit 299bcd9)

### Current Warnings (non-blocking)
- WARN-1: Dockerfile HEALTHCHECK hardcodes 8088 — Cloud Run injects PORT=8080, HEALTHCHECK ignored by Cloud Run
- WARN-2: CORS_ALLOWED_ORIGINS not in cloudrun/agenda.json env_vars (defaults are correct)
- WARN-3: No golangci-lint in CI (only go vet)

### Current State
- deploy_target: cloud-run
- cloudrun/agenda.json: present and correct
- Docker build: PASS — 8.5MB alpine:3.21
- Tests: 57 PASS (api, auth, domain, events packages)
- go vet: PASS
- RLS: enabled on calendars, events, attendees
- Auth: RS256/JWKS from OID (OID_ISSUER_URL env var)
- NoopProducer: active when REDPANDA_BROKERS unset (Cloud Run default)
- PR#3 (2eba31a): Pub/Sub producer via EVENT_BUS factory pattern — replaces Redpanda
- Cloud Run env already has EVENT_BUS=pubsub, PUBSUB_TOPIC=agenda-events, PUBSUB_TOPIC_DLQ=agenda-events-dlq, GCP_PROJECT_ID=orbus-ods-staging (set before this deploy, not added by devops agent)
- Tagged URL (no traffic): https://rev-2eba31a---agenda-dwpodymsna-ew.a.run.app
- To promote: bash ~/dev/ops/adlc-v2/scripts/cli/cloud-run-deploy.sh agenda --tag 2eba31a --promote

### Go Test Execution Note
- `go` binary not in agent PATH — run tests inside builder image:
  `docker run test-agenda-builder sh -c "cd /app && go test ./..."`
- Build builder: `docker build --target builder -t test-agenda-builder <path>`

### Coolify Config (legacy, kept for reference)
- UUID: q8g0o8k8c0gwc84gkw0coow4 (may no longer be active after Cloud Run migration)
- Staging was HEALTHY via Coolify before migration
