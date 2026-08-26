import { useEffect } from "react";
import { MapContainer, TileLayer, Marker, GeoJSON, useMap } from "react-leaflet";
import { usePortalStore } from "../store/portalStore";
import MarkerClusterGroup from "react-leaflet-cluster";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import "leaflet.markercluster/dist/MarkerCluster.css";
import "leaflet.markercluster/dist/MarkerCluster.Default.css";
import type { ZonesFC } from "../features/zones/types";

export type Vehicle = {
  plate: string;
  lat: number | null;
  lon: number | null;
  speed: number;
};

export type MapProps = {
  vehicles?: Vehicle[];
  zones?: ZonesFC | null;
  selectedVehicle?: Vehicle | null;
  selectedZoneId?: string | null;
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
      const currentZoom = map.getZoom();
      const targetZoom = Number.isFinite(currentZoom) ? currentZoom : 14;
      if (map.getCenter().distanceTo([vehicle.lat, vehicle.lon] as never) < 1) return;
      map.setView([vehicle.lat, vehicle.lon], targetZoom, { animate: true });
    }
  }, [vehicle, map]);
  return null;
}

function InvalidateSize() {
  const map = useMap() as unknown as { invalidateSize?: () => void };
  const activeTop = usePortalStore((s) => s.activeTop);
  useEffect(() => {
    const safe = () => {
      try {
        map.invalidateSize?.();
      } catch {
        // ignore in jsdom
      }
    };
    const t1 = window.setTimeout(safe, 80);
    const t2 = window.setTimeout(safe, 300);
    const t3 = window.setTimeout(safe, 600);
    const onResize = safe;
    window.addEventListener("resize", onResize);
    return () => {
      window.clearTimeout(t1);
      window.clearTimeout(t2);
      window.clearTimeout(t3);
      window.removeEventListener("resize", onResize);
    };
  }, [map, activeTop]);
  return null;
}

export default function Map({ vehicles = [], zones, selectedVehicle, selectedZoneId = null, children }: MapProps) {
  const markers = vehicles.filter((v) => v.lat != null && v.lon != null);
  const displayZones: ZonesFC | null = (() => {
    if (!zones) return null;
    if (!selectedZoneId) return zones;
    const filtered = zones.features.filter((f) => String(f.id) === selectedZoneId);
    return { type: "FeatureCollection", features: filtered };
  })();
  return (
    <MapContainer
      center={[4.71, -74.07]}
      zoom={12}
      className="leaflet-container h-full w-full"
      data-testid="map"
    >
      <TileLayer url={OSM_TILE_URL} attribution='&copy; OpenStreetMap contributors' keepBuffer={2} updateWhenIdle={false} updateWhenZooming />
      <Recenter vehicle={selectedVehicle} />
      <InvalidateSize />
      {children}
      {markers.length > 500 ? (
        <MarkerClusterGroup chunkedLoading>
          {markers.map((v) => (
            <Marker key={v.plate} position={[v.lat as number, v.lon as number]} title={v.plate} alt={`vehicle ${v.plate}`} />
          ))}
        </MarkerClusterGroup>
      ) : (
        markers.map((v) => (
          <Marker key={v.plate} position={[v.lat as number, v.lon as number]} title={v.plate} alt={`vehicle ${v.plate}`} />
        ))
      )}
      {displayZones ? (
        <GeoJSON
          key={selectedZoneId ?? "all"}
          data={displayZones as never}
          style={() => ({ color: "red", fillColor: "red", fillOpacity: 0.2, weight: 2 })}
        />
      ) : null}
    </MapContainer>
  );
}
