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

import { useState, useEffect, useCallback, useRef } from "react";
import { Select, Tree, Space, Typography, Spin } from "antd";
import { FolderOutlined, FileOutlined } from "@ant-design/icons";
import {
  ListDatabases, ListUserSchemas, ListStages, ListStageEntries,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import type { DataNode, EventDataNode } from "antd/es/tree";
import { quoteIdent } from "./ObjectNameCaseControl";

const { Text } = Typography;

// Sentinel stage value for the implicit user stage (@~). Stage names can never be
// "~", so it can't collide with a real named stage. When selected, the database/
// schema selectors are irrelevant (the user stage is account-/user-scoped); the
// backend's ListStageEntries detects this value and lists @~ unqualified.
const USER_STAGE = "~";

interface Props {
  /** Default database/schema to browse (the owning object's location). */
  db: string;
  schema: string;
  /**
   * Called when the user selects a stage file. `stage` is the fully-qualified,
   * quoted stage identifier (without the leading @, e.g. `"DB"."SC"."STG"`) and
   * `file` is the path within the stage. Selecting a folder is ignored.
   */
  onPick: (stage: string, file: string) => void;
  /** Heading shown above the picker. Defaults to a generic instruction. */
  label?: string;
  /**
   * Offer the implicit user stage (@~) as a selectable entry. Off by default:
   * Service / Streamlit require a *named* internal stage (their ROOT_LOCATION /
   * spec can't live in @~), so only the Model flow opts in.
   */
  allowUserStage?: boolean;
}

// A reusable internal-stage file browser. External stages are filtered out
// because every consumer (services, streamlits, models, …) references files in
// an **internal** named stage. Lives in components/shared/ so all object-type
// create modals can reuse one implementation instead of cloning a tree browser.
export default function StageFilePicker({ db, schema, onPick, label, allowUserStage }: Props) {
  const [databases, setDatabases] = useState<string[]>([]);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [pickerDb, setPickerDb] = useState(db);
  const [pickerSchema, setPickerSchema] = useState(schema);
  const [loadingDbs, setLoadingDbs] = useState(false);
  const [loadingSchemas, setLoadingSchemas] = useState(false);

  const [stages, setStages] = useState<snowflake.StageSummary[]>([]);
  const [loadingStages, setLoadingStages] = useState(false);
  const [selectedStage, setSelectedStage] = useState<string | undefined>();

  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loadingTree, setLoadingTree] = useState(false);
  const [treeError, setTreeError] = useState("");
  const [selectedPath, setSelectedPath] = useState("");

  // Load databases on mount.
  useEffect(() => {
    setLoadingDbs(true);
    ListDatabases()
      .then((dbs) => setDatabases(dbs ?? []))
      .catch(() => setDatabases([]))
      .finally(() => setLoadingDbs(false));
  }, []);

  // Load schemas when the picker database changes.
  useEffect(() => {
    setSchemas([]);
    if (!pickerDb) return;
    setLoadingSchemas(true);
    ListUserSchemas(pickerDb)
      .then((s) => setSchemas((s ?? []).filter((n) => n.toUpperCase() !== "INFORMATION_SCHEMA")))
      .catch(() => setSchemas([]))
      .finally(() => setLoadingSchemas(false));
  }, [pickerDb]);

  // Mirror the current selection into a ref so the db/schema effect can read it
  // without depending on it (which would re-fetch the stage list on every pick).
  const selectedStageRef = useRef(selectedStage);
  useEffect(() => { selectedStageRef.current = selectedStage; }, [selectedStage]);

  // Reload the named-stage list when db/schema changes. The user stage (@~) is
  // not db/schema-scoped, so leave it selected (and its tree intact) when chosen.
  useEffect(() => {
    setStages([]);
    if (selectedStageRef.current !== USER_STAGE) {
      setSelectedStage(undefined);
      setTreeData([]);
      setSelectedPath("");
    }
    if (!pickerDb || !pickerSchema) return;
    setLoadingStages(true);
    ListStages(pickerDb, pickerSchema)
      // Keep all internal stages: SHOW STAGES reports the type as "INTERNAL" but
      // also encryption-qualified variants like "INTERNAL NO CSE", so match by
      // prefix rather than exact equality (external stages report "EXTERNAL").
      .then((all) => setStages((all ?? []).filter((s) => (s.type ?? "").toUpperCase().startsWith("INTERNAL"))))
      .catch(() => setStages([]))
      .finally(() => setLoadingStages(false));
  }, [pickerDb, pickerSchema]);

  const loadEntries = useCallback(
    async (dirPath: string): Promise<snowflake.GitRepoEntry[]> => {
      if (!selectedStage) return [];
      return (await ListStageEntries(pickerDb, pickerSchema, selectedStage, dirPath)) ?? [];
    },
    [selectedStage, pickerDb, pickerSchema],
  );

  const entriesToNodes = (entries: snowflake.GitRepoEntry[]): DataNode[] =>
    entries.map((e) => ({
      key: e.path,
      title: e.name,
      isLeaf: !e.isDir,
      icon: e.isDir ? <FolderOutlined /> : <FileOutlined />,
      children: e.isDir ? [] : undefined,
    }));

  // Load the root tree when a stage is selected.
  useEffect(() => {
    if (!selectedStage) {
      setTreeData([]);
      return;
    }
    setLoadingTree(true);
    setSelectedPath("");
    setTreeError("");
    loadEntries("")
      .then((entries) => setTreeData(entriesToNodes(entries)))
      .catch((err) => setTreeError(String(err)))
      .finally(() => setLoadingTree(false));
  }, [selectedStage, loadEntries]);

  const onLoadData = useCallback(
    async (node: EventDataNode<DataNode>) => {
      const dirPath = node.key as string;
      try {
        const children = entriesToNodes(await loadEntries(dirPath));
        setTreeData((prev) => updateTreeData(prev, dirPath, children));
      } catch (err) {
        setTreeData((prev) => updateTreeData(prev, dirPath, [{
          key: `${dirPath}__error`, title: `Failed to load: ${err}`, isLeaf: true, icon: null,
        }]));
      }
    },
    [loadEntries],
  );

  return (
    <div style={{ border: "1px solid var(--border)", borderRadius: 6, padding: "10px 12px", background: "var(--bg)" }}>
      <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
        {label ?? "Browse internal stage — select a file"}
      </Text>
      <Space direction="vertical" style={{ width: "100%" }} size={8}>
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

        <Select
          placeholder={loadingStages ? "Loading stages…" : "Select internal stage…"}
          value={selectedStage}
          onChange={setSelectedStage}
          loading={loadingStages}
          showSearch
          optionFilterProp="label"
          size="small"
          style={{ width: "100%" }}
          options={[
            // The user stage (when allowed) isn't tied to db/schema.
            ...(allowUserStage ? [{ value: USER_STAGE, label: "User Stage (@~)" }] : []),
            ...stages.map((s) => ({ value: s.name, label: s.name })),
          ]}
          notFoundContent={loadingStages ? "Loading…" : "No internal stages found"}
          allowClear
        />

        {selectedStage && (
          <div style={{ maxHeight: 200, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 4, padding: 4 }}>
            {loadingTree ? (
              <div style={{ textAlign: "center", padding: 16 }}><Spin size="small" /></div>
            ) : treeError ? (
              <Text type="danger" style={{ fontSize: 11, padding: 8, display: "block" }}>Failed to load entries: {treeError}</Text>
            ) : treeData.length > 0 ? (
              <Tree
                treeData={treeData}
                loadData={onLoadData}
                showIcon
                selectable
                selectedKeys={selectedPath ? [selectedPath] : []}
                onSelect={(keys, info) => {
                  const key = keys[0] as string | undefined;
                  // Only files are valid targets; ignore folder clicks.
                  if (!key || (info.node as DataNode).isLeaf === false) return;
                  setSelectedPath(key);
                  // The user stage is referenced bare as @~; named stages are
                  // fully qualified. Consumers always prepend the leading @.
                  const qualified = selectedStage === USER_STAGE
                    ? USER_STAGE
                    : `${quoteIdent(pickerDb)}.${quoteIdent(pickerSchema)}.${quoteIdent(selectedStage)}`;
                  onPick(qualified, key);
                }}
                style={{ fontSize: 12 }}
              />
            ) : (
              <Text type="secondary" style={{ fontSize: 11, padding: 8, display: "block" }}>No entries found</Text>
            )}
          </div>
        )}
      </Space>
    </div>
  );
}

/** Recursively updates tree data by inserting children at the matching key. */
function updateTreeData(list: DataNode[], key: React.Key, children: DataNode[]): DataNode[] {
  return list.map((node) => {
    if (node.key === key) return { ...node, children };
    if (node.children) return { ...node, children: updateTreeData(node.children, key, children) };
    return node;
  });
}
