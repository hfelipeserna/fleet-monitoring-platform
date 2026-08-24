-- SPEC-002 Step1: critical_zones + GIST for ST_Within(telemetry.geom::geometry, critical_zones.geom)
-- Idempotent migration: IF NOT EXISTS everywhere, safe to re-run without altering hypertable.
-- Migrator must serialize DDL with pg_advisory_lock (e.g., SELECT pg_advisory_lock(727271) / pg_advisory_unlock(727271)).
-- No create_hypertable here; telemetry hypertable (SPEC-001) remains partitioned by received_at.

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS critical_zones (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
  geom geometry(Polygon,4326) NOT NULL CHECK (ST_IsValid(geom) AND ST_GeometryType(geom) = 'ST_Polygon' AND ST_Area(geom) > 0 AND ST_NPoints(geom) BETWEEN 4 AND 101),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS critical_zones_geom_gist ON critical_zones USING GIST (geom);

-- GIST coherente para ST_Within(telemetry.geom::geometry, critical_zones.geom)
-- telemetry.geom es GEOGRAPHY(Point,4326) GENERATED; critical_zones.geom es geometry(Polygon,4326).
-- El índice funcional ((geom::geometry)) permite que el planner use GIST en ST_Within sin cast dinámico caro.
-- Sin alterar hypertable: no se toca particionamiento ni chunk_interval de telemetry.
CREATE INDEX IF NOT EXISTS telemetry_geom_idx ON telemetry USING GIST ((geom::geometry));
