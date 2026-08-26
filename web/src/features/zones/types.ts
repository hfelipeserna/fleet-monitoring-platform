export type DraftPolygon = { type: "Polygon"; coordinates: number[][][] };

export type ZoneFeature = {
  type: "Feature";
  id: string;
  properties: { name: string };
  geometry: { type: "Polygon"; coordinates: number[][][] };
};

export type ZonesFC = {
  type: "FeatureCollection";
  features: ZoneFeature[];
};

export type ValidateResult = { valid: boolean; reason?: string };

export function validatePolygon(coords: number[][]): ValidateResult {
  if (!Array.isArray(coords) || coords.length < 4 || coords.length > 101) {
    return { valid: false, reason: "coords length must be 4..101" };
  }
  const first = coords[0];
  const last = coords[coords.length - 1];
  if (!Array.isArray(first) || !Array.isArray(last) || first.length < 2 || last.length < 2) {
    return { valid: false, reason: "invalid coordinate" };
  }
  const eqClosed =
    Math.round(first[0] * 1e6) / 1e6 === Math.round(last[0] * 1e6) / 1e6 &&
    Math.round(first[1] * 1e6) / 1e6 === Math.round(last[1] * 1e6) / 1e6;
  if (!eqClosed) return { valid: false, reason: "polygon not closed first!==last" };
  return { valid: true };
}
