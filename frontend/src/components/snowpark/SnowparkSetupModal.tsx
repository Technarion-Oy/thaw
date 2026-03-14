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
import { Modal, Button, Steps, Typography, Alert, Space, Tag, Radio, Divider, Select } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import {
  IsAppleSilicon,
  GetSnowparkConfig,
  SaveSnowparkConfig,
  SaveSnowparkPythonPath,
  ListSystemPythons,
  InstallCondaEnv,
  InstallSnowparkPackage,
  InstallJupyterNotebook,
  InstallVenvEnv,
  InstallSnowparkVenv,
  InstallJupyterVenv,
  DeleteVenvFolder,
} from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { useGitStore } from "../../store/gitStore";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

type Backend = "conda" | "venv";
type StepStatus = "idle" | "running" | "done" | "error";

interface StepState {
  status: StepStatus;
  log: string[];
  error: string | null;
}

interface StepDef {
  title: string;
  description: string;
  command: string;
}

function makeStepState(): StepState {
  return { status: "idle", log: [], error: null };
}

function condaSteps(isM1: boolean): StepDef[] {
  return [
    {
      title: "Create Conda Environment",
      description: "thaw_snowpark  ·  Python 3.12",
      command: isM1
        ? "CONDA_SUBDIR=osx-64 conda create -n thaw_snowpark -y \\\n  --override-channels -c https://repo.anaconda.com/pkgs/snowflake \\\n  python=3.12 numpy pandas pyarrow"
        : "conda create -n thaw_snowpark -y \\\n  --override-channels -c https://repo.anaconda.com/pkgs/snowflake \\\n  python=3.12 numpy pandas pyarrow",
    },
    {
      title: "Install Snowpark",
      description: "snowflake-snowpark-python  ·  pandas  ·  pyarrow",
      command: "conda install -n thaw_snowpark -y snowflake-snowpark-python pandas pyarrow",
    },
    {
      title: "Install Jupyter & SQL",
      description: "notebook  ·  ipython-sql  ·  sqlalchemy  (via pip)",
      command: "conda run -n thaw_snowpark pip install notebook ipython-sql sqlalchemy",
    },
  ];
}

function venvSteps(venvPath: string, withPandas: boolean, pythonBin: string): StepDef[] {
  return [
    {
      title: "Create Virtual Environment",
      description: venvPath,
      command: `"${pythonBin || "python3"}" -m venv "${venvPath}"`,
    },
    {
      title: "Install Snowpark",
      description: withPandas
        ? "snowflake-snowpark-python[pandas]"
        : "snowflake-snowpark-python",
      command: withPandas
        ? `"${venvPath}/bin/pip" install "snowflake-snowpark-python[pandas]"`
        : `"${venvPath}/bin/pip" install snowflake-snowpark-python`,
    },
    {
      title: "Install Jupyter & SQL",
      description: "notebook  ·  ipython-sql  ·  sqlalchemy  (via pip)",
      command: `"${venvPath}/bin/pip" install notebook ipython-sql sqlalchemy`,
    },
  ];
}

export default function SnowparkSetupModal({ onClose }: Props) {
  const [backend, setBackend]       = useState<Backend>("conda");
  const [withPandas, setWithPandas] = useState(true);
  const [isAppleSilicon, setIsAppleSilicon] = useState(false);
  const [venvPath, setVenvPath]     = useState("");
  const [pythonPath, setPythonPath] = useState("");
  const [availablePythons, setAvailablePythons] = useState<main.PythonInfo[]>([]);
  const [current, setCurrent]       = useState(0);
  const [steps, setSteps]           = useState<StepState[]>([makeStepState(), makeStepState(), makeStepState()]);
  const [, setConfigLoaded] = useState(false);
  const logEndRef = useRef<HTMLDivElement | null>(null);

  const exportDir    = useGitStore((s) => s.exportDir);
  const gitConfigLoaded = useGitStore((s) => s.configLoaded);
  const loadConfig   = useGitStore((s) => s.loadConfig);

  // Load saved config + detect machine on mount.
  useEffect(() => {
    IsAppleSilicon().then(setIsAppleSilicon);
    if (!gitConfigLoaded) loadConfig();
    GetSnowparkConfig().then((cfg) => {
      setBackend((cfg.backend as Backend) || "conda");
      setVenvPath(cfg.venvPath);
      setPythonPath(cfg.pythonPath || "");
      setConfigLoaded(true);
    });
    ListSystemPythons().then(setAvailablePythons);
  }, []);

  // Reset steps when backend changes.
  const handleBackendChange = (b: Backend) => {
    setBackend(b);
    setCurrent(0);
    setSteps([makeStepState(), makeStepState(), makeStepState()]);
  };

  // Stream install output from Go backend.
  useEffect(() => {
    const off = EventsOn("snowpark:install-output", (line: string) => {
      setSteps((prev) => {
        const next = [...prev];
        next[current] = { ...next[current], log: [...next[current].log, line] };
        return next;
      });
    });
    return () => (off as () => void)();
  }, [current]);

  // Auto-scroll log.
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [steps[current].log]);

  const patch = (idx: number, p: Partial<StepState>) =>
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, ...p } : s)));

  const runStep = async (idx: number) => {
    patch(idx, { status: "running", log: [], error: null });
    // Persist the backend choice on the first step so subsequent checks use it.
    if (idx === 0) await SaveSnowparkConfig(backend).catch(() => {});
    try {
      if (backend === "conda") {
        await [InstallCondaEnv, InstallSnowparkPackage, InstallJupyterNotebook][idx]();
      } else {
        const venvRunners = [
          () => InstallVenvEnv(),
          () => InstallSnowparkVenv(withPandas),
          () => InstallJupyterVenv(),
        ];
        await venvRunners[idx]();
      }
      patch(idx, { status: "done" });
      if (idx < 2) setCurrent(idx + 1);
    } catch (e) {
      patch(idx, { status: "error", error: String(e) });
    }
  };

  const handleDeleteVenv = () => {
    const venvPathDisplay = venvPath || `${exportDir || "~"}/snowpark_venv`;
    Modal.confirm({
      title: "Delete venv folder?",
      content: `This will permanently delete the virtual environment at:\n${venvPathDisplay}`,
      okText: "Delete",
      okButtonProps: { danger: true },
      onOk: async () => {
        await DeleteVenvFolder();
        setCurrent(0);
        setSteps([makeStepState(), makeStepState(), makeStepState()]);
      },
    });
  };

  const antdStatus = (s: StepStatus): "process" | "finish" | "error" | "wait" =>
    s === "running" ? "process" : s === "done" ? "finish" : s === "error" ? "error" : "wait";

  const stepIcons = steps.map((s) =>
    s.status === "running" ? <LoadingOutlined />
    : s.status === "done"  ? <CheckCircleOutlined />
    : s.status === "error" ? <CloseCircleOutlined />
    : undefined,
  );

  const defs   = backend === "conda" ? condaSteps(isAppleSilicon) : venvSteps(venvPath, withPandas, pythonPath);
  const cur    = steps[current];
  const allDone = steps.every((s) => s.status === "done");
  const anyRunning = steps.some((s) => s.status === "running");

  return (
    <Modal
      title="Snowpark Environment Setup"
      open
      onCancel={onClose}
      width={620}
      footer={[
        allDone
          ? <Button key="done" type="primary" onClick={onClose}>Done</Button>
          : <Button key="close" onClick={onClose}>Cancel</Button>,
      ]}
    >
      <Space direction="vertical" style={{ width: "100%" }} size={14}>

        {/* ── Backend choice ─────────────────────────────────────────────── */}
        <div>
          <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 6 }}>
            Environment type
          </Text>
          <Radio.Group
            value={backend}
            onChange={(e) => handleBackendChange(e.target.value as Backend)}
            disabled={anyRunning}
          >
            <Radio value="conda">conda</Radio>
            <Radio value="venv">venv  <Text type="secondary" style={{ fontSize: 11 }}>(uses system Python)</Text></Radio>
          </Radio.Group>
        </div>

        {/* ── Pandas option (venv only) ───────────────────────────────────── */}
        {backend === "venv" && (
          <div>
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 6 }}>
              Snowpark extras
            </Text>
            <Radio.Group
              value={withPandas}
              onChange={(e) => setWithPandas(e.target.value as boolean)}
              disabled={anyRunning || steps[1].status === "done"}
            >
              <Radio value={true}>
                Include pandas{" "}
                <Text type="secondary" style={{ fontSize: 11 }}>
                  (pip install "snowflake-snowpark-python[pandas]")
                </Text>
              </Radio>
              <Radio value={false}>
                Without pandas{" "}
                <Text type="secondary" style={{ fontSize: 11 }}>
                  (pip install snowflake-snowpark-python)
                </Text>
              </Radio>
            </Radio.Group>
          </div>
        )}

        {/* ── Python interpreter (venv only) ─────────────────────────────── */}
        {backend === "venv" && (
          <div>
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 6 }}>
              Python interpreter
            </Text>
            <Select
              style={{ width: "100%" }}
              size="small"
              value={pythonPath || undefined}
              placeholder="Auto-detect (python3 on PATH)"
              allowClear
              disabled={anyRunning || steps[0].status === "done"}
              onChange={(val: string | undefined) => {
                const v = val ?? "";
                setPythonPath(v);
                SaveSnowparkPythonPath(v).catch(() => {});
              }}
              options={availablePythons.map((p) => ({
                value: p.path,
                label: `Python ${p.version}  —  ${p.path}`,
              }))}
            />
          </div>
        )}

        <Divider style={{ margin: "2px 0" }} />

        {/* ── Apple Silicon warning (conda only) ─────────────────────────── */}
        {backend === "conda" && isAppleSilicon && (
          <Alert
            type="warning"
            icon={<WarningOutlined />}
            showIcon
            message="Apple Silicon detected"
            description={
              "The conda environment will be created with CONDA_SUBDIR=osx-64 " +
              "to work around a known pyOpenSSL incompatibility on Apple M-series chips."
            }
            style={{ fontSize: 12 }}
          />
        )}

        {/* ── Project directory ──────────────────────────────────────────── */}
        <div style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "6px 10px",
          background: "var(--bg-raised)",
          borderRadius: 6,
          border: "1px solid var(--border)",
        }}>
          <Text type="secondary" style={{ fontSize: 12, whiteSpace: "nowrap" }}>
            Project directory:
          </Text>
          <Text
            style={{ fontSize: 12, fontFamily: "monospace", flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
            title={exportDir || undefined}
          >
            {exportDir || (
              <Text type="secondary" style={{ fontSize: 12 }}>Not set — configure in the DDL Export panel</Text>
            )}
          </Text>
        </div>

        {/* ── Delete venv (venv only) ────────────────────────────────────── */}
        {backend === "venv" && (
          <div style={{ display: "flex", justifyContent: "flex-end" }}>
            <Button
              danger
              size="small"
              disabled={anyRunning}
              onClick={handleDeleteVenv}
            >
              Delete venv folder…
            </Button>
          </div>
        )}

        {/* ── Steps ──────────────────────────────────────────────────────── */}
        <Steps
          current={current}
          size="small"
          items={defs.map((s, i) => ({
            title: s.title,
            description: s.description,
            status: antdStatus(steps[i].status),
            icon: stepIcons[i],
          }))}
        />

        {/* ── Command + log ──────────────────────────────────────────────── */}
        <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
          <div style={{
            padding: "6px 10px",
            background: "var(--bg-raised)",
            borderBottom: "1px solid var(--border)",
            fontFamily: "monospace",
            fontSize: 11,
            color: "var(--text-muted)",
            whiteSpace: "pre",
          }}>
            {defs[current].command}
          </div>
          <div style={{
            height: 180,
            overflowY: "auto",
            padding: "6px 10px",
            background: "var(--bg)",
            fontFamily: "monospace",
            fontSize: 11,
          }}>
            {cur.log.length === 0 && cur.status === "idle" && (
              <Text type="secondary" style={{ fontSize: 11 }}>Press Run to start this step.</Text>
            )}
            {cur.log.map((line, i) => (
              <div key={i} style={{ whiteSpace: "pre-wrap", lineHeight: "1.5", color: "var(--text)" }}>
                {line}
              </div>
            ))}
            <div ref={logEndRef} />
          </div>
        </div>

        {/* ── Error ──────────────────────────────────────────────────────── */}
        {cur.error && (
          <Alert type="error" message={cur.error} showIcon style={{ fontSize: 12 }} />
        )}

        {/* ── Step controls ──────────────────────────────────────────────── */}
        {!allDone && (
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <Button
              type="primary"
              loading={cur.status === "running"}
              disabled={cur.status === "done"}
              onClick={() => runStep(current)}
            >
              {cur.status === "idle"    ? "Run"
               : cur.status === "running" ? "Running…"
               : cur.status === "error"   ? "Retry"
               : "Done"}
            </Button>
            {current > 0 && cur.status !== "running" && (
              <Button onClick={() => setCurrent((c) => c - 1)}>Back</Button>
            )}
            {cur.status === "done" && current < 2 && (
              <Button onClick={() => setCurrent((c) => c + 1)}>Next step</Button>
            )}
          </div>
        )}
        {allDone && (
          <Tag color="success" style={{ fontSize: 12 }}>
            ✓ All steps completed — environment is ready
          </Tag>
        )}

      </Space>
    </Modal>
  );
}
