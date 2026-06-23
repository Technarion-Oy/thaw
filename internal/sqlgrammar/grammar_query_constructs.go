package sqlgrammar

// Query syntax constructs (SELECT sub-clauses) — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseWith validates the Snowflake `WITH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/with
func (v *Validator) ParseWith() bool {
	return true
}

// ParseTopN validates the Snowflake `TOP_N` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/top_n
func (v *Validator) ParseTopN() bool {
	return true
}

// ParseInto validates the Snowflake `INTO` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/into
func (v *Validator) ParseInto() bool {
	return true
}

// ParseFrom validates the Snowflake `FROM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/from
func (v *Validator) ParseFrom() bool {
	return true
}

// ParseAtBefore validates the Snowflake `AT_BEFORE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/at-before
func (v *Validator) ParseAtBefore() bool {
	return true
}

// ParseChanges validates the Snowflake `CHANGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/changes
func (v *Validator) ParseChanges() bool {
	return true
}

// ParseConnectBy validates the Snowflake `CONNECT_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/connect-by
func (v *Validator) ParseConnectBy() bool {
	return true
}

// ParseJoin validates the Snowflake `JOIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/join
func (v *Validator) ParseJoin() bool {
	return true
}

// ParseAsofJoin validates the Snowflake `ASOF_JOIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/asof-join
func (v *Validator) ParseAsofJoin() bool {
	return true
}

// ParseLateral validates the Snowflake `LATERAL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/join-lateral
func (v *Validator) ParseLateral() bool {
	return true
}

// ParseMatchRecognize validates the Snowflake `MATCH_RECOGNIZE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/match_recognize
func (v *Validator) ParseMatchRecognize() bool {
	return true
}

// ParsePivot validates the Snowflake `PIVOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/pivot
func (v *Validator) ParsePivot() bool {
	return true
}

// ParseUnpivot validates the Snowflake `UNPIVOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/unpivot
func (v *Validator) ParseUnpivot() bool {
	return true
}

// ParseValues validates the Snowflake `VALUES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/values
func (v *Validator) ParseValues() bool {
	return true
}

// ParseSample validates the Snowflake `SAMPLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/sample
func (v *Validator) ParseSample() bool {
	return true
}

// ParseResample validates the Snowflake `RESAMPLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/resample
func (v *Validator) ParseResample() bool {
	return true
}

// ParseSemanticView validates the Snowflake `SEMANTIC_VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/semantic_view
func (v *Validator) ParseSemanticView() bool {
	return true
}

// ParseWhere validates the Snowflake `WHERE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/where
func (v *Validator) ParseWhere() bool {
	return true
}

// ParseGroupBy validates the Snowflake `GROUP_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by
func (v *Validator) ParseGroupBy() bool {
	return true
}

// ParseGroupByCube validates the Snowflake `GROUP_BY_CUBE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-cube
func (v *Validator) ParseGroupByCube() bool {
	return true
}

// ParseGroupByGroupingSets validates the Snowflake `GROUP_BY_GROUPING_SETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-grouping-sets
func (v *Validator) ParseGroupByGroupingSets() bool {
	return true
}

// ParseGroupByRollup validates the Snowflake `GROUP_BY_ROLLUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-rollup
func (v *Validator) ParseGroupByRollup() bool {
	return true
}

// ParseHaving validates the Snowflake `HAVING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/having
func (v *Validator) ParseHaving() bool {
	return true
}

// ParseQualify validates the Snowflake `QUALIFY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/qualify
func (v *Validator) ParseQualify() bool {
	return true
}

// ParseOrderBy validates the Snowflake `ORDER_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/order-by
func (v *Validator) ParseOrderBy() bool {
	return true
}

// ParseLimitFetch validates the Snowflake `LIMIT_FETCH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/limit
func (v *Validator) ParseLimitFetch() bool {
	return true
}

// ParseForUpdate validates the Snowflake `FOR_UPDATE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/for-update
func (v *Validator) ParseForUpdate() bool {
	return true
}
