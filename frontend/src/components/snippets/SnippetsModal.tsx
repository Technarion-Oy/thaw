// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { useState, useMemo, useEffect } from "react";
import { Modal, Input, Button, Menu } from "antd";
import type { MenuProps } from "antd";
import { useQueryStore } from "../../store/queryStore";
import { FUNCTION_CATEGORIES } from "../editor/snowflakeSql";

interface Snippet { name: string; sql: string; }
// A category is either a leaf group (has `items`) or a parent (has `children`),
// letting the left panel render as a cascading multi-level menu.
interface Category { label: string; items?: Snippet[]; children?: Category[]; }

// Built-in functions, sourced from the shared catalogue (no duplicate list) —
// callable form (`NAME()`) as the snippet body.
const fnItems = (names: readonly string[]): Snippet[] => names.map((n) => ({ name: n, sql: `${n}()` }));

const FUNCTIONS_CATEGORY: Category = {
  label: "Built-in Functions",
  children: FUNCTION_CATEGORIES.map((cat) => ({ label: cat.name, items: fnItems(cat.fns) })),
};

const CATEGORIES: Category[] = [
  {
    label: "Data Objects",
    items: [
      {
        name: "Table",
        sql: `CREATE OR REPLACE TABLE db.schema.my_table (
    id          NUMBER        NOT NULL PRIMARY KEY,
    name        VARCHAR(255)  NOT NULL,
    created_at  TIMESTAMP_NTZ DEFAULT CURRENT_TIMESTAMP()
)
DATA_RETENTION_TIME_IN_DAYS = 1
COMMENT = '';`,
      },
      {
        name: "View",
        sql: `CREATE OR REPLACE VIEW db.schema.my_view
COMMENT = ''
AS
SELECT id, name, created_at
FROM db.schema.my_table;`,
      },
      {
        name: "Materialized View",
        sql: `CREATE OR REPLACE MATERIALIZED VIEW db.schema.my_mv
COMMENT = ''
AS
SELECT id, COUNT(*) AS row_count
FROM db.schema.my_table
GROUP BY id;`,
      },
      {
        name: "Dynamic Table",
        sql: `CREATE OR REPLACE DYNAMIC TABLE db.schema.my_dynamic_table
    TARGET_LAG = '1 minute'
    WAREHOUSE  = my_warehouse
    COMMENT    = ''
AS
SELECT id, name, created_at
FROM db.schema.source_table;`,
      },
      {
        name: "Sequence",
        sql: `CREATE OR REPLACE SEQUENCE db.schema.my_seq
    START  = 1
    INCREMENT = 1
    COMMENT = '';`,
      },
    ],
  },
  {
    label: "Code",
    items: [
      {
        name: "Stored Procedure (Snowflake Scripting)",
        sql: `CREATE OR REPLACE PROCEDURE db.schema.my_procedure(param1 VARCHAR)
RETURNS VARCHAR
LANGUAGE SQL
EXECUTE AS CALLER
AS
$$
DECLARE
    result VARCHAR;
BEGIN
    result := 'Hello, ' || param1;
    RETURN result;
END;
$$;`,
      },
      {
        name: "Stored Procedure (Python)",
        sql: `CREATE OR REPLACE PROCEDURE db.schema.my_procedure(param1 VARCHAR)
RETURNS VARCHAR
LANGUAGE PYTHON
RUNTIME_VERSION = '3.11'
PACKAGES = ('snowflake-snowpark-python')
HANDLER = 'run'
EXECUTE AS CALLER
AS
$$
def run(session, param1: str) -> str:
    return f'Hello, {param1}'
$$;`,
      },
      {
        name: "UDF (SQL)",
        sql: `CREATE OR REPLACE FUNCTION db.schema.my_udf(x NUMBER)
RETURNS NUMBER
LANGUAGE SQL
AS
$$
    x * x
$$;`,
      },
      {
        name: "UDF (JavaScript)",
        sql: `CREATE OR REPLACE FUNCTION db.schema.my_udf(x FLOAT)
RETURNS FLOAT
LANGUAGE JAVASCRIPT
AS
$$
    return X * X;
$$;`,
      },
      {
        name: "UDF (Python)",
        sql: `CREATE OR REPLACE FUNCTION db.schema.my_udf(x FLOAT)
RETURNS FLOAT
LANGUAGE PYTHON
RUNTIME_VERSION = '3.11'
HANDLER = 'udf'
AS
$$
def udf(x: float) -> float:
    return x * x
$$;`,
      },
    ],
  },
  {
    label: "Automation",
    items: [
      {
        name: "Task",
        sql: `CREATE OR REPLACE TASK db.schema.my_task
    WAREHOUSE = my_warehouse
    SCHEDULE  = 'USING CRON 0 * * * * UTC'
    COMMENT   = ''
AS
    INSERT INTO db.schema.target_table
    SELECT * FROM db.schema.source_table;`,
      },
      {
        name: "Stream on Table",
        sql: `CREATE OR REPLACE STREAM db.schema.my_stream
    ON TABLE db.schema.my_table
    APPEND_ONLY = FALSE
    COMMENT = '';`,
      },
      {
        name: "Pipe",
        sql: `CREATE OR REPLACE PIPE db.schema.my_pipe
    AUTO_INGEST = TRUE
    COMMENT = ''
AS
COPY INTO db.schema.my_table
FROM @db.schema.my_stage/path/
FILE_FORMAT = (TYPE = 'CSV' SKIP_HEADER = 1);`,
      },
      {
        name: "Alert",
        sql: `CREATE OR REPLACE ALERT db.schema.my_alert
    WAREHOUSE = my_warehouse
    SCHEDULE  = '5 MINUTE'
    IF (EXISTS (
        SELECT 1 FROM db.schema.my_table
        WHERE created_at >= SNOWFLAKE.ALERT.LAST_SUCCESSFUL_SCHEDULED_TIME()
    ))
    THEN
        CALL SYSTEM$SEND_EMAIL(
            'my_notification_integration',
            'recipient@example.com',
            'Alert triggered',
            'Alert condition met.'
        );`,
      },
    ],
  },
  {
    label: "Storage",
    items: [
      {
        name: "Stage (Internal)",
        sql: `CREATE OR REPLACE STAGE db.schema.my_stage
    FILE_FORMAT = (TYPE = 'CSV' COMPRESSION = 'AUTO')
    COMMENT = '';`,
      },
      {
        name: "Stage (External S3)",
        sql: `CREATE OR REPLACE STAGE db.schema.my_ext_stage
    URL = 's3://my-bucket/path/'
    CREDENTIALS = (AWS_KEY_ID = '' AWS_SECRET_KEY = '')
    FILE_FORMAT = (TYPE = 'CSV' SKIP_HEADER = 1)
    COMMENT = '';`,
      },
      {
        name: "File Format (CSV)",
        sql: `CREATE OR REPLACE FILE FORMAT db.schema.my_csv_format
    TYPE = 'CSV'
    FIELD_DELIMITER = ','
    SKIP_HEADER = 1
    NULL_IF = ('NULL', 'null', '')
    EMPTY_FIELD_AS_NULL = TRUE
    COMPRESSION = 'AUTO'
    COMMENT = '';`,
      },
      {
        name: "File Format (Parquet)",
        sql: `CREATE OR REPLACE FILE FORMAT db.schema.my_parquet_format
    TYPE = 'PARQUET'
    SNAPPY_COMPRESSION = TRUE
    COMMENT = '';`,
      },
    ],
  },
  {
    label: "Governance",
    items: [
      {
        name: "Network Policy",
        sql: `CREATE OR REPLACE NETWORK POLICY my_network_policy
    ALLOWED_IP_LIST   = ('0.0.0.0/0')
    BLOCKED_IP_LIST   = ()
    COMMENT = '';`,
      },
      {
        name: "Resource Monitor",
        sql: `CREATE OR REPLACE RESOURCE MONITOR my_resource_monitor
    CREDIT_QUOTA = 100
    FREQUENCY    = MONTHLY
    START_TIMESTAMP = IMMEDIATELY
    TRIGGERS
        ON 75 PERCENT DO NOTIFY
        ON 100 PERCENT DO SUSPEND;`,
      },
    ],
  },
  {
    label: "Infrastructure",
    items: [
      {
        name: "Database",
        sql: `CREATE OR REPLACE DATABASE my_database
    DATA_RETENTION_TIME_IN_DAYS = 1
    COMMENT = '';`,
      },
      {
        name: "Schema",
        sql: `CREATE OR REPLACE SCHEMA db.my_schema
    DATA_RETENTION_TIME_IN_DAYS = 1
    COMMENT = '';`,
      },
      {
        name: "Warehouse",
        sql: `CREATE OR REPLACE WAREHOUSE my_warehouse
    WAREHOUSE_SIZE   = 'X-SMALL'
    AUTO_SUSPEND     = 60
    AUTO_RESUME      = TRUE
    INITIALLY_SUSPENDED = TRUE
    COMMENT = '';`,
      },
    ],
  },
  FUNCTIONS_CATEGORY,
];

// ── Tree helpers ──────────────────────────────────────────────────────────────

type MenuItems = NonNullable<MenuProps["items"]>;

/** Prune the tree to categories/items whose snippet name matches the query. */
function filterTree(nodes: Category[], q: string): Category[] {
  if (!q) return nodes;
  const out: Category[] = [];
  for (const n of nodes) {
    if (n.children) {
      const kids = filterTree(n.children, q);
      if (kids.length) out.push({ ...n, children: kids });
    } else if (n.items) {
      const items = n.items.filter((i) => i.name.toLowerCase().includes(q));
      if (items.length) out.push({ ...n, items });
    }
  }
  return out;
}

/** Build AntD Menu items and a key→snippet lookup. Leaf keys are full paths. */
function buildMenu(nodes: Category[], path: string, sink: Map<string, Snippet>): MenuItems {
  return nodes.map((n) => {
    const key = path ? `${path}/${n.label}` : n.label;
    const children: MenuItems = n.children
      ? buildMenu(n.children, key, sink)
      : (n.items ?? []).map((it) => {
          const ik = `${key}/${it.name}`;
          sink.set(ik, it);
          return { key: ik, label: it.name };
        });
    return { key, label: n.label, children };
  });
}

/** All non-leaf (category) keys — used to expand everything while searching. */
function categoryKeys(nodes: Category[], path: string): string[] {
  const keys: string[] = [];
  for (const n of nodes) {
    const key = path ? `${path}/${n.label}` : n.label;
    keys.push(key);
    if (n.children) keys.push(...categoryKeys(n.children, key));
  }
  return keys;
}

/** First leaf key in document order (default selection). */
function firstLeaf(nodes: Category[], sink: Map<string, Snippet>): string | undefined {
  return sink.keys().next().value ?? categoryKeys(nodes, "")[0];
}

interface Props {
  onClose: () => void;
}

export default function SnippetsModal({ onClose }: Props) {
  const [search, setSearch] = useState("");
  const [openKeys, setOpenKeys] = useState<string[]>([CATEGORIES[0].label]);
  const loadInNewTab = useQueryStore((s) => s.loadInNewTab);

  const q = search.trim().toLowerCase();

  const { menuItems, snippetMap, allCategoryKeys, defaultLeaf } = useMemo(() => {
    const tree = filterTree(CATEGORIES, q);
    const map = new Map<string, Snippet>();
    const items = buildMenu(tree, "", map);
    return {
      menuItems: items,
      snippetMap: map,
      allCategoryKeys: categoryKeys(tree, ""),
      defaultLeaf: firstLeaf(tree, map),
    };
  }, [q]);

  const [selectedKey, setSelectedKey] = useState<string | undefined>(defaultLeaf);

  // While searching, expand every matching category and jump to the first hit.
  // When cleared, collapse back to the first top-level category.
  useEffect(() => {
    if (q) {
      setOpenKeys(allCategoryKeys);
      if (defaultLeaf) setSelectedKey(defaultLeaf);
    } else {
      // Collapse back to the first top-level category and re-select its first
      // item, so the previewed/highlighted snippet is visible in the tree
      // (a match selected under a now-collapsed nested category would vanish).
      setOpenKeys([CATEGORIES[0].label]);
      if (defaultLeaf) setSelectedKey(defaultLeaf);
    }
  }, [q, allCategoryKeys, defaultLeaf]);

  const selected = (selectedKey && snippetMap.get(selectedKey)) || (defaultLeaf ? snippetMap.get(defaultLeaf) : undefined);

  return (
    <Modal
      open
      title="Code Snippets"
      width={900}
      footer={null}
      onCancel={onClose}
      styles={{ body: { padding: 0 } }}
    >
      <div style={{ display: "flex", height: 520 }}>
        {/* Left panel — cascading menu */}
        <div
          style={{
            width: 260,
            flexShrink: 0,
            borderRight: "1px solid var(--border)",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <div style={{ padding: "8px 8px 4px" }}>
            <Input
              placeholder="Search snippets…"
              size="small"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              allowClear
            />
          </div>
          <div style={{ flex: 1, overflowY: "auto", padding: "4px 0" }}>
            {menuItems.length > 0 ? (
              <Menu
                mode="inline"
                items={menuItems}
                openKeys={openKeys}
                onOpenChange={(keys) => setOpenKeys(keys as string[])}
                selectedKeys={selectedKey ? [selectedKey] : []}
                onClick={({ key }) => setSelectedKey(key)}
                style={{ border: "none", background: "transparent", fontSize: 12 }}
                inlineIndent={16}
              />
            ) : (
              <div style={{ padding: "12px", fontSize: 12, color: "var(--text-muted)" }}>
                No snippets match.
              </div>
            )}
          </div>
        </div>

        {/* Right panel */}
        <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden", padding: "12px 16px 12px 12px" }}>
          <pre
            style={{
              flex: 1,
              margin: 0,
              padding: "12px",
              background: "var(--bg-subtle, var(--bg-raised))",
              borderRadius: 6,
              border: "1px solid var(--border)",
              overflow: "auto",
              fontSize: 12,
              fontFamily: "monospace",
              whiteSpace: "pre",
              color: "var(--text)",
              lineHeight: 1.6,
            }}
          >
            <code>{selected?.sql ?? ""}</code>
          </pre>
          <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 12 }}>
            <Button
              type="primary"
              disabled={!selected}
              onClick={() => {
                if (!selected) return;
                loadInNewTab(selected.sql);
                onClose();
              }}
            >
              Open in New Tab
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
}
