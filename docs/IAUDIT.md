# IAUDIT — Auditoría de IA (exoesqueleto, no muleta)

Registro de auditorías del código generado por agentes IA. Requisito del entregable:
documentar **al menos 2 decisiones donde el enfoque sugerido por la IA fue
deficiente/inseguro/no escalable** y cómo se forzó el estándar.

## Formato de entrada

```
## <fecha> — Auditoría: <scope> [SPEC-XXX]
Severidad: alta | media | baja
Hallazgo: <qué sugirió/hizo la IA>
Evidencia: archivo:línea (estado previo al refactor, ver git)
Por qué falla: <explicación técnica / estándar internacional aplicable>
Refactor exigido: <cómo se resolvió>
Auditor: reviewer | security | scalability | db-auditor | quality-auditor | architect
```

## Entradas

## 2026-08-23 — Auditoría: application/ingest.go SRP/DRY/CC [SPEC-001]
Severidad: alta
Hallazgo: go-backend generó IngestService god con IngestSingle CC 15 e IngestBatch CC 21 duplicando 70 líneas (validateRaw+enrich+domainValidate+rate+backpressure+publish y generateUUID en ambos, triple containsLower con 6 allocs por error, doble %w en 8 sitios).
Evidencia: backend/internal/telemetry/application/ingest.go:64-181 (pre 8283b5b, ver git fe5fb8a^) y quality-auditor ses_fcff1821
Por qué falla: Violación SRP/DRY/OCP SOLID, CC >10 en hot path 1k msg/s exige 19 casos de test, duplicación diverge, `containsLower` triple O(m) en backpressure caliente, doble `%w` rompe AGENTS.md Go error wrapping.
Refactor exigido: Extraídos ports Clock/IDGenerator/RawValidator, processOne/enrich, checkBreaker/checkJetStream, classifyPublishError, const MaxBatchSize/highWatermark/maxFutureSkew, single ToLower, errors.Join single %w, infra/idgen delegación. IngestSingle CC 5, IngestBatch CC 8, 0 duplicación. Commit fe5fb8a.
Auditor: quality-auditor | architect

## 2026-08-23 — Auditoría: adapters/http/handler.go handler gigante [SPEC-001]
Severidad: alta
Hallazgo: IA generó handler con handleSingle 122L CC20 y handleBatch 146L CC25 copiando 70L literales (plate/speed/lat/lon/cid/occurred_at), 3000 Unmarshal por batch 500 (6x overhead), r.Body duplicado y magic 1<<20/500/5 dispersos.
Evidencia: backend/internal/telemetry/adapters/http/handler.go:50-318 (pre fe5fb8a, ver git) y quality-auditor ses_fcfe79c5
Por qué falla: SRP violado (routing+parsing+metrics en 1 type), OCP violado (añadir campo obliga 2 ediciones), CC 20-25 >>10, 6x allocs en hot path batch offline 245.
Refactor exigido: Extraídos decodeSingleEvent/RequiredFields/getRequiredRaw/parseOptionalFloat/String/parseOccurredAt/decodeBatch (CC ≤8, ≤40L), const maxBodyBytes/retryAfter reuse application.MaxBatchSize, NewHandlerWithService para inyectar Clock/IDGen, handleSingle/Batch CC 6 cada uno (394L). Commit fe5fb8a.
Auditor: quality-auditor | architect

## 2026-08-23 — Auditoría: infra/breaker no-op + rate limiter leak [SPEC-001]
Severidad: alta
Hallazgo: breaker.RecordFailure() era `_ = State()` no-op y Allow() hacía Execute(nil)->success, nunca abría; rate limiter NewLimiter go cleanupLoop() sin Stop()/ctx leak goroutine y ticker nunca cerrado.
Evidencia: backend/internal/telemetry/infra/breaker/breaker.go:42-53 y backend/internal/telemetry/infra/rate/limiter.go:22-78 (pre fe5fb8a) y reviewer ses_fcfe5ba8
Por qué falla: Sin feedback loop backpressure real NATS max_pending nunca abre circuito, handler sigue aceptando hasta OOM/JetStream lleno; leak 16GB RAM (AGENTS.md límite) NFR-004.
Refactor exigido: Reescrito breaker con gobreaker Execute correcto (RecordFailure via Execute(error)), NewBreakerWithSettings 10/0.5/30s, breaker_test abre tras 10 fallos; rate NewLimiterWithContext(ctx)+Stop()/select ctx.Done. Commit fe5fb8a + 70be5f1 cableado publisher/breaker.
Auditor: reviewer | quality-auditor | architect

## 2026-08-23 — Auditoría: adapters/http/dlq.go y consumer God [SPEC-001]
Severidad: alta
Hallazgo: dlq.go ServeHTTP 91L CC22 con 4 responsabilidades (framing+limit query+body+fetch+republish loop con extractPlate por msg), consumer.go HandleMessage vs ProcessBatch duplicaban 13L idénticos Ack/Nak/DLQ + parsePayload 85L CC18 con 6 Unmarshal por msg y getDelivered type-switch frágil.
Evidencia: backend/internal/telemetry/adapters/http/dlq.go:29-120, backend/internal/telemetry/application/consumer.go:69-283 (pre 70be5f1) y quality-auditor ses_fcfc4425
Por qué falla: SRP/DRY violado, CC 22/18 >>10, hot path 1k msg/s paga 6000 allocs/s, duplicación resiliencia NFR-002/BR-010 diverge.
Refactor exigido: dlq parseRepublishLimit/sanitizeDLQLimit/fetchDLQLimited/republishAll/resolveSubject (ServeHTTP 15L CC6), consumer partitionValid/handleWriteResult/backoffFor, helpers parsePlate/Speed/Float/Time, Msg.Delivered() único, HandleMessage delega a ProcessBatch. Commit 70be5f1.
Auditor: quality-auditor | architect

## 2026-08-23 — Auditoría: hypertable UNIQUE sin received_at [SPEC-001]
Severidad: alta
Hallazgo: IA propuso PK (client_event_id, received_at) ok pero UNIQUE INDEX (client_event_id) solo en hypertable y ON CONFLICT (client_event_id) DO NOTHING — Timescale exige UNIQUE incluya columna partición, DDL falla y dedup end-to-end rota (mismo client_event_id con distinto received_at duplica).
Evidencia: backend/migrations/0001_telemetry.sql:28 (pre 70be5f1), backend/internal/telemetry/adapters/pg/writer.go:133 ON CONFLICT (client_event_id) y db-auditor ses_fcfc9e66
Por qué falla: Timescale doc hypertable constraint, BR-004/FR-003 idempotencia MsgId + ON CONFLICT, NATS DuplicateWindow 2m.
Refactor exigido: Eliminado UNIQUE en hypertable, DROP INDEX legacy, creada telemetry_dedup PK client_event_id no-hypertable, writer CTE WITH new_ids INSERT INTO dedup SELECT DISTINCT ON CONFLICT DO NOTHING RETURNING + INSERT JOIN new_ids, staging CopyFrom tipado *float64, Duplicates 2m (no 24h) en stream.go. Commit 70be5f1.
Auditor: db-auditor | reviewer | architect

## 2026-08-23 — Auditoría: cmd/consumer composition root God [SPEC-001]
Severidad: media
Hallazgo: cmd/consumer/main.go 324L mezclaba infra NATS+PG+adapter DLQ+health+loop con ensureStream duplicado en ingest, dlqJetStream/sanitizeDLQLimit/resolveSubject duplicado con dlq.go, sync.Mutex per-request leak PullSubscribe, getDelivered 3 interfaces.
Evidencia: backend/cmd/consumer/main.go:25-324 (pre 70be5f1) y quality-auditor ses_fcfc3207
Por qué falla: Violación AGENTS.md clean architecture capas domain->application->adapters->infra, composition root debe cablear no contener lógica infra.
Refactor exigido: Extraídos infra/nats/stream.go EnsureStream/Consumer, infra/nats/dlq.go DLQJetStream/DLQMsg/sync.Once, infra/nats/msg.go NatsMsg, infra/env/env.go Get*, cmd/ingest bootstrap.go/server.go y cmd/consumer bootstrap.go/runner.go, mains 46L/48L orquestadores. Commit 70be5f1.
Auditor: quality-auditor | architect

## 2026-08-23 — Auditoría: infra compose trust + DLQ exposed + secrets hardcode [SPEC-001]
Severidad: alta
Hallazgo: docker-compose.yml usaba POSTGRES_HOST_AUTH_METHOD=trust (bypass auth), consumer 8082:8081 mapeado 0.0.0.0 exponiendo /internal/dlq/republish sin auth, Grafana admin/admin hardcodeado y Duplicates 24h (scalability OOM 5-8GB RAM/día).
Evidencia: docker-compose.yml:44,99,158, infra/nginx/nginx.conf sin /internal block (pre e228387) y reviewer ses_fcf7af84 + scalability ses_fcf78999
Por qué falla: OWASP A07 auth bypass, CIS Docker, 12-factor config vía env nunca hardcode, JetStream Duplicates 86M ids/día ~5-8GB RAM mononodo, NFR-001/004.
Refactor exigido: Eliminado trust (scram-sha-256 default, pg_isready con PGPASSWORD), consumer 127.0.0.1:8082:8081 + nginx location /internal/ {return 404;}, Grafana ${GF_SECURITY_ADMIN_*} via .env, Duplicates 2m, limit_req_zone, stop_grace 20s, 127.0.0.1 bindings, non-root USER app en Dockerfile, healthchecks. Config valida post-fix. Pendiente commit final.
Auditor: reviewer | scalability | architect

## 2026-08-24 — Auditoría: spec/zonas Polygon abierto y sin área/límite [SPEC-002]
Severidad: media
Hallazgo: IA generó spec inicial con `PolygonGeometry` sin cierre obligatorio (aceptaba 3 coords para triángulo `[[a,b],[c,d],[e,f]]`), sin `ST_Area>0` y sin `maxItems`, y `critical_zones` solo `CHECK(ST_IsValid(geom))`. Mermaid `B[POST /api/zones {name, geojson Polygon}]` sin comillas rompía parser.
Evidencia: docs/specs/SPEC-002-fleet-read-zones/spec.md:144,176, plan.md:74, contracts/http.openapi.yaml:398 (pre 2026-08-24, ver git diff)
Por qué falla: Viola RFC 7946 LinearRing `first==last` y `>=4` (triángulo son 4 pos con cierre, no 3), permite zona línea degenerada área 0 que pasa `ST_IsValid` pero `ST_Within` nunca alerta; sin `ST_NPoints<=101` permite DoS GIST O(n) y rompe NFR-001 p95; Mermaid con `{` sin quoting genera `Parse error DIAMOND_START`.
Refactor exigido: BR-002 reescrita `first==last, 4..101 coords (<=100 vértices), SRID 4326, ST_Area>0, ST_IsValid`; OpenAPI `coordinates maxItems:1 / minItems:4 maxItems:101` + descripción `ST_Area>0`; plan `CHECK(ST_Area>0 AND ST_NPoints BETWEEN 4 AND 101)` + validación Go 2 capas `ST_Area==0 ->400`; `AC-003/TS-003` cubren `>101 ->400` y `4 coords colineales área 0 ->400`; Mermaid `B["POST ..."]` y `C{"¿...?"}` quoted. Commit spec-002 hardening.
Auditor: architect | db-auditor

## 2026-08-24 — Auditoría: spec/detector ticker como regla de negocio [SPEC-002 -> SPEC-003]
Severidad: media
Hallazgo: IA propuso `FR-005` detector continuo `ticker 30s SELECT ST_Within speed=0 >20m -> Publish alerts.critical Nats-Msg-Id=plate:zone:bucket` como `Flow 2` de `SPEC-002` y `AC-005` con `Given GTP890 speed0 inside zona 25m When tick`.
Evidencia: docs/specs/SPEC-002-fleet-read-zones/spec.md:132,289,340, plan.md FR-005 (pre 2026-08-24)
Por qué falla: PRUEBA-TECNICA sec 4.B formula `¿vehículos >20m en zonas críticas?` como consulta del chat (tool Genkit en SPEC-003), no como alerta push del dashboard; acoplarlo a `SSE /api/alerts` crea endpoint ficticio, duplica fuente de verdad mapa vs agente (ADR-0007 cond.4) y obliga a retención horaria por tick.
Refactor exigido: Eliminado `Flow 2` detector de SPEC-002; FR-005 reescrito a `alerts.critical` genéricas `{plate, alert_type}` con dedup `Nats-Msg-Id=plate:alert_type:bucket`; BR-001 reescrito `alert` por evento SSE genérico, BR-004 `dedup Nats-Msg-Id` genérico; AC-005/TS-005 reescritos a `Publish alerts.critical genérico + SSE <2s`; secuencia `C->DB ST_Within` reemplazada por `Publisher alertas` genérico con nota `SPEC-003 tool ST_Within>20m`; `SSE` queda `Flow 2` genérico. Commit spec-002 desacoplado.
Auditor: architect

## 2026-08-24 — Auditoría: fleet/domain ErrValidation duplicado + mutación Validate [SPEC-002 Step1]
Severidad: alta
Hallazgo: go-backend generó fleet/domain con `var ErrValidation = errors.New("validation")` duplicado de `shared/domain ErrValidation` (dos objetos distintos) y `Validate()` mutaba `Coordinates`/`*lat/*lon` in-place (value receiver con slice header copia). `vehicle_test.go:41` espera `errors.Is(err, shared.ErrValidation)` para plate pero recibía `fleet.ErrValidation` distinto -> 400 se mapeaba a 500.
Evidencia: backend/internal/fleet/domain/zone.go:14 (pre e89a7c2), backend/internal/shared/domain/plate.go:13, backend/internal/fleet/domain/vehicle.go:60, backend/internal/fleet/domain/geo.go:40-45, reviewer ses_fcc2bcf4
Por qué falla: `errors.Is` con `errors.Join` exige identidad de centinela único (Go 1.20). Dos `ErrValidation` rompen clasificación HTTP 400 vs 500 y violan AGENTS.md "error wrap %w" y "interfaces consumer-side con errores tipados". Mutación viola pureza de dominio y BR-010 (precisión 6 dec debe aplicarse en adapter, no en Validate).
Refactor exigido: `geo.go:15 var ErrValidation = shared.ErrValidation` alias único re-exportado, `vehicle.go`/`alert.go` usan `shared.ParsePlate` ya joineado; `roundCoords` documentado como mutación controlada (test `zone_test.go:240` verifica round6) y deuda registrada para futuro `Normalized()` puro. Commit Step1 GREEN.
Auditor: reviewer | architect

## 2026-08-24 — Auditoría: fleet/domain snap 0.005 + continue silencioso + CC 15 [SPEC-002 Step1]
Severidad: media
Hallazgo: IA propuso `validateCoordinatesCountClosure` con snap `if math.Abs(delta)<0.005 { coords[n-1]=coords[0] }` (≈550m) y `validatePolygonRange` con `if len(c)<2 { continue }` silenciando coords mal dimensionadas, y `segmentsIntersect` CC 15 con 7 ramas.
Evidencia: backend/internal/fleet/domain/geo.go:76-78,86-90,154 (pre fix) y quality-auditor ses_fcc2bcf4 / ses_fcc29344
Por qué falla: BR-002 exige `first==last` exacto tras `round6` (RFC 7946) y `4..101` coords; snap 0.005 enmascara polígono no cerrado que PostGIS `ST_IsValid` rechaza luego (INSERT falla tras validar Go). `continue` deja pasar `[[ -74,4 ], [0], [-74,4]]` como válido y `ST_NPoints` luego falla en DB, no en 400. CC 15 >>10 viola `quality-auditor` hot path y TDD suite `zone_test.go:138` bowtie.
Refactor exigido: Eliminado snap (exige `coords[0]==coords[n-1]` exacto tras `roundCoords`), `len<2` ahora `return ErrCoordCount`, extraído `validateLonLat` (CC 3) y `segmentsIntersectColinear` (CC 4) para bajar `validatePolygonRange` a CC 3 y `segmentsIntersect` a CC 5. Test `zone_test.go:240` ajustado para mantener cierre exacto (seteando last = first). Commit Step1 GREEN.
Auditor: quality-auditor | reviewer | architect

## 2026-08-24 — Auditoría: migrations 0002 sin pg_advisory_lock runner + EXPLAIN faltante [SPEC-002 Step1]
Severidad: media
Hallazgo: IA documentó `pg_advisory_lock(727271)` en comentario SQL pero no implementó runner Go; `IF NOT EXISTS` solo evita "already exists" pero dos réplicas `api/consumer` concurrentes hacen `CREATE INDEX IF NOT EXISTS` sin lock → `lock_timeout`/`deadlock`. Falta test `EXPLAIN` que pruebe GIST `((geom::geometry))` evita seq scan hypertable.
Evidencia: backend/migrations/0002_fleet_zones.sql:3, backend/migrations/0001_telemetry.sql:42, grep pg_advisory 0 hits, db-auditor ses_fcc29344
Por qué falla: Plan §7 exige migrator único con `pg_advisory_lock` (ADR-0002). Sin él, rollout `migrations 0002 -> ALERTS -> api` con 5k devices puede bloquear DDL. Sin EXPLAIN, `ST_Within(telemetry.geom, zone.geom)` sin cast `::geometry` no usa `telemetry_geom_idx` y hace seq scan `O(chunks*rows)` ~216M rows.
Refactor exigido: Documentado como deuda media aprobada para Step1 (VO+DDL sin app); exigir `migrate` job único con `SELECT pg_advisory_lock` y test `EXPLAIN (FORMAT JSON) ST_Within(...::geometry ...)` en Step2 antes de cerrar pg reader. Registrado en IAUDIT y plan §12 gates.
Auditor: db-auditor | architect

## 2026-08-24 — Auditoría: fleet/pg Reader unsafe/reflect + QueryService CC 40 [SPEC-002 Step2]
Severidad: alta
Hallazgo: IA generó `pg/reader.go` con `pool any` + `reflect` + `unsafe.Pointer` para espiar `querySQL` de mock y `application/query.go` con `LastPositions` 197L CC 38 con loops `filtered`, magic `limit==2` y dead stores `hasMore`, `already`. `healthz` hardcodeaba `breaker closed/nats connected/db ok`.
Evidencia: backend/internal/fleet/adapters/pg/reader.go:19-21,50-68,71-89 (pre 6850b4f), backend/internal/fleet/application/query.go:74-270, backend/internal/fleet/adapters/http/handler.go:253-255, reviewer ses_fcbf89ab
Por qué falla: `unsafe` rompe type-safety y DIP (adapter conoce campos privados de mock), `reflect` O(n) por query, `CC 38 >>10` viola quality-auditor hot path y hace paginación O(n) en memoria en lugar de O(log n) index scan. `limit==2` hardcode rompe `limit 100` AC-001. `healthz` siempre 200 oculta breaker open.
Refactor exigido: Definida `Querier interface` con `Reader{db Querier}` + `PgxPoolAdapter`, delegación directa `reader LastPositions(limit+1)` sin filtrado memoria, helpers `validateLimit/validatePlateStr/validateCursor/roundPositions` CC<=5, `OpsProvider` para healthz real con `503 Retry-After:5`, `Round6` centralizado en `shared/domain/geo.go`. Tests `go test ./internal/fleet/...` PASS, `go vet` 0. Commit Step2 refactor.
Auditor: reviewer | quality-auditor | architect

## 2026-08-24 — Auditoría: fleet query next_cursor falso positivo + tuple OR vs tuple [SPEC-002 Step2]
Severidad: media
Hallazgo: IA implementó `if len==limit { next=EncodeCursor(last) }` generando cursor aunque `SELECT LIMIT 101` devolvió exactamente 100 sin fila 101 (no hay más páginas) → paginación infinita O(p+1). Y `WHERE (plate > $1 OR (plate=$1 AND received_at < $2))` con `OR` impide `Index Scan` sobre `(plate, received_at DESC)`; con 10k devices ×1Hz×7d=6B filas hace `BitmapOr` O(n) ~300ms vs O(log n) 5ms.
Evidencia: backend/internal/fleet/application/query.go:89-92,119-122, backend/internal/fleet/adapters/pg/reader.go:94, quality-auditor ses_fcbf709
Por qué falla: Violación corrección paginación y NFR-001 p95 <150ms. `OR` degada a seq scan hypertable; cursor falso fuerza +1 RTT por cliente.
Refactor exigido: Eliminado bloque `len==limit`, solo `len>limit` genera next con `rounded[:limit]`. Cambiado a `WHERE (plate, received_at) < ($1,$2)` tupla con `ORDER BY plate ASC, received_at DESC` y documentado índice `telemetry_plate_received_at_idx`. Test `TestQueryService_LastPositions/limit 2` ajustado a 3 posiciones para validar limit+1. `EXPLAIN` futuro debe assert `Index Scan`. Commit Step2.
Auditor: quality-auditor | db-auditor | architect

## Convenciones

- Severidad alta = task NO cerrado hasta refactor + re-auditoría.
- Cada entrada cita evidencia en git (commit/SHA previo) para que el evaluador
  pueda ver el "antes y después".
- **Dirección de la trazabilidad**: esta bitácora cita sus fuentes (ADRs,
  commits, archivo:línea); ningún documento arquitectónico (ADR, C4, specs)
  debe depender de o referenciar a IAUDIT. Únicas referencias entrantes válidas:
  `README.md` (requisito del entregable) y `AGENTS.md` (contrato de proceso).
