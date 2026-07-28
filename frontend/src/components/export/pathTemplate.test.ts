// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  DEFAULT_EXPORT_PATH_TEMPLATE,
  PATH_TEMPLATE_VARIABLES,
  applyTemplate,
  insertPlaceholder,
  validateTemplate,
} from "./pathTemplate";

describe("PATH_TEMPLATE_VARIABLES", () => {
  it("offers exactly the placeholders (*ddl.Object).FilePathFor substitutes", () => {
    expect(PATH_TEMPLATE_VARIABLES.map((v) => v.name)).toEqual([
      "{database}",
      "{schema}",
      "{object_type}",
      "{object_name}",
    ]);
  });

  it("covers every placeholder used by the default template", () => {
    const rendered = applyTemplate(DEFAULT_EXPORT_PATH_TEMPLATE);
    expect(rendered).not.toMatch(/[{}]/);
  });
});

describe("applyTemplate", () => {
  it("substitutes example values", () => {
    expect(applyTemplate(DEFAULT_EXPORT_PATH_TEMPLATE)).toBe(
      "MY_DATABASE/PUBLIC/tables/MY_TABLE.sql",
    );
  });

  it("falls back to the default for an empty or blank template", () => {
    const expected = applyTemplate(DEFAULT_EXPORT_PATH_TEMPLATE);
    expect(applyTemplate("")).toBe(expected);
    expect(applyTemplate("   ")).toBe(expected);
  });

  it("substitutes every occurrence and leaves literal text alone", () => {
    expect(applyTemplate("sql/{database}/{database}-{object_name}.sql")).toBe(
      "sql/MY_DATABASE/MY_DATABASE-MY_TABLE.sql",
    );
  });

  it("leaves an unknown placeholder untouched", () => {
    expect(applyTemplate("{database}/{bogus}.sql")).toBe("MY_DATABASE/{bogus}.sql");
  });
});

describe("validateTemplate", () => {
  it("accepts the default template", () => {
    expect(validateTemplate(DEFAULT_EXPORT_PATH_TEMPLATE)).toBeNull();
  });

  it("accepts a blank template — it falls back to the default, which ends in .sql", () => {
    expect(validateTemplate("")).toBeNull();
    expect(validateTemplate("   ")).toBeNull();
  });

  it("accepts any template ending in .sql, whatever its shape", () => {
    for (const t of [
      "{object_name}.sql",
      "sql/{database}/{schema}/{object_name}.sql",
      "{database}/{object_name}_v1.sql",
      "  {object_name}.sql  ",
    ]) {
      expect(validateTemplate(t)).toBeNull();
    }
  });

  it("accepts the extension case-insensitively", () => {
    expect(validateTemplate("{object_name}.SQL")).toBeNull();
  });

  it("rejects a template with no extension", () => {
    expect(validateTemplate(DEFAULT_EXPORT_PATH_TEMPLATE.replace(".sql", ""))).toMatch(/\.sql/);
    expect(validateTemplate("{database}/{object_name}")).toMatch(/\.sql/);
  });

  it("rejects a different extension, and .sql anywhere but the end", () => {
    expect(validateTemplate("{object_name}.txt")).not.toBeNull();
    expect(validateTemplate("{object_name}.sql.bak")).not.toBeNull();
    expect(validateTemplate("sql/{object_name}")).not.toBeNull();
  });

  it("does not fix the template up — .sql.sql is the user's call, not a silent append", () => {
    expect(validateTemplate("{object_name}.sql.sql")).toBeNull();
  });
});

describe("insertPlaceholder", () => {
  it("inserts at the caret and reports the caret after the insertion", () => {
    const r = insertPlaceholder("ab/cd", "{schema}", 3, 3);
    expect(r.value).toBe("ab/{schema}cd");
    expect(r.caret).toBe(3 + "{schema}".length);
  });

  it("replaces the selected range", () => {
    const r = insertPlaceholder("{database}/PUBLIC/x.sql", "{schema}", 11, 17);
    expect(r.value).toBe("{database}/{schema}/x.sql");
    expect(r.caret).toBe(19);
  });

  it("appends when there is no caret (the input is not focused)", () => {
    const r = insertPlaceholder("a/b", "{schema}", null, null);
    expect(r.value).toBe("a/b{schema}");
    expect(r.caret).toBe("a/b{schema}".length);
  });

  it("builds a template from empty without seeding the default", () => {
    const r = insertPlaceholder("", "{database}", null, null);
    expect(r.value).toBe("{database}");
    expect(r.caret).toBe("{database}".length);
  });

  it("clamps offsets past the end of a shortened value", () => {
    const r = insertPlaceholder("ab", "{x}", 99, 99);
    expect(r.value).toBe("ab{x}");
    expect(r.caret).toBe(5);
  });

  it("clamps a negative offset to the start", () => {
    expect(insertPlaceholder("ab", "{x}", -5, -5).value).toBe("{x}ab");
  });

  it("treats a backwards selection as a plain caret", () => {
    const r = insertPlaceholder("abcd", "{x}", 3, 1);
    expect(r.value).toBe("abc{x}d");
    expect(r.caret).toBe(6);
  });

  it("defaults the end of the selection to its start", () => {
    expect(insertPlaceholder("abcd", "{x}", 2).value).toBe("ab{x}cd");
  });
});
