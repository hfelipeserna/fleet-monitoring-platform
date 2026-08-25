import { useRef, useState } from "react";
import { useSSE } from "./useSSE";
import { getDedupKey, parseAlert } from "../lib/alert";

export type FleetAlert = {
  plate: string;
  alert_type: string;
  speed?: number;
  created_at: string;
  lat?: number;
  lon?: number;
  zone_name?: string;
  event_id?: string;
};

const MAX_ALERTS = 100;

export function useAlertsSSE(): FleetAlert[] {
  const [alerts, setAlerts] = useState<FleetAlert[]>([]);
  const seenRef = useRef<Set<string>>(new Set());

  useSSE("/api/alerts", {
    event: "alert:critical",
    onMessage: (e: MessageEvent) => {
      const raw = (e as MessageEvent).data;
      const data = parseAlert(raw);
      if (!data) return;
      const key = getDedupKey(data);
      if (seenRef.current.has(key)) return;
      seenRef.current.add(key);
      setAlerts((prev) => {
        const next = [...prev, data];
        if (next.length > MAX_ALERTS) {
          const evicted = next.shift();
          if (evicted) seenRef.current.delete(getDedupKey(evicted));
        }
        return next;
      });
    },
  });

  return alerts;
}

export default useAlertsSSE;
