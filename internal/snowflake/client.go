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

	sf "github.com/snowflakedb/gosnowflake"
)

// ConnectParams holds all fields needed to open a Snowflake connection.
// Authenticator values:
//   "snowflake"            – password (+ optional TOTP passcode)
//   "username_password_mfa" – password + MFA push notification
//   "externalbrowser"      – browser-based SSO (no password needed)
//   "okta"                 – Okta native SSO (requires OktaURL)
//   "snowflake_jwt"        – key-pair / JWT (requires PrivateKeyPath)
type ConnectParams struct {
	Account              string `json:"account"`
	User                 string `json:"user"`
	Password             string `json:"password"`
	Role                 string `json:"role"`
	Warehouse            string `json:"warehouse"`
	Database             string `json:"database"`
	Schema               string `json:"schema"`
	Authenticator        string `json:"authenticator"`
	// Passcode is a TOTP/hardware-token code; used with "snowflake" authenticator.
	Passcode             string `json:"passcode"`
	// OktaURL is the Okta account URL; required for "okta" authenticator.
	OktaURL              string `json:"oktaUrl"`
	// PrivateKeyPath is the path to a PEM-encoded private key; required for "snowflake_jwt".
	PrivateKeyPath       string `json:"privateKeyPath"`
	// PrivateKeyPassphrase decrypts the private key if it is encrypted.
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`
}

// SnowflakeObject represents a database object (table, view, etc.).
type SnowflakeObject struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Schema    string `json:"schema"`
	// Arguments holds the parameter type list for procedures and functions,
	// e.g. "NUMBER, VARCHAR". Empty for all other object kinds.
	Arguments string `json:"arguments"`
}

// QueryResult is the serialisable result of a SQL query.
type QueryResult struct {
	Columns      []string        `json:"columns"`
	Rows         [][]interface{} `json:"rows"`
	RowsAffected int64           `json:"rowsAffected"`
	QueryID      string          `json:"queryID"`
}

// Client wraps a *sql.DB with Snowflake-specific helpers.
//
// A single persistent connection (MaxOpenConns=1) is used intentionally so
// that session-level statements such as USE ROLE and USE WAREHOUSE apply
// consistently to every subsequent query. With a connection pool of N>1,
// those statements would only affect whichever connection happened to execute
// them, leaving other connections in the pool with the original session state.
type Client struct {
	db          *sql.DB
	mu          sync.RWMutex
	currentRole string // tracks the effective role after every UseRole call
}

// NewClient opens a new Snowflake connection. The provided context can be
// cancelled to abort the login handshake (useful for MFA/browser flows).
func NewClient(ctx context.Context, p ConnectParams) (*Client, error) {
	authMap := map[string]sf.AuthType{
		"username_password_mfa": sf.AuthTypeUsernamePasswordMFA,
		"externalbrowser":       sf.AuthTypeExternalBrowser,
		"okta":                  sf.AuthTypeOkta,
		"snowflake_jwt":         sf.AuthTypeJwt,
	}
	auth, ok := authMap[p.Authenticator]
	if !ok {
		auth = sf.AuthTypeSnowflake
	}

	// Interactive flows need more time; plain password should fail quickly.
	// LoginTimeout is the gosnowflake-internal control — context cancellation
	// alone is not reliable for aborting auth inside the driver.
	loginTimeout := 15 * time.Second
	if p.Authenticator == "username_password_mfa" ||
		p.Authenticator == "externalbrowser" ||
		p.Authenticator == "okta" {
		loginTimeout = 3 * time.Minute
	}

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

	dsn, err := sf.DSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dsn: %w", err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}

	// Use a single persistent connection so that USE ROLE / USE WAREHOUSE apply
	// to every subsequent query.  With MaxOpenConns>1, those session-level
	// statements only affect the connection that ran them; other connections in
	// the pool keep the original DSN role.  ConnMaxLifetime=0 prevents the
	// driver from silently replacing the connection (which would reset the role
	// back to the DSN default).
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// Record the initial role so CanCreateUsers/CanManageUsers can use it
	// without a round-trip.
	var initialRole string
	_ = db.QueryRowContext(ctx, "SELECT CURRENT_ROLE()").Scan(&initialRole)

	return &Client{db: db, currentRole: strings.TrimSpace(initialRole)}, nil
}

// IsAlive checks that the underlying connection is still usable.
func (c *Client) IsAlive() bool {
	return c.db.PingContext(context.Background()) == nil
}

// Close terminates the connection pool.
func (c *Client) Close() error {
	return c.db.Close()
}

// Execute runs one or more semicolon-separated SQL statements and returns the
// last result set. Using sf.WithMultiStatement(ctx, 0) tells the driver to
// accept any number of statements in a single call.
func (c *Client) Execute(ctx context.Context, query string) (*QueryResult, error) {
	multiCtx, err := sf.WithMultiStatement(ctx, 0)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(multiCtx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var last QueryResult
	for {
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		last = QueryResult{Columns: cols, Rows: [][]interface{}{}}

		for rows.Next() {
			vals := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return nil, err
			}
			last.Rows = append(last.Rows, vals)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if !rows.NextResultSet() {
			break
		}
	}

	return &last, nil
}

// SessionContext holds the current session's active role, warehouse, database and schema.
type SessionContext struct {
	Role      string `json:"role"`
	Warehouse string `json:"warehouse"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
}

// GetSessionContext returns the currently active role, warehouse, database, and schema.
func (c *Client) GetSessionContext(ctx context.Context) (SessionContext, error) {
	row := c.db.QueryRowContext(ctx,
		"SELECT CURRENT_ROLE(), CURRENT_WAREHOUSE(), CURRENT_DATABASE(), CURRENT_SCHEMA()")
	var sc SessionContext
	if err := row.Scan(&sc.Role, &sc.Warehouse, &sc.Database, &sc.Schema); err != nil {
		return SessionContext{}, err
	}
	return sc, nil
}

// ListRoles returns the names of all roles available to the current user.
func (c *Client) ListRoles(ctx context.Context) ([]string, error) {
	// SHOW ROLES columns: created_on, name, ...
	return c.queryStringSlice(ctx, "SHOW ROLES", 1)
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

// GetUserDDL constructs a CREATE USER DDL statement for the given user by
// running DESCRIBE USER and translating the property/value pairs.
func (c *Client) GetUserDDL(ctx context.Context, name string) (string, error) {
	escIdent := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	sq := func(s string) string { return `'` + strings.ReplaceAll(s, `'`, `''`) + `'` }

	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`DESCRIBE USER "%s"`, escIdent(name)))
	if err != nil {
		return "", err
	}
	defer rows.Close()

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
			lines = append(lines, fmt.Sprintf("    %s = %s", prop, sq(v)))
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
		lines = append(lines, fmt.Sprintf("    LOGIN_NAME = %s", sq(v)))
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

	sql := fmt.Sprintf("CREATE USER \"%s\"", escIdent(name))
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
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf(`SHOW GRANTS TO ROLE "%s"`, esc))
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

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

// CanCreateUsers returns (true, nil) when the current session role (or any
// role it inherits) allows creating users.
func (c *Client) CanCreateUsers(ctx context.Context) (bool, error) {
	c.mu.RLock()
	role := c.currentRole
	c.mu.RUnlock()

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

// CanManageUsers returns (true, nil) when the current session role (or any
// role it inherits) can ALTER or DROP other users.
func (c *Client) CanManageUsers(ctx context.Context) (bool, error) {
	c.mu.RLock()
	role := c.currentRole
	c.mu.RUnlock()

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
	escapedLike  := strings.ReplaceAll(name, "'", "''")
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
		rows.Close()
	}

	// ── CREATE ROLE ──────────────────────────────────────────────────────────
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE ROLE IF NOT EXISTS \"%s\"", escapedIdent))
	if comment != "" {
		sb.WriteString(fmt.Sprintf("\n  COMMENT = '%s'",
			strings.ReplaceAll(comment, "'", "''")))
	}
	sb.WriteString(";\n")

	// ── SHOW GRANTS TO ROLE → privileges granted to this role ────────────────
	if rows, err := c.db.QueryContext(ctx,
		fmt.Sprintf(`SHOW GRANTS TO ROLE "%s"`, escapedIdent)); err == nil {
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "privilege", "granted_on", "name", "grant_option")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if rows.Scan(ptrs...) != nil {
				continue
			}
			priv   := strVal(vals, idxs["privilege"])
			onType := strVal(vals, idxs["granted_on"])
			obj    := strVal(vals, idxs["name"])
			opt    := strings.EqualFold(strVal(vals, idxs["grant_option"]), "true")
			if priv == "" || onType == "" {
				continue
			}
			stmt := fmt.Sprintf("GRANT %s ON %s %s TO ROLE \"%s\"",
				priv, onType, obj, escapedIdent)
			if opt {
				stmt += " WITH GRANT OPTION"
			}
			sb.WriteString(stmt + ";\n")
		}
		rows.Close()
	}

	// ── SHOW GRANTS ON ROLE → who this role is granted to ────────────────────
	if rows, err := c.db.QueryContext(ctx,
		fmt.Sprintf(`SHOW GRANTS ON ROLE "%s"`, escapedIdent)); err == nil {
		cols, _ := rows.Columns()
		idxs := colIndexMap(cols, "granted_to", "grantee_name")
		for rows.Next() {
			vals, ptrs := makeValPtrs(len(cols))
			if rows.Scan(ptrs...) != nil {
				continue
			}
			grantedTo := strVal(vals, idxs["granted_to"])
			grantee   := strVal(vals, idxs["grantee_name"])
			if grantedTo == "" || grantee == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("GRANT ROLE \"%s\" TO %s \"%s\";\n",
				escapedIdent, grantedTo,
				strings.ReplaceAll(grantee, `"`, `""`)))
		}
		rows.Close()
	}

	return sb.String(), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

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
	if _, err := c.db.ExecContext(ctx, fmt.Sprintf(`USE ROLE "%s"`, escaped)); err != nil {
		return err
	}
	c.mu.Lock()
	c.currentRole = role
	c.mu.Unlock()
	return nil
}

// UseWarehouse switches the active warehouse for the current session.
func (c *Client) UseWarehouse(ctx context.Context, warehouse string) error {
	escaped := strings.ReplaceAll(warehouse, `"`, `""`)
	_, err := c.db.ExecContext(ctx, fmt.Sprintf(`USE WAREHOUSE "%s"`, escaped))
	return err
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
	defer rows.Close()

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

	var result []string
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
	return c.queryStringSlice(ctx, fmt.Sprintf("SHOW SCHEMAS IN DATABASE %s", database), 1)
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
	esc := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	query := fmt.Sprintf(`SHOW TABLES HISTORY IN SCHEMA "%s"."%s"`, esc(database), esc(schema))

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	idxs := colIndexMap(cols, "name", "dropped_on")
	if idxs["name"] < 0 {
		return nil, fmt.Errorf("no 'name' column in SHOW TABLES HISTORY result")
	}

	var result []DroppedTable
	for rows.Next() {
		vals, ptrs := makeValPtrs(len(cols))
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		droppedOn := strVal(vals, idxs["dropped_on"])
		if droppedOn == "" {
			continue // table is still alive
		}
		result = append(result, DroppedTable{
			Name:      strVal(vals, idxs["name"]),
			DroppedOn: droppedOn,
		})
	}
	return result, rows.Err()
}

// GetTableRetentionDays returns the Time Travel data-retention period in days
// for a single table. It runs SHOW TABLES LIKE and reads the retention_time
// column. Returns 1 (Snowflake's Standard-edition default) when the value
// cannot be determined.
func (c *Client) GetTableRetentionDays(ctx context.Context, database, schema, name string) (int, error) {
	esc := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	query := fmt.Sprintf(`SHOW TABLES LIKE '%s' IN SCHEMA "%s"."%s"`,
		strings.ReplaceAll(name, "'", "''"), esc(database), esc(schema))

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

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
	defer rows.Close()

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
	esc := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	query := fmt.Sprintf(`DESCRIBE TABLE "%s"."%s"."%s"`, esc(database), esc(schema), esc(name))
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
	defer rows.Close()

	cols, _ := rows.Columns()
	nameIdx, kindIdx, argsIdx, builtinIdx := -1, -1, -1, -1
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
		objects = append(objects, SnowflakeObject{Name: name, Kind: kind, Schema: schema, Arguments: argTypes})
	}
	return objects, rows.Err()
}

// ListObjects returns all objects inside a schema by running multiple SHOW
// commands concurrently. Individual commands that fail (e.g. due to missing
// privileges on a particular object type) are silently skipped so that the
// rest still appear.
func (c *Client) ListObjects(ctx context.Context, database, schema string) ([]SnowflakeObject, error) {
	q := fmt.Sprintf("%s.%s", database, schema)

	type showCmd struct {
		query string
		kind  string // empty → read from result's "kind" column
	}
	commands := []showCmd{
		{fmt.Sprintf("SHOW OBJECTS IN SCHEMA %s", q), ""},
		{fmt.Sprintf("SHOW PROCEDURES IN SCHEMA %s", q), "PROCEDURE"},
		{fmt.Sprintf("SHOW FUNCTIONS IN SCHEMA %s", q), "FUNCTION"},
		{fmt.Sprintf("SHOW TASKS IN SCHEMA %s", q), "TASK"},
		{fmt.Sprintf("SHOW STREAMS IN SCHEMA %s", q), "STREAM"},
		{fmt.Sprintf("SHOW STAGES IN SCHEMA %s", q), "STAGE"},
		{fmt.Sprintf("SHOW FILE FORMATS IN SCHEMA %s", q), "FILE FORMAT"},
		{fmt.Sprintf("SHOW PIPES IN SCHEMA %s", q), "PIPE"},
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
	return all, nil
}

// GetObjectDDL returns the definition of a single schema object using
// GET_DDL('<kind>', '<db>.<schema>.<name>'). The name components are
// double-quote escaped to handle mixed-case and special characters.
//
// For procedures and functions the arguments parameter must contain the
// parameter type list (e.g. "NUMBER, VARCHAR") so that Snowflake can resolve
// the correct overload. Pass an empty string for all other object kinds.
func (c *Client) GetObjectDDL(ctx context.Context, database, schema, kind, name, arguments string) (string, error) {
	escapeIdent := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	qualified := fmt.Sprintf(`"%s"."%s"."%s"`, escapeIdent(database), escapeIdent(schema), escapeIdent(name))
	// Procedures and functions require the argument type list appended to the
	// qualified name so Snowflake can resolve the right overload.
	upperKind := strings.ToUpper(kind)
	if (upperKind == "PROCEDURE" || upperKind == "FUNCTION") && arguments != "" {
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

// GetCompleteDatabaseDDL returns DDL for all objects in a database.
//
// It combines two sources:
//  1. GET_DDL('DATABASE', ...) — fast single-call coverage for schemas,
//     tables, views, sequences, functions, and procedures.
//  2. Per-schema SHOW + individual GET_DDL for object types that the
//     database-level GET_DDL does not reliably include: stages, streams,
//     tasks, file formats, and pipes.
//
// The extra calls run concurrently across all schemas. Individual GET_DDL
// failures (e.g. permission denied on a specific object) are silently skipped
// so that one inaccessible object does not abort the whole database export.
func (c *Client) GetCompleteDatabaseDDL(ctx context.Context, database string) (string, error) {
	mainDDL, err := c.GetDatabaseDDL(ctx, database)
	if err != nil {
		return "", err
	}

	schemas, err := c.ListSchemas(ctx, database)
	if err != nil {
		// Cannot list schemas — return what we have from database-level GET_DDL.
		return mainDDL, nil
	}

	type extraKind struct {
		verb string // SHOW <verb> IN SCHEMA …
		kind string // GET_DDL('<kind>', …)
	}
	extras := []extraKind{
		{"STAGES", "STAGE"},
		{"STREAMS", "STREAM"},
		{"TASKS", "TASK"},
		{"FILE FORMATS", "FILE FORMAT"},
		{"PIPES", "PIPE"},
	}

	var (
		mu       sync.Mutex
		extraDDL []string
		wg       sync.WaitGroup
	)

	for _, schema := range schemas {
		for _, ek := range extras {
			wg.Add(1)
			go func(schema string, ek extraKind) {
				defer wg.Done()
				q := fmt.Sprintf("SHOW %s IN SCHEMA %s.%s", ek.verb, database, schema)
				objs, err := c.showInSchema(ctx, q, ek.kind, schema)
				if err != nil {
					return
				}
				for _, obj := range objs {
					src, err := c.GetObjectDDL(ctx, database, schema, ek.kind, obj.Name, "")
					if err != nil {
						continue // skip inaccessible objects
					}
					d := strings.TrimRight(src, ";\n ")
					mu.Lock()
					extraDDL = append(extraDDL, d)
					mu.Unlock()
				}
			}(schema, ek)
		}
	}
	wg.Wait()

	if len(extraDDL) == 0 {
		return mainDDL, nil
	}

	var sb strings.Builder
	sb.WriteString(mainDDL)
	for _, d := range extraDDL {
		sb.WriteString("\n")
		sb.WriteString(d)
		sb.WriteString(";\n")
	}
	return sb.String(), nil
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
	esc := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
	db := esc(database)

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
				` FROM "%s".INFORMATION_SCHEMA.COLUMNS c`+
				` JOIN "%s".INFORMATION_SCHEMA.TABLES t`+
				` ON c.TABLE_SCHEMA = t.TABLE_SCHEMA AND c.TABLE_NAME = t.TABLE_NAME`+
				` WHERE c.TABLE_SCHEMA != 'INFORMATION_SCHEMA'`+
				` AND t.TABLE_TYPE = 'BASE TABLE'`+
				` ORDER BY c.TABLE_SCHEMA, c.TABLE_NAME, c.ORDINAL_POSITION`, db, db)
		rows, err := c.db.QueryContext(ctx, query)
		if err != nil {
			colErr = err
			return
		}
		defer rows.Close()
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
		query := fmt.Sprintf(`SHOW PRIMARY KEYS IN DATABASE "%s"`, db)
		rows, err := c.db.QueryContext(ctx, query)
		if err != nil {
			pkErr = err
			return
		}
		defer rows.Close()
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
		query := fmt.Sprintf(`SHOW IMPORTED KEYS IN DATABASE "%s"`, db)
		rows, err := c.db.QueryContext(ctx, query)
		if err != nil {
			fkErr = err
			return
		}
		defer rows.Close()
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
	defer rows.Close()

	cols, _ := rows.Columns()
	var result []string
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
	esc := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }

	// Unique stage name — timestamp-based, no external uuid package required
	stageName := fmt.Sprintf("THAW_EXPORT_%d", time.Now().UnixNano())
	stageRef := fmt.Sprintf(`"%s"."%s".%s`, esc(params.Database), esc(params.Schema), stageName)
	stageAt := "@" + stageRef
	tableRef := fmt.Sprintf(`"%s"."%s"."%s"`, esc(params.Database), esc(params.Schema), esc(params.Table))

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
	copyRows.Close()

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
	getRows.Close()

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

// ImportTableParams specifies how to import a local file into a Snowflake table.
type ImportTableParams struct {
	Database  string `json:"database"`
	Schema    string `json:"schema"`
	Table     string `json:"table"`    // target table name
	FilePath  string `json:"filePath"` // absolute local path to the source file
	Format    string `json:"format"`   // "CSV", "JSON", "PARQUET"
	// CSV-specific
	Delimiter  string `json:"delimiter"`
	Header     bool   `json:"header"`
	NullString string `json:"nullString"`
	// Behaviour
	Overwrite   bool `json:"overwrite"`   // TRUNCATE TABLE before COPY INTO
	CreateTable bool `json:"createTable"` // CREATE TABLE using INFER_SCHEMA first
}

// ImportTableResult reports the outcome of a table import.
type ImportTableResult struct {
	RowsLoaded  int64 `json:"rowsLoaded"`
	FilesLoaded int   `json:"filesLoaded"`
}

// ImportTableData imports a local file into a Snowflake table via a temporary
// internal stage:
//  1. CREATE TEMPORARY STAGE in the same schema as the table
//  2. PUT local file → stage
//  3. Optionally CREATE TABLE USING TEMPLATE (INFER_SCHEMA) or TRUNCATE
//  4. COPY INTO table FROM @stage
//  5. DROP STAGE (deferred)
func (c *Client) ImportTableData(ctx context.Context, params ImportTableParams) (ImportTableResult, error) {
	esc := func(s string) string { return strings.ReplaceAll(s, `"`, `""`) }

	stageName := fmt.Sprintf("THAW_IMPORT_%d", time.Now().UnixNano())
	stageRef  := fmt.Sprintf(`"%s"."%s".%s`, esc(params.Database), esc(params.Schema), stageName)
	stageAt   := "@" + stageRef
	tableRef  := fmt.Sprintf(`"%s"."%s"."%s"`, esc(params.Database), esc(params.Schema), esc(params.Table))

	if _, err := c.db.ExecContext(ctx, "CREATE TEMPORARY STAGE "+stageRef); err != nil {
		return ImportTableResult{}, fmt.Errorf("create import stage: %w", err)
	}
	defer c.db.ExecContext(context.Background(), "DROP STAGE IF EXISTS "+stageRef) //nolint:errcheck

	// PUT local file to the stage
	fileURL, err := localFileURLForFile(params.FilePath)
	if err != nil {
		return ImportTableResult{}, fmt.Errorf("build file url: %w", err)
	}
	escapedURL := strings.ReplaceAll(fileURL, "'", "\\'")
	putSQL := fmt.Sprintf("PUT '%s' %s AUTO_COMPRESS=FALSE OVERWRITE=TRUE", escapedURL, stageAt)
	putRows, err := c.db.QueryContext(ctx, putSQL)
	if err != nil {
		return ImportTableResult{}, fmt.Errorf("upload file to stage: %w", err)
	}
	putRows.Close()

	// Optionally create the target table from the file's inferred schema
	if params.CreateTable {
		createSQL := buildCreateTableSQL(stageAt, tableRef, params)
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
	copyRows.Close()

	return ImportTableResult{RowsLoaded: rowsLoaded, FilesLoaded: filesLoaded}, nil
}

// buildCreateTableSQL returns a CREATE TABLE statement that derives its schema
// from the staged file using INFER_SCHEMA (CSV/PARQUET) or creates a single
// VARIANT column for JSON (whose schema cannot be reliably inferred).
func buildCreateTableSQL(stageAt, tableRef string, p ImportTableParams) string {
	if strings.ToUpper(p.Format) == "JSON" {
		return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (VALUE VARIANT)", tableRef)
	}

	var inferFF string
	if strings.ToUpper(p.Format) == "CSV" {
		delim := p.Delimiter
		if delim == "" {
			delim = ","
		}
		delim = strings.ReplaceAll(delim, "'", "\\'")
		if p.Header {
			inferFF = fmt.Sprintf("TYPE='CSV' PARSE_HEADER=TRUE FIELD_DELIMITER='%s'", delim)
		} else {
			inferFF = fmt.Sprintf("TYPE='CSV' SKIP_HEADER=0 FIELD_DELIMITER='%s'", delim)
		}
	} else { // PARQUET
		inferFF = "TYPE='PARQUET'"
	}

	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s\nUSING TEMPLATE (\n    SELECT ARRAY_AGG(OBJECT_CONSTRUCT(*))\n    FROM TABLE(INFER_SCHEMA(\n        LOCATION=>'%s',\n        FILE_FORMAT=>(%s)\n    ))\n)",
		tableRef, stageAt, inferFF)
}

// buildImportCopySQL returns the COPY INTO <table> FROM @stage statement.
func buildImportCopySQL(stageAt, tableRef string, p ImportTableParams) string {
	esc := func(s string) string { return strings.ReplaceAll(s, "'", "\\'") }

	switch strings.ToUpper(p.Format) {
	case "JSON":
		if p.CreateTable {
			// Table has a single VARIANT column named VALUE; load each JSON line as $1.
			return fmt.Sprintf(
				"COPY INTO %s (VALUE)\nFROM (SELECT $1 FROM %s)\nFILE_FORMAT = (TYPE='JSON' COMPRESSION=AUTO)\nFORCE = TRUE",
				tableRef, stageAt)
		}
		// Existing table: match JSON keys to column names.
		return fmt.Sprintf(
			"COPY INTO %s\nFROM %s\nFILE_FORMAT = (TYPE='JSON' COMPRESSION=AUTO)\nMATCH_BY_COLUMN_NAME = CASE_INSENSITIVE\nFORCE = TRUE",
			tableRef, stageAt)

	case "PARQUET":
		return fmt.Sprintf(
			"COPY INTO %s\nFROM %s\nFILE_FORMAT = (TYPE='PARQUET')\nMATCH_BY_COLUMN_NAME = CASE_INSENSITIVE\nFORCE = TRUE",
			tableRef, stageAt)

	default: // CSV
		delim := p.Delimiter
		if delim == "" {
			delim = ","
		}
		delim = esc(delim)
		nullIf := esc(p.NullString)

		if p.CreateTable && p.Header {
			// Table was created with PARSE_HEADER; use the same so column order matches.
			ff := fmt.Sprintf(
				"TYPE='CSV' PARSE_HEADER=TRUE FIELD_DELIMITER='%s' FIELD_OPTIONALLY_ENCLOSED_BY='\"' NULL_IF=('%s') EMPTY_FIELD_AS_NULL=TRUE COMPRESSION=AUTO",
				delim, nullIf)
			return fmt.Sprintf(
				"COPY INTO %s\nFROM %s\nFILE_FORMAT = (%s)\nMATCH_BY_COLUMN_NAME = CASE_INSENSITIVE\nFORCE = TRUE",
				tableRef, stageAt, ff)
		}

		skipHeader := 0
		if p.Header {
			skipHeader = 1
		}
		ff := fmt.Sprintf(
			"TYPE='CSV' FIELD_DELIMITER='%s' FIELD_OPTIONALLY_ENCLOSED_BY='\"' NULL_IF=('%s') EMPTY_FIELD_AS_NULL=TRUE SKIP_HEADER=%d COMPRESSION=AUTO",
			delim, nullIf, skipHeader)
		return fmt.Sprintf(
			"COPY INTO %s\nFROM %s\nFILE_FORMAT = (%s)\nFORCE = TRUE",
			tableRef, stageAt, ff)
	}
}
