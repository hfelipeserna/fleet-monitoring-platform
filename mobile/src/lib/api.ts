import Constants from 'expo-constants';

const getBaseUrl = (): string => {
  const c = Constants as unknown as Record<string, unknown>;
  const extra =
    (c.expoConfig as { extra?: Record<string, unknown> } | undefined)?.extra ??
    (c.default as { expoConfig?: { extra?: Record<string, unknown> } } | undefined)?.expoConfig?.extra ??
    {};
  const g = globalThis as unknown as Record<string, unknown>;
  const rawEnv =
    ((g.process as Record<string, unknown> | undefined)?.env as Record<string, unknown> | undefined)?.EXPO_PUBLIC_API_URL as
      | string
      | undefined;
  const envUrl = rawEnv && rawEnv !== 'undefined' && rawEnv !== '' ? rawEnv : undefined;
  const extraUrl = (extra as Record<string, unknown>).apiUrl as string | undefined;
  const url = (extraUrl && extraUrl !== 'undefined' ? extraUrl : undefined) ?? envUrl ?? 'http://localhost:8080';
  return String(url).replace(/\/+$/, '');
};

export async function postTelemetry(event: unknown, opts?: { signal?: AbortSignal }): Promise<Response> {
  const controller = opts?.signal ? null : new AbortController();
  const signal = opts?.signal ?? (controller as unknown as AbortController).signal;
  const timeout = controller ? setTimeout(() => (controller as unknown as AbortController).abort(), 5000) : null;
  try {
    const res = await fetch(`${getBaseUrl()}/v1/telemetry`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(event),
      signal: signal as unknown as AbortSignal,
    });
    return res;
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

export async function postBatch(events: unknown[], opts?: { signal?: AbortSignal }): Promise<Response> {
  const controller = opts?.signal ? null : new AbortController();
  const signal = opts?.signal ?? (controller as unknown as AbortController).signal;
  const timeout = controller ? setTimeout(() => (controller as unknown as AbortController).abort(), 5000) : null;
  try {
    const res = await fetch(`${getBaseUrl()}/v1/telemetry/batch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ events }),
      signal: signal as unknown as AbortSignal,
    });
    return res;
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

export { getBaseUrl };
