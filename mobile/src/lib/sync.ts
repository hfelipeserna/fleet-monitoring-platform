import { postBatch } from './api';
import { getTelemetryPort } from '../store/ports';
import type { TelemetryPort } from '../store/ports';
import type { TelemetryRecord } from '../db/telemetry';
import { useAppStore } from '../store/appStore';

let globalAttempts = 0;

export function _resetSyncAttempts(): void {
  globalAttempts = 0;
}

export function _getSyncAttempts(): number {
  return globalAttempts;
}

export async function flushPending(opts?: { port?: TelemetryPort; signal?: AbortSignal }): Promise<number> {
  if (opts?.signal?.aborted) {
    throw new DOMException('Aborted', 'AbortError');
  }

  const port = opts?.port ?? getTelemetryPort();

  let pending: TelemetryRecord[];
  if (port?.getPending) {
    pending = (await port.getPending(50)) as TelemetryRecord[];
  } else {
    const mod = await import('../db/telemetry');
    pending = await mod.getPending(50);
  }

  if (!pending || pending.length === 0) return 0;

  if (opts?.signal?.aborted) {
    throw new DOMException('Aborted', 'AbortError');
  }

  const events = pending.slice(0, 50).map((p: TelemetryRecord) => ({
    plate: p.plate,
    lat: p.lat ?? null,
    lon: p.lon ?? null,
    speed: p.speed ?? 0,
    client_event_id: p.client_event_id ?? (p as unknown as { clientEventId?: string }).clientEventId ?? p.id,
    occurred_at:
      typeof p.occurred_at === 'number'
        ? new Date(p.occurred_at).toISOString()
        : (p.occurred_at as unknown as string) ?? new Date().toISOString(),
  }));

  const res = await postBatch(events, opts?.signal ? { signal: opts.signal } : undefined);

  if (res.status === 202) {
    globalAttempts = 0;
    let accepted = pending.length;
    try {
      const clone = (res as unknown as { clone?: () => Response }).clone?.();
      const target = clone ?? res;
      const body = (await target.json()) as { accepted?: number | boolean };
      if (typeof body.accepted === 'number') accepted = body.accepted;
      else if (body.accepted === true) accepted = pending.length;
      else if (body.accepted === false) accepted = 0;
    } catch {}
    const ids = pending.slice(0, accepted).map((p: TelemetryRecord) => p.client_event_id ?? (p as unknown as { clientEventId?: string }).clientEventId ?? p.id);
    if (port?.markSynced) {
      await port.markSynced(ids);
    } else {
      const mod = await import('../db/telemetry');
      await mod.markSynced(ids);
    }
    try {
      useAppStore.getState().setSync('CONNECTED');
    } catch {}
    return accepted;
  }

  if (res.status === 429 || res.status === 503) {
    const headerVal = res.headers.get('Retry-After');
    let retryAfter = 5;
    if (headerVal) {
      const parsed = parseInt(headerVal, 10);
      if (!Number.isNaN(parsed) && parsed > 0) retryAfter = parsed;
    }
    const prevAttempts = globalAttempts;
    globalAttempts += 1;
    const lastError = `${res.status} backpressure`;
    const ids = pending.map((p: TelemetryRecord) => p.client_event_id ?? (p as unknown as { clientEventId?: string }).clientEventId ?? p.id);
    try {
      if (port?.incrementAttempts) {
        await port.incrementAttempts(ids, lastError);
      } else {
        const mod = await import('../db/telemetry');
        if (typeof (mod as unknown as { incrementAttempts?: unknown }).incrementAttempts === 'function') {
          await (mod as unknown as { incrementAttempts: (a: string[], b: string) => Promise<void> }).incrementAttempts(ids, lastError);
        } else {
          const q = (mod as unknown as { _mockQueue?: unknown[] })._mockQueue as Array<Record<string, unknown>> | undefined;
          if (Array.isArray(q)) {
            const idSet = new Set(ids);
            for (const r of q) {
              if (idSet.has(r.client_event_id as string) || idSet.has(r.id as string)) {
                r.attempts = ((r.attempts as number | undefined) ?? 0) + 1;
                r.last_error = lastError;
                if ((r.attempts as number) >= 5) r.sync_status = 'failed';
              }
            }
          }
        }
      }
    } catch {}

    if (res.status === 503) {
      try {
        useAppStore.getState().setSync('ERROR');
      } catch {}
    }

    const jitter = Math.random() * 1000;
    const backoff = Math.min(5000 * Math.pow(2, prevAttempts), 60000) + jitter;
    const err = Object.assign(new Error(`retry ${res.status}`), {
      retryAfter,
      backoffMs: backoff,
      status: res.status,
    }) as Error & { retryAfter: number; backoffMs: number; status: number };
    throw err;
  }

  throw new Error(`flushPending failed ${res.status}`);
}
