import { useState, useEffect } from "react";
import { Button, Input, Typography, Spin, Alert, Badge, Collapse, Space, Tooltip } from "antd";
import {
  FolderOpenOutlined,
  ReloadOutlined,
  BranchesOutlined,
  CloudUploadOutlined,
} from "@ant-design/icons";
import { useGitStore } from "../../store/gitStore";

const { Text } = Typography;
const { TextArea } = Input;

const CLR_BORDER    = "#30363d";
const CLR_SECONDARY = "#8b949e";
const CLR_ADDED     = "#3fb950";
const CLR_MODIFIED  = "#d29922";
const CLR_DELETED   = "#f85149";

export default function GitPanel() {
  const { dir, status, loading, pushing, error, pickDir, refreshStatus, push, clearError } =
    useGitStore();

  const [remoteURL,   setRemoteURL]   = useState("");
  const [branch,      setBranch]      = useState("main");
  const [message,     setMessage]     = useState("");
  const [authorName,  setAuthorName]  = useState("");
  const [authorEmail, setAuthorEmail] = useState("");
  const [token,       setToken]       = useState("");

  // Pre-fill remote URL when status loads it from the repo config
  useEffect(() => {
    if (status?.remoteURL && !remoteURL) {
      setRemoteURL(status.remoteURL);
    }
  }, [status?.remoteURL]);

  const totalChanged =
    (status?.modified?.length ?? 0) +
    (status?.added?.length   ?? 0) +
    (status?.deleted?.length ?? 0);

  const handlePush = async () => {
    await push({
      remoteURL:   remoteURL || status?.remoteURL || "",
      branch:      branch || "main",
      token,
      message:     message || "chore: export Snowflake DDL",
      authorName,
      authorEmail,
    });
    if (!useGitStore.getState().error) {
      setMessage("");
      setToken("");
    }
  };

  const headerLabel = (() => {
    if (!status?.isRepo) return "Git";
    const b = status.branch || "main";
    return status.ahead > 0 ? `Git · ${b} (↑${status.ahead})` : `Git · ${b}`;
  })();

  return (
    <div style={{ borderTop: `1px solid ${CLR_BORDER}`, marginTop: 8 }}>
      <Collapse
        ghost
        defaultActiveKey={[]}
        style={{ background: "transparent" }}
        items={[{
          key: "git",
          label: (
            <Space size={6}>
              <BranchesOutlined style={{ color: CLR_SECONDARY, fontSize: 13 }} />
              <Text style={{ fontSize: 11, color: CLR_SECONDARY, textTransform: "uppercase", letterSpacing: "0.08em" }}>
                {headerLabel}
              </Text>
              {totalChanged > 0 && (
                <Badge
                  count={totalChanged}
                  size="small"
                  style={{ backgroundColor: CLR_MODIFIED, fontSize: 10 }}
                />
              )}
            </Space>
          ),
          style: { border: "none" },
          children: (
            <div style={{ display: "flex", flexDirection: "column", gap: 8, padding: "0 4px 8px" }}>

              {/* ── Directory ─────────────────────────────────────── */}
              <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
                <Text
                  style={{
                    flex: 1, fontSize: 11, fontFamily: "monospace",
                    color: dir ? "#e6edf3" : CLR_SECONDARY,
                    overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
                  }}
                  title={dir}
                >
                  {dir || "No directory selected"}
                </Text>
                <Tooltip title="Pick directory">
                  <Button size="small" icon={<FolderOpenOutlined />} onClick={pickDir} />
                </Tooltip>
                {dir && (
                  <Tooltip title="Refresh">
                    <Button
                      size="small"
                      icon={<ReloadOutlined spin={loading} />}
                      onClick={refreshStatus}
                      disabled={loading}
                    />
                  </Tooltip>
                )}
              </div>

              {/* ── Status ────────────────────────────────────────── */}
              {dir && loading && (
                <Spin size="small" style={{ display: "block", margin: "4px auto" }} />
              )}

              {dir && status && !loading && (
                <>
                  {!status.isRepo && (
                    <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>
                      Not a git repository — will be initialised on push.
                    </Text>
                  )}
                  {status.isRepo && totalChanged === 0 && (
                    <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>
                      Working tree clean.
                      {status.ahead > 0 && ` ${status.ahead} commit(s) not yet pushed.`}
                    </Text>
                  )}
                  {totalChanged > 0 && (
                    <div style={{ maxHeight: 140, overflowY: "auto", display: "flex", flexDirection: "column", gap: 2 }}>
                      {status.added?.map((f) => (
                        <Text key={f} style={{ fontSize: 11, fontFamily: "monospace", color: CLR_ADDED }}>+ {f}</Text>
                      ))}
                      {status.modified?.map((f) => (
                        <Text key={f} style={{ fontSize: 11, fontFamily: "monospace", color: CLR_MODIFIED }}>~ {f}</Text>
                      ))}
                      {status.deleted?.map((f) => (
                        <Text key={f} style={{ fontSize: 11, fontFamily: "monospace", color: CLR_DELETED }}>- {f}</Text>
                      ))}
                    </div>
                  )}
                </>
              )}

              {/* ── Remote + Branch + Auth ────────────────────────── */}
              {dir && (
                <>
                  <Input
                    size="small"
                    placeholder="https://github.com/org/repo.git"
                    value={remoteURL}
                    onChange={(e) => setRemoteURL(e.target.value)}
                    style={{ fontSize: 12 }}
                    addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Remote</Text>}
                  />
                  <Input
                    size="small"
                    placeholder="main"
                    value={branch}
                    onChange={(e) => setBranch(e.target.value)}
                    style={{ fontSize: 12 }}
                    addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Branch</Text>}
                  />

                  <Collapse
                    ghost
                    size="small"
                    style={{ background: "transparent", marginLeft: -8 }}
                    items={[{
                      key: "auth",
                      label: <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Author & token</Text>,
                      style: { border: "none" },
                      children: (
                        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                          <Input
                            size="small"
                            placeholder="Your Name"
                            value={authorName}
                            onChange={(e) => setAuthorName(e.target.value)}
                            style={{ fontSize: 12 }}
                            addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Name</Text>}
                          />
                          <Input
                            size="small"
                            placeholder="you@example.com"
                            value={authorEmail}
                            onChange={(e) => setAuthorEmail(e.target.value)}
                            style={{ fontSize: 12 }}
                            addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Email</Text>}
                          />
                          <Input.Password
                            size="small"
                            placeholder="GitHub PAT (ghp_…)"
                            value={token}
                            onChange={(e) => setToken(e.target.value)}
                            style={{ fontSize: 12 }}
                            visibilityToggle
                          />
                        </div>
                      ),
                    }]}
                  />

                  <TextArea
                    size="small"
                    rows={2}
                    placeholder="Commit message (default: chore: export Snowflake DDL)"
                    value={message}
                    onChange={(e) => setMessage(e.target.value)}
                    style={{ fontSize: 12, resize: "none" }}
                  />

                  <Button
                    type="primary"
                    size="small"
                    block
                    icon={<CloudUploadOutlined />}
                    loading={pushing}
                    disabled={loading}
                    onClick={handlePush}
                  >
                    {pushing ? "Pushing…" : "Commit & Push"}
                  </Button>
                </>
              )}

              {/* ── Error ─────────────────────────────────────────── */}
              {error && (
                <Alert
                  type="error"
                  message={error}
                  showIcon
                  closable
                  onClose={clearError}
                  style={{ fontSize: 11 }}
                />
              )}
            </div>
          ),
        }]}
      />
    </div>
  );
}
