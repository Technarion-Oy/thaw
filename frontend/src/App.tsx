// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useState } from "react";
import { App as AntApp, Button, ConfigProvider, Input, Modal, theme, message, notification } from "antd";
import AppLayout from "./components/layout/AppLayout";
import { useConnectionStore } from "./store/connectionStore";
import ConnectModal from "./components/connection/ConnectModal";
import LayoutSettingsModal from "./components/settings/LayoutSettingsModal";
import AISettingsModal from "./components/settings/AISettingsModal";
import SnowparkCheckModal from "./components/snowpark/SnowparkCheckModal";
import SnowparkSetupModal from "./components/snowpark/SnowparkSetupModal";
import EditorPreferencesModal from "./components/editor/EditorPreferencesModal";
import FeatureFlagsModal from "./components/settings/FeatureFlagsModal";
import LoggingPreferencesModal from "./components/settings/LoggingPreferencesModal";
import FileWatchingModal from "./components/settings/FileWatchingModal";
import NotebookPrefsModal from "./components/notebook/NotebookPrefsModal";
import SessionManagementModal from "./components/settings/SessionManagementModal";
import MCPSessionsModal from "./components/settings/MCPSessionsModal";
import AboutModal from "./components/help/AboutModal";
import UpdateNotification from "./components/help/UpdateNotification";
import LicenseAgreement from "./components/setup/LicenseAgreement";
import { IsConnected, IsLicenseAccepted, SubmitMFACode } from "../wailsjs/go/app/App";
import { ClipboardGetText, ClipboardSetText, EventsOn } from "../wailsjs/runtime/runtime";
import { useMonaco } from "@monaco-editor/react";
import { useThemeStore, type ThemePreference } from "./store/themeStore";
import { useDiffStore } from "./store/diffStore";
import { useFeatureFlagsStore } from "./store/featureFlagsStore";
import { useNotebookPrefsStore } from "./store/notebookPrefsStore";

export default function App() {
  const monaco = useMonaco();
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
  const [loggingPrefsOpen, setLoggingPrefsOpen]       = useState(false);
  const [fileWatchingOpen, setFileWatchingOpen]       = useState(false);
  const [snowparkCheckOpen, setSnowparkCheckOpen]       = useState(false);
  const [snowparkSetupOpen, setSnowparkSetupOpen]       = useState(false);
  const [featureFlagsOpen, setFeatureFlagsOpen]         = useState(false);
  const [notebookPrefsOpen, setNotebookPrefsOpen]       = useState(false);
  const [sessionMgmtOpen, setSessionMgmtOpen]           = useState(false);
  const [mcpSessionsOpen, setMcpSessionsOpen]           = useState(false);
  const [aboutOpen, setAboutOpen]                       = useState(false);
  // First-launch license gate: null = still checking, false = must accept
  // (gate shown, workspace withheld), true = accepted (workspace revealed).
  const [licenseAccepted, setLicenseAccepted]           = useState<boolean | null>(null);
  const diffError    = useDiffStore((s) => s.error);
  const clearDiffError = useDiffStore((s) => s.clearError);

  // Interactive MFA re-prompt: the backend emits "mfa:prompt-code" when a
  // passcode-authenticated session must re-login (its lone connection was lost)
  // and needs a fresh one-time code. mfaPrompt holds the pending request.
  const [mfaPrompt, setMfaPrompt] = useState<{ requestId: string; user: string; account: string } | null>(null);
  const [mfaCode, setMfaCode] = useState("");

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

  // Check whether the license has been accepted. Until it has, the license gate
  // blocks the workspace. On any error, fail safe toward showing the gate.
  useEffect(() => {
    IsLicenseAccepted()
      .then(setLicenseAccepted)
      .catch(() => setLicenseAccepted(false));
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

  // MFA-token-caching hint. The backend emits this after an MFA connection when
  // the account's ALLOW_CLIENT_MFA_CACHING is confirmed off — in that state
  // pooled connections re-auth with the single-use passcode and fail, so Thaw
  // runs bulk work (DDL export) at reduced concurrency. Nudge an ACCOUNTADMIN to
  // enable it; dismissible forever via localStorage. See issue #804.
  useEffect(() => {
    const off = EventsOn("mfa:enable-caching-hint", () => {
      if (localStorage.getItem("thaw.mfaCachingHintDismissed") === "1") return;
      const sql = "ALTER ACCOUNT SET ALLOW_CLIENT_MFA_CACHING = TRUE;";
      const key = "mfa-caching-hint";
      notification.info({
        key,
        message: "Enable MFA token caching",
        description: (
          <div>
            <p style={{ marginTop: 0 }}>
              This account uses MFA but <code>ALLOW_CLIENT_MFA_CACHING</code> is off. Thaw keeps its
              connection pool small to avoid repeated MFA prompts and login errors during bulk actions
              like DDL export. An ACCOUNTADMIN can enable seamless caching:
            </p>
            <pre style={{ whiteSpace: "pre-wrap", userSelect: "text", margin: "8px 0" }}>{sql}</pre>
            <div style={{ display: "flex", gap: 8 }}>
              <Button
                size="small"
                type="primary"
                onClick={() => { void ClipboardSetText(sql); message.success("Copied to clipboard"); }}
              >
                Copy SQL
              </Button>
              <Button
                size="small"
                type="text"
                onClick={() => { localStorage.setItem("thaw.mfaCachingHintDismissed", "1"); notification.destroy(key); }}
              >
                Don't show again
              </Button>
            </div>
          </div>
        ),
        duration: 0,
        placement: "bottomRight",
      });
    });
    return () => off();
  }, []);

  // Listen for "Customize Layout…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:customize-layout", () => {
      setLayoutModalOpen(true);
    });
    return () => off();
  }, []);

  // Interactive MFA re-prompt. "mfa:prompt-code" opens the code modal;
  // "mfa:prompt-close" (timeout / ctx cancel on the backend) dismisses it if
  // still open for that request. See issue #804.
  useEffect(() => {
    const offPrompt = EventsOn("mfa:prompt-code", (p: { requestId: string; user: string; account: string }) => {
      setMfaCode("");
      setMfaPrompt(p);
    });
    const offClose = EventsOn("mfa:prompt-close", (requestId: string) => {
      setMfaPrompt((cur) => (cur && cur.requestId === requestId ? null : cur));
    });
    return () => { offPrompt(); offClose(); };
  }, []);

  // Listen for "Configure AI Inline Completions…" — from both the native menu (Wails event) and
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

  // Listen for "Logging Preferences…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:logging-preferences", () => setLoggingPrefsOpen(true));
    return () => off();
  }, []);

  // Listen for "File Watching…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:file-watching", () => setFileWatchingOpen(true));
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

  // Listen for "Session Management…" menu event.
  useEffect(() => {
    const off = EventsOn("menu:session-management", () => setSessionMgmtOpen(true));
    return () => off();
  }, []);

  // Listen for "MCP Sessions…" menu event and the in-app toolbar indicator.
  useEffect(() => {
    const off = EventsOn("menu:mcp-sessions", () => setMcpSessionsOpen(true));
    const open = () => setMcpSessionsOpen(true);
    window.addEventListener("thaw:open-mcp-sessions", open);
    return () => { off(); window.removeEventListener("thaw:open-mcp-sessions", open); };
  }, []);

  // Listen for "About Thaw…" menu event (Help → About Thaw).
  useEffect(() => {
    const off = EventsOn("menu:about", () => setAboutOpen(true));
    return () => off();
  }, []);

  // Open the connect modal on cold startup when not yet connected — but only
  // once the license has been accepted, so the gate is never buried under it.
  // Depends on [hydrated, licenseAccepted] so disconnecting during a session
  // does NOT reopen it.
  useEffect(() => {
    if (hydrated && licenseAccepted && !isConnected) setConnectModalOpen(true);
  }, [hydrated, licenseAccepted]); // eslint-disable-line react-hooks/exhaustive-deps

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
    type Field = HTMLInputElement | HTMLTextAreaElement;

    const isEditableInput = (el: Element | null): el is Field => {
      if (!(el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement)) return false;
      // Ignore Monaco's own code buffer — its paste must go through the editor
      // model (handled per-editor by patchMonacoClipboard, which also stops those
      // events from reaching here). `hasTextFocus()` is the public API for "the
      // editor's text input is focused" and is false for find/replace/rename
      // fields, which this global handler must cover. #593.
      if (el instanceof HTMLTextAreaElement && monaco?.editor.getEditors().some((ed) => ed.hasTextFocus())) {
        return false;
      }
      return true;
    };

    const selectedText = (el: Field) => el.value.slice(el.selectionStart ?? 0, el.selectionEnd ?? 0);

    // Replace the field's selection with `text`, driving the change through the
    // native value setter so React-controlled inputs (Ant Design etc.) fire
    // onChange, then dispatching `input` for any non-React listeners.
    const spliceValue = (el: Field, text: string) => {
      const start = el.selectionStart ?? 0;
      const end = el.selectionEnd ?? 0;
      const next = el.value.slice(0, start) + text + el.value.slice(end);
      const proto = el instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
      Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(el, next);
      el.dispatchEvent(new Event("input", { bubbles: true }));
      el.setSelectionRange(start + text.length, start + text.length);
    };

    const onKeyDown = async (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      // Cheap key check first — only v/c/x need the (editor-scanning) field check.
      const key = e.key;
      if (key !== "v" && key !== "c" && key !== "x") return;
      const target = document.activeElement;
      if (!isEditableInput(target)) return;

      if (key === "v") {
        e.preventDefault();
        const text = await ClipboardGetText();
        if (text) spliceValue(target, text);
      } else {
        const selected = selectedText(target);
        if (!selected) return;
        e.preventDefault();
        await ClipboardSetText(selected);
        if (key === "x") spliceValue(target, "");
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [monaco]);

  const antdAlgorithm = resolved === "dark" ? theme.darkAlgorithm : theme.defaultAlgorithm;
  const isDark = resolved === "dark";

  return (
    <ConfigProvider
      theme={{
        algorithm: antdAlgorithm,
        // Emit --ant-* CSS vars instead of inline-injected styles. Theme switches
        // become cheap (single :root var change) and custom CSS can layer on top
        // without specificity fights.
        cssVar: true,
        hashed: false,
        token: {
          // ── Typography
          fontFamily:        uiFont,
          fontSize:          13,
          fontSizeSM:        12,
          fontSizeLG:        15,
          lineHeight:        1.5715,
          fontWeightStrong:  600,

          // ── Brand
          colorPrimary:      isDark ? "#58a6ff" : "#0550ae",
          colorInfo:         isDark ? "#58a6ff" : "#0550ae",
          colorSuccess:      isDark ? "#3fb950" : "#1a7f37",
          colorWarning:      isDark ? "#d29922" : "#9a6700",
          colorError:        isDark ? "#f85149" : "#cf222e",

          // ── Surfaces (mirror the CSS variables in global.css)
          colorBgBase:       isDark ? "#0d1117" : "#ffffff",
          colorBgLayout:     isDark ? "#0d1117" : "#f6f8fa",
          colorBgContainer:  isDark ? "#161b22" : "#ffffff",
          colorBgElevated:   isDark ? "#21262d" : "#ffffff",
          colorBgSpotlight:  isDark ? "#262c34" : "#eaeef2",

          // ── Text (every tier ≥ AA against colorBgBase)
          colorTextBase:        isDark ? "#f0f6fc" : "#0f1419",
          colorText:            isDark ? "#f0f6fc" : "#0f1419",  // 16.0:1 / 19.6:1
          colorTextSecondary:   isDark ? "#c9d1d9" : "#4a5159",  // 11.4:1 /  8.6:1
          colorTextTertiary:    isDark ? "#8b949e" : "#6e7681",  //  5.3:1 /  4.7:1  ← was failing
          colorTextQuaternary:  isDark ? "#6e7681" : "#8c959f",
          colorTextDescription: isDark ? "#8b949e" : "#6e7681",

          // ── Borders
          //    colorBorder is what Antd uses for Input/Select/Button outlines, so it
          //    must clear WCAG 1.4.11 (3:1 non-text). The lighter, sub-3:1 tier moves
          //    to colorBorderSecondary, where it's used only for decorative dividers
          //    inside already-elevated containers (1.4.11 exempt).
          colorBorder:          isDark ? "#6e7681" : "#8c959f",  // 4.12:1 / 3.04:1 ✓
          colorBorderSecondary: isDark ? "#3d444d" : "#c2c8d0",  // decorative

          // ── Shape
          borderRadius:    6,
          borderRadiusLG:  8,
          borderRadiusSM:  4,
          controlHeight:   32,
          controlHeightSM: 26,
          controlHeightLG: 40,

          // ── Motion (snappier — desktop tool, not a marketing site)
          motionDurationFast: "0.08s",
          motionDurationMid:  "0.14s",
        },
        components: {
          Button: {
            fontWeight: 500,
            primaryShadow: "none",
            defaultShadow: "none",
            ...(isDark && {
              defaultColor:            "#f0f6fc",
              defaultBorderColor:      "#3d444d",
              defaultHoverColor:       "#ffffff",
              defaultHoverBorderColor: "#6e7681",
              defaultHoverBg:          "#21262d",
            }),
          },
          Input: {
            activeBorderColor: isDark ? "#58a6ff" : "#0550ae",
            hoverBorderColor:  isDark ? "#6e7681" : "#8c959f",
            activeShadow:      `0 0 0 2px ${isDark ? "rgba(88,166,255,.25)" : "rgba(5,80,174,.18)"}`,
          },
          Select: {
            optionSelectedBg: isDark ? "#21262d" : "#eaeef2",
            optionActiveBg:   isDark ? "#262c34" : "#f1f4f8",
          },
          Table: {
            headerBg:         isDark ? "#21262d" : "#f6f8fa",
            headerColor:      isDark ? "#f0f6fc" : "#0f1419",
            headerSplitColor: isDark ? "#30363d" : "#d8dde3",
            rowHoverBg:       isDark ? "#262c34" : "#eaeef2",
            borderColor:      isDark ? "#30363d" : "#d8dde3",
          },
          Menu: {
            itemColor:         isDark ? "#c9d1d9" : "#4a5159",
            itemSelectedColor: isDark ? "#ffffff" : "#0f1419",
            itemSelectedBg:    isDark ? "#262c34" : "#eaeef2",
            itemHoverColor:    isDark ? "#f0f6fc" : "#0f1419",
          },
          Tabs: {
            itemColor:         isDark ? "#8b949e" : "#6e7681",
            itemHoverColor:    isDark ? "#c9d1d9" : "#4a5159",
            itemSelectedColor: isDark ? "#f0f6fc" : "#0f1419",
            itemActiveColor:   isDark ? "#f0f6fc" : "#0f1419",
            inkBarColor:       isDark ? "#58a6ff" : "#0550ae",
          },
          Tooltip: {
            colorBgSpotlight:    isDark ? "#262c34" : "#0f1419",
            colorTextLightSolid: "#ffffff",
          },
          Modal: {
            contentBg: isDark ? "#161b22" : "#ffffff",
            headerBg:  isDark ? "#161b22" : "#ffffff",
          },
          Segmented: {
            itemSelectedBg:    isDark ? "#262c34" : "#ffffff",
            itemSelectedColor: isDark ? "#f0f6fc" : "#0f1419",
            itemColor:         isDark ? "#c9d1d9" : "#4a5159",
            itemHoverColor:    isDark ? "#f0f6fc" : "#0f1419",
          },
        },
      }}
    >
      <AntApp>
        {licenseAccepted === false && (
          <LicenseAgreement onAccept={() => setLicenseAccepted(true)} />
        )}
        {hydrated && licenseAccepted && <AppLayout />}
        {connectModalOpen && <ConnectModal onClose={() => setConnectModalOpen(false)} />}
        {layoutModalOpen && (
          <LayoutSettingsModal onClose={() => setLayoutModalOpen(false)} />
        )}
        {aiModalOpen && <AISettingsModal onClose={() => setAiModalOpen(false)} />}
        {editorPrefsOpen && (
          <EditorPreferencesModal onClose={() => setEditorPrefsOpen(false)} />
        )}
        {loggingPrefsOpen && (
          <LoggingPreferencesModal onClose={() => setLoggingPrefsOpen(false)} />
        )}
        {fileWatchingOpen && (
          <FileWatchingModal onClose={() => setFileWatchingOpen(false)} />
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
        {sessionMgmtOpen && (
          <SessionManagementModal onClose={() => setSessionMgmtOpen(false)} />
        )}
        {mcpSessionsOpen && (
          <MCPSessionsModal onClose={() => setMcpSessionsOpen(false)} />
        )}
        {aboutOpen && <AboutModal onClose={() => setAboutOpen(false)} />}
        <UpdateNotification />
        <Modal
          open={!!mfaPrompt}
          title="Enter a new MFA code"
          okText="Submit"
          okButtonProps={{ disabled: mfaCode.trim() === "" }}
          onOk={() => {
            if (mfaPrompt) void SubmitMFACode(mfaPrompt.requestId, mfaCode.trim());
            setMfaPrompt(null);
          }}
          onCancel={() => {
            if (mfaPrompt) void SubmitMFACode(mfaPrompt.requestId, "");
            setMfaPrompt(null);
          }}
          destroyOnClose
          maskClosable={false}
        >
          <p style={{ marginTop: 0 }}>
            Thaw needs a fresh one-time MFA code to reconnect
            {mfaPrompt?.user ? <> as <strong>{mfaPrompt.user}</strong></> : null}. Enter the
            current code from your authenticator app.
          </p>
          <Input
            autoFocus
            value={mfaCode}
            onChange={(e) => setMfaCode(e.target.value)}
            onPressEnter={() => {
              if (mfaPrompt && mfaCode.trim() !== "") {
                void SubmitMFACode(mfaPrompt.requestId, mfaCode.trim());
                setMfaPrompt(null);
              }
            }}
            placeholder="6-digit code"
            maxLength={8}
            inputMode="numeric"
            autoComplete="one-time-code"
          />
        </Modal>
      </AntApp>
    </ConfigProvider>
  );
}
