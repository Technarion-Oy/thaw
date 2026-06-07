// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { Modal, Button, Checkbox, message } from "antd";
import { CopyOutlined, EditOutlined } from "@ant-design/icons";
import { ListSchemas } from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { snowflake } from "../../../wailsjs/go/models";
import type { JoinQueryState, JoinPath, DesignerTable } from "./erTypes";
import { buildMermaid } from "./buildMermaid";
import ERDesigner from "./ERDesigner";
import ERCanvas from "./ERCanvas";
import { initFromERData } from "./erCanvasLayout";
import { findJoinPaths, buildJoinState } from "./joinPathfinder";
import JoinQueryPanel, { JoinPathDisambiguation } from "./JoinQueryPanel";
import { useQueryStore } from "../../store/queryStore";

interface Props {
  database: string;
  data: snowflake.ERDiagramData;
  onClose: () => void;
  onDesignerSuccess?: () => void;
}

export default function ERDiagramModal({ database, data, onClose, onDesignerSuccess }: Props) {
  const dataSchemas = useMemo(
    () => [...new Set(data.tables.map((t) => t.schema))],
    [data],
  );

  const [dbSchemas, setDbSchemas] = useState<string[]>([]);
  useEffect(() => {
    ListSchemas(database).then(setDbSchemas).catch(() => {});
  }, [database]);

  // Merge schemas from ER data with all database schemas, excluding INFORMATION_SCHEMA
  const allSchemas = useMemo(
    () => [...new Set([...dataSchemas, ...dbSchemas])]
      .filter((s) => s.toUpperCase() !== "INFORMATION_SCHEMA")
      .sort(),
    [dataSchemas, dbSchemas],
  );

  const [visibleSchemas, setVisibleSchemas] = useState<Set<string>>(new Set(dataSchemas));
  const [designerOpen, setDesignerOpen] = useState(false);
  const [selectedTableIds, setSelectedTableIds] = useState<string[]>([]);

  // Join builder state
  const [joinPanelOpen, setJoinPanelOpen] = useState(false);
  const [joinState, setJoinState] = useState<JoinQueryState | null>(null);
  const [joinPaths, setJoinPaths] = useState<JoinPath[] | null>(null);
  // Captured at the time paths were computed — prevents stale reads if
  // selection changes between opening disambiguation and clicking a path.
  const resolvedTablesRef = useRef<{ schema: string; name: string }[]>([]);

  const loadInNewTab = useQueryStore((s) => s.loadInNewTab);

  const designerTables = useMemo(() => initFromERData(data), [data]);

  // Build a lookup of table columns for the JoinQueryPanel column picker
  const tableColumnsMap = useMemo(() => {
    const m = new Map<string, string[]>();
    for (const t of data.tables) {
      const key = `${t.schema.toUpperCase()}.${t.name.toUpperCase()}`;
      m.set(key, t.columns.map((c) => c.name));
    }
    return m;
  }, [data]);

  // Map designer table UUIDs to schema.name for reverse lookup
  const tableIdToSchemaName = useMemo(() => {
    const m = new Map<string, { schema: string; name: string }>();
    for (const t of designerTables) {
      m.set(t.id, { schema: t.schema, name: t.name });
    }
    return m;
  }, [designerTables]);

  // Pre-built lookup: "SCHEMA.TABLE" (uppercase) → DesignerTable
  // Used by highlightedEdgeIds to avoid O(n) find per FK column.
  const designerTablesByKey = useMemo(() => {
    const m = new Map<string, DesignerTable>();
    for (const t of designerTables) {
      m.set(`${t.schema.toUpperCase()}.${t.name.toUpperCase()}`, t);
    }
    return m;
  }, [designerTables]);

  const toggleSchema = (schema: string) => {
    setVisibleSchemas((prev) => {
      const next = new Set(prev);
      if (next.has(schema)) {
        next.delete(schema);
      } else {
        next.add(schema);
      }
      return next;
    });
  };

  const copyMermaid = () => {
    ClipboardSetText(buildMermaid(designerTables, visibleSchemas));
  };

  // Handle "Build Query" from context menu.
  // Note: once the join panel is open, changing the canvas selection does not
  // update the panel — the user must close and re-trigger "Build Query" with
  // the new selection. This is intentional: live-updating would be disorienting
  // and the disambiguation flow doesn't support incremental changes.
  const handleBuildQuery = useCallback(
    (tableIds: string[]) => {
      const selected = tableIds
        .map((id) => tableIdToSchemaName.get(id))
        .filter((t): t is { schema: string; name: string } => !!t);

      if (selected.length < 2) return;

      const paths = findJoinPaths(selected, data.fks ?? []);
      if (paths.length === 0) {
        void message.warning("Selected tables are not connected by foreign keys");
        return;
      }

      resolvedTablesRef.current = selected;

      if (paths.length === 1) {
        // Single path — open panel directly
        const state = buildJoinState(paths[0], selected, database);
        setJoinState(state);
        setJoinPaths(null);
        setJoinPanelOpen(true);
      } else {
        // Multiple paths — show disambiguation
        setJoinPaths(paths);
        setJoinState(null);
        setJoinPanelOpen(true);
      }
    },
    [data.fks, tableIdToSchemaName, database],
  );

  const handleDisambiguationSelect = useCallback(
    (index: number) => {
      if (!joinPaths) return;
      const state = buildJoinState(joinPaths[index], resolvedTablesRef.current, database);
      setJoinState(state);
      setJoinPaths(null);
    },
    [joinPaths, database],
  );

  const handleCloseJoinPanel = useCallback(() => {
    setJoinPanelOpen(false);
    setJoinState(null);
    setJoinPaths(null);
  }, []);

  // Compute highlighted edge IDs for visual feedback on the canvas.
  // Uses structured fkPairs from JoinEntry to match specific FK columns,
  // avoiding brittle string parsing of ON conditions.
  const highlightedEdgeIds = useMemo(() => {
    if (!joinPanelOpen || !joinState) return undefined;

    // Build a set of normalised FK pair keys from the structured data
    const usedFKPairs = new Set<string>();
    for (const j of joinState.joins) {
      for (const pair of j.fkPairs) {
        const a = `${pair.from.schema}.${pair.from.table}.${pair.from.col}`.toUpperCase();
        const b = `${pair.to.schema}.${pair.to.table}.${pair.to.col}`.toUpperCase();
        const [lo, hi] = [a, b].sort();
        usedFKPairs.add(`${lo}=${hi}`);
      }
    }

    const ids = new Set<string>();
    for (const t of designerTables) {
      for (const c of t.columns) {
        if (!c.fkRef) continue;
        const parts = c.fkRef.split(".");
        if (parts.length !== 3) continue;
        const [refSchema, refTable, refCol] = parts;
        const targetTable = designerTablesByKey.get(
          `${refSchema.toUpperCase()}.${refTable.toUpperCase()}`,
        );
        if (!targetTable) continue;
        const targetCol = targetTable.columns.find(
          (tc) => tc.name.toUpperCase() === refCol.toUpperCase(),
        );
        if (!targetCol) continue;

        const fromRef = `${t.schema.toUpperCase()}.${t.name.toUpperCase()}.${c.name.toUpperCase()}`;
        const toRef = `${refSchema.toUpperCase()}.${refTable.toUpperCase()}.${refCol.toUpperCase()}`;
        const [lo, hi] = [fromRef, toRef].sort();
        if (usedFKPairs.has(`${lo}=${hi}`)) {
          ids.add(`fk-${t.id}-${c.id}-${targetTable.id}-${targetCol.id}`);
        }
      }
    }
    return ids.size > 0 ? ids : undefined;
  }, [joinPanelOpen, joinState, designerTables, designerTablesByKey]);

  // Compute highlighted node IDs (intermediate tables) for visual feedback
  const highlightedNodeIds = useMemo(() => {
    if (!joinPanelOpen || !joinState) return undefined;
    const ids = new Set<string>();
    for (const j of joinState.joins) {
      if (!j.isIntermediate) continue;
      const t = designerTables.find(
        (dt) => dt.schema.toUpperCase() === j.table.schema.toUpperCase() &&
                dt.name.toUpperCase() === j.table.name.toUpperCase(),
      );
      if (t) ids.add(t.id);
    }
    return ids.size > 0 ? ids : undefined;
  }, [joinPanelOpen, joinState, designerTables]);

  return (
    <>
    <Modal
      open
      title={`ER Diagram — ${database}`}
      onCancel={onClose}
      footer={null}
      width="90vw"
      styles={{ body: { padding: 0 } }}
    >
      {/* Toolbar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "8px 16px",
          borderBottom: "1px solid var(--border)",
          flexWrap: "wrap",
        }}
      >
        {/* Schema filter checkboxes */}
        <div style={{ display: "flex", gap: 12, flexWrap: "wrap", flex: 1, alignItems: "center" }}>
          {allSchemas.map((schema) => (
            <Checkbox
              key={schema}
              checked={visibleSchemas.has(schema)}
              onChange={() => toggleSchema(schema)}
            >
              <span style={{ fontSize: 12 }}>{schema}</span>
            </Checkbox>
          ))}
        </div>

        {/* Copy + design controls */}
        <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
          <Button size="small" icon={<CopyOutlined />} onClick={copyMermaid}>
            Copy Mermaid
          </Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => setDesignerOpen(true)}>
            Design Tables…
          </Button>
        </div>
      </div>

      {/* Canvas area — shrinks when join panel is open */}
      <div style={{ height: joinPanelOpen ? "45vh" : "70vh", transition: "height 0.2s ease" }}>
        <ERCanvas
          key={database}
          tables={designerTables}
          mode="readonly"
          database={database}
          visibleSchemas={visibleSchemas}
          selectedTableIds={selectedTableIds}
          onSelectionChange={setSelectedTableIds}
          onBuildQuery={handleBuildQuery}
          highlightedEdgeIds={highlightedEdgeIds}
          highlightedNodeIds={highlightedNodeIds}
        />
      </div>

      {/* Join query builder panel */}
      {joinPanelOpen && joinPaths && !joinState && (
        <JoinPathDisambiguation
          paths={joinPaths}
          onSelect={handleDisambiguationSelect}
          onCancel={handleCloseJoinPanel}
        />
      )}
      {joinPanelOpen && joinState && (
        <div style={{ height: "25vh" }}>
          <JoinQueryPanel
            state={joinState}
            tableColumns={tableColumnsMap}
            onChange={setJoinState}
            onOpenInEditor={(sql) => {
              loadInNewTab(sql);
              onClose();
            }}
            onClose={handleCloseJoinPanel}
          />
        </div>
      )}
    </Modal>

    {designerOpen && (
      <ERDesigner
        database={database}
        initialData={data}
        onClose={() => setDesignerOpen(false)}
        onSuccess={() => {
          setDesignerOpen(false);
          onDesignerSuccess?.();
        }}
      />
    )}
    </>
  );
}
