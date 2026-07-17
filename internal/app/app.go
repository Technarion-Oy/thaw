// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"thaw/internal/apperrors"
	"thaw/internal/config"
	"thaw/internal/filesystem"
	"thaw/internal/fnmeta"
	"thaw/internal/logger"
	"thaw/internal/mcp"
	"thaw/internal/migration"
	"thaw/internal/querylog"
	"thaw/internal/session"
	"thaw/internal/snowflake"
	"thaw/internal/snowpark"
	"thaw/internal/telemetry"
	"thaw/internal/version"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// tabSession holds the per-tab Snowflake client and the two-phase query
// execution state that was previously global on App.
type tabSession struct {
	client             *snowflake.Client
	lastUsed           atomic.Int64 // UnixNano timestamp for LRU eviction
	inUse              atomic.Int32 // incremented during non-query client RPCs to prevent eviction mid-flight
	queryMu            sync.Mutex
	queryID            string
	queryDone          chan struct{}
	queryResult        *snowflake.QueryResult
	queryErr           error
	queryCancelFunc    context.CancelFunc
	queryCancelCtxDone <-chan struct{}
	queryLogEntryID    int       // ID in queryLog for RUNNING → final status updates
	queryLogStart      time.Time // timestamp when the query was submitted, for duration
}

// tabSessionInitMu serializes lazy creation of new tab sessions so that two
// concurrent calls for the same tabId do not both open a connection.
var tabSessionInitMu sync.Mutex

// App is the main application struct. Methods bound here are callable from the frontend.
type App struct {
	ctx context.Context

	// connMu guards the shared connection state — client and connectParams —
	// which Connect/Disconnect write and many IPC methods read. Wails invokes
	// IPC methods on concurrent goroutines, so per the Go memory model these
	// pointer reads/writes must be synchronized. Readers must go through
	// currentClient()/currentConnectParams() rather than touching the fields
	// directly. (a.ctx is deliberately not guarded: it is set once in startup()
	// before any IPC method can run and is never reassigned.)
	connMu        sync.RWMutex
	client        *snowflake.Client
	connectParams *snowflake.ConnectParams // stored after a successful Connect for notebook session init

	cancelConnect    context.CancelFunc
	exportCancelFunc context.CancelFunc   // cancels an in-flight DDL export
	fnStore          *fnmeta.Store        // local SQLite cache for Snowflake function metadata
	logCleanup       func()               // closes the log rotation file on shutdown
	savedWindowState *session.WindowState // non-nil when a persisted window state was loaded at launch

	// oauthCancel cancels the in-flight GitHub OAuth loopback flow (frees port 3456
	// and its goroutine when the user dismisses the auth dialog). Guarded by oauthMu.
	oauthMu     sync.Mutex
	oauthCancel context.CancelFunc

	// Service instances for delegated business logic.
	migrationSvc *migration.Service
	snowparkSvc  *snowpark.Service

	// MCP server manager (multi-session, started/stopped on user action).
	mcpManager *mcp.Manager

	// Per-tab isolated Snowflake sessions.
	tabSessions sync.Map // string (tabId) → *tabSession

	// evictedContexts caches session context (role/wh/db/schema) for tabs whose
	// sessions were evicted by LRU. Restored transparently on next use.
	evictedContexts sync.Map // string (tabId) → snowflake.SessionContext

	// Session management runtime state (configurable via View → Advanced → Session Management…).
	sessionConfigMu    sync.RWMutex
	sessionMaxSessions int
	sessionMaxOpen     int
	sessionMaxIdle     int
	sessionInitMode    string
	sessionIdleTimeout time.Duration
	sessionIdleStopCh  chan struct{}

	// Git repository commit filters (repoKey -> commitHash).
	// repoKey format: "db.schema.repo"
	gitCommitFiltersMu sync.Mutex
	gitCommitFilters   map[string]string

	// Cached working directory — the single source of truth for this process's
	// folder, guarded by exportDirMu (set on startup and by SaveGitConfig).
	exportDirMu     sync.RWMutex
	cachedExportDir string

	// workdirOverridden marks this process as an "Open Folder in New Window"
	// instance: it was launched with --workdir=<dir>, so its folder lives only in
	// cachedExportDir and is never persisted back to the shared config (see
	// GetGitConfig / SaveGitConfig), letting windows operate on different folders
	// without fighting. Set once in NewApp and never mutated, so no lock is needed.
	workdirOverridden bool

	// File system watcher for the working directory.
	fsWatcherMu sync.Mutex
	fsWatcher   *filesystem.Watcher

	// Embedded terminal (pseudo-terminal).
	ptyMu  sync.Mutex
	ptmx   *os.File
	ptyCmd *exec.Cmd

	// Session-scoped query log for debugging and issue reporting.
	queryLog             *querylog.Log
	setQueryLogMenuCheck func(bool) // set by buildMenu; updates the native menu checkbox

	// Effective file-logging preferences (with IT-admin policy applied),
	// consulted by the OnQuery hook to decide whether to write SQL to thaw.log.
	logPrefsMu sync.RWMutex
	logPrefs   config.LogPrefs

	// thirdPartyNotices is the generated copyright-notice / license-text bundle
	// for every third-party package Thaw redistributes (embedded at build time
	// from THIRD_PARTY_NOTICES.md). Served to the About dialog via
	// GetThirdPartyNotices. Set once in NewApp and never mutated.
	thirdPartyNotices string
}

// NewApp creates and returns a new App instance for use with the Wails runtime.
// thirdPartyNotices is the embedded THIRD_PARTY_NOTICES.md content shown in the
// About dialog.
func NewApp(thirdPartyNotices string) *App {
	return &App{
		gitCommitFilters:  make(map[string]string),
		queryLog:          querylog.New(),
		workdirOverridden: workdirOverrideArg() != "",
		thirdPartyNotices: thirdPartyNotices,
	}
}

// currentClient returns the shared Snowflake client under the connection read
// lock (nil when disconnected). IPC readers must call this instead of touching
// a.client directly so they don't race with Connect/Disconnect, which run on
// concurrent Wails IPC goroutines. The lock is held only for the pointer read;
// callers operate on the returned snapshot, so a subsequent Disconnect that
// nils the field cannot turn a live call into a nil-deref.
func (a *App) currentClient() *snowflake.Client {
	a.connMu.RLock()
	defer a.connMu.RUnlock()
	return a.client
}

// currentConnectParams returns the params captured by the last successful
// Connect under the connection read lock (nil when disconnected).
func (a *App) currentConnectParams() *snowflake.ConnectParams {
	a.connMu.RLock()
	defer a.connMu.RUnlock()
	return a.connectParams
}

// workdirOverrideArg returns the directory passed via --workdir=<dir> on the
// command line, or "" if absent. Set by "Open Folder in New Window" when it
// relaunches the executable so the new instance opens that folder. Deliberately
// arg-only (no env fallback): direct exec always delivers argv, and honoring a
// stray/inherited env var would silently turn a normal launch into an override
// window (blanked remote/branch, non-persisting saves) with no clear indicator.
func workdirOverrideArg() string {
	for _, arg := range os.Args[1:] {
		if dir, ok := strings.CutPrefix(arg, "--workdir="); ok && dir != "" {
			return dir
		}
	}
	return ""
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
	// Apply persisted logging preferences (runtime log level, SQL-logging
	// switches) with any IT-admin policy on top of the build default level.
	a.applyLogPrefs(a.loadEffectiveLogPrefs())
	telemetry.Init(version.Version)
	logger.L.Info("application started")
	telemetry.Track(telemetry.EventAppStarted, nil)

	// Cache the export directory so file management IPC methods don't re-read config.
	// A --workdir override (this window was opened via "Open Folder in New Window")
	// wins over the persisted dir and retitles the window so the two are tellable apart.
	if dir := workdirOverrideArg(); dir != "" {
		logger.L.Info("launched with working-directory override", "dir", dir)
		a.setExportDir(dir)
		wailsruntime.WindowSetTitle(ctx, "Thaw — "+filepath.Base(dir))
	} else if cfg, err := config.Load(); err == nil {
		a.setExportDir(cfg.Git.ExportDir)
	}

	// Initialize the MCP manager with the Wails event emitter so MCP tools
	// can send events to the frontend (e.g. open_sql_tab).
	a.mcpManager = mcp.NewManager(func(eventName string, data interface{}) {
		wailsruntime.EventsEmit(ctx, eventName, data)
	})

	// Initialize delegated service instances.
	a.migrationSvc = migration.NewService(func(eventName string, data interface{}) {
		wailsruntime.EventsEmit(ctx, eventName, data)
	})
	// Let Snowpark resolve the working directory the override-aware way (an
	// "Open Folder in New Window" instance's folder lives only in memory).
	snowpark.SetWorkdirProvider(a.currentWorkdir)
	a.snowparkSvc = snowpark.NewService(ctx, func(tabId, role, wh, db, schema string) {
		if val, ok := a.tabSessions.Load(tabId); ok {
			ts := val.(*tabSession)
			ts.inUse.Add(1)
			defer ts.inUse.Add(-1)
			if role != "" {
				_ = ts.client.UseRole(ctx, role)
			}
			if wh != "" {
				_ = ts.client.UseWarehouse(ctx, wh)
			}
			if db != "" {
				_ = ts.client.UseDatabase(ctx, db)
			}
			if schema != "" {
				_ = ts.client.UseSchema(ctx, schema)
			}
		}
	})
	a.mcpManager.SetNotebookBackend(&notebookBackendAdapter{svc: a.snowparkSvc})

	// Open the function-metadata SQLite cache and seed it from the embedded
	// fallback JSON so autocomplete works immediately, even offline.
	if cfgDir, err := os.UserConfigDir(); err == nil {
		storeDir := filepath.Join(cfgDir, "Thaw")
		if store, err := fnmeta.Open(storeDir); err == nil {
			a.fnStore = store
			a.mcpManager.SetFnStore(store)
			go func() {
				if err := store.LoadFallback(); err != nil {
					logger.L.Warn("fnmeta: load fallback failed", "err", err)
				}
			}()
		} else {
			logger.L.Warn("fnmeta: open store failed", "err", err)
		}
	}

	// Apply session management config (pool limits, idle eviction).
	a.applySessionConfig(a.GetSessionConfig())
}

// isQueryRunning reports whether any tab has a query submitted by StartQuery still in flight.
func (a *App) isQueryRunning() bool {
	found := false
	a.tabSessions.Range(func(_, val any) bool {
		ts := val.(*tabSession)
		ts.queryMu.Lock()
		if ts.queryID != "" {
			found = true
		}
		ts.queryMu.Unlock()
		return !found
	})
	return found
}

// getOrInitTabSession returns the existing tab session for tabId, or lazily
// creates a new one (opening a fresh Snowflake connection that inherits the
// current connect params).
func (a *App) getOrInitTabSession(tabId string) (*tabSession, error) {
	if val, ok := a.tabSessions.Load(tabId); ok {
		ts := val.(*tabSession)
		ts.lastUsed.Store(time.Now().UnixNano())
		return ts, nil
	}
	tabSessionInitMu.Lock()
	// Double-check after acquiring the lock.
	if val, ok := a.tabSessions.Load(tabId); ok {
		tabSessionInitMu.Unlock()
		ts := val.(*tabSession)
		ts.lastUsed.Store(time.Now().UnixNano())
		return ts, nil
	}
	params := a.currentConnectParams()
	if params == nil {
		tabSessionInitMu.Unlock()
		return nil, apperrors.ErrNotConnected
	}
	logger.L.Info("creating new tab session", "tabId", tabId)
	a.evictIfNeeded()
	client, err := snowflake.NewClient(a.ctx, *params)
	if err != nil {
		tabSessionInitMu.Unlock()
		return nil, err
	}
	a.sessionConfigMu.RLock()
	maxOpen := a.sessionMaxOpen
	maxIdle := a.sessionMaxIdle
	a.sessionConfigMu.RUnlock()
	if maxOpen <= 0 {
		maxOpen = 4
	}
	if maxIdle <= 0 {
		maxIdle = 1
	}
	client.SetPoolLimits(maxOpen, maxIdle)
	// Inherit the query-log hook from the shared client so internal queries
	// on tab sessions are also captured.
	if shared := a.currentClient(); shared != nil {
		client.OnQuery = shared.OnQuery
	}
	ts := &tabSession{client: client}
	ts.lastUsed.Store(time.Now().UnixNano())
	a.tabSessions.Store(tabId, ts)
	tabSessionInitMu.Unlock()
	// Restore evicted session context outside the mutex — these are Snowflake
	// RPCs that can be slow on high-latency connections and must not block
	// other tabs from initializing their sessions concurrently.
	a.restoreSessionContext(tabId, ts)
	return ts, nil
}

// InitTabSession eagerly opens a dedicated Snowflake connection for the given
// tab ID.  Calling this after Connect ensures the tab session exists before the
// first query runs; subsequent calls for the same ID are no-ops.
func (a *App) InitTabSession(tabId string) error {
	_, err := a.getOrInitTabSession(tabId)
	return err
}

// CloseTabSession cancels any in-flight query and closes the Snowflake
// connection for the given tab, then removes it from the session map.
// It is a no-op when no session exists for tabId.
func (a *App) CloseTabSession(tabId string) {
	val, ok := a.tabSessions.LoadAndDelete(tabId)
	if !ok {
		a.evictedContexts.Delete(tabId)
		return
	}
	ts := val.(*tabSession)
	ts.queryMu.Lock()
	if ts.queryCancelFunc != nil {
		ts.queryCancelFunc()
	}
	ts.queryMu.Unlock()
	go ts.client.Close() //nolint:errcheck
	a.evictedContexts.Delete(tabId)
}

// evictIfNeeded closes the least-recently-used idle tab sessions until the
// session count is below the configured maximum. Must be called under
// tabSessionInitMu. Evicted session contexts are cached in evictedContexts
// so they can be restored transparently when the tab is next used.
func (a *App) evictIfNeeded() {
	a.sessionConfigMu.RLock()
	maxSessions := a.sessionMaxSessions
	a.sessionConfigMu.RUnlock()
	if maxSessions <= 0 {
		maxSessions = 8 // fallback before config is loaded
	}

	for {
		var count int
		a.tabSessions.Range(func(_, _ any) bool {
			count++
			return true
		})
		if count < maxSessions {
			return
		}

		// Find the LRU session that has no active query and is not in use.
		var lruTabId string
		var lruTime int64
		a.tabSessions.Range(func(key, val any) bool {
			ts := val.(*tabSession)
			ts.queryMu.Lock()
			hasQuery := ts.queryDone != nil
			ts.queryMu.Unlock()
			if hasQuery || ts.inUse.Load() > 0 {
				return true // skip sessions with active queries or in-use RPCs
			}
			lastUsed := ts.lastUsed.Load()
			if lruTabId == "" || lastUsed < lruTime {
				lruTabId = key.(string)
				lruTime = lastUsed
			}
			return true
		})
		if lruTabId == "" {
			return // all sessions are actively querying or in use; allow over-cap
		}

		// Evict: remove from map, cache context from connector's in-memory state
		// (no RPC — avoids blocking tabSessionInitMu on a Snowflake round-trip),
		// and close the connection asynchronously.
		if val, ok := a.tabSessions.LoadAndDelete(lruTabId); ok {
			ts := val.(*tabSession)
			a.evictedContexts.Store(lruTabId, ts.client.GetCachedSessionContext())
			logger.L.Info("evicting LRU tab session", "tabId", lruTabId)
			go ts.client.Close() //nolint:errcheck
		}
	}
}

// restoreSessionContext applies a previously-evicted session context to a
// freshly-created tab session. This ensures that switching back to an evicted
// tab transparently restores the user's role, warehouse, database, and schema.
// Called outside tabSessionInitMu so it must guard against concurrent eviction.
func (a *App) restoreSessionContext(tabId string, ts *tabSession) {
	val, ok := a.evictedContexts.LoadAndDelete(tabId)
	if !ok {
		return
	}
	ts.inUse.Add(1)
	defer ts.inUse.Add(-1)
	sctx := val.(snowflake.SessionContext)
	if sctx.Role != "" {
		if err := ts.client.UseRole(a.fctx(FeatureSessionSetup), sctx.Role); err != nil {
			logger.L.Debug("restoreSessionContext: failed to restore role", "tabId", tabId, "role", sctx.Role, "err", err)
		}
	}
	if sctx.Warehouse != "" {
		if err := ts.client.UseWarehouse(a.fctx(FeatureSessionSetup), sctx.Warehouse); err != nil {
			logger.L.Debug("restoreSessionContext: failed to restore warehouse", "tabId", tabId, "warehouse", sctx.Warehouse, "err", err)
		}
	}
	if sctx.Database != "" {
		if err := ts.client.UseDatabase(a.fctx(FeatureSessionSetup), sctx.Database); err != nil {
			logger.L.Debug("restoreSessionContext: failed to restore database", "tabId", tabId, "database", sctx.Database, "err", err)
		}
	}
	if sctx.Schema != "" {
		if err := ts.client.UseSchema(a.fctx(FeatureSessionSetup), sctx.Schema); err != nil {
			logger.L.Debug("restoreSessionContext: failed to restore schema", "tabId", tabId, "schema", sctx.Schema, "err", err)
		}
	}
}

// shutdown is called by the Wails runtime just before the application exits.
// It stops the embedded terminal, cancels any in-flight query, closes the
// Snowflake connection, and flushes logs and telemetry.
func (a *App) shutdown(_ context.Context) {
	// Persist window geometry so it can be restored on the next launch.
	w, h := wailsruntime.WindowGetSize(a.ctx)
	x, y := wailsruntime.WindowGetPosition(a.ctx)
	m := wailsruntime.WindowIsMaximised(a.ctx)
	_ = session.SaveWindowState(session.WindowState{X: x, Y: y, Width: w, Height: h, Maximized: m})

	// Stop idle eviction loop.
	a.sessionConfigMu.Lock()
	if a.sessionIdleStopCh != nil {
		close(a.sessionIdleStopCh)
		a.sessionIdleStopCh = nil
	}
	a.sessionConfigMu.Unlock()

	// Stop file system watcher.
	a.StopFileWatcher()

	// Stop all MCP sessions so their HTTP listeners and connections are freed.
	if a.mcpManager != nil {
		a.mcpManager.StopAll()
	}

	// Stop any running terminal process cleanly before the app exits.
	a.StopShell() //nolint:errcheck

	// Cancel any in-flight queries across all tab sessions so they stop
	// consuming Snowflake credits.  CancelQuery issues SYSTEM$CANCEL_QUERY in
	// a goroutine; give them a moment to fire before the process exits.
	if a.isQueryRunning() {
		var tabIds []string
		a.tabSessions.Range(func(key, _ any) bool {
			tabIds = append(tabIds, key.(string))
			return true
		})
		for _, tid := range tabIds {
			a.CancelQuery(tid)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Close all tab session clients asynchronously.
	a.tabSessions.Range(func(_, val any) bool {
		go val.(*tabSession).client.Close() //nolint:errcheck
		return true
	})

	if client := a.currentClient(); client != nil {
		// Close asynchronously — the gosnowflake driver sends an HTTP DELETE
		// /session to invalidate the token, which takes ~2 s. The app is
		// exiting anyway, so there is no need to wait; the OS will close the
		// TCP connection and Snowflake will expire the session on its own.
		go client.Close() //nolint:errcheck
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
	// Wire the query-log hook on the shared client so internal queries
	// (object listing, DDL fetching, session setup) are captured. Set it before
	// publishing the client below so tab sessions that snapshot it inherit the
	// hook and never observe it half-initialized.
	client.OnQuery = func(ctx context.Context, sql, qid string, err error, dur time.Duration) {
		src := querylog.GetSource(ctx)
		// File logging of executed SQL is independent of the in-memory query
		// log: it is governed solely by LogPrefs and writes to thaw.log.
		a.maybeFileLogQuery(src, sql, qid, err, dur)
		if !a.queryLog.IsEnabled() {
			return
		}
		if src == querylog.SourceUser {
			return // user queries are tracked separately with RUNNING→final status
		}
		status := querylog.StatusSuccess
		var errMsg string
		if err != nil {
			status = querylog.StatusFail
			errMsg = err.Error()
		}
		entry := querylog.Entry{
			Timestamp:  time.Now(),
			SQL:        sql,
			QueryID:    qid,
			Status:     status,
			DurationMs: dur.Milliseconds(),
			Error:      errMsg,
			Source:     querylog.SourceInternal,
			Feature:    querylog.GetFeature(ctx),
			TabID:      querylog.GetTabID(ctx),
		}
		entry.ID = a.queryLog.Record(entry)
		wailsruntime.EventsEmit(a.ctx, "querylog:entry", entry)
	}

	// Publish the connection under the write lock, then apply feature-flag
	// exclusions (which snapshot the client via currentClient(), so it must run
	// after the lock is released to avoid a self-deadlock on connMu).
	a.connMu.Lock()
	a.client = client
	a.connectParams = &params
	a.connMu.Unlock()
	a.applyFeatureFlagExclusions()

	logger.L.Info("connected", "account", params.Account, "user", params.User)
	telemetry.Track(telemetry.EventConnected, telemetry.Props{"authenticator": params.Authenticator})

	// Refresh the function metadata cache in the background.
	if a.fnStore != nil {
		go func() {
			if err := fnmeta.SyncFromSnowflake(a.fctx(FeatureSQLEditor), client, a.fnStore); err != nil {
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

// Disconnect closes the active Snowflake connection and all per-tab sessions.
func (a *App) Disconnect() error {
	// Stop all MCP sessions — they hold their own clients bound to this
	// connection and must not outlive it.
	if a.mcpManager != nil {
		a.mcpManager.StopAll()
	}

	// Close all tab sessions first.
	var tabIds []string
	a.tabSessions.Range(func(key, _ any) bool {
		tabIds = append(tabIds, key.(string))
		return true
	})
	for _, tid := range tabIds {
		a.CloseTabSession(tid)
	}

	// Clear any cached evicted session contexts.
	a.evictedContexts.Range(func(key, _ any) bool {
		a.evictedContexts.Delete(key)
		return true
	})

	// Clear the session query log on disconnect and notify the frontend so it
	// drops stale entries (the component may still be mounted).
	a.queryLog.Clear()
	wailsruntime.EventsEmit(a.ctx, "querylog:cleared")

	a.connMu.Lock()
	client := a.client
	a.client = nil
	a.connectParams = nil
	a.connMu.Unlock()
	if client == nil {
		return nil
	}
	err := client.Close()
	telemetry.Track(telemetry.EventDisconnected, nil)
	return err
}

// IsConnected returns true when a Snowflake connection is active.
func (a *App) IsConnected() bool {
	client := a.currentClient()
	return client != nil && client.IsAlive()
}

// applySessionConfig updates runtime session fields under lock and manages the idle eviction loop.
func (a *App) applySessionConfig(sc config.SessionConfig) {
	a.sessionConfigMu.Lock()
	a.sessionMaxSessions = sc.MaxSessions
	a.sessionMaxOpen = sc.MaxOpenConnsPerSession
	a.sessionMaxIdle = sc.MaxIdleConnsPerSession
	a.sessionInitMode = sc.InitMode
	a.sessionIdleTimeout = time.Duration(sc.IdleTimeoutMinutes) * time.Minute

	// Stop existing idle eviction loop if running.
	if a.sessionIdleStopCh != nil {
		close(a.sessionIdleStopCh)
		a.sessionIdleStopCh = nil
	}

	// Determine whether to start a new eviction loop while still holding the lock.
	var stop chan struct{}
	if sc.IdleTimeoutMinutes > 0 {
		stop = make(chan struct{})
		a.sessionIdleStopCh = stop
	}
	a.sessionConfigMu.Unlock()

	if stop != nil {
		go a.runIdleEvictionLoop(stop)
	}
}

// runIdleEvictionLoop periodically evicts sessions that have been idle longer than the configured timeout.
func (a *App) runIdleEvictionLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			a.evictIdleSessions()
		}
	}
}

// evictIdleSessions closes sessions whose lastUsed exceeds the idle timeout.
func (a *App) evictIdleSessions() {
	a.sessionConfigMu.RLock()
	timeout := a.sessionIdleTimeout
	a.sessionConfigMu.RUnlock()
	if timeout <= 0 {
		return
	}

	cutoff := time.Now().Add(-timeout).UnixNano()
	var toEvict []string

	a.tabSessions.Range(func(key, val any) bool {
		ts := val.(*tabSession)
		// Skip sessions with active queries or in-use RPCs.
		ts.queryMu.Lock()
		hasQuery := ts.queryDone != nil
		ts.queryMu.Unlock()
		if hasQuery || ts.inUse.Load() > 0 {
			return true
		}
		if ts.lastUsed.Load() < cutoff {
			toEvict = append(toEvict, key.(string))
		}
		return true
	})

	for _, tabId := range toEvict {
		val, ok := a.tabSessions.Load(tabId)
		if !ok {
			continue
		}
		ts := val.(*tabSession)
		// Re-check: skip if session was reactivated or is now in use.
		if ts.lastUsed.Load() >= cutoff || ts.inUse.Load() > 0 {
			continue
		}
		if _, ok := a.tabSessions.LoadAndDelete(tabId); ok {
			// Final guard: if the session was reactivated between our pre-check
			// and LoadAndDelete (e.g. getOrInitTabSession stamped lastUsed),
			// put it back. Safe under normal scheduling — the reactivation guard
			// covers the nanosecond-wide TOCTOU window between Load and
			// LoadAndDelete where getOrInitTabSession could stamp lastUsed.
			recentCutoff := time.Now().Add(-1 * time.Second).UnixNano()
			if ts.lastUsed.Load() >= recentCutoff {
				a.tabSessions.Store(tabId, ts)
				continue
			}
			// Cache session context from connector's in-memory state (no RPC)
			// only after confirming the deletion succeeded.
			a.evictedContexts.Store(tabId, ts.client.GetCachedSessionContext())
			logger.L.Info("evicting idle tab session", "tabId", tabId)
			go ts.client.Close() //nolint:errcheck
		}
	}
}

// AppInfo holds the application metadata shown in the About dialog.
type AppInfo struct {
	CompanyName    string `json:"companyName"`
	ProductName    string `json:"productName"`
	ProductVersion string `json:"productVersion"`
	Copyright      string `json:"copyright"`
	Comments       string `json:"comments"`
}

// GetAppInfo returns static application metadata for display in the About dialog.
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		CompanyName:    "Technarion Oy",
		ProductName:    "Thaw",
		ProductVersion: version.Version,
		Copyright:      "Copyright \u00a9 2026 Technarion Oy. Licensed under GPL-3.0-or-later.",
		Comments:       "Snowflake Manager \u2014 Free Software (GNU GPL v3)",
	}
}

// GetThirdPartyNotices returns the Markdown bundle of copyright notices and
// license texts for every third-party package Thaw redistributes. The content
// is embedded at build time from THIRD_PARTY_NOTICES.md and shown in the
// "About Thaw" dialog's Acknowledgements view.
func (a *App) GetThirdPartyNotices() string {
	return a.thirdPartyNotices
}

// alterObject issues `ALTER <objectType> <db>.<schema>.<name> <clause>`, the shared
// body behind the per-object Alter* IPC delegators. objectType carries any trailing
// modifier such as "IF EXISTS" (e.g. "TASK IF EXISTS").
func (a *App) alterObject(objectType, database, schema, name, clause string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER %s %s %s", objectType, snowflake.Qualify(database, schema, name), clause)
	_, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}
