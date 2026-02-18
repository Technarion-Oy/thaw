import { ConfigProvider, theme } from "antd";
import AppLayout from "./components/layout/AppLayout";
import { useConnectionStore } from "./store/connectionStore";
import ConnectModal from "./components/connection/ConnectModal";

export default function App() {
  const isConnected = useConnectionStore((s) => s.isConnected);

  return (
    <ConfigProvider
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: "#29B6F6",
          borderRadius: 6,
          fontFamily: "'Inter', 'SF Pro Text', system-ui, sans-serif",
        },
      }}
    >
      {isConnected ? <AppLayout /> : <ConnectModal />}
    </ConfigProvider>
  );
}
