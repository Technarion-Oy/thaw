// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useLayoutEffect, useRef, useMemo, useCallback } from "react";
import { Tree, Typography, Spin, Button, Input, Switch, Modal, Tooltip, App as AntApp } from "antd";
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
  SnippetsOutlined,
  FolderViewOutlined,
  CaretRightFilled,
  CaretDownFilled,
  PlusOutlined,
  MinusOutlined,
  UndoOutlined,
  BranchesOutlined,
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
  DuplicateFile,
  StartFileWatcher,
  StopFileWatcher,
  GitGetHeadFileContent,
} from "../../../wailsjs/go/app/App";
import { ClipboardSetText, EventsOn } from "../../../wailsjs/runtime/runtime";
import { useGitStore } from "../../store/gitStore";
import { sigilColor } from "../git/gitStatusUtil";
import { useQueryStore } from "../../store/queryStore";
import { useDiffStore } from "../../store/diffStore";
import { getPlatformOS, getCachedPlatformOS, revealLabel } from "./platformUtil";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
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

/** Detect the path separator used in a path (backslash on Windows, forward slash otherwise). */
function pathSep(p: string): string {
  return p.includes("\\") ? "\\" : "/";
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

/** Merge fresh entries into existing ones, preserving children of nodes that
 *  still exist so expanded subtrees aren't lost. Works for both root-level
 *  and subdirectory refreshes. */
function mergeNodes(prev: DataNode[], fresh: DataNode[]): DataNode[] {
  const oldByKey = new Map(prev.map((n) => [String(n.key), n]));
  return fresh.map((f) => {
    const existing = oldByKey.get(String(f.key));
    // Keep expanded children only if the fresh node is still a directory.
    // If a directory was replaced by a file with the same name, drop the stale children.
    return existing?.children && !f.isLeaf ? { ...f, children: existing.children } : f;
  });
}

function updateNode(nodes: DataNode[], targetKey: string, children: DataNode[], merge?: boolean): DataNode[] {
  return nodes.map((node) => {
    if (node.key === targetKey) {
      const merged = merge && node.children ? mergeNodes(node.children, children) : children;
      return { ...node, children: merged };
    }
    if ((node as any).children) {
      return { ...node, children: updateNode((node as any).children, targetKey, children, merge) };
    }
    return node;
  });
}

/** Create a DataNode for a new file or directory. */
function makeNode(path: string, name: string, isDir: boolean): DataNode {
  return {
    key: path,
    title: name,
    icon: (props: { expanded?: boolean }) =>
      isDir
        ? (props.expanded ? <FolderOpenOutlined /> : <FolderOutlined />)
        : <FileOutlined style={{ color: CLR_SECONDARY }} />,
    isLeaf: !isDir,
  };
}

/** Remove a node by key from the tree. */
function removeNode(nodes: DataNode[], key: string): DataNode[] {
  return nodes
    .filter((n) => n.key !== key)
    .map((n) =>
      n.children ? { ...n, children: removeNode(n.children, key) } : n
    );
}

/** Rename a node (update key + title) and recursively re-key all descendants. */
function renameTreeNode(
  nodes: DataNode[],
  oldKey: string,
  newKey: string,
  newTitle: string,
): DataNode[] {
  return nodes.map((n) => {
    if (n.key === oldKey) {
      return {
        ...n,
        key: newKey,
        title: newTitle,
        children: n.children ? reKeyChildren(n.children, String(oldKey), newKey) : undefined,
      };
    }
    return n.children
      ? { ...n, children: renameTreeNode(n.children, oldKey, newKey, newTitle) }
      : n;
  });
}

/** Recursively update descendant keys when a parent path changes. */
function reKeyChildren(nodes: DataNode[], oldPrefix: string, newPrefix: string): DataNode[] {
  return nodes.map((n) => ({
    ...n,
    key: newPrefix + String(n.key).substring(oldPrefix.length),
    children: n.children ? reKeyChildren(n.children, oldPrefix, newPrefix) : undefined,
  }));
}

/** Insert a child into a parent's children, maintaining dirs-first alphabetical order.
 *  If the parent hasn't been expanded yet (no children array), the node is not inserted
 *  — it will appear naturally when the user expands the directory. */
function addChild(nodes: DataNode[], parentKey: string, child: DataNode): DataNode[] {
  return nodes.map((n) => {
    if (n.key === parentKey) {
      if (!n.children) return n;
      const kids = [...n.children];
      const isDir = !child.isLeaf;
      const name = String(child.title ?? "");
      let i = 0;
      if (isDir) {
        while (i < kids.length && !kids[i].isLeaf && String(kids[i].title ?? "").localeCompare(name) < 0) i++;
      } else {
        while (i < kids.length && !kids[i].isLeaf) i++;
        while (i < kids.length && String(kids[i].title ?? "").localeCompare(name) < 0) i++;
      }
      kids.splice(i, 0, child);
      return { ...n, children: kids };
    }
    return n.children ? { ...n, children: addChild(n.children, parentKey, child) } : n;
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
  const { modal, message } = AntApp.useApp();
  const exportDir    = useGitStore((s) => s.exportDir);
  const gitStatus    = useGitStore((s) => s.status);
  const stageFile    = useGitStore((s) => s.stageFile);
  const unstageFile  = useGitStore((s) => s.unstageFile);
  const discardFile  = useGitStore((s) => s.discardFile);
  const resetHard    = useGitStore((s) => s.resetHard);
  const openGitOps   = useGitStore((s) => s.openGitOps);
  const pickExportDir = useGitStore((s) => s.pickExportDir);
  const refreshGitStatus = useGitStore((s) => s.refreshStatus);
  const loadGitConfig = useGitStore((s) => s.loadConfig);
  const gitConfigLoaded = useGitStore((s) => s.configLoaded);
  const openFile     = useQueryStore((s) => s.openFile);
  const currentFile = useQueryStore((s) => s.currentFile);
  const updateTabPath  = useQueryStore((s) => s.updateTabPath);
  const orphanTab      = useQueryStore((s) => s.orphanFileTab);

  // ── Git status overlay ───────────────────────────────────────────────────────
  // Git status paths are repo-relative ("MYDB/PUBLIC/T.sql"); tree node keys are
  // absolute OS paths the explorer built by joining the export dir with each name,
  // so `relOf` recovers a node's repo-relative path by stripping the export-dir
  // prefix (exact — no suffix guessing, which would false-match files that merely
  // share a basename). The uncapped `changedPaths` map drives coloring so the whole
  // tree is covered even in huge change sets; the capped staged/unstaged lists
  // drive the precise Stage/Unstage context menu.
  const gitOverlay = useMemo(() => {
    const byRel       = new Map<string, string>(); // file rel → status letter
    const dirLetter   = new Map<string, string>(); // dir rel → dominant letter
    const stagedRel   = new Set<string>();
    const unstagedRel = new Set<string>();
    const newFilesRel = new Set<string>(); // no committed version → discard deletes
    const partialRel  = new Set<string>(); // both staged & unstaged → discard also drops staged

    // Folder color = the most significant change beneath it. A/U are both "new"
    // (green), so a folder of only-new files stays green rather than reading as
    // modified. Higher rank wins.
    const RANK: Record<string, number> = { M: 5, R: 5, C: 5, D: 4, A: 2, U: 1 };
    const bumpDir = (dir: string, letter: string) => {
      const cur = dirLetter.get(dir);
      if (cur === undefined || (RANK[letter] ?? 0) > (RANK[cur] ?? 0)) dirLetter.set(dir, letter);
    };
    const addDirs = (rel: string, letter: string) => {
      let i = rel.lastIndexOf("/");
      while (i > 0) { bumpDir(rel.slice(0, i), letter); i = rel.lastIndexOf("/", i - 1); }
    };

    if (gitStatus) {
      // Uncapped: drives coloring and the authoritative new-file (discard-deletes)
      // classification for every changed file, including beyond the 500-cap arrays.
      for (const [p, cf] of Object.entries(gitStatus.changedPaths ?? {})) {
        const rel = p.replace(/\\/g, "/");
        byRel.set(rel, cf.status);
        if (cf.isNew) newFilesRel.add(rel);
        if (cf.partiallyStaged) partialRel.add(rel);
        addDirs(rel, cf.status);
      }
      for (const fc of (gitStatus.staged   ?? [])) stagedRel.add(fc.path.replace(/\\/g, "/"));
      for (const fc of (gitStatus.unstaged ?? [])) unstagedRel.add(fc.path.replace(/\\/g, "/"));
    }

    // Exact repo-relative path of a tree node, or null when it's outside the repo.
    const base = exportDir.replace(/[/\\]+$/, "").replace(/\\/g, "/");
    const relOf = (nodeKey: string): string | null => {
      const a = nodeKey.replace(/\\/g, "/");
      if (a === base) return "";
      if (base && a.startsWith(base + "/")) return a.slice(base.length + 1);
      return null;
    };

    return { byRel, dirLetter, stagedRel, unstagedRel, newFilesRel, partialRel, relOf };
  }, [gitStatus, exportDir]);

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

  // ── Inline rename state (VS Code–style editing in the tree) ────────────
  const [editingKey, setEditingKey] = useState<Key | null>(null);
  const [editingValue, setEditingValue] = useState("");
  const editActionRef = useRef<"idle" | "submitting" | "cancelled">("idle");
  const editInitRef = useRef(false);

  // ── Modal state (new folder / new file) ─────────────────────────────────
  const [inlineInput, setInlineInput] = useState<{ kind: "newFolder" | "newFile"; path: string; value: string } | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // ── Collapse state ──────────────────────────────────────────────────────────
  const [expanded, setExpanded] = useState(false);

  // ── Platform detection for labels ─────────────────────────────────────────
  const [platformOS, setPlatformOS] = useState<string | null>(getCachedPlatformOS());
  useEffect(() => { getPlatformOS().then(setPlatformOS); }, []);
  const revealText = revealLabel(platformOS);

  const pendingDiff   = useDiffStore((s) => s.pending);
  const selectForComp = useDiffStore((s) => s.selectForComparison);
  const compareWith   = useDiffStore((s) => s.compareWith);

  const fileWatcherEnabled = useFeatureFlagsStore((s) => s.flags.fileWatcher);
  const gitEnabled         = useFeatureFlagsStore((s) => s.flags.gitIntegration);

  // The standalone Git panel was folded into this panel, so the Files panel now
  // owns loading the git/export config on first mount (idempotent).
  useEffect(() => {
    if (!gitConfigLoaded) loadGitConfig();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gitConfigLoaded]);

  // Keep git status fresh for whatever directory the explorer is showing, so the
  // tree colors don't depend on some other surface having refreshed first.
  useEffect(() => {
    if (exportDir) refreshGitStatus(true); // background: don't surface status errors
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exportDir]);

  const gitRepo       = gitEnabled && !!gitStatus?.isRepo;
  // Empty branch = repo with no commits yet (Head() failed); don't imply "main".
  const gitBranch     = gitStatus?.branch || "(no commits)";
  const gitAhead      = gitStatus?.ahead ?? 0;
  const gitChanged    = gitStatus?.totalChanged ?? 0;
  const gitStagedTot  = gitStatus?.stagedTotal ?? 0;

  // ── Self-change suppression ────────────────────────────────────────────────
  // Tracks directories modified by in-app operations so watcher events don't
  // cause a redundant (flickering) refresh. Entries are auto-cleared after 500ms.
  const selfChangedDirs = useRef(new Set<string>());
  const selfChangeTimers = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  const markSelfChanged = (dir: string) => {
    selfChangedDirs.current.add(dir);
    const prev = selfChangeTimers.current.get(dir);
    if (prev) clearTimeout(prev);
    selfChangeTimers.current.set(dir, setTimeout(() => {
      selfChangedDirs.current.delete(dir);
      selfChangeTimers.current.delete(dir);
    }, 500));
  };

  // Clear pending self-change suppression timers on unmount.
  useEffect(() => {
    return () => {
      for (const t of selfChangeTimers.current.values()) clearTimeout(t);
    };
  }, []);

  // Stable refs so effects can read current values without re-registering.
  const loadedKeysRef = useRef(loadedKeys);
  loadedKeysRef.current = loadedKeys;
  const loadedRef = useRef(loaded);
  loadedRef.current = loaded;

  // Debounced git-status refresh so the tree's status colors update live (on
  // save, external edits, file ops) without a manual refresh, while coalescing
  // bursts so we don't run the (potentially expensive) status scan repeatedly.
  const gitRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const scheduleGitRefresh = useCallback(() => {
    if (!useFeatureFlagsStore.getState().flags.gitIntegration) return; // respect the feature flag
    if (gitRefreshTimerRef.current) clearTimeout(gitRefreshTimerRef.current);
    gitRefreshTimerRef.current = setTimeout(() => { useGitStore.getState().refreshStatus(true); }, 400);
  }, []);
  useEffect(() => () => { if (gitRefreshTimerRef.current) clearTimeout(gitRefreshTimerRef.current); }, []);

  // Refresh git colors when a file is saved in the editor (watcher-independent).
  useEffect(() => {
    const handler = () => scheduleGitRefresh();
    window.addEventListener("thaw:file-saved", handler);
    return () => window.removeEventListener("thaw:file-saved", handler);
  }, [scheduleGitRefresh]);

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

  // ── File system watcher lifecycle ──────────────────────────────────────────
  // Only watch while the panel is expanded to conserve inotify watches on Linux.
  // On re-expand, refresh root entries to catch changes that happened while collapsed.
  useEffect(() => {
    if (!exportDir || !fileWatcherEnabled || !expanded) return;
    StartFileWatcher(exportDir).catch((e) => console.warn("File watcher failed to start:", e));
    // Refresh root to pick up changes that occurred while collapsed.
    if (loadedRef.current) {
      ListDirectory(exportDir)
        .then((entries) => setTreeData((prev) => mergeNodes(prev, entriesToNodes(entries))))
        .catch(() => {});
    }
    return () => { StopFileWatcher().catch(() => {}); };
  }, [exportDir, fileWatcherEnabled, expanded]);

  // ── File system change listener ────────────────────────────────────────────
  useEffect(() => {
    if (!exportDir || !fileWatcherEnabled) return;
    const off = EventsOn("fs:changed", (evt: { dir: string }) => {
      // Any disk change may alter git status — refresh colors even for the app's
      // own mutations (which suppress the tree update below to avoid flicker).
      scheduleGitRefresh();
      if (selfChangedDirs.current.has(evt.dir)) return;

      // After refreshing a directory, prune loadedKeys entries that reference
      // children which no longer exist (prevents unbounded stale-key growth).
      const pruneLoadedKeys = (freshKeys: Set<string>) => {
        setLoadedKeys((prev) => prev.filter((k) => {
          const ks = String(k);
          const parent = ks.substring(0, ks.lastIndexOf("/")) || ks.substring(0, ks.lastIndexOf("\\"));
          // Only prune keys whose parent is the refreshed directory.
          if (parent !== evt.dir) return true;
          return freshKeys.has(ks);
        }));
      };

      if (evt.dir === exportDir) {
        // Root directory changed — merge new entries into existing tree
        // so expanded subtrees (children) are preserved.
        ListDirectory(exportDir)
          .then((entries) => {
            const fresh = entriesToNodes(entries);
            setTreeData((prev) => mergeNodes(prev, fresh));
            pruneLoadedKeys(new Set(entries.map((e) => e.path)));
          })
          .catch(() => {});
        return;
      }
      // Only refresh directories that are already expanded (in loadedKeys).
      if (!loadedKeysRef.current.some((k) => String(k) === evt.dir)) return;
      ListDirectory(evt.dir)
        .then((entries) => {
          setTreeData((prev) => updateNode(prev, evt.dir, entriesToNodes(entries), true));
          pruneLoadedKeys(new Set(entries.map((e) => e.path)));
        })
        .catch(() => {});
    });
    return off;
  }, [exportDir, fileWatcherEnabled, scheduleGitRefresh]);

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
    setLoading(true);
    // Refresh git status alongside the tree so the status colors stay current
    // (silent: a status-fetch failure shouldn't pop an error from the file tree).
    refreshGitStatus(true);
    try {
      // Re-fetch the root and every currently-loaded (expanded) directory in
      // parallel, then merge — this picks up external changes while PRESERVING the
      // expanded subtree. Replacing treeData with root-only nodes (the old
      // behavior) desynced rc-tree's uncontrolled expand state: folders collapsed
      // and could not be reopened.
      const loaded = loadedKeysRef.current.map(String);
      const [rootEntries, ...childResults] = await Promise.all([
        ListDirectory(exportDir),
        ...loaded.map(async (k) => {
          try { return { key: k, entries: await ListDirectory(k) }; }
          catch { return { key: k, entries: null as FileEntry[] | null }; }
        }),
      ]);
      setTreeData((prev) => {
        let tree = mergeNodes(prev, entriesToNodes(rootEntries));
        for (const r of childResults) {
          if (r.entries) tree = updateNode(tree, r.key, entriesToNodes(r.entries), true);
        }
        return tree;
      });
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

  // gitStore records failures in state.error, which only ChangesView renders — so
  // here we surface it as a toast, otherwise context-menu git actions fail silently.
  const reportGit = (okMsg: string) => {
    const err = useGitStore.getState().error;
    if (err) message.error(err);
    else message.success(okMsg);
  };

  // The store's git index isn't safe to write concurrently; bail if any index op
  // (stage/unstage/discard, commit, or reset --hard) is mid-flight — otherwise
  // overlapping writes race on the index and on the shared `error` flag.
  const gitBusy = () => {
    const s = useGitStore.getState();
    if (s.staging || s.committing || s.resetting) {
      message.warning("A git action is already running — try again in a moment");
      return true;
    }
    return false;
  };

  const handleStage = () => {
    if (!fileCtxMenu || gitBusy()) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    // stageFile never rejects (it stores errors in gitStore.error); reportGit surfaces them.
    stageFile(path).then(() => reportGit(`Staged ${name}`));
  };

  const handleUnstage = () => {
    if (!fileCtxMenu || gitBusy()) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    unstageFile(path).then(() => reportGit(`Unstaged ${name}`));
  };

  // Open a diff of the file's working-tree content against its last-committed
  // (HEAD) state. HEAD content comes from go-git; a deleted file reads as empty
  // on the working side so the diff shows what was removed.
  const handleCompareWithHead = async () => {
    if (!fileCtxMenu) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    try {
      const head = await GitGetHeadFileContent(path);
      let current = "";
      try { current = await ReadFile(path); } catch { /* file deleted in worktree */ }
      useQueryStore.getState().openDiff(`HEAD · ${name}`, head ?? "", `Working tree · ${name}`, current);
    } catch (e) {
      message.error(`Could not compare with last commit: ${String(e)}`);
    }
  };

  const handleDiscardGit = () => {
    if (!fileCtxMenu || gitBusy()) return; // don't open the modal mid-op
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    // New files (no committed version) get deleted by discard. Use the dedicated
    // set, not the display letter — a staged-new-then-modified file shows "M" but
    // is still new (and would be permanently deleted).
    const rel = gitOverlay.relOf(path);
    const isNew = rel != null && gitOverlay.newFilesRel.has(rel);
    // Discard always reverts to HEAD, so a file with both staged and unstaged
    // changes loses its staged part too — warn about that. From the uncapped set
    // so it's correct beyond the 500-file cap.
    const partiallyStaged = rel != null && gitOverlay.partialRel.has(rel);
    modal.confirm({
      title: isNew ? `Delete ${name}?` : `Discard changes to ${name}?`,
      content: isNew
        ? "Permanently deletes this file — it has never been committed and cannot be recovered."
        : partiallyStaged
          ? "Reverts the file to its last committed state — this also discards your staged changes for this file. This cannot be undone."
          : "Reverts the file to its last committed state. This cannot be undone.",
      okText: isNew ? "Delete" : "Discard",
      okButtonProps: { danger: true },
      onOk: async () => {
        // throw (not return) — a resolved onOk closes the modal, which would read
        // as success even though nothing was discarded. Throwing keeps it open.
        if (gitBusy()) throw new Error("busy");
        await discardFile(path);
        reportGit(isNew ? `Deleted ${name}` : `Discarded changes to ${name}`);
      },
    });
  };

  // Repo-wide reset --hard: discard every staged and unstaged change.
  const handleDiscardAll = () => {
    if (gitBusy()) return; // don't open the modal mid-op
    setFileCtxMenu(null);
    modal.confirm({
      title: "Discard all changes?",
      content: "Resets the entire working tree to the last commit (git reset --hard HEAD). Every staged and unstaged change across all files is permanently lost. This cannot be undone.",
      okText: "Discard all",
      okButtonProps: { danger: true },
      onOk: async () => {
        if (gitBusy()) throw new Error("busy"); // keep modal open; a resolved onOk reads as success
        await resetHard();
        reportGit("Discarded all changes — working tree reset to last commit");
      },
    });
  };

  const handleDuplicate = async () => {
    if (!fileCtxMenu || fileCtxMenu.isDir) return;
    const { path } = fileCtxMenu;
    setFileCtxMenu(null);
    try {
      const newPath = await DuplicateFile(path);
      const name = pathBase(newPath);
      const parentDir = pathDir(newPath);
      markSelfChanged(parentDir);
      setTreeData(prev => addChild(prev, parentDir, makeNode(newPath, name, false)));
      message.success(`Created ${name}`);
    } catch (e) {
      message.error(`Duplicate failed: ${String(e)}`);
    }
  };

  const handleDeleteConfirm = () => {
    if (!fileCtxMenu) return;
    const { path, name, isDir } = fileCtxMenu;
    setFileCtxMenu(null);
    modal.confirm({
      title: `Delete ${isDir ? "folder" : "file"}`,
      content: `Are you sure you want to delete "${name}"?${isDir ? " This item and all its contents will be permanently removed." : ""}`,
      okText: "Delete",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          if (isDir) {
            await DeleteDirectory(path);
          } else {
            await DeleteFile(path);
          }
          markSelfChanged(pathDir(path));
          // Read fresh tabs from the store (not the stale closure captured at render time).
          const currentTabs = useQueryStore.getState().tabs;
          const sep = pathSep(path);
          for (const tab of currentTabs) {
            if (tab.path === path || (isDir && tab.path?.startsWith(path + sep))) {
              orphanTab(tab.id);
            }
          }
          message.success(`Deleted ${name}`);
          // Update tree in-place instead of full refresh.
          setTreeData(prev => removeNode(prev, path));
          setLoadedKeys(prev => prev.filter(k => {
            const ks = String(k);
            return ks !== path && !ks.startsWith(path + sep);
          }));
          setSelectedKey(prev => {
            const sk = prev ? String(prev) : null;
            if (sk === path || (isDir && sk?.startsWith(path + sep))) return null;
            return prev;
          });
        } catch (e) {
          message.error(`Delete failed: ${String(e)}`);
          throw e; // Re-throw to keep the modal open on failure.
        }
      },
    });
  };

  const handleRenameStart = () => {
    if (!fileCtxMenu) return;
    editActionRef.current = "idle";
    editInitRef.current = false;
    setEditingKey(fileCtxMenu.path);
    setEditingValue(fileCtxMenu.name);
    setFileCtxMenu(null);
  };

  const submitRename = async () => {
    if (editActionRef.current !== "idle" || editingKey === null) return;
    const path = String(editingKey);
    const sanitized = editingValue.trim().replace(/[/\\]/g, "");
    if (!sanitized || sanitized === pathBase(path)) {
      editActionRef.current = "cancelled";
      setEditingKey(null);
      return;
    }
    if (/[:"*?<>|]/.test(sanitized)) {
      message.error("Name contains invalid characters (: \" * ? < > |)");
      editActionRef.current = "cancelled";
      setEditingKey(null);
      return;
    }
    const dir = pathDir(path);
    const sep = pathSep(path);
    const newPath = dir.endsWith(sep) ? `${dir}${sanitized}` : `${dir}${sep}${sanitized}`;
    editActionRef.current = "submitting";
    try {
      await RenameFile(path, newPath);
      markSelfChanged(dir);
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
      setTreeData(prev => renameTreeNode(prev, path, newPath, sanitized));
      setLoadedKeys(prev => prev.map(k => {
        const ks = String(k);
        if (ks === path) return newPath;
        if (ks.startsWith(prefix)) return newPath + ks.substring(path.length);
        return k;
      }));
      setSelectedKey(prev => {
        const sk = prev ? String(prev) : null;
        if (sk === path) return newPath;
        if (sk?.startsWith(prefix)) return newPath + sk.substring(path.length);
        return prev;
      });
      setEditingKey(null);
      message.success(`Renamed to ${sanitized}`);
    } catch (e) {
      message.error(`Rename failed: ${String(e)}`);
      editActionRef.current = "idle"; // allow retry
    }
  };

  const cancelRename = () => {
    editActionRef.current = "cancelled";
    setEditingKey(null);
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
    if (isSubmitting) return;
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
    // Reject characters invalid on Windows (and generally problematic).
    if (/[:"*?<>|]/.test(sanitized)) {
      message.error("Name contains invalid characters (: \" * ? < > |)");
      return;
    }
    setIsSubmitting(true);
    try {
      if (kind === "newFolder") {
        const sep = pathSep(path);
        const folderPath = `${path}${sep}${sanitized}`;
        await CreateDirectory(folderPath);
        markSelfChanged(path);
        setTreeData(prev => addChild(prev, path, makeNode(folderPath, sanitized, true)));
        message.success(`Created folder ${sanitized}`);
      } else {
        const sep = pathSep(path);
        const name = sanitized.endsWith(".sql") ? sanitized : `${sanitized}.sql`;
        const filePath = `${path}${sep}${name}`;
        await CreateFile(filePath);
        markSelfChanged(path);
        setTreeData(prev => addChild(prev, path, makeNode(filePath, name, false)));
        message.success(`Created ${name}`);
      }
      setInlineInput(null);
    } catch (e) {
      const prefix = kind === "newFolder" ? "Could not create folder" : "Could not create file";
      message.error(`${prefix}: ${String(e)}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  const titleRender = (nodeData: DataNode) => {
    if (editingKey !== null && nodeData.key === editingKey) {
      return (
        <Input
          size="small"
          autoFocus
          value={editingValue}
          onChange={(e) => setEditingValue(e.target.value)}
          onKeyDown={(e) => {
            e.stopPropagation(); // prevent tree keyboard navigation
            if (e.key === "Enter") submitRename();
            else if (e.key === "Escape") cancelRename();
          }}
          onBlur={submitRename}
          onClick={(e) => e.stopPropagation()} // prevent tree selection
          style={{ fontSize: 12, height: 22, padding: "0 4px" }}
          ref={(el) => {
            if (!el || editInitRef.current) return;
            editInitRef.current = true;
            const input = (el as any).input ?? el;
            if (input?.setSelectionRange) {
              const dot = editingValue.lastIndexOf(".");
              const end = dot > 0 ? dot : editingValue.length;
              requestAnimationFrame(() => input.setSelectionRange(0, end));
            }
          }}
        />
      );
    }
    // Git status overlay: color changed files (with a trailing sigil) and
    // emphasize directories that contain nested changes.
    const rel = gitOverlay.relOf(String(nodeData.key));
    const letter = rel != null ? gitOverlay.byRel.get(rel) : undefined;
    if (letter) {
      const color = sigilColor(letter);
      return (
        <span style={{ display: "inline-flex", alignItems: "center", gap: 6, width: "100%" }}>
          <span style={{ flex: 1, color, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {nodeData.title as React.ReactNode}
          </span>
          <span style={{ fontFamily: 'var(--editor-font, monospace)', fontSize: 10, fontWeight: 600, color, flexShrink: 0 }}>
            {letter}
          </span>
        </span>
      );
    }
    const dirLetter = rel != null ? gitOverlay.dirLetter.get(rel) : undefined;
    if (dirLetter) {
      const color = sigilColor(dirLetter);
      return <span style={{ color, fontWeight: 600 }}>{nodeData.title as React.ReactNode}</span>;
    }
    return <>{nodeData.title}</>;
  };

  const grouped = groupByPath(searchResults);

  // rc-tree memoizes its flattened node list by treeData identity, so a status
  // change alone won't re-run titleRender. Hand it a fresh top-level array
  // reference whenever the git overlay changes so the status colors repaint.
  const treeForRender = useMemo(() => [...treeData], [treeData, gitOverlay]);

  // Git status of the right-clicked file (drives the Stage/Unstage/Discard menu items).
  // ctxChanged: the file is changed at all. When we don't have a precise
  // staged/unstaged side (legacy-only data), offer both Stage and Unstage and let
  // the backend no-op the irrelevant one.
  const ctxRel       = fileCtxMenu && !fileCtxMenu.isDir ? gitOverlay.relOf(fileCtxMenu.path) : null;
  const ctxChanged   = ctxRel != null && gitOverlay.byRel.has(ctxRel);
  const ctxStagedHit   = ctxRel != null && gitOverlay.stagedRel.has(ctxRel);
  const ctxUnstagedHit = ctxRel != null && gitOverlay.unstagedRel.has(ctxRel);
  const ctxUnknownSide = ctxChanged && !ctxStagedHit && !ctxUnstagedHit;
  const ctxStaged   = ctxStagedHit   || ctxUnknownSide; // show Unstage
  const ctxUnstaged = ctxUnstagedHit || ctxUnknownSide; // show Stage
  // Comparable against HEAD only when there's a prior committed version — i.e. a
  // tracked change (modified/renamed/copied/deleted), not a brand-new file (A/U).
  const ctxLetter     = ctxRel != null ? gitOverlay.byRel.get(ctxRel) : undefined;
  const ctxComparable = ctxLetter === "M" || ctxLetter === "R" || ctxLetter === "C" || ctxLetter === "D";

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
        {/* Branch chip — folded in from the former Git panel; opens Git Operations */}
        {gitRepo && (
          <div
            onClick={(e) => { e.stopPropagation(); openGitOps(); }}
            title={`On branch ${gitBranch}${gitAhead > 0 ? ` · ${gitAhead} to push` : ""} — open Git Operations`}
            style={{ display: "flex", alignItems: "center", gap: 3, maxWidth: 96, cursor: "pointer", padding: "1px 5px", borderRadius: 4, background: "color-mix(in srgb, var(--text) 6%, transparent)" }}
          >
            <BranchesOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />
            <span style={{ fontFamily: 'var(--editor-font, monospace)', fontSize: 10, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {gitBranch}{gitAhead > 0 ? ` ↑${gitAhead}` : ""}
            </span>
          </div>
        )}
        {/* Changed-file count — at a glance, and opens Git Operations */}
        {gitRepo && gitChanged > 0 && (
          <div
            onClick={(e) => { e.stopPropagation(); openGitOps(); }}
            title={`${gitChanged} changed${gitStagedTot > 0 ? `, ${gitStagedTot} staged` : ""} — open Git Operations`}
            style={{ display: "flex", alignItems: "center", gap: 3, cursor: "pointer", padding: "1px 5px", borderRadius: 4, background: "color-mix(in srgb, var(--warning) 16%, transparent)" }}
          >
            <span style={{ fontFamily: 'var(--editor-font, monospace)', fontSize: 10, fontWeight: 600, color: "var(--warning)" }}>
              {gitChanged}{gitStagedTot > 0 ? `·${gitStagedTot}` : ""}
            </span>
          </div>
        )}
        <Button
          size="small"
          type="text"
          icon={<SearchOutlined style={{ fontSize: 11, color: searchOpen ? "var(--link)" : CLR_SECONDARY }} />}
          onClick={toggleSearch}
          style={{ height: 20, padding: "0 4px", minWidth: 0 }}
        />
        {gitEnabled && exportDir && (
          <Tooltip title="Git Operations…">
            <Button
              size="small"
              type="text"
              icon={<BranchesOutlined style={{ fontSize: 11 }} />}
              onClick={(e) => { e.stopPropagation(); openGitOps(); }}
              style={{ height: 20, padding: "0 4px", minWidth: 0 }}
            />
          </Tooltip>
        )}
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
            <div style={{ display: "flex", flexDirection: "column", gap: 6, padding: "2px 0 6px" }}>
              <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>No working directory selected.</Text>
              <Button size="small" icon={<FolderOpenOutlined />} onClick={pickExportDir}>Pick directory…</Button>
            </div>
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
                treeData={treeForRender}
                loadedKeys={loadedKeys}
                selectedKeys={selectedKey ? [selectedKey] : []}
                onLoad={(keys) => setLoadedKeys(keys)}
                loadData={onLoadData as any}
                onSelect={onSelect as any}
                onRightClick={onRightClick as any}
                titleRender={titleRender}
                showIcon
                blockNode
                style={{ background: "transparent", color: "var(--text)", fontSize: 12 }}
              />
            </div>
          )}
        </div>
      )}

      {/* Modal for new folder / new file */}
      {inlineInput && (
        <Modal
          open
          title={inlineInput.kind === "newFolder" ? "New Folder" : "New SQL File"}
          okText="Create"
          confirmLoading={isSubmitting}
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
            onPressEnter={() => { if (!isSubmitting) submitInlineInput(); }}
            placeholder={inlineInput.kind === "newFolder" ? "Folder name" : "File name (.sql)"}
            style={{ marginTop: 8 }}
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
          onKeyDown={(e) => {
            // WAI-ARIA menu pattern: ArrowDown/ArrowUp/Home/End navigate between items.
            const items = fileCtxRef.current?.querySelectorAll<HTMLElement>("[role='menuitem']");
            if (!items?.length) return;
            const idx = Array.from(items).indexOf(document.activeElement as HTMLElement);
            let next = -1;
            if (e.key === "ArrowDown") next = idx < items.length - 1 ? idx + 1 : 0;
            else if (e.key === "ArrowUp") next = idx > 0 ? idx - 1 : items.length - 1;
            else if (e.key === "Home") next = 0;
            else if (e.key === "End") next = items.length - 1;
            if (next >= 0) { e.preventDefault(); items[next].focus(); }
          }}
          onBlur={(e) => {
            // Dismiss menu when focus leaves the container entirely.
            // relatedTarget can be null in WKWebView during focus transitions
            // between sibling elements — defer check to next microtask.
            if (e.relatedTarget && !e.currentTarget.contains(e.relatedTarget as Node)) {
              setFileCtxMenu(null);
            } else if (!e.relatedTarget) {
              setTimeout(() => {
                if (!fileCtxRef.current?.contains(document.activeElement)) {
                  setFileCtxMenu(null);
                }
              }, 0);
            }
          }}
        >
          {/* ── File management actions ── */}
          <CtxItem icon={<FolderViewOutlined />} label={revealText} onClick={handleReveal} />
          <CtxItem icon={<CopyOutlined />} label="Copy Path" onClick={handleCopyPath} />
          {!fileCtxMenu.isDir && (
            <CtxItem icon={<SnippetsOutlined />} label="Duplicate" onClick={handleDuplicate} />
          )}
          <CtxItem icon={<EditOutlined />} label="Rename…" onClick={handleRenameStart} />
          <CtxItem icon={<DeleteOutlined />} label="Delete" onClick={handleDeleteConfirm} danger />

          {/* ── Git staging actions (changed files only) ── */}
          {gitEnabled && !fileCtxMenu.isDir && (ctxUnstaged || ctxStaged) && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              {ctxComparable && (
                <CtxItem icon={<DiffOutlined />} label="Compare with last commit" onClick={handleCompareWithHead} />
              )}
              {ctxUnstaged && (
                <CtxItem icon={<PlusOutlined />} label="Stage" onClick={handleStage} />
              )}
              {ctxStaged && (
                <CtxItem icon={<MinusOutlined />} label="Unstage" onClick={handleUnstage} />
              )}
              <CtxItem icon={<UndoOutlined />} label="Discard changes" onClick={handleDiscardGit} danger />
            </>
          )}

          {/* ── Repo-wide discard (reset --hard) — shown whenever the repo has changes ── */}
          {gitRepo && gitChanged > 0 && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              <CtxItem icon={<UndoOutlined />} label="Discard all changes (reset to last commit)" onClick={handleDiscardAll} danger />
            </>
          )}

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
      // Stop blur propagation so moving focus between items doesn't dismiss the parent menu.
      onBlur={(e) => { e.stopPropagation(); e.currentTarget.style.background = "transparent"; }}
      onClick={onClick}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onClick(); } }}
    >
      <span style={{ fontSize: 12, display: "flex" }}>{icon}</span>
      {label}
    </div>
  );
}
