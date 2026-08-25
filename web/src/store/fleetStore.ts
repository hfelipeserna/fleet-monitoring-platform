import { create } from "zustand";

export type FleetPosition = {
  plate: string;
  lat: number | null;
  lon: number | null;
  speed: number;
  received_at: string;
};

type FleetStore = {
  selectedPlate: string | null;
  vehicles: Map<string, FleetPosition>;
  vehicle: FleetPosition | null;
  setSelectedPlate: (plate: string | null) => void;
  upsertVehicle: (data: FleetPosition) => void;
  setVehicles: (vehicles: FleetPosition[]) => void;
};

export const useFleetStore = create<FleetStore>((set) => ({
  selectedPlate: null,
  vehicles: new Map<string, FleetPosition>(),
  vehicle: null,
  setSelectedPlate: (plate) => set({ selectedPlate: plate }),
  upsertVehicle: (data) =>
    set((state) => {
      const next = new Map(state.vehicles);
      next.set(data.plate, data);
      return { vehicles: next };
    }),
  setVehicles: (vehicles) => set({ vehicles: new Map(vehicles.map((v) => [v.plate, v])) }),
}));
