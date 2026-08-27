import 'react-native-get-random-values';
import { useCallback, useEffect, useRef } from 'react';
import { v4 as uuidv4 } from 'uuid';
import { useAppStore } from '../store/appStore';
import { intervalRegistry } from '../store/intervalRegistry';
import { postTelemetry } from '../lib/api';
import { isValidPlate } from '../lib/plate';

type ConnectParams = {
  plate: string;
  lat?: number;
  lon?: number;
  speed?: number;
};

export function useConnection() {
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
    useAppStore.setState({ __abortController: controller });

    try {
      await new Promise<void>((resolve) => setTimeout(resolve, 0));
      const fetchPromise = postTelemetry(event as unknown, { signal: controller.signal }) as Promise<Response>;
      await Promise.resolve();
      let timeoutId: ReturnType<typeof setTimeout> | undefined;
      const timeoutPromise = new Promise<never>((_, reject) => {
        timeoutId = setTimeout(() => reject(new Error('timeout')), 5000);
      });
      let res: Response;
      try {
        res = (await Promise.race([fetchPromise, timeoutPromise])) as Response;
      } finally {
        if (timeoutId) clearTimeout(timeoutId);
      }
      await new Promise<void>((resolve) => setTimeout(resolve, 0));
      if (res.status === 202) {
        useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', simEnabled: true });
      } else {
        useAppStore.setState({ conn: 'error', sync: 'ERROR', simEnabled: false });
      }
    } catch {
      await new Promise<void>((resolve) => setTimeout(resolve, 0));
      useAppStore.setState({ conn: 'error', sync: 'ERROR', simEnabled: false });
    } finally {
      abortRef.current = null;
      const current = useAppStore.getState().__abortController;
      if (current === controller) {
        useAppStore.setState({ __abortController: null });
      }
    }
  }, []);

  const disconnect = useCallback(async () => {
    const ac: AbortController | null = abortRef.current ?? useAppStore.getState().__abortController;
    if (ac) {
      try {
        ac.abort();
      } catch {}
      abortRef.current = null;
      useAppStore.setState({ __abortController: null });
    }
    const intervalId = useAppStore.getState().__telemetryInterval;
    if (intervalId !== null && intervalId !== undefined) {
      intervalRegistry.clear(intervalId);
      useAppStore.setState({ __telemetryInterval: null });
    } else {
      intervalRegistry.clearAll();
    }
    await useAppStore.getState().disconnect();
  }, []);

  return { connect, disconnect };
}
