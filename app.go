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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	sf "github.com/snowflakedb/gosnowflake"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/ai"
	"thaw/internal/config"
	"thaw/internal/ddl"
	"thaw/internal/filesystem"
	"thaw/internal/gitrepo"
	"thaw/internal/logger"
	"thaw/internal/sfconfig"
	"thaw/internal/snowflake"
	"thaw/internal/telemetry"
)

// App is the main application struct. Methods bound here are callable from the frontend.
type App struct {
	ctx           context.Context
	client        *snowflake.Client
	cancelConnect    context.CancelFunc
	exportCancelFunc context.CancelFunc // cancels an in-flight DDL export
	cancelChat       context.CancelFunc // cancels an in-flight AI chat request
	logCleanup       func()             // closes the log rotation file on shutdown

	// Two-phase query execution (StartQuery / WaitForQueryResult).
	queryMu         sync.Mutex
	queryID         string
	queryDone       chan struct{}
	queryResult     *snowflake.QueryResult
	queryErr        error
	queryCancelFunc context.CancelFunc // cancels the in-flight query context

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
// It stores the application context, initialises logging and telemetry.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.logCleanup = logger.Init()
	telemetry.Init(Version)
	logger.L.Info("application started")
	telemetry.Track(telemetry.EventAppStarted, nil)
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
			logger.L.Info("connection cancelled by user")
			return fmt.Errorf("connection cancelled")
		}
		logger.L.Error("connection failed", "account", params.Account, "err", err)
		telemetry.Track(telemetry.EventConnectionFailed, nil)
		return err
	}
	a.client = client
	logger.L.Info("connected", "account", params.Account, "user", params.User)
	telemetry.Track(telemetry.EventConnected, telemetry.Props{"authenticator": params.Authenticator})
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

// LoadSnowflakeCLIConfig reads ~/.snowflake/config.toml and returns all
// named connection profiles together with the configured default.
// Returns an empty config (not an error) when the file does not exist.
func (a *App) LoadSnowflakeCLIConfig() (sfconfig.Config, error) {
	cfg, err := sfconfig.Load()
	if err != nil {
		return sfconfig.Config{}, err
	}
	return *cfg, nil
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

// PickOpenFile opens a native open-file dialog filtered to SQL files and
// returns the chosen path, or an empty string if the user cancels.
func (a *App) PickOpenFile() string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open SQL file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL Files (*.sql)", Pattern: "*.sql"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickDataFile opens a native open-file dialog filtered to common data file
// formats (CSV, JSON, Parquet) and returns the chosen path, or an empty string
// if the user cancels.
func (a *App) PickDataFile() string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open data file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Data Files (*.csv;*.json;*.jsonl;*.ndjson;*.parquet)", Pattern: "*.csv;*.json;*.jsonl;*.ndjson;*.parquet"},
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "JSON Files (*.json;*.jsonl;*.ndjson)", Pattern: "*.json;*.jsonl;*.ndjson"},
			{DisplayName: "Parquet Files (*.parquet)", Pattern: "*.parquet"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickDataFileByFormat opens a native open-file dialog filtered to the file
// extensions that match the given format ("CSV", "JSON", or "PARQUET").
func (a *App) PickDataFileByFormat(format string) string {
	var filters []wailsruntime.FileFilter
	switch strings.ToUpper(format) {
	case "JSON":
		filters = []wailsruntime.FileFilter{
			{DisplayName: "JSON Files (*.json;*.jsonl;*.ndjson)", Pattern: "*.json;*.jsonl;*.ndjson"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "PARQUET":
		filters = []wailsruntime.FileFilter{
			{DisplayName: "Parquet Files (*.parquet)", Pattern: "*.parquet"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	default: // CSV
		filters = []wailsruntime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Open " + format + " file",
		Filters: filters,
	})
	if err != nil {
		return ""
	}
	return path
}

// PickSaveFile opens a native save-file dialog pre-populated with defaultName
// and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickSaveFile(defaultName string) string {
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save SQL file",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL Files (*.sql)", Pattern: "*.sql"},
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
	a.queryDone = nil // clear stale channel from previous query
	a.queryID = ""
	a.queryMu.Unlock()

	qidChan := make(chan string, 1)
	ctx = sf.WithQueryIDChan(ctx, qidChan)
	ctx = sf.WithAsyncMode(ctx) // ask Snowflake to return query ID immediately, before results are ready
	done := make(chan struct{})

	// Execute the query in a background goroutine so this method can return
	// as soon as the query ID arrives (before results are ready).
	go func() {
		result, err := a.client.Execute(ctx, sql)
		a.queryMu.Lock()
		a.queryResult = result
		a.queryErr = err
		a.queryMu.Unlock()
		close(done)
	}()

	// Block until the driver assigns a query ID (arrives with the first HTTP
	// response), the background goroutine finishes (fast query), or the query
	// is cancelled.
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
	a.queryID = queryID
	a.queryDone = done
	a.queryMu.Unlock()

	logger.L.Info("query started", "queryID", queryID)
	telemetry.Track(telemetry.EventQueryStarted, nil)
	return queryID, nil
}

// CancelQuery cancels the query currently in flight (started by StartQuery).
// It is a no-op if no query is running. In addition to cancelling the local
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
		logger.L.Info("cancelling query", "queryID", queryID)
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
func (a *App) WaitForQueryResult() (*snowflake.QueryResult, error) {
	a.queryMu.Lock()
	done := a.queryDone
	queryID := a.queryID
	a.queryMu.Unlock()

	if done == nil {
		return nil, fmt.Errorf("no query in progress")
	}
	<-done

	a.queryMu.Lock()
	result := a.queryResult
	err := a.queryErr
	// Clean up so a subsequent call does not re-read stale state.
	if a.queryCancelFunc != nil {
		a.queryCancelFunc() // no-op if already cancelled; ensures context resources are freed
		a.queryCancelFunc = nil
	}
	a.queryDone = nil
	a.queryID = ""
	a.queryMu.Unlock()

	if result != nil && queryID != "" {
		result.QueryID = queryID
	}
	if err != nil {
		logger.L.Error("query failed", "queryID", queryID, "err", err)
		telemetry.Track(telemetry.EventQueryFailed, nil)
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

// ListNotificationIntegrations returns the names of all notification integrations.
func (a *App) ListNotificationIntegrations() ([]string, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.ListNotificationIntegrations(a.ctx)
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
	like := strings.ReplaceAll(name, "'", "''")

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
		fmt.Sscanf(s, "%d", &n)
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

// ExportAllDatabasesDDL lists every database visible to the current role and
// exports DDL for all of them in parallel.
//
// Progress events ("ddl:progress") are emitted after each database completes,
// allowing the frontend to show a live progress bar.
func (a *App) ExportAllDatabasesDDL(outputDir string) ([]ddl.ExportResult, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}

	databases, err := a.client.ListExportableDatabases(a.ctx)
	if err != nil {
		return nil, err
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

// ListAIModels returns the models available for the given provider and API key.
// Returns nil (not an error) when the key is invalid or the request fails so
// the frontend can fall back to its static defaults.
func (a *App) ListAIModels(provider, apiKey string) []string {
	models, err := ai.ListModels(provider, apiKey)
	if err != nil {
		logger.L.Warn("failed to list AI models", "provider", provider, "err", err)
		return nil
	}
	return models
}

// TestAIModel makes a minimal one-token API call to verify that the given
// provider/key/model combination is valid and reachable.
// Returns an empty string on success or a human-readable error message.
func (a *App) TestAIModel(provider, apiKey, model string) string {
	if err := ai.TestModel(provider, apiKey, model); err != nil {
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

	msg, err := ai.Chat(chatCtx, cfg.AI.Provider, cfg.AI.APIKey, cfg.AI.Model,
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
// disabled, when no API key is set, or when the provider returns an error.
func (a *App) GetAISuggestion(prefix string) string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	if !cfg.AI.Enabled || cfg.AI.APIKey == "" {
		return ""
	}

	prompt := "Complete this Snowflake SQL query. Return ONLY the completion text to insert at the cursor — no explanation, no markdown, no repetition of existing text. Keep it to 1–2 lines.\n\n" + prefix

	suggestion, err := ai.GetSuggestion(cfg.AI.Provider, cfg.AI.APIKey, cfg.AI.Model, prompt)
	if err != nil {
		logger.L.Debug("AI suggestion failed", "provider", cfg.AI.Provider, "err", err)
		return ""
	}
	return suggestion
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
	defer f.Close()

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
// process exit is signalled by a "terminal:exit" event.
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
