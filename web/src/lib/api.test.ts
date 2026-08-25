// Covers [SPEC-002: AC-001, BR-007]
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { getApiBase } from "./api";

describe("getApiBase", () => {
  // Covers [SPEC-002: AC-001, BR-007]

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it("returns empty string when VITE_API_BASE_URL undefined", async () => {
    // Covers [SPEC-002: AC-001, BR-007]
    // Arrange
    vi.stubEnv("VITE_API_BASE_URL", "");
    const { getApiBase: fresh } = await import("./api");
    // Act
    const got = fresh();
    // Assert
    expect(got).toBe(""); // BR-007 empty means relative /api/chat fallback
  });

  it("trims trailing slash", async () => {
    // Covers [SPEC-002: AC-001, BR-007]
    // Arrange
    vi.stubEnv("VITE_API_BASE_URL", "http://localhost:8080/");
    const { normalizeBase } = await import("./api");
    // Act
    const got = normalizeBase("http://localhost:8080/");
    // Assert
    expect(got).toBe("http://localhost:8080");
    // Also verify getApiBase reflects stub via procEnv
    const { getApiBase: fresh } = await import("./api");
    expect(fresh()).toBe("http://localhost:8080");
  });

  it("trims only single trailing slash and keeps path", async () => {
    // Covers [SPEC-002: AC-001, BR-007]
    // Arrange
    const { normalizeBase } = await import("./api");
    // Act
    const got = normalizeBase("https://api.example.com/api/");
    // Assert
    expect(got).toBe("https://api.example.com/api");
  });

  it("returns without modification when no trailing slash", async () => {
    // Covers [SPEC-002: AC-001, BR-007]
    // Arrange
    const { normalizeBase } = await import("./api");
    // Act
    const got = normalizeBase("https://api.example.com");
    // Assert
    expect(got).toBe("https://api.example.com");
  });

  it("handles empty string env", async () => {
    // Covers [SPEC-002: AC-001, BR-007]
    // Arrange
    const { normalizeBase } = await import("./api");
    // Act
    const got = normalizeBase("");
    // Assert
    expect(got).toBe("");
  });
});
