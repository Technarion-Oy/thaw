// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useRef, useState, useCallback } from "react";
import {
  Modal, Button, Spin, Empty, Tree, Tag, Typography, Tooltip, Tabs, Alert,
} from "antd";
import {
  TableOutlined,
  EyeOutlined,
  FunctionOutlined,
  CodeOutlined,
  QuestionOutlined,
  SyncOutlined,
  WarningOutlined,
  ReloadOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
} from "@ant-design/icons";
import type { DataNode } from "antd/es/tree";
import {
  GetObjectDependencies,
  GetObjectDDL,
  GetObjectUsageDependencies,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { kindSupportsDdl } from "../../utils/objectDdl";

const { Text } = Typography;

// Object kinds whose DDL the parser engine can read (GetObjectDependencies).
// Everything else relies solely on the ACCOUNT_USAGE section.
const PARSER_KINDS = new Set(["VIEW", "PROCEDURE", "FUNCTION", "EXTERNAL FUNCTION"]);

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
type UsageRef = snowflake.ObjectDependencyRef;

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

// bucketFor maps a Snowflake domain (which may be multi-word, e.g.
// "MATERIALIZED VIEW", "EXTERNAL FUNCTION") to the closest kind icon/color by
// its last significant word, falling back to UNKNOWN.
function bucketFor(domain: string): string {
  const up = (domain || "").toUpperCase();
  if (up.includes("TABLE")) return "TABLE";
  if (up.includes("VIEW")) return "VIEW";
  if (up.includes("PROCEDURE")) return "PROCEDURE";
  if (up.includes("FUNCTION")) return "FUNCTION";
  return "UNKNOWN";
}

// isRoutineDomain reports whether the domain is a procedure/function kind whose
// GET_DDL requires an argument signature. OBJECT_DEPENDENCIES does not report
// argument types, so a GET_DDL for these would append "()" and fail to resolve
// any parameterized overload — we therefore suppress the DDL hover for them in
// the ACCOUNT_USAGE lists rather than show a silently-empty tooltip.
function isRoutineDomain(domain: string): boolean {
  const b = bucketFor(domain);
  return b === "PROCEDURE" || b === "FUNCTION";
}

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
  const bucket = bucketFor(kindUpper);
  const qualifiedName = [dep.database, dep.schema, dep.name].filter(Boolean).join(".");
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
      <span style={{ opacity: 0.65 }}>{KIND_ICON[bucket] ?? KIND_ICON.UNKNOWN}</span>
      <Text strong={bold} style={{ fontSize: 13 }}>{qualifiedName}</Text>
      <Tag
        color={KIND_COLOR[bucket] ?? "default"}
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

// ── ACCOUNT_USAGE flat list ─────────────────────────────────────────────────────

const USAGE_CAPTION: Record<"depends_on" | "referenced_by", string> = {
  depends_on:
    "Objects this one references, from SNOWFLAKE.ACCOUNT_USAGE.OBJECT_DEPENDENCIES. " +
    "Covers tables and non-SQL bodies the DDL parser cannot read. Requires governance " +
    "privileges (a grant on the SNOWFLAKE database / ACCOUNTADMIN) and may lag recent changes.",
  referenced_by:
    "Objects that reference this one, from SNOWFLAKE.ACCOUNT_USAGE.OBJECT_DEPENDENCIES. " +
    "Requires governance privileges (a grant on the SNOWFLAKE database / ACCOUNTADMIN) " +
    "and may lag recent changes.",
};

/**
 * UsageSection loads and renders a flat, de-duplicated list of object
 * dependency edges from ACCOUNT_USAGE.OBJECT_DEPENDENCIES in one direction.
 * It owns its own load lifecycle so switching tabs / refreshing is independent
 * of the parser tree. When autoLoad is true it fetches on first mount.
 */
function UsageSection({
  database, schema, name, direction, autoLoad,
}: {
  database: string;
  schema: string;
  name: string;
  direction: "depends_on" | "referenced_by";
  autoLoad: boolean;
}) {
  const [refs, setRefs] = useState<UsageRef[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    GetObjectUsageDependencies(database, schema, name, direction)
      .then((r) => setRefs(r ?? []))
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, [database, schema, name, direction]);

  useEffect(() => {
    if (autoLoad && refs === null && !loading) load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoLoad]);

  return (
    <div>
      <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
        {USAGE_CAPTION[direction]}
      </Text>

      {error && (
        <Alert
          type="warning"
          message="Could not load from ACCOUNT_USAGE"
          description={error}
          showIcon
          style={{ marginBottom: 8 }}
        />
      )}

      <Button
        size="small"
        icon={<ReloadOutlined />}
        onClick={load}
        loading={loading}
        style={{ marginBottom: 10 }}
      >
        {refs === null ? "Load from ACCOUNT_USAGE" : "Refresh"}
      </Button>

      {loading && refs === null && (
        <div style={{ display: "flex", justifyContent: "center", padding: 24 }}>
          <Spin />
        </div>
      )}

      {refs !== null && !loading && (
        refs.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={direction === "depends_on" ? "No dependencies recorded" : "No referencing objects recorded"}
            style={{ padding: "16px 0" }}
          />
        ) : (
          <div
            style={{
              maxHeight: "48vh",
              overflowY: "auto",
              background: "var(--bg)",
              border: "1px solid var(--border)",
              borderRadius: 6,
              padding: "6px 8px",
            }}
          >
            {refs.map((r, i) => {
              const label = (
                <NodeLabel
                  dep={{ database: r.database, schema: r.schema, name: r.name, kind: r.domain } as DepNode}
                />
              );
              return (
                <div key={`${r.database}.${r.schema}.${r.name}-${i}`} style={{ padding: "4px 2px" }}>
                  {/* OBJECT_DEPENDENCIES omits argument signatures, so a GET_DDL
                      for a procedure/function would fail — show the plain label
                      (no DDL hover) for routine kinds instead of a blank tooltip. */}
                  {isRoutineDomain(r.domain) ? (
                    label
                  ) : (
                    <DdlTooltip db={r.database} schema={r.schema} kind={r.domain} name={r.name} args="">
                      {label}
                    </DdlTooltip>
                  )}
                </div>
              );
            })}
          </div>
        )
      )}

      {refs !== null && !loading && refs.some((r) => isRoutineDomain(r.domain)) && (
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 8 }}>
          Hover-DDL preview is unavailable for procedures and functions here —
          their argument signature isn't recorded by OBJECT_DEPENDENCIES.
        </Text>
      )}
    </div>
  );
}

// ── parser tree section ─────────────────────────────────────────────────────────

function ParserTreeSection({
  database, schema, kind, name, args,
}: {
  database: string;
  schema: string;
  kind: string;
  name: string;
  args: string;
}) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [expandedKeys, setExpandedKeys] = useState<string[]>([]);

  // Metadata keyed by node key. Populated on load, read in titleRender.
  const nodeMetaRef = useRef<Map<string, NodeMeta>>(new Map());

  useEffect(() => {
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
  }, [database, schema, kind, name, args]);

  const hasChildren = treeData.length > 0 && !!treeData[0].children;

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
    <div>
      <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
        Parsed live from the object's DDL — instant and needs no extra privileges.
        Only SQL-language objects expand recursively; tables and non-SQL bodies are leaf nodes.
      </Text>

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

      {!loading && !error && treeData.length > 0 && (
        <>
          {!hasChildren && (
            <div style={{ marginBottom: 8, color: "var(--text-muted)", fontSize: 12 }}>
              This object has no resolvable dependencies in its DDL. Try the
              ACCOUNT_USAGE section below for objects the parser cannot read.
            </div>
          )}
          {hasChildren && (
            <div
              style={{
                maxHeight: "45vh",
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
          )}
        </>
      )}
    </div>
  );
}

// ── section divider ─────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "18px 0 8px",
};

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
  const [activeTab, setActiveTab] = useState<"depends_on" | "referenced_by">("depends_on");

  // Reset to the first tab each time the modal is (re)opened for a new object.
  useEffect(() => {
    if (open) setActiveTab("depends_on");
  }, [open, database, schema, kind, name]);

  const parserSupported = PARSER_KINDS.has((kind ?? "").toUpperCase());

  const dependsOnTab = (
    <div>
      {parserSupported && (
        <>
          <div style={{ ...SECTION_HEAD, marginTop: 4 }}>Parsed from DDL</div>
          {/* key forces a fresh mount (and reload) per object identity */}
          <ParserTreeSection
            key={`${database}.${schema}.${kind}.${name}.${args}`}
            database={database}
            schema={schema}
            kind={kind}
            name={name}
            args={args}
          />
        </>
      )}
      <div style={SECTION_HEAD}>From ACCOUNT_USAGE</div>
      <UsageSection
        key={`dep-${database}.${schema}.${name}`}
        database={database}
        schema={schema}
        name={name}
        direction="depends_on"
        autoLoad={!parserSupported}
      />
    </div>
  );

  const referencedByTab = (
    <UsageSection
      key={`ref-${database}.${schema}.${name}`}
      database={database}
      schema={schema}
      name={name}
      direction="referenced_by"
      autoLoad={activeTab === "referenced_by"}
    />
  );

  return (
    <Modal
      open={open}
      title={`Dependencies & References — ${name}`}
      onCancel={onClose}
      width={720}
      footer={[
        <Button key="close" onClick={onClose}>
          Close
        </Button>,
      ]}
      styles={{ body: { padding: "8px 16px 12px", minHeight: 200 } }}
    >
      <Tabs
        activeKey={activeTab}
        onChange={(k) => setActiveTab(k as "depends_on" | "referenced_by")}
        items={[
          {
            key: "depends_on",
            label: (
              <span>
                <ArrowDownOutlined style={{ marginRight: 6 }} />
                Depends on
              </span>
            ),
            children: dependsOnTab,
          },
          {
            key: "referenced_by",
            label: (
              <span>
                <ArrowUpOutlined style={{ marginRight: 6 }} />
                Referenced by
              </span>
            ),
            children: referencedByTab,
          },
        ]}
      />
    </Modal>
  );
}
