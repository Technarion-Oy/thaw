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
  /** The current SHOW COMPUTE POOLS row; DESCRIBE output is merged over it. The
   *  parent re-passes it after `onChanged`, since DESCRIBE omits some columns
   *  (e.g. backup_instance_families). */
  showRow: ResultRow;
  families: ResultRow[];
  onClose: () => void;
  /** Refreshes the SHOW listing; resolves once `showRow` is fresh. */
  onChanged: () => Promise<void>;
}

type EditExtra = Partial<React.ComponentProps<typeof EditRow>>;

// The settable properties: SHOW/DESCRIBE column, ALTER keyword, and the property
// key BuildAlterComputePoolPropertySql (internal/computepool) accepts. Columns
// not listed here render read-only.
const SETTABLE: { col: string; label: string; property: string; extra?: (fams: ResultRow[]) => EditExtra }[] = [
  { col: "min_nodes", label: "MIN_NODES", property: "minNodes", extra: () => ({ type: "number", min: 1 }) },
  { col: "max_nodes", label: "MAX_NODES", property: "maxNodes", extra: () => ({ type: "number", min: 1 }) },
  { col: "instance_family", label: "INSTANCE_FAMILY", property: "instanceFamily", extra: (f) => ({ type: "select", options: familyOptions(f) }) },
  { col: "backup_instance_families", label: "BACKUP_INSTANCE_FAMILIES", property: "backupInstanceFamilies", extra: () => ({ hint: "Comma-separated; blank unsets" }) },
  { col: "auto_resume", label: "AUTO_RESUME", property: "autoResume", extra: () => ({ type: "boolean" }) },
  { col: "auto_suspend_secs", label: "AUTO_SUSPEND_SECS", property: "autoSuspendSecs", extra: () => ({ type: "number", allowEmpty: true, hint: "Blank unsets" }) },
  { col: "placement_group", label: "PLACEMENT_GROUP", property: "placementGroup", extra: () => ({ hint: "Blank unsets" }) },
  { col: "comment", label: "COMMENT", property: "comment", extra: () => ({ hint: "Blank unsets" }) },
];
const SETTABLE_COLS = new Set(SETTABLE.map((s) => s.col));

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
    const n = Number(value);
    if (property === "minNodes" && n > Number(row?.max_nodes)) throw new Error("MIN_NODES must be ≤ MAX_NODES");
    if (property === "maxNodes" && n < Number(row?.min_nodes)) throw new Error("MAX_NODES must be ≥ MIN_NODES");
    await AlterComputePoolProperty(name, property, value);
    message.success(`${label} updated`);
    await onChanged(); // new showRow → load() re-runs
  };

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
              {SETTABLE.map((s) => (
                <EditRow
                  key={s.col} label={s.label} type="text" onSave={save(s.property, s.label)}
                  value={s.col === "backup_instance_families" ? familyListText(row[s.col]) : row[s.col] ?? ""}
                  {...s.extra?.(families)}
                />
              ))}
              <TagsRow tags={tags.tags} nameOptions={tags.nameOptions} onSetTag={tags.setTag} onUnsetTag={tags.unsetTag} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Status</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {Object.entries(row).filter(([k]) => !SETTABLE_COLS.has(k) && k !== "name").map(([k, v]) => (
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
