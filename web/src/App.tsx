import Map from "./map/Map";
import { useFleetStream } from "./hooks/useFleetStream";
import { useFleetStore } from "./store/fleetStore";
import { usePortalStore, type ActiveTop } from "./store/portalStore";
import AlertsPanel from "./features/monitoring/AlertsPanel";
import ChatTab from "./features/monitoring/ChatTab";
import VehicleSearch from "./features/monitoring/VehicleSearch";
import VehicleCard from "./features/monitoring/VehicleCard";
import ZonesList from "./features/zones/ZonesList";
import { PANEL_HEIGHT, getTabClass } from "./lib/ui";

function TopTabs({ activeTop, onChange }: { activeTop: ActiveTop; onChange: (v: ActiveTop) => void }) {
  return (
    <div role="tablist" aria-label="Vistas principales" className="flex gap-2 mt-2">
      <button
        type="button"
        role="tab"
        id="tab-monitoring"
        aria-controls="panel-monitoring"
        aria-selected={activeTop === "monitoring"}
        tabIndex={activeTop === "monitoring" ? 0 : -1}
        onClick={() => onChange("monitoring")}
        className={getTabClass(activeTop === "monitoring")}
      >
        Monitoring
      </button>
      <button
        type="button"
        role="tab"
        id="tab-zones"
        aria-controls="panel-zones"
        aria-selected={activeTop === "zones"}
        tabIndex={activeTop === "zones" ? 0 : -1}
        onClick={() => onChange("zones")}
        className={getTabClass(activeTop === "zones")}
      >
        Critical zones
      </button>
    </div>
  );
}

function MonitoringLeft({
  vehicle,
  notFound,
  selectedPlate,
  onClear,
}: {
  vehicle: ReturnType<typeof useFleetStream>["vehicle"];
  notFound: boolean;
  selectedPlate: string | null;
  onClear: () => void;
}) {
  return (
    <div>
      <VehicleSearch />
      {selectedPlate ? (
        <button type="button" onClick={onClear} aria-label="Clear vehicle info" className="mt-2 disabled:opacity-50">
          Clear
        </button>
      ) : null}
      <div className="mt-2">
        <VehicleCard vehicle={vehicle} notFound={notFound} />
      </div>
    </div>
  );
}

function MonitoringBottom({ activeBottom }: { activeBottom: string }) {
  return (
    <div className="border-t border-gray-200 bg-white">
      <AlertsPanel hidden={activeBottom !== "alerts"} />
      <ChatTab hidden={activeBottom !== "chat"} />
    </div>
  );
}

export default function App() {
  const { vehicles, vehicle, selectedPlate } = useFleetStream();
  const setSelectedPlate = useFleetStore((s) => s.setSelectedPlate);
  const activeTop = usePortalStore((s) => s.activeTop);
  const activeBottom = usePortalStore((s) => s.activeBottom);
  const setActiveTop = usePortalStore((s) => s.setActiveTop);
  const notFound = !!selectedPlate && vehicles.length > 0 && vehicle === null;

  function handleClear() {
    setSelectedPlate(null);
  }

  return (
    <div className="flex h-screen w-screen flex-col">
      <header className="p-2 border-b border-gray-200 bg-white">
        <h1 className="text-blue-700 font-bold">FLEET MONITORING PLATFORM</h1>
        <TopTabs activeTop={activeTop} onChange={setActiveTop} />
      </header>

      <div
        className={`grid gap-4 flex-1 p-2 min-h-0 ${activeTop === "monitoring" ? "lg:grid-cols-2" : "lg:grid-cols-[35%_65%]"}`}
      >
        <div
          hidden={activeTop !== "monitoring"}
          aria-hidden={activeTop !== "monitoring" ? "true" : undefined}
          id="panel-monitoring"
          role="tabpanel"
          aria-labelledby="tab-monitoring"
          className={activeTop !== "monitoring" ? "hidden" : ""}
        >
          <MonitoringLeft vehicle={vehicle} notFound={notFound} selectedPlate={selectedPlate} onClear={handleClear} />
        </div>
        <div
          hidden={activeTop !== "zones"}
          aria-hidden={activeTop !== "zones" ? "true" : undefined}
          id="panel-zones"
          role="tabpanel"
          aria-labelledby="tab-zones"
          className={activeTop !== "zones" ? "hidden" : `${PANEL_HEIGHT} flex flex-col`}
        >
          <ZonesList />
          <button type="button" className="mt-2 px-3 py-1 border border-black">
            Create zone
          </button>
        </div>
        <div className="flex-1 relative min-h-[300px]">
          <Map vehicles={vehicles} selectedVehicle={vehicle} />
        </div>
      </div>

      <div
        hidden={activeTop !== "monitoring"}
        aria-hidden={activeTop !== "monitoring" ? "true" : undefined}
        className={activeTop !== "monitoring" ? "hidden" : ""}
      >
        <MonitoringBottom activeBottom={activeBottom} />
      </div>
    </div>
  );
}
