// Covers [SPEC-004: BR-002]
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { usePlateHighlight } from "./usePlateHighlight";
import { useFleetStore } from "../store/fleetStore";

describe("usePlateHighlight", () => {
  // Covers [SPEC-004: BR-002]

  beforeEach(() => {
    // Arrange — reset Zustand store real
    useFleetStore.setState({ selectedPlate: null });
  });

  it("extrae TTF678 de reply y setea selectedPlate", () => {
    // Covers [SPEC-004: BR-002]
    // Arrange
    const { result } = renderHook(() => usePlateHighlight());
    expect(useFleetStore.getState().selectedPlate).toBeNull();

    // Act
    act(() => {
      result.current.highlightFromReply("GTP980 lleva 27m detenido");
    });

    // Assert
    expect(useFleetStore.getState().selectedPlate).toBe("GTP980");
    expect(result.current.selectedPlate).toBe("GTP980");
  });

  it("reply sin placa no cambia selectedPlate", () => {
    // Covers [SPEC-004: BR-002]
    // Arrange
    const { result } = renderHook(() => usePlateHighlight());
    expect(useFleetStore.getState().selectedPlate).toBeNull();

    // Act
    act(() => {
      result.current.highlightFromReply("sin placa aquí");
    });

    // Assert
    expect(useFleetStore.getState().selectedPlate).toBeNull();
    expect(result.current.selectedPlate).toBeNull();
  });

  it("múltiples placas usa la primera con /g", () => {
    // Covers [SPEC-004: BR-002]
    // Arrange
    const { result } = renderHook(() => usePlateHighlight());

    // Act
    act(() => {
      result.current.highlightFromReply("TTF678 y GTP980 detenidos en zona");
    });

    // Assert
    expect(useFleetStore.getState().selectedPlate).toBe("TTF678");
    expect(result.current.selectedPlate).toBe("TTF678");
  });

  it("no falla con lastIndex al llamar 2 veces seguidas", () => {
    // Covers [SPEC-004: BR-002]
    // Arrange
    const { result } = renderHook(() => usePlateHighlight());

    // Act
    act(() => {
      result.current.highlightFromReply("TTF678 detenido por speeding");
    });

    // Assert
    expect(useFleetStore.getState().selectedPlate).toBe("TTF678");

    // Arrange — reset para probar que el segundo match no es flaky por lastIndex global
    act(() => {
      useFleetStore.setState({ selectedPlate: null });
    });

    // Act
    act(() => {
      result.current.highlightFromReply("TTF678 detenido por speeding");
    });

    // Assert
    expect(useFleetStore.getState().selectedPlate).toBe("TTF678");
    expect(result.current.selectedPlate).toBe("TTF678");
  });

  it("lowercase no matchea (strict mayus)", () => {
    // Covers [SPEC-004: BR-002]
    // Arrange
    const { result } = renderHook(() => usePlateHighlight());

    // Act
    act(() => {
      result.current.highlightFromReply("ttf678 detenido");
    });

    // Assert
    expect(useFleetStore.getState().selectedPlate).toBeNull();
    expect(result.current.selectedPlate).toBeNull();
  });
});
