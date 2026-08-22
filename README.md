# fleet-monitoring-platform
Event-driven fleet monitoring platform with agentic AI telemetry analysis, reactive web portal, and offline-first mobile app.

## Arquitectura — C4 Nivel 1 (Contexto)

![C4 Nivel 1 — Contexto_N1](docs/c4/Contexto_N1-thumbnail.png)

Fuente: `docs/c4/workspace.dsl` (Structurizr DSL). Ver detalle en [docs/c4/01-context.md](docs/c4/01-context.md). Decisiones: ADR-0001 NATS JetStream, ADR-0003 Genkit + Gemini.

```bash
# validar DSL
docker run --rm -v "$(pwd)/docs/c4:/work" structurizr/structurizr validate -workspace /work/workspace.dsl
# ver interactivo
docker compose --profile docs up structurizr  # http://localhost:8080
```
