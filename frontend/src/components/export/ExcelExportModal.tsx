// SPDX-License-Identifier: GPL-3.0-or-later

// @thaw-domain: SQL Editor & Diagnostics

import { useMemo, useState } from "react";
import { Modal, Checkbox, Typography, Space, Button } from "antd";
import { deriveSheetName } from "../../utils/excelSheetName";

const { Text } = Typography;

// One selectable resultset from the tab's result history.
export interface ExcelExportEntry {
  id: string;       // HistoryEntry id
  index: number;    // 1-based resultset number shown in the history dropdown
  sql: string;      // the query that produced the result
  rowCount: number; // number of rows in the result
  pinned: boolean;
}

interface Props {
  open: boolean;
  entries: ExcelExportEntry[];
  onCancel: () => void;
  // Called with the selected ids in the order they should become sheets.
  onExport: (ids: string[]) => void;
}

const snippet = (s: string) => {
  const n = s.replace(/\s+/g, " ").trim();
  return n.length > 60 ? n.slice(0, 60) + "…" : n;
};

// Modal that lets the user pick which resultsets from the tab's history to
// export into a single multi-sheet .xlsx file (one sheet per resultset). The
// derived sheet name — 31-char capped, invalid chars stripped, de-duplicated —
// is previewed next to each entry so the outcome is visible before exporting.
export default function ExcelExportModal({ open, entries, onCancel, onExport }: Props) {
  // Default to every resultset selected, most-recent-first (entries arrive in
  // display order). Keyed by id so pins/reorders don't disturb the selection.
  const [selected, setSelected] = useState<Set<string>>(() => new Set(entries.map((e) => e.id)));

  // Sheet-name previews, computed in selection order so the de-dup suffixes
  // shown match exactly what the exporter will produce.
  const previews = useMemo(() => {
    const used = new Set<string>();
    const names = new Map<string, string>();
    for (const e of entries) {
      if (selected.has(e.id)) names.set(e.id, deriveSheetName(e.sql, e.index, used));
    }
    return names;
  }, [entries, selected]);

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const allChecked = selected.size === entries.length;
  const noneChecked = selected.size === 0;

  const selectedInOrder = entries.filter((e) => selected.has(e.id)).map((e) => e.id);

  return (
    <Modal
      title="Export resultsets to Excel"
      open={open}
      onCancel={onCancel}
      width={540}
      footer={[
        <Button key="cancel" onClick={onCancel}>Cancel</Button>,
        <Button key="export" type="primary" disabled={noneChecked} onClick={() => onExport(selectedInOrder)}>
          Export {selected.size} sheet{selected.size !== 1 ? "s" : ""}
        </Button>,
      ]}
    >
      <Text type="secondary" style={{ fontSize: 12 }}>
        Each selected resultset becomes its own sheet in a single <code>.xlsx</code> file.
      </Text>
      <div style={{ margin: "10px 0 6px" }}>
        <Checkbox
          indeterminate={!allChecked && !noneChecked}
          checked={allChecked}
          onChange={(e) => setSelected(e.target.checked ? new Set(entries.map((x) => x.id)) : new Set())}
        >
          Select all
        </Checkbox>
      </div>
      <div style={{ maxHeight: 320, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 4 }}>
        {entries.map((e) => (
          <label
            key={e.id}
            style={{
              display: "flex",
              alignItems: "flex-start",
              gap: 8,
              padding: "6px 10px",
              borderBottom: "1px solid var(--border)",
              cursor: "pointer",
            }}
          >
            <Checkbox checked={selected.has(e.id)} onChange={() => toggle(e.id)} style={{ marginTop: 1 }} />
            <div style={{ minWidth: 0, flex: 1 }}>
              <Space size={6} style={{ width: "100%" }}>
                <Text style={{ fontSize: 12, flexShrink: 0 }}>{e.pinned ? "📌 " : ""}#{e.index}</Text>
                <Text style={{ fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {snippet(e.sql) || <em>(no SQL)</em>}
                </Text>
              </Space>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 8, marginTop: 2 }}>
                <Text type="secondary" style={{ fontSize: 11, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {selected.has(e.id) ? <>Sheet: <code>{previews.get(e.id)}</code></> : "—"}
                </Text>
                <Text type="secondary" style={{ fontSize: 11, flexShrink: 0 }}>
                  {e.rowCount} row{e.rowCount !== 1 ? "s" : ""}
                </Text>
              </div>
            </div>
          </label>
        ))}
      </div>
    </Modal>
  );
}
