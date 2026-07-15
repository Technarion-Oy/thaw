// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from "vitest";
import { createTableClause, tableOptionsClauses } from "./tableDdl";

describe("createTableClause", () => {
  it("falls back to the historical IF NOT EXISTS when no options are given", () => {
    expect(createTableClause()).toBe("CREATE TABLE IF NOT EXISTS");
  });

  it("emits OR REPLACE + table type and drops IF NOT EXISTS when replacing", () => {
    expect(createTableClause({ orReplace: true, ifNotExists: true, tableType: "TRANSIENT" }))
      .toBe("CREATE OR REPLACE TRANSIENT TABLE");
  });

  it("keeps IF NOT EXISTS for a plain permanent table", () => {
    expect(createTableClause({ ifNotExists: true, tableType: "PERMANENT" }))
      .toBe("CREATE TABLE IF NOT EXISTS");
  });
});

describe("tableOptionsClauses", () => {
  it("returns nothing when options are absent", () => {
    expect(tableOptionsClauses()).toEqual([]);
  });

  it("emits set clauses in Snowflake order and skips blanks/zeros-as-empty", () => {
    expect(
      tableOptionsClauses({
        clusterBy: "col1, col2",
        enableSchemaEvolution: true,
        dataRetentionTimeInDays: 7,
        maxDataExtensionTimeInDays: "",
        changeTracking: true,
        comment: "it's fine",
      }),
    ).toEqual([
      "CLUSTER BY (col1, col2)",
      "ENABLE_SCHEMA_EVOLUTION = TRUE",
      "DATA_RETENTION_TIME_IN_DAYS = 7",
      "CHANGE_TRACKING = TRUE",
      "COMMENT = 'it''s fine'",
    ]);
  });
});
