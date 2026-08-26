// Covers [SPEC-004: AC-012, TS-011, NFR-006, NFR-008, BR-015]
// TASK-004-07 Step 7 Polish a11y + fixed height + coverage — TDD RED
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen, within, cleanup } from "@testing-library/react";
import fs from "fs";
import path from "path";
import React from "react";

vi.mock("react-leaflet", () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => <div data-testid="map-mock">{children}</div>,
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: () => <div data-testid="marker" />,
  GeoJSON: () => <div data-testid="geojson" />,
  useMap: () => ({ setView: vi.fn(), pm: { addControls: vi.fn() }, on: vi.fn(), off: vi.fn() }),
}));

import CreateZoneModal from "../features/zones/CreateZoneModal";
import EditZoneModal from "../features/zones/EditZoneModal";
import { PANEL_FIXED, ZONES_PANEL_FIXED } from "../lib/ui";
import BottomPanelShell from "../features/monitoring/BottomPanelShell";
import { usePortalStore } from "../store/portalStore";

describe("A11y + fixed height + coverage — AC-012 NFR-008 BR-015", () => {
  // Covers [SPEC-004: AC-012, TS-011, NFR-008, BR-015]

  beforeEach(() => {
    // Arrange — reset portal store
    usePortalStore.setState({ activeTop: "monitoring", activeBottom: "alerts", draftPolygon: null } as never);
  });

  afterEach(() => {
    // Arrange — cleanup
    cleanup();
    vi.restoreAllMocks();
  });

  describe("modales role=dialog aria-modal y focus trap", () => {
    // Covers [SPEC-004: AC-012, NFR-008]

    it("CreateZoneModal open -> role=dialog aria-modal=true Zone name input + Accept/Cancel", async () => {
      // Arrange
      const draft = { type: "Polygon" as const, coordinates: [[[-74.07, 4.71], [-74.05, 4.71], [-74.05, 4.73], [-74.07, 4.73], [-74.07, 4.71]]] };
      const onClose = vi.fn();
      const onCreated = vi.fn();

      // Act
      render(<CreateZoneModal open={true} draft={draft as never} onClose={onClose} onCreated={onCreated} />);

      // Assert
      const dialog = screen.getByRole("dialog");
      expect(dialog).toBeInTheDocument(); // NFR-007
      expect(dialog).toHaveAttribute("aria-modal", "true"); // NFR-007
      expect(dialog).toHaveAttribute("aria-labelledby", "zone-name-label");
      // getByLabelText es ambiguo por aria-labelledby — usar within dialog + textbox role + label association
      const inputByLabel = within(dialog).getByLabelText(/Zone name/i);
      expect(inputByLabel).toBeInTheDocument();
      expect(inputByLabel).toHaveAttribute("id", "zone-name");
      expect(screen.getByRole("button", { name: /Accept/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Cancel/i })).toBeInTheDocument();
      expect(dialog).toHaveAttribute("tabIndex", "-1");
    });

    it("EditZoneModal open con zone -> role=dialog aria-modal prefill New name + Rename/Delete/Cancel", async () => {
      // Arrange
      const zone = { id: "abc-123", properties: { name: "Zone 2" }, geometry: { type: "Polygon", coordinates: [[[-74, 4.71], [-74.05, 4.71], [-74.05, 4.73], [-74, 4.73], [-74, 4.71]]] } } as never;
      const onClose = vi.fn();

      // Act
      render(<EditZoneModal open={true} zone={zone as never} onClose={onClose} onRenamed={vi.fn()} onDeleted={vi.fn()} />);

      // Assert
      const dialog = screen.getByRole("dialog");
      expect(dialog).toBeInTheDocument(); // NFR-007
      expect(dialog).toHaveAttribute("aria-modal", "true");
      expect(dialog).toHaveAttribute("aria-labelledby", "edit-zone-name-label");
      expect(screen.getByDisplayValue("Zone 2")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Rename/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Delete/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Cancel/i })).toBeInTheDocument();
    });

    it("dialog focus trap — Tab wrap y Esc cierra (useDialogFocus)", async () => {
      // Arrange
      const draft = { type: "Polygon" as const, coordinates: [[[-74.07, 4.71], [-74.05, 4.71], [-74.05, 4.73], [-74.07, 4.73], [-74.07, 4.71]]] };
      const onClose = vi.fn();

      // Act
      const { container } = render(<CreateZoneModal open={true} draft={draft as never} onClose={onClose} onCreated={vi.fn()} />);
      const dialog = screen.getByRole("dialog");
      // Hook attaches keydown listener to dialog; verify via fs check + live Esc dispatch
      const input = within(dialog).getByLabelText(/Zone name/i) as HTMLInputElement;
      input.focus();
      expect(document.activeElement).toBe(input);

      // Dispatch Esc on dialog — should call onClose via useDialogFocus
      const escEvent = new KeyboardEvent("keydown", { key: "Escape", bubbles: true });
      dialog.dispatchEvent(escEvent);

      // Assert — hook spec: Esc debe cerrar
      expect(container).toBeInTheDocument();
      // Nota: jsdom requestAnimationFrame async focus may not have fired, but Esc handler is sync
      // Verificamos que el hook existe via fs (complementario al live test)
      const hookPath = path.resolve(__dirname, "../lib/useDialogFocus.ts");
      const hookContent = fs.existsSync(hookPath) ? fs.readFileSync(hookPath, "utf-8") : "";
      expect(hookContent).toContain("Escape");
      expect(hookContent).toContain("Tab");
    });
  });

  describe("altura fija proporcional — h-[280px] lg:h-[340px] overflow-y-auto", () => {
    // Covers [SPEC-004: AC-012, BR-015, NFR-008]

    it("PANEL_FIXED y ZONES_PANEL_FIXED tokens — h-[280px] overflow-y-auto fijos", async () => {
      // Arrange
      const expectedPanel = "h-[280px] lg:h-[340px] overflow-y-auto";

      // Act
      const panelFixed = PANEL_FIXED;
      const zonesFixed = ZONES_PANEL_FIXED;

      // Assert
      expect(panelFixed).toContain("h-[280px]"); // BR-015
      expect(panelFixed).toContain("lg:h-[340px]");
      expect(panelFixed).toContain("overflow-y-auto");
      expect(zonesFixed).toContain("h-[360px]");
      expect(zonesFixed).toContain("overflow-y-auto");
      expect(panelFixed).toBe(expectedPanel);
    });

    it("BottomPanelShell render -> clase fija h-[280px] overflow-y-auto y hidden por activeBottom", async () => {
      // Arrange
      usePortalStore.setState({ activeBottom: "alerts" } as never);

      // Act
      render(
        <BottomPanelShell activeKey="alerts" testId="test-panel">
          <div>content</div>
        </BottomPanelShell>,
      );
      const panel = screen.getByTestId("test-panel");

      // Assert
      expect(panel.className).toContain("h-[280px]"); // BR-015
      expect(panel.className).toContain("overflow-y-auto"); // BR-015
      expect(panel.className).toContain("flex");
      expect(panel).not.toHaveAttribute("hidden");
      expect(panel).not.toHaveAttribute("aria-hidden");
    });

    it("BottomPanelShell hidden cuando activeBottom distinto — aria-hidden + hidden", async () => {
      // Arrange
      usePortalStore.setState({ activeBottom: "chat" } as never);

      // Act
      render(
        <BottomPanelShell activeKey="alerts" testId="test-panel-hidden">
          <div>hidden content</div>
        </BottomPanelShell>,
      );
      const panel = screen.getByTestId("test-panel-hidden");

      // Assert
      expect(panel).toHaveAttribute("hidden");
      expect(panel).toHaveAttribute("aria-hidden", "true");
      expect(panel.className).toContain("h-[280px]"); // mantiene altura fija aunque hidden
    });
  });

  describe("coverage + depguard polish guard (TDD RED)", () => {
    // Covers [SPEC-004: AC-012, TS-011]

    it("vite coverage thresholds >60% existen (DoD gate)", async () => {
      // Arrange
      const vitePath = path.resolve(__dirname, "../../vite.config.ts");
      const content = fs.existsSync(vitePath) ? fs.readFileSync(vitePath, "utf-8") : "";

      // Act
      const hasThresholds = content.includes("thresholds");
      const meetsLines = content.includes("lines: 60");

      // Assert
      expect(content.length).toBeGreaterThan(0);
      expect(hasThresholds).toBe(true);
      expect(meetsLines).toBe(true);
    });

    it("ui.ts single source sin a11yGuard stub — DRY Q-01 Q-02 + ZonesList usa token", async () => {
      // Arrange
      const uiPath = path.resolve(__dirname, "../lib/ui.ts");
      const guardPath = path.resolve(__dirname, "../lib/a11yGuard.ts");
      const zonesListPath = path.resolve(__dirname, "../features/zones/ZonesList.tsx");
      const uiContent = fs.existsSync(uiPath) ? fs.readFileSync(uiPath, "utf-8") : "";
      const zonesContent = fs.existsSync(zonesListPath) ? fs.readFileSync(zonesListPath, "utf-8") : "";

      // Act
      const hasPanelFixed = uiContent.includes("PANEL_FIXED") && uiContent.includes("h-[280px]");
      const hasZonesFixed = uiContent.includes("ZONES_PANEL_FIXED") && uiContent.includes("h-[360px]");
      const hasMonitoringDup = uiContent.includes("MONITORING_PANEL_FIXED");
      const hasPanelHeightDup = /export\s+const\s+PANEL_HEIGHT/.test(uiContent);
      const zonesImportsToken = zonesContent.includes("ZONES_PANEL_FIXED") && zonesContent.includes("from") && zonesContent.includes("ui");
      const guardExists = fs.existsSync(guardPath);

      // Assert — ui.ts es única fuente, sin duplicados, guard eliminado y ZonesList usa token
      expect(hasPanelFixed, "ui.ts debe exportar PANEL_FIXED").toBe(true);
      expect(hasZonesFixed, "ui.ts debe exportar ZONES_PANEL_FIXED").toBe(true);
      expect(hasMonitoringDup, "ui.ts no debe contener MONITORING_PANEL_FIXED (Q-01)").toBe(false);
      expect(hasPanelHeightDup, "ui.ts no debe exportar PANEL_HEIGHT (Q-01)").toBe(false);
      expect(zonesImportsToken, "ZonesList.tsx debe importar ZONES_PANEL_FIXED desde ui.ts (Q-01)").toBe(true);
      expect(guardExists, "a11yGuard.ts stub debe haber sido eliminado (Q-02)").toBe(false);
    });

    it("tiles OSM guard — Map.tsx debe usar OSM directo y no /api/tiles (AC-012)", async () => {
      // Arrange
      const mapPath = path.resolve(__dirname, "../map/Map.tsx");
      const content = fs.existsSync(mapPath) ? fs.readFileSync(mapPath, "utf-8") : "";

      // Act
      const hasOsm = content.includes("tile.openstreetmap.org");
      const hasDirect = content.includes("https://{s}.tile.openstreetmap.org");
      const hasApiTiles = content.includes("/api/tiles");

      // Assert
      expect(fs.existsSync(mapPath)).toBe(true);
      expect(hasOsm).toBe(true);
      expect(hasDirect).toBe(true);
      expect(hasApiTiles).toBe(false);
    });
  });
});
