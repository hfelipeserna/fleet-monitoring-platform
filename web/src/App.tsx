import { useState } from "react";
import Map from "./map/Map";
import { useFleetStream } from "./hooks/useFleetStream";
import { useFleetStore } from "./store/fleetStore";
import { usePortalStore, type ActiveTop } from "./store/portalStore";
import AlertsPanel from "./features/monitoring/AlertsPanel";
import ChatTab from "./features/monitoring/ChatTab";
import VehicleSearch from "./features/monitoring/VehicleSearch";
import VehicleCard from "./features/monitoring/VehicleCard";
import ZonesList from "./features/zones/ZonesList";
import ZoneDrawControl from "./features/zones/ZoneDrawControl";
import CreateZoneModal from "./features/zones/CreateZoneModal";
import EditZoneModal from "./features/zones/EditZoneModal";
import { useZones } from "./features/zones/useZones";
import { ZONES_PANEL_FIXED, getTabClass } from "./lib/ui";
import type { ZoneFeature } from "./features/zones/types";

function TopTabs({ activeTop, onChange }: { activeTop: ActiveTop; onChange: (v: ActiveTop) => void }) {
  function handleKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    const tabs = Array.from(e.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
    const idx = tabs.indexOf(document.activeElement as HTMLButtonElement);
    if (e.key === "ArrowRight") {
      e.preventDefault();
      const next = idx >= 0 ? (idx + 1) % tabs.length : 0;
      tabs[next]?.focus();
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      const prev = idx >= 0 ? (idx - 1 + tabs.length) % tabs.length : tabs.length - 1;
      tabs[prev]?.focus();
    } else if (e.key === "Home") {
      e.preventDefault();
      tabs[0]?.focus();
    } else if (e.key === "End") {
      e.preventDefault();
      tabs[tabs.length - 1]?.focus();
    }
  }

  return (
    <div role="tablist" aria-label="Vistas principales" className="flex gap-2 mt-2" onKeyDown={handleKeyDown}>
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
        <button
          type="button"
          onClick={onClear}
          aria-label="Clear vehicle info"
          className="mt-2 bg-emerald-300 px-3 py-1 disabled:opacity-50"
        >
          Clear vehicle info
        </button>
      ) : null}
      <div className="mt-2">
        <VehicleCard vehicle={vehicle} notFound={notFound} />
      </div>
    </div>
  );
}

function BottomTabs({
  activeBottom,
  onChange,
}: {
  activeBottom: string;
  onChange: (v: "alerts" | "chat") => void;
}) {
  return (
    <div role="tablist" aria-label="Alertas y Chat" className="flex justify-center gap-2 mt-3 mb-2">
      <button
        type="button"
        role="tab"
        id="tab-alerts"
        aria-controls="panel-alerts"
        aria-selected={activeBottom === "alerts"}
        onClick={() => onChange("alerts")}
        className={`px-6 py-1.5 text-sm font-medium rounded-lg border-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400 ${
          activeBottom === "alerts" ? "bg-cyan-200 border-black text-black" : "bg-white border-black text-black hover:bg-gray-50"
        }`}
      >
        Alerts
      </button>
      <button
        type="button"
        role="tab"
        id="tab-chat"
        aria-controls="panel-chat"
        aria-selected={activeBottom === "chat"}
        onClick={() => onChange("chat")}
        className={`px-6 py-1.5 text-sm font-medium rounded-lg border-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400 ${
          activeBottom === "chat" ? "bg-cyan-200 border-black text-black" : "bg-white border-black text-black hover:bg-gray-50"
        }`}
      >
        Chat AI
      </button>
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
  const { zones, error: zonesError, refetch: refetchZones } = useZones();
  const setSelectedPlate = useFleetStore((s) => s.setSelectedPlate);
  const activeTop = usePortalStore((s) => s.activeTop);
  const activeBottom = usePortalStore((s) => s.activeBottom);
  const setActiveTop = usePortalStore((s) => s.setActiveTop);
  const setActiveBottom = usePortalStore((s) => s.setActiveBottom);
  const draftPolygon = usePortalStore((s) => s.draftPolygon);
  const setDraftPolygon = usePortalStore((s) => s.setDraftPolygon);
  const selectedZoneId = usePortalStore((s) => s.selectedZoneId);
  const setSelectedZoneId = usePortalStore((s) => s.setSelectedZoneId);
  const [createOpen, setCreateOpen] = useState(false);
  const [editZone, setEditZone] = useState<ZoneFeature | null>(null);
  const notFound = !!selectedPlate && vehicles.length > 0 && vehicle === null;
  const displayVehicles = selectedPlate ? (vehicle ? [vehicle] : vehicles) : vehicles;
  const displayZones =
    activeTop !== "zones"
      ? null
      : selectedZoneId
        ? { type: "FeatureCollection" as const, features: zones.features.filter((f) => String(f.id) === selectedZoneId) }
        : zones;

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
          className={activeTop !== "zones" ? "hidden" : `${ZONES_PANEL_FIXED} flex flex-col`}
        >
          <ZonesList zones={zones} error={zonesError} onEdit={(z) => setEditZone(z)} onSelect={(id) => setSelectedZoneId(id)} selectedId={selectedZoneId} />
          <button
            type="button"
            onClick={() => setCreateOpen(true)}
            disabled={draftPolygon == null}
            aria-disabled={draftPolygon == null}
            className={`mt-2 w-full rounded px-4 py-2 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-800 focus-visible:ring-offset-2 ${
              draftPolygon
                ? "bg-blue-800 text-white hover:bg-blue-900 border border-blue-900"
                : "bg-white text-gray-400 border border-gray-300 cursor-not-allowed opacity-60"
            }`}
          >
            Create zone
          </button>
        </div>
        <div className="flex-1 relative min-h-[300px]">
          <Map vehicles={displayVehicles} selectedVehicle={vehicle} zones={displayZones} selectedZoneId={selectedZoneId}>
            {activeTop === "zones" ? <ZoneDrawControl /> : null}
          </Map>
        </div>
      </div>

      <div
        hidden={activeTop !== "monitoring"}
        aria-hidden={activeTop !== "monitoring" ? "true" : undefined}
        className={activeTop !== "monitoring" ? "hidden" : "flex flex-col"}
      >
        <BottomTabs activeBottom={activeBottom} onChange={setActiveBottom} />
        <div className="flex-1 min-h-0">
          <MonitoringBottom activeBottom={activeBottom} />
        </div>
      </div>

      <CreateZoneModal
        open={createOpen}
        draft={draftPolygon}
        onClose={() => {
          setCreateOpen(false);
          setDraftPolygon(null);
        }}
        onCreated={() => {
          refetchZones();
          setCreateOpen(false);
        }}
      />
      <EditZoneModal
        open={!!editZone}
        zone={editZone}
        onClose={() => setEditZone(null)}
        onRenamed={() => {
          refetchZones();
          setEditZone(null);
        }}
        onDeleted={() => {
          refetchZones();
          setEditZone(null);
        }}
      />
    </div>
  );
}
