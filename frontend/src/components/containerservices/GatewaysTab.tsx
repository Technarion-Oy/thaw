// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useState } from "react";
import { App as AntApp, Button, Dropdown } from "antd";
import type { ColumnsType } from "antd/es/table";
import { CopyOutlined, MoreOutlined } from "@ant-design/icons";
import { DescribeGateway, DropGateway, ListGateways } from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { friendlyError } from "../common/errors";
import CreateGatewayModal from "../gateway/CreateGatewayModal";
import GatewayPropertiesModal from "../gateway/GatewayPropertiesModal";
import ContainerTabLayout from "./ContainerTabLayout";
import ScopedCreate from "./ScopedCreate";
import { rowsOf, type ResultRow } from "./computePools";

const fqn = (r: ResultRow) => `${r.database_name}.${r.schema_name}.${r.name}`;

/** Gateways tab of the Container Services dialog (issue #943). */
export default function GatewaysTab() {
  const { modal, message } = AntApp.useApp();
  const [rows, setRows] = useState<ResultRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [props, setProps] = useState<ResultRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(rowsOf(await ListGateways()).map((r) => ({ ...r, fqn: fqn(r) })));
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  // SHOW GATEWAYS may omit the ingress URL; DESCRIBE GATEWAY always carries it.
  const copyIngress = async (r: ResultRow) => {
    try {
      const url = r.ingress_url || rowsOf(await DescribeGateway(r.database_name, r.schema_name, r.name))[0]?.ingress_url;
      if (!url) return message.info("This gateway has no ingress URL yet.");
      ClipboardSetText(url);
      message.success("Ingress URL copied");
    } catch (e) {
      message.error(friendlyError(e));
    }
  };

  const confirmDrop = (r: ResultRow) => modal.confirm({
    title: `Drop gateway ${r.name}?`,
    content: "Traffic to its ingress URL stops being routed.",
    okText: "Drop",
    okButtonProps: { danger: true },
    onOk: async () => {
      try {
        await DropGateway(r.database_name, r.schema_name, r.name);
      } catch (e) {
        message.error(friendlyError(e));
        throw e;
      }
      message.success(`Dropped ${r.name}`);
      load();
    },
  });

  const columns: ColumnsType<ResultRow> = [
    { title: "Name", dataIndex: "fqn", ellipsis: true },
    { title: "Ingress URL", dataIndex: "ingress_url", ellipsis: true },
    { title: "PrivateLink URL", dataIndex: "privatelink_ingress_url", ellipsis: true },
    { title: "Owner", dataIndex: "owner", width: 140, ellipsis: true },
    {
      key: "menu", width: 40,
      render: (_, r) => (
        <Dropdown trigger={["click"]} menu={{
          items: [
            { key: "copy", label: "Copy ingress URL", icon: <CopyOutlined />, onClick: () => copyIngress(r) },
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

  return (
    <>
      <ContainerTabLayout<ResultRow>
        objectLabel="gateways" rowKey="fqn" columns={columns} rows={rows}
        loading={loading} error={error} onRefresh={load}
        newLabel="New gateway…" onNew={() => setCreating(true)}
        filter={(r, q) => [r.fqn, r.ingress_url, r.owner, r.comment].some((v) => v?.toLowerCase().includes(q))}
      />
      {creating && (
        <ScopedCreate title="New gateway" onClose={() => setCreating(false)}>
          {(db, schema) => (
            <CreateGatewayModal db={db} schema={schema} onClose={() => setCreating(false)} onSuccess={load} />
          )}
        </ScopedCreate>
      )}
      {props && (
        <GatewayPropertiesModal
          db={props.database_name} schema={props.schema_name} name={props.name}
          onClose={() => { setProps(null); load(); }}
        />
      )}
    </>
  );
}
