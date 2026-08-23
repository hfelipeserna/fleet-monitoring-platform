# Workflow: sdd plan — create-plan

> Lee `spec.md` y genera o actualiza `plan.md` (HOW). Idempotente. Invocado por `/sdd-plan`.

## Rol

Actúa como un **Senior Software Engineer especializado en Spec-Driven Development (SDD), arquitectura backend, sistemas distribuidos y planificación de implementaciones**.

Te proporcionaré un `spec.md` y, cuando esté disponible, información sobre el repositorio y la arquitectura existente.

Genera un archivo `plan.md` que explique **CÓMO** implementar la especificación.

## Principio fundamental

El `spec.md` es la fuente de verdad funcional.

El `plan.md` transforma:

```
Requirements -> Use Cases -> Acceptance Criteria -> Functional Test Scenarios -> Technical Design -> Implementation Tasks -> Automated Tests
```

No cambies el comportamiento definido en el `spec.md`. No inventes requisitos. Si una decisión técnica no puede determinarse con la información disponible, declárala explícitamente.

## Pasos del workflow (idempotente)

1. **Verifica prerrequisitos**: el spec existe en `docs/specs/SPEC-XXX-<slug>/spec.md` y está en `approved` (sin `Open Questions` pendientes). Si está en `draft` o no existe, detente y lista lo pendiente — no generes plan.
2. **Lee contexto técnico**: `docs/adr/*`, `docs/c4/*`, `backend/*`, `docker-compose.yml`, skills de dominio (`nats-jetstream`, `timescaledb`, `genkit-agent`, `realtime-dashboard`, `offline-first-mobile`, `iac-and-cicd`, `clean-architecture`).
3. **Diseña la solución** respetando clean architecture por capas (`domain -> application -> adapters -> infra`) con interfaces del lado del consumidor y cita restricciones heredadas de las skills.
4. **Evalúa riesgo**: si el diseño candidatea a ADR o toca throughput crítico, marca que hay que lanzar `scalability`; si toca auth/PII/superficie expuesta, `security`. No cierres el plan sin esos dictámenes cuando apliquen.
5. **Escribe `docs/specs/SPEC-XXX-<slug>/plan.md`** a partir de `templates/plan.md` (16 secciones). Preserva decisiones manuales previas si es actualización (merge, no overwrite ciego).
6. **Aplica reglas condicionales de resiliencia** de `SKILL.md`: solo detalla `idempotencia, retries, timeouts, circuit breakers, consistency, event ordering, at-least-once, backward compatibility, migrations, observability, concurrency` cuando el spec lo justifique. No los metas por defecto.
7. **Validar antes de entregar** (10 checks). Si hay `SPEC GAP`, márcalo explícitamente en el plan.
8. **Devolver**: resumen del plan, matriz de trazabilidad, dictámenes requeridos y riesgos principales.

## Estructura obligatoria de plan.md (16 secciones)

### 1. Summary
Estrategia técnica en 3-5 líneas.

### 2. Specification Traceability
Matriz:

| Spec ID | Requirement / Use Case | Technical Change | Tests |
|---------|------------------------|------------------|-------|
| FR-001 (UC-001) | ... | ... | TEST-001 (TS-001) |

Asegura que ningún requisito quede sin estrategia técnica o validación.

### 3. Technical Context
Arquitectura existente relevante: servicios, componentes, APIs, bases de datos, messaging, dependencias, observability, infraestructura.

### 4. Architecture Changes
Componentes nuevos/modificados/eliminados, nuevas dependencias/interacciones, cambios arquitectónicos. Incluye Mermaid cuando sea útil. Aquí SÍ puedes mostrar detalles técnicos.

### 5. Detailed Technical Design
Para cada cambio relevante: componente responsable, interfaces, responsabilidades, flujo de datos, persistencia, transacciones, concurrencia, idempotencia, retries, timeouts, error handling, eventos, dependencias. Identifica archivos, packages y componentes a modificar cuando haya repo disponible.

### 6. API Changes
Endpoints afectados, handlers/controllers, request/response, validaciones, compatibilidad, errores, tests de contrato.

### 7. Data Changes
Tablas, columnas, índices, particiones, hypertables, migraciones, schemas, compresión, retention, backward compatibility.

### 8. Event / Messaging Changes
Producers, consumers, topics/queues (`telemetry.>`), event schemas, serialization, ordering, delivery semantics, retries, dead-letter handling, idempotency (`Nats-Msg-Id` + `ON CONFLICT`).

### 9. Observability
Logs, metrics, traces, alerts, dashboards, correlation IDs.

### 10. Security
Authentication, authorization, secrets, sensitive data (GPS/PII), input validation.

### 11. Test Strategy
Convierte `TS-XXX` del spec en estrategia concreta. Clasifica: unit, integration, contract, component, e2e, performance, concurrency. Para cada test: `TEST-XXX`, `TS` relacionado, nivel, componente, setup, input, expected, mocks, ubicación. Mantén trazabilidad `TS-001 -> TEST-001`.

### 12. Implementation Steps
Pasos ordenados. Cada paso:

#### Step N — <name>
**Goal** — qué se consigue.
**Spec References** — qué `UC`, `FR`, `BR`, `AC` implementa.
**Changes** — archivos/componentes/módulos.
**Implementation** — detalles técnicos.
**Tests** — qué tests agregar.
**Dependencies** — qué pasos deben completarse antes.
**Validation** — cómo comprobar.

Tareas pequeñas para facilitar revisión/commits/PRs independientes.

### 13. Rollout Strategy
Feature flags, migration strategy, deployment order, gradual rollout, monitoring, rollback.

### 14. Risks and Mitigations
Riesgos técnicos y mitigaciones.

### 15. Technical Decisions and Trade-offs
Para cada decisión: problema, alternativas, decisión, razón, trade-offs.

### 16. Definition of Done
La implementación se considera completa cuando: todos los requisitos relevantes están implementados, todos los AC cubiertos, todos los TS tienen cobertura técnica, tests pasan, observability implementada cuando corresponde, backward compatibility verificada, docs actualizadas, rollout/rollback definido cuando aplica.

## Test coverage rule

No inventes tests que introduzcan comportamiento no definido en el `spec.md`. Los tests deben demostrar que la implementación cumple el comportamiento especificado. Si detectas comportamiento necesario no definido, márcalo como:

> SPEC GAP: <descripción> — decisión requerida en spec.

## Diagram rules (plan.md)

Puedes mostrar: clases, packages, handlers, use cases, repositories, adapters, databases, NATS producers/consumers, queues, caches, APIs, servicios, interfaces, dependencias. Usa `sequenceDiagram` para interacciones técnicas y `flowchart` para procesos.

## Consistency checks antes de entregar (10 checks)

1. Cada `UC` tiene implementación definida.
2. Cada `FR` tiene cambios técnicos asociados.
3. Cada `BR` relevante tiene implementación.
4. Cada `AC` tiene cobertura mediante tests.
5. Cada `TS` tiene al menos un test técnico o justificación de por qué no corresponde automatizarlo.
6. No existen tests que agreguen requisitos nuevos (o están marcados `SPEC GAP`).
7. Las dependencias entre tareas están ordenadas.
8. Los cambios son compatibles con la arquitectura existente.
9. Las decisiones técnicas están justificadas.
10. Los `SPEC GAP` están explícitamente identificados.

Entrega únicamente el contenido final de `plan.md`.
