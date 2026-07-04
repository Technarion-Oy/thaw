// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useRef, useState } from "react";
import { Modal, Button, Spin, Empty, Tree, Tag, Typography, Tooltip } from "antd";
import {
  TableOutlined,
  EyeOutlined,
  FunctionOutlined,
  CodeOutlined,
  QuestionOutlined,
  SyncOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import { GetObjectDependencies, GetObjectDDL } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { kindSupportsDdl } from "../../utils/objectDdl";

const { Text } = Typography;

// ── DDL tooltip ───────────────────────────────────────────────────────────────

const DDL_CACHE_TTL = 60_000; // ms
const ddlCache = new Map<string, { ddl: string; ts: number }>();

function DdlTooltip({
  db, schema, kind, name, args, children,
}: {
  db: string;
  schema: string;
  kind: string;
  name: string;
  args: string;
  children: React.ReactNode;
}) {
  const cacheKey = `${db}\x00${schema}\x00${kind}\x00${name}\x00${args}`;
  // Skip the doomed GET_DDL for kinds Snowflake can't render (SERVICE, MODEL, …),
  // matching every other frontend DDL entry point. Renders children with no tooltip.
  const ddlSupported = kindSupportsDdl(kind);

  const getCached = () => {
    const entry = ddlCache.get(cacheKey);
    return entry && Date.now() - entry.ts < DDL_CACHE_TTL ? entry.ddl : null;
  };

  const [content, setContent] = useState<string | null>(getCached);
  const [loading, setLoading] = useState(false);

  // Trigger the DDL fetch on mouse-enter so it's ready (or loading) by the time
  // the tooltip delay fires.  We no longer rely on onOpenChange because Ant Design
  // disables the tooltip entirely when title={null}, which prevents onOpenChange
  // from ever firing in the initial state.
  const triggerLoad = () => {
    if (loading || !ddlSupported) return;
    const fresh = getCached();
    if (fresh !== null) {
      if (content !== fresh) setContent(fresh);
      return;
    }
    setLoading(true);
    GetObjectDDL(db, schema, kind, name, args)
      .then((src) => {
        const text = src || "(empty)";
        ddlCache.set(cacheKey, { ddl: text, ts: Date.now() });
        setContent(text);
      })
      .catch(() => {
        ddlCache.set(cacheKey, { ddl: "", ts: Date.now() });
        setContent("");
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
      title={loading || content !== null ? overlay : null}
      placement="right"
      mouseEnterDelay={0.6}
      mouseLeaveDelay={0.1}
      getPopupContainer={() => document.body}
      overlayStyle={{ maxWidth: 540 }}
      overlayInnerStyle={{
        background: "var(--bg-overlay)",
        border: "1px solid var(--border)",
        borderRadius: 6,
        padding: "8px 12px",
        boxShadow: "0 4px 16px rgba(0,0,0,0.45)",
      }}
    >
      <span style={{ display: "block", whiteSpace: "nowrap" }} onMouseEnter={triggerLoad}>
        {children}
      </span>
    </Tooltip>
  );
}

// ── types ─────────────────────────────────────────────────────────────────────

export interface DependenciesModalProps {
  open: boolean;
  database: string;
  schema: string;
  kind: string;
  name: string;
  arguments: string;
  onClose: () => void;
}

type DepNode = snowflake.DependencyNode;

interface NodeMeta {
  dep: DepNode;
  args: string; // arg signature — set for the root node, empty for all children
}

// ── constants ─────────────────────────────────────────────────────────────────

const KIND_COLOR: Record<string, string> = {
  TABLE:     "blue",
  VIEW:      "green",
  PROCEDURE: "purple",
  FUNCTION:  "orange",
  UNKNOWN:   "default",
};

const KIND_ICON: Record<string, React.ReactNode> = {
  TABLE:     <TableOutlined style={{ fontSize: 13 }} />,
  VIEW:      <EyeOutlined style={{ fontSize: 13 }} />,
  PROCEDURE: <CodeOutlined style={{ fontSize: 13 }} />,
  FUNCTION:  <FunctionOutlined style={{ fontSize: 13 }} />,
  UNKNOWN:   <QuestionOutlined style={{ fontSize: 13 }} />,
};

// ── tree builder ──────────────────────────────────────────────────────────────

let nodeCounter = 0;

/**
 * Builds DataNode[] and simultaneously populates `meta` with the dependency
 * metadata for each node.  The meta map is keyed by node key and is the
 * single source of truth for titleRender — we do NOT rely on Ant Design Tree
 * preserving custom DataNode properties (it strips them internally).
 */
function toTreeNodes(
  nodes: DepNode[],
  parentKey: string,
  meta: Map<string, NodeMeta>,
): DataNode[] {
  return (nodes ?? []).map((n) => {
    const key = `${parentKey}-${++nodeCounter}`;
    meta.set(key, { dep: n, args: "" });
    const isLeaf = n.circular || !n.children || n.children.length === 0;
    return {
      key,
      // title must be a non-empty string so rc-tree doesn't skip the node;
      // titleRender replaces what is visually shown.
      title: [n.database, n.schema, n.name].filter(Boolean).join("."),
      isLeaf,
      children: isLeaf ? undefined : toTreeNodes(n.children ?? [], key, meta),
    } as DataNode;
  });
}

function allKeys(nodes: DataNode[]): string[] {
  const keys: string[] = [];
  const visit = (ns: DataNode[]) => {
    for (const n of ns) {
      keys.push(String(n.key));
      if (n.children) visit(n.children);
    }
  };
  visit(nodes);
  return keys;
}

// ── node label ────────────────────────────────────────────────────────────────

function NodeLabel({ dep, bold }: { dep: DepNode; bold?: boolean }) {
  const kindUpper = (dep.kind ?? "UNKNOWN").toUpperCase();
  const qualifiedName = [dep.database, dep.schema, dep.name].filter(Boolean).join(".");
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
      <span style={{ opacity: 0.65 }}>{KIND_ICON[kindUpper] ?? KIND_ICON.UNKNOWN}</span>
      <Text strong={bold} style={{ fontSize: 13 }}>{qualifiedName}</Text>
      <Tag
        color={KIND_COLOR[kindUpper] ?? "default"}
        style={{ fontSize: 11, lineHeight: "18px", padding: "0 5px", marginLeft: 2 }}
      >
        {kindUpper}
      </Tag>
      {dep.circular && (
        <Tag
          icon={<SyncOutlined />}
          color="warning"
          style={{ fontSize: 11, lineHeight: "18px", padding: "0 5px" }}
        >
          already shown
        </Tag>
      )}
      {dep.error && (
        <Tag
          icon={<WarningOutlined />}
          color="error"
          style={{ fontSize: 11, lineHeight: "18px", padding: "0 5px" }}
        >
          {dep.error}
        </Tag>
      )}
    </span>
  );
}

// ── component ─────────────────────────────────────────────────────────────────

export default function DependenciesModal({
  open,
  database,
  schema,
  kind,
  name,
  arguments: args,
  onClose,
}: DependenciesModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [expandedKeys, setExpandedKeys] = useState<string[]>([]);

  // Metadata keyed by node key.  Populated in useEffect, read in titleRender.
  // Using a ref (not state) so titleRender always sees the latest values
  // without causing extra re-renders.
  const nodeMetaRef = useRef<Map<string, NodeMeta>>(new Map());

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    setTreeData([]);
    setExpandedKeys([]);
    nodeCounter = 0;

    GetObjectDependencies(database, schema, kind, name, args)
      .then((root) => {
        nodeCounter = 0;
        const meta = new Map<string, NodeMeta>();

        meta.set("root", { dep: root, args });
        const children = toTreeNodes(root.children ?? [], "root", meta);

        nodeMetaRef.current = meta;

        const rootNode: DataNode = {
          key: "root",
          title: [root.database, root.schema, root.name].filter(Boolean).join("."),
          isLeaf: children.length === 0,
          children: children.length > 0 ? children : undefined,
        };
        const data = [rootNode];
        setTreeData(data);
        setExpandedKeys(allKeys(data));
      })
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const hasChildren = treeData.length > 0 && !!treeData[0].children;

  /**
   * titleRender is called by Ant Design Tree during its own render cycle for
   * every visible node.  We look up metadata from nodeMetaRef rather than
   * from the nodeData object itself, because Ant Design Tree strips unknown
   * DataNode properties internally.
   */
  const titleRender = (nodeData: DataNode) => {
    const meta = nodeMetaRef.current.get(String(nodeData.key));
    if (!meta) return nodeData.title as React.ReactNode;
    const { dep, args: nodeArgs } = meta;
    const kindUpper = (dep.kind ?? "UNKNOWN").toUpperCase();
    return (
      <DdlTooltip
        db={dep.database}
        schema={dep.schema}
        kind={kindUpper}
        name={dep.name}
        args={nodeArgs}
      >
        <NodeLabel dep={dep} bold={nodeData.key === "root"} />
      </DdlTooltip>
    );
  };

  return (
    <Modal
      open={open}
      title={`Dependencies — ${name}`}
      onCancel={onClose}
      width={680}
      footer={[
        <Button key="close" onClick={onClose}>
          Close
        </Button>,
      ]}
      styles={{ body: { padding: "12px 16px", minHeight: 120 } }}
    >
      {loading && (
        <div style={{ display: "flex", justifyContent: "center", padding: 40 }}>
          <Spin tip="Resolving dependencies…" />
        </div>
      )}

      {!loading && error && (
        <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>
          {error}
        </div>
      )}

      {!loading && !error && treeData.length === 0 && (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="No dependencies found"
          style={{ padding: "24px 0" }}
        />
      )}

      {!loading && !error && treeData.length > 0 && (
        <>
          {!hasChildren && (
            <div style={{ marginBottom: 8, color: "var(--text-muted)", fontSize: 12 }}>
              This object has no resolvable dependencies in its DDL.
            </div>
          )}
          <div
            style={{
              maxHeight: "55vh",
              overflowY: "auto",
              background: "var(--bg)",
              border: "1px solid var(--border)",
              borderRadius: 6,
              padding: "8px 4px",
            }}
          >
            <Tree
              treeData={treeData}
              titleRender={titleRender}
              expandedKeys={expandedKeys}
              onExpand={(keys) => setExpandedKeys(keys as string[])}
              showLine={{ showLeafIcon: false }}
              style={{ background: "transparent", fontSize: 13 }}
            />
          </div>
          <div style={{ marginTop: 8, fontSize: 11, color: "var(--text-muted)" }}>
            Only SQL-language objects are expanded. Tables and non-SQL procedures are shown as leaf nodes.
          </div>
        </>
      )}
    </Modal>
  );
}
