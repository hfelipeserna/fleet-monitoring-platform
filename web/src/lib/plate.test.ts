// Covers [SPEC-004: AC-003, BR-002]
import { describe, it, expect } from "vitest";
import { isValidPlate } from "./plate";

describe("isValidPlate", () => {
  // Covers [SPEC-004: AC-003, BR-002] — regex ^[A-Z]{3}[0-9]{3}$
  describe("regex validation disables Search when invalid", () => {
    // Covers [SPEC-004: AC-003, BR-002]

    it("TTF67 5 chars -> false disable Search", () => {
      // Arrange
      const input = "TTF67";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 5 chars invalido debe deshabilitar Search
    });

    it("TTF678 valid -> true enable", () => {
      // Arrange
      const input = "TTF678";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(true); // AC-003 BR-002 3 letras + 3 digitos habilita Search
    });

    it("lowercase ttf678 -> false", () => {
      // Arrange
      const input = "ttf678";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 lowercase invalido
    });

    it("GTP980 valid", () => {
      // Arrange
      const input = "GTP980";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(true); // AC-003 BR-002 GTP980 debe ser valido
    });
  });

  describe("bordes adicionales", () => {
    // Covers [SPEC-004: AC-003, BR-002]

    it("empty string -> false", () => {
      // Arrange
      const input = "";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 vacio invalido
    });

    it("TTF6789 7 chars -> false", () => {
      // Arrange
      const input = "TTF6789";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 7 chars invalido
    });

    it("TTF 678 con espacio -> false", () => {
      // Arrange
      const input = "TTF 678";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 espacio invalido
    });

    it("123456 solo digitos -> false", () => {
      // Arrange
      const input = "123456";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 solo digitos invalido
    });

    it("ABCDEF solo letras -> false", () => {
      // Arrange
      const input = "ABCDEF";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 solo letras invalido
    });

    it("TTF67A mix invalido -> false", () => {
      // Arrange
      const input = "TTF67A";
      // Act
      const result = isValidPlate(input);
      // Assert
      expect(result).toBe(false); // AC-003 BR-002 patron incorrecto
    });
  });
});
