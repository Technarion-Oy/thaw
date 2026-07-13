// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { Dropdown, Button } from "antd";
import { FieldNumberOutlined } from "@ant-design/icons";
import { needsQuoting, quoteIdent } from "./ObjectNameCaseControl";

// A schema-qualified sequence. Structurally matches the QualifiedObject rows the
// Column Properties modal parses out of SHOW SEQUENCES IN ACCOUNT.
export interface SequenceRef {
  db: string;
  schema: string;
  name: string;
  fqn: string;
}

// Build a `<db>.<schema>.<seq>.NEXTVAL` reference, quoting each identifier part
// only when it can't be expressed bare (special characters / reserved keyword).
// This is the one default expression Snowflake accepts on an existing column via
// ALTER TABLE … ALTER COLUMN … SET DEFAULT.
export function sequenceNextval(seq: { db: string; schema: string; name: string }): string {
  const part = (s: string) => (needsQuoting(s) ? quoteIdent(s) : s);
  return [seq.db, seq.schema, seq.name].filter(Boolean).map(part).join(".") + ".NEXTVAL";
}

/**
 * Small dropdown button (# icon) that fills a column DEFAULT with a sequence
 * NEXTVAL reference chosen from the account's sequences. Unlike the built-in
 * function picker (valid only at table-create time), a sequence reference is the
 * one default Snowflake permits on an existing column, so this is the picker
 * offered by the Column Properties modal. Disabled when no sequences are known.
 */
export default function SequenceDefaultPicker({ sequences, onPick }: { sequences: SequenceRef[]; onPick: (sql: string) => void }) {
  const disabled = sequences.length === 0;
  return (
    <Dropdown
      trigger={["click"]}
      disabled={disabled}
      menu={{
        style: { maxHeight: 320, overflowY: "auto" },
        items: sequences.map((s) => ({
          key: s.fqn,
          label: <code>{s.fqn}</code>,
          onClick: () => onPick(sequenceNextval(s)),
        })),
      }}
    >
      <Button
        size="small"
        icon={<FieldNumberOutlined />}
        disabled={disabled}
        title={disabled ? "No sequences available" : "Insert a sequence default (NEXTVAL)"}
      />
    </Dropdown>
  );
}
