// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useState } from "react";
import { App as AntApp, Button, Dropdown, Select, Space, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { MoreOutlined } from "@ant-design/icons";
import {
  AlterService, DropService, ListServiceEndpoints, ListServiceInstances, ListServicesInAccount,
} from "../../../wailsjs/go/app/App";
import { friendlyError } from "../common/errors";
import LazyResultTable from "../shared/LazyResultTable";
import ServicePropertiesModal from "../service/ServicePropertiesModal";
import ContainerTabLayout from "./ContainerTabLayout";
import ScopedCreate from "./ScopedCreate";
import CreateServiceModal from "../service/CreateServiceModal";
import { rowsOf, serviceLifecycle, serviceStatusColor, type ResultRow } from "./computePools";

const { Text } = Typography;
const fqn = (r: ResultRow) => `${r.database_name}.${r.schema_name}.${r.name}`;

/** Detail panel for the selected row: status plus the same lazy sections Properties has. */
function ServiceDetail({ r }: { r: ResultRow }) {
  return (
    <div>
      <Space size={6} style={{ marginBottom: 8 }}>
        <span style={{ display: "inline-block", width: 8, height: 8, borderRadius: 4, background: serviceStatusColor(r.status) }} />
        <Text strong style={{ fontSize: 12 }}>{r.status}</Text>
      </Space>
      <LazyResultTable title="Endpoints" noun="endpoint" load={() => ListServiceEndpoints(r.database_name, r.schema_name, r.name)} />
      <LazyResultTable title="Instances" noun="instance" load={() => ListServiceInstances(r.database_name, r.schema_name, r.name)} />
    </div>
  );
}

/** Services tab of the Container Services dialog (issue #944). */
export default function ServicesTab() {
  const { modal, message } = AntApp.useApp();
  const [rows, setRows] = useState<ResultRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>();
  const [poolFilter, setPoolFilter] = useState<string>();
  const [dbFilter, setDbFilter] = useState<string>();
  const [creating, setCreating] = useState(false);
  const [props, setProps] = useState<{ row: ResultRow; focusSpec: boolean } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Service names repeat across schemas, so key rows by their FQN.
      setRows(rowsOf(await ListServicesInAccount()).map((r) => ({ ...r, fqn: fqn(r) })));
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  const run = async (what: string, fn: () => Promise<void>) => {
    try {
      await fn();
    } catch (e) {
      message.error(friendlyError(e));
      throw e;
    }
    message.success(what);
    load();
  };

  const confirmLifecycle = (r: ResultRow, resume: boolean) => modal.confirm({
    title: `${resume ? "Resume" : "Suspend"} Service`,
    content: resume
      ? `Resume "${r.name}"? Snowflake recreates its containers from the service spec.`
      : `Suspend "${r.name}"? Snowflake shuts down and deletes its containers until the service is resumed.`,
    okText: resume ? "Resume" : "Suspend",
    okButtonProps: resume ? undefined : { danger: true },
    onOk: () => run(`Service "${r.name}" ${resume ? "resumed" : "suspended"}.`, () => AlterService(r.database_name, r.schema_name, r.name, resume ? "RESUME" : "SUSPEND")),
  });

  const confirmDrop = (r: ResultRow) => modal.confirm({
    title: `Drop service ${r.name}?`,
    content: "The service and its containers are deleted; this cannot be undone.",
    okText: "Drop",
    okButtonProps: { danger: true },
    onOk: () => run(`Dropped ${r.name}`, () => DropService(r.database_name, r.schema_name, r.name)),
  });

  const columns: ColumnsType<ResultRow> = [
    {
      title: "Status", dataIndex: "status", width: 110,
      render: (s: string) => (
        <Space size={6}>
          <span style={{ display: "inline-block", width: 8, height: 8, borderRadius: 4, background: serviceStatusColor(s) }} />
          {s}
        </Space>
      ),
    },
    { title: "Name", dataIndex: "name", ellipsis: true },
    { title: "Database.Schema", width: 180, ellipsis: true, render: (_, r) => `${r.database_name}.${r.schema_name}` },
    { title: "Compute pool", dataIndex: "compute_pool", ellipsis: true },
    { title: "Instances", width: 110, render: (_, r) => `${r.current_instances || "0"} (${r.min_instances}–${r.max_instances})` },
    { title: "Auto-resume", dataIndex: "auto_resume", width: 100 },
    { title: "Query warehouse", dataIndex: "query_warehouse", ellipsis: true },
    { title: "Owner", dataIndex: "owner", ellipsis: true },
    { title: "Updated", dataIndex: "updated_on", width: 180, ellipsis: true },
    {
      key: "menu", width: 40,
      render: (_, r) => {
        const action = serviceLifecycle(r.status);
        return (
          <Dropdown
            trigger={["click"]}
            menu={{
              items: [
                { key: "props", label: "Properties…", onClick: () => setProps({ row: r, focusSpec: false }) },
                { key: "life", label: action === "RESUME" ? "Resume" : "Suspend", disabled: !action, onClick: () => confirmLifecycle(r, action === "RESUME") },
                { key: "redeploy", label: "Redeploy…", onClick: () => setProps({ row: r, focusSpec: true }) },
                { type: "divider" },
                { key: "drop", label: "Drop", danger: true, onClick: () => confirmDrop(r) },
              ],
            }}
          >
            <Button size="small" type="text" icon={<MoreOutlined />} onClick={(e) => e.stopPropagation()} />
          </Dropdown>
        );
      },
    },
  ];

  const statuses = [...new Set(rows.map((r) => r.status).filter(Boolean))];
  const pools = [...new Set(rows.map((r) => r.compute_pool).filter(Boolean))];
  const dbs = [...new Set(rows.map((r) => r.database_name).filter(Boolean))];
  const filtered = rows
    .filter((r) => !statusFilter || r.status === statusFilter)
    .filter((r) => !poolFilter || r.compute_pool === poolFilter)
    .filter((r) => !dbFilter || r.database_name === dbFilter);

  return (
    <>
      <ContainerTabLayout<ResultRow>
        objectLabel="services"
        rowKey="fqn"
        columns={columns}
        rows={filtered}
        loading={loading}
        error={error}
        onRefresh={load}
        newLabel="New service…"
        onNew={() => setCreating(true)}
        facets={
          <>
            <Select
              size="small" allowClear placeholder="Status" style={{ width: 130 }}
              value={statusFilter} onChange={setStatusFilter}
              options={statuses.map((s) => ({ value: s, label: s }))}
            />
            <Select
              size="small" allowClear placeholder="Compute pool" style={{ width: 150 }}
              value={poolFilter} onChange={setPoolFilter}
              options={pools.map((p) => ({ value: p, label: p }))}
            />
            <Select
              size="small" allowClear placeholder="Database" style={{ width: 150 }}
              value={dbFilter} onChange={setDbFilter}
              options={dbs.map((d) => ({ value: d, label: d }))}
            />
          </>
        }
        filter={(r, q) => [r.fqn, r.compute_pool, r.query_warehouse, r.owner, r.comment].some((v) => v?.toLowerCase().includes(q))}
        detail={(r) => <ServiceDetail r={r} />}
      />

      {creating && (
        <ScopedCreate title="New service" onClose={() => setCreating(false)}>
          {(db, schema) => (
            <CreateServiceModal db={db} schema={schema} onClose={() => setCreating(false)} onSuccess={load} />
          )}
        </ScopedCreate>
      )}
      {props && (
        <ServicePropertiesModal
          db={props.row.database_name} schema={props.row.schema_name} name={props.row.name}
          focusSpec={props.focusSpec}
          onClose={() => { setProps(null); load(); }}
        />
      )}
    </>
  );
}
