// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import type { DataNode } from "antd/es/tree";
import {
  NEW_ITEM_KEY_PREFIX,
  newItemKey,
  isNewItemKey,
  insertSorted,
  findNode,
  childrenOf,
  insertPlaceholder,
  hasSiblingNamed,
  hasExtension,
  finalNewName,
  validateNewName,
} from "./fileTreeUtils";

const dir = (name: string, children?: DataNode[]): DataNode => ({
  key: `/root/${name}`, title: name, isLeaf: false, ...(children ? { children } : {}),
});
const file = (name: string, parent = "/root"): DataNode => ({
  key: `${parent}/${name}`, title: name, isLeaf: true,
});
const names = (nodes: DataNode[]) => nodes.map((n) => String(n.title));

describe("reserved placeholder key", () => {
  it("round-trips the parent directory", () => {
    expect(newItemKey("/root/sub")).toBe(`${NEW_ITEM_KEY_PREFIX}/root/sub`);
    expect(isNewItemKey(newItemKey("/root/sub"))).toBe(true);
  });

  it("never matches a real (absolute) path key", () => {
    expect(isNewItemKey("/root/sub/a.sql")).toBe(false);
    expect(isNewItemKey("C:\\work\\a.sql")).toBe(false);
    expect(isNewItemKey(undefined)).toBe(false);
    expect(isNewItemKey(123)).toBe(false);
  });
});

describe("insertSorted", () => {
  it("keeps directories first, then alphabetical", () => {
    const siblings = [dir("alpha"), dir("zeta"), file("a.sql"), file("z.sql")];
    expect(names(insertSorted(siblings, dir("mid")))).toEqual(["alpha", "mid", "zeta", "a.sql", "z.sql"]);
    expect(names(insertSorted(siblings, file("m.sql")))).toEqual(["alpha", "zeta", "a.sql", "m.sql", "z.sql"]);
  });

  it("does not mutate the input list", () => {
    const siblings = [file("a.sql")];
    insertSorted(siblings, file("b.sql"));
    expect(siblings).toHaveLength(1);
  });
});

describe("insertPlaceholder", () => {
  const placeholderDir: DataNode = { key: newItemKey("/root/sub"), title: "", isLeaf: false };
  const placeholderFile: DataNode = { key: newItemKey("/root/sub"), title: "", isLeaf: true };

  it("inserts at the root when the parent key is null", () => {
    const tree = [dir("alpha"), file("a.sql")];
    const rootPlaceholder: DataNode = { key: newItemKey("/root"), title: "", isLeaf: true };
    expect(names(insertPlaceholder(tree, null, rootPlaceholder))).toEqual(["alpha", "", "a.sql"]);
  });

  it("sorts a folder placeholder ahead of the existing directories", () => {
    const tree = [dir("sub", [dir("alpha"), file("a.sql", "/root/sub")])];
    const kids = insertPlaceholder(tree, "/root/sub", placeholderDir)[0].children!;
    expect(names(kids)).toEqual(["", "alpha", "a.sql"]);
  });

  it("sorts a file placeholder ahead of the existing files but after directories", () => {
    const tree = [dir("sub", [dir("alpha"), file("a.sql", "/root/sub")])];
    const kids = insertPlaceholder(tree, "/root/sub", placeholderFile)[0].children!;
    expect(names(kids)).toEqual(["alpha", "", "a.sql"]);
  });

  it("creates the children array for a directory that has none yet", () => {
    const tree = [dir("sub")];
    const kids = insertPlaceholder(tree, "/root/sub", placeholderFile)[0].children!;
    expect(names(kids)).toEqual([""]);
  });

  it("reaches nested directories and leaves the original tree untouched", () => {
    const tree = [dir("sub", [{ key: "/root/sub/deep", title: "deep", isLeaf: false, children: [] }])];
    const out = insertPlaceholder(tree, "/root/sub/deep", placeholderFile);
    expect(names(out[0].children![0].children!)).toEqual([""]);
    expect(tree[0].children![0].children).toEqual([]);
  });

  it("is a no-op for a parent that no longer exists", () => {
    const tree = [dir("sub", [])];
    expect(insertPlaceholder(tree, "/root/gone", placeholderFile)[0].children).toEqual([]);
  });
});

describe("findNode / childrenOf", () => {
  const tree = [dir("sub", [file("a.sql", "/root/sub")]), file("b.sql")];

  it("finds nested nodes", () => {
    expect(findNode(tree, "/root/sub/a.sql")?.title).toBe("a.sql");
    expect(findNode(tree, "/root/nope")).toBeNull();
  });

  it("returns the top level for the root parent", () => {
    expect(childrenOf(tree, null)).toBe(tree);
  });

  it("returns [] for an unloaded or missing directory", () => {
    expect(childrenOf([dir("sub")], "/root/sub")).toEqual([]);
    expect(childrenOf(tree, "/root/gone")).toEqual([]);
  });
});

describe("hasSiblingNamed", () => {
  const siblings = [dir("Reports"), file("Query.sql"), { key: newItemKey("/root"), title: "", isLeaf: true }];

  it("matches case-insensitively (macOS/Windows filesystems are)", () => {
    expect(hasSiblingNamed(siblings, "query.sql")).toBe(true);
    expect(hasSiblingNamed(siblings, "REPORTS")).toBe(true);
  });

  it("does not match a different name", () => {
    expect(hasSiblingNamed(siblings, "other.sql")).toBe(false);
  });

  it("ignores the placeholder row itself", () => {
    expect(hasSiblingNamed(siblings, "")).toBe(false);
  });
});

describe("hasExtension", () => {
  it("recognizes a real extension", () => {
    expect(hasExtension("report.sql")).toBe(true);
    expect(hasExtension(".env.local")).toBe(true);
  });

  it("treats a dotfile, a bare stem and a trailing dot as extension-less", () => {
    expect(hasExtension(".gitignore")).toBe(false);
    expect(hasExtension("report")).toBe(false);
    expect(hasExtension("report.")).toBe(false);
  });
});

describe("finalNewName", () => {
  it("appends .sql only to a bare stem", () => {
    expect(finalNewName(" report ", "newFile")).toBe("report.sql");
  });

  it("keeps whatever extension the user typed — any file type is creatable", () => {
    expect(finalNewName("report.sql", "newFile")).toBe("report.sql");
    expect(finalNewName("report.SQL", "newFile")).toBe("report.SQL");
    expect(finalNewName("schema.yml", "newFile")).toBe("schema.yml");
    expect(finalNewName("README.md", "newFile")).toBe("README.md");
    expect(finalNewName("archive.tar.gz", "newFile")).toBe("archive.tar.gz");
  });

  it("leaves dotfiles alone", () => {
    expect(finalNewName(".gitignore", "newFile")).toBe(".gitignore");
    expect(finalNewName(".env.local", "newFile")).toBe(".env.local");
  });

  it("leaves folder names alone", () => {
    expect(finalNewName(" reports ", "newFolder")).toBe("reports");
    expect(finalNewName("v1.2", "newFolder")).toBe("v1.2");
  });
});

describe("validateNewName", () => {
  const siblings = [dir("Reports"), file("Query.sql")];

  it("accepts a fresh name", () => {
    expect(validateNewName("fresh", "newFile", siblings)).toBeNull();
    expect(validateNewName("fresh", "newFolder", siblings)).toBeNull();
  });

  it("rejects an empty name", () => {
    expect(validateNewName("   ", "newFolder", siblings)).toMatch(/folder name must be provided/);
    expect(validateNewName("", "newFile", siblings)).toMatch(/file name must be provided/);
  });

  it("rejects path separators and Windows-invalid characters", () => {
    expect(validateNewName("a/b", "newFile", siblings)).toMatch(/path separators/);
    expect(validateNewName("a\\b", "newFile", siblings)).toMatch(/path separators/);
    for (const c of [":", '"', "*", "?", "<", ">", "|"]) {
      expect(validateNewName(`a${c}b`, "newFile", siblings)).toMatch(/cannot contain/);
    }
  });

  it("rejects the relative directory names", () => {
    expect(validateNewName(".", "newFolder", siblings)).toMatch(/"\." or "\.\."/);
    expect(validateNewName("..", "newFolder", siblings)).toMatch(/"\." or "\.\."/);
  });

  it("rejects a trailing dot (Windows strips it, and it names no extension)", () => {
    expect(validateNewName("report.", "newFile", siblings)).toMatch(/cannot end with/);
  });

  it("accepts any file type the user spells out", () => {
    for (const n of ["schema.yml", "README.md", ".gitignore", "archive.tar.gz"]) {
      expect(validateNewName(n, "newFile", siblings)).toBeNull();
    }
  });

  it("rejects a duplicate sibling, including one differing only by case", () => {
    expect(validateNewName("Reports", "newFolder", siblings)).toMatch(/already exists/);
    expect(validateNewName("reports", "newFolder", siblings)).toMatch(/already exists/);
  });

  it("compares the duplicate against the .sql-appended name", () => {
    // "query" becomes "query.sql", which collides with the existing "Query.sql".
    expect(validateNewName("query", "newFile", siblings)).toMatch(/"query\.sql" already exists/);
    // As a folder there is no append, so no collision.
    expect(validateNewName("query", "newFolder", siblings)).toBeNull();
  });
});
