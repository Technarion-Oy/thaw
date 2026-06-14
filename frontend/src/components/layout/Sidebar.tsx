// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useLayoutEffect, useMemo, useRef } from "react";
import { App as AntApp, Tree, Typography, Spin, Empty, Divider, Modal, Button, Input, Tooltip, Slider, Tag, Space, message, Select, type InputRef } from "antd";
import {
  DatabaseOutlined,
  TableOutlined,
  EyeOutlined,
  FunctionOutlined,
  CodeOutlined,
  InboxOutlined,
  ApiOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  FileOutlined,
  FolderOutlined,
  ReloadOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  BarChartOutlined,
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
  PlusOutlined,
  PlusSquareOutlined,
  MinusSquareOutlined,
  RightOutlined,
  ShareAltOutlined,
  ExperimentOutlined,
  BuildOutlined,
  DashboardOutlined,
  SyncOutlined,
  RetweetOutlined,
  CloudServerOutlined,
  BlockOutlined,
  AlertOutlined,
  TagsOutlined,
  EyeInvisibleOutlined,
  GlobalOutlined,
  ThunderboltOutlined,
  KeyOutlined,
  DisconnectOutlined,
  BranchesOutlined,
  CloseOutlined,
} from "@ant-design/icons";
import {
  objectIcon,
  databaseIcon,
  schemaIcon,
  typeGroupIcon,
  columnIcon,
} from "../sidebar/objectIcons";
import { ClipboardSetText, EventsOn } from "../../../wailsjs/runtime/runtime";
import type { DataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import { ListDatabases, ListSchemas, ListObjects, ListBasicObjects, ClearObjectCache, ClearObjectCacheForDatabase, GetObjectDDL, GetObjectProperties, ExportDatabaseDDL, ListDroppedTables, ListDroppedSchemas, ListDroppedDatabases, GetTableRetentionDays, GetDatabaseRetentionDays, GetSchemaRetentionDays, GetERDiagramData, FetchNotebookContent, DropTaskTree, GetQuotedIdentifiersIgnoreCase, MakeNotebookLive, GetTableColumnsWithTypes, GetTableForeignKeys, ListGitRepoEntries, ListGitBranches, ListGitTags, SetGitCommitFilter, GetGitCommitFilter, GetGitFileContent, ExecuteGitFile, DropDatabase, DropSchema, AlterPipe, AlterDynamicTable, AlterExternalTable, AlterMaterializedView, AlterAlert, ExecuteAlert, UploadFileToStage, PickOpenFile, ExecDDL, ListStageEntries, ExecuteStageFile, ListDbtProjectVersions, ListDbtProjectEntries, DownloadFileFromStage, RemoveStageFiles, PickDirectory, BuildDropColumnSql, BuildRenameColumnSql, BuildSetColumnNotNullSql, BuildDropColumnNotNullSql, BuildSetColumnCommentSql, BuildChangeColumnTypeSql } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl, { identToken, quoteIdent } from "../shared/ObjectNameCaseControl";
import type { snowflake } from "../../../wailsjs/go/models";
import { useQueryStore } from "../../store/queryStore";
import { insertAtCursor } from "../editor/editorRef";
import { useObjectStore } from "../../store/objectStore";
import { useConnectionStore } from "../../store/connectionStore";
import { useGitStore } from "../../store/gitStore";
import { useDiffStore } from "../../store/diffStore";
import { useInsertMappingStore } from "../../store/insertMappingStore";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import AccountPanel from "../account/AccountPanel";
import CallProcedureModal from "../procedure/CallProcedureModal";
import ExecuteNotebookModal from "../notebook/ExecuteNotebookModal";
import SelectFunctionModal from "../function/SelectFunctionModal";
import CreateTaskModal from "../task/CreateTaskModal";
import CreateDatabaseModal from "../database/CreateDatabaseModal";
import CreateTableModal from "../database/CreateTableModal";
import AddColumnModal from "../database/AddColumnModal";
import DataTypeSelect from "../shared/DataTypeSelect";
import CreateFileFormatModal from "../database/CreateFileFormatModal";
import ObjectSummariesModal from "../database/ObjectSummariesModal";
import ExecuteTaskModal from "../task/ExecuteTaskModal";
import TaskGraphModal from "../task/TaskGraphModal";
import TaskHistoryModal from "../task/TaskHistoryModal";
import TaskPropertiesModal from "../task/TaskPropertiesModal";
import TaskStatusesModal from "../task/TaskStatusesModal";
import ERDiagramModal from "../er/ERDiagramModal";
import ERDesigner from "../er/ERDesigner";
import ExportTableModal from "../export/ExportTableModal";
import ImportTableModal from "../export/ImportTableModal";
import PropertiesModal from "../common/PropertiesModal";
import BackupSetsModal from "../backup/BackupSetsModal";
import DependenciesModal from "../lineage/DependenciesModal";
import InsertMappingModal from "../database/InsertMappingModal";
import CreateSecretModal from "../secret/CreateSecretModal";
import ModifySecretModal from "../secret/ModifySecretModal";
import CreateGitRepositoryModal from "../gitrepoobj/CreateGitRepositoryModal";
import ModifyGitRepositoryModal from "../gitrepoobj/ModifyGitRepositoryModal";
import SetGitCommitFilterModal from "../gitrepoobj/SetGitCommitFilterModal";
import CreateDynamicTableModal from "../dynamictable/CreateDynamicTableModal";
import DynamicTablePropertiesModal from "../dynamictable/DynamicTablePropertiesModal";
import CreateExternalTableModal from "../externaltable/CreateExternalTableModal";
import ExternalTablePropertiesModal from "../externaltable/ExternalTablePropertiesModal";
import CreateMaterializedViewModal from "../materializedview/CreateMaterializedViewModal";
import MaterializedViewPropertiesModal from "../materializedview/MaterializedViewPropertiesModal";
import CreateAlertModal from "../alert/CreateAlertModal";
import AlertPropertiesModal from "../alert/AlertPropertiesModal";
import CreateTagModal from "../tag/CreateTagModal";
import TagPropertiesModal from "../tag/TagPropertiesModal";
import CreateMaskingPolicyModal from "../maskingpolicy/CreateMaskingPolicyModal";
import MaskingPolicyPropertiesModal from "../maskingpolicy/MaskingPolicyPropertiesModal";
import CreateNetworkRuleModal from "../networkrule/CreateNetworkRuleModal";
import NetworkRulePropertiesModal from "../networkrule/NetworkRulePropertiesModal";
import CreatePipeModal from "../pipe/CreatePipeModal";
import PipePropertiesModal from "../pipe/PipePropertiesModal";
import RefreshPipeModal from "../pipe/RefreshPipeModal";
import PipeCopyHistoryModal from "../pipe/PipeCopyHistoryModal";
import PipeStatusModal from "../pipe/PipeStatusModal";
import CreateStageModal from "../database/CreateStageModal";
import StagePropertiesModal from "../database/StagePropertiesModal";
import StageBrowserModal from "../database/StageBrowserModal";
import CreateDbtProjectModal from "../dbtproject/CreateDbtProjectModal";
import ExecuteDbtProjectModal from "../dbtproject/ExecuteDbtProjectModal";
import ModifyDbtProjectModal from "../dbtproject/ModifyDbtProjectModal";
import AddDbtProjectVersionModal from "../dbtproject/AddDbtProjectVersionModal";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

const { Text } = Typography;

const KIND_LABEL: Record<string, string> = {
  TABLE:         "Tables",
  VIEW:          "Views",
  "DYNAMIC TABLE": "Dynamic Tables",
  "EXTERNAL TABLE": "External Tables",
  "MATERIALIZED VIEW": "Materialized Views",
  ALERT:         "Alerts",
  TAG:           "Tags",
  "MASKING POLICY": "Masking Policies",
  "NETWORK RULE": "Network Rules",
  FUNCTION:      "Functions",
  PROCEDURE:     "Procedures",
  SEQUENCE:      "Sequences",
  STAGE:         "Stages",
  STREAM:        "Streams",
  TASK:          "Tasks",
  "FILE FORMAT": "File Formats",
  PIPE:          "Pipes",
  NOTEBOOK:      "Notebooks",
  SECRET:        "Secrets",
  "GIT REPOSITORY": "Git Repositories",
  "DBT PROJECT": "DBT Projects",
};

const KIND_ORDER = ["TABLE", "VIEW", "MATERIALIZED VIEW", "DYNAMIC TABLE", "EXTERNAL TABLE", "FUNCTION", "PROCEDURE", "SEQUENCE", "STAGE", "STREAM", "TASK", "ALERT", "TAG", "MASKING POLICY", "NETWORK RULE", "FILE FORMAT", "PIPE", "NOTEBOOK", "SECRET", "GIT REPOSITORY", "DBT PROJECT"];

const kindIcon = (kind: string) => objectIcon(kind);

interface ContextMenu {
  x: number;
  y: number;
  nodeKey: string;
  // dbtfile is intentionally absent — DBT files have no context menu actions
  nodeType: "db" | "schema" | "type" | "obj" | "col" | "gitcommits" | "gitdir" | "gitfile" | "stagedir" | "stagefile" | "dbtversion" | "dbtdir";
  objKind?: string;     // set for nodeType === "type" or "obj"
  objArgs?: string;     // parameter type list for PROCEDURE / FUNCTION
  isFinalizer?: boolean; // true when right-clicking a finalizer TASK node
  isRootTask?: boolean;  // true when right-clicking a root-level TASK node (no predecessors, not a finalizer)
  colMeta?: { dataType: string; nullable: boolean; isPrimaryKey: boolean; parentKind: string; comment: string }; // set for nodeType === "col"
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
  caseSensitive: boolean;
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

// Rebuild a database node's schema children from a fresh `ListSchemas` result
// without collapsing the tree. Schemas that are currently expanded keep their
// already-loaded children (so the open path and scroll position survive and the
// tree never flickers); every other schema — collapsed, newly created, or just
// restored via UNDROP — becomes a fresh childless node so its objects re-fetch
// on the next expand. New schemas appear and dropped ones disappear because the
// children are driven entirely by `schemaNames`. See issue #493.
function syncDatabaseSchemas(
  nodes: DataNode[],
  dbKey: string,
  db: string,
  schemaNames: string[],
  keepLoadedSchemaKeys: Set<string>,
): DataNode[] {
  return nodes.map((node) => {
    if (node.key === dbKey) {
      const existing = new Map(
        (((node as any).children ?? []) as DataNode[]).map((c) => [String(c.key), c]),
      );
      const children: DataNode[] = schemaNames.map((name) => {
        const schemaKey = `schema:${db}:${name}`;
        const prev = existing.get(schemaKey);
        // Preserve loaded children only for schemas we keep open and refresh;
        // anything else is reset to a childless node so it reloads on expand.
        if (prev && keepLoadedSchemaKeys.has(schemaKey) && (prev as any).children) {
          return prev;
        }
        return { title: name, key: schemaKey, icon: schemaIcon(), isLeaf: false };
      });
      return { ...node, children };
    }
    if ((node as any).children) {
      return { ...node, children: syncDatabaseSchemas((node as any).children, dbKey, db, schemaNames, keepLoadedSchemaKeys) };
    }
    return node;
  });
}

// Remove a node by key from the tree (used after DROP DATABASE / DROP SCHEMA).
function removeNode(nodes: DataNode[], targetKey: string): DataNode[] {
  return nodes
    .filter((node) => node.key !== targetKey)
    .map((node) => {
      if ((node as any).children) {
        const updated = removeNode((node as any).children, targetKey);
        // If the last child was removed, strip children entirely so
        // loadData re-triggers on the next expand (re-fetches from server).
        if (updated.length === 0) {
          const { children, ...rest } = node as any;
          return rest;
        }
        return { ...node, children: updated };
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
  const makeNode = (o: snowflake.SnowflakeObject, kids: DataNode[] = [], isFinalizer = false, isRootTask = false): DataNode => ({
    title:       o.name,
    key:         `obj:${db}:${schema}:TASK:${o.name}`,
    icon:        kindIcon("TASK"),
    isLeaf:      kids.length === 0,
    isFinalizer, // consumed by titleRender
    isRootTask,  // consumed by context menu for task history
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

  function buildSubTree(name: string, isRoot: boolean): DataNode {
    inTree.add(name.toUpperCase());
    const task = byName.get(name.toUpperCase())!;
    const kids = (childrenOf.get(name.toUpperCase()) ?? []).map((n) => buildSubTree(n, false));
    // Attach finalizer task as the last child if this is its designated root task.
    const finTask = finalizerOf.get(name.toUpperCase());
    if (finTask) {
      inTree.add(finTask.name.toUpperCase());
      kids.push(makeNode(finTask, [], true));
    }
    return makeNode(task, kids, false, isRoot);
  }

  const result: DataNode[] = [];
  for (const t of tasks) {
    // Skip finalizer tasks (placed inside their root) and tasks with a parent.
    if (finalizerNames.has(t.name.toUpperCase())) continue;
    if (!parentOf.has(t.name.toUpperCase())) result.push(buildSubTree(t.name, true));
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

// --- Pure helpers for tree node construction (module-level to avoid re-creation per render) ---

// Parses keys in the format prefix:DB:SCHEMA:NAME:path (stagefile, stagedir, dbtdir, dbtversion).
// Currently used for stage file actions (execute, download, delete, upload) and stageFileIsSql.
function parseStageOrDbtKey(menu: ContextMenu | null): { db: string; schema: string; name: string; path: string } | null {
  if (!menu) return null;
  const parts = menu.nodeKey.split(":");
  if (parts.length < 4) return null;
  return { db: parts[1], schema: parts[2], name: parts[3], path: parts.slice(4).join(":") };
}

function buildEntryNodes(
  db: string, schema: string, name: string, entries: snowflake.GitRepoEntry[],
  dirPrefix: string, filePrefix: string,
): DataNode[] {
  return entries.map((e) => ({
    title: e.name,
    key: `${e.isDir ? dirPrefix : filePrefix}:${db}:${schema}:${name}:${e.path}`,
    icon: e.isDir
      ? <FolderOutlined style={{ color: "var(--text-muted)" }} />
      : <FileOutlined style={{ color: "var(--text-muted)", fontSize: "10px" }} />,
    isLeaf: !e.isDir,
  }));
}

function emptyChildNode(parentKey: string): DataNode {
  return {
    title: (
      <Text type="secondary" style={{ fontStyle: "italic", fontSize: 11 }}>
        (empty)
      </Text>
    ),
    key: `empty:${parentKey}`,
    isLeaf: true,
  };
}

export default function Sidebar({ hideAccountPanel = false }: { hideAccountPanel?: boolean }) {
  const { modal, message: contextMsg } = AntApp.useApp();
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading]   = useState(false);
  const [loaded, setLoaded]         = useState(false);

  const [ctxMenu, setCtxMenu]     = useState<ContextMenu | null>(null);
  const [selectedNodeKeys, setSelectedNodeKeys] = useState<Set<string>>(new Set());
  const [selectedNodeArgs, setSelectedNodeArgs] = useState<Map<string, string>>(new Map());
  const [activeSubmenu, setActiveSubmenu] = useState<string | null>(null);
  const [submenuDir, setSubmenuDir] = useState<"left" | "right">("right");
  const submenuTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [ddlModal, setDdlModal]   = useState<ObjectDDL | null>(null);
  const [callModal, setCallModal] = useState<{ db: string; schema: string; name: string; rawArgs: string } | null>(null);
  const [selectFunctionModal, setSelectFunctionModal] = useState<{ db: string; schema: string; name: string; rawArgs: string } | null>(null);
  const [executeNotebookModal, setExecuteNotebookModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createDbOpen, setCreateDbOpen] = useState(false);
  const [createTableModal, setCreateTableModal] = useState<{ db: string; schema: string } | null>(null);
  const [addColumnModal, setAddColumnModal] = useState<{ db: string; schema: string; table: string } | null>(null);
  const [renameColumnModal, setRenameColumnModal] = useState<{ db: string; schema: string; table: string; oldName: string; newName: string; caseSensitive: boolean } | null>(null);
  const [renameColQiic, setRenameColQiic] = useState(false);
  const [columnCommentModal, setColumnCommentModal] = useState<{ db: string; schema: string; table: string; column: string; comment: string } | null>(null);
  const [changeDataTypeModal, setChangeDataTypeModal] = useState<{ db: string; schema: string; table: string; column: string; dataType: string } | null>(null);
  const [createStageModal, setCreateStageModal] = useState<{ db: string; schema: string } | null>(null);
  const [stagePropertiesModal, setStagePropertiesModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [stageBrowserModal, setStageBrowserModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createFileFormatModal, setCreateFileFormatModal] = useState<{ db: string; schema: string } | null>(null);
  const [createSecretModal, setCreateSecretModal] = useState<{ db: string; schema: string } | null>(null);
  const [modifySecretModal, setModifySecretModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createGitRepoModal, setCreateGitRepoModal] = useState<{ db: string; schema: string } | null>(null);
  const [modifyGitRepoModal, setModifyGitRepoModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [gitCommitFilterModal, setGitCommitFilterModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createDbtProjectModal, setCreateDbtProjectModal] = useState<{ db: string; schema: string } | null>(null);
  const [executeDbtProjectModal, setExecuteDbtProjectModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [modifyDbtProjectModal, setModifyDbtProjectModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [addDbtProjectVersionModal, setAddDbtProjectVersionModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createDynamicTableModal, setCreateDynamicTableModal] = useState<{ db: string; schema: string } | null>(null);
  const [dynamicTablePropsModal, setDynamicTablePropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createExternalTableModal, setCreateExternalTableModal] = useState<{ db: string; schema: string } | null>(null);
  const [externalTablePropsModal, setExternalTablePropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createMaterializedViewModal, setCreateMaterializedViewModal] = useState<{ db: string; schema: string } | null>(null);
  const [materializedViewPropsModal, setMaterializedViewPropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createAlertModal, setCreateAlertModal] = useState<{ db: string; schema: string } | null>(null);
  const [alertPropsModal, setAlertPropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createTagModal, setCreateTagModal] = useState<{ db: string; schema: string } | null>(null);
  const [tagPropsModal, setTagPropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createMaskingPolicyModal, setCreateMaskingPolicyModal] = useState<{ db: string; schema: string } | null>(null);
  const [maskingPolicyPropsModal, setMaskingPolicyPropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createNetworkRuleModal, setCreateNetworkRuleModal] = useState<{ db: string; schema: string } | null>(null);
  const [networkRulePropsModal, setNetworkRulePropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [createPipeModal, setCreatePipeModal] = useState<{ db: string; schema: string } | null>(null);
  const [pipePropsModal, setPipePropsModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [refreshPipeModal, setRefreshPipeModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [pipeCopyHistoryModal, setPipeCopyHistoryModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [pipeStatusModal, setPipeStatusModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [objectSummariesModal, setObjectSummariesModal] = useState<string | null>(null);
  const [createTaskModal, setCreateTaskModal] = useState<{ db: string; schema: string } | null>(null);
  const [executeTaskModal, setExecuteTaskModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [taskPropsModal, setTaskPropsModal] = useState<{ db: string; schema: string; name: string; isFinalizer?: boolean } | null>(null);
  const [taskGraphModal, setTaskGraphModal] = useState<{ db: string; schema: string; name: string } | null>(null);
  const [taskHistoryModal, setTaskHistoryModal] = useState<{ db: string; schema: string; name: string; isRoot: boolean } | null>(null);
  const [taskStatusesModal, setTaskStatusesModal] = useState<{ db: string; schema: string } | null>(null);
  const [undropModal, setUndropModal] = useState<UndropModal | null>(null);
  const [undropSchemasModal, setUndropSchemasModal] = useState<UndropSchemasModal | null>(null);
  const [undropDatabasesModal, setUndropDatabasesModal] = useState<UndropDatabasesModal | null>(null);
  const [renameModal, setRenameModal] = useState<RenameModal | null>(null);
  const [renameQiic, setRenameQiic]   = useState(false);
  const [timeTravelModal, setTimeTravelModal] = useState<TimeTravelModal | null>(null);
  const [erModal, setErModal] = useState<{ database: string; data: snowflake.ERDiagramData } | null>(null);
  const [mcpErDesigner, setMcpErDesigner] = useState<{ database: string; merged: snowflake.ERDiagramData; baseline: snowflake.ERDiagramData } | null>(null);
  const [propsModal, setPropsModal] = useState<{ title: string; rows: snowflake.PropertyPair[] | null; error: string | null; tableContext?: { db: string; schema: string; table: string } } | null>(null);
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
  const [loadingGitNodes, setLoadingGitNodes] = useState<Set<string>>(new Set());
  const [loadingTreeNodes, setLoadingTreeNodes] = useState<Set<string>>(new Set());
  const searchWasActive = useRef(false);
  const ctxRef = useRef<HTMLDivElement>(null);
  // Scroll container around the object tree; used to preserve scroll position
  // across in-place refreshes (issue #493 follow-up).
  const treeScrollRef = useRef<HTMLDivElement>(null);

  const pendingDiff   = useDiffStore((s) => s.pending);
  const selectForComp = useDiffStore((s) => s.selectForComparison);
  const compareWith   = useDiffStore((s) => s.compareWith);

  const featureFlags = useFeatureFlagsStore((s) => s.flags);

  const isConnected = useConnectionStore((s) => s.isConnected);
  const prevConnectedRef = useRef(isConnected);
  useEffect(() => {
    if (isConnected && !prevConnectedRef.current) {
      refreshAllDatabases();
    }
    prevConnectedRef.current = isConnected;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isConnected]);

  // MCP open_task_graph — opens the task graph modal from an MCP tool call.
  useEffect(() => {
    const off = EventsOn("mcp:open-task-graph", (payload: { database: string; schema: string; task: string }) => {
      setTaskGraphModal({ db: payload.database, schema: payload.schema, name: payload.task });
    });
    return () => off();
  }, []);

  // MCP open_er_designer — opens the ER designer pre-populated with AI-generated tables.
  useEffect(() => {
    const off = EventsOn("mcp:open-er-designer", (payload: { database: string; merged: snowflake.ERDiagramData; baseline: snowflake.ERDiagramData }) => {
      setMcpErDesigner({ database: payload.database, merged: payload.merged, baseline: payload.baseline });
    });
    return () => off();
  }, []);

  const insertTarget    = useInsertMappingStore((s) => s.target);
  const insertSources   = useInsertMappingStore((s) => s.sources);
  const setInsertTarget = useInsertMappingStore((s) => s.setTarget);
  const addInsertSource = useInsertMappingStore((s) => s.addSource);

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
    // NOTE: basicOnly=true means only TABLEs, VIEWs, and SEQUENCEs are
    // searched. Extended types (PROCEDURE, FUNCTION, TASK, STREAM, STAGE,
    // etc.) won't appear in search results. This is a deliberate trade-off:
    // 1 query per schema instead of 11.
    for (const dbNode of searchResults) {
      for (const schemaNode of ((dbNode as any).children ?? []) as DataNode[]) {
        const key = String(schemaNode.key);
        if (!(schemaNode as any).children && !loadingNodes.current.has(key)) {
          loadingNodes.current.add(key);
          onLoadData(schemaNode as any, setSearchResults, true).finally(() => loadingNodes.current.delete(key));
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

  // Derived flag: is the right-clicked stage file a .sql file? Used to gate Execute File and the divider.
  const stageFileIsSql = useMemo(
    () => ctxMenu?.nodeType === "stagefile" && parseStageOrDbtKey(ctxMenu)?.path.toLowerCase().endsWith(".sql"),
    [ctxMenu],
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
          icon: databaseIcon(),
          isLeaf: false,
        }))
      );
      useObjectStore.getState().setDatabases(dbs);
      setLoaded(true);
      window.dispatchEvent(new Event("thaw:refresh-diagnostics"));
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

  const refreshAllDatabases = async () => {
    await ClearObjectCache();
    setLoaded(false);
    setTreeData([]);
    setSearchQuery("");
    setSearchResults([]);
    setExpandedKeys([]);
    setSearchExpandedKeys([]);
    searchWasActive.current = false;
    loadingNodes.current.clear();
    setLoadingTreeNodes(new Set());
    useObjectStore.getState().setDatabases([]);
    doLoadDatabases();
  };

  // commit is setSearchResults when called from the cascade; omitted (→ setTreeData)
  // for user-triggered tree expansion. Cascade results never touch treeData.
  // basicOnly skips extended object types (PROCEDURE, FUNCTION, TASK, etc.)
  // for faster loading during the search cascade.
  const onLoadData = async (
    node: DataNode & { children?: DataNode[] },
    commit?: React.Dispatch<React.SetStateAction<DataNode[]>>,
    basicOnly?: boolean,
  ) => {
    if (node.children) return;
    const key    = String(node.key);
    const parts  = key.split(":");
    const setData = commit ?? setTreeData;

    if (parts[0] === "db") {
      const db = parts[1];
      setLoadingTreeNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        const schemas = await ListSchemas(db);
        setData((prev) =>
          updateNode(prev, key, schemas.map((s) => ({
            title:  s,
            key:    `schema:${db}:${s}`,
            icon:   schemaIcon(),
            isLeaf: false,
          })))
        );
        if (!commit) useObjectStore.getState().addSchemas(db, schemas);
      } catch {
        // Shared / restricted databases (e.g. SNOWFLAKE) don't support
        // SHOW SCHEMAS. Mark as empty so the cascade doesn't retry.
        setData((prev) => updateNode(prev, key, []));
      } finally {
        setLoadingTreeNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "schema") {
      const [, db, schema] = parts;
      setLoadingTreeNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        // When in search cascade mode (basicOnly), check if objectStore already
        // has objects for this schema from a prior normal expansion. This avoids
        // any IPC call and includes all object types (procedures, tasks, etc.).
        const cached = basicOnly
          ? useObjectStore.getState().objects.filter((o) => o.db === db && o.schema === schema)
          : [];
        const objects = cached.length > 0
          ? cached.map((o) => ({ name: o.name, kind: o.kind })) as snowflake.SnowflakeObject[]
          : basicOnly
            ? await ListBasicObjects(db, schema)
            : await ListObjects(db, schema);

        const groups: Record<string, typeof objects> = {};
        for (const obj of objects) {
          const k = (obj.kind || "OTHER").toUpperCase();
          if (!groups[k]) groups[k] = [];
          groups[k].push(obj);
        }

        // Filter out gated object types
        if (!featureFlags.dbtProjectBrowser) delete groups["DBT PROJECT"];

        const sortedKinds = [
          ...KIND_ORDER.filter((k) => groups[k]),
          ...Object.keys(groups).filter((k) => !KIND_ORDER.includes(k)).sort(),
        ];

        const typeNodes: DataNode[] = sortedKinds.map((kind) => ({
          title:    KIND_LABEL[kind] ?? kind,
          key:      `type:${db}:${schema}:${kind}`,
          icon:     typeGroupIcon(),
          children: kind === "TASK"
            ? buildTaskTree(groups[kind], db, schema)
            : groups[kind].map((o) => ({
                title:     o.name,
                key:       `obj:${db}:${schema}:${kind}:${o.name}`,
                icon:      kindIcon(kind),
                isLeaf:    kind !== "TABLE" && kind !== "VIEW" && kind !== "GIT REPOSITORY" && kind !== "STAGE" && kind !== "DBT PROJECT",
                arguments: o.arguments ?? "",
                rowCount:  o.rowCount,
              })),
        }));

        setData((prev) => updateNode(prev, key, typeNodes));
        if (!commit) useObjectStore.getState().addObjects(db, schema, objects.map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })));
      } catch {
        // Schema not accessible — mark as empty so the cascade doesn't retry.
        setData((prev) => updateNode(prev, key, []));
      } finally {
        setLoadingTreeNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "obj") {
      const db     = parts[1];
      const schema = parts[2];
      const kind   = parts[3];
      const name   = parts.slice(4).join(":");

      if (kind === "TABLE" || kind === "VIEW") {
        setLoadingTreeNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
        try {
          const [columns, fks] = await Promise.all([
            GetTableColumnsWithTypes(db, schema, name),
            kind === "TABLE" ? GetTableForeignKeys(db, schema, name) : Promise.resolve([]),
          ]);

          const fkMap = new Map<string, snowflake.TableForeignKey>();
          if (fks) {
            for (const fk of fks) {
              fkMap.set(fk.fkColumn, fk);
            }
          }

          const colNodes: DataNode[] = (columns || []).map((c) => {
            const isPK = c.isPrimaryKey;
            const fk = fkMap.get(c.name);

            const title = (
              <span style={{ display: "flex", alignItems: "center", width: "100%", overflow: "hidden" }}>
                <Text style={{ fontSize: "12px" }} ellipsis>
                  {c.name}
                  {fk && (
                    <Text type="secondary" style={{ fontSize: "10px", marginLeft: "4px", fontStyle: "italic" }}>
                      → {fk.pkTable}
                    </Text>
                  )}
                </Text>
                <span style={{
                  marginLeft: "auto",
                  fontSize: 10,
                  color: "var(--text-faint)",
                  fontFamily: "var(--editor-font, monospace)",
                  textTransform: "uppercase",
                  letterSpacing: "0.04em",
                }}>
                  {c.dataType.split("(")[0]}
                </span>
              </span>
            );

            return {
              title,
              key: `col:${db}:${schema}:${name}:${c.name}`,
              icon: columnIcon(c.dataType, {
                primaryKey: isPK,
                foreignKey: !!fk,
              }),
              isLeaf: true,
              colDataType: c.dataType,
              colNullable: c.nullable,
              colIsPrimaryKey: isPK,
              colParentKind: kind,
              colComment: c.comment,
            };
          });

          setData((prev) => updateNode(prev, key, colNodes));
        } catch (e) {
          console.error(e);
          setData((prev) => updateNode(prev, key, []));
        } finally {
          setLoadingTreeNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
        }
      } else if (kind === "GIT REPOSITORY") {
        const typeNodes: DataNode[] = [
          {
            title: "branches",
            key: `gitbranches:${db}:${schema}:${name}`,
            icon: <FolderOutlined style={{ color: "var(--text-muted)" }} />,
            isLeaf: false,
          },
          {
            title: "tags",
            key: `gittags:${db}:${schema}:${name}`,
            icon: <FolderOutlined style={{ color: "var(--text-muted)" }} />,
            isLeaf: false,
          },
          {
            title: "commits",
            key: `gitcommits:${db}:${schema}:${name}`,
            icon: <FolderOutlined style={{ color: "var(--text-muted)" }} />,
            isLeaf: false,
          },
        ];
        setData((prev) => updateNode(prev, key, typeNodes));
      } else if (kind === "STAGE") {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
        try {
          const entries = await ListStageEntries(db, schema, name, "");
          const nodes = buildEntryNodes(db, schema, name, entries ?? [], "stagedir", "stagefile");
          setData((prev) => updateNode(prev, key, nodes.length ? nodes : [emptyChildNode(key)]));
        } catch (e) {
          console.error(e);
          setData((prev) => clearNodeChildren(prev, key));
        } finally {
          setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
        }
      } else if (kind === "DBT PROJECT") {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
        try {
          const versions = await ListDbtProjectVersions(db, schema, name);
          const nodes: DataNode[] = (versions ?? []).map((v) => {
            const badge = v.isDefault ? " (default)" : "";
            const label = v.alias ? `${v.version} — ${v.alias}${badge}` : `${v.version}${badge}`;
            return {
              title: label,
              key: `dbtversion:${db}:${schema}:${name}:${v.version}`,
              icon: <FolderOutlined style={{ color: "var(--text-muted)" }} />,
              isLeaf: false,
            };
          });
          setData((prev) => updateNode(prev, key, nodes.length ? nodes : [emptyChildNode(key)]));
        } catch (e) {
          console.error(e);
          setData((prev) => clearNodeChildren(prev, key));
        } finally {
          setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
        }
      }
    } else if (parts[0] === "gitbranches") {
      const db     = parts[1];
      const schema = parts[2];
      const repo   = parts[3];
      setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        const branches = await ListGitBranches(db, schema, repo);
        const items = (branches || []).map((b) => ({
          title: b.name,
          key: `gitdir:${db}:${schema}:${repo}:branches/${b.name}`,
          icon: <FolderOutlined />,
          isLeaf: false,
        }));
        setData((prev) => updateNode(prev, key, items.length ? items : [gitEmptyNode(key)]));
      } catch (e) {
        console.error(e);
        setData((prev) => clearNodeChildren(prev, key));
      } finally {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "gittags") {
      const db     = parts[1];
      const schema = parts[2];
      const repo   = parts[3];
      setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        const tags = await ListGitTags(db, schema, repo);
        const items = (tags || []).map((t) => ({
          title: t.name,
          key: `gitdir:${db}:${schema}:${repo}:tags/${t.name}`,
          icon: <FolderOutlined />,
          isLeaf: false,
        }));
        setData((prev) => updateNode(prev, key, items.length ? items : [gitEmptyNode(key)]));
      } catch (e) {
        console.error(e);
        setData((prev) => clearNodeChildren(prev, key));
      } finally {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "gitcommits") {
      const db     = parts[1];
      const schema = parts[2];
      const repo   = parts[3];
      setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        const commitHash = await GetGitCommitFilter(db, schema, repo);
        if (commitHash) {
          setData((prev) => updateNode(prev, key, [
            {
              title: commitHash,
              key: `gitdir:${db}:${schema}:${repo}:commits/${commitHash}`,
              icon: <FolderOutlined />,
              isLeaf: false,
            }
          ]));
        } else {
          setData((prev) => updateNode(prev, key, [
            {
              title: (
                <Space size={4}>
                  <EditOutlined style={{ fontSize: 10, color: "var(--accent)" }} />
                  <Text type="secondary" style={{ fontStyle: "italic", fontSize: 12, cursor: "pointer" }}>
                    (no commit filter set — click to set)
                  </Text>
                </Space>
              ),
              key: `gitcommit-empty:${db}:${schema}:${repo}`,
              isLeaf: true,
            }
          ]));
        }
      } catch (e) {
        console.error(e);
        setData((prev) => clearNodeChildren(prev, key));
      } finally {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "gitdir") {
      const db       = parts[1];
      const schema   = parts[2];
      const repoName = parts[3];
      const dirPath  = parts.slice(4).join(":");
      setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        const entries = await ListGitRepoEntries(db, schema, repoName, dirPath);
        const nodes = buildGitEntryNodes(db, schema, repoName, entries ?? []);
        setData((prev) => updateNode(prev, key, nodes.length ? nodes : [gitEmptyNode(key)]));
      } catch (e) {
        console.error(e);
        setData((prev) => clearNodeChildren(prev, key));
      } finally {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "stagedir") {
      const db        = parts[1];
      const schema    = parts[2];
      const stageName = parts[3];
      const dirPath   = parts.slice(4).join(":");
      setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        const entries = await ListStageEntries(db, schema, stageName, dirPath);
        const nodes = buildEntryNodes(db, schema, stageName, entries ?? [], "stagedir", "stagefile");
        setData((prev) => updateNode(prev, key, nodes.length ? nodes : [emptyChildNode(key)]));
      } catch (e) {
        console.error(e);
        setData((prev) => clearNodeChildren(prev, key));
      } finally {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "dbtversion") {
      const db      = parts[1];
      const schema  = parts[2];
      const dbtName = parts[3];
      const version = parts.slice(4).join(":");
      setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        // Snowflake-native DBT PROJECTs store files under @project/versions/<N>/…
        // (observed from SHOW VERSIONS IN DBT PROJECT and LIST @project output)
        const entries = await ListDbtProjectEntries(db, schema, dbtName, `versions/${version}/`);
        const nodes = buildEntryNodes(db, schema, dbtName, entries ?? [], "dbtdir", "dbtfile");
        setData((prev) => updateNode(prev, key, nodes.length ? nodes : [emptyChildNode(key)]));
      } catch (e) {
        console.error(e);
        setData((prev) => clearNodeChildren(prev, key));
      } finally {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    } else if (parts[0] === "dbtdir") {
      const db      = parts[1];
      const schema  = parts[2];
      const dbtName = parts[3];
      const dirPath = parts.slice(4).join(":");
      setLoadingGitNodes((prev) => { const s = new Set(prev); s.add(key); return s; });
      try {
        const entries = await ListDbtProjectEntries(db, schema, dbtName, dirPath);
        const nodes = buildEntryNodes(db, schema, dbtName, entries ?? [], "dbtdir", "dbtfile");
        setData((prev) => updateNode(prev, key, nodes.length ? nodes : [emptyChildNode(key)]));
      } catch (e) {
        console.error(e);
        setData((prev) => clearNodeChildren(prev, key));
      } finally {
        setLoadingGitNodes((prev) => { const s = new Set(prev); s.delete(key); return s; });
      }
    }
  };

  function buildGitEntryNodes(db: string, schema: string, repoName: string, entries: snowflake.GitRepoEntry[]): DataNode[] {
    return entries.map((e) => ({
      title: e.name,
      key: e.isDir
        ? `gitdir:${db}:${schema}:${repoName}:${e.path}`
        : `gitfile:${db}:${schema}:${repoName}:${e.path}`,
      icon: e.isDir
        ? <FolderOutlined style={{ color: "var(--text-muted)" }} />
        : <FileOutlined style={{ color: "var(--text-muted)", fontSize: "10px" }} />,
      isLeaf: !e.isDir,
    }));
  }

  function gitEmptyNode(parentKey: string): DataNode {
    return {
      title: (
        <Text type="secondary" style={{ fontStyle: "italic", fontSize: 11 }}>
          (empty)
        </Text>
      ),
      key: `gitempty:${parentKey}`,
      isLeaf: true,
    };
  }

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
      const objKind     = key.split(":")[3];
      const objArgs     = (node as any).arguments ?? "";
      const isFinalizer = !!(node as any).isFinalizer;
      const isRootTask  = !!(node as any).isRootTask;
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "obj", objKind, objArgs, isFinalizer, isRootTask });
    } else if (key.startsWith("gitcommits:") || key.startsWith("gitcommit-empty:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "gitcommits" });
    } else if (key.startsWith("gitdir:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "gitdir" });
    } else if (key.startsWith("gitfile:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "gitfile" });
    } else if (key.startsWith("stagedir:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "stagedir" });
    } else if (key.startsWith("stagefile:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "stagefile" });
    } else if (key.startsWith("dbtversion:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "dbtversion" });
    } else if (key.startsWith("dbtdir:")) {
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "dbtdir" });
    } else if (key.startsWith("col:")) {
      const colMeta = {
        dataType: (node as any).colDataType ?? "",
        nullable: !!(node as any).colNullable,
        isPrimaryKey: !!(node as any).colIsPrimaryKey,
        parentKind: (node as any).colParentKind ?? "",
        comment: (node as any).colComment ?? "",
      };
      setCtxMenu({ x: event.clientX, y: event.clientY, nodeKey: key, nodeType: "col", colMeta });
    }
  };

  // Refresh a database's objects, preserving the open tree path AND the scroll
  // position.
  //
  // The naive approach — stripping the whole `db:` subtree — drops every
  // descendant `schema:`/`type:`/`obj:` node from treeData while their keys
  // linger in `expandedKeys`, so Ant Design renders the previously-open path
  // collapsed; the tree also briefly shrinks to nothing, which resets the
  // scroll container to the top (issue #493).
  //
  // Instead, re-fetch the schema list and rebuild the db node's children with
  // `syncDatabaseSchemas`, which keeps the loaded children of currently-open
  // schemas intact (no collapse, no flicker) while picking up new / restored
  // schemas and dropping removed ones, and resets collapsed schemas to childless
  // so they re-fetch on next expand. Then reload each open schema's objects in
  // place. The tree never collapses, so the open path and scroll survive and
  // created / renamed / dropped objects appear where the user is looking.
  //
  // `reveal` (used after a create) force-expands the new object's
  // schema → type path so a brand-new type group opens automatically.
  const refreshDatabaseByName = async (
    db: string,
    reveal?: { schema: string; kind?: string },
  ) => {
    await ClearObjectCacheForDatabase(db);
    const dbKey = `db:${db}`;

    // Schemas whose loaded children we keep and refresh: every one currently
    // expanded under this db, plus the reveal target (which may not have been
    // expanded before the create).
    const openSchemaKeys = new Set(
      expandedKeys.map(String).filter((k) => k.startsWith(`schema:${db}:`)),
    );
    if (reveal) {
      const revealSchemaKey = `schema:${db}:${reveal.schema}`;
      openSchemaKeys.add(revealSchemaKey);
      const newlyOpen = [dbKey, revealSchemaKey];
      if (reveal.kind) newlyOpen.push(`type:${db}:${reveal.schema}:${reveal.kind}`);
      setExpandedKeys((prev) => {
        const set = new Set(prev.map(String));
        newlyOpen.forEach((k) => set.add(k));
        return Array.from(set) as Key[];
      });
    }

    // If the database node itself isn't open (and we're not revealing into it),
    // nothing is visible to preserve — strip its children so the next expand
    // re-fetches everything, and we're done.
    if (!expandedKeys.includes(dbKey) && !reveal) {
      useObjectStore.getState().clearDatabase(db);
      setTreeData((prev) => clearNodeChildren(prev, dbKey));
      return;
    }

    // Capture scroll so we can pin it back if a row-count change nudges it.
    const savedScrollTop = treeScrollRef.current?.scrollTop ?? 0;

    // Re-fetch the schema list so new / restored / dropped schemas are reflected,
    // then rebuild the db node's children without collapsing the open ones.
    let schemaNames: string[];
    try {
      schemaNames = await ListSchemas(db);
    } catch {
      // Shared / restricted databases (e.g. SNOWFLAKE) don't support SHOW
      // SCHEMAS — treat as empty, matching onLoadData's db branch.
      schemaNames = [];
    }
    useObjectStore.getState().addSchemas(db, schemaNames);
    setTreeData((prev) => syncDatabaseSchemas(prev, dbKey, db, schemaNames, openSchemaKeys));

    // Reload the objects of each open schema in place (fresh data). After the
    // sync above the reveal target exists as a node, so onLoadData populates it.
    // The per-schema reloads are independent (each does its own functional
    // setData) and order-insensitive, so fan the IPCs out in parallel rather
    // than serializing one ListObjects round-trip per open schema.
    const schemaPrefix = `schema:${db}:`;
    await Promise.all(
      Array.from(openSchemaKeys)
        .filter((schemaKey) => schemaNames.includes(schemaKey.slice(schemaPrefix.length)))
        .map((schemaKey) => onLoadData({ key: schemaKey } as DataNode & { children?: DataNode[] })),
    );

    // Restore scroll after React commits the rebuilt rows. A double rAF makes
    // this deterministic: the first frame lets React flush the batched setData
    // commits, the second runs after layout so scrollTop sticks.
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (treeScrollRef.current) treeScrollRef.current.scrollTop = savedScrollTop;
      });
    });
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
    try {
      await ExecDDL(sql);
      message.success(`Table "${name}" restored`);
      refreshDatabaseByName(db);
    } catch (e) {
      message.error(String(e));
    }
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
    try {
      await ExecDDL(sql);
      message.success(`Schema "${name}" restored`);
      refreshDatabaseByName(db);
    } catch (e) {
      message.error(String(e));
    }
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
    try {
      await ExecDDL(sql);
      message.success(`Database "${name}" restored`);
      refreshAllDatabases();
    } catch (e) {
      message.error(String(e));
    }
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

  const makeNotebookLive = async () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    try {
      await MakeNotebookLive(db, schema, name);
      message.success(`Notebook "${name}" is now live.`);
    } catch (e) {
      message.error(`Failed to make notebook live: ${String(e)}`);
    }
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

  const openTaskHistory = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    setTaskHistoryModal({ db, schema, name, isRoot: !!ctxMenu.isRootTask });
  };

  const openCreateTable = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateTableModal({ db, schema });
  };

  const openCreateStage = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateStageModal({ db, schema });
  };

  const openStageProperties = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    setStagePropertiesModal({ db, schema, name });
  };

  const openStageBrowser = () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    setStageBrowserModal({ db, schema, name });
  };

  const uploadToStage = async () => {
    if (!ctxMenu) return;
    const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);

    const localPath = await PickOpenFile();
    if (!localPath) return;

    const stageRef = `@${db}.${schema}.${name}`;
    const hide = message.loading(`Uploading ${localPath} to ${stageRef}…`, 0);
    try {
      await UploadFileToStage(localPath, stageRef, 4, true, "AUTO_DETECT", true);
      hide();
      message.success(`Uploaded ${localPath} successfully.`);
    } catch (e) {
      hide();
      message.error(`Failed to upload file: ${String(e)}`);
    }
  };

  const openCreateFileFormat = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateFileFormatModal({ db, schema });
  };

  const openObjectSummaries = () => {
    if (!ctxMenu) return;
    const db = ctxMenu.nodeKey.slice("db:".length);
    setCtxMenu(null);
    setObjectSummariesModal(db);
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

  const openCreateSecret = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateSecretModal({ db, schema });
  };

  const openModifySecret = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    setModifySecretModal({ db, schema, name });
  };

  const openCreateGitRepository = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateGitRepoModal({ db, schema });
  };

  const openModifyGitRepository = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    setModifyGitRepoModal({ db, schema, name });
  };

  const openCreateDbtProject = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateDbtProjectModal({ db, schema });
  };

  const openExecuteDbtProject = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    setExecuteDbtProjectModal({ db, schema, name });
  };

  const openModifyDbtProject = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    setModifyDbtProjectModal({ db, schema, name });
  };

  const openAddDbtProjectVersion = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    setAddDbtProjectVersionModal({ db, schema, name });
  };

  const showDbtProjectVersions = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    useQueryStore.getState().executeInNewTab(`SHOW VERSIONS IN DBT PROJECT ${quoteIdent(db)}.${quoteIdent(schema)}.${quoteIdent(name)};`);
  };

  const describeDbtProject = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    useQueryStore.getState().executeInNewTab(`DESCRIBE DBT PROJECT ${quoteIdent(db)}.${quoteIdent(schema)}.${quoteIdent(name)};`);
  };

  const openCreatePipe = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreatePipeModal({ db, schema });
  };

  const openPipeProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setPipePropsModal({ db, schema, name });
  };

  const openRefreshPipe = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setRefreshPipeModal({ db, schema, name });
  };

  const openPipeCopyHistory = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setPipeCopyHistoryModal({ db, schema, name });
  };

  const pausePipeExecution = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Pause Pipe Execution",
      content: `Pause execution of pipe "${name}"? Snowpipe will stop ingesting files until resumed.`,
      okText: "Pause",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await AlterPipe(db, schema, name, "SET PIPE_EXECUTION_PAUSED = TRUE");
          contextMsg.success(`Pipe "${name}" paused.`);
        } catch (e) {
          contextMsg.error(`Failed to pause pipe: ${String(e)}`);
        }
      },
    });
  };

  const unpausePipeExecution = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Resume Pipe Execution",
      content: `Resume execution of pipe "${name}"?`,
      okText: "Resume",
      onOk: async () => {
        try {
          await AlterPipe(db, schema, name, "SET PIPE_EXECUTION_PAUSED = FALSE");
          contextMsg.success(`Pipe "${name}" resumed.`);
        } catch (e) {
          contextMsg.error(`Failed to resume pipe: ${String(e)}`);
        }
      },
    });
  };

  const openPipeStatusModal = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setPipeStatusModal({ db, schema, name });
  };

  const openCreateDynamicTable = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateDynamicTableModal({ db, schema });
  };

  const openDynamicTableProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setDynamicTablePropsModal({ db, schema, name });
  };

  const suspendDynamicTable = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Suspend Dynamic Table",
      content: `Suspend automatic refreshes for "${name}"?`,
      okText: "Suspend",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await AlterDynamicTable(db, schema, name, "SUSPEND");
          contextMsg.success(`Dynamic table "${name}" suspended.`);
        } catch (e) {
          contextMsg.error(`Failed to suspend dynamic table: ${String(e)}`);
        }
      },
    });
  };

  const resumeDynamicTable = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Resume Dynamic Table",
      content: `Resume automatic refreshes for "${name}"?`,
      okText: "Resume",
      onOk: async () => {
        try {
          await AlterDynamicTable(db, schema, name, "RESUME");
          contextMsg.success(`Dynamic table "${name}" resumed.`);
        } catch (e) {
          contextMsg.error(`Failed to resume dynamic table: ${String(e)}`);
        }
      },
    });
  };

  const refreshDynamicTable = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Refresh Dynamic Table",
      content: `Trigger a manual refresh of "${name}" now?`,
      okText: "Refresh",
      onOk: async () => {
        try {
          await AlterDynamicTable(db, schema, name, "REFRESH");
          contextMsg.success(`Dynamic table "${name}" refresh triggered.`);
        } catch (e) {
          contextMsg.error(`Failed to refresh dynamic table: ${String(e)}`);
        }
      },
    });
  };

  const openCreateExternalTable = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateExternalTableModal({ db, schema });
  };

  const openExternalTableProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setExternalTablePropsModal({ db, schema, name });
  };

  const refreshExternalTable = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Refresh External Table",
      content: `Refresh the file-level metadata of "${name}" now?`,
      okText: "Refresh",
      onOk: async () => {
        try {
          await AlterExternalTable(db, schema, name, "REFRESH");
          contextMsg.success(`External table "${name}" refresh triggered.`);
        } catch (e) {
          contextMsg.error(`Failed to refresh external table: ${String(e)}`);
        }
      },
    });
  };

  const openCreateMaterializedView = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateMaterializedViewModal({ db, schema });
  };

  const openMaterializedViewProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setMaterializedViewPropsModal({ db, schema, name });
  };

  const suspendMaterializedView = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Suspend Materialized View",
      content: `Suspend use and maintenance of "${name}"?`,
      okText: "Suspend",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await AlterMaterializedView(db, schema, name, "SUSPEND");
          contextMsg.success(`Materialized view "${name}" suspended.`);
        } catch (e) {
          contextMsg.error(`Failed to suspend materialized view: ${String(e)}`);
        }
      },
    });
  };

  const resumeMaterializedView = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Resume Materialized View",
      content: `Resume use and maintenance of "${name}"?`,
      okText: "Resume",
      onOk: async () => {
        try {
          await AlterMaterializedView(db, schema, name, "RESUME");
          contextMsg.success(`Materialized view "${name}" resumed.`);
        } catch (e) {
          contextMsg.error(`Failed to resume materialized view: ${String(e)}`);
        }
      },
    });
  };

  const openCreateAlert = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateAlertModal({ db, schema });
  };

  const openAlertProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setAlertPropsModal({ db, schema, name });
  };

  const openCreateTag = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateTagModal({ db, schema });
  };

  const openTagProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setTagPropsModal({ db, schema, name });
  };

  const openCreateMaskingPolicy = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateMaskingPolicyModal({ db, schema });
  };

  const openMaskingPolicyProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setMaskingPolicyPropsModal({ db, schema, name });
  };

  const openCreateNetworkRule = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    setCtxMenu(null);
    setCreateNetworkRuleModal({ db, schema });
  };

  const openNetworkRuleProperties = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    setNetworkRulePropsModal({ db, schema, name });
  };

  const suspendAlert = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Suspend Alert",
      content: `Suspend scheduled evaluation of "${name}"?`,
      okText: "Suspend",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await AlterAlert(db, schema, name, "SUSPEND");
          contextMsg.success(`Alert "${name}" suspended.`);
        } catch (e) {
          contextMsg.error(`Failed to suspend alert: ${String(e)}`);
        }
      },
    });
  };

  const resumeAlert = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Resume Alert",
      content: `Resume scheduled evaluation of "${name}"?`,
      okText: "Resume",
      onOk: async () => {
        try {
          await AlterAlert(db, schema, name, "RESUME");
          contextMsg.success(`Alert "${name}" resumed.`);
        } catch (e) {
          contextMsg.error(`Failed to resume alert: ${String(e)}`);
        }
      },
    });
  };

  const executeAlert = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts.slice(4).join(":");
    setCtxMenu(null);
    modal.confirm({
      title: "Execute Alert",
      content: `Run an immediate evaluation of "${name}" now?`,
      okText: "Execute",
      onOk: async () => {
        try {
          await ExecuteAlert(db, schema, name);
          contextMsg.success(`Alert "${name}" executed.`);
        } catch (e) {
          contextMsg.error(`Failed to execute alert: ${String(e)}`);
        }
      },
    });
  };

  const openCommitFilterModal = () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    let db: string, schema: string, name: string;
    if (ctxMenu.nodeKey.startsWith("obj:")) {
      db = parts[1];
      schema = parts[2];
      name = parts[4];
    } else {
      db = parts[1];
      schema = parts[2];
      name = parts[3];
    }
    setCtxMenu(null);
    setGitCommitFilterModal({ db, schema, name });
  };

  const clearGitCommitFilter = async () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[3];
    const nodeKey = ctxMenu.nodeKey;
    setCtxMenu(null);
    await SetGitCommitFilter(db, schema, name, "");
    setTreeData((prev) => clearNodeChildren(prev, nodeKey.startsWith("gitcommit-empty") ? `gitcommits:${db}:${schema}:${name}` : nodeKey));
    message.success("Commit filter cleared");
  };

  const refreshTreeNode = () => {
    if (!ctxMenu) return;
    const key = ctxMenu.nodeKey;
    setCtxMenu(null);
    setTreeData((prev) => clearNodeChildren(prev, key));
  };

  const viewGitFileContent = async () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const repoName = parts[3];
    const filePath = parts.slice(4).join(":");
    setCtxMenu(null);
    const hide = message.loading(`Reading ${filePath}…`, 0);
    try {
      const content = await GetGitFileContent(db, schema, repoName, filePath);
      useQueryStore.getState().loadInNewTab(content);
    } catch (e) {
      message.error(String(e));
    } finally {
      hide();
    }
  };

  const executeGitFile = async () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const repoName = parts[3];
    const filePath = parts.slice(4).join(":");
    setCtxMenu(null);
    const hide = message.loading(`Executing ${filePath}…`, 0);
    try {
      await ExecuteGitFile(db, schema, repoName, filePath);
      message.success(`${filePath} executed successfully`);
    } catch (e) {
      message.error(String(e));
    } finally {
      hide();
    }
  };


  // --- Stage file action handlers ---

  const executeStageFile = async () => {
    const k = parseStageOrDbtKey(ctxMenu);
    if (!k) return;
    setCtxMenu(null);
    const hide = message.loading(`Executing ${k.path}…`, 0);
    try {
      await ExecuteStageFile(k.db, k.schema, k.name, k.path);
      message.success(`${k.path} executed successfully`);
    } catch (e) {
      message.error(String(e));
    } finally {
      hide();
    }
  };

  const downloadStageFile = async () => {
    const k = parseStageOrDbtKey(ctxMenu);
    if (!k) return;
    setCtxMenu(null);
    const localPath = await PickDirectory();
    if (!localPath) return;
    const stageRef = `@${quoteIdent(k.db)}.${quoteIdent(k.schema)}.${quoteIdent(k.name)}/${k.path}`;
    const hide = message.loading(`Downloading ${k.path}…`, 0);
    try {
      await DownloadFileFromStage(stageRef, localPath, 4, "");
      message.success(`Downloaded ${k.path} successfully.`);
    } catch (e) {
      message.error(`Failed to download file: ${String(e)}`);
    } finally {
      hide();
    }
  };

  const deleteStageFile = () => {
    const k = parseStageOrDbtKey(ctxMenu);
    if (!k) return;
    const fileKey = ctxMenu!.nodeKey;
    setCtxMenu(null);
    const stageRef = `@${quoteIdent(k.db)}.${quoteIdent(k.schema)}.${quoteIdent(k.name)}/${k.path}`;
    modal.confirm({
      title: "Delete Stage File",
      content: `Are you sure you want to delete ${k.path}?`,
      okText: "Delete",
      okButtonProps: { danger: true },
      onOk: async () => {
        const hide = message.loading(`Deleting ${k.path}…`, 0);
        try {
          await RemoveStageFiles(stageRef, "");
          message.success(`${k.path} deleted.`);
          setTreeData((prev) => removeNode(prev, fileKey));
        } catch (e) {
          message.error(`Failed to delete: ${String(e)}`);
        } finally {
          hide();
        }
      },
    });
  };

  const uploadToStageDir = async () => {
    const k = parseStageOrDbtKey(ctxMenu);
    if (!k) return;
    const nodeKey = ctxMenu!.nodeKey;
    setCtxMenu(null);
    const localPath = await PickOpenFile();
    if (!localPath) return;
    const stageRef = `@${quoteIdent(k.db)}.${quoteIdent(k.schema)}.${quoteIdent(k.name)}/${k.path}`;
    const hide = message.loading(`Uploading to ${k.name}/${k.path}…`, 0);
    try {
      await UploadFileToStage(localPath, stageRef, 4, true, "AUTO_DETECT", true);
      message.success(`Uploaded successfully.`);
    } catch (e) {
      message.error(`Failed to upload file: ${String(e)}`);
      return; // Skip re-fetch on upload failure
    } finally {
      hide();
    }
    // Re-fetch directory contents so the new file appears without collapsing.
    // Separated from the upload try/catch so a re-fetch failure doesn't show
    // the misleading "Failed to upload" message.
    try {
      const entries = await ListStageEntries(k.db, k.schema, k.name, k.path);
      const nodes = buildEntryNodes(k.db, k.schema, k.name, entries ?? [], "stagedir", "stagefile");
      setTreeData((prev) => updateNode(prev, nodeKey, nodes.length ? nodes : [emptyChildNode(nodeKey)]));
    } catch (e) {
      console.error("Failed to refresh directory after upload:", e);
      // Fall back to clearing children so the next expand re-fetches
      setTreeData((prev) => clearNodeChildren(prev, nodeKey));
    }
  };

  // --- DBT Project file action handlers ---

  const fetchGitRepository = async () => {
    if (!ctxMenu) return;
    const parts = ctxMenu.nodeKey.split(":");
    const db = parts[1];
    const schema = parts[2];
    const name = parts[4];
    setCtxMenu(null);
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const sql = `ALTER GIT REPOSITORY ${q(db)}.${q(schema)}.${q(name)} FETCH;`;
    const hide = message.loading("Fetching repository…", 0);
    try {
      const { ExecDDL } = await import("../../../wailsjs/go/app/App");
      await ExecDDL(sql);
      hide();
      message.success("Repository fetched successfully");
    } catch (err) {
      hide();
      message.error(String(err));
    }
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
      case "DYNAMIC TABLE": sql = `DROP DYNAMIC TABLE ${fullName};`; break;
      case "EXTERNAL TABLE": sql = `DROP EXTERNAL TABLE ${fullName};`; break;
      case "MATERIALIZED VIEW": sql = `DROP MATERIALIZED VIEW ${fullName};`; break;
      case "ALERT":       sql = `DROP ALERT ${fullName};`; break;
      case "TAG":         sql = `DROP TAG ${fullName};`; break;
      case "MASKING POLICY": sql = `DROP MASKING POLICY ${fullName};`; break;
      case "NETWORK RULE": sql = `DROP NETWORK RULE ${fullName};`; break;
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

    modal.confirm({
      title: `Drop ${objKind.toLowerCase()} "${name}"?`,
      content: `This will permanently delete ${db}.${schema}.${name}. This action cannot be undone.`,
      okText: "Drop",
      okType: "danger",
      cancelText: "Cancel",
      onOk: async () => {
        try {
          await ExecDDL(sql);
          message.success(`Dropped ${objKind.toLowerCase()} "${name}"`);
          refreshDatabaseByName(db);
        } catch (e) {
          message.error(String(e));
        }
      },
    });
  };

  const deleteTaskGraph = () => {
    if (!ctxMenu) return;
    const { nodeKey } = ctxMenu;
    setCtxMenu(null);
    // key format: obj:DB:SCHEMA:KIND:NAME
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    modal.confirm({
      title: `Delete task graph "${name}"?`,
      content: `This will suspend and permanently drop "${name}" and all its child tasks. This action cannot be undone.`,
      okText: "Delete Graph",
      okType: "danger",
      cancelText: "Cancel",
      onOk: async () => {
        try {
          await DropTaskTree(db, schema, name);
          refreshDatabaseByName(db);
        } catch (e) {
          message.error(String(e));
        }
      },
    });
  };

  const dropDatabaseNode = async () => {
    if (!ctxMenu) return;
    const db = ctxMenu.nodeKey.slice("db:".length);
    setCtxMenu(null);
    let retentionDays = 1;
    try {
      retentionDays = await GetDatabaseRetentionDays(db);
    } catch {
      // fall back to default; non-fatal
    }
    const retentionNote = retentionDays > 0
      ? `This action can be undone within the ${retentionDays}-day Time Travel retention window.`
      : "No Time Travel retention is configured. This action cannot be undone.";
    let dropMode = "CASCADE";
    modal.confirm({
      title: `Drop database "${db}"?`,
      content: (
        <div>
          <p style={{ marginBottom: 8 }}>
            This will permanently drop <strong>{db}</strong> and all its schemas and objects.
          </p>
          <p style={{ marginBottom: 12, color: retentionDays > 0 ? "var(--text-muted)" : "#cf222e" }}>
            {retentionNote}
          </p>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span>Mode:</span>
            <Select
              defaultValue="CASCADE"
              style={{ width: 120 }}
              onChange={(v: string) => { dropMode = v; }}
              options={[
                { value: "CASCADE", label: "CASCADE" },
                { value: "RESTRICT", label: "RESTRICT" },
              ]}
            />
          </div>
        </div>
      ),
      okText: "Drop",
      okType: "danger",
      cancelText: "Cancel",
      onOk: async () => {
        try {
          await DropDatabase(db, dropMode);
          useObjectStore.getState().removeDatabase(db);
          setTreeData((prev) => prev.filter((n) => n.key !== `db:${db}`));
        } catch (e) {
          contextMsg.error(String(e));
        }
      },
    });
  };

  const dropSchemaNode = async () => {
    if (!ctxMenu) return;
    // key format: schema:DB:SCHEMA
    const [, db, schema] = ctxMenu.nodeKey.split(":");
    setCtxMenu(null);
    let retentionDays = 1;
    try {
      retentionDays = await GetSchemaRetentionDays(db, schema);
    } catch {
      // fall back to default; non-fatal
    }
    const retentionNote = retentionDays > 0
      ? `This action can be undone within the ${retentionDays}-day Time Travel retention window.`
      : "No Time Travel retention is configured. This action cannot be undone.";
    let dropMode = "CASCADE";
    modal.confirm({
      title: `Drop schema "${db}.${schema}"?`,
      content: (
        <div>
          <p style={{ marginBottom: 8 }}>
            This will permanently drop <strong>{db}.{schema}</strong> and all its objects.
          </p>
          <p style={{ marginBottom: 12, color: retentionDays > 0 ? "var(--text-muted)" : "#cf222e" }}>
            {retentionNote}
          </p>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span>Mode:</span>
            <Select
              defaultValue="CASCADE"
              style={{ width: 120 }}
              onChange={(v: string) => { dropMode = v; }}
              options={[
                { value: "CASCADE", label: "CASCADE" },
                { value: "RESTRICT", label: "RESTRICT" },
              ]}
            />
          </div>
        </div>
      ),
      okText: "Drop",
      okType: "danger",
      cancelText: "Cancel",
      onOk: async () => {
        try {
          await DropSchema(db, schema, dropMode);
          useObjectStore.getState().removeSchema(db, schema);
          setTreeData((prev) => removeNode(prev, `schema:${db}:${schema}`));
        } catch (e) {
          contextMsg.error(String(e));
        }
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
    setRenameModal({ db, schema, kind: objKind, oldName, newName: oldName, caseSensitive: false });
    setRenameQiic(false);
    GetQuotedIdentifiersIgnoreCase().then((v) => setRenameQiic(v ?? false)).catch(() => {});
  };

  const executeRename = async () => {
    if (!renameModal) return;
    const { db, schema, kind, oldName, newName, caseSensitive } = renameModal;
    const trimmed = newName.trim();
    if (!trimmed || trimmed === oldName) {
      setRenameModal(null);
      return;
    }
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const fullOld = `${q(db)}.${q(schema)}.${q(oldName)}`;
    const fullNew = `${q(db)}.${q(schema)}.${identToken(trimmed, caseSensitive)}`;

    let sql: string;
    switch (kind) {
      case "TABLE":       sql = `ALTER TABLE ${fullOld} RENAME TO ${fullNew};`; break;
      case "VIEW":        sql = `ALTER VIEW ${fullOld} RENAME TO ${fullNew};`; break;
      case "DYNAMIC TABLE": sql = `ALTER DYNAMIC TABLE ${fullOld} RENAME TO ${fullNew};`; break;
      case "MATERIALIZED VIEW": sql = `ALTER MATERIALIZED VIEW ${fullOld} RENAME TO ${fullNew};`; break;
      case "SEQUENCE":    sql = `ALTER SEQUENCE ${fullOld} RENAME TO ${fullNew};`; break;
      case "STAGE":       sql = `ALTER STAGE ${fullOld} RENAME TO ${fullNew};`; break;
      case "STREAM":      sql = `ALTER STREAM ${fullOld} RENAME TO ${fullNew};`; break;
      case "TASK":        sql = `ALTER TASK ${fullOld} RENAME TO ${fullNew};`; break;
      case "FILE FORMAT": sql = `ALTER FILE FORMAT ${fullOld} RENAME TO ${fullNew};`; break;
      case "PIPE":        sql = `ALTER PIPE ${fullOld} RENAME TO ${fullNew};`; break;
      default:            sql = `ALTER ${kind} ${fullOld} RENAME TO ${fullNew};`;
    }

    setRenameModal(null);
    try {
      await ExecDDL(sql);
      message.success(`Renamed "${oldName}" to "${trimmed}"`);
      await refreshDatabaseByName(db, { schema, kind });
    } catch (e) {
      message.error(String(e));
    }
  };

  // ── Column context menu handlers ──────────────────────────────────────────

  // Refresh only the columns of a specific table (surgical — no full DB refresh).
  const refreshTableColumns = (db: string, schema: string, table: string) => {
    // The "TABLE" kind is hardcoded because every caller is a column ALTER
    // action, which Snowflake only permits on tables (never views). If this is
    // ever reused for view columns, the obj: key prefix must use the right kind.
    const tableKey = `obj:${db}:${schema}:TABLE:${table}`;
    setTreeData((prev) => clearNodeChildren(prev, tableKey));
  };

  // Helper to parse col: key → { db, schema, table, column }
  const parseColKey = (key: string) => {
    // key format: col:DB:SCHEMA:TABLE:COLUMN
    const [, db, schema, table, ...colParts] = key.split(":");
    return { db, schema, table, column: colParts.join(":") };
  };

  const openAddColumnModal = () => {
    if (!ctxMenu) return;
    // Can be called from TABLE obj: node or col: node
    if (ctxMenu.nodeType === "obj") {
      const [, db, schema, , ...nameParts] = ctxMenu.nodeKey.split(":");
      setCtxMenu(null);
      setAddColumnModal({ db, schema, table: nameParts.join(":") });
    } else if (ctxMenu.nodeType === "col") {
      const { db, schema, table } = parseColKey(ctxMenu.nodeKey);
      setCtxMenu(null);
      setAddColumnModal({ db, schema, table });
    }
  };

  const dropColumn = () => {
    if (!ctxMenu) return;
    const { db, schema, table, column } = parseColKey(ctxMenu.nodeKey);
    setCtxMenu(null);

    modal.confirm({
      title: `Drop column "${column}"?`,
      content: `This will permanently remove column "${column}" from ${db}.${schema}.${table}. This action cannot be undone.`,
      okText: "Drop",
      okType: "danger",
      cancelText: "Cancel",
      onOk: async () => {
        try {
          await ExecDDL(await BuildDropColumnSql(db, schema, table, column));
          message.success(`Dropped column "${column}"`);
          refreshTableColumns(db, schema, table);
        } catch (e) {
          message.error(String(e));
        }
      },
    });
  };

  const openRenameColumn = () => {
    if (!ctxMenu) return;
    const { db, schema, table, column } = parseColKey(ctxMenu.nodeKey);
    setCtxMenu(null);
    setRenameColumnModal({ db, schema, table, oldName: column, newName: column, caseSensitive: false });
    setRenameColQiic(false);
    GetQuotedIdentifiersIgnoreCase().then((v) => setRenameColQiic(v ?? false)).catch(() => {});
  };

  const executeRenameColumn = async () => {
    if (!renameColumnModal) return;
    const { db, schema, table, oldName, newName, caseSensitive } = renameColumnModal;
    const trimmed = newName.trim();
    if (!trimmed || trimmed === oldName) {
      setRenameColumnModal(null);
      return;
    }
    setRenameColumnModal(null);
    try {
      await ExecDDL(await BuildRenameColumnSql(db, schema, table, oldName, trimmed, caseSensitive));
      message.success(`Renamed column "${oldName}" to "${trimmed}"`);
      refreshTableColumns(db, schema, table);
    } catch (e) {
      message.error(String(e));
    }
  };

  const setColumnNotNull = async () => {
    if (!ctxMenu) return;
    const { db, schema, table, column } = parseColKey(ctxMenu.nodeKey);
    setCtxMenu(null);
    try {
      await ExecDDL(await BuildSetColumnNotNullSql(db, schema, table, column));
      message.success(`Set NOT NULL on "${column}"`);
      refreshTableColumns(db, schema, table);
    } catch (e) {
      message.error(String(e));
    }
  };

  const dropColumnNotNull = async () => {
    if (!ctxMenu) return;
    const { db, schema, table, column } = parseColKey(ctxMenu.nodeKey);
    setCtxMenu(null);
    try {
      await ExecDDL(await BuildDropColumnNotNullSql(db, schema, table, column));
      message.success(`Dropped NOT NULL on "${column}"`);
      refreshTableColumns(db, schema, table);
    } catch (e) {
      message.error(String(e));
    }
  };

  const openColumnComment = () => {
    if (!ctxMenu) return;
    const { db, schema, table, column } = parseColKey(ctxMenu.nodeKey);
    const existing = ctxMenu.colMeta?.comment ?? "";
    setCtxMenu(null);
    setColumnCommentModal({ db, schema, table, column, comment: existing });
  };

  const executeColumnComment = async () => {
    if (!columnCommentModal) return;
    const { db, schema, table, column, comment } = columnCommentModal;
    setColumnCommentModal(null);
    try {
      await ExecDDL(await BuildSetColumnCommentSql(db, schema, table, column, comment));
      message.success(comment.trim() ? `Set comment on "${column}"` : `Removed comment from "${column}"`);
      refreshTableColumns(db, schema, table);
    } catch (e) {
      message.error(String(e));
    }
  };

  const openChangeDataType = () => {
    if (!ctxMenu) return;
    const { db, schema, table, column } = parseColKey(ctxMenu.nodeKey);
    const currentType = ctxMenu.colMeta?.dataType ?? "VARCHAR";
    setCtxMenu(null);
    setChangeDataTypeModal({ db, schema, table, column, dataType: currentType });
  };

  const executeChangeDataType = async () => {
    if (!changeDataTypeModal) return;
    const { db, schema, table, column, dataType } = changeDataTypeModal;
    const trimmed = dataType.trim();
    if (!trimmed) {
      setChangeDataTypeModal(null);
      return;
    }
    setChangeDataTypeModal(null);
    try {
      await ExecDDL(await BuildChangeColumnTypeSql(db, schema, table, column, trimmed));
      message.success(`Changed data type of "${column}" to ${trimmed}`);
      refreshTableColumns(db, schema, table);
    } catch (e) {
      message.error(String(e));
    }
  };

  const insertColumnName = () => {
    if (!ctxMenu) return;
    const { column } = parseColKey(ctxMenu.nodeKey);
    setCtxMenu(null);
    insertAtCursor(quoteIdent(column));
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
      setTaskPropsModal({ db, schema, name, isFinalizer: ctxMenu.isFinalizer });
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

  const selectForInsertTarget = () => {
    if (!ctxMenu) return;
    const { nodeKey } = ctxMenu;
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    setInsertTarget({ db, schema, name });
    message.success(`Selected as insert target: ${name}`);
  };

  const selectAsInsertSource = () => {
    if (!ctxMenu) return;
    const { nodeKey } = ctxMenu;
    const [, db, schema, , ...nameParts] = nodeKey.split(":");
    const name = nameParts.join(":");
    setCtxMenu(null);
    addInsertSource({ db, schema, name });
  };

  const addSelectedAsInsertSources = () => {
    selectedNodeKeys.forEach((key) => {
      const parts  = key.split(":");
      const db     = parts[1];
      const schema = parts[2];
      const name   = parts.slice(4).join(":");
      addInsertSource({ db, schema, name });
    });
    setSelectedNodeKeys(new Set());
    setSelectedNodeArgs(new Map());
    setCtxMenu(null);
  };

  const deleteSelectedObjects = () => {
    const items = Array.from(selectedNodeKeys).map((key) => {
      const parts = key.split(":");
      return {
        key,
        db:   parts[1],
        schema: parts[2],
        kind: parts[3],
        name: parts.slice(4).join(":"),
        args: selectedNodeArgs.get(key) ?? "",
      };
    });
    setSelectedNodeKeys(new Set());
    setSelectedNodeArgs(new Map());
    setCtxMenu(null);

    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const buildDropSql = (db: string, schema: string, kind: string, name: string, args: string): string => {
      const fullName = `${q(db)}.${q(schema)}.${q(name)}`;
      switch (kind) {
        case "TABLE":       return `DROP TABLE ${fullName};`;
        case "VIEW":        return `DROP VIEW ${fullName};`;
        case "DYNAMIC TABLE": return `DROP DYNAMIC TABLE ${fullName};`;
        case "EXTERNAL TABLE": return `DROP EXTERNAL TABLE ${fullName};`;
        case "MATERIALIZED VIEW": return `DROP MATERIALIZED VIEW ${fullName};`;
        case "ALERT":       return `DROP ALERT ${fullName};`;
        case "TAG":         return `DROP TAG ${fullName};`;
        case "MASKING POLICY": return `DROP MASKING POLICY ${fullName};`;
        case "NETWORK RULE": return `DROP NETWORK RULE ${fullName};`;
        case "SEQUENCE":    return `DROP SEQUENCE ${fullName};`;
        case "STAGE":       return `DROP STAGE ${fullName};`;
        case "STREAM":      return `DROP STREAM ${fullName};`;
        case "TASK":        return `DROP TASK ${fullName};`;
        case "FILE FORMAT": return `DROP FILE FORMAT ${fullName};`;
        case "PIPE":        return `DROP PIPE ${fullName};`;
        case "FUNCTION":    return `DROP FUNCTION ${fullName}(${args});`;
        case "PROCEDURE":   return `DROP PROCEDURE ${fullName}(${args});`;
        default:            return `DROP ${kind} ${fullName};`;
      }
    };

    modal.confirm({
      title: `Drop ${items.length} objects?`,
      content: (
        <div>
          <p style={{ marginBottom: 8 }}>This will permanently drop the following objects. This action cannot be undone.</p>
          <ul style={{ margin: 0, paddingLeft: 20, maxHeight: 200, overflowY: "auto" }}>
            {items.map((item) => (
              <li key={item.key} style={{ fontFamily: "monospace", fontSize: 12 }}>
                {item.kind}: {item.db}.{item.schema}.{item.name}
              </li>
            ))}
          </ul>
        </div>
      ),
      okText: "Drop All",
      okType: "danger",
      cancelText: "Cancel",
      onOk: async () => {
        const affectedDbs = new Set<string>();
        let failed = 0;
        for (const item of items) {
          try {
            await ExecDDL(buildDropSql(item.db, item.schema, item.kind, item.name, item.args));
            affectedDbs.add(item.db);
          } catch (e) {
            failed++;
            message.error(`Failed to drop ${item.kind.toLowerCase()} "${item.name}": ${String(e)}`);
          }
        }
        if (failed === 0) {
          message.success(`Dropped ${items.length} object${items.length > 1 ? "s" : ""}`);
        } else if (failed < items.length) {
          message.warning(`Dropped ${items.length - failed} of ${items.length} objects`);
        }
        affectedDbs.forEach((db) => refreshDatabaseByName(db));
      },
    });
  };

  // ── Render ──────────────────────────────────────────────────────────────────

  const menuItem = (label: string, icon: React.ReactNode, onClick: () => void, color?: string, disabled?: boolean, disabledReason?: string) => {
    const el = (
      <div
        style={{
          display: "flex", alignItems: "center", gap: 8, padding: "6px 14px", fontSize: 13,
          cursor: disabled ? "default" : "pointer",
          color: disabled ? "var(--text-muted)" : (color ?? "var(--text)"),
          opacity: disabled ? 0.45 : 1,
        }}
        onMouseEnter={(e) => { if (!disabled) e.currentTarget.style.background = "var(--border)"; }}
        onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
        onClick={disabled ? undefined : onClick}
      >
        {icon}
        {label}
      </div>
    );
    return disabled && disabledReason
      ? <Tooltip title={disabledReason} placement="right" mouseEnterDelay={0.4}>{el}</Tooltip>
      : el;
  };

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
        <Tooltip title="Create database">
          <Button
            type="text"
            size="small"
            icon={<PlusOutlined style={{ fontSize: 11 }} />}
            onClick={() => setCreateDbOpen(true)}
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

      {!treeCollapsed && !isConnected && (
        <div style={{ padding: "24px 16px", textAlign: "center", color: "var(--text-muted)" }}>
          <DisconnectOutlined style={{ fontSize: 24, marginBottom: 8, display: "block", margin: "0 auto 8px" }} />
          <div style={{ fontSize: 13, marginBottom: 12 }}>Not connected to Snowflake</div>
          <Button size="small" type="primary" onClick={() => window.dispatchEvent(new Event("thaw:connect"))}>
            Connect to browse objects
          </Button>
        </div>
      )}

      {!treeCollapsed && isConnected && (
        <div ref={treeScrollRef} style={{ height: treeHeight, overflow: "auto" }}>
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
                  setLoadingTreeNodes(new Set());
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
            <div
              style={{ overflowX: "auto" }}
              onClick={(e) => {
                // Clear multi-selection on any plain (non-modifier) click that
                // bubbles up from the tree. Ctrl/Cmd+clicks on obj nodes
                // call stopPropagation() so they never reach this handler.
                if (!e.ctrlKey && !e.metaKey && selectedNodeKeys.size > 0) {
                  setSelectedNodeKeys(new Set());
                  setSelectedNodeArgs(new Map());
                }
              }}
            >
            <Tree
              className="object-browser-tree"
              treeData={displayData}
              onRightClick={onRightClick as any}
               selectedKeys={Array.from(selectedNodeKeys)}
               multiple
               switcherIcon={(props: any) => {
                 if (props.isLeaf) return null;
                 return props.expanded ? <MinusSquareOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} /> : <PlusSquareOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />;
               }}
               onSelect={(_keys, info) => {
                 const { nativeEvent, node } = info;
                 const key = String(node.key);

                 if (key.startsWith("gitcommit-empty:")) {
                   const parts = key.split(":");
                   setGitCommitFilterModal({ db: parts[1], schema: parts[2], name: parts[3] });
                   return;
                 }

                 if (!key.startsWith("obj:")) return;


                if (nativeEvent.ctrlKey || nativeEvent.metaKey) {
                  nativeEvent.stopPropagation();
                  setSelectedNodeKeys((prev) => {
                    const next = new Set(prev);
                    if (next.has(key)) next.delete(key); else next.add(key);
                    return next;
                  });
                  setSelectedNodeArgs((prev) => {
                    const next = new Map(prev);
                    if (next.has(key)) {
                      next.delete(key);
                    } else {
                      const args = (node as any).arguments ?? "";
                      if (args) next.set(key, args);
                    }
                    return next;
                  });
                }
              }}
              expandedKeys={searchQuery ? searchExpandedKeys : expandedKeys}
              expandAction={"click" as any}
              motion={false as any}
              onExpand={(keys, { expanded, node }) => {
                if (searchQuery) {
                  setSearchExpandedKeys(keys as Key[]);
                  // Trigger lazy load when a node without children is expanded during search.
                  if (expanded && !(node as any).children) {
                    onLoadData(node as unknown as DataNode & { children?: DataNode[] }, setSearchResults);
                  }
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
              }}              showIcon
              blockNode
              style={{ background: "transparent", color: "var(--text)" }}
              titleRender={(node) => {
                const key = String(node.key);
                if (key.startsWith("db:") || key.startsWith("schema:")) {
                  const isLoading = loadingTreeNodes.has(key);
                  if (isLoading) {
                    return (
                      <Space size={4}>
                        <span>{node.title as React.ReactNode}</span>
                        <SyncOutlined spin style={{ fontSize: 10, color: "var(--text-muted)" }} />
                      </Space>
                    );
                  }
                  return node.title as React.ReactNode;
                }
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
                  const isSelected = selectedNodeKeys.has(key);
                  const isInsertSource =
                    (kind === "TABLE" || kind === "VIEW") &&
                    insertTarget !== null &&
                    insertSources.some(
                      (s) => s.db === db && s.schema === schema && s.name === name,
                    );
                  const selectionStyle: React.CSSProperties | undefined =
                    isSelected
                      ? { background: "color-mix(in srgb, var(--accent) 22%, transparent)", borderRadius: 3, outline: "1px solid var(--accent)", outlineOffset: 1 }
                      : isInsertSource
                        ? { borderRadius: 3, outline: "1px dashed var(--accent)", outlineOffset: 1 }
                        : undefined;

                  if (kind === "TABLE" || kind === "VIEW") {
                    const isLoadingObj = loadingTreeNodes.has(key);
                    return (
                      <span
                        draggable
                        onDragStart={(e) => {
                          e.dataTransfer.setData("thaw/table", JSON.stringify({ db, schema, name }));
                          e.dataTransfer.effectAllowed = "copy";
                          e.stopPropagation();
                        }}
                        style={selectionStyle}
                      >
                        {isLoadingObj ? (
                          <Space size={4}>
                            {tooltip}
                            <SyncOutlined spin style={{ fontSize: 10, color: "var(--text-muted)" }} />
                          </Space>
                        ) : tooltip}
                      </span>
                    );
                  }
                  if (kind === "STAGE" || kind === "DBT PROJECT" || kind === "GIT REPOSITORY") {
                    const isLoadingObj = loadingGitNodes.has(key);
                    return (
                      <span style={selectionStyle}>
                        {isLoadingObj ? (
                          <Space size={4}>
                            {tooltip}
                            <SyncOutlined spin style={{ fontSize: 10, color: "var(--text-muted)" }} />
                          </Space>
                        ) : tooltip}
                      </span>
                    );
                  }
                  return (
                    <span style={selectionStyle}>
                      {tooltip}
                    </span>
                  );
                }
                if (
                  key.startsWith("gitbranches:") ||
                  key.startsWith("gittags:") ||
                  key.startsWith("gitcommits:") ||
                  key.startsWith("gitdir:") ||
                  key.startsWith("stagedir:") ||
                  key.startsWith("dbtversion:") ||
                  key.startsWith("dbtdir:")
                ) {
                  const isLoading = loadingGitNodes.has(key);
                  return (
                    <Space size={4}>
                      <span>{node.title as React.ReactNode}</span>
                      {isLoading && <SyncOutlined spin style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                    </Space>
                  );
                }
                return node.title as React.ReactNode;
              }}
            />
            </div>
          )}
        </div>
      )}

      {/* Resize handle */}
      {!treeCollapsed && isConnected && (
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
          {ctxMenu.nodeType === "db" && menuItem("Create Database…", <DatabaseOutlined style={{ fontSize: 12 }} />, () => { setCtxMenu(null); setCreateDbOpen(true); })}
          {ctxMenu.nodeType === "db" && menuItem("Insert Name", <CodeOutlined style={{ fontSize: 12 }} />, insertFullName)}
          {ctxMenu.nodeType === "db" && menuItem("Refresh", <ReloadOutlined style={{ fontSize: 12 }} />, refreshDatabase)}
          {ctxMenu.nodeType === "db" && menuItem("Show Dropped Schemas…", <RollbackOutlined style={{ fontSize: 12 }} />, showDroppedSchemas)}
          {ctxMenu.nodeType === "db" && menuItem("Export DDL", <CloudUploadOutlined style={{ fontSize: 12 }} />, exportDatabase, undefined, !featureFlags.ddlExport, "DDL Export is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "db" && menuItem("ER Diagram…", <ApartmentOutlined style={{ fontSize: 12 }} />, generateERDiagram, undefined, !featureFlags.erDiagramDesigner, "ER Diagram & Designer is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "db" && menuItemSub("Reports", <BarChartOutlined style={{ fontSize: 12 }} />, "db-reports", (
            <>
              {menuItem("Object Summaries", <DashboardOutlined style={{ fontSize: 12 }} />, openObjectSummaries)}
            </>
          ))}
          {ctxMenu.nodeType === "db" && menuItem("Backup Sets…", <SaveOutlined style={{ fontSize: 12 }} />, openBackupSets, undefined, !featureFlags.backupPoliciesAndSets, "Backup Policies & Sets is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "db" && menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, viewProperties)}
          {ctxMenu.nodeType === "db" && <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />}
          {ctxMenu.nodeType === "db" && menuItem("Drop Database…", <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />, dropDatabaseNode, "#f85149")}
          {ctxMenu.nodeType === "schema" && menuItem("Insert Name", <CodeOutlined style={{ fontSize: 12 }} />, insertFullName)}
          {ctxMenu.nodeType === "schema" && menuItemSub("Create Object", <PlusSquareOutlined style={{ fontSize: 12 }} />, "create-object", (
            <>
              {menuItem("Table…", <TableOutlined style={{ fontSize: 12 }} />, openCreateTable)}
              {menuItem("Dynamic Table…", <RetweetOutlined style={{ fontSize: 12 }} />, openCreateDynamicTable)}
              {menuItem("External Table…", <CloudServerOutlined style={{ fontSize: 12 }} />, openCreateExternalTable)}
              {menuItem("Materialized View…", <BlockOutlined style={{ fontSize: 12 }} />, openCreateMaterializedView)}
              {menuItem("Stage…", <InboxOutlined style={{ fontSize: 12 }} />, openCreateStage)}
              {menuItem("File Format…", <FileTextOutlined style={{ fontSize: 12 }} />, openCreateFileFormat, undefined, !featureFlags.fileFormatBuilder, "File Format Builder is disabled. Enable it under View → Enabled Features…")}

              {menuItem("Task…", <ClockCircleOutlined style={{ fontSize: 12 }} />, openCreateTask)}
              {menuItem("Alert…", <AlertOutlined style={{ fontSize: 12 }} />, openCreateAlert)}
              {menuItem("Tag…", <TagsOutlined style={{ fontSize: 12 }} />, openCreateTag)}
              {menuItem("Masking Policy…", <EyeInvisibleOutlined style={{ fontSize: 12 }} />, openCreateMaskingPolicy)}
              {menuItem("Network Rule…", <GlobalOutlined style={{ fontSize: 12 }} />, openCreateNetworkRule)}
              {menuItem("Pipe…", <ApiOutlined style={{ fontSize: 12 }} />, openCreatePipe)}
              {menuItem("Secret…", <KeyOutlined style={{ fontSize: 12 }} />, openCreateSecret)}
              {menuItem("Git Repository…", <BranchesOutlined style={{ fontSize: 12 }} />, openCreateGitRepository)}
              {menuItem("DBT Project…", <BuildOutlined style={{ fontSize: 12 }} />, openCreateDbtProject, undefined, !featureFlags.dbtProjectBrowser, "DBT Project Browser is disabled. Enable it under View → Enabled Features…")}
</>
              ))}          {ctxMenu.nodeType === "schema" && menuItem("Show Dropped Tables…", <RollbackOutlined style={{ fontSize: 12 }} />, showDroppedTables)}
          {ctxMenu.nodeType === "schema" && menuItem("Export Data…", <DownloadOutlined style={{ fontSize: 12 }} />, openSchemaExportModal, undefined, !featureFlags.exportTableData, "Table Data Export is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "schema" && menuItem("Import Data…", <UploadOutlined style={{ fontSize: 12 }} />, openSchemaImportModal, undefined, !featureFlags.tableDataImport, "Table Data Import is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "schema" && menuItem("Backup Sets…", <SaveOutlined style={{ fontSize: 12 }} />, openBackupSets, undefined, !featureFlags.backupPoliciesAndSets, "Backup Policies & Sets is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "schema" && menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, viewProperties)}
          {ctxMenu.nodeType === "schema" && ctxMenu.nodeKey.split(":")[2] !== "INFORMATION_SCHEMA" && <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />}
          {ctxMenu.nodeType === "schema" && ctxMenu.nodeKey.split(":")[2] !== "INFORMATION_SCHEMA" && menuItem("Drop Schema…", <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />, dropSchemaNode, "#f85149")}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "TASK" &&
            menuItem("Task Statuses…", <DashboardOutlined style={{ fontSize: 12 }} />, openTaskStatuses)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "TASK" &&
            menuItem("Create Task…", <ClockCircleOutlined style={{ fontSize: 12 }} />, openCreateTask)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "STAGE" &&
            menuItem("Create Stage…", <InboxOutlined style={{ fontSize: 12 }} />, openCreateStage)}

          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "DYNAMIC TABLE" &&
            menuItem("Create Dynamic Table…", <RetweetOutlined style={{ fontSize: 12 }} />, openCreateDynamicTable)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "EXTERNAL TABLE" &&
            menuItem("Create External Table…", <CloudServerOutlined style={{ fontSize: 12 }} />, openCreateExternalTable)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "MATERIALIZED VIEW" &&
            menuItem("Create Materialized View…", <BlockOutlined style={{ fontSize: 12 }} />, openCreateMaterializedView)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "ALERT" &&
            menuItem("Create Alert…", <AlertOutlined style={{ fontSize: 12 }} />, openCreateAlert)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "TAG" &&
            menuItem("Create Tag…", <TagsOutlined style={{ fontSize: 12 }} />, openCreateTag)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "MASKING POLICY" &&
            menuItem("Create Masking Policy…", <EyeInvisibleOutlined style={{ fontSize: 12 }} />, openCreateMaskingPolicy)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "NETWORK RULE" &&
            menuItem("Create Network Rule…", <GlobalOutlined style={{ fontSize: 12 }} />, openCreateNetworkRule)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "PIPE" &&
            menuItem("Create Pipe…", <ApiOutlined style={{ fontSize: 12 }} />, openCreatePipe)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "FILE FORMAT" &&
            menuItem("Create File Format…", <FileTextOutlined style={{ fontSize: 12 }} />, openCreateFileFormat)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "SECRET" &&
            menuItem("Create Secret…", <KeyOutlined style={{ fontSize: 12 }} />, openCreateSecret)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "FILE FORMAT" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, viewProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "STAGE" &&
            menuItem("Manage Storage Files…", <SearchOutlined style={{ fontSize: 12 }} />, openStageBrowser, undefined, !featureFlags.getCommand && !featureFlags.removeCommand, "Stage browsing is only useful when GET or REMOVE commands are enabled.")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "STAGE" &&
            menuItem("Upload File to Stage…", <UploadOutlined style={{ fontSize: 12 }} />, uploadToStage, undefined, !featureFlags.putCommand, "PUT commands are disabled. Enable them under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "STAGE" &&
            menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, openStageProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "SECRET" &&
            menuItem("Modify…", <EditOutlined style={{ fontSize: 12 }} />, openModifySecret)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DYNAMIC TABLE" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openDynamicTableProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DYNAMIC TABLE" &&
            menuItem("Refresh…", <SyncOutlined style={{ fontSize: 12 }} />, refreshDynamicTable)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DYNAMIC TABLE" &&
            menuItem("Suspend", <PauseCircleOutlined style={{ fontSize: 12 }} />, suspendDynamicTable)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DYNAMIC TABLE" &&
            menuItem("Resume", <PlayCircleOutlined style={{ fontSize: 12 }} />, resumeDynamicTable)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "EXTERNAL TABLE" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openExternalTableProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "EXTERNAL TABLE" &&
            menuItem("Refresh…", <SyncOutlined style={{ fontSize: 12 }} />, refreshExternalTable)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "MATERIALIZED VIEW" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openMaterializedViewProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "MATERIALIZED VIEW" &&
            menuItem("Suspend", <PauseCircleOutlined style={{ fontSize: 12 }} />, suspendMaterializedView)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "MATERIALIZED VIEW" &&
            menuItem("Resume", <PlayCircleOutlined style={{ fontSize: 12 }} />, resumeMaterializedView)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "ALERT" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openAlertProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "ALERT" &&
            menuItem("Suspend", <PauseCircleOutlined style={{ fontSize: 12 }} />, suspendAlert)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "ALERT" &&
            menuItem("Resume", <PlayCircleOutlined style={{ fontSize: 12 }} />, resumeAlert)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "ALERT" &&
            menuItem("Execute", <ThunderboltOutlined style={{ fontSize: 12 }} />, executeAlert)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TAG" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openTagProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "MASKING POLICY" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openMaskingPolicyProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "NETWORK RULE" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openNetworkRuleProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PIPE" &&
            menuItem("Properties…", <FileOutlined style={{ fontSize: 12 }} />, openPipeProperties)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PIPE" &&
            menuItem("Refresh…", <SyncOutlined style={{ fontSize: 12 }} />, openRefreshPipe)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PIPE" &&
            menuItem("View Copy History…", <HistoryOutlined style={{ fontSize: 12 }} />, openPipeCopyHistory)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PIPE" &&
            menuItem("Pause Execution", <PauseCircleOutlined style={{ fontSize: 12 }} />, pausePipeExecution)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PIPE" &&
            menuItem("Resume Execution", <PlayCircleOutlined style={{ fontSize: 12 }} />, unpausePipeExecution)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PIPE" &&
            menuItem("Check Status…", <DashboardOutlined style={{ fontSize: 12 }} />, openPipeStatusModal)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "GIT REPOSITORY" &&
            menuItem("Create Git Repository…", <BranchesOutlined style={{ fontSize: 12 }} />, openCreateGitRepository)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "GIT REPOSITORY" &&
            menuItem("Fetch", <SyncOutlined style={{ fontSize: 12 }} />, fetchGitRepository)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "GIT REPOSITORY" &&
            menuItem("Modify…", <EditOutlined style={{ fontSize: 12 }} />, openModifyGitRepository)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "GIT REPOSITORY" &&
            menuItem("Set Commit Filter…", <EditOutlined style={{ fontSize: 12 }} />, openCommitFilterModal)}
          {ctxMenu.nodeType === "type" && ctxMenu.objKind === "DBT PROJECT" &&
            menuItem("Create DBT Project…", <BuildOutlined style={{ fontSize: 12 }} />, openCreateDbtProject)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DBT PROJECT" &&
            menuItem("Execute…", <PlayCircleOutlined style={{ fontSize: 12 }} />, openExecuteDbtProject)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DBT PROJECT" &&
            menuItem("Show Versions", <EyeOutlined style={{ fontSize: 12 }} />, showDbtProjectVersions)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DBT PROJECT" &&
            menuItem("Describe", <FileOutlined style={{ fontSize: 12 }} />, describeDbtProject)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DBT PROJECT" &&
            menuItem("Modify…", <EditOutlined style={{ fontSize: 12 }} />, openModifyDbtProject)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "DBT PROJECT" &&
            menuItem("Add Version…", <PlusOutlined style={{ fontSize: 12 }} />, openAddDbtProjectVersion)}
          {ctxMenu.nodeType === "gitcommits" &&
            menuItem("Set Commit Filter…", <EditOutlined style={{ fontSize: 12 }} />, openCommitFilterModal)}
          {ctxMenu.nodeType === "gitcommits" &&
            menuItem("Clear Commit Filter", <CloseOutlined style={{ fontSize: 12 }} />, clearGitCommitFilter)}
          {(ctxMenu.nodeKey.startsWith("gitdir:") || ctxMenu.nodeKey.startsWith("gitbranches:") || ctxMenu.nodeKey.startsWith("gittags:") || ctxMenu.nodeKey.startsWith("gitcommits:")) &&
            menuItem("Refresh", <ReloadOutlined style={{ fontSize: 12 }} />, refreshTreeNode)}
          {ctxMenu.nodeType === "gitfile" && menuItem("View Content", <EyeOutlined style={{ fontSize: 12 }} />, viewGitFileContent)}
          {ctxMenu.nodeType === "gitfile" && menuItem("Execute File", <PlayCircleOutlined style={{ fontSize: 12 }} />, executeGitFile)}

          {/* Stage directory/file context menu */}
          {ctxMenu.nodeType === "stagedir" && menuItem("Refresh", <ReloadOutlined style={{ fontSize: 12 }} />, refreshTreeNode)}
          {ctxMenu.nodeType === "stagedir" &&
            menuItem("Upload File…", <UploadOutlined style={{ fontSize: 12 }} />, uploadToStageDir, undefined, !featureFlags.putCommand, "PUT commands are disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "stagefile" && stageFileIsSql &&
            menuItem("Execute File", <PlayCircleOutlined style={{ fontSize: 12 }} />, executeStageFile)}
          {ctxMenu.nodeType === "stagefile" &&
            menuItem("Download…", <DownloadOutlined style={{ fontSize: 12 }} />, downloadStageFile, undefined, !featureFlags.getCommand, "GET commands are disabled. Enable them under View → Enabled Features…")}
          {ctxMenu.nodeType === "stagefile" && <Divider style={{ margin: "4px 0" }} />}
          {ctxMenu.nodeType === "stagefile" &&
            menuItem("Delete…", <DeleteOutlined style={{ fontSize: 12 }} />, deleteStageFile, undefined, !featureFlags.removeCommand, "REMOVE commands are disabled. Enable them under View → Enabled Features…")}

          {/* DBT Project version/directory/file context menu */}
          {(ctxMenu.nodeType === "dbtversion" || ctxMenu.nodeType === "dbtdir") && menuItem("Refresh", <ReloadOutlined style={{ fontSize: 12 }} />, refreshTreeNode)}

          {ctxMenu.nodeType === "obj" && (ctxMenu.objKind === "TABLE" || ctxMenu.objKind === "VIEW" || ctxMenu.objKind === "DYNAMIC TABLE" || ctxMenu.objKind === "EXTERNAL TABLE" || ctxMenu.objKind === "MATERIALIZED VIEW") &&
            menuItem("Select Top 1000 Rows", <TableOutlined style={{ fontSize: 12 }} />, selectTop1000)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Select for Insert Target", <SyncOutlined style={{ fontSize: 12 }} />, selectForInsertTarget, undefined, !featureFlags.insertMapping, "Insert Mapping is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && (ctxMenu.objKind === "TABLE" || ctxMenu.objKind === "VIEW") && insertTarget !== null &&
            menuItem(`Add as Insert Source for ${insertTarget.name}`, <SyncOutlined style={{ fontSize: 12, color: "var(--accent)" }} />, selectAsInsertSource, undefined, !featureFlags.insertMapping, "Insert Mapping is disabled. Enable it under View → Enabled Features…")}
          {selectedNodeKeys.size > 0 && insertTarget !== null &&
            menuItem(
              `Add ${selectedNodeKeys.size} selected as Insert Sources for ${insertTarget.name}`,
              <SyncOutlined style={{ fontSize: 12, color: "var(--accent)" }} />,
              addSelectedAsInsertSources,
              undefined, !featureFlags.insertMapping, "Insert Mapping is disabled. Enable it under View → Enabled Features…",
            )}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Time Travel Query…", <HistoryOutlined style={{ fontSize: 12 }} />, openTimeTravelModal)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Export Data…", <DownloadOutlined style={{ fontSize: 12 }} />, openExportModal, undefined, !featureFlags.exportTableData, "Table Data Export is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Import Data…", <UploadOutlined style={{ fontSize: 12 }} />, openImportModal, undefined, !featureFlags.tableDataImport, "Table Data Import is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Backup Sets…", <SaveOutlined style={{ fontSize: 12 }} />, openBackupSets, undefined, !featureFlags.backupPoliciesAndSets, "Backup Policies & Sets is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TABLE" &&
            menuItem("Add Column…", <PlusOutlined style={{ fontSize: 12 }} />, openAddColumnModal, undefined, !featureFlags.columnManagement, "Column Management is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TASK" &&
            menuItem("Execute Task", <PlayCircleOutlined style={{ fontSize: 12 }} />, executeTask)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TASK" &&
            menuItem("View Task Graph…", <ShareAltOutlined style={{ fontSize: 12 }} />, openTaskGraph, undefined, !featureFlags.taskGraphVisualizer, "Task Graph Visualizer is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TASK" &&
            menuItem("View Run History…", <HistoryOutlined style={{ fontSize: 12 }} />, openTaskHistory)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "TASK" && !ctxMenu.isFinalizer &&
            menuItem("Delete Task Graph…", <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />, deleteTaskGraph, "#f85149")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "PROCEDURE" &&
            menuItem("Call Procedure", <PlayCircleOutlined style={{ fontSize: 12 }} />, callProcedure)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "FUNCTION" &&
            menuItem("Call Function…", <FunctionOutlined style={{ fontSize: 12 }} />, selectFunction)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "NOTEBOOK" &&
            menuItem("Open Notebook", <ExperimentOutlined style={{ fontSize: 12 }} />, openNotebookFromSnowflake, undefined, !featureFlags.snowparkNotebooks, "Snowpark & Notebooks is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "NOTEBOOK" &&
            menuItem("Execute Notebook…", <PlayCircleOutlined style={{ fontSize: 12 }} />, executeNotebook, undefined, !featureFlags.snowparkNotebooks, "Snowpark & Notebooks is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind === "NOTEBOOK" &&
            menuItem("Make Live", <CloudUploadOutlined style={{ fontSize: 12 }} />, makeNotebookLive, undefined, !featureFlags.snowparkNotebooks, "Snowpark & Notebooks is disabled. Enable it under View → Enabled Features…")}
          {ctxMenu.nodeType === "obj" && menuItem("Insert Full Name", <CodeOutlined style={{ fontSize: 12 }} />, insertFullName)}
          {ctxMenu.nodeType === "obj" && menuItem("View Definition", null, viewDefinition)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind !== "PIPE" && ctxMenu.objKind !== "STAGE" && ctxMenu.objKind !== "DYNAMIC TABLE" && ctxMenu.objKind !== "EXTERNAL TABLE" && ctxMenu.objKind !== "MATERIALIZED VIEW" && ctxMenu.objKind !== "ALERT" && ctxMenu.objKind !== "TAG" && ctxMenu.objKind !== "MASKING POLICY" && ctxMenu.objKind !== "NETWORK RULE" && menuItem("Properties", <FileOutlined style={{ fontSize: 12 }} />, viewProperties)}
          {ctxMenu.nodeType === "obj" &&
            menuItem("Select for Comparison", <DiffOutlined style={{ fontSize: 12 }} />, selectObjForComparison)}
          {ctxMenu.nodeType === "obj" && pendingDiff !== null &&
            menuItem(`Compare with: ${pendingDiff.label}`, <DiffOutlined style={{ fontSize: 12, color: "var(--accent)" }} />, compareObjWith)}
          {ctxMenu.nodeType === "obj" &&
            (ctxMenu.objKind === "VIEW" || ctxMenu.objKind === "PROCEDURE" || ctxMenu.objKind === "FUNCTION") &&
            menuItem("View Dependencies…", <ShareAltOutlined style={{ fontSize: 12 }} />, viewDependencies)}
          {ctxMenu.nodeType === "obj" && ctxMenu.objKind !== "FUNCTION" && ctxMenu.objKind !== "PROCEDURE" && ctxMenu.objKind !== "EXTERNAL TABLE" && ctxMenu.objKind !== "ALERT" && ctxMenu.objKind !== "NETWORK RULE" &&
            menuItem("Rename…", <EditOutlined style={{ fontSize: 12 }} />, renameObject)}
          {ctxMenu.nodeType === "obj" && <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />}
          {ctxMenu.nodeType === "obj" && menuItem("Delete…", <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />, deleteObject, "#f85149")}
          {selectedNodeKeys.size >= 2 && (
            <>
              <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              {menuItem(
                `Delete ${selectedNodeKeys.size} selected objects…`,
                <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />,
                deleteSelectedObjects,
                "#f85149",
              )}
            </>
          )}

          {/* Column context menu */}
          {ctxMenu.nodeType === "col" &&
            menuItem("Insert Column Name", <CodeOutlined style={{ fontSize: 12 }} />, insertColumnName)}
          {ctxMenu.nodeType === "col" && ctxMenu.colMeta?.parentKind === "TABLE" && (() => {
            const disabled = !featureFlags.columnManagement;
            const reason = "Column Management is disabled. Enable it under View → Enabled Features…";
            return (
              <>
                <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
                {menuItem("Rename Column…", <EditOutlined style={{ fontSize: 12 }} />, openRenameColumn, undefined, disabled, reason)}
                {menuItem("Change Data Type…", <EditOutlined style={{ fontSize: 12 }} />, openChangeDataType, undefined, disabled, reason)}
                {menuItem("Set Comment…", <EditOutlined style={{ fontSize: 12 }} />, openColumnComment, undefined, disabled, reason)}
                {ctxMenu.colMeta.nullable && !ctxMenu.colMeta.isPrimaryKey &&
                  menuItem("Set NOT NULL", null, setColumnNotNull, undefined, disabled, reason)}
                {!ctxMenu.colMeta.nullable && !ctxMenu.colMeta.isPrimaryKey &&
                  menuItem("Drop NOT NULL", null, dropColumnNotNull, undefined, disabled, reason)}
                <div style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
                {menuItem("Drop Column…", <DeleteOutlined style={{ fontSize: 12, color: "#f85149" }} />, dropColumn, "#f85149", disabled, reason)}
              </>
            );
          })()}
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
          isFinalizer={taskPropsModal.isFinalizer}
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

      {/* Task History modal */}
      {taskHistoryModal && (
        <TaskHistoryModal
          db={taskHistoryModal.db}
          schema={taskHistoryModal.schema}
          name={taskHistoryModal.name}
          isRoot={taskHistoryModal.isRoot}
          onClose={() => setTaskHistoryModal(null)}
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

      {/* Create Database modal */}
      {createDbOpen && (
        <CreateDatabaseModal
          onClose={() => setCreateDbOpen(false)}
          onSuccess={refreshAllDatabases}
        />
      )}

      {/* Create Table modal */}
      {createTableModal && (
        <CreateTableModal
          db={createTableModal.db}
          schema={createTableModal.schema}
          onClose={() => setCreateTableModal(null)}
          onSuccess={() => refreshDatabaseByName(createTableModal.db, { schema: createTableModal.schema, kind: "TABLE" })}
        />
      )}
      {createStageModal && (
        <CreateStageModal
          db={createStageModal.db}
          schema={createStageModal.schema}
          onClose={() => setCreateStageModal(null)}
          onSuccess={() => refreshDatabaseByName(createStageModal.db, { schema: createStageModal.schema, kind: "STAGE" })}
        />
      )}
      {stagePropertiesModal && (
        <StagePropertiesModal
          db={stagePropertiesModal.db}
          schema={stagePropertiesModal.schema}
          name={stagePropertiesModal.name}
          onClose={() => setStagePropertiesModal(null)}
          onSuccess={() => refreshDatabaseByName(stagePropertiesModal.db)}
        />
      )}

      {stageBrowserModal && (
        <StageBrowserModal
          db={stageBrowserModal.db}
          schema={stageBrowserModal.schema}
          name={stageBrowserModal.name}
          onClose={() => setStageBrowserModal(null)}
        />
      )}

      {/* Create File Format modal */}
      {createFileFormatModal && (
        <CreateFileFormatModal
          db={createFileFormatModal.db}
          schema={createFileFormatModal.schema}
          onClose={() => setCreateFileFormatModal(null)}
          onSuccess={() => refreshDatabaseByName(createFileFormatModal.db, { schema: createFileFormatModal.schema, kind: "FILE FORMAT" })}
        />
      )}

      {/* Object Summaries modal */}
      {objectSummariesModal && (
        <ObjectSummariesModal
          db={objectSummariesModal}
          onClose={() => setObjectSummariesModal(null)}
        />
      )}

      {/* Create Task modal */}
      {createTaskModal && (
        <CreateTaskModal
          db={createTaskModal.db}
          schema={createTaskModal.schema}
          onClose={() => setCreateTaskModal(null)}
          onSuccess={() => refreshDatabaseByName(createTaskModal.db, { schema: createTaskModal.schema, kind: "TASK" })}
        />
      )}

      {createSecretModal && (
        <CreateSecretModal
          db={createSecretModal.db}
          schema={createSecretModal.schema}
          onClose={() => setCreateSecretModal(null)}
          onSuccess={() => refreshDatabaseByName(createSecretModal.db, { schema: createSecretModal.schema, kind: "SECRET" })}
        />
      )}

      {modifySecretModal && (
        <ModifySecretModal
          db={modifySecretModal.db}
          schema={modifySecretModal.schema}
          name={modifySecretModal.name}
          onClose={() => setModifySecretModal(null)}
          onSuccess={() => refreshDatabaseByName(modifySecretModal.db)}
        />
      )}

      {createGitRepoModal && (
        <CreateGitRepositoryModal
          db={createGitRepoModal.db}
          schema={createGitRepoModal.schema}
          onClose={() => setCreateGitRepoModal(null)}
          onSuccess={() => refreshDatabaseByName(createGitRepoModal.db, { schema: createGitRepoModal.schema, kind: "GIT REPOSITORY" })}
        />
      )}

      {modifyGitRepoModal && (
        <ModifyGitRepositoryModal
          db={modifyGitRepoModal.db}
          schema={modifyGitRepoModal.schema}
          name={modifyGitRepoModal.name}
          onClose={() => setModifyGitRepoModal(null)}
          onSuccess={() => refreshDatabaseByName(modifyGitRepoModal.db)}
        />
      )}

      {gitCommitFilterModal && (
        <SetGitCommitFilterModal
          db={gitCommitFilterModal.db}
          schema={gitCommitFilterModal.schema}
          name={gitCommitFilterModal.name}
          onClose={() => setGitCommitFilterModal(null)}
          onSuccess={() => setTreeData((prev) => clearNodeChildren(prev, `gitcommits:${gitCommitFilterModal.db}:${gitCommitFilterModal.schema}:${gitCommitFilterModal.name}`))}
        />
      )}

      {createDbtProjectModal && (
        <CreateDbtProjectModal
          db={createDbtProjectModal.db}
          schema={createDbtProjectModal.schema}
          onClose={() => setCreateDbtProjectModal(null)}
          onSuccess={() => refreshDatabaseByName(createDbtProjectModal.db, { schema: createDbtProjectModal.schema, kind: "DBT PROJECT" })}
        />
      )}

      {executeDbtProjectModal && (
        <ExecuteDbtProjectModal
          db={executeDbtProjectModal.db}
          schema={executeDbtProjectModal.schema}
          name={executeDbtProjectModal.name}
          onClose={() => setExecuteDbtProjectModal(null)}
        />
      )}

      {modifyDbtProjectModal && (
        <ModifyDbtProjectModal
          db={modifyDbtProjectModal.db}
          schema={modifyDbtProjectModal.schema}
          name={modifyDbtProjectModal.name}
          onClose={() => setModifyDbtProjectModal(null)}
          onSuccess={() => refreshDatabaseByName(modifyDbtProjectModal.db)}
        />
      )}

      {addDbtProjectVersionModal && (
        <AddDbtProjectVersionModal
          db={addDbtProjectVersionModal.db}
          schema={addDbtProjectVersionModal.schema}
          name={addDbtProjectVersionModal.name}
          onClose={() => setAddDbtProjectVersionModal(null)}
          onSuccess={() => refreshDatabaseByName(addDbtProjectVersionModal.db)}
        />
      )}

      {createDynamicTableModal && (
        <CreateDynamicTableModal
          db={createDynamicTableModal.db}
          schema={createDynamicTableModal.schema}
          onClose={() => setCreateDynamicTableModal(null)}
          onSuccess={() => refreshDatabaseByName(createDynamicTableModal.db, { schema: createDynamicTableModal.schema, kind: "DYNAMIC TABLE" })}
        />
      )}

      {dynamicTablePropsModal && (
        <DynamicTablePropertiesModal
          db={dynamicTablePropsModal.db}
          schema={dynamicTablePropsModal.schema}
          name={dynamicTablePropsModal.name}
          onClose={() => setDynamicTablePropsModal(null)}
        />
      )}

      {createExternalTableModal && (
        <CreateExternalTableModal
          db={createExternalTableModal.db}
          schema={createExternalTableModal.schema}
          onClose={() => setCreateExternalTableModal(null)}
          onSuccess={() => refreshDatabaseByName(createExternalTableModal.db, { schema: createExternalTableModal.schema, kind: "EXTERNAL TABLE" })}
        />
      )}

      {createMaterializedViewModal && (
        <CreateMaterializedViewModal
          db={createMaterializedViewModal.db}
          schema={createMaterializedViewModal.schema}
          onClose={() => setCreateMaterializedViewModal(null)}
          onSuccess={() => refreshDatabaseByName(createMaterializedViewModal.db, { schema: createMaterializedViewModal.schema, kind: "MATERIALIZED VIEW" })}
        />
      )}

      {materializedViewPropsModal && (
        <MaterializedViewPropertiesModal
          db={materializedViewPropsModal.db}
          schema={materializedViewPropsModal.schema}
          name={materializedViewPropsModal.name}
          onClose={() => setMaterializedViewPropsModal(null)}
        />
      )}

      {createAlertModal && (
        <CreateAlertModal
          db={createAlertModal.db}
          schema={createAlertModal.schema}
          onClose={() => setCreateAlertModal(null)}
          onSuccess={() => refreshDatabaseByName(createAlertModal.db, { schema: createAlertModal.schema, kind: "ALERT" })}
        />
      )}

      {alertPropsModal && (
        <AlertPropertiesModal
          db={alertPropsModal.db}
          schema={alertPropsModal.schema}
          name={alertPropsModal.name}
          onClose={() => setAlertPropsModal(null)}
        />
      )}

      {createTagModal && (
        <CreateTagModal
          db={createTagModal.db}
          schema={createTagModal.schema}
          onClose={() => setCreateTagModal(null)}
          onSuccess={() => refreshDatabaseByName(createTagModal.db, { schema: createTagModal.schema, kind: "TAG" })}
        />
      )}

      {tagPropsModal && (
        <TagPropertiesModal
          db={tagPropsModal.db}
          schema={tagPropsModal.schema}
          name={tagPropsModal.name}
          onClose={() => setTagPropsModal(null)}
        />
      )}

      {createMaskingPolicyModal && (
        <CreateMaskingPolicyModal
          db={createMaskingPolicyModal.db}
          schema={createMaskingPolicyModal.schema}
          onClose={() => setCreateMaskingPolicyModal(null)}
          onSuccess={() => refreshDatabaseByName(createMaskingPolicyModal.db, { schema: createMaskingPolicyModal.schema, kind: "MASKING POLICY" })}
        />
      )}

      {maskingPolicyPropsModal && (
        <MaskingPolicyPropertiesModal
          db={maskingPolicyPropsModal.db}
          schema={maskingPolicyPropsModal.schema}
          name={maskingPolicyPropsModal.name}
          onClose={() => setMaskingPolicyPropsModal(null)}
        />
      )}

      {createNetworkRuleModal && (
        <CreateNetworkRuleModal
          db={createNetworkRuleModal.db}
          schema={createNetworkRuleModal.schema}
          onClose={() => setCreateNetworkRuleModal(null)}
          onSuccess={() => refreshDatabaseByName(createNetworkRuleModal.db, { schema: createNetworkRuleModal.schema, kind: "NETWORK RULE" })}
        />
      )}

      {networkRulePropsModal && (
        <NetworkRulePropertiesModal
          db={networkRulePropsModal.db}
          schema={networkRulePropsModal.schema}
          name={networkRulePropsModal.name}
          onClose={() => setNetworkRulePropsModal(null)}
        />
      )}

      {externalTablePropsModal && (
        <ExternalTablePropertiesModal
          db={externalTablePropsModal.db}
          schema={externalTablePropsModal.schema}
          name={externalTablePropsModal.name}
          onClose={() => setExternalTablePropsModal(null)}
        />
      )}

      {createPipeModal && (
        <CreatePipeModal
          db={createPipeModal.db}
          schema={createPipeModal.schema}
          onClose={() => setCreatePipeModal(null)}
          onSuccess={() => refreshDatabaseByName(createPipeModal.db, { schema: createPipeModal.schema, kind: "PIPE" })}
        />
      )}

      {pipePropsModal && (
        <PipePropertiesModal
          db={pipePropsModal.db}
          schema={pipePropsModal.schema}
          name={pipePropsModal.name}
          onClose={() => setPipePropsModal(null)}
        />
      )}

      {refreshPipeModal && (
        <RefreshPipeModal
          db={refreshPipeModal.db}
          schema={refreshPipeModal.schema}
          name={refreshPipeModal.name}
          onClose={() => setRefreshPipeModal(null)}
        />
      )}

      {pipeCopyHistoryModal && (
        <PipeCopyHistoryModal
          db={pipeCopyHistoryModal.db}
          schema={pipeCopyHistoryModal.schema}
          name={pipeCopyHistoryModal.name}
          onClose={() => setPipeCopyHistoryModal(null)}
        />
      )}

      {pipeStatusModal && (
        <PipeStatusModal
          db={pipeStatusModal.db}
          schema={pipeStatusModal.schema}
          name={pipeStatusModal.name}
          onClose={() => setPipeStatusModal(null)}
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
        width={460}
      >
        <div style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>New name</div>
          <Input
            value={renameModal?.newName ?? ""}
            onChange={(e) => setRenameModal((prev) => prev ? { ...prev, newName: e.target.value } : null)}
            onPressEnter={executeRename}
            autoFocus
            style={{ marginBottom: 8 }}
          />
          <ObjectNameCaseControl
            name={renameModal?.newName ?? ""}
            caseSensitive={renameModal?.caseSensitive ?? false}
            onCaseSensitiveChange={(v) => setRenameModal((prev) => prev ? { ...prev, caseSensitive: v } : null)}
            quotedIdentifiersIgnoreCase={renameQiic}
          />
        </div>
      </Modal>

      {/* Add Column modal */}
      {addColumnModal && (
        <AddColumnModal
          db={addColumnModal.db}
          schema={addColumnModal.schema}
          table={addColumnModal.table}
          onClose={() => setAddColumnModal(null)}
          onSuccess={() => refreshTableColumns(addColumnModal.db, addColumnModal.schema, addColumnModal.table)}
        />
      )}

      {/* Rename Column modal */}
      <Modal
        open={renameColumnModal !== null}
        title={renameColumnModal ? `Rename column "${renameColumnModal.oldName}"` : ""}
        onOk={executeRenameColumn}
        onCancel={() => setRenameColumnModal(null)}
        okText="Rename"
        width={460}
      >
        <div style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>New name</div>
          <Input
            value={renameColumnModal?.newName ?? ""}
            onChange={(e) => setRenameColumnModal((prev) => prev ? { ...prev, newName: e.target.value } : null)}
            onPressEnter={executeRenameColumn}
            autoFocus
            style={{ marginBottom: 8 }}
          />
          <ObjectNameCaseControl
            name={renameColumnModal?.newName ?? ""}
            caseSensitive={renameColumnModal?.caseSensitive ?? false}
            onCaseSensitiveChange={(v) => setRenameColumnModal((prev) => prev ? { ...prev, caseSensitive: v } : null)}
            quotedIdentifiersIgnoreCase={renameColQiic}
          />
        </div>
      </Modal>

      {/* Column Comment modal */}
      <Modal
        open={columnCommentModal !== null}
        title={columnCommentModal ? `Set comment on "${columnCommentModal.column}"` : ""}
        onOk={executeColumnComment}
        onCancel={() => setColumnCommentModal(null)}
        okText="Save"
        width={460}
      >
        <div style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>Comment (prefilled with the current value; leave empty to remove)</div>
          <Input.TextArea
            value={columnCommentModal?.comment ?? ""}
            onChange={(e) => setColumnCommentModal((prev) => prev ? { ...prev, comment: e.target.value } : null)}
            rows={3}
            autoFocus
          />
        </div>
      </Modal>

      {/* Change Data Type modal */}
      <Modal
        open={changeDataTypeModal !== null}
        title={changeDataTypeModal ? `Change data type of "${changeDataTypeModal.column}"` : ""}
        onOk={executeChangeDataType}
        onCancel={() => setChangeDataTypeModal(null)}
        okText="Change"
        width={460}
      >
        <div style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>New data type</div>
          <DataTypeSelect
            value={changeDataTypeModal?.dataType ?? "VARCHAR"}
            onChange={(v) => setChangeDataTypeModal((prev) => prev ? { ...prev, dataType: v } : null)}
          />
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8, lineHeight: 1.5 }}>
            Snowflake only permits a narrow set of <code>SET DATA TYPE</code> changes — e.g. increasing a
            VARCHAR length or a NUMBER scale. Arbitrary base-type conversions are rejected by the server.
          </div>
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

      {/* MCP ER Designer — opened by the open_er_designer MCP tool */}
      {mcpErDesigner && (
        <ERDesigner
          database={mcpErDesigner.database}
          initialData={mcpErDesigner.baseline}
          mergedData={mcpErDesigner.merged}
          onClose={() => setMcpErDesigner(null)}
          onSuccess={() => {
            refreshDatabaseByName(mcpErDesigner.database);
            setMcpErDesigner(null);
          }}
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

      <InsertMappingModal />

    </div>
  );
}
