// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useMemo, useCallback } from "react";
import {
  Modal, Tabs, Table, Input, Select, Button, Space, Typography, Alert, Tag, Tooltip,
  Empty, Form, message, Popconfirm,
} from "antd";
import {
  TagsOutlined, ReloadOutlined, EditOutlined, DeleteOutlined, PlusOutlined,
  CheckOutlined, CloseOutlined, SearchOutlined,
} from "@ant-design/icons";
import {
  ListAccountTags, GetAllTagReferences, SetObjectTag, UnsetObjectTag,
  ListDatabases, ListUserSchemas, ListObjects, GetTableColumns,
} from "../../../wailsjs/go/app/App";
import { tag as tagModels } from "../../../wailsjs/go/models";
import type { snowflake } from "../../../wailsjs/go/models";
import { parseAllowedValues } from "./allowedValues";

const { Text } = Typography;

// ─── QueryResult helpers ─────────────────────────────────────────────────────

// Case-insensitive column index lookup. SHOW TAGS reports lower-case column
// names while the TAG_REFERENCES SELECT reports upper-case ones, so callers look
// columns up by name rather than position.
function colIdx(res: snowflake.QueryResult | null, name: string): number {
  if (!res?.columns) return -1;
  return res.columns.findIndex((c) => c.toLowerCase() === name.toLowerCase());
}

function cell(row: unknown[], idx: number): string {
  if (idx < 0 || idx >= row.length) return "";
  const v = row[idx];
  return v === null || v === undefined ? "" : String(v);
}

// ─── Domain → object-name shape ──────────────────────────────────────────────

// Domains commonly available in the Apply-tag form, ordered roughly by how often
// they carry tags. The full set Snowflake supports is larger; users can type any
// other domain via the "Other…" option.
const COMMON_DOMAINS = [
  "TABLE", "VIEW", "COLUMN", "MATERIALIZED VIEW", "EXTERNAL TABLE", "DYNAMIC TABLE",
  "SCHEMA", "DATABASE", "STAGE", "STREAM", "TASK", "PIPE", "FUNCTION", "PROCEDURE",
  "WAREHOUSE", "ROLE", "USER", "INTEGRATION", "ACCOUNT",
];

// Account-level domains have no database/schema qualification.
const ACCOUNT_LEVEL = new Set(["WAREHOUSE", "ROLE", "USER", "INTEGRATION", "ACCOUNT", "DATABASE"]);

// Parent objects offered when tagging a COLUMN. Restricted to the kinds whose
// columns Snowflake lets you tag via `ALTER … ALTER COLUMN SET TAG` — tables and
// views. The selected parent's kind is carried into ObjectTagRef.parentKind so
// the backend emits ALTER VIEW vs ALTER TABLE correctly. The strings match the
// `kind` reported by ListObjects.
const COLUMN_PARENT_KINDS = new Set(["TABLE", "VIEW"]);

// ─── Reference rows ──────────────────────────────────────────────────────────

interface RefRow {
  key: string;
  tagDatabase: string;
  tagSchema: string;
  tagName: string;
  tagValue: string;
  objectDatabase: string;
  objectSchema: string;
  objectName: string;
  columnName: string;
  domain: string;
}

function toRefRows(res: snowflake.QueryResult | null): RefRow[] {
  if (!res?.rows) return [];
  const i = {
    td: colIdx(res, "TAG_DATABASE"), ts: colIdx(res, "TAG_SCHEMA"), tn: colIdx(res, "TAG_NAME"),
    tv: colIdx(res, "TAG_VALUE"), od: colIdx(res, "OBJECT_DATABASE"), os: colIdx(res, "OBJECT_SCHEMA"),
    on: colIdx(res, "OBJECT_NAME"), cn: colIdx(res, "COLUMN_NAME"), dm: colIdx(res, "DOMAIN"),
  };
  return res.rows.map((row, n) => ({
    key: String(n),
    tagDatabase: cell(row, i.td),
    tagSchema: cell(row, i.ts),
    tagName: cell(row, i.tn),
    tagValue: cell(row, i.tv),
    objectDatabase: cell(row, i.od),
    objectSchema: cell(row, i.os),
    objectName: cell(row, i.on),
    columnName: cell(row, i.cn),
    domain: cell(row, i.dm),
  }));
}

function refObjectRef(r: RefRow): tagModels.ObjectTagRef {
  return tagModels.ObjectTagRef.createFrom({
    domain: r.domain,
    database: r.objectDatabase,
    schema: r.objectSchema,
    name: r.objectName,
    column: r.columnName,
  });
}

function objectPath(r: RefRow): string {
  const parts = [r.objectDatabase, r.objectSchema, r.objectName].filter(Boolean);
  let p = parts.join(".");
  if (r.columnName) p += `.${r.columnName}`;
  return p || "(account)";
}

// ─── Apply-tag form ──────────────────────────────────────────────────────────

interface CatalogTag { database: string; schema: string; name: string; allowedValues: string[] }

interface ApplyTagModalProps {
  catalog: CatalogTag[];
  onClose: () => void;
  onApplied: () => void;
}

function ApplyTagModal({ catalog, onClose, onApplied }: ApplyTagModalProps) {
  const [tagKey, setTagKey] = useState<string | null>(catalog.length ? `${catalog[0].database}.${catalog[0].schema}.${catalog[0].name}` : null);
  const [domain, setDomain] = useState("TABLE");
  const [database, setDatabase] = useState("");
  const [schema, setSchema] = useState("");
  const [objName, setObjName] = useState("");
  const [column, setColumn] = useState("");
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Cascading option lists for the database → schema → object → column pickers.
  const [databases, setDatabases] = useState<string[]>([]);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [objects, setObjects] = useState<snowflake.SnowflakeObject[]>([]);
  const [columns, setColumns] = useState<string[]>([]);
  const [dbLoading, setDbLoading] = useState(false);
  const [schemaLoading, setSchemaLoading] = useState(false);
  const [objLoading, setObjLoading] = useState(false);
  const [colLoading, setColLoading] = useState(false);

  const isColumn = domain === "COLUMN";
  // A "schema-level" object lives inside db.schema (TABLE, VIEW, STAGE, COLUMN's
  // table, …) — as opposed to account-level objects and the special-cased
  // DATABASE / SCHEMA / ACCOUNT domains.
  const schemaLevel = !ACCOUNT_LEVEL.has(domain) && domain !== "SCHEMA" && domain !== "ACCOUNT";
  const showDatabase = domain !== "ACCOUNT" && (schemaLevel || domain === "DATABASE" || domain === "SCHEMA");
  const showSchema = schemaLevel;
  const showName = domain !== "ACCOUNT" && domain !== "DATABASE";
  const nameLabel = domain === "SCHEMA" ? "Schema name" : isColumn ? "Table or view" : "Object name";
  // For SCHEMA the name is itself a schema, so its dropdown reuses the schema
  // list; account-level objects (WAREHOUSE/ROLE/USER/INTEGRATION) have no cheap
  // listing, so their name is typed free-form.
  const nameFromObjects = schemaLevel;
  const nameFromSchemas = domain === "SCHEMA";
  const nameIsFreeText = showName && !nameFromObjects && !nameFromSchemas;

  // Object names matching the chosen domain (or any column-bearing object for
  // COLUMN). ListObjects reports each object's `kind`, which matches the domain
  // strings for the schema-level domains offered here.
  const objectOptions = useMemo(() => {
    const matches = objects.filter((o) =>
      isColumn ? COLUMN_PARENT_KINDS.has(o.kind) : o.kind === domain);
    return matches.map((o) => o.name).sort();
  }, [objects, domain, isColumn]);

  // The kind of the chosen parent object, carried into ObjectTagRef.parentKind so
  // a column on a view tags via ALTER VIEW rather than ALTER TABLE.
  const selectedParentKind = useMemo(
    () => objects.find((o) => o.name === objName)?.kind ?? "",
    [objects, objName],
  );

  const selectedTag = useMemo(
    () => catalog.find((c) => `${c.database}.${c.schema}.${c.name}` === tagKey) ?? null,
    [catalog, tagKey],
  );
  const allowedValues = selectedTag?.allowedValues ?? [];

  // Switching tags clears a value that the newly-selected tag may not permit.
  const onTagChange = (k: string) => { setTagKey(k); setValue(""); };

  // ── Cascading loads ──
  // Databases, once.
  useEffect(() => {
    setDbLoading(true);
    ListDatabases().then((d) => setDatabases(d ?? [])).catch(() => setDatabases([])).finally(() => setDbLoading(false));
  }, []);

  // Schemas when the database changes (needed for schema-level objects and the
  // SCHEMA / nested cases). ListUserSchemas omits INFORMATION_SCHEMA, whose
  // objects are read-only system views that cannot carry tags.
  useEffect(() => {
    if (!database || (!showSchema && !nameFromSchemas)) { setSchemas([]); return; }
    let live = true;
    setSchemaLoading(true);
    ListUserSchemas(database)
      .then((s) => { if (live) setSchemas(s ?? []); })
      .catch(() => { if (live) setSchemas([]); })
      .finally(() => { if (live) setSchemaLoading(false); });
    return () => { live = false; };
  }, [database, showSchema, nameFromSchemas]);

  // Objects when (database, schema) is set for a schema-level domain.
  useEffect(() => {
    if (!nameFromObjects || !database || !schema) { setObjects([]); return; }
    let live = true;
    setObjLoading(true);
    ListObjects(database, schema)
      .then((o) => { if (live) setObjects(o ?? []); })
      .catch(() => { if (live) setObjects([]); })
      .finally(() => { if (live) setObjLoading(false); });
    return () => { live = false; };
  }, [nameFromObjects, database, schema]);

  // Columns when a table is chosen for the COLUMN domain.
  useEffect(() => {
    if (!isColumn || !database || !schema || !objName) { setColumns([]); return; }
    let live = true;
    setColLoading(true);
    GetTableColumns(database, schema, objName)
      .then((c) => { if (live) setColumns(c ?? []); })
      .catch(() => { if (live) setColumns([]); })
      .finally(() => { if (live) setColLoading(false); });
    return () => { live = false; };
  }, [isColumn, database, schema, objName]);

  // ── Reset dependent selections on upstream change ──
  const onDomainChange = (d: string) => {
    setDomain(d);
    setSchema(""); setObjName(""); setColumn("");
  };
  const onDatabaseChange = (d: string) => {
    setDatabase(d);
    setSchema(""); setObjName(""); setColumn("");
  };
  const onSchemaChange = (s: string) => {
    setSchema(s);
    setObjName(""); setColumn("");
  };
  const onObjNameChange = (n: string) => {
    setObjName(n);
    setColumn("");
  };

  // A value and every identifying field shown for the chosen domain must be set.
  const canApply = !!selectedTag
    && !!value.trim()
    && (!showDatabase || !!database.trim())
    && (!showSchema || !!schema.trim())
    && (!showName || !!objName.trim())
    && (!isColumn || !!column.trim());

  const apply = async () => {
    if (!selectedTag) { setError("Select a tag to apply."); return; }
    setBusy(true);
    setError(null);
    try {
      const ref = tagModels.ObjectTagRef.createFrom({
        domain,
        database: showDatabase ? database : "",
        schema: showSchema ? schema : "",
        name: domain === "DATABASE" ? database : (showName ? objName : ""),
        column: isColumn ? column : "",
        parentKind: isColumn ? selectedParentKind : "",
      });
      await SetObjectTag(ref, selectedTag.database, selectedTag.schema, selectedTag.name, value);
      message.success("Tag applied");
      onApplied();
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const toOptions = (vals: string[]) => vals.map((v) => ({ value: v, label: v }));
  const selectStyle = { width: "100%" } as const;

  return (
    <Modal
      open
      title={<Space size={6}><PlusOutlined /><span>Apply tag to object</span></Space>}
      onCancel={onClose}
      width={560}
      okText="Apply"
      onOk={apply}
      confirmLoading={busy}
      okButtonProps={{ disabled: !canApply }}
    >
      {error && <Alert type="error" message={error} showIcon closable onClose={() => setError(null)} style={{ marginBottom: 12 }} />}
      <Form layout="vertical" size="small">
        <Form.Item label="Tag" help={catalog.length ? undefined : "Load the tag catalog first."}>
          <Select
            showSearch
            value={tagKey}
            onChange={onTagChange}
            placeholder="Select a tag"
            optionFilterProp="label"
            options={catalog.map((c) => ({
              value: `${c.database}.${c.schema}.${c.name}`,
              label: `${c.database}.${c.schema}.${c.name}`,
            }))}
          />
        </Form.Item>

        <Form.Item label="Object domain">
          <Select
            value={domain}
            onChange={onDomainChange}
            options={COMMON_DOMAINS.map((d) => ({ value: d, label: d }))}
          />
        </Form.Item>

        {domain !== "ACCOUNT" && (
          <Space style={{ width: "100%" }} align="start" size={8}>
            {showDatabase && (
              <Form.Item label="Database" style={{ flex: 1, minWidth: 0 }}>
                <Select
                  showSearch value={database || undefined} onChange={onDatabaseChange}
                  placeholder="Database" optionFilterProp="label" loading={dbLoading}
                  options={toOptions(databases)} style={selectStyle}
                />
              </Form.Item>
            )}
            {showSchema && (
              <Form.Item label="Schema" style={{ flex: 1, minWidth: 0 }}>
                <Select
                  showSearch value={schema || undefined} onChange={onSchemaChange}
                  placeholder="Schema" optionFilterProp="label" loading={schemaLoading}
                  disabled={!database} options={toOptions(schemas)} style={selectStyle}
                />
              </Form.Item>
            )}
            {showName && (
              <Form.Item label={nameLabel} style={{ flex: 1, minWidth: 0 }}>
                {nameIsFreeText ? (
                  <Input value={objName} onChange={(e) => onObjNameChange(e.target.value)} placeholder="NAME" />
                ) : (
                  <Select
                    showSearch value={objName || undefined} onChange={onObjNameChange}
                    placeholder={nameLabel} optionFilterProp="label"
                    loading={nameFromObjects ? objLoading : schemaLoading}
                    disabled={nameFromObjects ? (!database || !schema) : !database}
                    options={toOptions(nameFromSchemas ? schemas : objectOptions)} style={selectStyle}
                  />
                )}
              </Form.Item>
            )}
          </Space>
        )}

        {isColumn && (
          <Form.Item label="Column">
            <Select
              showSearch value={column || undefined} onChange={setColumn}
              placeholder="Column" optionFilterProp="label" loading={colLoading}
              disabled={!objName} options={toOptions(columns)} style={selectStyle}
            />
          </Form.Item>
        )}

        <Form.Item
          label="Value"
          required
          help={allowedValues.length > 0
            ? "This tag restricts values to its allowed list."
            : "The tag value to assign."}
        >
          {allowedValues.length > 0 ? (
            <Select
              showSearch value={value || undefined} onChange={setValue}
              placeholder="Select an allowed value" optionFilterProp="label"
              options={toOptions(allowedValues)} style={selectStyle}
            />
          ) : (
            <Input value={value} onChange={(e) => setValue(e.target.value)} placeholder="value" />
          )}
        </Form.Item>
      </Form>
    </Modal>
  );
}

// ─── Editable tag-value cell ─────────────────────────────────────────────────

function ValueCell({ row, allowedValues, onSave, busy }: {
  row: RefRow; allowedValues: string[]; onSave: (r: RefRow, v: string) => Promise<void>; busy: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(row.tagValue);

  if (!editing) {
    return (
      <Space size={4}>
        <span>{row.tagValue || <Text type="secondary">(empty)</Text>}</span>
        <Tooltip title="Edit value">
          <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />}
            onClick={() => { setDraft(row.tagValue); setEditing(true); }} style={{ color: "var(--text-muted)" }} />
        </Tooltip>
      </Space>
    );
  }
  const commit = async () => { await onSave(row, draft); setEditing(false); };
  return (
    <Space size={4}>
      {allowedValues.length > 0 ? (
        <Select
          size="small" showSearch autoFocus value={draft || undefined} onChange={setDraft}
          placeholder="Allowed value" optionFilterProp="label" style={{ width: 150 }}
          options={allowedValues.map((v) => ({ value: v, label: v }))}
        />
      ) : (
        <Input size="small" value={draft} onChange={(e) => setDraft(e.target.value)} style={{ width: 140 }}
          onPressEnter={commit} autoFocus />
      )}
      <Tooltip title="Save">
        <Button size="small" type="primary" icon={<CheckOutlined />} loading={busy} onClick={commit} />
      </Tooltip>
      <Tooltip title="Cancel">
        <Button size="small" icon={<CloseOutlined />} onClick={() => setEditing(false)} />
      </Tooltip>
    </Space>
  );
}

// ─── Main modal ──────────────────────────────────────────────────────────────

interface Props { onClose: () => void }

export default function TagManagementModal({ onClose }: Props) {
  const [activeTab, setActiveTab] = useState("references");

  // Catalog (SHOW TAGS IN ACCOUNT)
  const [catalog, setCatalog] = useState<snowflake.QueryResult | null>(null);
  const [catalogErr, setCatalogErr] = useState<string | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogSearch, setCatalogSearch] = useState("");

  // References (ACCOUNT_USAGE.TAG_REFERENCES)
  const [refs, setRefs] = useState<snowflake.QueryResult | null>(null);
  const [refsErr, setRefsErr] = useState<string | null>(null);
  const [refsLoading, setRefsLoading] = useState(false);
  const [busyKey, setBusyKey] = useState<string | null>(null);

  // Reference filters
  const [fTag, setFTag] = useState<string | null>(null);
  const [fValue, setFValue] = useState("");
  const [fDatabase, setFDatabase] = useState<string | null>(null);
  const [fDomain, setFDomain] = useState<string | null>(null);
  const [fSearch, setFSearch] = useState("");

  const [applyOpen, setApplyOpen] = useState(false);

  const loadCatalog = useCallback(async () => {
    setCatalogLoading(true);
    setCatalogErr(null);
    try {
      setCatalog(await ListAccountTags());
    } catch (e) {
      setCatalogErr(String(e));
    } finally {
      setCatalogLoading(false);
    }
  }, []);

  const loadRefs = useCallback(async () => {
    setRefsLoading(true);
    setRefsErr(null);
    try {
      setRefs(await GetAllTagReferences());
    } catch (e) {
      setRefsErr(String(e));
    } finally {
      setRefsLoading(false);
    }
  }, []);

  // Load both on first open. The catalog also feeds the Apply form's tag picker.
  useEffect(() => { loadRefs(); loadCatalog(); }, [loadRefs, loadCatalog]);

  const allRefRows = useMemo(() => toRefRows(refs), [refs]);

  const catalogTags: CatalogTag[] = useMemo(() => {
    if (!catalog?.rows) return [];
    const nm = colIdx(catalog, "name");
    const db = colIdx(catalog, "database_name");
    const sc = colIdx(catalog, "schema_name");
    const av = colIdx(catalog, "allowed_values");
    return catalog.rows.map((r) => ({
      database: cell(r, db), schema: cell(r, sc), name: cell(r, nm),
      allowedValues: parseAllowedValues(cell(r, av)),
    }));
  }, [catalog]);

  // Lookup of a tag's allowed values by `db.schema.name`, so reference rows can
  // offer them when editing a value inline.
  const allowedValuesByTag = useMemo(() => {
    const m = new Map<string, string[]>();
    catalogTags.forEach((t) => m.set(`${t.database}.${t.schema}.${t.name}`, t.allowedValues));
    return m;
  }, [catalogTags]);

  // Distinct filter option sets, derived from the reference rows.
  const tagOptions = useMemo(
    () => Array.from(new Set(allRefRows.map((r) => r.tagName).filter(Boolean))).sort(),
    [allRefRows],
  );
  const dbOptions = useMemo(
    () => Array.from(new Set(allRefRows.map((r) => r.objectDatabase).filter(Boolean))).sort(),
    [allRefRows],
  );
  const domainOptions = useMemo(
    () => Array.from(new Set(allRefRows.map((r) => r.domain).filter(Boolean))).sort(),
    [allRefRows],
  );

  const filteredRefs = useMemo(() => {
    const search = fSearch.trim().toLowerCase();
    const val = fValue.trim().toLowerCase();
    return allRefRows.filter((r) => {
      if (fTag && r.tagName !== fTag) return false;
      if (fDatabase && r.objectDatabase !== fDatabase) return false;
      if (fDomain && r.domain !== fDomain) return false;
      if (val && !r.tagValue.toLowerCase().includes(val)) return false;
      if (search) {
        const hay = `${r.tagName} ${r.tagValue} ${objectPath(r)} ${r.domain}`.toLowerCase();
        if (!hay.includes(search)) return false;
      }
      return true;
    });
  }, [allRefRows, fTag, fValue, fDatabase, fDomain, fSearch]);

  const clearFilters = () => { setFTag(null); setFValue(""); setFDatabase(null); setFDomain(null); setFSearch(""); };
  const anyFilter = fTag || fValue || fDatabase || fDomain || fSearch;

  const saveValue = async (r: RefRow, v: string) => {
    setBusyKey(r.key);
    try {
      await SetObjectTag(refObjectRef(r), r.tagDatabase, r.tagSchema, r.tagName, v);
      message.success("Tag value updated — references may take a moment to refresh.");
      await loadRefs();
    } catch (e) {
      message.error(String(e));
    } finally {
      setBusyKey(null);
    }
  };

  const removeTag = async (r: RefRow) => {
    setBusyKey(r.key);
    try {
      await UnsetObjectTag(refObjectRef(r), r.tagDatabase, r.tagSchema, r.tagName);
      message.success("Tag removed — references may take a moment to refresh.");
      await loadRefs();
    } catch (e) {
      message.error(String(e));
    } finally {
      setBusyKey(null);
    }
  };

  // ── Catalog tab ──
  const catalogColumns = (catalog?.columns ?? []).map((c, ci) => ({
    title: c, dataIndex: ci, key: String(ci), ellipsis: true,
    render: (v: unknown) => (v === null || v === undefined ? "" : String(v)),
  }));
  const filteredCatalog = useMemo(() => {
    if (!catalog?.rows) return [];
    const s = catalogSearch.trim().toLowerCase();
    const colCount = catalog.columns?.length ?? 0;
    const rows = catalog.rows.map((row, n) => {
      const obj: Record<string, unknown> = { __k: n };
      row.forEach((c, ci) => { obj[ci] = c; });
      return obj;
    });
    if (!s) return rows;
    // Match data columns only (skip the synthetic __k row-index key, which would
    // otherwise let a digit search hit rows by their position).
    return rows.filter((o) => {
      for (let ci = 0; ci < colCount; ci++) {
        const v = o[ci];
        if (v !== null && v !== undefined && String(v).toLowerCase().includes(s)) return true;
      }
      return false;
    });
  }, [catalog, catalogSearch]);

  const catalogTab = (
    <>
      <Space style={{ marginBottom: 8 }} wrap>
        <Input
          size="small" allowClear prefix={<SearchOutlined />} placeholder="Search tags…"
          value={catalogSearch} onChange={(e) => setCatalogSearch(e.target.value)} style={{ width: 240 }}
        />
        <Button size="small" icon={<ReloadOutlined />} onClick={loadCatalog} loading={catalogLoading}>Refresh</Button>
        <Text type="secondary" style={{ fontSize: 12 }}>{filteredCatalog.length} tag(s)</Text>
      </Space>
      {catalogErr && <Alert type="warning" message="Could not load tags" description={catalogErr} showIcon style={{ marginBottom: 8 }} />}
      {catalog ? (
        <Table
          size="small" rowKey="__k" loading={catalogLoading}
          columns={catalogColumns} dataSource={filteredCatalog}
          pagination={{ pageSize: 15, showSizeChanger: false, size: "small" }}
          scroll={{ y: 380, x: "max-content" }}
        />
      ) : (
        catalogLoading ? null : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No tags loaded" />
      )}
    </>
  );

  // ── References tab ──
  const refColumns = [
    {
      title: "Tag", key: "tag", width: 160, ellipsis: true,
      render: (_: unknown, r: RefRow) => (
        <Tooltip title={`${r.tagDatabase}.${r.tagSchema}.${r.tagName}`}>
          <Tag color="blue" style={{ fontSize: 11 }}>{r.tagName}</Tag>
        </Tooltip>
      ),
    },
    {
      title: "Value", key: "value", width: 200,
      render: (_: unknown, r: RefRow) => (
        <ValueCell
          row={r}
          allowedValues={allowedValuesByTag.get(`${r.tagDatabase}.${r.tagSchema}.${r.tagName}`) ?? []}
          onSave={saveValue}
          busy={busyKey === r.key}
        />
      ),
    },
    {
      title: "Object", key: "object", ellipsis: true,
      render: (_: unknown, r: RefRow) => <Text style={{ fontFamily: "var(--font-mono, monospace)", fontSize: 12 }}>{objectPath(r)}</Text>,
    },
    {
      title: "Domain", dataIndex: "domain", key: "domain", width: 130,
      render: (v: string) => <Text type="secondary" style={{ fontSize: 12 }}>{v}</Text>,
    },
    {
      title: "", key: "actions", width: 48,
      render: (_: unknown, r: RefRow) => (
        <Popconfirm
          title="Remove this tag from the object?"
          okText="Remove" okButtonProps={{ danger: true }}
          onConfirm={() => removeTag(r)}
        >
          <Tooltip title="Remove tag">
            <Button type="text" size="small" danger icon={<DeleteOutlined style={{ fontSize: 12 }} />} />
          </Tooltip>
        </Popconfirm>
      ),
    },
  ];

  const referencesTab = (
    <>
      <Space style={{ marginBottom: 8 }} wrap size={[8, 8]}>
        <Select
          size="small" allowClear showSearch placeholder="Tag" style={{ width: 160 }}
          value={fTag} onChange={setFTag}
          options={tagOptions.map((t) => ({ value: t, label: t }))}
        />
        <Input
          size="small" allowClear placeholder="Value contains…" style={{ width: 150 }}
          value={fValue} onChange={(e) => setFValue(e.target.value)}
        />
        <Select
          size="small" allowClear showSearch placeholder="Database" style={{ width: 150 }}
          value={fDatabase} onChange={setFDatabase}
          options={dbOptions.map((d) => ({ value: d, label: d }))}
        />
        <Select
          size="small" allowClear showSearch placeholder="Domain" style={{ width: 150 }}
          value={fDomain} onChange={setFDomain}
          options={domainOptions.map((d) => ({ value: d, label: d }))}
        />
        <Input
          size="small" allowClear prefix={<SearchOutlined />} placeholder="Search…" style={{ width: 170 }}
          value={fSearch} onChange={(e) => setFSearch(e.target.value)}
        />
        {anyFilter && <Button size="small" onClick={clearFilters}>Clear</Button>}
        <Button size="small" icon={<ReloadOutlined />} onClick={loadRefs} loading={refsLoading}>Refresh</Button>
        <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => setApplyOpen(true)}>Apply tag…</Button>
      </Space>

      <Space style={{ marginBottom: 8 }} size={8}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {filteredRefs.length} of {allRefRows.length} reference(s)
        </Text>
        {refs?.truncated && (
          <Text type="warning" style={{ fontSize: 12 }}>· results capped — narrow the scope to see more</Text>
        )}
      </Space>

      {refsErr && <Alert type="warning" message="Could not load tag references" description={refsErr} showIcon style={{ marginBottom: 8 }} />}

      <Table<RefRow>
        size="small" rowKey="key" loading={refsLoading}
        columns={refColumns} dataSource={filteredRefs}
        pagination={{ pageSize: 15, showSizeChanger: false, size: "small" }}
        scroll={{ y: 360 }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No tag references" /> }}
      />
    </>
  );

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <TagsOutlined style={{ color: "var(--link)" }} />
          <span>Tag Management</span>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={1080}
      styles={{ body: { maxHeight: "76vh", overflowY: "auto", paddingTop: 8 } }}
    >
      <Alert
        type="info" showIcon banner style={{ marginBottom: 10, fontSize: 12 }}
        message="Tag references come from SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES — they require governance privileges and can lag recent changes by a few minutes."
      />
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          { key: "references", label: "References", children: referencesTab },
          { key: "catalog", label: "Tag catalog", children: catalogTab },
        ]}
      />
      {applyOpen && (
        <ApplyTagModal
          catalog={catalogTags}
          onClose={() => setApplyOpen(false)}
          onApplied={loadRefs}
        />
      )}
    </Modal>
  );
}
