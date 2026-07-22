// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect, useCallback } from "react";
import { Modal, Spin, Button, Input, Space, Typography, Alert, Checkbox, Select, message } from "antd";
import { UserOutlined, CheckOutlined, SearchOutlined, KeyOutlined, DeleteOutlined, TagsOutlined, SafetyOutlined, ApiOutlined } from "@ant-design/icons";
import {
  GetObjectProperties, AlterUserProperty, ListWarehouses, ListRoles, ParseSecondaryRoles,
  SetUserPolicy, UnsetUserPolicy, SetUserTags, UnsetUserTags,
  RemoveUserMfaMethod, AddUserDelegatedAuth, RemoveUserDelegatedAuth,
  ListAccountAuthenticationPolicies, ListAccountPasswordPolicies, ListAccountSessionPolicies,
  ListSecurityIntegrations, ListAccountTags, GetUserTagReferences,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { EditRow, InfoRow, SECTION_HEAD, LABEL_TD, friendlyError } from "../common/PropertyRows";
import TagsRow, { type EditableTag } from "../shared/TagsRow";
import { parseAllowedValues } from "../tag/allowedValues";
import KeyPairAuthModal, { type KeySlot, SLOT_PROPERTY } from "./KeyPairAuthModal";

const { Text } = Typography;

// quoteIdent double-quotes an identifier part, doubling embedded quotes — the
// client-side mirror of snowflake.QuoteIdent, used to build the quoted FQN that
// the ALTER USER builders parse back with exact case.
const quoteIdent = (s: string) => `"${s.replace(/"/g, '""')}"`;

// A dropdown option parsed from a SHOW … result. allowedValues is populated from
// SHOW TAGS' allowed_values column (empty for policies, which have no such
// column) — a non-empty list turns the tag value field into a whitelist dropdown.
interface NameOption { value: string; label: string; allowedValues: string[] }

// nameOptionsFromShow turns a SHOW … result (which shares the
// name / database_name / schema_name columns across POLICIES and TAGS) into
// dropdown options: the value is the quoted FQN (passed to the ALTER builders so
// mixed-case names round-trip), the label the readable dotted name, and (for
// tags) allowedValues parsed from the allowed_values column.
function nameOptionsFromShow(res: snowflake.QueryResult | null): NameOption[] {
  const cols = res?.columns ?? [];
  const iName    = cols.indexOf("name");
  const iDb      = cols.indexOf("database_name");
  const iSc      = cols.indexOf("schema_name");
  const iAllowed = cols.indexOf("allowed_values");
  if (iName < 0) return [];
  return (res?.rows ?? []).map((r) => {
    const nm = String(r[iName]);
    const db = iDb >= 0 && r[iDb] != null ? String(r[iDb]) : "";
    const sc = iSc >= 0 && r[iSc] != null ? String(r[iSc]) : "";
    const parts = [db, sc, nm].filter(Boolean);
    const allowedValues = iAllowed >= 0 && r[iAllowed] != null ? parseAllowedValues(String(r[iAllowed])) : [];
    return { value: parts.map(quoteIdent).join("."), label: parts.join("."), allowedValues };
  });
}

// userTagsToEditable maps a GetUserTagReferences result (TAG_DATABASE /
// TAG_SCHEMA / TAG_NAME / TAG_VALUE) into removable chips. The chip key is the
// quoted FQN handed straight to UnsetUserTags.
function userTagsToEditable(res: snowflake.QueryResult | null): EditableTag[] {
  const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
  const ci = (n: string) => cols.indexOf(n);
  const dbI = ci("tag_database"), scI = ci("tag_schema"), nmI = ci("tag_name"), vlI = ci("tag_value");
  if (nmI < 0) return [];
  return (res?.rows ?? []).map((row): EditableTag => {
    const tdb = dbI >= 0 && row[dbI] != null ? String(row[dbI]) : "";
    const tsc = scI >= 0 && row[scI] != null ? String(row[scI]) : "";
    const tnm = String(row[nmI] ?? "");
    const qualified = [tdb, tsc, tnm].filter(Boolean).map(quoteIdent).join(".");
    return { key: qualified, name: tnm, value: vlI >= 0 && row[vlI] != null ? String(row[vlI]) : "", removable: true };
  });
}

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
  // In DESCRIBE-degraded mode fp is always "" — a key may still be set, we just
  // can't see it. Treat the slot as possibly-occupied so the "Replace…" label
  // and the overwrite confirmation are never silently skipped (locking out the
  // old private key without warning). We deliberately don't lean on the
  // aggregate SHOW USERS HAS_RSA_PUBLIC_KEY here: it doesn't say which slot, so
  // trusting it could still skip the confirmation for a genuinely-set slot.
  const mayHaveKey = hasKey || degraded;

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
              {mayHaveKey ? "Replace…" : "Set…"}
            </Button>
            {/* Also offered when degraded: the role can't DESCRIBE USER, so a
                key may be set even though the fingerprint reads "unknown" —
                let the admin UNSET defensively rather than hiding the option. */}
            {mayHaveKey && (
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

// ─── Helper: access-policy row ───────────────────────────────────────────────

/**
 * One row per attachable policy kind (Authentication / Password / Session). The
 * policy is picked from a searchable dropdown of every policy of that kind in the
 * account (`ListAccount*Policies`); the option value is the quoted FQN passed to
 * SetUserPolicy. A FORCE toggle detaches any same-kind policy already attached.
 * Assignments aren't reported by SHOW USERS, so there's no current-value display.
 * Routes through SetUserPolicy / UnsetUserPolicy → users.BuildSet/UnsetPolicySQL.
 */
function PolicyRow({
  name, label, kind, options, onReload, search,
}: {
  name: string;
  label: string;
  kind: "AUTHENTICATION" | "PASSWORD" | "SESSION";
  options: { value: string; label: string }[];
  onReload: () => Promise<void>;
  search?: string;
}) {
  const [val, setVal]     = useState("");
  const [force, setForce] = useState(false);
  const [busy, setBusy]   = useState(false);
  if (search && !`${label} policy`.toLowerCase().includes(search.toLowerCase())) return null;

  const set = async () => {
    setBusy(true);
    try {
      await SetUserPolicy(name, kind, val, force);
      message.success(`${label} policy set on ${name}`);
      setVal("");
      await onReload();
    } catch (e) {
      message.error(friendlyError(e), 6);
    } finally {
      setBusy(false);
    }
  };

  const unset = () => {
    Modal.confirm({
      title: `Unset ${label.toLowerCase()} policy on ${name}?`,
      content: "This detaches the policy currently attached to the user, if any.",
      okText: "Unset",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await UnsetUserPolicy(name, kind);
          message.success(`${label} policy unset on ${name}`);
          await onReload();
        } catch (e) {
          message.error(friendlyError(e), 6);
        }
      },
    });
  };

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label} policy</td>
      <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
        <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
          <Select
            size="small"
            showSearch
            allowClear
            value={val || undefined}
            onChange={(v) => setVal(v ?? "")}
            placeholder={options.length ? "select policy…" : "no policies visible"}
            options={options}
            filterOption={(input, opt) => (opt?.label ?? "").toLowerCase().includes(input.toLowerCase())}
            style={{ minWidth: 220 }}
          />
          <Checkbox checked={force} onChange={(e) => setForce(e.target.checked)} style={{ fontSize: 12 }}>
            FORCE
          </Checkbox>
          <Button size="small" type="primary" icon={<CheckOutlined />} loading={busy} disabled={!val.trim()} onClick={set}>
            Set
          </Button>
          <Button size="small" danger onClick={unset}>Unset</Button>
        </div>
      </td>
    </tr>
  );
}

// ─── Helper: MFA method removal row ──────────────────────────────────────────

const MFA_METHODS = ["TOTP", "PASSKEY", "DUO"] as const;

/**
 * Removes one enrolled MFA method (`ALTER USER … REMOVE MFA METHOD <method>`) so
 * the user can re-enroll. Bypass windows are still set via MINS_TO_BYPASS_MFA in
 * the Security section; this is the destructive per-method removal.
 */
function MfaRemoveRow({ name, onReload, search }: { name: string; onReload: () => Promise<void>; search?: string }) {
  const [busy, setBusy] = useState<string>("");
  if (search && !"remove mfa method".includes(search.toLowerCase())) return null;

  const remove = (method: string) => {
    Modal.confirm({
      title: `Remove ${method} MFA method from ${name}?`,
      content: "The user loses this factor and must re-enroll it.",
      okText: "Remove",
      okButtonProps: { danger: true },
      onOk: async () => {
        setBusy(method);
        try {
          await RemoveUserMfaMethod(name, method);
          message.success(`Removed ${method} from ${name}`);
          await onReload();
        } catch (e) {
          message.error(friendlyError(e), 6);
        } finally {
          setBusy("");
        }
      },
    });
  };

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>Remove MFA method</td>
      <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
        <Space size={4} wrap>
          {MFA_METHODS.map((mth) => (
            <Button key={mth} size="small" danger loading={busy === mth} onClick={() => remove(mth)}>
              {mth}
            </Button>
          ))}
        </Space>
      </td>
    </tr>
  );
}

// ─── Helper: delegated-authorization rows ────────────────────────────────────

/**
 * Manages delegated authorizations that let a security integration act on the
 * user's behalf for a role. Add attaches ROLE→INTEGRATION; Remove detaches one
 * role (or, with the role left blank, every delegated authorization for the
 * integration). Routes through Add/RemoveUserDelegatedAuth →
 * users.Build{Add,Remove}DelegatedAuthSQL.
 */
function DelegatedAuthRows({ name, roleOptions, integrationOptions, onReload, search }: {
  name: string;
  roleOptions: string[];
  integrationOptions: string[];
  onReload: () => Promise<void>;
  search?: string;
}) {
  const [role, setRole]       = useState("");
  const [integration, setInt] = useState("");
  const [busy, setBusy]       = useState<string>("");
  if (search && !"delegated authorization role security integration".includes(search.toLowerCase())) return null;

  const add = async () => {
    setBusy("add");
    try {
      await AddUserDelegatedAuth(name, role, integration);
      message.success(`Delegated authorization added for ${name}`);
      await onReload();
    } catch (e) {
      message.error(friendlyError(e), 6);
    } finally {
      setBusy("");
    }
  };

  const remove = async () => {
    setBusy("remove");
    try {
      await RemoveUserDelegatedAuth(name, role, integration);
      message.success(`Delegated authorization removed for ${name}`);
      await onReload();
    } catch (e) {
      message.error(friendlyError(e), 6);
    } finally {
      setBusy("");
    }
  };

  const roleOpts = roleOptions.map((r) => ({ value: r, label: r }));
  const intOpts  = integrationOptions.map((i) => ({ value: i, label: i }));
  const filterByLabel = (input: string, opt?: { label?: string }) =>
    (opt?.label ?? "").toLowerCase().includes(input.toLowerCase());

  return (
    <>
      <tr>
        <td style={LABEL_TD}>Role</td>
        <td style={{ padding: "6px 0" }}>
          <Select
            size="small" showSearch allowClear value={role || undefined}
            onChange={(v) => setRole(v ?? "")} options={roleOpts} filterOption={filterByLabel}
            placeholder="role (leave empty to remove all)" style={{ minWidth: 260 }}
          />
        </td>
      </tr>
      <tr style={{ borderBottom: "1px solid var(--border)" }}>
        <td style={LABEL_TD}>Security integration</td>
        <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
          <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
            <Select
              size="small" showSearch allowClear value={integration || undefined}
              onChange={(v) => setInt(v ?? "")} options={intOpts} filterOption={filterByLabel}
              placeholder={intOpts.length ? "select integration…" : "no integrations visible"}
              style={{ minWidth: 200 }}
            />
            <Button size="small" type="primary" loading={busy === "add"} disabled={!role.trim() || !integration.trim()} onClick={add}>
              Add
            </Button>
            <Button size="small" danger loading={busy === "remove"} disabled={!integration.trim()} onClick={remove}>
              Remove
            </Button>
          </div>
        </td>
      </tr>
    </>
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
  // Picker lists for the policy / delegated-auth / tag dropdowns and the current
  // user-tag chips — all best-effort (empty on failure; the actions still work).
  const [roleNames, setRoleNames]       = useState<string[]>([]);
  const [integrationNames, setIntNames] = useState<string[]>([]);
  const [authPolicyOpts, setAuthPolicyOpts]       = useState<{ value: string; label: string }[]>([]);
  const [pwPolicyOpts, setPwPolicyOpts]           = useState<{ value: string; label: string }[]>([]);
  const [sessionPolicyOpts, setSessionPolicyOpts] = useState<{ value: string; label: string }[]>([]);
  const [tagNameOpts, setTagNameOpts]   = useState<NameOption[]>([]);
  const [userTags, setUserTags]         = useState<EditableTag[]>([]);

  // Reload the tags currently applied to the user (removable chips). Best-effort:
  // account-level TAG_REFERENCES needs a current database, so this may return
  // nothing — SET/UNSET still work regardless.
  const reloadTags = useCallback(async () => {
    try {
      setUserTags(userTagsToEditable(await GetUserTagReferences(name)));
    } catch {
      setUserTags([]);
    }
  }, [name]);

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
  useEffect(() => { reloadTags(); }, [reloadTags]);
  // Populate the picker lists once per open — all best-effort.
  useEffect(() => {
    ListRoles().then((r) => setRoleNames(r ?? [])).catch(() => setRoleNames([]));
    ListSecurityIntegrations()
      .then((i) => setIntNames((i ?? []).map((x) => x.name))).catch(() => setIntNames([]));
    ListAccountAuthenticationPolicies().then((r) => setAuthPolicyOpts(nameOptionsFromShow(r))).catch(() => setAuthPolicyOpts([]));
    ListAccountPasswordPolicies().then((r) => setPwPolicyOpts(nameOptionsFromShow(r))).catch(() => setPwPolicyOpts([]));
    ListAccountSessionPolicies().then((r) => setSessionPolicyOpts(nameOptionsFromShow(r))).catch(() => setSessionPolicyOpts([]));
    ListAccountTags().then((r) => setTagNameOpts(nameOptionsFromShow(r))).catch(() => setTagNameOpts([]));
  }, [name]);

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
            <MfaRemoveRow name={name} onReload={load} search={search} />
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

          <div style={SECTION_HEAD}><Space size={6}><SafetyOutlined />Access policies</Space></div>
          {!search && (
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 6, lineHeight: 1.5 }}>
              Attach an authentication, password, or session policy. Use <b>FORCE</b> to replace a
              same-kind policy already attached. Current assignments aren't shown here (SHOW USERS
              omits them) — inspect with <code>DESCRIBE USER</code> or <code>POLICY_REFERENCES</code>.
            </div>
          )}
          <table style={tableStyle}><tbody>
            <PolicyRow name={name} label="Authentication" kind="AUTHENTICATION" options={authPolicyOpts}    onReload={load} search={search} />
            <PolicyRow name={name} label="Password"       kind="PASSWORD"       options={pwPolicyOpts}      onReload={load} search={search} />
            <PolicyRow name={name} label="Session"        kind="SESSION"        options={sessionPolicyOpts} onReload={load} search={search} />
          </tbody></table>

          {(!search || "tags".includes(search.toLowerCase())) && (
            <>
              <div style={SECTION_HEAD}><Space size={6}><TagsOutlined />Tags</Space></div>
              {/* Shared object-store tag editor: current tags render as chips you
                  click to remove; the name field is a dropdown of account tags. */}
              <table style={tableStyle}><tbody>
                <TagsRow
                  tags={userTags}
                  nameOptions={tagNameOpts}
                  onSetTag={async (tagName, tagValue) => { await SetUserTags(name, [{ name: tagName, value: tagValue }]); await reloadTags(); }}
                  onUnsetTag={async (key) => { await UnsetUserTags(name, [key]); await reloadTags(); }}
                />
              </tbody></table>
            </>
          )}

          <div style={SECTION_HEAD}><Space size={6}><ApiOutlined />Delegated authorization</Space></div>
          {!search && (
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 6, lineHeight: 1.5 }}>
              Let a security integration act on the user's behalf for a role. Leave the role blank
              and click <b>Remove</b> to detach every delegated authorization for the integration.
            </div>
          )}
          <table style={tableStyle}><tbody>
            <DelegatedAuthRows name={name} roleOptions={roleNames} integrationOptions={integrationNames} onReload={load} search={search} />
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
          // hasKey || degraded: mirror KeyPairSlotRow — when the role can't
          // DESCRIBE USER the fingerprint is blank but a key may be set, so
          // treat the slot as occupied and force the overwrite confirmation.
          slotHasKey={
            val(keyModal === "RSA_PUBLIC_KEY" ? "RSA_PUBLIC_KEY_FP" : "RSA_PUBLIC_KEY_2_FP").trim() !== "" ||
            m["__DESCRIBE_DEGRADED__"] === "1"
          }
          onApplied={load}
          onClose={() => setKeyModal(null)}
        />
      )}
    </Modal>
  );
}
