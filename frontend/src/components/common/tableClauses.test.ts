// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import * as c from "./tableClauses";

describe("tableClauses", () => {
  it("drops columns", () => {
    expect(c.dropColumns(["A"], "CASCADE")).toBe(`DROP COLUMN "A" CASCADE`);
    expect(c.dropColumns(["A", "b c"])).toBe(`DROP COLUMN "A", "b c"`);
    expect(() => c.dropColumns(["A", "B"], "CASCADE")).toThrow();
    expect(() => c.dropColumns([])).toThrow();
  });

  it("adds constraints", () => {
    expect(c.addConstraint("primary key (id)")).toBe("ADD PRIMARY KEY (id)");
    expect(c.addConstraint("CONSTRAINT uq UNIQUE (a, b) not enforced rely")).toBe("ADD CONSTRAINT uq UNIQUE (a, b) NOT ENFORCED RELY");
    expect(c.addConstraint("foreign key (cid) references db.s.cust (id)")).toBe("ADD FOREIGN KEY (cid) REFERENCES db.s.cust (id)");
    expect(c.addConstraint("CONSTRAINT ck CHECK (x > 0)")).toBe("ADD CONSTRAINT ck CHECK (x > 0)");
    expect(() => c.addConstraint("CHECK (x > 0)")).toThrow();
    expect(() => c.addConstraint("FOREIGN KEY (a)")).toThrow();
    expect(() => c.addConstraint("PRIMARY KEY (a); DROP TABLE t")).toThrow();
    expect(() => c.addConstraint("CONSTRAINT ck CHECK (1); DROP TABLE t; (1)")).toThrow();
  });

  it("drops and renames constraints", () => {
    expect(c.dropConstraint("pk_1 cascade")).toBe("DROP CONSTRAINT pk_1 CASCADE");
    expect(c.dropConstraint("PRIMARY KEY")).toBe("DROP PRIMARY KEY");
    expect(c.dropConstraint("unique (a,b) restrict")).toBe("DROP UNIQUE (a, b) RESTRICT");
    expect(() => c.dropConstraint("FOREIGN KEY")).toThrow();
    expect(() => c.dropConstraint("x; y")).toThrow();
    expect(c.alterConstraint("pk_1 not enforced rely")).toBe("ALTER CONSTRAINT pk_1 NOT ENFORCED RELY");
    expect(c.alterConstraint("PRIMARY KEY (id) NOVALIDATE")).toBe("ALTER PRIMARY KEY (id) NOVALIDATE");
    expect(() => c.alterConstraint("pk_1")).toThrow();
    expect(() => c.alterConstraint("pk; x RELY")).toThrow();
    expect(c.renameConstraint("a to b")).toBe("RENAME CONSTRAINT a TO b");
    expect(c.renameConstraint('a "B c"')).toBe('RENAME CONSTRAINT a TO "B c"');
    expect(() => c.renameConstraint("a")).toThrow();
  });

  it("builds data metric function clauses", () => {
    expect(c.addDataMetricFunction("SNOWFLAKE.CORE.NULL_COUNT ON (email)")).toBe("ADD DATA METRIC FUNCTION SNOWFLAKE.CORE.NULL_COUNT ON (email)");
    expect(c.addDataMetricFunction("SNOWFLAKE.CORE.ROW_COUNT ON ()")).toBe("ADD DATA METRIC FUNCTION SNOWFLAKE.CORE.ROW_COUNT ON ()");
    expect(c.addDataMetricFunction("f ON (a) FILTER (a > 1) EXECUTE AS ROLE dq"))
      .toBe("ADD DATA METRIC FUNCTION f ON (a) FILTER (a > 1) EXECUTE AS ROLE dq");
    expect(() => c.addDataMetricFunction("f ON (a) GRANT x")).toThrow();
    expect(() => c.addDataMetricFunction("f ON (a) FILTER (1); DROP TABLE t")).toThrow();
    expect(c.addDataMetricFunction("f ON (a) EXPECTATION nn (VALUE = 0)")).toBe("ADD DATA METRIC FUNCTION f ON (a) EXPECTATION nn (VALUE = 0)");
    expect(c.addDataMetricFunction("db.s.ref ON (cid, TABLE db.s.cust(id))")).toBe("ADD DATA METRIC FUNCTION db.s.ref ON (cid, TABLE db.s.cust(id))");
    expect(c.modifyDataMetricFunctionAction("f ON (a) DROP EXPECTATION nn")).toBe("MODIFY DATA METRIC FUNCTION f ON (a) DROP EXPECTATION nn");
    expect(c.modifyDataMetricFunctionAction("f ON (a) FILTER ()")).toBe("MODIFY DATA METRIC FUNCTION f ON (a) FILTER ()");
    expect(c.modifyDataMetricFunctionAction("f SET DATA_QUALITY_NOTIFICATION = TRUE")).toBe("MODIFY DATA METRIC FUNCTION f SET DATA_QUALITY_NOTIFICATION = TRUE");
    expect(() => c.modifyDataMetricFunctionAction("f ON (a)")).toThrow();
    expect(() => c.modifyDataMetricFunctionAction("f ON (a) GRANT")).toThrow();
    expect(c.dropDataMetricFunction("f ON (a, b)")).toBe("DROP DATA METRIC FUNCTION f ON (a, b)");
    expect(c.modifyDataMetricFunction("f ON (a)", "SUSPEND")).toBe("MODIFY DATA METRIC FUNCTION f ON (a) SUSPEND");
    expect(c.dataMetricSchedule("5 MINUTE")).toBe("SET DATA_METRIC_SCHEDULE = '5 MINUTE'");
    expect(c.dataMetricSchedule("'TRIGGER_ON_CHANGES'")).toBe("SET DATA_METRIC_SCHEDULE = 'TRIGGER_ON_CHANGES'");
    expect(c.dataMetricSchedule(" ")).toBe("UNSET DATA_METRIC_SCHEDULE");
  });

  it("builds governance and masking options", () => {
    expect(c.replaceRowAccessPolicy("old", "db.s.new", "a, b")).toBe("DROP ROW ACCESS POLICY old, ADD ROW ACCESS POLICY db.s.new ON (a, b)");
    expect(c.setJoinPolicy("db.s.jp", true)).toBe("SET JOIN POLICY db.s.jp FORCE");
    expect(c.withMaskingOptions(`ALTER TABLE t ALTER COLUMN "C" SET MASKING POLICY p;`, "C, region", true))
      .toBe(`ALTER TABLE t ALTER COLUMN "C" SET MASKING POLICY p USING (C, region) FORCE;`);
    expect(c.withMaskingOptions("X;", "", false)).toBe("X;");
  });
});
