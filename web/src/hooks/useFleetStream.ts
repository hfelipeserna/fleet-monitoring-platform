import { useCallback, useMemo } from "react";
import { useFleetStore, type FleetPosition } from "../store/fleetStore";
import { getApiBase } from "../lib/api";
import { useSSE } from "./useSSE";

export function parseFleetPosition(raw: string): FleetPosition | null {
  try {
    const data = JSON.parse(raw) as Record<string, unknown>;
    if (!data || typeof data.plate !== "string" || typeof data.received_at !== "string") return null;
    const latRaw = data.lat as unknown;
    const lonRaw = data.lon as unknown;
    const lat = latRaw === null || latRaw === undefined ? null : typeof latRaw === "number" ? latRaw : null;
    const lon = lonRaw === null || lonRaw === undefined ? null : typeof lonRaw === "number" ? lonRaw : null;
    if (lat !== null && typeof lat !== "number") return null;
    if (lon !== null && typeof lon !== "number") return null;
    if (lat === null && lon === null) {
      // allow nullable coords per BR-006, keep raw nulls
    } else if (lat === null || lon === null) {
      // one null is still valid per BR-006 but keep as null; do not discard
    }
    // validate speed is number if present, default? require it
    if (typeof data.speed !== "number") return null;
    return {
      plate: data.plate as string,
      lat,
      lon,
      speed: data.speed as number,
      received_at: data.received_at as string,
    };
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
