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
  /** The tab's cached SHOW COMPUTE POOL INSTANCE FAMILIES loader (retries after a failure). */
  loadFamilies: () => Promise<ResultRow[]>;
  onClose: () => void;
  /** Refreshes the SHOW listing; resolves once `showRow` is fresh. */
  onChanged: () => Promise<void>;
}

type EditExtra = Partial<React.ComponentProps<typeof EditRow>>;

// The settable properties, keyed by Snowflake keyword — the same keys as the
// `settable` table behind BuildAlterComputePoolPropertySql (internal/computepool).
// The SHOW/DESCRIBE column is the lower-cased keyword; other columns render read-only.
const SETTABLE: [string, EditExtra][] = [
  ["MIN_NODES", { type: "number", min: 1 }],
  ["MAX_NODES", { type: "number", min: 1 }],
  ["INSTANCE_FAMILY", { type: "select" }],
  ["BACKUP_INSTANCE_FAMILIES", { hint: "Comma-separated; blank unsets" }],
  ["AUTO_RESUME", { type: "boolean" }],
  ["AUTO_SUSPEND_SECS", { type: "number", allowEmpty: true, hint: "Blank unsets" }],
  ["PLACEMENT_GROUP", { hint: "Blank unsets" }],
  ["COMMENT", { hint: "Blank unsets" }],
];
const SETTABLE_COLS = new Set(SETTABLE.map(([kw]) => kw.toLowerCase()));

export default function ComputePoolPropertiesModal({ name, showRow, loadFamilies, onClose, onChanged }: Props) {
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

  const save = (property: string) => async (value: string) => {
    const n = Number(value);
    if (property === "MIN_NODES" && n > Number(row?.max_nodes)) throw new Error("MIN_NODES must be ≤ MAX_NODES");
    if (property === "MAX_NODES" && n < Number(row?.min_nodes)) throw new Error("MAX_NODES must be ≥ MIN_NODES");
    await AlterComputePoolProperty(name, property, value);
    message.success(`${property} updated`);
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
              {SETTABLE.map(([kw, extra]) => {
                const col = kw.toLowerCase();
                const value = col === "backup_instance_families" ? familyListText(row[col]) : row[col] ?? "";
                return (
                  <EditRow
                    key={kw} label={kw} type="text" value={value} onSave={save(kw)} {...extra}
                    // Lazy, with a loading cue; the pool's current family stays
                    // selectable even if the account list omits it.
                    loadOptions={kw === "INSTANCE_FAMILY" ? () => loadFamilies().then((f) => {
                      const opts = familyOptions(f);
                      return !value || opts.some((o) => o.value === value) ? opts : [{ value, label: value }, ...opts];
                    }) : undefined}
                  />
                );
              })}
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
