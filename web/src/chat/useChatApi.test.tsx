// Covers [SPEC-003: AC-006, AC-007, BR-007, BR-008]
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { useChatApi } from "./useChatApi";

describe("useChatApi", () => {
  // Covers [SPEC-003: AC-006, AC-007, BR-007, BR-008]
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
    (import.meta as unknown as { env: Record<string, string> }).env = {};
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("empty trimmed returns null without fetch", async () => {
    // Covers [SPEC-003: AC-006, BR-007]
    // Arrange
    const fetchMock = vi.fn();
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    let got: unknown;
    await act(async () => {
      got = await result.current.sendMessage("   ");
    });
    // Assert
    expect(got).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("successful fetch returns data and clears error loading false", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    const data = { reply: "GTP980 detenido 27m", citations: [{ tool: "findVehiclesStoppedInCriticalZones", count: 2 }] };
    globalThis.fetch = vi.fn(async () =>
      new Response(JSON.stringify(data), { status: 200, headers: { "Content-Type": "application/json" } }),
    ) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    let got: unknown;
    await act(async () => {
      got = await result.current.sendMessage("hola");
    });
    // Assert
    expect(got).toEqual(data);
    expect(result.current.error).toBeNull();
    expect(result.current.loading).toBe(false);
  });

  it("429 sets rate limited error with Retry-After", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () =>
      new Response(JSON.stringify({ error: "rate limited" }), { status: 429, headers: { "Retry-After": "6" } }),
    ) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    let got: unknown;
    await act(async () => {
      got = await result.current.sendMessage("hola");
    });
    // Assert
    expect(got).toBeNull();
    expect(result.current.error).toContain("429");
    expect(result.current.error).toContain("Retry-After");
    expect(result.current.error).toContain("6");
  });

  it("429 without Retry-After defaults to 6", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => new Response("rate", { status: 429 })) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    // Assert
    expect(result.current.error).toContain("6");
  });

  it("503 sets agente temporalmente no disponible", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => new Response("unavailable", { status: 503 })) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      const r = await result.current.sendMessage("hola");
      expect(r).toBeNull();
    });
    // Assert
    expect(result.current.error).toBe("agente temporalmente no disponible");
  });

  it("!ok reads text error", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => new Response("bad request zod", { status: 400 })) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    // Assert
    expect(result.current.error).toBe("bad request zod");
  });

  it("!ok with empty text falls back to error status", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => new Response("", { status: 500 })) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    // Assert
    expect(result.current.error).toBe("error 500");
  });

  it("AbortError sets timeout 15s message", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => {
      const e = new DOMException("aborted", "AbortError");
      throw e;
    }) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    // Assert
    expect(result.current.error).toBe("timeout 15s agente no disponible");
  });

  it("generic network error sets message", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => {
      throw new Error("network down");
    }) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    // Assert
    expect(result.current.error).toBe("network down");
  });

  it("error without message falls back to error de red", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => {
      throw {};
    }) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    // Assert
    expect(result.current.error).toBe("error de red");
  });

  it("Expected signal fallback retries without signal", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    const data = { reply: "ok retry" };
    let calls = 0;
    globalThis.fetch = vi.fn(async (_url: string, opts?: RequestInit) => {
      calls++;
      if (calls === 1) {
        // first call with signal throws Expected signal
        if (opts?.signal) throw new Error("Expected signal to be an instance of AbortSignal");
        throw new Error("should have signal");
      }
      return new Response(JSON.stringify(data), { status: 200 });
    }) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    let got: unknown;
    await act(async () => {
      got = await result.current.sendMessage("hola");
    });
    // Assert
    expect(calls).toBe(2);
    expect(got).toEqual(data);
  });

  it("non Expected signal error is thrown and handled as network error", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => {
      throw new Error("other signal failure");
    }) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    // Assert
    expect(result.current.error).toBe("other signal failure");
  });

  it("clearError clears error", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    globalThis.fetch = vi.fn(async () => new Response("", { status: 500 })) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    await act(async () => {
      await result.current.sendMessage("hola");
    });
    expect(result.current.error).not.toBeNull();
    // Act
    act(() => {
      result.current.clearError();
    });
    // Assert
    expect(result.current.error).toBeNull();
  });

  it("loading true during fetch and false after", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    let resolve!: (v: Response) => void;
    globalThis.fetch = vi.fn(() => new Promise<Response>((res) => (resolve = res))) as unknown as typeof fetch;
    const { result } = renderHook(() => useChatApi());
    // Act
    let promise: Promise<unknown>;
    act(() => {
      promise = result.current.sendMessage("hola");
    });
    // Assert loading true immediately after call (before resolve)
    // need to flush microtask for setLoading
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.loading).toBe(true);
    const data = { reply: "done" };
    await act(async () => {
      resolve(new Response(JSON.stringify(data), { status: 200 }));
      await promise;
    });
    expect(result.current.loading).toBe(false);
  });

  it("unmount aborts controller and clears timeout", async () => {
    // Covers [SPEC-003: AC-007, BR-007]
    // Arrange
    vi.useFakeTimers();
    const abortSpy = vi.spyOn(AbortController.prototype, "abort");
    globalThis.fetch = vi.fn(() => new Promise(() => {})) as unknown as typeof fetch;
    const { result, unmount } = renderHook(() => useChatApi());
    act(() => {
      // fire and forget, will set timeout 15000
      void result.current.sendMessage("hola");
    });
    // advance to 0 to ensure timer set
    act(() => {
      vi.advanceTimersByTime(0);
    });
    // Act
    unmount();
    // Assert
    expect(abortSpy).toHaveBeenCalled();
    abortSpy.mockRestore();
  });
});
