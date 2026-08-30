# ADR-0009 — k6 como herramienta de caos y carga (descarta JMeter)

- **Fecha:** 2026-08-29
- **Estado:** Aceptado
- **Decisores:** `architect` + dictamen `scalability` (carga 5k @1k msg/s) y `security` (sin secretos en scripts)
- **Afecta a:** SPEC-006, `infra/k6/`, `infra/terraform/`, `AGENTS.md` máquina 16GB, ADR-0001 (NATS liviano)

## Contexto

PRUEBA-TECNICA Sec 4.E exige *"Script (k6, JMeter) para simular cientos de vehículos, inyectando 10% de peticiones duplicadas y 5% de errores"* y *"scripts IaC (Terraform, AWS CDK) y ejecución local orquestada vía Docker Compose"*. La máquina de dev tiene **16GB RAM** (core 6-9GB + observabilidad +2GB). Hay que elegir herramienta de carga que pruebe **dedup por `client_event_id` → `Nats-Msg-Id` DuplicateWindow 2m** sin romper la filosofía *un binario liviano* que justificó elegir NATS JetStream sobre Kafka en ADR-0001.

## Decisión

**k6** (binario Go, scripts JS) como herramienta canónica de caos/carga.

- Scripts en `infra/k6/load.js` (`constant-vus 300 / 5m`) y `infra/k6/chaos.js` (batch mix), thresholds `p(95)<250`, `http_req_failed<0.07` (5% 400 +2% tol), `checks>0.99`, `BASE_URL` vía `__ENV`, `node --check` en CI.
- Inyección: `Math.random()<0.10` → reuse `client_event_id` previo; `Math.random()<0.0526` del resto → `5%` payload inválido (`GTP98`, `speed -1`, `lat 100`, `{}`, `sin plate`, `sin lon`, `badJson '{'`).
- Terraform sigue en `infra/terraform/` (`~1.15/aws ~5`, `fmt/validate`).

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo |
|---|---|---|
| **k6 (elegida)** | Adoptada | Binario Go ~50MB, 1-2GB para 1000 VU, script JS versionable/mergeable, `node --check` en CI, 3 líneas para `10%/5%` y `DuplicateWindow` reuse, coherente con ADR-0001 *un binario liviano* |
| **JMeter** | Descartada | JVM ~1GB/100 threads, `.jmx` XML 500+ líneas no revisable por `reviewer`, `Groovy/Beanshell` para `10%/5%` frágil, rompe 16GB (core 6-9GB + JMeter + observability >16GB), contradictorio con haber descartado Kafka por RAM |

## Consecuencias

**Positivas:**
- `docker compose up` (6-9GB) + `k6 300 VU` (1-2GB) cabe en 16GB; smoke `10 VU 10s` (~49 req/s) para dev sin observability.
- Dedup demostrable: `10%` → `202` pero `count(distinct client_event_id)` estable; `5%` → `400` sin `PublishAsync`; verificado vivo `490 reqs p95 7.2ms failed 4.08% checks 99.72%` y `633 total 633 uniq 0 dups 0 GTP98`.
- `ci.yml` `load-tests` con `node --check` deja de ser stub pasivo; `compose config` sigue gate.

**Negativas:**
- Sin GUI para QA no-dev (mitigado: README con `k6 run` copy-paste).
- Reportes HTML requieren `prometheus-nats-exporter` ya en stack, no built-in como JMeter.
- `http_req_failed` cuenta `400` como fail → threshold debe ser `0.07` no `0.02` (lección IAUDIT 2026-08-29).

## Condiciones

1. `thresholds` documentados en SPEC-006 `BR-003` y en scripts (`k6 run --vus 10 --duration 10s` para dev).
2. Sin secretos hardcodeados en `*.js` (`__ENV.BASE_URL` solo).
3. Validación via `node --check` + `docker compose config` en CI, no `k6 Cloud`.

## Referencias

- SPEC-006 `FR-001..005` + `BR-001..003` + `AC-001..004/009/010`.
- Benchmark vivo 2026-08-29: `k6 run --vus 10 --duration 10s` `p95 7.2ms failed 4.08% checks 99.72%` + `select count(*) 633 uniq 633`.
- AGENTS.md 16GB, ADR-0001 (NATS 5GB dev), `skill chaos-load-testing` y `skill iac-and-cicd`.
