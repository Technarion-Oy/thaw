// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useRef, useState } from "react";
import { App as AntApp, Button, Descriptions, Dropdown, Input, Select, Space } from "antd";
import type { ColumnsType } from "antd/es/table";
import { MoreOutlined, TableOutlined } from "@ant-design/icons";
import {
  AlterComputePool, DropComputePool, ListComputePoolInstanceFamilies, ListComputePoolsDetailed,
  StopAllComputePoolServices,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { friendlyError } from "../common/errors";
import ContainerTabLayout from "./ContainerTabLayout";
import CreateComputePoolModal from "./CreateComputePoolModal";
import ComputePoolPropertiesModal from "./ComputePoolPropertiesModal";
import InstanceFamiliesModal from "./InstanceFamiliesModal";
import { rowsOf, stateColor, type ResultRow } from "./computePools";

/** Compute pools tab of the Container Services dialog (issue #941). */
export default function ComputePoolsTab() {
  const { modal, message } = AntApp.useApp();
  const [rows, setRows] = useState<ResultRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stateFilter, setStateFilter] = useState<string>();
  const [creating, setCreating] = useState(false);
  const [props, setProps] = useState<ResultRow | null>(null);
  const [showFamilies, setShowFamilies] = useState(false);

  // SHOW COMPUTE POOL INSTANCE FAMILIES: fetched once per dialog open, on first
  // need, and shared by the create picker, Properties and the reference table.
  // A failed fetch clears the cached promise, so the next need retries.
  const [famRes, setFamRes] = useState<snowflake.QueryResult | null>(null);
  const [famError, setFamError] = useState<string | null>(null);
  const famPromise = useRef<Promise<ResultRow[]> | null>(null);
  const loadFamilies = useCallback(() => {
    if (!famPromise.current) {
      setFamError(null);
      famPromise.current = ListComputePoolInstanceFamilies()
        .then((r) => { setFamRes(r); return rowsOf(r); })
        .catch((e) => { famPromise.current = null; setFamError(friendlyError(e)); throw e; });
    }
    return famPromise.current;
  }, []);
  const needFamilies = () => { loadFamilies().catch(() => {}); };
  const families = rowsOf(famRes);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(rowsOf(await ListComputePoolsDetailed()));
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
      return;
    }
    message.success(what);
    load();
  };

  const confirmLifecycle = (r: ResultRow, resume: boolean) => modal.confirm({
    title: `${resume ? "Resume" : "Suspend"} compute pool ${r.name}?`,
    content: resume
      ? "Snowflake provisions MIN_NODES nodes and resumes the pool's services."
      : "Services on the pool are suspended and running jobs are terminated. Its nodes are released until it resumes.",
    okText: resume ? "Resume" : "Suspend",
    onOk: () => run(`${r.name} ${resume ? "resumed" : "suspended"}`, () => AlterComputePool(r.name, resume ? "RESUME" : "SUSPEND")),
  });

  // Workload types go into the SQL unquoted, so the backend rejects anything but
  // bare words; checking here keeps a typo from closing the confirm first.
  const confirmStopAll = (r: ResultRow) => {
    let types = "";
    modal.confirm({
      title: `Stop all services on ${r.name}?`,
      content: (
        <Space direction="vertical" style={{ width: "100%" }}>
          <span>Every service and job running on the pool is terminated. The pool itself keeps running.</span>
          <Input
            placeholder="Only these workload types (optional, comma-separated)"
            title="Snowflake workload type names — letters, digits and underscores"
            onChange={(e) => { types = e.target.value; }}
          />
        </Space>
      ),
      okText: "Stop all",
      okButtonProps: { danger: true },
      onOk: () => {
        const list = types.split(",").map((t) => t.trim()).filter(Boolean);
        const bad = list.find((t) => !/^[A-Za-z_][A-Za-z0-9_]*$/.test(t));
        if (bad) {
          message.error(`"${bad}" is not a workload type name (letters, digits, underscores).`);
          return Promise.reject(); // keeps the confirm open
        }
        return run(`Stopped all services on ${r.name}`, () => StopAllComputePoolServices(r.name, list));
      },
    });
  };

  const confirmDrop = (r: ResultRow) => modal.confirm({
    title: `Drop compute pool ${r.name}?`,
    content: "The pool and its nodes are deleted; this cannot be undone. Snowflake refuses while services run on it — "
      + "Stop all services first, which terminates them.",
    okText: "Drop",
    okButtonProps: { danger: true },
    onOk: () => run(`Dropped ${r.name}`, () => DropComputePool(r.name)),
  });

  const columns: ColumnsType<ResultRow> = [
    {
      title: "State", dataIndex: "state", width: 110,
      render: (s: string) => (
        <Space size={6}>
          <span style={{ display: "inline-block", width: 8, height: 8, borderRadius: 4, background: stateColor(s) }} />
          {s}
        </Space>
      ),
    },
    { title: "Name", dataIndex: "name", ellipsis: true },
    {
      title: "Nodes", width: 110,
      render: (_, r) => `${r.active_nodes || "0"} (${r.min_nodes}–${r.max_nodes})`,
    },
    { title: "Instance family", dataIndex: "instance_family", ellipsis: true },
    { title: "Auto-resume", dataIndex: "auto_resume", width: 100 },
    { title: "Auto-suspend", dataIndex: "auto_suspend_secs", width: 105, render: (v: string) => (v ? `${v}s` : "") },
    { title: "Owner", dataIndex: "owner", ellipsis: true },
    { title: "Application", dataIndex: "application", ellipsis: true },
    { title: "Comment", dataIndex: "comment", ellipsis: true },
    {
      key: "menu", width: 40,
      render: (_, r) => {
        const suspended = r.state?.toUpperCase() === "SUSPENDED";
        return (
          <Dropdown
            trigger={["click"]}
            menu={{
              items: [
                { key: "props", label: "Properties…", onClick: () => { needFamilies(); setProps(r); } },
                { key: "life", label: suspended ? "Resume" : "Suspend", onClick: () => confirmLifecycle(r, suspended) },
                { key: "stop", label: "Stop all services…", danger: true, onClick: () => confirmStopAll(r) },
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

  const states = [...new Set(rows.map((r) => r.state).filter(Boolean))];

  return (
    <>
      <ContainerTabLayout<ResultRow>
        objectLabel="compute pools"
        rowKey="name"
        columns={columns}
        rows={stateFilter ? rows.filter((r) => r.state === stateFilter) : rows}
        loading={loading}
        error={error}
        onRefresh={load}
        newLabel="New compute pool…"
        onNew={() => { needFamilies(); setCreating(true); }}
        facets={
          <>
            <Select
              size="small" allowClear placeholder="State" style={{ width: 130 }}
              value={stateFilter} onChange={setStateFilter}
              options={states.map((s) => ({ value: s, label: s }))}
            />
            <Button size="small" icon={<TableOutlined />} onClick={() => { needFamilies(); setShowFamilies(true); }}>
              Instance families…
            </Button>
          </>
        }
        filter={(r, q) => [r.name, r.instance_family, r.owner, r.application, r.comment].some((v) => v?.toLowerCase().includes(q))}
        detail={(r) => (
          <Descriptions size="small" bordered column={3} styles={{ label: { fontSize: 11 }, content: { fontSize: 12 } }}>
            {Object.entries(r).filter(([, v]) => v !== "").map(([k, v]) => (
              <Descriptions.Item key={k} label={k}>{v}</Descriptions.Item>
            ))}
          </Descriptions>
        )}
      />

      {creating && (
        <CreateComputePoolModal
          families={families} familiesLoading={!famRes && !famError}
          onShowFamilies={() => setShowFamilies(true)}
          onClose={() => setCreating(false)} onSuccess={load}
        />
      )}
      {props && (
        <ComputePoolPropertiesModal
          // Live row, so an edit's refreshed SHOW columns reach the modal.
          name={props.name} showRow={rows.find((r) => r.name === props.name) ?? props} loadFamilies={loadFamilies}
          onClose={() => setProps(null)} onChanged={load}
        />
      )}
      {showFamilies && <InstanceFamiliesModal result={famRes} error={famError} onClose={() => setShowFamilies(false)} />}
    </>
  );
}
