---
description: Lee spec.md aprobado y genera o actualiza plan.md (HOW) de forma idempotente. Uso: /sdd-plan <SPEC-ID>.
agent: architect
---

Ejecuta el workflow `sdd plan` (`workflows/create-plan.md` de la skill `sdd`) con el argumento: "$ARGUMENTS" (debe ser un `SPEC-ID` como `SPEC-001`).

1. Verifica que existe `docs/specs/<SPEC-ID>/spec.md` y está en `approved` (sin `Open Questions` pendientes). Si no, detente y lista lo pendiente — no generes plan.
2. Lee `spec.md`, `docs/adr/*`, `docs/c4/*`, y el repo (`backend/*`, `docker-compose.yml`) para `Technical Context`. Consulta las skills de dominio aplicables (`nats-jetstream`, `timescaledb`, `genkit-agent`, `realtime-dashboard`, `offline-first-mobile`, `iac-and-cicd`, `clean-architecture`) y cita restricciones heredadas.
3. Escribe `docs/specs/<SPEC-ID>/plan.md` desde `templates/plan.md` (16 secciones). Incluye matriz `Spec ID -> Requirement/UC -> Technical Change -> Tests`, `Detailed Technical Design` por componente, y `Implementation Steps` atomizados con `Spec References`, `Tests` y `Dependencies`. Aplica reglas condicionales de resiliencia solo cuando el spec lo justifique.
4. Si detectas comportamiento necesario no definido en `spec.md`, no lo agregues silenciosamente — márcalo como `> SPEC GAP` y explica la decisión requerida.
5. Evalúa si el diseño requiere dictámenes `scalability` (throughput crítico) y/o `security` (auth/PII/superficie expuesta). No cierres el plan sin ellos cuando apliquen.
6. Valida los 10 consistency checks de `create-plan.md` antes de entregar.
7. Devuélveme: resumen del plan, matriz de trazabilidad, dictámenes requeridos y riesgos principales.
