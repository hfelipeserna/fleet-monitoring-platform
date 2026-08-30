// Covers [SPEC-005: AC-006] FR-006 FR-007 BR-007 BR-008 TS-006
// TASK-005-06 TDD RED: Rutas Bogotá ~20 pts, lat/lon reales, speed 0/45/85

import { BOGOTA_ROUTE } from './bogota';

describe('BOGOTA_ROUTE // Covers [SPEC-005: AC-006] FR-007 BR-008', () => {
  describe('route data ~17-20 pts Aeropuerto El Dorado', () => {
    it('exports array 15-22 points (17 Aeropuerto)', () => {
      // Arrange
      const route = BOGOTA_ROUTE;

      // Act
      const len = route.length;

      // Assert
      expect(Array.isArray(route)).toBe(true);
      expect(len).toBeGreaterThanOrEqual(15);
      expect(len).toBeLessThanOrEqual(22);
    });

    it('first point is Bogotá/Aeropuerto 4.67,-74.10 ±0.05', () => {
      // Arrange
      const route = BOGOTA_ROUTE;

      // Act
      const p0 = route[0];

      // Assert
      expect(p0.lat).toBeCloseTo(4.67, 1);
      expect(p0.lon).toBeCloseTo(-74.10, 1);
    });

    it('all points lat in [-90,90] lon in [-180,180] speed >=0', () => {
      // Arrange
      const route = BOGOTA_ROUTE;

      // Act
      const invalid = route.filter((p: any) => p.lat < -90 || p.lat > 90 || p.lon < -180 || p.lon > 180 || p.speed < 0);

      // Assert
      expect(invalid).toEqual([]);
      expect(route.every((p: any) => typeof p.lat === 'number' && typeof p.lon === 'number' && typeof p.speed === 'number')).toBe(true);
    });

    it('speed variado incluye 0, ~40-50 y >80 para speeding_on/off', () => {
      // Arrange
      const route = BOGOTA_ROUTE;

      // Act
      const speeds = route.map((p: any) => p.speed);

      // Assert
      expect(speeds).toContain(0);
      expect(speeds.some((s: number) => s >= 35 && s <= 50)).toBe(true);
      expect(speeds.some((s: number) => s > 80)).toBe(true);
    });

    it('ruta Bogotá distinta a Medellín (centros diferentes)', () => {
      // Arrange
      const bog = BOGOTA_ROUTE[0];

      // Act
      const isMedellinCenter = Math.abs(bog.lat - 6.2442) < 0.01 && Math.abs(bog.lon - (-75.5812)) < 0.01;

      // Assert
      expect(isMedellinCenter).toBe(false);
      expect(bog.lat).toBeCloseTo(4.67, 1);
    });
  });
});
