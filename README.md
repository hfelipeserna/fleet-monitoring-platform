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
