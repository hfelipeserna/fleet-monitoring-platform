import { useEffect } from "react";
import { MapContainer, TileLayer, Marker, GeoJSON, useMap } from "react-leaflet";
import MarkerClusterGroup from "react-leaflet-cluster";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import "leaflet.markercluster/dist/MarkerCluster.css";
import "leaflet.markercluster/dist/MarkerCluster.Default.css";
import type { ZonesFC } from "../features/zones/types";

export type Vehicle = {
  plate: string;
  lat: number;
  lon: number;
  speed: number;
};

export type MapProps = {
  vehicles?: Vehicle[];
  zones?: ZonesFC | null;
  selectedVehicle?: Vehicle | null;
  children?: React.ReactNode;
};

const OSM_TILE_URL = "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png";

if (typeof window !== "undefined") {
  (L.Browser as unknown as Record<string, unknown>).svg = true;
  (L.Browser as unknown as Record<string, unknown>).inlineSvg = true;
}
delete (L.Icon.Default.prototype as unknown as { _getIconUrl?: unknown })._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png",
  iconUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png",
  shadowUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png",
});

function Recenter({ vehicle }: { vehicle?: Vehicle | null }) {
  const map = useMap();
  useEffect(() => {
    if (vehicle && typeof vehicle.lat === "number" && typeof vehicle.lon === "number") {
      map.setView([vehicle.lat, vehicle.lon], 14);
    }
  }, [vehicle, map]);
  return null;
}

export default function Map({ vehicles = [], zones, selectedVehicle, children }: MapProps) {
  return (
    <MapContainer
      center={[4.71, -74.07]}
      zoom={12}
      className="leaflet-container h-full w-full"
      data-testid="map"
    >
      <TileLayer url={OSM_TILE_URL} attribution='&copy; OpenStreetMap contributors' />
      <Recenter vehicle={selectedVehicle} />
      {children}
      {vehicles.length > 500 ? (
        <MarkerClusterGroup chunkedLoading>
          {vehicles.map((v) => (
            <Marker key={v.plate} position={[v.lat, v.lon]} title={v.plate} alt={`vehicle ${v.plate}`} />
          ))}
        </MarkerClusterGroup>
      ) : (
        vehicles.map((v) => (
          <Marker key={v.plate} position={[v.lat, v.lon]} title={v.plate} alt={`vehicle ${v.plate}`} />
        ))
      )}
      {zones ? (
        <GeoJSON data={zones as never} style={() => ({ color: "red", fillColor: "red", fillOpacity: 0.2, weight: 2 })} />
      ) : null}
    </MapContainer>
  );
}
