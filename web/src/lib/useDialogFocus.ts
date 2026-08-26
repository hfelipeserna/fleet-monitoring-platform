import { useEffect, useRef } from "react";

export function useDialogFocus(open: boolean, onClose: () => void) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const prevRef = useRef<Element | null>(null);

  useEffect(() => {
    if (!open) return;
    prevRef.current = document.activeElement;
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (!dialog.hasAttribute("tabindex")) {
      dialog.setAttribute("tabindex", "-1");
    }
    const focusableSelector =
      'a[href], button:not([disabled]), textarea, input:not([disabled]), select, [tabindex]:not([tabindex="-1"])';
    const getNodes = () =>
      Array.from(dialog.querySelectorAll<HTMLElement>(focusableSelector));

    const raf = requestAnimationFrame(() => {
      const nodes = getNodes();
      const first = nodes[0];
      if (first) first.focus();
      else dialog.focus();
    });

    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
        return;
      }
      if (e.key === "Tab") {
        const nodes = getNodes();
        if (nodes.length === 0) {
          e.preventDefault();
          return;
        }
        const firstEl = nodes[0];
        const lastEl = nodes[nodes.length - 1];
        if (e.shiftKey) {
          if (document.activeElement === firstEl) {
            e.preventDefault();
            lastEl.focus();
          }
        } else if (document.activeElement === lastEl) {
          e.preventDefault();
          firstEl.focus();
        }
      }
    }

    dialog.addEventListener("keydown", handleKey);
    return () => {
      cancelAnimationFrame(raf);
      dialog.removeEventListener("keydown", handleKey);
      const prev = prevRef.current as HTMLElement | null;
      if (prev && typeof prev.focus === "function") {
        try {
          prev.focus();
        } catch {
          // ignore
        }
      }
    };
  }, [open, onClose]);

  return dialogRef;
}
