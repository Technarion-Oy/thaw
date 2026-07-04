// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState } from "react";
import {
  Form, Input, Checkbox, Radio, Space,
  Divider, InputNumber, Button, Table, Tooltip,
} from "antd";
import { TableOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl, { identToken } from "../shared/ObjectNameCaseControl";
import DataTypeSelect from "../shared/DataTypeSelect";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import DefaultFunctionPicker from "../shared/DefaultFunctionPicker";
import { useQuotedIdentifiers, useCreateSubmit } from "../shared/createModalHooks";

interface ColumnDef {
  key: string;
  name: string;
  type: string;
  notNull: boolean;
  primaryKey: boolean;
  unique: boolean;
  defaultValue: string;
  comment: string;
}

interface TableConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  tableType: "PERMANENT" | "TRANSIENT" | "TEMPORARY" | "VOLATILE";
  columns: ColumnDef[];
  clusterBy: string;
  dataRetentionTimeInDays: number | "";
  maxDataExtensionTimeInDays: number | "";
  changeTracking: boolean;
  enableSchemaEvolution: boolean;
  comment: string;
}

const DEFAULTS: TableConfig = {
  name: "",
  caseSensitive: false,
  orReplace: false,
  ifNotExists: false,
  tableType: "PERMANENT",
  columns: [
    { key: "1", name: "ID", type: "NUMBER(38,0)", notNull: true, primaryKey: true, unique: false, defaultValue: "", comment: "" },
  ],
  clusterBy: "",
  dataRetentionTimeInDays: "",
  maxDataExtensionTimeInDays: "",
  changeTracking: false,
  enableSchemaEvolution: false,
  comment: "",
};

function buildSql(db: string, schema: string, cfg: TableConfig): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const sq = (s: string) => "'" + s.replace(/'/g, "''") + "'";

  let createClause = "CREATE";
  if (cfg.orReplace) createClause += " OR REPLACE";

  if (cfg.tableType !== "PERMANENT") {
    createClause += ` ${cfg.tableType}`;
  }

  createClause += " TABLE";
  if (cfg.ifNotExists && !cfg.orReplace) createClause += " IF NOT EXISTS";

  const nameToken = identToken(cfg.name || "table_name", cfg.caseSensitive);
  const lines: string[] = [
    `${createClause} "${esc(db)}"."${esc(schema)}".${nameToken} (`,
  ];

  // Columns
  cfg.columns.forEach((col, idx) => {
    let line = `    "${esc(col.name)}" ${col.type}`;
    // Snowflake column grammar: DEFAULT precedes NOT NULL and the inline
    // PRIMARY KEY / UNIQUE constraints.
    if (col.defaultValue.trim()) line += ` DEFAULT ${col.defaultValue.trim()}`;
    if (col.notNull) line += " NOT NULL";
    if (col.primaryKey) line += " PRIMARY KEY";
    if (col.unique && !col.primaryKey) line += " UNIQUE";
    if (col.comment.trim()) line += ` COMMENT ${sq(col.comment.trim())}`;
    
    lines.push(line + (idx === cfg.columns.length - 1 ? "" : ","));
  });

  lines.push(")");

  // Table options
  if (cfg.clusterBy.trim()) lines.push(`CLUSTER BY (${cfg.clusterBy.trim()})`);
  if (cfg.enableSchemaEvolution) lines.push("ENABLE_SCHEMA_EVOLUTION = TRUE");
  if (cfg.dataRetentionTimeInDays !== "") lines.push(`DATA_RETENTION_TIME_IN_DAYS = ${cfg.dataRetentionTimeInDays}`);
  if (cfg.maxDataExtensionTimeInDays !== "") lines.push(`MAX_DATA_EXTENSION_TIME_IN_DAYS = ${cfg.maxDataExtensionTimeInDays}`);
  if (cfg.changeTracking) lines.push("CHANGE_TRACKING = TRUE");
  if (cfg.comment.trim()) lines.push(`COMMENT = ${sq(cfg.comment.trim())}`);

  return lines.join("\n") + ";";
}

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateTableModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<TableConfig>(DEFAULTS);
  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const { creating, error: createError, setError: setCreateError, submit } = useCreateSubmit();

  const set = <K extends keyof TableConfig>(key: K, value: TableConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const addColumn = () => {
    const newKey = String(Date.now());
    set("columns", [...cfg.columns, { 
      key: newKey, name: "", type: "VARCHAR", notNull: false, 
      primaryKey: false, unique: false, defaultValue: "", comment: "" 
    }]);
  };

  const removeColumn = (key: string) => {
    set("columns", cfg.columns.filter(c => c.key !== key));
  };

  const updateColumn = (key: string, patch: Partial<ColumnDef>) => {
    set("columns", cfg.columns.map(c => c.key === key ? { ...c, ...patch } : c));
  };

  const canSubmit = cfg.name.trim() !== "" && cfg.columns.length > 0 && cfg.columns.every(c => c.name.trim() !== "" && c.type.trim() !== "");

  const handleCreate = () => {
    if (!canSubmit) return;
    const sql = buildSql(db, schema, cfg);
    submit(async () => {
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const preview = buildSql(db, schema, cfg);

  const columns: ColumnsType<ColumnDef> = [
    {
      title: "Name",
      dataIndex: "name",
      width: 150,
      render: (val, record) => (
        <Input 
          size="small" 
          value={val} 
          placeholder="col_name"
          onChange={e => updateColumn(record.key, { name: e.target.value })} 
        />
      )
    },
    {
      title: "Type",
      dataIndex: "type",
      width: 200,
      render: (val, record) => (
        <DataTypeSelect
          value={val}
          onChange={(v) => updateColumn(record.key, { type: v })}
        />
      )
    },
    {
      title: "P",
      dataIndex: "primaryKey",
      width: 40,
      align: "center",
      render: (val, record) => (
        <Tooltip title="Primary Key">
          <Checkbox 
            checked={val} 
            onChange={e => updateColumn(record.key, { primaryKey: e.target.checked })} 
          />
        </Tooltip>
      )
    },
    {
      title: "N",
      dataIndex: "notNull",
      width: 40,
      align: "center",
      render: (val, record) => (
        <Tooltip title="Not Null">
          <Checkbox 
            checked={val} 
            onChange={e => updateColumn(record.key, { notNull: e.target.checked })} 
          />
        </Tooltip>
      )
    },
    {
      title: "Default",
      dataIndex: "defaultValue",
      render: (val, record) => (
        <div style={{ display: "flex", gap: 4 }}>
          <Input
            size="small"
            value={val}
            placeholder="NULL"
            onChange={e => updateColumn(record.key, { defaultValue: e.target.value })}
          />
          <DefaultFunctionPicker onPick={(sql) => updateColumn(record.key, { defaultValue: sql })} />
        </div>
      )
    },
    {
      title: "",
      key: "actions",
      width: 40,
      render: (_, record) => (
        <Button 
          type="text" 
          size="small" 
          danger 
          icon={<DeleteOutlined />} 
          onClick={() => removeColumn(record.key)} 
        />
      )
    }
  ];

  const divider = (label: string) => (
    <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "16px 0 8px" }}>
      {label}
    </Divider>
  );

  return (
    <CreateModalShell
      icon={<TableOutlined />}
      okIcon={<PlusOutlined />}
      title="Create table"
      subtitle={`${db}.${schema}`}
      width={800}
      error={createError}
      errorTitle="Table creation failed"
      onErrorClose={() => setCreateError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleCreate}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Table name"
          placeholder="MY_TABLE"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
        />
        <Form.Item style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Table type" style={{ marginBottom: 12 }}>
          <Radio.Group
            value={cfg.tableType}
            onChange={(e) => set("tableType", e.target.value)}
            size="small"
          >
            <Radio value="PERMANENT">Permanent</Radio>
            <Radio value="TRANSIENT">Transient</Radio>
            <Radio value="TEMPORARY">Temporary</Radio>
            <Radio value="VOLATILE">Volatile</Radio>
          </Radio.Group>
        </Form.Item>

        {divider("Columns")}
        
        <Table
          dataSource={cfg.columns}
          columns={columns}
          pagination={false}
          size="small"
          rowKey="key"
          footer={() => (
            <Button type="dashed" block icon={<PlusOutlined />} onClick={addColumn}>
              Add Column
            </Button>
          )}
          style={{ marginBottom: 16 }}
        />

        {divider("Table Options")}

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item 
            label="Cluster by" 
            style={{ marginBottom: 12 }}
            help="Comma-separated column names or expressions"
          >
            <Input value={cfg.clusterBy} onChange={e => set("clusterBy", e.target.value)} placeholder="col1, TO_DATE(col2)" />
          </Form.Item>
          <Form.Item label="Comment" style={{ marginBottom: 12 }}>
            <Input value={cfg.comment} onChange={e => set("comment", e.target.value)} placeholder="Table comment" />
          </Form.Item>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Data retention (days)" style={{ marginBottom: 12 }}>
            <InputNumber 
              value={cfg.dataRetentionTimeInDays} 
              onChange={v => set("dataRetentionTimeInDays", v ?? "")} 
              min={0} max={90} 
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Max data extension (days)" style={{ marginBottom: 12 }}>
            <InputNumber 
              value={cfg.maxDataExtensionTimeInDays} 
              onChange={v => set("maxDataExtensionTimeInDays", v ?? "")} 
              min={0} max={90} 
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <Space size={24} style={{ marginBottom: 16 }}>
          <Checkbox checked={cfg.changeTracking} onChange={e => set("changeTracking", e.target.checked)}>
            Change tracking
          </Checkbox>
          <Checkbox checked={cfg.enableSchemaEvolution} onChange={e => set("enableSchemaEvolution", e.target.checked)}>
            Enable schema evolution
          </Checkbox>
        </Space>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
