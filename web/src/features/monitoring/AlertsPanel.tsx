import { useAlertsSSE, type FleetAlert } from "../../hooks/useAlertsSSE";
import { BottomPanelShell } from "./BottomPanelShell";

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

export default function AlertsPanel({ hidden }: AlertsPanelProps) {
  const alerts = useAlertsSSE();

  return (
    <BottomPanelShell activeKey="alerts" testId="alerts-panel" asLog hidden={hidden}>
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
    </BottomPanelShell>
  );
}
