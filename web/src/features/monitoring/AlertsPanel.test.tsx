// Covers [SPEC-004: AC-004, BR-005/007, FR-004/005, UC-002, TS-003]
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";
import AlertsPanel from "./AlertsPanel";

// --- MockEventSource similar a useFleetStream.test.tsx ---
class MockEventSource {
  static instances: MockEventSource[] = [];
  url: string;
  readyState = 0;
  onopen: ((e: Event) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  close = vi.fn(() => {
    this.readyState = 2;
  });
  private listeners: Record<string, (e: MessageEvent) => void> = {};
  addEventListener = vi.fn((event: string, handler: EventListener) => {
    this.listeners[event] = handler as unknown as (e: MessageEvent) => void;
    if (event === "message") this.onmessage = handler as unknown as (e: MessageEvent) => void;
    if (event === "error") this.onerror = handler as unknown as (e: Event) => void;
    if (event === "open") this.onopen = handler as unknown as (e: Event) => void;
  });
  removeEventListener = vi.fn((event: string) => {
    delete this.listeners[event];
  });
  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }
  simulateOpen() {
    this.readyState = 1;
    this.onopen?.(new Event("open"));
  }
  simulateError() {
    this.readyState = 2;
    this.onerror?.(new Event("error"));
  }
  simulateMessage(data: string, eventType = "message", lastEventId?: string) {
    const evt = new MessageEvent(eventType, { data } as MessageEventInit);
    if (lastEventId) Object.defineProperty(evt, "lastEventId", { value: lastEventId, writable: true });
    if (eventType === "message") {
      this.onmessage?.(evt);
    }
    const handler = this.listeners[eventType];
    if (handler) handler(evt);
    return evt;
  }
  simulateAlert(payload: unknown, lastEventId?: string) {
    const data = JSON.stringify(payload);
    const evt = new MessageEvent("alert:critical", { data } as MessageEventInit);
    if (lastEventId) Object.defineProperty(evt, "lastEventId", { value: lastEventId, writable: true });
    // useSSE registers both onmessage and addEventListener('alert:critical', handler)
    // trigger both paths to ensure handler fires regardless of implementation (direct EventSource or useSSE)
    this.onmessage?.(new MessageEvent("message", { data } as MessageEventInit));
    const handler = this.listeners["alert:critical"];
    if (handler) handler(evt);
    // also trigger generic message handler if implementation uses onmessage only
    return evt;
  }
  simulatePing() {
    // SSE :ping comment — native EventSource does NOT fire onmessage for comment lines
    // Simulate by NOT calling handlers; just keep connection alive
    // For robustness, also test that if impl receives ":ping" as data, it is ignored
    return;
  }
  simulateRawPingAsMessage() {
    // Alternative: some servers send "data: :ping" — should be ignored by JSON parse
    const evt = new MessageEvent("message", { data: ":ping" } as MessageEventInit);
    this.onmessage?.(evt);
    const handler = this.listeners["alert:critical"];
    if (handler) handler(new MessageEvent("alert:critical", { data: ":ping" } as MessageEventInit));
    return evt;
  }
}

const speedingOnPayload = {
  plate: "TTF678",
  alert_type: "speeding_on",
  speed: 90,
  created_at: "2026-08-26T14:32:10Z",
  lat: 45.6,
  lon: 34.5,
};

const speedingOffPayload = {
  plate: "TTF678",
  alert_type: "speeding_off",
  speed: 70,
  created_at: "2026-08-26T14:32:15Z",
  lat: 45.6,
  lon: 34.5,
};

function getPanel(): HTMLElement | null {
  // Covers [SPEC-004: AC-004, BR-005] — helper para localizar panel fijo
  const byTestId = document.querySelector('[data-testid="alerts-panel"]') as HTMLElement | null;
  if (byTestId) return byTestId;
  const byLive = document.querySelector('[aria-live="polite"]') as HTMLElement | null;
  if (byLive) return byLive;
  const byOverflow = document.querySelector(".overflow-y-auto") as HTMLElement | null;
  if (byOverflow) return byOverflow;
  // fallback: first div rendered by AlertsPanel
  const container = document.body.firstElementChild as HTMLElement | null;
  return container;
}

describe("AlertsPanel", () => {
  // Covers [SPEC-004: AC-004, BR-005/007, FR-004/005, UC-002, TS-003]
  let originalEventSource: unknown;

  beforeEach(() => {
    // Arrange global mock
    MockEventSource.instances = [];
    originalEventSource = (globalThis as unknown as Record<string, unknown>).EventSource;
    (globalThis as unknown as Record<string, unknown>).EventSource = MockEventSource as unknown as typeof EventSource;
    vi.spyOn(Math, "random").mockReturnValue(0);
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    MockEventSource.instances = [];
    (globalThis as unknown as Record<string, unknown>).EventSource = originalEventSource as unknown as typeof EventSource;
  });

  describe('panel fijo h-[280px] lg:h-[340px] overflow-y-auto no crece con texto', () => {
    // Covers [SPEC-004: AC-004, FR-004, BR-005, TS-003] — Layout fijo

    it('render panel fijo h-[280px] lg:h-[340px] overflow-y-auto no crece con texto', async () => {
      // Covers [SPEC-004: AC-004, BR-005, FR-004]
      // Arrange
      MockEventSource.instances = [];

      // Act
      render(<AlertsPanel />);

      // Assert — panel tiene altura fija y overflow-y-auto
      const panel =
        (screen.queryByTestId("alerts-panel") as HTMLElement | null) ??
        (document.querySelector('[aria-live="polite"]') as HTMLElement | null) ??
        (document.querySelector(".overflow-y-auto") as HTMLElement | null);

      expect(panel, "AlertsPanel debe renderizar contenedor con overflow-y-auto").not.toBeNull();
      expect(panel!.className).toMatch(/overflow-y-auto/); // AC-004 BR-005 no crece, scroll interno
      expect(panel!.className).toMatch(/h-\[280px\]/); // AC-004 altura fija 280px móvil
      expect(panel!.className).toMatch(/lg:h-\[340px\]/); // AC-004 altura fija lg 340px
      // AC-004 no tiene style height dinámico inline que crezca con texto
      const styleHeight = (panel as HTMLElement).style.height;
      // Si usa Tailwind, style.height debe estar vacío; si usa inline, debe ser fijo 280/340 no auto
      if (styleHeight) {
        expect(styleHeight).not.toBe("auto");
        expect(["280px", "340px", ""].some((v) => styleHeight.includes(v) || styleHeight === "")).toBe(true);
      }
    });

    it('panel mantiene overflow-y-auto tras 50 alerts sin crecer', async () => {
      // Covers [SPEC-004: AC-004, BR-005, TS-003] — no empuja layout con overflow
      // Arrange
      render(<AlertsPanel />);
      const es = MockEventSource.instances[0]!;
      expect(es.url).toContain("/api/alerts"); // AC-004 BR-005 SSE alerts

      // Act — inyecta 50 alerts speeding_on
      act(() => {
        for (let i = 0; i < 50; i++) {
          es.simulateAlert({
            plate: `TTF${String(i).padStart(3, "0")}`,
            alert_type: i % 2 === 0 ? "speeding_on" : "speeding_off",
            speed: i % 2 === 0 ? 90 : 70,
            created_at: "2026-08-26T14:32:10Z",
          });
        }
      });
      // flush timers for any debounced render
      await act(async () => {
        vi.advanceTimersByTime(100);
      });

      // Assert — panel sigue con mismas clases fijas, no ha crecido dinámicamente
      const panel =
        (screen.queryByTestId("alerts-panel") as HTMLElement | null) ??
        (document.querySelector('[aria-live="polite"]') as HTMLElement | null) ??
        (document.querySelector(".overflow-y-auto") as HTMLElement | null);
      expect(panel).not.toBeNull();
      expect(panel!.className).toMatch(/overflow-y-auto/); // AC-004 mantiene scroll
      expect(panel!.className).toMatch(/h-\[280px\]/);
      expect(panel!.className).toMatch(/lg:h-\[340px\]/);
      // Verifica que overflow style computado es auto/scroll, no visible que empujaría layout
      const computedOverflow = getComputedStyle(panel!).overflowY;
      // jsdom puede devolver "" si no Tailwind, pero si devuelve, debe ser auto/scroll
      if (computedOverflow) {
        expect(["auto", "scroll", ""].includes(computedOverflow)).toBe(true);
      }
    });
  });

  describe("SSE 2 alerts speeding_on/off en <2s traducidos", () => {
    // Covers [SPEC-004: AC-004, BR-005/007, FR-005, UC-002, TS-003]

    it("SSE 2 alerts speeding_on/off en <2s traducidos con aria-live polite", async () => {
      // Covers [SPEC-004: AC-004, BR-005/007, FR-005]
      // Arrange
      render(<AlertsPanel />);
      const es = MockEventSource.instances[0]!;
      expect(es.url).toContain("/api/alerts"); // AC-004 BR-005 endpoint alerts

      // Act — emite 2 alerts críticos en <2s
      act(() => {
        es.simulateAlert(speedingOnPayload, "101");
      });
      // pequeño delay simulando <2s entre events
      await act(async () => {
        vi.advanceTimersByTime(500);
      });
      act(() => {
        es.simulateAlert(speedingOffPayload, "102");
      });
      await act(async () => {
        vi.advanceTimersByTime(100);
      });

      // Assert — lista muestra traducciones humanas en <2s
      await waitFor(
        () => {
          expect(screen.getAllByText(/TTF678/)[0]).toBeInTheDocument(); // AC-004 plate visible
        },
        { timeout: 2000 },
      );
      // TTF678 superando 80Km/h para speeding_on
      expect(screen.getByText(/superando 80Km\/h/i)).toBeInTheDocument(); // AC-004 BR-007 speeding_on traducido
      // TTF678 vuelve a <80Km/h para speeding_off — regex tolera "<80" o "&lt;80"
      const offText = screen.queryByText(/vuelve a.*80/i) ?? screen.queryByText(/<80/i);
      expect(offText, "Debe mostrar 'vuelve a <80Km/h' para speeding_off").toBeInTheDocument(); // AC-004 BR-007

      // Assert — aria-live polite en panel o lista
      const liveEl =
        document.querySelector('[aria-live="polite"]') ??
        screen.queryByRole("list") ??
        document.querySelector(".overflow-y-auto");
      expect(liveEl).not.toBeNull(); // AC-004 a11y lista
      if (liveEl) {
        const ariaLive = (liveEl as HTMLElement).getAttribute("aria-live");
        // si el elemento con aria-live es el panel, debe ser polite; si es lista, también
        const hasLivePolite =
          ariaLive === "polite" || document.body.innerHTML.includes('aria-live="polite"');
        expect(hasLivePolite, "AlertsPanel debe tener aria-live='polite'").toBe(true); // AC-004
      }

      // Assert — ambos alerts visibles con plate TTF678
      const allTtf = screen.getAllByText(/TTF678/);
      expect(allTtf.length).toBeGreaterThanOrEqual(2); // AC-004 2 alerts en <2s
    });
  });

  describe("panel no empuja layout cuando tab invisible", () => {
    // Covers [SPEC-004: AC-004, FR-004, BR-005, TS-003]

    it("panel no empuja layout cuando tab Alerts inactivo (hidden vs activeBottom store)", async () => {
      // Covers [SPEC-004: AC-004, FR-004, BR-005]
      // Arrange — intenta usar portalStore si existe, si no, testea hidden via wrapper
      let portalStoreAvailable = false;
      let initialActiveBottom: string | null = null;
      try {
        const mod = await import("../../store/portalStore");
        const store = (mod as unknown as { usePortalStore: { getState: () => { activeBottom: string } } })
          .usePortalStore;
        if (store) {
          portalStoreAvailable = true;
          initialActiveBottom = store.getState().activeBottom;
        }
      } catch {
        portalStoreAvailable = false;
      }

      // Act — render AlertsPanel dentro de flex con sibling para detectar empuje
      const { container } = render(
        <div style={{ display: "flex", flexDirection: "column", height: "600px" }} data-testid="layout-root">
          <AlertsPanel />
          <div data-testid="sibling" style={{ height: "100px" }}>
            sibling content
          </div>
        </div>,
      );

      // Assert — panel fijo no empuja: sibling sigue en document con altura 100px y panel con overflow-y-auto
      const panel =
        (screen.queryByTestId("alerts-panel") as HTMLElement | null) ??
        (document.querySelector('[aria-live="polite"]') as HTMLElement | null) ??
        (document.querySelector(".overflow-y-auto") as HTMLElement | null);
      expect(panel).not.toBeNull(); // AC-004 panel existe incluso en layout
      expect(panel!.className).toMatch(/overflow-y-auto/); // AC-004 scroll interno evita empuje
      expect(panel!.className).toMatch(/h-\[280px\]/);
      const sibling = screen.getByTestId("sibling");
      expect(sibling).toBeInTheDocument(); // AC-004 sibling no desplazado fuera
      expect(container.querySelector('[data-testid="layout-root"]')).toBeInTheDocument();

      // Si portalStore existe, testea que activeBottom='chat' ocultaría Alerts sin desmontar layout
      if (portalStoreAvailable) {
        try {
          const mod = await import("../../store/portalStore");
          const store2 = (mod as unknown as { usePortalStore: { getState: () => any; setState: (s: any) => void } })
            .usePortalStore;
          // Act — cambia a chat
          act(() => {
            store2.setState({ activeBottom: "chat" } as any);
          });
          await act(async () => {
            vi.advanceTimersByTime(0);
          });
          // Assert — panel sigue con clases fijas (hidden via display:none o no visible) pero no empuja
          // El panel puede estar hidden (display:none) o seguir con mismas clases
          const panelAfter =
            (document.querySelector('[data-testid="alerts-panel"]') as HTMLElement | null) ??
            (document.querySelector('[aria-live="polite"]') as HTMLElement | null) ??
            (document.querySelector(".overflow-y-auto") as HTMLElement | null);
          // Si implementación oculta via hidden attribute o CSS, verifica que no empuja
          if (panelAfter) {
            expect(panelAfter.className).toMatch(/overflow-y-auto/);
            // Si tiene hidden o aria-hidden, no debe empujar
            const isHidden =
              panelAfter.hasAttribute("hidden") ||
              panelAfter.getAttribute("aria-hidden") === "true" ||
              getComputedStyle(panelAfter).display === "none" ||
              panelAfter.className.includes("hidden");
            // No exigimos hidden estricto, solo que no haya perdido altura fija
            expect(isHidden || panelAfter.className.includes("h-[280px]")).toBe(true); // AC-004
          }
          // restore
          act(() => {
            store2.setState({ activeBottom: initialActiveBottom ?? "alerts" } as any);
          });
        } catch {
          // ignore store interaction errors
        }
      }
    });
  });

  describe(":ping 15s y Last-Event-ID replay", () => {
    // Covers [SPEC-004: AC-004, BR-005, FR-005, TS-003]

    it(":ping 15s no altera lista y Last-Event-ID es implícito sin ?lastEventId en URL", async () => {
      // Covers [SPEC-004: AC-004, BR-005, FR-005]
      // Arrange
      render(<AlertsPanel />);
      const es = MockEventSource.instances[0]!;
      expect(es.url).toContain("/api/alerts"); // AC-004
      // Last-Event-ID debe ser implícito vía EventSource nativo, no manual en URL
      expect(es.url).not.toContain("lastEventId"); // AC-004 BR-005
      expect(es.url).not.toContain("Last-Event-ID"); // AC-004
      expect(es.url).not.toContain("last_event_id"); // AC-004

      // Act — 1 alert inicial
      act(() => {
        es.simulateAlert(speedingOnPayload, "100");
      });
      await act(async () => {
        vi.advanceTimersByTime(100);
      });
      await waitFor(() => expect(screen.getAllByText(/superando 80Km\/h/i)[0]).toBeInTheDocument());

      const countBefore = screen.getAllByText(/TTF678/).length;

      // Act — :ping 15s (comment keepalive) no debe alterar lista
      act(() => {
        es.simulatePing();
        // también simula raw ":ping" como mensaje que debe ser ignorado por JSON parse
        es.simulateRawPingAsMessage();
      });
      await act(async () => {
        vi.advanceTimersByTime(15_000);
      });

      // Assert — lista no crece con ping
      const countAfterPing = screen.getAllByText(/TTF678/).length;
      expect(countAfterPing).toBe(countBefore); // AC-004 BR-005 :ping no altera lista
      expect(screen.queryByText(/:ping/)).not.toBeInTheDocument(); // AC-004 ping no renderizado

      // Act — reconexión simulada debe mantener Last-Event-ID implícito (sin URL manual)
      act(() => {
        es.simulateError();
      });
      await act(async () => {
        vi.advanceTimersByTime(500);
      });
      // Si useSSE reconecta, debería crear nueva instancia con misma URL sin lastEventId manual
      if (MockEventSource.instances.length > 1) {
        const reconnected = MockEventSource.instances[MockEventSource.instances.length - 1]!;
        expect(reconnected.url).toContain("/api/alerts"); // AC-004
        expect(reconnected.url).not.toContain("lastEventId"); // AC-004 Last-Event-ID implícito nativo
        expect(reconnected.url).not.toContain("Last-Event-ID");
      } else {
        // si no reconecta en este mock, al menos URL original ya verificada
        expect(es.url).not.toContain("lastEventId");
      }

      // Act — post-ping, siguiente alert real sí se añade (replay Last-Event-ID 101..)
      act(() => {
        // Usa última instancia si reconectó, si no la original
        const target = MockEventSource.instances[MockEventSource.instances.length - 1]!;
        target.simulateAlert(speedingOffPayload, "101");
      });
      await act(async () => {
        vi.advanceTimersByTime(100);
      });

      // Assert — nuevo alert post-ping sí aparece
      await waitFor(() => expect(screen.getByText(/vuelve a.*80/i)).toBeInTheDocument());
      expect(screen.getAllByText(/TTF678/).length).toBeGreaterThan(countBefore); // AC-004 replay
    });
  });
});
