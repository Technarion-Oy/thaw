package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_CreateProcedure(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectWarning bool
		expectedMatch string
	}{
		// ── Valid Cases ──────────────────────────────────────────────────────
		{
			name:          "Valid Javascript procedure",
			sql:           "CREATE OR REPLACE PROCEDURE my_proc(param1 VARCHAR) RETURNS VARCHAR LANGUAGE JAVASCRIPT AS $$ return 'hello'; $$",
			expectWarning: false,
		},
		{
			name:          "Valid SQL procedure",
			sql:           "CREATE PROCEDURE my_proc() RETURNS NUMBER LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid Python procedure",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid Python procedure with IMPORTS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' IMPORTS = ('@stage/file.py') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid with EXECUTE AS OWNER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS OWNER AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with STRICT",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER) RETURNS NUMBER LANGUAGE SQL STRICT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with CALLED ON NULL INPUT",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER) RETURNS NUMBER LANGUAGE SQL CALLED ON NULL INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid table-valued procedure",
			sql:           "CREATE PROCEDURE get_data() RETURNS TABLE(name VARCHAR, age INT) LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT name, age FROM t); END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid procedure with EXECUTE AS in body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS $$ var s = 'EXECUTE AS INVOKER'; $$",
			expectWarning: false,
		},
		{
			name:          "Valid Java procedure",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA RUNTIME_VERSION = '11' HANDLER = 'MyClass.handler' AS $$ class MyClass { public static String handler() { return \"hello\"; } } $$",
			expectWarning: false,
		},
		{
			name:          "Valid Scala procedure",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SCALA RUNTIME_VERSION = '2.12' HANDLER = 'MyObject.handler' AS $$ object MyObject { def handler(): String = \"hello\" } $$",
			expectWarning: false,
		},
		{
			name:          "Valid with EXECUTE AS CALLER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS CALLER AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid Python with both PACKAGES and IMPORTS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') IMPORTS = ('@stage/file.py') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid with RETURNS NULL ON NULL INPUT",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER) RETURNS NUMBER LANGUAGE SQL RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with schema-qualified name",
			sql:           "CREATE PROCEDURE mydb.myschema.my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with multiple parameters",
			sql:           "CREATE PROCEDURE my_proc(a INT, b VARCHAR, c FLOAT, d BOOLEAN) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid multiline procedure",
			sql:           "CREATE OR REPLACE PROCEDURE my_proc()\n  RETURNS VARCHAR\n  LANGUAGE SQL\n  EXECUTE AS OWNER\nAS\n$$\nBEGIN\n  RETURN 'ok';\nEND;\n$$",
			expectWarning: false,
		},
		{
			name:          "Valid OR REPLACE procedure",
			sql:           "CREATE OR REPLACE PROCEDURE my_proc() RETURNS NUMBER LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid lowercase keywords",
			sql:           "create or replace procedure my_proc() returns varchar language sql as $$ begin return 'ok'; end; $$",
			expectWarning: false,
		},
		{
			name:          "Valid mixed case keywords",
			sql:           "Create Or Replace Procedure my_proc() Returns Varchar Language Sql As $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid mixed case language value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE python RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		{
			name:          "Valid extra whitespace between keywords",
			sql:           "CREATE   OR   REPLACE   PROCEDURE   my_proc()   RETURNS   VARCHAR   LANGUAGE   SQL   AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid leading whitespace",
			sql:           "   CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid parameterized types",
			sql:           "CREATE PROCEDURE my_proc(a VARCHAR(100), b NUMBER(10,2)) RETURNS VARCHAR(255) LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with DEFAULT parameter values",
			sql:           "CREATE PROCEDURE my_proc(a VARCHAR DEFAULT 'hello', b NUMBER DEFAULT 42) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid with COMMENT clause",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'This is a test procedure' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid Java without RUNTIME_VERSION",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA HANDLER = 'MyClass.handler' AS $$ class MyClass { public static String handler() { return \"hello\"; } } $$",
			expectWarning: false,
		},
		{
			name:          "Valid Scala without RUNTIME_VERSION",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SCALA HANDLER = 'MyObj.handler' AS $$ object MyObj { def handler(): String = \"hello\" } $$",
			expectWarning: false,
		},
		{
			name:          "Valid with empty body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ $$",
			expectWarning: false,
		},
		{
			name:          "Valid no parameters",
			sql:           "CREATE PROCEDURE my_proc() RETURNS NUMBER LANGUAGE JAVASCRIPT AS $$ return 1; $$",
			expectWarning: false,
		},

		// ── Invalid Cases ────────────────────────────────────────────────────
		{
			name:          "Missing RETURNS",
			sql:           "CREATE PROCEDURE my_proc() LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},
		{
			name:          "Missing LANGUAGE",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		{
			name:          "Invalid LANGUAGE",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE RUBY AS $$ puts 'hello' $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		{
			name:          "Missing AS body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		{
			name:          "Conflict CALLED ON NULL and RETURNS NULL ON NULL",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT RETURNS NULL ON NULL INPUT AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		{
			name:          "Conflict CALLED ON NULL and STRICT",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT STRICT AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		{
			name:          "Duplicate STRICT and RETURNS NULL ON NULL",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL STRICT RETURNS NULL ON NULL INPUT AS $$ $$",
			expectWarning: true,
			expectedMatch: "redundant",
		},
		{
			name:          "Invalid EXECUTE AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS USER AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		{
			name:          "Python missing RUNTIME_VERSION",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "RUNTIME_VERSION is required",
		},
		{
			name:          "Python missing PACKAGES and IMPORTS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "PACKAGES or IMPORTS is required",
		},
		{
			name:          "Invalid parameter type",
			sql:           "CREATE PROCEDURE my_proc(param1 UNKNOWNTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'hello'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
		{
			name:          "Invalid return type",
			sql:           "CREATE PROCEDURE my_proc() RETURNS BADTYPE LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
		{
			name:          "Invalid EXECUTE AS INVOKER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS INVOKER AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		{
			name:          "Missing RETURNS and LANGUAGE",
			sql:           "CREATE PROCEDURE my_proc() AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},
		{
			name:          "Invalid LANGUAGE GOLANG",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE GOLANG AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		{
			name:          "Python missing RUNTIME_VERSION with IMPORTS only",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON IMPORTS = ('@stage/file.py') AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "RUNTIME_VERSION is required",
		},
		{
			name:          "Missing LANGUAGE with lowercase keywords",
			sql:           "create procedure my_proc() returns varchar as $$ begin return 'ok'; end; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		{
			name:          "Missing RETURNS with lowercase keywords",
			sql:           "create procedure my_proc() language sql as $$ begin return 1; end; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},
		{
			name:          "Missing AS body with lowercase keywords",
			sql:           "create procedure my_proc() returns varchar language sql",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		{
			name:          "Invalid LANGUAGE lowercase",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE rust AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		{
			name:          "Invalid EXECUTE AS ROLE",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS ROLE AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		{
			name:          "Invalid parameter type with parameterized valid return type",
			sql:           "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR(100) LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
		{
			name:          "Python missing PACKAGES and IMPORTS with HANDLER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' HANDLER = 'main' AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "PACKAGES or IMPORTS is required",
		},
		{
			name:          "Python missing everything",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "RUNTIME_VERSION is required",
		},
		{
			name:          "Multiple invalid parameter types",
			sql:           "CREATE PROCEDURE my_proc(a FAKETYPE, b ALSOFAKE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},

		// ── Edge Cases ──────────────────────────────────────────────────────

		// EXECUTE AS skipping: the AS-body finder must skip "EXECUTE AS" and
		// still report a missing body AS when no real AS follows.
		{
			name:          "EXECUTE AS CALLER without body AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS CALLER",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		{
			name:          "EXECUTE AS OWNER without body AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS OWNER",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		// Single-quoted body is valid Snowflake syntax.
		{
			name:          "Valid single-quoted body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS 'return 1;'",
			expectWarning: false,
		},
		// Quoted procedure name with special characters.
		{
			name:          "Valid quoted procedure name",
			sql:           `CREATE PROCEDURE "my proc"() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},
		// Quoted procedure name that is a SQL keyword.
		{
			name:          "Valid quoted SQL keyword as procedure name",
			sql:           `CREATE PROCEDURE "SELECT"() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},
		{
			name:          "Valid quoted dot-separated procedure name",
			sql:           `CREATE PROCEDURE "MY_DB"."MY SCHEMA"."my_proc"() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},
		// COMMENT clause containing "AS" keyword should not be picked up as the body AS.
		{
			name:          "Valid COMMENT containing AS keyword",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'known as foo' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// All three null-input handling clauses together: mutually exclusive + redundant.
		{
			name:          "All three null-input clauses",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT STRICT RETURNS NULL ON NULL INPUT AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		// Tab and newline characters between keywords.
		{
			name:          "Valid with tab between keywords",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR\tLANGUAGE\tSQL\tAS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Python with HANDLER, RUNTIME_VERSION, and PACKAGES (all present, valid).
		{
			name:          "Valid Python with HANDLER and PACKAGES",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') HANDLER = 'main' AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		// Python with HANDLER, RUNTIME_VERSION, and IMPORTS (all present, valid).
		{
			name:          "Valid Python with HANDLER and IMPORTS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' IMPORTS = ('@stage/file.py') HANDLER = 'main' AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		// Invalid EXECUTE AS with lowercase.
		{
			name:          "Invalid EXECUTE AS lowercase admin",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL execute as admin AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		// Empty parameter list with whitespace is valid.
		{
			name:          "Valid empty params with whitespace",
			sql:           "CREATE PROCEDURE my_proc(   ) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Newlines inside parameter list.
		{
			name:          "Valid newlines in parameter list",
			sql:           "CREATE PROCEDURE my_proc(\n  a INT,\n  b VARCHAR,\n  c FLOAT\n) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Trailing semicolon after dollar-quoted body.
		{
			name:          "Valid trailing semicolon after dollar-quoted body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;",
			expectWarning: false,
		},
		// EXECUTE AS with lowercase valid values.
		{
			name:          "Valid EXECUTE AS caller lowercase",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL execute as caller AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid EXECUTE AS owner lowercase",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL execute as owner AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Tagged dollar-quoting for procedure body.
		{
			name:          "Valid tagged dollar-quoting body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $proc$ BEGIN RETURN 'ok'; END; $proc$",
			expectWarning: false,
		},
		// Keywords in body must not trigger preamble warnings.
		{
			name:          "Body keywords don't trigger preamble warnings",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ LANGUAGE PYTHON; EXECUTE AS INVOKER; STRICT; CALLED ON NULL INPUT; $$",
			expectWarning: false,
		},
		// Body containing nested CREATE PROCEDURE must not trigger nested validation.
		{
			name:          "Body containing CREATE PROCEDURE does not trigger nested validation",
			sql:           "CREATE PROCEDURE outer_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ CREATE PROCEDURE inner() RETURNS INT LANGUAGE PYTHON; $$",
			expectWarning: false,
		},
		// Body containing RETURNS and LANGUAGE string values must not confuse preamble checks.
		{
			name:          "Body with RETURNS and LANGUAGE in string literal does not trigger warnings",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ DECLARE x VARCHAR; BEGIN x := 'LANGUAGE PYTHON RETURNS INT'; RETURN x; END; $$",
			expectWarning: false,
		},
		// Java with RUNTIME_VERSION: Python-specific checks must not fire.
		{
			name:          "Valid Java with RUNTIME_VERSION no Python warnings",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA RUNTIME_VERSION = '11' HANDLER = 'MyClass.handler' AS $$ class MyClass {} $$",
			expectWarning: false,
		},
		// Scala with RUNTIME_VERSION: Python-specific checks must not fire.
		{
			name:          "Valid Scala with RUNTIME_VERSION no Python warnings",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SCALA RUNTIME_VERSION = '2.12' HANDLER = 'MyObj.handler' AS $$ object MyObj {} $$",
			expectWarning: false,
		},
		// Block comment between keywords in preamble.
		{
			name:          "Valid with block comment in preamble",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR /* a comment */ LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Line comment between keywords in preamble.
		{
			name:          "Valid with line comment in preamble",
			sql:           "CREATE PROCEDURE my_proc()\n-- a comment\nRETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Missing everything: bare CREATE PROCEDURE stub.
		{
			name:          "Bare minimum procedure missing everything",
			sql:           "CREATE PROCEDURE my_proc()",
			expectWarning: true,
			expectedMatch: "Missing mandatory",
		},
		// Trailing semicolon without AS body is a common typo.
		{
			name:          "Trailing semicolon without AS body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL;",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		// JavaScript with RUNTIME_VERSION: Python-specific PACKAGES/IMPORTS check must not fire.
		{
			name:          "Valid JavaScript with RUNTIME_VERSION no Python warnings",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT RUNTIME_VERSION = '1.0' AS $$ return 1; $$",
			expectWarning: false,
		},
		// RETURNS TABLE must not trigger "Unknown data type TABLE" while invalid param types are still caught.
		{
			name:          "RETURNS TABLE with invalid parameter type",
			sql:           "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS TABLE(name VARCHAR) LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
		// RETURNS TABLE with empty column list.
		{
			name:          "Valid RETURNS TABLE with empty column list",
			sql:           "CREATE PROCEDURE my_proc() RETURNS TABLE() LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: false,
		},
		// Near-miss LANGUAGE typo.
		{
			name:          "Invalid LANGUAGE typo JAVASCRIP",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIP AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		// Single-quoted body with escaped (doubled) single quotes.
		{
			name:          "Valid single-quoted body with escaped quotes",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS 'return ''hello'';'",
			expectWarning: false,
		},
		// Single-quoted empty body.
		{
			name:          "Valid single-quoted empty body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS ''",
			expectWarning: false,
		},
		// RETURNS NULL ON NULL INPUT must not produce "Unknown data type NULL" from ValidateDataTypes.
		{
			name:          "Valid RETURNS NULL ON NULL INPUT does not trigger data type warning",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER) RETURNS VARCHAR LANGUAGE SQL RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		// Procedure without opening parenthesis (malformed name): preamble validation still runs.
		{
			name:          "Missing parentheses in parameter list",
			sql:           "CREATE PROCEDURE my_proc RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Duplicate EXECUTE AS clauses: first valid match accepted, no warning.
		{
			name:          "Valid duplicate EXECUTE AS CALLER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS CALLER EXECUTE AS OWNER AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Procedure name containing "AS" at non-word-boundary (underscore-delimited).
		{
			name:          "Valid procedure name containing AS with underscores",
			sql:           "CREATE PROCEDURE has_as_in_name() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Invalid return type with valid parameter types: return type independently validated.
		{
			name:          "Invalid return type with valid parameter types",
			sql:           "CREATE PROCEDURE my_proc(a INT, b VARCHAR) RETURNS BADRETURNTYPE LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
		// DEFAULT parameter with invalid type: type checking fires regardless of DEFAULT.
		{
			name:          "Invalid type with DEFAULT value",
			sql:           "CREATE PROCEDURE my_proc(a BADTYPE DEFAULT 42) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
		// LANGUAGE with a realistic near-miss numeric suffix.
		{
			name:          "Invalid LANGUAGE PYTHON3",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON3 AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		// LANGUAGE followed immediately by AS: the AS is consumed by body-detection,
		// leaving the LANGUAGE regex unmatched in the preamble.
		{
			name:          "LANGUAGE with no value followed by AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		// Reversed order of null-input clauses: order must not affect detection.
		{
			name:          "Conflict STRICT then CALLED ON NULL INPUT (reversed order)",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL STRICT CALLED ON NULL INPUT AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		// Parameter name containing "as" prefix: word boundary prevents false AS match.
		{
			name:          "Valid parameter name starting with as",
			sql:           "CREATE PROCEDURE my_proc(as_flag BOOLEAN) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// RETURNS TABLE with invalid column type: column types inside TABLE()
		// are not validated by ValidateDataTypes — only procedure parameter types are.
		{
			name:          "RETURNS TABLE with invalid column type is not flagged",
			sql:           "CREATE PROCEDURE my_proc() RETURNS TABLE(name BADTYPE) LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: false,
		},
		// RETURNS NULL ON NULL INPUT without a separate RETURNS <type> clause:
		// the \bRETURNS\b check matches the RETURNS in "RETURNS NULL ON NULL INPUT",
		// so the missing return-type is not caught (known limitation).
		{
			name:          "Known limitation: RETURNS NULL ON NULL INPUT masks missing RETURNS type",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER) LANGUAGE SQL RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false, // false negative: validator sees RETURNS keyword
		},
		// COMMENT clause placed before RETURNS containing the word AS:
		// premature AS detection may truncate the preamble before RETURNS.
		{
			name:          "COMMENT containing AS no longer truncates preamble",
			sql:           "CREATE PROCEDURE my_proc() COMMENT = 'known as foo' RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// Parameterized return type: ValidateDataTypes must validate the base
		// type name and ignore the precision/scale parenthetical.
		{
			name:          "Valid parameterized return type VARCHAR(255)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR(255) LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid parameterized return type NUMBER(10,2)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS NUMBER(10,2) LANGUAGE SQL AS $$ BEGIN RETURN 3.14; END; $$",
			expectWarning: false,
		},
		{
			name:          "Invalid parameterized return type BADTYPE(10)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS BADTYPE(10) LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},

		// LANGUAGE with a purely numeric value: the regex captures [a-zA-Z0-9_]+
		// so "123" is captured but is not a valid Snowflake language.
		{
			name:          "Invalid LANGUAGE numeric value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE 123 AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},

		// Combining valid EXECUTE AS with valid null-input handling:
		// independent clauses should not interfere with each other.
		{
			name:          "Valid EXECUTE AS CALLER with STRICT",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL EXECUTE AS CALLER STRICT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid EXECUTE AS OWNER with CALLED ON NULL INPUT",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL EXECUTE AS OWNER CALLED ON NULL INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},

		// Quoted procedure name that matches a validation keyword:
		// the name must not confuse preamble checks.
		{
			name:          "Valid quoted procedure name LANGUAGE",
			sql:           `CREATE PROCEDURE "LANGUAGE"() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},
		{
			name:          "Valid quoted procedure name RETURNS",
			sql:           `CREATE PROCEDURE "RETURNS"() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},

		// RETURNS keyword only inside a line comment: the validator does not
		// strip comments from parseText, so RETURNS inside a comment satisfies
		// the \bRETURNS\b check (known false negative).
		{
			name:          "RETURNS in line comment correctly detected as missing",
			sql:           "CREATE PROCEDURE my_proc()\n-- RETURNS VARCHAR\nLANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},
		// RETURNS keyword only inside a block comment: same false negative.
		{
			name:          "RETURNS in block comment correctly detected as missing",
			sql:           "CREATE PROCEDURE my_proc() /* RETURNS VARCHAR */ LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},
		// LANGUAGE keyword only inside a block comment: the regex
		// matches LANGUAGE inside the comment (known false negative).
		{
			name:          "LANGUAGE in block comment correctly detected as missing",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR /* LANGUAGE SQL */ AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},

		// Mixed valid and invalid parameter types in a multi-param list:
		// the invalid type is caught regardless of surrounding valid types.
		{
			name:          "Invalid type between valid types in parameter list",
			sql:           "CREATE PROCEDURE my_proc(a INT, b BADTYPE, c VARCHAR) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},

		// Java/Scala without HANDLER: procedure validator does not require
		// HANDLER (unlike the function validator which does).
		{
			name:          "Valid Java procedure without HANDLER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA AS $$ class MyClass {} $$",
			expectWarning: false,
		},
		{
			name:          "Valid Scala procedure without HANDLER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SCALA AS $$ object MyObj {} $$",
			expectWarning: false,
		},
		// EXECUTE AS where EXECUTE and AS are separated by a newline:
		// the \s+ in the EXECUTE AS skip regex must match across lines.
		{
			name:          "Valid EXECUTE AS with newline between EXECUTE and AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE\nAS CALLER AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Known limitation: COMMENT containing LANGUAGE keyword satisfies
		// the \bLANGUAGE\s+...\b regex check (analogous to RETURNS-in-comment).
		// Uses SQL to avoid Python-specific secondary warnings.
		{
			name:          "COMMENT containing LANGUAGE correctly detected as missing",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR COMMENT = 'LANGUAGE SQL' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		// First EXECUTE AS is invalid, second is valid: FindStringSubmatch returns
		// only the first match, so the invalid value is caught.
		{
			name:          "Invalid first EXECUTE AS with valid second produces warning",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS ADMIN EXECUTE AS CALLER AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		// Bare CREATE PROCEDURE with no name or parameter list:
		// reIsCreateProcedure still matches and preamble validation runs.
		{
			name:          "Bare CREATE PROCEDURE with no name",
			sql:           "CREATE PROCEDURE",
			expectWarning: true,
			expectedMatch: "Missing mandatory",
		},

		// ── IF NOT EXISTS syntax ─────────────────────────────────────────

		// IF NOT EXISTS is valid Snowflake syntax for procedures.
		{
			name:          "Valid with IF NOT EXISTS",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid OR REPLACE with IF NOT EXISTS and schema-qualified name",
			sql:           "CREATE OR REPLACE PROCEDURE IF NOT EXISTS mydb.myschema.my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "IF NOT EXISTS with missing LANGUAGE still caught",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		// IF NOT EXISTS prevents reCreateProcExt from matching (regex expects identPath
		// immediately after PROCEDURE), so parameter data types are NOT validated.
		{
			name:          "Known limitation: IF NOT EXISTS skips parameter data type validation",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: reCreateProcExt doesn't match IF NOT EXISTS
		},

		// ── Snowflake-specific data types ────────────────────────────────

		{
			name:          "Valid VARIANT parameter type",
			sql:           "CREATE PROCEDURE my_proc(v VARIANT) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid OBJECT parameter type",
			sql:           "CREATE PROCEDURE my_proc(o OBJECT) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid ARRAY return type",
			sql:           "CREATE PROCEDURE my_proc() RETURNS ARRAY LANGUAGE SQL AS $$ BEGIN RETURN ARRAY_CONSTRUCT(); END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid TIMESTAMP_LTZ parameter type",
			sql:           "CREATE PROCEDURE my_proc(ts TIMESTAMP_LTZ) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── AS body tokenization edge cases ──────────────────────────────

		// AS immediately followed by $$ (no space): still valid.
		{
			name:          "Valid AS immediately followed by dollar-quote (no space)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS$$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// EXECUTE AS with quoted identifier: regex [a-zA-Z0-9_]+ won't
		// match, so no EXECUTE AS warning fires (known false negative).
		{
			name:          "EXECUTE AS with quoted identifier correctly flagged",
			sql:           `CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS "ADMIN" AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		// COPY GRANTS clause (valid Snowflake syntax for OR REPLACE procedures).
		{
			name:          "Valid OR REPLACE with COPY GRANTS",
			sql:           "CREATE OR REPLACE PROCEDURE my_proc() COPY GRANTS RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Title-case EXECUTE AS values: case-insensitive comparison must match.
		{
			name:          "Valid EXECUTE AS Caller title case",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS Caller AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid EXECUTE AS Owner title case",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS Owner AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// EXECUTE AS at EOF with no value token: the EXECUTE AS skip logic
		// correctly skips the AS, leaving asBodyIdx = -1. No invalid
		// EXECUTE AS value warning because the value regex doesn't match.
		{
			name:          "EXECUTE AS at EOF with no value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		// Python with lowercase clause keywords: (?i) flag must match.
		{
			name:          "Valid Python with lowercase runtime_version and packages",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON runtime_version = '3.8' packages = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		// LANGUAGE keyword at EOF with no value and no AS clause.
		{
			name:          "LANGUAGE at EOF with no value and no AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE",
			expectWarning: true,
			expectedMatch: "Missing mandatory",
		},
		// Space between procedure name and opening parenthesis.
		{
			name:          "Valid procedure with space before parenthesis",
			sql:           "CREATE PROCEDURE my_proc () RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Additional edge cases ─────────────────────────────────────────

		// AS immediately followed by single-quote body (no space between AS and quote).
		{
			name:          "Valid AS immediately followed by single quote (no space)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS'return 1;'",
			expectWarning: false,
		},
		// CRLF line endings: \r\n must not break regex matching.
		{
			name:          "Valid with CRLF line endings",
			sql:           "CREATE PROCEDURE my_proc()\r\nRETURNS VARCHAR\r\nLANGUAGE SQL\r\nAS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Known limitation: procedure name exactly matching keyword "strict"
		// triggers false positive when CALLED ON NULL INPUT is present,
		// because \bSTRICT\b matches the procedure name in the preamble.
		{
			name:          "Known limitation: procedure named strict with CALLED ON NULL INPUT",
			sql:           "CREATE PROCEDURE strict() RETURNS INT LANGUAGE SQL CALLED ON NULL INPUT AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		// Known limitation: quoted procedure name containing "AS" at word boundary
		// truncates the preamble early, causing false missing-clause warnings.
		{
			name:          "Quoted name containing AS no longer truncates preamble",
			sql:           `CREATE PROCEDURE "MY AS PROC"() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},
		// CALLED ON NULL INPUT with extra spaces between keywords:
		// \s+ in the regex handles multiple spaces.
		{
			name:          "Valid CALLED ON NULL INPUT with extra spaces",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL CALLED   ON   NULL   INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		// RETURNS NULL ON NULL INPUT with extra spaces between keywords.
		{
			name:          "Valid RETURNS NULL ON NULL INPUT with extra spaces",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL RETURNS   NULL   ON   NULL   INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		// CALLED ON NULL INPUT spanning newlines: \s+ matches across lines.
		{
			name:          "Valid CALLED ON NULL INPUT spanning newlines",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL CALLED\nON\nNULL\nINPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		// EXECUTE AS spanning a newline between EXECUTE and AS.
		// The skip regex uses \s+ which includes newlines, and
		// the EXECUTE AS value regex also uses \s+.
		{
			name:          "Valid EXECUTE AS spanning newline with OWNER value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE\nAS\nOWNER AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Python procedure with IF NOT EXISTS: Python-specific checks
		// (RUNTIME_VERSION, PACKAGES/IMPORTS) still fire on the preamble.
		{
			name:          "IF NOT EXISTS Python missing RUNTIME_VERSION",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "RUNTIME_VERSION is required",
		},
		// Duplicate LANGUAGE clauses: FindStringSubmatch returns the first match,
		// so the first (invalid) value is checked.
		{
			name:          "Duplicate LANGUAGE first invalid second valid",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE RUBY LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		// LANGUAGE followed by line comment then value on next line:
		// \s+ cannot span across the comment text, so the regex fails to capture the value.
		{
			name:          "LANGUAGE value after line comment correctly parsed",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE\n-- a comment\nSQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// RETURNS TABLE with multiple typed columns: TABLE() column types
		// are not validated, only procedure parameter types are.
		{
			name:          "Valid RETURNS TABLE with multiple typed columns",
			sql:           "CREATE PROCEDURE my_proc() RETURNS TABLE(id INT, name VARCHAR, amount NUMBER(10,2)) LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT 1, 'a', 1.0); END; $$",
			expectWarning: false,
		},
		// Procedure with SECURE keyword: Snowflake doesn't support SECURE PROCEDURE,
		// so the regex doesn't match and no procedure validation runs.
		{
			name:          "CREATE SECURE PROCEDURE not matched by procedure validator",
			sql:           "CREATE SECURE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// RETURNS TABLE without parentheses: ValidateDataTypes skips "TABLE"
		// so no "Unknown data type TABLE" warning should fire.
		{
			name:          "Valid RETURNS TABLE without parentheses",
			sql:           "CREATE PROCEDURE my_proc() RETURNS TABLE LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: false,
		},
		// AS at end of statement with no body content: preamble is truncated
		// at AS, all mandatory clauses are present, no warning fires.
		{
			name:          "Valid AS at end of statement with no body content after it",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS",
			expectWarning: false,
		},
		// Java HANDLER + IMPORTS without AS body: Snowflake allows staged-handler
		// procedures without inline body, but the validator always requires AS.
		{
			name:          "Known limitation: Java HANDLER+IMPORTS without AS body is flagged",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA RUNTIME_VERSION = '11' HANDLER = 'MyClass.handler' IMPORTS = ('@stage/my.jar')",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		// COMMENT containing "STRICT" keyword with actual CALLED ON NULL INPUT:
		// the \bSTRICT\b regex matches inside the COMMENT string value, producing
		// a false mutually exclusive conflict warning.
		{
			name:          "COMMENT containing STRICT no longer triggers false conflict",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL COMMENT = 'STRICT mode' CALLED ON NULL INPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},

		// ── Additional edge cases ─────────────────────────────────────────

		// Python staged handler without AS body: Snowflake allows staged-handler
		// procedures without inline body, but the validator always requires AS.
		{
			name:          "Known limitation: Python HANDLER+IMPORTS without AS body is flagged",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') HANDLER = 'main'",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		// Duplicate RETURNS clauses: first RETURNS satisfies the \bRETURNS\b check,
		// so no "missing RETURNS" warning fires.
		{
			name:          "Duplicate RETURNS clauses do not cause false missing RETURNS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR RETURNS NUMBER LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// No space between closing paren and RETURNS: the regex still matches
		// because \b matches at the boundary between ) and R.
		{
			name:          "Valid no space between closing paren and RETURNS",
			sql:           "CREATE PROCEDURE my_proc()RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Newline between CREATE and PROCEDURE: \s+ in reIsCreateProcedure
		// handles whitespace including newlines.
		{
			name:          "Valid CREATE followed by newline then PROCEDURE",
			sql:           "CREATE\nPROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// CREATE OR REPLACE with newlines between keywords.
		{
			name:          "Valid CREATE OR REPLACE with newlines between keywords",
			sql:           "CREATE\nOR\nREPLACE\nPROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Missing closing dollar-quote (unterminated body): preamble is
		// truncated at the first AS, mandatory clauses are checked normally.
		{
			name:          "Missing closing dollar-quote does not crash",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok';",
			expectWarning: false,
		},
		// COMMENT with semicolon in value: statement splitting is done
		// by the tokeniser which respects string boundaries, so this is valid.
		{
			name:          "Valid COMMENT with semicolon in value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'test; procedure' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// COMMENT with empty string value.
		{
			name:          "Valid COMMENT with empty string value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = '' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Scala staged handler without AS body: same as Java case.
		{
			name:          "Known limitation: Scala HANDLER+IMPORTS without AS body is flagged",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SCALA RUNTIME_VERSION = '2.12' HANDLER = 'MyObj.handler' IMPORTS = ('@stage/my.jar')",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		// EXECUTE AS DEFINER is not valid in Snowflake (only CALLER and OWNER).
		{
			name:          "Invalid EXECUTE AS DEFINER",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS DEFINER AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},

		// ── Clause ordering and additional parameter types ────────────────

		// LANGUAGE before RETURNS: preamble regex checks are order-independent.
		{
			name:          "Valid LANGUAGE before RETURNS (reversed clause order)",
			sql:           "CREATE PROCEDURE my_proc() LANGUAGE SQL RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// GEOGRAPHY, GEOMETRY, and BINARY as parameter types (tested as return
		// types elsewhere, but parameter-type validation uses a separate code path).
		{
			name:          "Valid GEOGRAPHY parameter type",
			sql:           "CREATE PROCEDURE my_proc(g GEOGRAPHY) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid GEOMETRY parameter type",
			sql:           "CREATE PROCEDURE my_proc(g GEOMETRY) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid BINARY parameter type",
			sql:           "CREATE PROCEDURE my_proc(b BINARY) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// IF NOT EXISTS with Python missing PACKAGES/IMPORTS (RUNTIME_VERSION case already tested).
		{
			name:          "IF NOT EXISTS Python missing PACKAGES and IMPORTS",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "PACKAGES or IMPORTS is required",
		},

		// ── CREATE TEMPORARY/TEMP PROCEDURE ─────────────────────────────

		// CREATE TEMPORARY PROCEDURE is not matched by reIsCreateProcedure
		// (regex expects CREATE [OR REPLACE] PROCEDURE), so no procedure
		// validation runs and no warnings fire (known limitation).
		{
			name:          "Known limitation: CREATE TEMPORARY PROCEDURE not validated",
			sql:           "CREATE TEMPORARY PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: regex doesn't match TEMPORARY
		},
		{
			name:          "Known limitation: CREATE TEMP PROCEDURE not validated",
			sql:           "CREATE TEMP PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: regex doesn't match TEMP
		},
		// CREATE OR REPLACE TEMPORARY PROCEDURE is similarly not matched.
		{
			name:          "Known limitation: CREATE OR REPLACE TEMPORARY PROCEDURE not validated",
			sql:           "CREATE OR REPLACE TEMPORARY PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: TEMPORARY between OR REPLACE and PROCEDURE
		},

		// ── LANGUAGE regex captures wrong token ─────────────────────────

		// LANGUAGE followed by another keyword (e.g. RETURNS) is captured
		// by the LANGUAGE regex as the value, producing "Unknown language"
		// instead of "Missing LANGUAGE".
		{
			name:          "LANGUAGE captures next keyword as value",
			sql:           "CREATE PROCEDURE my_proc() LANGUAGE RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},

		// ── Additional valid parameter types ────────────────────────────

		{
			name:          "Valid TIMESTAMP parameter type",
			sql:           "CREATE PROCEDURE my_proc(ts TIMESTAMP) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid INTEGER parameter type",
			sql:           "CREATE PROCEDURE my_proc(n INTEGER) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid DATE parameter type",
			sql:           "CREATE PROCEDURE my_proc(d DATE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid STRING parameter type",
			sql:           "CREATE PROCEDURE my_proc(s STRING) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid TIME parameter type",
			sql:           "CREATE PROCEDURE my_proc(t TIME) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid DOUBLE parameter type",
			sql:           "CREATE PROCEDURE my_proc(d DOUBLE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid ARRAY parameter type",
			sql:           "CREATE PROCEDURE my_proc(a ARRAY) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── NOT NULL after return type ───────────────────────────────────

		// Snowflake allows NOT NULL after the return type; the reReturnsType
		// regex captures only the base type name, so NOT NULL does not interfere.
		{
			name:          "Valid RETURNS VARCHAR NOT NULL",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR NOT NULL LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── COMMENT with escaped single quotes ──────────────────────────

		// Escaped single quotes (doubled) inside a COMMENT value must not
		// break the preamble AS detection.
		{
			name:          "Valid COMMENT with escaped single quotes",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'It''s a test' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── CREATE AGGREGATE PROCEDURE (not valid Snowflake syntax) ─────

		// Snowflake does not support CREATE AGGREGATE PROCEDURE; the regex
		// doesn't match, so no procedure validation runs.
		{
			name:          "CREATE AGGREGATE PROCEDURE not matched by procedure validator",
			sql:           "CREATE AGGREGATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Null-input clause case insensitivity ─────────────────────────

		// Lowercase null-input clauses: (?i) flag must match.
		{
			name:          "Valid lowercase called on null input",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL called on null input AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid lowercase strict",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL strict AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid lowercase returns null on null input",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL returns null on null input AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},
		{
			name:          "Conflict lowercase called on null input and strict",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL called on null input strict AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		{
			name:          "Conflict mixed case Called On Null Input and Strict",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL Called On Null Input Strict AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},

		// ── COPY GRANTS with invalid clauses ────────────────────────────

		// COPY GRANTS must not interfere with mandatory clause detection.
		{
			name:          "COPY GRANTS with missing LANGUAGE",
			sql:           "CREATE OR REPLACE PROCEDURE my_proc() COPY GRANTS RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},
		{
			name:          "COPY GRANTS with missing RETURNS",
			sql:           "CREATE OR REPLACE PROCEDURE my_proc() COPY GRANTS LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},

		// ── Clause ordering variations ───────────────────────────────────

		// Python with reversed clause order: PACKAGES before RUNTIME_VERSION.
		{
			name:          "Valid Python with PACKAGES before RUNTIME_VERSION",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') RUNTIME_VERSION = '3.8' AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},
		// EXECUTE AS between RETURNS and LANGUAGE: order-independent checks.
		{
			name:          "Valid EXECUTE AS CALLER between RETURNS and LANGUAGE",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR EXECUTE AS CALLER LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// STRICT before RETURNS clause: null-input checks are order-independent.
		{
			name:          "Valid STRICT before RETURNS",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL STRICT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},

		// ── EXECUTE AS with tab separator ────────────────────────────────

		// Tab between EXECUTE and AS: \s+ in the skip regex includes tabs.
		{
			name:          "Valid EXECUTE AS with tab between EXECUTE and AS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE\tAS CALLER AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Two-word return types ─────────────────────────────────────────

		// RETURNS DOUBLE PRECISION: the two-word type is captured as "DOUBLE"
		// by the single-word reReturnsType regex, which is a valid type.
		{
			name:          "Valid RETURNS DOUBLE PRECISION (two-word type)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS DOUBLE PRECISION LANGUAGE SQL AS $$ BEGIN RETURN 1.0; END; $$",
			expectWarning: false,
		},

		// ── Unicode in body ───────────────────────────────────────────────

		// Unicode characters in the procedure body must not affect preamble validation.
		{
			name:          "Valid procedure with Unicode in body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN '日本語テスト'; END; $$",
			expectWarning: false,
		},

		// ── RETURNS NULL ON NULL INPUT spanning newlines ──────────────────

		// CALLED ON NULL INPUT spanning newlines is tested above; the symmetric
		// case for RETURNS NULL ON NULL INPUT must also work via \s+.
		{
			name:          "Valid RETURNS NULL ON NULL INPUT spanning newlines",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL RETURNS\nNULL\nON\nNULL\nINPUT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: false,
		},

		// ── LANGUAGE value case insensitivity for non-Python languages ────

		// Only Python lowercase/mixed-case language value is tested above;
		// verify that Java, Scala, JavaScript, and SQL are also case-insensitive.
		{
			name:          "Valid LANGUAGE javascript lowercase",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE javascript AS $$ return 1; $$",
			expectWarning: false,
		},
		{
			name:          "Valid LANGUAGE Java title case",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE Java HANDLER = 'MyClass.handler' AS $$ class MyClass {} $$",
			expectWarning: false,
		},
		{
			name:          "Valid LANGUAGE scala lowercase",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE scala HANDLER = 'MyObj.handler' AS $$ object MyObj {} $$",
			expectWarning: false,
		},
		{
			name:          "Valid LANGUAGE Sql mixed case",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE Sql AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── IF NOT EXISTS with invalid EXECUTE AS ────────────────────────

		// IF NOT EXISTS is tested with missing LANGUAGE and missing PACKAGES,
		// but not with invalid EXECUTE AS. Preamble validation still runs.
		{
			name:          "IF NOT EXISTS with invalid EXECUTE AS",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS ADMIN AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},

		// ── COMMENT containing EXECUTE AS keyword ────────────────────────

		// COMMENT value containing "EXECUTE AS" must not confuse the
		// EXECUTE AS skip logic for body-AS detection.
		{
			name:          "Valid COMMENT containing EXECUTE AS keyword",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'EXECUTE AS CALLER example' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Python with lowercase imports keyword ────────────────────────

		// runtime_version and packages lowercase are tested; imports lowercase is not.
		{
			name:          "Valid Python with lowercase imports keyword",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' imports = ('@stage/file.py') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},

		// ── Procedure name with leading underscore ───────────────────────

		// Underscore-prefixed identifiers are valid in Snowflake.
		{
			name:          "Valid procedure name with leading underscore",
			sql:           "CREATE PROCEDURE _my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── EXECUTE AS value that is a valid language name ───────────────

		// EXECUTE AS PYTHON is invalid (must be CALLER or OWNER), even though
		// PYTHON is a valid LANGUAGE value.
		{
			name:          "Invalid EXECUTE AS PYTHON",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS PYTHON AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},

		// ── Unclosed / malformed parameter lists ────────────────────────

		// Missing closing parenthesis: extractBalancedBlockPat returns ""
		// so no parameter type validation occurs, but preamble validation
		// (RETURNS, LANGUAGE, AS) still runs and passes.
		{
			name:          "Unclosed parenthesis in parameter list does not crash",
			sql:           "CREATE PROCEDURE my_proc(a INT, b VARCHAR RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Nested parentheses in DEFAULT values ────────────────────────

		// extractBalancedBlockPat must track paren depth correctly so that
		// nested parens inside DEFAULT function calls don't prematurely
		// close the parameter list.
		{
			name:          "Valid nested parens in DEFAULT value with function call",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER DEFAULT COALESCE(NULL, 0), b VARCHAR) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Deeper nesting: DEFAULT with nested function calls.
		{
			name:          "Valid deeply nested parens in DEFAULT value",
			sql:           "CREATE PROCEDURE my_proc(a NUMBER DEFAULT ROUND(ABS(-1.5), 0), b VARCHAR) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Closing paren inside quoted DEFAULT value ────────────────────

		// extractBalancedBlockPat respects single-quoted strings, so a
		// closing paren inside a DEFAULT string literal must not close the
		// parameter list prematurely.
		{
			name:          "Valid closing paren inside single-quoted DEFAULT value",
			sql:           "CREATE PROCEDURE my_proc(a VARCHAR DEFAULT ')', b INT) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Same for double-quoted identifiers containing parens.
		{
			name:          "Valid closing paren inside double-quoted DEFAULT value",
			sql:           `CREATE PROCEDURE my_proc(a VARCHAR DEFAULT "col)", b INT) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},

		// ── FindStringSubmatch first-match semantics ────────────────────

		// First EXECUTE AS valid (CALLER), second invalid (ADMIN):
		// FindStringSubmatch returns the first match, which is valid,
		// so no EXECUTE AS warning fires.
		{
			name:          "Valid first EXECUTE AS CALLER second invalid ignored",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS CALLER EXECUTE AS ADMIN AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Duplicate valid LANGUAGE clauses: first match is valid (SQL),
		// so no unknown/missing LANGUAGE warning fires regardless of
		// second LANGUAGE value.
		{
			name:          "Valid duplicate LANGUAGE clauses both valid",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL LANGUAGE PYTHON AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// First LANGUAGE valid (SQL), second invalid (RUBY):
		// FindStringSubmatch returns the first match which is valid,
		// so no LANGUAGE warning fires.
		{
			name:          "Valid first LANGUAGE valid second invalid ignored",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL LANGUAGE RUBY AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Custom dollar-quote tags resembling keywords ────────────────

		// Custom dollar-quote tag that looks like a keyword: the body
		// delimiter $RETURNS$ must not confuse preamble RETURNS detection.
		{
			name:          "Valid custom dollar-quote tag resembling RETURNS keyword",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $RETURNS$ BEGIN RETURN 'ok'; END; $RETURNS$",
			expectWarning: false,
		},
		{
			name:          "Valid custom dollar-quote tag resembling LANGUAGE keyword",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $LANGUAGE$ BEGIN RETURN 'ok'; END; $LANGUAGE$",
			expectWarning: false,
		},

		// ── Non-Python languages with PACKAGES/IMPORTS ──────────────────

		// Java with PACKAGES clause: Python-specific PACKAGES requirement
		// check must not fire for non-Python languages.
		{
			name:          "Valid Java with PACKAGES clause no Python warnings",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA RUNTIME_VERSION = '11' PACKAGES = ('com.example:lib:1.0') HANDLER = 'MyClass.handler' AS $$ class MyClass {} $$",
			expectWarning: false,
		},
		// Scala with IMPORTS clause: Python-specific checks must not fire.
		{
			name:          "Valid Scala with IMPORTS clause no Python warnings",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SCALA RUNTIME_VERSION = '2.12' IMPORTS = ('@stage/my.jar') HANDLER = 'MyObj.handler' AS $$ object MyObj {} $$",
			expectWarning: false,
		},
		// JavaScript with no RUNTIME_VERSION, PACKAGES, or IMPORTS:
		// Python-specific checks must not fire for JavaScript.
		{
			name:          "Valid JavaScript without RUNTIME_VERSION or PACKAGES",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS $$ return 1; $$",
			expectWarning: false,
		},

		// ── Procedure name with keyword substrings ──────────────────────

		// Unquoted procedure name containing keyword substring: \b word
		// boundary in validation regexes prevents false matches.
		{
			name:          "Valid unquoted procedure name containing returns substring",
			sql:           "CREATE PROCEDURE returns_data() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid unquoted procedure name containing language substring",
			sql:           "CREATE PROCEDURE language_handler() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Bare CREATE OR REPLACE PROCEDURE ────────────────────────────

		// Bare CREATE OR REPLACE PROCEDURE with no name or body:
		// reIsCreateProcedure still matches and all mandatory clause
		// warnings fire.
		{
			name:          "Bare CREATE OR REPLACE PROCEDURE with no name",
			sql:           "CREATE OR REPLACE PROCEDURE",
			expectWarning: true,
			expectedMatch: "Missing mandatory",
		},

		// ── COMMENT with special content ────────────────────────────────

		// COMMENT with newline in value: multi-line string in COMMENT
		// must not break preamble validation.
		{
			name:          "Valid COMMENT with newline in value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'line1\nline2' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// COMMENT value containing "RETURNS LANGUAGE" must not be flagged: the
		// tokenizer classifies the COMMENT value as a string literal, so the
		// "RETURNS" inside it is never treated as a keyword. (Previously the
		// regex-based validator matched inside the string and falsely flagged
		// "LANGUAGE" as an unknown data type.)
		{
			name:          "COMMENT containing RETURNS LANGUAGE does not trigger false data type warning",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'RETURNS LANGUAGE EXECUTE AS' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Block comment placement ─────────────────────────────────────

		// Block comment between CREATE and PROCEDURE: parseText keeps
		// comments intact and \s+ in reIsCreateProcedure cannot span
		// the comment text, so the regex fails to match and no procedure
		// validation runs (known limitation).
		{
			name:          "Known limitation: block comment between CREATE and PROCEDURE not validated",
			sql:           "CREATE /* a comment */ PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: regex \s+ can't span block comment
		},
		// Block comment between OR and REPLACE similarly breaks the regex.
		{
			name:          "Known limitation: block comment between OR and REPLACE not validated",
			sql:           "CREATE OR /* comment */ REPLACE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative
		},

		// ── COPY GRANTS + IF NOT EXISTS combined ────────────────────────

		// Both are valid optional clauses that can appear together.
		{
			name:          "Valid OR REPLACE with COPY GRANTS and IF NOT EXISTS",
			sql:           "CREATE OR REPLACE PROCEDURE IF NOT EXISTS my_proc() COPY GRANTS RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── IF NOT EXISTS with null-input clause conflict ────────────────

		// Null-input conflict checks run on the preamble and must still
		// fire when IF NOT EXISTS is present.
		{
			name:          "IF NOT EXISTS with null-input clause conflict",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc(a INT) RETURNS INT LANGUAGE SQL CALLED ON NULL INPUT STRICT AS $$ BEGIN RETURN a; END; $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},

		// ── Trailing comma in parameter list ────────────────────────────

		// A trailing comma after the last parameter produces an empty
		// segment in parseColumnDefs; processColumnDef must handle the
		// empty/whitespace-only segment gracefully (0 tokens → skip).
		{
			name:          "Valid trailing comma in parameter list does not crash or warn",
			sql:           "CREATE PROCEDURE my_proc(a INT, b VARCHAR,) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── CREATE OR ALTER PROCEDURE ────────────────────────────────

		// CREATE OR ALTER PROCEDURE is valid Snowflake syntax but not matched
		// by reIsCreateProcedure (regex expects CREATE [OR REPLACE] PROCEDURE).
		{
			name:          "Known limitation: CREATE OR ALTER PROCEDURE not validated",
			sql:           "CREATE OR ALTER PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: regex doesn't match OR ALTER
		},
		{
			name:          "Known limitation: CREATE OR ALTER PROCEDURE missing LANGUAGE not caught",
			sql:           "CREATE OR ALTER PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: regex doesn't match OR ALTER
		},

		// ── IF NOT EXISTS additional combinations ────────────────────

		// IF NOT EXISTS with invalid LANGUAGE value: preamble validation
		// still runs and catches the unknown language.
		{
			name:          "IF NOT EXISTS with invalid LANGUAGE",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE RUBY AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		// IF NOT EXISTS with missing AS body: preamble validation catches it.
		{
			name:          "IF NOT EXISTS with missing AS body",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE SQL",
			expectWarning: true,
			expectedMatch: "Missing mandatory AS",
		},
		// IF NOT EXISTS with missing RETURNS clause.
		{
			name:          "IF NOT EXISTS with missing RETURNS",
			sql:           "CREATE PROCEDURE IF NOT EXISTS my_proc() LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory RETURNS",
		},

		// ── LANGUAGE value with special characters ───────────────────

		// Hyphenated value where first segment is not a valid language:
		// the regex [a-zA-Z0-9_]+ captures only the part before the hyphen.
		{
			name:          "LANGUAGE with hyphenated value captures first segment only",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PY-THON AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		// Hyphenated value where first segment IS a valid language:
		// "JAVA-SCRIPT" captures "JAVA", which is valid, so no warning fires.
		{
			name:          "Known limitation: LANGUAGE JAVA-SCRIPT accepted as JAVA",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA-SCRIPT HANDLER = 'MyClass.handler' AS $$ class MyClass {} $$",
			expectWarning: false, // false negative: regex captures "JAVA" which is valid
		},

		// ── LANGUAGE value across plain newline ──────────────────────────

		// LANGUAGE followed by value on the next line (no comment): \s+ in
		// the LANGUAGE regex includes \n, so the value is captured.
		{
			name:          "Valid LANGUAGE value on next line (plain newline)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE\nSQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── RETURNS TABLE + null-input handling combinations ─────────────

		// RETURNS TABLE combined with null-input clauses: the two distinct
		// RETURNS keywords (RETURNS TABLE and RETURNS NULL ON NULL INPUT)
		// must not interfere with each other.
		{
			name:          "Valid RETURNS TABLE with CALLED ON NULL INPUT",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL CALLED ON NULL INPUT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid RETURNS TABLE with STRICT",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL STRICT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid RETURNS TABLE with RETURNS NULL ON NULL INPUT",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: false,
		},
		// RETURNS TABLE with null-input conflict: the conflict check must
		// still fire even when the return type is TABLE.
		{
			name:          "RETURNS TABLE with CALLED ON NULL INPUT and STRICT conflict",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL CALLED ON NULL INPUT STRICT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},
		// RETURNS TABLE with STRICT + RETURNS NULL ON NULL INPUT: the
		// redundancy check must still fire even when the return type is TABLE.
		{
			name:          "RETURNS TABLE with STRICT and RETURNS NULL ON NULL INPUT redundancy",
			sql:           "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL STRICT RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$",
			expectWarning: true,
			expectedMatch: "redundant",
		},

		// ── AS body without dollar-quoting ───────────────────────────────

		// Snowflake SQL procedures support bare AS body without dollar-quoting:
		// CREATE PROCEDURE ... AS BEGIN ... END; is valid syntax.
		{
			name:          "Valid AS BEGIN...END body without dollar-quoting",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS BEGIN RETURN 'ok'; END;",
			expectWarning: false,
		},
		// Known limitation: non-dollar-quoted body with DECLARE...BEGIN...END
		// exposes the body to other validators (e.g., transaction BEGIN),
		// producing false positives.
		{
			name:          "Known limitation: AS DECLARE...BEGIN...END without dollar-quoting triggers false positives",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS DECLARE x VARCHAR; BEGIN x := 'ok'; RETURN x; END;",
			expectWarning: true,
			expectedMatch: "BEGIN",
		},

		// ── Procedure name with $ character ─────────────────────────────

		// Snowflake allows $ in unquoted identifiers; the regex uses [\w$]+
		// in identPath patterns.
		{
			name:          "Valid procedure name with dollar sign",
			sql:           "CREATE PROCEDURE my$proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid schema-qualified name with dollar sign",
			sql:           "CREATE PROCEDURE my_db.my$schema.my$proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Single-character invalid LANGUAGE ───────────────────────────

		// Single-character value: the LANGUAGE regex [a-zA-Z0-9_]+ captures
		// even a single character, which is then validated against the
		// allowed language list.
		{
			name:          "Invalid LANGUAGE single character C",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE C AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},

		// ── COMMENT containing Python-specific keywords ─────────────────

		// Known limitation: RUNTIME_VERSION inside a COMMENT string value
		// satisfies the \bRUNTIME_VERSION\b check (analogous to
		// LANGUAGE-in-comment known limitation).
		{
			name:          "COMMENT containing RUNTIME_VERSION correctly detected as missing",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON COMMENT = 'RUNTIME_VERSION is set' IMPORTS = ('@stage/f.py') AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "RUNTIME_VERSION",
		},
		// Known limitation: IMPORTS inside a COMMENT string value satisfies
		// the \bIMPORTS\b check, masking the missing PACKAGES/IMPORTS clause.
		{
			name:          "COMMENT containing IMPORTS correctly detected as missing",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' COMMENT = 'IMPORTS needed' AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "PACKAGES or IMPORTS",
		},
		// Known limitation: PACKAGES inside a COMMENT string value satisfies
		// the \bPACKAGES\b check similarly.
		{
			name:          "COMMENT containing PACKAGES correctly detected as missing",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' COMMENT = 'PACKAGES list' AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "PACKAGES or IMPORTS",
		},

		// ── COPY GRANTS + IF NOT EXISTS with missing clause ─────────────

		// Both COPY GRANTS and IF NOT EXISTS are valid optional clauses; a
		// missing mandatory clause must still be caught when both are present.
		{
			name:          "COPY GRANTS and IF NOT EXISTS with missing LANGUAGE",
			sql:           "CREATE OR REPLACE PROCEDURE IF NOT EXISTS my_proc() COPY GRANTS RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},

		// ── EXECUTE AS with value "AS" ──────────────────────────────────

		// When the EXECUTE AS value is literally "AS", the AS body finder
		// finds the second occurrence of AS and treats it as the body delimiter,
		// truncating the preamble before the EXECUTE AS value regex can match it.
		// Result: no EXECUTE AS warning fires (false negative) and the body
		// AS detection still works correctly.
		{
			name:          "Known limitation: EXECUTE AS value literally AS truncates preamble before EXECUTE AS check",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false, // false negative: body-AS finder treats the value AS as the body delimiter
		},

		// ── IF NOT EXISTS case insensitivity ─────────────────────────────

		// Lowercase IF NOT EXISTS: reIsCreateProcedure matches CREATE [OR
		// REPLACE] PROCEDURE, and the preamble regex checks use (?i), so
		// lowercase if not exists between PROCEDURE and the name must not
		// break validation.
		{
			name:          "Valid lowercase if not exists",
			sql:           "CREATE PROCEDURE if not exists my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid mixed case If Not Exists",
			sql:           "CREATE PROCEDURE If Not Exists my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Lowercase if not exists with missing LANGUAGE still caught",
			sql:           "create procedure if not exists my_proc() returns varchar as $$ begin return 'ok'; end; $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},

		// ── Invalid EXECUTE AS spanning newlines ────────────────────────

		// Invalid EXECUTE AS value with newlines between EXECUTE, AS, and
		// the value: the validator regex uses \s+ which includes newlines,
		// so the invalid value must still be caught.
		{
			name:          "Invalid EXECUTE AS spanning newlines with ADMIN value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE\nAS\nADMIN AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},

		// ── Schema-qualified name with invalid param type ────────────────

		// reCreateProcExt uses _identPath to match schema-qualified names;
		// parameter types must still be validated for 3-part qualified names.
		{
			name:          "Invalid param type with schema-qualified procedure name",
			sql:           "CREATE PROCEDURE mydb.myschema.my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},

		// ── Null-input reversed ordering ────────────────────────────────

		// RETURNS NULL ON NULL INPUT before CALLED ON NULL INPUT: the regex
		// checks are order-independent, so the reversed order must also
		// produce a mutually exclusive conflict.
		{
			name:          "Conflict RETURNS NULL ON NULL INPUT then CALLED ON NULL INPUT (reversed order)",
			sql:           "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL RETURNS NULL ON NULL INPUT CALLED ON NULL INPUT AS $$ $$",
			expectWarning: true,
			expectedMatch: "mutually exclusive",
		},

		// ── Procedure body with nested dollar-quote tags ────────────────

		// Body with nested dollar-quote tags: the outer $$ delimiters
		// should correctly contain inner $tag$ delimiters without
		// breaking preamble extraction.
		{
			name:          "Valid body with nested dollar-quote tags",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ DECLARE x VARCHAR; BEGIN x := $tag$inner text$tag$; RETURN x; END; $$",
			expectWarning: false,
		},

		// ── Only whitespace between AS and body ─────────────────────────

		// AS followed by newlines then dollar-quote: preamble should
		// still be correctly truncated at the AS keyword.
		{
			name:          "Valid AS followed by newlines then dollar-quote",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS\n\n\n$$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Python with HANDLER only (no PACKAGES/IMPORTS) ──────────────

		// Python with only HANDLER and RUNTIME_VERSION but no PACKAGES or
		// IMPORTS: the PACKAGES/IMPORTS check must still fire.
		{
			name:          "Python with HANDLER only missing PACKAGES and IMPORTS",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' HANDLER = 'main' AS $$ def main(session): return 'hello' $$",
			expectWarning: true,
			expectedMatch: "PACKAGES or IMPORTS is required",
		},

		// ── Case-insensitive data type validation ─────────────────────

		// All existing tests use UPPERCASE data types. The data type validator
		// must be case-insensitive since Snowflake identifiers are case-insensitive.
		{
			name:          "Valid lowercase parameter types int and varchar",
			sql:           "CREATE PROCEDURE my_proc(a int, b varchar, c number) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid lowercase return type varchar",
			sql:           "CREATE PROCEDURE my_proc() RETURNS varchar LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid mixed-case parameter types Int and Varchar",
			sql:           "CREATE PROCEDURE my_proc(a Int, b Varchar, c Boolean) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid mixed-case return type Number",
			sql:           "CREATE PROCEDURE my_proc() RETURNS Number LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: false,
		},
		// Invalid type in lowercase must still be caught.
		{
			name:          "Invalid lowercase parameter type badtype caught",
			sql:           "CREATE PROCEDURE my_proc(a badtype) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},
		// Invalid return type in lowercase must still be caught.
		{
			name:          "Invalid lowercase return type badreturn caught",
			sql:           "CREATE PROCEDURE my_proc() RETURNS badreturn LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown data type",
		},

		// ── Parameter names matching validation keywords ─────────────

		// Parameter names that match validation keywords (like "returns",
		// "language", "strict") must not confuse preamble checks because
		// preamble extraction operates on text after the parameter list.
		{
			name:          "Valid parameter named returns does not confuse preamble",
			sql:           "CREATE PROCEDURE my_proc(returns VARCHAR) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Known limitation: parameter named "language" is matched by the
		// \bLANGUAGE\s+...\b regex in the preamble (which includes param
		// list text), producing a false "Unknown LANGUAGE 'INT'" warning.
		{
			name:          "Known limitation: parameter named language confuses preamble LANGUAGE check",
			sql:           "CREATE PROCEDURE my_proc(language INT) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},

		// ── Adjacent dollar-quote delimiters (no whitespace) ─────────

		// AS followed by $$$$ (two adjacent dollar-quote delimiters with
		// zero-width empty body): the tokenizer must handle this correctly.
		{
			name:          "Valid adjacent dollar-quote empty body no space",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$$$",
			expectWarning: false,
		},

		// ── COMMENT containing dollar-quote characters ───────────────

		// Dollar-quote characters inside a COMMENT string value must not
		// interfere with body delimiter detection.
		{
			name:          "Valid COMMENT containing dollar-quote characters in value",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL COMMENT = 'test $$ content' AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Python RUNTIME_VERSION with multi-digit minor version ─────

		// Multi-digit minor version (e.g., 3.11): the regex captures the
		// entire quoted string, so version numbers with two-digit minor
		// parts must work.
		{
			name:          "Valid Python with RUNTIME_VERSION 3.11",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.11' PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},

		// ── Multiple DEFAULT values in parameter list ─────────────────

		// Multiple parameters with DEFAULT values: parseColumnDefs must
		// correctly split on commas while respecting DEFAULT expressions.
		{
			name:          "Valid multiple parameters with DEFAULT values",
			sql:           "CREATE PROCEDURE my_proc(a INT DEFAULT 1, b VARCHAR DEFAULT 'test', c BOOLEAN DEFAULT TRUE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Quoted all-digits procedure name ──────────────────────────

		// Quoted identifier containing only digits is valid in Snowflake.
		{
			name:          "Valid quoted procedure name all digits",
			sql:           `CREATE PROCEDURE "12345"() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`,
			expectWarning: false,
		},

		// ── NOT NULL on various return types ──────────────────────────

		// Only RETURNS VARCHAR NOT NULL is tested above. Verify NOT NULL
		// works with other return types without producing false positives.
		{
			name:          "Valid RETURNS INT NOT NULL",
			sql:           "CREATE PROCEDURE my_proc() RETURNS INT NOT NULL LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid RETURNS VARIANT NOT NULL",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARIANT NOT NULL LANGUAGE SQL AS $$ BEGIN RETURN PARSE_JSON('{}'); END; $$",
			expectWarning: false,
		},
		{
			name:          "Valid RETURNS NUMBER(10,2) NOT NULL",
			sql:           "CREATE PROCEDURE my_proc() RETURNS NUMBER(10,2) NOT NULL LANGUAGE SQL AS $$ BEGIN RETURN 3.14; END; $$",
			expectWarning: false,
		},

		// ── LANGUAGE value regex boundary tests ─────────────────────────

		// Underscore-containing LANGUAGE value: the regex [a-zA-Z0-9_]+
		// captures "JAVA_SCRIPT" as one token, which is not a valid language.
		{
			name:          "Invalid LANGUAGE with underscore JAVA_SCRIPT",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVA_SCRIPT AS $$ $$",
			expectWarning: true,
			expectedMatch: "Unknown or unsupported LANGUAGE",
		},
		// Single-quoted LANGUAGE value: the regex [a-zA-Z0-9_]+ does not
		// match quoted strings, so LANGUAGE 'SQL' produces "Missing LANGUAGE".
		{
			name:          "Known limitation: single-quoted LANGUAGE value produces missing LANGUAGE",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE 'SQL' AS $$ $$",
			expectWarning: true,
			expectedMatch: "Missing mandatory LANGUAGE",
		},

		// ── Unquoted procedure name matching exact keywords ─────────────

		// Unquoted procedure name "LANGUAGE" does not confuse the LANGUAGE
		// check because LANGUAGE() does not match LANGUAGE\s+([a-zA-Z0-9_]+).
		{
			name:          "Valid unquoted procedure name exactly LANGUAGE",
			sql:           "CREATE PROCEDURE LANGUAGE() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
		// Unquoted procedure name "RETURNS" does not confuse the RETURNS
		// check because \bRETURNS\b matches the real RETURNS clause too.
		{
			name:          "Valid unquoted procedure name exactly RETURNS",
			sql:           "CREATE PROCEDURE RETURNS() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},

		// ── Python multi-package PACKAGES clause ────────────────────────

		// PACKAGES clause with multiple comma-separated packages: the
		// \bPACKAGES\b presence check matches regardless of value complexity.
		{
			name:          "Valid Python with multiple packages in PACKAGES clause",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python', 'pandas', 'numpy') AS $$ def main(session): return 'hello' $$",
			expectWarning: false,
		},

		// ── EXECUTE AS with SQL identifier values ───────────────────────

		// EXECUTE AS VARCHAR: "VARCHAR" is not CALLER or OWNER, so the
		// validator flags it even though VARCHAR is a valid SQL data type.
		{
			name:          "Invalid EXECUTE AS VARCHAR (data type name as value)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS VARCHAR AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},
		// EXECUTE AS SQL: "SQL" is not CALLER or OWNER, so it is flagged
		// even though SQL is a valid LANGUAGE value.
		{
			name:          "Invalid EXECUTE AS SQL (language name as value)",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS SQL AS $$ $$",
			expectWarning: true,
			expectedMatch: "EXECUTE AS must be CALLER or OWNER",
		},

		// ── Dollar-quote tag mismatches ──────────────────────────────────

		// Mismatched dollar-quote tags: the AS-body finder locates the
		// first standalone \bAS\b, truncates the preamble there, and does
		// not validate body delimiter matching. So no warning fires.
		{
			name:          "Valid preamble with mismatched dollar-quote tags in body",
			sql:           "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $tag1$ body $tag2$",
			expectWarning: false,
		},

		// ── Leading comment before CREATE PROCEDURE ─────────────────────

		// Leading line comment before CREATE PROCEDURE on the next line:
		// reIsCreateProcedure uses ^\s*CREATE which cannot match the comment
		// text, but the tokenizer's first-token check should still route
		// the statement through procedure validation if CREATE is the
		// first SQL token after stripping comments.
		{
			name:          "Valid procedure preceded by leading line comment",
			sql:           "-- setup comment\nCREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
			expectWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)

			// ValidateDataTypes is a separate validator; combine its output to test parameter and return-type checking.
			markers = append(markers, ValidateDataTypes(tt.sql, ranges)...)

			warnings := getWarnings(markers)

			if tt.expectWarning {
				if len(warnings) == 0 {
					t.Fatalf("Expected warnings for %q, got 0", tt.sql)
				}
				found := false
				for _, w := range warnings {
					if strings.Contains(strings.ToLower(w.Message), strings.ToLower(tt.expectedMatch)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning matching %q, got: %v", tt.expectedMatch, warnings[0].Message)
				}
			} else {
				if len(warnings) > 0 {
					t.Errorf("Expected 0 warnings for %q, got %d: %v", tt.sql, len(warnings), warnings)
				}
			}
		})
	}
}

func TestValidateSnowflakePatterns_ProcedureMultipleWarnings(t *testing.T) {
	t.Run("Python missing everything produces both RUNTIME_VERSION and PACKAGES warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRuntime := false
		foundPackages := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "packages or imports is required") {
				foundPackages = true
			}
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning, not found")
		}
		if !foundPackages {
			t.Error("Expected PACKAGES or IMPORTS warning, not found")
		}
	})

	t.Run("All three null-input clauses produce both mutually exclusive and redundant warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT STRICT RETURNS NULL ON NULL INPUT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExclusive := false
		foundRedundant := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "mutually exclusive") {
				foundExclusive = true
			}
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
		}
		if !foundExclusive {
			t.Error("Expected mutually exclusive warning, not found")
		}
		if !foundRedundant {
			t.Error("Expected redundant warning, not found")
		}
	})

	t.Run("Missing RETURNS and LANGUAGE produces both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundReturns := false
		foundLanguage := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "missing mandatory language") {
				foundLanguage = true
			}
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
		if !foundLanguage {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
	})

	t.Run("Bare minimum procedure produces AS, RETURNS, and LANGUAGE warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc()"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundAS := false
		foundReturns := false
		foundLanguage := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory as") {
				foundAS = true
			}
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "missing mandatory language") {
				foundLanguage = true
			}
		}
		if !foundAS {
			t.Error("Expected missing AS warning, not found")
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
		if !foundLanguage {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
	})

	t.Run("Invalid EXECUTE AS without body AS produces both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS USER"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExecAs := false
		foundMissingAs := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
			if strings.Contains(msg, "missing mandatory as") {
				foundMissingAs = true
			}
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
		if !foundMissingAs {
			t.Error("Expected missing AS warning, not found")
		}
	})

	t.Run("Invalid parameter type AND invalid EXECUTE AS produce both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL EXECUTE AS ADMIN AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		foundType := false
		foundExecAs := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown data type") {
				foundType = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
		}
		if !foundType {
			t.Error("Expected unknown data type warning, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
	})

	t.Run("Python missing RUNTIME_VERSION AND invalid EXECUTE AS produce both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') EXECUTE AS ADMIN AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRuntime := false
		foundExecAs := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
	})

	t.Run("Missing LANGUAGE and invalid parameter type produce warnings from both validators", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		foundLang := false
		foundType := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory language") {
				foundLang = true
			}
			if strings.Contains(msg, "unknown data type") {
				foundType = true
			}
		}
		if !foundLang {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
		if !foundType {
			t.Error("Expected unknown data type warning, not found")
		}
	})

	t.Run("Missing RETURNS and invalid return type in ValidateDataTypes", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a INT) LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundReturns := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory returns") {
				foundReturns = true
			}
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
	})

	t.Run("Invalid param types AND invalid return type produce both data type warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADPARAM) RETURNS BADRETURN LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundParam := false
		foundReturn := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown data type 'badparam'") {
				foundParam = true
			}
			if strings.Contains(msg, "unknown data type 'badreturn'") {
				foundReturn = true
			}
		}
		if !foundParam {
			t.Error("Expected unknown data type warning for BADPARAM, not found")
		}
		if !foundReturn {
			t.Error("Expected unknown data type warning for BADRETURN, not found")
		}
	})

	t.Run("Python with PACKAGES and IMPORTS but missing RUNTIME_VERSION", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') IMPORTS = ('@stage/file.py') AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRuntime := false
		foundPackages := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "packages or imports is required") {
				foundPackages = true
			}
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning, not found")
		}
		if foundPackages {
			t.Error("Should not produce PACKAGES/IMPORTS warning when both are present")
		}
	})

	t.Run("Invalid LANGUAGE and missing RETURNS produce both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() LANGUAGE RUBY AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundLang := false
		foundReturns := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown or unsupported language") {
				foundLang = true
			}
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
		}
		if !foundLang {
			t.Error("Expected unknown LANGUAGE warning, not found")
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
	})

	t.Run("Missing RETURNS, LANGUAGE, and AS produce all three warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a INT)"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundAS := false
		foundReturns := false
		foundLanguage := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory as") {
				foundAS = true
			}
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "missing mandatory language") {
				foundLanguage = true
			}
		}
		if !foundAS {
			t.Error("Expected missing AS warning, not found")
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
		if !foundLanguage {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
	})

	t.Run("Invalid LANGUAGE with null-input conflict produces both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE RUBY CALLED ON NULL INPUT STRICT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundLang := false
		foundNull := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown or unsupported language") {
				foundLang = true
			}
			if strings.Contains(msg, "mutually exclusive") {
				foundNull = true
			}
		}
		if !foundLang {
			t.Error("Expected unknown LANGUAGE warning, not found")
		}
		if !foundNull {
			t.Error("Expected mutually exclusive warning, not found")
		}
	})

	t.Run("Invalid LANGUAGE, invalid EXECUTE AS, and null-input conflict produce all three warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE RUBY EXECUTE AS ADMIN CALLED ON NULL INPUT STRICT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundLang := false
		foundExecAs := false
		foundNull := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown or unsupported language") {
				foundLang = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
			if strings.Contains(msg, "mutually exclusive") {
				foundNull = true
			}
		}
		if !foundLang {
			t.Error("Expected unknown LANGUAGE warning, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
		if !foundNull {
			t.Error("Expected mutually exclusive warning, not found")
		}
	})

	t.Run("Python missing everything with invalid EXECUTE AS produces all warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON EXECUTE AS ADMIN AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRuntime := false
		foundPackages := false
		foundExecAs := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "packages or imports is required") {
				foundPackages = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning, not found")
		}
		if !foundPackages {
			t.Error("Expected PACKAGES or IMPORTS warning, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
	})

	t.Run("EXECUTE AS at EOF with no value produces missing AS but no invalid EXECUTE AS warning", func(t *testing.T) {
		// EXECUTE AS at EOF: the AS is consumed by the EXECUTE AS skip, so asBodyIdx=-1
		// producing "Missing mandatory AS". No "EXECUTE AS must be CALLER or OWNER"
		// because the value regex doesn't match (no value token after AS).
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundMissingAs := false
		foundExecAsInvalid := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory as") {
				foundMissingAs = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAsInvalid = true
			}
		}
		if !foundMissingAs {
			t.Error("Expected missing AS warning, not found")
		}
		if foundExecAsInvalid {
			t.Error("Should not produce 'EXECUTE AS must be CALLER or OWNER' when there's no value after EXECUTE AS")
		}
	})

	t.Run("LANGUAGE at EOF produces both missing LANGUAGE and missing AS warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundLang := false
		foundAS := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory language") {
				foundLang = true
			}
			if strings.Contains(msg, "missing mandatory as") {
				foundAS = true
			}
		}
		if !foundLang {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
		if !foundAS {
			t.Error("Expected missing AS warning, not found")
		}
	})

	t.Run("STRICT + RETURNS NULL ON NULL INPUT without CALLED produces only redundant warning", func(t *testing.T) {
		// When CALLED ON NULL INPUT is absent, only the "redundant" warning should
		// fire, NOT the "mutually exclusive" warning.
		sql := "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL STRICT RETURNS NULL ON NULL INPUT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRedundant := false
		foundExclusive := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
			if strings.Contains(msg, "mutually exclusive") {
				foundExclusive = true
			}
		}
		if !foundRedundant {
			t.Error("Expected redundant warning, not found")
		}
		if foundExclusive {
			t.Error("Should not produce mutually exclusive warning when CALLED ON NULL INPUT is absent")
		}
	})

	t.Run("Three invalid param types produce exactly three data type warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADONE, b BADTWO, c BADTHREE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		typeWarnings := 0
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				typeWarnings++
			}
		}
		if typeWarnings != 3 {
			t.Errorf("Expected exactly 3 unknown data type warnings, got %d", typeWarnings)
		}
	})

	t.Run("Null-input conflict + Python missing RUNTIME_VERSION produce both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') CALLED ON NULL INPUT STRICT AS $$ def main(session, a): return a $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRuntime := false
		foundNull := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "mutually exclusive") {
				foundNull = true
			}
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning, not found")
		}
		if !foundNull {
			t.Error("Expected mutually exclusive warning, not found")
		}
	})

	t.Run("Invalid return type (ValidateDataTypes) + invalid EXECUTE AS produce both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS BADRETURN LANGUAGE SQL EXECUTE AS ADMIN AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		foundType := false
		foundExecAs := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown data type") {
				foundType = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
		}
		if !foundType {
			t.Error("Expected unknown data type warning for BADRETURN, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
	})

	t.Run("Invalid LANGUAGE without body AS produces both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE RUBY"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundLang := false
		foundAS := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown or unsupported language") {
				foundLang = true
			}
			if strings.Contains(msg, "missing mandatory as") {
				foundAS = true
			}
		}
		if !foundLang {
			t.Error("Expected unknown LANGUAGE warning, not found")
		}
		if !foundAS {
			t.Error("Expected missing AS warning, not found")
		}
	})

	t.Run("COPY GRANTS with missing RETURNS and LANGUAGE produces both warnings", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE my_proc() COPY GRANTS AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundReturns := false
		foundLanguage := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "missing mandatory language") {
				foundLanguage = true
			}
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
		if !foundLanguage {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
	})

	t.Run("Lowercase null-input conflict produces mutually exclusive warning", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL called on null input returns null on null input AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExclusive := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "mutually exclusive") {
				foundExclusive = true
				break
			}
		}
		if !foundExclusive {
			t.Error("Expected mutually exclusive warning for lowercase null-input clauses, not found")
		}
	})

	t.Run("Kitchen-sink valid Python procedure with all optional clauses produces no warnings", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE my_proc(a INT, b VARCHAR) RETURNS VARCHAR NOT NULL LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') IMPORTS = ('@stage/file.py') HANDLER = 'main' EXECUTE AS CALLER CALLED ON NULL INPUT COMMENT = 'test procedure' AS $$ def main(session, a, b): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for kitchen-sink valid procedure, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("IF NOT EXISTS Python missing RUNTIME_VERSION with invalid EXECUTE AS produces both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') EXECUTE AS ADMIN AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRuntime := false
		foundExecAs := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
	})

	t.Run("Multiple invalid param types AND missing LANGUAGE produce warnings from both validators", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADONE, b BADTWO) RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		foundLang := false
		typeCount := 0
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory language") {
				foundLang = true
			}
			if strings.Contains(msg, "unknown data type") {
				typeCount++
			}
		}
		if !foundLang {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
		if typeCount < 2 {
			t.Errorf("Expected at least 2 unknown data type warnings, got %d", typeCount)
		}
	})

	t.Run("Kitchen-sink valid SQL procedure with all optional clauses produces no warnings", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE my_proc(a NUMBER(10,2), b VARCHAR(255), c BOOLEAN) RETURNS TABLE(id INT, name VARCHAR) LANGUAGE SQL EXECUTE AS OWNER STRICT COMMENT = 'comprehensive test' AS $$ BEGIN RETURN TABLE(SELECT 1, 'a'); END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for kitchen-sink valid SQL procedure, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Missing AS body with invalid parameter type produces both warnings from independent validators", func(t *testing.T) {
		// ValidateSnowflakePatterns fires "missing AS" while ValidateDataTypes
		// fires "unknown data type" — the two validators are independent.
		sql := "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		foundAS := false
		foundType := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory as") {
				foundAS = true
			}
			if strings.Contains(msg, "unknown data type") {
				foundType = true
			}
		}
		if !foundAS {
			t.Error("Expected missing AS warning, not found")
		}
		if !foundType {
			t.Error("Expected unknown data type warning, not found")
		}
	})

	t.Run("Bare CREATE OR REPLACE PROCEDURE produces all three mandatory warnings", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundAS := false
		foundReturns := false
		foundLanguage := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory as") {
				foundAS = true
			}
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "missing mandatory language") {
				foundLanguage = true
			}
		}
		if !foundAS {
			t.Error("Expected missing AS warning, not found")
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
		if !foundLanguage {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
	})

	t.Run("First valid EXECUTE AS hides second invalid from FindStringSubmatch", func(t *testing.T) {
		// FindStringSubmatch returns only the first match; if it's valid (CALLER),
		// the second invalid value (ADMIN) is never checked.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE AS CALLER EXECUTE AS ADMIN AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "execute as must be caller or owner") {
				t.Errorf("First valid EXECUTE AS should prevent invalid EXECUTE AS warning, got: %s", w.Message)
			}
		}
	})

	t.Run("First valid LANGUAGE hides second invalid from FindStringSubmatch", func(t *testing.T) {
		// FindStringSubmatch returns only the first match; if it's valid (SQL),
		// the second invalid value (RUBY) is never checked.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL LANGUAGE RUBY AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown or unsupported language") || strings.Contains(msg, "missing mandatory language") {
				t.Errorf("First valid LANGUAGE should prevent LANGUAGE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("CALLED ON NULL INPUT + RETURNS NULL ON NULL INPUT without STRICT produces only mutually exclusive warning", func(t *testing.T) {
		// Symmetric to the STRICT + RETURNS NULL test above: when STRICT is absent,
		// only "mutually exclusive" should fire, NOT "redundant".
		sql := "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT RETURNS NULL ON NULL INPUT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExclusive := false
		foundRedundant := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "mutually exclusive") {
				foundExclusive = true
			}
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
		}
		if !foundExclusive {
			t.Error("Expected mutually exclusive warning, not found")
		}
		if foundRedundant {
			t.Error("Should not produce redundant warning when STRICT is absent")
		}
	})

	t.Run("CALLED ON NULL INPUT + STRICT without RETURNS NULL produces only mutually exclusive warning", func(t *testing.T) {
		// When RETURNS NULL ON NULL INPUT is absent, only "mutually exclusive"
		// should fire, NOT "redundant".
		sql := "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL CALLED ON NULL INPUT STRICT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExclusive := false
		foundRedundant := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "mutually exclusive") {
				foundExclusive = true
			}
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
		}
		if !foundExclusive {
			t.Error("Expected mutually exclusive warning, not found")
		}
		if foundRedundant {
			t.Error("Should not produce redundant warning when RETURNS NULL ON NULL INPUT is absent")
		}
	})

	t.Run("Missing LANGUAGE does not produce Python-specific warnings", func(t *testing.T) {
		// When LANGUAGE is missing, langMatch == nil so the else block
		// (which contains Python-specific checks) never executes.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version") {
				t.Errorf("Missing LANGUAGE should not trigger Python RUNTIME_VERSION warning, got: %s", w.Message)
			}
			if strings.Contains(msg, "packages or imports") {
				t.Errorf("Missing LANGUAGE should not trigger Python PACKAGES/IMPORTS warning, got: %s", w.Message)
			}
		}
	})

	t.Run("Invalid LANGUAGE does not produce Python-specific warnings", func(t *testing.T) {
		// When LANGUAGE is invalid (RUBY), Python-specific checks (RUNTIME_VERSION,
		// PACKAGES/IMPORTS) must NOT fire because they are gated on lang == "PYTHON".
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE RUBY AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version") {
				t.Errorf("Invalid LANGUAGE should not trigger Python RUNTIME_VERSION warning, got: %s", w.Message)
			}
			if strings.Contains(msg, "packages or imports") {
				t.Errorf("Invalid LANGUAGE should not trigger Python PACKAGES/IMPORTS warning, got: %s", w.Message)
			}
		}
	})

	t.Run("COPY GRANTS with invalid EXECUTE AS produces EXECUTE AS warning", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE my_proc() COPY GRANTS RETURNS VARCHAR LANGUAGE SQL EXECUTE AS ADMIN AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExecAs := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "execute as must be caller or owner") {
				foundExecAs = true
				break
			}
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning with COPY GRANTS, not found")
		}
	})

	t.Run("IF NOT EXISTS Python missing everything produces both Python-specific warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE IF NOT EXISTS my_proc() RETURNS VARCHAR LANGUAGE PYTHON AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRuntime := false
		foundPackages := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "packages or imports is required") {
				foundPackages = true
			}
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning with IF NOT EXISTS, not found")
		}
		if !foundPackages {
			t.Error("Expected PACKAGES or IMPORTS warning with IF NOT EXISTS, not found")
		}
	})

	t.Run("Multiline kitchen-sink Python procedure produces no warnings", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE my_proc(\n  a INT,\n  b VARCHAR\n)\nRETURNS VARCHAR NOT NULL\nLANGUAGE PYTHON\nRUNTIME_VERSION = '3.8'\nPACKAGES = ('snowflake-snowpark-python')\nIMPORTS = ('@stage/file.py')\nHANDLER = 'main'\nEXECUTE AS CALLER\nCALLED ON NULL INPUT\nCOMMENT = 'test procedure'\nAS\n$$\ndef main(session, a, b):\n    return 'hello'\n$$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for multiline kitchen-sink Python procedure, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("IF NOT EXISTS with invalid LANGUAGE and missing RETURNS produce both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE IF NOT EXISTS my_proc() LANGUAGE RUBY AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundLang := false
		foundReturns := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown or unsupported language") {
				foundLang = true
			}
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
		}
		if !foundLang {
			t.Error("Expected unknown LANGUAGE warning, not found")
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
	})

	t.Run("IF NOT EXISTS with missing RETURNS, LANGUAGE, and AS produce all three warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE IF NOT EXISTS my_proc()"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundAS := false
		foundReturns := false
		foundLanguage := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory as") {
				foundAS = true
			}
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "missing mandatory language") {
				foundLanguage = true
			}
		}
		if !foundAS {
			t.Error("Expected missing AS warning, not found")
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
		if !foundLanguage {
			t.Error("Expected missing LANGUAGE warning, not found")
		}
	})

	t.Run("Unclosed paren still validates preamble but skips param type checking", func(t *testing.T) {
		// An unterminated parameter list produces no param type warnings (the
		// token-based walker skips unbalanced paren groups). But
		// ValidateSnowflakePatterns still checks mandatory clauses.
		sql := "CREATE PROCEDURE my_proc(a BADTYPE, b ALSOBAD RETURNS VARCHAR LANGUAGE SQL AS $$ $$"
		ranges := GetStatementRanges(sql)

		// Preamble validation should not fire warnings (RETURNS/LANGUAGE/AS all present).
		preambleMarkers := ValidateSnowflakePatterns(sql, ranges)
		preambleWarnings := getWarnings(preambleMarkers)
		if len(preambleWarnings) > 0 {
			t.Errorf("Expected 0 preamble warnings for unclosed paren, got %d: %v", len(preambleWarnings), preambleWarnings)
		}

		// Data type validation should produce no param warnings because
		// extractBalancedBlockPat returns empty for the unclosed paren.
		typeMarkers := ValidateDataTypes(sql, ranges)
		typeWarnings := getWarnings(typeMarkers)
		for _, w := range typeWarnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") ||
				strings.Contains(strings.ToLower(w.Message), "alsobad") {
				t.Errorf("Unclosed paren should skip param type validation, got: %s", w.Message)
			}
		}
	})

	t.Run("RETURNS TABLE with CALLED ON NULL INPUT and STRICT produces conflict warning", func(t *testing.T) {
		// Null-input conflict detection must fire even when the return type
		// is TABLE (two RETURNS keywords in the preamble).
		sql := "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL CALLED ON NULL INPUT STRICT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExclusive := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "mutually exclusive") {
				foundExclusive = true
				break
			}
		}
		if !foundExclusive {
			t.Error("Expected mutually exclusive warning for RETURNS TABLE + CALLED ON NULL INPUT + STRICT, not found")
		}
	})

	t.Run("RETURNS TABLE with RETURNS NULL ON NULL INPUT produces no conflict", func(t *testing.T) {
		// RETURNS TABLE and RETURNS NULL ON NULL INPUT both contain RETURNS
		// but are independent clauses. No conflict should fire.
		sql := "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for RETURNS TABLE + RETURNS NULL ON NULL INPUT, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("RETURNS TABLE with STRICT + RETURNS NULL produces redundant warning only", func(t *testing.T) {
		// When CALLED ON NULL INPUT is absent but both STRICT and RETURNS NULL
		// ON NULL INPUT are present with a TABLE return type, only the
		// "redundant" warning should fire, NOT "mutually exclusive".
		sql := "CREATE PROCEDURE my_proc(a INT) RETURNS TABLE(id INT) LANGUAGE SQL STRICT RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN TABLE(SELECT 1); END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRedundant := false
		foundExclusive := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
			if strings.Contains(msg, "mutually exclusive") {
				foundExclusive = true
			}
		}
		if !foundRedundant {
			t.Error("Expected redundant warning for RETURNS TABLE + STRICT + RETURNS NULL, not found")
		}
		if foundExclusive {
			t.Error("Should not produce mutually exclusive warning when CALLED ON NULL INPUT is absent")
		}
	})

	t.Run("COPY GRANTS + IF NOT EXISTS with missing RETURNS and LANGUAGE produce both warnings", func(t *testing.T) {
		sql := "CREATE OR REPLACE PROCEDURE IF NOT EXISTS my_proc() COPY GRANTS AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundReturns := false
		foundLanguage := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "missing mandatory language") {
				foundLanguage = true
			}
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning with COPY GRANTS + IF NOT EXISTS, not found")
		}
		if !foundLanguage {
			t.Error("Expected missing LANGUAGE warning with COPY GRANTS + IF NOT EXISTS, not found")
		}
	})

	t.Run("Invalid EXECUTE AS + null-input redundancy produce both warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a INT) RETURNS INT LANGUAGE SQL EXECUTE AS ADMIN STRICT RETURNS NULL ON NULL INPUT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExecAs := false
		foundRedundant := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
		if !foundRedundant {
			t.Error("Expected redundant null-input warning, not found")
		}
	})

	t.Run("Multiline invalid EXECUTE AS value is still caught", func(t *testing.T) {
		// EXECUTE\nAS\nADMIN: the EXECUTE AS skip regex uses \s+ which
		// matches newlines, and the value regex also uses \s+, so the
		// invalid value must be caught across newline boundaries.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL EXECUTE\nAS\nADMIN AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExecAs := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "execute as must be caller or owner") {
				foundExecAs = true
				break
			}
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning for multiline EXECUTE AS ADMIN, not found")
		}
	})

	t.Run("RETURNS NULL ON NULL INPUT before CALLED ON NULL INPUT produces only mutually exclusive warning", func(t *testing.T) {
		// Reversed order: RETURNS NULL first, then CALLED. The regex
		// checks hasCalledOnNull && (hasReturnsNull || hasStrict) are
		// order-independent, so the conflict must still be detected.
		sql := "CREATE PROCEDURE my_proc(a int) RETURNS int LANGUAGE SQL RETURNS NULL ON NULL INPUT CALLED ON NULL INPUT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundExclusive := false
		foundRedundant := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "mutually exclusive") {
				foundExclusive = true
			}
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
		}
		if !foundExclusive {
			t.Error("Expected mutually exclusive warning for reversed null-input clause order, not found")
		}
		if foundRedundant {
			t.Error("Should not produce redundant warning when STRICT is absent")
		}
	})

	t.Run("Python procedure missing everything plus missing RETURNS produces all four warnings", func(t *testing.T) {
		// Missing RETURNS + RUNTIME_VERSION + PACKAGES/IMPORTS: all four
		// warnings (missing RETURNS, RUNTIME_VERSION, PACKAGES/IMPORTS)
		// must fire simultaneously.
		sql := "CREATE PROCEDURE my_proc() LANGUAGE PYTHON AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundReturns := false
		foundRuntime := false
		foundPackages := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory returns") {
				foundReturns = true
			}
			if strings.Contains(msg, "runtime_version is required") {
				foundRuntime = true
			}
			if strings.Contains(msg, "packages or imports is required") {
				foundPackages = true
			}
		}
		if !foundReturns {
			t.Error("Expected missing RETURNS warning, not found")
		}
		if !foundRuntime {
			t.Error("Expected RUNTIME_VERSION warning, not found")
		}
		if !foundPackages {
			t.Error("Expected PACKAGES or IMPORTS warning, not found")
		}
	})

	t.Run("Schema-qualified procedure with invalid param type AND invalid return type", func(t *testing.T) {
		sql := "CREATE PROCEDURE mydb.myschema.my_proc(a BADPARAM) RETURNS BADRETURN LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundParam := false
		foundReturn := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown data type 'badparam'") {
				foundParam = true
			}
			if strings.Contains(msg, "unknown data type 'badreturn'") {
				foundReturn = true
			}
		}
		if !foundParam {
			t.Error("Expected unknown data type warning for BADPARAM with schema-qualified name, not found")
		}
		if !foundReturn {
			t.Error("Expected unknown data type warning for BADRETURN with schema-qualified name, not found")
		}
	})

	t.Run("IF NOT EXISTS with null-input redundancy produces redundant warning", func(t *testing.T) {
		sql := "CREATE PROCEDURE IF NOT EXISTS my_proc(a INT) RETURNS INT LANGUAGE SQL STRICT RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN a; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundRedundant := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "redundant") {
				foundRedundant = true
				break
			}
		}
		if !foundRedundant {
			t.Error("Expected redundant warning for IF NOT EXISTS + STRICT + RETURNS NULL ON NULL INPUT, not found")
		}
	})

	t.Run("Python procedure does not require HANDLER (unlike functions)", func(t *testing.T) {
		// Unlike CREATE FUNCTION where HANDLER is mandatory for Python,
		// CREATE PROCEDURE does not require HANDLER. Verify no HANDLER
		// warning fires for a valid Python procedure without HANDLER.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "handler") {
				t.Errorf("Python procedure should not require HANDLER, got: %s", w.Message)
			}
		}
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for valid Python procedure without HANDLER, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Lowercase valid data types produce no warnings from either validator", func(t *testing.T) {
		// Verify case-insensitive data type validation with combined validators.
		sql := "CREATE PROCEDURE my_proc(a int, b varchar, c boolean) RETURNS number LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for lowercase valid data types, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Mixed-case valid data types produce no warnings from either validator", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a Int, b Varchar, c Float) RETURNS Boolean LANGUAGE SQL AS $$ BEGIN RETURN TRUE; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for mixed-case valid data types, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Invalid param type + invalid return type + invalid EXECUTE AS + null-input conflict produce all warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADPARAM) RETURNS BADRETURN LANGUAGE SQL EXECUTE AS ADMIN CALLED ON NULL INPUT STRICT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		foundParamType := false
		foundReturnType := false
		foundExecAs := false
		foundNull := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown data type") && strings.Contains(msg, "badparam") {
				foundParamType = true
			}
			if strings.Contains(msg, "unknown data type") && strings.Contains(msg, "badreturn") {
				foundReturnType = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
			if strings.Contains(msg, "mutually exclusive") {
				foundNull = true
			}
		}
		if !foundParamType {
			t.Error("Expected unknown data type warning for BADPARAM, not found")
		}
		if !foundReturnType {
			t.Error("Expected unknown data type warning for BADRETURN, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
		if !foundNull {
			t.Error("Expected mutually exclusive warning, not found")
		}
	})

	t.Run("Invalid LANGUAGE + invalid EXECUTE AS + null-input redundancy + invalid param type produce all warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a FAKETYPE) RETURNS VARCHAR LANGUAGE RUBY EXECUTE AS ADMIN STRICT RETURNS NULL ON NULL INPUT AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		foundLang := false
		foundExecAs := false
		foundRedundant := false
		foundParamType := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "unknown or unsupported language") {
				foundLang = true
			}
			if strings.Contains(msg, "execute as must be caller or owner") {
				foundExecAs = true
			}
			if strings.Contains(msg, "redundant") {
				foundRedundant = true
			}
			if strings.Contains(msg, "unknown data type") {
				foundParamType = true
			}
		}
		if !foundLang {
			t.Error("Expected unknown LANGUAGE warning, not found")
		}
		if !foundExecAs {
			t.Error("Expected EXECUTE AS warning, not found")
		}
		if !foundRedundant {
			t.Error("Expected redundant null-input warning, not found")
		}
		if !foundParamType {
			t.Error("Expected unknown data type warning for FAKETYPE, not found")
		}
	})
}

func TestValidateSnowflakePatterns_ProcedureInMultiStatement(t *testing.T) {
	t.Run("Valid procedure as second statement", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Invalid procedure as second statement", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected warnings for procedure missing LANGUAGE in second statement")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory language") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected missing LANGUAGE warning, got: %v", warnings[0].Message)
		}
	})

	t.Run("Invalid procedure between valid statements", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc() LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$;\nSELECT 2;"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory returns") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected missing RETURNS warning for procedure in middle of multi-statement SQL")
		}
	})

	t.Run("Multiple procedures validated independently", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected warnings for second procedure missing LANGUAGE")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory language") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected missing LANGUAGE warning, got: %v", warnings[0].Message)
		}
	})

	t.Run("Valid procedure as first statement", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nSELECT 1"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Both procedures valid, no warnings", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2() RETURNS NUMBER LANGUAGE JAVASCRIPT AS $$ return 1; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for two valid procedures, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Three statements: SELECT, valid procedure, SELECT", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nSELECT 2"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for valid procedure between SELECTs, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Both procedures invalid, each validated independently", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2() LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundMissingLang := false
		foundMissingReturns := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory language") {
				foundMissingLang = true
			}
			if strings.Contains(msg, "missing mandatory returns") {
				foundMissingReturns = true
			}
		}
		if !foundMissingLang {
			t.Error("Expected missing LANGUAGE warning for p1, not found")
		}
		if !foundMissingReturns {
			t.Error("Expected missing RETURNS warning for p2, not found")
		}
	})

	t.Run("ValidateDataTypes catches invalid param type in second statement", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning in second statement, not found")
		}
	})

	t.Run("Valid procedure mixed with invalid procedure data types", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2(a FAKETYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for p2, not found")
		}
	})

	t.Run("Python procedure as second statement with missing RUNTIME_VERSION", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON PACKAGES = ('snowflake-snowpark-python') AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "runtime_version is required") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected RUNTIME_VERSION warning for Python procedure in second statement, not found")
		}
	})

	t.Run("Procedure and function in same SQL validated independently", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE FUNCTION my_func() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)

		foundProcLang := false
		foundFuncLang := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "missing mandatory language") && strings.Contains(msg, "procedure") {
				foundProcLang = true
			}
			if strings.Contains(msg, "missing mandatory language") && strings.Contains(msg, "function") {
				foundFuncLang = true
			}
		}
		if !foundProcLang {
			t.Error("Expected missing LANGUAGE warning for procedure, not found")
		}
		if !foundFuncLang {
			t.Error("Expected missing LANGUAGE warning for function, not found")
		}
	})

	t.Run("Python procedure missing PACKAGES as third statement", func(t *testing.T) {
		sql := "SELECT 1;\nSELECT 2;\nCREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE PYTHON RUNTIME_VERSION = '3.8' AS $$ def main(session): return 'hello' $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "packages or imports is required") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected PACKAGES or IMPORTS warning for Python procedure in third statement, not found")
		}
	})

	t.Run("ValidateDataTypes catches invalid return type in second statement", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc() RETURNS BADRETURN LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for invalid return type in second statement, not found")
		}
	})

	t.Run("Valid procedure and invalid procedure return types in multi-statement", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2() RETURNS BADRETURN LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") &&
				strings.Contains(strings.ToLower(w.Message), "badreturn") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADRETURN in second procedure, not found")
		}
	})

	t.Run("Empty statement (double semicolons) before procedure does not interfere", func(t *testing.T) {
		sql := "SELECT 1;;\nCREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings after empty statement, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("First invalid procedure does not contaminate second valid procedure", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)

		// p1 is missing LANGUAGE so should produce a warning.
		foundLangWarning := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory language") {
				foundLangWarning = true
				// Ensure the warning points to line 1 (p1), not line 2 (p2).
				if w.StartLineNumber != 1 {
					t.Errorf("Expected missing LANGUAGE warning on line 1 (p1), got line %d", w.StartLineNumber)
				}
				break
			}
		}
		if !foundLangWarning {
			t.Error("Expected missing LANGUAGE warning for p1, not found")
		}

		// p2 is valid; no warnings should point to line 2.
		for _, w := range warnings {
			if w.StartLineNumber == 2 {
				t.Errorf("Valid p2 on line 2 should not produce warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("Empty statement (double semicolons) before invalid procedure still catches errors", func(t *testing.T) {
		sql := "SELECT 1;;\nCREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory language") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected missing LANGUAGE warning after empty statement, not found")
		}
	})
}

func TestValidateSnowflakePatterns_ProcedureMarkerPositions(t *testing.T) {
	t.Run("Single-line procedure marker spans line 1", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one warning (missing LANGUAGE)")
		}
		for _, w := range warnings {
			if w.StartLineNumber != 1 {
				t.Errorf("Expected StartLineNumber=1, got %d for: %s", w.StartLineNumber, w.Message)
			}
			if w.EndLineNumber != 1 {
				t.Errorf("Expected EndLineNumber=1, got %d for: %s", w.EndLineNumber, w.Message)
			}
		}
	})

	t.Run("Multi-line procedure marker spans all statement lines", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc()\nRETURNS VARCHAR\nAS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one warning (missing LANGUAGE)")
		}
		for _, w := range warnings {
			if w.StartLineNumber != 1 {
				t.Errorf("Expected StartLineNumber=1, got %d for: %s", w.StartLineNumber, w.Message)
			}
			if w.EndLineNumber != 3 {
				t.Errorf("Expected EndLineNumber=3, got %d for: %s", w.EndLineNumber, w.Message)
			}
		}
	})

	t.Run("Procedure in second statement has correct line range", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc()\nRETURNS VARCHAR\nAS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one warning (missing LANGUAGE)")
		}
		for _, w := range warnings {
			if w.StartLineNumber != 2 {
				t.Errorf("Expected StartLineNumber=2, got %d for: %s", w.StartLineNumber, w.Message)
			}
			if w.EndLineNumber != 4 {
				t.Errorf("Expected EndLineNumber=4, got %d for: %s", w.EndLineNumber, w.Message)
			}
		}
	})

	t.Run("Marker StartColumn is 1 and EndColumn is 100", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one warning (missing RETURNS)")
		}
		w := warnings[0]
		if w.StartColumn != 1 {
			t.Errorf("Expected StartColumn=1, got %d", w.StartColumn)
		}
		if w.EndColumn != 100 {
			t.Errorf("Expected EndColumn=100, got %d", w.EndColumn)
		}
	})
}

func TestValidateSnowflakePatterns_ProcedureMarkerSeverity(t *testing.T) {
	t.Run("ValidateSnowflakePatterns markers have severity 4 (Warning)", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		if len(markers) == 0 {
			t.Fatal("Expected at least one marker")
		}
		for _, m := range markers {
			if m.Severity != 4 {
				t.Errorf("Expected Severity=4 (Warning), got %d for: %s", m.Severity, m.Message)
			}
		}
	})

	t.Run("ValidateDataTypes markers have severity 4 (Warning)", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		if len(markers) == 0 {
			t.Fatal("Expected at least one marker")
		}
		for _, m := range markers {
			if m.Severity != 4 {
				t.Errorf("Expected Severity=4 (Warning), got %d for: %s", m.Severity, m.Message)
			}
		}
	})
}

func TestValidateSnowflakePatterns_ProcedureBoundaryConditions(t *testing.T) {
	t.Run("Empty SQL produces no procedure warnings", func(t *testing.T) {
		sql := ""
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for empty SQL, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Whitespace-only SQL produces no procedure warnings", func(t *testing.T) {
		sql := "   \n\t\n   "
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		markers = append(markers, ValidateDataTypes(sql, ranges)...)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 warnings for whitespace SQL, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Non-procedure CREATE does not produce procedure warnings", func(t *testing.T) {
		sql := "CREATE TABLE my_table (id INT)"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") {
				t.Errorf("CREATE TABLE should not trigger procedure warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("Semicolon-only SQL produces no procedure warnings", func(t *testing.T) {
		sql := ";"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "procedure") {
				t.Errorf("Semicolon-only SQL should not trigger procedure warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("Comment-only SQL produces no procedure warnings", func(t *testing.T) {
		sql := "-- CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "procedure") {
				t.Errorf("Commented-out procedure should not trigger procedure warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("Block-commented procedure produces no procedure warnings", func(t *testing.T) {
		sql := "/* CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ $$ */"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "procedure") {
				t.Errorf("Block-commented procedure should not trigger procedure warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("DROP PROCEDURE does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "DROP PROCEDURE my_proc(VARCHAR)"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("DROP PROCEDURE should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("ALTER PROCEDURE does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "ALTER PROCEDURE my_proc(VARCHAR) SET COMMENT = 'updated'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("ALTER PROCEDURE should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("DESCRIBE PROCEDURE does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "DESCRIBE PROCEDURE my_proc(VARCHAR)"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("DESCRIBE PROCEDURE should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("CREATE FUNCTION does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "CREATE FUNCTION my_func() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") {
				t.Errorf("CREATE FUNCTION should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("CREATE PROCEDURE inside string literal does not trigger validation", func(t *testing.T) {
		sql := "SELECT 'CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL'"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "procedure") {
				t.Errorf("CREATE PROCEDURE inside string literal should not trigger warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("CALL statement does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "CALL my_proc()"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("CALL statement should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("SHOW PROCEDURES does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "SHOW PROCEDURES"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("SHOW PROCEDURES should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("SELECT with procedure keyword in alias does not trigger procedure warnings", func(t *testing.T) {
		sql := "SELECT 1 AS procedure_result"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("SELECT with procedure keyword in alias should not trigger procedure warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("PROCEDURE keyword without CREATE does not trigger procedure warnings", func(t *testing.T) {
		sql := "PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("PROCEDURE without CREATE should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("REPLACE PROCEDURE without CREATE does not trigger procedure warnings", func(t *testing.T) {
		sql := "REPLACE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("REPLACE PROCEDURE without CREATE should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("CREATE VIEW does not trigger procedure warnings", func(t *testing.T) {
		sql := "CREATE VIEW my_view AS SELECT 1"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("CREATE VIEW should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("EXECUTE PROCEDURE does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "EXECUTE PROCEDURE my_proc()"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory returns") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory language") {
				t.Errorf("EXECUTE PROCEDURE should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("GRANT on PROCEDURE does not trigger CREATE PROCEDURE warnings", func(t *testing.T) {
		sql := "GRANT USAGE ON PROCEDURE my_proc(VARCHAR) TO ROLE my_role"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "create procedure") ||
				strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("GRANT on PROCEDURE should not trigger CREATE PROCEDURE warnings, got: %s", w.Message)
			}
		}
	})

	t.Run("CREATE TEMPORARY PROCEDURE does not trigger procedure validation", func(t *testing.T) {
		sql := "CREATE TEMPORARY PROCEDURE my_proc() LANGUAGE SQL"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("CREATE TEMPORARY PROCEDURE should not trigger procedure warnings (regex doesn't match), got: %s", w.Message)
			}
		}
	})

	t.Run("CREATE OR ALTER PROCEDURE does not trigger procedure validation", func(t *testing.T) {
		sql := "CREATE OR ALTER PROCEDURE my_proc() LANGUAGE SQL"
		ranges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "missing mandatory") {
				t.Errorf("CREATE OR ALTER PROCEDURE should not trigger procedure warnings (regex doesn't match), got: %s", w.Message)
			}
		}
	})

	t.Run("Quoted procedure name still validates parameter types in ValidateDataTypes", func(t *testing.T) {
		// reCreateProcExt uses _identPath which includes quoted identifiers,
		// so parameter types are validated even for quoted procedure names.
		sql := `CREATE PROCEDURE "my proc"(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE with quoted procedure name, not found")
		}
	})
}

func TestValidateSnowflakePatterns_ProcedureReturnTypes(t *testing.T) {
	// Verify Snowflake-specific return types don't produce false positives from ValidateDataTypes.
	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "RETURNS VARIANT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS VARIANT LANGUAGE SQL AS $$ BEGIN RETURN PARSE_JSON('{}'); END; $$",
		},
		{
			name: "RETURNS OBJECT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS OBJECT LANGUAGE SQL AS $$ BEGIN RETURN OBJECT_CONSTRUCT(); END; $$",
		},
		{
			name: "RETURNS GEOGRAPHY",
			sql:  "CREATE PROCEDURE my_proc() RETURNS GEOGRAPHY LANGUAGE SQL AS $$ BEGIN RETURN NULL; END; $$",
		},
		{
			name: "RETURNS GEOMETRY",
			sql:  "CREATE PROCEDURE my_proc() RETURNS GEOMETRY LANGUAGE SQL AS $$ BEGIN RETURN NULL; END; $$",
		},
		{
			name: "RETURNS BOOLEAN",
			sql:  "CREATE PROCEDURE my_proc() RETURNS BOOLEAN LANGUAGE SQL AS $$ BEGIN RETURN TRUE; END; $$",
		},
		{
			name: "RETURNS FLOAT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS FLOAT LANGUAGE SQL AS $$ BEGIN RETURN 1.0; END; $$",
		},
		{
			name: "RETURNS BINARY",
			sql:  "CREATE PROCEDURE my_proc() RETURNS BINARY LANGUAGE SQL AS $$ BEGIN RETURN NULL; END; $$",
		},
		{
			name: "RETURNS DATE",
			sql:  "CREATE PROCEDURE my_proc() RETURNS DATE LANGUAGE SQL AS $$ BEGIN RETURN CURRENT_DATE(); END; $$",
		},
		{
			name: "RETURNS TIMESTAMP_NTZ",
			sql:  "CREATE PROCEDURE my_proc() RETURNS TIMESTAMP_NTZ LANGUAGE SQL AS $$ BEGIN RETURN CURRENT_TIMESTAMP(); END; $$",
		},
		{
			name: "RETURNS TIMESTAMP_TZ",
			sql:  "CREATE PROCEDURE my_proc() RETURNS TIMESTAMP_TZ LANGUAGE SQL AS $$ BEGIN RETURN CURRENT_TIMESTAMP(); END; $$",
		},
		{
			name: "RETURNS TIMESTAMP_LTZ",
			sql:  "CREATE PROCEDURE my_proc() RETURNS TIMESTAMP_LTZ LANGUAGE SQL AS $$ BEGIN RETURN CURRENT_TIMESTAMP(); END; $$",
		},
		{
			name: "RETURNS STRING",
			sql:  "CREATE PROCEDURE my_proc() RETURNS STRING LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
		},
		{
			name: "RETURNS ARRAY",
			sql:  "CREATE PROCEDURE my_proc() RETURNS ARRAY LANGUAGE SQL AS $$ BEGIN RETURN ARRAY_CONSTRUCT(); END; $$",
		},
		{
			name: "RETURNS NUMBER parameterized",
			sql:  "CREATE PROCEDURE my_proc() RETURNS NUMBER(10,2) LANGUAGE SQL AS $$ BEGIN RETURN 3.14; END; $$",
		},
		{
			name: "RETURNS INTEGER",
			sql:  "CREATE PROCEDURE my_proc() RETURNS INTEGER LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
		{
			name: "RETURNS INT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS INT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
		{
			name: "RETURNS DOUBLE",
			sql:  "CREATE PROCEDURE my_proc() RETURNS DOUBLE LANGUAGE SQL AS $$ BEGIN RETURN 1.0; END; $$",
		},
		{
			name: "RETURNS TEXT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS TEXT LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$",
		},
		{
			name: "RETURNS TIME",
			sql:  "CREATE PROCEDURE my_proc() RETURNS TIME LANGUAGE SQL AS $$ BEGIN RETURN CURRENT_TIME(); END; $$",
		},
		{
			name: "RETURNS TIMESTAMP",
			sql:  "CREATE PROCEDURE my_proc() RETURNS TIMESTAMP LANGUAGE SQL AS $$ BEGIN RETURN CURRENT_TIMESTAMP(); END; $$",
		},
		{
			name: "RETURNS DATETIME",
			sql:  "CREATE PROCEDURE my_proc() RETURNS DATETIME LANGUAGE SQL AS $$ BEGIN RETURN CURRENT_TIMESTAMP(); END; $$",
		},
		{
			name: "RETURNS CHAR",
			sql:  "CREATE PROCEDURE my_proc() RETURNS CHAR LANGUAGE SQL AS $$ BEGIN RETURN 'a'; END; $$",
		},
		{
			name: "RETURNS CHARACTER",
			sql:  "CREATE PROCEDURE my_proc() RETURNS CHARACTER LANGUAGE SQL AS $$ BEGIN RETURN 'a'; END; $$",
		},
		{
			name: "RETURNS CHAR parameterized",
			sql:  "CREATE PROCEDURE my_proc() RETURNS CHAR(10) LANGUAGE SQL AS $$ BEGIN RETURN 'hello'; END; $$",
		},
		{
			name: "RETURNS NUMERIC",
			sql:  "CREATE PROCEDURE my_proc() RETURNS NUMERIC LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
		{
			name: "RETURNS DECIMAL",
			sql:  "CREATE PROCEDURE my_proc() RETURNS DECIMAL LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
		{
			name: "RETURNS DECIMAL parameterized",
			sql:  "CREATE PROCEDURE my_proc() RETURNS DECIMAL(10,2) LANGUAGE SQL AS $$ BEGIN RETURN 3.14; END; $$",
		},
		{
			name: "RETURNS BIGINT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS BIGINT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
		{
			name: "RETURNS SMALLINT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS SMALLINT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
		{
			name: "RETURNS TINYINT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS TINYINT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
		{
			name: "RETURNS REAL",
			sql:  "CREATE PROCEDURE my_proc() RETURNS REAL LANGUAGE SQL AS $$ BEGIN RETURN 1.0; END; $$",
		},
		{
			name: "RETURNS VARBINARY",
			sql:  "CREATE PROCEDURE my_proc() RETURNS VARBINARY LANGUAGE SQL AS $$ BEGIN RETURN NULL; END; $$",
		},
		{
			name: "RETURNS BYTEINT",
			sql:  "CREATE PROCEDURE my_proc() RETURNS BYTEINT LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := GetStatementRanges(tt.sql)
			markers := ValidateSnowflakePatterns(tt.sql, ranges)
			markers = append(markers, ValidateDataTypes(tt.sql, ranges)...)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %s, got %d: %v", tt.name, len(warnings), warnings)
			}
		})
	}
}

func TestValidateDataTypes_ProcedureMarkerPrecision(t *testing.T) {
	t.Run("Invalid parameter type has precise column position", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning")
		}
		w := warnings[0]
		if w.StartLineNumber != 1 {
			t.Errorf("Expected StartLineNumber=1, got %d", w.StartLineNumber)
		}
		// "BADTYPE" starts at column 28 (after "CREATE PROCEDURE my_proc(a ")
		expectedCol := strings.Index(sql, "BADTYPE") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("BADTYPE") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADTYPE"), w.EndColumn)
		}
	})

	t.Run("Invalid return type has precise column position", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS BADRETURN LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid return type")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BADRETURN") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("BADRETURN") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADRETURN"), w.EndColumn)
		}
	})

	t.Run("Multiline procedure invalid param type has correct line number", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(\n  a INT,\n  b FAKETYPE\n) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning")
		}
		w := warnings[0]
		if w.StartLineNumber != 3 {
			t.Errorf("Expected StartLineNumber=3 for param on third line, got %d", w.StartLineNumber)
		}
	})

	t.Run("Procedure in second statement has correct offset", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning in second statement")
		}
		w := warnings[0]
		if w.StartLineNumber != 2 {
			t.Errorf("Expected StartLineNumber=2 for procedure in second statement, got %d", w.StartLineNumber)
		}
	})

	t.Run("Multiple invalid params produce distinct markers", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADONE, b BADTWO) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) < 2 {
			t.Fatalf("Expected at least 2 data type warnings, got %d", len(warnings))
		}
		// Each marker should have a distinct StartColumn
		if warnings[0].StartColumn == warnings[1].StartColumn {
			t.Errorf("Expected distinct column positions for two invalid params, both got col %d", warnings[0].StartColumn)
		}
	})

	t.Run("Single-token parameter (name only, no type) does not crash", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		// processColumnDef requires >= 2 tokens; single-token param should be silently skipped.
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(w.Message, "a") && strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				t.Errorf("Single-token parameter name should not be validated as a type, got: %s", w.Message)
			}
		}
	})

	t.Run("Quoted parameter type is not validated", func(t *testing.T) {
		sql := `CREATE PROCEDURE my_proc(a "BADTYPE") RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				t.Errorf("Quoted type should not be validated, got: %s", w.Message)
			}
		}
	})

	t.Run("Parameter DEFAULT with comma in string does not split incorrectly", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a VARCHAR DEFAULT 'x,y', b INT) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		// Both VARCHAR and INT are valid; no warnings expected.
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Parameter with block comment between name and type", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a /* comment */ BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after block comment, not found")
		}
	})

	t.Run("Parameter with line comment between name and type", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(\n  a -- my param\n  BADTYPE\n) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after line comment, not found")
		}
	})

	t.Run("Invalid return type on different line has correct line number", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc()\nRETURNS BADRETURN\nLANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid return type")
		}
		w := warnings[0]
		if w.StartLineNumber != 2 {
			t.Errorf("Expected StartLineNumber=2 for return type on second line, got %d", w.StartLineNumber)
		}
	})

	t.Run("Parameter with NOT NULL constraint validates type correctly", func(t *testing.T) {
		// NOT NULL after the type should not confuse type extraction;
		// processColumnDef takes the second token as the type name.
		sql := "CREATE PROCEDURE my_proc(a BADTYPE NOT NULL) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") &&
				strings.Contains(strings.ToLower(w.Message), "badtype") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE with NOT NULL constraint, not found")
		}
	})

	t.Run("Parameterized invalid return type has correct column span", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS BADTYPE(10) LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid parameterized return type")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BADTYPE") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		// EndColumn should span only "BADTYPE", not "(10)"
		if w.EndColumn != expectedCol+len("BADTYPE") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADTYPE"), w.EndColumn)
		}
	})

	t.Run("Multiple invalid params on different lines have correct line numbers", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(\n  a BADONE,\n  b BADTWO\n) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) < 2 {
			t.Fatalf("Expected at least 2 data type warnings, got %d", len(warnings))
		}

		foundLine2 := false
		foundLine3 := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badone") && w.StartLineNumber == 2 {
				foundLine2 = true
			}
			if strings.Contains(strings.ToLower(w.Message), "badtwo") && w.StartLineNumber == 3 {
				foundLine3 = true
			}
		}
		if !foundLine2 {
			t.Error("Expected BADONE warning on line 2, not found")
		}
		if !foundLine3 {
			t.Error("Expected BADTWO warning on line 3, not found")
		}
	})

	t.Run("Unclosed paren produces no data type markers at all", func(t *testing.T) {
		// extractBalancedBlockPat returns "" for unclosed parens, so
		// no parameter type markers are produced (graceful failure).
		sql := "CREATE PROCEDURE my_proc(a BADTYPE, b ALSOBAD RETURNS VARCHAR LANGUAGE SQL AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") ||
				strings.Contains(strings.ToLower(w.Message), "alsobad") {
				t.Errorf("Unclosed paren should produce no param type markers, got: %s", w.Message)
			}
		}
	})

	t.Run("Nested parens in DEFAULT do not break param type extraction", func(t *testing.T) {
		// extractBalancedBlockPat tracks paren depth, so COALESCE(NULL, 0)
		// must not prematurely close the parameter list. The second param
		// BADTYPE should still be validated.
		sql := "CREATE PROCEDURE my_proc(a NUMBER DEFAULT COALESCE(NULL, 0), b BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after nested-paren DEFAULT, not found")
		}
	})

	t.Run("Duplicate RETURNS with second type invalid is caught by ValidateDataTypes", func(t *testing.T) {
		// reReturnsType uses FindAllStringSubmatchIndex, so both RETURNS types
		// are independently validated. The first (VARCHAR) is valid; the second
		// (BADRET) must still produce a warning.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR RETURNS BADRET LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badret") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADRET in second RETURNS clause, not found")
		}
	})

	t.Run("Trailing comma in parameter list does not produce false type warnings", func(t *testing.T) {
		// parseColumnDefs splits by commas; the trailing comma produces an empty
		// segment which processColumnDef must silently skip (0 tokens).
		sql := "CREATE PROCEDURE my_proc(a INT, b VARCHAR,) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for trailing comma, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Closing paren in single-quoted DEFAULT does not truncate param list", func(t *testing.T) {
		// extractBalancedBlockPat respects single-quoted strings, so ')' inside
		// a DEFAULT string literal must not close the parameter list. The second
		// param BADTYPE should still be validated.
		sql := "CREATE PROCEDURE my_proc(a VARCHAR DEFAULT ')', b BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after paren-in-string DEFAULT, not found")
		}
	})

	t.Run("RETURNS TABLE does not produce unknown data type TABLE warning", func(t *testing.T) {
		// ValidateDataTypes filters out TABLE from RETURNS TABLE(...) to avoid
		// a false "Unknown data type 'TABLE'" warning.
		sql := "CREATE PROCEDURE my_proc() RETURNS TABLE(id INT, name VARCHAR) LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT 1, 'a'); END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "'table'") {
				t.Errorf("RETURNS TABLE should not produce unknown data type TABLE warning, got: %s", w.Message)
			}
		}
	})

	t.Run("RETURNS NULL ON NULL INPUT does not produce unknown data type NULL warning", func(t *testing.T) {
		// ValidateDataTypes filters out NULL from RETURNS NULL ON NULL INPUT to
		// avoid a false "Unknown data type 'NULL'" warning.
		sql := "CREATE PROCEDURE my_proc(a NUMBER) RETURNS VARCHAR LANGUAGE SQL RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "'null'") {
				t.Errorf("RETURNS NULL ON NULL INPUT should not produce unknown data type NULL warning, got: %s", w.Message)
			}
		}
	})

	t.Run("RETURNS inside dollar-quoted body is not flagged", func(t *testing.T) {
		// The tokenizer treats the $$...$$ body as a single DollarQuoted token,
		// so the "RETURNS" inside it is never seen as a keyword. (The old
		// regex-based validator scanned rawText and falsely flagged it.)
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ RETURNS BADTYPE $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				t.Errorf("RETURNS BADTYPE inside dollar-quoted body must not be flagged, got: %s", w.Message)
			}
		}
	})

	t.Run("cast shorthand inside dollar-quoted body is not flagged", func(t *testing.T) {
		// ::BADTYPE inside the $$...$$ body must not be flagged; the body is a
		// single token, not scanned for casts.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS $$ var x = y::BADTYPE; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				t.Errorf("::BADTYPE inside dollar-quoted body must not be flagged, got: %s", w.Message)
			}
		}
	})

	t.Run("RETURNS type on separate line has correct line number", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc()\nRETURNS BADRETURN\nLANGUAGE SQL\nAS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid return type on second line")
		}
		w := warnings[0]
		if w.StartLineNumber != 2 {
			t.Errorf("Expected StartLineNumber=2 for RETURNS on second line, got %d", w.StartLineNumber)
		}
		// "BADRETURN" starts at column 9 on line 2 (after "RETURNS ")
		if w.StartColumn != 9 {
			t.Errorf("Expected StartColumn=9, got %d", w.StartColumn)
		}
	})

	t.Run("ValidateDataTypes catches invalid params in both multi-statement procedures", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1(a BADONE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2(b BADTWO) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundBadOne := false
		foundBadTwo := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "badone") {
				foundBadOne = true
			}
			if strings.Contains(msg, "badtwo") {
				foundBadTwo = true
			}
		}
		if !foundBadOne {
			t.Error("Expected unknown data type warning for BADONE in first procedure, not found")
		}
		if !foundBadTwo {
			t.Error("Expected unknown data type warning for BADTWO in second procedure, not found")
		}
	})

	t.Run("ValidateDataTypes catches invalid return types in both multi-statement procedures", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS BADRET1 LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$;\nCREATE PROCEDURE p2() RETURNS BADRET2 LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundRet1 := false
		foundRet2 := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "badret1") {
				foundRet1 = true
			}
			if strings.Contains(msg, "badret2") {
				foundRet2 = true
			}
		}
		if !foundRet1 {
			t.Error("Expected unknown data type warning for BADRET1 in first procedure, not found")
		}
		if !foundRet2 {
			t.Error("Expected unknown data type warning for BADRET2 in second procedure, not found")
		}
	})

	t.Run("ValidateDataTypes produces zero markers for valid-only procedure", func(t *testing.T) {
		// Explicit check that no false positives are produced when all
		// parameter types and the return type are valid Snowflake types.
		sql := "CREATE PROCEDURE my_proc(a INT, b VARCHAR, c NUMBER(10,2), d BOOLEAN) RETURNS VARCHAR(255) LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for valid-only procedure, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("ValidateDataTypes produces zero markers for procedure with no parameters", func(t *testing.T) {
		// Empty parameter list must not produce any data type markers.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for no-param procedure, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("First procedure invalid type does not affect second procedure valid types", func(t *testing.T) {
		// ValidateDataTypes must process each statement independently;
		// BADTYPE in p1 must not leak warnings into p2.
		sql := "CREATE PROCEDURE p1(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2(a INT, b VARCHAR) RETURNS NUMBER LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		// Exactly one warning expected: BADTYPE in p1.
		typeCount := 0
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				typeCount++
				if !strings.Contains(strings.ToLower(w.Message), "badtype") {
					t.Errorf("Expected data type warning for BADTYPE only, got: %s", w.Message)
				}
				if w.StartLineNumber != 1 {
					t.Errorf("Expected BADTYPE warning on line 1, got line %d", w.StartLineNumber)
				}
			}
		}
		if typeCount != 1 {
			t.Errorf("Expected exactly 1 unknown data type warning, got %d", typeCount)
		}
	})

	t.Run("Schema-qualified 3-part name with invalid param has correct marker", func(t *testing.T) {
		// reCreateProcExt uses _identPath which supports up to 3-part names.
		// Verify that parameter type validation still works for 3-part names
		// and markers have correct positions.
		sql := "CREATE PROCEDURE mydb.myschema.my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for schema-qualified name with BADTYPE")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BADTYPE") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("BADTYPE") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADTYPE"), w.EndColumn)
		}
	})

	t.Run("Interspersed valid and invalid params all invalid caught", func(t *testing.T) {
		// Four parameters: valid, invalid, valid, invalid. Both invalid types
		// must be independently flagged with correct positions.
		sql := "CREATE PROCEDURE my_proc(a INT, b BADONE, c VARCHAR, d BADTWO) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundBadOne := false
		foundBadTwo := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "badone") {
				foundBadOne = true
			}
			if strings.Contains(msg, "badtwo") {
				foundBadTwo = true
			}
		}
		if !foundBadOne {
			t.Error("Expected unknown data type warning for BADONE, not found")
		}
		if !foundBadTwo {
			t.Error("Expected unknown data type warning for BADTWO, not found")
		}
	})

	t.Run("Quoted 3-part name with invalid param type still validated", func(t *testing.T) {
		// reCreateProcExt supports quoted identifiers in the path. Verify
		// parameter types are validated for fully-quoted 3-part names.
		sql := `CREATE PROCEDURE "MY_DB"."MY_SCHEMA"."MY_PROC"(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE with quoted 3-part name, not found")
		}
	})

	t.Run("Lowercase invalid param type produces marker with correct position", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a badtype) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for lowercase badtype")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "badtype") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("badtype") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("badtype"), w.EndColumn)
		}
	})

	t.Run("Lowercase valid params produce zero data type markers", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a int, b varchar, c float) RETURNS number LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for lowercase valid types, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Mixed-case invalid return type produces marker", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS BadReturn LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for mixed-case BadReturn")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BadReturn") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
	})
}
