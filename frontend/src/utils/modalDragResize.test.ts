// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { clamp } from "./modalDragResize";

describe("clamp", () => {
  it("passes a value that is within bounds through unchanged", () => {
    expect(clamp(50, 0, 100)).toBe(50);
  });

  it("floors to lo and ceils to hi — the drag viewport bounds", () => {
    expect(clamp(-30, 0, 100)).toBe(0);
    expect(clamp(160, 0, 100)).toBe(100);
  });

  it("handles a negative lower bound (KEEP_X - width can be negative)", () => {
    expect(clamp(-500, -420, 900)).toBe(-420);
  });
});
