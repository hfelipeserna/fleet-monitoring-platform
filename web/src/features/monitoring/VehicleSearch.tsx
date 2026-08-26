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
      className="flex items-start gap-2"
    >
      <div className="flex items-center gap-2">
        <label htmlFor="plate-input" className="text-sm">
          Plate
        </label>
        <input
          id="plate-input"
          aria-label="Plate"
          aria-describedby={showHint ? "plate-hint" : undefined}
          placeholder=""
          value={plate}
          onChange={(e) => setPlate(e.target.value)}
          className="rounded-md border-2 border-black px-2 py-1 text-sm w-28 focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>
      <button
        type="submit"
        disabled={!valid}
        className={`px-4 py-1 text-sm font-medium rounded-md border-2 border-black transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${
          valid ? "bg-blue-800 text-white hover:bg-blue-900" : "bg-white text-gray-400 border-gray-300 cursor-not-allowed"
        }`}
      >
        Search
      </button>
      {showHint ? (
        <span id="plate-hint" role="alert" className="text-xs text-red-600 ml-1">
          3 letras + 3 dígitos
        </span>
      ) : null}
    </form>
  );
}
