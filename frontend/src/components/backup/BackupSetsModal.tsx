// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import {
  Modal,
  Button,
  Table,
  Space,
  Tag,
  Spin,
  Alert,
  Popconfirm,
  Form,
  Input,
  Select,
  Checkbox,
  Typography,
  message,
} from "antd";
import { PlusOutlined, PlusCircleOutlined, EditOutlined, DeleteOutlined, MinusCircleOutlined, ReloadOutlined, RollbackOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { ListBackupSets, CreateBackupSet, DropBackupSet, AlterBackupSet, ListBackupPolicies, ListBackups, AddBackup, DeleteOldestBackup, RestoreFromBackup, ListDatabases, ListSchemas, GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import ObjectNameCaseControl, { identToken } from "../shared/ObjectNameCaseControl";
import dayjs from "dayjs";

const { Text } = Typography;

interface Props {
  // The scope context — what was right-clicked
  scopeType: "DATABASE" | "SCHEMA" | "TABLE";
  db: string;
  schema: string;
  table: string;  // only set when scopeType === "TABLE"
  onClose: () => void;
}

type AlterAction = "rename" | "set-comment" | "unset-comment" | "apply-policy" | "suspend-policy" | "resume-policy";

interface AlterState {
  name: string;           // backup set name being altered
  backupSetDb: string;
  backupSetSchema: string;
  action: AlterAction;
  value: string;          // for rename / set-comment / apply-policy
  caseSensitive: boolean; // only relevant for rename action
}

interface CreateState {
  open: boolean;
  name: string;
  caseSensitive: boolean;
  nameDb: string;    // database component of the backup set's own fully-qualified name
  nameSchema: string; // schema component — empty until user selects for DATABASE scope
  forType: "DATABASE" | "SCHEMA" | "TABLE";
  objectFQN: string;
  backupPolicy: string;
  orReplace: boolean;
  ifNotExists: boolean;
  loading: boolean;
  error: string | null;
}

interface RestoreState {
  backupSetName: string;  // name of the backup set
  backupSetDb: string;
  backupSetSchema: string;
  backupID: string;       // UUID identifier of the specific backup
  objectType: "DATABASE" | "SCHEMA" | "TABLE";
  objectName: string;     // original FQN (shown for reference, not used as target)
  // For DATABASE / SCHEMA: the user types a plain new name here
  targetName: string;
  // For TABLE: the user picks db + schema from dropdowns and types just the table name
  targetDb: string;
  targetSchema: string;
  targetTableName: string;
  loading: boolean;
  error: string | null;
}

function objectFQNFor(props: Props): string {
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
  if (props.scopeType === "DATABASE") return q(props.db);
  if (props.scopeType === "SCHEMA") return `${q(props.db)}.${q(props.schema)}`;
  return `${q(props.db)}.${q(props.schema)}.${q(props.table)}`;
}

function defaultForType(scopeType: Props["scopeType"]): "DATABASE" | "SCHEMA" | "TABLE" {
  if (scopeType === "DATABASE") return "DATABASE";
  if (scopeType === "SCHEMA") return "SCHEMA";
  return "TABLE";
}

function showScope(props: Props): string {
  if (props.scopeType === "DATABASE") return `DATABASE ${props.db}`;
  if (props.scopeType === "SCHEMA") return `SCHEMA ${props.db}.${props.schema}`;
  return `TABLE ${props.db}.${props.schema}.${props.table}`;
}

function listScopeArgs(props: Props): ["DATABASE" | "SCHEMA" | "TABLE", string, string, string] {
  if (props.scopeType === "DATABASE") return ["DATABASE", props.db, "", ""];
  if (props.scopeType === "SCHEMA")   return ["SCHEMA",   props.db, props.schema, ""];
  return ["TABLE", props.db, props.schema, props.table];
}

export default function BackupSetsModal(props: Props) {
  const { scopeType, db, schema, table, onClose } = props;

  const [rows,    setRows]    = useState<main.BackupSetRow[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error,   setError]   = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);

  // New state for the search filter
  const [nameFilter, setNameFilter] = useState("");

  const [alterState, setAlterState] = useState<AlterState | null>(null);
  const [alterLoading, setAlterLoading] = useState(false);

  const [policyList,        setPolicyList]        = useState<string[]>([]);
  const [policyListLoading, setPolicyListLoading] = useState(false);

  // Per-backup-set cache: null = not loaded, "loading" = in flight, array = loaded
  const [backupCache, setBackupCache] = useState<Record<string, main.BackupRow[] | "loading">>({});
  const [backupErrors, setBackupErrors] = useState<Record<string, string>>({});
  // Tracks which backup sets have an in-flight ADD BACKUP call
  const [addingBackup, setAddingBackup] = useState<Record<string, boolean>>({});
  const [restoreState, setRestoreState] = useState<RestoreState | null>(null);

  // For the create backup set name db/schema dropdowns
  const [createDbList,         setCreateDbList]         = useState<string[]>([]);
  const [createDbLoading,      setCreateDbLoading]      = useState(false);
  const [createSchemaList,     setCreateSchemaList]     = useState<string[]>([]);
  const [createSchemaLoading,  setCreateSchemaLoading]  = useState(false);

  const loadCreateDatabases = async () => {
    if (createDbList.length > 0 || createDbLoading) return;
    setCreateDbLoading(true);
    try { setCreateDbList((await ListDatabases()) ?? []); }
    catch { /* ignore */ }
    finally { setCreateDbLoading(false); }
  };

  const loadCreateSchemas = async (dbName: string) => {
    if (!dbName) { setCreateSchemaList([]); return; }
    setCreateSchemaLoading(true);
    try { setCreateSchemaList(((await ListSchemas(dbName)) ?? []).filter((s) => s.toUpperCase() !== "INFORMATION_SCHEMA")); }
    catch { setCreateSchemaList([]); }
    finally { setCreateSchemaLoading(false); }
  };

  // For the TABLE restore target db/schema dropdowns
  const [restoreDbList,       setRestoreDbList]       = useState<string[]>([]);
  const [restoreDbLoading,    setRestoreDbLoading]    = useState(false);
  const [restoreSchemaList,   setRestoreSchemaList]   = useState<string[]>([]);
  const [restoreSchemaLoading,setRestoreSchemaLoading]= useState(false);

  const loadRestoreDatabases = async () => {
    if (restoreDbList.length > 0 || restoreDbLoading) return;
    setRestoreDbLoading(true);
    try {
      const data = await ListDatabases();
      setRestoreDbList(data ?? []);
    } catch { /* ignore */ }
    finally { setRestoreDbLoading(false); }
  };

  const loadRestoreSchemas = async (dbName: string) => {
    if (!dbName) { setRestoreSchemaList([]); return; }
    setRestoreSchemaLoading(true);
    try {
      const data = await ListSchemas(dbName);
      setRestoreSchemaList(data ?? []);
    } catch { setRestoreSchemaList([]); }
    finally { setRestoreSchemaLoading(false); }
  };

  const loadBackups = async (record: main.BackupSetRow) => {
    const key = record.name;
    setBackupCache((c) => ({ ...c, [key]: "loading" }));
    setBackupErrors((e) => { const n = { ...e }; delete n[key]; return n; });
    try {
      const data = await ListBackups(record.name, record.backupSetDb, record.backupSetSchema);
      setBackupCache((c) => ({ ...c, [key]: data ?? [] }));
    } catch (e) {
      setBackupCache((c) => { const n = { ...c }; delete n[key]; return n; });
      setBackupErrors((prev) => ({ ...prev, [key]: String(e) }));
    }
  };

  const handleAddBackup = async (record: main.BackupSetRow) => {
    setAddingBackup(a => ({ ...a, [record.name]: true }));
    try {
      await AddBackup(record.name, record.backupSetDb, record.backupSetSchema);
      message.success(`Backup added to "${record.name}".`);
      await loadBackups(record);
    } catch (e) {
      message.error(String(e));
    } finally {
      setAddingBackup(a => ({ ...a, [record.name]: false }));
    }
  };

  const handleDeleteOldestBackup = async (record: main.BackupSetRow) => {
    try {
      await DeleteOldestBackup(record.name, record.backupSetDb, record.backupSetSchema);
      message.success(`Oldest eligible backup deleted from "${record.name}".`);
      loadBackups(record);
    } catch (e) {
      message.error(String(e));
    }
  };

  const handleRestore = async () => {
    if (!restoreState) return;
    const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
    let targetName: string;
    if (restoreState.objectType === "TABLE") {
      if (!restoreState.targetDb || !restoreState.targetSchema || !restoreState.targetTableName.trim()) {
        setRestoreState((s) => s ? { ...s, error: "Database, schema, and table name are all required." } : s);
        return;
      }
      targetName = `${q(restoreState.targetDb)}.${q(restoreState.targetSchema)}.${q(restoreState.targetTableName.trim())}`;
    } else {
      if (!restoreState.targetName.trim()) {
        setRestoreState((s) => s ? { ...s, error: "Target name is required." } : s);
        return;
      }
      targetName = restoreState.targetName.trim();
    }
    setRestoreState((s) => s ? { ...s, loading: true, error: null } : s);
    try {
      await RestoreFromBackup(
        restoreState.objectType,
        targetName,
        restoreState.backupSetName,
        restoreState.backupSetDb,
        restoreState.backupSetSchema,
        restoreState.backupID,
        db,
      );
      message.success(`Restore completed successfully.`);
      setRestoreState(null);
    } catch (e) {
      setRestoreState((s) => s ? { ...s, loading: false, error: String(e) } : s);
    }
  };

  const loadPolicies = async () => {
    if (policyList.length > 0 || policyListLoading) return;
    setPolicyListLoading(true);
    try {
      const data = await ListBackupPolicies();
      setPolicyList((data ?? []).map((p) => p.name));
    } catch {
      // silently ignore — user can still type freely
    } finally {
      setPolicyListLoading(false);
    }
  };

  const [createState, setCreateState] = useState<CreateState>({
    open: false,
    name: "",
    caseSensitive: false,
    nameDb: db,
    nameSchema: scopeType === "DATABASE" ? "" : schema,
    forType: defaultForType(scopeType),
    objectFQN: objectFQNFor(props),
    backupPolicy: "",
    orReplace: false,
    ifNotExists: false,
    loading: false,
    error: null,
  });

  const loadRows = async (filterTerm: string = nameFilter) => {
    setLoading(true);
    setError(null);
    setRows(null);
    setBackupCache({});
    setBackupErrors({});
    try {
      const [sType, sDb, sSchema, sTable] = listScopeArgs(props);
      const data = await ListBackupSets(sType, sDb, sSchema, sTable, filterTerm);
      const sets = data ?? [];
      setRows(sets);
      for (const record of sets) {
        loadBackups(record);
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadRows(nameFilter);
    GetQuotedIdentifiersIgnoreCase().then((v) => setQuotedIdentifiersIgnoreCase(v ?? false)).catch(() => {});
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleDrop = async (row: main.BackupSetRow) => {
    try {
      await DropBackupSet(row.name, row.backupSetDb, row.backupSetSchema);
      message.success(`Backup set "${row.name}" dropped.`);
      loadRows(nameFilter);
    } catch (e) {
      message.error(String(e));
    }
  };

  const handleAlterSubmit = async () => {
    if (!alterState) return;
    setAlterLoading(true);
    try {
      let alteration = "";
      const esc = (s: string) => s.replace(/'/g, "''");
      switch (alterState.action) {
        case "rename":
          alteration = `RENAME TO ${identToken(alterState.value, alterState.caseSensitive)}`;
          break;
        case "set-comment":
          alteration = `SET COMMENT = '${esc(alterState.value)}'`;
          break;
        case "unset-comment":
          alteration = "UNSET COMMENT";
          break;
        case "apply-policy":
          alteration = `APPLY BACKUP POLICY ${alterState.value}`;
          break;
        case "suspend-policy":
          alteration = "SUSPEND BACKUP POLICY";
          break;
        case "resume-policy":
          alteration = "RESUME BACKUP POLICY";
          break;
      }
      await AlterBackupSet(alterState.name, alterState.backupSetDb, alterState.backupSetSchema, alteration);
      message.success("Backup set updated.");
      setAlterState(null);
      loadRows(nameFilter);
    } catch (e) {
      message.error(String(e));
    } finally {
      setAlterLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!createState.nameDb || !createState.nameSchema) {
      setCreateState((s) => ({ ...s, error: "Database and schema for the backup set name are required." }));
      return;
    }
    setCreateState((s) => ({ ...s, loading: true, error: null }));
    try {
      await CreateBackupSet(
        createState.name,
        createState.nameDb,
        createState.nameSchema,
        createState.forType,
        createState.objectFQN,
        db,
        createState.orReplace,
        createState.ifNotExists,
        createState.caseSensitive,
      );
      if (createState.backupPolicy.trim()) {
        await AlterBackupSet(createState.name, createState.nameDb, createState.nameSchema, `APPLY BACKUP POLICY ${createState.backupPolicy.trim()}`);
      }
      message.success(`Backup set "${createState.name}" created.`);
      setCreateState((s) => ({ ...s, open: false, name: "", backupPolicy: "", loading: false }));
      loadRows(nameFilter);
    } catch (e) {
      setCreateState((s) => ({ ...s, loading: false, error: String(e) }));
    }
  };

  const fmtBytes = (n: number) => {
    if (!n) return "—";
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
    return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
  };

  const statusColor = (status: string) => {
    const s = (status || "").toUpperCase();
    if (s === "ACTIVE") return "green";
    if (s === "SUSPENDED") return "orange";
    return "default";
  };

  const columns: ColumnsType<main.BackupSetRow> = [
    {
      key: "name",
      title: "Name",
      dataIndex: "name",
      render: (v: string, row: main.BackupSetRow) => {
        return (
          <div style={{ display: "flex", alignItems: "center", gap: 4, overflow: "hidden" }}>
            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontWeight: 500 }}>{v}</span>
            <Button
              size="small"
              type="text"
              icon={<PlusCircleOutlined style={{ fontSize: 11 }} />}
              title="Add backup"
              loading={addingBackup[row.name]}
              onClick={(e) => { e.stopPropagation(); handleAddBackup(row); }}
              style={{ flexShrink: 0, height: 18, padding: "0 2px", minWidth: 0, color: "var(--colorTextTertiary)" }}
            />
          </div>
        );
      },
    },
    {
      key: "location",
      title: "Location",
      render: (_: unknown, row: main.BackupSetRow) => {
        const loc = [row.backupSetDb, row.backupSetSchema].filter(Boolean).join(".");
        return loc ? <Text type="secondary" style={{ fontSize: 12 }}>{loc}</Text> : <Text type="secondary">—</Text>;
      }
    },
    {
      key: "objectType",
      title: "For",
      dataIndex: "objectType",
      width: 100,
    },
    {
      key: "objectName",
      title: "Object",
      dataIndex: "objectName",
      ellipsis: true,
    },
    {
      key: "status",
      title: "Status",
      dataIndex: "status",
      width: 100,
      render: (v: string) => v ? <Tag color={statusColor(v)}>{v}</Tag> : null,
    },
    {
      key: "createdOn",
      title: "Created",
      dataIndex: "createdOn",
      width: 120,
      render: (v: string) => v ? dayjs(v).format("DD MMM YYYY") : "—",
    },
    {
      key: "comment",
      title: "Comment",
      dataIndex: "comment",
      ellipsis: true,
      render: (v: string) => v || <Text type="secondary">—</Text>,
    },
    {
      key: "actions",
      title: "",
      width: 120,
      fixed: "right", // Keeps actions visible while scrolling
      render: (_: unknown, row: main.BackupSetRow) => (
        <Space size={4}>
          <Button
            size="small"
            type="text"
            icon={<EditOutlined />}
            title="Alter…"
            onClick={() => setAlterState({ name: row.name, backupSetDb: row.backupSetDb, backupSetSchema: row.backupSetSchema, action: "rename", value: row.name, caseSensitive: false })}
          />
          {(() => {
            const cached = backupCache[row.name];
            const noBackups = Array.isArray(cached) && cached.length === 0;
            return (
              <Popconfirm
                title="Delete the oldest eligible backup in this set?"
                description="Only the oldest backup without a legal hold can be deleted."
                onConfirm={() => handleDeleteOldestBackup(row)}
                okText="Delete"
                okButtonProps={{ danger: true }}
                disabled={noBackups}
              >
                <Button
                  size="small"
                  type="text"
                  danger={!noBackups}
                  disabled={noBackups}
                  icon={<MinusCircleOutlined />}
                  title={noBackups ? "No backups in this set" : "Delete oldest backup"}
                />
              </Popconfirm>
            );
          })()}
          <Popconfirm
            title={`Drop backup set "${row.name}"?`}
            onConfirm={() => handleDrop(row)}
            okText="Drop"
            okButtonProps={{ danger: true }}
          >
            <Button size="small" type="text" danger icon={<DeleteOutlined />} title="Drop backup set" />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const titleText =
    scopeType === "DATABASE"
      ? `Backup Sets — DATABASE ${db}`
      : scopeType === "SCHEMA"
      ? `Backup Sets — SCHEMA ${db}.${schema}`
      : `Backup Sets — TABLE ${db}.${schema}.${table}`;

  const needsValue =
    alterState &&
    (alterState.action === "rename" || alterState.action === "set-comment" || alterState.action === "apply-policy");
  const valueLabel =
    alterState?.action === "rename"
      ? "New name"
      : alterState?.action === "set-comment"
      ? "Comment"
      : "Policy name";

  return (
    <>
      <Modal
        open
        title={titleText}
        onCancel={onClose}
        width={1100} // Increased width so columns have room to breathe
        footer={null}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>
            {rows !== null ? `${rows.length} backup set${rows.length !== 1 ? "s" : ""} for ${showScope(props)}` : ""}
          </Text>
          <Space size={8}>
            <Input.Search
              placeholder="Filter by name..."
              allowClear
              onSearch={(val) => {
                setNameFilter(val);
                loadRows(val);
              }}
              style={{ width: 200 }}
              size="small"
            />
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={() => loadRows(nameFilter)}
              loading={loading}
            >
              Refresh
            </Button>
            <Button
              size="small"
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                loadPolicies();
                loadCreateDatabases();
                const defaultNameSchema = scopeType === "DATABASE" ? "" : schema;
                if (db) loadCreateSchemas(db);
                setCreateState({
                  open: true,
                  name: "",
                  caseSensitive: false,
                  nameDb: db,
                  nameSchema: defaultNameSchema,
                  forType: defaultForType(scopeType),
                  objectFQN: objectFQNFor(props),
                  backupPolicy: "",
                  orReplace: false,
                  ifNotExists: false,
                  loading: false,
                  error: null,
                });
              }}
            >
              Create
            </Button>
          </Space>
        </div>

        {loading && <div style={{ textAlign: "center", padding: 32 }}><Spin /></div>}
        {error && <Alert type="error" message={error} style={{ marginBottom: 8 }} />}
        {rows !== null && !loading && (
          <Table<main.BackupSetRow>
            dataSource={rows.map(row => ({
              ...row,
              _rev: Array.isArray(backupCache[row.name])
                ? (backupCache[row.name] as main.BackupRow[]).length
                : backupCache[row.name] === "loading" ? -1 : -2,
            } as main.BackupSetRow))}
            columns={columns}
            rowKey="name"
            size="small"
            scroll={{ x: 'max-content' }} // Allows columns to expand horizontally beyond modal width
            pagination={{ pageSize: 20, showSizeChanger: false, hideOnSinglePage: true }}
            locale={{ emptyText: "No backup sets found." }}
            expandable={{
              onExpand: (expanded, record) => {
                if (expanded && !(record.name in backupCache)) {
                  loadBackups(record);
                }
              },
              expandedRowRender: (record) => {
                const entry = backupCache[record.name];
                const err = backupErrors[record.name];
                const isLoading = entry === "loading" || entry === undefined;
                const backupCols: ColumnsType<main.BackupRow> = [
                  {
                    key: "name",
                    title: "Backup",
                    dataIndex: "name",
                    render: (v: string) => <Text code style={{ fontSize: 11 }}>{v}</Text>,
                  },
                  {
                    key: "status",
                    title: "Status",
                    dataIndex: "status",
                    width: 100,
                    render: (v: string) => v ? <Tag color={statusColor(v)} style={{ fontSize: 11 }}>{v}</Tag> : null,
                  },
                  {
                    key: "createdOn",
                    title: "Created",
                    dataIndex: "createdOn",
                    width: 130,
                    render: (v: string) => v ? dayjs(v).format("DD MMM YYYY") : "—",
                  },
                  {
                    key: "sizeBytes",
                    title: "Size",
                    dataIndex: "sizeBytes",
                    width: 100,
                    render: (v: number) => fmtBytes(v),
                  },
                  {
                    key: "comment",
                    title: "Comment",
                    dataIndex: "comment",
                    ellipsis: true,
                    render: (v: string) => v || <Text type="secondary">—</Text>,
                  },
                  {
                    key: "actions",
                    title: "",
                    width: 60,
                    fixed: "right", // Keeps actions visible while scrolling
                    render: (_: unknown, backup: main.BackupRow) => (
                      <Button
                        size="small"
                        type="text"
                        icon={<RollbackOutlined />}
                        title="Restore from this backup…"
                        onClick={() => {
                          const inferredType = (
                            record.objectType?.toUpperCase() as "DATABASE" | "SCHEMA" | "TABLE" | ""
                          ) || scopeType;
                          setRestoreState({
                            backupSetName: record.name,
                            backupSetDb: record.backupSetDb,
                            backupSetSchema: record.backupSetSchema,
                            backupID: backup.id || backup.name,
                            objectType: inferredType,
                            objectName: record.objectName,
                            targetName: "",
                            targetDb: db,
                            targetSchema: schema,
                            targetTableName: "",
                            loading: false,
                            error: null,
                          });
                          if (inferredType === "TABLE") {
                            loadRestoreDatabases();
                            loadRestoreSchemas(db);
                          }
                        }}
                      />
                    ),
                  },
                ];
                return (
                  <div style={{ marginLeft: 24, marginBottom: 4 }}>
                    <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 4 }}>
                      <Button
                        size="small"
                        type="primary"
                        icon={<PlusOutlined />}
                        loading={addingBackup[record.name]}
                        onClick={() => handleAddBackup(record)}
                      >
                        Add Backup
                      </Button>
                    </div>
                    {isLoading && <div style={{ padding: "8px 0" }}><Spin size="small" /></div>}
                    {err && <Alert type="error" message={err} style={{ margin: "4px 0" }} />}
                    {!isLoading && !err && (
                      <Table<main.BackupRow>
                        dataSource={entry as main.BackupRow[]}
                        columns={backupCols}
                        rowKey="name"
                        size="small"
                        scroll={{ x: 'max-content' }} // Apply scroll to inner table too
                        pagination={false}
                        locale={{ emptyText: "No backups in this set." }}
                      />
                    )}
                  </div>
                );
              },
            }}
          />
        )}
      </Modal>

      {/* Alter modal */}
      {alterState && (
        <Modal
          open
          title={`Alter Backup Set: ${alterState.name}`}
          onCancel={() => setAlterState(null)}
          onOk={handleAlterSubmit}
          confirmLoading={alterLoading}
          okText="Apply"
          width={460}
        >
          <Form layout="vertical" style={{ marginTop: 8 }}>
            <Form.Item label="Action">
              <Select
                value={alterState.action}
                onChange={(v) => {
                  if (v === "apply-policy") loadPolicies();
                  setAlterState((s) => s ? { ...s, action: v, value: v === "rename" ? s.name : "" } : s);
                }}
                options={[
                  { value: "rename",         label: "Rename To" },
                  { value: "set-comment",    label: "Set Comment" },
                  { value: "unset-comment",  label: "Unset Comment" },
                  { value: "apply-policy",   label: "Apply Backup Policy" },
                  { value: "suspend-policy", label: "Suspend Backup Policy" },
                  { value: "resume-policy",  label: "Resume Backup Policy" },
                ]}
              />
            </Form.Item>
            {needsValue && alterState.action === "apply-policy" && (
              <Form.Item label="Backup policy">
                <Select
                  value={alterState.value || undefined}
                  onChange={(v) => setAlterState((s) => s ? { ...s, value: v } : s)}
                  options={policyList.map((p) => ({ value: p, label: p }))}
                  loading={policyListLoading}
                  onDropdownVisibleChange={(open) => { if (open) loadPolicies(); }}
                  placeholder="Select a policy…"
                  showSearch
                  style={{ width: "100%" }}
                />
              </Form.Item>
            )}
            {needsValue && alterState.action !== "apply-policy" && (
              <Form.Item label={valueLabel}>
                <Input
                  value={alterState.value}
                  onChange={(e) => setAlterState((s) => s ? { ...s, value: e.target.value } : s)}
                  placeholder={alterState.action === "rename" ? "new_backup_set_name" : "Your comment…"}
                />
              </Form.Item>
            )}
            {alterState.action === "rename" && (
              <Form.Item style={{ marginBottom: 0 }}>
                <ObjectNameCaseControl
                  name={alterState.value}
                  caseSensitive={alterState.caseSensitive}
                  onCaseSensitiveChange={(v) => setAlterState((s) => s ? { ...s, caseSensitive: v } : s)}
                  quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
                />
              </Form.Item>
            )}
          </Form>
        </Modal>
      )}

      {/* Restore modal */}
      {restoreState && (
        <Modal
          open
          title={
            <Space>
              <RollbackOutlined />
              Restore {restoreState.objectType}
            </Space>
          }
          onCancel={() => setRestoreState(null)}
          onOk={handleRestore}
          confirmLoading={restoreState.loading}
          okText="Restore"
          okButtonProps={{ danger: true }}
          width={520}
        >
          <Form layout="vertical" style={{ marginTop: 8 }}>
            <Form.Item label="Backup set">
              <Text code>{restoreState.backupSetName}</Text>
            </Form.Item>
            <Form.Item
              label="Backup identifier (UUID)"
              help={<span style={{ fontSize: 11 }}>UUID passed to the IDENTIFIER clause. Edit if the value shown is incorrect.</span>}
            >
              <Input
                value={restoreState.backupID}
                onChange={(e) => setRestoreState((s) => s ? { ...s, backupID: e.target.value } : s)}
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              />
            </Form.Item>
            <Form.Item label="Object type">
              <Text code>{restoreState.objectType}</Text>
            </Form.Item>
            <Form.Item label="Original object">
              <Text type="secondary" style={{ fontSize: 12 }}>{restoreState.objectName || "—"}</Text>
            </Form.Item>
            {restoreState.objectType === "TABLE" ? (
              <>
                <Form.Item
                  label="Target database"
                  required
                  help={<span style={{ fontSize: 11 }}>Database to restore the table into. Defaults to the source database.</span>}
                >
                  <Select
                    value={restoreState.targetDb || undefined}
                    onChange={(v) => {
                      setRestoreState((s) => s ? { ...s, targetDb: v, targetSchema: "", } : s);
                      setRestoreSchemaList([]);
                      loadRestoreSchemas(v);
                    }}
                    options={restoreDbList.map((d) => ({ value: d, label: d }))}
                    loading={restoreDbLoading}
                    showSearch
                    placeholder="Select database…"
                    style={{ width: "100%" }}
                  />
                </Form.Item>
                <Form.Item
                  label="Target schema"
                  required
                  help={<span style={{ fontSize: 11 }}>Schema to restore the table into. Defaults to the source schema.</span>}
                >
                  <Select
                    value={restoreState.targetSchema || undefined}
                    onChange={(v) => setRestoreState((s) => s ? { ...s, targetSchema: v } : s)}
                    options={restoreSchemaList.map((s) => ({ value: s, label: s }))}
                    loading={restoreSchemaLoading}
                    disabled={!restoreState.targetDb}
                    showSearch
                    placeholder="Select schema…"
                    style={{ width: "100%" }}
                  />
                </Form.Item>
                <Form.Item
                  label="New table name"
                  required
                  help={<span style={{ fontSize: 11 }}>Snowflake requires a new name — you cannot restore over an existing object.</span>}
                >
                  <Input
                    value={restoreState.targetTableName}
                    onChange={(e) => setRestoreState((s) => s ? { ...s, targetTableName: e.target.value } : s)}
                    placeholder="MY_TABLE_RESTORED"
                    status={restoreState.error && !restoreState.targetTableName.trim() ? "error" : undefined}
                  />
                </Form.Item>
              </>
            ) : (
              <Form.Item
                label="New name (target)"
                required
                help={<span style={{ fontSize: 11 }}>Snowflake requires a new name — you cannot restore over an existing object.</span>}
              >
                <Input
                  value={restoreState.targetName}
                  onChange={(e) => setRestoreState((s) => s ? { ...s, targetName: e.target.value } : s)}
                  placeholder={`${restoreState.objectName || "MY_OBJECT"}_RESTORED`}
                  status={restoreState.error && !restoreState.targetName.trim() ? "error" : undefined}
                />
              </Form.Item>
            )}

            {restoreState.error && (
              <Alert type="error" message={restoreState.error} style={{ marginBottom: 8 }} />
            )}
          </Form>
        </Modal>
      )}

      {/* Create modal */}
      {createState.open && (
        <Modal
          open
          title="Create Backup Set"
          onCancel={() => setCreateState((s) => ({ ...s, open: false }))}
          onOk={handleCreate}
          confirmLoading={createState.loading}
          okText="Create"
          width={520}
        >
          <Form layout="vertical" style={{ marginTop: 8 }}>
            <Form.Item label="Backup set database" required>
              <Select
                value={createState.nameDb || undefined}
                onChange={(v) => {
                  setCreateState((s) => ({ ...s, nameDb: v, nameSchema: "" }));
                  setCreateSchemaList([]);
                  loadCreateSchemas(v);
                }}
                options={createDbList.map((d) => ({ value: d, label: d }))}
                loading={createDbLoading}
                showSearch
                placeholder="Select database…"
                style={{ width: "100%" }}
              />
            </Form.Item>
            <Form.Item label="Backup set schema" required>
              <Select
                value={createState.nameSchema || undefined}
                onChange={(v) => setCreateState((s) => ({ ...s, nameSchema: v }))}
                options={createSchemaList.map((s) => ({ value: s, label: s }))}
                loading={createSchemaLoading}
                disabled={!createState.nameDb}
                showSearch
                placeholder="Select schema…"
                style={{ width: "100%" }}
              />
            </Form.Item>
            <Form.Item label="Backup set name" required style={{ marginBottom: 4 }}>
              <Input
                value={createState.name}
                onChange={(e) => setCreateState((s) => ({ ...s, name: e.target.value }))}
                placeholder="my_backup_set"
              />
            </Form.Item>
            <Form.Item style={{ marginBottom: 12 }}>
              <ObjectNameCaseControl
                name={createState.name}
                caseSensitive={createState.caseSensitive}
                onCaseSensitiveChange={(v) => setCreateState((s) => ({ ...s, caseSensitive: v }))}
                quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
              />
            </Form.Item>
            <Form.Item label="For type" required>
              <Select
                value={createState.forType}
                onChange={(v) => setCreateState((s) => ({ ...s, forType: v }))}
                options={[
                  { value: "DATABASE", label: "DATABASE" },
                  { value: "SCHEMA",   label: "SCHEMA" },
                  { value: "TABLE",    label: "TABLE" },
                ]}
              />
            </Form.Item>
            <Form.Item label="Object (fully qualified name)" required>
              <Input
                value={createState.objectFQN}
                onChange={(e) => setCreateState((s) => ({ ...s, objectFQN: e.target.value }))}
                placeholder={`"MY_DB"."MY_SCHEMA"."MY_TABLE"`}
              />
            </Form.Item>
            <Form.Item
              label="Backup policy"
              help={<span style={{ fontSize: 11 }}>Optional — applies the selected policy after creation</span>}
            >
              <Select
                value={createState.backupPolicy || undefined}
                onChange={(v) => setCreateState((s) => ({ ...s, backupPolicy: v ?? "" }))}
                options={policyList.map((p) => ({ value: p, label: p }))}
                loading={policyListLoading}
                onDropdownVisibleChange={(open) => { if (open) loadPolicies(); }}
                placeholder="Select a policy…"
                showSearch
                allowClear
                style={{ width: "100%" }}
              />
            </Form.Item>
            <Form.Item>
              <Space direction="vertical" size={4}>
                <Checkbox
                  checked={createState.orReplace}
                  onChange={(e) => setCreateState((s) => ({ ...s, orReplace: e.target.checked }))}
                >
                  OR REPLACE
                </Checkbox>
                <Checkbox
                  checked={createState.ifNotExists}
                  disabled={createState.orReplace}
                  onChange={(e) => setCreateState((s) => ({ ...s, ifNotExists: e.target.checked }))}
                >
                  IF NOT EXISTS
                </Checkbox>
              </Space>
            </Form.Item>
            {createState.error && (
              <Alert type="error" message={createState.error} style={{ marginBottom: 8 }} />
            )}
          </Form>
        </Modal>
      )}
    </>
  );
}