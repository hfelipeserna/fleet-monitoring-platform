import { useEffect, useRef } from "react";

export function useSSE(
  url: string | null,
  opts?: { onMessage?: (e: MessageEvent) => void; event?: string },
): void {
  const onMessageRef = useRef(opts?.onMessage);
  const eventRef = useRef(opts?.event);
  useEffect(() => {
    onMessageRef.current = opts?.onMessage;
  }, [opts?.onMessage]);
  useEffect(() => {
    eventRef.current = opts?.event;
  }, [opts?.event]);

  useEffect(() => {
    if (!url) return;
    let closed = false;
    let backoff = 500;
    let timer: number | null = null;
    let es: EventSource | null = null;

    function connect() {
      if (closed) return;
      const ES = (globalThis as unknown as { EventSource?: new (u: string) => EventSource }).EventSource as
        | (new (u: string) => EventSource)
        | undefined;
      if (!ES) return;
      try {
        es = new ES(url as string);
      } catch {
        return;
      }
      es.onopen = () => {
        backoff = 500;
      };
      es.onerror = () => {
        es?.close();
        if (closed) return;
        const jitter = Math.random() * 500;
        const delay = backoff + jitter;
        timer = window.setTimeout(connect, delay);
        backoff = Math.min(backoff * 2, 30000);
      };
      const handler = (e: Event) => {
        onMessageRef.current?.(e as MessageEvent);
      };
      es.onmessage = handler as (e: MessageEvent) => void;
      const ev = eventRef.current;
      if (ev) {
        es.addEventListener(ev, handler);
      }
    }

    connect();

    return () => {
      closed = true;
      if (timer !== null) clearTimeout(timer);
      es?.close();
    };
  }, [url]);
}

export default useSSE;
