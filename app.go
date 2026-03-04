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
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sf "github.com/snowflakedb/gosnowflake"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

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
	logCleanup       func()             // closes the log rotation file on shutdown

	// Two-phase query execution (StartQuery / WaitForQueryResult).
	queryMu         sync.Mutex
	queryID         string
	queryDone       chan struct{}
	queryResult     *snowflake.QueryResult
	queryErr        error
	queryCancelFunc context.CancelFunc // cancels the in-flight query context
}

func NewApp() *App {
	return &App{}
}

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

func (a *App) shutdown(_ context.Context) {
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

	opts := ddl.ExportOptions{OutputDir: outputDir}

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

	opts := ddl.ExportOptions{OutputDir: outputDir}

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
