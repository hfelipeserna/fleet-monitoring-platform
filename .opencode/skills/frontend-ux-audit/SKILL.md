---
name: frontend-ux-audit
description: Use when building or reviewing SPA React dashboard — UX audit, a11y WCAG, Tailwind design system, Figma fidelity, React performance, SSE UX. Trigger: frontend audit, UX review, a11y, accessibility, React best practices, SPA, dashboard, mapa, Tailwind, Figma, panel, modal, plate, zone.
---

# Frontend & UX Audit (React + Vite)

Checklist para auditar SPA del portal (maquetas Figma 8) antes de cerrar un task web. Complementa `reviewer` y `quality-auditor` enfocándose en **lo que ve el operador**.

## Cuándo usar

- Antes de `done` de cualquier task `web/src/**`.
- Al crear `VehicleCard`, `AlertsPanel`, `ChatTab`, `ZonesList/Map`, `Geoman` draft, modales `Create/Edit Zone`.

## Checklist rápido

### React
- [ ] Hooks deps completas, cleanup `EventSource`/`map.pm`/`setInterval` en `return`
- [ ] `key={zone.id}` estable, no `index`
- [ ] `useFleetStream(selectedPlate)` reconecta `telemetry.raw.>` vs `telemetry.raw.TTF678`
- [ ] `tsc --noEmit` y `npm run build` verdes

### Performance
- [ ] `fleet:position` no remonta `Markers` completos
- [ ] `MarkerClusterGroup chunkedLoading` si >500
- [ ] Leaflet `lazy` import, Geoman 30KB justificado

### A11y WCAG AA
- [ ] `label` + `aria-label` en `Plate`, `aria-live=polite` en Alerts
- [ ] `role=dialog aria-modal` + focus trap + `Esc` en modales
- [ ] Keyboard: Tab `Search->Clear->Tabs`, Enter en Search
- [ ] Contraste `Moving #16a34a / Idle #dc2626` OK

### Design / Figma
- [ ] Colores tokenizados (emerald-300 Clear, blue-800 Create, red 0.2)
- [ ] `h-[280px] lg:h-[340px] overflow-y-auto` 3 paneles (Alerts/Chat/Zones) — no crece con texto
- [ ] Empty: `placa no encontrada` mantiene flota, `zones==0` sin polígono
- [ ] Errores inline: `409 zone name already exists` bajo input, `400 validation` no toast vacío

### UX Nielsen
- [ ] `Search disabled` si `!PLATE_RE`, hint visible
- [ ] `Create zone disabled` sin draft, habilita con `draftPolygon`
- [ ] `Cancel` descarta draft, `Delete` confirmada en modal
- [ ] `Last update HH:mm:ss` y `⚠️` solo >80

## Workflow auditor

1. Lanza `frontend-auditor`:
   ```ts
   task({ subagent_type: "frontend-auditor", prompt: "Audita web/src/features/monitoring + zones para SPEC-004, maquetas 8, criterios AC-001..012" })
   ```
2. Corrige hallazgos altos, documenta en `docs/IAUDIT.md` si IA sugirió patrón deficiente (ej: `leaflet-draw` sin cierre).
3. Re-audita hasta 0 altas.

## Evitar

- No mezclar `CSS modules` + `Tailwind` para mismo componente.
- No `WebSocket` si SSE basta.
- No hardcodear secretos en `VITE_*`.
