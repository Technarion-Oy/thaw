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

import { useCallback, useEffect, useRef, useState } from "react";
import { Input, Button, Typography, Tooltip } from "antd";
import type { InputRef } from "antd";
import {
  CloseOutlined,
  UpOutlined,
  DownOutlined,
  SearchOutlined,
  SwapOutlined,
} from "@ant-design/icons";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;

// ── Types ────────────────────────────────────────────────────────────────────

interface MatchLocation {
  tabId: string;
  tabTitle: string;
  isNotebook: boolean;
  line: number;     // 1-based within the text (or cell for notebooks)
  column: number;   // 1-based
  length: number;
  preview: string;
  cellIndex?: number; // 0-based, for notebook cells only
}

interface Props {
  onClose: () => void;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Extract cell sources from a serialized Jupyter notebook. */
function getNotebookCellSources(json: string): Array<{ index: number; source: string }> {
  try {
    const nb = JSON.parse(json);
    if (!Array.isArray(nb.cells)) return [];
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return nb.cells.map((cell: any, i: number) => {
      const src = Array.isArray(cell.source)
        ? cell.source.join("")
        : (cell.source ?? "");
      return { index: i, source: src };
    });
  } catch {
    return [];
  }
}

/**
 * Replace a match at a specific position within a single notebook cell and
 * return the re-serialised notebook JSON.
 */
function replaceInNotebookCell(
  json: string,
  cellIndex: number,
  replaceText: string,
  line: number,   // 1-based within the cell
  column: number, // 1-based
  matchLength: number,
): string {
  try {
    const nb = JSON.parse(json);
    if (!Array.isArray(nb.cells) || cellIndex >= nb.cells.length) return json;
    const cell = nb.cells[cellIndex];
    let src = Array.isArray(cell.source)
      ? cell.source.join("")
      : (cell.source ?? "");

    const lines = src.split("\n");
    if (line - 1 >= lines.length) return json;
    const lineStr = lines[line - 1];
    lines[line - 1] =
      lineStr.substring(0, column - 1) +
      replaceText +
      lineStr.substring(column - 1 + matchLength);
    src = lines.join("\n");

    reserializeCellSource(cell, src);
    return JSON.stringify(nb, null, 1);
  } catch {
    return json;
  }
}

/** Re-serialize a cell's source string as an array of lines (Jupyter convention). */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function reserializeCellSource(cell: any, src: string): void {
  const srcLines = src.split("\n");
  cell.source = srcLines.map((l: string, i: number) =>
    i < srcLines.length - 1 ? l + "\n" : l,
  );
  if (cell.source.length > 1 && cell.source[cell.source.length - 1] === "") {
    cell.source.pop();
  }
}

/**
 * Apply all match replacements across a notebook's cells in a single JSON
 * parse/serialize cycle (avoids O(n) JSON round-trips per match).
 */
function replaceAllInNotebook(
  json: string,
  tabMatches: MatchLocation[],
  searchTerm: string,
  replaceTerm: string,
  useRegex: boolean,
  caseSensitive: boolean,
): string {
  try {
    const nb = JSON.parse(json);
    if (!Array.isArray(nb.cells)) return json;

    // Group matches by cell index.
    const byCell = new Map<number, MatchLocation[]>();
    for (const m of tabMatches) {
      if (m.cellIndex == null) continue;
      const list = byCell.get(m.cellIndex) ?? [];
      list.push(m);
      byCell.set(m.cellIndex, list);
    }

    for (const [cellIdx, cellMatches] of byCell) {
      if (cellIdx >= nb.cells.length) continue;
      const cell = nb.cells[cellIdx];
      let src = Array.isArray(cell.source)
        ? cell.source.join("")
        : (cell.source ?? "");

      if (useRegex) {
        // Regex mode: String.prototype.replace handles capture-group expansion.
        try {
          const regex = new RegExp(searchTerm, caseSensitive ? "g" : "gi");
          const lines = src.split("\n");
          for (let i = 0; i < lines.length; i++) {
            lines[i] = lines[i].replace(regex, replaceTerm);
          }
          src = lines.join("\n");
        } catch { continue; }
      } else {
        // Literal mode: positional replacement in reverse order.
        const sorted = [...cellMatches].sort((a, b) =>
          a.line !== b.line ? b.line - a.line : b.column - a.column,
        );
        const lines = src.split("\n");
        for (const m of sorted) {
          if (m.line - 1 >= lines.length) continue;
          const lineStr = lines[m.line - 1];
          lines[m.line - 1] =
            lineStr.substring(0, m.column - 1) +
            replaceTerm +
            lineStr.substring(m.column - 1 + m.length);
        }
        src = lines.join("\n");
      }

      reserializeCellSource(cell, src);
    }

    return JSON.stringify(nb, null, 1);
  } catch {
    return json;
  }
}

// ── Component ────────────────────────────────────────────────────────────────

export default function CrossTabSearch({ onClose }: Props) {
  const [searchTerm, setSearchTerm] = useState("");
  const [replaceTerm, setReplaceTerm] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [useRegex, setUseRegex] = useState(false);
  const [showReplace, setShowReplace] = useState(false);
  const [matches, setMatches] = useState<MatchLocation[]>([]);
  const [currentIdx, setCurrentIdx] = useState(0);
  const searchRef = useRef<InputRef>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  // Focus the search input on mount.
  useEffect(() => {
    searchRef.current?.focus();
  }, []);

  // ── Search ─────────────────────────────────────────────────────────────────

  const computeMatches = useCallback(
    (term: string) => {
      if (!term) {
        setMatches([]);
        setCurrentIdx(0);
        return;
      }

      let regex: RegExp;
      try {
        const flags = caseSensitive ? "g" : "gi";
        regex = useRegex
          ? new RegExp(term, flags)
          : new RegExp(escapeRegExp(term), flags);
      } catch {
        // Invalid regex — treat as "no matches".
        setMatches([]);
        setCurrentIdx(0);
        return;
      }

      const tabs = useQueryStore.getState().tabs;
      const results: MatchLocation[] = [];

      for (const tab of tabs) {
        if (tab.diff) continue; // skip diff tabs

        const isNotebook = (tab.kind ?? "sql") === "notebook";

        if (isNotebook) {
          const cells = getNotebookCellSources(tab.sql);
          for (const cell of cells) {
            if (!cell.source) continue;
            searchText(cell.source, regex, (line, column, length, preview) => {
              results.push({
                tabId: tab.id,
                tabTitle: tab.title,
                isNotebook: true,
                line,
                column,
                length,
                preview,
                cellIndex: cell.index,
              });
            });
          }
        } else {
          searchText(tab.sql, regex, (line, column, length, preview) => {
            results.push({
              tabId: tab.id,
              tabTitle: tab.title,
              isNotebook: false,
              line,
              column,
              length,
              preview,
            });
          });
        }
      }

      setMatches(results);
      setCurrentIdx((prev) => (prev < results.length ? prev : 0));
    },
    [caseSensitive, useRegex],
  );

  // Debounced search when term / options change.
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => computeMatches(searchTerm), 150);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [searchTerm, computeMatches]);

  // ── Navigation ─────────────────────────────────────────────────────────────

  const goToMatch = useCallback(
    (idx: number) => {
      if (idx < 0 || idx >= matches.length) return;
      const m = matches[idx];
      const { activeTabId, activateTab } = useQueryStore.getState();
      const needsSwitch = m.tabId !== activeTabId;

      if (needsSwitch) activateTab(m.tabId);

      // Reuse the existing thaw:scroll-to-line event that SqlEditor already
      // handles.  For notebook tabs the event is harmless (no editor listener).
      const emit = () => {
        window.dispatchEvent(
          new CustomEvent("thaw:scroll-to-line", {
            detail: {
              line: m.line,
              matchStart: m.column - 1,           // 0-based for SqlEditor
              matchEnd: m.column - 1 + m.length,  // 0-based, exclusive
            },
          }),
        );
      };

      // Allow time for the editor to mount after a tab switch.
      if (needsSwitch) setTimeout(emit, 150);
      else emit();
    },
    [matches],
  );

  const nextMatch = useCallback(() => {
    if (matches.length === 0) return;
    const next = (currentIdx + 1) % matches.length;
    setCurrentIdx(next);
    goToMatch(next);
  }, [matches, currentIdx, goToMatch]);

  const prevMatch = useCallback(() => {
    if (matches.length === 0) return;
    const prev = (currentIdx - 1 + matches.length) % matches.length;
    setCurrentIdx(prev);
    goToMatch(prev);
  }, [matches, currentIdx, goToMatch]);

  // Auto-navigate to the first match when search results change so the
  // counter ("1 of N") is accurate — without this, the editor wouldn't
  // reveal/highlight anything until the user explicitly presses Enter.
  useEffect(() => {
    if (matches.length > 0) {
      goToMatch(0);
    }
  }, [matches, goToMatch]);

  // ── Replace ────────────────────────────────────────────────────────────────

  const applyReplace = useCallback(
    (tabId: string, newContent: string) => {
      const store = useQueryStore.getState();
      store.setSqlForTab(tabId, newContent);
      // setSqlForTab only patches the tabs array — the flat `sql` alias must
      // be updated separately when replacing in the active tab.
      if (tabId === store.activeTabId) {
        useQueryStore.setState({ sql: newContent });
      }
    },
    [],
  );

  const replaceCurrent = useCallback(() => {
    if (matches.length === 0 || currentIdx >= matches.length) return;
    const m = matches[currentIdx];
    const tab = useQueryStore.getState().tabs.find((t) => t.id === m.tabId);
    if (!tab) return;

    // Resolve the source lines for the match (cell source for notebooks,
    // full SQL for regular tabs) so we can extract the matched substring.
    let sourceLines: string[];
    if (m.isNotebook && m.cellIndex != null) {
      const cells = getNotebookCellSources(tab.sql);
      const cell = cells.find((c) => c.index === m.cellIndex);
      if (!cell) return;
      sourceLines = cell.source.split("\n");
    } else {
      sourceLines = tab.sql.split("\n");
    }
    if (m.line - 1 >= sourceLines.length) return;

    // In regex mode, expand capture-group back-references ($1, $2, etc.)
    // by running the replacement through String.prototype.replace.
    let effectiveReplace = replaceTerm;
    if (useRegex) {
      try {
        const matched = sourceLines[m.line - 1].substring(m.column - 1, m.column - 1 + m.length);
        const re = new RegExp(searchTerm, caseSensitive ? "" : "i");
        effectiveReplace = matched.replace(re, replaceTerm);
      } catch { /* fall back to literal replaceTerm */ }
    }

    if (m.isNotebook && m.cellIndex != null) {
      const newJson = replaceInNotebookCell(
        tab.sql, m.cellIndex, effectiveReplace, m.line, m.column, m.length,
      );
      applyReplace(m.tabId, newJson);
    } else {
      const lines = tab.sql.split("\n");
      const lineStr = lines[m.line - 1];
      if (lineStr == null) return;
      lines[m.line - 1] =
        lineStr.substring(0, m.column - 1) +
        effectiveReplace +
        lineStr.substring(m.column - 1 + m.length);
      applyReplace(m.tabId, lines.join("\n"));
    }

    // Recompute after the store update propagates.
    setTimeout(() => computeMatches(searchTerm), 50);
  }, [matches, currentIdx, replaceTerm, useRegex, searchTerm, caseSensitive, applyReplace, computeMatches]);

  const replaceAll = useCallback(() => {
    if (matches.length === 0) return;

    // Group matches by tab.
    const byTab = new Map<string, MatchLocation[]>();
    for (const m of matches) {
      const list = byTab.get(m.tabId) ?? [];
      list.push(m);
      byTab.set(m.tabId, list);
    }

    for (const [tabId, tabMatches] of byTab) {
      const tab = useQueryStore.getState().tabs.find((t) => t.id === tabId);
      if (!tab) continue;

      if (tabMatches[0]?.isNotebook) {
        // Single JSON parse/serialize for all cell replacements.
        const newJson = replaceAllInNotebook(
          tab.sql, tabMatches, searchTerm, replaceTerm, useRegex, caseSensitive,
        );
        applyReplace(tabId, newJson);
      } else if (useRegex) {
        // Regex mode: String.prototype.replace handles capture-group expansion.
        try {
          const regex = new RegExp(searchTerm, caseSensitive ? "g" : "gi");
          const lines = tab.sql.split("\n");
          for (let i = 0; i < lines.length; i++) {
            lines[i] = lines[i].replace(regex, replaceTerm);
          }
          applyReplace(tabId, lines.join("\n"));
        } catch { /* invalid regex — skip */ }
      } else {
        // Literal mode: positional replacement in reverse line/column order.
        const sorted = [...tabMatches].sort((a, b) =>
          a.line !== b.line ? b.line - a.line : b.column - a.column,
        );
        const lines = tab.sql.split("\n");
        for (const m of sorted) {
          const lineStr = lines[m.line - 1];
          if (lineStr == null) continue;
          lines[m.line - 1] =
            lineStr.substring(0, m.column - 1) +
            replaceTerm +
            lineStr.substring(m.column - 1 + m.length);
        }
        applyReplace(tabId, lines.join("\n"));
      }
    }

    setTimeout(() => computeMatches(searchTerm), 50);
  }, [matches, replaceTerm, useRegex, searchTerm, caseSensitive, applyReplace, computeMatches]);

  // ── Keyboard handlers ──────────────────────────────────────────────────────

  const handleSearchKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") onClose();
    else if (e.key === "Enter" && e.shiftKey) prevMatch();
    else if (e.key === "Enter") nextMatch();
  };

  const handleReplaceKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") onClose();
    else if (e.key === "Enter") replaceCurrent();
  };

  const handleClose = () => {
    setSearchTerm("");
    setReplaceTerm("");
    setMatches([]);
    onClose();
  };

  // ── Derived state ──────────────────────────────────────────────────────────

  const tabCount = new Set(matches.map((m) => m.tabId)).size;

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <div
      style={{
        background: "var(--bg-raised)",
        borderBottom: "1px solid var(--border)",
        padding: "6px 12px",
        display: "flex",
        flexDirection: "column",
        gap: 4,
        flexShrink: 0,
      }}
    >
      {/* Row 1: Search */}
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <Tooltip title={showReplace ? "Hide Replace" : "Show Replace"}>
          <Button
            type="text"
            size="small"
            icon={
              <SwapOutlined
                style={{
                  fontSize: 11,
                  transform: showReplace ? "rotate(90deg)" : undefined,
                  transition: "transform 0.15s",
                }}
              />
            }
            style={{ height: 22, padding: "0 4px", minWidth: 0 }}
            onClick={() => setShowReplace(!showReplace)}
          />
        </Tooltip>
        <SearchOutlined style={{ fontSize: 12, color: "var(--text-muted)" }} />
        <Input
          ref={searchRef}
          size="small"
          placeholder="Search across tabs..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          onKeyDown={handleSearchKeyDown}
          style={{ width: 240, fontSize: 11 }}
          allowClear
        />
        <Tooltip title="Match Case">
          <Button
            type={caseSensitive ? "primary" : "text"}
            size="small"
            style={{
              height: 22,
              padding: "0 5px",
              minWidth: 0,
              fontSize: 11,
              fontWeight: 600,
            }}
            onClick={() => setCaseSensitive(!caseSensitive)}
          >
            Aa
          </Button>
        </Tooltip>
        <Tooltip title="Use Regular Expression">
          <Button
            type={useRegex ? "primary" : "text"}
            size="small"
            style={{
              height: 22,
              padding: "0 5px",
              minWidth: 0,
              fontSize: 11,
              fontWeight: 600,
            }}
            onClick={() => setUseRegex(!useRegex)}
          >
            .*
          </Button>
        </Tooltip>

        {matches.length > 0 && (
          <Text style={{ fontSize: 11, color: "var(--text-muted)", whiteSpace: "nowrap" }}>
            {currentIdx + 1} of {matches.length} in {tabCount}{" "}
            tab{tabCount !== 1 ? "s" : ""}
          </Text>
        )}
        {searchTerm && matches.length === 0 && (
          <Text style={{ fontSize: 11, color: "var(--text-faint)", whiteSpace: "nowrap" }}>
            No matches
          </Text>
        )}

        <Button
          type="text"
          size="small"
          icon={<UpOutlined style={{ fontSize: 10 }} />}
          disabled={matches.length === 0}
          onClick={prevMatch}
          style={{ height: 22, padding: "0 4px", minWidth: 0 }}
        />
        <Button
          type="text"
          size="small"
          icon={<DownOutlined style={{ fontSize: 10 }} />}
          disabled={matches.length === 0}
          onClick={nextMatch}
          style={{ height: 22, padding: "0 4px", minWidth: 0 }}
        />
        <Button
          type="text"
          size="small"
          icon={<CloseOutlined style={{ fontSize: 10 }} />}
          onClick={handleClose}
          style={{ height: 22, padding: "0 4px", minWidth: 0 }}
        />
      </div>

      {/* Row 2: Replace (when expanded) */}
      {showReplace && (
        <div style={{ display: "flex", alignItems: "center", gap: 6, paddingLeft: 28 }}>
          <Input
            size="small"
            placeholder="Replace with..."
            value={replaceTerm}
            onChange={(e) => setReplaceTerm(e.target.value)}
            onKeyDown={handleReplaceKeyDown}
            style={{ width: 240, fontSize: 11 }}
          />
          <Button
            size="small"
            disabled={matches.length === 0}
            onClick={replaceCurrent}
            style={{ fontSize: 11, height: 22 }}
          >
            Replace
          </Button>
          <Button
            size="small"
            disabled={matches.length === 0}
            onClick={replaceAll}
            style={{ fontSize: 11, height: 22 }}
          >
            Replace All
          </Button>
        </div>
      )}
    </div>
  );
}

// ── Search helper ────────────────────────────────────────────────────────────

/**
 * Search `text` for all occurrences of `regex` and invoke `onMatch` for each.
 * Matches are reported with 1-based line/column numbers.
 */
function searchText(
  text: string,
  regex: RegExp,
  onMatch: (line: number, column: number, length: number, preview: string) => void,
) {
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const lineStr = lines[i];
    regex.lastIndex = 0;
    let m: RegExpExecArray | null;
    while ((m = regex.exec(lineStr)) !== null) {
      onMatch(
        i + 1,
        m.index + 1,
        m[0].length,
        lineStr.trim(),
      );
      if (!regex.global) break;
      if (m[0].length === 0) { regex.lastIndex++; } // prevent infinite loop on zero-width matches
    }
  }
}
