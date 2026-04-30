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
import { App as AntApp, ConfigProvider, theme, message } from "antd";
import AppLayout from "./components/layout/AppLayout";
import { useConnectionStore } from "./store/connectionStore";
import ConnectModal from "./components/connection/ConnectModal";
import LayoutSettingsModal from "./components/settings/LayoutSettingsModal";
import AISettingsModal from "./components/settings/AISettingsModal";
import SnowparkCheckModal from "./components/snowpark/SnowparkCheckModal";
import SnowparkSetupModal from "./components/snowpark/SnowparkSetupModal";
import EditorPreferencesModal from "./components/editor/EditorPreferencesModal";
import FeatureFlagsModal from "./components/settings/FeatureFlagsModal";
import NotebookPrefsModal from "./components/notebook/NotebookPrefsModal";
import { IsConnected } from "../wailsjs/go/main/App";
import { ClipboardGetText, ClipboardSetText, EventsOn } from "../wailsjs/runtime/runtime";
import { useThemeStore, type ThemePreference } from "./store/themeStore";
import { useDiffStore } from "./store/diffStore";
import { useFeatureFlagsStore } from "./store/featureFlagsStore";
import { useNotebookPrefsStore } from "./store/notebookPrefsStore";

export default function App() {
  const isConnected    = useConnectionStore((s) => s.isConnected);
  const setIsConnected = useConnectionStore((s) => s.setIsConnected);
  const resolved      = useThemeStore((s) => s.resolved);
  const syncSystem    = useThemeStore((s) => s.syncSystem);
  const setPreference = useThemeStore((s) => s.setPreference);
  const uiFont        = useThemeStore((s) => s.uiFont);

  const [connectModalOpen, setConnectModalOpen]         = useState(false);
  const [layoutModalOpen, setLayoutModalOpen]         = useState(false);
  const [aiModalOpen, setAiModalOpen]                 = useState(false);
  const [editorPrefsOpen, setEditorPrefsOpen]         = useState(false);
  const [snowparkCheckOpen, setSnowparkCheckOpen]       = useState(false);
  const [snowparkSetupOpen, setSnowparkSetupOpen]       = useState(false);
  const [featureFlagsOpen, setFeatureFlagsOpen]         = useState(false);
  const [notebookPrefsOpen, setNotebookPrefsOpen]       = useState(false);
  const diffError    = useDiffStore((s) => s.error);
  const clearDiffError = useDiffStore((s) => s.clearError);

  // Whether the Zustand persist store has finished hydrating from sessionStorage.
  // We hold off rendering AppLayout until we know the true persisted state so
  // the UI initialises with the correct connection/theme/session values, avoiding
  // a flash of incorrect state on reload.
  const [hydrated, setHydrated] = useState(
    () => useConnectionStore.persist.hasHydrated()
  );
  useEffect(() => {
    if (!hydrated) {
      const unsub = useConnectionStore.persist.onFinishHydration(() => setHydrated(true));
      return unsub;
    }
  }, [hydrated]);

  useEffect(() => {
    if (diffError) {
      message.error(`Comparison failed: ${diffError}`);
      clearDiffError();
    }
  }, [diffError]);

  // After a frontend reload the Go backend keeps the connection alive.
  // Sync the store to the actual backend state: set connected if the backend
  // is still alive, or clear it if the backend was restarted (so the user
  // gets ConnectModal with pre-filled params rather than a broken AppLayout).
  useEffect(() => {
    IsConnected().then((alive) => {
      setIsConnected(alive);
    });
  }, []);

  // Load feature flags from persisted config on startup.
  const loadFeatureFlags = useFeatureFlagsStore((s) => s.load);
  useEffect(() => { void loadFeatureFlags(); }, []);

  // Load notebook preferences from persisted config on startup.
  const loadNotebookPrefs = useNotebookPrefsStore((s) => s.load);
  useEffect(() => { void loadNotebookPrefs(); }, []);

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

  // Listen for "Configure AI…" — from both the native menu (Wails event) and
  // the ⌘, keyboard shortcut (browser custom event).
  useEffect(() => {
    const wailsOff = EventsOn("menu:configure-ai", () => setAiModalOpen(true));
    const domHandler = () => setAiModalOpen(true);
    window.addEventListener("thaw:configure-ai", domHandler);
    return () => { wailsOff(); window.removeEventListener("thaw:configure-ai", domHandler); };
  }, []);

  // Listen for "Editor Preferences…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:editor-preferences", () => setEditorPrefsOpen(true));
    return () => off();
  }, []);

  // Listen for Snowpark menu events.
  useEffect(() => {
    const offCheck = EventsOn("menu:snowpark-check", () => setSnowparkCheckOpen(true));
    const offSetup = EventsOn("menu:snowpark-setup", () => setSnowparkSetupOpen(true));
    return () => { (offCheck as () => void)(); (offSetup as () => void)(); };
  }, []);

  // Listen for "Feature Flags…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:feature-flags", () => setFeatureFlagsOpen(true));
    return () => off();
  }, []);

  // Listen for "Notebook Preferences…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:notebook-preferences", () => setNotebookPrefsOpen(true));
    return () => off();
  }, []);

  // Open the connect modal on cold startup when not yet connected.
  // Depends only on [hydrated] so disconnecting during a session does NOT reopen it.
  useEffect(() => {
    if (hydrated && !isConnected) setConnectModalOpen(true);
  }, [hydrated]); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-close the connect modal once a connection is established.
  useEffect(() => {
    if (isConnected) setConnectModalOpen(false);
  }, [isConnected]);

  // Allow any component to open the connect modal via a custom DOM event.
  useEffect(() => {
    const handler = () => setConnectModalOpen(true);
    window.addEventListener("thaw:connect", handler);
    return () => window.removeEventListener("thaw:connect", handler);
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
  const isDark = resolved === "dark";

  return (
    <ConfigProvider
      theme={{
        algorithm: antdAlgorithm,
        token: {
          colorPrimary: isDark ? "#40c8fc" : "#0969da",
          borderRadius: 6,
          fontFamily: uiFont,
          fontWeightStrong: 600,
        },
        components: {
          Button: {
            fontWeight: 500,
            // Dark mode: vivid text and a clearly visible border on default buttons.
            ...(isDark && {
              defaultColor:            "#f0f6fc",
              defaultBorderColor:      "#768390",
              defaultHoverColor:       "#ffffff",
              defaultHoverBorderColor: "#adbac7",
              defaultHoverBg:          "#2d333b",
            }),
          },
        },
      }}
    >
      <AntApp>
        {hydrated && <AppLayout />}
        {connectModalOpen && <ConnectModal onClose={() => setConnectModalOpen(false)} />}
        {layoutModalOpen && (
          <LayoutSettingsModal onClose={() => setLayoutModalOpen(false)} />
        )}
        {aiModalOpen && <AISettingsModal onClose={() => setAiModalOpen(false)} />}
        {editorPrefsOpen && (
          <EditorPreferencesModal onClose={() => setEditorPrefsOpen(false)} />
        )}
        {snowparkCheckOpen && (
          <SnowparkCheckModal
            onClose={() => setSnowparkCheckOpen(false)}
            onSetup={() => setSnowparkSetupOpen(true)}
          />
        )}
        {snowparkSetupOpen && (
          <SnowparkSetupModal onClose={() => setSnowparkSetupOpen(false)} />
        )}
        {featureFlagsOpen && (
          <FeatureFlagsModal onClose={() => setFeatureFlagsOpen(false)} />
        )}
        {notebookPrefsOpen && (
          <NotebookPrefsModal onClose={() => setNotebookPrefsOpen(false)} />
        )}
      </AntApp>
    </ConfigProvider>
  );
}
