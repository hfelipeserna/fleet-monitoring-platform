import type { ChatMessage } from "./useChatMessages";
import { MessageItem } from "./MessageItem";
import styles from "./ChatWidget.module.css";

export function MessageList({ messages }: { messages: ChatMessage[] }) {
  return (
    <div role="log" aria-live="polite" aria-label="historial de chat" className={styles.log}>
      {messages.map((m) => (
        <MessageItem key={m.id} message={m} />
      ))}
    </div>
  );
}
