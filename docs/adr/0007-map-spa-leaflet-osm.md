# ADR-0007 — Render de mapas en SPA: Leaflet + OSM raster + GeoJSON para zonas críticas

- **Fecha:** 2026-08-22
- **Estado:** Aceptado (ligero, reversible; no bloqueante)
- **Decisores:** `architect`

## Contexto

`docs/PRUEBA-TECNICA.md` §4.C exige *"Dashboard Reactivo: SPA que consuma los datos procesados mediante WebSockets o SSE. Mostrar mapa, alertas en tiempo real y chat con la IA"*.

El mismo §4.B incluye la query canónica que ata mapa e IA: *“¿Qué vehículos llevan detenidos más de 20 minutos en zonas críticas?”*. Sin definición de **zona crítica**, ni el mapa puede pintarla ni el agente puede responder sin alucinar. Ambos deben compartir la misma fuente de verdad.

Restricciones del entorno: SPA Vite (no Next SSR), máquina dev 16 GB, costo cero MVP (coherente con ADR-0003 free tier), sin secretos en frontend (ADR-0004), y Nivel 1 ya fijado sin `Map Provider` externo (ver `docs/c4/01-context.md` - tiles directo `web -> OSM`, nunca proxy por `cmd/api`).

Horizonte de escala: miles de dispositivos con marker por vehículo + polígonos de zonas.

## Decisión

**Leaflet + react-leaflet + OSM raster + overlay GeoJSON para zonas críticas**, consumido directo desde el browser:

- **Tiles:** `https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png` directo `web -> OSM`. Sin API key, sin token en `.env`, sin proxy por backend.
- **Librería:** `leaflet` (~40 KB gz) + `react-leaflet` + `leaflet.markercluster` para clusterizar flota en el MVP.
- **Zonas críticas:** `GET /api/zones -> GeoJSON (Polygon)` renderizado como `<GeoJSON style={color:'red', fillOpacity:0.2} />`. Origen: tabla `critical_zones(id, name, geom geometry(Polygon,4326))` o `geojson jsonb` si no se activa PostGIS en el MVP. Seed inicial con 2-3 zonas; evolución por `POST /api/zones`.
- **Tiempo real:** la misma `/api/zones` alimenta al agente IA. Tool `findVehiclesStoppedInCriticalZones(durationMin)` hace `ST_Within(last_pos, zone.geom) AND now() - last_movement > interval` en TimescaleDB; el LLM solo formatea.

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo (para este workload y entorno) |
|---|---|---|
| **Leaflet + OSM raster (elegida)** | Adoptada | 0 costo, 0 keys, bundle mínimo, `markercluster` resuelve 1k-5k markers del MVP, OSM tiles libres, sin secreto expuesto |
| **MapLibre GL (vector)** | Reservada para escala | Vector + clustering GPU superior para >10k markers, mismo modelo sin key (compatible OSM). Overkill MVP: 350 KB + estilo vectorial para ganancia no medible aún |
| **Mapbox GL** | Descartada MVP | Vector top pero exige `MAPBOX_TOKEN` en frontend (secreto expuesto, rompe ADR-0004) + facturación por 1M tiles (rompe costo cero ADR-0003) |
| **Google Maps JS** | Descartada MVP | Idem Mapbox + facturación por carga + API key obligatoria; sin ventaja para flota interna |
| **Proxy de tiles por `cmd/api`** | Prohibida | Añade ancho de banda y latencia al backend, rompe aislamiento front↔agente (ADR-0003 cond. 9), y haría falso el Nivel 1 (`Fleet platform -> Map Provider`) |

## Condiciones obligatorias

1. **Tiles nunca por backend:** `web` hace `fetch` directo a OSM. Prohibido `GET /api/tiles/*` proxy. CI con `depguard` puede bloquear `import leaflet` en `backend/`.
2. **Cero secretos en frontend:** ninguna `VITE_*_TOKEN` para mapas en el MVP. Si se migra a Mapbox, el token va por `env` del build pero nunca hardcodeado; documentar rotación (ADR-0004).
3. **Clustering desde día 1 si >500 markers visibles:** `leaflet.markercluster` obligatorio para no matar el DOM en mapa con flota completa. Alternativa futura: MapLibre `supercluster`.
4. **Zonas críticas como GeoJSON canónico:** el mismo `GET /api/zones` alimenta mapa y tool del agente. Nunca duplicar definición de zona entre frontend y backend. Zona sin `geom` válida se rechaza en `POST /api/zones` (validación de polígono cerrado).
5. **Import dinámico en SPA Vite:** `const Map = dynamic(() => import('./Map'), {ssr:false})` o `lazy(() => import(...))` para evitar SSR/hydrate mismatch (aunque somos SPA, deja el patrón listo si se migra a Next).
6. **Trigger de migración a vector (documentado, NO implementado):** >10k markers simultáneos o necesidad de estilos vectoriales avanzados (bearing, pitch, 3D) → migrar a MapLibre GL con el mismo endpoint `GET /api/zones` (cambio solo en `web/`, sin tocar `backend/` ni contrato).

## Consecuencias

**Positivas:**
- Costo cero y sin gestión de keys (coherente con free tier Gemini del MVP).
- Aislamiento correcto: Nivel 1 limpio (sin `Map Provider` externo), Nivel 2 muestra `Web App -> OSM` como dependencia de contenedor, no de sistema.
- Misma fuente de verdad para mapa y para la query canónica de la IA (no hay divergencia de zonas).
- Bundle ligero y footprint 16 GB intacto.

**Negativas:**
- Raster OSM es menos nítido al hacer zoom que vector y consume más ancho de banda por tile (~20-30 KB/tile vs vector delta).
- Sin vector, animaciones avanzadas (rotación, pitch) no disponibles hasta migrar a MapLibre.
- Dependencia de disponibilidad de `tile.openstreetmap.org` (rate limit generoso para MVP, pero sin SLA; mitigado por cache del browser y por no ser crítico: lista/alertas siguen funcionando sin tiles).

## Referencias

- `docs/PRUEBA-TECNICA.md` §4.B (query zonas críticas 20 min) y §4.C (mapa + SSE)
- `docs/c4/01-context.md` (`Qué NO va en Nivel 1` - Map Provider a Nivel 2)
- `docs/c4/workspace.dsl` (Contexto_N1 sin Map Provider - tiles directo web->OSM)
- ADR-0003 (costo cero, sin secretos) y ADR-0004 (no hardcodear tokens)
- ADR-0002 (SPA Vite) y ADR-0005 (4 bins - web separado de api/agent)
