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
  RetweetOutlined,
  CloudServerOutlined,
  BlockOutlined,
  AlertOutlined,
  TagsOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
  SafetyOutlined,
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
  KeyOutlined,
  FontSizeOutlined,
  CalendarOutlined,
  CheckSquareOutlined,
  CodeSandboxOutlined,
  BuildOutlined,
  GlobalOutlined,
  BarChartOutlined,
  LinkOutlined,
  ContainerOutlined,
  DeploymentUnitOutlined,
  AppstoreOutlined,
  GoldOutlined,
  MergeCellsOutlined,
  AuditOutlined,
  ApiOutlined,
  FundOutlined,
} from "@ant-design/icons";

// ── CSS variable name per kind ─────────────────────────────────────────────
const KIND_VAR: Record<string, string> = {
  TABLE:             "--icon-table",
  VIEW:              "--icon-view",
  "DYNAMIC TABLE":   "--icon-dynamictable",
  "EXTERNAL TABLE":  "--icon-externaltable",
  "ICEBERG TABLE":   "--icon-icebergtable",
  "HYBRID TABLE":    "--icon-hybridtable",
  "EVENT TABLE":     "--icon-eventtable",
  "MATERIALIZED VIEW": "--icon-materializedview",
  ALERT:             "--icon-alert",
  TAG:               "--icon-tag",
  "MASKING POLICY":  "--icon-maskingpolicy",
  "ROW ACCESS POLICY": "--icon-rowaccesspolicy",
  "NETWORK RULE":    "--icon-networkrule",
  "IMAGE REPOSITORY": "--icon-imagerepository",
  SERVICE:           "--icon-service",
  STREAMLIT:         "--icon-streamlit",
  FUNCTION:          "--icon-function",
  "EXTERNAL FUNCTION": "--icon-externalfunction",
  "DATA METRIC FUNCTION": "--icon-datametricfunction",
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
  "DBT PROJECT":     "--icon-dbtproject",
};

// ── Icon component per kind ────────────────────────────────────────────────
const KIND_ICON: Record<string, React.ComponentType<{ style?: React.CSSProperties }>> = {
  TABLE:            TableOutlined,
  VIEW:             EyeOutlined,
  "DYNAMIC TABLE":  RetweetOutlined,
  "EXTERNAL TABLE": CloudServerOutlined,
  "ICEBERG TABLE":  GoldOutlined,
  "HYBRID TABLE":   MergeCellsOutlined,
  "EVENT TABLE":    AuditOutlined,
  "MATERIALIZED VIEW": BlockOutlined,
  ALERT:            AlertOutlined,
  TAG:              TagsOutlined,
  "MASKING POLICY": EyeInvisibleOutlined,
  "ROW ACCESS POLICY": SafetyOutlined,
  "NETWORK RULE":   GlobalOutlined,
  "IMAGE REPOSITORY": ContainerOutlined,
  SERVICE:          DeploymentUnitOutlined,
  STREAMLIT:        AppstoreOutlined,
  FUNCTION:         FunctionOutlined,
  "EXTERNAL FUNCTION": ApiOutlined,
  "DATA METRIC FUNCTION": FundOutlined,
  PROCEDURE:        CodeOutlined,
  SEQUENCE:         NumberOutlined,
  STAGE:            InboxOutlined,
  STREAM:           ThunderboltOutlined,
  TASK:             ClockCircleOutlined,
  "FILE FORMAT":    FileTextOutlined,
  PIPE:             ShareAltOutlined,
  NOTEBOOK:         ExperimentOutlined,
  SECRET:           LockOutlined,
  "GIT REPOSITORY": BranchesOutlined,
  "DBT PROJECT":    BuildOutlined,
};

// ── Public API ────────────────────────────────────────────────────────────

export function columnFamily(rawType: string): "text" | "number" | "datetime" | "boolean" | "variant" | "array" | "binary" | "geo" | "vector" {
  const t = rawType.toUpperCase().trim();
  if (t.startsWith("NUMBER") || t.startsWith("DECIMAL") || t.startsWith("NUMERIC") || t.startsWith("INT") || t.startsWith("FLOAT") || t.startsWith("DOUBLE") || t.startsWith("REAL") || t.startsWith("BYTEINT")) return "number";
  if (t.startsWith("DATE") || t.startsWith("TIME") || t.startsWith("TIMESTAMP")) return "datetime";
  if (t === "BOOLEAN") return "boolean";
  if (t === "VARIANT" || t === "OBJECT" || t === "MAP") return "variant";
  if (t === "ARRAY") return "array";
  if (t.startsWith("BINARY") || t.startsWith("VARBINARY")) return "binary";
  if (t === "GEOGRAPHY" || t === "GEOMETRY") return "geo";
  if (t.startsWith("VECTOR")) return "vector";
  return "text";
}

export function columnIcon(rawType: string, opts?: { primaryKey?: boolean; foreignKey?: boolean }): ReactNode {
  if (opts?.primaryKey) return <span className="thaw-col-icon" data-family="pk"><KeyOutlined /></span>;
  if (opts?.foreignKey) return <span className="thaw-col-icon" data-family="fk"><LinkOutlined /></span>;

  const fam = columnFamily(rawType);
  let Icon = FontSizeOutlined;
  switch (fam) {
    case "text":     Icon = FontSizeOutlined; break;
    case "number":   Icon = NumberOutlined; break;
    case "datetime": Icon = CalendarOutlined; break;
    case "boolean":  Icon = CheckSquareOutlined; break;
    case "variant":  Icon = CodeSandboxOutlined; break;
    case "array":    Icon = BuildOutlined; break;
    case "binary":   Icon = FileOutlined; break;
    case "geo":      Icon = GlobalOutlined; break;
    case "vector":   Icon = BarChartOutlined; break;
  }
  return <span className="thaw-col-icon" data-family={fam}><Icon /></span>;
}

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
