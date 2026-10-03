// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { AutoComplete, Button, Checkbox, Form, Input, InputNumber, Select, Typography } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import { getIn, type Doc, type Path } from "./specDoc";

/**
 * Path-bound field primitives for the SPCS specification forms (issue #958).
 * Each reads its value from the parsed spec document in SpecCtx and writes
 * back through `set(path, value)`; clearing a field deletes the key.
 */

export interface SpecCtxValue {
  doc: Doc;
  set: (p: Path, v: unknown) => void;
  /** Several writes at once (each `set` re-serialises from the same doc). */
  edit: (fn: (doc: Doc) => Doc) => void;
}
export const SpecCtx = createContext<SpecCtxValue>(null!);
export const useSpec = () => useContext(SpecCtx);

const ITEM: React.CSSProperties = { marginBottom: 6 };
const opts = (xs: string[]) => xs.map((x) => ({ value: x, label: x }));

/** "0.5" → 0.5 but "1.0" / "500m" stay strings, so typing never rewrites the input. */
export const numberish = (s: string): unknown => (s.trim() !== "" && String(Number(s)) === s.trim() ? Number(s) : s);

interface Base { p: Path; label: string; help?: string; placeholder?: string; span?: number }
const spanStyle = (span?: number): React.CSSProperties => ({ ...ITEM, gridColumn: span ? `span ${span}` : undefined });

export function Grid({ children, cols = 3 }: { children: ReactNode; cols?: number }) {
  return <div style={{ display: "grid", gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`, gap: "0 12px" }}>{children}</div>;
}

export function Str({ p, label, help, placeholder, span, numeric, options }: Base & { numeric?: boolean; options?: string[] }) {
  const { doc, set } = useSpec();
  const v = getIn(doc, p);
  const value = v == null ? "" : String(v);
  const onChange = (s: string) => set(p, numeric ? numberish(s) : s);
  return (
    <Form.Item label={label} help={help} style={spanStyle(span)}>
      {options ? (
        <AutoComplete value={value} onChange={onChange} options={opts(options)} placeholder={placeholder}
          filterOption={(input, o) => String(o?.value).toLowerCase().includes(input.toLowerCase())} />
      ) : (
        <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} />
      )}
    </Form.Item>
  );
}

export function Num({ p, label, help, placeholder, span, min, max }: Base & { min?: number; max?: number }) {
  const { doc, set } = useSpec();
  const v = getIn(doc, p);
  return (
    <Form.Item label={label} help={help} style={spanStyle(span)}>
      <InputNumber value={typeof v === "number" ? v : v == null ? null : Number(v)} min={min} max={max}
        placeholder={placeholder} style={{ width: "100%" }} onChange={(n) => set(p, n ?? undefined)} />
    </Form.Item>
  );
}

/** Single select, or a string list with `mode` "multiple" (fixed options) / "tags" (free entry). */
export function Sel({ p, label, help, placeholder, span, options = [], mode }: Base & { options?: string[]; mode?: "multiple" | "tags" }) {
  const { doc, set } = useSpec();
  const v = getIn(doc, p);
  return (
    <Form.Item label={label} help={help} style={spanStyle(span)}>
      <Select allowClear mode={mode} value={v ?? undefined} options={opts(options)} placeholder={placeholder}
        onChange={(x) => set(p, x ?? undefined)} />
    </Form.Item>
  );
}

export const Tags = (props: Base & { options?: string[] }) => <Sel {...props} mode="tags" />;

/** Checkbox; checking back to the documented default removes the key. */
export function Bool({ p, label, def = false, span }: { p: Path; label: string; def?: boolean; span?: number }) {
  const { doc, set } = useSpec();
  const v = getIn(doc, p);
  return (
    <Form.Item label=" " style={spanStyle(span)}>
      <Checkbox checked={v ?? def} onChange={(e) => set(p, e.target.checked === def ? undefined : e.target.checked)}>
        {label}
      </Checkbox>
    </Form.Item>
  );
}

/**
 * Key → value rows over a mapping. Rows are local state (a half-typed row has
 * no key yet) and re-sync when the mapping changes underneath. `valueOptions`
 * turns the value into a Select; `wrap`/`unwrap` map row strings to stored values.
 */
export function KV({ p, label, keyPh = "NAME", valPh = "value", valueOptions, wrap = (s: string) => s, unwrap = (x: unknown) => String(x ?? "") }: {
  p: Path; label: string; keyPh?: string; valPh?: string; valueOptions?: string[];
  wrap?: (s: string) => unknown; unwrap?: (x: unknown) => string;
}) {
  const { doc, set } = useSpec();
  const map: Record<string, unknown> = getIn(doc, p) ?? {};
  const entries = (): [string, string][] => Object.entries(map).map(([k, v]) => [k, unwrap(v)]);
  const [rows, setRows] = useState(entries);
  // First row wins on a repeated key, so typing a name that already exists
  // can't overwrite the other row's value; the repeat is flagged below.
  const toMap = (r: [string, string][]) => {
    const m: Record<string, unknown> = {};
    for (const [k, v] of r) if (k.trim() && !(k.trim() in m)) m[k.trim()] = wrap(v);
    return m;
  };
  const dup = (i: number) => rows[i][0].trim() !== "" && rows.findIndex(([k]) => k.trim() === rows[i][0].trim()) !== i;
  const key = JSON.stringify(map);
  useEffect(() => {
    if (JSON.stringify(toMap(rows)) !== key) setRows(entries());
  }, [key]); // eslint-disable-line react-hooks/exhaustive-deps
  const write = (r: [string, string][]) => { setRows(r); set(p, toMap(r)); };
  const cell = (i: number, j: 0 | 1, v: string) => write(rows.map((r, k) => (k === i ? (j ? [r[0], v] : [v, r[1]]) : r)));
  return (
    <Form.Item label={label} style={{ ...ITEM, gridColumn: "1 / -1" }}>
      {rows.map(([k, v], i) => (
        <div key={i} style={{ display: "flex", gap: 6, marginBottom: 4 }}>
          <Input value={k} onChange={(e) => cell(i, 0, e.target.value)} placeholder={keyPh} style={{ flex: 1 }}
            status={dup(i) ? "error" : undefined} title={dup(i) ? "Duplicate name — this row is ignored" : undefined} />
          {valueOptions ? (
            <Select value={v || undefined} onChange={(x) => cell(i, 1, x ?? "")} options={opts(valueOptions)} placeholder={valPh} style={{ flex: 1 }} />
          ) : (
            <Input value={v} onChange={(e) => cell(i, 1, e.target.value)} placeholder={valPh} style={{ flex: 1 }} />
          )}
          <Button type="text" icon={<MinusCircleOutlined />} onClick={() => write(rows.filter((_, k) => k !== i))} />
        </div>
      ))}
      <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={() => setRows([...rows, ["", ""]])}>Add</Button>
    </Form.Item>
  );
}

/** Repeatable cards over an array; `children(base, i)` renders one item's fields. */
export function List({ p, label, noun, item = () => ({}), max, children }: {
  p: Path; label?: string; noun: string; item?: () => unknown; max?: number;
  children: (base: Path, i: number) => ReactNode;
}) {
  const { doc, set } = useSpec();
  const xs: unknown[] = Array.isArray(getIn(doc, p)) ? getIn(doc, p) : [];
  const [gen, setGen] = useState(0);
  return (
    <div style={{ gridColumn: "1 / -1", marginBottom: 8 }}>
      {label && <Typography.Text strong style={{ fontSize: 12, display: "block", marginBottom: 4 }}>{label}</Typography.Text>}
      {/* Keyed by index + removal count: removing a row remounts the rest, so
          row-local state (KV rows, port/range, env/dir) re-derives from the
          document; adding one leaves the others (and their open groups) alone. */}
      {xs.map((_, i) => (
        <div key={`${i}/${gen}`} style={{ display: "flex", gap: 4, border: "1px solid var(--border)", borderRadius: 6, padding: "6px 8px", marginBottom: 6 }}>
          <div style={{ flex: 1, minWidth: 0 }}>{children([...p, i], i)}</div>
          <Button type="text" size="small" icon={<MinusCircleOutlined />} title={`Remove ${noun}`}
            onClick={() => { setGen(gen + 1); set(p, xs.filter((_, j) => j !== i)); }} />
        </div>
      ))}
      <Button type="dashed" size="small" icon={<PlusOutlined />} disabled={max != null && xs.length >= max}
        onClick={() => set(p, [...xs, item()])}>
        Add {noun}
      </Button>
    </div>
  );
}

/**
 * Collapsible sub-section (native <details>); starts open when its subtree has
 * a value. Only the initial state is computed — clearing the last field inside
 * must not snap it shut under the cursor.
 */
export function Group({ title, p, open, children, cols }: { title: string; p?: Path; open?: boolean; children: ReactNode; cols?: number }) {
  const { doc } = useSpec();
  const [initial] = useState(() => open ?? (p ? getIn(doc, p) != null : false));
  return (
    <details open={initial} style={{ gridColumn: "1 / -1", margin: "2px 0 8px" }}>
      <summary style={{ cursor: "pointer", fontSize: 12, fontWeight: 600, color: "var(--text-muted)", marginBottom: 6 }}>{title}</summary>
      <Grid cols={cols}>{children}</Grid>
    </details>
  );
}
