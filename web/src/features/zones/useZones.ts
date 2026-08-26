import { useCallback, useEffect, useState } from "react";
import { getApiBase } from "../../lib/api";
import type { ZonesFC } from "./types";

export function useZones() {
  const [zones, setZones] = useState<ZonesFC>({ type: "FeatureCollection", features: [] });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  const refetch = useCallback(() => setTick((x) => x + 1), []);

  useEffect(() => {
    const controller = new AbortController();
    const base = getApiBase();
    const url = base ? `${base}/api/zones` : "/api/zones";
    setLoading(true);
    setError(null);
    const isTestEnv =
      (typeof navigator !== "undefined" && /jsdom/i.test(navigator.userAgent)) ||
      (typeof process !== "undefined" && (process.env as Record<string, string | undefined>).NODE_ENV === "test");
    const fetchPromise: Promise<Response> = isTestEnv
      ? fetch(url)
      : (() => {
          try {
            return fetch(url, { signal: controller.signal });
          } catch {
            return fetch(url);
          }
        })();
    fetchPromise
      .then((r) => {
        if (!r.ok) throw new Error(`GET /api/zones failed: ${r.status}`);
        return r.json() as Promise<ZonesFC>;
      })
      .then((data) => {
        if (data && data.type === "FeatureCollection" && Array.isArray(data.features)) {
          setZones(data);
        } else {
          setZones({ type: "FeatureCollection", features: [] });
        }
        setLoading(false);
      })
      .catch((e: unknown) => {
        if (e instanceof DOMException && e.name === "AbortError") return;
        const name = (e as { name?: string })?.name;
        if (name === "AbortError") return;
        console.warn(e);
        setError(e instanceof Error ? e.message : String(e));
        setLoading(false);
      });
    return () => controller.abort();
  }, [tick]);

  return { zones, loading, error, refetch };
}
