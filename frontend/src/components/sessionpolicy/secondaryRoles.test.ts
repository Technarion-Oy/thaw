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
import { formatRoles, reconcileAll } from "./secondaryRoles";

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

// The inverse parse (DESCRIBE cell → role tokens) now lives in Go as
// snowflake.ParseSecondaryRoles and is covered by TestParseSecondaryRoles /
// TestSecondaryRolesRoundTrip in internal/snowflake/identifiers_test.go.

describe("reconcileAll", () => {
  it("collapses to ALL when ALL is added last", () => {
    expect(reconcileAll(["R1", "R2", "ALL"])).toEqual(["ALL"]);
  });

  it("drops ALL when a named role is added after it", () => {
    expect(reconcileAll(["ALL", "R1"])).toEqual(["R1"]);
    expect(reconcileAll(["ALL", "R1", "R2"])).toEqual(["R1", "R2"]);
  });

  it("leaves a sole ALL untouched", () => {
    expect(reconcileAll(["ALL"])).toEqual(["ALL"]);
  });

  it("leaves named-only lists untouched", () => {
    expect(reconcileAll(["R1", "R2"])).toEqual(["R1", "R2"]);
  });

  it("leaves the empty list untouched", () => {
    expect(reconcileAll([])).toEqual([]);
  });

  it("treats ALL case-insensitively", () => {
    // lowercase "all" added last still collapses to the canonical ALL literal
    expect(reconcileAll(["r1", "all"])).toEqual(["ALL"]);
    // a role added after a lowercase "all" still drops it
    expect(reconcileAll(["all", "r1"])).toEqual(["r1"]);
  });
});
