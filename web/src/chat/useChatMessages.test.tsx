// Covers [SPEC-003: AC-006, AC-007, BR-008]
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useChatMessages } from "./useChatMessages";

describe("useChatMessages", () => {
  // Covers [SPEC-003: AC-006, AC-007, BR-008]

  beforeEach(() => {
    // Arrange deterministic Math.random (setup.ts already stubs to 0)
    vi.spyOn(Date, "now").mockReturnValue(1000000);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("initial messages empty", () => {
    // Covers [SPEC-003: AC-006, BR-008]
    // Arrange
    const { result } = renderHook(() => useChatMessages());
    // Act
    const msgs = result.current.messages;
    // Assert
    expect(msgs).toHaveLength(0);
  });

  it("push adds message with id", () => {
    // Covers [SPEC-003: AC-006, BR-008]
    // Arrange
    const { result } = renderHook(() => useChatMessages());
    // Act
    act(() => {
      result.current.push({ role: "user", content: "hola" });
    });
    // Assert
    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]!.content).toBe("hola");
    expect(result.current.messages[0]!.role).toBe("user");
    expect(result.current.messages[0]!.id).toBeDefined();
  });

  it("pushUser adds user message", () => {
    // Covers [SPEC-003: AC-006, BR-008]
    // Arrange
    const { result } = renderHook(() => useChatMessages());
    // Act
    act(() => {
      result.current.pushUser("¿qué vehículos?");
    });
    // Assert
    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]!.role).toBe("user");
    expect(result.current.messages[0]!.content).toBe("¿qué vehículos?");
  });

  it("pushAssistant adds assistant with citations", () => {
    // Covers [SPEC-003: AC-007, BR-008]
    // Arrange
    const { result } = renderHook(() => useChatMessages());
    const citations = [{ tool: "findVehiclesStoppedInCriticalZones", count: 2 }];
    // Act
    act(() => {
      result.current.pushAssistant("GTP980 detenido 27m", citations);
    });
    // Assert
    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]!.role).toBe("assistant");
    expect(result.current.messages[0]!.citations).toEqual(citations);
  });

  it("limit caps messages to last N", () => {
    // Covers [SPEC-003: AC-006, BR-008]
    // Arrange
    const { result } = renderHook(() => useChatMessages(2));
    // Act
    act(() => {
      result.current.pushUser("m1");
      result.current.pushUser("m2");
      result.current.pushUser("m3");
    });
    // Assert
    expect(result.current.messages).toHaveLength(2);
    expect(result.current.messages[0]!.content).toBe("m2");
    expect(result.current.messages[1]!.content).toBe("m3");
  });

  it("default limit 50 keeps all under threshold", () => {
    // Covers [SPEC-003: AC-006, BR-008]
    // Arrange
    const { result } = renderHook(() => useChatMessages());
    // Act
    act(() => {
      for (let i = 0; i < 5; i++) result.current.pushUser(`m${i}`);
    });
    // Assert
    expect(result.current.messages).toHaveLength(5);
  });

  it("fallback id when crypto.randomUUID throws", () => {
    // Covers [SPEC-003: AC-006, BR-008]
    // Arrange
    const origCrypto = globalThis.crypto;
    const fakeCrypto = {
      randomUUID: () => {
        throw new Error("no uuid");
      },
    } as unknown as Crypto;
    // jsdom crypto is getter only; use defineProperty
    Object.defineProperty(globalThis, "crypto", { value: fakeCrypto, writable: true, configurable: true });
    const { result } = renderHook(() => useChatMessages());
    // Act
    act(() => {
      result.current.pushUser("fallback");
    });
    // Assert
    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]!.id).toMatch(/1000000-/); // Date.now mock 1000000
    // restore
    Object.defineProperty(globalThis, "crypto", { value: origCrypto, writable: true, configurable: true });
  });
});
