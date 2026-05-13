// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"

	sf "thaw/internal/snowflake"
)

// ── Precompiled regexes for ValidateSnowflakePatterns ─────────────────────────

const (
	// ReIdentifier matches a Snowflake identifier part: either a double-quoted
	// string with escaped quotes (""""), or a bare word containing [a-zA-Z0-9_$].
	ReIdentifier = `(?:"(?:""|[^"])*"|[\w$]+)`

	_ident          = `(?:[a-zA-Z_][a-zA-Z0-9_$]*|"[^"]+")`
	_identPath      = _ident + `(?:\.` + _ident + `){0,2}`
	_balancedParens = `\([^()]*(?:(?:\([^()]*\))[^()]*)*\)`

	// _grantObjType matches the object-type token(s) in a GRANT/REVOKE ON clause.
	// Two-word types are listed explicitly before the single-word fallback so that
	// a greedy `(\w+(?:\s+\w+)?)` cannot swallow the object name (e.g. matching
	// "TABLE my_table" instead of just "TABLE").
	_grantObjType = `EXTERNAL\s+TABLE|MATERIALIZED\s+VIEW|HYBRID\s+TABLE|ICEBERG\s+TABLE|DYNAMIC\s+TABLE|FILE\s+FORMAT|\w+`
)

var (
	// ── Snowflake false-positive guard ───────────────────────────────────────
	// Statements matching this regex contain Snowflake syntax that the
	// node-sql-parser doesn't support; we skip them.  We keep this guard here so
	// the preamble validators don't accidentally emit for them.
	reSnowflakeFP = regexp.MustCompile(
		`(?i)\bTABLESAMPLE\b|\bSAMPLE\s*\(|\bWITHIN\s+GROUP\b|\bCONNECT\s+BY\b` +
			`|\bAT\s*\(|\bBEFORE\s*\(|\bIN\s+TABLE\b` +
			`|CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(?:STAGE` +
			`|REPLICATION|FAILOVER|APPLICATION)\b` +
			`|ALTER\s+(?:TABLE|VIEW|STREAM|DATABASE|STAGE|PIPE|PROCEDURE|FUNCTION` +
			`|ALERT|EXTERNAL|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK` +
			`|REPLICATION|FAILOVER)\b` +
			`|DROP\s+(?:TABLE|VIEW|STREAM|STAGE|PIPE|PROCEDURE|FUNCTION)\b` +
			`|UNDROP\s+(?:DATABASE|SCHEMA|TABLE)\b` +
			`|INSERT\s+OVERWRITE\b` +
			`|TRUNCATE\s+\S+\s+IF\b` +
			`|\bLATERAL\s+FLATTEN\b` +
			`|\bINFER_SCHEMA\b`,
	)

	// ── Custom check patterns ─────────────────────────────────────────────────
	reLateralFlatten      = regexp.MustCompile(`(?i)\bLATERALFLATTEN\b`)
	reFlattenFromJoin     = regexp.MustCompile(`(?i)(?:FROM|JOIN|,)\s+FLATTEN\s*\(`)
	reLateralOK           = regexp.MustCompile(`(?i)\bLATERAL\s+FLATTEN\s*\(`)
	reTableFlatten        = regexp.MustCompile(`(?i)\bTABLE\s*\(\s*FLATTEN\s*\(`)
	reQualifyAfterOrder   = regexp.MustCompile(`(?is)\bORDER\s+BY[\s\S]+?\bQUALIFY\b`)
	reVariantDotPath      = regexp.MustCompile(`(?i)\b([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\b`)
	reOrReplace           = regexp.MustCompile(`(?i)\bOR\s+REPLACE\b`)
	reIfNotExists         = regexp.MustCompile(`(?i)\bIF\s+NOT\s+EXISTS\b`)
	reStripStringLiterals = regexp.MustCompile(`'(?:''|[^'])*'`)
	// rePatternClusterBy — distinct from the CLUSTER BY pattern in `tableProps` for CREATE TABLE.
	rePatternClusterBy = regexp.MustCompile(`(?i)\bCLUSTER\s+BY\b`)
	reDataRetention    = regexp.MustCompile(`(?i)\bDATA_RETENTION_TIME_IN_DAYS\b`)
	reConstraintCol    = regexp.MustCompile(`(?i)^(?:CONSTRAINT|PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY)\b`)
	reVirtualColAS     = regexp.MustCompile(`(?i)\bAS\s*\([\s\S]*\)\s*$`)
	rePartitionBy      = regexp.MustCompile(`(?i)^PARTITION\s+BY\b`)

	reWithLocation = regexp.MustCompile(`(?i)\bWITH\s+LOCATION\s*=`)
	reFileFormat   = regexp.MustCompile(`(?i)\bFILE_FORMAT\s*=`)

	// ── CREATE VIEW ───────────────────────────────────────────────────────────
	reIsCreateView = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?` +
			`(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE)\s+)?` +
			`(?:RECURSIVE\s+)?(?:INTERACTIVE\s+)?(?:MATERIALIZED\s+)?VIEW\b`)

	reValidCreateViewPreamble = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?` +
			`(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE)\s+)?` +
			`(?:RECURSIVE\s+)?(?:INTERACTIVE\s+)?(?:MATERIALIZED\s+)?` +
			`VIEW\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath +
			`(?:\s*` + _balancedParens + `)?` +
			`(?:\s+(?:` +
			`COPY\s+GRANTS` +
			`|COMMENT\s*=\s*'(?:[^']|'')*'` +
			`|CHANGE_TRACKING\s*=\s*(?:TRUE|FALSE)` +
			`|(?:WITH\s+)?ROW\s+ACCESS\s+POLICY\s+` + _identPath + `\s+ON\s*` + _balancedParens +
			`|(?:WITH\s+)?AGGREGATION\s+POLICY\s+` + _identPath + `(?:\s+ENTITY\s+KEY\s*` + _balancedParens + `)?` +
			`|(?:WITH\s+)?JOIN\s+POLICY\s+` + _identPath + `(?:\s+ALLOWED\s+JOIN\s+KEYS\s*` + _balancedParens + `)?` +
			`|CLUSTER\s+BY\s*` + _balancedParens +
			`|(?:WITH\s+)?TAG\s*` + _balancedParens +
			`|WITH\s+CONTACT\s*` + _balancedParens +
			`))*\s+AS\s+`)

	// ── CREATE TABLE ─────────────────────────────────────────────────────────
	reIsCreateTable       = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:(?:OR\s+(?:REPLACE|ALTER)|LOCAL|GLOBAL|TEMP|TEMPORARY|VOLATILE|TRANSIENT)\s+)*TABLE\b`)
	reCreateTablePreamble = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+(?:REPLACE|ALTER)\s+)?(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE|TRANSIENT)\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath)
	reIsCreateHybridTable = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:(?:OR\s+REPLACE|TRANSIENT)\s+)*HYBRID\s+TABLE\b`)
	reHybridTablePreamble = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:(?:OR\s+REPLACE|TRANSIENT)\s+)*HYBRID\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath)
	reIndexKeyword        = regexp.MustCompile(`(?i)\bINDEX\b`)
	reNotNull             = regexp.MustCompile(`(?i)\bNOT\s+NULL\b`)
	rePrimaryKey          = regexp.MustCompile(`(?i)\bPRIMARY\s+KEY\b`)
	rePrimaryKeyCols      = regexp.MustCompile(`(?i)PRIMARY\s+KEY\s*\([^)]+\)`)
	reChangeTracking      = regexp.MustCompile(`(?i)\bCHANGE_TRACKING\b`)
	reCopyGrants          = regexp.MustCompile(`(?i)\bCOPY\s+GRANTS\b`)
	reTransient           = regexp.MustCompile(`(?i)\bTRANSIENT\b`)
	reAutoIncrement       = regexp.MustCompile(`(?i)\b(?:AUTOINCREMENT|IDENTITY)\b`)

	// ── COPY INTO ────────────────────────────────────────────────────────────
	reIsCopyInto          = regexp.MustCompile(`(?i)^\s*COPY\s+INTO\b`)
	reCopyInto            = regexp.MustCompile(`(?i)^\s*COPY\s+INTO\s+(` + _identPath + `|@\S+|'[^']+')(?:\s*\([^)]*\))?(?:\s+|$)`)
	reCreateTableCTAS     = regexp.MustCompile(`(?i)^AS\s+(?:SELECT|WITH)\b`)
	reCreateTableClone    = regexp.MustCompile(`(?i)^(?:CLONE|LIKE)\b`)
	reCreateTableTemplate = regexp.MustCompile(`(?i)^USING\s+TEMPLATE\s*\(`)
	reCreateTableBackup   = regexp.MustCompile(`(?i)^FROM\s+BACKUP\s+SET\s+(?:` + _identPath + `\s+IDENTIFIER\s+)?'[^']+'\s*$`)

	tableProps = strings.Join([]string{
		`CLUSTER\s+BY\s*` + _balancedParens,
		`ENABLE_SCHEMA_EVOLUTION\s*=\s*(?:TRUE|FALSE)`,
		`DATA_RETENTION_TIME_IN_DAYS\s*=\s*\d+`,
		`MAX_DATA_EXTENSION_TIME_IN_DAYS\s*=\s*\d+`,
		`CHANGE_TRACKING\s*=\s*(?:TRUE|FALSE)`,
		`DEFAULT_DDL_COLLATION\s*=\s*'(?:[^']|'')*'`,
		`COPY\s+GRANTS`,
		`ERROR_LOGGING\s*=\s*(?:TRUE|FALSE)`,
		`COPY\s+TAGS`,
		`COMMENT\s*=\s*'(?:[^']|'')*'`,
		`ROW_TIMESTAMP\s*=\s*(?:TRUE|FALSE)`,
		`(?:WITH\s+)?ROW(?:\s+|_)ACCESS(?:\s+|_)POLICY\s+` + _identPath + `\s+ON\s*` + _balancedParens,
		`(?:WITH\s+)?AGGREGATION\s+POLICY\s+` + _identPath + `(?:\s+ENTITY\s+KEY\s*` + _balancedParens + `)?`,
		`(?:WITH\s+)?JOIN\s+POLICY\s+` + _identPath + `(?:\s+ALLOWED\s+JOIN\s+KEYS\s*` + _balancedParens + `)?`,
		`(?:WITH\s+)?STORAGE\s+LIFECYCLE\s+POLICY\s+` + _identPath + `\s+ON\s*` + _balancedParens,
		`(?:WITH\s+)?TAG\s*` + _balancedParens,
		`WITH\s+CONTACT\s*` + _balancedParens,
	}, "|")

	// ── CREATE DATABASE / SCHEMA ──────────────────────────────────────────────
	reIsCreateDbSchema = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(DATABASE|SCHEMA)\b`)

	dbSchemaProps = strings.Join([]string{
		`CLONE\s+` + _identPath + `(?:\s+(?:AT|BEFORE)\s*\(\s*(?:TIMESTAMP|OFFSET|STATEMENT)\s*=>\s*[^)]+\))?(?:\s+IGNORE\s+TABLES\s+WITH\s+INSUFFICIENT\s+DATA\s+RETENTION)?(?:\s+IGNORE\s+HYBRID\s+TABLES)?`,
		`WITH\s+MANAGED\s+ACCESS`,
		`(?:DATA_RETENTION_TIME_IN_DAYS|MAX_DATA_EXTENSION_TIME_IN_DAYS|ICEBERG_VERSION_DEFAULT)\s*=\s*\d+`,
		`(?:ENABLE_ICEBERG_MERGE_ON_READ|REPLACE_INVALID_CHARACTERS|ENABLE_DATA_COMPACTION)\s*=\s*(?:TRUE|FALSE)`,
		`(?:EXTERNAL_VOLUME|CATALOG)\s*=\s*` + _ident,
		`DEFAULT_DDL_COLLATION\s*=\s*'(?:[^']|'')*'`,
		`STORAGE_SERIALIZATION_POLICY\s*=\s*(?:COMPATIBLE|OPTIMIZED)`,
		`CLASSIFICATION_PROFILE\s*=\s*'(?:[^']|'')*'`,
		`COMMENT\s*=\s*'(?:[^']|'')*'`,
		`CATALOG_SYNC\s*=\s*'(?:[^']|'')*'`,
		`CATALOG_SYNC_NAMESPACE_MODE\s*=\s*(?:NEST|FLATTEN)`,
		`CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER\s*=\s*'(?:[^']|'')*'`,
		`(?:WITH\s+)?TAG\s*\([^)]+\)`,
		`(?:WITH\s+)?CONTACT\s*\([^)]+\)`,
		`OBJECT_VISIBILITY\s*=\s*(?:PRIVILEGED|` + _ident + `)`,
		// CREATE DATABASE <name> FROM SHARE <provider_account>.<share_name>
		`FROM\s+SHARE\s+` + _ident + `\.` + _ident,
	}, "|")

	reValidCreateDbSchema = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(?:DATABASE|SCHEMA)\s+(?:IF\s+NOT\s+EXISTS\s+)?` +
			_identPath + `(?:\s+(?:` + dbSchemaProps + `))*\s*$`)

	// ── DROP DATABASE / SCHEMA ────────────────────────────────────────────────
	reIsDropDbSchema    = regexp.MustCompile(`(?i)^\s*DROP\s+(DATABASE|SCHEMA)\b`)
	reValidDropDbSchema = regexp.MustCompile(`(?i)^\s*DROP\s+(?:DATABASE|SCHEMA)\s+(?:IF\s+EXISTS\s+)?` + _identPath + `(?:\s+(?:CASCADE|RESTRICT))?\s*$`)

	// ── CREATE SEQUENCE ───────────────────────────────────────────────────────
	reIsCreateSeq    = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?SEQUENCE\b`)
	reValidCreateSeq = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?SEQUENCE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath +
			`(?:\s+WITH)?(?:\s+(?:` +
			`START(?:\s+WITH|\s*=)?\s+-?\d+` +
			`|INCREMENT(?:\s+BY|\s*=)?\s+-?\d+` +
			`|ORDER|NOORDER` +
			`|COMMENT\s*=\s*'(?:[^']|'')*'` +
			`))*\s*$`)

	// ── ALTER SEQUENCE ────────────────────────────────────────────────────────
	reIsAlterSeq    = regexp.MustCompile(`(?i)^\s*ALTER\s+SEQUENCE\b`)
	reValidAlterSeq = regexp.MustCompile(
		`(?i)^\s*ALTER\s+SEQUENCE\s+(?:IF\s+EXISTS\s+)?` + _identPath + `\s+` +
			`(?:RENAME\s+TO\s+` + _identPath +
			`|(?:SET\s+)?INCREMENT(?:\s+BY|\s*=)?\s+-?\d+` +
			`|SET(?:\s+(?:ORDER|NOORDER|COMMENT\s*=\s*'(?:[^']|'')*'))+` +
			`|UNSET\s+COMMENT` +
			`)\s*$`)

	// ── DROP SEQUENCE ─────────────────────────────────────────────────────────
	reIsDropSeq    = regexp.MustCompile(`(?i)^\s*DROP\s+SEQUENCE\b`)
	reValidDropSeq = regexp.MustCompile(`(?i)^\s*DROP\s+SEQUENCE\s+(?:IF\s+EXISTS\s+)?` + _identPath + `(?:\s+(?:CASCADE|RESTRICT))?\s*$`)

	// ── CREATE DYNAMIC TABLE ─────────────────────────────────────────────────
	reIsCreateDynTable = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?DYNAMIC\s+TABLE\b`)
	reDynHasTargetLag  = regexp.MustCompile(`(?i)\bTARGET_LAG\s*=`)
	reDynHasWarehouse  = regexp.MustCompile(`(?i)\bWAREHOUSE\s*=`)
	reDynHasAs         = regexp.MustCompile(`(?i)\bAS\s+(?:SELECT|WITH)\b`)

	// ── CREATE INTEGRATION ────────────────────────────────────────────────────
	reIsCreateIntegration = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:STORAGE|API|NOTIFICATION|SECURITY|EXTERNAL\s+ACCESS)\s+INTEGRATION\b`)
	reIntegrationName     = regexp.MustCompile(`(?i)INTEGRATION\s+(` + _identPath + `)`)
	reIntegrationType     = regexp.MustCompile(`(?i)\bTYPE\s*=\s*([a-zA-Z_0-9]+)`)
	reIntegrationProvider = regexp.MustCompile(`(?i)\b(?:STORAGE|API)_PROVIDER\s*=\s*('[^']+'|[a-zA-Z_0-9]+)`)

	// ── CREATE WAREHOUSE ──────────────────────────────────────────────────────
	reIsCreateWarehouse = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?WAREHOUSE\b`)
	whProps             = strings.Join([]string{
		`WAREHOUSE_SIZE`, `WAREHOUSE_TYPE`, `MAX_CLUSTER_COUNT`, `MIN_CLUSTER_COUNT`, `SCALING_POLICY`,
		`AUTO_SUSPEND`, `AUTO_RESUME`, `RESOURCE_MONITOR`, `COMMENT`,
		`ENABLE_QUERY_ACCELERATION`, `QUERY_ACCELERATION_MAX_SCALE_FACTOR`,
		`MAX_CONCURRENCY_LEVEL`, `STATEMENT_QUEUED_TIMEOUT_IN_SECONDS`, `STATEMENT_TIMEOUT_IN_SECONDS`,
	}, "|")

	// ── CREATE EXTERNAL TABLE ────────────────────────────────────────────────
	reIsCreateExternalTable = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?EXTERNAL\s+TABLE\b`)
	reExternalTablePreamble = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?EXTERNAL\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath)

	extTableProps = strings.Join([]string{
		`WITH\s+LOCATION\s*=\s*@\S+`,
		`REFRESH_ON_CREATE\s*=\s*(?:TRUE|FALSE)`,
		`AUTO_REFRESH\s*=\s*(?:TRUE|FALSE)`,
		`PATTERN\s*=\s*'(?:[^']|'')*'`,
		`FILE_FORMAT\s*=\s*\((?:FORMAT_NAME\s*=\s*` + _identPath + `|TYPE\s*=\s*[a-zA-Z]+)(?:\s+[^)]+)*\)`,
		`AWS_SNS_TOPIC\s*=\s*'(?:[^']|'')*'`,
		`INTEGRATION\s*=\s*'(?:[^']|'')*'`,
		`PARTITION_TYPE\s*=\s*USER_SPECIFIED`,
		`TABLE_FORMAT\s*=\s*DELTA`,
		`COPY\s+GRANTS`,
		`COMMENT\s*=\s*'(?:[^']|'')*'`,
		`(?:WITH\s+)?TAG\s*` + _balancedParens,
	}, "|")
	extTablePropsRe = regexp.MustCompile(`(?i)^\s*(?:(?:` + extTableProps + `)(?:\s+|$))*$`)

	// ── CREATE RESOURCE MONITOR ───────────────────────────────────────────────
	reIsCreateResourceMonitor = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?RESOURCE\s+MONITOR\b`)
	rmProps                   = strings.Join([]string{
		`CREDIT_QUOTA`, `FREQUENCY`, `START_TIMESTAMP`, `END_TIMESTAMP`, `NOTIFY_USERS`,
	}, "|")

	// ── CREATE STREAM ─────────────────────────────────────────────────────────
	reIsCreateStream = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?STREAM\b`)
	streamProps      = strings.Join([]string{
		`APPEND_ONLY\s*=\s*(?:TRUE|FALSE)`,
		`INSERT_ONLY\s*=\s*(?:TRUE|FALSE)`,
		`SHOW_INITIAL_ROWS\s*=\s*(?:TRUE|FALSE)`,
		`CHANGE_TRACKING\s*=\s*(?:TRUE|FALSE)`,
		`COMMENT\s*=\s*'(?:[^']|'')*'`,
	}, "|")

	reValidCreateStream = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?STREAM\s+(?:IF\s+NOT\s+EXISTS\s+)?` +
			_identPath + `(?:\s+COPY\s+GRANTS)?\s+ON\s+(?:TABLE|VIEW|STAGE|EXTERNAL\s+TABLE)\s+` + _identPath +
			`(?:\s+(?:AT|BEFORE)\s*` + _balancedParens + `)?` +
			`(?:\s+(?:` + streamProps + `))*\s*$`)

	// ── CREATE TASK ───────────────────────────────────────────────────────────
	reIsCreateTask = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?TASK\b`)
	taskProps      = strings.Join([]string{
		`WAREHOUSE`, `USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE`, `SCHEDULE`, `CONFIG`,
		`ALLOW_OVERLAPPING_EXECUTION`, `USER_TASK_TIMEOUT_MS`, `SUSPEND_TASK_AFTER_NUM_FAILURES`,
		`ERROR_INTEGRATION`, `COMMENT`, `AFTER`, `WHEN`, `FINALIZE`,
	}, "|")

	reCreateTaskName = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?TASK\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)
	reTaskAS         = regexp.MustCompile(`(?i)\bAS\b`)
	reTaskSchedule   = regexp.MustCompile(`(?i)\bSCHEDULE\s*=`)
	reTaskAfter      = regexp.MustCompile(`(?i)\bAFTER\b`)
	reTaskAfterNames = regexp.MustCompile(`(?i)\bAFTER\s+(` + _identPath + `(?:\s*,\s*` + _identPath + `)*)`)
	reTaskFinalizeBare = regexp.MustCompile(`(?i)\bFINALIZE\b`)
	reTaskFinalizeN    = regexp.MustCompile(`(?i)\bFINALIZE\s*=\s*(` + _identPath + `)`)
	reTaskWhen       = regexp.MustCompile(`(?i)\bWHEN\b`)
	reTaskWhenExpr   = regexp.MustCompile(`(?i)\bWHEN\s+\S`)

	// ── ALTER TASK ────────────────────────────────────────────────────────────
	reIsAlterTask     = regexp.MustCompile(`(?i)^\s*ALTER\s+TASK\b`)
	reAlterTaskName   = regexp.MustCompile(`(?i)^\s*ALTER\s+TASK\s+(?:IF\s+EXISTS\s+)?(` + _identPath + `)`)
	reAlterTaskResume = regexp.MustCompile(`(?i)\bRESUME\s*$`)
	reAlterTaskSusp   = regexp.MustCompile(`(?i)\bSUSPEND\s*$`)
	reAlterTaskSet    = regexp.MustCompile(`(?i)\bSET\b`)
	reAlterTaskUnset  = regexp.MustCompile(`(?i)\bUNSET\b`)
	reAlterTaskRemAfter     = regexp.MustCompile(`(?i)\bREMOVE\s+AFTER\b`)
	reAlterTaskRemAfterN    = regexp.MustCompile(`(?i)\bREMOVE\s+AFTER\s+(` + _identPath + `(?:\s*,\s*` + _identPath + `)*)`)
	reAlterTaskAddAfter     = regexp.MustCompile(`(?i)\bADD\s+AFTER\b`)
	reAlterTaskAddAfterN    = regexp.MustCompile(`(?i)\bADD\s+AFTER\s+(` + _identPath + `(?:\s*,\s*` + _identPath + `)*)`)
	reAlterTaskModifyAS     = regexp.MustCompile(`(?i)\bMODIFY\s+AS\b`)
	reAlterTaskModifyASBody = regexp.MustCompile(`(?i)\bMODIFY\s+AS\s+\S`)
	reAlterTaskModifyWhen   = regexp.MustCompile(`(?i)\bMODIFY\s+WHEN\b`)
	reAlterTaskModifyWhenE  = regexp.MustCompile(`(?i)\bMODIFY\s+WHEN\s+\S`)
	reAlterTaskSetFinalize  = regexp.MustCompile(`(?i)\bSET\s+FINALIZE\s*=`)
	reAlterTaskSetFinalizeN = regexp.MustCompile(`(?i)\bSET\s+FINALIZE\s*=\s*(` + _identPath + `)`)
	reAlterTaskUnsetProp   = regexp.MustCompile(`(?i)\bUNSET\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// ── CREATE ALERT ──────────────────────────────────────────────────────────
	reIsCreateAlert = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?ALERT\b`)
	alertProps      = strings.Join([]string{
		`WAREHOUSE`, `SCHEDULE`, `COMMENT`,
	}, "|")
	reAlertIfExists  = regexp.MustCompile(`(?i)\bIF\s*\(\s*EXISTS\s*\(`)
	reAlertThen      = regexp.MustCompile(`(?i)\bTHEN\b`)
	reAlertWarehouse = regexp.MustCompile(`(?i)\bWAREHOUSE\s*=`)
	reAlertSchedule  = regexp.MustCompile(`(?i)\bSCHEDULE\s*=`)

	// Regular expression to match property keys (e.g., KEY =)
	reProp = regexp.MustCompile(`(?i)\b([a-zA-Z_0-9]+)\s*=`)

	// ── CREATE PIPE ───────────────────────────────────────────────────────────
	reIsCreatePipe = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?PIPE\b`)
	pipeProps      = strings.Join([]string{
		`AUTO_INGEST`, `AWS_SNS_TOPIC`, `INTEGRATION`, `COMMENT`, `ERROR_INTEGRATION`,
	}, "|")

	// ── CREATE PROCEDURE ───────────────────────────────────────────────────────
	reIsCreateProcedure = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?PROCEDURE\b`)

	// ── CREATE FUNCTION ────────────────────────────────────────────────────────
	reIsCreateFunction = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:TEMPORARY\s+|TEMP\s+)?(?:AGGREGATE\s+)?FUNCTION\b`)

	// ── CREATE USER ───────────────────────────────────────────────────────────
	reIsCreateUser = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?USER\b`)
	userProps      = strings.Join([]string{
		`PASSWORD`, `LOGIN_NAME`, `DISPLAY_NAME`, `FIRST_NAME`, `MIDDLE_NAME`, `LAST_NAME`,
		`EMAIL`, `MUST_CHANGE_PASSWORD`, `DISABLED`, `DAYS_TO_EXPIRY`, `MINS_TO_UNLOCK`,
		`DEFAULT_WAREHOUSE`, `DEFAULT_NAMESPACE`, `DEFAULT_ROLE`, `RSA_PUBLIC_KEY`,
		`RSA_PUBLIC_KEY_2`, `COMMENT`, `TYPE`,
	}, "|")

	// ── CREATE ROLE ───────────────────────────────────────────────────────────
	reIsCreateRole = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?ROLE\b`)

	// ── CREATE MASKING POLICY ─────────────────────────────────────────────────
	reIsCreateMaskingPolicy = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?MASKING\s+POLICY\b`)

	// ── CREATE NETWORK POLICY ─────────────────────────────────────────────────
	reIsCreateNetworkPolicy       = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?NETWORK\s+POLICY\b`)
	reNetworkPolicyName           = regexp.MustCompile(`(?i)POLICY\s+(` + _identPath + `)`)
	reNetworkPolicyIPList         = regexp.MustCompile(`(?i)\b(ALLOWED_IP_LIST|BLOCKED_IP_LIST)\s*=\s*\(([^)]*)\)`)
	reNetworkPolicyHasAllowedIP   = regexp.MustCompile(`(?i)\bALLOWED_IP_LIST\s*=\s*\(([^)]*)\)`)
	reNetworkPolicyHasAllowedRules = regexp.MustCompile(`(?i)\bALLOWED_NETWORK_RULE_LIST\s*=\s*\(([^)]*)\)`)
	networkPolicyProps             = strings.Join([]string{
		`ALLOWED_IP_LIST`, `BLOCKED_IP_LIST`,
		`ALLOWED_NETWORK_RULE_LIST`, `BLOCKED_NETWORK_RULE_LIST`,
		`COMMENT`,
	}, "|")

	// ── CREATE ROW ACCESS POLICY ──────────────────────────────────────────────
	reIsCreateRowAccessPolicy = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?ROW\s+ACCESS\s+POLICY\b`)
	// reRowAccessPolicyAS matches the mandatory AS (...) parameter list.
	// The capture group holds the raw parameter list content; one level of
	// nested parens is supported to accommodate types like NUMBER(10,2).
	reRowAccessPolicyParamList = regexp.MustCompile(`(?i)\bAS\s*\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
	reRowAccessPolicyReturns = regexp.MustCompile(`(?i)\bRETURNS\s+BOOLEAN\b`)
	// reRowAccessPolicyArrow requires the -> to appear after RETURNS BOOLEAN,
	// preventing a bare -> elsewhere in the SQL from satisfying the check.
	reRowAccessPolicyArrow = regexp.MustCompile(`(?i)\bRETURNS\s+BOOLEAN\s*->`)
	reRowAccessPolicyASOpen    = regexp.MustCompile(`(?i)\bAS\s*\(`)

	// ── CREATE SESSION POLICY ─────────────────────────────────────────────────
	reIsCreateSessionPolicy   = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?SESSION\s+POLICY\b`)
	reSessionPolicyName       = regexp.MustCompile(`(?i)POLICY\s+(` + _identPath + `)`)
	reSessionIdleTimeout      = regexp.MustCompile(`(?i)\bSESSION_IDLE_TIMEOUT_MINS\s*=\s*(-?\d+)`)
	reSessionUIIdleTimeout    = regexp.MustCompile(`(?i)\bSESSION_UI_IDLE_TIMEOUT_MINS\s*=\s*(-?\d+)`)
	sessionPolicyProps        = strings.Join([]string{
		`SESSION_IDLE_TIMEOUT_MINS`, `SESSION_UI_IDLE_TIMEOUT_MINS`, `COMMENT`,
	}, "|")

	// ── CREATE PASSWORD POLICY ────────────────────────────────────────────────
	reIsCreatePasswordPolicy     = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?PASSWORD\s+POLICY\b`)
	rePasswordPolicyName         = regexp.MustCompile(`(?i)POLICY\s+(` + _identPath + `)`)
	rePasswordMinLength          = regexp.MustCompile(`(?i)\bPASSWORD_MIN_LENGTH\s*=\s*(-?\d+)`)
	rePasswordMaxLength          = regexp.MustCompile(`(?i)\bPASSWORD_MAX_LENGTH\s*=\s*(-?\d+)`)
	rePasswordMinUpperCase       = regexp.MustCompile(`(?i)\bPASSWORD_MIN_UPPER_CASE_CHARS\s*=\s*(-?\d+)`)
	rePasswordMinLowerCase       = regexp.MustCompile(`(?i)\bPASSWORD_MIN_LOWER_CASE_CHARS\s*=\s*(-?\d+)`)
	rePasswordMinNumeric         = regexp.MustCompile(`(?i)\bPASSWORD_MIN_NUMERIC_CHARS\s*=\s*(-?\d+)`)
	rePasswordMinSpecial         = regexp.MustCompile(`(?i)\bPASSWORD_MIN_SPECIAL_CHARS\s*=\s*(-?\d+)`)
	rePasswordMinAgeDays         = regexp.MustCompile(`(?i)\bPASSWORD_MIN_AGE_DAYS\s*=\s*(-?\d+)`)
	rePasswordMaxAgeDays         = regexp.MustCompile(`(?i)\bPASSWORD_MAX_AGE_DAYS\s*=\s*(-?\d+)`)
	rePasswordMaxRetries         = regexp.MustCompile(`(?i)\bPASSWORD_MAX_RETRIES\s*=\s*(-?\d+)`)
	rePasswordLockoutTimeMins    = regexp.MustCompile(`(?i)\bPASSWORD_LOCKOUT_TIME_MINS\s*=\s*(-?\d+)`)
	rePasswordHistory            = regexp.MustCompile(`(?i)\bPASSWORD_HISTORY\s*=\s*(-?\d+)`)
	passwordPolicyProps          = strings.Join([]string{
		`PASSWORD_MIN_LENGTH`, `PASSWORD_MAX_LENGTH`,
		`PASSWORD_MIN_UPPER_CASE_CHARS`, `PASSWORD_MIN_LOWER_CASE_CHARS`,
		`PASSWORD_MIN_NUMERIC_CHARS`, `PASSWORD_MIN_SPECIAL_CHARS`,
		`PASSWORD_MIN_AGE_DAYS`, `PASSWORD_MAX_AGE_DAYS`,
		`PASSWORD_MAX_RETRIES`, `PASSWORD_LOCKOUT_TIME_MINS`,
		`PASSWORD_HISTORY`, `COMMENT`,
	}, "|")

	// ── CREATE AGGREGATION POLICY ────────────────────────────────────────────
	reIsCreateAggregationPolicy   = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?AGGREGATION\s+POLICY\b`)
	reAggPolicyAS                 = regexp.MustCompile(`(?i)\bAS\s*\(`)
	reAggPolicyReturns            = regexp.MustCompile(`(?i)\bRETURNS\s+AGGREGATION_CONSTRAINT\b`)
	reAggPolicyArrow              = regexp.MustCompile(`(?i)\bRETURNS\s+AGGREGATION_CONSTRAINT\s*->`)
	reAggPolicyMinGroupSize       = regexp.MustCompile(`(?i)\bMIN_GROUP_SIZE\s*=>\s*(-?\d+)`)

	// ── CREATE PROJECTION POLICY ────────────────────────────────────────────
	reIsCreateProjectionPolicy    = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?PROJECTION\s+POLICY\b`)
	reProjPolicyAS                = regexp.MustCompile(`(?i)\bAS\s*\(`)
	reProjPolicyReturns           = regexp.MustCompile(`(?i)\bRETURNS\s+PROJECTION_CONSTRAINT\b`)
	reProjPolicyArrow             = regexp.MustCompile(`(?i)\bRETURNS\s+PROJECTION_CONSTRAINT\s*->`)
	reProjPolicyAllowValue        = regexp.MustCompile(`(?i)\bALLOW\s*=>\s*'([^']*)'`)

	// ── ALTER / DROP AGGREGATION POLICY ──────────────────────────────────────
	reIsAlterAggregationPolicy    = regexp.MustCompile(`(?i)^\s*ALTER\s+AGGREGATION\s+POLICY\b`)
	reIsDropAggregationPolicy     = regexp.MustCompile(`(?i)^\s*DROP\s+AGGREGATION\s+POLICY\b`)
	reAlterPolicyAction           = regexp.MustCompile(`(?i)\b(?:SET\s+BODY\s*->|SET\s+COMMENT\s*=|UNSET\s+COMMENT\b|RENAME\s+TO\b)`)
	reDropPolicyHasName           = regexp.MustCompile(`(?i)POLICY\s+(?:IF\s+EXISTS\s+)?` + _identPath)

	// ── ALTER / DROP PROJECTION POLICY ───────────────────────────────────────
	reIsAlterProjectionPolicy     = regexp.MustCompile(`(?i)^\s*ALTER\s+PROJECTION\s+POLICY\b`)
	reIsDropProjectionPolicy      = regexp.MustCompile(`(?i)^\s*DROP\s+PROJECTION\s+POLICY\b`)

	// ── CREATE PACKAGES POLICY ──────────────────────────────────────────────
	reIsCreatePackagesPolicy      = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?PACKAGES\s+POLICY\b`)
	rePkgPolicyLanguage           = regexp.MustCompile(`(?i)\bLANGUAGE\s+(\w+)`)

	// ── ALTER / DROP PACKAGES POLICY ────────────────────────────────────────
	reIsAlterPackagesPolicy       = regexp.MustCompile(`(?i)^\s*ALTER\s+PACKAGES\s+POLICY\b`)
	reIsDropPackagesPolicy        = regexp.MustCompile(`(?i)^\s*DROP\s+PACKAGES\s+POLICY\b`)
	reAlterPkgPolicyAction        = regexp.MustCompile(`(?i)\b(?:SET\s+(?:ALLOWLIST|BLOCKLIST|ADDITIONAL_CREATION_BLOCKLIST|COMMENT)\b|UNSET\s+(?:ALLOWLIST|BLOCKLIST|ADDITIONAL_CREATION_BLOCKLIST|COMMENT)\b)`)

	// ── GRANT / REVOKE ────────────────────────────────────────────────────────
	// reIsGrantRole is used inside validateGrant (not in the top-level dispatch)
	// to distinguish "GRANT ROLE <name>" (role assignment) from privilege grants.
	reIsGrantRole          = regexp.MustCompile(`(?i)^\s*GRANT\s+ROLE\b`)
	reIsGrantDatabaseRole  = regexp.MustCompile(`(?i)^\s*GRANT\s+DATABASE\s+ROLE\b`)
	reIsGrant              = regexp.MustCompile(`(?i)^\s*GRANT\b`)
	reIsRevoke             = regexp.MustCompile(`(?i)^\s*REVOKE\b`)
	reIsRevokeRole         = regexp.MustCompile(`(?i)^\s*REVOKE\s+ROLE\b`)
	reIsRevokeDatabaseRole = regexp.MustCompile(`(?i)^\s*REVOKE\s+DATABASE\s+ROLE\b`)
	// reGrantOnObject / reRevokeOnObject use a lazy ([\s\S]+?) to capture the
	// privilege list, stopping at the first occurrence of " ON ". This is safe
	// as long as no Snowflake privilege name itself contains the substring " ON ";
	// verify this assumption when adding new privileges to grantObjectPrivileges.
	reGrantOnObject  = regexp.MustCompile(`(?i)\bGRANT\s+([\s\S]+?)\s+ON\s+(ALL\s+|FUTURE\s+)?(` + _grantObjType + `)`)
	reRevokeOnObject = regexp.MustCompile(`(?i)\bREVOKE\s+(?:GRANT\s+OPTION\s+FOR\s+)?([\s\S]+?)\s+ON\s+(ALL\s+|FUTURE\s+)?(` + _grantObjType + `)`)
	reGrantee              = regexp.MustCompile(`(?i)\bTO\s+(?:ROLE|USER|DATABASE\s+ROLE|SHARE)\b`)
	reGranteeFrom          = regexp.MustCompile(`(?i)\bFROM\s+(?:ROLE|USER|DATABASE\s+ROLE|SHARE)\b`)
	reGrantAllFuture       = regexp.MustCompile(`(?i)\bON\s+(?:ALL|FUTURE)\b`)
	reGrantInQualifier     = regexp.MustCompile(`(?i)\bIN\s+(?:SCHEMA|DATABASE)\b`)
	reGrantToTable         = regexp.MustCompile(`(?i)\bTO\s+TABLE\b`)
	reWithGrantOption      = regexp.MustCompile(`(?i)\bWITH\s+GRANT\s+OPTION\b`)
	// reRevokeCascade / reRevokeRestrict match the keywords anywhere in the
	// statement. Unquoted identifiers that are exactly CASCADE or RESTRICT
	// (valid but uncommon Snowflake names) could in theory produce a false
	// positive — word boundaries mitigate this for composite names like
	// cascade_table, but a bare unquoted name CASCADE remains a theoretical
	// edge case. This is an accepted limitation documented here for future readers.
	reRevokeCascade  = regexp.MustCompile(`(?i)\bCASCADE\b`)
	reRevokeRestrict = regexp.MustCompile(`(?i)\bRESTRICT\b`)

	// ── CALL ──────────────────────────────────────────────────────────────────
	reIsCall         = regexp.MustCompile(`(?i)^\s*CALL\b`)
	reCallProcName   = regexp.MustCompile(`(?i)^\s*CALL\s+` + _identPath)
	reCallArgParens  = regexp.MustCompile(`(?i)^\s*CALL\s+` + _identPath + `\s*\(`)
	reCallInto       = regexp.MustCompile(`(?i)\bINTO\s+([^\s;,)]+)`)
	reWithProcAlias  = regexp.MustCompile(`(?i)^\s*WITH\s+(` + _ident + `)\s+AS\s+PROCEDURE\b`)
	// reAnyDollarTag matches both untagged ($$) and tagged ($tag$) Snowflake
	// dollar-quote delimiters; used to locate the closing body delimiter.
	reAnyDollarTag = regexp.MustCompile(`\$\w*\$`)

	// ── EXECUTE IMMEDIATE / EXECUTE TASK ─────────────────────────────────────
	reIsExecuteImmediate   = regexp.MustCompile(`(?i)^\s*EXECUTE\s+IMMEDIATE\b`)
	reIsExecuteTask        = regexp.MustCompile(`(?i)^\s*EXECUTE\s+TASK\b`)
	reIsExecute            = regexp.MustCompile(`(?i)^\s*EXECUTE\b`)
	// reExecImmHasArg requires a non-whitespace, non-semicolon character after
	// EXECUTE IMMEDIATE so that "EXECUTE IMMEDIATE ;" (space before semicolon)
	// is correctly flagged as missing an argument.
	reExecImmHasArg        = regexp.MustCompile(`(?i)^\s*EXECUTE\s+IMMEDIATE\s+[^\s;]`)
	reExecImmUsing         = regexp.MustCompile(`(?i)\bUSING\s*\(`)
	reExecImmUsingHasIdent = regexp.MustCompile(`(?i)\bUSING\s*\(\s*` + _ident)
	// reStripDollarQuoted strips dollar-quoted blocks ($$…$$ and $tag$…$tag$)
	// so that SQL content inside them does not cause false-positive USING checks.
	// The pattern intentionally matches mismatched tags ($foo$…$bar$): Go's
	// regexp package has no backreferences, so equal-tag enforcement is not
	// possible. Over-stripping is safe here — the goal is to remove content,
	// not to validate delimiters.
	reStripDollarQuoted    = regexp.MustCompile(`\$\w*\$[\s\S]*?\$\w*\$`)
	reExecTaskName         = regexp.MustCompile(`(?i)^\s*EXECUTE\s+TASK\s+` + _identPath)

	// ── PUT / GET / LIST / REMOVE stage commands ──────────────────────────────
	reIsPut          = regexp.MustCompile(`(?i)^\s*PUT\b`)
	reIsGet          = regexp.MustCompile(`(?i)^\s*GET\b`)
	reIsList         = regexp.MustCompile(`(?i)^\s*(?:LIST|LS)\b`)
	reIsRemove       = regexp.MustCompile(`(?i)^\s*(?:REMOVE|RM)\b`)
	// reFileURIArg matches a file:// URI argument (shared by PUT and GET).
	reFileURIArg     = regexp.MustCompile(`(?i)\bfile://\S+`)
	rePutKWStrip     = regexp.MustCompile(`(?i)^PUT\s+`)
	reStageRef       = regexp.MustCompile(`@\S+`)
	// rePutCorrectOrder validates that PUT has file:// before @stage.
	rePutCorrectOrder = regexp.MustCompile(`(?i)^\s*PUT\s+file://\S+\s+@\S+`)
	rePutSourceComp   = regexp.MustCompile(`(?i)\bSOURCE_COMPRESSION\s*=\s*(\w+)`)
	rePutOverwrite    = regexp.MustCompile(`(?i)\bOVERWRITE\s*=\s*(\w+)`)
	rePutAutoCompress = regexp.MustCompile(`(?i)\bAUTO_COMPRESS\s*=\s*(\w+)`)
	// reParallelOption matches a PARALLEL = <n> option (shared by PUT and GET).
	// The capture group includes an optional leading minus so that negative
	// values like PARALLEL = -1 are captured and fail the range check rather
	// than being silently skipped.
	reParallelOption = regexp.MustCompile(`(?i)\bPARALLEL\s*=\s*(-?\d+)`)
	reGetStageArg    = regexp.MustCompile(`(?i)^\s*GET\s+@\S+`)
	reListStageArg   = regexp.MustCompile(`(?i)^\s*(?:LIST|LS)\s+@\S+`)
	reRemoveStageArg = regexp.MustCompile(`(?i)^\s*(?:REMOVE|RM)\s+@\S+`)

	// validPutCompressions lists the accepted SOURCE_COMPRESSION values for PUT.
	validPutCompressions = []string{"AUTO_DETECT", "GZIP", "BZ2", "BROTLI", "ZSTD", "DEFLATE", "RAW_DEFLATE", "NONE"}

	// ── CREATE SHARE ─────────────────────────────────────────────────────────
	reIsCreateShare    = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?SHARE\b`)
	reCreateShareName  = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?SHARE\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)
	// ── ALTER SHARE ────────────────────────────────────────────────────────────
	reIsAlterShare          = regexp.MustCompile(`(?i)^\s*ALTER\s+SHARE\b`)
	reAlterShareAddAccounts = regexp.MustCompile(`(?i)\bADD\s+ACCOUNTS\b`)
	reAlterShareAddAcctsEq  = regexp.MustCompile(`(?i)\bADD\s+ACCOUNTS\s*=`)
	// reAlterShareHasAcctList verifies that ADD ACCOUNTS = is followed by at least one identifier.
	reAlterShareHasAcctList = regexp.MustCompile(`(?i)\bADD\s+ACCOUNTS\s*=\s*` + _ident)
	// reAlterShareRestrictTrailing matches RESTRICT only at the end of the cleaned
	// statement text. Anchoring to $ prevents false positives when the share name
	// (e.g. ALTER SHARE restrict ...) or a quoted identifier ("restrict") contains
	// the word RESTRICT somewhere other than the trailing position.
	reAlterShareRestrictTrailing = regexp.MustCompile(`(?i)\bRESTRICT\s*$`)

	// ── CREATE EVENT TABLE ──────────────────────────────────────────────────
	reIsCreateEventTable   = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?EVENT\s+TABLE\b`)
	reCreateEventTableName = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?EVENT\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)
	// reEventTableColumnList detects a parenthesised column list after the table name.
	// Event tables have a fixed schema and do not allow user-defined columns.
	reEventTableColumnList    = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?EVENT\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath + `\s*\(`)
	reEvtRetentionDays        = regexp.MustCompile(`(?i)\bDATA_RETENTION_TIME_IN_DAYS\s*=\s*(-?\d+\b|-?\w+)`)
	reEvtExtensionDays        = regexp.MustCompile(`(?i)\bMAX_DATA_EXTENSION_TIME_IN_DAYS\s*=\s*(-?\d+\b|-?\w+)`)
	reEvtChangeTrackingValue  = regexp.MustCompile(`(?i)\bCHANGE_TRACKING\s*=\s*(\w+)`)

	// ── CREATE EXTERNAL VOLUME ────────────────────────────────────────────────
	reIsCreateExternalVolume   = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?EXTERNAL\s+VOLUME\b`)
	reCreateExternalVolumeName = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?EXTERNAL\s+VOLUME\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)
	reExtVolHasStorageLocs     = regexp.MustCompile(`(?i)\bSTORAGE_LOCATIONS\s*=\s*\(`)
	// reExtVolLocationName matches NAME = ' to detect the required attribute.
	// \b already prevents matching embedded words like CONTAINER_NAME (_N has
	// no word boundary). The trailing ' further ensures we only match
	// string-valued assignments; on locClean, 'value' → '' so the opening '
	// of the empty placeholder still satisfies the pattern.
	reExtVolLocationName       = regexp.MustCompile(`(?i)\bNAME\s*=\s*'`)
	reExtVolStorageProvider    = regexp.MustCompile(`(?i)\bSTORAGE_PROVIDER\s*=\s*'([^']*)'`)
	reExtVolStorageBaseURL     = regexp.MustCompile(`(?i)\bSTORAGE_BASE_URL\s*=\s*'[^']*'`)
	reExtVolAwsRoleArn         = regexp.MustCompile(`(?i)\bSTORAGE_AWS_ROLE_ARN\s*=`)
	reExtVolAzureTenantID      = regexp.MustCompile(`(?i)\bAZURE_TENANT_ID\s*=`)
	reExtVolAwsExternalID      = regexp.MustCompile(`(?i)\bSTORAGE_AWS_EXTERNAL_ID\s*=`)
	// reExtVolHasEncryption detects any ENCRYPTION = ( block regardless of its
	// contents. Used as a coarse presence check before reExtVolEncryptionType,
	// which additionally requires TYPE = '...'. This ensures blocks like
	// ENCRYPTION = (KMS_KEY_ID = 'k') (no TYPE key) are not silently ignored.
	reExtVolHasEncryption  = regexp.MustCompile(`(?i)\bENCRYPTION\s*=\s*\(`)
	// reExtVolEncryptionType assumes TYPE is the first key inside the
	// ENCRYPTION block (i.e. ENCRYPTION = ( TYPE = '...' )). If TYPE appears
	// after another key (e.g. ENCRYPTION = (KMS_KEY_ID = 'k' TYPE = '...')),
	// the regex will not match and the validator will report a missing TYPE
	// key. This matches Snowflake's documented DDL convention where TYPE is
	// always the leading key in ENCRYPTION blocks.
	reExtVolEncryptionType = regexp.MustCompile(`(?i)\bENCRYPTION\s*=\s*\(\s*TYPE\s*=\s*'([^']*)'`)

	// ── CREATE TAG / ALTER TAG / DROP TAG ────────────────────────────────
	reIsCreateTag    = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?TAG\b`)
	reCreateTagName  = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?TAG\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)
	reIsAlterTag     = regexp.MustCompile(`(?i)^\s*ALTER\s+TAG\b`)
	reAlterTagName   = regexp.MustCompile(`(?i)^\s*ALTER\s+TAG\s+(?:IF\s+EXISTS\s+)?(` + _identPath + `)`)
	reIsDropTag      = regexp.MustCompile(`(?i)^\s*DROP\s+TAG\b`)
	reDropTagName    = regexp.MustCompile(`(?i)^\s*DROP\s+TAG\s+(?:IF\s+EXISTS\s+)?(` + _identPath + `)`)
	// reTagAllowedValues detects the presence of the ALLOWED_VALUES keyword
	// followed by whitespace. The actual string-literal list parsing is done
	// by reTagStringLiteralList.
	reTagAllowedValues     = regexp.MustCompile(`(?i)\bALLOWED_VALUES\s+`)
	reTagStringLiteralList = regexp.MustCompile(`(?i)\bALLOWED_VALUES\s+('(?:''|[^'])*'(?:\s*,\s*'(?:''|[^'])*')*)`)
	// reAlterTagRename matches ALTER TAG <name> RENAME TO <new_name>.
	reAlterTagRenameTo = regexp.MustCompile(`(?i)\bRENAME\s+TO\s+(` + _identPath + `)`)
	// reAlterTagAddAllowed matches ALTER TAG <name> ADD ALLOWED_VALUES.
	reAlterTagAddAllowed = regexp.MustCompile(`(?i)\bADD\s+ALLOWED_VALUES\b`)
	reAlterTagDropAllowed = regexp.MustCompile(`(?i)\bDROP\s+ALLOWED_VALUES\b`)
	// reAlterTagRenameToBare matches RENAME TO without requiring a name after it.
	reAlterTagRenameToBare = regexp.MustCompile(`(?i)\bRENAME\s+TO\b`)
	// reAlterTagUnsetAllowed matches ALTER TAG <name> UNSET ALLOWED_VALUES.
	reAlterTagUnsetAllowed = regexp.MustCompile(`(?i)\bUNSET\s+ALLOWED_VALUES\b`)
	reAlterTagSetComment   = regexp.MustCompile(`(?i)\bSET\s+COMMENT\s*=`)
	reAlterTagUnsetComment = regexp.MustCompile(`(?i)\bUNSET\s+COMMENT\b`)
	// reDropTagCascadeRestrict detects CASCADE or RESTRICT trailing the DROP TAG
	// statement. $ is safe here: parseText has trailing semicolons stripped and
	// clean has comments removed before matching.
	reDropTagCascadeRestrict = regexp.MustCompile(`(?i)\b(?:CASCADE|RESTRICT)\s*$`)

	// ── BEGIN / COMMIT / ROLLBACK / SAVEPOINT / RELEASE SAVEPOINT ────────
	reIsBegin            = regexp.MustCompile(`(?i)^\s*BEGIN\b`)
	reIsCommit           = regexp.MustCompile(`(?i)^\s*COMMIT\b`)
	reIsRollback         = regexp.MustCompile(`(?i)^\s*ROLLBACK\b`)
	reIsSavepoint        = regexp.MustCompile(`(?i)^\s*SAVEPOINT\b`)
	reIsReleaseSavepoint = regexp.MustCompile(`(?i)^\s*RELEASE\s+SAVEPOINT\b`)

	// BEGIN [WORK|TRANSACTION] [NAME <ident>] — the full valid form.
	reBeginValid = regexp.MustCompile(`(?i)^\s*BEGIN(\s+(WORK|TRANSACTION))?\s*$`)
	// Detects a scripting block: BEGIN followed by a known scripting keyword
	// (LET, IF, FOR, WHILE, LOOP, DECLARE, RETURN, CASE, CALL).
	// GetStatementRanges splits on semicolons, so the first "statement" of an
	// anonymous block looks like "BEGIN\n  LET x := 1" — this regex catches
	// that pattern so we skip transaction validation and txnDepth tracking.
	reBeginScripting = regexp.MustCompile(`(?i)^\s*BEGIN\s+(?:LET|IF|FOR|WHILE|LOOP|DECLARE|RETURN|CASE|CALL)\b`)
	// BEGIN ... NAME <ident> variant (supports quoted identifiers).
	reBeginName = regexp.MustCompile(`(?i)^\s*BEGIN(\s+(WORK|TRANSACTION))?\s+NAME\s+` + _ident + `\s*$`)
	// BEGIN ... NAME (bare, missing name).
	reBeginNameBare = regexp.MustCompile(`(?i)\bNAME\s*$`)
	// COMMIT [WORK] — full valid form.
	reCommitValid = regexp.MustCompile(`(?i)^\s*COMMIT(\s+WORK)?\s*$`)
	// ROLLBACK [WORK] [TO SAVEPOINT <ident>] — full valid form.
	reRollbackValid = regexp.MustCompile(`(?i)^\s*ROLLBACK(\s+WORK)?(\s+TO\s+SAVEPOINT\s+` + _ident + `)?\s*$`)
	// ROLLBACK ... TO without SAVEPOINT keyword.
	reRollbackToBare = regexp.MustCompile(`(?i)\bTO\b`)
	// ROLLBACK ... TO SAVEPOINT (without name).
	reRollbackToSavepointBare = regexp.MustCompile(`(?i)\bTO\s+SAVEPOINT\s*$`)
	// ROLLBACK ... TO SAVEPOINT <name> — detects the full form (used for block-level tracking).
	reRollbackToSavepointFull = regexp.MustCompile(`(?i)\bTO\s+SAVEPOINT\b`)
	// SAVEPOINT <name> — name is mandatory.
	reSavepointHasName = regexp.MustCompile(`(?i)^\s*SAVEPOINT\s+[^\s;]`)
	// RELEASE SAVEPOINT <name> — name is mandatory.
	reReleaseSavepointHasName = regexp.MustCompile(`(?i)^\s*RELEASE\s+SAVEPOINT\s+[^\s;]`)

	// ── USE ROLE / USE WAREHOUSE / USE SECONDARY ROLES ────────────────────
	reIsUseRole           = regexp.MustCompile(`(?i)^\s*USE\s+ROLE\b`)
	reIsUseWarehouse      = regexp.MustCompile(`(?i)^\s*USE\s+WAREHOUSE\b`)
	reIsUseSecondaryRoles = regexp.MustCompile(`(?i)^\s*USE\s+SECONDARY\s+ROLES\b`)
	// reUseRoleHasName requires a non-whitespace, non-semicolon character after
	// USE ROLE so that "USE ROLE ;" is correctly flagged as missing a role name.
	reUseRoleHasName      = regexp.MustCompile(`(?i)^\s*USE\s+ROLE\s+[^\s;]`)
	// reUseWarehouseHasName requires a non-whitespace, non-semicolon character after
	// USE WAREHOUSE so that "USE WAREHOUSE ;" is correctly flagged.
	reUseWarehouseHasName = regexp.MustCompile(`(?i)^\s*USE\s+WAREHOUSE\s+[^\s;]`)
	// reUseSecondaryRolesValue matches ALL or NONE after USE SECONDARY ROLES.
	reUseSecondaryRolesValue = regexp.MustCompile(`(?i)^\s*USE\s+SECONDARY\s+ROLES\s+(ALL|NONE)\b`)

	// ── ALTER SESSION ──────────────────────────────────────────────────────────
	reIsAlterSession    = regexp.MustCompile(`(?i)^\s*ALTER\s+SESSION\b`)
	reAlterSessionSet   = regexp.MustCompile(`(?i)^\s*ALTER\s+SESSION\s+SET\b`)
	reAlterSessionUnset = regexp.MustCompile(`(?i)^\s*ALTER\s+SESSION\s+UNSET\b`)
	// reAlterSessionParam extracts <PARAM> = <value> pairs from ALTER SESSION SET.
	// Value is either a quoted string (with escaped quotes) or a non-whitespace token.
	reAlterSessionParam      = regexp.MustCompile(`(?i)([A-Z_][A-Z0-9_]*)\s*=\s*('(?:''|[^'])*'|[^\s;]+)`)
	reAlterSessionParamSplit = regexp.MustCompile(`[,\s]+`)

	// ── CREATE STAGE ──────────────────────────────────────────────────────────
	reIsCreateStage = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TEMPORARY\s+)?STAGE\b`)
	// stageProps lists only top-level CREATE STAGE property keys.
	// Nested keys inside FILE_FORMAT=(...), ENCRYPTION=(...), DIRECTORY=(...),
	// CREDENTIALS=(...), and COPY_OPTIONS=(...) are stripped before validation
	// via stripParenContents so they never trigger a false positive.
	stageProps = strings.Join([]string{
		`URL`, `STORAGE_INTEGRATION`, `CREDENTIALS`, `ENCRYPTION`,
		`AWS_ACCESS_POINT_ARN`, `USE_PRIVATELINK_ENDPOINT`, `ENDPOINT`,
		`FILE_FORMAT`, `COPY_OPTIONS`, `COMMENT`, `DIRECTORY`,
	}, "|")

	// ── CREATE FILE FORMAT ───────────────────────────────────────────────────
	reIsCreateFileFormat  = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TEMPORARY\s+|TEMP\s+|TRANSIENT\s+)?FILE\s+FORMAT\b`)
	reFileFormatPropKey   = regexp.MustCompile(`(?i)\b([a-zA-Z_0-9]+)\s*=`)
	reFileFormatPropValue = regexp.MustCompile(`^\s*('[^']*'|[A-Za-z0-9_.-]+)`)
	reFileFormatValidEsc  = regexp.MustCompile(`^\\([ntr'\"]|x[0-9A-Fa-f]{2}|u[0-9A-Fa-f]{4}|[0-7]{1,3})$`)
	reFileFormatTemporary = regexp.MustCompile(`(?i)\b(TEMPORARY|TEMP)\b`)

	fileFormatCommonProps = []string{`TYPE`, `COMMENT`}

	fileFormatCsvProps = []string{
		`COMPRESSION`, `RECORD_DELIMITER`, `FIELD_DELIMITER`, `FILE_EXTENSION`,
		`PARSE_HEADER`, `SKIP_HEADER`, `SKIP_BLANK_LINES`, `DATE_FORMAT`,
		`TIME_FORMAT`, `TIMESTAMP_FORMAT`, `BINARY_FORMAT`, `ESCAPE`,
		`ESCAPE_UNENCLOSED_FIELD`, `TRIM_SPACE`, `FIELD_OPTIONALLY_ENCLOSED_BY`,
		`NULL_IF`, `ERROR_ON_COLUMN_COUNT_MISMATCH`, `REPLACE_INVALID_CHARACTERS`,
		`EMPTY_FIELD_AS_NULL`, `SKIP_BYTE_ORDER_MARK`, `ENCODING`, `MULTI_LINE`,
	}

	fileFormatJsonProps = []string{
		`COMPRESSION`, `FILE_EXTENSION`, `DATE_FORMAT`, `TIME_FORMAT`,
		`TIMESTAMP_FORMAT`, `BINARY_FORMAT`, `TRIM_SPACE`, `NULL_IF`,
		`ENABLE_OCTAL`, `ALLOW_DUPLICATE`, `STRIP_OUTER_ARRAY`, `STRIP_NULL_VALUES`,
		`REPLACE_INVALID_CHARACTERS`, `IGNORE_UTF8_ERRORS`, `SKIP_BYTE_ORDER_MARK`,
	}

	fileFormatAvroProps = []string{
		`COMPRESSION`, `TRIM_SPACE`, `REPLACE_INVALID_CHARACTERS`, `NULL_IF`, `SNAPPY_COMPRESSION_LEVEL`,
	}

	fileFormatOrcProps = []string{
		`TRIM_SPACE`, `REPLACE_INVALID_CHARACTERS`, `NULL_IF`,
	}

	fileFormatParquetProps = []string{
		`COMPRESSION`, `BINARY_AS_TEXT`, `USE_LOGICAL_TYPE`,
		`TRIM_SPACE`, `USE_VECTORIZED_SCANNER`,
		`REPLACE_INVALID_CHARACTERS`, `NULL_IF`,
	}

	fileFormatXmlProps = []string{
		`COMPRESSION`, `IGNORE_UTF8_ERRORS`, `PRESERVE_SPACE`, `STRIP_OUTER_ELEMENT`,
		`DISABLE_SNOWFLAKE_DATA`, `DISABLE_AUTO_CONVERT`, `REPLACE_INVALID_CHARACTERS`,
		`SKIP_BYTE_ORDER_MARK`,
	}

	// Pre-compiled allowed property regexes for each file format type
	reFileFormatAllowedCsv     = regexp.MustCompile("(?i)^(" + strings.Join(append(fileFormatCommonProps, fileFormatCsvProps...), "|") + ")$")
	reFileFormatAllowedJson    = regexp.MustCompile("(?i)^(" + strings.Join(append(fileFormatCommonProps, fileFormatJsonProps...), "|") + ")$")
	reFileFormatAllowedAvro    = regexp.MustCompile("(?i)^(" + strings.Join(append(fileFormatCommonProps, fileFormatAvroProps...), "|") + ")$")
	reFileFormatAllowedOrc     = regexp.MustCompile("(?i)^(" + strings.Join(append(fileFormatCommonProps, fileFormatOrcProps...), "|") + ")$")
	reFileFormatAllowedParquet = regexp.MustCompile("(?i)^(" + strings.Join(append(fileFormatCommonProps, fileFormatParquetProps...), "|") + ")$")
	reFileFormatAllowedXml     = regexp.MustCompile("(?i)^(" + strings.Join(append(fileFormatCommonProps, fileFormatXmlProps...), "|") + ")$")

	// ── CREATE ICEBERG TABLE ────────────────────────────────────────────────
	reIsCreateIcebergTable          = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?ICEBERG\s+TABLE\b`)
	reIsCreateTransientIcebergTable = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?TRANSIENT\s+ICEBERG\s+TABLE\b`)
	reGetStatementProperties        = regexp.MustCompile(`(?i)\b([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*('(?:''|[^'])*'|[\w$]+)`)

	// ── ALTER STAGE ───────────────────────────────────────────────────────────
	reIsAlterStage         = regexp.MustCompile(`(?i)^\s*ALTER\s+STAGE\b`)
	reAlterStageNoValidate = regexp.MustCompile(`(?i)\b(?:RENAME\s+TO|UNSET\b|SET\s+TAG\b)`)
	// alterStageProps lists valid top-level ALTER STAGE SET property keys.
	// SUBPATH is valid in ALTER STAGE ... REFRESH SUBPATH = '...'.
	alterStageProps = strings.Join([]string{
		`URL`, `STORAGE_INTEGRATION`, `CREDENTIALS`, `ENCRYPTION`,
		`AWS_ACCESS_POINT_ARN`, `USE_PRIVATELINK_ENDPOINT`,
		`FILE_FORMAT`, `COPY_OPTIONS`, `COMMENT`, `DIRECTORY`, `SUBPATH`,
	}, "|")

	// ── Shared ───────────────────────────────────────────────────────────────
	reIdentPathAnchored = regexp.MustCompile(`^` + _identPath)

	// ── SHOW ─────────────────────────────────────────────────────────────────
	reIsShow = regexp.MustCompile(`(?i)^\s*SHOW\b`)

	// ── DESCRIBE / DESC ──────────────────────────────────────────────────────
	reIsDescribe = regexp.MustCompile(`(?i)^\s*(?:DESCRIBE|DESC)\b`)

	// ── Parseable keywords ────────────────────────────────────────────────────
	parseableKWs = map[string]bool{
		"SELECT": true, "WITH": true, "INSERT": true, "UPDATE": true,
		"CREATE": true, "ALTER": true, "TRUNCATE": true, "CALL": true,
		"SHOW": true, "SET": true, "DROP": true, "UNDROP": true,
		"MERGE": true, "GRANT": true, "REVOKE": true, "COPY": true,
		"EXECUTE": true, "USE": true,
		"PUT": true, "GET": true, "LIST": true, "LS": true,
		"REMOVE": true, "RM": true,
		"DESCRIBE": true, "DESC": true,
		"BEGIN": true, "COMMIT": true, "ROLLBACK": true,
		"SAVEPOINT": true, "RELEASE": true,
	}
)

// showObjectTypes lists all valid Snowflake object type keywords after SHOW,
// sorted by word count descending so the longest match is attempted first.
// Within each group, entries are alphabetical.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show
var showObjectTypes = []string{
	// Three-word types
	"CORTEX SEARCH SERVICES",
	"DATA METRIC FUNCTIONS",
	"ROW ACCESS POLICIES",
	// Two-word types
	"AGGREGATION POLICIES",
	"AUTHENTICATION POLICIES",
	"CATALOG INTEGRATIONS",
	"COMPUTE POOLS",
	"DELEGATED AUTHORIZATIONS",
	"DYNAMIC TABLES",
	"EVENT TABLES",
	"EXPORTED KEYS",
	"EXTERNAL FUNCTIONS",
	"EXTERNAL TABLES",
	"EXTERNAL VOLUMES",
	"FAILOVER GROUPS",
	"FILE FORMATS",
	"FUTURE GRANTS",
	"GIT BRANCHES",
	"GIT REPOSITORIES",
	"HYBRID TABLES",
	"ICEBERG TABLES",
	"IMAGE REPOSITORIES",
	"IMPORTED KEYS",
	"MANAGED ACCOUNTS",
	"MASKING POLICIES",
	"MATERIALIZED VIEWS",
	"NETWORK POLICIES",
	"NETWORK RULES",
	"ORGANIZATION ACCOUNTS",
	"PACKAGES POLICIES",
	"PASSWORD POLICIES",
	"PRIMARY KEYS",
	"PROJECTION POLICIES",
	"REPLICATION DATABASES",
	"REPLICATION GROUPS",
	"RESOURCE MONITORS",
	"SESSION POLICIES",
	"UNIQUE KEYS",
	// Single-word types
	"ALERTS",
	"CHANNELS",
	"COLUMNS",
	"CONNECTIONS",
	"DATABASES",
	"ENDPOINTS",
	"FUNCTIONS",
	"GRANTS",
	"INTEGRATIONS",
	"LISTINGS",
	"LOCKS",
	"MODELS",
	"NOTEBOOKS",
	"OBJECTS",
	"PARAMETERS",
	"PIPES",
	"PROCEDURES",
	"REGIONS",
	"ROLES",
	"SCHEMAS",
	"SECRETS",
	"SEQUENCES",
	"SERVICES",
	"SHARES",
	"SNAPSHOTS",
	"STAGES",
	"STREAMLITS",
	"STREAMS",
	"TABLES",
	"TAGS",
	"TASKS",
	"TRANSACTIONS",
	"USERS",
	"VARIABLES",
	"VIEWS",
	"WAREHOUSES",
}

// showTerseEligible contains object types that support the TERSE modifier.
var showTerseEligible = map[string]bool{
	"TABLES":          true,
	"EXTERNAL TABLES": true,
	"VIEWS":           true,
	"SCHEMAS":         true,
	"DATABASES":       true,
	"STAGES":          true,
	"STREAMS":         true,
	"USERS":           true,
}

// showHistoryEligible contains object types that support the HISTORY modifier.
var showHistoryEligible = map[string]bool{
	"PIPES":                  true,
	"REPLICATION DATABASES":  true,
}

// showNoClauseValidation contains object types where optional clause validation
// is skipped because they have non-standard syntax (e.g. SHOW GRANTS ON ...).
var showNoClauseValidation = map[string]bool{
	"GRANTS":        true,
	"FUTURE GRANTS": true,
	"PARAMETERS":    true,
}

// describeObjectTypes lists all valid Snowflake object type keywords after
// DESCRIBE / DESC, sorted by word count descending so the longest match is
// attempted first.  Within each group, entries are alphabetical.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/desc
var describeObjectTypes = []string{
	// Four-word types
	"OPENFLOW DATA PLANE INTEGRATION",
	// Three-word types
	"CORTEX SEARCH SERVICE",
	"ONLINE FEATURE TABLE",
	"ROW ACCESS POLICY",
	"STORAGE LIFECYCLE POLICY",
	// Two-word types
	"AGGREGATION POLICY",
	"APPLICATION PACKAGE",
	"AUTHENTICATION POLICY",
	"BACKUP POLICY",
	"BACKUP SET",
	"CATALOG INTEGRATION",
	"COMPUTE POOL",
	"DBT PROJECT",
	"DCM PROJECT",
	"DYNAMIC TABLE",
	"EVENT TABLE",
	"EXTERNAL AGENT",
	"EXTERNAL TABLE",
	"EXTERNAL VOLUME",
	"FAILOVER GROUP",
	"FEATURE POLICY",
	"FILE FORMAT",
	"GIT REPOSITORY",
	"ICEBERG TABLE",
	"JOIN POLICY",
	"MAINTENANCE POLICY",
	"MASKING POLICY",
	"MATERIALIZED VIEW",
	"MCP SERVER",
	"MODEL MONITOR",
	"NETWORK POLICY",
	"NETWORK RULE",
	"NOTIFICATION INTEGRATION",
	"ORGANIZATION PROFILE",
	"PACKAGES POLICY",
	"PASSWORD POLICY",
	"POSTGRES INSTANCE",
	"PRIVACY POLICY",
	"PROJECTION POLICY",
	"REPLICATION GROUP",
	"RESOURCE MONITOR",
	"SEMANTIC VIEW",
	"SESSION POLICY",
	"SNAPSHOT POLICY",
	"SNAPSHOT SET",
	// Single-word types
	"AGENT",
	"ALERT",
	"APPLICATION",
	"CONFIGURATION",
	"DATABASE",
	"FUNCTION",
	"GATEWAY",
	"INTEGRATION",
	"LISTING",
	"NOTEBOOK",
	"PIPE",
	"PROCEDURE",
	"RESULT",
	"ROLE",
	"SCHEMA",
	"SECRET",
	"SEQUENCE",
	"SERVICE",
	"SHARE",
	"SNAPSHOT",
	"SPECIFICATION",
	"STAGE",
	"STREAM",
	"STREAMLIT",
	"TABLE",
	"TAG",
	"TASK",
	"TRANSACTION",
	"TYPE",
	"USER",
	"VIEW",
	"WAREHOUSE",
}

// describeAccountLevel contains account-level object types that should not be
// qualified with a database or schema prefix (db.schema.name).
var describeAccountLevel = map[string]bool{
	"WAREHOUSE":                        true,
	"USER":                             true,
	"ROLE":                             true,
	"INTEGRATION":                      true,
	"DATABASE":                         true,
	"SHARE":                            true,
	"RESOURCE MONITOR":                 true,
	"NOTIFICATION INTEGRATION":         true,
	"CATALOG INTEGRATION":              true,
	"COMPUTE POOL":                     true,
	"EXTERNAL VOLUME":                  true,
	"NETWORK POLICY":                   true,
	"ORGANIZATION PROFILE":             true,
	"OPENFLOW DATA PLANE INTEGRATION":  true,
	"POSTGRES INSTANCE":                true,
	"SPECIFICATION":                    true,
}

// describeNeedsSignature contains object types that require a parenthesised
// parameter-type signature for disambiguation (name overloading).
var describeNeedsSignature = map[string]bool{
	"FUNCTION":  true,
	"PROCEDURE": true,
}

// grantObjectPrivileges maps canonical Snowflake object types (upper-cased) to
// their valid privilege names. OWNERSHIP, ALL, and ALL PRIVILEGES are handled
// as universal special cases in validateGrant / validateRevoke and are omitted
// from this map intentionally.
var grantObjectPrivileges = map[string][]string{
	"TABLE": {
		"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "REBUILD",
		"EVOLVE SCHEMA",
	},
	"VIEW": {"SELECT", "REFERENCES"},
	"STAGE": {"READ", "WRITE"},
	"WAREHOUSE": {"USAGE", "MODIFY", "MONITOR", "OPERATE", "APPLYBUDGET"},
	"DATABASE": {
		"USAGE", "MODIFY", "MONITOR", "CREATE SCHEMA",
		"IMPORTED PRIVILEGES", "REFERENCE_USAGE", "APPLYBUDGET",
	},
	"SCHEMA": {
		"USAGE", "MODIFY", "MONITOR",
		"CREATE TABLE", "CREATE VIEW", "CREATE STAGE", "CREATE STREAM",
		"CREATE TASK", "CREATE PIPE", "CREATE FUNCTION", "CREATE PROCEDURE",
		"CREATE SEQUENCE", "CREATE FILE FORMAT", "CREATE MASKING POLICY",
		"CREATE ROW ACCESS POLICY", "CREATE DYNAMIC TABLE",
		"ADD SEARCH OPTIMIZATION", "CREATE ALERT", "CREATE NETWORK RULE",
		"CREATE SECRET", "CREATE SNOWFLAKE.CORTEX.SEARCH SERVICE",
		"CREATE STREAMLIT", "CREATE NOTEBOOK",
		"CREATE HYBRID TABLE", "CREATE ICEBERG TABLE", "CREATE EXTERNAL TABLE",
		"APPLYBUDGET",
	},
	"PIPE":        {"MONITOR", "OPERATE"},
	"INTEGRATION": {"USAGE"},
	"TASK":        {"MONITOR", "OPERATE"},
	"STREAM":      {"SELECT"},
	"USER":        {"MONITOR"},
	// TODO: add SEQUENCE, FILE FORMAT, EXTERNAL TABLE object-level privileges,
	// MATERIALIZED VIEW, and DYNAMIC TABLE when their privilege sets are confirmed.
	// Until then, validation is silently skipped for those object types
	// (knownObj = false), which avoids false positives but means invalid
	// privileges go unchecked.
	"ACCOUNT": {
		"CREATE ROLE", "CREATE USER", "CREATE WAREHOUSE", "CREATE DATABASE",
		"CREATE INTEGRATION", "CREATE NETWORK POLICY", "MANAGE GRANTS",
		"MONITOR USAGE", "EXECUTE TASK", "EXECUTE ALERT", "EXECUTE MANAGED TASK",
		"IMPORT SHARE", "OVERRIDE SHARE RESTRICTIONS", "ATTACH POLICY",
		"APPLY MASKING POLICY", "APPLY ROW ACCESS POLICY",
		"APPLY SESSION POLICY", "APPLY TAG", "APPLY AGGREGATION POLICY",
		"MANAGE WAREHOUSES", "CREATE SHARE", "APPLYBUDGET",
		"BIND SERVICE ENDPOINT", "CREATE COMPUTE POOL", "CREATE EXTERNAL VOLUME",
		"MANAGE ACCOUNT SUPPORT CASES", "RESOLVE ALL",
	},
}

// grantObjectTypePlurals maps plural/alternative Snowflake object-type names
// (upper-cased) to their canonical singular form, used when parsing
// ON ALL <objects> / ON FUTURE <objects> clauses.
var grantObjectTypePlurals = map[string]string{
	"TABLES":       "TABLE",
	"VIEWS":        "VIEW",
	"STAGES":       "STAGE",
	"WAREHOUSES":   "WAREHOUSE",
	"DATABASES":    "DATABASE",
	"SCHEMAS":      "SCHEMA",
	"INTEGRATIONS": "INTEGRATION",
	"TASKS":        "TASK",
	"STREAMS":      "STREAM",
	"USERS":        "USER",
	"PIPES":        "PIPE",
}

// ── ValidateSnowflakePatterns ─────────────────────────────────────────────────

// ValidateSnowflakePatterns checks each statement in stmtRanges against a set
// of Snowflake-specific rules that cannot be expressed as generic SQL syntax
// errors.  It is a pure Go replacement for the validateWithParser function in
// sqlDiagnostics.ts; the node-sql-parser dependency is dropped because
// ValidateSyntax already covers generic syntax errors via its tokenizer.
func ValidateSnowflakePatterns(sql string, stmtRanges []StatementRange) []DiagMarker {
	var markers []DiagMarker

	// ── Block-level transaction tracking ─────────────────────────────────
	txnDepth := 0
	var txnBeginRange StatementRange // range of the opening BEGIN (for end-of-script warning)

	for _, r := range stmtRanges {
		rawText := sqlStmt(sql, r)
		stripped := strings.TrimSpace(stripCommentsSQL(rawText))
		if stripped == "" {
			continue
		}
		firstTok := getFirstSQLToken(rawText)
		if !parseableKWs[firstTok] {
			continue
		}
		// parseText has trailing semicolons removed, used for preamble matching.
		parseText := strings.TrimRight(strings.TrimSpace(rawText), "; \t\r\n")

		// ── Custom check 1: LATERALFLATTEN typo ──────────────────────────
		if reLateralFlatten.MatchString(stripped) {
			seen := make(map[string]struct{})
			for _, loc := range reLateralFlatten.FindAllStringIndex(rawText, -1) {
				upTo := rawText[:loc[0]]
				lines := strings.Split(upTo, "\n")
				errLine := r.StartLine + len(lines) - 1
				errCol := len(lines[len(lines)-1]) + 1
				key := fmt.Sprintf("%d-%d", errLine, errCol)
				if _, dup := seen[key]; !dup {
					seen[key] = struct{}{}
					markers = append(markers, DiagMarker{
						StartLineNumber: errLine, StartColumn: errCol,
						EndLineNumber: errLine, EndColumn: errCol + 14,
						Message:  "Typo detected: Did you mean 'LATERAL FLATTEN'?",
						Severity: 4,
					})
				}
			}
		}

		// ── Custom check 2: FLATTEN without LATERAL ───────────────────────
		if reFlattenFromJoin.MatchString(stripped) &&
			!reLateralOK.MatchString(stripped) &&
			!reTableFlatten.MatchString(stripped) {
			reFlattenRaw := regexp.MustCompile(`(?i)\bFLATTEN\b`)
			seen := make(map[string]struct{})
			for _, loc := range reFlattenRaw.FindAllStringIndex(rawText, -1) {
				upTo := rawText[:loc[0]]
				lines := strings.Split(upTo, "\n")
				errLine := r.StartLine + len(lines) - 1
				errCol := len(lines[len(lines)-1]) + 1
				key := fmt.Sprintf("%d-%d", errLine, errCol)
				if _, dup := seen[key]; !dup {
					seen[key] = struct{}{}
					markers = append(markers, DiagMarker{
						StartLineNumber: errLine, StartColumn: errCol,
						EndLineNumber: errLine, EndColumn: errCol + 7,
						Message:  "FLATTEN used as a table function requires LATERAL. Use LATERAL FLATTEN(...) or TABLE(FLATTEN(...)).",
						Severity: 4,
					})
				}
			}
		}

		// ── Custom check 3: variant path with dots (payload.field.sub) ────
		for _, m := range reVariantDotPath.FindAllStringSubmatchIndex(stripped, -1) {
			submatch := stripped[m[0]:m[1]]
			g1Start, g1End := m[2], m[3]
			word1 := strings.ToLower(stripped[g1Start:g1End])
			if word1 == "payload" {
				rawPat := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(submatch) + `\b`)
				for _, rloc := range rawPat.FindAllStringIndex(rawText, -1) {
					upTo := rawText[:rloc[0]]
					lines := strings.Split(upTo, "\n")
					errLine := r.StartLine + len(lines) - 1
					errCol := len(lines[len(lines)-1]) + 1
					// Suggest colon notation
					parts := strings.SplitN(submatch, ".", 3)
					suggestion := parts[0] + ":" + strings.Join(parts[1:], ".")
					markers = append(markers, DiagMarker{
						StartLineNumber: errLine, StartColumn: errCol,
						EndLineNumber: errLine, EndColumn: errCol + len(submatch),
						Message:  "Missing colon for variant path. Use ':' for Snowflake JSON traversal (e.g. " + suggestion + ").",
						Severity: 4,
					})
				}
			}
		}

		// ── Custom check 4: QUALIFY after ORDER BY ────────────────────────
		if reQualifyAfterOrder.MatchString(stripped) {
			reQualify := regexp.MustCompile(`(?i)\bQUALIFY\b`)
			seen := make(map[string]struct{})
			for _, loc := range reQualify.FindAllStringIndex(rawText, -1) {
				upTo := rawText[:loc[0]]
				lines := strings.Split(upTo, "\n")
				errLine := r.StartLine + len(lines) - 1
				errCol := len(lines[len(lines)-1]) + 1
				key := fmt.Sprintf("%d-%d", errLine, errCol)
				if _, dup := seen[key]; !dup {
					seen[key] = struct{}{}
					markers = append(markers, DiagMarker{
						StartLineNumber: errLine, StartColumn: errCol,
						EndLineNumber: errLine, EndColumn: errCol + 7,
						Message:  "Snowflake 'QUALIFY' must come after 'WHERE' or 'HAVING' but before 'ORDER BY'.",
						Severity: 4,
					})
				}
			}
		}

		// ── Custom check 5: MERGE statement rules ─────────────────────────
		if firstTok == "MERGE" {
			// Find all WHEN clauses
			reWhen := regexp.MustCompile(`(?i)\bWHEN\s+`)
			locs := reWhen.FindAllStringIndex(rawText, -1)
			for i, loc := range locs {
				start := loc[0]
				end := len(rawText)
				if i+1 < len(locs) {
					end = locs[i+1][0]
				}
				clause := rawText[start:end]
				clauseStripped := strings.TrimSpace(stripCommentsSQL(clause))

				lines := strings.Split(rawText[:start], "\n")
				errLine := r.StartLine + len(lines) - 1
				errCol := len(lines[len(lines)-1]) + 1

				// 1. WHEN MATCHED (but NOT 'NOT MATCHED')
				if regexp.MustCompile(`(?i)^WHEN\s+MATCHED\b`).MatchString(clauseStripped) {
					if regexp.MustCompile(`(?i)\bTHEN\s+INSERT\b`).MatchString(clauseStripped) {
						markers = append(markers, DiagMarker{
							StartLineNumber: errLine, StartColumn: errCol,
							EndLineNumber: errLine, EndColumn: errCol + 12,
							Message:  "INSERT action is not allowed in WHEN MATCHED clause. Use UPDATE or DELETE.",
							Severity: 4,
						})
					}
				}

				// 2. WHEN NOT MATCHED (specifically NOT 'BY SOURCE')
				if regexp.MustCompile(`(?i)^WHEN\s+NOT\s+MATCHED\b`).MatchString(clauseStripped) &&
					!regexp.MustCompile(`(?i)\bBY\s+SOURCE\b`).MatchString(clauseStripped) {
					if regexp.MustCompile(`(?i)\bTHEN\s+(UPDATE|DELETE)\b`).MatchString(clauseStripped) {
						markers = append(markers, DiagMarker{
							StartLineNumber: errLine, StartColumn: errCol,
							EndLineNumber: errLine, EndColumn: errCol + 16,
							Message:  "UPDATE or DELETE action is not allowed in WHEN NOT MATCHED clause. Use INSERT.",
							Severity: 4,
						})
					}
				}

				// 3. WHEN NOT MATCHED BY SOURCE (Not supported by Snowflake)
				if regexp.MustCompile(`(?i)^WHEN\s+NOT\s+MATCHED\s+BY\s+SOURCE\b`).MatchString(clauseStripped) {
					markers = append(markers, DiagMarker{
						StartLineNumber: errLine, StartColumn: errCol,
						EndLineNumber: errLine, EndColumn: errCol + 26,
						Message:  "WHEN NOT MATCHED BY SOURCE is not supported by Snowflake. Use a subquery with a LEFT JOIN as your source to identify missing rows.",
						Severity: 4,
					})
				}
			}
		}

		// ── Preamble: CREATE VIEW ─────────────────────────────────────────
		if reIsCreateView.MatchString(parseText) {
			if !reValidCreateViewPreamble.MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE VIEW statement.", 4))
			}
			continue
		}

		// ── Preamble: CREATE EXTERNAL TABLE ──────────────────────────────
		if reIsCreateExternalTable.MatchString(parseText) {
			preambleMatch := reExternalTablePreamble.FindString(parseText)
			if preambleMatch == "" {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE EXTERNAL TABLE statement.", 4))
				continue
			}

			// OR REPLACE is already matched by the preamble regex if present, but it's invalid for EXTERNAL TABLE.
			if reOrReplace.MatchString(preambleMatch) {
				markers = append(markers, diagMarkerSpan(r, "OR REPLACE is not supported for EXTERNAL TABLE. Use DROP and CREATE.", 4))
				continue
			}

			// Use a clean version of stripped without string literals to avoid false positives
			// in string properties like COMMENT = '... CLUSTER BY ...'
			clean := reStripStringLiterals.ReplaceAllString(stripped, " ")

			if rePatternClusterBy.MatchString(clean) {
				markers = append(markers, diagMarkerSpan(r, "CLUSTER BY is not supported for EXTERNAL TABLE.", 4))
				continue
			}
			if reDataRetention.MatchString(clean) {
				markers = append(markers, diagMarkerSpan(r, "DATA_RETENTION_TIME_IN_DAYS is not applicable to EXTERNAL TABLE.", 4))
				continue
			}

			rest := strings.TrimSpace(parseText[len(preambleMatch):])
			rest = strings.TrimSpace(stripCommentsSQL(rest))

			if !strings.HasPrefix(rest, "(") {
				markers = append(markers, diagMarkerSpan(r, "EXTERNAL TABLE must have a column list.", 4))
				continue
			}

			// Find matching close paren for column list
			endIdx := findMatchingParen(rest)
			if endIdx == -1 {
				markers = append(markers, diagMarkerSpan(r, "Unclosed column list in CREATE EXTERNAL TABLE statement.", 4))
				continue
			}

			colList := rest[1:endIdx]
			// Column validation: must use AS <expr> for non-partition columns
			// We split by top-level commas and check each column.
			cols := splitTopLevelCommas(colList)

			// Snowflake rejects empty column lists
			if len(cols) == 0 || (len(cols) == 1 && strings.TrimSpace(cols[0]) == "") {
				markers = append(markers, diagMarkerSpan(r, "Column list must not be empty.", 4))
				continue
			}

			hasColError := false
			for _, col := range cols {
				col = strings.TrimSpace(col)
				if col == "" {
					continue
				}
				// Skip if it's a constraint like PRIMARY KEY or UNIQUE (though rare in EXTERNAL TABLE)
				if reConstraintCol.MatchString(col) {
					continue
				}
				// External table column must have "AS (" or "AS <expr>"
				if !reVirtualColAS.MatchString(col) {
					markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("Column '%s' in EXTERNAL TABLE must be a virtual column using AS <expr>.", col), 4))
					hasColError = true
				}
			}
			if hasColError {
				continue
			}

			after := strings.TrimSpace(rest[endIdx+1:])

			// Check for PARTITION BY
			if loc := rePartitionBy.FindStringIndex(after); loc != nil {
				// The next non-whitespace character must be '('
				remainder := strings.TrimSpace(after[loc[1]:])
				if !strings.HasPrefix(remainder, "(") {
					markers = append(markers, diagMarkerSpan(r, "PARTITION BY in EXTERNAL TABLE requires a parenthesised column list.", 4))
					continue
				}
				partEnd := findMatchingParen(remainder)
				if partEnd != -1 {
					after = strings.TrimSpace(remainder[partEnd+1:])
				} else {
					markers = append(markers, diagMarkerSpan(r, "Unclosed parenthesised column list in PARTITION BY clause.", 4))
					continue
				}
			}

			// Mandatory WITH LOCATION and FILE_FORMAT
			if !reWithLocation.MatchString(after) {
				markers = append(markers, diagMarkerSpan(r, "WITH LOCATION = @<stage> is mandatory for EXTERNAL TABLE.", 4))
				continue
			}
			if !reFileFormat.MatchString(after) {
				markers = append(markers, diagMarkerSpan(r, "FILE_FORMAT is mandatory for EXTERNAL TABLE.", 4))
				continue
			}

			// Validate remaining properties
			if after != "" && !extTablePropsRe.MatchString(after) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE EXTERNAL TABLE properties.", 4))
			}
			continue
		}

		// ── Preamble: CREATE ICEBERG TABLE ───────────────────────────────
		if reIsCreateIcebergTable.MatchString(parseText) {
			markers = append(markers, validateCreateIcebergTable(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE HYBRID TABLE ───────────────────────────────
		if reIsCreateHybridTable.MatchString(parseText) {
			markers = append(markers, validateCreateHybridTable(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE TABLE ────────────────────────────────────────
		if reIsCreateTable.MatchString(parseText) {
			// Specific Snowflake Error: OR REPLACE and IF NOT EXISTS are mutually exclusive
			if reOrReplace.MatchString(parseText) && reIfNotExists.MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE TABLE statement.", 4))
				continue
			}

			preambleMatch := reCreateTablePreamble.FindString(parseText)
			if preambleMatch == "" {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE TABLE statement.", 4))
				continue
			}
			rest := strings.TrimSpace(parseText[len(preambleMatch):])
			rest = strings.TrimSpace(strings.TrimSpace(stripCommentsSQL(rest)))

			isValid := false
			switch {
			case reCreateTableBackup.MatchString(rest):
				isValid = true
			case reCreateTableCTAS.MatchString(rest):
				isValid = true
			case reCreateTableClone.MatchString(rest):
				isValid = true
			case reCreateTableTemplate.MatchString(rest):
				isValid = true
			case strings.HasPrefix(rest, "("):
				// Find matching close paren (depth-aware, skip strings)
				endIdx := findMatchingParen(rest)
				if endIdx != -1 {
					colsContent := rest[1:endIdx]
					colsClean := reStripStringLiterals.ReplaceAllString(colsContent, " ")
					if reIndexKeyword.MatchString(colsClean) {
						markers = append(markers, diagMarkerSpan(r, "Secondary indexes (INDEX) are only supported on hybrid tables.", 4))
					}

					after := strings.TrimSpace(rest[endIdx+1:])
					tablePropsRe := regexp.MustCompile(`(?i)^(?:(?:` + tableProps + `)(?:\s+|$))*$`)
					if after == "" || tablePropsRe.MatchString(after) || reCreateTableCTAS.MatchString(after) {
						isValid = true
					}
				}
			}

			if !isValid {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE TABLE statement.", 4))
			}
			continue
		}

		// ── Preamble: CREATE DATABASE / SCHEMA ────────────────────────────
		if m := reIsCreateDbSchema.FindStringSubmatch(parseText); m != nil {
			kind := strings.ToUpper(m[1])
			if !reValidCreateDbSchema.MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE "+kind+" statement.", 4))
			}
			continue
		}

		// ── Preamble: DROP DATABASE / SCHEMA ─────────────────────────────
		if m := reIsDropDbSchema.FindStringSubmatch(parseText); m != nil {
			kind := strings.ToUpper(m[1])
			if !reValidDropDbSchema.MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in DROP "+kind+" statement.", 4))
			}
			continue
		}

		// ── Preamble: CREATE SEQUENCE ─────────────────────────────────────
		if reIsCreateSeq.MatchString(parseText) {
			unquoted := regexp.MustCompile(`"[^"]+"`).ReplaceAllString(parseText, `""`)
			bothOrderNoorder := regexp.MustCompile(`(?i)\bORDER\b`).MatchString(unquoted) &&
				regexp.MustCompile(`(?i)\bNOORDER\b`).MatchString(unquoted)
			if !reValidCreateSeq.MatchString(parseText) || bothOrderNoorder {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE SEQUENCE statement.", 4))
			}
			continue
		}

		// ── Preamble: ALTER SEQUENCE ──────────────────────────────────────
		if reIsAlterSeq.MatchString(parseText) {
			unquoted := regexp.MustCompile(`"[^"]+"`).ReplaceAllString(parseText, `""`)
			bothOrderNoorder := regexp.MustCompile(`(?i)\bORDER\b`).MatchString(unquoted) &&
				regexp.MustCompile(`(?i)\bNOORDER\b`).MatchString(unquoted)
			if !reValidAlterSeq.MatchString(parseText) || bothOrderNoorder {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in ALTER SEQUENCE statement.", 4))
			}
			continue
		}

		// ── Preamble: COPY INTO ──────────────────────────────────────────
		if reIsCopyInto.MatchString(parseText) {
			markers = append(markers, validateCopyInto(parseText, r)...)
			continue
		}

		// ── Preamble: DROP SEQUENCE ───────────────────────────────────────
		if reIsDropSeq.MatchString(parseText) {
			if !reValidDropSeq.MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in DROP SEQUENCE statement.", 4))
			}
			continue
		}

		// ── Preamble: CREATE DYNAMIC TABLE ───────────────────────────────
		if reIsCreateDynTable.MatchString(parseText) {
			if !reDynHasTargetLag.MatchString(parseText) ||
				!reDynHasWarehouse.MatchString(parseText) ||
				!reDynHasAs.MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE DYNAMIC TABLE statement.", 4))
			}
			continue
		}

		// ── Preamble: CREATE INTEGRATION ─────────────────────────────────
		if reIsCreateIntegration.MatchString(parseText) {
			// 1. Account-level check: no prefix allowed
			if m := reIntegrationName.FindStringSubmatch(parseText); m != nil {
				name := m[1]
				if strings.Contains(name, ".") {
					markers = append(markers, diagMarkerSpan(r, "Integrations are account-level objects and cannot have a database or schema prefix.", 4))
				}
			}

			// 2. Type-specific checks
			upper := strings.ToUpper(parseText)
			switch {
			case strings.Contains(upper, "API INTEGRATION"):
				if !reIntegrationProvider.MatchString(parseText) {
					markers = append(markers, diagMarkerSpan(r, "Missing required parameter API_PROVIDER for API Integration.", 4))
				}
			case strings.Contains(upper, "NOTIFICATION INTEGRATION"):
				if m := reIntegrationType.FindStringSubmatch(parseText); m != nil {
					t := strings.ToUpper(m[1])
					if t != "EMAIL" && t != "QUEUE" {
						markers = append(markers, diagMarkerSpan(r, "Invalid TYPE for Notification Integration. Valid types are EMAIL, QUEUE.", 4))
					}
				}
			case strings.Contains(upper, "SECURITY INTEGRATION"):
				if !reIntegrationType.MatchString(parseText) {
					markers = append(markers, diagMarkerSpan(r, "Missing required parameter TYPE for Security Integration.", 4))
				}
			case strings.Contains(upper, "EXTERNAL ACCESS INTEGRATION"):
				if regexp.MustCompile(`(?i)\bMAX_RETRIES\s*=`).MatchString(parseText) {
					markers = append(markers, diagMarkerSpan(r, "Unexpected property 'MAX_RETRIES' for External Access Integration.", 4))
				}
			}
			continue
		}

		// ── Preamble: CREATE WAREHOUSE ────────────────────────────────────
		if reIsCreateWarehouse.MatchString(parseText) {
			if m := regexp.MustCompile(`(?i)WAREHOUSE\s+(` + _identPath + `)`).FindStringSubmatch(parseText); m != nil {
				if strings.Contains(m[1], ".") {
					markers = append(markers, diagMarkerSpan(r, "Warehouses are account-level objects and cannot have a database or schema prefix.", 4))
				}
			}
			validateProperties(parseText, whProps, r, &markers)
			continue
		}

		// ── Preamble: CREATE RESOURCE MONITOR ────────────────────────────
		if reIsCreateResourceMonitor.MatchString(parseText) {
			validateProperties(parseText, rmProps, r, &markers)
			continue
		}

		// ── Preamble: CREATE STREAM ──────────────────────────────────────
		if reIsCreateStream.MatchString(parseText) {
			if regexp.MustCompile(`(?i)\bOR\s+REPLACE\b`).MatchString(parseText) &&
				regexp.MustCompile(`(?i)\bIF\s+NOT\s+EXISTS\b`).MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS modifiers.", 4))
				continue
			}

			if !reValidCreateStream.MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE STREAM statement.", 4))
			}
			continue
		}

		// ── Preamble: CREATE TASK ────────────────────────────────────────
		if reIsCreateTask.MatchString(parseText) {
			markers = append(markers, validateCreateTask(parseText, r)...)
			continue
		}

		// ── ALTER TASK ──────────────────────────────────────────────────
		if reIsAlterTask.MatchString(parseText) {
			markers = append(markers, validateAlterTask(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE ALERT ───────────────────────────────────────
		if reIsCreateAlert.MatchString(parseText) {
			markers = append(markers, validateCreateAlert(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE PIPE ────────────────────────────────────────
		if reIsCreatePipe.MatchString(parseText) {
			// 1. Conflict between OR REPLACE and IF NOT EXISTS
			if regexp.MustCompile(`(?i)\bOR\s+REPLACE\b`).MatchString(parseText) &&
				regexp.MustCompile(`(?i)\bIF\s+NOT\s+EXISTS\b`).MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE PIPE statement.", 4))
				continue
			}

			// 2. Mandatory AS COPY INTO
			asIdx := regexp.MustCompile(`(?i)\bAS\s+COPY\s+INTO\b`).FindStringIndex(parseText)
			if asIdx == nil {
				markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS COPY INTO clause in CREATE PIPE statement.", 4))
				continue
			}

			preamble := parseText[:asIdx[0]]
			// 3. Property validation
			validateProperties(preamble, pipeProps, r, &markers)

			// 4. AWS_SNS_TOPIC requires AUTO_INGEST = TRUE
			if regexp.MustCompile(`(?i)\bAWS_SNS_TOPIC\s*=`).MatchString(preamble) {
				if !regexp.MustCompile(`(?i)\bAUTO_INGEST\s*=\s*TRUE\b`).MatchString(preamble) {
					markers = append(markers, diagMarkerSpan(r, "AWS_SNS_TOPIC is only meaningful when AUTO_INGEST = TRUE.", 4))
				}
			}

			// 5. Warning for AUTO_INGEST = TRUE without stage source
			if regexp.MustCompile(`(?i)\bAUTO_INGEST\s*=\s*TRUE\b`).MatchString(preamble) {
				copyBody := parseText[asIdx[0]:]
				if !regexp.MustCompile(`(?i)\bFROM\s+@`).MatchString(copyBody) {
					markers = append(markers, diagMarkerSpan(r, "AUTO_INGEST = TRUE typically requires a stage source (FROM @stage).", 4))
				}
			}

			continue
		}

		// ── Preamble: CREATE FUNCTION ─────────────────────────────────────────
		if reIsCreateFunction.MatchString(parseText) {
			asBodyIdx := -1
			for _, loc := range regexp.MustCompile(`(?i)\bAS\b`).FindAllStringIndex(parseText, -1) {
				prefix := parseText[:loc[0]]
				if regexp.MustCompile(`(?i)\bEXECUTE\s+$`).MatchString(prefix) {
					continue
				}
				asBodyIdx = loc[0]
				break
			}

			preamble := parseText
			if asBodyIdx == -1 {
				markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS clause in CREATE FUNCTION statement.", 4))
			} else {
				preamble = parseText[:asBodyIdx]
			}

			isAggregate := regexp.MustCompile(`(?i)\bAGGREGATE\s+FUNCTION\b`).MatchString(preamble)
			isSecure := regexp.MustCompile(`(?i)\bSECURE\s+(?:AGGREGATE\s+)?FUNCTION\b`).MatchString(preamble)

			if isSecure && isAggregate {
				markers = append(markers, diagMarkerSpan(r, "SECURE is not supported for AGGREGATE functions.", 4))
			}

			// 1. Mandatory RETURNS
			returnsMatch := regexp.MustCompile(`(?i)\bRETURNS\s+(TABLE)?`).FindStringSubmatch(preamble)
			if returnsMatch == nil {
				markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS clause in CREATE FUNCTION statement.", 4))
			} else {
				isTable := strings.ToUpper(returnsMatch[1]) == "TABLE"
				if isAggregate && isTable {
					markers = append(markers, diagMarkerSpan(r, "AGGREGATE functions cannot return a TABLE.", 4))
				}
			}

			// 2. Mandatory LANGUAGE
			langMatch := regexp.MustCompile(`(?i)\bLANGUAGE\s+([a-zA-Z0-9_]+)\b`).FindStringSubmatch(preamble)
			if langMatch == nil {
				markers = append(markers, diagMarkerSpan(r, "Missing mandatory LANGUAGE clause in CREATE FUNCTION statement.", 4))
			} else {
				lang := strings.ToUpper(langMatch[1])
				switch lang {
				case "JAVASCRIPT", "PYTHON", "JAVA", "SCALA", "SQL":
					// valid
				default:
					markers = append(markers, diagMarkerSpan(r, "Unknown or unsupported LANGUAGE '"+langMatch[1]+"' in CREATE FUNCTION.", 4))
				}

				// Python specific checks
				if lang == "PYTHON" {
					if !regexp.MustCompile(`(?i)\bRUNTIME_VERSION\b`).MatchString(preamble) {
						markers = append(markers, diagMarkerSpan(r, "RUNTIME_VERSION is required for PYTHON functions.", 4))
					}
					if !regexp.MustCompile(`(?i)\bHANDLER\b`).MatchString(preamble) {
						markers = append(markers, diagMarkerSpan(r, "HANDLER is required for PYTHON functions.", 4))
					}
					hasPackages := regexp.MustCompile(`(?i)\bPACKAGES\b`).MatchString(preamble)
					hasImports := regexp.MustCompile(`(?i)\bIMPORTS\b`).MatchString(preamble)
					if !hasPackages && !hasImports {
						markers = append(markers, diagMarkerSpan(r, "PACKAGES or IMPORTS is required for PYTHON functions.", 4))
					}
				}

				// Java / Scala specific checks
				if lang == "JAVA" || lang == "SCALA" {
					if !regexp.MustCompile(`(?i)\bHANDLER\b`).MatchString(preamble) {
						markers = append(markers, diagMarkerSpan(r, "HANDLER is required for "+lang+" functions.", 4))
					}
				}
			}

			// 4. Null input handling: mutually exclusive / redundant
			hasCalledOnNull := regexp.MustCompile(`(?i)\bCALLED\s+ON\s+NULL\s+INPUT\b`).MatchString(preamble)
			hasReturnsNull := regexp.MustCompile(`(?i)\bRETURNS\s+NULL\s+ON\s+NULL\s+INPUT\b`).MatchString(preamble)
			hasStrict := regexp.MustCompile(`(?i)\bSTRICT\b`).MatchString(preamble)

			if hasCalledOnNull && (hasReturnsNull || hasStrict) {
				markers = append(markers, diagMarkerSpan(r, "CALLED ON NULL INPUT and RETURNS NULL ON NULL INPUT (or STRICT) are mutually exclusive.", 4))
			}
			if hasReturnsNull && hasStrict {
				markers = append(markers, diagMarkerSpan(r, "STRICT and RETURNS NULL ON NULL INPUT are redundant.", 4))
			}

			// MEMOIZABLE
			if regexp.MustCompile(`(?i)\bMEMOIZABLE\b`).MatchString(preamble) {
				returnsMatch := regexp.MustCompile(`(?i)\bRETURNS\s+(TABLE)?`).FindStringSubmatch(preamble)
				isTable := returnsMatch != nil && strings.ToUpper(returnsMatch[1]) == "TABLE"
				if isAggregate || isTable {
					markers = append(markers, diagMarkerSpan(r, "MEMOIZABLE is only valid for scalar functions.", 4))
				}
			}

			continue
		}
		if reIsCreateProcedure.MatchString(parseText) {
			asBodyIdx := -1
			for _, loc := range regexp.MustCompile(`(?i)\bAS\b`).FindAllStringIndex(parseText, -1) {
				prefix := parseText[:loc[0]]
				if regexp.MustCompile(`(?i)\bEXECUTE\s+$`).MatchString(prefix) {
					continue
				}
				asBodyIdx = loc[0]
				break
			}

			preamble := parseText
			if asBodyIdx == -1 {
				markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS clause in CREATE PROCEDURE statement.", 4))
			} else {
				preamble = parseText[:asBodyIdx]
			}

			// 1. Mandatory RETURNS
			if !regexp.MustCompile(`(?i)\bRETURNS\b`).MatchString(preamble) {
				markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS clause in CREATE PROCEDURE statement.", 4))
			}

			// 2. Mandatory LANGUAGE
			langMatch := regexp.MustCompile(`(?i)\bLANGUAGE\s+([a-zA-Z0-9_]+)\b`).FindStringSubmatch(preamble)
			if langMatch == nil {
				markers = append(markers, diagMarkerSpan(r, "Missing mandatory LANGUAGE clause in CREATE PROCEDURE statement.", 4))
			} else {
				lang := strings.ToUpper(langMatch[1])
				switch lang {
				case "JAVASCRIPT", "PYTHON", "JAVA", "SCALA", "SQL":
					// valid
				default:
					markers = append(markers, diagMarkerSpan(r, "Unknown or unsupported LANGUAGE '"+langMatch[1]+"' in CREATE PROCEDURE.", 4))
				}

				// Python specific checks
				if lang == "PYTHON" {
					if !regexp.MustCompile(`(?i)\bRUNTIME_VERSION\b`).MatchString(preamble) {
						markers = append(markers, diagMarkerSpan(r, "RUNTIME_VERSION is required for PYTHON procedures.", 4))
					}
					hasPackages := regexp.MustCompile(`(?i)\bPACKAGES\b`).MatchString(preamble)
					hasImports := regexp.MustCompile(`(?i)\bIMPORTS\b`).MatchString(preamble)
					if !hasPackages && !hasImports {
						markers = append(markers, diagMarkerSpan(r, "PACKAGES or IMPORTS is required for PYTHON procedures.", 4))
					}
				}
			}

			// 4. Null input handling: mutually exclusive / redundant
			hasCalledOnNull := regexp.MustCompile(`(?i)\bCALLED\s+ON\s+NULL\s+INPUT\b`).MatchString(preamble)
			hasReturnsNull := regexp.MustCompile(`(?i)\bRETURNS\s+NULL\s+ON\s+NULL\s+INPUT\b`).MatchString(preamble)
			hasStrict := regexp.MustCompile(`(?i)\bSTRICT\b`).MatchString(preamble)

			if hasCalledOnNull && (hasReturnsNull || hasStrict) {
				markers = append(markers, diagMarkerSpan(r, "CALLED ON NULL INPUT and RETURNS NULL ON NULL INPUT (or STRICT) are mutually exclusive.", 4))
			}
			if hasReturnsNull && hasStrict {
				markers = append(markers, diagMarkerSpan(r, "STRICT and RETURNS NULL ON NULL INPUT are redundant.", 4))
			}

			// 5. EXECUTE AS
			if execAsMatch := regexp.MustCompile(`(?i)\bEXECUTE\s+AS\s+([a-zA-Z0-9_]+)\b`).FindStringSubmatch(preamble); execAsMatch != nil {
				execVal := strings.ToUpper(execAsMatch[1])
				if execVal != "CALLER" && execVal != "OWNER" {
					markers = append(markers, diagMarkerSpan(r, "EXECUTE AS must be CALLER or OWNER.", 4))
				}
			}

			continue
		}

		// ── Preamble: CREATE USER ────────────────────────────────────────
		if reIsCreateUser.MatchString(parseText) {
			if m := regexp.MustCompile(`(?i)USER\s+(` + _identPath + `)`).FindStringSubmatch(parseText); m != nil {
				if strings.Contains(m[1], ".") {
					markers = append(markers, diagMarkerSpan(r, "Users are account-level objects and cannot have a database or schema prefix.", 4))
				}
			}
			validateProperties(parseText, userProps, r, &markers)
			continue
		}

		// ── Preamble: CREATE ROLE ────────────────────────────────────────
		if reIsCreateRole.MatchString(parseText) {
			if m := regexp.MustCompile(`(?i)ROLE\s+(` + _identPath + `)`).FindStringSubmatch(parseText); m != nil {
				if strings.Contains(m[1], ".") {
					markers = append(markers, diagMarkerSpan(r, "Roles are account-level objects and cannot have a database or schema prefix.", 4))
				}
			}
			continue
		}

		// ── Preamble: CREATE MASKING POLICY ──────────────────────────────
		if reIsCreateMaskingPolicy.MatchString(parseText) {
			if !regexp.MustCompile(`(?i)\bRETURNS\b`).MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Missing RETURNS clause in Masking Policy definition.", 4))
			}
			continue
		}

		// ── Preamble: CREATE NETWORK POLICY ──────────────────────────────
		if reIsCreateNetworkPolicy.MatchString(parseText) {
			markers = append(markers, validateCreateNetworkPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE SESSION POLICY ──────────────────────────────
		if reIsCreateSessionPolicy.MatchString(parseText) {
			markers = append(markers, validateCreateSessionPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE PASSWORD POLICY ─────────────────────────────
		if reIsCreatePasswordPolicy.MatchString(parseText) {
			markers = append(markers, validateCreatePasswordPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE ROW ACCESS POLICY ───────────────────────────
		if reIsCreateRowAccessPolicy.MatchString(parseText) {
			markers = append(markers, validateCreateRowAccessPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE AGGREGATION POLICY ─────────────────────────
		if reIsCreateAggregationPolicy.MatchString(parseText) {
			markers = append(markers, validateCreateAggregationPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE PROJECTION POLICY ──────────────────────────
		if reIsCreateProjectionPolicy.MatchString(parseText) {
			markers = append(markers, validateCreateProjectionPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: ALTER AGGREGATION POLICY ──────────────────────────
		if reIsAlterAggregationPolicy.MatchString(parseText) {
			markers = append(markers, validateAlterAggregationOrProjectionPolicy(parseText, r, "AGGREGATION")...)
			continue
		}

		// ── Preamble: ALTER PROJECTION POLICY ───────────────────────────
		if reIsAlterProjectionPolicy.MatchString(parseText) {
			markers = append(markers, validateAlterAggregationOrProjectionPolicy(parseText, r, "PROJECTION")...)
			continue
		}

		// ── Preamble: DROP AGGREGATION POLICY ───────────────────────────
		if reIsDropAggregationPolicy.MatchString(parseText) {
			markers = append(markers, validateDropAggregationOrProjectionPolicy(parseText, r, "AGGREGATION")...)
			continue
		}

		// ── Preamble: DROP PROJECTION POLICY ────────────────────────────
		if reIsDropProjectionPolicy.MatchString(parseText) {
			markers = append(markers, validateDropAggregationOrProjectionPolicy(parseText, r, "PROJECTION")...)
			continue
		}

		// ── Preamble: CREATE PACKAGES POLICY ────────────────────────────
		if reIsCreatePackagesPolicy.MatchString(parseText) {
			markers = append(markers, validateCreatePackagesPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: ALTER PACKAGES POLICY ─────────────────────────────
		if reIsAlterPackagesPolicy.MatchString(parseText) {
			markers = append(markers, validateAlterPackagesPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: DROP PACKAGES POLICY ──────────────────────────────
		if reIsDropPackagesPolicy.MatchString(parseText) {
			markers = append(markers, validateDropPackagesPolicy(parseText, r)...)
			continue
		}

		// ── Preamble: CREATE STAGE ───────────────────────────────────────
		// stripParenContents removes nested KEY=VALUE pairs inside blocks
		// like FILE_FORMAT=(...), ENCRYPTION=(...), DIRECTORY=(...) before
		// property validation, preventing false positives like "Unexpected
		// property 'TYPE'" for FILE_FORMAT=(TYPE='CSV' ...).
		if reIsCreateStage.MatchString(parseText) {
			validateProperties(stripParenContents(parseText), stageProps, r, &markers)
			continue
		}

		// ── Preamble: CREATE FILE FORMAT ─────────────────────────────────
		if reIsCreateFileFormat.MatchString(parseText) {
			markers = append(markers, validateCreateFileFormat(parseText, r)...)
			continue
		}

		// ── Preamble: ALTER STAGE ─────────────────────────────────────────
		// RENAME TO, UNSET TAG, SET TAG, and UNSET DCM forms carry dynamic
		// identifiers (new name, tag names) that cannot be property-validated.
		// All other forms (SET FILE_FORMAT=..., SET COMMENT=..., REFRESH, etc.)
		// are validated for top-level property keys after stripping nested parens.
		if reIsAlterStage.MatchString(parseText) {
			if !reAlterStageNoValidate.MatchString(parseText) {
				validateProperties(stripParenContents(parseText), alterStageProps, r, &markers)
			}
			continue
		}

		// ── WITH ... AS PROCEDURE (anonymous procedure) ───────────────────
		if reWithProcAlias.MatchString(parseText) {
			markers = append(markers, validateWithProcedureCall(parseText, r)...)
			continue
		}

		// ── CALL ─────────────────────────────────────────────────────────
		if reIsCall.MatchString(parseText) {
			markers = append(markers, validateCall(parseText, r)...)
			continue
		}

		// ── GRANT ────────────────────────────────────────────────────────
		if reIsGrant.MatchString(parseText) {
			markers = append(markers, validateGrant(parseText, r)...)
			continue
		}

		// ── REVOKE ───────────────────────────────────────────────────────
		if reIsRevoke.MatchString(parseText) {
			markers = append(markers, validateRevoke(parseText, r)...)
			continue
		}

		// ── EXECUTE IMMEDIATE ─────────────────────────────────────────────
		if reIsExecuteImmediate.MatchString(parseText) {
			markers = append(markers, validateExecuteImmediate(parseText, r)...)
			continue
		}

		// ── EXECUTE TASK ──────────────────────────────────────────────────
		if reIsExecuteTask.MatchString(parseText) {
			markers = append(markers, validateExecuteTask(parseText, r)...)
			continue
		}

		// ── Other EXECUTE forms (EXECUTE ALERT, etc.) — pass through ─────
		if reIsExecute.MatchString(parseText) {
			continue
		}

		// ── PUT ───────────────────────────────────────────────────────────
		if reIsPut.MatchString(parseText) {
			markers = append(markers, validatePut(parseText, r)...)
			continue
		}

		// ── GET ───────────────────────────────────────────────────────────
		if reIsGet.MatchString(parseText) {
			markers = append(markers, validateGet(parseText, r)...)
			continue
		}

		// ── LIST / LS ─────────────────────────────────────────────────────
		if reIsList.MatchString(parseText) {
			markers = append(markers, validateList(parseText, r)...)
			continue
		}

		// ── REMOVE / RM ───────────────────────────────────────────────────
		if reIsRemove.MatchString(parseText) {
			markers = append(markers, validateRemove(parseText, r)...)
			continue
		}

		// ── CREATE SHARE ─────────────────────────────────────────────────
		if reIsCreateShare.MatchString(parseText) {
			markers = append(markers, validateCreateShare(parseText, r)...)
			continue
		}

		// ── ALTER SHARE ──────────────────────────────────────────────────
		if reIsAlterShare.MatchString(parseText) {
			markers = append(markers, validateAlterShare(parseText, r)...)
			continue
		}

		// ── CREATE TAG ──────────────────────────────────────────────────
		if reIsCreateTag.MatchString(parseText) {
			markers = append(markers, validateCreateTag(parseText, r)...)
			continue
		}

		// ── ALTER TAG ───────────────────────────────────────────────────
		if reIsAlterTag.MatchString(parseText) {
			markers = append(markers, validateAlterTag(parseText, r)...)
			continue
		}

		// ── DROP TAG ────────────────────────────────────────────────────
		if reIsDropTag.MatchString(parseText) {
			markers = append(markers, validateDropTag(parseText, r)...)
			continue
		}

		// ── ALTER SESSION ────────────────────────────────────────────────
		if reIsAlterSession.MatchString(parseText) {
			markers = append(markers, validateAlterSession(parseText, r)...)
			continue
		}

		// ── CREATE EVENT TABLE ───────────────────────────────────────────
		if reIsCreateEventTable.MatchString(parseText) {
			markers = append(markers, validateCreateEventTable(parseText, r)...)
			continue
		}

		// ── CREATE EXTERNAL VOLUME ────────────────────────────────────────
		if reIsCreateExternalVolume.MatchString(parseText) {
			markers = append(markers, validateCreateExternalVolume(parseText, r)...)
			continue
		}

		// ── USE ROLE ─────────────────────────────────────────────────────
		if reIsUseRole.MatchString(parseText) {
			markers = append(markers, validateUseRole(parseText, r)...)
			continue
		}

		// ── USE WAREHOUSE ────────────────────────────────────────────────
		if reIsUseWarehouse.MatchString(parseText) {
			markers = append(markers, validateUseWarehouse(parseText, r)...)
			continue
		}

		// ── USE SECONDARY ROLES ──────────────────────────────────────────
		if reIsUseSecondaryRoles.MatchString(parseText) {
			markers = append(markers, validateUseSecondaryRoles(parseText, r)...)
			continue
		}

		// ── Other USE variants (DATABASE, SCHEMA, bare USE <name>) ───────
		// Valid session commands that don't need pattern validation here;
		// existence checks are handled separately in ValidateTablesExist.
		if firstTok == "USE" {
			continue
		}

		// ── DESCRIBE / DESC ──────────────────────────────────────────────
		if reIsDescribe.MatchString(parseText) {
			markers = append(markers, validateDescribe(parseText, r)...)
			continue
		}

		// ── SHOW (intercepted before the node-sql-parser) ───────────────
		if reIsShow.MatchString(parseText) {
			markers = append(markers, validateShow(parseText, r)...)
			continue
		}

		// ── BEGIN (transaction) ─────────────────────────────────────────
		if reIsBegin.MatchString(parseText) {
			// Skip anonymous scripting blocks (BEGIN followed by LET, IF, etc.).
			// GetStatementRanges splits on semicolons, so the first "statement"
			// of an anonymous block looks like "BEGIN\n  LET x := 1".
			beginStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			if reBeginScripting.MatchString(beginStripped) {
				continue // scripting block, not a transaction
			}
			markers = append(markers, validateBeginStripped(beginStripped, r)...)
			if txnDepth > 0 {
				markers = append(markers, diagMarkerSpan(r,
					"Snowflake does not support nested BEGIN. A transaction is already open.", 4))
				// Don't increment txnDepth — Snowflake rejects nested BEGIN,
				// so we keep tracking only the original transaction.
			} else {
				txnBeginRange = r
				txnDepth++
			}
			continue
		}

		// ── COMMIT ──────────────────────────────────────────────────────
		if reIsCommit.MatchString(parseText) {
			commitStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			markers = append(markers, validateCommitStripped(commitStripped, r)...)
			if txnDepth == 0 {
				markers = append(markers, diagMarkerSpan(r,
					"COMMIT with no open transaction. Add BEGIN before COMMIT.", 4))
			} else {
				txnDepth--
			}
			continue
		}

		// ── ROLLBACK ────────────────────────────────────────────────────
		if reIsRollback.MatchString(parseText) {
			// Strip comments once and reuse for both validation and block-level tracking.
			rollbackStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			markers = append(markers, validateRollbackStripped(rollbackStripped, r)...)
			// ROLLBACK TO SAVEPOINT does NOT end the transaction — only bare
			// ROLLBACK / ROLLBACK WORK closes it.
			isToSavepoint := reRollbackToSavepointFull.MatchString(rollbackStripped)
			if !isToSavepoint {
				if txnDepth == 0 {
					markers = append(markers, diagMarkerSpan(r,
						"ROLLBACK with no open transaction. Add BEGIN before ROLLBACK.", 4))
				} else {
					txnDepth--
				}
			}
			continue
		}

		// ── SAVEPOINT ───────────────────────────────────────────────────
		if reIsSavepoint.MatchString(parseText) {
			spStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			markers = append(markers, validateSavepointStripped(spStripped, r)...)
			continue
		}

		// ── RELEASE SAVEPOINT ───────────────────────────────────────────
		if reIsReleaseSavepoint.MatchString(parseText) {
			relStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			markers = append(markers, validateReleaseSavepointStripped(relStripped, r)...)
			continue
		}

		// ── Bare RELEASE (without SAVEPOINT keyword) ────────────────────
		if firstTok == "RELEASE" {
			markers = append(markers, diagMarkerSpan(r,
				"RELEASE requires SAVEPOINT keyword. Use RELEASE SAVEPOINT <name>.", 4))
			continue
		}

		// ── Skip Snowflake false-positive statements ──────────────────────
		// (statements with Snowflake-specific syntax that the parser can't
		// handle; we emit no error for these)
		checkText := regexp.MustCompile(`(?i)\bCLUSTER\s+BY\s*\([^)]+\)`).
			ReplaceAllString(stripped, "")
		if reSnowflakeFP.MatchString(checkText) {
			continue
		}
		// Generic SELECT/INSERT/UPDATE/WITH: no additional checks here.
		// ValidateSyntax (the tokenizer) already covers them.
	}

	// ── Post-loop: unclosed transaction check ────────────────────────────
	if txnDepth > 0 {
		markers = append(markers, diagMarkerSpan(txnBeginRange,
			"Transaction not committed or rolled back. Add COMMIT or ROLLBACK before the end of the script.", 4))
	}

	return markers
}

// findMatchingParen finds the index of the closing ')' that matches the opening
// '(' at position 0 of s, handling string literals and depth.
// Returns -1 if not found or s doesn't start with '('.
func findMatchingParen(s string) int {
	if len(s) == 0 || s[0] != '(' {
		return -1
	}
	depth := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return i
				}
			}
		}
	}
	return -1
}

// ── ValidateDataTypes ─────────────────────────────────────────────────────────

var (
	reCastShorthand  = regexp.MustCompile(`::\s*([a-zA-Z_][a-zA-Z0-9_]*)`)
	reCastFunction   = regexp.MustCompile(`(?i)\b(?:TRY_)?CAST\s*\([\s\S]+?\bAS\s+([a-zA-Z_][a-zA-Z0-9_]+)`)
	reAlterTableAdd  = regexp.MustCompile(`(?i)\bALTER\s+TABLE\s+` + _identPath + `\s+ADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?` + _ident + `\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	reCreateTableExt = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE|TRANSIENT)\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath + `\s*\(`)
	reCreateProcExt  = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?PROCEDURE\s+` + _identPath + `\s*\(`)
	reCreateFuncExt  = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:TEMPORARY\s+|TEMP\s+)?(?:AGGREGATE\s+)?FUNCTION\s+` + _identPath + `\s*\(`)
	reReturnsType    = regexp.MustCompile(`(?i)\bRETURNS\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s*\([^)]*\))?\b`)
)

// ValidateDataTypes checks that explicit data type declarations within
// CREATE TABLE, ALTER TABLE, and CAST() functions exist in Snowflake's registry.
func ValidateDataTypes(sql string, stmtRanges []StatementRange) []DiagMarker {
	var markers []DiagMarker

	validTypes := make(map[string]bool)
	for _, dt := range sf.AllDataTypes() {
		validTypes[strings.ToUpper(dt.Name)] = true
	}

	offsetToLineCol := func(offset int) (int, int) {
		line, col := 1, 1
		for i := 0; i < offset && i < len(sql); i++ {
			if sql[i] == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
		return line, col
	}

	checkType := func(typeName string, typeOffset int) {
		up := strings.ToUpper(typeName)
		if !validTypes[up] {
			line, col := offsetToLineCol(typeOffset)
			markers = append(markers, DiagMarker{
				StartLineNumber: line,
				StartColumn:     col,
				EndLineNumber:   line,
				EndColumn:       col + len(typeName),
				Message:         fmt.Sprintf("Unknown data type '%s'", up),
				Severity:        4, // Warning
			})
		}
	}

	for _, r := range stmtRanges {
		rawText := sqlStmt(sql, r)
		stmtOffset := r.StartOffset

		// 1. Shorthand cast (::)
		for _, m := range reCastShorthand.FindAllStringSubmatchIndex(rawText, -1) {
			if len(m) >= 4 && m[2] != -1 {
				typeName := rawText[m[2]:m[3]]
				checkType(typeName, stmtOffset+m[2])
			}
		}

		// 2. CAST / TRY_CAST function
		for _, m := range reCastFunction.FindAllStringSubmatchIndex(rawText, -1) {
			if len(m) >= 4 && m[2] != -1 {
				typeName := rawText[m[2]:m[3]]
				checkType(typeName, stmtOffset+m[2])
			}
		}

		// 3. ALTER TABLE ... ADD
		for _, m := range reAlterTableAdd.FindAllStringSubmatchIndex(rawText, -1) {
			if len(m) >= 4 && m[2] != -1 {
				typeName := rawText[m[2]:m[3]]
				checkType(typeName, stmtOffset+m[2])
			}
		}

		// 4. CREATE TABLE
		if m := reCreateTableExt.FindStringSubmatchIndex(rawText); m != nil {
			parenStart := m[1] - 1
			colsRaw := extractBalancedBlockPat(rawText, parenStart)
			if len(colsRaw) >= 2 {
				colsContent := colsRaw[1 : len(colsRaw)-1] // strip the outer ()
				contentOffset := stmtOffset + parenStart + 1

				parseColumnDefs(colsContent, contentOffset, checkType)
			}
		}

		// 5. CREATE PROCEDURE/FUNCTION (parameters and returns)
		if m := reCreateProcExt.FindStringSubmatchIndex(rawText); m != nil {
			parenStart := m[1] - 1
			colsRaw := extractBalancedBlockPat(rawText, parenStart)
			if len(colsRaw) >= 2 {
				colsContent := colsRaw[1 : len(colsRaw)-1]
				contentOffset := stmtOffset + parenStart + 1
				parseColumnDefs(colsContent, contentOffset, checkType)
			}
		} else if m := reCreateFuncExt.FindStringSubmatchIndex(rawText); m != nil {
			parenStart := m[1] - 1
			colsRaw := extractBalancedBlockPat(rawText, parenStart)
			if len(colsRaw) >= 2 {
				colsContent := colsRaw[1 : len(colsRaw)-1]
				contentOffset := stmtOffset + parenStart + 1
				parseColumnDefs(colsContent, contentOffset, checkType)
			}
		}

		// Check RETURNS type for any statement that has it (e.g. CREATE PROCEDURE / FUNCTION)
		if strings.Contains(strings.ToUpper(rawText), "CREATE") && strings.Contains(strings.ToUpper(rawText), "RETURNS") {
			for _, m := range reReturnsType.FindAllStringSubmatchIndex(rawText, -1) {
				if len(m) >= 4 && m[2] != -1 {
					typeName := rawText[m[2]:m[3]]
					// Ignore "NULL" in RETURNS NULL ON NULL INPUT and "TABLE" in RETURNS TABLE(...)
					if strings.ToUpper(typeName) != "NULL" && strings.ToUpper(typeName) != "TABLE" {
						checkType(typeName, stmtOffset+m[2])
					}
				}
			}
		}
	}

	return markers
}

// extractBalancedBlockPat returns the balanced substring starting at openIdx.
func extractBalancedBlockPat(s string, openIdx int) string {
	if openIdx < 0 || openIdx >= len(s) || s[openIdx] != '(' {
		return ""
	}
	depth := 0
	inSingle := false
	inDouble := false
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return s[openIdx : i+1]
				}
			}
		}
	}
	return ""
}

func parseColumnDefs(colsContent string, contentOffset int, onTypeFound func(string, int)) {
	depth := 0
	inSingle := false
	inDouble := false

	startIdx := 0
	for i := 0; i <= len(colsContent); i++ {
		var c byte
		if i < len(colsContent) {
			c = colsContent[i]
		} else {
			c = ',' // force end of last segment
		}

		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
			case ',':
				if depth == 0 {
					seg := colsContent[startIdx:i]
					processColumnDef(seg, contentOffset+startIdx, onTypeFound)
					startIdx = i + 1
				}
			}
		}
	}
}

func processColumnDef(seg string, segOffset int, onTypeFound func(string, int)) {
	reWord := regexp.MustCompile(`(?i)^[a-zA-Z_][a-zA-Z0-9_]*|"[^"]+"`)

	var tokens []struct {
		text   string
		offset int
	}

	i := 0
	for i < len(seg) {
		c := seg[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		// Strip line comments
		if c == '-' && i+1 < len(seg) && seg[i+1] == '-' {
			for i < len(seg) && seg[i] != '\n' {
				i++
			}
			continue
		}
		// Strip block comments
		if c == '/' && i+1 < len(seg) && seg[i+1] == '*' {
			i += 2
			for i < len(seg) {
				if i+1 < len(seg) && seg[i] == '*' && seg[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		if m := reWord.FindStringIndex(seg[i:]); m != nil {
			tokens = append(tokens, struct {
				text   string
				offset int
			}{seg[i : i+m[1]], segOffset + i})
			i += m[1]
			continue
		}

		i++ // skip over parens or unrecognized characters
	}

	// We need at least the column name and the datatype token
	if len(tokens) >= 2 {
		first := strings.ToUpper(tokens[0].text)
		// Ignore constraint definitions
		if first == "CONSTRAINT" || first == "PRIMARY" || first == "UNIQUE" || first == "FOREIGN" || first == "INDEX" || first == "CHECK" {
			return
		}
		typeToken := tokens[1]
		if !strings.HasPrefix(typeToken.text, `"`) {
			onTypeFound(typeToken.text, typeToken.offset)
		}
	}
}

// stripParenContents returns s with all content inside parentheses removed
// while keeping the parenthesis characters themselves.  String literals
// (single- or double-quoted) are tracked so that parentheses appearing inside
// quoted values are not counted as structural delimiters.
//
// Example:
//
//	"FILE_FORMAT = (TYPE = 'CSV' SKIP_HEADER = 1) COMMENT = 'x'"
//	→ "FILE_FORMAT = () COMMENT = 'x'"
//
// This prevents nested KEY=VALUE pairs inside blocks such as
// FILE_FORMAT=(...), ENCRYPTION=(...), CREDENTIALS=(...), or DIRECTORY=(...)
// from being falsely flagged as unexpected top-level properties.
func stripParenContents(s string) string {
	out := make([]byte, 0, len(s))
	depth := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case !inDouble && c == '\'':
			inSingle = !inSingle
			if depth == 0 {
				out = append(out, c)
			}
		case !inSingle && c == '"':
			inDouble = !inDouble
			if depth == 0 {
				out = append(out, c)
			}
		case inSingle || inDouble:
			if depth == 0 {
				out = append(out, c)
			}
		case c == '(':
			depth++
			out = append(out, c)
		case c == ')':
			depth--
			out = append(out, c)
		default:
			if depth == 0 {
				out = append(out, c)
			}
		}
	}
	return string(out)
}

// validateProperties scans s for words that look like property keys (KEY =)
// and checks if they match the pipe-separated list of validProps.
func validateProperties(s string, validProps string, r StatementRange, markers *[]DiagMarker) {
	reValid := regexp.MustCompile(`(?i)^(` + validProps + `)$`)

	strippedS := reStripStringLiterals.ReplaceAllString(s, "''")

	for _, m := range reProp.FindAllStringSubmatch(strippedS, -1) {
		key := m[1]
		if !reValid.MatchString(key) {
			*markers = append(*markers, diagMarkerSpan(r, fmt.Sprintf("Unexpected property '%s' in statement.", key), 4))
		}
	}
}

func validateCreateAlert(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	// 1. Mutually exclusive OR REPLACE and IF NOT EXISTS
	ifIdx := reAlertIfExists.FindStringIndex(parseText)

	preambleToCheck := parseText
	if ifIdx != nil {
		preambleToCheck = parseText[:ifIdx[0]]
	}

	if reOrReplace.MatchString(preambleToCheck) && reIfNotExists.MatchString(preambleToCheck) {
		markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE ALERT statement.", 4))
		return markers
	}

	// 2. Mandatory IF (EXISTS (...))
	if ifIdx == nil {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory IF (EXISTS (...)) clause in CREATE ALERT statement.", 4))
	}

	var preamble string
	var body string
	if ifIdx != nil {
		preamble = parseText[:ifIdx[0]]
		body = parseText[ifIdx[0]:]
	} else {
		// If IF (EXISTS ( is missing, consider the whole statement as preamble for other checks
		preamble = parseText
		body = ""
	}

	// 3. Mandatory THEN
	// body is empty when IF clause is absent; THEN check is skipped
	if body != "" && reAlertThen.FindStringIndex(stripParenContents(body)) == nil {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory THEN keyword in CREATE ALERT statement.", 4))
	}

	// 4. Mandatory WAREHOUSE
	if !reAlertWarehouse.MatchString(preamble) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory WAREHOUSE property in CREATE ALERT statement.", 4))
	}

	// 5. Mandatory SCHEDULE
	if !reAlertSchedule.MatchString(preamble) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory SCHEDULE property in CREATE ALERT statement.", 4))
	}

	// 6. Validate properties
	validateProperties(preamble, alertProps, r, &markers)

	return markers
}

func validateCreateNetworkPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. Account-level: name must not have a database or schema prefix.
	// sqlIdentPathHasDot is used so that a quoted identifier whose inner text
	// contains a dot (e.g. "my.policy") is not falsely flagged as a prefix.
	if m := reNetworkPolicyName.FindStringSubmatch(parseText); m != nil {
		if sqlIdentPathHasDot(m[1]) {
			markers = append(markers, diagMarkerSpan(r, "Network policies are account-level objects and cannot have a database or schema prefix.", 4))
		}
	}

	// 2. At least one of ALLOWED_IP_LIST or ALLOWED_NETWORK_RULE_LIST must be present
	// and non-empty. An empty list (e.g. ALLOWED_IP_LIST = ()) has no effect.
	// networkPolicyListHasEntries is used instead of a plain TrimSpace check so
	// that whitespace-only quoted entries like ('   ') are also treated as empty.
	allowedIPMatch := reNetworkPolicyHasAllowedIP.FindStringSubmatch(parseText)
	hasAllowedIP := allowedIPMatch != nil && networkPolicyListHasEntries(allowedIPMatch[1])
	allowedRulesMatch := reNetworkPolicyHasAllowedRules.FindStringSubmatch(parseText)
	hasAllowedRules := allowedRulesMatch != nil && networkPolicyListHasEntries(allowedRulesMatch[1])
	if !hasAllowedIP && !hasAllowedRules {
		markers = append(markers, diagMarkerSpan(r, "Network policy has no effect: at least one of ALLOWED_IP_LIST or ALLOWED_NETWORK_RULE_LIST must be specified and non-empty.", 4))
	}

	// 3. Validate IP lists and collect IPs for overlap check.
	var allowedIPs []string
	var blockedIPs []string
	for _, m := range reNetworkPolicyIPList.FindAllStringSubmatch(parseText, -1) {
		listKind := strings.ToUpper(m[1])
		listContent := m[2]
		for rawEntry := range strings.SplitSeq(listContent, ",") {
			entry := strings.TrimSpace(rawEntry)
			if entry == "" {
				continue
			}
			// Strip surrounding single quotes.
			if len(entry) >= 2 && entry[0] == '\'' && entry[len(entry)-1] == '\'' {
				entry = strings.TrimSpace(entry[1 : len(entry)-1])
			}
			if entry == "" {
				continue
			}
			if !isValidIPv4CIDR(entry) {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("Invalid IPv4 address or CIDR '%s' in %s. Expected an IPv4 address, optionally with a CIDR prefix (e.g. 192.168.0.0/24 or 10.0.0.1/32). IPv6 addresses must be added via ALLOWED_NETWORK_RULE_LIST.", entry, listKind),
					4))
				continue
			}
			if listKind == "ALLOWED_IP_LIST" {
				allowedIPs = append(allowedIPs, entry)
			} else {
				blockedIPs = append(blockedIPs, entry)
			}
		}
	}

	// 4. Warn if the same IP/CIDR string appears in both ALLOWED_IP_LIST and
	// BLOCKED_IP_LIST. Note: this is a string-exact comparison; semantic subnet
	// overlaps (e.g. 10.0.0.0/8 allowed vs 10.0.1.5 blocked) are not detected.
	if len(allowedIPs) > 0 && len(blockedIPs) > 0 {
		allowedSet := make(map[string]bool, len(allowedIPs))
		for _, ip := range allowedIPs {
			allowedSet[ip] = true
		}
		for _, ip := range blockedIPs {
			if allowedSet[ip] {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("IP '%s' appears in both ALLOWED_IP_LIST and BLOCKED_IP_LIST.", ip),
					4))
			}
		}
	}

	// 5. Validate top-level property keys (strip list contents to avoid false positives).
	validateProperties(stripParenContents(parseText), networkPolicyProps, r, &markers)

	return markers
}

// networkPolicyListHasEntries reports whether the raw paren content of a
// network policy list (e.g. the capture from ALLOWED_IP_LIST = (...)) contains
// at least one non-empty entry after stripping surrounding single quotes.
// This avoids the false-negative caused by whitespace-only quoted entries such
// as ('   ') which would otherwise pass a plain strings.TrimSpace != "" check.
func networkPolicyListHasEntries(content string) bool {
	for raw := range strings.SplitSeq(content, ",") {
		entry := strings.TrimSpace(raw)
		if len(entry) >= 2 && entry[0] == '\'' && entry[len(entry)-1] == '\'' {
			entry = strings.TrimSpace(entry[1 : len(entry)-1])
		}
		if entry != "" {
			return true
		}
	}
	return false
}

// isValidIPv4CIDR reports whether s is a valid IPv4 address, optionally
// followed by a CIDR prefix length (e.g. "192.168.0.0/24" or "10.0.0.1").
// IPv6 addresses are intentionally rejected: Snowflake's ALLOWED_IP_LIST and
// BLOCKED_IP_LIST only accept IPv4; IPv6 network rules must use ALLOWED_NETWORK_RULE_LIST.
func isValidIPv4CIDR(s string) bool {
	if strings.Contains(s, "/") {
		ip, _, err := net.ParseCIDR(s)
		return err == nil && ip.To4() != nil
	}
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}

// sqlIdentPathHasDot reports whether the SQL identifier path s contains a dot
// that acts as a namespace separator (e.g. "db.schema" or "db.schema.table").
// Dots that appear inside a double-quoted identifier token (e.g. "my.policy")
// are not counted, so a single-part quoted name never triggers a false positive.
func sqlIdentPathHasDot(s string) bool {
	inQuote := false
	for _, c := range s {
		switch c {
		case '"':
			inQuote = !inQuote
		case '.':
			if !inQuote {
				return true
			}
		}
	}
	return false
}

// validateCreateSessionPolicy checks structural requirements for a
// CREATE [OR REPLACE] SESSION POLICY statement.
func validateCreateSessionPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. Account-level: object name must not have a database or schema prefix.
	if m := reSessionPolicyName.FindStringSubmatch(parseText); m != nil {
		if sqlIdentPathHasDot(m[1]) {
			markers = append(markers, diagMarkerSpan(r, "Session policies are account-level objects and cannot have a database or schema prefix.", 4))
		}
	}

	// 2. Validate SESSION_IDLE_TIMEOUT_MINS range (0–56400).
	if m := reSessionIdleTimeout.FindStringSubmatch(parseText); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil && (v < 0 || v > 56400) {
			markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("SESSION_IDLE_TIMEOUT_MINS value %d is out of range (0–56400). Use 0 to disable the timeout.", v), 4))
		}
	}

	// 3. Validate SESSION_UI_IDLE_TIMEOUT_MINS range (0–56400).
	if m := reSessionUIIdleTimeout.FindStringSubmatch(parseText); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil && (v < 0 || v > 56400) {
			markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("SESSION_UI_IDLE_TIMEOUT_MINS value %d is out of range (0–56400). Use 0 to disable the timeout.", v), 4))
		}
	}

	// 4. Validate property keys.
	validateProperties(parseText, sessionPolicyProps, r, &markers)

	return markers
}

// validateCreatePasswordPolicy checks structural requirements for a
// CREATE [OR REPLACE] PASSWORD POLICY statement.
func validateCreatePasswordPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. Account-level: object name must not have a database or schema prefix.
	if m := rePasswordPolicyName.FindStringSubmatch(parseText); m != nil {
		if sqlIdentPathHasDot(m[1]) {
			markers = append(markers, diagMarkerSpan(r, "Password policies are account-level objects and cannot have a database or schema prefix.", 4))
		}
	}

	// 2. Per-property range validation.
	type intProp struct {
		re   *regexp.Regexp
		name string
		min  int
		max  int // -1 means no upper bound
	}

	props := []intProp{
		{rePasswordMinLength, "PASSWORD_MIN_LENGTH", 8, 256},
		{rePasswordMaxLength, "PASSWORD_MAX_LENGTH", 8, 256},
		{rePasswordMinUpperCase, "PASSWORD_MIN_UPPER_CASE_CHARS", 0, 256},
		{rePasswordMinLowerCase, "PASSWORD_MIN_LOWER_CASE_CHARS", 0, 256},
		{rePasswordMinNumeric, "PASSWORD_MIN_NUMERIC_CHARS", 0, 256},
		{rePasswordMinSpecial, "PASSWORD_MIN_SPECIAL_CHARS", 0, 256},
		{rePasswordMinAgeDays, "PASSWORD_MIN_AGE_DAYS", 0, 999},
		{rePasswordMaxAgeDays, "PASSWORD_MAX_AGE_DAYS", 0, 999},
		{rePasswordMaxRetries, "PASSWORD_MAX_RETRIES", 1, 10},
		{rePasswordLockoutTimeMins, "PASSWORD_LOCKOUT_TIME_MINS", 1, 999},
		{rePasswordHistory, "PASSWORD_HISTORY", 0, 24},
	}

	values := make(map[string]int)
	for _, p := range props {
		m := p.re.FindStringSubmatch(parseText)
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		values[p.name] = v
		if v < p.min {
			msg := fmt.Sprintf("%s value %d is below the minimum (%d).", p.name, v, p.min)
			markers = append(markers, diagMarkerSpan(r, msg, 4))
		} else if p.max >= 0 && v > p.max {
			msg := fmt.Sprintf("%s value %d exceeds the maximum (%d).", p.name, v, p.max)
			markers = append(markers, diagMarkerSpan(r, msg, 4))
		}
	}

	// 3. Cross-property checks.
	minLen, hasMinLen := values["PASSWORD_MIN_LENGTH"]
	maxLen, hasMaxLen := values["PASSWORD_MAX_LENGTH"]
	if hasMinLen && hasMaxLen && maxLen < minLen {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("PASSWORD_MAX_LENGTH (%d) must be greater than or equal to PASSWORD_MIN_LENGTH (%d).", maxLen, minLen),
			4))
	}
	minAge, hasMinAge := values["PASSWORD_MIN_AGE_DAYS"]
	maxAge, hasMaxAge := values["PASSWORD_MAX_AGE_DAYS"]
	if hasMinAge && hasMaxAge && maxAge > 0 && minAge > maxAge {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("PASSWORD_MIN_AGE_DAYS (%d) must be less than or equal to PASSWORD_MAX_AGE_DAYS (%d).", minAge, maxAge),
			4))
	}

	// 4. Validate property keys.
	validateProperties(parseText, passwordPolicyProps, r, &markers)

	return markers
}

// validateCreateRowAccessPolicy checks structural requirements for a
// CREATE ROW ACCESS POLICY statement.
func validateCreateRowAccessPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	// Restrict the check to the preamble before AS (...) so that an IF in
	// the policy body expression is not mistaken for the DDL modifier.
	asIdx := reRowAccessPolicyASOpen.FindStringIndex(parseText)
	preamble := parseText
	if asIdx != nil {
		preamble = parseText[:asIdx[0]]
	}
	if reOrReplace.MatchString(preamble) && reIfNotExists.MatchString(preamble) {
		markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE ROW ACCESS POLICY statement.", 4))
		return markers
	}

	// 2. Mandatory AS (<arg_name> <arg_type> [, ...]) parameter list.
	paramMatch := reRowAccessPolicyParamList.FindStringSubmatch(parseText)
	if paramMatch == nil {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS (<arg_name> <arg_type> [, ...]) parameter list in CREATE ROW ACCESS POLICY.", 4))
	} else {
		paramContent := strings.TrimSpace(paramMatch[1])
		if paramContent == "" {
			markers = append(markers, diagMarkerSpan(r, "Row access policy parameter list must declare at least one argument.", 4))
		} else {
			// Validate each parameter's declared data type.
			validTypes := make(map[string]bool)
			for _, dt := range sf.AllDataTypes() {
				validTypes[strings.ToUpper(dt.Name)] = true
			}
			for _, param := range splitCommaRespectingParens(paramContent) {
				param = strings.TrimSpace(param)
				if param == "" {
					continue
				}
				fields := strings.Fields(param)
				if len(fields) >= 2 {
					rawType := fields[1]
					// Strip optional precision/scale parens: VARCHAR(256) → VARCHAR.
					if idx := strings.Index(rawType, "("); idx != -1 {
						rawType = rawType[:idx]
					}
					typeName := strings.ToUpper(rawType)
					if !validTypes[typeName] {
						markers = append(markers, diagMarkerSpan(r,
							fmt.Sprintf("Unknown data type '%s' in row access policy parameter.", typeName), 4))
					}
				}
			}
		}
	}

	// 3. Mandatory RETURNS BOOLEAN clause.
	if !reRowAccessPolicyReturns.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS BOOLEAN clause in CREATE ROW ACCESS POLICY.", 4))
	}

	// 4. Mandatory -> separator between signature and body.
	if !reRowAccessPolicyArrow.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory '->' separator between signature and body in CREATE ROW ACCESS POLICY.", 4))
	}

	return markers
}

// splitCommaRespectingParens splits s by commas that are not inside
// parentheses.  This correctly separates parameter declarations like
// "a VARCHAR, b NUMBER(10,2)" into ["a VARCHAR", " b NUMBER(10,2)"].
func splitCommaRespectingParens(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// ── validateCreateAggregationPolicy ───────────────────────────────────────────

// validateCreateAggregationPolicy checks structural requirements for a
// CREATE [OR REPLACE] AGGREGATION POLICY statement.
func validateCreateAggregationPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	asIdx := reAggPolicyAS.FindStringIndex(parseText)
	preamble := parseText
	if asIdx != nil {
		preamble = parseText[:asIdx[0]]
	}
	if reOrReplace.MatchString(preamble) && reIfNotExists.MatchString(preamble) {
		markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE AGGREGATION POLICY statement.", 4))
		return markers
	}

	// 2. Mandatory AS () parameter list.
	if !reAggPolicyAS.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS () clause in CREATE AGGREGATION POLICY.", 4))
	}

	// 3. Mandatory RETURNS AGGREGATION_CONSTRAINT clause.
	if !reAggPolicyReturns.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS AGGREGATION_CONSTRAINT clause in CREATE AGGREGATION POLICY.", 4))
	}

	// 4. Mandatory -> separator between signature and body.
	if !reAggPolicyArrow.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory '->' separator between signature and body in CREATE AGGREGATION POLICY.", 4))
	}

	// 5. Validate MIN_GROUP_SIZE range (1–1 000 000) if present.
	if m := reAggPolicyMinGroupSize.FindStringSubmatch(parseText); m != nil {
		val, err := strconv.Atoi(m[1])
		if err == nil && (val < 1 || val > 1000000) {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MIN_GROUP_SIZE (%d) must be between 1 and 1000000.", val), 4))
		}
	}

	return markers
}

// ── validateCreateProjectionPolicy ───────────────────────────────────────────

// validateCreateProjectionPolicy checks structural requirements for a
// CREATE [OR REPLACE] PROJECTION POLICY statement.
func validateCreateProjectionPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	asIdx := reProjPolicyAS.FindStringIndex(parseText)
	preamble := parseText
	if asIdx != nil {
		preamble = parseText[:asIdx[0]]
	}
	if reOrReplace.MatchString(preamble) && reIfNotExists.MatchString(preamble) {
		markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE PROJECTION POLICY statement.", 4))
		return markers
	}

	// 2. Mandatory AS () parameter list.
	if !reProjPolicyAS.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS () clause in CREATE PROJECTION POLICY.", 4))
	}

	// 3. Mandatory RETURNS PROJECTION_CONSTRAINT clause.
	if !reProjPolicyReturns.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS PROJECTION_CONSTRAINT clause in CREATE PROJECTION POLICY.", 4))
	}

	// 4. Mandatory -> separator between signature and body.
	if !reProjPolicyArrow.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory '->' separator between signature and body in CREATE PROJECTION POLICY.", 4))
	}

	// 5. Validate ALLOW value if present: must be 'none' or 'transformation'.
	if m := reProjPolicyAllowValue.FindStringSubmatch(parseText); m != nil {
		val := strings.ToLower(m[1])
		if val != "none" && val != "transformation" {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("ALLOW value '%s' is invalid; must be 'none' or 'transformation'.", m[1]), 4))
		}
	}

	return markers
}

// ── validateAlterAggregationOrProjectionPolicy ───────────────────────────────

// validateAlterAggregationOrProjectionPolicy checks structural requirements for
// ALTER AGGREGATION POLICY or ALTER PROJECTION POLICY statements.
func validateAlterAggregationOrProjectionPolicy(parseText string, r StatementRange, policyType string) []DiagMarker {
	var markers []DiagMarker

	if !reAlterPolicyAction.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("ALTER %s POLICY requires SET BODY, SET COMMENT, UNSET COMMENT, or RENAME TO.", policyType), 4))
	}

	return markers
}

// ── validateDropAggregationOrProjectionPolicy ────────────────────────────────

// validateDropAggregationOrProjectionPolicy checks structural requirements for
// DROP AGGREGATION POLICY or DROP PROJECTION POLICY statements.
func validateDropAggregationOrProjectionPolicy(parseText string, r StatementRange, policyType string) []DiagMarker {
	var markers []DiagMarker

	if !reDropPolicyHasName.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("DROP %s POLICY requires a policy name.", policyType), 4))
	}

	return markers
}

// ── validateCreatePackagesPolicy ─────────────────────────────────────────────

// validateCreatePackagesPolicy checks structural requirements for a
// CREATE [OR REPLACE] PACKAGES POLICY statement.
func validateCreatePackagesPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(parseText) && reIfNotExists.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE PACKAGES POLICY statement.", 4))
		return markers
	}

	// 2. LANGUAGE is mandatory and must be PYTHON.
	m := rePkgPolicyLanguage.FindStringSubmatch(parseText)
	if m == nil {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory LANGUAGE clause in CREATE PACKAGES POLICY. Only LANGUAGE PYTHON is supported.", 4))
	} else if strings.ToUpper(m[1]) != "PYTHON" {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("LANGUAGE '%s' is not supported for PACKAGES POLICY; only PYTHON is allowed.", m[1]), 4))
	}

	return markers
}

// ── validateAlterPackagesPolicy ──────────────────────────────────────────────

// validateAlterPackagesPolicy checks structural requirements for an
// ALTER PACKAGES POLICY statement.
func validateAlterPackagesPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// Must contain a valid action.
	if !reAlterPkgPolicyAction.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER PACKAGES POLICY requires SET ALLOWLIST, SET BLOCKLIST, SET ADDITIONAL_CREATION_BLOCKLIST, SET COMMENT, UNSET ALLOWLIST, UNSET BLOCKLIST, UNSET ADDITIONAL_CREATION_BLOCKLIST, or UNSET COMMENT.", 4))
	}

	return markers
}

// ── validateDropPackagesPolicy ───────────────────────────────────────────────

// validateDropPackagesPolicy checks structural requirements for a
// DROP PACKAGES POLICY statement.
func validateDropPackagesPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// Policy name is required.
	if !reDropPolicyHasName.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r, "DROP PACKAGES POLICY requires a policy name.", 4))
	}

	return markers
}

// ── validateGrant ─────────────────────────────────────────────────────────────

// validateGrant validates a GRANT statement for structural correctness and
// privilege/object-type compatibility.
func validateGrant(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// ── GRANT ROLE / GRANT DATABASE ROLE ─────────────────────────────────────
	isGrantRole := reIsGrantRole.MatchString(parseText) ||
		reIsGrantDatabaseRole.MatchString(parseText)
	if isGrantRole {
		// WITH GRANT OPTION is not valid for role grants.
		if reWithGrantOption.MatchString(parseText) {
			markers = append(markers, diagMarkerSpan(r,
				"WITH GRANT OPTION is not valid for GRANT ROLE statements.", 4))
		}
		// Role grants use TO USER or TO ROLE, never TO TABLE.
		if reGrantToTable.MatchString(parseText) {
			markers = append(markers, diagMarkerSpan(r,
				"Unexpected syntax: Roles can be granted to other roles or users, but not directly to tables.", 4))
		}
		// Must have a grantee.
		if !reGrantee.MatchString(parseText) {
			markers = append(markers, diagMarkerSpan(r,
				"GRANT ROLE requires a TO ROLE or TO USER clause.", 4))
		}
		return markers
	}

	// ── GRANT <privileges> ON <object_type> ───────────────────────────────────
	m := reGrantOnObject.FindStringSubmatch(parseText)
	if m == nil {
		// No recognizable ON clause — incomplete or unsupported form; skip.
		return markers
	}
	privListRaw := m[1]
	allFuture := strings.TrimSpace(strings.ToUpper(m[2])) // "ALL", "FUTURE", or ""
	objectType := normalizeGrantObjectType(m[3])

	// ── GRANT <priv> ON ROLE is not valid Snowflake syntax ────────────────────
	// The correct form for role assignment is "GRANT ROLE <name> TO ROLE/USER".
	// Snowflake does not support granting privileges on role objects via ON ROLE.
	if objectType == "ROLE" {
		markers = append(markers, diagMarkerSpan(r,
			"'GRANT <privilege> ON ROLE' is not valid Snowflake syntax. "+
				"Use 'GRANT ROLE <name> TO ROLE/USER' to assign a role.", 4))
		return markers
	}

	// ── Grantee required ──────────────────────────────────────────────────────
	if !reGrantee.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"GRANT statement requires a grantee (TO ROLE, TO DATABASE ROLE, or TO USER).", 4))
	}

	// ── ON ALL / ON FUTURE requires IN SCHEMA or IN DATABASE ─────────────────
	if reGrantAllFuture.MatchString(parseText) && !reGrantInQualifier.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"ON ALL/FUTURE <objects> requires an IN SCHEMA or IN DATABASE qualifier.", 4))
	}

	// ── Privilege validation for known object types ───────────────────────────
	// Bulk grants (ALL/FUTURE) are skipped — the privilege set may be
	// legitimately broad and varies by object type.
	validPrivs, knownObj := grantObjectPrivileges[objectType]
	if knownObj && allFuture == "" {
		for _, priv := range splitPrivileges(privListRaw) {
			// OWNERSHIP, ALL, and ALL PRIVILEGES are always accepted.
			if priv == "OWNERSHIP" || priv == "ALL" || priv == "ALL PRIVILEGES" {
				continue
			}
			if !slices.Contains(validPrivs, priv) {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("Privilege '%s' is not valid for object type %s.", priv, objectType), 4))
			}
		}
	}

	return markers
}

// ── validateRevoke ────────────────────────────────────────────────────────────

// validateRevoke validates a REVOKE statement for structural correctness and
// privilege/object-type compatibility.
func validateRevoke(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// ── REVOKE ROLE / REVOKE DATABASE ROLE ────────────────────────────────────
	isRevokeRole := reIsRevokeRole.MatchString(parseText) ||
		reIsRevokeDatabaseRole.MatchString(parseText)
	if isRevokeRole {
		if !reGranteeFrom.MatchString(parseText) {
			markers = append(markers, diagMarkerSpan(r,
				"REVOKE ROLE requires a FROM ROLE or FROM USER clause.", 4))
		}
		return markers
	}

	// ── REVOKE <privileges> ON <object_type> ──────────────────────────────────
	m := reRevokeOnObject.FindStringSubmatch(parseText)
	if m == nil {
		return markers
	}
	privListRaw := m[1]
	// allFuture is "" for plain object revokes and "ALL" or "FUTURE" for bulk
	// revokes. It gates privilege validation: bulk revokes are always skipped
	// because the full privilege set is determined dynamically by Snowflake.
	allFuture := strings.TrimSpace(strings.ToUpper(m[2]))
	objectType := normalizeGrantObjectType(m[3])

	// ── REVOKE <priv> ON ROLE is not valid Snowflake syntax ──────────────────
	// The correct form for role revocation is "REVOKE ROLE <name> FROM ROLE/USER".
	if objectType == "ROLE" {
		markers = append(markers, diagMarkerSpan(r,
			"'REVOKE <privilege> ON ROLE' is not valid Snowflake syntax. "+
				"Use 'REVOKE ROLE <name> FROM ROLE/USER' to revoke a role.", 4))
		return markers
	}

	// ── ON ALL / ON FUTURE requires IN SCHEMA or IN DATABASE ─────────────────
	if reGrantAllFuture.MatchString(parseText) && !reGrantInQualifier.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"ON ALL/FUTURE <objects> requires an IN SCHEMA or IN DATABASE qualifier.", 4))
	}

	// ── CASCADE and RESTRICT are mutually exclusive ───────────────────────────
	if reRevokeCascade.MatchString(parseText) && reRevokeRestrict.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"CASCADE and RESTRICT are mutually exclusive in REVOKE statement.", 4))
	}

	// ── FROM clause required ──────────────────────────────────────────────────
	if !reGranteeFrom.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"REVOKE statement requires a FROM ROLE, FROM DATABASE ROLE, or FROM USER clause.", 4))
	}

	// ── Privilege validation for known object types ───────────────────────────
	validPrivs, knownObj := grantObjectPrivileges[objectType]
	if knownObj && allFuture == "" {
		for _, priv := range splitPrivileges(privListRaw) {
			if priv == "OWNERSHIP" || priv == "ALL" || priv == "ALL PRIVILEGES" {
				continue
			}
			if !slices.Contains(validPrivs, priv) {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("Privilege '%s' is not valid for object type %s.", priv, objectType), 4))
			}
		}
	}

	return markers
}

// splitPrivileges splits a comma-separated privilege list into individual
// normalised (upper-cased, internal-whitespace-collapsed) privilege strings.
// strings.Fields is used so that "CREATE  TABLE" (double space) normalises to
// "CREATE TABLE" and matches the map keys in grantObjectPrivileges.
func splitPrivileges(privList string) []string {
	var result []string
	for p := range strings.SplitSeq(privList, ",") {
		// Collapse internal whitespace runs (tabs, double-spaces, etc.) as well
		// as leading/trailing whitespace before the equality check.
		trimmed := strings.Join(strings.Fields(strings.ToUpper(p)), " ")
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// normalizeGrantObjectType converts plural and/or alternative Snowflake object
// type names to their canonical singular upper-cased form used as map keys in
// grantObjectPrivileges.
func normalizeGrantObjectType(t string) string {
	upper := strings.ToUpper(strings.TrimSpace(t))
	if singular, ok := grantObjectTypePlurals[upper]; ok {
		return singular
	}
	return upper
}


func validateCopyInto(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	m := reCopyInto.FindStringSubmatch(parseText)
	if m == nil {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in COPY INTO statement.", 4))
		return markers
	}

	target := m[1]
	rest := strings.TrimSpace(parseText[len(m[0]):])

	// FROM clause is mandatory
	fromMatch := regexp.MustCompile(`(?i)^FROM\s+`).FindStringIndex(rest)
	if fromMatch == nil {
		markers = append(markers, diagMarkerSpan(r, "COPY INTO statement is missing the mandatory FROM clause.", 4))
		return markers
	}

	isUnloading := strings.HasPrefix(target, "@") || strings.HasPrefix(target, "'")
	// Extract properties (everything after the FROM source)
	restOfFrom := rest[fromMatch[1]:]
	var properties string
	if strings.HasPrefix(restOfFrom, "(") {
		endIdx := findMatchingParen(restOfFrom)
		if endIdx != -1 {
			properties = strings.TrimSpace(restOfFrom[endIdx+1:])
		} else {
			properties = restOfFrom
		}
	} else {
		// Find first word that looks like a property key (KEY =)
		propIdx := regexp.MustCompile(`(?i)\b[a-zA-Z_0-9]+\s*=`).FindStringIndex(restOfFrom)
		if propIdx != nil {
			properties = restOfFrom[propIdx[0]:]
		}
	}

	if !isUnloading {
		// Loading (table target)
		hasFiles := regexp.MustCompile(`(?i)\bFILES\s*=\s*`).MatchString(properties)
		hasPattern := regexp.MustCompile(`(?i)\bPATTERN\s*=\s*`).MatchString(properties)
		if hasFiles && hasPattern {
			markers = append(markers, diagMarkerSpan(r, "FILES and PATTERN are mutually exclusive in COPY INTO statement.", 4))
		}

		// FILE_FORMAT
		if regexp.MustCompile(`(?i)\bFILE_FORMAT\s*=\s*\(`).MatchString(properties) {
			ffInner := extractParenContent(properties, "FILE_FORMAT")
			hasFFName := regexp.MustCompile(`(?i)\bFORMAT_NAME\s*=\s*`).MatchString(ffInner)
			hasFFType := regexp.MustCompile(`(?i)\bTYPE\s*=\s*`).MatchString(ffInner)
			if hasFFName && hasFFType {
				markers = append(markers, diagMarkerSpan(r, "FORMAT_NAME and inline TYPE are mutually exclusive in FILE_FORMAT clause.", 4))
			}
			if hasFFType {
				if !regexp.MustCompile(`(?i)\bTYPE\s*=\s*(?:'?"?)(CSV|JSON|AVRO|ORC|PARQUET|XML)(?:'?"?)\b`).MatchString(ffInner) {
					markers = append(markers, diagMarkerSpan(r, "Invalid FILE_FORMAT TYPE. Must be CSV, JSON, AVRO, ORC, PARQUET, or XML.", 4))
				}
			}
		}

		// ON_ERROR
		if onErrorMatch := regexp.MustCompile(`(?i)\bON_ERROR\s*=\s*([a-zA-Z_0-9%]+)`).FindStringSubmatch(properties); onErrorMatch != nil {
			val := strings.ToUpper(onErrorMatch[1])
			if val != "CONTINUE" && val != "ABORT_STATEMENT" && val != "SKIP_FILE" &&
				!regexp.MustCompile(`^SKIP_FILE_\d+%?$`).MatchString(val) {
				markers = append(markers, diagMarkerSpan(r, "Invalid ON_ERROR value. Must be CONTINUE, SKIP_FILE, SKIP_FILE_<n>, SKIP_FILE_<n>%, or ABORT_STATEMENT.", 4))
			}
		}

		validateBoolProp(properties, "PURGE", r, &markers)
		validateBoolProp(properties, "FORCE", r, &markers)
		validateBoolProp(properties, "LOAD_UNCERTAIN_FILES", r, &markers)

		if matchMatch := regexp.MustCompile(`(?i)\bMATCH_BY_COLUMN_NAME\s*=\s*(\w+)\b`).FindStringSubmatch(properties); matchMatch != nil {
			val := strings.ToUpper(matchMatch[1])
			if val != "CASE_SENSITIVE" && val != "CASE_INSENSITIVE" && val != "NONE" {
				markers = append(markers, diagMarkerSpan(r, "Invalid MATCH_BY_COLUMN_NAME value. Must be CASE_SENSITIVE, CASE_INSENSITIVE, or NONE.", 4))
			}
		}
	} else {
		// Unloading (stage target)
		validateBoolProp(properties, "OVERWRITE", r, &markers)
		validateBoolProp(properties, "SINGLE", r, &markers)
		validateBoolProp(properties, "INCLUDE_QUERY_ID", r, &markers)
		validateBoolProp(properties, "DETAILED_OUTPUT", r, &markers)

		if mfsMatch := regexp.MustCompile(`(?i)\bMAX_FILE_SIZE\s*=\s*(\S+)\b`).FindStringSubmatch(properties); mfsMatch != nil {
			val := mfsMatch[1]
			if !regexp.MustCompile(`^\d+$`).MatchString(val) || val == "0" {
				markers = append(markers, diagMarkerSpan(r, "MAX_FILE_SIZE must be a positive integer.", 4))
			}
		}
	}

	return markers
}

// reBoolPropMap holds pre-compiled regexes for validateBoolProp. Each entry
// matches PROP = value where value is a word token. The map is built at init
// time so regexes are compiled once rather than on every call.
var reBoolPropMap = func() map[string]*regexp.Regexp {
	props := []string{"ALLOW_WRITES", "PURGE", "FORCE", "LOAD_UNCERTAIN_FILES", "OVERWRITE", "SINGLE", "INCLUDE_QUERY_ID", "DETAILED_OUTPUT"}
	m := make(map[string]*regexp.Regexp, len(props))
	for _, p := range props {
		m[p] = regexp.MustCompile(`(?i)\b` + p + `\s*=\s*(\w+)\b`)
	}
	return m
}()

func validateBoolProp(s string, prop string, r StatementRange, markers *[]DiagMarker) {
	re := reBoolPropMap[prop]
	if re == nil {
		panic("validateBoolProp: unregistered prop " + prop + " — add it to reBoolPropMap")
	}
	if m := re.FindStringSubmatch(s); m != nil {
		val := strings.ToUpper(m[1])
		if val != "TRUE" && val != "FALSE" {
			*markers = append(*markers, diagMarkerSpan(r, fmt.Sprintf("%s must be TRUE or FALSE.", prop), 4))
		}
	}
}

// reParenKeyMap holds pre-compiled regexes for extractParenContent. Each entry
// matches KEY = ( to locate the start of a parenthesized block.
var reParenKeyMap = func() map[string]*regexp.Regexp {
	keys := []string{"FILE_FORMAT", "STORAGE_LOCATIONS"}
	m := make(map[string]*regexp.Regexp, len(keys))
	for _, k := range keys {
		m[k] = regexp.MustCompile(`(?i)\b` + k + `\s*=\s*\(`)
	}
	return m
}()

func extractParenContent(s string, key string) string {
	re := reParenKeyMap[key]
	if re == nil {
		panic("extractParenContent: unregistered key " + key + " — add it to reParenKeyMap")
	}
	loc := re.FindStringIndex(s)
	if loc == nil {
		return ""
	}
	content := s[loc[1]-1:]
	endIdx := findMatchingParen(content)
	if endIdx == -1 {
		return content[1:]
	}
	return content[1:endIdx]
}

func validateCreateIcebergTable(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	// Strip comments first to prevent comment-spoofing in property parsing
	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
	clean := reStripStringLiterals.ReplaceAllString(stripped, " ")
	props := getStatementProperties(stripped)

	catalog, hasCatalog := props["CATALOG"]
	isSnowflakeCatalog := hasCatalog && strings.EqualFold(strings.Trim(catalog, "'"), "SNOWFLAKE")

	// Rule: BASE_LOCATION is mandatory for all Iceberg tables.
	if val, ok := props["BASE_LOCATION"]; !ok || strings.TrimSpace(strings.Trim(val, "'")) == "" {
		markers = append(markers, diagMarkerSpan(r, "BASE_LOCATION is mandatory for all Iceberg tables and cannot be empty.", 4))
	}

	// Rule: EXTERNAL_VOLUME and CATALOG are mandatory for non-Snowflake catalogs.
	if !isSnowflakeCatalog {
		if val, ok := props["EXTERNAL_VOLUME"]; !ok || strings.TrimSpace(strings.Trim(val, "'")) == "" {
			markers = append(markers, diagMarkerSpan(r, "EXTERNAL_VOLUME is mandatory for Iceberg tables with external catalogs.", 4))
		}
		if val, ok := props["CATALOG"]; !ok || strings.TrimSpace(strings.Trim(val, "'")) == "" {
			markers = append(markers, diagMarkerSpan(r, "CATALOG is mandatory for Iceberg tables with external catalogs.", 4))
		}
	}

	// Rule: TRANSIENT is not supported for Iceberg tables.
	if reIsCreateTransientIcebergTable.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r, "TRANSIENT is not supported for Iceberg tables.", 4))
	}

	// Rule: CATALOG_TABLE_NAME and CATALOG_NAMESPACE are only valid for non-Snowflake catalogs.
	if isSnowflakeCatalog {
		if _, ok := props["CATALOG_TABLE_NAME"]; ok {
			markers = append(markers, diagMarkerSpan(r, "CATALOG_TABLE_NAME is only valid when CATALOG is not 'SNOWFLAKE'.", 4))
		}
		if _, ok := props["CATALOG_NAMESPACE"]; ok {
			markers = append(markers, diagMarkerSpan(r, "CATALOG_NAMESPACE is only valid when CATALOG is not 'SNOWFLAKE'.", 4))
		}
	}

	// Rule: OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(clean) && reIfNotExists.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE ICEBERG TABLE statement.", 4))
	}

	// Rule: OR REPLACE is not supported for external catalogs.
	if !isSnowflakeCatalog && reOrReplace.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "OR REPLACE is not supported for Iceberg tables backed by external catalogs.", 4))
	}

	// Rule: CLUSTER BY is only for Snowflake-managed tables.
	if !isSnowflakeCatalog && rePatternClusterBy.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "CLUSTER BY is supported only for Snowflake-managed Iceberg tables.", 4))
	}

	// Rule: DATA_RETENTION_TIME_IN_DAYS is only for Snowflake-managed tables.
	if !isSnowflakeCatalog && reDataRetention.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "DATA_RETENTION_TIME_IN_DAYS applies only to Snowflake-managed Iceberg tables.", 4))
	}

	// Value validation for specific properties
	if val, ok := props["REPLACE_INVALID_CHARACTERS"]; ok && !isBool(val) {
		markers = append(markers, diagMarkerSpan(r, "REPLACE_INVALID_CHARACTERS must be TRUE or FALSE.", 4))
	}
	if val, ok := props["AUTO_REFRESH"]; ok && !isBool(val) {
		markers = append(markers, diagMarkerSpan(r, "AUTO_REFRESH must be TRUE or FALSE.", 4))
	}
	if val, ok := props["REFRESH_MODE"]; ok && !isValidEnumValue(val, "AUTO", "FULL", "INCREMENTAL") {
		markers = append(markers, diagMarkerSpan(r, "Invalid REFRESH_MODE value. Must be AUTO, FULL, or INCREMENTAL.", 4))
	}
	if val, ok := props["INITIALIZE"]; ok && !isValidEnumValue(val, "ON_CREATE", "ON_SCHEDULE") {
		markers = append(markers, diagMarkerSpan(r, "Invalid INITIALIZE value. Must be ON_CREATE or ON_SCHEDULE.", 4))
	}

	return markers
}

// ── CREATE ICEBERG TABLE helpers ──────────────────────────────────────────

func getStatementProperties(s string) map[string]string {
	props := make(map[string]string)
	// Strip parentheses and their contents first, as they contain column definitions, CHECKs, etc.
	// This helps avoid spoofing property keys like CATALOG or EXTERNAL_VOLUME.
	s = stripParenContents(s)
	matches := reGetStatementProperties.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		props[strings.ToUpper(match[1])] = match[2]
	}
	return props
}

func isBool(s string) bool {
	upper := strings.ToUpper(strings.Trim(s, "'"))
	return upper == "TRUE" || upper == "FALSE"
}

func isValidEnumValue(val string, validValues ...string) bool {
	return slices.Contains(validValues, strings.ToUpper(strings.Trim(val, "'")))
}

func validateCreateHybridTable(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
	clean := reStripStringLiterals.ReplaceAllString(stripped, " ")

	if reOrReplace.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "OR REPLACE is not supported for hybrid tables.", 4))
	}

	if reTransient.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "TRANSIENT is not supported for hybrid tables.", 4))
	}

	if rePatternClusterBy.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "CLUSTER BY is not supported on hybrid tables.", 4))
	}

	if reDataRetention.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "DATA_RETENTION_TIME_IN_DAYS is not applicable to hybrid tables.", 4))
	}

	if reChangeTracking.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "CHANGE_TRACKING is not supported on hybrid tables.", 4))
	}

	if reCopyGrants.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r, "COPY GRANTS is not supported on hybrid tables.", 4))
	}

	preambleMatch := reHybridTablePreamble.FindString(stripped)
	if preambleMatch == "" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE HYBRID TABLE statement.", 4))
		return markers
	}

	rest := strings.TrimSpace(stripped[len(preambleMatch):])
	if strings.HasPrefix(rest, "(") {
		endIdx := findMatchingParen(rest)
		if endIdx != -1 {
			colsContent := rest[1:endIdx]

			var hasPK bool
			pkCols := make(map[string]bool)
			colHasNotNull := make(map[string]bool)

			segments := splitHybridSegments(colsContent)
			for _, seg := range segments {
				segClean := reStripStringLiterals.ReplaceAllString(seg, " ")
				upSeg := strings.ToUpper(segClean)

				// Normalize whitespace before checking prefixes
				content := strings.Join(strings.Fields(upSeg), " ")

				// Handle CONSTRAINT prefix
				if strings.HasPrefix(content, "CONSTRAINT") {
					constraintBody := strings.TrimSpace(content[10:]) // len("CONSTRAINT") == 10
					fields := strings.Fields(constraintBody)
					if len(fields) > 1 {
						content = strings.Join(fields[1:], " ")
					}
				}

				if strings.HasPrefix(content, "PRIMARY KEY") {
					hasPK = true
					// Out of line: PRIMARY KEY (c1, c2)
					if m := rePrimaryKeyCols.FindString(segClean); m != "" {
						if openIdx := strings.Index(m, "("); openIdx != -1 {
							mStr := m[openIdx+1 : len(m)-1]
							for p := range strings.SplitSeq(mStr, ",") {
								pkCols[normalizeIdent(p)] = true
							}
						}
					}
				} else if !strings.HasPrefix(content, "FOREIGN KEY") && !strings.HasPrefix(content, "UNIQUE") && !strings.HasPrefix(content, "INDEX") {
					// Column definition
					words := strings.Fields(segClean)
					if len(words) > 0 {
						colName := normalizeIdent(words[0])
						if rePrimaryKey.MatchString(upSeg) {
							hasPK = true
							pkCols[colName] = true
						}
						if reNotNull.MatchString(upSeg) || reAutoIncrement.MatchString(upSeg) {
							colHasNotNull[colName] = true
						}
					}
				}
			}

			if !hasPK {
				markers = append(markers, diagMarkerSpan(r, "Hybrid tables must have a PRIMARY KEY constraint.", 4))
			}

			// Check for NOT NULL on all PK columns
			for pkCol := range pkCols {
				if !colHasNotNull[pkCol] {
					markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("Primary key columns in a hybrid table must be NOT NULL (column '%s' omits it).", pkCol), 4))
				}
			}
		}
	}

	return markers
}

func normalizeIdent(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return s[1 : len(s)-1]
	}
	return strings.ToUpper(s)
}

func splitHybridSegments(s string) []string {
	var segments []string
	depth := 0
	inSingle := false
	inDouble := false
	start := 0
	for i := 0; i <= len(s); i++ {
		var c byte
		if i < len(s) {
			c = s[i]
		} else {
			c = ','
		}

		if inSingle {
			if c == '\'' {
				// Check for doubled quote (escaped quote)
				if i+1 < len(s) && s[i+1] == '\'' {
					i++ // Skip the escaped quote
				} else {
					inSingle = false
				}
			}
		} else if inDouble {
			if c == '"' {
				if i+1 < len(s) && s[i+1] == '"' {
					i++
				} else {
					inDouble = false
				}
			}
		} else {
			if c == '\'' {
				inSingle = true
			} else if c == '"' {
				inDouble = true
			} else if c == '(' {
				depth++
			} else if c == ')' {
				depth--
			} else if c == ',' && depth == 0 {
				segments = append(segments, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return segments
}

func validateCreateFileFormat(s string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(s))
	// Use length-preserving masking for string literals to maintain offsets
	strippedS := reStripStringLiterals.ReplaceAllStringFunc(stripped, func(m string) string {
		return strings.Repeat(" ", len(m))
	})

	// Snowflake Rule: OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(strippedS) && reIfNotExists.MatchString(strippedS) {
		markers = append(markers, diagMarkerSpan(r, "Conflict between OR REPLACE and IF NOT EXISTS in CREATE FILE FORMAT statement.", 4))
	}

	if reTransient.MatchString(strippedS) {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax: TRANSIENT is not supported for FILE FORMAT objects.", 4))
	}

	if reFileFormatTemporary.MatchString(strippedS) {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax: TEMPORARY is not supported for FILE FORMAT objects.", 4))
	}

	// 1. Extract all properties correctly by finding keys in strippedS and values in stripped
	type rawProp struct {
		key string
		val string
	}
	var props []rawProp
	var rawType string

	for _, m := range reFileFormatPropKey.FindAllStringSubmatchIndex(strippedS, -1) {
		key := strings.ToUpper(strippedS[m[2]:m[3]])
		// Find value in stripped starting after the "KEY ="
		valRest := stripped[m[1]:]
		// Match value: either a quoted string or a word
		valMatch := reFileFormatPropValue.FindStringSubmatch(valRest)
		val := ""
		if valMatch != nil {
			val = valMatch[1]
		}

		if key == "TYPE" {
			rawType = strings.ToUpper(strings.Trim(val, "'"))
		} else {
			props = append(props, rawProp{key: key, val: val})
		}
	}

	// If TYPE is not explicitly provided, it defaults to CSV
	if rawType == "" {
		rawType = "CSV"
	}

	var allowedRe *regexp.Regexp
	switch rawType {
	case "CSV":
		allowedRe = reFileFormatAllowedCsv
	case "JSON":
		allowedRe = reFileFormatAllowedJson
	case "AVRO":
		allowedRe = reFileFormatAllowedAvro
	case "ORC":
		allowedRe = reFileFormatAllowedOrc
	case "PARQUET":
		allowedRe = reFileFormatAllowedParquet
	case "XML":
		allowedRe = reFileFormatAllowedXml
	default:
		markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("Invalid TYPE '%s' for FILE FORMAT. Must be CSV, JSON, AVRO, ORC, PARQUET, or XML.", rawType), 4))
		return markers
	}

	// 2. Validate property keys and values
	for _, p := range props {
		if !allowedRe.MatchString(p.key) {
			markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("Property '%s' is not applicable for %s file format.", p.key, rawType), 4))
			continue
		}

		// 3. Type-specific value validations
		if rawType == "CSV" {
			switch p.key {
			case "FIELD_DELIMITER":
				val := strings.Trim(p.val, "'")
				if strings.ToUpper(val) != "NONE" {
					if len([]rune(val)) == 0 {
						markers = append(markers, diagMarkerSpan(r, "FIELD_DELIMITER cannot be empty.", 4))
					} else if len([]rune(val)) > 1 && !reFileFormatValidEsc.MatchString(val) {
						markers = append(markers, diagMarkerSpan(r, "FIELD_DELIMITER must be a single-character string or 'NONE'.", 4))
					}
				}
			case "SKIP_HEADER":
				val := strings.Trim(p.val, "'")
				if strings.HasPrefix(val, "-") {
					markers = append(markers, diagMarkerSpan(r, "SKIP_HEADER must be a non-negative integer.", 4))
				}
			}
		}
	}

	return markers
}

// ── validateCall ──────────────────────────────────────────────────────────────

// validateCall validates a standalone CALL statement for basic structural
// correctness per the Snowflake docs:
//   - Procedure name must be present — bare CALL; should be flagged.
//   - Argument list must be parenthesised — CALL my_proc 1, 2 should be flagged.
//   - INTO :<variable> must have a colon-prefixed variable in scripting contexts.
func validateCall(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// 1. Procedure name must be present.
	if !reCallProcName.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"Missing procedure name in CALL statement.", 4))
		return markers
	}

	// 2. Argument list must be parenthesised.
	if !reCallArgParens.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"CALL statement requires a parenthesised argument list. Use CALL proc_name() even when there are no arguments.", 4))
	}

	// 3. INTO :<variable> — the variable must be prefixed with ':' in scripting contexts.
	// Run against the comment-stripped text to avoid false positives when INTO
	// appears inside a -- or /* */ comment (e.g. "CALL p() -- INTO x is done").
	callStripped := stripCommentsSQL(parseText)
	if m := reCallInto.FindStringSubmatch(callStripped); m != nil {
		varToken := m[1]
		if !strings.HasPrefix(varToken, ":") {
			markers = append(markers, diagMarkerSpan(r, fmt.Sprintf(
				"INTO variable must be prefixed with ':' in Snowflake Scripting. Use INTO :%s instead of INTO %s.",
				varToken, varToken), 4))
		}
	}

	return markers
}

// ── validateWithProcedureCall ─────────────────────────────────────────────────

// validateWithProcedureCall validates a WITH <alias> AS PROCEDURE … CALL <alias>()
// anonymous procedure statement.  It checks that a CALL statement invoking the
// defined alias follows the dollar-quoted procedure body, and delegates the CALL
// structural checks to validateCall.
func validateWithProcedureCall(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// Extract the alias name.
	m := reWithProcAlias.FindStringSubmatch(parseText)
	if m == nil {
		return markers
	}
	alias := m[1]

	// Find the closing delimiter of the procedure body.
	// Snowflake supports both untagged ($$...$$) and tagged ($tag$...$tag$)
	// dollar-quoting.  We collect all $<tag>$ tokens via reAnyDollarTag and
	// treat the rightmost one as the closing delimiter so that tagged forms like
	// $proc$...$proc$ work correctly alongside the plain $$ form.
	var afterBody string
	if tagMatches := reAnyDollarTag.FindAllStringIndex(parseText, -1); len(tagMatches) > 0 {
		lastTagEnd := tagMatches[len(tagMatches)-1][1]
		afterBody = strings.TrimSpace(parseText[lastTagEnd:])
	} else {
		// No dollar-quoted body found; look for CALL anywhere in the statement.
		afterBody = parseText
	}

	if !reIsCall.MatchString(afterBody) {
		markers = append(markers, diagMarkerSpan(r, fmt.Sprintf(
			"WITH ... AS PROCEDURE block must end with CALL %s(...).", alias), 4))
		return markers
	}

	// Delegate structural validation of the trailing CALL to validateCall.
	// Note: validateCall only checks structural correctness (name present, parens,
	// INTO syntax) — it does not verify that the invoked name matches alias.
	// A CALL to a completely different procedure after the WITH block is not flagged
	// here; this is an intentional limitation of static regex-based validation.
	callText := strings.TrimSpace(afterBody)
	markers = append(markers, validateCall(callText, r)...)

	return markers
}

// ── validateExecuteImmediate ───────────────────────────────────────────────────

// validateExecuteImmediate validates an EXECUTE IMMEDIATE statement per the
// Snowflake docs:
//   - A SQL string argument is mandatory; bare EXECUTE IMMEDIATE is invalid.
//   - The argument may be a string literal ('…'), a dollar-quoted string ($$…$$),
//     a colon-prefixed scripting variable (:var), or a bare identifier.
//   - The optional USING clause, if present, must contain at least one bind
//     variable identifier.
func validateExecuteImmediate(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	// 1. A SQL string argument is mandatory.
	if !reExecImmHasArg.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"EXECUTE IMMEDIATE requires a SQL string argument (string literal, dollar-quoted string, or variable reference).", 4))
		return markers
	}

	// 2. USING clause, if present, must contain at least one bind variable.
	// Strip both single-quoted literals and dollar-quoted blocks first to avoid
	// false positives from USING appearing inside the SQL string argument itself
	// (e.g. EXECUTE IMMEDIATE $$MERGE INTO t USING () ON …$$).
	cleanText := reStripStringLiterals.ReplaceAllString(stripped, " ")
	cleanText = reStripDollarQuoted.ReplaceAllString(cleanText, " ")
	if reExecImmUsing.MatchString(cleanText) && !reExecImmUsingHasIdent.MatchString(cleanText) {
		markers = append(markers, diagMarkerSpan(r,
			"USING clause in EXECUTE IMMEDIATE must contain at least one bind variable.", 4))
	}

	return markers
}

// ── validateExecuteTask ───────────────────────────────────────────────────────

// validateExecuteTask validates an EXECUTE TASK statement per the Snowflake
// docs:
//   - A task name (qualified identifier) is required; bare EXECUTE TASK is invalid.
func validateExecuteTask(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	if !reExecTaskName.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"EXECUTE TASK requires a task name. Use EXECUTE TASK <task_name>.", 4))
	}

	return markers
}

// ── validatePut ───────────────────────────────────────────────────────────────

// validatePut validates a Snowflake PUT statement:
//   - file://<path> source is mandatory; bare paths (without file://) should warn.
//   - @<stage> destination is mandatory.
//   - file:// must appear before @<stage> (positional order).
//   - PARALLEL must be a positive integer between 1 and 99.
//   - SOURCE_COMPRESSION must be one of the known compression types.
//   - OVERWRITE must be TRUE or FALSE.
//   - AUTO_COMPRESS must be TRUE or FALSE.
func validatePut(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	// 1. file:// source is mandatory.
	if !reFileURIArg.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"PUT source path must use the file:// prefix (e.g. PUT file:///tmp/data.csv @mystage).", 4))
		return markers
	}

	// 2. @<stage> destination is mandatory.
	// Strip the PUT keyword so that identifiers that happen to contain "@" in
	// comments do not cause false negatives.
	afterKW := strings.TrimSpace(rePutKWStrip.ReplaceAllString(stripped, ""))
	if !reStageRef.MatchString(afterKW) {
		markers = append(markers, diagMarkerSpan(r,
			"PUT requires a stage destination (e.g. @mystage or @~/path/).", 4))
		return markers
	}

	// 3. Verify positional order: PUT file://<path> @<stage>.
	if !rePutCorrectOrder.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"PUT source and destination are in the wrong order. Correct syntax: PUT file://<path> @<stage>.", 4))
		return markers
	}

	// 4. PARALLEL must be 1–99.
	if m := reParallelOption.FindStringSubmatch(stripped); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > 99 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("PARALLEL must be a positive integer between 1 and 99, got '%s'.", m[1]), 4))
		}
	}

	// 5. SOURCE_COMPRESSION must be a known compression type.
	if m := rePutSourceComp.FindStringSubmatch(stripped); m != nil {
		compType := strings.ToUpper(m[1])
		if !slices.Contains(validPutCompressions, compType) {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Invalid SOURCE_COMPRESSION '%s'. Valid values: AUTO_DETECT, GZIP, BZ2, BROTLI, ZSTD, DEFLATE, RAW_DEFLATE, NONE.", m[1]), 4))
		}
	}

	// 6. OVERWRITE must be TRUE or FALSE.
	if m := rePutOverwrite.FindStringSubmatch(stripped); m != nil {
		if v := strings.ToUpper(m[1]); v != "TRUE" && v != "FALSE" {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("OVERWRITE must be TRUE or FALSE, got '%s'.", m[1]), 4))
		}
	}

	// 7. AUTO_COMPRESS must be TRUE or FALSE.
	if m := rePutAutoCompress.FindStringSubmatch(stripped); m != nil {
		if v := strings.ToUpper(m[1]); v != "TRUE" && v != "FALSE" {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("AUTO_COMPRESS must be TRUE or FALSE, got '%s'.", m[1]), 4))
		}
	}

	return markers
}

// ── validateGet ───────────────────────────────────────────────────────────────

// validateGet validates a Snowflake GET statement:
//   - @<stage> source is mandatory.
//   - file://<path> destination is mandatory.
//   - PARALLEL must be a positive integer between 1 and 99.
func validateGet(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	// 1. @<stage> source is mandatory (GET @stage …).
	if !reGetStageArg.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"GET requires a stage source (e.g. GET @mystage file:///tmp/).", 4))
		return markers
	}

	// 2. file:// destination is mandatory.
	if !reFileURIArg.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"GET destination path must use the file:// prefix (e.g. GET @mystage file:///tmp/).", 4))
		return markers
	}

	// 3. PARALLEL must be 1–99.
	if m := reParallelOption.FindStringSubmatch(stripped); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > 99 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("PARALLEL must be a positive integer between 1 and 99, got '%s'.", m[1]), 4))
		}
	}

	return markers
}

// ── validateList ──────────────────────────────────────────────────────────────

// validateList validates a Snowflake LIST (or LS alias) statement:
//   - @<stage> argument is mandatory; bare LIST; should be flagged.
func validateList(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	if !reListStageArg.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"LIST (LS) requires a stage argument (e.g. LIST @mystage).", 4))
	}

	return markers
}

// ── validateRemove ────────────────────────────────────────────────────────────

// validateRemove validates a Snowflake REMOVE (or RM alias) statement:
//   - @<stage> argument is mandatory.
func validateRemove(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	if !reRemoveStageArg.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"REMOVE (RM) requires a stage argument (e.g. REMOVE @mystage).", 4))
	}

	return markers
}

// ── validateCreateEventTable ──────────────────────────────────────────────────

// validateCreateEventTable checks structural requirements for
// CREATE [OR REPLACE] EVENT TABLE statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Event tables have a fixed schema — column definitions are not allowed.
//   - CLUSTER BY is not supported.
//   - DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer.
//   - MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer.
//   - CHANGE_TRACKING must be TRUE or FALSE.
//   - Only recognized properties are allowed (DEFAULT_DDL_COLLATION, COMMENT,
//     COPY GRANTS, TAG). COPY GRANTS is a standalone clause and bypasses
//     the property allowlist check.
func validateCreateEventTable(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// Strip comments and string literals so that keywords inside -- / /* */
	// comments or COMMENT values don't trigger false positive checks.
	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
	clean := reStripStringLiterals.ReplaceAllString(stripped, "''")

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(clean) && reIfNotExists.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"Conflict between OR REPLACE and IF NOT EXISTS in CREATE EVENT TABLE statement.", 4))
		return markers
	}

	// 2. Event table name is required.
	m := reCreateEventTableName.FindStringSubmatch(stripped)
	if m == nil {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE EVENT TABLE statement.", 4))
		return markers
	}

	// 3. Column definitions are not allowed — event tables have a fixed schema.
	if reEventTableColumnList.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"Event tables have a fixed schema and do not support column definitions.", 4))
	}

	// 4. CLUSTER BY is not supported for event tables.
	if rePatternClusterBy.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"CLUSTER BY is not supported for EVENT TABLE.", 4))
	}

	// 5. Validate property values using package-level regexes.
	if m := reEvtRetentionDays.FindStringSubmatch(clean); m != nil {
		if n, err := strconv.Atoi(m[1]); err != nil || n < 0 {
			markers = append(markers, diagMarkerSpan(r,
				"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer.", 4))
		}
	}
	if m := reEvtExtensionDays.FindStringSubmatch(clean); m != nil {
		if n, err := strconv.Atoi(m[1]); err != nil || n < 0 {
			markers = append(markers, diagMarkerSpan(r,
				"MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer.", 4))
		}
	}
	if m := reEvtChangeTrackingValue.FindStringSubmatch(clean); m != nil {
		if !isBool(m[1]) {
			markers = append(markers, diagMarkerSpan(r,
				"CHANGE_TRACKING must be TRUE or FALSE.", 4))
		}
	}

	// 6. Validate allowed properties. Use stripParenContents to avoid
	// false positives from keys inside TAG(...) or other paren blocks.
	// Note: COPY GRANTS is a standalone clause (no '='), so it bypasses
	// validateProperties entirely and needs no allowlist entry.
	validateProperties(stripParenContents(clean), `DATA_RETENTION_TIME_IN_DAYS|MAX_DATA_EXTENSION_TIME_IN_DAYS|CHANGE_TRACKING|DEFAULT_DDL_COLLATION|COMMENT|TAG`, r, &markers)

	return markers
}

// ── validateCreateShare ───────────────────────────────────────────────────────

// validateCreateShare checks structural requirements for CREATE [OR REPLACE] SHARE:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Account-level object: name must not have a database or schema prefix.
//   - Only COMMENT is a valid property.
func validateCreateShare(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// Strip string literals up front so that phrases like "IF NOT EXISTS" or
	// "OR REPLACE" inside a COMMENT value do not trigger false positive checks.
	stripped := reStripStringLiterals.ReplaceAllString(parseText, "''")

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(stripped) && reIfNotExists.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"Conflict between OR REPLACE and IF NOT EXISTS in CREATE SHARE statement.", 4))
		return markers
	}

	// 2. Share name is required; also used for the account-level prefix check.
	m := reCreateShareName.FindStringSubmatch(parseText)
	if m == nil {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE SHARE statement.", 4))
		return markers
	}
	if sqlIdentPathHasDot(m[1]) {
		markers = append(markers, diagMarkerSpan(r,
			"Shares are account-level objects and cannot have a database or schema prefix.", 4))
	}

	// 3. Only COMMENT is a valid property for CREATE SHARE.
	validateProperties(parseText, `COMMENT`, r, &markers)

	return markers
}

// ── validateCreateExternalVolume ──────────────────────────────────────────────

// extVolValidEncTypes lists the accepted ENCRYPTION TYPE values for a
// STORAGE_LOCATIONS location block (S3 and GCS only; AZURE does not support
// the ENCRYPTION parameter). Declared outside the regexp var block because
// it is a string slice, not a compiled regexp.
var extVolValidEncTypes = []string{"NONE", "AWS_SSE_S3", "AWS_SSE_KMS", "GCS_SSE_KMS"}

// validateCreateExternalVolume checks structural requirements for
// CREATE [OR REPLACE] EXTERNAL VOLUME statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Account-level object: name must not have a database or schema prefix.
//   - STORAGE_LOCATIONS is mandatory and must contain at least one location block.
//   - Each location is validated independently:
//   - NAME is required.
//   - STORAGE_PROVIDER must be one of: S3, S3GOV, S3CHINA, S3COMPAT, GCS, AZURE.
//   - STORAGE_BASE_URL is required.
//   - STORAGE_AWS_ROLE_ARN is required for S3, S3GOV, S3CHINA, and S3COMPAT.
//   - AZURE_TENANT_ID is required for AZURE.
//   - STORAGE_AWS_EXTERNAL_ID is only valid for S3-family providers.
//   - ENCRYPTION is not supported for AZURE (any ENCRYPTION block is rejected).
//     For S3/GCS an ENCRYPTION block must contain a TYPE key; its value must be
//     one of NONE, AWS_SSE_S3, AWS_SSE_KMS, or GCS_SSE_KMS, matched to the provider.
//   - ALLOW_WRITES must be TRUE or FALSE if present.
//   - The validator is permissive about extra, unrecognized attributes (e.g.
//     STORAGE_AWS_ROLE_ARN on a GCS location); only the fields listed above are checked.
func validateCreateExternalVolume(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// stripped has comments removed but string literals intact; used wherever
	// we need findMatchingParen / extractParenContent to correctly skip strings.
	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
	// clean has both comments and string literals removed; used for keyword
	// presence checks that must not match inside quoted values.
	clean := reStripStringLiterals.ReplaceAllString(stripped, "''")

	// 0. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(clean) && reIfNotExists.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"Conflict between OR REPLACE and IF NOT EXISTS in CREATE EXTERNAL VOLUME statement.", 4))
		return markers
	}

	// 1. Account-level: name must not have db.schema prefix.
	if m := reCreateExternalVolumeName.FindStringSubmatch(parseText); m != nil {
		if sqlIdentPathHasDot(m[1]) {
			markers = append(markers, diagMarkerSpan(r,
				"External volumes are account-level objects and cannot have a database or schema prefix.", 4))
		}
	}

	// 2. STORAGE_LOCATIONS is mandatory.
	if !reExtVolHasStorageLocs.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"STORAGE_LOCATIONS is mandatory for CREATE EXTERNAL VOLUME.", 4))
		return markers
	}

	// Extract STORAGE_LOCATIONS outer block, then split into individual location
	// entries. Each entry is a (…) block inside the outer (…).
	// We use stripped (comments removed, literals intact) rather than clean so
	// that findMatchingParen can skip parentheses inside quoted string values.
	// Known limitation: if a string literal before the real STORAGE_LOCATIONS
	// clause contains the substring "STORAGE_LOCATIONS = (", extractParenContent
	// may start extraction at the wrong parenthesis. This is extremely unlikely
	// in practice for Snowflake DDL. Comments are absent because stripCommentsSQL
	// was already applied; loc blocks therefore contain no -- or /* */ text.
	// An empty STORAGE_LOCATIONS = () block produces storLocContent == "" and
	// len(locations) == 0, which is caught by the check below.
	storLocContent := extractParenContent(stripped, "STORAGE_LOCATIONS")
	locations := splitLocationBlocks(storLocContent)
	if len(locations) == 0 {
		markers = append(markers, diagMarkerSpan(r,
			"STORAGE_LOCATIONS must contain at least one storage location block.", 4))
		return markers
	}

	// 3–9. Per-location validation.
	for _, loc := range locations {
		// locClean has string literals replaced with '' so structural keyword
		// checks cannot match inside quoted URL or ARN values.
		locClean := reStripStringLiterals.ReplaceAllString(loc, "''")

		// 3. NAME is required in every location block (checked before provider
		// so it is reported even when STORAGE_PROVIDER is also missing).
		if !reExtVolLocationName.MatchString(locClean) {
			markers = append(markers, diagMarkerSpan(r,
				"Each storage location requires a NAME attribute.", 4))
		}

		// 4. STORAGE_BASE_URL is required regardless of provider. Checked before
		// the STORAGE_PROVIDER guard so it is reported even when both attributes
		// are absent — the author sees all missing required fields at once.
		// Note: after literal stripping, STORAGE_BASE_URL = '' satisfies the
		// pattern — empty-string URLs are an accepted trade-off for a structural
		// preamble validator.
		if !reExtVolStorageBaseURL.MatchString(locClean) {
			markers = append(markers, diagMarkerSpan(r,
				"Each storage location requires STORAGE_BASE_URL.", 4))
		}

		// 5. STORAGE_PROVIDER must be present and valid. Use the literal-
		// preserving loc so the regex captures the actual provider string.
		// First-match assumption: if a NAME or COMMENT value happened to
		// contain "STORAGE_PROVIDER = 'S3'" the wrong match would be returned.
		// In practice this is negligible risk for Snowflake DDL.
		pm := reExtVolStorageProvider.FindStringSubmatch(loc)
		if pm == nil {
			markers = append(markers, diagMarkerSpan(r,
				"Each storage location requires STORAGE_PROVIDER (S3, S3GOV, S3CHINA, S3COMPAT, GCS, or AZURE).", 4))
			// Cannot validate provider-specific rules without knowing the provider.
			continue
		}
		provider := strings.ToUpper(pm[1])
		isS3 := provider == "S3" || provider == "S3GOV" || provider == "S3CHINA" || provider == "S3COMPAT"
		isGCS := provider == "GCS"
		isAzure := provider == "AZURE"
		if !isS3 && !isGCS && !isAzure {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Invalid STORAGE_PROVIDER '%s'. Must be S3, S3GOV, S3CHINA, S3COMPAT, GCS, or AZURE.", pm[1]), 4))
			continue
		}

		// 6. STORAGE_AWS_ROLE_ARN is required for S3, S3GOV, S3CHINA, and S3COMPAT.
		if isS3 && !reExtVolAwsRoleArn.MatchString(locClean) {
			markers = append(markers, diagMarkerSpan(r,
				"STORAGE_AWS_ROLE_ARN is required for S3, S3GOV, S3CHINA, and S3COMPAT storage providers.", 4))
		}

		// 7. AZURE_TENANT_ID is required for AZURE.
		if isAzure && !reExtVolAzureTenantID.MatchString(locClean) {
			markers = append(markers, diagMarkerSpan(r,
				"AZURE_TENANT_ID is required for AZURE storage provider.", 4))
		}

		// 8. STORAGE_AWS_EXTERNAL_ID is only valid for S3-family providers.
		if !isS3 && reExtVolAwsExternalID.MatchString(locClean) {
			markers = append(markers, diagMarkerSpan(r,
				"STORAGE_AWS_EXTERNAL_ID is only valid for S3, S3GOV, S3CHINA, or S3COMPAT storage providers.", 4))
		}

		// 9. ENCRYPTION handling is provider-specific (inside per-location loop).
		if isAzure {
			// AZURE uses native storage encryption; the ENCRYPTION parameter is
			// not supported at all for AZURE external volumes. Use the loose
			// presence regex (reExtVolHasEncryption) so blocks like
			// ENCRYPTION = (KMS_KEY_ID = 'k') without a TYPE key are caught too.
			if reExtVolHasEncryption.MatchString(locClean) {
				markers = append(markers, diagMarkerSpan(r,
					"AZURE storage locations do not support the ENCRYPTION parameter.", 4))
			}
		} else if reExtVolHasEncryption.MatchString(locClean) {
			// An ENCRYPTION block is present on an S3 or GCS location.
			// Use the literal-preserving loc so the regex captures the actual type string.
			ems := reExtVolEncryptionType.FindAllStringSubmatch(loc, -1)
			if len(ems) == 0 {
				// ENCRYPTION block exists but has no TYPE key — always an error.
				markers = append(markers, diagMarkerSpan(r,
					"ENCRYPTION block must specify a TYPE key (NONE, AWS_SSE_S3, AWS_SSE_KMS, or GCS_SSE_KMS).", 4))
			} else {
				for _, em := range ems {
					encType := strings.ToUpper(em[1])
					if !slices.Contains(extVolValidEncTypes, encType) {
						markers = append(markers, diagMarkerSpan(r,
							fmt.Sprintf("Invalid ENCRYPTION TYPE '%s'. Must be NONE, AWS_SSE_S3, AWS_SSE_KMS, or GCS_SSE_KMS.", em[1]), 4))
					} else if (encType == "AWS_SSE_S3" || encType == "AWS_SSE_KMS") && !isS3 {
						markers = append(markers, diagMarkerSpan(r,
							fmt.Sprintf("ENCRYPTION TYPE '%s' is only valid for S3, S3GOV, S3CHINA, or S3COMPAT storage providers.", em[1]), 4))
					} else if encType == "GCS_SSE_KMS" && !isGCS {
						markers = append(markers, diagMarkerSpan(r,
							"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS storage provider.", 4))
					}
				}
			}
		}
	}

	// 10. ALLOW_WRITES must be TRUE or FALSE if present. Use clean (comments and
	// string literals both removed) so neither "-- ALLOW_WRITES = maybe" in a
	// comment nor 'ALLOW_WRITES = MAYBE' inside a COMMENT string value can
	// produce a false positive.
	validateBoolProp(clean, "ALLOW_WRITES", r, &markers)

	return markers
}

// splitLocationBlocks iterates over s (the content inside a STORAGE_LOCATIONS
// outer paren) and returns each inner (…) block with its enclosing parens
// stripped. String literals inside the blocks are preserved so that
// per-location regex checks operate on real values.
func splitLocationBlocks(s string) []string {
	var blocks []string
	for i := 0; i < len(s); i++ {
		if s[i] != '(' {
			continue
		}
		end := findMatchingParen(s[i:])
		if end == -1 {
			break
		}
		blocks = append(blocks, s[i+1:i+end])
		i += end // skip past the closing ')'
	}
	return blocks
}

// ── validateAlterShare ────────────────────────────────────────────────────────

// validateAlterShare checks structural requirements for ALTER SHARE statements:
//   - ADD ACCOUNTS = requires at least one account identifier.
//   - RESTRICT is only valid with ADD ACCOUNTS.
func validateAlterShare(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// Strip string literals and comments, then trim, so that RESTRICT or
	// ACCOUNTS inside a COMMENT value or after a trailing line comment cannot
	// cause false positives. cleanText is also trimmed so the $ anchor in
	// reAlterShareRestrictTrailing reliably targets end-of-statement.
	noLiterals := reStripStringLiterals.ReplaceAllString(parseText, "''")
	cleanText := strings.TrimSpace(stripCommentsSQL(noLiterals))

	hasAddAccounts := reAlterShareAddAccounts.MatchString(cleanText)
	hasRestrict := reAlterShareRestrictTrailing.MatchString(cleanText)
	hasAddAcctsEq := reAlterShareAddAcctsEq.MatchString(cleanText)
	hasAcctList := reAlterShareHasAcctList.MatchString(cleanText)

	// RESTRICT is only valid with ADD ACCOUNTS.
	if hasRestrict && !hasAddAccounts {
		markers = append(markers, diagMarkerSpan(r,
			"RESTRICT is only valid with ADD ACCOUNTS in ALTER SHARE.", 4))
	}

	// ADD ACCOUNTS = requires at least one account identifier after the '='.
	if hasAddAcctsEq && !hasAcctList {
		markers = append(markers, diagMarkerSpan(r,
			"ADD ACCOUNTS requires at least one account identifier.", 4))
	}

	return markers
}

// ── validateUseRole ───────────────────────────────────────────────────────────

// validateUseRole validates a USE ROLE statement:
//   - A role name is mandatory; bare USE ROLE is invalid.
func validateUseRole(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	if !reUseRoleHasName.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"USE ROLE requires a role name. Use USE ROLE <role_name>.", 4))
	}

	return markers
}

// ── validateUseWarehouse ──────────────────────────────────────────────────────

// validateUseWarehouse validates a USE WAREHOUSE statement:
//   - A warehouse name is mandatory; bare USE WAREHOUSE is invalid.
func validateUseWarehouse(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	if !reUseWarehouseHasName.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"USE WAREHOUSE requires a warehouse name. Use USE WAREHOUSE <warehouse_name>.", 4))
	}

	return markers
}

// ── validateUseSecondaryRoles ─────────────────────────────────────────────────

// validateUseSecondaryRoles validates a USE SECONDARY ROLES statement:
//   - Only ALL or NONE are valid arguments.
//   - Any other value is flagged.
func validateUseSecondaryRoles(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	if !reUseSecondaryRolesValue.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"USE SECONDARY ROLES requires ALL or NONE.", 4))
	}

	return markers
}

// ── validateAlterSession ──────────────────────────────────────────────────────

// sessionParamKind describes the value constraint for a known session parameter.
type sessionParamKind int

const (
	spString   sessionParamKind = iota // any quoted string
	spBool                             // TRUE or FALSE
	spIntRange                         // integer within [min, max]
	spNonNeg                           // non-negative integer
	spEnum                             // one of a fixed set of values
)

type sessionParamSpec struct {
	kind sessionParamKind
	min  int
	max  int
	vals []string
}

var knownSessionParams = map[string]sessionParamSpec{
	"QUERY_TAG":                          {kind: spString},
	"TIMEZONE":                           {kind: spString},
	"TIMESTAMP_OUTPUT_FORMAT":            {kind: spString},
	"DATE_OUTPUT_FORMAT":                 {kind: spString},
	"TIME_OUTPUT_FORMAT":                 {kind: spString},
	"TIMESTAMP_INPUT_FORMAT":             {kind: spString},
	"TIMESTAMP_NTZ_OUTPUT_FORMAT":        {kind: spString},
	"TIMESTAMP_TZ_OUTPUT_FORMAT":         {kind: spString},
	"TIMESTAMP_LTZ_OUTPUT_FORMAT":        {kind: spString},
	"WEEK_START":                         {kind: spIntRange, min: 0, max: 7},
	"WEEK_OF_YEAR_POLICY":                {kind: spIntRange, min: 0, max: 1},
	"DATE_FIRST_DAY_OF_WEEK":             {kind: spIntRange, min: 0, max: 6},
	"BINARY_OUTPUT_FORMAT":               {kind: spEnum, vals: []string{"HEX", "BASE64", "UTF8"}},
	"ROWS_PER_RESULTSET":                 {kind: spNonNeg},
	"QUOTED_IDENTIFIERS_IGNORE_CASE":     {kind: spBool},
	"AUTOCOMMIT":                         {kind: spBool},
	"TRANSACTION_DEFAULT_ISOLATION_LEVEL": {kind: spEnum, vals: []string{"READ COMMITTED"}},
	"STRICT_JSON_OUTPUT":                 {kind: spBool},
	"JSON_INDENT":                        {kind: spIntRange, min: 0, max: 16},
	"MULTI_STATEMENT_COUNT":              {kind: spNonNeg},
	"USE_CACHED_RESULT":                  {kind: spBool},
	"PYTHON_PROFILER_MODULES":            {kind: spString},
	"PYTHON_PROFILER_TARGET_STAGE":       {kind: spString},
	"SIMULATED_DATA_SHARING_CONSUMER":    {kind: spString},
	"STATEMENT_TIMEOUT_IN_SECONDS":       {kind: spNonNeg},
	"LOCK_TIMEOUT":                       {kind: spNonNeg},
	"GEOGRAPHY_OUTPUT_FORMAT":            {kind: spEnum, vals: []string{"GEOJSON", "WKT", "WKB", "EWKT", "EWKB"}},
	"GEOMETRY_OUTPUT_FORMAT":             {kind: spEnum, vals: []string{"GEOJSON", "WKT", "WKB", "EWKT", "EWKB"}},
	"CLIENT_SESSION_KEEP_ALIVE":          {kind: spBool},
	"ABORT_DETACHED_QUERY":               {kind: spBool},
	"ERROR_ON_NONDETERMINISTIC_MERGE":    {kind: spBool},
	"ERROR_ON_NONDETERMINISTIC_UPDATE":   {kind: spBool},
	"CLIENT_RESULT_CHUNK_SIZE":           {kind: spNonNeg},
	"TWO_DIGIT_CENTURY_START":            {kind: spIntRange, min: 1900, max: 2100},
	"TIMESTAMP_TYPE_MAPPING":             {kind: spEnum, vals: []string{"TIMESTAMP_NTZ", "TIMESTAMP_LTZ", "TIMESTAMP_TZ"}},
	"NETWORK_POLICY":                     {kind: spString},
	"PERIODIC_DATA_REKEYING":             {kind: spBool},
	"CLIENT_MEMORY_LIMIT":                {kind: spNonNeg},
	"CLIENT_PREFETCH_THREADS":            {kind: spNonNeg},
}

// validateAlterSession validates ALTER SESSION SET / UNSET statements:
//   - ALTER SESSION without SET or UNSET is invalid.
//   - ALTER SESSION SET requires at least one <param> = <value> pair.
//   - ALTER SESSION UNSET requires at least one parameter name.
//   - Unknown parameter names produce a warning.
//   - Known parameter values are checked against their type constraints.
func validateAlterSession(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	isSet := reAlterSessionSet.MatchString(stripped)
	isUnset := reAlterSessionUnset.MatchString(stripped)

	if !isSet && !isUnset {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER SESSION requires SET or UNSET. Use ALTER SESSION SET <param> = <value> or ALTER SESSION UNSET <param>.", 4))
		return markers
	}

	if isSet {
		loc := reAlterSessionSet.FindStringIndex(stripped)
		rest := strings.TrimSpace(stripped[loc[1]:])

		pairs := reAlterSessionParam.FindAllStringSubmatch(rest, -1)
		if len(pairs) == 0 {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER SESSION SET requires at least one parameter assignment. Use ALTER SESSION SET <param> = <value>.", 4))
			return markers
		}

		// Check for stray tokens — parameter names without a = value assignment.
		// Remove all matched param = value regions, then look for leftover identifiers.
		residual := reAlterSessionParam.ReplaceAllString(rest, " ")
		for _, tok := range reAlterSessionParamSplit.Split(residual, -1) {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Parameter '%s' is missing '= <value>' assignment.", strings.ToUpper(tok)), 4))
		}

		for _, pair := range pairs {
			paramName := strings.ToUpper(pair[1])
			rawValue := pair[2]

			spec, known := knownSessionParams[paramName]
			if !known {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("Unknown session parameter '%s'.", paramName), 4))
				continue
			}

			// Unquote string values.
			value := rawValue
			if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
				value = value[1 : len(value)-1]
				value = strings.ReplaceAll(value, "''", "'")
			}

			switch spec.kind {
			case spString:
				// Any value accepted.
			case spBool:
				upper := strings.ToUpper(value)
				if upper != "TRUE" && upper != "FALSE" {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("%s must be TRUE or FALSE.", paramName), 4))
				}
			case spIntRange:
				n, err := strconv.Atoi(value)
				if err != nil || n < spec.min || n > spec.max {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("%s must be an integer between %d and %d.", paramName, spec.min, spec.max), 4))
				}
			case spNonNeg:
				n, err := strconv.Atoi(value)
				if err != nil || n < 0 {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("%s must be a non-negative integer.", paramName), 4))
				}
			case spEnum:
				upper := strings.ToUpper(value)
				if !slices.Contains(spec.vals, upper) {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("%s must be one of: %s.", paramName, strings.Join(spec.vals, ", ")), 4))
				}
			}
		}
	}

	if isUnset {
		loc := reAlterSessionUnset.FindStringIndex(stripped)
		rest := strings.TrimSpace(stripped[loc[1]:])

		if rest == "" {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER SESSION UNSET requires at least one parameter name.", 4))
			return markers
		}

		// Parameter names may be comma-separated or whitespace-separated.
		parts := reAlterSessionParamSplit.Split(rest, -1)
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			paramName := strings.ToUpper(p)
			if _, known := knownSessionParams[paramName]; !known {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("Unknown session parameter '%s'.", paramName), 4))
			}
		}
	}

	return markers
}

// ── validateShow ──────────────────────────────────────────────────────────────

// isKeywordBoundary reports whether position pos in s is at a word boundary
// (end of string, whitespace, semicolon, or opening parenthesis).
// Used by validateShow and validateDescribe for keyword-termination checks.
func isKeywordBoundary(s string, pos int) bool {
	if pos >= len(s) {
		return true
	}
	c := s[pos]
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ';' || c == '(' || c == ')'
}

// showClauseKeywords is the set of SHOW clause keywords that must not be
// consumed as identifiers when parsing optional scope names in the IN clause.
var showClauseKeywords = map[string]bool{
	"LIKE": true, "IN": true, "STARTS": true, "WITH": true,
	"LIMIT": true, "FROM": true,
}

// validateShow validates a SHOW <object_type> statement:
//   - The object type keyword must be one of the recognized Snowflake nouns.
//   - The TERSE modifier is only valid for TABLES, VIEWS, SCHEMAS, DATABASES,
//     STAGES.
//   - The HISTORY modifier is only valid for SHOW PIPES and SHOW REPLICATION DATABASES.
//   - LIKE requires a string literal argument.
//   - IN requires a valid scope (ACCOUNT, DATABASE, SCHEMA, TABLE).
//   - STARTS WITH requires a string literal argument.
//   - LIMIT requires a positive integer; the optional FROM requires a string
//     literal.
//   - Any trailing unrecognized text after the parsed clauses is flagged.
func validateShow(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	// Remove "SHOW" keyword.
	rest := strings.TrimSpace(stripped[len("SHOW"):])
	restUp := strings.ToUpper(rest)

	if restUp == "" {
		markers = append(markers, diagMarkerSpan(r,
			"SHOW requires an object type. Use SHOW TABLES, SHOW VIEWS, SHOW SCHEMAS, etc.", 4))
		return markers
	}

	// ── TERSE modifier ───────────────────────────────────────────────────
	isTerse := false
	if strings.HasPrefix(restUp, "TERSE") && isKeywordBoundary(restUp, 5) {
		isTerse = true
		rest = strings.TrimSpace(rest[5:])
		restUp = strings.ToUpper(rest)
	}

	// ── Object type (longest match first) ────────────────────────────────
	objType := ""
	for _, ot := range showObjectTypes {
		if strings.HasPrefix(restUp, ot) && isKeywordBoundary(restUp, len(ot)) {
			objType = ot
			rest = strings.TrimSpace(rest[len(ot):])
			restUp = strings.ToUpper(rest)
			break
		}
	}

	if objType == "" {
		if restUp == "" {
			// Reached when TERSE consumed everything, e.g. "SHOW TERSE".
			markers = append(markers, diagMarkerSpan(r,
				"SHOW TERSE requires an object type. Use SHOW TERSE TABLES, SHOW TERSE VIEWS, etc.", 4))
		} else {
			words := strings.Fields(restUp)
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Unknown object type '%s' in SHOW statement.", words[0]), 4))
		}
		return markers
	}

	// ── Validate TERSE eligibility ───────────────────────────────────────
	if isTerse && !showTerseEligible[objType] {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("TERSE is not valid for SHOW %s. TERSE is supported for TABLES, EXTERNAL TABLES, VIEWS, SCHEMAS, DATABASES, STAGES, STREAMS, USERS.", objType), 4))
	}

	// ── HISTORY modifier ─────────────────────────────────────────────────
	if strings.HasPrefix(restUp, "HISTORY") && isKeywordBoundary(restUp, 7) {
		if !showHistoryEligible[objType] {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("HISTORY is only valid for SHOW PIPES and SHOW REPLICATION DATABASES, not SHOW %s.", objType), 4))
		}
		rest = strings.TrimSpace(rest[7:])
		restUp = strings.ToUpper(rest)
	}

	// Skip clause validation for types with non-standard syntax.
	if showNoClauseValidation[objType] || restUp == "" {
		return markers
	}

	// Optional clauses are parsed in a loop so their order does not matter.
	// Snowflake accepts LIKE, IN, STARTS WITH, and LIMIT in any order.
	// Each clause is consumed at most once; the loop exits when no clause
	// keyword matches the current position.
	seenLike, seenIn, seenStartsWith, seenLimit := false, false, false, false
	for restUp != "" {
		// ── LIKE '<pattern>' ─────────────────────────────────────────
		if !seenLike && strings.HasPrefix(restUp, "LIKE") && isKeywordBoundary(restUp, 4) {
			seenLike = true
			rest = strings.TrimSpace(rest[4:])
			if rest == "" || rest[0] != '\'' {
				markers = append(markers, diagMarkerSpan(r,
					"LIKE requires a string literal. Use LIKE '<pattern>'.", 4))
				return markers
			}
			end := matchStringLiteral(rest)
			if end == -1 {
				markers = append(markers, diagMarkerSpan(r,
					"Unterminated string literal in LIKE clause.", 4))
				return markers
			}
			rest = strings.TrimSpace(rest[end:])
			restUp = strings.ToUpper(rest)
			continue
		}

		// ── IN { ACCOUNT | DATABASE [<db>] | SCHEMA [<schema>] | TABLE [<tbl>] | <ident> }
		if !seenIn && strings.HasPrefix(restUp, "IN") && isKeywordBoundary(restUp, 2) {
			seenIn = true
			rest = strings.TrimSpace(rest[2:])
			restUp = strings.ToUpper(rest)

			matched := false
			for _, scope := range []string{"ACCOUNT", "DATABASE", "SCHEMA", "TABLE"} {
				if strings.HasPrefix(restUp, scope) && isKeywordBoundary(restUp, len(scope)) {
					matched = true
					rest = strings.TrimSpace(rest[len(scope):])
					restUp = strings.ToUpper(rest)
					// Consume optional identifier path for non-ACCOUNT scopes,
					// but never swallow a clause keyword. Check the first path
					// component so that e.g. "my_db.LIKE" is not consumed whole.
					// Quoted identifiers (e.g. "LIKE") are always safe to consume.
					if scope != "ACCOUNT" && rest != "" {
						if m := reIdentPathAnchored.FindString(rest); m != "" {
							first := strings.SplitN(m, ".", 2)[0]
							if strings.HasPrefix(first, `"`) || !showClauseKeywords[strings.ToUpper(first)] {
								rest = strings.TrimSpace(rest[len(m):])
								restUp = strings.ToUpper(rest)
							}
						}
					}
					break
				}
			}

			// Implicit scope: Snowflake allows omitting the scope keyword
			// (e.g., SHOW TABLES IN my_schema). Try consuming an identifier
			// path as an implicit schema scope before reporting an error.
			if !matched {
				if rest == "" {
					markers = append(markers, diagMarkerSpan(r,
						"IN clause requires a scope. Use IN ACCOUNT, IN DATABASE, IN SCHEMA, or IN TABLE.", 4))
					return markers
				}
				if m := reIdentPathAnchored.FindString(rest); m != "" {
					first := strings.SplitN(m, ".", 2)[0]
					if strings.HasPrefix(first, `"`) || !showClauseKeywords[strings.ToUpper(first)] {
						rest = strings.TrimSpace(rest[len(m):])
						restUp = strings.ToUpper(rest)
						matched = true
					}
				}
				if !matched {
					words := strings.Fields(restUp)
					if len(words) > 0 {
						markers = append(markers, diagMarkerSpan(r,
							fmt.Sprintf("Invalid scope '%s' in IN clause. Valid scopes are ACCOUNT, DATABASE, SCHEMA, TABLE.", words[0]), 4))
					} else {
						markers = append(markers, diagMarkerSpan(r,
							"IN clause requires a scope. Use IN ACCOUNT, IN DATABASE, IN SCHEMA, or IN TABLE.", 4))
					}
					return markers
				}
			}
			continue
		}

		// ── STARTS WITH '<prefix>' ───────────────────────────────────
		if !seenStartsWith && strings.HasPrefix(restUp, "STARTS") && isKeywordBoundary(restUp, 6) {
			seenStartsWith = true
			rest = strings.TrimSpace(rest[6:])
			restUp = strings.ToUpper(rest)
			if !(strings.HasPrefix(restUp, "WITH") && isKeywordBoundary(restUp, 4)) {
				markers = append(markers, diagMarkerSpan(r,
					"Expected WITH after STARTS. Use STARTS WITH '<prefix>'.", 4))
				return markers
			}
			rest = strings.TrimSpace(rest[4:])
			if rest == "" || rest[0] != '\'' {
				markers = append(markers, diagMarkerSpan(r,
					"STARTS WITH requires a string literal. Use STARTS WITH '<prefix>'.", 4))
				return markers
			}
			end := matchStringLiteral(rest)
			if end == -1 {
				markers = append(markers, diagMarkerSpan(r,
					"Unterminated string literal in STARTS WITH clause.", 4))
				return markers
			}
			rest = strings.TrimSpace(rest[end:])
			restUp = strings.ToUpper(rest)
			continue
		}

		// ── LIMIT <n> [FROM '<name>'] ────────────────────────────────
		if !seenLimit && strings.HasPrefix(restUp, "LIMIT") && isKeywordBoundary(restUp, 5) {
			seenLimit = true
			rest = strings.TrimSpace(rest[5:])

			// Extract the number token.
			idx := strings.IndexAny(rest, " \t\n\r;")
			numStr := rest
			if idx != -1 {
				numStr = rest[:idx]
				rest = strings.TrimSpace(rest[idx:])
			} else {
				rest = ""
			}
			restUp = strings.ToUpper(rest)

			if numStr == "" {
				markers = append(markers, diagMarkerSpan(r,
					"LIMIT requires a positive integer. Use LIMIT <n>.", 4))
				return markers
			}

			n, err := strconv.Atoi(numStr)
			if err != nil || n <= 0 {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("LIMIT requires a positive integer, got '%s'.", numStr), 4))
				return markers
			}

			// Optional FROM '<name>'
			if strings.HasPrefix(restUp, "FROM") && isKeywordBoundary(restUp, 4) {
				rest = strings.TrimSpace(rest[4:])
				if rest == "" || rest[0] != '\'' {
					markers = append(markers, diagMarkerSpan(r,
						"FROM in LIMIT clause requires a string literal. Use LIMIT <n> FROM '<name>'.", 4))
					return markers
				}
				end := matchStringLiteral(rest)
				if end == -1 {
					markers = append(markers, diagMarkerSpan(r,
						"Unterminated string literal in LIMIT FROM clause.", 4))
					return markers
				}
				rest = strings.TrimSpace(rest[end:])
				restUp = strings.ToUpper(rest)
			}
			continue
		}

		// No clause keyword matched — exit the loop.
		break
	}

	// ── Trailing unrecognized content ────────────────────────────────────
	if restUp != "" {
		if words := strings.Fields(restUp); len(words) > 0 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Unexpected token '%s' in SHOW statement.", words[0]), 4))
		}
	}

	return markers
}

// validateDescribe validates a DESCRIBE / DESC statement:
//   - The object type keyword must be one of the recognized Snowflake nouns.
//   - An object name is mandatory after the object type keyword.
//   - FUNCTION and PROCEDURE require a parenthesised parameter-type signature.
//   - Account-level objects (WAREHOUSE, USER, ROLE, etc.) should not have a
//     database or schema prefix.
//   - Any trailing unrecognized text after the object name is flagged.
func validateDescribe(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
	strippedUp := strings.ToUpper(stripped)

	// Remove the leading DESCRIBE or DESC keyword.
	var rest string
	if strings.HasPrefix(strippedUp, "DESCRIBE") {
		rest = strings.TrimSpace(stripped[len("DESCRIBE"):])
	} else {
		rest = strings.TrimSpace(stripped[len("DESC"):])
	}
	restUp := strings.ToUpper(rest)

	if restUp == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DESCRIBE requires an object type and name. Use DESCRIBE TABLE <name>, DESCRIBE VIEW <name>, etc.", 4))
		return markers
	}

	// ── Object type (longest match first) ────────────────────────────────
	objType := ""
	for _, ot := range describeObjectTypes {
		if strings.HasPrefix(restUp, ot) && isKeywordBoundary(restUp, len(ot)) {
			objType = ot
			rest = strings.TrimSpace(rest[len(ot):])
			restUp = strings.ToUpper(rest)
			break
		}
	}

	if objType == "" {
		words := strings.Fields(restUp)
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("Unknown object type '%s' in DESCRIBE statement.", words[0]), 4))
		return markers
	}

	// ── RESULT and TRANSACTION are special: they take a string literal
	// (query ID / transaction ID) rather than an identifier path.
	if objType == "RESULT" || objType == "TRANSACTION" {
		if restUp == "" {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("DESCRIBE %s requires a query/transaction ID. Use DESCRIBE %s '<id>'.", objType, objType), 4))
		}
		return markers
	}

	// ── Object name is mandatory ─────────────────────────────────────────
	if restUp == "" {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("DESCRIBE %s requires an object name.", objType), 4))
		return markers
	}

	// ── FUNCTION / PROCEDURE: require parenthesised signature ────────────
	if describeNeedsSignature[objType] {
		if !strings.Contains(rest, "(") {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("DESCRIBE %s requires a parameter signature. Use DESCRIBE %s <name>(<arg_types>).", objType, objType), 4))
			return markers
		}
		return markers
	}

	// ── Consume the identifier path ──────────────────────────────────────
	m := reIdentPathAnchored.FindString(rest)
	if m == "" {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("Expected an object name after DESCRIBE %s.", objType), 4))
		return markers
	}

	// ── Account-level objects: warn on db/schema prefix ──────────────────
	if describeAccountLevel[objType] {
		if countIdentParts(m) > 1 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("%s is an account-level object and should not be qualified with a database or schema prefix.", objType), 4))
		}
	}

	// ── Trailing unrecognized content ────────────────────────────────────
	// Skip trailing check when remainder starts with '"' — this indicates
	// an escaped double-quote within a quoted identifier (e.g. "complex""name")
	// that _ident cannot fully consume.
	trailing := strings.TrimSpace(rest[len(m):])
	if trailing != "" && !strings.HasPrefix(trailing, "\"") {
		if words := strings.Fields(strings.ToUpper(trailing)); len(words) > 0 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Unexpected token '%s' after object name in DESCRIBE statement.", words[0]), 4))
		}
	}

	return markers
}

// ── validateCreateTag ─────────────────────────────────────────────────────────

// validateCreateTag checks structural requirements for
// CREATE [OR REPLACE] TAG [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Tag name is mandatory.
//   - ALLOWED_VALUES values must be string literals; duplicates are warned.
//   - Only ALLOWED_VALUES and COMMENT are valid properties.
func validateCreateTag(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := reStripStringLiterals.ReplaceAllString(parseText, "''")

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(stripped) && reIfNotExists.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"Conflict between OR REPLACE and IF NOT EXISTS in CREATE TAG statement.", 4))
		return markers
	}

	// 2. Tag name is required.
	m := reCreateTagName.FindStringSubmatch(parseText)
	if m == nil {
		markers = append(markers, diagMarkerSpan(r, "CREATE TAG requires a tag name.", 4))
		return markers
	}

	// 3. ALLOWED_VALUES values must be string literals; check for duplicates.
	if reTagAllowedValues.MatchString(stripped) {
		lm := reTagStringLiteralList.FindStringSubmatch(parseText)
		if lm == nil {
			markers = append(markers, diagMarkerSpan(r,
				"ALLOWED_VALUES requires a list of string literals (e.g. ALLOWED_VALUES 'v1', 'v2').", 4))
		} else {
			markers = append(markers, checkDuplicateAllowedValues(lm[1], r)...)
		}
	}

	// 4. Only COMMENT is a valid KEY = VALUE property for CREATE TAG.
	// ALLOWED_VALUES uses space-separated syntax (no '='), so reProp
	// inside validateProperties cannot match it — it is already validated above.
	validateProperties(parseText, `COMMENT`, r, &markers)

	return markers
}

// checkDuplicateAllowedValues parses a comma-separated list of string literals
// and warns about duplicate values.
func checkDuplicateAllowedValues(listStr string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	seen := make(map[string]string) // upper-case key → original-case value
	// Walk through the list extracting individual string literals.
	s := listStr
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t\r\n,")
		if len(s) == 0 {
			break
		}
		end := matchStringLiteral(s)
		if end == -1 {
			break
		}
		// Extract the raw value between outer quotes (without unescaping).
		raw := s[1 : end-1]
		key := strings.ToUpper(raw)
		if orig, exists := seen[key]; exists {
			msg := fmt.Sprintf("Duplicate value '%s' in ALLOWED_VALUES list.", raw)
			if raw != orig {
				msg = fmt.Sprintf("Duplicate value '%s' in ALLOWED_VALUES list (case-insensitive match with '%s').", raw, orig)
			}
			markers = append(markers, diagMarkerSpan(r, msg, 4))
		} else {
			seen[key] = raw
		}
		s = s[end:]
	}
	return markers
}

// ── validateAlterTag ─────────────────────────────────────────────────────────

// validateAlterTag checks structural requirements for ALTER TAG statements:
//   - Tag name is mandatory.
//   - RENAME TO requires a new name.
//   - ADD ALLOWED_VALUES / DROP ALLOWED_VALUES require at least one string literal.
//   - UNSET ALLOWED_VALUES takes no additional arguments.
//   - SET COMMENT / UNSET COMMENT are valid sub-commands.
//   - Unknown sub-commands produce a warning.
func validateAlterTag(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := reStripStringLiterals.ReplaceAllString(parseText, "''")
	clean := strings.TrimSpace(stripCommentsSQL(stripped))

	// 1. Tag name is required.
	nm := reAlterTagName.FindStringSubmatch(parseText)
	if nm == nil {
		markers = append(markers, diagMarkerSpan(r, "ALTER TAG requires a tag name.", 4))
		return markers
	}

	// Determine the sub-command by checking after the tag name.
	hasRename := reAlterTagRenameToBare.MatchString(clean)
	hasAddAllowed := reAlterTagAddAllowed.MatchString(clean)
	hasDropAllowed := reAlterTagDropAllowed.MatchString(clean)
	hasUnsetAllowed := reAlterTagUnsetAllowed.MatchString(clean)
	hasSetComment := reAlterTagSetComment.MatchString(clean)
	hasUnsetComment := reAlterTagUnsetComment.MatchString(clean)

	anyKnown := hasRename || hasAddAllowed || hasDropAllowed || hasUnsetAllowed || hasSetComment || hasUnsetComment

	if !anyKnown {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER TAG sub-command. Expected RENAME TO, ADD ALLOWED_VALUES, DROP ALLOWED_VALUES, UNSET ALLOWED_VALUES, SET COMMENT, or UNSET COMMENT.", 4))
		return markers
	}

	// Count sub-commands — Snowflake only allows one per ALTER TAG statement.
	subCmdCount := 0
	for _, has := range []bool{hasRename, hasAddAllowed, hasDropAllowed, hasUnsetAllowed, hasSetComment, hasUnsetComment} {
		if has {
			subCmdCount++
		}
	}
	if subCmdCount > 1 {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER TAG supports only one sub-command per statement.", 4))
	}

	// 2. RENAME TO requires a new name.
	if hasRename && !reAlterTagRenameTo.MatchString(parseText) {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER TAG RENAME TO requires a new tag name.", 4))
	}

	// 3. ADD ALLOWED_VALUES requires at least one string literal value.
	if hasAddAllowed {
		after := reAlterTagAddAllowed.FindStringIndex(parseText)
		if after == nil {
			markers = append(markers, diagMarkerSpan(r,
				"ADD ALLOWED_VALUES requires at least one string literal value.", 4))
		} else {
			rest := strings.TrimSpace(parseText[after[1]:])
			if len(rest) == 0 || rest[0] != '\'' {
				markers = append(markers, diagMarkerSpan(r,
					"ADD ALLOWED_VALUES requires at least one string literal value.", 4))
			} else {
				markers = append(markers, checkDuplicateAllowedValues(rest, r)...)
			}
		}
	}

	// 4. DROP ALLOWED_VALUES requires at least one string literal value.
	if hasDropAllowed {
		after := reAlterTagDropAllowed.FindStringIndex(parseText)
		if after == nil {
			markers = append(markers, diagMarkerSpan(r,
				"DROP ALLOWED_VALUES requires at least one string literal value.", 4))
		} else {
			rest := strings.TrimSpace(parseText[after[1]:])
			if len(rest) == 0 || rest[0] != '\'' {
				markers = append(markers, diagMarkerSpan(r,
					"DROP ALLOWED_VALUES requires at least one string literal value.", 4))
			} else {
				markers = append(markers, checkDuplicateAllowedValues(rest, r)...)
			}
		}
	}

	return markers
}

// ── validateDropTag ──────────────────────────────────────────────────────────

// validateDropTag checks structural requirements for DROP TAG statements:
//   - Tag name is mandatory.
//   - CASCADE / RESTRICT are not valid for DROP TAG and produce a warning.
func validateDropTag(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := reStripStringLiterals.ReplaceAllString(parseText, "''")
	clean := strings.TrimSpace(stripCommentsSQL(stripped))

	// 1. Tag name is required.
	m := reDropTagName.FindStringSubmatch(parseText)
	if m == nil {
		markers = append(markers, diagMarkerSpan(r, "DROP TAG requires a tag name.", 4))
		return markers
	}

	// 2. CASCADE / RESTRICT are not valid for DROP TAG.
	if reDropTagCascadeRestrict.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"CASCADE / RESTRICT are not valid for DROP TAG.", 4))
	}

	return markers
}

// ── validateCreateTask ────────────────────────────────────────────────────

// validateCreateTask checks structural requirements for CREATE TASK statements:
//   - Task name is mandatory.
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - AS clause is required (the task body).
//   - AFTER and SCHEDULE are mutually exclusive (child vs root task).
//   - Root tasks (no AFTER) must have SCHEDULE.
//   - Bare AFTER (no predecessor names) is invalid.
//   - FINALIZE must not be combined with AFTER or SCHEDULE.
//   - FINALIZE requires a root task name (FINALIZE = <name>).
//   - WHEN requires a boolean expression.
func validateCreateTask(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := reStripStringLiterals.ReplaceAllString(parseText, "''")

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if reOrReplace.MatchString(stripped) && reIfNotExists.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"Conflict between OR REPLACE and IF NOT EXISTS in CREATE TASK statement.", 4))
		return markers
	}

	// 2. Task name is required.
	if reCreateTaskName.FindStringSubmatch(parseText) == nil {
		markers = append(markers, diagMarkerSpan(r, "CREATE TASK requires a task name.", 4))
		return markers
	}

	// Split into preamble (before AS) and body for property and structural checks.
	// We need to find the standalone AS keyword, not AS inside a string or function name.
	asIdx := reTaskAS.FindStringIndex(stripped)
	hasAS := asIdx != nil

	var preamble string
	if hasAS {
		preamble = stripped[:asIdx[0]]
	} else {
		preamble = stripped
	}

	hasAfter := reTaskAfter.MatchString(preamble)
	hasSchedule := reTaskSchedule.MatchString(preamble)
	hasFinalize := reTaskFinalizeBare.MatchString(preamble)
	hasWhen := reTaskWhen.MatchString(preamble)

	// 3. AS clause is required.
	if !hasAS {
		markers = append(markers, diagMarkerSpan(r, "CREATE TASK requires an AS clause with a SQL statement body.", 4))
		return markers
	}

	// 4. FINALIZE conflicts.
	if hasFinalize {
		if hasAfter {
			markers = append(markers, diagMarkerSpan(r,
				"FINALIZE must not be combined with AFTER in a CREATE TASK statement.", 4))
		}
		if hasSchedule {
			markers = append(markers, diagMarkerSpan(r,
				"FINALIZE must not be combined with SCHEDULE in a CREATE TASK statement.", 4))
		}
		// FINALIZE requires the = <name> syntax (FINALIZE = <name>).
		if !reTaskFinalizeN.MatchString(preamble) {
			markers = append(markers, diagMarkerSpan(r,
				"FINALIZE requires a root task name (e.g. FINALIZE = root_task).", 4))
		}
		// Validate properties, then return — no SCHEDULE/AFTER checks for finalizer.
		validateProperties(preamble, taskProps, r, &markers)
		return markers
	}

	// 5. AFTER and SCHEDULE are mutually exclusive.
	if hasAfter && hasSchedule {
		markers = append(markers, diagMarkerSpan(r,
			"AFTER and SCHEDULE are mutually exclusive in a CREATE TASK statement. A child task (AFTER) must not also set SCHEDULE.", 4))
	}

	// 6. Bare AFTER without predecessor names.
	if hasAfter && !reTaskAfterNames.MatchString(preamble) {
		markers = append(markers, diagMarkerSpan(r,
			"AFTER requires at least one predecessor task name.", 4))
	}

	// 7. Root task without SCHEDULE.
	if !hasAfter && !hasSchedule {
		markers = append(markers, diagMarkerSpan(r,
			"Root task (no AFTER or FINALIZE clause) requires a SCHEDULE property.", 4))
	}

	// 8. WHEN checks.
	if hasWhen {
		// WHEN requires an expression (not bare WHEN followed directly by AS).
		if !reTaskWhenExpr.MatchString(preamble) {
			markers = append(markers, diagMarkerSpan(r,
				"WHEN requires a boolean expression.", 4))
		}
	}

	// 9. Validate properties.
	validateProperties(preamble, taskProps, r, &markers)

	return markers
}

// ── validateAlterTask ────────────────────────────────────────────────────────

// validateAlterTask checks structural requirements for ALTER TASK statements:
//   - Task name is mandatory.
//   - RESUME / SUSPEND are valid standalone sub-commands.
//   - SET / UNSET modify task properties.
//   - ADD AFTER / REMOVE AFTER manage DAG predecessors.
//   - MODIFY AS / MODIFY WHEN change the task body or condition.
//   - SET FINALIZE sets the finalizer root task.
//   - Unknown sub-commands produce a warning.
func validateAlterTask(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := reStripStringLiterals.ReplaceAllString(parseText, "''")
	clean := strings.TrimSpace(stripped)

	// 1. Task name is required.
	if reAlterTaskName.FindStringSubmatch(parseText) == nil {
		markers = append(markers, diagMarkerSpan(r, "ALTER TASK requires a task name.", 4))
		return markers
	}

	// Determine the sub-command.
	hasResume := reAlterTaskResume.MatchString(clean)
	hasSuspend := reAlterTaskSusp.MatchString(clean)
	hasSet := reAlterTaskSet.MatchString(clean)
	hasUnset := reAlterTaskUnset.MatchString(clean)
	hasRemAfter := reAlterTaskRemAfter.MatchString(clean)
	hasAddAfter := reAlterTaskAddAfter.MatchString(clean)
	hasModifyAS := reAlterTaskModifyAS.MatchString(clean)
	hasModifyWhen := reAlterTaskModifyWhen.MatchString(clean)
	hasSetFinalize := reAlterTaskSetFinalize.MatchString(clean)

	anyKnown := hasResume || hasSuspend || hasSet || hasUnset ||
		hasRemAfter || hasAddAfter || hasModifyAS || hasModifyWhen || hasSetFinalize

	if !anyKnown {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER TASK sub-command. Expected RESUME, SUSPEND, SET, UNSET, ADD AFTER, REMOVE AFTER, MODIFY AS, MODIFY WHEN, or SET FINALIZE.", 4))
		return markers
	}

	// 2. ADD AFTER requires at least one predecessor name.
	if hasAddAfter && !reAlterTaskAddAfterN.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"ADD AFTER requires at least one predecessor task name.", 4))
	}

	// 3. REMOVE AFTER requires at least one predecessor name.
	if hasRemAfter && !reAlterTaskRemAfterN.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"REMOVE AFTER requires at least one predecessor task name.", 4))
	}

	// 4. MODIFY AS requires a SQL body.
	if hasModifyAS && !reAlterTaskModifyASBody.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"MODIFY AS requires a SQL statement.", 4))
	}

	// 5. MODIFY WHEN requires a boolean expression.
	if hasModifyWhen && !reAlterTaskModifyWhenE.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"MODIFY WHEN requires a boolean expression.", 4))
	}

	// 6. SET FINALIZE requires a root task name.
	if hasSetFinalize && !reAlterTaskSetFinalizeN.MatchString(clean) {
		markers = append(markers, diagMarkerSpan(r,
			"SET FINALIZE requires a root task name (e.g. SET FINALIZE = root_task).", 4))
	}

	// 7. Validate property names for SET (excluding SET FINALIZE which is handled above).
	if hasSet && !hasSetFinalize {
		validateProperties(clean, taskProps, r, &markers)
	}

	// 8. Validate property name for UNSET.
	if hasUnset {
		reValid := regexp.MustCompile(`(?i)^(` + taskProps + `)$`)
		if m := reAlterTaskUnsetProp.FindStringSubmatch(clean); m != nil {
			if !reValid.MatchString(m[1]) {
				markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("Unexpected property '%s' in statement.", m[1]), 4))
			}
		}
	}

	return markers
}

// countIdentParts counts the number of dot-separated identifier segments in a
// matched identifier path, correctly skipping dots inside quoted identifiers.
// For example: "my.db".schema.tbl → 3, "my.warehouse" → 1.
func countIdentParts(m string) int {
	parts := 1
	for i := 0; i < len(m); i++ {
		switch m[i] {
		case '"':
			// Skip to closing quote (handles _ident's "[^"]+" pattern).
			i++
			for i < len(m) && m[i] != '"' {
				i++
			}
		case '.':
			parts++
		}
	}
	return parts
}

// matchStringLiteral returns the position right after the closing single quote
// of a SQL string literal at the start of s, or -1 if s does not start with a
// valid string literal.  Embedded '' (escaped quotes) are handled.
func matchStringLiteral(s string) int {
	if len(s) == 0 || s[0] != '\'' {
		return -1
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '\'' {
			if i+1 < len(s) && s[i+1] == '\'' {
				i++ // skip escaped quote
				continue
			}
			return i + 1
		}
	}
	return -1
}

// ── validateBegin ─────────────────────────────────────────────────────────────

// validateBeginStripped validates a BEGIN statement from already-stripped text
// (comments removed, trimmed).
//   - BEGIN [WORK | TRANSACTION] [NAME <name>]
//   - WORK and TRANSACTION are optional synonyms.
//   - NAME <name> provides an optional transaction name (identifier).
func validateBeginStripped(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// BEGIN [WORK|TRANSACTION] with optional NAME <ident>
	if reBeginName.MatchString(stripped) {
		return markers // valid: BEGIN [WORK|TRANSACTION] NAME <ident>
	}
	if reBeginNameBare.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"BEGIN NAME requires a transaction name. Use BEGIN NAME <name>.", 4))
		return markers
	}
	if !reBeginValid.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected token after BEGIN. Valid forms: BEGIN, BEGIN WORK, BEGIN TRANSACTION, BEGIN [TRANSACTION] NAME <name>.", 4))
	}
	return markers
}

// ── validateCommit ────────────────────────────────────────────────────────────

// validateCommitStripped validates a COMMIT statement from already-stripped text
// (comments removed, trimmed).
//   - COMMIT [WORK]
//   - WORK is optional and redundant; extra tokens should warn.
func validateCommitStripped(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	if !reCommitValid.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected token after COMMIT. Valid forms: COMMIT, COMMIT WORK.", 4))
	}
	return markers
}

// ── validateRollback ──────────────────────────────────────────────────────────

// validateRollbackStripped validates a ROLLBACK statement from already-stripped text
// (comments removed, trimmed). This avoids redundant stripCommentsSQL calls when
// the caller has already stripped the text for block-level tracking.
func validateRollbackStripped(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	if reRollbackValid.MatchString(stripped) {
		return markers // valid form
	}

	// Check for TO SAVEPOINT without name.
	if reRollbackToSavepointBare.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"ROLLBACK TO SAVEPOINT requires a savepoint name. Use ROLLBACK TO SAVEPOINT <name>.", 4))
		return markers
	}

	// Check for TO without SAVEPOINT keyword.
	if reRollbackToBare.MatchString(stripped) {
		// Has TO but no valid SAVEPOINT pattern — probably missing SAVEPOINT keyword.
		markers = append(markers, diagMarkerSpan(r,
			"ROLLBACK TO requires SAVEPOINT keyword. Use ROLLBACK TO SAVEPOINT <name>.", 4))
		return markers
	}

	markers = append(markers, diagMarkerSpan(r,
		"Unexpected token after ROLLBACK. Valid forms: ROLLBACK, ROLLBACK WORK, ROLLBACK [WORK] TO SAVEPOINT <name>.", 4))
	return markers
}

// ── validateSavepoint ─────────────────────────────────────────────────────────

// validateSavepointStripped validates a SAVEPOINT statement from already-stripped
// text (comments removed, trimmed).
//   - SAVEPOINT <name> — name is mandatory.
func validateSavepointStripped(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	if !reSavepointHasName.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"SAVEPOINT requires a savepoint name. Use SAVEPOINT <name>.", 4))
	}
	return markers
}

// ── validateReleaseSavepoint ──────────────────────────────────────────────────

// validateReleaseSavepointStripped validates a RELEASE SAVEPOINT statement from
// already-stripped text (comments removed, trimmed).
//   - RELEASE SAVEPOINT <name> — name is mandatory.
func validateReleaseSavepointStripped(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	if !reReleaseSavepointHasName.MatchString(stripped) {
		markers = append(markers, diagMarkerSpan(r,
			"RELEASE SAVEPOINT requires a savepoint name. Use RELEASE SAVEPOINT <name>.", 4))
	}
	return markers
}
