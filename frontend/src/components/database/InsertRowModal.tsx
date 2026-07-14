// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import { Table, Input, Segmented, Space, Tag, Typography, Tooltip, Spin, Alert } from "antd";
import { TableOutlined, PlusOutlined, InfoCircleOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import {
  GetTableColumnsWithTypes,
  BuildInsertRowSql,
  ExecDDL,
} from "../../../wailsjs/go/app/App";
import { snowflake, table } from "../../../wailsjs/go/models";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import DefaultFunctionPicker from "../shared/DefaultFunctionPicker";
import { useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

const { Text } = Typography;

// FieldMode is the per-column value mode. "expr" maps to the backend's
// "expression" mode; the others pass through unchanged.
type FieldMode = "value" | "expr" | "null" | "default";

interface FieldState {
  mode: FieldMode;
  value: string;
}

interface Props {
  db: string;
  schema: string;
  table: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// toBackendMode translates the UI mode to the Go builder's InsertRowValue.Mode.
function toBackendMode(mode: FieldMode): string {
  return mode === "expr" ? "expression" : mode;
}

function modeOptions(nullable: boolean) {
  const opts = [
    { label: "Value", value: "value" },
    { label: "Expr", value: "expr" },
  ];
  if (nullable) opts.push({ label: "NULL", value: "null" });
  opts.push({ label: "DEFAULT", value: "default" });
  return opts;
}

/**
 * Modal that inserts a single row into an existing table from a per-column form.
 * Columns are enumerated via GetTableColumnsWithTypes; each field renders as a
 * literal Value, a raw Expr (populated by the built-in function picker), NULL,
 * or DEFAULT. The generated INSERT is built in Go (BuildInsertRowSql) so literal
 * quoting stays consistent with the rest of the app, shown live via SqlPreview,
 * and executed with ExecDDL.
 */
export default function InsertRowModal({ db, schema, table: tableName, onClose, onSuccess }: Props) {
  const [columns, setColumns] = useState<snowflake.ColumnInfo[]>([]);
  const [fields, setFields] = useState<FieldState[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const { creating, error: createError, setError: setCreateError, submit } = useCreateSubmit();

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    GetTableColumnsWithTypes(db, schema, tableName)
      .then((cols) => {
        if (cancelled) return;
        const list = cols ?? [];
        setColumns(list);
        setFields(list.map(() => ({ mode: "value", value: "" })));
        setLoadError(null);
      })
      .catch((err) => {
        if (!cancelled) setLoadError(String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, [db, schema, tableName]);

  const setField = (index: number, patch: Partial<FieldState>) =>
    setFields((prev) => prev.map((f, i) => (i === index ? { ...f, ...patch } : f)));

  const buildCfg = (): table.InsertRowConfig =>
    table.InsertRowConfig.createFrom({
      values: columns.map((c, i) => ({
        column: c.name,
        dataType: c.dataType,
        mode: toBackendMode(fields[i]?.mode ?? "value"),
        value: fields[i]?.value ?? "",
      })),
    });

  const preview = useSqlPreview(
    () => (columns.length ? BuildInsertRowSql(db, schema, tableName, buildCfg()) : Promise.resolve("")),
    [db, schema, tableName, columns, fields],
  );

  const canSubmit = !loading && loadError == null && columns.length > 0;

  const handleSubmit = () => {
    if (!canSubmit) return;
    submit(async () => {
      const sql = await BuildInsertRowSql(db, schema, tableName, buildCfg());
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const tableColumns: ColumnsType<{ index: number }> = [
    {
      title: "Column",
      key: "column",
      width: 220,
      render: (_v, r) => {
        const c = columns[r.index];
        return (
          <div>
            <Space size={4} wrap>
              <Text strong style={{ fontSize: 12 }}>{c.name}</Text>
              {c.isPrimaryKey && <Tag color="gold" style={{ marginInlineEnd: 0, fontSize: 10, lineHeight: "16px" }}>PK</Tag>}
              {!c.nullable && <Tag style={{ marginInlineEnd: 0, fontSize: 10, lineHeight: "16px" }}>NOT NULL</Tag>}
              {c.comment && (
                <Tooltip title={c.comment}>
                  <InfoCircleOutlined style={{ color: "var(--text-muted)", fontSize: 11 }} />
                </Tooltip>
              )}
            </Space>
            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>{c.dataType}</div>
          </div>
        );
      },
    },
    {
      title: "Mode",
      key: "mode",
      width: 160,
      render: (_v, r) => {
        const c = columns[r.index];
        return (
          <Segmented
            size="small"
            value={fields[r.index]?.mode ?? "value"}
            options={modeOptions(c.nullable)}
            onChange={(v) => setField(r.index, { mode: v as FieldMode })}
          />
        );
      },
    },
    {
      title: "Value",
      key: "value",
      render: (_v, r) => {
        const mode = fields[r.index]?.mode ?? "value";
        const disabled = mode === "null" || mode === "default";
        const placeholder =
          mode === "null" ? "NULL" :
          mode === "default" ? "table default" :
          mode === "expr" ? "SQL expression" : "literal value";
        return (
          <Space.Compact style={{ width: "100%" }}>
            <Input
              size="small"
              value={disabled ? "" : (fields[r.index]?.value ?? "")}
              disabled={disabled}
              placeholder={placeholder}
              onChange={(e) => setField(r.index, { value: e.target.value })}
            />
            <DefaultFunctionPicker onPick={(sql) => setField(r.index, { mode: "expr", value: sql })} />
          </Space.Compact>
        );
      },
    },
  ];

  const rows = columns.map((_c, index) => ({ key: index, index }));

  return (
    <CreateModalShell
      icon={<TableOutlined />}
      okIcon={<PlusOutlined />}
      okText="Insert Row"
      title="Insert row"
      subtitle={`${db}.${schema}.${tableName}`}
      width={760}
      error={createError}
      errorTitle="Insert failed"
      onErrorClose={() => setCreateError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleSubmit}
    >
      {loadError && (
        <Alert
          type="error"
          message="Could not load columns"
          description={loadError}
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
      {loading ? (
        <div style={{ textAlign: "center", padding: "32px 0" }}>
          <Spin />
        </div>
      ) : (
        <>
          <Table
            size="small"
            pagination={false}
            columns={tableColumns}
            dataSource={rows}
            scroll={{ y: "45vh" }}
            style={{ marginBottom: 16 }}
          />
          <SqlPreview sql={preview} />
        </>
      )}
    </CreateModalShell>
  );
}
