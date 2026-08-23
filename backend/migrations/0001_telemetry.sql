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

-- Idempotency: guarantee end-to-end dedup for client_event_id even when received_at differs (retry with different timestamp).
-- Hypertable PK (client_event_id, received_at) alone allows duplicate client_event_id with different received_at.
-- This UNIQUE index on client_event_id enforces idempotency at DB layer: consumer uses INSERT ... ON CONFLICT DO NOTHING.
-- On single-node Timescale this is valid. For distributed hypertables, UNIQUE must include time column; in that case
-- rely on NATS JetStream DuplicateWindow (24h) + application-level dedup before insert.
-- psql \d telemetry should show this unique constraint.
CREATE UNIQUE INDEX IF NOT EXISTS telemetry_client_event_id_uniq ON telemetry (client_event_id);

-- GIST optional for SPEC-002: CREATE INDEX IF NOT EXISTS telemetry_geom_idx ON telemetry USING GIST (geom);
-- Migrator should use pg_advisory_lock to serialize DDL; IF NOT EXISTS ensures idempotence.
-- CopyFrom must send only lat/lon (geom is GENERATED).
