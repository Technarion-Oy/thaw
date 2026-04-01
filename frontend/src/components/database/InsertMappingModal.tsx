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
import { Modal, Table, Select, Input, Button, Space, Typography, Tag, Alert, message, Switch } from "antd";
import { ArrowRightOutlined, SyncOutlined } from "@ant-design/icons";
import { GetTableColumnsWithTypes } from "../../../wailsjs/go/main/App";
import { snowflake } from "../../../wailsjs/go/models";
import { useInsertMappingStore } from "../../store/insertMappingStore";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;
const { Option } = Select;

// Identifiers that are structurally safe (no special chars / spaces)
const SAFE_IDENT = /^[a-zA-Z_][a-zA-Z0-9_$]*$/;

// Snowflake reserved keywords — must always be double-quoted when used as identifiers
// Source: https://docs.snowflake.com/en/sql-reference/reserved-keywords
const SNOWFLAKE_RESERVED = new Set([
  "ACCOUNT", "ALL", "ALTER", "AND", "ANY", "AS",
  "BETWEEN", "BY",
  "CASE", "CAST", "CHECK", "COLUMN", "CONNECT", "CONNECTION", "CONSTRAINT",
  "CREATE", "CROSS", "CURRENT", "CURRENT_DATE", "CURRENT_TIME",
  "CURRENT_TIMESTAMP", "CURRENT_USER",
  "DATABASE", "DELETE", "DISTINCT", "DROP",
  "ELSE", "END", "EXISTS",
  "FAIL", "FALSE", "FOLLOWING", "FOR", "FOREIGN", "FROM", "FULL",
  "GRANT", "GROUP", "GSCLUSTER",
  "HAVING",
  "ILIKE", "IN", "INCREMENT", "INNER", "INSERT", "INTERSECT", "INTO", "IS", "ISSUE",
  "JOIN",
  "LATERAL", "LEFT", "LIKE", "LIMIT", "LOCALTIME", "LOCALTIMESTAMP",
  "MAX", "MIN",
  "MINUS",
  "NATURAL", "NOT", "NULL",
  "OF", "ON", "OR", "ORDER",
  "PRECEDING", "PRIMARY",
  "QUALIFY",
  "REGEXP", "REVOKE", "RIGHT", "RLIKE", "ROW", "ROWS",
  "SAMPLE", "SCHEMA", "SELECT", "SET", "SOME", "START",
  "TABLE", "TABLESAMPLE", "THEN", "TO", "TRIGGER", "TRUE", "TRY_CAST",
  "UNION", "UNIQUE", "UNPIVOT", "UPDATE", "USING",
  "VALUES", "VIEW",
  "WHEN", "WHENEVER", "WHERE", "WITH",
]);

interface ColumnMapping {
  targetCol: string;
  sourceExpr: string; // column name, constant, or expression
  isConstant: boolean;
  warnNullable?: boolean;
  typeMismatch?: boolean;
}

export default function InsertMappingModal() {
  const { target, source, modalOpen, setModalOpen, reset } = useInsertMappingStore();
  const [targetCols, setTargetCols] = useState<snowflake.ColumnInfo[]>([]);
  const [sourceCols, setSourceCols] = useState<snowflake.ColumnInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [mappings, setMappings] = useState<Record<string, ColumnMapping>>({});
  const [quoteIdentifiers, setQuoteIdentifiers] = useState(true);

  // Quote an identifier.
  // When the toggle is off, quoting is skipped only when ALL of:
  //   1. structurally safe (no spaces / special chars)
  //   2. not a Snowflake reserved keyword
  //   3. all-uppercase — Snowflake folds unquoted names to uppercase, so any
  //      lowercase letter means the name was created with quotes and is
  //      case-sensitive ("my_col" ≠ "MY_COL" ≠ my_col).
  const q = (s: string) =>
    quoteIdentifiers ||
    !SAFE_IDENT.test(s) ||
    SNOWFLAKE_RESERVED.has(s.toUpperCase()) ||
    s !== s.toUpperCase()
      ? `"${s.replace(/"/g, '""')}"`
      : s;


  useEffect(() => {
    if (modalOpen && target && source) {
      loadMetadata();
    }
  }, [modalOpen, target, source]);

  const loadMetadata = async () => {
    if (!target || !source) return;
    setLoading(true);
    try {
      const [tCols, sCols] = await Promise.all([
        GetTableColumnsWithTypes(target.db, target.schema, target.name),
        GetTableColumnsWithTypes(source.db, source.schema, source.name),
      ]);
      setTargetCols(tCols);
      setSourceCols(sCols);
      autoMap(tCols, sCols);
    } catch (e) {
      message.error(`Failed to load column metadata: ${e}`);
    } finally {
      setLoading(false);
    }
  };

  const autoMap = (tCols: snowflake.ColumnInfo[], sCols: snowflake.ColumnInfo[]) => {
    const newMappings: Record<string, ColumnMapping> = {};
    const sColMap = new Map(sCols.map(c => [c.name.toUpperCase(), c]));

    tCols.forEach(tc => {
      const match = sColMap.get(tc.name.toUpperCase());
      if (match) {
        newMappings[tc.name] = {
          targetCol: tc.name,
          sourceExpr: match.name,
          isConstant: false,
          warnNullable: !tc.nullable && match.nullable,
          typeMismatch: tc.dataType.split("(")[0] !== match.dataType.split("(")[0],
        };
      } else {
        newMappings[tc.name] = {
          targetCol: tc.name,
          sourceExpr: "NULL",
          isConstant: true,
        };
      }
    });
    setMappings(newMappings);
  };

  const handleSourceChange = (targetCol: string, value: string) => {
    const tc = targetCols.find(c => c.name === targetCol);
    const sc = sourceCols.find(c => c.name === value);
    
    if (value === "__constant__") {
      setMappings(prev => ({
        ...prev,
        [targetCol]: { ...prev[targetCol], isConstant: true, sourceExpr: "''", warnNullable: false, typeMismatch: false }
      }));
      return;
    }

    if (value === "NULL") {
      setMappings(prev => ({
        ...prev,
        [targetCol]: { ...prev[targetCol], isConstant: true, sourceExpr: "NULL", warnNullable: tc && !tc.nullable, typeMismatch: false }
      }));
      return;
    }

    setMappings(prev => ({
      ...prev,
      [targetCol]: {
        targetCol,
        sourceExpr: value,
        isConstant: false,
        warnNullable: tc && !tc.nullable && sc && sc.nullable,
        typeMismatch: tc && sc && tc.dataType.split("(")[0] !== sc.dataType.split("(")[0],
      }
    }));
  };

  const handleConstantChange = (targetCol: string, value: string) => {
    setMappings(prev => ({
      ...prev,
      [targetCol]: { ...prev[targetCol], sourceExpr: value }
    }));
  };

  const addCoalesce = (targetCol: string) => {
    const m = mappings[targetCol];
    const tc = targetCols.find(c => c.name === targetCol);
    let defaultVal = "''";
    if (tc?.dataType.toUpperCase().includes("NUMBER") || tc?.dataType.toUpperCase().includes("INT") || tc?.dataType.toUpperCase().includes("FLOAT")) {
      defaultVal = "0";
    } else if (tc?.dataType.toUpperCase().includes("BOOLEAN")) {
      defaultVal = "FALSE";
    } else if (tc?.dataType.toUpperCase().includes("DATE") || tc?.dataType.toUpperCase().includes("TIME")) {
      defaultVal = "CURRENT_TIMESTAMP()";
    }

    setMappings(prev => ({
      ...prev,
      [targetCol]: { ...prev[targetCol], sourceExpr: `COALESCE(${q(m.sourceExpr)}, ${defaultVal})`, warnNullable: false }
    }));
  };

  const addCast = (targetCol: string) => {
    const m = mappings[targetCol];
    const tc = targetCols.find(c => c.name === targetCol);
    if (!tc) return;

    setMappings(prev => ({
      ...prev,
      [targetCol]: { ...prev[targetCol], sourceExpr: `CAST(${q(m.sourceExpr)} AS ${tc.dataType})`, typeMismatch: false }
    }));
  };

  const generateSQL = () => {
    if (!target || !source) return "";

    const targetRef = `${q(target.db)}.${q(target.schema)}.${q(target.name)}`;
    const sourceRef = `${q(source.db)}.${q(source.schema)}.${q(source.name)}`;

    const cols = targetCols.map(tc => q(tc.name));
    const exprs = targetCols.map(tc => {
      const m = mappings[tc.name];
      if (m.isConstant) return m.sourceExpr;
      // Already an expression (COALESCE / CAST / etc.) — pass through as-is
      if (m.sourceExpr.includes("(") || m.sourceExpr.includes(" ")) return m.sourceExpr;
      return q(m.sourceExpr);
    });

    const pad = "    ";
    const colLines  = cols.map(c  => `${pad}${c}`).join(",\n");
    const exprLines = exprs.map(e => `${pad}${e}`).join(",\n");

    return [
      `INSERT INTO ${targetRef} (`,
      colLines,
      `)`,
      `SELECT`,
      exprLines,
      `FROM ${sourceRef};`,
    ].join("\n");
  };

  const handleInsert = () => {
    const sql = generateSQL();
    useQueryStore.getState().loadInNewTab(sql);
    setModalOpen(false);
    reset();
  };

  const columns = [
    {
      title: "Target Column",
      dataIndex: "name",
      key: "target",
      render: (name: string, record: snowflake.ColumnInfo) => (
        <Space direction="vertical" size={0}>
          <Text strong>{name}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.dataType} {!record.nullable && <Tag color="red" style={{ fontSize: 9 }}>NOT NULL</Tag>}</Text>
        </Space>
      )
    },
    {
      title: "",
      key: "arrow",
      width: 40,
      render: () => <ArrowRightOutlined style={{ color: "var(--text-muted)" }} />
    },
    {
      title: "Source Expression",
      key: "source",
      render: (_: any, record: snowflake.ColumnInfo) => {
        const m = mappings[record.name];
        if (!m) return null;

        return (
          <Space direction="vertical" style={{ width: "100%" }}>
            <Space>
              <Select
                showSearch
                style={{ width: 200 }}
                value={m.isConstant ? (m.sourceExpr === "NULL" ? "NULL" : "__constant__") : m.sourceExpr}
                onChange={(val) => handleSourceChange(record.name, val)}
              >
                <Option value="NULL"><Text type="secondary">NULL</Text></Option>
                <Option value="__constant__"><Text italic>Constant Value...</Text></Option>
                {sourceCols.map(sc => (
                  <Option key={sc.name} value={sc.name}>
                    {sc.name} <Text type="secondary" style={{ fontSize: 11 }}>({sc.dataType})</Text>
                  </Option>
                ))}
              </Select>
              {m.isConstant && m.sourceExpr !== "NULL" && (
                <Input 
                  style={{ width: 150 }} 
                  value={m.sourceExpr} 
                  onChange={e => handleConstantChange(record.name, e.target.value)} 
                />
              )}
            </Space>
            {m.warnNullable && (
              <Alert
                type="warning"
                showIcon
                message={
                  <Space>
                    <Text style={{ fontSize: 12 }}>Source may be NULL</Text>
                    <Button size="small" type="link" onClick={() => addCoalesce(record.name)}>Add COALESCE</Button>
                  </Space>
                }
              />
            )}
            {m.typeMismatch && (
              <Alert
                type="info"
                showIcon
                message={
                  <Space>
                    <Text style={{ fontSize: 12 }}>Type mismatch</Text>
                    <Button size="small" type="link" onClick={() => addCast(record.name)}>Add CAST</Button>
                  </Space>
                }
              />
            )}
          </Space>
        );
      }
    }
  ];

  return (
    <Modal
      title={
        <Space>
          <SyncOutlined />
          <span>Insert Mapping: {source?.name} → {target?.name}</span>
        </Space>
      }
      open={modalOpen}
      onCancel={() => { setModalOpen(false); reset(); }}
      width={800}
      onOk={handleInsert}
      okText="Generate SQL"
      destroyOnClose
    >
      <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 8 }}>
        <Space>
          <Text type="secondary" style={{ fontSize: 12 }}>Quote identifiers</Text>
          <Switch
            size="small"
            checked={quoteIdentifiers}
            onChange={setQuoteIdentifiers}
          />
        </Space>
      </div>
      <div style={{ maxHeight: "60vh", overflowY: "auto" }}>
        <Table
          dataSource={targetCols}
          columns={columns}
          rowKey="name"
          pagination={false}
          size="small"
          loading={loading}
        />
      </div>
    </Modal>
  );
}
