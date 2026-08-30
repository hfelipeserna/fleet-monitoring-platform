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

## 2026-08-26 — Auditoría: AlertsPanel dedup O(n²) + CC keepalive [SPEC-004 TASK-004-03a]
Severidad: alta
Hallazgo: IA generó `useAlertsSSE` con `setAlerts(prev=>{ if(prev.some(k)) return prev; return [...prev,data] })` O(n) scan + copia por `alert:critical` sin cap, y `isKeepAlive` + `JSON.parse` con 5 chequeos redundantes `raw === ":ping"` / `type==="ping"` / `trimmed===":ping"` / `data===":ping"` con try.
Evidencia: web/src/hooks/useAlertsSSE.ts:40-44 y 22-36 pre refactor (ses_fc5e602)
Por qué falla: `O(n²)` total: 1000 alertas retenidas * 1000 scan =1M comparaciones + 500k copias → fuga memoria navegador 5k flota (NFR-002), sin cap DOM crece infinito. CC>10 impide test `2^17` paths y `:ping` coment line SSE (BR-005) nunca dispara onmessage.
Refactor exigido: Extraído `lib/alert.ts` `getDedupKey/isValidAlert/isKeepAlive/parseAlert` single source CC<6, `useAlertsSSE` con `Set O(1) + MAX_ALERTS 100 cap` y `useSSE` filtra keepalive en dueño protocolo. `AlertsPanel` `role=log aria-live` + `role=list/listitem` + `<time dateTime>`. `setup.ts` limitado a jest-dom sin monkey-patch. `82/82 pass` re-auditoría 0 altas.
Auditor: quality-auditor | frontend-auditor | architect

- Severidad alta = task NO cerrado hasta refactor + re-auditoría.
## 2026-08-26 — Auditoría: Chat useChatApi race leak + BottomPanelShell DRY [SPEC-004 TASK-004-03b]
Severidad: alta
Hallazgo: IA generó `chat/useChatApi` con `AbortController + setTimeout 15s` sobrescribiendo `abortRef/timeoutRef` sin abortar/clear previo, `loading` toggle cruzado y `BottomPanelShell` duplicado: `ChatTab` y `AlertsPanel` copiaban `h-[280px] + role=log` y `App` montaba `ChatWidget` suelto fuera del panel fijo, doble `role=log` y doble scroll `h-[280px] + .log max-height 400px`.
Evidencia: web/src/chat/useChatApi.ts:24-27 pre refactor, web/src/features/monitoring/ChatTab.tsx:10-20 y AlertsPanel.tsx:24-39 duplicado, web/src/App.tsx:44 <ChatWidget/> suelto (ses_fc5c3d22)
Por qué falla: `2º sendMessage` overwrites `abortRef`/`timeoutRef` → `finally` de 1ª limpia timeout de 2ª (leak) y `loading` queda idle con request pending; `11 req/min` BR-011 reproduce 100%. DRY shell obliga 2 edits para cambiar `lg:h-[340px]`; doble `role=log` anuncia form 2 veces y doble scroll rompe `overflow-y-auto`.
Refactor exigido: `useChatApi` guarda `prevCtrl/prevT` y abort/clear antes de re-asignar, `if(loading) return` + cleanup condicional `if(current===id)`, helpers `buildRequest/fetchWithFallbackSignal/mapStatusToError` CC<5; `BottomPanelShell` centraliza `h-[280px] lg:h-[340px] overflow-y-auto flex flex-col` + `portalStore` hidden, `ChatTab/AlertsPanel` lo usan, `App` reemplaza Widget suelto por `ChatTab`, `role=log` solo en `MessageList`, `.log flex:1`. `87/87 pass` re-auditoría 0 altas.
Auditor: quality-auditor | frontend-auditor | architect

## 2026-08-26 — Auditoría: Zones fetch duplicado + JSON.stringify + 100vh [SPEC-004 TASK-004-05]
Severidad: media
Hallazgo: IA generó `ZonesList` y `Map` con `fetch GET /api/zones` duplicado (BR-004 única fuente rota, 2× p95 200ms + race setGeoJson), `ZoneDrawControl` con `JSON.stringify(first) !== JSON.stringify(last)` O(k) alloc frágil a `1 vs 1.0`, doble escritura `store + onDraftChange` y `catch(()=>undefined)` traga Abort/400, tipos `DraftPolygon` triplicados con `unknown` y `Map 100vh` overflow.
Evidencia: web/src/features/zones/ZonesList.tsx:19-32 + web/src/map/Map.tsx:54-65, ZoneDrawControl.tsx:61-67, store/portalStore.ts:9 pre refactor (ses_fc580)
Por qué falla: `2× fetch` rompe contrato canónico `GET /api/zones` mapa+agente, `JSON.stringify` falla con EPSG:4326 tolerancia y `O(k)` alloc, `as unknown` anula `tsc`, `100vh` + header desborda grid y doble `data-testid zones-list`.
Refactor exigido: `useZones` hook único con `AbortController + r.ok`, `Map` solo por prop `zones`, `types.ts` centraliza `DraftPolygon/validatePolygon` pura, `isKeepAlive` + `getDedupKey` single source, `Map h-full w-full` y tokens `ZONES_PANEL_FIXED`. `98/98 pass` re-auditoría 0 medias bloqueantes (solo 3 bajas factor 2 fetch).
Auditor: quality-auditor | frontend-auditor | architect

## 2026-08-26 — Auditoría: Modales huérfanos sin wiring + dialog sin focus trap [SPEC-004 TASK-004-06]
Severidad: alta
Hallazgo: IA generó `CreateZoneModal` y `EditZoneModal` con `role=dialog` pero nunca montados en `App/ZonesList` (botón `Create zone` sin `onClick`, filas sin `onDoubleClick`), sin focus-trap/Esc/autofocus, `Accept` siempre habilitado sin `name.trim()`, parsing `400/409` duplicado ×3 y `name` stale al reabrir.
Evidencia: web/src/features/zones/CreateZoneModal.tsx:6-72 + EditZoneModal.tsx:5-114 + App.tsx:123-126 pre refactor (ses_fc562f)
Por qué falla: Feature GREEN falso (tests aislados pasan pero portal no crea/edita zonas, AC-007/009 huérfanos), WCAG 2.4.3 focus escapa al mapa/leaflet, DRY ×3 drift `details/error/message`, UX click → flash 400.
Refactor exigido: `App` eleva `createOpen/editZone` + monta ambos modales con `onClose/onCreated/onRenamed/onDeleted → useZones refetch`, `lib/useDialogFocus` hook trap Tab/Esc + restore focus, `isAcceptDisabled=!name.trim()||!draft`, `api.ts parseZoneApiError/zoneApiUrl` DRY, `useEffect reset name/error` al abrir. `109/109 pass` re-auditoría 0 altas/medias.
Auditor: quality-auditor | frontend-auditor | architect

## 2026-08-26 — Auditoría: reviewer clean architecture + error wrap + secrets [SPEC-004]
Severidad: media
Hallazgo: IA dejó default DSN `postgres://fleet:fleet@localhost` en `consumer/bootstrap.go:41` y `backend/.golangci.yml` sin `pg_advisory_lock` runner; `gitleaks.toml` allowlist incompleta para `AQ.`/`sk-`; `specs/**/plan.md` referenciaban IAUDIT rompiendo trazabilidad unidireccional (ADRs deben ser fuente).
Evidencia: backend/cmd/consumer/bootstrap.go:41, docs/specs/SPEC-003/plan.md:318, docs/specs/SPEC-004/plan.md:277, reviewer ses_fc01814
Por qué falla: Viola AGENTS.md `domain → application → adapters → infra` y skill `ai-audit` trazabilidad unidireccional; default DSN expone password dummy en imagen si env no seteada.
Refactor exigido: Cambiar default DSN a `""` fail-closed y documentar rotación `change-me-local` vía secrets manager; reescribir `plan.md` referencias hacia ADRs/commits en vez de IAUDIT; mantener `gitleaks` con reglas `AQ\.` y `sk-`. Commit 808132e (no bloqueante, deuda media).
Auditor: reviewer | architect

## 2026-08-26 — Auditoría: security 3 altas (secretos, GPS sin auth, ingesta spoofing) [SPEC-004]
Severidad: alta
Hallazgo: IA propuso `.env` con `GEMINI_API_KEY` real en disco + `GET /api/fleet/positions` sin JWT + `POST /v1/telemetry` sin HMAC; TLS `sslmode=disable` y `nats://` sin TLS.
Evidencia: .env:22 `GEMINI_API_KEY=AQ...`, backend/cmd/api/bootstrap.go:208 mux sin auth, backend/internal/telemetry/adapters/http/ingest.go:19 sin token, docker-compose.yml:28 `sslmode=disable`, security ses_fc016769
Por qué falla: OWASP A07 Secrets, A01 Broken Access Control (GPS PII), A02 Crypto Failure; ingesta sin auth permite spoofing y alertas falsas.
Refactor exigido: Para MVP local deuda aceptada (AGENTS.md 16GB, ADR-0009 riesgo anónimo documentado, 127.0.0.1 binding, `GENKIT_ENV=dev` guard); prod exige JWT `ValidateAllowlist` + `X-Device-Token` HMAC + `sslmode=require` + NATS TLS. Rotar keys y borrar `.env.save`. No bloquea commit demo.
Auditor: security | architect

## 2026-08-26 — Auditoría: scalability 3k msg/s con NATS 5GB y single writer [SPEC-002]
Severidad: media
Hallazgo: IA dimensionó NATS `MAX_BYTES 5GB DiscardOld 24h` para 3k msg/s → retiene 1.15h luego `DiscardOld` pérdida silenciosa; `consumer` single replica + `telemetry_dedup` sin retention 8GB/día bloat; falta `continuous aggregate` y `compression_policy`.
Evidencia: infra/nats/stream.go:7 `Duplicates 2m`, docker-compose.yml:175 `JETSTREAM_MAX_BYTES 5368709120`, backend/internal/fleet/adapters/pg/reader.go:90 `COUNT(*)`, scalability ses_fc014a6c
Por qué falla: A 3k msg/s (15k disp @5s) stream 103GB/día, DB 38GB/día raw → 27GB/semana comprimido; sin `add_compression_policy(7d)` y `add_retention_policy(90d)` crece O(GB/día) y `GetFleetSummary COUNT(*)` escanea chunks.
Refactor exigido: Para MVP 67-200 msg/s (1k disp) mononodo correcto; para 3k escalar a `MAX_BYTES 50GB`, `NATS_MAX_PENDING 4096`, `consumer replicas 2`, `dedup retention 30d`, `compression 7d`, `continuous aggregate last_position`. Documentado como deuda escalabilidad, no bloquea demo.
Auditor: scalability | architect

## 2026-08-26 — Auditoría: db keyset OR vs tupla + COUNT(*) hot path [SPEC-002]
Severidad: alta
Hallazgo: IA implementó paginación `WHERE (plate > $1 OR (plate=$1 AND received_at < $2))` con `OR` que impide `Index Scan` en `(plate, received_at DESC)` → `BitmapOr` O(n) 300ms vs 5ms; y `GetFleetSummary` con `COUNT(*) WHERE received_at > now()-5m` sin índice BRIN escanea hypertable por request.
Evidencia: backend/internal/fleet/adapters/pg/reader.go:96, backend/cmd/agent/bootstrap.go:249 `SELECT count(*)`, db-auditor ses_fc0126be
Por qué falla: Violación NFR-001 p95 <150ms y Timescale best practice; `COUNT(*)` en hot path chat amplifica con 10 RPM.
Refactor exigido: Cambiar a `WHERE (plate, received_at) < ($1,$2)` tupla con `ORDER BY plate ASC, received_at DESC`; reemplazar `COUNT(*)` por `continuous aggregate telemetry_5m` o cache TTL 30s; añadir `telemetry_received_at_brin`. Deuda media/alta aprobada para MVP, exigir `db-auditor` gate antes de cierre SPEC-002/003.
Auditor: db-auditor | architect

## 2026-08-26 — Auditoría: quality hot path leak + dedup contención [SPEC-004]
Severidad: alta
Hallazgo: IA lanzó `go d.StartCleanup(Background)` nunca cancelable (leak 1 goroutine/detector) y `isDupLocked` escanea `dedup` map 10k entries bajo `mu.Lock()` con check-then-act publish fuera de lock → race duplicados y O(n) contención a 1000 msgs/s.
Evidencia: backend/internal/fleet/application/alert.go:64,98-110, web/src/store/fleetStore.ts:25 `new Map` O(N) por SSE, quality-auditor ses_fc00fef3
Por qué falla: Resource leak y contención hot path NFR-002; `upsertVehicle` copia `Map` 10k*400/s =4M copias/s.
Refactor exigido: Eliminar `go` del constructor, exigir `ctx` externo + `Stop()` con `goleak`; mover evicción a `sync.Map` + TTL heap O(log n) y dedup atómico `check+insert` bajo lock antes de publish; frontend usar `immer` o `Map.set` O(1). Deuda alta bloquea done si flota >1k.
Auditor: quality-auditor | architect

## 2026-08-26 — Auditoría: frontend bundle + panel fijo + re-render [SPEC-004]
Severidad: alta
Hallazgo: IA generó bundle monolítico 765k sin `lazy`/`manualChunks`, `ChatWidget.module.css` con `overflow-y: visible` rompe `h-[280px] lg:h-[340px] overflow-y-auto`, y `fleetStore` con `new Map` + `Array.from` remonta 5k markers por cada `fleet:position`.
Evidencia: web/vite.config.ts, web/src/chat/ChatWidget.module.css:2, web/src/store/fleetStore.ts:25, frontend-auditor ses_fc00e133
Por qué falla: Viola Figma fidelidad BR-015 y performance NFR-002; TTI >3s móvil, panel crece infinito, main thread bloqueado.
Refactor exigido: `React.lazy` Map + `manualChunks` leaflet/geoman/markdown, `ChatWidget.module.css` a `overflow-y-auto max-h-[220px]`, `useFleetStore` selectores granulares + `React.memo` Map. Deuda alta para flota >500.
Auditor: frontend-auditor | architect

## 2026-08-26 — Auditoría: alerts SSE fan-out + JSON snake_case + zone_name [SPEC-002/004]
Severidad: alta
Hallazgo: IA dejó `AlertDetector` sin cablear en `cmd/consumer` (ALERTS 0 msgs), `Alert` con `json:"AlertType"` PascalCase vs frontend `alert_type`, y `AlertSubscriber` con `Durable("api-sse-alerts")` compartido → load-balance no broadcast, más `translate` para `zone_exit` sin `zone_name`.
Evidencia: backend/cmd/consumer/main.go:37 sin detector, backend/internal/fleet/domain/alert.go:16 sin tags, backend/internal/fleet/adapters/nats/subscriber.go:40 `Durable`, web/src/features/monitoring/AlertsPanel.tsx:14, auditorías 2026-08-26 manual
Por qué falla: `speeding_on/off` y `zone_enter/exit` nunca llegaban a `AlertsPanel` (SSE vacío) y `HYU456 entra en zona` sin nombre; viola SPEC-002 FR-005/006 y FR-011.
Refactor exigido: Cableado `consumer.WithAlertProcessor(detector)` con `jsPublisher` + `PGZoneResolver` + breaker, `Alert` con `json:"alert_type"` etc. + `ZoneName`, `subscriber` efímero sin Durable (broadcast), `AlertsPanel` con `bg-red-100`/`bg-green-100` y `zone_exit` con nombre. Verificado `ALERTS:5` con `zone_name:"Rafael Uribe"` y SSE `id:13 speeding_on`. Commit 808132e.
Auditor: architect | reviewer

## 2026-08-27 — Auditoría: mobile plate PLATE_RE_GLOBAL stateful + dead code [SPEC-005 TASK-005-01]
Severidad: media
Hallazgo: mobile-expo generó `PLATE_RE_GLOBAL = /\b[A-Z]{3}[0-9]{3}\b/g` exportado sin uso, con flag `/g` stateful y `\b` que permite substring match dentro de texto libre. La IA lo propuso como "útil para buscar placas en chat" copiado de web sin necesidad en TASK-005-01.
Evidencia: mobile/src/lib/plate.ts:2 (pre commit 64bcc64, ver git diff) `export const PLATE_RE_GLOBAL = /\b[A-Z]{3}[0-9]{3}\b/g;` con 0 usages en `grep -R mobile/src`
Por qué falla: Código muerto viola AGENTS.md cero código muerto y SSOT `PLATE_RE=/^[A-Z]{3}[0-9]{3}$/` FR-001 BR-001. Flag `/g` con `RegExp.test()` es stateful (`lastIndex`) y produce flaky true→false alternado si se reutiliza en hot path batch 500 (250 falsos negativos). Patrón `\b` contradice anclas `^$` y permitiría validar `ACF3567` como `ACF356` interior, rompiendo BR-001. En web se evita exportando `PLATE_RE_SOURCE` + `new RegExp` por llamada.
Refactor exigido: Eliminado export en commit siguiente (solo `PLATE_RE` + `normalizePlate` + `isValidPlate` SSOT). Si en futuro se necesita extracción, crear `createPlateGlobal()` sin singleton stateful con test dedicado. `npm test 20/20 PASS`, `tsc --noEmit` OK. Commit feat(mobile) TASK-005-01.
Auditor: reviewer | quality-auditor | architect

## 2026-08-27 — Auditoría: mobile PlateInput triple normalize + Props genérico [SPEC-005 TASK-005-01]
Severidad: media
Hallazgo: IA generó `PlateInput` con `type Props` genérico y triple normalización por keystroke: `onChangeText=>normalize`, `valid=isValidPlate(plate)` renormaliza, `onPress=>onConnect(normalizePlate(plate))` renormaliza estado ya canónico. Propuesta inicial sin `useMemo` ni invariante documentado.
Evidencia: mobile/src/components/PlateInput.tsx:5 `type Props`, :11 `const valid=isValidPlate(plate)`, :18 `onChangeText={(t)=>setPlate(normalizePlate(t))}`, :31 `onConnect(normalizePlate(plate))` (pre fix)
Por qué falla: `Props` colisiona en barrel exports y dificulta grep/a11y (naming inglés descriptivo AGENTS.md). Triple normalize O(3*6) despreciable en Step1 pero patrón se replica a hot path `flushPending 500` y `useTelemetryGenerator 5s` y oculta invariante "state ya es canónico". No memoizado viola DRY y SOLID leve; web vs mobile `isValidPlate` diverge (mobile normaliza dentro, web exige caller) rompe LSP si se promueve a shared/domain.
Refactor exigido: Renombrado `PlateInputProps`, cambiado `onPress` a `onConnect(plate)` sin segundo normalize (state ya normalizado), documentado invariante. `npm test 20/20 PASS`. Costo bajo pero forzado para evitar drift en Steps 6-7.
Auditor: quality-auditor | reviewer | architect

## 2026-08-27 — Auditoría: mobile getBaseUrl Function eval + occurred_at number [SPEC-005 TASK-005-02]
Severidad: media
Hallazgo: IA generó `lib/api.ts` con `Function('return process.env.EXPO_PUBLIC_API_URL')()` (eval) para leer env y `occurred_at: Date.now()` number ms, y `AbortController` huérfano sin propagar `signal` (store y hook duplicaban timeout 5s). Propuesta copia de snippet web sin adaptar a Expo/Metro.
Evidencia: mobile/src/lib/api.ts:11 `Function(...)`, mobile/src/store/appStore.ts:82 `occurred_at: Date.now()`, mobile/src/hooks/useConnection.ts:50 `Date.now()`, mobile/src/store/appStore.ts:84-99 `controller+Promise.race` sin `signal`, mobile/src/hooks/useConnection.ts:53 `abortRef` huérfano (pre fix)
Por qué falla: `Function` es CWE-95 eval, rompe CSP/no-eval y escapa static analysis de secretos (security media, OWASP A03). `Date.now()` pierde TZ/ISO y rompe contrato `openapi` `occurred_at` ISO trazable NFR-005 y `slog` backend. Abort huérfano hace `Disconnect` no cancelar fetch real → race `connected` pisa `idle` tras disconnect (flota fantasma) y desperdicia 2 timers/request. `simEnabled` bypass `setConn` deja toggle gris tras 202 (AC-002).
Refactor exigido: Cambiado a `globalThis.process.env.EXPO_PUBLIC_API_URL` directo (Expo inlines) + `Constants.expoConfig.extra.apiUrl` fallback, `occurred_at: new Date().toISOString()` SSOT, `postTelemetry(event, {signal})` con único `AbortController` por intento compartido, `useConnection` pasa `signal` y set `simEnabled:true` vía `setState`, `appStore` usa `ISO` y `signal`. `npm test 49/49 PASS` (incluye `api.test` 7/7), `tsc --noEmit` OK. Commit feat(mobile) TASK-005-02.
Auditor: reviewer | security | architect

## 2026-08-27 — Auditoría: mobile store duplicado connect + hook sin DIP [SPEC-005 TASK-005-02]
Severidad: media
Hallazgo: IA duplicó máquina `idle->connecting->connected/error` en `appStore.connect()` y `useConnection.connect()` ambos importando `postTelemetry` directo, sin port/interface. Dos fuentes de verdad para mismo caso de uso.
Evidencia: mobile/src/store/appStore.ts:5 `import {postTelemetry}`, :63-109 `connect: async`, mobile/src/hooks/useConnection.ts:5 `import {postTelemetry}`, :28-81 segundo `connect` (pre fix)
Por qué falla: Viola AGENTS.md DIP/clean architecture `domain→application→adapters→infra` (store application depende de adapter `lib/api` concreto) y DRY. Cambio de `fetch` a `WatermelonDB queue` obliga 2 edits y diverge `simEnabled` (hook no lo seteaba). En backend equivale a `application` importando `adapters/http`.
Refactor exigido: Deuda aceptada para TASK-005-02 (tests `TEST-002` cubren ambos por tolerancia), exigir SSOT `useConnection` como orquestador único y `appStore` solo estado puro `setConn/sync/net/db` con `TelemetryPort` interface en TASK-005-07. Registrado para bloquear duplicación en batch. `npm test 49/49` no regresa.
Auditor: reviewer | quality-auditor | architect

## 2026-08-27 — Auditoría: mobile WatermelonDB no init + LokiJS memory + infra→store [SPEC-005 TASK-005-03]
Severidad: alta
Hallazgo: IA generó `db/index.ts` (infra) importando `useAppStore` (application) y enmascarando fallo como `OK`, `App.tsx` nunca llamó `initDatabase()`, y `schema` con `migrations as unknown` + `LokiJSAdapter` memory en vez de `SQLiteAdapter` persistente.
Evidencia: mobile/src/db/index.ts:2 `import {useAppStore}`, :10-35 `tryInitWatermelon` catch `return {_mock:true}`, :40 `useAppStore.setState({db:'OK'})` incluso en mock error, mobile/src/App.tsx:1-11 sin `initDatabase`, mobile/src/db/schema.ts:24 `as unknown`, mobile/src/db/index.ts:20 `LokiJSAdapter` hardcoded `useIncrementalIndexedDB:false` (pre fix)
Por qué falla: Viola DIP `infra→application` (store) y fail-open `OK` fake rompe FR-003 `WatermelonDB status ○ OK/ERROR` y NFR-002 `survive kill 245` (LokiJS memory pierde datos, AC-009). `migrations as unknown` anula `tsc` y `pending 0` hardcodeado oculta observabilidad. Sin `initDatabase` en `App`, `db:'OK'` optimista permite `POST` aunque DB rota (BR-003).
Refactor exigido: Eliminado `import useAppStore` de `db/`, `initDatabase` memoiza `initPromise` + `SQLiteAdapter` con fallback `LokiJS` solo para Jest (`useIncrementalIndexedDB:true`), `pending_telemetry.sync_status isIndexed:true`, `App.tsx` `useEffect initDatabase().then(s=>setDb(s))` con `Date.now` guard <1s, `database` export via `getDatabase()` no `default`. `npm test 71/71 PASS`, `tsc --noEmit` OK. Commit feat(mobile) TASK-005-03.
Auditor: reviewer | quality-auditor | architect

## 2026-08-27 — Auditoría: mobile store connect duplicado + StatusPanel triple subscribe [SPEC-005 TASK-005-03]
Severidad: alta
Hallazgo: IA duplicó 60L `connect` entre `store/appStore.ts` y `hooks/useConnection.ts` con timeout divergente, y `StatusPanel` con 3 `useAppStore` selectors sin `useShallow` + colores hex hardcodeados.
Evidencia: mobile/src/store/appStore.ts:63-106 vs mobile/src/hooks/useConnection.ts:28-81 (pre fix), mobile/src/components/StatusPanel.tsx:12-18 tres `useAppStore(s=>s.xxx)` sin memo
Por qué falla: DRY/SRP grave, drift `plate:string` vs `{plate,lat,lon,speed}` y `simEnabled` bypass. `StatusPanel` 3 renders/tick y hex `#16a34a/#dc2626` duplicado 5x viola DRY y perf O(3) hot path. `as unknown` en `db` desactiva type-check.
Refactor exigido: `store.connect` dejado como puro `setConn` sin `postTelemetry` (SSOT `useConnection` orquestador), `useAppStore` solo estado, `StatusPanel` futuro `useShallow` + `PALETTE` token (deuda `TASK-005-09`). `npm test 71/71` no regresa.
Auditor: quality-auditor | reviewer | architect

## 2026-08-27 — Auditoría: mobile Disconnect race + doble DELETE no atómico [SPEC-005 TASK-005-04]
Severidad: alta
Hallazgo: IA generó `appStore.disconnect()` con orden inverso `set idle -> await clearPending() -> abort() -> clearInterval(0)` fire-and-forget, `try{}catch{}` vacío swallow error, doble `AbortController` (`globalThis.__fleetAbortController` + `__abortController` stringly sin señal compartida) y `clearInterval(0)` mágico, `let disconnecting` global sin `useRef/store`. `db/telemetry.clearPending()` hacía doble `DELETE` no atómico: si `database.collections` existía hacía `database.write(destroy)` y además `adapter.unsafeExecute DELETE` (dos vías) y siempre `mockQueue.length=0` fuera de `try` enmascarando fallo, sin `purgePending` puro. `useConnection.disconnect()` abortaba después de `clearPending` dejando fetch en vuelo y `App.handleDisconnect` era `() => hookDisconnect()` fire-and-forget sin `disabled`, `pending 0` literal.
Evidencia: mobile/src/store/appStore.ts:37 `let disconnecting`, :75-112 orden `set idle`, `try{clearPending} catch{}`, `globalThis.__fleetAbortController`, `clearInterval(0)`, mobile/src/db/telemetry.ts:203-238 doble branch `if(collections) write` + `if(adapter) DELETE` + `mockQueue.length=0` fuera de try, mobile/src/hooks/useConnection.ts:92-133 abort después, mobile/src/App.tsx:48 `handleDisconnect=()=>hookDisconnect()` + `pending 0` literal (pre fix commit <TASK-005-04>, ver git diff)
Por qué falla: Race: intervalo 5s sigue generando `enqueue` durante `await clearPending()` (1GB/semana leak OFF->ON, BR-005) y `fetch` en vuelo pisa `idle` tras disconnect (flota fantasma, NFR-002). Doble DELETE no atómico: si `write` falla a medias, `DELETE` borra parcial y `mockQueue` oculta error → purga a medias con 20 pending quedan 10 huérfanos, `countPending` diverge y `Disconnect` reporta 0 falso (AC-004). `catch{}` swallow rompe fail-closed `db:ERROR` y no revert `plate` (pérdida idempotencia `client_event_id`). `clearInterval(0)` es no-op en iOS y no detiene generator 5s (batería). `fire-and-forget` permite doble tap y `pending 0` literal oculta observabilidad offline→sync.
Refactor exigido: `appStore` con `isDisconnecting/__abortController/__telemetryInterval` tipados en zustand (no `globalThis` stringly, no `let` global), `disconnect` reordenado `abort inmediato -> clearInterval sincrónico ANTES de await -> await purgePending() transacción única -> set idle DESPUÉS solo si OK else `db:ERROR` + revert `plate` + `throw`, sin `catch{}` vacío, sin `clearInterval(0)`. `telemetry.ts` extraído `purgePending()` puro: si `collections` usa `database.write(Promise.all(destroy))` sino `adapter.unsafeExecute DELETE`, nunca ambas, nunca `mockQueue` fuera de try, propaga error. `useConnection` aborta señal única pasada a `postTelemetry` (ya `opts.signal` no crea segundo controller) sincrónico antes de `clearInterval`. `App` `handleDisconnect async/await` con `disabled={isDisconnecting}` y `pending ${countPending()}` real. IA propuso purgar luego parar (no escalable: genera durante purga y deja fetch vivo) se forzó `abort->stop->purge`. `npm test 97/97 PASS`, `tsc --noEmit` 0.
Auditor: reviewer | architect

## 2026-08-27 — Auditoría: mobile RouteToggle doble View + a11y desplazada + ON->OFF leak [SPEC-005 TASK-005-05]
Severidad: media
Hallazgo: IA generó `RouteToggle` con 2 nodos (View fantasma con 7 @ts-ignore + Switch real sin testID), `trackColor` duplicado y `thumbColor` igual a track, `testID/hitSlop/minHeight` en View no semántico, y `ON->OFF` solo `setSimOn` sin limpiar `selectedRoute`/`intervalRegistry`/`clearPending`.
Evidencia: mobile/src/components/RouteToggle.tsx:21-51 `View testID="sim-toggle"` + `Switch` sin testID, `trackColor={{false:current,true:current}}` `thumbColor={current}`, mobile/src/store/appStore.ts:68 `setSimOn` sin guard purga, mobile/src/App.tsx:78 `setRoute` bypass (pre fix)
Por qué falla: Viola DRY/SRP + a11y FR-005/NFR-007 (Switch sin testID → falso positivo `getByTestId` encuentra View, VoiceOver no anuncia Switch, hitSlop en View no amplía área táctil, thumb sin contraste). `App` orquestaba limpieza `selectedRoute`/`interval` y fue eliminado en refactor, quedando leak: `selectedRoute` permanece verde/azul y `intervalRegistry` sigue encolando tras `simOn=false` (batería, NFR-001). `as unknown`×7 anula `tsc`.
Refactor exigido: Eliminado View fantasma, 1 único `Switch testID="sim-toggle"` con `accessibilityLabel`, `accessibilityState`, `hitSlop 10`, `minHeight 44`, `trackColor false #e5e7eb true #16a34a`, `thumbColor #ffffff`, sin `@ts-ignore`. `store` añade `toggleSimOn(v)` con `intervalRegistry.clear` + `set simOn:false, selectedRoute:null` sincrónico + `resolveClearPending().then(clearPending)` y `setConn` limpia `simOn` al salir de `connected`. `npm test 121/121 PASS`, `tsc 0`. Commit feat(mobile) TASK-005-05.
Auditor: quality-auditor | mobile-auditor | architect

## 2026-08-27 — Auditoría: mobile parches globales + missing sync + GPS stub [SPEC-005 integral 01..06]
Severidad: alta
Hallazgo: IA generó 3 parches globales no escalables: `intervalRegistry` con `Object.defineProperty(globalThis,'setInterval')` y `clearAll->clear(99999)`, `RouteToggle` con `React.createElement` patch `RCTSwitch`, y estado singleton `let simIdx=0` + `nextGpsPoint` hardcodeado sin `expo-location`; además faltaban `lib/sync.ts`/`hooks/useSync.ts` (plan §5) para `flush 5s/50` backoff, y `store` mezclaba `__abortController` infra en Zustand + doble timeout 5s `api`+`hook` con `sleep(0)×3`.
Evidencia: mobile/src/store/intervalRegistry.ts:11-58 defineProperty, mobile/src/components/RouteToggle.tsx:5-26 React.createElement, mobile/src/hooks/useSimulatedRoute.ts:6 let simIdx, mobile/src/hooks/useTelemetryGenerator.ts:10 hardcode 6.2442 + early-return `!simOn`, mobile/src/lib/api.ts:21 timeout interno + hook:59 Promise.race+sleep, mobile/src/store/appStore.ts:22 __abortController, mobile/src/lib/sync.ts 0 hits (pre fix, ver git log ed1093c^)
Por qué falla: Viola Clean Arch `infra` mutando primitiva lenguaje, SOLID OCP, `The Go` resource owner, `React purity`, `AGENTS.md` env y `shared mutable` sin owner → fuga `n` timers huérfanos 5s, test pollution `jest.useFakeTimers`, `tintColor` injection, `js fps <55` con `O(n²)` telemetry, `pending 245` nunca drena (FR-009 BR-009), `ON->OFF` no GPS real (AC-007), `5GB NATS` herd sin `jitter 0-60s` ni `Retry-After`, doble `AbortController` desperdicia 2 timers/req bajo 1k msg/s.
Refactor exigido: Eliminado `defineProperty`/`makeWrappedSet`→ clase pura `IntervalRegistry` sin `globalThis`, `RouteToggle` 1 único `Switch` con `trackColor` nativo sin `@ts-ignore`, `simIdx`→ `appStore.selectedRouteIdx` + `nextSimPoint(route,idx)` puro, `nextGpsPoint`→ `expo-location.getCurrentPositionAsync` con permiso guard, `TelemetryPort`/`IntervalPort` en `store/ports.ts` inyectado en `App.tsx` composition root, `lib/sync.ts` `flushPending 500` + `hooks/useSync.ts` `5s/50` `Retry-After`+`5s*2^n cap60+jitter`+`attempts>=5->failed`, timeout único `API_TIMEOUT_MS=5000` con `AbortSignal.timeout`, `store.connect`→ setters puros SSOT `useConnection`. `npm test 145/145 PASS`, `tsc 0`, `intervalRegistry` `clearAll` iterando solo `ids` propios. Commit refactor(mobile) integral.
Auditor: reviewer | quality-auditor | security | scalability | architect

## 2026-08-27 — Auditoría: mobile RouteButtons View-as-Pressable + lint bypass [SPEC-005 TASK-005-09]
Severidad: alta
Hallazgo: IA generó `RouteButtons` con `function Pressable(props:any){ return <View onPress> }` + `void RNPressable` y `mobile/package.json` con `lint: eslint ... || echo` sin `.eslintrc`, y `lib/api.ts` con doble `AbortController`+`setTimeout` sin `clearTimeout` + `Appfile` hardcode `fleet.ci@example.com`.
Evidencia: mobile/src/components/RouteButtons.tsx:2-9 View+onPress, mobile/package.json:13 `|| echo`, mobile/src/lib/api.ts:22 `setTimeout` sin clear, fastlane/Appfile:1 hardcode (pre fix, ver git diff 08ebbda^)
Por qué falla: `View.onPress` no dispara en RN (requiere `Pressable`), `disabled` fantasma → `AC-006` no funcional en device, `lint || echo` siempre verde oculta `any` en `flushPending 500` hot path, doble timer `5s` desperdicia 2 timers/req bajo 1k msg/s y leak `addEventListener` sin remove, `Appfile` hardcode obliga fork por team. Viola WCAG 2.5.5, `AGENTS.md` lint gate, `12-factor` env.
Refactor exigido: `RouteButtons` → `Pressable` nativo con `disabled/hitSlop/minHeight44`, `.eslintrc.json` real con `@typescript-eslint/no-explicit-any:error` y `lint` sin `|| echo` + `eslint` devDeps, `api.ts` timeout único `AbortSignal.timeout(API_TIMEOUT_MS)` con `clearTimeout` en `finally`, `Appfile` → `ENV["APP_IDENTIFIER"]`. `npm test 187/187 PASS`, `tsc 0`, `npm run lint 0`, `docker compose config -q 0`. Commit refactor(mobile) TASK-005-09.
Auditor: reviewer | quality-auditor | architect

## 2026-08-28 — Auditoría: mobile SDK 52→54 TS 7.0.2 rompe Metro + sdkVersion deprecated [SPEC-005 TASK-005-54]
Severidad: media
Hallazgo: IA `mobile-expo` propuso `npx expo install expo@54.0.12 jest-expo@54.0.12` con `npm install typescript@latest` → TS 7.0.2 + `mobile/tsconfig.json` con `moduleResolution node` y `baseUrl .` + `sdkVersion` retenido, copiado de template Expo 54 sin validar compatibilidad `@expo/cli 0.22.28`.
Evidencia: mobile/package.json:38 `typescript 7.0.2`, mobile/tsconfig.json:8 `moduleResolution node10`, :9 `baseUrl .`, mobile/app.json:8 `sdkVersion 54.0.0` (pre commit 6768ce1, ver git log 08ebbda^..bc46ced)
Por qué falla: TS 7.0.2 cambia `ts.getCurrentDirectory` API y `@expo/cli/evaluateTsConfig` hace `ts.getCurrentDirectory()` sin arg → `TypeError: getCurrentDirectory is not a function` y `npx expo start --go` aborta; `moduleResolution node10` + `baseUrl` es deprecado en TS 7 (`ignoreDeprecations 6.0` inválido en 5.9.3) y rompe `expo/tsconfig.base` bundler; `sdkVersion` es legacy en SDK 54 (Expo lo infiere de `expo` dep) y genera warning `expo-doctor` + EAS. Viola AGENTS.md `docker compose config` gate y NFR-002 DX.
Refactor exigido: Pin `typescript 5.9.3` (SDK 54 exige 5.9+ no 7), `tsconfig` con `module ESNext` + `moduleResolution bundler` sin `baseUrl/ignoreDeprecations`, `sdkVersion` documentado como deuda media (quitar en próximo chore, ver auditoría mobile-auditor M1). `npx expo start --go` OK sin TypeError, `tsc --noEmit 0`, `expo config sdkVersion 54.0.0` inferida. Commit 6768ce1 + df6baf2 + bc46ced.
Auditor: quality-auditor | mobile-auditor | architect

## 2026-08-28 — Auditoría: mobile SDK 52→54 React 19 + Watermelon JSI fallback silencioso [SPEC-005 TASK-005-54]
Severidad: media
Hallazgo: IA sugirió `react 18.3.1` con `expo 54.0.12` (peer mismatch `react 19.1.0` requerido) y `WatermelonDB` con `try SQLiteAdapter jsi:true catch LokiJSAdapter` silencioso + `babel-plugin-dynamic-import-node` global y `react-native-get-random-values` triplicado, heredado de SDK 52 sin auditar New Architecture RN 0.81.
Evidencia: mobile/package.json:14 `react 18.3.1`, mobile/src/db/index.ts:33 `try SQLiteAdapter` `catch LokiJS` sin log, babel.config.js:5 `plugins dynamic-import-node`, mobile/src/db/telemetry.ts:1 + hooks/useConnection.ts:1 `import react-native-get-random-values` (pre fix, ver mobile-auditor ses_fb9e9b58 y quality-auditor ses_fb9e4f0d)
Por qué falla: Expo Go 54 exige tríada `expo 54 / RN 0.81.5 / react 19.1.0` (bundledNativeModules.json); mismatch `18.3.1` da `expo install --fix ERESOLVE` y warning SDK 54. Watermelon `Untested on New Architecture` con fallback silencioso pierde durabilidad `245 persist` (LokiJS memory no IndexedDB) y `useWebWorker:false` bloquea JS thread `O(50)` flush 5s. `dynamic-import-node` transpila `import()`→`require` rompe tree-shaking `await import telemetry` y `getRandomValues` sin mantenimiento desde 2021 duplica side-effect 3×. Viola NFR-002 offline→sync y AGENTS.md 16GB.
Refactor exigido: Upgrade `react 19.1.0 + react-native 0.81.5 + jest-expo 54.0.12 + expo-* 18/19/16 + babel-preset-expo 54`, `@testing-library/react-native 13.3.3` + `@types/react 19.1.10` para `act` React 19, `expo-sqlite 16.0.10` alineado, mantener `WatermelonDB@0.27.1` con fallback LokiJS solo para Jest (`isJest` guard) + `expo.doctor.exclude` en próximo chore, `babel env.test` para plugin, `expo-standard-web-crypto` como deuda futura. `npm test 187/187 PASS`, `expo install --check Dependencies are up to date`, `expo-doctor 17/18`. Commit SDK54.
Auditor: mobile-auditor | quality-auditor | architect

## 2026-08-29 — Auditoría: k6 thresholds rate<0.02 ignora que 400 cuenta como failed [SPEC-006]
Severidad: media
Hallazgo: IA `devops` generó `infra/k6/load.js` con `thresholds: {http_req_failed:['rate<0.02'], checks:['rate>0.99']}` siguiendo literal `BR-003` `rate<0.02 (solo 5% inyectado)`. Propuso `copy` desde `skill chaos-load-testing` sin ajustar a semántica `k6`.
Evidencia: infra/k6/load.js:8-11 `http_req_failed rate<0.02` (pre a761ac3, ver `k6 run --vus 10 --duration 10s` repro: `failed 4.28% >0.02` con 21/490 `400` válidos) + infra/k6/chaos.js:19 idem + `k6` docs `http_req_failed` = `status outside 2xx-3xx`
Por qué falla: `k6` marca `http_req_failed=true` para todo no-2xx, por lo que `5%` de `400` inválidos (BR-002 requerido) ya hace `failed≈0.05` y `rate<0.02` siempre falla incluso con ingesta sana; el smoke real `490 reqs 21×400` dio `4.28%` y rompió thresholds aunque `p95 8.61ms` y `checks 99.79%` estaban verdes. Viola `quality-auditor` thresholds medibles y NFR-001 p95, y obliga a `Retry-After` mal calibrado.
Refactor exigido: Cambiado a `http_req_failed:['rate<0.07']` (`5%` base `400` + `2%` tol para 503 backpressure real) en `load.js:10` y `chaos.js:20`, `spec.md FR-004/BR-003/AC-001` y `plan.md` alineados a `0.07`, re-run `k6 run --vus 10 --duration 10s` → `p95 7.2ms failed 4.08% checks 99.72%` 3/3 verde, `select count(*) 633 uniq 633 0 dups 0 GTP98` confirma dedup. Commit a761ac3 + fd2d59a.
Auditor: architect | quality-auditor

## 2026-08-29 — Auditoría: Terraform 0.0.0.0/0:5432 + tfvars con password versionado [SPEC-006]
Severidad: alta
Hallazgo: IA `devops` propuso `modules/data/main.tf` con `ingress { cidr_blocks = ["0.0.0.0/0"] from_port 5432 }` para RDS y `terraform.tfvars` con `db_password="change-me-local"` versionado, copiado de ejemplo `RDS open` de `skill iac-and-cicd` sin hardening.
Evidencia: infra/terraform/modules/data/main.tf:15 `cidr_blocks 0.0.0.0/0` 5432 (pre refactor, ver git diff 45446d9^) + infra/terraform/terraform.tfvars (no `.example`) con `db_password` en claro (pre b5f9f49) y `docker-compose.yml` previo `POSTGRES_HOST_AUTH_METHOD=trust` (IAUDIT 2026-08-23)
Por qué falla: Viola CIS AWS + OWASP A07 `Security Groups` (`0.0.0.0/0:5432` expone `TimescaleDB` hypertable `telemetry` con GPS PII a internet) y AGENTS.md `nada de secretos en git` + `12-factor` env; `gitleaks` detecta `db_password` en repo público y bots cosechan en minutos; `terraform.tfvars` versionado obliga fork por env y rompe `fmt/validate` gate si se rota. Similar a hallazgo `trust` auditado 2026-08-23.
Refactor exigido: `modules/data/main.tf` cambiado a `security_groups = [var.ecs_sg_id]` (solo ECS SG puede `5432`), `.gitignore` `*.tfvars !*.tfvars.example !.terraform.lock.hcl`, solo `terraform.tfvars.example` versionado sin `db_password` (`# inject via TF_VAR_db_password`), `variables.tf` `db_password sensitive=true` sin default, `gitleaks` gate verde (`terraform fmt -check OK`, `init -backend=false && validate Success` root/dev/prod). Commit 45446d9 + b5f9f49.
Auditor: security | reviewer | architect

## 2026-08-30 — Auditoría: mobile App intervals leak + act warnings + hang 30s [SPEC-005 TASK-005-06]
Severidad: alta
Hallazgo: IA generó `mobile/src/App.tsx` con `setInterval(load,2000)` crudo sin `intervalRegistry` y `load()` haciendo `setPending(c)` incondicional sobre closure stale, e `initDatabase().then(setState)` incondicional incluso cuando `db===OK`. `useSync.ts` con `setInterval(trigger→flush,5000)` sin `IntervalPort` y `setSync('CONNECTED')` desde `CONNECTING` redundante. `RouteButtons.test.tsx` renderea `<App/>` completo con `jest.useFakeTimers()` pero sin mockear `useSync/useNetInfo`, dejando 3 intervalos compitiendo (`App 2s` + `useSync 5s` + `useTelemetryGenerator 5s`) + `initDatabase` lento con fakes → 5 warnings `An update to App/StatusPanel inside a test was not wrapped in act(...)` y 2 timeouts `Exceeded timeout 30000ms for hook/test` + `console.warn [db] init slow 10000ms` en CI.
Evidencia: mobile/src/App.tsx:57-72 `setInterval(load,2000)` sin `getIntervalPort()/register`, `pending` closure stale, `initDatabase().then` sin `cur===OK` guard (pre fix, ver `git diff` App.tsx 76e619d), mobile/src/hooks/useSync.ts:150-155 `setInterval(...)` sin `port.register` + `if(curSync !== 'CONNECTED') setSync` redundante (pre fix), mobile/src/components/RouteButtons.test.tsx:88-109 `jest.useFakeTimers()` sin `injectTelemetryPort` + sin `useSync` mock + `afterEach` sin `cleanup()` ni `flushMicrotasks` (pre fix, CI log `RouteButtons 61.553s FAIL`)
Por qué falla: `jest.useFakeTimers()` congela `Date.now()` y `setInterval` reales; `App` 2s dispara `setPending` fuera de `act` (React 19 requiere `act` para state updates en tests), `useSync` dispara `setSync('CONNECTED')` desde `CONNECTING` (ya lo hace `useConnection` 202) generando `StatusPanel` warning `setSync` fuera de `act`; `intervalRegistry.reset()` solo `Set.clear()` sin `clearInterval` deja timers huérfanos que sobreviven entre tests y hacen `advanceTimersByTime` disparar ticks de tests anteriores dentro de `act` no envuelto → warnings + hook timeout 30s. `getPendingCountSafe` caía a `await import('../db/telemetry')` si port no inyectado → hang 10s con fakes (`[db] init slow`). Viola AGENTS.md `define done: tests verdes sin hallazgos altos` y `quality-auditor` `O(n) alloc` + `16GB RAM` NFR-004 (leak 1 interval/test × 19 suites = 19 timers zombies) y `frontend-auditor` `a11y` (no aplica pero performance).
Refactor exigido: `App.tsx` con `pendingRef` (`useRef` + `pendingRef.current===c` guard) evitando `setState` redundante/churn fuera de `act`, `getPendingCountSafe()` helper con `getTelemetryPort()` primero y solo fallback `import`, `initDatabase` guard `if(useAppStore.getState().db==='OK') return` + `curNow!==status` guard + `void run()` async con `cancelled` flag, intervalo 2s registrado vía `getIntervalPort().register/clear` y `pendingRef` check en `handleDisconnect`; `useSync.ts` intervalo 5s registrado vía `getIntervalPort().register/clear` y `setSync` solo cuando `curSync==='ERROR'` (no desde `CONNECTING`); `RouteButtons.test.tsx` mockea `useSync/useNetInfo`, inyecta `TelemetryPort/IntervalPort` mocks con `intervalRegistry`, `jest.useFakeTimers()` + `flushMicrotasks()` (`await act+double Promise.resolve`) + `cleanup()` determinístico en `afterEach` con `clearAll/reset/useRealTimers`. `npm test -- --run --coverage --runInBand` 19 suites 187 tests PASS en 5.977s (antes 68.217s FAIL, 61.553s RouteButtons), 0 act warnings, 0 slow 10s. Commit fix(mobile) TASK-005-06.
Auditor: reviewer | quality-auditor | architect

## 2026-08-30 — Auditoría: mobile RouteButtons test flakiness sin determinismo + console.warn no silenciado [SPEC-005 TASK-005-06]
Severidad: media
Hallazgo: IA `test-engineer` inicial generó `RouteButtons.test.tsx` sin helper `flushMicrotasks`, sin `injectIntervalPort`, sin silenciar `console.warn [db] init`, y con `afterEach` que hacía `jest.restoreAllMocks()` restaurando `setInterval` fake a real mientras `intervalRegistry` aún tenía ids, sin `cleanup()` de RTL. Propuso `jest.useFakeTimers()` legacy y `global.fetch` mock por test sin `beforeEach` centralizado.
Evidencia: mobile/src/components/RouteButtons.test.tsx:88-109 `beforeEach` sin `mockCountPending.mockResolvedValue(0)` + sin `injectTelemetryPort` + `afterEach` con `jest.restoreAllMocks()` sin `cleanup` (pre fix, ver CI log `thrown: Exceeded timeout 30000ms for a hook at RouteButtons.test.tsx:5:1`)
Por qué falla: Sin `flushMicrotasks`, `fireEvent.press` → `selectRoute` async `clearPending().then(setState)` deja promesa pendiente fuera de `act` → warning `StatusPanel setSync not wrapped`. Sin `injectIntervalPort`, `useTelemetryGenerator` registra intervalo vía `setInterval` directo no rastreado por `intervalRegistry.getIds()` → assertion `ids.length>0` falla intermitente y `intervalRegistry.reset()` no limpia timers reales → leak. `restoreAllMocks` restaura `setInterval` nativo mientras `jest.useFakeTimers()` sigue activo → hook timeout 30s (`afterEach` at `node_modules/@testing-library/react-native/src/index.ts:15:5`). Sin silenciar `warn`, `[db] init slow` log contamina output y hace `collectCoverage` lento.
Refactor exigido: Añadido `flushMicrotasks` helper (`act+double resolve`), `injectTelemetryPort`/`injectIntervalPort` con `intervalRegistry` en `beforeEach`, `jest.spyOn(console,'warn').mockImplementation(()=>{})`, `afterEach` con `await act(flush)+cleanup()+clearAll/reset+useRealTimers+clearAllMocks` (no `restoreAllMocks` que rompe fakes). Determinismo 100% en CI `--runInBand`. Auditoría `reviewer` PASS.
Auditor: reviewer | test-engineer | architect

- Cada entrada cita evidencia en git (commit/SHA previo) para que el evaluador
  pueda ver el "antes y después".
- **Dirección de la trazabilidad**: esta bitácora cita sus fuentes (ADRs,
  commits, archivo:línea); ningún documento arquitectónico (ADR, C4, specs)
  debe depender de o referenciar a IAUDIT. Únicas referencias entrantes válidas:
  `README.md` (requisito del entregable) y `AGENTS.md` (contrato de proceso).
