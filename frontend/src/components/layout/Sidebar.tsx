// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef } from "react";
import { Tree, Typography, Spin, Empty, Divider, Modal, Button, Input, Tooltip, Slider, message } from "antd";
import {
  DatabaseOutlined,
  TableOutlined,
  EyeOutlined,
  FunctionOutlined,
  CodeOutlined,
  OrderedListOutlined,
  InboxOutlined,
  ApiOutlined,
  ClockCircleOutlined,
  FileOutlined,
  FolderOutlined,
  ReloadOutlined,
  PlayCircleOutlined,
  CloudUploadOutlined,
  DeleteOutlined,
  RollbackOutlined,
  EditOutlined,
  HistoryOutlined,
  ApartmentOutlined,
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import { ListDatabases, ListSchemas, ListObjects, GetObjectDDL, ExportDatabaseDDL, ListDroppedTables, GetTableRetentionDays, GetERDiagramData } from "../../../wailsjs/go/main/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { useQueryStore } from "../../store/queryStore";
import { useObjectStore } from "../../store/objectStore";
import { useGitStore } from "../../store/gitStore";
import AccountPanel from "../account/AccountPanel";
import CallProcedureModal from "../procedure/CallProcedureModal";
import ERDiagramModal from "../er/ERDiagramModal";

const { Text } = Typography;

const KIND_LABEL: Record<string, string> = {
  TABLE:         "Tables",
  VIEW:          "Views",
  FUNCTION:      "Functions",
  PROCEDURE:     "Procedures",
  SEQUENCE:      "Sequences",
  STAGE:         "Stages",
  STREAM:        "Streams",
  TASK:          "Tasks",
  "FILE FORMAT": "File Formats",
  PIPE:          "Pipes",
};

const KIND_ORDER = ["TABLE", "VIEW", "FUNCTION", "PROCEDURE", "SEQUENCE", "STAGE", "STREAM", "TASK", "FILE FORMAT", "PIPE"];

function kindIcon(kind: string) {
  switch (kind) {
    case "TABLE":       return <TableOutlined />;
    case "VIEW":        return <EyeOutlined />;
    case "FUNCTION":    return <FunctionOutlined />;
    case "PROCEDURE":   return <CodeOutlined />;
    case "SEQUENCE":    return <OrderedListOutlined />;
    case "STAGE":       return <InboxOutlined />;
    case "STREAM":      return <ApiOutlined />;
    case "TASK":        return <ClockCircleOutlined />;
    case "FILE FORMAT": return <FileOutlined />;
    case "PIPE":        return <ApiOutlined />;
    default:            return <FileOutlined />;
  }
}

interface ContextMenu {
  x: number;
  y: number;
  nodeKey: string;
  nodeType: "db" | "schema" | "obj";
  objKind?: string;  // set for nodeType === "obj"
  objArgs?: string;  // parameter type list for PROCEDURE / FUNCTION
}

interface UndropModal {
  db: string;
  schema: string;
  tables: snowflake.DroppedTable[] | null; // null = loading
  error: string | null;
}

interface RenameModal {
  db: string;
  schema: string;
  kind: string;
  oldName: string;
  newName: string;
}

interface TimeTravelModal {
  db: string;
  schema: string;
  name: string;
  retentionDays: number | null; // null = still loading
  minTs: number;   // Unix seconds — oldest queryable point
  maxTs: number;   // Unix seconds — now
  selectedTs: number; // Unix seconds — slider position
}

interface ObjectDDL {
  title: string;
  src: string;
  loading: boolean;
  error: string | null;
}

// Strip all children from a node so Tree will re-call loadData on next expand.
function clearNodeChildren(nodes: DataNode[], targetKey: string): DataNode[] {
  return nodes.map((node) => {
    if (node.key === targetKey) {
      const { children: _removed, ...rest } = node as DataNode & { children?: DataNode[] };
      return rest;
    }
    if ((node as any).children) {
      return { ...node, children: clearNodeChildren((node as any).children, targetKey) };
    }
    return node;
  });
}

// Cache DDL per unique key so we only fetch once per session.
const ddlCache = new Map<string, string>();

function ObjTooltip({ cacheKey, db, schema, kind, name, args, children }: {
  cacheKey: string;
  db: string;
  schema: string;
  kind: string;
  name: string;
  args: string;
  children: React.ReactNode;
}) {
  const [content, setContent] = useState<string | null>(() => ddlCache.get(cacheKey) ?? null);
  const [loading, setLoading] = useState(false);

  const onOpenChange = (open: boolean) => {
    if (!open || content !== null || loading) return;
    setLoading(true);
    GetObjectDDL(db, schema, kind, name, args)
      .then((src) => {
        const text = src || "(empty)";
        ddlCache.set(cacheKey, text);
        setContent(text);
      })
      .catch((e) => {
        const text = `Error: ${e}`;
        ddlCache.set(cacheKey, text);
        setContent(text);
      })
      .finally(() => setLoading(false));
  };

  const overlay = (
    <pre
      style={{
        margin: 0,
        fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
        fontSize: 11,
        lineHeight: 1.55,
        whiteSpace: "pre-wrap",
        wordBreak: "break-word",
        maxHeight: 340,
        overflowY: "auto",
        color: "var(--text)",
      }}
    >
      {loading ? "Loading…" : content}
    </pre>
  );

  return (
    <Tooltip
      title={overlay}
      placement="left"
      mouseEnterDelay={0.6}
      mouseLeaveDelay={0.1}
      onOpenChange={onOpenChange}
      overlayStyle={{ maxWidth: 540 }}
      overlayInnerStyle={{
        background: "var(--bg-overlay)",
        border: "1px solid var(--border)",
        borderRadius: 6,
        padding: "8px 12px",
        boxShadow: "0 4px 16px rgba(0,0,0,0.45)",
      }}
    >
      <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
        {children}
      </span>
    </Tooltip>
  );
}

export default function Sidebar() {
  const [treeData, setTreeData]     = useState<DataNode[]>([]);
  const [loadedKeys, setLoadedKeys] = useState<Key[]>([]);
  const [loading, setLoading]       = useState(false);
  const [loaded, setLoaded]         = useState(false);

  const [ctxMenu, setCtxMenu]     = useState<ContextMenu | null>(null);
  const [ddlModal, setDdlModal]   = useState<ObjectDDL | null>(null);
  const [callModal, setCallModal] = useState<{ db: string; schema: string; name: string; rawArgs: string } | null>(null);
  const [undropModal, setUndropModal] = useState<UndropModal | null>(null);
  const [renameModal, setRenameModal] = useState<RenameModal | null>(null);
  const [timeTravelModal, setTimeTravelModal] = useState<TimeTravelModal | null>(null);
  const [erModal, setErModal] = useState<{ database: string; data: snowflake.ERDiagramData } | null>(null);
  const ctxRef = useRef<HTMLDivElement>(null);

  // Close context menu on outside click
  useEffect(() => {
    if (!ctxMenu) return;
    const close = () => setCtxMenu(null);
    window.addEventListener("click", close);
    return () => window.removeEventListener("click", close);
  }, [ctxMenu]);

  const loadDatabases = async () => {
    if (loaded) return;
    setLoading(true);
    try {
      const dbs = await ListDatabases();
      setTreeData(
        dbs.map((db) => ({
          title: db,
          key: `db:${db}`,
          icon: <DatabaseOutlined />,
          isLeaf: false,
        }))
      );
      useObjectStore.getState().setDatabases(dbs);
      setLoaded(true);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const onLoadData = async (node: DataNode & { children?: DataNode[] }) => {
    if (node.children) return;
    const key   = String(node.key);
    const parts = key.split(":");

    if (parts[0] === "db") {
      const db      = parts[1];
      const schemas = await ListSchemas(db);
      setTreeData((prev) =>
        updateNode(prev, key, schemas.map((s) => ({
          title:  s,
          key:    `schema:${db}:${s}`,
          icon:   <FolderOutlined />,
          isLeaf: false,
        })))
      );
      useObjectStore.getState().addSchemas(db, schemas);
    } else if (parts[0] === "schema") {
      const [, db, schema] = parts;
      const objects        = await ListObjects(db, schema);

      const groups: Record<string, typeof objects> = {};
      for (const obj of objects) {
        const k = (obj.kind || "OTHER").toUpperCase();
        if (!groups[k]) groups[k] = [];
        groups[k].push(obj);
      }

      const sortedKinds = [
        ...KIND_ORDER.filter((k) => groups[k]),
        ...Object.keys(groups).filter((k) => !KIND_ORDER.includes(k)).sort(),
      ];

      const typeNodes: DataNode[] = sortedKinds.map((kind) => ({
        title:    KIND_LABEL[kind] ?? kind,
        key:      `type:${db}:${schema}:${kind}`,
        icon:     <FolderOutlined style={{ color: "var(--text-muted)" }} />,
        children: groups[kind].map((o) => ({
          title:     o.name,
          key:       `obj:${db}:${schema}:${kind}:${o.name}`,
          icon:      kindIcon(kind),
          isLeaf:    true,
          arguments: o.arguments ?? "",
        })),
      }));

      setTreeData((prev) => updateNode(prev, key, typeNodes));
      useObjectStore.getState().addObjects(db, schema, objects.map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })));
    }
  };

  function updateNode(nodes: DataNode[], targetKey: string, children: DataNode[]): DataNode[] {
    return nodes.map((node) => {
      if (node.key === targetKey) return { ...node, children };
      if ((node as any).children) return { ...node, children: updateNode((node as any).children, targetKey, children) };
      return node;
    });
  }

  // ── Context menu ────────────────────────────────────────────────────────────

  const onRightClick = ({ event, node }: { event: React.MouseEvent; node: DataNode }) => {
    event.preventDefault();
    const key = String(node.key);
    if (key.startsWith("db:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "db" });
    } else if (key.startsWith("schema:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "schema" });
    } else if (key.startsWith("obj:")) {
      // key format: obj:DB:SCHEMA:KIND:NAME
      const objKind = key.split(":")[3];
      const objArgs = (node as any).arguments ?? "";
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "obj", objKind, objArgs });
    }
  };

  const refreshDatabaseByName = (db: string) => {
    const dbKey = `db:${db}`;
    useObjectStore.getState().clearDatabase(db);
    // Remove every key that belongs to this database from loadedKeys.
    // Schema keys look like "schema:DBNAME:SCHEMANAME" — a different prefix
    // from "db:DBNAME" — so they must be evicted separately; otherwise Tree
    // sees them as already-loaded and never calls loadData for them again.
    setLoadedKeys((prev) =>
      prev.filter((k) => {
        const s = String(k);
        return !s.startsWith(dbKey) && !s.startsWith(`schema:${db}:`);
      })
    );
    // Strip children from treeData — Tree won't call loadData for a node
    // that still has a children array even if its key left loadedKeys.
    setTreeData((prev) => clearNodeChildren(prev, dbKey));
  };

  const refreshDatabase = () => {
    if (!ctxMenu) return;
    const db = ctxMenu.nodeKey.slice("db:".length);
    setCtxMenu(null);
    refreshDatabaseByName(db);
  };

  const showDroppedTables = async () => {
    if (!ctxMenu) return;
    // key format: schema:DB:SCHEMA
    const [, db, schema] = ctxMenu.nodeKey.split(":");
    setCtxMenu(null);
    setUndropModal({ db, schema, tables: null, error: null });
    try {
      const tables = await ListDroppedTables(db, schema);
      setUndropModal((prev) => prev ? { ...prev, tables: tables ?? [] } : null);
    } catch (e) {
      setUndropModal((prev) => prev ? { ...prev, tables: [], error: String(e) } : null);
    }
  };

  const undropTable = async (db: string, schema: string, name: string) => {
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const sql = `UNDROP TABLE ${q(db)}.${q(schema)}.${q(name)};`;
    setUndropModal(null);
    await useQueryStore.getState().executeWith(sql);
    refreshDatabaseByName(db);
  };

  const selectTop1000 = () => {
    if (!ctxMenu) return;
    setCtxMenu(null);

    // key format: obj:DB:SCHEMA:KIND:NAME
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    const sql = `SELECT * FROM "${db}"."${schema}"."${name}" LIMIT 1000;`;

    useQueryStore.getState().executeInNewTab(sql);
  };

  const callProcedure = () => {
    if (!ctxMenu) return;
    const { nodeKey, objArgs = "" } = ctxMenu;
    setCtxMenu(null);
    // key format: obj:DB:SCHEMA:KIND:NAME
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    setCallModal({ db, schema, name, rawArgs: objArgs });
  };

  const exportDatabase = async () => {
    if (!ctxMenu) return;
    const db = ctxMenu.nodeKey.slice("db:".length);
    setCtxMenu(null);
    const exportDir = useGitStore.getState().exportDir;
    if (!exportDir) {
      message.warning("Set a working directory in the Git panel first.");
      return;
    }
    const hide = message.loading(`Exporting ${db}…`, 0);
    try {
      const result = await ExportDatabaseDDL(db, exportDir);
      hide();
      const errs = result.errors?.length ?? 0;
      if (errs > 0) {
        message.warning(`${db}: ${result.files} files, ${errs} error(s)`);
      } else {
        message.success(`${db}: ${result.files} files written`);
      }
    } catch (e) {
      hide();
      message.error(String(e));
    }
  };

  const generateERDiagram = async () => {
    if (!ctxMenu) return;
    const db = ctxMenu.nodeKey.slice("db:".length);
    setCtxMenu(null);
    const hide = message.loading(`Loading ER diagram for ${db}…`, 0);
    try {
      const data = await GetERDiagramData(db);
      hide();
      setErModal({ database: db, data });
    } catch (e) {
      hide();
      message.error(String(e));
    }
  };

  const deleteObject = () => {
    if (!ctxMenu) return;
    const { nodeKey, objKind = "", objArgs = "" } = ctxMenu;
    setCtxMenu(null);

    // key format: obj:DB:SCHEMA:KIND:NAME
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");

    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const fullName = `${q(db)}.${q(schema)}.${q(name)}`;

    let sql: string;
    switch (objKind) {
      case "TABLE":       sql = `DROP TABLE ${fullName};`; break;
      case "VIEW":        sql = `DROP VIEW ${fullName};`; break;
      case "SEQUENCE":    sql = `DROP SEQUENCE ${fullName};`; break;
      case "STAGE":       sql = `DROP STAGE ${fullName};`; break;
      case "STREAM":      sql = `DROP STREAM ${fullName};`; break;
      case "TASK":        sql = `DROP TASK ${fullName};`; break;
      case "FILE FORMAT": sql = `DROP FILE FORMAT ${fullName};`; break;
      case "PIPE":        sql = `DROP PIPE ${fullName};`; break;
      case "FUNCTION":    sql = `DROP FUNCTION ${fullName}(${objArgs});`; break;
      case "PROCEDURE":   sql = `DROP PROCEDURE ${fullName}(${objArgs});`; break;
      default:            sql = `DROP ${objKind} ${fullName};`;
    }

    Modal.confirm({
      title: `Drop ${objKind.toLowerCase()} "${name}"?`,
      content: `This will permanently delete ${db}.${schema}.${name}. This action cannot be undone.`,
      okText: "Drop",
      okType: "danger",
      cancelText: "Cancel",
      onOk: async () => {
        await useQueryStore.getState().executeWith(sql);
        refreshDatabaseByName(db);
      },
    });
  };

  const renameObject = () => {
    if (!ctxMenu) return;
    const { nodeKey, objKind = "" } = ctxMenu;
    setCtxMenu(null);
    // key format: obj:DB:SCHEMA:KIND:NAME
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const oldName = nameParts.join(":");
    setRenameModal({ db, schema, kind: objKind, oldName, newName: oldName });
  };

  const executeRename = async () => {
    if (!renameModal) return;
    const { db, schema, kind, oldName, newName } = renameModal;
    const trimmed = newName.trim();
    if (!trimmed || trimmed === oldName) {
      setRenameModal(null);
      return;
    }
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const fullOld = `${q(db)}.${q(schema)}.${q(oldName)}`;
    const fullNew = `${q(db)}.${q(schema)}.${q(trimmed)}`;

    let sql: string;
    switch (kind) {
      case "TABLE":       sql = `ALTER TABLE ${fullOld} RENAME TO ${fullNew};`; break;
      case "VIEW":        sql = `ALTER VIEW ${fullOld} RENAME TO ${fullNew};`; break;
      case "SEQUENCE":    sql = `ALTER SEQUENCE ${fullOld} RENAME TO ${fullNew};`; break;
      case "STAGE":       sql = `ALTER STAGE ${fullOld} RENAME TO ${fullNew};`; break;
      case "STREAM":      sql = `ALTER STREAM ${fullOld} RENAME TO ${fullNew};`; break;
      case "TASK":        sql = `ALTER TASK ${fullOld} RENAME TO ${fullNew};`; break;
      case "FILE FORMAT": sql = `ALTER FILE FORMAT ${fullOld} RENAME TO ${fullNew};`; break;
      case "PIPE":        sql = `ALTER PIPE ${fullOld} RENAME TO ${fullNew};`; break;
      default:            sql = `ALTER ${kind} ${fullOld} RENAME TO ${fullNew};`;
    }

    setRenameModal(null);
    await useQueryStore.getState().executeWith(sql);
    refreshDatabaseByName(db);
  };

  const openTimeTravelModal = async () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);

    const maxTs = Math.floor(Date.now() / 1000);
    const defaultMin = maxTs - 86400; // 1 day fallback while loading
    setTimeTravelModal({ db, schema, name, retentionDays: null, minTs: defaultMin, maxTs, selectedTs: maxTs - 3600 });

    try {
      const days = await GetTableRetentionDays(db, schema, name);
      const retentionDays = Math.max(days, 1);
      const minTs = maxTs - retentionDays * 86400;
      setTimeTravelModal((prev) =>
        prev ? { ...prev, retentionDays, minTs, selectedTs: Math.max(prev.selectedTs, minTs) } : null,
      );
    } catch {
      setTimeTravelModal((prev) => prev ? { ...prev, retentionDays: 1 } : null);
    }
  };

  const executeTimeTravel = () => {
    if (!timeTravelModal) return;
    const { db, schema, name, selectedTs } = timeTravelModal;
    setTimeTravelModal(null);
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const sql = `SELECT * FROM ${q(db)}.${q(schema)}.${q(name)} AT(TIMESTAMP => TO_TIMESTAMP_NTZ(${selectedTs})) LIMIT 1000;`;
    useQueryStore.getState().executeInNewTab(sql);
  };

  const viewDefinition = async () => {
    if (!ctxMenu) return;
    const { nodeKey, objArgs = "" } = ctxMenu;
    setCtxMenu(null);

    // key format: obj:db:schema:kind:name
    const [, db, schema, kind, ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");

    setDdlModal({ title: `${kind}: ${db}.${schema}.${name}`, src: "", loading: true, error: null });
    try {
      const src = await GetObjectDDL(db, schema, kind, name, objArgs);
      setDdlModal((prev) => (prev ? { ...prev, src, loading: false } : null));
    } catch (e) {
      setDdlModal((prev) => (prev ? { ...prev, error: String(e), loading: false } : null));
    }
  };

  // ── Render ──────────────────────────────────────────────────────────────────

  const menuItem = (label: string, icon: React.ReactNode, onClick: () => void, color?: string) => (
    <div
      style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: color ?? "var(--text)" }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      onClick={onClick}
    >
      {icon}
      {label}
    </div>
  );

  return (
    <div style={{ padding: "8px 4px" }}>
      <Text type="secondary" style={{ fontSize: 11, padding: "0 12px", display: "block", marginBottom: 8, textTransform: "uppercase", letterSpacing: "0.08em" }}>
        Objects
      </Text>

      {loading && <Spin size="small" style={{ display: "block", margin: "16px auto" }} />}

      {!loaded && !loading && (
        <div style={{ padding: "16px 12px" }}>
          <Text type="secondary" style={{ cursor: "pointer", fontSize: 12 }} onClick={loadDatabases}>
            Click to load databases
          </Text>
        </div>
      )}

      {loaded && treeData.length === 0 && <Empty description="No databases" imageStyle={{ height: 40 }} />}

      {treeData.length > 0 && (
        <Tree
          treeData={treeData}
          loadedKeys={loadedKeys}
          onLoad={(keys) => setLoadedKeys(keys)}
          loadData={onLoadData as (node: DataNode) => Promise<void>}
          onRightClick={onRightClick as any}
          showIcon
          blockNode
          style={{ background: "transparent", color: "var(--text)" }}
          titleRender={(node) => {
            const key = String(node.key);
            if (key.startsWith("obj:")) {
              const parts = key.split(":");
              const db     = parts[1];
              const schema = parts[2];
              const kind   = parts[3];
              const name   = parts.slice(4).join(":");
              const args   = (node as any).arguments ?? "";
              return (
                <ObjTooltip cacheKey={key} db={db} schema={schema} kind={kind} name={name} args={args}>
                  {String(node.title)}
                </ObjTooltip>
              );
            }
            return node.title as React.ReactNode;
          }}
        />
      )}

      {/* Context menu */}
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
          }}
          onClick={(e) => e.stopPropagation()}
        >
          {ctxMenu.nodeType === "db" && menuItem("Refresh", <ReloadOutlined style={{ fontSize: 12 }} />, refreshDatabase)}
          {ctxMenu.nodeType === "db" && menuItem("Export DDL", <CloudUploadOutlined style={{ fontSize: 12 }} />, exportDatabase)}
          {ctxMenu.nodeType === "db" && menuItem("ER Diagram…", <ApartmentOutlined style={{ fontSize: 12 }} />, generateERDiagram)}
          {ctxMenu.nodeType === "schema" && menuItem("Show Dropped Tables…", <RollbackOutlined style={{ fontSize: 12 }} />, showDroppedTables)}
          {ctxMenu.nodeType === "obj" && (ctxMenu.objKind === "TABLE" || ctxMenu.objKind === "VIEW") &&
            menuItem("Select Top 1000 Rows", <TableOutlined style={{ fontSize: 12 }} />, selectTop1000)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Time Travel Query…", <HistoryOutlined style={{ fontSize: 12 }} />, openTimeTravelModal)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PROCEDURE" &&
            menuItem("Call Procedure", <PlayCircleOutlined style={{ fontSize: 12 }} />, callProcedure)}
          {ctxMenu.nodeType === "obj" && menuItem("View Definition", null, viewDefinition)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind !== "FUNCTION" && ctxMenu.objKind !== "PROCEDURE" &&
            menuItem("Rename…", <EditOutlined style={{ fontSize: 12 }} />, renameObject)}
          {ctxMenu.nodeType === "obj" && <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />}
          {ctxMenu.nodeType === "obj" && menuItem("Delete…", <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />, deleteObject, "#f85149")}
        </div>
      )}

      {/* Definition modal */}
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
            }}
          >
            {ddlModal.src}
          </pre>
        )}
      </Modal>

      <Divider style={{ borderColor: "var(--border)", margin: "8px 0 0" }} />
      <AccountPanel />

      {/* Call Procedure modal */}
      {callModal && (
        <CallProcedureModal
          db={callModal.db}
          schema={callModal.schema}
          name={callModal.name}
          rawArgs={callModal.rawArgs}
          onClose={() => setCallModal(null)}
        />
      )}

      {/* Undrop Tables modal */}
      <Modal
        open={undropModal !== null}
        title={undropModal ? `Dropped tables — ${undropModal.db}.${undropModal.schema}` : ""}
        onCancel={() => setUndropModal(null)}
        footer={null}
        width={560}
      >
        {undropModal?.tables === null && !undropModal?.error && (
          <div style={{ textAlign: "center", padding: "24px 0" }}>
            <Spin />
          </div>
        )}
        {undropModal?.error && (
          <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>
            {undropModal.error}
          </div>
        )}
        {undropModal?.tables !== null && !undropModal?.error && undropModal?.tables?.length === 0 && (
          <div style={{ color: "var(--text-muted)", fontSize: 13, padding: "12px 0" }}>
            No dropped tables found within the Time Travel retention window.
          </div>
        )}
        {undropModal?.tables !== null && !undropModal?.error && (undropModal?.tables?.length ?? 0) > 0 && (
          <div>
            {undropModal!.tables!.map((t) => (
              <div
                key={t.name}
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  padding: "8px 4px",
                  borderBottom: "1px solid var(--border)",
                }}
              >
                <div>
                  <div style={{ fontFamily: "monospace", fontSize: 13, color: "var(--text)" }}>{t.name}</div>
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>Dropped: {t.droppedOn}</div>
                </div>
                <Button
                  size="small"
                  icon={<RollbackOutlined />}
                  onClick={() => undropTable(undropModal!.db, undropModal!.schema, t.name)}
                >
                  Undrop
                </Button>
              </div>
            ))}
          </div>
        )}
      </Modal>
      {/* Time Travel modal */}
      <Modal
        open={timeTravelModal !== null}
        title={
          <span>
            <HistoryOutlined style={{ marginRight: 8, color: "var(--link)" }} />
            Time Travel — {timeTravelModal?.db}.{timeTravelModal?.schema}.{timeTravelModal?.name}
          </span>
        }
        onCancel={() => setTimeTravelModal(null)}
        onOk={executeTimeTravel}
        okText="Query"
        okButtonProps={{ disabled: timeTravelModal?.retentionDays === null }}
        width={620}
      >
        {(!timeTravelModal || timeTravelModal.retentionDays === null) ? (
          <div style={{ textAlign: "center", padding: "40px 0" }}>
            <Spin />
            <div style={{ marginTop: 12, fontSize: 12, color: "var(--text-muted)" }}>Loading retention info…</div>
          </div>
        ) : (
          <div style={{ padding: "20px 8px 8px" }}>
            <div style={{ marginBottom: 20, fontSize: 12, color: "var(--text-muted)" }}>
              Data retention window:{" "}
              <strong style={{ color: "var(--text)" }}>
                {timeTravelModal!.retentionDays} {timeTravelModal!.retentionDays === 1 ? "day" : "days"}
              </strong>
              {" · "}drag the handle to choose a point in time
            </div>

            <Slider
              min={timeTravelModal!.minTs}
              max={timeTravelModal!.maxTs}
              value={timeTravelModal!.selectedTs}
              step={60}
              onChange={(v) => setTimeTravelModal((prev) => prev ? { ...prev, selectedTs: v } : null)}
              tooltip={{ formatter: (v) => v ? new Date(v * 1000).toLocaleString() : "" }}
              marks={{
                [timeTravelModal!.minTs]: (
                  <span style={{ fontSize: 10, color: "var(--text-muted)", whiteSpace: "nowrap" }}>
                    {new Date(timeTravelModal!.minTs * 1000).toLocaleDateString(undefined, { month: "short", day: "numeric" })}
                  </span>
                ),
                [timeTravelModal!.maxTs]: (
                  <span style={{ fontSize: 10, color: "var(--text-muted)" }}>Now</span>
                ),
              }}
            />

            <div
              style={{
                marginTop: 28,
                padding: "14px 16px",
                background: "var(--bg)",
                border: "1px solid var(--border)",
                borderRadius: 6,
                textAlign: "center",
              }}
            >
              <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 4 }}>Selected time</div>
              <div style={{ fontFamily: "monospace", fontSize: 13, color: "var(--text)" }}>
                {new Date(timeTravelModal!.selectedTs * 1000).toLocaleString(undefined, {
                  weekday: "short", year: "numeric", month: "short",
                  day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit",
                })}
              </div>
            </div>

            <div style={{ marginTop: 12, fontSize: 11, color: "var(--text-faint)", fontFamily: "monospace", wordBreak: "break-all" }}>
              AT(TIMESTAMP =&gt; TO_TIMESTAMP_NTZ({timeTravelModal!.selectedTs}))
            </div>
          </div>
        )}
      </Modal>

      {/* Rename modal */}
      <Modal
        open={renameModal !== null}
        title={renameModal ? `Rename ${renameModal.kind.toLowerCase()} "${renameModal.oldName}"` : ""}
        onOk={executeRename}
        onCancel={() => setRenameModal(null)}
        okText="Rename"
        width={420}
      >
        <div style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>New name</div>
          <Input
            value={renameModal?.newName ?? ""}
            onChange={(e) => setRenameModal((prev) => prev ? { ...prev, newName: e.target.value } : null)}
            onPressEnter={executeRename}
            autoFocus
          />
        </div>
      </Modal>

      {/* ER Diagram modal */}
      {erModal && (
        <ERDiagramModal
          database={erModal.database}
          data={erModal.data}
          onClose={() => setErModal(null)}
        />
      )}
    </div>
  );
}
