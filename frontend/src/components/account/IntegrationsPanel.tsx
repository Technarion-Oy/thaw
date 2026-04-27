// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties   holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useLayoutEffect, useRef } from "react";
import { useConnectionStore } from "../../store/connectionStore";
import { Collapse, Space, Typography, Tree, Spin, Popconfirm, message } from "antd";
import {
  ApiOutlined,
  FileProtectOutlined,
  ReloadOutlined,
  PlusOutlined,
  DeleteOutlined,
  FileOutlined,
  EditOutlined,
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import {
  ListIntegrations,
  DropIntegration,
  CanCreateIntegration,
} from "../../../wailsjs/go/main/App";
import type { snowflake } from "../../../wailsjs/go/models";
import PropertiesModal from "../common/PropertiesModal";
import CreateIntegrationModal from "./CreateIntegrationModal";
import IntegrationModifyModal from "./IntegrationModifyModal";
import { GetIntegrationProperties } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";

const { Text } = Typography;
const CLR_SECONDARY = "var(--text-muted)";

const KINDS: { kind: string; label: string }[] = [
  { kind: "STORAGE",         label: "Storage Integrations" },
  { kind: "API",             label: "API Integrations" },
  { kind: "CATALOG",         label: "Catalog Integrations" },
  { kind: "EXTERNAL ACCESS", label: "External Access Integrations" },
  { kind: "NOTIFICATION",    label: "Notification Integrations" },
  { kind: "SECURITY",        label: "Security Integrations" },
];

interface CtxMenuState {
  x: number;
  y: number;
  type: "category" | "integration";
  kind: string;
  name?: string;
}

const TRUNCATE: React.CSSProperties = {
  overflow:     "hidden",
  textOverflow: "ellipsis",
  whiteSpace:   "nowrap",
  display:      "block",
};

function buildTreeData(
  loadedKinds: Set<string>,
  childrenMap: Map<string, snowflake.IntegrationRow[]>,
): DataNode[] {
  return KINDS.map(({ kind, label }) => {
    const rows = childrenMap.get(kind);
    const countSuffix = loadedKinds.has(kind) && rows ? ` (${rows.length})` : "";
    const children: DataNode[] | undefined = rows
      ? rows.map((r) => ({
          key:   `integration:${kind}:${r.name}`,
          title: <span style={TRUNCATE} title={r.name}>{r.name}</span>,
          icon:  <ApiOutlined style={{ color: CLR_SECONDARY, fontSize: 11 }} />,
          isLeaf: true,
        }))
      : undefined;
    return {
      key:    `category:${kind}`,
      title:  <span style={TRUNCATE} title={label + countSuffix}>{label}{countSuffix}</span>,
      icon:   <FileProtectOutlined style={{ color: CLR_SECONDARY }} />,
      isLeaf: false,
      children,
    };
  });
}

export default function IntegrationsPanel() {
  const [loadedKinds, setLoadedKinds] = useState<Set<string>>(new Set());
  const [loadingKinds, setLoadingKinds] = useState<Set<string>>(new Set());
  const [childrenMap, setChildrenMap] = useState<Map<string, snowflake.IntegrationRow[]>>(new Map());
  const [canCreate, setCanCreate] = useState(false);
  const [ctxMenu,   setCtxMenu]   = useState<CtxMenuState | null>(null);
  const ctxRef = useRef<HTMLDivElement>(null);
  const isConnected = useConnectionStore((s) => s.isConnected);

  // Modal state
  const [createOpen,    setCreateOpen]    = useState<{ kind: string } | null>(null);
  const [propertiesFor, setPropertiesFor] = useState<string | null>(null);
  const [propsData,     setPropsData]     = useState<{ rows: main.PropertyPair[] | null; error: string | null } | null>(null);
  const [modifyFor,     setModifyFor]     = useState<string | null>(null);
  const [dropConfirm,   setDropConfirm]   = useState<string | null>(null);
  const [dropKind,      setDropKind]      = useState<string>("");

  useEffect(() => {
    CanCreateIntegration().then(setCanCreate).catch(() => {});
  }, [isConnected]);

  // Clamp context menu in viewport before paint
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

  const loadKind = async (kind: string) => {
    if (loadingKinds.has(kind)) return;
    setLoadingKinds((prev) => new Set(prev).add(kind));
    try {
      const rows = await ListIntegrations(kind);
      setChildrenMap((prev) => {
        const next = new Map(prev);
        next.set(kind, rows ?? []);
        return next;
      });
      setLoadedKinds((prev) => new Set(prev).add(kind));
    } catch (e) {
      message.error(`Failed to load ${kind} integrations: ${String(e)}`);
    } finally {
      setLoadingKinds((prev) => {
        const next = new Set(prev);
        next.delete(kind);
        return next;
      });
    }
  };

  const reloadKind = (kind: string) => {
    setLoadedKinds((prev) => { const s = new Set(prev); s.delete(kind); return s; });
    loadKind(kind);
  };

  const onLoadData = (node: DataNode): Promise<void> => {
    const key = String(node.key);
    if (!key.startsWith("category:")) return Promise.resolve();
    const kind = key.slice("category:".length);
    if (loadedKinds.has(kind)) return Promise.resolve();
    return loadKind(kind);
  };

  const onRightClick = ({ event, node }: { event: React.MouseEvent; node: DataNode }) => {
    event.preventDefault();
    const key = String(node.key);
    if (key.startsWith("category:")) {
      const kind = key.slice("category:".length);
      setCtxMenu({ x: event.clientX, y: event.clientY, type: "category", kind });
    } else if (key.startsWith("integration:")) {
      // key = "integration:{KIND}:{name}"
      const rest = key.slice("integration:".length);
      // kind may contain spaces (e.g. "EXTERNAL ACCESS"), name follows the second ":"
      // We find the kind by matching KINDS list
      const found = KINDS.find(({ kind }) => rest.startsWith(kind + ":"));
      if (!found) return;
      const name = rest.slice(found.kind.length + 1);
      setCtxMenu({ x: event.clientX, y: event.clientY, type: "integration", kind: found.kind, name });
    }
  };

  const closeCtx = () => setCtxMenu(null);

  const openProperties = async (name: string) => {
    closeCtx();
    setPropertiesFor(name);
    setPropsData({ rows: null, error: null });
    try {
      const rows = await GetIntegrationProperties(name);
      setPropsData({ rows: rows ?? [], error: null });
    } catch (e) {
      setPropsData({ rows: [], error: String(e) });
    }
  };

  const openModify = (name: string) => {
    closeCtx();
    setModifyFor(name);
  };

  const startDrop = (kind: string, name: string) => {
    closeCtx();
    setDropKind(kind);
    setDropConfirm(name);
  };

  const confirmDrop = async () => {
    if (!dropConfirm) return;
    const kind = dropKind;
    const name = dropConfirm;
    setDropConfirm(null);
    try {
      await DropIntegration(name);
      message.success(`Integration "${name}" dropped`);
      reloadKind(kind);
    } catch (e) {
      message.error(String(e));
    }
  };

  const cancelDrop = () => setDropConfirm(null);

  const treeData = buildTreeData(loadedKinds, childrenMap);

  const anyLoading = loadingKinds.size > 0;

  return (
    <div style={{ borderTop: "1px solid var(--border)" }}>
      <Collapse
        ghost
        defaultActiveKey={[]}
        style={{ background: "transparent" }}
        items={[{
          key:   "integrations",
          label: (
            <Space size={6}>
              <FileProtectOutlined style={{ color: "var(--text)", fontSize: 13 }} />
              <Text style={{ fontSize: 11, color: "var(--text)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
                Integrations
              </Text>
            </Space>
          ),
          style: { border: "none" },
          extra: anyLoading ? (
            <Spin size="small" style={{ marginRight: 4 }} />
          ) : (
            <ReloadOutlined
              style={{ fontSize: 11, color: "var(--text-muted)", cursor: "pointer" }}
              title="Reload all loaded categories"
              onClick={(e) => {
                e.stopPropagation();
                loadedKinds.forEach((k) => reloadKind(k));
                CanCreateIntegration().then(setCanCreate).catch(() => {});
              }}
            />
          ),
          children: (
            <div
              style={{ padding: "0 4px 8px" }}
              onClick={() => ctxMenu && closeCtx()}
            >
              <Tree
                treeData={treeData}
                loadData={onLoadData as any}
                onRightClick={onRightClick as any}
                showIcon
                blockNode
                style={{ background: "transparent", color: "var(--text)", fontSize: 12 }}
              />
            </div>
          ),
        }]}
      />

      {/* Drop popconfirm — rendered as a hidden element we programmatically trigger */}
      <Popconfirm
        open={dropConfirm !== null}
        title={`Drop integration "${dropConfirm}"?`}
        description="This action cannot be undone."
        okText="Drop"
        okButtonProps={{ danger: true }}
        onConfirm={confirmDrop}
        onCancel={cancelDrop}
      >
        <span />
      </Popconfirm>

      {/* Context menu */}
      {ctxMenu && (
        <div
          ref={ctxRef}
          style={{
            position: "fixed",
            top:  ctxMenu.y,
            left: ctxMenu.x,
            zIndex: 9999,
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
            minWidth: 200,
            padding: "4px 0",
          }}
          onClick={(e) => e.stopPropagation()}
          onMouseLeave={closeCtx}
        >
          {ctxMenu.type === "category" ? (
            <div
              style={{
                display: "flex", alignItems: "center", gap: 8,
                padding: "6px 14px", fontSize: 13, cursor: "pointer",
                color: canCreate ? "var(--text)" : "var(--text-faint)",
                pointerEvents: canCreate ? "auto" : "none",
                opacity: canCreate ? 1 : 0.45,
              }}
              onMouseEnter={(e) => canCreate && (e.currentTarget.style.background = "var(--border)")}
              onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              onClick={() => {
                if (!canCreate) return;
                closeCtx();
                setCreateOpen({ kind: ctxMenu.kind });
              }}
            >
              <PlusOutlined style={{ fontSize: 12 }} />
              {`Create New ${ctxMenu.kind.charAt(0) + ctxMenu.kind.slice(1).toLowerCase()} Integration`}
            </div>
          ) : (
            <>
              <div
                style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "var(--text)" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                onClick={() => ctxMenu.name && openProperties(ctxMenu.name)}
              >
                <FileOutlined style={{ fontSize: 12 }} />
                Properties
              </div>
              <div
                style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "var(--text)" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                onClick={() => ctxMenu.name && openModify(ctxMenu.name)}
              >
                <EditOutlined style={{ fontSize: 12 }} />
                Modify
              </div>
              <div
                style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "#f85149" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                onClick={() => ctxMenu.name && startDrop(ctxMenu.kind, ctxMenu.name)}
              >
                <DeleteOutlined style={{ fontSize: 12 }} />
                Drop
              </div>
            </>
          )}
        </div>
      )}

      {/* Properties modal */}
      {propertiesFor && propsData && (
        <PropertiesModal
          title={`Properties: ${propertiesFor}`}
          rows={propsData.rows}
          error={propsData.error}
          onClose={() => { setPropertiesFor(null); setPropsData(null); }}
        />
      )}

      {/* Modify modal */}
      {modifyFor && (
        <IntegrationModifyModal
          name={modifyFor}
          onClose={() => setModifyFor(null)}
          onSuccess={() => {
            // find which kind this integration belongs to and reload it
            for (const [k, rows] of childrenMap.entries()) {
              if (rows.some((r) => r.name === modifyFor)) {
                reloadKind(k);
                break;
              }
            }
          }}
        />
      )}

      {/* Create modal */}
      {createOpen && (
        <CreateIntegrationModal
          kind={createOpen.kind}
          onClose={() => setCreateOpen(null)}
          onSuccess={() => reloadKind(createOpen.kind)}
        />
      )}
    </div>
  );
}
