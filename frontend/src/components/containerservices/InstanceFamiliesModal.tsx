// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { Alert, Button, Modal, Table } from "antd";
import { tableFromResult } from "../shared/LazyResultTable";
import type { snowflake } from "../../../wailsjs/go/models";

/**
 * Read-only SHOW COMPUTE POOL INSTANCE FAMILIES reference. Mostly opened for
 * `current_node_usage` — how many nodes of each family the account is using.
 */
export default function InstanceFamiliesModal({ result, error, onClose }: {
  result: snowflake.QueryResult | null;
  error: string | null;
  onClose: () => void;
}) {
  const t = tableFromResult(result);
  return (
    <Modal
      open title="Compute pool instance families" width={960} onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
    >
      {error && <Alert type="warning" showIcon message="Could not load instance families" description={error} style={{ marginBottom: 8 }} />}
      <Table
        size="small" loading={!result && !error} columns={t.columns} dataSource={t.data}
        pagination={false} scroll={{ x: true, y: 420 }}
      />
    </Modal>
  );
}
