import type { Citation } from "./useChatApi";
import styles from "./ChatWidget.module.css";

export function CitationList({ citations }: { citations: Citation[] }) {
  if (!citations.length) return null;
  return (
    <div aria-label="citations" className={styles.citations}>
      {citations.map((c) => (
        <span key={c.tool} className={styles.citation}>
          {c.tool} {c.count}
        </span>
      ))}
    </div>
  );
}
