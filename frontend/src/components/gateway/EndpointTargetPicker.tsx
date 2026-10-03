// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import { Select } from "antd";
import { ListDatabases, ListUserSchemas, ListObjects, ListServiceEndpoints } from "../../../wailsjs/go/app/App";
import { quoteIfNecessary } from "../editor/sqlEditorUtils";
import { parseEndpointRef } from "../service/specDoc";

interface Props {
  // The gateway's own database / schema — used as the initial selection so the
  // common case (routing to a co-located service) needs no extra clicks.
  defaultDb: string;
  defaultSchema: string;
  /** The endpoint reference `db.schema.service!endpoint` ("" when incomplete). */
  value: string;
  onChange: (value: string) => void;
}

// An identifier as written in the reference → the object's name for the list
// calls: quoted keeps its case, bare folds to upper case.
const raw = (id: string) => (id.startsWith('"') ? id.slice(1, -1).replace(/""/g, '"') : id.toUpperCase());

// EndpointTargetPicker is a row control for one gateway target: database →
// schema → service → endpoint searchable dropdowns that emit the
// fully-qualified `db.schema.service!endpoint` reference once all four are set
// (and "" while incomplete). The database / schema / service parts are held as
// written in the reference, so names that need quoting ("My Db", "a.b") are
// offered quoted and round-trip. Services come from ListObjects (filtered to kind
// SERVICE); endpoints come from SHOW ENDPOINTS IN SERVICE (ListServiceEndpoints).
export default function EndpointTargetPicker({ defaultDb, defaultSchema, value, onChange }: Props) {
  const [databases, setDatabases] = useState<string[]>([]);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [services, setServices] = useState<string[]>([]);
  const [endpoints, setEndpoints] = useState<string[]>([]);

  const m = parseEndpointRef(value);
  const [db, setDb] = useState(m?.[0] ?? quoteIfNecessary(defaultDb));
  const [schema, setSchema] = useState(m?.[1] ?? quoteIfNecessary(defaultSchema));
  const [service, setService] = useState<string>(m?.[2] ?? "");
  const [endpoint, setEndpoint] = useState<string>(m?.[3] ?? "");

  // Follow the value when it changes underneath (a row above was removed).
  useEffect(() => {
    const r = parseEndpointRef(value);
    if (r) { setDb(r[0]); setSchema(r[1]); setService(r[2]); setEndpoint(r[3]); }
  }, [value]);

  const [loadingServices, setLoadingServices] = useState(false);
  const [loadingEndpoints, setLoadingEndpoints] = useState(false);
  useEffect(() => { ListDatabases().then(setDatabases).catch(() => {}); }, []);

  useEffect(() => {
    if (!db) { setSchemas([]); return; }
    ListUserSchemas(raw(db)).then(setSchemas).catch(() => setSchemas([]));
  }, [db]);

  useEffect(() => {
    if (!db || !schema) { setServices([]); return; }
    setLoadingServices(true);
    ListObjects(raw(db), raw(schema))
      .then((objs) => setServices((objs ?? []).filter((o) => o.kind === "SERVICE").map((o) => o.name).sort()))
      .catch(() => setServices([]))
      .finally(() => setLoadingServices(false));
  }, [db, schema]);

  useEffect(() => {
    if (!db || !schema || !service) { setEndpoints([]); return; }
    setLoadingEndpoints(true);
    ListServiceEndpoints(raw(db), raw(schema), raw(service))
      .then((res) => {
        const cols = res?.columns ?? [];
        const idx = cols.findIndex((c) => c.toLowerCase() === "name");
        const names = idx >= 0 ? (res?.rows ?? []).map((r) => String(r[idx] ?? "")).filter(Boolean) : [];
        setEndpoints(names);
      })
      .catch(() => setEndpoints([]))
      .finally(() => setLoadingEndpoints(false));
  }, [db, schema, service]);

  const opts = (xs: string[]) => xs.map((x) => ({ label: x, value: quoteIfNecessary(x) }));
  const pick = (d: string, sc: string, sv: string, ep: string) => {
    setDb(d); setSchema(sc); setService(sv); setEndpoint(ep);
    onChange(d && sc && sv && ep ? `${d}.${sc}.${sv}!${ep}` : "");
  };

  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
      <Select showSearch size="small" style={{ minWidth: 120, flex: 1 }} placeholder="Database"
        value={db || undefined} options={opts(databases)} onChange={(v) => pick(v, "", "", "")} />
      <Select showSearch size="small" style={{ minWidth: 120, flex: 1 }} placeholder="Schema"
        value={schema || undefined} options={opts(schemas)} onChange={(v) => pick(db, v, "", "")} />
      <Select showSearch size="small" style={{ minWidth: 140, flex: 1 }} placeholder="Service"
        value={service || undefined} loading={loadingServices} options={opts(services)}
        notFoundContent={loadingServices ? "Loading…" : "No services"} onChange={(v) => pick(db, schema, v, "")} />
      <Select showSearch size="small" style={{ minWidth: 140, flex: 1 }} placeholder="Endpoint"
        value={endpoint || undefined} loading={loadingEndpoints} options={endpoints.map((x) => ({ label: x, value: x }))}
        notFoundContent={loadingEndpoints ? "Loading…" : "No endpoints"} onChange={(v) => pick(db, schema, service, v)} />
    </div>
  );
}
