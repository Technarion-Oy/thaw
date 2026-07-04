// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useCallback } from "react";
import { Modal, Spin, Button, Input, Space, Typography, Popconfirm, message } from "antd";
import { UserOutlined, CheckOutlined, SearchOutlined } from "@ant-design/icons";
import {
  GetObjectProperties, AlterUserProperty, ListWarehouses, ListRoles,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { EditRow, InfoRow, SECTION_HEAD, LABEL_TD, friendlyError } from "../common/PropertyRows";

const { Text } = Typography;

// ─── Helper: password reset row ──────────────────────────────────────────────

function PasswordRow({ onSave, search }: { onSave: (val: string) => Promise<void>; search?: string }) {
  const [val, setVal]       = useState("");
  const [saving, setSaving] = useState(false);
  if (search && !"password".includes(search.toLowerCase())) return null;

  const save = async () => {
    setSaving(true);
    try {
      await onSave(val);
      setVal("");
      message.success("Password updated");
    } catch (e) {
      message.error(friendlyError(e), 6);
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>Password</td>
      <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
        <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
          <Input.Password
            size="small"
            value={val}
            onChange={(e) => setVal(e.target.value)}
            placeholder="new password"
            autoComplete="new-password"
            style={{ maxWidth: 220 }}
            onPressEnter={save}
          />
          <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} disabled={!val} onClick={save}>
            Set
          </Button>
        </div>
      </td>
    </tr>
  );
}

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  name:    string;
  onClose: () => void;
}

/**
 * Properties: USER modal — the same per-property inline-edit pattern as
 * WarehousePropertiesModal and the object *PropertiesModals: every settable
 * ALTER USER property is a typed EditRow that saves independently through the
 * AlterUserProperty IPC (backed by users.BuildAlterUserPropertySQL), then the
 * property list reloads. Values come from SHOW USERS via GetObjectProperties.
 */
export default function UserPropertiesModal({ name, onClose }: Props) {
  const [rows, setRows]           = useState<snowflake.PropertyPair[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [search, setSearch]       = useState("");

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const r = await GetObjectProperties("", "", "USER", name);
      setRows(r ?? []);
    } catch (e) {
      setRows([]);
      setLoadError(friendlyError(e));
    }
  }, [name]);

  useEffect(() => { load(); }, [load]);

  // Lazy option loaders for the warehouse/role dropdowns — fetched by EditRow
  // the first time the row enters edit mode, not on every modal open.
  const noneOpt = { value: "", label: "— none —" };
  const warehouseOptions = () =>
    ListWarehouses().then((w) => [noneOpt, ...(w ?? []).map((x) => ({ value: x, label: x }))]);
  const roleOptions = () =>
    ListRoles().then((r) => [noneOpt, ...(r ?? []).map((x) => ({ value: x, label: x }))]);

  // SHOW USERS column → value, keys uppercased.
  const m: Record<string, string> = {};
  for (const p of rows ?? []) m[p.key.toUpperCase()] = p.value;
  const val = (key: string) => {
    const v = (m[key] ?? "").trim();
    return v === "null" ? "" : v;
  };
  const numVal = (key: string) => (/^\d+$/.test(val(key)) ? val(key) : "");

  // default_secondary_roles renders as e.g. ["ALL"] or [].
  const dsr = /all/i.test(val("DEFAULT_SECONDARY_ROLES")) ? "ALL"
    : val("DEFAULT_SECONDARY_ROLES").replace(/\s/g, "") === "[]" ? "NONE" : "";

  const save = (property: string) => async (v: string) => {
    await AlterUserProperty(name, property, v);
    await load();
  };

  const disableMfa = async () => {
    try {
      await AlterUserProperty(name, "disableMfa", "TRUE");
      message.success("MFA disabled");
      await load();
    } catch (e) {
      message.error(friendlyError(e), 6);
    }
  };

  const tableStyle: React.CSSProperties = { width: "100%", borderCollapse: "collapse", fontSize: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <UserOutlined style={{ color: "var(--link)" }} />
          <span>Properties: USER</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{name}</Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={640}
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
            style={{ marginBottom: 4 }}
          />

          <div style={SECTION_HEAD}>Identity</div>
          <table style={tableStyle}><tbody>
            <EditRow label="Login name"   value={val("LOGIN_NAME")}   type="text" search={search} onSave={save("loginName")} />
            <EditRow label="Display name" value={val("DISPLAY_NAME")} type="text" search={search} onSave={save("displayName")} />
            <EditRow label="First name"   value={val("FIRST_NAME")}   type="text" search={search} onSave={save("firstName")} />
            <EditRow label="Middle name"  value={val("MIDDLE_NAME")}  type="text" search={search} onSave={save("middleName")} />
            <EditRow label="Last name"    value={val("LAST_NAME")}    type="text" search={search} onSave={save("lastName")} />
            <EditRow label="Email"        value={val("EMAIL")}        type="text" search={search} onSave={save("email")} />
            <EditRow label="Comment"      value={val("COMMENT")}      type="text" search={search} onSave={save("comment")} />
            <EditRow
              label="Type" value={val("TYPE").toUpperCase()} type="select" search={search}
              options={[
                { value: "", label: "— unset —" },
                { value: "PERSON", label: "PERSON" },
                { value: "SERVICE", label: "SERVICE" },
                { value: "LEGACY_SERVICE", label: "LEGACY_SERVICE" },
              ]}
              onSave={save("type")}
            />
          </tbody></table>

          <div style={SECTION_HEAD}>Defaults</div>
          <table style={tableStyle}><tbody>
            <EditRow
              label="Default warehouse" value={val("DEFAULT_WAREHOUSE")} type="select" search={search}
              loadOptions={warehouseOptions}
              onSave={save("defaultWarehouse")}
            />
            <EditRow
              label="Default role" value={val("DEFAULT_ROLE")} type="select" search={search}
              loadOptions={roleOptions}
              onSave={save("defaultRole")}
            />
            <EditRow
              label="Default namespace" value={val("DEFAULT_NAMESPACE")} type="text" search={search}
              hint="DATABASE or DATABASE.SCHEMA — clear to unset"
              onSave={save("defaultNamespace")}
            />
            <EditRow
              label="Default secondary roles" value={dsr} type="select" search={search}
              options={[{ value: "", label: "— unset —" }, { value: "ALL", label: "All" }, { value: "NONE", label: "None" }]}
              onSave={save("defaultSecondaryRoles")}
            />
            <EditRow
              label="Network policy" value={val("NETWORK_POLICY")} type="text" search={search}
              hint="Policy name — clear to unset"
              onSave={save("networkPolicy")}
            />
          </tbody></table>

          <div style={SECTION_HEAD}>Security</div>
          <table style={tableStyle}><tbody>
            <EditRow label="Disabled"             value={val("DISABLED")}             type="boolean" search={search} onSave={save("disabled")} />
            <EditRow label="Must change password" value={val("MUST_CHANGE_PASSWORD")} type="boolean" search={search} onSave={save("mustChangePassword")} />
            <EditRow label="Days to expiry"     value={numVal("DAYS_TO_EXPIRY")}     type="number" search={search} hint="Clear to remove expiry"       onSave={save("daysToExpiry")} />
            <EditRow label="Mins to unlock"     value={numVal("MINS_TO_UNLOCK")}     type="number" search={search} hint="Clear to reset"               onSave={save("minsToUnlock")} />
            <EditRow label="Mins to bypass MFA" value={numVal("MINS_TO_BYPASS_MFA")} type="number" search={search} hint="Requires MFA enrolment; clear to reset" onSave={save("minsToBypassMfa")} />
            <PasswordRow search={search} onSave={async (v) => { await AlterUserProperty(name, "password", v); await load(); }} />
          </tbody></table>

          <div style={SECTION_HEAD}>Info</div>
          <table style={tableStyle}><tbody>
            <InfoRow label="Owner"              value={val("OWNER")}              search={search} />
            <InfoRow label="Created on"         value={val("CREATED_ON")}         search={search} />
            <InfoRow label="Last success login" value={val("LAST_SUCCESS_LOGIN")} search={search} />
            <InfoRow label="Has password"       value={val("HAS_PASSWORD")}       search={search} />
            <InfoRow label="Has RSA public key" value={val("HAS_RSA_PUBLIC_KEY")} search={search} />
            <InfoRow
              label="MFA (Duo)" value={val("EXT_AUTHN_DUO")} search={search}
              extra={val("EXT_AUTHN_DUO").toLowerCase() === "true" && (
                <Popconfirm title="Disable MFA for this user?" okText="Disable" onConfirm={disableMfa}>
                  <Button size="small" danger style={{ marginLeft: 12 }}>Disable MFA</Button>
                </Popconfirm>
              )}
            />
          </tbody></table>
        </>
      )}
    </Modal>
  );
}
