// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Space, Typography, Button, Alert,
  Divider, message, Select, Tooltip, Segmented,
} from "antd";
import {
  KeyOutlined,
  FolderOpenOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import {
  CheckAvailableKeyTools,
  GenerateKeyPair,
  AlterUserProperty,
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

/** The two RSA public-key slots Snowflake exposes for zero-downtime rotation. */
export type KeySlot = "RSA_PUBLIC_KEY" | "RSA_PUBLIC_KEY_2";

const SLOT_LABEL: Record<KeySlot, string> = {
  RSA_PUBLIC_KEY:   "Key 1",
  RSA_PUBLIC_KEY_2: "Key 2",
};

// slot → the users.BuildAlterUserPropertySQL property key.
export const SLOT_PROPERTY: Record<KeySlot, string> = {
  RSA_PUBLIC_KEY:   "rsaPublicKey",
  RSA_PUBLIC_KEY_2: "rsaPublicKey2",
};

/**
 * stripPem reduces a pasted public key to the bare base64 payload Snowflake
 * expects: it drops the -----BEGIN/-----END----- lines and all whitespace, so an
 * admin can paste either a full PEM file received from a user or an
 * already-stripped key.
 */
function stripPem(s: string): string {
  return s
    .split(/\r?\n/)
    .filter((l) => !l.trim().startsWith("-----"))
    .join("")
    .replace(/\s+/g, "");
}

interface Props {
  /** When set, the modal is in "apply to existing user" mode. */
  username?: string;
  /** Which key slot to target when applying to a user. Defaults to slot 1. */
  slot?: KeySlot;
  /** When true, applying confirms that the existing key in this slot is replaced. */
  slotHasKey?: boolean;
  /**
   * When provided (create-user mode), "Use this key" is shown instead of
   * "Apply to user", and the generated public key is passed to this callback.
   */
  onKeyPicked?: (publicKey: string) => void;
  /** Called after a successful apply so the parent can reload properties. */
  onApplied?: () => void;
  onClose: () => void;
}

export default function KeyPairAuthModal({
  username, slot = "RSA_PUBLIC_KEY", slotHasKey, onKeyPicked, onApplied, onClose,
}: Props) {
  const [source,        setSource]        = useState<"generate" | "paste">("generate");
  const [availableTools, setAvailableTools] = useState<string[]>([]);
  const [method,     setMethod]     = useState("go");
  const [keyPath,    setKeyPath]    = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [generating, setGenerating] = useState(false);
  const [result,     setResult]     = useState<keypair.KeyPairResult | null>(null);
  const [genError,   setGenError]   = useState<string | null>(null);
  const [pasted,     setPasted]     = useState("");
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

  // The public key that "Apply" / "Use this key" will register — from the
  // generated result or the pasted (and stripped) input, depending on source.
  const effectiveKey = source === "generate" ? (result?.publicKey ?? "") : stripPem(pasted);

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

  // applyKey registers effectiveKey with the user's target slot through the
  // single tested SQL builder (AlterUserProperty → users.BuildAlterUserPropertySQL).
  const applyKey = async () => {
    if (!effectiveKey || !username) return;
    setApplying(true);
    setApplyError(null);
    try {
      await AlterUserProperty(username, SLOT_PROPERTY[slot], effectiveKey);
      message.success(`RSA public key (${SLOT_LABEL[slot]}) applied to ${username}`);
      onApplied?.();
      onClose();
    } catch (e) {
      // A generated (but unapplied) key pair is already on disk; a pasted key
      // is not, so only the generate flow needs the orphaned-file hint.
      const hint = source === "generate" && result
        ? `\nThe generated key pair was NOT applied. Delete the unused key files at ` +
          `${keyPath.trim()} (and its _pub.pem), or keep them to retry.`
        : "";
      setApplyError(`${friendlyError(e)}${hint}`);
    } finally {
      setApplying(false);
    }
  };

  const handleApply = () => {
    if (!effectiveKey) return;
    if (slotHasKey) {
      Modal.confirm({
        title: `Replace ${SLOT_LABEL[slot]} on ${username}?`,
        content: "This user already has a public key in this slot. Applying will " +
          "overwrite it — anyone still authenticating with the old key will be locked out.",
        okText: "Replace",
        okButtonProps: { danger: true },
        onOk: applyKey,
      });
    } else {
      applyKey();
    }
  };

  const handleUseKey = () => {
    if (!effectiveKey || !onKeyPicked) return;
    onKeyPicked(effectiveKey);
    onClose();
  };

  const title = username
    ? `Key Pair Auth — ${username} · ${SLOT_LABEL[slot]}`
    : "Generate RSA Key Pair";

  const applyButtons = (
    <>
      {applyError && (
        <Alert
          type="error"
          message={<span style={{ fontSize: 12, whiteSpace: "pre-line" }}>{applyError}</span>}
          style={{ marginBottom: 12 }}
        />
      )}
      <Space>
        {username && (
          <Button type="primary" loading={applying} disabled={!effectiveKey} onClick={handleApply}>
            Apply to {username}
          </Button>
        )}
        {onKeyPicked && (
          <Button type="primary" disabled={!effectiveKey} onClick={handleUseKey}>
            Use this key
          </Button>
        )}
        <Button onClick={onClose}>Close</Button>
      </Space>
    </>
  );

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
      {username && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={
            <span style={{ fontSize: 11 }}>
              Snowflake supports two key slots for zero-downtime rotation: set{" "}
              <b>Key 2</b>, migrate every client to the new private key, then
              remove <b>Key 1</b>.
            </span>
          }
        />
      )}

      <Segmented
        block
        value={source}
        onChange={(v) => { setSource(v as "generate" | "paste"); setApplyError(null); }}
        options={[
          { label: "Generate new key", value: "generate" },
          { label: "Paste existing public key", value: "paste" },
        ]}
        style={{ marginBottom: 16 }}
      />

      {source === "paste" ? (
        <Form layout="vertical" size="small">
          <Form.Item
            label="Public key"
            style={{ marginBottom: 12 }}
            help={
              <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                Paste the user's RSA public key — a full PEM (with{" "}
                <code>-----BEGIN PUBLIC KEY-----</code> lines) or already-stripped
                base64. Header/footer lines and whitespace are removed automatically.
              </span>
            }
          >
            <Input.TextArea
              value={pasted}
              onChange={(e) => setPasted(e.target.value)}
              placeholder="-----BEGIN PUBLIC KEY-----&#10;MIIBIjANBgkq…&#10;-----END PUBLIC KEY-----"
              autoSize={{ minRows: 4, maxRows: 10 }}
              style={{ fontSize: 11, fontFamily: "monospace" }}
            />
          </Form.Item>
          {applyButtons}
        </Form>
      ) : (
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

              {applyButtons}
            </>
          )}
        </Form>
      )}
    </Modal>
  );
}
