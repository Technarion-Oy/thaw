# internal/snowpark

> Wails-bound Service for Snowpark/Jupyter notebook support: Python environment setup (conda or venv), per-tab kernel lifecycle, cell execution, DAP-based debugging, and pip registry management.

## Responsibility

- Manage two Python backend options: a named conda environment (`thaw_snowpark`) and a custom venv; detect, install, and verify each via `conda` / `python` / `pip` CLI calls.
- List system Python installations and evaluate their Snowpark compatibility (`ListSystemPythons`, `CheckSnowparkEnv`).
- Manage pip package operations in the active environment: list, install, uninstall (`ListEnvPackages`, `InstallEnvPackage`, `UninstallEnvPackage`).
- Install from / export to dependency files: `pip install -r requirements.txt` (`InstallRequirementsFile`), `pip install <dir>` from a `pyproject.toml` (`InstallPyprojectFile`), and `pip freeze` to a file (`FreezeRequirements`); file pickers `PickRequirementsFile` / `PickPyprojectFile`.
- Apply corporate pip registry settings (primary URL, extra indexes, proxy, CA cert, Basic Auth credentials) via `PipRegistryConfig` before every `pip install` (including requirements/pyproject installs).
- Create, read, save, and pick `.ipynb` notebook files (`NewNotebook`, `ReadNotebook`, `SaveNotebook`, `PickNotebookFile`).
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
| `doc.go` | Package doc and `// thaw:domain: Notebook & Developer Environment` annotation |

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
func (s *Service) InstallRequirementsFile(path string) error
func (s *Service) InstallPyprojectFile(path string) error
func (s *Service) FreezeRequirements(path string) (string, error)

// Config
func (s *Service) GetSnowparkConfig() SnowparkConfigResult
func (s *Service) GetPipRegistryConfig() (config.PipRegistryConfig, error)
func (s *Service) SavePipRegistryConfig(cfg config.PipRegistryConfig) error
```

## Patterns & integration

- `Service` is registered in `internal/app/run.go`'s `Bind` array; frontend imports from `wailsjs/go/snowpark/Service`.
- The kernel protocol is a line-oriented stdin/stdout protocol. The Go side sends code blocks terminated by `<<<THAW_RUN>>>` and reads until `<<<THAW_CELL_DONE>>>`. Specialised request types (completions, hover, SQL, syntax, debug) are distinguished by a leading marker line.
- Kernel sessions are stored in a package-level `sync.Map` keyed by `tabId`; each entry is a `notebookSession` struct holding the `*exec.Cmd`, stdin/stdout pipes, and a per-session mutex.
- The embedded `kernelPyScript` (a constant Go string containing ~500 lines of Python) is piped to the subprocess via stdin at startup. It sets up a shared namespace `g`, auto-creates a Snowpark `session` from `THAW_SF_*` environment variables, patches `session.sql()` to auto-collect DDL statements, and loops reading code blocks.
- Snowflake connection parameters for the kernel are injected via environment variables (`THAW_SF_ACCOUNT`, `THAW_SF_USER`, `THAW_SF_PASSWORD`, etc.) set at `StartNotebookSession` time (`notebookKernelEnv`), so the Python process shares the same connection as the active Wails tab.
- pip registry flags (`buildPipRegistrySetup`) are assembled from `config.PipRegistryConfig` immediately before each `pip install` invocation; credentials are embedded directly into registry URLs (no `.netrc` writes).
- `SyncTabContextFunc` is called when `syncKernelContext` detects that the kernel's `USE DATABASE/SCHEMA` state has drifted from the tab; this keeps the session context pane in sync.
- DAP debugging writes each cell to a temp file (`/tmp/thaw_cell_<id>.py`) so debugpy can map breakpoints to physical file lines, then connects to `debugpy` via a local TCP port.

## Gotchas

- Each kernel subprocess is a long-lived process; `StopNotebookSession` must be called on tab close to avoid orphaned Python processes. `App.shutdown()` calls `StopAll()` on the service.
- The kernel stdout/stdin protocol is synchronous (one request in-flight per kernel session). Concurrent cell executions on the same tab are serialized by the per-session mutex.
- `externalbrowser` authenticator cannot be automated; the kernel prints a warning and leaves `session` uncreated. Users must call `Session.builder` manually in a cell.
- The `kernelPyScript` filters Jedi completions whose names start with `_` or contain `thaw` to prevent internal kernel state from leaking into user autocomplete.
- `matplotlib` is forced to the `Agg` (non-interactive) backend at kernel startup; `plt.show()` captures the figure as a base64 PNG rather than opening a GUI window.
- On Apple Silicon (`darwin/arm64`), conda environment setup may require additional considerations; `IsAppleSilicon()` is exposed for callers that need platform-specific handling.
- `config.PipRegistryConfig.Password` is stored in `config.json` (0600), not in the system keychain; it is embedded into registry URLs at call time only, never written to `.netrc` or pip config files.
