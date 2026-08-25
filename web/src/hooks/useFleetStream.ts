import { useCallback, useMemo } from "react";
import { useFleetStore, type FleetPosition } from "../store/fleetStore";
import { getApiBase } from "../lib/api";
import { useSSE } from "./useSSE";

export function parseFleetPosition(raw: string): FleetPosition | null {
  try {
    const data = JSON.parse(raw) as FleetPosition;
    if (data && typeof data.plate === "string" && typeof data.lat === "number" && typeof data.lon === "number") {
      return data;
    }
    return null;
  } catch {
    return null;
  }
}

export function useFleetStream() {
  const selectedPlate = useFleetStore((s) => s.selectedPlate);
  const rawVehicles = useFleetStore((s) => s.vehicles);
  const upsertVehicle = useFleetStore((s) => s.upsertVehicle);

  const vehicles: FleetPosition[] = useMemo(() => Array.from(rawVehicles.values()), [rawVehicles]);

  const url = useMemo(() => {
    const base = getApiBase();
    const path = "/api/fleet/positions/stream";
    const qs = selectedPlate ? `?plate=${encodeURIComponent(selectedPlate)}` : "";
    const u = base ? `${base}${path}${qs}` : `${path}${qs}`;
    return u;
  }, [selectedPlate]);

  const handleMessage = useCallback(
    (e: MessageEvent) => {
      const parsed = parseFleetPosition((e as MessageEvent).data);
      if (parsed) upsertVehicle(parsed);
    },
    [upsertVehicle],
  );

  useSSE(url, { onMessage: handleMessage, event: "fleet:position" });

  const vehicle = useMemo(() => {
    if (!selectedPlate) return null;
    return rawVehicles.get(selectedPlate) ?? null;
  }, [rawVehicles, selectedPlate]);

  return { vehicles, vehicle, data: vehicles, selectedPlate };
}

export default useFleetStream;
