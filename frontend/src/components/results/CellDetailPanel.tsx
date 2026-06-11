// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: SQL Editor & Diagnostics

import React, { useEffect, useMemo, useRef, useState } from "react";
import { Button, Tooltip, message } from "antd";
import { CloseOutlined, CopyOutlined } from "@ant-design/icons";
import { useGridStore } from "../../store/gridStore";
import { usePanelLayoutStore } from "../../store/panelLayoutStore";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";

const MIN_WIDTH = 200;
const MAX_WIDTH = 700;

interface Props {
  /** Column names of the displayed result (original order, matching selection indices). */
  columns: string[];
  /** Called after the panel opens or switches to a different cell, so the
   *  parent can scroll the grid to keep that cell visible (the panel shrinks
   *  the grid viewport and can otherwise cover the selected cell). */
  onVisibleCellChange?: (rowIndex: number, colIndex: number) => void;
}

/**
 * Side panel showing the full content of the selected cell (the selection
 * anchor — the cell first clicked). Reads `selectionRange` and `tableRows`
 * from the gridStore singleton, so it must only be rendered alongside the
 * primary (non-standalone) ResultGrid.
 *
 * Opens when a cell is selected; closes via the ✕ button or Escape, and
 * reopens when a different cell is selected. Gated behind the
 * `cellDetailPanel` feature flag by the parent (QueryPage).
 */
export default function CellDetailPanel({ columns, onVisibleCellChange }: Props) {
  const selectionRange = useGridStore((s) => s.selectionRange);
  const tableRows = useGridStore((s) => s.tableRows);
  const width = usePanelLayoutStore((s) => s.cellDetailWidth);
  const setWidth = usePanelLayoutStore((s) => s.setCellDetailWidth);

  // The anchor cell is selectionRange.start* — stable while dragging a range,
  // changes when the user clicks a different cell.
  const anchorKey = selectionRange ? `${selectionRange.startRow}:${selectionRange.startCol}` : null;

  // Explicit close suppresses the panel for the current anchor only; selecting
  // a different cell reopens it.
  const [dismissedKey, setDismissedKey] = useState<string | null>(null);
  const visible = anchorKey !== null && anchorKey !== dismissedKey;

  const [showRaw, setShowRaw] = useState(false);

  const cell = useMemo(() => {
    if (!selectionRange || !tableRows) return null;
    const row = tableRows[selectionRange.startRow];
    if (!row) return null;
    const colIdx = selectionRange.startCol;
    if (colIdx < 0 || colIdx >= columns.length) return null;
    return {
      columnName: columns[colIdx],
      rowNumber: selectionRange.startRow + 1,
      value: row.original[colIdx],
    };
  }, [selectionRange, tableRows, columns]);

  const rawText = cell == null || cell.value == null ? null : String(cell.value);

  // Pretty-print JSON values; null when the value isn't parseable JSON.
  const prettyJson = useMemo(() => {
    if (rawText == null) return null;
    const t = rawText.trim();
    if (!t.startsWith("{") && !t.startsWith("[")) return null;
    try {
      const formatted = JSON.stringify(JSON.parse(t), null, 2);
      return formatted === t ? null : formatted;
    } catch {
      return null;
    }
  }, [rawText]);

  const displayText = prettyJson !== null && !showRaw ? prettyJson : rawText;

  // Keep the selected cell visible when the panel opens or switches cells.
  // The callback lives in a ref and the effect keys on visible/anchorKey only,
  // so unrelated parent re-renders don't re-scroll and fight user scrolling.
  const onVisibleCellChangeRef = useRef(onVisibleCellChange);
  onVisibleCellChangeRef.current = onVisibleCellChange;
  useEffect(() => {
    if (!visible || !anchorKey) return;
    const [row, col] = anchorKey.split(":").map(Number);
    onVisibleCellChangeRef.current?.(row, col);
  }, [visible, anchorKey]);

  // Close on Escape
  useEffect(() => {
    if (!visible) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      // Don't steal Escape from inputs (e.g. the grid search box)
      const tag = (document.activeElement as HTMLElement | null)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") return;
      setDismissedKey(anchorKey);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [visible, anchorKey]);

  const onResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = width;
    const onMove = (ev: MouseEvent) => {
      setWidth(Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, startW + (startX - ev.clientX))));
    };
    const onUp = () => {
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  };

  if (!visible || !cell) return null;

  const copyValue = async () => {
    await ClipboardSetText(rawText ?? "");
    message.success("Cell value copied");
  };

  return (
    <div
      style={{
        width,
        flexShrink: 0,
        display: "flex",
        flexDirection: "column",
        borderLeft: "1px solid var(--border)",
        background: "var(--bg-raised)",
        position: "relative",
        overflow: "hidden",
      }}
    >
      {/* Resize handle */}
      <div
        onMouseDown={onResizeStart}
        style={{
          position: "absolute",
          left: 0,
          top: 0,
          bottom: 0,
          width: 4,
          cursor: "col-resize",
          zIndex: 1,
        }}
        onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
        onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      />

      {/* Header */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          padding: "4px 6px 4px 10px",
          borderBottom: "1px solid var(--border)",
          flexShrink: 0,
        }}
      >
        <span
          title={cell.columnName}
          style={{
            fontWeight: 600,
            fontSize: 11,
            color: "var(--text)",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            flex: 1,
          }}
        >
          {cell.columnName}
        </span>
        <span style={{ fontSize: 10, color: "var(--text-faint)", flexShrink: 0 }}>
          Row {cell.rowNumber.toLocaleString()}
        </span>
        <Tooltip title="Copy cell value">
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
            style={{ height: 18, padding: "0 4px", minWidth: 0 }}
            onClick={copyValue}
          />
        </Tooltip>
        <Tooltip title="Close (Esc)">
          <Button
            type="text"
            size="small"
            icon={<CloseOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
            style={{ height: 18, padding: "0 4px", minWidth: 0 }}
            onClick={() => setDismissedKey(anchorKey)}
          />
        </Tooltip>
      </div>

      {/* Content */}
      <div
        style={{
          flex: 1,
          overflow: "auto",
          padding: "8px 10px",
        }}
      >
        {displayText === null ? (
          <span style={{ color: "var(--text-faint)", fontStyle: "italic", fontSize: 11, letterSpacing: "0.04em" }}>
            NULL
          </span>
        ) : (
          <pre
            style={{
              margin: 0,
              fontFamily: "var(--mono-font, ui-monospace, 'SF Mono', Menlo, monospace)",
              fontSize: 11,
              lineHeight: 1.5,
              color: "var(--text)",
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
              userSelect: "text",
              WebkitUserSelect: "text",
              cursor: "text",
            }}
          >
            {displayText}
          </pre>
        )}
      </div>

      {/* Footer */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "3px 10px",
          borderTop: "1px solid var(--border)",
          fontSize: 10,
          color: "var(--text-faint)",
          flexShrink: 0,
        }}
      >
        <span>{rawText === null ? "NULL" : `${rawText.length.toLocaleString()} chars`}</span>
        {prettyJson !== null && (
          <span
            role="button"
            onClick={() => setShowRaw((v) => !v)}
            style={{ marginLeft: "auto", cursor: "pointer", color: "var(--accent)", userSelect: "none" }}
          >
            {showRaw ? "Formatted JSON" : "Raw"}
          </span>
        )}
      </div>
    </div>
  );
}
