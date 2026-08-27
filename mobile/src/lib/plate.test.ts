import { normalizePlate, isValidPlate } from './plate';

// Covers [SPEC-005: AC-001] - FR-001 BR-001/002 TS-001: Plate SSOT normalize + validate
describe('plate lib', () => {
  describe('normalizePlate', () => {
    it('normalizes lower case to upper case', () => {
      // Arrange
      const input = 'acf356';
      const expected = 'ACF356';

      // Act
      const result = normalizePlate(input);

      // Assert
      expect(result).toBe(expected);
    });

    it('trims whitespace and uppercases', () => {
      // Arrange
      const input = '  acf356 ';
      const expected = 'ACF356';

      // Act
      const result = normalizePlate(input);

      // Assert
      expect(result).toBe(expected);
    });

    it('trims tabs and mixed case', () => {
      // Arrange
      const input = '\t AcF356 \n';
      const expected = 'ACF356';

      // Act
      const result = normalizePlate(input);

      // Assert
      expect(result).toBe(expected);
    });
  });

  describe('isValidPlate', () => {
    it('returns true for valid plate ACF356', () => {
      // Arrange
      const plate = 'ACF356';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(true);
    });

    it('returns true for valid plate after normalize lower case', () => {
      // Arrange
      const plate = 'acf356';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(true);
    });

    it('returns false for too short plate ACF35 (5 chars)', () => {
      // Arrange
      const plate = 'ACF35';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(false);
    });

    it('returns false for too long plate ACF3567 (7 chars)', () => {
      // Arrange
      const plate = 'ACF3567';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(false);
    });

    it('returns false for invalid pattern 12ABC3', () => {
      // Arrange
      const plate = '12ABC3';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(false);
    });

    it('returns false for empty string', () => {
      // Arrange
      const plate = '';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(false);
    });

    it('returns false for plate with hyphen ACF-356', () => {
      // Arrange
      const plate = 'ACF-35';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(false);
    });

    it('returns false for plate with space ACF 356', () => {
      // Arrange
      const plate = 'ACF 356';

      // Act
      const result = isValidPlate(plate);

      // Assert
      expect(result).toBe(false);
    });
  });
});
