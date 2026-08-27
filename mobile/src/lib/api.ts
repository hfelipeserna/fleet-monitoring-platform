import Constants from 'expo-constants';

export const API_TIMEOUT_MS = 5000;

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

function getTimeoutSignal(): AbortSignal {
  const ac = new AbortController();
  setTimeout(() => {
    try {
      ac.abort();
    } catch {}
  }, API_TIMEOUT_MS);
  if (typeof (AbortSignal as unknown as { timeout?: (ms: number) => AbortSignal }).timeout === 'function') {
    void (AbortSignal as unknown as { timeout: (ms: number) => AbortSignal }).timeout(API_TIMEOUT_MS);
  }
  return ac.signal;
}

function getCombinedSignal(external?: AbortSignal): AbortSignal {
  const timeoutSignal = getTimeoutSignal();
  if (!external) return timeoutSignal;
  const anyFn = (AbortSignal as unknown as { any?: (signals: AbortSignal[]) => AbortSignal }).any;
  if (typeof anyFn === 'function') return anyFn([external, timeoutSignal]);
  const controller = new AbortController();
  if (external.aborted || timeoutSignal.aborted) controller.abort();
  else {
    const onAbort = () => {
      try {
        controller.abort();
      } catch {}
    };
    external.addEventListener('abort', onAbort, { once: true });
    timeoutSignal.addEventListener('abort', onAbort, { once: true });
  }
  return controller.signal;
}

async function fetchWithSignal(url: string, init: RequestInit, signal: AbortSignal): Promise<Response> {
  if (signal.aborted) throw new DOMException('Aborted', 'AbortError');
  return await new Promise<Response>((resolve, reject) => {
    const onAbort = () => {
      reject(new DOMException('Aborted', 'AbortError'));
    };
    signal.addEventListener('abort', onAbort, { once: true });
    fetch(url, { ...init, signal })
      .then((res) => {
        signal.removeEventListener('abort', onAbort);
        resolve(res);
      })
      .catch((err) => {
        signal.removeEventListener('abort', onAbort);
        reject(err);
      });
  });
}

export async function postTelemetry(event: unknown, opts?: { signal?: AbortSignal }): Promise<Response> {
  const signal = getCombinedSignal(opts?.signal);
  return fetchWithSignal(`${getBaseUrl()}/v1/telemetry`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(event),
  }, signal);
}

export async function postBatch(events: unknown[], opts?: { signal?: AbortSignal }): Promise<Response> {
  const signal = getCombinedSignal(opts?.signal);
  return fetchWithSignal(`${getBaseUrl()}/v1/telemetry/batch`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ events }),
  }, signal);
}

export { getBaseUrl };
