// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Space, Typography, Button, Alert,
  Divider, message, Select, Tooltip,
} from "antd";
import {
  KeyOutlined,
  FolderOpenOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import {
  CheckAvailableKeyTools,
  GenerateKeyPair,
  SetUserPublicKey,
  PickDirectory,
} from "../../../wailsjs/go/app/App";
import type { keypair } from "../../../wailsjs/go/models";
import { friendlyError } from "../common/PropertyRows";

const { Text } = Typography;

const METHOD_LABELS: Record<string, string> = {
  go:           "Go built-in crypto",
  openssl:      "OpenSSL",
  "ssh-keygen": "ssh-keygen",
};

interface Props {
  /** When set, the modal is in "apply to existing user" mode. */
  username?: string;
  /**
   * When provided (create-user mode), "Use this key" is shown instead of
   * "Apply to user", and the generated public key is passed to this callback.
   */
  onKeyPicked?: (publicKey: string) => void;
  onClose: () => void;
}

export default function KeyPairAuthModal({ username, onKeyPicked, onClose }: Props) {
  const [availableTools, setAvailableTools] = useState<string[]>([]);
  const [method,     setMethod]     = useState("go");
  const [keyPath,    setKeyPath]    = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [generating, setGenerating] = useState(false);
  const [result,     setResult]     = useState<keypair.KeyPairResult | null>(null);
  const [genError,   setGenError]   = useState<string | null>(null);
  const [applying,   setApplying]   = useState(false);
  const [applyError, setApplyError] = useState<string | null>(null);

  useEffect(() => {
    CheckAvailableKeyTools().then((tools) => {
      setAvailableTools(tools);
      // Default to openssl if available, otherwise go.
      if (tools.includes("openssl")) setMethod("openssl");
    }).catch(() => setAvailableTools(["go"]));
  }, []);

  const isGo = method === "go";

  const pickPath = async () => {
    try {
      const dir = await PickDirectory();
      if (dir) setKeyPath(dir + "/rsa_key.p8");
    } catch { /* cancelled */ }
  };

  const handleGenerate = async () => {
    if (!keyPath.trim()) return;
    setGenerating(true);
    setGenError(null);
    setResult(null);
    setApplyError(null);
    try {
      const r = await GenerateKeyPair(method, keyPath.trim(), isGo ? "" : passphrase);
      setResult(r);
    } catch (e) {
      setGenError(friendlyError(e));
    } finally {
      setGenerating(false);
    }
  };

  const handleApply = async () => {
    if (!result || !username) return;
    setApplying(true);
    setApplyError(null);
    try {
      await SetUserPublicKey(username, result.publicKey);
      message.success(`RSA public key applied to ${username}`);
      onClose();
    } catch (e) {
      // The key pair was already written to disk by handleGenerate; if the
      // apply is refused (e.g. insufficient privileges), tell the user where
      // the now-unused private key lives so it isn't silently orphaned.
      setApplyError(
        `${friendlyError(e)}\nThe generated key pair was NOT applied. ` +
        `Delete the unused key files at ${keyPath.trim()} (and its _pub.pem), or keep them to retry.`,
      );
    } finally {
      setApplying(false);
    }
  };

  const handleUseKey = () => {
    if (!result || !onKeyPicked) return;
    onKeyPicked(result.publicKey);
    onClose();
  };

  const title = username
    ? `Configure Key Pair Auth — ${username}`
    : "Generate RSA Key Pair";

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <KeyOutlined style={{ color: "var(--link)" }} />
          <span>{title}</span>
        </Space>
      }
      onCancel={onClose}
      footer={null}
      width={560}
      styles={{ body: { paddingTop: 16 } }}
    >
      <Form layout="vertical" size="small">

        {/* Method selector */}
        <Form.Item label="Key generation method" style={{ marginBottom: 12 }}>
          <Select
            value={method}
            onChange={(v) => { setMethod(v); setResult(null); setGenError(null); }}
            options={availableTools.map((t) => ({ value: t, label: METHOD_LABELS[t] ?? t }))}
            style={{ width: "100%" }}
          />
        </Form.Item>

        {/* Private key path */}
        <Form.Item
          label="Private key output path"
          required
          style={{ marginBottom: 10 }}
          help={
            <span style={{ fontSize: 11 }}>
              PKCS#8 PEM file — keep this secret. Public key will be saved alongside.
            </span>
          }
        >
          <Space.Compact style={{ width: "100%" }}>
            <Input
              value={keyPath}
              onChange={(e) => setKeyPath(e.target.value)}
              placeholder="~/rsa_key.p8"
            />
            <Button icon={<FolderOpenOutlined />} onClick={pickPath} title="Browse…" />
          </Space.Compact>
        </Form.Item>

        {/* Passphrase — disabled for Go built-in */}
        <Form.Item
          label="Passphrase"
          style={{ marginBottom: 12 }}
          help={
            isGo ? (
              <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                Go built-in crypto does not support encrypted private keys.
                Choose OpenSSL or ssh-keygen to use a passphrase.
              </span>
            ) : (
              <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                Leave blank for an unencrypted key. If set, the private key will be encrypted.
              </span>
            )
          }
        >
          <Tooltip
            title={isGo ? "Not supported by Go built-in crypto" : undefined}
            placement="right"
          >
            <Input.Password
              value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
              placeholder={isGo ? "Not available" : "Leave blank for no passphrase"}
              disabled={isGo}
              autoComplete="new-password"
            />
          </Tooltip>
        </Form.Item>

        {genError && (
          <Alert
            type="error"
            message={<span style={{ fontSize: 12 }}>{genError}</span>}
            style={{ marginBottom: 12 }}
          />
        )}

        <Button
          type="primary"
          icon={<ThunderboltOutlined />}
          loading={generating}
          disabled={!keyPath.trim()}
          onClick={handleGenerate}
          style={{ marginBottom: 16 }}
        >
          Generate key pair
        </Button>

        {result && (
          <>
            <Divider
              orientation="left"
              orientationMargin={0}
              style={{ fontSize: 11, color: "var(--text-muted)", margin: "0 0 12px" }}
            >
              Generated key pair
            </Divider>

            <div style={{ marginBottom: 8 }}>
              <Text style={{ fontSize: 11, color: "var(--text-muted)", display: "block", marginBottom: 2 }}>
                Private key
              </Text>
              <Text style={{ fontSize: 12, fontFamily: "monospace" }}>{result.privateKeyPath}</Text>
            </div>
            <div style={{ marginBottom: 12 }}>
              <Text style={{ fontSize: 11, color: "var(--text-muted)", display: "block", marginBottom: 2 }}>
                Public key
              </Text>
              <Text style={{ fontSize: 12, fontFamily: "monospace" }}>{result.publicKeyPath}</Text>
            </div>

            <Form.Item label="Public key (for Snowflake)" style={{ marginBottom: 12 }}>
              <Input.TextArea
                value={result.publicKey}
                readOnly
                autoSize={{ minRows: 3, maxRows: 6 }}
                style={{ fontSize: 11, fontFamily: "monospace" }}
              />
            </Form.Item>

            {applyError && (
              <Alert
                type="error"
                message={<span style={{ fontSize: 12, whiteSpace: "pre-line" }}>{applyError}</span>}
                style={{ marginBottom: 12 }}
              />
            )}

            <Space>
              {username && (
                <Button type="primary" loading={applying} onClick={handleApply}>
                  Apply to {username}
                </Button>
              )}
              {onKeyPicked && (
                <Button type="primary" onClick={handleUseKey}>
                  Use this key
                </Button>
              )}
              <Button onClick={onClose}>Close</Button>
            </Space>
          </>
        )}
      </Form>
    </Modal>
  );
}
