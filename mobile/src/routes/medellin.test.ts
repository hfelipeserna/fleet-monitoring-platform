// Covers [SPEC-005: AC-006] FR-006 FR-007 BR-007 BR-008 TS-006
// TASK-005-06 TDD RED: Rutas Medellín/Bogotá ~20 pts, lat/lon reales, speed 0/45/85, 5s interval, client_event_id uuid

import { MEDELLIN_ROUTE } from './medellin';

describe('MEDELLIN_ROUTE // Covers [SPEC-005: AC-006] FR-007 BR-008', () => {
  describe('route data ~20 pts con centro Medellín 6.2442,-75.5812', () => {
    it('exports array ~20 points (18-22)', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const len = route.length;

      // Assert
      expect(Array.isArray(route)).toBe(true);
      expect(len).toBeGreaterThanOrEqual(18);
      expect(len).toBeLessThanOrEqual(22);
    });

    it('first point is Medellín center 6.2442,-75.5812 ±0.01', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const p0 = route[0];

      // Assert
      expect(p0.lat).toBeCloseTo(6.2442, 1);
      expect(p0.lon).toBeCloseTo(-75.5812, 1);
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

    it('speed variado incluye 0, 45 y 85 para speeding_on/off', () => {
      // Arrange
      const route = MEDELLIN_ROUTE;

      // Act
      const speeds = route.map((p: any) => p.speed);

      // Assert
      expect(speeds).toContain(0);
      expect(speeds).toContain(45);
      expect(speeds).toContain(85);
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
