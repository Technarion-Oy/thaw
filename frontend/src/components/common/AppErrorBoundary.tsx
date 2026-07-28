// SPDX-License-Identifier: GPL-3.0-or-later

import React from "react";

interface State { error: Error | null }

/**
 * Top-level error boundary wrapping the whole React tree (see `main.tsx`).
 *
 * Without it, any error thrown during React's render phase unmounts the root
 * and leaves a blank window with nothing in the logs — the symptom reported in
 * issue #875, where expanding an empty folder in the file browser made
 * `entries.map(...)` throw inside a `setState` updater (which React runs during
 * render, so the surrounding try/catch never sees it). `ResultGrid` has its own
 * boundary for query results; this one is the last line of defense for
 * everything else and turns a dead window into a recoverable message.
 *
 * Deliberately styled with plain DOM + CSS variables instead of Ant Design: the
 * crash may well have come from inside the theme provider / component tree we'd
 * otherwise be rendering into, so this fallback must not depend on any of it.
 */
export default class AppErrorBoundary extends React.Component<
  { children: React.ReactNode },
  State
> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // Console only — the same channel ResultGrid's boundary uses, and the one
    // the WebView inspector surfaces. Errors here may predate any IPC being
    // usable, so don't attempt a backend round-trip.
    console.error("Thaw crashed:", error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    const btn: React.CSSProperties = {
      font: "inherit",
      fontSize: 12,
      padding: "4px 12px",
      color: "var(--text)",
      background: "var(--bg-raised, transparent)",
      border: "1px solid var(--border-strong, currentColor)",
      borderRadius: 4,
      cursor: "pointer",
    };

    return (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: 12,
          height: "100vh",
          padding: 24,
          background: "var(--bg, #0d1117)",
          color: "var(--text, #f0f6fc)",
          fontFamily: "system-ui, sans-serif",
          textAlign: "center",
        }}
      >
        <div style={{ fontSize: 14, fontWeight: 600 }}>Something went wrong.</div>
        <div style={{ fontSize: 12, color: "var(--text-muted, #c9d1d9)", maxWidth: 520 }}>
          The interface hit an unexpected error. Your Snowflake session is still open —
          try again first; reloading starts a fresh window and drops in-memory state.
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <button type="button" style={btn} onClick={() => this.setState({ error: null })}>
            Try again
          </button>
          <button type="button" style={btn} onClick={() => window.location.reload()}>
            Reload
          </button>
        </div>
        <details style={{ maxWidth: 640, width: "100%", textAlign: "left" }}>
          <summary style={{ fontSize: 11, color: "var(--text-faint, #8b949e)", cursor: "pointer" }}>
            Details
          </summary>
          <pre
            style={{
              fontSize: 11,
              color: "var(--text-muted, #c9d1d9)",
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
              maxHeight: 240,
              overflow: "auto",
              margin: "8px 0 0",
            }}
          >
            {String(error.stack || error.message || error)}
          </pre>
        </details>
      </div>
    );
  }
}
