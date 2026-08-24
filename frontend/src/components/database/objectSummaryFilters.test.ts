// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { KIND_FILTERS, schemaFilters, matchesRowFilter, applyFilters } from "./objectSummaryFilters";
import type { table } from "../../../wailsjs/go/models";

const row = (schema: string) => ({ schema } as table.TableSummary);

const t = (name: string, schema: string, kind: string, rows: number) =>
  ({ name, schema, kind, rows } as table.TableSummary);

const DATA = [
  t("A", "PUBLIC", "BASE TABLE", 10),
  t("B", "PUBLIC", "VIEW", 0),
  t("C", "ANALYTICS", "BASE TABLE", 0),
  t("D", "ANALYTICS", "TRANSIENT", 5),
];

describe("applyFilters", () => {
  const names = (f: Parameters<typeof applyFilters>[1]) => applyFilters(DATA, f).map((r) => r.name);

  it("returns everything when nothing is selected", () => {
    expect(names({})).toEqual(["A", "B", "C", "D"]);
    expect(names({ name: null, kind: [], rows: undefined })).toEqual(["A", "B", "C", "D"]);
  });

  it("ORs values within a column and ANDs across columns", () => {
    expect(names({ kind: ["BASE TABLE", "VIEW"] })).toEqual(["A", "B", "C"]);
    expect(names({ name: ["ANALYTICS"], kind: ["BASE TABLE"] })).toEqual(["C"]);
    expect(names({ name: ["ANALYTICS"], rows: ["nonempty"] })).toEqual(["D"]);
  });

  it("filters empty vs non-empty rows", () => {
    expect(names({ rows: ["empty"] })).toEqual(["B", "C"]);
    expect(names({ rows: ["empty", "nonempty"] })).toEqual(["A", "B", "C", "D"]);
  });
});

describe("KIND_FILTERS", () => {
  it("covers every TABLE_TYPE INFORMATION_SCHEMA.TABLES can return", () => {
    expect(KIND_FILTERS.map((f) => f.value)).toEqual(
      expect.arrayContaining([
        "BASE TABLE",
        "TEMPORARY TABLE",
        "EXTERNAL TABLE",
        "EVENT TABLE",
        "VIEW",
        "MATERIALIZED VIEW",
      ]),
    );
  });
});

describe("schemaFilters", () => {
  it("returns distinct schemas sorted", () => {
    expect(schemaFilters([row("PUBLIC"), row("ANALYTICS"), row("PUBLIC")])).toEqual([
      { text: "ANALYTICS", value: "ANALYTICS" },
      { text: "PUBLIC", value: "PUBLIC" },
    ]);
  });

  it("handles an empty result", () => {
    expect(schemaFilters([])).toEqual([]);
  });
});

describe("matchesRowFilter", () => {
  it("splits empty from non-empty tables", () => {
    expect(matchesRowFilter("empty", 0)).toBe(true);
    expect(matchesRowFilter("empty", 5)).toBe(false);
    expect(matchesRowFilter("nonempty", 0)).toBe(false);
    expect(matchesRowFilter("nonempty", 5)).toBe(true);
  });
});
