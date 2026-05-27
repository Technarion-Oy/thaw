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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	sf "github.com/snowflakedb/gosnowflake/v2"
)

// ConnectParams holds all fields needed to open a Snowflake connection.
// Authenticator values:
//
//	"snowflake"            – password (+ optional TOTP passcode)
//	"username_password_mfa" – password + MFA push notification
//	"externalbrowser"      – browser-based SSO (no password needed)
//	"okta"                 – Okta native SSO (requires OktaURL)
//	"snowflake_jwt"        – key-pair / JWT (requires PrivateKeyPath)
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
			connExec(conn, fmt.Sprintf(`USE SCHEMA %s.%s`, QuoteIdent(db), QuoteIdent(schema)))
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
		"username_password_mfa": sf.AuthTypeUsernamePasswordMFA,
		"externalbrowser":       sf.AuthTypeExternalBrowser,
		"okta":                  sf.AuthTypeOkta,
		"snowflake_jwt":         sf.AuthTypeJwt,
	}
	auth, ok := authMap[strings.ToLower(p.Authenticator)]
	if !ok {
		auth = sf.AuthTypeSnowflake
	}

	// Interactive flows need more time; plain password should fail quickly.
	// LoginTimeout is the gosnowflake-internal control — context cancellation
	// alone is not reliable for aborting auth inside the driver.
	authenticatorLower := strings.ToLower(p.Authenticator)
	loginTimeout := 15 * time.Second
	if authenticatorLower == "username_password_mfa" ||
		authenticatorLower == "externalbrowser" ||
		authenticatorLower == "okta" {
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
	if err := c.db.QueryRowContext(ctx, "SELECT CURRENT_SESSION()").Scan(&id); err != nil {
		return "", err
	}
	return id, nil
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
	stmts := splitStatements(query)
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

		result, err := queryOnConn(stmtCtx, conn, stmt)
		if err != nil {
			return nil, err
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

	result, err := queryOnConn(ctx, conn, query)
	if err != nil {
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

	return result, nil
}

// SplitStatements splits a SQL string into individual statements on
// semicolons, respecting single-quoted strings, double-quoted identifiers,
// line comments (--), block comments (/* */), and Snowflake dollar-quoted
// strings ($$..$$ and $tag$..$tag$).
func SplitStatements(sql string) []string { return splitStatements(sql) }

// splitStatements is the internal implementation of SplitStatements.
func splitStatements(sql string) []string {
	var stmts []string
	var cur strings.Builder
	i, n := 0, len(sql)
	for i < n {
		ch := sql[i]
		switch {
		case ch == '-' && i+1 < n && sql[i+1] == '-':
			// Line comment — consume through end of line.
			for i < n && sql[i] != '\n' {
				cur.WriteByte(sql[i])
				i++
			}
		case ch == '/' && i+1 < n && sql[i+1] == '*':
			// Block comment — consume through */.
			cur.WriteByte(sql[i])
			cur.WriteByte(sql[i+1])
			i += 2
			for i < n {
				if sql[i] == '*' && i+1 < n && sql[i+1] == '/' {
					cur.WriteByte(sql[i])
					cur.WriteByte(sql[i+1])
					i += 2
					break
				}
				cur.WriteByte(sql[i])
				i++
			}
		case ch == '\'':
			// Single-quoted string — handle '' escaping.
			cur.WriteByte(ch)
			i++
			for i < n {
				c := sql[i]
				cur.WriteByte(c)
				i++
				if c == '\'' {
					if i < n && sql[i] == '\'' {
						cur.WriteByte(sql[i])
						i++
					} else {
						break
					}
				}
			}
		case ch == '"':
			// Double-quoted identifier.
			cur.WriteByte(ch)
			i++
			for i < n {
				c := sql[i]
				cur.WriteByte(c)
				i++
				if c == '"' {
					break
				}
			}
		case ch == '$':
			// Possible dollar-quoted string: $$...$$ or $tag$...$tag$.
			end := i + 1
			for end < n && (sql[end] == '_' ||
				(sql[end] >= 'a' && sql[end] <= 'z') ||
				(sql[end] >= 'A' && sql[end] <= 'Z') ||
				(sql[end] >= '0' && sql[end] <= '9')) {
				end++
			}
			if end < n && sql[end] == '$' {
				tag := sql[i : end+1] // e.g. "$$" or "$my_tag$"
				cur.WriteString(tag)
				i = end + 1
				for i < n {
					if strings.HasPrefix(sql[i:], tag) {
						cur.WriteString(tag)
						i += len(tag)
						break
					}
					cur.WriteByte(sql[i])
					i++
				}
			} else {
				cur.WriteByte(ch)
				i++
			}
		case ch == ';':
			stmt := normalizePutGet(strings.TrimSpace(cur.String()))
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
			cur.Reset()
			i++
		default:
			cur.WriteByte(ch)
			i++
		}
	}
	if stmt := normalizePutGet(strings.TrimSpace(cur.String())); stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
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
	escaped := strings.ReplaceAll(queryID, "'", "''")
	_, err := c.db.ExecContext(ctx, fmt.Sprintf("SELECT SYSTEM$CANCEL_QUERY('%s')", escaped))
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
	stmts := splitStatements(query)
	if len(stmts) == 0 {
		return &QueryResult{Rows: [][]interface{}{}}, nil
	}

	var last *QueryResult
	for _, stmt := range stmts {
		result, err := queryOnConn(ctx, conn, stmt)
		if err != nil {
			return nil, err
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
	if err := c.db.QueryRowContext(ctx, "SELECT CURRENT_AVAILABLE_ROLES()").Scan(&raw); err != nil {
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

	nameIdx := -1
	typeIdx := -1
	catIdx := -1
	enabledIdx := -1
	commentIdx := -1

	for i, col := range res.Columns {
		switch strings.ToUpper(col) {
		case "NAME":
			nameIdx = i
		case "TYPE":
			typeIdx = i
		case "CATEGORY":
			catIdx = i
		case "ENABLED":
			enabledIdx = i
		case "COMMENT":
			commentIdx = i
		}
	}

	var ints []SecurityIntegration
	for _, row := range res.Rows {
		s := SecurityIntegration{}
		if nameIdx != -1 {
			s.Name = strVal(row, nameIdx)
		}
		if typeIdx != -1 {
			s.Type = strVal(row, typeIdx)
		}
		if catIdx != -1 {
			s.Category = strVal(row, catIdx)
		}
		if enabledIdx != -1 {
			s.Enabled = strVal(row, enabledIdx) == "true"
		}
		if commentIdx != -1 {
			s.Comment = strVal(row, commentIdx)
		}
		ints = append(ints, s)
	}
	return ints, nil
}

// ListWarehouses returns the names of all warehouses visible to the current role.
func (c *Client) ListWarehouses(ctx context.Context) ([]string, error) {
	// SHOW WAREHOUSES columns: name, state, ...
	return c.queryStringSlice(ctx, "SHOW WAREHOUSES", 0)
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

	nameIdx := -1
	typeIdx := -1
	enabledIdx := -1
	commentIdx := -1

	for i, col := range res.Columns {
		switch strings.ToUpper(col) {
		case "NAME":
			nameIdx = i
		case "TYPE":
			typeIdx = i
		case "ENABLED":
			enabledIdx = i
		case "COMMENT":
			commentIdx = i
		}
	}

	var ints []ApiIntegration
	for _, row := range res.Rows {
		a := ApiIntegration{}
		if nameIdx != -1 {
			a.Name = strVal(row, nameIdx)
		}
		if typeIdx != -1 {
			a.Type = strVal(row, typeIdx)
		}
		if enabledIdx != -1 {
			a.Enabled = strVal(row, enabledIdx) == "true"
		}
		if commentIdx != -1 {
			a.Comment = strVal(row, commentIdx)
		}
		ints = append(ints, a)
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

	nameIdx := -1
	dbIdx := -1
	schemaIdx := -1

	for i, col := range res.Columns {
		switch strings.ToUpper(col) {
		case "NAME":
			nameIdx = i
		case "DATABASE_NAME":
			dbIdx = i
		case "SCHEMA_NAME":
			schemaIdx = i
		}
	}

	var secrets []AccountSecret
	for _, row := range res.Rows {
		s := AccountSecret{}
		if nameIdx != -1 {
			s.Name = strVal(row, nameIdx)
		}
		if dbIdx != -1 {
			s.DatabaseName = strVal(row, dbIdx)
		}
		if schemaIdx != -1 {
			s.SchemaName = strVal(row, schemaIdx)
		}
		secrets = append(secrets, s)
	}
	return secrets, nil
}

// GitRepoEntry represents a file or directory inside a Snowflake git repository stage.
type GitRepoEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size,omitempty"`
}

// GitBranch represents a branch in a Snowflake git repository.
type GitBranch struct {
	Name string `json:"name"`
}

// GitTag represents a tag in a Snowflake git repository.
type GitTag struct {
	Name string `json:"name"`
}

// ListGitRepoEntries returns the immediate children (files and directories) at
// dirPath within the git repository stage @database.schema.repoName/dirPath.
// Pass an empty dirPath to list the root. Directories are sorted first, then
// files; both groups are sorted case-insensitively by name.
func (c *Client) ListGitRepoEntries(ctx context.Context, database, schema, repoName, dirPath string) ([]GitRepoEntry, error) {
	// NORMALIZE DIRPATH
	// Remove leading slash so HasPrefix matches relPath safely.
	dirPath = strings.TrimPrefix(dirPath, "/")
	// Ensure trailing slash to prevent swallowing files into empty-named directories.
	if dirPath != "" && !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}

	sql := fmt.Sprintf(`LIST @%s.%s.%s/%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(repoName), dirPath)

	res, err := c.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

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
		return []GitRepoEntry{}, nil
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
			ur := strings.ToUpper(repoName)

			// Determine if the part before the first slash is a stage prefix.
			// It might be REPO, "REPO", @REPO, or a qualified DB.SCHEMA.REPO.
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

	return entries, nil
}

// ListGitBranches returns all branches in the given git repository.
func (c *Client) ListGitBranches(ctx context.Context, database, schema, repoName string) ([]GitBranch, error) {
	sql := fmt.Sprintf(`SHOW GIT BRANCHES IN GIT REPOSITORY %s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(repoName))

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
	sql := fmt.Sprintf(`SHOW GIT TAGS IN GIT REPOSITORY %s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(repoName))

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
	sql := fmt.Sprintf(`SELECT $1 FROM @%s.%s.%s/%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(repoName), filePath)

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

// ExecuteGitFile executes a SQL file from a git repository.
func (c *Client) ExecuteGitFile(ctx context.Context, database, schema, repoName, filePath string) error {
	sql := fmt.Sprintf(`EXECUTE IMMEDIATE FROM @%s.%s.%s/%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(repoName), filePath)

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
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf("SHOW %s INTEGRATIONS", upper))
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
	esc := strings.ReplaceAll(name, `"`, `""`)
	_, err := c.db.ExecContext(ctx, fmt.Sprintf(`DROP INTEGRATION %s`, esc))
	return err
}

// DropDatabase drops a database. mode must be "CASCADE" or "RESTRICT".
func (c *Client) DropDatabase(ctx context.Context, name string, mode string) error {
	if mode != "CASCADE" && mode != "RESTRICT" {
		mode = "CASCADE"
	}
	esc := strings.ReplaceAll(name, `"`, `""`)
	_, err := c.db.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE %s %s`, esc, mode))
	return err
}

// DropSchema drops a schema. mode must be "CASCADE" or "RESTRICT".
func (c *Client) DropSchema(ctx context.Context, database, schema string, mode string) error {
	if mode != "CASCADE" && mode != "RESTRICT" {
		mode = "CASCADE"
	}
	escDb := strings.ReplaceAll(database, `"`, `""`)
	escSch := strings.ReplaceAll(schema, `"`, `""`)
	_, err := c.db.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA %s.%s %s`, escDb, escSch, mode))
	return err
}

// ExecDDL executes a pre-built DDL statement (e.g. CREATE INTEGRATION …).
// The caller is responsible for ensuring the SQL is safe; use the integrations
// package helpers to build injection-safe DDL before calling this method.
func (c *Client) ExecDDL(ctx context.Context, sql string) error {
	_, err := c.db.ExecContext(ctx, sql)
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
		if err := c.db.QueryRowContext(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
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

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`DESCRIBE USER %s`, QuoteIdent(name)))
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
	esc := strings.ReplaceAll(role, `"`, `""`)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`SHOW GRANTS TO ROLE %s`, esc))
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
		esc := strings.ReplaceAll(role, `"`, `""`)
		rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`SHOW GRANTS TO ROLE %s`, esc))
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
		if err := c.db.QueryRowContext(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
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
	esc := strings.ReplaceAll(username, `"`, `""`)
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`SHOW GRANTS ON USER %s`, esc))
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
		if err := c.db.QueryRowContext(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
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
		if err := c.db.QueryRowContext(ctx, "SELECT CURRENT_ROLE()").Scan(&role); err != nil {
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
	escapedLike := strings.ReplaceAll(name, "'", "''")
	escapedIdent := strings.ReplaceAll(name, `"`, `""`)

	// ── Comment from SHOW ROLES LIKE ────────────────────────────────────────
	var comment string
	if rows, err := c.db.QueryContext(ctx,
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

	sb.WriteString(fmt.Sprintf("CREATE ROLE IF NOT EXISTS \"%s\"", escapedIdent))
	if comment != "" {
		sb.WriteString(fmt.Sprintf("\n  COMMENT = '%s'",
			strings.ReplaceAll(comment, "'", "''")))
	}
	sb.WriteString(";\n")

	// ── SHOW GRANTS TO ROLE → privileges granted to this role ────────────────
	if rows, err := c.db.QueryContext(ctx,
		fmt.Sprintf(`SHOW GRANTS TO ROLE %s`, escapedIdent)); err == nil {
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
			sb.WriteString(FormatRoleGrant(priv, onType, obj, escapedIdent, opt) + "\n")
		}
		rows.Close() //nolint:errcheck
	}

	// ── SHOW GRANTS ON ROLE → who this role is granted to ────────────────────
	if rows, err := c.db.QueryContext(ctx,
		fmt.Sprintf(`SHOW GRANTS ON ROLE %s`, escapedIdent)); err == nil {
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
			sb.WriteString(fmt.Sprintf("GRANT ROLE \"%s\" TO %s \"%s\";\n",
				escapedIdent, grantedTo,
				strings.ReplaceAll(grantee, `"`, `""`)))
		}
		rows.Close() //nolint:errcheck
	}

	return sb.String(), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// FormatRoleGrant builds a single GRANT statement line for a role DDL export.
// Special cases handled:
//   - ON ACCOUNT: object name is omitted (Snowflake requires bare ON ACCOUNT)
//   - USAGE ON ROLE: converted to GRANT ROLE ... TO ROLE ... (the executable
//     form of role membership); WITH GRANT OPTION is dropped because
//     GRANT ROLE ... WITH GRANT OPTION is not valid Snowflake syntax
func FormatRoleGrant(priv, onType, obj, escapedRole string, withGrantOption bool) string {
	var stmt string
	switch {
	case strings.EqualFold(onType, "ROLE"):
		// Quote the child role name — SHOW GRANTS returns bare identifiers even
		// for mixed-case roles (e.g. "My_Role" → My_Role in the name column).
		escapedChild := strings.ReplaceAll(obj, `"`, `""`)
		if strings.EqualFold(priv, "USAGE") {
			// USAGE on ROLE is Snowflake's internal representation of role membership.
			// The executable form is GRANT ROLE <name> TO ROLE <parent>.
			// WITH GRANT OPTION is not valid for GRANT ROLE statements.
			return fmt.Sprintf("GRANT ROLE \"%s\" TO ROLE \"%s\";", escapedChild, escapedRole)
		}
		stmt = fmt.Sprintf("GRANT %s ON ROLE \"%s\" TO ROLE \"%s\"", priv, escapedChild, escapedRole)
	case strings.EqualFold(onType, "ACCOUNT"):
		stmt = fmt.Sprintf("GRANT %s ON ACCOUNT TO ROLE \"%s\"", priv, escapedRole)
	default:
		stmt = fmt.Sprintf("GRANT %s ON %s %s TO ROLE \"%s\"", priv, onType, obj, escapedRole)
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
	escaped := strings.ReplaceAll(name, "'", "''")
	row := c.db.QueryRowContext(ctx, fmt.Sprintf("SELECT GET_DDL('WAREHOUSE', '%s')", escaped))
	var src string
	if err := row.Scan(&src); err != nil {
		return "", fmt.Errorf("GET_DDL(WAREHOUSE %s): %w", name, err)
	}
	return src, nil
}

// UseRole switches the active role for the current session.
func (c *Client) UseRole(ctx context.Context, role string) error {
	escaped := strings.ReplaceAll(role, `"`, `""`)
	if _, err := c.db.ExecContext(ctx, fmt.Sprintf(`USE ROLE %s`, escaped)); err != nil {
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
	escaped := strings.ReplaceAll(warehouse, `"`, `""`)
	if _, err := c.db.ExecContext(ctx, fmt.Sprintf(`USE WAREHOUSE %s`, escaped)); err != nil {
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
	escaped := strings.ReplaceAll(database, `"`, `""`)
	if _, err := c.db.ExecContext(ctx, fmt.Sprintf(`USE DATABASE %s`, escaped)); err != nil {
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

	escapedSc := strings.ReplaceAll(schema, `"`, `""`)
	var query string
	if db != "" {
		escapedDb := strings.ReplaceAll(db, `"`, `""`)
		query = fmt.Sprintf(`USE SCHEMA %s.%s`, escapedDb, escapedSc)
	} else {
		query = fmt.Sprintf(`USE SCHEMA %s`, escapedSc)
	}

	if _, err := c.db.ExecContext(ctx, query); err != nil {
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
	rows, err := c.db.QueryContext(ctx, "SHOW DATABASES")
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
	escaped := strings.ReplaceAll(database, `"`, `""`)
	return c.queryStringSlice(ctx, fmt.Sprintf(`SHOW SCHEMAS IN DATABASE %s`, escaped), 1)
}

// extractArgTypes parses the "arguments" column returned by SHOW PROCEDURES /
// SHOW FUNCTIONS. The format is "<name>(<types>) RETURN <return_type>", e.g.
// "GET_EMPLOYEE_STATUS(NUMBER) RETURN VARIANT". Returns just the types string,
// e.g. "NUMBER", or an empty string when there are no parameters.
func extractArgTypes(arguments string) string {
	start := strings.Index(arguments, "(")
	if start < 0 {
		return ""
	}
	end := strings.Index(arguments[start:], ")")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(arguments[start+1 : start+end])
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
		fmt.Sprintf(`SHOW TABLES HISTORY IN SCHEMA %s.%s`, QuoteIdent(database), QuoteIdent(schema)))
}

// listDroppedHistory is the shared helper for SHOW * HISTORY queries.
// It reads any result set that has "name" and "dropped_on" columns and returns
// only the rows where dropped_on is non-empty (i.e. the object is dropped).
func (c *Client) listDroppedHistory(ctx context.Context, query string) ([]DroppedTable, error) {
	rows, err := c.db.QueryContext(ctx, query)
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
	esc := strings.ReplaceAll(database, `"`, `""`)
	return c.listDroppedHistory(ctx, fmt.Sprintf(`SHOW SCHEMAS HISTORY IN DATABASE %s`, esc))
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
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return 1, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "key", "value")

	if rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return 1, err
		}
		if s := strVal(vals, idxs["value"]); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				return n, nil
			}
		}
	}
	return 1, nil // default: 1 day
}

// GetSchemaRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given schema. Returns 1 if the value cannot be determined.
func (c *Client) GetSchemaRetentionDays(ctx context.Context, database, schema string) (int, error) {
	query := fmt.Sprintf(`SHOW PARAMETERS LIKE 'DATA_RETENTION_TIME_IN_DAYS' IN SCHEMA %s.%s`, QuoteIdent(database), QuoteIdent(schema))
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return 1, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "key", "value")

	if rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return 1, err
		}
		if s := strVal(vals, idxs["value"]); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				return n, nil
			}
		}
	}
	return 1, nil // default: 1 day
}

// GetTableRetentionDays returns the Time Travel data-retention period in days
// for a single table. It runs SHOW TABLES LIKE and reads the retention_time
// column. Returns 1 (Snowflake's Standard-edition default) when the value
// cannot be determined.
func (c *Client) GetTableRetentionDays(ctx context.Context, database, schema, name string) (int, error) {
	query := fmt.Sprintf(`SHOW TABLES LIKE '%s' IN SCHEMA %s.%s`,
		strings.ReplaceAll(name, "'", "''"), QuoteIdent(database), QuoteIdent(schema))

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "retention_time")

	if rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return 0, err
		}
		if s := strVal(vals, idxs["retention_time"]); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				return n, nil
			}
		}
	}
	return 1, nil // default: 1 day
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
	rows, err := c.db.QueryContext(ctx, "SHOW USERS")
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

// GetTableColumns returns the ordered list of column names for a table or view
// by running DESCRIBE TABLE (which works for both base tables and views in Snowflake).
func (c *Client) GetTableColumns(ctx context.Context, database, schema, name string) ([]string, error) {
	query := fmt.Sprintf(`DESCRIBE TABLE %s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(name))
	rows, err := c.db.QueryContext(ctx, query)
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
	query := fmt.Sprintf(`SHOW IMPORTED KEYS IN TABLE %s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(table))
	rows, err := c.db.QueryContext(ctx, query)
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
}

// GetTableColumnsWithTypes returns the ordered column list for a table or view
// together with their data types by running DESCRIBE TABLE.
func (c *Client) GetTableColumnsWithTypes(ctx context.Context, database, schema, name string) ([]ColumnInfo, error) {
	query := fmt.Sprintf(`DESCRIBE TABLE %s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(name))
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "name", "type", "null?", "primary key", "unique key")

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
		})
	}
	return result, rows.Err()
}

// GetSchemaForeignKeys returns all FK→PK column mappings in a schema by running
// SHOW IMPORTED KEYS IN SCHEMA. This bulk call is cheaper than per-table SHOW
// IMPORTED KEYS when the editor needs to warm up FK data for many tables at once.
// The result set columns are identical to SHOW IMPORTED KEYS IN TABLE.
func (c *Client) GetSchemaForeignKeys(ctx context.Context, database, schema string) ([]TableForeignKey, error) {
	query := fmt.Sprintf(`SHOW IMPORTED KEYS IN SCHEMA %s.%s`, QuoteIdent(database), QuoteIdent(schema))
	rows, err := c.db.QueryContext(ctx, query)
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
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	nameIdx, kindIdx, argsIdx, builtinIdx, rowsIdx, predsIdx, taskRelIdx, finalizeColIdx := -1, -1, -1, -1, -1, -1, -1, -1
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
		}
	}
	if nameIdx < 0 {
		return nil, fmt.Errorf("no 'name' column in: %s cols=%v", query, cols)
	}
	if fixedKind == "" && kindIdx < 0 {
		return nil, fmt.Errorf("no 'kind' column in: %s cols=%v", query, cols)
	}

	captureArgs := fixedKind == "PROCEDURE" || fixedKind == "FUNCTION"

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
		kind := fixedKind
		if kind == "" {
			kind = fmt.Sprintf("%v", vals[kindIdx])
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

	q := fmt.Sprintf("%s.%s", QuoteIdent(database), QuoteIdent(schema))
	objs, err := c.showInSchema(ctx, fmt.Sprintf("SHOW OBJECTS IN SCHEMA %s", q), "", schema)
	if err != nil {
		return nil, err
	}
	c.putObjectCache(cacheKey, objs)
	return objs, nil
}

// ListExtendedObjects returns the "extended" objects inside a schema by running
// dedicated SHOW commands for object types not covered by SHOW OBJECTS
// (PROCEDURE, FUNCTION, TASK, STREAM, STAGE, FILE FORMAT, PIPE, NOTEBOOK,
// SECRET, GIT REPOSITORY). Individual commands that fail (e.g. due to missing
// privileges) are silently skipped. Includes the TASK finalize enrichment logic.
func (c *Client) ListExtendedObjects(ctx context.Context, database, schema string) ([]SnowflakeObject, error) {
	q := fmt.Sprintf("%s.%s", QuoteIdent(database), QuoteIdent(schema))

	type showCmd struct {
		query string
		kind  string
	}
	commands := []showCmd{
		{fmt.Sprintf("SHOW PROCEDURES IN SCHEMA %s", q), "PROCEDURE"},
		{fmt.Sprintf("SHOW FUNCTIONS IN SCHEMA %s", q), "FUNCTION"},
		{fmt.Sprintf("SHOW TASKS IN SCHEMA %s", q), "TASK"},
		{fmt.Sprintf("SHOW STREAMS IN SCHEMA %s", q), "STREAM"},
		{fmt.Sprintf("SHOW STAGES IN SCHEMA %s", q), "STAGE"},
		{fmt.Sprintf("SHOW FILE FORMATS IN SCHEMA %s", q), "FILE FORMAT"},
		{fmt.Sprintf("SHOW PIPES IN SCHEMA %s", q), "PIPE"},
		{fmt.Sprintf("SHOW NOTEBOOKS IN SCHEMA %s", q), "NOTEBOOK"},
		{fmt.Sprintf("SHOW SECRETS IN SCHEMA %s", q), "SECRET"},
		{fmt.Sprintf("SHOW GIT REPOSITORIES IN SCHEMA %s", q), "GIT REPOSITORY"},
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

	// GET_DDL fallback: for Snowflake editions that don't expose the FINALIZE
	// relationship via SHOW TASKS columns (task_relations / finalize), call
	// GET_DDL on standalone TASK objects to detect finalizer tasks.
	// "Standalone" = no predecessors AND no other task depends on it (not a root).
	enrichTaskFinalize(ctx, c, database, schema, all)

	return all, nil
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
	cacheKey := database + "\x00" + schema
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
	all := append(basic, extended...)
	c.putObjectCache(cacheKey, all)
	return all, nil
}

// getObjectCache returns a cached result if it exists and hasn't expired.
func (c *Client) getObjectCache(key string) ([]SnowflakeObject, bool) {
	c.objectCacheMu.RLock()
	defer c.objectCacheMu.RUnlock()
	entry, ok := c.objectCache[key]
	if !ok || time.Since(entry.ts) > objectCacheTTL {
		return nil, false
	}
	return entry.objects, true
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

// ClearObjectCacheForSchema removes cached object listings for a specific schema.
func (c *Client) ClearObjectCacheForSchema(database, schema string) {
	c.objectCacheMu.Lock()
	defer c.objectCacheMu.Unlock()
	delete(c.objectCache, database+"\x00"+schema)
	delete(c.objectCache, "basic\x00"+database+"\x00"+schema)
}

// ListFileFormats returns the names of all file formats in the specified schema.
func (c *Client) ListFileFormats(ctx context.Context, database, schema string) ([]string, error) {
	q := fmt.Sprintf("%s.%s", QuoteIdent(database), QuoteIdent(schema))
	return c.queryStringSlice(ctx, fmt.Sprintf("SHOW FILE FORMATS IN SCHEMA %s", q), 1)
}

// GetObjectDDL returns the definition of a single schema object using
// GET_DDL('<kind>', '<db>.<schema>.<name>'). The name components are
// double-quote escaped to handle mixed-case and special characters.
//
// For procedures and functions the arguments parameter must contain the
// parameter type list (e.g. "NUMBER, VARCHAR") so that Snowflake can resolve
// the correct overload. Pass an empty string for all other object kinds.
func (c *Client) GetObjectDDL(ctx context.Context, database, schema, kind, name, arguments string) (string, error) {
	qualified := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(name))
	// Procedures and functions require the argument type list (which may be
	// empty for zero-arg procedures) appended so Snowflake can resolve the
	// overload.  Omitting the parentheses entirely causes GET_DDL to return
	// "Object does not exist" even when the procedure exists.
	upperKind := strings.ToUpper(kind)
	if upperKind == "PROCEDURE" || upperKind == "FUNCTION" {
		qualified += fmt.Sprintf("(%s)", arguments)
	}
	escapedKind := strings.ReplaceAll(kind, "'", "''")
	query := fmt.Sprintf("SELECT GET_DDL('%s', '%s', true)", escapedKind, strings.ReplaceAll(qualified, "'", "''"))

	row := c.db.QueryRowContext(ctx, query)
	var src string
	if err := row.Scan(&src); err != nil {
		return "", fmt.Errorf("GET_DDL(%s %s): %w", kind, qualified, err)
	}
	return src, nil
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
	// GET_DDL does not support bind parameters for its object-name argument, so
	// we must interpolate it directly — escaping single quotes by doubling them.
	escaped := strings.ReplaceAll(database, "'", "''")
	query := fmt.Sprintf("SELECT GET_DDL('DATABASE', '%s', true)", escaped)

	row := c.db.QueryRowContext(ctx, query)
	var ddl string
	if err := row.Scan(&ddl); err != nil {
		return "", fmt.Errorf("GET_DDL(%s): %w", database, err)
	}
	return ddl, nil
}

// loadPrivateKey reads a PEM-encoded RSA private key from disk.
// If passphrase is non-empty, the key is assumed to be encrypted.
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
		rows, err := c.db.QueryContext(ctx, query)
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
		rows, err := c.db.QueryContext(ctx, query)
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
		rows, err := c.db.QueryContext(ctx, query)
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
	rows, err := c.db.QueryContext(ctx, query)
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
	tableRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(params.Database), QuoteIdent(params.Schema), QuoteIdent(params.Table))

	// Create a temporary stage (auto-dropped when the session ends)
	if _, err := c.db.ExecContext(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return ExportTableResult{}, fmt.Errorf("create export stage: %w", err)
	}
	// Explicit cleanup — also runs on error paths
	defer c.db.ExecContext(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// COPY data into the stage
	copySQL := buildExportCopySQL(stageAt, tableRef, params)
	copyRows, err := c.db.QueryContext(ctx, copySQL)
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
	getRows, err := c.db.QueryContext(ctx, getSQL)
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
	tableRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(params.Database), QuoteIdent(params.Schema), QuoteIdent(params.Table))

	if _, err := c.db.ExecContext(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return ImportTableResult{}, fmt.Errorf("create import stage: %w", err)
	}
	defer c.db.ExecContext(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// PUT each local file to the stage.
	for _, fp := range params.FilePaths {
		fileURL, err := localFileURLForFile(fp)
		if err != nil {
			return ImportTableResult{}, fmt.Errorf("build file url for %s: %w", filepath.Base(fp), err)
		}
		escapedURL := strings.ReplaceAll(fileURL, "'", "\\'")
		putSQL := fmt.Sprintf("PUT '%s' %s AUTO_COMPRESS=FALSE OVERWRITE=TRUE", escapedURL, stageAt)
		putRows, err := c.db.QueryContext(ctx, putSQL)
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
		if _, err := c.db.ExecContext(ctx, createFmtSQL); err != nil {
			return ImportTableResult{}, fmt.Errorf("create file format: %w", err)
		}
		defer c.db.ExecContext(context.Background(), "DROP FILE FORMAT IF EXISTS "+fmtRef) //nolint:errcheck
		inferFmtRef = fmtRef
	}

	if params.CreateTable {
		createSQL := buildCreateTableSQL(stageAt, tableRef, inferFmtRef, params)
		if _, err := c.db.ExecContext(ctx, createSQL); err != nil {
			return ImportTableResult{}, fmt.Errorf("create table: %w", err)
		}
	} else if params.Overwrite {
		if _, err := c.db.ExecContext(ctx, "TRUNCATE TABLE IF EXISTS "+tableRef); err != nil {
			return ImportTableResult{}, fmt.Errorf("truncate table: %w", err)
		}
	}

	copySQL := buildImportCopySQL(stageAt, tableRef, params)
	copyRows, err := c.db.QueryContext(ctx, copySQL)
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
		ffClause = fmt.Sprintf("FILE_FORMAT=>'%s'", strings.ReplaceAll(fmtRef, "'", "''"))
	} else if p.NamedFormat != "" {
		// User-selected existing named format.
		qualifiedFmt := QuoteIdent(p.Database) + "." + QuoteIdent(p.Schema) + "." + QuoteIdent(p.NamedFormat)
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
		qualifiedFmt := QuoteIdent(p.Database) + "." + QuoteIdent(p.Schema) + "." + QuoteIdent(p.NamedFormat)
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
	notebookRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(name))

	// DESC NOTEBOOK to find the stage URI of the latest version.
	descRows, err := c.db.QueryContext(ctx, "DESC NOTEBOOK "+notebookRef)
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
	getRows, err := c.db.QueryContext(ctx, getSQL)
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
	notebookRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(name))

	args := make([]string, len(params))
	for i, p := range params {
		args[i] = "'" + strings.ReplaceAll(p, "'", "''") + "'"
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
	ref := fmt.Sprintf("%s.%s.%s", QuoteIdent(database), QuoteIdent(schema), QuoteIdent(name))

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
	like := strings.ReplaceAll(name, "'", "''")
	sql := fmt.Sprintf("SHOW NOTEBOOKS LIKE '%s' IN SCHEMA %s.%s", like, QuoteIdent(database), QuoteIdent(schema))
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
	notebookRef := fmt.Sprintf(`%s.%s.%s`, QuoteIdent(database), QuoteIdent(schema), QuoteIdent(name))
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
	escapeLit := func(s string) string { return strings.ReplaceAll(s, "'", "''") }

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

	if _, err := c.db.ExecContext(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return fmt.Errorf("create notebook stage: %w", err)
	}
	defer c.db.ExecContext(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// PUT the .ipynb file to the stage.
	fileURL, err := localFileURLForFile(params.FilePath)
	if err != nil {
		return fmt.Errorf("build file url: %w", err)
	}
	escapedURL := strings.ReplaceAll(fileURL, "'", "\\'")
	putSQL := fmt.Sprintf("PUT '%s' %s AUTO_COMPRESS=FALSE OVERWRITE=TRUE", escapedURL, stageAt)
	putRows, err := c.db.QueryContext(ctx, putSQL)
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
	sb.WriteString(fmt.Sprintf("\n  MAIN_FILE = '%s'", escapeLit(mainFile)))
	if params.Comment != "" {
		sb.WriteString(fmt.Sprintf("\n  COMMENT = '%s'", escapeLit(params.Comment)))
	}
	if params.QueryWarehouse != "" {
		sb.WriteString(fmt.Sprintf("\n  QUERY_WAREHOUSE = %s", params.QueryWarehouse))
	}
	if params.IdleAutoShutdownSeconds > 0 {
		sb.WriteString(fmt.Sprintf("\n  IDLE_AUTO_SHUTDOWN_TIME_SECONDS = %d", params.IdleAutoShutdownSeconds))
	}
	if params.RuntimeName != "" {
		sb.WriteString(fmt.Sprintf("\n  RUNTIME_NAME = '%s'", escapeLit(params.RuntimeName)))
	}
	if params.ComputePool != "" {
		sb.WriteString(fmt.Sprintf("\n  COMPUTE_POOL = '%s'", escapeLit(params.ComputePool)))
	}
	if params.Warehouse != "" {
		sb.WriteString(fmt.Sprintf("\n  WAREHOUSE = %s", params.Warehouse))
	}

	if _, err := c.db.ExecContext(ctx, sb.String()); err != nil {
		return fmt.Errorf("create notebook: %w", err)
	}
	return nil
}
