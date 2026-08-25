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

  const handle = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const v = value.trim();
      if (!v || disabled) return;
      onSubmit(v);
      setValue("");
    },
    [value, onSubmit, disabled],
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
      <button type="submit" disabled={disabled} aria-label="Enviar" className={styles.button}>
        {disabled ? "Enviando..." : "Send"}
      </button>
    </form>
  );
}
