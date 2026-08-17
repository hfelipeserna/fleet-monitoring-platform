---
name: clean-architecture
description: Usa esta skill SIEMPRE al escribir, revisar o refactorizar código Go (o cualquier capa) de este repo. Reglas de capas (domain → application → adapters → infra), inversión de dependencias, manejo de errores, convenciones de naming y configuración. Trigger: clean architecture, capas, domain, aplicación, interface, dependencias, errors, go vet, refactor.
---

# Clean Architecture (estándar de proyecto)

La regla de oro: **las dependencias apuntan hacia adentro**. El dominio es el centro y no sabe nada de la tecnología.

## Capas y dependencias permitidas

| Capa | Importa |
|---|---|
| `domain` | solo stdlib y sus propias entidades/valores. NADA de terceros. |
| `application` | `domain` (usa sus interfaces/port). Orquesta casos de uso. |
| `adapters` | `application` + `domain`. Implementa interfaces del dominio (repos, publishers, providers). |
| `infra` | todo lo externo (NATS, pgx, LLM): nunca debe ser importado por `domain`. |

- Las **interfaces se definen donde se consumen** (consumer side). Ej: el caso de uso define `TelemetryWriter`; el adapter de NATS lo implementa.
- Un paquete `domain` que importe `infra/...` o `adapters/...` es un **hallazgo de severidad alta**.

## Errores en Go

```go
// SIEMPRE wrap y contexto:
return fmt.Errorf("persist telemetry %s: %w", eventID, err)

// NUNCA:
return err // error "crudo" hacia arriba
errors.New("algo falló") // sin wrap ni contexto
```

- En la API pública (handlers/gateways) traduce errores de dominio a HTTP/SSE con mensaje seguro (sin volcar stack interno).

## Naming y estilo

- Nombres cortos y descriptivos en inglés. Sin comentarios salvo que se pidan (autoexplicativo).
- Sin acoplamiento: entidades de dominio con tipos propios (`VehicleID`, `Status`, `GeoPoint`), no `string`/`float64` sueltos.

## Configuración

- 100% env vars; defaults solo para desarrollo local. Nunca credenciales en código ni en archivos versionados.

## Checklist al tocar código

1. `go build ./...` y `go vet ./...` en verde (o tsc/lint para TS).
2. Ninguna import prohibida entre capas.
3. Errores wrapped con `%w`.
4. Interfaces en el lado consumidor.
5. Config off env vars.