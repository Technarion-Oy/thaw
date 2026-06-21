// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { AutoComplete } from "antd";
import { SNOWFLAKE_DATA_TYPES, SNOWFLAKE_DATA_TYPE_NAMES } from "../../generated/snowflakeDataTypes";

// Data-type suggestions come from the backend registry (snowflake.AllDataTypes,
// surfaced via the generated snowflakeDataTypes.ts) so the list stays in sync
// with Snowflake's supported types rather than being hand-maintained. The
// AutoComplete still accepts any free-form text (e.g. NUMBER(38,0)), so the
// dropdown is a convenience, not a constraint.
const TYPE_OPTIONS = SNOWFLAKE_DATA_TYPES.map((dt) => ({
  value: dt.name,
  label: dt.paramHint ? `${dt.name} ${dt.paramHint}` : dt.name,
}));

// Set of canonical type names used to decide when to show the full suggestion
// list (see filterOption below).
const TYPE_NAME_SET = new Set(SNOWFLAKE_DATA_TYPE_NAMES);

interface Props {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  style?: React.CSSProperties;
}

/**
 * A data-type picker backed by the generated Snowflake data-type registry. It is
 * an AutoComplete (not a closed Select) so a parameterised type like
 * `NUMBER(38,0)` or `VARCHAR(255)` can still be typed in full.
 */
export default function DataTypeAutoComplete({
  value,
  onChange,
  placeholder = "TYPE (e.g. NUMBER)",
  style,
}: Props) {
  return (
    <AutoComplete
      placeholder={placeholder}
      value={value}
      options={TYPE_OPTIONS}
      onChange={onChange}
      // Show the full type list when the field already holds a complete,
      // recognised type (so the user can switch to another); otherwise filter by
      // substring as they type. Without this, the current value (e.g. "NUMBER")
      // would filter the dropdown down to a single matching option.
      filterOption={(input, option) => {
        const inp = input.toUpperCase();
        if (TYPE_NAME_SET.has(inp)) return true;
        return (option?.value ?? "").toUpperCase().includes(inp);
      }}
      style={style ?? { width: 220 }}
    />
  );
}
