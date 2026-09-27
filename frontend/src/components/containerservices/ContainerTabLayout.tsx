// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Alert, Button, Empty, Input, Space, Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { PlusOutlined, ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import { useSessionStore } from "../../store/sessionStore";

const { Text } = Typography;

interface Props<T> {
  /** Plural object name for the empty state and the row counter, e.g. "compute pools". */
  objectLabel: string;
  columns: ColumnsType<T>;
  rows: T[];
  rowKey: keyof T & string;
  loading?: boolean;
  /** Error banner text; hidden when null/undefined. */
  error?: string | null;
  onRefresh?: () => void;
  /** Primary action label, e.g. "New pool…". Omit to hide the button. */
  newLabel?: string;
  onNew?: () => void;
  /** Extra facet selects rendered between the search box and the refresh button. */
  facets?: ReactNode;
  /** Free-text match against one row; `q` is already lower-cased. */
  filter?: (row: T, q: string) => boolean;
  /** Detail panel for the selected row. Omit for tabs with no detail view. */
  detail?: (row: T) => ReactNode;
  /** Banner above the toolbar — used for per-tab caveats and placeholders. */
  notice?: ReactNode;
}

/**
 * Shared chrome for every Container Services tab, so the six read as one
 * system: toolbar (free-text filter → facets → refresh → one primary action),
 * dense table, and a detail panel for the selected row. Tabs supply their own
 * columns, rows and actions; everything else — search state, selection, the
 * role-naming empty state, the error banner — lives here.
 */
export default function ContainerTabLayout<T extends object>({
  objectLabel,
  columns,
  rows,
  rowKey,
  loading = false,
  error,
  onRefresh,
  newLabel,
  onNew,
  facets,
  filter,
  detail,
  notice,
}: Props<T>) {
  const [search, setSearch] = useState("");
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const role = useSessionStore((s) => s.role);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q || !filter) return rows;
    return rows.filter((r) => filter(r, q));
  }, [rows, search, filter]);

  const selected = filtered.find((r) => String(r[rowKey]) === selectedKey);

  return (
    <>
      {notice}

      <Space style={{ marginBottom: 8 }} wrap size={[8, 8]}>
        <Input
          size="small" allowClear prefix={<SearchOutlined />} placeholder="Search…"
          style={{ width: 170 }} value={search} onChange={(e) => setSearch(e.target.value)}
        />
        {facets}
        <Button size="small" icon={<ReloadOutlined />} onClick={onRefresh} loading={loading}>
          Refresh
        </Button>
        {newLabel && (
          <Button size="small" type="primary" icon={<PlusOutlined />} onClick={onNew} disabled={!onNew}>
            {newLabel}
          </Button>
        )}
      </Space>

      <div style={{ marginBottom: 8 }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {filtered.length} of {rows.length} {objectLabel}
        </Text>
      </div>

      {error && (
        <Alert
          type="warning" showIcon style={{ marginBottom: 8 }}
          message={`Could not load ${objectLabel}`} description={error}
        />
      )}

      <Table<T>
        size="small" rowKey={rowKey} loading={loading}
        columns={columns} dataSource={filtered}
        pagination={{ pageSize: 12, showSizeChanger: false, size: "small" }}
        scroll={{ y: 300 }}
        onRow={(row) => ({ onClick: () => setSelectedKey(String(row[rowKey])) })}
        rowClassName={(row) => (String(row[rowKey]) === selectedKey ? "ant-table-row-selected" : "")}
        locale={{
          emptyText: (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={
                // Names the role and the fix — an empty list here almost always
                // means "the active role can't see them", not "there are none".
                `No ${objectLabel} are visible to ${role || "the current role"}. Create one or switch role.`
              }
            />
          ),
        }}
      />

      {detail && selected && <div style={{ marginTop: 12 }}>{detail(selected)}</div>}
    </>
  );
}
