// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useMemo } from "react";
import {
  Modal,
  Select,
  DatePicker,
  Button,
  Table,
  Spin,
  Alert,
  Statistic,
  Tooltip as AntTooltip,
} from "antd";
import { CompressOutlined, ExpandOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import type { Dayjs } from "dayjs";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import { GetWarehouseMeteringHistory } from "../../../wailsjs/go/main/App";
import { useSessionStore } from "../../store/sessionStore";
import type { main } from "../../../wailsjs/go/models";

const { RangePicker } = DatePicker;

interface Props {
  onClose: () => void;
}

interface DailyEntry {
  date: string;
  compute: number;
  cloud: number;
}

export default function WarehouseMeteringModal({ onClose }: Props) {
  const warehouses     = useSessionStore((s) => s.warehouses);
  const loadWarehouses = useSessionStore((s) => s.loadWarehouses);

  const [warehouse, setWarehouse] = useState("");
  const [dateRange, setDateRange] = useState<[Dayjs, Dayjs]>([
    dayjs().subtract(30, "day"),
    dayjs(),
  ]);
  const [rows,       setRows]       = useState<main.WarehouseMeteringRow[] | null>(null);
  const [loading,    setLoading]    = useState(false);
  const [error,      setError]      = useState<string | null>(null);
  const [tableCollapsed, setTableCollapsed] = useState(false);

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await GetWarehouseMeteringHistory(
        warehouse,
        dateRange[0].startOf("day").toISOString(),
        dateRange[1].endOf("day").toISOString(),
      );
      setRows(data ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { loadWarehouses(); fetchData(); }, []);

  const { dailyData, totalCredits, totalCompute, totalCloud } = useMemo(() => {
    if (!rows) return { dailyData: [], totalCredits: 0, totalCompute: 0, totalCloud: 0 };

    const byDate: Record<string, DailyEntry> = {};
    let sumCredits = 0, sumCompute = 0, sumCloud = 0;

    for (const r of rows) {
      const date = r.startTime ? r.startTime.slice(0, 10) : "unknown";
      if (!byDate[date]) byDate[date] = { date, compute: 0, cloud: 0 };
      byDate[date].compute += r.creditsUsedCompute;
      byDate[date].cloud   += r.creditsUsedCloudServices;
      sumCredits += r.creditsUsed;
      sumCompute += r.creditsUsedCompute;
      sumCloud   += r.creditsUsedCloudServices;
    }

    return {
      dailyData:    Object.values(byDate).sort((a, b) => a.date.localeCompare(b.date)),
      totalCredits: sumCredits,
      totalCompute: sumCompute,
      totalCloud:   sumCloud,
    };
  }, [rows]);

  const columns: ColumnsType<main.WarehouseMeteringRow> = [
    {
      key: "startTime",
      title: "Start Time",
      dataIndex: "startTime",
      width: 160,
      render: (v: string) => v ? dayjs(v).format("YYYY-MM-DD HH:mm") : "—",
    },
    {
      key: "warehouseName",
      title: "Warehouse",
      dataIndex: "warehouseName",
      width: 150,
    },
    {
      key: "creditsUsed",
      title: "Total Credits",
      dataIndex: "creditsUsed",
      width: 120,
      align: "right",
      render: (v: number) => v.toFixed(4),
    },
    {
      key: "creditsUsedCompute",
      title: "Compute Credits",
      dataIndex: "creditsUsedCompute",
      width: 130,
      align: "right",
      render: (v: number) => v.toFixed(4),
    },
    {
      key: "creditsUsedCloudServices",
      title: "Cloud Svc Credits",
      dataIndex: "creditsUsedCloudServices",
      width: 140,
      align: "right",
      render: (v: number) => v.toFixed(4),
    },
  ];

  return (
    <Modal
      open
      title="Warehouse Credit Usage"
      onCancel={onClose}
      width={980}
      footer={null}
    >
      {/* Filter bar */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "flex-end", marginBottom: 12 }}>
        <div>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Warehouse</div>
          <Select
            size="small"
            value={warehouse}
            onChange={setWarehouse}
            style={{ width: 200 }}
            options={[
              { value: "", label: "All warehouses" },
              ...warehouses.map((w) => ({ value: w, label: w })),
            ]}
            onDropdownVisibleChange={(open) => { if (open) loadWarehouses(); }}
          />
        </div>
        <div>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Date range</div>
          <RangePicker
            size="small"
            style={{ width: 260 }}
            value={dateRange}
            onChange={(v) => { if (v?.[0] && v?.[1]) setDateRange([v[0], v[1]]); }}
          />
        </div>
        <Button type="primary" size="small" onClick={fetchData} loading={loading}>
          Apply
        </Button>
      </div>

      {loading && <div style={{ textAlign: "center", padding: 24 }}><Spin /></div>}
      {error   && <Alert type="error" message={error} style={{ marginBottom: 8 }} />}

      {rows && !loading && (
        <>
          {/* Summary cards */}
          <div style={{ display: "flex", gap: 24, marginBottom: 16 }}>
            <Statistic
              title="Total Credits"
              value={totalCredits}
              precision={4}
              style={{ minWidth: 140 }}
            />
            <Statistic
              title="Compute Credits"
              value={totalCompute}
              precision={4}
              style={{ minWidth: 140 }}
            />
            <Statistic
              title="Cloud Services Credits"
              value={totalCloud}
              precision={4}
              style={{ minWidth: 160 }}
            />
          </div>

          {/* Daily stacked bar chart */}
          {dailyData.length > 0 && (
            <div style={{ marginBottom: 16 }}>
              <ResponsiveContainer width="100%" height={240}>
                <BarChart data={dailyData} margin={{ top: 4, right: 16, bottom: 4, left: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                  <XAxis dataKey="date" tick={{ fontSize: 11, fill: "var(--text-muted)" }} />
                  <YAxis tick={{ fontSize: 11, fill: "var(--text-muted)" }} />
                  <Tooltip
                    contentStyle={{
                      background: "var(--bg-overlay)",
                      border: "1px solid var(--border)",
                      fontSize: 12,
                    }}
                  />
                  <Legend wrapperStyle={{ fontSize: 12 }} />
                  <Bar dataKey="compute" name="Compute"        stackId="a" fill="#1677ff" />
                  <Bar dataKey="cloud"   name="Cloud Services" stackId="a" fill="#fa8c16" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}

          {/* Hourly detail table */}
          <div>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4 }}>
              <span style={{ fontSize: 11, color: "var(--text-muted)", fontWeight: 500 }}>
                Hourly detail ({rows.length} row{rows.length !== 1 ? "s" : ""})
              </span>
              <AntTooltip title={tableCollapsed ? "Expand table" : "Collapse table"}>
                <Button
                  size="small"
                  type="text"
                  icon={tableCollapsed ? <ExpandOutlined /> : <CompressOutlined />}
                  onClick={() => setTableCollapsed((v) => !v)}
                  style={{ height: 20, padding: "0 4px" }}
                />
              </AntTooltip>
            </div>
            {!tableCollapsed && (
              <Table<main.WarehouseMeteringRow>
                dataSource={rows}
                columns={columns}
                rowKey={(r) => `${r.startTime}-${r.warehouseName}`}
                size="small"
                scroll={{ x: true }}
                pagination={{ pageSize: 20, showSizeChanger: false }}
              />
            )}
          </div>
        </>
      )}
    </Modal>
  );
}
