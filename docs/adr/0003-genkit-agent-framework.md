# ADR-0003 — Framework del agente IA: Google Genkit (genkit-go) + Gemini

- **Fecha:** 2026-08-21
- **Estado:** Aceptado (con condiciones; dictamen de `security` incorporado)
- **Enmienda 2026-08-22:** validación de costo — el MVP corre en free tier de la Gemini API (costo $0); ver condición 7
- **Enmienda 2026-08-22 (aislamiento front↔agente):** fachada BFF obligatoria — ver condición 9 nueva; vinculante para diseño y SPECs posteriores (coherente con ADR-0002 y ADR-0005)
- **Decisores:** `architect` + dictamen de `security` (obligatorio según AGENTS.md: el componente expone un endpoint de chat con acceso a datos de flota) + enmienda de aislamiento validada por `architect`

## Contexto

El BC `assistant` debe exponer un agente que responda consultas en lenguaje natural sobre el estado actual de la flota (ej. "¿qué vehículos llevan detenidos más de 20 minutos en zonas críticas?"), integrado en el backend Go mediante flows y tools que consultan datos reales vía port hacia `fleet`. El portal web lo consume por HTTP.

Restricciones del entorno: backend en Go con clean architecture (el framework no puede contaminar domain/application), repositorio público, máquina dev macOS Intel 16 GB, MVP de prueba técnica con horizonte de diseño de miles de dispositivos.

Requisitos del framework: soporte Go nativo y maduro, abstracción de flows (pipeline invocable/testable/trazable) y tools (funciones tipadas que conectan el LLM con los ports del dominio), observabilidad incluida, y modelo accesible sin contratos cloud para el MVP.

## Decisión

**Google Genkit (`genkit-go`) como framework del agente**, con **Gemini como modelo** vía API key (Google AI Studio) para el MVP:

- Los flows de Genkit viven exclusivamente en `assistant/adapters/` — domain/application del BC no importan nada del framework.
- Las tools son funciones Go registradas al flow que invocan ports del dominio; read-only por diseño.
- Exposición al portal: endpoint HTTP de chat → flow Genkit → Gemini (+ tools) → respuesta filtrada.

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo |
|---|---|---|
| **LangChain** | Descartada | Ecosistema Python/JS; bindings Go no oficiales y de madurez insuficiente |
| **Semantic Kernel** | Descartada | Foco .NET/MSFT; sin soporte Go |
| **Cliente directo de Gemini API** | Descartada | Máximo control pero reinventar flows/tools/tracing/retries sin valor diferencial para el MVP |
| **Genkit (elegida)** | Adoptada | Única con soporte Go nativo de primera parte (Google), flows+tools+tracing integrados, modelo Gemini sin fricción |

## Condiciones obligatorias (dictamen `security`)

1. **GEMINI_API_KEY**: solo env var en runtime (nunca default en código ni compose en claro — `${GEMINI_API_KEY}`); `.gitignore` cubre `.env*` salvo `.env.example`; secret scanning (gitleaks / GitHub Push Protection); procedimiento de rotación documentado (la key se considera comprometida si aparece en cualquier log o trace).
2. **Tools read-only con allowlist explícita**: toda validación de scope (qué vehículos/flotas puede ver el usuario) ocurre en código Go dentro de la tool usando la identidad del JWT — nunca delegada al LLM ni a IDs libres generados por el modelo.
3. **Prompt injection**: system prompt con anti-injection declarado PERO la confianza nunca recae en el prompt; controles reales en código (cond. 2) + output filtering post-LLM contra exfiltración de secretos/tokens/SQL.
4. **Límites de minimización de datos en tool layer**: máximo N vehículos por respuesta, campos mínimos (sin PII de conductor), precisión de posición limitada (~3 decimales), preferencia por agregados sobre listas crudas cuando la pregunta lo permita.
5. **Endpoint de chat**: authN JWT obligatoria, rate limit por usuario/IP (~10 req/min), límite de tamaño de input (~4 KB), timeout duro del flow (~15 s) con ctx propagado a tools/pgx, `maxInputTokens`/`maxOutputTokens` fijos (~1024 out), gobreaker sobre la llamada al LLM, semáforo global de concurrencia.
6. **Observabilidad**: Dev UI de Genkit prohibida fuera de desarrollo local y jamás expuesta por Compose; en producción traces sin contenido de prompts/outputs (solo metadatos); logs del endpoint sin contenido de chat ni PII.
7. **Costo cero para el MVP**: el agente opera exclusivamente dentro del free tier de la Gemini API. Modelos clase Flash únicamente (`gemini-2.5-flash` por defecto, seleccionable vía env var `GEMINI_MODEL` sin tocar código; a ago-2026 los modelos Pro no tienen capa gratuita). La key se crea en AI Studio sin tarjeta de crédito y debe quedar restringida al endpoint de Generative Language (obligatorio por política de Google desde jun-2026; las keys nuevas de AI Studio ya nacen restringidas). Presupuesto/alerta de gasto en la API key como red de seguridad ante exceder el free tier (límites ~10 RPM y cap diario según modelo/proyecto — coherente con el rate limit del endpoint de chat, cond. 5); migración a Vertex AI + service account/IAM anotada como condición previa a producción real (la AI Studio key es secreto plano sin IAM fino — aceptable solo en MVP).
8. **Re-auditoría `security` sobre la implementación concreta** del BC `assistant` antes de cerrar su SPEC: este dictamen cubre la decisión, no el código.
9. **Aislamiento front↔agente vía backend (BFF / Anti-Corruption Layer) — enmienda 2026-08-22, vinculante para diseño/SPECs posteriores:** el portal web (SPA React) **nunca** accede a TimescaleDB, NATS JetStream, Genkit ni a `GEMINI_API_KEY`, ni conoce prompts, tools o SQL. Flujo obligatorio: `SPA → cmd/api (POST /api/chat, SSE /api/alerts) → cmd/agent (flow Genkit) → tools read-only → port fleet → TimescaleDB/NATS`. Prohibido: (a) importar `pgx/nats.go/genkit` en `web/`, (b) llamar directo SPA→Gemini/DB, (c) exponer prompts/tools/SQL o trazas con PII al front. `cmd/api` es la única superficie pública; valida JWT, aplica rate limit/timeout/gobreaker (cond. 5) y filtra salida; `cmd/agent` valida scope por JWT en código (cond. 2) y minimiza datos (cond. 4). Les SPECs web/agent deben citar esta fachada en sus ACs y testear `401/429` sin tocar DB; `depguard` en CI bloquea imports `web→assistant` y `assistant/domain→adapters`.

## Consecuencias

**Positivas:**
- Soporte Go nativo de primera parte: flows y tools tipados compilan con el resto del módulo; sin FFI ni servicios intermedios en otro lenguaje.
- Flows testables e instrumentados de fábrica (tracing, steps) — acelera el cumplimiento de ACs del SPEC del agente.
- Cambiar de modelo (Gemini ↔ Vertex ↔ otros) queda confinado a adapters; domain/application intactos.
- Footprint mínimo local: no añade contenedores (contrasta con alternativas self-hosted).

**Negativas:**
- Acoplamiento al roadmap de genkit-go (framework joven; API susceptible de cambios — mitigado por confinarlo a adapters).
- La Dev UI es superficie de exposición si se despista a un perfil compartido (condición 6 obligatoria).
- At-least-once de latencia LLM (~segundos por request): el chat exige budgets de tokens/timeout explícitos o el costo escala sin control (condiciones 5 y 7).

## Referencias

- Dictamen `security` 2026-08-21 (APROBAR CON CONDICIONES): OWASP A07/A01/A04/A05/A09, OWASP LLM01 (prompt injection), GDPR art. 5 (minimización).
- AGENTS.md decisión fija #3 ("Genkit para el agente") — este ADR la materializa y condiciona.
- ADR-0002 (layout): `GEMINI_API_KEY` fuera del contrato de IaC; BC `assistant` aislado por depguard.
