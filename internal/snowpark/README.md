# internal/snowpark

> Wails-bound Service for Snowpark/Jupyter notebook support: Python environment setup (conda or venv), per-tab kernel lifecycle, cell execution, DAP-based debugging, and pip registry management.

## Responsibility

- Manage two Python backend options: a named conda environment (`thaw_snowpark`, created with the configured `CondaPythonVersions` entry — default `DefaultCondaPython`) and a custom venv; detect, install, and verify each via `conda` / `python` / `pip` CLI calls.
- List system Python installations and evaluate their Snowpark compatibility (`ListSystemPythons`, `CheckSnowparkEnv`).
- Manage pip package operations in the active environment: list, install, uninstall (`ListEnvPackages`, `InstallEnvPackage`, `UninstallEnvPackage`).
- Install from / export to dependency files: `pip install -r requirements.txt` (`InstallRequirementsFile`), `pip install <dir>` from a `pyproject.toml` (`InstallPyprojectFile`), and `pip freeze` to a file (`FreezeRequirements`); file pickers `PickRequirementsFile` / `PickPyprojectFile` / `PickFreezeOutputFile` (each install/freeze is a pick→run pair so the UI can detect cancel before touching the log).
- Apply corporate pip registry settings (primary URL, extra indexes, proxy, CA cert, Basic Auth credentials) via `PipRegistryConfig` before every `pip install` (including requirements/pyproject installs).
- Classify failed `pip install` runs from pip's own output and return an actionable error instead of a bare exit status (`pipfailure.go`; see Patterns).
- Create, read, save, and pick `.ipynb` notebook files (`NewNotebook`, `ReadNotebook`, `SaveNotebook`, `PickNotebookFile`). `ReadNotebook` maps a missing file (`os.ErrNotExist`) to an error carrying `filesystem.NotFoundMarker` (`"file not found"`) so callers — e.g. the frontend's tab-refresh, deciding whether to orphan a tab — can detect a deleted notebook with a stable, locale-independent string rather than the localized OS message.
- Start and stop per-tab Python kernel sessions as long-lived `python -c <kernelPyScript>` subprocesses (`StartNotebookSession`, `StopNotebookSession`).
- Execute Python code cells and SQL cells in a running kernel, returning captured stdout/stderr and matplotlib figures as base64 PNGs (`RunNotebookCell`, `RunNotebookCellSql`).
- Support DAP (Debug Adapter Protocol) cell debugging via `debugpy` (`StartDapProxy`, `DebugNotebookCell`).
- Provide kernel-side autocomplete and hover documentation via Jedi (`GetNotebookCompletions`, `GetNotebookHover`).
- Check Python syntax and undefined names using `ast.parse` + pyflakes (`CheckPythonSyntax`).
- Synchronise session context (role/warehouse/database/schema) between the tab and the active kernel (`NotebookUseContext`, `syncKernelContext`).
- Persist and load per-notebook breakpoints (`SaveNotebookBreakpoints`, `LoadNotebookBreakpoints`).

## Key files

| File | Purpose |
|------|---------|
| `snowpark.go` | All domain logic; the embedded `kernelPyScript` Python program; all `Service` methods |
| `streamlit.go` | Local **Streamlit preview**: `StartStreamlitPreview(appDir, mainFile)` runs `python -m streamlit run` from the app folder using the active env's Python (`snowparkPython`), streaming output as `snowpark:streamlit-*` events and announcing the local URL once the port is up (or a `…-error` event if it never answers); `StopStreamlitPreview()` kills it. The whole replace-and-record sequence runs under `streamlitMu` (see Gotchas). `streamlitCommand` is a package var so `streamlit_test.go` can stub the process. Reused by the deploy modal for a pre-deploy look. |
| `pipfailure.go` | **pip failure classifier**: `classifyPipOutput` turns captured pip stdout+stderr into a `pipFailure{Kind, Spec, Packages}`, and `pipFailureAdvice` renders it as one actionable paragraph. `describePipFailure` is the combined entry point used by `Service.explainPipFailure`. Pure functions over `[]string` — no process launches — so `pipfailure_test.go` covers it with canned pip transcripts. |
| `doc.go` | Package doc and `// thaw:domain: Snowpark & Developer Workflows` annotation |

## Key types & functions

```go
// snowpark.go:764
type Service struct { ctx context.Context; syncTabContext SyncTabContextFunc }
func NewService(ctx context.Context, syncTabContext SyncTabContextFunc) *Service

// Kernel lifecycle
func (s *Service) StartNotebookSession(client *snowflake.Client, connectParams *snowflake.ConnectParams, tabId string) error
func (s *Service) StopNotebookSession(tabId string) error
func (s *Service) GetKernelPythonVersion(tabId string) string

// Cell execution
func (s *Service) RunNotebookCell(tabId, cellId, code string) (NotebookCellOutput, error)
func (s *Service) RunNotebookCellSql(client *snowflake.Client, tabId, sql string) (NotebookSqlResult, error)

// Debug
func (s *Service) StartDapProxy() error
func (s *Service) DebugNotebookCell(tabId, cellId, code string) (NotebookCellOutput, error)

// Intellisense
func (s *Service) GetNotebookCompletions(tabId, code string, line, col int) ([]NotebookCompletion, error)
func (s *Service) GetNotebookHover(tabId, code string, line, col int) (string, error)
func (s *Service) CheckPythonSyntax(tabId, code, mode string) ([]NotebookSyntaxError, error)

// Environment management
func (s *Service) CheckSnowparkEnv() SnowparkCheckResult
func (s *Service) InstallCondaEnv() error
func (s *Service) InstallVenvEnv() error
func (s *Service) ListEnvPackages() ([]PackageInfo, error)
func (s *Service) InstallEnvPackage(pkg string) error
func (s *Service) PickRequirementsFile() (string, error)
func (s *Service) PickPyprojectFile() (string, error)
func (s *Service) InstallRequirementsFile(path string) error
func (s *Service) InstallPyprojectFile(path string) error
func (s *Service) PickFreezeOutputFile() (string, error)
func (s *Service) FreezeRequirements(path string) error

// Config
func (s *Service) GetSnowparkConfig() SnowparkConfigResult
func (s *Service) SaveSnowparkCondaPython(version string) error
func (s *Service) GetPipRegistryConfig() (config.PipRegistryConfig, error)
func (s *Service) SavePipRegistryConfig(cfg config.PipRegistryConfig) error
```

## Patterns & integration

- `Service` is registered in `internal/app/run.go`'s `Bind` array; frontend imports from `wailsjs/go/snowpark/Service`.
- The kernel protocol is a line-oriented stdin/stdout protocol. The Go side sends code blocks terminated by `<<<THAW_RUN>>>` and reads until `<<<THAW_CELL_DONE>>>`. Specialised request types (completions, hover, SQL, syntax, debug) are distinguished by a leading marker line.
- Kernel sessions are stored in a package-level `sync.Map` keyed by `tabId`; each entry is a `notebookSession` struct holding the `*exec.Cmd`, stdin/stdout pipes, and a per-session mutex.
- The embedded `kernelPyScript` (a constant Go string containing ~500 lines of Python) is piped to the subprocess via stdin at startup. It sets up a shared namespace `g`, auto-creates a Snowpark `session` from `THAW_SF_*` environment variables, patches `session.sql()` to auto-collect DDL statements, and loops reading code blocks.
- Snowflake connection parameters for the kernel are injected via environment variables (`THAW_SF_ACCOUNT`, `THAW_SF_USER`, `THAW_SF_PASSWORD`, etc.) set at `StartNotebookSession` time (`notebookKernelEnv`), so the Python process shares the same connection as the active Wails tab.
- pip registry flags (`buildPipRegistrySetup`, taking the already-loaded `*config.AppConfig`) are assembled from `config.PipRegistryConfig`; credentials are embedded directly into registry URLs (no `.netrc` writes). The package-manager install paths (`InstallEnvPackage`, `InstallRequirementsFile`, `InstallPyprojectFile`) funnel through `pipInstallCmd` (single config read → registry flags + env → backend dispatch via `pipCmdConfig`). The conda/venv environment-setup steps (`InstallJupyterNotebook`, `InstallSnowparkVenv`, `InstallJupyterVenv`) build their commands directly but apply the same `buildPipRegistrySetup` flags after a nil-guarded `config.Load` (they return an error rather than silently dropping registry flags). Read-only/offline pip commands (`pip list`, `pip freeze`) use `pipCmd` and deliberately omit the registry flags — `pip freeze` makes no network calls and rejects index options like `--index-url`.
- `streamAndCapture` runs a command, emits each stdout/stderr line as a Wails event for live UI progress, **and** returns the captured stdout and stderr lines separately (checking the stdout scanner's `Err()` so a truncated read is never written to disk, and surfacing a truncated stderr read in the log). The captured lines are returned **even when the command exits non-zero** — a failing pip run is exactly when its output matters. `freezeToFile` takes only stdout, filters it through `isFreezeLine` (dropping `conda run` activation/solver/deprecation noise that would make `pip install -r` reject the file), writes the kept lines, then emits a final `✓ Wrote N packages to <path>` summary on the same ordered event channel so it always lands after the streamed lines. `streamCombined` concatenates both streams for the install paths; `streamCommandTo` discards them.
- **pip failure classification** (`pipfailure.go`, issue #885). Every install path (`InstallEnvPackage`, `InstallRequirementsFile`, `InstallPyprojectFile`) routes its failure through `Service.explainPipFailure`, which appends one actionable paragraph to pip's own error when the output matches a known signature, and returns the error untouched otherwise. Three kinds are recognised:
  - `source-build` — pip fell back to a source tarball (`Using cached pandas-2.0.3.tar.gz`) and the build failed. This is the issue-#885 case: the pin predates the environment's interpreter, so no wheel exists and the sdist build dies on an unrelated-looking error (`No module named 'pkg_resources'`, a missing compiler, Cython errors) that users read as a conflict with what Thaw already installed. The advice names the spec and the interpreter, says explicitly that it is *not* a conflict, and gives the two fixes.
  - `wheel-mismatch` — `No matching distribution found for <spec>` **with** a populated `(from versions: …)` list: the project exists on the index but no file installs on this interpreter.
  - `not-found` — the same message with `(from versions: none)`: nothing under that name at all, so the advice points at the name and the pip registry settings.
  The classifier is deliberately **output-only** — no second `pip install --dry-run` probe — so it costs nothing beyond one `python --version` (`envPythonVersion`, called only on the failure path). Order between the two streams is not relied upon, since stdout and stderr are captured on separate pipes and concatenated.
- `SaveSnowparkCondaPython` validates against `CondaPythonVersions` before persisting: the value is interpolated into the `conda create` argument list, so an unvalidated string would be an argument-injection vector. `condaPythonVersion(cfg)` re-validates on read and falls back to `DefaultCondaPython`, so a config edited by hand can never produce a broken `python=` argument.
- `SyncTabContextFunc` is called when `syncKernelContext` detects that the kernel's `USE DATABASE/SCHEMA` state has drifted from the tab; this keeps the session context pane in sync.
- DAP debugging writes each cell to a temp file (`/tmp/thaw_cell_<id>.py`) so debugpy can map breakpoints to physical file lines, then connects to `debugpy` via a local TCP port.

## Gotchas

- **Working directory**: `defaultVenvPath` (`<workdir>/snowpark_venv`) resolves the working dir through `workingDir()`, which prefers the provider injected by `snowpark.SetWorkdirProvider` (set once in `App.startup` to `App.currentWorkdir`) over a bare `config.Load().Git.ExportDir`. This is required for **"Open Folder in New Window"** override instances, whose folder lives only in memory and is never persisted — reading config directly would wrongly pick the shared/main-window folder. All config writers (`SaveSnowparkConfig`, `SaveSnowparkVenvPath`, `SaveSnowparkPythonPath`, `SavePipRegistryConfig`, `ResetPipRegistryConfig`) go through `config.Update` (process-locked RMW) so a concurrent write can't revert them.
- Each kernel subprocess is a long-lived process; `StopNotebookSession` must be called on tab close to avoid orphaned Python processes. `App.shutdown()` calls `StopAll()` on the service.
- The Streamlit preview is likewise long-lived: the deploy modal calls `StopStreamlitPreview` when it closes (and on the "Stop preview" button), so the `streamlit run` process isn't orphaned. Only one preview runs at a time — starting another replaces it. A local preview is **not** a fidelity guarantee: Snowflake's Streamlit runtime pins specific Python/Streamlit versions and an allow-listed Anaconda set, so "runs locally" ≠ "runs in Snowflake" (this caveat is surfaced in the UI).
- **"One preview at a time" is enforced by holding `streamlitMu` across the whole stop → start → record sequence**, not just the final assignment. Taking the lock only to record the process let two concurrent starts (a double-click ahead of the first IPC round-trip) each launch a server, with the loser left running and no longer referenced — unkillable until the app quit. `stopPreviewLocked` is the lock-held kill used by both `StartStreamlitPreview` and `StopStreamlitPreview`. `TestStartStreamlitPreview_ConcurrentStartsLeaveOneProcess` covers it under `-race` with a stubbed command.
- The readiness poll gives up after `readinessAttempts × readinessInterval` (~20 s) and emits `snowpark:streamlit-error`; without it the UI waited on a ready event that was never coming. Trailing output from a superseded process is dropped rather than interleaved into the live log of the one that replaced it. `freeTCPPort` closes its probe listener before Streamlit binds the port — an accepted TOCTOU window for a local dev preview, now visible as that error event rather than a hang if it is ever lost.
- The kernel stdout/stdin protocol is synchronous (one request in-flight per kernel session). Concurrent cell executions on the same tab are serialized by the per-session mutex.
- `externalbrowser` authenticator cannot be automated; the kernel prints a warning and leaves `session` uncreated. Users must call `Session.builder` manually in a cell.
- The `kernelPyScript` filters Jedi completions whose names start with `_` or contain `thaw` to prevent internal kernel state from leaking into user autocomplete.
- `matplotlib` is forced to the `Agg` (non-interactive) backend at kernel startup; `plt.show()` captures the figure as a base64 PNG rather than opening a GUI window.
- On Apple Silicon (`darwin/arm64`), conda environment setup may require additional considerations; `IsAppleSilicon()` is exposed for callers that need platform-specific handling.
- `config.PipRegistryConfig` credential/proxy passwords are stored in the OS secure store (`internal/secrets`), **not** `config.json`. `GetPipRegistryConfig`/`SavePipRegistryConfig`/`ResetPipRegistryConfig` hydrate/persist/delete them (`hydratePipSecrets`/`storePipSecrets`/`deletePipSecrets`), and `buildPipRegistrySetup` re-hydrates before embedding them into registry/proxy URLs at call time only (never written to `.netrc` or pip config files). Removing a registry prunes its stored secret.
- `CheckSnowparkEnv` verifies `snowflake-snowpark-python` and `notebook` with `importlib.util.find_spec` (`moduleAvailableScript`), **not** a real `import`. Actually importing those packages executes heavy init code (pandas/pyarrow/cryptography, jupyter-server) that can take many seconds and intermittently fail under load, which previously produced flaky false negatives in the check. `find_spec` only resolves the module spec, so it is fast and deterministic.
