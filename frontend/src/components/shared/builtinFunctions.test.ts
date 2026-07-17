// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from "vitest";
import {
  DEFAULT_FUNCTIONS,
  DEFAULT_FUNCTION_CATEGORIES,
  filterDefaultFunctions,
} from "./builtinFunctions";

describe("DEFAULT_FUNCTIONS catalog", () => {
  it("tags every function with a known category", () => {
    for (const f of DEFAULT_FUNCTIONS) {
      expect(DEFAULT_FUNCTION_CATEGORIES).toContain(f.category);
    }
  });

  it("has unique SQL snippets (used as React keys)", () => {
    const sqls = DEFAULT_FUNCTIONS.map((f) => f.sql);
    expect(new Set(sqls).size).toBe(sqls.length);
  });
});

describe("filterDefaultFunctions", () => {
  it("returns every function grouped in category order for a blank query", () => {
    const groups = filterDefaultFunctions("");
    expect(groups.map((g) => g.category)).toEqual([...DEFAULT_FUNCTION_CATEGORIES]);
    const total = groups.reduce((n, g) => n + g.fns.length, 0);
    expect(total).toBe(DEFAULT_FUNCTIONS.length);
  });

  it("treats a whitespace-only query as blank", () => {
    expect(filterDefaultFunctions("   ")).toEqual(filterDefaultFunctions(""));
  });

  it("matches case-insensitively by function name", () => {
    const groups = filterDefaultFunctions("uuid");
    const names = groups.flatMap((g) => g.fns.map((f) => f.name));
    expect(names).toEqual(["UUID_STRING"]);
  });

  it("matches against the SQL snippet, not just the name", () => {
    // UNIX_TIMESTAMP's snippet is DATE_PART(EPOCH_SECOND, …) — no "epoch" in its name.
    const names = filterDefaultFunctions("epoch").flatMap((g) => g.fns.map((f) => f.name));
    expect(names).toContain("UNIX_TIMESTAMP");
  });

  it("matches against the description", () => {
    const names = filterDefaultFunctions("warehouse").flatMap((g) => g.fns.map((f) => f.name));
    expect(names).toContain("CURRENT_WAREHOUSE");
  });

  it("omits categories with no matches", () => {
    const groups = filterDefaultFunctions("uuid");
    expect(groups).toHaveLength(1);
    expect(groups[0].category).toBe("Identifiers & Misc");
  });

  it("returns an empty array when nothing matches", () => {
    expect(filterDefaultFunctions("nonexistent_zzz")).toEqual([]);
  });

  it("narrows to a single match for a unique substring (Enter-to-pick path)", () => {
    const groups = filterDefaultFunctions("uuid_string");
    const flat = groups.flatMap((g) => g.fns);
    expect(flat).toHaveLength(1);
    expect(flat[0].sql).toBe("UUID_STRING()");
  });
});
