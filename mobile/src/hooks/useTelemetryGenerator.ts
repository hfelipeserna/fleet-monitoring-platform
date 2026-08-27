import 'react-native-get-random-values';
import { useEffect } from 'react';
import { v4 as uuidv4 } from 'uuid';
import { useAppStore } from '../store/appStore';
import { intervalRegistry } from '../store/intervalRegistry';
import { nextSimPoint, resetSimIdx } from './useSimulatedRoute';
import { MEDELLIN_ROUTE } from '../routes/medellin';
import { BOGOTA_ROUTE } from '../routes/bogota';

function nextGpsPoint(): { lat: number; lon: number; speed: number } {
  return { lat: 6.2442, lon: -75.5812, speed: 0 };
}

export function useTelemetryGenerator(): void {
  const conn = useAppStore((s) => s.conn);
  const simOn = useAppStore((s) => s.simOn);
  const selectedRoute = useAppStore((s) => s.selectedRoute);

  useEffect(() => {
    if (conn !== 'connected' || !simOn || !selectedRoute) {
      const existing = useAppStore.getState().__telemetryInterval;
      if (existing !== null && existing !== undefined) {
        intervalRegistry.clear(existing);
        useAppStore.setState({ __telemetryInterval: null } as unknown as Record<string, unknown>);
      }
      return;
    }

    const prev = useAppStore.getState().__telemetryInterval;
    if (prev !== null && prev !== undefined) {
      intervalRegistry.clear(prev);
    }

    const route = selectedRoute === 'medellin' ? MEDELLIN_ROUTE : BOGOTA_ROUTE;

    const id = setInterval(async () => {
      try {
        const { enqueue } = await import('../db/telemetry');
        const plate = useAppStore.getState().plate;
        if (!plate) return;
        let pt: { lat: number; lon: number; speed: number };
        if (simOn) {
          pt = nextSimPoint(route as unknown as Array<{ lat: number; lon: number; speed: number }>);
        } else {
          pt = nextGpsPoint();
        }
        await enqueue({
          plate,
          lat: pt.lat,
          lon: pt.lon,
          speed: pt.speed,
          client_event_id: uuidv4(),
          occurred_at: new Date().toISOString(),
          sync_status: 'pending',
        });
      } catch {}
    }, 5000);

    intervalRegistry.register(id);
    useAppStore.setState({ __telemetryInterval: id } as unknown as Record<string, unknown>);

    return () => {
      intervalRegistry.clear(id);
      const cur = useAppStore.getState().__telemetryInterval;
      if (cur === id) {
        useAppStore.setState({ __telemetryInterval: null } as unknown as Record<string, unknown>);
      }
    };
  }, [conn, simOn, selectedRoute]);

  useEffect(() => {
    const handler = () => {
      if (useAppStore.getState().simOn && useAppStore.getState().selectedRoute) {
        resetSimIdx();
      }
    };
    let unsub: (() => void) | null = null;
    try {
      const store: unknown = useAppStore as unknown;
      if (store && typeof (store as { subscribe?: unknown }).subscribe === 'function') {
        unsub = (useAppStore.subscribe as unknown as (fn: (s: unknown, prev: unknown) => void) => () => void)((state: unknown, prev: unknown) => {
          const s = state as { selectedRoute: string | null; simOn: boolean };
          const p = prev as { selectedRoute: string | null; simOn: boolean };
          if (s.selectedRoute !== p.selectedRoute) {
            resetSimIdx();
          }
        });
      }
    } catch {}
    return () => {
      if (unsub) {
        try {
          unsub();
        } catch {}
      }
      void handler;
    };
  }, []);
}

export { nextGpsPoint };
