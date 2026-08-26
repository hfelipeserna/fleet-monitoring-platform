import "@testing-library/jest-dom";
import { afterEach, vi } from "vitest";

vi.stubGlobal("matchMedia", (query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: () => undefined,
  removeListener: () => undefined,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
  dispatchEvent: () => false,
}));

class MockResizeObserver {
  observe() {
    return;
  }
  unobserve() {
    return;
  }
  disconnect() {
    return;
  }
}
vi.stubGlobal("ResizeObserver", MockResizeObserver as unknown as typeof ResizeObserver);

if (typeof window.URL !== "undefined" && !window.URL.createObjectURL) {
  window.URL.createObjectURL = () => "blob:fake";
}

Object.defineProperty(window, "devicePixelRatio", { value: 1, writable: true });

window.scrollTo = () => undefined;
Element.prototype.scrollTo = () => undefined;

const _origCreateElementNS = document.createElementNS.bind(document);
document.createElementNS = ((ns: string, tag: string) => {
  const el = _origCreateElementNS(ns, tag);
  if (
    ns === "http://www.w3.org/2000/svg" &&
    tag === "svg" &&
    !(el as unknown as { createSVGRect?: unknown }).createSVGRect
  ) {
    (el as unknown as { createSVGRect: () => unknown }).createSVGRect = () => ({});
  }
  return el;
}) as typeof document.createElementNS;

afterEach(() => {
  vi.restoreAllMocks();
});
