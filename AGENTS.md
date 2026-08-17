# AGENTS.md — Fleet Monitoring Platform

Instrucciones globales para TODOS los agentes de este repositorio. Léelas antes de cualquier tarea.

## Propósito

MVP funcional (prueba técnica) de un Portal Corporativo de monitoreo de flotas:

- Ingesta orientada a eventos de alta concurrencia (Go + **NATS JetStream**).
- Persistencia de series de tiempo (**TimescaleDB**) para miles de dispositivos.
- **Agente IA** (Google **Genkit** en Go) que responde consultas en lenguaje natural sobre el estado de la flota.
- **SPA React** con mapa, alertas en tiempo real (SSE) y chat con la IA.
- **App móvil React Native (Expo)**, offline-first (SQLite/WatermelonDB) con sync en bloque.
- Orchestación local con **Docker Compose**; despliegue cloud con **Terraform**; **k6** para caos/carga; **GitHub Actions** + **Fastlane/EAS** para CI/CD.

### Decisiones arquitectónicas fijadas (NO revertirlas sin ADR)
1. **NATS JetStream** en lugar de Kafka/RabbitMQ: mismo modelo de streams durables + replay, pero un solo binario liviano (crítico: la máquina de desarrollo tiene 16 GB RAM). Justificar en README.
2. **TimescaleDB**: hypertables + compresión para telemetría de alta frecuencia.
3. **Genkit** (Google) para el agente: flows + tools nativas en Go, LLM a Gemini/Vertex.
4. **Go estándar de proyecto**: clean architecture por capas (domain → application → adapters → infra) con interfaces en el dominio y dependencias apuntando hacia adentro.

## Convenciones de código

- **No comentarios** a menos que se pidan; el código debe ser autoexplicativo.
- Naming en inglés; nombres cortos y descriptivos. Entidades de dominio sin acoplamiento a librerías de terceros.
- Errores en Go: siempre `return fmt.Errorf("...: %w", err)` (wrap, nunca `errors.New` aislado); no propagar errores "crudos" a la API pública.
- Interfaces del lado del **consumidor** (dependency inversion): define la interfaz donde se usa, no donde se implementa.
- NO introducir dependencias nuevas sin justificar en el PR/commit. Stack ya decidido: NATS (`nats.go`), TimescaleDB (`jackc/pgx`), genkit-go (google/genkit), gobreaker (sony/gobreaker) para circuit breakers.
- Configuración vía env vars, nunca hardcodeada. Nada de secretos en git.

## Definición de "done" para cada feature

1. Cubre los 4 cuadrantes de la prueba: ingestión, IA, web, móvil, infra, testing.
2. Compila sin errores y pasa `go vet` / lint / tests del módulo tocado.
3. `docker compose config` es válido cuando toca infra.
4. El agente `reviewer` hizo auditoría de clean architecture + seguridad y **no quedaron hallazgos de severidad alta**. Cuando una decisión se candidatea a ADR o el código toca seguridad, pasan además el dictamen de `scalability` (¿escala a miles de dispositivos?) y/o `security`.
5. Toda decisión o refactor relevante quedó registrado en `docs/IAUDIT.md` (auditoría de IA) y/o en un ADR en `docs/adr/`.
6. Commit con mensaje en **Conventional Commits** (`feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`, `ci:`) y relacionado a una tarea del backlog.
7. Se actualizó el `README.md` (instrucciones de ejecución) si cambió la DX.

## Workflow estándar del equipo de agentes

- **`architect`** (agente primario por defecto): descompone el problema, delega a especialistas y evalúa resultados. NO escribe código de features; escudriña y refactoriza lo generado.
- **Especialistas (subagentes)**: `go-backend`, `data-events`, `ai-agent`, `react-web`, `mobile-expo`, `devops`.
- **`security`** (subagente): auditoría dedicada de seguridad (secretos, inyecciones, authN/authZ, exposición de GPS/PII, hardening IaC/móvil) en lo que expone la flota.
- **`scalability`** (subagente): dictamina con números si una decisión escala a miles de dispositivos (particionado, backpressure, storage, resiliencia); obligatoria antes de un ADR.
- **`reviewer`**: auditor estricto (clean architecture, seguridad, resiliencia). Corre antes de dar por hecho un task.
- Regla de oro: **la IA es el exoesqueleto, no la muleta**. Todo código generado pasa por auditoría del `architect`/`reviewer`; los hallazgos y el refactor se documentan en `docs/IAUDIT.md` con al menos 2 ejemplos donde el enfoque sugerido era deficiente/inseguro/no escalable y se forzó el estándar.

## Herramientas y límites

- `context7` (MCP) disponible para docs de librerías (nats.go, pgx, genkit-go, etc.). Usa la skill de cada dominio antes de googlear.
- No dejar levantados servicios Docker de más: la máquina tiene 16 GB RAM. Preferir robustez de diseño sobre réplicas locales.
- Registro de auditoría automático: ediciones de archivos se registran en `.ai-audit/edit-log.yaml` vía plugin.