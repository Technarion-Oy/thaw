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
import { Modal, Table, Select, Input, Button, Space, Typography, Tag, Alert, message } from "antd";
import { ArrowRightOutlined, SyncOutlined } from "@ant-design/icons";
import { GetTableColumnsWithTypes } from "../../../wailsjs/go/main/App";
import { snowflake } from "../../../wailsjs/go/models";
import { useInsertMappingStore } from "../../store/insertMappingStore";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;
const { Option } = Select;

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
      [targetCol]: { ...prev[targetCol], sourceExpr: `COALESCE("${m.sourceExpr}", ${defaultVal})`, warnNullable: false }
    }));
  };

  const addCast = (targetCol: string) => {
    const m = mappings[targetCol];
    const tc = targetCols.find(c => c.name === targetCol);
    if (!tc) return;
    
    setMappings(prev => ({
      ...prev,
      [targetCol]: { ...prev[targetCol], sourceExpr: `CAST("${m.sourceExpr}" AS ${tc.dataType})`, typeMismatch: false }
    }));
  };

  const generateSQL = () => {
    if (!target || !source) return "";
    
    const quote = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const targetRef = `${quote(target.db)}.${quote(target.schema)}.${quote(target.name)}`;
    const sourceRef = `${quote(source.db)}.${quote(source.schema)}.${quote(source.name)}`;

    const cols = targetCols.map(tc => quote(tc.name));
    const exprs = targetCols.map(tc => {
      const m = mappings[tc.name];
      if (m.isConstant) return m.sourceExpr;
      // If it's already an expression (has COALESCE or CAST), use it as is
      if (m.sourceExpr.includes("(") || m.sourceExpr.includes(" ")) return m.sourceExpr;
      return quote(m.sourceExpr);
    });

    return `INSERT INTO ${targetRef} (${cols.join(", ")})\nSELECT ${exprs.join(", ")}\nFROM ${sourceRef};`;
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
      <div style={{ maxHeight: "60vh", overflowY: "auto", marginTop: 16 }}>
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
