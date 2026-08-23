# Workflow: sdd specify — create-spec

> Genera o actualiza `spec.md` (WHAT). Idempotente. Invocado por `/sdd-specify`.

## Rol

Actúa como un **Senior Software Engineer especializado en Spec-Driven Development (SDD), sistemas distribuidos, backend y diseño de APIs**.

A partir de la información que te proporcionaré, genera un archivo `spec.md`.

El propósito del `spec.md` es definir de forma precisa **QUÉ** debe hacer el sistema y cuál debe ser su comportamiento observable.

## Principio fundamental

El `spec.md` define:

* WHAT debe hacer el sistema.
* WHY existe el cambio.
* Qué comportamiento esperan los usuarios y sistemas involucrados.
* Qué reglas y restricciones deben cumplirse.
* Cómo se puede verificar funcionalmente el comportamiento.

NO debe definir prematuramente HOW se implementará.

Evita, salvo que sean restricciones explícitas:

* clases
* métodos
* packages
* interfaces internas
* nombres de archivos
* estructuras internas
* frameworks
* librerías
* detalles de implementación

No inventes información. Si existe una ambigüedad, inclúyela en `Open Questions`.

## Pasos del workflow (idempotente)

1. **Resolver SPEC-ID**: si es creación, siguiente correlativo en `docs/specs/` (`SPEC-001`, `SPEC-002`...). Si es actualización, reutiliza el `SPEC-ID` existente. Crea `docs/specs/SPEC-XXX-<slug>/spec.md` a partir de `templates/spec.md`.
2. **Leer contexto**: `docs/PRUEBA-TECNICA.md`, `docs/adr/*`, `spec.md` existente si hay.
3. **Redactar `spec.md` siguiendo la Estructura obligatoria** (17 secciones, ver abajo). Cada `UC/FR/BR/AC/TS` con ID correlativo. Mantén trazabilidad `UC -> FR -> AC -> TS`.
4. **Aplicar reglas condicionales de resiliencia** de `SKILL.md`: solo incluye `idempotencia, retries, timeouts, circuit breakers, consistency, event ordering, at-least-once, backward compatibility, migrations, observability, failure scenarios, concurrency` cuando sean relevantes al dominio del spec. Si no aplica, no lo menciones.
5. **Borrar contratos esqueleto**: si la feature expone HTTP/SSE o eventos NATS, crea `contracts/http.openapi.yaml` (OpenAPI 3.1) y/o `contracts/events.asyncapi.yaml` (AsyncAPI 3 con `Nats-Msg-Id` para dedup) vacíos pero válidos.
6. **Estado `draft`** con `Open Questions` listadas; NO lo apruebes tú solo — la aprobación es del usuario.
7. **Validar antes de entregar** (9 checks). Si falla, corrige.
8. **Devolver**: ruta del spec, resumen de `UC/FR/AC/TS`, preguntas abiertas y qué dictámenes (`security`, `scalability`) anticipas para `sdd plan`.

## Estructura obligatoria de spec.md (17 secciones)

### 1. Overview
Problema, contexto, objetivo, motivación del cambio.

### 2. Scope
#### In Scope
Qué comportamiento forma parte de esta especificación.
#### Out of Scope
Qué explícitamente no forma parte.

### 3. Actors and Systems
Usuarios, servicios, sistemas internos/externos, dependencias relevantes.

### 4. Use Cases
Define los casos de uso como `UC-001`, `UC-002`, etc.
Cada caso de uso debe contener:
* ID
* nombre
* actor
* objetivo
* preconditions
* trigger
* main flow
* alternative flows
* error flows
* postconditions
* business rules involucradas
No describas cómo se implementará.

### 5. Functional Requirements
Enumera como `FR-001`, `FR-002`, etc. Cada requisito debe ser específico, observable, verificable, independiente cuando sea posible. Relaciona con UC correspondientes.

### 6. Business Rules
Enumera como `BR-001`, `BR-002`, etc. Cada regla describe condición o comportamiento de negocio verificable.

### 7. Main Flows
Flujos principales. Cuando sea complejo, incluye diagrama Mermaid `flowchart`.

### 8. Alternative and Error Flows
Validaciones, datos inválidos, errores de negocio/temporales, dependencias no disponibles, timeouts, duplicados, operaciones repetidas, estados inválidos, escenarios límite. Describe QUÉ debe suceder, no cómo.

### 9. State and Transitions
Si existe máquina de estados: estados, eventos, transiciones, condiciones, estados finales, transiciones inválidas.

### 10. API / Interface Contracts
Cuando corresponda: endpoint, HTTP method, request, response, códigos HTTP, errores, validaciones, comportamiento esperado. Desde punto de vista observable.

### 11. Sequence Diagrams
Mermaid `sequenceDiagram` para interacciones entre actores y sistemas. Responde "¿Qué sistemas interactúan y en qué orden para cumplir el caso de uso?" Sin detalles internos.

### 12. Flow Diagrams
Mermaid `flowchart` para decisiones, procesos, estados, flujos de negocio complejos.

### 13. Non-Functional Requirements
Solo requisitos explícitos o necesarios: performance, availability, reliability, consistency, scalability, security, observability, backward compatibility. No inventes métricas ni SLOs.

### 14. Acceptance Criteria
Criterios verificables con `Given/When/Then`, ID `AC-001`, relacionados a UC/FR.
```
Given a valid payment
When the payment is confirmed
Then the payment status must become `CONFIRMED`.
```

### 15. Functional Test Scenarios
Derivados de UC y AC. NO son tests unitarios todavía. Cada escenario:
* ID `TS-001`
* caso de uso y requisito relacionado
* preconditions, input, action, expected result
Incluye happy paths, alternative paths, validation failures, business errors, dependency failures, edge cases, idempotency scenarios, retry scenarios cuando sean parte del comportamiento esperado.

### 16. Open Questions
Información faltante que pueda cambiar comportamiento esperado. Bloquea `approved`.

### 17. Assumptions
Solo supuestos necesarios y explícitamente marcados como tales.

## Reglas de trazabilidad

```
UC-001 -> FR-001 -> AC-001 -> TS-001
```

## Reglas para diagramas (spec.md)

Permitido: actores, servicios, sistemas externos, eventos, requests/responses, decisiones, estados.
Prohibido: clases, métodos, packages, interfaces internas, repositories, adapters, detalles concretos de implementación.

## Validación antes de entregar (9 checks)

1. Cada caso de uso tiene al menos un flujo principal.
2. Cada caso de uso contempla errores y alternativas relevantes.
3. Cada requisito funcional está relacionado con un caso de uso cuando corresponda.
4. Cada comportamiento importante tiene acceptance criteria.
5. Cada acceptance criterion tiene al menos un functional test scenario.
6. No existen functional test scenarios que introduzcan requisitos inexistentes.
7. Los diagramas representan comportamiento y no implementación.
8. No se han introducido decisiones técnicas prematuras.
9. Las ambigüedades están documentadas en Open Questions.

Entrega únicamente el contenido final de `spec.md`.
