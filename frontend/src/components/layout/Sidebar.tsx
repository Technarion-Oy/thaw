import { useState, useEffect, useRef } from "react";
import { Tree, Typography, Spin, Empty, Divider, Modal } from "antd";
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
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import { ListDatabases, ListSchemas, ListObjects, GetObjectDDL } from "../../../wailsjs/go/main/App";
import { useQueryStore } from "../../store/queryStore";
import { useObjectStore } from "../../store/objectStore";
import GitPanel from "../git/GitPanel";

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
  nodeType: "db" | "obj";
  objKind?: string; // set for nodeType === "obj"
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

export default function Sidebar() {
  const [treeData, setTreeData]     = useState<DataNode[]>([]);
  const [loadedKeys, setLoadedKeys] = useState<Key[]>([]);
  const [loading, setLoading]       = useState(false);
  const [loaded, setLoaded]         = useState(false);

  const [ctxMenu, setCtxMenu]   = useState<ContextMenu | null>(null);
  const [ddlModal, setDdlModal] = useState<ObjectDDL | null>(null);
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
        icon:     <FolderOutlined style={{ color: "#8b949e" }} />,
        children: groups[kind].map((o) => ({
          title:  o.name,
          key:    `obj:${db}:${schema}:${kind}:${o.name}`,
          icon:   kindIcon(kind),
          isLeaf: true,
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
    } else if (key.startsWith("obj:")) {
      // key format: obj:DB:SCHEMA:KIND:NAME
      const objKind = key.split(":")[3];
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "obj", objKind });
    }
  };

  const refreshDatabase = () => {
    if (!ctxMenu) return;
    const dbKey = ctxMenu.nodeKey;        // "db:DBNAME"
    const db    = dbKey.slice("db:".length); // "DBNAME"
    setCtxMenu(null);
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

  const selectTop1000 = () => {
    if (!ctxMenu) return;
    setCtxMenu(null);

    // key format: obj:DB:SCHEMA:KIND:NAME
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    const sql = `SELECT * FROM "${db}"."${schema}"."${name}" LIMIT 1000;`;

    useQueryStore.getState().executeWith(sql);
  };

  const viewDefinition = async () => {
    if (!ctxMenu) return;
    setCtxMenu(null);

    // key format: obj:db:schema:kind:name
    const [, db, schema, kind, ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");

    setDdlModal({ title: `${kind}: ${db}.${schema}.${name}`, src: "", loading: true, error: null });
    try {
      const src = await GetObjectDDL(db, schema, kind, name);
      setDdlModal((prev) => (prev ? { ...prev, src, loading: false } : null));
    } catch (e) {
      setDdlModal((prev) => (prev ? { ...prev, error: String(e), loading: false } : null));
    }
  };

  // ── Render ──────────────────────────────────────────────────────────────────

  const menuItem = (label: string, icon: React.ReactNode, onClick: () => void) => (
    <div
      style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "#e6edf3" }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "#30363d")}
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
          style={{ background: "transparent", color: "#e6edf3" }}
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
            background: "#1c2128",
            border: "1px solid #30363d",
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
            minWidth: 160,
          }}
          onClick={(e) => e.stopPropagation()}
        >
          {ctxMenu.nodeType === "db" && menuItem("Refresh", <ReloadOutlined style={{ fontSize: 12 }} />, refreshDatabase)}
          {ctxMenu.nodeType === "obj" && (ctxMenu.objKind === "TABLE" || ctxMenu.objKind === "VIEW") &&
            menuItem("Select Top 1000 Rows", <TableOutlined style={{ fontSize: 12 }} />, selectTop1000)}
          {ctxMenu.nodeType === "obj" && menuItem("View Definition", null, viewDefinition)}
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
              background: "#0d1117",
              color: "#e6edf3",
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

      <Divider style={{ borderColor: "#30363d", margin: "8px 0 0" }} />
      <GitPanel />
    </div>
  );
}
