# C4 Nivel 1 — Contexto

> Fuente canónica: `docs/PRUEBA-TECNICA.md` sec. 4.A-D + ADR-0001 (NATS JetStream) + ADR-0003 (Genkit/Gemini) + ADR-0002/0005 (monolito modular).

## Diagrama

**Fuente de verdad:** `docs/c4/workspace.dsl` (Structurizr DSL). Renderiza con Structurizr.

```bash
# 1. levantar docs (perfil docs, no afecta `compose up` normal)
docker compose --profile docs up structurizr
# 2. abrir http://localhost:8080 -> Contexto_N1
# 3. exportar PNG/SVG desde la UI para README/video

# Validar y regenerar exports sin levantar UI:
docker run --rm -v "$(pwd)/docs/c4:/work" structurizr/structurizr validate -workspace /work/workspace.dsl
docker run --rm -v "$(pwd)/docs/c4:/work" structurizr/structurizr export -workspace /work/workspace.dsl -format mermaid -output /work/mermaid
docker run --rm -v "$(pwd)/docs/c4:/work" structurizr/structurizr export -workspace /work/workspace.dsl -format plantuml -output /work/plantuml
```

Fallback Mermaid (compatible GitHub, regenerado desde `workspace.dsl` — no editar a mano):

```mermaid
graph LR
  linkStyle default fill:#ffffff

  subgraph diagram ["C4 Nivel 1 - Contexto - Fleet Monitoring Platform"]
    style diagram fill:#ffffff,stroke:#ffffff

    1["<div style='font-weight: bold'>Operador de Flota</div><div style='font-size: 70%; margin-top: 0px'>[Person]</div><div style='font-size: 80%; margin-top:10px'>Hace uso del portal web (SPA)</div>"]
    style 1 fill:#1e40af,stroke:#152c7a,color:#ffffff
    2["<div style='font-weight: bold'>Conductor</div><div style='font-size: 70%; margin-top: 0px'>[Person]</div><div style='font-size: 80%; margin-top:10px'>Hace uso de la aplicación<br />móvil</div>"]
    style 2 fill:#1e40af,stroke:#152c7a,color:#ffffff
    3("<div style='font-weight: bold'>Fleet monitoring platform</div><div style='font-size: 70%; margin-top: 0px'>[Software System]</div><div style='font-size: 80%; margin-top:10px'>Ingesta de datos, integración<br />con IA y dashboard</div>")
    style 3 fill:#3b82f6,stroke:#295bac,color:#ffffff
    4("<div style='font-weight: bold'>Gemini API</div><div style='font-size: 70%; margin-top: 0px'>[Software System]</div><div style='font-size: 80%; margin-top:10px'>IA generativa</div>")
    style 4 fill:#6b7280,stroke:#4a4f59,color:#ffffff

    1-. "<div>Consulta dashboard y chat con IA</div><div style='font-size: 70%'>[HTTPS - portal web]</div>" .->3
    3-. "<div>Alertas y datos en tiempo real</div><div style='font-size: 70%'>[SSE - /api/alerts]</div>" .->1
    2-. "<div>Envía telemetría GPS</div><div style='font-size: 70%'>[HTTPS - batch offline-first]</div>" .->3
    3-. "<div>Consultas NL</div><div style='font-size: 70%'>[HTTPS - Gemini API via Genkit]</div>" .->4

  end
```

## Elementos

| Elemento | Tipo C4 | Origen | Descripción |
|---|---|---|---|
| **Operador de Flota** | Person | `PRUEBA-TECNICA.md sec. 4.C` | Back-office que usa el Portal (SPA React + SSE + mapa + chat IA). |
| **Conductor** | Person | `sec. 4.D` | Usuario de campo que envía telemetría GPS desde la app Expo offline-first (SQLite/WatermelonDB, sync batch). |
| **Fleet monitoring platform** | Software System | Sistema en scope | Capta ingesta event-driven, persiste en TimescaleDB y expone dashboard + IA. Internamente son 4 bins Go (`ingest/consumer/api/agent`) + NATS JetStream — detalle Nivel 2. |
| **Gemini API** | Software System (External) | `sec. 4.B` + `ADR-0003` | IA generativa externa invocada vía Genkit (`genkit-go`) con `GEMINI_API_KEY`. Free tier Flash; tools read-only con scope por JWT. |

## Relaciones (C4)

| Origen -> Destino | Label | Protocolo | ADR / Spec |
|---|---|---|---|
| `Conductor -> Fleet platform` | `Envía telemetría GPS` | `POST /v1/telemetry/batch` (100-500 events, 1 MB max) + jitter 0-60s | ADR-0001 cond. 3-4, ADR-0006 |
| `Operador -> Fleet platform` | `Consulta dashboard y chat con IA` | `HTTPS` `POST /api/chat`, `GET /api/*` | sec. 4.C, ADR-0003 cond. 9 (BFF `cmd/api`) |
| `Fleet platform -> Operador` | `Alertas y datos en tiempo real` | `SSE /api/alerts` (push) | sec. 4.C |
| `Fleet platform -> Gemini API` | `Consultas NL` | `HTTPS` via Genkit flows + gobreaker + timeout 15s | ADR-0003 cond. 2,5 |

## Qué NO va en Nivel 1 (y por qué)

*   **Proveedor de mapas (OSM/Mapbox):** Va directo `web SPA -> Map Provider` en Nivel 2 (contenedores). Backend nunca proxea tiles — violaría ADR-0003 cond. 9 (aislamiento front↔agente). Añadirlo aquí ensucia el contexto de negocio.
*   **AWS / Terraform / ALB / NATS / TimescaleDB:** Son contenedores/infra interna. Aparecen en Nivel 2 + deployment view. `AGENTS.md` fija Terraform como entregable pero no es interacción runtime sistema→sistema en Nivel 1.
*   **Admin / DevOps:** Persona operativa, no usuario del portal. Aparece solo en vista de despliegue si hace falta.

## Validación

*   `docker compose config` válido (Structurizr con `profiles: ["docs"]` no levanta en `compose up` normal, no afecta 16GB).
*   `workspace.dsl` parsea en Structurizr Lite (theme default, autoLayout lr).
*   Trazable a `PRUEBA-TECNICA.md` y ADRs — sin dependencias nuevas.

## Siguiente

Nivel 2 — Contenedores: descomponer `Fleet monitoring platform` en `lb (nginx/ALB) -> cmd/ingest -> NATS JetStream (TELEMETRY/ALERTS) -> cmd/consumer -> TimescaleDB`, `cmd/api (SSE)`, `cmd/agent (Genkit)`, `web (Vite)`, `mobile (Expo + SQLite)`.
