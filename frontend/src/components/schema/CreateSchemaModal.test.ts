// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi } from "vitest";

// ObjectNameCaseControl calls GetSnowflakeKeywords() at module load time — stub
// it out rather than requiring a browser `window` for this pure-function test.
vi.mock("../../../wailsjs/go/sqleditor/Service", () => ({
  GetSnowflakeKeywords: () => Promise.resolve([]),
}));

import { buildCreateSchemaSql, type SchemaConfig } from "./CreateSchemaModal";

const base: SchemaConfig = {
  name: "",
  caseSensitive: false,
  orReplace: false,
  ifNotExists: false,
  transient: false,
  cloneSource: "",
  managedAccess: false,
  dataRetention: "",
  comment: "",
  tags: [],
};

describe("buildCreateSchemaSql", () => {
  it("is empty until a name is entered", () => {
    expect(buildCreateSchemaSql("DB", { ...base, name: "  " })).toBe("");
  });

  it("builds the minimal statement qualified by the database", () => {
    expect(buildCreateSchemaSql("my db", { ...base, name: "S1", ifNotExists: true })).toBe(
      'CREATE SCHEMA IF NOT EXISTS "my db".S1;',
    );
  });

  it("emits every option, escapes literals, and drops a non-integer retention", () => {
    expect(
      buildCreateSchemaSql("DB", {
        ...base,
        name: "my-schema",
        orReplace: true,
        ifNotExists: true, // ignored: mutually exclusive with OR REPLACE
        transient: true,
        cloneSource: "SRC",
        managedAccess: true,
        dataRetention: "7",
        comment: "it's",
        tags: [{ name: "GOV.TAGS.cost_center", value: "a'b" }],
      }),
    ).toBe(
      [
        'CREATE OR REPLACE TRANSIENT SCHEMA "DB"."my-schema"',
        '  CLONE "DB"."SRC"',
        "  WITH MANAGED ACCESS",
        "  DATA_RETENTION_TIME_IN_DAYS = 7",
        "  WITH TAG (GOV.TAGS.cost_center = 'a''b')",
        "  COMMENT = 'it''s';",
      ].join("\n"),
    );
    expect(buildCreateSchemaSql("DB", { ...base, name: "S", dataRetention: "1; DROP" })).toBe(
      'CREATE SCHEMA "DB".S;',
    );
  });
});
