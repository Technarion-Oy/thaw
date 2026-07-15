// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import {
  Modal, Checkbox, Input, Select, Button, Space, Typography, Radio,
  Progress, Alert, Tag, Collapse, Tooltip, message,
} from "antd";
import {
  CloudUploadOutlined,
  DatabaseOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  FolderOpenOutlined,
} from "@ant-design/icons";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import {
  ExportAllDatabasesDDL,
  CancelExport,
  ListExportableDatabases,
  ListUserSchemas,
  RevealInFinder,
} from "../../../wailsjs/go/app/App";
import { useGitStore } from "../../store/gitStore";
import { useSessionStore } from "../../store/sessionStore";
import { useConnectionStore } from "../../store/connectionStore";
import { getPlatformOS, getCachedPlatformOS, revealLabel } from "../files/platformUtil";
import type { ddl } from "../../../wailsjs/go/models";

const { Text } = Typography;

type ExportResult = ddl.ExportResult;

interface ProgressEvent {
  done: number;
  total: number;
  result: ExportResult;
}

// Values must match the ddl.Kind constants on the Go side.
const OBJECT_TYPES = [
  { value: "TABLE", label: "Tables" },
  { value: "VIEW", label: "Views" },
  { value: "FUNCTION", label: "Functions" },
  { value: "PROCEDURE", label: "Procedures" },
  { value: "SEQUENCE", label: "Sequences" },
  { value: "STAGE", label: "Stages" },
  { value: "STREAM", label: "Streams" },
  { value: "TASK", label: "Tasks" },
  { value: "FILE FORMAT", label: "File formats" },
  { value: "PIPE", label: "Pipes" },
];

const ALL_TYPES = OBJECT_TYPES.map((t) => t.value);
const DEFAULT_TEMPLATE = "{database}/{schema}/{object_type}/{object_name}.sql";

interface Props {
  onClose: () => void;
}

export default function ExportOptionsModal({ onClose }: Props) {
  const { exportDir, pickExportDir, exportPathTemplate } = useGitStore();
  const { warehouse: sessionWarehouse, warehouses, loadWarehouses } = useSessionStore();
  const isConnected = useConnectionStore((s) => s.isConnected);

  const [platformOS, setPlatformOS] = useState<string | null>(getCachedPlatformOS());
  useEffect(() => { getPlatformOS().then(setPlatformOS); }, []);
  const revealText = revealLabel(platformOS);

  // ── options state ──────────────────────────────────────────────────────────
  const [availableDbs, setAvailableDbs] = useState<string[]>([]);
  const [databases, setDatabases] = useState<string[]>([]);
  const [types, setTypes] = useState<string[]>(ALL_TYPES);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [schemaOptions, setSchemaOptions] = useState<string[]>([]);
  const [skipExisting, setSkipExisting] = useState(false);
  const [warehouse, setWarehouse] = useState<string | undefined>(undefined);
  const [template, setTemplate] = useState(exportPathTemplate || "");

  useEffect(() => {
    loadWarehouses();
    ListExportableDatabases()
      .then((list) => setAvailableDbs(list ?? []))
      .catch(() => setAvailableDbs([]));
  }, []);

  // Schema suggestions: union of schemas across the databases the export
  // will cover (selected ones, or every exportable database when none are).
  // ponytail: one SHOW SCHEMAS call per database, no cap — cheap metadata
  // queries, only fired while the dialog is open.
  const effectiveDbs = databases.length > 0 ? databases : availableDbs;
  useEffect(() => {
    if (effectiveDbs.length === 0) {
      setSchemaOptions([]);
      return;
    }
    let stale = false;
    Promise.all(
      effectiveDbs.map((db) =>
        ListUserSchemas(db)
          // Qualified values so same-named schemas in different databases
          // stay selectable individually; the backend also accepts bare
          // (typed) names, which match in every database.
          .then((l) => (l ?? []).map((s) => `${db}.${s}`))
          .catch(() => [] as string[]),
      ),
    ).then((lists) => {
      if (stale) return;
      setSchemaOptions(lists.flat().sort());
    });
    return () => { stale = true; };
  }, [databases, availableDbs]);

  // ── export state ──────────────────────────────────────────────────────────
  const [running, setRunning] = useState(false);
  const [progress, setProgress] = useState({ done: 0, total: 0 });
  const [results, setResults] = useState<ExportResult[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [finished, setFinished] = useState(false);

  const runExport = async () => {
    if (!exportDir || running) return;
    setRunning(true);
    setFinished(false);
    setResults([]);
    setError(null);
    setProgress({ done: 0, total: 0 });

    const off = EventsOn("ddl:progress", (payload: ProgressEvent) => {
      setProgress({ done: payload.done, total: payload.total });
      setResults((prev) => [...prev, payload.result]);
    });

    const allTypes = types.length === ALL_TYPES.length;
    const opts = {
      // Full selection = no filter: keeps the backend on the "export
      // everything" path and future-proofs against new kinds.
      objectTypes: allTypes ? [] : types,
      schemas,
      skipExisting,
      warehouse: warehouse ?? "",
      pathTemplate: template.trim(),
    };

    try {
      // Empty databases means "export all" on the backend. Plain object
      // literal is structurally compatible with the generated
      // DDLExportOptions class; cast per the Wails request-class gotcha.
      await ExportAllDatabasesDDL(exportDir, databases, opts as any);
    } catch (e) {
      setError(String(e));
    } finally {
      off();
      setRunning(false);
      setFinished(true);
      window.dispatchEvent(new CustomEvent("thaw:export-complete"));
    }
  };

  const pct = progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0;
  const totalFiles = results.reduce((s, r) => s + r.files, 0);
  const totalSkipped = results.reduce((s, r) => s + r.skipped, 0);
  const hasErrors = results.some((r) => (r.errors?.length ?? 0) > 0);

  const scopeLabel = databases.length === 0
    ? "All Databases"
    : databases.length === 1
      ? databases[0]
      : `${databases.length} Databases`;

  return (
    <Modal
      open
      title="Export Database DDL"
      width={560}
      onCancel={running ? undefined : onClose}
      closable={!running}
      maskClosable={false}
      keyboard={!running}
      footer={
        <Space>
          {running ? (
            <Button danger onClick={() => CancelExport()}>Cancel Export</Button>
          ) : (
            <Button onClick={onClose}>Close</Button>
          )}
          <Tooltip title={!isConnected ? "Connect to Snowflake to export" : undefined}>
            <Button
              type="primary"
              icon={<CloudUploadOutlined />}
              disabled={!isConnected || !exportDir || types.length === 0}
              loading={running}
              onClick={runExport}
            >
              {running ? `Exporting… (${progress.done}/${progress.total})` : `Export ${scopeLabel}`}
            </Button>
          </Tooltip>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>Output directory</Text>
          <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
            <Text
              style={{
                flex: 1, fontFamily: "monospace", fontSize: 12,
                color: exportDir ? "var(--text)" : "var(--text-muted)",
                overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
              }}
              title={exportDir}
            >
              {exportDir || "No directory selected"}
            </Text>
            <Button
              size="small"
              icon={<FolderOpenOutlined />}
              onClick={pickExportDir}
              disabled={running}
            >
              Choose…
            </Button>
          </div>
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>Databases</Text>
          <Select
            mode="multiple"
            value={databases}
            onChange={(v) => setDatabases(v)}
            allowClear
            showSearch
            disabled={running}
            placeholder="All databases"
            options={availableDbs.map((db) => ({ value: db, label: db }))}
            style={{ width: "100%" }}
            maxTagCount="responsive"
          />
          <Text type="secondary" style={{ fontSize: 11 }}>
            Leave empty to export all databases.
          </Text>
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>Schemas</Text>
          <Select
            mode="tags"
            value={schemas}
            onChange={(v) => setSchemas(v)}
            allowClear
            disabled={running}
            placeholder={
              databases.length === 1 ? `All schemas in ${databases[0]}` : "All schemas"
            }
            options={schemaOptions.map((s) => ({ value: s, label: s }))}
            style={{ width: "100%" }}
            maxTagCount="responsive"
            tokenSeparators={[","]}
          />
          <Text type="secondary" style={{ fontSize: 11 }}>
            Leave empty for all schemas. Suggestions are qualified as DATABASE.SCHEMA;
            a typed bare name matches that schema in every database (case-insensitive).
            Filters apply after fetching — GET_DDL always returns the whole database.
          </Text>
        </div>

        <div>
          <div style={{ display: "flex", alignItems: "center", marginBottom: 6 }}>
            <Text strong style={{ flex: 1 }}>Object types</Text>
            <Checkbox
              checked={types.length === ALL_TYPES.length}
              indeterminate={types.length > 0 && types.length < ALL_TYPES.length}
              disabled={running}
              onChange={() => setTypes(types.length === ALL_TYPES.length ? [] : ALL_TYPES)}
            >
              <Text type="secondary" style={{ fontSize: 12 }}>All</Text>
            </Checkbox>
          </div>
          <Checkbox.Group
            options={OBJECT_TYPES}
            value={types}
            disabled={running}
            onChange={(v) => setTypes(v as string[])}
            style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 2 }}
          />
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>File path template</Text>
          <Input
            value={template}
            onChange={(e) => setTemplate(e.target.value)}
            disabled={running}
            placeholder={DEFAULT_TEMPLATE}
            style={{ fontFamily: "monospace" }}
          />
          <Text type="secondary" style={{ fontSize: 11 }}>
            Applies to this export only. Overloaded functions/procedures are written as{" "}
            <span style={{ fontFamily: "monospace" }}>name__ARGTYPES.sql</span>.
          </Text>
        </div>

        <div style={{ display: "flex", gap: 24 }}>
          <div>
            <Text strong style={{ display: "block", marginBottom: 6 }}>Existing files</Text>
            <Radio.Group
              value={skipExisting}
              disabled={running}
              onChange={(e) => setSkipExisting(e.target.value)}
              options={[
                { value: false, label: "Overwrite" },
                { value: true, label: "Skip" },
              ]}
            />
          </div>
          <div style={{ flex: 1 }}>
            <Text strong style={{ display: "block", marginBottom: 6 }}>Warehouse</Text>
            <Select
              value={warehouse}
              onChange={setWarehouse}
              allowClear
              showSearch
              disabled={running}
              placeholder={sessionWarehouse ? `Session warehouse (${sessionWarehouse})` : "Session warehouse"}
              options={warehouses.map((w) => ({ value: w, label: w }))}
              style={{ width: "100%" }}
            />
          </div>
        </div>

        {running && progress.total > 0 && (
          <Progress percent={pct} size="small" format={() => `${progress.done} / ${progress.total}`} />
        )}

        {error && <Alert type="error" message={error} showIcon style={{ fontSize: 12 }} />}

        {finished && results.length > 0 && (
          <div>
            <div style={{ display: "flex", alignItems: "center", marginBottom: 6 }}>
              <Space size={4} style={{ flex: 1, flexWrap: "wrap" }}>
                <Tag icon={<CheckCircleOutlined />} color="green" style={{ margin: 0 }}>
                  {totalFiles} files
                </Tag>
                {totalSkipped > 0 && <Tag style={{ margin: 0 }}>{totalSkipped} skipped</Tag>}
                {hasErrors && (
                  <Tag icon={<WarningOutlined />} color="red" style={{ margin: 0 }}>errors</Tag>
                )}
              </Space>
              {exportDir && (
                <Tooltip title={revealText}>
                  <Button
                    type="text"
                    size="small"
                    icon={<FolderOpenOutlined />}
                    onClick={() => RevealInFinder(exportDir).catch((e) => message.error(`Could not reveal: ${String(e)}`))}
                    style={{ color: "var(--text-muted)", padding: "0 4px" }}
                  />
                </Tooltip>
              )}
            </div>
            <Collapse
              size="small"
              ghost
              style={{ maxHeight: 200, overflowY: "auto" }}
              items={results.map((r) => ({
                key: r.database,
                label: (
                  <Space size={4}>
                    <DatabaseOutlined style={{ fontSize: 11 }} />
                    <span style={{ fontSize: 12 }}>{r.database}</span>
                    <Tag style={{ fontSize: 10, margin: 0 }}>{r.files} files</Tag>
                    {(r.errors?.length ?? 0) > 0 && (
                      <Tag color="red" style={{ fontSize: 10, margin: 0 }}>
                        {r.errors!.length} err
                      </Tag>
                    )}
                  </Space>
                ),
                children:
                  (r.errors?.length ?? 0) > 0 ? (
                    <div style={{ fontSize: 11, color: "#f85149" }}>
                      {r.errors!.map((e, i) => <div key={i}>{e}</div>)}
                    </div>
                  ) : (
                    <div style={{ fontSize: 11, color: "#3fb950" }}>
                      {r.files} files written, {r.skipped} skipped
                    </div>
                  ),
              }))}
            />
          </div>
        )}
      </Space>
    </Modal>
  );
}
