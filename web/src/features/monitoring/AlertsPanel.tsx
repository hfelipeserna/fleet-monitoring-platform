import { useAlertsSSE, type FleetAlert } from "../../hooks/useAlertsSSE";
import { usePortalStore } from "../../store/portalStore";

function translate(alert: FleetAlert): string {
  const plate = alert.plate ?? "";
  switch (alert.alert_type) {
    case "speeding_on":
      return `${plate} superando 80Km/h`;
    case "speeding_off":
      return `${plate} vuelve a <80Km/h`;
    case "zone_enter":
      return `${plate} entra en zona ${alert.zone_name ?? ""}`.trim();
    case "zone_exit":
      return `${plate} sale de zona`;
    default:
      return "Alerta desconocida";
  }
}

export type AlertsPanelProps = {
  hidden?: boolean;
};

export default function AlertsPanel({ hidden: hiddenProp }: AlertsPanelProps) {
  const alerts = useAlertsSSE();
  const activeBottom = usePortalStore((s) => s.activeBottom);
  const hidden = hiddenProp !== undefined ? hiddenProp : activeBottom !== "alerts";

  return (
    <div
      data-testid="alerts-panel"
      className="h-[280px] lg:h-[340px] overflow-y-auto"
      role="log"
      aria-live="polite"
      aria-relevant="additions"
      aria-hidden={hidden ? "true" : undefined}
      hidden={hidden ? true : undefined}
      style={hidden ? { display: "none" } : undefined}
    >
      {alerts.length === 0 ? (
        <p className="p-2 text-sm text-gray-500">Sin alertas</p>
      ) : (
        <ul role="list" className="divide-y divide-gray-100">
          {alerts.map((a, idx) => (
            <li
              key={a.event_id ?? `${a.plate}-${a.created_at}-${idx}`}
              role="listitem"
              className="p-2 text-sm truncate break-words"
            >
              <span className="break-words">{translate(a)}</span>
              {" – "}
              <time dateTime={a.created_at}>
                {new Date(a.created_at).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                  second: "2-digit",
                  hour12: false,
                })}
              </time>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
