export const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function makePoint(idx: number, plate = 'TGY589') {
  return {
    plate,
    lat: 6.2442 + idx * 0.001,
    lon: -75.5812 + idx * 0.001,
    speed: [0, 45, 85][idx % 3],
    occurred_at: Date.now() + idx,
  };
}

export async function enqueueMany(telemetry: any, n: number, plate = 'TGY589'): Promise<void> {
  for (let i = 0; i < n; i++) {
    await telemetry.enqueue(makePoint(i, plate));
  }
}
