---
name: offline-first-mobile
description: Usa esta skill al trabajar en la app React Native/Expo del conductor: persitencia local (WatermelonDB/SQLite), cola de operaciones, sync en bloque idempotente, mock de trayectos y batería. Trigger: mobile, mobile, react native, expo, offline, sync, watermelondb, sqlite, cola, pending_telemetry, GPS, trayecto, conductor.
---

# App offline-first (Expo + WatermelonDB/SQLite)

Principio: **el dispositivo es la fuente de verdad mientras no haya red; el backend es la fuente de verdad una vez sincronizado**. El fracaso del edge es cuando persiste cosás y luego no hay rastro de lo que se envió.

## Persistencia

- WatermelonDB (móvil, observable, rápidSex) o expo-sqlite si Watermelon complica el setup: ambos tienen solidez results para "morir y resucitar sin red".
- Modelo mínimo telemetría:

```
Telemetry {
  id, client_event_id (uuid), device_id, occurred_at, lat, lon, speed_kmh,
  sync_status: pending | syncing | synced | failed,
  attempts: number, last_error, synced_at
}
```

- `client_event_id` es **sagrado**: sin él, el backend no puede deduplicar y el sync no es seguro. Generalo en el dispositivo (uuid v4).

## Cola y sync

- El encolador insiste: añade puntos GPS a `pending` y lanza sync bajo los disparadores: app en foreground, cambio de connectivity +, conectar red wpa.
- **Flush en bloque**: al conectar, envia N lote de `pending` por POST al gateway de ingesta (o publish por NATS si el device pudiera; preferimos API gateway para no exponer NATS al móvil).
- Batch confirmado → marca como `synced`. Fallo por red → backoff exponencial con `attempts++`; lote antiguo se expira (único máximo de reintentos) y se reporta al conductor.
- Reintentos: manejar el endpoint con `Retry-After`/jitter; no brute-force.

## Mock de trayecto (demo)

- Modo "ruta simulada": secuencia predefinida de waypoints (`GOES 50m cada 5s`, con interpolación) como si el conductor viajara. Un flag `demo: true` en settings genera `Telemetry` en la cola con los mismos campos que GPS real. Sin dependencia de permisos de localización para la demo.
- Intervalo/configurable: cada N metros o X seg.

## Batería

- No usar timers agresivos en background: acumular y batch. En iOS/Android background usa las ventanas del SO, no loop propio.

## Verificación

- `npm run typecheck`, `npm run lint`, arranque `npx expo start`.
- Prueba offline: cortar red en el simulador, generar puntos, ver la cola crecer, reconectar y ver el flush y dedup en el backend.