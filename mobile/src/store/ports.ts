export interface TelemetryPort {
  clearPending(): Promise<void>;
  enqueue(point: any): Promise<void>;
  getPending(limit: number): Promise<any[]>;
  countPending(): Promise<number>;
  markSynced?(ids: string[]): Promise<void>;
  incrementAttempts?(ids: string[], lastError: string): Promise<void>;
}

export interface IntervalPort {
  register(id: number): number;
  clear(id: number): void;
  clearAll(): void;
}

let _telemetryPort: TelemetryPort | null = null;
let _intervalPort: IntervalPort | null = null;

export function injectTelemetryPort(port: TelemetryPort): void {
  _telemetryPort = port;
}

export function injectIntervalPort(port: IntervalPort): void {
  _intervalPort = port;
}

export function getTelemetryPort(): TelemetryPort | null {
  return _telemetryPort;
}

export function getIntervalPort(): IntervalPort | null {
  return _intervalPort;
}

export function __resetPorts(): void {
  _telemetryPort = null;
  _intervalPort = null;
}
