import { useAppStore } from '../store/appStore';

export type RouteKind = 'medellin' | 'bogota';
export type RoutePoint = { lat: number; lon: number; speed: number };

let simIdx = 0;

export function nextSimPoint(route: Array<RoutePoint>): RoutePoint {
  if (!route || route.length === 0) return { lat: 0, lon: 0, speed: 0 };
  const pt = route[simIdx % route.length];
  simIdx = (simIdx + 1) % route.length;
  return pt;
}

export function resetSimIdx(): void {
  simIdx = 0;
}

export function getSimIdx(): number {
  return simIdx;
}

export async function selectRoute(route: RouteKind): Promise<void> {
  const { clearPending } = await import('../db/telemetry');
  await clearPending();
  simIdx = 0;
  useAppStore.setState({ selectedRoute: route } as unknown as Record<string, unknown>);
}

export async function toggleSim(v: boolean): Promise<void> {
  if (useAppStore.getState().conn !== 'connected') return;
  if (!v) {
    const { clearPending } = await import('../db/telemetry');
    try {
      await clearPending();
    } catch {}
    simIdx = 0;
    useAppStore.setState({ simOn: false, selectedRoute: null } as unknown as Record<string, unknown>);
    return;
  }
  useAppStore.setState({ simOn: true } as unknown as Record<string, unknown>);
}

export function useSimulatedRoute() {
  const selectedRoute = useAppStore((s) => s.selectedRoute);
  const simOn = useAppStore((s) => s.simOn);
  return {
    selectedRoute,
    simOn,
    selectRoute,
    nextSimPoint,
    resetSimIdx,
    getSimIdx,
    toggleSim,
  };
}
