// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  defaultColumnOrder,
  reorderColumnOrder,
  visualToOriginalIndex,
  columnIdFor,
} from "./columnOrderUtils";

describe("defaultColumnOrder", () => {
  it("builds {index}_{NAME} ids in SELECT order", () => {
    expect(defaultColumnOrder(["A", "B", "C"])).toEqual(["0_A", "1_B", "2_C"]);
  });
  it("handles duplicate column names (ids stay unique by index)", () => {
    expect(defaultColumnOrder(["X", "X"])).toEqual(["0_X", "1_X"]);
  });
  it("returns an empty array for no columns", () => {
    expect(defaultColumnOrder([])).toEqual([]);
  });
});

describe("reorderColumnOrder", () => {
  const order = ["0_A", "1_B", "2_C", "3_D"];

  it("moves a column to the left (before target)", () => {
    // Drag C before B → A, C, B, D
    expect(reorderColumnOrder(order, "2_C", "1_B", true)).toEqual(["0_A", "2_C", "1_B", "3_D"]);
  });

  it("moves a column to the right (after target)", () => {
    // Drag A after C → B, C, A, D
    expect(reorderColumnOrder(order, "0_A", "2_C", false)).toEqual(["1_B", "2_C", "0_A", "3_D"]);
  });

  it("moves the first column to before the last", () => {
    expect(reorderColumnOrder(order, "0_A", "3_D", true)).toEqual(["1_B", "2_C", "0_A", "3_D"]);
  });

  it("moves the last column to before the first", () => {
    expect(reorderColumnOrder(order, "3_D", "0_A", true)).toEqual(["3_D", "0_A", "1_B", "2_C"]);
  });

  it("returns the same reference when source == target", () => {
    expect(reorderColumnOrder(order, "1_B", "1_B", true)).toBe(order);
  });

  it("returns the same reference when the dragged id is absent", () => {
    expect(reorderColumnOrder(order, "9_Z", "1_B", true)).toBe(order);
  });

  it("returns the same reference when the target id is absent", () => {
    expect(reorderColumnOrder(order, "1_B", "9_Z", false)).toBe(order);
  });

  it("does not mutate the input array", () => {
    const copy = order.slice();
    reorderColumnOrder(order, "2_C", "1_B", true);
    expect(order).toEqual(copy);
  });
});

describe("visualToOriginalIndex", () => {
  it("is the identity when the map is null (default order)", () => {
    expect(visualToOriginalIndex(null, 0)).toBe(0);
    expect(visualToOriginalIndex(undefined, 3)).toBe(3);
  });

  it("translates visual positions to original indices for a reorder", () => {
    // Visual order A, C, B, D ⇒ original indices [0, 2, 1, 3]
    const map = [0, 2, 1, 3];
    expect(visualToOriginalIndex(map, 0)).toBe(0); // A
    expect(visualToOriginalIndex(map, 1)).toBe(2); // C
    expect(visualToOriginalIndex(map, 2)).toBe(1); // B
    expect(visualToOriginalIndex(map, 3)).toBe(3); // D
  });

  it("translates visual positions for a left-pinned column", () => {
    // Pin D to the left ⇒ visual order D, A, B, C ⇒ original [3, 0, 1, 2]
    const map = [3, 0, 1, 2];
    // A drag-select from visual 0 (D) to visual 1 (A) spans originals {3, 0}.
    expect(visualToOriginalIndex(map, 0)).toBe(3);
    expect(visualToOriginalIndex(map, 1)).toBe(0);
  });

  it("falls back to the position when out of range", () => {
    expect(visualToOriginalIndex([0, 1], 5)).toBe(5);
  });
});

describe("columnIdFor", () => {
  it("composes index and name", () => {
    expect(columnIdFor(3, "COL_NAME")).toBe("3_COL_NAME");
  });
});
