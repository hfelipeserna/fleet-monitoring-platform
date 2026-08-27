// Covers [SPEC-005: AC-009] FR-008 BR-010 TS-009
// TEST-009: enqueue 245 -> kill -> relaunch count 245 uniques - persist WatermelonDB file survives kill
// RED until mobile-expo use file SQLite persisted not memory mockQueue

jest.mock('expo-constants', () => ({
  default: { expoConfig: { extra: { apiUrl: 'http://localhost:8080' } } },
  expoConfig: { extra: { apiUrl: 'http://localhost:8080' } },
}), { virtual: true });

jest.mock('@react-native-community/netinfo', () => ({
  addEventListener: jest.fn(() => jest.fn()),
  fetch: jest.fn().mockResolvedValue({ isConnected: true, isInternetReachable: true }),
}), { virtual: true });

jest.mock('@nozbe/watermelondb', () => ({
  appSchema: (x: any) => x,
  tableSchema: (x: any) => x,
  Model: class {},
  field: () => () => {},
  date: () => () => {},
}), { virtual: true });

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function makePoint(idx: number) {
  return {
    plate: 'TGY589',
    lat: 6.2442 + idx * 0.00001,
    lon: -75.5812 + idx * 0.00001,
    speed: [0, 45, 85][idx % 3],
    occurred_at: Date.now() + idx,
  };
}

describe('db/persist - WatermelonDB survives kill AC-009 FR-008 BR-010', () => {
  beforeEach(async () => {
    // Arrange clean
    jest.resetModules();
    const telemetryFresh = await import('./telemetry');
    await (telemetryFresh as any).clearPending();
  });

  it('enqueue 245 -> kill -> relaunch count 245 uniques', async () => {
    // Arrange
    const telemetry1 = await import('./telemetry');
    await (telemetry1 as any).clearPending();
    for (let i = 0; i < 245; i++) {
      await (telemetry1 as any).enqueue(makePoint(i));
    }
    const countBefore = await (telemetry1 as any).countPending();
    const pendingBefore = await (telemetry1 as any).getPending(500);
    const idsBefore = new Set(pendingBefore.map((r: any) => r.client_event_id));
    expect(countBefore).toBe(245);
    expect(idsBefore.size).toBe(245);

    // Act - simulate kill: reset modules + re-import (fresh process) should restore from SQLite file
    jest.resetModules();
    // need to re-mock watermelondb after reset
    jest.doMock('@nozbe/watermelondb', () => ({
      appSchema: (x: any) => x,
      tableSchema: (x: any) => x,
      Model: class {},
      field: () => () => {},
      date: () => () => {},
    }), { virtual: true });
    jest.doMock('expo-constants', () => ({
      default: { expoConfig: { extra: { apiUrl: 'http://localhost:8080' } } },
      expoConfig: { extra: { apiUrl: 'http://localhost:8080' } },
    }), { virtual: true });
    jest.doMock('@react-native-community/netinfo', () => ({
      addEventListener: jest.fn(() => jest.fn()),
      fetch: jest.fn().mockResolvedValue({ isConnected: true, isInternetReachable: true }),
    }), { virtual: true });

    const telemetry2 = await import('./telemetry');
    // simulate relaunch initDatabase path that reloads from file - not mockQueue reset
    const countAfter = await (telemetry2 as any).countPending();
    const pendingAfter = (telemetry2 as any).getPending ? await (telemetry2 as any).getPending(500) : [];

    // Assert
    expect(countAfter).toBe(245);
    const idsAfter = new Set(pendingAfter.map((r: any) => r.client_event_id));
    expect(idsAfter.size).toBe(245);
    idsAfter.forEach((id: string) => expect(id).toMatch(UUID_RE));
  });

  it('client_event_id uniques preserved after kill, no duplicates', async () => {
    // Arrange
    const tele1 = await import('./telemetry');
    await (tele1 as any).clearPending();
    for (let i = 0; i < 245; i++) {
      await (tele1 as any).enqueue(makePoint(i + 1000));
    }

    // Act
    jest.resetModules();
    jest.doMock('@nozbe/watermelondb', () => ({
      appSchema: (x: any) => x,
      tableSchema: (x: any) => x,
      Model: class {},
      field: () => () => {},
      date: () => () => {},
    }), { virtual: true });
    const tele2 = await import('./telemetry');
    const pending = await (tele2 as any).getPending(500);
    const all = (tele2 as any)._mockQueue ? (tele2 as any)._mockQueue : pending;

    // Assert
    const ids = (all as any[]).map((r: any) => r.client_event_id);
    const uniq = new Set(ids);
    expect(ids.length).toBe(245);
    expect(uniq.size).toBe(245);
  });

  it('SELECT count(*) =245 after relaunch', async () => {
    // Arrange
    const t1 = await import('./telemetry');
    await (t1 as any).clearPending();
    for (let i = 0; i < 245; i++) {
      await (t1 as any).enqueue({ plate: 'ACF356', lat: 4.711 + i * 0.00001, lon: -74.072 + i * 0.00001, speed: i % 85 });
    }
    expect(await (t1 as any).countPending()).toBe(245);

    // Act
    jest.resetModules();
    jest.doMock('@nozbe/watermelondb', () => ({
      appSchema: (x: any) => x,
      tableSchema: (x: any) => x,
      Model: class {},
      field: () => () => {},
      date: () => () => {},
    }), { virtual: true });
    const t2 = await import('./telemetry');
    const count = await (t2 as any).countPending();

    // Assert
    expect(count).toBe(245);
  });
});
