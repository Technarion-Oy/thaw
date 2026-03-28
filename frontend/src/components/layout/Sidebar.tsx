// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useLayoutEffect, useMemo, useRef } from "react";
import { Tree, Typography, Spin, Empty, Divider, Modal, Button, Input, Tooltip, Slider, Tag, message, type InputRef } from "antd";
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
  DownloadOutlined,
  UploadOutlined,
  SearchOutlined,
  CaretRightFilled,
  CaretDownFilled,
  CopyOutlined,
  DiffOutlined,
  SaveOutlined,
  PlusSquareOutlined,
  RightOutlined,
  ShareAltOutlined,
  ExperimentOutlined,
  DashboardOutlined,
} from "@ant-design/icons";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { DataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import { ListDatabases, ListSchemas, ListObjects, GetObjectDDL, GetObjectProperties, ExportDatabaseDDL, ListDroppedTables, ListDroppedSchemas, ListDroppedDatabases, GetTableRetentionDays, GetERDiagramData, FetchNotebookContent } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import type { snowflake } from "../../../wailsjs/go/models";
import { useQueryStore } from "../../store/queryStore";
import { insertAtCursor } from "../editor/editorRef";
import { useObjectStore } from "../../store/objectStore";
import { useGitStore } from "../../store/gitStore";
import { useDiffStore } from "../../store/diffStore";
import AccountPanel from "../account/AccountPanel";
import CallProcedureModal from "../procedure/CallProcedureModal";
import ExecuteNotebookModal from "../notebook/ExecuteNotebookModal";
import SelectFunctionModal from "../function/SelectFunctionModal";
import CreateTaskModal from "../task/CreateTaskModal";
import ExecuteTaskModal from "../task/ExecuteTaskModal";
import TaskGraphModal from "../task/TaskGraphModal";
import TaskPropertiesModal from "../task/TaskPropertiesModal";
import TaskStatusesModal from "../task/TaskStatusesModal";
import ERDiagramModal from "../er/ERDiagramModal";
import ExportTableModal from "../export/ExportTableModal";
import ImportTableModal from "../export/ImportTableModal";
import PropertiesModal from "../common/PropertiesModal";
import BackupSetsModal from "../backup/BackupSetsModal";
import DependenciesModal from "../lineage/DependenciesModal";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

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
  NOTEBOOK:      "Notebooks",
};

const KIND_ORDER = ["TABLE", "VIEW", "FUNCTION", "PROCEDURE", "SEQUENCE", "STAGE", "STREAM", "TASK", "FILE FORMAT", "PIPE", "NOTEBOOK"];

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
    case "NOTEBOOK":    return <ExperimentOutlined />;
    default:            return <FileOutlined />;
  }
}

interface ContextMenu {
  x: number;
  y: number;
  nodeKey: string;
  nodeType: "db" | "schema" | "type" | "obj";
  objKind?: string;  // set for nodeType === "type" or "obj"
  objArgs?: string;  // parameter type list for PROCEDURE / FUNCTION
}

interface UndropModal {
  db: string;
  schema: string;
  tables: snowflake.DroppedTable[] | null; // null = loading
  error: string | null;
}

interface UndropSchemasModal {
  db: string;
  schemas: snowflake.DroppedTable[] | null; // null = loading
  error: string | null;
}

interface UndropDatabasesModal {
  databases: snowflake.DroppedTable[] | null; // null = loading
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

// Cache DDL per unique key; entries expire after DDL_CACHE_TTL so changes
// are visible without a full app restart.
const DDL_CACHE_TTL = 60_000; // ms
const ddlCache = new Map<string, { ddl: string; ts: number }>();

// Keep only obj: nodes whose title matches the query; prune empty parents.
// Parent task nodes (obj: keys with children) are included if any descendant
// matches OR if the node's own title matches.
function filterTree(nodes: DataNode[], query: string): DataNode[] {
  const lower = query.toLowerCase();
  return nodes.reduce<DataNode[]>((acc, node) => {
    const key      = String(node.key);
    const children = (node as any).children as DataNode[] | undefined;
    if (children !== undefined) {
      const filtered = filterTree(children, query);
      const selfMatch = key.startsWith("obj:") && String(node.title).toLowerCase().includes(lower);
      if (filtered.length > 0 || selfMatch) acc.push({ ...node, children: filtered });
    } else if (key.startsWith("obj:")) {
      if (String(node.title).toLowerCase().includes(lower)) acc.push(node);
    }
    return acc;
  }, []);
}

// Collect keys of all non-leaf nodes (used to auto-expand filtered results).
function getAllParentKeys(nodes: DataNode[]): Key[] {
  const keys: Key[] = [];
  for (const node of nodes) {
    const children = (node as any).children as DataNode[] | undefined;
    if (children !== undefined) {
      keys.push(node.key as Key);
      keys.push(...getAllParentKeys(children));
    }
  }
  return keys;
}

// Build a hierarchical DataNode tree for TASK objects using predecessor relationships.
// A task is nested under the first predecessor that also exists in this schema.
// Finalizer tasks (those with a FINALIZE clause) are placed as the last child
// of their root task with an isFinalizer marker for titleRender.
// Tasks with no local predecessor and no finalize relationship are placed at root.
function buildTaskTree(
  tasks: snowflake.SnowflakeObject[],
  db: string,
  schema: string,
): DataNode[] {
  const makeNode = (o: snowflake.SnowflakeObject, kids: DataNode[] = [], isFinalizer = false): DataNode => ({
    title:       o.name,
    key:         `obj:${db}:${schema}:TASK:${o.name}`,
    icon:        kindIcon("TASK"),
    isLeaf:      kids.length === 0,
    isFinalizer, // consumed by titleRender
    ...(kids.length > 0 ? { children: kids } : {}),
  } as DataNode);

  const byName = new Map<string, snowflake.SnowflakeObject>();
  for (const t of tasks) byName.set(t.name.toUpperCase(), t);

  // Build map: rootTaskName.toUpperCase() → finalizer task object
  const finalizerOf = new Map<string, snowflake.SnowflakeObject>();
  const finalizerNames = new Set<string>();
  for (const t of tasks) {
    if (t.finalize) {
      const rootName = extractName(t.finalize).toUpperCase();
      finalizerOf.set(rootName, t);
      finalizerNames.add(t.name.toUpperCase());
    }
  }

  const parentOf = new Map<string, string>();
  const childrenOf = new Map<string, string[]>();

  for (const t of tasks) {
    // Finalizer tasks have no AFTER predecessors — skip predecessor parsing for them.
    if (finalizerNames.has(t.name.toUpperCase())) continue;
    const preds = parsePredecessors(t.predecessors ?? "");
    const localParent = preds
      .map((p) => extractName(p).toUpperCase())
      .find((n) => byName.has(n));
    if (localParent) {
      parentOf.set(t.name.toUpperCase(), localParent);
      if (!childrenOf.has(localParent)) childrenOf.set(localParent, []);
      childrenOf.get(localParent)!.push(t.name);
    }
  }

  const inTree = new Set<string>();

  function buildSubTree(name: string): DataNode {
    inTree.add(name.toUpperCase());
    const task = byName.get(name.toUpperCase())!;
    const kids = (childrenOf.get(name.toUpperCase()) ?? []).map(buildSubTree);
    // Attach finalizer task as the last child if this is its designated root task.
    const finTask = finalizerOf.get(name.toUpperCase());
    if (finTask) {
      inTree.add(finTask.name.toUpperCase());
      kids.push(makeNode(finTask, [], true));
    }
    return makeNode(task, kids);
  }

  const result: DataNode[] = [];
  for (const t of tasks) {
    // Skip finalizer tasks (placed inside their root) and tasks with a parent.
    if (finalizerNames.has(t.name.toUpperCase())) continue;
    if (!parentOf.has(t.name.toUpperCase())) result.push(buildSubTree(t.name));
  }
  // Safety net: orphaned tasks not yet placed (shouldn't normally occur).
  for (const t of tasks) {
    if (!inTree.has(t.name.toUpperCase())) result.push(makeNode(t));
  }
  return result;
}

function ObjTooltip({ cacheKey, db, schema, kind, name, args, children }: {
  cacheKey: string;
  db: string;
  schema: string;
  kind: string;
  name: string;
  args: string;
  children: React.ReactNode;
}) {
  const getCached = () => {
    const entry = ddlCache.get(cacheKey);
    return entry && Date.now() - entry.ts < DDL_CACHE_TTL ? entry.ddl : null;
  };
  const [content, setContent] = useState<string | null>(getCached);
  const [loading, setLoading] = useState(false);

  const onOpenChange = (open: boolean) => {
    if (!open || loading) return;
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
        // Silently suppress DDL errors (e.g. shared databases like SNOWFLAKE
        // that don't support GET_DDL). Cache an empty string so we don't retry.
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
      title={loading || content ? overlay : null}
      placement="right"
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
      <span style={{ display: "block", whiteSpace: "nowrap" }}>
        {children}
      </span>
    </Tooltip>
  );
}

export default function Sidebar({ hideAccountPanel = false }: { hideAccountPanel?: boolean }) {
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading]   = useState(false);
  const [loaded, setLoaded]         = useState(false);

  const [ctxMenu, setCtxMenu]     = useState<ContextMenu | null>(null);
  const [activeSubmenu, setActiveSubmenu] = useState<string | null>(null);
  const [submenuDir, setSubmenuDir] = useState<"left" | "right">("right");
  const submenuTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [ddlModal, setDdlModal]   = useState<ObjectDDL | null>(null);
  const [callModal, setCallModal] = useState<{ db: string; schema: string; name: string; rawArgs: string } | null>(null);
  const [selectFunctionModal, setSelectFunctionModal] = useState<{ db: string; schema: string; name: string; rawArgs: string } | null>(null);
  const [executeNotebookModal, setExecuteNotebookModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createTaskModal, setCreateTaskModal] = useState<{ db: string; schema: string } | null>(null);
  const [executeTaskModal, setExecuteTaskModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [taskPropsModal, setTaskPropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [taskGraphModal, setTaskGraphModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [taskStatusesModal, setTaskStatusesModal] = useState<{ db: string; schema: string } | null>(null);
  const [undropModal, setUndropModal] = useState<UndropModal | null>(null);
  const [undropSchemasModal, setUndropSchemasModal] = useState<UndropSchemasModal | null>(null);
  const [undropDatabasesModal, setUndropDatabasesModal] = useState<UndropDatabasesModal | null>(null);
  const [renameModal, setRenameModal] = useState<RenameModal | null>(null);
  const [timeTravelModal, setTimeTravelModal] = useState<TimeTravelModal | null>(null);
  const [erModal, setErModal] = useState<{ database: string; data: snowflake.ERDiagramData } | null>(null);
  const [propsModal, setPropsModal] = useState<{ title: string; rows: main.PropertyPair[] | null; error: string | null; tableContext?: { db: string; schema: string; table: string } } | null>(null);
  const [exportModal, setExportModal] = useState<{ db: string; schema: string; table: string } | null>(null);
  const [importModal, setImportModal] = useState<{ db: string; schema: string; table: string } | null>(null);
  const [backupSetsModal, setBackupSetsModal] = useState<{ scopeType: "DATABASE" | "SCHEMA" | "TABLE"; db: string; schema: string; table: string } | null>(null);
  const [depsModal, setDepsModal] = useState<{ db: string; schema: string; kind: string; name: string; args: string } | null>(null);
  const [searchQuery, setSearchQuery]               = useState("");
  const searchInputRef = useRef<InputRef>(null);

  // ⌘⇧F / Ctrl+Shift+F — focus the object browser search input.
  useEffect(() => {
    const handler = () => searchInputRef.current?.focus();
    window.addEventListener("thaw:focus-object-search", handler);
    return () => window.removeEventListener("thaw:focus-object-search", handler);
  }, []);
  // Two separate expansion states so the cascade never touches the user's own
  // tree navigation state. On clear we just wipe searchExpandedKeys.
  const [expandedKeys, setExpandedKeys]             = useState<Key[]>([]);
  const [searchExpandedKeys, setSearchExpandedKeys] = useState<Key[]>([]);
  // searchResults holds a full copy of the tree built exclusively for the
  // active search cascade. treeData is NEVER written to by cascade loads.
  const [searchResults, setSearchResults]           = useState<DataNode[]>([]);
  const loadingNodes    = useRef<Set<string>>(new Set());
  const searchWasActive = useRef(false);
  const ctxRef = useRef<HTMLDivElement>(null);

  const pendingDiff   = useDiffStore((s) => s.pending);
  const selectForComp = useDiffStore((s) => s.selectForComparison);
  const compareWith   = useDiffStore((s) => s.compareWith);

  // Close context menu on outside click
  useEffect(() => {
    if (!ctxMenu) return;
    const close = () => setCtxMenu(null);
    window.addEventListener("click", close);
    return () => window.removeEventListener("click", close);
  }, [ctxMenu]);

  // ── Tree height resize ─────────────────────────────────────────────────────
  const [treeCollapsed, setTreeCollapsed] = useState(false);
  const [treeHeight, setTreeHeight] = useState(360);
  const [resizingTree, setResizingTree] = useState(false);
  const treeResizeStartY = useRef(0);
  const treeResizeStartH = useRef(0);

  useEffect(() => {
    if (!resizingTree) return;
    document.body.style.cursor = "row-resize";
    document.body.style.userSelect = "none";
    const onMove = (e: MouseEvent) => {
      const delta = e.clientY - treeResizeStartY.current;
      setTreeHeight(Math.max(80, Math.min(800, treeResizeStartH.current + delta)));
    };
    const onUp = () => setResizingTree(false);
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, [resizingTree]);

  // Cascade-load the full object tree into searchResults (never treeData).
  // treeData stays pristine, so clearing the search just resets searchResults.
  useEffect(() => {
    if (!searchQuery) return;

    // Step 1: ensure databases are loaded.
    if (!loaded) {
      loadDatabases();
      return; // re-runs when `loaded` flips true
    }

    // Step 2: on first activation seed searchResults from the current tree.
    if (!searchWasActive.current) {
      setSearchResults([...treeData]); // shallow copy — cascade writes new refs
      searchWasActive.current = true;
      return; // re-runs when searchResults initialises
    }

    if (searchResults.length === 0) return; // not yet seeded

    // Step 3: trigger schema loads for db nodes without children.
    let waiting = false;
    for (const dbNode of searchResults) {
      const key = String(dbNode.key);
      if (!(dbNode as any).children && !loadingNodes.current.has(key)) {
        loadingNodes.current.add(key);
        onLoadData(dbNode as any, setSearchResults).finally(() => loadingNodes.current.delete(key));
        waiting = true;
      }
    }
    if (waiting) return; // re-runs when searchResults gains schema children

    // Step 4: trigger object loads for schema nodes without children.
    for (const dbNode of searchResults) {
      for (const schemaNode of ((dbNode as any).children ?? []) as DataNode[]) {
        const key = String(schemaNode.key);
        if (!(schemaNode as any).children && !loadingNodes.current.has(key)) {
          loadingNodes.current.add(key);
          onLoadData(schemaNode as any, setSearchResults).finally(() => loadingNodes.current.delete(key));
          waiting = true;
        }
      }
    }
    if (waiting) return; // re-runs when searchResults gains object children

    // Step 5: all data loaded — expand every parent that contains a match.
    setSearchExpandedKeys(getAllParentKeys(filterTree(searchResults, searchQuery)));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchQuery, searchResults, loaded]);

  const displayData = useMemo(
    () => (searchQuery ? filterTree(searchResults, searchQuery) : treeData),
    [searchResults, treeData, searchQuery],
  );

  // Clamp context menu inside the viewport (runs before browser paint — no flash)
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

  const doLoadDatabases = async () => {
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

  const loadDatabases = () => {
    if (loaded) return;
    doLoadDatabases();
  };

  const refreshAllDatabases = () => {
    setLoaded(false);
    setTreeData([]);
    setSearchQuery("");
    setSearchResults([]);
    setExpandedKeys([]);
    setSearchExpandedKeys([]);
    searchWasActive.current = false;
    loadingNodes.current.clear();
    useObjectStore.getState().setDatabases([]);
    doLoadDatabases();
  };

  // commit is setSearchResults when called from the cascade; omitted (→ setTreeData)
  // for user-triggered tree expansion. Cascade results never touch treeData.
  const onLoadData = async (
    node: DataNode & { children?: DataNode[] },
    commit?: React.Dispatch<React.SetStateAction<DataNode[]>>,
  ) => {
    if (node.children) return;
    const key    = String(node.key);
    const parts  = key.split(":");
    const setData = commit ?? setTreeData;

    if (parts[0] === "db") {
      const db = parts[1];
      try {
        const schemas = await ListSchemas(db);
        setData((prev) =>
          updateNode(prev, key, schemas.map((s) => ({
            title:  s,
            key:    `schema:${db}:${s}`,
            icon:   <FolderOutlined />,
            isLeaf: false,
          })))
        );
        if (!commit) useObjectStore.getState().addSchemas(db, schemas);
      } catch {
        // Shared / restricted databases (e.g. SNOWFLAKE) don't support
        // SHOW SCHEMAS. Mark as empty so the cascade doesn't retry.
        setData((prev) => updateNode(prev, key, []));
      }
    } else if (parts[0] === "schema") {
      const [, db, schema] = parts;
      try {
        const objects = await ListObjects(db, schema);

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
          children: kind === "TASK"
            ? buildTaskTree(groups[kind], db, schema)
            : groups[kind].map((o) => ({
                title:     o.name,
                key:       `obj:${db}:${schema}:${kind}:${o.name}`,
                icon:      kindIcon(kind),
                isLeaf:    true,
                arguments: o.arguments ?? "",
                rowCount:  o.rowCount,
              })),
        }));

        setData((prev) => updateNode(prev, key, typeNodes));
        if (!commit) useObjectStore.getState().addObjects(db, schema, objects.map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })));
      } catch {
        // Schema not accessible — mark as empty so the cascade doesn't retry.
        setData((prev) => updateNode(prev, key, []));
      }
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
    } else if (key.startsWith("type:")) {
      // key format: type:DB:SCHEMA:KIND
      const objKind = key.split(":")[3];
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "type", objKind });
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
    // Stripping the children array is enough: onExpand will re-trigger
    // onLoadData when the user next expands this database.
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

  const showDroppedSchemas = async () => {
    if (!ctxMenu) return;
    const db = ctxMenu.nodeKey.slice("db:".length);
    setCtxMenu(null);
    setUndropSchemasModal({ db, schemas: null, error: null });
    try {
      const schemas = await ListDroppedSchemas(db);
      setUndropSchemasModal((prev) => prev ? { ...prev, schemas: schemas ?? [] } : null);
    } catch (e) {
      setUndropSchemasModal((prev) => prev ? { ...prev, schemas: [], error: String(e) } : null);
    }
  };

  const undropSchema = async (db: string, name: string) => {
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const sql = `UNDROP SCHEMA ${q(db)}.${q(name)};`;
    setUndropSchemasModal(null);
    await useQueryStore.getState().executeWith(sql);
    refreshDatabaseByName(db);
  };

  const showDroppedDatabases = async () => {
    setUndropDatabasesModal({ databases: null, error: null });
    try {
      const databases = await ListDroppedDatabases();
      setUndropDatabasesModal((prev) => prev ? { ...prev, databases: databases ?? [] } : null);
    } catch (e) {
      setUndropDatabasesModal((prev) => prev ? { ...prev, databases: [], error: String(e) } : null);
    }
  };

  const undropDatabase = async (name: string) => {
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const sql = `UNDROP DATABASE ${q(name)};`;
    setUndropDatabasesModal(null);
    await useQueryStore.getState().executeWith(sql);
    refreshAllDatabases();
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

  const selectFunction = () => {
    if (!ctxMenu) return;
    const { nodeKey, objArgs = "" } = ctxMenu;
    setCtxMenu(null);
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    setSelectFunctionModal({ db, schema, name, rawArgs: objArgs });
  };

  const executeNotebook = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    setExecuteNotebookModal({ db, schema, name });
  };

  const executeTask = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    setExecuteTaskModal({ db, schema, name });
  };

  const openTaskGraph = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    setTaskGraphModal({ db, schema, name });
  };

  const openCreateTask = () => {
    if (!ctxMenu) return;
    // Works for both schema:DB:SCHEMA and type:DB:SCHEMA:KIND keys
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateTaskModal({ db, schema });
  };

  const openTaskStatuses = () => {
    if (!ctxMenu) return;
    // key format: type:DB:SCHEMA:KIND
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setTaskStatusesModal({ db, schema });
  };

  const viewDependencies = () => {
    if (!ctxMenu) return;
    const { nodeKey, objKind = "", objArgs = "" } = ctxMenu;
    setCtxMenu(null);
    // key format: obj:DB:SCHEMA:KIND:NAME
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    setDepsModal({ db, schema, kind: objKind, name, args: objArgs });
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

  const openNotebookFromSnowflake = async () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    try {
      const content = await FetchNotebookContent(db, schema, name);
      useQueryStore.getState().openNotebookUnsaved(name, content);
    } catch (e) {
      message.error(`Failed to open notebook: ${String(e)}`);
    }
  };

  const openExportModal = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const table = nameParts.join(":");
    setCtxMenu(null);
    setExportModal({ db, schema, table });
  };

  const openImportModal = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const table = nameParts.join(":");
    setCtxMenu(null);
    setImportModal({ db, schema, table });
  };

  const openSchemaExportModal = () => {
    if (!ctxMenu) return;
    const [, db, schema] = ctxMenu.nodeKey.split(":");
    setCtxMenu(null);
    setExportModal({ db, schema, table: "" });
  };

  const openSchemaImportModal = () => {
    if (!ctxMenu) return;
    const [, db, schema] = ctxMenu.nodeKey.split(":");
    setCtxMenu(null);
    setImportModal({ db, schema, table: "" });
  };

  const openBackupSets = () => {
    if (!ctxMenu) return;
    const { nodeKey, nodeType } = ctxMenu;
    setCtxMenu(null);
    if (nodeType === "db") {
      const db = nodeKey.slice("db:".length);
      setBackupSetsModal({ scopeType: "DATABASE", db, schema: "", table: "" });
    } else if (nodeType === "schema") {
      const [, db, schema] = nodeKey.split(":");
      setBackupSetsModal({ scopeType: "SCHEMA", db, schema, table: "" });
    } else {
      // obj — TABLE
      const [, db, schema, , ...nameParts] = nodeKey.split(":");
      const table = nameParts.join(":");
      setBackupSetsModal({ scopeType: "TABLE", db, schema, table });
    }
  };

  const insertFullName = () => {
    if (!ctxMenu) return;
    const { nodeKey, nodeType } = ctxMenu;
    setCtxMenu(null);
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    if (nodeType === "db") {
      const db = nodeKey.slice("db:".length);
      insertAtCursor(q(db));
    } else if (nodeType === "schema") {
      const [, db, schema] = nodeKey.split(":");
      insertAtCursor(`${q(db)}.${q(schema)}`);
    } else {
      // key format: obj:DB:SCHEMA:KIND:NAME
      const [, db, schema, , ...nameParts] = nodeKey.split(":");
      const name = nameParts.join(":");
      insertAtCursor(`${q(db)}.${q(schema)}.${q(name)}`);
    }
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

  const viewProperties = async () => {
    if (!ctxMenu) return;
    const { nodeKey, nodeType, objKind = "" } = ctxMenu;
    setCtxMenu(null);

    let db = "", schema = "", kind = "", name = "", title = "";

    if (nodeType === "db") {
      db   = nodeKey.slice("db:".length);
      kind = "DATABASE";
      name = db;
      title = `Properties: DATABASE — ${db}`;
    } else if (nodeType === "schema") {
      // key format: schema:DB:SCHEMA
      [, db, schema] = nodeKey.split(":");
      kind  = "SCHEMA";
      name  = schema;
      title = `Properties: SCHEMA — ${db}.${schema}`;
    } else {
      // key format: obj:DB:SCHEMA:KIND:NAME
      const [, d, s, , ...nameParts] = nodeKey.split(":");
      db     = d;
      schema = s;
      kind   = objKind;
      name   = nameParts.join(":");
      title  = `Properties: ${objKind} — ${db}.${schema}.${name}`;
    }

    // Tasks get a dedicated editable properties modal.
    if (kind === "TASK") {
      setTaskPropsModal({ db, schema, name });
      return;
    }

    const tableContext = kind === "TABLE" ? { db, schema, table: name } : undefined;
    setPropsModal({ title, rows: null, error: null, tableContext });
    try {
      const rows = await GetObjectProperties(db, schema, kind, name);
      setPropsModal((prev) => prev ? { ...prev, rows: rows ?? [] } : null);
    } catch (e) {
      setPropsModal((prev) => prev ? { ...prev, rows: [], error: String(e) } : null);
    }
  };

  const selectObjForComparison = () => {
    if (!ctxMenu) return;
    const { nodeKey, objKind = "", objArgs = "" } = ctxMenu;
    const [, db, schema, kind, ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    const k = kind || objKind;
    setCtxMenu(null);
    selectForComp({
      category: "obj",
      label:    `${k}: ${db}.${schema}.${name}`,
      db, schema, kind: k, name, args: objArgs,
    });
    message.success(`Selected for comparison: ${name}`);
  };

  const compareObjWith = () => {
    if (!ctxMenu) return;
    const { nodeKey, objKind = "", objArgs = "" } = ctxMenu;
    const [, db, schema, kind, ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    const k = kind || objKind;
    setCtxMenu(null);
    compareWith({
      category: "obj",
      label:    `${k}: ${db}.${schema}.${name}`,
      db, schema, kind: k, name, args: objArgs,
    });
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

  // A menu item that reveals a cascading submenu on hover.
  // Uses a 150 ms hide-delay so the mouse can travel into the submenu panel
  // without it disappearing.
  const showSub = (key: string, triggerEl: HTMLElement) => {
    if (submenuTimer.current) clearTimeout(submenuTimer.current);
    const rect = triggerEl.getBoundingClientRect();
    setSubmenuDir(window.innerWidth - rect.right >= 160 ? "right" : "left");
    setActiveSubmenu(key);
  };
  const hideSub = () => {
    submenuTimer.current = setTimeout(() => setActiveSubmenu(null), 150);
  };
  const cancelHide = () => {
    if (submenuTimer.current) clearTimeout(submenuTimer.current);
  };

  const menuItemSub = (label: string, icon: React.ReactNode, subKey: string, children: React.ReactNode) => (
    <div style={{ position: "relative" }} onMouseEnter={(e) => showSub(subKey, e.currentTarget)} onMouseLeave={hideSub}>
      <div
        style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "6px 14px", fontSize: 13, cursor: "default", color: "var(--text)", background: activeSubmenu === subKey ? "var(--border)" : "transparent" }}
        onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
        onMouseLeave={(e) => (e.currentTarget.style.background = activeSubmenu === subKey ? "var(--border)" : "transparent")}
      >
        <span style={{ display: "flex", alignItems: "center", gap: 8 }}>{icon}{label}</span>
        <RightOutlined style={{ fontSize: 9, opacity: 0.5, marginLeft: 12 }} />
      </div>
      {activeSubmenu === subKey && (
        <div
          style={{ position: "absolute", top: 0, ...(submenuDir === "right" ? { left: "100%" } : { right: "100%" }), background: "var(--bg-overlay)", border: "1px solid var(--border)", borderRadius: 6, boxShadow: "0 4px 16px rgba(0,0,0,0.5)", minWidth: 160, zIndex: 10000 }}
          onMouseEnter={cancelHide}
          onMouseLeave={hideSub}
        >
          {children}
        </div>
      )}
    </div>
  );

  return (
    <div style={{ padding: "8px 4px" }}>
      <div style={{ display: "flex", alignItems: "center", padding: "0 4px 0 8px", marginBottom: treeCollapsed ? 4 : 8, gap: 2 }}>
        <div
          style={{ display: "flex", alignItems: "center", gap: 4, cursor: "pointer", flex: 1, padding: "2px 4px", borderRadius: 4 }}
          onClick={() => setTreeCollapsed((c) => !c)}
          onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
          onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
        >
          {treeCollapsed
            ? <CaretRightFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />
            : <CaretDownFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />
          }
          <Text style={{ fontSize: 11, color: "var(--text)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
            Objects
          </Text>
        </div>
        <Tooltip title="Show dropped databases">
          <Button
            type="text"
            size="small"
            icon={<RollbackOutlined style={{ fontSize: 11 }} />}
            onClick={showDroppedDatabases}
            style={{ height: 20, padding: "0 4px", minWidth: 0, color: "var(--text-muted)" }}
          />
        </Tooltip>
        <Tooltip title="Refresh all databases">
          <Button
            type="text"
            size="small"
            icon={<ReloadOutlined style={{ fontSize: 11 }} />}
            loading={loading}
            onClick={refreshAllDatabases}
            style={{ height: 20, padding: "0 4px", minWidth: 0, color: "var(--text-muted)" }}
          />
        </Tooltip>
      </div>

      {!treeCollapsed && (
        <div style={{ height: treeHeight, overflow: "auto" }}>
          <div style={{ padding: "0 8px 8px" }}>
            <Input
              ref={searchInputRef}
              size="small"
              placeholder="Filter objects…"
              prefix={<SearchOutlined style={{ color: "var(--text-muted)", fontSize: 11 }} />}
              allowClear
              value={searchQuery}
              onChange={(e) => {
                const val = e.target.value;
                if (!val && searchWasActive.current) {
                  setSearchResults([]);
                  setSearchExpandedKeys([]);
                  setExpandedKeys([]);
                  // Strip all cached schema/object children so the tree returns
                  // to a clean db-list-only view regardless of what was loaded
                  // during the cascade.
                  setTreeData((prev) =>
                    prev.map((dbNode) => {
                      const { children: _, ...rest } = dbNode as any;
                      return rest as DataNode;
                    })
                  );
                  loadingNodes.current.clear();
                  searchWasActive.current = false;
                }
                setSearchQuery(val);
              }}
              style={{ fontSize: 12 }}
            />
          </div>

          {loading && <Spin size="small" style={{ display: "block", margin: "16px auto" }} />}

          {!loaded && !loading && (
            <div style={{ padding: "16px 12px" }}>
              <Text type="secondary" style={{ cursor: "pointer", fontSize: 12 }} onClick={loadDatabases}>
                Click to load databases
              </Text>
            </div>
          )}

          {loaded && treeData.length === 0 && <Empty description="No databases" imageStyle={{ height: 40 }} />}

          {treeData.length > 0 && searchQuery && displayData.length === 0 && (
            <div style={{ padding: "12px", fontSize: 12, color: "var(--text-muted)" }}>
              No objects match "{searchQuery}"
            </div>
          )}

          {treeData.length > 0 && (!searchQuery || displayData.length > 0) && (
            <div style={{ overflowX: "auto" }}>
            <Tree
              treeData={displayData}
              onRightClick={onRightClick as any}
              expandedKeys={searchQuery ? searchExpandedKeys : expandedKeys}
              expandAction={"click" as any}
              motion={false as any}
              onExpand={(keys, { expanded, node }) => {
                if (searchQuery) {
                  setSearchExpandedKeys(keys as Key[]);
                } else {
                  setExpandedKeys(keys as Key[]);
                  // Trigger lazy load when a node without children is expanded.
                  // We drive loading from onExpand instead of the Tree's loadData
                  // prop so rc-tree never puts a node into "loading" state, which
                  // would block the user from collapsing it.
                  if (expanded && !(node as any).children) {
                    onLoadData(node as unknown as DataNode & { children?: DataNode[] });
                  }
                }
              }}
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
                  const args        = (node as any).arguments ?? "";
                  const rowCount    = (node as any).rowCount as number | undefined;
                  const isEmpty     = kind === "TABLE" && rowCount !== undefined && rowCount === 0;
                  const isFinalizer = !!(node as any).isFinalizer;
                  const tooltip = (
                    <ObjTooltip cacheKey={key} db={db} schema={schema} kind={kind} name={name} args={args}>
                      <span style={isEmpty ? { color: "var(--text-faint)" } : undefined}>
                        {String(node.title)}
                        {isFinalizer && (
                          <Tag color="purple" style={{ marginLeft: 5, fontSize: 10, lineHeight: "14px", padding: "0 4px", verticalAlign: "middle" }}>
                            Finalizer
                          </Tag>
                        )}
                      </span>
                    </ObjTooltip>
                  );
                  if (kind === "TABLE" || kind === "VIEW") {
                    return (
                      <span
                        draggable
                        onDragStart={(e) => {
                          e.dataTransfer.setData("thaw/table", JSON.stringify({ db, schema, name }));
                          e.dataTransfer.effectAllowed = "copy";
                          e.stopPropagation();
                        }}
                      >
                        {tooltip}
                      </span>
                    );
                  }
                  return tooltip;
                }
                return node.title as React.ReactNode;
              }}
            />
            </div>
          )}
        </div>
      )}

      {/* Resize handle */}
      {!treeCollapsed && (
        <div
          style={{
            height: 5,
            cursor: "row-resize",
            background: resizingTree ? "var(--accent)" : "transparent",
            borderBottom: "1px solid var(--border)",
            transition: resizingTree ? "none" : "background 0.15s",
          }}
          onMouseDown={(e) => {
            treeResizeStartY.current = e.clientY;
            treeResizeStartH.current = treeHeight;
            setResizingTree(true);
            e.preventDefault();
          }}
          onMouseEnter={(e) => { if (!resizingTree) e.currentTarget.style.background = "color-mix(in srgb, var(--accent) 26%, transparent)"; }}
          onMouseLeave={(e) => { if (!resizingTree) e.currentTarget.style.background = "transparent"; }}
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
          {ctxMenu.nodeType === "db" && menuItem("Insert Name", <CodeOutlined style={{ fontSize: 12 }} />, insertFullName)}
          {ctxMenu.nodeType === "db" && menuItem("Refresh", <ReloadOutlined style={{ fontSize: 12 }} />, refreshDatabase)}
          {ctxMenu.nodeType === "db" && menuItem("Show Dropped Schemas…", <RollbackOutlined style={{ fontSize: 12 }} />, showDroppedSchemas)}
          {ctxMenu.nodeType === "db" && menuItem("Export DDL", <CloudUploadOutlined style={{ fontSize: 12 }} />, exportDatabase)}
          {ctxMenu.nodeType === "db" && menuItem("ER Diagram…", <ApartmentOutlined style={{ fontSize: 12 }} />, generateERDiagram)}
          {ctxMenu.nodeType === "db" && menuItem("Backup Sets…", <SaveOutlined style={{ fontSize: 12 }} />, openBackupSets)}
          {ctxMenu.nodeType === "db" && menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, viewProperties)}
          {ctxMenu.nodeType === "schema" && menuItem("Insert Name", <CodeOutlined style={{ fontSize: 12 }} />, insertFullName)}
          {ctxMenu.nodeType === "schema" && menuItemSub("Create Object", <PlusSquareOutlined style={{ fontSize: 12 }} />, "create-object", (
            menuItem("Task…", <ClockCircleOutlined style={{ fontSize: 12 }} />, openCreateTask)
          ))}
          {ctxMenu.nodeType === "schema" && menuItem("Show Dropped Tables…", <RollbackOutlined style={{ fontSize: 12 }} />, showDroppedTables)}
          {ctxMenu.nodeType === "schema" && menuItem("Export Data…", <DownloadOutlined style={{ fontSize: 12 }} />, openSchemaExportModal)}
          {ctxMenu.nodeType === "schema" && menuItem("Import Data…", <UploadOutlined style={{ fontSize: 12 }} />, openSchemaImportModal)}
          {ctxMenu.nodeType === "schema" && menuItem("Backup Sets…", <SaveOutlined style={{ fontSize: 12 }} />, openBackupSets)}
          {ctxMenu.nodeType === "schema" && menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, viewProperties)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "TASK" &&
            menuItem("Task Statuses…", <DashboardOutlined style={{ fontSize: 12 }} />, openTaskStatuses)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "TASK" &&
            menuItem("Create Task…", <ClockCircleOutlined style={{ fontSize: 12 }} />, openCreateTask)}
          {ctxMenu.nodeType === "obj" && (ctxMenu.objKind === "TABLE" || ctxMenu.objKind === "VIEW") &&
            menuItem("Select Top 1000 Rows", <TableOutlined style={{ fontSize: 12 }} />, selectTop1000)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Time Travel Query…", <HistoryOutlined style={{ fontSize: 12 }} />, openTimeTravelModal)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Export Data…", <DownloadOutlined style={{ fontSize: 12 }} />, openExportModal)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Import Data…", <UploadOutlined style={{ fontSize: 12 }} />, openImportModal)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Backup Sets…", <SaveOutlined style={{ fontSize: 12 }} />, openBackupSets)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TASK" &&
            menuItem("Execute Task", <PlayCircleOutlined style={{ fontSize: 12 }} />, executeTask)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TASK" &&
            menuItem("View Task Graph…", <ShareAltOutlined style={{ fontSize: 12 }} />, openTaskGraph)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PROCEDURE" &&
            menuItem("Call Procedure", <PlayCircleOutlined style={{ fontSize: 12 }} />, callProcedure)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "FUNCTION" &&
            menuItem("Call Function…", <FunctionOutlined style={{ fontSize: 12 }} />, selectFunction)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "NOTEBOOK" &&
            menuItem("Open Notebook", <ExperimentOutlined style={{ fontSize: 12 }} />, openNotebookFromSnowflake)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "NOTEBOOK" &&
            menuItem("Execute Notebook…", <PlayCircleOutlined style={{ fontSize: 12 }} />, executeNotebook)}
          {ctxMenu.nodeType === "obj" && menuItem("Insert Full Name", <CodeOutlined style={{ fontSize: 12 }} />, insertFullName)}
          {ctxMenu.nodeType === "obj" && menuItem("View Definition", null, viewDefinition)}
          {ctxMenu.nodeType === "obj" && menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, viewProperties)}
          {ctxMenu.nodeType === "obj" &&
            menuItem("Select for Comparison", <DiffOutlined style={{ fontSize: 12 }} />, selectObjForComparison)}
          {ctxMenu.nodeType === "obj" && pendingDiff !== null &&
            menuItem(`Compare with: ${pendingDiff.label}`, <DiffOutlined style={{ fontSize: 12, color: "var(--accent)" }} />, compareObjWith)}
          {ctxMenu.nodeType === "obj" &&
            (ctxMenu.objKind === "VIEW" || ctxMenu.objKind === "PROCEDURE" || ctxMenu.objKind === "FUNCTION") &&
            menuItem("View Dependencies…", <ShareAltOutlined style={{ fontSize: 12 }} />, viewDependencies)}
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
        footer={[
          <Button
            key="copy"
            icon={<CopyOutlined />}
            disabled={!ddlModal?.src || !!ddlModal?.loading}
            onClick={() => {
              if (!ddlModal?.src) return;
              ClipboardSetText(ddlModal.src).then(() => message.success("Copied to clipboard"));
            }}
          >
            Copy
          </Button>,
          <Button key="close" onClick={() => setDdlModal(null)}>
            Close
          </Button>,
        ]}
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

      {!hideAccountPanel && (
        <>
          <Divider style={{ borderColor: "var(--border)", margin: "8px 0 0" }} />
          <AccountPanel />
        </>
      )}

      {/* Task Properties modal */}
      {taskPropsModal && (
        <TaskPropertiesModal
          db={taskPropsModal.db}
          schema={taskPropsModal.schema}
          name={taskPropsModal.name}
          onClose={() => setTaskPropsModal(null)}
        />
      )}

      {/* Task Graph modal */}
      {taskGraphModal && (
        <TaskGraphModal
          db={taskGraphModal.db}
          schema={taskGraphModal.schema}
          taskName={taskGraphModal.name}
          onClose={() => setTaskGraphModal(null)}
        />
      )}

      {/* Task Statuses modal */}
      {taskStatusesModal && (
        <TaskStatusesModal
          db={taskStatusesModal.db}
          schema={taskStatusesModal.schema}
          onClose={() => setTaskStatusesModal(null)}
        />
      )}

      {/* Execute Task modal */}
      {executeTaskModal && (
        <ExecuteTaskModal
          db={executeTaskModal.db}
          schema={executeTaskModal.schema}
          name={executeTaskModal.name}
          onClose={() => setExecuteTaskModal(null)}
        />
      )}

      {/* Execute Notebook modal */}
      {executeNotebookModal && (
        <ExecuteNotebookModal
          db={executeNotebookModal.db}
          schema={executeNotebookModal.schema}
          name={executeNotebookModal.name}
          onClose={() => setExecuteNotebookModal(null)}
        />
      )}

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

      {/* Select Function modal */}
      {selectFunctionModal && (
        <SelectFunctionModal
          db={selectFunctionModal.db}
          schema={selectFunctionModal.schema}
          name={selectFunctionModal.name}
          rawArgs={selectFunctionModal.rawArgs}
          onClose={() => setSelectFunctionModal(null)}
        />
      )}

      {/* Create Task modal */}
      {createTaskModal && (
        <CreateTaskModal
          db={createTaskModal.db}
          schema={createTaskModal.schema}
          onClose={() => setCreateTaskModal(null)}
        />
      )}

      {/* Backup Sets modal */}
      {backupSetsModal && (
        <BackupSetsModal
          scopeType={backupSetsModal.scopeType}
          db={backupSetsModal.db}
          schema={backupSetsModal.schema}
          table={backupSetsModal.table}
          onClose={() => setBackupSetsModal(null)}
        />
      )}

      {/* Dependencies modal */}
      {depsModal && (
        <DependenciesModal
          open
          database={depsModal.db}
          schema={depsModal.schema}
          kind={depsModal.kind}
          name={depsModal.name}
          arguments={depsModal.args}
          onClose={() => setDepsModal(null)}
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
      {/* Undrop Schemas modal */}
      <Modal
        open={undropSchemasModal !== null}
        title={undropSchemasModal ? `Dropped schemas — ${undropSchemasModal.db}` : ""}
        onCancel={() => setUndropSchemasModal(null)}
        footer={null}
        width={560}
      >
        {undropSchemasModal?.schemas === null && !undropSchemasModal?.error && (
          <div style={{ textAlign: "center", padding: "24px 0" }}><Spin /></div>
        )}
        {undropSchemasModal?.error && (
          <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>
            {undropSchemasModal.error}
          </div>
        )}
        {undropSchemasModal?.schemas !== null && !undropSchemasModal?.error && undropSchemasModal?.schemas?.length === 0 && (
          <div style={{ color: "var(--text-muted)", fontSize: 13, padding: "12px 0" }}>
            No dropped schemas found within the Time Travel retention window.
          </div>
        )}
        {undropSchemasModal?.schemas !== null && !undropSchemasModal?.error && (undropSchemasModal?.schemas?.length ?? 0) > 0 && (
          <div>
            {undropSchemasModal!.schemas!.map((s) => (
              <div key={s.name} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "8px 4px", borderBottom: "1px solid var(--border)" }}>
                <div>
                  <div style={{ fontFamily: "monospace", fontSize: 13, color: "var(--text)" }}>{s.name}</div>
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>Dropped: {s.droppedOn}</div>
                </div>
                <Button size="small" icon={<RollbackOutlined />} onClick={() => undropSchema(undropSchemasModal!.db, s.name)}>
                  Undrop
                </Button>
              </div>
            ))}
          </div>
        )}
      </Modal>

      {/* Undrop Databases modal */}
      <Modal
        open={undropDatabasesModal !== null}
        title="Dropped databases"
        onCancel={() => setUndropDatabasesModal(null)}
        footer={null}
        width={560}
      >
        {undropDatabasesModal?.databases === null && !undropDatabasesModal?.error && (
          <div style={{ textAlign: "center", padding: "24px 0" }}><Spin /></div>
        )}
        {undropDatabasesModal?.error && (
          <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>
            {undropDatabasesModal.error}
          </div>
        )}
        {undropDatabasesModal?.databases !== null && !undropDatabasesModal?.error && undropDatabasesModal?.databases?.length === 0 && (
          <div style={{ color: "var(--text-muted)", fontSize: 13, padding: "12px 0" }}>
            No dropped databases found within the Time Travel retention window.
          </div>
        )}
        {undropDatabasesModal?.databases !== null && !undropDatabasesModal?.error && (undropDatabasesModal?.databases?.length ?? 0) > 0 && (
          <div>
            {undropDatabasesModal!.databases!.map((d) => (
              <div key={d.name} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "8px 4px", borderBottom: "1px solid var(--border)" }}>
                <div>
                  <div style={{ fontFamily: "monospace", fontSize: 13, color: "var(--text)" }}>{d.name}</div>
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>Dropped: {d.droppedOn}</div>
                </div>
                <Button size="small" icon={<RollbackOutlined />} onClick={() => undropDatabase(d.name)}>
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
          onDesignerSuccess={() => refreshDatabaseByName(erModal.database)}
        />
      )}

      {/* Properties modal */}
      {propsModal && (
        <PropertiesModal
          title={propsModal.title}
          rows={propsModal.rows}
          error={propsModal.error}
          onClose={() => setPropsModal(null)}
          tableContext={propsModal.tableContext}
        />
      )}

      {/* Export Table Data modal */}
      {exportModal && (
        <ExportTableModal
          db={exportModal.db}
          schema={exportModal.schema}
          table={exportModal.table}
          onClose={() => setExportModal(null)}
        />
      )}

      {/* Import Table Data modal */}
      {importModal && (
        <ImportTableModal
          db={importModal.db}
          schema={importModal.schema}
          table={importModal.table}
          onClose={() => setImportModal(null)}
          onSuccess={() => refreshDatabaseByName(importModal.db)}
        />
      )}

    </div>
  );
}
