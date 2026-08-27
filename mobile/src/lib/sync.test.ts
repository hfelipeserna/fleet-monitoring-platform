// Covers [SPEC-005: AC-008, AC-010] FR-008 FR-009 BR-008 BR-009 BR-010
// TEST-008/010: Sync batch idempotente 60 offline + 429/503 backoff + attempts + jitter + client_event_id uuid
// RED until mobile-expo implemente lib/sync.ts completo con Retry-After+jitter+attempts y batch 50

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

import { flushPending, _resetSyncAttempts } from './sync';
import * as telemetry from '../db/telemetry';
import { useAppStore } from '../store/appStore';
import { __resetPorts } from '../store/ports';

// ---- helpers testdata ----
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function makePoint(idx: number, plate = 'TGY589') {
  return {
    plate,
    lat: 6.2442 + idx * 0.001,
    lon: -75.5812 + idx * 0.001,
    speed: [0, 45, 85][idx % 3],
    occurred_at: Date.now() + idx,
  };
}

async function enqueueMany(n: number, plate = 'TGY589'): Promise<void> {
  for (let i = 0; i < n; i++) {
    await (telemetry as any).enqueue(makePoint(i, plate));
  }
}

describe('lib/sync - Sync batch idempotente AC-008 AC-010', () => {
  const originalFetch = global.fetch;

  beforeEach(async () => {
    // Arrange baseline
    jest.useFakeTimers();
    _resetSyncAttempts();
    __resetPorts();
    useAppStore.getState().reset();
    jest.spyOn(Math, 'random').mockReturnValue(0.5);
    await (telemetry as any).clearPending();
    // default fetch 202
    global.fetch = jest.fn().mockResolvedValue({
      status: 202,
      headers: { get: jest.fn().mockReturnValue(null) },
      json: async () => ({ accepted: 50 }),
      clone: function () { return this; },
    } as any);
  });

  afterEach(async () => {
    jest.useRealTimers();
    jest.restoreAllMocks();
    global.fetch = originalFetch as any;
    await (telemetry as any).clearPending();
    _resetSyncAttempts();
  });

  describe('60 offline -> recover Net OK -> POST /batch 50 202 50 purged + Syncing CONNECTED AC-008', () => {
    it('flush batches 50 when 60 pending, 202 purges 50 leaving 10 and sets Syncing CONNECTED', async () => {
      // Arrange
      await enqueueMany(60, 'TGY589');
      expect(await (telemetry as any).countPending()).toBe(60);
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTING', net: 'OK', db: 'OK', plate: 'TGY589' } as any);
      let capturedBody: any = null;
      (global.fetch as jest.Mock).mockImplementation(async (_url: string, opts: any) => {
        capturedBody = JSON.parse(opts.body);
        return {
          status: 202,
          headers: { get: () => null },
          json: async () => ({ accepted: 50 }),
          clone: function () { return this; },
        } as any;
      });

      // Act
      const accepted = await flushPending();

      // Assert
      expect(capturedBody).not.toBeNull();
      expect(capturedBody.events).toHaveLength(50);
      expect(accepted).toBe(50);
      expect(await (telemetry as any).countPending()).toBe(10);
      expect(useAppStore.getState().sync).toBe('CONNECTED');
      expect(useAppStore.getState().net).toBe('OK');
    });

    it('second flush drains remaining 10 after first 50', async () => {
      // Arrange
      await enqueueMany(60, 'TGY589');
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTING', net: 'OK', db: 'OK' } as any);
      (global.fetch as jest.Mock)
        .mockResolvedValueOnce({
          status: 202,
          headers: { get: () => null },
          json: async () => ({ accepted: 50 }),
          clone: function () { return this; },
        } as any)
        .mockResolvedValueOnce({
          status: 202,
          headers: { get: () => null },
          json: async () => ({ accepted: 10 }),
          clone: function () { return this; },
        } as any);

      // Act
      const first = await flushPending();
      const second = await flushPending();

      // Assert
      expect(first).toBe(50);
      expect(second).toBe(10);
      expect(await (telemetry as any).countPending()).toBe(0);
    });
  });

  describe('429 Retry-After:5 keeps 60 + backoff 5s*2^n AC-008', () => {
    it('429 keeps 60 pending, attempts++ and throws with retryAfter 5 and backoff 5s*2^attempts + jitter', async () => {
      // Arrange
      await enqueueMany(60, 'TGY589');
      jest.spyOn(Math, 'random').mockReturnValue(0.2);
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 429,
        headers: { get: (k: string) => (k === 'Retry-After' ? '5' : null) },
        json: async () => ({}),
      } as any);

      // Act
      let err: any = null;
      try {
        await flushPending();
      } catch (e) {
        err = e;
      }

      // Assert
      expect(err).not.toBeNull();
      expect(err.retryAfter).toBe(5);
      // spec: first retry backoff = 5000 * 2^0 + jitter(200) = 5200 ; implementation currently uses 2^1 => 10200 -> RED
      expect(err.backoffMs).toBe(5000 + 200);
      expect(await (telemetry as any).countPending()).toBe(60);
      const pending = await (telemetry as any).getPending(500);
      expect(pending[0].attempts).toBe(1);
      expect(pending[0].last_error).toMatch(/429/);
    });

    it('second 429 doubles backoff to 10s + jitter capped 60s', async () => {
      // Arrange
      await enqueueMany(10, 'TGY589');
      jest.spyOn(Math, 'random').mockReturnValue(0.1);
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 429,
        headers: { get: () => '5' },
        json: async () => ({}),
      } as any);

      // Act
      let err1: any, err2: any;
      try { await flushPending(); } catch (e) { err1 = e; }
      try { await flushPending(); } catch (e) { err2 = e; }

      // Assert
      expect(err1.backoffMs).toBe(5000 + 100);
      // second: 5000*2^1 +100 =10100 ; current gives 20000+100 -> RED
      expect(err2.backoffMs).toBe(10000 + 100);
      expect(err2.backoffMs).toBeLessThanOrEqual(60000 + 1000);
    });
  });

  describe('503 Network OK + Syncing ERROR + backoff 60s + attempts last_error AC-010', () => {
    it('503 keeps Network OK but sets Syncing ERROR, attempts++ last_error 503 backpressure, backoff capped 60s', async () => {
      // Arrange
      await enqueueMany(20, 'TGY589');
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK' } as any);
      jest.spyOn(Math, 'random').mockReturnValue(0.0);
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 503,
        headers: { get: (k: string) => (k === 'Retry-After' ? '5' : null) },
        json: async () => ({}),
      } as any);

      // Act
      let err: any = null;
      try {
        await flushPending();
        // flushPending must internally set Syncing ERROR on 503 while net stays OK (BR-004 desacoplado)
        // if not, next assertion fails RED
      } catch (e) {
        err = e;
      }
      // simulate what production must guarantee: sync layer sets STORE sync to ERROR without manual test hack
      // Current implementation leaves sync unchanged -> RED
      // No manual setState here

      // Assert
      expect(err.status).toBe(503);
      expect(useAppStore.getState().net).toBe('OK');
      expect(useAppStore.getState().sync).toBe('ERROR');
      expect(err.backoffMs).toBeLessThanOrEqual(60000 + 1000);
      const pending = await (telemetry as any).getPending(500);
      expect(pending.length).toBe(20);
      expect(pending[0].attempts).toBe(1);
      expect(pending[0].last_error).toBe('503 backpressure');
    });

    it('backoff caps at 60s after many failures', async () => {
      // Arrange
      await enqueueMany(5, 'TGY589');
      jest.spyOn(Math, 'random').mockReturnValue(0.0);
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 503,
        headers: { get: () => '5' },
        json: async () => ({}),
      } as any);

      // Act
      let lastErr: any = null;
      for (let i = 0; i < 6; i++) {
        try { await flushPending(); } catch (e) { lastErr = e; }
      }

      // Assert
      expect(lastErr.backoffMs).toBe(60000);
      expect(lastErr.backoffMs).not.toBeGreaterThan(61000);
    });
  });

  describe('attempts>=5 -> failed no reintenta AC-008 BR-009', () => {
    it('after 5 failures records become failed and flushPending does not retry them', async () => {
      // Arrange
      await enqueueMany(20, 'TGY589');
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 429,
        headers: { get: () => '5' },
        json: async () => ({}),
      } as any);
      jest.spyOn(Math, 'random').mockReturnValue(0.0);

      // Act
      for (let i = 0; i < 5; i++) {
        try { await flushPending(); } catch {}
      }
      const pendingAfter = await (telemetry as any).getPending(500);
      const allQueue = (telemetry as any)._mockQueue as any[];
      const failedCount = allQueue.filter((r: any) => r.sync_status === 'failed').length;

      // Assert
      expect(pendingAfter.length).toBe(0);
      expect(failedCount).toBe(20);
      expect(allQueue[0].attempts).toBeGreaterThanOrEqual(5);
      // next flush should not call fetch
      (global.fetch as jest.Mock).mockClear();
      const n = await flushPending();
      expect(n).toBe(0);
      expect(global.fetch).not.toHaveBeenCalled();
    });

    it('failed records are not purged but kept with last_error', async () => {
      // Arrange
      await enqueueMany(3, 'TGY589');
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 503,
        headers: { get: () => '5' },
        json: async () => ({}),
      } as any);

      // Act
      for (let i = 0; i < 5; i++) {
        try { await flushPending(); } catch {}
      }

      // Assert
      const all = (telemetry as any)._mockQueue as any[];
      expect(all.length).toBe(3);
      expect(all.every((r: any) => r.sync_status === 'failed')).toBe(true);
      expect(all.every((r: any) => r.last_error === '503 backpressure')).toBe(true);
    });
  });

  describe('jitter 0-1s AC-008 BR-009', () => {
    it('jitter adds 0-1s random to backoff', async () => {
      // Arrange
      await enqueueMany(5, 'TGY589');
      const spy = jest.spyOn(Math, 'random').mockReturnValue(0.75);
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 429,
        headers: { get: () => '5' },
        json: async () => ({}),
      } as any);

      // Act
      let err: any = null;
      try { await flushPending(); } catch (e) { err = e; }

      // Assert
      expect(spy).toHaveBeenCalled();
      expect(err.backoffMs).toBe(5000 + 750);
      expect(err.backoffMs % 1).not.toBeNaN();
      const jitter = err.backoffMs - 5000;
      expect(jitter).toBeGreaterThanOrEqual(0);
      expect(jitter).toBeLessThan(1000);
    });

    it('different jitter values produce different backoffs', async () => {
      // Arrange
      await (telemetry as any).clearPending();
      await enqueueMany(5, 'TGY589');
      _resetSyncAttempts();
      jest.spyOn(Math, 'random').mockReturnValue(0.1);
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 429,
        headers: { get: () => '5' },
        json: async () => ({}),
      } as any);
      let err1: any;
      try { await flushPending(); } catch (e) { err1 = e; }

      await (telemetry as any).clearPending();
      await enqueueMany(5, 'TGY589');
      _resetSyncAttempts();
      jest.spyOn(Math, 'random').mockReturnValue(0.9);
      let err2: any;
      try { await flushPending(); } catch (e) { err2 = e; }

      // Assert
      expect(err1.backoffMs).not.toBe(err2.backoffMs);
      expect(err2.backoffMs - err1.backoffMs).toBeCloseTo(800, 0);
    });
  });

  describe('client_event_id uuid sagrado únicos AC-008 BR-008', () => {
    it('each enqueued point has unique uuid v4 client_event_id', async () => {
      // Arrange
      await enqueueMany(60, 'TGY589');

      // Act
      const pending = await (telemetry as any).getPending(500);

      // Assert
      expect(pending).toHaveLength(60);
      const ids = pending.map((r: any) => r.client_event_id);
      const uniq = new Set(ids);
      expect(uniq.size).toBe(60);
      ids.forEach((id: string) => expect(id).toMatch(UUID_RE));
    });

    it('POST /batch payload contains client_event_id uuid uniques for dedup Nats-Msg-Id', async () => {
      // Arrange
      await enqueueMany(50, 'TGY589');
      let payload: any = null;
      (global.fetch as jest.Mock).mockImplementation(async (_url: string, opts: any) => {
        payload = JSON.parse(opts.body);
        return {
          status: 202,
          headers: { get: () => null },
          json: async () => ({ accepted: payload.events.length }),
          clone: function () { return this; },
        } as any;
      });

      // Act
      await flushPending();

      // Assert
      expect(payload).not.toBeNull();
      expect(payload.events).toHaveLength(50);
      const ids = payload.events.map((e: any) => e.client_event_id);
      const uniq = new Set(ids);
      expect(uniq.size).toBe(50);
      ids.forEach((id: string) => expect(id).toMatch(UUID_RE));
    });
  });
});
