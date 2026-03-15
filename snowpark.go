// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"thaw/internal/config"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SnowparkCondaEnv is the name of the conda environment used for Snowpark.
const SnowparkCondaEnv = "thaw_snowpark"

// Markers used by the embedded Python kernel to delimit cell execution.
const kernelSentinel  = "<<<THAW_CELL_DONE>>>"
const kernelRunMarker = "<<<THAW_RUN>>>"

// kernelPyScript is a minimal stateful Python kernel.  It reads code blocks
// from stdin (terminated by kernelRunMarker), executes them in a shared
// namespace, captures stdout/stderr, captures matplotlib figures as base64
// PNGs, and writes a JSON result + sentinel line.
const kernelPyScript = `import sys, io, traceback, json, base64, warnings, os

SENTINEL   = "<<<THAW_CELL_DONE>>>"
RUN_MARKER = "<<<THAW_RUN>>>"

# Force matplotlib to use the non-interactive Agg backend so that show()
# does not try to open a GUI window.
try:
    import matplotlib
    matplotlib.use("Agg")
    # plt.show() on the Agg backend emits a UserWarning explaining that the
    # canvas is non-interactive.  Suppress it — figures are captured as PNG.
    warnings.filterwarnings("ignore", message="FigureCanvasAgg is non-interactive")
except Exception:
    pass

g = {}

# ── Auto-create Snowpark session matching the Thaw app connection ──────────────
# Connection parameters are injected via THAW_SF_* environment variables so
# that Python cells share the same account, role, warehouse, database and
# schema as SQL cells.  The function puts a 'session' variable directly into
# the user-cell namespace 'g' so it is available in every cell without any
# import or builder call.
def _thaw_create_session(ns):
    account = os.environ.get("THAW_SF_ACCOUNT", "")
    user    = os.environ.get("THAW_SF_USER", "")
    if not account or not user:
        return  # not connected

    auth = os.environ.get("THAW_SF_AUTHENTICATOR", "snowflake").lower()
    if auth == "externalbrowser":
        # Cannot automate browser-based SSO — the user must call Session.builder manually.
        print("[thaw] externalbrowser auth: create session manually with Session.builder", file=sys.stderr)
        return

    cfg = {"account": account, "user": user}
    for k, v in [
        ("role",      os.environ.get("THAW_SF_ROLE", "")),
        ("warehouse", os.environ.get("THAW_SF_WAREHOUSE", "")),
        ("database",  os.environ.get("THAW_SF_DATABASE", "")),
        ("schema",    os.environ.get("THAW_SF_SCHEMA", "")),
    ]:
        if v:
            cfg[k] = v

    pk_path = os.environ.get("THAW_SF_PRIVATE_KEY_PATH", "")
    pk_pass = os.environ.get("THAW_SF_PRIVATE_KEY_PASSPHRASE", "")
    okta_url = os.environ.get("THAW_SF_OKTA_URL", "")

    if auth in ("snowflake_jwt", "jwt"):
        cfg["authenticator"]   = "snowflake_jwt"
        cfg["private_key_file"] = pk_path  # snowflake-connector-python expects "private_key_file"
        if pk_pass:
            # passphrase must be bytes, not str
            cfg["private_key_file_pwd"] = pk_pass.encode("utf-8")
    elif auth == "okta" and okta_url:
        cfg["authenticator"] = okta_url  # Snowpark uses the Okta URL as authenticator
        cfg["password"]      = os.environ.get("THAW_SF_PASSWORD", "")
    else:
        cfg["password"] = os.environ.get("THAW_SF_PASSWORD", "")
        if auth and auth != "snowflake":
            cfg["authenticator"] = auth

    try:
        from snowflake.snowpark import Session as _Session
        # Redirect stdout during session creation — Snowpark may print banners
        # that would corrupt the stdin/stdout protocol.
        _old_out, sys.stdout = sys.stdout, io.StringIO()
        try:
            ns["session"] = _Session.builder.configs(cfg).create()
        finally:
            sys.stdout = _old_out
    except Exception as _e:
        # Store full traceback so it surfaces in the first cell's stderr output
        # rather than being lost to the discarded kernel stderr stream.
        import traceback as _tb
        _thaw_init_errors.append(_tb.format_exc())

_thaw_init_errors = []
_thaw_create_session(g)
del _thaw_create_session

def _capture_figures():
    """Return a list of base64-encoded PNG strings for all open figures."""
    images = []
    try:
        import matplotlib.pyplot as plt
        for num in plt.get_fignums():
            fig = plt.figure(num)
            buf = io.BytesIO()
            fig.savefig(buf, format="png", bbox_inches="tight", dpi=150)
            buf.seek(0)
            images.append(base64.b64encode(buf.read()).decode("utf-8"))
        plt.close("all")
    except Exception:
        pass
    return images

while True:
    lines = []
    while True:
        line = sys.stdin.readline()
        if not line:
            sys.exit(0)
        if line.rstrip('\n') == RUN_MARKER:
            break
        lines.append(line)

    code    = "".join(lines)
    buf_out = io.StringIO()
    buf_err = io.StringIO()
    # Drain any deferred session-init errors into the first cell's stderr.
    while _thaw_init_errors:
        buf_err.write(_thaw_init_errors.pop(0))
    old_out, old_err = sys.stdout, sys.stderr
    sys.stdout = buf_out
    sys.stderr = buf_err
    err_info   = None
    try:
        exec(compile(code, "<cell>", "exec"), g)
    except Exception:
        err_info = traceback.format_exc()
    finally:
        sys.stdout = old_out
        sys.stderr = old_err

    images = _capture_figures()
    result = {"stdout": buf_out.getvalue(), "stderr": buf_err.getvalue(), "error": err_info, "images": images}
    print(json.dumps(result), flush=True)
    print(SENTINEL, flush=True)
`

// ─── exported structs ─────────────────────────────────────────────────────────

// SnowparkCheckResult describes the state of the local Snowpark environment.
type SnowparkCheckResult struct {
	IsReady             bool   `json:"isReady"`
	Details             string `json:"details"`
	PythonPath          string `json:"pythonPath"`
	Version             string `json:"version"`             // Python version inside the active env
	SystemPythonVersion string `json:"systemPythonVersion"` // Python version found on PATH
	Backend             string `json:"backend"`             // "conda" | "venv"
	VenvPath            string `json:"venvPath"`            // effective venv path when backend=venv
	HasConda            bool   `json:"hasConda"`
	HasEnv              bool   `json:"hasEnv"`              // conda env exists
	HasVenv             bool   `json:"hasVenv"`             // venv exists
	HasSnowpark         bool   `json:"hasSnowpark"`
	HasNotebook         bool   `json:"hasNotebook"`
}

// SnowparkConfigResult is returned by GetSnowparkConfig.
type SnowparkConfigResult struct {
	Backend    string `json:"backend"`    // "conda" | "venv"
	VenvPath   string `json:"venvPath"`   // effective venv path (never empty)
	PythonPath string `json:"pythonPath"` // saved python binary for venv (may be empty)
}

// PythonInfo describes a Python interpreter found on the system.
type PythonInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// NotebookCellOutput is the result of running a single notebook cell.
type NotebookCellOutput struct {
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
	Error  string   `json:"error"`
	Images []string `json:"images"` // base64-encoded PNG figures (e.g. matplotlib plots)
}

// NotebookSqlResult is returned by RunNotebookSql.
type NotebookSqlResult struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	RowCount int64    `json:"rowCount"`
	QueryID  string   `json:"queryID"`
}

// PackageInfo describes a Python package installed in the Snowpark environment.
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ─── kernel session management ────────────────────────────────────────────────

type notebookSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
}

var (
	notebookSessions   sync.Map   // tabId (string) → *notebookSession
	kernelScriptOnce   sync.Once
	kernelScriptPath   string
	kernelScriptErr    error
)

func ensureKernelScript() (string, error) {
	kernelScriptOnce.Do(func() {
		f, err := os.CreateTemp("", "thaw_kernel_*.py")
		if err != nil {
			kernelScriptErr = err
			return
		}
		if _, err = f.WriteString(kernelPyScript); err != nil {
			kernelScriptErr = err
			f.Close()
			return
		}
		f.Close()
		kernelScriptPath = f.Name()
	})
	return kernelScriptPath, kernelScriptErr
}

// ─── IPC methods ──────────────────────────────────────────────────────────────

// IsAppleSilicon reports whether the host is an Apple Silicon (arm64) Mac.
func (a *App) IsAppleSilicon() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

// ─── venv helpers ─────────────────────────────────────────────────────────────

// defaultVenvPath returns the default venv location.
// Prefers <exportDir>/snowpark_venv so the env lives next to the project files;
// falls back to ~/snowpark_venv when no export directory is configured.
func defaultVenvPath() string {
	cfg, err := config.Load()
	if err == nil && cfg.Git.ExportDir != "" {
		return filepath.Join(cfg.Git.ExportDir, "snowpark_venv")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "snowpark_venv")
}

// venvPythonBin returns the python binary path inside a venv.
func venvPythonBin(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "python.exe")
	}
	return filepath.Join(venvPath, "bin", "python")
}

// venvPipBin returns the pip binary path inside a venv.
func venvPipBin(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "pip.exe")
	}
	return filepath.Join(venvPath, "bin", "pip")
}

// snowparkPython returns the Python executable to use for kernel sessions,
// choosing between the conda env and the venv based on persisted config.
func snowparkPython() (string, error) {
	cfg, _ := config.Load()
	if cfg.Snowpark.Backend == "venv" {
		venvPath := cfg.Snowpark.VenvPath
		if venvPath == "" {
			venvPath = defaultVenvPath()
		}
		py := venvPythonBin(venvPath)
		if _, err := os.Stat(py); err != nil {
			return "", fmt.Errorf("venv python not found at %s — run Setup", py)
		}
		return py, nil
	}
	// conda: find the python inside the env and return its absolute path.
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return "", fmt.Errorf("conda not found")
	}
	out, err := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv, "which", "python").Output()
	if err != nil {
		return "", fmt.Errorf("conda env '%s' python not found — run Setup", SnowparkCondaEnv)
	}
	return strings.TrimSpace(string(out)), nil
}

// ─── config IPC ───────────────────────────────────────────────────────────────

// GetSnowparkConfig returns the effective Snowpark environment settings.
func (a *App) GetSnowparkConfig() SnowparkConfigResult {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	return SnowparkConfigResult{Backend: backend, VenvPath: venvPath, PythonPath: cfg.Snowpark.PythonPath}
}

// SaveSnowparkConfig persists the backend choice.
func (a *App) SaveSnowparkConfig(backend string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Snowpark.Backend = backend
	return config.Save(cfg)
}

// SaveSnowparkPythonPath persists the chosen Python binary path for venv creation.
func (a *App) SaveSnowparkPythonPath(pythonPath string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Snowpark.PythonPath = pythonPath
	return config.Save(cfg)
}

// ListSystemPythons returns all Python interpreters found on the system.
func (a *App) ListSystemPythons() []PythonInfo {
	seen := map[string]bool{}
	var result []PythonInfo
	re := regexp.MustCompile(`Python (\d+\.\d+(?:\.\d+)?)`)

	add := func(path string) {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		if seen[real] {
			return
		}
		out, err := exec.Command(path, "--version").CombinedOutput()
		if err != nil {
			return
		}
		m := re.FindStringSubmatch(string(out))
		if len(m) < 2 {
			return
		}
		seen[real] = true
		result = append(result, PythonInfo{Path: path, Version: m[1]})
	}

	// PATH-based names.
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			add(p)
		}
	}
	for minor := 9; minor <= 15; minor++ {
		if p, err := exec.LookPath(fmt.Sprintf("python3.%d", minor)); err == nil {
			add(p)
		}
	}

	// Common installation prefixes.
	globs := []string{
		"/usr/bin/python3*",
		"/usr/local/bin/python3*",
		"/opt/homebrew/bin/python3*",
		"/opt/homebrew/opt/python*/bin/python3*",
	}
	if home, err := os.UserHomeDir(); err == nil {
		globs = append(globs,
			filepath.Join(home, ".pyenv/versions/*/bin/python3"),
			filepath.Join(home, ".pyenv/versions/*/bin/python3.*"),
		)
	}
	for _, pattern := range globs {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			add(m)
		}
	}

	return result
}

// detectSystemPython returns the version string (e.g. "3.11") of the first
// python3/python binary found on PATH, or "" if none is found.
func detectSystemPython() string {
	re := regexp.MustCompile(`Python (\d+\.\d+)`)
	for _, bin := range []string{"python3", "python"} {
		p, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		out, err := exec.Command(p, "--version").CombinedOutput()
		if err != nil {
			continue
		}
		if m := re.FindStringSubmatch(string(out)); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

// CheckSnowparkEnv inspects the local environment based on the configured backend.
func (a *App) CheckSnowparkEnv() SnowparkCheckResult {
	result := SnowparkCheckResult{}

	// Always detect system Python regardless of backend.
	result.SystemPythonVersion = detectSystemPython()

	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}
	result.Backend = backend

	if backend == "venv" {
		return a.checkVenvEnv(&result, cfg)
	}
	return a.checkCondaEnv(&result)
}

func (a *App) checkCondaEnv(result *SnowparkCheckResult) SnowparkCheckResult {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		result.Details = "conda not found. Install Miniconda or Anaconda and ensure it is on your PATH."
		return *result
	}
	result.HasConda = true

	listOut, err := exec.Command(condaPath, "env", "list").Output()
	if err != nil {
		result.Details = "Failed to list conda environments."
		return *result
	}
	if !strings.Contains(string(listOut), SnowparkCondaEnv) {
		result.Details = fmt.Sprintf("Conda environment '%s' not found. Run Snowpark > Setup Environment to create it.", SnowparkCondaEnv)
		return *result
	}
	result.HasEnv = true

	verOut, _ := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv, "python", "--version").CombinedOutput()
	re := regexp.MustCompile(`Python (\d+\.\d+)`)
	if m := re.FindStringSubmatch(string(verOut)); len(m) >= 2 {
		result.Version = m[1]
	}
	if result.Version != "" && result.Version < "3.9" {
		result.Details = fmt.Sprintf("Python %s in the env is too old (need 3.9+).", result.Version)
		return *result
	}

	if err := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
		"python", "-c", "import snowflake.snowpark").Run(); err != nil {
		result.Details = "snowflake-snowpark-python not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasSnowpark = true

	if err := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
		"python", "-c", "import notebook").Run(); err != nil {
		result.Details = "notebook package not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasNotebook = true

	pyPath, _ := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv, "which", "python").Output()
	result.PythonPath = strings.TrimSpace(string(pyPath))
	result.IsReady = true
	result.Details = "Environment is fully configured."
	return *result
}

func (a *App) checkVenvEnv(result *SnowparkCheckResult, cfg *config.AppConfig) SnowparkCheckResult {
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	result.VenvPath = venvPath

	python := venvPythonBin(venvPath)
	if _, err := os.Stat(python); err != nil {
		result.Details = fmt.Sprintf("venv not found at %s. Run Snowpark > Setup Environment.", venvPath)
		return *result
	}
	result.HasVenv = true

	verOut, _ := exec.Command(python, "--version").CombinedOutput()
	re := regexp.MustCompile(`Python (\d+\.\d+)`)
	if m := re.FindStringSubmatch(string(verOut)); len(m) >= 2 {
		result.Version = m[1]
	}
	if result.Version != "" && result.Version < "3.9" {
		result.Details = fmt.Sprintf("Python %s in the venv is too old (need 3.9+).", result.Version)
		return *result
	}

	if err := exec.Command(python, "-c", "import snowflake.snowpark").Run(); err != nil {
		result.Details = "snowflake-snowpark-python not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasSnowpark = true

	if err := exec.Command(python, "-c", "import notebook").Run(); err != nil {
		result.Details = "notebook package not installed. Run Snowpark > Setup Environment."
		return *result
	}
	result.HasNotebook = true

	result.PythonPath = python
	result.IsReady = true
	result.Details = "Environment is fully configured."
	return *result
}

// streamCommandTo runs cmd and emits each output line as the given event.
func (a *App) streamCommandTo(cmd *exec.Cmd, eventName string) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	emit := func(line string) {
		wailsruntime.EventsEmit(a.ctx, eventName, line)
	}

	var wg sync.WaitGroup
	scanPipe := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			emit(sc.Text())
		}
	}
	wg.Add(2)
	go scanPipe(stdout)
	go scanPipe(stderr)
	wg.Wait()
	return cmd.Wait()
}

// streamCommand runs cmd and emits each output line as a "snowpark:install-output" event.
func (a *App) streamCommand(cmd *exec.Cmd) error {
	return a.streamCommandTo(cmd, "snowpark:install-output")
}

// pipBinForEnv returns the pip binary path for the active backend environment.
func (a *App) pipBinForEnv() (string, error) {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}
	if backend == "venv" {
		venvPath := cfg.Snowpark.VenvPath
		if venvPath == "" {
			venvPath = defaultVenvPath()
		}
		pip := venvPipBin(venvPath)
		if _, err := os.Stat(pip); err != nil {
			return "", fmt.Errorf("venv pip not found at %s — run Setup first", pip)
		}
		return pip, nil
	}
	// conda: use "conda run -n thaw_snowpark pip"
	return "", nil
}

// ListEnvPackages returns all packages installed in the active Snowpark environment.
func (a *App) ListEnvPackages() ([]PackageInfo, error) {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}

	var cmd *exec.Cmd
	if backend == "venv" {
		pip, err := a.pipBinForEnv()
		if err != nil {
			return nil, err
		}
		cmd = exec.Command(pip, "list", "--format=json")
	} else {
		condaPath, err := exec.LookPath("conda")
		if err != nil {
			return nil, fmt.Errorf("conda not found: %w", err)
		}
		cmd = exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
			"pip", "list", "--format=json")
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pip list: %w", err)
	}

	var raw []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse pip list: %w", err)
	}

	pkgs := make([]PackageInfo, 0, len(raw))
	for _, r := range raw {
		pkgs = append(pkgs, PackageInfo{Name: r.Name, Version: r.Version})
	}
	return pkgs, nil
}

// InstallEnvPackage installs a single Python package in the active environment,
// streaming output via the "snowpark:package-output" event.
func (a *App) InstallEnvPackage(pkg string) error {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}

	var cmd *exec.Cmd
	if backend == "venv" {
		pip, err := a.pipBinForEnv()
		if err != nil {
			return err
		}
		cmd = exec.Command(pip, "install", pkg)
	} else {
		condaPath, err := exec.LookPath("conda")
		if err != nil {
			return fmt.Errorf("conda not found: %w", err)
		}
		cmd = exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
			"pip", "install", pkg)
	}

	if err := a.streamCommandTo(cmd, "snowpark:package-output"); err != nil {
		return fmt.Errorf("install %s failed: %w", pkg, err)
	}
	return nil
}

// UninstallEnvPackage removes a Python package from the active environment,
// streaming output via the "snowpark:package-output" event.
func (a *App) UninstallEnvPackage(pkg string) error {
	cfg, _ := config.Load()
	backend := cfg.Snowpark.Backend
	if backend == "" {
		backend = "conda"
	}

	var cmd *exec.Cmd
	if backend == "venv" {
		pip, err := a.pipBinForEnv()
		if err != nil {
			return err
		}
		cmd = exec.Command(pip, "uninstall", "-y", pkg)
	} else {
		condaPath, err := exec.LookPath("conda")
		if err != nil {
			return fmt.Errorf("conda not found: %w", err)
		}
		cmd = exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
			"pip", "uninstall", "-y", pkg)
	}

	if err := a.streamCommandTo(cmd, "snowpark:package-output"); err != nil {
		return fmt.Errorf("uninstall %s failed: %w", pkg, err)
	}
	return nil
}

// InstallCondaEnv creates the thaw_snowpark conda environment.
// On Apple Silicon the CONDA_SUBDIR=osx-64 workaround is applied automatically.
func (a *App) InstallCondaEnv() error {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return fmt.Errorf("conda not found: %w", err)
	}

	// Skip if already exists.
	if out, _ := exec.Command(condaPath, "env", "list").Output(); strings.Contains(string(out), SnowparkCondaEnv) {
		wailsruntime.EventsEmit(a.ctx, "snowpark:install-output",
			fmt.Sprintf("Conda environment '%s' already exists, skipping creation.", SnowparkCondaEnv))
		return nil
	}

	args := []string{
		"create", "--name", SnowparkCondaEnv, "-y",
		"--override-channels",
		"-c", "https://repo.anaconda.com/pkgs/snowflake",
		"python=3.12", "numpy", "pandas", "pyarrow",
	}
	cmd := exec.Command(condaPath, args...)
	if a.IsAppleSilicon() {
		// Apple M-series: force x86_64 to avoid pyOpenSSL ffi.callback crash.
		cmd.Env = append(os.Environ(), "CONDA_SUBDIR=osx-64")
		wailsruntime.EventsEmit(a.ctx, "snowpark:install-output",
			"Apple Silicon detected — creating x86_64 environment (CONDA_SUBDIR=osx-64).")
	}
	if err := a.streamCommand(cmd); err != nil {
		return fmt.Errorf("conda create failed: %w", err)
	}

	// Pin subdir for future installs on Apple Silicon.
	if a.IsAppleSilicon() {
		cfgCmd := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
			"conda", "config", "--env", "--set", "subdir", "osx-64")
		if e := a.streamCommand(cfgCmd); e != nil {
			wailsruntime.EventsEmit(a.ctx, "snowpark:install-output",
				"[warn] conda config --env --set subdir osx-64: "+e.Error())
		}
	}
	return nil
}

// InstallSnowparkPackage installs snowflake-snowpark-python (plus pandas/pyarrow)
// into the thaw_snowpark conda environment.
func (a *App) InstallSnowparkPackage() error {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return fmt.Errorf("conda not found: %w", err)
	}
	cmd := exec.Command(condaPath, "install", "-n", SnowparkCondaEnv, "-y",
		"snowflake-snowpark-python", "pandas", "pyarrow")
	if err := a.streamCommand(cmd); err != nil {
		return fmt.Errorf("snowpark install failed: %w", err)
	}
	return nil
}

// InstallJupyterNotebook installs notebook, ipython-sql and sqlalchemy via pip.
func (a *App) InstallJupyterNotebook() error {
	condaPath, err := exec.LookPath("conda")
	if err != nil {
		return fmt.Errorf("conda not found: %w", err)
	}
	cmd := exec.Command(condaPath, "run", "-n", SnowparkCondaEnv,
		"pip", "install", "notebook", "ipython-sql", "sqlalchemy")
	if err := a.streamCommand(cmd); err != nil {
		return fmt.Errorf("notebook install failed: %w", err)
	}
	return nil
}

// ─── venv install methods ─────────────────────────────────────────────────────

// InstallVenvEnv creates a Python venv at the configured path using system python3.
func (a *App) InstallVenvEnv() error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}

	// Skip if already exists.
	if _, err := os.Stat(venvPythonBin(venvPath)); err == nil {
		wailsruntime.EventsEmit(a.ctx, "snowpark:install-output",
			fmt.Sprintf("Virtual environment already exists at %s, skipping.", venvPath))
		return nil
	}

	python := cfg.Snowpark.PythonPath
	if python == "" {
		var err error
		python, err = exec.LookPath("python3")
		if err != nil {
			python, err = exec.LookPath("python")
			if err != nil {
				return fmt.Errorf("python3 not found on PATH: %w", err)
			}
		}
	}

	wailsruntime.EventsEmit(a.ctx, "snowpark:install-output",
		fmt.Sprintf("Creating venv at %s…", venvPath))
	if err := os.MkdirAll(filepath.Dir(venvPath), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	cmd := exec.Command(python, "-m", "venv", venvPath)
	if err := a.streamCommand(cmd); err != nil {
		return fmt.Errorf("venv create failed: %w", err)
	}
	return nil
}

// InstallSnowparkVenv installs snowflake-snowpark-python into the venv.
// Pass withPandas=true for the [pandas] extras variant.
func (a *App) InstallSnowparkVenv(withPandas bool) error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	pkg := "snowflake-snowpark-python"
	if withPandas {
		pkg = "snowflake-snowpark-python[pandas]"
	}
	cmd := exec.Command(venvPipBin(venvPath), "install", pkg)
	if err := a.streamCommand(cmd); err != nil {
		return fmt.Errorf("snowpark venv install failed: %w", err)
	}
	return nil
}

// DeleteVenvFolder removes the venv directory at the configured path.
func (a *App) DeleteVenvFolder() error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		return nil // already gone
	}
	return os.RemoveAll(venvPath)
}

// InstallJupyterVenv installs notebook, ipython-sql and sqlalchemy into the venv.
func (a *App) InstallJupyterVenv() error {
	cfg, _ := config.Load()
	venvPath := cfg.Snowpark.VenvPath
	if venvPath == "" {
		venvPath = defaultVenvPath()
	}
	cmd := exec.Command(venvPipBin(venvPath), "install", "notebook", "ipython-sql", "sqlalchemy")
	if err := a.streamCommand(cmd); err != nil {
		return fmt.Errorf("notebook venv install failed: %w", err)
	}
	return nil
}

// minimalNotebook is the nbformat v4 JSON written when creating a new notebook.
const minimalNotebook = `{
 "nbformat": 4,
 "nbformat_minor": 5,
 "metadata": {
  "kernelspec": {"display_name": "Python 3", "language": "python", "name": "python3"},
  "language_info": {"name": "python", "version": "3.12.0"}
 },
 "cells": [
  {
   "cell_type": "code",
   "execution_count": null,
   "metadata": {},
   "outputs": [],
   "source": []
  }
 ]
}
`

// NewNotebook shows a save dialog, writes a blank notebook, and returns the path and content.
func (a *App) NewNotebook() (string, error) {
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "New Notebook",
		DefaultFilename: "notebook.ipynb",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Jupyter Notebooks (*.ipynb)", Pattern: "*.ipynb"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	// Ensure .ipynb extension.
	if filepath.Ext(path) != ".ipynb" {
		path += ".ipynb"
	}
	if err := os.WriteFile(path, []byte(minimalNotebook), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// PickNotebookFile opens a file picker for .ipynb files and returns the chosen path.
func (a *App) PickNotebookFile() (string, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open Notebook",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Jupyter Notebooks (*.ipynb)", Pattern: "*.ipynb"},
		},
	})
	return path, err
}

// ReadNotebook reads an .ipynb file and returns its raw JSON content.
func (a *App) ReadNotebook(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveNotebook writes notebook JSON to the given path.
func (a *App) SaveNotebook(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// RunNotebookSql executes a SQL query via the active Snowflake connection and
// returns the result as columns + rows, suitable for table display in a notebook.
func (a *App) RunNotebookSql(sql string) (NotebookSqlResult, error) {
	if a.client == nil {
		return NotebookSqlResult{}, ErrNotConnected
	}
	result, err := a.client.Execute(a.ctx, sql)
	if err != nil {
		return NotebookSqlResult{}, err
	}
	rows := result.Rows
	if rows == nil {
		rows = [][]any{}
	}
	return NotebookSqlResult{
		Columns:  result.Columns,
		Rows:     rows,
		RowCount: result.RowsAffected,
		QueryID:  result.QueryID,
	}, nil
}

// notebookKernelEnv returns the THAW_SF_* environment variables to inject into
// the kernel subprocess so it can auto-create a matching Snowpark session.
// Falls back to nil (no extra vars) when not connected or params are unavailable.
func (a *App) notebookKernelEnv() []string {
	if a.connectParams == nil || a.client == nil {
		return nil
	}
	p := a.connectParams
	// Fetch the live session state — the user may have switched role / warehouse
	// / database / schema since connecting.
	ctx, err := a.client.GetSessionContext(a.ctx)
	if err != nil {
		// Fall back to original params if the query fails.
		ctx.Role      = p.Role
		ctx.Warehouse = p.Warehouse
		ctx.Database  = p.Database
		ctx.Schema    = p.Schema
	}
	return []string{
		"THAW_SF_ACCOUNT="               + p.Account,
		"THAW_SF_USER="                  + p.User,
		"THAW_SF_PASSWORD="              + p.Password,
		"THAW_SF_AUTHENTICATOR="         + p.Authenticator,
		"THAW_SF_OKTA_URL="              + p.OktaURL,
		"THAW_SF_PRIVATE_KEY_PATH="      + p.PrivateKeyPath,
		"THAW_SF_PRIVATE_KEY_PASSPHRASE="+ p.PrivateKeyPassphrase,
		"THAW_SF_ROLE="                  + ctx.Role,
		"THAW_SF_WAREHOUSE="             + ctx.Warehouse,
		"THAW_SF_DATABASE="              + ctx.Database,
		"THAW_SF_SCHEMA="                + ctx.Schema,
	}
}

// StartNotebookSession launches the Python kernel subprocess for a notebook tab.
// Safe to call multiple times — returns immediately if a session already exists.
func (a *App) StartNotebookSession(tabId string) error {
	if _, ok := notebookSessions.Load(tabId); ok {
		return nil
	}

	scriptPath, err := ensureKernelScript()
	if err != nil {
		return fmt.Errorf("kernel script: %w", err)
	}

	python, err := snowparkPython()
	if err != nil {
		return err
	}

	cmd := exec.Command(python, "-u", scriptPath)
	// Inject connection parameters so the kernel can auto-create a Snowpark
	// session that matches the app's active connection.
	if extra := a.notebookKernelEnv(); len(extra) > 0 {
		cmd.Env = append(os.Environ(), extra...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// Redirect stderr to /dev/null to avoid cluttering (kernel errors go through JSON).
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("kernel start: %w", err)
	}

	notebookSessions.Store(tabId, &notebookSession{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	})
	return nil
}

// RunNotebookCell sends code to the kernel and returns its output.
// StartNotebookSession must have been called for this tabId first.
func (a *App) RunNotebookCell(tabId string, code string) (NotebookCellOutput, error) {
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return NotebookCellOutput{}, fmt.Errorf("no kernel for tab %s — call StartNotebookSession first", tabId)
	}
	s := val.(*notebookSession)
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write code block + run marker.
	if _, err := fmt.Fprintf(s.stdin, "%s\n%s\n", code, kernelRunMarker); err != nil {
		return NotebookCellOutput{}, fmt.Errorf("write to kernel: %w", err)
	}

	// Read lines until the sentinel appears; the line just before it is JSON.
	var lastJSON string
	for {
		line, err := s.stdout.ReadString('\n')
		if err != nil {
			return NotebookCellOutput{}, fmt.Errorf("read from kernel: %w", err)
		}
		line = strings.TrimRight(line, "\n\r")
		if line == kernelSentinel {
			break
		}
		lastJSON = line
	}

	var out NotebookCellOutput
	if lastJSON != "" {
		if err := json.Unmarshal([]byte(lastJSON), &out); err != nil {
			out.Stdout = lastJSON
		}
	}
	return out, nil
}

// NotebookUseContext sends USE statements to the running Snowpark kernel for a
// notebook tab so the session matches the tab's role/warehouse/database/schema.
// Returns nil immediately if no kernel is running for tabId or all params are empty.
func (a *App) NotebookUseContext(tabId, role, warehouse, database, schema string) error {
	if role == "" && warehouse == "" && database == "" && schema == "" {
		return nil
	}
	val, ok := notebookSessions.Load(tabId)
	if !ok {
		return nil // no kernel running — silently ignore
	}
	s := val.(*notebookSession)

	escape := func(v string) string {
		return strings.ReplaceAll(v, "'", "\\'")
	}

	lines := []string{"if 'session' in globals():", "    _s = globals()['session']"}
	if role != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_role('%s')", escape(role)))
	}
	if warehouse != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_warehouse('%s')", escape(warehouse)))
	}
	if database != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_database('%s')", escape(database)))
	}
	if schema != "" {
		lines = append(lines, fmt.Sprintf("    _s.use_schema('%s')", escape(schema)))
	}
	code := strings.Join(lines, "\n")

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.stdin, "%s\n%s\n", code, kernelRunMarker); err != nil {
		return fmt.Errorf("write to kernel: %w", err)
	}

	// Drain stdout until the sentinel (discard output).
	for {
		line, err := s.stdout.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read from kernel: %w", err)
		}
		if strings.TrimRight(line, "\n\r") == kernelSentinel {
			break
		}
	}
	return nil
}

// StopNotebookSession kills the Python kernel for a notebook tab.
func (a *App) StopNotebookSession(tabId string) error {
	val, ok := notebookSessions.LoadAndDelete(tabId)
	if !ok {
		return nil
	}
	s := val.(*notebookSession)
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.stdin.Close()
	if s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}
