import { create } from "zustand";

export type ActiveBottom = "alerts" | "chat";
export type ActiveTop = "monitoring" | "zones";

type PortalStore = {
  activeBottom: ActiveBottom;
  activeTop: ActiveTop;
  setActiveBottom: (v: ActiveBottom) => void;
  setActiveTop: (v: ActiveTop) => void;
};

export const usePortalStore = create<PortalStore>((set) => ({
  activeBottom: "alerts",
  activeTop: "monitoring",
  setActiveBottom: (activeBottom) => set({ activeBottom }),
  setActiveTop: (activeTop) => set({ activeTop }),
}));

export default usePortalStore;
