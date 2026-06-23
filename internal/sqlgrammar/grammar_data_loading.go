package sqlgrammar

// Data loading & unloading / file staging — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseCopyFiles validates the Snowflake `COPY FILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/copy-files
func (v *Validator) ParseCopyFiles() bool {
	return true
}

// ParseCopyIntoLocation validates the Snowflake `COPY INTO <location>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/copy-into-location
func (v *Validator) ParseCopyIntoLocation() bool {
	return true
}

// ParseCopyIntoTable validates the Snowflake `COPY INTO <table>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/copy-into-table
func (v *Validator) ParseCopyIntoTable() bool {
	return true
}

// ParseGet validates the Snowflake `GET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/get
func (v *Validator) ParseGet() bool {
	return true
}

// ParseList validates the Snowflake `LIST` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/list
func (v *Validator) ParseList() bool {
	return true
}

// ParsePut validates the Snowflake `PUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/put
func (v *Validator) ParsePut() bool {
	return true
}

// ParseRemove validates the Snowflake `REMOVE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/remove
func (v *Validator) ParseRemove() bool {
	return true
}
