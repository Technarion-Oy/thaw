// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { KIND_FILTERS, schemaFilters, matchesRowFilter } from "./objectSummaryFilters";
import type { table } from "../../../wailsjs/go/models";

const row = (schema: string) => ({ schema } as table.TableSummary);

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
