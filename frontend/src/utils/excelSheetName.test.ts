// SPDX-License-Identifier: GPL-3.0-or-later

// Unit tests for Excel worksheet-name derivation: the 31-char cap, invalid-char
// stripping, empty-SQL fallback, and case-insensitive de-duplication.

import { describe, expect, it } from "vitest";
import { deriveSheetName, MAX_SHEET_NAME } from "./excelSheetName";

describe("deriveSheetName", () => {
  it("collapses whitespace and keeps short queries intact", () => {
    const used = new Set<string>();
    expect(deriveSheetName("SELECT   a\nFROM t", 1, used)).toBe("SELECT a FROM t");
  });

  it("strips the characters Excel forbids in sheet names", () => {
    const used = new Set<string>();
    // [ ] : * ? / \ are all invalid — each becomes a space, then collapsed.
    expect(deriveSheetName("SELECT a/b, c:d [x] *?\\ e", 1, used)).toBe("SELECT a b, c d x e");
  });

  it("caps names at 31 characters", () => {
    const used = new Set<string>();
    const name = deriveSheetName("SELECT column_one, column_two, column_three FROM t", 1, used);
    expect(name.length).toBeLessThanOrEqual(MAX_SHEET_NAME);
    expect(name).toBe("SELECT column_one, column_two,".slice(0, MAX_SHEET_NAME));
  });

  it("falls back to Result N for empty or all-invalid SQL", () => {
    const used = new Set<string>();
    expect(deriveSheetName("", 3, used)).toBe("Result 3");
    expect(deriveSheetName("   ", 4, used)).toBe("Result 4");
    expect(deriveSheetName("///", 5, used)).toBe("Result 5");
  });

  it("de-duplicates case-insensitively with numbered suffixes", () => {
    const used = new Set<string>();
    expect(deriveSheetName("SELECT 1", 1, used)).toBe("SELECT 1");
    expect(deriveSheetName("select 1", 2, used)).toBe("select 1 (2)");
    expect(deriveSheetName("SELECT 1", 3, used)).toBe("SELECT 1 (3)");
  });

  it("keeps de-duplicated names within the 31-char limit", () => {
    const used = new Set<string>();
    const long = "SELECT alpha, bravo, charlie, delta FROM sometable";
    const first = deriveSheetName(long, 1, used);
    const second = deriveSheetName(long, 2, used);
    expect(first.length).toBeLessThanOrEqual(MAX_SHEET_NAME);
    expect(second.length).toBeLessThanOrEqual(MAX_SHEET_NAME);
    expect(second).toMatch(/ \(2\)$/);
  });
});
