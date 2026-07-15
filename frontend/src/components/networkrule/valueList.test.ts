// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from "vitest";
import { parseValueList, setValueListClause, q1 } from "./valueList";

describe("parseValueList", () => {
  it("parses a bare comma-separated string", () => {
    expect(parseValueList("example.com:443,api.example.com:443")).toEqual([
      "example.com:443",
      "api.example.com:443",
    ]);
  });

  it("parses a single bare value", () => {
    expect(parseValueList("192.168.1.0/24")).toEqual(["192.168.1.0/24"]);
  });

  it("parses a JSON array (compact)", () => {
    expect(parseValueList('["example.com:443","company.com:80"]')).toEqual([
      "example.com:443",
      "company.com:80",
    ]);
  });

  it("parses a JSON array (pretty-printed / multiline)", () => {
    const raw = '[\n  "example.com:443",\n  "company.com:80"\n]';
    expect(parseValueList(raw)).toEqual(["example.com:443", "company.com:80"]);
  });

  it("trims whitespace and drops empty entries", () => {
    expect(parseValueList(" a , , b ,")).toEqual(["a", "b"]);
  });

  it("treats empty / null cells as an empty list", () => {
    expect(parseValueList("")).toEqual([]);
    expect(parseValueList("   ")).toEqual([]);
    expect(parseValueList("null")).toEqual([]);
    expect(parseValueList("NULL")).toEqual([]);
  });

  it("preserves commas inside a JSON array element (e.g. opaque resource paths)", () => {
    // A bare comma-split would mangle this; the JSON path keeps it intact.
    const raw = '["/subscriptions/abc/resourceGroups/rg,extra/privateEndpoints/pe"]';
    expect(parseValueList(raw)).toEqual([
      "/subscriptions/abc/resourceGroups/rg,extra/privateEndpoints/pe",
    ]);
  });
});

describe("setValueListClause", () => {
  it("builds a SET clause that replaces the whole list", () => {
    expect(setValueListClause(["a", "b"])).toBe("SET VALUE_LIST = ('a', 'b')");
  });

  it("maps an empty list to UNSET", () => {
    expect(setValueListClause([])).toBe("UNSET VALUE_LIST");
  });

  it("escapes single quotes in values", () => {
    expect(setValueListClause(["o'hare.example.com:443"])).toBe(
      "SET VALUE_LIST = ('o''hare.example.com:443')",
    );
  });
});

describe("q1", () => {
  it("single-quotes and doubles embedded quotes", () => {
    expect(q1("a'b")).toBe("'a''b'");
  });
});

describe("round-trip (parse → edit → re-serialize)", () => {
  // The headline risk the review flagged: parsing a DESCRIBE output then
  // re-serializing on the next edit must not corrupt the rule. Verify both
  // documented-possible input formats survive a parse + add-one-value round-trip.
  it("survives a JSON-array round-trip", () => {
    const values = parseValueList('["example.com:443","company.com:80"]');
    const clause = setValueListClause([...values, "new.example.com:443"]);
    expect(clause).toBe(
      "SET VALUE_LIST = ('example.com:443', 'company.com:80', 'new.example.com:443')",
    );
  });

  it("survives a bare comma-separated round-trip", () => {
    const values = parseValueList("example.com:443,company.com:80");
    const clause = setValueListClause(values.filter((v) => v !== "company.com:80"));
    expect(clause).toBe("SET VALUE_LIST = ('example.com:443')");
  });
});
