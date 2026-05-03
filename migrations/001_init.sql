-- Agenda service schema initialization
-- Multi-tenant with RLS on all tables

CREATE SCHEMA IF NOT EXISTS agenda;

-- Calendars
CREATE TABLE agenda.calendars (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        VARCHAR(200) NOT NULL,
    description TEXT DEFAULT '',
    color       VARCHAR(7) NOT NULL DEFAULT '#3B82F6',
    owner_id    UUID NOT NULL,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_calendar_name_owner UNIQUE (tenant_id, owner_id, name)
);

-- Events
CREATE TABLE agenda.events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    calendar_id     UUID NOT NULL REFERENCES agenda.calendars(id),
    title           VARCHAR(500) NOT NULL,
    description     TEXT DEFAULT '',
    location        VARCHAR(500) DEFAULT '',
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ NOT NULL,
    all_day         BOOLEAN NOT NULL DEFAULT false,
    timezone        VARCHAR(50) NOT NULL DEFAULT 'UTC',
    status          VARCHAR(20) NOT NULL DEFAULT 'confirmed'
                    CHECK (status IN ('confirmed', 'tentative', 'cancelled')),
    recurrence_rule TEXT DEFAULT '',
    created_by      UUID NOT NULL,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_event_time CHECK (end_time > start_time)
);

-- Attendees
CREATE TABLE agenda.attendees (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    event_id     UUID NOT NULL REFERENCES agenda.events(id) ON DELETE CASCADE,
    user_id      UUID,
    email        VARCHAR(320) NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'accepted', 'declined', 'tentative')),
    role         VARCHAR(20) NOT NULL DEFAULT 'required'
                 CHECK (role IN ('organizer', 'required', 'optional')),
    responded_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_attendee_event_email UNIQUE (event_id, email)
);

-- Indexes: Calendar lookups
CREATE INDEX idx_calendars_tenant ON agenda.calendars(tenant_id);
CREATE INDEX idx_calendars_owner ON agenda.calendars(tenant_id, owner_id);

-- Indexes: Event lookups and time-range queries
CREATE INDEX idx_events_tenant ON agenda.events(tenant_id);
CREATE INDEX idx_events_calendar ON agenda.events(calendar_id, start_time, end_time);
CREATE INDEX idx_events_time_range ON agenda.events(tenant_id, start_time, end_time)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_events_created_by ON agenda.events(tenant_id, created_by);

-- Indexes: Attendee lookups
CREATE INDEX idx_attendees_event ON agenda.attendees(event_id);
CREATE INDEX idx_attendees_user ON agenda.attendees(tenant_id, user_id);
CREATE INDEX idx_attendees_email ON agenda.attendees(tenant_id, email);

-- RLS
ALTER TABLE agenda.calendars ENABLE ROW LEVEL SECURITY;
ALTER TABLE agenda.events ENABLE ROW LEVEL SECURITY;
ALTER TABLE agenda.attendees ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_calendars ON agenda.calendars
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE POLICY tenant_isolation_events ON agenda.events
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE POLICY tenant_isolation_attendees ON agenda.attendees
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
