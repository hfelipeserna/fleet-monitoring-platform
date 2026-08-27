import 'react-native-get-random-values';
import { v4 as uuidv4 } from 'uuid';
import { create } from 'zustand';
import { isValidPlate } from '../lib/plate';
import { postTelemetry } from '../lib/api';

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
  setPlate: (plate: string) => void;
  setConn: (conn: ConnState) => void;
  setSync: (sync: SyncState) => void;
  setNet: (net: NetState) => void;
  setDb: (db: DbState) => void;
  setSimOn: (v: boolean) => void;
  connect: (plate: string) => Promise<void>;
  disconnect: () => void;
  reset: () => void;
}

function syncForConn(conn: ConnState): SyncState {
  if (conn === 'connected') return 'CONNECTED';
  if (conn === 'error') return 'ERROR';
  return 'CONNECTING';
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

  setPlate: (plate) => set({ plate }),
  setConn: (conn) =>
    set({
      conn,
      sync: syncForConn(conn),
      simEnabled: conn === 'connected',
      ...(conn === 'idle' ? { simOn: false, selectedRoute: null } : {}),
    } as Partial<AppState>),
  setSync: (sync) => set({ sync }),
  setNet: (net) => set({ net }),
  setDb: (db) => set({ db }),
  setSimOn: (simOn) => set({ simOn }),

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
      return;
    }
    try {
      const event = {
        plate: normalized,
        lat: 0,
        lon: 0,
        speed: 0,
        client_event_id: uuidv4(),
        occurred_at: new Date().toISOString(),
      };
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 5000);
      const race = Promise.race([
        postTelemetry(event, { signal: controller.signal }),
        new Promise<never>((_, reject) =>
          setTimeout(() => reject(new Error('timeout')), 5000),
        ),
      ]);
      let res: Response;
      try {
        res = (await race) as Response;
      } finally {
        clearTimeout(timeout);
      }
      if (res.status === 202) {
        set({ conn: 'connected', sync: 'CONNECTED', simEnabled: true });
      } else {
        set({ conn: 'error', sync: 'ERROR', simEnabled: false });
      }
    } catch {
      set({ conn: 'error', sync: 'ERROR', simEnabled: false });
    }
  },

  disconnect: () =>
    set({
      plate: '',
      conn: 'idle',
      sync: 'CONNECTING',
      net: 'OK',
      db: 'OK',
      simOn: false,
      simEnabled: false,
      selectedRoute: null,
    }),

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
    }),
}));
