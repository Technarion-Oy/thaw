package sqlgrammar

// Data loading & unloading / file staging — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseCopyFiles validates the Snowflake `COPY FILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/copy-files
//
// Syntax:
//
//	COPY FILES INTO @[<namespace>.]<stage_name>[/<path>/]
//	  FROM @[<namespace>.]<stage_name>[/<path>/]
//	  [ FILES = ( '<file_name>' [ , '<file_name>' ] [ , ... ] ) ]
//	  [ PATTERN = '<regex_pattern>' ]
//	  [ DETAILED_OUTPUT = { TRUE | FALSE } ]
//
//	COPY FILES INTO @[<namespace>.]<stage_name>[/<path>/]
//	  FROM ( SELECT <existing_url> [ , <new_filename> ] FROM ... )
//	  [ DETAILED_OUTPUT = { TRUE | FALSE } ]
func (v *Validator) ParseCopyFiles() bool {
	return true
}

// ParseCopyIntoLocation validates the Snowflake `COPY INTO <location>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/copy-into-location
//
// Syntax:
//
//	COPY INTO { internalStage | externalStage | externalLocation }
//	     FROM { [<namespace>.]<table_name> | ( <query> ) }
//	[ PARTITION BY <expr> ]
//	[ FILE_FORMAT = ( { FORMAT_NAME = '[<namespace>.]<file_format_name>' |
//	                    TYPE = { CSV | JSON | PARQUET } [ formatTypeOptions ] } ) ]
//	[ copyOptions ]
//	[ VALIDATION_MODE = RETURN_ROWS ]
//	[ HEADER ]
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	  | @[<namespace>.]%<table_name>[/<path>]
//	  | @~[/<path>]
//
//	externalStage ::=
//	  @[<namespace>.]<ext_stage_name>[/<path>]
//
//	externalLocation (for Amazon S3) ::=
//	  '<protocol>://<bucket>[/<path>]'
//	  [ { STORAGE_INTEGRATION = <integration_name> } | { CREDENTIALS = ( {  { AWS_KEY_ID = '<string>' AWS_SECRET_KEY = '<string>' [ AWS_TOKEN = '<string>' ] } } ) } ]
//	  [ ENCRYPTION = ( [ TYPE = 'AWS_CSE' ] [ MASTER_KEY = '<string>' ] |
//	                   [ TYPE = 'AWS_SSE_S3' ] |
//	                   [ TYPE = 'AWS_SSE_KMS' [ KMS_KEY_ID = '<string>' ] ] |
//	                   [ TYPE = 'NONE' ] ) ]
//
//	externalLocation (for Google Cloud Storage) ::=
//	  'gcs://<bucket>[/<path>]'
//	  [ STORAGE_INTEGRATION = <integration_name> ]
//	  [ ENCRYPTION = ( [ TYPE = 'GCS_SSE_KMS' ] [ KMS_KEY_ID = '<string>' ] | [ TYPE = 'NONE' ] ) ]
//
//	externalLocation (for Microsoft Azure) ::=
//	  'azure://<account>.blob.core.windows.net/<container>[/<path>]'
//	  [ { STORAGE_INTEGRATION = <integration_name> } | { CREDENTIALS = ( [ AZURE_SAS_TOKEN = '<string>' ] ) } ]
//	  [ ENCRYPTION = ( [ TYPE = { 'AZURE_CSE' | 'NONE' } ] [ MASTER_KEY = '<string>' ] ) ]
//
//	copyOptions ::=
//	     OVERWRITE = TRUE | FALSE
//	     SINGLE = TRUE | FALSE
//	     MAX_FILE_SIZE = <num>
//	     INCLUDE_QUERY_ID = TRUE | FALSE
//	     DETAILED_OUTPUT = TRUE | FALSE
func (v *Validator) ParseCopyIntoLocation() bool {
	return true
}

// ParseCopyIntoTable validates the Snowflake `COPY INTO <table>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/copy-into-table
//
// Syntax:
//
//	/* Standard data load */
//	COPY INTO [<namespace>.]<table_name>
//	     FROM { internalStage | externalStage | externalLocation }
//	[ FILES = ( '<file_name>' [ , '<file_name>' ] [ , ... ] ) ]
//	[ PATTERN = '<regex_pattern>' ]
//	[ FILE_FORMAT = ( { FORMAT_NAME = '[<namespace>.]<file_format_name>' |
//	                    TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML } [ formatTypeOptions ] } ) ]
//	[ copyOptions ]
//	[ VALIDATION_MODE = RETURN_<n>_ROWS | RETURN_ERRORS | RETURN_ALL_ERRORS ]
//
//	/* Data load with transformation */
//	COPY INTO [<namespace>.]<table_name> [ ( <col_name> [ , <col_name> ... ] ) ]
//	     FROM ( SELECT [<alias>.]$<file_col_num>[.<element>] [ , [<alias>.]$<file_col_num>[.<element>] ... ]
//	            FROM { internalStage | externalStage } )
//	[ FILES = ( '<file_name>' [ , '<file_name>' ] [ , ... ] ) ]
//	[ PATTERN = '<regex_pattern>' ]
//	[ FILE_FORMAT = ( { FORMAT_NAME = '[<namespace>.]<file_format_name>' |
//	                    TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML } [ formatTypeOptions ] } ) ]
//	[ copyOptions ]
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	  | @[<namespace>.]%<table_name>[/<path>]
//	  | @~[/<path>]
//
//	externalStage ::=
//	  @[<namespace>.]<ext_stage_name>[/<path>]
//
//	externalLocation (for Amazon S3) ::=
//	  '<protocol>://<bucket>[/<path>]'
//	  [ { STORAGE_INTEGRATION = <integration_name> } | { CREDENTIALS = ( {  { AWS_KEY_ID = '<string>' AWS_SECRET_KEY = '<string>' [ AWS_TOKEN = '<string>' ] } } ) } ]
//	  [ ENCRYPTION = ( [ TYPE = 'AWS_CSE' ] [ MASTER_KEY = '<string>' ] |
//	                    [ TYPE = 'AWS_SSE_S3' ] |
//	                    [ TYPE = 'AWS_SSE_KMS' [ KMS_KEY_ID = '<string>' ] ] |
//	                    [ TYPE = 'NONE' ] ) ]
//
//	externalLocation (for Google Cloud Storage) ::=
//	  'gcs://<bucket>[/<path>]'
//	  [ STORAGE_INTEGRATION = <integration_name> ]
//	  [ ENCRYPTION = ( [ TYPE = 'GCS_SSE_KMS' ] [ KMS_KEY_ID = '<string>' ] | [ TYPE = 'NONE' ] ) ]
//
//	externalLocation (for Microsoft Azure) ::=
//	  'azure://<account>.blob.core.windows.net/<container>[/<path>]'
//	  [ { STORAGE_INTEGRATION = <integration_name> } | { CREDENTIALS = ( [ AZURE_SAS_TOKEN = '<string>' ] ) } ]
//	  [ ENCRYPTION = ( [ TYPE = { 'AZURE_CSE' | 'NONE' } ] [ MASTER_KEY = '<string>' ] ) ]
func (v *Validator) ParseCopyIntoTable() bool {
	return true
}

// ParseGet validates the Snowflake `GET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/get
//
// Syntax:
//
//	GET internalStage file://<local_directory_path>
//	    [ PARALLEL = <integer> ]
//	    [ PATTERN = '<regex_pattern>' ]
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	  | @[<namespace>.]%<table_name>[/<path>]
//	  | @~[/<path>]
func (v *Validator) ParseGet() bool {
	return true
}

// ParseList validates the Snowflake `LIST` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/list
//
// Syntax:
//
//	LIST { internalStage | externalStage } [ PATTERN = '<regex_pattern>' ]
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.] <int_stage_name>[/<path>]
//	  | @[<namespace>.] %<table_name>[/<path>]
//	  | @~[/<path>]
//
//	externalStage ::=
//	  @[<namespace>.] <ext_stage_name>[/<path>]
//
//	LIST repositoryClone [ PATTERN = '<regex_pattern>' ]
//
//	Where:
//
//	repositoryClone ::=
//	  @[<namespace>.] <repository_clone>/<path>
func (v *Validator) ParseList() bool {
	return true
}

// ParsePut validates the Snowflake `PUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/put
//
// Syntax:
//
//	PUT file://<absolute_path_to_file>/<filename> internalStage
//	    [ PARALLEL = <integer> ]
//	    [ AUTO_COMPRESS = TRUE | FALSE ]
//	    [ SOURCE_COMPRESSION = AUTO_DETECT | GZIP | BZ2 | BROTLI | ZSTD | DEFLATE | RAW_DEFLATE | NONE ]
//	    [ OVERWRITE = TRUE | FALSE ]
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	  | @[<namespace>.]%<table_name>[/<path>]
//	  | @~[/<path>]
func (v *Validator) ParsePut() bool {
	return true
}

// ParseRemove validates the Snowflake `REMOVE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/remove
//
// Syntax:
//
//	REMOVE { internalStage | externalStage } [ PATTERN = '<regex_pattern>' ]
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	  | @[<namespace>.]%<table_name>[/<path>]
//	  | @~[/<path>]
//
//	externalStage ::=
//	    @[<namespace>.]<ext_stage_name>[/<path>]
func (v *Validator) ParseRemove() bool {
	return true
}
