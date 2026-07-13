package sqlgrammar

import "thaw/internal/sqltok"

// isSlashes reports whether s is one or more '/' and nothing else. The
// tokenizer emits a URI authority separator (`//` in s3://, `///` in
// file:///) as a single slash-run operator, so a file:// local path surfaces
// as one Operator token rather than several.
func isSlashes(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '/' {
			return false
		}
	}
	return true
}

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
	// stageRef: @[~|%][<namespace>.]<name>[/<path>] — leniently consume the @
	// token followed by any run of path-ish tokens (idents, dots, '/', '%', '~',
	// numbers). A bare '@~' or '@%table' is accepted too.
	stageRef := func() bool {
		at := v.Peek()
		if !v.Match(sqltok.At) {
			return false
		}
		// Only consume tokens directly adjacent to the previous one (no
		// intervening whitespace), so the stage path stops before the next
		// clause word (FROM, PATTERN, …).
		lastEnd := at.End
		matched := false
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
				(t.Kind == sqltok.Other && t.Text(v.src) == "~")
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
			matched = true
		}
		return matched
	}
	options := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("FILES", func() bool { return v.parseParenList(v.parseString) }),
				v.option("PATTERN", v.parseString),
				v.option("DETAILED_OUTPUT", v.parseBool),
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("COPY") },
		func() bool { return v.MatchWord("FILES") },
		func() bool { return v.MatchWord("INTO") },
		stageRef,
		func() bool { return v.MatchWord("FROM") },
		// FROM may be a stage ref or a ( SELECT ... ) sub-query.
		func() bool { return v.Choice(stageRef, v.consumeBalancedParens) },
		options,
	)
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
	stageRef := func() bool {
		at := v.Peek()
		if !v.Match(sqltok.At) {
			return false
		}
		// Only consume tokens directly adjacent to the previous one (no
		// intervening whitespace), so the stage path stops before the next
		// clause word (FROM, PATTERN, …).
		lastEnd := at.End
		matched := false
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
				(t.Kind == sqltok.Other && t.Text(v.src) == "~")
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
			matched = true
		}
		return matched
	}
	// destination: an internal/external stage (@…) or an external location string.
	dest := func() bool {
		return v.Choice(stageRef, v.parseString)
	}
	// source after FROM: a table name or a ( <query> ).
	source := func() bool {
		return v.Choice(v.consumeBalancedParens, v.parseIdentPath)
	}
	// trailing clause soup (FILE_FORMAT, copy options, PARTITION BY, etc.). Each
	// trailing token-run is accepted leniently; the required skeleton above is
	// COPY INTO <dest> FROM <source>.
	trailing := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("PARTITION", "BY") },
						func() bool { return v.Choice(v.consumeBalancedParens, v.parseScalar) },
					)
				},
				v.option("FILE_FORMAT", v.consumeBalancedParens),
				v.option("OVERWRITE", v.parseBool),
				v.option("SINGLE", v.parseBool),
				v.option("MAX_FILE_SIZE", v.parseNumber),
				v.option("INCLUDE_QUERY_ID", v.parseBool),
				v.option("DETAILED_OUTPUT", v.parseBool),
				v.option("VALIDATION_MODE", v.parseScalar),
				v.option("STORAGE_INTEGRATION", v.parseIdentPath),
				v.option("CREDENTIALS", v.consumeBalancedParens),
				v.option("ENCRYPTION", v.consumeBalancedParens),
				func() bool { return v.MatchWord("HEADER") },
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("COPY") },
		func() bool { return v.MatchWord("INTO") },
		dest,
		func() bool { return v.MatchWord("FROM") },
		source,
		trailing,
	)
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
	stageRef := func() bool {
		at := v.Peek()
		if !v.Match(sqltok.At) {
			return false
		}
		// Only consume tokens directly adjacent to the previous one (no
		// intervening whitespace), so the stage path stops before the next
		// clause word (FROM, PATTERN, …).
		lastEnd := at.End
		matched := false
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
				(t.Kind == sqltok.Other && t.Text(v.src) == "~")
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
			matched = true
		}
		return matched
	}
	// FROM source: a stage (@…), an external-location string, or a ( SELECT … )
	// transformation sub-query.
	source := func() bool {
		return v.Choice(stageRef, v.parseString, v.consumeBalancedParens)
	}
	trailing := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("FILES", func() bool { return v.parseParenList(v.parseString) }),
				v.option("PATTERN", v.parseString),
				v.option("FILE_FORMAT", v.consumeBalancedParens),
				v.option("VALIDATION_MODE", v.parseScalar),
				v.option("STORAGE_INTEGRATION", v.parseIdentPath),
				v.option("CREDENTIALS", v.consumeBalancedParens),
				v.option("ENCRYPTION", v.consumeBalancedParens),
				// generic copyOptions (ON_ERROR, SIZE_LIMIT, PURGE, FORCE, …).
				v.option2(func() bool { return v.Match(sqltok.Identifier) }, v.parseScalar),
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("COPY") },
		func() bool { return v.MatchWord("INTO") },
		v.parseIdentPath,
		// optional ( <col>, … ) column list for transformation loads.
		func() bool { return v.Optional(v.consumeBalancedParens) },
		func() bool { return v.MatchWord("FROM") },
		source,
		trailing,
	)
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
	stageRef := func() bool {
		at := v.Peek()
		if !v.Match(sqltok.At) {
			return false
		}
		// Only consume tokens directly adjacent to the previous one (no
		// intervening whitespace), so the stage path stops before the next
		// clause word (FROM, PATTERN, …).
		lastEnd := at.End
		matched := false
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
				(t.Kind == sqltok.Other && t.Text(v.src) == "~")
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
			matched = true
		}
		return matched
	}
	// localPath: file://<dir> — either a quoted string or a bare file://… run of
	// path-ish tokens consumed only while directly adjacent (no whitespace), so
	// the path stops before the next option word (PARALLEL, PATTERN, …).
	localPath := func() bool {
		if v.Match(sqltok.StringLit) {
			return true
		}
		fileTok := v.Peek()
		if !v.MatchWord("file") {
			return false
		}
		lastEnd := fileTok.End
		if v.Peek().Start != lastEnd || !v.Match(sqltok.Colon) {
			v.expect(":")
			return false
		}
		lastEnd = v.tokens[v.pos-1].End
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (isSlashes(t.Text(v.src)) || t.Text(v.src) == "*"))
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
		}
		return true
	}
	options := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("PARALLEL", v.parseNumber),
				v.option("PATTERN", v.parseString),
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("GET") },
		stageRef,
		localPath,
		options,
	)
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
	stageRef := func() bool {
		at := v.Peek()
		if !v.Match(sqltok.At) {
			return false
		}
		// Only consume tokens directly adjacent to the previous one (no
		// intervening whitespace), so the stage path stops before the next
		// clause word (FROM, PATTERN, …).
		lastEnd := at.End
		matched := false
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
				(t.Kind == sqltok.Other && t.Text(v.src) == "~")
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
			matched = true
		}
		return matched
	}
	return v.Sequence(
		// LIST has the synonym LS.
		func() bool {
			return v.Choice(func() bool { return v.MatchWord("LIST") }, func() bool { return v.MatchWord("LS") })
		},
		stageRef,
		func() bool { return v.Optional(v.option("PATTERN", v.parseString)) },
	)
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
	// localPath: file://<path> consumed only while directly adjacent, so it stops
	// before the stage ref / option words that follow.
	localPath := func() bool {
		if v.Match(sqltok.StringLit) {
			return true
		}
		fileTok := v.Peek()
		if !v.MatchWord("file") {
			return false
		}
		lastEnd := fileTok.End
		if v.Peek().Start != lastEnd || !v.Match(sqltok.Colon) {
			v.expect(":")
			return false
		}
		lastEnd = v.tokens[v.pos-1].End
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (isSlashes(t.Text(v.src)) || t.Text(v.src) == "*"))
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
		}
		return true
	}
	stageRef := func() bool {
		at := v.Peek()
		if !v.Match(sqltok.At) {
			return false
		}
		// Only consume tokens directly adjacent to the previous one (no
		// intervening whitespace), so the stage path stops before the next
		// clause word (PARALLEL, OVERWRITE, …).
		lastEnd := at.End
		matched := false
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
				(t.Kind == sqltok.Other && t.Text(v.src) == "~")
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
			matched = true
		}
		return matched
	}
	options := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("PARALLEL", v.parseNumber),
				v.option("AUTO_COMPRESS", v.parseBool),
				v.option("SOURCE_COMPRESSION", v.wordsValue(
					"AUTO_DETECT", "GZIP", "BZ2", "BROTLI", "ZSTD", "DEFLATE", "RAW_DEFLATE", "NONE")),
				v.option("OVERWRITE", v.parseBool),
			)
		})
	}
	return v.Sequence(
		// PUT is not a reserved keyword in the lexer; match it as a word.
		func() bool { return v.MatchWord("PUT") },
		localPath,
		stageRef,
		options,
	)
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
	stageRef := func() bool {
		at := v.Peek()
		if !v.Match(sqltok.At) {
			return false
		}
		// Only consume tokens directly adjacent to the previous one (no
		// intervening whitespace), so the stage path stops before the next
		// clause word (FROM, PATTERN, …).
		lastEnd := at.End
		matched := false
		for !v.AtEnd() {
			t := v.Peek()
			if t.Start != lastEnd {
				break
			}
			ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
				(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
				(t.Kind == sqltok.Other && t.Text(v.src) == "~")
			if !ok {
				break
			}
			lastEnd = t.End
			v.advance()
			matched = true
		}
		return matched
	}
	return v.Sequence(
		// REMOVE has the synonym RM.
		func() bool {
			return v.Choice(func() bool { return v.MatchWord("REMOVE") }, func() bool { return v.MatchWord("RM") })
		},
		stageRef,
		func() bool { return v.Optional(v.option("PATTERN", v.parseString)) },
	)
}
