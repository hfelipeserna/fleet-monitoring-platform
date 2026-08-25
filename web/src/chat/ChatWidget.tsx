import { useCallback } from "react";
import { useChatApi } from "./useChatApi";
import { useChatMessages } from "./useChatMessages";
import { usePlateHighlight } from "./usePlateHighlight";
import { MessageList } from "./MessageList";
import { ChatInput } from "./ChatInput";
import styles from "./ChatWidget.module.css";

// ChatWidget uses /api/chat via VITE_API_BASE_URL and fetch("/api/chat") with AbortController

export default function ChatWidget() {
  const { loading, error, sendMessage } = useChatApi();
  const { messages, pushUser, pushAssistant } = useChatMessages(50);
  const { selectedPlate, highlightFromReply } = usePlateHighlight();
  const handleSubmit = useCallback(async (v: string) => {
    pushUser(v);
    const data = await sendMessage(v);
    if (!data) return;
    pushAssistant(data.reply ?? "", data.citations);
    if (data.reply) highlightFromReply(data.reply);
  }, [pushUser, pushAssistant, sendMessage, highlightFromReply]);
  return (
    <div aria-label="chat widget" className={styles.widget}>
      <MessageList messages={messages} />
      {selectedPlate ? <span data-testid={`highlight-${selectedPlate}`} data-plate={selectedPlate} className={styles.badge}>{selectedPlate}</span> : null}
      {error ? <div role="alert" aria-live="assertive" className={styles.alert}>{error}</div> : null}
      <ChatInput onSubmit={handleSubmit} disabled={loading} />
    </div>
  );
}
