import Map from "./map/Map";
import { useFleetStream } from "./hooks/useFleetStream";
import { useFleetStore } from "./store/fleetStore";
import AlertsPanel from "./features/monitoring/AlertsPanel";
import ChatTab from "./features/monitoring/ChatTab";
import VehicleSearch from "./features/monitoring/VehicleSearch";
import VehicleCard from "./features/monitoring/VehicleCard";

export default function App() {
  const { vehicles, vehicle, selectedPlate } = useFleetStream();
  const setSelectedPlate = useFleetStore((s) => s.setSelectedPlate);
  const notFound = !!selectedPlate && vehicles.length > 0 && vehicle === null;

  function handleClear() {
    setSelectedPlate(null);
  }

  return (
    <div className="flex h-screen w-screen">
      <div className="flex flex-1 flex-col relative">
        <div className="p-2 border-b border-gray-200 bg-white">
          <VehicleSearch />
          {selectedPlate ? (
            <button
              type="button"
              onClick={handleClear}
              aria-label="Clear vehicle info"
              className="mt-2 disabled:opacity-50"
            >
              Clear
            </button>
          ) : null}
        </div>
        <div className="flex-1 relative">
          <Map vehicles={vehicles} selectedVehicle={vehicle} />
        </div>
        <div className="p-2 border-t border-gray-200 bg-white">
          <VehicleCard vehicle={vehicle} notFound={notFound} />
        </div>
        <AlertsPanel />
        <ChatTab />
      </div>
    </div>
  );
}
