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

import { useState, useEffect } from "react";
import {
  Form, Input, Select, Space, Typography, Button, Checkbox, Collapse, Alert,
} from "antd";
import { MergeCellsOutlined, PlusOutlined, DeleteOutlined, KeyOutlined } from "@ant-design/icons";
import { BuildCreateHybridTableSql, ExecDDL, HybridIndexColumnOptions } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// UI-side shapes. Index key / INCLUDE columns are selected from the form's
// columns and stored as identifier lists.
type HTColumn = { name: string; type: string; notNull: boolean; primaryKey: boolean; default: string };
type HTIndex = { name: string; columns: string[]; include: string[] };

export default function CreateHybridTableModal({ db, schema, onClose, onSuccess }: Props) {
  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [comment, setComment] = useState("");
  const [columns, setColumns] = useState<HTColumn[]>([
    { name: "ID", type: "NUMBER", notNull: true, primaryKey: true, default: "" },
  ]);
  const [indexes, setIndexes] = useState<HTIndex[]>([]);
  // Columns auto-removed from an index by the reconcile effect (renamed / deleted
  // / retyped to a barred datatype), surfaced as a dismissible hint.
  const [prunedNote, setPrunedNote] = useState<string[]>([]);
  // Index-eligible column names, computed by the backend from the form's columns
  // (the datatype rules live in internal/hybridtable, surfaced via
  // HybridIndexColumnOptions).
  const [keyEligible, setKeyEligible] = useState<string[]>([]);
  const [includeEligible, setIncludeEligible] = useState<string[]>([]);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();

  // The builder takes the Wails-generated config shape; we map the UI state into
  // it only at the IPC boundary.
  const toBuilderCfg = () => ({
    name,
    caseSensitive,
    ifNotExists,
    comment,
    columns: columns.map((c) => ({
      name: c.name,
      type: c.type,
      notNull: c.notNull,
      primaryKey: c.primaryKey,
      default: c.default,
    })),
    indexes: indexes.map((i) => ({
      name: i.name,
      columns: i.columns,
      include: i.include,
    })),
  });

  const preview = useSqlPreview(
    () => BuildCreateHybridTableSql(db, schema, toBuilderCfg() as any),
    [db, schema, name, caseSensitive, ifNotExists, comment, columns, indexes],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  // ── Column editing ─────────────────────────────────────────────────────────
  const addColumn = () =>
    setColumns([...columns, { name: "", type: "VARCHAR", notNull: false, primaryKey: false, default: "" }]);
  const updateColumn = (i: number, patch: Partial<HTColumn>) =>
    setColumns(columns.map((c, idx) => (idx === i ? { ...c, ...patch } : c)));
  const removeColumn = (i: number) => setColumns(columns.filter((_, idx) => idx !== i));

  // ── Index editing ──────────────────────────────────────────────────────────
  const addIndex = () => setIndexes([...indexes, { name: "", columns: [], include: [] }]);
  const updateIndex = (i: number, patch: Partial<HTIndex>) =>
    setIndexes(indexes.map((x, idx) => (idx === i ? { ...x, ...patch } : x)));
  const removeIndex = (i: number) => setIndexes(indexes.filter((_, idx) => idx !== i));

  const namedColumns = columns.filter((c) => c.name.trim() !== "");
  // Index key / INCLUDE column choices come from the backend-computed eligible
  // sets (the datatype rules — semi-structured / geospatial / VECTOR /
  // TIMESTAMP_TZ for keys; semi-structured / geospatial for INCLUDE — live in
  // internal/hybridtable).
  const keyColumnOptions = keyEligible.map((n) => ({ value: n, label: n }));
  const includeColumnOptions = includeEligible.map((n) => ({ value: n, label: n }));

  // Whenever the columns change, recompute eligibility via the backend and
  // reconcile index selections: drop any key / INCLUDE column that no longer
  // exists or is no longer type-eligible (renamed, removed, or retyped to a
  // barred type), so a stale selection can't leak into the generated DDL.
  useEffect(() => {
    let cancelled = false;
    const cols = namedColumns.map((c) => ({ name: c.name, type: c.type }));
    HybridIndexColumnOptions(cols as any).then((opts) => {
      if (cancelled) return;
      const keyCols = opts?.keyColumns ?? [];
      const incCols = opts?.includeColumns ?? [];
      setKeyEligible(keyCols);
      setIncludeEligible(incCols);
      const keyValid = new Set(keyCols);
      const incValid = new Set(incCols);
      const removed = new Set<string>();
      setIndexes((prev) => {
        let changed = false;
        const next = prev.map((idx) => {
          const c2 = idx.columns.filter((c) => keyValid.has(c));
          const i2 = idx.include.filter((c) => incValid.has(c));
          if (c2.length !== idx.columns.length || i2.length !== idx.include.length) {
            changed = true;
            idx.columns.filter((c) => !keyValid.has(c)).forEach((c) => removed.add(c));
            idx.include.filter((c) => !incValid.has(c)).forEach((c) => removed.add(c));
            return { ...idx, columns: c2, include: i2 };
          }
          return idx;
        });
        return changed ? next : prev;
      });
      if (removed.size > 0) setPrunedNote([...removed]);
    }).catch(() => { /* eligibility is advisory; leave prior options on error */ });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [columns]);

  const hasPrimaryKey = namedColumns.some((c) => c.primaryKey);
  // A hybrid table requires a name, at least one column, and a PRIMARY KEY.
  const canSubmit = name.trim().length > 0 && namedColumns.length > 0 && hasPrimaryKey;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      const sql = await BuildCreateHybridTableSql(db, schema, toBuilderCfg() as any);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const indexesBody = (
    <Space direction="vertical" size={8} style={{ width: "100%" }}>
      <Text type="secondary" style={{ fontSize: 12 }}>
        Secondary indexes speed up point lookups on non-primary-key columns. INCLUDE columns are
        returned by the index without an extra table lookup.
      </Text>
      {prunedNote.length > 0 && (
        <Alert
          type="warning"
          showIcon
          closable
          onClose={() => setPrunedNote([])}
          message={`Removed from indexes: ${prunedNote.join(", ")} (column renamed, removed, or no longer an eligible type).`}
        />
      )}
      {indexes.map((idx, i) => {
        // A column cannot be both a key and an INCLUDE column of the same index
        // (Snowflake rejects it as a duplicate), so each dropdown hides what the
        // other has already selected.
        const keyOpts = keyColumnOptions.filter((o) => !idx.include.includes(o.value));
        const includeOpts = includeColumnOptions.filter((o) => !idx.columns.includes(o.value));
        return (
          <Space key={i} align="start" style={{ width: "100%" }} wrap={false}>
            <Input
              size="small"
              placeholder="Index name"
              value={idx.name}
              onChange={(e) => updateIndex(i, { name: e.target.value })}
              style={{ width: 150 }}
            />
            <Select
              size="small"
              mode="multiple"
              placeholder="Key columns"
              value={idx.columns}
              onChange={(v) => updateIndex(i, { columns: v })}
              options={keyOpts}
              style={{ width: 220 }}
              notFoundContent="No eligible columns"
            />
            <Select
              size="small"
              mode="multiple"
              placeholder="Include (optional)"
              value={idx.include}
              onChange={(v) => updateIndex(i, { include: v })}
              options={includeOpts}
              style={{ width: 200 }}
              notFoundContent="No eligible columns"
            />
            <Button size="small" type="text" icon={<DeleteOutlined />} onClick={() => removeIndex(i)} />
          </Space>
        );
      })}
      <Button size="small" icon={<PlusOutlined />} onClick={addIndex}>Add index</Button>
    </Space>
  );

  return (
    <CreateModalShell
      icon={<MergeCellsOutlined />}
      title="Create Hybrid Table"
      subtitle={`${db}.${schema}`}
      width={820}
      error={error}
      errorTitle="Hybrid table creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        {/* Hybrid tables don't support OR REPLACE, so only IF NOT EXISTS is
            offered (no shared NameWithReplaceOptions). */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Hybrid table name" required style={{ marginBottom: 4 }}>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="MY_HYBRID_TABLE"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox checked={ifNotExists} onChange={(e) => setIfNotExists(e.target.checked)}>
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={name}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item
          label="Columns"
          required
          style={{ marginBottom: 8 }}
          help={hasPrimaryKey ? undefined : "Mark at least one column as part of the primary key — hybrid tables require one."}
          validateStatus={namedColumns.length > 0 && !hasPrimaryKey ? "warning" : undefined}
        >
          <Space direction="vertical" size={6} style={{ width: "100%" }}>
            {columns.map((c, i) => (
              <Space key={i} align="center" style={{ width: "100%" }} wrap={false}>
                <Input
                  size="small"
                  placeholder="Column name"
                  value={c.name}
                  onChange={(e) => updateColumn(i, { name: e.target.value })}
                  style={{ width: 170 }}
                />
                <Input
                  size="small"
                  placeholder="Type"
                  value={c.type}
                  onChange={(e) => updateColumn(i, { type: e.target.value })}
                  style={{ width: 130 }}
                />
                <Input
                  size="small"
                  placeholder="Default (optional)"
                  value={c.default}
                  onChange={(e) => updateColumn(i, { default: e.target.value })}
                  style={{ width: 140 }}
                />
                <Checkbox
                  checked={c.notNull || c.primaryKey}
                  disabled={c.primaryKey}
                  onChange={(e) => updateColumn(i, { notNull: e.target.checked })}
                >
                  <span style={{ fontSize: 12 }}>NOT NULL</span>
                </Checkbox>
                {/* PK forces NOT NULL via the derived `checked` above; the
                    column's own notNull value is left untouched so un-ticking PK
                    restores it. */}
                <Checkbox
                  checked={c.primaryKey}
                  onChange={(e) => updateColumn(i, { primaryKey: e.target.checked })}
                >
                  <span style={{ fontSize: 12 }}><KeyOutlined /> PK</span>
                </Checkbox>
                <Button size="small" type="text" icon={<DeleteOutlined />} onClick={() => removeColumn(i)} />
              </Space>
            ))}
            <Button size="small" icon={<PlusOutlined />} onClick={addColumn}>Add column</Button>
          </Space>
        </Form.Item>

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[{ key: "indexes", label: "Secondary indexes", children: indexesBody }]}
        />

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
