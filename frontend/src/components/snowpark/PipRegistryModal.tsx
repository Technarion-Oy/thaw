// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import {
  Modal,
  Button,
  Input,
  Radio,
  Select,
  Space,
  Switch,
  Typography,
} from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import {
  GetPipRegistryConfig,
  SavePipRegistryConfig,
  ResetPipRegistryConfig,
  PickCACertFile,
} from "../../../wailsjs/go/main/App";

const { Title, Text } = Typography;

interface Credential {
  registry: string;
  username: string;
  password: string;
}

interface PipRegConfig {
  primaryURL: string;
  additionalRegistries: string[];
  behavior: string;
  credentials: Credential[];
  enableProxy: boolean;
  proxyURL: string;
  proxyUsername: string;
  proxyPassword: string;
  proxyBypassHosts: string;
  trustedHosts: string;
  customCACertPath: string;
}

interface Props {
  open: boolean;
  onClose: () => void;
}

const emptyConfig = (): PipRegConfig => ({
  primaryURL: "",
  additionalRegistries: [],
  behavior: "extra",
  credentials: [],
  enableProxy: false,
  proxyURL: "",
  proxyUsername: "",
  proxyPassword: "",
  proxyBypassHosts: "",
  trustedHosts: "",
  customCACertPath: "",
});

function normalize(loaded: any): PipRegConfig {
  return {
    primaryURL: loaded?.primaryURL ?? "",
    additionalRegistries: loaded?.additionalRegistries ?? [],
    behavior: loaded?.behavior || "extra",
    credentials: (loaded?.credentials ?? []).map((c: any) => ({
      registry: c.registry ?? "",
      username: c.username ?? "",
      password: c.password ?? "",
    })),
    enableProxy: loaded?.enableProxy ?? false,
    proxyURL: loaded?.proxyURL ?? "",
    proxyUsername: loaded?.proxyUsername ?? "",
    proxyPassword: loaded?.proxyPassword ?? "",
    proxyBypassHosts: loaded?.proxyBypassHosts ?? "",
    trustedHosts: loaded?.trustedHosts ?? "",
    customCACertPath: loaded?.customCACertPath ?? "",
  };
}

export default function PipRegistryModal({ open, onClose }: Props) {
  const [cfg, setCfg] = useState<PipRegConfig>(emptyConfig());
  const [credTarget, setCredTarget] = useState<string | undefined>(undefined);
  const [credUser, setCredUser] = useState("");
  const [credPass, setCredPass] = useState("");
  const [saving, setSaving] = useState(false);

  // Load config when the modal opens.
  useEffect(() => {
    if (!open) return;
    GetPipRegistryConfig()
      .then((loaded) => {
        setCfg(normalize(loaded));
        setCredTarget(undefined);
        setCredUser("");
        setCredPass("");
      })
      .catch(() => {});
  }, [open]);

  const set = (patch: Partial<PipRegConfig>) =>
    setCfg((prev) => ({ ...prev, ...patch }));

  const handleAddRegistry = () =>
    set({ additionalRegistries: [...cfg.additionalRegistries, ""] });

  const handleRegistryChange = (idx: number, val: string) => {
    const next = [...cfg.additionalRegistries];
    next[idx] = val;
    set({ additionalRegistries: next });
  };

  const handleRemoveRegistry = (idx: number) => {
    set({ additionalRegistries: cfg.additionalRegistries.filter((_, i) => i !== idx) });
  };

  const handleUpsertCredential = () => {
    if (!credTarget || !credUser) return;
    const existing = cfg.credentials.filter((c) => c.registry !== credTarget);
    const cred: Credential = { registry: credTarget, username: credUser, password: credPass };
    set({ credentials: [...existing, cred] });
    setCredUser("");
    setCredPass("");
  };

  const handleRemoveCredential = (registry: string) => {
    set({ credentials: cfg.credentials.filter((c) => c.registry !== registry) });
  };

  const handlePickCACert = () => {
    PickCACertFile()
      .then((path) => { if (path) set({ customCACertPath: path }); })
      .catch(() => {});
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await SavePipRegistryConfig(cfg as any);
      onClose();
    } catch (_e) {
      // ignore
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    await ResetPipRegistryConfig().catch(() => {});
    const reloaded = await GetPipRegistryConfig().catch(() => null);
    setCfg(reloaded ? normalize(reloaded) : emptyConfig());
    setCredTarget(undefined);
    setCredUser("");
    setCredPass("");
  };

  // Registry URL options for the credentials target selector.
  const registryOptions = [cfg.primaryURL, ...cfg.additionalRegistries]
    .filter(Boolean)
    .map((u) => ({ value: u, label: u }));

  return (
    <Modal
      title="Configure pip Registry"
      open={open}
      onCancel={onClose}
      width={560}
      footer={[
        <Button key="reset" danger style={{ float: "left" }} onClick={handleReset}>
          Reset to Defaults
        </Button>,
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="save" type="primary" loading={saving} onClick={handleSave}>Save</Button>,
      ]}
    >
      <Space direction="vertical" style={{ width: "100%" }} size={16}>

        {/* ── 3.1 Registry Settings ─────────────────────────────────────── */}
        <div>
          <Title level={5} style={{ margin: "0 0 10px" }}>Registry Settings</Title>

          <Space direction="vertical" style={{ width: "100%" }} size={8}>
            <div>
              <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                Primary Registry URL
              </Text>
              <Input
                size="small"
                placeholder="https://pypi.org/simple"
                value={cfg.primaryURL}
                onChange={(e) => set({ primaryURL: e.target.value })}
                style={{ fontFamily: "monospace", fontSize: 12 }}
              />
            </div>

            <div>
              <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                Additional Registries
              </Text>
              <Space direction="vertical" style={{ width: "100%" }} size={4}>
                {cfg.additionalRegistries.map((reg, idx) => (
                  <div key={idx} style={{ display: "flex", gap: 6 }}>
                    <Input
                      size="small"
                      value={reg}
                      onChange={(e) => handleRegistryChange(idx, e.target.value)}
                      style={{ flex: 1, fontFamily: "monospace", fontSize: 12 }}
                    />
                    <Button
                      size="small"
                      type="text"
                      danger
                      icon={<MinusCircleOutlined />}
                      onClick={() => handleRemoveRegistry(idx)}
                    />
                  </div>
                ))}
                <Button
                  size="small"
                  icon={<PlusOutlined />}
                  onClick={handleAddRegistry}
                  style={{ alignSelf: "flex-start" }}
                >
                  Add Registry
                </Button>
              </Space>
            </div>

            <div>
              <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                Registry Behavior
              </Text>
              <Radio.Group
                value={cfg.behavior || "extra"}
                onChange={(e) => set({ behavior: e.target.value as string })}
              >
                <Space direction="vertical" size={2}>
                  <Radio value="override">
                    Override default PyPI entirely
                    <Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>(--index-url)</Text>
                  </Radio>
                  <Radio value="extra">
                    Search custom registry in addition to default PyPI
                    <Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>(--extra-index-url)</Text>
                  </Radio>
                </Space>
              </Radio.Group>
            </div>
          </Space>
        </div>

        {/* ── 3.2 Authentication ────────────────────────────────────────── */}
        <div>
          <Title level={5} style={{ margin: "0 0 10px" }}>Authentication</Title>

          <Space direction="vertical" style={{ width: "100%" }} size={8}>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <div style={{ flex: "1 1 160px" }}>
                <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                  Target Registry
                </Text>
                <Select
                  size="small"
                  style={{ width: "100%" }}
                  placeholder="Select registry"
                  value={credTarget}
                  onChange={setCredTarget}
                  options={registryOptions}
                  allowClear
                />
              </div>
              <div style={{ flex: "1 1 100px" }}>
                <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                  Username
                </Text>
                <Input
                  size="small"
                  value={credUser}
                  onChange={(e) => setCredUser(e.target.value)}
                />
              </div>
              <div style={{ flex: "1 1 100px" }}>
                <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                  Password / Access Token
                </Text>
                <Input.Password
                  size="small"
                  value={credPass}
                  onChange={(e) => setCredPass(e.target.value)}
                />
              </div>
            </div>
            <Button
              size="small"
              type="dashed"
              disabled={!credTarget || !credUser}
              onClick={handleUpsertCredential}
            >
              Add / Update Credentials
            </Button>

            {cfg.credentials.length > 0 && (
              <div>
                <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                  Configured credentials
                </Text>
                <Space direction="vertical" style={{ width: "100%" }} size={2}>
                  {cfg.credentials.map((c) => (
                    <div key={c.registry} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "2px 0" }}>
                      <Text style={{ fontFamily: "monospace", fontSize: 11 }}>{c.registry}</Text>
                      <Button
                        size="small"
                        type="text"
                        danger
                        onClick={() => handleRemoveCredential(c.registry)}
                      >
                        Remove
                      </Button>
                    </div>
                  ))}
                </Space>
              </div>
            )}
          </Space>
        </div>

        {/* ── 3.3 Proxy & SSL Settings ─────────────────────────────────── */}
        <div>
          <Title level={5} style={{ margin: "0 0 10px" }}>Proxy &amp; SSL Settings</Title>

          <Space direction="vertical" style={{ width: "100%" }} size={8}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <Switch
                size="small"
                checked={cfg.enableProxy}
                onChange={(v) => set({ enableProxy: v })}
              />
              <Text style={{ fontSize: 13 }}>Enable Proxy</Text>
            </div>

            {cfg.enableProxy && (
              <>
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                    Proxy URL
                  </Text>
                  <Input
                    size="small"
                    placeholder="http://proxy.company.com:8080"
                    value={cfg.proxyURL}
                    onChange={(e) => set({ proxyURL: e.target.value })}
                    style={{ fontFamily: "monospace", fontSize: 12 }}
                  />
                </div>
                <div style={{ display: "flex", gap: 8 }}>
                  <div style={{ flex: 1 }}>
                    <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                      Proxy Username
                    </Text>
                    <Input
                      size="small"
                      value={cfg.proxyUsername}
                      onChange={(e) => set({ proxyUsername: e.target.value })}
                    />
                  </div>
                  <div style={{ flex: 1 }}>
                    <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                      Proxy Password
                    </Text>
                    <Input.Password
                      size="small"
                      value={cfg.proxyPassword}
                      onChange={(e) => set({ proxyPassword: e.target.value })}
                    />
                  </div>
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                    Bypass Proxy for these hosts
                  </Text>
                  <Input
                    size="small"
                    placeholder="localhost, 127.0.0.1"
                    value={cfg.proxyBypassHosts}
                    onChange={(e) => set({ proxyBypassHosts: e.target.value })}
                    style={{ fontFamily: "monospace", fontSize: 12 }}
                  />
                </div>
              </>
            )}

            <div>
              <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                Trusted Hosts
              </Text>
              <Input
                size="small"
                placeholder="internal.registry.com"
                value={cfg.trustedHosts}
                onChange={(e) => set({ trustedHosts: e.target.value })}
                style={{ fontFamily: "monospace", fontSize: 12 }}
              />
            </div>

            <div>
              <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 4 }}>
                Custom CA Certificate
              </Text>
              <div style={{ display: "flex", gap: 6 }}>
                <Input
                  size="small"
                  readOnly
                  value={cfg.customCACertPath}
                  placeholder="No certificate selected"
                  style={{ flex: 1, fontFamily: "monospace", fontSize: 12 }}
                />
                <Button size="small" onClick={handlePickCACert}>Browse…</Button>
                {cfg.customCACertPath && (
                  <Button size="small" onClick={() => set({ customCACertPath: "" })}>Clear</Button>
                )}
              </div>
            </div>
          </Space>
        </div>

      </Space>
    </Modal>
  );
}
