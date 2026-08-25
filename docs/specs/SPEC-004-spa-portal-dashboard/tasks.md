# Tasks — SPEC-004: SPA Portal refine (Monitoring + Critical zones con dibujo real)

> Derivado de `plan.md` §12 (7 Steps). Cada Task = unidad asignable a un especialista. Todo commit cita `SPEC-004` y el `AC` que cubre. Estado: `pending | in_progress | done`.

## Resumen de trazabilidad

| Task | Step (plan.md) | Especialista | FR/BR/AC | TS/TEST | Depende de |
|------|----------------|--------------|----------|---------|------------|
| TASK-004-01 | Step 1 Plate + Card | `react-web` + `test-engineer` | FR-001/002, BR-001/002/006/007/009/010 -> AC-001/002/003 | TS-001/002 -> TEST-001/002 | — |
| TASK-004-02 | Step 2 Fleet stream + Clear | `react-web` + `test-engineer` | FR-001/003, BR-008 -> AC-001/010 | TS-009 -> TEST-009 | 01 |
| TASK-004-03a | Step 3a Bottom Alerts fijo | `react-web` + `test-engineer` | FR-004/005, BR-005/007 -> AC-004 | TS-003 -> TEST-003 | 02 |
| TASK-004-03b | Step 3b Bottom Chat AI fijo | `react-web` + `test-engineer` | FR-004/006, BR-011 -> AC-005 | TS-004 -> TEST-004 | 03a |
| TASK-004-04 | Step 4 Top tabs layout | `react-web` + `test-engineer` | FR-007, BR-014/015 -> AC-011 | TS-010 -> TEST-010 | 03b |
| TASK-004-05 | Step 5 Zones list + Map + Geoman draw | `react-web` + `test-engineer` | FR-008/009, BR-004/012 -> AC-006/007 | TS-005/006 -> TEST-005/006 | 04 |
| TASK-004-06 | Step 6 Modales Create/Edit/Delete | `react-web` + `test-engineer` | FR-010/011, BR-012/013 -> AC-007/008/009 | TS-007/008 -> TEST-007/008 | 05 |
| TASK-004-07 | Step 7 Polish a11y + depguard + coverage | `react-web` + `test-engineer` + `frontend-auditor` + `reviewer` | Transversal BR-015 NFR-006/008 -> AC-012 | TS-011 -> TEST-011 | 01..06 |

---

### TASK-004-01 — Plate regex + Vehicle card (Moving/Idle + Last update)
**Especialista:** `react-web` (impl) + `test-engineer` (RED)
**Goal (Step 1):** Input `Plate [____] [Search]` valida `PLATE_RE=/^[A-Z]{3}[0-9]{3}$/` (disable + hint), card `Plate/Lat/Lon/Speed⚠️/Status/Last update` con `Moving #16a34a` vs `Idle #dc2626`, `⚠️` si `speed>80`, `Last update HH:mm:ss`, `placa no encontrada` mantiene flota.
**Spec refs:** UC-001, FR-001/002, BR-001/002/006/007/009/010, AC-001/002/003, TS-001/002
**Archivos:** `web/src/lib/plate.ts`, `web/src/features/monitoring/VehicleSearch.tsx`, `VehicleCard.tsx`, `VehicleStatusBadge.tsx`
**Tests (TDD RED primero):**
- `lib/plate.test.ts` `isValidPlate("TTF67")==false`, `TTF678==true` // AC-003
- `features/monitoring/VehicleCard.test.tsx` `speed 0 -> Idle red` `speed 90 -> Moving green + ⚠️` `Last update from received_at` `notFound -> "placa no encontrada" + flota` // AC-001/002 AAA
**Validation:** `npm test -- --run lib/plate` RED->GREEN, `npm run lint` (`tsc --noEmit`)
**Audit gates:** `frontend-auditor` (a11y label, contraste AA, hint) + `reviewer` (depguard)
**Done:** tests verdes citando `// AC-001` + auditor sin altas.

### TASK-004-02 — Fleet stream filtrado ?plate + Clear vehicle info
**Especialista:** `react-web` + `test-engineer`
**Goal (Step 2):** `Search TTF678` suscribe `EventSource /api/fleet/positions/stream?plate=TTF678` centra `map.setView` y anima marker, `Clear` resetea `selectedPlate=null` y reconecta `stream` sin plate (todos cluster).
**Spec refs:** UC-001, FR-001/003, BR-008, AC-001/010, TS-009
**Archivos:** `web/src/hooks/useFleetStream.ts`, `store/fleetStore.ts`, `store/portalStore.ts`, `App.tsx`
**Tests:**
- `hooks/useFleetStream.test.tsx` `sin plate -> todos`, `?plate=TTF678 -> solo ese`, `Clear -> sin plate` + `map.setView` spy // AC-010
**Validation:** `curl /api/fleet/positions?plate=TTF678`, `curl -N /api/fleet/positions/stream?plate=TTF678`
**Audit gates:** `frontend-auditor` (cleanup unmount, no re-render full fleet) + `quality-auditor` O(n)
**Dep:** TASK-004-01

### TASK-004-03a — Bottom tab Alerts fijo con scroll + SSE
**Especialista:** `react-web` + `test-engineer`
**Goal (Step 3a):** Panel `Alerts` `h-[280px] lg:h-[340px] overflow-y-auto` no crece con texto, `aria-live=polite`, consume `GET /api/alerts` 4 tipos `speeding_on/off` con `:ping 15s`.
**Spec refs:** UC-002, FR-004/005, BR-005/007, AC-004, TS-003
**Archivos:** `web/src/features/monitoring/AlertsPanel.tsx`, `hooks/useAlertsSSE.ts`
**Tests:** `AlertsPanel.test.tsx` 2 msgs en <2s, `expect(panel).toHaveClass("overflow-y-auto")` `h-[280px]`, `aria-live` // AC-004
**Validation:** `npm test -- --run AlertsPanel`
**Audit gates:** `frontend-auditor` (fijo + aria-live) + `reviewer` (sin PII)
**Dep:** TASK-004-02

### TASK-004-03b — Bottom tab Chat AI fijo embed
**Especialista:** `react-web` + `test-engineer`
**Goal (Step 3b):** Tab `Chat AI` misma altura fija que Alerts, embebe `ChatWidget` reutilizado (`POST /api/chat` markdown + citations), input + botón azul `↩`, `429 Retry-After`.
**Spec refs:** UC-003, FR-004/006, BR-011, AC-005, TS-004
**Archivos:** `web/src/features/monitoring/ChatTab.tsx`
**Tests:** `ChatTab.test.tsx` click tab -> panel fijo, send "hola" -> `POST /api/chat` 200 markdown, `429` inline // AC-005
**Validation:** `npm test -- --run ChatTab`
**Audit gates:** `frontend-auditor` (input focus, botón azul, scroll aislado)
**Dep:** TASK-004-03a

### TASK-004-04 — Top tabs Monitoring|Critical zones + layout proporcional
**Especialista:** `react-web` + `test-engineer`
**Goal (Step 4):** Store `activeTop:'monitoring'|'zones'` Zustand, toggle sin reload, `Monitoring grid lg:grid-cols-2` 50/50, `Critical zones grid 35/65`, activo negro `#1f2937`.
**Spec refs:** UC-005, FR-007, BR-014/015, AC-011, TS-010
**Archivos:** `web/src/store/portalStore.ts`, `web/src/App.tsx`
**Tests:** `App.test.tsx` click `Monitoring<->Critical zones` sin reload, `expect(panel).toHaveClass("overflow-y-auto")` // AC-011
**Validation:** `npm run build` + manual toggle `http://localhost:5173`
**Audit gates:** `frontend-auditor` (Figma fidelity, responsive)
**Dep:** TASK-004-03b

### TASK-004-05 — Zones list + Map + Geoman draw draft
**Especialista:** `react-web` + `test-engineer`
**Goal (Step 5):** `Zones list` `h-[360px] lg:h-[480px] overflow-y-auto` filas `Zone N` alterna verde/celeste `key={id}`, `Map` `GeoJSON red fillOpacity 0.2`, `ZoneDrawControl` Geoman `pm:create` draft `setDraftPolygon`, `Create zone` disabled sin draft.
**Spec refs:** UC-004, FR-008/009, BR-004/012, AC-006/007, TS-005/006
**Archivos:** `web/src/features/zones/ZonesList.tsx`, `ZonesMap.tsx`, `ZoneDrawControl.tsx`
**Tests:** `ZonesList.test.tsx` `zones 0 -> Create disabled + sin polygon`, `4 zonas -> 4 filas + 4 polygons`, `ZoneDrawControl.test.tsx` draft enable // AC-006/007
**Validation:** `GET /api/zones` manual + draw en mapa
**Audit gates:** `frontend-auditor` (draft aislado, `key={id}`, rojo 0.2)
**Dep:** TASK-004-04

### TASK-004-06 — Modales Create/Edit/Delete zones
**Especialista:** `react-web` + `test-engineer`
**Goal (Step 6):** `CreateZoneModal` centrado `bg-black/50` `role=dialog aria-modal` `Zone name [input] [Accept][Cancel]` -> `POST /api/zones 201` (400/409 inline), `Cancel` descarta draft; `EditZoneModal` dblclick prefill -> `PUT 200` Rename / `DELETE 204` Delete / `Cancel` sin API.
**Spec refs:** UC-004, FR-010/011, BR-012/013, AC-007/008/009, TS-007/008
**Archivos:** `web/src/features/zones/CreateZoneModal.tsx`, `EditZoneModal.tsx`
**Tests:** `CreateZoneModal.test.tsx` `201 refresh` vs `400/409 inline no cierra`, `EditZoneModal.test.tsx` `dblclick -> Rename 200 Delete 204 Cancel` // AC-007/008/009
**Validation:** modal via UI + `curl -X POST /api/zones`
**Audit gates:** `frontend-auditor` (focus trap, `Esc`, error 409 bajo input, Cancel descarta) + `security` (rate 10/min)
**Dep:** TASK-004-05

### TASK-004-07 — Polish a11y + depguard + coverage + auditoría final
**Especialista:** `react-web` + `test-engineer` + `frontend-auditor` + `reviewer` + `security`
**Goal (Step 7):** `role=dialog` + `aria-label` + focus trap, `depguard` 0 `pgx/nats/genkit` en `web`, `npm test --coverage >60%`, `docker compose config -q` + `npm run build` + `golangci-lint` verdes.
**Spec refs:** Transversal, NFR-006/008, AC-012, TS-011
**Archivos:** `web/src/lib/depguard.test.ts`, a11y fixes
**Tests:** `depguard.test.ts` + axe a11y // AC-012
**Validation:** `npm test -- --coverage --run`, `docker compose config -q`, `npm run build`
**Audit gates (final):** `frontend-auditor` checklist completo (React/perf/WCAG/Design/UX/Figma), `reviewer` sin altas, `security` sin secretos — bloquea cierre si alta + registra `docs/IAUDIT.md`.
**Dep:** TASK-004-01..06

---

## Orden de ejecución sugerido (para PRs atómicos)

1. `TASK-004-01` solo -> PR1
2. `TASK-004-02` -> PR2 (depende 01)
3. `TASK-004-03a + 03b` -> PR3
4. `TASK-004-04` -> PR4
5. `TASK-004-05 + 06` -> PR5 (o separar si se quiere: 05 luego 06)
6. `TASK-004-07` -> PR6 cierre + `IAUDIT`

## Validación global (DoD)

- [ ] `FR-001..012` con AC trazable y `TS->TEST` verde
- [ ] `tcs --noEmit` `go vet` `docker compose config` + `npm run build` por Step
- [ ] Gates: `frontend-auditor` por Step (web/**) + `reviewer/security` — ver `plan.md §12.1`
- [ ] `docs/IAUDIT.md` >=1 hallazgo si IA propuso `leaflet-draw` sin cierre
- [ ] Commits `feat(spa): ... [SPEC-004]` Conventional Commits
