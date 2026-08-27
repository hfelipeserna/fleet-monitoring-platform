// Covers [SPEC-005: AC-002] FR-002 FR-003 BR-003/004
// TEST-002: postTelemetry con AbortController 5s y EXPO_PUBLIC_API_URL fallback

const ORIGINAL_ENV = process.env.EXPO_PUBLIC_API_URL;
const ORIGINAL_FETCH = global.fetch;

jest.mock('expo-constants', () => ({
  default: { expoConfig: { extra: {} } },
  expoConfig: { extra: {} },
}));

// Import after mock; will be RED until lib/api.ts exists
import { postTelemetry, postBatch } from './api';

describe('lib/api', () => {
  beforeEach(() => {
    // Arrange baseline
    jest.useFakeTimers();
    global.fetch = jest.fn().mockResolvedValue({
      status: 202,
      ok: true,
      headers: { get: jest.fn().mockReturnValue(null) } as any,
      json: async () => ({ accepted: true }),
    } as any);
  });

  afterEach(() => {
    jest.useRealTimers();
    jest.clearAllMocks();
    global.fetch = ORIGINAL_FETCH as any;
    process.env.EXPO_PUBLIC_API_URL = ORIGINAL_ENV;
    jest.resetModules();
  });

  describe('postTelemetry', () => {
    it('posts to POST /v1/telemetry with plate, lat, lon, speed, client_event_id uuid, occurred_at', async () => {
      // Arrange
      const event = {
        plate: 'TGY589',
        lat: 6.2442,
        lon: -75.5812,
        speed: 42,
        client_event_id: '550e8400-e29b-41d4-a716-446655440000',
        occurred_at: new Date().toISOString(),
      } as any;

      // Act
      await postTelemetry(event);

      // Assert
      expect(global.fetch).toHaveBeenCalledTimes(1);
      const [url, opts] = (global.fetch as jest.Mock).mock.calls[0];
      expect(url).toMatch(/\/v1\/telemetry$/);
      expect(opts.method).toBe('POST');
      expect(opts.headers).toMatchObject({ 'Content-Type': 'application/json' });
      const body = JSON.parse(opts.body);
      expect(body.plate).toBe('TGY589');
      expect(body.lat).toBe(6.2442);
      expect(body.lon).toBe(-75.5812);
      expect(body.speed).toBe(42);
      expect(body.client_event_id).toBe('550e8400-e29b-41d4-a716-446655440000');
      expect(body.occurred_at).toBeDefined();
      expect(opts.signal).toBeInstanceOf(AbortSignal);
    });

    it('uses AbortController with 5s timeout and aborts on timeout', async () => {
      // Arrange
      let capturedSignal: AbortSignal | undefined;
      (global.fetch as jest.Mock).mockImplementation((_url: string, opts: any) => {
        capturedSignal = opts.signal;
        return new Promise((_resolve, reject) => {
          capturedSignal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')));
        });
      });
      const event = {
        plate: 'TGY589',
        lat: 6.2442,
        lon: -75.5812,
        speed: 10,
        client_event_id: '550e8400-e29b-41d4-a716-446655440001',
        occurred_at: new Date().toISOString(),
      } as any;
      const promise = postTelemetry(event);

      // Act
      jest.advanceTimersByTime(5000);

      // Assert
      await expect(promise).rejects.toThrow();
      expect(capturedSignal?.aborted).toBe(true);
    });

    it('does not abort before 5s', async () => {
      // Arrange
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 202,
        ok: true,
        headers: { get: jest.fn().mockReturnValue(null) } as any,
        json: async () => ({ accepted: true }),
      } as any);
      const event = {
        plate: 'ACF356',
        lat: 4.711,
        lon: -74.0721,
        speed: 0,
        client_event_id: '550e8400-e29b-41d4-a716-446655440002',
        occurred_at: new Date().toISOString(),
      } as any;
      const promise = postTelemetry(event);

      // Act
      jest.advanceTimersByTime(4000);

      // Assert
      await expect(promise).resolves.toBeDefined();
    });

    it('fallbacks to http://localhost:8080 when EXPO_PUBLIC_API_URL not set and expo extra empty', async () => {
      // Arrange
      delete process.env.EXPO_PUBLIC_API_URL;
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 202,
        headers: { get: jest.fn().mockReturnValue(null) } as any,
        json: async () => ({ accepted: true }),
      } as any);
      const event = {
        plate: 'ACF356',
        lat: 6.2,
        lon: -75.5,
        speed: 5,
        client_event_id: '550e8400-e29b-41d4-a716-446655440003',
        occurred_at: new Date().toISOString(),
      } as any;

      // Act
      await postTelemetry(event);

      // Assert
      const [url] = (global.fetch as jest.Mock).mock.calls[0];
      expect(url).toBe('http://localhost:8080/v1/telemetry');
    });

    it('uses EXPO_PUBLIC_API_URL when set', async () => {
      // Arrange
      process.env.EXPO_PUBLIC_API_URL = 'http://192.168.1.10:8080';
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 202,
        headers: { get: jest.fn().mockReturnValue(null) } as any,
        json: async () => ({ accepted: true }),
      } as any);
      const event = {
        plate: 'TGY589',
        lat: 6.2,
        lon: -75.5,
        speed: 15,
        client_event_id: '550e8400-e29b-41d4-a716-446655440004',
        occurred_at: new Date().toISOString(),
      } as any;

      // Act
      await postTelemetry(event);

      // Assert
      const [url] = (global.fetch as jest.Mock).mock.calls[0];
      expect(url).toBe('http://192.168.1.10:8080/v1/telemetry');
    });

    it('propagates 400 response without throwing on fetch layer (caller decides error)', async () => {
      // Arrange
      (global.fetch as jest.Mock).mockResolvedValue({
        status: 400,
        ok: false,
        headers: { get: jest.fn().mockReturnValue(null) } as any,
        json: async () => ({ error: 'invalid plate' }),
      } as any);
      const event = {
        plate: 'TGY589',
        lat: 6.2,
        lon: -75.5,
        speed: 10,
        client_event_id: '550e8400-e29b-41d4-a716-446655440005',
        occurred_at: new Date().toISOString(),
      } as any;

      // Act
      const res = await postTelemetry(event);

      // Assert
      expect(res.status).toBe(400);
    });
  });

  describe('postBatch', () => {
    it('posts to /v1/telemetry/batch with events 1..500 wrapper', async () => {
      // Arrange
      const events = [
        {
          plate: 'TGY589',
          lat: 6.2,
          lon: -75.5,
          speed: 42,
          client_event_id: '550e8400-e29b-41d4-a716-446655440010',
          occurred_at: new Date().toISOString(),
        },
      ];

      // Act
      // postBatch may not exist yet -> RED
      await (postBatch as any)(events);

      // Assert
      const [url, opts] = (global.fetch as jest.Mock).mock.calls[0];
      expect(url).toMatch(/\/v1\/telemetry\/batch$/);
      expect(opts.method).toBe('POST');
      const body = JSON.parse(opts.body);
      expect(body.events).toHaveLength(1);
      expect(body.events[0].plate).toBe('TGY589');
    });
  });
});
