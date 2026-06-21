// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"fmt"
	"strings"
	"thaw/internal/apperrors"
	"thaw/internal/objects"
	"thaw/internal/snowflake"
)

// DropDatabase drops a database. mode must be "CASCADE" or "RESTRICT".
func (a *App) DropDatabase(name string, mode string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return a.client.DropDatabase(a.ctx, name, mode)
}

// DropSchema drops a schema. mode must be "CASCADE" or "RESTRICT".
func (a *App) DropSchema(database, schema string, mode string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return a.client.DropSchema(a.ctx, database, schema, mode)
}

// ListDatabases returns all databases visible to the current role.
func (a *App) ListDatabases() ([]string, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListDatabases(a.ctx)
}

// ListSchemas returns all schemas in the given database.
func (a *App) ListSchemas(database string) ([]string, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListSchemas(a.ctx, database)
}

// ListFileFormats returns all file formats in the given schema.
func (a *App) ListFileFormats(database, schema string) ([]string, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListFileFormats(a.ctx, database, schema)
}

// ListObjects returns tables, views, etc. inside a schema.
func (a *App) ListObjects(database, schema string) ([]snowflake.SnowflakeObject, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListObjects(a.ctx, database, schema)
}

// ListBasicObjects returns the basic objects (TABLE, VIEW, SEQUENCE, etc.)
// inside a schema via a single SHOW OBJECTS IN SCHEMA command.
func (a *App) ListBasicObjects(database, schema string) ([]snowflake.SnowflakeObject, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListBasicObjects(a.ctx, database, schema)
}

// ClearObjectCache removes all cached object listings from the Snowflake client,
// forcing the next ListObjects/ListBasicObjects call to re-query Snowflake.
func (a *App) ClearObjectCache() {
	if a.client == nil {
		return
	}
	a.client.ClearObjectCache()
}

// ClearObjectCacheForDatabase removes all cached object listings for every
// schema under the given database, forcing subsequent calls to re-query Snowflake.
func (a *App) ClearObjectCacheForDatabase(database string) {
	if a.client == nil {
		return
	}
	a.client.ClearObjectCacheForDatabase(database)
}

// GetDatabaseRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given database. Returns 1 if the value cannot be determined.
func (a *App) GetDatabaseRetentionDays(dbName string) (int, error) {
	if a.client == nil {
		return 0, apperrors.ErrNotConnected
	}
	return a.client.GetDatabaseRetentionDays(a.ctx, dbName)
}

// GetSchemaRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given schema. Returns 1 if the value cannot be determined.
func (a *App) GetSchemaRetentionDays(database, schema string) (int, error) {
	if a.client == nil {
		return 0, apperrors.ErrNotConnected
	}
	return a.client.GetSchemaRetentionDays(a.ctx, database, schema)
}

// GetTableRetentionDays returns the Time Travel data retention period in days
// for the given table. Returns 1 if the value cannot be determined.
func (a *App) GetTableRetentionDays(database, schema, name string) (int, error) {
	if a.client == nil {
		return 0, apperrors.ErrNotConnected
	}
	return a.client.GetTableRetentionDays(a.ctx, database, schema, name)
}

// ListDroppedTables returns tables in the schema that are within the Time Travel
// retention window and can be recovered with UNDROP TABLE.
func (a *App) ListDroppedTables(database, schema string) ([]snowflake.DroppedTable, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListDroppedTables(a.ctx, database, schema)
}

// ListDroppedSchemas returns schemas in the database that are within the Time
// Travel retention window and can be recovered with UNDROP SCHEMA.
func (a *App) ListDroppedSchemas(database string) ([]snowflake.DroppedTable, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListDroppedSchemas(a.ctx, database)
}

// ListDroppedDatabases returns databases that are within the Time Travel
// retention window and can be recovered with UNDROP DATABASE.
func (a *App) ListDroppedDatabases() ([]snowflake.DroppedTable, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListDroppedDatabases(a.ctx)
}

// GetProcedureParams fetches the DDL for a stored procedure and returns its
// parameter list with real parameter names parsed from the DDL.
func (a *App) GetProcedureParams(database, schema, name, argTypes string) ([]snowflake.ProcParam, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetProcedureParams(a.ctx, database, schema, name, argTypes)
}

// GetTableColumns returns the ordered column names for a table or view.
func (a *App) GetTableColumns(database, schema, name string) ([]string, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetTableColumns(a.ctx, database, schema, name)
}

// GetTableForeignKeys returns the foreign keys where the given table is the
// referencing side. Used by the editor's JOIN ON autocomplete.
func (a *App) GetTableForeignKeys(database, schema, table string) ([]snowflake.TableForeignKey, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetTableForeignKeys(a.ctx, database, schema, table)
}

// GetTableColumnsWithTypes returns ordered column names and data types for a
// table or view. Used by the editor's JOIN ON autocomplete for type-compatible
// same-name column suggestions.
func (a *App) GetTableColumnsWithTypes(database, schema, name string) ([]snowflake.ColumnInfo, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetTableColumnsWithTypes(a.ctx, database, schema, name)
}

// ListGitRepoEntries returns the immediate children (files and directories) at
// dirPath inside the git repository stage @database.schema.repoName/dirPath.
// Pass an empty dirPath to list the root.
func (a *App) ListGitRepoEntries(database, schema, repoName, dirPath string) ([]snowflake.GitRepoEntry, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}

	// If we are listing the root of the "commits" virtual folder,
	// check if a filter has been set.
	if strings.HasPrefix(dirPath, "commits") {
		parts := strings.Split(strings.Trim(dirPath, "/"), "/")
		if len(parts) == 1 { // listing "commits/"
			a.gitCommitFiltersMu.Lock()
			repoKey := fmt.Sprintf("%s.%s.%s", database, schema, repoName)
			commitHash := a.gitCommitFilters[repoKey]
			a.gitCommitFiltersMu.Unlock()

			if commitHash == "" {
				// Return an empty list or a special entry indicating no filter?
				// For now, return empty. The frontend will handle the "Set Filter" UI.
				return []snowflake.GitRepoEntry{}, nil
			}
			// If we have a hash, we want to list @repo/commits/<hash>/
			// but ListGitRepoEntries will be called with "commits/<hash>/" next.
		}
	}

	return a.client.ListGitRepoEntries(a.ctx, database, schema, repoName, dirPath)
}

// ListGitBranches returns all branches in the given git repository.
func (a *App) ListGitBranches(database, schema, repoName string) ([]snowflake.GitBranch, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListGitBranches(a.ctx, database, schema, repoName)
}

// ListGitTags returns all tags in the given git repository.
func (a *App) ListGitTags(database, schema, repoName string) ([]snowflake.GitTag, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.ListGitTags(a.ctx, database, schema, repoName)
}

// SetGitCommitFilter sets a commit hash filter for a specific repository.
func (a *App) SetGitCommitFilter(database, schema, repoName, commitHash string) {
	a.gitCommitFiltersMu.Lock()
	defer a.gitCommitFiltersMu.Unlock()
	repoKey := fmt.Sprintf("%s.%s.%s", database, schema, repoName)
	if commitHash == "" {
		delete(a.gitCommitFilters, repoKey)
	} else {
		a.gitCommitFilters[repoKey] = commitHash
	}
}

// GetGitCommitFilter returns the current commit hash filter for a repository.
func (a *App) GetGitCommitFilter(database, schema, repoName string) string {
	a.gitCommitFiltersMu.Lock()
	defer a.gitCommitFiltersMu.Unlock()
	repoKey := fmt.Sprintf("%s.%s.%s", database, schema, repoName)
	return a.gitCommitFilters[repoKey]
}

// GetGitFileContent reads a file from a git repository and returns its content.
func (a *App) GetGitFileContent(database, schema, repoName, filePath string) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
	}
	return a.client.GetGitFileContent(a.ctx, database, schema, repoName, filePath)
}

// ExecuteGitFile executes a SQL file from a git repository.
func (a *App) ExecuteGitFile(database, schema, repoName, filePath string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return a.client.ExecuteGitFile(a.ctx, database, schema, repoName, filePath)
}

// GetSchemaForeignKeys returns all FK→PK column mappings in the given schema
// from INFORMATION_SCHEMA. Used by the editor to bulk-warm FK data for the
// JOIN ON autocomplete instead of issuing per-table SHOW IMPORTED KEYS calls.
func (a *App) GetSchemaForeignKeys(database, schema string) ([]snowflake.TableForeignKey, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetSchemaForeignKeys(a.ctx, database, schema)
}

// GetFunctionInfo fetches the DDL for a user-defined function and returns its
// parameter list together with a flag indicating whether it is a table function.
func (a *App) GetFunctionInfo(database, schema, name, argTypes string) (*snowflake.FunctionInfo, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.GetFunctionInfo(a.ctx, database, schema, name, argTypes)
}

// GetObjectDDL returns the definition of a single schema object using
// Snowflake's GET_DDL function. kind should be one of: TABLE, VIEW,
// DYNAMIC TABLE, EXTERNAL TABLE, MATERIALIZED VIEW, ALERT, TAG, FUNCTION,
// PROCEDURE, SEQUENCE, STAGE, STREAM, TASK, FILE FORMAT, PIPE.
// For procedures and functions, arguments must be the parameter type list
// (e.g. "NUMBER, VARCHAR") so Snowflake can resolve the correct overload.
// Pass an empty string for all other object kinds.
func (a *App) GetObjectDDL(database, schema, kind, name, arguments string) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
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
		return snowflake.DependencyNode{}, apperrors.ErrNotConnected
	}
	return a.client.GetObjectDependencies(a.ctx, database, schema, kind, name, arguments)
}

// GetObjectProperties returns structured metadata for any Snowflake object by
// running the appropriate SHOW or DESCRIBE command and returning the result as
// key/value pairs. kind is one of: TABLE, VIEW, DYNAMIC TABLE, EXTERNAL TABLE,
// MATERIALIZED VIEW, ALERT, TAG, FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM, TASK,
// FILE FORMAT, PIPE, WAREHOUSE, ROLE, USER.
func (a *App) GetObjectProperties(database, schema, kind, name string) ([]snowflake.PropertyPair, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return objects.GetObjectProperties(a.ctx, a.client, database, schema, kind, name)
}

// GetRoutineProperties returns SHOW metadata for one specific overload of a
// FUNCTION or PROCEDURE, selected by its argument-type signature (args, e.g.
// "NUMBER, VARCHAR"). Overloaded routines return one SHOW row per signature;
// this picks the matching one so the properties panel reflects the overload the
// user acted on rather than always the first. Pass the same signature threaded
// into AlterFunction / AlterProcedure.
func (a *App) GetRoutineProperties(database, schema, kind, name, args string) ([]snowflake.PropertyPair, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return objects.GetRoutineProperties(a.ctx, a.client, database, schema, kind, name, args)
}

// GetColumnComments returns the comment for every column in a table, ordered
// by ordinal position.
func (a *App) GetColumnComments(database, schema, table string) ([]objects.ColumnComment, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return objects.GetColumnComments(a.ctx, a.client, database, schema, table)
}

// SetColumnComment sets (or clears) the COMMENT on a single table column.
func (a *App) SetColumnComment(database, schema, table, column, comment string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	return objects.SetColumnComment(a.ctx, a.client, database, schema, table, column, comment)
}
