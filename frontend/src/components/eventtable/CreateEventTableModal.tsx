// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import {
  Form, Input, InputNumber, Select, Checkbox, Typography, Collapse,
} from "antd";
import { AuditOutlined } from "@ant-design/icons";
import { BuildCreateEventTableSql, ExecDDL, GetCollations } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import TagInput from "../shared/TagInput";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { eventtable, snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain data shape for form state. The Wails-generated `EventTableConfig` class
// carries a `convertValues` method (it has a nested `tags` array), which a plain
// object literal can't satisfy; we cast to the generated type only at the IPC
// boundary (`cfg as any`).
type ETConfig = Omit<eventtable.EventTableConfig, "convertValues" | "tags"> & {
  tags: { name: string; value: string }[];
};

export default function CreateEventTableModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<ETConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    clusterBy: "",
    dataRetentionTimeInDays: "",
    maxDataExtensionTimeInDays: "",
    changeTracking: "",
    defaultDdlCollation: "",
    copyGrants: false,
    comment: "",
    tags: [],
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateEventTableSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  // DEFAULT_DDL_COLLATION choices come from the backend (internal/snowflake).
  const [collations, setCollations] = useState<snowflake.CollationOption[]>([]);
  useEffect(() => {
    GetCollations()
      .then((opts) => setCollations(opts ?? []))
      .catch(() => setCollations([]));
  }, []);

  const set = <K extends keyof ETConfig>(key: K, value: ETConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      // Build from the current cfg at submit time rather than reusing the
      // `preview` state, which is refreshed by an async effect and can lag a
      // keystroke behind the latest cfg.
      const sql = await BuildCreateEventTableSql(db, schema, cfg as any);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const advancedBody = (
    <>
      <Form.Item label="Cluster by" style={itemStyle} help="Comma-separated clustering expressions on the predefined columns, e.g. timestamp, resource_attributes:service.name.">
        <Input
          value={cfg.clusterBy}
          onChange={(e) => set("clusterBy", e.target.value)}
          placeholder="(optional)"
        />
      </Form.Item>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Data retention (days)" style={itemStyle} help="Time Travel retention period (DATA_RETENTION_TIME_IN_DAYS).">
          <InputNumber
            min={0}
            style={{ width: "100%" }}
            value={cfg.dataRetentionTimeInDays === "" ? undefined : Number(cfg.dataRetentionTimeInDays)}
            onChange={(v) => set("dataRetentionTimeInDays", v == null ? "" : String(v))}
            placeholder="(default)"
          />
        </Form.Item>
        <Form.Item label="Max data extension (days)" style={itemStyle} help="MAX_DATA_EXTENSION_TIME_IN_DAYS — extends retention to keep streams from going stale.">
          <InputNumber
            min={0}
            style={{ width: "100%" }}
            value={cfg.maxDataExtensionTimeInDays === "" ? undefined : Number(cfg.maxDataExtensionTimeInDays)}
            onChange={(v) => set("maxDataExtensionTimeInDays", v == null ? "" : String(v))}
            placeholder="(default)"
          />
        </Form.Item>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Change tracking" style={itemStyle} help="Enable change tracking so the event table can be a stream source.">
          <Select
            allowClear
            value={cfg.changeTracking || undefined}
            onChange={(v) => set("changeTracking", v ?? "")}
            placeholder="(default — FALSE)"
            options={[{ value: "TRUE", label: "TRUE" }, { value: "FALSE", label: "FALSE" }]}
          />
        </Form.Item>
        <Form.Item label="Default DDL collation" style={itemStyle} help="DEFAULT_DDL_COLLATION applied to string columns.">
          <Select
            showSearch
            allowClear
            value={cfg.defaultDdlCollation || undefined}
            onChange={(v) => set("defaultDdlCollation", v ?? "")}
            placeholder="(optional — no collation)"
            style={{ width: "100%" }}
            options={collations.map((c) => ({ value: c.value, label: c.label }))}
          />
        </Form.Item>
      </div>
      <Form.Item style={itemStyle}>
        {/* COPY GRANTS only has an effect when an existing object is replaced,
            so it's gated on OR REPLACE — disabled (and force-cleared) otherwise. */}
        <Checkbox
          checked={cfg.copyGrants}
          disabled={!cfg.orReplace}
          onChange={(e) => set("copyGrants", e.target.checked)}
        >
          Copy grants
        </Checkbox>
        <Text type="secondary" style={{ fontSize: 12, display: "block" }}>
          {cfg.orReplace
            ? "Retain access privileges from the object replaced via OR REPLACE."
            : "Enable OR REPLACE to retain access privileges from the replaced object."}
        </Text>
      </Form.Item>
      <TagInput
        tags={cfg.tags}
        onChange={(tags) => set("tags", tags)}
        help="Table-level tags applied at creation"
        itemStyle={itemStyle}
      />
    </>
  );

  return (
    <CreateModalShell
      icon={<AuditOutlined />}
      title="Create Event Table"
      subtitle={`${db}.${schema}`}
      width={680}
      error={error}
      errorTitle="Event table creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Event table name"
          placeholder="MY_EVENT_TABLE"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          // COPY GRANTS only applies with a replace, so clear it when OR REPLACE
          // is turned off to keep the generated SQL honest.
          onOrReplaceChange={(v) => setCfg((prev) => ({ ...prev, orReplace: v, copyGrants: v ? prev.copyGrants : false }))}
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

        <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 12 }}>
          Event tables have a fixed, predefined schema for telemetry (logs, traces,
          metrics) — no columns are defined here.
        </Text>

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[{ key: "advanced", label: "Advanced options", children: advancedBody }]}
        />

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
