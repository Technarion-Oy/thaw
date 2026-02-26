// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState } from "react";
import { Collapse, Space, Button, Typography, Tree, Spin, Modal, message } from "antd";
import {
  TeamOutlined,
  ThunderboltOutlined,
  ReloadOutlined,
  ExportOutlined,
  CopyOutlined,
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import {
  ListRoles,
  ListWarehouses,
  GetRoleDDL,
  GetWarehouseDDL,
  ExportAccountObjectsDDL,
} from "../../../wailsjs/go/main/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { useGitStore } from "../../store/gitStore";

const { Text } = Typography;
const CLR_BORDER    = "var(--border)";
const CLR_SECONDARY = "var(--text-muted)";

interface DdlModal {
  title: string;
  src: string;
  loading: boolean;
  error: string | null;
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
              <TeamOutlined style={{ color: CLR_SECONDARY, fontSize: 13 }} />
              <Text style={{ fontSize: 11, color: CLR_SECONDARY, textTransform: "uppercase", letterSpacing: "0.08em" }}>
                Account Objects
              </Text>
            </Space>
          ),
          style: { border: "none" },
          extra: loaded ? (
            <Space size={2}>
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
            </Space>
          ) : undefined,
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
                    defaultExpandAll
                    showIcon
                    blockNode
                    style={{ background: "transparent", color: "var(--text)", fontSize: 12 }}
                  />
                </div>
              )}
            </div>
          ),
        }]}
      />

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
