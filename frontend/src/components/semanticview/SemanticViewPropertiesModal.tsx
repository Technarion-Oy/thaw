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

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip, Table, Empty, Popconfirm,
} from "antd";
import {
  ApartmentOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterSemanticView, GetSemanticViewTags,
  DescribeSemanticView, ListSemanticDimensions, ListSemanticFacts, ListSemanticMetrics,
  ListSemanticDimensionsForMetric,
} from "../../../wailsjs/go/app/App";
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
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "top",
  width: 160,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }

// Render a raw QueryResult (columns + rows) as an antd Table. Shared by the
// Dimensions / Facts / Metrics / Describe sections, all of which expose
// SHOW/DESCRIBE output verbatim.
function ResultTable({ res }: { res: snowflake.QueryResult }) {
  if (!res.rows || res.rows.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No rows" />;
  }
  return (
    <Table
      size="small"
      rowKey={(_r, i) => String(i)}
      pagination={res.rows.length > 10 ? { pageSize: 10 } : false}
      scroll={{ x: true }}
      columns={(res.columns ?? []).map((c, ci) => ({
        title: c,
        dataIndex: ci,
        key: String(ci),
        ellipsis: true,
        render: (v: unknown) => (v === null || v === undefined ? "" : String(v)),
      }))}
      dataSource={res.rows.map((row) => {
        const obj: Record<number, unknown> = {};
        row.forEach((cell, ci) => { obj[ci] = cell; });
        return obj;
      })}
    />
  );
}

// A lazily-loaded section: a Load/Refresh button that fetches a QueryResult on
// demand (SHOW SEMANTIC … can be slow and shouldn't run until the user asks).
function LazySection({
  title, description, loader,
}: {
  title: string;
  description?: string;
  loader: () => Promise<snowflake.QueryResult>;
}) {
  const [res, setRes] = useState<snowflake.QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      setRes(await loader());
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <div style={SECTION_HEAD}>{title}</div>
      {description && (
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          {description}
        </Text>
      )}
      {error && (
        <Alert type="warning" message={`Could not load ${title.toLowerCase()}`} description={error} showIcon style={{ marginBottom: 8 }} />
      )}
      <Button size="small" icon={<ReloadOutlined />} onClick={load} loading={loading} style={{ marginBottom: 8 }}>
        {res ? "Refresh" : "Load"}
      </Button>
      {res && <ResultTable res={res} />}
    </>
  );
}

// ─── EditRow (single-line settings, e.g. comment) ────────────────────────────

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

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(draft);
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const unset = async () => {
    if (!onUnset) return;
    setSaving(true);
    setError(null);
    try {
      await onUnset();
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
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

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function SemanticViewPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Tags — loaded with the properties (immediate-consistency INFORMATION_SCHEMA
  // read); SET/UNSET TAG to edit.
  const [tags, setTags] = useState<snowflake.QueryResult | null>(null);
  const [addingTag, setAddingTag] = useState(false);
  const [tagName, setTagName] = useState("");
  const [tagValue, setTagValue] = useState("");

  // Dimensions-for-metric lookup — which dimensions are queryable alongside a
  // given metric.
  const [metricName, setMetricName] = useState("");
  const [forMetric, setForMetric] = useState<snowflake.QueryResult | null>(null);
  const [forMetricError, setForMetricError] = useState<string | null>(null);
  const [forMetricLoading, setForMetricLoading] = useState(false);

  const reloadTags = useCallback(async () => {
    try {
      const t = await GetSemanticViewTags(db, schema, name);
      setTags(t);
    } catch {
      // Tag listing is best-effort; SET/UNSET TAG still work if the read fails.
      setTags(null);
    }
  }, [db, schema, name]);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "SEMANTIC VIEW", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
    reloadTags();
  }, [db, schema, name, reloadTags]);

  useEffect(() => { reload(); }, [reload]);

  const viewRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterSemanticView(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterSemanticView(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const addTag = async () => {
    const tn = tagName.trim();
    if (tn === "") return;
    setActionError(null);
    try {
      // Tag name may be a qualified identifier (db.schema.tag) — inserted
      // verbatim; the value is a quoted string literal.
      await AlterSemanticView(db, schema, name, `SET TAG ${tn} = ${q1(tagValue)}`);
      setAddingTag(false);
      setTagName("");
      setTagValue("");
      await reloadTags();
    } catch (e) {
      setActionError(`Set tag failed: ${String(e)}`);
    }
  };

  const removeTag = async (qualifiedName: string) => {
    setActionError(null);
    try {
      await AlterSemanticView(db, schema, name, `UNSET TAG ${qualifiedName}`);
      await reloadTags();
    } catch (e) {
      setActionError(`Unset tag failed: ${String(e)}`);
    }
  };

  const loadForMetric = async () => {
    if (metricName.trim() === "") return;
    setForMetricLoading(true);
    setForMetricError(null);
    try {
      setForMetric(await ListSemanticDimensionsForMetric(db, schema, name, metricName.trim()));
    } catch (e) {
      setForMetricError(String(e));
    } finally {
      setForMetricLoading(false);
    }
  };

  const comment = find("comment");
  const owner = find("owner");
  const createdOn = find("created_on");

  // Keys handled by dedicated sections above the generic Properties table.
  const handledKeys = new Set(["comment", "owner", "created_on"]);

  // Resolve the tag columns once (GetSemanticViewTags returns INFORMATION_SCHEMA
  // TAG_REFERENCES rows: tag_database/tag_schema/tag_name/tag_value).
  const tagRows = (() => {
    if (!tags || !tags.columns || !tags.rows) return [];
    const idx = (n: string) => tags.columns.findIndex((c) => c.toLowerCase() === n);
    const dbI = idx("tag_database"), scI = idx("tag_schema"), nmI = idx("tag_name"), vlI = idx("tag_value");
    if (nmI < 0) return [];
    return tags.rows.map((row) => {
      const tdb = dbI >= 0 ? String(row[dbI] ?? "") : "";
      const tsc = scI >= 0 ? String(row[scI] ?? "") : "";
      const tnm = String(row[nmI] ?? "");
      const qualified = [tdb, tsc, tnm].filter(Boolean).map((p) => `"${p}"`).join(".");
      return { name: tnm, qualified, value: vlI >= 0 ? String(row[vlI] ?? "") : "" };
    });
  })();

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ApartmentOutlined style={{ color: "var(--link)" }} />
          <span>Semantic View Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {viewRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={860}
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

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Owner</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>{owner || <Text type="secondary">(unknown)</Text>}</td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Created</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>{createdOn || <Text type="secondary">(unknown)</Text>}</td>
              </tr>
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Tags</div>
          <Space direction="vertical" size={8} style={{ width: "100%" }}>
            <Space wrap>
              {tagRows.length === 0 && <Text type="secondary" style={{ fontSize: 12 }}>No tags set.</Text>}
              {tagRows.map((t) => (
                <Popconfirm
                  key={t.qualified}
                  title={`Unset tag ${t.name}?`}
                  onConfirm={() => removeTag(t.qualified)}
                  okText="Unset"
                  cancelText="Cancel"
                >
                  <span style={{
                    display: "inline-flex", alignItems: "center", gap: 4,
                    border: "1px solid var(--border)", borderRadius: 4,
                    padding: "1px 6px", fontSize: 12, cursor: "pointer",
                  }}>
                    <span style={{ color: "var(--text)" }}>{t.name}</span>
                    {t.value !== "" && <span style={{ color: "var(--text-muted)" }}>= {t.value}</span>}
                    <CloseOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />
                  </span>
                </Popconfirm>
              ))}
            </Space>
            {addingTag ? (
              <Space>
                <Input size="small" placeholder="tag name (or db.schema.tag)" value={tagName} onChange={(e) => setTagName(e.target.value)} style={{ width: 220 }} />
                <Input size="small" placeholder="value" value={tagValue} onChange={(e) => setTagValue(e.target.value)} style={{ width: 160 }} onPressEnter={addTag} />
                <Button size="small" type="primary" icon={<CheckOutlined />} onClick={addTag} disabled={tagName.trim() === ""} />
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setAddingTag(false); setTagName(""); setTagValue(""); }} />
              </Space>
            ) : (
              <Button size="small" icon={<PlusOutlined />} onClick={() => setAddingTag(true)}>Add tag</Button>
            )}
          </Space>

          <LazySection
            title="Structure"
            description="DESCRIBE SEMANTIC VIEW — one row per logical table, relationship, dimension, fact, or metric property."
            loader={() => DescribeSemanticView(db, schema, name)}
          />

          <LazySection
            title="Dimensions"
            description="SHOW SEMANTIC DIMENSIONS — the dimensions exposed by this view."
            loader={() => ListSemanticDimensions(db, schema, name)}
          />

          <LazySection
            title="Facts"
            description="SHOW SEMANTIC FACTS — the facts exposed by this view."
            loader={() => ListSemanticFacts(db, schema, name)}
          />

          <LazySection
            title="Metrics"
            description="SHOW SEMANTIC METRICS — the metrics exposed by this view."
            loader={() => ListSemanticMetrics(db, schema, name)}
          />

          <div style={SECTION_HEAD}>Dimensions for metric</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            SHOW SEMANTIC DIMENSIONS … FOR METRIC — which dimensions can be queried alongside a specific metric.
          </Text>
          {forMetricError && (
            <Alert type="warning" message="Could not load dimensions for metric" description={forMetricError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Space style={{ marginBottom: 8 }}>
            <Input
              size="small"
              placeholder="metric name"
              value={metricName}
              onChange={(e) => setMetricName(e.target.value)}
              style={{ width: 220 }}
              onPressEnter={loadForMetric}
            />
            <Button size="small" icon={<ReloadOutlined />} onClick={loadForMetric} loading={forMetricLoading} disabled={metricName.trim() === ""}>
              Show
            </Button>
          </Space>
          {forMetric && <ResultTable res={forMetric} />}

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <tr key={r.key}>
                    <td style={LABEL_TD}>{r.key}</td>
                    <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
                      {r.value || <Text type="secondary">(empty)</Text>}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
