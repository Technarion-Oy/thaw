// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect, useCallback } from "react";
import { Modal, Spin, Button, Input, Space, Typography, Alert, message } from "antd";
import { UserOutlined, CheckOutlined, SearchOutlined, KeyOutlined, DeleteOutlined } from "@ant-design/icons";
import {
  GetObjectProperties, AlterUserProperty, ListWarehouses, ListRoles, ParseSecondaryRoles,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { EditRow, InfoRow, SECTION_HEAD, LABEL_TD, friendlyError } from "../common/PropertyRows";
import KeyPairAuthModal, { type KeySlot, SLOT_PROPERTY } from "./KeyPairAuthModal";

const { Text } = Typography;

// ─── Helper: RSA public-key slot row ─────────────────────────────────────────

/**
 * One row per RSA public-key slot (Key 1 / Key 2). Shows the current
 * fingerprint and last-set time from DESCRIBE USER, or "not set" — and, when the
 * role can't DESCRIBE USER (degraded), a caveat instead of implying "not set".
 * Set/Replace opens KeyPairAuthModal targeting this slot; Remove UNSETs it.
 */
function KeyPairSlotRow({
  name, slot, label, fp, lastSet, degraded, onReload, onOpen, search,
}: {
  name: string;
  slot: KeySlot;
  label: string;
  fp: string;
  lastSet: string;
  degraded: boolean;
  onReload: () => Promise<void>;
  onOpen: () => void;
  search?: string;
}) {
  const [removing, setRemoving] = useState(false);
  const hay = `${label} rsa public key fingerprint`.toLowerCase();
  if (search && !hay.includes(search.toLowerCase())) return null;

  const hasKey = fp.trim() !== "";

  const remove = () => {
    Modal.confirm({
      title: `Remove ${label} from ${name}?`,
      content: "This UNSETs the RSA public key in this slot. Anyone still " +
        "authenticating with the matching private key will be locked out.",
      okText: "Remove",
      okButtonProps: { danger: true },
      onOk: async () => {
        setRemoving(true);
        try {
          await AlterUserProperty(name, SLOT_PROPERTY[slot], "");
          message.success(`Removed ${label} from ${name}`);
          await onReload();
        } catch (e) {
          message.error(friendlyError(e), 6);
        } finally {
          setRemoving(false);
        }
      },
    });
  };

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
          <div style={{ flex: 1, minWidth: 200 }}>
            {degraded && !hasKey ? (
              <span style={{ fontSize: 11, fontStyle: "italic", color: "var(--text-faint)" }}>
                unknown — your role can't DESCRIBE USER
              </span>
            ) : hasKey ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <span style={{ fontFamily: "monospace", fontSize: 11, wordBreak: "break-all", color: "var(--text)" }}>
                  {fp}
                </span>
                {lastSet && (
                  <span style={{ fontSize: 10, color: "var(--text-muted)" }}>set {lastSet}</span>
                )}
              </div>
            ) : (
              <span style={{ fontSize: 12, fontStyle: "italic", color: "var(--text-faint)" }}>not set</span>
            )}
          </div>
          <Space size={4}>
            <Button size="small" icon={<KeyOutlined />} onClick={onOpen}>
              {hasKey ? "Replace…" : "Set…"}
            </Button>
            {/* Also offered when degraded: the role can't DESCRIBE USER, so a
                key may be set even though the fingerprint reads "unknown" —
                let the admin UNSET defensively rather than hiding the option. */}
            {(hasKey || degraded) && (
              <Button size="small" danger icon={<DeleteOutlined />} loading={removing} onClick={remove}>
                Remove
              </Button>
            )}
          </Space>
        </div>
      </td>
    </tr>
  );
}

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
          <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} disabled={!val.trim()} onClick={save}>
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
  // "ALL" | "NONE" | "" (unset) — derived from DEFAULT_SECONDARY_ROLES via the
  // backend's tested ParseSecondaryRoles rather than a bespoke regex.
  const [dsr, setDsr]             = useState("");
  // Which RSA public-key slot's KeyPairAuthModal is open (null = none).
  const [keyModal, setKeyModal]   = useState<KeySlot | null>(null);

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const r = await GetObjectProperties("", "", "USER", name);
      setRows(r ?? []);
      const raw = (r ?? []).find((p) => p.key.toUpperCase() === "DEFAULT_SECONDARY_ROLES")?.value ?? "";
      if (!raw.trim() || raw.trim() === "null") {
        setDsr("");
      } else {
        // On parse failure keep the raw value: the select then displays it
        // verbatim (an unknown option) instead of masquerading as "— unset —",
        // and the no-op guard prevents a save from that state.
        const roles = await ParseSecondaryRoles(raw).catch(() => null);
        setDsr(roles === null ? raw.trim() : roles.some((x) => x.toUpperCase() === "ALL") ? "ALL" : "NONE");
      }
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

  const save = (property: string) => async (v: string) => {
    await AlterUserProperty(name, property, v);
    await load();
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
          {/* Backend marker: DESCRIBE USER was denied for this role, so
              DESCRIBE-only fields below may be hidden rather than unset. */}
          {m["__DESCRIBE_DEGRADED__"] === "1" && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 8 }}
              message={<span style={{ fontSize: 12 }}>
                Some properties (network policy, middle name, …) may be hidden — your role
                lacks DESCRIBE USER on this user, so blank fields below are not necessarily unset.
              </span>}
            />
          )}
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
            <EditRow label="Days to expiry"     value={numVal("DAYS_TO_EXPIRY")}     type="number" allowEmpty search={search} hint="Clear to remove expiry"       onSave={save("daysToExpiry")} />
            <EditRow label="Mins to unlock"     value={numVal("MINS_TO_UNLOCK")}     type="number" allowEmpty search={search} hint="Clear to reset"               onSave={save("minsToUnlock")} />
            <EditRow label="Mins to bypass MFA" value={numVal("MINS_TO_BYPASS_MFA")} type="number" allowEmpty search={search} hint="Requires MFA enrolment; clear to reset" onSave={save("minsToBypassMfa")} />
            <PasswordRow search={search} onSave={async (v) => { await AlterUserProperty(name, "password", v); await load(); }} />
          </tbody></table>

          <div style={SECTION_HEAD}>Key pair authentication</div>
          {!search && (
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 6, lineHeight: 1.5 }}>
              Two slots enable zero-downtime rotation: set <b>Key 2</b>, migrate every
              client to the new private key, then remove <b>Key 1</b>.
            </div>
          )}
          {/* _FP and _LAST_SET_TIME (both slots) are real DESCRIBE USER
              properties, merged into the map by GetObjectProperties; when the
              role can't DESCRIBE USER they're absent and the rows fall back to
              the degraded / "not set" states. */}
          <table style={tableStyle}><tbody>
            <KeyPairSlotRow
              name={name} slot="RSA_PUBLIC_KEY" label="Key 1" search={search}
              fp={val("RSA_PUBLIC_KEY_FP")} lastSet={val("RSA_PUBLIC_KEY_LAST_SET_TIME")}
              degraded={m["__DESCRIBE_DEGRADED__"] === "1"}
              onReload={load} onOpen={() => setKeyModal("RSA_PUBLIC_KEY")}
            />
            <KeyPairSlotRow
              name={name} slot="RSA_PUBLIC_KEY_2" label="Key 2" search={search}
              fp={val("RSA_PUBLIC_KEY_2_FP")} lastSet={val("RSA_PUBLIC_KEY_2_LAST_SET_TIME")}
              degraded={m["__DESCRIBE_DEGRADED__"] === "1"}
              onReload={load} onOpen={() => setKeyModal("RSA_PUBLIC_KEY_2")}
            />
          </tbody></table>

          <div style={SECTION_HEAD}>Info</div>
          <table style={tableStyle}><tbody>
            <InfoRow label="Owner"              value={val("OWNER")}              search={search} />
            <InfoRow label="Created on"         value={val("CREATED_ON")}         search={search} />
            <InfoRow label="Last success login" value={val("LAST_SUCCESS_LOGIN")} search={search} />
            <InfoRow label="Has password"       value={val("HAS_PASSWORD")}       search={search} />
            {/* RSA slots are set/removed above; this SHOW USERS boolean is kept
                because it survives DESCRIBE-degraded mode (when the per-slot
                fingerprints read "unknown"), so a lower-privileged admin can
                still tell whether any key is set. */}
            <InfoRow label="Has RSA public key" value={val("HAS_RSA_PUBLIC_KEY")} search={search} />
            {/* Read-only: MFA is managed via Mins to bypass MFA above or
                ALTER USER … REMOVE MFA METHOD in the SQL editor. */}
            <InfoRow label="MFA (Duo)" value={val("EXT_AUTHN_DUO")} search={search} />
          </tbody></table>
        </>
      )}

      {keyModal && (
        <KeyPairAuthModal
          username={name}
          slot={keyModal}
          slotHasKey={val(keyModal === "RSA_PUBLIC_KEY" ? "RSA_PUBLIC_KEY_FP" : "RSA_PUBLIC_KEY_2_FP").trim() !== ""}
          onApplied={load}
          onClose={() => setKeyModal(null)}
        />
      )}
    </Modal>
  );
}
