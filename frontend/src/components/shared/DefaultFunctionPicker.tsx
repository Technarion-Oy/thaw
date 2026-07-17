// SPDX-License-Identifier: GPL-3.0-or-later

import { useMemo, useState } from "react";
import { Dropdown, Button, Input } from "antd";
import { FunctionOutlined, SearchOutlined } from "@ant-design/icons";
import {
  filterDefaultFunctions,
  type BuiltinFn,
} from "./builtinFunctions";

/**
 * Small dropdown button that fills a column DEFAULT with a Snowflake built-in
 * function. The panel is searchable and groups the curated catalog by category
 * so the full set of functions valid as a CREATE TABLE column DEFAULT is easy to
 * scan. Arrow keys move the highlight and Enter picks it, so the panel stays
 * keyboard-operable (matching the old antd Menu it replaced). Shared by the
 * Create Table dialog and the ER Designer column editor. Function-expression
 * defaults are rejected on an existing column, so this picker is not used by the
 * Column Properties modal (see SequenceDefaultPicker).
 */
export default function DefaultFunctionPicker({ onPick }: { onPick: (sql: string) => void }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);

  const groups = useMemo(() => filterDefaultFunctions(query), [query]);

  // Flatten the visible functions so arrow keys can walk across categories.
  const flat = useMemo(() => groups.flatMap((g) => g.fns), [groups]);

  function pick(f: BuiltinFn | undefined) {
    if (!f) return;
    onPick(f.sql);
    setOpen(false);
    setQuery("");
    setActive(0);
  }

  function onSearchChange(value: string) {
    setQuery(value);
    setActive(0); // Reset the highlight to the first match on every keystroke.
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (flat.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((i) => (i + 1) % flat.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => (i - 1 + flat.length) % flat.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      pick(flat[active]);
    }
  }

  return (
    <Dropdown
      open={open}
      trigger={["click"]}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery("");
        setActive(0);
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
              onChange={(e) => onSearchChange(e.target.value)}
              onKeyDown={onKeyDown}
            />
          </div>
          <div role="listbox" style={{ maxHeight: 300, overflowY: "auto", padding: "4px 0" }}>
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
                  {g.fns.map((f) => {
                    const isActive = flat[active]?.sql === f.sql;
                    return (
                      <div
                        key={f.sql}
                        role="option"
                        aria-selected={isActive}
                        onClick={() => pick(f)}
                        onMouseEnter={() => setActive(flat.findIndex((x) => x.sql === f.sql))}
                        title={f.desc}
                        style={{
                          padding: "4px 12px",
                          cursor: "pointer",
                          lineHeight: 1.35,
                          background: isActive ? "var(--bg-hover)" : "transparent",
                        }}
                      >
                        <code style={{ fontSize: 12 }}>{f.sql}</code>
                        <div style={{ color: "var(--text-muted)", fontSize: 11 }}>{f.desc}</div>
                      </div>
                    );
                  })}
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
