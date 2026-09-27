// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Alert, Button, Space, Table, Typography } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { SECTION_HEAD } from "./PropertyRows";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

/** Build antd Table columns/data from a raw QueryResult, one column per result column. */
export function tableFromResult(res: snowflake.QueryResult | null) {
  const columns = (res?.columns ?? []).map((col, idx) => ({
    title: col,
    dataIndex: String(idx),
    key: String(idx),
    ellipsis: true,
    render: (v: unknown) => (
      <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v == null ? "" : String(v)}</span>
    ),
  }));
  const data = (res?.rows ?? []).map((row, ri) => {
    const obj: Record<string, unknown> = { key: ri };
    row.forEach((cell, ci) => { obj[String(ci)] = cell; });
    return obj;
  });
  return { columns, data };
}

/**
 * A lazily loaded properties-modal section rendering a raw QueryResult as an
 * antd table: a "Load <noun>s" button until first load, then a count + Refresh.
 */
export default function LazyResultTable({ title, noun, load }: {
  title: string;
  noun: string;
  load: () => Promise<snowflake.QueryResult>;
}) {
  const [res, setRes] = useState<snowflake.QueryResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async () => {
    setLoading(true);
    setError(null);
    try {
      setRes(await load() ?? null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  const t = tableFromResult(res);
  return (
    <>
      <div style={SECTION_HEAD}>{title}</div>
      {error && (
        <Alert type="error" message={`Failed to load ${noun}s`} description={error} showIcon style={{ marginBottom: 8 }} />
      )}
      {res ? (
        <>
          <Space style={{ marginBottom: 8 }}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t.data.length === 0 ? `No ${noun}s.` : `${t.data.length} ${noun}${t.data.length === 1 ? "" : "s"}.`}
            </Text>
            <Button size="small" icon={<ReloadOutlined />} onClick={run} loading={loading}>Refresh</Button>
          </Space>
          {t.data.length > 0 && (
            <Table size="small" columns={t.columns} dataSource={t.data} pagination={false} scroll={{ x: true }} />
          )}
        </>
      ) : (
        <Button size="small" icon={<ReloadOutlined />} onClick={run} loading={loading}>Load {noun}s</Button>
      )}
    </>
  );
}
