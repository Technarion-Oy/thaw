package sqlgrammar

// Data Manipulation Language (DML) — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseDelete validates the Snowflake `DELETE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/delete
//
// Syntax:
//
//	DELETE FROM <table_name>
//	            [ USING <additional_table_or_query> [, <additional_table_or_query> ] ]
//	            [ WHERE <condition> ]
func (v *Validator) ParseDelete() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("DELETE") },
		func() bool { return v.MatchWord("FROM") },
		v.parseIdentPath,
		// [ USING <tables> ] [ WHERE <condition> ] — free-form tail.
		func() bool { return v.consumeRest() },
	)
}

// ParseInsert validates the Snowflake `INSERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/insert
//
// Syntax:
//
//	INSERT [ OVERWRITE ] INTO <target_table> [ ( <target_col_name> [ , ... ] ) ]
//	       {
//	           VALUES ( { <value> | DEFAULT | NULL } [ , ... ] ) [ , ( ... ) ]
//	         | <query>
//	       }
func (v *Validator) ParseInsert() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("INSERT") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("OVERWRITE") }) },
		func() bool { return v.MatchWord("INTO") },
		v.parseIdentPath,
		// [ ( <target_col_name> [ , ... ] ) ] { VALUES (...) [, (...)] | <query> }.
		// The body is free-form (a column list, VALUES, a SELECT, or a
		// parenthesized subquery), so require at least one more token and consume
		// the rest — handles INSERT INTO t (SELECT …) where the leading paren is
		// the subquery itself, not a target column list.
		func() bool {
			if v.AtEnd() {
				v.expect("VALUES or query")
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseInsertMultiTable validates the Snowflake `INSERT (multi-table)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/insert-multi-table
//
// Syntax:
//
//	-- Unconditional multi-table insert
//	INSERT [ OVERWRITE ] ALL
//	  intoClause [ ... ]
//	<subquery>
//
//	-- Conditional multi-table insert
//	INSERT [ OVERWRITE ] { FIRST | ALL }
//	  { WHEN <condition> THEN intoClause [ ... ] }
//	  [ ... ]
//	  [ ELSE intoClause ]
//	<subquery>
//
//	Where:
//
//	intoClause ::=
//	  INTO <target_table> [ ( <target_col_name> [ , ... ] ) ] [ VALUES ( { <source_col_name> | DEFAULT | NULL } [ , ... ] ) ]
func (v *Validator) ParseInsertMultiTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("INSERT") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("OVERWRITE") }) },
		// { ALL | FIRST | ALL } — require the multi-table selector.
		v.wordsValue("ALL", "FIRST"),
		// The WHEN/THEN/ELSE intoClauses and the trailing <subquery>: free-form.
		func() bool {
			if v.AtEnd() {
				v.expect("intoClause or subquery")
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseMerge validates the Snowflake `MERGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/merge
//
// Syntax:
//
//	MERGE INTO <target_table>
//	  USING <source>
//	  ON <join_expr>
//	  { matchedClause | notMatchedClause } [ ... ]
//
//	Where:
//
//	matchedClause ::=
//	  WHEN MATCHED
//	    [ AND <case_predicate> ]
//	    THEN { UPDATE { ALL BY NAME | SET <col_name> = <expr> [ , <col_name> = <expr> ... ] } | DELETE } [ ... ]
//
//	notMatchedClause ::=
//	   WHEN NOT MATCHED
//	     [ AND <case_predicate> ]
//	     THEN INSERT { ALL BY NAME | [ ( <col_name> [ , ... ] ) ] VALUES ( <expr> [ , ... ] ) }
func (v *Validator) ParseMerge() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("MERGE") },
		func() bool { return v.MatchWord("INTO") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("USING") },
		// <source> may be a table name or a parenthesized subquery.
		func() bool {
			return v.Choice(
				func() bool { return v.consumeBalancedParens() },
				v.parseIdentPath,
			)
		},
		// [ [AS] <alias> ] ON <join_expr> { matched | notMatched } [ ... ] —
		// free-form, but require the ON keyword somewhere ahead.
		func() bool {
			for !v.AtEnd() {
				if v.MatchWord("ON") {
					return v.consumeRest()
				}
				v.advance()
			}
			v.expect("ON")
			return false
		},
	)
}

// ParseSelect validates the Snowflake `SELECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/select
//
// Syntax:
//
//	[ ... ]
//	SELECT [ { ALL | DISTINCT } ]
//	       [ TOP <n> ]
//	       [{<object_name>|<alias>}.].*
//
//	       [ ILIKE '<pattern>' ]
//
//	       [ EXCLUDE
//	         {
//	           <col_name> | ( <col_name>, <col_name>, ... )
//	         }
//	       ]
//
//	       [ REPLACE
//	         {
//	           ( <expr> AS <col_name> [ , <expr> AS <col_name>, ... ] )
//	         }
//	       ]
//
//	       [ RENAME
//	         {
//	           <col_name> AS <col_alias>
//	           | ( <col_name> AS <col_alias>, <col_name> AS <col_alias>, ... )
//	         }
//	       ]
//
//	[ ... ]
//	SELECT [ { ALL | DISTINCT } ]
//	       [ TOP <n> ]
//	       {
//	         [{<object_name>|<alias>}.]<col_name>
//	         | [{<object_name>|<alias>}.]$<col_position>
//	         | <expr>
//	       }
//	       [ [ AS ] <col_alias> ]
//	       [ , ... ]
//	[ ... ]
func (v *Validator) ParseSelect() bool {
	return v.Sequence(
		// Require the SELECT keyword; the projection list, FROM/WHERE/GROUP BY/
		// etc. clause soup is free-form.
		func() bool { return v.MatchWord("SELECT") },
		func() bool { return v.consumeRest() },
	)
}

// ParseTruncateTable validates the Snowflake `TRUNCATE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/truncate-table
//
// Syntax:
//
//	TRUNCATE [ TABLE ] [ IF EXISTS ] <name>
//
//	TRUNCATE [ TABLE ] [ IF EXISTS ] ERROR_TABLE( <base_table_name> )
func (v *Validator) ParseTruncateTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("TRUNCATE") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("TABLE") }) },
		v.ifExists,
		// <name>  |  ERROR_TABLE( <base_table_name> )
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("ERROR_TABLE") },
						func() bool { return v.consumeBalancedParens() },
					)
				},
				v.parseIdentPath,
			)
		},
	)
}

// ParseUpdate validates the Snowflake `UPDATE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/update
//
// Syntax:
//
//	UPDATE <target_table>
//	       SET <col_name> = <value> [ , <col_name> = <value> , ... ]
//	        [ FROM <additional_tables> ]
//	        [ WHERE <condition> ]
func (v *Validator) ParseUpdate() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("UPDATE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("SET") },
		// SET <col> = <value> [, ...] [ FROM ... ] [ WHERE ... ] — free-form, but
		// require at least one token (the first assignment) after SET.
		func() bool {
			if v.AtEnd() {
				v.expect("column assignment")
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseTruncateMaterializedView validates the Snowflake `TRUNCATE MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/truncate-materialized-view
//
// Syntax:
//
//	TRUNCATE MATERIALIZED VIEW <name>
func (v *Validator) ParseTruncateMaterializedView() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("TRUNCATE") },
		func() bool { return v.MatchWord("MATERIALIZED") },
		func() bool { return v.MatchWord("VIEW") },
		v.parseIdentPath,
	)
}
