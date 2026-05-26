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
// @thaw-domain: Core IPC & App Lifecycle

import type { ReactNode } from "react";
import { Button, Dropdown, Space, Typography, Select, Tag, Tooltip } from "antd";
import {
  PlayCircleOutlined,
  StopOutlined,
  DisconnectOutlined,
  LinkOutlined,
  FileAddOutlined,
  BookOutlined,
  SaveOutlined,
} from "@ant-design/icons";
import { useConnectionStore } from "../../store/connectionStore";
import { useSessionStore } from "../../store/sessionStore";

const { Text } = Typography;

const selectStyle = { fontSize: 12, width: 130 };

export interface ToolbarProps {
  /** Whether a query is currently running in the active tab. */
  isRunning: boolean;
  /** Whether cancellation has been requested. */
  isCancelling: boolean;
  /** The currently selected SQL text (if any). */
  selectedSql: string;
  /** Current username display. */
  currentUser: string | null;
  /** Current region display. */
  currentRegion: string | null;
  /** Handler: run the current query. */
  onRun: () => void;
  /** Handler: cancel the running query. */
  onCancel: () => void;
  /** Handler: disconnect from Snowflake. */
  onDisconnect: () => void;
  /** Handler: open session properties modal. */
  onOpenSessionProperties: () => void;
  /** Handler: open Snowsight. */
  onOpenSnowsight: () => void;
  /** Handler: create a new SQL tab. */
  onNewSql: () => void;
  /** Handler: create a new notebook tab. */
  onNewNotebook: () => void;
  /** Handler: save the current tab. */
  onSave: () => void;
  /**
   * Tab-specific toolbar section (e.g. notebook controls). Rendered after
   * a vertical separator if present; pass the entire cluster as a fragment.
   */
  contextSlot?: ReactNode;
}

export default function Toolbar({
  isRunning,
  isCancelling,
  selectedSql,
  currentUser,
  currentRegion,
  onRun,
  onCancel,
  onDisconnect,
  onOpenSessionProperties,
  onOpenSnowsight,
  onNewSql,
  onNewNotebook,
  onSave,
  contextSlot,
}: ToolbarProps) {
  const params = useConnectionStore((s) => s.params);
  const isConnected = useConnectionStore((s) => s.isConnected);
  const role = useSessionStore((s) => s.role);
  const warehouse = useSessionStore((s) => s.warehouse);
  const database = useSessionStore((s) => s.database);
  const schema = useSessionStore((s) => s.schema);
  const roles = useSessionStore((s) => s.roles);
  const warehouses = useSessionStore((s) => s.warehouses);
  const databases = useSessionStore((s) => s.databases);
  const schemas = useSessionStore((s) => s.schemas);
  const loadingContext = useSessionStore((s) => s.loadingContext);
  const loadingRoles = useSessionStore((s) => s.loadingRoles);
  const loadingWarehouses = useSessionStore((s) => s.loadingWarehouses);
  const loadingDatabases = useSessionStore((s) => s.loadingDatabases);
  const loadingSchemas = useSessionStore((s) => s.loadingSchemas);
  const switchingRole = useSessionStore((s) => s.switchingRole);
  const switchingWarehouse = useSessionStore((s) => s.switchingWarehouse);
  const switchingDatabase = useSessionStore((s) => s.switchingDatabase);
  const switchingSchema = useSessionStore((s) => s.switchingSchema);
  const loadRoles = useSessionStore((s) => s.loadRoles);
  const loadWarehouses = useSessionStore((s) => s.loadWarehouses);
  const loadDatabases = useSessionStore((s) => s.loadDatabases);
  const loadSchemas = useSessionStore((s) => s.loadSchemas);
  const switchRole = useSessionStore((s) => s.switchRole);
  const switchWarehouse = useSessionStore((s) => s.switchWarehouse);
  const switchDatabase = useSessionStore((s) => s.switchDatabase);
  const switchSchema = useSessionStore((s) => s.switchSchema);

  return (
    <div
      style={{
        padding: "6px 12px",
        borderBottom: "1px solid var(--border)",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        background: "var(--bg-raised)",
      }}
    >
      {/* ── Left: execution controls + button grid ── */}
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        {/* Run/Cancel + hint */}
        <Space size={4}>
          {isRunning ? (
            <Button
              danger
              icon={<StopOutlined />}
              loading={isCancelling}
              onClick={onCancel}
              size="small"
            >
              {isCancelling ? "Cancelling\u2026" : "Cancel"}
            </Button>
          ) : (
            <Tooltip title={!isConnected ? "Connect to Snowflake to run queries" : undefined}>
              <Button
                type="primary"
                icon={<PlayCircleOutlined />}
                onClick={onRun}
                size="small"
                disabled={!isConnected}
              >
                Run
              </Button>
            </Tooltip>
          )}
          <Text type="secondary" style={{ fontSize: 11, whiteSpace: "nowrap" }}>
            {isRunning
              ? "Esc to cancel"
              : selectedSql.trim()
              ? "\u2318\u21B5 \u00B7 running selection"
              : "\u2318\u21B5 to run"}
          </Text>
        </Space>

        {/* Separator */}
        <div style={{ width: 1, alignSelf: "stretch", background: "var(--border)" }} />

        {/* Action button grid: 3 columns, 1 or 2 rows */}
        <div className="thaw-tb-group">
          <Tooltip title="New SQL query">
            <Button className="thaw-tb-icon-btn" aria-label="New SQL query"
              icon={<FileAddOutlined />} onClick={onNewSql} />
          </Tooltip>
          <Tooltip title="New notebook">
            <Button className="thaw-tb-icon-btn" aria-label="New notebook"
              icon={<BookOutlined />} onClick={onNewNotebook} />
          </Tooltip>
          <Tooltip title="Save (\u2318S)">
            <Button className="thaw-tb-icon-btn" aria-label="Save"
              icon={<SaveOutlined />} onClick={onSave} />
          </Tooltip>
        </div>

        {/* Tab-specific section (notebook kernel pill + actions) */}
        {contextSlot && (
          <>
            <div className="thaw-tb-sep" />
            {contextSlot}
          </>
        )}
      </div>

      {/* ── Right: connect button or session context ── */}
      {!isConnected ? (
        <Button
          icon={<LinkOutlined />}
          type="primary"
          size="small"
          onClick={() => window.dispatchEvent(new Event("thaw:connect"))}
        >
          Connect to Snowflake
        </Button>
      ) : null}
      <Space size={6} style={{ display: isConnected ? undefined : "none" }}>
        {/* Session selectors: two rows (role+wh / db+schema) */}
        <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <Space size={6}>
            <Tooltip title={role ? `Role: ${role}` : "Active role"}>
              <Select
                size="small"
                style={selectStyle}
                value={role || undefined}
                placeholder={loadingContext ? "\u2026" : "Role"}
                loading={loadingRoles || switchingRole}
                showSearch
                optionFilterProp="label"
                onChange={switchRole}
                onDropdownVisibleChange={(open) => { if (open) loadRoles(); }}
                options={roles.map((r) => ({ value: r, label: r }))}
                dropdownStyle={{ minWidth: 200 }}
              />
            </Tooltip>
            <Tooltip title={warehouse ? `Warehouse: ${warehouse}` : "Active warehouse"}>
              <Select
                size="small"
                style={selectStyle}
                value={warehouse || undefined}
                placeholder={loadingContext ? "\u2026" : "Warehouse"}
                loading={loadingWarehouses || switchingWarehouse}
                showSearch
                optionFilterProp="label"
                onChange={switchWarehouse}
                onDropdownVisibleChange={(open) => { if (open) loadWarehouses(); }}
                options={warehouses.map((w) => ({ value: w, label: w }))}
                dropdownStyle={{ minWidth: 200 }}
              />
            </Tooltip>
          </Space>
          <Space size={6}>
            <Tooltip title={database ? `Database: ${database}` : "Active database"}>
              <Select
                size="small"
                style={selectStyle}
                value={database || undefined}
                placeholder={loadingContext ? "\u2026" : "Database"}
                loading={loadingDatabases || switchingDatabase}
                showSearch
                optionFilterProp="label"
                onChange={switchDatabase}
                onDropdownVisibleChange={(open) => { if (open) loadDatabases(); }}
                options={databases.map((d) => ({ value: d, label: d }))}
                dropdownStyle={{ minWidth: 200 }}
              />
            </Tooltip>
            <Tooltip title={schema ? `Schema: ${schema}` : "Active schema"}>
              <Select
                size="small"
                style={selectStyle}
                value={schema || undefined}
                placeholder={loadingContext ? "\u2026" : "Schema"}
                loading={loadingSchemas || switchingSchema}
                showSearch
                optionFilterProp="label"
                onChange={switchSchema}
                onDropdownVisibleChange={(open) => { if (open) loadSchemas(); }}
                options={schemas.map((s) => ({ value: s, label: s }))}
                dropdownStyle={{ minWidth: 200 }}
              />
            </Tooltip>
          </Space>
        </div>

        {params && (
          <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 2 }}>
            {(currentUser || currentRegion) && (
              <div style={{ fontSize: 10, color: "var(--text-muted)", fontFamily: "monospace", lineHeight: 1 }}>
                {[currentUser, currentRegion].filter(Boolean).join(" \u00B7 ")}
              </div>
            )}
            <Dropdown
              trigger={["contextMenu"]}
              menu={{
                items: [
                  { key: "session-props", label: "Session Properties", onClick: onOpenSessionProperties },
                  { key: "snowsight", label: "Open Snowsight\u2026", onClick: onOpenSnowsight },
                ],
              }}
            >
              <Tag color="blue" style={{ fontSize: 11, margin: 0, cursor: "context-menu" }}>
                {params.account} \u00B7 {params.user}
              </Tag>
            </Dropdown>
          </div>
        )}
        <Button
          icon={<DisconnectOutlined />}
          size="small"
          danger
          onClick={onDisconnect}
        >
          Disconnect
        </Button>
      </Space>
    </div>
  );
}
