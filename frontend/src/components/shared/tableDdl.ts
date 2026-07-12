// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

/**
 * Table-level CREATE TABLE options shared by the Create Table modal and the ER
 * Designer's diff SQL generator (issue #615). Kept here so the keyword prefix
 * (OR REPLACE / TRANSIENT / IF NOT EXISTS) and the trailing option clauses
 * (CLUSTER BY, retention, change-tracking, comment…) are emitted identically by
 * both consumers.
 */
export interface TableOptions {
  tableType?: "PERMANENT" | "TRANSIENT" | "TEMPORARY" | "VOLATILE";
  orReplace?: boolean;
  ifNotExists?: boolean;
  clusterBy?: string;
  dataRetentionTimeInDays?: number | "";
  maxDataExtensionTimeInDays?: number | "";
  changeTracking?: boolean;
  enableSchemaEvolution?: boolean;
  comment?: string;
}

/**
 * The `CREATE [OR REPLACE] [TRANSIENT|TEMPORARY|VOLATILE] TABLE [IF NOT EXISTS]`
 * keyword prefix (without the table reference).
 *
 * When `o` is undefined (a table not defined via the modal — e.g. inline-added
 * in the ER Designer) it falls back to the historical `CREATE TABLE IF NOT
 * EXISTS` so existing behaviour is unchanged.
 */
export function createTableClause(o?: TableOptions): string {
  let c = "CREATE";
  if (o?.orReplace) c += " OR REPLACE";
  const tt = o?.tableType ?? "PERMANENT";
  if (tt !== "PERMANENT") c += ` ${tt}`;
  c += " TABLE";
  const ifNotExists = o ? (o.ifNotExists && !o.orReplace) : true;
  if (ifNotExists) c += " IF NOT EXISTS";
  return c;
}

/** Trailing table-option clauses, one per line (empty when `o` is undefined). */
export function tableOptionsClauses(o?: TableOptions): string[] {
  if (!o) return [];
  const sq = (s: string) => "'" + s.replace(/'/g, "''") + "'";
  const lines: string[] = [];
  if (o.clusterBy?.trim()) lines.push(`CLUSTER BY (${o.clusterBy.trim()})`);
  if (o.enableSchemaEvolution) lines.push("ENABLE_SCHEMA_EVOLUTION = TRUE");
  if (o.dataRetentionTimeInDays !== undefined && o.dataRetentionTimeInDays !== "")
    lines.push(`DATA_RETENTION_TIME_IN_DAYS = ${o.dataRetentionTimeInDays}`);
  if (o.maxDataExtensionTimeInDays !== undefined && o.maxDataExtensionTimeInDays !== "")
    lines.push(`MAX_DATA_EXTENSION_TIME_IN_DAYS = ${o.maxDataExtensionTimeInDays}`);
  if (o.changeTracking) lines.push("CHANGE_TRACKING = TRUE");
  if (o.comment?.trim()) lines.push(`COMMENT = ${sq(o.comment.trim())}`);
  return lines;
}
