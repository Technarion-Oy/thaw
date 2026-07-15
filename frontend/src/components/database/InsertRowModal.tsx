// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import { Table, Input, Button, Dropdown, Space, Tag, Typography, Tooltip, Spin, Alert } from "antd";
import type { MenuProps } from "antd";
import {
  TableOutlined,
  PlusOutlined,
  DeleteOutlined,
  FunctionOutlined,
  InfoCircleOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import {
  GetTableColumnsWithTypes,
  BuildInsertRowsSql,
  ExecDDL,
} from "../../../wailsjs/go/app/App";
import { snowflake, table } from "../../../wailsjs/go/models";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { DEFAULT_FUNCTIONS } from "../shared/builtinFunctions";
import { useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import InsertCellInput from "./InsertCellInput";
import { parseColumnType } from "./insertCellTypes";

const { Text } = Typography;

// FieldMode is the per-cell value mode. "expr" maps to the backend's
// "expression" mode; the others pass through unchanged.
type FieldMode = "value" | "expr" | "null" | "default";

interface CellState {
  mode: FieldMode;
  value: string;
}

interface Props {
  db: string;
  schema: string;
  table: string;
  onClose: () => void;
  onSuccess?: (rowCount: number) => void;
}

// toBackendMode translates the UI mode to the Go builder's InsertRowValue.Mode.
function toBackendMode(mode: FieldMode): string {
  return mode === "expr" ? "expression" : mode;
}

function emptyCell(): CellState {
  return { mode: "value", value: "" };
}

// cellMenuItems builds the per-cell dropdown: the value modes plus the built-in
// function shortcuts (which set the cell to a raw expression).
function cellMenuItems(nullable: boolean): MenuProps["items"] {
  const items: MenuProps["items"] = [
    { key: "value", label: "Value (literal)" },
    { key: "expr", label: "Expression (raw SQL)" },
  ];
  if (nullable) items.push({ key: "null", label: "NULL" });
  items.push({ key: "default", label: "DEFAULT" });
  items.push({ type: "divider" });
  items.push({
    type: "group",
    label: "Functions",
    children: DEFAULT_FUNCTIONS.map((f) => ({
      key: `fn:${f.sql}`,
      label: (
        <span>
          <code>{f.sql}</code>{" "}
          <span style={{ color: "var(--text-muted)", fontSize: 11 }}>{f.desc}</span>
        </span>
      ),
    })),
  });
  return items;
}

/**
 * Modal that inserts one or more rows into an existing table from a per-column
 * grid form. Columns are enumerated via GetTableColumnsWithTypes; each cell is a
 * literal Value, a raw Expr (populated by the built-in function picker), NULL, or
 * DEFAULT, chosen from the per-cell dropdown. Rows can be added/removed. The
 * generated multi-row INSERT is built in Go (BuildInsertRowsSql) so literal
 * quoting stays consistent, shown live via SqlPreview, and executed with ExecDDL.
 */
export default function InsertRowModal({ db, schema, table: tableName, onClose, onSuccess }: Props) {
  const [columns, setColumns] = useState<snowflake.ColumnInfo[]>([]);
  const [rows, setRows] = useState<CellState[][]>([]);
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
        setRows([list.map(() => emptyCell())]);
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

  const setCell = (rowIdx: number, colIdx: number, patch: Partial<CellState>) =>
    setRows((prev) =>
      prev.map((row, ri) =>
        ri === rowIdx ? row.map((cell, ci) => (ci === colIdx ? { ...cell, ...patch } : cell)) : row,
      ),
    );

  const addRow = () => setRows((prev) => [...prev, columns.map(() => emptyCell())]);
  const removeRow = (rowIdx: number) => setRows((prev) => prev.filter((_r, i) => i !== rowIdx));

  const buildCfg = (): table.InsertRowsConfig =>
    table.InsertRowsConfig.createFrom({
      rows: rows.map((row) => ({
        values: columns.map((c, i) => ({
          column: c.name,
          dataType: c.dataType,
          mode: toBackendMode(row[i]?.mode ?? "value"),
          value: row[i]?.value ?? "",
        })),
      })),
    });

  const preview = useSqlPreview(
    () => (columns.length ? BuildInsertRowsSql(db, schema, tableName, buildCfg()) : Promise.resolve("")),
    [db, schema, tableName, columns, rows],
  );

  const canSubmit = !loading && loadError == null && columns.length > 0 && rows.length > 0;

  const handleSubmit = () => {
    if (!canSubmit) return;
    submit(async () => {
      const sql = await BuildInsertRowsSql(db, schema, tableName, buildCfg());
      await ExecDDL(sql);
      onSuccess?.(rows.length);
      onClose();
    });
  };

  // One grid column per table column, plus a leading row-number column and a
  // trailing delete-row action.
  const tableColumns: ColumnsType<{ index: number }> = [
    {
      title: "#",
      key: "__index",
      width: 44,
      fixed: "left",
      render: (_v, r) => <Text type="secondary" style={{ fontSize: 11 }}>{r.index + 1}</Text>,
    },
    ...columns.map((c, colIdx) => {
      const parsed = parseColumnType(c.dataType);
      return {
      key: c.name,
      width: 210,
      title: (
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
          <div style={{ fontSize: 11, color: "var(--text-muted)", fontWeight: 400 }}>{c.dataType}</div>
        </div>
      ),
      render: (_v: unknown, r: { index: number }) => {
        const cell = rows[r.index]?.[colIdx] ?? emptyCell();
        const placeholder = cell.mode === "null" ? "NULL" : "table default";
        // The per-cell value-mode / function dropdown. In Value mode it is handed
        // to InsertCellInput as `trailing` so it lines up with the value field
        // even when a helper toolbar sits above it; otherwise it sits next to the
        // Expr / NULL / DEFAULT control in a plain Space.Compact row.
        const modeDropdown = (
          <Dropdown
            trigger={["click"]}
            menu={{
              items: cellMenuItems(c.nullable),
              onClick: ({ key }) => {
                if (key.startsWith("fn:")) setCell(r.index, colIdx, { mode: "expr", value: key.slice(3) });
                else setCell(r.index, colIdx, { mode: key as FieldMode });
              },
            }}
          >
            <Button size="small" icon={<FunctionOutlined />} title="Value mode / function" />
          </Dropdown>
        );

        if (cell.mode === "value") {
          return (
            <InsertCellInput
              parsed={parsed}
              value={cell.value}
              onChange={(value) => setCell(r.index, colIdx, { value })}
              trailing={modeDropdown}
            />
          );
        }
        // Raw-SQL box for an Expression, or a disabled placeholder for
        // NULL / DEFAULT (whose value is ignored by the builder).
        const input =
          cell.mode === "expr" ? (
            <Input
              size="small"
              value={cell.value}
              placeholder="SQL expression"
              prefix={<Text type="secondary" style={{ fontSize: 10 }}>ƒx</Text>}
              onChange={(e) => setCell(r.index, colIdx, { value: e.target.value })}
            />
          ) : (
            <Input size="small" value="" disabled placeholder={placeholder} />
          );
        return (
          <Space.Compact style={{ width: "100%" }}>
            {input}
            {modeDropdown}
          </Space.Compact>
        );
      },
      };
    }),
    {
      title: "",
      key: "__actions",
      width: 46,
      fixed: "right",
      render: (_v, r) => (
        <Tooltip title={rows.length <= 1 ? "At least one row is required" : "Remove row"}>
          <Button
            size="small"
            type="text"
            danger
            icon={<DeleteOutlined />}
            disabled={rows.length <= 1}
            onClick={() => removeRow(r.index)}
          />
        </Tooltip>
      ),
    },
  ];

  return (
    <CreateModalShell
      icon={<TableOutlined />}
      okIcon={<PlusOutlined />}
      okText={rows.length > 1 ? `Insert ${rows.length} Rows` : "Insert Row"}
      title="Insert rows"
      subtitle={`${db}.${schema}.${tableName}`}
      width={900}
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
            dataSource={rows.map((_r, index) => ({ key: index, index }))}
            scroll={{ x: "max-content", y: "42vh" }}
            style={{ marginBottom: 12 }}
            footer={() => (
              <Button size="small" type="dashed" icon={<PlusOutlined />} onClick={addRow}>
                Add row
              </Button>
            )}
          />
          <SqlPreview sql={preview} />
        </>
      )}
    </CreateModalShell>
  );
}
