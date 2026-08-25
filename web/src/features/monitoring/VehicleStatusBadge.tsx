type Props = {
  speed?: number;
  status?: "moving" | "idle";
};

export default function VehicleStatusBadge({ speed, status }: Props) {
  const derived: "moving" | "idle" =
    status ?? (speed !== undefined && speed > 0 ? "moving" : "idle");
  const color = derived === "moving" ? "#16a34a" : "#dc2626";
  const label = derived === "moving" ? "Moving" : "Idle";
  const isMoving = derived === "moving";

  return (
    <span className={isMoving ? "font-semibold px-1.5 py-0.5 rounded bg-green-50" : undefined}>
      <span style={{ color }}>{label}</span>
    </span>
  );
}
