try {
  require('react-native-get-random-values');
} catch {}
import { useEffect, useRef } from 'react';
import { v4 as uuidv4 } from 'uuid';
import * as Location from 'expo-location';
import { useAppStore } from '../store/appStore';
import { getTelemetryPort, getIntervalPort } from '../store/ports';
import type { TelemetryPort, IntervalPort } from '../store/ports';
import { nextSimPoint } from './useSimulatedRoute';
import { MEDELLIN_ROUTE } from '../routes/medellin';
import { BOGOTA_ROUTE } from '../routes/bogota';

export async function nextGpsPoint(): Promise<{ lat: number; lon: number; speed: number }> {
  const perm = await Location.requestForegroundPermissionsAsync();
  if (perm.status !== 'granted') throw new Error('location permission denied');
  const pos = await Location.getCurrentPositionAsync({});
  return { lat: pos.coords.latitude, lon: pos.coords.longitude, speed: pos.coords.speed ?? 0 };
}

function intervalClear(id: number): void {
  const port = getIntervalPort();
  if (port) {
    port.clear(id);
    return;
  }
  clearInterval(id);
}

function intervalRegister(id: number): number {
  const port = getIntervalPort();
  if (port) return port.register(id);
  return id;
}

async function enqueuePoint(point: unknown, port?: TelemetryPort): Promise<void> {
  const injected = port ?? getTelemetryPort();
  if (injected) {
    await injected.enqueue(point);
    return;
  }
  const mod = await import('../db/telemetry');
  await (mod.enqueue as unknown as (p: unknown) => Promise<void>)(point);
}

export function useTelemetryGenerator(telemetryPort?: TelemetryPort, intervalPort?: IntervalPort): void {
  const conn = useAppStore((s) => s.conn);
  const simOn = useAppStore((s) => s.simOn);
  const selectedRoute = useAppStore((s) => s.selectedRoute);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const latestPosRef = useRef<{ lat: number; lon: number; speed: number } | null>(null);
  const watchSubRef = useRef<{ remove: () => void } | null>(null);
  const permissionGrantedRef = useRef<boolean | null>(null);

  useEffect(() => {
    if (conn !== 'connected') {
      if (intervalRef.current !== null && intervalRef.current !== undefined) {
        const idNum = intervalRef.current as unknown as number;
        if (intervalPort) intervalPort.clear(idNum);
        else intervalClear(idNum);
        intervalRef.current = null;
      }
      if (watchSubRef.current) {
        try {
          watchSubRef.current.remove();
        } catch {}
        watchSubRef.current = null;
      }
      latestPosRef.current = null;
      permissionGrantedRef.current = null;
      return;
    }

    if (intervalRef.current !== null && intervalRef.current !== undefined) {
      const idNum = intervalRef.current as unknown as number;
      if (intervalPort) intervalPort.clear(idNum);
      else intervalClear(idNum);
    }

    if (simOn) {
      if (watchSubRef.current) {
        try {
          watchSubRef.current.remove();
        } catch {}
        watchSubRef.current = null;
      }
      latestPosRef.current = null;
      permissionGrantedRef.current = null;
    } else {
      if (!watchSubRef.current) {
        (async () => {
          try {
            const perm = await Location.requestForegroundPermissionsAsync();
            permissionGrantedRef.current = perm.status === 'granted';
            if (!permissionGrantedRef.current) return;
            const sub = await (Location as unknown as { watchPositionAsync?: (opts: unknown, cb: (pos: { coords: { latitude: number; longitude: number; speed: number | null } }) => void) => Promise<{ remove: () => void }> }).watchPositionAsync?.(
              {
                accuracy: (Location as unknown as { Accuracy?: { High: number } }).Accuracy?.High ?? 6,
                timeInterval: 1000,
                distanceInterval: 0,
              },
              (pos) => {
                latestPosRef.current = { lat: pos.coords.latitude, lon: pos.coords.longitude, speed: pos.coords.speed ?? 0 };
              },
            );
            if (sub && typeof sub.remove === 'function') {
              watchSubRef.current = sub;
            }
          } catch {}
        })();
      }
    }

    const id = setInterval(() => {
      void (async () => {
        try {
          const state = useAppStore.getState();
          const plate = state.plate;
          if (!plate) return;
          let pt: { lat: number; lon: number; speed: number };
          const curSimOn = state.simOn;
          const curRouteKind = state.selectedRoute;
          const curIdx = (state as unknown as { selectedRouteIdx?: number }).selectedRouteIdx ?? 0;
          if (curSimOn && curRouteKind) {
            const route = curRouteKind === 'medellin' ? MEDELLIN_ROUTE : BOGOTA_ROUTE;
            pt = nextSimPoint(route as unknown as Array<{ lat: number; lon: number; speed: number }>, curIdx);
            const nextIdx = route.length > 0 ? (curIdx + 1) % route.length : 0;
            useAppStore.setState({ selectedRouteIdx: nextIdx } as unknown as Record<string, unknown>);
          } else {
            const cached = latestPosRef.current;
            if (cached) {
              pt = cached;
            } else {
              if (permissionGrantedRef.current === false) return;
              if (permissionGrantedRef.current === null) {
                try {
                  const perm = await Location.requestForegroundPermissionsAsync();
                  permissionGrantedRef.current = perm.status === 'granted';
                  if (!permissionGrantedRef.current) return;
                } catch {
                  return;
                }
              }
              try {
                const pos = await Location.getCurrentPositionAsync({});
                pt = { lat: pos.coords.latitude, lon: pos.coords.longitude, speed: pos.coords.speed ?? 0 };
              } catch {
                return;
              }
            }
          }
          await enqueuePoint(
            {
              plate,
              lat: pt.lat,
              lon: pt.lon,
              speed: Math.round(pt.speed ?? 0),
              client_event_id: uuidv4(),
              occurred_at: new Date().toISOString(),
              sync_status: 'pending',
            },
            telemetryPort,
          );
        } catch {}
      })().catch(() => {});
    }, 5000);

    if (intervalPort) intervalPort.register(id as unknown as number);
    else intervalRegister(id as unknown as number);
    intervalRef.current = id;

    return () => {
      const idNum = id as unknown as number;
      if (intervalPort) intervalPort.clear(idNum);
      else intervalClear(idNum);
      if (intervalRef.current === id) {
        intervalRef.current = null;
      }
      if (watchSubRef.current) {
        try {
          watchSubRef.current.remove();
        } catch {}
        watchSubRef.current = null;
      }
      permissionGrantedRef.current = null;
    };
  }, [conn, simOn, selectedRoute, telemetryPort, intervalPort]);
}
