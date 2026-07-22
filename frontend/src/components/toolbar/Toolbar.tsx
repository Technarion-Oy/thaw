// SPDX-License-Identifier: GPL-3.0-or-later
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
import MCPIndicator from "./MCPIndicator";

const { Text } = Typography;

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
  /** Handler: open account parameters modal. */
  onOpenAccountParameters: () => void;
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
  /**
   * Tab-specific primary action shown above the three global icon
   * buttons (New SQL / New notebook / Save). When present, the globals
   * shrink to a compact two-row stack. Pass nothing on tabs without a
   * primary action (e.g. SQL tabs) and the row stays single-row.
   */
  primaryAction?: ReactNode;
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
  onOpenAccountParameters,
  onOpenSnowsight,
  onNewSql,
  onNewNotebook,
  onSave,
  contextSlot,
  primaryAction,
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
    <div className="thaw-tb">
      {/* ── Left: execution controls + button grid ── */}
      <div className="thaw-tb-left">
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
          <Text type="secondary" className="thaw-tb-hint">
            {isRunning
              ? "Esc to cancel"
              : selectedSql.trim()
              ? "\u2318\u21B5 \u00B7 running selection"
              : "\u2318\u21B5 to run"}
          </Text>
        </Space>

        {/* Separator */}
        <div className="thaw-tb-sep" />

        {/* Action button grid: 3 columns, 1 or 2 rows */}
        {primaryAction ? (
          <div className="thaw-tb-vstack">
            {primaryAction}
            <div className="thaw-tb-vstack-row">
              <Tooltip title="New SQL query">
                <Button className="thaw-tb-vstack-icon" aria-label="New SQL query"
                  icon={<FileAddOutlined />} onClick={onNewSql} />
              </Tooltip>
              <Tooltip title="New notebook">
                <Button className="thaw-tb-vstack-icon" aria-label="New notebook"
                  icon={<BookOutlined />} onClick={onNewNotebook} />
              </Tooltip>
              <Tooltip title="Save (⌘S)">
                <Button className="thaw-tb-vstack-icon" aria-label="Save"
                  icon={<SaveOutlined />} onClick={onSave} />
              </Tooltip>
            </div>
          </div>
        ) : (
          <div className="thaw-tb-group">
            <Tooltip title="New SQL query">
              <Button className="thaw-tb-icon-btn" aria-label="New SQL query"
                icon={<FileAddOutlined />} onClick={onNewSql} />
            </Tooltip>
            <Tooltip title="New notebook">
              <Button className="thaw-tb-icon-btn" aria-label="New notebook"
                icon={<BookOutlined />} onClick={onNewNotebook} />
            </Tooltip>
            <Tooltip title="Save (⌘S)">
              <Button className="thaw-tb-icon-btn" aria-label="Save"
                icon={<SaveOutlined />} onClick={onSave} />
            </Tooltip>
          </div>
        )}

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
          className="thaw-tb-right"
          icon={<LinkOutlined />}
          type="primary"
          size="small"
          onClick={() => window.dispatchEvent(new Event("thaw:connect"))}
        >
          Connect to Snowflake
        </Button>
      ) : null}
      <Space size={6} wrap className="thaw-tb-right" style={{ display: isConnected ? undefined : "none" }}>
        {/* Session selectors: two rows (role+wh / db+schema) */}
        <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <Space size={6}>
            <Tooltip title={role ? `Role: ${role}` : "Active role"}>
              <Select
                size="small"
                className="thaw-tb-select"
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
                className="thaw-tb-select"
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
                className="thaw-tb-select"
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
                className="thaw-tb-select"
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

        <MCPIndicator>
          <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 2 }}>
            {params && (
              <>
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
                      { key: "account-params", label: "Account Parameters", onClick: onOpenAccountParameters },
                      { key: "snowsight", label: "Open Snowsight\u2026", onClick: onOpenSnowsight },
                    ],
                  }}
                >
                  <Tag color="blue" style={{ fontSize: 11, margin: 0, cursor: "context-menu" }}>
                    {params.account} · {params.user}
                  </Tag>
                </Dropdown>
              </>
            )}
            <Button
              icon={<DisconnectOutlined />}
              size="small"
              danger
              onClick={onDisconnect}
              style={{ width: "100%", marginTop: 2 }}
            >
              Disconnect
            </Button>
          </div>
        </MCPIndicator>
      </Space>
    </div>
  );
}
