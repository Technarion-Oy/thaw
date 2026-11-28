import { describe, it, expect, vi, beforeEach } from "vitest";

const runtimeMocks = vi.hoisted(() => {
  class MockRuntimeError extends Error {
    constructor(message: string) {
      super(message);
      this.name = "RuntimeError";
    }
  }
  return {
    Browser: {
      OpenURL: vi.fn(),
    },
    Call: {
      RuntimeError: MockRuntimeError,
    },
    Clipboard: {
      Text: vi.fn(),
      SetText: vi.fn(),
    },
    Events: {
      Emit: vi.fn(),
      On: vi.fn(),
    },
  };
});

vi.mock("@wailsio/runtime", () => runtimeMocks);

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

const DIRECT_WAILS_RUNTIME = /from\s+["'](?:@wailsio\/runtime|[^"']*wailsjs\/runtime(?:\/runtime)?)["']/g;

describe("wailsRuntime wrapper", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("unwraps WailsEvent.data in EventsOn and returns the unsubscribe callback", () => {
    const off = vi.fn();
    let capturedHandler: ((ev: { name: string; data: any }) => void) | undefined;
    runtimeMocks.Events.On.mockImplementation((_name, handler) => {
      capturedHandler = handler;
      return off;
    });
    const cb = vi.fn();

    const returned = EventsOn("menu:theme", cb);

    expect(runtimeMocks.Events.On).toHaveBeenCalledWith("menu:theme", expect.any(Function));
    expect(returned).toBe(off);

    // Payload present (including falsy boolean / empty string) is forwarded.
    capturedHandler?.({ name: "menu:theme", data: "dark" });
    expect(cb).toHaveBeenLastCalledWith("dark");

    capturedHandler?.({ name: "menu:query-log-toggle", data: false });
    expect(cb).toHaveBeenLastCalledWith(false);

    capturedHandler?.({ name: "dap:proxy-ready", data: "" });
    expect(cb).toHaveBeenLastCalledWith("");

    // Null/undefined payload (0-arg backend emit) invokes callback with no args.
    cb.mockClear();
    capturedHandler?.({ name: "menu:about", data: null });
    expect(cb).toHaveBeenCalledWith();
    expect(cb.mock.calls[0]).toHaveLength(0);
  });

  it("forwards EventsEmit for 0, 1, and multiple payload arguments", () => {
    runtimeMocks.Events.Emit.mockResolvedValue(false);

    EventsEmit("ping");
    expect(runtimeMocks.Events.Emit).toHaveBeenNthCalledWith(1, "ping");

    EventsEmit("dap:client-to-backend", "frame");
    expect(runtimeMocks.Events.Emit).toHaveBeenNthCalledWith(2, "dap:client-to-backend", "frame");

    EventsEmit("multi", "a", 2);
    expect(runtimeMocks.Events.Emit).toHaveBeenNthCalledWith(3, "multi", ["a", 2]);
  });

  it("forwards ClipboardGetText and maps ClipboardSetText resolve/reject to boolean", async () => {
    runtimeMocks.Clipboard.Text.mockResolvedValue("copied");
    runtimeMocks.Clipboard.SetText.mockResolvedValueOnce(undefined);
    runtimeMocks.Clipboard.SetText.mockRejectedValueOnce(new Error("denied"));

    await expect(ClipboardGetText()).resolves.toBe("copied");
    await expect(ClipboardSetText("hello")).resolves.toBe(true);
    expect(runtimeMocks.Clipboard.SetText).toHaveBeenCalledWith("hello");
    await expect(ClipboardSetText("fail")).resolves.toBe(false);
  });

  it("forwards BrowserOpenURL", () => {
    runtimeMocks.Browser.OpenURL.mockResolvedValue(undefined);
    BrowserOpenURL("https://example.com");
    expect(runtimeMocks.Browser.OpenURL).toHaveBeenCalledWith("https://example.com");
  });

  it("formats RuntimeError via String(err) without a RuntimeError prefix", () => {
    const err = new runtimeMocks.Call.RuntimeError("SQL compilation error: table not found");
    expect(String(err)).toBe("SQL compilation error: table not found");
    expect(`${err}`).toBe("SQL compilation error: table not found");
    expect(err.message).toBe("SQL compilation error: table not found");
  });
});

describe("wailsRuntime isolation guard", () => {
  it("keeps all @wailsio/runtime imports inside utils/wailsRuntime.ts", () => {
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
