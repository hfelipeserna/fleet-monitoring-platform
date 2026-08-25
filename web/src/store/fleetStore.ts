import { create } from "zustand";

type FleetStore = {
  selectedPlate: string | null;
  setSelectedPlate: (plate: string | null) => void;
};

export const useFleetStore = create<FleetStore>((set) => ({
  selectedPlate: null,
  setSelectedPlate: (plate) => set({ selectedPlate: plate }),
}));
