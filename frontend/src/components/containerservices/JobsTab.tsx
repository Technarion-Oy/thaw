// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useState } from "react";
import { App as AntApp, Button, Descriptions, Dropdown, Modal, Select, Space } from "antd";
import type { ColumnsType } from "antd/es/table";
import { MoreOutlined } from "@ant-design/icons";
import { DropJobService, ListJobServices } from "../../../wailsjs/go/app/App";
import { friendlyError } from "../common/errors";
import ServicePropertiesModal, { ServiceLogs } from "../service/ServicePropertiesModal";
import ContainerTabLayout from "./ContainerTabLayout";
import RunJobModal from "./RunJobModal";
import { rowsOf, type ResultRow } from "./computePools";

/** Job status → dot colour (SHOW JOB SERVICES `status`). */
export function jobStatusColor(status: string | undefined): string {
  switch ((status ?? "").toUpperCase()) {
    case "DONE": return "var(--success)";
    case "PENDING": case "RUNNING": return "var(--link)";
    case "SUSPENDED": case "DELETED": return "var(--text-faint)";
    default: return "var(--danger)"; // FAILED, INTERNAL_ERROR, …
  }
}

const fqn = (r: ResultRow) => `${r.database_name}.${r.schema_name}.${r.name}`;

/** Jobs tab of the Container Services dialog (issue #942). */
export default function JobsTab({ onCloseDialog }: { onCloseDialog: () => void }) {
  const { modal, message } = AntApp.useApp();
  const [rows, setRows] = useState<ResultRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>();
  const [running, setRunning] = useState(false);
  const [props, setProps] = useState<ResultRow | null>(null);
  const [logs, setLogs] = useState<ResultRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Job names repeat across schemas, so key rows by their FQN.
      setRows(rowsOf(await ListJobServices()).map((r) => ({ ...r, fqn: fqn(r) })));
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  const confirmDrop = (r: ResultRow) => modal.confirm({
    title: `Drop job service ${r.name}?`,
    content: "The job service is deleted; a running job is terminated and its logs are no longer retrievable.",
    okText: "Drop",
    okButtonProps: { danger: true },
    onOk: async () => {
      try {
        await DropJobService(r.database_name, r.schema_name, r.name);
      } catch (e) {
        message.error(friendlyError(e));
        throw e;
      }
      message.success(`Dropped ${r.name}`);
      load();
    },
  });

  // A synchronous job blocks its query tab until it finishes, so close the
  // dialog to show that tab; an async job returns at once and is listed here.
  const onRan = (isAsync: boolean) => (isAsync ? load() : onCloseDialog());

  const columns: ColumnsType<ResultRow> = [
    {
      title: "Status", dataIndex: "status", width: 120,
      render: (s: string) => (
        <Space size={6}>
          <span style={{ display: "inline-block", width: 8, height: 8, borderRadius: 4, background: jobStatusColor(s) }} />
          {s}
        </Space>
      ),
    },
    { title: "Name", dataIndex: "fqn", ellipsis: true },
    { title: "Compute pool", dataIndex: "compute_pool", ellipsis: true },
    { title: "Async", dataIndex: "is_async_job", width: 70 },
    { title: "Created", dataIndex: "created_on", width: 180, ellipsis: true },
    { title: "Owner", dataIndex: "owner", ellipsis: true },
    {
      key: "menu", width: 40,
      render: (_, r) => (
        <Dropdown
          trigger={["click"]}
          menu={{
            items: [
              { key: "logs", label: "Logs…", onClick: () => setLogs(r) },
              { key: "props", label: "Properties…", onClick: () => setProps(r) },
              { type: "divider" },
              { key: "drop", label: "Drop", danger: true, onClick: () => confirmDrop(r) },
            ],
          }}
        >
          <Button size="small" type="text" icon={<MoreOutlined />} onClick={(e) => e.stopPropagation()} />
        </Dropdown>
      ),
    },
  ];

  const statuses = [...new Set(rows.map((r) => r.status).filter(Boolean))];

  return (
    <>
      <ContainerTabLayout<ResultRow>
        objectLabel="job services"
        rowKey="fqn"
        columns={columns}
        rows={statusFilter ? rows.filter((r) => r.status === statusFilter) : rows}
        loading={loading}
        error={error}
        onRefresh={load}
        newLabel="Run job…"
        onNew={() => setRunning(true)}
        facets={
          <Select
            size="small" allowClear placeholder="Status" style={{ width: 130 }}
            value={statusFilter} onChange={setStatusFilter}
            options={statuses.map((s) => ({ value: s, label: s }))}
          />
        }
        filter={(r, q) => [r.fqn, r.compute_pool, r.owner, r.comment].some((v) => v?.toLowerCase().includes(q))}
        detail={(r) => (
          <Descriptions size="small" bordered column={3} styles={{ label: { fontSize: 11 }, content: { fontSize: 12 } }}>
            {Object.entries(r).filter(([k, v]) => v !== "" && k !== "fqn").map(([k, v]) => (
              <Descriptions.Item key={k} label={k}>{v}</Descriptions.Item>
            ))}
          </Descriptions>
        )}
      />

      {running && <RunJobModal onClose={() => setRunning(false)} onRan={onRan} />}
      {props && (
        <ServicePropertiesModal
          db={props.database_name} schema={props.schema_name} name={props.name}
          onClose={() => setProps(null)}
        />
      )}
      {logs && (
        <Modal open title={`Logs — ${logs.fqn}`} width={860} onCancel={() => setLogs(null)}
          footer={<Button onClick={() => setLogs(null)}>Close</Button>}>
          <ServiceLogs db={logs.database_name} schema={logs.schema_name} name={logs.name} />
        </Modal>
      )}
    </>
  );
}
