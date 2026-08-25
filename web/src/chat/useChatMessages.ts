import { useCallback, useState } from "react";
import type { Citation } from "./useChatApi";

export type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  citations?: Citation[];
};

const LIMIT = 50;

function newId(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  }
}

export function useChatMessages(limit = LIMIT) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);

  const push = useCallback(
    (msg: Omit<ChatMessage, "id">) => {
      const withId: ChatMessage = { ...msg, id: newId() };
      setMessages((prev) => [...prev.slice(-(limit - 1)), withId]);
    },
    [limit],
  );

  const pushUser = useCallback((content: string) => push({ role: "user", content }), [push]);
  const pushAssistant = useCallback(
    (content: string, citations?: Citation[]) => push({ role: "assistant", content, citations }),
    [push],
  );

  return { messages, push, pushUser, pushAssistant };
}
