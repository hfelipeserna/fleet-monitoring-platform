import VehicleStatusBadge from "./VehicleStatusBadge";

type Vehicle = {
  plate: string;
  lat: number | null;
  lon: number | null;
  speed: number;
  received_at: string;
};

type Props = {
  vehicle?: Vehicle | null;
  notFound?: boolean;
};

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: "UTC",
  });
}

export default function VehicleCard({ vehicle, notFound }: Props) {
  const isNotFound = notFound || vehicle == null;

  if (isNotFound) {
    return (
      <div className="min-h-[120px]">
        <p>placa no encontrada</p>
      </div>
    );
  }

  const data = vehicle!;
  const latText = data.lat == null ? "—" : data.lat.toFixed(6);
  const lonText = data.lon == null ? "—" : data.lon.toFixed(6);
  const timeText = formatTime(data.received_at);

  return (
    <div className="min-h-[120px]">
      <div>Plate: {data.plate}</div>
      <div>Latitude: {latText}</div>
      <div>Longitude: {lonText}</div>
      <div>
        Speed: {data.speed} {data.speed > 80 ? <span aria-label="speed alert">⚠️</span> : null}
      </div>
      <div>
        Status: <VehicleStatusBadge status={data.speed > 0 ? "moving" : "idle"} />
      </div>
      <div>Last update: {timeText}</div>
    </div>
  );
}
