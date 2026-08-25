import { useCallback } from "react";
import { useFleetStore } from "../store/fleetStore";
import { PLATE_RE_GLOBAL } from "../lib/plate";

export function usePlateHighlight() {
  const setSelectedPlate = useFleetStore((s) => s.setSelectedPlate);
  const selectedPlate = useFleetStore((s) => s.selectedPlate);

  const highlightFromReply = useCallback(
    (reply: string) => {
      const plates = reply.match(PLATE_RE_GLOBAL);
      if (plates && plates.length > 0) setSelectedPlate(plates[0]!);
    },
    [setSelectedPlate],
  );

  return { selectedPlate, highlightFromReply };
}
