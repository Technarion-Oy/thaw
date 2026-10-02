// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import { AutoComplete, Form, Radio, Select } from "antd";
import {
  ListImageRepositories, ListImagesInRepository, ListSecretsInAccount, ListSnapshots,
} from "../../../wailsjs/go/app/App";
import { rowsOf } from "../containerservices/computePools";
import { getIn, setIn, type Doc, type Path } from "./specDoc";
import { Bool, Grid, Group, KV, List, Num, Sel, Str, Tags, useSpec } from "./specFields";

const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"];
const METRIC_GROUPS = ["system", "system_limits", "status", "network", "storage"];
const names = (xs: unknown): string[] =>
  (Array.isArray(xs) ? xs : []).map((x: Doc) => x?.name).filter((n): n is string => typeof n === "string" && n !== "");

/** An image repository as its registry path segments (lowercase, as image paths use them). */
interface Repo { db: string; schema: string; name: string; seg: [string, string, string] }

// Image "name:tag" entries per repository, fetched once per session on first use.
const imageCache = new Map<string, Promise<string[]>>();
const imagesOf = (r: Repo) => {
  const key = r.seg.join("/");
  if (!imageCache.has(key)) {
    imageCache.set(key, ListImagesInRepository(r.db, r.schema, r.name)
      .then((res) => rowsOf(res).map((i) => (i.image_path ? i.image_path.split("/").slice(3).join("/") : i.image_name)).filter(Boolean))
      .catch(() => []));
  }
  return imageCache.get(key)!;
};

/**
 * Image as database → schema → repository → image:tag dropdowns over the
 * account's image repositories, composing `/db/schema/repo/image:tag`. The
 * last box also accepts a typed image:tag.
 */
function ImageField({ p, repos }: { p: Path; repos: Repo[] }) {
  const { doc, set } = useSpec();
  const value: string = getIn(doc, p) ?? "";
  const parse = (v: string) => {
    const s = v.split("/").filter(Boolean);
    return s.length >= 4 ? [s[0], s[1], s[2], s.slice(3).join("/")] : null;
  };
  const [pick, setPick] = useState<string[]>(() => parse(value) ?? ["", "", "", ""]);
  // Follow the value when it changes underneath (YAML edit, row removed above).
  useEffect(() => { const v = parse(value); if (v) setPick(v); }, [value]);
  const [db, schema, repo, image] = pick;
  const update = (next: string[]) => {
    setPick(next);
    set(p, next.every(Boolean) ? `/${next.join("/")}` : undefined);
  };

  const lc = (x: string) => x.toLowerCase();
  const uniq = (xs: string[]) => [...new Set(xs)].map((x) => ({ value: x, label: x }));
  const repoRow = repos.find((r) => lc(r.seg.join("/")) === lc(`${db}/${schema}/${repo}`));
  const [images, setImages] = useState<string[]>([]);
  // Typed text is offered as an option too: the image may not be pushed yet,
  // or the role can't list the repository's images (needs READ).
  const [search, setSearch] = useState("");
  const typed = search.trim();
  const imageOptions = [
    ...uniq(images),
    ...(typed && !images.includes(typed) ? [{ value: typed, label: `Use "${typed}"` }] : []),
  ];
  useEffect(() => {
    if (!repoRow) { setImages([]); return; }
    let live = true;
    imagesOf(repoRow).then((xs) => live && setImages(xs));
    return () => { live = false; };
  }, [repoRow]);

  const sel = { showSearch: true, size: "small" as const, style: { flex: 1, minWidth: 110 } };
  return (
    <Form.Item label="Image" style={{ marginBottom: 6, gridColumn: "span 2" }}
      help={value ? <code style={{ fontSize: 11 }}>{value}</code> : undefined}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
        <Select {...sel} placeholder="Database" value={db || undefined}
          options={uniq(repos.map((r) => r.seg[0]))} onChange={(v) => update([v, "", "", ""])} />
        <Select {...sel} placeholder="Schema" value={schema || undefined}
          options={uniq(repos.filter((r) => lc(r.seg[0]) === lc(db)).map((r) => r.seg[1]))}
          onChange={(v) => update([db, v, "", ""])} />
        <Select {...sel} placeholder="Repository" value={repo || undefined}
          options={uniq(repos.filter((r) => lc(r.seg[0]) === lc(db) && lc(r.seg[1]) === lc(schema)).map((r) => r.seg[2]))}
          onChange={(v) => update([db, schema, v, ""])} />
        <Select {...sel} style={{ flex: 1.5, minWidth: 150 }} placeholder="image:tag" value={image || undefined}
          options={imageOptions} optionFilterProp="value" onSearch={setSearch} disabled={!repo}
          onChange={(v) => { setSearch(""); update([db, schema, repo, v]); }}
          notFoundContent="No images listed — type image:tag" />
      </div>
    </Form.Item>
  );
}

/** One container secret: the secret, then env var + key or a directory. */
function SecretRow({ base, secrets }: { base: Path; secrets: string[] }) {
  const { doc, set, edit } = useSpec();
  const ss = getIn(doc, [...base, "snowflakeSecret"]);
  const [asDir, setAsDir] = useState(getIn(doc, [...base, "directoryPath"]) != null);
  const pickMode = (dir: boolean) => {
    setAsDir(dir);
    edit((d) => dir
      ? setIn(setIn(d, [...base, "envVarName"], undefined), [...base, "secretKeyRef"], undefined)
      : setIn(d, [...base, "directoryPath"], undefined));
  };
  return (
    <Grid>
      <Form.Item label="Secret" style={{ marginBottom: 6, gridColumn: "span 2" }}>
        {/* ponytail: objectReference (secret by reference) is YAML-only. */}
        <AutoComplete
          value={typeof ss === "string" ? ss : ss?.objectName ?? ""}
          onChange={(v) => set([...base, "snowflakeSecret"], typeof ss === "object" && ss ? { ...ss, objectName: v || undefined } : v)}
          options={secrets.map((s) => ({ value: s }))} placeholder="db.schema.secret"
          filterOption={(input, o) => String(o?.value).toLowerCase().includes(input.toLowerCase())}
        />
      </Form.Item>
      <Form.Item label="Expose as" style={{ marginBottom: 6 }}>
        <Radio.Group size="small" optionType="button" value={asDir ? "dir" : "env"} onChange={(e) => pickMode(e.target.value === "dir")}
          options={[{ value: "env", label: "Env var" }, { value: "dir", label: "Directory" }]} />
      </Form.Item>
      {asDir ? (
        <Str p={[...base, "directoryPath"]} label="Directory path" span={3} placeholder="/usr/local/creds" />
      ) : (
        <>
          <Str p={[...base, "envVarName"]} label="Env var name" span={2} placeholder="LOGIN_PASSWORD" />
          <Sel p={[...base, "secretKeyRef"]} label="Secret key" options={["username", "password", "secret_string"]} />
        </>
      )}
    </Grid>
  );
}

function Conditions({ p, label }: { p: Path; label: string }) {
  return (
    <List p={p} label={label} noun="condition">
      {(b) => (
        <Grid cols={4}>
          <Str p={[...b, "metricName"]} label="Metric" placeholder="container.cpu.usage" />
          <Sel p={[...b, "aggregationType"]} label="Aggregation" options={["min", "max", "avg"]} />
          <Num p={[...b, "stabilizationPeriodSecs"]} label="Stabilization (s)" min={0} />
          <Num p={[...b, "targetScaling", "targetValue"]} label="Target value" />
        </Grid>
      )}
    </List>
  );
}

/**
 * Service / job specification form (issue #958), rendered inside SpecEditor.
 * `job` hides what EXECUTE JOB SERVICE rejects: endpoints, service roles,
 * readiness probes and autoscaling.
 */
export default function ServiceSpecForm({ job = false }: { job?: boolean }) {
  const { doc } = useSpec();
  const [repos, setRepos] = useState<Repo[]>([]);
  const [secrets, setSecrets] = useState<string[]>([]);
  const [snapshots, setSnapshots] = useState<string[]>([]);
  useEffect(() => {
    // Path segments come from repository_url (host/db/schema/repo) — the registry's own spelling.
    ListImageRepositories().then((r) => setRepos(rowsOf(r).map((x) => {
      const u = (x.repository_url ?? "").split("/").slice(1);
      const seg = (u.length === 3 ? u : [x.database_name, x.schema_name, x.name].map((n) => n.toLowerCase())) as Repo["seg"];
      return { db: x.database_name, schema: x.schema_name, name: x.name, seg };
    }))).catch(() => {});
    ListSecretsInAccount().then((xs) => setSecrets((xs ?? []).map((x) => `${x.databaseName}.${x.schemaName}.${x.name}`))).catch(() => {});
    ListSnapshots().then((r) => setSnapshots(rowsOf(r).map((x) => `${x.database_name}.${x.schema_name}.${x.name}`))).catch(() => {});
  }, []);

  const volumes = names(doc?.spec?.volumes);
  const endpoints = names(doc?.spec?.endpoints);
  const sp = (...k: (string | number)[]): Path => ["spec", ...k];

  return (
    <Grid>
      <List p={sp("containers")} label="Containers" noun="container">
        {(c) => (
          <Grid>
            <Str p={[...c, "name"]} label="Name" placeholder="main" />
            <ImageField p={[...c, "image"]} repos={repos} />
            <Tags p={[...c, "command"]} label="Command" span={3} placeholder="Overrides the image ENTRYPOINT" />
            <Tags p={[...c, "args"]} label="Args" span={3} placeholder="Overrides the image CMD" />
            <Group title="Environment" p={[...c, "env"]}>
              <KV p={[...c, "env"]} label="Variables" />
            </Group>
            <Group title="Resources" p={[...c, "resources"]}>
              <Str p={[...c, "resources", "requests", "cpu"]} label="CPU request" numeric placeholder="0.5 or 500m" />
              <Str p={[...c, "resources", "requests", "memory"]} label="Memory request" placeholder="1Gi" />
              <Num p={[...c, "resources", "requests", "nvidia.com/gpu"]} label="GPU request" min={0} />
              <Str p={[...c, "resources", "limits", "cpu"]} label="CPU limit" numeric placeholder="1" />
              <Str p={[...c, "resources", "limits", "memory"]} label="Memory limit" placeholder="2Gi" />
              <Num p={[...c, "resources", "limits", "nvidia.com/gpu"]} label="GPU limit" min={0} />
            </Group>
            <Group title="Probes" p={job ? [...c, "livenessProbe"] : [...c, "readinessProbe"]} cols={4}>
              {!job && <>
                <Num p={[...c, "readinessProbe", "port"]} label="Readiness port" min={1} />
                <Str p={[...c, "readinessProbe", "path"]} label="Readiness path" placeholder="/healthz" />
                <Num p={[...c, "readinessProbe", "periodSeconds"]} label="Period (s)" placeholder="5" min={1} />
                <Num p={[...c, "readinessProbe", "failureThreshold"]} label="Failure threshold" placeholder="3" min={1} />
              </>}
              <Num p={[...c, "livenessProbe", "port"]} label="Liveness port" min={1} />
              <Str p={[...c, "livenessProbe", "path"]} label="Liveness path" placeholder="/healthz" />
              <Num p={[...c, "livenessProbe", "periodSeconds"]} label="Period (s)" placeholder="10" min={1} />
              <Num p={[...c, "livenessProbe", "failureThreshold"]} label="Failure threshold" placeholder="3" min={1} />
            </Group>
            <Group title="Volume mounts" p={[...c, "volumeMounts"]}>
              <List p={[...c, "volumeMounts"]} noun="mount">
                {(m) => (
                  <Grid>
                    <Sel p={[...m, "name"]} label="Volume" options={volumes} placeholder="Add volumes below" />
                    <Str p={[...m, "mountPath"]} label="Mount path" span={2} placeholder="/data" />
                  </Grid>
                )}
              </List>
            </Group>
            <Group title="Secrets" p={[...c, "secrets"]}>
              <List p={[...c, "secrets"]} noun="secret">{(s) => <SecretRow base={s} secrets={secrets} />}</List>
            </Group>
            <Group title="Session token" p={[...c, "sessionTokenConfig"]}>
              <Bool p={[...c, "mountSessionToken"]} label="Mount session token" def />
              {getIn(doc, [...c, "mountSessionToken"]) !== false && <>
                <Str p={[...c, "sessionTokenConfig", "path"]} label="Path" placeholder="/snowflake/session/token" />
                <Str p={[...c, "sessionTokenConfig", "permission"]} label="Permission" placeholder="0644" />
                <Bool p={[...c, "sessionTokenConfig", "mountConnectionConfig"]} label="Mount connection config" />
              </>}
            </Group>
          </Grid>
        )}
      </List>

      {!job && (
        <List p={sp("endpoints")} label="Endpoints" noun="endpoint" item={() => ({ port: 8080 })}>
          {(e) => <EndpointRow base={e} />}
        </List>
      )}

      <List p={sp("volumes")} label="Volumes" noun="volume" item={() => ({ source: "local" })}>
        {(v) => {
          const source = getIn(doc, [...v, "source"]);
          return (
            <Grid cols={4}>
              <Str p={[...v, "name"]} label="Name" placeholder="data" />
              <Sel p={[...v, "source"]} label="Source" options={["local", "memory", "block", "stage"]} />
              {(source === "memory" || source === "block") && <Str p={[...v, "size"]} label="Size" placeholder="10Gi" />}
              {source === "stage" && <>
                {/* ponytail: stageConfig.resources is YAML-only. */}
                <Str p={[...v, "stageConfig", "name"]} label="Stage" placeholder="@my_stage/path" />
                <Num p={[...v, "uid"]} label="uid" min={0} />
                <Num p={[...v, "gid"]} label="gid" min={0} />
                <Str p={[...v, "stageConfig", "metadataCache"]} label="Metadata cache" placeholder="1m" />
              </>}
              {source === "block" && (
                <Group title="Block storage" p={[...v, "blockConfig"]}>
                  <Str p={[...v, "blockConfig", "initialContents", "fromSnapshot"]} label="From snapshot" span={2} options={snapshots} />
                  <Sel p={[...v, "blockConfig", "encryption"]} label="Encryption" options={["SNOWFLAKE_SSE", "SNOWFLAKE_FULL"]} />
                  <Num p={[...v, "blockConfig", "iops"]} label="IOPS" min={0} />
                  <Num p={[...v, "blockConfig", "throughput"]} label="Throughput (MiB/s)" min={0} />
                  <Str p={[...v, "blockConfig", "snapshotDeleteAfter"]} label="Snapshot delete after" placeholder="7d" />
                  <Bool p={[...v, "blockConfig", "snapshotOnDelete"]} label="Snapshot on delete" def={!job} />
                </Group>
              )}
            </Grid>
          );
        }}
      </List>

      <Group title="Logging & monitoring" p={sp("logExporters")} cols={2}>
        <Sel p={sp("logExporters", "eventTableConfig", "logLevel")} label="Event table log level" options={["INFO", "ERROR", "NONE"]} />
        <Sel p={sp("platformMonitor", "metricConfig", "groups")} label="Platform metric groups" mode="multiple" options={METRIC_GROUPS} />
      </Group>

      {!job && (
        <Group title="Autoscaling" p={sp("resourceManagement")} cols={1}>
          <Conditions p={sp("resourceManagement", "autoScalingPolicies", "scaleUp", "anyConditions")} label="Scale up when any of" />
          <Conditions p={sp("resourceManagement", "autoScalingPolicies", "scaleDown", "allConditions")} label="Scale down when all of" />
          <Conditions p={sp("resourceManagement", "autoScalingPolicies", "suspend", "allConditions")} label="Suspend when all of" />
        </Group>
      )}

      <Group title="Capabilities" p={["capabilities"]} cols={2}>
        <Bool p={["capabilities", "securityContext", "executeAsCaller"]} label="Execute as caller (caller's rights)" />
        <Bool p={["capabilities", "securityContext", "enableCustomCredentials"]} label="Enable custom credentials" />
      </Group>

      {!job && (
        <List p={["serviceRoles"]} label="Service roles" noun="service role">
          {(r) => (
            <Grid>
              <Str p={[...r, "name"]} label="Name" placeholder="API_USER" />
              <Sel p={[...r, "endpoints"]} label="Endpoints" span={2} mode="multiple" options={endpoints} />
            </Grid>
          )}
        </List>
      )}
    </Grid>
  );
}

/** One endpoint. Port ⇄ port range is local state: an empty range has no key to derive it from. */
function EndpointRow({ base }: { base: Path }) {
  const { doc, edit } = useSpec();
  const [range, setRange] = useState(getIn(doc, [...base, "portRange"]) != null);
  const toggle = (toRange: boolean) => {
    setRange(toRange);
    // A range is TCP-only and never public.
    edit((d) => toRange
      ? setIn(setIn(setIn(d, [...base, "port"], undefined), [...base, "protocol"], "TCP"), [...base, "public"], undefined)
      : setIn(d, [...base, "portRange"], undefined));
  };
  return (
    <Grid cols={4}>
      <Str p={[...base, "name"]} label="Name" placeholder="api" />
      {range
        ? <Str p={[...base, "portRange"]} label="Port range" placeholder="5000-5010" />
        : <Num p={[...base, "port"]} label="Port" min={1} max={65535} />}
      <Sel p={[...base, "protocol"]} label="Protocol" options={["HTTP", "TCP"]} placeholder="HTTP" />
      <Bool p={[...base, "public"]} label="Public" />
      <Form.Item label=" " style={{ marginBottom: 6, gridColumn: "1 / -1" }}>
        <Radio.Group size="small" optionType="button" value={range ? "range" : "port"} onChange={(ev) => toggle(ev.target.value === "range")}
          options={[{ value: "port", label: "Single port" }, { value: "range", label: "Port range (TCP)" }]} />
      </Form.Item>
      <Group title="CORS" p={[...base, "corsSettings"]} cols={2}>
        <Tags p={[...base, "corsSettings", "Access-Control-Allow-Origin"]} label="Allowed origins" placeholder="https://app.example.com" />
        <Tags p={[...base, "corsSettings", "Access-Control-Allow-Methods"]} label="Allowed methods" options={HTTP_METHODS} />
        <Tags p={[...base, "corsSettings", "Access-Control-Allow-Headers"]} label="Allowed headers" />
        <Tags p={[...base, "corsSettings", "Access-Control-Expose-Headers"]} label="Exposed headers" />
      </Group>
    </Grid>
  );
}
