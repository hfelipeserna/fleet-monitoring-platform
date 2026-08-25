import { useCallback } from "react";
import { useFleetStore } from "../store/fleetStore";

const PLATE_RE = /\b[A-Z]{3}[0-9]{3}\b/g;

export function usePlateHighlight() {
  const setSelectedPlate = useFleetStore((s) => s.setSelectedPlate);
  const selectedPlate = useFleetStore((s) => s.selectedPlate);

  const highlightFromReply = useCallback(
    (reply: string) => {
      const plates = reply.match(PLATE_RE);
      if (plates && plates.length > 0) setSelectedPlate(plates[0]!);
    },
    [setSelectedPlate],
  );

  return { selectedPlate, highlightFromReply };
}
