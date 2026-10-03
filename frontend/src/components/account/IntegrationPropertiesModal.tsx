// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback, useRef } from "react";
import { Modal, Spin, Button, Input, Space, Typography, message } from "antd";
import { ApiOutlined, CheckOutlined, CopyOutlined, SearchOutlined, TagsOutlined } from "@ant-design/icons";
import { DescribeIntegration, AlterIntegrationProperty, AlterIntegration } from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { integrations } from "../../../wailsjs/go/models";
import { EditRow, InfoRow, SECTION_HEAD, LABEL_TD, friendlyError } from "../common/PropertyRows";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";

const { Text } = Typography;

/**
 * Set-only row for a secret DESCRIBE INTEGRATION never returns (API_KEY,
 * OAUTH_CLIENT_SECRET, …) — like the password row in UserPropertiesModal, but
 * with the Snowflake error inline under the row.
 */
function SecretRow({ label, search, onSave }: { label: string; search?: string; onSave: (val: string) => Promise<void> }) {
  const [val, setVal]       = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError]   = useState<string | null>(null);
  if (search && !label.toLowerCase().includes(search.toLowerCase())) return null;

  const save = async () => {
    if (!val.trim()) return;
    setSaving(true);
    setError(null);
    try {
      await onSave(val);
      setVal("");
      message.success(`${label} updated`);
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
        <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
          <Input.Password
            size="small"
            value={val}
            onChange={(e) => setVal(e.target.value)}
            placeholder="new value (never shown)"
            autoComplete="new-password"
            style={{ maxWidth: 260 }}
            onPressEnter={save}
          />
          <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} disabled={!val.trim()} onClick={save}>
            Set
          </Button>
        </div>
        {error && (
          <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace", lineHeight: 1.4, paddingLeft: 2, marginTop: 4 }}>
            {error}
          </div>
        )}
      </td>
    </tr>
  );
}

interface Props {
  /** STORAGE | API | CATALOG | EXTERNAL ACCESS | NOTIFICATION | SECURITY */
  kind:    string;
  name:    string;
  /** changed is true when at least one property was saved. */
  onClose: (changed: boolean) => void;
}

/**
 * Properties: INTEGRATION modal — one data-driven component for all six
 * integration kinds. Rows come from DESCRIBE INTEGRATION; the backend
 * allow-list (internal/integrations) marks which are editable and with which
 * editor, so no property list lives here. Each editable row saves on its own
 * through AlterIntegrationProperty, then DESCRIBE is re-read.
 */
export default function IntegrationPropertiesModal({ kind, name, onClose }: Props) {
  const [rows, setRows]           = useState<integrations.Property[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [search, setSearch]       = useState("");
  const changed = useRef(false);

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      setRows((await DescribeIntegration(kind, name)) ?? []);
    } catch (e) {
      setRows([]);
      setLoadError(friendlyError(e));
    }
  }, [kind, name]);

  useEffect(() => { load(); }, [load]);

  const tags = useObjectTags({
    kind: "INTEGRATION", db: "", schema: "", name,
    alter: (clause) => AlterIntegration(kind, name, clause),
  });

  const save = (property: string) => async (v: string) => {
    await AlterIntegrationProperty(kind, name, property, v);
    changed.current = true;
    await load();
  };

  const copyAll = () => {
    const text = (rows ?? []).filter((r) => !r.secret).map((r) => `${r.name}\t${r.value}`).join("\n");
    ClipboardSetText(text).then(() => message.success("Copied to clipboard"));
  };

  const close = () => onClose(changed.current);
  const tableStyle: React.CSSProperties = { width: "100%", borderCollapse: "collapse", fontSize: 12 };

  const renderRow = (r: integrations.Property) => {
    if (r.secret) return <SecretRow key={r.name} label={r.name} search={search} onSave={save(r.name)} />;
    if (!r.editable) return <InfoRow key={r.name} label={r.name} value={r.value} search={search} />;

    const clear = r.unsettable ? " — clear to unset" : "";
    switch (r.editor) {
      case "boolean":
        return <EditRow key={r.name} label={r.name} value={r.value} type="boolean" search={search} onSave={save(r.name)} />;
      case "number":
        return (
          <EditRow
            key={r.name} label={r.name} value={/^\d+$/.test(r.value) ? r.value : ""} type="number" search={search}
            allowEmpty={r.unsettable} hint={r.unsettable ? "Clear to unset" : undefined} onSave={save(r.name)}
          />
        );
      case "select":
        return (
          <EditRow
            key={r.name} label={r.name} value={r.value.toUpperCase()} type="select" search={search}
            options={[
              ...(r.unsettable ? [{ value: "", label: "— unset —" }] : []),
              ...(r.options ?? []).map((o) => ({ value: o, label: o })),
            ]}
            onSave={save(r.name)}
          />
        );
      case "list":
      case "identList": {
        const keywords = (r.options ?? []).length ? `, or ${(r.options ?? []).join(" / ")}` : "";
        return (
          <EditRow
            key={r.name} label={r.name} value={r.value} type="text" search={search}
            hint={`Comma-separated${keywords}${clear}`} onSave={save(r.name)}
          />
        );
      }
      default:
        return (
          <EditRow
            key={r.name} label={r.name} value={r.value} type="text" search={search}
            hint={r.unsettable ? "Clear to unset" : undefined} onSave={save(r.name)}
          />
        );
    }
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ApiOutlined style={{ color: "var(--link)" }} />
          <span>Properties: {kind} INTEGRATION</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{name}</Text>
        </Space>
      }
      onCancel={close}
      footer={[
        <Button key="copy" icon={<CopyOutlined />} disabled={!rows?.length} onClick={copyAll}>Copy</Button>,
        <Button key="close" onClick={close}>Close</Button>,
      ]}
      width={760}
      styles={{ body: { paddingTop: 12, maxHeight: "72vh", overflowY: "auto" } }}
    >
      {loadError && (
        <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>{loadError}</div>
      )}
      {rows === null && !loadError && (
        <div style={{ textAlign: "center", padding: "32px 0" }}><Spin /></div>
      )}

      {rows !== null && !loadError && (
        <>
          <Input
            prefix={<SearchOutlined style={{ color: "var(--text-faint)" }} />}
            placeholder="Search properties…"
            allowClear
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ marginBottom: 8 }}
          />
          <table style={tableStyle}><tbody>{rows.map(renderRow)}</tbody></table>

          {(!search || "tags".includes(search.toLowerCase())) && (
            <>
              <div style={SECTION_HEAD}><Space size={6}><TagsOutlined />Tags</Space></div>
              <table style={tableStyle}><tbody>
                <TagsRow hideLabel tags={tags.tags} nameOptions={tags.nameOptions} onSetTag={tags.setTag} onUnsetTag={tags.unsetTag} />
              </tbody></table>
            </>
          )}
        </>
      )}
    </Modal>
  );
}
