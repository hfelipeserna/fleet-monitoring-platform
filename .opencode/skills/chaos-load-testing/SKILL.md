---
name: chaos-load-testing
description: Usa esta skill al crear o correr scripts de prueba de carga/caos con k6: simular cientos de vehículos, inyectar 10% de peticiones duplicadas y 5% de errores/payloads inválidos, y validar que la dedup y la resiliencia funcionan. Trigger: k6, load, carga, caos, chaos, duplicados, errores, inyección, throughput, latencia, benchmark.
---

# Caos y carga con k6

k6 = scripting en JS, corridas locales o CI, métricas precisas. El objetivo no es solo "no crashea": es **demostrar resiliencia**: duplicados no se duplican, errores no rompen el stream, y el throughput se mantiene dentro de presupuesto.

## Escenario fijado (requisito de la prueba)

- **Cientos de vehículos** simultáneos (por ej. 300 VU, cada VU = un vehículo).
- **10% de peticiones duplicadas**: 1 de cada 10 mensajes reenvía el mismo `client_event_id` con el mismo payload. La aserción clave: el backend acepta ambos pero en DB solo hay una fila por `event_id`.
- **5% de errores/payloads inválidos**: mensajes con campos faltantes, mal formados, o de device desconocido. El backend debe responder `4xx` sin crashear ni contaminar el stream.

## Estructura del script (`tests/load/k6-*.js`)

```js
export const options = {
  scenarios: {
    fleet: { executor: 'constant-vus', vus: 300, duration: '5m' },
  },
  thresholds: {
    http_req_duration: ['p(95)<250'],   // presupuesto de latencia
    http_req_failed: ['rate<0.02'],     // 5% inyectado se rechaza pero nada más falla
    checks: ['rate>0.99'],
  },
};
```

- Cada VU: genera lat/lon ligeramente cambiantes (ruta simulada), `client_event_id` = `uuidv4()`, y decide duplicado/error según probabilidad.
- Usá checks: `response.status === 202` en el happy path, `status === 409/400` en duplicados/inválidos (diseño del endpoint), y la verificación de no-duplicado vía endpoint de cuenta de telemetría o query a la DB.
- Variantes: `ramp-up` para ver degradación; y un segundo script que tire **errores de red/downtime** (apagá NATS/DB unos segundos) para verificar el circuit breaker y el NAK del consumidor (cola no se pierde).

## Qué reporto al final

1. Requests/sec y p95 latencia por etapa (baseline, duplicados, errores).
2. Count de duplicados aceptados y únicos en DB (dedup funcionando).
3. Comportamiento bajo caos: cuántos mensajes se perdieron (0), cuántos se reintentaron.
4. Link/snippet a los thresholds para CI.

## Reglas

- El script debe correr contra un entorno levantado (`docker compose up -d`) y poder correr en CI contra un stack efímero.
- Nada de secrets en el script.