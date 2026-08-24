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

## Convenciones

- Severidad alta = task NO cerrado hasta refactor + re-auditoría.
- Cada entrada cita evidencia en git (commit/SHA previo) para que el evaluador
  pueda ver el "antes y después".
- **Dirección de la trazabilidad**: esta bitácora cita sus fuentes (ADRs,
  commits, archivo:línea); ningún documento arquitectónico (ADR, C4, specs)
  debe depender de o referenciar a IAUDIT. Únicas referencias entrantes válidas:
  `README.md` (requisito del entregable) y `AGENTS.md` (contrato de proceso).
