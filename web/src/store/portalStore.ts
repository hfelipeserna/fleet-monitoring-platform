import { create } from "zustand";
import type { DraftPolygon } from "../features/zones/types";

export type ActiveBottom = "alerts" | "chat";
export type ActiveTop = "monitoring" | "zones";

type PortalStore = {
  activeBottom: ActiveBottom;
  activeTop: ActiveTop;
  draftPolygon: DraftPolygon | null;
  setActiveBottom: (v: ActiveBottom) => void;
  setActiveTop: (v: ActiveTop) => void;
  setDraftPolygon: (v: DraftPolygon | null) => void;
};

export const usePortalStore = create<PortalStore>((set) => ({
  activeBottom: "alerts",
  activeTop: "monitoring",
  draftPolygon: null,
  setActiveBottom: (activeBottom) => set({ activeBottom }),
  setActiveTop: (activeTop) => set({ activeTop }),
  setDraftPolygon: (draftPolygon) => set({ draftPolygon }),
}));

export default usePortalStore;
