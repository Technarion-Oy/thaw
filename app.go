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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	sf "github.com/snowflakedb/gosnowflake/v2"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/ai"
	"thaw/internal/config"
	"thaw/internal/ddl"
	"thaw/internal/filesystem"
	"thaw/internal/fnmeta"
	"thaw/internal/gitrepo"
	"thaw/internal/logger"
	"thaw/internal/sfconfig"
	"thaw/internal/snowflake"
	"thaw/internal/sqleditor"
	"thaw/internal/telemetry"
)

// App is the main application struct. Methods bound here are callable from the frontend.
type App struct {
	ctx           context.Context
	client        *snowflake.Client
	connectParams *snowflake.ConnectParams // stored after a successful Connect for notebook session init
	cancelConnect    context.CancelFunc
	exportCancelFunc    context.CancelFunc // cancels an in-flight DDL export
	migrationCancelFunc context.CancelFunc // cancels an in-flight schema migration
	cancelChat          context.CancelFunc // cancels an in-flight AI chat request
	fnStore          *fnmeta.Store      // local SQLite cache for Snowflake function metadata
	logCleanup       func()             // closes the log rotation file on shutdown
	savedWindowState *WindowState       // non-nil when a persisted window state was loaded at launch

	// Two-phase query execution (StartQuery / WaitForQueryResult).
	queryMu             sync.Mutex
	queryID             string
	queryDone           chan struct{}
	queryResult         *snowflake.QueryResult
	queryErr            error
	queryCancelFunc     context.CancelFunc  // cancels the in-flight query context
	queryCancelCtxDone  <-chan struct{}      // closed when the in-flight query context is canceled

	// Embedded terminal (pseudo-terminal).
	ptyMu  sync.Mutex
	ptmx   *os.File
	ptyCmd *exec.Cmd
}

// NewApp creates and returns a new App instance for use with the Wails runtime.
func NewApp() *App {
	return &App{}
}

// startup is called by the Wails runtime after the application window is ready.
// It stores the application context, initializes logging and telemetry.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.savedWindowState != nil {
		wailsruntime.WindowSetPosition(ctx, a.savedWindowState.X, a.savedWindowState.Y)
		if a.savedWindowState.Maximized {
			wailsruntime.WindowMaximise(ctx)
		}
	}
	a.logCleanup = logger.Init()
	telemetry.Init(Version)
	logger.L.Info("application started")
	telemetry.Track(telemetry.EventAppStarted, nil)

	// Open the function-metadata SQLite cache and seed it from the embedded
	// fallback JSON so autocomplete works immediately, even offline.
	if cfgDir, err := os.UserConfigDir(); err == nil {
		storeDir := filepath.Join(cfgDir, "Thaw")
		if store, err := fnmeta.Open(storeDir); err == nil {
			a.fnStore = store
			go func() {
				if err := store.LoadFallback(); err != nil {
					logger.L.Warn("fnmeta: load fallback failed", "err", err)
				}
			}()
		} else {
			logger.L.Warn("fnmeta: open store failed", "err", err)
		}
	}
}

// isQueryRunning reports whether a query submitted by StartQuery is still in flight.
func (a *App) isQueryRunning() bool {
	a.queryMu.Lock()
	defer a.queryMu.Unlock()
	return a.queryID != ""
}

// shutdown is called by the Wails runtime just before the application exits.
// It stops the embedded terminal, cancels any in-flight query, closes the
// Snowflake connection, and flushes logs and telemetry.
func (a *App) shutdown(_ context.Context) {
	// Persist window geometry so it can be restored on the next launch.
	w, h := wailsruntime.WindowGetSize(a.ctx)
	x, y := wailsruntime.WindowGetPosition(a.ctx)
	m := wailsruntime.WindowIsMaximised(a.ctx)
	_ = saveWindowState(WindowState{X: x, Y: y, Width: w, Height: h, Maximized: m})

	// Stop any running terminal process cleanly before the app exits.
	a.StopShell() //nolint:errcheck

	// Cancel any in-flight query so it stops consuming credits in Snowflake.
	// CancelQuery issues SYSTEM$CANCEL_QUERY in a goroutine; give it a moment
	// to fire before the process exits.
	a.queryMu.Lock()
	hasQuery := a.queryID != ""
	a.queryMu.Unlock()
	if hasQuery {
		a.CancelQuery()
		time.Sleep(500 * time.Millisecond)
	}

	if a.client != nil {
		// Close asynchronously — the gosnowflake driver sends an HTTP DELETE
		// /session to invalidate the token, which takes ~2 s. The app is
		// exiting anyway, so there is no need to wait; the OS will close the
		// TCP connection and Snowflake will expire the session on its own.
		go a.client.Close() //nolint:errcheck
	}

	if a.fnStore != nil {
		a.fnStore.Close() //nolint:errcheck
	}

	telemetry.Track(telemetry.EventAppStopped, telemetry.Props{
		"duration_s": int(telemetry.SessionDuration().Seconds()),
	})
	logger.L.Info("application shutting down")
	if a.logCleanup != nil {
		a.logCleanup()
	}
}

// Connect opens a Snowflake connection with the provided parameters.
// It can be interrupted by calling CancelConnect.
func (a *App) Connect(params snowflake.ConnectParams) error {
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelConnect = cancel
	defer func() {
		cancel()
		a.cancelConnect = nil
	}()

	logger.L.Info("connecting to Snowflake", "account", params.Account, "user", params.User, "authenticator", params.Authenticator)
	client, err := snowflake.NewClient(ctx, params)
	if err != nil {
		if ctx.Err() != nil {
			logger.L.Info("connection canceled by user")
			return fmt.Errorf("connection canceled")
		}
		logger.L.Error("connection failed", "account", params.Account, "err", err)
		telemetry.Track(telemetry.EventConnectionFailed, nil)
		return err
	}
	a.client = client
	a.connectParams = &params
	logger.L.Info("connected", "account", params.Account, "user", params.User)
	telemetry.Track(telemetry.EventConnected, telemetry.Props{"authenticator": params.Authenticator})

	// Refresh the function metadata cache in the background.
	if a.fnStore != nil {
		go func() {
			if err := fnmeta.SyncFromSnowflake(a.ctx, client, a.fnStore); err != nil {
				logger.L.Warn("fnmeta: background sync failed", "err", err)
			}
		}()
	}

	return nil
}

// CancelConnect aborts an in-progress Connect call.
func (a *App) CancelConnect() {
	if a.cancelConnect != nil {
		a.cancelConnect()
	}
}

// CancelExport aborts an in-progress DDL export started by ExportAllDatabasesDDL
// or ExportDatabaseDDL. It is a no-op if no export is running.
func (a *App) CancelExport() {
	if a.exportCancelFunc != nil {
		a.exportCancelFunc()
	}
}

// CancelChat aborts an in-progress AI chat request. It is a no-op if no
// request is in flight.
func (a *App) CancelChat() {
	if a.cancelChat != nil {
		a.cancelChat()
	}
}

// LoadSnowflakeCLIConfig reads the Snowflake CLI configuration file (either from
// the custom path set by PickSnowflakeCLIConfigPath or the default location)
// and returns all named connection profiles together with the default one.
func (a *App) LoadSnowflakeCLIConfig() (sfconfig.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return sfconfig.Config{}, err
	}
	scfg, err := sfconfig.Load(cfg.SnowflakeCLIConfigPath)
	if err != nil {
		return sfconfig.Config{}, err
	}
	return *scfg, nil
}

// TableSummary represents detailed information about a table in a database.
type TableSummary struct {
	Name          string `json:"name"`
	Schema        string `json:"schema"`
	Kind          string `json:"kind"` // BASE TABLE, VIEW, etc.
	Rows          int64  `json:"rows"`
	Bytes         int64  `json:"bytes"`
	Owner         string `json:"owner"`
	RetentionTime int    `json:"retentionTime"`
	// Use string for Wails binding compatibility with time.Time
	Created     string `json:"created"`
	LastAltered string `json:"lastAltered"`
	Comment     string `json:"comment"`
}

// GetDatabaseTableSummary returns detailed information about all tables in the
// specified database.
func (a *App) GetDatabaseTableSummary(dbName string) ([]TableSummary, error) {
	if a.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	qdb := "\"" + strings.ReplaceAll(dbName, "\"", "\"\"") + "\""
	query := fmt.Sprintf(`
		SELECT 
			TABLE_NAME, 
			TABLE_SCHEMA,
			TABLE_TYPE, 
			ROW_COUNT, 
			BYTES, 
			TABLE_OWNER, 
			RETENTION_TIME, 
			CREATED, 
			LAST_ALTERED, 
			COMMENT 
		FROM %s.INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_TYPE IN ('BASE TABLE', 'TRANSIENT', 'TEMPORARY')
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`, qdb)

	res, err := a.client.QuerySingle(a.ctx, query)
	if err != nil {
		return nil, err
	}

	var tables []TableSummary
	for _, row := range res.Rows {
		if len(row) < 10 {
			continue
		}
		t := TableSummary{
			Name:    fmt.Sprintf("%v", row[0]),
			Schema:  fmt.Sprintf("%v", row[1]),
			Kind:    fmt.Sprintf("%v", row[2]),
			Owner:   fmt.Sprintf("%v", row[5]),
		}

		if row[9] != nil {
			t.Comment = fmt.Sprintf("%v", row[9])
		}

		// Parsing numeric values
		t.Rows, _ = strconv.ParseInt(fmt.Sprintf("%v", row[3]), 10, 64)
		t.Bytes, _ = strconv.ParseInt(fmt.Sprintf("%v", row[4]), 10, 64)
		retTime, _ := strconv.Atoi(fmt.Sprintf("%v", row[6]))
		t.RetentionTime = retTime

		// Parsing times and converting to string for Wails compatibility
		if row[7] != nil {
			if ts, ok := row[7].(time.Time); ok {
				t.Created = ts.Format(time.RFC3339)
			}
		}
		if row[8] != nil {
			if ts, ok := row[8].(time.Time); ok {
				t.LastAltered = ts.Format(time.RFC3339)
			}
		}

		tables = append(tables, t)
	}

	return tables, nil
}

// GetSnowflakeCLIConfigPath returns the current path from which Snowflake CLI
// connection profiles are being loaded.
func (a *App) GetSnowflakeCLIConfigPath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	if cfg.SnowflakeCLIConfigPath != "" {
		return cfg.SnowflakeCLIConfigPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}
	return filepath.Join(home, ".snowflake", "config.toml"), nil
}

// PickSnowflakeCLIConfigPath opens a native file dialog to select a new
// Snowflake CLI configuration file. The selected path is persisted and
// used for all subsequent profile loads.
func (a *App) PickSnowflakeCLIConfigPath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	initialDir := ""
	if cfg.SnowflakeCLIConfigPath != "" {
		initialDir = filepath.Dir(cfg.SnowflakeCLIConfigPath)
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			initialDir = filepath.Join(home, ".snowflake")
		}
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Select Snowflake CLI Config",
		DefaultDirectory: initialDir,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Snowflake CLI Config (*.toml)", Pattern: "*.toml"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}

	cfg.SnowflakeCLIConfigPath = path
	if err := config.Save(cfg); err != nil {
		return "", err
	}
	return path, nil
}

// ─── Git / export configuration ──────────────────────────────────────────────

// GetGitConfig returns the persisted git / export settings.
func (a *App) GetGitConfig() (config.GitConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.GitConfig{}, err
	}
	return cfg.Git, nil
}

// SaveGitConfig persists git / export settings to disk.
// The token field is intentionally absent — it must never be written.
func (a *App) SaveGitConfig(gitCfg config.GitConfig) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Git = gitCfg
	return config.Save(cfg)
}

// ─── Git integration ──────────────────────────────────────────────────────────

// GitStatus returns the git status for the given directory.
// Safe to call on any directory — non-repos return IsRepo=false without error.
func (a *App) GitStatus(dir string) (gitrepo.RepoStatus, error) {
	return gitrepo.GetStatus(dir)
}

// GitCommitAndPush stages all changes, commits, and pushes to the remote.
// The Token field is used only in-memory for the push URL and is never persisted.
func (a *App) GitCommitAndPush(params gitrepo.PushParams) error {
	return gitrepo.CommitAndPush(a.ctx, params)
}

// GitPull fetches and merges changes from the remote branch.
// The Token field is used only in-memory for the pull URL and is never persisted.
func (a *App) GitPull(params gitrepo.PullParams) error {
	return gitrepo.Pull(a.ctx, params)
}

// ListDirectory returns the direct children of path (dirs first, then files).
func (a *App) ListDirectory(path string) ([]filesystem.FileEntry, error) {
	return filesystem.ListDir(path)
}

// ReadFile returns the text content of the file at path.
func (a *App) ReadFile(path string) (string, error) {
	return filesystem.ReadFile(path)
}

// ReadFileHead returns the first maxBytes bytes of the file at path.
// It is intended for lightweight file previews and is safe to call on large files.
func (a *App) ReadFileHead(path string, maxBytes int) (string, error) {
	return filesystem.ReadFileHead(path, maxBytes)
}

// SaveFile writes content to path, creating parent directories as needed.
func (a *App) SaveFile(path, content string) error {
	return filesystem.WriteFile(path, content)
}

// SearchFiles walks dir recursively and returns lines matching query.
// If useRegex is true, query is treated as a regular expression;
// otherwise a case-insensitive substring search is performed.
func (a *App) SearchFiles(dir, query string, useRegex bool) ([]filesystem.SearchMatch, error) {
	return filesystem.SearchFiles(dir, query, useRegex)
}

// ─── Account-level objects (roles, warehouses) ────────────────────────────────

// KeyPairResult holds the paths and public key content produced by GenerateKeyPair.
type KeyPairResult struct {
	PrivateKeyPath string `json:"privateKeyPath"`
	PublicKeyPath  string `json:"publicKeyPath"`
	PublicKey      string `json:"publicKey"` // stripped of PEM headers, ready for ALTER USER
}

// AccountExportResult reports the outcome of exporting account-level objects.
type AccountExportResult struct {
	Roles      int      `json:"roles"`
	Warehouses int      `json:"warehouses"`
	Errors     []string `json:"errors,omitempty"`
}

// GetRoleDDL returns the DDL definition of a Snowflake role.
func (a *App) GetRoleDDL(name string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	return a.client.GetRoleDDL(a.ctx, name)
}

// GetWarehouseDDL returns the DDL definition of a Snowflake warehouse.
func (a *App) GetWarehouseDDL(name string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	return a.client.GetWarehouseDDL(a.ctx, name)
}

// ExportAccountObjectsDDL exports all accessible roles and warehouses as SQL files
// under <outputDir>/_account/roles/ and <outputDir>/_account/warehouses/.
func (a *App) ExportAccountObjectsDDL(outputDir string) (AccountExportResult, error) {
	if a.client == nil {
		return AccountExportResult{}, ErrNotConnected
	}

	var result AccountExportResult

	// ── Roles ────────────────────────────────────────────────────────────────
	roles, err := a.client.ListRoles(a.ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list roles: %v", err))
	} else {
		for _, role := range roles {
			src, err := a.client.GetRoleDDL(a.ctx, role)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("role %s: %v", role, err))
				continue
			}
			path := filepath.Join(outputDir, "_account", "roles", sanitizeAccountFilename(role)+".sql")
			if writeErr := filesystem.WriteFile(path, strings.TrimRight(src, "\n")+"\n"); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("write role %s: %v", role, writeErr))
				continue
			}
			result.Roles++
		}
	}

	// ── Warehouses ───────────────────────────────────────────────────────────
	warehouses, err := a.client.ListWarehouses(a.ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list warehouses: %v", err))
	} else {
		for _, wh := range warehouses {
			src, err := a.client.GetWarehouseDDL(a.ctx, wh)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("warehouse %s: %v", wh, err))
				continue
			}
			path := filepath.Join(outputDir, "_account", "warehouses", sanitizeAccountFilename(wh)+".sql")
			if writeErr := filesystem.WriteFile(path, strings.TrimRight(src, "\n")+"\n"); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("write warehouse %s: %v", wh, writeErr))
				continue
			}
			result.Warehouses++
		}
	}

	return result, nil
}

// sanitizeAccountFilename replaces characters that are invalid in file names.
func sanitizeAccountFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

// PickOpenFile opens a native open-file dialog filtered to SQL, YAML and
// Python files and returns the chosen path, or an empty string if canceled.
// The dialog opens in the configured export directory when one is set.
func (a *App) PickOpenFile() string {
	defaultDir := ""
	if cfg, err := config.Load(); err == nil {
		defaultDir = cfg.Git.ExportDir
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Open file",
		DefaultDirectory: defaultDir,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Supported Files (*.sql, *.yml, *.yaml, *.py)", Pattern: "*.sql;*.yml;*.yaml;*.py"},
			{DisplayName: "SQL Files (*.sql)", Pattern: "*.sql"},
			{DisplayName: "YAML Files (*.yml, *.yaml)", Pattern: "*.yml;*.yaml"},
			{DisplayName: "Python Files (*.py)", Pattern: "*.py"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// dataFileFilters returns dialog file filters for the given import format.
func dataFileFilters(format string) []wailsruntime.FileFilter {
	switch strings.ToUpper(format) {
	case "JSON":
		return []wailsruntime.FileFilter{
			{DisplayName: "JSON Files (*.json;*.jsonl;*.ndjson)", Pattern: "*.json;*.jsonl;*.ndjson"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "PARQUET":
		return []wailsruntime.FileFilter{
			{DisplayName: "Parquet Files (*.parquet)", Pattern: "*.parquet"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "AVRO":
		return []wailsruntime.FileFilter{
			{DisplayName: "Avro Files (*.avro)", Pattern: "*.avro"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "ORC":
		return []wailsruntime.FileFilter{
			{DisplayName: "ORC Files (*.orc)", Pattern: "*.orc"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	default: // CSV
		return []wailsruntime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	}
}

// PickDataFile opens a native open-file dialog filtered to common data file
// formats and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickDataFile() string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open data file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Data Files (*.csv;*.json;*.jsonl;*.ndjson;*.parquet;*.avro;*.orc)", Pattern: "*.csv;*.json;*.jsonl;*.ndjson;*.parquet;*.avro;*.orc"},
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "JSON Files (*.json;*.jsonl;*.ndjson)", Pattern: "*.json;*.jsonl;*.ndjson"},
			{DisplayName: "Parquet Files (*.parquet)", Pattern: "*.parquet"},
			{DisplayName: "Avro Files (*.avro)", Pattern: "*.avro"},
			{DisplayName: "ORC Files (*.orc)", Pattern: "*.orc"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickDataFileByFormat opens a native open-file dialog filtered to the file
// extensions that match the given format.
func (a *App) PickDataFileByFormat(format string) string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Open " + format + " file",
		Filters: dataFileFilters(format),
	})
	if err != nil {
		return ""
	}
	return path
}

// PickDataFilesByFormat opens a native open-file dialog (multi-select) filtered
// to the extensions that match the given format. Returns the selected paths, or
// nil if the user cancels.
func (a *App) PickDataFilesByFormat(format string) []string {
	filters := dataFileFilters(format)
	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Open " + format + " files",
		Filters: filters,
	})
	if err != nil {
		return nil
	}
	return paths
}

// PickSaveFile opens a native save-file dialog pre-populated with defaultName
// and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickSaveFile(defaultName string) string {
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save file",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL Files (*.sql)", Pattern: "*.sql"},
			{DisplayName: "YAML Files (*.yml, *.yaml)", Pattern: "*.yml;*.yaml"},
			{DisplayName: "Python Files (*.py)", Pattern: "*.py"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickSaveExportFile opens a native save-file dialog with filters appropriate
// for the requested format ("csv" or "excel") and returns the chosen path, or
// an empty string if the user cancels.
func (a *App) PickSaveExportFile(defaultName, format string) string {
	var filters []wailsruntime.FileFilter
	title := "Save export file"
	switch format {
	case "csv":
		title = "Save as CSV"
		filters = []wailsruntime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "excel":
		title = "Save as Excel"
		filters = []wailsruntime.FileFilter{
			{DisplayName: "Excel Files (*.xlsx)", Pattern: "*.xlsx"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	default:
		filters = []wailsruntime.FileFilter{
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
		Filters:         filters,
	})
	if err != nil {
		return ""
	}
	return path
}

// SaveBinaryFile decodes the base64-encoded content and writes the raw bytes
// to path. Used for binary export formats such as Excel (.xlsx).
func (a *App) SaveBinaryFile(path, base64Content string) error {
	data, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// PickDirectory opens a native folder-picker dialog and returns the selected path.
// Returns an empty string if the user cancels.
func (a *App) PickDirectory() string {
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select repository directory",
	})
	if err != nil {
		return ""
	}
	return path
}

// Disconnect closes the active Snowflake connection.
func (a *App) Disconnect() error {
	if a.client == nil {
		return nil
	}
	err := a.client.Close()
	a.client = nil
	telemetry.Track(telemetry.EventDisconnected, nil)
	return err
}

// IsConnected returns true when a Snowflake connection is active.
func (a *App) IsConnected() bool {
	return a.client != nil && a.client.IsAlive()
}

// ExecuteQuery runs a SQL statement and returns the result set.
// Used by context-menu shortcuts (e.g. "Select Top 1000"). For the main editor
// flow use StartQuery + WaitForQueryResult to surface the query ID early.
func (a *App) ExecuteQuery(sql string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	qidChan := make(chan string, 1)
	ctx := sf.WithQueryIDChan(a.ctx, qidChan)

	result, err := a.client.Execute(ctx, sql)
	if result != nil {
		select {
		case qid := <-qidChan:
			result.QueryID = qid
		default:
		}
	}
	return result, err
}

// StartQuery submits a SQL statement and returns the Snowflake query ID as
// soon as Snowflake assigns one.  For queries that need more than one HTTP
// round-trip (slow queries) this returns while execution is still in progress,
// giving the frontend a chance to display the query ID in the loading spinner.
// Call WaitForQueryResult afterwards to obtain the actual rows.
// An in-flight query can be stopped with CancelQuery.
func (a *App) StartQuery(sql string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}

	// Create a per-query cancellable context and replace any previous one.
	ctx, cancel := context.WithCancel(a.ctx)

	a.queryMu.Lock()
	if a.queryCancelFunc != nil {
		a.queryCancelFunc() // cancel any still-running previous query
	}
	a.queryCancelFunc = cancel
	a.queryCancelCtxDone = ctx.Done()
	a.queryDone = nil // clear stale channel from previous query
	a.queryID = ""
	a.queryMu.Unlock()

	qidChan := make(chan string, 1)
	ctx = sf.WithQueryIDChan(ctx, qidChan)
	ctx = sf.WithAsyncMode(ctx) // ask Snowflake to return query ID immediately, before results are ready
	done := make(chan struct{})

	// Execute the query in a background goroutine so this method can return
	// as soon as the query ID arrives (before results are ready).
	var wg sync.WaitGroup
	go func() {
		result, err := a.client.Execute(ctx, sql, func(idx, total int, stmtQidChan <-chan string) {
			// Notify the frontend which statement is about to run.
			wailsruntime.EventsEmit(a.ctx, "query:statement-start",
				map[string]int{"index": idx, "total": total})
			// Watch for the per-statement query ID.  The channel is closed
			// by Execute once queryOnConn returns, so this goroutine always
			// terminates without needing ctx.Done().
			wg.Add(1)
			go func(i int, ch <-chan string) {
				defer wg.Done()
				// The gosnowflake driver closes ch after writing the qid, so
				// this select always terminates.  ctx.Done() is a fallback for
				// the rare case where the query is canceled before the driver
				// writes to the channel.
				select {
				case qid := <-ch:
					if qid != "" {
						// Keep a.queryID up to date so WaitForQueryResult can
						// embed the last statement's query ID in the result.
						a.queryMu.Lock()
						a.queryID = qid
						a.queryMu.Unlock()
						wailsruntime.EventsEmit(a.ctx, "query:statement-qid",
							map[string]interface{}{"index": i, "queryID": qid})
					}
				case <-ctx.Done():
				}
			}(idx, stmtQidChan)
		})
		// Wait for every per-statement qid goroutine to finish before
		// closing done, so WaitForQueryResult always reads a complete a.queryID.
		wg.Wait()
		a.queryMu.Lock()
		a.queryResult = result
		a.queryErr = err
		a.queryMu.Unlock()
		close(done)
	}()

	// Block until the driver assigns a query ID (arrives with the first HTTP
	// response), the background goroutine finishes (fast query), or the query
	// is canceled.
	var queryID string
	select {
	case qid := <-qidChan:
		queryID = qid
	case <-done:
		// Fast query: results arrived before our select ran. Drain the channel.
		select {
		case qid := <-qidChan:
			queryID = qid
		default:
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}

	a.queryMu.Lock()
	// For single-statement queries, queryID comes from the outer qidChan
	// (async mode) and should be stored.  For multi-statement queries the
	// outer qidChan never fires (queryID = ""), so we leave a.queryID as-is:
	// the per-statement qid goroutines (guarded by wg.Wait before close(done))
	// have already written the last statement's query ID into a.queryID.
	if queryID != "" {
		a.queryID = queryID
	}
	a.queryDone = done
	a.queryMu.Unlock()

	logger.L.Info("query started", "queryID", queryID)
	telemetry.Track(telemetry.EventQueryStarted, nil)
	return queryID, nil
}

// CancelQuery cancels the query currently in flight (started by StartQuery).
// It is a no-op if no query is running. In addition to canceling the local
// context, it issues SYSTEM$CANCEL_QUERY so that Snowflake stops the query
// server-side and stops consuming credits.
func (a *App) CancelQuery() {
	a.queryMu.Lock()
	cancel := a.queryCancelFunc
	queryID := a.queryID
	a.queryMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if queryID != "" && a.client != nil {
		logger.L.Info("canceling query", "queryID", queryID)
		telemetry.Track(telemetry.EventQueryCancelled, nil)
		go func() {
			ctx, done := context.WithTimeout(a.ctx, 15*time.Second)
			defer done()
			if err := a.client.CancelSnowflakeQuery(ctx, queryID); err != nil {
				logger.L.Warn("SYSTEM$CANCEL_QUERY failed", "queryID", queryID, "err", err)
			}
		}()
	}
}

// WaitForQueryResult blocks until the query submitted by StartQuery completes
// and returns the result set with the query ID embedded.
//
// If CancelQuery is called and the background goroutine does not finish within
// a 2-second grace period (e.g. the gosnowflake driver stalls while draining
// Arrow chunks after context cancellation), WaitForQueryResult returns
// context.Canceled immediately so the UI can reset without waiting for the
// driver to recover.  The background goroutine continues running and will clean
// up on its own once the driver eventually releases the connection.
func (a *App) WaitForQueryResult() (*snowflake.QueryResult, error) {
	a.queryMu.Lock()
	done := a.queryDone
	ctxDone := a.queryCancelCtxDone
	a.queryMu.Unlock()

	if done == nil {
		return nil, fmt.Errorf("no query in progress")
	}

	select {
	case <-done:
		// Normal path: background goroutine finished.
	case <-a.ctx.Done():
		// App is shutting down.
		return nil, a.ctx.Err()
	case <-ctxDone:
		// CancelQuery was called.  Give the driver a short window to respond
		// cleanly (it usually does — the Arrow error is logged before returning).
		select {
		case <-done:
			// Finished in time; fall through to the normal result-read below.
		case <-time.After(2 * time.Second):
			// Driver is stuck (Arrow chunk drain blocked on network I/O).
			// Unblock the UI now; the goroutine will clean up asynchronously.
			logger.L.Warn("query goroutine did not finish after cancellation; unblocking UI")
			a.queryMu.Lock()
			if a.queryCancelFunc != nil {
				a.queryCancelFunc()
				a.queryCancelFunc = nil
			}
			a.queryDone = nil
			a.queryID = ""
			a.queryCancelCtxDone = nil
			a.queryMu.Unlock()
			return nil, context.Canceled
		}
	}

	a.queryMu.Lock()
	result := a.queryResult
	err := a.queryErr
	// Read queryID after done fires so multi-statement queries get the last
	// per-statement qid (updated by wg-tracked goroutines before close(done)).
	queryID := a.queryID
	// Snapshot whether the query was explicitly canceled by the user BEFORE
	// calling queryCancelFunc: the cancel func also closes ctxDone, so
	// checking after cleanup would always report "canceled".
	var wasExplicitlyCancelled bool
	select {
	case <-ctxDone:
		wasExplicitlyCancelled = true
	default:
	}
	// Clean up so a subsequent call does not re-read stale state.
	if a.queryCancelFunc != nil {
		a.queryCancelFunc() // no-op if already canceled; ensures context resources are freed
		a.queryCancelFunc = nil
	}
	a.queryDone = nil
	a.queryID = ""
	a.queryCancelCtxDone = nil
	a.queryMu.Unlock()

	if result != nil && queryID != "" {
		result.QueryID = queryID
	}
	// Backstop: if the query was explicitly canceled (user called CancelQuery)
	// but the driver still returned a driver-level error (e.g. "Object does not
	// exist" from an aborted S3 pre-signed URL), replace it with
	// context.Canceled so the frontend shows "query canceled", not a
	// misleading error message.
	if err != nil && wasExplicitlyCancelled {
		err = context.Canceled
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			logger.L.Info("query canceled", "queryID", queryID)
		} else {
			logger.L.Error("query failed", "queryID", queryID, "err", err)
			telemetry.Track(telemetry.EventQueryFailed, nil)
		}
	} else {
		logger.L.Info("query completed", "queryID", queryID)
		telemetry.Track(telemetry.EventQueryCompleted, nil)
	}
	return result, err
}

// GetSessionContext returns the currently active role, warehouse, database and schema.
func (a *App) GetSessionContext() (snowflake.SessionContext, error) {
	if a.client == nil {
		return snowflake.SessionContext{}, ErrNotConnected
	}
	return a.client.GetSessionContext(a.ctx)
}

// ListRoles returns all roles visible to the current role (SHOW ROLES).
// Used for informational displays and user-management role pickers.
func (a *App) ListRoles() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListRoles(a.ctx)
}

// ListAvailableRoles returns only the roles the current user can switch to
// (CURRENT_AVAILABLE_ROLES). Used for the role-selection toolbar dropdown.
func (a *App) ListAvailableRoles() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListAvailableRoles(a.ctx)
}

// ListWarehouses returns all warehouses visible to the current role.
func (a *App) ListWarehouses() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListWarehouses(a.ctx)
}

// AlterWarehouseProperty applies a single SET property to a warehouse.
// property must be one of: size, warehouseType, autoSuspend, autoResume, comment,
// maxClusterCount, minClusterCount, scalingPolicy, resourceMonitor,
// enableQueryAcceleration, queryAccelerationMaxScaleFactor,
// maxConcurrencyLevel, statementQueuedTimeout, statementTimeout.
func (a *App) AlterWarehouseProperty(name, property, value string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	escId  := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	escStr := func(s string) string { return strings.ReplaceAll(s, `'`, `''`) }
	wh     := fmt.Sprintf(`"%s"`, escId(name))

	// allowlist checks for enum-typed values that are interpolated unquoted into SQL.
	checkEnum := func(v string, allowed ...string) (string, error) {
		u := strings.ToUpper(strings.TrimSpace(v))
		for _, a := range allowed {
			if u == a {
				return u, nil
			}
		}
		return "", fmt.Errorf("invalid value %q for warehouse property %q", v, property)
	}
	// validateInt parses v as a non-negative integer and returns it as a string safe for SQL interpolation.
	validateInt := func(v string) (string, error) {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 {
			return "", fmt.Errorf("invalid integer value %q for warehouse property %q", v, property)
		}
		return strconv.Itoa(n), nil
	}

	var query string
	switch property {
	case "size":
		v, err := checkEnum(value,
			"X-SMALL", "SMALL", "MEDIUM", "LARGE", "X-LARGE",
			"2X-LARGE", "3X-LARGE", "4X-LARGE", "5X-LARGE", "6X-LARGE")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET WAREHOUSE_SIZE = %s`, wh, v)
	case "warehouseType":
		v, err := checkEnum(value, "STANDARD", "SNOWPARK-OPTIMIZED")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET WAREHOUSE_TYPE = %s`, wh, v)
	case "autoSuspend":
		if value == "0" || value == "" {
			query = fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_SUSPEND = NULL`, wh)
		} else {
			v, err := validateInt(value)
			if err != nil {
				return err
			}
			query = fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_SUSPEND = %s`, wh, v)
		}
	case "autoResume":
		v, err := checkEnum(value, "TRUE", "FALSE")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_RESUME = %s`, wh, v)
	case "comment":
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET COMMENT = '%s'`, wh, escStr(value))
	case "maxClusterCount":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET MAX_CLUSTER_COUNT = %s`, wh, v)
	case "minClusterCount":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET MIN_CLUSTER_COUNT = %s`, wh, v)
	case "scalingPolicy":
		v, err := checkEnum(value, "STANDARD", "ECONOMY")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET SCALING_POLICY = %s`, wh, v)
	case "resourceMonitor":
		if strings.TrimSpace(value) == "" {
			query = fmt.Sprintf(`ALTER WAREHOUSE %s SET RESOURCE_MONITOR = NULL`, wh)
		} else {
			query = fmt.Sprintf(`ALTER WAREHOUSE %s SET RESOURCE_MONITOR = "%s"`, wh, escId(value))
		}
	case "enableQueryAcceleration":
		v, err := checkEnum(value, "TRUE", "FALSE")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET ENABLE_QUERY_ACCELERATION = %s`, wh, v)
	case "queryAccelerationMaxScaleFactor":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET QUERY_ACCELERATION_MAX_SCALE_FACTOR = %s`, wh, v)
	case "maxConcurrencyLevel":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET MAX_CONCURRENCY_LEVEL = %s`, wh, v)
	case "statementQueuedTimeout":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = %s`, wh, v)
	case "statementTimeout":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET STATEMENT_TIMEOUT_IN_SECONDS = %s`, wh, v)
	default:
		return fmt.Errorf("unknown warehouse property: %s", property)
	}
	_, err := a.client.Execute(a.ctx, query)
	return err
}

// AlterWarehouseSuspend suspends the named warehouse.
func (a *App) AlterWarehouseSuspend(name string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	_, err := a.client.Execute(a.ctx, fmt.Sprintf(`ALTER WAREHOUSE "%s" SUSPEND`, escId(name)))
	return err
}

// AlterWarehouseResume resumes the named warehouse if it is suspended.
func (a *App) AlterWarehouseResume(name string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	_, err := a.client.Execute(a.ctx, fmt.Sprintf(`ALTER WAREHOUSE "%s" RESUME IF SUSPENDED`, escId(name)))
	return err
}

// AlterWarehouseAbortAllQueries issues ABORT ALL QUERIES on the named warehouse.
func (a *App) AlterWarehouseAbortAllQueries(name string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	_, err := a.client.Execute(a.ctx, fmt.Sprintf(`ALTER WAREHOUSE "%s" ABORT ALL QUERIES`, escId(name)))
	return err
}

// AlterWarehouseRename renames a warehouse and returns the new name.
func (a *App) AlterWarehouseRename(name, newName string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	_, err := a.client.Execute(a.ctx, fmt.Sprintf(`ALTER WAREHOUSE "%s" RENAME TO "%s"`, escId(name), escId(newName)))
	return err
}

// GetWarehouseParameters returns per-warehouse parameter overrides (MAX_CONCURRENCY_LEVEL,
// STATEMENT_QUEUED_TIMEOUT_IN_SECONDS, STATEMENT_TIMEOUT_IN_SECONDS) sourced from
// SHOW PARAMETERS IN WAREHOUSE. The returned map key is the parameter name.
func (a *App) GetWarehouseParameters(name string) ([]PropertyPair, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	qr, err := a.client.Execute(a.ctx, fmt.Sprintf(`SHOW PARAMETERS IN WAREHOUSE "%s"`, escId(name)))
	if err != nil {
		return nil, err
	}
	want := map[string]bool{
		"MAX_CONCURRENCY_LEVEL":               true,
		"STATEMENT_QUEUED_TIMEOUT_IN_SECONDS": true,
		"STATEMENT_TIMEOUT_IN_SECONDS":        true,
	}
	// Find column indices for "key" and "value".
	keyIdx, valIdx := -1, -1
	for i, c := range qr.Columns {
		switch strings.ToLower(c) {
		case "key":
			keyIdx = i
		case "value":
			valIdx = i
		}
	}
	var result []PropertyPair
	for _, row := range qr.Rows {
		if keyIdx < 0 || keyIdx >= len(row) {
			continue
		}
		key := fmt.Sprint(row[keyIdx])
		val := ""
		if valIdx >= 0 && valIdx < len(row) && row[valIdx] != nil {
			val = fmt.Sprint(row[valIdx])
		}
		if want[strings.ToUpper(key)] {
			result = append(result, PropertyPair{Key: key, Value: val})
		}
	}
	return result, nil
}

// ListUsers returns all users visible to the current role.
// Returns an error if the role lacks the required privilege.
func (a *App) ListUsers() ([]snowflake.SnowflakeUser, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListUsers(a.ctx)
}

// GetUserDDL returns a CREATE USER DDL statement for the given user.
func (a *App) GetUserDDL(name string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	return a.client.GetUserDDL(a.ctx, name)
}

// CanManageUsers returns true when the current role can alter or drop users.
func (a *App) CanManageUsers() (bool, error) {
	if a.client == nil {
		return false, ErrNotConnected
	}
	return a.client.CanManageUsers(a.ctx)
}

// CanCreateUsers returns true when the current role can create users.
func (a *App) CanCreateUsers() (bool, error) {
	if a.client == nil {
		return false, ErrNotConnected
	}
	return a.client.CanCreateUsers(a.ctx)
}

// CanModifyUserAuth returns true when the current session role (or any role it
// inherits) has OWNERSHIP or MODIFY PROGRAMMATIC AUTHENTICATION METHODS on the
// named user.
func (a *App) CanModifyUserAuth(username string) (bool, error) {
	if a.client == nil {
		return false, ErrNotConnected
	}
	return a.client.CanModifyUserAuth(a.ctx, username)
}

// CheckAvailableKeyTools returns the list of available key generation methods.
// "go" (Go built-in crypto) is always present. "openssl" and "ssh-keygen" are
// included only when their executables are found on PATH.
func (a *App) CheckAvailableKeyTools() []string {
	tools := []string{"go"}
	if _, err := exec.LookPath("openssl"); err == nil {
		tools = append(tools, "openssl")
	}
	if _, err := exec.LookPath("ssh-keygen"); err == nil {
		tools = append(tools, "ssh-keygen")
	}
	return tools
}

// GenerateKeyPair generates an RSA-2048 key pair using the specified method.
//
//   - "go"        — pure Go crypto (PKCS#8, no passphrase support)
//   - "openssl"   — openssl CLI (PKCS#8; passphrase encrypts the private key)
//   - "ssh-keygen"— ssh-keygen CLI (OpenSSH/PKCS8 private key; passphrase
//     encrypts the private key; public key saved as PKCS8 PEM)
func (a *App) GenerateKeyPair(method, privateKeyPath, passphrase string) (KeyPairResult, error) {
	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0700); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot create directory: %w", err)
	}
	switch method {
	case "go":
		return generateKeyPairGo(privateKeyPath)
	case "openssl":
		return generateKeyPairOpenSSL(privateKeyPath, passphrase)
	case "ssh-keygen":
		return generateKeyPairSSHKeygen(privateKeyPath, passphrase)
	default:
		return KeyPairResult{}, fmt.Errorf("unknown key generation method %q", method)
	}
}

// generateKeyPairGo uses the Go standard library to produce an unencrypted
// RSA-2048 PKCS#8 private key and a PKIX public key. Passphrase is not supported.
func generateKeyPairGo(privateKeyPath string) (KeyPairResult, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("key generation failed: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("PKCS8 marshal failed: %w", err)
	}
	var privBuf bytes.Buffer
	if err = pem.Encode(&privBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: privDER}); err != nil {
		return KeyPairResult{}, err
	}
	if err = os.WriteFile(privateKeyPath, privBuf.Bytes(), 0600); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot write private key: %w", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("public key marshal failed: %w", err)
	}
	ext := filepath.Ext(privateKeyPath)
	pubKeyPath := strings.TrimSuffix(privateKeyPath, ext) + "_pub.pem"
	var pubBuf bytes.Buffer
	if err = pem.Encode(&pubBuf, &pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}); err != nil {
		return KeyPairResult{}, err
	}
	if err = os.WriteFile(pubKeyPath, pubBuf.Bytes(), 0644); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot write public key: %w", err)
	}
	return KeyPairResult{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  pubKeyPath,
		PublicKey:      stripPEMContent(pubBuf.String()),
	}, nil
}

// generateKeyPairOpenSSL uses the openssl CLI to produce a PKCS#8 private key
// (encrypted with AES-256 if passphrase is non-empty) and a PKIX public key.
func generateKeyPairOpenSSL(privateKeyPath, passphrase string) (KeyPairResult, error) {
	tool, err := exec.LookPath("openssl")
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("openssl not found in PATH")
	}

	// Step 1: generate raw RSA-2048 key (piped, never saved to disk).
	rawPEM, err := exec.Command(tool, "genrsa", "2048").Output()
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("openssl genrsa failed: %w", err)
	}

	// Step 2: convert to PKCS#8 and write to privateKeyPath.
	pkcs8Args := []string{"pkcs8", "-topk8", "-inform", "PEM", "-outform", "PEM", "-out", privateKeyPath}
	if passphrase != "" {
		pkcs8Args = append(pkcs8Args, "-passout", "pass:"+passphrase)
	} else {
		pkcs8Args = append(pkcs8Args, "-nocrypt")
	}
	pkcs8Cmd := exec.Command(tool, pkcs8Args...)
	pkcs8Cmd.Stdin = strings.NewReader(string(rawPEM))
	if out, err2 := pkcs8Cmd.CombinedOutput(); err2 != nil {
		return KeyPairResult{}, fmt.Errorf("openssl pkcs8 failed: %s", strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(privateKeyPath, 0600)

	// Step 3: extract public key.
	ext := filepath.Ext(privateKeyPath)
	pubKeyPath := strings.TrimSuffix(privateKeyPath, ext) + "_pub.pem"
	rsaArgs := []string{"rsa", "-in", privateKeyPath, "-pubout", "-out", pubKeyPath}
	if passphrase != "" {
		rsaArgs = append(rsaArgs, "-passin", "pass:"+passphrase)
	}
	if out, err2 := exec.Command(tool, rsaArgs...).CombinedOutput(); err2 != nil {
		return KeyPairResult{}, fmt.Errorf("openssl rsa pubout failed: %s", strings.TrimSpace(string(out)))
	}

	pubPEM, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot read public key: %w", err)
	}
	return KeyPairResult{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  pubKeyPath,
		PublicKey:      stripPEMContent(string(pubPEM)),
	}, nil
}

// generateKeyPairSSHKeygen uses ssh-keygen to produce an RSA-2048 private key
// (encrypted if passphrase is non-empty) and converts the public key to
// PKCS#8 PEM format suitable for Snowflake.
func generateKeyPairSSHKeygen(privateKeyPath, passphrase string) (KeyPairResult, error) {
	tool, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("ssh-keygen not found in PATH")
	}

	// Generate RSA-2048 key pair. -N sets the passphrase ("" = none).
	genArgs := []string{"-t", "rsa", "-b", "2048", "-f", privateKeyPath, "-N", passphrase}
	if out, err2 := exec.Command(tool, genArgs...).CombinedOutput(); err2 != nil {
		return KeyPairResult{}, fmt.Errorf("ssh-keygen failed: %s", strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(privateKeyPath, 0600)

	// Convert the OpenSSH public key to PKCS#8 PEM format for Snowflake.
	sshPubPath := privateKeyPath + ".pub"
	pubPEMBytes, err := exec.Command(tool, "-e", "-m", "pkcs8", "-f", sshPubPath).Output()
	if err != nil {
		return KeyPairResult{}, fmt.Errorf("ssh-keygen public key export failed: %w", err)
	}
	ext := filepath.Ext(privateKeyPath)
	pubKeyPath := strings.TrimSuffix(privateKeyPath, ext) + "_pub.pem"
	if err = os.WriteFile(pubKeyPath, pubPEMBytes, 0644); err != nil {
		return KeyPairResult{}, fmt.Errorf("cannot write public key: %w", err)
	}
	return KeyPairResult{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  pubKeyPath,
		PublicKey:      stripPEMContent(string(pubPEMBytes)),
	}, nil
}

// stripPEMContent returns the base64 payload of a PEM block with all header,
// footer, and blank lines removed — the format expected by ALTER USER SET RSA_PUBLIC_KEY.
func stripPEMContent(pemStr string) string {
	var lines []string
	for _, line := range strings.Split(pemStr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "-----") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "")
}

// SetUserPublicKey applies an RSA public key to a Snowflake user.
func (a *App) SetUserPublicKey(username, publicKey string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	esc := strings.ReplaceAll(username, `"`, `""`)
	sq := strings.ReplaceAll(publicKey, "'", "''")
	_, err := a.client.Execute(a.ctx, fmt.Sprintf(`ALTER USER "%s" SET RSA_PUBLIC_KEY='%s'`, esc, sq))
	return err
}

// ListNotificationIntegrations returns the names of all notification integrations.
func (a *App) ListNotificationIntegrations() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListNotificationIntegrations(a.ctx)
}

// ListExternalVolumes returns the names of all external volumes visible to the current role.
func (a *App) ListExternalVolumes() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListExternalVolumes(a.ctx)
}

// ListIntegrations runs SHOW <kind> INTEGRATIONS and returns structured rows.
// kind may be "STORAGE", "API", "CATALOG", "EXTERNAL ACCESS", "NOTIFICATION", or "SECURITY".
func (a *App) ListIntegrations(kind string) ([]snowflake.IntegrationRow, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListIntegrations(a.ctx, kind)
}

// GetIntegrationProperties runs DESCRIBE INTEGRATION for the named integration
// and returns the result as key/value pairs.
func (a *App) GetIntegrationProperties(name string) ([]PropertyPair, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	esc := strings.ReplaceAll(name, `"`, `""`)
	res, err := a.client.Execute(a.ctx, fmt.Sprintf(`DESCRIBE INTEGRATION "%s"`, esc))
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return []PropertyPair{}, nil
	}
	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	// DESCRIBE INTEGRATION returns rows of (property, property_type, property_value, property_default)
	// We return property / property_value pairs.
	var pairs []PropertyPair
	for _, row := range res.Rows {
		if len(row) < 3 {
			continue
		}
		k := toString(row[0])
		v := toString(row[2])
		if k != "" {
			pairs = append(pairs, PropertyPair{Key: k, Value: v})
		}
	}
	return pairs, nil
}

// DropIntegration drops the named integration.
func (a *App) DropIntegration(name string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.DropIntegration(a.ctx, name)
}

// CanCreateIntegration returns true when the current role can create integrations.
func (a *App) CanCreateIntegration() (bool, error) {
	if a.client == nil {
		return false, ErrNotConnected
	}
	return a.client.CanCreateIntegration(a.ctx)
}

// UseRole switches the session to the given role.
func (a *App) UseRole(role string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.UseRole(a.ctx, role)
}

// UseWarehouse switches the session to the given warehouse.
func (a *App) UseWarehouse(warehouse string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.UseWarehouse(a.ctx, warehouse)
}

// UseDatabase switches the session to the given database.
func (a *App) UseDatabase(database string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.UseDatabase(a.ctx, database)
}

// UseSchema switches the session to the given schema.
func (a *App) UseSchema(schema string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.UseSchema(a.ctx, schema)
}

// GetCurrentRegion returns the result of SELECT CURRENT_REGION(), which
// encodes both the cloud provider and the deployment region, e.g.
// "AWS_US_EAST_1", "AZURE_EASTUS2", or "GCP_US_CENTRAL1".
func (a *App) GetCurrentRegion() (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	qr, err := a.client.Execute(a.ctx, `SELECT CURRENT_REGION()`)
	if err != nil {
		return "", err
	}
	if len(qr.Rows) > 0 && len(qr.Rows[0]) > 0 && qr.Rows[0][0] != nil {
		return fmt.Sprint(qr.Rows[0][0]), nil
	}
	return "", nil
}

// GetSnowsightURL returns the Snowsight login page URL for the current account,
// formed as https://<org>-<account>.snowflakecomputing.com using
// CURRENT_ORGANIZATION_NAME() and CURRENT_ACCOUNT_NAME().
func (a *App) GetSnowsightURL() (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	qr, err := a.client.Execute(a.ctx, `SELECT 'https://' || LOWER(CURRENT_ORGANIZATION_NAME()) || '-' || LOWER(CURRENT_ACCOUNT_NAME()) || '.snowflakecomputing.com'`)
	if err != nil {
		return "", err
	}
	if len(qr.Rows) > 0 && len(qr.Rows[0]) > 0 && qr.Rows[0][0] != nil {
		return fmt.Sprint(qr.Rows[0][0]), nil
	}
	return "", nil
}

// GetCurrentUser returns the result of SELECT CURRENT_USER(), which reflects
// the canonical Snowflake username exactly as stored (preserving case).
func (a *App) GetCurrentUser() (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	qr, err := a.client.Execute(a.ctx, `SELECT CURRENT_USER()`)
	if err != nil {
		return "", err
	}
	if len(qr.Rows) > 0 && len(qr.Rows[0]) > 0 && qr.Rows[0][0] != nil {
		return fmt.Sprint(qr.Rows[0][0]), nil
	}
	return "", nil
}

// ListDatabases returns all databases visible to the current role.
func (a *App) ListDatabases() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListDatabases(a.ctx)
}

// ListSchemas returns all schemas in the given database.
func (a *App) ListSchemas(database string) ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListSchemas(a.ctx, database)
}

// ListObjects returns tables, views, etc. inside a schema.
func (a *App) ListObjects(database, schema string) ([]snowflake.SnowflakeObject, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListObjects(a.ctx, database, schema)
}

// GetDatabaseRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given database. Returns 1 if the value cannot be determined.
func (a *App) GetDatabaseRetentionDays(dbName string) (int, error) {
	if a.client == nil {
		return 0, ErrNotConnected
	}
	return a.client.GetDatabaseRetentionDays(a.ctx, dbName)
}

// GetTableRetentionDays returns the Time Travel data retention period in days
// for the given table. Returns 1 if the value cannot be determined.
func (a *App) GetTableRetentionDays(database, schema, name string) (int, error) {
	if a.client == nil {
		return 0, ErrNotConnected
	}
	return a.client.GetTableRetentionDays(a.ctx, database, schema, name)
}

// ListDroppedTables returns tables in the schema that are within the Time Travel
// retention window and can be recovered with UNDROP TABLE.
func (a *App) ListDroppedTables(database, schema string) ([]snowflake.DroppedTable, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListDroppedTables(a.ctx, database, schema)
}

// ListDroppedSchemas returns schemas in the database that are within the Time
// Travel retention window and can be recovered with UNDROP SCHEMA.
func (a *App) ListDroppedSchemas(database string) ([]snowflake.DroppedTable, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListDroppedSchemas(a.ctx, database)
}

// ListDroppedDatabases returns databases that are within the Time Travel
// retention window and can be recovered with UNDROP DATABASE.
func (a *App) ListDroppedDatabases() ([]snowflake.DroppedTable, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListDroppedDatabases(a.ctx)
}

// GetProcedureParams fetches the DDL for a stored procedure and returns its
// parameter list with real parameter names parsed from the DDL.
func (a *App) GetProcedureParams(database, schema, name, argTypes string) ([]snowflake.ProcParam, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetProcedureParams(a.ctx, database, schema, name, argTypes)
}

// GetTableColumns returns the ordered column names for a table or view.
func (a *App) GetTableColumns(database, schema, name string) ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetTableColumns(a.ctx, database, schema, name)
}

// GetTableForeignKeys returns the foreign keys where the given table is the
// referencing side. Used by the editor's JOIN ON autocomplete.
func (a *App) GetTableForeignKeys(database, schema, table string) ([]snowflake.TableForeignKey, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetTableForeignKeys(a.ctx, database, schema, table)
}

// GetTableColumnsWithTypes returns ordered column names and data types for a
// table or view. Used by the editor's JOIN ON autocomplete for type-compatible
// same-name column suggestions.
func (a *App) GetTableColumnsWithTypes(database, schema, name string) ([]snowflake.ColumnInfo, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetTableColumnsWithTypes(a.ctx, database, schema, name)
}

// GetSchemaForeignKeys returns all FK→PK column mappings in the given schema
// from INFORMATION_SCHEMA. Used by the editor to bulk-warm FK data for the
// JOIN ON autocomplete instead of issuing per-table SHOW IMPORTED KEYS calls.
func (a *App) GetSchemaForeignKeys(database, schema string) ([]snowflake.TableForeignKey, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetSchemaForeignKeys(a.ctx, database, schema)
}

// ── SQL editor analysis IPC methods ───────────────────────────────────────────
// These methods expose the proprietary SQL analysis algorithms (previously in
// the TypeScript frontend) as backend IPC calls so the logic is protected by
// Go binary obfuscation rather than being visible in the JS bundle.

// AnalyzeSqlSyntax runs the custom Snowflake SQL tokenizer on the given text
// and returns structural error markers (unclosed strings, unmatched parens,
// bad scripting assignments, etc.).  No Snowflake connection is required.
func (a *App) AnalyzeSqlSyntax(sql string) []sqleditor.DiagMarker {
	return sqleditor.ValidateSyntax(sql)
}

// ParseJoinTableRefs extracts all FROM/JOIN table references (with aliases)
// from the given SQL text.  No Snowflake connection is required.
func (a *App) ParseJoinTableRefs(sql string) []sqleditor.JoinTableRef {
	return sqleditor.ParseJoinTables(sql)
}

// ComputeJoinOnConditions computes JOIN ON / USING condition suggestions using
// FK constraints, PK naming heuristics, and type-compatible same-name columns.
// The caller is responsible for fetching and passing FK and column data;
// no Snowflake connection is required by this method.
func (a *App) ComputeJoinOnConditions(req sqleditor.JoinOnSuggestionsReq) []sqleditor.JoinCondition {
	return sqleditor.ComputeJoinOnConditions(req)
}

// AnalyzeSqlSemantics validates alias.column references in SQL against the
// provided column info, returning Warning markers for unrecognised column names.
// No Snowflake connection is required.
func (a *App) AnalyzeSqlSemantics(sql string, resolvedRefs []sqleditor.ResolvedRef, colEntries []sqleditor.ColEntry) []sqleditor.DiagMarker {
	return sqleditor.ValidateSemantics(sql, resolvedRefs, colEntries)
}

// GetScriptingCompletions extracts declared Snowflake Scripting variables
// visible at cursorOffset and determines whether a ':' prefix is required for
// completions. No Snowflake connection is required.
func (a *App) GetScriptingCompletions(sql string, cursorOffset int) sqleditor.ScriptingCompletionResult {
	return sqleditor.GetScriptingCompletions(sql, cursorOffset)
}

// GetSqlStatementRanges splits sql into per-statement line ranges and byte offsets.
// No Snowflake connection is required.
func (a *App) GetSqlStatementRanges(sql string) []sqleditor.StatementRange {
	return sqleditor.GetStatementRanges(sql)
}

// GetIdentifierAtColumn parses the dot-separated identifier (e.g. db.schema.table)
// under the zero-indexed cursor column col within a single line of SQL.
// Returns nil when the column is not on any identifier.
// No Snowflake connection is required.
func (a *App) GetIdentifierAtColumn(line string, col int) []string {
	return sqleditor.GetIdentifierAtColumn(line, col)
}

// FindSqlTokenPositions walks sql and returns the line/column positions of bare
// words in bareTargets and double-quoted identifiers in quotedTargets, skipping
// string literals and comments.  No Snowflake connection is required.
func (a *App) FindSqlTokenPositions(sql string, bareTargets []string, quotedTargets []string) []sqleditor.TokenMatch {
	return sqleditor.FindTokenPositions(sql, bareTargets, quotedTargets)
}

// GetActiveFunctionCall parses the SQL prefix (text from document start to
// cursor) and returns the innermost open function call with its active parameter
// index.  Returns nil when the cursor is not inside a named function call.
// No Snowflake connection is required.
func (a *App) GetActiveFunctionCall(prefix string) *sqleditor.FunctionCallContext {
	return sqleditor.GetActiveFunctionCall(prefix)
}

// ParseSignatureParams extracts the byte spans of each parameter within a
// function signature string for Monaco parameter-label highlighting.
// No Snowflake connection is required.
func (a *App) ParseSignatureParams(sig string) []sqleditor.SignatureParam {
	return sqleditor.ParseSignatureParams(sig)
}

// ApplySqlCasing applies token-level keyword/identifier/function casing to a
// formatted SQL string.  Quoted identifiers, string literals, dollar-quoted
// blocks, and comments are passed through unchanged.
// No Snowflake connection is required.
func (a *App) ApplySqlCasing(sql, keywordCase, identifierCase, functionCase string) string {
	return sqleditor.ApplyCasing(sql, keywordCase, identifierCase, functionCase)
}

// GetFunctionInfo fetches the DDL for a user-defined function and returns its
// parameter list together with a flag indicating whether it is a table function.
func (a *App) GetFunctionInfo(database, schema, name, argTypes string) (*snowflake.FunctionInfo, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.GetFunctionInfo(a.ctx, database, schema, name, argTypes)
}

// GetObjectDDL returns the definition of a single schema object using
// Snowflake's GET_DDL function. kind should be one of: TABLE, VIEW, FUNCTION,
// PROCEDURE, SEQUENCE, STAGE, STREAM, TASK, FILE FORMAT, PIPE.
// For procedures and functions, arguments must be the parameter type list
// (e.g. "NUMBER, VARCHAR") so Snowflake can resolve the correct overload.
// Pass an empty string for all other object kinds.
func (a *App) GetObjectDDL(database, schema, kind, name, arguments string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	return a.client.GetObjectDDL(a.ctx, database, schema, kind, name, arguments)
}

// GetObjectDependencies parses the DDL of a VIEW, PROCEDURE, or FUNCTION and
// returns a recursive tree of objects it depends on.  Tables are leaf nodes;
// views and SQL-language procedures/functions are expanded recursively.
// arguments should be the parameter type list for procedures/functions
// (e.g. "NUMBER, VARCHAR") or an empty string for views.
func (a *App) GetObjectDependencies(database, schema, kind, name, arguments string) (snowflake.DependencyNode, error) {
	if a.client == nil {
		return snowflake.DependencyNode{}, ErrNotConnected
	}
	return a.client.GetObjectDependencies(a.ctx, database, schema, kind, name, arguments)
}

// PropertyPair is a single key/value property row returned by GetObjectProperties.
type PropertyPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GetObjectProperties returns structured metadata for any Snowflake object by
// running the appropriate SHOW or DESCRIBE command and returning the result as
// key/value pairs. kind is one of: TABLE, VIEW, FUNCTION, PROCEDURE, SEQUENCE,
// STAGE, STREAM, TASK, FILE FORMAT, PIPE, WAREHOUSE, ROLE, USER.
func (a *App) GetObjectProperties(database, schema, kind, name string) ([]PropertyPair, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}

	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	like := strings.ReplaceAll(name, `\`, `\\`)
	like = strings.ReplaceAll(like, "'", "''")

	var query string
	switch strings.ToUpper(kind) {
	case "DATABASE":
		query = fmt.Sprintf("SHOW DATABASES LIKE '%s'", like)
	case "SCHEMA":
		query = fmt.Sprintf("SHOW SCHEMAS LIKE '%s' IN DATABASE %s", like, q(database))
	case "TABLE":
		query = fmt.Sprintf("SHOW TABLES LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "VIEW":
		query = fmt.Sprintf("SHOW VIEWS LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "FUNCTION":
		query = fmt.Sprintf("SHOW FUNCTIONS LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "PROCEDURE":
		query = fmt.Sprintf("SHOW PROCEDURES LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "SEQUENCE":
		query = fmt.Sprintf("SHOW SEQUENCES LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "STAGE":
		query = fmt.Sprintf("SHOW STAGES LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "STREAM":
		query = fmt.Sprintf("SHOW STREAMS LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "TASK":
		query = fmt.Sprintf("SHOW TASKS LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "FILE FORMAT":
		query = fmt.Sprintf("SHOW FILE FORMATS LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "PIPE":
		query = fmt.Sprintf("SHOW PIPES LIKE '%s' IN SCHEMA %s.%s", like, q(database), q(schema))
	case "WAREHOUSE":
		query = fmt.Sprintf("SHOW WAREHOUSES LIKE '%s'", like)
	case "ROLE":
		query = fmt.Sprintf("SHOW ROLES LIKE '%s'", like)
	case "USER":
		query = fmt.Sprintf("DESCRIBE USER %s", q(name))
	default:
		return nil, fmt.Errorf("unsupported object kind: %s", kind)
	}

	res, err := a.client.Execute(a.ctx, query)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return []PropertyPair{}, nil
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	var pairs []PropertyPair
	if strings.ToUpper(kind) == "USER" {
		// DESCRIBE USER returns rows of (property, value, default) — use property/value columns.
		for _, row := range res.Rows {
			if len(row) < 2 {
				continue
			}
			k := toString(row[0])
			v := toString(row[1])
			if k != "" {
				pairs = append(pairs, PropertyPair{Key: k, Value: v})
			}
		}
	} else {
		// SHOW commands: first matching row; each column name is the property key.
		row := res.Rows[0]
		for i, col := range res.Columns {
			val := ""
			if i < len(row) {
				val = toString(row[i])
			}
			pairs = append(pairs, PropertyPair{Key: col, Value: val})
		}
	}
	return pairs, nil
}

// SessionParam holds one row from SHOW PARAMETERS.
type SessionParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// SessionVar holds one row from SHOW VARIABLES.
type SessionVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// BackupPolicyRow holds one row from SHOW BACKUP POLICIES.
type BackupPolicyRow struct {
	Name            string `json:"name"`
	CreatedOn       string `json:"createdOn"`
	Owner           string `json:"owner"`
	Schedule        string `json:"schedule"`
	ExpireAfterDays int64  `json:"expireAfterDays"`
	RetentionLock   bool   `json:"retentionLock"`
	Comment         string `json:"comment"`
}

// BackupRow holds one row from SHOW BACKUPS IN BACKUP SET.
type BackupRow struct {
	ID        string `json:"id"`        // UUID used in IDENTIFIER clause of CREATE ... FROM BACKUP SET
	Name      string `json:"name"`      // human-readable name / timestamp label
	CreatedOn string `json:"createdOn"`
	Status    string `json:"status"`
	SizeBytes int64  `json:"sizeBytes"`
	Comment   string `json:"comment"`
}

// BackupSetRow holds one row from SHOW BACKUP SETS.
type BackupSetRow struct {
	Name            string `json:"name"`
	BackupSetDb     string `json:"backupSetDb"`
	BackupSetSchema string `json:"backupSetSchema"`
	CreatedOn       string `json:"createdOn"`
	ObjectType      string `json:"objectType"`
	ObjectName      string `json:"objectName"`
	ObjectDb        string `json:"objectDb"`
	ObjectSchema    string `json:"objectSchema"`
	Status          string `json:"status"`
	Comment         string `json:"comment"`
}

// WarehouseMeteringRow holds one row from ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY.
type WarehouseMeteringRow struct {
	StartTime                string  `json:"startTime"`
	EndTime                  string  `json:"endTime"`
	WarehouseName            string  `json:"warehouseName"`
	CreditsUsed              float64 `json:"creditsUsed"`
	CreditsUsedCompute       float64 `json:"creditsUsedCompute"`
	CreditsUsedCloudServices float64 `json:"creditsUsedCloudServices"`
}

// QueryHistoryRow holds one row from INFORMATION_SCHEMA.QUERY_HISTORY*.
type QueryHistoryRow struct {
	QueryID       string `json:"queryId"`
	QueryText     string `json:"queryText"`
	QueryType     string `json:"queryType"`
	UserName      string `json:"userName"`
	WarehouseName string `json:"warehouseName"`
	DatabaseName  string `json:"databaseName"`
	SchemaName    string `json:"schemaName"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	ElapsedMs     int64  `json:"elapsedMs"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"errorMessage"`
	RowsProduced  int64  `json:"rowsProduced"`
	BytesScanned  int64  `json:"bytesScanned"`
}

// colIdx returns the index of the first column whose lowercase name matches any
// of the given alternatives, or -1 if none match.
func colIdx(cols []string, names ...string) int {
	for i, c := range cols {
		lc := strings.ToLower(c)
		for _, n := range names {
			if lc == n {
				return i
			}
		}
	}
	return -1
}

// GetSessionParameters returns the current session parameters from SHOW PARAMETERS IN SESSION.
func (a *App) GetSessionParameters() ([]SessionParam, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	res, err := a.client.Execute(a.ctx, "SHOW PARAMETERS IN SESSION")
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case string:
			return t
		case []byte:
			return string(t)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	// SHOW PARAMETERS columns: key, value, default, level, description, type
	keyIdx  := colIdx(res.Columns, "key", "name")
	valIdx  := colIdx(res.Columns, "value")
	typIdx  := colIdx(res.Columns, "type")
	descIdx := colIdx(res.Columns, "description")

	var params []SessionParam
	for _, row := range res.Rows {
		key, val, typ, desc := "", "", "", ""
		if keyIdx >= 0 && keyIdx < len(row) { key = toString(row[keyIdx]) }
		if valIdx >= 0 && valIdx < len(row)  { val = toString(row[valIdx]) }
		if typIdx >= 0 && typIdx < len(row)  { typ = toString(row[typIdx]) }
		if descIdx >= 0 && descIdx < len(row) { desc = toString(row[descIdx]) }
		if key != "" {
			params = append(params, SessionParam{Key: key, Value: val, Type: typ, Description: desc})
		}
	}
	if params == nil {
		params = []SessionParam{}
	}
	return params, nil
}

// GetSessionVariables returns the current session variables from SHOW VARIABLES.
func (a *App) GetSessionVariables() ([]SessionVar, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	res, err := a.client.Execute(a.ctx, "SHOW VARIABLES")
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case string:
			return t
		case []byte:
			return string(t)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	// SHOW VARIABLES columns: name, value, default, type, ...
	nameIdx := colIdx(res.Columns, "name", "key")
	valIdx  := colIdx(res.Columns, "value")
	typIdx  := colIdx(res.Columns, "type")

	var vars []SessionVar
	for _, row := range res.Rows {
		name, val, typ := "", "", ""
		if nameIdx >= 0 && nameIdx < len(row) { name = toString(row[nameIdx]) }
		if valIdx >= 0 && valIdx < len(row)   { val = toString(row[valIdx]) }
		if typIdx >= 0 && typIdx < len(row)   { typ = toString(row[typIdx]) }
		if name != "" {
			vars = append(vars, SessionVar{Key: name, Value: val, Type: typ})
		}
	}
	if vars == nil {
		vars = []SessionVar{}
	}
	return vars, nil
}

// quoteIfString wraps value in single quotes (with escaping) when paramType
// indicates a string-like type; returns value unchanged for booleans/numbers.
func quoteIfString(value, paramType string) string {
	switch strings.ToUpper(paramType) {
	case "BOOLEAN", "NUMBER", "FIXED", "FLOAT":
		return value
	default:
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	}
}

// SetSessionParameter applies ALTER SESSION SET key = value for the given parameter.
func (a *App) SetSessionParameter(name, value, paramType string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	valExpr := quoteIfString(value, paramType)
	_, err := a.client.Execute(a.ctx, "ALTER SESSION SET "+name+" = "+valExpr)
	return err
}

// SetSessionVariable applies SET name = value for the given session variable.
func (a *App) SetSessionVariable(name, value, varType string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	valExpr := quoteIfString(value, varType)
	_, err := a.client.Execute(a.ctx, "SET "+name+" = "+valExpr)
	return err
}

// ColumnComment holds a column name and its optional comment.
type ColumnComment struct {
	Column  string `json:"column"`
	Comment string `json:"comment"`
}

// GetColumnComments returns the comment for every column in a table, ordered
// by ordinal position.
func (a *App) GetColumnComments(database, schema, table string) ([]ColumnComment, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	escStr := func(s string) string { return strings.ReplaceAll(s, `'`, `''`) }
	query := fmt.Sprintf(
		`SELECT COLUMN_NAME, COALESCE(COMMENT, '') AS COMMENT`+
			` FROM "%s".INFORMATION_SCHEMA.COLUMNS`+
			` WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'`+
			` ORDER BY ORDINAL_POSITION`,
		escId(database), escStr(strings.ToUpper(schema)), escStr(strings.ToUpper(table)),
	)
	res, err := a.client.Execute(a.ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]ColumnComment, 0, len(res.Rows))
	for _, row := range res.Rows {
		col, cmt := "", ""
		if len(row) > 0 && row[0] != nil {
			col = fmt.Sprint(row[0])
		}
		if len(row) > 1 && row[1] != nil {
			cmt = fmt.Sprint(row[1])
		}
		out = append(out, ColumnComment{Column: col, Comment: cmt})
	}
	return out, nil
}

// SetColumnComment sets (or clears) the COMMENT on a single table column.
func (a *App) SetColumnComment(database, schema, table, column, comment string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	escStr := func(s string) string { return strings.ReplaceAll(s, `'`, `''`) }
	query := fmt.Sprintf(
		`ALTER TABLE "%s"."%s"."%s" MODIFY COLUMN "%s" COMMENT '%s'`,
		escId(database), escId(schema), escId(table), escId(column), escStr(comment),
	)
	_, err := a.client.Execute(a.ctx, query)
	return err
}

// TableSettings holds the modifiable table-level properties that can be
// changed via ALTER TABLE ... SET without re-creating the table.
type TableSettings struct {
	ClusterBy             string `json:"clusterBy"`
	EnableSchemaEvolution bool   `json:"enableSchemaEvolution"`
	DataRetentionDays     int    `json:"dataRetentionDays"`
	MaxDataExtensionDays  int    `json:"maxDataExtensionDays"`
	ChangeTracking        bool   `json:"changeTracking"`
	DefaultDDLCollation   string `json:"defaultDDLCollation"`
	Comment               string `json:"comment"`
}

// GetTableSettings reads the current values of all modifiable table properties
// by running SHOW TABLES and (for collation) SHOW PARAMETERS.
func (a *App) GetTableSettings(database, schema, table string) (TableSettings, error) {
	if a.client == nil {
		return TableSettings{}, ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	escStr := func(s string) string { return strings.ReplaceAll(s, `'`, `''`) }

	res, err := a.client.Execute(a.ctx, fmt.Sprintf(
		`SHOW TABLES LIKE '%s' IN SCHEMA "%s"."%s"`,
		escStr(table), escId(database), escId(schema),
	))
	if err != nil {
		return TableSettings{}, err
	}

	// Build column-name → index map (case-insensitive).
	colIdx := make(map[string]int, len(res.Columns))
	for i, c := range res.Columns {
		colIdx[strings.ToLower(c)] = i
	}

	// Find the row whose name matches exactly (LIKE can return partial matches).
	var row []interface{}
	for _, r := range res.Rows {
		idx, ok := colIdx["name"]
		if ok && idx < len(r) && r[idx] != nil && strings.EqualFold(fmt.Sprint(r[idx]), table) {
			row = r
			break
		}
	}
	if row == nil {
		return TableSettings{}, fmt.Errorf("table %q not found", table)
	}

	get := func(name string) string {
		idx, ok := colIdx[name]
		if !ok || idx >= len(row) || row[idx] == nil {
			return ""
		}
		return fmt.Sprint(row[idx])
	}
	parseBool := func(s string) bool {
		s = strings.ToLower(strings.TrimSpace(s))
		return s == "y" || s == "true" || s == "on" || s == "1"
	}
	parseInt := func(s string) int {
		var n int
		_, _ = fmt.Sscanf(s, "%d", &n)
		return n
	}

	settings := TableSettings{
		ClusterBy:             get("cluster_by"),
		EnableSchemaEvolution: parseBool(get("enable_schema_evolution")),
		DataRetentionDays:     parseInt(get("retention_time")),
		MaxDataExtensionDays:  parseInt(get("max_data_extension_time_in_days")),
		ChangeTracking:        parseBool(get("change_tracking")),
		Comment:               get("comment"),
		DefaultDDLCollation:   get("default_ddl_collation"),
	}

	// Fallback: read DEFAULT_DDL_COLLATION from SHOW PARAMETERS if not in SHOW TABLES.
	if settings.DefaultDDLCollation == "" {
		pres, perr := a.client.Execute(a.ctx, fmt.Sprintf(
			`SHOW PARAMETERS LIKE 'DEFAULT_DDL_COLLATION' IN TABLE "%s"."%s"."%s"`,
			escId(database), escId(schema), escId(table),
		))
		if perr == nil && len(pres.Rows) > 0 {
			pidx := make(map[string]int, len(pres.Columns))
			for i, c := range pres.Columns {
				pidx[strings.ToLower(c)] = i
			}
			if vi, ok := pidx["value"]; ok && vi < len(pres.Rows[0]) && pres.Rows[0][vi] != nil {
				settings.DefaultDDLCollation = fmt.Sprint(pres.Rows[0][vi])
			}
		}
	}

	return settings, nil
}

// AlterTableProperty applies a single ALTER TABLE SET change.
// property must be one of: clusterBy, enableSchemaEvolution, dataRetentionDays,
// maxDataExtensionDays, changeTracking, defaultDDLCollation, comment.
func (a *App) AlterTableProperty(database, schema, table, property, value string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	escId := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	escStr := func(s string) string { return strings.ReplaceAll(s, `'`, `''`) }
	tbl := fmt.Sprintf(`"%s"."%s"."%s"`, escId(database), escId(schema), escId(table))

	var query string
	switch property {
	case "clusterBy":
		if strings.TrimSpace(value) == "" {
			query = fmt.Sprintf(`ALTER TABLE %s DROP CLUSTERING KEY`, tbl)
		} else {
			query = fmt.Sprintf(`ALTER TABLE %s CLUSTER BY (%s)`, tbl, value)
		}
	case "enableSchemaEvolution":
		query = fmt.Sprintf(`ALTER TABLE %s SET ENABLE_SCHEMA_EVOLUTION = %s`, tbl, strings.ToUpper(value))
	case "dataRetentionDays":
		query = fmt.Sprintf(`ALTER TABLE %s SET DATA_RETENTION_TIME_IN_DAYS = %s`, tbl, value)
	case "maxDataExtensionDays":
		query = fmt.Sprintf(`ALTER TABLE %s SET MAX_DATA_EXTENSION_TIME_IN_DAYS = %s`, tbl, value)
	case "changeTracking":
		query = fmt.Sprintf(`ALTER TABLE %s SET CHANGE_TRACKING = %s`, tbl, strings.ToUpper(value))
	case "defaultDDLCollation":
		query = fmt.Sprintf(`ALTER TABLE %s SET DEFAULT_DDL_COLLATION = '%s'`, tbl, escStr(value))
	case "comment":
		query = fmt.Sprintf(`ALTER TABLE %s SET COMMENT = '%s'`, tbl, escStr(value))
	default:
		return fmt.Errorf("unknown property: %s", property)
	}

	_, err := a.client.Execute(a.ctx, query)
	return err
}

// ExportTableData exports a Snowflake table to the local filesystem using a
// temporary internal stage. The stage is dropped automatically after the
// download completes or on error.
func (a *App) ExportTableData(params snowflake.ExportTableParams) (snowflake.ExportTableResult, error) {
	if a.client == nil {
		return snowflake.ExportTableResult{}, ErrNotConnected
	}
	return a.client.ExportTableData(a.ctx, params)
}

// ImportTableData imports a local file into a Snowflake table using a temporary
// internal stage. The stage is dropped automatically after the upload completes
// or on error.
func (a *App) ImportTableData(params snowflake.ImportTableParams) (snowflake.ImportTableResult, error) {
	if a.client == nil {
		return snowflake.ImportTableResult{}, ErrNotConnected
	}
	return a.client.ImportTableData(a.ctx, params)
}

// AlterTask runs an ALTER TASK IF EXISTS statement on the given task.
// clause is everything that follows the task name in the ALTER statement,
// for example "RESUME", "SUSPEND", "SET COMMENT = 'hello'", or
// "MODIFY AS SELECT 1". The caller is responsible for correct SQL quoting
// inside the clause; this method only double-quotes the task identifier.
// ExecDDL executes an arbitrary DDL/DML statement and discards the result set.
// It is intended for one-shot statements (CREATE, ALTER, DROP, etc.) where the
// caller needs to know whether the statement succeeded without routing the SQL
// through the editor's query pipeline.
func (a *App) ExecDDL(sql string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

func (a *App) AlterTask(database, schema, name, clause string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	sql := fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s %s", q(database), q(schema), q(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// TaskFinalizabilityRow describes a task and whether it can serve as a finalizer.
// DisabledReason is empty for eligible tasks; non-empty with a human-readable
// explanation for tasks that cannot be finalizers.
type TaskFinalizabilityRow struct {
	Name           string `json:"name"`
	DisabledReason string `json:"disabledReason"`
}

// TaskStatusRow holds the current state and last-run information for a single task.
type TaskStatusRow struct {
	Name         string `json:"name"`
	TaskState    string `json:"taskState"`    // STARTED | SUSPENDED
	Predecessors string `json:"predecessors"` // raw predecessor string from SHOW TASKS
	LastRunState string `json:"lastRunState"` //nolint:misspell // SUCCEEDED | FAILED | RUNNING | SKIPPED | CANCELLED | ""
	LastRunTime  string `json:"lastRunTime"`  // ISO-8601 timestamp or ""
	ErrorMsg     string `json:"errorMsg"`     // exception text when last run failed
	Finalize     string `json:"finalize"`     // fully-qualified root task name for finalizer tasks, "" otherwise
}

// TaskStatusesResult wraps the per-task rows and an optional history-query
// error message.  HistoryError is non-empty when INFORMATION_SCHEMA.TASK_HISTORY
// could not be queried (e.g. insufficient privileges); in that case Rows still
// contain the task names and STARTED/SUSPENDED states from SHOW TASKS.
type TaskStatusesResult struct {
	Rows         []TaskStatusRow `json:"rows"`
	HistoryError string          `json:"historyError"`
}

// GetTaskStatuses returns the current state and last-run result for every task
// ListFinalizableTasks returns every task in the schema along with an
// eligibility verdict.  DisabledReason is empty for tasks that can be used as
// a finalizer; non-empty with a short human-readable reason for tasks that
// cannot (has predecessors, has a schedule, has child tasks, or is already a
// finalizer for another root task).
//
// Eligible tasks are returned first, ineligible tasks sorted after.
func (a *App) ListFinalizableTasks(database, schema string) ([]TaskFinalizabilityRow, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	res, err := a.client.Execute(a.ctx,
		fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	nameIdx, predsIdx, schedIdx, finalizeIdx := -1, -1, -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "predecessors", "predecessor":
			predsIdx = i
		case "schedule":
			schedIdx = i
		case "finalize", "finalize_task":
			finalizeIdx = i
		}
	}
	if nameIdx < 0 {
		return nil, nil
	}

	type taskMeta struct {
		name     string
		preds    string
		schedule string
		finalize string
	}
	metas := make([]taskMeta, 0, len(res.Rows))
	for _, row := range res.Rows {
		name := ""
		if nameIdx < len(row) {
			name = toString(row[nameIdx])
		}
		if name == "" {
			continue
		}
		preds := ""
		if predsIdx >= 0 && predsIdx < len(row) {
			preds = toString(row[predsIdx])
		}
		sched := ""
		if schedIdx >= 0 && schedIdx < len(row) {
			sched = toString(row[schedIdx])
		}
		fin := ""
		if finalizeIdx >= 0 && finalizeIdx < len(row) {
			fin = toString(row[finalizeIdx])
		}
		metas = append(metas, taskMeta{name: name, preds: preds, schedule: sched, finalize: fin})
	}

	// Build the set of task names that appear as a predecessor of any other task.
	hasChildren := make(map[string]bool, len(metas))
	for _, r := range metas {
		p := r.preds
		if p == "" || p == "[]" || p == "<nil>" {
			continue
		}
		p = strings.TrimPrefix(p, "[")
		p = strings.TrimSuffix(p, "]")
		for _, part := range strings.Split(p, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			bare := strings.Trim(segs[len(segs)-1], `"`)
			if bare != "" {
				hasChildren[strings.ToUpper(bare)] = true
			}
		}
	}

	isBlank := func(s string) bool { return s == "" || s == "[]" || s == "<nil>" }

	var eligible, disabled []TaskFinalizabilityRow
	for _, r := range metas {
		var reason string
		switch {
		case !isBlank(r.preds):
			reason = "Already a child task (has predecessors)"
		case r.schedule != "" && r.schedule != "null":
			reason = "Has its own schedule"
		case hasChildren[strings.ToUpper(r.name)]:
			reason = "Has child tasks"
		case r.finalize != "" && r.finalize != "null":
			reason = "Already a finalizer for another task"
		}
		row := TaskFinalizabilityRow{Name: r.name, DisabledReason: reason}
		if reason == "" {
			eligible = append(eligible, row)
		} else {
			disabled = append(disabled, row)
		}
	}
	return append(eligible, disabled...), nil
}

// ListRootTasks returns task finalizability rows for the given schema.
// Deprecated: use ListFinalizableTasks directly.
func (a *App) ListRootTasks(database, schema string) ([]TaskFinalizabilityRow, error) {
	return a.ListFinalizableTasks(database, schema)
}

// TaskHasChildren reports whether any task in the schema lists taskName as a
// predecessor (i.e. the task has at least one dependent / child task).
func (a *App) TaskHasChildren(database, schema, taskName string) (bool, error) {
	if a.client == nil {
		return false, ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	res, err := a.client.Execute(a.ctx,
		fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return false, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	predsIdx := -1
	for i, col := range res.Columns {
		if strings.ToLower(col) == "predecessors" || strings.ToLower(col) == "predecessor" {
			predsIdx = i
			break
		}
	}
	if predsIdx < 0 {
		return false, nil
	}

	upper := strings.ToUpper(taskName)
	for _, row := range res.Rows {
		preds := ""
		if predsIdx < len(row) {
			preds = toString(row[predsIdx])
		}
		if preds == "" || preds == "[]" || preds == "<nil>" {
			continue
		}
		p := strings.TrimPrefix(preds, "[")
		p = strings.TrimSuffix(p, "]")
		for _, part := range strings.Split(p, ",") {
			part = strings.TrimSpace(part)
			segs := strings.Split(part, ".")
			bare := strings.ToUpper(strings.Trim(segs[len(segs)-1], `"`))
			if bare == upper {
				return true, nil
			}
		}
	}
	return false, nil
}

// EnableTaskDependents resumes the named task and all of its descendants.
// Tasks are resumed in leaf-first (post-order) so that children are active
// before their parent, which Snowflake requires when enabling a task graph.
// SYSTEM$TASK_DEPENDENTS_ENABLE is intentionally NOT used here because:
//   (a) it is unavailable in some Snowflake editions,
//   (b) in many editions it does not resume the root task itself.
// Instead we build the dependency graph from SHOW TASKS and issue individual
// ALTER TASK … RESUME statements in the correct order.
func (a *App) EnableTaskDependents(database, schema, taskName string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case []interface{}:
			// gosnowflake may decode VARIANT array columns as []interface{}.
			// Re-encode as a bracket-comma string so the bracket-strip parser below works.
			parts := make([]string, 0, len(t))
			for _, el := range t {
				if el != nil {
					parts = append(parts, fmt.Sprintf("%v", el))
				}
			}
			return "[" + strings.Join(parts, ",") + "]"
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	// 1. Fetch all tasks in the schema to build the dependency graph.
	res, err := a.client.Execute(a.ctx,
		fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return err
	}

	nameIdx, predsIdx := -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "predecessors", "predecessor":
			predsIdx = i
		}
	}
	if nameIdx < 0 {
		// No tasks found — just resume the root and return.
		_, err = a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", q(database), q(schema), q(taskName)))
		return err
	}

	// Build a children map: parent (upper) → []child names (original case).
	children := make(map[string][]string)
	taskNames := make(map[string]string) // upper → original case
	for _, row := range res.Rows {
		name := ""
		if nameIdx < len(row) {
			name = toString(row[nameIdx])
		}
		if name == "" {
			continue
		}
		taskNames[strings.ToUpper(name)] = name
		if predsIdx < 0 || predsIdx >= len(row) {
			continue
		}
		preds := toString(row[predsIdx])
		if preds == "" || preds == "[]" || preds == "<nil>" || preds == "null" {
			continue
		}
		preds = strings.TrimPrefix(preds, "[")
		preds = strings.TrimSuffix(preds, "]")
		for _, part := range strings.Split(preds, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			parent := strings.ToUpper(strings.Trim(segs[len(segs)-1], `"`))
			children[parent] = append(children[parent], name)
		}
	}

	// 2. Collect all descendants via BFS, then resume in leaf-first (post-order).
	rootUpper := strings.ToUpper(taskName)
	visited := map[string]bool{rootUpper: true}
	bfsOrder := []string{rootUpper}
	queue := []string{rootUpper}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			cu := strings.ToUpper(child)
			if !visited[cu] {
				visited[cu] = true
				bfsOrder = append(bfsOrder, cu)
				queue = append(queue, cu)
			}
		}
	}

	// Resume in reverse BFS order (leaves first) so Snowflake accepts each RESUME.
	for i := len(bfsOrder) - 1; i >= 0; i-- {
		upper := bfsOrder[i]
		name := upper
		if orig, ok := taskNames[upper]; ok {
			name = orig
		}
		if _, err := a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", q(database), q(schema), q(name))); err != nil {
			return fmt.Errorf("resuming task %q: %w", name, err)
		}
	}
	return nil
}

// SuspendTaskList suspends each task in the provided list in order.
// The caller is responsible for the correct ordering: the root task should
// appear first so it stops scheduling new runs before its children are touched.
// This is used by the frontend which already has the full graph state and can
// compute the correct order without re-parsing SHOW TASKS predecessor columns.
func (a *App) SuspendTaskList(database, schema string, names []string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	for _, name := range names {
		if _, err := a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s SUSPEND", q(database), q(schema), q(name))); err != nil {
			return fmt.Errorf("suspending task %q: %w", name, err)
		}
	}
	return nil
}

// ResumeTaskList resumes each task in the provided list in order.
// The caller is responsible for the correct ordering: leaf tasks should appear
// first and the root task last, since Snowflake requires all predecessor tasks
// to be STARTED before a successor task can be resumed.
// This is used by the frontend which already has the full graph state and can
// compute the correct order without re-parsing SHOW TASKS predecessor columns.
func (a *App) ResumeTaskList(database, schema string, names []string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	for _, name := range names {
		if _, err := a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s RESUME", q(database), q(schema), q(name))); err != nil {
			return fmt.Errorf("resuming task %q: %w", name, err)
		}
	}
	return nil
}

// SuspendTaskGraph suspends the root task first (to stop it from scheduling new
// runs) and then suspends every descendant task in the graph.  It uses SHOW
// TASKS IN SCHEMA to build the dependency graph and does a BFS from the root
// task to find all descendants before issuing ALTER TASK … SUSPEND for each.
func (a *App) SuspendTaskGraph(database, schema, taskName string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case []interface{}:
			parts := make([]string, 0, len(t))
			for _, el := range t {
				if el != nil {
					parts = append(parts, fmt.Sprintf("%v", el))
				}
			}
			return "[" + strings.Join(parts, ",") + "]"
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	// 1. Suspend the root task first so it cannot schedule new child runs.
	if _, err := a.client.Execute(a.ctx,
		fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s SUSPEND", q(database), q(schema), q(taskName))); err != nil {
		return err
	}

	// 2. Fetch all tasks in the schema to build the dependency graph.
	res, err := a.client.Execute(a.ctx,
		fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return err
	}

	nameIdx, predsIdx := -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "predecessors", "predecessor":
			predsIdx = i
		}
	}
	if nameIdx < 0 {
		return nil // no name column — nothing more to do
	}

	// Build a children map: parent (upper) → []child names (original case).
	children := make(map[string][]string)
	taskNames := make(map[string]string) // upper → original case
	for _, row := range res.Rows {
		name := ""
		if nameIdx < len(row) {
			name = toString(row[nameIdx])
		}
		if name == "" {
			continue
		}
		taskNames[strings.ToUpper(name)] = name
		if predsIdx < 0 || predsIdx >= len(row) {
			continue
		}
		preds := toString(row[predsIdx])
		if preds == "" || preds == "[]" || preds == "<nil>" || preds == "null" {
			continue
		}
		preds = strings.TrimPrefix(preds, "[")
		preds = strings.TrimSuffix(preds, "]")
		for _, part := range strings.Split(preds, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			parent := strings.ToUpper(strings.Trim(segs[len(segs)-1], `"`))
			children[parent] = append(children[parent], name)
		}
	}

	// 3. BFS to collect all descendants of the root task.
	rootUpper := strings.ToUpper(taskName)
	visited := map[string]bool{rootUpper: true}
	queue := []string{rootUpper}
	var descendants []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			cu := strings.ToUpper(child)
			if !visited[cu] {
				visited[cu] = true
				descendants = append(descendants, child)
				queue = append(queue, cu)
			}
		}
	}

	// 4. Suspend each descendant.
	for _, child := range descendants {
		// Use the original-case name from SHOW TASKS when available.
		if orig, ok := taskNames[strings.ToUpper(child)]; ok {
			child = orig
		}
		if _, err := a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s SUSPEND", q(database), q(schema), q(child))); err != nil {
			return fmt.Errorf("suspending child task %q: %w", child, err)
		}
	}
	return nil
}

// DropTaskTree suspends and drops the named task and all of its descendants.
// Tasks are processed in post-order (leaves first, root last) so children are
// always removed before their parent. Each task is suspended before it is
// dropped; SUSPEND failures are treated as non-fatal (the task may already be
// suspended or not exist). The DROP uses IF EXISTS so missing tasks are skipped
// silently. Returns the first DROP error encountered, if any.
func (a *App) DropTaskTree(database, schema, taskName string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	// List all tasks to build the dependency graph.
	res, err := a.client.Execute(a.ctx,
		fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return err
	}

	nameIdx, predsIdx := -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "predecessors", "predecessor":
			predsIdx = i
		}
	}
	if nameIdx < 0 {
		return fmt.Errorf("SHOW TASKS did not return a name column")
	}

	childrenOf := make(map[string][]string) // UPPER(parent) → []child original-case names
	taskNames := make(map[string]string)    // UPPER(name) → original-case name
	for _, row := range res.Rows {
		name := ""
		if nameIdx < len(row) {
			name = toString(row[nameIdx])
		}
		if name == "" {
			continue
		}
		taskNames[strings.ToUpper(name)] = name
		if predsIdx < 0 || predsIdx >= len(row) {
			continue
		}
		preds := toString(row[predsIdx])
		if preds == "" || preds == "[]" || preds == "<nil>" {
			continue
		}
		preds = strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(preds), "]"), "[")
		for _, part := range strings.Split(preds, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.Split(part, ".")
			parent := strings.ToUpper(strings.Trim(segs[len(segs)-1], `"`))
			childrenOf[parent] = append(childrenOf[parent], name)
		}
	}

	// Post-order DFS: collect tasks in leaf-first order.
	var dropOrder []string
	visited := make(map[string]bool)
	var dfs func(name string)
	dfs = func(name string) {
		upper := strings.ToUpper(name)
		if visited[upper] {
			return
		}
		visited[upper] = true
		for _, child := range childrenOf[upper] {
			dfs(child)
		}
		// Use original-case name if known.
		if orig, ok := taskNames[upper]; ok {
			name = orig
		}
		dropOrder = append(dropOrder, name)
	}
	dfs(taskName)

	// Suspend then drop each task (leaves first).
	for _, name := range dropOrder {
		_, _ = a.client.Execute(a.ctx,
			fmt.Sprintf("ALTER TASK IF EXISTS %s.%s.%s SUSPEND", q(database), q(schema), q(name)))
		if _, err := a.client.Execute(a.ctx,
			fmt.Sprintf("DROP TASK IF EXISTS %s.%s.%s", q(database), q(schema), q(name))); err != nil {
			return fmt.Errorf("dropping task %q: %w", name, err)
		}
	}
	return nil
}

// in the given schema.  It runs two queries:
//  1. SHOW TASKS IN SCHEMA — yields the task list and their STARTED/SUSPENDED state.
//  2. INFORMATION_SCHEMA.TASK_HISTORY — yields the most-recent run status for
//     each task within the last 14 days.  History rows are returned ordered by
//     SCHEDULED_TIME DESC and deduplicated in Go so no window-function syntax is
//     required.  A history-query failure is reported in HistoryError rather than
//     as a hard error, so callers always receive the task list.
func (a *App) GetTaskStatuses(database, schema string) (TaskStatusesResult, error) {
	if a.client == nil {
		return TaskStatusesResult{}, ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			// Use RFC3339 so JavaScript's Date constructor can parse it reliably.
			// fmt.Sprintf("%v", t) produces "2006-01-02 15:04:05 +0000 UTC" which
			// the V8 Date parser rejects due to the trailing " UTC" abbreviation.
			if t.IsZero() {
				return ""
			}
			return t.UTC().Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	colIdx := func(cols []string, names ...string) int {
		for i, c := range cols {
			lc := strings.ToLower(c)
			for _, n := range names {
				if lc == n {
					return i
				}
			}
		}
		return -1
	}

	// ── Step 1: list all tasks and their current state ────────────────────────
	showRes, err := a.client.Execute(a.ctx,
		fmt.Sprintf("SHOW TASKS IN SCHEMA %s.%s", q(database), q(schema)))
	if err != nil {
		return TaskStatusesResult{}, err
	}

	nameIdx     := colIdx(showRes.Columns, "name")
	stateIdx    := colIdx(showRes.Columns, "state")
	predsIdx    := colIdx(showRes.Columns, "predecessors", "predecessor")
	finalizeIdx := colIdx(showRes.Columns, "finalize", "finalize_task")
	taskRelIdx  := colIdx(showRes.Columns, "task_relations")

	// extractFinalize reads the finalize root-task reference from a task_relations
	// VARIANT value. gosnowflake may decode VARIANT columns as:
	//   • map[string]interface{} — already-parsed JSON object
	//   • string / []byte       — raw JSON text
	// Key comparison is case-insensitive to handle Snowflake edition variations.
	extractFinalize := func(v interface{}) string {
		if v == nil {
			return ""
		}
		isFinalizeKey := func(k string) bool {
			lk := strings.ToLower(k)
			return lk == "finalize" || lk == "finalize_task"
		}
		tryMap := func(m map[string]interface{}) string {
			for k, val := range m {
				if isFinalizeKey(k) && val != nil {
					if s := fmt.Sprintf("%v", val); s != "" && s != "<nil>" && s != "null" {
						return s
					}
				}
			}
			return ""
		}
		if m, ok := v.(map[string]interface{}); ok {
			return tryMap(m)
		}
		raw := ""
		switch t := v.(type) {
		case string:
			raw = t
		case []byte:
			raw = string(t)
		default:
			raw = fmt.Sprintf("%v", v)
		}
		if raw == "" || raw == "null" {
			return ""
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return ""
		}
		for k, val := range m {
			if isFinalizeKey(k) {
				var s string
				if err := json.Unmarshal(val, &s); err == nil && s != "" {
					return s
				}
			}
		}
		return ""
	}

	// Debug: log columns + first few rows so finalizer detection issues are visible.
	logger.L.Debug("SHOW TASKS columns", "cols", showRes.Columns, "finalizeIdx", finalizeIdx, "taskRelIdx", taskRelIdx)
	for i, row := range showRes.Rows {
		if i >= 5 {
			break
		}
		nm := ""
		if nameIdx >= 0 && nameIdx < len(row) {
			nm = toString(row[nameIdx])
		}
		tr := ""
		if taskRelIdx >= 0 && taskRelIdx < len(row) {
			tr = fmt.Sprintf("%#v", row[taskRelIdx])
		}
		fi := ""
		if finalizeIdx >= 0 && finalizeIdx < len(row) {
			fi = fmt.Sprintf("%#v", row[finalizeIdx])
		}
		logger.L.Debug("SHOW TASKS row", "i", i, "name", nm, "task_relations", tr, "finalize_col", fi)
	}

	type entry struct {
		name         string
		taskState    string
		predecessors string
		finalize     string
	}
	var tasks []entry
	nameMap := map[string]int{} // UPPER(name) → index in tasks slice
	for _, row := range showRes.Rows {
		name := ""
		if nameIdx >= 0 && nameIdx < len(row) {
			name = toString(row[nameIdx])
		}
		if name == "" {
			continue
		}
		state := ""
		if stateIdx >= 0 && stateIdx < len(row) {
			state = toString(row[stateIdx])
		}
		preds := ""
		if predsIdx >= 0 && predsIdx < len(row) {
			preds = toString(row[predsIdx])
		}
		// Finalize: try dedicated column first, then parse from task_relations VARIANT.
		finalize := ""
		if finalizeIdx >= 0 && finalizeIdx < len(row) {
			finalize = toString(row[finalizeIdx])
		}
		if finalize == "" && taskRelIdx >= 0 && taskRelIdx < len(row) {
			finalize = extractFinalize(row[taskRelIdx])
		}
		nameMap[strings.ToUpper(name)] = len(tasks)
		tasks = append(tasks, entry{name: name, taskState: strings.ToUpper(state), predecessors: preds, finalize: finalize})
	}

	rows := make([]TaskStatusRow, len(tasks))
	for i, t := range tasks {
		rows[i] = TaskStatusRow{Name: t.name, TaskState: t.taskState, Predecessors: t.predecessors, Finalize: t.finalize}
	}

	// ── Optional: detect finalizer tasks via GET_DDL ──────────────────────────
	// SHOW TASKS may not expose the FINALIZE relationship in all Snowflake
	// editions (task_relations column absent or in unexpected format). As a
	// reliable fallback, run GET_DDL for any truly standalone task (no
	// predecessors, no children) whose finalize field is still empty, and
	// parse the FINALIZE clause out of the DDL text.
	{
		// Build set of task names that appear as predecessors (i.e. have children).
		hasChildrenSet := make(map[string]bool, len(tasks))
		for _, t := range tasks {
			if t.predecessors == "" || t.predecessors == "[]" || t.predecessors == "<nil>" {
				continue
			}
			cleaned := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(t.predecessors), "]"), "[")
			for _, part := range strings.Split(cleaned, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				segs := strings.Split(part, ".")
				bare := strings.Trim(segs[len(segs)-1], `"`)
				if bare != "" {
					hasChildrenSet[strings.ToUpper(bare)] = true
				}
			}
		}

		parseFinalizeFromDDL := func(ddl string) string {
			// Find "FINALIZE" keyword (case-insensitive) followed by "=".
			upper := strings.ToUpper(ddl)
			idx := strings.Index(upper, "FINALIZE")
			if idx < 0 {
				return ""
			}
			rest := strings.TrimSpace(ddl[idx+len("FINALIZE"):])
			if len(rest) == 0 || rest[0] != '=' {
				return ""
			}
			rest = strings.TrimSpace(rest[1:])
			// Read until whitespace; strip trailing punctuation.
			end := strings.IndexAny(rest, " \t\n\r")
			if end < 0 {
				end = len(rest)
			}
			return strings.TrimRight(rest[:end], ";,")
		}

		for i, t := range tasks {
			if rows[i].Finalize != "" {
				continue // already detected via task_relations column
			}
			if t.predecessors != "" && t.predecessors != "[]" && t.predecessors != "<nil>" {
				continue // has predecessors — cannot be a finalizer
			}
			if hasChildrenSet[strings.ToUpper(t.name)] {
				continue // has children — it is a root/parent task, not a finalizer
			}
			// Standalone task: use GET_DDL to check for FINALIZE clause.
			fqn := fmt.Sprintf(`"%s"."%s"."%s"`,
				strings.ReplaceAll(database, `"`, `""`),
				strings.ReplaceAll(schema, `"`, `""`),
				strings.ReplaceAll(t.name, `"`, `""`),
			)
			ddlRes, ddlErr := a.client.Execute(a.ctx,
				fmt.Sprintf("SELECT GET_DDL('TASK', '%s')", strings.ReplaceAll(fqn, "'", "''")))
			if ddlErr != nil || len(ddlRes.Rows) == 0 || len(ddlRes.Rows[0]) == 0 {
				continue
			}
			ddl := toString(ddlRes.Rows[0][0])
			if fin := parseFinalizeFromDDL(ddl); fin != "" {
				rows[i].Finalize = fin
				logger.L.Info("finalizer detected via GET_DDL", "task", t.name, "finalize", fin)
			}
		}
	}

	// ── Step 2: fetch run history (best-effort) ───────────────────────────────
	// Use SELECT * so we never hard-code column names — Snowflake editions differ
	// on whether columns are called TASK_NAME/RUN_STATUS or NAME/STATE etc.
	// Schema filtering and deduplication (most-recent run per task) are done in
	// Go so no WHERE clause column names are assumed either.
	// RESULT_LIMIT => 10000 gives us up to 10 000 recent runs across all tasks.
	// Primary sort: SCHEDULED_TIME DESC NULLS FIRST so manually-triggered runs
	// (EXECUTE TASK sets SCHEDULED_TIME = NULL) appear before older scheduled
	// runs.  Secondary sort: COMPLETED_TIME DESC NULLS FIRST so that among
	// multiple manual runs with the same NULL scheduled time the most recently
	// completed entry wins the deduplication; a currently-executing run (NULL
	// COMPLETED_TIME) floats to the very top via NULLS FIRST.
	histSQL := fmt.Sprintf(
		`SELECT * FROM TABLE(%s.INFORMATION_SCHEMA.TASK_HISTORY(`+
			`SCHEDULED_TIME_RANGE_START => DATEADD('day', -7, CURRENT_TIMESTAMP()),`+
			`RESULT_LIMIT => 10000))`+
			` ORDER BY SCHEDULED_TIME DESC NULLS FIRST, COMPLETED_TIME DESC NULLS FIRST`,
		q(database))

	histRes, histErr := a.client.Execute(a.ctx, histSQL)
	if histErr != nil {
		return TaskStatusesResult{Rows: rows, HistoryError: histErr.Error()}, nil
	}

	// Locate columns by trying multiple known naming conventions.
	tnIdx := colIdx(histRes.Columns, "task_name", "name")
	rsIdx := colIdx(histRes.Columns, "run_status", "state", "status")
	ctIdx := colIdx(histRes.Columns, "completed_time", "completion_time")
	qsIdx := colIdx(histRes.Columns, "query_start_time", "start_time")
	exIdx := colIdx(histRes.Columns, "exception_text", "error_message", "error_msg")
	scIdx := colIdx(histRes.Columns, "task_schema", "schema_name", "schema")

	// toTime extracts a time.Time from a raw column value (nil → zero time).
	toTime := func(v interface{}) time.Time {
		if v == nil {
			return time.Time{}
		}
		if t, ok := v.(time.Time); ok {
			return t
		}
		return time.Time{}
	}

	// best tracks the most-recently-executed run per task.
	// sortKey = COMPLETED_TIME if non-zero, else QUERY_START_TIME.
	// This ensures that graph-summary / never-started rows (both columns NULL)
	// never shadow a real completed run, even if SQL ORDER BY puts them first.
	type bestEntry struct {
		sortKey  time.Time
		runState string
		runTime  string
		errorMsg string
	}
	best := map[string]bestEntry{}

	for _, row := range histRes.Rows {
		// Filter to this schema in Go (avoids relying on the WHERE column name).
		if scIdx >= 0 && scIdx < len(row) {
			rowSchema := strings.ToUpper(toString(row[scIdx]))
			if rowSchema != strings.ToUpper(schema) {
				continue
			}
		}

		taskName := ""
		if tnIdx >= 0 && tnIdx < len(row) {
			taskName = toString(row[tnIdx])
		}
		upper := strings.ToUpper(taskName)
		if _, ok := nameMap[upper]; !ok {
			continue
		}

		// Compute recency key: prefer COMPLETED_TIME, fall back to QUERY_START_TIME.
		var completedAt, queryStartAt time.Time
		if ctIdx >= 0 && ctIdx < len(row) {
			completedAt = toTime(row[ctIdx])
		}
		if qsIdx >= 0 && qsIdx < len(row) {
			queryStartAt = toTime(row[qsIdx])
		}
		sortKey := completedAt
		if sortKey.IsZero() {
			sortKey = queryStartAt
		}

		prev, hasPrev := best[upper]
		// Accept this row if:
		//   • no previous entry exists, OR
		//   • this row has a non-zero sortKey and it's newer than the previous one.
		// A zero-sortKey row (meta/summary entry) only wins if nothing else exists.
		if !hasPrev || (!sortKey.IsZero() && sortKey.After(prev.sortKey)) {
			e := bestEntry{sortKey: sortKey}
			if rsIdx >= 0 && rsIdx < len(row) {
				e.runState = toString(row[rsIdx])
			}
			if ctIdx >= 0 && ctIdx < len(row) {
				e.runTime = toString(row[ctIdx])
			}
			if exIdx >= 0 && exIdx < len(row) {
				e.errorMsg = toString(row[exIdx])
			}
			best[upper] = e
		}
	}

	// Write best entries into the result rows.
	for upper, e := range best {
		idx, ok := nameMap[upper]
		if !ok {
			continue
		}
		rows[idx].LastRunState = e.runState
		rows[idx].LastRunTime = e.runTime
		rows[idx].ErrorMsg = e.errorMsg
	}

	return TaskStatusesResult{Rows: rows}, nil
}

// ExecuteTask manually triggers a single run of a Snowflake Task.
// Pass a non-empty config JSON string to use USING CONFIG, or set
// retryLast to true to re-execute the last failed run.
func (a *App) ExecuteTask(database, schema, name, config string, retryLast bool) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.ExecuteTask(a.ctx, database, schema, name, config, retryLast)
}

// ExecuteNotebook runs EXECUTE NOTEBOOK against a Snowflake Notebook object and
// returns the resulting query ID. Each element of params is treated as a string
// literal value and is automatically single-quoted in the generated SQL.
func (a *App) ExecuteNotebook(database, schema, name string, params []string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	return a.client.ExecuteNotebook(a.ctx, database, schema, name, params)
}

// GetNotebookQueryWarehouse returns the QUERY_WAREHOUSE currently configured on
// the given Snowflake Notebook, or an empty string if none is set.
func (a *App) GetNotebookQueryWarehouse(database, schema, name string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	return a.client.GetNotebookQueryWarehouse(a.ctx, database, schema, name)
}

// SetNotebookQueryWarehouse updates the QUERY_WAREHOUSE property of the given
// Snowflake Notebook via ALTER NOTEBOOK … SET QUERY_WAREHOUSE.
func (a *App) SetNotebookQueryWarehouse(database, schema, name, warehouse string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.SetNotebookQueryWarehouse(a.ctx, database, schema, name, warehouse)
}

// FetchNotebookContent retrieves the content of a Snowflake Notebook object.
// It describes the notebook to find its stage URI, downloads the .ipynb file
// to a temporary local directory, reads the file, and returns the nbformat JSON.
// The temporary directory is cleaned up automatically.
func (a *App) FetchNotebookContent(database, schema, name string) (string, error) {
	if a.client == nil {
		return "", ErrNotConnected
	}
	return a.client.FetchNotebookContent(a.ctx, database, schema, name)
}

// DeployNotebook uploads a local .ipynb file to a temporary Snowflake internal
// stage and creates a NOTEBOOK object from it. The temporary stage is dropped
// automatically after the notebook is created (or on error).
func (a *App) DeployNotebook(params snowflake.DeployNotebookParams) error {
	if a.client == nil {
		return ErrNotConnected
	}
	return a.client.DeployNotebook(a.ctx, params)
}

// GetERDiagramData fetches column metadata, primary keys, and foreign keys for
// every table in the database and returns the data needed to render an Entity
// Relationship Diagram on the frontend.
func (a *App) GetERDiagramData(database string) (snowflake.ERDiagramData, error) {
	if a.client == nil {
		return snowflake.ERDiagramData{}, ErrNotConnected
	}
	return a.client.GetERDiagramData(a.ctx, database)
}

// ─── DDL export ───────────────────────────────────────────────────────────────

// ddlProgressEvent is the Wails event name emitted during export.
const ddlProgressEvent = "ddl:progress"

// DDLProgressPayload is the structure emitted with each ddl:progress event.
type DDLProgressPayload struct {
	Done   int            `json:"done"`
	Total  int            `json:"total"`
	Result ddl.ExportResult `json:"result"`
}

// ExportDatabaseDDL fetches the complete DDL for a single database via
// GET_DDL, splits it into one file per object, and writes the files under
// outputDir/<database>/.
//
// Progress is also emitted as a "ddl:progress" Wails event so the frontend
// can update a progress indicator in real time.
func (a *App) ExportDatabaseDDL(database, outputDir string) (ddl.ExportResult, error) {
	if a.client == nil {
		return ddl.ExportResult{}, ErrNotConnected
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.exportCancelFunc = cancel
	defer func() {
		cancel()
		a.exportCancelFunc = nil
	}()

	var pathTemplate string
	if cfg, err := config.Load(); err == nil {
		pathTemplate = cfg.Git.ExportPathTemplate
	}
	opts := ddl.ExportOptions{OutputDir: outputDir, PathTemplate: pathTemplate}

	var result ddl.ExportResult
	ddl.ExportDatabases(
		ctx,
		[]string{database},
		a.client.GetCompleteDatabaseDDL,
		opts,
		func(done, total int, res ddl.ExportResult) {
			result = res
			wailsruntime.EventsEmit(a.ctx, ddlProgressEvent, DDLProgressPayload{
				Done:   done,
				Total:  total,
				Result: res,
			})
		},
	)

	return result, nil
}

// ListExportableDatabases returns the names of all databases that can be
// exported (own databases; shared/imported databases such as
// SNOWFLAKE_SAMPLE_DATA are excluded).  The frontend uses this list to
// populate the database-selection checkboxes in the Export DDL panel.
func (a *App) ListExportableDatabases() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListExportableDatabases(a.ctx)
}

// ExportAllDatabasesDDL exports DDL for the given databases in parallel.
// When databases is nil or empty every exportable database owned by the
// account is exported (same behavior as before database selection was added).
//
// Progress events ("ddl:progress") are emitted after each database completes,
// allowing the frontend to show a live progress bar.
func (a *App) ExportAllDatabasesDDL(outputDir string, databases []string) ([]ddl.ExportResult, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}

	if len(databases) == 0 {
		var err error
		databases, err = a.client.ListExportableDatabases(a.ctx)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.exportCancelFunc = cancel
	defer func() {
		cancel()
		a.exportCancelFunc = nil
	}()

	var pathTemplate string
	if cfg, err := config.Load(); err == nil {
		pathTemplate = cfg.Git.ExportPathTemplate
	}
	opts := ddl.ExportOptions{OutputDir: outputDir, PathTemplate: pathTemplate}

	results := ddl.ExportDatabases(
		ctx,
		databases,
		a.client.GetCompleteDatabaseDDL,
		opts,
		func(done, total int, res ddl.ExportResult) {
			wailsruntime.EventsEmit(a.ctx, ddlProgressEvent, DDLProgressPayload{
				Done:   done,
				Total:  total,
				Result: res,
			})
		},
	)

	return results, nil
}

// ─── AI configuration ─────────────────────────────────────────────────────────

// GetAIConfig returns the persisted AI provider settings.
func (a *App) GetAIConfig() config.AIConfig {
	cfg, err := config.Load()
	if err != nil {
		return config.AIConfig{}
	}
	return cfg.AI
}

// SaveAIConfig persists AI provider settings to disk.
func (a *App) SaveAIConfig(aiCfg config.AIConfig) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.AI = aiCfg
	return config.Save(cfg)
}

// ─── Editor preferences ───────────────────────────────────────────────────────

// GetEditorPrefs returns the persisted SQL editor formatting preferences.
// Returns sensible defaults when the config does not exist yet.
func (a *App) GetEditorPrefs() config.EditorPrefs {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultEditorPrefs()
	}
	prefs := cfg.Editor
	// Back-fill any zero fields with defaults so callers always get a fully populated struct.
	defaults := config.DefaultEditorPrefs()
	if prefs.KeywordCase == "" {
		prefs.KeywordCase = defaults.KeywordCase
	}
	if prefs.IdentifierCase == "" {
		prefs.IdentifierCase = defaults.IdentifierCase
	}
	if prefs.FunctionCase == "" {
		prefs.FunctionCase = defaults.FunctionCase
	}
	if prefs.IndentStyle == "" {
		prefs.IndentStyle = defaults.IndentStyle
	}
	if prefs.IndentSize == 0 {
		prefs.IndentSize = defaults.IndentSize
	}
	if prefs.CommaPosition == "" {
		prefs.CommaPosition = defaults.CommaPosition
	}
	if prefs.OperatorPosition == "" {
		prefs.OperatorPosition = defaults.OperatorPosition
	}
	return prefs
}

// SaveEditorPrefs persists SQL editor formatting preferences to disk.
func (a *App) SaveEditorPrefs(prefs config.EditorPrefs) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Editor = prefs
	return config.Save(cfg)
}

// ─── Feature flags ────────────────────────────────────────────────────────────

// GetFeatureFlags returns the persisted feature flag settings.
// When the config predates feature flags (Initialized == false) the defaults
// are returned so every feature is enabled out of the box.
func (a *App) GetFeatureFlags() config.FeatureFlags {
	cfg, err := config.Load()
	if err != nil || !cfg.FeatureFlags.Initialized {
		return config.DefaultFeatureFlags()
	}
	return cfg.FeatureFlags
}

// SaveFeatureFlags persists feature flag settings to disk.
// Initialized is always set to true so subsequent loads use the saved values.
func (a *App) SaveFeatureFlags(flags config.FeatureFlags) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	flags.Initialized = true
	cfg.FeatureFlags = flags
	return config.Save(cfg)
}

// ListAIModels returns the models available for the given provider and API key.
// Returns nil (not an error) when the key is invalid or the request fails so
// the frontend can fall back to its static defaults.
// ollamaPort is the Ollama server port (0 = default 11434); ignored for other providers.
func (a *App) ListAIModels(provider, apiKey string, ollamaPort int) []string {
	models, err := ai.ListModels(provider, apiKey, ollamaPort)
	if err != nil {
		logger.L.Warn("failed to list AI models", "provider", provider, "err", err)
		return nil
	}
	return models
}

// TestAIModel makes a minimal one-token API call to verify that the given
// provider/key/model combination is valid and reachable.
// Returns an empty string on success or a human-readable error message.
// ollamaPort is the Ollama server port (0 = default 11434); ignored for other providers.
func (a *App) TestAIModel(provider, apiKey, model string, ollamaPort int) string {
	if err := ai.TestModel(provider, apiKey, model, ollamaPort); err != nil {
		return err.Error()
	}
	return ""
}

// SendChatMessage runs one agentic chat turn. currentSQL is the text currently
// in the editor (may be empty). lastResultSummary is a pre-formatted text
// summary of the most recent query result (may be empty). Both are injected
// into the system prompt so the AI has context without the user having to paste.
func (a *App) SendChatMessage(
	history []ai.UIMessage,
	userText string,
	currentSQL string,
	lastResultSummary string,
	agentMode bool,
) ([]ai.UIMessage, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	cfg, err := config.Load()
	if err != nil || !cfg.AI.Enabled || cfg.AI.APIKey == "" {
		return nil, fmt.Errorf("AI not configured or disabled")
	}

	workDir := cfg.Git.ExportDir

	chatCtx, cancel := context.WithCancel(a.ctx)
	a.cancelChat = cancel
	defer func() {
		cancel()
		a.cancelChat = nil
	}()

	executor := func(name, inputJSON string) (string, bool) {
		var args map[string]string
		json.Unmarshal([]byte(inputJSON), &args) //nolint:errcheck
		switch name {
		case "get_session_context":
			sc, err := a.client.GetSessionContext(a.ctx)
			if err != nil {
				return err.Error(), true
			}
			return fmt.Sprintf("role: %s\nwarehouse: %s\ndatabase: %s\nschema: %s",
				sc.Role, sc.Warehouse, sc.Database, sc.Schema), false
		case "list_databases":
			dbs, err := a.client.ListDatabases(a.ctx)
			if err != nil {
				return err.Error(), true
			}
			return strings.Join(dbs, "\n"), false
		case "list_schemas":
			schemas, err := a.client.ListSchemas(a.ctx, args["database"])
			if err != nil {
				return err.Error(), true
			}
			return strings.Join(schemas, "\n"), false
		case "list_tables":
			objs, err := a.client.ListObjects(a.ctx, args["database"], args["schema"])
			if err != nil {
				return err.Error(), true
			}
			lines := make([]string, len(objs))
			for i, o := range objs {
				lines[i] = o.Name + " (" + o.Kind + ")"
			}
			return strings.Join(lines, "\n"), false
		case "describe_table":
			esc := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
			query := fmt.Sprintf(`DESCRIBE TABLE "%s"."%s"."%s"`,
				esc(args["database"]), esc(args["schema"]), esc(args["table"]))
			res, err := a.client.Execute(a.ctx, query)
			if err != nil {
				return err.Error(), true
			}
			return formatDescribeResult(res), false
		case "run_sql":
			res, err := a.client.Execute(a.ctx, args["query"])
			if err != nil {
				return err.Error(), true
			}
			return formatChatQueryResult(res), false
		case "list_directory":
			p := args["path"]
			if !filepath.IsAbs(p) {
				p = filepath.Join(workDir, p)
			}
			p = filepath.Clean(p)
			entries, err := filesystem.ListDir(p)
			if err != nil {
				return err.Error(), true
			}
			lines := make([]string, len(entries))
			for i, e := range entries {
				if e.IsDir {
					lines[i] = e.Name + "/"
				} else {
					lines[i] = fmt.Sprintf("%s (%d bytes)", e.Name, e.Size)
				}
			}
			return strings.Join(lines, "\n"), false
		case "read_file":
			p := args["path"]
			if !filepath.IsAbs(p) {
				p = filepath.Join(workDir, p)
			}
			p = filepath.Clean(p)
			content, err := filesystem.ReadFile(p)
			if err != nil {
				return err.Error(), true
			}
			const maxBytes = 50_000
			if len(content) > maxBytes {
				content = content[:maxBytes] + "\n... (truncated)"
			}
			return content, false
		case "run_command":
			cmd := exec.CommandContext(chatCtx, "sh", "-c", args["command"])
			cmd.Dir = workDir
			out, err := cmd.CombinedOutput()
			output := strings.TrimSpace(string(out))
			if err != nil {
				if output != "" {
					return output, true
				}
				return err.Error(), true
			}
			const maxBytes = 50_000
			if len(output) > maxBytes {
				output = output[:maxBytes] + "\n... (truncated)"
			}
			return output, false
		}
		return "unknown tool", true
	}

	msg, err := ai.Chat(chatCtx, cfg.AI.Provider, cfg.AI.APIKey, cfg.AI.Model, cfg.AI.OllamaPort,
		history, userText, currentSQL, lastResultSummary, agentMode, workDir, executor)
	if err != nil {
		return nil, err
	}
	return []ai.UIMessage{{Role: "user", Text: userText}, msg}, nil
}

// formatDescribeResult extracts name and type columns from a DESCRIBE TABLE result.
func formatDescribeResult(res *snowflake.QueryResult) string {
	if res == nil {
		return "(no result)"
	}
	nameIdx, typeIdx := -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "type":
			typeIdx = i
		}
	}
	if nameIdx < 0 {
		return "(unexpected DESCRIBE result)"
	}
	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	var lines []string
	for _, row := range res.Rows {
		name := ""
		typ := ""
		if nameIdx < len(row) {
			name = toString(row[nameIdx])
		}
		if typeIdx >= 0 && typeIdx < len(row) {
			typ = toString(row[typeIdx])
		}
		if name == "" {
			continue
		}
		if typ != "" {
			lines = append(lines, name+" "+typ)
		} else {
			lines = append(lines, name)
		}
	}
	return strings.Join(lines, "\n")
}

// formatChatQueryResult renders up to 50 rows of a query result as a plain-text table.
func formatChatQueryResult(res *snowflake.QueryResult) string {
	if res == nil {
		return "(no result)"
	}
	toString := func(v interface{}) string {
		if v == nil {
			return "NULL"
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	var sb strings.Builder
	sb.WriteString(strings.Join(res.Columns, " | "))
	sb.WriteByte('\n')
	limit := len(res.Rows)
	if limit > 50 {
		limit = 50
	}
	for _, row := range res.Rows[:limit] {
		vals := make([]string, len(row))
		for i, v := range row {
			vals[i] = toString(v)
		}
		sb.WriteString(strings.Join(vals, " | "))
		sb.WriteByte('\n')
	}
	if len(res.Rows) > 50 {
		sb.WriteString(fmt.Sprintf("... (%d rows total)\n", len(res.Rows)))
	}
	return sb.String()
}

// GetAISuggestion calls the configured AI provider and returns an inline SQL
// completion for the given prefix text. Returns an empty string when AI is
// disabled, when no API key is set (non-Ollama), or when the provider returns an error.
func (a *App) GetAISuggestion(prefix string) string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	if !cfg.AI.Enabled || (cfg.AI.Provider != "ollama" && cfg.AI.APIKey == "") {
		return ""
	}

	prompt := "Complete this Snowflake SQL query. Return ONLY the completion text to insert at the cursor — no explanation, no markdown, no repetition of existing text. Keep it to 1–2 lines.\n\n" + prefix

	suggestion, err := ai.GetSuggestion(cfg.AI.Provider, cfg.AI.APIKey, cfg.AI.Model, prompt, cfg.AI.OllamaPort)
	if err != nil {
		logger.L.Debug("AI suggestion failed", "provider", cfg.AI.Provider, "err", err)
		return ""
	}
	return suggestion
}

// SuggestImportOptions calls the configured AI provider with the given file
// sample content and returns a JSON string containing suggested Snowflake
// COPY INTO format options. format should be "CSV" or "JSON".
// Returns an error when AI is not configured or the provider call fails.
func (a *App) SuggestImportOptions(format, sampleContent string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.AI.Enabled || (cfg.AI.Provider != "ollama" && cfg.AI.APIKey == "") {
		return "", fmt.Errorf("AI is not configured — enable it in Settings → AI")
	}

	result, err := ai.SuggestFormatOptions(cfg.AI.Provider, cfg.AI.APIKey, cfg.AI.Model, format, sampleContent, cfg.AI.OllamaPort)
	if err != nil {
		logger.L.Debug("AI format suggestion failed", "provider", cfg.AI.Provider, "err", err)
		return "", fmt.Errorf("AI suggestion failed: %w", err)
	}
	return result, nil
}

// ─── Function metadata (autocomplete + hover) ────────────────────────────────

// GetFunctionSuggestions returns up to 50 Snowflake functions whose name
// starts with prefix (case-insensitive). It reads the local SQLite cache so
// results are available instantly, even before a connection is established.
func (a *App) GetFunctionSuggestions(prefix string) ([]fnmeta.FunctionMeta, error) {
	if a.fnStore == nil {
		return nil, nil
	}
	return a.fnStore.Search(strings.ToUpper(prefix))
}

// GetAllFunctionNames returns every distinct function name and type in the
// local SQLite cache. Used by the editor to build its decoration/highlight set.
func (a *App) GetAllFunctionNames() ([]fnmeta.FunctionMeta, error) {
	if a.fnStore == nil {
		return nil, nil
	}
	return a.fnStore.GetAllNames()
}

// GetFunctionTooltip returns all overloads for the given Snowflake function
// name. The name is matched case-insensitively via an exact lookup in the
// local SQLite cache.
func (a *App) GetFunctionTooltip(name string) ([]fnmeta.FunctionMeta, error) {
	if a.fnStore == nil {
		return nil, nil
	}
	return a.fnStore.Lookup(strings.ToUpper(name))
}

// ─── Embedded terminal ────────────────────────────────────────────────────────

// GetAvailableShells reads /etc/shells and returns the list of valid shells.
// Lines starting with '#' are skipped, as are paths that do not exist on disk.
// Falls back to ["/bin/zsh", "/bin/bash", "/bin/sh"] when the file cannot be read.
func (a *App) GetAvailableShells() []string {
	f, err := os.Open("/etc/shells")
	if err != nil {
		return []string{"/bin/zsh", "/bin/bash", "/bin/sh"}
	}
	defer f.Close() //nolint:errcheck

	var shells []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, err := os.Stat(line); err == nil {
			shells = append(shells, line)
		}
	}
	if len(shells) == 0 {
		return []string{"/bin/zsh", "/bin/bash", "/bin/sh"}
	}
	return shells
}

// StartShell launches the given shell in a pseudo-terminal.
// If a shell is already running it is stopped first.
// dir sets the working directory; when empty the shell inherits the process cwd.
// Output from the shell is emitted as base64-encoded "terminal:data" events;
// process exit is signaled by a "terminal:exit" event.
func (a *App) StartShell(shell, dir string) error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()

	// Stop any previously running shell (already locked, so call internals directly).
	if a.ptmx != nil {
		a.ptmx.Close()  //nolint:errcheck
		if a.ptyCmd != nil && a.ptyCmd.Process != nil {
			a.ptyCmd.Process.Kill() //nolint:errcheck
		}
		a.ptmx = nil
		a.ptyCmd = nil
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if dir != "" {
		cmd.Dir = dir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	a.ptmx = ptmx
	a.ptyCmd = cmd

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				encoded := base64.StdEncoding.EncodeToString(buf[:n])
				wailsruntime.EventsEmit(a.ctx, "terminal:data", encoded)
			}
			if err != nil {
				// EOF or closed — shell exited.
				a.ptyMu.Lock()
				a.ptmx = nil
				a.ptyCmd = nil
				a.ptyMu.Unlock()
				wailsruntime.EventsEmit(a.ctx, "terminal:exit")
				return
			}
		}
	}()

	return nil
}

// WriteShell sends data (keystrokes) to the running shell's stdin.
func (a *App) WriteShell(data string) error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()
	if a.ptmx == nil {
		return nil
	}
	_, err := a.ptmx.Write([]byte(data))
	return err
}

// ResizeShell updates the terminal window size of the running pseudo-terminal.
func (a *App) ResizeShell(cols, rows int) error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()
	if a.ptmx == nil {
		return nil
	}
	return pty.Setsize(a.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// StopShell kills the running shell and closes the pseudo-terminal.
// It is a no-op when no shell is running.
func (a *App) StopShell() error {
	a.ptyMu.Lock()
	defer a.ptyMu.Unlock()
	if a.ptmx == nil {
		return nil
	}
	a.ptmx.Close() //nolint:errcheck
	if a.ptyCmd != nil && a.ptyCmd.Process != nil {
		a.ptyCmd.Process.Kill() //nolint:errcheck
	}
	a.ptmx = nil
	a.ptyCmd = nil
	return nil
}

// GetQueryHistory queries Snowflake's INFORMATION_SCHEMA.QUERY_HISTORY* table
// functions and returns a slice of QueryHistoryRow ordered by start time desc.
//
// filterType:          "session" | "user" | "warehouse" | "all"
// sessionID:           non-empty → SESSION_ID => <id>  (used when filterType="session")
// userName:            non-empty → USER_NAME => '<name>'
// warehouseName:       non-empty → WAREHOUSE_NAME => '<name>'
// endTimeStart/End:    RFC3339 strings or "" for no filter
// resultLimit:         max rows returned (1–10 000)
// includeClientGenerated: include client-generated statements
// CanViewWarehouseMeteringHistory returns true when the current role has SELECT
// access to SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY.  It runs a
// zero-row probe query so it is fast and never touches real data.
func (a *App) CanViewWarehouseMeteringHistory() (bool, error) {
	if a.client == nil {
		return false, ErrNotConnected
	}
	_, err := a.client.QuerySingle(a.ctx,
		"SELECT 1 FROM SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY LIMIT 0")
	if err != nil {
		return false, nil //nolint:nilerr // permission denied is not a caller error
	}
	return true, nil
}

// GetWarehouseMeteringHistory returns hourly credit usage records from
// SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY. Rows are ordered by
// START_TIME ascending. warehouse, startDate, and endDate are all optional
// filters; dates must be RFC3339 strings when provided.
func (a *App) GetWarehouseMeteringHistory(warehouse, startDate, endDate string) ([]WarehouseMeteringRow, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	var conds []string
	if warehouse != "" {
		conds = append(conds, fmt.Sprintf("WAREHOUSE_NAME = '%s'", strings.ReplaceAll(warehouse, "'", "''")))
	}
	if startDate != "" {
		conds = append(conds, fmt.Sprintf("START_TIME >= '%s'::TIMESTAMP_LTZ", startDate))
	}
	if endDate != "" {
		conds = append(conds, fmt.Sprintf("START_TIME < '%s'::TIMESTAMP_LTZ", endDate))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	query := fmt.Sprintf(`
SELECT START_TIME, END_TIME, WAREHOUSE_NAME,
       CREDITS_USED, CREDITS_USED_COMPUTE, CREDITS_USED_CLOUD_SERVICES
FROM SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY
%s
ORDER BY START_TIME ASC`, where)

	res, err := a.client.QuerySingle(a.ctx, query)
	if err != nil {
		return nil, err
	}

	startIdx := colIdx(res.Columns, "start_time")
	endIdx   := colIdx(res.Columns, "end_time")
	nameIdx  := colIdx(res.Columns, "warehouse_name")
	usedIdx  := colIdx(res.Columns, "credits_used")
	compIdx  := colIdx(res.Columns, "credits_used_compute")
	cloudIdx := colIdx(res.Columns, "credits_used_cloud_services")

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprint(v)
		}
	}
	toFloat := func(v interface{}) float64 {
		if v == nil {
			return 0
		}
		switch t := v.(type) {
		case float64:
			return t
		case float32:
			return float64(t)
		case []byte:
			f, _ := strconv.ParseFloat(string(t), 64)
			return f
		case string:
			f, _ := strconv.ParseFloat(t, 64)
			return f
		}
		return 0
	}

	rows := make([]WarehouseMeteringRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, WarehouseMeteringRow{
			StartTime:                toString(row[startIdx]),
			EndTime:                  toString(row[endIdx]),
			WarehouseName:            toString(row[nameIdx]),
			CreditsUsed:              toFloat(row[usedIdx]),
			CreditsUsedCompute:       toFloat(row[compIdx]),
			CreditsUsedCloudServices: toFloat(row[cloudIdx]),
		})
	}
	return rows, nil
}

// GetQueryHistory queries SNOWFLAKE.INFORMATION_SCHEMA.QUERY_HISTORY* table
// functions and returns a slice of QueryHistoryRow ordered by start time desc.
//
//   - filterType:             "session" | "user" | "warehouse" | "all"
//   - sessionID:              non-empty → SESSION_ID => <id> (filterType="session")
//   - userName:               non-empty → USER_NAME => '<name>'
//   - warehouseName:          non-empty → WAREHOUSE_NAME => '<name>'
//   - endTimeStart/End:       RFC3339 strings or "" for no filter
//   - resultLimit:            max rows returned (1–10 000)
//   - includeClientGenerated: include client-generated statements
func (a *App) GetQueryHistory(
	filterType string,
	sessionID string,
	userName string,
	warehouseName string,
	endTimeStart string,
	endTimeEnd string,
	resultLimit int,
	includeClientGenerated bool,
) ([]QueryHistoryRow, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}

	// Choose the table function name.
	var funcName string
	switch filterType {
	case "session":
		funcName = "QUERY_HISTORY_BY_SESSION"
	case "user":
		funcName = "QUERY_HISTORY_BY_USER"
	case "warehouse":
		funcName = "QUERY_HISTORY_BY_WAREHOUSE"
	default:
		funcName = "QUERY_HISTORY"
	}

	// Build the named-argument list.
	var args []string
	switch filterType {
	case "session":
		if sessionID != "" {
			args = append(args, fmt.Sprintf("SESSION_ID => %s", sessionID))
		}
	case "user":
		if userName != "" {
			args = append(args, fmt.Sprintf("USER_NAME => '%s'", strings.ReplaceAll(userName, "'", "''")))
		}
	case "warehouse":
		if warehouseName != "" {
			args = append(args, fmt.Sprintf("WAREHOUSE_NAME => '%s'", strings.ReplaceAll(warehouseName, "'", "''")))
		}
	}
	if endTimeStart != "" {
		args = append(args, fmt.Sprintf("END_TIME_RANGE_START => '%s'::TIMESTAMP_LTZ", endTimeStart))
	}
	if endTimeEnd != "" {
		args = append(args, fmt.Sprintf("END_TIME_RANGE_END => '%s'::TIMESTAMP_LTZ", endTimeEnd))
	}
	if resultLimit > 0 {
		args = append(args, fmt.Sprintf("RESULT_LIMIT => %d", resultLimit))
	}
	if includeClientGenerated {
		args = append(args, "INCLUDE_CLIENT_GENERATED_STATEMENT => TRUE")
	}

	var argClause string
	if len(args) > 0 {
		argClause = strings.Join(args, ", ")
	}

	query := fmt.Sprintf(`
SELECT QUERY_ID, QUERY_TEXT, QUERY_TYPE, USER_NAME, WAREHOUSE_NAME,
       DATABASE_NAME, SCHEMA_NAME, START_TIME, END_TIME,
       TOTAL_ELAPSED_TIME, EXECUTION_STATUS, ERROR_MESSAGE,
       ROWS_PRODUCED, BYTES_SCANNED
FROM table(SNOWFLAKE.information_schema.%s(%s))
ORDER BY START_TIME DESC`, funcName, argClause)

	res, err := a.client.QuerySingle(a.ctx, query)
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprint(v)
		}
	}

	toInt64 := func(v interface{}) int64 {
		s := toString(v)
		if s == "" {
			return 0
		}
		// Handle potential float strings like "1234.00"
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
		return 0
	}

	qidIdx  := colIdx(res.Columns, "query_id")
	qtxtIdx := colIdx(res.Columns, "query_text")
	qtypIdx := colIdx(res.Columns, "query_type")
	userIdx := colIdx(res.Columns, "user_name")
	whIdx   := colIdx(res.Columns, "warehouse_name")
	dbIdx   := colIdx(res.Columns, "database_name")
	schIdx  := colIdx(res.Columns, "schema_name")
	stIdx   := colIdx(res.Columns, "start_time")
	etIdx   := colIdx(res.Columns, "end_time")
	elIdx   := colIdx(res.Columns, "total_elapsed_time")
	statIdx := colIdx(res.Columns, "execution_status")
	errIdx  := colIdx(res.Columns, "error_message")
	rpIdx   := colIdx(res.Columns, "rows_produced")
	bsIdx   := colIdx(res.Columns, "bytes_scanned")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	rows := make([]QueryHistoryRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, QueryHistoryRow{
			QueryID:       toString(get(row, qidIdx)),
			QueryText:     toString(get(row, qtxtIdx)),
			QueryType:     toString(get(row, qtypIdx)),
			UserName:      toString(get(row, userIdx)),
			WarehouseName: toString(get(row, whIdx)),
			DatabaseName:  toString(get(row, dbIdx)),
			SchemaName:    toString(get(row, schIdx)),
			StartTime:     toString(get(row, stIdx)),
			EndTime:       toString(get(row, etIdx)),
			ElapsedMs:     toInt64(get(row, elIdx)),
			Status:        toString(get(row, statIdx)),
			ErrorMessage:  toString(get(row, errIdx)),
			RowsProduced:  toInt64(get(row, rpIdx)),
			BytesScanned:  toInt64(get(row, bsIdx)),
		})
	}
	return rows, nil
}

// ─── Backup Sets ──────────────────────────────────────────────────────────────

// ListBackupSets returns backup sets scoped to a database, schema, or table.
// bsFQN builds a (possibly fully-qualified) backup set identifier.
// q must be the double-quote identifier-quoting function used at the call site.
func bsFQN(q func(string) string, name, bsDb, bsSchema string) string {
	if bsDb != "" && bsSchema != "" {
		return q(bsDb) + "." + q(bsSchema) + "." + q(name)
	}
	if bsDb != "" {
		return q(bsDb) + "." + q(name)
	}
	return q(name)
}

// ListBackupSets returns backup sets whose backed-up object matches the right-clicked item.
// It uses SHOW BACKUP SETS IN DATABASE <db> and post-filters by object_kind / object_name /
// object_database_name / object_schema_name so that only backup sets actually covering the
// specified database, schema, or table are returned — not all backup sets stored there.
func (a *App) ListBackupSets(scopeType, db, schema, table string) ([]BackupSetRow, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	// Always query at the database level; the object columns tell us what is backed up.
	query := fmt.Sprintf("SHOW BACKUP SETS IN DATABASE %s", q(db))
	res, err := a.client.Execute(a.ctx, query)
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	nameIdx    := colIdx(res.Columns, "name")
	bsDbIdx    := colIdx(res.Columns, "database_name")
	bsSchIdx   := colIdx(res.Columns, "schema_name")
	createdIdx := colIdx(res.Columns, "created_on")
	otypeIdx   := colIdx(res.Columns, "object_kind")
	onameIdx   := colIdx(res.Columns, "object_name")
	objDbIdx   := colIdx(res.Columns, "object_database_name")
	objSchIdx  := colIdx(res.Columns, "object_schema_name")
	statusIdx  := colIdx(res.Columns, "backup_policy_state", "status")
	commentIdx := colIdx(res.Columns, "comment")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	upperScope := strings.ToUpper(scopeType)
	rows := make([]BackupSetRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		otype  := strings.ToUpper(toString(get(row, otypeIdx)))
		oname  := toString(get(row, onameIdx))
		objDb  := toString(get(row, objDbIdx))
		objSch := toString(get(row, objSchIdx))

		// Post-filter: only include backup sets whose backed-up object matches
		// the right-clicked item.
		var match bool
		switch upperScope {
		case "DATABASE":
			match = otype == "DATABASE" && strings.EqualFold(oname, db)
		case "SCHEMA":
			match = otype == "SCHEMA" &&
				strings.EqualFold(objDb, db) &&
				strings.EqualFold(oname, schema)
		case "TABLE":
			match = (otype == "TABLE" || otype == "EXTERNAL TABLE") &&
				strings.EqualFold(objDb, db) &&
				strings.EqualFold(objSch, schema) &&
				strings.EqualFold(oname, table)
		default:
			return nil, fmt.Errorf("unsupported scope: %s", scopeType)
		}
		if !match {
			continue
		}

		rowBsDb  := toString(get(row, bsDbIdx))
		rowBsSch := toString(get(row, bsSchIdx))
		if rowBsDb == "" {
			rowBsDb = db
		}
		rows = append(rows, BackupSetRow{
			Name:            toString(get(row, nameIdx)),
			BackupSetDb:     rowBsDb,
			BackupSetSchema: rowBsSch,
			CreatedOn:       toString(get(row, createdIdx)),
			ObjectType:      otype,
			ObjectName:      oname,
			ObjectDb:        objDb,
			ObjectSchema:    objSch,
			Status:          toString(get(row, statusIdx)),
			Comment:         toString(get(row, commentIdx)),
		})
	}
	return rows, nil
}

// CreateBackupSet creates a new backup set for a DATABASE, SCHEMA, or TABLE.
// forType must be "DATABASE", "SCHEMA", or "TABLE".
// nameDb and nameSchema locate the backup set object itself (its fully-qualified name).
// db is the database name used to set the session context before the CREATE.
// objectFQN is the fully-qualified target object, e.g. "MY_DB" or "MY_DB"."MY_SCHEMA"."MY_TABLE".
func (a *App) CreateBackupSet(name, nameDb, nameSchema, forType, objectFQN, db string, orReplace, ifNotExists bool) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	// Snowflake requires a current database to be set for CREATE BACKUP SET,
	// even when the object name is fully qualified.
	if db != "" {
		if _, err := a.client.Execute(a.ctx, fmt.Sprintf("USE DATABASE %s", q(db))); err != nil {
			return err
		}
	}

	// Build the (optionally fully-qualified) backup set name.
	var nameFQN string
	switch {
	case nameDb != "" && nameSchema != "":
		nameFQN = q(nameDb) + "." + q(nameSchema) + "." + q(name)
	case nameDb != "":
		nameFQN = q(nameDb) + "." + q(name)
	default:
		nameFQN = q(name)
	}

	var sb strings.Builder
	sb.WriteString("CREATE ")
	if orReplace {
		sb.WriteString("OR REPLACE ")
	}
	sb.WriteString("BACKUP SET ")
	if ifNotExists && !orReplace {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(nameFQN)
	sb.WriteString(" FOR ")
	sb.WriteString(strings.ToUpper(forType))
	sb.WriteString(" ")
	sb.WriteString(objectFQN)

	_, err := a.client.Execute(a.ctx, sb.String())
	return err
}

// DropBackupSet drops the named backup set.
func (a *App) DropBackupSet(name, bsDb, bsSchema string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	fqn := bsFQN(q, name, bsDb, bsSchema)
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("DROP BACKUP SET %s", fqn))
	return err
}

// AlterBackupSet executes ALTER BACKUP SET <fqn> <alteration>.
// alteration is the full action fragment, e.g. "RENAME TO new_name",
// "SET COMMENT = 'text'", "UNSET COMMENT", "SUSPEND BACKUP POLICY", etc.
func (a *App) AlterBackupSet(name, bsDb, bsSchema, alteration string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	fqn := bsFQN(q, name, bsDb, bsSchema)
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER BACKUP SET %s %s", fqn, alteration))
	return err
}

// ─── Backup Policies ──────────────────────────────────────────────────────────

// ListBackupPolicies runs SHOW BACKUP POLICIES and returns all visible policies.
func (a *App) ListBackupPolicies() ([]BackupPolicyRow, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	res, err := a.client.Execute(a.ctx, "SHOW BACKUP POLICIES")
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	toBool := func(v interface{}) bool {
		s := strings.ToUpper(toString(v))
		return s == "TRUE" || s == "YES" || s == "1"
	}

	nameIdx    := colIdx(res.Columns, "name")
	createdIdx := colIdx(res.Columns, "created_on")
	ownerIdx   := colIdx(res.Columns, "owner")
	schedIdx   := colIdx(res.Columns, "schedule")
	expireIdx  := colIdx(res.Columns, "expire_after_days")
	lockIdx    := colIdx(res.Columns, "retention_lock", "with_retention_lock")
	commentIdx := colIdx(res.Columns, "comment")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	toInt64 := func(v interface{}) int64 {
		s := toString(v)
		if s == "" {
			return 0
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		return 0
	}

	rows := make([]BackupPolicyRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, BackupPolicyRow{
			Name:            toString(get(row, nameIdx)),
			CreatedOn:       toString(get(row, createdIdx)),
			Owner:           toString(get(row, ownerIdx)),
			Schedule:        toString(get(row, schedIdx)),
			ExpireAfterDays: toInt64(get(row, expireIdx)),
			RetentionLock:   toBool(get(row, lockIdx)),
			Comment:         toString(get(row, commentIdx)),
		})
	}
	return rows, nil
}

// CreateBackupPolicy creates a new backup policy.
// schedule: optional, e.g. "60 MINUTE", "6 HOUR", "USING CRON 0 2 * * * UTC"
// expireAfterDays: 0 means not set
// tags: optional raw tag expression e.g. `"MY_TAG" = 'value'`
func (a *App) CreateBackupPolicy(name, schedule string, expireAfterDays int64, retentionLock bool, comment, tags string, orReplace, ifNotExists bool) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	esc := func(s string) string { return strings.ReplaceAll(s, "'", "''") }

	var sb strings.Builder
	sb.WriteString("CREATE ")
	if orReplace {
		sb.WriteString("OR REPLACE ")
	}
	sb.WriteString("BACKUP POLICY ")
	if ifNotExists && !orReplace {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(q(name))
	if tags != "" {
		sb.WriteString(fmt.Sprintf(" WITH TAG (%s)", tags))
	}
	if retentionLock {
		sb.WriteString(" WITH RETENTION LOCK")
	}
	if schedule != "" {
		sb.WriteString(fmt.Sprintf(" SCHEDULE = '%s'", esc(schedule)))
	}
	if expireAfterDays > 0 {
		sb.WriteString(fmt.Sprintf(" EXPIRE_AFTER_DAYS = %d", expireAfterDays))
	}
	if comment != "" {
		sb.WriteString(fmt.Sprintf(" COMMENT = '%s'", esc(comment)))
	}

	_, err := a.client.Execute(a.ctx, sb.String())
	return err
}

// DropBackupPolicy drops the named backup policy.
func (a *App) DropBackupPolicy(name string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("DROP BACKUP POLICY %s", q(name)))
	return err
}

// AlterBackupPolicy executes ALTER BACKUP POLICY <name> <alteration>.
// alteration is the full action fragment, e.g. "RENAME TO new_name",
// "SET SCHEDULE = '60 MINUTE'", "SET COMMENT = 'text'", "UNSET COMMENT", etc.
func (a *App) AlterBackupPolicy(name, alteration string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER BACKUP POLICY %s %s", q(name), alteration))
	return err
}

// ─── Backups (snapshots inside a backup set) ──────────────────────────────────

// ListBackups runs SHOW BACKUPS IN BACKUP SET <name> and returns the result.
// db must be non-empty; it is used to set a current database context first.
func (a *App) ListBackups(backupSetName, bsDb, bsSchema string) ([]BackupRow, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

	fqn := bsFQN(q, backupSetName, bsDb, bsSchema)
	res, err := a.client.Execute(a.ctx, fmt.Sprintf("SHOW BACKUPS IN BACKUP SET %s", fqn))
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	toInt64 := func(v interface{}) int64 {
		s := toString(v)
		if s == "" {
			return 0
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
		return 0
	}

	// Snowflake internally uses "snapshot" terminology; column names vary by version.
	idIdx      := colIdx(res.Columns, "backup_id", "snapshot_id", "id", "identifier", "uuid")
	nameIdx    := colIdx(res.Columns, "name", "backup_name", "snapshot_name", "backup", "snapshot")
	createdIdx := colIdx(res.Columns, "created_on")
	statusIdx  := colIdx(res.Columns, "status")
	sizeIdx    := colIdx(res.Columns, "size_bytes", "size")
	commentIdx := colIdx(res.Columns, "comment")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	rows := make([]BackupRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		idVal   := toString(get(row, idIdx))
		nameVal := toString(get(row, nameIdx))
		// If no dedicated name column was found, fall back to created_on — Snowflake
		// uses the creation timestamp as the backup identifier in DROP BACKUP.
		if nameVal == "" {
			nameVal = toString(get(row, createdIdx))
		}
		rows = append(rows, BackupRow{
			ID:        idVal,
			Name:      nameVal,
			CreatedOn: toString(get(row, createdIdx)),
			Status:    toString(get(row, statusIdx)),
			SizeBytes: toInt64(get(row, sizeIdx)),
			Comment:   toString(get(row, commentIdx)),
		})
	}
	return rows, nil
}

// AddBackup triggers ALTER BACKUP SET <fqn> ADD BACKUP to create a new backup snapshot.
func (a *App) AddBackup(backupSetName, bsDb, bsSchema string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	fqn := bsFQN(q, backupSetName, bsDb, bsSchema)
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER BACKUP SET %s ADD BACKUP", fqn))
	return err
}

// RestoreFromBackup executes RESTORE [OR REPLACE] <objectType> <targetName> FROM BACKUP <backupName>.
// db must be non-empty; it is used to set a current database context first.
// targetName is the fully-qualified target object name (may differ from the original to restore into a new object).
// RestoreFromBackup executes:
//
//	CREATE <objectType> <targetName>
//	  FROM BACKUP SET <backupSetName>
//	  IDENTIFIER '<backupID>'
//
// Snowflake does not support OR REPLACE for this form — the target must be a new name.
// db must be non-empty; it is used to set a current database context first.
// targetName is used as-is (caller provides the identifier, quoted or unquoted).
// backupID is the UUID returned by SHOW BACKUPS (stored as a single-quoted string literal).
func (a *App) RestoreFromBackup(objectType, targetName, backupSetName, backupID, db string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	objType := strings.ToUpper(strings.TrimSpace(objectType))
	if objType == "" {
		return fmt.Errorf("object type must be DATABASE, SCHEMA, or TABLE")
	}
	if targetName == "" {
		return fmt.Errorf("target name must not be empty")
	}
	if backupSetName == "" {
		return fmt.Errorf("backup set name must not be empty")
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	if db != "" {
		if _, err := a.client.Execute(a.ctx, fmt.Sprintf("USE DATABASE %s", q(db))); err != nil {
			return err
		}
	}
	var sb strings.Builder
	sb.WriteString("CREATE ")
	sb.WriteString(objType)
	sb.WriteString(" ")
	sb.WriteString(targetName)
	sb.WriteString(" FROM BACKUP SET ")
	sb.WriteString(q(backupSetName))
	sb.WriteString(" IDENTIFIER '")
	sb.WriteString(strings.ReplaceAll(backupID, "'", "''"))
	sb.WriteString("'")
	// Must use QuerySingle (plain db.QueryContext) — multi-statement mode breaks
	// the FROM BACKUP SET ... IDENTIFIER syntax just like TABLE() function calls.
	_, err := a.client.QuerySingle(a.ctx, sb.String())
	return err
}

// DeleteOldestBackup finds the oldest backup in the set that has no legal hold
// and deletes it using ALTER BACKUP SET … DELETE BACKUP IDENTIFIER '<id>'.
// Snowflake only permits deleting the single oldest eligible backup at a time.
func (a *App) DeleteOldestBackup(backupSetName, bsDb, bsSchema string) error {
	if a.client == nil {
		return ErrNotConnected
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	fqn := bsFQN(q, backupSetName, bsDb, bsSchema)

	res, err := a.client.Execute(a.ctx, fmt.Sprintf("SHOW BACKUPS IN BACKUP SET %s", fqn))
	if err != nil {
		return err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	idIdx        := colIdx(res.Columns, "backup_id", "snapshot_id", "id", "identifier", "uuid")
	createdIdx   := colIdx(res.Columns, "created_on")
	legalHoldIdx := colIdx(res.Columns, "is_under_legal_hold", "legal_hold", "under_legal_hold")

	type candidate struct {
		id        string
		createdOn string
	}
	var best *candidate

	for _, row := range res.Rows {
		lh := strings.ToUpper(strings.TrimSpace(toString(get(row, legalHoldIdx))))
		if lh == "Y" || lh == "TRUE" || lh == "YES" || lh == "1" {
			continue
		}
		id := toString(get(row, idIdx))
		if id == "" {
			continue
		}
		created := toString(get(row, createdIdx))
		if best == nil || created < best.createdOn {
			best = &candidate{id: id, createdOn: created}
		}
	}

	if best == nil {
		return fmt.Errorf("no eligible backup found (all backups may be under legal hold)")
	}

	escapedID := strings.ReplaceAll(best.id, "'", "''")
	_, err = a.client.Execute(a.ctx, fmt.Sprintf(
		"ALTER BACKUP SET %s DELETE BACKUP IDENTIFIER '%s'",
		fqn, escapedID,
	))
	return err
}
