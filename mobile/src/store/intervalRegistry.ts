export class IntervalRegistry {
  private ids = new Set<number>();
  register(id: number): number {
    this.ids.add(id);
    return id;
  }
  clear(id: number): void {
    clearInterval(id);
    this.ids.delete(id);
  }
  clearAll(): void {
    for (const id of this.ids) {
      try {
        clearInterval(id);
      } catch {}
    }
    this.ids.clear();
  }
  getIds(): number[] {
    return [...this.ids];
  }
  reset(): void {
    this.clearAll();
  }
}

export const intervalRegistry = new IntervalRegistry();

export const intervalPort = {
  register: (id: number) => intervalRegistry.register(id),
  clear: (id: number) => intervalRegistry.clear(id),
  clearAll: () => intervalRegistry.clearAll(),
};
