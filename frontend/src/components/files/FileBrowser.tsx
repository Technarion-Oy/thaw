// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Tree, Typography, Spin, Collapse, Space, Button, message } from "antd";
import {
  FolderOutlined,
  FolderOpenOutlined,
  FileOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import type { DataNode, EventDataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import { ListDirectory, ReadFile } from "../../../wailsjs/go/main/App";
import { useGitStore } from "../../store/gitStore";
import { useQueryStore } from "../../store/queryStore";
import type { filesystem } from "../../../wailsjs/go/models";

type FileEntry = filesystem.FileEntry;

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

export default function FileBrowser() {
  const exportDir  = useGitStore((s) => s.exportDir);
  const openFile   = useQueryStore((s) => s.openFile);
  const currentFile = useQueryStore((s) => s.currentFile);

  const [treeData,    setTreeData]    = useState<DataNode[]>([]);
  const [loadedKeys,  setLoadedKeys]  = useState<Key[]>([]);
  const [selectedKey, setSelectedKey] = useState<Key | null>(null);
  const [loading,     setLoading]     = useState(false);
  const [loaded,      setLoaded]      = useState(false);

  // Reset tree when the working directory changes
  useEffect(() => {
    setLoaded(false);
    setTreeData([]);
    setLoadedKeys([]);
    setSelectedKey(null);
  }, [exportDir]);

  // Keep selected key in sync when file is opened from elsewhere (e.g. DDL view)
  useEffect(() => {
    if (!currentFile) setSelectedKey(null);
  }, [currentFile]);

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
    if ((node as any).isLeaf === false) return; // directory — let Tree handle expand
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
    if ((Array.isArray(keys) ? keys : [keys]).includes("files")) {
      loadRoot();
    }
  };

  return (
    <div style={{ borderTop: `1px solid ${CLR_BORDER}` }}>
      <Collapse
        ghost
        defaultActiveKey={[]}
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
          extra: loaded ? (
            <Button
              size="small"
              type="text"
              icon={<ReloadOutlined style={{ fontSize: 11 }} />}
              loading={loading}
              onClick={(e) => { e.stopPropagation(); refresh(); }}
              style={{ height: 18, padding: "0 4px", minWidth: 0 }}
            />
          ) : undefined,
          children: (
            <div style={{ padding: "0 4px 8px" }}>
              {!exportDir && (
                <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>
                  Set a working directory in the Git section below.
                </Text>
              )}

              {exportDir && loading && (
                <Spin size="small" style={{ display: "block", margin: "8px auto" }} />
              )}

              {exportDir && !loading && loaded && treeData.length === 0 && (
                <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Directory is empty.</Text>
              )}

              {loaded && treeData.length > 0 && (
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
