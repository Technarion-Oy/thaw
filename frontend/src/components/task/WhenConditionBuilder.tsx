// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Button, Select, Input, Checkbox, Spin } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { ListObjects } from "../../../wailsjs/go/app/App";

// ── Types ─────────────────────────────────────────────────────────────────────

type JoinOp  = "AND" | "OR";
type CastType = "" | "BOOLEAN" | "FLOAT" | "STRING";

interface StreamCond {
  id: string; join: JoinOp; type: "stream";
  stream: string; negated: boolean;
}
interface PredCond {
  id: string; join: JoinOp; type: "pred";
  task: string; cast: CastType; negated: boolean; op: string; val: string;
}
interface CustomCond {
  id: string; join: JoinOp; type: "custom"; expr: string;
}
type Cond = StreamCond | PredCond | CustomCond;

// ── Shared select options ─────────────────────────────────────────────────────

const JOIN_OPTS = [
  { label: "AND", value: "AND" },
  { label: "OR",  value: "OR"  },
];

const CAST_OPTS = [
  { label: "No cast", value: ""        },
  { label: "BOOLEAN", value: "BOOLEAN" },
  { label: "FLOAT",   value: "FLOAT"   },
  { label: "STRING",  value: "STRING"  },
];

const OP_OPTS = [
  { label: "—",  value: ""   },
  { label: "=",  value: "="  },
  { label: "≠",  value: "!=" },
  { label: "<",  value: "<"  },
  { label: ">",  value: ">"  },
  { label: "≤",  value: "<=" },
  { label: "≥",  value: ">=" },
];

// ── Expression builder ────────────────────────────────────────────────────────

function buildExpr(conds: Cond[]): string {
  const valid = conds.filter(c =>
    c.type === "stream" ? !!c.stream :
    c.type === "pred"   ? !!c.task   :
    !!c.expr.trim(),
  );
  return valid.map((c, i) => {
    const pre = i === 0 ? "" : ` ${c.join} `;
    if (c.type === "stream") {
      const fn = `SYSTEM$STREAM_HAS_DATA('${c.stream}')`;
      return pre + (c.negated ? `NOT ${fn}` : fn);
    }
    if (c.type === "pred") {
      let fn = `SYSTEM$GET_PREDECESSOR_RETURN_VALUE('${c.task}')`;
      if (c.cast) fn += `::${c.cast}`;
      const cmp = c.op && c.val.trim() ? ` ${c.op} ${c.val.trim()}` : "";
      return pre + (c.negated ? `NOT (${fn}${cmp})` : `${fn}${cmp}`);
    }
    return pre + c.expr.trim();
  }).join("");
}

// ── Expression parser ─────────────────────────────────────────────────────────

let _seq = 0;
const uid = () => `c${++_seq}`;

function parseExpr(raw: string): Cond[] {
  const t = raw.trim();
  if (!t) return [];
  const parts: { join: JoinOp; expr: string }[] = [];
  let join: JoinOp = "AND";
  for (const tok of t.split(/(\bAND\b|\bOR\b)/i)) {
    const upper = tok.trim().toUpperCase();
    if (upper === "AND") { join = "AND"; continue; }
    if (upper === "OR")  { join = "OR";  continue; }
    const e = tok.trim();
    if (e) parts.push({ join: parts.length === 0 ? "AND" : join, expr: e });
  }
  return parts.map(({ join, expr }): Cond => {
    // NOT SYSTEM$STREAM_HAS_DATA('x')
    const snm = expr.match(/^NOT\s+SYSTEM\$STREAM_HAS_DATA\('([^']+)'\)$/i);
    if (snm) return { id: uid(), join, type: "stream", stream: snm[1], negated: true };
    // SYSTEM$STREAM_HAS_DATA('x')
    const sm = expr.match(/^SYSTEM\$STREAM_HAS_DATA\('([^']+)'\)$/i);
    if (sm)  return { id: uid(), join, type: "stream", stream: sm[1], negated: false };
    // NOT (SYSTEM$GET_PREDECESSOR_RETURN_VALUE('x')::CAST op val) or NOT SYSTEM$GET...
    const pneg = expr.match(/^NOT\s+\(?SYSTEM\$GET_PREDECESSOR_RETURN_VALUE\('([^']+)'\)(?:::(\w+))?(?:\s*([!=<>]+)\s*(.+?))?\)?\s*$/i);
    if (pneg) return { id: uid(), join, type: "pred", negated: true, task: pneg[1], cast: (pneg[2]?.toUpperCase() || "") as CastType, op: pneg[3] || "", val: pneg[4]?.trim() || "" };
    // SYSTEM$GET_PREDECESSOR_RETURN_VALUE('x')::CAST op val
    const pm = expr.match(/^SYSTEM\$GET_PREDECESSOR_RETURN_VALUE\('([^']+)'\)(?:::(\w+))?\s*([!=<>]+)\s*(.+)$/i);
    if (pm) return { id: uid(), join, type: "pred", negated: false, task: pm[1], cast: (pm[2]?.toUpperCase() || "") as CastType, op: pm[3], val: pm[4].trim() };
    // SYSTEM$GET_PREDECESSOR_RETURN_VALUE('x')::CAST  (no comparison)
    const ps = expr.match(/^SYSTEM\$GET_PREDECESSOR_RETURN_VALUE\('([^']+)'\)(?:::(\w+))?$/i);
    if (ps) return { id: uid(), join, type: "pred", negated: false, task: ps[1], cast: (ps[2]?.toUpperCase() || "") as CastType, op: "", val: "" };
    return { id: uid(), join, type: "custom", expr };
  });
}

// ── ConditionRow ──────────────────────────────────────────────────────────────

interface RowProps {
  cond:       Cond;
  isFirst:    boolean;
  streamOpts: { label: string; value: string }[];
  taskOpts:   { label: string; value: string }[];
  loading:    boolean;
  onChange:   (patch: Partial<Cond>) => void;
  onRemove:   () => void;
}

function ConditionRow({ cond, isFirst, streamOpts, taskOpts, loading, onChange, onRemove }: RowProps) {
  return (
    <div style={{
      display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap",
      padding: "6px 10px", background: "var(--bg-overlay)",
      border: "1px solid var(--border)", borderRadius: 6,
    }}>
      {/* Join / IF label */}
      {isFirst ? (
        <span style={{ fontSize: 11, fontWeight: 600, color: "var(--text-muted)", width: 40, flexShrink: 0 }}>IF</span>
      ) : (
        <Select size="small" value={cond.join} options={JOIN_OPTS}
          onChange={(v) => onChange({ join: v })}
          style={{ width: 62, flexShrink: 0 }} />
      )}

      {/* NOT checkbox (stream + pred) */}
      {(cond.type === "stream" || cond.type === "pred") && (
        <Checkbox checked={cond.negated} onChange={(e) => onChange({ negated: e.target.checked } as any)}
          style={{ fontSize: 12, flexShrink: 0 }}>NOT</Checkbox>
      )}

      {/* Stream condition */}
      {cond.type === "stream" && (
        <>
          <span style={{ fontSize: 11, color: "var(--text-muted)", whiteSpace: "nowrap", flexShrink: 0 }}>
            STREAM_HAS_DATA
          </span>
          <Select size="small" showSearch allowClear
            placeholder={loading ? "Loading…" : "stream name"}
            value={cond.stream || undefined}
            options={streamOpts}
            onChange={(v) => onChange({ stream: v ?? "" } as any)}
            notFoundContent={loading ? <Spin size="small" /> : "No streams in schema"}
            style={{ flex: 1, minWidth: 140 }} />
        </>
      )}

      {/* Predecessor condition */}
      {cond.type === "pred" && (
        <>
          <span style={{ fontSize: 11, color: "var(--text-muted)", whiteSpace: "nowrap", flexShrink: 0 }}>
            GET_PREDECESSOR_VALUE
          </span>
          <Select size="small" showSearch allowClear
            placeholder={loading ? "Loading…" : "task name"}
            value={cond.task || undefined}
            options={taskOpts}
            onChange={(v) => onChange({ task: v ?? "" } as any)}
            notFoundContent={loading ? <Spin size="small" /> : "No tasks in schema"}
            style={{ flex: 1, minWidth: 140 }} />
          <Select size="small" value={cond.cast} options={CAST_OPTS}
            onChange={(v) => onChange({ cast: v } as any)}
            style={{ width: 90, flexShrink: 0 }} />
          <Select size="small" value={cond.op} options={OP_OPTS}
            onChange={(v) => onChange({ op: v } as any)}
            style={{ width: 58, flexShrink: 0 }} />
          {cond.op && (
            <Input size="small" value={cond.val}
              onChange={(e) => onChange({ val: e.target.value } as any)}
              placeholder="value"
              style={{ width: 90, fontFamily: "monospace", fontSize: 12, flexShrink: 0 }} />
          )}
        </>
      )}

      {/* Custom expression */}
      {cond.type === "custom" && (
        <Input size="small" value={cond.expr}
          onChange={(e) => onChange({ expr: e.target.value } as any)}
          placeholder="SQL boolean expression…"
          style={{ flex: 1, fontFamily: "monospace", fontSize: 12 }} />
      )}

      <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onRemove}
        style={{ flexShrink: 0, marginLeft: "auto" }} />
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export interface WhenConditionBuilderProps {
  db:       string;
  schema:   string;
  value:    string;
  onChange: (expr: string) => void;
}

export default function WhenConditionBuilder({ db, schema, value, onChange }: WhenConditionBuilderProps) {
  const [mode,  setMode]  = useState<"visual" | "raw">("visual");
  const [conds, setConds] = useState<Cond[]>(() => parseExpr(value));
  const [raw,   setRaw]   = useState(value);
  const [streams,  setStreams]  = useState<string[]>([]);
  const [tasks,    setTasks]    = useState<string[]>([]);
  const [loading,  setLoading]  = useState(true);

  useEffect(() => {
    ListObjects(db, schema)
      .then((objs) => {
        setStreams(objs.filter(o => o.kind?.toUpperCase() === "STREAM").map(o => o.name));
        setTasks(objs.filter(o => o.kind?.toUpperCase() === "TASK").map(o => o.name));
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [db, schema]);

  const switchMode = (m: "visual" | "raw") => {
    if (m === "raw")    setRaw(buildExpr(conds));
    else                setConds(parseExpr(raw));
    setMode(m);
  };

  const notifyVisual = (next: Cond[]) => {
    setConds(next);
    onChange(buildExpr(next));
  };

  const notifyRaw = (v: string) => {
    setRaw(v);
    onChange(v.trim());
  };

  const addCond = (type: "stream" | "pred" | "custom") => {
    const c: Cond =
      type === "stream" ? { id: uid(), join: "AND", type: "stream", stream: "",  negated: false }
    : type === "pred"   ? { id: uid(), join: "AND", type: "pred",   task: "",    negated: false, cast: "", op: "", val: "" }
                        : { id: uid(), join: "AND", type: "custom", expr: "" };
    notifyVisual([...conds, c]);
  };

  const streamOpts = streams.map(s => ({ label: s, value: s }));
  const taskOpts   = tasks.map(t => ({ label: t, value: t }));

  const preview = mode === "raw" ? raw.trim() : buildExpr(conds);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {/* Mode toggle */}
      <div style={{ display: "flex", justifyContent: "flex-end" }}>
        <div style={{ display: "flex" }}>
          <Button size="small"
            type={mode === "visual" ? "primary" : "default"}
            style={{ borderRadius: "4px 0 0 4px" }}
            onClick={() => mode === "raw" && switchMode("visual")}>
            Visual
          </Button>
          <Button size="small"
            type={mode === "raw" ? "primary" : "default"}
            style={{ borderRadius: "0 4px 4px 0", marginLeft: -1 }}
            onClick={() => mode === "visual" && switchMode("raw")}>
            Raw SQL
          </Button>
        </div>
      </div>

      {/* Visual builder */}
      {mode === "visual" && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {conds.length === 0 ? (
            <div style={{ fontSize: 12, color: "var(--text-faint)", fontStyle: "italic" }}>
              No conditions — task will always run when triggered.
            </div>
          ) : (
            conds.map((c, idx) => (
              <ConditionRow key={c.id} cond={c} isFirst={idx === 0}
                streamOpts={streamOpts} taskOpts={taskOpts} loading={loading}
                onChange={(patch) => notifyVisual(conds.map(x => x.id === c.id ? { ...x, ...patch } as Cond : x))}
                onRemove={() => notifyVisual(conds.filter(x => x.id !== c.id))} />
            ))
          )}

          {/* Add buttons */}
          <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginTop: 2 }}>
            <Button size="small" icon={<PlusOutlined />} onClick={() => addCond("stream")}>
              STREAM_HAS_DATA
            </Button>
            <Button size="small" icon={<PlusOutlined />} onClick={() => addCond("pred")}>
              GET_PREDECESSOR_VALUE
            </Button>
            <Button size="small" icon={<PlusOutlined />} onClick={() => addCond("custom")}>
              Custom
            </Button>
          </div>
        </div>
      )}

      {/* Raw SQL */}
      {mode === "raw" && (
        <Input.TextArea
          value={raw}
          onChange={(e) => notifyRaw(e.target.value)}
          autoSize={{ minRows: 3, maxRows: 8 }}
          style={{ fontFamily: "monospace", fontSize: 12 }}
          placeholder="e.g. SYSTEM$STREAM_HAS_DATA('MY_STREAM')"
        />
      )}

      {/* Preview */}
      {preview && (
        <div style={{
          padding: "4px 8px", background: "var(--bg)",
          border: "1px solid var(--border)", borderRadius: 4,
          fontFamily: "monospace", fontSize: 11,
          color: "var(--text-muted)", wordBreak: "break-all",
        }}>
          <span style={{ fontSize: 10, color: "var(--text-faint)", fontFamily: "sans-serif" }}>WHEN </span>
          {preview}
        </div>
      )}
    </div>
  );
}
