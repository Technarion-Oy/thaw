// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration
//
// Distinct, themeable icons for the object browser tree.
//
// Each Snowflake object kind gets:
//   1. A unique Ant Design Outlined icon (e.g. Functions are an italic ƒ, not
//      a generic file).
//   2. A unique colour drawn from a CSS variable in global.css (--icon-*), so
//      the palette adapts to dark vs. light theme without recompiling TS.
//
// Why custom inline-style colour instead of @ant-design/icons TwoTone?
//   TwoTone icons only expose two control points and don't pick up CSS vars —
//   they hardcode hex via the `twoToneColor` prop, which means the palette
//   has to be split across two files and re-evaluated on theme change.
//   Outlined + `style={{ color: 'var(--icon-x)' }}` lets CSS do the work.

import type { ReactNode } from "react";
import {
  // Containers
  DatabaseOutlined,
  FolderOutlined,
  // Snowflake object kinds
  TableOutlined,
  EyeOutlined,
  FunctionOutlined,
  CodeOutlined,
  NumberOutlined,
  InboxOutlined,
  ThunderboltOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  ShareAltOutlined,
  ExperimentOutlined,
  LockOutlined,
  BranchesOutlined,
  FileOutlined,
} from "@ant-design/icons";

// ── CSS variable name per kind ─────────────────────────────────────────────
// Keys are the UPPERCASE Snowflake kind strings emitted by ListObjects.
const KIND_VAR: Record<string, string> = {
  TABLE:             "--icon-table",
  VIEW:              "--icon-view",
  FUNCTION:          "--icon-function",
  PROCEDURE:         "--icon-procedure",
  SEQUENCE:          "--icon-sequence",
  STAGE:             "--icon-stage",
  STREAM:            "--icon-stream",
  TASK:              "--icon-task",
  "FILE FORMAT":     "--icon-fileformat",
  PIPE:              "--icon-pipe",
  NOTEBOOK:          "--icon-notebook",
  SECRET:            "--icon-secret",
  "GIT REPOSITORY":  "--icon-gitrepo",
};

// ── Icon component per kind ────────────────────────────────────────────────
const KIND_ICON: Record<string, React.ComponentType<{ style?: React.CSSProperties }>> = {
  TABLE:            TableOutlined,
  VIEW:             EyeOutlined,
  FUNCTION:         FunctionOutlined,
  PROCEDURE:        CodeOutlined,
  SEQUENCE:         NumberOutlined,
  STAGE:            InboxOutlined,
  STREAM:           ThunderboltOutlined,    // signals real-time data flow
  TASK:             ClockCircleOutlined,
  "FILE FORMAT":    FileTextOutlined,
  PIPE:             ShareAltOutlined,       // distinct from STREAM (was clashing on ApiOutlined)
  NOTEBOOK:         ExperimentOutlined,
  SECRET:           LockOutlined,           // distinct from PK column icon (KeyOutlined)
  "GIT REPOSITORY": BranchesOutlined,
};

// ── Public API ────────────────────────────────────────────────────────────

/** Coloured icon for a Snowflake object kind (TABLE / VIEW / FUNCTION / …). */
export function objectIcon(kind: string): ReactNode {
  const Icon  = KIND_ICON[kind]  ?? FileOutlined;
  const cssVar = KIND_VAR[kind]  ?? "--text-muted";
  return <Icon style={{ color: `var(${cssVar})` }} />;
}

/** Icon for a database (top-level node). */
export function databaseIcon(): ReactNode {
  return <DatabaseOutlined style={{ color: "var(--icon-database)" }} />;
}

/** Icon for a schema. */
export function schemaIcon(): ReactNode {
  return <FolderOutlined style={{ color: "var(--icon-schema)" }} />;
}

/** Icon for a “kind group” folder (Tables / Views / …). Stays decorative
 *  so the coloured object icons inside it pop without competition. */
export function typeGroupIcon(): ReactNode {
  return <FolderOutlined style={{ color: "var(--text-faint)" }} />;
}
