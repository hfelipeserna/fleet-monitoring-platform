import type { FleetAlert } from "../hooks/useAlertsSSE";

export function getDedupKey(alert: FleetAlert): string {
  if (alert.event_id) return alert.event_id;
  return `${alert.plate}-${alert.created_at}-${alert.alert_type}`;
}

export function isValidAlert(d: unknown): d is FleetAlert {
  if (!d || typeof d !== "object") return false;
  const a = d as FleetAlert;
  return Boolean(a.plate && a.alert_type && a.created_at);
}

export function isKeepAlive(raw: unknown): boolean {
  const s = String(raw ?? "").trim();
  return s === "" || s === ":ping" || s === ": ping";
}

export function parseAlert(raw: unknown): FleetAlert | null {
  if (typeof raw !== "string") return null;
  const trimmed = raw.trim();
  if (isKeepAlive(trimmed)) return null;
  try {
    const data = JSON.parse(trimmed) as FleetAlert;
    if (!isValidAlert(data)) return null;
    return data;
  } catch {
    return null;
  }
}
