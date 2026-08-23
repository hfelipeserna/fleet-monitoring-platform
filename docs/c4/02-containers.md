# C4 Nivel 2 — Contenedores

> Fuente: `docs/c4/workspace.dsl` vista `Contenedores_N2`. Deriva de ADR-0001 (NATS), ADR-0002 (monorepo 4 bins), ADR-0005 (monolito modular), ADR-0006 (LB), ADR-0007 (Leaflet OSM).

## Diagrama

**Fuente de verdad:** `workspace.dsl`. No editar PNG a mano.

![C4 Nivel 2 - Contenedores_N2](Contenedores_N2-thumbnail.png)

```bash
# validar
docker run --rm -v "$(pwd)/docs/c4:/work" structurizr/structurizr validate -workspace /work/workspace.dsl
# ver interactivo
docker compose --profile docs up structurizr  # http://localhost:8080 -> Contenedores_N2
```

## Contenedores (9 dentro de Fleet monitoring platform)

| Contenedor | Tecnología | Propósito | ADR |
|---|---|---|---|
| **Mobile App** | React Native Expo, SQLite/WatermelonDB | Conductor envía telemetría offline-first, cola local, sync batch 100-500 | sec. 4.D, ADR-0006 cond.4 |
| **Web Application** | React Vite, Leaflet + markercluster, SSE | Operador ve mapa, alertas, chat | sec. 4.C, ADR-0007 |
| **Load Balancer** | nginx:alpine / ALB | L7 TLS, healthcheck `/healthz`, drain 15-30s, round-robin | ADR-0006 |
| **Ingest API** | Go `cmd/ingest` | Write edge: valida -> `PublishAsync` + `PublishAsyncComplete` antes del 200 | ADR-0001 cond.2-3 |
| **NATS JetStream** | NATS JetStream (streams TELEMETRY/ALERTS) | Bus durable, replay, backpressure (`MaxAckPending`, `MaxBytes`) | ADR-0001 |
| **Consumer Worker** | Go `cmd/consumer` | Pull durable, batch `CopyFrom` 500-1000 a TimescaleDB, publica `alerts.*` | ADR-0001 cond.4 |
| **TimescaleDB** | TimescaleDB + PostGIS | Hypertable `(device_id,time)` + `critical_zones` + continuous aggregates | ADR-0001 cond.4 |
| **Platform API** | Go `cmd/api` | Read edge, BFF: `GET /api/*`, `SSE /api/alerts`, `POST /api/chat` -> agent | ADR-0003 cond.9, ADR-0005 |
| **AI Agent** | Go `cmd/agent` + Genkit | Flows + tools read-only (`ST_Within` zonas) -> Gemini | ADR-0003 |

**Externos:** `Gemini API` (LLM) y `Map Tile Provider` (OSM raster).

## Flujos clave

**Ingesta (conductor):**
`Mobile -> LB -> Ingest -> NATS(TELEMETRY) -> Consumer -> TimescaleDB`
`Consumer -> NATS(ALERTS)` si genera alerta

**Lectura + IA (operador):**
`Web -> Platform API -> TimescaleDB` (dashboard)
`Web -> Platform API -> NATS(ALERTS) -> Web` (SSE)
`Web -> Platform API -> AI Agent -> Gemini` (chat) + `Agent -> TimescaleDB/NATS` (tools read-only)

**Mapas:**
`Web -> Map Tile Provider` directo (nunca proxy por `api`), `Web <- Platform API <- TimescaleDB` para GeoJSON de zonas críticas (ADR-0007)

## Decisiones trazables

*   **Monolito modular, no microservicios:** 1 Go module, 4 bins replicables. NATS desacopla, no hay RPC entre bins (ADR-0005). LB habilita 2-3 réplicas `ingest` sin mesh (ADR-0006).
*   **Backpressure acotado:** `PublishAsyncMaxPending 512-1024` por réplica + `MaxAckPending` en consumer (ADR-0001 cond.2). `503` = infra saturada, `429` = quota por device (distintos).
*   **BFF Obligatorio:** `web` nunca toca `db/nats/genkit` directo (ADR-0003 cond.9). Todo pasa por `api`.
*   **Zonas críticas canónicas:** `GET /api/zones` GeoJSON alimenta mapa y tool del agente (ADR-0007). Misma `geom` para `ST_Within`.

## Qué NO va en Nivel 2

*   Componentes internos Go (domain/application/adapters) -> Nivel 3
*   Infra Terraform/AWS detalle -> vista Despliegue
*   CI/CD (GitHub Actions, Fastlane) -> vista Despliegue

## Siguiente

Nivel 3 — Componentes de `Ingest API` / `Platform API` / `AI Agent` con clean architecture (domain->application->adapters->infra, depguard).
