# Wiki Compiler — Persistent Memory

## Wiki Location
- Wiki base: `~/dev/docs/wiki/`
- Entity pages: `~/dev/docs/wiki/entities/`
- Synthesis pages: `~/dev/docs/wiki/synthesis/`
- Decisions: `~/dev/docs/wiki/decisions/`
- Log: `~/dev/docs/log.md` (prepend new entries at top)
- Compilation report: `~/dev/docs/wiki/COMPILATION-REPORT.md`
- Index: `~/dev/docs/wiki/INDEX.md`

## Key Paths (data sources)
- Pipeline state: `~/.claude/agent-memory/pipeline/state.md`
- Service map: `~/dev/ops/agents/service-project-map.json`
- External deps: `~/dev/ops/external-deps.md`
- Reviews: `~/dev/ops/reviews/{service}/`
- Specs: `~/dev/specs/ods-platform/` (no `specs/` subfolder for ods-platform — specs in context.bak or generated)

## Patterns Confirmed
- log.md entries are prepended (newest at top) — always insert before the previous top entry
- synthesis/ files use `security.md` not `security-posture.md` — INDEX.md links match
- Lejecos feature services (analytics, author-interface, payments, etc.) have no entity pages — intentional, covered by lejecos-strapi-cms.md and lejecos-nextjs-frontend.md
- Infrastructure services (infra-redpanda, meilisearch, minio, minio-lejecos) have no entity pages — intentional, covered by architecture.md
- `~/dev/docs/wiki/synthesis/` has: architecture.md, debt.md, deployment.md, external-deps.md, roadmap.md, security.md, veille.md (7 files)
- `~/dev/docs/wiki/entities/` has 17 service pages

## Compilation Run History
- 2026-05-04 (first full): 26 pages created from scratch
- 2026-05-04 (refresh #2): 7 pages updated — post agenda r14/r15 reviews completed

## Known Persistent Issues (as of 2026-05-04)
- Agenda spec.md missing — PDLC BLOCKED (CRITICAL)
- DocEditor spec.md missing
- 12 PRs awaiting human merge
- PostgreSQL 17.9 upgrade required (CVE-2026-2005)
- SecureMail deploy blocked (MASTER_ENCRYPTION_KEY + JWT_RSA_PUBLIC_KEY_B64)
- Agenda HS256 JWT — production gate

## Agenda Review Summary
- 15 review rounds (r1-r15) completed as of 2026-05-04
- Final state: ALL_REVIEWS_PASS at commit 09beeff
- PR#2 open — human review required
- Architect r15: 7/8 PASS (WARN: HS256 JWT, WARN: correlation ID)
- Security r14: OWASP 5/10, MEDIUM — staging unblocked, production blocked on 4 MEDIUM
- DevOps r14: 15/15 PASS
