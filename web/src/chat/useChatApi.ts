import { useCallback, useEffect, useRef, useState } from "react";
import { getApiBase } from "../lib/api";

export type Citation = { tool: string; count: number };
export type ChatResponse = { reply: string; citations?: Citation[] };

export function useChatApi() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timeoutRef.current !== null) clearTimeout(timeoutRef.current);
      abortRef.current?.abort();
    };
  }, []);

  const sendMessage = useCallback(async (message: string): Promise<ChatResponse | null> => {
    const trimmed = message.trim();
    if (!trimmed) return null;
    const base = getApiBase();
    const controller = new AbortController();
    abortRef.current = controller;
    const id = setTimeout(() => controller.abort(), 15000);
    timeoutRef.current = id;
    setLoading(true);
    setError(null);
    async function doFetch(withSignal: boolean) {
      return fetch(`${base}/api/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ message: trimmed }),
        ...(withSignal ? { signal: controller.signal } : {}),
      });
    }
    try {
      let res: Response;
      try {
        res = await doFetch(true);
      } catch (e: unknown) {
        const msg = (e as { message?: string })?.message ?? "";
        if (msg.includes("Expected signal")) res = await doFetch(false);
        else throw e;
      }
      if (res.status === 429) {
        const retryAfter = res.headers.get("Retry-After") ?? "6";
        setError(`429 rate limited retry after ${retryAfter}s (Retry-After: ${retryAfter})`);
        return null;
      }
      if (res.status === 503) {
        setError("agente temporalmente no disponible");
        return null;
      }
      if (!res.ok) {
        const text = await res.text();
        setError(text || `error ${res.status}`);
        return null;
      }
      const data = (await res.json()) as ChatResponse;
      return data;
    } catch (err: unknown) {
      const e = err as { name?: string; message?: string };
      if (e?.name === "AbortError") setError("timeout 15s agente no disponible");
      else setError(e?.message ?? "error de red");
      return null;
    } finally {
      clearTimeout(id);
      timeoutRef.current = null;
      setLoading(false);
    }
  }, []);

  const clearError = useCallback(() => setError(null), []);

  return { loading, error, sendMessage, clearError };
}
