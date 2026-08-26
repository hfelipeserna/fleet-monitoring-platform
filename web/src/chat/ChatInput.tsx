import { FormEvent, useState, useCallback } from "react";
import styles from "./ChatWidget.module.css";

export function ChatInput({
  onSubmit,
  disabled,
}: {
  onSubmit: (value: string) => void;
  disabled?: boolean;
}) {
  const [value, setValue] = useState("");
  const isEmpty = value.trim() === "";
  const isDisabled = Boolean(disabled || isEmpty);

  const handle = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const v = value.trim();
      if (!v || disabled || isEmpty) return;
      onSubmit(v);
      setValue("");
    },
    [value, onSubmit, disabled, isEmpty],
  );

  return (
    <form onSubmit={handle} aria-label="formulario de chat" className={styles.form}>
      <input
        aria-label="chat input"
        placeholder="Escribe tu pregunta"
        value={value}
        onChange={(ev) => setValue(ev.target.value)}
        disabled={disabled}
        className={styles.input}
      />
      <button
        type="submit"
        disabled={isDisabled}
        aria-disabled={isDisabled ? "true" : undefined}
        aria-label="Enviar"
        className={`${styles.button} bg-blue-600 hover:bg-blue-700 text-white focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 rounded disabled:opacity-50`}
      >
        {disabled ? "Enviando..." : "↩"}
      </button>
    </form>
  );
}
