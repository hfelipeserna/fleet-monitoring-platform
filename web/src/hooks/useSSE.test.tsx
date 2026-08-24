// Covers [SPEC-002: AC-007/008/009, BR-008/009]
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSSE } from "./useSSE";

// --- MockEventSource (msw-compatible minimal) ---
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
  addEventListener = vi.fn();
  removeEventListener = vi.fn();
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
  simulateMessage(data: string, lastEventId?: string) {
    const evt = new MessageEvent("message", { data } as MessageEventInit);
    if (lastEventId) (evt as unknown as Record<string, string>).lastEventId = lastEventId;
    this.onmessage?.(evt);
  }
}

describe("useSSE", () => {
  // Covers [SPEC-002: AC-007/008/009, BR-008/009]
  let originalEventSource: unknown;

  beforeEach(() => {
    // Arrange global mock
    MockEventSource.instances = [];
    originalEventSource = (globalThis as unknown as Record<string, unknown>).EventSource;
    (globalThis as unknown as Record<string, unknown>).EventSource = MockEventSource as unknown as typeof EventSource;
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    MockEventSource.instances = [];
    (globalThis as unknown as Record<string, unknown>).EventSource = originalEventSource as unknown as typeof EventSource;
  });

  it("backoff on error 0.5s *2 cap 30s", async () => {
    // Arrange
    const url = "/api/alerts";
    const { unmount } = renderHook(() => useSSE(url));

    // Act — first EventSource created synchronously
    expect(MockEventSource.instances).toHaveLength(1); // AC-007 initial connect

    const first = MockEventSource.instances[0]!;
    // trigger error -> should schedule reconnect after 500ms
    act(() => {
      first.simulateError();
    });

    // Assert — before 500ms no reconnect
    expect(MockEventSource.instances).toHaveLength(1); // AC-007 not yet reconnected

    // Act — advance 500ms -> 2nd instance with backoff 500
    act(() => {
      vi.advanceTimersByTime(500);
    });
    // Assert
    expect(MockEventSource.instances).toHaveLength(2); // AC-007 backoff 0.5s
    expect(MockEventSource.instances[1]!.url).toBe(url); // AC-007 same URL, Last-Event-ID implicit via native EventSource

    // Act — 2nd error -> backoff 1000ms
    act(() => {
      MockEventSource.instances[1]!.simulateError();
    });
    act(() => {
      vi.advanceTimersByTime(999);
    });
    expect(MockEventSource.instances).toHaveLength(2); // AC-007 still waiting 1s
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(MockEventSource.instances).toHaveLength(3); // AC-007 backoff 1s

    // Act — escalate to cap 30s (simulate repeated errors doubling)
    // 3rd -> 2s, 4th ->4s, 5th->8s, 6th->16s, 7th->32s capped 30s
    const expectedBackoffs = [2000, 4000, 8000, 16000, 30000, 30000];
    for (const backoff of expectedBackoffs) {
      const last = MockEventSource.instances[MockEventSource.instances.length - 1]!;
      act(() => {
        last.simulateError();
        vi.advanceTimersByTime(backoff);
      });
      expect(MockEventSource.instances[MockEventSource.instances.length - 1]!.url).toBe(url); // AC-007
    }
    // Assert final cap
    expect(MockEventSource.instances.length).toBeGreaterThan(7); // AC-007 cap exercised
    // cleanup
    unmount();
  });

  it("onopen resets backoff to 0.5s", async () => {
    // Arrange
    const url = "/api/fleet/positions/stream";
    renderHook(() => useSSE(url));
    const first = MockEventSource.instances[0]!;

    // Act — error -> backoff 500 then open resets
    act(() => {
      first.simulateError();
      vi.advanceTimersByTime(500);
    });
    expect(MockEventSource.instances).toHaveLength(2); // AC-007 500ms backoff

    const second = MockEventSource.instances[1]!;
    act(() => {
      second.simulateOpen();
    });

    // Act — next error should again be 500ms (reset), not 1000ms
    act(() => {
      second.simulateError();
      vi.advanceTimersByTime(500);
    });

    // Assert
    expect(MockEventSource.instances).toHaveLength(3); // AC-007 reset to 0.5s
    expect(MockEventSource.instances[2]!.url).toBe(url); // AC-007
  });

  it("cleanup calls es.close on unmount (no leak)", async () => {
    // Arrange
    const url = "/api/alerts";
    const { unmount } = renderHook(() => useSSE(url));
    const es = MockEventSource.instances[0]!;

    // Act
    unmount();

    // Assert
    expect(es.close).toHaveBeenCalledTimes(1); // AC-007 close on unmount
    // AC-007 Last-Event-ID is implicit via native EventSource (no manual ?lastEventId)
    expect(es.url).not.toContain("lastEventId");
    expect(es.url).not.toContain("Last-Event-ID");
  });

  it("toggle filter reconecta sin ?plate (ver todos)", async () => {
    // Arrange
    const withPlate = "/api/fleet/positions/stream?plate=GTP980";
    const withoutPlate = "/api/fleet/positions/stream";
    const { rerender, unmount } = renderHook(({ u }) => useSSE(u), {
      initialProps: { u: withPlate },
    });

    // Assert initial plate filtered
    expect(MockEventSource.instances[0]!.url).toBe(withPlate); // AC-007 plate filter
    expect(MockEventSource.instances[0]!.url).toContain("?plate=GTP980"); // AC-007

    // Act — toggle to "Ver todos" (sin ?plate)
    rerender({ u: withoutPlate });

    // Need to flush timers/effects
    await act(async () => {
      vi.advanceTimersByTime(0);
    });

    // Assert — new EventSource without ?plate, previous closed
    expect(MockEventSource.instances[0]!.close).toHaveBeenCalled(); // AC-007 previous closed on url change
    const last = MockEventSource.instances[MockEventSource.instances.length - 1]!;
    expect(last.url).toBe(withoutPlate); // AC-007 sin filtro
    expect(last.url).not.toContain("?plate"); // AC-007 Ver todos sin ?plate
    // Last-Event-ID implícito: url no debe llevar lastEventId manual
    expect(last.url).not.toContain("lastEventId"); // AC-007

    unmount();
  });
});
