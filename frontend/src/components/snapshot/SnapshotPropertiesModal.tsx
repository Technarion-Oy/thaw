// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useState } from "react";
import { Alert, Button, Modal, Space, Spin, Typography } from "antd";
import { CameraOutlined } from "@ant-design/icons";
import { AlterSnapshot, DescribeSnapshot } from "../../../wailsjs/go/app/App";
import { EditRow, InfoRow, SECTION_HEAD } from "../common/PropertyRows";
import { rowsOf, type ResultRow } from "../containerservices/computePools";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

/** DESCRIBE SNAPSHOT rows plus the one mutable property, COMMENT. */
export default function SnapshotPropertiesModal({ db, schema, name, onClose }: Props) {
  const [row, setRow] = useState<ResultRow | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setError(null);
      setRow(rowsOf(await DescribeSnapshot(db, schema, name))[0] ?? {});
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);
  useEffect(() => { load(); }, [load]);

  // ALTER SNAPSHOT has no UNSET; a blank comment is SET COMMENT = ''.
  const saveComment = async (v: string) => {
    await AlterSnapshot(db, schema, name, `SET COMMENT = '${v.replace(/'/g, "''")}'`);
    await load();
  };

  return (
    <Modal
      open width={720} onCancel={onClose} footer={<Button onClick={onClose}>Close</Button>}
      title={
        <Space size={6}>
          <CameraOutlined style={{ color: "var(--link)" }} />
          <span>Snapshot Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{`${db}.${schema}.${name}`}</Text>
        </Space>
      }
      styles={{ body: { maxHeight: "75vh", overflowY: "auto" } }}
    >
      {error && <Alert type="error" showIcon message="DESCRIBE SNAPSHOT failed" description={error} />}
      {!row && !error && <div style={{ textAlign: "center", padding: 32 }}><Spin /></div>}
      {row && (
        <>
          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="COMMENT" type="text" value={row.comment ?? ""} onSave={saveComment} />
            </tbody>
          </table>
          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {Object.entries(row).filter(([k]) => k !== "comment").map(([k, v]) => (
                <InfoRow key={k} label={k.toUpperCase()} value={v} />
              ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
