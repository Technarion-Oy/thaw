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

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { Select, Tree, Segmented, Space, Typography, Spin } from "antd";
import {
  FolderOutlined,
  FileOutlined,
  BranchesOutlined,
  TagOutlined,
} from "@ant-design/icons";
import {
  ListDatabases,
  ListSchemas,
  ListObjects,
  ListGitBranches,
  ListGitTags,
  ListGitRepoEntries,
  ListStageEntries,
  ListDbtProjectVersions,
  ListWorkspaces,
  ListWorkspaceEntries,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import type { DataNode, EventDataNode } from "antd/es/tree";
import { quoteIdent } from "../shared/ObjectNameCaseControl";

const { Text } = Typography;

type SourceType = "gitRepo" | "stage" | "dbtProject" | "workspace";

interface Props {
  db: string;
  schema: string;
  value: string;
  onChange: (v: string) => void;
  /** "stage-only" hides dbt project and workspace types (used for AddVersion) */
  mode?: "full" | "stage-only";
}

const TYPE_OPTIONS_FULL: { label: string; value: SourceType }[] = [
  { label: "Git Repository", value: "gitRepo" },
  { label: "Internal Stage", value: "stage" },
  { label: "dbt Project", value: "dbtProject" },
  { label: "Workspace", value: "workspace" },
];

const TYPE_OPTIONS_STAGE_ONLY: { label: string; value: SourceType }[] = [
  { label: "Git Repository", value: "gitRepo" },
  { label: "Internal Stage", value: "stage" },
];

export default function SourceLocationPicker({ db, schema, value, onChange, mode = "full" }: Props) {
  const [sourceType, setSourceType] = useState<SourceType>("gitRepo");

  // Database/schema selectors (for browsing objects outside the target schema)
  const [databases, setDatabases] = useState<string[]>([]);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [pickerDb, setPickerDb] = useState(db);
  const [pickerSchema, setPickerSchema] = useState(schema);
  const [loadingDbs, setLoadingDbs] = useState(false);
  const [loadingSchemas, setLoadingSchemas] = useState(false);

  // Object list
  const [objects, setObjects] = useState<snowflake.SnowflakeObject[]>([]);
  const [loadingObjects, setLoadingObjects] = useState(false);
  const [selectedObject, setSelectedObject] = useState<string | undefined>();

  // Workspace list (account-wide, not schema-scoped)
  const [workspaces, setWorkspaces] = useState<snowflake.WorkspaceInfo[]>([]);
  const [loadingWorkspaces, setLoadingWorkspaces] = useState(false);
  const [workspaceError, setWorkspaceError] = useState("");
  const [selectedWorkspace, setSelectedWorkspace] = useState<snowflake.WorkspaceInfo | undefined>();

  // Git repo ref state
  const [branches, setBranches] = useState<string[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  const [loadingRefs, setLoadingRefs] = useState(false);
  const [selectedRef, setSelectedRef] = useState<string | undefined>();
  const [refType, setRefType] = useState<"branch" | "tag">("branch");

  // dbt project version state
  const [versions, setVersions] = useState<snowflake.DbtProjectVersion[]>([]);
  const [loadingVersions, setLoadingVersions] = useState(false);
  const [selectedVersion, setSelectedVersion] = useState<string | undefined>();

  // Tree state
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loadingTree, setLoadingTree] = useState(false);
  const [treeError, setTreeError] = useState<string>("");
  const [selectedPath, setSelectedPath] = useState<string>("");

  // Track whether onChange came from us (to avoid feedback loops)
  const suppressRef = useRef(false);

  // Stable ref for the onChange callback — prevents the reset effect from
  // re-running when the parent passes a new closure identity on each render.
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  // Load databases on mount
  useEffect(() => {
    setLoadingDbs(true);
    ListDatabases()
      .then((dbs) => setDatabases(dbs ?? []))
      .catch(() => setDatabases([]))
      .finally(() => setLoadingDbs(false));
  }, []);

  // Load schemas when pickerDb changes
  useEffect(() => {
    setSchemas([]);
    if (!pickerDb) return;
    setLoadingSchemas(true);
    ListSchemas(pickerDb)
      .then((s) => setSchemas((s ?? []).filter((n) => n.toUpperCase() !== "INFORMATION_SCHEMA")))
      .catch(() => setSchemas([]))
      .finally(() => setLoadingSchemas(false));
  }, [pickerDb]);

  // Reset pickerDb/pickerSchema to the modal's db/schema when type or parent props change.
  // Also clear the parent value when the source type changes so a stale location string
  // from a different source type doesn't linger in the text input.
  const prevSourceType = useRef(sourceType);
  useEffect(() => {
    if (prevSourceType.current !== sourceType) {
      prevSourceType.current = sourceType;
      suppressRef.current = true;
      onChangeRef.current("");
    }
    setPickerDb(db);
    setPickerSchema(schema);
    setSelectedObject(undefined);
    setSelectedWorkspace(undefined);
    setSelectedRef(undefined);
    setSelectedVersion(undefined);
    setSelectedPath("");
    setTreeData([]);
    setBranches([]);
    setTags([]);
    setVersions([]);
  }, [sourceType, db, schema]);

  // Load objects when type or pickerDb/pickerSchema changes (for non-workspace types)
  useEffect(() => {
    if (sourceType === "workspace") return;
    setSelectedObject(undefined);
    setSelectedRef(undefined);
    setSelectedVersion(undefined);
    setSelectedPath("");
    setTreeData([]);
    setBranches([]);
    setTags([]);
    setVersions([]);

    if (!pickerDb || !pickerSchema) return;

    setLoadingObjects(true);
    ListObjects(pickerDb, pickerSchema)
      .then((objs) => {
        const filtered = (objs ?? []).filter((o) => {
          if (sourceType === "gitRepo") return o.kind === "GIT REPOSITORY";
          if (sourceType === "stage") return o.kind === "STAGE";
          if (sourceType === "dbtProject") return o.kind === "DBT PROJECT";
          return false;
        });
        setObjects(filtered);
      })
      .catch(() => setObjects([]))
      .finally(() => setLoadingObjects(false));
  }, [sourceType, pickerDb, pickerSchema]);

  // Load workspaces when workspace type is selected
  useEffect(() => {
    if (sourceType !== "workspace") return;
    setLoadingWorkspaces(true);
    setWorkspaces([]);
    setWorkspaceError("");
    setSelectedWorkspace(undefined);
    setSelectedPath("");
    setTreeData([]);

    ListWorkspaces()
      .then((ws) => setWorkspaces(ws ?? []))
      .catch((err) => {
        setWorkspaces([]);
        setWorkspaceError(String(err));
      })
      .finally(() => setLoadingWorkspaces(false));
  }, [sourceType]);

  // Load refs when a git repo is selected
  useEffect(() => {
    if (sourceType !== "gitRepo" || !selectedObject) return;
    setLoadingRefs(true);
    setBranches([]);
    setTags([]);
    setSelectedRef(undefined);
    setRefType("branch");

    Promise.all([
      ListGitBranches(pickerDb, pickerSchema, selectedObject).catch(() => []),
      ListGitTags(pickerDb, pickerSchema, selectedObject).catch(() => []),
    ]).then(([b, t]) => {
      setBranches((b ?? []).map((x) => x.name));
      setTags((t ?? []).map((x) => x.name));
      const branchNames = (b ?? []).map((x) => x.name);
      const defaultBranch = branchNames.find((n) => n === "main") ?? branchNames.find((n) => n === "master") ?? branchNames[0];
      if (defaultBranch) {
        setSelectedRef(defaultBranch);
        setRefType("branch");
      }
    }).finally(() => setLoadingRefs(false));
  }, [sourceType, selectedObject, pickerDb, pickerSchema]);

  // Load versions when a dbt project is selected
  useEffect(() => {
    if (sourceType !== "dbtProject" || !selectedObject) return;
    setLoadingVersions(true);
    setVersions([]);
    setSelectedVersion(undefined);

    ListDbtProjectVersions(pickerDb, pickerSchema, selectedObject)
      .then((v) => {
        setVersions(v ?? []);
        const def = (v ?? []).find((x) => x.isDefault);
        if (def) setSelectedVersion(def.version);
      })
      .catch(() => setVersions([]))
      .finally(() => setLoadingVersions(false));
  }, [sourceType, selectedObject, pickerDb, pickerSchema]);

  // selectedRef is intentionally omitted from deps — the ref-specific path prefix
  // (e.g. "branches/main/") is prepended by the caller, not by this callback.
  // When the ref changes, the tree-loading effect rebuilds the tree from scratch.
  const loadEntries = useCallback(async (dirPath: string): Promise<snowflake.GitRepoEntry[]> => {
    if (sourceType === "gitRepo" && selectedObject) {
      return (await ListGitRepoEntries(pickerDb, pickerSchema, selectedObject, dirPath)) ?? [];
    }
    if (sourceType === "stage" && selectedObject) {
      return (await ListStageEntries(pickerDb, pickerSchema, selectedObject, dirPath)) ?? [];
    }
    if (sourceType === "workspace" && selectedWorkspace) {
      return (await ListWorkspaceEntries(
        selectedWorkspace.database, selectedWorkspace.schema,
        selectedWorkspace.name, dirPath,
      )) ?? [];
    }
    return [];
  }, [sourceType, selectedObject, selectedWorkspace, pickerDb, pickerSchema]);

  const entriesToNodes = (entries: snowflake.GitRepoEntry[]): DataNode[] => {
    return entries.map((e) => ({
      key: e.path,
      title: e.name,
      isLeaf: !e.isDir,
      icon: e.isDir ? <FolderOutlined /> : <FileOutlined />,
      children: e.isDir ? [] : undefined,
    }));
  };

  // Load root tree when object (+ ref for git) is ready
  useEffect(() => {
    if (sourceType === "dbtProject") return;

    const hasObject = sourceType === "workspace" ? !!selectedWorkspace : !!selectedObject;
    if (!hasObject) {
      setTreeData([]);
      return;
    }

    if (sourceType === "gitRepo" && !selectedRef) {
      setTreeData([]);
      return;
    }

    setLoadingTree(true);
    setSelectedPath("");
    setTreeError("");

    const dirPath = sourceType === "gitRepo"
      ? `${refType === "branch" ? "branches" : "tags"}/${selectedRef}/`
      : "";

    loadEntries(dirPath).then((entries) => {
      setTreeData(entriesToNodes(entries));
    }).catch((err) => {
      setTreeError(String(err));
    }).finally(() => setLoadingTree(false));
  }, [sourceType, selectedObject, selectedWorkspace, selectedRef, refType, pickerDb, pickerSchema, loadEntries]);

  // Assembled source location string (memoized to avoid recomputation)
  const assembledLocation = useMemo(() => {
    const q = (s: string) => quoteIdent(s);

    if (sourceType === "gitRepo") {
      if (!selectedObject || !selectedRef) return "";
      const refPrefix = refType === "branch" ? "branches" : "tags";
      const path = selectedPath || "";
      return `@${q(pickerDb)}.${q(pickerSchema)}.${q(selectedObject)}/${refPrefix}/${selectedRef}/${path}`;
    }

    if (sourceType === "stage") {
      if (!selectedObject) return "";
      const path = selectedPath || "";
      return `@${q(pickerDb)}.${q(pickerSchema)}.${q(selectedObject)}/${path}`;
    }

    if (sourceType === "dbtProject") {
      if (!selectedObject || !selectedVersion) return "";
      return `snow://dbt/${q(pickerDb)}.${q(pickerSchema)}.${q(selectedObject)}/versions/${selectedVersion}`;
    }

    if (sourceType === "workspace") {
      if (!selectedWorkspace) return "";
      const path = selectedPath || "";
      return `snow://workspace/${encodeURIComponent(selectedWorkspace.name)}/versions/live/${path}`;
    }

    return "";
  }, [sourceType, selectedObject, selectedWorkspace, selectedRef, refType, selectedVersion, selectedPath, pickerDb, pickerSchema]);

  // Emit the assembled source location to the parent.
  // The `assembledLocation &&` guard ensures we never clear the parent's manually-typed
  // value on mount (assembledLocation starts as "" before the user interacts with the picker).
  // suppressRef prevents onChange↔value ping-pong: we set it before calling onChange and
  // clear it at the top of the next effect run. This assumes React processes the parent's
  // state update and re-render (propagating the new `value` prop) before this effect fires
  // again — true for synchronous renders, which is the standard React 18 behavior for
  // state updates triggered from effects.
  useEffect(() => {
    if (suppressRef.current) {
      suppressRef.current = false;
      return;
    }
    if (assembledLocation && assembledLocation !== value) {
      suppressRef.current = true;
      onChangeRef.current(assembledLocation);
    }
  }, [assembledLocation, value]);

  const onLoadData = useCallback(async (node: EventDataNode<DataNode>) => {
    const dirPath = node.key as string;
    try {
      const entries = await loadEntries(dirPath);
      const children = entriesToNodes(entries);
      setTreeData((prev) => updateTreeData(prev, dirPath, children));
    } catch (err) {
      // Mark node as leaf so Ant Design removes the loading spinner
      setTreeData((prev) => updateTreeData(prev, dirPath, [{
        key: `${dirPath}__error`,
        title: `Failed to load: ${err}`,
        isLeaf: true,
        icon: null,
      }]));
    }
  }, [loadEntries]);

  const typeOptions = mode === "stage-only" ? TYPE_OPTIONS_STAGE_ONLY : TYPE_OPTIONS_FULL;

  const refOptions = [
    ...branches.map((b) => ({ label: b, value: `branch:${b}`, icon: <BranchesOutlined /> })),
    ...tags.map((t) => ({ label: t, value: `tag:${t}`, icon: <TagOutlined /> })),
  ];

  const needsDbSchema = sourceType !== "workspace";

  const showTree = sourceType !== "dbtProject" &&
    (sourceType === "workspace" ? !!selectedWorkspace : !!selectedObject) &&
    (sourceType !== "gitRepo" || !!selectedRef);

  return (
    <div style={{
      border: "1px solid var(--border)",
      borderRadius: 6,
      padding: "10px 12px",
      background: "var(--bg)",
    }}>
      <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
        Source Location Picker
      </Text>

      <Space direction="vertical" style={{ width: "100%" }} size={8}>
        {/* Type selector */}
        <Segmented
          options={typeOptions}
          value={sourceType}
          onChange={(v) => setSourceType(v as SourceType)}
          size="small"
          block
        />

        {/* Database / Schema selectors (not needed for workspace) */}
        {needsDbSchema && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
            <Select
              placeholder="Database"
              value={pickerDb || undefined}
              onChange={(v) => { setPickerDb(v); setPickerSchema(""); }}
              loading={loadingDbs}
              showSearch
              optionFilterProp="label"
              size="small"
              options={databases.map((d) => ({ value: d, label: d }))}
              allowClear
            />
            <Select
              placeholder="Schema"
              value={pickerSchema || undefined}
              onChange={setPickerSchema}
              loading={loadingSchemas}
              showSearch
              optionFilterProp="label"
              size="small"
              options={schemas.map((s) => ({ value: s, label: s }))}
              allowClear
              disabled={!pickerDb}
            />
          </div>
        )}

        {/* Object selector (for non-workspace types) */}
        {needsDbSchema && (
          <Select
            placeholder={
              sourceType === "gitRepo" ? "Select git repository..."
                : sourceType === "stage" ? "Select stage..."
                : "Select dbt project..."
            }
            value={selectedObject}
            onChange={setSelectedObject}
            loading={loadingObjects}
            showSearch
            optionFilterProp="label"
            size="small"
            style={{ width: "100%" }}
            options={objects.map((o) => ({ value: o.name, label: o.name }))}
            allowClear
          />
        )}

        {/* Workspace selector */}
        {sourceType === "workspace" && (
          <Select
            placeholder="Select workspace..."
            value={selectedWorkspace?.name}
            onChange={(name) => {
              const ws = workspaces.find((w) => w.name === name);
              setSelectedWorkspace(ws);
              setSelectedPath("");
              setTreeData([]);
            }}
            loading={loadingWorkspaces}
            showSearch
            optionFilterProp="label"
            size="small"
            style={{ width: "100%" }}
            options={workspaces.map((w) => ({ value: w.name, label: w.name }))}
            allowClear
          />
        )}
        {sourceType === "workspace" && workspaceError && (
          <Text type="danger" style={{ fontSize: 11 }}>
            Unable to list workspaces. Your role may lack account-level privileges.
          </Text>
        )}

        {/* Ref selector for git repos */}
        {sourceType === "gitRepo" && selectedObject && (
          <Select
            placeholder="Select branch or tag..."
            value={selectedRef ? `${refType}:${selectedRef}` : undefined}
            onChange={(v) => {
              if (!v) { setSelectedRef(undefined); return; }
              const [refKind, ...rest] = v.split(":");
              setRefType(refKind as "branch" | "tag");
              setSelectedRef(rest.join(":"));
            }}
            loading={loadingRefs}
            showSearch
            optionFilterProp="label"
            size="small"
            style={{ width: "100%" }}
            options={refOptions}
            allowClear
          />
        )}

        {/* Version selector for dbt projects */}
        {sourceType === "dbtProject" && selectedObject && (
          <Select
            placeholder="Select version..."
            value={selectedVersion}
            onChange={setSelectedVersion}
            loading={loadingVersions}
            showSearch
            optionFilterProp="label"
            size="small"
            style={{ width: "100%" }}
            options={versions.map((v) => ({
              value: v.version,
              label: v.alias ? `${v.version} (${v.alias})${v.isDefault ? " [default]" : ""}` : `${v.version}${v.isDefault ? " [default]" : ""}`,
            }))}
            allowClear
          />
        )}

        {/* Path browser tree */}
        {showTree && (
          <div style={{
            maxHeight: 200,
            overflowY: "auto",
            border: "1px solid var(--border)",
            borderRadius: 4,
            padding: 4,
          }}>
            {loadingTree ? (
              <div style={{ textAlign: "center", padding: 16 }}>
                <Spin size="small" />
              </div>
            ) : treeError ? (
              <Text type="danger" style={{ fontSize: 11, padding: 8, display: "block" }}>
                Failed to load entries: {treeError}
              </Text>
            ) : treeData.length > 0 ? (
              <Tree
                treeData={treeData}
                loadData={onLoadData}
                showIcon
                selectable
                selectedKeys={selectedPath ? [selectedPath] : []}
                onSelect={(keys) => {
                  const key = keys[0] as string | undefined;
                  setSelectedPath(key ?? "");
                }}
                style={{ fontSize: 12 }}
              />
            ) : (
              <Text type="secondary" style={{ fontSize: 11, padding: 8, display: "block" }}>
                No entries found
              </Text>
            )}
          </div>
        )}

        {/* Assembled location preview */}
        {assembledLocation && (
          <div style={{
            padding: "4px 8px",
            background: "var(--bg-elevated, var(--bg))",
            borderRadius: 4,
            border: "1px dashed var(--border)",
          }}>
            <Text style={{ fontSize: 11, fontFamily: "'JetBrains Mono', monospace", wordBreak: "break-all" }}>
              {assembledLocation}
            </Text>
          </div>
        )}
      </Space>
    </div>
  );
}

/** Recursively updates tree data by inserting children at the matching key. */
function updateTreeData(list: DataNode[], key: React.Key, children: DataNode[]): DataNode[] {
  return list.map((node) => {
    if (node.key === key) {
      return { ...node, children };
    }
    if (node.children) {
      return { ...node, children: updateTreeData(node.children, key, children) };
    }
    return node;
  });
}
