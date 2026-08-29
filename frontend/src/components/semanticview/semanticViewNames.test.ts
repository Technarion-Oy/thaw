// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { quoteFqn, parseQuotedFqn } from "./semanticViewNames";

describe("quoteFqn", () => {
  it("quotes each part", () => {
    expect(quoteFqn("DB", "SC", "ORDERS")).toBe(`"DB"."SC"."ORDERS"`);
  });

  it("doubles embedded quotes", () => {
    expect(quoteFqn("DB", "SC", 'we"ird')).toBe(`"DB"."SC"."we""ird"`);
  });

  it("yields an empty reference when no object is picked", () => {
    expect(quoteFqn("DB", "SC", "")).toBe("");
  });
});

describe("parseQuotedFqn", () => {
  it("round-trips a plain reference", () => {
    expect(parseQuotedFqn(quoteFqn("OTHER_DB", "SC", "SEARCH")))
      .toEqual({ db: "OTHER_DB", schema: "SC", name: "SEARCH" });
  });

  it("round-trips names containing quotes and dots", () => {
    expect(parseQuotedFqn(quoteFqn("DB", "MY.SC", 'we"ird')))
      .toEqual({ db: "DB", schema: "MY.SC", name: 'we"ird' });
  });

  it("returns blanks for a reference that isn't three quoted parts", () => {
    const blank = { db: "", schema: "", name: "" };
    expect(parseQuotedFqn("")).toEqual(blank);
    expect(parseQuotedFqn("DB.SC.NAME")).toEqual(blank);
    expect(parseQuotedFqn(`"SC"."NAME"`)).toEqual(blank);
  });
});
