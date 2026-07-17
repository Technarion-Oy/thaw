// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useMemo, useState } from "react";
import { Modal, Input, Collapse, Tag, Spin, Empty } from "antd";
import { SafetyCertificateOutlined, SearchOutlined } from "@ant-design/icons";
import { GetThirdPartyNotices } from "../../../wailsjs/go/app/App";
import { parseNotices } from "./parseNotices";

interface Props { onClose: () => void; }

export default function ThirdPartyNoticesModal({ onClose }: Props) {
  // `md` is the fetched Markdown; `loaded` tracks whether the fetch has settled.
  // These are separate because an empty string is a valid *loaded* value (fetch
  // failure or empty file) and must fall through to the Empty state rather than
  // being mistaken for "still loading".
  const [md, setMd] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [search, setSearch] = useState("");

  useEffect(() => {
    GetThirdPartyNotices()
      .then((text) => setMd(text))
      .catch(() => setMd(""))
      .finally(() => setLoaded(true));
  }, []);

  const parsed = useMemo(() => (loaded ? parseNotices(md) : null), [loaded, md]);

  const q = search.trim().toLowerCase();
  const groups = useMemo(() => {
    if (!parsed) return [];
    // Drop package-less sections (e.g. the "Contents" table of contents) and,
    // when searching, packages that don't match.
    return parsed.groups
      .map((g) => ({
        ...g,
        packages: q
          ? g.packages.filter(
              (p) =>
                p.name.toLowerCase().includes(q) ||
                p.license.toLowerCase().includes(q),
            )
          : g.packages,
      }))
      .filter((g) => g.packages.length > 0);
  }, [parsed, q]);

  const total = parsed ? parsed.groups.reduce((n, g) => n + g.packages.length, 0) : 0;

  return (
    <Modal
      open
      title={
        <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <SafetyCertificateOutlined />
          Acknowledgements & Third-Party Licenses
        </span>
      }
      onCancel={onClose}
      footer={null}
      width={780}
      styles={{ body: { padding: "12px 16px 16px" } }}
    >
      {parsed === null ? (
        <div style={{ padding: "40px 0", textAlign: "center" }}>
          <Spin />
        </div>
      ) : total === 0 ? (
        <Empty
          style={{ padding: "24px 0" }}
          description="Could not load the third-party notices."
        />
      ) : (
        <>
          {parsed.intro.length > 0 && (
            <div style={{ fontSize: 13, color: "var(--text-secondary, #aaa)", marginBottom: 12 }}>
              {parsed.intro.map((p, i) => (
                <p key={i} style={{ margin: i === 0 ? "0 0 8px" : "8px 0 0" }}>{p}</p>
              ))}
            </div>
          )}

          <Input
            prefix={<SearchOutlined style={{ color: "var(--text-muted, #888)" }} />}
            placeholder={`Search ${total} packages by name or license…`}
            allowClear
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ marginBottom: 14 }}
            autoFocus
          />

          <div style={{ maxHeight: 520, overflowY: "auto" }}>
            {groups.length === 0 ? (
              <Empty description="No matching packages" />
            ) : (
              groups.map((group) => (
                <div key={group.title} style={{ marginBottom: 18 }}>
                  <div style={{
                    fontSize: 11,
                    fontWeight: 600,
                    textTransform: "uppercase",
                    letterSpacing: "0.06em",
                    color: "var(--text-muted, #888)",
                    marginBottom: 8,
                    paddingBottom: 4,
                    borderBottom: "1px solid var(--border-color, #303030)",
                  }}>
                    {group.title} ({group.packages.length})
                  </div>
                  <Collapse
                    accordion
                    size="small"
                    items={group.packages.map((p) => ({
                      // Key by name@version: a few packages (immer, react-is,
                      // zustand) are bundled at more than one version, so name
                      // alone is not unique.
                      key: `${p.name}@${p.version}`,
                      label: (
                        <span style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                          <span style={{ fontFamily: "var(--font-mono, monospace)", fontSize: 12 }}>{p.name}</span>
                          {p.version && (
                            <span style={{ color: "var(--text-muted, #888)", fontSize: 11 }}>{p.version}</span>
                          )}
                          {p.license && <Tag color="blue" style={{ marginInlineEnd: 0 }}>{p.license}</Tag>}
                        </span>
                      ),
                      children: (
                        <pre style={{
                          margin: 0,
                          maxHeight: 320,
                          overflow: "auto",
                          fontSize: 11,
                          lineHeight: 1.5,
                          whiteSpace: "pre-wrap",
                          wordBreak: "break-word",
                          color: "var(--text-secondary, #bbb)",
                        }}>
                          {p.text || "No license text was bundled with this package."}
                        </pre>
                      ),
                    }))}
                  />
                </div>
              ))
            )}
          </div>
        </>
      )}
    </Modal>
  );
}
