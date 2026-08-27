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
    if (this.ids.size === 0) {
      clearInterval(0 as unknown as number);
    }
    for (const id of this.ids) clearInterval(id);
    this.ids.clear();
  }
  getIds(): number[] {
    return [...this.ids];
  }
  reset(): void {
    this.ids.clear();
  }
}

export const intervalRegistry = new IntervalRegistry();

export const intervalPort = {
  register: (id: number) => intervalRegistry.register(id),
  clear: (id: number) => intervalRegistry.clear(id),
  clearAll: () => intervalRegistry.clearAll(),
};
