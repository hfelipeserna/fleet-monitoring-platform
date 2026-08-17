---
name: realtime-dashboard
description: Usa esta skill al construir el portal web (React): dashboard reactivo con mapa, suscripción a eventos en tiempo real vía SSE, reconexión con backoff, y chat con la IA. Trigger: dashboard, SPA, React, SSE, server-sent events, mapa, leaflet, maplibre, alertas, tiempo real, realtime, EventSource, chat.
---

# Dashboard reactivo (SPA + SSE)

## Comunicación en tiempo real

- El backend expone `GET /api/events` con `Content-Type: text/event-stream`. Tipos de evento:
  - `telemetry:position` (por device), `telemetry:batch`
  - `alert:critical` (detección en el ingestor), `alert:resolved`
  - `fleet:status` (resumen), `chat:token` (streaming de la IA)
- El cliente usa `EventSource` (nativo, SSE) con reconexión automática + backoff: capturá `onerror` y reintentá con delay progresivo (500ms → 1s → 2s... cap).
- **No uses WebSocket** sin razón: SSE es server→client, más simple, HTTP standard, y cubre nuestro caso (el móvil no lo necesita, la web solo recibe).
- Estado compartido del dashboard: el cliente tiene el snapshot inicial (`GET /api/state`) y después aplica deltas por SSE. Así no re-consultas todo en cada evento.

## Mapa

- Leaflet (liviano) o MapLibre GL. Marcadores por vehículo con estado (color = estado: moving/idle/alert). Click en marcador → panel con detalles y telemetría reciente.
- Rendering: para cientos de marcadores, cluster (`leaflet.markercluster`) o canvas; no re-renderices todo el mapa en cada SSE.

## Panel de alertas y chat IA

- Alertas: lista con severidad, filtrable; al recibir `alert:critical` → highlight + sonido opcional. Siempre con timestamp del evento, no del cliente.
- Chat IA: POST al endpoint del agente, renderiza tokens de la respuesta (SSE `chat:token`) o respuesta completa. Guarda historial en memoria del cliente (contexto del turno).

## Estado y calidad

- Estado global: Zustand o TanStack Query (elegí uno; documentá). Hooks separados: `useSSE`, `useFleetMap`, `useChat`.
- Cleanup: cerrar `EventSource` y cancelar timers/backoff en unmount. Sin leaks.
- UX de fallo: si se cae el stream, mostrar "reconectando…" (no dashboard roto en silencio).
- `npm run build` y `npm run lint` en verde; TS strict.