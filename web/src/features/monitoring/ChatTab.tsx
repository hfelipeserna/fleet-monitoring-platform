import ChatWidget from "../../chat/ChatWidget";
import { BottomPanelShell } from "./BottomPanelShell";

export type ChatTabProps = {
  hidden?: boolean;
};

export default function ChatTab({ hidden }: ChatTabProps) {
  return (
    <BottomPanelShell activeKey="chat" testId="chat-panel" hidden={hidden}>
      <ChatWidget />
    </BottomPanelShell>
  );
}
