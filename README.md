# fleet-monitoring-platform
Event-driven fleet monitoring platform with agentic AI telemetry analysis, reactive web portal, and offline-first mobile app.

## 1. Levantar en 3 pasos (Quickstart)

**Requisitos:** `Docker ≥24 + Compose v2`, `Go 1.25`, `Node ≥20` (solo para `web`/`newman`), `colima` en macOS `colima start --cpu 4 --memory 8 --arch x86_64` (16 GB máquina).

```bash
# 1) secrets (nunca en git)
cp .env.example .env
# edita POSTGRES_PASSWORD y GF_SECURITY_ADMIN_PASSWORD si quieres (dev default fleet/change-me)

# 2) levantar core (api + web + lb + ingest + consumer + nats + timescaledb) — 30-40s primera vez
docker compose up --build --wait
docker compose ps   # todos (healthy)
# api       127.0.0.1:8083->8080   (healthy)
# web       127.0.0.1:5173->80     (healthy)
# lb        127.0.0.1:8080->80     (healthy)
# ingest    127.0.0.1:8081->8080   (healthy)
# consumer  127.0.0.1:8082->8081   (healthy)
# nats      127.0.0.1:4222/8222    (healthy)
# timescaledb 127.0.0.1:5432       (healthy)

# 3) smoke (vía LB — único entry point)
curl -s http://localhost:8080/api/healthz | jq
# {"status":"ok","breaker":"closed","nats":"connected","db":"total=1 idle=1"}
curl -s http://localhost:8080/healthz | jq        # LB → ingest
curl -s http://localhost:8080/metrics | head -n 5 # ingest
curl -s http://localhost:8083/metrics | head -n 5 # api: breaker_state api_sse_connections p95_latency_ms
open http://localhost:5173                        # web Leaflet (ver §5)
```

**Parar / reset:**
```bash
docker compose logs -f api web lb ingest consumer   # ver logs
docker compose down             # apaga sin borrar datos
docker compose down -v          # resetea pgdata/nats_data (hypertable + streams)
docker compose config -q && echo "compose ok"
```

### Con observabilidad (+2GB RAM) y docs C4

```bash
# observabilidad: prometheus, grafana, loki, tempo (profile)
docker compose --profile observability up --build --wait
# o si ya tienes el core Up:
docker compose --profile observability up -d --wait
# UI: Prometheus 9090, Grafana 3000 (admin/change-me), Loki 3100, Tempo 3200

# docs C4 Structurizr:
docker compose --profile docs up structurizr  # http://localhost:8088 → Contenedores_N2

# bajar solo observabilidad sin apagar core:
docker compose --profile observability down
```

---

## 2. Cómo probar con Postman (flujo completo)

> `baseUrl = http://localhost:8080` (via LB). No uses `8083` directo salvo debug. Ingesta siempre baseUrl. Web en `http://localhost:5173`.

**Requisitos:** `Postman Desktop` (o `newman` CLI `npm i -g newman`).

### 2.1 Importar

`Postman → File → Import → Folder` `infra/postman/`:

| Colección | Para qué | Requests |
|-----------|----------|----------|
| `Fleet.postman_collection.json` | **SPEC-001** ingesta `POST /v1/telemetry` single/batch | 8 (202, 400 plate, 429, 503, dedup) |
| `Fleet.read.postman_collection.json` | **SPEC-002** read BFF `GET /api/fleet/positions` + history + health/metrics + SSE notas | 10 (keyset, plate filter, cursor, zona) |

Variables: `baseUrl=http://localhost:8080` (ya viene), `fleet_next_cursor` se setea auto tras `limit=2`.

### 2.2 Flujo recomendado (orden — 3 min)

**Paso A — Cargar datos (SPEC-001):**
1. Abre `Fleet` → `POST /v1/telemetry single GTP980 202` → **Send** → `202 {"accepted":true}` (va por NATS → consumer → TimescaleDB).
2. Cambia `plate` en Body (`GTP980` → `ABC123`, `TTY423`) y reenvía 2-3 veces → tendrás 3 placas con posiciones.
3. Prueba `dedup`: reenvía mismo `client_event_id` → `202` pero no duplica (ver `SELECT count(*) FROM telemetry`).
4. Prueba `invalid plate 400`: `GTP98` → `400`.

**Paso B — Probar read BFF (SPEC-002 Read):**
1. `GET /api/fleet/positions?limit=2` → `200` valida `vehicles.length==2`, `next_cursor` base64 `plate|RFC3339Nano`, `status` `moving/idle`, 6 dec (`pm.test` verde).
2. `GET /api/fleet/positions page2 with cursor` (usa `{{fleet_next_cursor}}`) → `200` restante.
3. `GET /api/fleet/positions?plate=GTP980&limit=100` → solo ese (`BR-012`, toggle "Ver todos" del mapa).
4. `GET /api/fleet/positions?plate=GTP98 400 invalid` → `400`.
5. `GET /api/vehicles/GTP890/history?from=...&to=...&limit=5` → `200` `DESC`.
6. `GET /api/vehicles/GTP89/history 400` y `from>to 400` → `400`.
7. `GET /api/healthz` y `GET /api/metrics` → `200` (health `breaker/nats/db`, metrics `breaker_state api_sse_connections p95`).

**Paso C — Zonas (Step 3, GeoJSON):**
*No hay colección dedicada — usar curl o importar snippet:*
```bash
curl -s -X POST http://localhost:8080/api/zones -H 'Content-Type: application/json' -d '{"name":"Norte","geojson":{"type":"Polygon","coordinates":[[[-74.07,4.71],[-74.05,4.71],[-74.05,4.73],[-74.07,4.73],[-74.07,4.71]]]}}' | jq
# 201 con id UUID — validar 400 con 3 pts o no cerrado
curl -s http://localhost:8080/api/zones | jq  # FeatureCollection roja fillOpacity 0.2 (mapa)
```

**Paso D — SSE (Postman NO soporta EventSource — usar curl):**
```bash
# fleet:position todos (ver todos)
curl -N -H 'Accept: text/event-stream' http://localhost:8080/api/fleet/positions/stream
# fleet:position solo GTP980
curl -N -H 'Accept: text/event-stream' 'http://localhost:8080/api/fleet/positions/stream?plate=GTP980'
# alerts (4 tipos zone_enter/exit speeding_on/off)
curl -N -H 'Accept: text/event-stream' http://localhost:8080/api/alerts
# replay: Last-Event-ID (7d retention ALERTS)
curl -N -H 'Accept: text/event-stream' -H 'Last-Event-ID: 100' http://localhost:8080/api/alerts
# espera event: fleet:position id: seq data: {plate,lat,lon,speed,received_at} + :ping 15s + retry:5000
```

**Paso E — Mapa (Step 6):**
Abre `http://localhost:5173` → `MapContainer` con `TileLayer https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png` directo (no `/api/tiles`), `MarkerClusterGroup` activo con 600 markers → DOM <500, overlay zonas rojo.

### 2.3 CLI (CI / sin Postman Desktop)

```bash
npm i -g newman
newman run infra/postman/Fleet.postman_collection.json --env-var baseUrl=http://localhost:8080 --reporters cli
newman run infra/postman/Fleet.read.postman_collection.json --env-var baseUrl=http://localhost:8080 --reporters cli
# ambos deben: 0 failed
```

---

## 3. Puertos (entry point único LB)

| Puerto host | Servicio | Interno | Perfil | Notas |
|---|---|---|---|---|
| **127.0.0.1:8080** | **lb (nginx)** | 80 | default | **Único entry point** — `POST /v1/telemetry`, `GET /api/*`, `/healthz`, `/metrics` → `proxy_buffering off 3600s` para SSE |
| 127.0.0.1:8081 | ingest | 8080 | default | Directo ingest (bypass LB, DX only) |
| 127.0.0.1:8082 | consumer | 8081 | default | Health consumer (loopback) |
| 127.0.0.1:8083 | api (BFF) | 8080 | default | Directo api (bypass LB) |
| 127.0.0.1:5173 | web | 80 | default | SPA Vite (Leaflet) |
| 127.0.0.1:4222 | nats | 4222 | default | NATS client |
| 127.0.0.1:8222 | nats | 8222 | default | `/varz` |
| 127.0.0.1:7777 | nats-exporter | 7777 | default | Prometheus |
| 127.0.0.1:5432 | timescaledb | 5432 | default | PG15 (`POSTGRES_PORT` env) |
| 127.0.0.1:9090 | prometheus | 9090 | observability | Targets `api:8080/metrics` + `ingest` `consumer` |
| 127.0.0.1:3000 | grafana | 3000 | observability | `admin / change-me` |
| 127.0.0.1:3100 | loki | 3100 | observability | |
| 127.0.0.1:3200/4317/4318 | tempo | 3200 | observability | OTLP |
| 127.0.0.1:8088 | structurizr | 8080 | docs | C4 |

`curl http://localhost:8083/metrics | grep -E 'breaker_state|api_sse|p95|jetstream'` para ver BFF sin LB.

---

## 4. API — Ejemplos curl directos

```bash
# health + breaker — ingest y api BFF
curl -s http://localhost:8080/healthz | jq              # LB → ingest
curl -s http://localhost:8080/api/healthz | jq           # LB → api
curl -s http://localhost:8083/healthz | jq               # directo api

# read BFF (requiere datos previos §2)
curl -s "http://localhost:8080/api/fleet/positions?limit=2" | jq
curl -s "http://localhost:8080/api/fleet/positions?plate=GTP980&limit=2" | jq
curl -s "http://localhost:8080/api/vehicles/GTP980/history?from=2026-08-24T00:00:00Z&to=2026-08-24T23:59:59Z&limit=5" | jq

# ingest single/batch (SPEC-001)
curl -s -X POST http://localhost:8080/v1/telemetry -H 'Content-Type: application/json' -d '{"plate":"GTP980","speed":42,"lat":4.711,"lon":-74.072,"client_event_id":"550e8400-e29b-41d4-a716-446655440001","occurred_at":"2026-08-23T12:00:00Z"}' | jq
curl -s -X POST http://localhost:8080/v1/telemetry/batch -H 'Content-Type: application/json' -d '{"events":[{"plate":"GTP890","speed":10,"lat":4.7,"lon":-74.072,"client_event_id":"550e8400-e29b-41d4-a716-446655440002"}]}' | jq
```

Contratos: `docs/specs/SPEC-001-telemetry-ingest/contracts/http.openapi.yaml`, `docs/specs/SPEC-002-fleet-read-zones/contracts/http.openapi.yaml` + `events.asyncapi.yaml`.

---

## 5. Observabilidad — Visualizar vía web

Stack opcional `--profile observability` (core 6-9GB, +observability ~2GB):

| URL | Qué ver |
|---|---|
| `http://localhost:9090` | Prometheus → `Status → Targets` Up (`api:8080/metrics` `ingest` `consumer` `nats-exporter`) → `Graph` queries abajo |
| `http://localhost:3000` | Grafana `Dashboards → Fleet` (p95, breaker, sse_clients) — `admin / change-me` (`GF_SECURITY_ADMIN_*`) |
| `http://localhost:3100/ready` | Loki |
| `http://localhost:3200/status` | Tempo |
| `http://localhost:7777/metrics` | NATS exporter raw |
| `http://localhost:8222/varz` | NATS JetStream JSON |

**Queries Prometheus (Graph → Execute, Range 5m):**
```promql
breaker_state                    # 0 closed 1 half-open 2 open
api_sse_connections              # SSE clients fleet:position
p95_latency_ms                   # NFR-001 p95 <150ms GET /fleet/positions
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{job="api"}[5m]))
up{job="api"}                    # 1 healthy
jetstream_bytes                  # vs max_bytes 5GB (alerta >=80%)
num_ack_pending{stream="TELEMETRY"}
```

**Sin Grafana:**
```bash
curl -s http://localhost:9090/api/v1/query?query=breaker_state | jq
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
curl -s http://localhost:8083/metrics | grep -E 'breaker_state|api_sse|p95'
```

---

## 6. Arquitectura — C4

Fuente de verdad: `docs/c4/workspace.dsl`. Ver [Nivel 1](docs/c4/01-context.md) y [Nivel 2](docs/c4/02-containers.md).

### Nivel 1 — Contexto
![C4 Nivel 1 — Contexto_N1](docs/c4/Contexto_N1-thumbnail.png)

### Nivel 2 — Contenedores
![C4 Nivel 2 — Contenedores_N2](docs/c4/Contenedores_N2-thumbnail.png)

```bash
docker run --rm -v "$(pwd)/docs/c4:/work" structurizr/structurizr validate -workspace /work/workspace.dsl
docker compose --profile docs up structurizr  # http://localhost:8088
```

## 7. Contratos

```bash
npx @redocly/cli lint docs/specs/SPEC-001-telemetry-ingest/contracts/http.openapi.yaml
npx @redocly/cli lint docs/specs/SPEC-002-fleet-read-zones/contracts/http.openapi.yaml
npx @asyncapi/cli validate docs/specs/SPEC-001-telemetry-ingest/contracts/events.asyncapi.yaml
```

## 8. Testing

```bash
# unit + handler (sin infra)
go test ./...           # backend/
go vet ./...

# integración (requiere compose up --wait)
go test -tags=integration ./...

# un paquete
go test -run TestPlate ./internal/shared/domain -v
go test -run TestHandler ./internal/telemetry/adapters/http -v

# web
cd web && npm test -- --run && npm run build

# lint clean architecture
golangci-lint run ./...
```

## 9. Decisiones (ADRs)

| ADR | Decisión | Tradeoff |
|---|---|---|
| [ADR-0001](docs/adr/0001-nats-jetstream-backbone.md) | **NATS JetStream** vs Kafka/RabbitMQ | Un binario 50-150 MB, `replicas 1 dev / 3 prod`, `max_bytes 5GB dev`, ~400k msg/s sobrado para 1-3k msg/s. Kafka ~100× RAM rompe 16 GB. |
| ADR Timescale | **TimescaleDB** hypertable `telemetry` | `chunk 1 día`, `PK(client_event_id, received_at)`, `index(plate, received_at DESC)`, `CopyFrom 500-1000`, `geom GEOGRAPHY(Point,4326) GENERATED`. |
| [ADR-0005](docs/adr/0005-modular-monolith-vs-microservices.md) | Monolito modular Go | Sin orquestador local; LB nginx. Particionado futuro por `telemetry.raw.{shard}` si >10k msg/s. |
| [ADR-0006](docs/adr/0006-ingest-load-balancer.md) | LB + rate limiting dual | `12 evt/min burst 20` online + `1 batch/5s 500/30s` offline → `429`; `PublishAsyncMaxPending` / breaker → `503`. |

Ver `docs/adr/`.

## 10. Auditoría de IA — Exoesqueleto, no muleta

> Al menos 2 decisiones donde la IA sugirió enfoque deficiente y se forzó el estándar. Bitácora en [`docs/IAUDIT.md`](docs/IAUDIT.md).

**Caso 1 — Polígono sin cierre [SPEC-002]:** IA generó `Polygon` sin `first==last`, 3 pts y sin tope → `BR-002` forzada `first==last 4..101 coords ST_Area>0 ST_IsValid SRID 4326` + `CHECK(ST_Area>0 AND ST_NPoints BETWEEN 4 AND 101)`.

**Caso 2 — Detector `stopped >20m` como ticker [SPEC-002→003]:** IA propuso `ticker 30s` push a `SSE /api/alerts` → movido a tool read-only `findVehiclesStoppedInCriticalZones` en SPEC-003, SSE genérico `alerts.critical` con `dedup 2m`.
