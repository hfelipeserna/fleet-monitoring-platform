---
name: ai-audit
description: Usa esta skill al documentar auditorías de IA y decisiones en docs/IAUDIT.md y docs/adr/. Cubre cómo registrar refactors forzados por el arquitecto (requisito del entregable), formato de hallazgos y el ADR de NATS vs Kafka. Trigger: auditoría, IAUDIT, audit, ADR, decision record, documentar, refactor forzado, entregable, clean architecture hallazgos.
---

# Auditoría de IA y registro de decisiones

El entregable pide **evidencia de que la IA es el exoesqueleto, no la muleta**: al menos 2 decisiones donde el enfoque sugerido por la IA fue deficiente/inseguro/no escalable y forzaste el estándar. Este repo documenta eso en `docs/IAUDIT.md` y las decisiones firmes en `docs/adr/`.

## docs/IAUDIT.md (auditoría continua)

- **¿Cuándo?** Cada vez que el `architect`/`reviewer` rechaza o refactoriza código generado, o un especialista sugiere algo contra el estándar (hallazgo severidad >= media).
- **¿Cómo?** Apéndice cronológico. Cada entrada con la estructura del reviewer:

```md
## [YYYY-MM-DD] <scope o feature>
- Severidad: alta | media | baja
- Hallazgo: <qué propuso la IA y por qué estaba mal>
- Evidencia: backend/internal/.../file.go:42
- Por qué falla: <clean architecture / seguridad / escalabilidad / resiliencia>
- Decisión: <qué forzaste> + archivo de referencia del refactor
```

- Mantené al menos 2 entradas de severidad alta con enfoque deficiente sugerido (requisito explícito del README). Ejemplos típicos: import de `domain` → `infra`, `errors.New` sin wrap, secretos hardcodeados, sync móvil sin `client_event_id` (colisión en dedup), SSE sin reconexión.

## ADRs en docs/adr/

- Formato `NNNN-titulo.md` (ADR = Architecture Decision Record). Corto y con esta estructura:

```md
# ADR-0001: NATS JetStream como bus de eventos

## Estado
Aceptado

## Contexto
Tensión entre requerimiento ("bus de eventos tipo Kafka/RabbitMQ"), alta concurrencia y máquina de dev con 16 GB RAM.

## Decisión
Usar NATS JetStream (streams durables + replay + dedup) sobre Kafka/RabbitMQ.

## Consecuencias
- + Un solo binario liviano, imagen ~15 MB vs JVM/Zookeeper (Kafka) o Erlang VM (RabbitMQ).
- + Mismo modelo de streams durables, replays y durable consumers.
- - Ecosistema/ops menos conocido por algunos equipos; documentado en README.
```

- Cualquier "NO revertir sin ADR" de AGENTS.md debe tener su ADR.

## Relación con el README

- El README debe linkear a `docs/IAUDIT.md` y `docs/adr/`. La sección "auditoría de IA" del README resume los 2+ hallazgos y cómo se forzó el estándar (con links a las entradas).