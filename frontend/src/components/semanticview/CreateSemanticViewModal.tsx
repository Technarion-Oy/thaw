// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useMemo } from "react";
import { Form, Input, InputNumber, Checkbox, Alert, Collapse, Typography } from "antd";
import { ApartmentOutlined } from "@ant-design/icons";
import {
  BuildCreateSemanticViewSql, ExecDDL, ListDatabases,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import TagInput from "../shared/TagInput";
import type { TagItem } from "../shared/TagInput";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import {
  TablesSection, RelationshipsSection, ExpressionsSection, VerifiedQueriesSection,
  useTableColumns, useObjectCache, toLogicalTable, toExpression, aliasOf,
} from "./semanticViewForm";
import type {
  TableRow, SemRelationship, ExpressionRow, SemVerifiedQuery,
} from "./semanticViewForm";
import {
  diffAliases, hasAliasChange, remapAlias, remapQualified,
} from "./semanticViewAliases";
import Editor from "@monaco-editor/react";
import { setActiveSnippetEditor } from "../editor/SqlEditor";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

// Snowflake's documented floor for MAX_STALENESS (seconds).
const MIN_MAX_STALENESS = 120;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

/**
 * Form-driven CREATE SEMANTIC VIEW dialog. Each clause of the order-sensitive
 * definition (TABLES → RELATIONSHIPS → FACTS → DIMENSIONS → METRICS) is its own
 * section of pickers, and the builder — not the user — emits them in the order
 * Snowflake requires. The raw-SQL editor under "Advanced" remains as an escape
 * hatch: anything typed there replaces the whole structured definition.
 */
export default function CreateSemanticViewModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);

  const [tables, setTables] = useState<TableRow[]>([]);
  const [relationships, setRelationships] = useState<SemRelationship[]>([]);
  const [facts, setFacts] = useState<ExpressionRow[]>([]);
  const [dimensions, setDimensions] = useState<ExpressionRow[]>([]);
  const [metrics, setMetrics] = useState<ExpressionRow[]>([]);

  const [comment, setComment] = useState("");
  const [maxStaleness, setMaxStaleness] = useState<number | null>(null);
  const [aiSqlGeneration, setAiSqlGeneration] = useState("");
  const [aiQuestionCategorization, setAiQuestionCategorization] = useState("");
  const [verifiedQueries, setVerifiedQueries] = useState<SemVerifiedQuery[]>([]);
  const [tags, setTags] = useState<TagItem[]>([]);
  const [copyGrants, setCopyGrants] = useState(false);
  const [body, setBody] = useState("");

  // A semantic view can reference tables and Cortex search services in any
  // database, so the pickers get the full database list; the view's own db/schema
  // only seed a new row.
  const [dbOptions, setDbOptions] = useState<string[]>([db]);
  useEffect(() => {
    ListDatabases()
      // The view's own database seeds every new table row, so it must be an
      // option even if ListDatabases filters differently from whatever listed it.
      .then((d) => setDbOptions(d?.includes(db) ? d : [db, ...(d ?? [])]))
      .catch(() => {});
  }, [db]);

  const columnsFor = useTableColumns(tables);
  const objectCache = useObjectCache();

  // The other sections reference logical tables by alias — a copied string, not
  // a live reference — so every edit to the table list is diffed and the
  // dependent rows are rewritten: a renamed alias is followed, a removed one
  // cleared. Without this a rename or delete silently leaves rows pointing at an
  // alias that no longer exists in TABLES ( … ), which only fails once Snowflake
  // runs the statement.
  const updateTables = (next: TableRow[]) => {
    const diff = diffAliases(tables.map(aliasOf), next.map(aliasOf));
    setTables(next);
    if (!hasAliasChange(diff)) return;
    setRelationships((rs) => rs.map((r) => ({
      ...r,
      table: remapAlias(r.table, diff),
      refTable: remapAlias(r.refTable, diff),
    })));
    const remapRows = (rows: ExpressionRow[]) => rows.map((e) => ({
      ...e,
      tableAlias: remapAlias(e.tableAlias, diff),
      nonAdditiveBy: e.nonAdditiveBy
        .map((d) => ({ ...d, dimension: remapQualified(d.dimension, diff) }))
        .filter((d) => d.dimension),
    }));
    setFacts(remapRows);
    setDimensions(remapRows);
    setMetrics(remapRows);
  };

  const cfg = useMemo(() => ({
    name,
    caseSensitive,
    orReplace,
    ifNotExists,
    body,
    tables: tables.map(toLogicalTable),
    relationships,
    facts: facts.map(toExpression),
    dimensions: dimensions.map(toExpression),
    metrics: metrics.map(toExpression),
    comment,
    maxStaleness: maxStaleness ?? 0,
    aiSqlGeneration,
    aiQuestionCategorization,
    verifiedQueries,
    tags,
    copyGrants,
  }), [
    name, caseSensitive, orReplace, ifNotExists, body,
    tables, relationships, facts, dimensions, metrics,
    comment, maxStaleness, aiSqlGeneration, aiQuestionCategorization,
    verifiedQueries, tags, copyGrants,
  ]);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateSemanticViewSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  // Dimension references for a metric's NON ADDITIVE BY picker. Only rows the
  // builder will actually emit are offered — picking a half-finished dimension
  // would leave the metric referencing one that never appears in DIMENSIONS.
  const dimensionNames = dimensions
    .filter((d) => d.tableAlias.trim() && d.name.trim())
    .map((d) => `${d.tableAlias.trim()}.${d.name.trim()}`);

  // A complete expression needs all three of `<table_alias>.<name> AS <sql_expr>`
  // — the grammar has no alias-less form — matching what the builder accepts.
  const complete = (e: ExpressionRow) =>
    e.tableAlias.trim().length > 0 && e.name.trim().length > 0 && e.expr.trim().length > 0;
  // Snowflake requires at least one dimension or metric, and the definition needs
  // at least one logical table. The raw-SQL escape hatch bypasses both — its
  // content is the definition, and the builder can't validate it.
  const structuredValid =
    tables.some((t) => t.table.trim().length > 0) &&
    (dimensions.some(complete) || metrics.some(complete));
  // A pasted snippet may still carry <database>.<schema> style placeholders; they
  // would only fail server-side, so block them here as the old default-body guard
  // did.
  // Three letters minimum so a spaceless comparison (`a<b>c`) isn't mistaken
  // for a placeholder.
  const bodyPlaceholders = /<[a-z_]{3,}>/i.test(body);
  const canSubmit =
    name.trim().length > 0 &&
    (body.trim().length > 0 ? !bodyPlaceholders : structuredValid);

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const rawSqlBody = (
    <>
      <Alert
        type={body.trim() ? (bodyPlaceholders ? "error" : "warning") : "info"}
        showIcon
        style={{ marginBottom: 8 }}
        message={
          bodyPlaceholders
            ? "The raw definition still contains <placeholder> tokens — replace them with real names before creating."
            : body.trim()
            ? "The raw definition below replaces the TABLES / RELATIONSHIPS / FACTS / DIMENSIONS / METRICS sections above. Clear it to go back to the form."
            : "Escape hatch for anything the form doesn't cover. Anything typed here replaces the structured definition above; the clause order is then yours to get right."
        }
      />
      <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
        <Editor
          height={220}
          language="sql"
          theme={editorTheme}
          value={body}
          onChange={(v) => setBody(v ?? "")}
          onMount={(editor) => {
            patchMonacoClipboard(editor);
            editor.onContextMenu(() => setActiveSnippetEditor(editor));
            editor.onDidDispose(() => setActiveSnippetEditor(null));
          }}
          options={{
            minimap: { enabled: false },
            scrollBeyondLastLine: false,
            fontSize: 12,
            wordWrap: "on",
            automaticLayout: true,
          }}
        />
      </div>
    </>
  );

  const viewOptionsBody = (
    <>
      <Form.Item
        label="MAX_STALENESS"
        style={itemStyle}
        help={`Seconds a cached result may lag behind the source. Minimum ${MIN_MAX_STALENESS}.`}
      >
        <InputNumber
          min={MIN_MAX_STALENESS}
          style={{ width: 200 }}
          value={maxStaleness}
          onChange={setMaxStaleness}
          placeholder="not set"
        />
      </Form.Item>

      <Form.Item label="AI_SQL_GENERATION" style={itemStyle}>
        <Input.TextArea
          value={aiSqlGeneration}
          onChange={(e) => setAiSqlGeneration(e.target.value)}
          placeholder="Instructions steering how Cortex Analyst writes SQL against this view"
          autoSize={{ minRows: 1, maxRows: 3 }}
        />
      </Form.Item>

      <Form.Item label="AI_QUESTION_CATEGORIZATION" style={itemStyle}>
        <Input.TextArea
          value={aiQuestionCategorization}
          onChange={(e) => setAiQuestionCategorization(e.target.value)}
          placeholder="Instructions for categorizing incoming questions"
          autoSize={{ minRows: 1, maxRows: 3 }}
        />
      </Form.Item>

      <VerifiedQueriesSection rows={verifiedQueries} onChange={setVerifiedQueries} />

      <TagInput tags={tags} onChange={setTags} itemStyle={itemStyle} />

      <Form.Item style={itemStyle}>
        <Checkbox checked={copyGrants} onChange={(e) => setCopyGrants(e.target.checked)}>
          COPY GRANTS
        </Checkbox>
        <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
          Retain access grants from a replaced semantic view of the same name.
        </Text>
      </Form.Item>
    </>
  );

  return (
    <CreateModalShell
      icon={<ApartmentOutlined />}
      title="Create Semantic View"
      subtitle={`${db}.${schema}`}
      width={980}
      error={error}
      errorTitle="Semantic view creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="A semantic view defines a semantic layer over physical tables for natural-language querying with Cortex Analyst. Add the logical tables, the relationships between them, and the facts / dimensions / metrics that describe the data — the clauses are emitted in the order Snowflake requires. At least one dimension or metric is required."
        />

        <NameWithReplaceOptions
          label="Semantic view name"
          placeholder="MY_SEMANTIC_VIEW"
          name={name}
          onNameChange={setName}
          orReplace={orReplace}
          ifNotExists={ifNotExists}
          onOrReplaceChange={setOrReplace}
          onIfNotExistsChange={setIfNotExists}
        />

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={name}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input.TextArea
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder="Optional description of this semantic view"
            autoSize={{ minRows: 1, maxRows: 3 }}
          />
        </Form.Item>

        <TablesSection
          cache={objectCache}
          dbOptions={dbOptions}
          defaultDb={db}
          defaultSchema={schema}
          rows={tables}
          onChange={updateTables}
          columnsFor={columnsFor}
        />

        <RelationshipsSection
          rows={relationships}
          tables={tables}
          onChange={setRelationships}
          columnsFor={columnsFor}
        />

        {([
          ["FACTS", facts, setFacts],
          ["DIMENSIONS", dimensions, setDimensions],
          ["METRICS", metrics, setMetrics],
        ] as const).map(([kind, rows, setRows]) => (
          <ExpressionsSection
            key={kind}
            kind={kind}
            rows={rows}
            tables={tables}
            relationships={relationships}
            dimensionNames={dimensionNames}
            cache={objectCache}
            dbOptions={dbOptions}
            onChange={setRows}
          />
        ))}

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[
            { key: "options", label: "View options", children: viewOptionsBody },
            { key: "raw", label: "Advanced — raw SQL definition", children: rawSqlBody },
          ]}
        />

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          ALTER only changes the comment, tags, or name — change the definition with “OR REPLACE”. Semantic views require Cortex AI to be enabled in your account.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
