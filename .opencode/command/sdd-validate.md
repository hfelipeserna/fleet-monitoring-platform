---
description: Valida coherencia spec.md vs plan.md vs tests (solo lectura, idempotente). Uso: /sdd-validate [SPEC-ID | --all].
agent: architect
---

Ejecuta `sdd validate` (skill `sdd` + `scripts/validate.mjs`) con el argumento: "$ARGUMENTS".

1. Si `$ARGUMENTS` es un `SPEC-ID` (ej. `SPEC-001`), valida solo `docs/specs/SPEC-001-*/spec.md` y `plan.md`. Si es `--all`, valida todos los specs. Sin argumento, valida el SPEC-XXX más reciente.
2. Corre `node .opencode/skills/sdd/scripts/validate.mjs $ARGUMENTS` y captura la salida. El script es idempotente y de solo lectura (no modifica archivos).
3. Detecta y reporta con severidad:
   - requisito sin implementación (FR sin Technical Change)
   - caso de uso sin AC (UC sin AC)
   - AC sin test (AC sin TS/TEST)
   - test que introduce comportamiento no especificado (TEST sin TS padre -> SPEC GAP)
   - contradicciones spec vs plan
   - tareas sin justificación (Step sin Spec References)
   - SPEC GAP explícito
   - diagramas con implementación en spec (class/repository/adapter en Mermaid de spec.md)
   - cambios de arquitectura no contemplados (plan menciona componente/ADR no trazado)
4. Valida también estructura: 17 secciones en spec, 16 en plan, matriz de trazabilidad, y formato de IDs.
5. Si hay hallazgos de severidad `alta`, el validador sale con exit 1 (CI fail). No intentes autocorregir — reporta y propone fixes como `Open Questions` o `SPEC GAP` según corresponda.
6. Devuélveme: reporte por SPEC-ID con conteos `UC/FR/BR/AC/TS/TEST`, lista de hallazgos con severidad y archivo:línea, y veredicto `PASS/FAIL` con próximos pasos.
