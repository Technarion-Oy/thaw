package sqlgrammar

// Data Manipulation Language (DML) — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseDelete validates the Snowflake `DELETE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/delete
func (v *Validator) ParseDelete() bool {
	return true
}

// ParseInsert validates the Snowflake `INSERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/insert
func (v *Validator) ParseInsert() bool {
	return true
}

// ParseInsertMultiTable validates the Snowflake `INSERT (multi-table)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/insert-multi-table
func (v *Validator) ParseInsertMultiTable() bool {
	return true
}

// ParseMerge validates the Snowflake `MERGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/merge
func (v *Validator) ParseMerge() bool {
	return true
}

// ParseSelect validates the Snowflake `SELECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/select
func (v *Validator) ParseSelect() bool {
	return true
}

// ParseTruncateTable validates the Snowflake `TRUNCATE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/truncate-table
func (v *Validator) ParseTruncateTable() bool {
	return true
}

// ParseUpdate validates the Snowflake `UPDATE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/update
func (v *Validator) ParseUpdate() bool {
	return true
}
