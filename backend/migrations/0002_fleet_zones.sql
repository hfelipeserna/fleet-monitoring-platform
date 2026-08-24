-- SPEC-002 Step1: critical_zones + GIST for ST_Within(telemetry.geom::geometry, critical_zones.geom)
-- Idempotent migration: IF NOT EXISTS everywhere, safe to re-run without altering hypertable.
-- Migrator must serialize DDL with pg_advisory_lock (e.g., SELECT pg_advisory_lock(727271) / pg_advisory_unlock(727271)).
-- No create_hypertable here; telemetry hypertable (SPEC-001) remains partitioned by received_at.
-- PostGIS is optional for Step2 (read path without zones). If not available, critical_zones is created without postgis checks and GIST is skipped.

DO $$
BEGIN
  CREATE EXTENSION IF NOT EXISTS postgis;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'postgis not available - critical_zones without postgis checks';
END
$$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis') THEN
    CREATE TABLE IF NOT EXISTS critical_zones (
      id UUID PRIMARY KEY,
      name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
      geom geometry(Polygon,4326) NOT NULL CHECK (ST_IsValid(geom) AND ST_GeometryType(geom) = 'ST_Polygon' AND ST_Area(geom) > 0 AND ST_NPoints(geom) BETWEEN 4 AND 101),
      created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );
    CREATE INDEX IF NOT EXISTS critical_zones_geom_gist ON critical_zones USING GIST (geom);
    CREATE INDEX IF NOT EXISTS telemetry_geom_idx ON telemetry USING GIST ((geom::geometry));
  ELSE
    CREATE TABLE IF NOT EXISTS critical_zones (
      id UUID PRIMARY KEY,
      name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
      geom TEXT NOT NULL,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );
    RAISE NOTICE 'critical_zones created without postgis, GIST skipped';
  END IF;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'critical_zones creation failed';
END
$$;
