// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import { Form, Input, Select, Checkbox, Space, Collapse, DatePicker } from "antd";
import { ThunderboltOutlined } from "@ant-design/icons";
import type { Dayjs } from "dayjs";
import {
  BuildCreateStreamSql, ExecDDL, ListDatabases, ListUserSchemas, ListObjects,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { stream, snowflake } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const SOURCE_TYPES = ["TABLE", "VIEW", "EXTERNAL TABLE", "STAGE", "DYNAMIC TABLE"];

const esc = (s: string) => s.replace(/"/g, '""');
const qualify = (d: string, s: string, n: string) => `"${esc(d)}"."${esc(s)}"."${esc(n)}"`;

// Plain data shape for form state. The Wails-generated `StreamConfig` class
// carries a `convertValues` method that a plain object literal can't satisfy; we
// cast to the generated type only at the IPC boundary (`cfg as any`).
type StrConfig = Omit<stream.StreamConfig, "convertValues">;

export default function CreateStreamModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<StrConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    copyGrants: false,
    sourceType: "TABLE",
    source: "",
    timeTravelMode: "",
    timeTravelKind: "OFFSET",
    timeTravelValue: "",
    appendOnly: false,
    showInitialRows: false,
    insertOnly: false,
    comment: "",
  });

  // Source-object picker state: a database → schema → object cascade seeded with
  // the stream's own db/schema (the source may live in a different schema). The
  // picked object's fully-qualified, quoted name becomes cfg.source.
  const [srcDb, setSrcDb] = useState(db);
  const [srcSchema, setSrcSchema] = useState(schema);
  const [srcObject, setSrcObject] = useState("");
  const [dbOptions, setDbOptions] = useState<string[]>([]);
  const [schemaOptions, setSchemaOptions] = useState<string[]>([]);
  const [objects, setObjects] = useState<snowflake.SnowflakeObject[]>([]);
  const [loadingSchemas, setLoadingSchemas] = useState(false);
  const [loadingObjects, setLoadingObjects] = useState(false);

  // Existing streams in the new stream's own schema, for the Time Travel
  // `STREAM => '<name>'` picker (create the stream at the same offset as another).
  const [streamOptions, setStreamOptions] = useState<string[]>([]);
  const [loadingStreams, setLoadingStreams] = useState(false);

  // The TIMESTAMP Time Travel picker holds a Dayjs for display; cfg.timeTravelValue
  // carries the derived SQL literal the builder emits verbatim.
  const [timeTravelTs, setTimeTravelTs] = useState<Dayjs | null>(null);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateStreamSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof StrConfig>(key: K, value: StrConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Load databases once; schemas/objects react to the current picker selection.
  useEffect(() => {
    ListDatabases().then((d) => setDbOptions(d ?? [])).catch(() => {});
  }, []);

  useEffect(() => {
    if (!srcDb) { setSchemaOptions([]); return; }
    setLoadingSchemas(true);
    ListUserSchemas(srcDb)
      .then((s) => setSchemaOptions(s ?? []))
      .catch(() => setSchemaOptions([]))
      .finally(() => setLoadingSchemas(false));
  }, [srcDb]);

  useEffect(() => {
    if (!srcDb || !srcSchema) { setObjects([]); return; }
    setLoadingObjects(true);
    ListObjects(srcDb, srcSchema)
      .then((objs) => setObjects(objs ?? []))
      .catch(() => setObjects([]))
      .finally(() => setLoadingObjects(false));
  }, [srcDb, srcSchema]);

  // The new stream's own db/schema are fixed props, so load its sibling streams
  // once for the Time Travel STREAM picker.
  useEffect(() => {
    if (!db || !schema) { setStreamOptions([]); return; }
    setLoadingStreams(true);
    ListObjects(db, schema)
      .then((objs) => setStreamOptions((objs ?? []).filter((o) => o.kind === "STREAM").map((o) => o.name)))
      .catch(() => setStreamOptions([]))
      .finally(() => setLoadingStreams(false));
  }, [db, schema]);

  // Objects of the currently-selected source type, in the selected db/schema.
  const objectOptions = objects
    .filter((o) => o.kind === cfg.sourceType)
    .map((o) => o.name);

  const pickObject = (name: string) => {
    setSrcObject(name);
    set("source", name ? qualify(srcDb, srcSchema, name) : "");
  };
  const pickSourceType = (t: string) => {
    // The object list is type-specific, so any prior pick is no longer valid.
    // The optional clauses are also source-type-specific, so reset them —
    // otherwise a clause valid for the previous type (e.g. APPEND_ONLY or Time
    // Travel for a table) could leak into a CREATE STREAM for a type that rejects
    // it (e.g. a stage or dynamic table).
    setSrcObject("");
    setTimeTravelTs(null);
    setCfg((prev) => ({
      ...prev,
      sourceType: t,
      source: "",
      timeTravelMode: "",
      timeTravelValue: "",
      appendOnly: false,
      showInitialRows: false,
      insertOnly: false,
    }));
  };
  // Each Time Travel kind uses a different value widget/shape, so switching kind
  // invalidates any value entered for the previous one — clear it (and the
  // TIMESTAMP picker's Dayjs).
  const pickTimeTravelKind = (v: string) => {
    setTimeTravelTs(null);
    setCfg((prev) => ({ ...prev, timeTravelKind: v, timeTravelValue: "" }));
  };
  const pickSrcDb = (v?: string) => {
    setSrcDb(v ?? "");
    setSrcSchema("");
    pickObject("");
  };
  const pickSrcSchema = (v?: string) => {
    setSrcSchema(v ?? "");
    pickObject("");
  };

  // A Time Travel mode without a value would be silently dropped by the builder
  // (timeTravelClause returns "" when the value is empty). Surface that as an
  // explicit incomplete state rather than letting the clause vanish unnoticed.
  const timeTravelIncomplete =
    ["TABLE", "EXTERNAL TABLE", "VIEW"].includes(cfg.sourceType) &&
    !!cfg.timeTravelMode &&
    cfg.timeTravelValue.trim().length === 0;

  const canSubmit =
    cfg.name.trim().length > 0 && cfg.source.trim().length > 0 && !timeTravelIncomplete;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  // The optional clauses are source-type-specific (per Snowflake's CREATE STREAM
  // reference, which documents a distinct grammar per source type):
  //   - AT | BEFORE Time Travel:         TABLE, EXTERNAL TABLE, VIEW
  //   - APPEND_ONLY / SHOW_INITIAL_ROWS: TABLE, VIEW
  //   - INSERT_ONLY:                     EXTERNAL TABLE
  //   - STAGE / DYNAMIC TABLE:           none of the above
  // COPY GRANTS is valid for any source.
  const timeTravelSource = ["TABLE", "EXTERNAL TABLE", "VIEW"].includes(cfg.sourceType);
  const rowChangeSource = ["TABLE", "VIEW"].includes(cfg.sourceType);
  const externalSource = cfg.sourceType === "EXTERNAL TABLE";

  // The Time Travel value widget is kind-specific: TIMESTAMP gets a date-time
  // picker (its Dayjs is formatted into a `'…'::timestamp` literal the builder
  // emits verbatim), STREAM a dropdown of sibling streams (quoted as a string
  // literal by the builder), and OFFSET / STATEMENT a free-text field.
  const offsetStatementPlaceholder: Record<string, string> = {
    OFFSET: "-60 (seconds; negative = past)",
    STATEMENT: "query id",
  };
  const timeTravelValueWidget = () => {
    if (!cfg.timeTravelMode) {
      return <Input style={{ width: 260 }} disabled placeholder="value" />;
    }
    if (cfg.timeTravelKind === "TIMESTAMP") {
      return (
        <DatePicker
          showTime
          style={{ width: 260 }}
          value={timeTravelTs}
          onChange={(d) => {
            setTimeTravelTs(d);
            set("timeTravelValue", d ? `'${d.format("YYYY-MM-DD HH:mm:ss")}'::timestamp` : "");
          }}
          placeholder="Select date & time"
        />
      );
    }
    if (cfg.timeTravelKind === "STREAM") {
      return (
        <Select
          showSearch
          style={{ width: 260 }}
          value={cfg.timeTravelValue || undefined}
          onChange={(v) => set("timeTravelValue", v ?? "")}
          loading={loadingStreams}
          options={streamOptions.map((n) => ({ value: n, label: n }))}
          placeholder="Select a stream"
          notFoundContent={loadingStreams ? "Loading…" : "No streams in this schema"}
        />
      );
    }
    return (
      <Input
        style={{ width: 260 }}
        value={cfg.timeTravelValue}
        onChange={(e) => set("timeTravelValue", e.target.value)}
        placeholder={offsetStatementPlaceholder[cfg.timeTravelKind] ?? ""}
      />
    );
  };

  const advancedBody = (
    <Space direction="vertical" size={12} style={{ display: "flex" }}>
      {timeTravelSource && (
        <div>
          <div style={{ marginBottom: 4, fontSize: 12, opacity: 0.65 }}>
            Time Travel (AT / BEFORE)
          </div>
          <Space size={8} wrap>
            <Select
              style={{ width: 120 }}
              value={cfg.timeTravelMode || ""}
              onChange={(v) => set("timeTravelMode", v)}
              options={[
                { value: "", label: "None" },
                { value: "AT", label: "AT" },
                { value: "BEFORE", label: "BEFORE" },
              ]}
            />
            <Select
              style={{ width: 150 }}
              disabled={!cfg.timeTravelMode}
              value={cfg.timeTravelKind}
              onChange={pickTimeTravelKind}
              options={["TIMESTAMP", "OFFSET", "STATEMENT", "STREAM"].map((k) => ({ value: k, label: k }))}
            />
            {timeTravelValueWidget()}
          </Space>
          {timeTravelIncomplete && (
            <div style={{ marginTop: 4, fontSize: 12, color: "var(--ant-color-error, #ff4d4f)" }}>
              Enter a {cfg.timeTravelKind.toLowerCase()} value or set the mode back to None.
            </div>
          )}
        </div>
      )}
      <Form.Item style={{ marginBottom: 0 }}>
        <Space size={16} wrap>
          {rowChangeSource && (
            <Checkbox checked={cfg.appendOnly} onChange={(e) => set("appendOnly", e.target.checked)}>
              APPEND_ONLY
            </Checkbox>
          )}
          {rowChangeSource && (
            <Checkbox checked={cfg.showInitialRows} onChange={(e) => set("showInitialRows", e.target.checked)}>
              SHOW_INITIAL_ROWS
            </Checkbox>
          )}
          {externalSource && (
            <Checkbox checked={cfg.insertOnly} onChange={(e) => set("insertOnly", e.target.checked)}>
              INSERT_ONLY
            </Checkbox>
          )}
          <Checkbox checked={cfg.copyGrants} onChange={(e) => set("copyGrants", e.target.checked)}>
            COPY GRANTS
          </Checkbox>
        </Space>
      </Form.Item>
    </Space>
  );

  return (
    <CreateModalShell
      icon={<ThunderboltOutlined />}
      title="Create Stream"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Stream creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Stream name"
          placeholder="MY_STREAM"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
        />

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Source type" style={itemStyle}>
          <Select
            value={cfg.sourceType}
            onChange={pickSourceType}
            options={SOURCE_TYPES.map((t) => ({ value: t, label: t }))}
          />
        </Form.Item>

        <Form.Item
          label="Source object"
          required
          style={itemStyle}
          help={cfg.source ? `Tracks ${cfg.source}` : "Pick the object the stream tracks"}
        >
          <Space size={8} wrap>
            <Select
              showSearch
              placeholder="Database"
              style={{ width: 170 }}
              value={srcDb || undefined}
              onChange={pickSrcDb}
              options={dbOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              showSearch
              placeholder="Schema"
              style={{ width: 170 }}
              value={srcSchema || undefined}
              onChange={pickSrcSchema}
              disabled={!srcDb}
              loading={loadingSchemas}
              options={schemaOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              showSearch
              placeholder={cfg.sourceType.toLowerCase()}
              style={{ width: 200 }}
              value={srcObject || undefined}
              onChange={(v) => pickObject(v ?? "")}
              disabled={!srcSchema}
              loading={loadingObjects}
              options={objectOptions.map((n) => ({ value: n, label: n }))}
              notFoundContent={loadingObjects ? "Loading…" : `No ${cfg.sourceType.toLowerCase()} objects`}
            />
          </Space>
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[{ key: "advanced", label: "Advanced options", children: advancedBody }]}
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
