// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import { Form, Input, Select, Checkbox, Space, Collapse } from "antd";
import { ThunderboltOutlined } from "@ant-design/icons";
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
    // The CDC flags are also source-type-specific, so reset them — otherwise a
    // flag valid for the previous type (e.g. APPEND_ONLY for a table) could leak
    // into a CREATE STREAM for a type that rejects it (e.g. an external table).
    setSrcObject("");
    setCfg((prev) => ({
      ...prev,
      sourceType: t,
      source: "",
      appendOnly: false,
      showInitialRows: false,
      insertOnly: false,
    }));
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

  const canSubmit = cfg.name.trim().length > 0 && cfg.source.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  // CDC flags are source-type-specific: APPEND_ONLY / SHOW_INITIAL_ROWS apply to
  // streams on tables, views and dynamic tables; INSERT_ONLY applies only to
  // streams on external tables; stage (directory-table) streams take none of
  // them. COPY GRANTS is valid for any source.
  const rowChangeSource = ["TABLE", "VIEW", "DYNAMIC TABLE"].includes(cfg.sourceType);
  const externalSource = cfg.sourceType === "EXTERNAL TABLE";

  const advancedBody = (
    <Form.Item style={{ marginBottom: 8 }}>
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
