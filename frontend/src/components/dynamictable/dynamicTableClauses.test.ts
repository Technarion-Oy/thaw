// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import * as c from "./dynamicTableClauses";

describe("dynamicTableClauses", () => {
  it("validates identifiers", () => {
    expect(c.sqlIdent(' db."My Sc".t ')).toBe('db."My Sc".t');
    expect(() => c.sqlIdent("t; DROP TABLE x")).toThrow();
    expect(() => c.sqlIdent("a.b", 1)).toThrow();
    expect(c.sqlIdentList("a, \"B c\"")).toBe('a, "B c"');
  });

  it("builds table-level clauses", () => {
    expect(c.clusterBy(" a, b ")).toBe("CLUSTER BY (a, b)");
    expect(c.clusterBy("")).toBe("DROP CLUSTERING KEY");
    expect(c.searchOptimization("ADD", "EQUALITY(c1)")).toBe("ADD SEARCH OPTIMIZATION ON EQUALITY(c1)");
    expect(c.searchOptimization("SUSPEND", " ")).toBe("SUSPEND SEARCH OPTIMIZATION");
    expect(c.executeAsUser("svc")).toBe("SET EXECUTE AS USER svc");
    expect(c.executeAsUser("svc use secondary roles all")).toBe("SET EXECUTE AS USER svc USE SECONDARY ROLES ALL");
    expect(c.executeAsUser("svc USE SECONDARY ROLES r1, r2")).toBe("SET EXECUTE AS USER svc USE SECONDARY ROLES r1, r2");
    expect(() => c.executeAsUser("svc; x")).toThrow();
    expect(c.setContact("SUPPORT", "db.s.ops")).toBe("SET CONTACT SUPPORT = db.s.ops");
    expect(() => c.contactPurpose("a b")).toThrow();
    expect(c.addRowAccessPolicy("p", "a,b")).toBe("ADD ROW ACCESS POLICY p ON (a, b)");
    expect(c.setAggregationPolicy("p", "id", true)).toBe("SET AGGREGATION POLICY p ENTITY KEY (id) FORCE");
    expect(c.setAggregationPolicy("p", "", false)).toBe("SET AGGREGATION POLICY p");
    expect(c.addStorageLifecyclePolicy("p", "ts")).toBe("ADD STORAGE LIFECYCLE POLICY p ON (ts)");
  });

  it("builds column-level clauses", () => {
    expect(c.columnComment("Col", "it's")).toBe(`ALTER COLUMN "Col" COMMENT 'it''s'`);
    expect(c.columnComment("Col", " ")).toBe(`ALTER COLUMN "Col" UNSET COMMENT`);
    expect(c.columnMaskingPolicy("C", "db.s.mp USING (C, region) force"))
      .toBe(`ALTER COLUMN "C" SET MASKING POLICY db.s.mp USING (C, region) FORCE`);
    expect(() => c.columnMaskingPolicy("C", "mp; drop")).toThrow();
    expect(c.columnProjectionPolicy("C", "pp")).toBe(`ALTER COLUMN "C" SET PROJECTION POLICY pp`);
    expect(c.unsetColumnPolicy("C", "MASKING")).toBe(`ALTER COLUMN "C" UNSET MASKING POLICY`);
    expect(c.columnSetTag("C", "gov.tags.pii = 'email'")).toBe(`ALTER COLUMN "C" SET TAG gov.tags.pii = 'email'`);
    expect(c.columnUnsetTag("C", "pii")).toBe(`ALTER COLUMN "C" UNSET TAG pii`);
  });
});
