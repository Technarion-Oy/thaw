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
	"thaw/internal/alert"
	"thaw/internal/column"
	"thaw/internal/datametricfunction"
	"thaw/internal/dbtproject"
	"thaw/internal/dynamictable"
	"thaw/internal/eventtable"
	"thaw/internal/externalfunction"
	"thaw/internal/externaltable"
	"thaw/internal/fileformat"
	"thaw/internal/hybridtable"
	"thaw/internal/icebergtable"
	"thaw/internal/imagerepository"
	"thaw/internal/maskingpolicy"
	"thaw/internal/materializedview"
	"thaw/internal/networkrule"
	"thaw/internal/passwordpolicy"
	"thaw/internal/pipe"
	"thaw/internal/procedure"
	"thaw/internal/rowaccesspolicy"
	"thaw/internal/secret"
	"thaw/internal/service"
	"thaw/internal/snowflake"
	"thaw/internal/snowgitrepo"
	"thaw/internal/stage"
	"thaw/internal/streamlit"
	"thaw/internal/tag"
)

// BuildCreateSecretSql returns the SQL for creating a secret.
func (a *App) BuildCreateSecretSql(database, schema string, cfg secret.SecretConfig) (string, error) {
	return secret.BuildCreateSecretSql(database, schema, cfg)
}

// BuildModifySecretSql returns one or more SQL statements for modifying a secret.
func (a *App) BuildModifySecretSql(database, schema, name string, cfg secret.SecretConfig, originalComment string) ([]string, error) {
	return secret.BuildModifySecretSql(database, schema, name, cfg, originalComment)
}

// BuildCreateGitRepositorySql returns the SQL for creating a GIT REPOSITORY object.
func (a *App) BuildCreateGitRepositorySql(database, schema string, cfg snowgitrepo.GitRepositoryConfig) (string, error) {
	return snowgitrepo.BuildCreateGitRepositorySql(database, schema, cfg)
}

// BuildModifyGitRepositorySql returns one or more ALTER GIT REPOSITORY statements.
func (a *App) BuildModifyGitRepositorySql(database, schema, name string, cfg snowgitrepo.GitRepositoryConfig, originalComment, originalIntegration, originalCredentials string) ([]string, error) {
	return snowgitrepo.BuildModifyGitRepositorySql(database, schema, name, cfg, originalComment, originalIntegration, originalCredentials)
}

// BuildCreateDbtProjectSql returns the SQL for creating a DBT PROJECT object.
func (a *App) BuildCreateDbtProjectSql(database, schema string, cfg dbtproject.CreateConfig) (string, error) {
	return dbtproject.BuildCreateDbtProjectSql(database, schema, cfg)
}

// BuildAlterDbtProjectSetSql returns one or more ALTER DBT PROJECT SET/UNSET statements.
func (a *App) BuildAlterDbtProjectSetSql(database, schema, name string, cfg dbtproject.AlterSetConfig, origComment, origDbtVersion, origDefaultTarget string, origIntegrations []string) ([]string, error) {
	return dbtproject.BuildAlterDbtProjectSetSql(database, schema, name, cfg, origComment, origDbtVersion, origDefaultTarget, origIntegrations)
}

// BuildExecuteDbtProjectSql returns the SQL for executing a DBT PROJECT.
func (a *App) BuildExecuteDbtProjectSql(database, schema, name string, cfg dbtproject.ExecuteConfig) (string, error) {
	return dbtproject.BuildExecuteDbtProjectSql(database, schema, name, cfg)
}

// BuildAddDbtProjectVersionSql returns the SQL for adding a version to a DBT PROJECT.
func (a *App) BuildAddDbtProjectVersionSql(database, schema, name, versionAlias, sourceLocation string) (string, error) {
	return dbtproject.BuildAddVersionSql(database, schema, name, versionAlias, sourceLocation)
}

// BuildAddColumnSql returns the SQL for an ALTER TABLE ... ADD COLUMN statement.
func (a *App) BuildAddColumnSql(database, schema, table string, cfg column.AddColumnConfig) (string, error) {
	return column.BuildAddColumnSql(database, schema, table, cfg)
}

// BuildDropColumnSql returns the SQL for an ALTER TABLE ... DROP COLUMN statement.
func (a *App) BuildDropColumnSql(database, schema, table, col string) string {
	return column.BuildDropColumnSql(database, schema, table, col)
}

// BuildRenameColumnSql returns the SQL for an ALTER TABLE ... RENAME COLUMN statement.
func (a *App) BuildRenameColumnSql(database, schema, table, oldName, newName string, caseSensitive bool) string {
	return column.BuildRenameColumnSql(database, schema, table, oldName, newName, caseSensitive)
}

// BuildSetColumnNotNullSql returns the SQL for setting a column NOT NULL.
func (a *App) BuildSetColumnNotNullSql(database, schema, table, col string) string {
	return column.BuildSetNotNullSql(database, schema, table, col)
}

// BuildDropColumnNotNullSql returns the SQL for dropping a column's NOT NULL constraint.
func (a *App) BuildDropColumnNotNullSql(database, schema, table, col string) string {
	return column.BuildDropNotNullSql(database, schema, table, col)
}

// BuildSetColumnCommentSql returns the SQL for setting (or UNSETting) a column comment.
func (a *App) BuildSetColumnCommentSql(database, schema, table, col, comment string) string {
	return column.BuildSetColumnCommentSql(database, schema, table, col, comment)
}

// BuildChangeColumnTypeSql returns the SQL for an ALTER COLUMN ... SET DATA TYPE statement.
func (a *App) BuildChangeColumnTypeSql(database, schema, table, col, dataType string) string {
	return column.BuildChangeDataTypeSql(database, schema, table, col, dataType)
}

// BuildCreatePipeSql returns the SQL for creating a Snowflake PIPE.
func (a *App) BuildCreatePipeSql(database, schema string, cfg pipe.PipeConfig) (string, error) {
	return pipe.BuildCreatePipeSql(database, schema, cfg)
}

// BuildRefreshPipeSql returns the SQL for an ALTER PIPE ... REFRESH statement.
func (a *App) BuildRefreshPipeSql(database, schema, name string, cfg pipe.RefreshPipeConfig) (string, error) {
	return pipe.BuildRefreshPipeSql(database, schema, name, cfg)
}

// BuildCreateDynamicTableSql returns the SQL for creating a Snowflake DYNAMIC TABLE.
func (a *App) BuildCreateDynamicTableSql(database, schema string, cfg dynamictable.DynamicTableConfig) (string, error) {
	return dynamictable.BuildCreateDynamicTableSql(database, schema, cfg)
}

// BuildCreateExternalTableSql returns the SQL for creating a Snowflake EXTERNAL TABLE.
func (a *App) BuildCreateExternalTableSql(database, schema string, cfg externaltable.ExternalTableConfig) (string, error) {
	return externaltable.BuildCreateExternalTableSql(database, schema, cfg)
}

// BuildCreateIcebergTableSql returns the SQL for creating a Snowflake ICEBERG TABLE.
func (a *App) BuildCreateIcebergTableSql(database, schema string, cfg icebergtable.IcebergTableConfig) (string, error) {
	return icebergtable.BuildCreateIcebergTableSql(database, schema, cfg)
}

// BuildCreateEventTableSql returns the SQL for creating a Snowflake EVENT TABLE.
func (a *App) BuildCreateEventTableSql(database, schema string, cfg eventtable.EventTableConfig) (string, error) {
	return eventtable.BuildCreateEventTableSql(database, schema, cfg)
}

// BuildCreateExternalFunctionSql returns the SQL for creating a Snowflake
// EXTERNAL FUNCTION.
func (a *App) BuildCreateExternalFunctionSql(database, schema string, cfg externalfunction.ExternalFunctionConfig) (string, error) {
	return externalfunction.BuildCreateExternalFunctionSql(database, schema, cfg)
}

// BuildCreateDataMetricFunctionSql returns the SQL for creating a Snowflake DATA
// METRIC FUNCTION.
func (a *App) BuildCreateDataMetricFunctionSql(database, schema string, cfg datametricfunction.DataMetricFunctionConfig) (string, error) {
	return datametricfunction.BuildCreateDataMetricFunctionSql(database, schema, cfg)
}

// BuildCreateHybridTableSql returns the SQL for creating a Snowflake HYBRID TABLE.
func (a *App) BuildCreateHybridTableSql(database, schema string, cfg hybridtable.HybridTableConfig) (string, error) {
	return hybridtable.BuildCreateHybridTableSql(database, schema, cfg)
}

// BuildCreateMaterializedViewSql returns the SQL for creating a Snowflake MATERIALIZED VIEW.
func (a *App) BuildCreateMaterializedViewSql(database, schema string, cfg materializedview.MaterializedViewConfig) (string, error) {
	return materializedview.BuildCreateMaterializedViewSql(database, schema, cfg)
}

// BuildCreateAlertSql returns the SQL for creating a Snowflake ALERT.
func (a *App) BuildCreateAlertSql(database, schema string, cfg alert.AlertConfig) (string, error) {
	return alert.BuildCreateAlertSql(database, schema, cfg)
}

// BuildCreateTagSql returns the SQL for creating a Snowflake TAG.
func (a *App) BuildCreateTagSql(database, schema string, cfg tag.TagConfig) (string, error) {
	return tag.BuildCreateTagSql(database, schema, cfg)
}

// BuildCreateMaskingPolicySql returns the SQL for creating a Snowflake MASKING POLICY.
func (a *App) BuildCreateMaskingPolicySql(database, schema string, cfg maskingpolicy.MaskingPolicyConfig) (string, error) {
	return maskingpolicy.BuildCreateMaskingPolicySql(database, schema, cfg)
}

// BuildCreateRowAccessPolicySql returns the SQL for creating a Snowflake ROW ACCESS POLICY.
func (a *App) BuildCreateRowAccessPolicySql(database, schema string, cfg rowaccesspolicy.RowAccessPolicyConfig) (string, error) {
	return rowaccesspolicy.BuildCreateRowAccessPolicySql(database, schema, cfg)
}

// BuildCreatePasswordPolicySql returns the SQL for creating a Snowflake PASSWORD POLICY.
func (a *App) BuildCreatePasswordPolicySql(database, schema string, cfg passwordpolicy.PasswordPolicyConfig) (string, error) {
	return passwordpolicy.BuildCreatePasswordPolicySql(database, schema, cfg)
}

// BuildCreateNetworkRuleSql returns the SQL for creating a Snowflake NETWORK RULE.
func (a *App) BuildCreateNetworkRuleSql(database, schema string, cfg networkrule.NetworkRuleConfig) (string, error) {
	return networkrule.BuildCreateNetworkRuleSql(database, schema, cfg)
}

// BuildCreateImageRepositorySql returns the SQL for creating a Snowflake IMAGE REPOSITORY.
func (a *App) BuildCreateImageRepositorySql(database, schema string, cfg imagerepository.ImageRepositoryConfig) (string, error) {
	return imagerepository.BuildCreateImageRepositorySql(database, schema, cfg)
}

// BuildCreateServiceSql returns the SQL for creating a Snowflake SERVICE (SPCS).
func (a *App) BuildCreateServiceSql(database, schema string, cfg service.ServiceConfig) (string, error) {
	return service.BuildCreateServiceSql(database, schema, cfg)
}

// BuildCreateStreamlitSql returns the SQL for creating a Snowflake STREAMLIT app.
func (a *App) BuildCreateStreamlitSql(database, schema string, cfg streamlit.StreamlitConfig) (string, error) {
	return streamlit.BuildCreateStreamlitSql(database, schema, cfg)
}

// BuildCreateStageSql returns the CREATE STAGE SQL statement.
func (a *App) BuildCreateStageSql(cfg stage.StageConfig) string {
	return stage.BuildCreateStageSql(cfg)
}

// BuildAlterStageSql returns the ALTER STAGE SQL statement.
func (a *App) BuildAlterStageSql(cfg stage.AlterStageConfig) string {
	return stage.BuildAlterStageSql(cfg)
}

// BuildCreateFileFormatSql returns the CREATE FILE FORMAT SQL statement for the
// given configuration. Only parameters that differ from Snowflake's defaults are
// included, keeping the output concise.
func (a *App) BuildCreateFileFormatSql(database, schema string, cfg fileformat.FileFormatConfig) string {
	return fileformat.BuildCreateFileFormatSql(database, schema, cfg)
}

// GetAllDataTypes returns the complete list of supported Snowflake data types
// with their canonical names and autocompletion hints.  The list is static and
// derived from the same registry that ValidateDataType uses, so the editor
// completion list and the validator always agree.
func (a *App) GetAllDataTypes() []snowflake.DataTypeInfo {
	return snowflake.AllDataTypes()
}

// BuildCallStatement constructs a CALL SQL statement for a stored procedure.
func (a *App) BuildCallStatement(db, schema, name string, args []procedure.Argument) string {
	return procedure.BuildCallStatement(db, schema, name, args)
}

// BuildFunctionSelectStatement constructs a SELECT SQL statement for a user-defined function.
func (a *App) BuildFunctionSelectStatement(db, schema, name string, args []procedure.Argument, isTableFunction bool) string {
	return procedure.BuildFunctionSelectStatement(db, schema, name, args, isTableFunction)
}

// IsBoolean reports whether the given Snowflake data type is a boolean.
func (a *App) IsBoolean(dataType string) bool {
	return snowflake.IsBoolean(dataType)
}

// IsNumeric reports whether the given Snowflake data type is a numeric type.
func (a *App) IsNumeric(dataType string) bool {
	return snowflake.IsNumeric(dataType)
}

// NeedsQuotes reports whether a value of the given data type should be quoted in SQL.
func (a *App) NeedsQuotes(dataType string) bool {
	return snowflake.NeedsQuotes(dataType)
}

// GetCollations returns the curated list of Snowflake collation specifications
// for populating the column collation dropdown.
func (a *App) GetCollations() []snowflake.CollationOption {
	return snowflake.Collations()
}

// GetCollationLocales returns the supported collation locales (building blocks
// for assembling a custom collation specification).
func (a *App) GetCollationLocales() []snowflake.CollationLocale {
	return snowflake.CollationLocales()
}

// GetCollationSpecifiers returns the supported collation specifiers (case,
// accent, punctuation, etc.) for assembling a custom collation specification.
func (a *App) GetCollationSpecifiers() []snowflake.CollationSpecifier {
	return snowflake.CollationSpecifiers()
}
