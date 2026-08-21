---
description: Descompone el plan de un spec en tareas delegables con trazabilidad a los criterios de aceptación. Uso: /tasks <SPEC-ID>.
agent: architect
---

Descompón el plan de "$ARGUMENTS" (`docs/specs/<SPEC-ID>/plan.md`) en tareas ejecutables.

1. **Genera la lista de tareas**: cada tarea indica especialista asignado (`go-backend`, `data-events`, `ai-agent`, `react-web`, `mobile-expo`, `devops`, `test-engineer`), archivos/componentes que toca y los ACs que cubre con formato `[SPEC-XXX: AC-N]`. Una tarea debe ser verificable de forma aislada.
2. **Ordena por dependencias**: dominio antes que adapters; contratos antes que consumidores; cada tarea de feature lleva su tarea paralela de `test-engineer` (AAA + suites) que cubra los ACs asignados.
3. **Define gates de cierre por tarea**: compila/vet/lint, tests verdes citando sus ACs, `reviewer` sin hallazgos altos; más `db-auditor` si toca queries/SQL, `quality-auditor` en refactors de lógica caliente, `security`/`scalability` según el plan.
4. **NO implementes**: solo produce el desglose.
5. Devuélveme: tabla de tareas (ID, especialista, scope, ACs cubiertos, gates), camino crítico y qué tareas puedo lanzar ya en paralelo.
