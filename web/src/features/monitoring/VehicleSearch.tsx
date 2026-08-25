import { useState } from "react";
import { isValidPlate } from "../../lib/plate";
import { useFleetStore } from "../../store/fleetStore";

type Props = {
  onSearch?: (plate: string) => void;
};

export default function VehicleSearch({ onSearch }: Props) {
  const [plate, setPlate] = useState("");
  const setSelectedPlate = useFleetStore((s) => s.setSelectedPlate);
  const normalized = plate.trim().toUpperCase();
  const valid = isValidPlate(normalized);
  const showHint = plate.length > 0 && !valid;

  function handleSearch() {
    if (!valid) return;
    if (onSearch) onSearch(normalized);
    setSelectedPlate(normalized);
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        handleSearch();
      }}
    >
      <input
        aria-label="Plate"
        aria-describedby={showHint ? "plate-hint" : undefined}
        placeholder="Plate"
        value={plate}
        onChange={(e) => setPlate(e.target.value)}
      />
      <button type="submit" disabled={!valid} className="disabled:opacity-50">
        Search
      </button>
      {showHint ? (
        <span id="plate-hint" role="alert">
          3 letras + 3 dígitos
        </span>
      ) : null}
    </form>
  );
}
