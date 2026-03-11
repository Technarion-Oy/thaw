// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useMemo } from "react";
import { Modal, Input, Button } from "antd";
import { useQueryStore } from "../../store/queryStore";

interface Snippet  { name: string; sql: string; }
interface Category { label: string; items: Snippet[]; }

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
];

interface Props {
  onClose: () => void;
}

export default function SnippetsModal({ onClose }: Props) {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<Snippet>(CATEGORIES[0].items[0]);
  const loadInNewTab = useQueryStore((s) => s.loadInNewTab);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return CATEGORIES;
    return CATEGORIES.map((cat) => ({
      ...cat,
      items: cat.items.filter((item) => item.name.toLowerCase().includes(q)),
    })).filter((cat) => cat.items.length > 0);
  }, [search]);

  // Auto-select first match when search changes
  const firstMatch = filtered[0]?.items[0];
  const effectiveSelected =
    search.trim() && firstMatch && !filtered.some((c) => c.items.includes(selected))
      ? firstMatch
      : selected;

  const handleSearch = (value: string) => {
    setSearch(value);
    const q = value.trim().toLowerCase();
    if (q) {
      const match = CATEGORIES.flatMap((c) => c.items).find((i) => i.name.toLowerCase().includes(q));
      if (match) setSelected(match);
    }
  };

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
        {/* Left panel */}
        <div
          style={{
            width: 220,
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
              onChange={(e) => handleSearch(e.target.value)}
              allowClear
            />
          </div>
          <div style={{ flex: 1, overflowY: "auto", padding: "4px 0" }}>
            {filtered.map((cat) => (
              <div key={cat.label}>
                <div
                  style={{
                    padding: "4px 12px",
                    fontSize: 11,
                    fontWeight: 600,
                    color: "var(--text-muted)",
                    textTransform: "uppercase",
                    letterSpacing: "0.04em",
                  }}
                >
                  {cat.label}
                </div>
                {cat.items.map((item) => {
                  const isActive = item === effectiveSelected;
                  return (
                    <div
                      key={item.name}
                      onClick={() => setSelected(item)}
                      style={{
                        padding: "5px 12px 5px 20px",
                        fontSize: 12,
                        cursor: "pointer",
                        background: isActive ? "var(--border)" : "transparent",
                        color: isActive ? "var(--text)" : "var(--text-muted)",
                        borderRadius: 4,
                        margin: "1px 4px",
                        whiteSpace: "nowrap",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                      }}
                    >
                      {item.name}
                    </div>
                  );
                })}
              </div>
            ))}
            {filtered.length === 0 && (
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
            <code>{effectiveSelected.sql}</code>
          </pre>
          <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 12 }}>
            <Button
              type="primary"
              onClick={() => {
                loadInNewTab(effectiveSelected.sql);
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
