import { useZones } from "./useZones";
import type { ZoneFeature, ZonesFC } from "./types";
import { ZONES_PANEL_FIXED } from "../../lib/ui";

type ZonesListProps = {
  zones?: ZonesFC;
  error?: string | null;
  onEdit?: (zone: ZoneFeature) => void;
  onSelect?: (id: string | null) => void;
  selectedId?: string | null;
};

export default function ZonesList({ zones: zonesProp, error: errorProp, onEdit, onSelect, selectedId }: ZonesListProps) {
  const fallback = useZones();
  const zones = zonesProp ?? fallback.zones;
  const error = errorProp !== undefined ? errorProp : fallback.error;

  if (error) {
    return (
      <div
        data-testid="zones-list"
        role="alert"
        aria-live="assertive"
        className={`flex-1 ${ZONES_PANEL_FIXED}`}
      >
        <p className="p-2 text-sm text-red-600">Error loading zones: {error}</p>
      </div>
    );
  }

  return (
    <div data-testid="zones-list" aria-live="polite" className={`flex-1 ${ZONES_PANEL_FIXED}`}>

      {zones.features.map((f, i) => {
        const isSelected = selectedId != null && String(f.id) === selectedId;
        const base = i % 2 === 0 ? "bg-emerald-100" : "bg-cyan-100";
        const selectedCls = isSelected ? " bg-blue-100 border-y-2 border-blue-500" : " border-y-2 border-transparent";
        return (
          <div
            key={String(f.id)}
            className={`${base} p-2 cursor-pointer hover:bg-gray-100${selectedCls}`}
            onClick={() => onSelect?.(String(f.id))}
            onDoubleClick={() => onEdit?.(f)}
            role={onEdit || onSelect ? "button" : undefined}
            tabIndex={onEdit || onSelect ? 0 : undefined}
            aria-label={onEdit ? `Editar zona ${f.properties.name}` : f.properties.name}
            aria-selected={isSelected ? "true" : undefined}
            onKeyDown={
              onEdit
                ? (e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onEdit(f);
                    }
                  }
                : undefined
            }
          >
            {f.properties.name}
          </div>
        );
      })}
    </div>
  );
}
