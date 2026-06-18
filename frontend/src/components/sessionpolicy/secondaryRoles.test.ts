// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { describe, expect, it } from "vitest";
import { formatRoles, parseRoles } from "./secondaryRoles";

// Char-only stand-in for the shared needsQuoting the modal injects at runtime:
// quote anything that isn't a valid bare identifier, plus a sample reserved word
// (ORDER) so the reserved-keyword branch is exercised without a Wails runtime.
const SIMPLE_IDENT = /^[A-Za-z_][A-Za-z0-9_$]*$/;
const RESERVED = new Set(["ORDER", "SELECT"]);
const needsQuoting = (n: string) => !SIMPLE_IDENT.test(n) || RESERVED.has(n.toUpperCase());

describe("formatRoles", () => {
  it("renders ALL as the quoted literal (case-insensitive)", () => {
    expect(formatRoles(["ALL"], needsQuoting)).toBe("('ALL')");
    expect(formatRoles(["all"], needsQuoting)).toBe("('ALL')");
  });

  it("emits simple identifiers bare", () => {
    expect(formatRoles(["R1", "R2"], needsQuoting)).toBe("(R1, R2)");
  });

  it("emits lowercase bare (Snowflake uppercases on resolution)", () => {
    expect(formatRoles(["analyst"], needsQuoting)).toBe("(analyst)");
  });

  it("double-quotes a role needing quoting", () => {
    expect(formatRoles(["my role"], needsQuoting)).toBe('("my role")');
  });

  it("double-quotes a reserved-keyword role (mirrors the backend)", () => {
    expect(formatRoles(["ORDER"], needsQuoting)).toBe('("ORDER")');
  });

  it("escapes embedded double-quotes", () => {
    expect(formatRoles(['we"ird'], needsQuoting)).toBe('("we""ird")');
  });

  it("skips blank entries and trims", () => {
    expect(formatRoles(["", "  ", " R1 "], needsQuoting)).toBe("(R1)");
  });

  it("renders an empty list as ()", () => {
    expect(formatRoles([], needsQuoting)).toBe("()");
  });
});

describe("parseRoles", () => {
  it("parses a SQL tuple with the ALL literal", () => {
    expect(parseRoles("('ALL')")).toEqual(["ALL"]);
  });

  it("parses a SQL tuple of bare identifiers", () => {
    expect(parseRoles("(R1, R2)")).toEqual(["R1", "R2"]);
  });

  it("parses a SQL tuple mixing bare and quoted entries", () => {
    expect(parseRoles('(R1, "my role")')).toEqual(["R1", "my role"]);
  });

  it("parses a JSON-style array (the form many list columns use)", () => {
    expect(parseRoles('["ALL"]')).toEqual(["ALL"]);
    expect(parseRoles('["R1","R2"]')).toEqual(["R1", "R2"]);
  });

  it("treats empty / null / empty-tuple as no roles", () => {
    expect(parseRoles("")).toEqual([]);
    expect(parseRoles("null")).toEqual([]);
    expect(parseRoles("()")).toEqual([]);
    expect(parseRoles("[]")).toEqual([]);
  });

  it("round-trips formatRoles output", () => {
    for (const roles of [["ALL"], ["R1", "R2"], ["analyst"]]) {
      expect(parseRoles(formatRoles(roles, needsQuoting))).toEqual(
        roles.map((r) => (r.toUpperCase() === "ALL" ? "ALL" : r)),
      );
    }
  });
});
