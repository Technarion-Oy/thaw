import { describe, it, expect } from "vitest";

// Ant Design's static modal helpers (Modal.confirm / .info / .warning / .error /
// .success) render into a detached React root created outside the
// <ConfigProvider> in App.tsx, so they never see the theme tokens and always
// come up in the default light palette — a white dialog over the dark UI
// (issue #884: "Delete venv folder…" in the Snowpark setup modal).
//
// The fix everywhere is the hook-based instance from <AntApp>:
//   const { modal } = AntApp.useApp();  →  modal.confirm({ … })
// This test is the guard that keeps a static call from creeping back in. The
// <Modal> component is fine and unaffected — only the helpers are matched.
//
// Vite's glob import gives us every source file as raw text; `wailsjs/` sits
// outside src/ and so is never matched, and `generated/` holds registry data,
// not UI.
const SOURCES = import.meta.glob("../**/*.{ts,tsx}", {
  query: "?raw",
  eager: true,
  import: "default",
}) as Record<string, string>;

const STATIC_MODAL = /\bModal\.(confirm|info|warning|error|success)\s*\(/g;

describe("themed confirmation dialogs", () => {
  it("never calls the static Modal.confirm/.info/.warning/.error/.success", () => {
    const offenders: string[] = [];
    for (const [path, src] of Object.entries(SOURCES)) {
      if (path.includes("/generated/") || path.endsWith(".test.ts")) continue;
      for (const m of src.matchAll(STATIC_MODAL)) {
        const line = src.slice(0, m.index).split("\n").length;
        // Paths are relative to this file (src/components/).
        offenders.push(`${path}:${line} — ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("scans a plausible number of source files", () => {
    // A glob that silently stops matching would turn this into a no-op guard.
    expect(Object.keys(SOURCES).length).toBeGreaterThan(100);
  });
});
