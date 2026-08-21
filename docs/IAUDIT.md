# IAUDIT — Auditoría de IA (exoesqueleto, no muleta)

Registro de auditorías del código generado por agentes IA. Requisito del entregable:
documentar **al menos 2 decisiones donde el enfoque sugerido por la IA fue
deficiente/inseguro/no escalable** y cómo se forzó el estándar.

## Formato de entrada

```
## <fecha> — Auditoría: <scope> [SPEC-XXX]
Severidad: alta | media | baja
Hallazgo: <qué sugirió/hizo la IA>
Evidencia: archivo:línea (estado previo al refactor, ver git)
Por qué falla: <explicación técnica / estándar internacional aplicable>
Refactor exigido: <cómo se resolvió>
Auditor: reviewer | security | scalability | db-auditor | quality-auditor | architect
```

## Entradas

_(aún ninguna desde el reinicio del repositorio. Las entradas de la iteración
anterior —incluido el cierre del bypass del secret-guard del plugin— quedaron
archivadas en la rama `backup/pre-reset-harness`; se re-generarán sobre el código
nuevo conforme avance cada SPEC.)_

## Convenciones

- Severidad alta = task NO cerrado hasta refactor + re-auditoría.
- Cada entrada cita evidencia en git (commit/SHA previo) para que el evaluador
  pueda ver el "antes y después".
