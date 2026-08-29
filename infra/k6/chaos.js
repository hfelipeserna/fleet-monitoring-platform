import http from 'k6/http';
import { check, sleep } from 'k6';

// Chaos variant: batch ingestion + dedup verification.
// Dedup verification (no DB creds needed):
//   curl -s http://localhost:8080/api/fleet/positions?limit=500 | jq length
// Compare before/after k6 run: unique vehicles must not grow with 10% dup.
// DB-level: SELECT count(DISTINCT client_event_id) FROM telemetry;
//          duplicates return 202 but second insert is ON CONFLICT DO NOTHING.
// Invalid 5% must return 400 and never reach NATS stream/consumer.
// JetStream guard: stream TELEMETRY max_bytes 5GB dev; check via
//   curl -s http://localhost:8222/jsz | jq .jetstream

export const options = {
  scenarios: {
    batch: { executor: 'constant-vus', vus: 50, duration: '2m' },
  },
  thresholds: {
    http_req_duration: ['p(95)<250'],
    http_req_failed: ['rate<0.02'],
    checks: ['rate>0.99'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

const lastIds = [];

function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

function randomPlate() {
  return `GTP${100 + Math.floor(Math.random() * 900)}`;
}

function randomLocation() {
  if (Math.random() < 0.5) {
    return { lat: 6.14 + Math.random() * 0.2, lon: -75.68 + Math.random() * 0.2 };
  }
  return { lat: 4.65 + Math.random() * 0.12, lon: -74.12 + Math.random() * 0.1 };
}

function validPayload(id) {
  const loc = randomLocation();
  return {
    plate: randomPlate(),
    speed: Math.floor(Math.random() * 91),
    lat: loc.lat,
    lon: loc.lon,
    client_event_id: id,
    occurred_at: new Date().toISOString(),
  };
}

function invalidPayload() {
  const loc = randomLocation();
  const pick = Math.floor(Math.random() * 7);
  switch (pick) {
    case 0:
      return { plate: 'GTP98', speed: 30, lat: loc.lat, lon: loc.lon, client_event_id: uuidv4(), occurred_at: new Date().toISOString() };
    case 1:
      return { plate: randomPlate(), speed: -1, lat: loc.lat, lon: loc.lon, client_event_id: uuidv4(), occurred_at: new Date().toISOString() };
    case 2:
      return { plate: randomPlate(), speed: 30, lat: 100, lon: 200, client_event_id: uuidv4(), occurred_at: new Date().toISOString() };
    case 3:
      return {};
    case 4:
      return { speed: 30, lat: loc.lat, lon: loc.lon, client_event_id: uuidv4(), occurred_at: new Date().toISOString() };
    case 5:
      return { plate: randomPlate(), speed: 30, lat: loc.lat, client_event_id: uuidv4(), occurred_at: new Date().toISOString() };
    case 6:
      return '{';
    default:
      return {};
  }
}

function buildBatch() {
  const events = [];
  let hasInvalid = false;
  for (let i = 0; i < 10; i++) {
    const r = Math.random();
    if (r < 0.10 && lastIds.length > 0) {
      const dupId = lastIds[Math.floor(Math.random() * lastIds.length)];
      events.push(validPayload(dupId));
    } else if (Math.random() < 0.0526) {
      events.push(invalidPayload());
      hasInvalid = true;
    } else {
      const id = uuidv4();
      if (lastIds.length >= 1000) lastIds.shift();
      lastIds.push(id);
      events.push(validPayload(id));
    }
  }
  return { events, hasInvalid };
}

export default function () {
  const mode = Math.random() < 0.5 ? 'single' : 'batch';

  if (mode === 'batch') {
    const { events, hasInvalid } = buildBatch();
    const batchStr = JSON.stringify(events);
    const res = http.post(`${BASE_URL}/v1/telemetry/batch`, batchStr, {
      headers: { 'Content-Type': 'application/json' },
    });
    check(res, {
      '202 batch valid': (r) => (!hasInvalid ? r.status === 202 : true),
      '400 batch invalid': (r) => (hasInvalid ? r.status === 400 : true),
      'no 500': (r) => r.status !== 500,
    });
  } else {
    const r = Math.random();
    let payloadStr;
    let expectedStatus;
    if (r < 0.10 && lastIds.length > 0) {
      const dupId = lastIds[Math.floor(Math.random() * lastIds.length)];
      payloadStr = JSON.stringify(validPayload(dupId));
      expectedStatus = 202;
    } else if (Math.random() < 0.0526) {
      const p = invalidPayload();
      payloadStr = typeof p === 'string' ? p : JSON.stringify(p);
      expectedStatus = 400;
    } else {
      const id = uuidv4();
      if (lastIds.length >= 1000) lastIds.shift();
      lastIds.push(id);
      payloadStr = JSON.stringify(validPayload(id));
      expectedStatus = 202;
    }
    const res = http.post(`${BASE_URL}/v1/telemetry`, payloadStr, {
      headers: { 'Content-Type': 'application/json' },
    });
    check(res, {
      '202 valid': (r) => (expectedStatus === 202 ? r.status === 202 : true),
      '400 invalid': (r) => (expectedStatus === 400 ? r.status === 400 : true),
      'no 500': (r) => r.status !== 500,
    });
  }

  sleep(0.2);
}
