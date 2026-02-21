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
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	RowsAffected int64     `json:"rowsAffected"`
}

// Client wraps a *sql.DB with Snowflake-specific helpers.
type Client struct {
	db *sql.DB
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

	// Tune the connection pool so that many concurrent DDL fetches are possible
	// without opening an unbounded number of connections to Snowflake.
	db.SetMaxOpenConns(32)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return &Client{db: db}, nil
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
		last = QueryResult{Columns: cols}

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

// UseRole switches the active role for the current session.
func (c *Client) UseRole(ctx context.Context, role string) error {
	escaped := strings.ReplaceAll(role, `"`, `""`)
	_, err := c.db.ExecContext(ctx, fmt.Sprintf(`USE ROLE "%s"`, escaped))
	return err
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
	query := fmt.Sprintf("SELECT GET_DDL('%s', '%s')", escapedKind, strings.ReplaceAll(qualified, "'", "''"))

	row := c.db.QueryRowContext(ctx, query)
	var src string
	if err := row.Scan(&src); err != nil {
		return "", fmt.Errorf("GET_DDL(%s %s): %w", kind, qualified, err)
	}
	return src, nil
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
	query := fmt.Sprintf("SELECT GET_DDL('DATABASE', '%s')", escaped)

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
