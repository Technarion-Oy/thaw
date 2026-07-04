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

import { useState, useEffect } from "react";
import { Select, InputNumber, Button, Typography } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import { ListDatabases, ListUserSchemas, ListObjects, ListServiceEndpoints } from "../../../wailsjs/go/app/App";

const { Text } = Typography;

interface Props {
  // The gateway's own database / schema — used as the initial selection so the
  // common case (routing to a co-located service) needs no extra clicks.
  defaultDb: string;
  defaultSchema: string;
  // Called with a ready-to-insert YAML target block when the user clicks Insert.
  onInsert: (block: string) => void;
}

// EndpointTargetPicker lets the user pick a service endpoint by browsing
// database → schema → service → endpoint (rather than hand-typing the
// fully-qualified `db.schema.service!endpoint` reference into the YAML), then
// inserts a complete weighted `targets` entry into the specification editor.
// Services come from ListObjects (filtered to kind SERVICE); endpoints come from
// SHOW ENDPOINTS IN SERVICE (ListServiceEndpoints). All selects are searchable,
// so a service / endpoint not surfaced by the listing can still be typed.
export default function EndpointTargetPicker({ defaultDb, defaultSchema, onInsert }: Props) {
  const [databases, setDatabases] = useState<string[]>([]);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [services, setServices] = useState<string[]>([]);
  const [endpoints, setEndpoints] = useState<string[]>([]);

  const [db, setDb] = useState(defaultDb);
  const [schema, setSchema] = useState(defaultSchema);
  const [service, setService] = useState<string>("");
  const [endpoint, setEndpoint] = useState<string>("");
  const [weight, setWeight] = useState<number>(100);

  const [loadingServices, setLoadingServices] = useState(false);
  const [loadingEndpoints, setLoadingEndpoints] = useState(false);

  useEffect(() => { ListDatabases().then(setDatabases).catch(() => {}); }, []);

  useEffect(() => {
    if (!db) { setSchemas([]); return; }
    ListUserSchemas(db).then(setSchemas).catch(() => setSchemas([]));
  }, [db]);

  useEffect(() => {
    if (!db || !schema) { setServices([]); return; }
    setLoadingServices(true);
    ListObjects(db, schema)
      .then((objs) => setServices((objs ?? []).filter((o) => o.kind === "SERVICE").map((o) => o.name).sort()))
      .catch(() => setServices([]))
      .finally(() => setLoadingServices(false));
  }, [db, schema]);

  useEffect(() => {
    if (!db || !schema || !service) { setEndpoints([]); return; }
    setLoadingEndpoints(true);
    ListServiceEndpoints(db, schema, service)
      .then((res) => {
        const cols = res?.columns ?? [];
        const idx = cols.findIndex((c) => c.toLowerCase() === "name");
        const names = idx >= 0 ? (res?.rows ?? []).map((r) => String(r[idx] ?? "")).filter(Boolean) : [];
        setEndpoints(names);
      })
      .catch(() => setEndpoints([]))
      .finally(() => setLoadingEndpoints(false));
  }, [db, schema, service]);

  const opts = (xs: string[]) => xs.map((x) => ({ label: x, value: x }));
  const canInsert = !!(db && schema && service && endpoint);

  const handleInsert = () => {
    if (!canInsert) return;
    const block = `  - type: endpoint\n    value: ${db}.${schema}.${service}!${endpoint}\n    weight: ${weight}\n`;
    onInsert(block);
  };

  return (
    <div
      style={{
        border: "1px solid var(--border)",
        borderRadius: 6,
        padding: "10px 12px",
        marginBottom: 8,
        background: "var(--bg-subtle, transparent)",
      }}
    >
      <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
        Add an endpoint target — pick a service and endpoint, then Insert to drop a weighted{" "}
        <code>db.schema.service!endpoint</code> entry into the specification at the cursor.
      </Text>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "flex-end" }}>
        <Select
          showSearch
          size="small"
          style={{ minWidth: 130 }}
          placeholder="Database"
          value={db || undefined}
          options={opts(databases)}
          onChange={(v) => { setDb(v); setSchema(""); setService(""); setEndpoint(""); }}
        />
        <Select
          showSearch
          size="small"
          style={{ minWidth: 130 }}
          placeholder="Schema"
          value={schema || undefined}
          options={opts(schemas)}
          onChange={(v) => { setSchema(v); setService(""); setEndpoint(""); }}
        />
        <Select
          showSearch
          size="small"
          style={{ minWidth: 150 }}
          placeholder="Service"
          value={service || undefined}
          loading={loadingServices}
          options={opts(services)}
          notFoundContent={loadingServices ? "Loading…" : "No services"}
          onChange={(v) => { setService(v); setEndpoint(""); }}
        />
        <Select
          showSearch
          size="small"
          style={{ minWidth: 150 }}
          placeholder="Endpoint"
          value={endpoint || undefined}
          loading={loadingEndpoints}
          options={opts(endpoints)}
          notFoundContent={loadingEndpoints ? "Loading…" : "No endpoints"}
          onChange={(v) => setEndpoint(v)}
        />
        <InputNumber
          size="small"
          style={{ width: 90 }}
          min={0}
          max={100}
          value={weight}
          onChange={(v) => setWeight(typeof v === "number" ? v : 100)}
          addonBefore="wt"
        />
        <Button size="small" type="primary" icon={<PlusOutlined />} disabled={!canInsert} onClick={handleInsert}>
          Insert
        </Button>
      </div>
    </div>
  );
}
