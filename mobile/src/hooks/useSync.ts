import { useEffect, useRef } from 'react';
import { useAppStore } from '../store/appStore';
import { getTelemetryPort } from '../store/ports';
import { flushPending } from '../lib/sync';

export function useSync(): void {
  const net = useAppStore((s) => s.net);
  const conn = useAppStore((s) => s.conn);
  const retryAttemptsRef = useRef(0);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const flushingRef = useRef(false);

  useEffect(() => {
    const clearBackoff = () => {
      if (timeoutRef.current !== null) {
        clearTimeout(timeoutRef.current as unknown as number);
        timeoutRef.current = null;
      }
    };

    const abortPending = () => {
      if (abortRef.current) {
        try {
          abortRef.current.abort();
        } catch {}
        abortRef.current = null;
      }
    };

    const doFlush = async () => {
      if (flushingRef.current) return;
      const { net: curNet, conn: curConn } = useAppStore.getState();
      if (curNet !== 'OK' || curConn !== 'connected') return;

      let pendingCount = 0;
      try {
        const port = getTelemetryPort();
        if (port) pendingCount = await port.countPending();
        else {
          const mod = await import('../db/telemetry');
          pendingCount = await mod.countPending();
        }
      } catch {
        return;
      }

      if (pendingCount === 0) return;

      if (abortRef.current) {
        try {
          abortRef.current.abort();
        } catch {}
      }
      const controller = new AbortController();
      abortRef.current = controller;
      flushingRef.current = true;

      try {
        const port = getTelemetryPort() ?? undefined;
        const n = await flushPending({ port, signal: controller.signal });
        if (n > 0) retryAttemptsRef.current = 0;
      } catch (e: unknown) {
        const err = e as { name?: string; retryAfter?: number; status?: number; backoffMs?: number };
        if (controller.signal.aborted || err?.name === 'AbortError') return;
        if (err?.status === 503) {
          try {
            useAppStore.getState().setSync('ERROR');
          } catch {}
        }
        if (err?.status === 429 || err?.status === 503 || typeof err?.retryAfter === 'number') {
          retryAttemptsRef.current += 1;
          const jitter = Math.random() * 1000;
          const backoff = err.backoffMs ?? Math.min(5000 * Math.pow(2, retryAttemptsRef.current), 60000) + jitter;
          const retryAfterMs = (err.retryAfter ?? 5) * 1000;
          const delay = Math.max(backoff, retryAfterMs);
          clearBackoff();
          timeoutRef.current = setTimeout(() => {
            timeoutRef.current = null;
            void doFlush();
          }, delay);
        } else {
          retryAttemptsRef.current += 1;
          const jitter = Math.random() * 1000;
          const delay = Math.min(5000 * Math.pow(2, retryAttemptsRef.current), 60000) + jitter;
          clearBackoff();
          timeoutRef.current = setTimeout(() => {
            timeoutRef.current = null;
            void doFlush();
          }, delay);
        }
      } finally {
        flushingRef.current = false;
      }
    };

    const checkAndFlush = () => {
      const { net: curNet, conn: curConn } = useAppStore.getState();
      if (curNet === 'OK' && curConn === 'connected') {
        void doFlush();
      }
    };

    const triggerIfReady = async () => {
      const { net: curNet, conn: curConn } = useAppStore.getState();
      if (curNet !== 'OK' || curConn !== 'connected') return;
      let pending = 0;
      try {
        const port = getTelemetryPort();
        if (port) pending = await port.countPending();
        else {
          const mod = await import('../db/telemetry');
          pending = await mod.countPending();
        }
      } catch {}
      if (pending >= 50) {
        clearBackoff();
        void doFlush();
      }
    };

    checkAndFlush();
    intervalRef.current = setInterval(() => {
      const { net: curNet, conn: curConn } = useAppStore.getState();
      if (curNet === 'OK' && curConn === 'connected') {
        void triggerIfReady().then(() => checkAndFlush());
      }
    }, 5000);

    const unsub = useAppStore.subscribe((state, prev) => {
      if (state.net === 'OK' && state.conn === 'connected' && (prev.net !== 'OK' || prev.conn !== 'connected')) {
        clearBackoff();
        retryAttemptsRef.current = 0;
        void doFlush();
      }
    });

    return () => {
      if (intervalRef.current !== null) {
        clearInterval(intervalRef.current as unknown as number);
        intervalRef.current = null;
      }
      clearBackoff();
      abortPending();
      try {
        unsub();
      } catch {}
    };
  }, [net, conn]);
}
