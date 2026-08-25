export const PLATE_RE = /^[A-Z]{3}[0-9]{3}$/;
export const PLATE_RE_SOURCE = "\\b[A-Z]{3}[0-9]{3}\\b";
export const PLATE_RE_GLOBAL = new RegExp(PLATE_RE_SOURCE, "g");

export function isValidPlate(s: string): boolean {
  return PLATE_RE.test(s);
}
