// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { useState } from "react";
import { Modal, Select, InputNumber, Space, Typography, Button } from "antd";
import { useGridStore, type FormatConfig, type FormatType } from "../../store/gridStore";

const { Text } = Typography;

interface Props {
  columnId: string;
  columnName: string;
  sampleValues: unknown[];
  onClose: () => void;
}

// Detect column type from sample values.
function detectType(values: unknown[]): "number" | "datetime" | "string" {
  let numCount = 0;
  let dateCount = 0;
  let total = 0;

  for (const v of values) {
    if (v == null) continue;
    total++;
    const s = String(v);
    if (!isNaN(Number(v)) && v !== "" && v !== true && v !== false) {
      numCount++;
    } else if (!isNaN(Date.parse(s)) && s.length > 6) {
      dateCount++;
    }
  }

  if (total === 0) return "string";
  if (numCount / total > 0.7) return "number";
  if (dateCount / total > 0.7) return "datetime";
  return "string";
}

function formatPreview(value: unknown, config: FormatConfig): string {
  if (value == null) return "NULL";
  const s = String(value);

  switch (config.type) {
    case "number": {
      const n = Number(value);
      if (isNaN(n)) return s;
      return new Intl.NumberFormat(config.locale ?? undefined, {
        minimumFractionDigits: config.decimals ?? 0,
        maximumFractionDigits: config.decimals ?? 6,
      }).format(n);
    }
    case "currency": {
      const n = Number(value);
      if (isNaN(n)) return s;
      return new Intl.NumberFormat(config.locale ?? undefined, {
        style: "currency",
        currency: config.currency ?? "USD",
        minimumFractionDigits: config.decimals ?? 2,
        maximumFractionDigits: config.decimals ?? 2,
      }).format(n);
    }
    case "percentage": {
      const n = Number(value);
      if (isNaN(n)) return s;
      return new Intl.NumberFormat(config.locale ?? undefined, {
        style: "percent",
        minimumFractionDigits: config.decimals ?? 1,
        maximumFractionDigits: config.decimals ?? 1,
      }).format(n);
    }
    case "datetime": {
      const d = new Date(s);
      if (isNaN(d.getTime())) return s;
      return new Intl.DateTimeFormat(config.locale ?? undefined, {
        dateStyle: "medium",
        timeStyle: "medium",
        timeZone: config.timezone === "utc" ? "UTC" : undefined,
      }).format(d);
    }
    default:
      return s;
  }
}

/** Apply a FormatConfig to a cell value. Exported for use in ResultGrid cell renderer. */
export function applyFormat(value: unknown, config: FormatConfig): string {
  return formatPreview(value, config);
}

export default function DataTypeFormatModal({ columnId, columnName, sampleValues, onClose }: Props) {
  const existing = useGridStore((s) => s.columnFormats[columnId]);
  const setColumnFormat = useGridStore((s) => s.setColumnFormat);
  const clearColumnFormat = useGridStore((s) => s.clearColumnFormat);

  const detectedType = detectType(sampleValues);

  const [formatType, setFormatType] = useState<FormatType>(
    existing?.type ?? (detectedType === "datetime" ? "datetime" : "number"),
  );
  const [decimals, setDecimals] = useState<number>(existing?.decimals ?? 2);
  const [currency, setCurrency] = useState<string>(existing?.currency ?? "USD");
  const [timezone, setTimezone] = useState<"utc" | "local">(existing?.timezone ?? "local");

  const currentConfig: FormatConfig = {
    type: formatType,
    decimals,
    currency: formatType === "currency" ? currency : undefined,
    timezone: formatType === "datetime" ? timezone : undefined,
  };

  const previewValues = sampleValues.filter((v) => v != null).slice(0, 5);

  const handleApply = () => {
    setColumnFormat(columnId, currentConfig);
    onClose();
  };

  const handleClear = () => {
    clearColumnFormat(columnId);
    onClose();
  };

  return (
    <Modal
      title={`Format Column: ${columnName}`}
      open
      onCancel={onClose}
      footer={[
        <Button key="clear" size="small" onClick={handleClear}>
          Clear Format
        </Button>,
        <Button key="cancel" size="small" onClick={onClose}>
          Cancel
        </Button>,
        <Button key="apply" size="small" type="primary" onClick={handleApply}>
          Apply
        </Button>,
      ]}
      width={400}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <div>
          <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
            Detected type: {detectedType}
          </Text>
        </div>

        <Space direction="vertical" size={8} style={{ width: "100%" }}>
          <div>
            <Text style={{ fontSize: 12 }}>Format as:</Text>
            <Select
              size="small"
              value={formatType}
              onChange={(v) => setFormatType(v)}
              style={{ width: "100%", marginTop: 4, fontSize: 11 }}
              options={[
                { label: "Number", value: "number" },
                { label: "Currency", value: "currency" },
                { label: "Percentage (multiplies by 100)", value: "percentage" },
                { label: "Date/Time", value: "datetime" },
              ]}
            />
          </div>

          {(formatType === "number" || formatType === "currency" || formatType === "percentage") && (
            <div>
              <Text style={{ fontSize: 12 }}>Decimal places:</Text>
              <InputNumber
                size="small"
                min={0}
                max={10}
                value={decimals}
                onChange={(v) => setDecimals(v ?? 2)}
                style={{ width: "100%", marginTop: 4, fontSize: 11 }}
              />
            </div>
          )}

          {formatType === "currency" && (
            <div>
              <Text style={{ fontSize: 12 }}>Currency:</Text>
              <Select
                size="small"
                value={currency}
                onChange={(v) => setCurrency(v)}
                style={{ width: "100%", marginTop: 4, fontSize: 11 }}
                showSearch
                options={[
                  { label: "USD ($)", value: "USD" },
                  { label: "EUR (\u20ac)", value: "EUR" },
                  { label: "GBP (\u00a3)", value: "GBP" },
                  { label: "JPY (\u00a5)", value: "JPY" },
                  { label: "CHF", value: "CHF" },
                  { label: "CAD (C$)", value: "CAD" },
                  { label: "AUD (A$)", value: "AUD" },
                  { label: "SEK (kr)", value: "SEK" },
                  { label: "NOK (kr)", value: "NOK" },
                  { label: "DKK (kr)", value: "DKK" },
                ]}
              />
            </div>
          )}

          {formatType === "datetime" && (
            <div>
              <Text style={{ fontSize: 12 }}>Timezone:</Text>
              <Select
                size="small"
                value={timezone}
                onChange={(v) => setTimezone(v)}
                style={{ width: "100%", marginTop: 4, fontSize: 11 }}
                options={[
                  { label: "Local Time", value: "local" },
                  { label: "UTC", value: "utc" },
                ]}
              />
            </div>
          )}
        </Space>

        {previewValues.length > 0 && (
          <div style={{ marginTop: 8 }}>
            <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>Preview:</Text>
            <div
              style={{
                marginTop: 4,
                padding: 8,
                background: "var(--bg)",
                borderRadius: 4,
                border: "1px solid var(--border)",
                fontSize: 12,
                fontFamily: "monospace",
              }}
            >
              {previewValues.map((v, i) => (
                <div key={i} style={{ display: "flex", justifyContent: "space-between", gap: 12 }}>
                  <span style={{ color: "var(--text-muted)" }}>{String(v)}</span>
                  <span style={{ color: "var(--text)" }}>{formatPreview(v, currentConfig)}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </Modal>
  );
}
