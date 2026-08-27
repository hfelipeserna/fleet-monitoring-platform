// Covers [SPEC-005: AC-006] FR-006 FR-007 BR-007 BR-008 TS-006
// TASK-005-06 TDD RED: Rutas Medellín/Bogotá ~20 pts, lat/lon reales, speed 0/45/85, 5s interval, client_event_id uuid

import { MEDELLIN_ROUTE } from './medellin';

describe('MEDELLIN_ROUTE // Covers [SPEC-005: AC-006] FR-007 BR-008', () => {
  describe('route data ~20-40 pts Aeropuerto José María Córdova', () => {
    it('exports array 15-45 points (20 nominal, 40 Aeropuerto)', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const len = route.length;

      // Assert
      expect(Array.isArray(route)).toBe(true);
      expect(len).toBeGreaterThanOrEqual(15);
      expect(len).toBeLessThanOrEqual(50);
    });

    it('first point is Medellín/Aeropuerto 6.23,-75.56 ±0.05', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const p0 = route[0];

      // Assert
      expect(p0.lat).toBeCloseTo(6.23, 1);
      expect(p0.lon).toBeCloseTo(-75.56, 1);
    });

    it('all points lat in [-90,90] lon in [-180,180] speed >=0', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const invalid = route.filter((p: any) => p.lat < -90 || p.lat > 90 || p.lon < -180 || p.lon > 180 || p.speed < 0);

      // Assert
      expect(invalid).toEqual([]);
      expect(route.every((p: any) => typeof p.lat === 'number' && typeof p.lon === 'number' && typeof p.speed === 'number')).toBe(true);
    });

    it('speed variado incluye 0, ~45 y >80 para speeding_on/off', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const speeds = route.map((p: any) => p.speed);

      // Assert
      expect(speeds).toContain(0);
      expect(speeds.some((s: number) => s >= 40 && s <= 50)).toBe(true);
      expect(speeds.some((s: number) => s > 80)).toBe(true);
    });

    it('speed values son int >=0 (nullable lat/lon not allowed para ruta simulada)', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const nonInt = route.filter((p: any) => !Number.isInteger(p.speed));

      // Assert
      expect(nonInt).toEqual([]);
    });
  });
});
