# ADR-0002 — Estructura del repositorio y del módulo Go (monorepo + bounded contexts)

- **Fecha:** 2026-08-17
- **Estado:** Aceptado (con condiciones; dictamen de `scalability` incorporado)
- **Decisores:** `architect` + dictamen de `scalability` (obligatorio según AGENTS.md para decisiones candidatas a ADR)

## Contexto

La plataforma necesita una estructura de repo y de código Go que soporte el MVP (4 contenedores backend + SPA + Expo + IaC) y que escale en el horizonte declarado (10.000-50.000 dispositivos, 2.000-10.000 msg/s) sin refactors estructurales. La decisión debe respetar las reglas de AGENTS.md: clean architecture por capas (domain → application → adapters → infra) con interfaces en el dominio, dependencias hacia adentro, entidades de dominio sin acoplamiento a terceros, config 100% por env, sin secretos en git.

Alternativas consideradas: multirepo por servicio, módulos Go separados por bin (con `go.work`), layout por capas global sin bounded contexts, schema gestionado por IaC.

## Decisión

**Monorepo único** con el siguiente layout:

```
fleet-monitoring-platform/
├── backend/                 # módulo Go (único)
│   ├── cmd/                 # 4 bins: ingest, consumer, api, agent
│   ├── internal/            # bounded contexts (ver abajo)
│   ├── migrations/          # schema TimescaleDB versionado con el código
│   └── pkg/                 # reservado vacío (librerías públicas futuras)
├── web/                     # SPA React + Vite
├── mobile/                  # Expo + fastlane/ + EAS
├── infra/                   # terraform/ (envs dev/prod) + k6/
├── .github/workflows/       # CI/CD (raíz, obligatorio para GitHub Actions)
├── docker-compose.yml       # dev local (DX desde la raíz)
├── .env.example             # contrato único env entre IaC y código
├── docs/                    # adr/, c4/, IAUDIT.md
└── README.md
```

**Un solo Go module** (`backend/go.mod`) con cuatro bins en `cmd/`:

- `cmd/ingest` → Telemetry Ingest API (write edge, HTTP)
- `cmd/consumer` → Telemetry Consumer (NATS worker → TimescaleDB)
- `cmd/api` → Platform API (read edge, HTTP + SSE alerts)
- `cmd/agent` → AI Agent (Genkit flows + chat)

**Clean architecture por bounded contexts (DDD)** en `backend/internal/`:

```go
internal/
├── telemetry/    # BC ingesta + series de tiempo
│   ├── domain/   # Device, TelemetryEvent, Alert; ports (interfaces)
│   ├── application/  # casos de uso (sin I/O)
│   ├── adapters/ # nats, pg, http (único lugar con deps de terceros)
│   └── infra/    # config/builders
├── fleet/        # BC estado de flota + alertas (lectura/SSE)
├── assistant/    # BC agente IA (Genkit)
└── shared/domain # kernel: value objects (DeviceID, EventID, Timestamp, GeoPoint)
```

Reglas de dependencia: `domain` → solo stdlib + `shared/domain`; `application` → solo `domain` propio; `adapters` → `application` + `domain` + librerías (nats.go, pgx, genkit); `infra` + `cmd/*` = composition root (wiring). Los BCs se comunican por eventos/ports, nunca por imports entre BCs.

**`migrations/` en el repo** (no en IaC): el schema de aplicación (hypertables, compresión, continuous aggregates) se versiona con el código. Terraform gestiona solo el ciclo de vida del recurso DB (instancia, red, roles, extensiones) — nunca DDL evolutivo.

**`pkg/` reservado vacío**: convención de límite para librerías públicas; no publicar nada del dominio ahí sin un consumidor externo real.

## Condiciones obligatorias (dictamen `scalability`)

1. **Enforcement como regla de CI desde el día 1** (`go-arch-lint`/`arch-go`/`golangci-lint` con `depguard`): `cmd/<bin>` → solo su BC + `shared/domain`; `domain/`+`application/` → import list vacía de terceros y de adapters; BCs se comunican solo por ports. **Obligatorio sí o sí; a partir de 3 BCs es condición de merge.** Sin esto, el módulo único degenera en acoplamiento silencioso.
2. **`shared/domain` = solo value objects**, tope **30 archivos / 500 LOC**. Disparador de split a módulo propio (`go.work`): pasar el tope, primer conflicto de versiones entre bins, o **>8-10 `cmd/` bins**. La recompilación del árbol al tocar el kernel es aceptable (1-2 min incremental, 30-60 paquetes) mientras se mantenga el tope.
3. **CI con path-filtering desde el día 1** (`paths:` en GitHub Actions): pipeline full solo ante cambios en raíz; jobs por path para el resto. Umbral de revisión del monorepo: **full >20 min o mediana de PR >10 min** con filtros activos → el build full por docs/ es la razón equivocada para ir a multirepo.
4. **Disparadores de split del monorepo (documentados, NO implementados)**: ≥2 de (a) >2 equipos con release cadence propia, (b) >6-8 artefactos desplegables, (c) CI >20 min, (d) git >300-500 MB. **Orden de split: `mobile/` primero** (cadencia de tienda/EAS), luego `infra/` (apply independiente). **Nunca** split por servicio Go — comparten dominio, migrations y deploy.
5. **Migrations aplicadas por un único job con `pg_advisory_lock`**: nunca desde réplicas del consumer en carrera. Terraform solo crea el recurso DB vacío + roles/extensions; hypertable/compresión/continuous aggregates en la migration 0001. Gateo per-env solo cuando haya **>2 entornos prod-like**.
6. **`.env.example` como contrato único** mientras haya **<50 variables y <3 entornos** de config divergente; pasado eso, overlays por servicio manteniendo el canónico en raíz. No exponer en el contrato de IaC variables que no necesita (ej. `GEMINI_API_KEY` solo lee el agent).
7. **Footprint 16 GB**: nunca 4 bins con `-race` simultáneos (mín. 1 bin por feature con race); k6 solo en CI (1.000 VUs ≈ 1-2 GB); presupuestar la VM de Docker Desktop (2-4 GB). Total vivo esperado 6-9 GB de los 16.
8. **Gobernanza de pruebas**: unit = `go test ./...` (paralelo por paquete, 2-3 min); integración con NATS/DB gateada con build tags (`//go:build integration`) en job aparte, para que un flaky de integración no bloquee el módulo completo.

## Consecuencias

**Positivas:**
- Consistencia de versionado: docker-compose en el commit X construye el código del mismo commit — deploy determinista sin manifests de imágenes.
- Un solo `go.sum` resuelve el grafo completo de dependencias: no hay infierno de versiones entre bins.
- Tests unitarios cubren todo el módulo con un solo comando (un job de CI, resultado único).
- El layout es **carga-independiente**: escalar de 1k a 10k msg/s no cambia nada de estructura (se replican los mismos 4 bins y se aplican shards del ADR-0001).
- Migrations con el código: dev las aplica en arranque del contenedor, prod en deploy; el schema nunca es constraint de escala (KB vs GB/día de telemetría).

**Negativas:**
- El enforcement de imports es responsabilidad continua del CI — sin él, el módulo único degenera en acoplamiento al cabo de 3-4 BCs.
- `shared/domain` es el único nodo con fan-out 100%: cada cambio recompila el árbol (1-2 min incremental) — mitigado por el tope de 30 archivos.
- Los tests de integración exigen job aparte con build tags, añadiendo complejidad al pipeline.

**Puntos de quiebre identificados (en orden de aparición):**
1. Enforcement ausente en CI (acoplamiento silencioso entre bins/BCs) — quiebre de hoy, no de escala.
2. `shared/domain` God package >30 archivos.
3. CI sin path-filtering → build full de 15-20 min por cualquier cambio.
4. Disparadores de split no escritos — partir tarde es 10× más caro.

## Referencias

- Dictamen `scalability` 2026-08-17 (ADOPTAR CON CONDICIONES): estimaciones de build cold backend 3-6 min, pipeline full 13-20 min, mediana PR 4-8 min con path-filters; footprint dev vivo 6-9 GB; umbrales numéricos de quiebre y split (secciones 1-6 del dictamen).
- ADR-0001 (NATS JetStream backbone) — del que este ADR depende estructuralmente.
- H2 en `docs/IAUDIT.md` — controles P0 de seguridad que este layout materializa (`.env.example` raíz, `.gitignore`, guard en CI).
- Registrado en `docs/IAUDIT.md` (A2 — validación de `scalability` previa a ADR).