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
import { Button, Input, Modal, Radio, Select, Switch, Tag, Typography, message } from "antd";
import { GetAIConfig, GetSystemRAMGB, ListAIModels, SaveAIConfig, TestAIModel } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

type Provider = "openai" | "google" | "ollama";

interface AIState {
  enabled:       boolean;
  provider:      Provider;
  apiKey:        string;
  model:         string;
  ollamaPort:    number; // 0 means default (11434)
  ollamaNumCtx:  number; // 0 means let Ollama decide (usually 4096)
}

// Context-window presets for Ollama. 0 = auto (Ollama default, usually 4K).
const CTX_OPTIONS = [
  { value: 0,      label: "Auto (Ollama default, ~4K)" },
  { value: 8192,   label: "8K" },
  { value: 16384,  label: "16K" },
  { value: 32768,  label: "32K  —  recommended for 8–16 GB RAM" },
  { value: 65536,  label: "64K" },
  { value: 131072, label: "128K  —  recommended for 32+ GB RAM" },
];

function recommendedNumCtx(ramGB: number): number {
  if (ramGB >= 32) return 131072;
  if (ramGB >= 8)  return 32768;
  return 8192;
}

const PROVIDER_LABEL: Record<Provider, string> = {
  openai: "OpenAI",
  google: "Google AI Studios",
  ollama: "Ollama (Local)",
};

const FALLBACK_MODELS: Record<Provider, string[]> = {
  openai: ["gpt-4o-mini", "gpt-4o"],
  google: ["gemini-2.0-flash-lite", "gemini-2.0-flash", "gemini-1.5-pro"],
  ollama: [],
};

const DEFAULT_MODEL: Record<Provider, string> = {
  openai: "gpt-4o-mini",
  google: "gemini-2.0-flash-lite",
  ollama: "",
};

export default function AISettingsModal({ onClose }: Props) {
  const [state, setState] = useState<AIState>({
    enabled:      false,
    provider:     "openai",
    apiKey:       "",
    model:        DEFAULT_MODEL.openai,
    ollamaPort:   0,
    ollamaNumCtx: 0,
  });
  const [saving, setSaving]       = useState(false);
  const [detectedRAM, setDetectedRAM] = useState(0);

  // Tracks the last-saved config so we can show "currently in use" info.
  const [savedConfig, setSavedConfig] = useState<{ provider: Provider; model: string } | null>(null);

  // Dynamic model list fetched from the provider API.
  const [fetchedModels, setFetchedModels] = useState<string[] | null>(null);
  const [modelsFetching, setModelsFetching] = useState(false);

  // Model connectivity test.
  const [modelTest, setModelTest] = useState<"idle" | "testing" | "ok" | "error">("idle");
  const [modelTestMsg, setModelTestMsg] = useState("");

  // Debounce timer refs.
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const testDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load saved config and detect system RAM on open.
  useEffect(() => {
    GetAIConfig().then((cfg) => {
      const provider = (cfg.provider as Provider) || "openai";
      const model = cfg.model || DEFAULT_MODEL[provider];
      setState({
        enabled:      cfg.enabled ?? false,
        provider,
        apiKey:       cfg.apiKey ?? "",
        model,
        ollamaPort:   cfg.ollamaPort ?? 0,
        ollamaNumCtx: cfg.ollamaNumCtx ?? 0,
      });
      setSavedConfig({ provider, model });
    });
    GetSystemRAMGB().then((gb) => setDetectedRAM(gb));
  }, []);

  // Fetch available models whenever provider or apiKey changes (debounced).
  // For Ollama (local), no API key is required — fetch immediately.
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    setFetchedModels(null);

    const isLocal = state.provider === "ollama";
    if (!isLocal && state.apiKey.length < 8) return; // key too short to be valid

    // cancelled flag prevents a stale in-flight fetch (e.g. a slow Google
    // request) from overwriting the model list after the provider was switched.
    let cancelled = false;

    debounceRef.current = setTimeout(async () => {
      setModelsFetching(true);
      try {
        const models = await ListAIModels(state.provider, state.apiKey, state.ollamaPort);
        if (cancelled) return;
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
        if (!cancelled) setModelsFetching(false);
      }
    }, isLocal ? 0 : 700);

    return () => {
      cancelled = true;
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [state.provider, state.apiKey, state.ollamaPort]);

  // Test model connectivity whenever provider, apiKey, or model changes.
  useEffect(() => {
    if (testDebounceRef.current) clearTimeout(testDebounceRef.current);
    setModelTest("idle");
    setModelTestMsg("");

    const isLocal = state.provider === "ollama";
    if (!isLocal && state.apiKey.length < 8) return;
    if (!state.model) return;

    let cancelledTest = false;

    testDebounceRef.current = setTimeout(async () => {
      setModelTest("testing");
      try {
        const errMsg = await TestAIModel(state.provider, state.apiKey, state.model, state.ollamaPort, state.ollamaNumCtx);
        if (cancelledTest) return;
        if (errMsg) {
          setModelTest("error");
          setModelTestMsg(errMsg);
        } else {
          setModelTest("ok");
          setModelTestMsg("");
        }
      } catch (e) {
        if (!cancelledTest) {
          setModelTest("error");
          setModelTestMsg(String(e));
        }
      }
    }, 800);

    return () => {
      cancelledTest = true;
      if (testDebounceRef.current) clearTimeout(testDebounceRef.current);
    };
  }, [state.provider, state.apiKey, state.model, state.ollamaNumCtx]);

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
        enabled:      state.enabled,
        provider:     state.provider,
        apiKey:       state.apiKey,
        model:        state.model,
        ollamaPort:   state.ollamaPort,
        ollamaNumCtx: state.ollamaNumCtx,
      } as any);
      setSavedConfig({ provider: state.provider, model: state.model });
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

        {/* Currently in use */}
        {savedConfig && (
          <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>In use:</Text>
            <Tag color="blue" style={{ margin: 0 }}>{PROVIDER_LABEL[savedConfig.provider]}</Tag>
            {savedConfig.model && (
              <Text type="secondary" style={{ fontSize: 12 }}>{savedConfig.model}</Text>
            )}
          </div>
        )}

        {/* Provider */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 6 }}>Provider</Text>
          <Radio.Group
            value={state.provider}
            onChange={(e) => setProvider(e.target.value as Provider)}
          >
            <Radio value="openai">OpenAI</Radio>
            <Radio value="google">Google AI Studios</Radio>
            <Radio value="ollama">Ollama (Local)</Radio>
          </Radio.Group>
        </div>

        {/* API Key — hidden for local Ollama */}
        {state.provider !== "ollama" && (
          <div>
            <Text type="secondary" style={{ display: "block", marginBottom: 6 }}>API Key</Text>
            <Input.Password
              value={state.apiKey}
              onChange={(e) => setState((s) => ({ ...s, apiKey: e.target.value }))}
              placeholder="Paste your API key here"
            />
          </div>
        )}

        {/* Ollama port — shown only for local Ollama */}
        {state.provider === "ollama" && (
          <div>
            <Text type="secondary" style={{ display: "block", marginBottom: 6 }}>Port</Text>
            <Input
              style={{ width: 120 }}
              value={state.ollamaPort || ""}
              onChange={(e) => {
                const v = parseInt(e.target.value, 10);
                setState((s) => ({ ...s, ollamaPort: isNaN(v) ? 0 : v }));
              }}
              placeholder="11434"
              suffix={<Text type="secondary" style={{ fontSize: 11 }}>default 11434</Text>}
            />
          </div>
        )}

        {/* Ollama context window — shown only for local Ollama */}
        {state.provider === "ollama" && (
          <div>
            <Text type="secondary" style={{ display: "block", marginBottom: 6 }}>
              Context window (num_ctx)
            </Text>
            <Select
              style={{ width: "100%" }}
              value={state.ollamaNumCtx}
              onChange={(v) => setState((s) => ({ ...s, ollamaNumCtx: v }))}
              options={CTX_OPTIONS}
            />
            {detectedRAM > 0 && (
              <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 5 }}>
                Detected RAM: {detectedRAM} GB
                {" — "}recommended:{" "}
                <Text
                  style={{ fontSize: 11, cursor: "pointer", color: "var(--link)" }}
                  onClick={() => setState((s) => ({ ...s, ollamaNumCtx: recommendedNumCtx(detectedRAM) }))}
                >
                  {(recommendedNumCtx(detectedRAM) / 1024).toFixed(0)}K (click to apply)
                </Text>
              </Text>
            )}
            <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 4 }}>
              Gemma 4 and other large models need a higher value than Ollama's default 4K.
              Higher values use more RAM/VRAM.
            </Text>
          </div>
        )}

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
        {state.provider === "ollama" ? (
          <Text type="secondary" style={{ fontSize: 12 }}>
            Ollama runs locally — no API key required. Make sure Ollama is running
            at <Text code style={{ fontSize: 12 }}>http://localhost:11434</Text>.
          </Text>
        ) : (
          <Text type="secondary" style={{ fontSize: 12 }}>
            Your API key is stored locally in{" "}
            <Text code style={{ fontSize: 12 }}>~/.config/thaw/config.json</Text>{" "}
            (permissions: 0600).
          </Text>
        )}
      </div>
    </Modal>
  );
}
