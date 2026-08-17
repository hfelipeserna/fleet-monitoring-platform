---
name: genkit-agent
description: Usa esta skill al implementar el agente IA con Google Genkit en Go: flows, tools conectadas al dominio, Gemini, dev, guardrails y cómo exponerlo a la SPA. Trigger: Genkit, genkit-go, agente, agente IA, LLM, Gemini, tool, flow, IA, AI, natural language.
---

# Agente IA con Google Genkit (Go)

Genkit es el framework de Google para apps de IA con soporte nativo de Go. Un generativo: **flows** (pipeline invocable, testable, trazable) + **tools** que conectan el agente con datos reales de la flota.

## Estructura

- `agent.go`: definí el flow (`genkit.DefineFlow`).
- `tools.go`: funciones Go expuestas al LLM vía `genkit.DefineTool`.
- `types.go`: define el "tool schema" en JSON schema para que Gemini sepa qué llamar.
- El flujo corre como servicio Go dentro del backend; la SPA lo consume por HTTP/SSE (ver skill `realtime-dashboard`).

## Patrón de tool (lo importante)

```go
// La tool consulta al APPLICATION/adapters, no a la DB directa.
getStopped := genkit.DefineTool(
  ai, "getVehiclesStopped",
  jsonSchemaFor{VehStoppedQuery{}},
  "Return vehicles stopped for at least the given duration inside an optional area",
  func(ctx context.Context, q VehStoppedQuery) ([]Vehicle, error) {
      return svc.ListStopped(ctx, q.MinStop, q.Area, time.Now())
  },
)
```

- Cada tool = **una consulta verificable al dominio**. El LLM jamás genera ficción: si la tool no responde, el agente dice "no pude consultar el estado".
- Firewall de capabilities: solo exponé tools de LECTURA de estado/alerta; nada de escritura ni acciones destructivas.

## Sistema/contexto

- Prompt system con: rol, formato de respuesta esperada, y **contexto resumido de flota** (N activos, N detenidos, top alertas) actualizado antes de cada turno.
- Pasa el estado como contexto (no le pidas a la IA adivinar); el LLM razona y llama tools para profundizar.

## Guardrails

- Valida entrada del usuario: longitud, contenido (rechazar instrucciones de prompt-injection lo mejor posible), no más de N tools por turno.
- Timeout global del flow y límite de tokens (maxOutputTokens). Fallback genérico si el LLM no responde (circuit breaker/handling de error).

## Dev

- Working dev flow: `genkit dev` (o exporta un flow HTTP en modo dev) para probar tools/prompt sin desplegar. Documentá el comando en README.

## Verificación

- `go build ./...`, `go vet ./...`, tests unitarios del flow con tools con doble/fake del repositorio.