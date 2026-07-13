package sqlgrammar

import "testing"

// Regression tests for issue #716 — false positives across several grammar rules.
// Each block reproduces a statement the grammar wrongly rejected before the fix.

// COPY INTO <table>: the generic copyOptions PURGE / FORCE arrive as lexer
// keywords, so a strict Identifier key match rejected them.
func TestIssue716_CopyIntoTablePurgeForce(t *testing.T) {
	assertValid(t, (*Validator).ParseCopyIntoTable,
		"COPY INTO t FROM (SELECT $1 FROM @s) PATTERN = '.*[.]csv' PURGE = TRUE",
		"COPY INTO t FROM @s FORCE = TRUE",
		"COPY INTO t FROM @s ON_ERROR = CONTINUE PURGE = TRUE FORCE = FALSE",
	)
	// A bare option name with no `= <value>` is still not a valid copyOption.
	assertInvalid(t, (*Validator).ParseCopyIntoTable,
		"COPY INTO t FROM @s PURGE",
	)
}

// COPY INTO <location>: HEADER is modeled as a bare word from the docs skeleton,
// but the common `HEADER = TRUE|FALSE` form failed at the `=`.
func TestIssue716_CopyIntoLocationHeader(t *testing.T) {
	assertValid(t, (*Validator).ParseCopyIntoLocation,
		"COPY INTO @s FROM t HEADER = TRUE",
		"COPY INTO @s FROM t HEADER = FALSE",
		"COPY INTO @s FROM t HEADER", // bare form still accepted
		// The destination now shares parseStageRef, so a `/path` suffix on the
		// stage (the same handling COPY INTO <table> gained) is accepted here too.
		"COPY INTO @s/archive/ FROM t",
	)
}

// EXECUTE IMMEDIATE $$ … $$ — the dollar-quoted body is the docs' primary example.
func TestIssue716_ExecuteImmediateDollarQuoted(t *testing.T) {
	assertValid(t, (*Validator).ParseExecuteImmediate,
		"EXECUTE IMMEDIATE $$ SELECT 1 $$",
		"EXECUTE IMMEDIATE $$ SELECT 1 $$ USING (a)",
	)
}

// CREATE EXTERNAL TABLE LOCATION = @stage/path/ — the naive stageValue had no
// `/path` suffix handling.
func TestIssue716_ExternalTableLocationPath(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateExternalTable,
		"CREATE EXTERNAL TABLE t (c INT) LOCATION = @mystage/path/ FILE_FORMAT = (TYPE = CSV)",
		"CREATE EXTERNAL TABLE t (c INT) WITH LOCATION = @mystage/sub/dir/",
	)
}

// CLUSTER BY LINEAR(…) — accepted in ALTER TABLE and emitted by SHOW TABLES, but
// rejected across the CREATE rules.
func TestIssue716_ClusterByLinear(t *testing.T) {
	assertValid(t, (*Validator).ParseCreateTable,
		"CREATE TABLE t (i INT) CLUSTER BY LINEAR(a, b)",
		"CREATE TABLE t (i INT) CLUSTER BY (a, b)", // non-LINEAR still valid
	)
	assertValid(t, (*Validator).ParseCreateDynamicTable,
		"CREATE DYNAMIC TABLE t TARGET_LAG = '1 minute' WAREHOUSE = wh CLUSTER BY LINEAR(a) AS SELECT * FROM s",
	)
	assertValid(t, (*Validator).ParseCreateMaterializedView,
		"CREATE MATERIALIZED VIEW v CLUSTER BY LINEAR(a) AS SELECT * FROM t",
	)
}

// The ALTER clustering actions must accept CLUSTER BY LINEAR(…) too — SHOW …
// round-trips DDL in that form. Before the fix these still used the pre-helper
// `CLUSTER BY <parens>` pattern and flagged LINEAR as a syntax error.
func TestIssue716_AlterClusterByLinear(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterIcebergTable,
		"ALTER ICEBERG TABLE t CLUSTER BY LINEAR(a, b)",
		"ALTER ICEBERG TABLE t CLUSTER BY (a, b)", // non-LINEAR still valid
	)
	assertValid(t, (*Validator).ParseAlterMaterializedView,
		"ALTER MATERIALIZED VIEW v CLUSTER BY LINEAR(a)",
		"ALTER MATERIALIZED VIEW v CLUSTER BY (a)", // non-LINEAR still valid
	)
}
