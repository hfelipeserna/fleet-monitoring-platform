// Covers [SPEC-002: AC-007, BR-008]
import { describe, it, expect, beforeEach } from "vitest";
import { useFleetStore } from "./fleetStore";

describe("useFleetStore", () => {
  // Covers [SPEC-002: AC-007, BR-008]

  beforeEach(() => {
    // Arrange reset store
    useFleetStore.setState({ selectedPlate: null });
  });

  it("initial selectedPlate is null", () => {
    // Covers [SPEC-002: AC-007, BR-008]
    // Arrange
    const state = useFleetStore.getState();
    // Act
    const plate = state.selectedPlate;
    // Assert
    expect(plate).toBeNull();
  });

  it("setSelectedPlate sets plate", () => {
    // Covers [SPEC-002: AC-007, BR-008]
    // Arrange
    const { setSelectedPlate } = useFleetStore.getState();
    // Act
    setSelectedPlate("GTP980");
    // Assert
    expect(useFleetStore.getState().selectedPlate).toBe("GTP980");
  });

  it("setSelectedPlate null clears", () => {
    // Covers [SPEC-002: AC-007, BR-008]
    // Arrange
    useFleetStore.setState({ selectedPlate: "GTP980" });
    const { setSelectedPlate } = useFleetStore.getState();
    // Act
    setSelectedPlate(null);
    // Assert
    expect(useFleetStore.getState().selectedPlate).toBeNull();
  });

  it("setSelectedPlate overwrites previous", () => {
    // Covers [SPEC-002: AC-007, BR-008]
    // Arrange
    const { setSelectedPlate } = useFleetStore.getState();
    setSelectedPlate("AAA111");
    // Act
    setSelectedPlate("BBB222");
    // Assert
    expect(useFleetStore.getState().selectedPlate).toBe("BBB222");
  });

  it("selector subscription notifies", () => {
    // Covers [SPEC-002: AC-007, BR-008]
    // Arrange
    let observed: string | null = "init-sentinel" as unknown as string | null;
    const unsub = useFleetStore.subscribe((s) => {
      observed = s.selectedPlate;
    });
    // Act
    useFleetStore.getState().setSelectedPlate("TTY423");
    // Assert
    expect(observed).toBe("TTY423");
    unsub();
  });
});
