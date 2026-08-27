export const PLATE_RE = /^[A-Z]{3}[0-9]{3}$/;
export const normalizePlate = (s: string): string => s.trim().toUpperCase();
export const isValidPlate = (s: string): boolean => PLATE_RE.test(normalizePlate(s));
