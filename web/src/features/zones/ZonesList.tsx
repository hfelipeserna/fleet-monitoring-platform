import { useZones } from "./useZones";
import type { ZoneFeature } from "./types";

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
        className="flex-1 overflow-y-auto h-[360px] lg:h-[480px]"
      >
        <p className="p-2 text-sm text-red-600">Error loading zones: {error}</p>
      </div>
    );
  }

  return (
    <div
      data-testid="zones-list"
      aria-live="polite"
      className="flex-1 overflow-y-auto h-[360px] lg:h-[480px]"
    >
      {zones.features.map((f, i) => (
        <div
          key={String(f.id)}
          className={i % 2 === 0 ? "bg-emerald-100 p-2" : "bg-cyan-100 p-2"}
          onDoubleClick={() => onEdit?.(f)}
          role={onEdit ? "button" : undefined}
          tabIndex={onEdit ? 0 : undefined}
          onKeyDown={
            onEdit
              ? (e) => {
                  if (e.key === "Enter") onEdit(f);
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
