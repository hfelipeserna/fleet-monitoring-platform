---
description: Convierte un spec aprobado en plan técnico (plan.md) validando contra ADRs, skills de dominio y auditorías previas. Uso: /plan <SPEC-ID>.
agent: architect
---

Elabora el plan técnico para "$ARGUMENTS" (debe existir `docs/specs/<SPEC-ID>/spec.md`).

1. **Verifica prerrequisitos**: el spec existe y está `approved` (sin preguntas abiertas). Si no, detente y lista lo pendiente.
2. **Diseña la solución**: capas (domain → application → adapters → infra), componentes afectados por servicio (backend/web/móvil/infra), y cómo se respetan las interfaces del lado del consumidor.
3. **Consulta las skills de dominio** aplicables (`nats-jetstream`, `timescaledb`, `genkit-agent`, `realtime-dashboard`, `offline-first-mobile`, `iac-and-cicd`) y cita las restricciones que el plan hereda.
4. **Evalúa riesgo**: si el diseño candidatea a ADR o toca throughput crítico, marca explícitamente que hay que lanzar `scalability`; si toca auth/PII/superficie expuesta, `security`. No cierres el plan sin esos dictámenes.
5. **Escribe `docs/specs/<SPEC-ID>/plan.md`**: componentes, decisiones tomadas (con link a ADR si aplica), riesgos y orden de construcción.
6. Devuélveme: resumen del plan, dictámenes requeridos antes de implementar y riesgos principales.
