// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Button, Segmented, Space, Typography } from "antd";
import { CopyOutlined } from "@ant-design/icons";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { useConnectionStore } from "../../store/connectionStore";
import { registrySnippets, type LoginMethod } from "./registrySnippets";

const { Text } = Typography;

const METHODS: { value: LoginMethod; label: string }[] = [
  { value: "cli", label: "Snowflake CLI" },
  { value: "token", label: "CLI token pipe" },
  { value: "pat", label: "Docker with PAT" },
];
const HINTS: Record<LoginMethod, string> = {
  cli: "Works for every Snowflake auth type. Needs snow and Docker installed; add --connection <name> if not using the default.",
  token: "Same, and suits other clients or CI. The token lasts about one hour.",
  pat: "Paste a programmatic access token at the password prompt.",
};

const codeStyle: React.CSSProperties = {
  fontFamily: "var(--font-mono)", fontSize: 12, margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-all", flex: 1,
};

function Snippet({ text }: { text: string }) {
  return (
    <div style={{ display: "flex", gap: 8, alignItems: "flex-start", marginTop: 6 }}>
      <pre style={codeStyle}>{text}</pre>
      <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => ClipboardSetText(text)} />
    </div>
  );
}

/** Login-method picker plus push/pull commands for one repository URL. Copies host and user only, never a secret. */
export default function RegistryCommands({ repositoryUrl }: { repositoryUrl: string }) {
  const params = useConnectionStore((s) => s.params);
  const s = registrySnippets(repositoryUrl, params);
  const [picked, setPicked] = useState<LoginMethod | null>(null);
  const method = picked ?? s.defaultMethod;

  return (
    <div>
      <Space direction="vertical" size={2} style={{ width: "100%" }}>
        <Text strong style={{ fontSize: 12 }}>Registry commands</Text>
        <Segmented size="small" options={METHODS} value={method} onChange={(v) => setPicked(v as LoginMethod)} />
        <Text type="secondary" style={{ fontSize: 11 }}>{HINTS[method]}</Text>
      </Space>
      <Snippet text={s.login[method]} />
      <Snippet text={s.push} />
    </div>
  );
}
