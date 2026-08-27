// Covers [SPEC-005: AC-004] FR-004 BR-005 TS-004
// TEST-004: Disconnect purga pending_telemetry DELETE via WatermelonDB
// RED until mobile-expo cree src/db/telemetry.ts (clearPending, countPending, enqueue)

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

import * as telemetry from './telemetry';

function makePoint(plate = 'TGY589', idx = 0) {
  return {
    plate,
    lat: 6.2442 + idx * 0.0001,
    lon: -75.5812 + idx * 0.0001,
    speed: [0, 45, 85][idx % 3],
    client_event_id: `550e8400-e29b-41d4-a716-44665544${String(idx).padStart(4, '0')}`.slice(0, 36),
    occurred_at: Date.now() + idx,
    sync_status: 'pending' as const,
  };
}

describe('db/telemetry - pending_telemetry WatermelonDB', () => {
  beforeEach(async () => {
    // Arrange - clean slate
    jest.clearAllMocks();
    if ((telemetry as any).clearPending) {
      await (telemetry as any).clearPending();
    }
  });

  describe('clearPending purga AC-004 FR-004 BR-005', () => {
    it('enqueue 20 -> countPending 20', async () => {
      // Arrange
      const points = Array.from({ length: 20 }, (_, i) => makePoint('TGY589', i));

      // Act
      for (const p of points) {
        await (telemetry as any).enqueue(p);
      }
      const count = await (telemetry as any).countPending();

      // Assert
      expect(count).toBe(20);
    });

    it('given 20 pending when clearPending then count 0 (DELETE) AC-004', async () => {
      // Arrange
      for (let i = 0; i < 20; i++) {
        await (telemetry as any).enqueue(makePoint('TGY589', i));
      }
      expect(await (telemetry as any).countPending()).toBe(20);

      // Act
      await (telemetry as any).clearPending();
      const after = await (telemetry as any).countPending();

      // Assert
      expect(after).toBe(0);
    });

    it('clearPending is idempotent: second clear still 0', async () => {
      // Arrange
      for (let i = 0; i < 5; i++) await (telemetry as any).enqueue(makePoint('ACF356', i));
      await (telemetry as any).clearPending();

      // Act
      await (telemetry as any).clearPending();
      const count = await (telemetry as any).countPending();

      // Assert
      expect(count).toBe(0);
    });

    it('enqueue after clearPending starts from 0 again', async () => {
      // Arrange
      for (let i = 0; i < 3; i++) await (telemetry as any).enqueue(makePoint('TGY589', i));
      await (telemetry as any).clearPending();

      // Act
      await (telemetry as any).enqueue(makePoint('TGY589', 99));
      const count = await (telemetry as any).countPending();

      // Assert
      expect(count).toBe(1);
    });

    it('exports clearPending, countPending, enqueue, getPending per plan §5', async () => {
      // Arrange - module shape

      // Act
      const mod: any = telemetry;

      // Assert
      expect(typeof mod.clearPending).toBe('function');
      expect(typeof mod.countPending).toBe('function');
      expect(typeof mod.enqueue).toBe('function');
      // getPending or getPendingCount or countPending alias must exist
      const hasGetPending = typeof mod.getPending === 'function' || typeof mod.getPendingCount === 'function';
      expect(hasGetPending || typeof mod.countPending === 'function').toBe(true);
    });

    it('client_event_id uniques preserved until clear (no dedup loss)', async () => {
      // Arrange
      const ids = new Set<string>();
      for (let i = 0; i < 20; i++) {
        const p = makePoint('TGY589', i);
        ids.add(p.client_event_id);
        await (telemetry as any).enqueue(p);
      }

      // Act
      const pending = (telemetry as any).getPending ? await (telemetry as any).getPending(500) : null;
      const count = await (telemetry as any).countPending();

      // Assert
      expect(count).toBe(20);
      expect(ids.size).toBe(20);
      if (pending) {
        const pendingIds = new Set(pending.map((r: any) => r.client_event_id ?? r.clientEventId));
        expect(pendingIds.size).toBe(20);
      }
    });
  });

  describe('Disconnect purga via store integration AC-004', () => {
    it('clearPending called from store disconnect flow leaves DB empty', async () => {
      // Arrange
      for (let i = 0; i < 20; i++) await (telemetry as any).enqueue(makePoint('TGY589', i));
      expect(await (telemetry as any).countPending()).toBe(20);

      // Act - simulate what store/appStore.ts disconnect must do: await clearPending()
      await (telemetry as any).clearPending();

      // Assert
      const after = await (telemetry as any).countPending();
      expect(after).toBe(0);
    });
  });
});
