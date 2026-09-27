// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useState } from "react";
import { Alert, Button, Modal, Spin, message } from "antd";
import {
  AlterComputePool, AlterComputePoolProperty, DescribeComputePool, ListComputePoolNodes,
} from "../../../wailsjs/go/app/App";
import { EditRow, InfoRow, SECTION_HEAD } from "../common/PropertyRows";
import LazyResultTable from "../shared/LazyResultTable";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";
import { familyListText, familyOptions, rowsOf, type ResultRow } from "./computePools";

interface Props {
  name: string;
  /** The SHOW COMPUTE POOLS row; DESCRIBE output is merged over it. */
  showRow: ResultRow;
  families: ResultRow[];
  onClose: () => void;
  onChanged: () => void;
}

// Settable columns get EditRows below; everything else is read-only.
const EDITABLE = new Set([
  "min_nodes", "max_nodes", "auto_resume", "auto_suspend_secs", "placement_group",
  "instance_family", "backup_instance_families", "comment",
]);

export default function ComputePoolPropertiesModal({ name, showRow, families, onClose, onChanged }: Props) {
  const [row, setRow] = useState<ResultRow | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setRow({ ...showRow, ...(rowsOf(await DescribeComputePool(name))[0] ?? {}) });
    } catch (e) {
      // DESCRIBE failing still leaves the SHOW row to show and edit.
      setRow(showRow);
      setError(String(e));
    }
  }, [name, showRow]);
  useEffect(() => { load(); }, [load]);

  const tags = useObjectTags({
    kind: "COMPUTE POOL", db: "", schema: "", name,
    alter: (clause) => AlterComputePool(name, clause),
  });

  const save = (property: string, label: string) => async (value: string) => {
    await AlterComputePoolProperty(name, property, value);
    message.success(`${label} updated`);
    onChanged();
    await load();
  };

  const edit = (col: string, label: string, property: string, extra: Partial<React.ComponentProps<typeof EditRow>> = {}) => (
    <EditRow label={label} value={row?.[col] ?? ""} type="text" onSave={save(property, label)} {...extra} />
  );

  return (
    <Modal
      open title={`Compute pool: ${name}`} width={720} onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      styles={{ body: { maxHeight: "75vh", overflowY: "auto" } }}
    >
      {error && <Alert type="warning" showIcon message="DESCRIBE COMPUTE POOL failed" description={error} style={{ marginBottom: 8 }} />}
      {!row ? <div style={{ textAlign: "center", padding: 32 }}><Spin /></div> : (
        <>
          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {edit("min_nodes", "MIN_NODES", "minNodes", { type: "number", min: 1 })}
              {edit("max_nodes", "MAX_NODES", "maxNodes", { type: "number", min: 1 })}
              {edit("instance_family", "INSTANCE_FAMILY", "instanceFamily", { type: "select", options: familyOptions(families) })}
              {edit("backup_instance_families", "BACKUP_INSTANCE_FAMILIES", "backupInstanceFamilies", {
                value: familyListText(row.backup_instance_families), hint: "Comma-separated; blank unsets",
              })}
              {edit("auto_resume", "AUTO_RESUME", "autoResume", { type: "boolean" })}
              {edit("auto_suspend_secs", "AUTO_SUSPEND_SECS", "autoSuspendSecs", { type: "number", allowEmpty: true, hint: "Blank unsets" })}
              {edit("placement_group", "PLACEMENT_GROUP", "placementGroup", { hint: "Blank unsets" })}
              {edit("comment", "COMMENT", "comment", { hint: "Blank unsets" })}
              <TagsRow tags={tags.tags} nameOptions={tags.nameOptions} onSetTag={tags.setTag} onUnsetTag={tags.unsetTag} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Status</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {Object.entries(row).filter(([k]) => !EDITABLE.has(k) && k !== "name").map(([k, v]) => (
                <InfoRow key={k} label={k.toUpperCase()} value={v} />
              ))}
            </tbody>
          </table>

          <LazyResultTable title="Nodes" noun="node" load={() => ListComputePoolNodes(name)} />
        </>
      )}
    </Modal>
  );
}
