import Map from "./map/Map";
import ChatWidget from "./chat/ChatWidget";

export default function App() {
  return (
    <div style={{ display: "flex", height: "100vh", width: "100vw" }}>
      <div style={{ flex: 1, position: "relative" }}>
        <Map vehicles={[]} />
      </div>
      <ChatWidget />
    </div>
  );
}
