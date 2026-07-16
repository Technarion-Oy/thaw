// SPDX-License-Identifier: GPL-3.0-or-later

// A boolean toggle that STAGES its change instead of committing on flip.
//
// Flipping the switch to a value different from the committed `checked` reveals
// Save / Cancel controls: Cancel reverts to the committed value with no write,
// Save commits via `onConfirm`. Toggling back to the committed value also
// clears the pending state. This gives every Properties-modal boolean an
// "unselect" path (issue #519) so an in-progress toggle can be backed out
// in place, without closing and reopening the modal.

import { useState } from "react";
import { Switch, Button, Tooltip } from "antd";
import { CheckOutlined, CloseOutlined } from "@ant-design/icons";
import { friendlyError } from "./errors";

export interface ConfirmSwitchProps {
  /** Committed (loaded) value. */
  checked:   boolean;
  size?:     "small" | "default";
  /** Commit the staged value. Should perform the ALTER and refresh so that,
   *  once it resolves, `checked` reflects the new value. Throws on failure. */
  onConfirm: (next: boolean) => Promise<void>;
}

export function ConfirmSwitch({ checked, size = "small", onConfirm }: ConfirmSwitchProps) {
  const [pending, setPending] = useState<boolean | null>(null);
  const [saving,  setSaving]  = useState(false);
  const [error,   setError]   = useState<string | null>(null);

  // A change is staged only while pending differs from the committed value.
  const staged  = pending !== null && pending !== checked;
  const display = pending ?? checked;

  const onChange = (next: boolean) => {
    setError(null);
    // Toggling back to the committed value clears the pending state entirely.
    setPending(next === checked ? null : next);
  };

  const cancel = () => { setPending(null); setError(null); };

  const save = async () => {
    if (pending === null) return;
    setSaving(true);
    setError(null);
    try {
      await onConfirm(pending);
      setPending(null);
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <Switch
          size={size}
          checked={display}
          disabled={saving}
          loading={saving}
          onChange={onChange}
        />
        {staged && (
          <>
            <Tooltip title="Save">
              <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={save} />
            </Tooltip>
            <Tooltip title="Cancel">
              <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancel} />
            </Tooltip>
          </>
        )}
      </div>
      {error && (
        <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace", lineHeight: 1.4, paddingLeft: 2 }}>
          {error}
        </div>
      )}
    </div>
  );
}
