// SPDX-License-Identifier: GPL-3.0-or-later

import { expect, test } from "vitest";
import { parseNotices } from "./parseNotices";

// A fixture mirroring the exact shape gen_third_party_notices.go emits: intro
// prose (hard-wrapped, with a blank-line-separated second paragraph), a skipped
// admonition/contents block, two groups, a package with a fenced license text,
// a package with the prose "no license file" fallback, and a package bundled at
// two versions.
const FIXTURE = [
  "# Third-Party Notices & Acknowledgements",
  "",
  "Thaw is built on the work of many open-source projects. This file lists",
  "every third-party package that ships inside the Thaw binary.",
  "",
  "Thaw itself is free software licensed under the GNU GPL v3.0 or later.",
  "",
  "> **This file is generated.** Do not edit it by hand.",
  "",
  "## Contents",
  "",
  "- [Backend — Go modules](#backend--go-modules) (1)",
  "- [Frontend — npm packages](#frontend--npm-packages) (3)",
  "",
  "## Backend — Go modules",
  "",
  "| Package | Version | License |",
  "|---------|---------|---------|",
  "| `example.com/mod` | v1.2.3 | MIT |",
  "",
  "### example.com/mod",
  "",
  "- **Version:** v1.2.3",
  "- **License:** MIT",
  "",
  "```",
  "Copyright (c) 2021 Someone",
  "",
  "Permission is hereby granted...",
  "```",
  "",
  "## Frontend — npm packages",
  "",
  "| Package | Version | License |",
  "|---------|---------|---------|",
  "| `no-license-pkg` | 2.0.0 | MIT |",
  "| `zustand` | 4.5.7 | MIT |",
  "| `zustand` | 5.0.11 | MIT |",
  "",
  "### no-license-pkg",
  "",
  "- **Version:** 2.0.0",
  "- **License:** MIT",
  "",
  "_No license file was found in the distributed package. Refer to the project's repository for its license terms._",
  "",
  "### zustand",
  "",
  "- **Version:** 4.5.7",
  "- **License:** MIT",
  "",
  "```",
  "MIT License — zustand 4",
  "```",
  "",
  "### zustand",
  "",
  "- **Version:** 5.0.11",
  "- **License:** MIT",
  "",
  "```",
  "MIT License — zustand 5",
  "```",
  "",
].join("\n");

test("parses groups and packages from the generated shape", () => {
  const { groups } = parseNotices(FIXTURE);
  // The empty "Contents" table-of-contents section produces a group with no
  // packages; the modal filters those out, but the parser keeps it, so we assert
  // on the two real package groups explicitly.
  const backend = groups.find((g) => g.title.startsWith("Backend"));
  const frontend = groups.find((g) => g.title.startsWith("Frontend"));
  expect(backend?.packages).toHaveLength(1);
  expect(frontend?.packages).toHaveLength(3);

  const contents = groups.find((g) => g.title === "Contents");
  expect(contents?.packages).toHaveLength(0);
});

test("captures version, license, and fenced license text for a normal package", () => {
  const { groups } = parseNotices(FIXTURE);
  const mod = groups.flatMap((g) => g.packages).find((p) => p.name === "example.com/mod");
  expect(mod).toBeDefined();
  expect(mod?.version).toBe("v1.2.3");
  expect(mod?.license).toBe("MIT");
  expect(mod?.text).toBe("Copyright (c) 2021 Someone\n\nPermission is hereby granted...");
});

test("uses the prose fallback as text when no license file was bundled", () => {
  const { groups } = parseNotices(FIXTURE);
  const pkg = groups.flatMap((g) => g.packages).find((p) => p.name === "no-license-pkg");
  expect(pkg?.text).toContain("No license file was found");
  // Markdown emphasis underscores are stripped.
  expect(pkg?.text).not.toContain("_");
});

test("keeps a package bundled at two versions as distinct entries", () => {
  const { groups } = parseNotices(FIXTURE);
  const zustands = groups.flatMap((g) => g.packages).filter((p) => p.name === "zustand");
  expect(zustands.map((p) => p.version).sort()).toEqual(["4.5.7", "5.0.11"]);
  expect(zustands.find((p) => p.version === "4.5.7")?.text).toBe("MIT License — zustand 4");
  expect(zustands.find((p) => p.version === "5.0.11")?.text).toBe("MIT License — zustand 5");
});

test("handles a longer fence around a license text that contains a ``` line", () => {
  // Mirrors what gen_third_party_notices.go's fenceFor emits: a 4-backtick fence
  // because the license text itself contains a bare 3-backtick line. A naive
  // three-backtick parser would close the block early and truncate the text.
  const md = [
    "## Frontend — npm packages",
    "",
    "### fenced-pkg",
    "",
    "- **Version:** 1.0.0",
    "- **License:** MIT",
    "",
    "````",
    "Example usage:",
    "```",
    "code sample inside the license",
    "```",
    "End of notice.",
    "````",
    "",
  ].join("\n");
  const { groups } = parseNotices(md);
  const pkg = groups.flatMap((g) => g.packages).find((p) => p.name === "fenced-pkg");
  expect(pkg?.version).toBe("1.0.0");
  expect(pkg?.text).toBe(
    "Example usage:\n```\ncode sample inside the license\n```\nEnd of notice.",
  );
});

test("joins hard-wrapped intro prose into blank-line-separated paragraphs", () => {
  const { intro } = parseNotices(FIXTURE);
  expect(intro).toHaveLength(2);
  expect(intro[0]).toBe(
    "Thaw is built on the work of many open-source projects. This file lists every third-party package that ships inside the Thaw binary.",
  );
  expect(intro[1]).toBe("Thaw itself is free software licensed under the GNU GPL v3.0 or later.");
  // The generated-file admonition (blockquote) and the contents list are excluded.
  expect(intro.join(" ")).not.toContain("generated");
});
