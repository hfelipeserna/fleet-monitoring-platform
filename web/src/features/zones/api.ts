import { getApiBase } from "../../lib/api";

export function zoneApiUrl(path: string): string {
  const base = getApiBase();
  return base ? `${base}${path}` : path;
}

export async function parseZoneApiError(res: Response): Promise<string> {
  let msg = "";
  try {
    const data = (await res.json()) as { error?: string; details?: string[]; message?: string };
    if (data?.details && Array.isArray(data.details) && data.details.length) msg = data.details.join(", ");
    else if (data?.message) msg = data.message;
    else if (data?.error) msg = data.error;
    if (data?.error === "validation" && data?.message && msg === data.error) msg = data.message;
    if (msg === "validation" && data?.message) msg = data.message;
  } catch {
    msg = "";
  }
  if (!msg) msg = `Request failed ${res.status}`;
  if (res.status === 409 && !/zone name already exists/i.test(msg)) {
    msg = "zone name already exists";
  }
  return msg;
}
