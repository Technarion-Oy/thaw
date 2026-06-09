// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef, useMemo } from "react";
import {
  Modal, Table, Select, Input, Button, Space, Typography,
  Tag, message, Switch, Radio, Tooltip,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import { SyncOutlined, DeleteOutlined, InfoCircleOutlined } from "@ant-design/icons";
import { GetTableColumnsWithTypes } from "../../../wailsjs/go/app/App";
import { snowflake } from "../../../wailsjs/go/models";
import { useInsertMappingStore } from "../../store/insertMappingStore";
import { useObjectStore } from "../../store/objectStore";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;
const { Option } = Select;

// Identifiers that are structurally safe (no special chars / spaces)
const SAFE_IDENT = /^[a-zA-Z_][a-zA-Z0-9_$]*$/;

// Snowflake reserved keywords — must always be double-quoted when used as identifiers
const SNOWFLAKE_RESERVED = new Set([
  "ACCOUNT", "ALL", "ALTER", "AND", "ANY", "AS",
  "BETWEEN", "BY",
  "CASE", "CAST", "CHECK", "COLUMN", "CONNECT", "CONNECTION", "CONSTRAINT",
  "CREATE", "CROSS", "CROSS", "CURRENT", "CURRENT_DATE", "CURRENT_TIME",
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
  const { target, sources, modalOpen, setModalOpen, addSource, removeSource, reset } =
    useInsertMappingStore();
  const allObjects = useObjectStore((s) => s.objects);

  const [targetCols, setTargetCols] = useState<snowflake.ColumnInfo[]>([]);
  // Parallel arrays indexed by source position
  const [allSourceCols, setAllSourceCols] = useState<(snowflake.ColumnInfo[] | undefined)[]>([]);
  const [allMappings, setAllMappings]     = useState<(Record<string, ColumnMapping> | undefined)[]>([]);
  const [unionAll, setUnionAll]           = useState(true);
  const [loadingTarget, setLoadingTarget] = useState(false);
  const [loadingSources, setLoadingSources] = useState(false);
  const [quoteIdentifiers, setQuoteIdentifiers] = useState(true);

  // Prevent concurrent duplicate loads for the same source index
  const loadingIndices = useRef<Set<number>>(new Set());

  // ── Quoting helper ──────────────────────────────────────────────────────────
  const q = (s: string) =>
    quoteIdentifiers ||
    !SAFE_IDENT.test(s) ||
    SNOWFLAKE_RESERVED.has(s.toUpperCase()) ||
    s !== s.toUpperCase()
      ? `"${s.replace(/"/g, '""')}"`
      : s;

  // ── Auto-map source columns to target columns by name ──────────────────────
  const buildAutoMap = (
    tCols: snowflake.ColumnInfo[],
    sCols: snowflake.ColumnInfo[],
  ): Record<string, ColumnMapping> => {
    const map: Record<string, ColumnMapping> = {};
    const sColMap = new Map(sCols.map((c) => [c.name.toUpperCase(), c]));
    tCols.forEach((tc) => {
      const match = sColMap.get(tc.name.toUpperCase());
      if (match) {
        map[tc.name] = {
          targetCol: tc.name,
          sourceExpr: match.name,
          isConstant: false,
          warnNullable: !tc.nullable && match.nullable,
          typeMismatch: tc.dataType.split("(")[0] !== match.dataType.split("(")[0],
        };
      } else {
        map[tc.name] = { targetCol: tc.name, sourceExpr: "NULL", isConstant: true };
      }
    });
    return map;
  };

  // ── Load target + any not-yet-loaded sources ────────────────────────────────
  useEffect(() => {
    if (!modalOpen || !target) return;

    const runLoad = async () => {
      let tCols = targetCols;
      if (tCols.length === 0) {
        setLoadingTarget(true);
        try {
          tCols = await GetTableColumnsWithTypes(target.db, target.schema, target.name);
          setTargetCols(tCols);
        } catch (e) {
          message.error(`Failed to load target columns: ${e}`);
          return;
        } finally {
          setLoadingTarget(false);
        }
      }

      const toLoad = sources
        .map((src, i) => ({ src, i }))
        .filter(({ i }) => allSourceCols[i] === undefined && !loadingIndices.current.has(i));
      if (toLoad.length === 0) return;

      toLoad.forEach(({ i }) => loadingIndices.current.add(i));
      setLoadingSources(true);
      try {
        const results = await Promise.all(
          toLoad.map(async ({ src, i }) => {
            const sCols = await GetTableColumnsWithTypes(src.db, src.schema, src.name);
            return { i, sCols };
          }),
        );
        setAllSourceCols((prev) => {
          const next = [...prev];
          results.forEach(({ i, sCols }) => { next[i] = sCols; });
          return next;
        });
        setAllMappings((prev) => {
          const next = [...prev];
          results.forEach(({ i, sCols }) => { next[i] = buildAutoMap(tCols, sCols); });
          return next;
        });
      } catch (e) {
        message.error(`Failed to load source columns: ${e}`);
      } finally {
        toLoad.forEach(({ i }) => loadingIndices.current.delete(i));
        setLoadingSources(false);
      }
    };

    runLoad();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [modalOpen, target, sources.length]);

  const handleRemoveSource = (i: number) => {
    if (sources.length === 1) {
      setModalOpen(false);
      reset();
      return;
    }
    setAllSourceCols((prev) => prev.filter((_, idx) => idx !== i));
    setAllMappings((prev) => prev.filter((_, idx) => idx !== i));
    removeSource(i);
  };

  const handleAddFromPicker = (val: string) => {
    const [db, schema, name] = val.split("\0");
    addSource({ db, schema, name });
  };

  const updateMapping = (idx: number, patch: Record<string, ColumnMapping>) => {
    setAllMappings((prev) => {
      const next = [...prev];
      next[idx] = patch;
      return next;
    });
  };

  const handleSourceChange = (idx: number, targetCol: string, value: string) => {
    const tc   = targetCols.find((c) => c.name === targetCol);
    const sCols = allSourceCols[idx] ?? [];
    const sc   = sCols.find((c) => c.name === value);
    const prev = allMappings[idx] ?? {};

    if (value === "__constant__") {
      updateMapping(idx, {
        ...prev,
        [targetCol]: { ...prev[targetCol], isConstant: true, sourceExpr: "''", warnNullable: false, typeMismatch: false },
      });
      return;
    }
    if (value === "NULL") {
      updateMapping(idx, {
        ...prev,
        [targetCol]: { ...prev[targetCol], isConstant: true, sourceExpr: "NULL", warnNullable: !!(tc && !tc.nullable), typeMismatch: false },
      });
      return;
    }
    updateMapping(idx, {
      ...prev,
      [targetCol]: {
        targetCol,
        sourceExpr: value,
        isConstant: false,
        warnNullable: !!(tc && !tc.nullable && sc && sc.nullable),
        typeMismatch: !!(tc && sc && tc.dataType.split("(")[0] !== sc.dataType.split("(")[0]),
      },
    });
  };

  const handleConstantChange = (idx: number, targetCol: string, value: string) => {
    const prev = allMappings[idx] ?? {};
    updateMapping(idx, { ...prev, [targetCol]: { ...prev[targetCol], sourceExpr: value } });
  };

  const addCoalesce = (idx: number, targetCol: string) => {
    const prev = allMappings[idx] ?? {};
    const m  = prev[targetCol];
    const tc = targetCols.find((c) => c.name === targetCol);
    let defaultVal = "''";
    const dt = tc?.dataType.toUpperCase() ?? "";
    if (dt.includes("NUMBER") || dt.includes("INT") || dt.includes("FLOAT")) defaultVal = "0";
    else if (dt.includes("BOOLEAN")) defaultVal = "FALSE";
    else if (dt.includes("DATE") || dt.includes("TIME")) defaultVal = "CURRENT_TIMESTAMP()";
    updateMapping(idx, { ...prev, [targetCol]: { ...m, sourceExpr: `COALESCE(${q(m.sourceExpr)}, ${defaultVal})`, warnNullable: false } });
  };

  const addCast = (idx: number, targetCol: string) => {
    const prev = allMappings[idx] ?? {};
    const m  = prev[targetCol];
    const tc = targetCols.find((c) => c.name === targetCol);
    if (!tc) return;
    updateMapping(idx, { ...prev, [targetCol]: { ...m, sourceExpr: `CAST(${q(m.sourceExpr)} AS ${tc.dataType})`, typeMismatch: false } });
  };

  // ── SQL generation ──────────────────────────────────────────────────────────
  const generateSQL = () => {
    if (!target || sources.length === 0) return "";

    const targetRef = `${q(target.db)}.${q(target.schema)}.${q(target.name)}`;
    const cols      = targetCols.map((tc) => q(tc.name));
    const pad       = "    ";
    const colLines  = cols.map((c) => `${pad}${c}`).join(",\n");

    const selects = sources.map((src, i) => {
      const srcRef   = `${q(src.db)}.${q(src.schema)}.${q(src.name)}`;
      const mappings = allMappings[i] ?? {};
      const exprs    = targetCols.map((tc) => {
        const m = mappings[tc.name];
        if (!m || m.isConstant) return m?.sourceExpr ?? "NULL";
        if (m.sourceExpr.includes("(") || m.sourceExpr.includes(" ")) return m.sourceExpr;
        return q(m.sourceExpr);
      });
      return `SELECT\n${exprs.map((e) => `${pad}${e}`).join(",\n")}\nFROM ${srcRef}`;
    });

    const unionKw = unionAll ? "\nUNION ALL\n" : "\nUNION\n";
    return [`INSERT INTO ${targetRef} (`, colLines, `)`, selects.join(unionKw) + ";"].join("\n");
  };

  const handleInsert = () => {
    const sql = generateSQL();
    useQueryStore.getState().loadInNewTab(sql);
    setModalOpen(false);
    reset();
  };

  // ── Column table definition (side-by-side) ────────────────────────────────
  const tableColumns = useMemo<ColumnsType<snowflake.ColumnInfo>>(() => {
    const cols: ColumnsType<snowflake.ColumnInfo> = [
      {
        title: "Target Column",
        dataIndex: "name",
        key: "target",
        fixed: "left" as const,
        width: 180,
        render: (name: string, record: snowflake.ColumnInfo) => (
          <Space direction="vertical" size={0}>
            <Text strong>{name}</Text>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {record.dataType}{" "}
              {!record.nullable && <Tag color="red" style={{ fontSize: 9 }}>NOT NULL</Tag>}
            </Text>
          </Space>
        ),
      },
    ];

    sources.forEach((src, idx) => {
      cols.push({
        title: (
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Tooltip title={`${src.db}.${src.schema}.${src.name}`}>
              <Text strong style={{ maxWidth: 120 }} ellipsis>{src.name}</Text>
            </Tooltip>
            <Button 
              type="text" 
              size="small" 
              icon={<DeleteOutlined style={{ fontSize: 11 }} />} 
              onClick={() => handleRemoveSource(idx)} 
            />
          </div>
        ),
        key: `source_${idx}`,
        width: 300,
        render: (_: unknown, record: snowflake.ColumnInfo) => {
          const m = (allMappings[idx] ?? {})[record.name];
          if (!m) return <div />;
          const sCols = allSourceCols[idx] ?? [];
          return (
            <Space direction="vertical" style={{ width: "100%" }}>
              <Space.Compact style={{ width: "100%" }}>
                <Select
                  showSearch
                  style={{ width: "60%" }}
                  size="small"
                  value={m.isConstant ? (m.sourceExpr === "NULL" ? "NULL" : "__constant__") : m.sourceExpr}
                  onChange={(val) => handleSourceChange(idx, record.name, val)}
                >
                  <Option value="NULL"><Text type="secondary">NULL</Text></Option>
                  <Option value="__constant__"><Text italic>Constant...</Text></Option>
                  {sCols.map((sc) => (
                    <Option key={sc.name} value={sc.name}>
                      {sc.name}{" "}
                      <Text type="secondary" style={{ fontSize: 10 }}>({sc.dataType})</Text>
                    </Option>
                  ))}
                </Select>
                {m.isConstant && m.sourceExpr !== "NULL" && (
                  <Input
                    style={{ width: "40%" }}
                    size="small"
                    value={m.sourceExpr}
                    onChange={(e) => handleConstantChange(idx, record.name, e.target.value)}
                  />
                )}
              </Space.Compact>
              {m.warnNullable && (
                <div style={{ marginTop: 2 }}>
                  <Tag color="orange" style={{ fontSize: 9, margin: 0 }}>NULLable Source</Tag>
                  <Button size="small" type="link" style={{ fontSize: 10, padding: "0 4px" }} onClick={() => addCoalesce(idx, record.name)}>COALESCE</Button>
                </div>
              )}
              {m.typeMismatch && (
                <div style={{ marginTop: 2 }}>
                  <Tag color="blue" style={{ fontSize: 9, margin: 0 }}>Type Mismatch</Tag>
                  <Button size="small" type="link" style={{ fontSize: 10, padding: "0 4px" }} onClick={() => addCast(idx, record.name)}>CAST</Button>
                </div>
              )}
            </Space>
          );
        },
      });
    });

    return cols;
  }, [sources, allSourceCols, allMappings, targetCols, quoteIdentifiers, handleRemoveSource, handleSourceChange, handleConstantChange, addCoalesce, addCast, q]);

  const tableOptions = allObjects
    .filter((o) => o.kind === "TABLE" || o.kind === "VIEW")
    .map((o) => ({
      label: `${o.db}.${o.schema}.${o.name}`,
      value: `${o.db}\0${o.schema}\0${o.name}`,
    }));

  // ── Render ──────────────────────────────────────────────────────────────────
  return (
    <Modal
      title={
        <Space>
          <SyncOutlined />
          <span>Insert Mapping → {target?.name}</span>
        </Space>
      }
      open={modalOpen}
      onCancel={() => { setModalOpen(false); reset(); }}
      width="95vw"
      style={{ top: 20 }}
      onOk={handleInsert}
      okText="Generate SQL"
      mask={false}
      destroyOnHidden
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 16 }}>
        <Space direction="vertical" size={4}>
          <Space>
            <Text type="secondary" style={{ fontSize: 12 }}>Combine mode:</Text>
            <Radio.Group
              value={unionAll ? "all" : "distinct"}
              onChange={(e) => setUnionAll(e.target.value === "all")}
              size="small"
              optionType="button"
              buttonStyle="solid"
            >
              <Radio.Button value="all">UNION ALL</Radio.Button>
              <Radio.Button value="distinct">UNION</Radio.Button>
            </Radio.Group>
            <Tooltip title={unionAll
              ? "Multiple rows with the same data are kept (UNION ALL)." 
              : "Duplicate rows are removed (UNION)."}>
              <InfoCircleOutlined style={{ fontSize: 12, color: "var(--text-muted)" }} />
            </Tooltip>
          </Space>
        </Space>

        <Space direction="vertical" align="end" size={4}>
          <Select
            showSearch
            placeholder="+ Add source table"
            style={{ width: 230 }}
            size="small"
            value={null}
            onChange={handleAddFromPicker}
            options={tableOptions}
            filterOption={(input, opt) =>
              String(opt?.label ?? "").toLowerCase().includes(input.toLowerCase())
            }
          />
          <Space>
            <Text type="secondary" style={{ fontSize: 11 }}>Quote identifiers</Text>
            <Switch size="small" checked={quoteIdentifiers} onChange={setQuoteIdentifiers} />
          </Space>
        </Space>
      </div>

      <div style={{ border: "1px solid var(--border)", borderRadius: 8, overflow: "hidden" }}>
        <Table
          dataSource={targetCols}
          columns={tableColumns}
          rowKey="name"
          pagination={false}
          size="small"
          scroll={{ x: "max-content", y: "55vh" }}
          loading={loadingTarget || loadingSources}
        />
      </div>
    </Modal>
  );
}
