// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { quoteIdent, nameOptionsFromShow, userTagsToEditable, parseMfaMethods } from "./userPropertyUtils";
import type { snowflake } from "../../../wailsjs/go/models";

// Minimal QueryResult shim — only columns/rows are read by the parsers.
const qr = (columns: string[], rows: unknown[][]): snowflake.QueryResult =>
  ({ columns, rows } as unknown as snowflake.QueryResult);

describe("quoteIdent", () => {
  it("wraps in double quotes and doubles embedded quotes", () => {
    expect(quoteIdent("ALICE")).toBe(`"ALICE"`);
    expect(quoteIdent(`My"Tag`)).toBe(`"My""Tag"`);
  });
});

describe("nameOptionsFromShow", () => {
  it("builds quoted FQN value + readable label from SHOW POLICIES/TAGS columns", () => {
    const res = qr(
      ["name", "database_name", "schema_name"],
      [["POL", "DB", "SEC"], ["Mixed", "DB", "SEC"]],
    );
    expect(nameOptionsFromShow(res)).toEqual([
      { value: `"DB"."SEC"."POL"`, label: "DB.SEC.POL", allowedValues: [] },
      { value: `"DB"."SEC"."Mixed"`, label: "DB.SEC.Mixed", allowedValues: [] },
    ]);
  });

  it("parses allowed_values into a whitelist", () => {
    const res = qr(
      ["name", "database_name", "schema_name", "allowed_values"],
      [["T", "DB", "SEC", '["a","b"]'], ["U", "DB", "SEC", null]],
    );
    const opts = nameOptionsFromShow(res);
    expect(opts[0].allowedValues).toEqual(["a", "b"]);
    expect(opts[1].allowedValues).toEqual([]);
  });

  it("returns [] when the name column is absent", () => {
    expect(nameOptionsFromShow(qr(["other"], [["x"]]))).toEqual([]);
    expect(nameOptionsFromShow(null)).toEqual([]);
  });

  it("skips blank database/schema parts (account-level names)", () => {
    const res = qr(["name", "database_name", "schema_name"], [["BARE", "", ""]]);
    expect(nameOptionsFromShow(res)[0]).toEqual({ value: `"BARE"`, label: "BARE", allowedValues: [] });
  });
});

describe("userTagsToEditable", () => {
  it("maps TAG_* columns to removable chips with a quoted FQN key", () => {
    const res = qr(
      ["TAG_DATABASE", "TAG_SCHEMA", "TAG_NAME", "TAG_VALUE"],
      [["DB", "SEC", "COST_CENTER", "eng"]],
    );
    expect(userTagsToEditable(res)).toEqual([
      { key: `"DB"."SEC"."COST_CENTER"`, name: "COST_CENTER", value: "eng", removable: true },
    ]);
  });

  it("tolerates null value and missing name column", () => {
    const res = qr(["TAG_DATABASE", "TAG_SCHEMA", "TAG_NAME", "TAG_VALUE"], [["DB", "SEC", "T", null]]);
    expect(userTagsToEditable(res)[0].value).toBe("");
    expect(userTagsToEditable(qr(["x"], [["y"]]))).toEqual([]);
  });
});

describe("parseMfaMethods", () => {
  it("maps SHOW MFA METHODS rows, keeping name as the removal identifier", () => {
    const res = qr(
      ["name", "type", "comment", "last_used", "created_on"],
      [["MFA_1", "TOTP", "phone", "2026-01-01", "2025-01-01"], ["MFA_2", "DUO", null, null, null]],
    );
    expect(parseMfaMethods(res)).toEqual([
      { name: "MFA_1", type: "TOTP", comment: "phone", lastUsed: "2026-01-01" },
      { name: "MFA_2", type: "DUO", comment: "", lastUsed: "" },
    ]);
  });

  it("drops rows with an empty name and handles a missing name column", () => {
    expect(parseMfaMethods(qr(["name", "type"], [["", "TOTP"]]))).toEqual([]);
    expect(parseMfaMethods(qr(["type"], [["TOTP"]]))).toEqual([]);
    expect(parseMfaMethods(null)).toEqual([]);
  });
});
