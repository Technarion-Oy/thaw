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
	"thaw/internal/sqltok"
)

// ── Precompiled regexes for ValidateSnowflakePatterns ─────────────────────────

const (
	// ReIdentifier matches a Snowflake identifier part: either a double-quoted
	// string with escaped quotes (""""), or a bare word containing [a-zA-Z0-9_$].
	ReIdentifier = `(?:"(?:""|[^"])*"|[\w$]+)`

	_ident          = `(?:[a-zA-Z_][a-zA-Z0-9_$]*|"[^"]+")`
	_identPath      = _ident + `(?:\.` + _ident + `){0,2}`
	_balancedParens = `\([^()]*(?:(?:\([^()]*\))[^()]*)*\)`
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
			`|REPLICATION|FAILOVER|APPLICATION\s+PACKAGE|APPLICATION|DATASHARE|SERVICE|IMAGE\s+REPOSITORY|GIT\s+REPOSITORY)\b` +
			`|ALTER\s+(?:TABLE|VIEW|STREAM|DATABASE|STAGE|PIPE|PROCEDURE|FUNCTION` +
			`|ALERT|EXTERNAL|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK` +
			`|REPLICATION|FAILOVER|APPLICATION\s+PACKAGE|APPLICATION|DATASHARE|SERVICE|IMAGE\s+REPOSITORY|GIT\s+REPOSITORY)\b` +
			`|DROP\s+(?:TABLE|VIEW|STREAM|STAGE|PIPE|PROCEDURE|FUNCTION|APPLICATION\s+PACKAGE|APPLICATION|DATASHARE|SERVICE|IMAGE\s+REPOSITORY|GIT\s+REPOSITORY)\b` +
			`|EXECUTE\s+(?:JOB\s+)?SERVICE\b` +
			`|UNDROP\s+(?:DATABASE|SCHEMA|TABLE)\b` +
			`|^INSERT\s+(?:OVERWRITE\s+)?(?:ALL|FIRST)\b` +
			`|TRUNCATE\s+\S+\s+IF\b` +
			`|\bLATERAL\s+FLATTEN\b` +
			`|\bINFER_SCHEMA\b` +
			`|\bPIVOT\s*\(` +
			`|\bUNPIVOT\b` +
			`|\bMATCH_RECOGNIZE\s*\(` +
			`|\bASOF\s+JOIN\b`,
	)

	// (reCortexFuncCall removed — token-based)

	reConstraintCol = regexp.MustCompile(`(?i)^(?:CONSTRAINT|PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY)\b`)
	reVirtualColAS  = regexp.MustCompile(`(?i)\bAS\s*\([\s\S]*\)\s*$`)
	rePartitionBy   = regexp.MustCompile(`(?i)^PARTITION\s+BY\b`)

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
	reHybridTablePreamble = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:(?:OR\s+REPLACE|TRANSIENT)\s+)*HYBRID\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath)

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
	reValidDropDbSchema = regexp.MustCompile(`(?i)^\s*DROP\s+(?:DATABASE|SCHEMA)\s+(?:IF\s+EXISTS\s+)?` + _identPath + `(?:\s+(?:CASCADE|RESTRICT))?\s*$`)

	// ── CREATE SEQUENCE ───────────────────────────────────────────────────────
	reValidCreateSeq = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?SEQUENCE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + _identPath +
			`(?:\s+WITH)?(?:\s+(?:` +
			`START(?:\s+WITH|\s*=)?\s+-?\d+` +
			`|INCREMENT(?:\s+BY|\s*=)?\s+-?\d+` +
			`|ORDER|NOORDER` +
			`|COMMENT\s*=\s*'(?:[^']|'')*'` +
			`))*\s*$`)

	// ── ALTER SEQUENCE ────────────────────────────────────────────────────────
	reValidAlterSeq = regexp.MustCompile(
		`(?i)^\s*ALTER\s+SEQUENCE\s+(?:IF\s+EXISTS\s+)?` + _identPath + `\s+` +
			`(?:RENAME\s+TO\s+` + _identPath +
			`|(?:SET\s+)?INCREMENT(?:\s+BY|\s*=)?\s+-?\d+` +
			`|SET(?:\s+(?:ORDER|NOORDER|COMMENT\s*=\s*'(?:[^']|'')*'))+` +
			`|UNSET\s+COMMENT` +
			`)\s*$`)

	// ── DROP SEQUENCE ─────────────────────────────────────────────────────────
	reValidDropSeq = regexp.MustCompile(`(?i)^\s*DROP\s+SEQUENCE\s+(?:IF\s+EXISTS\s+)?` + _identPath + `(?:\s+(?:CASCADE|RESTRICT))?\s*$`)

	// ── CREATE DYNAMIC TABLE ─────────────────────────────────────────────────
	reIsCreateDynTable = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?DYNAMIC\s+TABLE\b`)

	// ── ALTER DYNAMIC TABLE ──────────────────────────────────────────────────
	// (reAlterDynTableName and 11 ALTER DYNAMIC TABLE regexes removed — token-based)

	// ── CREATE INTEGRATION ────────────────────────────────────────────────────
	// (reIntegrationName, reIntegrationType, reIntegrationProvider removed — token-based)

	// ── CREATE WAREHOUSE ──────────────────────────────────────────────────────
	whProps = strings.Join([]string{
		`WAREHOUSE_SIZE`, `WAREHOUSE_TYPE`, `MAX_CLUSTER_COUNT`, `MIN_CLUSTER_COUNT`, `SCALING_POLICY`,
		`AUTO_SUSPEND`, `AUTO_RESUME`, `INITIALLY_SUSPENDED`, `RESOURCE_MONITOR`, `COMMENT`,
		`ENABLE_QUERY_ACCELERATION`, `QUERY_ACCELERATION_MAX_SCALE_FACTOR`,
		`MAX_CONCURRENCY_LEVEL`, `STATEMENT_QUEUED_TIMEOUT_IN_SECONDS`, `STATEMENT_TIMEOUT_IN_SECONDS`,
		`RESOURCE_CONSTRAINT`,
	}, "|")

	// ── CREATE EXTERNAL TABLE ────────────────────────────────────────────────
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
	rmProps = strings.Join([]string{
		`CREDIT_QUOTA`, `FREQUENCY`, `START_TIMESTAMP`, `END_TIMESTAMP`, `NOTIFY_USERS`,
	}, "|")

	// ── CREATE STREAM ─────────────────────────────────────────────────────────
	streamProps = strings.Join([]string{
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

	// (CREATE TASK regexes removed — token-based)

	// (ALTER TASK regexes removed — token-based)

	// ── CREATE ALERT ──────────────────────────────────────────────────────────
	alertProps      = strings.Join([]string{
		`WAREHOUSE`, `SCHEDULE`, `COMMENT`,
	}, "|")
	// (reAlertIfExists, reAlertThen, reAlertWarehouse, reAlertSchedule removed — token-based)

	// ── CREATE PIPE ───────────────────────────────────────────────────────────
	pipeProps = strings.Join([]string{
		`AUTO_INGEST`, `AWS_SNS_TOPIC`, `INTEGRATION`, `COMMENT`, `ERROR_INTEGRATION`,
	}, "|")

	// ── CREATE USER ───────────────────────────────────────────────────────────
	userProps = strings.Join([]string{
		`PASSWORD`, `LOGIN_NAME`, `DISPLAY_NAME`, `FIRST_NAME`, `MIDDLE_NAME`, `LAST_NAME`,
		`EMAIL`, `MUST_CHANGE_PASSWORD`, `DISABLED`, `DAYS_TO_EXPIRY`, `MINS_TO_UNLOCK`,
		`DEFAULT_WAREHOUSE`, `DEFAULT_NAMESPACE`, `DEFAULT_ROLE`, `RSA_PUBLIC_KEY`,
		`RSA_PUBLIC_KEY_2`, `COMMENT`, `TYPE`,
	}, "|")

	// ── CREATE NETWORK POLICY ─────────────────────────────────────────────────
	// (reNetworkPolicyName, reNetworkPolicyIPList, reNetworkPolicyHasAllowedIP,
	// reNetworkPolicyHasAllowedRules removed — token-based)
	networkPolicyProps = strings.Join([]string{
		`ALLOWED_IP_LIST`, `BLOCKED_IP_LIST`,
		`ALLOWED_NETWORK_RULE_LIST`, `BLOCKED_NETWORK_RULE_LIST`,
		`COMMENT`,
	}, "|")

	// ── CREATE ROW ACCESS POLICY ──────────────────────────────────────────────
	// (reRowAccessPolicyParamList, reRowAccessPolicyReturns, reRowAccessPolicyArrow,
	// reRowAccessPolicyASOpen removed — token-based)

	// ── CREATE SESSION POLICY ─────────────────────────────────────────────────
	// (reSessionPolicyName, reSessionIdleTimeout, reSessionUIIdleTimeout removed — token-based)
	sessionPolicyProps = strings.Join([]string{
		`SESSION_IDLE_TIMEOUT_MINS`, `SESSION_UI_IDLE_TIMEOUT_MINS`, `COMMENT`,
	}, "|")

	// ── CREATE PASSWORD POLICY ────────────────────────────────────────────────
	// (rePasswordPolicyName and 11 password property regexes removed — token-based)
	passwordPolicyProps = strings.Join([]string{
		`PASSWORD_MIN_LENGTH`, `PASSWORD_MAX_LENGTH`,
		`PASSWORD_MIN_UPPER_CASE_CHARS`, `PASSWORD_MIN_LOWER_CASE_CHARS`,
		`PASSWORD_MIN_NUMERIC_CHARS`, `PASSWORD_MIN_SPECIAL_CHARS`,
		`PASSWORD_MIN_AGE_DAYS`, `PASSWORD_MAX_AGE_DAYS`,
		`PASSWORD_MAX_RETRIES`, `PASSWORD_LOCKOUT_TIME_MINS`,
		`PASSWORD_HISTORY`, `COMMENT`,
	}, "|")

	// ── CREATE AGGREGATION POLICY ────────────────────────────────────────────
	// (reAggPolicyAS, reAggPolicyReturns, reAggPolicyArrow,
	// reAggPolicyMinGroupSize removed — token-based)

	// ── CREATE PROJECTION POLICY ────────────────────────────────────────────
	// (reProjPolicyAS, reProjPolicyReturns, reProjPolicyArrow,
	// reProjPolicyAllowValue removed — token-based)

	// ── ALTER / DROP AGGREGATION POLICY ──────────────────────────────────────
	// (reAlterPolicyAction removed — token-based)

	// ── ALTER / DROP PROJECTION POLICY ───────────────────────────────────────

	// ── CREATE PACKAGES POLICY ──────────────────────────────────────────────
	// (rePkgPolicyLanguage removed — token-based)

	// ── ALTER / DROP PACKAGES POLICY ────────────────────────────────────────
	// (reAlterPkgPolicyAction removed — token-based)

	// ── CREATE / ALTER / DROP REPLICATION GROUP ─────────────────────────────
	// (reReplGroupName and 12 REPLICATION/FAILOVER GROUP regexes removed — token-based)

	// ── CREATE / ALTER / DROP FAILOVER GROUP ────────────────────────────────
	// (reFailoverGroupAllowedAccounts, reAlterFailoverPrimary .. Resume removed — token-based)

	// ── Time Travel AT / BEFORE clauses ──────────────────────────────────────
	// (reTimeTravelClause, reTimeTravelBare, reTimeTravelArg, reTimeTravelBareKW removed — token-based)

	// validPutCompressions lists the accepted SOURCE_COMPRESSION values for PUT.
	validPutCompressions = []string{"AUTO_DETECT", "GZIP", "BZ2", "BROTLI", "ZSTD", "DEFLATE", "RAW_DEFLATE", "NONE"}

	// ── CREATE SHARE ─────────────────────────────────────────────────────────
	// (reCreateShareName removed — token-based)
	// (ALTER SHARE regexes removed — token-based)

	// (ALTER DATASHARE regexes removed — token-based)

	// (CREATE / EXECUTE / ALTER SERVICE regexes removed — token-based)

	// ── CREATE / ALTER Secret ─────────────────────────────────────────────
	// (reSecretType, reSecretAPIA, reSecretUsername, reSecretPassword, reSecretString,
	// reSecretEnabled, reSecretAlgorithm, reSecretOAuthScopes, reSecretOAuthRT,
	// reSecretOAuthRTExp, reAlterSecretName, reAlterSecretAction removed — token-based)

	// ── CREATE / ALTER / DROP APPLICATION PACKAGE (Native Apps) ────────────
	// (reCreateApplicationPackageName .. reAppPkgDistribution removed — token-based)

	// ── CREATE / ALTER / DROP APPLICATION (Native Apps) ───────────────────
	// (reCreateApplicationName .. reAlterAppUnsetBare removed — token-based)

	// ── CREATE EVENT TABLE ──────────────────────────────────────────────────
	// (reCreateEventTableName .. reEvtChangeTrackingValue removed — token-based)

	// ── CREATE TAG / ALTER TAG / DROP TAG ────────────────────────────────
	// (reCreateTagName, reAlterTagName, reAlterTagRenameTo .. reAlterTagUnsetComment removed — token-based)
	// (reTagAllowedValues, reTagStringLiteralList removed — token-based)
	// ── CREATE NOTEBOOK / ALTER NOTEBOOK / DROP NOTEBOOK ─────────────
	// (all notebook regexes removed — token-based)

	// ── PIVOT / UNPIVOT ──────────────────────────────────────────────────
	// Valid aggregate functions for PIVOT
	pivotValidAggs = map[string]bool{
		"SUM": true, "AVG": true, "COUNT": true, "MAX": true, "MIN": true,
		"ANY_VALUE": true, "LISTAGG": true, "MEDIAN": true,
		"STDDEV": true, "VARIANCE": true,
	}

	// ── ASOF JOIN ──────────────────────────────────────────────────────
	// Detection: matches ASOF JOIN (the core Snowflake time-series join).
	// Kept for findTopLevelMatches tests; production code uses token-based matching.
	reAsofJoinClause = regexp.MustCompile(`(?i)\bASOF\s+JOIN\b`)

	// ── ALTER TABLE … ADD/DROP SEARCH OPTIMIZATION ─────────────────────
	// Detection: matches ALTER TABLE <name> ADD SEARCH OPTIMIZATION or
	// ALTER TABLE <name> DROP SEARCH OPTIMIZATION.
	// ON clause: captures everything after ON (expression list).
	// (reSearchOptOnClause, reSearchOptExpr removed — token-based)
	// Valid search optimization expression types.
	searchOptValidExprs = map[string]bool{
		"EQUALITY":  true,
		"SUBSTRING": true,
		"GEO":       true,
		"FULL_TEXT": true,
	}

	// ── ALTER TABLE … SWAP WITH ─────────────────────────────────────────
	// (reAlterTableSwapWithName, reAlterTableSwapTarget, reAlterTableSwapTrailing removed — token-based)

	// ── INSERT ALL / INSERT FIRST / INSERT OVERWRITE ────────────────────
	// (reIsInsertAll, reIsInsertFirst, reIsInsertOverwrite,
	// reInsertOverwriteSource, reInsertOverwritePrefix removed — token-based)

	// (reInsertMultiThenInto removed — token-based)



	// ── CREATE STAGE ──────────────────────────────────────────────────────────
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
	// (reFileFormatPropKey removed — token-based)
	reFileFormatPropValue = regexp.MustCompile(`^\s*('[^']*'|[A-Za-z0-9_.-]+)`)
	reFileFormatValidEsc  = regexp.MustCompile(`^\\([ntr'\"]|x[0-9A-Fa-f]{2}|u[0-9A-Fa-f]{4}|[0-7]{1,3})$`)
	// (reFileFormatTemporary removed — token-based)

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
	// (reIsCreateTransientIcebergTable removed — token-based)
	// ── ALTER STAGE ───────────────────────────────────────────────────────────
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

	// ── DESCRIBE / DESC ──────────────────────────────────────────────────────

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

	// knownCortexFunctions lists the Snowflake Cortex AI function names
	// (upper-cased).  Any SNOWFLAKE.CORTEX.<name>() call where <name> is not
	// in this set will produce a warning diagnostic.
	// Reference: https://docs.snowflake.com/en/guides-overview-ai-features
	knownCortexFunctions = map[string]bool{
		"COMPLETE":        true,
		"EXTRACT_ANSWER":  true,
		"SENTIMENT":       true,
		"SUMMARIZE":       true,
		"TRANSLATE":       true,
		"CLASSIFY_TEXT":   true,
		"EMBED_TEXT_768":  true,
		"EMBED_TEXT_1024": true,
		"FINETUNE":        true,
		"SEARCH_PREVIEW":  true,
		"TRY_COMPLETE":    true,
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
	"WORKSPACES",
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
	"PIPES":                 true,
	"REPLICATION DATABASES": true,
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
	"WAREHOUSE":                       true,
	"USER":                            true,
	"ROLE":                            true,
	"INTEGRATION":                     true,
	"DATABASE":                        true,
	"SHARE":                           true,
	"RESOURCE MONITOR":                true,
	"NOTIFICATION INTEGRATION":        true,
	"CATALOG INTEGRATION":             true,
	"COMPUTE POOL":                    true,
	"EXTERNAL VOLUME":                 true,
	"NETWORK POLICY":                  true,
	"ORGANIZATION PROFILE":            true,
	"OPENFLOW DATA PLANE INTEGRATION": true,
	"POSTGRES INSTANCE":               true,
	"SPECIFICATION":                   true,
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
	"VIEW":      {"SELECT", "REFERENCES"},
	"STAGE":     {"READ", "WRITE"},
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
	// Snowflake account-level (global) privileges. This list is maintained from
	// real SHOW GRANTS TO ROLE output and the Snowflake documentation. Snowflake
	// adds new privileges regularly; unknown privileges are flagged as warnings.
	"ACCOUNT": {
		// APPLY policies
		"APPLY AGGREGATION POLICY", "APPLY AUTHENTICATION POLICY",
		"APPLY BACKUP RETENTION LOCK", "APPLY CONTACT",
		"APPLY JOIN POLICY", "APPLY LEGAL HOLD",
		"APPLY MASKING POLICY", "APPLY PACKAGES POLICY",
		"APPLY PASSWORD POLICY", "APPLY PRIVACY POLICY",
		"APPLY PROJECTION POLICY", "APPLY RESOURCE GROUP",
		"APPLY ROW ACCESS POLICY", "APPLY SESSION POLICY",
		"APPLY STORAGE LIFECYCLE POLICY", "APPLY TAG",
		"ATTACH POLICY",
		// AUDIT
		"AUDIT",
		// BIND
		"BIND SERVICE ENDPOINT",
		// CANCEL
		"CANCEL QUERY",
		// CREATE
		"CREATE ACCOUNT", "CREATE APPLICATION", "CREATE APPLICATION PACKAGE",
		"CREATE API INTEGRATION", "CREATE COMPUTE POOL",
		"CREATE DATABASE", "CREATE EXTERNAL ACCESS INTEGRATION",
		"CREATE EXTERNAL VOLUME", "CREATE FAILOVER GROUP",
		"CREATE INTEGRATION", "CREATE LISTING", "CREATE MIGRATION",
		"CREATE NETWORK POLICY",
		"CREATE OPENFLOW DATA PLANE INTEGRATION",
		"CREATE OPENFLOW RUNTIME INTEGRATION",
		"CREATE ORGANIZATION LISTING", "CREATE POSTGRES INSTANCE",
		"CREATE PREVIEW APPLICATION", "CREATE PROVISIONED THROUGHPUT",
		"CREATE REPLICATION GROUP", "CREATE ROLE",
		"CREATE SECURITY INTEGRATION", "CREATE SHARE",
		"CREATE SNOWFLAKE INTELLIGENCE", "CREATE UPSTREAM REPOSITORY",
		"CREATE USER", "CREATE WAREHOUSE",
		// DELETE
		"DELETE LINEAGE",
		// EXECUTE
		"EXECUTE ALERT", "EXECUTE AUTO CLASSIFICATION",
		"EXECUTE DATA METRIC FUNCTION", "EXECUTE MANAGED ALERT",
		"EXECUTE MANAGED TASK", "EXECUTE SPARK APPLICATION", "EXECUTE TASK",
		// IMPORT
		"IMPORT ORGANIZATION LISTING", "IMPORT ORGANIZATION USER GROUPS",
		"IMPORT SHARE",
		// INGEST
		"INGEST LINEAGE",
		// MANAGE
		"MANAGE ACCOUNT SUPPORT CASES", "MANAGE BILLING",
		"MANAGE DATA QUALITY", "MANAGE EVENT SHARING",
		"MANAGE FIREWALL_CONFIGURATION", "MANAGE GRANTS",
		"MANAGE LISTING AUTO FULFILLMENT",
		"MANAGE ORGANIZATION SUPPORT CASES",
		"MANAGE POSTGRES PRIVATE CONNECTIVITY",
		"MANAGE SHARE TARGET", "MANAGE USER SUPPORT CASES",
		"MANAGE WAREHOUSES",
		// MODIFY
		"MODIFY LOG EVENT LEVEL", "MODIFY LOG LEVEL",
		"MODIFY METRIC LEVEL",
		"MODIFY SESSION LOG EVENT LEVEL", "MODIFY SESSION LOG LEVEL",
		"MODIFY SESSION METRIC LEVEL", "MODIFY SESSION TRACE LEVEL",
		"MODIFY TRACE LEVEL",
		// MONITOR
		"MONITOR", "MONITOR EXECUTION", "MONITOR ROLE",
		"MONITOR SECURITY", "MONITOR USAGE", "MONITOR USER",
		// OVERRIDE
		"OVERRIDE SHARE RESTRICTIONS",
		// PURCHASE
		"PURCHASE DATA EXCHANGE LISTING",
		// READ
		"READ SESSION",
		"READ UNREDACTED AI OBSERVABILITY EVENTS TABLE",
		"READ UNREDACTED ERROR TABLE",
		// REPLICATE
		"REPLICATE",
		// RESOLVE
		"RESOLVE ALL",
		// USE
		"USE AI FUNCTIONS",
		// VIEW
		"VIEW LINEAGE",
		// Legacy / uncategorised
		"APPLYBUDGET",
	},
	// ROLE is listed so it becomes a known object type for validation;
	// OWNERSHIP itself is accepted universally, but listing it here
	// ensures invalid privileges like USAGE get flagged.
	"ROLE": {"OWNERSHIP"},
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
// parseTextRoute pairs a guard predicate with a validation function. When the
// guard matches parseText, the validator is called and its markers are
// appended. The statement is then skipped (continue).
type parseTextRoute struct {
	guard func(sig []sqltok.Token, parseText string) bool
	fn    func(string, StatementRange) []DiagMarker
}

// parseTextRoutes is the declarative dispatch table for
// ValidateSnowflakePatterns. Order matters: the first matching guard wins
// (mirroring the original if/continue chain). Guards are token-based
// predicates that check statement preambles via significant-token sequences.
var parseTextRoutes = []parseTextRoute{
	// ── CREATE TABLE variants (more specific first) ──
	{guardCreateWithMods([][]string{{"TRANSIENT"}}, "ICEBERG", "TABLE"), validateCreateIcebergTable},
	{guardCreateWithMods([][]string{{"TRANSIENT"}}, "HYBRID", "TABLE"), validateCreateHybridTable},
	{guardCreate("DYNAMIC", "TABLE"), validateCreateDynTable},
	{guardCreate("EXTERNAL", "TABLE"), validateCreateExternalTable},
	{func(_ []sqltok.Token, sql string) bool { return reIsCreateTable.MatchString(sql) }, validateCreateTablePreamble},

	// ── VIEW ──
	{guardCreateWithMods([][]string{{"SECURE"}, {"LOCAL", "GLOBAL"}, {"TEMP", "TEMPORARY", "VOLATILE"}, {"RECURSIVE"}, {"INTERACTIVE"}, {"MATERIALIZED"}}, "VIEW"), validateCreateView},

	// ── COPY INTO ──
	{guardKW("COPY", "INTO"), validateCopyInto},

	// ── TASK ──
	{guardCreate("TASK"), validateCreateTask},
	{guardAlter("TASK"), validateAlterTask},
	{guardDrop("TASK"), validateDropTask},

	// ── ALERT ──
	{guardCreate("ALERT"), validateCreateAlert},

	// ── Policies ──
	{guardCreate("NETWORK", "POLICY"), validateCreateNetworkPolicy},
	{guardCreate("SESSION", "POLICY"), validateCreateSessionPolicy},
	{guardCreate("PASSWORD", "POLICY"), validateCreatePasswordPolicy},
	{guardCreate("ROW", "ACCESS", "POLICY"), validateCreateRowAccessPolicy},
	{guardCreate("AGGREGATION", "POLICY"), validateCreateAggregationPolicy},
	{guardCreate("PROJECTION", "POLICY"), validateCreateProjectionPolicy},
	{guardAlter("AGGREGATION", "POLICY"), func(pt string, r StatementRange) []DiagMarker {
		return validateAlterAggregationOrProjectionPolicy(pt, r, "AGGREGATION")
	}},
	{guardAlter("PROJECTION", "POLICY"), func(pt string, r StatementRange) []DiagMarker {
		return validateAlterAggregationOrProjectionPolicy(pt, r, "PROJECTION")
	}},
	{guardDrop("AGGREGATION", "POLICY"), func(pt string, r StatementRange) []DiagMarker {
		return validateDropAggregationOrProjectionPolicy(pt, r, "AGGREGATION")
	}},
	{guardDrop("PROJECTION", "POLICY"), func(pt string, r StatementRange) []DiagMarker {
		return validateDropAggregationOrProjectionPolicy(pt, r, "PROJECTION")
	}},
	{guardCreate("PACKAGES", "POLICY"), validateCreatePackagesPolicy},
	{guardAlter("PACKAGES", "POLICY"), validateAlterPackagesPolicy},
	{guardDrop("PACKAGES", "POLICY"), validateDropPackagesPolicy},

	// ── Replication / Failover Groups ──
	{guardCreate("REPLICATION", "GROUP"), validateCreateReplicationGroup},
	{guardAlter("REPLICATION", "GROUP"), func(pt string, r StatementRange) []DiagMarker {
		return validateAlterReplicationOrFailoverGroup(pt, r, "REPLICATION")
	}},
	{guardDrop("REPLICATION", "GROUP"), func(pt string, r StatementRange) []DiagMarker {
		return validateDropReplicationOrFailoverGroup(pt, r, "REPLICATION")
	}},
	{guardCreate("FAILOVER", "GROUP"), validateCreateFailoverGroup},
	{guardAlter("FAILOVER", "GROUP"), func(pt string, r StatementRange) []DiagMarker {
		return validateAlterReplicationOrFailoverGroup(pt, r, "FAILOVER")
	}},
	{guardDrop("FAILOVER", "GROUP"), func(pt string, r StatementRange) []DiagMarker {
		return validateDropReplicationOrFailoverGroup(pt, r, "FAILOVER")
	}},

	// ── FILE FORMAT ──
	{guardCreateWithMods([][]string{{"TEMPORARY", "TEMP", "TRANSIENT"}}, "FILE", "FORMAT"), validateCreateFileFormat},

	// ── CALL / Procedure ──
	{guardWithProcAlias(), validateWithProcedureCall},
	{guardKW("CALL"), validateCall},

	// ── GRANT / REVOKE ──
	{guardKW("GRANT"), validateGrant},
	{guardKW("REVOKE"), validateRevoke},

	// ── EXECUTE ──
	{guardKW("EXECUTE", "IMMEDIATE"), validateExecuteImmediate},
	{guardKW("EXECUTE", "TASK"), validateExecuteTask},
	{guardExecuteService(), validateExecuteService},

	// ── Stage commands ──
	{guardKW("PUT"), validatePut},
	{guardKW("GET"), validateGet},
	{guardKWAlt("LIST", "LS"), validateList},
	{guardKWAlt("REMOVE", "RM"), validateRemove},

	// ── Shares ──
	{guardCreate("SHARE"), validateCreateShare},
	{guardAlter("SHARE"), validateAlterShare},
	{guardCreate("DATASHARE"), validateCreateDatashare},
	{guardAlter("DATASHARE"), validateAlterDatashare},
	{guardDrop("DATASHARE"), validateDropDatashare},

	// ── SPCS (Compute Pool, Service, Image/Git Repository) ──
	{guardCreate("COMPUTE", "POOL"), validateCreateComputePool},
	{guardCreate("SERVICE"), validateCreateService},
	{guardAlter("SERVICE"), validateAlterService},
	{guardDrop("SERVICE"), validateDropService},
	{guardCreate("IMAGE", "REPOSITORY"), validateCreateImageRepository},
	{guardDrop("IMAGE", "REPOSITORY"), validateDropImageRepository},
	{guardAlter("IMAGE", "REPOSITORY"), validateAlterImageRepository},
	{guardCreate("GIT", "REPOSITORY"), validateCreateGitRepository},
	{guardAlter("GIT", "REPOSITORY"), validateAlterGitRepository},
	{guardDrop("GIT", "REPOSITORY"), validateDropGitRepository},

	// ── Secret ──
	{guardCreate("SECRET"), validateCreateSecret},
	{guardAlter("SECRET"), validateAlterSecret},

	// ── Native Apps (APPLICATION PACKAGE before APPLICATION) ──
	{guardCreate("APPLICATION", "PACKAGE"), validateCreateApplicationPackage},
	{guardAlter("APPLICATION", "PACKAGE"), validateAlterApplicationPackage},
	{guardDrop("APPLICATION", "PACKAGE"), validateDropApplicationPackage},
	{guardCreate("APPLICATION"), validateCreateApplication},
	{guardAlter("APPLICATION"), validateAlterApplication},
	{guardDrop("APPLICATION"), validateDropApplication},

	// ── Tag ──
	{guardCreate("TAG"), validateCreateTag},
	{guardAlter("TAG"), validateAlterTag},
	{guardDrop("TAG"), validateDropTag},

	// ── Notebook ──
	{guardCreate("NOTEBOOK"), validateCreateNotebook},
	{guardAlter("NOTEBOOK"), validateAlterNotebook},
	{guardDrop("NOTEBOOK"), validateDropNotebook},

	// ── Session ──
	{guardAlter("SESSION"), validateAlterSession},

	// ── Event Table ──
	{guardCreateWithMods([][]string{{"TRANSIENT"}}, "EVENT", "TABLE"), validateCreateEventTable},

	// ── Database / Schema ──
	{guardCreateWithMods([][]string{{"TRANSIENT"}}, "DATABASE"), validateCreateDbOrSchema("DATABASE")},
	{guardCreateWithMods([][]string{{"TRANSIENT"}}, "SCHEMA"), validateCreateDbOrSchema("SCHEMA")},
	{guardDrop("DATABASE"), validateDropDbOrSchema("DATABASE")},
	{guardDrop("SCHEMA"), validateDropDbOrSchema("SCHEMA")},

	// ── Sequence ──
	{guardCreate("SEQUENCE"), validateCreateSequence},
	{guardAlter("SEQUENCE"), validateAlterSequence},
	{guardDrop("SEQUENCE"), validateDropSequence},

	// ── Integration ──
	{guardCreateIntegration(), validateCreateIntegration},

	// ── Warehouse ──
	{guardCreate("WAREHOUSE"), validateCreateWarehouse},

	// ── Resource Monitor ──
	{guardCreate("RESOURCE", "MONITOR"), validateCreateResourceMonitor},

	// ── Stream ──
	{guardCreate("STREAM"), validateCreateStream},

	// ── Pipe ──
	{guardCreate("PIPE"), validateCreatePipe},

	// ── Function / Procedure ──
	{guardCreateWithMods([][]string{{"SECURE"}, {"TEMPORARY", "TEMP"}, {"AGGREGATE"}}, "FUNCTION"), validateCreateFunction},
	{guardCreate("PROCEDURE"), validateCreateProcedure},

	// ── User / Role ──
	{guardCreate("USER"), validateCreateUser},
	{guardCreate("ROLE"), validateCreateRole},

	// ── Masking Policy ──
	{guardCreate("MASKING", "POLICY"), validateCreateMaskingPolicy},

	// ── Stage ──
	{guardCreateWithMods([][]string{{"TEMPORARY"}}, "STAGE"), validateCreateStage},
	{guardAlter("STAGE"), validateAlterStage},

	// ── External Volume ──
	{guardCreate("EXTERNAL", "VOLUME"), validateCreateExternalVolume},

	// ── USE ──
	{guardKW("USE", "ROLE"), validateUseRole},
	{guardKW("USE", "WAREHOUSE"), validateUseWarehouse},
	{guardKW("USE", "SECONDARY", "ROLES"), validateUseSecondaryRoles},

	// ── DESCRIBE / SHOW ──
	{guardKWAlt("DESCRIBE", "DESC"), validateDescribe},
	{guardKW("SHOW"), validateShow},
}

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
		{
			rawToks := sqltok.Tokenize(rawText)
			for _, tok := range rawToks {
				if tok.Kind == sqltok.EOF {
					break
				}
				if (tok.Kind == sqltok.Keyword || tok.Kind == sqltok.Identifier) &&
					strings.EqualFold(tok.Text(rawText), "LATERALFLATTEN") {
					errLine := r.StartLine + tok.Line - 1
					errCol := tok.Col
					markers = append(markers, DiagMarker{
						StartLineNumber: errLine, StartColumn: errCol,
						EndLineNumber: errLine, EndColumn: errCol + (tok.End - tok.Start),
						Message:  "Typo detected: Did you mean 'LATERAL FLATTEN'?",
						Severity: 4,
					})
				}
			}
		}

		// ── Custom check 2: FLATTEN without LATERAL ───────────────────────
		{
			sig := sigToks(sqltok.Tokenize(stripped))
			hasFlattenFromJoin := false
			hasLateralFlatten := false
			hasTableFlatten := false
			for i, tok := range sig {
				u := tokUpper(tok, stripped)
				if u == "FLATTEN" {
					if i > 0 && tokUpper(sig[i-1], stripped) == "LATERAL" {
						hasLateralFlatten = true
					}
					if i > 0 {
						prev := sig[i-1]
						prevU := tokUpper(prev, stripped)
						if prevU == "FROM" || prevU == "JOIN" || prev.Kind == sqltok.Comma {
							hasFlattenFromJoin = true
						}
					}
					if i >= 2 && tokUpper(sig[i-2], stripped) == "TABLE" && sig[i-1].Kind == sqltok.LParen {
						hasTableFlatten = true
					}
				}
			}
			if hasFlattenFromJoin && !hasLateralFlatten && !hasTableFlatten {
				rawToks := sqltok.Tokenize(rawText)
				for _, tok := range rawToks {
					if tok.Kind == sqltok.EOF {
						break
					}
					if (tok.Kind == sqltok.Keyword || tok.Kind == sqltok.Identifier) &&
						strings.EqualFold(tok.Text(rawText), "FLATTEN") {
						errLine := r.StartLine + tok.Line - 1
						errCol := tok.Col
						markers = append(markers, DiagMarker{
							StartLineNumber: errLine, StartColumn: errCol,
							EndLineNumber: errLine, EndColumn: errCol + (tok.End - tok.Start),
							Message:  "FLATTEN used as a table function requires LATERAL. Use LATERAL FLATTEN(...) or TABLE(FLATTEN(...)).",
							Severity: 4,
						})
					}
				}
			}
		}

		// ── Custom check 3: variant path with dots (payload.field.sub) ────
		{
			rawSig := sigToks(sqltok.Tokenize(rawText))
			for i := 0; i+4 < len(rawSig); i++ {
				if rawSig[i].Kind == sqltok.Identifier &&
					rawSig[i+1].Kind == sqltok.Dot &&
					rawSig[i+2].Kind == sqltok.Identifier &&
					rawSig[i+3].Kind == sqltok.Dot &&
					rawSig[i+4].Kind == sqltok.Identifier {
					word1 := strings.ToLower(rawSig[i].Text(rawText))
					if word1 == "payload" {
						startTok := rawSig[i]
						endTok := rawSig[i+4]
						submatch := rawText[startTok.Start:endTok.End]
						errLine := r.StartLine + startTok.Line - 1
						errCol := startTok.Col
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
		}

		// ── Custom check 4: QUALIFY after ORDER BY ────────────────────────
		{
			sig := sigToks(sqltok.Tokenize(stripped))
			orderByIdx := -1
			for i := 0; i+1 < len(sig); i++ {
				if tokUpper(sig[i], stripped) == "ORDER" && tokUpper(sig[i+1], stripped) == "BY" {
					orderByIdx = i
					break
				}
			}
			if orderByIdx >= 0 {
				hasQualifyAfter := false
				for i := orderByIdx + 2; i < len(sig); i++ {
					if tokUpper(sig[i], stripped) == "QUALIFY" {
						hasQualifyAfter = true
						break
					}
				}
				if hasQualifyAfter {
					rawToks := sqltok.Tokenize(rawText)
					for _, tok := range rawToks {
						if tok.Kind == sqltok.EOF {
							break
						}
						if (tok.Kind == sqltok.Keyword || tok.Kind == sqltok.Identifier) &&
							strings.EqualFold(tok.Text(rawText), "QUALIFY") {
							errLine := r.StartLine + tok.Line - 1
							errCol := tok.Col
							markers = append(markers, DiagMarker{
								StartLineNumber: errLine, StartColumn: errCol,
								EndLineNumber: errLine, EndColumn: errCol + (tok.End - tok.Start),
								Message:  "Snowflake 'QUALIFY' must come after 'WHERE' or 'HAVING' but before 'ORDER BY'.",
								Severity: 4,
							})
						}
					}
				}
			}
		}

		// ── Custom check 5: Time Travel AT / BEFORE clause validation ────
		if strings.Contains(strings.ToUpper(stripped), "AT") || strings.Contains(strings.ToUpper(stripped), "BEFORE") {
			markers = append(markers, validateTimeTravelClauses(stripped, r)...)
		}

		// ── Custom check 6: MERGE statement rules ─────────────────────────
		if firstTok == "MERGE" {
			// Tokenize rawText and find top-level WHEN clause boundaries.
			mergeToks := sqltok.Tokenize(rawText)
			mergeSig := sigToks(mergeToks)
			// Collect byte positions of top-level WHEN tokens (depth 0).
			var whenStarts []int
			depth := 0
			for _, t := range mergeSig {
				switch t.Kind {
				case sqltok.LParen:
					depth++
				case sqltok.RParen:
					if depth > 0 {
						depth--
					}
				default:
					if depth == 0 && tokUpper(t, rawText) == "WHEN" {
						whenStarts = append(whenStarts, t.Start)
					}
				}
			}
			for i, start := range whenStarts {
				end := len(rawText)
				if i+1 < len(whenStarts) {
					end = whenStarts[i+1]
				}
				clause := rawText[start:end]
				clauseSig := sigToks(sqltok.Tokenize(stripCommentsSQL(clause)))

				lines := strings.Split(rawText[:start], "\n")
				errLine := r.StartLine + len(lines) - 1
				errCol := len(lines[len(lines)-1]) + 1

				// Classify the clause by inspecting leading sig tokens.
				isWhenMatched := len(clauseSig) >= 2 && kwAt(clauseSig, clause, 0, "WHEN") && kwAt(clauseSig, clause, 1, "MATCHED")
				isWhenNotMatched := len(clauseSig) >= 3 && kwAt(clauseSig, clause, 0, "WHEN") && kwAt(clauseSig, clause, 1, "NOT") && kwAt(clauseSig, clause, 2, "MATCHED")
				hasBySource := hasKWPair(clauseSig, clause, "BY", "SOURCE")

				// 1. WHEN MATCHED (but NOT 'NOT MATCHED')
				if isWhenMatched && !isWhenNotMatched {
					if hasKWPair(clauseSig, clause, "THEN", "INSERT") {
						markers = append(markers, DiagMarker{
							StartLineNumber: errLine, StartColumn: errCol,
							EndLineNumber: errLine, EndColumn: errCol + 12,
							Message:  "INSERT action is not allowed in WHEN MATCHED clause. Use UPDATE or DELETE.",
							Severity: 4,
						})
					}
				}

				// 2. WHEN NOT MATCHED (specifically NOT 'BY SOURCE')
				if isWhenNotMatched && !hasBySource {
					if hasKWPair(clauseSig, clause, "THEN", "UPDATE") || hasKWPair(clauseSig, clause, "THEN", "DELETE") {
						markers = append(markers, DiagMarker{
							StartLineNumber: errLine, StartColumn: errCol,
							EndLineNumber: errLine, EndColumn: errCol + 16,
							Message:  "UPDATE or DELETE action is not allowed in WHEN NOT MATCHED clause. Use INSERT.",
							Severity: 4,
						})
					}
				}

				// 3. WHEN NOT MATCHED BY SOURCE (Not supported by Snowflake)
				if isWhenNotMatched && hasBySource {
					markers = append(markers, DiagMarker{
						StartLineNumber: errLine, StartColumn: errCol,
						EndLineNumber: errLine, EndColumn: errCol + 26,
						Message:  "WHEN NOT MATCHED BY SOURCE is not supported by Snowflake. Use a subquery with a LEFT JOIN as your source to identify missing rows.",
						Severity: 4,
					})
				}
			}
		}

		// ── Custom check 7: unknown SNOWFLAKE.CORTEX.<function> ──────────
		// Token-based scan for SNOWFLAKE.CORTEX.<func>( pattern. The
		// tokenizer naturally classifies comment/string/$$-block content as
		// non-identifier tokens, so no separate inert-region check is needed.
		// Skip for GRANT/REVOKE statements where function signatures appear
		// as object references (e.g. GRANT USAGE ON PROCEDURE SNOWFLAKE.CORTEX.X(...)).
		if firstTok != "GRANT" && firstTok != "REVOKE" {
			cortexSig := sigToks(sqltok.Tokenize(rawText))
			for i := 0; i+5 < len(cortexSig); i++ {
				if tokUpper(cortexSig[i], rawText) == "SNOWFLAKE" &&
					cortexSig[i+1].Kind == sqltok.Dot &&
					tokUpper(cortexSig[i+2], rawText) == "CORTEX" &&
					cortexSig[i+3].Kind == sqltok.Dot &&
					isIdent(cortexSig[i+4]) &&
					cortexSig[i+5].Kind == sqltok.LParen {
					funcName := strings.ToUpper(cortexSig[i+4].Text(rawText))
					if !knownCortexFunctions[funcName] {
						startTok := cortexSig[i]
						endTok := cortexSig[i+4]
						fullMatch := rawText[startTok.Start:endTok.End]
						upTo := rawText[:startTok.Start]
						lines := strings.Split(upTo, "\n")
						errLine := r.StartLine + len(lines) - 1
						errCol := len(lines[len(lines)-1]) + 1
						markers = append(markers, DiagMarker{
							StartLineNumber: errLine, StartColumn: errCol,
							EndLineNumber: errLine, EndColumn: errCol + len(fullMatch),
							Message:  "Unknown Cortex function '" + cortexSig[i+4].Text(rawText) + "'. Known functions: COMPLETE, EXTRACT_ANSWER, SENTIMENT, SUMMARIZE, TRANSLATE, CLASSIFY_TEXT, EMBED_TEXT_768, EMBED_TEXT_1024, FINETUNE, SEARCH_PREVIEW, TRY_COMPLETE.",
							Severity: 4,
						})
					}
					i += 5 // skip past this match
				}
			}
		}

		// ── Table-driven dispatch: try each parseTextRoute in order. ──────
		// Routes with dedicated validator functions are checked first. If a
		// guard matches, the validator runs and we continue to the next
		// statement. Inline dispatch blocks below handle statements that
		// require access to loop-local variables (txnDepth, stripped, etc.).
		ptToks := sqltok.Tokenize(parseText)
		ptSig := sigToks(ptToks)
		routeMatched := false
		for _, route := range parseTextRoutes {
			if route.guard(ptSig, parseText) {
				markers = append(markers, route.fn(parseText, r)...)
				routeMatched = true
				break
			}
		}
		if routeMatched {
			continue
		}

		// ── Other EXECUTE forms (EXECUTE ALERT, etc.) — pass through ─────
		if firstTok == "EXECUTE" {
			continue
		}

		// ── Other USE variants (DATABASE, SCHEMA, bare USE <name>) ───────
		// Valid session commands that don't need pattern validation here;
		// existence checks are handled separately in ValidateTablesExist.
		if firstTok == "USE" {
			continue
		}

		// ── BEGIN (transaction) ─────────────────────────────────────────
		if firstTok == "BEGIN" {
			// Skip anonymous scripting blocks (BEGIN followed by LET, IF, etc.).
			// GetStatementRanges splits on semicolons, so the first "statement"
			// of an anonymous block looks like "BEGIN\n  LET x := 1".
			beginStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			beginSig := sigToks(sqltok.Tokenize(beginStripped))
			if len(beginSig) >= 2 {
				u := tokUpper(beginSig[1], beginStripped)
				if u == "LET" || u == "IF" || u == "FOR" || u == "WHILE" || u == "LOOP" || u == "DECLARE" || u == "RETURN" || u == "CASE" || u == "CALL" {
					continue // scripting block, not a transaction
				}
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
		if firstTok == "COMMIT" {
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
		if firstTok == "ROLLBACK" {
			// Strip comments once and reuse for both validation and block-level tracking.
			rollbackStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			markers = append(markers, validateRollbackStripped(rollbackStripped, r)...)
			// ROLLBACK TO SAVEPOINT does NOT end the transaction — only bare
			// ROLLBACK / ROLLBACK WORK closes it.
			rbSig := sigToks(sqltok.Tokenize(rollbackStripped))
			isToSavepoint := false
			for j := 0; j+1 < len(rbSig); j++ {
				if tokUpper(rbSig[j], rollbackStripped) == "TO" && tokUpper(rbSig[j+1], rollbackStripped) == "SAVEPOINT" {
					isToSavepoint = true
					break
				}
			}
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
		if firstTok == "SAVEPOINT" {
			spStripped := strings.TrimSpace(stripCommentsSQL(parseText))
			markers = append(markers, validateSavepointStripped(spStripped, r)...)
			continue
		}

		// ── RELEASE SAVEPOINT ───────────────────────────────────────────
		if guardKW("RELEASE", "SAVEPOINT")(ptSig, parseText) {
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

		// ── INSERT ALL structural validation ─────────────────────────────
		// Validate before the FP guard skips the statement.
		if guardInsertVariant(ptSig, parseText, "ALL") {
			markers = append(markers, validateInsertAll(stripped, r)...)
			continue
		}

		// ── INSERT FIRST structural validation ───────────────────────────
		// Validate before the FP guard skips the statement.
		if guardInsertVariant(ptSig, parseText, "FIRST") {
			markers = append(markers, validateInsertFirst(stripped, r)...)
			continue
		}

		// ── INSERT OVERWRITE structural validation ───────────────────────
		// Validate before the FP guard skips the statement.
		if guardKW("INSERT", "OVERWRITE")(ptSig, parseText) {
			markers = append(markers, validateInsertOverwrite(stripped, r)...)
			continue
		}

		// ── ALTER TABLE … SWAP WITH ──────────────────────────────────────
		// Validate before the FP guard skips the statement.
		if guardAlterTableAction(ptSig, parseText, "SWAP", "WITH") {
			markers = append(markers, validateAlterTableSwapWith(stripped, r)...)
			continue
		}

		// ── ALTER TABLE … ADD/DROP SEARCH OPTIMIZATION ──────────────────
		// Validate before the FP guard skips the statement.
		if guardAlterTableSearchOpt(ptSig, parseText) {
			markers = append(markers, validateAlterTableSearchOptimization(stripped, r)...)
			continue
		}

		// ── ALTER DYNAMIC TABLE lifecycle commands ───────────────────────
		// Validate before the FP guard skips the statement.
		if guardAlter("DYNAMIC", "TABLE")(ptSig, parseText) {
			markers = append(markers, validateAlterDynamicTable(parseText, r)...)
			continue
		}

		// ── PIVOT / UNPIVOT structural validation ────────────────────────
		// Validate before the FP guard skips the statement.
		if strings.Contains(strings.ToUpper(stripped), "PIVOT") {
			markers = append(markers, validatePivotClauses(stripped, r)...)
		}
		if strings.Contains(strings.ToUpper(stripped), "UNPIVOT") {
			markers = append(markers, validateUnpivotClauses(stripped, r)...)
		}

		// ── MATCH_RECOGNIZE structural validation ────────────────────────
		// Validate before the FP guard skips the statement.
		if strings.Contains(strings.ToUpper(stripped), "MATCH_RECOGNIZE") {
			markers = append(markers, validateMatchRecognizeClauses(stripped, r)...)
		}

		// ── ASOF JOIN structural validation ──────────────────────────────
		// Validate before the FP guard skips the statement.
		if strings.Contains(strings.ToUpper(stripped), "ASOF") {
			markers = append(markers, validateAsofJoinClauses(stripped, r)...)
		}

		// ── Skip Snowflake false-positive statements ──────────────────────
		// (statements with Snowflake-specific syntax that the parser can't
		// handle; we emit no error for these)
		checkText := stripClusterByParens(stripped)
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

// ── Extracted validator functions (dispatch-table entries) ─────────────────────

// validateCreateView checks the CREATE VIEW preamble against the comprehensive
// regex that covers all optional clauses (COPY GRANTS, COMMENT, policies, etc.).
func validateCreateView(parseText string, r StatementRange) []DiagMarker {
	if !reValidCreateViewPreamble.MatchString(parseText) {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in CREATE VIEW statement.", 4)}
	}
	return nil
}

// validateCreateExternalTable validates CREATE EXTERNAL TABLE statements
// including column list, PARTITION BY, WITH LOCATION, FILE_FORMAT, and property checks.
func validateCreateExternalTable(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
	sig := sigToks(sqltok.Tokenize(stripped))

	preambleMatch := reExternalTablePreamble.FindString(parseText)
	if preambleMatch == "" {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in CREATE EXTERNAL TABLE statement.", 4)}
	}

	// OR REPLACE is invalid for EXTERNAL TABLE.
	if hasKWPair(sig, stripped, "OR", "REPLACE") {
		return []DiagMarker{diagMarkerSpan(r, "OR REPLACE is not supported for EXTERNAL TABLE. Use DROP and CREATE.", 4)}
	}

	if hasKWPair(sig, stripped, "CLUSTER", "BY") {
		return []DiagMarker{diagMarkerSpan(r, "CLUSTER BY is not supported for EXTERNAL TABLE.", 4)}
	}
	if hasKW(sig, stripped, "DATA_RETENTION_TIME_IN_DAYS") {
		return []DiagMarker{diagMarkerSpan(r, "DATA_RETENTION_TIME_IN_DAYS is not applicable to EXTERNAL TABLE.", 4)}
	}

	rest := strings.TrimSpace(parseText[len(preambleMatch):])
	rest = strings.TrimSpace(stripCommentsSQL(rest))

	if !strings.HasPrefix(rest, "(") {
		return []DiagMarker{diagMarkerSpan(r, "EXTERNAL TABLE must have a column list.", 4)}
	}

	// Find matching close paren for column list
	endIdx := findMatchingParen(rest)
	if endIdx == -1 {
		return []DiagMarker{diagMarkerSpan(r, "Unclosed column list in CREATE EXTERNAL TABLE statement.", 4)}
	}

	colList := rest[1:endIdx]
	cols := splitTopLevelCommas(colList)

	// Snowflake rejects empty column lists
	if len(cols) == 0 || (len(cols) == 1 && strings.TrimSpace(cols[0]) == "") {
		return []DiagMarker{diagMarkerSpan(r, "Column list must not be empty.", 4)}
	}

	hasColError := false
	for _, col := range cols {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		if reConstraintCol.MatchString(col) {
			continue
		}
		if !reVirtualColAS.MatchString(col) {
			markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("Column '%s' in EXTERNAL TABLE must be a virtual column using AS <expr>.", col), 4))
			hasColError = true
		}
	}
	if hasColError {
		return markers
	}

	after := strings.TrimSpace(rest[endIdx+1:])

	// Check for PARTITION BY
	if loc := rePartitionBy.FindStringIndex(after); loc != nil {
		remainder := strings.TrimSpace(after[loc[1]:])
		if !strings.HasPrefix(remainder, "(") {
			return append(markers, diagMarkerSpan(r, "PARTITION BY in EXTERNAL TABLE requires a parenthesised column list.", 4))
		}
		partEnd := findMatchingParen(remainder)
		if partEnd != -1 {
			after = strings.TrimSpace(remainder[partEnd+1:])
		} else {
			return append(markers, diagMarkerSpan(r, "Unclosed parenthesised column list in PARTITION BY clause.", 4))
		}
	}

	// Mandatory WITH LOCATION and FILE_FORMAT
	afterSig := sigToks(sqltok.Tokenize(after))
	if !(hasKW(afterSig, after, "WITH") && hasKWAssign(afterSig, after, "LOCATION")) {
		return append(markers, diagMarkerSpan(r, "WITH LOCATION = @<stage> is mandatory for EXTERNAL TABLE.", 4))
	}
	if !hasKWAssign(afterSig, after, "FILE_FORMAT") {
		return append(markers, diagMarkerSpan(r, "FILE_FORMAT is mandatory for EXTERNAL TABLE.", 4))
	}

	// Validate remaining properties
	if after != "" && !extTablePropsRe.MatchString(after) {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE EXTERNAL TABLE properties.", 4))
	}
	return markers
}

// validateCreateTablePreamble validates CREATE TABLE preamble: OR REPLACE vs
// IF NOT EXISTS conflict, column list, table properties, CTAS, CLONE, LIKE,
// and USING TEMPLATE forms.
func validateCreateTablePreamble(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigToks(sqltok.Tokenize(parseText))

	// Specific Snowflake Error: OR REPLACE and IF NOT EXISTS are mutually exclusive
	if marker, conflict := checkOrReplaceConflictTok(sig, parseText, r, "CREATE TABLE"); conflict {
		return []DiagMarker{marker}
	}

	preambleMatch := reCreateTablePreamble.FindString(parseText)
	if preambleMatch == "" {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in CREATE TABLE statement.", 4)}
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
		endIdx := findMatchingParen(rest)
		if endIdx != -1 {
			colsContent := rest[1:endIdx]
			colsClean := stripQuotedIdents(sqltok.StripStrings(colsContent))
			colsSig := sigToks(sqltok.Tokenize(colsClean))
			if hasKW(colsSig, colsClean, "INDEX") {
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
	return markers
}

// validateCreateDbOrSchema returns a validator for CREATE DATABASE or CREATE SCHEMA.
func validateCreateDbOrSchema(kind string) func(string, StatementRange) []DiagMarker {
	return func(parseText string, r StatementRange) []DiagMarker {
		if !reValidCreateDbSchema.MatchString(parseText) {
			return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in CREATE "+kind+" statement.", 4)}
		}
		return nil
	}
}

// validateDropDbOrSchema returns a validator for DROP DATABASE or DROP SCHEMA.
func validateDropDbOrSchema(kind string) func(string, StatementRange) []DiagMarker {
	return func(parseText string, r StatementRange) []DiagMarker {
		if !reValidDropDbSchema.MatchString(parseText) {
			return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in DROP "+kind+" statement.", 4)}
		}
		return nil
	}
}

// validateCreateSequence validates CREATE SEQUENCE including ORDER/NOORDER conflict.
func validateCreateSequence(parseText string, r StatementRange) []DiagMarker {
	sig := sigToks(sqltok.Tokenize(parseText))
	bothOrderNoorder := hasKW(sig, parseText, "ORDER") && hasKW(sig, parseText, "NOORDER")
	if !reValidCreateSeq.MatchString(parseText) || bothOrderNoorder {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in CREATE SEQUENCE statement.", 4)}
	}
	return nil
}

// validateAlterSequence validates ALTER SEQUENCE including ORDER/NOORDER conflict.
func validateAlterSequence(parseText string, r StatementRange) []DiagMarker {
	sig := sigToks(sqltok.Tokenize(parseText))
	bothOrderNoorder := hasKW(sig, parseText, "ORDER") && hasKW(sig, parseText, "NOORDER")
	if !reValidAlterSeq.MatchString(parseText) || bothOrderNoorder {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in ALTER SEQUENCE statement.", 4)}
	}
	return nil
}

// validateDropSequence validates DROP SEQUENCE syntax.
func validateDropSequence(parseText string, r StatementRange) []DiagMarker {
	if !reValidDropSeq.MatchString(parseText) {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in DROP SEQUENCE statement.", 4)}
	}
	return nil
}

// validateCreateDynTable validates CREATE DYNAMIC TABLE for mandatory clauses
// (TARGET_LAG, WAREHOUSE, AS SELECT/WITH) using token-based detection.
func validateCreateDynTable(parseText string, r StatementRange) []DiagMarker {
	sig := sigToks(sqltok.Tokenize(parseText))
	if !hasKWAssign(sig, parseText, "TARGET_LAG") ||
		!hasKWAssign(sig, parseText, "WAREHOUSE") ||
		!(hasKWPair(sig, parseText, "AS", "SELECT") || hasKWPair(sig, parseText, "AS", "WITH")) {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in CREATE DYNAMIC TABLE statement.", 4)}
	}
	return nil
}

// validateCreateIntegration validates CREATE INTEGRATION statements:
// account-level name check and type-specific property requirements.
func validateCreateIntegration(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Account-level check: no prefix allowed.
	// Find the name token(s) after INTEGRATION keyword.
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "INTEGRATION" {
			name, _ := readIdentPath(sig, stripped, i+1)
			if name != "" && strings.Contains(name, ".") {
				markers = append(markers, diagMarkerSpan(r, "Integrations are account-level objects and cannot have a database or schema prefix.", 4))
			}
			break
		}
	}

	// 2. Type-specific checks
	switch {
	case hasKWPair(sig, stripped, "API", "INTEGRATION"):
		if !hasKWAssign(sig, stripped, "API_PROVIDER") {
			markers = append(markers, diagMarkerSpan(r, "Missing required parameter API_PROVIDER for API Integration.", 4))
		}
	case hasKWPair(sig, stripped, "NOTIFICATION", "INTEGRATION"):
		if typeVal, ok := findKWAssign(sig, stripped, "TYPE"); ok {
			t := strings.ToUpper(typeVal)
			if t != "EMAIL" && t != "QUEUE" {
				markers = append(markers, diagMarkerSpan(r, "Invalid TYPE for Notification Integration. Valid types are EMAIL, QUEUE.", 4))
			}
		}
	case hasKWPair(sig, stripped, "SECURITY", "INTEGRATION"):
		if !hasKWAssign(sig, stripped, "TYPE") {
			markers = append(markers, diagMarkerSpan(r, "Missing required parameter TYPE for Security Integration.", 4))
		}
	case hasKWSeq(sig, stripped, "EXTERNAL", "ACCESS", "INTEGRATION"):
		if hasKWAssign(sig, stripped, "MAX_RETRIES") {
			markers = append(markers, diagMarkerSpan(r, "Unexpected property 'MAX_RETRIES' for External Access Integration.", 4))
		}
	}
	return markers
}

// validateCreateWarehouse validates CREATE WAREHOUSE: account-level prefix check
// and property validation.
func validateCreateWarehouse(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigToks(sqltok.Tokenize(parseText))
	name, _ := extractNameAfterCreate(sig, parseText, nil, "WAREHOUSE")
	if name != "" && strings.Contains(name, ".") {
		markers = append(markers, diagMarkerSpan(r, "Warehouses are account-level objects and cannot have a database or schema prefix.", 4))
	}
	validateProperties(parseText, whProps, r, &markers)
	return markers
}

// validateCreateResourceMonitor validates CREATE RESOURCE MONITOR properties.
func validateCreateResourceMonitor(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	validateProperties(parseText, rmProps, r, &markers)
	return markers
}

// validateCreateStream validates CREATE STREAM: OR REPLACE vs IF NOT EXISTS
// conflict and full preamble regex.
func validateCreateStream(parseText string, r StatementRange) []DiagMarker {
	sig := sigToks(sqltok.Tokenize(parseText))
	if marker, conflict := checkOrReplaceConflictTok(sig, parseText, r, "CREATE STREAM"); conflict {
		return []DiagMarker{marker}
	}

	if !reValidCreateStream.MatchString(parseText) {
		return []DiagMarker{diagMarkerSpan(r, "Unexpected syntax in CREATE STREAM statement.", 4)}
	}
	return nil
}

// validateCreatePipe validates CREATE PIPE: OR REPLACE conflict, mandatory
// AS COPY INTO, property validation, and AUTO_INGEST/AWS_SNS_TOPIC checks.
func validateCreatePipe(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigToks(sqltok.Tokenize(parseText))

	// 1. Conflict between OR REPLACE and IF NOT EXISTS
	if marker, conflict := checkOrReplaceConflictTok(sig, parseText, r, "CREATE PIPE"); conflict {
		return []DiagMarker{marker}
	}

	// 2. Mandatory AS COPY INTO — find using tokens.
	asIdx := -1
	for pi := 0; pi+2 < len(sig); pi++ {
		if tokUpper(sig[pi], parseText) == "AS" &&
			tokUpper(sig[pi+1], parseText) == "COPY" &&
			tokUpper(sig[pi+2], parseText) == "INTO" {
			asIdx = pi
			break
		}
	}
	if asIdx < 0 {
		return []DiagMarker{diagMarkerSpan(r, "Missing mandatory AS COPY INTO clause in CREATE PIPE statement.", 4)}
	}

	preambleSig := sig[:asIdx]
	preamble := parseText[:sig[asIdx].Start]
	// 3. Property validation
	validateProperties(preamble, pipeProps, r, &markers)

	// 4. AWS_SNS_TOPIC requires AUTO_INGEST = TRUE
	if hasKWAssign(preambleSig, parseText, "AWS_SNS_TOPIC") {
		autoIngestVal, _ := findKWAssignIdent(preambleSig, parseText, "AUTO_INGEST")
		if !strings.EqualFold(autoIngestVal, "TRUE") {
			markers = append(markers, diagMarkerSpan(r, "AWS_SNS_TOPIC is only meaningful when AUTO_INGEST = TRUE.", 4))
		}
	}

	// 5. Warning for AUTO_INGEST = TRUE without stage source
	autoIngestVal, _ := findKWAssignIdent(preambleSig, parseText, "AUTO_INGEST")
	if strings.EqualFold(autoIngestVal, "TRUE") {
		copySig := sig[asIdx:]
		hasFromStage := false
		for pi := 0; pi+1 < len(copySig); pi++ {
			if tokUpper(copySig[pi], parseText) == "FROM" &&
				pi+1 < len(copySig) && copySig[pi+1].Kind == sqltok.At {
				hasFromStage = true
				break
			}
		}
		if !hasFromStage {
			markers = append(markers, diagMarkerSpan(r, "AUTO_INGEST = TRUE typically requires a stage source (FROM @stage).", 4))
		}
	}

	return markers
}

// validateCreateFunction validates CREATE FUNCTION: mandatory RETURNS, LANGUAGE,
// AS body, AGGREGATE/SECURE conflicts, Python/Java/Scala requirements, MEMOIZABLE.
func validateCreateFunction(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigToks(sqltok.Tokenize(parseText))
	asBodyIdx := findFuncBodyAS(sig, parseText)

	preambleSig := sig
	if asBodyIdx < 0 {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS clause in CREATE FUNCTION statement.", 4))
	} else {
		preambleSig = sig[:asBodyIdx]
	}

	isAggregate := hasKWPair(preambleSig, parseText, "AGGREGATE", "FUNCTION")
	isSecure := hasKW(preambleSig, parseText, "SECURE")

	if isSecure && isAggregate {
		markers = append(markers, diagMarkerSpan(r, "SECURE is not supported for AGGREGATE functions.", 4))
	}

	// 1. Mandatory RETURNS
	returnsIdx := -1
	isTable := false
	for pi := 0; pi < len(preambleSig); pi++ {
		if tokUpper(preambleSig[pi], parseText) == "RETURNS" {
			returnsIdx = pi
			if pi+1 < len(preambleSig) && tokUpper(preambleSig[pi+1], parseText) == "TABLE" {
				isTable = true
			}
			break
		}
	}
	if returnsIdx < 0 {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS clause in CREATE FUNCTION statement.", 4))
	} else if isAggregate && isTable {
		markers = append(markers, diagMarkerSpan(r, "AGGREGATE functions cannot return a TABLE.", 4))
	}

	// 2. Mandatory LANGUAGE
	lang := findLanguage(preambleSig, parseText)
	if lang == "" {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory LANGUAGE clause in CREATE FUNCTION statement.", 4))
	} else {
		switch lang {
		case "JAVASCRIPT", "PYTHON", "JAVA", "SCALA", "SQL":
			// valid
		default:
			markers = append(markers, diagMarkerSpan(r, "Unknown or unsupported LANGUAGE '"+lang+"' in CREATE FUNCTION.", 4))
		}

		if lang == "PYTHON" {
			checkPythonRequirements(preambleSig, parseText, r, &markers, "functions")
		}
		if lang == "JAVA" || lang == "SCALA" {
			if !hasKW(preambleSig, parseText, "HANDLER") {
				markers = append(markers, diagMarkerSpan(r, "HANDLER is required for "+lang+" functions.", 4))
			}
		}
	}

	checkNullInputHandling(preambleSig, parseText, r, &markers)

	// MEMOIZABLE
	if hasKW(preambleSig, parseText, "MEMOIZABLE") {
		if isAggregate || isTable {
			markers = append(markers, diagMarkerSpan(r, "MEMOIZABLE is only valid for scalar functions.", 4))
		}
	}

	return markers
}

// validateCreateProcedure validates CREATE PROCEDURE: mandatory RETURNS,
// LANGUAGE, AS body, Python requirements, EXECUTE AS clause.
func validateCreateProcedure(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigToks(sqltok.Tokenize(parseText))

	// CREATE OR ALTER PROCEDURE is not validated here (Snowflake-specific
	// syntax not yet fully supported by the validator).
	if len(sig) >= 3 && tokUpper(sig[1], parseText) == "OR" && tokUpper(sig[2], parseText) == "ALTER" {
		return nil
	}

	asBodyIdx := findFuncBodyAS(sig, parseText)

	preambleSig := sig
	if asBodyIdx < 0 {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS clause in CREATE PROCEDURE statement.", 4))
	} else {
		preambleSig = sig[:asBodyIdx]
	}

	// 1. Mandatory RETURNS
	if !hasKW(preambleSig, parseText, "RETURNS") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS clause in CREATE PROCEDURE statement.", 4))
	}

	// 2. Mandatory LANGUAGE
	lang := findLanguage(preambleSig, parseText)
	if lang == "" {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory LANGUAGE clause in CREATE PROCEDURE statement.", 4))
	} else {
		switch lang {
		case "JAVASCRIPT", "PYTHON", "JAVA", "SCALA", "SQL":
			// valid
		default:
			markers = append(markers, diagMarkerSpan(r, "Unknown or unsupported LANGUAGE '"+lang+"' in CREATE PROCEDURE.", 4))
		}

		if lang == "PYTHON" {
			checkPythonRequirements(preambleSig, parseText, r, &markers, "procedures")
		}
	}

	checkNullInputHandling(preambleSig, parseText, r, &markers)

	// 5. EXECUTE AS
	for pi := 0; pi+2 < len(preambleSig); pi++ {
		if tokUpper(preambleSig[pi], parseText) == "EXECUTE" &&
			tokUpper(preambleSig[pi+1], parseText) == "AS" &&
			isIdent(preambleSig[pi+2]) {
			execVal := strings.ToUpper(preambleSig[pi+2].Text(parseText))
			if execVal != "CALLER" && execVal != "OWNER" {
				markers = append(markers, diagMarkerSpan(r, "EXECUTE AS must be CALLER or OWNER.", 4))
			}
			break
		}
	}

	return markers
}

// validateCreateUser validates CREATE USER: account-level prefix + properties.
func validateCreateUser(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigToks(sqltok.Tokenize(parseText))
	name, _ := extractNameAfterCreate(sig, parseText, nil, "USER")
	if name != "" && strings.Contains(name, ".") {
		markers = append(markers, diagMarkerSpan(r, "Users are account-level objects and cannot have a database or schema prefix.", 4))
	}
	validateProperties(parseText, userProps, r, &markers)
	return markers
}

// validateCreateRole validates CREATE ROLE: account-level prefix check.
func validateCreateRole(parseText string, r StatementRange) []DiagMarker {
	sig := sigToks(sqltok.Tokenize(parseText))
	name, _ := extractNameAfterCreate(sig, parseText, nil, "ROLE")
	if name != "" && strings.Contains(name, ".") {
		return []DiagMarker{diagMarkerSpan(r, "Roles are account-level objects and cannot have a database or schema prefix.", 4)}
	}
	return nil
}

// validateCreateMaskingPolicy validates CREATE MASKING POLICY: mandatory RETURNS clause.
func validateCreateMaskingPolicy(parseText string, r StatementRange) []DiagMarker {
	sig := sigToks(sqltok.Tokenize(parseText))
	if !hasKW(sig, parseText, "RETURNS") {
		return []DiagMarker{diagMarkerSpan(r, "Missing RETURNS clause in Masking Policy definition.", 4)}
	}
	return nil
}

// validateCreateStage validates CREATE STAGE properties (after stripping nested parens).
func validateCreateStage(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	validateProperties(stripParenContents(parseText), stageProps, r, &markers)
	return markers
}

// validateAlterStage validates ALTER STAGE: skip RENAME TO / UNSET / SET TAG forms,
// otherwise validate properties.
func validateAlterStage(parseText string, r StatementRange) []DiagMarker {
	sig := sigToks(sqltok.Tokenize(parseText))
	noValidate := hasKWPair(sig, parseText, "RENAME", "TO") ||
		hasKWPair(sig, parseText, "SET", "TAG") ||
		hasKW(sig, parseText, "UNSET")
	if !noValidate {
		var markers []DiagMarker
		validateProperties(stripParenContents(parseText), alterStageProps, r, &markers)
		return markers
	}
	return nil
}

// ── Shared validation helpers (DRY) ───────────────────────────────────────────

// stripStringsPreserveLen replaces string literals with spaces of the same
// byte length, preserving offsets for downstream index-based operations.
func stripStringsPreserveLen(sql string) string {
	buf := []byte(sql)
	for _, tok := range sqltok.Tokenize(sql) {
		if tok.Kind == sqltok.EOF {
			break
		}
		if tok.Kind == sqltok.StringLit {
			for i := tok.Start; i < tok.End; i++ {
				buf[i] = ' '
			}
		}
	}
	return string(buf)
}

// cleanParseText strips SQL comments and string literals, returning a trimmed
// result suitable for regex-based property/keyword detection. Comments are
// replaced with whitespace (preserving newlines) and string literals are
// replaced with a single space each. The tokenizer handles interaction between
// comments and strings correctly (e.g. apostrophes inside comments, comment
// markers inside strings).
func cleanParseText(s string) string {
	return strings.TrimSpace(sqltok.StripStrings(sqltok.StripComments(s)))
}

// stripDollarQuoted replaces dollar-quoted blocks ($$…$$ and $tag$…$tag$)
// with a single space, using the tokenizer to handle nesting correctly.
func stripDollarQuoted(sql string) string {
	tokens := sqltok.Tokenize(sql)
	var sb strings.Builder
	sb.Grow(len(sql))
	prev := 0
	for _, tok := range tokens {
		if tok.Kind == sqltok.EOF {
			break
		}
		if tok.Kind == sqltok.DollarQuoted {
			sb.WriteString(sql[prev:tok.Start])
			sb.WriteByte(' ')
			prev = tok.End
		}
	}
	sb.WriteString(sql[prev:])
	return sb.String()
}

// checkAccountLevelPrefix returns a diagnostic if the SQL identifier path
// contains a dot outside of double-quoted segments, indicating a database or
// schema prefix on an account-level object.
func checkAccountLevelPrefix(name string, r StatementRange, objType string) *DiagMarker {
	if sqlIdentPathHasDot(name) {
		m := diagMarkerSpan(r,
			objType+" are account-level objects and cannot have a database or schema prefix.", 4)
		return &m
	}
	return nil
}

// checkNameSwallowedByIF detects the case where a regex captures "IF" as the
// object name because the IF [NOT] EXISTS / IF EXISTS clause consumed the
// actual name slot. Returns the error marker and true if the name was swallowed.
func checkNameSwallowedByIF(name string, clean string, r StatementRange, reExists *regexp.Regexp, errMsg string) (DiagMarker, bool) {
	if strings.EqualFold(name, "IF") && reExists.MatchString(clean) {
		return diagMarkerSpan(r, errMsg, 4), true
	}
	return DiagMarker{}, false
}

// ── PIVOT / UNPIVOT validation ────────────────────────────────────────────────

// validatePivotClauses checks all PIVOT(...) occurrences in the statement for
// structural correctness: valid aggregate function, FOR ... IN ..., non-empty
// IN list.
func validatePivotClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigToks(sqltok.Tokenize(stripped))

	// Find all PIVOT ( occurrences in the token stream.
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], stripped) != "PIVOT" || sig[i+1].Kind != sqltok.LParen {
			continue
		}
		// Make sure this is not UNPIVOT (the U and N are separate tokens? No —
		// UNPIVOT is a single keyword token, so PIVOT here is standalone).

		// Extract balanced paren content.
		pivotBody := extractParenContentTok(sig, stripped, i)
		if pivotBody == "" {
			continue
		}

		// 1. Validate aggregate function — first ident followed by ( inside the body.
		bodySig := sigToks(sqltok.Tokenize(pivotBody))
		if len(bodySig) >= 2 && isIdent(bodySig[0]) && bodySig[1].Kind == sqltok.LParen {
			funcName := strings.ToUpper(bodySig[0].Text(pivotBody))
			if !pivotValidAggs[funcName] {
				markers = append(markers, diagMarkerSpan(r,
					"'"+funcName+"' is not a valid aggregate function for PIVOT. Use SUM, AVG, COUNT, MAX, MIN, ANY_VALUE, LISTAGG, MEDIAN, STDDEV, or VARIANCE.", 4))
			}
		}

		// 2. Check FOR ... IN ( is present in body.
		hasForIn := false
		for j := 0; j+2 < len(bodySig); j++ {
			if tokUpper(bodySig[j], pivotBody) == "FOR" {
				for k := j + 1; k+1 < len(bodySig); k++ {
					if tokUpper(bodySig[k], pivotBody) == "IN" && bodySig[k+1].Kind == sqltok.LParen {
						hasForIn = true
						// 3. Check IN list is not empty — LParen immediately followed by RParen.
						if k+2 < len(bodySig) && bodySig[k+2].Kind == sqltok.RParen {
							markers = append(markers, diagMarkerSpan(r,
								"PIVOT IN list must not be empty. Provide at least one literal value.", 4))
						}
						break
					}
				}
				break
			}
		}
		if !hasForIn {
			markers = append(markers, diagMarkerSpan(r,
				"PIVOT requires FOR <column> IN (<values>).", 4))
		}
	}
	return markers
}

// validateUnpivotClauses checks all UNPIVOT(...) occurrences in the statement
// for structural correctness: FOR ... IN ..., non-empty IN list.
func validateUnpivotClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigToks(sqltok.Tokenize(stripped))

	// Find all UNPIVOT [INCLUDE|EXCLUDE NULLS] ( occurrences.
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) != "UNPIVOT" {
			continue
		}
		// Skip optional INCLUDE/EXCLUDE NULLS.
		j := i + 1
		if j < len(sig) {
			u := tokUpper(sig[j], stripped)
			if u == "INCLUDE" || u == "EXCLUDE" {
				j++
				if j < len(sig) && tokUpper(sig[j], stripped) == "NULLS" {
					j++
				}
			}
		}
		if j >= len(sig) || sig[j].Kind != sqltok.LParen {
			continue
		}

		// Extract balanced paren content.
		unpivotBody := extractParenContentTok(sig, stripped, j-1)
		if unpivotBody == "" {
			continue
		}

		// Tokenize the body for structural checks.
		bodySig := sigToks(sqltok.Tokenize(unpivotBody))

		// 1. Check FOR ... IN ( is present.
		hasForIn := false
		for k := 0; k+2 < len(bodySig); k++ {
			if tokUpper(bodySig[k], unpivotBody) == "FOR" {
				for m := k + 1; m+1 < len(bodySig); m++ {
					if tokUpper(bodySig[m], unpivotBody) == "IN" && bodySig[m+1].Kind == sqltok.LParen {
						hasForIn = true
						// 2. Check IN list is not empty.
						if m+2 < len(bodySig) && bodySig[m+2].Kind == sqltok.RParen {
							markers = append(markers, diagMarkerSpan(r,
								"UNPIVOT IN list must not be empty. Provide at least one column name.", 4))
						}
						break
					}
				}
				break
			}
		}
		if !hasForIn {
			markers = append(markers, diagMarkerSpan(r,
				"UNPIVOT requires FOR <name_column> IN (<columns>).", 4))
		}
	}
	return markers
}

// ── MATCH_RECOGNIZE validation ────────────────────────────────────────────────

// validateMatchRecognizeClauses checks all MATCH_RECOGNIZE(...) occurrences in
// the statement for structural correctness:
//   - mandatory PATTERN clause with at least one variable
//   - mandatory DEFINE clause
//   - ONE ROW PER MATCH / ALL ROWS PER MATCH mutual exclusion
//   - AFTER MATCH SKIP target validity
func validateMatchRecognizeClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	clean := cleanParseText(stripped)

	sig := sigToks(sqltok.Tokenize(clean))

	// Find all MATCH_RECOGNIZE ( occurrences at the top level.
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], clean) != "MATCH_RECOGNIZE" || sig[i+1].Kind != sqltok.LParen {
			continue
		}
		// Extract balanced paren body as tokens.
		bodyStart := i + 2
		depth := 1
		bodyEnd := bodyStart
		for j := bodyStart; j < len(sig); j++ {
			switch sig[j].Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
				if depth == 0 {
					bodyEnd = j
					goto foundBody
				}
			}
		}
		continue
	foundBody:
		body := sig[bodyStart:bodyEnd]

		// 1. Validate mandatory PATTERN clause.
		patternIdx := findKWLParen(body, clean, "PATTERN")
		if patternIdx < 0 {
			markers = append(markers, diagMarkerSpan(r,
				"MATCH_RECOGNIZE requires a PATTERN clause.", 4))
		} else {
			// Check for empty PATTERN — LParen immediately followed by RParen.
			lpIdx := patternIdx + 1 // the LParen after PATTERN
			if lpIdx < len(body) && lpIdx+1 < len(body) && body[lpIdx+1].Kind == sqltok.RParen {
				// Verify the RParen matches this LParen (depth=1).
				markers = append(markers, diagMarkerSpan(r,
					"MATCH_RECOGNIZE PATTERN must contain at least one pattern variable.", 4))
			}
		}

		// 2. Validate mandatory DEFINE clause (DEFINE must be followed by at least
		//    one binding — bare "DEFINE)" is treated as missing).
		hasDefine := false
		for j := 0; j < len(body); j++ {
			if tokUpper(body[j], clean) == "DEFINE" && j+1 < len(body) {
				hasDefine = true
				break
			}
		}
		if !hasDefine {
			markers = append(markers, diagMarkerSpan(r,
				"MATCH_RECOGNIZE requires a DEFINE clause to bind pattern variables.", 4))
		}

		// 3. ONE ROW PER MATCH / ALL ROWS PER MATCH mutual exclusion.
		hasOneRow := hasKWSeq4(body, clean, "ONE", "ROW", "PER", "MATCH")
		hasAllRows := hasKWSeq4(body, clean, "ALL", "ROWS", "PER", "MATCH")
		if hasOneRow && hasAllRows {
			markers = append(markers, diagMarkerSpan(r,
				"ONE ROW PER MATCH and ALL ROWS PER MATCH are mutually exclusive. Use one or the other.", 4))
		}

		// 4. AFTER MATCH SKIP target validation.
		for j := 0; j+2 < len(body); j++ {
			if tokUpper(body[j], clean) == "AFTER" &&
				tokUpper(body[j+1], clean) == "MATCH" &&
				tokUpper(body[j+2], clean) == "SKIP" {
				// Collect target tokens until a boundary keyword or end.
				target := j + 3
				end := len(body)
				for k := target; k < len(body); k++ {
					u := tokUpper(body[k], clean)
					if u == "PATTERN" || u == "DEFINE" || u == "MEASURES" ||
						u == "ONE" || u == "ALL" || u == "ORDER" || u == "PARTITION" {
						end = k
						break
					}
				}
				if target < end {
					targetToks := body[target:end]
					if !isValidAfterMatchSkipTarget(targetToks, clean) {
						markers = append(markers, diagMarkerSpan(r,
							"Invalid AFTER MATCH SKIP target. Use TO NEXT ROW, PAST LAST ROW, TO FIRST <variable>, or TO LAST <variable>.", 4))
					}
				}
				break
			}
		}
	}
	return markers
}

// isValidAfterMatchSkipTarget checks if the token sequence represents a valid
// AFTER MATCH SKIP target: TO NEXT ROW, PAST LAST ROW, TO FIRST <ident>, TO LAST <ident>.
func isValidAfterMatchSkipTarget(toks []sqltok.Token, sql string) bool {
	if len(toks) < 2 {
		return false
	}
	first := tokUpper(toks[0], sql)
	switch first {
	case "TO":
		if len(toks) >= 3 && tokUpper(toks[1], sql) == "NEXT" && tokUpper(toks[2], sql) == "ROW" {
			return true
		}
		if len(toks) >= 3 && (tokUpper(toks[1], sql) == "FIRST" || tokUpper(toks[1], sql) == "LAST") && isIdent(toks[2]) {
			return true
		}
	case "PAST":
		if len(toks) >= 3 && tokUpper(toks[1], sql) == "LAST" && tokUpper(toks[2], sql) == "ROW" {
			return true
		}
	}
	return false
}

// stripClusterByParens removes CLUSTER BY (...) clauses from the text to prevent
// false positive syntax errors on Snowflake-specific DDL.
func stripClusterByParens(s string) string {
	toks := sqltok.Tokenize(s)
	sig := sigToks(toks)
	// Find CLUSTER BY ( positions and collect byte ranges to remove.
	type spanRange struct{ start, end int }
	var ranges []spanRange
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], s) == "CLUSTER" && tokUpper(sig[i+1], s) == "BY" &&
			sig[i+2].Kind == sqltok.LParen {
			// Find the matching close paren.
			depth := 1
			for j := i + 3; j < len(sig); j++ {
				switch sig[j].Kind {
				case sqltok.LParen:
					depth++
				case sqltok.RParen:
					depth--
					if depth == 0 {
						ranges = append(ranges, spanRange{sig[i].Start, sig[j].End})
						i = j
						goto nextCluster
					}
				}
			}
		nextCluster:
		}
	}
	if len(ranges) == 0 {
		return s
	}
	var b strings.Builder
	prev := 0
	for _, r := range ranges {
		b.WriteString(s[prev:r.start])
		prev = r.end
	}
	b.WriteString(s[prev:])
	return b.String()
}

// ── CREATE FUNCTION / PROCEDURE helpers ──────────────────────────────────────

// findFuncBodyAS finds the index of the AS keyword that introduces the function/
// procedure body, skipping "EXECUTE AS" which is a different construct. Returns -1
// if no AS body is found.
func findFuncBodyAS(sig []sqltok.Token, sql string) int {
	for i := 0; i < len(sig); i++ {
		upper := tokUpper(sig[i], sql)
		if upper != "AS" {
			// Handle AS$$ / AS$tag$ glued to dollar-quote (no space).
			if len(upper) > 2 && upper[:2] == "AS" && upper[2] == '$' {
				if i > 0 && tokUpper(sig[i-1], sql) == "EXECUTE" {
					continue
				}
				return i
			}
			continue
		}
		// Skip EXECUTE AS — check if the previous significant token is EXECUTE.
		if i > 0 && tokUpper(sig[i-1], sql) == "EXECUTE" {
			continue
		}
		return i
	}
	return -1
}

// findLanguage finds LANGUAGE <ident> in the preamble and returns the uppercased
// language name, or "" if not found.
func findLanguage(sig []sqltok.Token, sql string) string {
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], sql) == "LANGUAGE" {
			next := sig[i+1]
			if isIdent(next) || next.Kind == sqltok.NumberLit {
				return strings.ToUpper(next.Text(sql))
			}
		}
	}
	return ""
}

// checkPythonRequirements validates Python-specific requirements in the preamble:
// RUNTIME_VERSION, HANDLER, and PACKAGES or IMPORTS.
func checkPythonRequirements(sig []sqltok.Token, sql string, r StatementRange, markers *[]DiagMarker, objType string) {
	if !hasKW(sig, sql, "RUNTIME_VERSION") {
		*markers = append(*markers, diagMarkerSpan(r, "RUNTIME_VERSION is required for PYTHON "+objType+".", 4))
	}
	if objType == "functions" && !hasKW(sig, sql, "HANDLER") {
		*markers = append(*markers, diagMarkerSpan(r, "HANDLER is required for PYTHON "+objType+".", 4))
	}
	if !hasKW(sig, sql, "PACKAGES") && !hasKW(sig, sql, "IMPORTS") {
		*markers = append(*markers, diagMarkerSpan(r, "PACKAGES or IMPORTS is required for PYTHON "+objType+".", 4))
	}
}

// checkNullInputHandling validates mutual exclusion of null input handling clauses:
// CALLED ON NULL INPUT vs RETURNS NULL ON NULL INPUT vs STRICT.
func checkNullInputHandling(sig []sqltok.Token, sql string, r StatementRange, markers *[]DiagMarker) {
	hasCalledOnNull := hasKWSeq4(sig, sql, "CALLED", "ON", "NULL", "INPUT")
	hasReturnsNull := false
	for i := 0; i+4 < len(sig); i++ {
		if tokUpper(sig[i], sql) == "RETURNS" &&
			tokUpper(sig[i+1], sql) == "NULL" &&
			tokUpper(sig[i+2], sql) == "ON" &&
			tokUpper(sig[i+3], sql) == "NULL" &&
			tokUpper(sig[i+4], sql) == "INPUT" {
			hasReturnsNull = true
			break
		}
	}
	hasStrict := hasKW(sig, sql, "STRICT")

	if hasCalledOnNull && (hasReturnsNull || hasStrict) {
		*markers = append(*markers, diagMarkerSpan(r, "CALLED ON NULL INPUT and RETURNS NULL ON NULL INPUT (or STRICT) are mutually exclusive.", 4))
	}
	if hasReturnsNull && hasStrict {
		*markers = append(*markers, diagMarkerSpan(r, "STRICT and RETURNS NULL ON NULL INPUT are redundant.", 4))
	}
}

// ── ASOF JOIN validation ───────────────────────────────────────────────────────

// validateAsofJoinClauses checks all ASOF JOIN occurrences in the statement for
// structural correctness:
//   - mandatory MATCH_CONDITION clause (unless USING FUNCTION form is used)
//   - comparison operator inside MATCH_CONDITION must be >=, >, <=, or <
//   - ON and USING clauses are not valid with ASOF JOIN
func validateAsofJoinClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	clean := cleanParseText(stripped)
	sig := sigToks(sqltok.Tokenize(clean))

	// Find all top-level ASOF JOIN positions (skip matches inside parens).
	type asofPos struct{ afterIdx int } // index into sig after "JOIN"
	var asofPositions []asofPos
	depth := 0
	for i := 0; i+1 < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(sig[i], clean) == "ASOF" && tokUpper(sig[i+1], clean) == "JOIN" {
				asofPositions = append(asofPositions, asofPos{afterIdx: i + 2})
			}
		}
	}

	for idx, ap := range asofPositions {
		// Scope: tokens after this ASOF JOIN up to the next top-level ASOF JOIN.
		scopeEnd := len(sig)
		if idx+1 < len(asofPositions) {
			scopeEnd = asofPositions[idx+1].afterIdx - 2
		}
		scope := sig[ap.afterIdx:scopeEnd]

		hasMatchCondition := findKWLParen(scope, clean, "MATCH_CONDITION") >= 0
		hasUsingFunction := hasUsingFunctionTok(scope, clean)

		// 1. Check for invalid ON clause.
		flaggedOnOrUsing := false
		if hasOnClauseTok(scope, clean, hasMatchCondition) {
			markers = append(markers, diagMarkerSpan(r,
				"ON clause is not valid with ASOF JOIN. Use MATCH_CONDITION instead.", 4))
			flaggedOnOrUsing = true
		}

		// 2. Check for invalid USING clause (plain USING, not USING FUNCTION).
		if hasUsingClauseTok(scope, clean, hasUsingFunction) {
			markers = append(markers, diagMarkerSpan(r,
				"USING clause is not valid with ASOF JOIN. Use MATCH_CONDITION instead.", 4))
			flaggedOnOrUsing = true
		}

		// 3. Validate MATCH_CONDITION or USING FUNCTION is present.
		if !hasMatchCondition && !hasUsingFunction && !flaggedOnOrUsing {
			markers = append(markers, diagMarkerSpan(r,
				"ASOF JOIN requires a MATCH_CONDITION clause. Use ASOF JOIN <table> MATCH_CONDITION (<left_expr> >= <right_expr>).", 4))
			continue
		}

		// 4. If MATCH_CONDITION is present, validate the comparison operator.
		if hasMatchCondition {
			mcIdx := findKWLParen(scope, clean, "MATCH_CONDITION")
			if mcIdx >= 0 {
				// Check if paren is properly closed (balanced).
				matched := false
				if mcIdx+1 < len(scope) && scope[mcIdx+1].Kind == sqltok.LParen {
					depth := 1
					for j := mcIdx + 2; j < len(scope); j++ {
						switch scope[j].Kind {
						case sqltok.LParen:
							depth++
						case sqltok.RParen:
							depth--
							if depth == 0 {
								matched = true
							}
						}
						if matched {
							break
						}
					}
				}
				if matched {
					mcBody := extractParenContentTok(scope, clean, mcIdx)
					// Empty body or body without valid comparison.
					if !containsAsofValidComparison(mcBody) {
						markers = append(markers, diagMarkerSpan(r,
							"MATCH_CONDITION comparison must use one of: >=, >, <=, <. Operators =, <>, != are not supported.", 4))
					}
				}
			}
		}
	}
	return markers
}

// hasOnClauseTok checks if a top-level ON keyword appears in the token scope,
// excluding ON that appears after MATCH_CONDITION.
func hasOnClauseTok(scope []sqltok.Token, sql string, hasMatchCondition bool) bool {
	mcIdx := -1
	if hasMatchCondition {
		mcIdx = findKWLParen(scope, sql, "MATCH_CONDITION")
	}
	depth := 0
	for i := 0; i < len(scope); i++ {
		switch scope[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(scope[i], sql) == "ON" {
				if mcIdx >= 0 && i > mcIdx {
					continue
				}
				return true
			}
		}
	}
	return false
}

// hasUsingClauseTok checks if USING ( appears at the top level, and it's not
// the USING (func(...)) function form.
func hasUsingClauseTok(scope []sqltok.Token, sql string, hasUsingFunction bool) bool {
	if hasUsingFunction {
		return false
	}
	depth := 0
	for i := 0; i < len(scope); i++ {
		switch scope[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && tokUpper(scope[i], sql) == "USING" &&
				i+1 < len(scope) && scope[i+1].Kind == sqltok.LParen {
				return true
			}
		}
	}
	return false
}

// hasUsingFunctionTok checks for the USING (func_name(...)) pattern in scope.
// This detects: USING ( ident [. ident]* (
func hasUsingFunctionTok(scope []sqltok.Token, sql string) bool {
	for i := 0; i+2 < len(scope); i++ {
		if tokUpper(scope[i], sql) != "USING" || scope[i+1].Kind != sqltok.LParen {
			continue
		}
		// After '(' look for ident [.ident]* (
		j := i + 2
		if j >= len(scope) || !isIdent(scope[j]) {
			continue
		}
		j++
		for j+1 < len(scope) && scope[j].Kind == sqltok.Dot && isIdent(scope[j+1]) {
			j += 2
		}
		if j < len(scope) && scope[j].Kind == sqltok.LParen {
			return true
		}
	}
	return false
}

// containsAsofValidComparison checks whether the MATCH_CONDITION body contains
// one of the valid comparison operators (>=, >, <=, <) and does NOT contain only
// invalid operators (=, <>, !=).
func containsAsofValidComparison(body string) bool {
	for i := 0; i < len(body); i++ {
		ch := body[i]
		switch ch {
		case '>':
			if i+1 < len(body) && body[i+1] == '=' {
				return true // >=
			}
			return true // >
		case '<':
			if i+1 < len(body) && body[i+1] == '>' {
				i++ // <> — invalid, skip past '>'
				continue
			}
			if i+1 < len(body) && body[i+1] == '=' {
				return true // <=
			}
			return true // <
		case '!':
			if i+1 < len(body) && body[i+1] == '=' {
				i++ // != — invalid, skip past '='
				continue
			}
		case '=':
			// Bare = — invalid, skip
			continue
		}
	}
	return false
}

// ── INSERT ALL / INSERT FIRST / INSERT OVERWRITE validation ───────────────────

// validateInsertAll validates INSERT [OVERWRITE] ALL statements.
// Rules:
//   - At least one INTO clause is required
//   - If WHEN branches are present, at least one is required (ELSE alone is invalid)
//   - Each WHEN branch must contain a THEN INTO
//   - A trailing SELECT is mandatory
func validateInsertAll(stripped string, r StatementRange) []DiagMarker {
	return validateInsertMultiTable("ALL", stripped, r)
}

// validateInsertFirst validates INSERT [OVERWRITE] FIRST statements.
// Rules:
//   - At least one WHEN branch is required (INSERT FIRST always requires conditions)
//   - Each WHEN branch must contain a THEN INTO
//   - A trailing SELECT is mandatory
func validateInsertFirst(stripped string, r StatementRange) []DiagMarker {
	return validateInsertMultiTable("FIRST", stripped, r)
}

// validateInsertMultiTable is the shared implementation for INSERT ALL and
// INSERT FIRST validation.
func validateInsertMultiTable(keyword string, stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	clean := strings.TrimSpace(sqltok.StripStrings(stripped))
	upper := strings.ToUpper(clean)

	// Find the position of the trailing top-level SELECT. Keywords after
	// this position belong to the source query and must be ignored (e.g.
	// CASE WHEN/ELSE inside the SELECT clause).
	trailingSelectPos := findLastTopLevelSelectPos(upper)

	// Scan for top-level WHEN, ELSE, and INTO keywords (depth == 0) that
	// appear before the trailing SELECT.
	scanLimit := len(upper)
	if trailingSelectPos >= 0 {
		scanLimit = trailingSelectPos
	}

	whenPositions, hasElse, hasInto := scanInsertMultiKeywords(upper, scanLimit)
	hasWhen := len(whenPositions) > 0

	// Determine if this is a conditional form (has WHEN or ELSE) or
	// unconditional (bare INSERT ALL INTO ... INTO ... SELECT).
	isConditional := hasWhen || hasElse

	if keyword == "FIRST" {
		// INSERT FIRST always requires WHEN branches.
		if !hasWhen {
			markers = append(markers, diagMarkerSpan(r,
				"INSERT FIRST requires at least one WHEN branch. Use WHEN <condition> THEN INTO <table>.", 4))
			return markers
		}
	}

	if isConditional {
		// Conditional form: require at least one WHEN (ELSE alone is invalid).
		if !hasWhen {
			markers = append(markers, diagMarkerSpan(r,
				"INSERT "+keyword+" requires at least one WHEN branch when using conditional insert. Use WHEN <condition> THEN INTO <table>.", 4))
			return markers
		}

		// Each WHEN must have a THEN INTO.
		for i, whenPos := range whenPositions {
			end := scanLimit
			if i+1 < len(whenPositions) {
				end = whenPositions[i+1]
			}
			clause := upper[whenPos:end]
			clauseSig := sigToks(sqltok.Tokenize(clause))
			if !hasKWPair(clauseSig, clause, "THEN", "INTO") {
				markers = append(markers, diagMarkerSpan(r,
					"WHEN branch must contain INTO clause. Use WHEN <condition> THEN INTO <table>.", 4))
			}
		}
	} else {
		// Unconditional form: require at least one INTO.
		if !hasInto {
			markers = append(markers, diagMarkerSpan(r,
				"INSERT "+keyword+" requires at least one INTO clause.", 4))
			return markers
		}
	}

	// Trailing SELECT is mandatory for all multi-table inserts.
	if trailingSelectPos < 0 {
		markers = append(markers, diagMarkerSpan(r,
			"INSERT "+keyword+" requires a source SELECT at the end of the statement.", 4))
	}

	return markers
}

// scanInsertMultiKeywords scans upper (already uppercased) up to scanLimit for
// top-level (depth == 0) WHEN, ELSE, and INTO keywords. Returns the positions
// of each WHEN, and whether ELSE and INTO were found.
// Note: word-boundary checks peek one character past the keyword end using
// len(upper) (not scanLimit) because we need to verify the character after the
// keyword is not a word character. This is safe — the keyword start is always
// within scanLimit, and we only read (not match) one byte beyond.
func scanInsertMultiKeywords(upper string, scanLimit int) (whenPositions []int, hasElse bool, hasInto bool) {
	depth := 0
	for i := 0; i < scanLimit; i++ {
		switch upper[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case 'W':
			if depth == 0 && i+4 <= scanLimit && upper[i:i+4] == "WHEN" {
				if i > 0 && isWordChar(rune(upper[i-1])) {
					continue
				}
				if i+4 < len(upper) && isWordChar(rune(upper[i+4])) {
					continue
				}
				whenPositions = append(whenPositions, i)
			}
		case 'E':
			if depth == 0 && i+4 <= scanLimit && upper[i:i+4] == "ELSE" {
				if i > 0 && isWordChar(rune(upper[i-1])) {
					continue
				}
				if i+4 < len(upper) && isWordChar(rune(upper[i+4])) {
					continue
				}
				hasElse = true
			}
		case 'I':
			if depth == 0 && i+4 <= scanLimit && upper[i:i+4] == "INTO" {
				if i > 0 && isWordChar(rune(upper[i-1])) {
					continue
				}
				if i+4 < len(upper) && isWordChar(rune(upper[i+4])) {
					continue
				}
				hasInto = true
			}
		}
	}
	return
}

// findLastTopLevelSelectPos returns the position of the last top-level SELECT
// keyword (depth == 0) in s, or -1 if none found.
func findLastTopLevelSelectPos(upper string) int {
	depth := 0
	lastPos := -1
	for i := 0; i < len(upper); i++ {
		switch upper[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case 'S':
			if depth == 0 && i+6 <= len(upper) && upper[i:i+6] == "SELECT" {
				if i > 0 && isWordChar(rune(upper[i-1])) {
					continue
				}
				if i+6 < len(upper) && isWordChar(rune(upper[i+6])) {
					continue
				}
				lastPos = i
			}
		}
	}
	return lastPos
}

// validateInsertOverwrite validates INSERT OVERWRITE INTO statements.
// Rules:
//   - INTO is required after OVERWRITE (bare INSERT OVERWRITE <table> is invalid)
//   - A source SELECT or VALUES is required
func validateInsertOverwrite(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigToks(sqltok.Tokenize(stripped))
	if len(sig) < 2 {
		return nil
	}

	// If this is actually INSERT OVERWRITE ALL or INSERT OVERWRITE FIRST,
	// those are handled by validateInsertAll/validateInsertFirst.
	// Check: INSERT [OVERWRITE] ALL/FIRST
	idx := 1 // start after INSERT
	if tokUpper(sig[idx], stripped) == "OVERWRITE" {
		idx++
	}
	if idx < len(sig) {
		kw := tokUpper(sig[idx], stripped)
		if kw == "ALL" || kw == "FIRST" {
			return nil
		}
	}

	// INSERT OVERWRITE must be followed by INTO.
	// sig[0]=INSERT, sig[1]=OVERWRITE — check sig[2] for INTO.
	if len(sig) < 3 || tokUpper(sig[2], stripped) != "INTO" {
		markers = append(markers, diagMarkerSpan(r,
			"INSERT OVERWRITE requires INTO. Use INSERT OVERWRITE INTO <table>.", 4))
		return markers
	}

	// Check for a source: SELECT or VALUES must appear after the table name
	// and optional column list.
	i := 3 // after INTO
	if i < len(sig) && isIdent(sig[i]) {
		_, i = readIdentPath(sig, stripped, i)
	}
	// Skip optional column list (parenthesized).
	if i < len(sig) && sig[i].Kind == sqltok.LParen {
		depth := 0
		for ; i < len(sig); i++ {
			switch sig[i].Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
				if depth == 0 {
					i++
					goto doneColList
				}
			}
		}
	doneColList:
	}

	hasSource := false
	for ; i < len(sig); i++ {
		u := tokUpper(sig[i], stripped)
		if u == "SELECT" || u == "VALUES" {
			hasSource = true
			break
		}
	}
	if !hasSource {
		markers = append(markers, diagMarkerSpan(r,
			"INSERT OVERWRITE INTO requires a source SELECT or VALUES clause.", 4))
	}

	return markers
}

// findTopLevelMatches returns all matches of re in s that are not inside
// parenthesized groups. This prevents nested ASOF JOINs inside subqueries
// from corrupting the scope splitting of the outer ASOF JOIN.
func findTopLevelMatches(s string, re *regexp.Regexp) [][]int {
	allLocs := re.FindAllStringIndex(s, -1)
	if len(allLocs) == 0 {
		return nil
	}
	// Precompute paren depth at every position that starts a match.
	// We scan once to build a depth array up to the last match start.
	maxPos := allLocs[len(allLocs)-1][0]
	depth := 0
	depthAt := make(map[int]int, len(allLocs))
	matchIdx := 0
	for i := 0; i <= maxPos && matchIdx < len(allLocs); i++ {
		if i == allLocs[matchIdx][0] {
			depthAt[i] = depth
			matchIdx++
		}
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}

	var result [][]int
	for _, loc := range allLocs {
		if depthAt[loc[0]] == 0 {
			result = append(result, loc)
		}
	}
	return result
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

	strippedS := sqltok.StripStrings(s)
	sig := sigToks(sqltok.Tokenize(strippedS))

	for i := 0; i+1 < len(sig); i++ {
		if (sig[i].Kind == sqltok.Keyword || sig[i].Kind == sqltok.Identifier) &&
			sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(strippedS) == "=" {
			key := sig[i].Text(strippedS)
			if !reValid.MatchString(key) {
				*markers = append(*markers, diagMarkerSpan(r, fmt.Sprintf("Unexpected property '%s' in statement.", key), 4))
			}
		}
	}
}

func validateCreateAlert(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Mutually exclusive OR REPLACE and IF NOT EXISTS.
	// Restrict check to tokens before the IF ( EXISTS ( condition clause.
	ifExistsIdx := -1
	for i := 0; i+3 < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "IF" &&
			sig[i+1].Kind == sqltok.LParen &&
			tokUpper(sig[i+2], stripped) == "EXISTS" &&
			sig[i+3].Kind == sqltok.LParen {
			ifExistsIdx = i
			break
		}
	}

	if ifExistsIdx >= 0 {
		preambleSig := sig[:ifExistsIdx]
		if marker, conflict := checkOrReplaceConflictTok(preambleSig, stripped, r, "CREATE ALERT"); conflict {
			markers = append(markers, marker)
			return markers
		}
	} else {
		if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE ALERT"); conflict {
			markers = append(markers, marker)
			return markers
		}
	}

	// 2. Mandatory IF (EXISTS (...))
	if ifExistsIdx < 0 {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory IF (EXISTS (...)) clause in CREATE ALERT statement.", 4))
	}

	// 3. Mandatory THEN — must appear at paren depth 0 after the IF clause.
	if ifExistsIdx >= 0 {
		hasThen := false
		depth := 0
		for i := ifExistsIdx; i < len(sig); i++ {
			switch sig[i].Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
			default:
				if depth == 0 && tokUpper(sig[i], stripped) == "THEN" {
					hasThen = true
				}
			}
			if hasThen {
				break
			}
		}
		if !hasThen {
			markers = append(markers, diagMarkerSpan(r, "Missing mandatory THEN keyword in CREATE ALERT statement.", 4))
		}
	}

	// 4. Mandatory WAREHOUSE (in preamble before IF clause)
	preambleSig := sig
	if ifExistsIdx >= 0 {
		preambleSig = sig[:ifExistsIdx]
	}
	if !hasKWAssign(preambleSig, stripped, "WAREHOUSE") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory WAREHOUSE property in CREATE ALERT statement.", 4))
	}

	// 5. Mandatory SCHEDULE
	if !hasKWAssign(preambleSig, stripped, "SCHEDULE") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory SCHEDULE property in CREATE ALERT statement.", 4))
	}

	// 6. Validate properties
	preamble := parseText
	if ifExistsIdx >= 0 {
		preamble = stripped[:sig[ifExistsIdx].Start]
	}
	validateProperties(preamble, alertProps, r, &markers)

	return markers
}

func validateCreateNetworkPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Account-level: name must not have a database or schema prefix.
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "POLICY" && i+1 < len(sig) && isIdent(sig[i+1]) {
			name, _ := readIdentPath(sig, stripped, i+1)
			if pfx := checkAccountLevelPrefix(name, r, "Network policies"); pfx != nil {
				markers = append(markers, *pfx)
			}
			break
		}
	}

	// 2. At least one of ALLOWED_IP_LIST or ALLOWED_NETWORK_RULE_LIST must be present
	// and non-empty.
	allowedIPContent := findKWAssignParenContent(sig, stripped, "ALLOWED_IP_LIST")
	hasAllowedIP := allowedIPContent != "" && networkPolicyListHasEntries(allowedIPContent)
	allowedRulesContent := findKWAssignParenContent(sig, stripped, "ALLOWED_NETWORK_RULE_LIST")
	hasAllowedRules := allowedRulesContent != "" && networkPolicyListHasEntries(allowedRulesContent)
	if !hasAllowedIP && !hasAllowedRules {
		markers = append(markers, diagMarkerSpan(r, "Network policy has no effect: at least one of ALLOWED_IP_LIST or ALLOWED_NETWORK_RULE_LIST must be specified and non-empty.", 4))
	}

	// 3. Validate IP lists and collect IPs for overlap check.
	var allowedIPs []string
	var blockedIPs []string
	for _, listKind := range []string{"ALLOWED_IP_LIST", "BLOCKED_IP_LIST"} {
		listContent := findKWAssignParenContent(sig, stripped, listKind)
		if listContent == "" {
			continue
		}
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
	// BLOCKED_IP_LIST.
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Account-level: object name must not have a database or schema prefix.
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "POLICY" && i+1 < len(sig) && isIdent(sig[i+1]) {
			name, _ := readIdentPath(sig, stripped, i+1)
			if pfx := checkAccountLevelPrefix(name, r, "Session policies"); pfx != nil {
				markers = append(markers, *pfx)
			}
			break
		}
	}

	// 2. Validate SESSION_IDLE_TIMEOUT_MINS range (0–56400).
	if v, ok := findKWAssignInt(sig, stripped, "SESSION_IDLE_TIMEOUT_MINS"); ok {
		if v < 0 || v > 56400 {
			markers = append(markers, diagMarkerSpan(r, fmt.Sprintf("SESSION_IDLE_TIMEOUT_MINS value %d is out of range (0–56400). Use 0 to disable the timeout.", v), 4))
		}
	}

	// 3. Validate SESSION_UI_IDLE_TIMEOUT_MINS range (0–56400).
	if v, ok := findKWAssignInt(sig, stripped, "SESSION_UI_IDLE_TIMEOUT_MINS"); ok {
		if v < 0 || v > 56400 {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Account-level: object name must not have a database or schema prefix.
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "POLICY" && i+1 < len(sig) && isIdent(sig[i+1]) {
			name, _ := readIdentPath(sig, stripped, i+1)
			if pfx := checkAccountLevelPrefix(name, r, "Password policies"); pfx != nil {
				markers = append(markers, *pfx)
			}
			break
		}
	}

	// 2. Per-property range validation.
	type intProp struct {
		name string
		min  int
		max  int // -1 means no upper bound
	}

	props := []intProp{
		{"PASSWORD_MIN_LENGTH", 8, 256},
		{"PASSWORD_MAX_LENGTH", 8, 256},
		{"PASSWORD_MIN_UPPER_CASE_CHARS", 0, 256},
		{"PASSWORD_MIN_LOWER_CASE_CHARS", 0, 256},
		{"PASSWORD_MIN_NUMERIC_CHARS", 0, 256},
		{"PASSWORD_MIN_SPECIAL_CHARS", 0, 256},
		{"PASSWORD_MIN_AGE_DAYS", 0, 999},
		{"PASSWORD_MAX_AGE_DAYS", 0, 999},
		{"PASSWORD_MAX_RETRIES", 1, 10},
		{"PASSWORD_LOCKOUT_TIME_MINS", 1, 999},
		{"PASSWORD_HISTORY", 0, 24},
	}

	values := make(map[string]int)
	for _, p := range props {
		v, ok := findKWAssignInt(sig, stripped, p.name)
		if !ok {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	// Restrict check to preamble before AS ( so that IF in the policy body
	// is not mistaken for the DDL modifier.
	asIdx := findKWLParen(sig, stripped, "AS")
	preambleSig := sig
	if asIdx >= 0 {
		preambleSig = sig[:asIdx]
	}
	if marker, conflict := checkOrReplaceConflictTok(preambleSig, stripped, r, "CREATE ROW ACCESS POLICY"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Mandatory AS (<arg_name> <arg_type> [, ...]) parameter list.
	if asIdx < 0 {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS (<arg_name> <arg_type> [, ...]) parameter list in CREATE ROW ACCESS POLICY.", 4))
	} else {
		// Extract content inside AS ( ... ) respecting paren depth.
		paramContent := extractParenContentTok(sig, stripped, asIdx)
		paramContent = strings.TrimSpace(paramContent)
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
	if !hasKWPair(sig, stripped, "RETURNS", "BOOLEAN") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS BOOLEAN clause in CREATE ROW ACCESS POLICY.", 4))
	}

	// 4. Mandatory -> separator between signature and body.
	if !hasKWPairArrow(sig, stripped, "RETURNS", "BOOLEAN") {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	asIdx := findKWLParen(sig, stripped, "AS")
	preambleSig := sig
	if asIdx >= 0 {
		preambleSig = sig[:asIdx]
	}
	if marker, conflict := checkOrReplaceConflictTok(preambleSig, stripped, r, "CREATE AGGREGATION POLICY"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Mandatory AS () parameter list.
	if asIdx < 0 {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS () clause in CREATE AGGREGATION POLICY.", 4))
	}

	// 3. Mandatory RETURNS AGGREGATION_CONSTRAINT clause.
	if !hasKWPair(sig, stripped, "RETURNS", "AGGREGATION_CONSTRAINT") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS AGGREGATION_CONSTRAINT clause in CREATE AGGREGATION POLICY.", 4))
	}

	// 4. Mandatory -> separator between signature and body.
	if !hasKWPairArrow(sig, stripped, "RETURNS", "AGGREGATION_CONSTRAINT") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory '->' separator between signature and body in CREATE AGGREGATION POLICY.", 4))
	}

	// 5. Validate MIN_GROUP_SIZE range (1–1 000 000) if present.
	// MIN_GROUP_SIZE => value (uses fat arrow =>)
	if val, ok := findKWFatArrowInt(sig, stripped, "MIN_GROUP_SIZE"); ok {
		if val < 1 || val > 1000000 {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	asIdx := findKWLParen(sig, stripped, "AS")
	preambleSig := sig
	if asIdx >= 0 {
		preambleSig = sig[:asIdx]
	}
	if marker, conflict := checkOrReplaceConflictTok(preambleSig, stripped, r, "CREATE PROJECTION POLICY"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Mandatory AS () parameter list.
	if asIdx < 0 {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory AS () clause in CREATE PROJECTION POLICY.", 4))
	}

	// 3. Mandatory RETURNS PROJECTION_CONSTRAINT clause.
	if !hasKWPair(sig, stripped, "RETURNS", "PROJECTION_CONSTRAINT") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory RETURNS PROJECTION_CONSTRAINT clause in CREATE PROJECTION POLICY.", 4))
	}

	// 4. Mandatory -> separator between signature and body.
	if !hasKWPairArrow(sig, stripped, "RETURNS", "PROJECTION_CONSTRAINT") {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory '->' separator between signature and body in CREATE PROJECTION POLICY.", 4))
	}

	// 5. Validate ALLOW value if present: must be 'none' or 'transformation'.
	if val, ok := findKWFatArrowStr(sig, stripped, "ALLOW"); ok {
		lower := strings.ToLower(val)
		if lower != "none" && lower != "transformation" {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("ALLOW value '%s' is invalid; must be 'none' or 'transformation'.", val), 4))
		}
	}

	return markers
}

// ── validateAlterAggregationOrProjectionPolicy ───────────────────────────────

// validateAlterAggregationOrProjectionPolicy checks structural requirements for
// ALTER AGGREGATION POLICY or ALTER PROJECTION POLICY statements.
func validateAlterAggregationOrProjectionPolicy(parseText string, r StatementRange, policyType string) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	hasSetBody := hasKWPair(sig, stripped, "SET", "BODY")
	hasSetComment := hasKWPair(sig, stripped, "SET", "COMMENT")
	hasUnsetComment := hasKWPair(sig, stripped, "UNSET", "COMMENT")
	hasRenameTo := hasKWPair(sig, stripped, "RENAME", "TO")

	if !hasSetBody && !hasSetComment && !hasUnsetComment && !hasRenameTo {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))
	// DROP <policyType> POLICY [IF EXISTS] <name>
	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", policyType, "POLICY")
	if name == "" {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE PACKAGES POLICY"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. LANGUAGE is mandatory and must be PYTHON.
	lang, hasLang := findKWFollowedByIdent(sig, stripped, "LANGUAGE")
	if !hasLang {
		markers = append(markers, diagMarkerSpan(r, "Missing mandatory LANGUAGE clause in CREATE PACKAGES POLICY. Only LANGUAGE PYTHON is supported.", 4))
	} else if strings.ToUpper(lang) != "PYTHON" {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("LANGUAGE '%s' is not supported for PACKAGES POLICY; only PYTHON is allowed.", lang), 4))
	}

	return markers
}

// ── validateAlterPackagesPolicy ──────────────────────────────────────────────

// validateAlterPackagesPolicy checks structural requirements for an
// ALTER PACKAGES POLICY statement.
func validateAlterPackagesPolicy(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// Must contain a valid action.
	validProps := []string{"ALLOWLIST", "BLOCKLIST", "ADDITIONAL_CREATION_BLOCKLIST", "COMMENT"}
	anyKnown := false
	for _, prop := range validProps {
		if hasKWPair(sig, stripped, "SET", prop) || hasKWPair(sig, stripped, "UNSET", prop) {
			anyKnown = true
			break
		}
	}
	if !anyKnown {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))
	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "PACKAGES", "POLICY")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "DROP PACKAGES POLICY requires a policy name.", 4))
	}

	return markers
}

// ── validateGrant ─────────────────────────────────────────────────────────────

// validateGrant validates a GRANT statement for structural correctness and
// privilege/object-type compatibility.
func validateGrant(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// ── GRANT ROLE / GRANT DATABASE ROLE ─────────────────────────────────────
	isGrantRole := (len(sig) >= 2 && tokUpper(sig[1], stripped) == "ROLE") ||
		(len(sig) >= 3 && tokUpper(sig[1], stripped) == "DATABASE" && tokUpper(sig[2], stripped) == "ROLE")
	if isGrantRole {
		// WITH GRANT OPTION is not valid for role grants.
		if hasKWSeq(sig, stripped, "WITH", "GRANT", "OPTION") {
			markers = append(markers, diagMarkerSpan(r,
				"WITH GRANT OPTION is not valid for GRANT ROLE statements.", 4))
		}
		// Role grants use TO USER or TO ROLE, never TO TABLE.
		if hasKWPair(sig, stripped, "TO", "TABLE") {
			markers = append(markers, diagMarkerSpan(r,
				"Unexpected syntax: Roles can be granted to other roles or users, but not directly to tables.", 4))
		}
		// Must have a grantee.
		if !hasGrantee(sig, stripped, "TO") {
			markers = append(markers, diagMarkerSpan(r,
				"GRANT ROLE requires a TO ROLE or TO USER clause.", 4))
		}
		return markers
	}

	// ── GRANT <privileges> ON <object_type> ───────────────────────────────────
	privListRaw, allFuture, objectType, ok := findOnObjectClause(sig, stripped)
	if !ok {
		return markers
	}

	// ── Grantee required ──────────────────────────────────────────────────────
	if !hasGrantee(sig, stripped, "TO") {
		markers = append(markers, diagMarkerSpan(r,
			"GRANT statement requires a grantee (TO ROLE, TO DATABASE ROLE, or TO USER).", 4))
	}

	// ── ON ALL / ON FUTURE requires IN SCHEMA or IN DATABASE ─────────────────
	if hasKWPairAny(sig, stripped, "ON", []string{"ALL", "FUTURE"}) && !hasKWPairAny(sig, stripped, "IN", []string{"SCHEMA", "DATABASE"}) {
		markers = append(markers, diagMarkerSpan(r,
			"ON ALL/FUTURE <objects> requires an IN SCHEMA or IN DATABASE qualifier.", 4))
	}

	// ── Privilege validation for known object types ───────────────────────────
	validPrivs, knownObj := grantObjectPrivileges[objectType]
	if knownObj && allFuture == "" {
		for _, priv := range splitPrivileges(privListRaw) {
			if priv == "OWNERSHIP" || priv == "ALL" || priv == "ALL PRIVILEGES" {
				continue
			}
			if !slices.Contains(validPrivs, priv) {
				msg := fmt.Sprintf("Privilege '%s' is not valid for object type %s.", priv, objectType)
				if objectType == "ROLE" && priv == "USAGE" {
					msg = "'GRANT USAGE ON ROLE' is not valid Snowflake syntax. " +
						"Use 'GRANT ROLE <name> TO ROLE/USER' to assign a role."
				}
				markers = append(markers, diagMarkerSpan(r, msg, 4))
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// ── REVOKE ROLE / REVOKE DATABASE ROLE ────────────────────────────────────
	isRevokeRole := (len(sig) >= 2 && tokUpper(sig[1], stripped) == "ROLE") ||
		(len(sig) >= 3 && tokUpper(sig[1], stripped) == "DATABASE" && tokUpper(sig[2], stripped) == "ROLE")
	if isRevokeRole {
		if !hasGrantee(sig, stripped, "FROM") {
			markers = append(markers, diagMarkerSpan(r,
				"REVOKE ROLE requires a FROM ROLE or FROM USER clause.", 4))
		}
		return markers
	}

	// ── REVOKE <privileges> ON <object_type> ──────────────────────────────────
	// Skip GRANT OPTION FOR prefix if present.
	privListRaw, allFuture, objectType, ok := findOnObjectClause(sig, stripped)
	if !ok {
		return markers
	}

	// ── ON ALL / ON FUTURE requires IN SCHEMA or IN DATABASE ─────────────────
	if hasKWPairAny(sig, stripped, "ON", []string{"ALL", "FUTURE"}) && !hasKWPairAny(sig, stripped, "IN", []string{"SCHEMA", "DATABASE"}) {
		markers = append(markers, diagMarkerSpan(r,
			"ON ALL/FUTURE <objects> requires an IN SCHEMA or IN DATABASE qualifier.", 4))
	}

	// ── CASCADE and RESTRICT are mutually exclusive ───────────────────────────
	hasCascade := hasKW(sig, stripped, "CASCADE")
	hasRestrict := hasKW(sig, stripped, "RESTRICT")
	if hasCascade && hasRestrict {
		markers = append(markers, diagMarkerSpan(r,
			"CASCADE and RESTRICT are mutually exclusive in REVOKE statement.", 4))
	}

	// ── FROM clause required ──────────────────────────────────────────────────
	if !hasGrantee(sig, stripped, "FROM") {
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
				msg := fmt.Sprintf("Privilege '%s' is not valid for object type %s.", priv, objectType)
				if objectType == "ROLE" && priv == "USAGE" {
					msg = "'REVOKE USAGE ON ROLE' is not valid Snowflake syntax. " +
						"Use 'REVOKE ROLE <name> FROM ROLE/USER' to revoke a role."
				}
				markers = append(markers, diagMarkerSpan(r, msg, 4))
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

	sig := sigToks(sqltok.Tokenize(parseText))

	// Expect: COPY INTO <target> ...
	if len(sig) < 3 || tokUpper(sig[0], parseText) != "COPY" || tokUpper(sig[1], parseText) != "INTO" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in COPY INTO statement.", 4))
		return markers
	}

	// Read target: could be ident path, @stage, or 'string literal'.
	targetIdx := 2
	if targetIdx >= len(sig) {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in COPY INTO statement.", 4))
		return markers
	}

	var target string
	var afterTargetIdx int
	if sig[targetIdx].Kind == sqltok.At {
		// Stage reference: @stage_name or @db.schema.stage/path
		afterTargetIdx = targetIdx + 1
		for afterTargetIdx < len(sig) {
			k := sig[afterTargetIdx].Kind
			if k == sqltok.Identifier || k == sqltok.QuotedIdent || k == sqltok.Dot {
				afterTargetIdx++
			} else if k == sqltok.Operator && sig[afterTargetIdx].Text(parseText) == "/" {
				// Slash in stage paths
				afterTargetIdx++
			} else {
				break
			}
		}
		target = "@"
		if afterTargetIdx > targetIdx+1 {
			target += parseText[sig[targetIdx+1].Start:sig[afterTargetIdx-1].End]
		}
	} else if sig[targetIdx].Kind == sqltok.StringLit {
		target = sig[targetIdx].Text(parseText)
		afterTargetIdx = targetIdx + 1
	} else {
		// Identifier path (table name)
		_, pathEnd := readIdentPath(sig, parseText, targetIdx)
		target = parseText[sig[targetIdx].Start:sig[pathEnd-1].End]
		afterTargetIdx = pathEnd
	}

	// Skip optional column list: (col1, col2, ...)
	if afterTargetIdx < len(sig) && sig[afterTargetIdx].Kind == sqltok.LParen {
		depth := 1
		afterTargetIdx++
		for afterTargetIdx < len(sig) {
			switch sig[afterTargetIdx].Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
				if depth == 0 {
					afterTargetIdx++
					goto doneColList
				}
			}
			afterTargetIdx++
		}
	doneColList:
	}

	// FROM clause is mandatory.
	if afterTargetIdx >= len(sig) || tokUpper(sig[afterTargetIdx], parseText) != "FROM" {
		markers = append(markers, diagMarkerSpan(r, "COPY INTO statement is missing the mandatory FROM clause.", 4))
		return markers
	}

	isUnloading := strings.HasPrefix(target, "@") || strings.HasPrefix(target, "'")

	// Skip FROM and its source to find properties.
	fromIdx := afterTargetIdx + 1
	// The FROM source can be: ident path, @stage, 'string', or (SELECT ...)
	propStartIdx := fromIdx
	if propStartIdx < len(sig) {
		if sig[propStartIdx].Kind == sqltok.LParen {
			// Subquery: skip balanced parens.
			depth := 1
			propStartIdx++
			for propStartIdx < len(sig) {
				switch sig[propStartIdx].Kind {
				case sqltok.LParen:
					depth++
				case sqltok.RParen:
					depth--
					if depth == 0 {
						propStartIdx++
						goto doneFromSource
					}
				}
				propStartIdx++
			}
		doneFromSource:
		} else {
			// Skip source tokens until we hit a keyword = pattern (property).
			for propStartIdx < len(sig) {
				if propStartIdx+1 < len(sig) && isIdent(sig[propStartIdx]) &&
					sig[propStartIdx+1].Kind == sqltok.Operator &&
					sig[propStartIdx+1].Text(parseText) == "=" {
					break
				}
				propStartIdx++
			}
		}
	}

	propSig := sig[propStartIdx:]

	if !isUnloading {
		// Loading (table target)
		hasFiles := hasKWAssign(propSig, parseText, "FILES")
		hasPattern := hasKWAssign(propSig, parseText, "PATTERN")
		if hasFiles && hasPattern {
			markers = append(markers, diagMarkerSpan(r, "FILES and PATTERN are mutually exclusive in COPY INTO statement.", 4))
		}

		// FILE_FORMAT
		ffContent := findKWAssignParenContent(propSig, parseText, "FILE_FORMAT")
		if ffContent != "" {
			ffSig := sigToks(sqltok.Tokenize(ffContent))
			hasFFName := hasKWAssign(ffSig, ffContent, "FORMAT_NAME")
			hasFFType := hasKWAssign(ffSig, ffContent, "TYPE")
			if hasFFName && hasFFType {
				markers = append(markers, diagMarkerSpan(r, "FORMAT_NAME and inline TYPE are mutually exclusive in FILE_FORMAT clause.", 4))
			}
			if hasFFType {
				typeVal, ok := findKWAssignStr(ffSig, ffContent, "TYPE")
				if !ok {
					typeVal, _ = findKWAssignIdent(ffSig, ffContent, "TYPE")
				}
				typeUpper := strings.ToUpper(strings.Trim(typeVal, "'\""))
				validTypes := map[string]bool{
					"CSV": true, "JSON": true, "AVRO": true,
					"ORC": true, "PARQUET": true, "XML": true,
				}
				if !validTypes[typeUpper] {
					markers = append(markers, diagMarkerSpan(r, "Invalid FILE_FORMAT TYPE. Must be CSV, JSON, AVRO, ORC, PARQUET, or XML.", 4))
				}
			}
		}

		// ON_ERROR
		if onErrVal, ok := findKWAssignIdent(propSig, parseText, "ON_ERROR"); ok {
			val := strings.ToUpper(onErrVal)
			if val != "CONTINUE" && val != "ABORT_STATEMENT" && val != "SKIP_FILE" &&
				!isValidSkipFileValue(val) {
				markers = append(markers, diagMarkerSpan(r, "Invalid ON_ERROR value. Must be CONTINUE, SKIP_FILE, SKIP_FILE_<n>, SKIP_FILE_<n>%, or ABORT_STATEMENT.", 4))
			}
		}

		validateBoolPropTok(propSig, parseText, "PURGE", r, &markers)
		validateBoolPropTok(propSig, parseText, "FORCE", r, &markers)
		validateBoolPropTok(propSig, parseText, "LOAD_UNCERTAIN_FILES", r, &markers)

		if matchVal, ok := findKWAssignIdent(propSig, parseText, "MATCH_BY_COLUMN_NAME"); ok {
			val := strings.ToUpper(matchVal)
			if val != "CASE_SENSITIVE" && val != "CASE_INSENSITIVE" && val != "NONE" {
				markers = append(markers, diagMarkerSpan(r, "Invalid MATCH_BY_COLUMN_NAME value. Must be CASE_SENSITIVE, CASE_INSENSITIVE, or NONE.", 4))
			}
		}
	} else {
		// Unloading (stage target)
		validateBoolPropTok(propSig, parseText, "OVERWRITE", r, &markers)
		validateBoolPropTok(propSig, parseText, "SINGLE", r, &markers)
		validateBoolPropTok(propSig, parseText, "INCLUDE_QUERY_ID", r, &markers)
		validateBoolPropTok(propSig, parseText, "DETAILED_OUTPUT", r, &markers)

		if mfsVal, ok := findKWAssignIdent(propSig, parseText, "MAX_FILE_SIZE"); ok {
			if !isPositiveIntStr(mfsVal) {
				markers = append(markers, diagMarkerSpan(r, "MAX_FILE_SIZE must be a positive integer.", 4))
			}
		} else if mfsVal, ok := findKWAssignInt(propSig, parseText, "MAX_FILE_SIZE"); ok {
			if mfsVal <= 0 {
				markers = append(markers, diagMarkerSpan(r, "MAX_FILE_SIZE must be a positive integer.", 4))
			}
		}
	}

	return markers
}

// isValidSkipFileValue checks SKIP_FILE_<n> or SKIP_FILE_<n>% format.
func isValidSkipFileValue(val string) bool {
	if !strings.HasPrefix(val, "SKIP_FILE_") {
		return false
	}
	rest := val[len("SKIP_FILE_"):]
	rest = strings.TrimSuffix(rest, "%")
	if rest == "" {
		return false
	}
	for _, b := range rest {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

// isPositiveIntStr checks if s is a string of digits representing a positive integer.
func isPositiveIntStr(s string) bool {
	if s == "" || s == "0" {
		return false
	}
	for _, b := range s {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

// findKWAssignIdent scans for KEYWORD = <identifier> and returns the identifier text.
func findKWAssignIdent(sig []sqltok.Token, sql, keyword string) (string, bool) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != keyword {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=" && isIdent(sig[i+2]) {
			return sig[i+2].Text(sql), true
		}
	}
	return "", false
}

// validateBoolPropTok checks if PROP = <value> appears in the significant
// token stream and flags when the value is not TRUE or FALSE.
func validateBoolPropTok(sig []sqltok.Token, sql, prop string, r StatementRange, markers *[]DiagMarker) {
	for i := 0; i+2 < len(sig); i++ {
		if tokUpper(sig[i], sql) != prop {
			continue
		}
		if sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(sql) == "=" {
			val := strings.ToUpper(sig[i+2].Text(sql))
			if val != "TRUE" && val != "FALSE" {
				*markers = append(*markers, diagMarkerSpan(r, fmt.Sprintf("%s must be TRUE or FALSE.", prop), 4))
			}
			return
		}
	}
}

func validateCreateIcebergTable(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	// Strip comments first to prevent comment-spoofing in property parsing
	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
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
	// Only match TRANSIENT in the preamble (between CREATE and ICEBERG).
	icebergSig := sigToks(sqltok.Tokenize(stripped))
	for i := 0; i < len(icebergSig); i++ {
		u := tokUpper(icebergSig[i], stripped)
		if u == "ICEBERG" || u == "TABLE" {
			break // past the preamble
		}
		if u == "TRANSIENT" {
			markers = append(markers, diagMarkerSpan(r, "TRANSIENT is not supported for Iceberg tables.", 4))
			break
		}
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
	sig := sigToks(sqltok.Tokenize(stripped))
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE ICEBERG TABLE"); conflict {
		markers = append(markers, marker)
	}

	// Rule: OR REPLACE is not supported for external catalogs.
	if !isSnowflakeCatalog && hasKWPair(sig, stripped, "OR", "REPLACE") {
		markers = append(markers, diagMarkerSpan(r, "OR REPLACE is not supported for Iceberg tables backed by external catalogs.", 4))
	}

	// Rule: CLUSTER BY is only for Snowflake-managed tables.
	if !isSnowflakeCatalog && hasKWPair(sig, stripped, "CLUSTER", "BY") {
		markers = append(markers, diagMarkerSpan(r, "CLUSTER BY is supported only for Snowflake-managed Iceberg tables.", 4))
	}

	// Rule: DATA_RETENTION_TIME_IN_DAYS is only for Snowflake-managed tables.
	if !isSnowflakeCatalog && hasKW(sig, stripped, "DATA_RETENTION_TIME_IN_DAYS") {
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
	sig := sigToks(sqltok.Tokenize(s))
	for i := 0; i+2 < len(sig); i++ {
		if (sig[i].Kind == sqltok.Keyword || sig[i].Kind == sqltok.Identifier) &&
			sig[i+1].Kind == sqltok.Operator && sig[i+1].Text(s) == "=" {
			key := strings.ToUpper(sig[i].Text(s))
			val := sig[i+2].Text(s)
			props[key] = val
		}
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
	hybridSig := sigToks(sqltok.Tokenize(stripped))

	if hasKWPair(hybridSig, stripped, "OR", "REPLACE") {
		markers = append(markers, diagMarkerSpan(r, "OR REPLACE is not supported for hybrid tables.", 4))
	}

	if marker, conflict := checkOrReplaceConflictTok(hybridSig, stripped, r, "CREATE HYBRID TABLE"); conflict {
		markers = append(markers, marker)
	}

	if hasKW(hybridSig, stripped, "TRANSIENT") {
		markers = append(markers, diagMarkerSpan(r, "TRANSIENT is not supported for hybrid tables.", 4))
	}

	if hasKWPair(hybridSig, stripped, "CLUSTER", "BY") {
		markers = append(markers, diagMarkerSpan(r, "CLUSTER BY is not supported on hybrid tables.", 4))
	}

	if hasKW(hybridSig, stripped, "DATA_RETENTION_TIME_IN_DAYS") {
		markers = append(markers, diagMarkerSpan(r, "DATA_RETENTION_TIME_IN_DAYS is not applicable to hybrid tables.", 4))
	}

	if hasKW(hybridSig, stripped, "CHANGE_TRACKING") {
		markers = append(markers, diagMarkerSpan(r, "CHANGE_TRACKING is not supported on hybrid tables.", 4))
	}

	if hasKWPair(hybridSig, stripped, "COPY", "GRANTS") {
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
				segClean := sqltok.StripStrings(seg)
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
					// Out of line: PRIMARY KEY (c1, c2) -- scan tokens
					segSig := sigToks(sqltok.Tokenize(segClean))
					for j := 0; j < len(segSig)-1; j++ {
						if tokUpper(segSig[j], segClean) == "PRIMARY" && tokUpper(segSig[j+1], segClean) == "KEY" {
							for k := j + 2; k < len(segSig); k++ {
								if segSig[k].Kind == sqltok.LParen {
									depth := 1
									start := k
									for l := k + 1; l < len(segSig) && depth > 0; l++ {
										if segSig[l].Kind == sqltok.LParen {
											depth++
										}
										if segSig[l].Kind == sqltok.RParen {
											depth--
										}
										if depth == 0 {
											inner := segClean[segSig[start].End:segSig[l].Start]
											for p := range strings.SplitSeq(inner, ",") {
												pkCols[normalizeIdent(p)] = true
											}
										}
									}
									break
								}
							}
							break
						}
					}
				} else if !strings.HasPrefix(content, "FOREIGN KEY") && !strings.HasPrefix(content, "UNIQUE") && !strings.HasPrefix(content, "INDEX") {
					// Column definition â extract leading identifier
					// (handles quoted identifiers with spaces like "MY COL").
					colName := extractLeadingIdent(segClean)
					if colName != "" {
						segSig := sigToks(sqltok.Tokenize(segClean))
						if hasKWPair(segSig, segClean, "PRIMARY", "KEY") {
							hasPK = true
							pkCols[colName] = true
						}
						// Strip quoted identifiers and parenthesized content
						// before checking NOT NULL / AUTOINCREMENT so that
						// column names like "AUTOINCREMENT" or expressions
						// like CHECK(id IS NOT NULL) don't cause false matches.
						bareUpSeg := strings.ToUpper(stripQuotedIdentsAndParens(segClean))
						bareSig := sigToks(sqltok.Tokenize(bareUpSeg))
						if hasKWPair(bareSig, bareUpSeg, "NOT", "NULL") || hasKW(bareSig, bareUpSeg, "NOTNULL") ||
							hasKW(bareSig, bareUpSeg, "AUTOINCREMENT") || hasKW(bareSig, bareUpSeg, "IDENTITY") {
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

// extractLeadingIdent extracts the first SQL identifier from a column
// definition segment, handling double-quoted identifiers that may contain
// spaces (e.g. "MY COL" INT NOT NULL).
func extractLeadingIdent(seg string) string {
	seg = strings.TrimSpace(seg)
	m := reIdentOrQuoted.FindString(seg)
	if m == "" {
		return ""
	}
	return normalizeIdent(m)
}

// stripQuotedIdents replaces double-quoted identifiers with a single space,
// preventing identifier names like "INDEX" from matching keyword regexes.
func stripQuotedIdents(s string) string {
	var buf strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '"' {
			buf.WriteByte(' ')
			i++
			for i < len(s) {
				if s[i] == '"' {
					if i+1 < len(s) && s[i+1] == '"' {
						i += 2
					} else {
						i++
						break
					}
				} else {
					i++
				}
			}
		} else {
			buf.WriteByte(s[i])
			i++
		}
	}
	return buf.String()
}

// stripQuotedIdentsAndParens removes double-quoted identifiers and
// parenthesized content (depth-aware) from s, replacing them with spaces.
// This prevents identifiers named "AUTOINCREMENT" or expressions like
// CHECK(id IS NOT NULL) from matching keyword regexes.
func stripQuotedIdentsAndParens(s string) string {
	var buf strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '"' {
			buf.WriteByte(' ')
			i++
			for i < len(s) {
				if s[i] == '"' {
					if i+1 < len(s) && s[i+1] == '"' {
						i += 2
					} else {
						i++
						break
					}
				} else {
					i++
				}
			}
		} else if s[i] == '(' {
			buf.WriteByte(' ')
			depth := 1
			i++
			for i < len(s) && depth > 0 {
				switch s[i] {
				case '(':
					depth++
				case ')':
					depth--
				}
				i++
			}
		} else {
			buf.WriteByte(s[i])
			i++
		}
	}
	return buf.String()
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

	// Snowflake Rule: OR REPLACE and IF NOT EXISTS are mutually exclusive.
	ffSig := sigToks(sqltok.Tokenize(stripped))
	if marker, conflict := checkOrReplaceConflictTok(ffSig, stripped, r, "CREATE FILE FORMAT"); conflict {
		markers = append(markers, marker)
	}

	if hasKW(ffSig, stripped, "TRANSIENT") {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax: TRANSIENT is not supported for FILE FORMAT objects.", 4))
	}

	if hasKW(ffSig, stripped, "TEMPORARY") || hasKW(ffSig, stripped, "TEMP") {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax: TEMPORARY is not supported for FILE FORMAT objects.", 4))
	}

	// 1. Extract all properties by scanning tokens for KEY = VALUE patterns.
	type rawProp struct {
		key string
		val string
	}
	var props []rawProp
	var rawType string

	for i := 0; i+2 < len(ffSig); i++ {
		if !isIdent(ffSig[i]) {
			continue
		}
		if ffSig[i+1].Kind != sqltok.Operator || ffSig[i+1].Text(stripped) != "=" {
			continue
		}
		key := strings.ToUpper(ffSig[i].Text(stripped))
		val := ffSig[i+2].Text(stripped)

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

	stripped := stripCommentsSQL(parseText)
	toks := sqltok.Tokenize(stripped)
	sig := sigToks(toks)

	// 1. Procedure name must be present: CALL <ident_path>
	if len(sig) < 2 || !isIdent(sig[1]) {
		markers = append(markers, diagMarkerSpan(r,
			"Missing procedure name in CALL statement.", 4))
		return markers
	}

	// 2. Argument list must be parenthesised — scan for LParen after ident path.
	hasParens := false
	for _, t := range toks {
		if t.Kind == sqltok.LParen {
			hasParens = true
			break
		}
	}
	if !hasParens {
		markers = append(markers, diagMarkerSpan(r,
			"CALL statement requires a parenthesised argument list. Use CALL proc_name() even when there are no arguments.", 4))
	}

	// 3. INTO :<variable> — the variable must be prefixed with ':' in scripting contexts.
	for i, t := range sig {
		if tokUpper(t, stripped) == "INTO" && i+1 < len(sig) {
			varTok := sig[i+1]
			varText := varTok.Text(stripped)
			if !strings.HasPrefix(varText, ":") {
				markers = append(markers, diagMarkerSpan(r, fmt.Sprintf(
					"INTO variable must be prefixed with ':' in Snowflake Scripting. Use INTO :%s instead of INTO %s.",
					varText, varText), 4))
			}
			break
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

	toks := sqltok.Tokenize(parseText)
	sig := sigToks(toks)

	// Extract the alias name: WITH <alias> AS PROCEDURE
	if len(sig) < 4 || tokUpper(sig[0], parseText) != "WITH" ||
		!isIdent(sig[1]) || tokUpper(sig[2], parseText) != "AS" ||
		tokUpper(sig[3], parseText) != "PROCEDURE" {
		return markers
	}
	alias := sig[1].Text(parseText)

	// Find the closing delimiter of the procedure body by scanning tokens
	// for the last DollarQuoted token, then looking after it.
	afterBodyStart := -1
	for i := len(toks) - 1; i >= 0; i-- {
		if toks[i].Kind == sqltok.DollarQuoted {
			afterBodyStart = toks[i].End
			break
		}
	}

	var afterBody string
	if afterBodyStart >= 0 {
		afterBody = strings.TrimSpace(parseText[afterBodyStart:])
	} else {
		// No dollar-quoted body found; look for CALL anywhere in the statement.
		afterBody = parseText
	}

	// Check if CALL follows the body.
	afterSig := sigToks(sqltok.Tokenize(afterBody))
	if len(afterSig) == 0 || tokUpper(afterSig[0], afterBody) != "CALL" {
		markers = append(markers, diagMarkerSpan(r, fmt.Sprintf(
			"WITH ... AS PROCEDURE block must end with CALL %s(...).", alias), 4))
		return markers
	}

	// Delegate structural validation of the trailing CALL to validateCall.
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. A SQL string argument is mandatory: EXECUTE IMMEDIATE <arg>
	// sig[0]=EXECUTE, sig[1]=IMMEDIATE, sig[2]=argument
	if len(sig) < 3 {
		markers = append(markers, diagMarkerSpan(r,
			"EXECUTE IMMEDIATE requires a SQL string argument (string literal, dollar-quoted string, or variable reference).", 4))
		return markers
	}

	// 2. USING clause, if present, must contain at least one bind variable.
	// Scan tokens outside strings/dollar-quoted blocks for USING keyword.
	// The USING clause requires a parenthesised list: USING (<idents>).
	toks := sqltok.Tokenize(stripped)
	for i, t := range toks {
		if t.Kind == sqltok.StringLit || t.Kind == sqltok.DollarQuoted {
			continue
		}
		if (t.Kind == sqltok.Keyword || t.Kind == sqltok.Identifier) &&
			strings.EqualFold(t.Text(stripped), "USING") {
			// USING must be followed by '(' to be a USING clause.
			// Bare "USING" without parens is not a USING clause (could be a variable name).
			hasParen := false
			for j := i + 1; j < len(toks); j++ {
				k := toks[j].Kind
				if k == sqltok.Whitespace || k == sqltok.Newline {
					continue
				}
				if k == sqltok.LParen {
					hasParen = true
				}
				break
			}
			if !hasParen {
				break
			}

			// Check that the paren list contains at least one valid identifier.
			// Bare identifiers and non-empty quoted identifiers are valid.
			// Colon-prefixed variables (:v1) and empty quoted identifiers ("") are not.
			hasValidIdent := false
			for j := i + 1; j < len(toks); j++ {
				k := toks[j].Kind
				if k == sqltok.Whitespace || k == sqltok.Newline || k == sqltok.LParen || k == sqltok.Comma {
					continue
				}
				if k == sqltok.RParen {
					break
				}
				if k == sqltok.Identifier || k == sqltok.Keyword {
					hasValidIdent = true
					break
				}
				if k == sqltok.QuotedIdent && isNonEmptyIdent(toks[j], stripped) {
					hasValidIdent = true
					break
				}
				break
			}
			if !hasValidIdent {
				markers = append(markers, diagMarkerSpan(r,
					"USING clause in EXECUTE IMMEDIATE must contain at least one bind variable.", 4))
			}
			break
		}
	}

	return markers
}

// ── validateExecuteTask ───────────────────────────────────────────────────────

// validateExecuteTask validates an EXECUTE TASK statement per the Snowflake
// docs:
//   - A task name (qualified identifier) is required; bare EXECUTE TASK is invalid.
func validateExecuteTask(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))
	// EXECUTE TASK <name> → need at least 3 significant tokens with a non-empty ident.
	if len(sig) < 3 || !isNonEmptyIdent(sig[2], stripped) {
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

	stripped := stripCommentsSQL(parseText)
	toks := sqltok.Tokenize(stripped)

	// Locate file:// and @ positions for order/presence checks.
	fileURIPos := strings.Index(strings.ToUpper(stripped), "FILE://")
	hasFileURI := fileURIPos >= 0

	atPos := -1
	for _, t := range toks {
		if t.Kind == sqltok.At {
			atPos = t.Start
			break
		}
	}

	// 1. file:// source is mandatory.
	if !hasFileURI {
		markers = append(markers, diagMarkerSpan(r,
			"PUT source path must use the file:// prefix (e.g. PUT file:///tmp/data.csv @mystage).", 4))
		return markers
	}

	// 2. @<stage> destination is mandatory.
	if atPos < 0 {
		markers = append(markers, diagMarkerSpan(r,
			"PUT requires a stage destination (e.g. @mystage or @~/path/).", 4))
		return markers
	}

	// 3. Verify positional order: PUT file://<path> @<stage>.
	if fileURIPos > atPos {
		markers = append(markers, diagMarkerSpan(r,
			"PUT source and destination are in the wrong order. Correct syntax: PUT file://<path> @<stage>.", 4))
		return markers
	}

	// 4-7. Check option values using token scan for <OPTION> = <value> patterns.
	markers = append(markers, checkOptionValue(toks, stripped, r, "PARALLEL", func(val string) string {
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 99 {
			return fmt.Sprintf("PARALLEL must be a positive integer between 1 and 99, got '%s'.", val)
		}
		return ""
	})...)
	markers = append(markers, checkOptionValue(toks, stripped, r, "SOURCE_COMPRESSION", func(val string) string {
		if !slices.Contains(validPutCompressions, strings.ToUpper(val)) {
			return fmt.Sprintf("Invalid SOURCE_COMPRESSION '%s'. Valid values: AUTO_DETECT, GZIP, BZ2, BROTLI, ZSTD, DEFLATE, RAW_DEFLATE, NONE.", val)
		}
		return ""
	})...)
	markers = append(markers, checkOptionValue(toks, stripped, r, "OVERWRITE", func(val string) string {
		if v := strings.ToUpper(val); v != "TRUE" && v != "FALSE" {
			return fmt.Sprintf("OVERWRITE must be TRUE or FALSE, got '%s'.", val)
		}
		return ""
	})...)
	markers = append(markers, checkOptionValue(toks, stripped, r, "AUTO_COMPRESS", func(val string) string {
		if v := strings.ToUpper(val); v != "TRUE" && v != "FALSE" {
			return fmt.Sprintf("AUTO_COMPRESS must be TRUE or FALSE, got '%s'.", val)
		}
		return ""
	})...)

	return markers
}

// ── validateGet ───────────────────────────────────────────────────────────────

// validateGet validates a Snowflake GET statement:
//   - @<stage> source is mandatory.
//   - file://<path> destination is mandatory.
//   - PARALLEL must be a positive integer between 1 and 99.
func validateGet(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	toks := sqltok.Tokenize(stripped)

	// Locate @ and file:// positions.
	atPos := -1
	for _, t := range toks {
		if t.Kind == sqltok.At {
			atPos = t.Start
			break
		}
	}
	fileURIPos := strings.Index(strings.ToUpper(stripped), "FILE://")

	// 1. @<stage> source is mandatory (GET @stage …).
	if atPos < 0 {
		markers = append(markers, diagMarkerSpan(r,
			"GET requires a stage source (e.g. GET @mystage file:///tmp/).", 4))
		return markers
	}

	// 2. file:// destination is mandatory.
	if fileURIPos < 0 {
		markers = append(markers, diagMarkerSpan(r,
			"GET destination path must use the file:// prefix (e.g. GET @mystage file:///tmp/).", 4))
		return markers
	}

	// 3. Verify positional order: GET @stage file://<path>.
	if fileURIPos < atPos {
		markers = append(markers, diagMarkerSpan(r,
			"GET requires a stage source (e.g. GET @mystage file:///tmp/).", 4))
		return markers
	}

	// 4. PARALLEL must be 1–99.
	markers = append(markers, checkOptionValue(toks, stripped, r, "PARALLEL", func(val string) string {
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 99 {
			return fmt.Sprintf("PARALLEL must be a positive integer between 1 and 99, got '%s'.", val)
		}
		return ""
	})...)

	return markers
}

// ── validateList ──────────────────────────────────────────────────────────────

// validateList validates a Snowflake LIST (or LS alias) statement:
//   - @<stage> argument is mandatory; bare LIST; should be flagged.
func validateList(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	toks := sqltok.Tokenize(stripCommentsSQL(parseText))
	// LIST @<stage> → look for At token after keyword.
	hasAt := false
	for _, t := range toks {
		if t.Kind == sqltok.At {
			hasAt = true
			break
		}
	}
	if !hasAt {
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

	toks := sqltok.Tokenize(stripCommentsSQL(parseText))
	// REMOVE @<stage> → look for At token after keyword.
	hasAt := false
	for _, t := range toks {
		if t.Kind == sqltok.At {
			hasAt = true
			break
		}
	}
	if !hasAt {
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

	// Strip comments so that keywords inside -- / /* */ comments don't trigger
	// false positive checks. The tokenizer naturally handles string literals —
	// keyword searches via tokUpper skip StringLit tokens.
	stripped := strings.TrimSpace(stripCommentsSQL(parseText))

	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE EVENT TABLE"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Event table name is required.
	name, nameIdx := extractNameAfterCreate(sig, stripped, []string{"TRANSIENT"}, "EVENT", "TABLE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE EVENT TABLE statement.", 4))
		return markers
	}

	// 3. TRANSIENT is not supported for event tables.
	if hasKW(sig, stripped, "TRANSIENT") {
		markers = append(markers, diagMarkerSpan(r,
			"TRANSIENT is not supported for event tables.", 4))
	}

	// 4. Column definitions are not allowed — event tables have a fixed schema.
	// Check if a LParen immediately follows the name path (not after TAG or
	// other keywords). Walk past the name path and check the next token.
	afterName := nameIdx
	for afterName < len(sig) && (isIdent(sig[afterName]) || sig[afterName].Kind == sqltok.Dot) {
		afterName++
	}
	if afterName < len(sig) && sig[afterName].Kind == sqltok.LParen {
		// Only flag if the token before LParen is NOT TAG (which takes a paren block).
		prevUpper := tokUpper(sig[afterName-1], stripped)
		if prevUpper != "TAG" {
			markers = append(markers, diagMarkerSpan(r,
				"Event tables have a fixed schema and do not support column definitions.", 4))
		}
	}

	// 5. CLUSTER BY is not supported for event tables.
	if hasKWPair(sig, stripped, "CLUSTER", "BY") {
		markers = append(markers, diagMarkerSpan(r,
			"CLUSTER BY is not supported for EVENT TABLE.", 4))
	}

	// 5. Validate property values.
	if hasKWAssign(sig, stripped, "DATA_RETENTION_TIME_IN_DAYS") {
		v, ok := findKWAssignInt(sig, stripped, "DATA_RETENTION_TIME_IN_DAYS")
		if !ok || v < 0 {
			markers = append(markers, diagMarkerSpan(r,
				"DATA_RETENTION_TIME_IN_DAYS must be a non-negative integer.", 4))
		}
	}
	if hasKWAssign(sig, stripped, "MAX_DATA_EXTENSION_TIME_IN_DAYS") {
		v, ok := findKWAssignInt(sig, stripped, "MAX_DATA_EXTENSION_TIME_IN_DAYS")
		if !ok || v < 0 {
			markers = append(markers, diagMarkerSpan(r,
				"MAX_DATA_EXTENSION_TIME_IN_DAYS must be a non-negative integer.", 4))
		}
	}
	if val, ok := findKWAssign(sig, stripped, "CHANGE_TRACKING"); ok {
		if !isBool(val) {
			markers = append(markers, diagMarkerSpan(r,
				"CHANGE_TRACKING must be TRUE or FALSE.", 4))
		}
	}

	// 6. Validate allowed properties. Use stripParenContents to avoid
	// false positives from keys inside TAG(...) or other paren blocks.
	validateProperties(stripParenContents(sqltok.StripStrings(stripped)), `DATA_RETENTION_TIME_IN_DAYS|MAX_DATA_EXTENSION_TIME_IN_DAYS|CHANGE_TRACKING|DEFAULT_DDL_COLLATION|COMMENT|TAG`, r, &markers)

	return markers
}

// ── validateCreateShare ───────────────────────────────────────────────────────

// validateCreateShare checks structural requirements for CREATE [OR REPLACE] SHARE:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Account-level object: name must not have a database or schema prefix.
//   - Only COMMENT is a valid property.
func validateCreateShare(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE SHARE"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Share name is required; also used for the account-level prefix check.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "SHARE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE SHARE statement.", 4))
		return markers
	}
	if marker, swallowed := checkNameSwallowedByIFTok(name, sig, stripped, r,
		"Unexpected syntax in CREATE SHARE statement."); swallowed {
		markers = append(markers, marker)
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, "Shares"); pfx != nil {
		markers = append(markers, *pfx)
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

	stripped := strings.TrimSpace(stripCommentsSQL(parseText))
	clean := cleanParseText(parseText)
	sig := sigToks(sqltok.Tokenize(clean))

	// 0. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, clean, r, "CREATE EXTERNAL VOLUME"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 1. Account-level: name must not have db.schema prefix.
	name, _ := extractNameAfterCreate(sig, clean, nil, "EXTERNAL", "VOLUME")
	if name != "" {
		if pfx := checkAccountLevelPrefix(name, r, "External volumes"); pfx != nil {
			markers = append(markers, *pfx)
		}
	}

	// 2. STORAGE_LOCATIONS is mandatory.
	slContent := findKWAssignParenContent(sig, clean, "STORAGE_LOCATIONS")
	if slContent == "" && !hasKWAssign(sig, clean, "STORAGE_LOCATIONS") {
		markers = append(markers, diagMarkerSpan(r,
			"STORAGE_LOCATIONS is mandatory for CREATE EXTERNAL VOLUME.", 4))
		return markers
	}

	// Extract location blocks from STORAGE_LOCATIONS outer parens.
	// Tokenize stripped (comments removed, literals intact) so the tokenizer
	// correctly handles parens inside quoted string values.
	strippedSig := sigToks(sqltok.Tokenize(stripped))
	storLocContent := findKWAssignParenContent(strippedSig, stripped, "STORAGE_LOCATIONS")
	locations := splitLocationBlocks(storLocContent)
	if len(locations) == 0 {
		markers = append(markers, diagMarkerSpan(r,
			"STORAGE_LOCATIONS must contain at least one storage location block.", 4))
		return markers
	}

	// 3–9. Per-location validation.
	for _, loc := range locations {
		locSig := sigToks(sqltok.Tokenize(loc))

		// 3. NAME is required — check NAME = '<string>'.
		if _, ok := findKWAssignStr(locSig, loc, "NAME"); !ok {
			markers = append(markers, diagMarkerSpan(r,
				"Each storage location requires a NAME attribute.", 4))
		}

		// 4. STORAGE_BASE_URL is required.
		if _, ok := findKWAssignStr(locSig, loc, "STORAGE_BASE_URL"); !ok {
			markers = append(markers, diagMarkerSpan(r,
				"Each storage location requires STORAGE_BASE_URL.", 4))
		}

		// 5. STORAGE_PROVIDER must be present and valid.
		providerVal, hasProvider := findKWAssignStr(locSig, loc, "STORAGE_PROVIDER")
		if !hasProvider {
			markers = append(markers, diagMarkerSpan(r,
				"Each storage location requires STORAGE_PROVIDER (S3, S3GOV, S3CHINA, S3COMPAT, GCS, or AZURE).", 4))
			continue
		}
		provider := strings.ToUpper(strings.Trim(providerVal, "'"))
		isS3 := provider == "S3" || provider == "S3GOV" || provider == "S3CHINA" || provider == "S3COMPAT"
		isGCS := provider == "GCS"
		isAzure := provider == "AZURE"
		if !isS3 && !isGCS && !isAzure {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Invalid STORAGE_PROVIDER '%s'. Must be S3, S3GOV, S3CHINA, S3COMPAT, GCS, or AZURE.", strings.Trim(providerVal, "'")), 4))
			continue
		}

		// 6. STORAGE_AWS_ROLE_ARN is required for S3-family.
		if isS3 && !hasKWAssign(locSig, loc, "STORAGE_AWS_ROLE_ARN") {
			markers = append(markers, diagMarkerSpan(r,
				"STORAGE_AWS_ROLE_ARN is required for S3, S3GOV, S3CHINA, and S3COMPAT storage providers.", 4))
		}

		// 7. AZURE_TENANT_ID is required for AZURE.
		if isAzure && !hasKWAssign(locSig, loc, "AZURE_TENANT_ID") {
			markers = append(markers, diagMarkerSpan(r,
				"AZURE_TENANT_ID is required for AZURE storage provider.", 4))
		}

		// 8. STORAGE_AWS_EXTERNAL_ID is only valid for S3-family.
		if !isS3 && hasKWAssign(locSig, loc, "STORAGE_AWS_EXTERNAL_ID") {
			markers = append(markers, diagMarkerSpan(r,
				"STORAGE_AWS_EXTERNAL_ID is only valid for S3, S3GOV, S3CHINA, or S3COMPAT storage providers.", 4))
		}

		// 9. ENCRYPTION handling.
		encContent := findKWAssignParenContent(locSig, loc, "ENCRYPTION")
		hasEncryption := encContent != "" || hasKWAssign(locSig, loc, "ENCRYPTION")
		if isAzure && hasEncryption {
			markers = append(markers, diagMarkerSpan(r,
				"AZURE storage locations do not support the ENCRYPTION parameter.", 4))
		} else if hasEncryption && !isAzure {
			if encContent == "" {
				markers = append(markers, diagMarkerSpan(r,
					"ENCRYPTION block must specify a TYPE key (NONE, AWS_SSE_S3, AWS_SSE_KMS, or GCS_SSE_KMS).", 4))
			} else {
				encSig := sigToks(sqltok.Tokenize(encContent))
				encType, hasType := findKWAssignStr(encSig, encContent, "TYPE")
				if !hasType {
					markers = append(markers, diagMarkerSpan(r,
						"ENCRYPTION block must specify a TYPE key (NONE, AWS_SSE_S3, AWS_SSE_KMS, or GCS_SSE_KMS).", 4))
				} else {
					et := strings.ToUpper(strings.Trim(encType, "'"))
					if !slices.Contains(extVolValidEncTypes, et) {
						markers = append(markers, diagMarkerSpan(r,
							fmt.Sprintf("Invalid ENCRYPTION TYPE '%s'. Must be NONE, AWS_SSE_S3, AWS_SSE_KMS, or GCS_SSE_KMS.", strings.Trim(encType, "'")), 4))
					} else if (et == "AWS_SSE_S3" || et == "AWS_SSE_KMS") && !isS3 {
						markers = append(markers, diagMarkerSpan(r,
							fmt.Sprintf("ENCRYPTION TYPE '%s' is only valid for S3, S3GOV, S3CHINA, or S3COMPAT storage providers.", strings.Trim(encType, "'")), 4))
					} else if et == "GCS_SSE_KMS" && !isGCS {
						markers = append(markers, diagMarkerSpan(r,
							"ENCRYPTION TYPE 'GCS_SSE_KMS' is only valid for GCS storage provider.", 4))
					}
				}
			}
		}
	}

	// 10. ALLOW_WRITES must be TRUE or FALSE if present.
	validateBoolPropTok(sig, clean, "ALLOW_WRITES", r, &markers)

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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	hasAddAccounts := hasKWPair(sig, stripped, "ADD", "ACCOUNTS")

	// RESTRICT is only valid with ADD ACCOUNTS.
	// Check if RESTRICT is the last significant token (trailing position).
	if len(sig) > 0 && tokUpper(sig[len(sig)-1], stripped) == "RESTRICT" && !hasAddAccounts {
		markers = append(markers, diagMarkerSpan(r,
			"RESTRICT is only valid with ADD ACCOUNTS in ALTER SHARE.", 4))
	}

	// ADD ACCOUNTS = requires at least one account identifier after the '='.
	if hasAddAccounts && !hasKWPairAssignIdent(sig, stripped, "ADD", "ACCOUNTS") {
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

	sig := sigToks(sqltok.Tokenize(stripCommentsSQL(parseText)))
	// USE ROLE <name> → need at least 3 significant tokens.
	if len(sig) < 3 || !isIdent(sig[2]) {
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

	sig := sigToks(sqltok.Tokenize(stripCommentsSQL(parseText)))
	// USE WAREHOUSE <name> → need at least 3 significant tokens.
	if len(sig) < 3 || !isIdent(sig[2]) {
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))
	// USE SECONDARY ROLES (ALL|NONE) → need 4 tokens with specific 4th value.
	if len(sig) < 4 {
		markers = append(markers, diagMarkerSpan(r,
			"USE SECONDARY ROLES requires ALL or NONE.", 4))
		return markers
	}
	v := tokUpper(sig[3], stripped)
	if v != "ALL" && v != "NONE" {
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
	"QUERY_TAG":                           {kind: spString},
	"TIMEZONE":                            {kind: spString},
	"TIMESTAMP_OUTPUT_FORMAT":             {kind: spString},
	"DATE_OUTPUT_FORMAT":                  {kind: spString},
	"TIME_OUTPUT_FORMAT":                  {kind: spString},
	"TIMESTAMP_INPUT_FORMAT":              {kind: spString},
	"TIMESTAMP_NTZ_OUTPUT_FORMAT":         {kind: spString},
	"TIMESTAMP_TZ_OUTPUT_FORMAT":          {kind: spString},
	"TIMESTAMP_LTZ_OUTPUT_FORMAT":         {kind: spString},
	"WEEK_START":                          {kind: spIntRange, min: 0, max: 7},
	"WEEK_OF_YEAR_POLICY":                 {kind: spIntRange, min: 0, max: 1},
	"DATE_FIRST_DAY_OF_WEEK":              {kind: spIntRange, min: 0, max: 6},
	"BINARY_OUTPUT_FORMAT":                {kind: spEnum, vals: []string{"HEX", "BASE64", "UTF8"}},
	"ROWS_PER_RESULTSET":                  {kind: spNonNeg},
	"QUOTED_IDENTIFIERS_IGNORE_CASE":      {kind: spBool},
	"AUTOCOMMIT":                          {kind: spBool},
	"TRANSACTION_DEFAULT_ISOLATION_LEVEL": {kind: spEnum, vals: []string{"READ COMMITTED"}},
	"STRICT_JSON_OUTPUT":                  {kind: spBool},
	"JSON_INDENT":                         {kind: spIntRange, min: 0, max: 16},
	"MULTI_STATEMENT_COUNT":               {kind: spNonNeg},
	"USE_CACHED_RESULT":                   {kind: spBool},
	"PYTHON_PROFILER_MODULES":             {kind: spString},
	"PYTHON_PROFILER_TARGET_STAGE":        {kind: spString},
	"SIMULATED_DATA_SHARING_CONSUMER":     {kind: spString},
	"STATEMENT_TIMEOUT_IN_SECONDS":        {kind: spNonNeg},
	"LOCK_TIMEOUT":                        {kind: spNonNeg},
	"GEOGRAPHY_OUTPUT_FORMAT":             {kind: spEnum, vals: []string{"GEOJSON", "WKT", "WKB", "EWKT", "EWKB"}},
	"GEOMETRY_OUTPUT_FORMAT":              {kind: spEnum, vals: []string{"GEOJSON", "WKT", "WKB", "EWKT", "EWKB"}},
	"CLIENT_SESSION_KEEP_ALIVE":           {kind: spBool},
	"ABORT_DETACHED_QUERY":                {kind: spBool},
	"ERROR_ON_NONDETERMINISTIC_MERGE":     {kind: spBool},
	"ERROR_ON_NONDETERMINISTIC_UPDATE":    {kind: spBool},
	"CLIENT_RESULT_CHUNK_SIZE":            {kind: spNonNeg},
	"TWO_DIGIT_CENTURY_START":             {kind: spIntRange, min: 1900, max: 2100},
	"TIMESTAMP_TYPE_MAPPING":              {kind: spEnum, vals: []string{"TIMESTAMP_NTZ", "TIMESTAMP_LTZ", "TIMESTAMP_TZ"}},
	"NETWORK_POLICY":                      {kind: spString},
	"PERIODIC_DATA_REKEYING":              {kind: spBool},
	"CLIENT_MEMORY_LIMIT":                 {kind: spNonNeg},
	"CLIENT_PREFETCH_THREADS":             {kind: spNonNeg},
}

// validateAlterSession validates ALTER SESSION SET / UNSET statements:
//   - ALTER SESSION without SET or UNSET is invalid.
//   - ALTER SESSION SET requires at least one <param> = <value> pair.
//   - ALTER SESSION UNSET requires at least one parameter name.
//   - Unknown parameter names produce a warning.
//   - Known parameter values are checked against their type constraints.
func validateAlterSession(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// sig[0]=ALTER, sig[1]=SESSION, sig[2]=SET|UNSET
	if len(sig) < 3 {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER SESSION requires SET or UNSET. Use ALTER SESSION SET <param> = <value> or ALTER SESSION UNSET <param>.", 4))
		return markers
	}

	action := tokUpper(sig[2], stripped)
	if action != "SET" && action != "UNSET" {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER SESSION requires SET or UNSET. Use ALTER SESSION SET <param> = <value> or ALTER SESSION UNSET <param>.", 4))
		return markers
	}

	// Tokens after ALTER SESSION SET/UNSET.
	rest := sig[3:]

	if action == "SET" {
		// Parse <param> = <value> pairs from the significant token stream.
		type paramPair struct {
			name  string
			value string
		}
		var pairs []paramPair
		var stray []string

		i := 0
		for i < len(rest) {
			// Skip commas between assignments.
			if rest[i].Kind == sqltok.Comma {
				i++
				continue
			}
			if !isIdent(rest[i]) {
				i++
				continue
			}
			paramName := tokUpper(rest[i], stripped)
			// Check for = after the param name.
			if i+1 < len(rest) && rest[i+1].Kind == sqltok.Operator && rest[i+1].Text(stripped) == "=" {
				if i+2 < len(rest) {
					valTok := rest[i+2]
					rawValue := valTok.Text(stripped)
					pairs = append(pairs, paramPair{name: paramName, value: rawValue})
					i += 3
				} else {
					// = without value — still count as a pair attempt.
					stray = append(stray, paramName)
					i += 2
				}
			} else {
				// Identifier without = → stray parameter.
				stray = append(stray, paramName)
				i++
			}
		}

		if len(pairs) == 0 {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER SESSION SET requires at least one parameter assignment. Use ALTER SESSION SET <param> = <value>.", 4))
			return markers
		}

		for _, s := range stray {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Parameter '%s' is missing '= <value>' assignment.", s), 4))
		}

		for _, pair := range pairs {
			spec, known := knownSessionParams[pair.name]
			if !known {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("Unknown session parameter '%s'.", pair.name), 4))
				continue
			}

			// Unquote string values.
			value := pair.value
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
						fmt.Sprintf("%s must be TRUE or FALSE.", pair.name), 4))
				}
			case spIntRange:
				n, err := strconv.Atoi(value)
				if err != nil || n < spec.min || n > spec.max {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("%s must be an integer between %d and %d.", pair.name, spec.min, spec.max), 4))
				}
			case spNonNeg:
				n, err := strconv.Atoi(value)
				if err != nil || n < 0 {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("%s must be a non-negative integer.", pair.name), 4))
				}
			case spEnum:
				upper := strings.ToUpper(value)
				if !slices.Contains(spec.vals, upper) {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("%s must be one of: %s.", pair.name, strings.Join(spec.vals, ", ")), 4))
				}
			}
		}
	}

	if action == "UNSET" {
		if len(rest) == 0 {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER SESSION UNSET requires at least one parameter name.", 4))
			return markers
		}

		// Parameter names may be comma-separated.
		// Any non-comma significant token is treated as a parameter name.
		// This means = and values in "UNSET QUERY_TAG = 'test'" are flagged.
		for _, tok := range rest {
			if tok.Kind == sqltok.Comma {
				continue
			}
			text := strings.ToUpper(tok.Text(stripped))
			if _, known := knownSessionParams[text]; !known {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("Unknown session parameter '%s'.", text), 4))
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE TAG"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Tag name is required.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "TAG")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "CREATE TAG requires a tag name.", 4))
		return markers
	}
	if marker, swallowed := checkNameSwallowedByIFTok(name, sig, stripped, r,
		"CREATE TAG requires a tag name."); swallowed {
		markers = append(markers, marker)
		return markers
	}

	// 3. ALLOWED_VALUES values must be string literals; check for duplicates.
	if hasKW(sig, stripped, "ALLOWED_VALUES") {
		// Find ALLOWED_VALUES in the full token stream of parseText and
		// extract the string literal list that follows.
		avToks := sqltok.Tokenize(parseText)
		avIdx := -1
		for i, t := range avToks {
			if (t.Kind == sqltok.Keyword || t.Kind == sqltok.Identifier) &&
				strings.EqualFold(t.Text(parseText), "ALLOWED_VALUES") {
				avIdx = i
				break
			}
		}
		if avIdx >= 0 {
			// ALLOWED_VALUES does not use "=" — flag if "=" follows immediately.
			nextSigIdx := -1
			for j := avIdx + 1; j < len(avToks); j++ {
				k := avToks[j].Kind
				if k != sqltok.Whitespace && k != sqltok.Newline {
					nextSigIdx = j
					break
				}
			}
			hasEqualsAfter := nextSigIdx >= 0 && avToks[nextSigIdx].Kind == sqltok.Operator && avToks[nextSigIdx].Text(parseText) == "="
			if hasEqualsAfter {
				markers = append(markers, diagMarkerSpan(r,
					"ALLOWED_VALUES requires a list of string literals (e.g. ALLOWED_VALUES 'v1', 'v2').", 4))
			} else {
				// Collect properly terminated string literals after ALLOWED_VALUES.
				hasValidStrings := false
				for j := avIdx + 1; j < len(avToks); j++ {
					if avToks[j].Kind == sqltok.StringLit {
						text := avToks[j].Text(parseText)
						if len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
							hasValidStrings = true
							break
						}
					}
				}
				if !hasValidStrings {
					markers = append(markers, diagMarkerSpan(r,
						"ALLOWED_VALUES requires a list of string literals (e.g. ALLOWED_VALUES 'v1', 'v2').", 4))
				} else {
					// Extract the substring from after ALLOWED_VALUES to the end of the last valid StringLit.
					listStart := avToks[avIdx].End
					listEnd := listStart
					for j := avIdx + 1; j < len(avToks); j++ {
						if avToks[j].Kind == sqltok.StringLit {
							text := avToks[j].Text(parseText)
							if len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
								listEnd = avToks[j].End
							}
						} else if avToks[j].Kind == sqltok.EOF || isIdent(avToks[j]) {
							break // stop at next keyword/identifier (a property key)
						}
					}
					markers = append(markers, checkDuplicateAllowedValues(strings.TrimSpace(parseText[listStart:listEnd]), r)...)
				}
			}
		}
	}

	// 4. Only COMMENT is a valid KEY = VALUE property for CREATE TAG.
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Tag name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "TAG")
	if name == "" {
		// When the old regex captured "IF" as the name (ALTER TAG IF EXISTS
		// with no actual name), it fell through to sub-command detection.
		// Preserve that behavior: if IF EXISTS is present but no name
		// follows, skip the name error and let the sub-command check fire.
		ifExistsNoName := len(sig) >= 4 &&
			tokUpper(sig[2], stripped) == "IF" &&
			tokUpper(sig[3], stripped) == "EXISTS" &&
			(len(sig) == 4 || !isNonEmptyIdent(sig[4], stripped))
		if !ifExistsNoName {
			markers = append(markers, diagMarkerSpan(r, "ALTER TAG requires a tag name.", 4))
			return markers
		}
	}

	// Determine the sub-command by checking token sequences.
	hasRename := hasKWPair(sig, stripped, "RENAME", "TO")
	hasAddAllowed := hasKWPair(sig, stripped, "ADD", "ALLOWED_VALUES")
	hasDropAllowed := hasKWPair(sig, stripped, "DROP", "ALLOWED_VALUES")
	hasUnsetAllowed := hasKWPair(sig, stripped, "UNSET", "ALLOWED_VALUES")
	hasSetComment := hasKWAssign(sig, stripped, "COMMENT") && hasKW(sig, stripped, "SET")
	hasUnsetComment := hasKWPair(sig, stripped, "UNSET", "COMMENT")

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
	if hasRename {
		found := false
		for i := 0; i+2 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "RENAME" && tokUpper(sig[i+1], stripped) == "TO" {
				if i+2 < len(sig) && isNonEmptyIdent(sig[i+2], stripped) {
					found = true
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER TAG RENAME TO requires a new tag name.", 4))
		}
	}

	// 3. ADD ALLOWED_VALUES requires at least one string literal value.
	if hasAddAllowed {
		found := false
		for i := 0; i+1 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "ADD" && tokUpper(sig[i+1], stripped) == "ALLOWED_VALUES" {
				// Check original parseText for string literals after the keyword position.
				after := sig[i+1].End
				rest := strings.TrimSpace(parseText[after:])
				if len(rest) > 0 && rest[0] == '\'' {
					found = true
					markers = append(markers, checkDuplicateAllowedValues(rest, r)...)
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"ADD ALLOWED_VALUES requires at least one string literal value.", 4))
		}
	}

	// 4. DROP ALLOWED_VALUES requires at least one string literal value.
	if hasDropAllowed {
		found := false
		for i := 0; i+1 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "DROP" && tokUpper(sig[i+1], stripped) == "ALLOWED_VALUES" {
				after := sig[i+1].End
				rest := strings.TrimSpace(parseText[after:])
				if len(rest) > 0 && rest[0] == '\'' {
					found = true
					markers = append(markers, checkDuplicateAllowedValues(rest, r)...)
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"DROP ALLOWED_VALUES requires at least one string literal value.", 4))
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

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Tag name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "TAG")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "DROP TAG requires a tag name.", 4))
		return markers
	}

	// 2. CASCADE / RESTRICT are not valid for DROP TAG.
	if len(sig) > 0 {
		lastKW := tokUpper(sig[len(sig)-1], stripped)
		if lastKW == "CASCADE" || lastKW == "RESTRICT" {
			markers = append(markers, diagMarkerSpan(r,
				"CASCADE / RESTRICT are not valid for DROP TAG.", 4))
		}
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

	stripped := sqltok.StripStrings(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE TASK"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Task name is required.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "TASK")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "CREATE TASK requires a task name.", 4))
		return markers
	}

	// 3. CLONE variant — CREATE TASK <name> CLONE <source> requires no AS/SCHEDULE.
	// Check for CLONE followed by an identifier (the source name).
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "CLONE" && i+1 < len(sig) && isIdent(sig[i+1]) {
			return markers
		}
	}

	// Split sig into preamble (before AS) and check body. Find the standalone
	// AS keyword that introduces the task body, skipping EXECUTE AS.
	asIdx := -1
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "AS" {
			// Skip EXECUTE AS.
			if i > 0 && tokUpper(sig[i-1], stripped) == "EXECUTE" {
				continue
			}
			asIdx = i
			break
		}
	}
	hasAS := asIdx >= 0

	// Build preamble tokens.
	var pre []sqltok.Token
	var preSrc string
	if hasAS {
		pre = sig[:asIdx]
		preSrc = stripped
	} else {
		pre = sig
		preSrc = stripped
	}

	hasAfter := hasKW(pre, preSrc, "AFTER")
	hasSchedule := hasKWAssign(pre, preSrc, "SCHEDULE")
	hasFinalize := hasKW(pre, preSrc, "FINALIZE")
	hasWhen := hasKW(pre, preSrc, "WHEN")

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
		_, hasFinalizeAssign := findKWAssign(pre, preSrc, "FINALIZE")
		if !hasFinalizeAssign {
			markers = append(markers, diagMarkerSpan(r,
				"FINALIZE requires a root task name (e.g. FINALIZE = root_task).", 4))
		}
		return markers
	}

	// 5. AFTER and SCHEDULE are mutually exclusive.
	if hasAfter && hasSchedule {
		markers = append(markers, diagMarkerSpan(r,
			"AFTER and SCHEDULE are mutually exclusive in a CREATE TASK statement. A child task (AFTER) must not also set SCHEDULE.", 4))
	}

	// 6. Bare AFTER without predecessor names.
	if hasAfter {
		afterHasName := false
		for i := 0; i < len(pre); i++ {
			if tokUpper(pre[i], preSrc) == "AFTER" && i+1 < len(pre) && isIdent(pre[i+1]) {
				afterHasName = true
				break
			}
		}
		if !afterHasName {
			markers = append(markers, diagMarkerSpan(r,
				"AFTER requires at least one predecessor task name.", 4))
		}
	}

	// 7. Root task without SCHEDULE.
	if !hasAfter && !hasSchedule {
		markers = append(markers, diagMarkerSpan(r,
			"Root task (no AFTER or FINALIZE clause) requires a SCHEDULE property.", 4))
	}

	// 8. WHEN checks.
	if hasWhen {
		// WHEN requires an expression — check that something follows WHEN.
		whenHasExpr := false
		for i := 0; i < len(pre); i++ {
			if tokUpper(pre[i], preSrc) == "WHEN" && i+1 < len(pre) {
				whenHasExpr = true
				break
			}
		}
		if !whenHasExpr {
			markers = append(markers, diagMarkerSpan(r,
				"WHEN requires a boolean expression.", 4))
		}
	}

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

	stripped := sqltok.StripStrings(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Task name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "TASK")
	if name == "" {
		// Preserve old regex behavior for ALTER TASK IF EXISTS (no name).
		ifExistsNoName := len(sig) >= 4 &&
			tokUpper(sig[2], stripped) == "IF" &&
			tokUpper(sig[3], stripped) == "EXISTS" &&
			(len(sig) == 4 || !isNonEmptyIdent(sig[4], stripped))
		if !ifExistsNoName {
			markers = append(markers, diagMarkerSpan(r, "ALTER TASK requires a task name.", 4))
			return markers
		}
	}

	// Determine the sub-command.
	// RESUME and SUSPEND must be the last significant token (no trailing content).
	lastTok := ""
	if len(sig) > 0 {
		lastTok = tokUpper(sig[len(sig)-1], stripped)
	}
	hasResume := lastTok == "RESUME"
	hasSuspend := lastTok == "SUSPEND"
	hasSet := hasKW(sig, stripped, "SET")
	hasUnset := hasKW(sig, stripped, "UNSET")
	hasRemAfter := hasKWPair(sig, stripped, "REMOVE", "AFTER")
	hasAddAfter := hasKWPair(sig, stripped, "ADD", "AFTER")
	hasModifyAS := hasKWPair(sig, stripped, "MODIFY", "AS")
	hasModifyWhen := hasKWPair(sig, stripped, "MODIFY", "WHEN")
	hasSetFinalize := hasKWAssign(sig, stripped, "FINALIZE") && hasSet
	hasUnsetFinalize := hasKWPair(sig, stripped, "UNSET", "FINALIZE")
	hasRemoveWhen := hasKWPair(sig, stripped, "REMOVE", "WHEN")
	hasSetTag := hasKWPair(sig, stripped, "SET", "TAG")
	hasUnsetTag := hasKWPair(sig, stripped, "UNSET", "TAG")

	anyKnown := hasResume || hasSuspend || hasSet || hasUnset ||
		hasRemAfter || hasAddAfter || hasModifyAS || hasModifyWhen ||
		hasSetFinalize || hasUnsetFinalize || hasRemoveWhen || hasSetTag || hasUnsetTag

	if !anyKnown {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER TASK sub-command. Expected RESUME, SUSPEND, SET, UNSET, ADD AFTER, REMOVE AFTER, MODIFY AS, MODIFY WHEN, REMOVE WHEN, SET FINALIZE, UNSET FINALIZE, SET TAG, or UNSET TAG.", 4))
		return markers
	}

	// 2. ADD AFTER requires at least one predecessor name.
	if hasAddAfter {
		found := false
		for i := 0; i+2 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "ADD" && tokUpper(sig[i+1], stripped) == "AFTER" {
				if i+2 < len(sig) && isIdent(sig[i+2]) {
					found = true
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"ADD AFTER requires at least one predecessor task name.", 4))
		}
	}

	// 3. REMOVE AFTER requires at least one predecessor name.
	if hasRemAfter {
		found := false
		for i := 0; i+2 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "REMOVE" && tokUpper(sig[i+1], stripped) == "AFTER" {
				if i+2 < len(sig) && isIdent(sig[i+2]) {
					found = true
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"REMOVE AFTER requires at least one predecessor task name.", 4))
		}
	}

	// 4. MODIFY AS requires a SQL body.
	if hasModifyAS {
		found := false
		for i := 0; i+2 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "MODIFY" && tokUpper(sig[i+1], stripped) == "AS" {
				if i+2 < len(sig) {
					found = true
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"MODIFY AS requires a SQL statement.", 4))
		}
	}

	// 5. MODIFY WHEN requires a boolean expression.
	if hasModifyWhen {
		found := false
		for i := 0; i+2 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "MODIFY" && tokUpper(sig[i+1], stripped) == "WHEN" {
				if i+2 < len(sig) {
					found = true
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"MODIFY WHEN requires a boolean expression.", 4))
		}
	}

	// 6. SET FINALIZE requires a root task name.
	if hasSetFinalize {
		_, hasFinalizeIdent := findKWAssign(sig, stripped, "FINALIZE")
		if !hasFinalizeIdent {
			markers = append(markers, diagMarkerSpan(r,
				"SET FINALIZE requires a root task name (e.g. SET FINALIZE = root_task).", 4))
		}
	}

	// No property validation for SET/UNSET — tasks accept arbitrary session
	// parameters (TIMEZONE, QUERY_TAG, STATEMENT_TIMEOUT_IN_SECONDS, etc.)
	// which would produce false-positive warnings. Sub-commands like SET TAG,
	// SET CONTACT, SET EXECUTE AS, UNSET FINALIZE, and UNSET DCM PROJECT are
	// all recognized by the anyKnown check via hasSet/hasUnset.

	return markers
}

// ── validateDropTask ─────────────────────────────────────────────────────────

// validateDropTask checks structural requirements for DROP TASK statements:
//   - Task name is mandatory.
func validateDropTask(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))
	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "TASK")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "DROP TASK requires a task name.", 4))
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
// valid string literal.  Embedded doubled single-quotes are handled.
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
	sig := sigToks(sqltok.Tokenize(stripped))

	// Valid forms (token sequences):
	//   BEGIN
	//   BEGIN WORK
	//   BEGIN TRANSACTION
	//   BEGIN [WORK|TRANSACTION] NAME <ident>
	i := 0
	if i >= len(sig) || tokUpper(sig[i], stripped) != "BEGIN" {
		return markers
	}
	i++
	if i < len(sig) {
		u := tokUpper(sig[i], stripped)
		if u == "WORK" || u == "TRANSACTION" {
			i++
		}
	}
	if i < len(sig) && tokUpper(sig[i], stripped) == "NAME" {
		i++
		if i >= len(sig) || !isIdent(sig[i]) {
			markers = append(markers, diagMarkerSpan(r,
				"BEGIN NAME requires a transaction name. Use BEGIN NAME <name>.", 4))
			return markers
		}
		return markers // valid: BEGIN [WORK|TRANSACTION] NAME <ident>
	}
	if i < len(sig) {
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
	sig := sigToks(sqltok.Tokenize(stripped))

	// Valid forms: COMMIT, COMMIT WORK
	n := len(sig)
	if n == 1 {
		return markers // bare COMMIT
	}
	if n == 2 && tokUpper(sig[1], stripped) == "WORK" {
		return markers // COMMIT WORK
	}
	markers = append(markers, diagMarkerSpan(r,
		"Unexpected token after COMMIT. Valid forms: COMMIT, COMMIT WORK.", 4))
	return markers
}

// ── validateRollback ──────────────────────────────────────────────────────────

// validateRollbackStripped validates a ROLLBACK statement from already-stripped text
// (comments removed, trimmed). This avoids redundant stripCommentsSQL calls when
// the caller has already stripped the text for block-level tracking.
func validateRollbackStripped(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker
	sig := sigToks(sqltok.Tokenize(stripped))

	// Valid forms: ROLLBACK, ROLLBACK WORK, ROLLBACK [WORK] TO SAVEPOINT <name>
	i := 1 // skip ROLLBACK
	if i < len(sig) && tokUpper(sig[i], stripped) == "WORK" {
		i++
	}
	if i == len(sig) {
		return markers // bare ROLLBACK or ROLLBACK WORK
	}
	// Expect TO SAVEPOINT <name>
	if tokUpper(sig[i], stripped) == "TO" {
		i++
		if i < len(sig) && tokUpper(sig[i], stripped) == "SAVEPOINT" {
			i++
			if i < len(sig) && isIdent(sig[i]) {
				return markers // valid: ROLLBACK [WORK] TO SAVEPOINT <name>
			}
			markers = append(markers, diagMarkerSpan(r,
				"ROLLBACK TO SAVEPOINT requires a savepoint name. Use ROLLBACK TO SAVEPOINT <name>.", 4))
			return markers
		}
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
	sig := sigToks(sqltok.Tokenize(stripped))

	// SAVEPOINT <name> — name is mandatory (at least 2 sig tokens)
	if len(sig) < 2 || !isIdent(sig[1]) {
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
	sig := sigToks(sqltok.Tokenize(stripped))

	// RELEASE SAVEPOINT <name> — name is mandatory (at least 3 sig tokens)
	if len(sig) < 3 || !isIdent(sig[2]) {
		markers = append(markers, diagMarkerSpan(r,
			"RELEASE SAVEPOINT requires a savepoint name. Use RELEASE SAVEPOINT <name>.", 4))
	}
	return markers
}

// ── validateTimeTravelClauses ─────────────────────────────────────────────────

// validateTimeTravelClauses scans the statement for AT(...) / BEFORE(...)
// Time Travel clauses and validates their structural correctness:
//   - The clause must use parentheses (bare AT TIMESTAMP ... is invalid).
//   - Exactly one keyword argument: TIMESTAMP =>, OFFSET =>, STATEMENT =>,
//     or STREAM => (AT only).
//   - Multiple keyword arguments are invalid.
//   - STREAM => is not valid inside BEFORE.
//   - The => operator is required.
func validateTimeTravelClauses(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	sig := sigToks(sqltok.Tokenize(stripped))

	// Check for bare AT/BEFORE without parentheses after a table reference.
	// Pattern: FROM/JOIN <ident> AT/BEFORE TIMESTAMP/OFFSET/STATEMENT/STREAM
	ttKWs := []string{"TIMESTAMP", "OFFSET", "STATEMENT", "STREAM"}
	for i := 0; i+3 < len(sig); i++ {
		u := tokUpper(sig[i], stripped)
		if u != "FROM" && u != "JOIN" {
			continue
		}
		// Skip ident path after FROM/JOIN
		j := i + 1
		if !isIdent(sig[j]) {
			continue
		}
		_, j = readIdentPath(sig, stripped, j)
		if j >= len(sig) {
			continue
		}
		ab := tokUpper(sig[j], stripped)
		if ab != "AT" && ab != "BEFORE" {
			continue
		}
		// Check if next token is a time travel keyword (not LParen).
		if j+1 < len(sig) && sig[j+1].Kind != sqltok.LParen {
			nextU := tokUpper(sig[j+1], stripped)
			for _, kw := range ttKWs {
				if nextU == kw {
					markers = append(markers, diagMarkerSpan(r,
						"Time Travel clause requires parentheses. Use AT (TIMESTAMP => ...) or BEFORE (STATEMENT => ...).", 4))
					break
				}
			}
		}
	}

	// Find each AT(...) / BEFORE(...) occurrence and validate contents.
	for i := 0; i+1 < len(sig); i++ {
		keyword := tokUpper(sig[i], stripped)
		if keyword != "AT" && keyword != "BEFORE" {
			continue
		}
		if sig[i+1].Kind != sqltok.LParen {
			continue
		}

		// Extract tokens inside the parentheses.
		depth := 0
		innerStart := i + 2
		innerEnd := innerStart
		for j := i + 1; j < len(sig); j++ {
			switch sig[j].Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
				if depth == 0 {
					innerEnd = j
					goto foundClose
				}
			}
		}
		continue // Unbalanced — the syntax checker will flag it.
	foundClose:

		innerSig := sig[innerStart:innerEnd]

		// Count how many valid keyword arguments appear (KW =>).
		var args []string
		for k := 0; k+1 < len(innerSig); k++ {
			u := tokUpper(innerSig[k], stripped)
			if (u == "TIMESTAMP" || u == "OFFSET" || u == "STATEMENT" || u == "STREAM") &&
				innerSig[k+1].Kind == sqltok.Operator && innerSig[k+1].Text(stripped) == "=>" {
				args = append(args, u)
			}
		}

		streamExpected := ""
		streamPlain := ""
		if keyword == "AT" {
			streamExpected = ", STREAM =>"
			streamPlain = ", STREAM"
		}

		if len(args) == 0 {
			// Check if the user wrote a keyword without =>
			bareKW := ""
			for k := 0; k < len(innerSig); k++ {
				u := tokUpper(innerSig[k], stripped)
				if u == "TIMESTAMP" || u == "OFFSET" || u == "STATEMENT" || u == "STREAM" {
					bareKW = u
					break
				}
			}
			if bareKW != "" {
				markers = append(markers, diagMarkerSpan(r,
					"Missing '=>' operator in "+keyword+" clause. Use "+bareKW+" => <value>.", 4))
			} else {
				markers = append(markers, diagMarkerSpan(r,
					"Invalid "+keyword+" clause. Expected one of: TIMESTAMP =>, OFFSET =>, STATEMENT =>"+streamExpected+".", 4))
			}
			continue
		}

		if len(args) > 1 {
			markers = append(markers, diagMarkerSpan(r,
				"Multiple keyword arguments in "+keyword+" clause. Only one of TIMESTAMP, OFFSET, STATEMENT"+streamPlain+" is allowed.", 4))
			continue
		}

		// Exactly one argument — validate STREAM restriction.
		if args[0] == "STREAM" && keyword == "BEFORE" {
			markers = append(markers, diagMarkerSpan(r,
				"STREAM => is not valid in a BEFORE clause. STREAM is only supported with AT.", 4))
		}
	}

	return markers
}

// ── validateCreateReplicationGroup ────────────────────────────────────────────

// validateCreateReplicationGroup checks structural requirements for
// CREATE [OR REPLACE] REPLICATION GROUP:
//   - Group name is required and must not have a db.schema prefix (account-level).
//   - OBJECT_TYPES is mandatory.
//   - ALLOWED_ACCOUNTS is mandatory.
//   - If OBJECT_TYPES includes DATABASES, ALLOWED_DATABASES is required.
//   - If OBJECT_TYPES includes INTEGRATIONS, ALLOWED_INTEGRATION_TYPES is required.
func validateCreateReplicationGroup(parseText string, r StatementRange) []DiagMarker {
	return validateCreateReplOrFailoverGroup(parseText, r, "REPLICATION")
}

// ── validateCreateFailoverGroup ──────────────────────────────────────────────

// validateCreateFailoverGroup checks structural requirements for
// CREATE [OR REPLACE] FAILOVER GROUP. Same rules as REPLICATION GROUP.
func validateCreateFailoverGroup(parseText string, r StatementRange) []DiagMarker {
	return validateCreateReplOrFailoverGroup(parseText, r, "FAILOVER")
}

// validateCreateReplOrFailoverGroup is the shared implementation for both
// CREATE REPLICATION GROUP and CREATE FAILOVER GROUP validation.
func validateCreateReplOrFailoverGroup(parseText string, r StatementRange, groupType string) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Group name is required and must be account-level (no dot prefix).
	name, nameIdx := extractNameAfterKeywords(sig, stripped, "CREATE", groupType, "GROUP")
	// Also try CREATE OR REPLACE <groupType> GROUP
	if name == "" {
		name, nameIdx = extractNameAfterCreate(sig, stripped, nil, groupType, "GROUP")
	}
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("CREATE %s GROUP requires a group name.", groupType), 4))
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, groupType+" groups"); pfx != nil {
		markers = append(markers, *pfx)
	}

	// Use tokens after the name for clause detection.
	_, afterIdx := readIdentPath(sig, stripped, nameIdx)
	afterName := sig[afterIdx:]

	// 2. OBJECT_TYPES is mandatory.
	if !hasKWAssign(afterName, stripped, "OBJECT_TYPES") {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("Missing mandatory OBJECT_TYPES in CREATE %s GROUP.", groupType), 4))
		return markers
	}

	// 3. ALLOWED_ACCOUNTS (or ALLOWED_FAILOVER_ACCOUNTS for failover) is mandatory.
	if groupType == "FAILOVER" {
		if !hasKWAssign(afterName, stripped, "ALLOWED_ACCOUNTS") && !hasKWAssign(afterName, stripped, "ALLOWED_FAILOVER_ACCOUNTS") {
			markers = append(markers, diagMarkerSpan(r,
				"Missing mandatory ALLOWED_ACCOUNTS or ALLOWED_FAILOVER_ACCOUNTS in CREATE FAILOVER GROUP.", 4))
		}
	} else {
		if !hasKWAssign(afterName, stripped, "ALLOWED_ACCOUNTS") {
			markers = append(markers, diagMarkerSpan(r,
				"Missing mandatory ALLOWED_ACCOUNTS in CREATE REPLICATION GROUP.", 4))
		}
	}

	// 4–5. Extract the OBJECT_TYPES value portion and check for DATABASES / INTEGRATIONS.
	otValue := extractObjectTypesValue(afterName, stripped)
	if strings.Contains(otValue, "DATABASES") && !hasKWAssign(afterName, stripped, "ALLOWED_DATABASES") {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("OBJECT_TYPES includes DATABASES but ALLOWED_DATABASES is missing in CREATE %s GROUP.", groupType), 4))
	}
	if strings.Contains(otValue, "INTEGRATIONS") && !hasKWAssign(afterName, stripped, "ALLOWED_INTEGRATION_TYPES") {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("OBJECT_TYPES includes INTEGRATIONS but ALLOWED_INTEGRATION_TYPES is missing in CREATE %s GROUP.", groupType), 4))
	}

	return markers
}

// ── validateAlterReplicationOrFailoverGroup ───────────────────────────────────

// validateAlterReplicationOrFailoverGroup checks structural requirements for
// ALTER REPLICATION GROUP and ALTER FAILOVER GROUP:
//   - Group name is required.
//   - Must contain a valid action (ADD, REMOVE, MOVE DATABASES, SET, RENAME TO,
//     or for FAILOVER: PRIMARY, REFRESH, SUSPEND, RESUME).
func validateAlterReplicationOrFailoverGroup(parseText string, r StatementRange, groupType string) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Group name is required and must be account-level (no dot prefix).
	name, nameIdx := extractNameAfterKeywords(sig, stripped, "ALTER", groupType, "GROUP")
	if name == "" {
		// Handle IF EXISTS without a name.
		ifExistsNoName := len(sig) >= 5 &&
			tokUpper(sig[3], stripped) == "IF" &&
			tokUpper(sig[4], stripped) == "EXISTS" &&
			(len(sig) == 5 || !isNonEmptyIdent(sig[5], stripped))
		if !ifExistsNoName {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("ALTER %s GROUP requires a group name.", groupType), 4))
			return markers
		}
	}
	if name != "" {
		if pfx := checkAccountLevelPrefix(name, r, groupType+" groups"); pfx != nil {
			markers = append(markers, *pfx)
		}
	}

	// 2. Must contain a valid action after the group name.
	var afterName []sqltok.Token
	if name != "" {
		_, pathEnd := readIdentPath(sig, stripped, nameIdx)
		if pathEnd < len(sig) {
			afterName = sig[pathEnd:]
		}
	} else {
		// No name (IF EXISTS case) — start after IF EXISTS.
		startIdx := 5
		if startIdx < len(sig) {
			afterName = sig[startIdx:]
		}
	}

	hasAdd := hasKW(afterName, stripped, "ADD")
	hasRemove := hasKW(afterName, stripped, "REMOVE")
	hasMoveDatabases := hasKWPair(afterName, stripped, "MOVE", "DATABASES")
	hasSet := false
	for i := 0; i+1 < len(afterName); i++ {
		if tokUpper(afterName[i], stripped) == "SET" {
			next := tokUpper(afterName[i+1], stripped)
			if next == "REPLICATION_SCHEDULE" || next == "OBJECT_TYPES" {
				hasSet = true
				break
			}
		}
	}
	hasRename := hasKWPair(afterName, stripped, "RENAME", "TO")

	hasAction := hasAdd || hasRemove || hasMoveDatabases || hasSet || hasRename

	if groupType == "FAILOVER" {
		hasAction = hasAction ||
			hasKW(afterName, stripped, "PRIMARY") ||
			hasKW(afterName, stripped, "REFRESH") ||
			hasKW(afterName, stripped, "SUSPEND") ||
			hasKW(afterName, stripped, "RESUME")
	}

	if !hasAction {
		if groupType == "FAILOVER" {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER FAILOVER GROUP requires an action: ADD, REMOVE, MOVE DATABASES, SET, RENAME TO, PRIMARY, REFRESH, SUSPEND, or RESUME.", 4))
		} else {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER REPLICATION GROUP requires an action: ADD, REMOVE, MOVE DATABASES, SET, or RENAME TO.", 4))
		}
		return markers
	}

	// 3. MOVE DATABASES requires TO REPLICATION GROUP <name>.
	if hasMoveDatabases && !hasKWSeqFollowedByIdent(afterName, stripped, "TO", "REPLICATION", "GROUP") {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("MOVE DATABASES in ALTER %s GROUP requires TO REPLICATION GROUP <name>.", groupType), 4))
	}

	return markers
}

// ── validateDropReplicationOrFailoverGroup ────────────────────────────────────

// validateDropReplicationOrFailoverGroup checks structural requirements for
// DROP REPLICATION GROUP and DROP FAILOVER GROUP:
//   - Group name is required.
func validateDropReplicationOrFailoverGroup(parseText string, r StatementRange, groupType string) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// DROP (REPLICATION|FAILOVER) GROUP [IF EXISTS] <name>
	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", groupType, "GROUP")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("DROP %s GROUP requires a group name.", groupType), 4))
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, groupType+" groups"); pfx != nil {
		markers = append(markers, *pfx)
	}

	return markers
}

// ── validateCreateComputePool ─────────────────────────────────────────────────

// validateCreateComputePool checks structural requirements for
// CREATE [OR REPLACE] COMPUTE POOL [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Compute pools are account-level objects: name must not have a db.schema prefix.
//   - MIN_NODES, MAX_NODES, and INSTANCE_FAMILY are mandatory.
//   - MIN_NODES must be >= 1.
//   - MAX_NODES must be >= MIN_NODES.
//   - INSTANCE_FAMILY must be one of the valid Snowpark Container Services SKUs.
//   - AUTO_RESUME and INITIALLY_SUSPENDED must be TRUE or FALSE.
//   - AUTO_SUSPEND_SECS must be a non-negative integer.
//   - Only known properties are accepted.
func validateCreateComputePool(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE COMPUTE POOL"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Pool name is required; also used for account-level prefix check.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "COMPUTE", "POOL")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE COMPUTE POOL statement.", 4))
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, "Compute pools"); pfx != nil {
		markers = append(markers, *pfx)
	}

	// 3. Mandatory properties: MIN_NODES, MAX_NODES, INSTANCE_FAMILY.
	minNodesVal, hasMinNodesProp := findKWAssignInt(sig, stripped, "MIN_NODES")
	maxNodesVal, hasMaxNodesProp := findKWAssignInt(sig, stripped, "MAX_NODES")
	instanceFamilyRaw, hasInstanceFamily := findKWAssign(sig, stripped, "INSTANCE_FAMILY")

	if !hasMinNodesProp {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory property MIN_NODES in CREATE COMPUTE POOL statement.", 4))
	}
	if !hasMaxNodesProp {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory property MAX_NODES in CREATE COMPUTE POOL statement.", 4))
	}
	if !hasInstanceFamily {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory property INSTANCE_FAMILY in CREATE COMPUTE POOL statement.", 4))
	}

	// 4. Validate MIN_NODES value (>= 1).
	if hasMinNodesProp && minNodesVal < 1 {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("MIN_NODES value %d is below the minimum (1).", minNodesVal), 4))
	}

	// 5. Validate MAX_NODES value (>= 1, >= MIN_NODES).
	if hasMaxNodesProp {
		if maxNodesVal < 1 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MAX_NODES value %d is below the minimum (1).", maxNodesVal), 4))
		}
		if hasMinNodesProp && maxNodesVal < minNodesVal {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MAX_NODES (%d) must be >= MIN_NODES (%d).", maxNodesVal, minNodesVal), 4))
		}
	}

	// 6. Validate INSTANCE_FAMILY against known SKUs.
	validFamilies := []string{
		"CPU_X64_XS", "CPU_X64_S", "CPU_X64_M", "CPU_X64_L", "CPU_X64_XL",
		"HIGHMEM_X64_S", "HIGHMEM_X64_M", "HIGHMEM_X64_L", "HIGHMEM_X64_SL",
		"GPU_NV_S", "GPU_NV_M", "GPU_NV_L", "GPU_NV_XL", "GPU_NV_4XL",
	}
	if hasInstanceFamily {
		family := strings.ToUpper(instanceFamilyRaw)
		if !slices.Contains(validFamilies, family) {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Invalid INSTANCE_FAMILY '%s'. Valid values: %s.",
					instanceFamilyRaw, strings.Join(validFamilies, ", ")), 4))
		}
	}

	// 7. Validate AUTO_SUSPEND_SECS (non-negative integer).
	if susVal, ok := findKWAssignInt(sig, stripped, "AUTO_SUSPEND_SECS"); ok {
		if susVal < 0 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("AUTO_SUSPEND_SECS value %d must be a non-negative integer.", susVal), 4))
		}
	}

	// 8. Validate boolean properties.
	validateBoolPropTok(sig, stripped, "AUTO_RESUME", r, &markers)
	validateBoolPropTok(sig, stripped, "INITIALLY_SUSPENDED", r, &markers)

	// 9. Only known properties are accepted.
	noComments := strings.TrimSpace(stripCommentsSQL(parseText))
	validateProperties(noComments, `MIN_NODES|MAX_NODES|INSTANCE_FAMILY|AUTO_RESUME|AUTO_SUSPEND_SECS|COMMENT|INITIALLY_SUSPENDED`, r, &markers)

	return markers
}

// ── validateCreateDatashare ───────────────────────────────────────────────────

// validateCreateDatashare checks structural requirements for
// CREATE [OR REPLACE] DATASHARE [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Datashares are account-level objects: name must not have a db.schema prefix.
//   - SHARE_RESTRICTIONS must be TRUE or FALSE if present.
//   - Only COMMENT and SHARE_RESTRICTIONS are valid properties.
func validateCreateDatashare(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE DATASHARE"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Datashare name is required; also used for the account-level prefix check.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "DATASHARE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE DATASHARE statement.", 4))
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, "Datashares"); pfx != nil {
		markers = append(markers, *pfx)
	}

	// 3. Only COMMENT and SHARE_RESTRICTIONS are valid properties.
	noComments := strings.TrimSpace(stripCommentsSQL(parseText))
	validateProperties(noComments, `COMMENT|SHARE_RESTRICTIONS`, r, &markers)

	// 4. SHARE_RESTRICTIONS must be TRUE or FALSE.
	validateBoolPropTok(sig, stripped, "SHARE_RESTRICTIONS", r, &markers)

	return markers
}

// ── validateAlterDatashare ────────────────────────────────────────────────────

// validateAlterDatashare checks structural requirements for ALTER DATASHARE statements:
//   - A datashare name is required.
//   - ADD ACCOUNTS = requires at least one account identifier.
//   - REMOVE ACCOUNTS = requires at least one account identifier.
//   - ADD DATABASES requires at least one database identifier.
//   - REMOVE DATABASES requires at least one database identifier.
//   - SHARE_RESTRICTIONS must be TRUE or FALSE if present (valid with ADD ACCOUNTS).
//   - Unknown sub-commands warn.
func validateAlterDatashare(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Datashare name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "DATASHARE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER DATASHARE requires a datashare name.", 4))
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, "Datashares"); pfx != nil {
		markers = append(markers, *pfx)
	}

	// 2. If none of the known actions are present, warn about unknown sub-command.
	hasAddAccounts := hasKWPair(sig, stripped, "ADD", "ACCOUNTS")
	hasRemoveAccounts := hasKWPair(sig, stripped, "REMOVE", "ACCOUNTS")
	hasAddDatabases := hasKWPair(sig, stripped, "ADD", "DATABASES")
	hasRemoveDatabases := hasKWPair(sig, stripped, "REMOVE", "DATABASES")
	hasSetComment := hasKWPair(sig, stripped, "SET", "COMMENT")
	hasUnsetComment := hasKWPair(sig, stripped, "UNSET", "COMMENT")

	anyKnown := hasAddAccounts || hasRemoveAccounts || hasAddDatabases || hasRemoveDatabases || hasSetComment || hasUnsetComment
	if !anyKnown {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER DATASHARE sub-command. Expected ADD ACCOUNTS, REMOVE ACCOUNTS, ADD DATABASES, REMOVE DATABASES, SET COMMENT, or UNSET COMMENT.", 4))
		return markers
	}

	// 3. ADD ACCOUNTS = requires at least one account.
	// Check: ADD ACCOUNTS followed by = then at least one ident.
	if hasAddAccounts && !hasKWPairAssignIdent(sig, stripped, "ADD", "ACCOUNTS") {
		markers = append(markers, diagMarkerSpan(r,
			"ADD ACCOUNTS requires at least one account identifier.", 4))
	}

	// 4. REMOVE ACCOUNTS = requires at least one account.
	if hasRemoveAccounts && !hasKWPairAssignIdent(sig, stripped, "REMOVE", "ACCOUNTS") {
		markers = append(markers, diagMarkerSpan(r,
			"REMOVE ACCOUNTS requires at least one account identifier.", 4))
	}

	// 5. ADD DATABASES requires at least one database.
	if hasAddDatabases && !hasKWPairFollowedByIdent(sig, stripped, "ADD", "DATABASES") {
		markers = append(markers, diagMarkerSpan(r,
			"ADD DATABASES requires at least one database identifier.", 4))
	}

	// 6. REMOVE DATABASES requires at least one database.
	if hasRemoveDatabases && !hasKWPairFollowedByIdent(sig, stripped, "REMOVE", "DATABASES") {
		markers = append(markers, diagMarkerSpan(r,
			"REMOVE DATABASES requires at least one database identifier.", 4))
	}

	// 7. SHARE_RESTRICTIONS validation: always check the boolean value, and
	// warn if it appears without ADD ACCOUNTS (the only valid context).
	hasShareRestrictions := hasKW(sig, stripped, "SHARE_RESTRICTIONS")
	if hasShareRestrictions {
		validateBoolPropTok(sig, stripped, "SHARE_RESTRICTIONS", r, &markers)
		if !hasAddAccounts {
			markers = append(markers, diagMarkerSpan(r,
				"SHARE_RESTRICTIONS is only valid with ADD ACCOUNTS in ALTER DATASHARE.", 4))
		}
	}

	return markers
}

// ── validateDropDatashare ─────────────────────────────────────────────────────

// validateDropDatashare checks structural requirements for DROP DATASHARE:
//   - Datashare name is required.
//   - Datashares are account-level objects: name must not have a db.schema prefix.
func validateDropDatashare(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "DATASHARE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DROP DATASHARE requires a datashare name.", 4))
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, "Datashares"); pfx != nil {
		markers = append(markers, *pfx)
	}

	return markers
}

// ── validateCreateService ─────────────────────────────────────────────────────

// validateCreateService checks structural requirements for
// CREATE [OR REPLACE] SERVICE [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - IN COMPUTE POOL <name> is mandatory.
//   - Exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE is required.
//   - MIN_INSTANCES must be a non-negative integer if present.
//   - MAX_INSTANCES must be >= MIN_INSTANCES if both are present.
//   - AUTO_RESUME must be TRUE or FALSE if present.
//   - Only known properties are accepted.
func validateCreateService(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	// Strip dollar-quoted bodies so inline YAML does not cause false-positive
	// property warnings (e.g. keys inside $$spec$$ matching reProp).
	noDollar := stripDollarQuoted(parseText)
	noLiterals := sqltok.StripStrings(noDollar)
	clean := strings.TrimSpace(stripCommentsSQL(noLiterals))

	sig := sigToks(sqltok.Tokenize(clean))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, clean, r, "CREATE SERVICE"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Service name is required.
	name, _ := extractNameAfterCreate(sig, clean, nil, "SERVICE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in CREATE SERVICE statement.", 4))
		return markers
	}

	// 3. IN COMPUTE POOL is mandatory.
	if !hasKWSeq(sig, clean, "IN", "COMPUTE", "POOL") {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory IN COMPUTE POOL clause in CREATE SERVICE statement.", 4))
	}

	// 4. Exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE is required.
	hasSpecFile := hasFromSpecKW(sig, clean, []string{"SPECIFICATION_FILE", "SPECIFICATION_TEMPLATE_FILE"})
	hasSpec := hasFromSpecKW(sig, clean, []string{"SPECIFICATION", "SPECIFICATION_TEMPLATE"})
	if hasSpec && hasSpecFile {
		markers = append(markers, diagMarkerSpan(r,
			"CREATE SERVICE requires exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE, not both.", 4))
	} else if !hasSpec && !hasSpecFile {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory FROM SPECIFICATION or FROM SPECIFICATION_FILE clause in CREATE SERVICE statement.", 4))
	}

	// 5. Validate MIN_INSTANCES value (non-negative).
	var minInstances int
	hasMinInstances := false
	if v, ok := findKWAssignInt(sig, clean, "MIN_INSTANCES"); ok {
		minInstances = v
		hasMinInstances = true
		if v < 0 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MIN_INSTANCES value %d must be a non-negative integer.", v), 4))
		}
	}

	// 6. Validate MAX_INSTANCES value (non-negative, >= MIN_INSTANCES).
	if v, ok := findKWAssignInt(sig, clean, "MAX_INSTANCES"); ok {
		if v < 0 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MAX_INSTANCES value %d must be a non-negative integer.", v), 4))
		}
		if hasMinInstances && v < minInstances {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MAX_INSTANCES (%d) must be >= MIN_INSTANCES (%d).", v, minInstances), 4))
		}
	}

	// 7. Validate boolean properties.
	validateBoolPropTok(sig, clean, "AUTO_RESUME", r, &markers)

	// 8. Only known properties are accepted.
	noComments := strings.TrimSpace(stripCommentsSQL(noDollar))
	validateProperties(noComments,
		`MIN_INSTANCES|MAX_INSTANCES|MIN_READY_INSTANCES|EXTERNAL_ACCESS_INTEGRATIONS|AUTO_RESUME|AUTO_SUSPEND_SECS|QUERY_WAREHOUSE|COMMENT|SPECIFICATION_FILE|SPECIFICATION_TEMPLATE_FILE|LOG_LEVEL`,
		r, &markers)

	return markers
}

// ── validateExecuteService ────────────────────────────────────────────────────

// validateExecuteService checks structural requirements for
// EXECUTE SERVICE <name> statements (job services):
//   - IN COMPUTE POOL <name> is mandatory.
//   - Exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE is required.
//   - MIN_INSTANCES / MAX_INSTANCES are not supported (flagged if present).
//   - Only known properties are accepted.
func validateExecuteService(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	noDollar := stripDollarQuoted(parseText)
	noLiterals := sqltok.StripStrings(noDollar)
	clean := strings.TrimSpace(stripCommentsSQL(noLiterals))

	sig := sigToks(sqltok.Tokenize(clean))

	// 1. Service name is required.
	// EXECUTE [JOB] SERVICE <name>
	name, _ := extractNameAfterKeywords(sig, clean, "EXECUTE", "SERVICE")
	if name == "" {
		// Try with JOB keyword: EXECUTE JOB SERVICE <name>
		name, _ = extractNameAfterKeywords(sig, clean, "EXECUTE", "JOB", "SERVICE")
	}
	if name == "" {
		markers = append(markers, diagMarkerSpan(r, "Unexpected syntax in EXECUTE SERVICE statement.", 4))
		return markers
	}

	// 2. IN COMPUTE POOL is mandatory.
	if !hasKWSeq(sig, clean, "IN", "COMPUTE", "POOL") {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory IN COMPUTE POOL clause in EXECUTE SERVICE statement.", 4))
	}

	// 3. Exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE is required.
	hasSpecFile := hasFromSpecKW(sig, clean, []string{"SPECIFICATION_FILE", "SPECIFICATION_TEMPLATE_FILE"})
	hasSpec := hasFromSpecKW(sig, clean, []string{"SPECIFICATION", "SPECIFICATION_TEMPLATE"})
	if hasSpec && hasSpecFile {
		markers = append(markers, diagMarkerSpan(r,
			"EXECUTE SERVICE requires exactly one of FROM SPECIFICATION or FROM SPECIFICATION_FILE, not both.", 4))
	} else if !hasSpec && !hasSpecFile {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory FROM SPECIFICATION or FROM SPECIFICATION_FILE clause in EXECUTE SERVICE statement.", 4))
	}

	// 4. MIN_INSTANCES / MAX_INSTANCES are not supported for job services.
	if hasKWAssign(sig, clean, "MIN_INSTANCES") {
		markers = append(markers, diagMarkerSpan(r,
			"MIN_INSTANCES is not supported in EXECUTE SERVICE (job services run once).", 4))
	}
	if hasKWAssign(sig, clean, "MAX_INSTANCES") {
		markers = append(markers, diagMarkerSpan(r,
			"MAX_INSTANCES is not supported in EXECUTE SERVICE (job services run once).", 4))
	}

	// 5. Only known properties are accepted.
	noComments := strings.TrimSpace(stripCommentsSQL(noDollar))
	validateProperties(noComments,
		`EXTERNAL_ACCESS_INTEGRATIONS|QUERY_WAREHOUSE|COMMENT|SPECIFICATION_FILE|SPECIFICATION_TEMPLATE_FILE|MIN_INSTANCES|MAX_INSTANCES|NAME|ASYNC|REPLICAS`,
		r, &markers)

	return markers
}

// ── validateAlterService ──────────────────────────────────────────────────────

// validateAlterService checks structural requirements for ALTER SERVICE statements:
//   - A service name is required.
//   - At least one known sub-command must be present (SUSPEND, RESUME,
//     SET MIN_INSTANCES/MAX_INSTANCES/COMMENT/QUERY_WAREHOUSE,
//     UNSET COMMENT/QUERY_WAREHOUSE, FROM SPECIFICATION).
//   - MIN_INSTANCES / MAX_INSTANCES values are validated when used with SET.
func validateAlterService(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	noDollar := stripDollarQuoted(parseText)
	noLiterals := sqltok.StripStrings(noDollar)
	clean := strings.TrimSpace(stripCommentsSQL(noLiterals))

	sig := sigToks(sqltok.Tokenize(clean))

	// 1. Service name is required.
	name, _ := extractNameAfterKeywords(sig, clean, "ALTER", "SERVICE")
	if name == "" {
		// Preserve old regex behavior: ALTER SERVICE IF EXISTS with no name
		// captured "IF" as name, fell through to sub-command check.
		ifExistsNoName := len(sig) >= 4 &&
			tokUpper(sig[2], clean) == "IF" &&
			tokUpper(sig[3], clean) == "EXISTS" &&
			(len(sig) == 4 || !isNonEmptyIdent(sig[4], clean))
		if !ifExistsNoName {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER SERVICE requires a service name.", 4))
			return markers
		}
	}

	// 2. At least one known action must be present.
	hasSuspend := hasKW(sig, clean, "SUSPEND")
	hasResume := hasKW(sig, clean, "RESUME")
	// FROM [@stage] SPECIFICATION* — stage reference tokens may appear between FROM and SPECIFICATION.
	hasFromSpec := hasFromSpecKW(sig, clean, []string{"SPECIFICATION", "SPECIFICATION_TEMPLATE", "SPECIFICATION_FILE", "SPECIFICATION_TEMPLATE_FILE"})

	// Check if SET/UNSET has a known property following it.
	// Bare SET or UNSET (with no following word) is treated as unknown sub-command.
	knownSetProps := []string{"MIN_INSTANCES", "MAX_INSTANCES", "COMMENT", "QUERY_WAREHOUSE"}
	hasKnownSet := false
	hasBareSet := false
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], clean) == "SET" {
			if i+1 < len(sig) && isIdent(sig[i+1]) {
				// SET followed by a word — check if it's a known property.
				prop := tokUpper(sig[i+1], clean)
				for _, kp := range knownSetProps {
					if prop == kp {
						hasKnownSet = true
						break
					}
				}
				if !hasKnownSet {
					hasBareSet = true // SET with unknown property
				}
			}
			break
		}
	}
	knownUnsetProps := []string{"COMMENT", "QUERY_WAREHOUSE", "MIN_INSTANCES", "MAX_INSTANCES"}
	hasKnownUnset := false
	hasBareUnset := false
	for i := 0; i < len(sig); i++ {
		if tokUpper(sig[i], clean) == "UNSET" {
			if i+1 < len(sig) && isIdent(sig[i+1]) {
				prop := tokUpper(sig[i+1], clean)
				for _, kp := range knownUnsetProps {
					if prop == kp {
						hasKnownUnset = true
						break
					}
				}
				if !hasKnownUnset {
					hasBareUnset = true
				}
			}
			break
		}
	}

	anyKnown := hasSuspend || hasResume || hasKnownSet || hasKnownUnset || hasFromSpec
	if !anyKnown {
		if hasBareSet {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown property in ALTER SERVICE SET. Valid properties: MIN_INSTANCES, MAX_INSTANCES, COMMENT, QUERY_WAREHOUSE.", 4))
		} else if hasBareUnset {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown property in ALTER SERVICE UNSET. Valid properties: COMMENT, QUERY_WAREHOUSE, MIN_INSTANCES, MAX_INSTANCES.", 4))
		} else {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown ALTER SERVICE sub-command. Expected SUSPEND, RESUME, SET, UNSET, or FROM SPECIFICATION.", 4))
		}
		return markers
	}

	// 3. Validate MIN_INSTANCES value (non-negative) if present in SET.
	var minInstances int
	hasMinInstances := false
	if v, ok := findKWAssignInt(sig, clean, "MIN_INSTANCES"); ok {
		minInstances = v
		hasMinInstances = true
		if v < 0 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MIN_INSTANCES value %d must be a non-negative integer.", v), 4))
		}
	}

	// 4. Validate MAX_INSTANCES value (non-negative, >= MIN_INSTANCES) if present.
	if v, ok := findKWAssignInt(sig, clean, "MAX_INSTANCES"); ok {
		if v < 0 {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MAX_INSTANCES value %d must be a non-negative integer.", v), 4))
		}
		if hasMinInstances && v < minInstances {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("MAX_INSTANCES (%d) must be >= MIN_INSTANCES (%d).", v, minInstances), 4))
		}
	}

	// 5. Validate allowed properties when SET is used.
	if hasKnownSet || hasBareSet {
		noComments := strings.TrimSpace(stripCommentsSQL(noDollar))
		validateProperties(noComments,
			`MIN_INSTANCES|MAX_INSTANCES|COMMENT|QUERY_WAREHOUSE`,
			r, &markers)
	}

	return markers
}

// ── validateDropService ───────────────────────────────────────────────────────

// validateDropService checks structural requirements for DROP SERVICE:
//   - Service name is required.
func validateDropService(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "SERVICE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DROP SERVICE requires a service name.", 4))
	}

	return markers
}

// ── validateCreateImageRepository ─────────────────────────────────────────────

// validateCreateImageRepository checks structural requirements for
// CREATE [OR REPLACE] IMAGE REPOSITORY [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - A repository name is required.
//   - Image repositories are schema-level objects: three-part names are valid.
//   - Only COMMENT is a valid property.
func validateCreateImageRepository(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE IMAGE REPOSITORY"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Repository name is required.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "IMAGE", "REPOSITORY")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected syntax in CREATE IMAGE REPOSITORY statement.", 4))
		return markers
	}
	if marker, swallowed := checkNameSwallowedByIFTok(name, sig, stripped, r, "Unexpected syntax in CREATE IMAGE REPOSITORY statement."); swallowed {
		markers = append(markers, marker)
		return markers
	}

	// 3. Only COMMENT is a valid property.
	noComments := strings.TrimSpace(stripCommentsSQL(parseText))
	validateProperties(noComments, `COMMENT`, r, &markers)

	return markers
}

// ── validateDropImageRepository ───────────────────────────────────────────────

// validateDropImageRepository checks structural requirements for DROP IMAGE REPOSITORY:
//   - Repository name is required.
func validateDropImageRepository(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "IMAGE", "REPOSITORY")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DROP IMAGE REPOSITORY requires a repository name.", 4))
	}

	return markers
}

// ── validateAlterImageRepository ──────────────────────────────────────────────

// validateAlterImageRepository warns that ALTER IMAGE REPOSITORY is not
// supported in the current Snowflake specification.
func validateAlterImageRepository(_ string, r StatementRange) []DiagMarker {
	return []DiagMarker{
		diagMarkerSpan(r,
			"ALTER IMAGE REPOSITORY is not supported in the current Snowflake specification.", 4),
	}
}

// ── validateCreateApplicationPackage ──────────────────────────────────────────

// validateCreateApplicationPackage checks structural requirements for
// CREATE [OR REPLACE] APPLICATION PACKAGE [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Application packages are account-level objects: name must not have a db.schema prefix.
//   - DISTRIBUTION must be INTERNAL or EXTERNAL if present.
//   - Only DISTRIBUTION and COMMENT are valid properties.
func validateCreateApplicationPackage(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE APPLICATION PACKAGE"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Package name is required; also used for account-level prefix check.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "APPLICATION", "PACKAGE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected syntax in CREATE APPLICATION PACKAGE statement.", 4))
		return markers
	}
	if marker, swallowed := checkNameSwallowedByIFTok(name, sig, stripped, r, "Unexpected syntax in CREATE APPLICATION PACKAGE statement."); swallowed {
		markers = append(markers, marker)
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, "Application packages"); pfx != nil {
		markers = append(markers, *pfx)
	}

	// 3. DISTRIBUTION must be INTERNAL or EXTERNAL if present.
	if distVal, ok := findKWAssign(sig, stripped, "DISTRIBUTION"); ok {
		val := strings.ToUpper(distVal)
		if val != "INTERNAL" && val != "EXTERNAL" {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("DISTRIBUTION must be INTERNAL or EXTERNAL, got '%s'.", distVal), 4))
		}
	}

	// 4. Only DISTRIBUTION and COMMENT are valid properties.
	noComments := strings.TrimSpace(stripCommentsSQL(parseText))
	validateProperties(noComments, `DISTRIBUTION|COMMENT`, r, &markers)

	return markers
}

// ── validateAlterApplicationPackage ───────────────────────────────────────────

// validateAlterApplicationPackage checks structural requirements for
// ALTER APPLICATION PACKAGE <name> statements:
//   - Package name is required.
//   - Must contain a known sub-command (SET DEFAULT RELEASE DIRECTIVE, ADD VERSION,
//     DROP VERSION, SET DISTRIBUTION).
//   - DISTRIBUTION must be INTERNAL or EXTERNAL if present.
func validateAlterApplicationPackage(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Package name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "APPLICATION", "PACKAGE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER APPLICATION PACKAGE requires a package name.", 4))
		return markers
	}

	// 2. At least one known action must be present.
	hasKnownAction := hasKWSeq(sig, stripped, "SET", "DEFAULT", "RELEASE") ||
		hasKWPair(sig, stripped, "SET", "DISTRIBUTION") ||
		hasKWPair(sig, stripped, "ADD", "VERSION") ||
		hasKWPair(sig, stripped, "DROP", "VERSION")
	if !hasKnownAction {
		// Distinguish "unknown SET property" from "unknown sub-command".
		hasSetIdent := false
		for i := 0; i+1 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "SET" && isIdent(sig[i+1]) {
				hasSetIdent = true
				break
			}
		}
		if hasSetIdent {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown property in ALTER APPLICATION PACKAGE SET. Valid properties: DEFAULT RELEASE DIRECTIVE, DISTRIBUTION.", 4))
		} else {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown ALTER APPLICATION PACKAGE sub-command. Expected SET DEFAULT RELEASE DIRECTIVE, ADD VERSION, DROP VERSION, or SET DISTRIBUTION.", 4))
		}
		return markers
	}

	// 3. DISTRIBUTION must be INTERNAL or EXTERNAL if present.
	if distVal, ok := findKWAssign(sig, stripped, "DISTRIBUTION"); ok {
		val := strings.ToUpper(distVal)
		if val != "INTERNAL" && val != "EXTERNAL" {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("DISTRIBUTION must be INTERNAL or EXTERNAL, got '%s'.", distVal), 4))
		}
	}

	return markers
}

// ── validateDropApplicationPackage ────────────────────────────────────────────

// validateDropApplicationPackage checks structural requirements for
// DROP APPLICATION PACKAGE [IF EXISTS] <name>:
//   - Package name is required.
func validateDropApplicationPackage(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "APPLICATION", "PACKAGE")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DROP APPLICATION PACKAGE requires a package name.", 4))
	}

	return markers
}

// ── validateCreateApplication ─────────────────────────────────────────────────

// validateCreateApplication checks structural requirements for
// CREATE [OR REPLACE] APPLICATION [IF NOT EXISTS] <name> FROM APPLICATION PACKAGE <pkg>:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Applications are account-level objects: name must not have a db.schema prefix.
//   - FROM APPLICATION PACKAGE clause is mandatory.
//   - If USING VERSION is specified, PATCH is also required.
//   - DEBUG_MODE must be TRUE or FALSE if present.
//   - Only USING, DEBUG_MODE, and COMMENT are valid properties.
func validateCreateApplication(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE APPLICATION"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Application name is required.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "APPLICATION")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected syntax in CREATE APPLICATION statement.", 4))
		return markers
	}
	if marker, swallowed := checkNameSwallowedByIFTok(name, sig, stripped, r, "Unexpected syntax in CREATE APPLICATION statement."); swallowed {
		markers = append(markers, marker)
		return markers
	}
	if pfx := checkAccountLevelPrefix(name, r, "Applications"); pfx != nil {
		markers = append(markers, *pfx)
	}

	// 3. FROM APPLICATION PACKAGE is mandatory.
	if !hasKWSeq(sig, stripped, "FROM", "APPLICATION", "PACKAGE") {
		markers = append(markers, diagMarkerSpan(r,
			"Missing mandatory FROM APPLICATION PACKAGE clause in CREATE APPLICATION statement.", 4))
	}

	// 4. If USING VERSION is present, PATCH must also be present.
	if hasKWPair(sig, stripped, "USING", "VERSION") && !hasKW(sig, stripped, "PATCH") {
		markers = append(markers, diagMarkerSpan(r,
			"USING VERSION requires a PATCH number in CREATE APPLICATION statement.", 4))
	}

	// 5. DEBUG_MODE must be TRUE or FALSE if present.
	validateBoolPropTok(sig, stripped, "DEBUG_MODE", r, &markers)

	// 6. Only known properties are accepted.
	noComments := strings.TrimSpace(stripCommentsSQL(parseText))
	validateProperties(noComments, `DEBUG_MODE|COMMENT`, r, &markers)

	return markers
}

// ── validateAlterApplication ──────────────────────────────────────────────────

// validateAlterApplication checks structural requirements for
// ALTER APPLICATION <name> statements:
//   - Application name is required.
//   - Must contain a known sub-command (UPGRADE, SET DEBUG_MODE, UNSET DEBUG_MODE).
//   - DEBUG_MODE must be TRUE or FALSE when used with SET.
func validateAlterApplication(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Application name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "APPLICATION")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER APPLICATION requires an application name.", 4))
		return markers
	}

	// 2. At least one known action must be present.
	hasKnownAction := hasKW(sig, stripped, "UPGRADE") ||
		hasKWPair(sig, stripped, "SET", "DEBUG_MODE") ||
		hasKWPair(sig, stripped, "UNSET", "DEBUG_MODE")
	if !hasKnownAction {
		// Distinguish "unknown property within SET/UNSET" vs "unknown sub-command".
		hasSetIdent := false
		hasUnsetIdent := false
		for i := 0; i+1 < len(sig); i++ {
			u := tokUpper(sig[i], stripped)
			if u == "SET" && isIdent(sig[i+1]) {
				hasSetIdent = true
			}
			if u == "UNSET" && isIdent(sig[i+1]) {
				hasUnsetIdent = true
			}
		}
		if hasSetIdent {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown property in ALTER APPLICATION SET. Valid properties: DEBUG_MODE.", 4))
		} else if hasUnsetIdent {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown property in ALTER APPLICATION UNSET. Valid properties: DEBUG_MODE.", 4))
		} else {
			markers = append(markers, diagMarkerSpan(r,
				"Unknown ALTER APPLICATION sub-command. Expected UPGRADE, SET DEBUG_MODE, or UNSET DEBUG_MODE.", 4))
		}
		return markers
	}

	// 3. If UPGRADE USING VERSION is present, PATCH must also be present.
	if hasKWPair(sig, stripped, "USING", "VERSION") && !hasKW(sig, stripped, "PATCH") {
		markers = append(markers, diagMarkerSpan(r,
			"USING VERSION requires a PATCH number in ALTER APPLICATION UPGRADE.", 4))
	}

	// 4. DEBUG_MODE must be TRUE or FALSE when used with SET.
	validateBoolPropTok(sig, stripped, "DEBUG_MODE", r, &markers)

	return markers
}

// ── validateDropApplication ───────────────────────────────────────────────────

// validateDropApplication checks structural requirements for
// DROP APPLICATION [IF EXISTS] <name> [CASCADE]:
//   - Application name is required.
func validateDropApplication(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "APPLICATION")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DROP APPLICATION requires an application name.", 4))
	}

	return markers
}

// ── validateCreateGitRepository ──────────────────────────────────────────────

// validateCreateGitRepository checks structural requirements for
// CREATE [OR REPLACE] GIT REPOSITORY [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Schema-level object: three-part names (db.schema.name) are valid.
//   - API_INTEGRATION = <name> is mandatory.
//   - ORIGIN = '<url>' is mandatory; value must start with https:// or git@.
//   - GIT_CREDENTIALS and COMMENT are optional valid properties.
func validateCreateGitRepository(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE GIT REPOSITORY"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Repository name is required.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "GIT", "REPOSITORY")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected syntax in CREATE GIT REPOSITORY statement.", 4))
		return markers
	}
	if marker, swallowed := checkNameSwallowedByIFTok(name, sig, stripped, r, "Unexpected syntax in CREATE GIT REPOSITORY statement."); swallowed {
		markers = append(markers, marker)
		return markers
	}

	// 3. API_INTEGRATION is mandatory — needs KEYWORD = <ident>.
	if _, ok := findKWAssign(sig, stripped, "API_INTEGRATION"); !ok {
		markers = append(markers, diagMarkerSpan(r,
			"CREATE GIT REPOSITORY requires API_INTEGRATION = <integration_name>.", 4))
	}

	// 4. ORIGIN is mandatory and must be a valid-looking URL.
	// Use the original (with string literals) for URL value extraction.
	origSig := sigToks(sqltok.Tokenize(strings.TrimSpace(stripCommentsSQL(parseText))))
	originURL, hasOriginStr := findKWAssignStr(origSig, strings.TrimSpace(stripCommentsSQL(parseText)), "ORIGIN")
	if !hasOriginStr {
		if hasKWAssign(sig, stripped, "ORIGIN") {
			// ORIGIN = is present but value is not a string literal.
			markers = append(markers, diagMarkerSpan(r,
				"ORIGIN value must be a string literal (e.g. ORIGIN = 'https://...').", 4))
		} else {
			markers = append(markers, diagMarkerSpan(r,
				"CREATE GIT REPOSITORY requires ORIGIN = '<url>'.", 4))
		}
	} else {
		if !strings.HasPrefix(strings.ToLower(originURL), "https://") && !strings.HasPrefix(originURL, "git@") {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("ORIGIN URL should start with 'https://' or 'git@', got '%s'.", originURL), 4))
		}
	}

	// 5. Only API_INTEGRATION, ORIGIN, GIT_CREDENTIALS, and COMMENT are valid properties.
	noComments := strings.TrimSpace(stripCommentsSQL(parseText))
	validateProperties(noComments, `API_INTEGRATION|ORIGIN|GIT_CREDENTIALS|COMMENT`, r, &markers)

	return markers
}

// ── validateAlterGitRepository ───────────────────────────────────────────────

// validateAlterGitRepository checks structural requirements for ALTER GIT REPOSITORY:
//   - Repository name is required.
//   - FETCH — triggers a sync; no arguments.
//   - SET API_INTEGRATION / GIT_CREDENTIALS / COMMENT — valid SET targets.
//   - UNSET GIT_CREDENTIALS / COMMENT — valid UNSET targets.
//   - Unknown sub-commands warn.
func validateAlterGitRepository(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Repository name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "GIT", "REPOSITORY")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER GIT REPOSITORY requires a repository name.", 4))
		return markers
	}

	// 2. Check for known sub-commands.
	hasKnownAction := hasKW(sig, stripped, "FETCH") ||
		hasKWPair(sig, stripped, "SET", "API_INTEGRATION") ||
		hasKWPair(sig, stripped, "SET", "GIT_CREDENTIALS") ||
		hasKWPair(sig, stripped, "SET", "COMMENT") ||
		hasKWPair(sig, stripped, "UNSET", "GIT_CREDENTIALS") ||
		hasKWPair(sig, stripped, "UNSET", "COMMENT")
	if !hasKnownAction {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER GIT REPOSITORY sub-command. Expected FETCH, SET API_INTEGRATION/GIT_CREDENTIALS/COMMENT, or UNSET GIT_CREDENTIALS/COMMENT.", 4))
	}

	return markers
}

// ── validateDropGitRepository ────────────────────────────────────────────────

// validateDropGitRepository checks structural requirements for DROP GIT REPOSITORY:
//   - Repository name is required.
func validateDropGitRepository(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "GIT", "REPOSITORY")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DROP GIT REPOSITORY requires a repository name.", 4))
	}

	return markers
}

// ── CREATE SECRET data-driven type validation ────────────────────────────────

// secretPropDef associates a keyword with its display name and owning type(s).
type secretPropDef struct {
	keyword string // e.g. "API_AUTHENTICATION"
	name    string
	owner   string // human-readable owner type(s)
}

// secretMandatoryDef describes a mandatory property for a given TYPE.
type secretMandatoryDef struct {
	keyword string // e.g. "API_AUTHENTICATION"
	hint    string // e.g. "API_AUTHENTICATION = <security_integration_name>"
}

// secretTypeAllowed maps each TYPE to the set of type-specific property keywords
// that are valid for it. Properties not in this set trigger a cross-type warning.
var secretTypeAllowed = map[string]map[string]bool{
	"OAUTH2":               {"API_AUTHENTICATION": true, "OAUTH_SCOPES": true, "OAUTH_REFRESH_TOKEN": true, "OAUTH_REFRESH_TOKEN_EXPIRY_TIME": true},
	"PASSWORD":             {"USERNAME": true, "PASSWORD": true},
	"GENERIC_STRING":       {"SECRET_STRING": true},
	"CLOUD_PROVIDER_TOKEN": {"API_AUTHENTICATION": true, "ENABLED": true},
	"SYMMETRIC_KEY":        {"ALGORITHM": true},
}

// secretTypeMandatory maps each TYPE to its mandatory properties.
var secretTypeMandatory = map[string][]secretMandatoryDef{
	"OAUTH2":               {{"API_AUTHENTICATION", "API_AUTHENTICATION = <security_integration_name>"}},
	"PASSWORD":             {{"USERNAME", "USERNAME = '<username>'"}, {"PASSWORD", "PASSWORD = '<password>'"}},
	"GENERIC_STRING":       {{"SECRET_STRING", "SECRET_STRING = '<value>'"}},
	"CLOUD_PROVIDER_TOKEN": {{"API_AUTHENTICATION", "API_AUTHENTICATION = <security_integration_name>"}},
	"SYMMETRIC_KEY":        {{"ALGORITHM", "ALGORITHM = '<algorithm>'"}},
}

// secretTypedProps lists all type-specific properties with their owning type.
// This is iterated for cross-type violation detection.
var secretTypedProps = []secretPropDef{
	{"API_AUTHENTICATION", "API_AUTHENTICATION", "OAUTH2 or CLOUD_PROVIDER_TOKEN"},
	{"USERNAME", "USERNAME", "PASSWORD"},
	{"PASSWORD", "PASSWORD", "PASSWORD"},
	{"SECRET_STRING", "SECRET_STRING", "GENERIC_STRING"},
	{"ENABLED", "ENABLED", "CLOUD_PROVIDER_TOKEN"},
	{"ALGORITHM", "ALGORITHM", "SYMMETRIC_KEY"},
	{"OAUTH_SCOPES", "OAUTH_SCOPES", "OAUTH2"},
	{"OAUTH_REFRESH_TOKEN", "OAUTH_REFRESH_TOKEN", "OAUTH2"},
	{"OAUTH_REFRESH_TOKEN_EXPIRY_TIME", "OAUTH_REFRESH_TOKEN_EXPIRY_TIME", "OAUTH2"},
}

// ── validateCreateSecret ─────────────────────────────────────────────────────

// validateCreateSecret checks structural requirements for
// CREATE [OR REPLACE] SECRET [IF NOT EXISTS] <name> statements:
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - Schema-level object: three-part names (db.schema.name) are valid.
//   - TYPE is mandatory; must be OAUTH2, PASSWORD, GENERIC_STRING, CLOUD_PROVIDER_TOKEN, or SYMMETRIC_KEY.
//   - TYPE = OAUTH2 requires API_AUTHENTICATION.
//   - TYPE = PASSWORD requires USERNAME and PASSWORD.
//   - TYPE = GENERIC_STRING requires SECRET_STRING.
//   - TYPE = CLOUD_PROVIDER_TOKEN requires API_AUTHENTICATION.
//   - TYPE = SYMMETRIC_KEY requires ALGORITHM.
//   - Properties belonging to a different TYPE are flagged.
func validateCreateSecret(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE SECRET"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Secret name is required.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "SECRET")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected syntax in CREATE SECRET statement.", 4))
		return markers
	}
	if marker, swallowed := checkNameSwallowedByIFTok(name, sig, stripped, r, "Unexpected syntax in CREATE SECRET statement."); swallowed {
		markers = append(markers, marker)
		return markers
	}

	// 3. TYPE is mandatory.
	typeVal, hasType := findKWAssign(sig, stripped, "TYPE")
	if !hasType {
		markers = append(markers, diagMarkerSpan(r,
			"CREATE SECRET requires TYPE = OAUTH2 | PASSWORD | GENERIC_STRING | CLOUD_PROVIDER_TOKEN | SYMMETRIC_KEY.", 4))
		return markers
	}

	secretType := strings.ToUpper(typeVal)

	// 4. Validate TYPE value and type-specific properties.
	allowed, ok := secretTypeAllowed[secretType]
	if !ok {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("Unknown TYPE '%s'. Valid types: OAUTH2, PASSWORD, GENERIC_STRING, CLOUD_PROVIDER_TOKEN, SYMMETRIC_KEY.", typeVal), 4))
		return markers
	}

	// Check mandatory properties.
	if mandatory, hasMandatory := secretTypeMandatory[secretType]; hasMandatory {
		for _, mp := range mandatory {
			if !hasKWAssign(sig, stripped, mp.keyword) {
				markers = append(markers, diagMarkerSpan(r,
					fmt.Sprintf("TYPE = %s requires %s.", secretType, mp.hint), 4))
			}
		}
	}

	// Cross-type property checks.
	for _, sp := range secretTypedProps {
		if allowed[sp.keyword] {
			continue // property is valid for this TYPE
		}
		if hasKWAssign(sig, stripped, sp.keyword) {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("%s is not valid for TYPE = %s. %s belongs to TYPE = %s.",
					sp.name, secretType, sp.name, sp.owner), 4))
		}
	}

	// 5. Only known properties are accepted.
	validateProperties(parseText, `TYPE|API_AUTHENTICATION|OAUTH_SCOPES|OAUTH_REFRESH_TOKEN|OAUTH_REFRESH_TOKEN_EXPIRY_TIME|USERNAME|PASSWORD|SECRET_STRING|ENABLED|ALGORITHM|COMMENT`, r, &markers)

	return markers
}

// ── validateAlterSecret ──────────────────────────────────────────────────────

// validateAlterSecret checks structural requirements for ALTER SECRET [IF EXISTS]:
//   - Secret name is required.
//   - SET SECRET_STRING / USERNAME / PASSWORD / OAUTH_REFRESH_TOKEN /
//     OAUTH_REFRESH_TOKEN_EXPIRY_TIME / OAUTH_SCOPES / API_AUTHENTICATION /
//     COMMENT are valid SET targets.
//   - UNSET COMMENT is a valid UNSET target.
//   - Unknown sub-commands warn.
func validateAlterSecret(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Secret name is required.
	// Note: extractNameAfterKeywords skips IF EXISTS, so
	// "ALTER SECRET IF EXISTS" (no name after) returns "". The old regex
	// captured "IF" as the name via backtracking and then fell through to the
	// sub-command check. Preserve that by detecting this specific pattern.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "SECRET")
	if name == "" {
		// Special case: "ALTER SECRET IF EXISTS" (no name) — fall through to
		// sub-command check rather than returning "missing name" immediately.
		ifExistsNoName := len(sig) >= 4 &&
			tokUpper(sig[2], stripped) == "IF" &&
			tokUpper(sig[3], stripped) == "EXISTS" &&
			(len(sig) == 4 || !isNonEmptyIdent(sig[4], stripped))
		if !ifExistsNoName {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER SECRET requires a secret name.", 4))
			return markers
		}
	}

	// 2. Check for known sub-commands.
	validSetProps := []string{"SECRET_STRING", "USERNAME", "PASSWORD", "OAUTH_REFRESH_TOKEN",
		"OAUTH_REFRESH_TOKEN_EXPIRY_TIME", "OAUTH_SCOPES", "API_AUTHENTICATION", "COMMENT"}
	hasKnownAction := false
	for _, prop := range validSetProps {
		if hasKWPair(sig, stripped, "SET", prop) {
			hasKnownAction = true
			break
		}
	}
	if !hasKnownAction {
		hasKnownAction = hasKWPair(sig, stripped, "UNSET", "COMMENT")
	}
	if !hasKnownAction {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER SECRET sub-command. Expected SET SECRET_STRING/USERNAME/PASSWORD/OAUTH_REFRESH_TOKEN/OAUTH_REFRESH_TOKEN_EXPIRY_TIME/OAUTH_SCOPES/API_AUTHENTICATION/COMMENT, or UNSET COMMENT.", 4))
	}

	return markers
}

// ── validateCreateNotebook ──────────────────────────────────────────────────

// validateCreateNotebook checks structural requirements for CREATE NOTEBOOK:
//   - Notebook name is mandatory.
//   - OR REPLACE and IF NOT EXISTS are mutually exclusive.
//   - When FROM is specified, MAIN_FILE is required.
func validateCreateNotebook(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. OR REPLACE and IF NOT EXISTS are mutually exclusive.
	if marker, conflict := checkOrReplaceConflictTok(sig, stripped, r, "CREATE NOTEBOOK"); conflict {
		markers = append(markers, marker)
		return markers
	}

	// 2. Notebook name is required.
	name, _ := extractNameAfterCreate(sig, stripped, nil, "NOTEBOOK")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"CREATE NOTEBOOK requires a notebook name.", 4))
		return markers
	}

	// 3. When FROM is specified, MAIN_FILE is required.
	// Check FROM followed by a string literal in token stream.
	hasFrom := false
	for i := 0; i+1 < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "FROM" && sig[i+1].Kind == sqltok.StringLit {
			hasFrom = true
			break
		}
	}
	if hasFrom && !hasKWAssign(sig, stripped, "MAIN_FILE") {
		markers = append(markers, diagMarkerSpan(r,
			"MAIN_FILE is required when FROM is specified in CREATE NOTEBOOK.", 4))
	}

	return markers
}

// ── validateAlterNotebook ───────────────────────────────────────────────────

// validateAlterNotebook checks structural requirements for ALTER NOTEBOOK:
//   - Notebook name is mandatory.
//   - Known sub-commands: SET, UNSET, RENAME TO, ADD LIVE VERSION.
//   - RENAME TO requires a target name.
func validateAlterNotebook(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Notebook name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "ALTER", "NOTEBOOK")
	if name == "" {
		// When the old regex captured "IF" as the name (ALTER NOTEBOOK IF EXISTS
		// with no actual name), it fell through to sub-command detection.
		ifExistsNoName := len(sig) >= 4 &&
			tokUpper(sig[2], stripped) == "IF" &&
			tokUpper(sig[3], stripped) == "EXISTS" &&
			(len(sig) == 4 || !isNonEmptyIdent(sig[4], stripped))
		if !ifExistsNoName {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER NOTEBOOK requires a notebook name.", 4))
			return markers
		}
	}

	// 2. Check for known sub-commands.
	hasSet := hasKW(sig, stripped, "SET")
	hasUnset := hasKW(sig, stripped, "UNSET")
	hasRename := hasKWPair(sig, stripped, "RENAME", "TO")
	hasAddLive := hasKWSeq(sig, stripped, "ADD", "LIVE", "VERSION")
	if !hasSet && !hasUnset && !hasRename && !hasAddLive {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER NOTEBOOK sub-command. Expected SET, UNSET, RENAME TO, or ADD LIVE VERSION FROM LAST.", 4))
		return markers
	}

	// 3. RENAME TO requires a target name.
	if hasRename {
		// Find RENAME TO and check for a name after it.
		found := false
		for i := 0; i+2 < len(sig); i++ {
			if tokUpper(sig[i], stripped) == "RENAME" && tokUpper(sig[i+1], stripped) == "TO" {
				if i+2 < len(sig) && isNonEmptyIdent(sig[i+2], stripped) {
					found = true
				}
				break
			}
		}
		if !found {
			markers = append(markers, diagMarkerSpan(r,
				"RENAME TO requires a new notebook name.", 4))
		}
	}

	// 4. ADD LIVE VERSION requires FROM LAST.
	if hasAddLive && !hasKWPair(sig, stripped, "FROM", "LAST") {
		markers = append(markers, diagMarkerSpan(r,
			"ADD LIVE VERSION requires FROM LAST.", 4))
	}

	return markers
}

// ── validateDropNotebook ────────────────────────────────────────────────────

// validateDropNotebook checks structural requirements for DROP NOTEBOOK:
//   - Notebook name is mandatory.
//   - CASCADE / RESTRICT are not valid for DROP NOTEBOOK.
func validateDropNotebook(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Notebook name is required.
	name, _ := extractNameAfterKeywords(sig, stripped, "DROP", "NOTEBOOK")
	if name == "" {
		markers = append(markers, diagMarkerSpan(r,
			"DROP NOTEBOOK requires a notebook name.", 4))
		return markers
	}

	// 2. CASCADE / RESTRICT are not valid for DROP NOTEBOOK.
	if len(sig) > 0 {
		lastKW := tokUpper(sig[len(sig)-1], stripped)
		if lastKW == "CASCADE" || lastKW == "RESTRICT" {
			markers = append(markers, diagMarkerSpan(r,
				"CASCADE / RESTRICT are not valid for DROP NOTEBOOK.", 4))
		}
	}

	return markers
}

// ── validateAlterTableSearchOptimization ────────────────────────────────────

// validateAlterTableSearchOptimization checks structural requirements for
// ALTER TABLE <name> ADD/DROP SEARCH OPTIMIZATION:
//   - Bare ADD/DROP SEARCH OPTIMIZATION (no ON clause) is valid.
//   - ON <expression_list>: each expression must be EQUALITY, SUBSTRING, GEO,
//     or FULL_TEXT. Unknown expression type names are flagged.
func validateAlterTableSearchOptimization(stripped string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	clean := strings.TrimSpace(sqltok.StripStrings(stripped))
	soSig := sigToks(sqltok.Tokenize(clean))

	// Find SEARCH OPTIMIZATION ON sequence in tokens.
	onIdx := -1
	for i := 0; i+2 < len(soSig); i++ {
		if tokUpper(soSig[i], clean) == "SEARCH" &&
			tokUpper(soSig[i+1], clean) == "OPTIMIZATION" &&
			tokUpper(soSig[i+2], clean) == "ON" {
			onIdx = i + 2
			break
		}
	}
	if onIdx < 0 {
		return markers
	}

	// Extract the text after ON.
	onBody := ""
	if onIdx+1 < len(soSig) {
		onBody = strings.TrimSpace(clean[soSig[onIdx].End:])
	}

	if onBody == "" {
		markers = append(markers, diagMarkerSpan(r,
			"SEARCH OPTIMIZATION ON requires at least one expression (e.g. EQUALITY, SUBSTRING, GEO, FULL_TEXT).", 4))
		return markers
	}

	// Split the ON body into top-level comma-separated expressions and
	// validate each expression type.
	exprs := splitTopLevelCommas(onBody)
	for _, expr := range exprs {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		// Token-scan each expression for IDENTIFIER( pattern.
		exprSig := sigToks(sqltok.Tokenize(expr))
		if len(exprSig) < 2 || !isIdent(exprSig[0]) || exprSig[1].Kind != sqltok.LParen {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Invalid search optimization expression: %q. Expected EQUALITY, SUBSTRING, GEO, or FULL_TEXT.", expr), 4))
			continue
		}
		funcName := strings.ToUpper(exprSig[0].Text(expr))
		if !searchOptValidExprs[funcName] {
			markers = append(markers, diagMarkerSpan(r,
				fmt.Sprintf("Unknown search optimization type %q. Valid types are EQUALITY, SUBSTRING, GEO, FULL_TEXT.", funcName), 4))
		}
		// Check for trailing content after the closing paren of the
		// expression — a missing comma between expressions like
		// EQUALITY(c1) SUBSTRING(c2) would otherwise go undetected.
		if openIdx := strings.Index(expr, "("); openIdx != -1 {
			closeIdx := findMatchingParen(expr[openIdx:])
			if closeIdx != -1 {
				trailing := strings.TrimRight(strings.TrimSpace(expr[openIdx+closeIdx+1:]), "; ")
				if trailing != "" {
					markers = append(markers, diagMarkerSpan(r,
						fmt.Sprintf("Unexpected trailing content after search optimization expression %s(...): %q. Separate multiple expressions with commas.", funcName, trailing), 4))
				}
			}
		}
	}

	return markers
}

// ── validateAlterDynamicTable ───────────────────────────────────────────────

// validateAlterDynamicTable checks structural requirements for
// ALTER DYNAMIC TABLE lifecycle commands:
//   - Table name is mandatory.
//   - Known sub-commands: REFRESH, SUSPEND, RESUME, SET, UNSET, SWAP WITH, RENAME TO.
//   - SWAP WITH requires a target table name.
//   - RENAME TO requires a new name.
//   - SET TARGET_LAG value must be a valid lag time or DOWNSTREAM.
//   - Unknown sub-commands produce a warning.
func validateAlterDynamicTable(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Table name is required.
	name, nameIdx := extractNameAfterKeywords(sig, stripped, "ALTER", "DYNAMIC", "TABLE")
	var afterName []sqltok.Token
	if name == "" {
		// Handle IF EXISTS without a name (old regex captured "IF" as name).
		ifExistsNoName := len(sig) >= 5 &&
			tokUpper(sig[3], stripped) == "IF" &&
			tokUpper(sig[4], stripped) == "EXISTS" &&
			(len(sig) == 5 || !isNonEmptyIdent(sig[5], stripped))
		if !ifExistsNoName {
			markers = append(markers, diagMarkerSpan(r,
				"ALTER DYNAMIC TABLE requires a table name.", 4))
			return markers
		}
		// No name — sub-commands start after IF EXISTS
		if 5 < len(sig) {
			afterName = sig[5:]
		}
	} else {
		// Skip past the identifier path to find sub-commands.
		_, pathEnd := readIdentPath(sig, stripped, nameIdx)
		if pathEnd < len(sig) {
			afterName = sig[pathEnd:]
		}
	}

	hasRefresh := hasKW(afterName, stripped, "REFRESH")
	hasSuspend := hasKW(afterName, stripped, "SUSPEND")
	hasResume := hasKW(afterName, stripped, "RESUME")
	hasSet := hasKW(afterName, stripped, "SET")
	hasUnset := hasKW(afterName, stripped, "UNSET")
	hasSwap := hasKWPair(afterName, stripped, "SWAP", "WITH")
	hasRename := hasKWPair(afterName, stripped, "RENAME", "TO")

	anyKnown := hasRefresh || hasSuspend || hasResume || hasSet || hasUnset || hasSwap || hasRename

	if !anyKnown {
		markers = append(markers, diagMarkerSpan(r,
			"Unknown ALTER DYNAMIC TABLE sub-command. Expected REFRESH, SUSPEND, RESUME, SET, UNSET, SWAP WITH, or RENAME TO.", 4))
		return markers
	}

	// Count sub-commands — Snowflake only allows one per statement.
	subCmdCount := 0
	for _, has := range []bool{hasRefresh, hasSuspend, hasResume, hasSet, hasUnset, hasSwap, hasRename} {
		if has {
			subCmdCount++
		}
	}
	if subCmdCount > 1 {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER DYNAMIC TABLE supports only one sub-command per statement.", 4))
	}

	// 3. SWAP WITH requires a target table name.
	if hasSwap {
		hasTarget := false
		for i := 0; i+2 < len(afterName); i++ {
			if tokUpper(afterName[i], stripped) == "SWAP" && tokUpper(afterName[i+1], stripped) == "WITH" {
				if i+2 < len(afterName) && isIdent(afterName[i+2]) {
					hasTarget = true
				}
				break
			}
		}
		if !hasTarget {
			markers = append(markers, diagMarkerSpan(r,
				"SWAP WITH requires a target table name.", 4))
		}
	}

	// 4. RENAME TO requires a new name.
	if hasRename {
		hasTarget := false
		for i := 0; i+2 < len(afterName); i++ {
			if tokUpper(afterName[i], stripped) == "RENAME" && tokUpper(afterName[i+1], stripped) == "TO" {
				if i+2 < len(afterName) && isIdent(afterName[i+2]) {
					hasTarget = true
				}
				break
			}
		}
		if !hasTarget {
			markers = append(markers, diagMarkerSpan(r,
				"RENAME TO requires a new table name.", 4))
		}
	}

	// 5. Bare SET / UNSET without a property name.
	if hasSet && !hasUnset && len(afterName) == 1 && tokUpper(afterName[0], stripped) == "SET" {
		markers = append(markers, diagMarkerSpan(r,
			"SET requires at least one property (e.g. TARGET_LAG, WAREHOUSE).", 4))
	}
	if hasUnset && !hasSet && len(afterName) == 1 && tokUpper(afterName[0], stripped) == "UNSET" {
		markers = append(markers, diagMarkerSpan(r,
			"UNSET requires at least one property name.", 4))
	}

	// 6. SET TARGET_LAG value validation.
	if hasSet && hasKWAssign(afterName, stripped, "TARGET_LAG") {
		valid := false
		for i := 0; i+2 < len(afterName); i++ {
			if tokUpper(afterName[i], stripped) != "TARGET_LAG" {
				continue
			}
			if afterName[i+1].Kind != sqltok.Operator || afterName[i+1].Text(stripped) != "=" {
				continue
			}
			valTok := afterName[i+2]
			if tokUpper(valTok, stripped) == "DOWNSTREAM" {
				valid = true
			} else if valTok.Kind == sqltok.StringLit {
				// Validate quoted duration: must be '<positive_int> <unit>'
				raw := valTok.Text(stripped)
				if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
					valid = isValidTargetLagDuration(raw[1 : len(raw)-1])
				}
			}
			break
		}
		if !valid {
			markers = append(markers, diagMarkerSpan(r,
				"Invalid TARGET_LAG value. Expected a quoted duration (e.g. '1 minute') or DOWNSTREAM.", 4))
		}
	}

	return markers
}

// ── validateAlterTableSwapWith ──────────────────────────────────────────────

// validateAlterTableSwapWith checks structural requirements for
// ALTER TABLE <name> SWAP WITH <other_name>:
//   - Both source and target table names are required.
//   - Source and target must be different identifiers (same name is a no-op).
//   - No additional clauses are allowed after the target table name.
func validateAlterTableSwapWith(parseText string, r StatementRange) []DiagMarker {
	var markers []DiagMarker

	stripped := stripCommentsSQL(parseText)
	sig := sigToks(sqltok.Tokenize(stripped))

	// 1. Extract source table name: ALTER TABLE [IF EXISTS] <name> SWAP WITH ...
	srcName, srcEnd := extractNameAfterKeywords(sig, stripped, "ALTER", "TABLE")
	if srcName == "" {
		markers = append(markers, diagMarkerSpan(r,
			"ALTER TABLE … SWAP WITH requires a table name.", 4))
		return markers
	}
	srcParts := normalizeSnowflakeIdent(srcName)

	// 2. Target table name is required after SWAP WITH.
	var tgtName string
	var tgtEnd int
	for i := srcEnd; i+2 < len(sig); i++ {
		if tokUpper(sig[i], stripped) == "SWAP" && tokUpper(sig[i+1], stripped) == "WITH" {
			if i+2 < len(sig) && isIdent(sig[i+2]) {
				tgtName, tgtEnd = readIdentPath(sig, stripped, i+2)
			}
			break
		}
	}
	if tgtName == "" {
		markers = append(markers, diagMarkerSpan(r,
			"SWAP WITH requires a target table name.", 4))
		return markers
	}
	tgtParts := normalizeSnowflakeIdent(tgtName)

	// 3. Source and target must be different (same name is a no-op).
	if slices.Equal(srcParts, tgtParts) {
		markers = append(markers, diagMarkerSpan(r,
			fmt.Sprintf("SWAP WITH the same table '%s' is a no-op.", tgtName), 4))
	}

	// 4. No additional clauses after the target table name.
	// Skip any trailing semicolons.
	hasTrailing := false
	for i := tgtEnd; i < len(sig); i++ {
		if sig[i].Kind != sqltok.Semicolon {
			hasTrailing = true
			break
		}
	}
	if hasTrailing {
		markers = append(markers, diagMarkerSpan(r,
			"Unexpected clause after SWAP WITH target table. SWAP WITH must be the final clause.", 4))
	}

	return markers
}

// normalizeSnowflakeIdent normalises a possibly multi-part Snowflake identifier
// (e.g. db.schema.table) into a slice of canonical parts. Unquoted parts are
// uppercased (case-insensitive in Snowflake). Quoted parts preserve their
// exact case (case-sensitive in Snowflake), with escaped "" unescaped.
// This enables correct same-table comparisons: "orders" != ORDERS,
// "A.B" (1 part) != A.B (2 parts), "lower" != "LOWER".
func normalizeSnowflakeIdent(s string) []string {
	rawParts := splitIdentParts(s)
	normalized := make([]string, len(rawParts))
	for i, p := range rawParts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
			inner := p[1 : len(p)-1]
			normalized[i] = strings.ReplaceAll(inner, `""`, `"`)
		} else {
			normalized[i] = strings.ToUpper(p)
		}
	}
	return normalized
}

// splitIdentParts splits a multi-part identifier on dots that are NOT inside
// double quotes. "A.B" is a single part; A.B is two parts.
func splitIdentParts(s string) []string {
	var parts []string
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			inQuote = !inQuote
		} else if s[i] == '.' && !inQuote {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
