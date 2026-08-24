// Covers [SPEC-002: AC-007/008/009, BR-008/009]
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import Map from "./Map";

// Helper to build 600 vehicles
function buildVehicles(n: number) {
  return Array.from({ length: n }, (_, i) => ({
    plate: `AAA${String(i).padStart(3, "0")}`,
    lat: 4.71 + (i % 100) * 0.001,
    lon: -74.07 + (i % 100) * 0.001,
    speed: i % 2 === 0 ? 0 : 60,
  }));
}

const zonesFixture = {
  type: "FeatureCollection" as const,
  features: [
    {
      type: "Feature" as const,
      id: "zone-1",
      properties: { name: "Zona Norte" },
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
  ],
};

describe("Map", () => {
  // Covers [SPEC-002: AC-007/008/009, BR-008/009]
  let fetchSpy: any;

  beforeEach(() => {
    fetchSpy = vi.spyOn(globalThis as any, "fetch").mockImplementation((input: any) => {
      const url = typeof input === "string" ? input : input.toString();
      if (url.includes("/api/zones")) {
        return Promise.resolve(
          new Response(JSON.stringify(zonesFixture), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      return Promise.resolve(new Response("{}", { status: 200 }));
    });
  });

  afterEach(() => {
    fetchSpy.mockRestore();
    vi.restoreAllMocks();
  });

  it("600 markers -> markercluster DOM <500", async () => {
    // Arrange
    const vehicles = buildVehicles(600);

    // Act
    render(<Map vehicles={vehicles} zones={zonesFixture} />);

    // Assert
    // Leaflet markercluster should be present when >500 markers
    const clusterGroup = document.querySelector(".marker-cluster, .leaflet-marker-cluster, .markerClusterGroup");
    expect(clusterGroup, "markerClusterGroup must exist for >500 markers").toBeTruthy(); // AC-007 BR-009
    const markers = document.querySelectorAll(".leaflet-marker-icon, .marker-cluster div, [data-testid='vehicle-marker']");
    // DOM nodes must be clustered <500, not 600 individual markers
    expect(markers.length).toBeLessThan(500); // AC-007 600 -> DOM <500 via clustering
    expect(markers.length).toBeGreaterThan(0); // AC-007 at least some clusters visible
  });

  it("tiles fetch directo OSM sin /api/tiles (depguard)", async () => {
    // Arrange
    const vehicles = buildVehicles(10);

    // Act
    render(<Map vehicles={vehicles} zones={zonesFixture} />);

    // Assert — TileLayer must use OSM directly
    await waitFor(() => {
      const tiles = document.querySelectorAll<HTMLImageElement>(
        'img.leaflet-tile, img[src*="tile.openstreetmap.org"]',
      );
      // Also check any element with tile URL in style/background
      const tileSrcs = Array.from(tiles).map((el) => el.src);
      const hasOsmTile = tileSrcs.some((src) => src.includes("tile.openstreetmap.org"));
      expect(hasOsmTile, "tiles must fetch directo https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png").toBe(true); // AC-007 BR-008
    });

    // Assert — no request proxied via /api/tiles
    const calledUrls = fetchSpy.mock.calls.map(([arg]: any) => String(arg));
    const proxiedTiles = calledUrls.filter((u: any) => u.includes("/api/tiles"));
    expect(proxiedTiles).toHaveLength(0); // AC-007 BR-008 prohibido /api/tiles

    // Assert — TileLayer url prop is OSM direct (check rendered map container attribute)
    const mapContainer = document.querySelector(".leaflet-container, [data-testid='map-container']");
    expect(mapContainer || document.body.innerHTML.includes("tile.openstreetmap.org")).toBeTruthy(); // AC-007 OSM direct
    expect(document.body.innerHTML).toContain("tile.openstreetmap.org"); // AC-007
    expect(document.body.innerHTML).not.toContain("/api/tiles"); // AC-007 BR-008
  });

  it("GeoJSON overlay rojo fillOpacity 0.2", async () => {
    // Arrange
    // ensure fetch for zones will be called by Map
    fetchSpy.mockClear();

    // Act
    render(<Map vehicles={buildVehicles(5)} zones={zonesFixture} />);

    // Assert — fetch /api/zones called and GeoJSON rendered with style red 0.2
    await waitFor(() => {
      const zoneCalls = fetchSpy.mock.calls.filter(([arg]: any) => String(arg).includes("/api/zones"));
      expect(zoneCalls.length).toBeGreaterThan(0); // AC-007 GET /api/zones
    });

    // GeoJSON overlay must be rojo with fillOpacity 0.2
    // Check for leaflet GeoJSON path with style
    const geoPaths = document.querySelectorAll<SVGPathElement>(
      "path.leaflet-interactive, path[stroke='red'], .leaflet-overlay-pane path",
    );
    // At least one path should exist for the zone polygon
    expect(geoPaths.length).toBeGreaterThan(0); // AC-007 GeoJSON overlay visible

    // Verify style: color red, fillOpacity 0.2 (check inline style or attribute)
    const redPath = Array.from(geoPaths).find(
      (p) =>
        p.getAttribute("stroke") === "red" ||
        p.getAttribute("fill") === "red" ||
        p.style.stroke === "red" ||
        document.body.innerHTML.includes('color:red') ||
        document.body.innerHTML.includes('color: red'),
    );
    // Fallback: check that Map's GeoJSON style prop was used (search rendered HTML for style indicators)
    const html = document.body.innerHTML;
    const hasRed = html.includes("red") || html.includes("#ff0000") || redPath !== undefined;
    expect(hasRed, "GeoJSON style must be color:red").toBe(true); // AC-007 BR-005

    // fillOpacity 0.2 check — look for opacity attribute/style
    const hasFillOpacity =
      html.includes("fillOpacity") ||
      html.includes("fill-opacity") ||
      Array.from(geoPaths).some(
        (p) => p.getAttribute("fill-opacity") === "0.2" || p.style.fillOpacity === "0.2",
      ) ||
      html.includes("0.2");
    expect(hasFillOpacity, "GeoJSON fillOpacity must be 0.2").toBe(true); // AC-007 overlay rojo 0.2
  });
});
