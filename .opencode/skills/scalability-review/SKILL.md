---
name: scalability-review
description: Usa esta skill al evaluar diseños y decisiones de arquitectura contra carga de miles de dispositivos: throughput, particionado, contención, backpressure, almacenamiento, resiliencia y costo. Trigger: escalabilidad, scalable, throughput, msgs/s, particionado, sharding, backpressure, hot key, storage, scale, ADR.
---

# Escalabilidad (criterios de evaluación)

Se aplica antes de fijar decisiones (ADR) y al auditar componentes críticos. La meta: dictaminar con números y condiciones de borde, no con opiniones.

## Carga de referencia de la prueba

| Variable | Valor típico |
|---|---|
| Dispositivos | 1.000+ |
| Tasa de publicación | 1 posición/5–15 s por dispositivo, pico |
| Flujo | móvil → API ingesta → NATS JetStream → consumidor → TimescaleDB → SSE/IA |

Fórmula pico rápida: `vehículos ÷ intervalo_medio` mensajes/s (ej. 1.000/6 s ≈ 167 msg/s, trivial; 5.000/5 s = 1.000 msg/s). Proyectá también bytes/día: `msg × payload_bytes`.

## Los 6 ejes a evaluar

1. **Throughput agregado y por componente**: ¿gateway, stream, consumer y DB absorben el pico? Estimación numérica por etapa.
2. **Particionado / scale-out**: ¿el componente escala horizontal? JetStream subject por `device_id` + consumers durables balanceados; hypertable por tiempo con opción de sharding por `device_id`; gateway stateless.
3. **Contención / hot keys**: NATS mononodo como punto único; un solo writer/connection pool de DB; índices ausentes → latencia `O(n)` en `WHERE device_id AND time`.
4. **Backpressure**: dependencias vía colas con ack/Nak + `MaxDeliver` y workers acotados. Si el consumer va lento, el stream acota y replaya; el ciclo no se pierde.
5. **Storage y costo**: raw estado pico → bytes/día; con compresión Timescale (~10–20x) y retention, cuánto. Continuous aggregates para no escanear chunks viejos.
6. **Resiliencia en escala**: reinicio de NATS/DB en pico → replay/reintentos duplican carga; circuit breakers (gobreaker) evitan cascada; `Nats-Msg-Id` + `ON CONFLICT (event_id)` hacen el replay idempotente.

## Anti-patrones a marcar (severidad según contexto)

- DB en línea caliente: writes individuales por evento en vez de batch.
- Lime el transport sin backpressure (colas "ilimitadas" que acumulan).
- Todo el dashboard re-consultando raw en vez de aggregates.
- Punto único no justificado (NATS mononodo es OK para MVP si hay ADR y plan de particionado).
- **Sobre-ingeniería**: construir sharding/cluster de 3 nodos para 167 msg/s en dev con 16 GB RAM — señalada como hallazgo "no escalable" al revés (costo de operar sin carga).

## Output del dictamen

```
Decisión evaluada: ...
Veredicto: escalable | escalable con límites | no escalable
Condiciones de borde: ...
Números: ...
Recomendación de ADR: si/no — por qué
```