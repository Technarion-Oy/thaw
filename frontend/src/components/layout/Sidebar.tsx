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
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import { ListDatabases, ListSchemas, ListObjects, GetObjectDDL } from "../../../wailsjs/go/main/App";
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
  nodeKey: string; // "obj:db:schema:kind:name"
}

interface ObjectDDL {
  title: string;
  src: string;
  loading: boolean;
  error: string | null;
}

export default function Sidebar() {
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading]   = useState(false);
  const [loaded, setLoaded]     = useState(false);

  const [ctxMenu, setCtxMenu]   = useState<ContextMenu | null>(null);
  const [ddlModal, setDdlModal] = useState<ObjectDDL | null>(null);
  const ctxRef = useRef<HTMLDivElement>(null);

  // Close context menu when clicking outside
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
      setLoaded(true);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const onLoadData = async ({ key, children }: DataNode & { children?: DataNode[] }) => {
    if (children) return;
    const parts = String(key).split(":");

    if (parts[0] === "db") {
      const db = parts[1];
      const schemas = await ListSchemas(db);
      setTreeData((prev) =>
        updateNode(prev, String(key), schemas.map((s) => ({
          title: s,
          key: `schema:${db}:${s}`,
          icon: <FolderOutlined />,
          isLeaf: false,
        })))
      );
    } else if (parts[0] === "schema") {
      const [, db, schema] = parts;
      const objects = await ListObjects(db, schema);

      // Group by kind
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
        title: KIND_LABEL[kind] ?? kind,
        key: `type:${db}:${schema}:${kind}`,
        icon: <FolderOutlined style={{ color: "#8b949e" }} />,
        children: groups[kind].map((o) => ({
          title: o.name,
          key: `obj:${db}:${schema}:${kind}:${o.name}`,
          icon: kindIcon(kind),
          isLeaf: true,
        })),
      }));

      setTreeData((prev) => updateNode(prev, String(key), typeNodes));
    }
  };

  function updateNode(nodes: DataNode[], targetKey: string, children: DataNode[]): DataNode[] {
    return nodes.map((node) => {
      if (node.key === targetKey) return { ...node, children };
      if (node.children) return { ...node, children: updateNode(node.children, targetKey, children) };
      return node;
    });
  }

  const onRightClick = ({ event, node }: { event: React.MouseEvent; node: DataNode }) => {
    event.preventDefault();
    const key = String(node.key);
    if (!key.startsWith("obj:")) return; // only leaf objects have a definition
    setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key });
  };

  const viewDefinition = async () => {
    if (!ctxMenu) return;
    setCtxMenu(null);

    // key format: obj:db:schema:kind:name
    const [, db, schema, kind, ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":"); // name might theoretically contain colons

    setDdlModal({ title: `${kind}: ${db}.${schema}.${name}`, src: "", loading: true, error: null });
    try {
      const src = await GetObjectDDL(db, schema, kind, name);
      setDdlModal((prev) => prev ? { ...prev, src, loading: false } : null);
    } catch (e) {
      setDdlModal((prev) => prev ? { ...prev, error: String(e), loading: false } : null);
    }
  };

  return (
    <div style={{ padding: "8px 4px" }}>
      <Text type="secondary" style={{ fontSize: 11, padding: "0 12px", display: "block", marginBottom: 8, textTransform: "uppercase", letterSpacing: "0.08em" }}>
        Objects
      </Text>

      {loading && <Spin size="small" style={{ display: "block", margin: "16px auto" }} />}

      {!loaded && !loading && (
        <div style={{ padding: "16px 12px" }}>
          <Text
            type="secondary"
            style={{ cursor: "pointer", fontSize: 12 }}
            onClick={loadDatabases}
          >
            Click to load databases
          </Text>
        </div>
      )}

      {loaded && treeData.length === 0 && <Empty description="No databases" imageStyle={{ height: 40 }} />}

      {treeData.length > 0 && (
        <Tree
          treeData={treeData}
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
          <div
            style={{ padding: "6px 14px", fontSize: 13, cursor: "pointer", color: "#e6edf3" }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "#30363d")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
            onClick={viewDefinition}
          >
            View Definition
          </div>
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
