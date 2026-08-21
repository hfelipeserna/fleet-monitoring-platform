# Plantilla de especificación (SPEC)

> Copiar este archivo a `docs/specs/SPEC-XXX-<slug>/spec.md`. No editar la plantilla.

## Meta

- **SPEC-ID**: SPEC-
- **Título**:
- **Estado**: draft | approved | implemented
- **Backlog**: <tarea/épica relacionada>

## Historias de usuario

- Como \<rol\>, quiero \<capacidad\>, para \<beneficio\>.

## Criterios de aceptación

Formato Gherkin. Un escenario = un test automatizable. ID: `AC-SPEC-XXX-N`.

```gherkin
AC-SPEC-XXX-1:
  Given <estado inicial>
  When <acción>
  Then <resultado observable y medible>

AC-SPEC-XXX-2 (borde):
  Given ...
  When ...
  Then ...

AC-SPEC-XXX-3 (fallo esperado):
  Given ...
  When ...
  Then ...
```

## Contratos

- HTTP/SSE: `contracts/http.openapi.yaml` (si aplica)
- Eventos NATS: `contracts/events.asyncapi.yaml` (subject, payload, headers, dedup con `Nats-Msg-Id`, reintentos)

## Fuera de alcance

- <lo que explícitamente NO hace esta feature>

## Preguntas abiertas (bloquean `approved`)

- [ ] <ambigüedad a resolver antes de implementar>
