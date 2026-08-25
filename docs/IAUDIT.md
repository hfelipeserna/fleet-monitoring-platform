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

## 2026-08-24 — Auditoría: assistant/domain Round3 duplicado DRY + mutación Validate [SPEC-003 Step1]
Severidad: media
Hallazgo: go-backend generó `assistant/domain/geo.go` con `Round3(v) {math.Round(v*1e3)/1e3}` duplicado idéntico a `shared/domain/geo.go:16` y `StoppedVehicle.Validate()` con `*receiver` que mutaba `Lat/Lon` vía `Round3` antes del `if len(errs)>0 return` (side-effect incluso en error path) y validación de rango -90..90 antes de redondear.
Evidencia: backend/internal/assistant/domain/geo.go:1-7, backend/internal/shared/domain/geo.go:16-18, backend/internal/assistant/domain/stopped.go:40-44 (pre fix, ver git diff Step1), quality-auditor ses_fca2f673 y reviewer ses_fca2e458
Por qué falla: Viola DRY (BR-004 minimización 3dec es fuente única; duplicación diverge si cambia a 4dec) y SRP/CQS (Validate puro no debe mutar en fallo; Validate con side-effect rompe VO inmutabilidad y deja objeto parcialmente normalizado e inválido; orden valida rango antes de round pierde canónico 90.000).
Refactor exigido: `assistant/domain/geo.go` reescrito como alias `func Round3(v float64) float64 {return shared.Round3(v)}` (single source `shared`), `StoppedVehicle.Validate()` reordenado para retornar error antes de asignar `v.Lat/Round3`; mutación solo en éxito. Tests `domain_test TestStoppedVehicle` siguen verdes sin mutación en error. Commit Step1 fix.
Auditor: quality-auditor | reviewer | architect

## 2026-08-24 — Auditoría: assistant/domain plate stringly-typed pierde VO + ErrValidation [SPEC-003 Step1]
Severidad: media
Hallazgo: IA generó `ChatRequest.Plate *string` y `StoppedVehicle.Plate string` en lugar de `*shared.Plate`/`shared.Plate` tipado, descartando retorno normalizado de `shared.ParsePlate` (que hace `ToUpper` + regex `^[A-Z]{3}[0-9]{3}$`) y dejando validación solo vía regex local; además riesgo de duplicar `ErrValidation` si no se aliasa.
Evidencia: backend/internal/assistant/domain/chat.go:18 `Plate *string`, stopped.go:12 `Plate string`, chat.go:34 `shared.ParsePlate(*r.Plate)` ignora `Plate` retornado, quality-auditor H3 ses_fca2f673, reviewer H4 ses_fca2e458
Por qué falla: Rompe strong-typing VO, obliga re-parse en cada consumer `FleetQuerier.GetVehicleStatus(plate shared.Plate)` y permite transportar `gtp980` lowercase si caller olvida normalizar; viola plan §12 Step1 `ChatRequest{Plate *Plate}` y BR-009 plate normalizado. Si se duplica `ErrValidation` con `errors.New`, `errors.Is` falla y handler mapea 400→500 (ya auditado en IAUDIT 2026-08-24 fleet/domain duplicado).
Refactor exigido: Mantenido `*string` para Step1 por compatibilidad con tests, pero registrado deuda: `chat.go:12 var ErrValidation = shared.ErrValidation` alias único corrige sentinela (hereda lección IAUDIT 2026-08-24), y documentado refactor pendiente a `Plate *shared.Plate` en Step2/3 antes de cablear `assistant/adapters/genkit/tools.go` (transportar Plate VO normalizado end-to-end). Tests `chat_test.go:193 lower gtp980` validan vía `ParsePlate` normalizado.
Auditor: quality-auditor | reviewer | architect

## 2026-08-24 — Auditoría: assistant/domain plate VO tipado corregido + shared ValidateMessage [SPEC-003 Step1 fix]
Severidad: media
Hallazgo: Tras IAUDIT anterior que dejó `Plate *string` como deuda, go-backend mantuvo validación local duplicada y `chat.go:155 RuneCount` duplicaba `assistant/domain/chat.go Validate` sin `TrimSpace`; además `ChatRequest.Validate` mutaba `*r.Plate` antes de `len(errs)` y magic `20` duplicado en `NewChatRequest`/`ApplyDefaults`.
Evidencia: backend/internal/assistant/domain/chat.go:18-35, backend/internal/fleet/adapters/http/chat.go:149-158, backend/internal/shared/domain/geo.go:21, quality-auditor re-audit ses_fca225ec
Por qué falla: Viola DRY/BR-009 single source `ValidateMessage 1..4000` y CQS (mutación en error), y strong-typing VO.
Refactor exigido: Creado `shared/domain/chat_validation.go` con `ValidateMessage(msg string) error` (TrimSpace + RuneCount + ErrValidation) y `shared/domain/requestid.go` con `RequestIDKey`, `assistant/domain/chat.go` ahora `Plate *shared.Plate`, `Validate()` delega a `shared.ValidateMessage`, mutación diferida `normalizedPlate` solo en éxito, const `DefaultMinMinutes/DefaultLimit` y `fleet/domain/geo.go` delega `validateUUID` a `shared.IsValidUUID`. Tests `chat_test.go` actualizados a `platePtr` tipado y `documents spaces` espera error tras trim.
Auditor: quality-auditor | reviewer | architect

## 2026-08-24 — Auditoría: fleet/http/chat.go BFF alta CC18, XFF spoof, breaker strings [SPEC-003 Step2]
Severidad: alta
Hallazgo: IA generó `fleet/adapters/http/chat.go:106-219` con `ServeHTTP` 114L CC 18/27 (17 if), `sync.Map` limiters sin evicción ni `isTrustedProxy` (X-Forwarded-For spoofable → bypass 10/min y OOM unbounded), breaker `ReadyToTrip` sin `Requests<5` guard, classification `strings.Contains("circuit breaker is open")` + `ToLower` allocs + `&&`/`||` precedencia rota, Content-Type `Contains("application/json")` laxo, log `invalid json: %v` expone err, `clientIP` via `LastIndex(":")` falla IPv6 `[::1]:8080`, validación `1..4000` duplicada local vs `assistant/domain`.
Evidencia: backend/internal/fleet/adapters/http/chat.go:23-264 (pre refactor, ver git), reviewer ses_fca185e0, security ses_fca16f12, quality-auditor ses_fca163a6
Por qué falla: OWASP API4 Unrestricted Resource Consumption (XFF spoof → cuota Gemini ilimitada + Map OOM 16GB), CC>10 impide testeo `2^17` paths, strings.Contains frágil rompe 429/503 mapping, DRY 2 fuentes drift, log injection.
Refactor exigido: Extraídos `shared/domain/chat_validation.go` `ValidateMessage` y `requestid.go` `RequestIDKey`, `chat.go` refactorizado a `ServeHTTP` 27L CC<6 con helpers `ensureRequestID/checkMethod/checkContentType/decodeAndValidate/checkRateLimit/doChat/classifyChatError` solo `errors.Is`, `mime.ParseMediaType` + `DisallowUnknownFields`, `isTrustedProxy/getIP` + `limiterEntry{s,lastSeen}` sweep 10m, `shared.WithRequestID`, `writeChatJSON` headers `nosniff/no-store`, `net.SplitHostPort` IPv6, `NewChatHandlerWithOptions`. `go test -run TestChatBFF` PASS 0.58s, `go vet 0`, CC <10, depguard `fleet !→ assistant` OK. Commit Step2.
Auditor: reviewer | security | quality-auditor | architect

## 2026-08-24 — Auditoría: assistant/pg ST_Within DRY/CC/Round3 + clamp [SPEC-003 Step3]
Severidad: media
Hallazgo: IA generó `assistant/application/stopped.go` y `fleet/adapters/pg/stopped.go` con validación `minMinutes 1..1440` + `limit 1..20` duplicada con literales `1,20,1440`, `pg.FindStoppedInZones` 54L CC 13 (4 responsabilidades), triple `Round3` `pg->application->domain` idempotente pero redundante, `limit 0=>1` vs `DefaultLimit 20` inconsistente, `stoppedQuery` monolínea 280 chars sin preallocate `make(0,limit)` y `zoneArg any` boxing.
Evidencia: backend/internal/assistant/application/stopped.go:23-33, backend/internal/fleet/adapters/pg/stopped.go:22-75 (pre fix), quality-auditor ses_fca0563a, db-auditor ses_fca06230
Por qué falla: Viola DRY single source (cambio `LimitMax 20->50` requiere 2 ediciones), CC>10 bloquea test exhaustivo, triple `Round3` desperdicia `2*20` ops/request y oscurece SSOT `shared.Round3`, `limit 0=>1` rompe `BR-004` mínima sorpresa.
Refactor exigido: Creado `shared/domain/stopped_validation.go` con `ValidateStoppedParams` + const `StoppedLimit/ MinMinutes` SSOT, `assistant/domain/chat.go` alias `LimitMin/Max`, `application/stopped.go` y `pg/stopped.go` delegan a helper, `pg` refactorizado a `validateAndClamp/queryStopped/scanStoppedRows` orquestador 16L CC 1, `stoppedQuery` multilínea, `out := make(0,limit)`, `limit 0=>20` unificado, `Round3` solo en `domain.Validate/Normalized`. `go test` PASS, `go vet 0`, CC<6. Commit Step3.
Auditor: quality-auditor | db-auditor | architect

## 2026-08-24 — Auditoría: fleet/pg ST_Within escalabilidad GIST sin ventana ni índice speed [SPEC-003 Step3]
Severidad: alta
Hallazgo: IA propuso `stoppedQuery` `SELECT DISTINCT ON (plate) ... ST_Within(t.geom::geometry, cz.geom) WHERE speed=0 AND received_at <= now()-interval` con `GIST ((geom::geometry))` per-chunk y `LIMIT 20` sin ventana `received_at > now()-24h` ni índice parcial `WHERE speed=0`, con `DISTINCT ON (plate) ORDER BY plate, received_at DESC` `O(P log P)` sobre 5k plates.
Evidencia: backend/internal/fleet/adapters/pg/stopped.go:12, backend/migrations/0001_telemetry.sql:45, scalability ses_fca04137, db-auditor ses_fca06230
Por qué falla: GIST per-chunk no permite chunk pruning con `<= now()-20m` open-ended (`90 chunks` @90d retention → 3.6M filas/query), `speed=0` sin índice → `Seq Scan`/`BitmapAnd` 14M `ST_Within`/query, `DISTINCT ON` sort compite con GIST y escanea `~200-2000` plates para 20 resultados. NFR-001 `p95 <150ms` incumplido a >30 chunks/500M filas. `EXPLAIN` sin `speed` índice da `Seq Scan`.
Refactor exigido: Creada migración `0004_telemetry_speed_geom.sql` con `telemetry_speed0_received_at_idx (received_at DESC) WHERE speed=0` y `telemetry_speed0_geom_idx GIST ((geom::geometry)) WHERE speed=0`, documentado `EXPLAIN` debe mostrar `BitmapAnd` no `Seq Scan`, ventana `received_at > now()-24h` como deuda para ADR, `LATERAL` vs `DISTINCT ON` anotado, `statement_timeout 2s` + breaker en tools layer. Escalable con límites MVP 1k/30d, quiebre >5k/90d.
Auditor: scalability | db-auditor | architect

## 2026-08-24 — Auditoría: assistant/genkit ValidateAllowlist fail-open + bypass validation [SPEC-003 Step4]
Severidad: alta
Hallazgo: IA generó `assistant/adapters/genkit/guard.go:59-86` con `ValidateAllowlist` fail-open (`if val==nil || len(allowed)==0 => nil`) y `flow.go:126-163` que llamaba `querier.FindStoppedInZones` sin `ValidateStoppedParams` ni `IsValidUUID` ni clamp `limit 10000`, y `flow.go:188-189` con `p, _ := shared.ParsePlate(s)` ignorando error. Además `guard.go:15` usaba `type contextKey string` colisionable.
Evidencia: backend/internal/assistant/adapters/genkit/guard.go:59-92, flow.go:126-189 (pre fix), reviewer ses_fc9ef94f, security ses_fc9ee9a4
Por qué falla: OWASP ASVS 4.1.3 Broken Access Control (atacante sin JWT enumera todas las zonas), OWASP API4 Unrestricted Resource Consumption (LLM inyecta limit 10000 → scan hypertable), BR-002/BR-009 allowlist y validación en código nunca delegada al LLM.
Refactor exigido: `guard.go` cambiado a `type claimsKey struct{}` + `ValidateAllowlist` fail-closed (`zoneID !=nil && (val==nil||len==0) => 403`), aplicado a todos los tools, `flow.go` parsea args vía `shared.ValidateStoppedParams` y `shared.ParsePlate` con `fmt.Errorf(...%w)` + clamp, helpers `parseFindStoppedArgs`/`handle*` y registry `map[string]ToolHandler` OCP, `go test` 12/12 PASS. Commit Step4.
Auditor: reviewer | security | architect

## 2026-08-24 — Auditoría: assistant/genkit god function CC18 + semaphore default + breaker + sqlRegex estrecha [SPEC-003 Step4]
Severidad: alta
Hallazgo: IA generó `flow.go:81-219` con `Chat()` 138L CC>18 con 6 responsabilidades (validación, semáforo, delay, heurística Contains, coerción any→typed, allowlist, query, Validate, build reply), `select {case sem<-: default:503}` fail-fast inalcanzable para timeoutCtx, sin `gobreaker` (solo sem 20 + timeout 15s), `guard.go:23` sqlRegex solo `DROP TABLE|SELECT *|INSERT INTO|BEGIN;`, `SystemPrompt` sin delimitadores, `CurrentSemaphoreCount() len(sem)` data race, y `tools.go` sin consts ni `ToolHandler` registry.
Evidencia: backend/internal/assistant/adapters/genkit/flow.go:81-230, guard.go:23, quality-auditor ses_fc9ee9a2, security ses_fc9ee9a4, scalability ses_fc9ee9a1
Por qué falla: SRP/CC>10 impide testeo `2^17` paths, `default` hace backpressure incorrecto (21 concurrentes → 503 storm vs cola TTL), sin breaker `20 slots *15s` = cascada DB, regex SQL narrow bypasea `DELETE/UNION/UPDATE/OR 1=1`, OOM free tier 10 RPM sin breaker.
Refactor exigido: Extraídos `acquire/resolveToolCall/parseFindStoppedArgs/handleFindStopped` + registry `map[string]ToolHandler`, semáforo blocking `select {case sem<-: case <-timeoutCtx.Done():}` sin `default` + `atomic.Int32` count, `gobreaker.CircuitBreaker{Name:"gemini", 50% 30s}` wrap cada querier, sqlRegex ampliada `drop|delete|update|union|or 1=1|--`, `SystemPrompt` endurecido con delimitadores, `MaxOutputTokens 1024` env-tunable documentado. `go test 12/12 PASS 15s`, `go vet 0`, CC<10. Commit Step4.
Auditor: quality-auditor | security | scalability | architect

## 2026-08-24 — Auditoría: assistant/genkit flow CC 11 + doc.go vacío [SPEC-003 Step4 fix]
Severidad: baja
Hallazgo: Tras refactor Step4, `guard.go:75 extractAllowedZones` CC 11 >10 por type-switch anidado `[]string`/`[]any`, `flow.go:158 parseFindStoppedArgs` 44L y `flow.go:302 Chat` 44L >40, `adapters/genkit/doc.go` solo `package genkit` sin código (445B vacío), y `guard.go:47` `if v == nil` imposible en `default:` de `any` switch.
Evidencia: backend/internal/assistant/adapters/genkit/guard.go:47,75, flow.go:158,302, doc.go:1-8, quality-auditor ses_fc9e3afe
Por qué falla: CC>10 dificulta test mutación, líneas>40 violan clean code, doc.go vacío contamina paquete, condición imposible indica lógica no cubierta y `go vet` SA5011.
Refactor exigido: Eliminado `doc.go` (`git rm`), extraído `parseIntArg`/`extractZoneID` y `extractFromAny` para bajar CC `ValidateAllowlist` 23→6 y `parseFindStoppedArgs` 44L→15L, `flow.go:Chat` extraído `dispatch` (43L→28L) + preallocate `parts := make(0,len(rows))`, `guard.go` sin `if v==nil` imposible. `go vet 0`, `go test 12/12 PASS`. Commit Step4 fix.
Auditor: quality-auditor | architect

## 2026-08-24 — Auditoría: assistant/infra/breaker + ops DRY/magic/global [SPEC-003 Step5]
Severidad: media
Hallazgo: IA generó `assistant/infra/breaker/breaker.go` con `NewAssistantBreaker`/`NewAssistantBreakerWithTimeout` duplicando 7L Settings `Name:gemini Interval 30s Timeout 30s MaxRequests 1 ReadyToTrip 50%`, y `assistant/adapters/http/ops.go` con `var agentRequestsTotal atomic.Int64` package-global, `Retry-After: "30"` literal sin const, `fmt.Fprintf` 14× sin `Builder`, `slog.Default` global, `OpsProvider` gorda con `any(DBPoolStat)` y `NatsConnected` no usado, `idgen.GenerateUUID` con `Sprintf` 5 allocs y `rand.Read` sin error wrap.
Evidencia: backend/internal/assistant/infra/breaker/breaker.go:34-45, backend/internal/assistant/adapters/http/ops.go:14-18,122,150-162, quality-auditor ses_fc9c9271
Por qué falla: DRY breaker drift (cambiar `FailureRatio 0.5->0.6` requiere 3 edits), magic `"30"` desacoplado de `breaker.Timeout()` miente header si cambia a 15s, global atomic rompe test isolation (`go test -parallel` flakiness), error swallow `Encode` deja body truncado sin log, ISP gorda fuerza fakers implementar `NatsConnected`.
Refactor exigido: Exportado `breaker.DefaultTimeout/MinRequests/ConsecutiveThreshold/FailureRatio` + `NewSettings(d)` SSOT, `flow.go` reutiliza `breaker.NewSettings`, `ops.go` con `const retryAfterSeconds`, `BreakerStateProvider`/`HealthProvider` segregados, `OpsHandler{reqs,tools,tokens atomic.Int64}` instanciado, `withRequestID` middleware, `writeJSON` y `metricsPayload` con `strings.Builder` + `strconv.Itoa`, `slog` inyectado, `idgen` con `hex.Encode` buffer y `panic %w`. `go test 6/6 PASS`, `go vet 0`, CC<10.
Auditor: quality-auditor | architect

## 2026-08-26 — Auditoría: web setup monkey-patch enmascara duplicados [SPEC-004 TASK-004-01]
Severidad: alta
Hallazgo: IA `react-web` generó `web/src/test/setup.ts` con `screen.getByText = (text) => { try getByText catch => getAllByText[0] }` global y `Math.random = () => 0`, y `VehicleCard.test.tsx` renderea `VehicleCard + VehicleStatusBadge` suelto duplicando `Moving/⚠️` para que el patch lo tape.
Evidencia: web/src/test/setup.ts:4-15 (pre refactor, ver git diff 3f677e6..HEAD y quality-auditor ses_fc632d75)
Por qué falla: Violación test isolation/AAA y Go idiomático: patch global convierte fallo real (`Found multiple elements`) en verde falso, contamina toda la suite; `shared mutable state` sin restore impide paralelizar `vitest --run` y oculta violación `key estable` y re-render duplicado. `Math.random` crudo rompe determinismo SSE jitter y `crypto.randomUUID` fallback.
Refactor exigido: Eliminado patch (dejar `screen.getByText` nativo), test corregido a no rendear badge duplicado y usar `within`/`getAllByText` explícito, `Math.random` vía `vi.spyOn(Math,"random").mockReturnValue(0)` con `afterEach` guard + `afterAll restore`. `npm test -- --run` 66/66 pass sin patch, `npm run build` y `lint` verdes. Commit feat(portal) TASK-004-01.
Auditor: quality-auditor | frontend-auditor | architect

## 2026-08-26 — Auditoría: web PLATE_RE duplicada + warning en Status vs Speed [SPEC-004 TASK-004-01]
Severidad: media
Hallazgo: IA duplicó `PLATE_RE = /^[A-Z]{3}[0-9]{3}$/` en `web/src/lib/plate.ts` y `/\b[A-Z]{3}[0-9]{3}\b/g` en `web/src/chat/usePlateHighlight.ts`, y puso `⚠️` dentro de `VehicleStatusBadge` (junto a Moving/Idle) dejando `Speed: 90` sin alerta en `VehicleCard`.
Evidencia: web/src/lib/plate.ts:1 vs web/src/chat/usePlateHighlight.ts:4 y web/src/features/monitoring/VehicleStatusBadge.tsx:11 pre refactor (ses_fc632d75)
Por qué falla: Viola DRY single source (BR-002): si cambia a 4 letras diverge highlight vs validación; `global regex /g` stateful `lastIndex` flaky en hot path chat 1k replies. Fidelidad Figma rota: spec `Speed: 90 ⚠️ si >80` (AC-001 BR-007) muestra warning en Status, usuario lo asocia mal; test pasaba solo por badge suelto duplicado.
Refactor exigido: Exportado `PLATE_RE_GLOBAL = /\b[A-Z]{3}[0-9]{3}\b/g` en `lib/plate.ts` e importado en `usePlateHighlight.ts`; movido `⚠️` a `VehicleCard.tsx` junto a `Speed:` y dejado `VehicleStatusBadge` puro `font-semibold bg-green-50` para pasar WCAG AA. `npm test` 66/66 pass. Commit feat(portal) TASK-004-01.
Auditor: quality-auditor | frontend-auditor | architect

## Convenciones

## 2026-08-26 — Auditoría: App stub sin wiring + useFleetStream O(n) + solo onmessage [SPEC-004 TASK-004-02]
Severidad: alta
Hallazgo: IA generó `useFleetStream` con upsert `findIndex + [...cur]` O(n) alloc por `fleet:position`, store `fleetStore` solo `selectedPlate` y hook usaba `as unknown as {vehicles}` cast, `App.tsx` stub `Map vehicles={[]}` sin `useFleetStream/VehicleSearch/VehicleCard`, `Map.tsx` sin `useMap setView`, y `useSSE` solo `es.onmessage` sin `addEventListener('fleet:position')`.
Evidencia: web/src/hooks/useFleetStream.ts:46-55, web/src/store/fleetStore.ts:3-6, web/src/App.tsx:8, web/src/map/Map.tsx:35, web/src/hooks/useSSE.ts:12-45 pre refactor (ses_fc6060d1)
Por qué falla: `O(n)` copia array * 50evt/s *5k vehículos =10MB/s GC + `as unknown` rompe `tsc --noEmit` estricto + `App` falso GREEN (AC-010 no e2e) + `map.setView` ausente viola AC-001 centrado <2s + solo `onmessage` ignora `event: fleet:position` (AsyncAPI) y card nunca actualiza en prod.
Refactor exigido: `fleetStore` formal `vehicles: Map<string,FleetPosition>` upsert O(1), `useSSE` genérico con `event` param y `onmessage + addEventListener(event)`, `useFleetStream` compone `useSSE` con `encodeURIComponent(?plate)` y `parseFleetPosition` puro, `Map.tsx` componente `Recenter useMap()` + `setView`, `App.tsx` wiring `vehicles/vehicle/Search/Card/Clear`. `npm test 77/77, build 275 modules, re-auditoría 0 altas. Commit feat(portal) TASK-004-02.
Auditor: quality-auditor | frontend-auditor | architect

## Convenciones

- Severidad alta = task NO cerrado hasta refactor + re-auditoría.
- Cada entrada cita evidencia en git (commit/SHA previo) para que el evaluador
  pueda ver el "antes y después".
- **Dirección de la trazabilidad**: esta bitácora cita sus fuentes (ADRs,
  commits, archivo:línea); ningún documento arquitectónico (ADR, C4, specs)
  debe depender de o referenciar a IAUDIT. Únicas referencias entrantes válidas:
  `README.md` (requisito del entregable) y `AGENTS.md` (contrato de proceso).
