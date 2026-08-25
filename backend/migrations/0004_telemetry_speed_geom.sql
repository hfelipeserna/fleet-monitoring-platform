-- SPEC-003 Step 3: speed=0 partial indexes for FindStoppedInZones
-- BTREE partial for speed=0 + received_at pruning drives time filter; GIST partial for ST_Within when speed=0
-- EXPLAIN for stopped query should show: Bitmap Index Scan on telemetry_speed0_received_at_idx and/or GIST on geom, not Seq Scan
-- SELECT DISTINCT ON (plate) ... WHERE speed=0 AND received_at <= now() - $1 AND ST_Within(geom::geometry, cz.geom)
-- IF NOT EXISTS ensures idempotence; hypertable chunk pruning still benefits from received_at index
-- Migrator must use pg_advisory_lock as in 0002

DO $$
BEGIN
  -- BTREE partial: accelerates WHERE speed=0 AND received_at <= now() - interval and chunk pruning on hypertable
  CREATE INDEX IF NOT EXISTS telemetry_speed0_received_at_idx ON telemetry (received_at DESC) WHERE speed = 0;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'telemetry_speed0_received_at_idx creation skipped';
END
$$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis') THEN
    -- GIST partial for ST_Within when speed=0; complements existing telemetry_geom_idx but filtered to stopped vehicles
    CREATE INDEX IF NOT EXISTS telemetry_speed0_geom_idx ON telemetry USING GIST ((geom::geometry)) WHERE speed = 0;
  END IF;
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'telemetry_speed0_geom_idx creation skipped';
END
$$;

-- EXPLAIN example (run with DATABASE_URL set):
-- EXPLAIN SELECT DISTINCT ON (t.plate) t.plate FROM telemetry t JOIN critical_zones cz ON ST_Within(t.geom::geometry, cz.geom) WHERE t.speed=0 AND t.received_at <= now() - interval '20 min' LIMIT 20;
-- Expected plan: Custom Scan (ChunkAppend) -> Index Scan using telemetry_speed0_received_at_idx or BitmapAnd with telemetry_speed0_geom_idx, no Seq Scan on telemetry
