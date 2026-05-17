// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Core IPC & App Lifecycle

import { useState, useEffect, useCallback, useRef } from "react";
import { Form, Input, Button, Alert, Space, Typography, Select, Divider, Tooltip, Modal, Popconfirm, message } from "antd";
import { CloudServerOutlined, FolderOpenOutlined, SaveOutlined, CopyOutlined, DeleteOutlined, StarOutlined, PlusOutlined, EditOutlined } from "@ant-design/icons";
import UserAgreementModal from "./UserAgreementModal";
import {
  Connect, CancelConnect, LoadSnowflakeCLIConfig,
  GetSnowflakeCLIConfigPath, PickSnowflakeCLIConfigPath,
  SaveProfile, DeleteProfile, CloneProfile, SetDefaultProfile, ClearDefaultProfile, RenameProfile,
} from "../../../wailsjs/go/main/App";
import { sfconfig } from "../../../wailsjs/go/models";
import { useConnectionStore, type ConnectionParams } from "../../store/connectionStore";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";

const { Title, Text } = Typography;

const AUTH_OPTIONS = [
  {
    value: "username_password_mfa",
    label: "Password + MFA push",
    description: "Approve a push notification on your MFA device",
  },
  {
    value: "externalbrowser",
    label: "Browser SSO",
    description: "Opens a browser window for SSO / MFA",
  },
  {
    value: "snowflake",
    label: "Password only",
    description: "Classic username + password (optionally with a TOTP code)",
  },
  {
    value: "okta",
    label: "Okta native SSO",
    description: "Authenticates directly against your Okta tenant",
  },
  {
    value: "snowflake_jwt",
    label: "Key pair (JWT)",
    description: "RSA private key — no password needed",
  },
];

const needsPassword = (auth: string) =>
  auth !== "externalbrowser" && auth !== "snowflake_jwt";

export default function ConnectModal({ onClose }: { onClose?: () => void }) {
  const [form] = Form.useForm<ConnectionParams>();
  const [loading, setLoading]   = useState(false);
  const [error, setError]       = useState<string | null>(null);
  const [auth, setAuth]         = useState("username_password_mfa");
  const [agreementOpen, setAgreementOpen] = useState(false);
  const setConnected            = useConnectionStore((s) => s.setConnected);
  const profileManagerEnabled   = useFeatureFlagsStore((s) => s.flags.snowflakeCLIProfileManager);

  const [cliConfig, setCliConfig] = useState<sfconfig.Config | null>(null);
  const [cliConfigPath, setCliConfigPath] = useState<string>("");
  const [selectedProfile, setSelectedProfile] = useState<string | undefined>(undefined);
  const [nameModalOpen, setNameModalOpen] = useState(false);
  const [nameModalMode, setNameModalMode] = useState<"new" | "clone" | "rename">("new");
  const [nameModalValue, setNameModalValue] = useState("");
  const [profileBusy, setProfileBusy] = useState(false);
  const profileBusyRef = useRef(false);

  const refreshCliConfig = useCallback((selectAfter?: string) => {
    LoadSnowflakeCLIConfig()
      .then((cfg) => {
        const hasCfg = cfg.connections?.length ? cfg : null;
        setCliConfig(hasCfg);
        if (selectAfter && hasCfg?.connections?.find((c) => c.name === selectAfter)) {
          setSelectedProfile(selectAfter);
        } else if (selectAfter === undefined && hasCfg?.defaultConnection) {
          // Initial load: auto-select the default profile.
          setSelectedProfile(hasCfg.defaultConnection);
        }
      })
      .catch(() => {
        setCliConfig(null);
      });
  }, []);

  // Load Snowflake CLI config and path once on mount.
  useEffect(() => {
    GetSnowflakeCLIConfigPath().then(setCliConfigPath);
    refreshCliConfig();
  }, [refreshCliConfig]);

  const changeCliConfigPath = async () => {
    try {
      const path = await PickSnowflakeCLIConfigPath();
      if (path) {
        setCliConfigPath(path);
        setSelectedProfile(undefined);
        refreshCliConfig();
      }
    } catch (e) {
      console.error("Failed to pick config path", e);
    }
  };

  const clearProfileSelection = () => {
    setSelectedProfile(undefined);
    form.resetFields();
    setAuth("username_password_mfa");
  };

  const applyCliConnection = (name: string) => {
    setSelectedProfile(name);
    const conn = cliConfig?.connections?.find((c) => c.name === name);
    if (!conn) return;

    const authValue = (conn.authenticator || "username_password_mfa").toLowerCase();
    setAuth(authValue);

    form.setFieldsValue({
      account:              conn.account,
      user:                 conn.user,
      password:             conn.password,
      role:                 conn.role,
      warehouse:            conn.warehouse,
      database:             conn.database,
      schema:               conn.schema,
      authenticator:        authValue,
      passcode:             conn.passcode,
      oktaUrl:              conn.oktaUrl,
      privateKeyPath:       conn.privateKeyPath,
      privateKeyPassphrase: conn.privateKeyPassphrase,
    });
  };

  const profileNameIsValid = (name: string) =>
    /^[A-Za-z0-9_-]+$/.test(name);

  const existingProfileNames = new Set(
    cliConfig?.connections?.map((c) => c.name) ?? [],
  );

  /** True when the name modal should block submission due to a duplicate. */
  const nameModalHasDuplicate = (() => {
    const name = nameModalValue.trim();
    if (!name) return false;
    // In rename mode, the current profile name is not a conflict.
    if (nameModalMode === "rename" && name === selectedProfile) return false;
    return existingProfileNames.has(name);
  })();

  const buildConnectionFromForm = (profileName: string) => {
    const values = form.getFieldsValue(true);
    return new sfconfig.Connection({
      name:                 profileName,
      account:              values.account || "",
      user:                 values.user || "",
      password:             values.password || "",
      role:                 values.role || "",
      warehouse:            values.warehouse || "",
      database:             values.database || "",
      schema:               values.schema || "",
      authenticator:        values.authenticator || "",
      passcode:             values.passcode || "",
      oktaUrl:              values.oktaUrl || "",
      privateKeyPath:       values.privateKeyPath || "",
      privateKeyPassphrase: values.privateKeyPassphrase || "",
    });
  };

  /** Wraps an async profile operation with a ref-based busy guard. */
  const withProfileBusy = <T extends unknown[]>(
    fn: (...args: T) => Promise<void>,
  ) => async (...args: T) => {
    if (profileBusyRef.current) return;
    profileBusyRef.current = true;
    setProfileBusy(true);
    try { await fn(...args); } finally {
      profileBusyRef.current = false;
      setProfileBusy(false);
    }
  };

  const handleSaveProfile = withProfileBusy(async (profileName: string) => {
    try {
      await SaveProfile(buildConnectionFromForm(profileName));
      message.success(`Profile "${profileName}" saved`);
      refreshCliConfig(profileName);
    } catch (e) {
      message.error(`Failed to save profile: ${e}`);
    }
  });

  const handleCloneProfile = withProfileBusy(async (newName: string) => {
    if (!selectedProfile) return;
    try {
      await CloneProfile(selectedProfile, newName);
      message.success(`Profile "${selectedProfile}" cloned as "${newName}"`);
      refreshCliConfig(newName);
    } catch (e) {
      message.error(`Failed to clone profile: ${e}`);
    }
  });

  const handleRenameProfile = withProfileBusy(async (newName: string) => {
    if (!selectedProfile) return;
    try {
      await RenameProfile(selectedProfile, newName);
      message.success(`Profile renamed to "${newName}"`);
      refreshCliConfig(newName);
    } catch (e) {
      message.error(`Failed to rename profile: ${e}`);
    }
  });

  const isSelectedDefault = !!(selectedProfile && cliConfig?.defaultConnection === selectedProfile);

  const handleToggleDefault = withProfileBusy(async () => {
    if (!selectedProfile) return;
    try {
      if (isSelectedDefault) {
        await ClearDefaultProfile();
        message.success(`"${selectedProfile}" is no longer the default`);
      } else {
        await SetDefaultProfile(selectedProfile);
        message.success(`"${selectedProfile}" set as default`);
      }
      refreshCliConfig(selectedProfile);
    } catch (e) {
      message.error(`Failed to update default: ${e}`);
    }
  });

  const handleDeleteProfile = withProfileBusy(async () => {
    if (!selectedProfile) return;
    const name = selectedProfile;
    try {
      await DeleteProfile(name);
      message.success(`Profile "${name}" deleted`);
      setSelectedProfile(undefined);
      form.resetFields();
      setAuth("username_password_mfa");
      refreshCliConfig();
    } catch (e) {
      message.error(`Failed to delete profile: ${e}`);
    }
  });

  const openNameModal = (mode: "new" | "clone" | "rename") => {
    setNameModalMode(mode);
    setNameModalValue(mode === "rename" ? (selectedProfile || "") : "");
    setNameModalOpen(true);
  };

  const confirmNameModal = () => {
    const name = nameModalValue.trim();
    if (!name || !profileNameIsValid(name) || nameModalHasDuplicate) return;
    setNameModalOpen(false);
    if (nameModalMode === "new") {
      handleSaveProfile(name);
    } else if (nameModalMode === "clone") {
      handleCloneProfile(name);
    } else if (nameModalMode === "rename") {
      handleRenameProfile(name);
    }
  };

  const onFinish = async (values: ConnectionParams) => {
    setLoading(true);
    setError(null);
    try {
      await Connect(values);
      setConnected(values);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      open
      centered
      width={540}
      maskClosable={false}
      closable={!!onClose}
      onCancel={onClose}
      styles={{ body: { maxHeight: "60vh", overflowY: "auto" } }}
      footer={
        <div style={{ display: "flex", flexDirection: "column", alignItems: "stretch", gap: 0 }}>
          {loading ? (
            <Button danger block onClick={() => CancelConnect()}>
              Cancel
            </Button>
          ) : (
            <Button type="primary" block onClick={() => form.submit()}>
              {auth === "externalbrowser" ? "Connect (opens browser)" : "Connect"}
            </Button>
          )}
          <div style={{ textAlign: "center", marginTop: 12 }}>
            <Button
              type="link"
              size="small"
              onClick={() => setAgreementOpen(true)}
              style={{ fontSize: 12, color: "var(--text-muted)" }}
            >
              User Agreement
            </Button>
          </div>
        </div>
      }
    >
      <Space direction="vertical" size={24} style={{ width: "100%" }}>
          <Space align="center">
            <CloudServerOutlined style={{ fontSize: 28, color: "#29B6F6" }} />
            <Title level={3} style={{ margin: 0, color: "var(--text)" }}>
              Connect to Snowflake
            </Title>
          </Space>

          {/* ── Snowflake CLI profiles ──────────────────────────────────── */}
          {profileManagerEnabled && <div>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end", marginBottom: 6 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Snowflake CLI profiles
              </Text>
              <Tooltip title={cliConfigPath}>
                <Button
                  type="link"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  onClick={changeCliConfigPath}
                  style={{ fontSize: 11, padding: 0, height: "auto" }}
                >
                  Change config…
                </Button>
              </Tooltip>
            </div>

            {cliConfig && cliConfig.connections?.length > 0 ? (
              <Select
                style={{ width: "100%" }}
                placeholder="Select a connection profile…"
                onChange={applyCliConnection}
                onClear={clearProfileSelection}
                value={selectedProfile}
                allowClear
                options={cliConfig.connections.map((c) => ({
                  value: c.name,
                  label: cliConfig.defaultConnection === c.name ? `${c.name} (default)` : c.name,
                }))}
              />
            ) : (
              <div style={{
                padding: "8px 12px",
                background: "var(--bg-faint)",
                border: "1px dashed var(--border)",
                borderRadius: 6,
                textAlign: "center"
              }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  No profiles found in {cliConfigPath.split("/").pop() || "config.toml"}
                </Text>
              </div>
            )}

            {/* ── Profile action buttons ─────────────────────────────── */}
            <div style={{ display: "flex", gap: 4, marginTop: 6, flexWrap: "wrap" }}>
              <Tooltip title="Create a new profile from the current form values">
                <Button
                  size="small"
                  icon={<PlusOutlined />}
                  disabled={profileBusy}
                  onClick={() => openNameModal("new")}
                >
                  New
                </Button>
              </Tooltip>
              {cliConfig && (
                <>
                  <Popconfirm
                    title={`Overwrite profile "${selectedProfile}" with current form values?`}
                    onConfirm={() => selectedProfile && handleSaveProfile(selectedProfile)}
                    okText="Overwrite"
                    disabled={!selectedProfile || profileBusy}
                  >
                    <Tooltip title="Overwrite the selected profile with the current form values">
                      <Button
                        size="small"
                        icon={<SaveOutlined />}
                        disabled={!selectedProfile || profileBusy}
                      >
                        Save
                      </Button>
                    </Tooltip>
                  </Popconfirm>
                  <Tooltip title="Rename the selected profile">
                    <Button
                      size="small"
                      icon={<EditOutlined />}
                      disabled={!selectedProfile || profileBusy}
                      onClick={() => openNameModal("rename")}
                    >
                      Rename
                    </Button>
                  </Tooltip>
                  <Tooltip title="Clone the selected profile under a new name">
                    <Button
                      size="small"
                      icon={<CopyOutlined />}
                      disabled={!selectedProfile || profileBusy}
                      onClick={() => openNameModal("clone")}
                    >
                      Clone
                    </Button>
                  </Tooltip>
                  <Tooltip title={isSelectedDefault ? "Remove as default profile" : "Set the selected profile as default"}>
                    <Button
                      size="small"
                      icon={<StarOutlined />}
                      disabled={!selectedProfile || profileBusy}
                      type={isSelectedDefault ? "primary" : "default"}
                      onClick={handleToggleDefault}
                    >
                      Default
                    </Button>
                  </Tooltip>
                  <Popconfirm
                    title={`Delete profile "${selectedProfile}"?`}
                    onConfirm={handleDeleteProfile}
                    okText="Delete"
                    okType="danger"
                    disabled={!selectedProfile || profileBusy}
                  >
                    <Tooltip title="Delete the selected profile">
                      <Button
                        size="small"
                        danger
                        icon={<DeleteOutlined />}
                        disabled={!selectedProfile || profileBusy}
                      >
                        Delete
                      </Button>
                    </Tooltip>
                  </Popconfirm>
                </>
              )}
            </div>

            <Divider style={{ borderColor: "var(--border)", margin: "16px 0 4px" }} />
          </div>}

          {error && <Alert type="error" message={error} showIcon />}

          <Form
            form={form}
            layout="vertical"
            onFinish={onFinish}
            requiredMark={false}
            initialValues={{ authenticator: "username_password_mfa" }}
          >
            {/* ── Connection details ─────────────────────────────────── */}
            <Form.Item name="account" label="Account" rules={[{ required: true }]}>
              <Input placeholder="myorg-account  or  locator.region (e.g. xy12345.eu-north-1)" />
            </Form.Item>

            <Space.Compact style={{ width: "100%", gap: 8, display: "flex" }}>
              <Form.Item name="role" label="Role" style={{ flex: 1 }}>
                <Input placeholder="SYSADMIN" />
              </Form.Item>
              <Form.Item name="warehouse" label="Warehouse" style={{ flex: 1 }}>
                <Input placeholder="COMPUTE_WH" />
              </Form.Item>
            </Space.Compact>

            <Space.Compact style={{ width: "100%", gap: 8, display: "flex" }}>
              <Form.Item name="database" label="Database" style={{ flex: 1 }}>
                <Input placeholder="optional" />
              </Form.Item>
              <Form.Item name="schema" label="Schema" style={{ flex: 1 }}>
                <Input placeholder="optional" />
              </Form.Item>
            </Space.Compact>

            <Divider style={{ borderColor: "var(--border)", margin: "4px 0 16px" }} />

            {/* ── Authentication ─────────────────────────────────────── */}
            <Form.Item name="authenticator" label="Authentication method">
              <Select
                onChange={(v) => {
                  setAuth(v);
                  form.resetFields(["passcode", "oktaUrl", "privateKeyPath", "privateKeyPassphrase"]);
                }}
                options={AUTH_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
                optionRender={(option) => {
                  const o = AUTH_OPTIONS.find((x) => x.value === option.value)!;
                  return (
                    <div>
                      <div>{o.label}</div>
                      <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                        {o.description}
                      </div>
                    </div>
                  );
                }}
              />
            </Form.Item>

            {/* Username */}
            {auth !== "externalbrowser" && (
              <Form.Item name="user" label="Username" rules={[{ required: true }]}>
                <Input autoComplete="username" />
              </Form.Item>
            )}

            {/* Password */}
            {needsPassword(auth) && (
              <Form.Item name="password" label="Password" rules={[{ required: true }]}>
                <Input.Password autoComplete="current-password" />
              </Form.Item>
            )}

            {/* TOTP passcode (snowflake authenticator only) */}
            {auth === "snowflake" && (
              <Form.Item name="passcode" label="TOTP passcode (optional)">
                <Input placeholder="6-digit code" maxLength={8} />
              </Form.Item>
            )}

            {/* Okta URL */}
            {auth === "okta" && (
              <Form.Item
                name="oktaUrl"
                label="Okta account URL"
                rules={[{ required: true, type: "url" }]}
              >
                <Input placeholder="https://mycompany.okta.com" />
              </Form.Item>
            )}

            {/* Key pair */}
            {auth === "snowflake_jwt" && (
              <>
                <Form.Item
                  name="privateKeyPath"
                  label="Private key path"
                  rules={[{ required: true }]}
                >
                  <Input placeholder="/path/to/rsa_key.p8" />
                </Form.Item>
                <Form.Item name="privateKeyPassphrase" label="Key passphrase (if encrypted)">
                  <Input.Password />
                </Form.Item>
              </>
            )}

          </Form>

          <UserAgreementModal open={agreementOpen} onClose={() => setAgreementOpen(false)} />

          {/* ── Profile name sub-modal ──────────────────────────── */}
          <Modal
            open={nameModalOpen}
            title={
              nameModalMode === "new" ? "New Profile"
                : nameModalMode === "clone" ? "Clone Profile"
                : "Rename Profile"
            }
            okText={
              nameModalMode === "new" ? "Create"
                : nameModalMode === "clone" ? "Clone"
                : "Rename"
            }
            onOk={confirmNameModal}
            onCancel={() => setNameModalOpen(false)}
            okButtonProps={{
              disabled:
                !nameModalValue.trim()
                || !profileNameIsValid(nameModalValue.trim())
                || nameModalHasDuplicate,
            }}
            destroyOnClose
            width={360}
          >
            <div style={{ marginBottom: 8 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {nameModalMode === "new"
                  ? "Enter a name for the new profile. The current form values will be saved."
                  : nameModalMode === "clone"
                  ? `Cloning "${selectedProfile}". Enter a name for the new profile.`
                  : `Renaming "${selectedProfile}". Enter the new name.`}
              </Text>
            </div>
            <Input
              autoFocus
              placeholder="profile-name"
              value={nameModalValue}
              onChange={(e) => setNameModalValue(e.target.value)}
              onPressEnter={confirmNameModal}
              status={
                nameModalValue.trim() && (!profileNameIsValid(nameModalValue.trim()) || nameModalHasDuplicate)
                  ? "error"
                  : undefined
              }
            />
            {nameModalValue.trim() && !profileNameIsValid(nameModalValue.trim()) && (
              <Text type="danger" style={{ fontSize: 11, marginTop: 4, display: "block" }}>
                Only letters, numbers, hyphens, and underscores are allowed.
              </Text>
            )}
            {nameModalHasDuplicate && (
              <Text type="danger" style={{ fontSize: 11, marginTop: 4, display: "block" }}>
                A profile named "{nameModalValue.trim()}" already exists.
              </Text>
            )}
          </Modal>
      </Space>
    </Modal>
  );
}
