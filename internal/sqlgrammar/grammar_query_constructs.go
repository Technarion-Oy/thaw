package sqlgrammar

// Query syntax constructs (SELECT sub-clauses) — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseWith validates the Snowflake `WITH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/with
//
// Syntax:
//
//	[ WITH
//	       <cte_name1> [ ( <cte_column_list> ) ] AS ( SELECT ...  )
//	   [ , <cte_name2> [ ( <cte_column_list> ) ] AS ( SELECT ...  ) ]
//	   [ , <cte_nameN> [ ( <cte_column_list> ) ] AS ( SELECT ...  ) ]
//	]
//	SELECT ...
//
//	[ WITH [ RECURSIVE ]
//	       <cte_name1> ( <cte_column_list> ) AS ( anchorClause UNION ALL recursiveClause )
//	   [ , <cte_name2> ( <cte_column_list> ) AS ( anchorClause UNION ALL recursiveClause ) ]
//	   [ , <cte_nameN> ( <cte_column_list> ) AS ( anchorClause UNION ALL recursiveClause ) ]
//	]
//	SELECT ...
//
//	anchorClause ::=
//	    SELECT <anchor_column_list> FROM ...
//
//	recursiveClause ::=
//	    SELECT <recursive_column_list> FROM ... [ JOIN ... ]
func (v *Validator) ParseWith() bool {
	return true
}

// ParseTopN validates the Snowflake `TOP_N` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/top_n
//
// Syntax:
//
//	SELECT
//	  [ TOP <n> ]
//	    ...
//	FROM ...
//	[ ORDER BY ... ]
//	[ ... ]
func (v *Validator) ParseTopN() bool {
	return true
}

// ParseInto validates the Snowflake `INTO` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/into
//
// Syntax:
//
//	SELECT <expression1>
//	   [ , <expression2> ]
//	   [ , <expressionN> ]
//	[ INTO :<variable1> ]
//	   [ , :<variable2> ]
//	   [ , :<variableN> ]
//	FROM ...
//	WHERE ...
//	[ ... ]
func (v *Validator) ParseInto() bool {
	return true
}

// ParseFrom validates the Snowflake `FROM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/from
//
// Syntax:
//
//	SELECT ...
//	FROM objectReference [ JOIN objectReference [ ... ] ]
//	[ ... ]
//
//	objectReference ::=
//	   {
//	      [<namespace>.]<object_name>
//	           [ AT | BEFORE ( <object_state> ) ]
//	           [ CHANGES ( <change_tracking_type> ) ]
//	           [ MATCH_RECOGNIZE ]
//	           [ PIVOT | UNPIVOT ]
//	           [ [ AS ] <alias_name> ]
//	           [ SAMPLE ]
//	      | <table_function>
//	           [ PIVOT | UNPIVOT ]
//	           [ [ AS ] <alias_name> ]
//	           [ SAMPLE ]
//	      | ( VALUES (...) )
//	           [ SAMPLE ]
//	      | [ LATERAL ] ( <subquery> )
//	           [ [ AS ] <alias_name> ]
//	      | @[<namespace>.]<stage_name>[/<path>]
//	           [ ( FILE_FORMAT => <format_name>, PATTERN => '<regex_pattern>' ) ]
//	           [ [ AS ] <alias_name> ]
//	      | DIRECTORY( @<stage_name> )
//	      | SEMANTIC_VIEW( ... )
//	      | ERROR_TABLE( <base_table_name> )
//	      | DYNAMIC_TABLE_REFRESH_BOUNDARY( <object_name> )
//	           [ AT | BEFORE ( <object_state> ) ]
//	           [ CHANGES ( <change_tracking_type> ) ]
//	           [ [ AS ] <alias_name> ]
//	   }
func (v *Validator) ParseFrom() bool {
	return true
}

// ParseAtBefore validates the Snowflake `AT_BEFORE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/at-before
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	  { AT | BEFORE }
//	  (
//	    { TIMESTAMP => <timestamp> |
//	      OFFSET => <time_difference> |
//	      STATEMENT => <id> |
//	      STREAM => '<name>' }
//	  )
//	[ ... ]
func (v *Validator) ParseAtBefore() bool {
	return true
}

// ParseChanges validates the Snowflake `CHANGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/changes
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	   CHANGES ( INFORMATION => { DEFAULT | APPEND_ONLY } )
//	   AT ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> | STREAM => '<name>' } ) | BEFORE ( STATEMENT => <id> )
//	   [ END( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	[ ... ]
func (v *Validator) ParseChanges() bool {
	return true
}

// ParseConnectBy validates the Snowflake `CONNECT_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/connect-by
//
// Syntax:
//
//	SELECT <column_list> [ , <level_expression> ]
//	  FROM <data_source>
//	    START WITH <predicate>
//	    CONNECT BY [ PRIOR ] <col1_identifier> = [ PRIOR ] <col2_identifier>
//	           [ , [ PRIOR ] <col3_identifier> = [ PRIOR ] <col4_identifier> ]
//	           ...
//	  ...
func (v *Validator) ParseConnectBy() bool {
	return true
}

// ParseJoin validates the Snowflake `JOIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/join
//
// Syntax:
//
//	SELECT ...
//	FROM <object_ref1> [
//	                     {
//	                       INNER
//	                       | { LEFT | RIGHT | FULL } [ OUTER ]
//	                     }
//	                     [ DIRECTED ]
//	                   ]
//	                   JOIN <object_ref2>
//	  [ ON <condition> ]
//	[ ... ]
//
//	SELECT *
//	FROM <object_ref1> [
//	                     {
//	                       INNER
//	                       | { LEFT | RIGHT | FULL } [ OUTER ]
//	                     }
//	                     [ DIRECTED ]
//	                   ]
//	                   JOIN <object_ref2>
//	  [ USING( <column_list> ) ]
//	[ ... ]
//
//	SELECT ...
//	FROM <object_ref1> [
//	                     {
//	                       NATURAL [
//	                                 {
//	                                   INNER
//	                                   | { LEFT | RIGHT | FULL } [ OUTER ]
//	                                 }
//	                                 [ DIRECTED ]
//	                               }
//	                       | CROSS  [ DIRECTED ]
//	                     }
//	                   ]
//	                   JOIN <object_ref2>
//	[ ... ]
func (v *Validator) ParseJoin() bool {
	return true
}

// ParseAsofJoin validates the Snowflake `ASOF_JOIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/asof-join
//
// Syntax:
//
//	FROM <left_table> ASOF JOIN <right_table>
//	  MATCH_CONDITION ( <left_table.timecol> <comparison_operator> <right_table.timecol> )
//	  [ ON <table.col> = <table.col> [ AND ... ] | USING ( <column_list> ) ]
func (v *Validator) ParseAsofJoin() bool {
	return true
}

// ParseLateral validates the Snowflake `LATERAL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/join-lateral
//
// Syntax:
//
//	SELECT ...
//	  FROM <left_hand_table_expression>, LATERAL ( <inline_view> )
//	...
func (v *Validator) ParseLateral() bool {
	return true
}

// ParseMatchRecognize validates the Snowflake `MATCH_RECOGNIZE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/match_recognize
//
// Syntax:
//
//	MATCH_RECOGNIZE (
//	    [ PARTITION BY <expr> [, ... ] ]
//	    [ ORDER BY <expr> [, ... ] ]
//	    [ MEASURES <expr> [AS] <alias> [, ... ] ]
//	    [ ONE ROW PER MATCH |
//	      ALL ROWS PER MATCH [ { SHOW EMPTY MATCHES | OMIT EMPTY MATCHES | WITH UNMATCHED ROWS } ]
//	      ]
//	    [ AFTER MATCH SKIP
//	          {
//	          PAST LAST ROW   |
//	          TO NEXT ROW   |
//	          TO [ { FIRST | LAST} ] <symbol>
//	          }
//	      ]
//	    PATTERN ( <pattern> )
//	    DEFINE <symbol> AS <expr> [, ... ]
//	)
func (v *Validator) ParseMatchRecognize() bool {
	return true
}

// ParsePivot validates the Snowflake `PIVOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/pivot
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	   PIVOT ( <aggregate_function> ( <pivot_column> ) [ [ AS ] <alias> ]
//	            FOR <value_column> IN (
//	              <pivot_value_1> [ [ AS ] <alias> ] [ , <pivot_value_2> [ [ AS ] <alias> ] ... ]
//	              | ANY [ ORDER BY ... ]
//	              | <subquery>
//	            )
//	            [ DEFAULT ON NULL (<value>) ]
//	         )
//
//	[ ... ]
func (v *Validator) ParsePivot() bool {
	return true
}

// ParseUnpivot validates the Snowflake `UNPIVOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/unpivot
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	    UNPIVOT [ { INCLUDE | EXCLUDE } NULLS ]
//	      ( <value_column>
//	        FOR <name_column> IN (
//	          <col> [ [ AS ] <col_alias> ],
//	          ...
//	        )
//	      )
//
//	[ ... ]
func (v *Validator) ParseUnpivot() bool {
	return true
}

// ParseValues validates the Snowflake `VALUES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/values
//
// Syntax:
//
//	SELECT ...
//	FROM ( VALUES ( <expr> [ , <expr> [ , ... ] ] ) [ , ( ... ) ] )
//	  [ [ AS ] <table_alias> [ ( <column_alias> [ , ... ] ) ] ]
//	[ ... ]
func (v *Validator) ParseValues() bool {
	return true
}

// ParseSample validates the Snowflake `SAMPLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/sample
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	  { SAMPLE | TABLESAMPLE } [ samplingMethod ]
//	[ ... ]
//
//	samplingMethod ::= { { BERNOULLI | ROW } ( { <probability> | <num> ROWS } ) |
//	                     { SYSTEM | BLOCK } ( <probability> ) [ { REPEATABLE | SEED } ( <seed> ) ] }
func (v *Validator) ParseSample() bool {
	return true
}

// ParseResample validates the Snowflake `RESAMPLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/resample
//
// Syntax:
//
//	FROM <object_reference> [ [ AS ] <alias_name> ]
//	  RESAMPLE(
//	    USING <time_series_column>
//	    INCREMENT BY <time_series_constant>
//	    [ PARTITION BY <partition_column> [ , ... ] ]
//	    [ METADATA_COLUMNS
//	        { IS_GENERATED() | BUCKET_START() } [ [ AS ] <alias_name> ] [ , ... ] ]
//	    )
func (v *Validator) ParseResample() bool {
	return true
}

// ParseSemanticView validates the Snowflake `SEMANTIC_VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/semantic_view
//
// Syntax:
//
//	SEMANTIC_VIEW(
//	  [<namespace>.]<semantic_view_name>
//	  [
//	    {
//	      METRICS <metric_expr> [ [ AS ] <alias> ] [ , ... ] |
//	      FACTS <fact_expr>  [ , ... ]
//	    }
//	  ]
//	  [ DIMENSIONS <dimension_expr>  [ [ AS ] <alias> ] [ , ... ] ]
//	  [ WHERE <predicate> ]
//	)
func (v *Validator) ParseSemanticView() bool {
	return true
}

// ParseWhere validates the Snowflake `WHERE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/where
//
// Syntax:
//
//	...
//	WHERE <predicate>
//	[ ... ]
func (v *Validator) ParseWhere() bool {
	return true
}

// ParseGroupBy validates the Snowflake `GROUP_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by
//
// Syntax:
//
//	SELECT ...
//	  FROM ...
//	  [ ... ]
//	  GROUP BY groupItem [ , groupItem [ , ... ] ]
//	  [ ... ]
//
//	SELECT ...
//	  FROM ...
//	  [ ... ]
//	  GROUP BY ALL
//	  [ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
func (v *Validator) ParseGroupBy() bool {
	return true
}

// ParseGroupByCube validates the Snowflake `GROUP_BY_CUBE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-cube
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ... ]
//	GROUP BY [ groupItem [ , groupItem [ , ... ] ] , ] CUBE ( groupItem [ , groupItem [ , ... ] ] )
//	[ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
func (v *Validator) ParseGroupByCube() bool {
	return true
}

// ParseGroupByGroupingSets validates the Snowflake `GROUP_BY_GROUPING_SETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-grouping-sets
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ... ]
//	GROUP BY [ groupItem [ , groupItem [ , ... ] ] , ] GROUPING SETS ( groupSet [ , groupSet [ , ... ] ] )
//	[ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
//
//	groupSet ::= groupItem | ( groupItem [ , groupItem [ , ... ] ] )
func (v *Validator) ParseGroupByGroupingSets() bool {
	return true
}

// ParseGroupByRollup validates the Snowflake `GROUP_BY_ROLLUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-rollup
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ... ]
//	GROUP BY [ groupItem [ , groupItem [ , ... ] ] , ] ROLLUP ( groupItem [ , groupItem [ , ... ] ] )
//	[ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
func (v *Validator) ParseGroupByRollup() bool {
	return true
}

// ParseHaving validates the Snowflake `HAVING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/having
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	GROUP BY ...
//	HAVING <predicate>
//	[ ... ]
func (v *Validator) ParseHaving() bool {
	return true
}

// ParseQualify validates the Snowflake `QUALIFY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/qualify
//
// Syntax:
//
//	QUALIFY <predicate>
//
//	SELECT <column_list>
//	  FROM <data_source>
//	  [GROUP BY ...]
//	  [HAVING ...]
//	  QUALIFY <predicate>
//	  [ ... ]
func (v *Validator) ParseQualify() bool {
	return true
}

// ParseOrderBy validates the Snowflake `ORDER_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/order-by
//
// Syntax:
//
//	SELECT ...
//	  FROM ...
//	  ORDER BY orderItem [ , orderItem , ... ]
//	  [ ... ]
//
//	orderItem ::= { <column_alias> | <position> | <expr> } [ { ASC | DESC } ] [ NULLS { FIRST | LAST } ]
//
//	SELECT ...
//	  FROM ...
//	  ORDER BY ALL [ { ASC | DESC } ] [ NULLS { FIRST | LAST } ]
//	  [ ... ]
func (v *Validator) ParseOrderBy() bool {
	return true
}

// ParseLimitFetch validates the Snowflake `LIMIT_FETCH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/limit
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ORDER BY ... ]
//	LIMIT <count> [ OFFSET <start> ]
//	[ ... ]
//
//	SELECT ...
//	FROM ...
//	[ ORDER BY ... ]
//	[ OFFSET <start> ] [ { ROW | ROWS } ] FETCH [ { FIRST | NEXT } ] <count> [ { ROW | ROWS } ] [ ONLY ]
//	[ ... ]
func (v *Validator) ParseLimitFetch() bool {
	return true
}

// ParseForUpdate validates the Snowflake `FOR_UPDATE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/for-update
//
// Syntax:
//
//	SELECT ...
//	  FROM ...
//	  [ ... ]
//	  FOR UPDATE [ NOWAIT | WAIT <wait_time> ]
func (v *Validator) ParseForUpdate() bool {
	return true
}
