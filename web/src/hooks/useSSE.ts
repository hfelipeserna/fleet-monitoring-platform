import { useEffect, useRef } from "react";

export function useSSE(
  url: string | null,
  opts?: { onMessage?: (e: MessageEvent) => void },
): void {
  const onMessageRef = useRef(opts?.onMessage);
  useEffect(() => {
    onMessageRef.current = opts?.onMessage;
  }, [opts?.onMessage]);

  useEffect(() => {
    if (!url) return;
    let closed = false;
    let backoff = 500;
    let timer: number | null = null;
    let es: EventSource | null = null;

    function connect() {
      if (closed) return;
      es = new EventSource(url as string);
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
      es.onmessage = (e: MessageEvent) => {
        onMessageRef.current?.(e);
      };
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
