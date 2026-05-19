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

import { useCallback, useEffect, useRef } from "react";
import { Input, Button, Typography } from "antd";
import { CloseOutlined, UpOutlined, DownOutlined, SearchOutlined } from "@ant-design/icons";
import { useGridStore, type CellCoord } from "../../store/gridStore";

const { Text } = Typography;

interface Props {
  columnCount: number;
  onScrollToRow: (rowIndex: number) => void;
  onClose: () => void;
}

export default function GridSearch({ columnCount, onScrollToRow, onClose }: Props) {
  const searchTerm = useGridStore((s) => s.searchTerm);
  const setSearchTerm = useGridStore((s) => s.setSearchTerm);
  const searchMatches = useGridStore((s) => s.searchMatches);
  const setSearchMatches = useGridStore((s) => s.setSearchMatches);
  const currentMatchIndex = useGridStore((s) => s.currentMatchIndex);
  const nextMatch = useGridStore((s) => s.nextMatch);
  const prevMatch = useGridStore((s) => s.prevMatch);
  const inputRef = useRef<any>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  // Focus input on mount
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const tableRows = useGridStore((s) => s.tableRows);

  // Compute matches when search term changes (debounced).
  // Searches over tableRows (filtered/sorted model) so matches align with visible grid indices.
  const computeMatches = useCallback(
    (term: string) => {
      if (!term || !tableRows) {
        setSearchMatches([]);
        return;
      }
      const lower = term.toLowerCase();
      const matches: CellCoord[] = [];
      for (let row = 0; row < tableRows.length; row++) {
        const orig = tableRows[row].original;
        for (let col = 0; col < columnCount; col++) {
          const val = orig[col];
          if (val != null && String(val).toLowerCase().includes(lower)) {
            matches.push({ row, col });
          }
        }
      }
      setSearchMatches(matches);
    },
    [tableRows, columnCount, setSearchMatches],
  );

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => computeMatches(searchTerm), 200);
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [searchTerm, computeMatches]);

  // Scroll to current match
  useEffect(() => {
    if (searchMatches.length > 0 && currentMatchIndex < searchMatches.length) {
      onScrollToRow(searchMatches[currentMatchIndex].row);
    }
  }, [currentMatchIndex, searchMatches, onScrollToRow]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      onClose();
    } else if (e.key === "Enter") {
      if (e.shiftKey) prevMatch();
      else nextMatch();
    }
  };

  const handleClose = () => {
    setSearchTerm("");
    setSearchMatches([]);
    onClose();
  };

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        padding: "4px 8px",
        background: "var(--bg-raised)",
        borderBottom: "1px solid var(--border)",
        flexShrink: 0,
      }}
    >
      <SearchOutlined style={{ fontSize: 12, color: "var(--text-muted)" }} />
      <Input
        ref={inputRef}
        size="small"
        placeholder="Search results..."
        value={searchTerm}
        onChange={(e) => setSearchTerm(e.target.value)}
        onKeyDown={handleKeyDown}
        style={{ width: 200, fontSize: 11 }}
        allowClear
      />
      {searchMatches.length > 0 && (
        <Text style={{ fontSize: 11, color: "var(--text-muted)", whiteSpace: "nowrap" }}>
          {currentMatchIndex + 1} of {searchMatches.length}
        </Text>
      )}
      {searchTerm && searchMatches.length === 0 && (
        <Text style={{ fontSize: 11, color: "var(--text-faint)", whiteSpace: "nowrap" }}>
          No matches
        </Text>
      )}
      <Button
        type="text"
        size="small"
        icon={<UpOutlined style={{ fontSize: 10 }} />}
        disabled={searchMatches.length === 0}
        onClick={prevMatch}
        style={{ height: 20, padding: "0 4px", minWidth: 0 }}
      />
      <Button
        type="text"
        size="small"
        icon={<DownOutlined style={{ fontSize: 10 }} />}
        disabled={searchMatches.length === 0}
        onClick={nextMatch}
        style={{ height: 20, padding: "0 4px", minWidth: 0 }}
      />
      <Button
        type="text"
        size="small"
        icon={<CloseOutlined style={{ fontSize: 10 }} />}
        onClick={handleClose}
        style={{ height: 20, padding: "0 4px", minWidth: 0 }}
      />
    </div>
  );
}
