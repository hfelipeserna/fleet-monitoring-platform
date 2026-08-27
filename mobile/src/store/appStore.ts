import { create } from 'zustand';
import { isValidPlate } from '../lib/plate';
import { intervalRegistry } from './intervalRegistry';

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
  isDisconnecting: boolean;
  __abortController: AbortController | null;
  __telemetryInterval: ReturnType<typeof setInterval> | null;
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

async function resolveClearPending(): Promise<() => Promise<void>> {
  if (injectedClearPending) return injectedClearPending;
  const mod = await import('../db/telemetry');
  return mod.clearPending as () => Promise<void>;
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
  isDisconnecting: false,
  __abortController: null,
  __telemetryInterval: null,

  setPlate: (plate) => set({ plate }),
  setConn: (conn) =>
    set({
      conn,
      sync: syncForConn(conn),
      simEnabled: conn === 'connected',
      ...(conn !== 'connected' ? { simOn: false } : {}),
      ...(conn === 'idle' ? { selectedRoute: null } : {}),
    } as Partial<AppState>),
  setSync: (sync) => set({ sync }),
  setNet: (net) => set({ net }),
  setDb: (db) => set({ db }),
  setSimOn: (v) => {
    if (get().conn !== 'connected') return;
    if (!v) {
      const iid = get().__telemetryInterval;
      if (iid !== null && iid !== undefined) {
        intervalRegistry.clear(iid);
        set({ simOn: false, selectedRoute: null, __telemetryInterval: null });
      } else {
        intervalRegistry.clearAll();
        set({ simOn: false, selectedRoute: null });
      }
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
      const iid = get().__telemetryInterval;
      if (iid !== null && iid !== undefined) {
        intervalRegistry.clear(iid);
        set({ simOn: false, selectedRoute: null, __telemetryInterval: null });
      } else {
        intervalRegistry.clearAll();
        set({ simOn: false, selectedRoute: null });
      }
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
  setAbortController: (c: AbortController | null) => set({ __abortController: c }),
  setTelemetryInterval: (id) => {
    if (id !== null && id !== undefined) intervalRegistry.register(id);
    set({ __telemetryInterval: id });
  },

  connect: async (plate: string) => {
    const normalized = plate.trim().toUpperCase();
    if (!isValidPlate(normalized)) {
      set({ conn: 'error', sync: 'ERROR' });
      return;
    }
    set({ plate: normalized, conn: 'connecting', sync: 'CONNECTING' });
    const { db, net } = get();
    if (db !== 'OK' || net !== 'OK') {
      set({ conn: 'error', sync: 'ERROR' });
    }
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
      const ac: AbortController | null = get().__abortController;
      if (ac) {
        try {
          ac.abort();
        } catch {}
        set({ __abortController: null });
      }
      const intervalId = get().__telemetryInterval;
      if (intervalId !== null && intervalId !== undefined) {
        intervalRegistry.clear(intervalId);
        set({ __telemetryInterval: null });
      } else {
        intervalRegistry.clearAll();
      }
      const { clearPending } = await import('../db/telemetry');
      await clearPending();
      set({
        plate: '',
        conn: 'idle',
        sync: 'CONNECTING',
        simOn: false,
        simEnabled: false,
        selectedRoute: null,
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
      isDisconnecting: false,
      __abortController: null,
      __telemetryInterval: null,
    }),
}));
