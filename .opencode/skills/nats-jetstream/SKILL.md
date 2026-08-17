---
name: nats-jetstream
description: Usa esta skill al trabajar con el bus de eventos NATS JetStream en Go: streams, consumidores durables, publish/ack, dedup con Nats-Msg-Id, replay. Trigger: NATS, JetStream, stream, consumer, subscribe, publish, dedup, idempotencia, nats.go, event bus.
---

# NATS JetStream (nats.go)

Recordá: nuestro bus es **JetStream** (durable streams + replay), no NATS Core puro. Distínguelo siempre.

## Flujo base

```go
nc, _ := nats.Connect(natsURL)
js, _ := nc.JetStream()

// Stream durable con límites
_, err := js.AddStream(&nats.StreamConfig{
    Name:      "TELEMETRY",
    Subjects:  []string{"telemetry.>"},
    Retention: nats.LimitsPolicy,
    Storage:   nats.FileStorage,
    MaxAge:    time.Hour * 24,
})

// Publisher con idempotencia lado servidor
js.Publish("telemetry.pos", data, nats.MsgId(eventID))
```

- `nats.MsgId(event.id)` habilita la **deduplication window** del stream (default ~2min). Ese es nuestro primer filtro de duplicados.
- Filtro de seguridad: la DB hace `INSERT ... ON CONFLICT (event_id) DO NOTHING` como red.

## Consumidor durable + ack

```go
_, err := js.Subscribe("telemetry.>", handler, nats.Durable("ingestor"), nats.ManualAck())
```

- `ManualAck()` obliga a `msg.Ack()` explícito: el ingestor ackea solo tras persistir. Esto da **at-least-once** sin perder eventos.
- Fallo transitorio → `msg.Nak()` (reintento/backoff); mensaje definitivamente inválido → `msg.Term()` (muerte silenciosa, no bloquea el stream).
- Procesa con workers acotados: `nats.MaxDeliver`, `nats.AckWait`; controla el número de goroutines para backpressure.

## Concurrencia y backpressure

- Batch inserts a TimescaleDB (pgx `CopyFrom` o prepared batch), NUNCA un INSERT por evento.
- El consumer no vuela: si la DB va lenta, deja que acks se atrasen (replay natural) antes de drenar los streams.

## Trazabilidad

Todo evento lleva: `device_id`, `event_id` (UUID), `occurred_at` (RFC3339), `source`. El wrapper de publicación lo garantiza; nunca publiques JSON crudo sin estos campos.

## Cosas que reviso en código

- ¿Usa `nc.JetStream()` y streams duraderos, o `nc.Publish` a un topic crudo?
- ¿Hay `MsgId` en publish y `ON CONFLICT (event_id)` en DB?
- ¿Todos los consumers son durables con ack manual y manejo de Nak/Term?
- ¿Timeouts/circuit breaker en publish (gobreaker) para no cuelgar el ingestor si NATS responde lento?