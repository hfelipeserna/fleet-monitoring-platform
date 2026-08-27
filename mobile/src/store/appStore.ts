import { create } from 'zustand';
import {
  TelemetryPort,
  IntervalPort,
  getTelemetryPort,
  getIntervalPort,
  injectTelemetryPort as injectPortTelemetry,
  injectIntervalPort as injectPortInterval,
} from './ports';

export type ConnState = 'idle' | 'connecting' | 'connected' | 'error';
export type SyncState = 'CONNECTING' | 'CONNECTED' | 'ERROR';
export type NetState = 'OK' | 'ERROR';
export type DbState = 'OK' | 'ERROR';

type RouteKind = 'medellin' | 'bogota' | null;

interface AppState {
  plate: string;
  conn: ConnState;
  sync: SyncState;
  net: NetState;
  db: DbState;
  simOn: boolean;
  simEnabled: boolean;
  selectedRoute: RouteKind;
  selectedRouteIdx: number;
  isDisconnecting: boolean;
  setPlate: (plate: string) => void;
  setConn: (conn: ConnState) => void;
  setSync: (sync: SyncState) => void;
  setNet: (net: NetState) => void;
  setDb: (db: DbState) => void;
  setSimOn: (v: boolean) => void;
  toggleSimOn: (v: boolean) => Promise<void>;
  setAbortController: (c: AbortController | null) => void;
  setTelemetryInterval: (id: ReturnType<typeof setInterval> | null) => void;
  connect: (plate: string) => Promise<void>;
  disconnect: () => Promise<void>;
  reset: () => void;
  injectClearPending: (fn: () => Promise<void>) => void;
}

function syncForConn(conn: ConnState): SyncState {
  if (conn === 'connected') return 'CONNECTED';
  if (conn === 'error') return 'ERROR';
  return 'CONNECTING';
}

let injectedClearPending: (() => Promise<void>) | null = null;

export function injectTelemetryPort(port: TelemetryPort): void {
  injectPortTelemetry(port);
}

export function injectIntervalPort(port: IntervalPort): void {
  injectPortInterval(port);
}

async function resolveClearPending(): Promise<() => Promise<void>> {
  const port = getTelemetryPort();
  if (port) return () => port.clearPending();
  if (injectedClearPending) return injectedClearPending;
  const mod = await import('../db/telemetry');
  return mod.clearPending as () => Promise<void>;
}

function intervalClearAll(): void {
  const port = getIntervalPort();
  if (port) {
    port.clearAll();
    return;
  }
  clearInterval(0 as unknown as number);
}

export const useAppStore = create<AppState>((set, get) => ({
  plate: '',
  conn: 'idle',
  sync: 'CONNECTING',
  net: 'OK',
  db: 'OK',
  simOn: false,
  simEnabled: false,
  selectedRoute: null,
  selectedRouteIdx: 0,
  isDisconnecting: false,

  setPlate: (plate) => set({ plate }),
  setConn: (conn) =>
    set({
      conn,
      sync: syncForConn(conn),
      simEnabled: conn === 'connected',
      ...(conn !== 'connected' ? { simOn: false } : {}),
      ...(conn === 'idle' ? { selectedRoute: null, selectedRouteIdx: 0 } : {}),
    } as Partial<AppState>),
  setSync: (sync) => set({ sync }),
  setNet: (net) => set({ net }),
  setDb: (db) => set({ db }),
  setSimOn: (v) => {
    if (get().conn !== 'connected') return;
    if (!v) {
      intervalClearAll();
      set({ simOn: false, selectedRoute: null, selectedRouteIdx: 0 });
      resolveClearPending()
        .then((fn) => fn().catch(() => {}))
        .catch(() => {});
      return;
    }
    set({ simOn: true });
  },
  toggleSimOn: async (v: boolean) => {
    if (get().conn !== 'connected') return;
    if (!v) {
      intervalClearAll();
      set({ simOn: false, selectedRoute: null, selectedRouteIdx: 0 });
      try {
        const fn = await resolveClearPending();
        await fn();
      } catch {}
      return;
    }
    set({ simOn: true });
  },
  injectClearPending: (fn: () => Promise<void>) => {
    injectedClearPending = fn;
  },
  setAbortController: () => {},
  setTelemetryInterval: () => {},

  connect: async (plate: string) => {
    const normalized = plate.trim().toUpperCase();
    set({ plate: normalized, conn: 'connecting', sync: 'CONNECTING' });
  },

  disconnect: async () => {
    if (get().isDisconnecting) return;
    set({ isDisconnecting: true });
    const prevPlate = get().plate;
    const prevConn = get().conn;
    const prevSync = get().sync;
    const prevSimOn = get().simOn;
    const prevSimEnabled = get().simEnabled;
    const prevRoute = get().selectedRoute;
    try {
      intervalClearAll();
      const fn = await resolveClearPending();
      await fn();
      set({
        plate: '',
        conn: 'idle',
        sync: 'CONNECTING',
        simOn: false,
        simEnabled: false,
        selectedRoute: null,
        selectedRouteIdx: 0,
      });
    } catch (err) {
      set({
        db: 'ERROR',
        plate: prevPlate,
        conn: prevConn,
        sync: prevSync,
        simOn: prevSimOn,
        simEnabled: prevSimEnabled,
        selectedRoute: prevRoute,
      });
      throw err;
    } finally {
      set({ isDisconnecting: false });
    }
  },

  reset: () =>
    set({
      plate: '',
      conn: 'idle',
      sync: 'CONNECTING',
      net: 'OK',
      db: 'OK',
      simOn: false,
      simEnabled: false,
      selectedRoute: null,
      selectedRouteIdx: 0,
      isDisconnecting: false,
    }),
}));
