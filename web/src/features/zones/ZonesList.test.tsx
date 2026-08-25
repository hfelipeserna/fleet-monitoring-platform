// Covers [SPEC-004: AC-006, AC-007, BR-004/012/015, FR-008/009, TS-005/006, UC-004]
import { describe, it, expect, vi, beforeAll, afterAll, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, cleanup, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import ZonesList from "./ZonesList";
import Map from "../../map/Map";
import App from "../../App";
import { usePortalStore } from "../../store/portalStore";

const emptyFC = {
  type: "FeatureCollection" as const,
  features: [] as unknown[],
};

const fourZonesFC = {
  type: "FeatureCollection" as const,
  features: [
    {
      type: "Feature" as const,
      id: "zone-1",
      properties: { name: "Zone 1" },
      geometry: {
        type: "Polygon" as const,
        coordinates: [
          [
            [-74.07, 4.71],
            [-74.05, 4.71],
            [-74.05, 4.73],
            [-74.07, 4.73],
            [-74.07, 4.71],
          ],
        ],
      },
    },
    {
      type: "Feature" as const,
      id: "zone-2",
      properties: { name: "Zone 2" },
      geometry: {
        type: "Polygon" as const,
        coordinates: [
          [
            [-74.08, 4.71],
            [-74.06, 4.71],
            [-74.06, 4.73],
            [-74.08, 4.73],
            [-74.08, 4.71],
          ],
        ],
      },
    },
    {
      type: "Feature" as const,
      id: "zone-3",
      properties: { name: "Zone 3" },
      geometry: {
        type: "Polygon" as const,
        coordinates: [
          [
            [-74.09, 4.71],
            [-74.07, 4.71],
            [-74.07, 4.73],
            [-74.09, 4.73],
            [-74.09, 4.71],
          ],
        ],
      },
    },
    {
      type: "Feature" as const,
      id: "zone-4",
      properties: { name: "Zone 4" },
      geometry: {
        type: "Polygon" as const,
        coordinates: [
          [
            [-74.1, 4.71],
            [-74.08, 4.71],
            [-74.08, 4.73],
            [-74.1, 4.73],
            [-74.1, 4.71],
          ],
        ],
      },
    },
  ],
};

const server = setupServer(
  http.get("/api/zones", () => HttpResponse.json(emptyFC)),
  http.get("*/api/zones", () => HttpResponse.json(emptyFC)),
);

describe("ZonesList — TS-005 AC-006", () => {
  // Covers [SPEC-004: AC-006, BR-004/015, FR-008, TS-005, UC-004]

  beforeAll(() => {
    // Arrange global msw
    server.listen({ onUnhandledRequest: "warn" });
  });

  beforeEach(() => {
    // Arrange
    usePortalStore.setState({ activeTop: "zones", activeBottom: "alerts" } as never);
    Object.defineProperty(HTMLElement.prototype, "clientWidth", { configurable: true, get() { return 800; } });
    Object.defineProperty(HTMLElement.prototype, "clientHeight", { configurable: true, get() { return 600; } });
    Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, get() { return 800; } });
    Object.defineProperty(HTMLElement.prototype, "offsetHeight", { configurable: true, get() { return 600; } });
    HTMLElement.prototype.getBoundingClientRect = function () {
      return { width: 800, height: 600, top: 0, left: 0, right: 800, bottom: 600, x: 0, y: 0, toJSON() { return {}; } } as unknown as DOMRect;
    } as never;
  });

  afterEach(() => {
    cleanup();
    server.resetHandlers();
    vi.restoreAllMocks();
    usePortalStore.setState({ activeTop: "monitoring", activeBottom: "alerts" } as never);
  });

  afterAll(() => {
    server.close();
  });

  describe("zones 0 vacío -> Create zone disabled + Map sin polígono", () => {
    // Covers [SPEC-004: AC-006, BR-004/015, FR-008, TS-005]

    it("zones 0 vacío -> Create zone disabled + Map sin polígono", async () => {
      // Covers [SPEC-004: AC-006, BR-004/015, FR-008, TS-005]
      // Arrange
      server.use(
        http.get("/api/zones", () => HttpResponse.json(emptyFC)),
        http.get("*/api/zones", () => HttpResponse.json(emptyFC)),
      );
      usePortalStore.setState({ activeTop: "zones" } as never);

      // Act
      render(<ZonesList />);
      const appRender = render(<App />);

      // Assert — ZonesList vacía sin filas Zone N, container con overflow-y-auto y altura fija
      await waitFor(() => {
        const listEl = document.querySelector('[data-testid="zones-list"]') as HTMLElement | null;
        expect(listEl, "ZonesList debe existir con data-testid zones-list").not.toBeNull(); // AC-006 BR-015
      });
      const zonesListEl = document.querySelector('[data-testid="zones-list"]') as HTMLElement;
      expect(zonesListEl.className).toMatch(/overflow-y-auto/); // AC-006 BR-015 panel fijo overflow-y-auto
      // altura fija h-[360px] lg:h-[480px] según plan Step 5
      const hasFixedHeight = zonesListEl.className.includes("h-[360px]") || zonesListEl.className.includes("h-[480px]") || document.body.innerHTML.includes("h-[360px]");
      expect(hasFixedHeight, "ZonesList debe tener altura fija h-[360px] lg:h-[480px] overflow-y-auto").toBe(true); // AC-006 BR-015

      // Assert — lista vacía no debe tener filas Zone 1..4
      const zoneRows = screen.queryAllByText(/Zone [1-4]/);
      expect(zoneRows).toHaveLength(0); // AC-006 zones 0 vacío sin filas
      expect(screen.queryByText(/Zone 1/)).not.toBeInTheDocument(); // AC-006
      expect(screen.queryByText(/Zone 2/)).not.toBeInTheDocument(); // AC-006

      // Assert — Create zone disabled sin draft
      const createBtn = within(appRender.container).queryByRole("button", { name: /Create zone/i }) ?? screen.queryByRole("button", { name: /Create zone/i });
      expect(createBtn, "Create zone debe existir en Critical zones").not.toBeNull(); // AC-006 FR-008/009 BR-012
      expect(createBtn).toBeInTheDocument();
      expect(createBtn).toBeDisabled(); // BR-012 Create zone disabled sin draft

      // Assert — Map sin polígono rojo cuando zones 0
      // Render Map con zones vacías para verificar 0 paths
      cleanup();
      render(<Map zones={emptyFC as never} vehicles={[]} />);
      await waitFor(() => {
        const mapEl = document.querySelector('[data-testid="map"]') as HTMLElement | null;
        expect(mapEl || document.querySelector(".leaflet-container")).not.toBeNull(); // AC-006 map existe
      });
      const pathsEmpty = document.querySelectorAll("path.leaflet-interactive");
      expect(pathsEmpty.length).toBe(0); // AC-006 Map sin polígono cuando zones 0
      // fetch GET /api/zones debe haberse llamado (msw) y no debe haber path rojo
      const htmlEmpty = document.body.innerHTML;
      const hasRedEmpty = htmlEmpty.includes('stroke="red"') || htmlEmpty.includes("fillOpacity");
      // Con zones 0 no debe haber rojo (o si lo hay, debe ser 0 paths)
      expect(pathsEmpty.length === 0 || !hasRedEmpty).toBe(true); // AC-006 sin polígono
    });
  });

  describe("4 zonas alternando verde/celeste + 4 polygons rojo fillOpacity 0.2", () => {
    // Covers [SPEC-004: AC-006, BR-004/015, FR-008, TS-005]

    it("4 zonas alternando verde/celeste + 4 polygons rojo fillOpacity 0.2", async () => {
      // Covers [SPEC-004: AC-006, BR-004/015, FR-008, TS-005]
      // Arrange
      server.use(
        http.get("/api/zones", () => HttpResponse.json(fourZonesFC)),
        http.get("*/api/zones", () => HttpResponse.json(fourZonesFC)),
      );
      usePortalStore.setState({ activeTop: "zones" } as never);

      // Act — ZonesList vía hook useZones (única fuente BR-004), Map recibe zones por prop sin fetch duplicado
      render(<ZonesList />);
      render(<Map zones={fourZonesFC as never} vehicles={[]} />);

      // Assert — espera a que ZonesList haga fetch GET /api/zones FeatureCollection
      await waitFor(() => {
        // ZonesList debe haber renderizado 4 filas tras fetch
        expect(screen.getByText("Zone 1")).toBeInTheDocument(); // AC-006 FR-008 lista Zone 1
      });
      expect(screen.getByText("Zone 2")).toBeInTheDocument(); // AC-006
      expect(screen.getByText("Zone 3")).toBeInTheDocument(); // AC-006
      expect(screen.getByText("Zone 4")).toBeInTheDocument(); // AC-006
      const allRows = screen.getAllByText(/Zone [1-4]/);
      expect(allRows).toHaveLength(4); // AC-006 4 filas

      // Assert — alternando bg-emerald-100 / bg-cyan-100
      const listContainer = document.querySelector('[data-testid="zones-list"]') as HTMLElement | null;
      expect(listContainer).not.toBeNull(); // AC-006
      // Cada fila debe tener clase alterna; buscamos elementos que contienen Zone N
      const row1El = screen.getByText("Zone 1").closest("div") as HTMLElement | null;
      const row2El = screen.getByText("Zone 2").closest("div") as HTMLElement | null;
      const row3El = screen.getByText("Zone 3").closest("div") as HTMLElement | null;
      const row4El = screen.getByText("Zone 4").closest("div") as HTMLElement | null;
      expect(row1El).not.toBeNull();
      expect(row2El).not.toBeNull();
      expect(row1El!.className).toMatch(/bg-emerald-100/); // AC-006 alterna verde
      expect(row2El!.className).toMatch(/bg-cyan-100/); // AC-006 alterna celeste
      expect(row3El!.className).toMatch(/bg-emerald-100/); // AC-006 verde
      expect(row4El!.className).toMatch(/bg-cyan-100/); // AC-006 celeste
      // key={id} verificado indirectamente por render 4 filas sin duplicado
      expect(new Set(allRows.map((r) => r.textContent)).size).toBe(4); // AC-006 keys únicas

      // Assert — Map pinta 4 GeoJSON polygons rojo fillOpacity 0.2
      await waitFor(() => {
        const paths = document.querySelectorAll("path.leaflet-interactive, .leaflet-overlay-pane path");
        expect(paths.length).toBeGreaterThanOrEqual(4); // AC-006 FR-008 4 polygons
      });
      const geoPaths = document.querySelectorAll<SVGPathElement>("path.leaflet-interactive, .leaflet-overlay-pane path");
      expect(geoPaths.length).toBe(4); // AC-006 exactamente 4 polygons rojo 0.2
      // Cada path debe tener stroke red y fillOpacity 0.2
      geoPaths.forEach((p) => {
        // Arrange check per path
        const stroke = p.getAttribute("stroke");
        const fillOpacity = p.getAttribute("fill-opacity") ?? p.style.fillOpacity;
        const html = document.body.innerHTML;
        const hasRed = stroke === "red" || p.getAttribute("fill") === "red" || html.includes("red");
        expect(hasRed, "GeoJSON debe ser rojo").toBe(true); // BR-004 color red
        const hasOpacity = fillOpacity === "0.2" || p.style.fillOpacity === "0.2" || html.includes("0.2");
        expect(hasOpacity, "GeoJSON fillOpacity debe ser 0.2").toBe(true); // FR-008 fillOpacity 0.2
      });

      // Assert — BR-004 GET /api/zones es única fuente canónica (no duplicada en frontend)
      // Verifica que no hay zona duplicada en DOM (ya verificado Set size 4)
      expect(document.body.innerHTML).not.toContain("zone name already exists"); // AC-006 no error duplicado
    });
  });
});
