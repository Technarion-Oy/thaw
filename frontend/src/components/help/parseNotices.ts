// SPDX-License-Identifier: GPL-3.0-or-later

export interface Pkg {
  name: string;
  version: string;
  license: string;
  text: string;
}

export interface Group {
  title: string;
  packages: Pkg[];
}

export interface Parsed {
  intro: string[];
  groups: Group[];
}

// parseNotices turns the generated THIRD_PARTY_NOTICES.md into structured groups
// of packages. It relies only on the shapes gen_third_party_notices.go emits:
// `## <group>` headers, a summary table, then one `### <name>` section per
// package with `- **Version:**` / `- **License:**` bullets and a fenced license
// text block. Anything it doesn't recognise falls back to the raw intro text.
export function parseNotices(md: string): Parsed {
  const lines = md.split("\n");
  // introRaw keeps blank lines so hard-wrapped prose can be re-joined into
  // paragraphs (blank line = paragraph break) below.
  const introRaw: string[] = [];
  const groups: Group[] = [];
  let group: Group | null = null;
  let pkg: Pkg | null = null;
  // fence is the opening delimiter (e.g. "```") while inside a code block, or
  // null otherwise. The generator uses a delimiter longer than any backtick run
  // in a license text, so the close must be at least as long as the open — track
  // the exact opening string rather than assuming three backticks.
  let fence: string | null = null;
  const fenceBuf: string[] = [];

  const flushPkg = () => {
    if (pkg && group) group.packages.push(pkg);
    pkg = null;
  };

  // A code fence is a line of only backticks (≥3). closesFence also requires the
  // closing run to be at least as long as the opening one (CommonMark).
  const fenceLen = (line: string): number => {
    const m = line.trim().match(/^(`{3,})$/);
    return m ? m[1].length : 0;
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    if (fence !== null) {
      const len = fenceLen(line);
      if (len >= fence.length) {
        if (pkg) pkg.text = fenceBuf.join("\n");
        fenceBuf.length = 0;
        fence = null;
      } else {
        fenceBuf.push(line);
      }
      continue;
    }

    if (line.startsWith("### ")) {
      flushPkg();
      pkg = { name: line.slice(4).trim(), version: "", license: "", text: "" };
      continue;
    }
    if (line.startsWith("## ")) {
      flushPkg();
      group = { title: line.slice(3).trim(), packages: [] };
      groups.push(group);
      continue;
    }
    if (line.startsWith("# ")) continue; // top-level title — shown via modal header

    if (pkg) {
      const openLen = fenceLen(line);
      if (openLen > 0) { fence = line.trim(); continue; }
      const ver = line.match(/^-\s+\*\*Version:\*\*\s*(.*)$/);
      if (ver) { pkg.version = ver[1].trim(); continue; }
      const lic = line.match(/^-\s+\*\*License:\*\*\s*(.*)$/);
      if (lic) { pkg.license = lic[1].trim(); continue; }
      // Prose fallback (e.g. the "No license file was found" note) becomes the text.
      if (line.trim() && !pkg.text) pkg.text = line.trim().replace(/_/g, "");
      continue;
    }

    if (!group) {
      // Skip the "generated file" admonition and the contents list; keep the
      // descriptive prose so the modal explains what it is showing. Blank lines
      // are kept as paragraph separators.
      const t = line.trim();
      if (!t) introRaw.push("");
      else if (!t.startsWith(">") && !t.startsWith("-") && !t.startsWith("#") && !t.startsWith("|")) {
        introRaw.push(t);
      }
    }
  }
  flushPkg();

  // Collapse the hard-wrapped intro into paragraphs: runs of non-blank lines
  // join with a space; blank lines break paragraphs.
  const intro: string[] = [];
  let para: string[] = [];
  const flushPara = () => { if (para.length) { intro.push(para.join(" ")); para = []; } };
  for (const line of introRaw) {
    if (line === "") flushPara();
    else para.push(line);
  }
  flushPara();

  return { intro, groups };
}
