export function getApiBase(): string {
  return (
    ((import.meta as unknown as { env?: Record<string, string> }).env?.VITE_API_BASE_URL ?? "").replace(/\/$/, "")
  );
}

export async function apiFetch(path: string, init: RequestInit & { timeoutMs?: number } = {}): Promise<Response> {
  const base = getApiBase();
  const url = path.startsWith("http") ? path : `${base}${path}`;
  const { timeoutMs = 15000, ...rest } = init;
  const controller = new AbortController();
  const id = window.setTimeout(() => controller.abort(), timeoutMs);
  const signal = (rest.signal ?? controller.signal) as unknown as AbortSignal;
  try {
    try {
      return await fetch(url, { ...rest, signal });
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message ?? "";
      if (msg.includes("Expected signal")) return await fetch(url, { ...rest });
      throw e;
    }
  } finally {
    window.clearTimeout(id);
  }
}
