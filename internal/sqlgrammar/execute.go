package sqlgrammar

// CALL / EXECUTE / EXPLAIN / COMMENT — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseCall validates the Snowflake `CALL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/call
//
// Syntax:
//
//	CALL <procedure_name> ( [ [ <arg_name> => ] <arg> , ... ] )
//	  [ INTO :<snowflake_scripting_variable> ]
func (v *Validator) ParseCall() bool {
	return true
}

// ParseCallWithAnonymousProcedure validates the Snowflake `CALL (with anonymous procedure)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/call-with
//
// Syntax:
//
//	WITH <name> AS PROCEDURE ([ <arg_name> <arg_data_type> ]) [ , ... ] )
//	  RETURNS { <result_data_type> [ [ NOT ] NULL ] | TABLE ( [ <col_name> <col_data_type> [ , ... ] ] ) }
//	  LANGUAGE { JAVA }
//	  RUNTIME_VERSION = '<scala_or_java_runtime_version>'
//	  PACKAGES = ( 'com.snowflake:snowpark:<version>' [, '<package_name_and_version>' ...] )
//	  [ IMPORTS = ( '<stage_path_and_directory_or_file_name_to_read>' [, '<stage_path_and_directory_or_file_name_to_read>' ...] ) ]
//	  HANDLER = '<fully_qualified_method_name>'
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ AS '<procedure_definition>' ]
//	  [ , <cte_nameN> [ ( <cte_column_list> ) ] AS ( SELECT ...  ) ]
//	CALL <name> ( [ [ <arg_name> => ] <arg> , ... ] )
//	  [ INTO :<snowflake_scripting_variable> ]
//
//	(JavaScript / Python / Scala / Snowflake Scripting variants follow the same
//	WITH <name> AS PROCEDURE (...) ... CALL <name> (...) shape with a language-
//	specific procedure definition.)
func (v *Validator) ParseCallWithAnonymousProcedure() bool {
	return true
}

// ParseComment validates the Snowflake `COMMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/comment
//
// Syntax:
//
//	COMMENT [ IF EXISTS ] ON <object_type> <object_name> IS '<string_literal>';
//
//	COMMENT [ IF EXISTS ] ON COLUMN <table_name>.<column_name> IS '<string_literal>';
func (v *Validator) ParseComment() bool {
	return true
}

// ParseExecuteAlert validates the Snowflake `EXECUTE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-alert
//
// Syntax:
//
//	EXECUTE ALERT <name>
func (v *Validator) ParseExecuteAlert() bool {
	return true
}

// ParseExecuteDbtProject validates the Snowflake `EXECUTE DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-dbt-project
//
// Syntax:
//
//	EXECUTE DBT PROJECT [ IF EXISTS ] <name>
//	  [ ARGS = '[ <dbt_command> ] [ --<dbt_cli_option> <option_value_1> [ ... ] ] [ ... ]' ]
//	  [ DBT_VERSION = 'version_number' ]
//
//	EXECUTE DBT PROJECT [ IF EXISTS ] [ FROM WORKSPACE <name> ]
//	  [ ARGS = '[ <dbt_command> ] [ --<dbt_cli_option> <option_value_1> [ ... ] [ ... ] ]' ]
//	  [ DBT_VERSION = 'version_number' ]
//	  [ PROJECT_ROOT = '<subdirectory_path>' ]
func (v *Validator) ParseExecuteDbtProject() bool {
	return true
}

// ParseExecuteDcmProject validates the Snowflake `EXECUTE DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-dcm-project
//
// Syntax:
//
//	EXECUTE DCM PROJECT <name>
//	  PLAN
//	  [ USING [ CONFIGURATION <config_name> ] [ (<expr>, [, <expr>, ...]) ] ]
//	  FROM '<source-files_path>'
//
//	EXECUTE DCM PROJECT <name>
//	  DEPLOY [ AS "<deployment_name_alias>" ]
//	  [ USING [ CONFIGURATION <name> ] [ (<expr>, [, <expr>, ...]) ] ]
//	  FROM '<source-files_path>'
//
//	EXECUTE DCM PROJECT <name>
//	  REFRESH ALL
//
//	EXECUTE DCM PROJECT <name>
//	  TEST ALL
//
//	EXECUTE DCM PROJECT <name>
//	  PREVIEW <fully_qualified_table_object_name>
//	  USING CONFIGURATION <config_name>
//	  FROM '<source_files_path>'
//
//	EXECUTE DCM PROJECT <name>
//	  PURGE [ AS "<deployment_name_alias>" ]
func (v *Validator) ParseExecuteDcmProject() bool {
	return true
}

// ParseExecuteImmediate validates the Snowflake `EXECUTE IMMEDIATE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-immediate
//
// Syntax:
//
//	EXECUTE IMMEDIATE '<string_literal>'
//	    [ USING ( <bind_variable> [ , <bind_variable> ... ] ) ]
//
//	EXECUTE IMMEDIATE <variable>
//	    [ USING ( <bind_variable> [ , <bind_variable> ... ] ) ]
//
//	EXECUTE IMMEDIATE $<session_variable>
//	    [ USING ( <bind_variable> [ , <bind_variable> ... ] ) ]
func (v *Validator) ParseExecuteImmediate() bool {
	return true
}

// ParseExecuteImmediateFrom validates the Snowflake `EXECUTE IMMEDIATE FROM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-immediate-from
//
// Syntax:
//
//	EXECUTE IMMEDIATE
//	  FROM { absoluteFilePath | relativeFilePath }
//	  [ USING ( <key> => <value> [ , <key> => <value> [ , ... ] ]  )  ]
//	  [ DRY_RUN = { TRUE | FALSE } ]
//
//	Where:
//
//	absoluteFilePath ::=
//	   @[ <namespace>. ]<stage_name>/<path>/<filename>
//
//	relativeFilePath ::=
//	   '[ / | ./ | ../ ]<path>/<filename>'
func (v *Validator) ParseExecuteImmediateFrom() bool {
	return true
}

// ParseExecuteJobService validates the Snowflake `EXECUTE JOB SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-job-service
//
// Syntax:
//
//	EXECUTE JOB SERVICE
//	  IN COMPUTE POOL <compute_pool_name>
//	  {
//	     fromSpecification
//	     | fromSpecificationTemplate
//	  }
//	  [ NAME = [<db>.<schema>.]<name> ]
//	  [ ASYNC = { TRUE | FALSE } ]
//	  [ REPLICAS = = <num> ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>']
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <EAI_name> [ , ... ] ) ]
//
//	Where:
//
//	fromSpecification ::=
//	  {
//	    FROM @<stage> SPECIFICATION_FILE = '<yaml_file_stage_path>'
//	    | FROM SPECIFICATION <specification_text>
//	  }
//
//	fromSpecificationTemplate ::=
//	  {
//	    FROM @<stage> SPECIFICATION_TEMPLATE_FILE = '<yaml_file_stage_path>'
//	    | FROM SPECIFICATION_TEMPLATE <specification_text>
//	  }
//	  USING ( <key> => <value> [ , <key> => <value> [ , ... ] ]  )
func (v *Validator) ParseExecuteJobService() bool {
	return true
}

// ParseExecuteNotebook validates the Snowflake `EXECUTE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-notebook
//
// Syntax:
//
//	EXECUTE NOTEBOOK <name>([ <parameter_string> [ , ... ] ]);
func (v *Validator) ParseExecuteNotebook() bool {
	return true
}

// ParseExecuteNotebookProject validates the Snowflake `EXECUTE NOTEBOOK PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-notebook-project
//
// Syntax:
//
//	EXECUTE NOTEBOOK PROJECT <database_name>.<schema_name>.<project_name>
//	  MAIN_FILE = 'notebook.ipynb'
//	  COMPUTE_POOL = '<compute_pool_name>'
//	  QUERY_WAREHOUSE = '<warehouse_name>'
//	  RUNTIME = '<runtime_version>'
//	  [ ARGUMENTS = '<parameter_string>' ]
//	  [ REQUIREMENTS_FILE = '<path/to/requirements.txt>' ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
//	  [ SECRETS = ( <database_name>.<schema_name>.<secret_name> [ , ... ] ) ];
func (v *Validator) ParseExecuteNotebookProject() bool {
	return true
}

// ParseExecuteTask validates the Snowflake `EXECUTE TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-task
//
// Syntax:
//
//	EXECUTE TASK <name>
//	  [ USING CONFIG = <configuration_string> ]
//
//	EXECUTE TASK <name> RETRY LAST
//
//	EXECUTE TASK <name> RETRY GRAPH RUN GROUP '<graph_run_group_id>'
func (v *Validator) ParseExecuteTask() bool {
	return true
}

// ParseExplain validates the Snowflake `EXPLAIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/explain
//
// Syntax:
//
//	EXPLAIN [ USING { TABULAR | JSON | TEXT } ] <statement>
func (v *Validator) ParseExplain() bool {
	return true
}
