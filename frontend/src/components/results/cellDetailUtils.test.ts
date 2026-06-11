// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { describe, expect, it } from "vitest";
import {
  DISPLAY_CAP,
  JSON_DETECT_CAP,
  prettyPrintJson,
  truncateForDisplay,
  reconcileDismissedKey,
  computeCellScrollLeft,
} from "./cellDetailUtils";

// ── prettyPrintJson ───────────────────────────────────────────────────────────

describe("prettyPrintJson", () => {
  it("pretty-prints compact JSON objects", () => {
    expect(prettyPrintJson('{"a":1,"b":[2,3]}')).toBe('{\n  "a": 1,\n  "b": [\n    2,\n    3\n  ]\n}');
  });

  it("pretty-prints compact JSON arrays", () => {
    expect(prettyPrintJson("[1,2]")).toBe("[\n  1,\n  2\n]");
  });

  it("tolerates surrounding whitespace", () => {
    expect(prettyPrintJson('  {"a":1} \n')).toBe('{\n  "a": 1\n}');
  });

  it("returns null for non-JSON-shaped text", () => {
    expect(prettyPrintJson("hello world")).toBeNull();
    expect(prettyPrintJson("42")).toBeNull(); // valid JSON, but not an object/array
    expect(prettyPrintJson('"quoted"')).toBeNull();
  });

  it("returns null for malformed JSON", () => {
    expect(prettyPrintJson('{"a":')).toBeNull();
    expect(prettyPrintJson("[1,2,")).toBeNull();
  });

  it("returns null when the value is already in the formatted form", () => {
    const formatted = '{\n  "a": 1\n}';
    expect(prettyPrintJson(formatted)).toBeNull();
  });

  it("skips values above JSON_DETECT_CAP without parsing", () => {
    const huge = '{"a":"' + "x".repeat(JSON_DETECT_CAP) + '"}';
    expect(prettyPrintJson(huge)).toBeNull();
  });
});

// ── truncateForDisplay ────────────────────────────────────────────────────────

describe("truncateForDisplay", () => {
  it("passes small values through untouched", () => {
    expect(truncateForDisplay("abc", false)).toEqual({ text: "abc", truncated: false });
  });

  it("passes values exactly at the cap through untouched", () => {
    const text = "x".repeat(DISPLAY_CAP);
    expect(truncateForDisplay(text, false)).toEqual({ text, truncated: false });
  });

  it("truncates values above the cap", () => {
    const text = "x".repeat(DISPLAY_CAP + 1);
    const result = truncateForDisplay(text, false);
    expect(result.truncated).toBe(true);
    expect(result.text).toHaveLength(DISPLAY_CAP);
  });

  it("returns the full value when showFull is set", () => {
    const text = "x".repeat(DISPLAY_CAP + 1);
    expect(truncateForDisplay(text, true)).toEqual({ text, truncated: false });
  });
});

// ── reconcileDismissedKey ─────────────────────────────────────────────────────

describe("reconcileDismissedKey", () => {
  it("keeps a dismissal while the anchor stays on the dismissed cell", () => {
    expect(reconcileDismissedKey("3:2", "3:2")).toBe("3:2");
  });

  it("clears the dismissal when the anchor moves to a different cell", () => {
    expect(reconcileDismissedKey("3:2", "4:2")).toBeNull();
  });

  it("clears the dismissal when the selection is cleared (new result)", () => {
    // Prevents a dismissal from result A suppressing the same coordinates in result B.
    expect(reconcileDismissedKey("0:0", null)).toBeNull();
  });

  it("stays clear when nothing was dismissed", () => {
    expect(reconcileDismissedKey(null, "1:1")).toBeNull();
    expect(reconcileDismissedKey(null, null)).toBeNull();
  });
});

// ── computeCellScrollLeft ─────────────────────────────────────────────────────

describe("computeCellScrollLeft", () => {
  // Viewport 500px wide with a 30px row-number gutter sticky on the left:
  const base = {
    scrollLeft: 0,
    clientWidth: 500,
    stickyLeadingWidth: 30,
    stickyTrailingWidth: 0,
  };

  it("returns null when the column is fully visible", () => {
    expect(computeCellScrollLeft({ ...base, colStart: 100, colWidth: 80 })).toBeNull();
  });

  it("scrolls right just enough to expose a column hidden past the right edge", () => {
    // Column ends at 700, visible window ends at 500 → scroll by 200.
    expect(computeCellScrollLeft({ ...base, colStart: 600, colWidth: 100 })).toBe(200);
  });

  it("scrolls left just enough to expose a column hidden behind the leading sticky region", () => {
    // Scrolled to 300: window starts at 330; column at 250 → scroll back by 80.
    expect(computeCellScrollLeft({ ...base, scrollLeft: 300, colStart: 250, colWidth: 60 })).toBe(220);
  });

  it("excludes pinned-right columns from the visible window", () => {
    // Window end is 500 - 100 = 400; column ends at 450 → scroll by 50.
    expect(
      computeCellScrollLeft({ ...base, stickyTrailingWidth: 100, colStart: 350, colWidth: 100 }),
    ).toBe(50);
  });

  it("keeps the start of an over-wide column in view instead of overshooting", () => {
    // Column is 800 wide in a ~470 window: exposing the end would push the
    // start behind the gutter; clamp to keeping the start at the window edge.
    expect(computeCellScrollLeft({ ...base, colStart: 130, colWidth: 800 })).toBe(100);
  });
});
