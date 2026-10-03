// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Alert, Segmented, Space, Typography } from "antd";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { dumpSpec, parseSpec, setIn, specProblems, type SpecKind } from "./specDoc";
import { SpecCtx } from "./specFields";

interface Props {
  kind: SpecKind;
  /** The specification YAML — the one value parents keep and hand to the SQL builders. */
  value: string;
  onChange: (text: string) => void;
  /** A Jinja template: always YAML, no schema (it isn't valid YAML until rendered). */
  template?: boolean;
  height?: number;
  /** The structured form (fields bound through SpecCtx). */
  children: ReactNode;
}

/**
 * Form | YAML specification editor shared by the SPCS dialogs (issue #958).
 * The YAML text stays the source of truth: Form mode edits its parsed
 * document and re-serialises; YAML mode is Monaco with the kind's JSON schema
 * (model path `<kind>.spcs`, see editor/monacoSetup.ts). Text that doesn't
 * parse as a plain mapping (templates, syntax errors) stays in YAML mode.
 */
export default function SpecEditor({ kind, value, onChange, template, height = 320, children }: Props) {
  const editorTheme = useThemeStore((s) => s.resolved) === "dark" ? "vs-dark" : "vs";
  const parsed = useMemo(() => parseSpec(value), [value]);
  const formable = !template && "doc" in parsed;
  const [mode, setMode] = useState<"form" | "yaml">(formable ? "form" : "yaml");
  const [notice, setNotice] = useState<string | null>(null);
  const reason = template ? "Templates are edited as YAML." : "error" in parsed ? parsed.error : null;

  // The text can stop fitting the form underneath us (Template ticked, a spec
  // loaded that doesn't parse) — fall back to YAML and say why.
  useEffect(() => {
    if (mode === "form" && !formable) { setMode("yaml"); setNotice(reason); }
  }, [mode, formable, reason]);

  const switchTo = (m: "form" | "yaml") => {
    if (m === "form" && !formable) { setNotice(reason); return; }
    setNotice(null);
    setMode(m);
  };

  const problems = useMemo(() => (template ? [] : specProblems(kind, value)), [kind, value, template]);
  const doc = "doc" in parsed ? parsed.doc : {};
  const ctx = {
    doc,
    set: (p: (string | number)[], v: unknown) => onChange(dumpSpec(setIn(doc, p, v))),
    edit: (fn: (d: typeof doc) => typeof doc) => onChange(dumpSpec(fn(doc))),
  };

  return (
    <div>
      <Space size={8} style={{ marginBottom: 6 }}>
        <Segmented size="small" value={mode} onChange={(m) => switchTo(m as "form" | "yaml")}
          options={[{ value: "form", label: "Form", disabled: !!template }, { value: "yaml", label: "YAML" }]} />
        {notice && <Typography.Text type="secondary" style={{ fontSize: 11 }}>{notice}</Typography.Text>}
      </Space>
      {mode === "form" ? (
        <SpecCtx.Provider value={ctx}>{children}</SpecCtx.Provider>
      ) : (
        <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
          <Editor
            height={height}
            language="yaml"
            path={`inmemory://spcs/${template ? "template" : kind}.spcs`}
            theme={editorTheme}
            value={value}
            onChange={(v) => onChange(v ?? "")}
            onMount={(editor) => patchMonacoClipboard(editor)}
            options={{ minimap: { enabled: false }, scrollBeyondLastLine: false, fontSize: 12, wordWrap: "on", automaticLayout: true }}
          />
        </div>
      )}
      {problems.length > 0 && (
        <Alert type="warning" showIcon style={{ marginTop: 8 }} message="Fix before submitting"
          description={<ul style={{ margin: 0, paddingLeft: 18 }}>{problems.map((p) => <li key={p}>{p}</li>)}</ul>} />
      )}
    </div>
  );
}
