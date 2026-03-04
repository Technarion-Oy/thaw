// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import { ConfigProvider, theme } from "antd";
import AppLayout from "./components/layout/AppLayout";
import { useConnectionStore } from "./store/connectionStore";
import ConnectModal from "./components/connection/ConnectModal";
import LayoutSettingsModal from "./components/settings/LayoutSettingsModal";
import AISettingsModal from "./components/settings/AISettingsModal";
import { IsConnected } from "../wailsjs/go/main/App";
import { ClipboardGetText, ClipboardSetText, EventsOn } from "../wailsjs/runtime/runtime";
import { useThemeStore, type ThemePreference } from "./store/themeStore";

export default function App() {
  const isConnected    = useConnectionStore((s) => s.isConnected);
  const setIsConnected = useConnectionStore((s) => s.setIsConnected);
  const resolved      = useThemeStore((s) => s.resolved);
  const syncSystem    = useThemeStore((s) => s.syncSystem);
  const setPreference = useThemeStore((s) => s.setPreference);
  const uiFont        = useThemeStore((s) => s.uiFont);

  const [layoutModalOpen, setLayoutModalOpen] = useState(false);
  const [aiModalOpen, setAiModalOpen] = useState(false);

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

  // Listen for "Customize Layout…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:customize-layout", () => {
      setLayoutModalOpen(true);
    });
    return () => off();
  }, []);

  // Listen for "Configure AI…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:configure-ai", () => setAiModalOpen(true));
    return () => off();
  }, []);

  // Global clipboard fix for WKWebView on macOS.
  // navigator.clipboard.readText() is blocked, so Cmd+V in regular <input> /
  // <textarea> elements fails silently. We intercept keydown and manually
  // insert text via the Wails native clipboard API. The React synthetic event
  // system is triggered via the native input value setter so controlled
  // components (Ant Design inputs etc.) update their state correctly.
  // Cmd+C / Cmd+X are handled symmetrically for consistency.
  useEffect(() => {
    const isEditableInput = (el: Element | null): el is HTMLInputElement | HTMLTextAreaElement =>
      el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement;

    // Insert text at the current selection of a native input / textarea.
    const spliceValue = (target: HTMLInputElement | HTMLTextAreaElement, text: string) => {
      const start = target.selectionStart ?? 0;
      const end   = target.selectionEnd   ?? 0;
      const next  = target.value.slice(0, start) + text + target.value.slice(end);
      // Use the native setter so React's synthetic onChange fires.
      const proto  = target instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
      const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
      setter?.call(target, next);
      target.dispatchEvent(new Event("input", { bubbles: true }));
      target.setSelectionRange(start + text.length, start + text.length);
    };

    const onKeyDown = async (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      const target = document.activeElement;
      if (!isEditableInput(target)) return;

      if (e.key === "v") {
        e.preventDefault();
        const text = await ClipboardGetText();
        if (text) spliceValue(target, text);
      } else if (e.key === "c" || e.key === "x") {
        const selected = target.value.slice(
          target.selectionStart ?? 0,
          target.selectionEnd   ?? 0,
        );
        if (!selected) return;
        e.preventDefault();
        await ClipboardSetText(selected);
        if (e.key === "x") spliceValue(target, "");
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  const antdAlgorithm = resolved === "dark" ? theme.darkAlgorithm : theme.defaultAlgorithm;

  return (
    <ConfigProvider
      theme={{
        algorithm: antdAlgorithm,
        token: {
          colorPrimary: resolved === "dark" ? "#29B6F6" : "#0969da",
          borderRadius: 6,
          fontFamily: uiFont,
        },
      }}
    >
      {isConnected ? <AppLayout /> : <ConnectModal />}
      {layoutModalOpen && (
        <LayoutSettingsModal onClose={() => setLayoutModalOpen(false)} />
      )}
      {aiModalOpen && <AISettingsModal onClose={() => setAiModalOpen(false)} />}
    </ConfigProvider>
  );
}
