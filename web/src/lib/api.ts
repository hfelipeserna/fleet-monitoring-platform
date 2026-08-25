export function getApiBase(): string {
  const metaEnv = (import.meta as unknown as { env?: Record<string, string> }).env?.VITE_API_BASE_URL;
  const procEnv = typeof process !== "undefined" ? (process.env as Record<string, string | undefined>).VITE_API_BASE_URL : undefined;
  return ((metaEnv ?? procEnv ?? "").replace(/\/$/, ""));
}

export function normalizeBase(url: string): string {
  return (url ?? "").replace(/\/$/, "");
}
