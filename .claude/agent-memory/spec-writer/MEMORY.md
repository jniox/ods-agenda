# Spec Writer Memory

## ODS Platform Spec Patterns
- Specs dir: `~/dev/specs/ods-platform/specs/{service}/spec.md`
- No `context/` dir exists under ods-platform — architecture and business rules not yet written there
- Reference spec for format: `~/dev/specs/ods-dashboard/specs/dashboard/spec.md`
- Backlog file: `~/dev/specs/ods-platform/gestion/backlog.md`
- Service map: `~/dev/ops/agents/service-project-map.json` — update stack field when it differs from actual

## Agenda Service
- Stack: Go (not Rust as previously listed in service-project-map)
- Domain models already implemented in `internal/domain/` (calendar.go, event.go, attendee.go, errors.go)
- Port: 8088
- DB schema: agenda
- Phase: P5
