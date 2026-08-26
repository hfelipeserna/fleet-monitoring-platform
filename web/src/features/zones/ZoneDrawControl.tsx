import { useEffect, useRef } from "react";
import { useMap } from "react-leaflet";
import { usePortalStore } from "../../store/portalStore";
import { validatePolygon, type DraftPolygon } from "./types";
import "@geoman-io/leaflet-geoman-free";
import "@geoman-io/leaflet-geoman-free/dist/leaflet-geoman.css";

type Props = {
  onDraftChange?: (v: DraftPolygon | null) => void;
};

type PmMap = ReturnType<typeof useMap> & {
  pm?: {
    addControls: (o: Record<string, unknown>) => void;
    removeControls: () => void;
  };
  on: (e: string, cb: (ev: unknown) => void) => void;
  off: (e: string, cb: (ev: unknown) => void) => void;
  removeLayer?: (layer: unknown) => void;
};

type PmCreateEvent = {
  shape?: string;
  layer: { toGeoJSON: () => { type: string; geometry?: DraftPolygon; properties: unknown } & Partial<DraftPolygon> };
};

export default function ZoneDrawControl({ onDraftChange }: Props) {
  const map = useMap() as PmMap;
  const setDraftPolygon = usePortalStore((s) => s.setDraftPolygon);
  const draftLayerRef = useRef<unknown>(null);

  const draftPolygon = usePortalStore((s) => s.draftPolygon);

  useEffect(() => {
    if (!draftPolygon && draftLayerRef.current) {
      try {
        if (map.removeLayer) map.removeLayer(draftLayerRef.current);
        else if ((draftLayerRef.current as unknown as { remove?: () => void }).remove)
          (draftLayerRef.current as unknown as { remove: () => void }).remove();
      } catch {
        // ignore
      }
      draftLayerRef.current = null;
    }
  }, [draftPolygon, map]);

  useEffect(() => {
    if (!map || !map.pm) return;
    map.pm.addControls({
      position: "topleft",
      drawPolygon: true,
      drawMarker: false,
      drawPolyline: false,
      drawRectangle: false,
      drawCircle: false,
      drawCircleMarker: false,
      editMode: false,
      dragMode: false,
      cutPolygon: false,
      removalMode: true,
    });

    const handleCreate = (e: unknown) => {
      const ev = e as PmCreateEvent;
      let geom: DraftPolygon | null = null;
      try {
        const gj = ev.layer.toGeoJSON();
        const maybeGeom = (gj as { geometry?: DraftPolygon }).geometry;
        if (maybeGeom && maybeGeom.type === "Polygon" && Array.isArray(maybeGeom.coordinates)) {
          geom = maybeGeom;
        } else {
          const alt = gj as unknown as DraftPolygon;
          if (alt.type === "Polygon" && Array.isArray(alt.coordinates)) geom = alt;
        }
      } catch {
        geom = null;
      }
      let draft: DraftPolygon | null = null;
      if (geom && geom.type === "Polygon" && Array.isArray(geom.coordinates) && Array.isArray(geom.coordinates[0])) {
        const c = geom.coordinates[0] as unknown as number[][];
        const result = validatePolygon(c);
        if (result.valid) draft = { type: "Polygon", coordinates: [c] };
      }
      if (draft) {
        draftLayerRef.current = ev.layer;
      } else {
        // invalid: remove visual layer so it doesn't remain as phantom draft
        try {
          if (map.removeLayer) map.removeLayer(ev.layer);
          else if ((ev.layer as unknown as { remove?: () => void }).remove) (ev.layer as unknown as { remove: () => void }).remove();
        } catch {
          // ignore
        }
        draftLayerRef.current = null;
      }
      setDraftPolygon(draft);
      if (onDraftChange) onDraftChange(draft);
    };

    const handleRemove = () => {
      draftLayerRef.current = null;
      setDraftPolygon(null);
      if (onDraftChange) onDraftChange(null);
    };

    const handleMapClick = (e: unknown) => {
      const stateDraft = usePortalStore.getState().draftPolygon;
      if (!stateDraft) return;
      if (!draftLayerRef.current) return;
      const ev = e as { latlng?: { lat: number; lng: number } };
      if (!ev.latlng) return;
      const layer = draftLayerRef.current as unknown as {
        getBounds?: () => { contains: (ll: unknown) => boolean };
        getLatLngs?: () => unknown;
      };
      let inside = false;
      try {
        if (layer.getBounds) {
          const bounds = layer.getBounds();
          if (bounds && bounds.contains) {
            inside = bounds.contains(ev.latlng as unknown as never);
          }
        }
      } catch {
        inside = false;
      }
      if (!inside) {
        try {
          if (map.removeLayer) map.removeLayer(draftLayerRef.current);
          else if ((draftLayerRef.current as unknown as { remove?: () => void }).remove)
            (draftLayerRef.current as unknown as { remove: () => void }).remove();
        } catch {
          // ignore
        }
        draftLayerRef.current = null;
        setDraftPolygon(null);
        if (onDraftChange) onDraftChange(null);
      }
    };

    map.on("pm:create", handleCreate as (ev: unknown) => void);
    map.on("pm:remove", handleRemove as (ev: unknown) => void);
    map.on("click", handleMapClick as (ev: unknown) => void);

    return () => {
      try {
        map.off("pm:create", handleCreate as (ev: unknown) => void);
        map.off("pm:remove", handleRemove as (ev: unknown) => void);
        map.off("click", handleMapClick as (ev: unknown) => void);
        map.pm?.removeControls();
      } catch {
        // ignore
      }
    };
  }, [map, onDraftChange, setDraftPolygon]);

  return null;
}
