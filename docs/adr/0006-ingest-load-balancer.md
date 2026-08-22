# ADR-0006 — Balanceador de carga L7 delante del módulo de ingesta (recomendación)

- **Fecha:** 2026-08-22
- **Estado:** Aceptado (recomendación con condiciones; dictamen de `scalability` incorporado)
- **Enmienda 2026-08-22:** precisión de rate limiting dual batch/offline-first — ver condiciones 3-4 enmendadas; vinculante para diseño y SPECs posteriores (coherente con ADR-0001 cond. 3 enmendada)
- **Decisores:** `architect` + dictamen de `scalability` (obligatorio según AGENTS.md para decisiones candidatas a ADR) + enmienda validada por `architect`

## Contexto

El enunciado canónico exige dimensionar la ingesta para **cientos o miles de dispositivos enviando telemetría a alta frecuencia**, con el agravante de que el móvil es **offline-first y sincroniza en bloque al reconectar** (`docs/PRUEBA-TECNICA.md` §4.A y §4.D). Ese patrón no es carga uniforme: es sostenido + **thundering herd** en la ventana de reconexión.

Arquitectura vigente:

- ADR-0001 fija carga de referencia **5.000 dispositivos @ 1 evento/5 s = 1.000 msg/s sostenidos**, picos 2-3× (2.000-3.000 msg/s), horizonte 50.000 dispositivos (10.000 msg/s), payload 200-500 B. JetStream R1 file async ~370k msg/s; el broker no es el cuello (la carga usa 0,3-2 %). Cuello real: **TimescaleDB single-writer 15-50k filas/s** con batching; disco/retención (~1,4 GB/h a 1.000 msg/s).
- ADR-0002/ADR-0005: monorepo, 1 módulo Go, **4 bins** (`cmd/ingest`, `cmd/consumer`, `cmd/api`, `cmd/agent`). `ingest` es **stateless** (valida → `PublishAsync` a JetStream → `PublishAsyncComplete` antes del 200). Escalado previsto: réplicas de bins completos, sin mesh.

Sin balanceador, `ingest` es SPOF aunque sea stateless: una sola réplica directa no da HA, no permite drain en deploys y no absorbe el herd sin batch. Con batch + jitter el herd es problema de buffer NATS/DB, no de HTTP; sin batch, el herd sí satura HTTP.

Restricción dura: dev macOS Intel 16 GB RAM, `docker compose` local; prod Terraform AWS. **Prohibido K8s/service mesh en MVP** (ADR-0005).

## Decisión

**Recomendar balanceador L7 HTTP delante de `cmd/ingest` como componente de borde de la ingesta.** No es obligatorio para correr el MVP en dev con 1 réplica, pero es la topología recomendada y la que se despliega en prod desde el día 1.

- **Local (Compose):** `nginx:alpine` o `traefik` o `caddy` (<20 MB RSS) como servicio `lb` que hace reverse-proxy a `ingest`. Un binario, config en repo (`infra/nginx/nginx.conf` o labels traefik), sin control plane.
- **Prod (Terraform AWS):** **Application Load Balancer (ALB)** multi-AZ delante de ECS Service / ASG de `ingest`. TLS termina en el LB; healthcheck y drain nativos.
- **Algoritmo:** round-robin / least-conn, **sin session affinity** (stateless obligatorio). Healthcheck `GET /healthz` + drain 15-30 s.
- **`ingest` permanece stateless:** dedup por `client_event_id` en dominio, ventana `PublishAsync` acotada por réplica, idempotencia sin estado local. Cualquier réplica atiende cualquier dispositivo.
- **Contrato HTTP:** se mantiene `POST /v1/telemetry` (1 evento) por compatibilidad, pero el móvil **debe** usar **`POST /v1/telemetry/batch` (100-500 eventos/req, máx. 1 MB)**. El LB no resuelve el herd si el cliente no batchea.

Esta decisión **habilita el scale-out horizontal** previsto en ADR-0005 sin pagar mesh/K8s y con coste marginal (~15 MB RAM local, ~$16/mes ALB).

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo (para este workload y entorno) |
|---|---|---|
| **LB L7 ligero (elegida: nginx/traefik local + ALB prod)** | **Recomendada** | Habilita 2-3 réplicas stateless con healthcheck/drain/TLS; 20-40k rps/core nginx, ALB 10k+ rps/AZ autoescala — nunca es el cuello; encaja en 16 GB y en Compose/Terraform sin control plane |
| **Sin LB, 1 réplica directa** | Descartada para prod; tolerada solo en dev/demo <1.000 msg/s | SPOF: fallo o deploy = pérdida total de ingesta; no hay HA ni escalado aunque el bin sea stateless; desperdicia el diseño replicable de ADR-0005 |
| **DNS round-robin** | Descartada | Sin healthcheck ni drain; cache DNS 30-60 s desbalancea y magnifica el herd; falso escalado |
| **Service mesh / K8s Ingress (Istio/Linkerd)** | Descartada para MVP | Prohibida por ADR-0005; control plane 2-4 GB RAM resta >20 % de los 16 GB dev; beneficio cero hasta >30k msg/s; sobre-ingeniería |

## Condiciones obligatorias (dictamen `scalability`)

1. **LB ligero y contrato explícito:** Compose: `lb` (<20 MB) con `healthcheck` a `GET /healthz`; prod: ALB multi-AZ. **Drain 15-30 s**, TLS en LB. Sin affinity.
2. **Stateless obligatorio:** `ingest` sin estado local; dedup por `client_event_id`; round-robin. Cualquier réplica sirve cualquier `device_id`.
3. **Backpressure acotado por réplica (distinto de rate limiting):** `PublishAsync` ventana **512-1.024**, `PublishAsyncComplete` timeout **2-5 s**, `PublishAsyncMaxPending` acotado. Si ventana llena o JetStream `MaxBytes` cercano → **503 + `Retry-After`** (no bloquear; `503` = saturación infra, `429` = quota por device). `gobreaker` hacia NATS (ADR-0005). Este `503` es independiente del `429` de rate limiting (cond. 4).
4. **Endpoint batch obligatorio + rate limiting dual (enmienda 2026-08-22, vinculante para SPECs):** `POST /v1/telemetry/batch` **100-500 eventos/req, máx. 1 MB** — el móvil offline-first DEBE usarlo; `POST /v1/telemetry` queda para compat. **(a) Online:** token bucket por `device_id` 12 eventos/min (1/5 s, burst 20) → `429 + Retry-After: 5`; **(b) Batch recuperación:** bucket separado 1 batch/5 s, 500 eventos/30 s sliding window — excederlo también es `429` con reencole local, no `503`; **(c) Global LB:** `limit_req 10k rps` burst 2k. Sin batch el herd (90k-540k eventos) no es resoluble a nivel LB; con batch + este rate limiting dual, el herd baja a 30-90 rps y se drena sin violar SLO. SPECs de ingesta/móvil deben citar estos umbrales como ACs y testear `429` vs `503` por separado.
5. **Suavizado del herd en cliente:** jitter aleatorio **0-60 s** + backoff exponencial + sync escalonado en el SDK móvil. Con jitter 120 s, un burst de 9k rps cae a <1k rps.
6. **JetStream como buffer del burst:** stream `TELEMETRY` R1 file `MaxBytes` dimensionado (10 GB ≈ 5 h de backlog a 1.000 msg/s), `DiscardOld` con alerta, consumer durable Pull `MaxAckPending 10k`, persistencia en TimescaleDB en batches **500-1.000 filas** (`CopyFrom`). Dashboard lee continuous aggregates, no raw.
7. **Observabilidad y trigger de escala:** métricas obligatorias `ingest_inflight`, `nats_pending`, `p95_ingest_ms`, `db_lag_s`, `jetstream_bytes`. Alertas si `p95>200 ms` o `pending>5k` por >2 min. Local: escalar manual a **2 réplicas** si `p95>150 ms` sostenido o simulacro herd >3k rps. Prod: ASG/ECS `desired 2` desde día 1, escala a 3 si `CPU>60 %` o `ALB p95>200 ms` o `nats_pending>10k` 3 min.

## Consecuencias

**Positivas:**
- Ingesta con **HA y zero-downtime deploys** (drain) sin tocar código de negocio.
- Absorbe **thundering herd**: sin batch, herd 5k flota (5-15 % offline 30-60 min) = **90k-540k eventos = 1,5-9k rps sin batch / 30-90 rps con batch 100×**; 1 réplica Go (5-8k msg/s por core) satura sin batch, 2-3 réplicas + batch + jitter lo absorben sin romper SLO. Con sostenido 1k msg/s, 1 réplica va a 12-20 % CPU; pico 3k msg/s a 38-60 % — 2 réplicas dan headroom 40 %.
- Coste marginal: ~15 MB RAM local, ~$16/mes ALB; evita reescribir por saturación.
- Coherente con ADR-0001/ADR-0005: el LB no mueve el cuello (sigue en DB/disco), pero lo hace escalable horizontalmente.

**Negativas:**
- Salto extra de red (1-5 ms) y config adicional (nginx.conf / ALB target group) que mantener.
- Mala calibración de healthcheck/drain puede enmascarar réplicas degradadas (falso sano) — mitigado por condición 7.
- Más allá de ~10k msg/s sostenidos / ~9k rps burst con 2-3 réplicas, el LB deja de ser el límite; exige particionar consumer JetStream (subjects por `device_id` hash) y shardear TimescaleDB — ADR nuevo, no este.

**Disparador de particionado (hereda y precisa ADR-0001 cond.7 y ADR-0005 cond.5):** >10k msg/s sostenidos o >50k dispositivos (20-30k msg/s pico) → streams por shard + N consumers + workers DB por `device_id`. Hasta entonces, réplicas de `ingest` detrás del LB.

## Referencias

- Dictamen `scalability` 2026-08-22 — ADR-0006 candidato: veredicto **ADOPTAR CON CONDICIONES**, números de herd, capacidad por réplica (5-8k msg/s/core), y condiciones 1-7 incorporadas arriba.
- `docs/PRUEBA-TECNICA.md` §4.A (ingesta alta concurrencia, resiliencia) y §4.D (offline-first, sync en bloque).
- ADR-0001 (JetStream backbone, retención/bytes, backpressure), ADR-0002 (layout monorepo), ADR-0005 (monolito modular, breakers, escalado por réplicas).
- Stack decidido en AGENTS.md: `nats.go`, `jackc/pgx`, `sony/gobreaker`; config por env vars.
