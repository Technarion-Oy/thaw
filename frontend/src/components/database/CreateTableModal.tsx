// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Select, Checkbox, Radio, Space,
  Typography, Divider, InputNumber, Button, Table, Tooltip, Alert,
} from "antd";
import { TableOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { ExecDDL, GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl, { identToken } from "../shared/ObjectNameCaseControl";

const { Text } = Typography;

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
    if (col.notNull) line += " NOT NULL";
    if (col.primaryKey) line += " PRIMARY KEY";
    if (col.unique && !col.primaryKey) line += " UNIQUE";
    if (col.defaultValue.trim()) line += ` DEFAULT ${col.defaultValue.trim()}`;
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
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);

  useEffect(() => {
    Promise.resolve()
      .then(() => GetQuotedIdentifiersIgnoreCase())
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
  }, []);

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

  const handleCreate = async () => {
    if (!canSubmit) return;
    const sql = buildSql(db, schema, cfg);
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
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
      width: 150,
      render: (val, record) => (
        <Select
          size="small"
          showSearch
          value={val}
          style={{ width: "100%" }}
          onChange={v => updateColumn(record.key, { type: v })}
          options={[
            { value: "NUMBER(38,0)", label: "NUMBER(38,0)" },
            { value: "VARCHAR", label: "VARCHAR" },
            { value: "VARCHAR(16777216)", label: "VARCHAR(16777216)" },
            { value: "BOOLEAN", label: "BOOLEAN" },
            { value: "DATE", label: "DATE" },
            { value: "TIMESTAMP_NTZ", label: "TIMESTAMP_NTZ" },
            { value: "TIMESTAMP_TZ", label: "TIMESTAMP_TZ" },
            { value: "VARIANT", label: "VARIANT" },
            { value: "OBJECT", label: "OBJECT" },
            { value: "ARRAY", label: "ARRAY" },
            { value: "FLOAT", label: "FLOAT" },
          ]}
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
        <Input 
          size="small" 
          value={val} 
          placeholder="NULL"
          onChange={e => updateColumn(record.key, { defaultValue: e.target.value })} 
        />
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
    <Modal
      open
      title={
        <Space size={6}>
          <TableOutlined style={{ color: "var(--link)" }} />
          <span>Create table</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={creating}>Cancel</Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreate}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={800}
      styles={{ body: { paddingTop: 16, maxHeight: "80vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Table creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Table name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_TABLE"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Space direction="vertical" size={4}>
              <Checkbox
                checked={cfg.orReplace}
                onChange={(e) => {
                  set("orReplace", e.target.checked);
                  if (e.target.checked) set("ifNotExists", false);
                }}
              >
                OR REPLACE
              </Checkbox>
              <Checkbox
                checked={cfg.ifNotExists}
                disabled={cfg.orReplace}
                onChange={(e) => set("ifNotExists", e.target.checked)}
              >
                IF NOT EXISTS
              </Checkbox>
            </Space>
          </Form.Item>
        </div>
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

        <div
          style={{
            padding: "10px 12px",
            background: "var(--bg)",
            borderRadius: 6,
            border: "1px solid var(--border)",
            marginTop: 4,
          }}
        >
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            SQL Preview
          </Text>
          <pre
            style={{
              margin: 0,
              color: "var(--text)",
              fontSize: 11,
              fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {preview}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
