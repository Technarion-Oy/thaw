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
import { Modal, Button, Steps, Typography, Alert, Space, Tag, Radio, Divider, Select, Input, List, Spin } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExportOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  LoadingOutlined,
  SettingOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import PipRegistryModal from "./PipRegistryModal";
import {
  IsAppleSilicon,
  GetSnowparkConfig,
  SaveSnowparkConfig,
  SaveSnowparkPythonPath,
  ListSystemPythons,
  CheckSnowparkEnv,
  InstallCondaEnv,
  InstallSnowparkPackage,
  InstallJupyterNotebook,
  InstallVenvEnv,
  InstallSnowparkVenv,
  InstallJupyterVenv,
  DeleteVenvFolder,
  SaveSnowparkVenvPath,
  VenvFolderExists,
  ListEnvPackages,
  InstallEnvPackage,
  UninstallEnvPackage,
  PickRequirementsFile,
  PickPyprojectFile,
  InstallRequirementsFile,
  InstallPyprojectFile,
  FreezeRequirements,
  PickDirectory,
} from "../../../wailsjs/go/app/App";
import type { snowpark } from "../../../wailsjs/go/models";
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

function stepsFromEnvCheck(env: snowpark.SnowparkCheckResult): { steps: StepState[]; firstIncomplete: number } {
  const newSteps: StepState[] = [
    { status: "done", log: [], error: null },
    { status: env.hasSnowpark ? "done" : "idle", log: [], error: null },
    { status: env.hasNotebook ? "done" : "idle", log: [], error: null },
  ];
  const first = newSteps.findIndex((s) => s.status !== "done");
  return { steps: newSteps, firstIncomplete: first === -1 ? 3 : first };
}

function CheckItem({ ok, label }: { ok: boolean; label: string }) {
  return (
    <div>
      {ok
        ? <CheckCircleOutlined style={{ color: "#52c41a" }} />
        : <CloseCircleOutlined style={{ color: "#ff4d4f" }} />}
      {" "}{label}
    </div>
  );
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
      description: "notebook  ·  ipython-sql  ·  sqlalchemy  ·  pyflakes  (via pip)",
      command: "conda run -n thaw_snowpark pip install notebook ipython-sql sqlalchemy pyflakes",
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
      description: "notebook  ·  ipython-sql  ·  sqlalchemy  ·  pyflakes  (via pip)",
      command: `"${venvPath}/bin/pip" install notebook ipython-sql sqlalchemy pyflakes`,
    },
  ];
}

export default function SnowparkSetupModal({ onClose }: Props) {
  const [backend, setBackend]         = useState<Backend>("conda");
  const [withPandas, setWithPandas]   = useState(true);
  const [registryOpen, setRegistryOpen] = useState(false);
  const [isAppleSilicon, setIsAppleSilicon] = useState(false);
  const [venvPath, setVenvPath]     = useState("");
  const [pythonPath, setPythonPath] = useState("");
  const [availablePythons, setAvailablePythons] = useState<snowpark.PythonInfo[]>([]);
  const [current, setCurrent]       = useState(0);
  const [steps, setSteps]           = useState<StepState[]>([makeStepState(), makeStepState(), makeStepState()]);
  const [, setConfigLoaded] = useState(false);
  const [venvFolderExists, setVenvFolderExists] = useState(false);
  const [useExisting, setUseExisting] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<snowpark.SnowparkCheckResult | null>(null);
  const logEndRef = useRef<HTMLDivElement | null>(null);

  // Package management (step 3)
  const [packages, setPackages]           = useState<snowpark.PackageInfo[]>([]);
  const [packagesLoading, setPackagesLoading] = useState(false);
  const [packagesError, setPackagesError] = useState<string | null>(null);
  const [packageInput, setPackageInput]   = useState("");
  const [packageLog, setPackageLog]       = useState<string[]>([]);
  const [packageOpRunning, setPackageOpRunning] = useState(false);
  const [uninstallingPkg, setUninstallingPkg] = useState<string | null>(null);
  // Which dependency-file operation is in flight (drives per-button spinners); null = idle.
  const [depFileOp, setDepFileOp] = useState<"requirements" | "pyproject" | "freeze" | null>(null);
  const depFileRunning = depFileOp !== null;
  const pkgLogEndRef = useRef<HTMLDivElement | null>(null);

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
    // If the environment is already fully set up, pre-mark all steps done
    // and go straight to the package manager. If partially configured (has
    // venv but missing packages), auto-enter "use existing" mode.
    CheckSnowparkEnv().then((env) => {
      if (env.isReady) {
        setSteps([
          { status: "done", log: [], error: null },
          { status: "done", log: [], error: null },
          { status: "done", log: [], error: null },
        ]);
        setCurrent(3);
      } else if (env.hasVenv) {
        setUseExisting(true);
        setValidationResult(env);
        setVenvFolderExists(true);
        const result = stepsFromEnvCheck(env);
        setSteps(result.steps);
        setCurrent(result.firstIncomplete);
      }
    });
  }, []);

  // Check whether the venv folder exists on disk.
  useEffect(() => {
    if (backend !== "venv") return;
    VenvFolderExists().then(setVenvFolderExists).catch(() => setVenvFolderExists(false));
  }, [backend, venvPath]);

  // Reset steps when backend changes.
  const handleBackendChange = (b: Backend) => {
    setBackend(b);
    setCurrent(0);
    setSteps([makeStepState(), makeStepState(), makeStepState()]);
    setUseExisting(false);
    setValidationResult(null);
  };

  const handleBrowseVenv = async () => {
    let dir: string;
    try {
      dir = await PickDirectory();
    } catch {
      return;
    }
    if (!dir || dir === venvPath) return;
    setVenvPath(dir);
    await SaveSnowparkVenvPath(dir).catch(() => {});
    setValidationResult(null);
    if (useExisting) {
      setUseExisting(false);
      setCurrent(0);
      setSteps([makeStepState(), makeStepState(), makeStepState()]);
    }
  };

  const handleUseExisting = async () => {
    setValidating(true);
    setValidationResult(null);
    patch(0, { status: "idle", error: null });
    const trimmed = venvPath.trim();
    try {
      await SaveSnowparkVenvPath(trimmed);
      await SaveSnowparkConfig(backend);
      setVenvPath(trimmed);
      const env = await CheckSnowparkEnv();
      setValidationResult(env);
      if (!env.hasVenv) {
        // Path is invalid — show error but don't switch to "use existing" mode.
        return;
      }
      setUseExisting(true);
      setVenvFolderExists(true);
      const result = stepsFromEnvCheck(env);
      setSteps(result.steps);
      setCurrent(result.firstIncomplete);
    } catch (e) {
      setValidationResult(null);
      setSteps((prev) => {
        const next = [...prev];
        next[0] = { ...next[0], status: "error", error: String(e) };
        return next;
      });
    } finally {
      setValidating(false);
    }
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

  // Auto-scroll install log.
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [steps[Math.min(current, 2)].log]);

  // Stream package operation output.
  useEffect(() => {
    const off = EventsOn("snowpark:package-output", (line: string) => {
      setPackageLog((prev) => [...prev, line]);
    });
    return () => (off as () => void)();
  }, []);

  // Auto-scroll package log.
  useEffect(() => {
    pkgLogEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [packageLog]);

  // Load installed packages when entering step 3.
  useEffect(() => {
    if (current !== 3) return;
    setPackagesLoading(true);
    setPackagesError(null);
    ListEnvPackages()
      .then((pkgs) => { setPackages(pkgs); setPackagesError(null); })
      .catch((e) => { setPackages([]); setPackagesError(String(e)); })
      .finally(() => setPackagesLoading(false));
  }, [current]);

  const patch = (idx: number, p: Partial<StepState>) =>
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, ...p } : s)));

  const runStep = async (idx: number) => {
    if (useExisting && idx === 0) return; // existing venv — use handleUseExisting instead
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
      if (backend === "venv" && idx === 0) VenvFolderExists().then(setVenvFolderExists).catch(() => {});
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
        setVenvFolderExists(false);
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

  // refreshPackages reloads the package list without letting a transient list
  // failure masquerade as an operation failure (it surfaces via packagesError).
  const refreshPackages = async () => {
    try {
      setPackages(await ListEnvPackages());
      setPackagesError(null);
    } catch (e) {
      setPackagesError(String(e));
    }
  };

  const handleInstallPackage = async () => {
    const pkg = packageInput.trim();
    if (!pkg) return;
    setPackageOpRunning(true);
    setPackageLog([]);
    try {
      await InstallEnvPackage(pkg);
      setPackageInput("");
      await refreshPackages();
    } catch (e) {
      setPackageLog((prev) => [...prev, String(e)]);
    } finally {
      setPackageOpRunning(false);
    }
  };

  const handleUninstallPackage = (name: string) => {
    Modal.confirm({
      title: `Uninstall ${name}?`,
      content: `This will remove ${name} from the Snowpark environment.`,
      okText: "Uninstall",
      okButtonProps: { danger: true },
      onOk: async () => {
        setUninstallingPkg(name);
        setPackageLog([]);
        try {
          await UninstallEnvPackage(name);
          await refreshPackages();
        } catch (e) {
          setPackageLog((prev) => [...prev, String(e)]);
        } finally {
          setUninstallingPkg(null);
        }
      },
    });
  };

  const handleInstallRequirements = async () => {
    // Capture the current log so a cancelled picker restores it rather than
    // leaving the panel blank with no indication the action was a no-op.
    const prevLog = packageLog;
    setDepFileOp("requirements");
    setPackageLog([]);
    try {
      const path = await PickRequirementsFile();
      if (!path) {
        setPackageLog(prevLog);
        return;
      }
      setPackageLog([`$ pip install -r ${path}`]);
      await InstallRequirementsFile(path);
      await refreshPackages();
    } catch (e) {
      setPackageLog((prev) => [...prev, String(e)]);
    } finally {
      setDepFileOp(null);
    }
  };

  const handleInstallPyproject = async () => {
    const prevLog = packageLog;
    setDepFileOp("pyproject");
    setPackageLog([]);
    try {
      const path = await PickPyprojectFile();
      if (!path) {
        setPackageLog(prevLog);
        return;
      }
      // pip is given the directory containing the file, not the file itself.
      setPackageLog([`Installing project from ${path}…`]);
      await InstallPyprojectFile(path);
      await refreshPackages();
    } catch (e) {
      setPackageLog((prev) => [...prev, String(e)]);
    } finally {
      setDepFileOp(null);
    }
  };

  const handleFreezeRequirements = async () => {
    // Capture the current log so a cancelled save dialog can restore it rather
    // than leaving the "$ pip freeze…" placeholder stranded in the panel.
    const prevLog = packageLog;
    setDepFileOp("freeze");
    setPackageLog(["$ pip freeze…"]);
    try {
      const written = await FreezeRequirements();
      setPackageLog(written ? [`✓ Wrote requirements to ${written}`] : prevLog);
    } catch (e) {
      setPackageLog((prev) => [...prev, String(e)]);
    } finally {
      setDepFileOp(null);
    }
  };

  const rawDefs = backend === "conda" ? condaSteps(isAppleSilicon) : venvSteps(venvPath, withPandas, pythonPath);
  const defs = useExisting
    ? rawDefs.map((d, i) =>
        i === 0
          ? { ...d, title: "Detect Virtual Environment", command: `# Existing venv detected at "${venvPath}"` }
          : d,
      )
    : rawDefs;
  const setupDone = steps.every((s) => s.status === "done");
  // cur is only valid for setup steps 0-2; use a safe fallback for step 3.
  const cur    = steps[Math.min(current, 2)];
  const anyRunning = steps.some((s) => s.status === "running");

  return (
    <Modal
      title="Snowpark Environment Setup"
      open
      onCancel={onClose}
      width={620}
      footer={[
        current === 3
          ? <Button key="back" onClick={() => setCurrent(2)} style={{ float: "left" }}>Back</Button>
          : null,
        current < 3
          ? <Button key="manage" onClick={() => setCurrent(3)}>Manage Packages</Button>
          : null,
        setupDone
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
            disabled={anyRunning || validating}
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
              disabled={anyRunning || validating || steps[1].status === "done"}
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

        {/* ── Venv location (venv only) ──────────────────────────────────── */}
        {backend === "venv" && (
          <div>
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 6 }}>
              Virtual environment location
            </Text>
            <div style={{ display: "flex", gap: 8 }}>
              <Input
                size="small"
                value={venvPath}
                disabled={anyRunning || validating || steps[0].status === "done"}
                style={{ fontFamily: "monospace", fontSize: 12, flex: 1 }}
                onChange={(e) => {
                  setVenvPath(e.target.value);
                  setValidationResult(null);
                  if (useExisting) {
                    setUseExisting(false);
                    setCurrent(0);
                    setSteps([makeStepState(), makeStepState(), makeStepState()]);
                  }
                }}
                onBlur={() => {
                  const trimmed = venvPath.trim();
                  if (trimmed !== venvPath) setVenvPath(trimmed);
                  SaveSnowparkVenvPath(trimmed).catch(() => {});
                }}
                onPressEnter={(e) => {
                  (e.target as HTMLInputElement).blur();
                }}
              />
              <Button
                size="small"
                icon={<FolderOpenOutlined />}
                disabled={anyRunning || validating || steps[0].status === "done"}
                onClick={handleBrowseVenv}
              >
                Browse
              </Button>
            </div>
          </div>
        )}

        {/* ── Use Existing venv (venv only) ─────────────────────────────── */}
        {backend === "venv" && (
          <div>
            {!useExisting && (
              <Button
                size="small"
                loading={validating}
                disabled={anyRunning || !venvPath.trim()}
                onClick={handleUseExisting}
              >
                Use Existing
              </Button>
            )}
            {validationResult && !validationResult.hasVenv && current === 0 && (
              <Alert
                type="error"
                showIcon
                style={{ fontSize: 12, marginTop: 8 }}
                message="No virtual environment found at this path."
              />
            )}
            {validationResult && validationResult.hasVenv && current === 0 && (
              <Alert
                type={validationResult.isReady ? "success" : "info"}
                showIcon
                style={{ fontSize: 12, marginTop: 8 }}
                message={
                  <div style={{ fontSize: 12 }}>
                    <div style={{ marginBottom: 4 }}>
                      {validationResult.version
                        ? `Python ${validationResult.version} detected`
                        : "Virtual environment detected"}
                    </div>
                    <CheckItem ok={validationResult.hasSnowpark} label="snowflake-snowpark-python" />
                    <CheckItem ok={validationResult.hasNotebook} label="notebook" />
                  </div>
                }
              />
            )}
          </div>
        )}

        {/* ── Python interpreter (venv only, create-new mode) ─────────────── */}
        {backend === "venv" && !useExisting && (
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

        {/* ── pip Registry ────────────────────────────────────────────── */}
        <div style={{ display: "flex", justifyContent: "flex-end" }}>
          <Button
            size="small"
            icon={<SettingOutlined />}
            onClick={() => setRegistryOpen(true)}
          >
            Configure pip Registry…
          </Button>
        </div>

        <Text type="secondary" style={{ fontSize: 11 }}>
          This wizard does not cover all{" "}
          {backend === "conda" ? "conda" : "venv"} operations. See the{" "}
          {backend === "conda" ? (
            <a href="https://docs.conda.io/projects/conda/en/stable/" target="_blank" rel="noreferrer">
              conda documentation
            </a>
          ) : (
            <a href="https://docs.python.org/3/library/venv.html" target="_blank" rel="noreferrer">
              Python venv documentation
            </a>
          )}{" "}
          for the full reference.
        </Text>

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

        {/* ── Delete venv / Create new (venv only) ──────────────────────── */}
        {backend === "venv" && (
          <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
            {useExisting && (
              <Button
                size="small"
                disabled={anyRunning}
                onClick={() => {
                  setUseExisting(false);
                  setValidationResult(null);
                  setCurrent(0);
                  setSteps([makeStepState(), makeStepState(), makeStepState()]);
                }}
              >
                Create New Instead
              </Button>
            )}
            {!useExisting && (
              <Button
                danger
                size="small"
                disabled={anyRunning || !venvFolderExists}
                onClick={handleDeleteVenv}
              >
                Delete venv folder…
              </Button>
            )}
          </div>
        )}

        {/* ── Steps ──────────────────────────────────────────────────────── */}
        <Steps
          current={current}
          size="small"
          items={[
            ...defs.map((s, i) => ({
              title: s.title,
              description: s.description,
              status: antdStatus(steps[i].status),
              icon: stepIcons[i],
            })),
            {
              title: "Manage Packages",
              description: "Install or remove libraries",
              status: current === 3 ? "process" as const : "wait" as const,
            },
          ]}
          onChange={(idx) => {
            if (idx < 3) setCurrent(idx);
            else setCurrent(3);
          }}
        />

        {/* ── Package manager (step 3) ────────────────────────────────────── */}
        {current === 3 && (
          <Space direction="vertical" style={{ width: "100%" }} size={10}>
            {/* Install input */}
            <div style={{ display: "flex", gap: 8 }}>
              <Input
                placeholder="Package name (e.g. scikit-learn)"
                size="small"
                value={packageInput}
                onChange={(e) => setPackageInput(e.target.value)}
                onPressEnter={handleInstallPackage}
                disabled={packageOpRunning || depFileRunning}
                style={{ flex: 1, fontFamily: "monospace", fontSize: 12 }}
              />
              <Button
                type="primary"
                size="small"
                loading={packageOpRunning && !uninstallingPkg}
                disabled={!packageInput.trim() || !!uninstallingPkg || depFileRunning}
                onClick={handleInstallPackage}
              >
                Install
              </Button>
            </div>

            {/* Dependency files */}
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Button
                size="small"
                icon={<FileTextOutlined />}
                loading={depFileOp === "requirements"}
                disabled={packageOpRunning || !!uninstallingPkg || depFileRunning}
                onClick={handleInstallRequirements}
              >
                Install requirements.txt
              </Button>
              <Button
                size="small"
                icon={<FileTextOutlined />}
                loading={depFileOp === "pyproject"}
                disabled={packageOpRunning || !!uninstallingPkg || depFileRunning}
                onClick={handleInstallPyproject}
              >
                Install pyproject.toml
              </Button>
              <Button
                size="small"
                icon={<ExportOutlined />}
                loading={depFileOp === "freeze"}
                disabled={packageOpRunning || !!uninstallingPkg || depFileRunning || packages.length === 0}
                onClick={handleFreezeRequirements}
              >
                Freeze to requirements.txt
              </Button>
            </div>

            {/* Output log */}
            {(packageLog.length > 0 || packageOpRunning || depFileRunning) && (
              <div style={{
                border: "1px solid var(--border)",
                borderRadius: 6,
                overflow: "hidden",
              }}>
                <div style={{
                  height: 120,
                  overflowY: "auto",
                  padding: "6px 10px",
                  background: "var(--bg)",
                  fontFamily: "monospace",
                  fontSize: 11,
                }}>
                  {packageLog.map((line, i) => (
                    <div key={i} style={{ whiteSpace: "pre-wrap", lineHeight: "1.5", color: "var(--text)" }}>
                      {line}
                    </div>
                  ))}
                  <div ref={pkgLogEndRef} />
                </div>
              </div>
            )}

            {/* Package list */}
            {packagesError && (
              <Alert type="error" message={packagesError} showIcon style={{ fontSize: 12 }} />
            )}
            <div style={{
              border: "1px solid var(--border)",
              borderRadius: 6,
              maxHeight: 260,
              overflowY: "auto",
            }}>
              {packagesLoading
                ? <div style={{ padding: 16, textAlign: "center" }}><Spin size="small" /></div>
                : (
                  <List
                    size="small"
                    dataSource={packages}
                    renderItem={(pkg) => (
                      <List.Item
                        style={{ padding: "4px 10px" }}
                        actions={[
                          <Button
                            key="uninstall"
                            danger
                            size="small"
                            loading={uninstallingPkg === pkg.name}
                            disabled={packageOpRunning && uninstallingPkg !== pkg.name}
                            onClick={() => handleUninstallPackage(pkg.name)}
                          >
                            Uninstall
                          </Button>,
                        ]}
                      >
                        <span style={{ fontFamily: "monospace", fontSize: 12 }}>
                          {pkg.name}
                        </span>
                        <span style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)", marginLeft: 8 }}>
                          {pkg.version}
                        </span>
                      </List.Item>
                    )}
                  />
                )
              }
            </div>
          </Space>
        )}

        {/* ── Command + log (setup steps 0-2) ────────────────────────────── */}
        {current < 3 && (
          <>
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
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {useExisting && current === 0
                      ? "Press Validate to check the existing virtual environment."
                      : "Press Run to start this step."}
                  </Text>
                )}
                {cur.log.map((line, i) => (
                  <div key={i} style={{ whiteSpace: "pre-wrap", lineHeight: "1.5", color: "var(--text)" }}>
                    {line}
                  </div>
                ))}
                <div ref={logEndRef} />
              </div>
            </div>

            {/* ── Error ────────────────────────────────────────────────────── */}
            {cur.error && (
              <Alert type="error" message={cur.error} showIcon style={{ fontSize: 12 }} />
            )}

            {/* ── Step controls ────────────────────────────────────────────── */}
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              {useExisting && current === 0 ? (
                <Button
                  type="primary"
                  loading={validating}
                  disabled={cur.status === "done" || !venvPath.trim()}
                  onClick={handleUseExisting}
                >
                  {cur.status === "done" ? "Done" : "Validate"}
                </Button>
              ) : (
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
              )}
              {current > 0 && cur.status !== "running" && (
                <Button onClick={() => setCurrent((c) => c - 1)}>Back</Button>
              )}
              {cur.status === "done" && current < 2 && (
                <Button onClick={() => setCurrent((c) => c + 1)}>Next step</Button>
              )}
            </div>

            {setupDone && (
              <Tag color="success" style={{ fontSize: 12 }}>
                ✓ All steps completed — environment is ready
              </Tag>
            )}
          </>
        )}

      </Space>

      <PipRegistryModal open={registryOpen} onClose={() => setRegistryOpen(false)} />
    </Modal>
  );
}
