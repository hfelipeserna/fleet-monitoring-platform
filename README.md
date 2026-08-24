# fleet-monitoring-platform
Event-driven fleet monitoring platform with agentic AI telemetry analysis, reactive web portal, and offline-first mobile app.

## Arquitectura — C4

Fuente de verdad: `docs/c4/workspace.dsl` (Structurizr DSL). Ver detalle en [Nivel 1](docs/c4/01-context.md) y [Nivel 2](docs/c4/02-containers.md).

### Nivel 1 — Contexto

![C4 Nivel 1 — Contexto_N1](docs/c4/Contexto_N1-thumbnail.png)

**Personas y sistema:** `Operador de Flota` visualiza el portal web, `Conductor` envía telemetría GPS, `Fleet monitoring platform` orquesta ingesta + IA + dashboard, `Gemini API` resuelve consultas en lenguaje natural (sec. 4.B).

### Nivel 2 — Contenedores

![C4 Nivel 2 — Contenedores_N2](docs/c4/Contenedores_N2-thumbnail.png)

9 contenedores del monolito modular + LB + NATS + TimescaleDB. Ver detalle en [Nivel 2](docs/c4/02-containers.md).

```bash
# validar DSL
docker run --rm -v "$(pwd)/docs/c4:/work" structurizr/structurizr validate -workspace /work/workspace.dsl
# ver interactivo
docker compose --profile docs up structurizr  # http://localhost:8080 -> Contenedores_N2
```

## Requisitos

| Herramienta | Versión | Notas |
|---|---|---|
| Go | 1.25 | `backend/go.mod` — `go vet` / `golangci-lint` |
| Docker | ≥ 24 + Compose v2 | `docker compose config` debe validar |
| colima | recomendado macOS | `colima start --cpu 4 --memory 8 --arch x86_64` (máquina 16 GB, sin emulación amd64) |
| Node | ≥ 20 | solo para `newman` (Postman CLI) y lint de contratos |
| Make / Bash | - | helpers `infra/` |

> Secrets vía `.env` (nunca en git). Ver `.env.example`.

## Cómo levantar

### Opción A — Solo core (default, 6-9GB RAM)

```bash
cp .env.example .env  # edita POSTGRES_PASSWORD, GF_SECURITY_ADMIN_PASSWORD si quieres

# levanta api, lb, ingest, consumer, nats, timescaledb y espera a healthy (30-40s primera vez por tune TimescaleDB)
docker compose up --build --wait

# verifica
docker compose ps
# api          Up (healthy)   127.0.0.1:8083->8080
# lb           Up (healthy)   0.0.0.0:8080->80
# ingest       Up (healthy)   0.0.0.0:8081->8080
# consumer     Up (healthy)   127.0.0.1:8082->8081
# nats         Up (healthy)   127.0.0.1:4222
# timescaledb  Up (healthy)   127.0.0.1:5432

# smoke rápido
curl -s http://localhost:8083/healthz | jq
# {"status":"ok","breaker":"closed","nats":"connected","db":"total=1 idle=1"}
curl -s http://localhost:8080/api/healthz | jq  # vía LB (proxy_buffering off para SSE)

# down / reset
docker compose logs -f api ingest consumer  # ver logs
docker compose down -v                      # borra pgdata/nats_data (resetea hypertable)
```

### Opción B — Con observabilidad (PROM/Grafana/Loki/Tempo, +2GB RAM)

```bash
# mismo core + prometheus, grafana, loki, tempo (profile observability)
docker compose --profile observability up --build --wait

docker compose --profile observability ps
# + prometheus   Up (healthy) 0.0.0.0:9090->9090
# + grafana      Up (healthy) 0.0.0.0:3000->3000
# + loki         Up           0.0.0.0:3100->3100
# + tempo        Up           0.0.0.0:3200->3200 + 4317/4318

# solo observabilidad (si ya tienes el core levantado)
docker compose --profile observability up -d --wait

# bajar solo observabilidad sin apagar el core
docker compose --profile observability down
```

### Opción C — Con docs C4

```bash
docker compose --profile docs up structurizr  # http://localhost:8088 → Contenedores_N2
```

### Validación infra

```bash
docker compose config                    # válido sin warnings (default)
docker compose --profile observability config  # valida prometheus/grafana/loki/tempo
```

## Puertos

| Puerto host | Servicio | Interno | Perfil | Descripción |
|---|---|---|---|---|
| 8080 | lb (nginx) | 80 | default | Único entry point — `POST /v1/telemetry`, `GET /api/fleet/positions`, `/api/vehicles/{plate}/history`, `/healthz`, `/metrics` → `proxy_buffering off` para SSE |
| 8081 | ingest | 8080 | default | Directo ingest (bypass LB) |
| 127.0.0.1:8082 | consumer | 8081 | default | Health consumer (loopback) |
| 127.0.0.1:8083 | api (BFF) | 8080 | default | Directo api BFF (bypass LB) |
| 127.0.0.1:4222 | nats | 4222 | default | NATS client |
| 127.0.0.1:8222 | nats | 8222 | default | NATS `/varz` |
| 127.0.0.1:7777 | nats-exporter | 7777 | default | `prometheus-nats-exporter` |
| 127.0.0.1:5432 | timescaledb | 5432 | default | TimescaleDB PG15 (`POSTGRES_PORT` env) |
| 9090 | prometheus | 9090 | `observability` | Prometheus — Targets `ingest`, `api`, `consumer`, `nats-exporter` |
| 3000 | grafana | 3000 | `observability` | Grafana (`GF_SECURITY_ADMIN_*`) |
| 3100 | loki | 3100 | `observability` | Loki |
| 3200 | tempo | 3200 | `observability` | Tempo + OTLP `4317`/`4318` |
| 8088 | structurizr | 8080 | `docs` | C4 DSL viewer |

## API — Ejemplos

Base local `http://localhost:8080` (vía LB). Directo `http://localhost:8081` ingest, `http://localhost:8083` api.

```bash
# health + breaker — ingest y api BFF
curl -s http://localhost:8080/healthz | jq              # LB → ingest
curl -s http://localhost:8080/api/healthz | jq           # LB → api
curl -s http://localhost:8083/healthz | jq               # directo api
curl -s http://localhost:8080/metrics | head -n 20       # ingest
curl -s http://localhost:8083/metrics | head -n 20       # api: breaker_state, api_sse_connections, p95_latency_ms

# read BFF — requiere datos previos (ver Quickstart)
curl -s "http://localhost:8080/api/fleet/positions?limit=2" | jq
# {"vehicles":[{"plate":"GTP980","lat":4.711,"lon":-74.072,"speed":42,"received_at":"...","status":"moving"}],"next_cursor":"..."}
curl -s "http://localhost:8080/api/fleet/positions?plate=GTP980&limit=2" | jq
curl -s "http://localhost:8080/api/vehicles/GTP980/history?from=2026-08-24T00:00:00Z&to=2026-08-24T23:59:59Z&limit=5" | jq

# SSE (no soportado en Postman, usar curl)
curl -N -H 'Accept: text/event-stream' http://localhost:8080/api/fleet/positions/stream
curl -N -H 'Accept: text/event-stream' 'http://localhost:8080/api/fleet/positions/stream?plate=GTP980'

# ingest single/batch (SPEC-001)
curl -s -X POST http://localhost:8080/v1/telemetry -H 'Content-Type: application/json' -d '{"plate":"GTP980","speed":42,"lat":4.711,"lon":-74.072,"client_event_id":"550e8400-e29b-41d4-a716-446655440001","occurred_at":"2026-08-23T12:00:00Z"}'
# {"accepted":true}
curl -s -X POST http://localhost:8080/v1/telemetry -H 'Content-Type: application/json' -d '{"plate":"GTP890","speed":0,"lat":null,"lon":null,"client_event_id":"550e8400-e29b-41d4-a716-446655440002"}'
curl -s -X POST http://localhost:8080/v1/telemetry/batch -H 'Content-Type: application/json' -d '{"events":[{"plate":"GTP890","speed":10,"lat":4.7,"lon":-74.072,"client_event_id":"550e8400-e29b-41d4-a716-446655440003"},{"plate":"TTY423","speed":20,"lat":4.71,"lon":-74.07,"client_event_id":"550e8400-e29b-41d4-a716-446655440004"}]}'
```

Contratos: `docs/specs/SPEC-001-telemetry-ingest/contracts/http.openapi.yaml` y `events.asyncapi.yaml`, `docs/specs/SPEC-002-fleet-read-zones/contracts/http.openapi.yaml`.

## Cómo probar en Postman

Requisitos: `Docker Desktop` + `Postman Desktop` (o `newman` CLI).

### Importar

`File → Import → Folder` `infra/postman/`:
* `Fleet.postman_collection.json` — **SPEC-001** ingest
* `Fleet.read.postman_collection.json` — **SPEC-002** read BFF (10 requests)

Variables: `baseUrl = http://localhost:8080` (LB, no `8083`).

### Flujo recomendado (orden)

1. **Cargar datos** con colección **SPEC-001**: ejecuta `POST /v1/telemetry single GTP980 202` 2-3 veces cambiando `plate` (`GTP980`, `ABC123`, `TTY423`); cada `202 {"accepted":true}` va por NATS → `consumer` → TimescaleDB. Verifica `dedup` (mismo `client_event_id` → `202` sin duplicar) y `invalid plate 400`.

2. **Probar read BFF** con colección **SPEC-002 Read**:
   * `GET /api/fleet/positions?limit=2` → `200` `pm.test` valida `vehicles.length==2`, `next_cursor` base64 `plate|RFC3339Nano`, `status`, 6 dec, sin `OFFSET`.
   * `GET /api/fleet/positions page2 with cursor` → `cursor={{fleet_next_cursor}}` → `200` restante.
   * `GET /api/fleet/positions?plate=GTP980` → solo ese (BR-012).
   * `GET /api/vehicles/GTP890/history?from&to&limit=5` → `200` `DESC`.
   * `GET /api/healthz` y `GET /api/metrics` → `200`.
   * SSE: usar `curl` como arriba (Postman no soporta `EventSource`).

### CLI

```bash
npm i -g newman
newman run infra/postman/Fleet.postman_collection.json --env-var baseUrl=http://localhost:8080 --reporters cli
newman run infra/postman/Fleet.read.postman_collection.json --env-var baseUrl=http://localhost:8080 --reporters cli
```

---

## Observabilidad — Cómo levantar y visualizar vía web

Stack opcional `--profile observability` (no carga por defecto; core 6-9GB, +observability ~2GB):

### Levantar

```bash
# Opción A: todo junto
docker compose --profile observability up --build --wait
# Opción B: si ya tienes el core Up, añade solo observabilidad
docker compose --profile observability up -d --wait
```

### Visualizar vía web

| URL | Qué ver | Credenciales |
|---|---|---|
| `http://localhost:9090` | **Prometheus** → `Status → Targets` deben estar `Up`: `api:8080/metrics`, `ingest:8080/metrics`, `consumer:8081/metrics`, `nats-exporter:7777`, `nats:8222/varz`. `Graph` → queries abajo | — |
| `http://localhost:3000` | **Grafana** → `Dashboards → Fleet` (Ingest + Read). Datasources `Prometheus`, `Loki`, `Tempo` ya provisionados | `admin` / `change-me` (ver `.env` `GF_SECURITY_ADMIN_*`) |
| `http://localhost:3100/ready` | **Loki** `ready` | — |
| `http://localhost:3200/status` | **Tempo** `ingesting` | — |
| `http://localhost:7777/metrics` | **NATS exporter** raw | — |
| `http://localhost:8222/varz` | **NATS** JSON `jetstream` | — |

Grafana viene provisionado en `infra/observability/grafana/` (datasources + dashboards `fleet-read.json` con `p95`, `breaker`, `sse_clients`, `jetstream_bytes`). Si no ves datos, espera 15s (scrape `15s`) y genera tráfico (`POST /v1/telemetry` + `GET /api/fleet/positions`).

### Queries Prometheus útiles (Graph → Execute, Range 5m, Step 15s)

```promql
breaker_state                    # 0 closed, 1 half-open, 2 open
api_sse_connections              # SSE clients (fleet:position)
p95_latency_ms                   # NFR-001 p95 <150ms para GET /fleet/positions
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{job="api"}[5m]))
up{job="api"}                    # 1 = healthy
jetstream_bytes                  # vs max_bytes 5GB (alerta >=80%)
num_ack_pending{stream="TELEMETRY"}
rate(http_requests_total{job="ingest",code="429"}[5m])  # rate limit por placa
rate(http_requests_total{job="ingest",code="503"}[5m])  # backpressure
```

### Verificar sin Grafana (curl)

```bash
curl -s http://localhost:8083/metrics | grep -E 'breaker_state|api_sse|p95|jetstream'
curl -s http://localhost:9090/api/v1/query?query=breaker_state | jq
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
docker compose logs -f api | grep -E 'plate|cursor|breaker'
docker compose logs -f ingest | grep ingest_inflight
```

### Apagar observabilidad sin apagar el core

```bash
docker compose --profile observability down
docker compose ps  # api/lb/ingest siguen Up
```

## Contratos

```bash
# OpenAPI 3.1
npx @redocly/cli lint docs/specs/SPEC-001-telemetry-ingest/contracts/http.openapi.yaml
npx @readme/openapi-parser docs/specs/SPEC-001-telemetry-ingest/contracts/http.openapi.yaml

# AsyncAPI 2.x (telemetry.raw.{plate}, Nats-Msg-Id=client_event_id)
npx @asyncapi/cli validate docs/specs/SPEC-001-telemetry-ingest/contracts/events.asyncapi.yaml
```

CI valida `docker compose config` y lint de contratos en cada PR.

## Testing

```bash
# unit + handler (sin infra)
go test ./...                         # backend/  (vet incluido en CI)
go vet ./...

# con integración (requiere docker compose up --wait: NATS + TimescaleDB reales)
go test -tags=integration ./...       # incluye CopyFrom, DLQ, breaker, stream

# un paquete
go test -run TestPlate ./internal/shared/domain -v
go test -run TestHandler ./internal/telemetry/adapters/http -v

# lint clean architecture (domain/application stdlib-only)
golangci-lint run ./...
```

Tags: `//go:build integration` para tests que tocan NATS/TimescaleDB; sin tag corren offline.

## Decisiones (ADRs)

| ADR | Decisión | Tradeoff |
|---|---|---|
| [ADR-0001](docs/adr/0001-nats-jetstream-backbone.md) | **NATS JetStream** vs Kafka/RabbitMQ/Redis Streams | Un binario 50-150 MB, `storage=file`, `replicas 1 dev / 3 prod`, `max_bytes 5 GB dev`, `MaxDeliver 3 -> telemetry.dlq`, ~400k msg/s async (sobrado para 1-3k msg/s). Kafka descartado: ~100× RAM y operativa para mismo throughput; rompe límite 16 GB. At-least-once → idempotencia por `client_event_id` (MsgId + `ON CONFLICT DO NOTHING`). |
| ADR Timescale | **TimescaleDB** hypertable `telemetry` (`received_at` server time) | `chunk 1 día`, `PK(client_event_id, received_at)`, `index(plate, received_at DESC)`, `CopyFrom 500-1000`, `geom GEOGRAPHY(Point,4326) GENERATED` desde `lat/lon`. Compresión 5-15× futura. Single-writer 15-50k filas/s es cuello real, no el broker. |
| [ADR-0005](docs/adr/0005-modular-monolith-vs-microservices.md) | Monolito modular Go (`ingest` + `consumer` binarios) | Sin orquestador local; LB `nginx:alpine` round-robin. Particionado futuro por `telemetry.raw.{shard}` si >10k msg/s. |
| [ADR-0006](docs/adr/0006-ingest-load-balancer.md) | LB + rate limiting dual | `12 evt/min burst 20` online + `1 batch/5s 500/30s` offline → `429`; `PublishAsyncMaxPending` / `jetstream_bytes>=80%` / breaker → `503` distinto. |

Ver `docs/adr/` y `docs/PRUEBA-TECNICA.md` sec. 4.A.

## Auditoría de IA — Exoesqueleto, no muleta

> Requisito del entregable: al menos 2 decisiones donde la IA sugirió un enfoque deficiente y se forzó el estándar. Bitácora completa en [`docs/IAUDIT.md`](docs/IAUDIT.md).

### Caso 1 — Polígono sin cierre obligatorio y sin límite de vértices [SPEC-002]

**Hallazgo IA:** el asistente generó `PolygonGeometry` con `coordinates` sin validar `first==last`, aceptando 3 posiciones para triángulo (`[[a,b],[c,d],[e,f]]`) y sin tope de vértices ni `ST_Area>0`. El `CHECK` de `critical_zones` era solo `ST_IsValid(geom)`.

**Por qué falla:** GeoJSON RFC 7946 exige LinearRing cerrado `first==last` y `>=4` posiciones; una línea degenerada `[[0,0],[1,0],[0,0]]` con área 0 es `ST_IsValid=true` pero no es zona. Sin `ST_NPoints<=101` un polígono de 10k coords es DoS para `GIST` y `ST_Within` (O(n) por punto) y rompe NFR-001.

**Decisión forzada (architect):** `BR-002` reescrita a `first==last, 4..101 coords (<=100 vértices), SRID 4326, ST_Area>0, ST_IsValid`. Contrato OpenAPI `minItems:4 maxItems:101` + `CHECK(ST_Area>0 AND ST_NPoints BETWEEN 4 AND 101)` + validación Go en 2 capas (`ST_Area==0 ->400` antes de SQL). Mermaid y `AC-003/TS-003` ahora cubren `>101 coords ->400` y `área 0 colineal ->400`. Ver `docs/IAUDIT.md#2026-08-24-poligono`.

### Caso 2 — Detector `stopped >20m in zone` como ticker del dashboard [SPEC-002 → SPEC-003]

**Hallazgo IA:** el asistente propuso `FR-005` como detector continuo `ticker 30s -> SELECT ST_Within ... speed=0 >20m -> Publish alerts.critical` y lo modeló como `Flow 2` de `SPEC-002` con `Nats-Msg-Id=plate:zone:bucket`.

**Por qué falla:** `PRUEBA-TECNICA.md sec 4.B` formula `¿vehículos >20m en zonas críticas?` como **consulta del chat** (Genkit tool bajo demanda), no como alerta push del dashboard. Acoplarlo al `SSE /api/alerts` crea una regla de negocio que parece endpoint, duplica la fuente de verdad (mapa vs agente) y obliga a retener `hour` de histórico para cada tick.

**Decisión forzada (architect):** eliminado `Flow 2` de `SPEC-002`; `SSE /api/alerts` queda genérico (`alerts.critical {plate, alert_type}`) con `dedup Nats-Msg-Id` de 2m. La evaluación `ST_Within + >20m` se mueve a `SPEC-003` como tool read-only `findVehiclesStoppedInCriticalZones(durationMin, zoneId)` que consulta `GET /api/zones` canónico. `BR-001/BR-004` y `AC-005` reescritos a alerta genérica. Ver `docs/IAUDIT.md#2026-08-24-detector`.

Ver `docs/adr/` y `docs/PRUEBA-TECNICA.md` sec. 4.A.
