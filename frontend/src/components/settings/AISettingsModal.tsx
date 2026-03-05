// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useRef, useState } from "react";
import { Button, Input, Modal, Radio, Select, Switch, Typography, message } from "antd";
import { GetAIConfig, ListAIModels, SaveAIConfig, TestAIModel } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

type Provider = "openai" | "google";

interface AIState {
  enabled:  boolean;
  provider: Provider;
  apiKey:   string;
  model:    string;
}

const FALLBACK_MODELS: Record<Provider, string[]> = {
  openai: ["gpt-4o-mini", "gpt-4o"],
  google: ["gemini-2.0-flash-lite", "gemini-2.0-flash", "gemini-1.5-pro"],
};

const DEFAULT_MODEL: Record<Provider, string> = {
  openai: "gpt-4o-mini",
  google: "gemini-2.0-flash-lite",
};

export default function AISettingsModal({ onClose }: Props) {
  const [state, setState] = useState<AIState>({
    enabled:  false,
    provider: "openai",
    apiKey:   "",
    model:    DEFAULT_MODEL.openai,
  });
  const [saving, setSaving] = useState(false);

  // Dynamic model list fetched from the provider API.
  const [fetchedModels, setFetchedModels] = useState<string[] | null>(null);
  const [modelsFetching, setModelsFetching] = useState(false);

  // Model connectivity test.
  const [modelTest, setModelTest] = useState<"idle" | "testing" | "ok" | "error">("idle");
  const [modelTestMsg, setModelTestMsg] = useState("");

  // Debounce timer refs.
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const testDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load saved config on open.
  useEffect(() => {
    GetAIConfig().then((cfg) => {
      const provider = (cfg.provider as Provider) || "openai";
      setState({
        enabled:  cfg.enabled ?? false,
        provider,
        apiKey:   cfg.apiKey ?? "",
        model:    cfg.model || DEFAULT_MODEL[provider],
      });
    });
  }, []);

  // Fetch available models whenever provider or apiKey changes (debounced).
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    setFetchedModels(null);

    if (state.apiKey.length < 8) return; // key too short to be valid

    debounceRef.current = setTimeout(async () => {
      setModelsFetching(true);
      try {
        const models = await ListAIModels(state.provider, state.apiKey);
        if (models && models.length > 0) {
          setFetchedModels(models);
          // If the currently selected model isn't in the fetched list, reset to
          // the first available one.
          setState((s) => ({
            ...s,
            model: models.includes(s.model) ? s.model : models[0],
          }));
        }
      } finally {
        setModelsFetching(false);
      }
    }, 700);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [state.provider, state.apiKey]);

  // Test model connectivity whenever provider, apiKey, or model changes.
  useEffect(() => {
    if (testDebounceRef.current) clearTimeout(testDebounceRef.current);
    setModelTest("idle");
    setModelTestMsg("");

    if (state.apiKey.length < 8 || !state.model) return;

    testDebounceRef.current = setTimeout(async () => {
      setModelTest("testing");
      try {
        const errMsg = await TestAIModel(state.provider, state.apiKey, state.model);
        if (errMsg) {
          setModelTest("error");
          setModelTestMsg(errMsg);
        } else {
          setModelTest("ok");
          setModelTestMsg("");
        }
      } catch (e) {
        setModelTest("error");
        setModelTestMsg(String(e));
      }
    }, 800);

    return () => {
      if (testDebounceRef.current) clearTimeout(testDebounceRef.current);
    };
  }, [state.provider, state.apiKey, state.model]);

  function setProvider(provider: Provider) {
    setState((s) => ({ ...s, provider, model: DEFAULT_MODEL[provider] }));
    setFetchedModels(null);
  }

  const modelOptions = (fetchedModels ?? FALLBACK_MODELS[state.provider]).map((m) => ({
    value: m,
    label: m,
  }));

  async function handleSave() {
    setSaving(true);
    try {
      await SaveAIConfig({
        enabled:  state.enabled,
        provider: state.provider,
        apiKey:   state.apiKey,
        model:    state.model,
      } as any);
      message.success("AI settings saved");
      onClose();
    } catch (err) {
      message.error(String(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      title="Configure AI"
      open
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="save" type="primary" loading={saving} onClick={handleSave}>Save</Button>,
      ]}
      width={480}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 16, paddingTop: 8 }}>

        {/* Enable toggle */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <Text>Enable AI suggestions</Text>
          <Switch
            checked={state.enabled}
            onChange={(v) => setState((s) => ({ ...s, enabled: v }))}
          />
        </div>

        {/* Provider */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 6 }}>Provider</Text>
          <Radio.Group
            value={state.provider}
            onChange={(e) => setProvider(e.target.value as Provider)}
          >
            <Radio value="openai">OpenAI</Radio>
            <Radio value="google">Google AI Studios</Radio>
          </Radio.Group>
        </div>

        {/* API Key */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 6 }}>API Key</Text>
          <Input.Password
            value={state.apiKey}
            onChange={(e) => setState((s) => ({ ...s, apiKey: e.target.value }))}
            placeholder="Paste your API key here"
          />
        </div>

        {/* Model */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 6 }}>
            Model
            {modelsFetching && (
              <Text type="secondary" style={{ marginLeft: 8, fontSize: 11 }}>
                fetching…
              </Text>
            )}
            {!modelsFetching && fetchedModels && (
              <Text type="secondary" style={{ marginLeft: 8, fontSize: 11 }}>
                ({fetchedModels.length} available)
              </Text>
            )}
          </Text>
          <Select
            style={{ width: "100%" }}
            value={state.model}
            loading={modelsFetching}
            onChange={(v) => setState((s) => ({ ...s, model: v }))}
            options={modelOptions}
          />
          {modelTest !== "idle" && (
            <div style={{ marginTop: 5, fontSize: 11, display: "flex", alignItems: "center", gap: 4 }}>
              {modelTest === "testing" && (
                <Text type="secondary">Testing model…</Text>
              )}
              {modelTest === "ok" && (
                <span style={{ color: "#52c41a" }}>● Model OK</span>
              )}
              {modelTest === "error" && (
                <span style={{ color: "#f5222d" }} title={modelTestMsg}>
                  ● {modelTestMsg}
                </span>
              )}
            </div>
          )}
        </div>

        {/* Storage note */}
        <Text type="secondary" style={{ fontSize: 12 }}>
          Your API key is stored locally in{" "}
          <Text code style={{ fontSize: 12 }}>~/.config/thaw/config.json</Text>{" "}
          (permissions: 0600).
        </Text>
      </div>
    </Modal>
  );
}
