// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import { Form, Input, Select } from "antd";

export const ALLOWED_VALUES_SEQUENCE = "ALLOWED_VALUES_SEQUENCE";

export const PROPAGATE_OPTIONS = [
  { value: "", label: "Disabled (no propagation)" },
  { value: "ON_DEPENDENCY_AND_DATA_MOVEMENT", label: "On dependency and data movement" },
  { value: "ON_DEPENDENCY", label: "On dependency only" },
  { value: "ON_DATA_MOVEMENT", label: "On data movement only" },
];

// How ON_CONFLICT is expressed: omit it, use the ALLOWED_VALUES_SEQUENCE
// keyword, or pin a fixed string value.
type ConflictMode = "none" | "sequence" | "value";

function deriveMode(onConflict: string): ConflictMode {
  if (onConflict === "") return "none";
  if (onConflict === ALLOWED_VALUES_SEQUENCE) return "sequence";
  return "value";
}

interface Props {
  /** PROPAGATE mode ("" disables it); one of PROPAGATE_OPTIONS' values. */
  propagate: string;
  /** Resolved ON_CONFLICT: "" (none), ALLOWED_VALUES_SEQUENCE, or a fixed value. */
  onConflict: string;
  /** Emits the next resolved { propagate, onConflict }. */
  onChange: (next: { propagate: string; onConflict: string }) => void;
  itemStyle?: React.CSSProperties;
  disabled?: boolean;
}

// TagPropagationFields renders the PROPAGATE mode picker and its nested
// ON_CONFLICT control, shared by the Create Tag modal and the Tag properties
// panel. It is fully controlled by (propagate, onConflict) and reports the
// resolved values back through onChange; the ON_CONFLICT sub-mode and the
// fixed-value draft are tracked locally and resynced when the controlled values
// change from outside (e.g. when the properties panel loads current settings).
export default function TagPropagationFields({ propagate, onConflict, onChange, itemStyle, disabled }: Props) {
  const [mode, setMode] = useState<ConflictMode>(deriveMode(onConflict));
  const [valueDraft, setValueDraft] = useState(deriveMode(onConflict) === "value" ? onConflict : "");

  // Resync the local sub-mode / draft when the controlled values change from
  // outside. While the user is mid-edit in "value" mode we keep that mode even
  // if the resolved value is momentarily empty.
  useEffect(() => {
    if (onConflict === ALLOWED_VALUES_SEQUENCE) {
      setMode("sequence");
    } else if (onConflict === "") {
      setMode((m) => (m === "value" ? m : "none"));
    } else {
      setMode("value");
      setValueDraft(onConflict);
    }
  }, [onConflict, propagate]);

  const applyConflict = (nextMode: ConflictMode, draft: string) => {
    setMode(nextMode);
    setValueDraft(draft);
    const oc = nextMode === "sequence" ? ALLOWED_VALUES_SEQUENCE : nextMode === "value" ? draft : "";
    onChange({ propagate, onConflict: oc });
  };

  const setPropagate = (v: string) => {
    // ON_CONFLICT only applies alongside PROPAGATE — clear it when disabling.
    if (!v) {
      setMode("none");
      setValueDraft("");
      onChange({ propagate: "", onConflict: "" });
      return;
    }
    onChange({ propagate: v, onConflict });
  };

  return (
    <>
      <Form.Item
        label="Propagate"
        style={itemStyle}
        help="Automatically propagate this tag from source objects to target objects."
      >
        <Select
          value={propagate}
          onChange={setPropagate}
          options={PROPAGATE_OPTIONS}
          disabled={disabled}
        />
      </Form.Item>

      {propagate && (
        <Form.Item
          label="On conflict"
          style={itemStyle}
          help="How to resolve conflicts between propagated tag values."
        >
          <Select
            value={mode}
            onChange={(m) => applyConflict(m, valueDraft)}
            disabled={disabled}
            options={[
              { value: "none", label: "Default (no override)" },
              { value: "sequence", label: "By allowed-values order (ALLOWED_VALUES_SEQUENCE)" },
              { value: "value", label: "Fixed value…" },
            ]}
          />
          {mode === "value" && (
            <Input
              style={{ marginTop: 8 }}
              value={valueDraft}
              onChange={(e) => applyConflict("value", e.target.value)}
              placeholder="conflict value"
              disabled={disabled}
            />
          )}
        </Form.Item>
      )}
    </>
  );
}
