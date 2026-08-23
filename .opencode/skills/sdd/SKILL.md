---
name: sdd
description: Skill orquestadora de Spec-Driven Development (SDD) para la Fleet Monitoring Platform. Genera y valida spec.md (WHAT/WHY) y plan.md (HOW) con trazabilidad UC->FR->BR->AC->TS, detección de gaps y reglas condicionales de resiliencia. Trigger: sdd, spec, plan, validate, SPEC-ID, trazabilidad, Gherkin, UC-, FR-, BR-, AC-, TS-, SPEC GAP, idempotencia, retries, circuit breaker.
---

# SDD — Spec-Driven Development

## Principio fundamental

**El `spec.md` es la fuente de verdad funcional; el `plan.md` es una derivación técnica; `validate` es el guardián de coherencia.**

* `spec.md` define **QUÉ** hace el sistema y **POR QUÉ**, su comportamiento observable y cómo verificarlo. Nunca define prematuramente **CÓMO** se implementará.
* `plan.md` transforma `Requirements -> Use Cases -> Acceptance Criteria -> Functional Test Scenarios -> Technical Design -> Implementation Steps -> Automated Tests` sin inventar requisitos.
* `validate` detecta incoherencias sin ejecutar código. Es más valioso que prompts separados porque evita que código y spec diverjan silenciosamente.

Ningún especialista implementa sin `spec.md` en `approved`; ningún task cierra si sus `AC` no tienen test que los cubra.

## Estructura en disco

```
docs/specs/
├── _template.md                    # legacy, ahora delega en sdd/templates/spec.md
└── SPEC-XXX-<slug>/
    ├── spec.md                     # 17 secciones (WHAT)
    ├── plan.md                     # 16 secciones (HOW)
    └── contracts/                  # OpenAPI 3.1 / AsyncAPI 3 cuando aplique
        ├── http.openapi.yaml
        └── events.asyncapi.yaml

.opencode/skills/sdd/
├── SKILL.md                        # este archivo
├── workflows/
│   ├── create-spec.md              # workflow sdd specify
│   └── create-plan.md              # workflow sdd plan
├── templates/
│   ├── spec.md                     # plantilla canónica spec (17 secciones)
│   └── plan.md                     # plantilla canónica plan (16 secciones)
└── scripts/
    └── validate.mjs                # linter sdd validate
```

Comandos (harness `opencode`):

* `sdd specify` -> `/sdd-specify` (`spec.md` idempotente: genera o actualiza)
* `sdd plan`    -> `/sdd-plan`    (`plan.md` idempotente: lee `spec.md` y genera o actualiza)
* `sdd validate`-> `/sdd-validate` (audita `spec.md` vs `plan.md` vs tests)

## Workflows

| Comando | Workflow | Entrada | Salida | Idempotente |
|---------|----------|---------|--------|-------------|
| `sdd specify` | `workflows/create-spec.md` | descripción NL + `docs/PRUEBA-TECNICA.md` + `spec.md` existente | `docs/specs/SPEC-XXX-<slug>/spec.md` en `draft` | Sí: re-ejecuta y mergea sin duplicar IDs |
| `sdd plan` | `workflows/create-plan.md` | `spec.md` en `approved` + repo/arch existente | `docs/specs/SPEC-XXX-<slug>/plan.md` | Sí: regenera secciones preservando decisiones manuales marcadas |
| `sdd validate` | `scripts/validate.mjs` | `spec.md` + `plan.md` + tests | reporte `SPEC GAP` + exit code | Sí: solo lectura |

## Reglas condicionales de resiliencia (no inventar requisitos)

Los siguientes conceptos **solo se incluyen cuando son relevantes al dominio del spec**. La skill debe evaluar relevancia antes de introducirlos como `FR/BR/AC`:

| Concepto | Incluir solo si... | Ejemplo en Fleet Platform |
|----------|--------------------|---------------------------|
| `idempotencia`, `at-least-once`, `dedup Nats-Msg-Id` | UC toca ingesta/eventos/webhooks/operaciones repetibles | `telemetry.>` , `POST /ingest`, sync móvil offline |
| `retries`, `timeouts`, `circuit breakers (gobreaker)` | Actor incluye sistema externo, NATS, TimescaleDB, Gemini/Vertex | publicador NATS, writer pgx, llamada LLM |
| `event ordering` | Hay ordering semántico por `device_id` o `occurred_at` | posiciones por vehículo |
| `consistency` | Hay estado transaccional o lectura tras escritura | crear hypertable + índice, aggregate |
| `backward compatibility`, `migrations` | Se modifican contratos/APIs/schemas existentes | nuevo campo `event_id`, cambio OpenAPI |
| `observability` (logs/metrics/traces/correlationId) | Path crítico, alta frecuencia o requisito explícito | ingestor, SSE, agente Genkit |
| `concurrency`, `backpressure` | Throughput > ~100 msg/s o consumers concurrentes | 1000+ devices x 5-15s |
| `failure scenarios` | Siempre, pero acotado a `Alternative and Error Flows` del dominio | validación, duplicado, timeout, dependencia caída |

Si no aplica: **no lo menciones**. No inventes SLOs, latencias ni métricas sin fuente. Toda ambigüedad va a `Open Questions`.

## Trazabilidad obligatoria

```
UC-001 (Use Case)
  ↓
FR-001 / BR-001 (Requirement / Rule)
  ↓
AC-001 (Acceptance Criterion - Given/When/Then)
  ↓
TS-001 (Functional Test Scenario)
  ↓
TEST-001 (Test técnico en plan.md)
  ↓
Step N (Implementation Step)
```

* IDs correlativos por spec: `UC-001`, `FR-001`, `BR-001`, `AC-001`, `TS-001`, `TEST-001`.
* Commits citan `SPEC-ID`: `feat(telemetry): batch ingest [SPEC-001]`.
* Tests nombran el `AC/TS` que cubren en comentario o nombre.
* Cierre = todos los `AC` tienen test verde + `reviewer`/`db-auditor`/`quality-auditor` sin hallazgos altos.

## Reglas para diagramas

**En `spec.md` (funcional):** Permitido: actores, servicios, sistemas externos, eventos, requests/responses, decisiones, estados. Prohibido: clases, métodos, packages, interfaces internas, repositories, adapters.

**En `plan.md` (técnico):** Permitido todo lo anterior + clases, packages, handlers, use cases, repositories, adapters, DBs, topics/queues, caches, interfaces, dependencias.

`spec.md` usa `sequenceDiagram` para interacciones entre sistemas y `flowchart` para decisiones de negocio. `plan.md` puede detallar implementación.

## Validación (sdd validate) — detectores

| Detector | Severidad | Mensaje |
|----------|-----------|---------|
| requisito sin implementación | alta | `FR-XXX` sin `Technical Change` ni `Step` en `plan.md` |
| caso de uso sin acceptance criteria | alta | `UC-XXX` sin `AC-XXX` que lo cite |
| acceptance criterion sin test | alta | `AC-XXX` sin `TS-XXX` / sin `TEST-XXX` |
| test que introduce comportamiento no especificado | alta | `TEST-XXX`/`TS-XXX` sin `FR/BR/AC` padre -> marcar `SPEC GAP` |
| contradicciones entre spec y plan | media | `FR/BR` dice X pero `Architecture Changes` hace Y |
| tareas técnicas sin justificación | media | `Step N` sin `Spec References (UC/FR/BR/AC)` |
| SPEC GAP explícito | media | Bloque `> SPEC GAP` en plan sin resolución en spec |
| diagramas con implementación en spec | media | Mermaid en `spec.md` contiene `class|method|repository|adapter|interface` |
| cambios de arquitectura no contemplados | media | `plan.md` introduce componente/ADR sin `Technical Context` ni link ADR |

`validate` es solo lectura, idempotente y debe correr en CI.

## Relación con skills de dominio

`create-plan` debe consultar y citar restricciones de: `nats-jetstream`, `timescaledb`, `genkit-agent`, `realtime-dashboard`, `offline-first-mobile`, `iac-and-cicd`, `clean-architecture`, `scalability-review`, `security-review`. Si toca throughput crítico o auth/PII, exige dictamen `scalability`/`security` antes de cerrar el plan.

## Antipatrones a rechazar

* Spec que describe implementación ("usaremos un mapa concurrente") en vez de comportamiento.
* AC no medible ("la UI debe ser rápida") sin métrica en `Open Questions`.
* Spec sin `Out of Scope` (los especialistas inventan features).
* Código commiteado con `SPEC-ID` inexistente o `spec.md` en `draft`.
* Plan que inventa `FR` no trazable a `spec.md` (debe marcarse `SPEC GAP`).
