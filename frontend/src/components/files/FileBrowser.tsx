// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useLayoutEffect, useRef } from "react";
import { Tree, Typography, Spin, Button, Input, Switch, Modal, message } from "antd";
import {
  FolderOutlined,
  FolderOpenOutlined,
  FolderAddOutlined,
  FileOutlined,
  FileAddOutlined,
  ReloadOutlined,
  SearchOutlined,
  DiffOutlined,
  DeleteOutlined,
  EditOutlined,
  CopyOutlined,
  FolderViewOutlined,
  CaretRightFilled,
  CaretDownFilled,
} from "@ant-design/icons";
import type { DataNode, EventDataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import {
  ListDirectory,
  ReadFile,
  SearchFiles,
  RevealInFinder,
  DeleteFile,
  DeleteDirectory,
  RenameFile,
  CreateDirectory,
  CreateFile,
} from "../../../wailsjs/go/main/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { useGitStore } from "../../store/gitStore";
import { useQueryStore } from "../../store/queryStore";
import { useDiffStore } from "../../store/diffStore";
import { getPlatformOS, getCachedPlatformOS, revealLabel } from "./platformUtil";
import type { filesystem } from "../../../wailsjs/go/models";

type FileEntry    = filesystem.FileEntry;
type SearchMatch  = filesystem.SearchMatch;

const { Text } = Typography;
const CLR_SECONDARY = "var(--text-muted)";

/** Extract the directory portion of a path, handling both / and \ separators. */
function pathDir(p: string): string {
  const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  if (i < 0) return ".";
  // i == 0 means root separator (e.g. "/filename") — preserve the separator.
  return i === 0 ? p.substring(0, 1) : p.substring(0, i);
}

/** Extract the filename from a path, handling both / and \ separators. */
function pathBase(p: string): string {
  const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  return i >= 0 ? p.substring(i + 1) : p;
}

function entriesToNodes(entries: FileEntry[]): DataNode[] {
  return entries.map((e) => ({
    key:    e.path,
    title:  e.name,
    icon:   (props: { expanded?: boolean }) =>
      e.isDir
        ? (props.expanded ? <FolderOpenOutlined /> : <FolderOutlined />)
        : <FileOutlined style={{ color: CLR_SECONDARY }} />,
    isLeaf: !e.isDir,
  }));
}

function updateNode(nodes: DataNode[], targetKey: string, children: DataNode[]): DataNode[] {
  return nodes.map((node) => {
    if (node.key === targetKey) return { ...node, children };
    if ((node as any).children) {
      return { ...node, children: updateNode((node as any).children, targetKey, children) };
    }
    return node;
  });
}

// Returns a context window around the match so long lines display usefully.
function getSnippet(
  line: string,
  start: number,
  end: number,
  ctx = 50,
): { before: string; match: string; after: string; ellipsisBefore: boolean; ellipsisAfter: boolean } {
  const snippetStart = Math.max(0, start - ctx);
  const snippetEnd   = Math.min(line.length, end + ctx);
  return {
    before:         line.slice(snippetStart, start),
    match:          line.slice(start, end),
    after:          line.slice(end, snippetEnd),
    ellipsisBefore: snippetStart > 0,
    ellipsisAfter:  snippetEnd < line.length,
  };
}

function groupByPath(matches: SearchMatch[]): Map<string, SearchMatch[]> {
  const map = new Map<string, SearchMatch[]>();
  for (const m of matches) {
    if (!map.has(m.path)) map.set(m.path, []);
    map.get(m.path)!.push(m);
  }
  return map;
}

export default function FileBrowser() {
  const exportDir   = useGitStore((s) => s.exportDir);
  const openFile    = useQueryStore((s) => s.openFile);
  const currentFile = useQueryStore((s) => s.currentFile);
  const updateTabPath  = useQueryStore((s) => s.updateTabPath);
  const orphanTab      = useQueryStore((s) => s.orphanFileTab);

  // ── File tree state ────────────────────────────────────────────────────────
  const [treeData,    setTreeData]    = useState<DataNode[]>([]);
  const [loadedKeys,  setLoadedKeys]  = useState<Key[]>([]);
  const [selectedKey, setSelectedKey] = useState<Key | null>(null);
  const [loading,     setLoading]     = useState(false);
  const [loaded,      setLoaded]      = useState(false);

  // ── Search state ───────────────────────────────────────────────────────────
  const [searchOpen,    setSearchOpen]    = useState(false);
  const [searchQuery,   setSearchQuery]   = useState("");
  const [useRegex,      setUseRegex]      = useState(false);
  const [searchResults, setSearchResults] = useState<SearchMatch[]>([]);
  const [searching,     setSearching]     = useState(false);
  const [searchError,   setSearchError]   = useState<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // ── Context menu ─────────────────────────────────────────────────────────
  const [fileCtxMenu, setFileCtxMenu] = useState<{ x: number; y: number; path: string; name: string; isDir: boolean } | null>(null);
  const fileCtxRef = useRef<HTMLDivElement>(null);

  // ── Inline input state (rename / new folder / new file) ─────────────────
  const [inlineInput, setInlineInput] = useState<{ kind: "rename" | "newFolder" | "newFile"; path: string; value: string } | null>(null);

  // ── Collapse state ──────────────────────────────────────────────────────────
  const [expanded, setExpanded] = useState(false);

  // ── Platform detection for labels ─────────────────────────────────────────
  const [platformOS, setPlatformOS] = useState<string | null>(getCachedPlatformOS());
  useEffect(() => { getPlatformOS().then(setPlatformOS); }, []);
  const revealText = revealLabel(platformOS);

  const pendingDiff   = useDiffStore((s) => s.pending);
  const selectForComp = useDiffStore((s) => s.selectForComparison);
  const compareWith   = useDiffStore((s) => s.compareWith);

  // Close file context menu on outside click or Escape key
  useEffect(() => {
    if (!fileCtxMenu) return;
    const close = () => setFileCtxMenu(null);
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") close(); };
    window.addEventListener("click", close);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("keydown", onKey);
    };
  }, [fileCtxMenu]);

  // Clamp file context menu inside the viewport and focus the first item (runs before browser paint — no flash)
  useLayoutEffect(() => {
    if (!fileCtxMenu || !fileCtxRef.current) return;
    const el = fileCtxRef.current;
    const { width, height } = el.getBoundingClientRect();
    const pad = 8;
    const left = Math.max(pad, Math.min(fileCtxMenu.x, window.innerWidth  - width  - pad));
    const top  = Math.max(pad, Math.min(fileCtxMenu.y, window.innerHeight - height - pad));
    el.style.left = `${left}px`;
    el.style.top  = `${top}px`;
    // Auto-focus the first menu item for keyboard accessibility.
    const firstItem = el.querySelector<HTMLElement>("[role='menuitem']");
    firstItem?.focus();
  }, [fileCtxMenu]);

  // Reset tree when the working directory changes
  useEffect(() => {
    setLoaded(false);
    setTreeData([]);
    setLoadedKeys([]);
    setSelectedKey(null);
  }, [exportDir]);

  // Keep selected key in sync with the active tab (including tab switches)
  useEffect(() => {
    setSelectedKey(currentFile);
  }, [currentFile]);

  // Debounced search
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (!searchQuery.trim() || !exportDir || !searchOpen) {
      setSearchResults([]);
      setSearchError(null);
      return;
    }

    debounceRef.current = setTimeout(async () => {
      setSearching(true);
      setSearchError(null);
      try {
        const matches = await SearchFiles(exportDir, searchQuery, useRegex);
        setSearchResults(matches ?? []);
      } catch (e) {
        setSearchError(String(e));
        setSearchResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [searchQuery, useRegex, exportDir, searchOpen]);

  const loadRoot = async () => {
    if (!exportDir || loading || loaded) return;
    setLoading(true);
    try {
      const entries = await ListDirectory(exportDir);
      setTreeData(entriesToNodes(entries));
      setLoaded(true);
    } catch {
      // non-fatal
    } finally {
      setLoading(false);
    }
  };

  const refresh = async () => {
    setFileCtxMenu(null); // dismiss stale context menu
    setLoaded(false);
    setTreeData([]);
    setLoadedKeys([]);
    setSelectedKey(null);
    setLoading(true);
    try {
      const entries = await ListDirectory(exportDir);
      setTreeData(entriesToNodes(entries));
      setLoaded(true);
    } catch {
      // non-fatal
    } finally {
      setLoading(false);
    }
  };

  // Refresh the tree automatically when an export finishes
  useEffect(() => {
    const handler = () => { if (loaded) refresh(); };
    window.addEventListener("thaw:export-complete", handler);
    return () => window.removeEventListener("thaw:export-complete", handler);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loaded, exportDir]);

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    if ((node as any).children) return;
    const path = String(node.key);
    try {
      const entries = await ListDirectory(path);
      setTreeData((prev) => updateNode(prev, path, entriesToNodes(entries)));
    } catch {
      // non-fatal
    }
  };

  const onSelect = async (_keys: Key[], info: { node: DataNode }) => {
    const node = info.node;
    if ((node as any).isLeaf === false) return;
    const path = String(node.key);
    setSelectedKey(path);
    try {
      const content = await ReadFile(path);
      openFile(path, content);
    } catch (e) {
      message.error(`Could not open file: ${String(e)}`);
      setSelectedKey(null);
    }
  };

  const toggleExpanded = () => {
    const next = !expanded;
    setExpanded(next);
    if (next) loadRoot();
  };

  const toggleSearch = (e: React.MouseEvent) => {
    e.stopPropagation();
    const opening = !searchOpen;
    setSearchOpen(opening);
    if (opening) {
      setExpanded(true);
    } else {
      setSearchQuery("");
      setSearchResults([]);
      setSearchError(null);
    }
  };

  const handleResultClick = async (match: SearchMatch) => {
    try {
      const content = await ReadFile(match.path);
      openFile(match.path, content);
      setTimeout(() => {
        window.dispatchEvent(
          new CustomEvent("thaw:scroll-to-line", {
            detail: {
              line:       match.lineNumber,
              matchStart: match.matchStart,
              matchEnd:   match.matchEnd,
            },
          })
        );
      }, 50);
    } catch {
      // non-fatal
    }
  };

  const onRightClick = ({ event, node }: { event: React.MouseEvent; node: DataNode }) => {
    event.preventDefault();
    const path = String(node.key);
    const name = pathBase(path);
    const isDir = (node as any).isLeaf === false;
    setFileCtxMenu({ x: event.clientX, y: event.clientY, path, name, isDir });
  };

  const selectFileForComparison = () => {
    if (!fileCtxMenu) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    selectForComp({ category: "file", label: `FILE: ${name}`, path });
    message.success(`Selected for comparison: ${name}`);
  };

  const compareFileWith = () => {
    if (!fileCtxMenu) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    compareWith({ category: "file", label: `FILE: ${name}`, path });
  };

  const handleReveal = () => {
    if (!fileCtxMenu) return;
    RevealInFinder(fileCtxMenu.path).catch((e) => message.error(`Could not reveal: ${String(e)}`));
    setFileCtxMenu(null);
  };

  const handleCopyPath = async () => {
    if (!fileCtxMenu) return;
    const p = fileCtxMenu.path;
    setFileCtxMenu(null);
    try {
      await ClipboardSetText(p);
      message.success("Path copied");
    } catch {
      message.error("Failed to copy path");
    }
  };

  const handleDeleteConfirm = () => {
    if (!fileCtxMenu) return;
    const { path, name, isDir } = fileCtxMenu;
    setFileCtxMenu(null);
    Modal.confirm({
      title: `Delete ${isDir ? "folder" : "file"}`,
      content: `Are you sure you want to delete "${name}"?${isDir ? " All contents will be permanently removed." : ""}`,
      okText: "Delete",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          if (isDir) {
            await DeleteDirectory(path);
          } else {
            await DeleteFile(path);
          }
          // Read fresh tabs from the store (not the stale closure captured at render time).
          const currentTabs = useQueryStore.getState().tabs;
          const sep = path.includes("\\") ? "\\" : "/";
          for (const tab of currentTabs) {
            if (tab.path === path || (isDir && tab.path?.startsWith(path + sep))) {
              orphanTab(tab.id);
            }
          }
          message.success(`Deleted ${name}`);
          refresh();
        } catch (e) {
          message.error(`Delete failed: ${String(e)}`);
          throw e; // Re-throw to keep the modal open on failure.
        }
      },
    });
  };

  const handleRenameStart = () => {
    if (!fileCtxMenu) return;
    setInlineInput({ kind: "rename", path: fileCtxMenu.path, value: fileCtxMenu.name });
    setFileCtxMenu(null);
  };

  const handleNewFolderStart = () => {
    if (!fileCtxMenu) return;
    setInlineInput({ kind: "newFolder", path: fileCtxMenu.path, value: "" });
    setFileCtxMenu(null);
  };

  const handleNewFileStart = () => {
    if (!fileCtxMenu) return;
    setInlineInput({ kind: "newFile", path: fileCtxMenu.path, value: "" });
    setFileCtxMenu(null);
  };

  const submitInlineInput = async () => {
    if (!inlineInput || !inlineInput.value.trim()) {
      setInlineInput(null);
      return;
    }
    const { kind, path, value } = inlineInput;
    const sanitized = value.trim().replace(/[/\\]/g, "");
    if (!sanitized) {
      message.error("Name cannot be empty or contain path separators");
      return;
    }
    try {
      if (kind === "rename") {
        const dir = pathDir(path);
        const sep = path.includes("\\") ? "\\" : "/";
        // Avoid double separator when dir is a root (e.g. "/" or "C:\").
        const newPath = dir.endsWith(sep) ? `${dir}${sanitized}` : `${dir}${sep}${sanitized}`;
        await RenameFile(path, newPath);
        // Read fresh tabs from the store (not the stale closure captured at render time).
        // Uses updateTabPath (not markSaved) to preserve the dirty state — the file's
        // disk content was moved but any unsaved in-memory edits remain uncommitted.
        const currentTabs = useQueryStore.getState().tabs;
        const prefix = path + sep;
        for (const tab of currentTabs) {
          if (tab.path === path) {
            updateTabPath(tab.id, newPath, sanitized);
          } else if (tab.path?.startsWith(prefix)) {
            const updatedPath = newPath + tab.path.substring(path.length);
            updateTabPath(tab.id, updatedPath, pathBase(updatedPath));
          }
        }
        message.success(`Renamed to ${sanitized}`);
      } else if (kind === "newFolder") {
        const sep = path.includes("\\") ? "\\" : "/";
        await CreateDirectory(`${path}${sep}${sanitized}`);
        message.success(`Created folder ${sanitized}`);
      } else if (kind === "newFile") {
        const sep = path.includes("\\") ? "\\" : "/";
        const name = sanitized.endsWith(".sql") ? sanitized : `${sanitized}.sql`;
        await CreateFile(`${path}${sep}${name}`);
        message.success(`Created ${name}`);
      }
      setInlineInput(null);
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  const grouped = groupByPath(searchResults);

  return (
    <div style={{ padding: "4px 4px" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", padding: "0 4px 0 8px", marginBottom: expanded ? 4 : 0, gap: 2 }}>
        <div
          style={{ display: "flex", alignItems: "center", gap: 4, cursor: "pointer", flex: 1, padding: "2px 4px", borderRadius: 4 }}
          onClick={toggleExpanded}
          onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
          onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
        >
          {expanded
            ? <CaretDownFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />
            : <CaretRightFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />
          }
          <FolderOutlined style={{ color: "var(--text)", fontSize: 13 }} />
          <Text style={{ fontSize: 11, color: "var(--text)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
            Files
          </Text>
        </div>
        <Button
          size="small"
          type="text"
          icon={<SearchOutlined style={{ fontSize: 11, color: searchOpen ? "var(--link)" : CLR_SECONDARY }} />}
          onClick={toggleSearch}
          style={{ height: 20, padding: "0 4px", minWidth: 0 }}
        />
        {loaded && (
          <Button
            size="small"
            type="text"
            icon={<ReloadOutlined style={{ fontSize: 11 }} />}
            loading={loading}
            onClick={(e) => { e.stopPropagation(); refresh(); }}
            style={{ height: 20, padding: "0 4px", minWidth: 0 }}
          />
        )}
      </div>

      {/* Content */}
      {expanded && (
        <div style={{ padding: "0 4px" }}>
          {!exportDir && (
            <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>
              Set a working directory in the Git section below.
            </Text>
          )}

          {exportDir && searchOpen && (
            <>
              {/* ── Search input ── */}
              <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 6 }}>
                <Input
                  size="small"
                  placeholder={useRegex ? "Regex pattern…" : "Search files…"}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  prefix={<SearchOutlined style={{ color: CLR_SECONDARY, fontSize: 11 }} />}
                  style={{ flex: 1, fontSize: 12 }}
                  allowClear
                  autoFocus
                />
                <Switch
                  size="small"
                  checked={useRegex}
                  onChange={setUseRegex}
                  title="Regular expression"
                />
                <Text style={{ fontSize: 10, color: useRegex ? "var(--link)" : CLR_SECONDARY, userSelect: "none" }}>
                  .*
                </Text>
              </div>

              {/* ── Search states ── */}
              {searching && (
                <Spin size="small" style={{ display: "block", margin: "8px auto" }} />
              )}

              {!searching && searchError && (
                <Text style={{ fontSize: 11, color: "#f85149", display: "block", wordBreak: "break-word" }}>
                  {searchError}
                </Text>
              )}

              {!searching && !searchError && searchQuery.trim() && searchResults.length === 0 && (
                <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>No results.</Text>
              )}

              {/* ── Search results ── */}
              {searchResults.length > 0 && (
                <div>
                  {Array.from(grouped.entries()).map(([path, matches]) => {
                    const relPath = exportDir
                      ? path.replace(exportDir, "").replace(/^[/\\]/, "")
                      : path;
                    return (
                      <div key={path} style={{ marginBottom: 10 }}>
                        <div
                          title={path}
                          style={{
                            fontSize: 11,
                            color: "var(--link)",
                            fontWeight: 500,
                            marginBottom: 2,
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          <FileOutlined style={{ marginRight: 4, fontSize: 10 }} />
                          {relPath}
                        </div>
                        {matches.map((m) => {
                          const { before, match, after, ellipsisBefore, ellipsisAfter } =
                            getSnippet(m.lineContent, m.matchStart, m.matchEnd);
                          return (
                            <div
                              key={`${m.path}:${m.lineNumber}`}
                              onClick={() => handleResultClick(m)}
                              style={{
                                display: "flex",
                                alignItems: "baseline",
                                gap: 6,
                                padding: "1px 4px",
                                cursor: "pointer",
                                borderRadius: 3,
                                overflow: "hidden",
                              }}
                              onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
                              onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                            >
                              <span style={{ color: CLR_SECONDARY, fontSize: 10, flexShrink: 0, fontFamily: "monospace" }}>
                                {m.lineNumber}
                              </span>
                              <span
                                style={{
                                  fontFamily: "monospace",
                                  fontSize: 11,
                                  color: "var(--text)",
                                  overflow: "hidden",
                                  whiteSpace: "nowrap",
                                  textOverflow: "ellipsis",
                                  flexShrink: 1,
                                  minWidth: 0,
                                }}
                              >
                                {ellipsisBefore && <span style={{ color: CLR_SECONDARY }}>…</span>}
                                {before}
                                <mark style={{ background: "#ffa657", color: "#0d1117", borderRadius: 2, padding: "0 1px" }}>
                                  {match}
                                </mark>
                                {after}
                                {ellipsisAfter && <span style={{ color: CLR_SECONDARY }}>…</span>}
                              </span>
                            </div>
                          );
                        })}
                      </div>
                    );
                  })}
                  {searchResults.length >= 200 && (
                    <Text style={{ fontSize: 10, color: CLR_SECONDARY }}>
                      Showing first 200 results.
                    </Text>
                  )}
                </div>
              )}
            </>
          )}

          {exportDir && !searchOpen && loading && (
            <Spin size="small" style={{ display: "block", margin: "8px auto" }} />
          )}

          {exportDir && !searchOpen && !loading && loaded && treeData.length === 0 && (
            <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Directory is empty.</Text>
          )}

          {!searchOpen && loaded && treeData.length > 0 && (
            <div style={{ overflow: "hidden" }}>
              <Tree
                treeData={treeData}
                loadedKeys={loadedKeys}
                selectedKeys={selectedKey ? [selectedKey] : []}
                onLoad={(keys) => setLoadedKeys(keys)}
                loadData={onLoadData as any}
                onSelect={onSelect as any}
                onRightClick={onRightClick as any}
                showIcon
                blockNode
                style={{ background: "transparent", color: "var(--text)", fontSize: 12 }}
              />
            </div>
          )}
        </div>
      )}

      {/* Inline input modal for rename / new folder / new file */}
      {inlineInput && (
        <Modal
          open
          title={
            inlineInput.kind === "rename" ? "Rename" :
            inlineInput.kind === "newFolder" ? "New Folder" :
            "New SQL File"
          }
          okText={inlineInput.kind === "rename" ? "Rename" : "Create"}
          onOk={submitInlineInput}
          onCancel={() => setInlineInput(null)}
          width={360}
          destroyOnClose
        >
          <Input
            autoFocus
            size="small"
            value={inlineInput.value}
            onChange={(e) => setInlineInput({ ...inlineInput, value: e.target.value })}
            onPressEnter={submitInlineInput}
            placeholder={
              inlineInput.kind === "rename" ? "New name" :
              inlineInput.kind === "newFolder" ? "Folder name" :
              "File name (.sql)"
            }
            style={{ marginTop: 8 }}
            ref={(el) => {
              // Select just the filename stem (before the last dot) for easy editing.
              if (el && inlineInput.kind === "rename") {
                const input = el.input ?? el;
                if (input && typeof input.setSelectionRange === "function") {
                  const dot = inlineInput.value.lastIndexOf(".");
                  const end = dot > 0 ? dot : inlineInput.value.length;
                  requestAnimationFrame(() => input.setSelectionRange(0, end));
                }
              }
            }}
          />
        </Modal>
      )}

      {/* Context menu */}
      {fileCtxMenu && (
        <div
          ref={fileCtxRef}
          role="menu"
          aria-label="File actions"
          aria-orientation="vertical"
          style={{
            position: "fixed",
            top: fileCtxMenu.y,
            left: fileCtxMenu.x,
            zIndex: 9999,
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
            minWidth: 180,
            padding: "4px 0",
          }}
          onClick={(e) => e.stopPropagation()}
          onContextMenu={(e) => e.preventDefault()}
          onBlur={(e) => {
            // Dismiss menu when focus leaves the container entirely.
            if (!e.currentTarget.contains(e.relatedTarget as Node)) {
              setFileCtxMenu(null);
            }
          }}
        >
          {/* ── File management actions ── */}
          <CtxItem icon={<FolderViewOutlined />} label={revealText} onClick={handleReveal} />
          <CtxItem icon={<CopyOutlined />} label="Copy Path" onClick={handleCopyPath} />
          <CtxItem icon={<EditOutlined />} label="Rename…" onClick={handleRenameStart} />
          <CtxItem icon={<DeleteOutlined />} label="Delete" onClick={handleDeleteConfirm} danger />

          {/* ── Directory-only actions ── */}
          {fileCtxMenu.isDir && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              <CtxItem icon={<FolderAddOutlined />} label="New Folder…" onClick={handleNewFolderStart} />
              <CtxItem icon={<FileAddOutlined />} label="New SQL File…" onClick={handleNewFileStart} />
            </>
          )}

          {/* ── Comparison actions (files only) ── */}
          {!fileCtxMenu.isDir && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              <CtxItem icon={<DiffOutlined />} label="Select for Comparison" onClick={selectFileForComparison} />
              {pendingDiff !== null && (
                <CtxItem
                  icon={<DiffOutlined style={{ color: "var(--accent)" }} />}
                  label={`Compare with: ${pendingDiff.label}`}
                  onClick={compareFileWith}
                />
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}

/** Reusable context menu item */
function CtxItem({ icon, label, onClick, danger }: { icon: React.ReactNode; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <div
      role="menuitem"
      tabIndex={0}
      style={{
        display: "flex", alignItems: "center", gap: 8,
        padding: "6px 14px", fontSize: 13, cursor: "pointer",
        color: danger ? "#f85149" : "var(--text)",
        outline: "none",
      }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      onFocus={(e) => (e.currentTarget.style.background = "var(--border)")}
      onBlur={(e) => (e.currentTarget.style.background = "transparent")}
      onClick={onClick}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onClick(); } }}
    >
      <span style={{ fontSize: 12, display: "flex" }}>{icon}</span>
      {label}
    </div>
  );
}
