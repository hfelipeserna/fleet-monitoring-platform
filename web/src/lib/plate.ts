export const PLATE_RE = /^[A-Z]{3}[0-9]{3}$/;
export const PLATE_RE_GLOBAL = /\b[A-Z]{3}[0-9]{3}\b/g;

export function isValidPlate(s: string): boolean {
  return PLATE_RE.test(s);
}
