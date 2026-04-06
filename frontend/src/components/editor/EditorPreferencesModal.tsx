import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Col,
  Divider,
  Form,
  Modal,
  Radio,
  Row,
  Segmented,
  Space,
  Typography,
} from "antd";
import { GetEditorPrefs, SaveEditorPrefs } from "../../../wailsjs/go/main/App";
import { DEFAULT_EDITOR_PREFS, EditorPrefs, formatSQL } from "../../utils/sqlFormatter";

const { Text } = Typography;

// ── Sample SQL shown in the live preview ─────────────────────────────────────
const SAMPLE_SQL = `with orders_cte as (
  select order_id, customer_id, total_amount::decimal(18,2) as amount,
    src:status::string as status, to_date(created_at) as order_date
  from raw.orders
  where status ilike 'active' and total_amount > 0
    and customer_id in (select id from customers where region = 'EU')
), ranked as (
  select *, row_number() over (partition by customer_id order by order_date desc) as rn
  from orders_cte
)
select customer_id, amount, status, order_date
from ranked
where rn = 1
order by amount desc nulls last`;

// ── Helpers ───────────────────────────────────────────────────────────────────

function Section({ title }: { title: string }) {
  return (
    <Divider orientation="left" style={{ fontSize: 12, color: "var(--text-muted)", margin: "20px 0 8px" }}>
      {title}
    </Divider>
  );
}

function radioOpts(opts: { label: string; value: string }[]) {
  return opts.map((o) => (
    <Radio key={o.value} value={o.value}>
      <Text style={{ fontSize: 12 }}>{o.label}</Text>
    </Radio>
  ));
}

// ── Component ─────────────────────────────────────────────────────────────────

interface Props {
  onClose: () => void;
}

export default function EditorPreferencesModal({ onClose }: Props) {
  const [prefs, setPrefs]       = useState<EditorPrefs>(DEFAULT_EDITOR_PREFS);
  const [saving, setSaving]     = useState(false);
  const [error, setError]       = useState<string | null>(null);
  const [preview, setPreview]   = useState("");

  // Load persisted prefs on mount.
  useEffect(() => {
    GetEditorPrefs().then((p) => setPrefs(p as unknown as EditorPrefs)).catch(() => {});
  }, []);

  // Recompute preview whenever prefs change.
  useEffect(() => {
    void formatSQL(SAMPLE_SQL, prefs).then(setPreview);
  }, [prefs]);

  function set<K extends keyof EditorPrefs>(key: K, value: EditorPrefs[K]) {
    setPrefs((p) => ({ ...p, [key]: value }));
  }

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      await SaveEditorPrefs(prefs);
      // Notify the editor so it can refresh its prefs reference.
      window.dispatchEvent(new CustomEvent("thaw:editor-prefs-changed", { detail: prefs }));
      onClose();
    } catch (e: unknown) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open
      title="Editor Preferences"
      onCancel={onClose}
      width={860}
      styles={{ body: { paddingTop: 4, maxHeight: "78vh", overflowY: "auto" } }}
      footer={
        <Space>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" loading={saving} onClick={handleSave}>
            Save
          </Button>
        </Space>
      }
    >
      {error && (
        <Alert type="error" message={error} closable onClose={() => setError(null)} style={{ marginBottom: 12 }} />
      )}

      <Row gutter={24}>
        {/* ── Left: controls ───────────────────────────────────────────── */}
        <Col span={12}>
          {/* Casing */}
          <Section title="Casing" />

          <Form layout="vertical" size="small">
            <Form.Item
              label={<Text style={{ fontSize: 12 }}>Keyword casing</Text>}
              help={<Text type="secondary" style={{ fontSize: 11 }}>SELECT, FROM, WHERE, WINDOW, QUALIFY…</Text>}
              style={{ marginBottom: 16 }}
            >
              <Segmented
                size="small"
                value={prefs.keywordCase}
                onChange={(v) => set("keywordCase", v as EditorPrefs["keywordCase"])}
                options={[
                  { label: "UPPER", value: "UPPER" },
                  { label: "lower", value: "lower" },
                  { label: "Title", value: "Title" },
                  { label: "Preserve", value: "Preserve" },
                ]}
              />
            </Form.Item>

            <Form.Item
              label={<Text style={{ fontSize: 12 }}>Identifier casing</Text>}
              help={<Text type="secondary" style={{ fontSize: 11 }}>Unquoted table / column names. Double-quoted identifiers are never modified.</Text>}
              style={{ marginBottom: 16 }}
            >
              <Segmented
                size="small"
                value={prefs.identifierCase}
                onChange={(v) => set("identifierCase", v as EditorPrefs["identifierCase"])}
                options={[
                  { label: "Preserve", value: "Preserve" },
                  { label: "UPPER",    value: "UPPER"    },
                  { label: "lower",    value: "lower"    },
                ]}
              />
            </Form.Item>

            <Form.Item
              label={<Text style={{ fontSize: 12 }}>Function casing</Text>}
              help={<Text type="secondary" style={{ fontSize: 11 }}>TO_DATE(), AVG(), IFF(), and any other function call.</Text>}
              style={{ marginBottom: 0 }}
            >
              <Segmented
                size="small"
                value={prefs.functionCase}
                onChange={(v) => set("functionCase", v as EditorPrefs["functionCase"])}
                options={[
                  { label: "UPPER", value: "UPPER" },
                  { label: "lower", value: "lower" },
                ]}
              />
            </Form.Item>
          </Form>

          {/* Indentation */}
          <Section title="Indentation" />

          <Form layout="vertical" size="small">
            <Form.Item
              label={<Text style={{ fontSize: 12 }}>Indent style</Text>}
              style={{ marginBottom: 16 }}
            >
              <Radio.Group
                value={prefs.indentStyle}
                onChange={(e) => set("indentStyle", e.target.value)}
              >
                {radioOpts([
                  { label: "Spaces", value: "spaces" },
                  { label: "Tabs",   value: "tabs"   },
                ])}
              </Radio.Group>
            </Form.Item>

            <Form.Item
              label={<Text style={{ fontSize: 12 }}>Indent size</Text>}
              help={<Text type="secondary" style={{ fontSize: 11 }}>Snowflake SQL often nests deeply — 2 is recommended.</Text>}
              style={{ marginBottom: 0 }}
            >
              <Radio.Group
                value={String(prefs.indentSize)}
                onChange={(e) => set("indentSize", Number(e.target.value) as 2 | 4)}
                disabled={prefs.indentStyle === "tabs"}
              >
                {radioOpts([
                  { label: "2 spaces", value: "2" },
                  { label: "4 spaces", value: "4" },
                ])}
              </Radio.Group>
            </Form.Item>
          </Form>

          {/* Structure */}
          <Section title="Structure" />

          <Form layout="vertical" size="small">
            <Form.Item
              label={<Text style={{ fontSize: 12 }}>Comma position</Text>}
              help={
                <Text type="secondary" style={{ fontSize: 11 }}>
                  Trailing: <code>col1, col2</code> — Leading: <code>, col1</code>
                </Text>
              }
              style={{ marginBottom: 16 }}
            >
              <Radio.Group
                value={prefs.commaPosition}
                onChange={(e) => set("commaPosition", e.target.value)}
              >
                {radioOpts([
                  { label: "Trailing (end of line)", value: "trailing" },
                  { label: "Leading (start of line)", value: "leading" },
                ])}
              </Radio.Group>
            </Form.Item>

            <Form.Item
              label={<Text style={{ fontSize: 12 }}>AND / OR position</Text>}
              help={
                <Text type="secondary" style={{ fontSize: 11 }}>
                  Before: AND/OR at start of new line — After: AND/OR at end of line
                </Text>
              }
              style={{ marginBottom: 0 }}
            >
              <Radio.Group
                value={prefs.operatorPosition}
                onChange={(e) => set("operatorPosition", e.target.value)}
              >
                {radioOpts([
                  { label: "Before new line", value: "before" },
                  { label: "After new line",  value: "after"  },
                ])}
              </Radio.Group>
            </Form.Item>
          </Form>

          {/* Snowflake notes */}
          <Section title="Snowflake dialect (always applied)" />
          <Space direction="vertical" size={4} style={{ fontSize: 11, color: "var(--text-muted)" }}>
            <Text type="secondary" style={{ fontSize: 11 }}>• No whitespace around <code>::</code> (type cast) and <code>:</code> (VARIANT path)</Text>
            <Text type="secondary" style={{ fontSize: 11 }}>• WITH placed on its own line; CTE body indented</Text>
            <Text type="secondary" style={{ fontSize: 11 }}>• LATERAL FLATTEN block treated as a single unit</Text>
          </Space>
        </Col>

        {/* ── Right: live preview ───────────────────────────────────────── */}
        <Col span={12}>
          <Section title="Preview" />
          <pre
            style={{
              background: "var(--bg-subtle, #1e1e1e)",
              border: "1px solid var(--border-color, #333)",
              borderRadius: 6,
              padding: "10px 12px",
              fontSize: 11,
              lineHeight: 1.55,
              fontFamily: "var(--font-mono, 'Menlo', 'Consolas', monospace)",
              whiteSpace: "pre-wrap",
              overflowY: "auto",
              maxHeight: "62vh",
              margin: 0,
              color: "var(--text-primary)",
            }}
          >
            {preview}
          </pre>
        </Col>
      </Row>
    </Modal>
  );
}
