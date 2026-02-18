package snowflake

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	sf "github.com/snowflakedb/gosnowflake"
)

// ConnectParams holds all fields needed to open a Snowflake connection.
type ConnectParams struct {
	Account   string `json:"account"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Role      string `json:"role"`
	Warehouse string `json:"warehouse"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
}

// SnowflakeObject represents a database object (table, view, etc.).
type SnowflakeObject struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Schema string `json:"schema"`
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

// NewClient opens a new Snowflake connection.
func NewClient(p ConnectParams) (*Client, error) {
	dsn, err := sf.DSN(&sf.Config{
		Account:   p.Account,
		User:      p.User,
		Password:  p.Password,
		Role:      p.Role,
		Warehouse: p.Warehouse,
		Database:  p.Database,
		Schema:    p.Schema,
	})
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

	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
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

// Execute runs an arbitrary SQL statement and returns a QueryResult.
func (c *Client) Execute(ctx context.Context, query string) (*QueryResult, error) {
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result QueryResult
	result.Columns = cols

	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, vals)
	}

	return &result, rows.Err()
}

// ListDatabases returns all databases the user can see.
func (c *Client) ListDatabases(ctx context.Context) ([]string, error) {
	return c.queryStringSlice(ctx, "SHOW DATABASES", 1)
}

// ListSchemas returns schemas inside a database.
func (c *Client) ListSchemas(ctx context.Context, database string) ([]string, error) {
	return c.queryStringSlice(ctx, fmt.Sprintf("SHOW SCHEMAS IN DATABASE %s", database), 1)
}

// ListObjects returns tables and views inside a schema.
func (c *Client) ListObjects(ctx context.Context, database, schema string) ([]SnowflakeObject, error) {
	rows, err := c.db.QueryContext(ctx,
		fmt.Sprintf("SHOW OBJECTS IN SCHEMA %s.%s", database, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, _ := rows.Columns()
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
		// SHOW OBJECTS columns: created_on, name, database_name, schema_name, kind, ...
		name := fmt.Sprintf("%v", vals[1])
		kind := fmt.Sprintf("%v", vals[4])
		objects = append(objects, SnowflakeObject{Name: name, Kind: kind, Schema: schema})
	}
	return objects, rows.Err()
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
