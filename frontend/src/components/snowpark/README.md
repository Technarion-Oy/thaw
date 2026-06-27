# frontend/src/components/snowpark

> Modals for setting up, managing, and configuring the Snowpark Python environment used by Thaw notebooks.

## Responsibility

Provides the UI for Snowpark environment management: health checking, guided installation
(conda or venv backend), package management, and pip registry configuration. Connects to
the backend `snowpark` package via IPC and streams installation output via Wails events.

## Files

| File | Purpose |
|------|---------|
| `SnowparkCheckModal.tsx` | Environment health check: calls `CheckSnowparkEnv` on mount and on "Re-check". Displays pass/fail rows for Python, conda/venv, snowflake-snowpark-python, and notebook. Shows "Setup Environment…" link if not ready. |
| `SnowparkSetupModal.tsx` | Multi-step installation wizard (3 steps + package manager). Supports conda and venv. Streams install output via `EventsOn("snowpark:install-output")`. Step 3 embeds a full package manager (install/uninstall, plus install from `requirements.txt`/`pyproject.toml` and freeze to `requirements.txt`). |
| `PipRegistryModal.tsx` | Pip registry configuration: primary URL, extra index URLs, credentials per registry, proxy settings, trusted hosts, and CA cert path. Loads/saves via `GetPipRegistryConfig`/`SavePipRegistryConfig`. |

## Patterns & integration

**IPC calls:**
- `CheckSnowparkEnv()` — returns `snowpark.SnowparkCheckResult` with per-check pass/fail flags
- `InstallSnowparkStep(step, backend)` — installs one step (conda/venv setup or package install); streams output via `snowpark:install-output` Wails event
- `ListEnvPackages()` / `InstallEnvPackage(name)` / `UninstallEnvPackage(name)` — package manager in Step 3
- `PickRequirementsFile()` + `InstallRequirementsFile(path)`, `PickPyprojectFile()` + `InstallPyprojectFile(path)`, `FreezeRequirements("")` — dependency-file install/export buttons in Step 3
- `SaveSnowparkConfig(cfg)` / `SaveSnowparkVenvPath(path)` / `SaveSnowparkPythonPath(path)` — persist environment settings
- `GetPipRegistryConfig()` / `SavePipRegistryConfig(cfg)` — load/save pip registry settings
- `PickCACertFile()` — native file picker for CA certificate path

**Event streaming:** `SnowparkSetupModal` subscribes to `"snowpark:install-output"` with `EventsOn` and appends lines to a scrollable terminal-style output area. The cleanup function is called on unmount.

**Apple Silicon detection:** `SnowparkSetupModal` checks `navigator.platform` for `"Mac"` and CPU architecture hints to show a `CONDA_SUBDIR=osx-64` workaround note when conda is selected on Apple Silicon.

**Stores used:** `gitStore` — reads `projectDir` to suggest the project directory for the venv path.

## Gotchas

- Installation steps are sequential (one step at a time). Each `InstallSnowparkStep` call streams its output before the next step can begin.
- `SnowparkCheckModal` uses `snowpark.SnowparkCheckResult` typed from `wailsjs/go/models`. The check result drives both the status row display and the conditional "Setup Environment…" button.
