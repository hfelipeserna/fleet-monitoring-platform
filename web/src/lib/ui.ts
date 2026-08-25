export const PANEL_HEIGHT = "h-[280px] lg:h-[340px]";
export const PANEL_FIXED = `${PANEL_HEIGHT} overflow-y-auto`;

export function getTabClass(active: boolean): string {
  return active
    ? "bg-black text-white px-3 py-1 focus:outline-none focus:ring-2 focus:ring-black"
    : "bg-white border border-black px-3 py-1 focus:outline-none focus:ring-2 focus:ring-black";
}
