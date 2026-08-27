// Covers [SPEC-005: AC-011] FR-010 FR-011 BR-010 BR-011 TS-011 TEST-011
// Task TASK-005-09 RED: verifica EAS/Fastlane + GH Actions + .env antes de que mobile-expo+devops cree los artefactos.
// Debe fallar RED hasta que existan eas.json dev/preview/prod, fastlane/Fastfile+Appfile y mobile.yml con path-filter + EAS Build.

import * as fs from 'fs';
import * as path from 'path';

function readJson(file: string): unknown {
  const raw = fs.readFileSync(file, 'utf8');
  return JSON.parse(raw);
}

function resolveMobile(p: string): string {
  // from src/config -> mobile/
  return path.resolve(__dirname, '..', '..', p);
}

function resolveRoot(p: string): string {
  // from src/config -> repo root (mobile/src/config -> ../../..)
  return path.resolve(__dirname, '..', '..', '..', p);
}

describe('mobile CI/CD config // Covers [SPEC-005: AC-011] FR-010/011 BR-010', () => {
  describe('eas.json dev/preview/prod', () => {
    it('exists and has build profiles dev/preview/prod|production', () => {
      // Arrange
      const easPath = resolveMobile('eas.json');

      // Act
      const exists = fs.existsSync(easPath);

      // Assert
      expect(exists).toBe(true);
      // Covers AC-011: eas.json con 3 perfiles
      const json = readJson(easPath) as Record<string, unknown>;
      const raw = JSON.stringify(json);
      // debe contener build.profiles con dev, preview y prod/production
      expect(raw).toContain('dev');
      expect(raw).toContain('preview');
      const hasProd = raw.includes('"prod"') || raw.includes('"production"');
      expect(hasProd).toBe(true);
      const build = (json as { build?: Record<string, unknown> }).build;
      expect(build).toBeDefined();
      expect((build as Record<string, unknown>)?.dev).toBeDefined();
      expect((build as Record<string, unknown>)?.preview).toBeDefined();
      const prodProfile = (build as Record<string, unknown>)?.production ?? (build as Record<string, unknown>)?.prod;
      expect(prodProfile).toBeDefined();
    });

    it('has cli version and no secret hardcodeada (no GEMINI_API_KEY, no sk-)', () => {
      // Arrange
      const easPath = resolveMobile('eas.json');

      // Act
      const raw = fs.existsSync(easPath) ? fs.readFileSync(easPath, 'utf8') : '';

      // Assert
      expect(raw.length).toBeGreaterThan(0);
      expect(raw).not.toMatch(/GEMINI_API_KEY/);
      expect(raw).not.toMatch(/sk-/);
    });
  });

  describe('.env.example EXPO_PUBLIC_API_URL', () => {
    it('exists and contains EXPO_PUBLIC_API_URL with LB http://host:8080', () => {
      // Arrange
      const envPath = resolveMobile('.env.example');

      // Act
      const exists = fs.existsSync(envPath);
      const raw = exists ? fs.readFileSync(envPath, 'utf8') : '';

      // Assert
      expect(exists).toBe(true);
      expect(raw).toContain('EXPO_PUBLIC_API_URL');
      expect(raw).toMatch(/EXPO_PUBLIC_API_URL\s*=\s*http:\/\//);
      // diferencia localhost vs LAN IP documentada
      expect(raw).toMatch(/localhost:8080|192\.168\.\d+\.\d+:8080|\$.*8080/);
    });
  });

  describe('fastlane/Fastfile + Appfile', () => {
    it('fastlane/Fastfile exists at fastlane/Fastfile or mobile/fastlane/Fastfile', () => {
      // Arrange
      const atRoot = resolveRoot('fastlane/Fastfile');
      const atRootLower = resolveRoot('Fastfile');
      const atMobile = resolveMobile('fastlane/Fastfile');

      // Act
      const exists = fs.existsSync(atRoot) || fs.existsSync(atRootLower) || fs.existsSync(atMobile);

      // Assert
      expect(exists).toBe(true);
      const file = fs.existsSync(atRoot) ? atRoot : fs.existsSync(atMobile) ? atMobile : atRootLower;
      const raw = fs.readFileSync(file, 'utf8');
      expect(raw.length).toBeGreaterThan(0);
      expect(raw).toMatch(/lane|fastlane_version/);
      expect(raw).not.toMatch(/GEMINI_API_KEY|sk-/);
    });

    it('fastlane/Appfile exists', () => {
      // Arrange
      const atRoot = resolveRoot('fastlane/Appfile');
      const atMobile = resolveMobile('fastlane/Appfile');

      // Act
      const exists = fs.existsSync(atRoot) || fs.existsSync(atMobile);

      // Assert
      expect(exists).toBe(true);
      const file = fs.existsSync(atRoot) ? atRoot : atMobile;
      const raw = fs.readFileSync(file, 'utf8');
      expect(raw.length).toBeGreaterThan(0);
    });
  });

  describe('.github/workflows/mobile.yml path-filter + EAS Build', () => {
    it('exists with path-filter mobile/** and EAS Build', () => {
      // Arrange
      const wf = resolveRoot('.github/workflows/mobile.yml');

      // Act
      const exists = fs.existsSync(wf);

      // Assert
      expect(exists).toBe(true);
      const raw = fs.readFileSync(wf, 'utf8');
      expect(raw).toMatch(/mobile\/\*\*/);
      // path-filter via paths: o dorny/paths-filter o on.push.paths
      const hasPathFilter = raw.includes('paths') && raw.includes('mobile');
      expect(hasPathFilter).toBe(true);
      expect(raw).toMatch(/eas build|EAS Build|eas\/build-action|expo.*eas/i);
    });

    it('does not contain secrets hardcodeados', () => {
      // Arrange
      const wf = resolveRoot('.github/workflows/mobile.yml');

      // Act
      const raw = fs.existsSync(wf) ? fs.readFileSync(wf, 'utf8') : '';

      // Assert
      expect(raw).not.toMatch(/GEMINI_API_KEY\s*:\s*["'][A-Za-z0-9_-]{20,}/);
      expect(raw.length).toBeGreaterThan(0);
    });
  });
});
