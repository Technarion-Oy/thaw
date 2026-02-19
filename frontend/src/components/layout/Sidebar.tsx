import { useState } from "react";
import { Tree, Typography, Spin, Empty, Divider } from "antd";
import { DatabaseOutlined, TableOutlined, EyeOutlined } from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import { ListDatabases, ListSchemas, ListObjects } from "../../../wailsjs/go/main/App";
import GitPanel from "../git/GitPanel";

const { Text } = Typography;

export default function Sidebar() {
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);

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
          isLeaf: false,
        })))
      );
    } else if (parts[0] === "schema") {
      const [, db, schema] = parts;
      const objects = await ListObjects(db, schema);
      setTreeData((prev) =>
        updateNode(prev, String(key), objects.map((o) => ({
          title: o.name,
          key: `obj:${db}:${schema}:${o.name}`,
          icon: o.kind === "VIEW" ? <EyeOutlined /> : <TableOutlined />,
          isLeaf: true,
        })))
      );
    }
  };

  // Recursively update a node's children in the tree.
  function updateNode(nodes: DataNode[], targetKey: string, children: DataNode[]): DataNode[] {
    return nodes.map((node) => {
      if (node.key === targetKey) return { ...node, children };
      if (node.children) return { ...node, children: updateNode(node.children, targetKey, children) };
      return node;
    });
  }

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
          showIcon
          blockNode
          style={{ background: "transparent", color: "#e6edf3" }}
        />
      )}

      <Divider style={{ borderColor: "#30363d", margin: "8px 0 0" }} />
      <GitPanel />
    </div>
  );
}
