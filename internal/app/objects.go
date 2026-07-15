// SPDX-License-Identifier: GPL-3.0-or-later

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
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return client.DropDatabase(a.fctx(FeatureObjectBrowser), name, mode)
}

// DropSchema drops a schema. mode must be "CASCADE" or "RESTRICT".
func (a *App) DropSchema(database, schema string, mode string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return client.DropSchema(a.fctx(FeatureObjectBrowser), database, schema, mode)
}

// ListDatabases returns all databases visible to the current role.
func (a *App) ListDatabases() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListDatabases(a.fctx(FeatureObjectBrowser))
}

// ListUserDatabases returns the user-managed databases visible to the current
// role — all databases except shared / imported ones (non-empty origin, e.g.
// SNOWFLAKE_SAMPLE_DATA), which cannot be altered or swapped. Use it when
// offering databases as targets for DDL / governance operations (e.g. the
// SWAP WITH picker) rather than as a raw catalog list.
func (a *App) ListUserDatabases() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListUserDatabases(a.fctx(FeatureObjectBrowser))
}

// ListSchemas returns all schemas in the given database.
func (a *App) ListSchemas(database string) ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListSchemas(a.fctx(FeatureObjectBrowser), database)
}

// ListUserSchemas returns the user-managed schemas in the given database — all
// schemas except the read-only INFORMATION_SCHEMA. Use it when offering schemas
// as targets for object / DDL / governance operations (e.g. the Apply-tag
// picker) rather than as a raw catalog list.
func (a *App) ListUserSchemas(database string) ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListUserSchemas(a.fctx(FeatureObjectBrowser), database)
}

// ListFileFormats returns all file formats in the given schema.
func (a *App) ListFileFormats(database, schema string) ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListFileFormats(a.fctx(FeatureObjectBrowser), database, schema)
}

// ListObjects returns tables, views, etc. inside a schema.
func (a *App) ListObjects(database, schema string) ([]snowflake.SnowflakeObject, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListObjects(a.fctx(FeatureObjectBrowser), database, schema)
}

// ListBasicObjects returns the basic objects (TABLE, VIEW, SEQUENCE, etc.)
// inside a schema via a single SHOW OBJECTS IN SCHEMA command.
func (a *App) ListBasicObjects(database, schema string) ([]snowflake.SnowflakeObject, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListBasicObjects(a.fctx(FeatureObjectBrowser), database, schema)
}

// ClearObjectCache removes all cached object listings from the Snowflake client,
// forcing the next ListObjects/ListBasicObjects call to re-query Snowflake.
func (a *App) ClearObjectCache() {
	client := a.currentClient()
	if client == nil {
		return
	}
	client.ClearObjectCache()
}

// ClearObjectCacheForDatabase removes all cached object listings for every
// schema under the given database, forcing subsequent calls to re-query Snowflake.
func (a *App) ClearObjectCacheForDatabase(database string) {
	client := a.currentClient()
	if client == nil {
		return
	}
	client.ClearObjectCacheForDatabase(database)
}

// GetDatabaseRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given database. Returns 1 if the value cannot be determined.
func (a *App) GetDatabaseRetentionDays(dbName string) (int, error) {
	client := a.currentClient()
	if client == nil {
		return 0, apperrors.ErrNotConnected
	}
	return client.GetDatabaseRetentionDays(a.fctx(FeatureObjectBrowser), dbName)
}

// GetSchemaRetentionDays returns the DATA_RETENTION_TIME_IN_DAYS parameter
// for the given schema. Returns 1 if the value cannot be determined.
func (a *App) GetSchemaRetentionDays(database, schema string) (int, error) {
	client := a.currentClient()
	if client == nil {
		return 0, apperrors.ErrNotConnected
	}
	return client.GetSchemaRetentionDays(a.fctx(FeatureObjectBrowser), database, schema)
}

// GetTableRetentionDays returns the Time Travel data retention period in days
// for the given table. Returns 1 if the value cannot be determined.
func (a *App) GetTableRetentionDays(database, schema, name string) (int, error) {
	client := a.currentClient()
	if client == nil {
		return 0, apperrors.ErrNotConnected
	}
	return client.GetTableRetentionDays(a.fctx(FeatureObjectBrowser), database, schema, name)
}

// ListDroppedTables returns tables in the schema that are within the Time Travel
// retention window and can be recovered with UNDROP TABLE.
func (a *App) ListDroppedTables(database, schema string) ([]snowflake.DroppedTable, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListDroppedTables(a.fctx(FeatureObjectBrowser), database, schema)
}

// ListDroppedSchemas returns schemas in the database that are within the Time
// Travel retention window and can be recovered with UNDROP SCHEMA.
func (a *App) ListDroppedSchemas(database string) ([]snowflake.DroppedTable, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListDroppedSchemas(a.fctx(FeatureObjectBrowser), database)
}

// ListDroppedDatabases returns databases that are within the Time Travel
// retention window and can be recovered with UNDROP DATABASE.
func (a *App) ListDroppedDatabases() ([]snowflake.DroppedTable, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListDroppedDatabases(a.fctx(FeatureObjectBrowser))
}

// GetProcedureParams fetches the DDL for a stored procedure and returns its
// parameter list with real parameter names parsed from the DDL.
func (a *App) GetProcedureParams(database, schema, name, argTypes string) ([]snowflake.ProcParam, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetProcedureParams(a.fctx(FeatureObjectBrowser), database, schema, name, argTypes)
}

// GetTableColumns returns the ordered column names for a table or view.
func (a *App) GetTableColumns(database, schema, name string) ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetTableColumns(a.fctx(FeatureObjectBrowser), database, schema, name)
}

// GetTableForeignKeys returns the foreign keys where the given table is the
// referencing side. Used by the editor's JOIN ON autocomplete.
func (a *App) GetTableForeignKeys(database, schema, table string) ([]snowflake.TableForeignKey, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetTableForeignKeys(a.fctx(FeatureObjectBrowser), database, schema, table)
}

// GetTableColumnsWithTypes returns ordered column names and data types for a
// table or view. Used by the editor's JOIN ON autocomplete for type-compatible
// same-name column suggestions.
func (a *App) GetTableColumnsWithTypes(database, schema, name string) ([]snowflake.ColumnInfo, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetTableColumnsWithTypes(a.fctx(FeatureObjectBrowser), database, schema, name)
}

// GetColumnDetails returns the DEFAULT expression and masking policy attached to
// a single column, backing the column properties editor's Default and Masking
// Policy sections.
func (a *App) GetColumnDetails(database, schema, table, column string) (snowflake.ColumnDetails, error) {
	client := a.currentClient()
	if client == nil {
		return snowflake.ColumnDetails{}, apperrors.ErrNotConnected
	}
	return client.GetColumnDetails(a.fctx(FeatureObjectBrowser), database, schema, table, column)
}

// ListGitRepoEntries returns the immediate children (files and directories) at
// dirPath inside the git repository stage @database.schema.repoName/dirPath.
// Pass an empty dirPath to list the root.
func (a *App) ListGitRepoEntries(database, schema, repoName, dirPath string) ([]snowflake.GitRepoEntry, error) {
	client := a.currentClient()
	if client == nil {
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

	return client.ListGitRepoEntries(a.fctx(FeatureObjectBrowser), database, schema, repoName, dirPath)
}

// ListGitBranches returns all branches in the given git repository.
func (a *App) ListGitBranches(database, schema, repoName string) ([]snowflake.GitBranch, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListGitBranches(a.fctx(FeatureObjectBrowser), database, schema, repoName)
}

// ListGitTags returns all tags in the given git repository.
func (a *App) ListGitTags(database, schema, repoName string) ([]snowflake.GitTag, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListGitTags(a.fctx(FeatureObjectBrowser), database, schema, repoName)
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
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.GetGitFileContent(a.fctx(FeatureObjectBrowser), database, schema, repoName, filePath)
}

// ExecuteGitFile executes a SQL file from a git repository.
func (a *App) ExecuteGitFile(database, schema, repoName, filePath string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	return client.ExecuteGitFile(a.fctx(FeatureObjectBrowser), database, schema, repoName, filePath)
}

// GetSchemaForeignKeys returns all FK→PK column mappings in the given schema
// from INFORMATION_SCHEMA. Used by the editor to bulk-warm FK data for the
// JOIN ON autocomplete instead of issuing per-table SHOW IMPORTED KEYS calls.
func (a *App) GetSchemaForeignKeys(database, schema string) ([]snowflake.TableForeignKey, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetSchemaForeignKeys(a.fctx(FeatureObjectBrowser), database, schema)
}

// GetFunctionInfo fetches the DDL for a user-defined function and returns its
// parameter list together with a flag indicating whether it is a table function.
func (a *App) GetFunctionInfo(database, schema, name, argTypes string) (*snowflake.FunctionInfo, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.GetFunctionInfo(a.fctx(FeatureObjectBrowser), database, schema, name, argTypes)
}

// GetObjectDDL returns the definition of a single schema object using
// Snowflake's GET_DDL function. kind should be one of: TABLE, VIEW,
// DYNAMIC TABLE, EXTERNAL TABLE, MATERIALIZED VIEW, ALERT, TAG, FUNCTION,
// PROCEDURE, SEQUENCE, STAGE, STREAM, TASK, FILE FORMAT, PIPE.
// For procedures and functions, arguments must be the parameter type list
// (e.g. "NUMBER, VARCHAR") so Snowflake can resolve the correct overload.
// Pass an empty string for all other object kinds.
func (a *App) GetObjectDDL(database, schema, kind, name, arguments string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	return client.GetObjectDDL(a.fctx(FeatureObjectBrowser), database, schema, kind, name, arguments)
}

// GetObjectDependencies parses the DDL of a VIEW, PROCEDURE, or FUNCTION and
// returns a recursive tree of objects it depends on.  Tables are leaf nodes;
// views and SQL-language procedures/functions are expanded recursively.
// arguments should be the parameter type list for procedures/functions
// (e.g. "NUMBER, VARCHAR") or an empty string for views.
func (a *App) GetObjectDependencies(database, schema, kind, name, arguments string) (snowflake.DependencyNode, error) {
	client := a.currentClient()
	if client == nil {
		return snowflake.DependencyNode{}, apperrors.ErrNotConnected
	}
	return client.GetObjectDependencies(a.fctx(FeatureObjectBrowser), database, schema, kind, name, arguments)
}

// GetObjectProperties returns structured metadata for any Snowflake object by
// running the appropriate SHOW or DESCRIBE command and returning the result as
// key/value pairs. kind is one of: TABLE, VIEW, DYNAMIC TABLE, EXTERNAL TABLE,
// MATERIALIZED VIEW, ALERT, TAG, FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM, TASK,
// FILE FORMAT, PIPE, WAREHOUSE, ROLE, USER.
func (a *App) GetObjectProperties(database, schema, kind, name string) ([]snowflake.PropertyPair, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return objects.GetObjectProperties(a.fctx(FeatureObjectBrowser), client, database, schema, kind, name)
}

// GetRoutineProperties returns SHOW metadata for one specific overload of a
// FUNCTION or PROCEDURE, selected by its argument-type signature (args, e.g.
// "NUMBER, VARCHAR"). Overloaded routines return one SHOW row per signature;
// this picks the matching one so the properties panel reflects the overload the
// user acted on rather than always the first. Pass the same signature threaded
// into AlterFunction / AlterProcedure.
func (a *App) GetRoutineProperties(database, schema, kind, name, args string) ([]snowflake.PropertyPair, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return objects.GetRoutineProperties(a.fctx(FeatureObjectBrowser), client, database, schema, kind, name, args)
}
