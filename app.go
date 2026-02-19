package main

import (
	"context"
	"fmt"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/ddl"
	"thaw/internal/gitrepo"
	"thaw/internal/snowflake"
)

// App is the main application struct. Methods bound here are callable from the frontend.
type App struct {
	ctx           context.Context
	client        *snowflake.Client
	cancelConnect context.CancelFunc
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.client != nil {
		a.client.Close()
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

	client, err := snowflake.NewClient(ctx, params)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("connection cancelled")
		}
		return err
	}
	a.client = client
	return nil
}

// CancelConnect aborts an in-progress Connect call.
func (a *App) CancelConnect() {
	if a.cancelConnect != nil {
		a.cancelConnect()
	}
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
	return err
}

// IsConnected returns true when a Snowflake connection is active.
func (a *App) IsConnected() bool {
	return a.client != nil && a.client.IsAlive()
}

// ExecuteQuery runs a SQL statement and returns the result set.
func (a *App) ExecuteQuery(sql string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}
	return a.client.Execute(a.ctx, sql)
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

	opts := ddl.ExportOptions{OutputDir: outputDir}

	var result ddl.ExportResult
	ddl.ExportDatabases(
		a.ctx,
		[]string{database},
		a.client.GetDatabaseDDL,
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

	databases, err := a.client.ListDatabases(a.ctx)
	if err != nil {
		return nil, err
	}

	opts := ddl.ExportOptions{OutputDir: outputDir}

	results := ddl.ExportDatabases(
		a.ctx,
		databases,
		a.client.GetDatabaseDDL,
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
