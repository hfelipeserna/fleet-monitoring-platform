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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-[1000]" onClick={handleOverlayClick}>
      <div
        ref={dialogRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="zone-name-label"
        className="bg-white p-4 rounded shadow-lg min-w-[300px]"
        onClick={(e) => e.stopPropagation()}
      >
        <label id="zone-name-label" htmlFor="zone-name">
          Zone name
        </label>
        <input
          id="zone-name"
          name="name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          aria-invalid={!!error}
          aria-describedby={error ? "zone-name-error" : undefined}
        />
        {error ? (
          <span id="zone-name-error" role="alert">
            {error}
          </span>
        ) : null}
        <div className="flex gap-2 mt-2">
          <button type="button" onClick={handleAccept} disabled={isAcceptDisabled}>
            Accept
          </button>
          <button type="button" onClick={handleCancel}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
