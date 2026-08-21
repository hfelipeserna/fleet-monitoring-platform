---
name: spec-authoring
description: Usa esta skill al escribir, planificar o descomponer features con Spec-Driven Development (SDD): specs en docs/specs/, criterios de aceptación verificables, contratos OpenAPI/AsyncAPI y trazabilidad spec → task → commit → test. Trigger: spec, especificación, SDD, specify, criterios de aceptación, Gherkin, Given/When/Then, OpenAPI, AsyncAPI, contrato, SPEC-ID, trazabilidad.
---

# Spec-Driven Development (SDD)

Principio: **el spec es la fuente de verdad; el código es una derivación del spec**. Ningún especialista implementa sin un spec aprobado, y ningún task se cierra si sus criterios de aceptación no tienen test que los cubra.

## Estructura de specs

```
docs/specs/
├── _template.md            plantilla canónica (copiar, nunca editar)
└── SPEC-XXX-<slug>/
    ├── spec.md             requerimientos + criterios de aceptación
    ├── plan.md             diseño técnico y decisiones (opcional si trivial)
    └── contracts/          OpenAPI / AsyncAPI cuando aplique
        ├── http.openapi.yaml
        └── events.asyncapi.yaml
```

## Formato de spec.md

1. **Meta**: `SPEC-ID` (correlativo), estado (`draft | approved | implemented`), feature padre del backlog.
2. **Historias de usuario**: `Como <rol>, quiero <capacidad>, para <beneficio>`.
3. **Criterios de aceptación**: en Gherkin (`Given/When/Then`), uno por escenario. Reglas:
   - Cada criterio debe ser **verificable por un test automatizado**. Si no lo es, es ambiguo: refinar antes de codificar.
   - Cubrir el camino feliz + casos de borde + fallo esperado (timeout, duplicado, inválido).
4. **Fuera de alcance**: explícito, para frenar el scope creep de los especialistas.
5. **Contratos**: referenciar schemas OpenAPI/AsyncAPI. Los agentes generan clientes/productores desde aquí, nunca desde suposiciones.
6. **No soluciones**: el spec dice *qué* y *para qué*; el *cómo* vive en `plan.md`.

## Trazabilidad

- Cada criterio de aceptación lleva ID: `AC-SPEC-001-1`, `-2`, ...
- Las tareas del plan citan los ACs que cubren: `[SPEC-001: AC-2, AC-3]`.
- Los commits citan el SPEC-ID: `feat(telemetry): batch ingest [SPEC-001]`.
- Los tests nombran el AC en el comentario de suite o nombre de test.
- Cierre del spec = todos los ACs tienen test verde + reviewer/db-auditor/quality-auditor sin hallazgos altos.

## Contratos machine-readable

- **HTTP/SSE** → OpenAPI 3.1 en `contracts/http.openapi.yaml`. Validar con lint en CI.
- **Eventos NATS** → AsyncAPI 3 en `contracts/events.asyncapi.yaml`: subject, payload schema, headers (`Nats-Msg-Id` para dedup), política de reintentos.
- Cambiar un contrato exige actualizar el spec primero (PR del spec antes del PR de código).

## Antipatrones a rechazar

- Spec que describe implementación ("usaremos un mapa concurrente") en vez de comportamiento.
- Criterios de aceptación no medibles ("la UI debe ser rápida").
- Specs sin fuera-de-alcance (los especialistas inventan features).
- Código commiteado con SPEC-ID inexistente o spec en `draft`.
