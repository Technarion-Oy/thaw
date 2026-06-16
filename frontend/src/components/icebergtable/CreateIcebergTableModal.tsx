// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import {
  Form, Input, Select, Space, Typography, Button, Collapse,
} from "antd";
import { GoldOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateIcebergTableSql, ExecDDL, ListExternalVolumes, ListIntegrations } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import TagInput from "../shared/TagInput";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { icebergtable } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain data shape for form state. The Wails-generated `IcebergTableConfig` class
// carries a `convertValues` method (it has nested `columns`/`tags` arrays), which
// a plain object literal can't satisfy; we cast to the generated type only at the
// IPC boundary (`cfg as any`).
type ITColumn = { name: string; type: string };
type ITConfig = Omit<icebergtable.IcebergTableConfig, "convertValues" | "columns" | "tags"> & {
  columns: ITColumn[];
  tags: { name: string; value: string }[];
};

export default function CreateIcebergTableModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<ITConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    tableType: "snowflake",
    columns: [{ name: "", type: "VARCHAR" }],
    externalVolume: "",
    catalog: "",
    baseLocation: "",
    catalogTableName: "",
    catalogNamespace: "",
    metadataFilePath: "",
    replaceInvalidCharacters: "",
    autoRefresh: "",
    clusterBy: "",
    comment: "",
    tags: [],
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateIcebergTableSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const [volumes, setVolumes] = useState<string[]>([]);
  const [loadingVolumes, setLoadingVolumes] = useState(false);
  // Catalog integrations available in the account ({ name, type }), for the
  // external-variant catalog picker.
  const [catalogs, setCatalogs] = useState<{ name: string; type: string }[]>([]);
  const [loadingCatalogs, setLoadingCatalogs] = useState(false);

  useEffect(() => {
    setLoadingVolumes(true);
    ListExternalVolumes()
      .then((names) => setVolumes(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingVolumes(false));

    setLoadingCatalogs(true);
    ListIntegrations("CATALOG")
      .then((rows) => setCatalogs((rows ?? []).map((r) => ({ name: r.name, type: r.type }))))
      .catch(() => {})
      .finally(() => setLoadingCatalogs(false));
  }, []);

  const set = <K extends keyof ITConfig>(key: K, value: ITConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Table-type drives which fields are shown and which is mandatory; the four
  // variants mirror Snowflake's CREATE ICEBERG TABLE docs.
  const tableType = cfg.tableType;
  const isSnowflake = tableType === "snowflake";
  const isExternalCatalog = tableType === "external_catalog";
  const isDelta = tableType === "delta";
  const isIcebergFiles = tableType === "iceberg_files";
  // Whether this variant uses a catalog integration (external variants do).
  const usesCatalog = !isSnowflake;
  const usesAutoRefresh = isExternalCatalog || isDelta;

  // ── Column editing (Snowflake-managed only) ───────────────────────────────
  const addColumn = () => set("columns", [...cfg.columns, { name: "", type: "VARCHAR" }]);
  const updateColumn = (i: number, patch: Partial<ITColumn>) =>
    set("columns", cfg.columns.map((c, idx) => (idx === i ? { ...c, ...patch } : c)));
  const removeColumn = (i: number) =>
    set("columns", cfg.columns.filter((_, idx) => idx !== i));

  const namedColumns = cfg.columns.filter((c) => c.name.trim() !== "");

  // CATALOG and EXTERNAL_VOLUME are optional for every variant (a default may be
  // set on the schema/database/account), so only the variant's locator field is
  // required. Snowflake-managed additionally requires at least one column.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    (isSnowflake
      ? namedColumns.length > 0 && cfg.baseLocation.trim().length > 0
      : isExternalCatalog
        ? cfg.catalogTableName.trim().length > 0
        : isDelta
          ? cfg.baseLocation.trim().length > 0
          : /* iceberg_files */ cfg.metadataFilePath.trim().length > 0);

  const TABLE_TYPE_OPTIONS = [
    { value: "snowflake", label: "Snowflake-managed" },
    { value: "external_catalog", label: "External Iceberg catalog (REST / AWS Glue)" },
    { value: "delta", label: "Delta Lake files" },
    { value: "iceberg_files", label: "Iceberg files in object storage" },
  ];

  const TABLE_TYPE_HELP: Record<string, string> = {
    snowflake: "Snowflake is the Iceberg catalog and manages the table. Define columns and a base location.",
    external_catalog: "An external Iceberg REST API or AWS Glue catalog owns the metadata. Columns are inferred from the catalog.",
    delta: "Read Delta Lake files in object storage. The catalog integration must use CATALOG_SOURCE = OBJECT_STORE and TABLE_FORMAT = DELTA.",
    iceberg_files: "Read existing Iceberg metadata/data files in object storage (unmanaged). Point at the metadata JSON file.",
  };

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      // Build from the current cfg at submit time rather than reusing the
      // `preview` state, which is refreshed by an async effect and can lag a
      // keystroke behind the latest cfg.
      const sql = await BuildCreateIcebergTableSql(db, schema, cfg as any);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const columnsBody = (
    <Space direction="vertical" size={6} style={{ width: "100%" }}>
      {cfg.columns.length === 0 && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          Add at least one column — a Snowflake-managed Iceberg table defines its own schema.
        </Text>
      )}
      {cfg.columns.map((c, i) => (
        <Space key={i} align="start" style={{ width: "100%" }} wrap={false}>
          <Input
            size="small"
            placeholder="Column name"
            value={c.name}
            onChange={(e) => updateColumn(i, { name: e.target.value })}
            style={{ width: 200 }}
          />
          <Input
            size="small"
            placeholder="Type"
            value={c.type}
            onChange={(e) => updateColumn(i, { type: e.target.value })}
            style={{ width: 160 }}
          />
          <Button size="small" type="text" icon={<DeleteOutlined />} onClick={() => removeColumn(i)} />
        </Space>
      ))}
      <Button size="small" icon={<PlusOutlined />} onClick={addColumn}>Add column</Button>
    </Space>
  );

  const advancedBody = (
    <>
      {/* CLUSTER BY applies only to Snowflake-managed tables. */}
      {isSnowflake && (
        <Form.Item label="Cluster by" style={itemStyle} help="Comma-separated clustering expressions, e.g. event_date, region.">
          <Input
            value={cfg.clusterBy}
            onChange={(e) => set("clusterBy", e.target.value)}
            placeholder="(optional)"
          />
        </Form.Item>
      )}
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
      icon={<GoldOutlined />}
      title="Create Iceberg Table"
      subtitle={`${db}.${schema}`}
      width={760}
      error={error}
      errorTitle="Iceberg table creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Iceberg table name"
          placeholder="MY_ICEBERG_TABLE"
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

        {/* Table type selects the CREATE ICEBERG TABLE variant; the required
            fields differ per variant (see Snowflake's per-variant docs). */}
        <Form.Item label="Table type" style={itemStyle} help={TABLE_TYPE_HELP[tableType]}>
          <Select
            value={tableType}
            onChange={(v) => set("tableType", v)}
            options={TABLE_TYPE_OPTIONS}
          />
        </Form.Item>

        <Form.Item label="External volume" style={itemStyle} help="Cloud-storage volume holding the Iceberg data. Optional when a default external volume is set on the schema, database, or account.">
          <Select
            showSearch
            allowClear
            value={cfg.externalVolume || undefined}
            onChange={(v) => set("externalVolume", v ?? "")}
            options={volumes.map((n) => ({ value: n, label: n }))}
            placeholder="(optional — uses the default external volume)"
            loading={loadingVolumes}
            notFoundContent={loadingVolumes ? "Loading…" : "No external volumes found"}
          />
        </Form.Item>

        {/* Catalog integration — optional for every external variant (a default
            may be set on the schema/database/account). Snowflake-managed always
            uses CATALOG = 'SNOWFLAKE', so the field is hidden there. */}
        {usesCatalog && (
          <Form.Item label="Catalog integration" style={itemStyle} help={
            isDelta
              ? "Catalog integration configured with CATALOG_SOURCE = OBJECT_STORE and TABLE_FORMAT = DELTA. Optional when a default is set."
              : "Catalog integration for this table. Optional when a default is set on the schema, database, or account."
          }>
            <Select
              showSearch
              allowClear
              value={cfg.catalog || undefined}
              onChange={(v) => set("catalog", v ?? "")}
              options={catalogs.map((c) => ({ value: c.name, label: c.name, type: c.type }))}
              // Surface each integration's type in the dropdown so the user can
              // pick the right one (e.g. an OBJECT_STORE / Delta integration);
              // the selected control still shows just the name.
              optionRender={(opt) => (
                <Space direction="vertical" size={0}>
                  <span>{opt.data.value}</span>
                  {opt.data.type && <Text type="secondary" style={{ fontSize: 11 }}>{opt.data.type}</Text>}
                </Space>
              )}
              placeholder="(optional — uses the default catalog integration)"
              loading={loadingCatalogs}
              notFoundContent={loadingCatalogs ? "Loading…" : "No catalog integrations found"}
            />
          </Form.Item>
        )}

        {/* Snowflake-managed: column list + base location. */}
        {isSnowflake && (
          <>
            <Form.Item label="Base location" required style={itemStyle} help="Directory within the external volume where Snowflake writes the table's data and metadata.">
              <Input
                value={cfg.baseLocation}
                onChange={(e) => set("baseLocation", e.target.value)}
                placeholder="my_db/my_schema/my_table"
              />
            </Form.Item>
            <Form.Item label="Columns" required style={{ marginBottom: 8 }}>
              {columnsBody}
            </Form.Item>
          </>
        )}

        {/* External Iceberg catalog (REST / AWS Glue): catalog table name + namespace. */}
        {isExternalCatalog && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
            <Form.Item label="Catalog table name" required style={itemStyle} help="Table identifier as recognized by the external catalog (no namespace prefix).">
              <Input
                value={cfg.catalogTableName}
                onChange={(e) => set("catalogTableName", e.target.value)}
                placeholder="orders"
              />
            </Form.Item>
            <Form.Item label="Catalog namespace" style={itemStyle} help="Namespace/database in the catalog. Optional when a default is set on the integration.">
              <Input
                value={cfg.catalogNamespace}
                onChange={(e) => set("catalogNamespace", e.target.value)}
                placeholder="(optional)"
              />
            </Form.Item>
          </div>
        )}

        {/* Delta Lake files: base location pointing at the directory with _delta_log/. */}
        {isDelta && (
          <Form.Item label="Base location" required style={itemStyle} help="Relative path from the external volume to the directory containing the Delta files (with a _delta_log/ subfolder).">
            <Input
              value={cfg.baseLocation}
              onChange={(e) => set("baseLocation", e.target.value)}
              placeholder="path/to/delta-table"
            />
          </Form.Item>
        )}

        {/* Iceberg files in object storage: metadata file path. */}
        {isIcebergFiles && (
          <Form.Item label="Metadata file path" required style={itemStyle} help="Relative path (from the external volume) to the Iceberg metadata file, e.g. path/to/metadata/v1.metadata.json — no leading slash.">
            <Input
              value={cfg.metadataFilePath}
              onChange={(e) => set("metadataFilePath", e.target.value)}
              placeholder="path/to/metadata/v1.metadata.json"
            />
          </Form.Item>
        )}

        {/* Shared external-variant options: REPLACE_INVALID_CHARACTERS (all
            external variants) + AUTO_REFRESH (external catalog + Delta only). */}
        {usesCatalog && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
            <Form.Item label="Replace invalid chars" style={itemStyle} help="Replace invalid UTF-8 characters with the Unicode replacement character.">
              <Select
                allowClear
                value={cfg.replaceInvalidCharacters || undefined}
                onChange={(v) => set("replaceInvalidCharacters", v ?? "")}
                placeholder="(default — FALSE)"
                options={[{ value: "TRUE", label: "TRUE" }, { value: "FALSE", label: "FALSE" }]}
              />
            </Form.Item>
            {usesAutoRefresh && (
              <Form.Item label="Auto refresh" style={itemStyle} help="Automatically poll the source for metadata updates.">
                <Select
                  allowClear
                  value={cfg.autoRefresh || undefined}
                  onChange={(v) => set("autoRefresh", v ?? "")}
                  placeholder="(default — FALSE)"
                  options={[{ value: "TRUE", label: "TRUE" }, { value: "FALSE", label: "FALSE" }]}
                />
              </Form.Item>
            )}
          </div>
        )}

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
