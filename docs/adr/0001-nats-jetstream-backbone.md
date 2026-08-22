# ADR-0001 — Backbone de mensajería event-driven con NATS JetStream

- **Fecha:** 2026-08-17
- **Estado:** Aceptado (con condiciones; dictamen de `scalability` incorporado)
- **Decisores:** `architect` + dictamen de `scalability` (obligatorio según AGENTS.md para decisiones candidatas a ADR)

## Contexto

La plataforma debe ingestar telemetría de **miles de dispositivos a alta frecuencia** y desacoplar el write path (ingesta que acusa rápido al dispositivo) de la persistencia en TimescaleDB (sujeta a backpressure real de disco). Se requiere **replay de eventos**: si el consumer cae, al volver debe reprocesar sin pérdida de telemetría (durabilidad del stream, no sujeción en memoria).

Carga de referencia: 5.000 dispositivos @ 1 evento/5 s → **1.000 msg/s sostenidos**, picos 2-3× (2.000-3.000 msg/s). Horizonte de diseño: 10.000-50.000 dispositivos (2.000-10.000 msg/s). Payload 200-500 B.

Restricción dura de entorno: máquina de desarrollo **macOS Intel (darwin/amd64) con 16 GB RAM**; la infra local debe caber holgada (un solo binario, sin cluster en dev). Todas las imágenes elegidas publican `linux/amd64` nativo (sin emulación en Docker Desktop sobre Intel).

## Decisión

NATS **JetStream** como único backbone de eventos, con streams durables y sujetos versionados por contexto:

- `telemetry.raw.{region}.{device_id}` → stream `TELEMETRY` → consume el bin `consumer` con **ack explícito** y persiste en TimescaleDB. (Sujetos jerárquicos: JetStream ≥ 2.8 filtra por subject sin barrido lineal; habilita el particionado futuro.)
- `alerts.*` → stream `ALERTS` → consume `fleet` (read model/SSE) y el `AI Agent`.
- Idempotencia resuelta en **dominio** (`client_event_id` UUID v4, dedup); el broker NO ofrece exactly-once (decisión deliberada: el coste de fingirlo no vale para telemetría).

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo (para este workload y entorno) |
|---|---|---|
| **Kafka** | Descartada | ~100× más RAM y ~10× más superficie operativa para entregar lo mismo; rompe la restricción de 16 GB |
| **RabbitMQ** | Descartada | Replay débil (solo vía plugin Streams) |
| **Redis Streams** | Descartada | Exige durabilidad costosa y liderazgo multi-nodo frágil |
| **NATS Core (sin JetStream)** | Prohibida (cond. 8) | Fire-and-forget: sin durabilidad, mezcla de modelos |

## Condiciones obligatorias (dictamen `scalability`)

1. **Stream TELEMETRY:** `storage=file`, `retention=limits`, `discard=old`; **dev: replicas 1, `max_age=24h`, `max_bytes=5 GB`** (a 1.000 msg/s sostenidos el stream crece ~1,4 GB/h); **prod: replicas 3, `max_age=72h`, `max_bytes=50-100 GB`**. ALERTS: retención 7 días (volumen despreciable).
2. **Consumidor durable sí o sí**: nombre fijo, `AckExplicit`, `AckWait 30-60 s`, **`MaxDeliver=3` + DLQ `telemetry.dlq`**, `MaxAckPending` acotado (100-1.000). Esta ventana acotada + `PublishAsyncMaxPending` en la ingesta son **el** mecanismo de backpressure.
3. **Ingesta:** `js.PublishAsync` con ventana acotada y `PublishAsyncComplete()` **antes** del 200 al móvil; rate-limit por dispositivo (mín. 2-5 s entre eventos).
4. **Persistencia TimescaleDB:** escritura en batches (multi-row INSERT ≥ 500 filas o `CopyFrom`), **índice `(device_id, time)` obligatorio**, continuous aggregates para el dashboard, compresión desde el día 1 (5-15×).
5. **Monitoreo desde el día 1** (`prometheus-nats-exporter`): lag del consumer (`num_ack_pending`), bytes de stream ≥ 80% de `max_bytes`, `nats-server_jetstream_consumer_ack_pending`. Sin estas alertas, `discard=old` convierte el replay en pérdida silenciosa.
6. **Ventana de pérdida R1 declarada:** en dev, una caída de host puede perder hasta 2 min de mensajes ya acked (`sync_interval` default). Aceptable en demo; prod con R3 (ack por quórum) o `sync_always` si se requiere pérdida-cero.
7. **Disparador de particionado (documentado, NO implementado en MVP):** >10.000 msg/s o >50.000 dispositivos → streams por shard `telemetry.raw.{shard}` + durable consumer por shard + 2-4 workers de DB shardeados por `device_id`. Hasta entonces mononodo R1; clusterizar antes es sobre-ingeniería.
8. **Prohibido:** core-NATS (fire-and-forget) para telemetría — mezcla de modelos sin durabilidad; subjects planos sin jerarquía — imposibilita particionado futuro y filtrado por flota.

## Consecuencias

**Positivas:**
- Margen de 2-3 órdenes de magnitud en el broker para la carga de referencia (1.000-3.000 msg/s es el 0,3-2% del techo asíncrono ~370k msg/s R1 file).
- Resiliencia del write path: si TimescaleDB cae, la ingesta sigue aceptando y el stream retiene (1 h de caída = 1,4 GB de backlog); el backlog se drena al volver la DB.
- Un solo binario, 50-150 MB RSS en dev — footprint total del sistema < 4 GB de los 16.
- Consumer escalable en paralelo por subject (particionado natural, sin rebalanceo pesado).

**Negativas:**
- At-least-once obliga a idempotencia en el dominio (coste asumido: `client_event_id`).
- Naturaleza single-writer: el cuello de botella real es la escritura a TimescaleDB (15-50k filas/s con batching), no el broker.
- Retención en disco requiere gobernanza explícita (quota/edad) o se pierde telemetría silenciosamente; por eso las condiciones 1 y 5 son obligatorias.

**Cuello de botella real identificado:** disco (bytes/día de retención) y el single-writer a TimescaleDB — no el broker. El monitoreo y el batching de DB (condiciones 4 y 5) mitigan el único riesgo operativo serio.

## Referencias

- Benchmarks natsbench oficiales (core pub 14,8 M msg/s; JS sync 35,7k @ 28 µs; JS async file 403,8k), benchmark Go oficial nats-server PR #3425 (sync R1 file 32,5 µs; async W=1000 3,5 µs; R3 1 KB 22,7 µs), issue #5637 (contención same-node, filtrado por subject sin barrido lineal desde 2.8), docs de durabilidad JetStream (`sync_interval` 2 min, pérdida R1).