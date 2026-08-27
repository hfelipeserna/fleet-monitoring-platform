import { TelemetryPort } from '../store/ports';
import { clearPending, enqueue, getPending, countPending, markSynced, incrementAttempts } from './telemetry';

export const telemetryPort: TelemetryPort = {
  clearPending: () => clearPending(),
  enqueue: (point: unknown) => enqueue(point as Parameters<typeof enqueue>[0]) as unknown as Promise<void>,
  getPending: (limit: number) => getPending(limit) as unknown as Promise<unknown[]>,
  countPending: () => countPending(),
  markSynced: (ids: string[]) => markSynced(ids),
  incrementAttempts: (ids: string[], lastError: string) => incrementAttempts(ids, lastError),
};
