import 'react-native-get-random-values';
import { v4 as uuidv4 } from 'uuid';
import { database } from './index';

export type SyncStatus = 'pending' | 'syncing' | 'synced' | 'failed';

export type TelemetryRecord = {
  id: string;
  client_event_id: string;
  plate: string;
  lat: number | null;
  lon: number | null;
  speed: number;
  occurred_at: number;
  sync_status: SyncStatus;
  attempts: number;
  last_error: string | null;
  synced_at: number | null;
};

type EnqueueInput = {
  plate: string;
  lat?: number | null;
  lon?: number | null;
  speed?: number;
  client_event_id?: string;
  occurred_at?: number | string;
  sync_status?: SyncStatus;
  attempts?: number;
  last_error?: string | null;
};

const mockQueue: TelemetryRecord[] = [];

function isMock(): boolean {
  try {
    const db = database as unknown as Record<string, unknown> | null;
    if (!db) return true;
    if ((db as Record<string, unknown>)._mock) return true;
    const isJest =
      typeof process !== 'undefined' &&
      !!((process as unknown as Record<string, unknown>).env as Record<string, unknown> | undefined)?.JEST_WORKER_ID;
    if (isJest) return true;
    if (typeof process !== 'undefined' && (process.env.NODE_ENV === 'test' || (process.env as unknown as Record<string, unknown>).JEST_WORKER_ID)) {
      return true;
    }
  } catch {}
  const isNodeNoWindow = typeof window === 'undefined';
  if (isNodeNoWindow) {
    const isJest2 =
      typeof process !== 'undefined' &&
      !!((process.env.JEST_WORKER_ID || process.env.NODE_ENV === 'test') as unknown as boolean);
    if (isJest2) return true;
  }
  return false;
}

function toTimestamp(v: number | string | undefined): number {
  if (typeof v === 'number') return v;
  if (typeof v === 'string') {
    const n = Date.parse(v);
    if (!Number.isNaN(n)) return n;
    const num = Number(v);
    if (!Number.isNaN(num)) return num;
  }
  return Date.now();
}

export async function enqueue(point: EnqueueInput): Promise<TelemetryRecord> {
  const now = Date.now();
  const record: TelemetryRecord = {
    id: uuidv4(),
    client_event_id: point.client_event_id ?? uuidv4(),
    plate: point.plate,
    lat: point.lat ?? null,
    lon: point.lon ?? null,
    speed: point.speed ?? 0,
    occurred_at: point.occurred_at !== undefined ? toTimestamp(point.occurred_at as number | string) : now,
    sync_status: (point.sync_status as SyncStatus) ?? 'pending',
    attempts: point.attempts ?? 0,
    last_error: point.last_error ?? null,
    synced_at: null,
  };

  if (isMock()) {
    mockQueue.push(record);
    return record;
  }

  const db = database as unknown as {
    write?: (fn: () => Promise<void>) => Promise<void>;
    collections?: { get: (name: string) => { create: (fn: (r: unknown) => void) => Promise<void> } };
  } | null;
  if (db && db.collections && db.write) {
    const col = db.collections.get('pending_telemetry');
    await db.write(async () => {
      await col.create((r: unknown) => {
        const rec = r as Record<string, unknown>;
        rec.client_event_id = record.client_event_id;
        rec.clientEventId = record.client_event_id;
        rec.plate = record.plate;
        rec.lat = record.lat;
        rec.lon = record.lon;
        rec.speed = record.speed;
        rec.occurred_at = record.occurred_at;
        rec.occurredAt = record.occurred_at;
        rec.sync_status = record.sync_status;
        rec.syncStatus = record.sync_status;
        rec.attempts = record.attempts;
        rec.last_error = record.last_error;
        rec.lastError = record.last_error;
      });
    });
    return record;
  }
  const adapter = (database as unknown as Record<string, unknown>)?.adapter as Record<string, unknown> | undefined;
  const exec = adapter?.unsafeExecute as ((o: unknown) => Promise<void>) | undefined;
  if (exec) {
    await exec({
      sql: 'INSERT INTO pending_telemetry (id, client_event_id, plate, lat, lon, speed, occurred_at, sync_status, attempts, last_error) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
      args: [
        record.id,
        record.client_event_id,
        record.plate,
        record.lat,
        record.lon,
        record.speed,
        record.occurred_at,
        record.sync_status,
        record.attempts,
        record.last_error,
      ],
    });
    return record;
  }
  throw new Error('enqueue: no database adapter available');
}

export async function getPending(limit = 500): Promise<TelemetryRecord[]> {
  if (isMock()) {
    return mockQueue.filter((r) => r.sync_status === 'pending').slice(0, limit);
  }
  const db = database as unknown as {
    collections?: { get: (name: string) => { query: (...args: unknown[]) => { fetch: () => Promise<unknown[]> } } };
  } | null;
  if (db && db.collections) {
    const col = db.collections.get('pending_telemetry');
    const { Q } = await import('@nozbe/watermelondb');
    const rows = await (col.query as unknown as (a: unknown, b: unknown) => { fetch: () => Promise<TelemetryRecord[]> })(
      (Q as unknown as { where: (a: string, b: unknown) => unknown }).where('sync_status', 'pending'),
      (Q as unknown as { take: (n: number) => unknown }).take(limit),
    ).fetch();
    return rows.slice(0, limit);
  }
  const adapter = (database as unknown as Record<string, unknown>)?.adapter as Record<string, unknown> | undefined;
  const query = adapter?.unsafeQueryRaw as ((args: unknown) => Promise<unknown[]>) | undefined;
  if (query) {
    const rows = await query({ sql: 'SELECT * FROM pending_telemetry WHERE sync_status = ? LIMIT ?', args: ['pending', limit] });
    return rows as TelemetryRecord[];
  }
  throw new Error('getPending: no database adapter available');
}

export async function getPendingCount(): Promise<number> {
  return countPending();
}

export async function countPending(): Promise<number> {
  if (isMock()) {
    return mockQueue.filter((r) => r.sync_status === 'pending').length;
  }
  const db = database as unknown as {
    collections?: { get: (name: string) => { query: (...args: unknown[]) => { fetchCount: () => Promise<number>; fetch: () => Promise<unknown[]> } } };
  } | null;
  if (db && db.collections) {
    const col = db.collections.get('pending_telemetry');
    const { Q } = await import('@nozbe/watermelondb');
    const q = (col.query as unknown as (a: unknown) => { fetchCount: () => Promise<number> })(
      (Q as unknown as { where: (a: string, b: unknown) => unknown }).where('sync_status', 'pending'),
    );
    if (q && typeof q.fetchCount === 'function') {
      return await q.fetchCount();
    }
  }
  const adapter = (database as unknown as Record<string, unknown>)?.adapter as Record<string, unknown> | undefined;
  const query = adapter?.unsafeQueryRaw as ((args: unknown) => Promise<unknown[]>) | undefined;
  if (query) {
    const rows = await query({ sql: 'SELECT COUNT(*) as count FROM pending_telemetry WHERE sync_status = ?', args: ['pending'] });
    const first = rows[0] as Record<string, unknown> | undefined;
    if (first) {
      const v = first.count ?? first['COUNT(*)'] ?? first['count(*)'] ?? 0;
      return Number(v);
    }
    return 0;
  }
  throw new Error('countPending: no database adapter available');
}

async function purgePending(): Promise<void> {
  const db = database as unknown as {
    write?: (fn: () => Promise<void>) => Promise<void>;
    collections?: { get: (name: string) => unknown };
    adapter?: unknown;
  } | null;
  if (db && db.collections && db.write) {
    const col = db.collections.get('pending_telemetry') as unknown as {
      query: () => { fetch: () => Promise<Array<{ destroyPermanently: () => Promise<void> }>> };
    };
    await db.write(async () => {
      const all = await col.query().fetch();
      await Promise.all(all.map((r) => r.destroyPermanently()));
    });
    return;
  }
  const adapter = (database as unknown as Record<string, unknown>)?.adapter as Record<string, unknown> | undefined;
  const exec = adapter?.unsafeExecute as ((o: unknown) => Promise<void>) | undefined;
  if (exec) {
    await exec({ sql: 'DELETE FROM pending_telemetry', args: [] });
    return;
  }
  throw new Error('purgePending: no database adapter available');
}

export async function clearPending(): Promise<void> {
  if (isMock()) {
    mockQueue.length = 0;
    return;
  }
  await purgePending();
}

export async function markSynced(ids: string[]): Promise<void> {
  if (!ids || ids.length === 0) return;
  const idSet = new Set(ids);
  if (isMock()) {
    const filtered = mockQueue.filter((r) => !idSet.has(r.client_event_id) && !idSet.has(r.id));
    mockQueue.length = 0;
    mockQueue.push(...filtered);
    return;
  }
  const db = database as unknown as {
    write?: (fn: () => Promise<void>) => Promise<void>;
    collections?: { get: (name: string) => unknown };
  } | null;
  if (db && db.write && db.collections) {
    const col = db.collections.get('pending_telemetry') as unknown as {
      find: (id: string) => Promise<{ destroyPermanently: () => Promise<void> }>;
      query: (...args: unknown[]) => { fetch: () => Promise<Array<{ id: string; client_event_id?: string; clientEventId?: string; destroyPermanently: () => Promise<void> }>> };
    };
    await db.write(async () => {
      const all = await col.query().fetch();
      for (const r of all) {
        const cid = (r as unknown as Record<string, unknown>).client_event_id as string | undefined;
        const cid2 = (r as unknown as Record<string, unknown>).clientEventId as string | undefined;
        if (idSet.has(r.id) || (cid && idSet.has(cid)) || (cid2 && idSet.has(cid2))) {
          await r.destroyPermanently();
        }
      }
      for (const id of ids) {
        try {
          const rec = await col.find(id);
          await rec.destroyPermanently();
        } catch {}
      }
    });
    return;
  }
  const adapter = (database as unknown as Record<string, unknown>)?.adapter as Record<string, unknown> | undefined;
  const exec = adapter?.unsafeExecute as ((o: unknown) => Promise<void>) | undefined;
  if (exec && ids.length > 0) {
    const placeholders = ids.map(() => '?').join(',');
    await exec({ sql: `DELETE FROM pending_telemetry WHERE id IN (${placeholders}) OR client_event_id IN (${placeholders})`, args: [...ids, ...ids] });
    return;
  }
  throw new Error('markSynced: no database adapter available');
}

export const _mockQueue = mockQueue;
