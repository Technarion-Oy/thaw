// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { buildQuickChartData, sampleRowIndices } from "./quickChartUtils";

const rowsOf = (n: number, make: (i: number) => unknown[]) =>
  Array.from({ length: n }, (_, i) => ({ original: make(i) }));

const range = (startRow: number, endRow: number, startCol = 0, endCol = 1) => ({
  startRow,
  endRow,
  startCol,
  endCol,
});

describe("sampleRowIndices", () => {
  it("returns every row when the range fits", () => {
    expect(sampleRowIndices(3, 6, 10)).toEqual([3, 4, 5, 6]);
  });
  it("returns evenly spaced rows including first and last when it does not", () => {
    const idx = sampleRowIndices(10, 1_000_009, 5);
    expect(idx).toHaveLength(5);
    expect(idx[0]).toBe(10);
    expect(idx[4]).toBe(1_000_009);
    expect(new Set(idx).size).toBe(5);
  });
});

describe("buildQuickChartData", () => {
  it("charts small selections row for row", () => {
    const rows = rowsOf(4, (i) => [`n${i}`, i * 10]);
    const out = buildQuickChartData(rows, ["NAME", "VAL"], range(1, 3), null);
    expect(out.xKey).toBe("NAME");
    expect(out.valueKeys).toEqual(["VAL"]);
    expect(out.totalRows).toBe(3);
    expect(out.data).toEqual([
      { NAME: "n1", VAL: 10 },
      { NAME: "n2", VAL: 20 },
      { NAME: "n3", VAL: 30 },
    ]);
  });

  it("caps large selections and reports the full row count", () => {
    const rows = rowsOf(50_000, (i) => [`n${i}`, i]);
    const out = buildQuickChartData(rows, ["NAME", "VAL"], range(0, 49_999), null, 100);
    expect(out.data).toHaveLength(100);
    expect(out.totalRows).toBe(50_000);
    expect(out.data[0]).toEqual({ NAME: "n0", VAL: 0 });
    expect(out.data[99]).toEqual({ NAME: "n49999", VAL: 49_999 });
  });

  it("detects a numeric column whose leading rows are null", () => {
    const rows = rowsOf(10_000, (i) => [`n${i}`, i < 5_000 ? null : i]);
    const out = buildQuickChartData(rows, ["NAME", "VAL"], range(0, 9_999), null, 100);
    expect(out.xKey).toBe("NAME");
    expect(out.valueKeys).toEqual(["VAL"]);
  });

  it("follows the visual column order", () => {
    const rows = rowsOf(2, (i) => [i, `n${i}`, i * 2]);
    const out = buildQuickChartData(rows, ["A", "NAME", "B"], range(0, 1, 0, 1), [2, 1, 0]);
    expect(out.xKey).toBe("NAME");
    expect(out.valueKeys).toEqual(["B"]);
  });
});
