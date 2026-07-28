// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useRef, type ReactNode } from "react";
import { Input, Space, Tag, Typography } from "antd";
import type { InputRef } from "antd";
import {
  DEFAULT_EXPORT_PATH_TEMPLATE,
  PATH_TEMPLATE_VARIABLES,
  applyTemplate,
  insertPlaceholder,
  validateTemplate,
} from "./pathTemplate";

const { Text } = Typography;

interface Props {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  /** Rendered directly under the input, e.g. "Applies to this export only." */
  hint?: ReactNode;
}

/**
 * The DDL export file-path template input, shared by `ExportOptionsModal` and
 * `ExportPathFormatModal`: a monospace text field, one clickable insert tag per
 * supported placeholder, and a live preview with example values substituted.
 *
 * Clicking a tag inserts at the caret (replacing the selection) and puts the
 * caret back after the inserted text, so a template can be built in any order.
 * When the input is not focused the placeholder is appended.
 *
 * A template that does not end in `.sql` is flagged inline; the owning dialog
 * calls `validateTemplate()` itself to disable its primary button.
 */
export default function PathTemplateField({ label, value, onChange, disabled, hint }: Props) {
  const error = validateTemplate(value);
  const inputRef = useRef<InputRef>(null);
  // Caret to restore once React has committed the new value; the DOM node still
  // holds the *old* selection at the time onChange runs.
  const pendingCaret = useRef<number | null>(null);

  useEffect(() => {
    const caret = pendingCaret.current;
    if (caret == null) return;
    pendingCaret.current = null;
    const el = inputRef.current?.input;
    if (!el) return;
    el.focus();
    el.setSelectionRange(caret, caret);
  }, [value]);

  function insert(placeholder: string) {
    const el = inputRef.current?.input;
    // An input that was never focused reports selectionStart 0, which would
    // prepend rather than append — only trust the selection while it has focus.
    const focused = !!el && document.activeElement === el;
    const next = insertPlaceholder(
      value,
      placeholder,
      focused ? el!.selectionStart : null,
      focused ? el!.selectionEnd : null,
    );
    pendingCaret.current = next.caret;
    onChange(next.value);
  }

  return (
    <div>
      <Text strong style={{ display: "block", marginBottom: 6 }}>{label}</Text>
      <Input
        ref={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        status={error ? "error" : undefined}
        placeholder={DEFAULT_EXPORT_PATH_TEMPLATE}
        style={{ fontFamily: "monospace" }}
      />
      {error ? (
        <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>
      ) : hint ? (
        <Text type="secondary" style={{ fontSize: 11 }}>{hint}</Text>
      ) : null}

      <div style={{ marginTop: 8 }}>
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
          Click to insert at the cursor:
        </Text>
        <Space wrap size={[4, 4]}>
          {PATH_TEMPLATE_VARIABLES.map((v) => (
            <span key={v.name} title={v.desc}>
              <Tag
                style={{
                  cursor: disabled ? "not-allowed" : "pointer",
                  fontFamily: "monospace",
                  opacity: disabled ? 0.5 : 1,
                }}
                // Keep the focus (and therefore the caret) in the input while
                // the tag is clicked.
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => { if (!disabled) insert(v.name); }}
              >
                {v.name}
              </Tag>
              <Text type="secondary" style={{ fontSize: 11 }}>{v.desc}</Text>
            </span>
          ))}
        </Space>
      </div>

      <div style={{ marginTop: 8 }}>
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
          Preview
        </Text>
        <div
          style={{
            fontFamily: "monospace",
            fontSize: 12,
            padding: "6px 10px",
            background: "var(--bg-raised)",
            borderRadius: 4,
            border: "1px solid var(--border)",
            color: "var(--text)",
            overflowX: "auto",
            whiteSpace: "nowrap",
          }}
        >
          {applyTemplate(value)}
        </div>
      </div>
    </div>
  );
}
