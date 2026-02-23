// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef } from "react";
import { Tree, Typography, Spin, Collapse, Space, Button, Input, Switch, message } from "antd";
import {
  FolderOutlined,
  FolderOpenOutlined,
  FileOutlined,
  ReloadOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import type { DataNode, EventDataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import { ListDirectory, ReadFile, SearchFiles } from "../../../wailsjs/go/main/App";
import { useGitStore } from "../../store/gitStore";
import { useQueryStore } from "../../store/queryStore";
import type { filesystem } from "../../../wailsjs/go/models";

type FileEntry    = filesystem.FileEntry;
type SearchMatch  = filesystem.SearchMatch;

const { Text } = Typography;
const CLR_BORDER    = "#30363d";
const CLR_SECONDARY = "#8b949e";

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

  // ── Collapse controlled key (so search icon can expand the panel) ──────────
  const [activeKeys, setActiveKeys] = useState<string[]>([]);

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

  const onCollapseChange = (keys: string | string[]) => {
    const next = Array.isArray(keys) ? keys : [keys];
    setActiveKeys(next);
    if (next.includes("files")) loadRoot();
  };

  const toggleSearch = (e: React.MouseEvent) => {
    e.stopPropagation();
    const opening = !searchOpen;
    setSearchOpen(opening);
    if (opening) {
      // Expand panel if it isn't already open
      setActiveKeys((prev) => prev.includes("files") ? prev : [...prev, "files"]);
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

  const grouped = groupByPath(searchResults);

  return (
    <div style={{ borderTop: `1px solid ${CLR_BORDER}` }}>
      <Collapse
        ghost
        activeKey={activeKeys}
        style={{ background: "transparent" }}
        onChange={onCollapseChange as any}
        items={[{
          key:   "files",
          label: (
            <Space size={6}>
              <FolderOutlined style={{ color: CLR_SECONDARY, fontSize: 13 }} />
              <Text style={{ fontSize: 11, color: CLR_SECONDARY, textTransform: "uppercase", letterSpacing: "0.08em" }}>
                Files
              </Text>
            </Space>
          ),
          style: { border: "none" },
          extra: (
            <Space size={2}>
              <Button
                size="small"
                type="text"
                icon={
                  <SearchOutlined
                    style={{ fontSize: 11, color: searchOpen ? "#58a6ff" : CLR_SECONDARY }}
                  />
                }
                onClick={toggleSearch}
                style={{ height: 18, padding: "0 4px", minWidth: 0 }}
              />
              {loaded && (
                <Button
                  size="small"
                  type="text"
                  icon={<ReloadOutlined style={{ fontSize: 11 }} />}
                  loading={loading}
                  onClick={(e) => { e.stopPropagation(); refresh(); }}
                  style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                />
              )}
            </Space>
          ),
          children: (
            <div style={{ padding: "0 4px 8px" }}>
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
                    <Text style={{ fontSize: 10, color: useRegex ? "#58a6ff" : CLR_SECONDARY, userSelect: "none" }}>
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
                                color: "#58a6ff",
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
                                  onMouseEnter={(e) => (e.currentTarget.style.background = "#30363d")}
                                  onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                                >
                                  <span style={{ color: CLR_SECONDARY, fontSize: 10, flexShrink: 0, fontFamily: "monospace" }}>
                                    {m.lineNumber}
                                  </span>
                                  <span
                                    style={{
                                      fontFamily: "monospace",
                                      fontSize: 11,
                                      color: "#e6edf3",
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
                <Tree
                  treeData={treeData}
                  loadedKeys={loadedKeys}
                  selectedKeys={selectedKey ? [selectedKey] : []}
                  onLoad={(keys) => setLoadedKeys(keys)}
                  loadData={onLoadData as any}
                  onSelect={onSelect as any}
                  showIcon
                  blockNode
                  style={{ background: "transparent", color: "#e6edf3", fontSize: 12 }}
                />
              )}
            </div>
          ),
        }]}
      />
    </div>
  );
}
