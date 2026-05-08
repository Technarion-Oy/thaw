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
	"regexp"
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
			`|FILE\s+FORMAT|ALERT|SHARE` +
			`|NETWORK|ROW\s+ACCESS` +
			`|SESSION|PASSWORD|REPLICATION|FAILOVER|APPLICATION)\b` +
			`|ALTER\s+(?:TABLE|VIEW|STREAM|DATABASE|STAGE|PIPE|PROCEDURE|FUNCTION` +
			`|ALERT|SHARE|EXTERNAL|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK` +
			`|REPLICATION|FAILOVER)\b` +
			`|DROP\s+(?:TABLE|VIEW|STREAM|STAGE|PIPE|PROCEDURE|FUNCTION)\b` +
			`|UNDROP\s+(?:DATABASE|SCHEMA|TABLE)\b` +
			`|INSERT\s+OVERWRITE\b` +
			`|TRUNCATE\s+\S+\s+IF\b` +
			`|\bLATERAL\s+FLATTEN\b` +
			`|\bINFER_SCHEMA\b`,
	)

	// ── Custom check patterns ─────────────────────────────────────────────────
	reLateralFlatten    = regexp.MustCompile(`(?i)\bLATERALFLATTEN\b`)
	reFlattenFromJoin   = regexp.MustCompile(`(?i)(?:FROM|JOIN|,)\s+FLATTEN\s*\(`)
	reLateralOK         = regexp.MustCompile(`(?i)\bLATERAL\s+FLATTEN\s*\(`)
	reTableFlatten      = regexp.MustCompile(`(?i)\bTABLE\s*\(\s*FLATTEN\s*\(`)
	reQualifyAfterOrder = regexp.MustCompile(`(?is)\bORDER\s+BY[\s\S]+?\bQUALIFY\b`)
	reVariantDotPath    = regexp.MustCompile(`(?i)\b([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\b`)
	reOrReplace         = regexp.MustCompile(`(?i)\bOR\s+REPLACE\b`)
	reIfNotExists       = regexp.MustCompile(`(?i)\bIF\s+NOT\s+EXISTS\b`)
	reStripStringLiterals = regexp.MustCompile(`'(?:''|[^'])*'`)
	// rePatternClusterBy — distinct from the CLUSTER BY pattern in `tableProps` for CREATE TABLE.
	rePatternClusterBy  = regexp.MustCompile(`(?i)\bCLUSTER\s+BY\b`)
	reDataRetention     = regexp.MustCompile(`(?i)\bDATA_RETENTION_TIME_IN_DAYS\b`)
	reConstraintCol     = regexp.MustCompile(`(?i)^(?:CONSTRAINT|PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY)\b`)
	reVirtualColAS      = regexp.MustCompile(`(?i)\bAS\s*\([\s\S]*\)\s*$`)
	rePartitionBy       = regexp.MustCompile(`(?i)^PARTITION\s+BY\b`)

	reWithLocation      = regexp.MustCompile(`(?i)\bWITH\s+LOCATION\s*=`)
	reFileFormat        = regexp.MustCompile(`(?i)\bFILE_FORMAT\s*=`)

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

	// ── COPY INTO ────────────────────────────────────────────────────────────
	reIsCopyInto = regexp.MustCompile(`(?i)^\s*COPY\s+INTO\b`)
	reCopyInto   = regexp.MustCompile(`(?i)^\s*COPY\s+INTO\s+(` + _identPath + `|@\S+|'[^']+')(?:\s*\([^)]*\))?(?:\s+|$)`)
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

	// ── CREATE DYNAMIC TABLE ──────────────────────────────────────────────────
	reIsCreateDynTable = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?DYNAMIC\s+TABLE\b`)
	reDynHasTargetLag  = regexp.MustCompile(`(?i)\bTARGET_LAG\s*=`)
	reDynHasWarehouse  = regexp.MustCompile(`(?i)\bWAREHOUSE\s*=`)
	reDynHasAs         = regexp.MustCompile(`(?i)\bAS\s+(?:SELECT|WITH)\b`)

	// ── CREATE INTEGRATION ────────────────────────────────────────────────────
	reIsCreateIntegration = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:STORAGE|API|NOTIFICATION|SECURITY|EXTERNAL\s+ACCESS)\s+INTEGRATION\b`)
	reIntegrationName    = regexp.MustCompile(`(?i)INTEGRATION\s+(` + _identPath + `)`)
	reIntegrationType    = regexp.MustCompile(`(?i)\bTYPE\s*=\s*([a-zA-Z_0-9]+)`)
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
		`ERROR_INTEGRATION`, `COMMENT`, `AFTER`, `WHEN`,
	}, "|")

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

	// ── GRANT ─────────────────────────────────────────────────────────────────
	reIsGrantRole = regexp.MustCompile(`(?i)^\s*GRANT\s+ROLE\b`)

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

	// ── Parseable keywords ────────────────────────────────────────────────────
	parseableKWs = map[string]bool{
		"SELECT": true, "WITH": true, "INSERT": true, "UPDATE": true,
		"CREATE": true, "ALTER": true, "TRUNCATE": true, "CALL": true,
		"SHOW": true, "SET": true, "DROP": true, "UNDROP": true,
		"MERGE": true, "GRANT": true, "REVOKE": true, "COPY": true,
	}
)

// ── ValidateSnowflakePatterns ─────────────────────────────────────────────────

// ValidateSnowflakePatterns checks each statement in stmtRanges against a set
// of Snowflake-specific rules that cannot be expressed as generic SQL syntax
// errors.  It is a pure Go replacement for the validateWithParser function in
// sqlDiagnostics.ts; the node-sql-parser dependency is dropped because
// ValidateSyntax already covers generic syntax errors via its tokenizer.
func ValidateSnowflakePatterns(sql string, stmtRanges []StatementRange) []DiagMarker {
	var markers []DiagMarker

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
			// Tasks ARE schema objects, so they CAN have prefixes. No account-level check.
			// Validate properties up to the AS keyword
			asIdx := regexp.MustCompile(`(?i)\bAS\b`).FindStringIndex(parseText)
			if asIdx != nil {
				validateProperties(parseText[:asIdx[0]], taskProps, r, &markers)
			}
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

		// ── Preamble: CREATE STAGE ───────────────────────────────────────
		// stripParenContents removes nested KEY=VALUE pairs inside blocks
		// like FILE_FORMAT=(...), ENCRYPTION=(...), DIRECTORY=(...) before
		// property validation, preventing false positives like "Unexpected
		// property 'TYPE'" for FILE_FORMAT=(TYPE='CSV' ...).
		if reIsCreateStage.MatchString(parseText) {
			validateProperties(stripParenContents(parseText), stageProps, r, &markers)
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

		// ── GRANT ────────────────────────────────────────────────────────
		if reIsGrantRole.MatchString(parseText) {
			if regexp.MustCompile(`(?i)\bTO\s+TABLE\b`).MatchString(parseText) {
				markers = append(markers, diagMarkerSpan(r, "Unexpected syntax: Roles can be granted to other roles or users, but not directly to tables.", 4))
			}
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
	reProp := regexp.MustCompile(`(?i)\b([a-zA-Z_0-9]+)\s*=`)
	reValid := regexp.MustCompile(`(?i)^(` + validProps + `)$`)

	for _, m := range reProp.FindAllStringSubmatch(s, -1) {
		key := m[1]
		if !reValid.MatchString(key) {
			*markers = append(*markers, diagMarkerSpan(r, fmt.Sprintf("Unexpected property '%s' in statement.", key), 4))
		}
	}
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

func validateBoolProp(s string, prop string, r StatementRange, markers *[]DiagMarker) {
	re := regexp.MustCompile(`(?i)\b` + prop + `\s*=\s*(\w+)\b`)
	if m := re.FindStringSubmatch(s); m != nil {
		val := strings.ToUpper(m[1])
		if val != "TRUE" && val != "FALSE" {
			*markers = append(*markers, diagMarkerSpan(r, fmt.Sprintf("%s must be TRUE or FALSE.", prop), 4))
		}
	}
}

func extractParenContent(s string, key string) string {
	re := regexp.MustCompile(`(?i)\b` + key + `\s*=\s*\(`)
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
