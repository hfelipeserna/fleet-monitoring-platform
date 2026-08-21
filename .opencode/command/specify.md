---
description: Crea un spec (docs/specs/SPEC-XXX) desde una descripción en lenguaje natural, siguiendo la skill spec-authoring. Uso: /specify <descripción de la feature>.
agent: architect
---

Crea el spec para la feature: "$ARGUMENTS" siguiendo la skill `spec-authoring`.

1. **Asigna el SPEC-ID**: siguiente correlativo en `docs/specs/` y crea `docs/specs/SPEC-XXX-<slug>/spec.md` a partir de `_template.md`.
2. **Redacta historias de usuario** desde la descripción; pregunta lo que falte, no inventes requisitos.
3. **Escribe criterios de aceptación** en Gherkin con ID (`AC-SPEC-XXX-N`): camino feliz + bordes + fallo esperado (timeout, duplicado, payload inválido). Cada AC debe ser verificable por test automatizado; los que no lo sean, refínelos o márcalos como pregunta abierta.
4. **Delimita fuera-de-alcance** explícitamente.
5. **Borra contratos**: si la feature expone HTTP/SSE o publica eventos NATS, crea el esqueleto OpenAPI/AsyncAPI en `contracts/`.
6. **Estado final `draft`** con preguntas abiertas listadas; NO lo apruebes tú solo — la aprobación es del usuario.
7. Devuélveme: ruta del spec, resumen de ACs, preguntas abiertas y qué dictámenes (`security`, `scalability`) anticipas para `/plan`.
