import "@testing-library/jest-dom";
import { afterAll, afterEach, vi } from "vitest";

const randomSpy = vi.spyOn(Math, "random").mockReturnValue(0);

afterEach(() => {
  if (!vi.isMockFunction(Math.random)) {
    vi.spyOn(Math, "random").mockReturnValue(0);
  }
});

afterAll(() => {
  randomSpy.mockRestore();
});

Object.defineProperty(HTMLElement.prototype, "clientWidth", {
  configurable: true,
  get() {
    return 800;
  },
});
Object.defineProperty(HTMLElement.prototype, "clientHeight", {
  configurable: true,
  get() {
    return 600;
  },
});
Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
  configurable: true,
  get() {
    return 800;
  },
});
Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
  configurable: true,
  get() {
    return 600;
  },
});

HTMLElement.prototype.getBoundingClientRect = function () {
  return {
    width: 800,
    height: 600,
    top: 0,
    left: 0,
    right: 800,
    bottom: 600,
    x: 0,
    y: 0,
    toJSON() {
      return {};
    },
  } as unknown as DOMRect;
} as never;

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

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
});

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
globalThis.ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver;

if (typeof window.URL !== "undefined" && !window.URL.createObjectURL) {
  window.URL.createObjectURL = () => "blob:fake";
}

Object.defineProperty(window, "devicePixelRatio", { value: 1, writable: true });

window.scrollTo = () => undefined;
Element.prototype.scrollTo = () => undefined;
