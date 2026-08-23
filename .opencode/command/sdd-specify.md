---
description: Genera o actualiza spec.md (WHAT) de forma idempotente. Uso: /sdd-specify <descripción de la feature> | /sdd-specify SPEC-XXX <cambios>.
agent: architect
---

Ejecuta el workflow `sdd specify` (`workflows/create-spec.md` de la skill `sdd`) con el argumento: "$ARGUMENTS".

1. Si `$ARGUMENTS` empieza con `SPEC-XXX`, es actualización idempotente de `docs/specs/SPEC-XXX-<slug>/spec.md` existente; si no, es creación con siguiente correlativo.
2. Carga `docs/PRUEBA-TECNICA.md`, `docs/adr/*` y `templates/spec.md` como base. No inventes requisitos — toda ambigüedad va a `Open Questions` y no apruebes el spec tú solo (estado `draft`).
3. Redacta las 17 secciones obligatorias (Overview -> Assumptions) con IDs `UC-001`, `FR-001`, `BR-001`, `AC-001` (Given/When/Then), `TS-001` y trazabilidad `UC->FR->AC->TS`. Aplica las reglas condicionales de resiliencia de `SKILL.md` (idempotencia, retries, timeouts, circuit breakers, etc. solo cuando el dominio lo justifique).
4. Si la feature expone HTTP/SSE o eventos NATS, crea esqueletos `contracts/http.openapi.yaml` y `contracts/events.asyncapi.yaml`.
5. Valida los 9 checks de `create-spec.md` antes de entregar. Si falla, corrige.
6. Devuélveme: ruta del spec, resumen UC/FR/BR/AC/TS, preguntas abiertas y dictámenes anticipados (`security`/`scalability`) para `/sdd-plan`.
