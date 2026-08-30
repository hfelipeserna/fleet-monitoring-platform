import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    fleet: { executor: 'constant-vus', vus: 300, duration: '5m' },
  },
  thresholds: {
    http_req_duration: ['p(95)<250'],
    http_req_failed: ['rate<0.07'],
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
    return {
      lat: 6.14 + Math.random() * 0.2,
      lon: -75.68 + Math.random() * 0.2,
    };
  }
  return {
    lat: 4.65 + Math.random() * 0.12,
    lon: -74.12 + Math.random() * 0.1,
  };
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

export default function () {
  const r = Math.random();
  let payloadStr;
  let expectedStatus;

  if (r < 0.10 && lastIds.length > 0) {
    const dupId = lastIds[Math.floor(Math.random() * lastIds.length)];
    const payload = validPayload(dupId);
    payloadStr = JSON.stringify(payload);
    expectedStatus = 202;
  } else if (Math.random() < 0.0526) {
    const payload = invalidPayload();
    if (typeof payload === 'string') {
      payloadStr = payload;
    } else {
      payloadStr = JSON.stringify(payload);
    }
    expectedStatus = 400;
  } else {
    const id = uuidv4();
    if (lastIds.length >= 1000) lastIds.shift();
    lastIds.push(id);
    const payload = validPayload(id);
    payloadStr = JSON.stringify(payload);
    expectedStatus = 202;
  }

  const res = http.post(`${BASE_URL}/v1/telemetry`, payloadStr, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, {
    '202 valid': (res) => (expectedStatus === 202 ? res.status === 202 : true),
    '400 invalid': (res) => (expectedStatus === 400 ? res.status === 400 : true),
    'no 500': (res) => res.status !== 500,
  });

  sleep(0.2);
}
