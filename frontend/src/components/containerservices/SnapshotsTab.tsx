// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useState } from "react";
import { App as AntApp, Button, Dropdown, Select } from "antd";
import type { ColumnsType } from "antd/es/table";
import { MoreOutlined, UndoOutlined } from "@ant-design/icons";
import { DropSnapshot, ListSnapshots, UndropSnapshot } from "../../../wailsjs/go/app/App";
import { friendlyError } from "../common/errors";
import CreateSnapshotModal from "../snapshot/CreateSnapshotModal";
import RestoreSnapshotModal from "../snapshot/RestoreSnapshotModal";
import SnapshotPropertiesModal from "../snapshot/SnapshotPropertiesModal";
import ContainerTabLayout from "./ContainerTabLayout";
import ScopedCreate from "./ScopedCreate";
import { rowsOf, type ResultRow } from "./computePools";

// SHOW SNAPSHOTS column names are unverified on a live account; read the
// service / instance under their likely aliases.
const service = (r: ResultRow) => r.service_name ?? r.service ?? "";
const instance = (r: ResultRow) => r.instance_id ?? r.instance ?? "";

/** Snapshots tab of the Container Services dialog (issue #945). */
export default function SnapshotsTab() {
  const { modal, message } = AntApp.useApp();
  const [rows, setRows] = useState<ResultRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [svcFilter, setSvcFilter] = useState<string>();
  const [dbFilter, setDbFilter] = useState<string>();
  const [creating, setCreating] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [props, setProps] = useState<ResultRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(rowsOf(await ListSnapshots()).map((r) => ({ ...r, fqn: `${r.database_name}.${r.schema_name}.${r.name}` })));
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  const undo = async (r: ResultRow) => {
    try {
      await UndropSnapshot(r.database_name, r.schema_name, r.name);
      message.success(`Restored ${r.name}`);
      load();
    } catch (e) {
      message.error(friendlyError(e));
    }
  };

  const confirmDrop = (r: ResultRow) => modal.confirm({
    title: `Drop snapshot ${r.name}?`,
    content: "It can be restored with UNDROP SNAPSHOT within the schema's retention period.",
    okText: "Drop",
    okButtonProps: { danger: true },
    onOk: async () => {
      try {
        await DropSnapshot(r.database_name, r.schema_name, r.name);
      } catch (e) {
        message.error(friendlyError(e));
        throw e;
      }
      message.success({
        content: <>Dropped {r.name} <Button size="small" type="link" icon={<UndoOutlined />} onClick={() => undo(r)}>Undo</Button></>,
        duration: 8,
      });
      load();
    },
  });

  const columns: ColumnsType<ResultRow> = [
    { title: "Name", dataIndex: "name", ellipsis: true },
    { title: "Database.Schema", width: 170, ellipsis: true, render: (_, r) => `${r.database_name}.${r.schema_name}` },
    { title: "Service", ellipsis: true, render: (_, r) => service(r) },
    { title: "Volume", dataIndex: "volume_name", width: 110, ellipsis: true },
    { title: "Instance", width: 80, render: (_, r) => instance(r) },
    { title: "Size", dataIndex: "size", width: 90, ellipsis: true },
    { title: "State", dataIndex: "state", width: 100, ellipsis: true },
    { title: "Comment", dataIndex: "comment", ellipsis: true },
    {
      key: "menu", width: 40,
      render: (_, r) => (
        <Dropdown trigger={["click"]} menu={{
          items: [
            { key: "props", label: "Properties…", onClick: () => setProps(r) },
            { type: "divider" },
            { key: "drop", label: "Drop", danger: true, onClick: () => confirmDrop(r) },
          ],
        }}>
          <Button size="small" type="text" icon={<MoreOutlined />} onClick={(e) => e.stopPropagation()} />
        </Dropdown>
      ),
    },
  ];

  const services = [...new Set(rows.map(service).filter(Boolean))];
  const dbs = [...new Set(rows.map((r) => r.database_name).filter(Boolean))];
  const filtered = rows
    .filter((r) => !svcFilter || service(r) === svcFilter)
    .filter((r) => !dbFilter || r.database_name === dbFilter);

  return (
    <>
      <ContainerTabLayout<ResultRow>
        objectLabel="volume snapshots" rowKey="fqn" columns={columns} rows={filtered}
        loading={loading} error={error} onRefresh={load}
        newLabel="New snapshot…" onNew={() => setCreating(true)}
        facets={
          <>
            <Select size="small" allowClear placeholder="Service" style={{ width: 150 }}
              value={svcFilter} onChange={setSvcFilter} options={services.map((s) => ({ value: s, label: s }))} />
            <Select size="small" allowClear placeholder="Database" style={{ width: 150 }}
              value={dbFilter} onChange={setDbFilter} options={dbs.map((d) => ({ value: d, label: d }))} />
            <Button size="small" icon={<UndoOutlined />} onClick={() => setRestoring(true)}>Restore dropped…</Button>
          </>
        }
        filter={(r, q) => [r.fqn, service(r), r.volume_name, r.comment].some((v) => v?.toLowerCase().includes(q))}
      />
      {creating && (
        <ScopedCreate title="New snapshot" onClose={() => setCreating(false)}>
          {(db, schema) => <CreateSnapshotModal db={db} schema={schema} onClose={() => setCreating(false)} onSuccess={load} />}
        </ScopedCreate>
      )}
      {restoring && (
        <ScopedCreate title="Restore dropped snapshot" onClose={() => setRestoring(false)}>
          {(db, schema) => (
            <RestoreSnapshotModal db={db} schema={schema} onClose={() => setRestoring(false)}
              onSuccess={(n) => { message.success(`Restored ${n}`); load(); }} />
          )}
        </ScopedCreate>
      )}
      {props && (
        <SnapshotPropertiesModal db={props.database_name} schema={props.schema_name} name={props.name}
          onClose={() => { setProps(null); load(); }} />
      )}
    </>
  );
}
