// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useState } from "react";
import { Button, Checkbox, Form, Input, InputNumber, Select } from "antd";
import { ClusterOutlined } from "@ant-design/icons";
import { BuildCreateComputePoolSql, ExecDDL } from "../../../wailsjs/go/app/App";
import { computepool } from "../../../wailsjs/go/models";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import TagInput, { type TagItem } from "../shared/TagInput";
import { useCreateSubmit, useSqlPreview } from "../shared/createModalHooks";
import { familyOptions, type ResultRow } from "./computePools";

interface Props {
  families: ResultRow[];
  familiesLoading: boolean;
  onShowFamilies: () => void;
  onClose: () => void;
  onSuccess: () => void;
}

const BOOL_OPTS = [{ value: "TRUE", label: "TRUE" }, { value: "FALSE", label: "FALSE" }];
const grid2: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" };
const item: React.CSSProperties = { marginBottom: 12 };

export default function CreateComputePoolModal({ families, familiesLoading, onShowFamilies, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState({
    name: "", ifNotExists: false, forApplication: "",
    minNodes: 1, maxNodes: 1, instanceFamily: "",
    autoResume: "", initiallySuspended: "", autoSuspendSecs: "",
    comment: "", placementGroup: "", backupInstanceFamilies: [] as string[],
  });
  const [tags, setTags] = useState<TagItem[]>([]);
  const set = <K extends keyof typeof cfg>(k: K, v: (typeof cfg)[K]) => setCfg((p) => ({ ...p, [k]: v }));

  const preview = useSqlPreview(
    () => BuildCreateComputePoolSql(computepool.ComputePoolConfig.createFrom({ ...cfg, tags })),
    [cfg, tags],
    { blankOnError: true },
  );
  const { creating, error, setError, submit } = useCreateSubmit();
  const canSubmit = !!cfg.name.trim() && !!cfg.instanceFamily && cfg.maxNodes >= cfg.minNodes && !!preview;
  const opts = familyOptions(families);

  return (
    <CreateModalShell
      icon={<ClusterOutlined />} title="Create Compute Pool" width={720}
      error={error} errorTitle="Compute pool creation failed" onErrorClose={() => setError(null)}
      creating={creating} canSubmit={canSubmit} onClose={onClose}
      onSubmit={() => submit(async () => { await ExecDDL(preview); onSuccess(); onClose(); })}
    >
      <Form layout="vertical" size="small">
        <div style={{ ...grid2, gridTemplateColumns: "1fr auto", alignItems: "end" }}>
          <Form.Item label="Pool name" required style={item}>
            <Input value={cfg.name} onChange={(e) => set("name", e.target.value)} placeholder="MY_POOL" />
          </Form.Item>
          <Form.Item style={item}>
            <Checkbox checked={cfg.ifNotExists} onChange={(e) => set("ifNotExists", e.target.checked)}>IF NOT EXISTS</Checkbox>
          </Form.Item>
        </div>

        <Form.Item
          label="Instance family" required style={item}
          extra={<Button type="link" size="small" style={{ padding: 0 }} onClick={onShowFamilies}>Compare instance families…</Button>}
        >
          <Select
            showSearch value={cfg.instanceFamily || undefined} onChange={(v) => setCfg((p) => ({
              ...p, instanceFamily: v ?? "",
              // A family can't be its own backup — drop it from the backup list.
              backupInstanceFamilies: p.backupInstanceFamilies.filter((b) => b !== v),
            }))}
            options={opts} loading={familiesLoading} placeholder="Select an instance family"
            notFoundContent={familiesLoading ? "Loading…" : "No instance families visible"}
          />
        </Form.Item>

        <div style={grid2}>
          <Form.Item label="Min nodes" required style={item}>
            <InputNumber min={1} value={cfg.minNodes} onChange={(v) => set("minNodes", v ?? 1)} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item label="Max nodes" required style={item}
            validateStatus={cfg.maxNodes < cfg.minNodes ? "error" : undefined}
            help={cfg.maxNodes < cfg.minNodes ? "Must be ≥ min nodes" : undefined}>
            <InputNumber min={1} value={cfg.maxNodes} onChange={(v) => set("maxNodes", v ?? 1)} style={{ width: "100%" }} />
          </Form.Item>
        </div>

        <Form.Item label="Backup instance families" style={item} help="Used, in order, when the primary family has no capacity.">
          <Select
            mode="multiple" value={cfg.backupInstanceFamilies} onChange={(v) => set("backupInstanceFamilies", v)}
            options={opts.filter((o) => o.value !== cfg.instanceFamily)} placeholder="(none)"
          />
        </Form.Item>

        <div style={{ ...grid2, gridTemplateColumns: "1fr 1fr 1fr" }}>
          <Form.Item label="Auto resume" style={item}>
            <Select allowClear value={cfg.autoResume || undefined} onChange={(v) => set("autoResume", v ?? "")} options={BOOL_OPTS} placeholder="Default (TRUE)" />
          </Form.Item>
          <Form.Item label="Initially suspended" style={item}>
            <Select allowClear value={cfg.initiallySuspended || undefined} onChange={(v) => set("initiallySuspended", v ?? "")} options={BOOL_OPTS} placeholder="Default (FALSE)" />
          </Form.Item>
          <Form.Item label="Auto suspend (secs)" style={item}>
            <InputNumber
              min={0} value={cfg.autoSuspendSecs === "" ? undefined : Number(cfg.autoSuspendSecs)}
              onChange={(v) => set("autoSuspendSecs", v == null ? "" : String(v))} placeholder="3600" style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <div style={grid2}>
          <Form.Item label="For application" style={item} help="Native App that owns the pool (optional).">
            <Input value={cfg.forApplication} onChange={(e) => set("forApplication", e.target.value)} />
          </Form.Item>
          <Form.Item label="Placement group" style={item}>
            <Input value={cfg.placementGroup} onChange={(e) => set("placementGroup", e.target.value)} />
          </Form.Item>
        </div>

        <Form.Item label="Comment" style={item}>
          <Input value={cfg.comment} onChange={(e) => set("comment", e.target.value)} />
        </Form.Item>

        <TagInput tags={tags} onChange={setTags} itemStyle={item} />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
