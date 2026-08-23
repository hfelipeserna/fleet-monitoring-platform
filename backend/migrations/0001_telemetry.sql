CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS telemetry (
  client_event_id UUID NOT NULL,
  plate TEXT NOT NULL CHECK (plate ~ '^[A-Z]{3}[0-9]{3}$'),
  received_at TIMESTAMPTZ NOT NULL,
  occurred_at TIMESTAMPTZ,
  lat DOUBLE PRECISION CHECK (lat BETWEEN -90 AND 90),
  lon DOUBLE PRECISION CHECK (lon BETWEEN -180 AND 180),
  speed INT NOT NULL CHECK (speed >= 0),
  geom GEOGRAPHY(Point,4326) GENERATED ALWAYS AS (
    CASE WHEN lat IS NULL OR lon IS NULL THEN NULL
    ELSE ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography END
  ) STORED,
  PRIMARY KEY (client_event_id, received_at)
);

SELECT create_hypertable('telemetry','received_at', chunk_time_interval => INTERVAL '1 day', if_not_exists => TRUE, migrate_data => TRUE);

CREATE INDEX IF NOT EXISTS telemetry_plate_received_at_idx ON telemetry (plate, received_at DESC);

-- Idempotency: TimescaleDB hypertable UNIQUE/PK must include partition column (received_at).
-- A UNIQUE INDEX on (client_event_id) alone violates hypertable constraints and fails on create_hypertable.
-- End-to-end dedup is enforced via:
--   1) NATS JetStream DuplicateWindow 2m (Duplicates=2m per SPEC FR-003, MsgId=client_event_id) for short ingestion dedup, and
--   2) DB-level dedup table telemetry_dedup (PK client_event_id) joined at insert time.
-- This keeps hypertable PK (client_event_id, received_at) valid while guaranteeing global idempotency
-- even when retry carries different received_at/occurred_at.
-- psql \d telemetry should show PK (client_event_id, received_at); \d telemetry_dedup should show PK client_event_id.
DROP INDEX IF EXISTS telemetry_client_event_id_uniq;

CREATE TABLE IF NOT EXISTS telemetry_dedup (
  client_event_id UUID PRIMARY KEY,
  first_seen TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Optional retention for dedup (not a hypertable): keep 30d via periodic DELETE or pg_cron.
-- Example: DELETE FROM telemetry_dedup WHERE first_seen < now() - INTERVAL '30 days';
-- Do not convert telemetry_dedup to hypertable; it is small and indexed by PK.

-- GIST optional for SPEC-002: CREATE INDEX IF NOT EXISTS telemetry_geom_idx ON telemetry USING GIST (geom);
-- Migrator should use pg_advisory_lock to serialize DDL; IF NOT EXISTS ensures idempotence.
-- CopyFrom must send only lat/lon (geom is GENERATED).
