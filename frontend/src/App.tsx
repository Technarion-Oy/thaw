// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect } from "react";
import { ConfigProvider, theme } from "antd";
import AppLayout from "./components/layout/AppLayout";
import { useConnectionStore } from "./store/connectionStore";
import ConnectModal from "./components/connection/ConnectModal";
import { IsConnected } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { useThemeStore, type ThemePreference } from "./store/themeStore";

export default function App() {
  const isConnected    = useConnectionStore((s) => s.isConnected);
  const setIsConnected = useConnectionStore((s) => s.setIsConnected);
  const resolved       = useThemeStore((s) => s.resolved);
  const syncSystem     = useThemeStore((s) => s.syncSystem);
  const setPreference  = useThemeStore((s) => s.setPreference);

  // After a frontend reload the Go backend keeps the connection alive.
  // Restore the connected state so the user isn't kicked to the login screen.
  useEffect(() => {
    IsConnected().then((alive) => {
      if (alive) setIsConnected(true);
    });
  }, []);

  // Listen for system-level color-scheme changes and update the resolved theme.
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    mq.addEventListener("change", syncSystem);
    return () => mq.removeEventListener("change", syncSystem);
  }, [syncSystem]);

  // Listen for View > Appearance menu events from the native menu bar.
  useEffect(() => {
    const off = EventsOn("menu:theme", (value: string) => {
      setPreference(value as ThemePreference);
    });
    return () => off();
  }, [setPreference]);

  const antdAlgorithm = resolved === "dark" ? theme.darkAlgorithm : theme.defaultAlgorithm;

  return (
    <ConfigProvider
      theme={{
        algorithm: antdAlgorithm,
        token: {
          colorPrimary: resolved === "dark" ? "#29B6F6" : "#0969da",
          borderRadius: 6,
          fontFamily: "'Inter', 'SF Pro Text', system-ui, sans-serif",
        },
      }}
    >
      {isConnected ? <AppLayout /> : <ConnectModal />}
    </ConfigProvider>
  );
}
