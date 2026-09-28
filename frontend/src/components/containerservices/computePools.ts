// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import type { snowflake } from "../../../wailsjs/go/models";

/** One SHOW/DESCRIBE row keyed by lower-cased column name, every cell stringified. */
export type ResultRow = Record<string, string>;

export function rowsOf(res: snowflake.QueryResult | null | undefined): ResultRow[] {
  const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
  return (res?.rows ?? []).map((r) =>
    Object.fromEntries(cols.map((c, i) => [c, r[i] == null ? "" : String(r[i])])));
}

/** One-line summary of a SHOW COMPUTE POOL INSTANCE FAMILIES row for pickers. */
export function familyLabel(f: ResultRow): string {
  const parts = [f.name];
  if (f.vcpu) parts.push(`${f.vcpu} vCPU`);
  if (f.memory_gib) parts.push(`${f.memory_gib} GiB`);
  const gpu = f.gpu && f.gpu.toUpperCase() !== "NONE" ? f.gpu : "";
  if (gpu || (f.gpu_count && f.gpu_count !== "0")) {
    parts.push(`${f.gpu_count || "?"}× ${gpu || "GPU"}${f.gpu_memory_gib ? ` (${f.gpu_memory_gib} GiB)` : ""}`);
  }
  if (f.current_node_usage) parts.push(`${f.current_node_usage} in use`);
  return parts.join(" · ");
}

export const familyOptions = (fams: ResultRow[]) =>
  fams.map((f) => ({ value: f.name, label: familyLabel(f) }));

/** SHOW COMPUTE POOLS shows backup families as `["A","B"]` or `A,B`; normalize to `A, B`. */
export const familyListText = (v: string | undefined) =>
  (v ?? "").replace(/[[\]"']/g, "").split(",").map((s) => s.trim()).filter(Boolean).join(", ");

export function stateColor(state: string | undefined): string {
  switch ((state ?? "").toUpperCase()) {
    case "ACTIVE": case "IDLE": return "var(--success)";
    case "SUSPENDED": return "var(--text-faint)";
    case "STARTING": case "RESIZING": return "var(--link)";
    case "STOPPING": return "var(--warning)";
    default: return "var(--danger)";
  }
}
