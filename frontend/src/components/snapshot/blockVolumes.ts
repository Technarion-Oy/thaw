// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import type { ResultRow } from "../containerservices/computePools";

/**
 * Snapshot-able volumes from SHOW SERVICE VOLUMES rows: only `block` volumes
 * (never stage / local / memory), each with the instance ids it is mounted on.
 * The type column is read under its known aliases; a row without one is skipped.
 */
export function blockVolumes(rows: ResultRow[]): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const r of rows) {
    if ((r.volume_type ?? r.source ?? r.type ?? "").toLowerCase() !== "block") continue;
    const name = r.volume_name ?? r.name;
    if (!name) continue;
    const ids = out.get(name) ?? [];
    const id = r.instance_id ?? r.instance;
    if (id && !ids.includes(id)) ids.push(id);
    out.set(name, ids);
  }
  return out;
}
