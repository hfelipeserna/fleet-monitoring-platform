---
description: Ejecuta una auditoría del código generado con el agente reviewer y registra los hallazgos en docs/IAUDIT.md.
agent: architect
---

Ejecuta una auditoría formal del código del proyecto (o del scope $ARGUMENTS si se indica, ej: "backend/internal/ingestor").

1. Lanza el subagente `reviewer` sobre el scope indicado (clean architecture, seguridad, resiliencia, correctitud de dominio). Si hay código/lógica de seguridad (handlers, auth, gateways, móvil, IaC), lanza además al especialista `security`. Si el scope candidatea a ADR o toca un componente crítico de throughput, lanza al especialista `scalability`.
2. Recolecta los reportes. Si hay hallazgos de severidad alta, NO los marques como resueltos: refactoriza con el especialista correspondiente y re-audita.
3. Volca el reporte en `docs/IAUDIT.md` siguiendo la skill `ai-audit` (estructura severidad/hallazgo/evidencia/por-qué-falla/decisión).
4. Devuélveme un resumen: hallazgos por severidad, qué se refactorizó, y la entrada(es) de IAUDIT añadidas.
5. Recuerda: necesitamos documentar al menos 2 ejemplos donde el enfoque sugerido por la IA era deficiente/inseguro/no escalable para el README.