// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import YAML from "yaml";

/**
 * Pure helpers behind the SPCS specification forms (issue #958): YAML ⇄ plain
 * object, immutable path setters, and the cross-field checks per spec kind.
 * The forms edit the parsed document directly, so keys they don't render
 * survive a Form round trip untouched.
 */

export type SpecKind = "service" | "job" | "inference" | "gateway";
export type Path = (string | number)[];
// The parsed YAML document — untyped by design, the schemas live in specSchemas/.
export type Doc = any; // eslint-disable-line @typescript-eslint/no-explicit-any

/** Parse spec text into an editable object, or say why the form can't take it. */
export function parseSpec(text: string): { doc: Doc } | { error: string } {
  if (/\{\{/.test(text)) return { error: "Template variables ({{ … }}) can't be edited as a form." };
  try {
    const doc = YAML.parse(text);
    if (doc == null) return { doc: {} };
    if (typeof doc !== "object" || Array.isArray(doc)) return { error: "The specification must be a YAML mapping." };
    return { doc };
  } catch (e) {
    return { error: `Not valid YAML: ${(e as Error).message.split("\n")[0]}` };
  }
}

export const dumpSpec = (doc: Doc): string =>
  doc && Object.keys(doc).length ? YAML.stringify(doc, { lineWidth: 0 }) : "";

export const getIn = (doc: Doc, path: Path): Doc =>
  path.reduce((o, k) => (o == null ? undefined : o[k]), doc);

const isEmpty = (v: unknown) =>
  v === undefined || v === null || v === "" ||
  (Array.isArray(v) ? v.length === 0 : typeof v === "object" && Object.keys(v as object).length === 0);

/**
 * Immutable set. An empty value (undefined, "", [], {}) deletes the key and
 * prunes mapping parents left empty — but never removes an array element, so
 * a freshly added `{}` list item stays put.
 */
export function setIn(doc: Doc, path: Path, value: unknown): Doc {
  if (!path.length) return value;
  const [k, ...rest] = path;
  const base: Doc = Array.isArray(doc) ? [...doc] : doc && typeof doc === "object" ? { ...doc } : typeof k === "number" ? [] : {};
  const child = setIn(base[k], rest, value);
  if (isEmpty(child) && !Array.isArray(base)) delete base[k];
  else base[k] = child;
  return base;
}

// ── Cross-field checks ───────────────────────────────────────────────────────

const DNS_NAME = /^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$/;
const arr = (v: unknown): Doc[] => (Array.isArray(v) ? v : []);
const named = (kind: string, i: number, x: Doc) => `${kind} ${x?.name ? `"${x.name}"` : i + 1}`;

export function validateServiceSpec(doc: Doc, job = false): string[] {
  const p: string[] = [];
  const spec = doc?.spec ?? {};
  const containers = arr(spec.containers);
  const volumes = arr(spec.volumes);
  const endpoints = arr(spec.endpoints);
  const volNames = new Set(volumes.map((v) => v?.name));
  const epNames = new Set(endpoints.map((e) => e?.name));
  const nameCheck = (kind: string, i: number, x: Doc) => {
    if (!x?.name) p.push(`${kind} ${i + 1}: name is required.`);
    else if (!DNS_NAME.test(x.name)) p.push(`${named(kind, i, x)}: name must be lowercase letters, digits and hyphens, start with a letter, end alphanumeric, ≤ 63 chars.`);
  };

  if (!containers.length) p.push("At least one container is required.");
  containers.forEach((c, i) => {
    nameCheck("Container", i, c);
    const n = named("Container", i, c);
    if (!c?.image) p.push(`${n}: image is required.`);
    if (c?.resources?.requests?.["nvidia.com/gpu"] != null && c?.resources?.limits?.["nvidia.com/gpu"] == null)
      p.push(`${n}: a GPU limit is required when a GPU request is set.`);
    arr(c?.volumeMounts).forEach((m) => {
      if (!volNames.has(m?.name)) p.push(`${n}: volume mount "${m?.name ?? ""}" doesn't match any volume.`);
      if (!m?.mountPath) p.push(`${n}: volume mount "${m?.name ?? ""}" needs a mount path.`);
    });
    if (c?.mountSessionToken === false && c?.sessionTokenConfig)
      p.push(`${n}: sessionTokenConfig requires mountSessionToken.`);
    arr(c?.secrets).forEach((s, j) => {
      const sn = `${n} secret ${j + 1}`;
      const ref = s?.snowflakeSecret ?? {}; // a bare string is the objectName shorthand
      if (typeof ref !== "string" && !ref.objectName === !ref.objectReference) p.push(`${sn}: set exactly one of object name / object reference.`);
      if (!s?.envVarName === !s?.directoryPath) p.push(`${sn}: set either an env var or a directory path.`);
      if (s?.envVarName && !s?.secretKeyRef) p.push(`${sn}: an env var needs a secret key (username, password or secret_string).`);
    });
    if (job && c?.readinessProbe) p.push(`${n}: jobs don't support readinessProbe.`);
  });

  endpoints.forEach((e, i) => {
    nameCheck("Endpoint", i, e);
    const n = named("Endpoint", i, e);
    if ((e?.port == null) === (e?.portRange == null)) p.push(`${n}: set either a port or a port range.`);
    if (e?.portRange != null && (e?.public || e?.protocol !== "TCP")) p.push(`${n}: a port range must be TCP and not public.`);
    if (e?.public && e?.protocol && e.protocol !== "HTTP") p.push(`${n}: public endpoints must use HTTP.`);
    const cors = e?.corsSettings;
    if (cors) {
      const origins = arr(cors["Access-Control-Allow-Origin"]);
      if (!origins.length) p.push(`${n}: CORS needs at least one allowed origin.`);
      if (origins.includes("*")) p.push(`${n}: CORS origins can't be "*".`);
    }
  });

  volumes.forEach((v, i) => {
    nameCheck("Volume", i, v);
    const n = named("Volume", i, v);
    if (!v?.source) p.push(`${n}: source is required.`);
    if ((v?.source === "memory" || v?.source === "block") && !v?.size) p.push(`${n}: size is required for ${v.source} volumes.`);
    if ((v?.uid != null || v?.gid != null) && v?.source !== "stage") p.push(`${n}: uid/gid apply to stage volumes only.`);
  });

  arr(doc?.serviceRoles).forEach((r, i) => {
    if (!r?.name || !/^\w+$/.test(r.name)) p.push(`Service role ${i + 1}: name must be letters, digits and underscores.`);
    arr(r?.endpoints).forEach((e) => {
      if (!epNames.has(e)) p.push(`Service role ${r?.name ?? i + 1}: endpoint "${e}" isn't declared.`);
    });
  });

  if (job) {
    if (endpoints.length) p.push("Jobs don't support endpoints.");
    if (doc?.serviceRoles) p.push("Jobs don't support serviceRoles.");
    if (spec.resourceManagement) p.push("Jobs don't support autoscaling (resourceManagement).");
  }
  return p;
}

const ENDPOINT_REF = /^[^.!\s]+\.[^.!\s]+\.[^.!\s]+![^.!\s]+$/;

export function validateGatewaySpec(doc: Doc): string[] {
  const p: string[] = [];
  const spec = doc?.spec ?? {};
  const target = (label: string, t: Doc) => {
    if (!t?.value) p.push(`${label}: pick an endpoint.`);
    else if (!ENDPOINT_REF.test(t.value)) p.push(`${label}: "${t.value}" isn't db.schema.service!endpoint.`);
  };
  if (spec.type === "traffic_split") {
    const ts = arr(spec.targets);
    if (!ts.length || ts.length > 5) p.push("A traffic split needs 1–5 targets.");
    ts.forEach((t, i) => target(`Target ${i + 1}`, t));
    const sum = ts.reduce((s, t) => s + (Number(t?.weight) || 0), 0);
    if (ts.length && sum !== 100) p.push(`Target weights sum to ${sum}; they must sum to 100.`);
  } else if (spec.type === "shadow_traffic") {
    const prim = arr(spec.primary);
    const shadow = arr(spec.shadow);
    if (prim.length !== 1) p.push("Shadow traffic needs exactly one primary endpoint.");
    prim.forEach((t) => target("Primary", t));
    if (!shadow.length) p.push("Shadow traffic needs at least one shadow endpoint.");
    shadow.forEach((t, i) => {
      target(`Shadow ${i + 1}`, t);
      const w = Number(t?.weight);
      if (t?.weight == null || !(w >= 0 && w <= 100)) p.push(`Shadow ${i + 1}: weight must be 0–100.`);
    });
  } else {
    p.push("spec.type must be traffic_split or shadow_traffic.");
  }
  return p;
}

export function validateInferenceSpec(doc: Doc): string[] {
  const p: string[] = [];
  if (!doc?.output?.stage_location) p.push("output.stage_location is required.");
  const cols = doc?.input?.column_handling ?? {};
  Object.entries(cols).forEach(([col, h]: [string, Doc]) => {
    if (!h?.convert_to) p.push(`Column ${col}: pick a convert_to.`);
  });
  return p;
}

/**
 * Problems blocking submit for spec text of a kind. Unparseable text (YAML
 * syntax errors, templates) returns [] — the YAML editor's schema squiggles
 * and Snowflake report those.
 */
export function specProblems(kind: SpecKind, text: string): string[] {
  if (!text.trim()) return [];
  const r = parseSpec(text);
  if ("error" in r) return [];
  switch (kind) {
    case "gateway": return validateGatewaySpec(r.doc);
    case "inference": return validateInferenceSpec(r.doc);
    default: return validateServiceSpec(r.doc, kind === "job");
  }
}
