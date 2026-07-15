// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { Input, Select, DatePicker, TimePicker, Tooltip, Dropdown, Button, Space } from "antd";
import type { MenuProps } from "antd";
import dayjs from "dayjs";
import customParseFormat from "dayjs/plugin/customParseFormat";
import { WarningOutlined, SnippetsOutlined, FormatPainterOutlined } from "@ant-design/icons";
import type { ParsedColumnType } from "./insertCellTypes";
import { validateValue, snippetsFor, formatJson } from "./insertCellTypes";

// Parse the cell's stored string back into a Dayjs using the exact display
// format below. Without this plugin dayjs ignores the format argument and falls
// back to the native Date parser, which reads TIME_FMT ("HH:mm:ss") as Invalid
// Date and rewrites an offset-bearing timestamp to the machine's local offset on
// every re-render (and behaves differently across the native webviews Wails
// embeds per platform). The plugin makes format-based parsing deterministic.
dayjs.extend(customParseFormat);

// Display formats for the date/time pickers. These double as the literal text
// stored in the cell value and handed to the Go builder as a single-quoted
// string that Snowflake implicitly casts to the column's date/time type.
const DATE_FMT = "YYYY-MM-DD";
const TIME_FMT = "HH:mm:ss";
const TS_FMT = "YYYY-MM-DD HH:mm:ss";

interface Props {
  parsed: ParsedColumnType;
  value: string;
  onChange: (value: string) => void;
  // trailing is placed on the same aligned row as the input widget — the caller
  // passes the per-cell value-mode / function dropdown so the fx button lines up
  // with the value field rather than with the helper toolbar above it.
  trailing?: React.ReactNode;
}

// suffix renders a small warning icon with the validation message when the
// current value fails its family's pre-validation. Validation is UX-only — the
// user can still submit; Snowflake makes the final call.
function statusFor(value: string, parsed: ParsedColumnType): { status?: "warning"; suffix?: React.ReactNode } {
  const err = validateValue(value, parsed);
  if (!err) return {};
  return {
    status: "warning",
    suffix: (
      <Tooltip title={err}>
        <WarningOutlined style={{ color: "var(--ant-color-warning, #faad14)" }} />
      </Tooltip>
    ),
  };
}

/**
 * SnippetToolbar is the compact helper row shown above the JSON / geospatial
 * textareas: a template picker that replaces the cell value with a scaffold
 * (`{}`, `[1, 2, 3]`, a WKT `POINT(…)`, …) and — for JSON columns — a Format
 * action that pretty-prints valid JSON in place. Both write plain strings back;
 * the Go builder renders them the same as hand-typed input.
 */
function SnippetToolbar({
  parsed,
  value,
  onChange,
  showFormat,
}: Pick<Props, "parsed" | "value" | "onChange"> & { showFormat: boolean }) {
  const snippets = snippetsFor(parsed);
  if (snippets.length === 0 && !showFormat) return null;

  const items: MenuProps["items"] = snippets.map((s, i) => ({ key: String(i), label: s.label }));
  const formatted = showFormat ? formatJson(value) : null;

  return (
    <Space size={4} style={{ marginTop: 4 }}>
      {snippets.length > 0 && (
        <Dropdown
          trigger={["click"]}
          menu={{ items, onClick: ({ key }) => onChange(snippets[Number(key)].value) }}
        >
          <Button size="small" type="text" icon={<SnippetsOutlined />} style={{ fontSize: 11, height: 20, padding: "0 4px" }}>
            Template
          </Button>
        </Dropdown>
      )}
      {showFormat && (
        <Tooltip title={formatted == null ? "Enter valid JSON to format" : "Pretty-print JSON"}>
          <Button
            size="small"
            type="text"
            icon={<FormatPainterOutlined />}
            disabled={formatted == null || formatted === value}
            onClick={() => formatted != null && onChange(formatted)}
            style={{ fontSize: 11, height: 20, padding: "0 4px" }}
          >
            Format
          </Button>
        </Tooltip>
      )}
    </Space>
  );
}

// renderControl builds just the value widget for a column's type family, with no
// surrounding layout — InsertCellInput composes the helper toolbar and the
// trailing dropdown around it.
function renderControl({ parsed, value, onChange }: Props): React.ReactNode {
  switch (parsed.family) {
    case "boolean":
      return (
        <Select
          size="small"
          style={{ width: "100%" }}
          value={value === "" ? undefined : value}
          placeholder="TRUE / FALSE"
          allowClear
          onChange={(v) => onChange(v ?? "")}
          options={[
            { value: "TRUE", label: "TRUE" },
            { value: "FALSE", label: "FALSE" },
          ]}
        />
      );

    case "date":
      return (
        <DatePicker
          size="small"
          style={{ width: "100%" }}
          format={DATE_FMT}
          value={value ? dayjs(value, DATE_FMT) : null}
          onChange={(d) => onChange(d ? d.format(DATE_FMT) : "")}
        />
      );

    case "time":
      return (
        <TimePicker
          size="small"
          style={{ width: "100%" }}
          format={TIME_FMT}
          value={value ? dayjs(value, TIME_FMT) : null}
          onChange={(d) => onChange(d ? d.format(TIME_FMT) : "")}
        />
      );

    case "timestamp":
      return (
        <DatePicker
          size="small"
          showTime
          style={{ width: "100%" }}
          format={TS_FMT}
          value={value ? dayjs(value, TS_FMT) : null}
          onChange={(d) => onChange(d ? d.format(TS_FMT) : "")}
        />
      );

    case "timestamptz":
      // Zone-aware types (TIMESTAMP_TZ / _LTZ) store local wall-clock time with
      // no explicit offset; Snowflake applies the session timezone on insert.
      // Emitting an offset here would be self-defeating — the picker re-parses
      // its own output, and dayjs normalises the offset to the machine's local
      // one on the next render, so the string would drift from what the user
      // entered. The mode dropdown's Expression escape hatch covers cases where
      // an explicit offset is required.
      return (
        <DatePicker
          size="small"
          showTime
          style={{ width: "100%" }}
          format={TS_FMT}
          value={value ? dayjs(value, TS_FMT) : null}
          onChange={(d) => onChange(d ? d.format(TS_FMT) : "")}
        />
      );

    case "json": {
      const err = validateValue(value, parsed);
      return (
        <Input.TextArea
          size="small"
          autoSize={{ minRows: 1, maxRows: 6 }}
          status={err ? "warning" : undefined}
          value={value}
          placeholder={parsed.base === "ARRAY" ? "[1, 2, 3]" : '{"key": "value"}'}
          onChange={(e) => onChange(e.target.value)}
          // Input.TextArea has no suffix slot, so — unlike the numeric/vector/
          // uuid/binary widgets which show a WarningOutlined via statusFor — the
          // JSON hint is surfaced through the warning border + title tooltip.
          title={err ?? undefined}
        />
      );
    }

    case "numeric": {
      const { status, suffix } = statusFor(value, parsed);
      const hint =
        parsed.precision != null
          ? parsed.scale
            ? `NUMBER(${parsed.precision},${parsed.scale})`
            : `NUMBER(${parsed.precision})`
          : "number";
      return (
        <Input
          size="small"
          inputMode="decimal"
          status={status}
          value={value}
          placeholder={hint}
          suffix={suffix}
          onChange={(e) => onChange(e.target.value)}
        />
      );
    }

    case "vector": {
      const { status, suffix } = statusFor(value, parsed);
      const dim = parsed.dimension != null ? `${parsed.dimension} × ${parsed.elementType ?? "num"}` : "[n, n, …]";
      return (
        <Input
          size="small"
          status={status}
          value={value}
          placeholder={dim}
          suffix={suffix}
          onChange={(e) => onChange(e.target.value)}
        />
      );
    }

    case "uuid": {
      const { status, suffix } = statusFor(value, parsed);
      return (
        <Input
          size="small"
          status={status}
          value={value}
          placeholder="8-4-4-4-12 hex"
          suffix={suffix}
          onChange={(e) => onChange(e.target.value)}
        />
      );
    }

    case "binary": {
      const { status, suffix } = statusFor(value, parsed);
      return (
        <Input
          size="small"
          status={status}
          value={value}
          placeholder="hex (e.g. DEADBEEF)"
          suffix={suffix}
          onChange={(e) => onChange(e.target.value)}
        />
      );
    }

    case "geo":
      return (
        <Input.TextArea
          size="small"
          autoSize={{ minRows: 1, maxRows: 4 }}
          value={value}
          placeholder="WKT or GeoJSON, e.g. POINT(-122.4 37.8)"
          onChange={(e) => onChange(e.target.value)}
        />
      );

    default:
      // text / other — plain input, with a max-length hint where declared.
      return (
        <Input
          size="small"
          value={value}
          maxLength={parsed.family === "text" ? parsed.length : undefined}
          showCount={parsed.family === "text" && parsed.length != null}
          placeholder="literal value"
          onChange={(e) => onChange(e.target.value)}
        />
      );
  }
}

/**
 * InsertCellInput renders the type-appropriate widget for a single Insert Row
 * cell in "value" mode, together with the caller's trailing value-mode dropdown.
 * The widget is chosen from the column's parsed data type (numeric / boolean /
 * date-time / JSON / vector / …) and always writes a plain string back through
 * onChange — the Go builder (internal/table/insert.go) does the injection-safe
 * SQL rendering, so this component only picks the control and surfaces
 * non-blocking validation hints.
 *
 * For the JSON / geospatial families a helper toolbar (templates + JSON Format)
 * hangs on a row of its own *below* the input. Keeping the input as the first
 * (top) element of every cell lets the input and its trailing fx dropdown line
 * up across all columns — the grid cells are top-aligned, so a toolbar above the
 * input would push these cells' fields a row lower than the plain ones.
 */
export default function InsertCellInput(props: Props) {
  const { parsed, value, onChange, trailing } = props;
  const showFormat = parsed.family === "json";
  const hasToolbar = parsed.family === "json" || parsed.family === "geo";

  const row = (
    <Space.Compact style={{ width: "100%" }}>
      {renderControl(props)}
      {trailing}
    </Space.Compact>
  );

  if (!hasToolbar) return row;

  return (
    <div style={{ width: "100%" }}>
      {row}
      <SnippetToolbar parsed={parsed} value={value} onChange={onChange} showFormat={showFormat} />
    </div>
  );
}
