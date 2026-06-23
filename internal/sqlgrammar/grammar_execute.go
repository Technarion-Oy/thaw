package sqlgrammar

// CALL / EXECUTE / EXPLAIN / COMMENT — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseCall validates the Snowflake `CALL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/call
func (v *Validator) ParseCall() bool {
	return true
}

// ParseCallWithAnonymousProcedure validates the Snowflake `CALL (with anonymous procedure)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/call-with
func (v *Validator) ParseCallWithAnonymousProcedure() bool {
	return true
}

// ParseComment validates the Snowflake `COMMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/comment
func (v *Validator) ParseComment() bool {
	return true
}

// ParseExecuteAlert validates the Snowflake `EXECUTE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-alert
func (v *Validator) ParseExecuteAlert() bool {
	return true
}

// ParseExecuteDbtProject validates the Snowflake `EXECUTE DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-dbt-project
func (v *Validator) ParseExecuteDbtProject() bool {
	return true
}

// ParseExecuteDcmProject validates the Snowflake `EXECUTE DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-dcm-project
func (v *Validator) ParseExecuteDcmProject() bool {
	return true
}

// ParseExecuteImmediate validates the Snowflake `EXECUTE IMMEDIATE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-immediate
func (v *Validator) ParseExecuteImmediate() bool {
	return true
}

// ParseExecuteImmediateFrom validates the Snowflake `EXECUTE IMMEDIATE FROM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-immediate-from
func (v *Validator) ParseExecuteImmediateFrom() bool {
	return true
}

// ParseExecuteJobService validates the Snowflake `EXECUTE JOB SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-job-service
func (v *Validator) ParseExecuteJobService() bool {
	return true
}

// ParseExecuteNotebook validates the Snowflake `EXECUTE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-notebook
func (v *Validator) ParseExecuteNotebook() bool {
	return true
}

// ParseExecuteNotebookProject validates the Snowflake `EXECUTE NOTEBOOK PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-notebook-project
func (v *Validator) ParseExecuteNotebookProject() bool {
	return true
}

// ParseExecuteTask validates the Snowflake `EXECUTE TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-task
func (v *Validator) ParseExecuteTask() bool {
	return true
}

// ParseExplain validates the Snowflake `EXPLAIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/explain
func (v *Validator) ParseExplain() bool {
	return true
}
