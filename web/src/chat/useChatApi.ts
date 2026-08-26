import { useCallback, useEffect, useRef, useState } from "react";
import { getApiBase } from "../lib/api";

export type Citation = { tool: string; count: number };
export type ChatResponse = { reply: string; citations?: Citation[] };

function buildRequest(trimmed: string, signal?: AbortSignal): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ message: trimmed }),
    ...(signal ? { signal } : {}),
  };
}

async function fetchWithFallbackSignal(base: string, trimmed: string, signal: AbortSignal): Promise<Response> {
  try {
    return await fetch(`${base}/api/chat`, buildRequest(trimmed, signal));
  } catch (e: unknown) {
    const msg = (e as { message?: string })?.message ?? "";
    if (msg.includes("Expected signal")) return fetch(`${base}/api/chat`, buildRequest(trimmed));
    throw e;
  }
}

function mapStatusToError(status: number, retryAfter: string | null, fallbackText: string): string | null {
  if (status === 429) {
    const v = retryAfter ?? "6";
    return `429 rate limited retry after ${v}s (Retry-After: ${v})`;
  }
  if (status === 503) return "agente temporalmente no disponible";
  if (status < 200 || status >= 300) return fallbackText || `error ${status}`;
  return null;
}

function mapThrownToError(err: unknown): string {
  const e = err as { name?: string; message?: string };
  if (e?.name === "AbortError") return "timeout 15s agente no disponible";
  return e?.message ?? "error de red";
}

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

  const sendMessage = useCallback(
    async (message: string): Promise<ChatResponse | null> => {
      if (loading) return null;
      const trimmed = message.trim();
      if (!trimmed) return null;
      const prevCtrl = abortRef.current;
      const prevT = timeoutRef.current;
      if (prevCtrl) prevCtrl.abort();
      if (prevT) clearTimeout(prevT);
      const base = getApiBase();
      const controller = new AbortController();
      abortRef.current = controller;
      const id = setTimeout(() => controller.abort(), 15000);
      timeoutRef.current = id;
      setLoading(true);
      setError(null);
      try {
        const res = await fetchWithFallbackSignal(base, trimmed, controller.signal);
        if (!res.ok) {
          const text = await res.text();
          const mapped = mapStatusToError(res.status, res.headers.get("Retry-After"), text);
          if (mapped) setError(mapped);
          return null;
        }
        const data = (await res.json()) as ChatResponse;
        return data;
      } catch (err: unknown) {
        setError(mapThrownToError(err));
        return null;
      } finally {
        clearTimeout(id);
        if (timeoutRef.current === id) timeoutRef.current = null;
        if (abortRef.current === controller) abortRef.current = null;
        setLoading(false);
      }
    },
    [loading],
  );

  const clearError = useCallback(() => setError(null), []);

  return { loading, error, sendMessage, clearError };
}
