// SPDX-License-Identifier: GPL-3.0-or-later

import { useMemo, useState } from "react";
import { Dropdown, Button, Input } from "antd";
import { FunctionOutlined, SearchOutlined } from "@ant-design/icons";
import {
  DEFAULT_FUNCTIONS,
  DEFAULT_FUNCTION_CATEGORIES,
  type BuiltinFn,
} from "./builtinFunctions";

/**
 * Small dropdown button that fills a column DEFAULT with a Snowflake built-in
 * function. The panel is searchable and groups the curated catalog by category
 * so the full set of functions valid as a CREATE TABLE column DEFAULT is easy to
 * scan. Shared by the Create Table dialog and the ER Designer column editor.
 * Function-expression defaults are rejected on an existing column, so this
 * picker is not used by the Column Properties modal (see SequenceDefaultPicker).
 */
export default function DefaultFunctionPicker({ onPick }: { onPick: (sql: string) => void }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const groups = useMemo(() => {
    const q = query.trim().toLowerCase();
    const match = (f: BuiltinFn) =>
      q === "" ||
      f.name.toLowerCase().includes(q) ||
      f.sql.toLowerCase().includes(q) ||
      f.desc.toLowerCase().includes(q);
    return DEFAULT_FUNCTION_CATEGORIES.map((category) => ({
      category,
      fns: DEFAULT_FUNCTIONS.filter((f) => f.category === category && match(f)),
    })).filter((g) => g.fns.length > 0);
  }, [query]);

  function pick(f: BuiltinFn) {
    onPick(f.sql);
    setOpen(false);
    setQuery("");
  }

  return (
    <Dropdown
      open={open}
      trigger={["click"]}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery("");
      }}
      dropdownRender={() => (
        <div
          style={{
            width: 340,
            background: "var(--bg-raised)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            boxShadow: "0 6px 16px rgba(0,0,0,0.24)",
            overflow: "hidden",
          }}
        >
          <div style={{ padding: 8, borderBottom: "1px solid var(--border)" }}>
            <Input
              size="small"
              autoFocus
              allowClear
              prefix={<SearchOutlined style={{ color: "var(--text-muted)" }} />}
              placeholder="Search functions…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                // Enter picks the single remaining match — quick keyboard flow.
                if (e.key === "Enter") {
                  const only = groups.length === 1 && groups[0].fns.length === 1;
                  if (only) pick(groups[0].fns[0]);
                }
              }}
            />
          </div>
          <div style={{ maxHeight: 300, overflowY: "auto", padding: "4px 0" }}>
            {groups.length === 0 ? (
              <div style={{ padding: "12px", fontSize: 12, color: "var(--text-muted)", textAlign: "center" }}>
                No functions match “{query}”
              </div>
            ) : (
              groups.map((g) => (
                <div key={g.category}>
                  <div
                    style={{
                      padding: "4px 12px",
                      fontSize: 10,
                      fontWeight: 600,
                      textTransform: "uppercase",
                      letterSpacing: 0.4,
                      color: "var(--text-muted)",
                    }}
                  >
                    {g.category}
                  </div>
                  {g.fns.map((f) => (
                    <div
                      key={f.sql}
                      onClick={() => pick(f)}
                      title={f.desc}
                      style={{
                        padding: "4px 12px",
                        cursor: "pointer",
                        lineHeight: 1.35,
                      }}
                      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-hover)")}
                      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                    >
                      <code style={{ fontSize: 12 }}>{f.sql}</code>
                      <div style={{ color: "var(--text-muted)", fontSize: 11 }}>{f.desc}</div>
                    </div>
                  ))}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    >
      <Button size="small" icon={<FunctionOutlined />} title="Insert function default" />
    </Dropdown>
  );
}
