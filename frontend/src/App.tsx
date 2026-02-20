import { useEffect } from "react";
import { ConfigProvider, theme } from "antd";
import AppLayout from "./components/layout/AppLayout";
import { useConnectionStore } from "./store/connectionStore";
import ConnectModal from "./components/connection/ConnectModal";
import { IsConnected } from "../wailsjs/go/main/App";

export default function App() {
  const isConnected  = useConnectionStore((s) => s.isConnected);
  const setIsConnected = useConnectionStore((s) => s.setIsConnected);

  // After a frontend reload the Go backend keeps the connection alive.
  // Restore the connected state so the user isn't kicked to the login screen.
  useEffect(() => {
    IsConnected().then((alive) => {
      if (alive) setIsConnected(true);
    });
  }, []);

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
