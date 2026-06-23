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
	return true
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
	return true
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
	return true
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
	return true
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
	return true
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
	return true
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
	return true
}

// ParseTruncateMaterializedView validates the Snowflake `TRUNCATE MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/truncate-materialized-view
//
// Syntax:
//
//	TRUNCATE MATERIALIZED VIEW <name>
func (v *Validator) ParseTruncateMaterializedView() bool {
	return true
}
