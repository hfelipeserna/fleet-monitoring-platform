import { useEffect, useState } from "react";
import { MapContainer, TileLayer, Marker, GeoJSON } from "react-leaflet";
import MarkerClusterGroup from "react-leaflet-cluster";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import "leaflet.markercluster/dist/MarkerCluster.css";
import "leaflet.markercluster/dist/MarkerCluster.Default.css";
import { getApiBase } from "../lib/api";

export type Vehicle = {
  plate: string;
  lat: number;
  lon: number;
  speed: number;
};

export type MapProps = {
  vehicles?: Vehicle[];
  zones?: unknown;
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

export default function Map({ vehicles = [], zones }: MapProps) {
  const [geoJson, setGeoJson] = useState<unknown>(zones ?? null);

  useEffect(() => {
    if (zones) setGeoJson(zones);
  }, [zones]);

  useEffect(() => {
    const controller = new AbortController();
    const base = getApiBase();
    const url = base ? `${base}/api/zones` : "/api/zones";
    fetch(url, { signal: controller.signal })
      .then((r) => r.json())
      .then((data) => {
        if (!zones) setGeoJson(data);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [zones]);

  return (
    <MapContainer
      center={[4.71, -74.07]}
      zoom={12}
      style={{ height: "100vh", width: "100%" }}
      data-testid="map"
      className="leaflet-container"
    >
      <TileLayer url={OSM_TILE_URL} attribution='&copy; OpenStreetMap contributors' />

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

      {geoJson ? (
        <GeoJSON data={geoJson as never} style={() => ({ color: "red", fillColor: "red", fillOpacity: 0.2, weight: 2 })} />
      ) : null}
    </MapContainer>
  );
}
