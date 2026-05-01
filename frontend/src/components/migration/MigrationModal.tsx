// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties found in a valid
// license agreement with Technarion Oy.

import { useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Descriptions,
  Divider,
  Input,
  Modal,
  Progress,
  Radio,
  Select,
  Space,
  Steps,
  Tag,
  Typography,
  message,
} from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { DiffEditor } from "@monaco-editor/react";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { AgGridReact } from "ag-grid-react";
import type { ColDef } from "ag-grid-community";
import {
  ScanMigrationSource,
  AnalyzeMigration,
  CreateMigrationSnapshot,
  ExecuteMigration,
  CancelMigration,
  GenerateMigrationScript,
  ListDatabases,
  PickDirectory,
} from "../../../wailsjs/go/main/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { useThemeStore } from "../../store/themeStore";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

// ─── backend types (mirrors migration.go structs) ─────────────────────────────

interface MigrationObject {
  filePath: string;
  database: string;
  schema: string;
  objectKind: string;
  objectName: string;
  argSig: string;
  ddl: string;
  isReplace: boolean;
}

interface MigrationDiffItem {
  object: MigrationObject;
  status: "new" | "changed" | "unchanged" | "removed";
  localDDL: string;
  remoteDDL: string;
}

interface MigrationAnalyzeProgress {
  done: number;
  total: number;
}

interface MigrationExecEvent {
  done: number;
  total: number;
  object: string;
  status: "running" | "success" | "failed" | "skipped";
  error: string;
  pass: number;
}

// ─── multi-db types ───────────────────────────────────────────────────────────

interface SourceMapping {
  id: string;
  sourceDir: string;
  targetDB: string;
}

interface DBProtection {
  database: string;
  doBackup: boolean;
  backupSetDB: string;
  backupSetSchema: string;
  backupSetName: string;
  doClone: boolean;
  cloneDB: string;
}

// ─── helpers ──────────────────────────────────────────────────────────────────

function statusColor(status: string): string {
  switch (status) {
    case "new":
      return "green";
    case "changed":
      return "orange";
    case "unchanged":
      return "default";
    case "removed":
      return "red";
    case "success":
      return "success";
    case "failed":
      return "error";
    case "skipped":
      return "warning";
    case "running":
      return "processing";
    default:
      return "default";
  }
}

function objectLabel(mo: MigrationObject): string {
  return `${mo.database}.${mo.schema}.${mo.objectKind}.${mo.objectName}`;
}

function kindCounts(items: MigrationObject[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const item of items) {
    const k = item.objectKind;
    counts[k] = (counts[k] ?? 0) + 1;
  }
  return counts;
}

// Extract bare table/view names from a DDL string (FROM/JOIN references)
function extractReferencedNames(ddl: string): string[] {
  const re = /(?:FROM|JOIN)\s+"?(\w+)"?/gi;
  const names: string[] = [];
  let m: RegExpExecArray | null;
  while ((m = re.exec(ddl)) !== null) {
    names.push(m[1].toUpperCase());
  }
  return names;
}

let nextMappingId = 1;

// ─── MigrationModal ───────────────────────────────────────────────────────────

export default function MigrationModal({ onClose }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const monacoTheme = resolved === "dark" ? "vs-dark" : "vs";
  const loadInNewTab = useQueryStore((s) => s.loadInNewTab);
  const [messageApi, contextHolder] = message.useMessage();

  const [step, setStep] = useState(0);

  // Step 0 — Configure
  const [mappings, setMappings] = useState<SourceMapping[]>([
    { id: "0", sourceDir: "", targetDB: "" },
  ]);
  const [databases, setDatabases] = useState<string[]>([]);
  const [scanning, setScanning] = useState(false);

  // Step 1 — Scan results
  const [scanObjects, setScanObjects] = useState<MigrationObject[]>([]);
  const [scannedMappingCount, setScannedMappingCount] = useState(0);

  // Step 2 — Review
  const [analyzing, setAnalyzing] = useState(false);
  const [analyzeProgress, setAnalyzeProgress] = useState({ done: 0, total: 0 });
  const [diffItems, setDiffItems] = useState<MigrationDiffItem[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [activeDiff, setActiveDiff] = useState<MigrationDiffItem | null>(null);
  const [statusFilter, setStatusFilter] = useState<"all" | "new" | "changed">("all");
  const gridRef = useRef<AgGridReact>(null);

  // Step 3 — Strategy & Protect
  const [tableStrategy, setTableStrategy] = useState<string>("in_place");
  const [dbProtections, setDbProtections] = useState<Record<string, DBProtection>>({});
  const [snapshotting, setSnapshotting] = useState(false);

  // Step 4 — Deploy
  const [execEvents, setExecEvents] = useState<MigrationExecEvent[]>([]);
  const [deployDone, setDeployDone] = useState(false);
  const [latestProgress, setLatestProgress] = useState({ done: 0, total: 0 });

  // Load databases on mount
  useEffect(() => {
    ListDatabases().then(setDatabases).catch(() => {});
  }, []);

  // ── Mapping helpers ────────────────────────────────────────────────────────

  function addMapping() {
    setMappings((prev) => [
      ...prev,
      { id: String(nextMappingId++), sourceDir: "", targetDB: "" },
    ]);
  }

  function removeMapping(id: string) {
    setMappings((prev) => prev.filter((m) => m.id !== id));
  }

  function updateMapping(id: string, updates: Partial<SourceMapping>) {
    setMappings((prev) =>
      prev.map((m) => (m.id === id ? { ...m, ...updates } : m))
    );
  }

  async function handlePickDir(id: string) {
    const dir = await PickDirectory().catch(() => "");
    if (dir) updateMapping(id, { sourceDir: dir });
  }

  // ── Step 0 ────────────────────────────────────────────────────────────────

  async function handleScan() {
    const validMappings = mappings.filter((m) => m.sourceDir);
    if (validMappings.length === 0) return;
    setScanning(true);
    try {
      const allObjects = new Map<string, MigrationObject>();
      for (const mapping of validMappings) {
        const objects = await ScanMigrationSource(mapping.sourceDir) as unknown as MigrationObject[];
        for (const obj of objects) {
          const withDB = { ...obj, database: obj.database || mapping.targetDB };
          allObjects.set(objectLabel(withDB), withDB);
        }
      }
      setScanObjects([...allObjects.values()]);
      setScannedMappingCount(validMappings.length);
      setStep(1);
    } catch (e: any) {
      messageApi.error("Scan failed: " + (e?.message ?? String(e)));
    } finally {
      setScanning(false);
    }
  }

  // ── Step 1 ────────────────────────────────────────────────────────────────

  async function handleAnalyze() {
    setAnalyzing(true);
    setAnalyzeProgress({ done: 0, total: scanObjects.length });

    const off = EventsOn("migration:analyze:progress", (data: MigrationAnalyzeProgress) => {
      setAnalyzeProgress({ done: data.done, total: data.total });
    });

    const fallbackDB = mappings[0]?.targetDB || "";

    try {
      const rawItems = await AnalyzeMigration(scanObjects as any, fallbackDB);
      const items = rawItems as unknown as MigrationDiffItem[];
      setDiffItems(items);
      // Pre-select new + changed
      const keys = new Set<string>();
      for (const item of items) {
        if (item.status === "new" || item.status === "changed") {
          keys.add(objectLabel(item.object));
        }
      }
      setSelectedKeys(keys);
      setStep(2);
    } catch (e: any) {
      messageApi.error("Analysis failed: " + (e?.message ?? String(e)));
    } finally {
      off();
      setAnalyzing(false);
    }
  }

  // ── Step 2 — dependency helpers ───────────────────────────────────────────

  const diffItemByName = useCallback(
    (name: string) => diffItems.find((d) => d.object.objectName.toUpperCase() === name),
    [diffItems]
  );

  // When a row is checked: auto-select dependencies if they are new/changed
  function handleCheck(item: MigrationDiffItem, checked: boolean) {
    const key = objectLabel(item.object);
    setSelectedKeys((prev) => {
      const next = new Set(prev);

      if (checked) {
        next.add(key);
        // Auto-select referenced tables/views that are new or changed
        const refs = extractReferencedNames(item.localDDL);
        let autoCount = 0;
        for (const ref of refs) {
          const dep = diffItemByName(ref);
          if (dep && (dep.status === "new" || dep.status === "changed")) {
            const depKey = objectLabel(dep.object);
            if (!next.has(depKey)) {
              next.add(depKey);
              autoCount++;
            }
          }
        }
        if (autoCount > 0) {
          messageApi.info(`Auto-selected ${autoCount} dependenc${autoCount === 1 ? "y" : "ies"}`);
        }
      } else {
        // Block uncheck if a currently-selected view/procedure depends on this table
        const blocked = diffItems.find((d) => {
          if (!next.has(objectLabel(d.object))) return false;
          const kind = d.object.objectKind.toUpperCase();
          if (kind !== "VIEW" && kind !== "PROCEDURE" && kind !== "MATERIALIZED VIEW") return false;
          const refs = extractReferencedNames(d.localDDL);
          return refs.includes(item.object.objectName.toUpperCase());
        });
        if (blocked) {
          messageApi.warning(`Required by: ${blocked.object.objectName}`);
          return prev; // no change
        }
        next.delete(key);
      }

      return next;
    });
  }

  function handleSelectAllNewChanged() {
    const keys = new Set<string>();
    for (const item of diffItems) {
      if (item.status === "new" || item.status === "changed") {
        keys.add(objectLabel(item.object));
      }
    }
    setSelectedKeys(keys);
  }

  async function handleOpenInEditor() {
    const selectedItems = diffItems.filter((d) => selectedKeys.has(objectLabel(d.object)));
    const fallbackDB = mappings[0]?.targetDB || "";
    try {
      const sql = await GenerateMigrationScript(
        selectedItems as any,
        fallbackDB,
        tableStrategy
      ) as unknown as string;
      loadInNewTab(sql);
      onClose();
    } catch (e: any) {
      messageApi.error("Failed to generate script: " + (e?.message ?? String(e)));
    }
  }

  // Grid column definitions
  const reviewCols: ColDef<MigrationDiffItem>[] = [
    {
      headerName: "",
      width: 44,
      pinned: "left",
      cellRenderer: (params: { data?: MigrationDiffItem }) => {
        if (!params.data) return null;
        const item = params.data;
        const key = objectLabel(item.object);
        return (
          <Checkbox
            checked={selectedKeys.has(key)}
            disabled={item.status === "removed"}
            onChange={(e) => handleCheck(item, e.target.checked)}
          />
        );
      },
    },
    {
      headerName: "Status",
      width: 110,
      field: "status",
      cellRenderer: (params: { value?: string }) => (
        <Tag color={statusColor(params.value ?? "")}>{(params.value ?? "").toUpperCase()}</Tag>
      ),
    },
    { headerName: "Kind", width: 120, valueGetter: (p) => p.data?.object.objectKind },
    { headerName: "Name", flex: 1, valueGetter: (p) => p.data?.object.objectName },
    { headerName: "Schema", width: 120, valueGetter: (p) => p.data?.object.schema },
    { headerName: "Database", width: 130, valueGetter: (p) => p.data?.object.database },
    {
      headerName: "File",
      flex: 1,
      valueGetter: (p) =>
        p.data?.object.filePath
          ? p.data.object.filePath.split("/").pop()
          : "",
    },
  ];

  const filteredDiff = diffItems.filter((item) => {
    if (statusFilter === "all") return true;
    return item.status === statusFilter;
  });

  // ── Step 3 helpers ─────────────────────────────────────────────────────────

  function getSelectedDBs(): string[] {
    const dbs = new Set<string>();
    for (const key of selectedKeys) {
      const item = diffItems.find((d) => objectLabel(d.object) === key);
      if (item?.object.database) dbs.add(item.object.database);
    }
    return [...dbs].sort();
  }

  function getDBProtection(database: string): DBProtection {
    return (
      dbProtections[database] ?? {
        database,
        doBackup: false,
        backupSetDB: "",
        backupSetSchema: "",
        backupSetName: "",
        doClone: false,
        cloneDB: "",
      }
    );
  }

  function updateDBProtection(database: string, updates: Partial<DBProtection>) {
    setDbProtections((prev) => {
      const existing: DBProtection = prev[database] ?? {
        database,
        doBackup: false,
        backupSetDB: "",
        backupSetSchema: "",
        backupSetName: "",
        doClone: false,
        cloneDB: "",
      };
      return { ...prev, [database]: { ...existing, ...updates } };
    });
  }

  async function handleProtectAndDeploy() {
    setSnapshotting(true);
    try {
      for (const prot of Object.values(dbProtections)) {
        if (!prot.doBackup && !prot.doClone) continue;
        await CreateMigrationSnapshot(
          prot.database,
          prot.backupSetDB,
          prot.backupSetSchema,
          prot.backupSetName,
          prot.doBackup,
          prot.cloneDB,
          prot.doClone
        );
      }
    } catch (e: any) {
      messageApi.error("Snapshot failed: " + (e?.message ?? String(e)));
      setSnapshotting(false);
      return;
    }
    setSnapshotting(false);
    setStep(4);
    handleDeploy();
  }

  // ── Step 4 ────────────────────────────────────────────────────────────────

  async function handleDeploy() {
    setExecEvents([]);
    setDeployDone(false);
    setLatestProgress({ done: 0, total: selectedKeys.size });

    const selectedObjects = diffItems
      .filter((d) => selectedKeys.has(objectLabel(d.object)))
      .map((d) => d.object);

    const fallbackDB = mappings[0]?.targetDB || "";

    const off = EventsOn("migration:exec:progress", (evt: MigrationExecEvent) => {
      setExecEvents((prev) => [...prev, evt]);
      setLatestProgress({ done: evt.done, total: evt.total });
    });

    try {
      await ExecuteMigration(selectedObjects as any, fallbackDB, 5, tableStrategy);
    } catch (e: any) {
      messageApi.error("Deploy error: " + (e?.message ?? String(e)));
    } finally {
      off();
      setDeployDone(true);
    }
  }

  async function handleCancel() {
    await CancelMigration().catch(() => {});
  }

  // ── Render steps ──────────────────────────────────────────────────────────

  function renderStep0() {
    const hasAnyDir = mappings.some((m) => m.sourceDir);

    return (
      <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>
            Source Mappings
          </Text>
          <Text type="secondary" style={{ fontSize: 12, marginBottom: 12, display: "block" }}>
            Map each source directory to its target database. The target database is used as the
            fallback for objects without an explicit USE DATABASE context.
          </Text>
          <Space direction="vertical" style={{ width: "100%", gap: 8 }}>
            {mappings.map((mapping) => (
              <div key={mapping.id} style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <Space.Compact style={{ flex: 1 }}>
                  <Input
                    value={mapping.sourceDir}
                    onChange={(e) => updateMapping(mapping.id, { sourceDir: e.target.value })}
                    placeholder="/path/to/sql/files"
                    style={{ fontFamily: "monospace" }}
                  />
                  <Button onClick={() => handlePickDir(mapping.id)}>Browse…</Button>
                </Space.Compact>
                <Select
                  showSearch
                  value={mapping.targetDB || undefined}
                  onChange={(v) => updateMapping(mapping.id, { targetDB: v })}
                  options={databases.map((d) => ({ value: d, label: d }))}
                  placeholder="Target DB (optional)"
                  style={{ width: 200 }}
                />
                {mappings.length > 1 && (
                  <Button
                    type="text"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => removeMapping(mapping.id)}
                  />
                )}
              </div>
            ))}
          </Space>
          <Button
            type="dashed"
            icon={<PlusOutlined />}
            onClick={addMapping}
            style={{ marginTop: 8 }}
          >
            Add Database
          </Button>
        </div>

        <div style={{ display: "flex", justifyContent: "flex-end" }}>
          <Button
            type="primary"
            onClick={handleScan}
            loading={scanning}
            disabled={!hasAnyDir}
          >
            Scan
          </Button>
        </div>
      </Space>
    );
  }

  function renderStep1() {
    const counts = kindCounts(scanObjects);
    const srcLabel =
      scannedMappingCount === 1
        ? "1 source directory"
        : `${scannedMappingCount} source directories`;

    return (
      <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
        <div>
          <Text type="secondary">
            Found <strong>{scanObjects.length}</strong> object{scanObjects.length !== 1 ? "s" : ""}{" "}
            across <strong>{srcLabel}</strong>.
          </Text>
        </div>

        <Descriptions bordered size="small" column={3}>
          {Object.entries(counts).map(([kind, count]) => (
            <Descriptions.Item key={kind} label={kind}>
              {count}
            </Descriptions.Item>
          ))}
        </Descriptions>

        {analyzing && (
          <div>
            <Progress
              percent={
                analyzeProgress.total > 0
                  ? Math.round((analyzeProgress.done / analyzeProgress.total) * 100)
                  : 0
              }
              status="active"
              format={() => `${analyzeProgress.done} / ${analyzeProgress.total}`}
            />
          </div>
        )}

        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <Button onClick={() => setStep(0)}>Back</Button>
          <Button
            type="primary"
            onClick={handleAnalyze}
            loading={analyzing}
            disabled={scanObjects.length === 0}
          >
            Analyze
          </Button>
        </div>
      </Space>
    );
  }

  function renderStep2() {
    return (
      <Space direction="vertical" style={{ width: "100%", gap: 12 }}>
        <Space wrap>
          <Text type="secondary">
            <strong>{selectedKeys.size}</strong> selected
          </Text>
          <Button size="small" onClick={handleSelectAllNewChanged}>
            Select All New + Changed
          </Button>
          <Space>
            <Text type="secondary" style={{ fontSize: 12 }}>
              Filter:
            </Text>
            {(["all", "new", "changed"] as const).map((f) => (
              <Button
                key={f}
                size="small"
                type={statusFilter === f ? "primary" : "default"}
                onClick={() => setStatusFilter(f)}
              >
                {f.charAt(0).toUpperCase() + f.slice(1)}
              </Button>
            ))}
          </Space>
        </Space>

        <div
          className={resolved === "dark" ? "ag-theme-alpine-dark" : "ag-theme-alpine"}
          style={{ height: 260, width: "100%" }}
        >
          <AgGridReact
            ref={gridRef}
            rowData={filteredDiff}
            columnDefs={reviewCols}
            rowSelection="single"
            onRowClicked={(e) => {
              if (e.data) setActiveDiff(e.data as MigrationDiffItem);
            }}
            defaultColDef={{ resizable: true }}
            suppressMovableColumns
          />
        </div>

        {activeDiff && (
          <div style={{ height: 240, border: "1px solid var(--border)", borderRadius: 4, overflow: "hidden" }}>
            <DiffEditor
              theme={monacoTheme}
              language="sql"
              original={activeDiff.localDDL}
              modified={activeDiff.remoteDDL}
              onMount={(editor) => patchMonacoClipboard(editor)}
              options={{
                readOnly: true,
                minimap: { enabled: false },
                fontSize: 12,
                scrollBeyondLastLine: false,
              }}
            />
          </div>
        )}

        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <Button onClick={() => setStep(1)}>Back</Button>
          <Button type="primary" onClick={() => setStep(3)} disabled={selectedKeys.size === 0}>
            Continue
          </Button>
        </div>
      </Space>
    );
  }

  function renderStep3() {
    const hasSelectedTables = [...selectedKeys].some((key) => {
      const item = diffItems.find((d) => objectLabel(d.object) === key);
      return item?.object.objectKind.toUpperCase() === "TABLE";
    });

    const selectedDBs = getSelectedDBs();

    return (
      <Space direction="vertical" style={{ width: "100%", gap: 20 }}>
        {hasSelectedTables && (
          <div>
            <Text strong style={{ display: "block", marginBottom: 8 }}>
              Table Migration Strategy
            </Text>
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 10 }}>
              Controls how existing TABLE objects are updated. Has no effect on new tables or
              non-table objects.
            </Text>
            <Radio.Group
              value={tableStrategy}
              onChange={(e) => setTableStrategy(e.target.value)}
              style={{ display: "flex", flexDirection: "column", gap: 8 }}
            >
              <Radio value="in_place">
                <strong>Smart In-Place</strong>
                <Text type="secondary" style={{ fontSize: 12, marginLeft: 6 }}>
                  ADD / DROP / ALTER COLUMN — no data movement, safest for compatible changes
                </Text>
              </Radio>
              <Radio value="blue_green_swap">
                <strong>Blue/Green Swap</strong>
                <Text type="secondary" style={{ fontSize: 12, marginLeft: 6 }}>
                  Builds new schema alongside, copies shared columns, then atomically swaps
                </Text>
              </Radio>
              <Radio value="view_abstraction">
                <strong>View-Based Soft Cutover</strong>
                <Text type="secondary" style={{ fontSize: 12, marginLeft: 6 }}>
                  Renames old table to{" "}
                  <code style={{ fontSize: 11 }}>_v1</code>, creates new table, leaves compat view
                </Text>
              </Radio>
              <Radio value="destructive_rebuild">
                <strong>Destructive Rebuild</strong>
                <Text type="secondary" style={{ fontSize: 12, marginLeft: 6 }}>
                  DROP + CREATE — all existing data is permanently lost
                </Text>
              </Radio>
            </Radio.Group>
            {tableStrategy === "destructive_rebuild" && (
              <Alert
                type="error"
                message="Data Loss Warning"
                description="Destructive Rebuild will DROP existing tables before recreating them. All existing data will be permanently lost and cannot be recovered."
                showIcon
                style={{ marginTop: 10 }}
              />
            )}
          </div>
        )}

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>
            Safety Snapshot (Optional)
          </Text>
          <Text type="secondary" style={{ fontSize: 12, marginBottom: 12, display: "block" }}>
            Create a backup set or zero-copy clone before deploying each target database.
          </Text>
          {selectedDBs.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>
              No databases detected from selected objects.
            </Text>
          ) : (
            selectedDBs.map((db, idx) => {
              const prot = getDBProtection(db);
              return (
                <div key={db}>
                  {idx > 0 && <Divider style={{ margin: "12px 0" }} />}
                  <Text strong style={{ display: "block", marginBottom: 8, fontSize: 13 }}>
                    {db}
                  </Text>
                  <Space direction="vertical" style={{ width: "100%", gap: 6, paddingLeft: 8 }}>
                    <Checkbox
                      checked={prot.doBackup}
                      onChange={(e) => updateDBProtection(db, { doBackup: e.target.checked })}
                    >
                      Create Backup Set
                    </Checkbox>
                    {prot.doBackup && (
                      <div style={{ marginTop: 4, paddingLeft: 24 }}>
                        <Space direction="vertical" style={{ width: "100%", gap: 6 }}>
                          <Input
                            placeholder="Backup Set Name"
                            value={prot.backupSetName}
                            onChange={(e) => updateDBProtection(db, { backupSetName: e.target.value })}
                          />
                          <Space.Compact style={{ width: "100%" }}>
                            <Input
                              placeholder="Database"
                              value={prot.backupSetDB}
                              onChange={(e) => updateDBProtection(db, { backupSetDB: e.target.value })}
                              style={{ width: "50%" }}
                            />
                            <Input
                              placeholder="Schema"
                              value={prot.backupSetSchema}
                              onChange={(e) => updateDBProtection(db, { backupSetSchema: e.target.value })}
                              style={{ width: "50%" }}
                            />
                          </Space.Compact>
                        </Space>
                      </div>
                    )}

                    <Checkbox
                      checked={prot.doClone}
                      onChange={(e) => updateDBProtection(db, { doClone: e.target.checked })}
                    >
                      Create Zero-Copy Clone
                    </Checkbox>
                    {prot.doClone && (
                      <div style={{ marginTop: 4, paddingLeft: 24 }}>
                        <Input
                          placeholder="Clone Database Name"
                          value={prot.cloneDB}
                          onChange={(e) => updateDBProtection(db, { cloneDB: e.target.value })}
                        />
                      </div>
                    )}
                  </Space>
                </div>
              );
            })
          )}
        </div>

        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <Button onClick={() => setStep(2)}>Back</Button>
          <Space>
            <Button onClick={handleOpenInEditor} disabled={selectedKeys.size === 0}>
              Open in SQL Editor
            </Button>
            <Button type="primary" onClick={handleProtectAndDeploy} loading={snapshotting}>
              Continue to Deploy
            </Button>
          </Space>
        </div>
      </Space>
    );
  }

  function renderStep4() {
    const pct =
      latestProgress.total > 0
        ? Math.round((latestProgress.done / latestProgress.total) * 100)
        : 0;

    // Exec events table (only terminal events — skip "running")
    const terminalEvents = execEvents.filter(
      (e) => e.status === "success" || e.status === "failed" || e.status === "skipped"
    );

    const execCols: ColDef<MigrationExecEvent>[] = [
      { headerName: "Pass", width: 70, field: "pass" },
      {
        headerName: "Kind",
        width: 130,
        valueGetter: (p) => {
          const parts = (p.data?.object ?? "").split(".");
          return parts[2] ?? "";
        },
      },
      {
        headerName: "Name",
        flex: 1,
        valueGetter: (p) => {
          const parts = (p.data?.object ?? "").split(".");
          return parts[3] ?? p.data?.object ?? "";
        },
      },
      {
        headerName: "Status",
        width: 110,
        field: "status",
        cellRenderer: (params: { value?: string }) => (
          <Tag color={statusColor(params.value ?? "")}>{(params.value ?? "").toUpperCase()}</Tag>
        ),
      },
      { headerName: "Error", flex: 2, field: "error" },
    ];

    return (
      <Space direction="vertical" style={{ width: "100%", gap: 12 }}>
        <Progress
          percent={pct}
          status={deployDone ? "normal" : "active"}
          format={() => `${latestProgress.done} / ${latestProgress.total}`}
        />

        <div
          className={resolved === "dark" ? "ag-theme-alpine-dark" : "ag-theme-alpine"}
          style={{ height: 320, width: "100%" }}
        >
          <AgGridReact
            rowData={terminalEvents}
            columnDefs={execCols}
            defaultColDef={{ resizable: true }}
            suppressMovableColumns
          />
        </div>

        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
          {!deployDone && (
            <Button danger onClick={handleCancel}>
              Cancel
            </Button>
          )}
          <Button type="primary" disabled={!deployDone} onClick={onClose}>
            Done
          </Button>
        </div>
      </Space>
    );
  }

  // ── Modal render ──────────────────────────────────────────────────────────

  const stepTitles = ["Configure", "Scan Results", "Review", "Strategy & Protect", "Deploy"];

  return (
    <>
      {contextHolder}
      <Modal
        open
        title="Schema Migration"
        width={900}
        onCancel={onClose}
        footer={null}
        destroyOnClose
      >
        <Steps
          current={step}
          size="small"
          style={{ marginBottom: 24 }}
          items={stepTitles.map((title) => ({ title }))}
        />

        {step === 0 && renderStep0()}
        {step === 1 && renderStep1()}
        {step === 2 && renderStep2()}
        {step === 3 && renderStep3()}
        {step === 4 && renderStep4()}
      </Modal>
    </>
  );
}
