import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Form,
  Input,
  Modal,
  Radio,
  Select,
  Slider,
  Tooltip,
} from "antd";
import { PlusOutlined, DeleteOutlined, CopyOutlined } from "@ant-design/icons";
import {
  ListDatabases,
  ListExternalVolumes,
  ListIntegrations,
  ExecDDL,
  GetDatabaseRetentionDays,
  GetQuotedIdentifiersIgnoreCase,
} from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import ObjectNameCaseControl, { identToken } from "../shared/ObjectNameCaseControl";

interface Tag {
  name: string;
  value: string;
}

interface DbConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  transient: boolean;
  ifNotExists: boolean;
  // Clone
  clone: boolean;
  cloneSource: string;
  cloneAt: "none" | "at" | "before";
  cloneTsMode: "timestamp" | "offset" | "statement";
  cloneTimestamp: number;
  cloneOffset: string;
  cloneStatement: string;
  ignoreInsufficientRetention: boolean;
  ignoreHybridTables: boolean;
  // Data retention
  dataRetention: string;
  maxDataExtension: string;
  // Iceberg / External storage
  externalVolume: string;
  catalog: string;
  icebergVersion: string;
  enableIcebergMergeOnRead: string; // "" | "true" | "false"
  // Storage
  replaceInvalidChars: string; // "" | "true" | "false"
  defaultDdlCollation: string;
  storageSerialization: string;
  enableDataCompaction: string; // "" | "true" | "false"
  // Catalog sync
  catalogSync: string;
  catalogSyncNamespaceMode: string;
  catalogSyncDelimiter: string;
  // Tags
  tags: Tag[];
  // Visibility & misc
  objectVisibilityMode: "none" | "privileged" | "yaml";
  objectVisibilityYaml: string;
  comment: string;
}

const NOW = Math.floor(Date.now() / 1000);

function defaultConfig(): DbConfig {
  return {
    name: "",
    caseSensitive: false,
    orReplace: false,
    transient: false,
    ifNotExists: false,
    clone: false,
    cloneSource: "",
    cloneAt: "none",
    cloneTsMode: "timestamp",
    cloneTimestamp: NOW,
    cloneOffset: "",
    cloneStatement: "",
    ignoreInsufficientRetention: false,
    ignoreHybridTables: false,
    dataRetention: "",
    maxDataExtension: "",
    externalVolume: "",
    catalog: "",
    icebergVersion: "",
    enableIcebergMergeOnRead: "",
    replaceInvalidChars: "",
    defaultDdlCollation: "",
    storageSerialization: "",
    enableDataCompaction: "",
    catalogSync: "",
    catalogSyncNamespaceMode: "",
    catalogSyncDelimiter: "",
    tags: [],
    objectVisibilityMode: "none",
    objectVisibilityYaml: "",
    comment: "",
  };
}

function q(s: string): string {
  return '"' + s.replace(/"/g, '""') + '"';
}

function sq(s: string): string {
  return "'" + s.replace(/'/g, "''") + "'";
}

// q() is kept for tag names, clone source, and other already-existing
// identifier references. identToken() from the shared helper is used for the
// newly-created object name so that case sensitivity is respected.

function formatTs(unix: number): string {
  return new Date(unix * 1000).toISOString().replace("T", " ").replace("Z", "");
}

function buildSql(cfg: DbConfig): string {
  const lines: string[] = [];

  const nameToken = identToken(cfg.name, cfg.caseSensitive);

  const parts = ["CREATE"];
  if (cfg.orReplace) parts.push("OR REPLACE");
  if (cfg.transient) parts.push("TRANSIENT");
  parts.push("DATABASE");
  if (cfg.ifNotExists && !cfg.orReplace) parts.push("IF NOT EXISTS");
  parts.push(nameToken);
  lines.push(parts.join(" "));

  if (cfg.clone && cfg.cloneSource) {
    let cloneLine = `  CLONE ${q(cfg.cloneSource)}`;
    if (cfg.cloneAt !== "none") {
      let timeExpr = "";
      if (cfg.cloneTsMode === "timestamp") {
        timeExpr = `TIMESTAMP => '${formatTs(cfg.cloneTimestamp)}'::TIMESTAMP_LTZ`;
      } else if (cfg.cloneTsMode === "offset" && cfg.cloneOffset) {
        timeExpr = `OFFSET => ${cfg.cloneOffset}`;
      } else if (cfg.cloneTsMode === "statement" && cfg.cloneStatement) {
        timeExpr = `STATEMENT => '${cfg.cloneStatement.replace(/'/g, "''")}'`;
      }
      if (timeExpr) cloneLine += ` ${cfg.cloneAt.toUpperCase()} (${timeExpr})`;
    }
    lines.push(cloneLine);
    if (cfg.ignoreInsufficientRetention)
      lines.push("  IGNORE TABLES WITH INSUFFICIENT DATA RETENTION");
    if (cfg.ignoreHybridTables) lines.push("  IGNORE HYBRID TABLES");
  }

  if (cfg.dataRetention !== "") lines.push(`  DATA_RETENTION_TIME_IN_DAYS = ${cfg.dataRetention}`);
  if (cfg.maxDataExtension !== "") lines.push(`  MAX_DATA_EXTENSION_TIME_IN_DAYS = ${cfg.maxDataExtension}`);
  if (cfg.externalVolume) lines.push(`  EXTERNAL_VOLUME = ${q(cfg.externalVolume)}`);
  if (cfg.catalog) lines.push(`  CATALOG = ${q(cfg.catalog)}`);
  if (cfg.icebergVersion) lines.push(`  ICEBERG_VERSION_DEFAULT = ${cfg.icebergVersion}`);
  if (cfg.enableIcebergMergeOnRead) lines.push(`  ENABLE_ICEBERG_MERGE_ON_READ = ${cfg.enableIcebergMergeOnRead.toUpperCase()}`);
  if (cfg.replaceInvalidChars) lines.push(`  REPLACE_INVALID_CHARACTERS = ${cfg.replaceInvalidChars.toUpperCase()}`);
  if (cfg.defaultDdlCollation) lines.push(`  DEFAULT_DDL_COLLATION = ${sq(cfg.defaultDdlCollation)}`);
  if (cfg.storageSerialization) lines.push(`  STORAGE_SERIALIZATION_POLICY = ${cfg.storageSerialization}`);
  if (cfg.enableDataCompaction) lines.push(`  ENABLE_DATA_COMPACTION = ${cfg.enableDataCompaction.toUpperCase()}`);
  if (cfg.catalogSync) lines.push(`  CATALOG_SYNC = ${q(cfg.catalogSync)}`);
  if (cfg.catalogSyncNamespaceMode) lines.push(`  CATALOG_SYNC_NAMESPACE_MODE = ${cfg.catalogSyncNamespaceMode}`);
  if (cfg.catalogSyncNamespaceMode === "FLATTEN" && cfg.catalogSyncDelimiter)
    lines.push(`  CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER = ${sq(cfg.catalogSyncDelimiter)}`);
  if (cfg.objectVisibilityMode === "privileged") lines.push("  OBJECT_VISIBILITY = PRIVILEGED");
  else if (cfg.objectVisibilityMode === "yaml" && cfg.objectVisibilityYaml)
    lines.push(`  OBJECT_VISIBILITY = $$${cfg.objectVisibilityYaml}$$`);
  if (cfg.comment) lines.push(`  COMMENT = ${sq(cfg.comment)}`);

  const validTags = cfg.tags.filter((t) => t.name);
  if (validTags.length > 0) {
    lines.push(`  WITH TAG (${validTags.map((t) => `${q(t.name)} = ${sq(t.value)}`).join(", ")})`);
  }

  return lines.join("\n") + ";";
}

const TRISTATE_OPTIONS = [
  { value: "", label: "Not set" },
  { value: "true", label: "TRUE" },
  { value: "false", label: "FALSE" },
];

const STORAGE_SERIALIZATION_OPTIONS = [
  { value: "", label: "Not set" },
  { value: "COMPATIBLE", label: "COMPATIBLE" },
  { value: "OPTIMIZED", label: "OPTIMIZED" },
];

const ICEBERG_VERSION_OPTIONS = [
  { value: "", label: "(default)" },
  { value: "2", label: "2" },
  { value: "3", label: "3" },
];

const NAMESPACE_MODE_OPTIONS = [
  { value: "", label: "Not set" },
  { value: "NEST", label: "NEST" },
  { value: "FLATTEN", label: "FLATTEN" },
];

function sectionHeader(label: string) {
  return (
    <div
      style={{
        fontSize: 11,
        fontWeight: 600,
        color: "var(--text-muted)",
        textTransform: "uppercase",
        letterSpacing: "0.05em",
        borderBottom: "1px solid var(--border)",
        paddingBottom: 4,
        marginBottom: 12,
        marginTop: 20,
      }}
    >
      {label}
    </div>
  );
}

interface Props {
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateDatabaseModal({ onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<DbConfig>(defaultConfig);
  const [databases, setDatabases] = useState<string[]>([]);
  const [externalVolumes, setExternalVolumes] = useState<string[]>([]);
  const [catalogIntegrations, setCatalogIntegrations] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cloneSourceRetentionDays, setCloneSourceRetentionDays] = useState<number | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);

  useEffect(() => {
    // Use Promise.resolve().then() so synchronous throws (e.g. method not yet
    // registered) become rejections caught by .catch(). Use ?? [] because Go nil
    // slices serialize to JSON null, not [], causing .map() to throw on re-render.
    Promise.resolve()
      .then(() => ListDatabases())
      .then((dbs) => setDatabases(dbs ?? []))
      .catch(() => {});
    Promise.resolve()
      .then(() => ListExternalVolumes())
      .then((vols) => setExternalVolumes(vols ?? []))
      .catch(() => {});
    Promise.resolve()
      .then(() => ListIntegrations("CATALOG"))
      .then((rows) => setCatalogIntegrations((rows ?? []).map((r) => r.name)))
      .catch(() => {});
    Promise.resolve()
      .then(() => GetQuotedIdentifiersIgnoreCase())
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
  }, []);

  // When clone source DB changes, fetch its actual retention period and clamp
  // the timestamp to stay within the available time-travel window.
  const cloneSource = cfg.cloneSource;
  useEffect(() => {
    if (!cloneSource) {
      setCloneSourceRetentionDays(null);
      return;
    }
    let cancelled = false;
    setCloneSourceRetentionDays(null); // show loading state
    Promise.resolve()
      .then(() => GetDatabaseRetentionDays(cloneSource))
      .then((days) => {
        if (cancelled) return;
        const d = days ?? 1;
        setCloneSourceRetentionDays(d);
        const newMin = NOW - d * 24 * 3600;
        setCfg((prev) => ({
          ...prev,
          cloneTimestamp: Math.max(newMin, Math.min(prev.cloneTimestamp, NOW)),
        }));
      })
      .catch(() => {
        if (!cancelled) setCloneSourceRetentionDays(1);
      });
    return () => { cancelled = true; };
  }, [cloneSource]);

  function patch<K extends keyof DbConfig>(key: K, value: DbConfig[K]) {
    setCfg((prev) => ({ ...prev, [key]: value }));
  }


  // Clone time-travel slider bounds derived from the source DB's retention period.
  const retentionDays = cloneSourceRetentionDays ?? 1;
  const cloneSliderMin = NOW - retentionDays * 24 * 3600;
  const noTimeTravel = cloneSourceRetentionDays === 0;

  const sql = buildSql(cfg);

  async function handleSubmit() {
    setError(null);
    setSubmitting(true);
    try {
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    } catch (e: unknown) {
      setError(String(e));
    } finally {
      setSubmitting(false);
    }
  }

  const dbOptions = databases.map((d) => ({ value: d, label: d }));
  const evOptions = externalVolumes.map((v) => ({ value: v, label: v }));
  const catOptions = catalogIntegrations.map((c) => ({ value: c, label: c }));

  return (
    <Modal
      open
      title="Create Database"
      width={700}
      onCancel={onClose}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
      footer={[
        <Button key="cancel" onClick={onClose}>
          Cancel
        </Button>,
        <Button
          key="create"
          type="primary"
          disabled={!cfg.name.trim()}
          loading={submitting}
          onClick={handleSubmit}
        >
          Create
        </Button>,
      ]}
    >
      <Form layout="vertical" size="small">

        {/* ── Name ── */}
        {sectionHeader("Name")}
        <Form.Item label="Database name" style={{ marginBottom: 8 }}>
          <Input
            autoFocus
            value={cfg.name}
            onChange={(e) => patch("name", e.target.value)}
            placeholder="MY_DATABASE"
          />
        </Form.Item>
        <div style={{ display: "flex", gap: 16, flexWrap: "wrap", marginBottom: 8 }}>
          <Checkbox
            checked={cfg.orReplace}
            onChange={(e) => {
              patch("orReplace", e.target.checked);
              if (e.target.checked) patch("ifNotExists", false);
            }}
          >
            OR REPLACE
          </Checkbox>
          <Checkbox
            checked={cfg.transient}
            onChange={(e) => patch("transient", e.target.checked)}
          >
            TRANSIENT
          </Checkbox>
          <Checkbox
            checked={cfg.ifNotExists}
            disabled={cfg.orReplace}
            onChange={(e) => patch("ifNotExists", e.target.checked)}
          >
            IF NOT EXISTS
          </Checkbox>
        </div>
        <Form.Item label="Identifier case" style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => patch("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        {/* ── Clone ── */}
        {sectionHeader("Clone")}
        <Checkbox
          checked={cfg.clone}
          onChange={(e) => patch("clone", e.target.checked)}
          style={{ marginBottom: 10 }}
        >
          Clone from existing database
        </Checkbox>
        {cfg.clone && (
          <div style={{ paddingLeft: 16, borderLeft: "2px solid var(--border)", marginBottom: 8 }}>
            <Form.Item label="Source database" style={{ marginBottom: 8 }}>
              <Select
                showSearch
                value={cfg.cloneSource || undefined}
                onChange={(v) => patch("cloneSource", v ?? "")}
                placeholder="Select database…"
                style={{ width: "100%" }}
                options={dbOptions}
              />
            </Form.Item>
            <Form.Item label="Time travel" style={{ marginBottom: noTimeTravel ? 4 : 8 }}>
              <Radio.Group
                value={cfg.cloneAt}
                onChange={(e) => patch("cloneAt", e.target.value)}
                disabled={noTimeTravel}
              >
                <Radio value="none">None</Radio>
                <Radio value="at">AT</Radio>
                <Radio value="before">BEFORE</Radio>
              </Radio.Group>
            </Form.Item>
            {noTimeTravel && (
              <div style={{ fontSize: 12, color: "#faad14", marginBottom: 8, display: "flex", gap: 6 }}>
                <span>⚠</span>
                <span>Time travel is disabled for this database (DATA_RETENTION_TIME_IN_DAYS = 0).</span>
              </div>
            )}
            {cfg.cloneAt !== "none" && !noTimeTravel && (
              <>
                <Form.Item label="Mode" style={{ marginBottom: 8 }}>
                  <Radio.Group
                    value={cfg.cloneTsMode}
                    onChange={(e) => patch("cloneTsMode", e.target.value)}
                  >
                    <Radio value="timestamp">TIMESTAMP</Radio>
                    <Radio value="offset">OFFSET</Radio>
                    <Radio value="statement">STATEMENT</Radio>
                  </Radio.Group>
                </Form.Item>
                {cfg.cloneTsMode === "timestamp" && (
                  <Form.Item
                    label={
                      <span>
                        {`Timestamp: ${formatTs(cfg.cloneTimestamp)} UTC`}
                        {cloneSourceRetentionDays === null
                          ? <span style={{ marginLeft: 8, color: "var(--text-muted)", fontSize: 11 }}>(loading retention…)</span>
                          : <span style={{ marginLeft: 8, color: "var(--text-muted)", fontSize: 11 }}>
                              ({retentionDays} day{retentionDays !== 1 ? "s" : ""} retention)
                            </span>
                        }
                      </span>
                    }
                    style={{ marginBottom: 8 }}
                  >
                    <Slider
                      min={cloneSliderMin}
                      max={NOW}
                      step={60}
                      value={cfg.cloneTimestamp}
                      onChange={(v) => patch("cloneTimestamp", v)}
                      tooltip={{ formatter: (v) => (v ? formatTs(v) : "") }}
                      marks={{
                        [cloneSliderMin]: (
                          <span style={{ fontSize: 10, color: "var(--text-muted)", whiteSpace: "nowrap" }}>
                            {formatTs(cloneSliderMin).slice(0, 10)}
                          </span>
                        ),
                        [NOW]: (
                          <span style={{ fontSize: 10, color: "var(--text-muted)" }}>now</span>
                        ),
                      }}
                    />
                  </Form.Item>
                )}
                {cfg.cloneTsMode === "offset" && (
                  <Form.Item label="Offset (seconds, signed integer)" style={{ marginBottom: 8 }}>
                    <Input
                      value={cfg.cloneOffset}
                      onChange={(e) => patch("cloneOffset", e.target.value)}
                      placeholder="-3600"
                    />
                  </Form.Item>
                )}
                {cfg.cloneTsMode === "statement" && (
                  <Form.Item label="Query ID" style={{ marginBottom: 8 }}>
                    <Input
                      value={cfg.cloneStatement}
                      onChange={(e) => patch("cloneStatement", e.target.value)}
                      placeholder="01abc123-0000-0000-0000-000000000000"
                    />
                  </Form.Item>
                )}
              </>
            )}
            <Checkbox
              checked={cfg.ignoreInsufficientRetention}
              onChange={(e) => patch("ignoreInsufficientRetention", e.target.checked)}
              style={{ marginBottom: 6 }}
            >
              IGNORE TABLES WITH INSUFFICIENT DATA RETENTION
            </Checkbox>
            <br />
            <Checkbox
              checked={cfg.ignoreHybridTables}
              onChange={(e) => patch("ignoreHybridTables", e.target.checked)}
              style={{ marginTop: 6 }}
            >
              IGNORE HYBRID TABLES
            </Checkbox>
          </div>
        )}

        {/* ── Data Retention ── */}
        {sectionHeader("Data Retention")}
        <Form.Item
          label="DATA_RETENTION_TIME_IN_DAYS"
          tooltip="Standard Edition: 0 or 1. Enterprise Edition: 0–90 (permanent), 0 or 1 (transient)."
          style={{ marginBottom: 8 }}
        >
          <Input
            type="number"
            min={0}
            value={cfg.dataRetention}
            onChange={(e) => patch("dataRetention", e.target.value)}
            placeholder="(default)"
            style={{ width: 120 }}
          />
        </Form.Item>
        <Form.Item label="MAX_DATA_EXTENSION_TIME_IN_DAYS" style={{ marginBottom: 8 }}>
          <Input
            type="number"
            min={0}
            value={cfg.maxDataExtension}
            onChange={(e) => patch("maxDataExtension", e.target.value)}
            placeholder="(default)"
            style={{ width: 120 }}
          />
        </Form.Item>

        {/* ── Iceberg & External Storage ── */}
        {sectionHeader("Iceberg & External Storage")}
        <Form.Item label="EXTERNAL_VOLUME" style={{ marginBottom: 8 }}>
          <Select
            allowClear
            showSearch
            value={cfg.externalVolume || undefined}
            onChange={(v) => patch("externalVolume", v ?? "")}
            placeholder="(none)"
            style={{ width: "100%" }}
            options={evOptions}
          />
        </Form.Item>
        <Form.Item label="CATALOG" style={{ marginBottom: 8 }}>
          <Select
            allowClear
            showSearch
            value={cfg.catalog || undefined}
            onChange={(v) => patch("catalog", v ?? "")}
            placeholder="(none)"
            style={{ width: "100%" }}
            options={catOptions}
          />
        </Form.Item>
        <Form.Item label="ICEBERG_VERSION_DEFAULT" style={{ marginBottom: 8 }}>
          <Select
            value={cfg.icebergVersion}
            onChange={(v) => patch("icebergVersion", v)}
            style={{ width: 140 }}
            options={ICEBERG_VERSION_OPTIONS}
          />
        </Form.Item>
        <Form.Item label="ENABLE_ICEBERG_MERGE_ON_READ" style={{ marginBottom: 8 }}>
          <Select
            value={cfg.enableIcebergMergeOnRead}
            onChange={(v) => patch("enableIcebergMergeOnRead", v)}
            style={{ width: 160 }}
            options={TRISTATE_OPTIONS}
          />
        </Form.Item>

        {/* ── Storage Policy ── */}
        {sectionHeader("Storage Policy")}
        <Form.Item label="REPLACE_INVALID_CHARACTERS" style={{ marginBottom: 8 }}>
          <Select
            value={cfg.replaceInvalidChars}
            onChange={(v) => patch("replaceInvalidChars", v)}
            style={{ width: 160 }}
            options={TRISTATE_OPTIONS}
          />
        </Form.Item>
        <Form.Item label="DEFAULT_DDL_COLLATION" style={{ marginBottom: 8 }}>
          <Input
            value={cfg.defaultDdlCollation}
            onChange={(e) => patch("defaultDdlCollation", e.target.value)}
            placeholder="(none)"
            style={{ width: 200 }}
          />
        </Form.Item>
        <Form.Item label="STORAGE_SERIALIZATION_POLICY" style={{ marginBottom: 8 }}>
          <Select
            value={cfg.storageSerialization}
            onChange={(v) => patch("storageSerialization", v)}
            style={{ width: 180 }}
            options={STORAGE_SERIALIZATION_OPTIONS}
          />
        </Form.Item>
        <Form.Item label="ENABLE_DATA_COMPACTION" style={{ marginBottom: 8 }}>
          <Select
            value={cfg.enableDataCompaction}
            onChange={(v) => patch("enableDataCompaction", v)}
            style={{ width: 160 }}
            options={TRISTATE_OPTIONS}
          />
        </Form.Item>

        {/* ── Catalog Sync ── */}
        {sectionHeader("Catalog Sync")}
        <Form.Item label="CATALOG_SYNC" style={{ marginBottom: 8 }}>
          <Select
            allowClear
            showSearch
            value={cfg.catalogSync || undefined}
            onChange={(v) => patch("catalogSync", v ?? "")}
            placeholder="(none)"
            style={{ width: "100%" }}
            options={catOptions}
          />
        </Form.Item>
        <Form.Item label="CATALOG_SYNC_NAMESPACE_MODE" style={{ marginBottom: 8 }}>
          <Select
            value={cfg.catalogSyncNamespaceMode}
            onChange={(v) => {
              patch("catalogSyncNamespaceMode", v);
              if (v !== "FLATTEN") patch("catalogSyncDelimiter", "");
            }}
            style={{ width: 180 }}
            options={NAMESPACE_MODE_OPTIONS}
          />
        </Form.Item>
        {cfg.catalogSyncNamespaceMode === "FLATTEN" && (
          <Form.Item
            label="CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER"
            style={{ marginBottom: 8 }}
            validateStatus={
              cfg.catalogSyncDelimiter && !/^[0-9A-Za-z_$-]*$/.test(cfg.catalogSyncDelimiter)
                ? "error"
                : undefined
            }
            help={
              cfg.catalogSyncDelimiter && !/^[0-9A-Za-z_$-]*$/.test(cfg.catalogSyncDelimiter)
                ? "Only alphanumeric, _, $, - allowed"
                : undefined
            }
          >
            <Input
              value={cfg.catalogSyncDelimiter}
              onChange={(e) => patch("catalogSyncDelimiter", e.target.value)}
              placeholder="_"
              style={{ width: 160 }}
            />
          </Form.Item>
        )}

        {/* ── Tags ── */}
        {sectionHeader("Tags")}
        {cfg.tags.map((tag, i) => (
          <div
            key={i}
            style={{ display: "flex", gap: 8, alignItems: "center", marginBottom: 6 }}
          >
            <Input
              value={tag.name}
              onChange={(e) =>
                patch(
                  "tags",
                  cfg.tags.map((t, j) => (j === i ? { ...t, name: e.target.value } : t))
                )
              }
              placeholder="tag name"
              style={{ flex: 1 }}
            />
            <span style={{ color: "var(--text-muted)" }}>=</span>
            <Input
              value={tag.value}
              onChange={(e) =>
                patch(
                  "tags",
                  cfg.tags.map((t, j) => (j === i ? { ...t, value: e.target.value } : t))
                )
              }
              placeholder="tag value"
              style={{ flex: 1 }}
            />
            <Button
              type="text"
              size="small"
              icon={<DeleteOutlined />}
              onClick={() =>
                patch(
                  "tags",
                  cfg.tags.filter((_, j) => j !== i)
                )
              }
              danger
            />
          </div>
        ))}
        <Button
          icon={<PlusOutlined />}
          size="small"
          onClick={() => patch("tags", [...cfg.tags, { name: "", value: "" }])}
          style={{ marginTop: 4 }}
        >
          Add tag
        </Button>

        {/* ── Visibility & Comment ── */}
        {sectionHeader("Visibility & Comment")}
        <Form.Item label="OBJECT_VISIBILITY" style={{ marginBottom: 8 }}>
          <Radio.Group
            value={cfg.objectVisibilityMode}
            onChange={(e) => patch("objectVisibilityMode", e.target.value)}
          >
            <Radio value="none">Not set</Radio>
            <Radio value="privileged">PRIVILEGED</Radio>
            <Radio value="yaml">Custom YAML</Radio>
          </Radio.Group>
        </Form.Item>
        {cfg.objectVisibilityMode === "yaml" && (
          <Form.Item style={{ marginBottom: 8 }}>
            <Input.TextArea
              rows={6}
              value={cfg.objectVisibilityYaml}
              onChange={(e) => patch("objectVisibilityYaml", e.target.value)}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={"# Example:\nname_visibility:\n  - name: COLUMN_NAME\n    visibility: PROTECTED"}
            />
          </Form.Item>
        )}
        <Form.Item label="COMMENT" style={{ marginBottom: 8 }}>
          <Input
            value={cfg.comment}
            onChange={(e) => patch("comment", e.target.value)}
            placeholder="(none)"
          />
        </Form.Item>

        {/* ── SQL Preview ── */}
        {sectionHeader("SQL Preview")}
        <div style={{ position: "relative" }}>
          <pre
            style={{
              background: "var(--bg-secondary, #1e1e1e)",
              color: "var(--text-primary, #d4d4d4)",
              fontFamily: "monospace",
              fontSize: 12,
              padding: "10px 36px 10px 10px",
              borderRadius: 4,
              overflowX: "auto",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
              margin: 0,
            }}
          >
            {sql}
          </pre>
          <Tooltip title="Copy SQL">
            <Button
              type="text"
              size="small"
              icon={<CopyOutlined />}
              style={{
                position: "absolute",
                top: 6,
                right: 6,
                color: "var(--text-muted)",
              }}
              onClick={() => ClipboardSetText(sql)}
            />
          </Tooltip>
        </div>

        {error && (
          <Alert
            type="error"
            message={error}
            style={{ marginTop: 12 }}
            showIcon
          />
        )}
      </Form>
    </Modal>
  );
}
