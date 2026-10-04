// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import { Form, Input, Checkbox, Select } from "antd";
import { PartitionOutlined } from "@ant-design/icons";
import { ExecDDL, ListUserSchemas } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl, { identToken, needsQuoting, quoteIdent, quoteQualifiedIdent } from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import TagInput, { type TagItem } from "../shared/TagInput";
import { quoteTextLit } from "../shared/sqlEscape";
import { useQuotedIdentifiers, useCreateSubmit } from "../shared/createModalHooks";

export interface SchemaConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  transient: boolean;
  cloneSource: string;
  managedAccess: boolean;
  dataRetention: string;
  comment: string;
  tags: TagItem[];
}

// Tag names are usually qualified (db.schema.tag); quote a part only when it
// can't stand as a bare identifier so an unquoted name keeps resolving
// case-insensitively.
const tagNameToken = (name: string) =>
  name.split(".").map((p) => identToken(p, false)).join(".");

/** Builds the CREATE SCHEMA statement; returns "" until a name is entered. */
export function buildCreateSchemaSql(db: string, cfg: SchemaConfig): string {
  const name = cfg.name.trim();
  if (!name) return "";

  const head = ["CREATE"];
  if (cfg.orReplace) head.push("OR REPLACE");
  if (cfg.transient) head.push("TRANSIENT");
  head.push("SCHEMA");
  if (cfg.ifNotExists && !cfg.orReplace) head.push("IF NOT EXISTS");
  head.push(`${quoteIdent(db)}.${identToken(name, cfg.caseSensitive)}`);

  const lines = [head.join(" ")];
  if (cfg.cloneSource) lines.push(`  CLONE ${quoteQualifiedIdent(db, cfg.cloneSource)}`);
  if (cfg.managedAccess) lines.push("  WITH MANAGED ACCESS");
  // Spliced unquoted, so only a plain non-negative integer is let through.
  if (/^\d+$/.test(cfg.dataRetention)) lines.push(`  DATA_RETENTION_TIME_IN_DAYS = ${cfg.dataRetention}`);
  const tags = cfg.tags.filter((t) => t.name);
  if (tags.length > 0) {
    lines.push(`  WITH TAG (${tags.map((t) => `${tagNameToken(t.name)} = ${quoteTextLit(t.value)}`).join(", ")})`);
  }
  if (cfg.comment) lines.push(`  COMMENT = ${quoteTextLit(cfg.comment)}`);
  return lines.join("\n") + ";";
}

interface Props {
  db: string;
  onClose: () => void;
  /** Called with the schema name as Snowflake stores it. */
  onSuccess?: (schema: string) => void;
}

export default function CreateSchemaModal({ db, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<SchemaConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    transient: false,
    cloneSource: "",
    managedAccess: false,
    dataRetention: "",
    comment: "",
    tags: [],
  });
  const [schemas, setSchemas] = useState<string[]>([]);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const { creating, error, setError, submit } = useCreateSubmit();

  useEffect(() => {
    // ?? [] because a Go nil slice serializes to JSON null.
    ListUserSchemas(db).then((s) => setSchemas(s ?? [])).catch(() => {});
  }, [db]);

  const set = <K extends keyof SchemaConfig>(key: K, value: SchemaConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const sql = buildCreateSchemaSql(db, cfg);
  const canSubmit = sql !== "";

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(sql);
      const name = cfg.name.trim();
      const keepsCase = (cfg.caseSensitive || needsQuoting(name)) && !quotedIdentifiersIgnoreCase;
      onSuccess?.(keepsCase ? name : name.toUpperCase());
      onClose();
    });
  };

  return (
    <CreateModalShell
      icon={<PartitionOutlined />}
      title="Create Schema"
      subtitle={db}
      width={640}
      error={error}
      errorTitle="Schema creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Schema name"
          placeholder="MY_SCHEMA"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
          extra={
            <Checkbox checked={cfg.transient} onChange={(e) => set("transient", e.target.checked)}>
              TRANSIENT
            </Checkbox>
          }
        />
        <Form.Item style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        {/* ponytail: clone source is limited to schemas of this database, at the
            current point in time. Add a database picker / AT|BEFORE when someone
            needs a cross-database or Time Travel clone from the dialog. */}
        <Form.Item label="Clone from schema" style={{ marginBottom: 8 }}>
          <Select
            allowClear
            showSearch
            value={cfg.cloneSource || undefined}
            onChange={(v) => set("cloneSource", v ?? "")}
            placeholder="(none)"
            options={schemas.map((s) => ({ value: s, label: s }))}
          />
        </Form.Item>

        <Form.Item style={{ marginBottom: 8 }}>
          <Checkbox checked={cfg.managedAccess} onChange={(e) => set("managedAccess", e.target.checked)}>
            WITH MANAGED ACCESS
          </Checkbox>
        </Form.Item>

        <Form.Item
          label="DATA_RETENTION_TIME_IN_DAYS"
          tooltip="Standard Edition: 0 or 1. Enterprise Edition: 0–90 (permanent), 0 or 1 (transient)."
          style={{ marginBottom: 8 }}
        >
          <Input
            type="number"
            min={0}
            value={cfg.dataRetention}
            onChange={(e) => set("dataRetention", e.target.value)}
            placeholder="(default)"
            style={{ width: 120 }}
          />
        </Form.Item>

        <Form.Item label="Comment" style={{ marginBottom: 8 }}>
          <Input value={cfg.comment} onChange={(e) => set("comment", e.target.value)} placeholder="(none)" />
        </Form.Item>

        <TagInput tags={cfg.tags} onChange={(tags) => set("tags", tags)} itemStyle={{ marginBottom: 12 }} />

        <SqlPreview sql={sql} placeholder="-- Enter a schema name" />
      </Form>
    </CreateModalShell>
  );
}
