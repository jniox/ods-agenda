# agenda

ODS Platform Agenda service (Go). Manages calendars, events, and attendee responses.

## Event Bus Configuration

The service publishes CloudEvents v1.0 to an event bus. The backend is selected via the `EVENT_BUS` environment variable:

| `EVENT_BUS` value | Backend | Description |
|---|---|---|
| `pubsub` | Google Cloud Pub/Sub | Production path. Requires `GCP_PROJECT_ID` and `PUBSUB_TOPIC`. |
| `kafka` | Kafka / Redpanda | Legacy path (deprecated). Requires `REDPANDA_BROKERS`. |
| _(unset)_ | Kafka | Backward-compatible default. |

### Environment Variables

| Variable | Required when | Description |
|---|---|---|
| `EVENT_BUS` | Always recommended | `pubsub` or `kafka` |
| `GCP_PROJECT_ID` | `EVENT_BUS=pubsub` | GCP project ID (e.g. `orbus-ods-staging`) |
| `PUBSUB_TOPIC` | `EVENT_BUS=pubsub` | Pub/Sub topic name (e.g. `agenda-events`) |
| `PUBSUB_TOPIC_DLQ` | `EVENT_BUS=pubsub` | Dead-letter topic (e.g. `agenda-events-dlq`) |
| `REDPANDA_BROKERS` | `EVENT_BUS=kafka` | Comma-separated broker addresses |

### IAM Requirements (Cloud Pub/Sub)

The Cloud Run runtime service account needs `roles/pubsub.publisher` on the target topic.

### Local Development with Pub/Sub Emulator

```bash
# Start the emulator via Docker
docker run -d --name pubsub-emulator -p 8681:8681 messagebird/gcloud-pubsub-emulator:latest

# Set the emulator host (disables auth, connects to local emulator)
export PUBSUB_EMULATOR_HOST=localhost:8681

# Run tests (pubsub tests auto-skip if emulator is not available)
go test ./internal/events/ -v
```

### CloudEvents Envelope

All events follow CloudEvents v1.0 spec. When using Pub/Sub, CloudEvents headers are set as message attributes for server-side filtering:

| Attribute | Example |
|---|---|
| `ce-specversion` | `1.0` |
| `ce-id` | `550e8400-e29b-41d4-a716-446655440000` |
| `ce-type` | `ods.agenda.event.created` |
| `ce-source` | `/agenda/events` |
| `ce-time` | `2026-05-13T10:00:00Z` |
| `ce-datacontenttype` | `application/json` |
| `tenant_id` | `a1b2c3d4-...` |

The message body contains the full CloudEvent JSON payload. `OrderingKey` is set to `tenant_id` for ordered delivery per tenant.

## Running

```bash
export DATABASE_URL="postgres://ods:ods-dev-2026@127.0.0.1:5433/ods"
export OID_ISSUER_URL="https://oid.staging.orbusdigital.com"
export EVENT_BUS=pubsub
export GCP_PROJECT_ID=orbus-ods-staging
export PUBSUB_TOPIC=agenda-events
go run ./cmd/server/
```

## Testing

```bash
go test ./...
go vet ./...
```
