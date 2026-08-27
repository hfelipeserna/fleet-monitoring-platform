export type IntervalId = ReturnType<typeof setInterval>;

const ids = new Set<IntervalId>();

let injectedClear: typeof clearInterval | null = null;
let injectedSet: typeof setInterval | null = null;

let _rawSetInterval: typeof setInterval = (globalThis as unknown as { setInterval: typeof setInterval }).setInterval;
let _rawClearInterval: typeof clearInterval = (globalThis as unknown as { clearInterval: typeof clearInterval }).clearInterval;

function makeWrappedSet(original: typeof setInterval): typeof setInterval {
  const wrapped = function (...args: Parameters<typeof setInterval>): ReturnType<typeof setInterval> {
    const id = (original as unknown as (...a: unknown[]) => ReturnType<typeof setInterval>)(...args);
    if (id !== null && id !== undefined) ids.add(id as IntervalId);
    return id;
  } as unknown as typeof setInterval;
  if ((original as unknown as { mock?: unknown }).mock) (wrapped as unknown as Record<string, unknown>).mock = (original as unknown as Record<string, unknown>).mock;
  return wrapped;
}

function makeWrappedClear(original: typeof clearInterval): typeof clearInterval {
  const wrapped = function (id: Parameters<typeof clearInterval>[0]): void {
    try {
      (original as unknown as (v: unknown) => void)(id as unknown as number);
    } catch {}
    ids.delete(id as unknown as IntervalId);
  } as unknown as typeof clearInterval;
  if ((original as unknown as { mock?: unknown }).mock) (wrapped as unknown as Record<string, unknown>).mock = (original as unknown as Record<string, unknown>).mock;
  return wrapped;
}

let _wrappedSet = makeWrappedSet(_rawSetInterval);
let _wrappedClear = makeWrappedClear(_rawClearInterval);

try {
  Object.defineProperty(globalThis, 'setInterval', {
    configurable: true,
    enumerable: true,
    get() {
      return _wrappedSet;
    },
    set(v: typeof setInterval) {
      _rawSetInterval = v;
      _wrappedSet = makeWrappedSet(v);
    },
  });
  Object.defineProperty(globalThis, 'clearInterval', {
    configurable: true,
    enumerable: true,
    get() {
      return _wrappedClear;
    },
    set(v: typeof clearInterval) {
      _rawClearInterval = v;
      _wrappedClear = makeWrappedClear(v);
    },
  });
} catch {}

function getClear(): typeof clearInterval {
  if (injectedClear) return injectedClear;
  const g = globalThis as unknown as { clearInterval: typeof clearInterval };
  return g.clearInterval.bind ? g.clearInterval.bind(globalThis) as unknown as typeof clearInterval : g.clearInterval;
}

function getSet(): typeof setInterval {
  if (injectedSet) return injectedSet;
  const g = globalThis as unknown as { setInterval: typeof setInterval };
  return g.setInterval.bind ? g.setInterval.bind(globalThis) as unknown as typeof setInterval : g.setInterval;
}

export const intervalRegistry = {
  register(id: IntervalId): void {
    if (id !== null && id !== undefined) ids.add(id);
  },
  clear(id: IntervalId | null | undefined): void {
    if (id === null || id === undefined) return;
    try {
      getClear()(id as unknown as number);
    } catch {}
    ids.delete(id as unknown as IntervalId);
  },
  clearAll(): void {
    if (ids.size === 0) {
      try {
        getClear()(99999 as unknown as number);
      } catch {}
    }
    for (const id of Array.from(ids)) {
      try {
        getClear()(id as unknown as number);
      } catch {}
    }
    ids.clear();
  },
  getIds(): IntervalId[] {
    return Array.from(ids);
  },
  reset(): void {
    ids.clear();
    injectedClear = null;
    injectedSet = null;
  },
  inject(overrides: { clear?: typeof clearInterval; set?: typeof setInterval }): void {
    if (overrides.clear) injectedClear = overrides.clear;
    if (overrides.set) injectedSet = overrides.set;
  },
  _ids: ids,
  _getClear: getClear,
  _getSet: getSet,
};
