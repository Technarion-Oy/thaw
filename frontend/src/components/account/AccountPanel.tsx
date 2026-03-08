// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useLayoutEffect, useRef } from "react";
import { Collapse, Space, Button, Typography, Tree, Spin, Modal, message } from "antd";
import {
  TeamOutlined,
  ThunderboltOutlined,
  ReloadOutlined,
  ExportOutlined,
  CopyOutlined,
  FileOutlined,
  DiffOutlined,
  HistoryOutlined,
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import {
  ListRoles,
  ListWarehouses,
  GetRoleDDL,
  GetWarehouseDDL,
  ExportAccountObjectsDDL,
  GetObjectProperties,
} from "../../../wailsjs/go/main/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { useGitStore } from "../../store/gitStore";
import { useDiffStore } from "../../store/diffStore";
import UserManagementPanel from "./UserManagementPanel";
import PropertiesModal from "../common/PropertiesModal";
import QueryHistoryModal from "./QueryHistoryModal";
import BackupPoliciesPanel from "../backup/BackupPoliciesPanel";
import type { main } from "../../../wailsjs/go/models";

const { Text } = Typography;
const CLR_BORDER    = "var(--border)";
const CLR_SECONDARY = "var(--text-muted)";

interface DdlModal {
  title: string;
  src: string;
  loading: boolean;
  error: string | null;
}

interface AccountCtxMenu {
  x: number;
  y: number;
  kind: "role" | "warehouse";
  name: string;
}

function buildTree(roles: string[], warehouses: string[]): DataNode[] {
  return [
    {
      key:      "group:roles",
      title:    `Roles (${roles.length})`,
      icon:     <TeamOutlined style={{ color: CLR_SECONDARY }} />,
      isLeaf:   false,
      children: roles.map((name) => ({
        key:    `role:${name}`,
        title:  name,
        icon:   <TeamOutlined style={{ color: CLR_SECONDARY, fontSize: 11 }} />,
        isLeaf: true,
      })),
    },
    {
      key:      "group:warehouses",
      title:    `Warehouses (${warehouses.length})`,
      icon:     <ThunderboltOutlined style={{ color: CLR_SECONDARY }} />,
      isLeaf:   false,
      children: warehouses.map((name) => ({
        key:    `warehouse:${name}`,
        title:  name,
        icon:   <ThunderboltOutlined style={{ color: CLR_SECONDARY, fontSize: 11 }} />,
        isLeaf: true,
      })),
    },
  ];
}

export default function AccountPanel() {
  const exportDir = useGitStore((s) => s.exportDir);

  const [loaded,     setLoaded]     = useState(false);
  const [loading,    setLoading]    = useState(false);
  const [error,      setError]      = useState<string | null>(null);
  const [roles,      setRoles]      = useState<string[]>([]);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [exporting,  setExporting]  = useState(false);
  const [ddlModal,   setDdlModal]   = useState<DdlModal | null>(null);
  const [ctxMenu,    setCtxMenu]    = useState<AccountCtxMenu | null>(null);
  const [propsModal, setPropsModal] = useState<{ title: string; rows: main.PropertyPair[] | null; error: string | null } | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);
  const ctxRef = useRef<HTMLDivElement>(null);

  const pendingDiff   = useDiffStore((s) => s.pending);
  const selectForComp = useDiffStore((s) => s.selectForComparison);
  const compareWith   = useDiffStore((s) => s.compareWith);

  // ── Loading ──────────────────────────────────────────────────────────────

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    const [rolesRes, whRes] = await Promise.allSettled([ListRoles(), ListWarehouses()]);
    setRoles(rolesRes.status      === "fulfilled" ? (rolesRes.value ?? []) : []);
    setWarehouses(whRes.status    === "fulfilled" ? (whRes.value   ?? []) : []);
    if (rolesRes.status === "rejected" && whRes.status === "rejected") {
      setError(String(rolesRes.reason));
    }
    setLoaded(true);
    setLoading(false);
  };

  const loadIfNeeded = () => {
    if (!loaded && !loading) fetchData();
  };

  const refresh = () => {
    setLoaded(false);
    setRoles([]);
    setWarehouses([]);
    setError(null);
    fetchData();
  };

  // ── Context menu ─────────────────────────────────────────────────────────

  const onRightClick = ({ event, node }: { event: React.MouseEvent; node: DataNode }) => {
    event.preventDefault();
    const key = String(node.key);
    if (key.startsWith("role:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, kind: "role", name: key.slice("role:".length) });
    } else if (key.startsWith("warehouse:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, kind: "warehouse", name: key.slice("warehouse:".length) });
    }
  };

  // Clamp context menu inside the viewport (runs before browser paint — no flash).
  useLayoutEffect(() => {
    if (!ctxMenu || !ctxRef.current) return;
    const el = ctxRef.current;
    const { width, height } = el.getBoundingClientRect();
    const pad = 8;
    const left = Math.max(pad, Math.min(ctxMenu.x, window.innerWidth  - width  - pad));
    const top  = Math.max(pad, Math.min(ctxMenu.y, window.innerHeight - height - pad));
    el.style.left = `${left}px`;
    el.style.top  = `${top}px`;
  }, [ctxMenu]);

  const openProperties = async (kind: "role" | "warehouse", name: string) => {
    setCtxMenu(null);
    const sfKind = kind === "role" ? "ROLE" : "WAREHOUSE";
    setPropsModal({ title: `Properties: ${sfKind} — ${name}`, rows: null, error: null });
    try {
      const rows = await GetObjectProperties("", "", sfKind, name);
      setPropsModal((prev) => prev ? { ...prev, rows: rows ?? [] } : null);
    } catch (e) {
      setPropsModal((prev) => prev ? { ...prev, rows: [], error: String(e) } : null);
    }
  };

  const selectForComparison = () => {
    if (!ctxMenu) return;
    const { kind, name } = ctxMenu;
    setCtxMenu(null);
    selectForComp({
      category: kind,
      label:    `${kind.toUpperCase()}: ${name}`,
      name,
    });
    message.success(`Selected for comparison: ${name}`);
  };

  const compareWithSelected = () => {
    if (!ctxMenu) return;
    const { kind, name } = ctxMenu;
    setCtxMenu(null);
    compareWith({
      category: kind,
      label:    `${kind.toUpperCase()}: ${name}`,
      name,
    });
  };

  // ── DDL view ─────────────────────────────────────────────────────────────

  const onSelect = async (_keys: Key[], info: { node: DataNode }) => {
    const key = String(info.node.key);

    if (key.startsWith("role:")) {
      const name = key.slice("role:".length);
      setDdlModal({ title: `ROLE: ${name}`, src: "", loading: true, error: null });
      try {
        const src = await GetRoleDDL(name);
        setDdlModal((prev) => prev ? { ...prev, src, loading: false } : null);
      } catch (e) {
        setDdlModal((prev) => prev ? { ...prev, error: String(e), loading: false } : null);
      }
    } else if (key.startsWith("warehouse:")) {
      const name = key.slice("warehouse:".length);
      setDdlModal({ title: `WAREHOUSE: ${name}`, src: "", loading: true, error: null });
      try {
        const src = await GetWarehouseDDL(name);
        setDdlModal((prev) => prev ? { ...prev, src, loading: false } : null);
      } catch (e) {
        setDdlModal((prev) => prev ? { ...prev, error: String(e), loading: false } : null);
      }
    }
  };

  // ── Export ───────────────────────────────────────────────────────────────

  const exportAll = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!exportDir) {
      message.warning("Set a working directory in the Git section below.");
      return;
    }
    setExporting(true);
    try {
      const result = await ExportAccountObjectsDDL(exportDir);
      const errs   = result.errors?.length ?? 0;
      const summary = `${result.roles} role${result.roles !== 1 ? "s" : ""}, ${result.warehouses} warehouse${result.warehouses !== 1 ? "s" : ""} exported.`;
      if (errs > 0) {
        message.warning(`${summary} ${errs} error(s).`);
      } else {
        message.success(summary);
      }
      window.dispatchEvent(new CustomEvent("thaw:export-complete"));
    } catch (err) {
      message.error(String(err));
    } finally {
      setExporting(false);
    }
  };

  // ── Render ───────────────────────────────────────────────────────────────

  const treeData = buildTree(roles, warehouses);

  return (
    <div style={{ borderTop: `1px solid ${CLR_BORDER}` }}>
      <Collapse
        ghost
        defaultActiveKey={[]}
        style={{ background: "transparent" }}
        onChange={(keys) => {
          if ((Array.isArray(keys) ? keys : [keys]).includes("account")) loadIfNeeded();
        }}
        items={[{
          key:   "account",
          label: (
            <Space size={6}>
              <TeamOutlined style={{ color: "var(--text)", fontSize: 13 }} />
              <Text style={{ fontSize: 11, color: "var(--text)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
                Administration
              </Text>
            </Space>
          ),
          style: { border: "none" },
          extra: (
            <Space size={2} onClick={(e) => e.stopPropagation()}>
              <Button
                size="small"
                type="text"
                icon={<HistoryOutlined style={{ fontSize: 11 }} />}
                title="Query Activity"
                onClick={() => setHistoryOpen(true)}
                style={{ height: 18, padding: "0 4px", minWidth: 0 }}
              />
              {loaded && <>
                <Button
                  size="small"
                  type="text"
                  icon={<ExportOutlined style={{ fontSize: 11 }} />}
                  loading={exporting}
                  title="Export roles & warehouses to files"
                  onClick={exportAll}
                  style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                />
                <Button
                  size="small"
                  type="text"
                  icon={<ReloadOutlined style={{ fontSize: 11 }} />}
                  loading={loading}
                  onClick={(e) => { e.stopPropagation(); refresh(); }}
                  style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                />
              </>}
            </Space>
          ),
          children: (
            <div style={{ padding: "0 4px 8px" }}>
              {loading && (
                <Spin size="small" style={{ display: "block", margin: "8px auto" }} />
              )}

              {!loading && error && (
                <Text style={{ fontSize: 11, color: "#f85149", display: "block", padding: "0 8px" }}>
                  {error}
                </Text>
              )}

              {!loading && loaded && (
                <div style={{ overflow: "hidden" }}>
                  <Tree
                    treeData={treeData}
                    onSelect={onSelect as any}
                    onRightClick={onRightClick as any}
                    defaultExpandAll
                    showIcon
                    blockNode
                    style={{ background: "transparent", color: "var(--text)", fontSize: 12 }}
                  />
                </div>
              )}

              {loaded && <UserManagementPanel />}
              {loaded && <BackupPoliciesPanel />}
            </div>
          ),
        }]}
      />

      {/* Account object context menu */}
      {ctxMenu && (
        <div
          ref={ctxRef}
          style={{
            position: "fixed",
            top: ctxMenu.y,
            left: ctxMenu.x,
            zIndex: 9999,
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
            minWidth: 160,
            padding: "4px 0",
          }}
          onClick={(e) => e.stopPropagation()}
          onMouseLeave={() => setCtxMenu(null)}
        >
          <div
            style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "var(--text)" }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
            onClick={() => openProperties(ctxMenu.kind, ctxMenu.name)}
          >
            <FileOutlined style={{ fontSize: 12 }} />
            Properties
          </div>
          <div
            style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "var(--text)" }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
            onClick={selectForComparison}
          >
            <DiffOutlined style={{ fontSize: 12 }} />
            Select for Comparison
          </div>
          {pendingDiff !== null && (
            <div
              style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "var(--text)" }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
              onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              onClick={compareWithSelected}
            >
              <DiffOutlined style={{ fontSize: 12, color: "var(--accent)" }} />
              {`Compare with: ${pendingDiff.label}`}
            </div>
          )}
        </div>
      )}

      {/* Query history modal */}
      {historyOpen && <QueryHistoryModal onClose={() => setHistoryOpen(false)} />}

      {/* Properties modal */}
      {propsModal && (
        <PropertiesModal
          title={propsModal.title}
          rows={propsModal.rows}
          error={propsModal.error}
          onClose={() => setPropsModal(null)}
        />
      )}

      {/* DDL modal */}
      <Modal
        open={ddlModal !== null}
        title={ddlModal?.title}
        onCancel={() => setDdlModal(null)}
        footer={null}
        width={780}
        styles={{ body: { padding: 0 } }}
      >
        {ddlModal?.loading && (
          <div style={{ padding: 32, textAlign: "center" }}>
            <Spin />
          </div>
        )}
        {ddlModal?.error && (
          <div style={{ padding: 16, color: "#f85149", fontFamily: "monospace", fontSize: 12 }}>
            {ddlModal.error}
          </div>
        )}
        {!ddlModal?.loading && !ddlModal?.error && ddlModal?.src && (
          <div style={{ position: "relative" }}>
            {/* Copy button */}
            <Button
              size="small"
              icon={<CopyOutlined />}
              title="Copy to clipboard"
              style={{
                position: "absolute",
                top: 10,
                right: 10,
                zIndex: 1,
                background: "var(--border)",
                border: "1px solid var(--text-faint)",
                color: "var(--text)",
              }}
              onClick={async () => {
                await ClipboardSetText(ddlModal.src);
                message.success("Copied to clipboard");
              }}
            />
            {/* DDL text — userSelect overrides Wails' global user-select:none */}
            <pre
              style={{
                margin: 0,
                padding: 16,
                background: "var(--bg)",
                color: "var(--text)",
                fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
                fontSize: 12,
                lineHeight: 1.6,
                overflowX: "auto",
                maxHeight: "60vh",
                overflowY: "auto",
                borderRadius: "0 0 6px 6px",
                userSelect: "text",
                WebkitUserSelect: "text",
              } as React.CSSProperties}
              onContextMenu={async (e) => {
                e.preventDefault();
                const selection = window.getSelection()?.toString().trim();
                const text = selection || ddlModal.src;
                await ClipboardSetText(text);
                message.success(selection ? "Selection copied" : "Copied to clipboard");
              }}
            >
              {ddlModal.src}
            </pre>
          </div>
        )}
      </Modal>
    </div>
  );
}
