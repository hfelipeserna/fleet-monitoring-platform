---
name: offline-first-mobile
description: Usa esta skill al trabajar en la app React Native/Expo del conductor: persistencia local (WatermelonDB/SQLite), cola de operaciones, sync en bloque idempotente, mock de trayectos y batería. Trigger: mobile, react native, expo, offline, sync, watermelondb, sqlite, cola, pending_telemetry, GPS, trayecto, conductor, plate, disconnect, toggle, NetInfo, syncing, WatermelonDB, Medellín, Bogotá.
---

# App offline-first (Expo + WatermelonDB/SQLite) — SPEC-005

Principio: **el dispositivo es la fuente de verdad mientras no haya red; el backend es la fuente de verdad una vez sincronizado**. El fracaso del edge es persistir y luego no tener rastro de lo enviado. Referencia canónica: `docs/specs/SPEC-005-mobile-offline-first/spec.md` + `plan.md` + `tasks.md`.

## Persistencia

- WatermelonDB (observable, O(1) query, JSI) con adapter `expo-sqlite`; fallback a `expo-sqlite` puro si JSI bloquea `expo go` — misma API de dominio (`enqueue/getPending/markSynced/clearPending/countPending`).
- Modelo SPEC-005 FR-008/BR-008:

```
pending_telemetry {
  id: string (Watermelon auto),
  client_event_id: string (uuid v4, indexed, sagrado),
  plate: string (^[A-Z]{3}[0-9]{3}$, indexed),
  lat: number | null, lon: number | null (validasi [-90,90]/[-180,180]),
  speed: number (int >=0),
  occurred_at: number (ms epoch),
  sync_status: 'pending'|'syncing'|'synced'|'failed',
  attempts: number, last_error?: string, synced_at?: number
}
appSchema v1 + migrations; Watermelon file en documentDirectory sandbox, sobrevive a kill (AC-009 245 restores).
```

- `client_event_id` es **sagrado**: UUID v4 vía `react-native-get-random-values` + `uuid`. Sin él, dedup `Nats-Msg-Id` 2m + `ON CONFLICT DO NOTHING` del backend rompe y reconexión duplica. Nunca secuencial/predecible.
- SSOT placa: `lib/plate.ts` exporta `PLATE_RE=/^[A-Z]{3}[0-9]{3}$/`, `normalizePlate`, `isValidPlate`. Input `PlateInput` hace `toUpperCase+trim`, `Connect` disabled hasta match, hint `"3 letras + 3 números"` (FR-001/BR-001).

## Máquina de estados (FR-002/BR-003)

```
idle (input vacío, BD vacía, toggle OFF gris deshabilitado)
  --[input ACF356 valid + Connect click]--> connecting
connecting: valida WatermelonDB OK + Network OK + POST /v1/telemetry {plate,lat,lon,speed,client_event_id} timeout 5s
  --[202]--> connected (Syncing CONNECTED rojo, toggle OFF habilitado)
  --[timeout 5s /400/429/503/Network ERROR/DB ERROR]--> error (Syncing ERROR rojo)
error --[Disconnect]--> idle (purga BD + abort + clear interval + input "")
connected --[Network ERROR persistente sin 202]--> error
```

`store/appStore.ts` (zustand) guarda `conn: idle|connecting|connected|error, sync: CONNECTING|CONNECTED|ERROR, net: OK|ERROR, db: OK|ERROR, simEnabled/simOn/selectedRoute`. `useConnection.ts` orquesta `Promise.race([postSingle, sleep(5000)])` con `AbortController`.

## Dos conceptos desacoplados (FR-003/BR-004)

- `Network connectivity OK/ERROR` desde `expo-netinfo` (`NetInfo.addEventListener` + `fetch` inicial; cleanup en unmount). No hace `fetch` si `ERROR`.
- `Syncing data ... CONNECTING/CONNECTED/ERROR` desde respuesta `202` del LB. `WatermelonDB status OK/ERROR` tercer dot.
- Desacoplados: avión -> `Network ERROR` + `Syncing ERROR` + `DB OK`; backend `503` con red -> `Network OK` verde + `Syncing ERROR` rojo. `StatusPanel.tsx` fiel a wireframes `#dc2626` Syncing rojo, `#16a34a` OK verde.

## Cola y sync (FR-008/009, BR-008/009)

- Encolado cada `5s`: `simOn ? nextSimPoint(route) : nextGpsPoint()` (`expo-location` solo en `ON->OFF`, no necesario para demo). Flush cada `5s` o `pending>=50` si `connected && net OK`.
- **Flush en bloque**: `lib/sync.ts` `flushPending` -> `getPending(500)` -> `postBatch({events:1..500})` (`lib/api.ts` base `EXPO_PUBLIC_API_URL` vía `Constants.expoConfig.extra.apiUrl` + env, timeout 5s `AbortController`).
  - `202 {accepted:N}` -> `markSynced`/`delete`, `setSync('CONNECTED')`.
  - `429/503` -> lee `Retry-After` (default `5`), throw retry, `useSync.ts` backoff `5s*2^attempts` cap `60s` + jitter `0-1s`, `attempts++`, no vacía.
  - `attempts>=5 -> failed`, `last_error="503 backpressure"` (AC-010).
  - `400 all-or-nothing` mantiene pending.
- Backpressure y quota: `12/min burst20` por placa (SPEC-001 BR-005) -> batch cuenta 1 req, `5s/50` respeta `500/30s`.
- Idempotencia: `client_event_id` -> `Nats-Msg-Id` dedup 2m; sobrevive a kill y a 245 reconexión (TS-009).

## Ruta simulada (FR-005/006/007, BR-006/007)

- `RouteToggle.tsx` `<Switch disabled={conn!=='connected'} testID="sim-toggle">` `Activar ruta simulada OFF` gris `#e5e7eb` -> `ON` verde `#16a34a`.
- `ON` habilita `RouteButtons.tsx` Medellín/Bogotá azul `#93c5fd`; click -> `await db.clearPending(); idx=0; setSelected(route)` + botón seleccionado verde `#86efac`.
- `routes/medellin.ts`/`bogota.ts` ~20 pts cada una `lat 6.2442,-75.5812 / 4.7110,-74.0721` `speed 0/45/85` (para `speeding_on/off`, `zone_enter/exit` en web). `useSimulatedRoute.ts` `nextSimPoint` idx circular + `toggleSim(false)` purga + `setSimOn(false)` y pasa a `useTelemetryGenerator` GPS real.
- Fidelidad 6 wireframes: `Connect #86efac / Disconnect #f9a8d4 / Syncing #dc2626 / OK #16a34a / ERROR #dc2626 / ruta gris #e5e7eb -> azul #93c5fd -> verde #86efac`, fonts sketch, touch >=44pt.

## Batería y performance

- No timers agresivos en background; `setInterval 5s` solo en foreground `connected`. En iOS/Android usa ventanas SO, no loop propio. Flush batch no brute-force (jitter).

## Testing (TDD AAA, plan §11)

- Unit/component: `lib/plate.test.ts`, `PlateInput.test.tsx` (AC-001), `useConnection.test.tsx` (AC-002, msw 202 vs timeout), `StatusPanel.test.tsx` + `useNetInfo.test.tsx` (AC-003/010 desacoplados), `appStore.test.tsx` Disconnect (AC-004), `RouteToggle.test.tsx` (AC-005), `RouteButtons.test.tsx` (AC-006/007).
- Integration: `lib/sync.test.ts` 60 offline -> batch 50 202 + 429 backoff (AC-008/010), `db/telemetry.test.ts` 245 kill restores (AC-009), `503` Network OK + Syncing ERROR.
- Stack: `jest` + `@testing-library/react-native` + `msw` + WatermelonDB mock. Cada test con `// Arrange // Act // Assert` + `// Covers AC-XXX`.
- Visual: `App.test.tsx` snapshot 6 wireframes colores.

## CI/CD móvil

- Ver skill `expo-workflow` + `iac-and-cicd`: `eas.json` dev/preview/prod, `fastlane/` Appfile/Fastfile, `.github/workflows/mobile.yml` path-filter `mobile/**`.

## Verificación

- `npm run typecheck` (`tsc --noEmit` strict), `npm run lint`, `npm test -- --run`, `npx expo start --tunnel` + Expo Go en cel con `EXPO_PUBLIC_API_URL=http://LAN_IP:8080` `202`.
- Offline: avión -> cola crece (245), reconecta -> `POST /batch 245 -> 202`, purga validada via `countPending()`.
