export function makePersistPoint(idx: number, plate = 'TGY589') {
  return {
    plate,
    lat: 6.2442 + idx * 0.00001,
    lon: -75.5812 + idx * 0.00001,
    speed: [0, 45, 85][idx % 3],
    occurred_at: Date.now() + idx,
  };
}

export function uuidRegex() {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
}
