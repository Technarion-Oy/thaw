// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import { App as AntApp, Modal, Spin, Button, Input, Space, Tag, Select } from "antd";
import { EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined } from "@ant-design/icons";
import { ConfirmSwitch } from "../common/ConfirmSwitch";
import DataTypeSelect from "../shared/DataTypeSelect";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import SequenceDefaultPicker from "../shared/SequenceDefaultPicker";
import { parseAllowedValues } from "../tag/allowedValues";
import type { snowflake } from "../../../wailsjs/go/models";
import {
  ExecDDL,
  GetColumnDetails,
  GetColumnTagReferences,
  GetQuotedIdentifiersIgnoreCase,
  ListAccountMaskingPolicies,
  ListAccountSequences,
  ListAccountTags,
  SetObjectTag,
  UnsetObjectTag,
  BuildRenameColumnSql,
  BuildChangeColumnTypeSql,
  BuildSetColumnNotNullSql,
  BuildDropColumnNotNullSql,
  BuildSetColumnCommentSql,
  BuildSetColumnDefaultSql,
  BuildDropColumnDefaultSql,
  BuildSetColumnMaskingPolicySql,
  BuildUnsetColumnMaskingPolicySql,
} from "../../../wailsjs/go/app/App";

// A schema-qualified object (masking policy or tag) parsed from a SHOW … result.
// allowedValues is populated from SHOW TAGS' allowed_values column (empty for
// masking policies, which have no such column).
interface QualifiedObject { db: string; schema: string; name: string; fqn: string; allowedValues: string[]; }

// SHOW MASKING POLICIES / SHOW TAGS share the name/database_name/schema_name
// columns; pull the qualified object list out by column name.
function parseObjectList(res: snowflake.QueryResult): QualifiedObject[] {
  const cols = res.columns ?? [];
  const iName = cols.indexOf("name");
  const iDb = cols.indexOf("database_name");
  const iSc = cols.indexOf("schema_name");
  const iAllowed = cols.indexOf("allowed_values");
  if (iName < 0) return [];
  return (res.rows ?? []).map((r) => {
    const name = String(r[iName]);
    const db = iDb >= 0 && r[iDb] != null ? String(r[iDb]) : "";
    const schema = iSc >= 0 && r[iSc] != null ? String(r[iSc]) : "";
    const allowedValues = iAllowed >= 0 && r[iAllowed] != null ? parseAllowedValues(String(r[iAllowed])) : [];
    return { db, schema, name, fqn: [db, schema, name].filter(Boolean).join("."), allowedValues };
  });
}

interface ColMeta {
  dataType: string;
  nullable: boolean;
  isPrimaryKey: boolean;
  comment: string;
}

interface Props {
  db: string;
  schema: string;
  table: string;
  column: string;
  parentKind: string; // "TABLE"
  initial: ColMeta;
  onClose: () => void;
  onChanged: () => void; // refresh the table's columns in the sidebar
}

const ROW_LABEL: React.CSSProperties = {
  padding: "8px 12px 8px 0",
  color: "var(--text-muted)",
  fontFamily: "monospace",
  whiteSpace: "nowrap",
  verticalAlign: "top",
  width: 150,
  minWidth: 130,
};

interface ColTag { db: string; schema: string; name: string; value: string; }

export default function ColumnPropertiesModal({ db, schema, table, column, parentKind, initial, onClose, onChanged }: Props) {
  const { modal, message } = AntApp.useApp();

  const [cur, setCur] = useState<ColMeta>(initial);
  const [colDefault, setColDefault] = useState("");
  const [maskingPolicy, setMaskingPolicy] = useState("");
  const [loading, setLoading] = useState(true);
  const [detailsError, setDetailsError] = useState(false);
  const [saving, setSaving] = useState(false);
  const [qiic, setQiic] = useState(false);

  // Which section is in edit mode, plus its draft value.
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);

  // Pick lists for the masking-policy, sequence, and tag dropdowns.
  const [policies, setPolicies] = useState<QualifiedObject[]>([]);
  const [sequences, setSequences] = useState<QualifiedObject[]>([]);
  const [tagCatalog, setTagCatalog] = useState<QualifiedObject[]>([]);

  // Tags.
  const [tags, setTags] = useState<ColTag[] | null>(null);
  const [tagName, setTagName] = useState("");
  const [tagValue, setTagValue] = useState("");
  // Allowed values for the selected tag (SYSTEM$GET_TAG_ALLOWED_VALUES); null
  // means the tag accepts any value, so the value field stays free-text.
  const [tagAllowed, setTagAllowed] = useState<string[] | null>(null);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase().then((v) => setQiic(v ?? false)).catch(() => {});
    GetColumnDetails(db, schema, table, column)
      .then((d) => { setColDefault(d.default || ""); setMaskingPolicy(d.maskingPolicy || ""); })
      .catch((e) => { setDetailsError(true); message.error(String(e)); })
      .finally(() => setLoading(false));
    ListAccountMaskingPolicies().then((r) => setPolicies(parseObjectList(r))).catch(() => {});
    ListAccountSequences().then((r) => setSequences(parseObjectList(r))).catch(() => {});
    ListAccountTags().then((r) => setTagCatalog(parseObjectList(r))).catch(() => {});
    loadTags();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [db, schema, table, column]);

  const loadTags = () => {
    GetColumnTagReferences(parentKind, db, schema, table)
      .then((res) => {
        const rows = res.rows ?? [];
        setTags(rows
          .filter((r) => String(r[0]).toLowerCase() === column.toLowerCase())
          .map((r) => ({ db: String(r[1]), schema: String(r[2]), name: String(r[3]), value: r[4] == null ? "" : String(r[4]) })));
      })
      .catch(() => setTags([]));
  };

  // `silent` suppresses the error toast (but still rethrows) for callers that
  // render their own inline error — e.g. ConfirmSwitch, which would otherwise
  // show the failure twice (raw toast + inline friendly message).
  const exec = async (sql: string, okMsg: string, after?: () => void, silent?: boolean) => {
    try {
      await ExecDDL(sql);
      message.success(okMsg);
      onChanged();
      after?.();
    } catch (e) {
      if (!silent) message.error(String(e));
      throw e;
    }
  };

  // Run a statement. Safe edits execute immediately; only edits that can lose or
  // truncate data (`warning` set) get a confirmation dialog with a SQL preview.
  // Returns a promise that settles when the work is fully done (including the
  // user acting on the dialog) so the caller's `saving` guard stays up until then.
  const run = (sql: string, okMsg: string, after?: () => void, warning?: string): Promise<void> => {
    if (!warning) return exec(sql, okMsg, after).catch(() => {});
    return new Promise<void>((resolve) => {
      modal.confirm({
        title: "This change may affect existing data",
        width: 580,
        okText: "Run anyway",
        okButtonProps: { danger: true },
        content: (
          <div>
            <div style={{ marginBottom: 8, color: "var(--text)" }}>{warning}</div>
            <div style={{ padding: "10px 12px", background: "var(--bg)", borderRadius: 6, border: "1px solid var(--border)" }}>
              <pre style={{ margin: 0, color: "var(--text)", fontSize: 11, fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", whiteSpace: "pre-wrap", wordBreak: "break-all" }}>{sql}</pre>
            </div>
          </div>
        ),
        // Returning the promise keeps the dialog (and its loading OK button) open
        // until ExecDDL settles; afterClose resolves once the dialog is gone.
        onOk: () => exec(sql, okMsg, after),
        afterClose: () => resolve(),
      });
    });
  };

  const startEdit = (key: string, value: string) => { setEditing(key); setDraft(value); setCaseSensitive(false); };
  const cancelEdit = () => { setEditing(null); setDraft(""); };

  // ── Section savers ──────────────────────────────────────────────────────────

  const saveName = async () => {
    const trimmed = draft.trim();
    if (!trimmed || trimmed === column) { cancelEdit(); return; }
    const sql = await BuildRenameColumnSql(db, schema, table, column, trimmed, caseSensitive);
    // Clear edit state only on success — a failed DDL keeps the typed name.
    // The column identity changes on rename, so close the modal afterwards.
    return run(sql, `Renamed column to "${trimmed}"`, () => { cancelEdit(); onClose(); });
  };

  const saveDataType = async () => {
    const trimmed = draft.trim();
    if (!trimmed || trimmed === cur.dataType) { cancelEdit(); return; }
    const sql = await BuildChangeColumnTypeSql(db, schema, table, column, trimmed);
    // Don't clear edit state before the confirm dialog — if the user cancels,
    // their typed type must survive. cancelEdit runs only on confirmed success.
    return run(sql, `Changed data type to ${trimmed}`, () => { cancelEdit(); setCur((c) => ({ ...c, dataType: trimmed })); },
      `Changing the data type of "${column}" from ${cur.dataType} to ${trimmed} can truncate or fail on existing rows. Snowflake only permits a narrow set of conversions (e.g. widening a VARCHAR or NUMBER).`);
  };

  const toggleNullable = async (checked: boolean) => {
    // checked = nullable. SET NOT NULL when turning off; DROP NOT NULL when on.
    // Use exec (not run) so a failed DDL rejects — ConfirmSwitch keeps the
    // staged toggle and surfaces the error inline. Pass silent so exec doesn't
    // also raise a toast for the same failure.
    const sql = checked
      ? await BuildDropColumnNotNullSql(db, schema, table, column)
      : await BuildSetColumnNotNullSql(db, schema, table, column);
    await exec(sql, checked ? "Dropped NOT NULL" : "Set NOT NULL", () => setCur((c) => ({ ...c, nullable: checked })), true);
  };

  const saveDefault = async () => {
    const trimmed = draft.trim();
    if (trimmed === colDefault.trim()) { cancelEdit(); return; }
    const sql = trimmed
      ? await BuildSetColumnDefaultSql(db, schema, table, column, trimmed)
      : await BuildDropColumnDefaultSql(db, schema, table, column);
    // Clear edit state only on success so a failed DDL keeps the typed value.
    return run(sql, trimmed ? "Default updated" : "Default dropped", () => { cancelEdit(); setColDefault(trimmed); });
  };

  const saveComment = async () => {
    if (draft === (cur.comment ?? "")) { cancelEdit(); return; }
    const sql = await BuildSetColumnCommentSql(db, schema, table, column, draft);
    // The backend trims and emits UNSET COMMENT for whitespace-only input, so
    // mirror that here — store the trimmed value to keep the display in sync.
    const v = draft.trim();
    // Clear edit state only on success so a failed DDL keeps the typed comment.
    return run(sql, v ? "Comment updated" : "Comment removed", () => { cancelEdit(); setCur((c) => ({ ...c, comment: v })); });
  };

  // Split a qualified name into db/schema/name, preferring a known catalog match
  // (handles quoted/dotted identifiers); falls back to a naive dotted split with
  // the table's db/schema as defaults.
  const splitQualified = (fqn: string, catalog: QualifiedObject[]) => {
    const hit = catalog.find((o) => o.fqn === fqn);
    if (hit) return { db: hit.db, schema: hit.schema, name: hit.name };
    const parts = fqn.split(".");
    return { name: parts.pop()!, schema: parts.pop() ?? schema, db: parts.pop() ?? db };
  };

  const saveMasking = async () => {
    const trimmed = draft.trim();
    if (trimmed === maskingPolicy) { cancelEdit(); return; }
    let sql: string;
    if (!trimmed) {
      sql = await BuildUnsetColumnMaskingPolicySql(db, schema, table, column);
    } else {
      const p = splitQualified(trimmed, policies);
      sql = await BuildSetColumnMaskingPolicySql(db, schema, table, column, p.db, p.schema, p.name);
    }
    // Replacing or removing an existing governance policy is destructive — confirm
    // with a SQL preview. This also guards the case where the policy list failed
    // to load (empty dropdown) so the user can't silently unset a live policy.
    const warning = maskingPolicy
      ? (trimmed
          ? `This replaces the masking policy currently on "${column}" (${maskingPolicy}) with ${trimmed}.`
          : `This removes the masking policy currently on "${column}" (${maskingPolicy}).`)
      : undefined;
    return run(sql, trimmed ? "Masking policy set" : "Masking policy unset", () => { cancelEdit(); setMaskingPolicy(trimmed); }, warning);
  };

  // ── Tags (via the existing tag governance system) ──────────────────────────

  const tagRef = () => ({ domain: "COLUMN", database: db, schema, name: table, column, parentKind } as any);

  // After a set/unset, update local state directly rather than re-querying:
  // TAG_REFERENCES_ALL_COLUMNS has propagation latency and a refetch returns
  // stale rows that still include a just-removed tag (or omit a just-added one).
  const sameTag = (a: { db: string; schema: string; name: string }, b: ColTag) =>
    a.db === b.db && a.schema === b.schema && a.name === b.name;

  // On tag selection, switch the value field between a dropdown (the tag's
  // allowed-values whitelist) and free text. The allowed values come from the
  // SHOW TAGS catalog already loaded into tagCatalog — no extra round-trip.
  const onTagNameChange = (v?: string) => {
    const name = v ?? "";
    setTagName(name);
    setTagValue("");
    const allowed = tagCatalog.find((t) => t.fqn === name)?.allowedValues ?? [];
    setTagAllowed(allowed.length ? allowed : null);
  };

  const addTag = async () => {
    const name = tagName.trim();
    if (!name) return;
    const t = splitQualified(name, tagCatalog);
    const value = tagValue.trim();
    try {
      await SetObjectTag(tagRef(), t.db, t.schema, t.name, value);
      message.success(`Tag "${t.name}" set`);
      setTagName(""); setTagValue("");
      setTags((cur) => {
        const list = cur ?? [];
        const entry = { db: t.db, schema: t.schema, name: t.name, value };
        const i = list.findIndex((x) => sameTag(t, x));
        if (i < 0) return [...list, entry];
        const copy = list.slice(); copy[i] = entry; return copy; // retag updates the value
      });
    } catch (e) { message.error(String(e)); }
  };

  const removeTag = async (t: ColTag) => {
    try {
      await UnsetObjectTag(tagRef(), t.db, t.schema, t.name);
      message.success(`Tag "${t.name}" removed`);
      setTags((cur) => (cur ?? []).filter((x) => !sameTag(t, x)));
    } catch (e) { message.error(String(e)); }
  };

  // ── Render helpers ──────────────────────────────────────────────────────────

  const displayValue = (v: string) => (
    <span style={{ fontFamily: "monospace", color: v ? "var(--text)" : "var(--text-faint)", fontStyle: v ? "normal" : "italic", flex: 1, wordBreak: "break-word" }}>
      {v || "—"}
    </span>
  );

  const editButton = (key: string, value: string, disabled = false) => (
    <Button size="small" type="text" disabled={disabled} icon={<EditOutlined />} onClick={() => startEdit(key, value)} style={{ flexShrink: 0, color: "var(--text-faint)" }} />
  );

  // The confirm button guards against a double-click firing two concurrent saves
  // (each an async IPC + DDL round-trip) by disabling itself until the save settles.
  const confirmButtons = (onSave: () => void) => (
    <>
      <Button size="small" type="primary" loading={saving} disabled={saving} icon={<CheckOutlined />}
        onClick={() => { if (saving) return; setSaving(true); Promise.resolve(onSave()).finally(() => setSaving(false)); }} />
      <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancelEdit} />
    </>
  );

  const row = (label: string, body: React.ReactNode) => (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={ROW_LABEL}>{label}</td>
      <td style={{ padding: "6px 0", verticalAlign: "middle" }}>{body}</td>
    </tr>
  );

  const textRow = (label: string, key: string, value: string, onSave: () => void, textarea = false, editDisabled = false) =>
    row(label, editing === key ? (
      <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
        {textarea ? (
          <Input.TextArea value={draft} onChange={(e) => setDraft(e.target.value)} rows={2} autoFocus style={{ fontFamily: "monospace", fontSize: 12 }} />
        ) : (
          <Input size="small" value={draft} onChange={(e) => setDraft(e.target.value)} onPressEnter={onSave} autoFocus style={{ fontFamily: "monospace", fontSize: 12 }} />
        )}
        {confirmButtons(onSave)}
      </div>
    ) : (
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>{displayValue(value)}{editButton(key, value, editDisabled)}</div>
    ));

  return (
    <Modal open title={`Column "${column}"`} onCancel={onClose} width={620} footer={[<Button key="close" onClick={onClose}>Close</Button>]}>
      {loading ? (
        <div style={{ textAlign: "center", padding: "32px 0" }}><Spin /></div>
      ) : (
        <>
        {detailsError && (
          <div style={{ marginBottom: 12, padding: "8px 12px", borderRadius: 6, background: "var(--bg)", border: "1px solid var(--border)", color: "var(--text-faint)", fontSize: 12 }}>
            Couldn't load the column's default and masking policy. Editing those is disabled to avoid overwriting values that didn't load.
          </div>
        )}
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
          <tbody>
            {/* Name */}
            {textRow("Name", "name", column, saveName)}
            {editing === "name" && (
              <tr><td /><td style={{ paddingBottom: 8 }}>
                <ObjectNameCaseControl name={draft} caseSensitive={caseSensitive} onCaseSensitiveChange={setCaseSensitive} quotedIdentifiersIgnoreCase={qiic} />
              </td></tr>
            )}

            {/* Data type */}
            {row("Data type", editing === "dataType" ? (
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <DataTypeSelect value={draft || cur.dataType} onChange={setDraft} />
                {confirmButtons(saveDataType)}
              </div>
            ) : (
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>{displayValue(cur.dataType)}{editButton("dataType", cur.dataType)}</div>
            ))}

            {/* Nullable */}
            {row("Nullable", cur.isPrimaryKey ? (
              <span style={{ color: "var(--text-faint)", fontStyle: "italic" }}>NOT NULL (primary key)</span>
            ) : (
              <ConfirmSwitch checked={cur.nullable} onConfirm={toggleNullable} />
            ))}

            {/* Default — free-text plus a sequence picker. On an existing column
                Snowflake's ALTER … ALTER COLUMN … SET DEFAULT accepts only a
                sequence reference (<seq>.NEXTVAL); function defaults like
                CURRENT_TIMESTAMP() are valid solely at table-create time, so the
                ƒ picker used by Create Table / the ER Designer is deliberately
                not offered here. The picker inserts a NEXTVAL reference from the
                account's sequences. */}
            {row("Default", editing === "default" ? (
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <Input size="small" value={draft} onChange={(e) => setDraft(e.target.value)} onPressEnter={saveDefault} autoFocus style={{ flex: 1, minWidth: 0, fontFamily: "monospace", fontSize: 12 }} />
                <SequenceDefaultPicker sequences={sequences} onPick={setDraft} />
                {confirmButtons(saveDefault)}
              </div>
            ) : (
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>{displayValue(colDefault)}{editButton("default", colDefault, detailsError)}</div>
            ))}

            {/* Comment */}
            {textRow("Comment", "comment", cur.comment ?? "", saveComment, true)}

            {/* Masking policy */}
            {row("Masking policy", editing === "masking" ? (
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <Select
                  size="small"
                  showSearch
                  allowClear
                  placeholder="Select a masking policy (clear to unset)"
                  value={draft || undefined}
                  onChange={(v) => setDraft(v ?? "")}
                  style={{ flex: 1, minWidth: 0 }}
                  options={policies.map((p) => ({ value: p.fqn, label: p.fqn }))}
                />
                {confirmButtons(saveMasking)}
              </div>
            ) : (
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>{displayValue(maskingPolicy)}{editButton("masking", maskingPolicy, detailsError)}</div>
            ))}

            {/* Tags */}
            {row("Tags", (
              <Space direction="vertical" size={6} style={{ width: "100%" }}>
                {tags === null ? <Spin size="small" /> : tags.length > 0 && (
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                    {tags.map((t) => (
                      <Tag key={`${t.db}.${t.schema}.${t.name}`} closable onClose={(e) => { e.preventDefault(); removeTag(t); }}>
                        {t.name}{t.value ? `: ${t.value}` : ""}
                      </Tag>
                    ))}
                  </div>
                )}
                <Space>
                  <Select
                    size="small"
                    showSearch
                    placeholder="Tag name"
                    value={tagName || undefined}
                    onChange={onTagNameChange}
                    style={{ width: 170 }}
                    options={tagCatalog.map((t) => ({ value: t.fqn, label: t.fqn }))}
                  />
                  {tagAllowed ? (
                    <Select
                      size="small"
                      showSearch
                      placeholder="Tag value"
                      value={tagValue || undefined}
                      onChange={(v) => setTagValue(v ?? "")}
                      style={{ width: 150 }}
                      options={tagAllowed.map((v) => ({ value: v, label: v }))}
                    />
                  ) : (
                    <Input size="small" value={tagValue} onChange={(e) => setTagValue(e.target.value)} onPressEnter={addTag} placeholder="Tag value" style={{ width: 150 }} />
                  )}
                  <Button size="small" icon={<PlusOutlined />} onClick={addTag} disabled={!tagName.trim()}>Add</Button>
                </Space>
              </Space>
            ))}
          </tbody>
        </table>
        </>
      )}
    </Modal>
  );
}
