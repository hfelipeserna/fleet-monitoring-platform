import { vi } from "vitest";

export class MockEventSource {
  static instances: MockEventSource[] = [];
  url: string;
  readyState = 0;
  onopen: ((e: Event) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  close = vi.fn(() => {
    this.readyState = 2;
  });
  private listeners: Record<string, (e: MessageEvent) => void> = {};
  addEventListener = vi.fn((event: string, handler: EventListener) => {
    this.listeners[event] = handler as unknown as (e: MessageEvent) => void;
    if (event === "message") this.onmessage = handler as unknown as (e: MessageEvent) => void;
    if (event === "error") this.onerror = handler as unknown as (e: Event) => void;
    if (event === "open") this.onopen = handler as unknown as (e: Event) => void;
  });
  removeEventListener = vi.fn((event: string) => {
    delete this.listeners[event];
  });
  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }
  simulateOpen() {
    this.readyState = 1;
    this.onopen?.(new Event("open"));
  }
  simulateError() {
    this.readyState = 2;
    this.onerror?.(new Event("error"));
  }
  simulateMessage(data: string, eventType = "message", lastEventId?: string) {
    const evt = new MessageEvent(eventType, { data } as MessageEventInit);
    if (lastEventId) Object.defineProperty(evt, "lastEventId", { value: lastEventId, writable: true });
    if (eventType === "message") {
      this.onmessage?.(evt);
    }
    const handler = this.listeners[eventType];
    if (handler) handler(evt);
    return evt;
  }
  simulateAlert(payload: unknown, lastEventId?: string) {
    const data = JSON.stringify(payload);
    const evt = new MessageEvent("alert:critical", { data } as MessageEventInit);
    if (lastEventId) Object.defineProperty(evt, "lastEventId", { value: lastEventId, writable: true });
    this.onmessage?.(new MessageEvent("message", { data } as MessageEventInit));
    const handler = this.listeners["alert:critical"];
    if (handler) handler(evt);
    return evt;
  }
  simulatePing() {
    return;
  }
  simulateRawPingAsMessage() {
    const evt = new MessageEvent("message", { data: ":ping" } as MessageEventInit);
    this.onmessage?.(evt);
    const handler = this.listeners["alert:critical"];
    if (handler) handler(new MessageEvent("alert:critical", { data: ":ping" } as MessageEventInit));
    return evt;
  }
}
