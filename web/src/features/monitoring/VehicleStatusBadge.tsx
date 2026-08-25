type Props = {
  status: "moving" | "idle";
};

export default function VehicleStatusBadge({ status }: Props) {
  const isMoving = status === "moving";

  return (
    <span
      className={isMoving ? "font-semibold px-1.5 py-0.5 rounded bg-green-50" : "font-semibold"}
      style={{ color: isMoving ? "#16a34a" : "#dc2626" }}
    >
      {isMoving ? "Moving" : "Idle"}
    </span>
  );
}
