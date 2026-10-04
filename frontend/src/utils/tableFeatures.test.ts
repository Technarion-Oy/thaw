// SPDX-License-Identifier: GPL-3.0-or-later

import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { test, expect } from "vitest";
import { useTable } from "@tanstack/react-table";
import { resultGridFeatures } from "./tableFeatures";
import { columnFilterFn } from "../components/results/ColumnFilterDropdown";

// Fails if a feature ResultGrid relies on is dropped from resultGridFeatures:
// v9 silently ignores state for features that are not registered.
test("resultGridFeatures sorts, filters, pins, orders and sizes", () => {
  let out: unknown;
  function Probe() {
    const table = useTable({
      features: resultGridFeatures,
      data: [[2, "b"], [10, "a"], [1, "c"]] as unknown[][],
      columns: [0, 1].map((i) => ({
        id: `${i}_C`,
        accessorFn: (r: unknown[]) => r[i],
        header: "h",
        filterFn: columnFilterFn,
      })),
      state: {
        sorting: [{ id: "0_C", desc: true }],
        columnFilters: [{ id: "1_C", value: { checkedValues: new Set(["a", "b"]) } }],
        columnPinning: { start: ["1_C"], end: [] },
        columnOrder: ["1_C", "0_C"],
        columnSizing: { "0_C": 77 },
      },
      columnResizeMode: "onChange",
    });
    const rows = table.getRowModel().rows;
    out = {
      rows: rows.map((r) => r.original[0]),
      all: table.getAllLeafColumns().map((c) => c.id),
      start: table.getStartLeafColumns().map((c) => c.id),
      center: table.getCenterLeafColumns().map((c) => c.id),
      size: table.getAllLeafColumns().find((c) => c.id === "0_C")?.getSize(),
      cells: rows[0].getAllCells().length,
      headers: table.getHeaderGroups()[0].headers.length,
    };
    return null;
  }
  renderToString(createElement(Probe));
  expect(out).toEqual({
    rows: [10, 2],
    all: ["1_C", "0_C"],
    start: ["1_C"],
    center: ["0_C"],
    size: 77,
    cells: 2,
    headers: 2,
  });
});
