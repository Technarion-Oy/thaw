package sqlgrammar

// UNDROP commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseUndropDatabase validates the Snowflake `UNDROP DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-database
func (v *Validator) ParseUndropDatabase() bool {
	return true
}

// ParseUndropDynamicTable validates the Snowflake `UNDROP DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-dynamic-table
func (v *Validator) ParseUndropDynamicTable() bool {
	return true
}

// ParseUndropExternalVolume validates the Snowflake `UNDROP EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-external-volume
func (v *Validator) ParseUndropExternalVolume() bool {
	return true
}

// ParseUndropIcebergTable validates the Snowflake `UNDROP ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-iceberg-table
func (v *Validator) ParseUndropIcebergTable() bool {
	return true
}

// ParseUndropNotebook validates the Snowflake `UNDROP NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-notebook
func (v *Validator) ParseUndropNotebook() bool {
	return true
}

// ParseUndropSchema validates the Snowflake `UNDROP SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-schema
func (v *Validator) ParseUndropSchema() bool {
	return true
}

// ParseUndropTable validates the Snowflake `UNDROP TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-table
func (v *Validator) ParseUndropTable() bool {
	return true
}

// ParseUndropView validates the Snowflake `UNDROP VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-view
func (v *Validator) ParseUndropView() bool {
	return true
}

// ParseUndropObj validates the Snowflake `UNDROP <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop
func (v *Validator) ParseUndropObj() bool {
	return true
}

// ParseUndropAccount validates the Snowflake `UNDROP ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-account
func (v *Validator) ParseUndropAccount() bool {
	return true
}

// ParseUndropSnapshot validates the Snowflake `UNDROP SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-snapshot
func (v *Validator) ParseUndropSnapshot() bool {
	return true
}

// ParseUndropStreamlit validates the Snowflake `UNDROP STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-streamlit
func (v *Validator) ParseUndropStreamlit() bool {
	return true
}

// ParseUndropTag validates the Snowflake `UNDROP TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-tag
func (v *Validator) ParseUndropTag() bool {
	return true
}

// ParseUndropType validates the Snowflake `UNDROP TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/undrop-type
func (v *Validator) ParseUndropType() bool {
	return true
}
