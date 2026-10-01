// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useEffect, useMemo, useState } from "react";
import { Checkbox, Form, Input, Select } from "antd";
import { CameraOutlined } from "@ant-design/icons";
import {
  BuildCreateSnapshotSql, ExecDDL, ListServicesInAccount, ListServiceVolumes,
} from "../../../wailsjs/go/app/App";
import { snapshot } from "../../../wailsjs/go/models";
import { friendlyError } from "../common/errors";
import CreateModalShell from "../shared/CreateModalShell";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import SqlPreview from "../shared/SqlPreview";
import TagInput, { type TagItem } from "../shared/TagInput";
import { useCreateSubmit, useQuotedIdentifiers, useSqlPreview } from "../shared/createModalHooks";
import { rowsOf } from "../containerservices/computePools";
import { blockVolumes } from "./blockVolumes";

export interface ServiceRef { db: string; schema: string; name: string }

interface Props {
  /** Where the snapshot is created. */
  db: string;
  schema: string;
  /** Pre-selects the source service (the Service properties Volumes section). */
  service?: ServiceRef;
  onClose: () => void;
  onSuccess?: () => void;
}

const item: React.CSSProperties = { marginBottom: 12 };
const svcKey = (s: ServiceRef) => JSON.stringify([s.db, s.schema, s.name]);

/** CREATE SNAPSHOT form: service → block volume → instance pickers, live SQL. */
export default function CreateSnapshotModal({ db, schema, service: initial, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState({ name: "", caseSensitive: false, orReplace: false, ifNotExists: false, comment: "" });
  const [tags, setTags] = useState<TagItem[]>([]);
  const [services, setServices] = useState<ServiceRef[]>(initial ? [initial] : []);
  const [service, setService] = useState<ServiceRef | undefined>(initial);
  const [volumes, setVolumes] = useState<Map<string, string[]>>(new Map());
  const [volumesLoading, setVolumesLoading] = useState(false);
  const [volumesError, setVolumesError] = useState<string | null>(null);
  const [volume, setVolume] = useState("");
  const [instance, setInstance] = useState("");
  const set = <K extends keyof typeof cfg>(k: K, v: (typeof cfg)[K]) => setCfg((p) => ({ ...p, [k]: v }));

  // Every service the role can see, with the target schema's own listed first.
  useEffect(() => {
    ListServicesInAccount().then((res) => {
      const all = rowsOf(res).map((r) => ({ db: r.database_name, schema: r.schema_name, name: r.name }));
      const local = (s: ServiceRef) => (s.db === db && s.schema === schema ? 0 : 1);
      setServices(all.sort((a, b) => local(a) - local(b)));
    }).catch(() => {});
  }, [db, schema]);

  useEffect(() => {
    setVolume("");
    setInstance("");
    setVolumes(new Map());
    setVolumesError(null);
    if (!service) return;
    let live = true;
    setVolumesLoading(true);
    ListServiceVolumes(service.db, service.schema, service.name)
      .then((res) => live && setVolumes(blockVolumes(rowsOf(res))))
      .catch((e) => live && setVolumesError(friendlyError(e)))
      .finally(() => live && setVolumesLoading(false));
    return () => { live = false; };
  }, [service]);

  const instances = useMemo(() => volumes.get(volume) ?? [], [volumes, volume]);
  useEffect(() => { if (instances.length === 1) setInstance(instances[0]); }, [instances]);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateSnapshotSql(db, schema, snapshot.SnapshotConfig.createFrom({
      ...cfg, tags, volume, instance,
      serviceDatabase: service?.db ?? "", serviceSchema: service?.schema ?? "", serviceName: service?.name ?? "",
    })),
    [db, schema, cfg, tags, service, volume, instance],
    { blankOnError: true },
  );
  const { creating, error, setError, submit } = useCreateSubmit();
  const canSubmit = !!cfg.name.trim() && !!service && !!volume && instance !== "" && !!preview;
  const opts = (xs: string[]) => xs.map((x) => ({ value: x, label: x }));

  return (
    <CreateModalShell
      icon={<CameraOutlined />} title="Create Snapshot" subtitle={`${db}.${schema}`} width={680}
      error={error} errorTitle="Snapshot creation failed" onErrorClose={() => setError(null)}
      creating={creating} canSubmit={canSubmit} onClose={onClose}
      onSubmit={() => submit(async () => { await ExecDDL(preview); onSuccess?.(); onClose(); })}
    >
      <Form layout="vertical" size="small">
        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive; selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Snapshot name" required style={{ marginBottom: 4 }}>
            <Input value={cfg.name} onChange={(e) => set("name", e.target.value)} placeholder="MY_SNAPSHOT" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox checked={cfg.orReplace}
              onChange={(e) => setCfg((p) => ({ ...p, orReplace: e.target.checked, ifNotExists: e.target.checked ? false : p.ifNotExists }))}>
              OR REPLACE
            </Checkbox>
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox checked={cfg.ifNotExists}
              onChange={(e) => setCfg((p) => ({ ...p, ifNotExists: e.target.checked, orReplace: e.target.checked ? false : p.orReplace }))}>
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>
        <Form.Item style={item}>
          <ObjectNameCaseControl name={cfg.name} caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)} quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase} />
        </Form.Item>

        <Form.Item label="Service" required style={item}>
          <Select showSearch placeholder="Service with a block-storage volume"
            value={service ? svcKey(service) : undefined}
            options={services.map((s) => ({ value: svcKey(s), label: `${s.db}.${s.schema}.${s.name}` }))}
            onChange={(k) => setService(services.find((s) => svcKey(s) === k))} />
        </Form.Item>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 160px", gap: "0 16px" }}>
          <Form.Item label="Volume" required style={item}
            validateStatus={volumesError ? "error" : undefined}
            help={volumesError ?? (service && !volumesLoading && volumes.size === 0 ? "This service has no block-storage volumes." : "Only block volumes can be snapshotted.")}>
            <Select value={volume || undefined} loading={volumesLoading} disabled={!service}
              options={opts([...volumes.keys()])} onChange={(v) => { setVolume(v); setInstance(""); }} />
          </Form.Item>
          <Form.Item label="Instance" required style={item}>
            <Select value={instance || undefined} disabled={!volume} options={opts(instances)} onChange={setInstance} />
          </Form.Item>
        </div>

        <Form.Item label="Comment" style={item}>
          <Input value={cfg.comment} onChange={(e) => set("comment", e.target.value)} placeholder="optional comment" />
        </Form.Item>
        <TagInput tags={tags} onChange={setTags} itemStyle={item} />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
