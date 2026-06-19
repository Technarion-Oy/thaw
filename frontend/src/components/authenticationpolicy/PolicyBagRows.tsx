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
// CLIENT_POLICY). The editing UI for each bag is a controlled `<Bag>Fields`
// component (value + onChange) reused by BOTH the Create modal and the Properties
// modal. The `<Bag>Row` wrappers add the Properties-only Set/Unset lifecycle
// (parse the current DESCRIBE value on Edit, serialize via `App.Build*Value` on
// Save). No SQL serialization or DESCRIBE parsing happens here — the `( … )`
// grammar lives entirely in the Go `authenticationpolicy` package.

import { useState, useEffect, useRef, useCallback, type ReactNode } from "react";
import { Alert, AutoComplete, Button, InputNumber, Select, Space, Tooltip, Typography } from "antd";
import { EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import {
  BuildMFAPolicyValue, ParseMFAPolicy,
  BuildPATPolicyValue, ParsePATPolicy,
  BuildWorkloadIdentityPolicyValue, ParseWorkloadIdentityPolicy,
  BuildClientPolicyValue, ParseClientPolicy,
  ReconcileAllExclusiveList, AuthenticationPolicyClientDrivers,
  AuthenticationPolicyClientDriverVersions, AuthenticationPolicyBagOptions,
} from "../../../wailsjs/go/app/App";
import type { authenticationpolicy } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Local plain shapes for component state. We deliberately do NOT use the
// Wails-generated model classes here: those carry a `convertValues` method a
// plain object literal can't satisfy, and they type optional pointers as
// `field?: number` (undefined) whereas our editors use `number | null`. The
// structs are reconstructed as plain objects and cast (`value as any`) at the IPC
// boundary, where Wails marshals null → a nil Go pointer.
interface ClientEntry { driver: string; minimumVersion: string }

// Controlled value shapes — plain objects matching each bag's Go struct json
// tags, so they pass straight to `Build<Bag>Value` and into the create config.
export interface MFAPolicyValue { allowedMethods: string[]; enforceMfaOnExternalAuthentication: string }
export interface PATPolicyValue {
  defaultExpiryInDays: number | null;
  maxExpiryInDays: number | null;
  networkPolicyEvaluation: string;
  requireRoleRestrictionForServiceUsers: boolean | null;
}
export interface WorkloadIdentityPolicyValue {
  allowedProviders: string[];
  allowedAwsAccounts: string[];
  allowedAzureIssuers: string[];
  allowedOidcIssuers: string[];
}
export interface ClientPolicyValue { entries: ClientEntry[] }

export const emptyMFAPolicy = (): MFAPolicyValue => ({ allowedMethods: [], enforceMfaOnExternalAuthentication: "" });
export const emptyPATPolicy = (): PATPolicyValue => ({
  defaultExpiryInDays: null, maxExpiryInDays: null, networkPolicyEvaluation: "", requireRoleRestrictionForServiceUsers: null,
});
export const emptyWorkloadIdentityPolicy = (): WorkloadIdentityPolicyValue => ({
  allowedProviders: [], allowedAwsAccounts: [], allowedAzureIssuers: [], allowedOidcIssuers: [],
});
export const emptyClientPolicy = (): ClientPolicyValue => ({ entries: [] });

// Whether each bag carries any set sub-property (used to gate Set in the
// Properties modal — a Set of an empty bag would serialize to the invalid `()`).
export const mfaPolicyHasContent = (v: MFAPolicyValue) => v.allowedMethods.length > 0 || v.enforceMfaOnExternalAuthentication !== "";
export const patPolicyHasContent = (v: PATPolicyValue) =>
  v.defaultExpiryInDays !== null || v.maxExpiryInDays !== null || v.networkPolicyEvaluation !== "" || v.requireRoleRestrictionForServiceUsers !== null;
export const workloadIdentityPolicyHasContent = (v: WorkloadIdentityPolicyValue) =>
  v.allowedProviders.length > 0 || v.allowedAwsAccounts.length > 0 || v.allowedAzureIssuers.length > 0 || v.allowedOidcIssuers.length > 0;

// clientPolicyError reports a blocking problem with the CLIENT_POLICY entries (a
// half-filled row, or a repeated driver) as a user message, or null when valid.
// Shared by both modals so the same rules gate Save and Create-submit.
export function clientPolicyError(value: ClientPolicyValue): string | null {
  const hasPartial = value.entries.some((e) => {
    const d = !!e.driver?.trim();
    const v = !!e.minimumVersion?.trim();
    return (d || v) && !(d && v);
  });
  if (hasPartial) return "Every row needs both a driver and a version — complete or remove the incomplete row.";
  const drivers = value.entries.filter((e) => e.driver?.trim()).map((e) => e.driver.trim().toUpperCase());
  if (new Set(drivers).size !== drivers.length) return "Each driver can appear only once — remove the duplicate row.";
  return null;
}

// Count of fully-filled CLIENT_POLICY rows (a Set needs at least one).
export const clientPolicyValidCount = (value: ClientPolicyValue) =>
  value.entries.filter((e) => e.driver?.trim() && e.minimumVersion?.trim()).length;

const opts = (vals: string[]) => vals.map((v) => ({ value: v, label: v }));

// The bag enum options (allowed methods, providers, network-policy evaluation, …)
// are static grammar data sourced from the Go backend so the editors aren't a
// second copy of the grammar. Fetched once per session via a module-level cache
// shared across every field editor; the cache is cleared on failure so a later
// mount retries.
let bagOptionsPromise: Promise<authenticationpolicy.BagParamOptions> | null = null;
function useBagOptions(): authenticationpolicy.BagParamOptions | null {
  const [o, setO] = useState<authenticationpolicy.BagParamOptions | null>(null);
  useEffect(() => {
    if (!bagOptionsPromise) bagOptionsPromise = AuthenticationPolicyBagOptions();
    bagOptionsPromise.then((r) => setO(r ?? null)).catch(() => { bagOptionsPromise = null; setO(null); });
  }, []);
  return o;
}

// reconcileAllExclusiveLocal mirrors snowflake.ReconcileAllExclusive: ALL is
// mutually exclusive with named items, keeping whichever kind was chosen last
// (the selection arrives in selection order). Used only as a fallback when the
// backend IPC is unavailable so an invalid ('ALL', X) is never left committed.
function reconcileAllExclusiveLocal(v: string[]): string[] {
  const isAll = (s: string) => s.trim().toUpperCase() === "ALL";
  if (v.length <= 1 || !v.some(isAll)) return v;
  return isAll(v[v.length - 1]) ? ["ALL"] : v.filter((s) => !isAll(s));
}

// useReconciledSelection returns a multi-select onChange handler that commits the
// new selection *immediately* (so a rapid second pick is computed from the fresh
// value and never drops a token), then collapses ALL-vs-specific exclusivity in
// the backend and applies only the most recent result (a generation guard, so
// out-of-order IPC resolutions can't revert to a stale list). `commit` is read
// through a ref, so the async update always targets the latest surrounding state
// (no clobbering a sibling field that changed during the round-trip). If the IPC
// rejects (e.g. not connected), it falls back to the local reconcile so an
// invalid ('ALL', X) is never left committed.
export function useReconciledSelection(commit: (list: string[]) => void) {
  const gen = useRef(0);
  const commitRef = useRef(commit);
  commitRef.current = commit;
  return useCallback((v: string[]) => {
    const g = ++gen.current;
    commitRef.current(v);
    ReconcileAllExclusiveList(v)
      .then((r) => { if (gen.current === g) commitRef.current(r ?? v); })
      .catch(() => { if (gen.current === g) commitRef.current(reconcileAllExclusiveLocal(v)); });
  }, []);
}

const FIELD_LABEL: React.CSSProperties = { fontSize: 11, color: "var(--text-muted)", display: "block", marginBottom: 2 };

// Whether the DESCRIBE value carries real content (so a parse that yields an
// empty struct means the format wasn't understood, not that the bag is unset).
// An unset/empty bag renders as "", "()", or "{}" (Snowflake's empty-object
// form) — possibly with inner whitespace — so none of those count as content.
const rawHasContent = (raw: string) => {
  const t = raw.trim();
  return t !== "" && !/^\(\s*\)$/.test(t) && !/^\{\s*\}$/.test(t);
};

// ── Controlled field editors (shared by Create + Properties) ─────────────────

// BAG_FIELDS wraps each editor's sub-fields so they carry their own vertical
// spacing — the editors are used both standalone (Create) and inside BagShell,
// neither of which spaces the inner fields for them.
const BAG_FIELDS: React.CSSProperties = { display: "flex", width: "100%" };

export function MFAPolicyFields({ value, onChange }: { value: MFAPolicyValue; onChange: (v: MFAPolicyValue) => void }) {
  const onMethods = useReconciledSelection((v) => onChange({ ...value, allowedMethods: v }));
  const bo = useBagOptions();
  return (
    <Space direction="vertical" size={6} style={BAG_FIELDS}>
      <div style={{ width: "100%" }}>
        <Text style={FIELD_LABEL}>Allowed methods</Text>
        <Select mode="multiple" size="small" value={value.allowedMethods}
          onChange={onMethods}
          placeholder="default (ALL)"
          options={opts(bo?.mfaAllowedMethods ?? [])} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Enforce MFA on external authentication</Text>
        <Select allowClear size="small" value={value.enforceMfaOnExternalAuthentication || undefined}
          onChange={(v) => onChange({ ...value, enforceMfaOnExternalAuthentication: v ?? "" })}
          placeholder="default (NONE)" options={opts(bo?.mfaEnforceExternal ?? [])} style={{ width: 200 }} />
      </div>
    </Space>
  );
}

export function PATPolicyFields({ value, onChange }: { value: PATPolicyValue; onChange: (v: PATPolicyValue) => void }) {
  const bo = useBagOptions();
  return (
    <Space direction="vertical" size={6} style={BAG_FIELDS}>
      <Space wrap>
        <div>
          <Text style={FIELD_LABEL}>Default expiry (days)</Text>
          <InputNumber size="small" min={1} max={365} precision={0} value={value.defaultExpiryInDays}
            onChange={(v) => onChange({ ...value, defaultExpiryInDays: v ?? null })} placeholder="15" style={{ width: 140 }} />
        </div>
        <div>
          <Text style={FIELD_LABEL}>Max expiry (days)</Text>
          <InputNumber size="small" min={1} max={365} precision={0} value={value.maxExpiryInDays}
            onChange={(v) => onChange({ ...value, maxExpiryInDays: v ?? null })} placeholder="365" style={{ width: 140 }} />
        </div>
      </Space>
      <div>
        <Text style={FIELD_LABEL}>Network policy evaluation</Text>
        <Select allowClear size="small" value={value.networkPolicyEvaluation || undefined}
          onChange={(v) => onChange({ ...value, networkPolicyEvaluation: v ?? "" })} placeholder="default (ENFORCED_REQUIRED)"
          options={opts(bo?.patNetworkPolicyEvaluation ?? [])} style={{ width: 280 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Require role restriction for service users</Text>
        <Select allowClear size="small"
          value={value.requireRoleRestrictionForServiceUsers === null ? undefined : value.requireRoleRestrictionForServiceUsers ? "TRUE" : "FALSE"}
          onChange={(v) => onChange({ ...value, requireRoleRestrictionForServiceUsers: v === undefined ? null : v === "TRUE" })}
          placeholder="default (TRUE)" options={opts(bo?.patRequireRoleRestriction ?? [])} style={{ width: 200 }} />
      </div>
    </Space>
  );
}

export function WorkloadIdentityPolicyFields({ value, onChange }: { value: WorkloadIdentityPolicyValue; onChange: (v: WorkloadIdentityPolicyValue) => void }) {
  const onProviders = useReconciledSelection((v) => onChange({ ...value, allowedProviders: v }));
  const bo = useBagOptions();
  return (
    <Space direction="vertical" size={6} style={BAG_FIELDS}>
      <div>
        <Text style={FIELD_LABEL}>Allowed providers</Text>
        <Select mode="multiple" size="small" value={value.allowedProviders}
          onChange={onProviders}
          placeholder="ALL / AWS / AZURE / GCP / OIDC"
          options={opts(bo?.workloadAllowedProviders ?? [])} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Allowed AWS accounts (12-digit IDs)</Text>
        <Select mode="tags" size="small" value={value.allowedAwsAccounts}
          onChange={(v) => onChange({ ...value, allowedAwsAccounts: v })} placeholder="123456789012" tokenSeparators={[","]} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Allowed Azure issuers</Text>
        <Select mode="tags" size="small" value={value.allowedAzureIssuers}
          onChange={(v) => onChange({ ...value, allowedAzureIssuers: v })} placeholder="authority URLs" tokenSeparators={[","]} style={{ width: 360 }} />
      </div>
      <div>
        <Text style={FIELD_LABEL}>Allowed OIDC issuers</Text>
        <Select mode="tags" size="small" value={value.allowedOidcIssuers}
          onChange={(v) => onChange({ ...value, allowedOidcIssuers: v })} placeholder="https://issuer…" tokenSeparators={[","]} style={{ width: 360 }} />
      </div>
    </Space>
  );
}

export function ClientPolicyFields({ value, onChange }: { value: ClientPolicyValue; onChange: (v: ClientPolicyValue) => void }) {
  // The selectable drivers come from the backend (the version-governed subset of
  // the shared snowflake.ClientDrivers catalog), and the per-driver version hints
  // from SYSTEM$CLIENT_VERSION_INFO(). Both are session-static — fetched once.
  // Best-effort: a failure (e.g. no connection) just means manual entry.
  const [driverOptions, setDriverOptions] = useState<string[]>([]);
  const [versions, setVersions] = useState<Record<string, authenticationpolicy.DriverVersionHint>>({});
  useEffect(() => {
    AuthenticationPolicyClientDrivers().then((d) => setDriverOptions(d ?? []));
    AuthenticationPolicyClientDriverVersions()
      .then((hints) => {
        const map: Record<string, authenticationpolicy.DriverVersionHint> = {};
        (hints ?? []).forEach((h) => { map[h.driver] = h; });
        setVersions(map);
      })
      .catch(() => setVersions({}));
  }, []);

  const entries = value.entries;
  const update = (i: number, patch: Partial<ClientEntry>) =>
    onChange({ entries: entries.map((e, idx) => (idx === i ? { ...e, ...patch } : e)) });
  const remove = (i: number) => onChange({ entries: entries.filter((_, idx) => idx !== i) });
  const add = () => onChange({ entries: [...entries, { driver: "", minimumVersion: "" }] });
  const error = clientPolicyError(value);

  return (
    <Space direction="vertical" size={6} style={BAG_FIELDS}>
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
        {error && <Text type="warning" style={{ fontSize: 11 }}>{error}</Text>}
      </Space>
    </Space>
  );
}

// ── Shared row chrome (Properties modal Set/Unset lifecycle) ─────────────────
// Renders the label + either the read-only current value (with an Edit pencil)
// or the structured form (`children`) plus Save / Unset / Cancel. The per-bag
// components own the struct state and the build/parse calls; this only handles
// layout, the editing/saving/error lifecycle, and the buttons.

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "top", width: 220,
};

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

interface RowProps {
  rawValue: string;
  onSet: (value: string) => Promise<void>;
  onUnset: () => Promise<void>;
}

export function MFAPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [value, setValue] = useState<MFAPolicyValue>(emptyMFAPolicy());
  const [parseFailed, setParseFailed] = useState(false);

  const begin = async () => {
    const p = await ParseMFAPolicy(rawValue);
    const v: MFAPolicyValue = { allowedMethods: p?.allowedMethods ?? [], enforceMfaOnExternalAuthentication: p?.enforceMfaOnExternalAuthentication ?? "" };
    setValue(v);
    setParseFailed(rawHasContent(rawValue) && !mfaPolicyHasContent(v));
  };
  const save = async () => { await onSet(await BuildMFAPolicyValue(value as any)); };

  return (
    <BagShell label="MFA policy" rawValue={rawValue} canSave={mfaPolicyHasContent(value)} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <MFAPolicyFields value={value} onChange={setValue} />
    </BagShell>
  );
}

export function PATPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [value, setValue] = useState<PATPolicyValue>(emptyPATPolicy());
  const [parseFailed, setParseFailed] = useState(false);

  const begin = async () => {
    const p = await ParsePATPolicy(rawValue);
    const v: PATPolicyValue = {
      defaultExpiryInDays: p?.defaultExpiryInDays ?? null,
      maxExpiryInDays: p?.maxExpiryInDays ?? null,
      networkPolicyEvaluation: p?.networkPolicyEvaluation ?? "",
      requireRoleRestrictionForServiceUsers: p?.requireRoleRestrictionForServiceUsers ?? null,
    };
    setValue(v);
    setParseFailed(rawHasContent(rawValue) && !patPolicyHasContent(v));
  };
  const save = async () => { await onSet(await BuildPATPolicyValue(value as any)); };

  return (
    <BagShell label="PAT policy" rawValue={rawValue} canSave={patPolicyHasContent(value)} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <PATPolicyFields value={value} onChange={setValue} />
    </BagShell>
  );
}

export function WorkloadIdentityPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [value, setValue] = useState<WorkloadIdentityPolicyValue>(emptyWorkloadIdentityPolicy());
  const [parseFailed, setParseFailed] = useState(false);

  const begin = async () => {
    const p = await ParseWorkloadIdentityPolicy(rawValue);
    const v: WorkloadIdentityPolicyValue = {
      allowedProviders: p?.allowedProviders ?? [],
      allowedAwsAccounts: p?.allowedAwsAccounts ?? [],
      allowedAzureIssuers: p?.allowedAzureIssuers ?? [],
      allowedOidcIssuers: p?.allowedOidcIssuers ?? [],
    };
    setValue(v);
    setParseFailed(rawHasContent(rawValue) && !workloadIdentityPolicyHasContent(v));
  };
  const save = async () => { await onSet(await BuildWorkloadIdentityPolicyValue(value as any)); };

  return (
    <BagShell label="Workload identity policy" rawValue={rawValue} canSave={workloadIdentityPolicyHasContent(value)} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <WorkloadIdentityPolicyFields value={value} onChange={setValue} />
    </BagShell>
  );
}

export function ClientPolicyRow({ rawValue, onSet, onUnset }: RowProps) {
  const [value, setValue] = useState<ClientPolicyValue>(emptyClientPolicy());
  const [parseFailed, setParseFailed] = useState(false);

  const begin = async () => {
    const p = await ParseClientPolicy(rawValue);
    const v: ClientPolicyValue = { entries: (p?.entries ?? []).map((e) => ({ driver: e.driver, minimumVersion: e.minimumVersion })) };
    setValue(v);
    setParseFailed(rawHasContent(rawValue) && v.entries.length === 0);
  };
  // A half-filled or duplicate row would corrupt the bag, so block Save (the
  // editor surfaces the reason); a Set also needs at least one complete row.
  const canSave = clientPolicyValidCount(value) > 0 && clientPolicyError(value) === null;
  const save = async () => { await onSet(await BuildClientPolicyValue(value as any)); };

  return (
    <BagShell label="Client policy" rawValue={rawValue} canSave={canSave} parseFailed={parseFailed} onBeginEdit={begin} onSave={save} onUnset={onUnset}>
      <ClientPolicyFields value={value} onChange={setValue} />
    </BagShell>
  );
}
