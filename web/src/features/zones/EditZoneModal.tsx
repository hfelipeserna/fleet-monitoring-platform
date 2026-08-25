import { useEffect, useState } from "react";
import { useDialogFocus } from "../../lib/useDialogFocus";
import { parseZoneApiError, zoneApiUrl } from "./api";
import type { ZoneFeature } from "./types";

type ZoneLike = {
  id: string;
  properties: { name: string };
};

type EditZoneModalProps = {
  open: boolean;
  zone: ZoneLike | ZoneFeature | null;
  onClose: () => void;
  onRenamed: () => void;
  onDeleted: () => void;
};

export default function EditZoneModal({ open, zone, onClose, onRenamed, onDeleted }: EditZoneModalProps) {
  const [newName, setNewName] = useState(zone?.properties.name ?? "");
  const [error, setError] = useState("");
  const dialogRef = useDialogFocus(open, onClose);

  useEffect(() => {
    if (zone) setNewName(zone.properties.name);
  }, [zone?.properties.name, zone?.id, open]);

  useEffect(() => {
    if (open) setError("");
  }, [open]);

  if (!open || !zone) return null;

  async function handleRename() {
    const url = zoneApiUrl(`/api/zones/${zone!.id}`);
    try {
      const res = await fetch(url, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: newName.trim() }),
      });
      if (res.ok) {
        setError("");
        onRenamed();
        onClose();
        return;
      }
      const msg = await parseZoneApiError(res);
      setError(msg);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function handleDelete() {
    const url = zoneApiUrl(`/api/zones/${zone!.id}`);
    try {
      const res = await fetch(url, { method: "DELETE" });
      if (res.ok || res.status === 204) {
        setError("");
        onDeleted();
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
    onClose();
  }

  function handleOverlayClick(e: React.MouseEvent<HTMLDivElement>) {
    if (e.target === e.currentTarget) onClose();
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center" onClick={handleOverlayClick}>
      <div
        ref={dialogRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="edit-zone-name-label"
        className="bg-white p-4 rounded shadow-lg min-w-[300px]"
        onClick={(e) => e.stopPropagation()}
      >
        <label id="edit-zone-name-label" htmlFor="edit-zone-name">
          New name
        </label>
        <input
          id="edit-zone-name"
          name="name"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          aria-invalid={!!error}
          aria-describedby={error ? "edit-zone-name-error" : undefined}
        />
        {error ? (
          <span id="edit-zone-name-error" role="alert">
            {error}
          </span>
        ) : null}
        <div className="flex gap-2 mt-2">
          <button type="button" onClick={handleRename}>
            Rename
          </button>
          <button type="button" onClick={handleDelete}>
            Delete
          </button>
          <button type="button" onClick={handleCancel}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
