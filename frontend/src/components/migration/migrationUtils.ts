// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Schema Migration

import type { CSSProperties } from "react";

// ─── backend types (mirrors migration.go structs) ─────────────────────────────

export interface MigrationObject {
  filePath: string;
  database: string;
  schema: string;
  objectKind: string;
  objectName: string;
  argSig: string;
  ddl: string;
  isReplace: boolean;
}

export interface MigrationDiffItem {
  object: MigrationObject;
  status: "new" | "changed" | "unchanged" | "removed";
  localDDL: string;
  remoteDDL: string;
}

export interface MigrationExecEvent {
  done: number;
  total: number;
  object: string;
  status: "running" | "success" | "failed" | "skipped";
  error: string;
  pass: number;
}

// ─── helpers ──────────────────────────────────────────────────────────────────

export function statusColor(status: string): string {
  switch (status) {
    case "new":
      return "green";
    case "changed":
      return "orange";
    case "unchanged":
      return "default";
    case "removed":
      return "red";
    case "success":
      return "success";
    case "failed":
      return "error";
    case "skipped":
      return "warning";
    case "running":
      return "processing";
    default:
      return "default";
  }
}

export function objectLabel(mo: MigrationObject): string {
  return `${mo.database}.${mo.schema}.${mo.objectKind}.${mo.objectName}`;
}

// ─── Shared grid table styles ─────────────────────────────────────────────────

export const gridTableStyle: CSSProperties = {
  width: "100%",
  borderCollapse: "collapse",
  tableLayout: "fixed",
  fontSize: 12,
  fontFamily: "var(--ui-font, 'Inter', 'SF Pro Text', system-ui, sans-serif)",
};

export const gridHeaderStyle: CSSProperties = {
  position: "sticky",
  top: 0,
  zIndex: 2,
  background: "var(--bg-raised)",
};
