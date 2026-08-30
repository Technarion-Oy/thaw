// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi } from "vitest";

// This module calls GetSnowflakeKeywords() at module load time — stub it out
// rather than requiring a browser `window` for this pure-function test.
vi.mock("../../../wailsjs/go/sqleditor/Service", () => ({
  GetSnowflakeKeywords: () => Promise.resolve([]),
}));

import { quoteQualifiedIdent } from "./ObjectNameCaseControl";

describe("quoteQualifiedIdent", () => {
  it("quotes and joins each part", () => {
    expect(quoteQualifiedIdent("DB", "SC", "T")).toBe('"DB"."SC"."T"');
  });

  it("drops blank parts instead of quoting an empty segment", () => {
    expect(quoteQualifiedIdent("DB", "", "T")).toBe('"DB"."T"');
  });

  it("doubles an embedded quote in any part", () => {
    expect(quoteQualifiedIdent(`a"b`, "T")).toBe('"a""b"."T"');
  });
});
