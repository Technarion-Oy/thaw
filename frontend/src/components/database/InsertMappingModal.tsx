// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef } from "react";
import {
  Modal, Table, Select, Input, Button, Space, Typography,
  Tag, Alert, message, Switch, Tabs, Radio,
} from "antd";
import { ArrowRightOutlined, SyncOutlined, CloseOutlined } from "@ant-design/icons";
import { GetTableColumnsWithTypes } from "../../../wailsjs/go/main/App";
import { snowflake } from "../../../wailsjs/go/models";
import { useInsertMappingStore } from "../../store/insertMappingStore";
import { useObjectStore } from "../../store/objectStore";
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
  const { target, sources, modalOpen, setModalOpen, addSource, removeSource, reset } =
    useInsertMappingStore();
  const allObjects = useObjectStore((s) => s.objects);

  const [targetCols, setTargetCols] = useState<snowflake.ColumnInfo[]>([]);
  // Parallel arrays indexed by source position
  const [allSourceCols, setAllSourceCols] = useState<(snowflake.ColumnInfo[] | undefined)[]>([]);
  const [allMappings, setAllMappings]     = useState<(Record<string, ColumnMapping> | undefined)[]>([]);
  const [activeTab, setActiveTab]         = useState(0);
  const [unionAll, setUnionAll]           = useState(true);
  const [loadingTarget, setLoadingTarget] = useState(false);
  const [loadingSources, setLoadingSources] = useState(false);
  const [quoteIdentifiers, setQuoteIdentifiers] = useState(true);

  // Prevent concurrent duplicate loads for the same source index
  const loadingIndices = useRef<Set<number>>(new Set());

  // ── Quoting helper ──────────────────────────────────────────────────────────
  // Skip quotes only when ALL of: structurally safe, not a reserved keyword,
  // and all-uppercase (Snowflake folds unquoted names to uppercase, so any
  // lowercase letter means the name was created with quotes).
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
      // Load target columns once
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

      // Load any source not yet fetched
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

  // ── Source management ───────────────────────────────────────────────────────
  const handleRemoveSource = (i: number) => {
    if (sources.length === 1) {
      // Removing the last source closes the modal
      setModalOpen(false);
      reset();
      return;
    }
    setAllSourceCols((prev) => prev.filter((_, idx) => idx !== i));
    setAllMappings((prev) => prev.filter((_, idx) => idx !== i));
    setActiveTab((prev) => Math.min(prev, sources.length - 2));
    removeSource(i);
  };

  const handleAddFromPicker = (val: string) => {
    const [db, schema, name] = val.split("\0");
    const newIdx = sources.length; // index after store update
    addSource({ db, schema, name });
    setTimeout(() => setActiveTab(newIdx), 0);
  };

  // ── Mapping mutations (scoped to a source tab index) ───────────────────────
  const updateMapping = (tabIdx: number, patch: Record<string, ColumnMapping>) => {
    setAllMappings((prev) => {
      const next = [...prev];
      next[tabIdx] = patch;
      return next;
    });
  };

  const handleSourceChange = (tabIdx: number, targetCol: string, value: string) => {
    const tc   = targetCols.find((c) => c.name === targetCol);
    const sCols = allSourceCols[tabIdx] ?? [];
    const sc   = sCols.find((c) => c.name === value);
    const prev = allMappings[tabIdx] ?? {};

    if (value === "__constant__") {
      updateMapping(tabIdx, {
        ...prev,
        [targetCol]: { ...prev[targetCol], isConstant: true, sourceExpr: "''", warnNullable: false, typeMismatch: false },
      });
      return;
    }
    if (value === "NULL") {
      updateMapping(tabIdx, {
        ...prev,
        [targetCol]: { ...prev[targetCol], isConstant: true, sourceExpr: "NULL", warnNullable: !!(tc && !tc.nullable), typeMismatch: false },
      });
      return;
    }
    updateMapping(tabIdx, {
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

  const handleConstantChange = (tabIdx: number, targetCol: string, value: string) => {
    const prev = allMappings[tabIdx] ?? {};
    updateMapping(tabIdx, { ...prev, [targetCol]: { ...prev[targetCol], sourceExpr: value } });
  };

  const addCoalesce = (tabIdx: number, targetCol: string) => {
    const prev = allMappings[tabIdx] ?? {};
    const m  = prev[targetCol];
    const tc = targetCols.find((c) => c.name === targetCol);
    let defaultVal = "''";
    const dt = tc?.dataType.toUpperCase() ?? "";
    if (dt.includes("NUMBER") || dt.includes("INT") || dt.includes("FLOAT")) defaultVal = "0";
    else if (dt.includes("BOOLEAN")) defaultVal = "FALSE";
    else if (dt.includes("DATE") || dt.includes("TIME")) defaultVal = "CURRENT_TIMESTAMP()";
    updateMapping(tabIdx, { ...prev, [targetCol]: { ...m, sourceExpr: `COALESCE(${q(m.sourceExpr)}, ${defaultVal})`, warnNullable: false } });
  };

  const addCast = (tabIdx: number, targetCol: string) => {
    const prev = allMappings[tabIdx] ?? {};
    const m  = prev[targetCol];
    const tc = targetCols.find((c) => c.name === targetCol);
    if (!tc) return;
    updateMapping(tabIdx, { ...prev, [targetCol]: { ...m, sourceExpr: `CAST(${q(m.sourceExpr)} AS ${tc.dataType})`, typeMismatch: false } });
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

  // ── Column table definition (per source tab) ────────────────────────────────
  const buildColumns = (tabIdx: number) => {
    const sCols    = allSourceCols[tabIdx] ?? [];
    const mappings = allMappings[tabIdx]   ?? {};

    return [
      {
        title: "Target Column",
        dataIndex: "name",
        key: "target",
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
      {
        title: "",
        key: "arrow",
        width: 40,
        render: () => <ArrowRightOutlined style={{ color: "var(--text-muted)" }} />,
      },
      {
        title: "Source Expression",
        key: "source",
        render: (_: unknown, record: snowflake.ColumnInfo) => {
          const m = mappings[record.name];
          if (!m) return null;
          return (
            <Space direction="vertical" style={{ width: "100%" }}>
              <Space>
                <Select
                  showSearch
                  style={{ width: 200 }}
                  value={m.isConstant ? (m.sourceExpr === "NULL" ? "NULL" : "__constant__") : m.sourceExpr}
                  onChange={(val) => handleSourceChange(tabIdx, record.name, val)}
                >
                  <Option value="NULL"><Text type="secondary">NULL</Text></Option>
                  <Option value="__constant__"><Text italic>Constant Value...</Text></Option>
                  {sCols.map((sc) => (
                    <Option key={sc.name} value={sc.name}>
                      {sc.name}{" "}
                      <Text type="secondary" style={{ fontSize: 11 }}>({sc.dataType})</Text>
                    </Option>
                  ))}
                </Select>
                {m.isConstant && m.sourceExpr !== "NULL" && (
                  <Input
                    style={{ width: 150 }}
                    value={m.sourceExpr}
                    onChange={(e) => handleConstantChange(tabIdx, record.name, e.target.value)}
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
                      <Button size="small" type="link" onClick={() => addCoalesce(tabIdx, record.name)}>
                        Add COALESCE
                      </Button>
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
                      <Button size="small" type="link" onClick={() => addCast(tabIdx, record.name)}>
                        Add CAST
                      </Button>
                    </Space>
                  }
                />
              )}
            </Space>
          );
        },
      },
    ];
  };

  // ── Searchable table/view picker from objectStore ───────────────────────────
  const tableOptions = allObjects
    .filter((o) => o.kind === "TABLE" || o.kind === "VIEW")
    .map((o) => ({
      label: `${o.db}.${o.schema}.${o.name}`,
      value: `${o.db}\0${o.schema}\0${o.name}`,
    }));

  // ── Tab items ───────────────────────────────────────────────────────────────
  const tabItems = sources.map((src, i) => ({
    key: String(i),
    label: (
      <Space size={4}>
        <span>{src.name}</span>
        <CloseOutlined
          style={{ fontSize: 10, color: "var(--text-muted)" }}
          onClick={(e) => { e.stopPropagation(); handleRemoveSource(i); }}
        />
      </Space>
    ),
    children: (
      <div style={{ maxHeight: "50vh", overflowY: "auto" }}>
        <Table
          dataSource={targetCols}
          columns={buildColumns(i)}
          rowKey="name"
          pagination={false}
          size="small"
          loading={loadingTarget || (loadingSources && !allSourceCols[i])}
        />
      </div>
    ),
  }));

  // ── Render ──────────────────────────────────────────────────────────────────
  return (
    <Modal
      title={
        <Space>
          <SyncOutlined />
          <span>
            Insert Mapping:{" "}
            {sources.length === 0 ? "…" : sources.map((s) => s.name).join(", ")}
            {" → "}
            {target?.name}
          </span>
        </Space>
      }
      open={modalOpen}
      onCancel={() => { setModalOpen(false); reset(); }}
      width={880}
      onOk={handleInsert}
      okText="Generate SQL"
      // mask=false keeps the sidebar accessible so users can right-click
      // additional tables and add them as sources while the modal is open
      mask={false}
      destroyOnHidden
    >
      {/* ── Toolbar ── */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
        <Space>
          {sources.length > 1 && (
            <>
              <Text type="secondary" style={{ fontSize: 12 }}>Combine with:</Text>
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
            </>
          )}
        </Space>
        <Space>
          <Text type="secondary" style={{ fontSize: 12 }}>Quote identifiers</Text>
          <Switch size="small" checked={quoteIdentifiers} onChange={setQuoteIdentifiers} />
        </Space>
      </div>

      {/* ── Source tabs + add-source picker ── */}
      <Tabs
        type="card"
        size="small"
        activeKey={String(activeTab)}
        onChange={(k) => setActiveTab(Number(k))}
        tabBarExtraContent={
          <Select
            showSearch
            placeholder="+ Add source table"
            style={{ width: 230, marginLeft: 8 }}
            value={null}
            onChange={handleAddFromPicker}
            options={tableOptions}
            notFoundContent={
              <Text type="secondary" style={{ fontSize: 12 }}>
                Expand schemas in the sidebar to load tables
              </Text>
            }
            filterOption={(input, opt) =>
              String(opt?.label ?? "").toLowerCase().includes(input.toLowerCase())
            }
          />
        }
        items={tabItems}
      />
    </Modal>
  );
}
