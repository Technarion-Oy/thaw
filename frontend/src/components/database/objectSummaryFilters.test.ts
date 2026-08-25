// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { KIND_FILTERS, schemaFilters, matchesRowFilter, applyFilters, registryKind, compareCounts } from "./objectSummaryFilters";
import { KIND_VAR } from "../sidebar/objectIcons";
import { OBJECT_KINDS } from "../../generated/objectKinds";
import type { table } from "../../../wailsjs/go/models";

const row = (schema: string) => ({ schema } as table.TableSummary);

const t = (name: string, schema: string, kind: string, rows: number) =>
  ({ name, schema, kind, rows } as table.TableSummary);

// -1 is the backend's table.UnknownCount: Snowflake reports no ROW_COUNT for views.
const DATA = [
  t("A", "PUBLIC", "BASE TABLE", 10),
  t("B", "PUBLIC", "VIEW", -1),
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
    expect(names({ rows: ["empty"] })).toEqual(["C"]);
    expect(names({ rows: ["nonempty"] })).toEqual(["A", "D"]);
  });

  it("never calls a view empty — it has no row count to judge", () => {
    expect(names({ rows: ["empty", "nonempty"] })).toEqual(["A", "C", "D"]);
    expect(names({ kind: ["VIEW"], rows: ["nonempty"] })).toEqual([]);
    expect(names({ kind: ["VIEW"], rows: ["empty"] })).toEqual([]);
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

  it("matches neither option when the count is unknown", () => {
    expect(matchesRowFilter("empty", -1)).toBe(false);
    expect(matchesRowFilter("nonempty", -1)).toBe(false);
  });
});

describe("compareCounts", () => {
  it("sorts unknown counts after every known value", () => {
    expect([5, -1, 0, 2].sort(compareCounts)).toEqual([0, 2, 5, -1]);
  });

  it("orders known counts numerically", () => {
    expect(compareCounts(1, 2)).toBeLessThan(0);
    expect(compareCounts(2, 1)).toBeGreaterThan(0);
    expect(compareCounts(-1, -1)).toBe(0);
  });
});

describe("registryKind", () => {
  it("maps every filterable TABLE_TYPE onto a canonical object kind with a colour", () => {
    const known = new Set(OBJECT_KINDS.map((k) => k.name));
    for (const { value } of KIND_FILTERS) {
      const kind = registryKind(String(value));
      expect(known).toContain(kind);
      expect(KIND_VAR[kind]).toBeDefined();
    }
  });

  it("folds the table variants onto TABLE and keeps view/external kinds", () => {
    expect(registryKind("BASE TABLE")).toBe("TABLE");
    expect(registryKind("TRANSIENT")).toBe("TABLE");
    expect(registryKind("TEMPORARY TABLE")).toBe("TABLE");
    expect(registryKind("MATERIALIZED VIEW")).toBe("MATERIALIZED VIEW");
    expect(registryKind("EXTERNAL TABLE")).toBe("EXTERNAL TABLE");
  });
});
