// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import {
  Form, Input, Select, Typography, Button, Collapse,
} from "antd";
import { AlertOutlined, PlusOutlined } from "@ant-design/icons";
import {
  BuildCreateAlertSql, ExecDDL, ListWarehouses,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import TagInput from "../shared/TagInput";
import MonacoSqlField from "../shared/MonacoSqlField";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { alert as alertModels } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import CallProcedureModal from "../procedure/CallProcedureModal";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_CONDITION = "SELECT *\n  FROM my_source_table\n  WHERE my_metric > 100";
const DEFAULT_ACTION = "INSERT INTO my_alert_log\n  SELECT CURRENT_TIMESTAMP()";

// Plain data shape for form state. The Wails-generated `AlertConfig` class
// carries a `convertValues` method (it has a nested `tags` array), which a plain
// object literal can't satisfy; we cast to the generated type only at the IPC
// boundary (`cfg as any`).
type AlertCfg = Omit<alertModels.AlertConfig, "convertValues" | "tags"> & {
  tags: { name: string; value: string }[];
};

export default function CreateAlertModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<AlertCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    warehouse: "",
    schedule: "60 MINUTE",
    comment: "",
    tags: [],
    condition: DEFAULT_CONDITION,
    action: DEFAULT_ACTION,
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateAlertSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const [warehouseOptions, setWarehouseOptions] = useState<string[]>([]);

  // "Insert CALL procedure" picker — shares the database/schema selection with
  // the table picker; the selected option carries the procedure's overload
  // signature so the reused CallProcedureModal can resolve the correct overload.
  const [pickerProcIdx, setPickerProcIdx] = useState<string>("");
  const [callModal, setCallModal] = useState<{ db: string; schema: string; name: string; rawArgs: string } | null>(null);

  useEffect(() => {
    ListWarehouses().then((w) => setWarehouseOptions(w ?? [])).catch(() => {});
  }, []);

  const set = <K extends keyof AlertCfg>(key: K, value: AlertCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // The editors seed placeholder templates; treat the untouched placeholders as
  // "not ready" so Create can't fire a statement that references the obviously
  // fake objects and fails server-side.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.schedule.trim().length > 0 &&
    cfg.condition.trim().length > 0 &&
    cfg.condition.trim() !== DEFAULT_CONDITION.trim() &&
    cfg.action.trim().length > 0 &&
    cfg.action.trim() !== DEFAULT_ACTION.trim();

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<AlertOutlined />}
      title="Create Alert"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Alert creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Alert name"
          placeholder="MY_ALERT"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
        />

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item
            label="Schedule"
            required
            style={itemStyle}
            help="e.g. 60 MINUTE or USING CRON 0 9 * * * UTC"
          >
            <Input
              value={cfg.schedule}
              onChange={(e) => set("schedule", e.target.value)}
              placeholder="60 MINUTE"
            />
          </Form.Item>
          <Form.Item
            label="Warehouse"
            style={itemStyle}
            help="Leave empty for a serverless alert"
          >
            <Select
              showSearch
              allowClear
              placeholder="(serverless)"
              value={cfg.warehouse || undefined}
              onChange={(v) => set("warehouse", v ?? "")}
              options={warehouseOptions.map((n) => ({ value: n, label: n }))}
            />
          </Form.Item>
        </div>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[{
            key: "advanced",
            label: "Advanced options",
            children: (
              <TagInput
                tags={cfg.tags}
                onChange={(tags) => set("tags", tags)}
                help="Alert-level tags applied at creation"
              />
            ),
          }]}
        />

        <MonacoSqlField
          label="Condition — IF (EXISTS (…))"
          required
          help="The alert fires when this query returns at least one row"
          value={cfg.condition}
          onChange={(v) => set("condition", v)}
          placeholder={DEFAULT_CONDITION}
          height={120}
          objectKinds={["TABLE", "VIEW"]}
          defaultDb={db}
          defaultSchema={schema}
          notFoundText="No tables or views"
          // Clear the procedure selection when the source db/schema changes, so a
          // stale index can't point at a different procedure (or out of range).
          onSourceChange={() => setPickerProcIdx("")}
          extraPickerRow={({ db: pickerDb, schema: pickerSchema, objects, loading, insert }) => {
            const procOptions = objects
              .filter((o) => o.kind === "PROCEDURE")
              .map((o) => ({ name: o.name, args: o.arguments ?? "" }));
            return (
              <>
                <div style={{ display: "flex", gap: 8, marginBottom: 8, flexWrap: "wrap", alignItems: "center" }}>
                  <Text type="secondary" style={{ fontSize: 11 }}>Insert CALL procedure:</Text>
                  <Select
                    size="small"
                    showSearch
                    placeholder="Procedure"
                    style={{ width: 280 }}
                    value={pickerProcIdx || undefined}
                    onChange={(v) => setPickerProcIdx(v ?? "")}
                    disabled={!pickerSchema}
                    loading={loading}
                    options={procOptions.map((p, i) => ({
                      value: String(i),
                      label: p.args ? `${p.name}(${p.args})` : `${p.name}()`,
                    }))}
                    notFoundContent={loading ? "Loading…" : "No procedures"}
                  />
                  <Button
                    size="small"
                    icon={<PlusOutlined />}
                    onClick={() => {
                      if (pickerProcIdx === "") return;
                      const proc = procOptions[Number(pickerProcIdx)];
                      if (!proc) return;
                      setCallModal({ db: pickerDb, schema: pickerSchema, name: proc.name, rawArgs: proc.args });
                    }}
                    disabled={pickerProcIdx === ""}
                  >
                    Insert CALL…
                  </Button>
                  <Text type="secondary" style={{ fontSize: 10 }}>
                    (uses the same database / schema selected above)
                  </Text>
                </div>
                {callModal && (
                  <CallProcedureModal
                    db={callModal.db}
                    schema={callModal.schema}
                    name={callModal.name}
                    rawArgs={callModal.rawArgs}
                    onClose={() => setCallModal(null)}
                    onInsert={(sql) => {
                      // The CALL goes inside IF (EXISTS (…)); strip the statement
                      // terminator the builder appends so it reads naturally here.
                      insert(sql.trim().replace(/;\s*$/, ""), "insert-call");
                    }}
                  />
                )}
              </>
            );
          }}
        />

        <Form.Item label="Action — THEN" required style={itemStyle} help="The statement executed when the condition is met">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={100}
              language="sql"
              theme={editorTheme}
              value={cfg.action}
              onChange={(v) => set("action", v ?? "")}
              onMount={(editor) => { patchMonacoClipboard(editor); }}
              options={{
                minimap: { enabled: false },
                lineNumbers: "off",
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
              }}
            />
          </div>
        </Form.Item>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
