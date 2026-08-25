# ADR-0008 — LLM provider pluggable en cmd/agent: Zen (OpenCode) como primario, Gemini como fallback

- **Fecha:** 2026-08-25
- **Estado:** Aceptado (enmienda a ADR-0003; SPEC-003 BR-011 sin fallback hallucinated)
- **Decisores:** `architect`
- **Afecta a:** ADR-0003 (Genkit/Gemini), ADR-0004 (secretos), `backend/cmd/agent/bootstrap.go`, `docker-compose.yml`, `.env.example`, SPEC-003

## Contexto

SPEC-003 fijó `gemini-2.5-flash` vía `googlegenai` con `GEMINI_API_KEY`/`GEMINI_MODEL` (ADR-0003 cond. 7, free tier $0). En la ventana de demo 2026-08:

- Free tier Gemini ~20 req/día por proyecto/modelo; excedido bloquea el chat del agente en la demo.
- Keys nuevas de AI Studio con sufijo `AQ.` responden `404 Model not found` / `429` incluso en `gemini-2.5-flash`; `gemini-3.6-flash` también `429 quota exceeded`. No es recoverable desde código.
- Se probaron dos proxies OpenAI-compatibles: **OpenRouter** (requiere saldo inicial, sin free tier útil) y **Zen (opencode.ai)** (`https://opencode.ai/zen/v1`, OpenAI-compatible). Zen en plan free sin saldo inicial sirve `hy3-free` sin quota Gemini y mantiene el flujo demo sin pagar.

Sin LLM disponible, `POST /api/chat` cae a `503 agente temporalmente no disponible` (BR-011 de SPEC-003 prohíbe fallback hallucinated — no se puede inventar respuesta sin LLM). Bloquea el cuadrante IA de la prueba técnica.

Restricciones: mantener `genkit-go` (ADR-0003), costo $0 MVP, tools read-only + breaker + guardrails intactos, sin secretos en git (ADR-0004), y `docker compose config` sin leaks.

## Decisión

**LLM provider pluggable en `cmd/agent` vía Genkit, con selección por env var en `bootstrap.go`:**

```go
if ocKey := os.Getenv("OPENCODE_API_KEY"); ocKey != "" {
    ocBase  := envOr("OPENCODE_BASE_URL", "https://opencode.ai/zen/v1")
    ocModel := envOr("OPENCODE_MODEL", "hy3-free")
    oai := &openai.OpenAI{APIKey: ocKey, Opts: []option.RequestOption{option.WithBaseURL(ocBase)}}
    g := genkit.Init(ctx, genkit.WithPlugins(oai), genkit.WithDefaultModel("openai/"+ocModel))
    flow.SetGenkit(g, "openai/"+ocModel)
} else {
    apiKey   := os.Getenv("GEMINI_API_KEY")
    modelEnv := envOr("GEMINI_MODEL", "gemini-2.5-flash")
    g := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: apiKey, APIVersion: "v1"}),
        genkit.WithDefaultModel("googleai/"+modelEnv))
    flow.SetGenkit(g, modelEnv)
}
```

- **Si `OPENCODE_API_KEY` presente** → plugin `compat_oai/openai` contra `OPENCODE_BASE_URL` (default `https://opencode.ai/zen/v1`) con modelo `OPENCODE_MODEL` (actual `hy3-free`; verificados vía Zen: `big-pickle`, `muse-spark-1.2-contributor-free` también disponibles free).
- **Sino fallback** → `googlegenai` con `GEMINI_MODEL` (default `gemini-2.5-flash`), idéntico a SPEC-003 original.
- Prioridad explícita: Zen primero (desbloquea demo), Gemini como fallback sin cambiar código.
- Todo lo demás invariante: `assistantFlow`, tools read-only con allowlist JWT, `gobreaker`, timeout 15s, `maxOutputTokens 1024`, output filter, `GENKIT_ENV`, BFF `cmd/api → cmd/agent`.

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo |
|---|---|---|
| **Zen pluggable + fallback Gemini (elegida)** | Adoptada | Costo $0 se mantiene (Zen free sin saldo, `hy3-free` sin quota Gemini), un solo `if` en `bootstrap.go`, sin duplicar flows/tools, demo desbloqueada |
| **Solo Gemini** | Descartada | Bloquea demo por 404/429 en keys nuevas y cap 20/día; sin mitigación |
| **OpenRouter como primario** | Descartada | Exige prepago/saldo inicial; rompe costo $0 MVP aunque sea OpenAI-compatible |
| **Cliente OpenAI directo sin Genkit** | Descartada | Pierde `DefineFlow/DefineTool`/tracing de Genkit (ADR-0003); reinventar sin valor |
| **Fallback hallucinated sin LLM** | Prohibida | Viola SPEC-003 BR-011: sin LLM → `503`, nunca respuesta inventada |

## Condiciones obligatorias

1. **Env vars (ADR-0004):** `OPENCODE_API_KEY`, `OPENCODE_MODEL`, `OPENCODE_BASE_URL` solo por env (`${VAR}` en compose, nunca literal). `.env.example` declara nombres sin valores; `.gitignore` cubre `.env* !.env.example`; gitleaks/Push Protection bloquean leak. `GEMINI_API_KEY`/`GEMINI_MODEL` se conservan como fallback.
2. **Compose:** servicio `agent` expone `OPENCODE_API_KEY`, `OPENCODE_MODEL:-hy3-free`, `OPENCODE_BASE_URL:-https://opencode.ai/zen/v1` junto a `GEMINI_*`; `docker compose config` no revela secretos.
3. **Healthz:** `GeminiState()` reporta `connected:opencode:<model>` si Zen activo, `connected:gemini:<model>` si Gemini, `missing-key` si ninguno. `GET /healthz` nunca expone el valor de la key.
4. **Sin fallback hallucinated (BR-011):** si ambas keys ausentes → `503 {error:"503 service unavailable: no LLM API key configured"}` con `Retry-After: 30` y breaker open; nunca `200` con reply inventado.
5. **Guardrails intactos:** tools read-only, scope por JWT en código, output filter (`GEMINI_API_KEY`/`OPENCODE_API_KEY`/`DATABASE_URL`/SQL/JWT), rate limit 10 req/min, `GENKIT_ENV=prod` sin DevUI expuesta.
6. **Costo $0:** modelos permitidos en MVP: `hy3-free`/`big-pickle`/`muse-spark-1.2-contributor-free` vía Zen o `gemini-2.5-flash` vía Gemini; Pro/paid solo vía ADR posterior con Vertex IAM.

## Consecuencias

**Positivas:**
- Demo desbloqueada sin quota Gemini ni saldo OpenRouter; `hy3-free` vía `https://opencode.ai/zen/v1` responde dentro del free tier Zen.
- Cambio confinado a `backend/cmd/agent/bootstrap.go` + envs; domain/application/tools/breaker/guardrails sin tocar.
- Fallback preserva compatibilidad con SPEC-003 original (proyectos con `GEMINI_API_KEY` válida siguen funcionando).
- Healthz distingue proveedor activo para diagnóstico sin exponer secretos.

**Negativas:**
- Dos proveedores que documentar/rotar (dos keys en vez de una); Zen es dependencia externa adicional sin SLA para MVP (mitigado por fallback a Gemini).
- `OPENCODE_BASE_URL` y `OPENCODE_MODEL` amplían superficie de config (asumido: <50 vars, ADR-0002).
- Modelos Zen free pueden cambiar de disponibilidad/cuota sin aviso; requiere pin de `OPENCODE_MODEL` por env y test de smoke en CI.

## Referencias

- ADR-0003 (Genkit + Gemini free tier, cond. 1/7/9) y ADR-0004 (secretos por env, gitleaks)
- SPEC-003 `docs/specs/SPEC-003-assistant-chat/spec.md` BR-011 (sin fallback hallucinated → 503)
- `backend/cmd/agent/bootstrap.go:60-67` (`GeminiState`), `:122-154` (branch `OPENCODE_API_KEY` vs `GEMINI_API_KEY`)
- `docker-compose.yml:115-119` (envs `GEMINI_*` + `OPENCODE_*`), `.env.example:28-29`
- Incidencias Gemini 2026-08: `404 AQ.` new keys, `429` `gemini-2.5-flash`/`3.6-flash`, free tier 20/día; pruebas OpenRouter (requiere saldo) y Zen `https://opencode.ai/zen/v1` con `hy3-free` ok
