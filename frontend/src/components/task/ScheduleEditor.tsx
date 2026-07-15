// SPDX-License-Identifier: GPL-3.0-or-later

import { useState } from "react";
import { Radio, Select, InputNumber, Input, Space } from "antd";
import { TIMEZONE_OPTIONS } from "../../utils/timezones";

// ── Types ─────────────────────────────────────────────────────────────────────

type ScheduleMode = "none" | "interval" | "cron";
type IntervalUnit = "HOURS" | "MINUTES" | "SECONDS";

// ── Snowflake interval constraints ────────────────────────────────────────────
// SECONDS: 10 – 691,200  (10 s → 8 days)
// MINUTES:  1 – 11,520   (1 min → 8 days)
// HOURS:    1 – 192      (1 hr  → 8 days)

const LIMITS: Record<IntervalUnit, { min: number; max: number; hint: string }> = {
  SECONDS: { min: 10,   max: 691200, hint: "10 – 691,200" },
  MINUTES: { min: 1,    max: 11520,  hint: "1 – 11,520"   },
  HOURS:   { min: 1,    max: 192,    hint: "1 – 192"       },
};

const UNIT_OPTS: { value: IntervalUnit; label: string }[] = [
  { value: "SECONDS", label: "Seconds" },
  { value: "MINUTES", label: "Minutes" },
  { value: "HOURS",   label: "Hours"   },
];

// ── Parse / build ─────────────────────────────────────────────────────────────

interface ParsedSchedule {
  mode: ScheduleMode;
  num:  number | null;
  unit: IntervalUnit;
  expr: string;   // cron expression (5-field Snowflake format)
  tz:   string;   // timezone for cron
}

function parseSchedule(raw: string): ParsedSchedule {
  const t = (raw || "").trim();
  if (!t) return { mode: "none", num: null, unit: "MINUTES", expr: "", tz: "UTC" };

  // USING CRON <expr> <timezone>
  const cronM = t.match(/^USING\s+CRON\s+(.+?)\s+(\S+)\s*$/i);
  if (cronM) return { mode: "cron", num: null, unit: "MINUTES", expr: cronM[1], tz: cronM[2] };

  // <num> HOURS | MINUTES | SECONDS
  const intM = t.match(/^(\d+)\s+(HOURS?|MINUTES?|SECONDS?)\s*$/i);
  if (intM) {
    const u = intM[2].toUpperCase();
    const unit: IntervalUnit = u.startsWith("H") ? "HOURS" : u.startsWith("M") ? "MINUTES" : "SECONDS";
    return { mode: "interval", num: parseInt(intM[1], 10), unit, expr: "", tz: "UTC" };
  }

  // Unrecognised — fall back to cron raw-text mode
  return { mode: "cron", num: null, unit: "MINUTES", expr: t, tz: "" };
}

function buildScheduleStr(p: ParsedSchedule): string {
  if (p.mode === "interval" && p.num !== null) return `${p.num} ${p.unit}`;
  if (p.mode === "cron" && p.expr.trim())
    return `USING CRON ${p.expr.trim()} ${p.tz.trim() || "UTC"}`;
  return "";
}

// ── Component ─────────────────────────────────────────────────────────────────

export interface ScheduleEditorProps {
  /** Current schedule string, e.g. "5 MINUTES" or "USING CRON 0 9 * * * UTC". */
  value:    string;
  /** Called with the new schedule string on every change (empty string = no schedule). */
  onChange: (schedule: string) => void;
}

/**
 * Controlled schedule editor.  Internal UI state is initialised from `value`
 * once on mount.  The parent should use `key` to force a re-mount when it needs
 * to reset the editor (e.g. after saving and reloading properties).
 */
export default function ScheduleEditor({ value, onChange }: ScheduleEditorProps) {
  const [s, setS] = useState<ParsedSchedule>(() => parseSchedule(value));

  const update = (patch: Partial<ParsedSchedule>) => {
    const next = { ...s, ...patch };
    setS(next);
    onChange(buildScheduleStr(next));
  };

  const lim = LIMITS[s.unit];
  const numOutOfRange =
    s.mode === "interval" && s.num !== null && (s.num < lim.min || s.num > lim.max);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>

      {/* Mode selector */}
      <Radio.Group
        size="small"
        value={s.mode}
        onChange={(e) => update({ mode: e.target.value as ScheduleMode })}
      >
        <Radio value="none">None (triggered / dependent)</Radio>
        <Radio value="interval">Interval</Radio>
        <Radio value="cron">Cron</Radio>
      </Radio.Group>

      {/* Interval fields */}
      {s.mode === "interval" && (
        <Space align="center">
          <InputNumber
            size="small"
            value={s.num ?? undefined}
            onChange={(v) => update({ num: v ?? null })}
            min={lim.min}
            max={lim.max}
            placeholder={String(lim.min)}
            status={numOutOfRange ? "error" : undefined}
            style={{ width: 110 }}
          />
          <Select<IntervalUnit>
            size="small"
            value={s.unit}
            options={UNIT_OPTS}
            onChange={(v) => {
              const newLim = LIMITS[v];
              // clamp the number to the new unit's valid range if one is set
              const clamped =
                s.num !== null
                  ? Math.min(Math.max(s.num, newLim.min), newLim.max)
                  : null;
              update({ unit: v, num: clamped });
            }}
            style={{ width: 110 }}
          />
          <span style={{
            fontSize: 11,
            fontStyle: "italic",
            color: numOutOfRange ? "#f85149" : "var(--text-faint)",
          }}>
            {lim.hint}
          </span>
        </Space>
      )}

      {/* Cron fields */}
      {s.mode === "cron" && (
        <Space wrap>
          <Input
            size="small"
            value={s.expr}
            onChange={(e) => update({ expr: e.target.value })}
            placeholder="0 9 * * *"
            style={{ width: 180, fontFamily: "monospace", fontSize: 12 }}
          />
          <Select
            size="small"
            showSearch
            value={s.tz || undefined}
            options={TIMEZONE_OPTIONS}
            onChange={(v) => update({ tz: v ?? "UTC" })}
            filterOption={(input, opt) =>
              String(opt?.label ?? "").toLowerCase().includes(input.toLowerCase())
            }
            placeholder="Select timezone…"
            style={{ width: 260 }}
            listHeight={300}
          />
          <span style={{ fontSize: 11, color: "var(--text-faint)", fontStyle: "italic" }}>
            min hr dom mon dow
          </span>
        </Space>
      )}

    </div>
  );
}
