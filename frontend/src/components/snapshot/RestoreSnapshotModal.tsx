// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { UndropSnapshot } from "../../../wailsjs/go/app/App";
import { friendlyError } from "../common/errors";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: (name: string) => void;
}

/**
 * Name prompt for UNDROP SNAPSHOT. There is no SHOW SNAPSHOTS HISTORY to list
 * dropped snapshots from, so the user types the name.
 */
export default function RestoreSnapshotModal({ db, schema, onClose, onSuccess }: Props) {
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const restore = async () => {
    setBusy(true);
    setError(null);
    try {
      await UndropSnapshot(db, schema, name.trim());
      onSuccess?.(name.trim());
      onClose();
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal open title={`Restore dropped snapshot in ${db}.${schema}`} width={460}
      okText="Restore" okButtonProps={{ disabled: !name.trim(), loading: busy }}
      onOk={restore} onCancel={onClose}>
      <Form layout="vertical" size="small">
        <Form.Item label="Snapshot name" help="Exact name as shown before it was dropped (case-sensitive).">
          <Input autoFocus value={name} onChange={(e) => setName(e.target.value)} onPressEnter={() => name.trim() && restore()} />
        </Form.Item>
      </Form>
      {error && <Alert type="error" showIcon message={error} style={{ marginTop: 12 }} />}
    </Modal>
  );
}
