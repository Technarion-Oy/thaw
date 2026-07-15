// SPDX-License-Identifier: GPL-3.0-or-later

import { Dropdown, Button } from "antd";
import { FunctionOutlined } from "@ant-design/icons";
import { DEFAULT_FUNCTIONS } from "./builtinFunctions";

/**
 * Small dropdown button that fills a column DEFAULT with a built-in function.
 * Shared by the Create Table dialog and the ER Designer column editor.
 */
export default function DefaultFunctionPicker({ onPick }: { onPick: (sql: string) => void }) {
  return (
    <Dropdown
      trigger={["click"]}
      menu={{
        items: DEFAULT_FUNCTIONS.map((f) => ({
          key: f.sql,
          label: (
            <span>
              <code>{f.sql}</code>{" "}
              <span style={{ color: "var(--text-muted)", fontSize: 11 }}>{f.desc}</span>
            </span>
          ),
          onClick: () => onPick(f.sql),
        })),
      }}
    >
      <Button size="small" icon={<FunctionOutlined />} title="Insert function default" />
    </Dropdown>
  );
}
