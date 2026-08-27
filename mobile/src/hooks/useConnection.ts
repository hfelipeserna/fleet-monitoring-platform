import 'react-native-get-random-values';
import { useCallback, useEffect, useRef } from 'react';
import { v4 as uuidv4 } from 'uuid';
import { useAppStore } from '../store/appStore';
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

    abortRef.current = new AbortController();

    try {
      await new Promise<void>((resolve) => setTimeout(resolve, 0));
      const fetchPromise = postTelemetry(event as unknown, { signal: abortRef.current.signal }) as Promise<Response>;
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
    }
  }, []);

  const disconnect = useCallback(() => {
    if (abortRef.current) {
      try {
        abortRef.current.abort();
      } catch {}
      abortRef.current = null;
    }
    const s = useAppStore.getState();
    if (s.disconnect) s.disconnect();
    else
      useAppStore.setState({
        plate: '',
        conn: 'idle',
        sync: 'CONNECTING',
        net: 'OK',
        db: 'OK',
      } as unknown as Record<string, unknown>);
  }, []);

  return { connect, disconnect };
}
