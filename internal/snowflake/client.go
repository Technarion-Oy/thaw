// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sf "github.com/snowflakedb/gosnowflake/v2"

	"thaw/internal/sqltok"
)

// ConnectParams holds all fields needed to open a Snowflake connection.
// Authenticator values:
//
//	"snowflake"                 – password (+ optional TOTP passcode)
//	"username_password_mfa"     – password + MFA push notification
//	"externalbrowser"           – browser-based SSO (no password needed)
//	"okta"                      – Okta native SSO (requires OktaURL)
//	"snowflake_jwt"             – key-pair / JWT (requires PrivateKeyPath)
//	"oauth"                     – external OAuth token pass-through (Token or TokenFilePath)
//	"programmatic_access_token" – Snowflake PAT (Token or TokenFilePath; no user required)
//	"oauth_authorization_code"  – browser-based OAuth2 authorization-code flow
//	"oauth_client_credentials"  – non-interactive OAuth2 client-credentials flow
//	"workload_identity"         – cloud-native CSP identity (AWS/Azure/GCP)
type ConnectParams struct {
	Account       string `json:"account"`
	User          string `json:"user"`
	Password      string `json:"password"`
	Role          string `json:"role"`
	Warehouse     string `json:"warehouse"`
	Database      string `json:"database"`
	Schema        string `json:"schema"`
	Authenticator string `json:"authenticator"`
	// Passcode is a TOTP/hardware-token code; used with "snowflake" authenticator.
	Passcode string `json:"passcode"`
	// OktaURL is the Okta account URL; required for "okta" authenticator.
	OktaURL string `json:"oktaUrl"`
	// PrivateKeyPath is the path to a PEM-encoded private key; required for "snowflake_jwt".
	PrivateKeyPath string `json:"privateKeyPath"`
	// PrivateKeyPassphrase decrypts the private key if it is encrypted.
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`

	// Token is an OAuth or programmatic access token; used with "oauth" and
	// "programmatic_access_token" authenticators. Mutually exclusive with
	// TokenFilePath (the driver rejects setting both).
	Token string `json:"token"`
	// TokenFilePath points to a file the driver reads the token from instead
	// of Token. Mutually exclusive with Token.
	TokenFilePath string `json:"tokenFilePath"`

	// OAuthClientID is the OAuth2 client ID; required for the
	// "oauth_authorization_code" and "oauth_client_credentials" flows.
	OAuthClientID string `json:"oauthClientId"`
	// OAuthClientSecret is the OAuth2 client secret for the same flows.
	OAuthClientSecret string `json:"oauthClientSecret"`
	// OAuthTokenRequestURL is the IdP token endpoint for the OAuth2 flows.
	OAuthTokenRequestURL string `json:"oauthTokenRequestUrl"`
	// OAuthAuthorizationURL is the IdP authorization endpoint; used by the
	// "oauth_authorization_code" flow.
	OAuthAuthorizationURL string `json:"oauthAuthorizationUrl"`
	// OAuthRedirectURI is the redirect URI registered with the IdP. Optional;
	// the driver defaults to http://127.0.0.1:<random port>.
	OAuthRedirectURI string `json:"oauthRedirectUri"`
	// OAuthScope is a comma-separated list of scopes. Optional; derived from
	// the role when empty.
	OAuthScope string `json:"oauthScope"`
	// EnableSingleUseRefreshTokens enables single-use refresh tokens for the
	// Snowflake IdP in the OAuth2 flows.
	EnableSingleUseRefreshTokens bool `json:"enableSingleUseRefreshTokens"`

	// WorkloadIdentityProvider selects the CSP identity provider for the
	// "workload_identity" authenticator (e.g. "AWS", "AZURE", "GCP").
	WorkloadIdentityProvider string `json:"workloadIdentityProvider"`
	// WorkloadIdentityEntraResource is the Azure-only Entra resource for WIF.
	WorkloadIdentityEntraResource string `json:"workloadIdentityEntraResource"`
	// WorkloadIdentityImpersonationPath is a comma-separated impersonation
	// chain for AWS/GCP WIF. Not supported on Azure (the driver rejects it).
	WorkloadIdentityImpersonationPath string `json:"workloadIdentityImpersonationPath"`

	// Forward-proxy configuration. When ProxyHost is set, these map directly to
	// the gosnowflake sf.Config proxy fields and take precedence over the
	// HTTP_PROXY/HTTPS_PROXY/NO_PROXY environment variables. The driver defaults
	// ProxyProtocol to "http" when ProxyHost is set but ProxyProtocol is empty.
	ProxyHost     string `json:"proxyHost"`
	ProxyPort     int    `json:"proxyPort"`
	ProxyUser     string `json:"proxyUser"`
	ProxyPassword string `json:"proxyPassword"`
	ProxyProtocol string `json:"proxyProtocol"`
	NoProxy       string `json:"noProxy"`
}

// SnowflakeObject represents a database object (table, view, etc.).
type SnowflakeObject struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Schema string `json:"schema"`
	// Arguments holds the parameter type list for procedures and functions,
	// e.g. "NUMBER, VARCHAR". Empty for all other object kinds.
	Arguments string `json:"arguments"`
	// RowCount is populated for TABLE objects from the "rows" column of
	// SHOW OBJECTS. Nil means the count was unavailable (e.g. views, or
	// when SHOW OBJECTS does not include a rows column).
	RowCount *int64 `json:"rowCount,omitempty"`
	// Predecessors holds the raw predecessor string from SHOW TASKS.
	// Only populated for TASK objects; empty for all other kinds.
	Predecessors string `json:"predecessors,omitempty"`
	// Finalize holds the fully-qualified root task name for tasks that use a
	// FINALIZE clause (e.g. "DB"."SCHEMA"."ROOT_TASK"). Only populated for
	// TASK objects with a FINALIZE clause; empty for all other kinds and tasks.
	Finalize string `json:"finalize,omitempty"`
}

// QueryResult is the serialisable result of a SQL query.
// maxQueryRows is the maximum number of rows returned in a single query result.
// Results larger than this are silently truncated; QueryResult.Truncated is set
// to true so the frontend can warn the user. The cap exists to prevent the Wails
// IPC layer (JSON serialization + deserialization) from blocking the app for
// tens of seconds on very large result sets.
const maxQueryRows = 50_000

type QueryResult struct {
	Columns      []string        `json:"columns"`
	Rows         [][]interface{} `json:"rows"`
	RowsAffected int64           `json:"rowsAffected"`
	QueryID      string          `json:"queryID"`
	// Truncated is true when the result contained more than maxQueryRows rows
	// and was capped.  The frontend displays a warning in that case.
	Truncated bool `json:"truncated"`
}

// sessionConnector wraps a gosnowflake Connector and applies the current
// role, warehouse, database, and schema to every new connection the pool
// creates.  This lets the pool keep its normal concurrency (MaxOpenConns=32)
// while ensuring that each fresh connection immediately reflects the
// most-recently-switched session state — without requiring a single
// serializing connection.
//
// When UseRole, UseWarehouse, UseDatabase, or UseSchema is called:
//  1. The SQL statement is executed on whatever connection the pool provides.
//  2. The stored state is updated here (under the mutex).
//  3. All idle pool connections are flushed (SetMaxIdleConns 0 → restore) so
//     the next query that needs a connection gets a new one, which will have
//     this connector's Connect() called and therefore inherit the new state.
type sessionConnector struct {
	base driver.Connector // sf.NewConnector returns driver.Connector in v2
	mu   sync.RWMutex
	role string
	wh   string
	db   string // active database
	sc   string // active schema
}

// Connect opens a new raw driver connection via the underlying gosnowflake
// connector and immediately applies the stored role and warehouse, ensuring
// that every pooled connection reflects the current session state.
func (sc *sessionConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := sc.base.Connect(ctx)
	if err != nil {
		return nil, err
	}
	sc.mu.RLock()
	role, wh, db, schema := sc.role, sc.wh, sc.db, sc.sc
	sc.mu.RUnlock()

	// Helper for escaping identifiers

	if role != "" {
		connExec(conn, fmt.Sprintf(`USE ROLE %s`, QuoteIdent(role)))
	}
	if wh != "" {
		connExec(conn, fmt.Sprintf(`USE WAREHOUSE %s`, QuoteIdent(wh)))
	}
	if db != "" {
		connExec(conn, fmt.Sprintf(`USE DATABASE %s`, QuoteIdent(db)))
	}
	if schema != "" {
		// Use fully qualified name to be robust against database switches
		// resetting the schema context.
		if db != "" {
			connExec(conn, fmt.Sprintf(`USE SCHEMA %s`, Qualify(db, schema)))
		} else {
			connExec(conn, fmt.Sprintf(`USE SCHEMA %s`, QuoteIdent(schema)))
		}
	}
	return conn, nil
}

// Driver returns the underlying gosnowflake driver. Required by driver.Connector.
func (sc *sessionConnector) Driver() driver.Driver { return sc.base.Driver() }

// connExec runs a single statement on a raw driver.Conn (best-effort; errors
// are silently dropped because the caller has no useful recovery path here).
func connExec(conn driver.Conn, query string) {
	stmt, err := conn.Prepare(query)
	if err != nil {
		return
	}
	defer stmt.Close()          //nolint:errcheck
	stmt.Exec([]driver.Value{}) //nolint:errcheck,staticcheck // SA1019: called via driver interface, no modern alternative
}

// Client wraps a *sql.DB with Snowflake-specific helpers.
//
// Session state (role, warehouse) is managed through sessionConnector so that
// every connection the pool creates automatically inherits the current state.
// The pool runs at full concurrency (MaxOpenConns=32), which is essential for
// parallel DDL export across many databases and schemas.
type Client struct {
	db        *sql.DB
	connector *sessionConnector

	objectCacheMu sync.RWMutex
	objectCache   map[string]objectCacheEntry

	// currentUser caches SELECT CURRENT_USER() — constant for the connection's
	// lifetime. Resolved lazily via GetCurrentUserCached; failures are not cached.
	currentUserMu sync.RWMutex
	currentUser   string

	// excludedExtendedKinds stores a map[string]bool of object kinds to skip
	// in ListExtendedObjects. Accessed via SetExcludedExtendedKinds (write)
	// and getExcludedExtendedKinds (read) which use atomic.Value for safe
	// concurrent access without locking.
	excludedExtendedKinds atomic.Value // stores map[string]bool

	// OnQuery is an optional hook called after every SQL statement execution.
	// Parameters: ctx, sql text, query ID (may be empty), error (nil on success),
	// wall-clock duration. Nil-checked before invocation.
	OnQuery func(ctx context.Context, sql string, queryID string, err error, dur time.Duration)
}

// ── Instrumented DB wrappers ────────────────────────────────────────────────
// These replace direct c.db.QueryContext / ExecContext / QueryRowContext calls
// so that every SQL round-trip fires the OnQuery hook.

// queryCtx wraps c.db.QueryContext with the OnQuery hook.
func (c *Client) queryCtx(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := c.db.QueryContext(ctx, query, args...)
	if c.OnQuery != nil {
		c.OnQuery(ctx, query, "", err, time.Since(start))
	}
	return rows, err
}

// execCtx wraps c.db.ExecContext with the OnQuery hook.
func (c *Client) execCtx(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := c.db.ExecContext(ctx, query, args...)
	if c.OnQuery != nil {
		c.OnQuery(ctx, query, "", err, time.Since(start))
	}
	return result, err
}

// queryRowCtx wraps c.db.QueryRowContext with the OnQuery hook.
// NOTE: The hook always fires with err=nil because QueryRowContext defers the
// actual error to Row.Scan(). Queries routed through this wrapper will appear
// as SUCCESS in the query log even if the subsequent Scan() fails. This is a
// known limitation of the sql.Row API — the network round-trip succeeded but
// the row-level result may still carry an error.
func (c *Client) queryRowCtx(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	row := c.db.QueryRowContext(ctx, query, args...)
	if c.OnQuery != nil {
		c.OnQuery(ctx, query, "", nil, time.Since(start))
	}
	return row
}

// SetExcludedExtendedKinds atomically replaces the set of object kinds that
// ListExtendedObjects will skip. Safe to call concurrently with ListExtendedObjects.
func (c *Client) SetExcludedExtendedKinds(kinds map[string]bool) {
	c.excludedExtendedKinds.Store(kinds)
}

func (c *Client) getExcludedExtendedKinds() map[string]bool {
	v := c.excludedExtendedKinds.Load()
	if v == nil {
		return nil
	}
	return v.(map[string]bool)
}

type objectCacheEntry struct {
	objects []SnowflakeObject
	ts      time.Time
}

const objectCacheTTL = 30 * time.Second

// NewClient opens a new Snowflake connection. The provided context can be
// canceled to abort the login handshake (useful for MFA/browser flows).
func NewClient(ctx context.Context, p ConnectParams) (*Client, error) {
	authMap := map[string]sf.AuthType{
		"username_password_mfa":     sf.AuthTypeUsernamePasswordMFA,
		"externalbrowser":           sf.AuthTypeExternalBrowser,
		"okta":                      sf.AuthTypeOkta,
		"snowflake_jwt":             sf.AuthTypeJwt,
		"oauth":                     sf.AuthTypeOAuth,
		"programmatic_access_token": sf.AuthTypePat,
		"oauth_authorization_code":  sf.AuthTypeOAuthAuthorizationCode,
		"oauth_client_credentials":  sf.AuthTypeOAuthClientCredentials,
		"workload_identity":         sf.AuthTypeWorkloadIdentityFederation,
	}
	auth, ok := authMap[strings.ToLower(p.Authenticator)]
	if !ok {
		auth = sf.AuthTypeSnowflake
	}

	// Interactive flows need more time; plain password should fail quickly.
	// LoginTimeout is the gosnowflake-internal control — context cancellation
	// alone is not reliable for aborting auth inside the driver.
	// externalbrowser and oauth_authorization_code both open a browser and wait
	// for the user to complete consent, so they share the long interactive
	// timeout. okta and MFA-push are interactive on the device side.
	authenticatorLower := strings.ToLower(p.Authenticator)
	loginTimeout := 15 * time.Second
	if authenticatorLower == "username_password_mfa" ||
		authenticatorLower == "externalbrowser" ||
		authenticatorLower == "okta" ||
		authenticatorLower == "oauth_authorization_code" {
		loginTimeout = 3 * time.Minute
	}

	keepAlive := "true"
	cfg := &sf.Config{
		Account:       p.Account,
		User:          p.User,
		Password:      p.Password,
		Role:          p.Role,
		Warehouse:     p.Warehouse,
		Database:      p.Database,
		Schema:        p.Schema,
		Authenticator: auth,
		Passcode:      p.Passcode,
		LoginTimeout:  loginTimeout,
		// ServerSessionKeepAlive prevents the driver from sending DELETE /session
		// when the pool recycles a connection, which would invalidate the
		// shared Snowflake session and break all other pool connections.
		ServerSessionKeepAlive: true,
		// Params must be initialized to a non-nil map.  ParseDSN does this
		// automatically, but we construct the Config directly, so we must do
		// it ourselves — otherwise the driver panics with "assignment to entry
		// in nil map" when it writes session parameters back into cfg.Params.
		//
		// client_session_keep_alive tells the driver to start a heartbeat
		// goroutine (startHeartBeat) on every new connection.  Without it,
		// Snowflake times out the idle session during long-running queries.
		Params: map[string]*string{
			"client_session_keep_alive": &keepAlive,
		},
	}

	if p.OktaURL != "" {
		u, err := url.Parse(p.OktaURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Okta URL: %w", err)
		}
		cfg.OktaURL = u
	}

	if p.PrivateKeyPath != "" {
		key, err := loadPrivateKey(p.PrivateKeyPath, p.PrivateKeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("load private key: %w", err)
		}
		cfg.PrivateKey = key
	}

	// Token-based authenticators (oauth, programmatic_access_token). The driver
	// rejects setting both Token and TokenFilePath, so surface a clear error
	// before the login handshake.
	if p.Token != "" && p.TokenFilePath != "" {
		return nil, fmt.Errorf("token and tokenFilePath cannot both be set")
	}
	cfg.Token = p.Token
	cfg.TokenFilePath = p.TokenFilePath

	// OAuth2 flows (oauth_authorization_code, oauth_client_credentials).
	cfg.OauthClientID = p.OAuthClientID
	cfg.OauthClientSecret = p.OAuthClientSecret
	cfg.OauthTokenRequestURL = p.OAuthTokenRequestURL
	cfg.OauthAuthorizationURL = p.OAuthAuthorizationURL
	cfg.OauthRedirectURI = p.OAuthRedirectURI
	cfg.OauthScope = p.OAuthScope
	cfg.EnableSingleUseRefreshTokens = p.EnableSingleUseRefreshTokens

	// Workload Identity Federation. Impersonation path is a comma-separated
	// chain split into the driver's []string; Azure does not support it (the
	// driver rejects the combination, so reject it early with a clear message).
	cfg.WorkloadIdentityProvider = p.WorkloadIdentityProvider
	cfg.WorkloadIdentityEntraResource = p.WorkloadIdentityEntraResource
	if path := splitCommaList(p.WorkloadIdentityImpersonationPath); len(path) > 0 {
		if strings.EqualFold(strings.TrimSpace(p.WorkloadIdentityProvider), "azure") {
			return nil, fmt.Errorf("impersonation path is not supported for the Azure workload identity provider")
		}
		cfg.WorkloadIdentityImpersonationPath = path
	}

	// Forward proxy. When ProxyHost is set, these take precedence over the
	// HTTP_PROXY/HTTPS_PROXY/NO_PROXY environment variables (which the driver
	// otherwise honors as a fallback). Empty ProxyHost leaves all proxy fields
	// unset so non-proxied connections are unaffected.
	if p.ProxyHost != "" {
		cfg.ProxyHost = p.ProxyHost
		cfg.ProxyPort = p.ProxyPort
		cfg.ProxyUser = p.ProxyUser
		cfg.ProxyPassword = p.ProxyPassword
		cfg.ProxyProtocol = p.ProxyProtocol
		cfg.NoProxy = p.NoProxy
	}

	// Build the connector directly from the config (avoids the DSN round-trip
	// and gives us a driver.Connector we can wrap).
	sc := &sessionConnector{
		base: sf.NewConnector(&sf.SnowflakeDriver{}, *cfg),
		role: strings.TrimSpace(p.Role),
		wh:   strings.TrimSpace(p.Warehouse),
		db:   strings.TrimSpace(p.Database), // initialize from profile so Connect() applies it
		sc:   strings.TrimSpace(p.Schema),   // on connections created after pool recycles
	}

	db := sql.OpenDB(sc)

	// Keep a modest pool to avoid Snowflake session quota exhaustion.
	// With ServerSessionKeepAlive=true, each pool connection holds a live
	// Snowflake session that persists until Snowflake's 4h timeout — even
	// after the Go side closes it.  Setting MaxIdleConns equal to
	// MaxOpenConns prevents connection churn that creates zombie sessions.
	// Use SetPoolLimits(32, 32) for bulk operations like DDL export.
	db.SetMaxOpenConns(DefaultMaxOpenConns)
	db.SetMaxIdleConns(DefaultMaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close() //nolint:errcheck
		return nil, err
	}

	// Capture the role the server actually assigned (may differ from p.Role if
	// the user left it blank or Snowflake normalised it).
	var serverRole string
	if err2 := db.QueryRowContext(ctx, "SELECT CURRENT_ROLE()").Scan(&serverRole); err2 == nil {
		sc.mu.Lock()
		sc.role = strings.TrimSpace(serverRole)
		sc.mu.Unlock()
	}

	return &Client{db: db, connector: sc, objectCache: make(map[string]objectCacheEntry)}, nil
}

// IsAlive checks that the underlying connection is still usable.
func (c *Client) IsAlive() bool {
	return c.db.PingContext(context.Background()) == nil
}

// DefaultMaxOpenConns is the shared client's default MaxOpenConns (used by NewClient).
const DefaultMaxOpenConns = 8

// DefaultMaxIdleConns is the shared client's default MaxIdleConns (used by NewClient).
const DefaultMaxIdleConns = 8

// SetPoolLimits overrides the connection pool's MaxOpenConns and MaxIdleConns.
// Tab sessions use smaller limits (e.g. 4/1) since they only run one query at
// a time; the shared client uses DefaultMaxOpenConns/DefaultMaxIdleConns.
func (c *Client) SetPoolLimits(maxOpen, maxIdle int) {
	c.db.SetMaxOpenConns(maxOpen)
	c.db.SetMaxIdleConns(maxIdle)
}

// GetSessionID returns the Snowflake session ID via SELECT CURRENT_SESSION().
func (c *Client) GetSessionID(ctx context.Context) (string, error) {
	var id string
	if err := c.queryRowCtx(ctx, "SELECT CURRENT_SESSION()").Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// GetCurrentUser returns the current session's user via SELECT CURRENT_USER().
func (c *Client) GetCurrentUser(ctx context.Context) (string, error) {
	var name string
	if err := c.queryRowCtx(ctx, "SELECT CURRENT_USER()").Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

// GetCurrentUserCached returns the current session's user, resolving it via
// GetCurrentUser on first use and caching the result for the client's lifetime.
// The user is constant for a connection, so this avoids repeating the round-trip.
// A failed lookup is not cached, so a later call can retry.
func (c *Client) GetCurrentUserCached(ctx context.Context) (string, error) {
	// Warm path: a shared read lock so concurrent callers don't serialize.
	c.currentUserMu.RLock()
	cached := c.currentUser
	c.currentUserMu.RUnlock()
	if cached != "" {
		return cached, nil
	}
	// Cold path: resolve outside the lock (no RPC under the mutex), then store
	// under the write lock, re-checking in case another caller won the race.
	name, err := c.GetCurrentUser(ctx)
	if err != nil {
		return "", err
	}
	c.currentUserMu.Lock()
	defer c.currentUserMu.Unlock()
	if c.currentUser != "" {
		return c.currentUser, nil
	}
	c.currentUser = name
	return name, nil
}

// GetCachedSessionContext returns the session context from the connector's
// in-memory cache without making a Snowflake RPC. Useful when the caller needs
// the context but cannot tolerate network latency (e.g. under a mutex).
func (c *Client) GetCachedSessionContext() SessionContext {
	c.connector.mu.RLock()
	ctx := SessionContext{
		Role:      c.connector.role,
		Warehouse: c.connector.wh,
		Database:  c.connector.db,
		Schema:    c.connector.sc,
	}
	c.connector.mu.RUnlock()
	return ctx
}

// Close terminates the connection pool.
func (c *Client) Close() error {
	return c.db.Close()
}

// GetConn returns a pinned connection from the pool. The caller MUST call
// conn.Close() when finished.
func (c *Client) GetConn(ctx context.Context) (*sql.Conn, error) {
	return c.db.Conn(ctx)
}

// refreshConnectorState updates the stored session state and flushes the
// connection pool ONLY if a change occurred that would affect future
// connections (role, warehouse, or a database switch that resets schema).
// database name or schema name changes are synced to the connector but
// do not require a pool flush as new connections will inherit them anyway.
func (c *Client) refreshConnectorState(role, wh, db, sc string) {
	c.connector.mu.Lock()
	// Only flush if role or warehouse changed, or if database changed.
	// We don't flush for schema-only changes as they are cheap.
	flushNeeded := (role != "" && c.connector.role != role) ||
		(wh != "" && c.connector.wh != wh) ||
		(db != "" && c.connector.db != db)

	if role != "" {
		c.connector.role = role
	}
	if wh != "" {
		c.connector.wh = wh
	}
	if db != "" {
		c.connector.db = db
	}
	if sc != "" {
		c.connector.sc = sc
	}
	c.connector.mu.Unlock()

	if flushNeeded {
		// Flush idle connections so they are re-created (and go through
		// Connect()) with the new state.
		c.db.SetMaxIdleConns(0)
		c.db.SetMaxIdleConns(2)
	}
}

// isContextChangingQuery returns true if the SQL statement appears to be one
// that changes the session's role, warehouse, database, or schema.
func isContextChangingQuery(sql string) bool {
	s := strings.TrimSpace(sql)
	if len(s) < 3 {
		return false
	}
	up := strings.ToUpper(s)
	// Covers USE ROLE, USE WAREHOUSE, USE DATABASE, USE SCHEMA, and bare USE <db>.<sch>.
	if strings.HasPrefix(up, "USE") {
		return true
	}
	// Covers ALTER SESSION SET ...
	if strings.HasPrefix(up, "ALTER") && strings.Contains(up, "SESSION") {
		return true
	}
	return false
}

// Execute runs one or more semicolon-separated SQL statements sequentially and
// returns the last result set.
//
// For multi-statement scripts two things are required:
//
//  1. A dedicated *sql.Conn so every statement shares the same Snowflake
//     session. database/sql's pool would otherwise route consecutive calls to
//     different connections (different sessions), breaking LAST_QUERY_ID() /
//     RESULT_SCAN which are session-scoped.
//
//  2. A plain context WITHOUT sf.WithAsyncMode. Async mode makes the driver
//     return a placeholder rows object immediately and poll Snowflake in a
//     background goroutine. database/sql keeps the *sql.Conn marked as busy
//     until all rows are closed; the second statement's conn.QueryContext
//     then deadlocks waiting for the conn to become free. Running without
//     async mode means each statement blocks until its results arrive, which
//     is exactly what sequential scripts need.
//
// Execute runs one or more semicolon-separated SQL statements sequentially and
// returns the last result set.
//
// onProgress, if provided, is called once per statement just before execution
// begins.  It receives the zero-based statement index, total statement count,
// and a receive-only channel that will deliver the Snowflake query ID for that
// statement as soon as the driver receives it from Snowflake (well before the
// statement finishes).  The parameter is variadic so all existing callers
// remain unchanged.
func (c *Client) Execute(ctx context.Context, query string, onProgress ...func(idx, total int, qidChan <-chan string)) (*QueryResult, error) {
	rawStmts := sqltok.Split(query)
	stmts := make([]string, len(rawStmts))
	for i, s := range rawStmts {
		stmts[i] = normalizePutGet(s)
	}
	if len(stmts) == 0 {
		return &QueryResult{Rows: [][]interface{}{}}, nil
	}
	if len(stmts) == 1 {
		stmt := stmts[0]
		// PUT/GET commands are incompatible with async mode. When the caller's
		// context carries sf.WithAsyncMode (as StartQuery always does), the
		// Snowflake server returns the query ID immediately without the full
		// execResponse payload, so src_locations is empty and the driver
		// raises "264004: failed to parse location". Use a plain cancellable
		// context (no async mode) for file-transfer statements, mirroring the
		// same pattern already used for multi-statement execution above.
		upper := strings.ToUpper(stmt)
		if strings.HasPrefix(upper, "PUT ") || strings.HasPrefix(upper, "PUT\t") ||
			strings.HasPrefix(upper, "GET ") || strings.HasPrefix(upper, "GET\t") {
			syncCtx, syncCancel := context.WithCancel(context.Background())
			defer syncCancel()
			go func() {
				select {
				case <-ctx.Done():
					syncCancel()
				case <-syncCtx.Done():
				}
			}()
			return c.QuerySingle(syncCtx, stmt)
		}
		return c.QuerySingle(ctx, stmt)
	}

	// Build a plain cancellable context that inherits cancellation from ctx
	// but carries none of the sf-specific context values (async mode, qidChan).
	execCtx, execCancel := context.WithCancel(context.Background())
	defer execCancel()
	go func() {
		select {
		case <-ctx.Done():
			execCancel()
		case <-execCtx.Done():
		}
	}()

	conn, err := c.db.Conn(execCtx)
	if err != nil {
		return nil, err
	}
	defer conn.Close() //nolint:errcheck

	var last *QueryResult
	for i, stmt := range stmts {
		// Attach a fresh per-statement qidChan so the driver can deliver the
		// Snowflake query ID for this specific statement.
		qidChan := make(chan string, 1)
		stmtCtx := sf.WithQueryIDChan(execCtx, qidChan)

		if len(onProgress) > 0 && onProgress[0] != nil {
			onProgress[0](i, len(stmts), qidChan)
		}

		start := time.Now()
		result, err := queryOnConn(stmtCtx, conn, stmt)
		dur := time.Since(start)
		if err != nil {
			if c.OnQuery != nil {
				c.OnQuery(ctx, stmt, "", err, dur)
			}
			return nil, err
		}
		if c.OnQuery != nil {
			c.OnQuery(ctx, stmt, "", nil, dur)
		}
		last = result
	}

	// Sync session context while the connection is still pinned so the
	// connector's state correctly reflects any USE statements just executed.
	// Only do this if at least one statement might have changed the context,
	// or if it's a multi-statement script (safer).
	if len(stmts) > 1 || isContextChangingQuery(query) {
		// execCtx was built with context.WithCancel(context.Background())
		// so it does NOT have any async mode flags.
		_, _ = c.GetSessionContextOnConn(execCtx, conn)
	}

	return last, nil
}

// ctxErrOrErr returns ctx.Err() when the context is done, otherwise err.
// Used to normalise gosnowflake driver errors that arise as side-effects of
// context cancellation (e.g. "Object does not exist" from an S3 pre-signed URL
// that the driver tried to re-check after the HTTP request was canceled).
func ctxErrOrErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// queryOnConn executes a single SQL statement on a pinned *sql.Conn and
// returns its result set.
func queryOnConn(ctx context.Context, conn *sql.Conn, query string) (*QueryResult, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, ctxErrOrErr(ctx, err)
	}

	cols, err := rows.Columns()
	if err != nil {
		rows.Close() //nolint:errcheck
		return nil, ctxErrOrErr(ctx, err)
	}
	result := &QueryResult{Columns: cols, Rows: [][]interface{}{}}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			rows.Close() //nolint:errcheck
			return nil, ctxErrOrErr(ctx, err)
		}
		result.Rows = append(result.Rows, vals)
		if len(result.Rows) >= maxQueryRows {
			result.Truncated = true
			break
		}
	}
	// When the context was canceled the gosnowflake driver may stall inside
	// rows.Close() while draining buffered Arrow chunks over the network.
	// When the row limit was hit there may be many remaining Arrow chunks.
	// In both cases fire Close in a goroutine so this function returns
	// immediately without blocking the caller.
	if ctx.Err() != nil {
		go rows.Close() //nolint:errcheck
		return nil, ctx.Err()
	}
	if result.Truncated {
		go rows.Close() //nolint:errcheck
		return result, nil
	}
	rows.Close() //nolint:errcheck
	return result, rows.Err()
}

// QuerySingle executes a single SQL statement without multi-statement mode and
// returns the result set. Use this instead of Execute for TABLE() function calls
// and other queries that are incompatible with the multi-statement API.
func (c *Client) QuerySingle(ctx context.Context, query string) (*QueryResult, error) {
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	start := time.Now()
	result, err := queryOnConn(ctx, conn, query)
	dur := time.Since(start)
	if err != nil {
		if c.OnQuery != nil {
			c.OnQuery(ctx, query, "", err, dur)
		}
		return nil, err
	}

	// Sync session context while the connection is still pinned so the
	// connector's state correctly reflects any USE statements just executed.
	if isContextChangingQuery(query) {
		// To ensure the sync query is NOT async, we use a background context
		// that inherits cancellation from ctx but carries no Snowflake flags.
		syncCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-syncCtx.Done():
			}
		}()
		_, _ = c.GetSessionContextOnConn(syncCtx, conn)
	}

	if c.OnQuery != nil {
		c.OnQuery(ctx, query, "", nil, dur)
	}
	return result, nil
}

// normalizePutGet prepares PUT and GET statements for the gosnowflake driver:
//
//  1. Collapses internal newlines to spaces — the Snowflake server's location
//     parser treats newlines as line terminators, so a multi-line PUT/GET
//     (e.g. "PUT file://...\n @stage") causes the server to return an empty
//     src_locations list, which the driver reports as "264004: failed to parse
//     location".
//
//  2. Ensures the file:// path in PUT commands is single-quoted — the driver
//     requires the path to be quoted (as in `PUT 'file://...' @stage`) so the
//     server correctly echoes the path back in src_locations. Unquoted paths
//     (e.g. typed directly in the query editor) result in empty src_locations.
//     Already-quoted paths are left unchanged.
func normalizePutGet(stmt string) string {
	upper := strings.ToUpper(stmt)
	if !strings.HasPrefix(upper, "PUT ") && !strings.HasPrefix(upper, "PUT\t") &&
		!strings.HasPrefix(upper, "GET ") && !strings.HasPrefix(upper, "GET\t") {
		return stmt
	}
	// Step 1: collapse newlines.
	s := strings.ReplaceAll(stmt, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	// Step 2: quote unquoted file:// paths in PUT commands.
	if strings.HasPrefix(upper, "PUT ") || strings.HasPrefix(upper, "PUT\t") {
		s = quotePutFilePath(s)
	}
	return s
}

// quotePutFilePath wraps the file:// path in a PUT statement with single quotes
// if it is not already quoted. Any single quotes within the path are escaped as \'.
func quotePutFilePath(stmt string) string {
	const proto = "file://"
	idx := strings.Index(strings.ToLower(stmt), proto)
	if idx < 0 {
		return stmt
	}
	// Already quoted — nothing to do.
	if idx > 0 && stmt[idx-1] == '\'' {
		return stmt
	}
	// Find end of the unquoted path: first whitespace or semicolon.
	end := idx + len(proto)
	for end < len(stmt) && stmt[end] != ' ' && stmt[end] != '\t' && stmt[end] != ';' {
		end++
	}
	path := stmt[idx:end]
	// Escape any literal single quotes in the path.
	path = strings.ReplaceAll(path, "'", `\'`)
	return stmt[:idx] + "'" + path + "'" + stmt[end:]
}

// CancelSnowflakeQuery asks Snowflake to abort the query with the given ID.
// This is a best-effort call; the caller may ignore errors.
func (c *Client) CancelSnowflakeQuery(ctx context.Context, queryID string) error {
	escaped := EscapeStringLit(queryID)
	_, err := c.execCtx(ctx, fmt.Sprintf("SELECT SYSTEM$CANCEL_QUERY('%s')", escaped))
	return err
}

// SessionContext holds the current session's active role, warehouse, database and schema.
type SessionContext struct {
	Role      string `json:"role"`
	Warehouse string `json:"warehouse"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
}

// GetSessionContext returns the currently active role, warehouse, database, and schema.
// It also syncs the connector state so that UseSchema/UseWarehouse always use the
// correct database and schema context, even after USE commands run in the SQL editor
// bypassed the UseDatabase/UseSchema IPC methods.
func (c *Client) GetSessionContext(ctx context.Context) (SessionContext, error) {
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return SessionContext{}, err
	}
	defer func() { _ = conn.Close() }()
	return c.GetSessionContextOnConn(ctx, conn)
}

// GetSessionContextOnConn is the same as GetSessionContext but runs on a pinned connection.
//
// NOTE: Intentionally not instrumented via the OnQuery hook. This runs on a
// pinned *sql.Conn (not c.db) so the queryRowCtx wrapper cannot be used, and
// the query fires after every QuerySingle/ExecuteOnConn call — logging it
// would add significant noise to the query log without useful signal.
func (c *Client) GetSessionContextOnConn(ctx context.Context, conn *sql.Conn) (SessionContext, error) {
	// Ensure sync query is NOT async even if the parent context is.
	syncCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-syncCtx.Done():
		}
	}()

	row := conn.QueryRowContext(syncCtx,
		"SELECT CURRENT_ROLE(), CURRENT_WAREHOUSE(), CURRENT_DATABASE(), CURRENT_SCHEMA()")
	// Warehouse, database and schema can be SQL NULL when not set in the session.
	var role, warehouse, database, schema sql.NullString
	if err := row.Scan(&role, &warehouse, &database, &schema); err != nil {
		return SessionContext{}, err
	}
	sc := SessionContext{
		Role:      role.String,
		Warehouse: warehouse.String,
		Database:  database.String,
		Schema:    schema.String,
	}
	c.refreshConnectorState(sc.Role, sc.Warehouse, sc.Database, sc.Schema)
	return sc, nil
}

// ExecuteOnConn runs one or more SQL statements on a pinned connection and
// returns the last result set. It also syncs the session context.
func (c *Client) ExecuteOnConn(ctx context.Context, conn *sql.Conn, query string) (*QueryResult, error) {
	rawStmts := sqltok.Split(query)
	stmts := make([]string, len(rawStmts))
	for i, s := range rawStmts {
		stmts[i] = normalizePutGet(s)
	}
	if len(stmts) == 0 {
		return &QueryResult{Rows: [][]interface{}{}}, nil
	}

	var last *QueryResult
	for _, stmt := range stmts {
		start := time.Now()
		result, err := queryOnConn(ctx, conn, stmt)
		dur := time.Since(start)
		if err != nil {
			if c.OnQuery != nil {
				c.OnQuery(ctx, stmt, "", err, dur)
			}
			return nil, err
		}
		if c.OnQuery != nil {
			c.OnQuery(ctx, stmt, "", nil, dur)
		}
		last = result
	}

	// Sync session context while the connection is still pinned.
	_, _ = c.GetSessionContextOnConn(ctx, conn)

	return last, nil
}

// ListRoles returns all roles visible to the current role via SHOW ROLES.
// Used for informational displays (Account Objects panel, user-management
// default-role pickers) where the full visible set is desired.
func (c *Client) ListRoles(ctx context.Context) ([]string, error) {
	// SHOW ROLES columns: created_on, name, ...
	return c.queryStringSlice(ctx, "SHOW ROLES", 1)
}

// ListAvailableRoles returns only the roles the current user can actually
// switch to. It uses CURRENT_AVAILABLE_ROLES() which returns the JSON array
// of every role reachable through the user's role hierarchy — exactly the set
// that USE ROLE will accept without error.
func (c *Client) ListAvailableRoles(ctx context.Context) ([]string, error) {
	var raw string
	if err := c.queryRowCtx(ctx, "SELECT CURRENT_AVAILABLE_ROLES()").Scan(&raw); err != nil {
		return nil, err
	}
	// Result is a JSON array string, e.g. ["PUBLIC","SYSADMIN","ACCOUNTADMIN"]
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	var roles []string
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		token = strings.Trim(token, `"`)
		if token != "" {
			roles = append(roles, token)
		}
	}
	sort.Strings(roles)
	return roles, nil
}

type SecurityIntegration struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
	Comment  string `json:"comment"`
}

// ListSecurityIntegrations returns all security integrations.
func (c *Client) ListSecurityIntegrations(ctx context.Context) ([]SecurityIntegration, error) {
	res, err := c.Execute(ctx, "SHOW SECURITY INTEGRATIONS")
	if err != nil {
		return nil, err
	}

	idxs := colIndexMap(res.Columns, "name", "type", "category", "enabled", "comment")
	var ints []SecurityIntegration
	for _, row := range res.Rows {
		ints = append(ints, SecurityIntegration{
			Name:     strVal(row, idxs["name"]),
			Type:     strVal(row, idxs["type"]),
			Category: strVal(row, idxs["category"]),
			Enabled:  strVal(row, idxs["enabled"]) == "true",
			Comment:  strVal(row, idxs["comment"]),
		})
	}
	return ints, nil
}

// ListWarehouses returns the names of all warehouses visible to the current role.
func (c *Client) ListWarehouses(ctx context.Context) ([]string, error) {
	// SHOW WAREHOUSES columns: name, state, ...
	return c.queryStringSlice(ctx, "SHOW WAREHOUSES", 0)
}

// ListComputePools returns the names of all compute pools visible to the current
// role. Compute pools host Snowpark Container Services (SERVICE / EXECUTE JOB
// SERVICE) and are account-level objects.
func (c *Client) ListComputePools(ctx context.Context) ([]string, error) {
	// SHOW COMPUTE POOLS columns: name, state, min_nodes, ...
	return c.queryStringSlice(ctx, "SHOW COMPUTE POOLS", 0)
}

// ListNotificationIntegrations returns the names of all notification integrations.
// These are used for ERROR_INTEGRATION and SUCCESS_INTEGRATION in tasks.
func (c *Client) ListNotificationIntegrations(ctx context.Context) ([]string, error) {
	// SHOW NOTIFICATION INTEGRATIONS columns: created_on, name, type, category, enabled, comment
	return c.queryStringSlice(ctx, "SHOW NOTIFICATION INTEGRATIONS", 1)
}

// ListExternalVolumes returns the names of all external volumes visible to the current role.
func (c *Client) ListExternalVolumes(ctx context.Context) ([]string, error) {
	// SHOW EXTERNAL VOLUMES columns: created_on, name, ...
	return c.queryStringSlice(ctx, "SHOW EXTERNAL VOLUMES", 1)
}

// ApiIntegration holds metadata for a single API integration.
type ApiIntegration struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	Comment string `json:"comment"`
}

// ListApiIntegrations returns all API integrations visible to the current role.
func (c *Client) ListApiIntegrations(ctx context.Context) ([]ApiIntegration, error) {
	res, err := c.Execute(ctx, "SHOW API INTEGRATIONS")
	if err != nil {
		return nil, err
	}

	idxs := colIndexMap(res.Columns, "name", "type", "enabled", "comment")
	var ints []ApiIntegration
	for _, row := range res.Rows {
		ints = append(ints, ApiIntegration{
			Name:    strVal(row, idxs["name"]),
			Type:    strVal(row, idxs["type"]),
			Enabled: strVal(row, idxs["enabled"]) == "true",
			Comment: strVal(row, idxs["comment"]),
		})
	}
	return ints, nil
}

// AccountSecret holds the name and location of a secret visible at the account level.
type AccountSecret struct {
	Name         string `json:"name"`
	DatabaseName string `json:"databaseName"`
	SchemaName   string `json:"schemaName"`
}

// ListSecretsInAccount returns all secrets visible to the current role across the account.
func (c *Client) ListSecretsInAccount(ctx context.Context) ([]AccountSecret, error) {
	res, err := c.Execute(ctx, "SHOW SECRETS IN ACCOUNT")
	if err != nil {
		return nil, err
	}

	idxs := colIndexMap(res.Columns, "name", "database_name", "schema_name")
	var secrets []AccountSecret
	for _, row := range res.Rows {
		secrets = append(secrets, AccountSecret{
			Name:         strVal(row, idxs["name"]),
			DatabaseName: strVal(row, idxs["database_name"]),
			SchemaName:   strVal(row, idxs["schema_name"]),
		})
	}
	return secrets, nil
}

// GitRepoEntry represents a file or directory entry returned by listing
// a Snowflake git repository, internal stage, or workspace. The struct
// is generic (name, path, isDir, size) and reused across all location types.
type GitRepoEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size,omitempty"`
}

// StageEntry is a documentation-only alias for GitRepoEntry, used by
// ListStageEntries for readability. Go type aliases provide no compile-time
// distinction — a StageEntry is freely interchangeable with GitRepoEntry.
type StageEntry = GitRepoEntry

// WorkspaceEntry is a documentation-only alias for GitRepoEntry, used by
// ListWorkspaceEntries for readability. Same caveat as StageEntry.
type WorkspaceEntry = GitRepoEntry

// GitBranch represents a branch in a Snowflake git repository.
type GitBranch struct {
	Name string `json:"name"`
}

// GitTag represents a tag in a Snowflake git repository.
type GitTag struct {
	Name string `json:"name"`
}

// parseListEntries parses the result of a LIST @stage/path command into
// directory-aware GitRepoEntry values. stageName is the unquoted object name
// used to strip the stage/repo prefix from the NAME column. dirPath is the
// normalized directory prefix (with trailing slash when non-empty).
func parseListEntries(res *QueryResult, stageName, dirPath string) []GitRepoEntry {
	nameIdx := -1
	sizeIdx := -1
	for i, col := range res.Columns {
		switch strings.ToUpper(col) {
		case "NAME":
			nameIdx = i
		case "SIZE":
			sizeIdx = i
		}
	}
	if nameIdx == -1 {
		return []GitRepoEntry{}
	}

	seen := make(map[string]struct{})
	var entries []GitRepoEntry

	for _, row := range res.Rows {
		fullName := strVal(row, nameIdx)

		// The NAME column typically contains the stage/repo prefix, e.g.:
		// "MYREPO/branches/main/file.txt" or "DB.SCHEMA.MYREPO/branches/main/file.txt"
		// or even "@DB.SCHEMA.MYREPO/branches/main/file.txt".
		// We want the relative path: "branches/main/file.txt".
		relPath := fullName
		if slashIdx := strings.Index(fullName, "/"); slashIdx >= 0 {
			prefix := fullName[:slashIdx]
			up := strings.ToUpper(prefix)
			ur := strings.ToUpper(stageName)

			// Determine if the part before the first slash is a stage prefix.
			// It might be STAGE, "STAGE", @STAGE, or a qualified DB.SCHEMA.STAGE.
			isPrefix := up == ur ||
				up == `"`+ur+`"` ||
				strings.HasPrefix(up, "@") ||
				strings.HasSuffix(up, "."+ur) ||
				strings.HasSuffix(up, ".\""+ur+`"`)

			if isPrefix {
				relPath = fullName[slashIdx+1:]
			}
		}

		if !strings.HasPrefix(relPath, dirPath) {
			continue
		}
		rest := relPath[len(dirPath):]
		if rest == "" {
			continue
		}

		// Determine immediate child name and whether it is a directory.
		parts := strings.SplitN(rest, "/", 2)
		childName := parts[0]
		isDir := len(parts) > 1

		childPath := dirPath + childName
		if isDir {
			childPath += "/"
		}

		if _, dup := seen[childPath]; dup {
			continue
		}
		seen[childPath] = struct{}{}

		var size int64
		if sizeIdx != -1 && !isDir {
			if v, err2 := strconv.ParseInt(strVal(row, sizeIdx), 10, 64); err2 == nil {
				size = v
			}
		}

		entries = append(entries, GitRepoEntry{
			Name:  childName,
			Path:  childPath,
			IsDir: isDir,
			Size:  size,
		})
	}

	// Directories first, then files; each group sorted case-insensitively.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries
}

// normalizeDirPath normalizes a directory path for LIST queries: strips
// leading slash, ensures trailing slash when non-empty.
func normalizeDirPath(dirPath string) string {
	dirPath = strings.TrimPrefix(dirPath, "/")
	if dirPath != "" && !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}
	return dirPath
}

// ListGitRepoEntries returns the immediate children (files and directories) at
// dirPath within the git repository stage @database.schema.repoName/dirPath.
// Pass an empty dirPath to list the root. Directories are sorted first, then
// files; both groups are sorted case-insensitively by name.
func (c *Client) ListGitRepoEntries(ctx context.Context, database, schema, repoName, dirPath string) ([]GitRepoEntry, error) {
	dirPath = normalizeDirPath(dirPath)

	sql := fmt.Sprintf(`LIST @%s/%s`, Qualify(database, schema, repoName), dirPath)

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	return parseListEntries(res, repoName, dirPath), nil
}

// ListStageEntries returns the immediate children (files and directories) at
// dirPath within an internal named stage @database.schema.stageName/dirPath.
// Pass an empty dirPath to list the root. Reuses the same directory-aware
// parsing logic as ListGitRepoEntries.
func (c *Client) ListStageEntries(ctx context.Context, database, schema, stageName, dirPath string) ([]StageEntry, error) {
	dirPath = normalizeDirPath(dirPath)

	// The user stage (@~) is implicit and not database/schema-scoped, so it must
	// not be qualified; every other (named) stage is @database.schema.name.
	ref := Qualify(database, schema, stageName)
	if stageName == "~" {
		ref = "~"
	}
	sql := fmt.Sprintf(`LIST @%s/%s`, ref, dirPath)

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	return parseListEntries(res, stageName, dirPath), nil
}

// StageSummary is a single row from SHOW STAGES, carrying the fields needed to
// distinguish and reference a stage in pickers.
type StageSummary struct {
	Name string `json:"name"`
	Type string `json:"type"` // INTERNAL or EXTERNAL
	URL  string `json:"url"`  // external storage URL (empty for internal stages)
}

// ListStages runs SHOW STAGES IN SCHEMA and returns each stage with its
// INTERNAL/EXTERNAL type, so callers can filter (e.g. external tables may only
// reference an EXTERNAL stage).
func (c *Client) ListStages(ctx context.Context, database, schema string) ([]StageSummary, error) {
	sql := fmt.Sprintf("SHOW STAGES IN SCHEMA %s", Qualify(database, schema))
	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	// SHOW STAGES columns: created_on, name, database_name, schema_name, url,
	// has_credentials, has_encryption_key, owner, comment, region, type, cloud, …
	nameIdx := ColIdx(res.Columns, "name")
	typeIdx := ColIdx(res.Columns, "type")
	urlIdx := ColIdx(res.Columns, "url")

	stages := make([]StageSummary, 0, len(res.Rows))
	for _, row := range res.Rows {
		s := StageSummary{}
		s.Name = Cell(row, nameIdx)
		s.Type = strings.ToUpper(Cell(row, typeIdx))
		s.URL = Cell(row, urlIdx)
		if s.Name != "" {
			stages = append(stages, s)
		}
	}
	return stages, nil
}

// ListModels runs SHOW MODELS IN ACCOUNT and returns each model as a
// fully-qualified, quoted identifier (e.g. "DB"."SCHEMA"."NAME"), so a picker can
// offer every model the current role can see as a copy source for CREATE MODEL or
// ALTER MODEL … ADD VERSION. Models the role cannot access are simply absent.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	res, err := c.Execute(ctx, "SHOW MODELS IN ACCOUNT")
	if err != nil {
		return nil, err
	}

	// SHOW MODELS columns: created_on, name, model_type, database_name,
	// schema_name, owner, comment, versions, default_version_name, aliases.
	nameIdx := ColIdx(res.Columns, "name")
	dbIdx := ColIdx(res.Columns, "database_name")
	scIdx := ColIdx(res.Columns, "schema_name")

	out := make([]string, 0, len(res.Rows))
	for _, row := range res.Rows {
		name := Cell(row, nameIdx)
		if name == "" {
			continue
		}
		parts := make([]string, 0, 3)
		if db := Cell(row, dbIdx); db != "" {
			parts = append(parts, QuoteIdent(db))
		}
		if sc := Cell(row, scIdx); sc != "" {
			parts = append(parts, QuoteIdent(sc))
		}
		parts = append(parts, QuoteIdent(name))
		out = append(out, strings.Join(parts, "."))
	}
	return out, nil
}

// DbtProjectVersion represents a version of a Snowflake-native DBT PROJECT.
type DbtProjectVersion struct {
	Version   string `json:"version"`
	Alias     string `json:"alias"`
	IsDefault bool   `json:"isDefault"`
}

// ListDbtProjectVersions returns all versions of the given DBT PROJECT.
func (c *Client) ListDbtProjectVersions(ctx context.Context, database, schema, name string) ([]DbtProjectVersion, error) {
	sql := fmt.Sprintf(`SHOW VERSIONS IN DBT PROJECT %s`, Qualify(database, schema, name))

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	versionIdx := -1
	aliasIdx := -1
	defaultIdx := -1
	for i, col := range res.Columns {
		switch strings.ToUpper(col) {
		case "VERSION":
			versionIdx = i
		case "ALIAS":
			aliasIdx = i
		case "IS_DEFAULT", "DEFAULT":
			defaultIdx = i
		}
	}
	if versionIdx == -1 {
		return []DbtProjectVersion{}, nil
	}

	var versions []DbtProjectVersion
	for _, row := range res.Rows {
		v := DbtProjectVersion{
			Version: strVal(row, versionIdx),
		}
		if aliasIdx != -1 {
			v.Alias = strVal(row, aliasIdx)
		}
		if defaultIdx != -1 {
			d := strings.ToLower(strVal(row, defaultIdx))
			v.IsDefault = d == "true" || d == "1" || d == "yes" || d == "y"
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// WorkspaceInfo represents a Snowflake workspace. Workspaces are discovered via
// SHOW GIT REPOSITORIES (they appear as repos with REPOSITORY_ORIGIN = "WORKSPACE").
type WorkspaceInfo struct {
	Name     string `json:"name"`
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Owner    string `json:"owner"`
}

// ListWorkspaces returns all workspaces visible to the current user.
// Workspaces appear as git repositories whose name follows the pattern
// <USER>$.<SCHEMA>."<workspace_name>". We filter by checking the
// repository_origin column for "WORKSPACE" or by name pattern.
//
// TODO: revisit once Snowflake provides a dedicated SHOW WORKSPACES command
// or supports WHERE/LIKE filtering for REPOSITORY_ORIGIN. The current approach
// fetches all git repos account-wide, which may be slow on large accounts.
func (c *Client) ListWorkspaces(ctx context.Context) ([]WorkspaceInfo, error) {
	res, err := c.Execute(ctx, "SHOW GIT REPOSITORIES IN ACCOUNT")
	if err != nil {
		return nil, err
	}

	nameIdx := -1
	dbIdx := -1
	schemaIdx := -1
	ownerIdx := -1
	originIdx := -1
	for i, col := range res.Columns {
		switch strings.ToUpper(col) {
		case "NAME":
			nameIdx = i
		case "DATABASE_NAME":
			dbIdx = i
		case "SCHEMA_NAME":
			schemaIdx = i
		case "OWNER":
			ownerIdx = i
		case "REPOSITORY_ORIGIN":
			originIdx = i
		}
	}
	if nameIdx == -1 {
		return []WorkspaceInfo{}, nil
	}

	var workspaces []WorkspaceInfo
	for _, row := range res.Rows {
		// Filter: prefer the REPOSITORY_ORIGIN column ("WORKSPACE") which
		// is present on modern Snowflake versions. Fall back to the "$" name
		// heuristic only when the column is missing from the response (older
		// versions). Note: the "$" heuristic can false-positive on repos
		// that happen to contain "$" in their name.
		origin := ""
		if originIdx != -1 {
			origin = strings.ToUpper(strVal(row, originIdx))
		}
		name := strVal(row, nameIdx)
		if originIdx != -1 {
			// Column available — use authoritative check only.
			if origin != "WORKSPACE" {
				continue
			}
		} else {
			// Column unavailable — fall back to name heuristic.
			if !strings.Contains(name, "$") {
				continue
			}
		}
		w := WorkspaceInfo{Name: name}
		if dbIdx != -1 {
			w.Database = strVal(row, dbIdx)
		}
		if schemaIdx != -1 {
			w.Schema = strVal(row, schemaIdx)
		}
		if ownerIdx != -1 {
			w.Owner = strVal(row, ownerIdx)
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, nil
}

// ListWorkspaceEntries returns directory-aware entries within a workspace.
// The workspace is addressed as a git repository stage: @db.schema."workspace_name"/dirPath.
func (c *Client) ListWorkspaceEntries(ctx context.Context, database, schema, workspaceName, dirPath string) ([]WorkspaceEntry, error) {
	dirPath = normalizeDirPath(dirPath)

	sql := fmt.Sprintf(`LIST @%s/%s`, Qualify(database, schema, workspaceName), dirPath)

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	return parseListEntries(res, workspaceName, dirPath), nil
}

// ListGitBranches returns all branches in the given git repository.
func (c *Client) ListGitBranches(ctx context.Context, database, schema, repoName string) ([]GitBranch, error) {
	sql := fmt.Sprintf(`SHOW GIT BRANCHES IN GIT REPOSITORY %s`, Qualify(database, schema, repoName))

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	nameIdx := -1
	for i, col := range res.Columns {
		if strings.ToUpper(col) == "NAME" {
			nameIdx = i
			break
		}
	}
	if nameIdx == -1 {
		return nil, fmt.Errorf("NAME column not found in SHOW GIT BRANCHES output")
	}

	var branches []GitBranch
	for _, row := range res.Rows {
		branches = append(branches, GitBranch{
			Name: strVal(row, nameIdx),
		})
	}
	return branches, nil
}

// ListGitTags returns all tags in the given git repository.
func (c *Client) ListGitTags(ctx context.Context, database, schema, repoName string) ([]GitTag, error) {
	sql := fmt.Sprintf(`SHOW GIT TAGS IN GIT REPOSITORY %s`, Qualify(database, schema, repoName))

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	nameIdx := -1
	for i, col := range res.Columns {
		if strings.ToUpper(col) == "NAME" {
			nameIdx = i
			break
		}
	}
	if nameIdx == -1 {
		return nil, fmt.Errorf("NAME column not found in SHOW GIT TAGS output")
	}

	var tags []GitTag
	for _, row := range res.Rows {
		tags = append(tags, GitTag{
			Name: strVal(row, nameIdx),
		})
	}
	return tags, nil
}

// GetGitFileContent reads a file from a git repository and returns its content.
func (c *Client) GetGitFileContent(ctx context.Context, database, schema, repoName, filePath string) (string, error) {
	sql := fmt.Sprintf(`SELECT $1 FROM @%s/%s`, Qualify(database, schema, repoName), filePath)

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return "", err
	}

	var content strings.Builder
	for _, row := range res.Rows {
		if len(row) > 0 {
			content.WriteString(strVal(row, 0))
			content.WriteString("\n")
		}
	}
	return content.String(), nil
}

// ExecuteGitFile executes a SQL file via EXECUTE IMMEDIATE FROM @db.schema.name/path.
// Also used for stage files (via App.ExecuteStageFile) since the SQL pattern is identical.
func (c *Client) ExecuteGitFile(ctx context.Context, database, schema, repoName, filePath string) error {
	sql := fmt.Sprintf(`EXECUTE IMMEDIATE FROM @%s/%s`, Qualify(database, schema, repoName), filePath)

	_, err := c.Execute(ctx, sql)
	return err
}

// IntegrationRow holds metadata returned by SHOW <kind> INTEGRATIONS.
type IntegrationRow struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
	Comment  string `json:"comment"`
}

// ListIntegrations runs SHOW <kind> INTEGRATIONS and returns the result rows.
// kind may be "STORAGE", "API", "CATALOG", "EXTERNAL ACCESS", "NOTIFICATION", or "SECURITY".
// Column layouts differ by integration type; we use column names for resilience.
func (c *Client) ListIntegrations(ctx context.Context, kind string) ([]IntegrationRow, error) {
	upper := strings.ToUpper(strings.TrimSpace(kind))
	validKinds := map[string]bool{
		"STORAGE": true, "API": true, "CATALOG": true,
		"EXTERNAL ACCESS": true, "NOTIFICATION": true, "SECURITY": true,
	}
	if !validKinds[upper] {
		return nil, fmt.Errorf("unknown integration kind: %q", kind)
	}
	rows, err := c.queryCtx(ctx, fmt.Sprintf("SHOW %s INTEGRATIONS", upper))
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	idx := colIndexMap(cols, "name", "type", "category", "enabled", "comment")

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

	result := []IntegrationRow{}
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		get := func(key string) string {
			i := idx[key]
			if i < 0 || i >= len(vals) {
				return ""
			}
			return toString(vals[i])
		}
		enabledStr := strings.ToLower(get("enabled"))
		result = append(result, IntegrationRow{
			Name:     get("name"),
			Type:     get("type"),
			Category: get("category"),
			Enabled:  enabledStr == "true" || enabledStr == "1" || enabledStr == "yes",
			Comment:  get("comment"),
		})
	}
	return result, rows.Err()
}

// DropIntegration drops the named integration.
func (c *Client) DropIntegration(ctx context.Context, name string) error {
	_, err := c.execCtx(ctx, dropIntegrationStmt(name))
	return err
}

// DropDatabase drops a database. mode should be "CASCADE" or "RESTRICT"; any
// other value defaults to CASCADE (see normalizeDropMode).
func (c *Client) DropDatabase(ctx context.Context, name string, mode string) error {
	_, err := c.execCtx(ctx, dropDatabaseStmt(name, mode))
	return err
}

// DropSchema drops a schema. mode should be "CASCADE" or "RESTRICT"; any other
// value defaults to CASCADE (see normalizeDropMode).
func (c *Client) DropSchema(ctx context.Context, database, schema string, mode string) error {
	_, err := c.execCtx(ctx, dropSchemaStmt(database, schema, mode))
	return err
}

// ExecDDL executes a pre-built DDL statement (e.g. CREATE INTEGRATION …).
// The caller is responsible for ensuring the SQL is safe; use the integrations
// package helpers to build injection-safe DDL before calling this method.
func (c *Client) ExecDDL(ctx context.Context, sql string) error {
	_, err := c.execCtx(ctx, sql)
	return err
}

// CanCreateIntegration returns (true, nil) when the given role (or any role it
// inherits) allows creating integrations. If role is empty, the connector's
// current role is used as a fallback.
func (c *Client) CanCreateIntegration(ctx context.Context, role string) (bool, error) {
	if role == "" {
		c.connector.mu.RLock()
		role = c.connector.role
		c.connector.mu.RUnlock()
	}

	if role == "" {
		if err := c.queryRowCtx(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
			return false, fmt.Errorf("CanCreateIntegration: %w", err)
		}
		role = strings.TrimSpace(role)
	}

	return c.walkRoleHierarchy(ctx, role,
		map[string]bool{"CREATE INTEGRATION": true},
	)
}

// GetUserDDL constructs a CREATE USER DDL statement for the given user by
// running DESCRIBE USER and translating the property/value pairs.
func (c *Client) GetUserDDL(ctx context.Context, name string) (string, error) {

	rows, err := c.queryCtx(ctx, fmt.Sprintf(`DESCRIBE USER %s`, QuoteIdent(name)))
	if err != nil {
		return "", err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "property", "value")

	props := map[string]string{}
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if rows.Scan(ptrs...) != nil {
			continue
		}
		prop := strings.ToUpper(strVal(vals, idxs["property"]))
		val := strVal(vals, idxs["value"])
		if val != "" && val != "null" {
			props[prop] = val
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	var lines []string

	addStr := func(prop, key string) {
		if v, ok := props[key]; ok {
			lines = append(lines, fmt.Sprintf("    %s = %s", prop, QuoteStringLit(v)))
		}
	}
	addIdent := func(prop, key string) {
		if v, ok := props[key]; ok {
			lines = append(lines, fmt.Sprintf("    %s = %s", prop, v))
		}
	}
	addBool := func(prop, key string) {
		if v, ok := props[key]; ok {
			upper := strings.ToUpper(v)
			if upper == "TRUE" || upper == "FALSE" {
				lines = append(lines, fmt.Sprintf("    %s = %s", prop, upper))
			}
		}
	}

	// Only emit LOGIN_NAME when it differs from the username.
	if v, ok := props["LOGIN_NAME"]; ok && !strings.EqualFold(v, name) {
		lines = append(lines, fmt.Sprintf("    LOGIN_NAME = %s", QuoteStringLit(v)))
	}
	addStr("DISPLAY_NAME", "DISPLAY_NAME")
	addStr("FIRST_NAME", "FIRST_NAME")
	addStr("LAST_NAME", "LAST_NAME")
	addStr("EMAIL", "EMAIL")
	addIdent("DEFAULT_WAREHOUSE", "DEFAULT_WAREHOUSE")
	addIdent("DEFAULT_ROLE", "DEFAULT_ROLE")
	addIdent("DEFAULT_NAMESPACE", "DEFAULT_NAMESPACE")
	addStr("COMMENT", "COMMENT")
	addBool("MUST_CHANGE_PASSWORD", "MUST_CHANGE_PASSWORD")
	if v, ok := props["DAYS_TO_EXPIRY"]; ok {
		if n, err2 := strconv.Atoi(v); err2 == nil && n > 0 {
			lines = append(lines, fmt.Sprintf("    DAYS_TO_EXPIRY = %d", n))
		}
	}
	addBool("DISABLED", "DISABLED")

	sql := fmt.Sprintf("CREATE USER \"%s\"", QuoteIdent(name))
	if len(lines) > 0 {
		sql += "\n" + strings.Join(lines, "\n")
	}
	return sql + ";", nil
}

// userAdminRole returns true for Snowflake's built-in roles that have implicit
// user-management privileges even when SHOW GRANTS TO ROLE cannot be queried.
func userAdminRole(upper string) bool {
	switch upper {
	case "ACCOUNTADMIN", "SECURITYADMIN", "USERADMIN":
		return true
	}
	return false
}

// roleGrantsPrivilege checks SHOW GRANTS TO ROLE for direct account-level
// privileges and collects inherited role names for further traversal.
// Returns (found, inheritedRoles, error).
func (c *Client) roleGrantsPrivilege(
	ctx context.Context,
	role string,
	acceptedPrivs map[string]bool,
) (bool, []string, error) {
	rows, err := c.queryCtx(ctx, showGrantsToRoleStmt(role))
	if err != nil {
		return false, nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "privilege", "granted_on", "name")

	var inherited []string
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if rows.Scan(ptrs...) != nil {
			continue
		}
		priv := strings.ToUpper(strVal(vals, idxs["privilege"]))
		on := strings.ToUpper(strVal(vals, idxs["granted_on"]))
		name := strVal(vals, idxs["name"])

		if on == "ACCOUNT" && acceptedPrivs[priv] {
			return true, nil, nil
		}
		if on == "ROLE" && name != "" {
			inherited = append(inherited, name)
		}
	}
	return false, inherited, rows.Err()
}

// walkRoleHierarchy traverses the role hierarchy breadth-first looking for
// account-level privileges or a known system role with implicit capabilities.
func (c *Client) walkRoleHierarchy(
	ctx context.Context,
	startRole string,
	acceptedPrivs map[string]bool,
) (bool, error) {
	seen := map[string]bool{}
	queue := []string{startRole}

	for len(queue) > 0 && len(seen) <= 20 {
		role := queue[0]
		queue = queue[1:]

		upper := strings.ToUpper(strings.TrimSpace(role))
		if seen[upper] {
			continue
		}
		seen[upper] = true

		// Fast-path: system roles with implicit user-management privileges.
		if userAdminRole(upper) {
			return true, nil
		}

		found, inherited, err := c.roleGrantsPrivilege(ctx, role, acceptedPrivs)
		if err != nil {
			// SHOW GRANTS may be restricted for this role; skip it.
			continue
		}
		if found {
			return true, nil
		}
		queue = append(queue, inherited...)
	}
	return false, nil
}

// collectRoleHierarchy returns the set of all role names (uppercase) reachable
// from startRole, including startRole itself. Capped at 30 hops to prevent cycles.
func (c *Client) collectRoleHierarchy(ctx context.Context, startRole string) (map[string]bool, error) {
	seen := map[string]bool{}
	queue := []string{startRole}
	for len(queue) > 0 && len(seen) <= 30 {
		role := queue[0]
		queue = queue[1:]
		upper := strings.ToUpper(strings.TrimSpace(role))
		if seen[upper] {
			continue
		}
		seen[upper] = true
		rows, err := c.queryCtx(ctx, showGrantsToRoleStmt(role))
		if err != nil {
			continue // restricted; skip this role
		}
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "granted_on", "name")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if rows.Scan(ptrs...) != nil {
				continue
			}
			if strings.ToUpper(strVal(vals, idxs["granted_on"])) == "ROLE" {
				if name := strVal(vals, idxs["name"]); name != "" {
					queue = append(queue, name)
				}
			}
		}
		rows.Close() //nolint:errcheck
	}
	return seen, nil
}

// CanModifyUserAuth returns true when the current session role (or any role it
// inherits) holds OWNERSHIP or MODIFY PROGRAMMATIC AUTHENTICATION METHODS on
// the named user. System admin roles always have authority.
func (c *Client) CanModifyUserAuth(ctx context.Context, username string) (bool, error) {
	c.connector.mu.RLock()
	role := c.connector.role
	c.connector.mu.RUnlock()
	if role == "" {
		if err := c.queryRowCtx(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
			return false, fmt.Errorf("CanModifyUserAuth: %w", err)
		}
		role = strings.TrimSpace(role)
	}

	// Fast-path: built-in admin roles have implicit authority.
	if userAdminRole(strings.ToUpper(role)) {
		return true, nil
	}

	// Collect the full role hierarchy.
	hierarchy, err := c.collectRoleHierarchy(ctx, role)
	if err != nil {
		return false, err
	}
	for r := range hierarchy {
		if userAdminRole(r) {
			return true, nil
		}
	}

	// Check object-level grants on this specific user.
	rows, err := c.queryCtx(ctx, showGrantsOnUserStmt(username))
	if err != nil {
		return false, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "privilege", "grantee_name")
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if rows.Scan(ptrs...) != nil {
			continue
		}
		priv := strings.ToUpper(strVal(vals, idxs["privilege"]))
		grantee := strings.ToUpper(strings.TrimSpace(strVal(vals, idxs["grantee_name"])))
		if hierarchy[grantee] &&
			(priv == "OWNERSHIP" || priv == "MODIFY PROGRAMMATIC AUTHENTICATION METHODS") {
			return true, nil
		}
	}
	return false, rows.Err()
}

// CanCreateUsers returns (true, nil) when the given role (or any role it
// inherits) allows creating users. If role is empty, the connector's current
// role is used as a fallback.
func (c *Client) CanCreateUsers(ctx context.Context, role string) (bool, error) {
	if role == "" {
		c.connector.mu.RLock()
		role = c.connector.role
		c.connector.mu.RUnlock()
	}

	if role == "" {
		// Fallback: ask the DB directly (e.g. before first UseRole call).
		if err := c.queryRowCtx(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
			return false, fmt.Errorf("CanCreateUsers: %w", err)
		}
		role = strings.TrimSpace(role)
	}

	return c.walkRoleHierarchy(ctx, role,
		map[string]bool{"CREATE USER": true, "MANAGE GRANTS": true},
	)
}

// CanManageUsers returns (true, nil) when the given role (or any role it
// inherits) can ALTER or DROP other users. If role is empty, the connector's
// current role is used as a fallback.
func (c *Client) CanManageUsers(ctx context.Context, role string) (bool, error) {
	if role == "" {
		c.connector.mu.RLock()
		role = c.connector.role
		c.connector.mu.RUnlock()
	}

	if role == "" {
		if err := c.queryRowCtx(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
			return false, fmt.Errorf("CanManageUsers: %w", err)
		}
		role = strings.TrimSpace(role)
	}

	return c.walkRoleHierarchy(ctx, role,
		map[string]bool{"MANAGE GRANTS": true},
	)
}

// GetRoleDDL constructs the DDL for a single role from SHOW commands.
// Snowflake does not support GET_DDL for roles, so we build the output from:
//   - SHOW ROLES LIKE '<name>'       → CREATE ROLE with optional comment
//   - SHOW GRANTS TO ROLE "<name>"   → GRANT <priv> ON … TO ROLE statements
//   - SHOW GRANTS ON ROLE "<name>"   → GRANT ROLE … TO ROLE/USER statements
func (c *Client) GetRoleDDL(ctx context.Context, name string) (string, error) {
	escapedLike := EscapeStringLit(name)
	quotedIdent := QuoteIdent(name)

	// ── Comment from SHOW ROLES LIKE ────────────────────────────────────────
	var comment string
	if rows, err := c.queryCtx(ctx,
		fmt.Sprintf("SHOW ROLES LIKE '%s'", escapedLike)); err == nil {
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "comment")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if rows.Scan(ptrs...) == nil {
				v := strVal(vals, idxs["comment"])
				if v != "" {
					comment = v
				}
				break
			}
		}
		rows.Close() //nolint:errcheck
	}

	// ── CREATE ROLE ──────────────────────────────────────────────────────────
	var sb strings.Builder

	// Snowflake system roles have grants on internal object types (CLASS,
	// APPLICATION_ROLE, DATABASE_ROLE, IMAGE_REPOSITORY, etc.) that cannot
	// be recreated with standard SQL. Emit a warning header for these roles.
	if isSystemRole(name) {
		sb.WriteString("-- WARNING: This is a Snowflake system role. The DDL below is for\n")
		sb.WriteString("-- informational purposes only and may contain invalid syntax that\n")
		sb.WriteString("-- cannot be executed. Do not run this script.\n\n")
	}

	sb.WriteString(fmt.Sprintf("CREATE ROLE IF NOT EXISTS %s", quotedIdent))
	if comment != "" {
		sb.WriteString(fmt.Sprintf("\n  COMMENT = '%s'",
			EscapeStringLit(comment)))
	}
	sb.WriteString(";\n")

	// ── SHOW GRANTS TO ROLE → privileges granted to this role ────────────────
	if rows, err := c.queryCtx(ctx, showGrantsToRoleStmt(name)); err == nil {
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "privilege", "granted_on", "name", "grant_option")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if rows.Scan(ptrs...) != nil {
				continue
			}
			priv := strVal(vals, idxs["privilege"])
			onType := strVal(vals, idxs["granted_on"])
			obj := strVal(vals, idxs["name"])
			opt := strings.EqualFold(strVal(vals, idxs["grant_option"]), "true")
			if priv == "" || onType == "" {
				continue
			}
			sb.WriteString(FormatRoleGrant(priv, onType, obj, name, opt) + "\n")
		}
		rows.Close() //nolint:errcheck
	}

	// ── SHOW GRANTS ON ROLE → who this role is granted to ────────────────────
	if rows, err := c.queryCtx(ctx, showGrantsOnRoleStmt(name)); err == nil {
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "granted_to", "grantee_name")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if rows.Scan(ptrs...) != nil {
				continue
			}
			grantedTo := strVal(vals, idxs["granted_to"])
			grantee := strVal(vals, idxs["grantee_name"])
			if grantedTo == "" || grantee == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("GRANT ROLE %s TO %s %s;\n",
				quotedIdent, grantedTo, QuoteIdent(grantee)))
		}
		rows.Close() //nolint:errcheck
	}

	return sb.String(), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// FormatRoleGrant builds a single GRANT statement line for a role DDL export.
// role is the raw role name and is double-quoted via QuoteIdent; obj is the raw
// object name only for the ON ROLE case (also quoted) and otherwise an
// already-qualified reference inserted verbatim. Special cases handled:
//   - ON ACCOUNT: object name is omitted (Snowflake requires bare ON ACCOUNT)
//   - USAGE ON ROLE: converted to GRANT ROLE ... TO ROLE ... (the executable
//     form of role membership); WITH GRANT OPTION is dropped because
//     GRANT ROLE ... WITH GRANT OPTION is not valid Snowflake syntax
func FormatRoleGrant(priv, onType, obj, role string, withGrantOption bool) string {
	quotedRole := QuoteIdent(role)
	var stmt string
	switch {
	case strings.EqualFold(onType, "ROLE"):
		// Quote the child role name — SHOW GRANTS returns bare identifiers even
		// for mixed-case roles (e.g. "My_Role" → My_Role in the name column).
		quotedChild := QuoteIdent(obj)
		if strings.EqualFold(priv, "USAGE") {
			// USAGE on ROLE is Snowflake's internal representation of role membership.
			// The executable form is GRANT ROLE <name> TO ROLE <parent>.
			// WITH GRANT OPTION is not valid for GRANT ROLE statements.
			return fmt.Sprintf("GRANT ROLE %s TO ROLE %s;", quotedChild, quotedRole)
		}
		stmt = fmt.Sprintf("GRANT %s ON ROLE %s TO ROLE %s", priv, quotedChild, quotedRole)
	case strings.EqualFold(onType, "ACCOUNT"):
		stmt = fmt.Sprintf("GRANT %s ON ACCOUNT TO ROLE %s", priv, quotedRole)
	default:
		stmt = fmt.Sprintf("GRANT %s ON %s %s TO ROLE %s", priv, onType, obj, quotedRole)
	}
	if withGrantOption {
		stmt += " WITH GRANT OPTION"
	}
	return stmt + ";"
}

// isSystemRole returns true for Snowflake built-in system roles whose DDL
// cannot be faithfully represented with standard SQL.
func isSystemRole(name string) bool {
	switch strings.ToUpper(name) {
	case "ACCOUNTADMIN", "SYSADMIN", "SECURITYADMIN", "USERADMIN", "ORGADMIN", "PUBLIC":
		return true
	}
	return false
}

// colIndexMap returns a map of (lowercase column name → column index)
// for the requested column names. Unknown columns map to -1.
func colIndexMap(cols []string, names ...string) map[string]int {
	result := make(map[string]int, len(names))
	for _, n := range names {
		result[n] = -1
	}
	for i, col := range cols {
		lower := strings.ToLower(col)
		if _, ok := result[lower]; ok {
			result[lower] = i
		}
	}
	return result
}

// makeValPtrs allocates n interface{} values and returns both the values
// slice and a parallel slice of pointers suitable for rows.Scan.
func makeValPtrs(n int) ([]interface{}, []interface{}) {
	vals := make([]interface{}, n)
	ptrs := make([]interface{}, n)
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	return vals, ptrs
}

// strVal returns the string representation of vals[i], or "" if i < 0 or nil.
func strVal(vals []interface{}, i int) string {
	if i < 0 || i >= len(vals) {
		return ""
	}
	s := fmt.Sprintf("%v", vals[i])
	if s == "<nil>" {
		return ""
	}
	return strings.TrimSpace(s)
}

// GetWarehouseDDL returns the DDL for a single warehouse using Snowflake's GET_DDL function.
func (c *Client) GetWarehouseDDL(ctx context.Context, name string) (string, error) {
	return c.GetObjectDDL(ctx, "", "", "WAREHOUSE", name, "")
}

// UseRole switches the active role for the current session.
func (c *Client) UseRole(ctx context.Context, role string) error {
	if _, err := c.execCtx(ctx, fmt.Sprintf(`USE ROLE %s`, QuoteIdent(role))); err != nil {
		return err
	}
	c.connector.mu.Lock()
	c.connector.role = role
	c.connector.mu.Unlock()
	// Flush idle connections so the next query gets a fresh connection that
	// goes through sessionConnector.Connect and inherits the new role.
	c.db.SetMaxIdleConns(0)
	c.db.SetMaxIdleConns(8)
	return nil
}

// UseWarehouse switches the active warehouse for the current session.
func (c *Client) UseWarehouse(ctx context.Context, warehouse string) error {
	if _, err := c.execCtx(ctx, fmt.Sprintf(`USE WAREHOUSE %s`, QuoteIdent(warehouse))); err != nil {
		return err
	}
	c.connector.mu.Lock()
	c.connector.wh = warehouse
	c.connector.mu.Unlock()
	// Flush idle connections so the next query gets a fresh connection that
	// goes through sessionConnector.Connect and inherits the new warehouse.
	c.db.SetMaxIdleConns(0)
	c.db.SetMaxIdleConns(8)
	return nil
}

// UseDatabase switches the active database for the current session.
// Switching the database also resets the active schema.
func (c *Client) UseDatabase(ctx context.Context, database string) error {
	if _, err := c.execCtx(ctx, fmt.Sprintf(`USE DATABASE %s`, QuoteIdent(database))); err != nil {
		return err
	}
	c.connector.mu.Lock()
	c.connector.db = database
	c.connector.sc = "" // schema context resets on database change
	c.connector.mu.Unlock()
	c.db.SetMaxIdleConns(0)
	c.db.SetMaxIdleConns(8)
	return nil
}

// UseSchema switches the active schema for the current session.
// The schema is always qualified with the current database (e.g.
// USE SCHEMA "db"."schema") so that stale pool connections that still carry
// an old database context execute the command in the correct database.
func (c *Client) UseSchema(ctx context.Context, schema string) error {
	c.connector.mu.RLock()
	db := c.connector.db
	c.connector.mu.RUnlock()

	var query string
	if db != "" {
		query = fmt.Sprintf(`USE SCHEMA %s`, Qualify(db, schema))
	} else {
		query = fmt.Sprintf(`USE SCHEMA %s`, QuoteIdent(schema))
	}

	if _, err := c.execCtx(ctx, query); err != nil {
		return err
	}
	c.connector.mu.Lock()
	c.connector.sc = schema
	c.connector.mu.Unlock()
	c.db.SetMaxIdleConns(0)
	c.db.SetMaxIdleConns(8)
	return nil
}

// ListDatabases returns all databases the user can see.
func (c *Client) ListDatabases(ctx context.Context) ([]string, error) {
	return c.queryStringSlice(ctx, "SHOW DATABASES", 1)
}

// ListExportableDatabases returns only databases that are owned by the account
// (origin column is empty). Shared / imported databases such as
// SNOWFLAKE_SAMPLE_DATA are excluded because GET_DDL is not supported on them.
func (c *Client) ListExportableDatabases(ctx context.Context) ([]string, error) {
	rows, err := c.queryCtx(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	// Locate the "name" and "origin" columns by header name.
	nameIdx, originIdx := 1, 4
	for i, col := range cols {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "origin":
			originIdx = i
		}
	}

	result := []string{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		origin := strings.TrimSpace(fmt.Sprintf("%v", vals[originIdx]))
		if origin == "" || origin == "<nil>" {
			result = append(result, fmt.Sprintf("%v", vals[nameIdx]))
		}
	}
	return result, rows.Err()
}

// ListSchemas returns schemas inside a database.
func (c *Client) ListSchemas(ctx context.Context, database string) ([]string, error) {
	return c.queryStringSlice(ctx, fmt.Sprintf(`SHOW SCHEMAS IN DATABASE %s`, QuoteIdent(database)), 1)
}

// ListUserSchemas returns the user-managed schemas inside a database — i.e.
// ListSchemas minus the per-database INFORMATION_SCHEMA, whose objects are
// read-only system views that cannot be created, altered, dropped, or tagged.
// Use this wherever the UI offers schemas as targets for object / DDL /
// governance operations rather than as a raw catalog listing.
func (c *Client) ListUserSchemas(ctx context.Context, database string) ([]string, error) {
	schemas, err := c.ListSchemas(ctx, database)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(schemas))
	for _, s := range schemas {
		if strings.EqualFold(s, "INFORMATION_SCHEMA") {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered, nil
}

// GetObjectTableType returns the TABLE_TYPE of a relational object from
// <database>.INFORMATION_SCHEMA.TABLES — e.g. "BASE TABLE", "VIEW",
// "MATERIALIZED VIEW", "EXTERNAL TABLE". It is used to distinguish a view from a
// table when an operation needs the right ALTER keyword (notably column tagging,
// where a view column requires ALTER VIEW … ALTER COLUMN). Returns "" when the
// object isn't found in INFORMATION_SCHEMA.TABLES.
func (c *Client) GetObjectTableType(ctx context.Context, database, schema, name string) (string, error) {
	query := fmt.Sprintf(
		"SELECT TABLE_TYPE FROM %s.INFORMATION_SCHEMA.TABLES "+
			"WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'",
		QuoteIdent(database), EscapeStringLit(schema), EscapeStringLit(name))
	vals, err := c.queryStringSlice(ctx, query, 0)
	if err != nil {
		return "", err
	}
	if len(vals) == 0 {
		return "", nil
	}
	return vals[0], nil
}

// extractArgTypes parses the "arguments" column returned by SHOW PROCEDURES /
// SHOW FUNCTIONS / SHOW DATA METRIC FUNCTIONS. The format is
// "<name>(<types>) RETURN <return_type>", e.g.
// "GET_EMPLOYEE_STATUS(NUMBER) RETURN VARIANT". Returns just the types string,
// e.g. "NUMBER", or an empty string when there are no parameters.
//
// The argument list of the outermost parentheses is matched by paren depth so
// that nested type expressions survive intact — data metric functions take a
// TABLE argument whose type is itself parenthesized (e.g.
// "MY_DMF(TABLE(NUMBER)) RETURN NUMBER" → "TABLE(NUMBER)"). A naive first-")"
// scan would truncate that to "TABLE(NUMBER".
func extractArgTypes(arguments string) string {
	start := strings.Index(arguments, "(")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(arguments); i++ {
		switch arguments[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return strings.TrimSpace(arguments[start+1 : i])
			}
		}
	}
	return strings.TrimSpace(arguments[start+1:])
}

// ExtractArgTypes is the exported form of extractArgTypes: it parses the
// parameter type list out of a SHOW FUNCTIONS / SHOW PROCEDURES "arguments" cell
// (e.g. "FOO(NUMBER, VARCHAR) RETURN NUMBER" → "NUMBER, VARCHAR"). It is used to
// match a specific overload's SHOW row against a threaded argument signature.
func ExtractArgTypes(arguments string) string {
	return extractArgTypes(arguments)
}

// DroppedTable represents a table that has been dropped but is still within
// the Snowflake Time Travel retention window and can be recovered with UNDROP.
type DroppedTable struct {
	Name      string `json:"name"`
	DroppedOn string `json:"droppedOn"`
}

// ListDroppedTables returns tables in the given schema that have been dropped
// but are still recoverable via Time Travel. It runs SHOW TABLES HISTORY and
// returns only rows where dropped_on is non-empty.
func (c *Client) ListDroppedTables(ctx context.Context, database, schema string) ([]DroppedTable, error) {
	return c.listDroppedHistory(ctx,
		fmt.Sprintf(`SHOW TABLES HISTORY IN SCHEMA %s`, Qualify(database, schema)))
}

// listDroppedHistory is the shared helper for SHOW * HISTORY queries.
// It reads any result set that has "name" and "dropped_on" columns and returns
// only the rows where dropped_on is non-empty (i.e. the object is dropped).
func (c *Client) listDroppedHistory(ctx context.Context, query string) ([]DroppedTable, error) {
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "name", "dropped_on")
	if idxs["name"] < 0 {
		return nil, fmt.Errorf("no 'name' column in result: %s", query)
	}

	result := []DroppedTable{}
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		droppedOn := strVal(vals, idxs["dropped_on"])
		if droppedOn == "" {
			continue
		}
		result = append(result, DroppedTable{
			Name:      strVal(vals, idxs["name"]),
			DroppedOn: droppedOn,
		})
	}
	return result, rows.Err()
}

// ListDroppedSchemas returns schemas in the given database that have been
// dropped but are still within the Time Travel retention window.
func (c *Client) ListDroppedSchemas(ctx context.Context, database string) ([]DroppedTable, error) {
	return c.listDroppedHistory(ctx, showSchemasHistoryStmt(database))
}

// ListDroppedDatabases returns all databases that have been dropped but are
// still within the Time Travel retention window.
func (c *Client) ListDroppedDatabases(ctx context.Context) ([]DroppedTable, error) {
	return c.listDroppedHistory(ctx, `SHOW DATABASES HISTORY`)
}

// GetDatabaseRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given database. Returns 1 if the value cannot be determined.
func (c *Client) GetDatabaseRetentionDays(ctx context.Context, dbName string) (int, error) {
	query := fmt.Sprintf(`SHOW PARAMETERS LIKE 'DATA_RETENTION_TIME_IN_DAYS' IN DATABASE %s`, QuoteIdent(dbName))
	return c.queryIntCell(ctx, query, "value", 1)
}

// GetSchemaRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given schema. Returns 1 if the value cannot be determined.
func (c *Client) GetSchemaRetentionDays(ctx context.Context, database, schema string) (int, error) {
	query := fmt.Sprintf(`SHOW PARAMETERS LIKE 'DATA_RETENTION_TIME_IN_DAYS' IN SCHEMA %s`, Qualify(database, schema))
	return c.queryIntCell(ctx, query, "value", 1)
}

// GetTableRetentionDays returns the Time Travel data-retention period in days
// for a single table. It runs SHOW TABLES LIKE and reads the retention_time
// column. Returns 1 (Snowflake's Standard-edition default) when the value
// cannot be determined.
func (c *Client) GetTableRetentionDays(ctx context.Context, database, schema, name string) (int, error) {
	query := fmt.Sprintf(`SHOW TABLES LIKE '%s' IN SCHEMA %s`,
		EscapeStringLit(name), Qualify(database, schema))
	return c.queryIntCell(ctx, query, "retention_time", 1)
}

// queryIntCell runs query and parses the named column of the first result row as
// an int. It returns def when the query returns no rows, the cell is empty, or
// the value is non-numeric; a query or scan error is returned alongside def. It
// is the shared reader behind the Get*RetentionDays helpers.
func (c *Client) queryIntCell(ctx context.Context, query, col string, def int) (int, error) {
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return def, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, col)
	if rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return def, err
		}
		if s := strVal(vals, idxs[col]); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				return n, nil
			}
		}
	}
	return def, nil
}

// SnowflakeUser holds the key properties of a Snowflake user account returned
// by SHOW USERS.
type SnowflakeUser struct {
	Name               string `json:"name"`
	LoginName          string `json:"loginName"`
	DisplayName        string `json:"displayName"`
	FirstName          string `json:"firstName"`
	LastName           string `json:"lastName"`
	Email              string `json:"email"`
	DefaultWarehouse   string `json:"defaultWarehouse"`
	DefaultRole        string `json:"defaultRole"`
	DefaultNamespace   string `json:"defaultNamespace"`
	Comment            string `json:"comment"`
	Disabled           bool   `json:"disabled"`
	MustChangePassword bool   `json:"mustChangePassword"`
	DaysToExpiry       string `json:"daysToExpiry"`
	Owner              string `json:"owner"`
	LastSuccessLogin   string `json:"lastSuccessLogin"`
}

// ListUsers returns all users visible to the current role via SHOW USERS.
// Returns an error (e.g. insufficient privileges) that the caller should
// treat as "user management not available".
func (c *Client) ListUsers(ctx context.Context) ([]SnowflakeUser, error) {
	rows, err := c.queryCtx(ctx, "SHOW USERS")
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	col := func(name string) int {
		for i, c := range cols {
			if strings.EqualFold(c, name) {
				return i
			}
		}
		return -1
	}

	vals, ptrs := makeValPtrs(len(cols))
	boolCol := func(name string) bool {
		s := strings.ToLower(strVal(vals, col(name)))
		return s == "true" || s == "1" || s == "yes"
	}

	var users []SnowflakeUser
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		users = append(users, SnowflakeUser{
			Name:               strVal(vals, col("name")),
			LoginName:          strVal(vals, col("login_name")),
			DisplayName:        strVal(vals, col("display_name")),
			FirstName:          strVal(vals, col("first_name")),
			LastName:           strVal(vals, col("last_name")),
			Email:              strVal(vals, col("email")),
			DefaultWarehouse:   strVal(vals, col("default_warehouse")),
			DefaultRole:        strVal(vals, col("default_role")),
			DefaultNamespace:   strVal(vals, col("default_namespace")),
			Comment:            strVal(vals, col("comment")),
			Disabled:           boolCol("disabled"),
			MustChangePassword: boolCol("must_change_password"),
			DaysToExpiry:       strVal(vals, col("days_to_expiry")),
			Owner:              strVal(vals, col("owner")),
			LastSuccessLogin:   strVal(vals, col("last_success_login")),
		})
	}
	return users, rows.Err()
}

// ProcParam describes a single parameter of a stored procedure.
type ProcParam struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// splitByTopLevelComma splits s at commas that are not nested inside parentheses,
// so that types like NUMBER(38,0) are kept intact.
func splitByTopLevelComma(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if rest := strings.TrimSpace(s[start:]); rest != "" {
		parts = append(parts, rest)
	}
	return parts
}

// parseProcedureDDL extracts the parameter list from a CREATE PROCEDURE DDL
// string. It finds the opening parenthesis of the parameter list, locates its
// matching close, then splits the content into name/type pairs.
//
// Snowflake DDL format (simplified):
//
//	CREATE OR REPLACE PROCEDURE "DB"."SCHEMA"."NAME"(param1 TYPE, param2 TYPE)
//	RETURNS ... LANGUAGE ... AS '...';
func parseProcedureDDL(ddl string) []ProcParam {
	start := strings.Index(ddl, "(")
	if start < 0 {
		return nil
	}
	// Walk forward to find the matching closing paren.
	depth, end := 0, -1
	for i := start; i < len(ddl); i++ {
		switch ddl[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}
	paramStr := strings.TrimSpace(ddl[start+1 : end])
	if paramStr == "" {
		return nil
	}

	var params []ProcParam
	for _, part := range splitByTopLevelComma(paramStr) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Remove any DEFAULT clause (e.g. "amount NUMBER DEFAULT 0").
		if i := strings.Index(strings.ToUpper(part), " DEFAULT "); i >= 0 {
			part = strings.TrimSpace(part[:i])
		}
		// First whitespace-separated token is the parameter name; the rest is the type.
		spaceIdx := strings.IndexAny(part, " \t")
		if spaceIdx < 0 {
			// Only a bare type with no name — use a placeholder.
			params = append(params, ProcParam{Name: "param", DataType: part})
			continue
		}
		params = append(params, ProcParam{
			Name:     part[:spaceIdx],
			DataType: strings.TrimSpace(part[spaceIdx+1:]),
		})
	}
	return params
}

// GetProcedureParams fetches the DDL for a stored procedure and returns its
// parameter list with the real parameter names. argTypes must be the types
// string stored in SnowflakeObject.Arguments (e.g. "NUMBER, VARCHAR") so
// Snowflake can resolve the correct overload.
func (c *Client) GetProcedureParams(ctx context.Context, database, schema, name, argTypes string) ([]ProcParam, error) {
	ddl, err := c.GetObjectDDL(ctx, database, schema, "PROCEDURE", name, argTypes)
	if err != nil {
		return nil, err
	}
	return parseProcedureDDL(ddl), nil
}

// FunctionInfo holds the parameter list and type classification of a UDF.
type FunctionInfo struct {
	Params          []ProcParam `json:"params"`
	IsTableFunction bool        `json:"isTableFunction"`
}

// UserFunction describes a user-defined function returned by SHOW USER FUNCTIONS.
// It is used to populate the request/response translator pickers in the external
// function builder. Qualified is the fully double-quoted identifier
// ("DB"."SCHEMA"."NAME") suitable for embedding directly in a
// REQUEST_TRANSLATOR / RESPONSE_TRANSLATOR clause; Arguments is the raw signature
// string (e.g. "MYFN(VARCHAR) RETURN VARCHAR") for display only.
type UserFunction struct {
	Name      string `json:"name"`
	Schema    string `json:"schema"`
	Database  string `json:"database"`
	Qualified string `json:"qualified"`
	Arguments string `json:"arguments"`
}

// ListUserFunctions runs SHOW USER FUNCTIONS (scoped to database when non-empty)
// and returns the accessible scalar UDFs. Built-in functions are excluded — SHOW
// USER FUNCTIONS already returns only user-defined functions — and so are table
// functions (is_table_function=Y) and external functions (is_external_function=Y),
// since the request/response translator slots that consume this list require a
// scalar UDF. Columns are located by name (case-insensitive) so column-order
// changes on Snowflake's side don't break parsing.
func (c *Client) ListUserFunctions(ctx context.Context, database string) ([]UserFunction, error) {
	query := "SHOW USER FUNCTIONS"
	if database != "" {
		query += fmt.Sprintf(" IN DATABASE %s", QuoteIdent(database))
	}
	res, err := c.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	nameIdx, schemaIdx, dbIdx, argsIdx, isTableFnIdx, isExternalFnIdx := -1, -1, -1, -1, -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "schema_name":
			schemaIdx = i
		case "catalog_name":
			dbIdx = i
		case "arguments":
			argsIdx = i
		case "is_table_function":
			isTableFnIdx = i
		case "is_external_function":
			isExternalFnIdx = i
		}
	}
	if nameIdx < 0 {
		return nil, fmt.Errorf("no 'name' column in SHOW USER FUNCTIONS result")
	}
	out := make([]UserFunction, 0, len(res.Rows))
	for _, row := range res.Rows {
		cell := func(idx int) string {
			if idx < 0 || idx >= len(row) || row[idx] == nil {
				return ""
			}
			return fmt.Sprintf("%v", row[idx])
		}
		// Translators must be scalar UDFs — skip table and external functions.
		if isTableFnIdx >= 0 && strings.EqualFold(cell(isTableFnIdx), "Y") {
			continue
		}
		if isExternalFnIdx >= 0 && strings.EqualFold(cell(isExternalFnIdx), "Y") {
			continue
		}
		uf := UserFunction{
			Name:      cell(nameIdx),
			Schema:    cell(schemaIdx),
			Database:  cell(dbIdx),
			Arguments: cell(argsIdx),
		}
		if uf.Name == "" {
			continue
		}
		parts := make([]string, 0, 3)
		if uf.Database != "" {
			parts = append(parts, QuoteIdent(uf.Database))
		}
		if uf.Schema != "" {
			parts = append(parts, QuoteIdent(uf.Schema))
		}
		parts = append(parts, QuoteIdent(uf.Name))
		uf.Qualified = strings.Join(parts, ".")
		out = append(out, uf)
	}
	return out, nil
}

// GetTableColumns returns the ordered list of column names for a table or view
// by running DESCRIBE TABLE (which works for both base tables and views in Snowflake).
func (c *Client) GetTableColumns(ctx context.Context, database, schema, name string) ([]string, error) {
	query := fmt.Sprintf(`DESCRIBE TABLE %s`, Qualify(database, schema, name))
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	nameIdx := -1
	for i, col := range cols {
		if strings.EqualFold(col, "name") {
			nameIdx = i
			break
		}
	}
	if nameIdx < 0 {
		return nil, fmt.Errorf("unexpected DESCRIBE TABLE result")
	}

	vals, ptrs := makeValPtrs(len(cols))
	var columnNames []string
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		if n := strVal(vals, nameIdx); n != "" {
			columnNames = append(columnNames, n)
		}
	}
	return columnNames, rows.Err()
}

// TableForeignKey describes a single foreign-key column mapping between two tables.
// It is returned by GetTableForeignKeys and used by the editor's JOIN ON autocomplete.
type TableForeignKey struct {
	PKDatabase     string `json:"pkDatabase"`
	PKSchema       string `json:"pkSchema"`
	PKTable        string `json:"pkTable"`
	PKColumn       string `json:"pkColumn"`
	FKDatabase     string `json:"fkDatabase"`
	FKSchema       string `json:"fkSchema"`
	FKTable        string `json:"fkTable"`
	FKColumn       string `json:"fkColumn"`
	ConstraintName string `json:"constraintName"` // fk_name column
	KeySequence    int    `json:"keySequence"`    // key_sequence column (1-based)
}

// GetTableForeignKeys returns every foreign key where the given table is the
// referencing (child / FK) side. It runs SHOW IMPORTED KEYS IN TABLE and maps
// the pk_*/fk_* columns into TableForeignKey values.
func (c *Client) GetTableForeignKeys(ctx context.Context, database, schema, table string) ([]TableForeignKey, error) {
	query := fmt.Sprintf(`SHOW IMPORTED KEYS IN TABLE %s`, Qualify(database, schema, table))
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols,
		"pk_database_name", "pk_schema_name", "pk_table_name", "pk_column_name",
		"fk_database_name", "fk_schema_name", "fk_table_name", "fk_column_name",
		"fk_name", "key_sequence",
	)

	result := []TableForeignKey{}
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		seq, _ := strconv.Atoi(strVal(vals, idxs["key_sequence"]))
		result = append(result, TableForeignKey{
			PKDatabase:     strVal(vals, idxs["pk_database_name"]),
			PKSchema:       strVal(vals, idxs["pk_schema_name"]),
			PKTable:        strVal(vals, idxs["pk_table_name"]),
			PKColumn:       strVal(vals, idxs["pk_column_name"]),
			FKDatabase:     strVal(vals, idxs["fk_database_name"]),
			FKSchema:       strVal(vals, idxs["fk_schema_name"]),
			FKTable:        strVal(vals, idxs["fk_table_name"]),
			FKColumn:       strVal(vals, idxs["fk_column_name"]),
			ConstraintName: strVal(vals, idxs["fk_name"]),
			KeySequence:    seq,
		})
	}
	return result, rows.Err()
}

// ColumnInfo holds the name and data-type string for a single table column.
// It is returned by GetTableColumnsWithTypes and used by the editor's JOIN ON
// autocomplete to filter same-name suggestions by type compatibility.
type ColumnInfo struct {
	Name         string `json:"name"`
	DataType     string `json:"dataType"` // e.g. "VARCHAR(256)", "NUMBER(38,0)"
	Nullable     bool   `json:"nullable"`
	IsPrimaryKey bool   `json:"isPrimaryKey"`
	IsUnique     bool   `json:"isUnique"`
	Comment      string `json:"comment"` // column comment, empty if none
}

// GetTableColumnsWithTypes returns the ordered column list for a table or view
// together with their data types by running DESCRIBE TABLE.
func (c *Client) GetTableColumnsWithTypes(ctx context.Context, database, schema, name string) ([]ColumnInfo, error) {
	query := fmt.Sprintf(`DESCRIBE TABLE %s`, Qualify(database, schema, name))
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "name", "type", "null?", "primary key", "unique key", "comment")

	result := []ColumnInfo{}
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		n := strVal(vals, idxs["name"])
		if n == "" {
			continue
		}
		result = append(result, ColumnInfo{
			Name:         n,
			DataType:     strVal(vals, idxs["type"]),
			Nullable:     strVal(vals, idxs["null?"]) == "Y",
			IsPrimaryKey: strVal(vals, idxs["primary key"]) == "Y",
			IsUnique:     strVal(vals, idxs["unique key"]) == "Y",
			Comment:      strVal(vals, idxs["comment"]),
		})
	}
	return result, rows.Err()
}

// ColumnDetails holds the column properties that DESCRIBE TABLE / the column
// list don't surface cheaply — the DEFAULT expression and the currently attached
// masking policy — for the column properties editor.
type ColumnDetails struct {
	Default       string `json:"default"`       // DEFAULT expression, empty if none
	MaskingPolicy string `json:"maskingPolicy"` // fully-qualified policy name, empty if none
}

// GetColumnDetails returns the DEFAULT expression and the masking policy attached
// to a single column. The default comes from DESCRIBE TABLE; the masking policy
// from the INFORMATION_SCHEMA.POLICY_REFERENCES table function (latency-free,
// unlike the ACCOUNT_USAGE view). A masking-policy lookup failure (missing
// governance privileges) is swallowed so the default still loads.
func (c *Client) GetColumnDetails(ctx context.Context, database, schema, table, column string) (ColumnDetails, error) {
	var out ColumnDetails

	// DEFAULT — scan DESCRIBE TABLE inside a closure so a panic mid-scan still
	// closes the cursor (defer) before the masking-policy lookup runs.
	if err := func() error {
		rows, err := c.queryCtx(ctx, fmt.Sprintf("DESCRIBE TABLE %s", Qualify(database, schema, table)))
		if err != nil {
			return err
		}
		defer rows.Close() //nolint:errcheck
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "name", "default")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if err := rows.Scan(ptrs...); err != nil {
				continue
			}
			if strVal(vals, idxs["name"]) == column {
				out.Default = strVal(vals, idxs["default"])
				break
			}
		}
		return rows.Err()
	}(); err != nil {
		return out, err
	}

	// Masking policy — best effort. REF_ENTITY_NAME takes a plain dotted FQN, not
	// a double-quoted one (Qualify's quoting would make the function match nothing).
	mpQuery := fmt.Sprintf(
		"SELECT POLICY_DB, POLICY_SCHEMA, POLICY_NAME "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.POLICY_REFERENCES(REF_ENTITY_NAME => '%s', REF_ENTITY_DOMAIN => 'TABLE')) "+
			"WHERE POLICY_KIND = 'MASKING_POLICY' AND REF_COLUMN_NAME = '%s'",
		QuoteIdent(database),
		EscapeStringLit(fmt.Sprintf("%s.%s.%s", database, schema, table)),
		EscapeStringLit(column))
	if mrows, err := c.queryCtx(ctx, mpQuery); err == nil {
		defer mrows.Close() //nolint:errcheck
		if mrows.Next() {
			var pdb, psc, pname string
			if err := mrows.Scan(&pdb, &psc, &pname); err == nil {
				// Plain dotted FQN — matches the unquoted form the frontend's
				// policy picker builds, so the current policy preselects in the
				// dropdown (Qualify's double-quoting would never match).
				out.MaskingPolicy = fmt.Sprintf("%s.%s.%s", pdb, psc, pname)
			}
		}
	}
	return out, nil
}

// GetSchemaForeignKeys returns all FK→PK column mappings in a schema by running
// SHOW IMPORTED KEYS IN SCHEMA. This bulk call is cheaper than per-table SHOW
// IMPORTED KEYS when the editor needs to warm up FK data for many tables at once.
// The result set columns are identical to SHOW IMPORTED KEYS IN TABLE.
func (c *Client) GetSchemaForeignKeys(ctx context.Context, database, schema string) ([]TableForeignKey, error) {
	query := fmt.Sprintf(`SHOW IMPORTED KEYS IN SCHEMA %s`, Qualify(database, schema))
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols,
		"pk_database_name", "pk_schema_name", "pk_table_name", "pk_column_name",
		"fk_database_name", "fk_schema_name", "fk_table_name", "fk_column_name",
		"fk_name", "key_sequence",
	)

	result := []TableForeignKey{}
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		seq, _ := strconv.Atoi(strVal(vals, idxs["key_sequence"]))
		result = append(result, TableForeignKey{
			PKDatabase:     strVal(vals, idxs["pk_database_name"]),
			PKSchema:       strVal(vals, idxs["pk_schema_name"]),
			PKTable:        strVal(vals, idxs["pk_table_name"]),
			PKColumn:       strVal(vals, idxs["pk_column_name"]),
			FKDatabase:     strVal(vals, idxs["fk_database_name"]),
			FKSchema:       strVal(vals, idxs["fk_schema_name"]),
			FKTable:        strVal(vals, idxs["fk_table_name"]),
			FKColumn:       strVal(vals, idxs["fk_column_name"]),
			ConstraintName: strVal(vals, idxs["fk_name"]),
			KeySequence:    seq,
		})
	}
	return result, rows.Err()
}

// GetFunctionInfo fetches the DDL for a user-defined function and returns its
// parameter list together with a flag indicating whether it is a table function
// (UDTF, whose DDL contains RETURNS TABLE) or a scalar function.
func (c *Client) GetFunctionInfo(ctx context.Context, database, schema, name, argTypes string) (*FunctionInfo, error) {
	ddl, err := c.GetObjectDDL(ctx, database, schema, "FUNCTION", name, argTypes)
	if err != nil {
		return nil, err
	}
	// A UDTF always has RETURNS TABLE(...) in its DDL; scalar functions never do.
	isTable := strings.Contains(strings.ToUpper(ddl), "RETURNS TABLE")
	return &FunctionInfo{
		Params:          parseProcedureDDL(ddl),
		IsTableFunction: isTable,
	}, nil
}

// showInSchema runs a SHOW command and collects results as SnowflakeObjects.
// If fixedKind is non-empty it is used as the Kind for every row; otherwise
// the "kind" column in the result set is read (as in SHOW OBJECTS).
// For PROCEDURE and FUNCTION kinds the "arguments" column is also captured so
// that GET_DDL can be called with the correct overload signature.
func (c *Client) showInSchema(ctx context.Context, query, fixedKind, schema string) ([]SnowflakeObject, error) {
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	nameIdx, kindIdx, argsIdx, builtinIdx, rowsIdx, predsIdx, taskRelIdx, finalizeColIdx, isDynamicIdx, isExternalIdx, isIcebergIdx, isHybridIdx, isExternalFnIdx, isDataMetricIdx := -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1
	for i, col := range cols {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "kind":
			kindIdx = i
		case "arguments":
			argsIdx = i
		case "is_builtin":
			builtinIdx = i
		case "rows":
			rowsIdx = i
		case "predecessors", "predecessor":
			predsIdx = i
		case "task_relations":
			taskRelIdx = i
		case "finalize", "finalize_task":
			finalizeColIdx = i
		case "is_dynamic":
			isDynamicIdx = i
		case "is_external":
			isExternalIdx = i
		case "is_iceberg":
			isIcebergIdx = i
		case "is_hybrid":
			isHybridIdx = i
		case "is_external_function":
			isExternalFnIdx = i
		case "is_data_metric":
			isDataMetricIdx = i
		}
	}
	if nameIdx < 0 {
		return nil, fmt.Errorf("no 'name' column in: %s cols=%v", query, cols)
	}
	if fixedKind == "" && kindIdx < 0 {
		return nil, fmt.Errorf("no 'kind' column in: %s cols=%v", query, cols)
	}

	captureArgs := fixedKind == "PROCEDURE" || fixedKind == "FUNCTION" || fixedKind == "EXTERNAL FUNCTION" || fixedKind == "DATA METRIC FUNCTION"

	var objects []SnowflakeObject
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		name := fmt.Sprintf("%v", vals[nameIdx])
		// Skip Snowflake-internal procedures/functions. These show up in SHOW
		// PROCEDURES / SHOW FUNCTIONS with is_builtin=Y (e.g. EXECUTE_AI_EVALUATION,
		// COMPUTE_AI_OBSERVABILITY_METRICS) or with the SYSTEM$ prefix, and are
		// not accessible to users via GET_DDL.
		if strings.HasPrefix(name, "SYSTEM$") {
			continue
		}
		if builtinIdx >= 0 && strings.EqualFold(fmt.Sprintf("%v", vals[builtinIdx]), "Y") {
			continue
		}
		// Dynamic tables surface in SHOW OBJECTS / SHOW TABLES with is_dynamic=Y.
		// They are listed separately via SHOW DYNAMIC TABLES (kind "DYNAMIC
		// TABLE"), so skip them here on the generic-kind path to avoid duplicate
		// tree entries (one under Tables, one under Dynamic Tables).
		if fixedKind == "" && isDynamicIdx >= 0 && strings.EqualFold(fmt.Sprintf("%v", vals[isDynamicIdx]), "Y") {
			continue
		}
		// External tables surface in SHOW OBJECTS with is_external=Y on editions
		// that expose the column. They are listed separately via SHOW EXTERNAL
		// TABLES (kind "EXTERNAL TABLE"), so skip them here on the generic-kind
		// path to avoid duplicate tree entries (one under Tables, one under
		// External Tables). dedupeExternalTables in ListObjects is the
		// belt-and-suspenders fallback when this column is absent.
		if fixedKind == "" && isExternalIdx >= 0 && strings.EqualFold(fmt.Sprintf("%v", vals[isExternalIdx]), "Y") {
			continue
		}
		// Iceberg tables surface in SHOW OBJECTS with is_iceberg=Y. They are
		// listed separately via SHOW ICEBERG TABLES (kind "ICEBERG TABLE"), so
		// skip them here on the generic-kind path to avoid duplicate tree entries
		// (one under Tables, one under Iceberg Tables). dedupeIcebergTables in
		// ListObjects is the belt-and-suspenders fallback when this column is
		// absent.
		if fixedKind == "" && isIcebergIdx >= 0 && strings.EqualFold(fmt.Sprintf("%v", vals[isIcebergIdx]), "Y") {
			continue
		}
		// Hybrid tables surface in SHOW OBJECTS / SHOW TABLES with is_hybrid=Y.
		// They are listed separately via SHOW HYBRID TABLES (kind "HYBRID
		// TABLE"), so skip them here on the generic-kind path to avoid duplicate
		// tree entries (one under Tables, one under Hybrid Tables).
		// dedupeHybridTables in ListObjects is the belt-and-suspenders fallback
		// when this column is absent.
		if fixedKind == "" && isHybridIdx >= 0 && strings.EqualFold(fmt.Sprintf("%v", vals[isHybridIdx]), "Y") {
			continue
		}
		// External functions surface in SHOW FUNCTIONS with
		// is_external_function=Y. Rather than dropping them here on the FUNCTION
		// path, relabel them to kind "EXTERNAL FUNCTION" so they still group under
		// External Functions even if the dedicated SHOW EXTERNAL FUNCTIONS command
		// fails for this schema (its per-type failure is silently swallowed in
		// ListExtendedObjects) — dropping here would make them vanish from the tree
		// entirely. The dedicated command also returns them, so
		// dedupeExternalFunctions collapses the resulting duplicate
		// EXTERNAL FUNCTION entries; on the column-absent edition it instead drops
		// the plain FUNCTION entry that collides with an EXTERNAL FUNCTION.
		externalFn := fixedKind == "FUNCTION" && isExternalFnIdx >= 0 && strings.EqualFold(fmt.Sprintf("%v", vals[isExternalFnIdx]), "Y")
		// Data metric functions surface in SHOW FUNCTIONS with is_data_metric=Y.
		// As with external functions, relabel them to kind "DATA METRIC FUNCTION"
		// so they group under Data Metric Functions even if the dedicated SHOW
		// DATA METRIC FUNCTIONS command failed for this schema; dedupeDataMetricFunctions
		// collapses the resulting duplicates (or drops the plain FUNCTION collision
		// on editions without the column).
		dataMetricFn := fixedKind == "FUNCTION" && isDataMetricIdx >= 0 && strings.EqualFold(fmt.Sprintf("%v", vals[isDataMetricIdx]), "Y")
		kind := fixedKind
		if kind == "" {
			kind = fmt.Sprintf("%v", vals[kindIdx])
		}
		if externalFn {
			kind = "EXTERNAL FUNCTION"
		}
		if dataMetricFn {
			kind = "DATA METRIC FUNCTION"
		}
		var argTypes string
		if captureArgs && argsIdx >= 0 {
			argTypes = extractArgTypes(fmt.Sprintf("%v", vals[argsIdx]))
		}
		var rowCount *int64
		if rowsIdx >= 0 && vals[rowsIdx] != nil {
			var n int64
			ok := false
			switch v := vals[rowsIdx].(type) {
			case int64:
				n, ok = v, true
			case float64:
				n, ok = int64(v), true
			case string:
				if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
					n, ok = parsed, true
				}
			case []uint8:
				if parsed, err := strconv.ParseInt(string(v), 10, 64); err == nil {
					n, ok = parsed, true
				}
			}
			if ok {
				rowCount = &n
			}
		}
		var preds string
		if fixedKind == "TASK" && predsIdx >= 0 && vals[predsIdx] != nil {
			raw := fmt.Sprintf("%v", vals[predsIdx])
			if raw != "<nil>" {
				preds = raw
			}
		}
		var finalize string
		if fixedKind == "TASK" {
			if finalizeColIdx >= 0 && vals[finalizeColIdx] != nil {
				if s := fmt.Sprintf("%v", vals[finalizeColIdx]); s != "" && s != "<nil>" && s != "null" {
					finalize = s
				}
			}
			if finalize == "" && taskRelIdx >= 0 && vals[taskRelIdx] != nil {
				finalize = parseFinalizeFromRelJSON(vals[taskRelIdx])
			}
		}
		objects = append(objects, SnowflakeObject{Name: name, Kind: kind, Schema: schema, Arguments: argTypes, RowCount: rowCount, Predecessors: preds, Finalize: finalize})
	}
	return objects, rows.Err()
}

// parseFinalizeFromRelJSON extracts the root-task name from a task_relations
// VARIANT column value. gosnowflake may return VARIANT columns as:
//   - map[string]interface{} — already-parsed JSON object
//   - string / []byte       — raw JSON text
//
// Key comparison is case-insensitive to handle Snowflake edition variations.
func parseFinalizeFromRelJSON(v interface{}) string {
	if v == nil {
		return ""
	}
	isFinalKey := func(k string) bool {
		lk := strings.ToLower(k)
		return lk == "finalize" || lk == "finalize_task"
	}
	if m, ok := v.(map[string]interface{}); ok {
		for k, val := range m {
			if isFinalKey(k) && val != nil {
				if s := fmt.Sprintf("%v", val); s != "" && s != "<nil>" && s != "null" {
					return s
				}
			}
		}
		return ""
	}
	var raw string
	switch t := v.(type) {
	case string:
		raw = t
	case []byte:
		raw = string(t)
	default:
		raw = fmt.Sprintf("%v", v)
	}
	if raw == "" || raw == "null" || raw == "<nil>" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	for k, val := range m {
		if isFinalKey(k) {
			var s string
			if err := json.Unmarshal(val, &s); err == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// parseFinalizeFromDDLText extracts the FINALIZE = ... value from task DDL
// text, e.g. from GET_DDL('TASK', ...) output.
func parseFinalizeFromDDLText(ddl string) string {
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
	end := strings.IndexAny(rest, " \t\n\r")
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimRight(rest[:end], ";,")
}

// ListBasicObjects returns the "basic" objects inside a schema by running a
// single SHOW OBJECTS IN SCHEMA command. This returns TABLEs, VIEWs,
// SEQUENCEs, and other object types exposed by the kind column — but not
// PROCEDUREs, FUNCTIONs, TASKs, STREAMs, STAGEs, etc.
func (c *Client) ListBasicObjects(ctx context.Context, database, schema string) ([]SnowflakeObject, error) {
	cacheKey := "basic\x00" + database + "\x00" + schema
	if cached, ok := c.getObjectCache(cacheKey); ok {
		return cached, nil
	}

	q := Qualify(database, schema)
	objs, err := c.showInSchema(ctx, fmt.Sprintf("SHOW OBJECTS IN SCHEMA %s", q), "", schema)
	if err != nil {
		return nil, err
	}
	c.putObjectCache(cacheKey, objs)
	return objs, nil
}

// ListExtendedObjects returns the "extended" objects inside a schema by running
// dedicated SHOW commands for object types not covered by SHOW OBJECTS (the
// authoritative list is the command slice below: DYNAMIC TABLE, EXTERNAL TABLE,
// ICEBERG TABLE, HYBRID TABLE, EVENT TABLE,
// MATERIALIZED VIEW, ALERT, TAG, MASKING POLICY, ROW ACCESS POLICY, JOIN POLICY,
// PRIVACY POLICY, STORAGE LIFECYCLE POLICY,
// PASSWORD POLICY, SESSION POLICY, AGGREGATION POLICY, PROJECTION POLICY,
// AUTHENTICATION POLICY, PACKAGES POLICY, NETWORK
// RULE, IMAGE REPOSITORY, SERVICE, STREAMLIT, PROCEDURE, FUNCTION,
// EXTERNAL FUNCTION, DATA METRIC FUNCTION, TASK, STREAM, STAGE, FILE FORMAT,
// PIPE, NOTEBOOK, SECRET, GIT REPOSITORY, DBT PROJECT, MODEL). Individual commands that
// fail (e.g. due to missing privileges) are silently skipped. Includes the TASK
// finalize enrichment logic.
func (c *Client) ListExtendedObjects(ctx context.Context, database, schema string) ([]SnowflakeObject, error) {
	q := Qualify(database, schema)

	type showCmd struct {
		query string
		kind  string
	}
	commands := []showCmd{
		{fmt.Sprintf("SHOW DYNAMIC TABLES IN SCHEMA %s", q), "DYNAMIC TABLE"},
		{fmt.Sprintf("SHOW EXTERNAL TABLES IN SCHEMA %s", q), "EXTERNAL TABLE"},
		{fmt.Sprintf("SHOW ICEBERG TABLES IN SCHEMA %s", q), "ICEBERG TABLE"},
		{fmt.Sprintf("SHOW HYBRID TABLES IN SCHEMA %s", q), "HYBRID TABLE"},
		{fmt.Sprintf("SHOW EVENT TABLES IN SCHEMA %s", q), "EVENT TABLE"},
		{fmt.Sprintf("SHOW MATERIALIZED VIEWS IN SCHEMA %s", q), "MATERIALIZED VIEW"},
		{fmt.Sprintf("SHOW ALERTS IN SCHEMA %s", q), "ALERT"},
		{fmt.Sprintf("SHOW TAGS IN SCHEMA %s", q), "TAG"},
		{fmt.Sprintf("SHOW MASKING POLICIES IN SCHEMA %s", q), "MASKING POLICY"},
		{fmt.Sprintf("SHOW ROW ACCESS POLICIES IN SCHEMA %s", q), "ROW ACCESS POLICY"},
		{fmt.Sprintf("SHOW JOIN POLICIES IN SCHEMA %s", q), "JOIN POLICY"},
		{fmt.Sprintf("SHOW PRIVACY POLICIES IN SCHEMA %s", q), "PRIVACY POLICY"},
		{fmt.Sprintf("SHOW STORAGE LIFECYCLE POLICIES IN SCHEMA %s", q), "STORAGE LIFECYCLE POLICY"},
		{fmt.Sprintf("SHOW PASSWORD POLICIES IN SCHEMA %s", q), "PASSWORD POLICY"},
		{fmt.Sprintf("SHOW SESSION POLICIES IN SCHEMA %s", q), "SESSION POLICY"},
		{fmt.Sprintf("SHOW AGGREGATION POLICIES IN SCHEMA %s", q), "AGGREGATION POLICY"},
		{fmt.Sprintf("SHOW PROJECTION POLICIES IN SCHEMA %s", q), "PROJECTION POLICY"},
		{fmt.Sprintf("SHOW AUTHENTICATION POLICIES IN SCHEMA %s", q), "AUTHENTICATION POLICY"},
		{fmt.Sprintf("SHOW PACKAGES POLICIES IN SCHEMA %s", q), "PACKAGES POLICY"},
		{fmt.Sprintf("SHOW NETWORK RULES IN SCHEMA %s", q), "NETWORK RULE"},
		{fmt.Sprintf("SHOW IMAGE REPOSITORIES IN SCHEMA %s", q), "IMAGE REPOSITORY"},
		{fmt.Sprintf("SHOW SERVICES IN SCHEMA %s", q), "SERVICE"},
		{fmt.Sprintf("SHOW GATEWAYS IN SCHEMA %s", q), "GATEWAY"},
		{fmt.Sprintf("SHOW CONTACTS IN SCHEMA %s", q), "CONTACT"},
		{fmt.Sprintf("SHOW STREAMLITS IN SCHEMA %s", q), "STREAMLIT"},
		{fmt.Sprintf("SHOW PROCEDURES IN SCHEMA %s", q), "PROCEDURE"},
		{fmt.Sprintf("SHOW FUNCTIONS IN SCHEMA %s", q), "FUNCTION"},
		{fmt.Sprintf("SHOW EXTERNAL FUNCTIONS IN SCHEMA %s", q), "EXTERNAL FUNCTION"},
		{fmt.Sprintf("SHOW DATA METRIC FUNCTIONS IN SCHEMA %s", q), "DATA METRIC FUNCTION"},
		{fmt.Sprintf("SHOW TASKS IN SCHEMA %s", q), "TASK"},
		{fmt.Sprintf("SHOW STREAMS IN SCHEMA %s", q), "STREAM"},
		{fmt.Sprintf("SHOW STAGES IN SCHEMA %s", q), "STAGE"},
		{fmt.Sprintf("SHOW FILE FORMATS IN SCHEMA %s", q), "FILE FORMAT"},
		{fmt.Sprintf("SHOW PIPES IN SCHEMA %s", q), "PIPE"},
		{fmt.Sprintf("SHOW NOTEBOOKS IN SCHEMA %s", q), "NOTEBOOK"},
		{fmt.Sprintf("SHOW SECRETS IN SCHEMA %s", q), "SECRET"},
		{fmt.Sprintf("SHOW GIT REPOSITORIES IN SCHEMA %s", q), "GIT REPOSITORY"},
		{fmt.Sprintf("SHOW DBT PROJECTS IN SCHEMA %s", q), "DBT PROJECT"},
		{fmt.Sprintf("SHOW MODELS IN SCHEMA %s", q), "MODEL"},
		{fmt.Sprintf("SHOW MODEL MONITORS IN SCHEMA %s", q), "MODEL MONITOR"},
		{fmt.Sprintf("SHOW DATASETS IN SCHEMA %s", q), "DATASET"},
		{fmt.Sprintf("SHOW CORTEX SEARCH SERVICES IN SCHEMA %s", q), "CORTEX SEARCH SERVICE"},
		{fmt.Sprintf("SHOW AGENTS IN SCHEMA %s", q), "AGENT"},
		{fmt.Sprintf("SHOW EXTERNAL AGENTS IN SCHEMA %s", q), "EXTERNAL AGENT"},
		{fmt.Sprintf("SHOW MCP SERVERS IN SCHEMA %s", q), "MCP SERVER"},
		{fmt.Sprintf("SHOW SEMANTIC VIEWS IN SCHEMA %s", q), "SEMANTIC VIEW"},
	}

	// Filter out disabled object kinds (set via SetExcludedExtendedKinds).
	excl := c.getExcludedExtendedKinds()
	if len(excl) > 0 {
		filtered := make([]showCmd, 0, len(commands))
		for _, cmd := range commands {
			if !excl[cmd.kind] {
				filtered = append(filtered, cmd)
			}
		}
		commands = filtered
	}

	type result struct {
		objs []SnowflakeObject
		err  error
	}
	results := make([]result, len(commands))

	var wg sync.WaitGroup
	for i, cmd := range commands {
		wg.Add(1)
		go func(i int, cmd showCmd) {
			defer wg.Done()
			results[i].objs, results[i].err = c.showInSchema(ctx, cmd.query, cmd.kind, schema)
		}(i, cmd)
	}
	wg.Wait()

	var all []SnowflakeObject
	for _, r := range results {
		if r.err != nil {
			continue // skip types we can't access
		}
		all = append(all, r.objs...)
	}

	// External functions reach this slice from two SHOW commands: SHOW EXTERNAL
	// FUNCTIONS (kind "EXTERNAL FUNCTION") and SHOW FUNCTIONS (where showInSchema
	// relabels is_external_function=Y rows to "EXTERNAL FUNCTION", or leaves them
	// as plain "FUNCTION" on editions without that column). dedupeExternalFunctions
	// collapses the duplicate "EXTERNAL FUNCTION" entries and drops any plain
	// "FUNCTION" that collides with one, so each external function appears exactly
	// once under External Functions — even if one of the two SHOW commands fails.
	all = dedupeFunctionVariant(all, "EXTERNAL FUNCTION")

	// Data metric functions reach this slice the same two ways external functions
	// do: SHOW DATA METRIC FUNCTIONS (kind "DATA METRIC FUNCTION") and SHOW
	// FUNCTIONS (where showInSchema relabels is_data_metric=Y rows). Collapse the
	// duplicate "DATA METRIC FUNCTION" entries and drop any plain "FUNCTION" that
	// collides with one.
	all = dedupeFunctionVariant(all, "DATA METRIC FUNCTION")

	// GET_DDL fallback: for Snowflake editions that don't expose the FINALIZE
	// relationship via SHOW TASKS columns (task_relations / finalize), call
	// GET_DDL on standalone TASK objects to detect finalizer tasks.
	// "Standalone" = no predecessors AND no other task depends on it (not a root).
	enrichTaskFinalize(ctx, c, database, schema, all)

	return all, nil
}

// dedupeFunctionVariant reconciles the two ways a function-like object of the
// given <kind> (e.g. "EXTERNAL FUNCTION", "DATA METRIC FUNCTION") reaches the
// extended-object list. SHOW <KIND>S returns it as kind <kind>; SHOW FUNCTIONS
// returns it too (with the variant's discriminator column), where showInSchema
// relabels it to <kind> when that column is present or leaves it as plain
// "FUNCTION" when it is absent. This pass therefore:
//
//   - collapses duplicate <kind> entries (one from each SHOW command) to a single
//     entry, and
//   - drops any plain "FUNCTION" entry whose (schema, name, arguments) collides
//     with a <kind> entry (the column-absent edition).
//
// Matching is case-insensitive and includes the argument signature because
// functions overload by signature. The input slice is not mutated.
func dedupeFunctionVariant(objs []SnowflakeObject, kind string) []SnowflakeObject {
	keys := make(map[string]struct{})
	for _, o := range objs {
		if o.Kind == kind {
			keys[externalFunctionDedupeKey(o)] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return objs
	}
	out := make([]SnowflakeObject, 0, len(objs))
	seen := make(map[string]struct{})
	for _, o := range objs {
		key := externalFunctionDedupeKey(o)
		switch o.Kind {
		case kind:
			if _, dup := seen[key]; dup {
				continue // already emitted from the other SHOW command
			}
			seen[key] = struct{}{}
		case "FUNCTION":
			if _, dup := keys[key]; dup {
				continue // a plain FUNCTION that is really <kind> (column absent)
			}
		}
		out = append(out, o)
	}
	return out
}

// externalFunctionDedupeKey builds the case-insensitive (schema, name, arguments)
// key used to match a FUNCTION against an EXTERNAL FUNCTION.
func externalFunctionDedupeKey(o SnowflakeObject) string {
	return strings.ToUpper(o.Schema) + "\x00" + strings.ToUpper(o.Name) + "\x00" + strings.ToUpper(strings.TrimSpace(o.Arguments))
}

// enrichTaskFinalize enriches TASK objects with FINALIZE metadata by calling
// GET_DDL on standalone tasks that don't already have the finalize field set.
func enrichTaskFinalize(ctx context.Context, c *Client, database, schema string, all []SnowflakeObject) {
	hasChildrenSet := map[string]bool{}
	for _, o := range all {
		if o.Kind != "TASK" {
			continue
		}
		preds := strings.TrimSpace(o.Predecessors)
		if preds == "" || preds == "[]" || preds == "<nil>" {
			continue
		}
		stripped := strings.TrimPrefix(strings.TrimSuffix(preds, "]"), "[")
		for _, part := range strings.Split(stripped, ",") {
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

	var enrichWG sync.WaitGroup
	for i, o := range all {
		if o.Kind != "TASK" || o.Finalize != "" {
			continue
		}
		preds := strings.TrimSpace(o.Predecessors)
		if preds != "" && preds != "[]" && preds != "<nil>" {
			continue
		}
		if hasChildrenSet[strings.ToUpper(o.Name)] {
			continue
		}
		i := i // capture for goroutine
		enrichWG.Add(1)
		go func() {
			defer enrichWG.Done()
			ddl, err := c.GetObjectDDL(ctx, database, schema, "TASK", all[i].Name, "")
			if err != nil {
				return
			}
			if fin := parseFinalizeFromDDLText(ddl); fin != "" {
				all[i].Finalize = fin
			}
		}()
	}
	enrichWG.Wait()
}

// ListObjects returns all objects inside a schema by running multiple SHOW
// commands concurrently. Individual commands that fail (e.g. due to missing
// privileges on a particular object type) are silently skipped so that the
// rest still appear.
func (c *Client) ListObjects(ctx context.Context, database, schema string) ([]SnowflakeObject, error) {
	cacheKey := "full\x00" + database + "\x00" + schema
	if cached, ok := c.getObjectCache(cacheKey); ok {
		return cached, nil
	}

	basic, err := c.ListBasicObjects(ctx, database, schema)
	if err != nil {
		return nil, err
	}
	extended, err := c.ListExtendedObjects(ctx, database, schema)
	if err != nil {
		// If extended objects fail, still return basic objects.
		return basic, nil
	}
	// Dynamic tables are listed explicitly by ListExtendedObjects (kind
	// "DYNAMIC TABLE"). They also surface in SHOW OBJECTS, where showInSchema
	// drops them via the is_dynamic=Y column. As a belt-and-suspenders against
	// Snowflake editions that don't expose that column (which would let a
	// dynamic table appear under both Tables and Dynamic Tables), also drop any
	// basic entry whose (schema, name) collides with an extended dynamic table —
	// a real table and a dynamic table cannot share a name in one schema.
	basic = dedupeDynamicTables(basic, extended)
	// External tables are listed explicitly by ListExtendedObjects (kind
	// "EXTERNAL TABLE") and are dropped from the generic SHOW OBJECTS path by the
	// is_external=Y column when present; drop any remaining (schema, name)
	// collision as a fallback for editions that omit that column.
	basic = dedupeExternalTables(basic, extended)
	// Iceberg tables are listed explicitly by ListExtendedObjects (kind "ICEBERG
	// TABLE") and are dropped from the generic SHOW OBJECTS path by the
	// is_iceberg=Y column when present; drop any remaining (schema, name)
	// collision as a fallback for editions that omit that column.
	basic = dedupeIcebergTables(basic, extended)
	// Hybrid tables are listed explicitly by ListExtendedObjects (kind "HYBRID
	// TABLE") and are dropped from the generic SHOW OBJECTS path by the
	// is_hybrid=Y column when present; drop any remaining (schema, name)
	// collision as a fallback for editions that omit that column.
	basic = dedupeHybridTables(basic, extended)
	// Event tables are listed explicitly by ListExtendedObjects (kind "EVENT
	// TABLE"). SHOW OBJECTS is not expected to surface them (there is no is_event
	// column), but as a belt-and-suspenders against editions that might return one
	// as a plain TABLE, drop any (schema, name) collision so it can't appear under
	// both Tables and Event Tables — a real table and an event table cannot share
	// a name in one schema.
	basic = dedupeEventTables(basic, extended)
	// Materialized views are listed explicitly by ListExtendedObjects (kind
	// "MATERIALIZED VIEW"). They can also surface in SHOW OBJECTS (typically as a
	// VIEW), so drop any (schema, name) collision to avoid duplicate tree entries
	// — a regular view and a materialized view cannot share a name in one schema.
	basic = dedupeMaterializedViews(basic, extended)
	// slices.Concat allocates a fresh backing array rather than appending into
	// basic's spare capacity: when neither dedupe reallocated, basic can still be
	// the "basic"-cache backing slice, and appending into it would alias that
	// cached entry's storage with this "full"-cache entry.
	all := slices.Concat(basic, extended)
	c.putObjectCache(cacheKey, all)
	return all, nil
}

// dedupeByExtendedKind removes from basic any object whose (schema, name)
// matches an extended object of the given kind, preventing duplicate tree nodes
// when SHOW OBJECTS surfaces an object that ListExtendedObjects already returned
// under a dedicated kind. Matching is case-insensitive.
func dedupeByExtendedKind(basic, extended []SnowflakeObject, kind string) []SnowflakeObject {
	keys := make(map[string]struct{})
	for _, o := range extended {
		if o.Kind == kind {
			keys[strings.ToUpper(o.Schema)+"\x00"+strings.ToUpper(o.Name)] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return basic
	}
	// Allocate a fresh slice — basic may be the cached backing array from
	// ListBasicObjects, so it must not be mutated in place.
	out := make([]SnowflakeObject, 0, len(basic))
	for _, o := range basic {
		if _, dup := keys[strings.ToUpper(o.Schema)+"\x00"+strings.ToUpper(o.Name)]; dup {
			continue
		}
		out = append(out, o)
	}
	return out
}

// dedupeExternalTables removes from basic any object whose (schema, name) matches
// an extended object of kind "EXTERNAL TABLE". See dedupeByExtendedKind.
func dedupeExternalTables(basic, extended []SnowflakeObject) []SnowflakeObject {
	return dedupeByExtendedKind(basic, extended, "EXTERNAL TABLE")
}

// dedupeDynamicTables removes from basic any object whose (schema, name) matches
// an extended object of kind "DYNAMIC TABLE". See dedupeByExtendedKind.
func dedupeDynamicTables(basic, extended []SnowflakeObject) []SnowflakeObject {
	return dedupeByExtendedKind(basic, extended, "DYNAMIC TABLE")
}

// dedupeMaterializedViews removes from basic any object whose (schema, name)
// matches an extended object of kind "MATERIALIZED VIEW". See dedupeByExtendedKind.
func dedupeMaterializedViews(basic, extended []SnowflakeObject) []SnowflakeObject {
	return dedupeByExtendedKind(basic, extended, "MATERIALIZED VIEW")
}

// dedupeIcebergTables removes from basic any object whose (schema, name) matches
// an extended object of kind "ICEBERG TABLE". See dedupeByExtendedKind.
func dedupeIcebergTables(basic, extended []SnowflakeObject) []SnowflakeObject {
	return dedupeByExtendedKind(basic, extended, "ICEBERG TABLE")
}

// dedupeHybridTables removes from basic any object whose (schema, name) matches
// an extended object of kind "HYBRID TABLE". See dedupeByExtendedKind.
func dedupeHybridTables(basic, extended []SnowflakeObject) []SnowflakeObject {
	return dedupeByExtendedKind(basic, extended, "HYBRID TABLE")
}

// dedupeEventTables removes from basic any object whose (schema, name) matches
// an extended object of kind "EVENT TABLE". See dedupeByExtendedKind.
func dedupeEventTables(basic, extended []SnowflakeObject) []SnowflakeObject {
	return dedupeByExtendedKind(basic, extended, "EVENT TABLE")
}

// getObjectCache returns a cached result if it exists and hasn't expired.
// The returned slice is a shallow clone so callers can safely append without
// corrupting the cached backing array. Expired entries are deleted on access.
func (c *Client) getObjectCache(key string) ([]SnowflakeObject, bool) {
	c.objectCacheMu.RLock()
	entry, ok := c.objectCache[key]
	if !ok {
		c.objectCacheMu.RUnlock()
		return nil, false
	}
	if time.Since(entry.ts) > objectCacheTTL {
		c.objectCacheMu.RUnlock()
		c.objectCacheMu.Lock()
		// Re-check: another goroutine may have written a fresh entry
		// between RUnlock and Lock.
		if e, ok := c.objectCache[key]; ok && time.Since(e.ts) > objectCacheTTL {
			delete(c.objectCache, key)
		}
		c.objectCacheMu.Unlock()
		return nil, false
	}
	result := slices.Clone(entry.objects)
	c.objectCacheMu.RUnlock()
	return result, true
}

// putObjectCache stores a result in the cache with the current timestamp.
func (c *Client) putObjectCache(key string, objects []SnowflakeObject) {
	c.objectCacheMu.Lock()
	defer c.objectCacheMu.Unlock()
	c.objectCache[key] = objectCacheEntry{objects: objects, ts: time.Now()}
}

// ClearObjectCache removes all cached object listings.
func (c *Client) ClearObjectCache() {
	c.objectCacheMu.Lock()
	defer c.objectCacheMu.Unlock()
	c.objectCache = make(map[string]objectCacheEntry)
}

// ClearObjectCacheForDatabase removes all cached object listings whose key
// contains the given database (both full and basic-only entries).
func (c *Client) ClearObjectCacheForDatabase(database string) {
	c.objectCacheMu.Lock()
	defer c.objectCacheMu.Unlock()
	fullPrefix := "full\x00" + database + "\x00"
	basicPrefix := "basic\x00" + database + "\x00"
	for k := range c.objectCache {
		if strings.HasPrefix(k, fullPrefix) || strings.HasPrefix(k, basicPrefix) {
			delete(c.objectCache, k)
		}
	}
}

// ClearObjectCacheForSchema removes cached object listings for a specific schema.
func (c *Client) ClearObjectCacheForSchema(database, schema string) {
	c.objectCacheMu.Lock()
	defer c.objectCacheMu.Unlock()
	delete(c.objectCache, "full\x00"+database+"\x00"+schema)
	delete(c.objectCache, "basic\x00"+database+"\x00"+schema)
}

// ListFileFormats returns the names of all file formats in the specified schema.
func (c *Client) ListFileFormats(ctx context.Context, database, schema string) ([]string, error) {
	q := Qualify(database, schema)
	return c.queryStringSlice(ctx, fmt.Sprintf("SHOW FILE FORMATS IN SCHEMA %s", q), 1)
}

// GetObjectDDL returns the DDL definition of a Snowflake object using GET_DDL.
//
// For account-level objects (warehouses, databases, etc.) pass empty strings
// for database and schema — the name is used unqualified.  For schema-scoped
// objects pass the owning database and schema so the function builds a fully
// qualified '<db>.<schema>.<name>' identifier.
//
// For procedures and functions the arguments parameter must contain the
// parameter type list (e.g. "NUMBER, VARCHAR") so that Snowflake can resolve
// the correct overload. Pass an empty string for all other object kinds.
func (c *Client) GetObjectDDL(ctx context.Context, database, schema, kind, name, arguments string) (string, error) {
	// GET_DDL has no object type for these kinds: buildGetDDLQuery would fall
	// through to ddlKind := kind and emit an invalid GET_DDL('<kind>', …). They
	// are already excluded at every frontend entry point (hover DDL, View
	// Definition, comparison); guard here too so a future caller can't trip the
	// invalid query (a packages-policy GET_DDL fails with "Cannot initialize
	// Snowflake Metadata. Dictionary unavailable").
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case "IMAGE REPOSITORY", "SERVICE", "GATEWAY", "PACKAGES POLICY", "MODEL", "MODEL MONITOR", "DATASET", "CORTEX SEARCH SERVICE", "EXTERNAL AGENT", "MCP SERVER":
		return "", fmt.Errorf("GET_DDL does not support %s objects", kind)
	}

	query, identifier := buildGetDDLQuery(database, schema, kind, name, arguments)

	row := c.queryRowCtx(ctx, query)
	var src string
	if err := row.Scan(&src); err != nil {
		return "", fmt.Errorf("GET_DDL(%s %s): %w", kind, identifier, err)
	}
	return src, nil
}

// buildGetDDLQuery constructs the GET_DDL SQL query and returns both the query
// string and the identifier used (for error messages).
func buildGetDDLQuery(database, schema, kind, name, arguments string) (query, identifier string) {
	// Account-level objects (warehouses, databases, etc.) use an unqualified
	// name; schema-scoped objects use a fully qualified database.schema.name.
	if database == "" && schema == "" {
		identifier = EscapeStringLit(name)
	} else {
		identifier = Qualify(database, schema, name)
		// Procedures and functions require the argument type list (which may be
		// empty for zero-arg procedures) appended so Snowflake can resolve the
		// overload.  Omitting the parentheses entirely causes GET_DDL to return
		// "Object does not exist" even when the procedure exists.
		//
		// Safety: arguments is interpolated into identifier, which is then
		// single-quote-escaped below before embedding in the GET_DDL string
		// literal. Any single quotes in arguments are doubled, preventing
		// breakout from the SQL string context. If this code is refactored,
		// ensure arguments still passes through the same single-quote escaping.
		upperKind := strings.ToUpper(kind)
		if upperKind == "PROCEDURE" || upperKind == "FUNCTION" || upperKind == "EXTERNAL FUNCTION" || upperKind == "DATA METRIC FUNCTION" {
			identifier += fmt.Sprintf("(%s)", arguments)
		}
		identifier = EscapeStringLit(identifier)
	}
	// GET_DDL expects the underscore form (e.g. 'DYNAMIC_TABLE',
	// 'EXTERNAL_TABLE') as the object_type, whereas the rest of the app uses the
	// space-separated SHOW kind ("DYNAMIC TABLE", "EXTERNAL TABLE"). Normalize it
	// here so DDL export works for these object kinds.
	ddlKind := kind
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case "DYNAMIC TABLE":
		ddlKind = "DYNAMIC_TABLE"
	case "EXTERNAL TABLE":
		ddlKind = "EXTERNAL_TABLE"
	case "ICEBERG TABLE":
		// GET_DDL has no ICEBERG_TABLE object type — Iceberg tables are
		// retrieved via the 'TABLE' type.
		ddlKind = "TABLE"
	case "HYBRID TABLE":
		// GET_DDL has no HYBRID_TABLE object type — hybrid tables are
		// retrieved via the 'TABLE' type.
		ddlKind = "TABLE"
	case "EVENT TABLE":
		// GET_DDL exposes a dedicated EVENT_TABLE object type (the SHOW kind
		// is space-separated; the GET_DDL object_type uses the underscore form).
		ddlKind = "EVENT_TABLE"
	case "MATERIALIZED VIEW":
		// GET_DDL has no MATERIALIZED_VIEW object type — TABLE and VIEW are
		// interchangeable and materialized views are retrieved via 'VIEW'.
		ddlKind = "VIEW"
	case "MASKING POLICY", "ROW ACCESS POLICY", "JOIN POLICY", "PRIVACY POLICY", "STORAGE LIFECYCLE POLICY", "PASSWORD POLICY", "SESSION POLICY", "AGGREGATION POLICY", "PROJECTION POLICY", "AUTHENTICATION POLICY":
		// GET_DDL exposes a single 'POLICY' object type covering most policy
		// kinds (masking, row access, join, privacy, storage lifecycle, password,
		// session, aggregation, projection, authentication, etc.), not a per-kind type. NOTE: packages policies are
		// deliberately NOT here — GET_DDL supports neither the 'POLICY' nor a
		// 'PACKAGES POLICY' object type for them (the call fails with "Cannot
		// initialize Snowflake Metadata. Dictionary unavailable"), so packages
		// policies have no GET_DDL mapping at all, handled like image repositories
		// and services.
		ddlKind = "POLICY"
	case "NETWORK RULE":
		ddlKind = "NETWORK_RULE"
	case "AGENT":
		// GET_DDL exposes Cortex agents under the CORTEX_AGENT object type (the
		// SHOW kind is "AGENT"). External agents have no GET_DDL object type and
		// are excluded in GetObjectDDL.
		ddlKind = "CORTEX_AGENT"
	case "EXTERNAL FUNCTION":
		// GET_DDL has no EXTERNAL_FUNCTION object type — external functions are
		// retrieved via the 'FUNCTION' type (with the argument signature appended
		// to the identifier above).
		ddlKind = "FUNCTION"
	case "DATA METRIC FUNCTION":
		// GET_DDL has no DATA_METRIC_FUNCTION object type — data metric functions
		// are retrieved via the 'FUNCTION' type (with the TABLE argument signature
		// appended to the identifier above).
		ddlKind = "FUNCTION"
	}
	escapedKind := EscapeStringLit(ddlKind)
	// The third argument (true) enables recursive DDL output for objects that
	// contain dependents (e.g. databases → schemas → tables).  For object types
	// without dependents (e.g. warehouses) Snowflake silently ignores it, so
	// passing true unconditionally is safe and keeps the code path simple.
	query = fmt.Sprintf("SELECT GET_DDL('%s', '%s', true)", escapedKind, identifier)
	return query, identifier
}

// GetCompleteDatabaseDDL returns the full DDL for a database in a single
// GET_DDL('DATABASE', ..., true) call.
//
// Modern Snowflake's database-level GET_DDL covers all object types:
// schemas, tables, views, sequences, functions, procedures, stages, streams,
// tasks, file formats, and pipes. One round-trip per database is therefore
// sufficient and is significantly faster than the previous approach of
// supplementing with per-schema SHOW + individual GET_DDL calls.
func (c *Client) GetCompleteDatabaseDDL(ctx context.Context, database string) (string, error) {
	return c.GetDatabaseDDL(ctx, database)
}

// GetDatabaseDDL returns the complete DDL for a database using Snowflake's
// GET_DDL function.  The result is a single SQL string containing CREATE
// statements for every object in the database (schemas, tables, views,
// functions, procedures, sequences, stages, streams, tasks, file formats,
// pipes).
//
// The database name is safely escaped to prevent SQL injection.
func (c *Client) GetDatabaseDDL(ctx context.Context, database string) (string, error) {
	return c.GetObjectDDL(ctx, "", "", "DATABASE", database, "")
}

// loadPrivateKey reads a PEM-encoded RSA private key from disk.
// If passphrase is non-empty, the key is assumed to be encrypted.
// splitCommaList splits a comma-separated string into a trimmed, non-empty
// slice. Returns nil when the input contains no meaningful entries.
func splitCommaList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func loadPrivateKey(path, passphrase string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	var der []byte
	if passphrase != "" {
		//nolint:staticcheck // x509.DecryptPEMBlock is deprecated but intentional here
		der, err = x509.DecryptPEMBlock(block, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("decrypt PEM block: %w", err)
		}
	} else {
		der = block.Bytes
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		// Fallback: try PKCS#1 format
		return x509.ParsePKCS1PrivateKey(der)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key in %s is not an RSA key", path)
	}
	return rsaKey, nil
}

// ERColumn describes a single column in an ER diagram entity.
type ERColumn struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	IsPK     bool   `json:"isPK"`
	Nullable string `json:"nullable"`
}

// ERTable represents a table (entity) in an ER diagram.
type ERTable struct {
	Schema  string     `json:"schema"`
	Name    string     `json:"name"`
	Columns []ERColumn `json:"columns"`
}

// ERForeignKey represents a foreign-key relationship between two tables.
type ERForeignKey struct {
	FromSchema string `json:"fromSchema"`
	FromTable  string `json:"fromTable"`
	FromCol    string `json:"fromCol"`
	ToSchema   string `json:"toSchema"`
	ToTable    string `json:"toTable"`
	ToCol      string `json:"toCol"`
}

// ERDiagramData is the full payload sent to the frontend to render an ER diagram.
type ERDiagramData struct {
	Database string         `json:"database"`
	Tables   []ERTable      `json:"tables"`
	FKs      []ERForeignKey `json:"fks"`
}

// GetERDiagramData fetches column metadata, primary keys, and foreign keys for
// every user table in the database concurrently and returns the data needed to
// render an Entity Relationship Diagram.
func (c *Client) GetERDiagramData(ctx context.Context, database string) (ERDiagramData, error) {
	db := QuoteIdent(database)

	type colRow struct {
		tableSchema, tableName, columnName, dataType, isNullable string
	}
	type pkRow struct {
		schema, table, column string
	}
	type fkRow struct {
		fromSchema, fromTable, fromCol, toSchema, toTable, toCol string
	}

	var (
		colRows []colRow
		pkRows  []pkRow
		fkRows  []fkRow
		colErr  error
		pkErr   error
		fkErr   error
		wg      sync.WaitGroup
	)

	// Fetch column metadata
	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf(
			`SELECT c.TABLE_SCHEMA, c.TABLE_NAME, c.COLUMN_NAME, c.DATA_TYPE, c.IS_NULLABLE`+
				` FROM %s.INFORMATION_SCHEMA.COLUMNS c`+
				` JOIN %s.INFORMATION_SCHEMA.TABLES t`+
				` ON c.TABLE_SCHEMA = t.TABLE_SCHEMA AND c.TABLE_NAME = t.TABLE_NAME`+
				` WHERE c.TABLE_SCHEMA != 'INFORMATION_SCHEMA'`+
				` AND t.TABLE_TYPE = 'BASE TABLE'`+
				` ORDER BY c.TABLE_SCHEMA, c.TABLE_NAME, c.ORDINAL_POSITION`, db, db)
		rows, err := c.queryCtx(ctx, query)
		if err != nil {
			colErr = err
			return
		}
		defer rows.Close() //nolint:errcheck
		for rows.Next() {
			var r colRow
			if err := rows.Scan(&r.tableSchema, &r.tableName, &r.columnName, &r.dataType, &r.isNullable); err != nil {
				continue
			}
			colRows = append(colRows, r)
		}
		colErr = rows.Err()
	}()

	// Fetch primary keys
	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`SHOW PRIMARY KEYS IN DATABASE %s`, db)
		rows, err := c.queryCtx(ctx, query)
		if err != nil {
			pkErr = err
			return
		}
		defer rows.Close() //nolint:errcheck
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "schema_name", "table_name", "column_name")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if err := rows.Scan(ptrs...); err != nil {
				continue
			}
			pkRows = append(pkRows, pkRow{
				schema: strVal(vals, idxs["schema_name"]),
				table:  strVal(vals, idxs["table_name"]),
				column: strVal(vals, idxs["column_name"]),
			})
		}
	}()

	// Fetch foreign keys (imported keys = FK child side)
	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`SHOW IMPORTED KEYS IN DATABASE %s`, db)
		rows, err := c.queryCtx(ctx, query)
		if err != nil {
			fkErr = err
			return
		}
		defer rows.Close() //nolint:errcheck
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "fk_schema_name", "fk_table_name", "fk_column_name", "pk_schema_name", "pk_table_name", "pk_column_name")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if err := rows.Scan(ptrs...); err != nil {
				continue
			}
			fkRows = append(fkRows, fkRow{
				fromSchema: strVal(vals, idxs["fk_schema_name"]),
				fromTable:  strVal(vals, idxs["fk_table_name"]),
				fromCol:    strVal(vals, idxs["fk_column_name"]),
				toSchema:   strVal(vals, idxs["pk_schema_name"]),
				toTable:    strVal(vals, idxs["pk_table_name"]),
				toCol:      strVal(vals, idxs["pk_column_name"]),
			})
		}
	}()

	wg.Wait()

	if colErr != nil {
		return ERDiagramData{}, colErr
	}

	// Build PK lookup: "schema\x00table\x00column" → true
	pkSet := make(map[string]bool)
	if pkErr == nil {
		for _, r := range pkRows {
			pkSet[r.schema+"\x00"+r.table+"\x00"+r.column] = true
		}
	}

	// Aggregate columns into tables
	type tableKey struct{ schema, name string }
	tablesMap := make(map[tableKey]*ERTable)
	for _, r := range colRows {
		k := tableKey{r.tableSchema, r.tableName}
		if _, ok := tablesMap[k]; !ok {
			tablesMap[k] = &ERTable{Schema: r.tableSchema, Name: r.tableName, Columns: []ERColumn{}}
		}
		isPK := pkSet[r.tableSchema+"\x00"+r.tableName+"\x00"+r.columnName]
		tablesMap[k].Columns = append(tablesMap[k].Columns, ERColumn{
			Name:     r.columnName,
			DataType: r.dataType,
			IsPK:     isPK,
			Nullable: r.isNullable,
		})
	}

	// Sort tables deterministically by schema then name
	keys := make([]tableKey, 0, len(tablesMap))
	for k := range tablesMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].schema != keys[j].schema {
			return keys[i].schema < keys[j].schema
		}
		return keys[i].name < keys[j].name
	})

	tables := make([]ERTable, 0, len(keys))
	for _, k := range keys {
		tables = append(tables, *tablesMap[k])
	}

	// Build FK list (non-fatal if SHOW IMPORTED KEYS failed)
	var fks []ERForeignKey
	if fkErr == nil {
		for _, r := range fkRows {
			fks = append(fks, ERForeignKey{
				FromSchema: r.fromSchema,
				FromTable:  r.fromTable,
				FromCol:    r.fromCol,
				ToSchema:   r.toSchema,
				ToTable:    r.toTable,
				ToCol:      r.toCol,
			})
		}
	}

	return ERDiagramData{
		Database: database,
		Tables:   tables,
		FKs:      fks,
	}, nil
}

// queryStringSlice is a helper that reads a single string column from a SHOW command.
func (c *Client) queryStringSlice(ctx context.Context, query string, colIdx int) ([]string, error) {
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	result := []string{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		result = append(result, fmt.Sprintf("%v", vals[colIdx]))
	}
	return result, rows.Err()
}

// ── Table data export ─────────────────────────────────────────────────────────

// ExportTableParams specifies how to export a Snowflake table to the local machine.
type ExportTableParams struct {
	Database    string `json:"database"`
	Schema      string `json:"schema"`
	Table       string `json:"table"`
	OutputDir   string `json:"outputDir"`
	Format      string `json:"format"`      // "CSV", "JSON", "PARQUET"
	Compression string `json:"compression"` // "NONE", "GZIP", "BZIP2", "SNAPPY"
	// CSV-specific
	Delimiter  string `json:"delimiter"`  // ",", "|", "\t", or a custom character
	Header     bool   `json:"header"`     // include column names as first row
	NullString string `json:"nullString"` // how NULL is represented: "", "\\N", "NULL"
}

// ExportTableResult reports the outcome of a table export.
type ExportTableResult struct {
	RowsUnloaded int64    `json:"rowsUnloaded"`
	Files        []string `json:"files"`     // base file names inside OutputDir
	OutputDir    string   `json:"outputDir"` // absolute path to the output sub-directory
}

// ExportTableData exports a Snowflake table to the local filesystem using a
// temporary internal stage:
//  1. CREATE TEMPORARY STAGE in the same schema as the table
//  2. COPY INTO @stage  (writes Snowflake-side files)
//  3. GET @stage        (downloads files to outputDir/<TABLE>/)
//  4. DROP STAGE        (explicit cleanup; the deferred call also fires on error)
func (c *Client) ExportTableData(ctx context.Context, params ExportTableParams) (ExportTableResult, error) {

	// Unique stage name — timestamp-based, no external uuid package required
	stageName := fmt.Sprintf("THAW_EXPORT_%d", time.Now().UnixNano())
	stageRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(params.Database), QuoteIdent(params.Schema), stageName)
	stageAt := "@" + stageRef
	tableRef := Qualify(params.Database, params.Schema, params.Table)

	// Create a temporary stage (auto-dropped when the session ends)
	if _, err := c.execCtx(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return ExportTableResult{}, fmt.Errorf("create export stage: %w", err)
	}
	// Explicit cleanup — also runs on error paths
	defer c.execCtx(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// COPY data into the stage
	copySQL := buildExportCopySQL(stageAt, tableRef, params)
	copyRows, err := c.queryCtx(ctx, copySQL)
	if err != nil {
		return ExportTableResult{}, fmt.Errorf("copy into stage: %w", err)
	}
	var rowsUnloaded int64
	if copyRows.Next() {
		_ = copyRows.Scan(&rowsUnloaded) // first column is rows_unloaded
	}
	copyRows.Close() //nolint:errcheck

	// Create the output sub-directory: <outputDir>/<TABLE>/
	outDir := filepath.Join(params.OutputDir, params.Table)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return ExportTableResult{}, fmt.Errorf("create output directory: %w", err)
	}

	// Build a file:// URL pointing at the output directory
	fileURL, err := localFileURL(outDir)
	if err != nil {
		return ExportTableResult{}, fmt.Errorf("build file url: %w", err)
	}

	// GET @stage → local directory
	getSQL := fmt.Sprintf("GET %s '%s'", stageAt, fileURL)
	getRows, err := c.queryCtx(ctx, getSQL)
	if err != nil {
		return ExportTableResult{}, fmt.Errorf("download files from stage: %w", err)
	}
	getCols, _ := getRows.Columns()
	var files []string
	for getRows.Next() {
		vals, ptrs := makeValPtrs(len(getCols))
		if err := getRows.Scan(ptrs...); err != nil {
			continue
		}
		// Result columns: file, size, status, message
		fileName := fmt.Sprintf("%v", vals[0])
		status := ""
		if len(vals) > 2 {
			status = strings.ToUpper(fmt.Sprintf("%v", vals[2]))
		}
		if status == "DOWNLOADED" || status == "" {
			files = append(files, filepath.Base(fileName))
		}
	}
	getRows.Close() //nolint:errcheck

	return ExportTableResult{
		RowsUnloaded: rowsUnloaded,
		Files:        files,
		OutputDir:    outDir,
	}, nil
}

// buildExportCopySQL constructs the COPY INTO <stage> SQL for the given params.
func buildExportCopySQL(stageAt, tableRef string, p ExportTableParams) string {
	comp := strings.ToUpper(p.Compression)
	if comp == "" {
		comp = "NONE"
	}

	var ff strings.Builder
	switch strings.ToUpper(p.Format) {
	case "JSON":
		fmt.Fprintf(&ff, "TYPE = 'JSON' COMPRESSION = %s", comp)
	case "PARQUET":
		snappy := "FALSE"
		if comp == "SNAPPY" {
			snappy = "TRUE"
		}
		fmt.Fprintf(&ff, "TYPE = 'PARQUET' SNAPPY_COMPRESSION = %s", snappy)
	default: // CSV
		delim := p.Delimiter
		if delim == "" {
			delim = ","
		}
		delim = strings.ReplaceAll(delim, "'", "\\'")
		nullIf := strings.ReplaceAll(p.NullString, "'", "\\'")
		fmt.Fprintf(&ff,
			"TYPE = 'CSV' FIELD_DELIMITER = '%s' FIELD_OPTIONALLY_ENCLOSED_BY = '\"' NULL_IF = ('%s') EMPTY_FIELD_AS_NULL = TRUE COMPRESSION = %s",
			delim, nullIf, comp)
	}

	// JSON and PARQUET require the source to be a query returning a single
	// VARIANT column; OBJECT_CONSTRUCT(*) converts each row to a JSON object.
	source := tableRef
	if f := strings.ToUpper(p.Format); f == "JSON" || f == "PARQUET" {
		source = fmt.Sprintf("(SELECT OBJECT_CONSTRUCT(*) FROM %s)", tableRef)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "COPY INTO %s\nFROM %s\nFILE_FORMAT = (\n    %s\n)\nOVERWRITE = TRUE", stageAt, source, ff.String())
	if strings.ToUpper(p.Format) == "CSV" && p.Header {
		sb.WriteString("\nHEADER = TRUE")
	}
	return sb.String()
}

// localFileURL converts an absolute local directory path to a file:// URL
// suitable for the gosnowflake GET command on all platforms.
//   - Unix:    /tmp/dir  → file:///tmp/dir/
//   - Windows: C:\dir    → file:///C:/dir/
func localFileURL(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	slashed := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed // Windows: "C:/..." → "/C:/..."
	}
	if !strings.HasSuffix(slashed, "/") {
		slashed += "/"
	}
	return "file://" + slashed, nil
}

// localFileURLForFile converts an absolute local file path to a file:// URL
// suitable for the gosnowflake PUT command on all platforms.
//   - Unix:    /tmp/data.csv   → file:///tmp/data.csv
//   - Windows: C:\data.csv     → file:///C:/data.csv
func localFileURLForFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	slashed := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed, nil
}

// ── Table data import ─────────────────────────────────────────────────────────

// FormatTypeOptions holds all Snowflake file-format options for a given type.
// Fields that are unused by a particular format are ignored by buildFFOptions.
// String values use SQL escape notation (e.g. "\n" for newline, "\\" for backslash).
// Keyword values (AUTO, NONE, HEX, …) are stored as their uppercase names.
type FormatTypeOptions struct {
	// ── Common ───────────────────────────────────────────────────────────────
	Compression              string   `json:"compression"` // AUTO | GZIP | BZ2 | …
	TrimSpace                bool     `json:"trimSpace"`
	ReplaceInvalidCharacters bool     `json:"replaceInvalidCharacters"`
	NullIf                   []string `json:"nullIf"` // list of null-indicator strings

	// ── CSV + JSON ───────────────────────────────────────────────────────────
	DateFormat        string `json:"dateFormat"` // AUTO | format-string
	TimeFormat        string `json:"timeFormat"`
	TimestampFormat   string `json:"timestampFormat"`
	BinaryFormat      string `json:"binaryFormat"`  // HEX | BASE64 | UTF8
	FileExtension     string `json:"fileExtension"` // NONE or extension string
	MultiLine         bool   `json:"multiLine"`
	SkipByteOrderMark bool   `json:"skipByteOrderMark"`
	IgnoreUtf8Errors  bool   `json:"ignoreUtf8Errors"` // JSON

	// ── CSV ──────────────────────────────────────────────────────────────────
	RecordDelimiter            string `json:"recordDelimiter"`
	FieldDelimiter             string `json:"fieldDelimiter"`
	ParseHeader                bool   `json:"parseHeader"`
	SkipHeader                 int    `json:"skipHeader"`
	SkipBlankLines             bool   `json:"skipBlankLines"`
	Escape                     string `json:"escape"` // NONE or char
	EscapeUnenclosedField      string `json:"escapeUnenclosedField"`
	FieldOptionallyEnclosedBy  string `json:"fieldOptionallyEnclosedBy"` // NONE or char
	ErrorOnColumnCountMismatch bool   `json:"errorOnColumnCountMismatch"`
	EmptyFieldAsNull           bool   `json:"emptyFieldAsNull"`
	Encoding                   string `json:"encoding"` // UTF8 | …

	// ── JSON-only ────────────────────────────────────────────────────────────
	EnableOctal     bool `json:"enableOctal"`
	AllowDuplicate  bool `json:"allowDuplicate"`
	StripOuterArray bool `json:"stripOuterArray"`
	StripNullValues bool `json:"stripNullValues"`

	// ── PARQUET ──────────────────────────────────────────────────────────────
	SnappyCompression    bool `json:"snappyCompression"`
	BinaryAsText         bool `json:"binaryAsText"`
	UseLogicalType       bool `json:"useLogicalType"`
	UseVectorizedScanner bool `json:"useVectorizedScanner"`
}

// ImportTableParams specifies how to import one or more local files into a Snowflake table.
type ImportTableParams struct {
	Database  string   `json:"database"`
	Schema    string   `json:"schema"`
	Table     string   `json:"table"`     // target table name
	FilePaths []string `json:"filePaths"` // one or more absolute local paths
	Format    string   `json:"format"`    // "CSV", "JSON", "AVRO", "ORC", "PARQUET"
	// Behavior
	Overwrite   bool `json:"overwrite"`   // TRUNCATE TABLE before COPY INTO
	CreateTable bool `json:"createTable"` // CREATE TABLE using INFER_SCHEMA first
	// Format-specific options (see FormatTypeOptions)
	Options FormatTypeOptions `json:"options"`

	// NamedFormat is the name of an existing file format in the target schema.
	// If set, the COPY INTO statement will use this format instead of inline options.
	NamedFormat string `json:"namedFormat"`
}

// ImportTableResult reports the outcome of a table import.
type ImportTableResult struct {
	RowsLoaded  int64 `json:"rowsLoaded"`
	FilesLoaded int   `json:"filesLoaded"`
}

// ImportTableData imports one or more local files into a Snowflake table via a
// temporary internal stage:
//  1. CREATE TEMPORARY STAGE in the same schema as the table
//  2. PUT each local file → stage
//  3. Optionally CREATE TABLE USING TEMPLATE (INFER_SCHEMA) or TRUNCATE
//  4. COPY INTO table FROM @stage  (loads all staged files in one pass)
//  5. DROP STAGE (deferred)
func (c *Client) ImportTableData(ctx context.Context, params ImportTableParams) (ImportTableResult, error) {

	if len(params.FilePaths) == 0 {
		return ImportTableResult{}, fmt.Errorf("no files specified")
	}

	stageName := fmt.Sprintf("THAW_IMPORT_%d", time.Now().UnixNano())
	stageRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(params.Database), QuoteIdent(params.Schema), stageName)
	stageAt := "@" + stageRef
	tableRef := Qualify(params.Database, params.Schema, params.Table)

	if _, err := c.execCtx(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return ImportTableResult{}, fmt.Errorf("create import stage: %w", err)
	}
	defer c.execCtx(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// PUT each local file to the stage.
	for _, fp := range params.FilePaths {
		fileURL, err := localFileURLForFile(fp)
		if err != nil {
			return ImportTableResult{}, fmt.Errorf("build file url for %s: %w", filepath.Base(fp), err)
		}
		escapedURL := strings.ReplaceAll(fileURL, "'", "\\'")
		putSQL := fmt.Sprintf("PUT '%s' %s AUTO_COMPRESS=FALSE OVERWRITE=TRUE", escapedURL, stageAt)
		putRows, err := c.queryCtx(ctx, putSQL)
		if err != nil {
			return ImportTableResult{}, fmt.Errorf("upload %s to stage: %w", filepath.Base(fp), err)
		}
		putRows.Close() //nolint:errcheck
	}

	// Optionally create the target table from the file's inferred schema.
	// INFER_SCHEMA does not support PARSE_HEADER / FIELD_DELIMITER as inline
	// FILE_FORMAT options, so for CSV with PARSE_HEADER=TRUE we create a named
	// file format, reference it by name, and drop it when done.
	var inferFmtRef string
	if params.CreateTable && strings.ToUpper(params.Format) == "CSV" && params.Options.ParseHeader {
		fmtName := fmt.Sprintf("THAW_IMPORT_FF_%d", time.Now().UnixNano())
		fmtRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(params.Database), QuoteIdent(params.Schema), fmtName)
		createFmtSQL := fmt.Sprintf("CREATE OR REPLACE FILE FORMAT %s %s", fmtRef, buildFFOptions(params.Format, params.Options))
		if _, err := c.execCtx(ctx, createFmtSQL); err != nil {
			return ImportTableResult{}, fmt.Errorf("create file format: %w", err)
		}
		defer c.execCtx(context.Background(), "DROP FILE FORMAT IF EXISTS "+fmtRef) //nolint:errcheck
		inferFmtRef = fmtRef
	}

	if params.CreateTable {
		createSQL := buildCreateTableSQL(stageAt, tableRef, inferFmtRef, params)
		if _, err := c.execCtx(ctx, createSQL); err != nil {
			return ImportTableResult{}, fmt.Errorf("create table: %w", err)
		}
	} else if params.Overwrite {
		if _, err := c.execCtx(ctx, "TRUNCATE TABLE IF EXISTS "+tableRef); err != nil {
			return ImportTableResult{}, fmt.Errorf("truncate table: %w", err)
		}
	}

	copySQL := buildImportCopySQL(stageAt, tableRef, params)
	copyRows, err := c.queryCtx(ctx, copySQL)
	if err != nil {
		return ImportTableResult{}, fmt.Errorf("copy into table: %w", err)
	}
	// COPY INTO result columns: file, status, rows_parsed, rows_loaded, …
	cols, _ := copyRows.Columns()
	var rowsLoaded int64
	var filesLoaded int
	for copyRows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := copyRows.Scan(ptrs...); err != nil {
			continue
		}
		filesLoaded++
		if len(vals) > 3 {
			switch v := vals[3].(type) {
			case int64:
				rowsLoaded += v
			case string:
				n, _ := strconv.ParseInt(v, 10, 64)
				rowsLoaded += n
			}
		}
	}
	copyRows.Close() //nolint:errcheck

	return ImportTableResult{RowsLoaded: rowsLoaded, FilesLoaded: filesLoaded}, nil
}

// ── File-format SQL helpers ───────────────────────────────────────────────────

// ffKeywords are Snowflake file-format option values that must not be quoted.
var ffKeywords = map[string]struct{}{
	"AUTO": {}, "GZIP": {}, "BZ2": {}, "BROTLI": {}, "ZSTD": {}, "DEFLATE": {},
	"RAW_DEFLATE": {}, "NONE": {}, "LZO": {}, "SNAPPY": {},
	"HEX": {}, "BASE64": {}, "UTF8": {},
}

// ffVal formats a file-format string option value: keyword values are returned
// unquoted (uppercase), all others are wrapped in single quotes.
// The string is assumed to use SQL escape notation (e.g. \n, \\).
func ffVal(s string) string {
	up := strings.ToUpper(s)
	if _, ok := ffKeywords[up]; ok {
		return up
	}
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}

func ffBool(v bool) string {
	if v {
		return "TRUE"
	}
	return "FALSE"
}

// ffNullIf formats the NULL_IF clause, e.g. NULL_IF = ('\N', 'NULL').
func ffNullIf(nullIf []string) string {
	if len(nullIf) == 0 {
		return "NULL_IF = ()"
	}
	parts := make([]string, len(nullIf))
	for i, s := range nullIf {
		parts[i] = "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
	}
	return "NULL_IF = (" + strings.Join(parts, ", ") + ")"
}

// buildFFOptions returns the TYPE='…' option string for a Snowflake file format.
// It does NOT include surrounding parentheses; callers add those as needed.
func buildFFOptions(format string, o FormatTypeOptions) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(" "); b.WriteString(s) }

	switch strings.ToUpper(format) {
	case "CSV":
		b.WriteString("TYPE='CSV'")
		w("COMPRESSION=" + ffVal(o.Compression))
		w("RECORD_DELIMITER=" + ffVal(o.RecordDelimiter))
		w("FIELD_DELIMITER=" + ffVal(o.FieldDelimiter))
		w("MULTI_LINE=" + ffBool(o.MultiLine))
		if o.ParseHeader {
			w("PARSE_HEADER=TRUE")
		} else {
			w(fmt.Sprintf("SKIP_HEADER=%d", o.SkipHeader))
		}
		w("SKIP_BLANK_LINES=" + ffBool(o.SkipBlankLines))
		w("DATE_FORMAT=" + ffVal(o.DateFormat))
		w("TIME_FORMAT=" + ffVal(o.TimeFormat))
		w("TIMESTAMP_FORMAT=" + ffVal(o.TimestampFormat))
		w("BINARY_FORMAT=" + ffVal(o.BinaryFormat))
		w("ESCAPE=" + ffVal(o.Escape))
		w("ESCAPE_UNENCLOSED_FIELD=" + ffVal(o.EscapeUnenclosedField))
		w("TRIM_SPACE=" + ffBool(o.TrimSpace))
		w("FIELD_OPTIONALLY_ENCLOSED_BY=" + ffVal(o.FieldOptionallyEnclosedBy))
		w(ffNullIf(o.NullIf))
		w("ERROR_ON_COLUMN_COUNT_MISMATCH=" + ffBool(o.ErrorOnColumnCountMismatch))
		w("REPLACE_INVALID_CHARACTERS=" + ffBool(o.ReplaceInvalidCharacters))
		w("EMPTY_FIELD_AS_NULL=" + ffBool(o.EmptyFieldAsNull))
		w("SKIP_BYTE_ORDER_MARK=" + ffBool(o.SkipByteOrderMark))
		w("ENCODING=" + ffVal(o.Encoding))
		if o.FileExtension != "" && strings.ToUpper(o.FileExtension) != "NONE" {
			w("FILE_EXTENSION=" + ffVal(o.FileExtension))
		}

	case "JSON":
		b.WriteString("TYPE='JSON'")
		w("COMPRESSION=" + ffVal(o.Compression))
		w("DATE_FORMAT=" + ffVal(o.DateFormat))
		w("TIME_FORMAT=" + ffVal(o.TimeFormat))
		w("TIMESTAMP_FORMAT=" + ffVal(o.TimestampFormat))
		w("BINARY_FORMAT=" + ffVal(o.BinaryFormat))
		w("TRIM_SPACE=" + ffBool(o.TrimSpace))
		w("MULTI_LINE=" + ffBool(o.MultiLine))
		w(ffNullIf(o.NullIf))
		if o.FileExtension != "" && strings.ToUpper(o.FileExtension) != "NONE" {
			w("FILE_EXTENSION=" + ffVal(o.FileExtension))
		}
		w("ENABLE_OCTAL=" + ffBool(o.EnableOctal))
		w("ALLOW_DUPLICATE=" + ffBool(o.AllowDuplicate))
		w("STRIP_OUTER_ARRAY=" + ffBool(o.StripOuterArray))
		w("STRIP_NULL_VALUES=" + ffBool(o.StripNullValues))
		w("REPLACE_INVALID_CHARACTERS=" + ffBool(o.ReplaceInvalidCharacters))
		w("IGNORE_UTF8_ERRORS=" + ffBool(o.IgnoreUtf8Errors))
		w("SKIP_BYTE_ORDER_MARK=" + ffBool(o.SkipByteOrderMark))

	case "AVRO":
		b.WriteString("TYPE='AVRO'")
		w("COMPRESSION=" + ffVal(o.Compression))
		w("TRIM_SPACE=" + ffBool(o.TrimSpace))
		w("REPLACE_INVALID_CHARACTERS=" + ffBool(o.ReplaceInvalidCharacters))
		w(ffNullIf(o.NullIf))

	case "ORC":
		b.WriteString("TYPE='ORC'")
		w("TRIM_SPACE=" + ffBool(o.TrimSpace))
		w("REPLACE_INVALID_CHARACTERS=" + ffBool(o.ReplaceInvalidCharacters))
		w(ffNullIf(o.NullIf))

	case "PARQUET":
		b.WriteString("TYPE='PARQUET'")
		w("COMPRESSION=" + ffVal(o.Compression))
		w("SNAPPY_COMPRESSION=" + ffBool(o.SnappyCompression))
		w("BINARY_AS_TEXT=" + ffBool(o.BinaryAsText))
		w("USE_LOGICAL_TYPE=" + ffBool(o.UseLogicalType))
		w("TRIM_SPACE=" + ffBool(o.TrimSpace))
		w("USE_VECTORIZED_SCANNER=" + ffBool(o.UseVectorizedScanner))
		w("REPLACE_INVALID_CHARACTERS=" + ffBool(o.ReplaceInvalidCharacters))
		w(ffNullIf(o.NullIf))
	}

	return b.String()
}

// ── CREATE TABLE ─────────────────────────────────────────────────────────────

// buildCreateTableSQL returns a CREATE TABLE statement that derives its schema
// from the staged file using INFER_SCHEMA (CSV/AVRO/ORC/PARQUET) or creates a
// single VARIANT column for JSON (whose schema cannot be reliably inferred).
//
// fmtRef, when non-empty, is the fully-qualified name of a pre-created named
// file format. INFER_SCHEMA requires a named format for CSV with
// PARSE_HEADER=TRUE because it does not accept PARSE_HEADER inline.
func buildCreateTableSQL(stageAt, tableRef, fmtRef string, p ImportTableParams) string {
	if strings.ToUpper(p.Format) == "JSON" && p.NamedFormat == "" {
		return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (VALUE VARIANT)", tableRef)
	}

	var ffClause string
	if fmtRef != "" {
		// Temporary/inferred format created by ImportTableData.
		ffClause = fmt.Sprintf("FILE_FORMAT=>'%s'", EscapeStringLit(fmtRef))
	} else if p.NamedFormat != "" {
		// User-selected existing named format.
		qualifiedFmt := Qualify(p.Database, p.Schema, p.NamedFormat)
		ffClause = fmt.Sprintf("FILE_FORMAT=>%s", qualifiedFmt)
	} else {
		// Inline options.
		ffClause = fmt.Sprintf("FILE_FORMAT=>(%s)", buildFFOptions(p.Format, p.Options))
	}

	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s\nUSING TEMPLATE (\n    SELECT ARRAY_AGG(OBJECT_CONSTRUCT(*))\n    FROM TABLE(INFER_SCHEMA(\n        LOCATION=>'%s',\n        %s\n    ))\n)",
		tableRef, stageAt, ffClause)
}

// ── COPY INTO ────────────────────────────────────────────────────────────────

// buildImportCopySQL returns the COPY INTO <table> FROM @stage statement.
func buildImportCopySQL(stageAt, tableRef string, p ImportTableParams) string {
	if p.NamedFormat != "" {
		qualifiedFmt := Qualify(p.Database, p.Schema, p.NamedFormat)
		return fmt.Sprintf(
			"COPY INTO %s\nFROM %s\nFILE_FORMAT = (FORMAT_NAME = %s)\nMATCH_BY_COLUMN_NAME = CASE_INSENSITIVE\nFORCE = TRUE",
			tableRef, stageAt, qualifiedFmt)
	}

	ff := buildFFOptions(p.Format, p.Options)

	switch strings.ToUpper(p.Format) {
	case "JSON":
		if p.CreateTable {
			// Table has a single VARIANT column named VALUE; load each JSON object as $1.
			return fmt.Sprintf(
				"COPY INTO %s (VALUE)\nFROM (SELECT $1 FROM %s)\nFILE_FORMAT = (%s)\nFORCE = TRUE",
				tableRef, stageAt, ff)
		}
		return fmt.Sprintf(
			"COPY INTO %s\nFROM %s\nFILE_FORMAT = (%s)\nMATCH_BY_COLUMN_NAME = CASE_INSENSITIVE\nFORCE = TRUE",
			tableRef, stageAt, ff)

	case "AVRO", "ORC", "PARQUET":
		return fmt.Sprintf(
			"COPY INTO %s\nFROM %s\nFILE_FORMAT = (%s)\nMATCH_BY_COLUMN_NAME = CASE_INSENSITIVE\nFORCE = TRUE",
			tableRef, stageAt, ff)

	default: // CSV
		if p.CreateTable && p.Options.ParseHeader {
			// Table was created with PARSE_HEADER; match columns by name.
			return fmt.Sprintf(
				"COPY INTO %s\nFROM %s\nFILE_FORMAT = (%s)\nMATCH_BY_COLUMN_NAME = CASE_INSENSITIVE\nFORCE = TRUE",
				tableRef, stageAt, ff)
		}
		return fmt.Sprintf(
			"COPY INTO %s\nFROM %s\nFILE_FORMAT = (%s)\nFORCE = TRUE",
			tableRef, stageAt, ff)
	}
}

// FetchNotebookContent retrieves the source of a Snowflake Notebook object and
// returns it as a string (nbformat v4 JSON).
//
// Steps:
//  1. DESC NOTEBOOK "<db>"."<schema>"."<name>" → read last_version_location_uri
//  2. GET <uri> to a temp directory
//  3. Read the downloaded .ipynb file
//  4. Clean up the temp directory
func (c *Client) FetchNotebookContent(ctx context.Context, database, schema, name string) (string, error) {
	notebookRef := Qualify(database, schema, name)

	// DESC NOTEBOOK to find the stage URI of the latest version.
	descRows, err := c.queryCtx(ctx, "DESC NOTEBOOK "+notebookRef)
	if err != nil {
		return "", fmt.Errorf("describe notebook: %w", err)
	}
	defer descRows.Close() //nolint:errcheck

	cols, err := descRows.Columns()
	if err != nil {
		return "", fmt.Errorf("describe notebook columns: %w", err)
	}

	// Find the last_version_location_uri column index.
	uriIdx := -1
	for i, col := range cols {
		if strings.EqualFold(col, "last_version_location_uri") {
			uriIdx = i
			break
		}
	}
	if uriIdx < 0 {
		return "", fmt.Errorf("DESC NOTEBOOK did not return a last_version_location_uri column (columns: %v)", cols)
	}

	var stageURI string
	for descRows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := descRows.Scan(ptrs...); err != nil {
			continue
		}
		if v := fmt.Sprintf("%v", vals[uriIdx]); v != "" && v != "<nil>" {
			stageURI = v
			break
		}
	}
	descRows.Close() //nolint:errcheck
	if stageURI == "" {
		return "", fmt.Errorf("last_version_location_uri is empty for notebook %s", notebookRef)
	}

	// Download the file to a temp directory.
	tmpDir, err := os.MkdirTemp("", "thaw_nb_")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	dirURL, err := localFileURL(tmpDir)
	if err != nil {
		return "", fmt.Errorf("build temp dir url: %w", err)
	}

	getSQL := fmt.Sprintf("GET '%s' '%s'", strings.ReplaceAll(stageURI, "'", "\\'"), dirURL)
	getRows, err := c.queryCtx(ctx, getSQL)
	if err != nil {
		return "", fmt.Errorf("GET notebook file: %w", err)
	}
	getRows.Close() //nolint:errcheck

	// Find the downloaded .ipynb file in the temp directory.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", fmt.Errorf("read temp dir: %w", err)
	}

	var ipynbPath string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".ipynb") {
			ipynbPath = filepath.Join(tmpDir, e.Name())
			break
		}
	}
	// Fallback: take the first file if no .ipynb was found (some runtimes omit
	// the extension in the stage path).
	if ipynbPath == "" && len(entries) > 0 {
		ipynbPath = filepath.Join(tmpDir, entries[0].Name())
	}
	if ipynbPath == "" {
		return "", fmt.Errorf("GET succeeded but no file was written to temp dir")
	}

	data, err := os.ReadFile(ipynbPath)
	if err != nil {
		return "", fmt.Errorf("read notebook file: %w", err)
	}
	return string(data), nil
}

// ExecuteNotebook runs EXECUTE NOTEBOOK against a Snowflake Notebook object.
// params contains the string literal values to pass; each value is
// single-quote escaped and wrapped in quotes before being embedded in the SQL.
// Pass an empty slice to execute with no parameters.
func (c *Client) ExecuteNotebook(ctx context.Context, database, schema, name string, params []string) (string, error) {
	notebookRef := Qualify(database, schema, name)

	args := make([]string, len(params))
	for i, p := range params {
		args[i] = QuoteStringLit(p)
	}
	sql := fmt.Sprintf("EXECUTE NOTEBOOK %s(%s)", notebookRef, strings.Join(args, ", "))

	result, err := c.Execute(ctx, sql)
	if err != nil {
		return "", err
	}
	return result.QueryID, nil
}

// ExecuteTask manually triggers a single run of a Snowflake Task.
//
// If retryLast is true the statement becomes:
//
//	EXECUTE TASK <ref> RETRY LAST
//
// which re-executes the last failed/canceled task graph run from where it
// failed. config and retryLast are mutually exclusive — when retryLast is
// true, config is ignored.
//
// If config is a non-empty JSON string and retryLast is false the statement
// becomes:
//
//	EXECUTE TASK <ref> USING CONFIG = $$<config>$$
//
// Otherwise a plain EXECUTE TASK <ref> is issued.
func (c *Client) ExecuteTask(ctx context.Context, database, schema, name, config string, retryLast bool) error {
	ref := Qualify(database, schema, name)

	var sql string
	switch {
	case retryLast:
		sql = fmt.Sprintf("EXECUTE TASK %s RETRY LAST", ref)
	case config != "":
		sql = fmt.Sprintf("EXECUTE TASK %s USING CONFIG = $$%s$$", ref, config)
	default:
		sql = fmt.Sprintf("EXECUTE TASK %s", ref)
	}
	_, err := c.Execute(ctx, sql)
	return err
}

// GetNotebookQueryWarehouse returns the QUERY_WAREHOUSE currently set on a
// Snowflake Notebook object, or an empty string if none is configured.
func (c *Client) GetNotebookQueryWarehouse(ctx context.Context, database, schema, name string) (string, error) {
	like := EscapeStringLit(name)
	sql := fmt.Sprintf("SHOW NOTEBOOKS LIKE '%s' IN SCHEMA %s", like, Qualify(database, schema))
	res, err := c.Execute(ctx, sql)
	if err != nil {
		return "", err
	}
	if len(res.Rows) == 0 {
		return "", nil
	}
	row := res.Rows[0]
	for i, col := range res.Columns {
		if strings.EqualFold(col, "query_warehouse") && i < len(row) {
			if row[i] == nil {
				return "", nil
			}
			switch v := row[i].(type) {
			case string:
				return v, nil
			case []byte:
				return string(v), nil
			default:
				return fmt.Sprintf("%v", v), nil
			}
		}
	}
	return "", nil
}

// SetNotebookQueryWarehouse issues ALTER NOTEBOOK … SET QUERY_WAREHOUSE on the
// given notebook object.
func (c *Client) SetNotebookQueryWarehouse(ctx context.Context, database, schema, name, warehouse string) error {
	notebookRef := Qualify(database, schema, name)
	sql := fmt.Sprintf(`ALTER NOTEBOOK %s SET QUERY_WAREHOUSE = %s`, notebookRef, QuoteIdent(warehouse))
	_, err := c.Execute(ctx, sql)
	return err
}

// DeployNotebookParams holds the parameters for deploying a local .ipynb
// notebook to Snowflake via a temporary internal stage.
type DeployNotebookParams struct {
	Database      string `json:"database"`
	Schema        string `json:"schema"`
	Name          string `json:"name"`          // notebook object name in Snowflake
	CaseSensitive bool   `json:"caseSensitive"` // when true, Name is double-quoted exactly; otherwise unquoted if valid
	FilePath      string `json:"filePath"`      // absolute local path to the .ipynb file; mutually exclusive with Content
	Content       string `json:"content"`       // raw nbformat JSON; used when FilePath is empty (unsaved notebooks)
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	// Optional CREATE NOTEBOOK clauses
	Comment                 string `json:"comment"`
	QueryWarehouse          string `json:"queryWarehouse"`          // warehouse for SQL queries inside the notebook
	IdleAutoShutdownSeconds int    `json:"idleAutoShutdownSeconds"` // 0 → omit
	RuntimeName             string `json:"runtimeName"`
	ComputePool             string `json:"computePool"`
	Warehouse               string `json:"warehouse"` // warehouse for Python runtime
}

// DeployNotebook uploads a local .ipynb file to a temporary internal stage and
// creates a Snowflake NOTEBOOK object from it, then drops the stage.
//
//  1. CREATE TEMPORARY STAGE in the target schema
//  2. PUT the .ipynb file to the stage
//  3. CREATE [OR REPLACE] NOTEBOOK … FROM '<stage>' MAIN_FILE = '<file>'
//  4. DROP STAGE (deferred – also fires on error)
func (c *Client) DeployNotebook(ctx context.Context, params DeployNotebookParams) error {

	// If no file path was given, write the in-memory content to a temp file so
	// the rest of the function can treat both cases identically.
	if params.FilePath == "" {
		if params.Content == "" {
			return fmt.Errorf("either FilePath or Content must be provided")
		}
		tmp, err := os.CreateTemp("", "thaw_nb_*.ipynb")
		if err != nil {
			return fmt.Errorf("create temp notebook file: %w", err)
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath) //nolint:errcheck
		if _, err := tmp.WriteString(params.Content); err != nil {
			tmp.Close() //nolint:errcheck
			return fmt.Errorf("write temp notebook file: %w", err)
		}
		tmp.Close() //nolint:errcheck
		params.FilePath = tmpPath
	}

	// Create a temporary stage in the target schema.
	stageName := fmt.Sprintf("THAW_NB_%d", time.Now().UnixNano())
	stageRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(params.Database), QuoteIdent(params.Schema), stageName)
	stageAt := "@" + stageRef

	if _, err := c.execCtx(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return fmt.Errorf("create notebook stage: %w", err)
	}
	defer c.execCtx(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// PUT the .ipynb file to the stage.
	fileURL, err := localFileURLForFile(params.FilePath)
	if err != nil {
		return fmt.Errorf("build file url: %w", err)
	}
	escapedURL := strings.ReplaceAll(fileURL, "'", "\\'")
	putSQL := fmt.Sprintf("PUT '%s' %s AUTO_COMPRESS=FALSE OVERWRITE=TRUE", escapedURL, stageAt)
	putRows, err := c.queryCtx(ctx, putSQL)
	if err != nil {
		return fmt.Errorf("upload notebook to stage: %w", err)
	}
	putRows.Close() //nolint:errcheck

	// Build the CREATE NOTEBOOK statement.
	notebookRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(params.Database), QuoteIdent(params.Schema), QuoteOrBare(params.Name, params.CaseSensitive))
	mainFile := filepath.Base(params.FilePath)

	var sb strings.Builder
	sb.WriteString("CREATE ")
	if params.OrReplace {
		sb.WriteString("OR REPLACE ")
	}
	sb.WriteString("NOTEBOOK ")
	if params.IfNotExists {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(notebookRef)
	sb.WriteString(fmt.Sprintf("\n  FROM '%s'", stageAt))
	sb.WriteString(fmt.Sprintf("\n  MAIN_FILE = '%s'", EscapeStringLit(mainFile)))
	if params.Comment != "" {
		sb.WriteString(fmt.Sprintf("\n  COMMENT = '%s'", EscapeStringLit(params.Comment)))
	}
	if params.QueryWarehouse != "" {
		sb.WriteString(fmt.Sprintf("\n  QUERY_WAREHOUSE = %s", params.QueryWarehouse))
	}
	if params.IdleAutoShutdownSeconds > 0 {
		sb.WriteString(fmt.Sprintf("\n  IDLE_AUTO_SHUTDOWN_TIME_SECONDS = %d", params.IdleAutoShutdownSeconds))
	}
	if params.RuntimeName != "" {
		sb.WriteString(fmt.Sprintf("\n  RUNTIME_NAME = '%s'", EscapeStringLit(params.RuntimeName)))
	}
	if params.ComputePool != "" {
		sb.WriteString(fmt.Sprintf("\n  COMPUTE_POOL = '%s'", EscapeStringLit(params.ComputePool)))
	}
	if params.Warehouse != "" {
		sb.WriteString(fmt.Sprintf("\n  WAREHOUSE = %s", params.Warehouse))
	}

	if _, err := c.execCtx(ctx, sb.String()); err != nil {
		return fmt.Errorf("create notebook: %w", err)
	}
	return nil
}

// ── Cross-schema object search ───────────────────────────────────────────────

// SearchResult holds the results of a cross-schema object and column name
// search against INFORMATION_SCHEMA.
type SearchResult struct {
	Objects []SearchObjectMatch `json:"objects"`
	Columns []SearchColumnMatch `json:"columns"`
}

// SearchObjectMatch represents a table/view whose name matched the search
// pattern.
type SearchObjectMatch struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

// SearchColumnMatch represents a column whose name matched the search pattern.
type SearchColumnMatch struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
	Column   string `json:"column"`
	DataType string `json:"dataType"`
}

// SearchObjects searches for objects and columns matching a SQL ILIKE pattern
// across all schemas in the given database. If database is empty, the session's
// current database is used. Both INFORMATION_SCHEMA.TABLES and
// INFORMATION_SCHEMA.COLUMNS are queried concurrently, each limited to 100 rows.
func (c *Client) SearchObjects(ctx context.Context, pattern, database string) (SearchResult, error) {
	if database == "" {
		sc, err := c.GetSessionContext(ctx)
		if err != nil {
			return SearchResult{}, fmt.Errorf("get session context: %w", err)
		}
		database = sc.Database
		if database == "" {
			return SearchResult{}, fmt.Errorf("no database specified and no current database in session")
		}
	}

	db := QuoteIdent(database)

	type objResult struct {
		objects []SearchObjectMatch
		err     error
	}
	type colResult struct {
		columns []SearchColumnMatch
		err     error
	}

	objCh := make(chan objResult, 1)
	colCh := make(chan colResult, 1)

	go func() {
		query := fmt.Sprintf(
			`SELECT TABLE_CATALOG, TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM %s.INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME ILIKE ? AND TABLE_SCHEMA != 'INFORMATION_SCHEMA' LIMIT 100`,
			db,
		)
		rows, err := c.queryCtx(ctx, query, pattern)
		if err != nil {
			objCh <- objResult{err: err}
			return
		}
		defer rows.Close() //nolint:errcheck

		var objects []SearchObjectMatch
		for rows.Next() {
			var m SearchObjectMatch
			if err := rows.Scan(&m.Database, &m.Schema, &m.Name, &m.Kind); err != nil {
				objCh <- objResult{err: err}
				return
			}
			objects = append(objects, m)
		}
		if objects == nil {
			objects = []SearchObjectMatch{}
		}
		objCh <- objResult{objects: objects, err: rows.Err()}
	}()

	go func() {
		query := fmt.Sprintf(
			`SELECT TABLE_CATALOG, TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE FROM %s.INFORMATION_SCHEMA.COLUMNS WHERE COLUMN_NAME ILIKE ? AND TABLE_SCHEMA != 'INFORMATION_SCHEMA' LIMIT 100`,
			db,
		)
		rows, err := c.queryCtx(ctx, query, pattern)
		if err != nil {
			colCh <- colResult{err: err}
			return
		}
		defer rows.Close() //nolint:errcheck

		var columns []SearchColumnMatch
		for rows.Next() {
			var m SearchColumnMatch
			if err := rows.Scan(&m.Database, &m.Schema, &m.Table, &m.Column, &m.DataType); err != nil {
				colCh <- colResult{err: err}
				return
			}
			columns = append(columns, m)
		}
		if columns == nil {
			columns = []SearchColumnMatch{}
		}
		colCh <- colResult{columns: columns, err: rows.Err()}
	}()

	objRes := <-objCh
	colRes := <-colCh

	if objRes.err != nil {
		return SearchResult{}, fmt.Errorf("search objects: %w", objRes.err)
	}
	if colRes.err != nil {
		return SearchResult{}, fmt.Errorf("search columns: %w", colRes.err)
	}

	return SearchResult{
		Objects: objRes.objects,
		Columns: colRes.columns,
	}, nil
}
