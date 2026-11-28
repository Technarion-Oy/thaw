import { describe, it, expect, vi, beforeEach } from "vitest";

const runtimeMocks = vi.hoisted(() => ({
  BrowserOpenURL: vi.fn(),
  ClipboardGetText: vi.fn(),
  ClipboardSetText: vi.fn(),
  EventsEmit: vi.fn(),
  EventsOn: vi.fn(),
}));

vi.mock("../../wailsjs/runtime/runtime", () => runtimeMocks);

import {
  BrowserOpenURL,
  ClipboardGetText,
  ClipboardSetText,
  EventsEmit,
  EventsOn,
} from "./wailsRuntime";

const SOURCES = import.meta.glob("../**/*.{ts,tsx}", {
  query: "?raw",
  eager: true,
  import: "default",
}) as Record<string, string>;

const DIRECT_WAILS_RUNTIME = /from\s+["'][^"']*wailsjs\/runtime(?:\/runtime)?["']/g;

describe("wailsRuntime wrapper", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards EventsOn and returns the unsubscribe callback", () => {
    const off = vi.fn();
    runtimeMocks.EventsOn.mockReturnValue(off);
    const cb = vi.fn();

    const returned = EventsOn("menu:about", cb);

    expect(runtimeMocks.EventsOn).toHaveBeenCalledWith("menu:about", cb);
    expect(returned).toBe(off);
  });

  it("forwards EventsEmit with variadic payloads", () => {
    EventsEmit("dap:client-to-backend", "frame", 1);
    expect(runtimeMocks.EventsEmit).toHaveBeenCalledWith("dap:client-to-backend", "frame", 1);
  });

  it("forwards ClipboardGetText and ClipboardSetText", async () => {
    runtimeMocks.ClipboardGetText.mockResolvedValue("copied");
    runtimeMocks.ClipboardSetText.mockResolvedValue(true);

    await expect(ClipboardGetText()).resolves.toBe("copied");
    await expect(ClipboardSetText("hello")).resolves.toBe(true);
    expect(runtimeMocks.ClipboardSetText).toHaveBeenCalledWith("hello");
  });

  it("forwards BrowserOpenURL", () => {
    BrowserOpenURL("https://example.com");
    expect(runtimeMocks.BrowserOpenURL).toHaveBeenCalledWith("https://example.com");
  });
});

describe("wailsRuntime isolation guard", () => {
  it("keeps all wailsjs/runtime imports inside utils/wailsRuntime.ts", () => {
    const offenders: string[] = [];
    for (const [path, src] of Object.entries(SOURCES)) {
      if (
        path === "./wailsRuntime.ts" ||
        path === "./wailsRuntime.test.ts" ||
        path.includes("/generated/")
      ) {
        continue;
      }
      for (const m of src.matchAll(DIRECT_WAILS_RUNTIME)) {
        const line = src.slice(0, m.index).split("\n").length;
        offenders.push(`${path}:${line} — ${m[0]}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("scans a plausible number of source files", () => {
    expect(Object.keys(SOURCES).length).toBeGreaterThan(100);
  });
});
