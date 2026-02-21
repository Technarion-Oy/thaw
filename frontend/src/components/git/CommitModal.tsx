import { useState, useMemo } from "react";
import { Modal, Checkbox, Button, Input, Space, Tag, Typography, Divider } from "antd";
import { CheckSquareOutlined, CloseSquareOutlined, CloudUploadOutlined } from "@ant-design/icons";
import type { RepoStatus } from "../../store/gitStore";

const { Text } = Typography;
const { TextArea } = Input;

const CLR_ADDED    = "#3fb950";
const CLR_MODIFIED = "#d29922";
const CLR_DELETED  = "#f85149";

interface FileEntry {
  path: string;
  change: "added" | "modified" | "deleted";
}

interface Props {
  status: RepoStatus;
  pushing: boolean;
  onClose: () => void;
  onCommit: (files: string[], message: string, token: string) => Promise<void>;
}

function extOf(path: string): string {
  const dot = path.lastIndexOf(".");
  return dot >= 0 ? path.slice(dot).toLowerCase() : "(no ext)";
}

export default function CommitModal({ status, pushing, onClose, onCommit }: Props) {
  const allFiles: FileEntry[] = useMemo(() => {
    const files: FileEntry[] = [];
    for (const f of (status.added    ?? [])) files.push({ path: f, change: "added" });
    for (const f of (status.modified ?? [])) files.push({ path: f, change: "modified" });
    for (const f of (status.deleted  ?? [])) files.push({ path: f, change: "deleted" });
    return files;
  }, [status]);

  const [selected, setSelected] = useState<Set<string>>(
    () => new Set(allFiles.map((f) => f.path))
  );
  const [message, setMessage] = useState("");
  const [token,   setToken]   = useState("");

  const extensions = useMemo(() => {
    const exts = new Set(allFiles.map((f) => extOf(f.path)));
    return [...exts].sort();
  }, [allFiles]);

  const toggle = (path: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const selectAll  = () => setSelected(new Set(allFiles.map((f) => f.path)));
  const selectNone = () => setSelected(new Set());
  const selectByExt = (ext: string) =>
    setSelected(new Set(allFiles.filter((f) => extOf(f.path) === ext).map((f) => f.path)));

  const handleCommit = () => onCommit([...selected], message, token);

  const clrOf     = (c: FileEntry["change"]) =>
    c === "added" ? CLR_ADDED : c === "modified" ? CLR_MODIFIED : CLR_DELETED;
  const prefixOf  = (c: FileEntry["change"]) =>
    c === "added" ? "+" : c === "modified" ? "~" : "-";

  return (
    <Modal
      open
      title="Commit & Push"
      onCancel={onClose}
      width={620}
      styles={{ body: { padding: "12px 16px" } }}
      footer={[
        <Button key="cancel" onClick={onClose} disabled={pushing}>
          Cancel
        </Button>,
        <Button
          key="commit"
          type="primary"
          icon={<CloudUploadOutlined />}
          loading={pushing}
          disabled={selected.size === 0}
          onClick={handleCommit}
        >
          {pushing ? "Pushing…" : `Commit & Push (${selected.size} file${selected.size !== 1 ? "s" : ""})`}
        </Button>,
      ]}
    >
      {/* ── File selection toolbar ──────────────────────────── */}
      <Space wrap size={4} style={{ marginBottom: 8 }}>
        <Button size="small" icon={<CheckSquareOutlined />} onClick={selectAll}>
          Select All
        </Button>
        <Button
          size="small"
          icon={<CloseSquareOutlined />}
          onClick={selectNone}
          disabled={selected.size === 0}
        >
          None
        </Button>
        {extensions.length > 1 && (
          <>
            <Divider type="vertical" style={{ borderColor: "#30363d" }} />
            {extensions.map((ext) => (
              <Tag
                key={ext}
                style={{ cursor: "pointer", userSelect: "none", fontSize: 11 }}
                onClick={() => selectByExt(ext)}
              >
                {ext}
              </Tag>
            ))}
          </>
        )}
      </Space>

      {/* ── File list ──────────────────────────────────────── */}
      <div
        style={{
          maxHeight: 280,
          overflowY: "auto",
          border: "1px solid #30363d",
          borderRadius: 6,
          padding: "4px 0",
          background: "#0d1117",
          marginBottom: 12,
        }}
      >
        {allFiles.length === 0 && (
          <Text style={{ display: "block", padding: "12px 16px", color: "#8b949e", fontSize: 12 }}>
            No changes detected.
          </Text>
        )}
        {allFiles.map(({ path, change }) => (
          <div
            key={path}
            style={{ display: "flex", alignItems: "center", gap: 8, padding: "3px 10px", cursor: "pointer" }}
            onClick={() => toggle(path)}
          >
            <Checkbox checked={selected.has(path)} onChange={() => toggle(path)} onClick={(e) => e.stopPropagation()} />
            <Text style={{ fontFamily: "monospace", fontSize: 12, color: clrOf(change), flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {prefixOf(change)} {path}
            </Text>
          </div>
        ))}
      </div>

      <Divider style={{ borderColor: "#30363d", margin: "0 0 10px" }} />

      {/* ── Commit message ─────────────────────────────────── */}
      <TextArea
        size="small"
        rows={2}
        placeholder="Commit message (default: chore: export Snowflake DDL)"
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        style={{ fontSize: 12, resize: "none", marginBottom: 8 }}
      />

      {/* ── Token ──────────────────────────────────────────── */}
      <Input.Password
        size="small"
        placeholder="GitHub PAT (ghp_…) — not saved"
        value={token}
        onChange={(e) => setToken(e.target.value)}
        style={{ fontSize: 12 }}
        visibilityToggle
      />
    </Modal>
  );
}
