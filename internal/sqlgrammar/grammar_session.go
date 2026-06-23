package sqlgrammar

// Sessions & context (USE, SET) — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseSet validates the Snowflake `SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/set
func (v *Validator) ParseSet() bool {
	return true
}

// ParseUnset validates the Snowflake `UNSET` command (drops a session variable).
// Reference: https://docs.snowflake.com/en/sql-reference/sql/unset
func (v *Validator) ParseUnset() bool {
	return true
}

// ParseUse validates the Snowflake `USE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use
func (v *Validator) ParseUse() bool {
	return true
}

// ParseUseDatabase validates the Snowflake `USE DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-database
func (v *Validator) ParseUseDatabase() bool {
	return true
}

// ParseUseRole validates the Snowflake `USE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-role
func (v *Validator) ParseUseRole() bool {
	return true
}

// ParseUseSchema validates the Snowflake `USE SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-schema
func (v *Validator) ParseUseSchema() bool {
	return true
}

// ParseUseSecondaryRoles validates the Snowflake `USE SECONDARY ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-secondary-roles
func (v *Validator) ParseUseSecondaryRoles() bool {
	return true
}

// ParseUseWarehouse validates the Snowflake `USE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-warehouse
func (v *Validator) ParseUseWarehouse() bool {
	return true
}
