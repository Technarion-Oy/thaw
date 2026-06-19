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

// Structured editors for the four nested "property-bag" parameters of an
// authentication policy (MFA_POLICY, PAT_POLICY, WORKLOAD_IDENTITY_POLICY,
// CLIENT_POLICY). Each editor renders the sub-properties as selects / numbers /
// toggles, but performs NO SQL serialization or DESCRIBE parsing of its own:
// pre-fill goes through `App.Parse*` and Save through `App.Build*Value`, so the
// `( … )` grammar lives entirely in the Go `authenticationpolicy` package. Each
// row's `onSet` receives the built value string; the parent issues
// `ALTER … SET <BAG> = <value>`.

import { useState, useEffect, type ReactNode } from "react";
import { Alert, AutoComplete, Button, InputNumber, Select, Space, Tooltip, Typography } from "antd";
import { EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import {
  BuildMFAPolicyValue, ParseMFAPolicy,
  BuildPATPolicyValue, ParsePATPolicy,
  BuildWorkloadIdentityPolicyValue, ParseWorkloadIdentityPolicy,
  BuildClientPolicyValue, ParseClientPolicy,
  ReconcileAllExclusiveList, AuthenticationPolicyClientDrivers,
  AuthenticationPolicyClientDriverVersions,
} from "../../../wailsjs/go/app/App";
import type { authenticationpolicy } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Local plain shapes for component state. We deliberately do NOT use the
// Wails-generated model classes here: those carry a `convertValues` method a
// plain object literal can't satisfy, and they type optional pointers as
// `field?: number` (undefined) whereas our editors use `number | null`. The
// structs are reconstructed as plain objects and cast (`cfg as any`) at the IPC
// boundary, where Wails marshals null → a nil Go pointer.
interface ClientEntry { driver: string; minimumVersion: string }

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "top", width: 220,
};

const opts = (vals: string[]) => vals.map((v) => ({ value: v, label: v }));

// Whether the DESCRIBE value carries real content (so a parse that yields an
// empty struct means the format wasn't understood, not that the bag is unset).
// An unset/empty bag renders as "", "()", or "{}" (Snowflake's empty-object
// form) — possibly with inner whitespace — so none of those count as content.
const rawHasContent = (raw: string) => {
  const t = raw.trim();
  return t !== "" && !/^\(\s*\)$/.test(t) && !/^\{\s*\}$/.test(t);
};

// ── Shared row chrome ────────────────────────────────────────────────────────
// Renders the label + either the read-only current value (with an Edit pencil)
// or the structured form (`children`) plus Save / Unset / Cancel. The per-bag
// components own the struct state and the build/parse calls; this only handles
// layout, the editing/saving/error lifecycle, and the buttons.

interface BagShellProps {
  label: string;
  rawValue: string;
  canSave: boolean;
  // True when the current value couldn't be parsed for pre-fill (raw was
  // non-empty but yielded an empty struct). Because Set replaces the WHOLE bag,
  // saving from a blank editor would wipe the unreadable config — so the fields
  // are hidden, a warning is shown, and Set is disabled (Unset still clears it
  // deliberately).
  parseFailed: boolean;
  onBeginEdit: () => Promise<void> | void;
  onSave: () => Promise<void>;
  onUnset: () => Promise<void>;
  children: ReactNode;
}

function BagShell({ label, rawValue, canSave, parseFailed, onBeginEdit, onSave, onUnset, children }: BagShellProps) {
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const begin = async () => {
    setError(null);
    await onBeginEdit();
    setEditing(true);
  };
  const run = async (fn: () => Promise<void>) => {
    setSaving(true);
    setError(null);
    try {
      await fn();
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
          <Space direction="vertical" size={6} style={{ width: "100%" }}>
            {parseFailed ? (
              <Alert
                type="warning"
                showIcon
                style={{ maxWidth: 420 }}
                message="Current value can't be shown"
                description="Thaw couldn't read the current setting, so editing here would replace the whole policy and wipe it. Edit it in a SQL worksheet, or use Unset to clear it deliberately."
              />
            ) : children}
            <Space>
              {!parseFailed && (
                <Tooltip title={canSave ? "Save" : "Set at least one property, or use Unset to clear"}>
                  <Button size="small" icon={<CheckOutlined />} type="primary" onClick={() => run(onSave)} loading={saving} disabled={!canSave}>Set</Button>
                </Tooltip>
              )}
              <Tooltip title="Reset to Snowflake default">
                <Button size="small" onClick={() => run(onUnset)} loading={saving}>Unset</Button>
              </Tooltip>
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setError(null); }} />
              </Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)", fontFamily: "var(--font-mono)", wordBreak: "break-word" }}>
              {rawHasContent(rawValue) ? rawValue : <Text type="secondary">(default)</Text>}
            </span>
            <Tooltip title="Edit">
              <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />} onClick={begin} style={{ color: "var(--text-muted)" }} />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

const FIELD_LABEL: React.CSSProperties = { fontSize: 11, color: "var(--text-muted)", display: "block", marginBottom: 2 };

interface RowProps {
  rawValue: string;
  onSet: (value: string) => Promise<void>;
  onUnset: () => Promise<void>;
}

// ── MFA_POLICY ───────────────────────────────────────────────────────────────

export function MFAPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [methods, setMethods] = useState<string[]>([]);
  const [enforce, setEnforce] = useState<string>("");
  const [parseFailed, setParseFailed] = useState(false);

  const begin = async () => {
    const p = await ParseMFAPolicy(rawValue);
    const m = p?.allowedMethods ?? [];
    const e = p?.enforceMfaOnExternalAuthentication ?? "";
    setMethods(m);
    setEnforce(e);
    setParseFailed(rawHasContent(rawValue) && m.length === 0 && e === "");
  };
  const canSave = methods.length > 0 || enforce !== "";
  const save = async () => {
    const cfg = { allowedMethods: methods, enforceMfaOnExternalAuthentication: enforce };
    await onSet(await BuildMFAPolicyValue(cfg as any));
  };

  return (
    <BagShell label="MFA policy" rawValue={rawValue} canSave={canSave} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <div style={{ width: "100%" }}>
        <Text style={FIELD_LABEL}>Allowed methods</Text>
        <Select mode="multiple" size="small" value={methods}
          onChange={async (v) => setMethods((await ReconcileAllExclusiveList(v)) ?? [])}
          placeholder="default (ALL)"
          options={opts(["ALL", "PASSKEY", "TOTP", "OTP", "DUO"])} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Enforce MFA on external authentication</Text>
        <Select allowClear size="small" value={enforce || undefined} onChange={(v) => setEnforce(v ?? "")}
          placeholder="default (NONE)" options={opts(["ALL", "NONE"])} style={{ width: 200 }} />
      </div>
    </BagShell>
  );
}

// ── PAT_POLICY ───────────────────────────────────────────────────────────────

export function PATPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [defExpiry, setDefExpiry] = useState<number | null>(null);
  const [maxExpiry, setMaxExpiry] = useState<number | null>(null);
  const [netEval, setNetEval] = useState<string>("");
  const [requireRole, setRequireRole] = useState<boolean | null>(null);
  const [parseFailed, setParseFailed] = useState(false);

  const begin = async () => {
    const p = await ParsePATPolicy(rawValue);
    const de = p?.defaultExpiryInDays ?? null;
    const me = p?.maxExpiryInDays ?? null;
    const ne = p?.networkPolicyEvaluation ?? "";
    const rr = p?.requireRoleRestrictionForServiceUsers ?? null;
    setDefExpiry(de);
    setMaxExpiry(me);
    setNetEval(ne);
    setRequireRole(rr);
    setParseFailed(rawHasContent(rawValue) && de === null && me === null && ne === "" && rr === null);
  };
  const canSave = defExpiry !== null || maxExpiry !== null || netEval !== "" || requireRole !== null;
  const save = async () => {
    const cfg = {
      defaultExpiryInDays: defExpiry, maxExpiryInDays: maxExpiry,
      networkPolicyEvaluation: netEval, requireRoleRestrictionForServiceUsers: requireRole,
    };
    await onSet(await BuildPATPolicyValue(cfg as any));
  };

  return (
    <BagShell label="PAT policy" rawValue={rawValue} canSave={canSave} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <Space wrap>
        <div>
          <Text style={FIELD_LABEL}>Default expiry (days)</Text>
          <InputNumber size="small" min={1} max={365} precision={0} value={defExpiry} onChange={(v) => setDefExpiry(v ?? null)} placeholder="15" style={{ width: 140 }} />
        </div>
        <div>
          <Text style={FIELD_LABEL}>Max expiry (days)</Text>
          <InputNumber size="small" min={1} max={365} precision={0} value={maxExpiry} onChange={(v) => setMaxExpiry(v ?? null)} placeholder="365" style={{ width: 140 }} />
        </div>
      </Space>
      <div>
        <Text style={FIELD_LABEL}>Network policy evaluation</Text>
        <Select allowClear size="small" value={netEval || undefined} onChange={(v) => setNetEval(v ?? "")} placeholder="default (ENFORCED_REQUIRED)"
          options={opts(["ENFORCED_REQUIRED", "ENFORCED_NOT_REQUIRED", "NOT_ENFORCED"])} style={{ width: 280 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Require role restriction for service users</Text>
        <Select allowClear size="small"
          value={requireRole === null ? undefined : requireRole ? "TRUE" : "FALSE"}
          onChange={(v) => setRequireRole(v === undefined ? null : v === "TRUE")}
          placeholder="default (TRUE)" options={opts(["TRUE", "FALSE"])} style={{ width: 200 }} />
      </div>
    </BagShell>
  );
}

// ── WORKLOAD_IDENTITY_POLICY ─────────────────────────────────────────────────

export function WorkloadIdentityPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [providers, setProviders] = useState<string[]>([]);
  const [awsAccounts, setAwsAccounts] = useState<string[]>([]);
  const [azureIssuers, setAzureIssuers] = useState<string[]>([]);
  const [oidcIssuers, setOidcIssuers] = useState<string[]>([]);
  const [parseFailed, setParseFailed] = useState(false);

  const begin = async () => {
    const p = await ParseWorkloadIdentityPolicy(rawValue);
    const pr = p?.allowedProviders ?? [];
    const aws = p?.allowedAwsAccounts ?? [];
    const az = p?.allowedAzureIssuers ?? [];
    const oidc = p?.allowedOidcIssuers ?? [];
    setProviders(pr);
    setAwsAccounts(aws);
    setAzureIssuers(az);
    setOidcIssuers(oidc);
    setParseFailed(rawHasContent(rawValue) && pr.length === 0 && aws.length === 0 && az.length === 0 && oidc.length === 0);
  };
  const canSave = providers.length > 0 || awsAccounts.length > 0 || azureIssuers.length > 0 || oidcIssuers.length > 0;
  const save = async () => {
    const cfg = {
      allowedProviders: providers, allowedAwsAccounts: awsAccounts,
      allowedAzureIssuers: azureIssuers, allowedOidcIssuers: oidcIssuers,
    };
    await onSet(await BuildWorkloadIdentityPolicyValue(cfg as any));
  };

  return (
    <BagShell label="Workload identity policy" rawValue={rawValue} canSave={canSave} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <div>
        <Text style={FIELD_LABEL}>Allowed providers</Text>
        <Select mode="multiple" size="small" value={providers}
          onChange={async (v) => setProviders((await ReconcileAllExclusiveList(v)) ?? [])}
          placeholder="ALL / AWS / AZURE / GCP / OIDC"
          options={opts(["ALL", "AWS", "AZURE", "GCP", "OIDC"])} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Allowed AWS accounts (12-digit IDs)</Text>
        <Select mode="tags" size="small" value={awsAccounts} onChange={setAwsAccounts} placeholder="123456789012" tokenSeparators={[","]} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Allowed Azure issuers</Text>
        <Select mode="tags" size="small" value={azureIssuers} onChange={setAzureIssuers} placeholder="authority URLs" tokenSeparators={[","]} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Allowed OIDC issuers</Text>
        <Select mode="tags" size="small" value={oidcIssuers} onChange={setOidcIssuers} placeholder="https://issuer…" tokenSeparators={[","]} style={{ width: 360 }} />
      </div>
    </BagShell>
  );
}

// ── CLIENT_POLICY ────────────────────────────────────────────────────────────

export function ClientPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [entries, setEntries] = useState<ClientEntry[]>([]);
  const [parseFailed, setParseFailed] = useState(false);
  // The selectable drivers come from the backend (the version-governed subset of
  // the shared snowflake.ClientDrivers catalog) — static data fetched once.
  const [driverOptions, setDriverOptions] = useState<string[]>([]);
  // Per-driver version hints (minimum supported / recommended) from
  // SYSTEM$CLIENT_VERSION_INFO(), keyed by driver token, used to suggest versions.
  const [versions, setVersions] = useState<Record<string, authenticationpolicy.DriverVersionHint>>({});
  useEffect(() => {
    AuthenticationPolicyClientDrivers().then((d) => setDriverOptions(d ?? []));
  }, []);

  const begin = async () => {
    const p = await ParseClientPolicy(rawValue);
    const es = (p?.entries ?? []).map((e) => ({ driver: e.driver, minimumVersion: e.minimumVersion }));
    setEntries(es);
    setParseFailed(rawHasContent(rawValue) && es.length === 0);
    // Version hints are best-effort — a failure (e.g. no connection) just means
    // the user types the version manually, as before.
    try {
      const hints = await AuthenticationPolicyClientDriverVersions();
      const map: Record<string, authenticationpolicy.DriverVersionHint> = {};
      (hints ?? []).forEach((h) => { map[h.driver] = h; });
      setVersions(map);
    } catch {
      setVersions({});
    }
  };
  const valid = entries.filter((e) => e.driver?.trim() && e.minimumVersion?.trim());
  // A half-filled row (driver xor version) would be silently dropped by the
  // builder, which reads as "it saved everything" — so block Save until every
  // started row is complete (or removed), rather than dropping it on save.
  const hasPartial = entries.some((e) => {
    const hasDriver = !!e.driver?.trim();
    const hasVersion = !!e.minimumVersion?.trim();
    return (hasDriver || hasVersion) && !(hasDriver && hasVersion);
  });
  // Two rows for the same driver would build a bag with a repeated key — block
  // Save and warn rather than silently dropping one (the backend dedupes too).
  const drivers = valid.map((e) => e.driver.trim().toUpperCase());
  const hasDuplicate = new Set(drivers).size !== drivers.length;
  const canSave = valid.length > 0 && !hasPartial && !hasDuplicate;
  const save = async () => {
    const cfg = { entries };
    await onSet(await BuildClientPolicyValue(cfg as any));
  };

  const update = (i: number, patch: Partial<ClientEntry>) =>
    setEntries((prev) => prev.map((e, idx) => (idx === i ? { ...e, ...patch } : e)));
  const remove = (i: number) => setEntries((prev) => prev.filter((_, idx) => idx !== i));
  const add = () => setEntries((prev) => [...prev, { driver: "", minimumVersion: "" }]);

  return (
    <BagShell label="Client policy" rawValue={rawValue} canSave={canSave} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <Text style={FIELD_LABEL}>Minimum driver/client versions</Text>
      <Space direction="vertical" size={4} style={{ width: "100%" }}>
        {entries.map((e, i) => {
          const hint = versions[(e.driver ?? "").toUpperCase()];
          // Recommended first, then minimum supported; drop blanks/duplicates.
          const verOptions = hint
            ? [
                hint.recommended && { value: hint.recommended, label: `${hint.recommended} · recommended` },
                hint.minimumSupported && hint.minimumSupported !== hint.recommended
                  && { value: hint.minimumSupported, label: `${hint.minimumSupported} · minimum supported` },
              ].filter(Boolean) as { value: string; label: string }[]
            : [];
          return (
            <Space key={i}>
              <Select size="small" showSearch value={e.driver || undefined} onChange={(v) => update(i, { driver: v })}
                placeholder="driver" options={opts(driverOptions)} style={{ width: 240 }} />
              <Tooltip title={hint ? `Supported: ${hint.minimumSupported || "—"} … recommended ${hint.recommended || "—"}` : ""}>
                <AutoComplete size="small" value={e.minimumVersion} options={verOptions} filterOption={false}
                  onChange={(val) => update(i, { minimumVersion: val })}
                  placeholder={hint?.recommended || "3.13.0"} style={{ width: 170 }} />
              </Tooltip>
              <Tooltip title="Remove"><Button size="small" type="text" icon={<DeleteOutlined />} onClick={() => remove(i)} /></Tooltip>
            </Space>
          );
        })}
        <Button size="small" icon={<PlusOutlined />} onClick={add}>Add driver</Button>
        {hasPartial && (
          <Text type="warning" style={{ fontSize: 11 }}>
            Every row needs both a driver and a version — complete or remove the incomplete row to save.
          </Text>
        )}
        {hasDuplicate && (
          <Text type="warning" style={{ fontSize: 11 }}>
            Each driver can appear only once — remove the duplicate row to save.
          </Text>
        )}
      </Space>
    </BagShell>
  );
}
