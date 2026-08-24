-- 409 por nombre duplicado (case-insensitive)
-- Idempotente: IF NOT EXISTS permite re-ejecutar sin error; lower(name) garantiza unicidad case-insensitive.
-- Comentario: índice único para detectar conflicto 409 Conflict por nombre duplicado en POST/PUT /api/zones.
CREATE UNIQUE INDEX IF NOT EXISTS critical_zones_name_unique ON critical_zones (lower(name));
