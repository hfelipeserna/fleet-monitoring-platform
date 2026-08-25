import ReactMarkdown from "react-markdown";
import type { ChatMessage } from "./useChatMessages";
import { CitationList } from "./CitationList";
import styles from "./ChatWidget.module.css";

export function MessageItem({ message }: { message: ChatMessage }) {
  return (
    <div data-testid={message.role === "assistant" ? "assistant-msg" : "user-msg"} className={styles.message}>
      {message.role === "assistant" ? (
        <ReactMarkdown
          skipHtml
          urlTransform={(url) =>
            url.startsWith("http://") || url.startsWith("https://") || url.startsWith("/") ? url : ""
          }
        >
          {message.content}
        </ReactMarkdown>
      ) : (
        <span>{message.content}</span>
      )}
      {message.citations ? <CitationList citations={message.citations} /> : null}
    </div>
  );
}
