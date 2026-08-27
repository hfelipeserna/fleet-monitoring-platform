# Tasks — SPEC-005: App móvil offline-first (Expo + WatermelonDB)

> Derivado de `plan.md` §12 (9 Steps). Cada Task = unidad asignable a un especialista. Todo commit cita `SPEC-005` y el `AC` que cubre. Estado: `pending | in_progress | done`.

## Resumen de trazabilidad

| Task | Step (plan.md) | Especialista | FR/BR/AC | TS/TEST | Depende de |
|------|----------------|--------------|----------|---------|------------|
| TASK-005-01 | Step 1 Plate regex + input | `mobile-expo` + `test-engineer` | FR-001, BR-001/002 -> AC-001 | TS-001 -> TEST-001 | — |
| TASK-005-02 | Step 2 Conexión idle->connecting->connected/error | `mobile-expo` + `test-engineer` | FR-002/003, BR-003/004 -> AC-002 | TS-002 -> TEST-002 | 01 |
| TASK-005-03 | Step 3 Status desacoplados + WatermelonDB OK | `mobile-expo` + `test-engineer` | FR-003, BR-004 -> AC-003/010 | TS-003/010 -> TEST-003/010 | 02 |
| TASK-005-04 | Step 4 Disconnect purga | `mobile-expo` + `test-engineer` | FR-004, BR-005 -> AC-004 | TS-004 -> TEST-004 | 02 |
| TASK-005-05 | Step 5 Toggle Activar ruta simulada | `mobile-expo` + `test-engineer` | FR-005, BR-006 -> AC-005 | TS-005 -> TEST-005 | 02 |
| TASK-005-06 | Step 6 Rutas Medellín/Bogotá | `mobile-expo` + `test-engineer` | FR-006/007, BR-007/008 -> AC-006 | TS-006 -> TEST-006 | 05 |
| TASK-005-07 | Step 7 Sync batch + offline 245 + persist kill | `mobile-expo` + `test-engineer` | FR-008/009, BR-008/009/010 -> AC-008/009/010 | TS-008/009/010 -> TEST-008/009/010 | 02, 06 |
| TASK-005-08 | Step 8 ON->OFF GPS real | `mobile-expo` + `test-engineer` | FR-006, BR-007 -> AC-007 | TS-007 -> TEST-007 | 06 |
| TASK-005-09 | Step 9 EAS/Fastlane + GH Actions + polish 6 wireframes | `mobile-expo` + `devops` + `test-engineer` + `reviewer` + `security` | FR-010/011/012, BR-010/011 -> AC-011/012 | TS-011/012 -> TEST-011/012 | 01..08 |

---

### TASK-005-01 — Plate regex + input (BR-001/002)
**Especialista:** `mobile-expo` (impl) + `test-engineer` (RED)
**Goal (Step 1):** Input `Plate [____]` valida `PLATE_RE=/^[A-Z]{3}[0-9]{3}$/`, normaliza `toUpperCase+trim`, `Connect` verde `#86efac` deshabilitado hasta match, hint `"3 letras + 3 números"`, sin placa no hay `fetch` ni BD con datos.
**Spec refs:** UC-001, FR-001, BR-001/002, AC-001, TS-001
**Archivos:** `mobile/src/lib/plate.ts`, `mobile/src/components/PlateInput.tsx`, `mobile/src/App.tsx` (idle), `mobile/app.json`, `mobile/package.json` init
**Tests (TDD RED primero):**
- `mobile/src/lib/plate.test.ts` `normalizePlate("acf356")==="ACF356"`, `isValidPlate("ACF356")==true`, `isValidPlate("ACF35")==false` // AC-001
- `mobile/src/components/PlateInput.test.tsx` `type acf356 -> ACF356 + Connect enabled verde`, `type ACF35 -> disabled + hint visible`, `sin placa -> no fetch` // AC-001 AAA
**Validation:** `npm test -- --run plate` RED->GREEN, `npx expo start --tunnel` manual input, `npx tsc --noEmit`
**Audit gates:** `reviewer` (no secreto `EXPO_PUBLIC_`, SSOT plate) + `quality-auditor` (plate single source)
**Done:** tests verdes citando `// AC-001` + auditor sin altas.

### TASK-005-02 — Conexión idle->connecting->connected/error
**Especialista:** `mobile-expo` + `test-engineer`
**Goal (Step 2):** Máquina `idle -> connecting -> connected -> error`. `Connect` click -> `connecting` muestra `Syncing data ... CONNECTING` rojo `#dc2626` + valida `WatermelonDB OK` + `Network OK` + `POST /v1/telemetry {plate, lat, lon, speed, client_event_id}` con timeout 5s; `202` -> `connected` `Syncing CONNECTED` + habilita toggle `OFF`; `timeout/400/429/503` -> `error` `Syncing ERROR` rojo.
**Spec refs:** UC-001, FR-002/003, BR-003/004, AC-002, TS-002
**Archivos:** `mobile/src/store/appStore.ts` (zustand `conn: idle|connecting|connected|error, sync, net, db`), `mobile/src/hooks/useConnection.ts`, `mobile/src/lib/api.ts` (`postTelemetry` + `AbortController` 5s), `mobile/src/components/StatusPanel.tsx` (Syncing)
**Tests:**
- `mobile/src/hooks/useConnection.test.tsx` `idle->connecting 202 -> connected + toggle habilitado`, `no 202 5s -> error + Syncing ERROR`, `400/429/503 -> error` // AC-002 msw fetch 202 mock
**Validation:** Expo Go `Connect TGY589` manual, `curl /v1/telemetry` via LB, `Network` mock avion
**Audit gates:** `reviewer` (error wrap `%w`, `AbortController` leak) + `security` (no `NATS` key en móvil, `EXPO_PUBLIC_API_URL` env)
**Dep:** TASK-005-01

### TASK-005-03 — Status desacoplados: Network vs Syncing + WatermelonDB
**Especialista:** `mobile-expo` + `test-engineer`
**Goal (Step 3):** Tres dots independientes: `WatermelonDB status ○ OK/#16a34a ERROR/#dc2626`, `Network connectivity ○ OK/ERROR` desde `expo-netinfo`, `Syncing data ... CONNECTING/CONNECTED/ERROR` desde `202`. Desacoplados: `Network OK + Syncing ERROR` si `503`; `Network ERROR + Syncing ERROR + DB OK` si avión.
**Spec refs:** UC-001/005, FR-003, BR-004, AC-003/010, TS-003/010
**Archivos:** `mobile/src/db/schema.ts`, `mobile/src/db/index.ts` (WatermelonDB init), `mobile/src/hooks/useNetInfo.ts` (`NetInfo.addEventListener` + `fetch`), `mobile/src/components/StatusPanel.tsx`
**Tests:**
- `mobile/src/components/StatusPanel.test.tsx` `connected -> Network OK verde + Syncing CONNECTED rojo`, `avion ON -> Network ERROR rojo + Syncing ERROR`, `503 con Net OK -> Network OK + Syncing ERROR` // AC-003/010 AAA
- `mobile/src/hooks/useNetInfo.test.tsx` listener registra y limpia en unmount
**Validation:** modo avión simulador, backend `503` via `docker compose stop ingest` o `msw 503`, dots colores spec
**Audit gates:** `quality-auditor` (NetInfo listener leak, WatermelonDB init <1s) + `reviewer`
**Dep:** TASK-005-02

### TASK-005-04 — Disconnect purga BD + reset idle
**Especialista:** `mobile-expo` + `test-engineer`
**Goal (Step 4):** `Disconnect` rosa `#f9a8d4` limpia `pending_telemetry` (DELETE), aborta `fetch` en vuelo, detiene intervalo generación, limpia input `""`, resetea `idle`, `Connect` vuelve verde disabled, `Activar ruta simulada` vuelve `OFF` gris deshabilitado y rutas grises.
**Spec refs:** UC-003, FR-004, BR-005, AC-004, TS-004
**Archivos:** `mobile/src/db/telemetry.ts` (`clearPending()`), `mobile/src/store/appStore.ts` `disconnect()`
**Tests:**
- `mobile/src/store/appStore.test.tsx` `given connected 20 pending + toggle ON, when Disconnect -> 0 pending + input "" + idle + OFF gris + intervalo stopped` // AC-004 mock WatermelonDB count
**Validation:** Expo Go `Disconnect` y `SELECT count(*)` 0, `Connect` disabled de nuevo
**Audit gates:** `reviewer` (abort + clear atómico, no race) + `quality-auditor`
**Dep:** TASK-005-02

### TASK-005-05 — Toggle Activar ruta simulada OFF/ON
**Especialista:** `mobile-expo` + `test-engineer`
**Goal (Step 5):** `OFF` gris `#e5e7eb` deshabilitado si no `connected`; en `connected` habilitado `OFF`; `ON` verde `#16a34a` habilita rutas azules `#93c5fd` sin selección aún.
**Spec refs:** UC-004, FR-005, BR-006, AC-005, TS-005
**Archivos:** `mobile/src/components/RouteToggle.tsx` (`Switch` + `disabled={conn!=='connected'}`), `mobile/src/store/appStore.ts` `simOn`
**Tests:**
- `mobile/src/components/RouteToggle.test.tsx` `idle -> OFF gris disabled`, `connected OFF -> habilitado`, `OFF->ON -> rutas azul habilitadas` // AC-005
**Validation:** visual 6 wireframes paso 1-3, `testID="sim-toggle"` a11y
**Audit gates:** `quality-auditor` (toggle a11y, touch 44pt) — o `frontend-auditor` mobile
**Dep:** TASK-005-02

### TASK-005-06 — Rutas Medellín/Bogotá con purga + verde seleccionado
**Especialista:** `mobile-expo` + `test-engineer`
**Goal (Step 6):** Rutas predeterminadas `Medellín` (6.2442,-75.5812) y `Bogotá` (4.7110,-74.0721) ~20 pts cada una, `speed` variado `0/45/85` para `speeding_on/off`, generación cada `5s` encola `pending` con `client_event_id` uuid, `POST /batch` cada `5s` o `>=50`. Click `Medellín` -> purga previo + `Medellín verde #86efac` + `Bogotá azul #93c5fd` + secuencia desde 0; click `Bogotá` -> inverso purga. Placa se mantiene.
**Spec refs:** UC-004, FR-006/007, BR-007/008, AC-006, TS-006
**Archivos:** `mobile/src/routes/medellin.ts`, `mobile/src/routes/bogota.ts` (placeholder 20 pts, reemplazable luego), `mobile/src/hooks/useSimulatedRoute.ts` (`selectRoute`, `nextSimPoint` idx), `mobile/src/hooks/useTelemetryGenerator.ts` (`setInterval 5000`), `mobile/src/components/RouteButtons.tsx`
**Tests:**
- `mobile/src/components/RouteButtons.test.tsx` `connected ON -> click Medellin -> purga + Medellin verde Bogota azul + encolado 5s`, `click Bogota -> purga Medellin + Bogota verde reinicia 0` // AC-006 WatermelonDB mock + routes
**Validation:** seleccionar Medellín y ver `pending` 12 en 60s, cambiar a Bogotá y ver purga + 0 -> 12 de nuevo, SPA `GET /fleet/positions?plate=TGY589` aparece
**Audit gates:** `reviewer` (`client_event_id` uuid sagrado, no `AsyncStorage`) + `scalability` (batch 500/30s quota 12/min)
**Dep:** TASK-005-05

### TASK-005-07 — Sync batch idempotente + offline 245 + persist tras kill
**Especialista:** `mobile-expo` + `test-engineer`
**Goal (Step 7):** Flush en bloque `POST /batch {events:1..500}` idempotente `Nats-Msg-Id` dedup 2m + `ON CONFLICT DO NOTHING`; `202 {accepted:N}` -> `synced`/`delete`; `429/503` respeta `Retry-After:5` + backoff `5s*2^n` cap `60s` + jitter `0-1s`, no vacía, `attempts++`, `attempts>=5 -> failed`; offline 60 encola sin red, al reconectar `batch 50 + resto`; kill con 245 -> restaura 245 tras relanzar.
**Spec refs:** UC-002, FR-008/009, BR-008/009/010, AC-008/009/010, TS-008/009/010
**Archivos:** `mobile/src/lib/sync.ts` (`flushPending`), `mobile/src/hooks/useSync.ts` (intervalo 5s + trigger Net OK + pending>=50), `mobile/src/db/telemetry.ts` (`getPending`, `markSynced`, `countPending`, `attempts`)
**Tests:**
- `mobile/src/lib/sync.test.ts` `60 offline -> recover Net OK -> POST /batch 50 202 50 purged + Syncing CONNECTED`, `429 Retry-After:5 -> keeps 60 + backoff 5s*2^n`, `503 -> Network OK + Syncing ERROR + backoff 60s + attempts last_error` // AC-008/010 msw + db mock AAA
- `mobile/src/db/telemetry.test.ts` `enqueue 245 -> kill -> relaunch count 245 uniques` // AC-009 WatermelonDB
**Validation:** cortar red (NetInfo ERROR), generar 60, reconectar y ver `202 50`, `docker logs ingest` `client_event_id` dedup, kill Expo Go y relanzar count
**Audit gates:** `reviewer` + `quality-auditor` (backoff hot path, no leak, `client_event_id` sagrado) + `scalability` (dedup 2m, no OOM) + `security` (sin secreto)
**Dep:** TASK-005-02, TASK-005-06

### TASK-005-08 — ON->OFF vuelve a GPS real (expo-location)
**Especialista:** `mobile-expo` + `test-engineer`
**Goal (Step 8):** Pasar `ON->OFF` limpia buffer simulado, rutas vuelven a gris deshabilitadas, y empieza a transmitir GPS real `expo-location` cada `5s` con misma placa.
**Spec refs:** UC-004, FR-006, BR-007, AC-007, TS-007
**Archivos:** `mobile/src/hooks/useSimulatedRoute.ts` `toggleSim(false) -> clearPending + setSimOn(false) + setSelected(null)`, `mobile/src/hooks/useTelemetryGenerator.ts` `nextGpsPoint` (`Location.getCurrentPositionAsync`), `mobile/src/components/RouteToggle.tsx`
**Tests:**
- `mobile/src/components/RouteToggle.test.tsx` `given simulado Medellin verde, when ON->OFF -> purga + gris + Location mock called` // AC-007
**Validation:** permiso location en simulador, `ON->OFF` y ver lat/lon reales vs simulados, SPA verifies
**Audit gates:** `security` (permiso `expo-location`, no track sin consent) + `reviewer`
**Dep:** TASK-005-06

### TASK-005-09 — EAS/Fastlane + GH Actions + polish 6 wireframes + cierre
**Especialista:** `mobile-expo` + `devops` + `test-engineer` + `reviewer` + `security`
**Goal (Step 9):** `eas.json` dev/preview/prod, `fastlane/Fastfile` + `Appfile`, `.github/workflows/mobile.yml` path-filter `mobile/**` + `EAS Build`, polish fidelidad 6 wireframes colores `#86efac` `#f9a8d4` `#dc2626` `#93c5fd` `#16a34a` `#e5e7eb`, `npx tsc --noEmit` + `expo lint` + `docker compose config -q` verdes, `docs/IAUDIT.md` con hallazgo si IA propone `AsyncStorage`.
**Spec refs:** FR-010/011/012, BR-010/011, AC-011/012, TS-011/012
**Archivos:** `mobile/eas.json`, `mobile/app.json` (`extra.apiUrl`), `fastlane/Fastfile`, `fastlane/Appfile`, `.github/workflows/mobile.yml`, `mobile/src/App.tsx` styles, `.env.example` (`EXPO_PUBLIC_API_URL`), `README.md` (sección Expo Go LAN IP)
**Tests:**
- `mobile/src/App.test.tsx` `render 6 wireframes snapshot -> colores exactos Connect verde, Disconnect rosa, Syncing rojo, OK verde, ruta gris->azul->verde, touch 44pt` // AC-012
- `mobile/e2e` manual `EXPO_PUBLIC_API_URL=http://192.168.1.10:8080 eas build --platform android --profile preview --local` dry-run + `npx expo start --tunnel` Expo Go 202
**Validation:** `npx tsc --noEmit --project mobile/tsconfig.json`, `npm test -- --coverage --run` >60%, `docker compose config -q`, `npm run build` web intacto
**Audit gates (final):** `reviewer` sin hallazgos altos + `security` (no secreto en `eas.json`/`.env`) + `quality-auditor` (hot path) + `scalability` (batch quota) — bloquea cierre si severidad alta + registra `docs/IAUDIT.md`.
**Dep:** TASK-005-01..08

---

## Orden de ejecución sugerido (PRs atómicos)

1. `TASK-005-01` solo -> PR1 plate
2. `TASK-005-02 + 03 + 04` -> PR2 conexión + status + disconnect (máquina completa)
3. `TASK-005-05 + 06` -> PR3 toggle + rutas Medellín/Bogotá
4. `TASK-005-07` -> PR4 sync batch (hot path, auditoría pesada)
5. `TASK-005-08` -> PR5 GPS real
6. `TASK-005-09` -> PR6 cierre EAS + polish + `IAUDIT`

Alternativa atómica estricta: 01,02,03,04,05,06,07,08,09 cada uno PR separado (9 PRs). Recomendado el agrupado de 6 PRs para no bloquear CI.

## Validación global (DoD AGENTS.md)

- [ ] `FR-001..012` con `AC-001..012` trazable y `TS->TEST` verde `npm test -- --run` en `mobile/`
- [ ] `npx tsc --noEmit` `npx expo lint` `go vet ./...` `docker compose config -q` por Step
- [ ] Gates: `reviewer` sin altos + `quality-auditor` backoff + `security` location/secrets + `scalability` batch — ver `plan.md §12.1`
- [ ] `docs/IAUDIT.md` >=1 hallazgo si IA propone `AsyncStorage` sin `client_event_id` dedup
- [ ] Commits `feat(mobile): ... [SPEC-005]` Conventional Commits + `mobile/` path-filter
- [ ] `README.md` actualizado con `npx expo start --tunnel` + `EXPO_PUBLIC_API_URL` LAN IP
