---
description: Genera un módulo/feature nuevo siguiendo clean architecture y el stack fijado. Uso: /scaffold <dominio> <feature>.
agent: architect
---

Implementa la feature o módulo "$ARGUMENTS" del backlog de la plataforma.

1. **Planifica**: descompón en tareas (domain → application → adapters → infra) y decide qué especialista ejecuta cada pieza (go-backend, data-events, ai-agent, react-web, mobile-expo, devops).
2. **Delega**: lanza los subagentes con `task` con contexto mínimo (stack fijado de AGENTS.md, skills a usar).
3. **Audita**: revisa cada entrega contra clean architecture (skill `clean-architecture`), la skill del dominio correspondiente, y pasa por el `reviewer` si hay dudas.
4. **Verifica**: `go build ./...` + `go vet ./...` (Go), `npm run build`/`lint` (web/mobile), `docker compose config` (infra).
5. **Documenta**: si hay una decisión nueva → ADR en `docs/adr/`; hallazgos del proceso → `docs/IAUDIT.md` (skill `ai-audit`).
6. **Devuélveme**: estructura creada, decisiones tomadas, resultado de verificación y un mensaje Conventional Commit sugerido (feat:/fix:/refactor:/test:).