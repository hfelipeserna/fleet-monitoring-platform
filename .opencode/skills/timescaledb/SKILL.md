---
name: timescaledb
description: Usa esta skill al modelar datos de telemetría en TimescaleDB con pgx: hypertables, chunking, índices, compresión, continuous aggregates y batch inserts para alta frecuencia. Trigger: TimescaleDB, hypertable, telemetry, continuous aggregate, pgx, batch, retention, SQL, series de tiempo.
---

# TimescaleDB (high-frequency telemetría)

Primero la pregunta técnica de la decisión: **por qué TimescaleDB (y no Postgres vainilla ni una NoSQL)**: Postgres con compromiso transaccional sólido + extensiones que convierten una tabla ordinaria en **hypertable** con chunking temporal y compresión automática. Para miles de dispositivos escribiendo GPS/estado varias veces por minuto, da latencia baja de inserts en caliente y lectura de rangos eficiente sin construir sharding a mano.

## Hypertable

```sql
CREATE TABLE telemetry (
  event_id    UUID NOT NULL,
  device_id   UUID NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  lat         DOUBLE PRECISION,
  lon         DOUBLE PRECISION,
  speed_kmh   DOUBLE PRECISION,
  fuel_pct    DOUBLE PRECISION,
  PRIMARY KEY (event_id, occurred_at)
);
SELECT create_hypertable('telemetry','occurred_at', chunk_time_interval => INTERVAL '1 day');
CREATE INDEX ON telemetry (device_id, occurred_at DESC);
```

- `chunk_time_interval` 1 día es buen punto de partida; ajustá según tasa de ingesta.
- PK clave compuesta `(event_id, occurred_at)` habilita la **dedup por upsert** (ON CONFLICT (event_id, occurred_at)).

## Escritura (pgx)

- SIEMPRE batch: `CopyFrom` de pgx para inserts masivos, o prepared statement por lote dentro de una transacción. Nunca una conexión por evento.
- Pool: `pgxpool.New` con límites; `pool.Acquire` dentro del worker del consumidor.
- Inserts idempotentes: `ON CONFLICT (event_id, occurred_at) DO NOTHING`.
- Circuit breaker (gobreaker) alrededor de las escrituras; si la DB falla, el ingestor NAK el lote en vez de perderlo.

## Lecturas de dashboard

- **Continuous aggregates** (vista materializada incremental) para promedios/max por minuto/hora por vehículo: no escaneés chunks en caliente en cada refresh de dashboard.

```sql
CREATE MATERIALIZED VIEW telemetry_min
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 minute', occurred_at) AS bucket, device_id,
       count(*) AS samples, MAX(speed_kmh) AS max_speed, AVG(speed_kmh) AS avg_speed
FROM telemetry GROUP BY bucket, device_id;
```

- Alertas de "detenido > N minutos": consultar último punto por vehículo (último timestamp) contra el actual; index `(device_id, occurred_at)` lo cubre.

## Compresión y retention

- Policy de compresión para chunks > X días (mejora storage a 10-20x), retention por env var (`RETENTION_DAYS`). Timescale se encarga por policy jobs; no borres a mano.

## Init en Docker

- Usá `/docker-entrypoint-initdb.d` de la imagen oficial `timescale/timescaledb` para crear la extensión, hypertables y políticas en el primer arranque. Idempotente: `IF NOT EXISTS`.