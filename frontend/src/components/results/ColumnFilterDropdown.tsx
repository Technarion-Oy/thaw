// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { useState, useMemo, useCallback, useEffect, useLayoutEffect, useRef } from "react";
import { Input, Checkbox, Button, Divider, Select, Space } from "antd";
import { FilterOutlined } from "@ant-design/icons";
import type { FilterFn } from "@tanstack/react-table";

type ConditionOp = "contains" | "startsWith" | "endsWith" | "equals" | "gt" | "lt" | "gte" | "lte";

interface Props {
  columnValues: unknown[];
  truncated?: boolean;
  currentFilter: ColumnFilterValue | undefined;
  onApply: (filter: ColumnFilterValue | undefined) => void;
  onClose: () => void;
  position: { x: number; y: number };
}

export interface ColumnFilterValue {
  checkedValues?: Set<string>;
  condition?: { op: ConditionOp; value: string };
}

export const columnFilterFn: FilterFn<unknown[]> = (
  row,
  columnId,
  filterValue: ColumnFilterValue,
) => {
  const cellValue = row.getValue(columnId);
  const cellStr = cellValue == null ? "" : String(cellValue);

  // Checked values filter
  if (filterValue.checkedValues && filterValue.checkedValues.size > 0) {
    if (!filterValue.checkedValues.has(cellStr)) return false;
  }

  // Condition filter
  if (filterValue.condition && filterValue.condition.value) {
    const { op, value } = filterValue.condition;
    const lower = cellStr.toLowerCase();
    const filterLower = value.toLowerCase();

    switch (op) {
      case "contains":
        if (!lower.includes(filterLower)) return false;
        break;
      case "startsWith":
        if (!lower.startsWith(filterLower)) return false;
        break;
      case "endsWith":
        if (!lower.endsWith(filterLower)) return false;
        break;
      case "equals":
        if (lower !== filterLower) return false;
        break;
      case "gt":
      case "lt":
      case "gte":
      case "lte": {
        const numCell = Number(cellValue);
        const numFilter = Number(value);
        if (isNaN(numCell) || isNaN(numFilter)) return false;
        if (op === "gt"  && !(numCell >  numFilter)) return false;
        if (op === "lt"  && !(numCell <  numFilter)) return false;
        if (op === "gte" && !(numCell >= numFilter)) return false;
        if (op === "lte" && !(numCell <= numFilter)) return false;
        break;
      }
    }
  }

  return true;
}

export default function ColumnFilterDropdown({
  columnValues,
  truncated,
  currentFilter,
  onApply,
  onClose,
  position,
}: Props) {
  const dropdownRef = useRef<HTMLDivElement>(null);

  // columnValues are already sorted unique strings from ResultGrid's filterColumnData
  const uniqueValues = useMemo(
    () => columnValues.map((v) => (v == null ? "" : String(v))),
    [columnValues],
  );

  const [searchText, setSearchText] = useState("");
  const [checkedValues, setCheckedValues] = useState<Set<string>>(() => {
    if (!currentFilter?.checkedValues) return new Set(uniqueValues);
    // Union restored filter with any new values that appeared after re-run,
    // so newly added rows aren't silently excluded.
    const restored = new Set(currentFilter.checkedValues);
    for (const v of uniqueValues) {
      if (!currentFilter.checkedValues.has(v)) restored.add(v);
    }
    return restored;
  });
  const [conditionOp, setConditionOp] = useState<ConditionOp>(
    currentFilter?.condition?.op ?? "contains",
  );
  const [conditionValue, setConditionValue] = useState(
    currentFilter?.condition?.value ?? "",
  );

  const filteredValues = useMemo(() => {
    if (!searchText) return uniqueValues;
    const lower = searchText.toLowerCase();
    return uniqueValues.filter((v) => v.toLowerCase().includes(lower));
  }, [uniqueValues, searchText]);

  const allChecked = filteredValues.every((v) => checkedValues.has(v));

  const toggleAll = useCallback(() => {
    if (allChecked) {
      const next = new Set(checkedValues);
      for (const v of filteredValues) next.delete(v);
      setCheckedValues(next);
    } else {
      const next = new Set(checkedValues);
      for (const v of filteredValues) next.add(v);
      setCheckedValues(next);
    }
  }, [allChecked, checkedValues, filteredValues]);

  const handleApply = () => {
    const isAllChecked = uniqueValues.every((v) => checkedValues.has(v));
    const hasCondition = conditionValue.trim() !== "";

    if (isAllChecked && !hasCondition) {
      onApply(undefined); // Clear filter
    } else {
      onApply({
        checkedValues: isAllChecked ? undefined : checkedValues,
        condition: hasCondition ? { op: conditionOp, value: conditionValue } : undefined,
      });
    }
    onClose();
  };

  const handleClear = () => {
    onApply(undefined);
    onClose();
  };

  // Dismiss on outside click or Escape
  useEffect(() => {
    const dismiss = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("mousedown", dismiss);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", dismiss);
      document.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  // Clamp dropdown to viewport so it doesn't render off-screen
  useLayoutEffect(() => {
    const el = dropdownRef.current;
    if (!el) return;
    const { width, height } = el.getBoundingClientRect();
    const pad = 8;
    el.style.left = `${Math.max(pad, Math.min(position.x, window.innerWidth - width - pad))}px`;
    el.style.top = `${Math.max(pad, Math.min(position.y, window.innerHeight - height - pad))}px`;
  }, [position]);

  return (
    <div
      ref={dropdownRef}
      onMouseDown={(e) => e.stopPropagation()}
      style={{
        position: "fixed",
        top: position.y,
        left: position.x,
        zIndex: 9999,
        background: "var(--bg-overlay)",
        border: "1px solid var(--border)",
        borderRadius: 6,
        boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
        width: 260,
        padding: 12,
        fontSize: 12,
      }}
    >
      {/* Value checklist */}
      <Input
        size="small"
        placeholder="Search values..."
        value={searchText}
        onChange={(e) => setSearchText(e.target.value)}
        style={{ marginBottom: 8, fontSize: 11 }}
        allowClear
      />
      <div style={{ marginBottom: 4 }}>
        <Checkbox checked={allChecked} onChange={toggleAll} style={{ fontSize: 11 }}>
          (Select All)
        </Checkbox>
      </div>
      <div style={{ maxHeight: 180, overflowY: "auto", marginBottom: 8 }}>
        {filteredValues.map((val) => (
          <div key={val || "__blank__"} style={{ padding: "1px 0" }}>
            <Checkbox
              checked={checkedValues.has(val)}
              onChange={() => {
                const next = new Set(checkedValues);
                if (next.has(val)) next.delete(val);
                else next.add(val);
                setCheckedValues(next);
              }}
              style={{ fontSize: 11 }}
            >
              {val || <span style={{ color: "var(--text-faint)", fontStyle: "italic" }}>(blank)</span>}
            </Checkbox>
          </div>
        ))}
        {truncated && (
          <div style={{ padding: "4px 0", fontSize: 10, color: "var(--text-faint)", fontStyle: "italic" }}>
            Showing first 1,000 unique values
          </div>
        )}
      </div>

      <Divider style={{ margin: "8px 0" }} />

      {/* Condition filter */}
      <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 4 }}>
        <FilterOutlined style={{ marginRight: 4 }} />
        Condition
      </div>
      <Space direction="vertical" size={4} style={{ width: "100%" }}>
        <Select
          size="small"
          value={conditionOp}
          onChange={(v) => setConditionOp(v)}
          style={{ width: "100%", fontSize: 11 }}
          options={[
            { label: "Contains", value: "contains" },
            { label: "Starts With", value: "startsWith" },
            { label: "Ends With", value: "endsWith" },
            { label: "Equals", value: "equals" },
            { label: "Greater Than", value: "gt" },
            { label: "Less Than", value: "lt" },
            { label: "Greater or Equal", value: "gte" },
            { label: "Less or Equal", value: "lte" },
          ]}
        />
        <Input
          size="small"
          placeholder="Value..."
          value={conditionValue}
          onChange={(e) => setConditionValue(e.target.value)}
          style={{ fontSize: 11 }}
        />
      </Space>

      <div style={{ display: "flex", justifyContent: "flex-end", gap: 6, marginTop: 12 }}>
        <Button size="small" onClick={handleClear}>Clear</Button>
        <Button size="small" type="primary" onClick={handleApply}>Apply</Button>
      </div>
    </div>
  );
}
