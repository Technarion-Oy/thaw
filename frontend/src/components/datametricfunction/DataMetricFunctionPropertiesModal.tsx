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

import { useState, useEffect, useCallback, useRef } from "react";
import {
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip, Tag, Select, Table, Popconfirm,
} from "antd";
import {
  FundOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined, PlusOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, DescribeDataMetricFunction, AlterDataMetricFunction, GetDataMetricFunctionReferences, GetDataMetricFunctionTags } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle",
  width: 220,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }
// Double-quote an identifier, doubling embedded quotes — mirrors the Go side's
// QuoteIdent so a name/identifier containing `"` can't break out of the quoting.
function quoteIdent(s: string) { return '"' + s.replace(/"/g, '""') + '"'; }

// The DESCRIBE FUNCTION property names that carry the DMF detail, rendered first
// (in this order) under a dedicated section. "body" is the SQL expression SHOW
// DATA METRIC FUNCTIONS omits.
const DETAIL_ORDER = ["signature", "returns", "language", "body"];

// ─── EditRow (inline comment editor) ─────────────────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  canUnset?: boolean;
  onSave: (val: string) => Promise<void>;
  onUnset?: () => Promise<void>;
}

function EditRow({ label, value, canUnset, onSave, onUnset }: EditRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // A successful save may unmount the modal (e.g. rename closes it), so skip the
  // post-await state updates once unmounted.
  const mounted = useRef(true);
  useEffect(() => () => { mounted.current = false; }, []);

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(draft);
      if (mounted.current) setEditing(false);
    } catch (e) {
      if (mounted.current) setError(String(e));
    } finally {
      if (mounted.current) setSaving(false);
    }
  };

  const unset = async () => {
    if (!onUnset) return;
    setSaving(true);
    setError(null);
    try {
      await onUnset();
      if (mounted.current) setEditing(false);
    } catch (e) {
      if (mounted.current) setError(String(e));
    } finally {
      if (mounted.current) setSaving(false);
    }
  };

  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Input
                size="small"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                style={{ width: 280 }}
                onPressEnter={save}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} />
              </Tooltip>
              {canUnset && onUnset && (
                <Tooltip title="Unset (remove)">
                  <Button size="small" onClick={unset} loading={saving}>Unset</Button>
                </Tooltip>
              )}
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} />
              </Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">(not set)</Text>}</span>
            <Tooltip title="Edit">
              <Button
                type="text"
                size="small"
                icon={<EditOutlined style={{ fontSize: 11 }} />}
                onClick={() => { setDraft(value); setEditing(true); }}
                style={{ color: "var(--text-muted)" }}
              />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

function InfoRow({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{
        padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word",
        ...(mono ? { fontFamily: "var(--font-mono, monospace)", whiteSpace: "pre-wrap" } : {}),
      }}>
        {value || <Text type="secondary">(empty)</Text>}
      </td>
    </tr>
  );
}

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  /** TABLE argument signature (e.g. "TABLE(NUMBER)") needed to resolve the
   *  overload for DESCRIBE / ALTER FUNCTION. */
  args: string;
  onClose: () => void;
  /** Called after a change that alters the tree (a rename) so the caller can
   *  refresh the object store. */
  onChanged?: () => void;
}

export default function DataMetricFunctionPropertiesModal({ db, schema, name, args, onClose, onChanged }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [detail, setDetail] = useState<snowflake.QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [secureBusy, setSecureBusy] = useState(false);

  // The tables/views this DMF is scheduled against (ACCOUNT_USAGE — loaded on
  // demand because of its access requirements and latency).
  const [refs, setRefs] = useState<snowflake.QueryResult | null>(null);
  const [refsLoading, setRefsLoading] = useState(false);
  const [refsError, setRefsError] = useState<string | null>(null);
  const loadRefs = async () => {
    setRefsLoading(true);
    setRefsError(null);
    try {
      setRefs((await GetDataMetricFunctionReferences(db, schema, name)) ?? null);
    } catch (e) {
      setRefsError(String(e));
    } finally {
      setRefsLoading(false);
    }
  };

  // Tags currently applied to the DMF (no-latency INFORMATION_SCHEMA lookup),
  // each rendered as a removable chip; an add form issues SET TAG.
  type DmfTag = { qualified: string; label: string; value: string };
  const [tags, setTags] = useState<DmfTag[]>([]);
  const [tagsError, setTagsError] = useState<string | null>(null);
  const [newTagName, setNewTagName] = useState("");
  const [newTagValue, setNewTagValue] = useState("");
  const [tagBusy, setTagBusy] = useState(false);

  const loadTags = useCallback(async () => {
    setTagsError(null);
    try {
      const res = await GetDataMetricFunctionTags(db, schema, name, args);
      const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
      const di = cols.indexOf("tag_database");
      const si = cols.indexOf("tag_schema");
      const ni = cols.indexOf("tag_name");
      const vi = cols.indexOf("tag_value");
      const parsed: DmfTag[] = (res?.rows ?? []).map((r) => {
        const tdb = String(r[di] ?? "");
        const tsc = String(r[si] ?? "");
        const tnm = String(r[ni] ?? "");
        return {
          qualified: `${quoteIdent(tdb)}.${quoteIdent(tsc)}.${quoteIdent(tnm)}`,
          label: `${tdb}.${tsc}.${tnm}`,
          value: String(r[vi] ?? ""),
        };
      });
      setTags(parsed);
    } catch (e) {
      setTags([]);
      setTagsError(String(e));
    }
  }, [db, schema, name, args]);

  const reload = useCallback(async () => {
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "DATA METRIC FUNCTION", name);
      // DESCRIBE FUNCTION supplies the body SHOW DATA METRIC FUNCTIONS omits.
      // Failure is non-fatal — the overview/comment still render.
      let d: snowflake.QueryResult | null = null;
      try {
        d = (await DescribeDataMetricFunction(db, schema, name, args)) ?? null;
      } catch {
        d = null;
      }
      setRows(props ?? []);
      setDetail(d);
      await loadTags();
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name, args, loadTags]);

  useEffect(() => { reload(); }, [reload]);

  const fnRef = `"${db}"."${schema}"."${name}"(${args})`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  // Project the DESCRIBE FUNCTION result (property / value columns) into a map.
  const describeMap = (): Record<string, string> => {
    const out: Record<string, string> = {};
    if (!detail) return out;
    const cols = (detail.columns ?? []).map((c) => c.toLowerCase());
    const propCi = cols.indexOf("property");
    const valCi = cols.indexOf("value");
    if (propCi < 0 || valCi < 0) return out;
    for (const r of detail.rows ?? []) {
      const k = String(r[propCi] ?? "").toLowerCase();
      if (k) out[k] = String(r[valCi] ?? "");
    }
    return out;
  };

  const dmap = describeMap();

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterDataMetricFunction(db, schema, name, args, "UNSET COMMENT");
    } else {
      await AlterDataMetricFunction(db, schema, name, args, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const setSecure = async (secure: boolean) => {
    setSecureBusy(true);
    setActionError(null);
    try {
      await AlterDataMetricFunction(db, schema, name, args, secure ? "SET SECURE" : "UNSET SECURE");
      await reload();
    } catch (e) {
      setActionError(`Secure update failed: ${String(e)}`);
    } finally {
      setSecureBusy(false);
    }
  };

  // Renaming changes the object's identity (and the tree node), so after a
  // successful RENAME TO we notify the caller to refresh and close the modal —
  // the open instance is keyed to the old name. RENAME TO takes only the new
  // name (no argument signature), qualified to the same schema.
  const doRename = async (next: string) => {
    const trimmed = next.trim();
    if (trimmed === "" || trimmed === name) return;
    await AlterDataMetricFunction(db, schema, name, args, `RENAME TO ${quoteIdent(db)}.${quoteIdent(schema)}.${quoteIdent(trimmed)}`);
    onChanged?.();
    onClose();
  };

  const addTag = async () => {
    const tn = newTagName.trim();
    if (tn === "") return;
    setTagBusy(true);
    setActionError(null);
    try {
      // The tag name may already be qualified (db.schema.tag); pass it through
      // verbatim. The value is a string literal.
      await AlterDataMetricFunction(db, schema, name, args, `SET TAG ${tn} = ${q1(newTagValue)}`);
      setNewTagName("");
      setNewTagValue("");
      await loadTags();
    } catch (e) {
      setActionError(`Set tag failed: ${String(e)}`);
    } finally {
      setTagBusy(false);
    }
  };

  const removeTag = async (qualified: string) => {
    setTagBusy(true);
    setActionError(null);
    try {
      await AlterDataMetricFunction(db, schema, name, args, `UNSET TAG ${qualified}`);
      await loadTags();
    } catch (e) {
      setActionError(`Unset tag failed: ${String(e)}`);
    } finally {
      setTagBusy(false);
    }
  };

  const comment = find("description") || find("comment");
  const owner = find("owner");
  const createdOn = find("created_on");
  const language = find("language") || dmap["language"];
  const isSecure = (find("is_secure") || "").toUpperCase() === "Y" || (find("is_secure") || "").toUpperCase() === "TRUE";

  // Keys rendered in the dedicated sections, hidden from the generic SHOW dump.
  const handledKeys = new Set(["description", "comment", "owner", "created_on", "language", "is_secure"]);
  // The detail keys already shown in the Data Metric Function section.
  const detailHandled = new Set(DETAIL_ORDER);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <FundOutlined style={{ color: "var(--icon-datametricfunction)" }} />
          <span>Data Metric Function Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {fnRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={760}
      styles={{ body: { maxHeight: "74vh", overflowY: "auto", paddingTop: 16 } }}
    >
      {!rows && !error && (
        <div style={{ textAlign: "center", padding: 32 }}>
          <Spin />
        </div>
      )}
      {error && (
        <Alert type="error" message="Failed to load properties" description={error} showIcon />
      )}
      {rows && (
        <>
          {actionError && (
            <Alert
              type="error"
              message={actionError}
              showIcon
              closable
              onClose={() => setActionError(null)}
              style={{ marginBottom: 12 }}
            />
          )}

          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            Data metric functions return a numeric data-quality metric. The body
            below comes from DESCRIBE FUNCTION.
          </Text>

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <InfoRow label="Owner" value={owner} />
              <InfoRow label="Created on" value={createdOn} />
              <InfoRow label="Language" value={language} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow
                label="Name (rename)"
                value={name}
                onSave={doRename}
              />
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
              <tr>
                <td style={LABEL_TD}>Secure</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space>
                    <Tag color={isSecure ? "green" : "default"}>{isSecure ? "SECURE" : "NOT SECURE"}</Tag>
                    <Select
                      size="small"
                      value={isSecure ? "TRUE" : "FALSE"}
                      onChange={(v) => setSecure(v === "TRUE")}
                      loading={secureBusy}
                      style={{ width: 100 }}
                      options={[{ value: "TRUE", label: "Secure" }, { value: "FALSE", label: "Not secure" }]}
                    />
                  </Space>
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Tags</div>
          {tagsError ? (
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 8 }}>
              Could not read applied tags ({tagsError}). You can still set a tag below.
            </Text>
          ) : tags.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 8 }}>
              No tags applied.
            </Text>
          ) : (
            <Space size={[6, 6]} wrap style={{ marginBottom: 8 }}>
              {tags.map((t) => (
                <Popconfirm
                  key={t.qualified}
                  title={`Unset tag ${t.label}?`}
                  okText="Unset"
                  okButtonProps={{ danger: true, loading: tagBusy }}
                  onConfirm={() => removeTag(t.qualified)}
                >
                  <Tag closable onClose={(e) => e.preventDefault()} style={{ cursor: "pointer" }}>
                    {t.label}{t.value !== "" ? ` = ${t.value}` : ""}
                  </Tag>
                </Popconfirm>
              ))}
            </Space>
          )}
          <Space.Compact style={{ width: "100%" }}>
            <Input
              size="small"
              placeholder="tag (e.g. governance.pii)"
              value={newTagName}
              onChange={(e) => setNewTagName(e.target.value)}
              style={{ width: "45%" }}
            />
            <Input
              size="small"
              placeholder="value"
              value={newTagValue}
              onChange={(e) => setNewTagValue(e.target.value)}
              onPressEnter={addTag}
              style={{ width: "40%" }}
            />
            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={addTag}
              loading={tagBusy}
              disabled={newTagName.trim() === ""}
            >
              Set
            </Button>
          </Space.Compact>

          <div style={SECTION_HEAD}>Data Metric Function Detail</div>
          {detail ? (
            <table style={{ width: "100%", borderCollapse: "collapse" }}>
              <tbody>
                {DETAIL_ORDER.filter((k) => dmap[k] !== undefined).map((k) => (
                  <InfoRow key={k} label={k} value={dmap[k]} mono={k === "body"} />
                ))}
                {/* Any remaining DESCRIBE rows not in the canonical order. */}
                {Object.keys(dmap)
                  .filter((k) => !detailHandled.has(k))
                  .map((k) => (
                    <InfoRow key={k} label={k} value={dmap[k]} />
                  ))}
              </tbody>
            </table>
          ) : (
            <Text type="secondary" style={{ fontSize: 12 }}>
              DESCRIBE FUNCTION returned no detail (insufficient privileges, or the
              function was dropped).
            </Text>
          )}

          <div style={{ ...SECTION_HEAD, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <span>Associated Tables &amp; Views</span>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={loadRefs}
              loading={refsLoading}
            >
              {refs ? "Refresh" : "Load"}
            </Button>
          </div>
          {refsError && (
            <Alert
              type="warning"
              message="Could not load associations"
              description={`${refsError} — listing the tables a DMF is scheduled against reads SNOWFLAKE.ACCOUNT_USAGE.DATA_METRIC_FUNCTION_REFERENCES, which needs access to the SNOWFLAKE database and carries the usual ACCOUNT_USAGE latency.`}
              showIcon
              style={{ marginBottom: 8 }}
            />
          )}
          {refs ? (
            (refs.rows ?? []).length === 0 ? (
              <Text type="secondary" style={{ fontSize: 12 }}>
                This data metric function is not currently scheduled against any
                table or view (or the association has not yet propagated to
                ACCOUNT_USAGE).
              </Text>
            ) : (
              <Table
                size="small"
                rowKey={(_, i) => String(i)}
                pagination={false}
                columns={(refs.columns ?? []).map((c, ci) => ({
                  title: c,
                  dataIndex: String(ci),
                  key: String(ci),
                  render: (_: unknown, row: (string | null)[]) => String(row[ci] ?? ""),
                }))}
                dataSource={refs.rows ?? []}
                scroll={{ x: true }}
              />
            )
          ) : (
            !refsError && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                Load to list the tables and views this data metric function is
                scheduled against.
              </Text>
            )
          )}

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <InfoRow key={r.key} label={r.key} value={r.value} />
                ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
