// Covers [SPEC-004: AC-012, TS-011, NFR-006, NFR-008]
// TASK-004-07 Step 7 Polish — TDD RED (debe fallar hasta implementar polish a11yGuard + tiles guard)
import { describe, it, expect } from "vitest";
import fs from "fs";
import path from "path";

function collectSrcFiles(dir: string, out: string[] = []): string[] {
  const entries = fs.existsSync(dir) ? fs.readdirSync(dir, { withFileTypes: true }) : [];
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name === "node_modules" || e.name === "dist" || e.name === ".git") continue;
      collectSrcFiles(full, out);
    } else if (e.isFile() && (full.endsWith(".ts") || full.endsWith(".tsx"))) {
      if (full.includes(".test.") || full.includes(".spec.")) continue;
      if (full.includes("/test/mocks") || full.includes("/test/setup")) continue;
      out.push(full);
    }
  }
  return out;
}

describe("Depguard transversal + OSM tiles + modales a11y — AC-012 NFR-006/008", () => {
  // Covers [SPEC-004: AC-012, TS-011, NFR-006]

  it("web/src no importa pgx/nats/genkit/jackc en todo web/src (transversal NFR-006 depguard)", async () => {
    // Arrange
    const webSrc = path.resolve(__dirname, "..");
    const files = collectSrcFiles(webSrc);

    // Act
    const forbidden: Array<{ file: string; hit: string }> = [];
    for (const file of files) {
      const content = fs.readFileSync(file, "utf-8");
      if (/from\s+["'][^"']*genkit/i.test(content) || /import\s+.*genkit/i.test(content)) {
        forbidden.push({ file, hit: "genkit" });
      }
      if (/from\s+["'][^"']*pgx/i.test(content) || /import\s+.*pgx/i.test(content) || /jackc\/pgx/i.test(content)) {
        forbidden.push({ file, hit: "pgx/jackc" });
      }
      if (/from\s+["'][^"']*nats/i.test(content) || /import\s+.*nats/i.test(content)) {
        forbidden.push({ file, hit: "nats" });
      }
      if (/require\s*\(\s*["'][^"']*genkit/.test(content) || /require\s*\(\s*["'][^"']*pgx/.test(content)) {
        forbidden.push({ file, hit: "require genkit/pgx" });
      }
    }

    // Assert
    expect(files.length).toBeGreaterThan(0);
    expect(forbidden, `web/src debe tener 0 imports pgx/nats/genkit/jackc — hallados: ${JSON.stringify(forbidden)}`).toEqual([]);
  });

  it("tiles fetch directo https://{s}.tile.openstreetmap.org sin /api/tiles (AC-012 FR-012)", async () => {
    // Arrange
    const mapPath = path.resolve(__dirname, "../map/Map.tsx");
    const altPath = path.resolve(process.cwd(), "src/map/Map.tsx");
    const p = fs.existsSync(mapPath) ? mapPath : fs.existsSync(altPath) ? altPath : mapPath;
    const content = fs.existsSync(p) ? fs.readFileSync(p, "utf-8") : "";
    const webSrc = path.resolve(__dirname, "..");
    const files = collectSrcFiles(webSrc);

    // Act
    const hasOsmDirect = content.includes("tile.openstreetmap.org") && content.includes("https://{s}.tile.openstreetmap.org");
    const mapHasApiTiles = content.includes("/api/tiles");
    const anyApiTiles = files.some((f) => fs.readFileSync(f, "utf-8").includes("/api/tiles"));

    // Assert
    expect(fs.existsSync(p), `Map.tsx debe existir en ${p}`).toBe(true);
    expect(content.length).toBeGreaterThan(0);
    expect(hasOsmDirect, "Map.tsx debe contener OSM directo https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png (AC-012)").toBe(true);
    expect(mapHasApiTiles, "Map.tsx no debe contener /api/tiles").toBe(false);
    expect(anyApiTiles, "ningún archivo web/src debe contener /api/tiles").toBe(false);
  });

  it("modales CreateZone/EditZone tienen role=dialog aria-modal y focus trap (NFR-008 a11y)", async () => {
    // Arrange
    const createPath = path.resolve(__dirname, "../features/zones/CreateZoneModal.tsx");
    const editPath = path.resolve(__dirname, "../features/zones/EditZoneModal.tsx");
    const dialogHookPath = path.resolve(__dirname, "./useDialogFocus.ts");
    const createContent = fs.existsSync(createPath) ? fs.readFileSync(createPath, "utf-8") : "";
    const editContent = fs.existsSync(editPath) ? fs.readFileSync(editPath, "utf-8") : "";
    const hookContent = fs.existsSync(dialogHookPath) ? fs.readFileSync(dialogHookPath, "utf-8") : "";

    // Act
    const createHasDialog = createContent.includes('role="dialog"') || createContent.includes("role='dialog'");
    const editHasDialog = editContent.includes('role="dialog"') || editContent.includes("role='dialog'");
    const createHasAriaModal = createContent.includes('aria-modal="true"') || createContent.includes("aria-modal={true}");
    const editHasAriaModal = editContent.includes('aria-modal="true"') || editContent.includes("aria-modal={true}");
    const createHasFocusTrap = createContent.includes("useDialogFocus") || createContent.includes("dialogRef");
    const editHasFocusTrap = editContent.includes("useDialogFocus") || editContent.includes("dialogRef");
    const hookHasTrap = hookContent.includes("Tab") && hookContent.includes("Escape");

    // Assert
    expect(fs.existsSync(createPath), "CreateZoneModal.tsx debe existir").toBe(true);
    expect(fs.existsSync(editPath), "EditZoneModal.tsx debe existir").toBe(true);
    expect(createHasDialog, "CreateZoneModal debe tener role=dialog (NFR-008)").toBe(true);
    expect(editHasDialog, "EditZoneModal debe tener role=dialog (NFR-008)").toBe(true);
    expect(createHasAriaModal, "CreateZoneModal debe tener aria-modal=true").toBe(true);
    expect(editHasAriaModal, "EditZoneModal debe tener aria-modal=true").toBe(true);
    expect(createHasFocusTrap, "CreateZoneModal debe usar useDialogFocus / focus trap").toBe(true);
    expect(editHasFocusTrap, "EditZoneModal debe usar useDialogFocus / focus trap").toBe(true);
    expect(hookHasTrap, "useDialogFocus.ts debe implementar Tab/Esc trap").toBe(true);
  });

  it("altura fija guard — paneles h-[280px] lg:h-[340px] overflow-y-auto + zones h-[360px] (BR-015 TDD RED polish)", async () => {
    // Arrange
    const uiPath = path.resolve(__dirname, "./ui.ts");
    const alertsPath = path.resolve(__dirname, "../features/monitoring/AlertsPanel.tsx");
    const chatTabPath = path.resolve(__dirname, "../features/monitoring/ChatTab.tsx");
    const bottomShellPath = path.resolve(__dirname, "../features/monitoring/BottomPanelShell.tsx");
    const zonesListPath = path.resolve(__dirname, "../features/zones/ZonesList.tsx");
    const uiContent = fs.existsSync(uiPath) ? fs.readFileSync(uiPath, "utf-8") : "";
    const shellContent = fs.existsSync(bottomShellPath) ? fs.readFileSync(bottomShellPath, "utf-8") : "";
    const alertsContent = fs.existsSync(alertsPath) ? fs.readFileSync(alertsPath, "utf-8") : "";
    const chatContent = fs.existsSync(chatTabPath) ? fs.readFileSync(chatTabPath, "utf-8") : "";
    const zonesContent = fs.existsSync(zonesListPath) ? fs.readFileSync(zonesListPath, "utf-8") : "";

    // Act
    const uiHasPanelFixed = uiContent.includes("PANEL_FIXED") && uiContent.includes("h-[280px]") && uiContent.includes("overflow-y-auto");
    const uiHasZonesPanel = uiContent.includes("ZONES_PANEL_FIXED") && uiContent.includes("h-[360px]");
    const shellHasFixed = shellContent.includes("PANEL_FIXED") || (shellContent.includes("h-[280px]") && shellContent.includes("overflow-y-auto"));

    // Assert — estas ya pasan (verde) pero documentan BR-015
    expect(uiHasPanelFixed, "ui.ts debe exportar PANEL_FIXED con h-[280px] overflow-y-auto (BR-015)").toBe(true);
    expect(uiHasZonesPanel, "ui.ts debe exportar ZONES_PANEL_FIXED con h-[360px] (BR-015)").toBe(true);
    expect(shellHasFixed, "BottomPanelShell debe usar PANEL_FIXED / h-[280px] overflow-y-auto").toBe(true);
    expect(alertsContent.length).toBeGreaterThan(0);
    expect(chatContent.length).toBeGreaterThan(0);
    expect(zonesContent.length).toBeGreaterThan(0);
  });

  it("ui.ts single source altura fija — solo PANEL_FIXED y ZONES_PANEL_FIXED sin duplicados (DRY Q-01 Q-02)", async () => {
    // Arrange
    const uiPath = path.resolve(__dirname, "./ui.ts");
    const guardPath = path.resolve(__dirname, "./a11yGuard.ts");
    const zonesListPath = path.resolve(__dirname, "../features/zones/ZonesList.tsx");
    const uiContent = fs.existsSync(uiPath) ? fs.readFileSync(uiPath, "utf-8") : "";
    const zonesContent = fs.existsSync(zonesListPath) ? fs.readFileSync(zonesListPath, "utf-8") : "";

    // Act
    const hasPanelFixed = uiContent.includes("PANEL_FIXED") && uiContent.includes("h-[280px]") && uiContent.includes("overflow-y-auto");
    const hasZonesFixed = uiContent.includes("ZONES_PANEL_FIXED") && uiContent.includes("h-[360px]");
    const hasMonitoringDuplicate = uiContent.includes("MONITORING_PANEL_FIXED");
    const hasPanelHeightDuplicate = /export\s+const\s+PANEL_HEIGHT/.test(uiContent);
    const zonesImportsToken = zonesContent.includes("ZONES_PANEL_FIXED") && zonesContent.includes("from") && zonesContent.includes("ui");
    const guardExists = fs.existsSync(guardPath);

    // Assert — DRY: ui.ts es única fuente, sin duplicados, guard eliminado y ZonesList usa token
    expect(hasPanelFixed, "ui.ts debe exportar PANEL_FIXED con h-[280px] overflow-y-auto").toBe(true);
    expect(hasZonesFixed, "ui.ts debe exportar ZONES_PANEL_FIXED con h-[360px]").toBe(true);
    expect(hasMonitoringDuplicate, "ui.ts no debe contener MONITORING_PANEL_FIXED duplicado (Q-01)").toBe(false);
    expect(hasPanelHeightDuplicate, "ui.ts no debe exportar PANEL_HEIGHT duplicado (Q-01)").toBe(false);
    expect(zonesImportsToken, "ZonesList.tsx debe importar ZONES_PANEL_FIXED desde ui.ts (Q-01)").toBe(true);
    expect(guardExists, "web/src/lib/a11yGuard.ts debe haber sido eliminado (Q-02)").toBe(false);
  });

  it("coverage thresholds >60% y build/lint guard (NFR transversal — verifica vite.config.ts)", async () => {
    // Arrange
    const vitePath = path.resolve(__dirname, "../../vite.config.ts");
    const altVite = path.resolve(process.cwd(), "vite.config.ts");
    const p = fs.existsSync(vitePath) ? vitePath : altVite;
    const content = fs.existsSync(p) ? fs.readFileSync(p, "utf-8") : "";

    // Act
    const hasThresholds = content.includes("thresholds");
    const has60 = content.includes("lines: 60") || content.includes("lines:60");
    const hasProviderV8 = content.includes('provider: "v8"') || content.includes("provider: 'v8'") || content.includes('provider: "v8"');

    // Assert
    expect(fs.existsSync(p), `vite.config.ts debe existir en ${p}`).toBe(true);
    expect(hasThresholds, "vite.config.ts debe definir coverage.thresholds").toBe(true);
    expect(has60, "coverage thresholds deben ser >=60% (DoD)").toBe(true);
    expect(hasProviderV8, "coverage provider debe ser v8").toBe(true);
  });
});
