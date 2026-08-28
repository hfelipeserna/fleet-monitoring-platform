import 'react-native-get-random-values';
import { useCallback, useEffect, useRef } from 'react';
import { v4 as uuidv4 } from 'uuid';
import { useAppStore } from '../store/appStore';
import { getIntervalPort } from '../store/ports';
import type { IntervalPort } from '../store/ports';
import { postTelemetry } from '../lib/api';
import { isValidPlate } from '../lib/plate';

type ConnectParams = {
  plate: string;
  lat?: number;
  lon?: number;
  speed?: number;
};

function intervalClearAll(port?: IntervalPort): void {
  const injected = port ?? getIntervalPort();
  if (injected) {
    injected.clearAll();
    return;
  }
  clearInterval(0 as unknown as number);
}

export function useConnection(intervalPort?: IntervalPort) {
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    return () => {
      if (abortRef.current) {
        try {
          abortRef.current.abort();
        } catch {}
      }
    };
  }, []);

  const connect = useCallback(async (params: ConnectParams) => {
    const plateRaw = params?.plate ?? '';
    const normalized = String(plateRaw).trim().toUpperCase();
    if (!isValidPlate(normalized)) {
      useAppStore.setState({ conn: 'error', sync: 'ERROR' });
      return;
    }

    const { db, net } = useAppStore.getState();
    useAppStore.setState({ plate: normalized, conn: 'connecting', sync: 'CONNECTING' });

    if (db !== 'OK' || net !== 'OK') {
      useAppStore.setState({ conn: 'error', sync: 'ERROR' });
      return;
    }

    const event = {
      plate: normalized,
      lat: params.lat ?? 0,
      lon: params.lon ?? 0,
      speed: params.speed ?? 0,
      client_event_id: uuidv4(),
      occurred_at: new Date().toISOString(),
    };

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      await Promise.resolve();
      const res = (await postTelemetry(event as unknown, { signal: controller.signal })) as Response;
      if (res.status === 202) {
        useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', simEnabled: true });
      } else if (res.status === 503) {
        useAppStore.setState({ conn: 'error', sync: 'ERROR', simEnabled: false });
      } else if (res.status === 429 || res.status === 400) {
        useAppStore.setState({ conn: 'error', sync: 'ERROR', simEnabled: false });
      } else {
        useAppStore.setState({ conn: 'error', sync: 'ERROR', simEnabled: false });
      }
    } catch (e: unknown) {
      if (controller.signal.aborted) {
        const cur = useAppStore.getState().conn;
        if (cur === 'idle') return;
      }
      const err = e as { name?: string; message?: string };
      const isAbort = err?.name === 'AbortError' || controller.signal.aborted;
      const isNetwork = !isAbort && (err?.message?.toLowerCase().includes('network') || err?.message?.toLowerCase().includes('failed to fetch') || err?.message?.toLowerCase().includes('load failed') || err instanceof TypeError);
      if (isNetwork) {
        console.warn('[conn] network error, keep CONNECTING for retry', err?.message);
        useAppStore.setState({ conn: 'connecting', sync: 'CONNECTING', simEnabled: false });
      } else {
        useAppStore.setState({ conn: 'error', sync: 'ERROR', simEnabled: false });
      }
    } finally {
      abortRef.current = null;
    }
  }, []);

  const disconnect = useCallback(async () => {
    const ac: AbortController | null = abortRef.current;
    if (ac) {
      try {
        ac.abort();
      } catch {}
      abortRef.current = null;
    }
    const port = intervalPort ?? getIntervalPort();
    if (port) {
      try {
        port.clearAll();
      } catch {}
    } else {
      intervalClearAll();
    }
    await useAppStore.getState().disconnect();
  }, [intervalPort]);

  return { connect, disconnect };
}
