import { useEffect, useState } from "react";
import { usePortalStore } from "../../store/portalStore";
import { useDialogFocus } from "../../lib/useDialogFocus";
import { parseZoneApiError, zoneApiUrl } from "./api";
import type { DraftPolygon } from "./types";

type CreateZoneModalProps = {
  open: boolean;
  draft: DraftPolygon | null;
  onClose: () => void;
  onCreated: () => void;
};

export default function CreateZoneModal({ open, draft, onClose, onCreated }: CreateZoneModalProps) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const setDraftPolygon = usePortalStore((s) => s.setDraftPolygon);
  const dialogRef = useDialogFocus(open, onClose);

  useEffect(() => {
    if (open) {
      setName("");
      setError("");
    }
  }, [open]);

  if (!open) return null;

  const isAcceptDisabled = !name.trim() || !draft;

  async function handleAccept() {
    if (!name.trim()) {
      setError("Zone name required");
      return;
    }
    if (!draft) return;
    const geojson = { type: "Polygon", coordinates: draft.coordinates };
    const url = zoneApiUrl("/api/zones");
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: name.trim(), geojson }),
      });
      if (res.ok) {
        setError("");
        setDraftPolygon(null);
        onCreated();
        onClose();
        return;
      }
      const msg = await parseZoneApiError(res);
      setError(msg);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function handleCancel() {
    setError("");
    setDraftPolygon(null);
    onClose();
  }

  function handleOverlayClick(e: React.MouseEvent<HTMLDivElement>) {
    if (e.target === e.currentTarget) onClose();
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-[1000] p-4" onClick={handleOverlayClick}>
      <div
        ref={dialogRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="zone-name-label"
        className="bg-white rounded-2xl border-2 border-black shadow-xl w-full max-w-[420px] p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <label id="zone-name-label" htmlFor="zone-name" className="block text-left text-sm font-normal mb-2">
          Zone name
        </label>
        <input
          id="zone-name"
          name="name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          aria-invalid={!!error}
          aria-describedby={error ? "zone-name-error" : undefined}
          autoFocus
          className="w-full rounded-lg border-2 border-black px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        />
        {error ? (
          <span id="zone-name-error" role="alert" className="mt-1 block text-sm text-red-600">
            {error}
          </span>
        ) : null}
        <div className="flex justify-center gap-3 mt-6">
          <button
            type="button"
            onClick={handleAccept}
            disabled={isAcceptDisabled}
            className="px-5 py-2 rounded-lg bg-blue-800 border-2 border-black text-white text-sm font-medium hover:bg-blue-900 disabled:bg-gray-100 disabled:text-gray-400 disabled:border-gray-300 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            Accept
          </button>
          <button
            type="button"
            onClick={handleCancel}
            className="px-5 py-2 rounded-lg bg-pink-100 border-2 border-black text-black text-sm font-medium hover:bg-pink-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pink-300"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
