import { useZones } from "./useZones";
import type { ZoneFeature } from "./types";
import { ZONES_PANEL_FIXED } from "../../lib/ui";

type ZonesListProps = {
  onEdit?: (zone: ZoneFeature) => void;
};

export default function ZonesList({ onEdit }: ZonesListProps) {
  const { zones, error } = useZones();

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

      {zones.features.map((f, i) => (
        <div
          key={String(f.id)}
          className={i % 2 === 0 ? "bg-emerald-100 p-2" : "bg-cyan-100 p-2"}
          onDoubleClick={() => onEdit?.(f)}
          role={onEdit ? "button" : undefined}
          tabIndex={onEdit ? 0 : undefined}
          aria-label={onEdit ? `Editar zona ${f.properties.name}` : undefined}
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
      ))}
    </div>
  );
}
