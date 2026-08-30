import { useAppStore } from '../store/appStore';
import { getTelemetryPort } from '../store/ports';
import type { TelemetryPort } from '../store/ports';

export type RouteKind = 'medellin' | 'bogota';
export type RoutePoint = { lat: number; lon: number; speed: number };

export function nextSimPoint(route: Array<RoutePoint>, idx: number): RoutePoint {
  if (!route || route.length === 0) return { lat: 0, lon: 0, speed: 0 };
  return route[idx % route.length];
}

export function resetSimIdx(): void {
  useAppStore.setState({ selectedRouteIdx: 0 } as unknown as Record<string, unknown>);
}

export function getSimIdx(): number {
  return (useAppStore.getState() as unknown as { selectedRouteIdx: number }).selectedRouteIdx ?? 0;
}

async function resolveClearPending(port?: TelemetryPort): Promise<() => Promise<void>> {
  const injected = port ?? getTelemetryPort();
  if (injected) return () => injected.clearPending();
  const mod = await import('../db/telemetry');
  return mod.clearPending as () => Promise<void>;
}

export async function selectRoute(route: RouteKind, telemetryPort?: TelemetryPort): Promise<void> {
  const fn = await resolveClearPending(telemetryPort);
  await fn();
  useAppStore.setState({ selectedRoute: route, selectedRouteIdx: 0 } as unknown as Record<string, unknown>);
}

export async function toggleSim(v: boolean, telemetryPort?: TelemetryPort): Promise<void> {
  if (useAppStore.getState().conn !== 'connected') return;
  if (!v) {
    const fn = await resolveClearPending(telemetryPort);
    try {
      await fn();
    } catch {}
    useAppStore.setState({ simOn: false, selectedRoute: null, selectedRouteIdx: 0 } as unknown as Record<string, unknown>);
    return;
  }
  useAppStore.setState({ simOn: true } as unknown as Record<string, unknown>);
}

export function useSimulatedRoute() {
  const selectedRoute = useAppStore((s) => s.selectedRoute);
  const simOn = useAppStore((s) => s.simOn);
  const selectedRouteIdx = useAppStore((s) => (s as unknown as { selectedRouteIdx: number }).selectedRouteIdx ?? 0);
  return {
    selectedRoute,
    simOn,
    selectedRouteIdx,
    selectRoute,
    nextSimPoint,
    resetSimIdx,
    getSimIdx,
    toggleSim,
  };
}
