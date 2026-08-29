// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { Button, Checkbox, Input, InputNumber, Select, Typography } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { ListObjects, ListUserSchemas, GetTableColumns } from "../../../wailsjs/go/app/App";
import type { semanticview, snowflake } from "../../../wailsjs/go/models";
import TagInput from "../shared/TagInput";
import type { TagItem } from "../shared/TagInput";
import { quoteFqn, parseQuotedFqn } from "./semanticViewNames";

const { Text } = Typography;

// Plain data shapes for form state. The Wails-generated classes carry a
// `convertValues` method a plain object literal can't satisfy; the config is
// cast to the generated type only at the IPC boundary.
export type SemTable = Omit<semanticview.LogicalTable, "convertValues">;
export type SemRelationship = semanticview.Relationship;
export type SemExpression = Omit<semanticview.Expression, "convertValues">;
export type SemNonAdditive = semanticview.NonAdditiveDim;
export type SemVerifiedQuery = semanticview.VerifiedQuery;

/** The clause an ExpressionsSection edits — gates the clause-specific fields. */
export type ExpressionKind = "FACTS" | "DIMENSIONS" | "METRICS";

/**
 * A TABLES row as the form holds it: the picked database/schema/table parts
 * (needed to fetch the table's columns) alongside the logical-table properties.
 * `name` is derived from the parts at the IPC boundary via `toLogicalTable`. A
 * semantic view may reference tables in any database, so each row carries its
 * own database rather than inheriting the view's.
 */
export interface TableRow extends Omit<SemTable, "name"> {
  db: string;
  schema: string;
  table: string;
}

export const emptyTableRow = (db: string, schema: string): TableRow => ({
  db, schema, table: "", alias: "",
  primaryKey: [], unique: [],
  constraintName: "", rangeStart: "", rangeEnd: "",
  synonyms: [], comment: "", tags: [],
});

export const emptyRelationship = (): SemRelationship => ({
  name: "", table: "", columns: [], refTable: "", refColumns: [],
  joinType: "", rangeStart: "", rangeEnd: "",
});

export const emptyExpression = (): SemExpression => ({
  visibility: "", tableAlias: "", name: "", filterLabel: false,
  using: [], nonAdditiveBy: [], expr: "",
  synonyms: [], tags: [], comment: "",
  cortexSearchService: "", cortexSearchColumn: "",
});

export const emptyVerifiedQuery = (): SemVerifiedQuery => ({
  name: "", question: "", verifiedAt: "", onboardingQuestion: false,
  verifiedBy: "", sql: "",
});

/** The alias a row is referenced by — the explicit alias, else the table name. */
export const aliasOf = (r: TableRow) => (r.alias.trim() || r.table);

/** Converts a form row to the builder's LogicalTable (quoted 3-part name). */
export const toLogicalTable = (r: TableRow): SemTable => ({
  ...r,
  name: quoteFqn(r.db, r.schema, r.table),
});

const colKey = (r: TableRow) => (r.db && r.schema && r.table ? quoteFqn(r.db, r.schema, r.table) : "");

/**
 * Fetches and caches the column list of every picked logical table, so the
 * PRIMARY KEY / UNIQUE / relationship pickers can offer real columns. Each
 * table is fetched at most once per modal instance, keyed by its full
 * database.schema.table path (rows may point at different databases).
 */
export function useTableColumns(rows: TableRow[]) {
  const [cache, setCache] = useState<Record<string, string[]>>({});
  const requested = useRef<Set<string>>(new Set());

  useEffect(() => {
    for (const r of rows) {
      const key = colKey(r);
      if (!key || requested.current.has(key)) continue;
      requested.current.add(key);
      GetTableColumns(r.db, r.schema, r.table)
        .then((cols) => setCache((c) => ({ ...c, [key]: cols ?? [] })))
        .catch(() => {});
    }
  }, [rows]);

  /** Columns of the row with this alias (empty until the fetch resolves). */
  return (alias: string): string[] => {
    const row = rows.find((r) => aliasOf(r) === alias);
    return row ? cache[colKey(row)] ?? [] : [];
  };
}

const ROW_STYLE: React.CSSProperties = {
  display: "flex", alignItems: "flex-start", gap: 8,
  border: "1px solid var(--border)", borderRadius: 6,
  padding: 8, marginBottom: 8,
};

/** One editable list entry: its fields, plus a remove button. */
function RowCard({ children, onRemove }: { children: ReactNode; onRemove: () => void }) {
  return (
    <div style={ROW_STYLE}>
      <div style={{ flex: 1, display: "flex", flexWrap: "wrap", gap: 8, alignItems: "center" }}>
        {children}
      </div>
      <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onRemove} />
    </div>
  );
}

/** A labelled clause section with an empty-state hint and an Add button. */
function Section({
  label, help, empty, addText, onAdd, children,
}: {
  label: string; help?: string; empty: boolean; addText: string;
  onAdd: () => void; children: ReactNode;
}) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ display: "flex", alignItems: "baseline", gap: 8, marginBottom: 6 }}>
        <Text strong style={{ fontSize: 12 }}>{label}</Text>
        {help && <Text type="secondary" style={{ fontSize: 11 }}>{help}</Text>}
      </div>
      {children}
      {empty && (
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 6 }}>
          (none)
        </Text>
      )}
      <Button size="small" icon={<PlusOutlined />} onClick={onAdd}>{addText}</Button>
    </div>
  );
}

const opts = (values: string[]) => values.map((v) => ({ value: v, label: v }));

/**
 * Database → schema → object cascade. A semantic view can reference objects in
 * any database the role can see, so the database is picked per row rather than
 * fixed to the view's own. `dbOptions` is loaded once by the modal; the schema
 * and object lists load on demand as the cascade is filled in. `kinds` filters
 * the object list (tables/views for the logical tables, Cortex search services
 * for a dimension's search binding).
 */
function ObjectPicker({
  dbOptions, db, schema, name, kinds, placeholder, onChange, width = 190,
}: {
  dbOptions: string[]; db: string; schema: string; name: string;
  kinds: string[]; placeholder: string;
  onChange: (db: string, schema: string, name: string) => void;
  width?: number;
}) {
  const [schemas, setSchemas] = useState<string[]>([]);
  const [objects, setObjects] = useState<snowflake.SnowflakeObject[]>([]);
  const [loadingSchemas, setLoadingSchemas] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!db) { setSchemas([]); return; }
    setLoadingSchemas(true);
    ListUserSchemas(db)
      .then((s) => setSchemas(s ?? []))
      .catch(() => setSchemas([]))
      .finally(() => setLoadingSchemas(false));
  }, [db]);

  useEffect(() => {
    if (!db || !schema) { setObjects([]); return; }
    setLoading(true);
    ListObjects(db, schema)
      .then((o) => setObjects(o ?? []))
      .catch(() => setObjects([]))
      .finally(() => setLoading(false));
  }, [db, schema]);

  return (
    <>
      <Select
        size="small" showSearch placeholder="Database" style={{ width: 150 }}
        value={db || undefined}
        onChange={(v) => onChange(v ?? "", "", "")}
        options={opts(dbOptions)}
      />
      <Select
        size="small" showSearch placeholder="Schema" style={{ width: 150 }}
        value={schema || undefined}
        onChange={(v) => onChange(db, v ?? "", "")}
        disabled={!db}
        loading={loadingSchemas}
        options={opts(schemas)}
      />
      <Select
        size="small" showSearch placeholder={placeholder} style={{ width }}
        value={name || undefined}
        onChange={(v) => onChange(db, schema, v ?? "")}
        disabled={!schema}
        loading={loading}
        options={opts(objects.filter((o) => kinds.includes(o.kind)).map((o) => o.name))}
        notFoundContent={loading ? "Loading…" : `No ${placeholder.toLowerCase()}`}
      />
    </>
  );
}

/**
 * TABLES ( … ) editor: a database/schema/table picker per row plus its alias,
 * PRIMARY KEY and UNIQUE column multi-selects, synonyms, comment, tags, and the
 * preview-only CONSTRAINT … DISTINCT RANGE bounds. Rows may point at any
 * database. Picking a table seeds a blank alias with the table name, so every
 * row is referenceable by the other sections.
 */
export function TablesSection({
  dbOptions, defaultDb, defaultSchema, rows, onChange, columnsFor,
}: {
  dbOptions: string[]; defaultDb: string; defaultSchema: string; rows: TableRow[];
  onChange: (rows: TableRow[]) => void;
  columnsFor: (alias: string) => string[];
}) {
  const patch = (i: number, next: Partial<TableRow>) =>
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...next } : r)));

  return (
    <Section
      label="TABLES"
      help="The physical tables and views the semantic layer is built on — from any database."
      empty={rows.length === 0}
      addText="Add table"
      onAdd={() => onChange([...rows, emptyTableRow(defaultDb, defaultSchema)])}
    >
      {rows.map((r, i) => {
        const columns = columnsFor(aliasOf(r));
        return (
          <RowCard key={i} onRemove={() => onChange(rows.filter((_, idx) => idx !== i))}>
            <ObjectPicker
              dbOptions={dbOptions} db={r.db} schema={r.schema} name={r.table}
              kinds={["TABLE", "VIEW", "MATERIALIZED VIEW", "DYNAMIC TABLE", "EXTERNAL TABLE"]}
              placeholder="Table / view"
              onChange={(db, schema, table) =>
                // Seed the alias from the table name unless the user set one.
                patch(i, {
                  db, schema, table,
                  alias: !r.alias || r.alias === r.table ? table : r.alias,
                  primaryKey: [], unique: [], rangeStart: "", rangeEnd: "",
                })
              }
            />
            <Input
              size="small" style={{ width: 130 }} placeholder="alias"
              value={r.alias} onChange={(e) => patch(i, { alias: e.target.value })}
            />
            <Select
              size="small" mode="multiple" allowClear style={{ minWidth: 180 }}
              placeholder="PRIMARY KEY" value={r.primaryKey}
              onChange={(v) => patch(i, { primaryKey: v })}
              options={opts(columns)}
            />
            <Select
              size="small" mode="multiple" allowClear style={{ minWidth: 160 }}
              placeholder="UNIQUE" value={r.unique}
              onChange={(v) => patch(i, { unique: v })}
              options={opts(columns)}
            />
            <Select
              size="small" mode="tags" allowClear style={{ minWidth: 170 }}
              placeholder="WITH SYNONYMS" value={r.synonyms}
              onChange={(v) => patch(i, { synonyms: v })}
              options={[]}
            />
            <Input
              size="small" style={{ width: 200 }} placeholder="COMMENT"
              value={r.comment} onChange={(e) => patch(i, { comment: e.target.value })}
            />
            {/* CONSTRAINT … DISTINCT RANGE is a Snowflake preview feature and is
                emitted only when both bounds are picked. */}
            <Select
              size="small" allowClear style={{ width: 165 }}
              placeholder="RANGE start (preview)" value={r.rangeStart || undefined}
              onChange={(v) => patch(i, { rangeStart: v ?? "" })}
              options={opts(columns)}
            />
            <Select
              size="small" allowClear style={{ width: 150 }}
              placeholder="RANGE end" value={r.rangeEnd || undefined}
              onChange={(v) => patch(i, { rangeEnd: v ?? "" })}
              options={opts(columns)}
            />
            <Input
              size="small" style={{ width: 150 }} placeholder="constraint name"
              value={r.constraintName} onChange={(e) => patch(i, { constraintName: e.target.value })}
            />
            <div style={{ flexBasis: "100%" }}>
              <TagInput tags={r.tags as TagItem[]} onChange={(t) => patch(i, { tags: t })} label="" />
            </div>
          </RowCard>
        );
      })}
    </Section>
  );
}

const JOIN_TYPES = [
  { value: "", label: "Standard" },
  { value: "ASOF", label: "ASOF (preview)" },
  { value: "BETWEEN", label: "BETWEEN … EXCLUSIVE (preview)" },
];

/**
 * RELATIONSHIPS ( … ) editor. The referenced columns are constrained to the
 * target table's PRIMARY KEY / UNIQUE columns (Snowflake requires the reference
 * to hit a declared key); when the target declares neither, every column of that
 * table is offered so the row can still be completed.
 */
export function RelationshipsSection({
  rows, tables, onChange, columnsFor,
}: {
  rows: SemRelationship[]; tables: TableRow[];
  onChange: (rows: SemRelationship[]) => void;
  columnsFor: (alias: string) => string[];
}) {
  const aliases = tables.map(aliasOf).filter(Boolean);
  const patch = (i: number, next: Partial<SemRelationship>) =>
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...next } : r)));

  const keyColumnsOf = (alias: string) => {
    const row = tables.find((t) => aliasOf(t) === alias);
    const keys = [...(row?.primaryKey ?? []), ...(row?.unique ?? [])];
    return keys.length > 0 ? keys : columnsFor(alias);
  };

  return (
    <Section
      label="RELATIONSHIPS"
      help="How the logical tables join. Referenced columns must be a PRIMARY KEY or UNIQUE key of the target."
      empty={rows.length === 0}
      addText="Add relationship"
      onAdd={() => onChange([...rows, emptyRelationship()])}
    >
      {rows.map((r, i) => (
        <RowCard key={i} onRemove={() => onChange(rows.filter((_, idx) => idx !== i))}>
          <Input
            size="small" style={{ width: 150 }} placeholder="relationship name"
            value={r.name} onChange={(e) => patch(i, { name: e.target.value })}
          />
          <Select
            size="small" showSearch style={{ width: 150 }} placeholder="from table"
            value={r.table || undefined}
            onChange={(v) => patch(i, { table: v ?? "", columns: [] })}
            options={opts(aliases)}
          />
          <Select
            size="small" mode="multiple" allowClear style={{ minWidth: 180 }}
            placeholder="columns" value={r.columns}
            onChange={(v) => patch(i, { columns: v })}
            options={opts(columnsFor(r.table))}
          />
          <Text type="secondary" style={{ fontSize: 11 }}>REFERENCES</Text>
          <Select
            size="small" showSearch style={{ width: 150 }} placeholder="to table"
            value={r.refTable || undefined}
            onChange={(v) => patch(i, { refTable: v ?? "", refColumns: [], rangeStart: "", rangeEnd: "" })}
            options={opts(aliases)}
          />
          <Select
            size="small" style={{ width: 200 }} value={r.joinType}
            onChange={(v) => patch(i, { joinType: v, refColumns: [], rangeStart: "", rangeEnd: "" })}
            options={JOIN_TYPES}
          />
          {r.joinType === "BETWEEN" ? (
            <>
              <Select
                size="small" allowClear style={{ width: 150 }} placeholder="range start"
                value={r.rangeStart || undefined}
                onChange={(v) => patch(i, { rangeStart: v ?? "" })}
                options={opts(columnsFor(r.refTable))}
              />
              <Select
                size="small" allowClear style={{ width: 150 }} placeholder="range end"
                value={r.rangeEnd || undefined}
                onChange={(v) => patch(i, { rangeEnd: v ?? "" })}
                options={opts(columnsFor(r.refTable))}
              />
            </>
          ) : (
            <Select
              size="small" mode="multiple" allowClear style={{ minWidth: 180 }}
              placeholder="referenced key columns" value={r.refColumns}
              onChange={(v) => patch(i, { refColumns: v })}
              options={opts(keyColumnsOf(r.refTable))}
            />
          )}
        </RowCard>
      ))}
    </Section>
  );
}

/** NON ADDITIVE BY ( … ) sub-editor: dimension + ASC/DESC + NULLS ordering. */
function NonAdditiveEditor({
  rows, dimensions, onChange,
}: {
  rows: SemNonAdditive[]; dimensions: string[];
  onChange: (rows: SemNonAdditive[]) => void;
}) {
  const patch = (i: number, next: Partial<SemNonAdditive>) =>
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...next } : r)));
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "center" }}>
      <Text type="secondary" style={{ fontSize: 11 }}>NON ADDITIVE BY</Text>
      {rows.map((r, i) => (
        <div key={i} style={{ display: "flex", gap: 4, alignItems: "center" }}>
          <Select
            size="small" showSearch style={{ width: 180 }} placeholder="dimension"
            value={r.dimension || undefined}
            onChange={(v) => patch(i, { dimension: v ?? "" })}
            options={opts(dimensions)}
          />
          <Select
            size="small" style={{ width: 90 }} value={r.direction}
            onChange={(v) => patch(i, { direction: v })}
            options={[{ value: "", label: "—" }, { value: "ASC", label: "ASC" }, { value: "DESC", label: "DESC" }]}
          />
          <Select
            size="small" style={{ width: 120 }} value={r.nulls}
            onChange={(v) => patch(i, { nulls: v })}
            options={[
              { value: "", label: "NULLS —" },
              { value: "FIRST", label: "NULLS FIRST" },
              { value: "LAST", label: "NULLS LAST" },
            ]}
          />
          <Button
            size="small" type="text" danger icon={<DeleteOutlined />}
            onClick={() => onChange(rows.filter((_, idx) => idx !== i))}
          />
        </div>
      ))}
      <Button size="small" icon={<PlusOutlined />} onClick={() => onChange([...rows, { dimension: "", direction: "", nulls: "" }])}>
        Add
      </Button>
    </div>
  );
}

const VISIBILITY = [
  { value: "", label: "Default" },
  { value: "PUBLIC", label: "PUBLIC" },
  { value: "PRIVATE", label: "PRIVATE" },
];

/**
 * FACTS / DIMENSIONS / METRICS editor. One component covers all three: the
 * grammars share the alias/name/expression/synonyms/tags/comment core, and the
 * `kind` prop gates the clause-specific controls — visibility (facts & metrics;
 * dimensions are always public), LABELS = (FILTER) (facts & dimensions), USING /
 * NON ADDITIVE BY (metrics), and the Cortex Search binding (dimensions).
 *
 * A window-function metric is written straight into the expression field —
 * `SUM(orders.amount) OVER (PARTITION BY … ORDER BY …)` — since Snowflake's
 * window-metric grammar adds no keywords around the expression itself.
 */
export function ExpressionsSection({
  kind, rows, tables, relationships, dimensionNames, dbOptions, onChange,
}: {
  kind: ExpressionKind;
  rows: SemExpression[];
  tables: TableRow[];
  relationships: SemRelationship[];
  dimensionNames: string[];
  dbOptions: string[];
  onChange: (rows: SemExpression[]) => void;
}) {
  const aliases = tables.map(aliasOf).filter(Boolean);
  const patch = (i: number, next: Partial<SemExpression>) =>
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...next } : r)));

  const isMetric = kind === "METRICS";
  const isDimension = kind === "DIMENSIONS";
  const relationshipNames = relationships.map((r) => r.name.trim()).filter(Boolean);

  const help = isMetric
    ? "Aggregations over the facts. Window metrics go in the expression: SUM(x) OVER (PARTITION BY … ORDER BY …)."
    : isDimension
      ? "Attributes to group and filter by. Dimensions are always public."
      : "Row-level numeric columns the metrics aggregate.";

  return (
    <Section
      label={kind}
      help={help}
      empty={rows.length === 0}
      addText={`Add ${kind.toLowerCase().replace(/s$/, "")}`}
      onAdd={() => onChange([...rows, emptyExpression()])}
    >
      {rows.map((r, i) => (
        <RowCard key={i} onRemove={() => onChange(rows.filter((_, idx) => idx !== i))}>
          {!isDimension && (
            <Select
              size="small" style={{ width: 110 }} value={r.visibility}
              onChange={(v) => patch(i, { visibility: v })}
              options={VISIBILITY}
            />
          )}
          <Select
            size="small" showSearch style={{ width: 150 }} placeholder="table alias"
            value={r.tableAlias || undefined}
            onChange={(v) => patch(i, { tableAlias: v ?? "" })}
            options={opts(aliases)}
          />
          <Input
            size="small" style={{ width: 160 }} placeholder="name"
            value={r.name} onChange={(e) => patch(i, { name: e.target.value })}
          />
          <Input
            size="small" style={{ minWidth: 260, flex: 1 }} placeholder="SQL expression (after AS)"
            value={r.expr} onChange={(e) => patch(i, { expr: e.target.value })}
          />
          {!isMetric && (
            <Checkbox
              checked={r.filterLabel}
              onChange={(e) => patch(i, { filterLabel: e.target.checked })}
            >
              <span style={{ fontSize: 11 }}>LABELS = (FILTER)</span>
            </Checkbox>
          )}
          <Select
            size="small" mode="tags" allowClear style={{ minWidth: 170 }}
            placeholder="WITH SYNONYMS" value={r.synonyms}
            onChange={(v) => patch(i, { synonyms: v })}
            options={[]}
          />
          <Input
            size="small" style={{ width: 200 }} placeholder="COMMENT"
            value={r.comment} onChange={(e) => patch(i, { comment: e.target.value })}
          />
          {isMetric && (
            <>
              <Select
                size="small" mode="multiple" allowClear style={{ minWidth: 200 }}
                placeholder="USING relationships (preview)" value={r.using}
                onChange={(v) => patch(i, { using: v })}
                options={opts(relationshipNames)}
              />
              <div style={{ flexBasis: "100%" }}>
                <NonAdditiveEditor
                  rows={r.nonAdditiveBy}
                  dimensions={dimensionNames}
                  onChange={(v) => patch(i, { nonAdditiveBy: v })}
                />
              </div>
            </>
          )}
          {isDimension && (
            <>
              <Text type="secondary" style={{ fontSize: 11 }}>WITH CORTEX SEARCH SERVICE</Text>
              <CortexSearchPicker
                dbOptions={dbOptions} value={r.cortexSearchService}
                onChange={(v) => patch(i, { cortexSearchService: v })}
              />
              <Input
                size="small" style={{ width: 150 }} placeholder="USING column"
                value={r.cortexSearchColumn}
                onChange={(e) => patch(i, { cortexSearchColumn: e.target.value })}
              />
            </>
          )}
          <div style={{ flexBasis: "100%" }}>
            <TagInput tags={r.tags as TagItem[]} onChange={(t) => patch(i, { tags: t })} label="" />
          </div>
        </RowCard>
      ))}
    </Section>
  );
}

/**
 * Picks a Cortex search service (from any database) and hands back its quoted,
 * fully-qualified name — the builder emits it verbatim. The cascade's parts are
 * parsed back out of that reference rather than held locally, so the control
 * stays in sync with its row when rows above it are removed.
 */
function CortexSearchPicker({
  dbOptions, value, onChange,
}: {
  dbOptions: string[]; value: string; onChange: (v: string) => void;
}) {
  const picked = parseQuotedFqn(value);
  // A database/schema with no service picked yet has nothing to store in the
  // reference, so the partial cascade is held until a service is chosen.
  const [pending, setPending] = useState({ db: "", schema: "" });

  return (
    <ObjectPicker
      dbOptions={dbOptions}
      db={picked.db || pending.db}
      schema={picked.schema || pending.schema}
      name={picked.name}
      kinds={["CORTEX SEARCH SERVICE"]} placeholder="Search service"
      onChange={(db, schema, name) => {
        setPending({ db, schema });
        onChange(quoteFqn(db, schema, name));
      }}
    />
  );
}

/**
 * AI_VERIFIED_QUERIES ( … ) editor — question/SQL pairs Cortex Analyst can reuse
 * verbatim. Name, question and SQL are all required; a row missing any of them
 * is dropped by the builder rather than emitted as invalid SQL.
 */
export function VerifiedQueriesSection({
  rows, onChange,
}: {
  rows: SemVerifiedQuery[]; onChange: (rows: SemVerifiedQuery[]) => void;
}) {
  const patch = (i: number, next: Partial<SemVerifiedQuery>) =>
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...next } : r)));

  return (
    <Section
      label="AI_VERIFIED_QUERIES"
      help="Question / SQL pairs Cortex Analyst reuses verbatim."
      empty={rows.length === 0}
      addText="Add verified query"
      onAdd={() => onChange([...rows, emptyVerifiedQuery()])}
    >
      {rows.map((r, i) => (
        <RowCard key={i} onRemove={() => onChange(rows.filter((_, idx) => idx !== i))}>
          <Input
            size="small" style={{ width: 160 }} placeholder="name"
            value={r.name} onChange={(e) => patch(i, { name: e.target.value })}
          />
          <Input
            size="small" style={{ minWidth: 240, flex: 1 }} placeholder="QUESTION"
            value={r.question} onChange={(e) => patch(i, { question: e.target.value })}
          />
          <InputNumber
            size="small" style={{ width: 170 }} placeholder="VERIFIED_AT (epoch)"
            value={r.verifiedAt ? Number(r.verifiedAt) : null}
            onChange={(v) => patch(i, { verifiedAt: v == null ? "" : String(v) })}
          />
          <Input
            size="small" style={{ width: 200 }} placeholder="VERIFIED_BY, e.g. ( analyst = jane )"
            value={r.verifiedBy} onChange={(e) => patch(i, { verifiedBy: e.target.value })}
          />
          <Checkbox
            checked={r.onboardingQuestion}
            onChange={(e) => patch(i, { onboardingQuestion: e.target.checked })}
          >
            <span style={{ fontSize: 11 }}>ONBOARDING_QUESTION</span>
          </Checkbox>
          <Input.TextArea
            size="small" style={{ flexBasis: "100%" }} placeholder="SQL"
            value={r.sql} onChange={(e) => patch(i, { sql: e.target.value })}
            autoSize={{ minRows: 1, maxRows: 4 }}
          />
        </RowCard>
      ))}
    </Section>
  );
}
