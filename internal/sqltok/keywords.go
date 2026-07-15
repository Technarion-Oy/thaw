// SPDX-License-Identifier: GPL-3.0-or-later

package sqltok

import (
	"sort"
	"strings"
)

// reservedKeywords is the Snowflake reserved-keyword set from the official spec.
// An unquoted identifier matching one of these is always a keyword.
// Source: internal/snowflake/identifiers.go
var reservedKeywords = map[string]struct{}{
	"ACCOUNT": {}, "ALL": {}, "ALTER": {}, "AND": {}, "ANY": {}, "AS": {},
	"BETWEEN": {}, "BY": {},
	"CASE": {}, "CAST": {}, "CHECK": {}, "COLUMN": {}, "CONNECT": {},
	"CONNECTION": {}, "CONSTRAINT": {}, "CREATE": {}, "CROSS": {},
	"CURRENT": {}, "CURRENT_DATE": {}, "CURRENT_TIME": {},
	"CURRENT_TIMESTAMP": {}, "CURRENT_USER": {},
	"DATABASE": {}, "DELETE": {}, "DISTINCT": {}, "DROP": {},
	"ELSE": {}, "EXISTS": {},
	"FALSE": {}, "FOLLOWING": {}, "FOR": {}, "FROM": {}, "FULL": {},
	"GRANT": {}, "GROUP": {}, "GSCLUSTER": {},
	"HAVING": {},
	"ILIKE": {}, "IN": {}, "INCREMENT": {}, "INNER": {}, "INSERT": {},
	"INTERSECT": {}, "INTO": {}, "IS": {}, "ISSUE": {},
	"JOIN": {},
	"LATERAL": {}, "LEFT": {}, "LIKE": {}, "LIMIT": {},
	"LOCALTIME": {}, "LOCALTIMESTAMP": {},
	"MINUS": {},
	"NATURAL": {}, "NOT": {}, "NULL": {},
	"OF": {}, "ON": {}, "OR": {}, "ORDER": {}, "ORGANIZATION": {},
	"QUALIFY": {},
	"REGEXP": {}, "REVOKE": {}, "RIGHT": {}, "RLIKE": {}, "ROW": {}, "ROWS": {},
	"SAMPLE": {}, "SELECT": {}, "SET": {}, "SHOW": {}, "SOME": {}, "START": {},
	"TABLE": {}, "TABLESAMPLE": {}, "THEN": {}, "TO": {}, "TRIGGER": {},
	"TRUE": {}, "TRY_CAST": {},
	"UNION": {}, "UNIQUE": {}, "UPDATE": {}, "USING": {},
	"VALUES": {}, "VIEW": {},
	"WHEN": {}, "WHENEVER": {}, "WHERE": {}, "WITH": {},
}

// keywords is the combined set of all SQL keywords used by the tokenizer.
// Union of sqlAllKeywords + sqlFormatterKeywords from sqleditor plus
// reservedKeywords from snowflake/identifiers.go, deduplicated.
var keywords = map[string]struct{}{
	// From sqlAllKeywords
	"SELECT": {}, "FROM": {}, "WHERE": {}, "GROUP": {}, "BY": {}, "HAVING": {}, "ORDER": {}, "LIMIT": {},
	"JOIN": {}, "INNER": {}, "LEFT": {}, "RIGHT": {}, "FULL": {}, "OUTER": {}, "CROSS": {}, "ON": {}, "USING": {},
	"NATURAL": {}, "QUALIFY": {}, "PIVOT": {}, "UNPIVOT": {}, "RECURSIVE": {},
	"WITH": {}, "AS": {}, "DISTINCT": {}, "ALL": {}, "UNION": {}, "INTERSECT": {}, "EXCEPT": {}, "MINUS": {},
	"INSERT": {}, "INTO": {}, "VALUES": {}, "UPDATE": {}, "SET": {}, "DELETE": {}, "MERGE": {}, "MATCHED": {},
	"CREATE": {}, "OR": {}, "REPLACE": {}, "TABLE": {}, "VIEW": {}, "TEMPORARY": {}, "TEMP": {}, "TRANSIENT": {},
	"DATABASE": {}, "SCHEMA": {}, "FUNCTION": {}, "PROCEDURE": {}, "TASK": {}, "STREAM": {}, "PIPE": {},
	"DROP": {}, "TRUNCATE": {}, "ALTER": {}, "RENAME": {}, "ADD": {}, "COLUMN": {}, "MODIFY": {},
	"USE": {}, "WAREHOUSE": {}, "ROLE": {}, "USER": {}, "GRANT": {}, "REVOKE": {}, "TO": {},
	"AND": {}, "NOT": {}, "IN": {}, "IS": {}, "NULL": {}, "TRUE": {}, "FALSE": {}, "BETWEEN": {}, "LIKE": {}, "ILIKE": {},
	"CASE": {}, "WHEN": {}, "THEN": {}, "ELSE": {}, "END": {}, "CAST": {}, "TRY_CAST": {}, "TYPE": {},
	"ASC": {}, "DESC": {}, "NULLS": {}, "FIRST": {}, "LAST": {},
	"OVER": {}, "PARTITION": {}, "ROWS": {}, "RANGE": {},
	"UNBOUNDED": {}, "PRECEDING": {}, "FOLLOWING": {}, "CURRENT": {}, "ROW": {},
	"CLUSTER": {}, "OFFSET": {}, "FETCH": {}, "NEXT": {}, "ONLY": {},
	// Aggregate / analytic
	"SUM": {}, "COUNT": {}, "AVG": {}, "MIN": {}, "MAX": {}, "MEDIAN": {}, "VARIANCE": {}, "STDDEV": {},
	// Date/time
	"DATE_TRUNC": {}, "DATEADD": {}, "DATEDIFF": {}, "DATE_PART": {}, "CURRENT_DATE": {}, "CURRENT_TIMESTAMP": {},
	"TO_DATE": {}, "TO_VARCHAR": {}, "TO_NUMBER": {}, "TO_TIMESTAMP": {}, "TO_TIMESTAMP_NTZ": {}, "TO_TIMESTAMP_LTZ": {}, "TO_TIMESTAMP_TZ": {},
	"TO_JSON": {}, "TO_VARIANT": {},
	// Conditional / conversion
	"IFF": {}, "IFNULL": {}, "NVL": {}, "COALESCE": {}, "DECODE": {}, "ZEROIFNULL": {}, "NULLIF": {},
	// Semi-structured
	"LISTAGG": {}, "ARRAY_AGG": {}, "OBJECT_AGG": {}, "GET": {}, "FLATTEN": {}, "LATERAL": {},
	// Generator
	"SEQ4": {}, "SEQ8": {}, "RANDOM": {}, "UNIFORM": {}, "RANDSTR": {}, "GENERATOR": {}, "ROWCOUNT": {},
	// Scripting
	"BEGIN": {}, "DECLARE": {}, "LET": {}, "VAR": {}, "RETURN": {}, "RETURNS": {}, "EXCEPTION": {}, "RAISE": {},
	"LOOP": {}, "WHILE": {}, "REPEAT": {}, "UNTIL": {}, "IF": {},
	"EXIT": {}, "CONTINUE": {}, "OPEN": {}, "CLOSE": {}, "CALL": {}, "EXECUTE": {}, "IMMEDIATE": {},
	"LANGUAGE": {}, "SQL": {}, "PYTHON": {}, "JAVASCRIPT": {}, "SCALA": {}, "JAVA": {}, "SECURE": {}, "VOLATILE": {},
	"IMMUTABLE": {}, "STABLE": {}, "INTERNAL": {}, "EXTERNAL": {}, "STAGE": {}, "FILE": {}, "FORMAT": {},
	"STORAGE": {}, "INTEGRATION": {}, "SECRET": {}, "GIT": {}, "REPOSITORY": {}, "NOTEBOOK": {},
	// Cortex
	"SNOWFLAKE": {}, "CORTEX": {}, "MATCH_CONDITION": {},
	// Data types are NOT listed here — they are injected via
	// RegisterDataTypeKeywords from the snowflake package's authoritative
	// registry (internal/snowflake/datatypes.go) to avoid duplicating the list.

	// From sqlFormatterKeywords (additions not in sqlAllKeywords)
	"ANY": {}, "AT": {}, "BEFORE": {},
	"CASCADE": {}, "COMMENT": {}, "COMMIT": {}, "CONNECT": {}, "CONSTRAINT": {}, "COPY": {},
	"CURRENT_ROLE": {}, "CURRENT_SCHEMA": {}, "CURRENT_TIME": {}, "CURRENT_USER": {}, "CURRENT_WAREHOUSE": {},
	"DEFAULT": {}, "DESCRIBE": {},
	"EXPLAIN": {}, "EXTRACT": {},
	"FORCE": {}, "FOREIGN": {},
	"GROUPING": {},
	"INDEX": {},
	"KEY": {},
	"MATCH_RECOGNIZE": {}, "MEASURES": {},
	"OF": {},
	"OVERWRITE": {}, "PATTERN": {},
	"PRIMARY": {}, "PURGE": {},
	"REFERENCES": {}, "RESTRICT": {},
	"ROLLBACK": {},
	"SAMPLE": {}, "SEMI": {}, "SEQUENCE": {},
	"SOME": {},
	"TABLESAMPLE": {}, "TOP": {}, "TRANSACTION": {},
	"UNIQUE": {},
	"WINDOW": {}, "WITHIN": {}, "WITHOUT": {},
	// Snowflake-specific from formatter
	"ANTI": {}, "ASOF": {}, "CLONE": {}, "CRON": {},
	"DYNAMIC": {}, "ENABLE": {}, "FINALIZE": {},
	"ICEBERG": {}, "MASKING": {}, "NETWORK": {},
	"NOTIFY": {}, "POLICY": {}, "PROJECTION": {}, "RECOVER": {},
	"REPLICATION": {}, "RESUME": {}, "SCHEDULE": {},
	"SHARE": {}, "SUSPEND": {}, "TABULAR": {},
	"TRIGGER": {}, "UNDROP": {},
	"INPUT": {},

	// From snowflake reserved keywords (additions not above)
	"ACCOUNT": {}, "CHECK": {}, "CONNECTION": {},
	"EXISTS": {},
	"GSCLUSTER": {},
	"INCREMENT": {}, "ISSUE": {},
	"LOCALTIME": {}, "LOCALTIMESTAMP": {},
	"ORGANIZATION": {},
	"REGEXP": {}, "RLIKE": {},
	"START": {},
	"WHENEVER": {},
}

// builtinFunctions is the set of Snowflake built-in function names.
var builtinFunctions = map[string]struct{}{
	"ABS": {}, "ACOS": {}, "ACOSH": {}, "ADD_MONTHS": {},
	"ANY_VALUE": {}, "APPROX_COUNT_DISTINCT": {}, "APPROX_PERCENTILE": {},
	"APPROX_TOP_K": {}, "ARRAY_AGG": {}, "ARRAY_APPEND": {},
	"ARRAY_CAT": {}, "ARRAY_COMPACT": {}, "ARRAY_CONSTRUCT": {},
	"ARRAY_CONTAINS": {}, "ARRAY_DISTINCT": {}, "ARRAY_EXCEPT": {},
	"ARRAY_FLATTEN": {}, "ARRAY_GENERATE_RANGE": {}, "ARRAY_INSERT": {},
	"ARRAY_INTERSECTION": {}, "ARRAY_MAX": {}, "ARRAY_MIN": {},
	"ARRAY_PREPEND": {}, "ARRAY_REMOVE": {}, "ARRAY_REMOVE_AT": {},
	"ARRAY_SIZE": {}, "ARRAY_SLICE": {}, "ARRAY_SORT": {},
	"ARRAY_TO_STRING": {}, "ARRAY_UNION_AGG": {}, "ARRAY_UNIQUE_AGG": {},
	"AS_ARRAY": {}, "AS_BINARY": {}, "AS_BOOLEAN": {}, "AS_CHAR": {},
	"AS_DATE": {}, "AS_DECIMAL": {}, "AS_DOUBLE": {}, "AS_INTEGER": {},
	"AS_NUMBER": {}, "AS_OBJECT": {}, "AS_REAL": {}, "AS_TIME": {},
	"AS_TIMESTAMP_LTZ": {}, "AS_TIMESTAMP_NTZ": {}, "AS_TIMESTAMP_TZ": {},
	"AS_TINYINT": {}, "AS_VARCHAR": {}, "ASIN": {}, "ASINH": {},
	"ATAN": {}, "ATAN2": {}, "ATANH": {}, "AVG": {},
	"BASE64_DECODE_STRING": {}, "BASE64_ENCODE": {}, "BITNOT": {},
	"BITSHIFTLEFT": {}, "BITSHIFTRIGHT": {}, "BITAND": {},
	"BITOR": {}, "BITXOR": {}, "BOOLAND": {}, "BOOLAND_AGG": {},
	"BOOLNOT": {}, "BOOLOR": {}, "BOOLOR_AGG": {}, "BOOLXOR": {},
	"BOOLXOR_AGG": {},
	"CASE": {}, "CAST": {}, "CBRT": {}, "CEIL": {}, "CEILING": {},
	"CHARINDEX": {}, "CHR": {}, "CHAR": {}, "COALESCE": {},
	"COLLATE": {}, "COLLATION": {}, "COMPRESS": {}, "CONCAT": {},
	"CONCAT_WS": {}, "CONDITIONAL_CHANGE_EVENT": {},
	"CONDITIONAL_TRUE_EVENT": {}, "CONTAINS": {}, "CONVERT_TIMEZONE": {},
	"COS": {}, "COSH": {}, "COUNT": {}, "COUNT_IF": {},
	"COVAR_POP": {}, "COVAR_SAMP": {}, "CUME_DIST": {},
	"DATE_FROM_PARTS": {}, "DATE_PART": {}, "DATE_TRUNC": {},
	"DATEADD": {}, "DATEDIFF": {}, "DAYNAME": {}, "DAYOFMONTH": {},
	"DAYOFWEEK": {}, "DAYOFWEEKISO": {}, "DAYOFYEAR": {},
	"DECODE": {}, "DECOMPRESS": {}, "DENSE_RANK": {}, "DIV0": {},
	"DIV0NULL": {},
	"EDITDISTANCE": {}, "ENDSWITH": {}, "EQUAL_NULL": {}, "EXP": {},
	"FIRST_VALUE": {}, "FLATTEN": {}, "FLOOR": {}, "FORMAT_DATE": {},
	"FORMAT_NUMBER": {},
	"GENERATOR": {}, "GET": {}, "GET_ABSOLUTE_PATH": {}, "GET_DDL": {},
	"GET_PATH": {}, "GET_PRESIGNED_URL": {}, "GET_STAGE_LOCATION": {},
	"GETBIT": {}, "GREATEST": {}, "GROUPING": {}, "GROUPING_ID": {},
	"HASH": {}, "HASH_AGG": {}, "HAVERSINE": {},
	"HEX_DECODE_BINARY": {}, "HEX_DECODE_STRING": {}, "HEX_ENCODE": {},
	"HOUR": {}, "HOURS": {},
	"IFF": {}, "IFNULL": {}, "IN": {}, "INITCAP": {},
	"INSERT": {}, "IS_ARRAY": {}, "IS_BINARY": {}, "IS_BOOLEAN": {},
	"IS_CHAR": {}, "IS_DATE": {}, "IS_DATE_VALUE": {},
	"IS_DECIMAL": {}, "IS_DOUBLE": {}, "IS_GRANTED_TO_INVOKER_ROLE": {},
	"IS_INTEGER": {}, "IS_NULL_VALUE": {}, "IS_OBJECT": {},
	"IS_REAL": {}, "IS_TIME": {}, "IS_TIMESTAMP_LTZ": {},
	"IS_TIMESTAMP_NTZ": {}, "IS_TIMESTAMP_TZ": {}, "IS_VARCHAR": {},
	"JAROWINKLER_SIMILARITY": {}, "JSON_EXTRACT_PATH_TEXT": {},
	"KURTOSIS": {}, "LAG": {}, "LAST_DAY": {}, "LAST_VALUE": {},
	"LEAD": {}, "LEAST": {}, "LEFT": {}, "LENGTH": {}, "LEN": {},
	"LISTAGG": {}, "LN": {}, "LOG": {}, "LOWER": {}, "LPAD": {},
	"LTRIM": {},
	"MAX": {}, "MAX_BY": {}, "MEDIAN": {}, "MIN": {}, "MIN_BY": {},
	"MINUTE": {}, "MINUTES": {}, "MOD": {}, "MODE": {},
	"MONTH": {}, "MONTHNAME": {}, "MONTHS_BETWEEN": {},
	"NORMAL": {}, "NTH_VALUE": {}, "NTILE": {}, "NULLIF": {},
	"NULLIFZERO": {}, "NVL": {}, "NVL2": {},
	"OBJECT_AGG": {}, "OBJECT_CONSTRUCT": {},
	"OBJECT_CONSTRUCT_KEEP_NULL": {}, "OBJECT_DELETE": {},
	"OBJECT_INSERT": {}, "OBJECT_KEYS": {}, "OBJECT_PICK": {},
	"PARSE_IP": {}, "PARSE_JSON": {}, "PARSE_URL": {},
	"PARSE_XML": {}, "PERCENT_RANK": {}, "PERCENTILE_CONT": {},
	"PERCENTILE_DISC": {}, "PI": {}, "POSITION": {}, "POW": {},
	"POWER": {},
	"RANDSTR": {}, "RANDOM": {}, "RANK": {}, "RATIO_TO_REPORT": {},
	"REGEXP": {}, "REGEXP_COUNT": {}, "REGEXP_EXTRACT": {},
	"REGEXP_EXTRACT_ALL": {}, "REGEXP_INSTR": {}, "REGEXP_LIKE": {},
	"REGEXP_REPLACE": {}, "REGEXP_SUBSTR": {}, "REPEAT": {},
	"REPLACE": {}, "REVERSE": {}, "RIGHT": {}, "ROUND": {},
	"ROW_NUMBER": {}, "RPAD": {}, "RTRIM": {},
	"SECOND": {}, "SECONDS": {}, "SHA1": {}, "SHA1_BINARY": {},
	"SHA1_HEX": {}, "SHA2": {}, "SHA2_BINARY": {}, "SHA2_HEX": {},
	"SIGN": {}, "SIN": {}, "SINH": {}, "SKEW": {},
	"SOUNDEX": {}, "SPACE": {}, "SPLIT": {}, "SPLIT_PART": {},
	"SPLIT_TO_TABLE": {}, "SQL_VARIANT_PROPERTY": {}, "SQRT": {},
	"SQUARE": {}, "STARTSWITH": {}, "STDDEV": {}, "STDDEV_POP": {},
	"STDDEV_SAMP": {}, "STRIP_NULL_VALUE": {}, "STRTOK": {},
	"STRTOK_SPLIT_TO_TABLE": {}, "STRTOK_TO_ARRAY": {},
	"SUBSTR": {}, "SUBSTRING": {}, "SUM": {},
	"SYSTEM$ABORT_TRANSACTION": {}, "SYSTEM$CANCEL_ALL_QUERIES": {},
	"SYSTEM$CANCEL_QUERY": {}, "SYSTEM$CLUSTERING_DEPTH": {},
	"SYSTEM$CLUSTERING_INFORMATION": {},
	"SYSTEM$GET_PREDECESSOR_RETURN_VALUE": {},
	"SYSTEM$STREAM_GET_TABLE_TIMESTAMP": {}, "SYSTEM$STREAM_HAS_DATA": {},
	"SYSTEM$TASK_DEPENDENTS_ENABLE": {}, "SYSTEM$TYPEOF": {},
	"SYSTEM$WAIT": {},
	"TAN": {}, "TANH": {}, "TIME_FROM_PARTS": {}, "TIMEADD": {},
	"TIMEDIFF": {}, "TIMESTAMPADD": {}, "TIMESTAMPDIFF": {},
	"TIMESTAMP_FROM_PARTS": {}, "TIMESTAMP_LTZ_FROM_PARTS": {},
	"TIMESTAMP_NTZ_FROM_PARTS": {}, "TIMESTAMP_TZ_FROM_PARTS": {},
	"TO_ARRAY": {}, "TO_BINARY": {}, "TO_BOOLEAN": {},
	"TO_CHAR": {}, "TO_DATE": {}, "TO_DECIMAL": {}, "TO_DOUBLE": {},
	"TO_GEOGRAPHY": {}, "TO_GEOMETRY": {}, "TO_JSON": {},
	"TO_NUMBER": {}, "TO_OBJECT": {}, "TO_REAL": {},
	"TO_TIME": {}, "TO_TIMESTAMP": {}, "TO_TIMESTAMP_LTZ": {},
	"TO_TIMESTAMP_NTZ": {}, "TO_TIMESTAMP_TZ": {}, "TO_VARIANT": {},
	"TO_VARCHAR": {}, "TO_XML": {}, "TRANSLATE": {}, "TRIM": {},
	"TRUNCATE": {}, "TRUNC": {}, "TRY_BASE64_DECODE_BINARY": {},
	"TRY_BASE64_DECODE_STRING": {}, "TRY_CAST": {},
	"TRY_HEX_DECODE_BINARY": {}, "TRY_HEX_DECODE_STRING": {},
	"TRY_PARSE_JSON": {}, "TRY_TO_BINARY": {}, "TRY_TO_BOOLEAN": {},
	"TRY_TO_DATE": {}, "TRY_TO_DECIMAL": {}, "TRY_TO_DOUBLE": {},
	"TRY_TO_NUMBER": {}, "TRY_TO_TIME": {}, "TRY_TO_TIMESTAMP": {},
	"TRY_TO_TIMESTAMP_LTZ": {}, "TRY_TO_TIMESTAMP_NTZ": {},
	"TRY_TO_TIMESTAMP_TZ": {}, "TYPEOF": {},
	"UNIFORM": {}, "UPPER": {}, "UNISTR": {},
	"VAR_POP": {}, "VAR_SAMP": {}, "VARIANCE": {},
	"VARIANCE_POP": {}, "VARIANCE_SAMP": {},
	"WEEK": {}, "WEEKISO": {}, "WEEKOFYEAR": {},
	"XMLGET": {}, "YEAR": {}, "YEAROFWEEK": {},
	"YEAROFWEEKISO": {}, "ZEROIFNULL": {},
}

// ReservedKeywordList returns the Snowflake reserved keywords as a sorted slice.
func ReservedKeywordList() []string {
	out := make([]string, 0, len(reservedKeywords))
	for kw := range reservedKeywords {
		out = append(out, kw)
	}
	sort.Strings(out)
	return out
}

// IsReserved reports whether upper (an already-uppercased word) is a
// Snowflake reserved keyword that must be quoted when used as an identifier.
func IsReserved(upper string) bool {
	_, ok := reservedKeywords[upper]
	return ok
}

// RegisterDataTypeKeywords adds the given (canonical, upper-case) Snowflake
// data-type names to the tokenizer's keyword set.  It exists so the data-type
// list lives in exactly one place — the snowflake package's authoritative
// registry — without sqltok (a leaf package) importing snowflake and creating
// an import cycle.  The snowflake package calls this once from its init, so by
// the time any snowflake-importing consumer tokenizes SQL the types are present.
// Not safe for concurrent use; intended to be called only during package init.
func RegisterDataTypeKeywords(names []string) {
	for _, n := range names {
		keywords[strings.ToUpper(n)] = struct{}{}
	}
}

// IsKeyword reports whether upper is any SQL keyword recognized by the tokenizer.
func IsKeyword(upper string) bool {
	_, ok := keywords[upper]
	return ok
}

// IsBuiltinFunction reports whether upper is a Snowflake built-in function name.
func IsBuiltinFunction(upper string) bool {
	_, ok := builtinFunctions[upper]
	return ok
}
