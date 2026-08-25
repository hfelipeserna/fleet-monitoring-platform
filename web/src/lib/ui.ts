export const PANEL_FIXED = "h-[280px] lg:h-[340px] overflow-y-auto";
export const ZONES_PANEL_FIXED = "h-[360px] lg:h-[480px] overflow-y-auto";

export function getTabClass(active: boolean): string {
  return active
    ? "bg-black text-white px-3 py-1 focus:outline-none focus:ring-2 focus:ring-black"
    : "bg-white border border-black px-3 py-1 focus:outline-none focus:ring-2 focus:ring-black";
}
