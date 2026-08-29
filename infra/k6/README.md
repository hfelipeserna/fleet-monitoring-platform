# k6 — Carga y caos (SPEC-006)

Scripts para PRUEBA-TECNICA Sec 4.E: 300 VU, 10% `client_event_id` duplicado (`Nats-Msg-Id` DuplicateWindow 2m), 5% payloads inválidos.

## Scripts

- `load.js` — `constant-vus 300 / 5m`, `POST /v1/telemetry` contra LB `http://localhost:8080`.
  Thresholds `p(95)<250`, `http_req_failed<0.02`, `checks>0.99`.
  Pool `lastIds` 1000, `rand<0.10` dup reuse id, `rand<0.0526` del resto (~5%) inválido (`GTP98/speed -1/lat 100/{} sin plate/...`), resto válido (`GTP###`, `speed 0..90`, `lat/lon` Medellín 6.14..6.34/-75.68..-75.48 o Bogotá 4.65..4.77/-74.12..-74.02, `uuid` + `occurred_at` ISO).

- `chaos.js` — variante batch/caos: `constant-vus 50 / 2m`, mezcla `POST /v1/telemetry` y `POST /v1/telemetry/batch` (10 eventos). Útil para dedup con batch y `ramping-vus` alternativo (ver `options`).

Ambos usan `__ENV.BASE_URL` (`http://localhost:8080` default) y no contienen secretos (`GEMINI_API_KEY`/`DATABASE_URL` no hardcodeados).

## Levantar stack

```bash
docker compose up --wait
curl -s http://localhost:8080/healthz | jq .
curl -s http://localhost:8222/varz | jq .jetstream
```

## Correr k6

```bash
brew install k6   # o grafana/k6:latest
k6 run infra/k6/load.js
BASE_URL=http://localhost:8080 k6 run infra/k6/load.js
k6 run --vus 10 --duration 10s infra/k6/load.js   # smoke 16GB
k6 run infra/k6/chaos.js
k6 run --summary-export=summary.json infra/k6/load.js
```

Docker alternativo:

```bash
docker run --rm -i --network host -e BASE_URL=http://localhost:8080 \
  -v $PWD/infra/k6:/scripts grafana/k6 run /scripts/load.js
```

## Verificar dedup (sin duplicar filas)

```bash
# snapshot antes
curl -s "http://localhost:8080/api/fleet/positions?limit=500" | jq 'length'

# tras k6 (o durante)
curl -s "http://localhost:8080/api/fleet/positions?limit=500" | jq 'length'
# únicos no deben crecer artificialmente por los 10% dup

# DB directa (si hay psql)
psql "$DATABASE_URL" -c "select count(distinct client_event_id) from telemetry;"
psql "$DATABASE_URL" -c "select count(*) from telemetry where plate='GTP98';"  # 0
```

Inválidos `5%` deben responder `400` y no publicar a NATS; duplicados `10%` responden `202` pero segundo insert es `ON CONFLICT DO NOTHING`.

JetStream bytes:

```bash
curl -s http://localhost:8222/jsz | jq .jetstream
# o: nats stream info TELEMETRY --server nats://localhost:4222
```

## Thresholds y CI

`ci.yml` job `load-tests` hace `node --check infra/k6/*.js` (syntax, no requiere stack). Full `k6 run` solo local/manual o job `k6` opcional. Si `p95>250` o `failed>0.02` o `checks<0.99`, k6 falla (señal backpressure/bug).

## Notas 16GB

- Stack `docker compose` ~6–9GB, `k6 300 VU` ~1–2GB (1000 VU similar). No levantes `observability` (`prometheus/grafana/loki/tempo`) + `k6 300 VU` + `go test -race` en los 4 bins simultáneo.
- Para dev, usa smoke `k6 run --vus 10 --duration 10s` o `--vus 50 --duration 30s`.
- `DuplicateWindow 2m` ya en `TELEMETRY` (SPEC-001) + `telemetry PK(client_event_id,received_at)` + `telemetry_dedup` cubren dedup corta y larga.
