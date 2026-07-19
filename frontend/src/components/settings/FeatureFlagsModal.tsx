// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useState } from "react";
import { Button, Modal, Switch, Tooltip, Typography, message } from "antd";
import { LockOutlined } from "@ant-design/icons";
import { GetFeatureFlags, SaveFeatureFlags } from "../../../wailsjs/go/app/App";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import type { config } from "../../../wailsjs/go/models";

const { Text } = Typography;

const ADMIN_TOOLTIP = "This setting is managed by your IT Administrator.";

interface Props {
  onClose: () => void;
}

// ─── Feature row ─────────────────────────────────────────────────────────────

interface FlagRowProps {
  label: string;
  description: string;
  checked: boolean;
  locked: boolean;
  onChange: (v: boolean) => void;
  preview?: boolean;
}

function FlagRow({ label, description, checked, locked, onChange, preview }: FlagRowProps) {
  const switchEl = (
    <Switch
      checked={checked}
      onChange={onChange}
      disabled={locked}
      size="small"
      style={{ flexShrink: 0, marginTop: 2 }}
    />
  );

  return (
    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 16 }}>
      <div style={{ display: "flex", alignItems: "flex-start", gap: 6 }}>
        {locked && (
          <Tooltip title={ADMIN_TOOLTIP}>
            <LockOutlined style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 3, flexShrink: 0 }} />
          </Tooltip>
        )}
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
            <Text style={{ fontSize: 13 }}>{label}</Text>
            {preview && (
              <span style={{
                fontSize: 10,
                fontWeight: 600,
                lineHeight: "16px",
                padding: "0 5px",
                borderRadius: 3,
                background: "color-mix(in srgb, var(--accent) 15%, transparent)",
                color: "var(--accent)",
                border: "1px solid color-mix(in srgb, var(--accent) 35%, transparent)",
                letterSpacing: "0.03em",
                textTransform: "uppercase",
                flexShrink: 0,
              }}>
                Preview
              </span>
            )}
          </div>
          {description && (
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>{description}</div>
          )}
        </div>
      </div>
      {locked ? (
        <Tooltip title={ADMIN_TOOLTIP}>{switchEl}</Tooltip>
      ) : (
        switchEl
      )}
    </div>
  );
}

// ─── Category section ─────────────────────────────────────────────────────────

interface CategoryProps {
  title: string;
  children: React.ReactNode;
}

function Category({ title, children }: CategoryProps) {
  return (
    <div>
      <div style={{
        fontSize: 11,
        fontWeight: 600,
        textTransform: "uppercase",
        letterSpacing: "0.05em",
        color: "var(--text-muted)",
        marginBottom: 10,
        paddingBottom: 6,
        borderBottom: "1px solid var(--border-color, rgba(0,0,0,0.08))",
      }}>
        {title}
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {children}
      </div>
    </div>
  );
}

// ─── Modal ────────────────────────────────────────────────────────────────────

export default function FeatureFlagsModal({ onClose }: Props) {
  const [flags, setFlags] = useState<config.FeatureFlags | null>(null);
  const [saving, setSaving] = useState(false);
  const locked = useFeatureFlagsStore((s) => s.locked);
  const loadStore = useFeatureFlagsStore((s) => s.load);

  useEffect(() => {
    GetFeatureFlags().then((f) => setFlags(f));
  }, []);

  function set<K extends keyof config.FeatureFlags>(key: K, value: config.FeatureFlags[K]) {
    setFlags((prev) => prev ? { ...prev, [key]: value } : prev);
  }

  async function handleSave() {
    if (!flags) return;
    setSaving(true);
    try {
      await SaveFeatureFlags(flags as any);
      await loadStore();
      message.success("Enabled features saved");
      onClose();
    } catch (err) {
      message.error(String(err));
    } finally {
      setSaving(false);
    }
  }

  if (!flags) return null;

  return (
    <Modal
      title="Enabled Features"
      open
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="save" type="primary" loading={saving} onClick={handleSave}>Save</Button>,
      ]}
      width={520}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 20, paddingTop: 8, paddingBottom: 4, maxHeight: "65vh", overflowY: "auto" }}>

        {/* ── Governance & Administration ── */}
        <Category title="Governance & Administration">
          <FlagRow
            label="User & Role Management"
            description="Create, edit, drop users, and manage key-pair authentication."
            checked={flags.userRoleManagement}
            locked={locked.userRoleManagement}
            onChange={(v) => set("userRoleManagement", v)}
          />
          <FlagRow
            label="Warehouse Management"
            description="Edit warehouse properties, suspend/resume, and abort queries."
            checked={flags.warehouseManagement}
            locked={locked.warehouseManagement}
            onChange={(v) => set("warehouseManagement", v)}
          />
          <FlagRow
            label="Warehouse Credit Usage"
            description="Visual charts and tables for account-level warehouse metering history."
            checked={flags.warehouseCreditUsage}
            locked={locked.warehouseCreditUsage}
            onChange={(v) => set("warehouseCreditUsage", v)}
          />
          <FlagRow
            label="Query Activity History"
            description="Searchable query logs scoped to session, user, or warehouse."
            checked={flags.queryActivityHistory}
            locked={locked.queryActivityHistory}
            onChange={(v) => set("queryActivityHistory", v)}
          />
          <FlagRow
            label="Integrations Management"
            description="Manage Storage, API, Security, Catalog, and other Snowflake integrations."
            checked={flags.integrationsManagement}
            locked={locked.integrationsManagement}
            onChange={(v) => set("integrationsManagement", v)}
          />
          <FlagRow
            label="Backup Policies & Sets"
            description="Manage account-level backup policies and object-scoped backup sets."
            checked={flags.backupPoliciesAndSets}
            locked={locked.backupPoliciesAndSets}
            onChange={(v) => set("backupPoliciesAndSets", v)}
          />
        </Category>

        {/* ── AI & Assistance ── */}
        <Category title="AI & Assistance">
          <FlagRow
            label="AI Inline Completions"
            description="Ghost-text SQL suggestions as you type in the editor."
            checked={flags.aiInlineCompletions}
            locked={locked.aiInlineCompletions}
            onChange={(v) => set("aiInlineCompletions", v)}
          />
        </Category>

        {/* ── Advanced Tools & Data Engineering ── */}
        <Category title="Advanced Tools & Data Engineering">
          <FlagRow
            label="Schema Migration"
            description="DDL diffing and multi-strategy deployment wizard."
            checked={flags.schemaMigration}
            locked={locked.schemaMigration}
            onChange={(v) => set("schemaMigration", v)}
          />
          <FlagRow
            label="dbt Project Scaffolding"
            description="Automated dbt project generation wired to the active Snowflake connection."
            checked={flags.dbtScaffolding}
            locked={locked.dbtScaffolding}
            onChange={(v) => set("dbtScaffolding", v)}
          />
          <FlagRow
            label="DBT Project Browser"
            description="Browse and manage Snowflake-native DBT PROJECT objects in the sidebar."
            checked={flags.dbtProjectBrowser}
            locked={locked.dbtProjectBrowser}
            onChange={(v) => set("dbtProjectBrowser", v)}
          />
          <FlagRow
            label="ER Diagram & Designer"
            description="Visual database modeling and interactive ALTER TABLE generation."
            checked={flags.erDiagramDesigner}
            locked={locked.erDiagramDesigner}
            onChange={(v) => set("erDiagramDesigner", v)}
          />
          <FlagRow
            label="Task Graph Visualizer"
            description="Interactive DAG viewer and manager for Snowflake task graphs."
            checked={flags.taskGraphVisualizer}
            locked={locked.taskGraphVisualizer}
            onChange={(v) => set("taskGraphVisualizer", v)}
          />
          <FlagRow
            label="Code Snippets"
            description="Library of curated CREATE OR REPLACE templates for common Snowflake objects."
            checked={flags.codeSnippets}
            locked={locked.codeSnippets}
            onChange={(v) => set("codeSnippets", v)}
          />
        </Category>

        {/* ── Developer Environments ── */}
        <Category title="Developer Environments">
          <FlagRow
            label="Snowpark & Notebooks"
            description="Embedded Python kernel and Jupyter-style notebook environment."
            checked={flags.snowparkNotebooks}
            locked={locked.snowparkNotebooks}
            onChange={(v) => set("snowparkNotebooks", v)}
          />
          <FlagRow
            label="Embedded Terminal"
            description="xterm.js OS shell panel in the results area."
            checked={flags.embeddedTerminal}
            locked={locked.embeddedTerminal}
            onChange={(v) => set("embeddedTerminal", v)}
          />
          <FlagRow
            label="Git Integration"
            description="Git status, commit, and push/pull UI for the working directory."
            checked={flags.gitIntegration}
            locked={locked.gitIntegration}
            onChange={(v) => set("gitIntegration", v)}
            preview
          />
        </Category>

        {/* ── Performance & Diagnostics ── */}
        <Category title="Performance & Diagnostics">
          <FlagRow
            label="Query Log"
            description="Session-scoped log of all SQL queries Thaw sends to Snowflake, for debugging and issue reporting."
            checked={flags.queryLog}
            locked={locked.queryLog}
            onChange={(v) => set("queryLog", v)}
          />
        </Category>

        {/* ── Connection ── */}
        <Category title="Connection">
          <FlagRow
            label="Snowflake CLI Profile Manager"
            description="Manage Snowflake CLI profiles (~/.snowflake/config.toml) from the connection dialog."
            checked={flags.snowflakeCLIProfileManager}
            locked={locked.snowflakeCLIProfileManager}
            onChange={(v) => set("snowflakeCLIProfileManager", v)}
          />
        </Category>

        {/* ── SQL Editor ── */}
        <Category title="SQL Editor">
          <FlagRow
            label="SQL Diagnostics"
            description="Real-time linting for syntax errors, anti-patterns, bad type casts, and missing tables."
            checked={flags.sqlDiagnostics}
            locked={locked.sqlDiagnostics}
            onChange={(v) => set("sqlDiagnostics", v)}
          />
          <FlagRow
            label="Schema Autocomplete"
            description="Schema-aware completions for databases, schemas, tables, columns, and JOIN conditions (requires Snowflake connection)."
            checked={flags.schemaAutocomplete}
            locked={locked.schemaAutocomplete}
            onChange={(v) => set("schemaAutocomplete", v)}
          />
          <FlagRow
            label="DDL Hover Tooltips"
            description="Hover over a table, view, or function name to see its DDL definition inline."
            checked={flags.ddlHoverTooltips}
            locked={locked.ddlHoverTooltips}
            onChange={(v) => set("ddlHoverTooltips", v)}
          />
        </Category>

        {/* ── Integrations ── */}
        <Category title="Integrations">
          <FlagRow
            label="MCP Server"
            description="Expose the active Snowflake connection to external AI clients over a local Model Context Protocol server (Tools → MCP Sessions)."
            checked={flags.mcpServer}
            locked={locked.mcpServer}
            onChange={(v) => set("mcpServer", v)}
            preview
          />
        </Category>

      </div>
    </Modal>
  );
}
