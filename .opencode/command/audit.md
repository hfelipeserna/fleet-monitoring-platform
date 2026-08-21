---
description: Ejecuta una auditoría del código generado con el agente reviewer y registra los hallazgos en docs/IAUDIT.md.
agent: architect
---

Ejecuta una auditoría formal del código del proyecto (o del scope $ARGUMENTS si se indica, ej: "backend/internal/ingestor").

1. Lanza el subagente `reviewer` sobre el scope indicado (clean architecture, seguridad, resiliencia, correctitud de dominio). Si hay código/lógica de seguridad (handlers, auth, gateways, móvil, IaC), lanza además al especialista `security`. Si el scope candidatea a ADR o toca un componente crítico de throughput, lanza al especialista `scalability`.
2. Lanza en paralelo los auditores complementarios según aplique: `db-auditor` si el scope toca queries/schemas/migraciones (paginación keyset vs offset, índices, inyección SQL, batch inserts); `quality-auditor` si es refactor o lógica caliente (O-notation, SOLID, patrones, clean code).
3. Recolecta los reportes. Si hay hallazgos de severidad alta, NO los marques como resueltos: refactoriza con el especialista correspondiente y re-audita.
4. Volca el reporte en `docs/IAUDIT.md` siguiendo la skill `ai-audit` (estructura severidad/hallazgo/evidencia/por-qué-falla/decisión).
5. Devuélveme un resumen: hallazgos por severidad y auditor, qué se refactorizó, y la entrada(es) de IAUDIT añadidas.
6. Recuerda: necesitamos documentar al menos 2 ejemplos donde el enfoque sugerido por la IA era deficiente/inseguro/no escalable para el README.