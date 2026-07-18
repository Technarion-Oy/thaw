package sqlgrammar

import (
	"strings"

	"thaw/internal/sqltok"
)

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
//	CALL { <procedure_name> | <model_name>!<method_name> } ( [ [ <arg_name> => ] <arg> , ... ] )
//	  [ INTO :<snowflake_scripting_variable> ]
func (v *Validator) ParseCall() bool {
	// scriptingVar: :<identifier> bind target.
	scriptingVar := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.Colon) },
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CALL") },
		v.parseIdentPath,
		// Optional model-method form: <model>!<method> (e.g. my_model!FORECAST(…)).
		callModelMethod(v),
		// argument list — free-form ( [ name => ] arg, … ).
		v.consumeBalancedParens,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("INTO") }, scriptingVar)
			})
		},
	)
}

// callModelMethod matches an optional `! <method>` suffix on a CALL callee — the
// Snowflake model-method invocation form `<model_name>!<method>(…)` (e.g.
// `CALL my_model!FORECAST(FORECASTING_PERIODS => 3)`).
func callModelMethod(v *Validator) Rule {
	return func() bool {
		return v.Optional(func() bool {
			return v.Sequence(
				func() bool { return v.MatchOp("!") },
				v.parseIdentPath,
			)
		})
	}
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
	scriptingVar := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.Colon) },
			v.parseIdentPath,
		)
	}
	// WITH <name> AS PROCEDURE — the required leading skeleton.
	if !v.Sequence(
		func() bool { return v.MatchKeyword("WITH") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("AS") },
		func() bool { return v.MatchWord("PROCEDURE") },
	) {
		return false
	}
	// The procedure definition (signature, RETURNS, LANGUAGE, body, optional extra
	// CTEs) is too free-form to model; consume tokens until the trailing CALL
	// keyword that begins the invocation.
	foundCall := false
	for !v.AtEnd() {
		t := v.Peek()
		if t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), "CALL") {
			foundCall = true
			break
		}
		v.advance()
	}
	if !foundCall {
		v.expect("CALL")
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("CALL") },
		v.parseIdentPath,
		callModelMethod(v),
		v.consumeBalancedParens,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("INTO") }, scriptingVar)
			})
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchKeyword("COMMENT") },
		func() bool { return v.ifExists() },
		func() bool { return v.MatchWord("ON") },
		// <object_type> <object_name> (object_type is a single word such as TABLE,
		// VIEW, COLUMN; the name may be dot-qualified).
		func() bool {
			t := v.Peek()
			if !t.Kind.IsIdentLike() {
				v.expect("object type")
				return false
			}
			v.advance()
			return true
		},
		v.parseIdentPath,
		func() bool { return v.MatchWord("IS") },
		v.parseString,
	)
}

// ParseExecuteAlert validates the Snowflake `EXECUTE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-alert
//
// Syntax:
//
//	EXECUTE ALERT <name>
func (v *Validator) ParseExecuteAlert() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("ALERT") },
		v.parseIdentPath,
	)
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
	options := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("ARGS", v.parseString),
				v.option("DBT_VERSION", v.parseString),
				v.option("PROJECT_ROOT", v.parseString),
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("DBT") },
		func() bool { return v.MatchWord("PROJECT") },
		func() bool { return v.ifExists() },
		// either <name> or [ FROM WORKSPACE <name> ].
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("FROM") },
							func() bool { return v.MatchWord("WORKSPACE") },
							v.parseIdentPath,
						)
					},
					v.parseIdentPath,
				)
			})
		},
		options,
	)
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
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("DCM") },
		func() bool { return v.MatchWord("PROJECT") },
		v.parseIdentPath,
		// the action keyword (PLAN | DEPLOY | REFRESH | TEST | PREVIEW | PURGE).
		v.wordsValue("PLAN", "DEPLOY", "REFRESH", "TEST", "PREVIEW", "PURGE"),
		// the remaining action-specific clauses (USING, FROM, AS, ALL, …) are
		// free-form; accept them all.
		v.consumeRest,
	)
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
	// the body: a '<string>' literal, a $$…$$ dollar-quoted block (the docs'
	// primary example), a $<session_variable>, or a bare <variable>.
	body := func() bool {
		return v.Choice(
			v.parseString,
			func() bool { return v.Match(sqltok.DollarQuoted) },
			func() bool {
				return v.Sequence(
					func() bool {
						t := v.Peek()
						if t.Kind == sqltok.Other && t.Text(v.src) == "$" {
							v.advance()
							return true
						}
						v.expect("$")
						return false
					},
					v.parseScalar,
				)
			},
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("IMMEDIATE") },
		body,
		// optional USING ( <bind>, … ).
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("USING") }, v.consumeBalancedParens)
			})
		},
	)
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
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("IMMEDIATE") },
		func() bool { return v.MatchWord("FROM") },
		// absolute (@stage/path) or relative ('…') file path.
		func() bool { return v.Choice(stageRef, v.parseString) },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("USING") }, v.consumeBalancedParens)
			})
		},
		func() bool { return v.Optional(v.option("DRY_RUN", v.parseBool)) },
	)
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
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("JOB") },
		func() bool { return v.MatchWord("SERVICE") },
		func() bool { return v.MatchWord("IN") },
		func() bool { return v.MatchWord("COMPUTE") },
		func() bool { return v.MatchWord("POOL") },
		v.parseIdentPath,
		// the FROM specification + options block is free-form; require at least the
		// FROM keyword then accept the remainder.
		func() bool { return v.MatchWord("FROM") },
		v.consumeRest,
	)
}

// ParseExecuteNotebook validates the Snowflake `EXECUTE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/execute-notebook
//
// Syntax:
//
//	EXECUTE NOTEBOOK <name>([ <parameter_string> [ , ... ] ]);
func (v *Validator) ParseExecuteNotebook() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("NOTEBOOK") },
		v.parseIdentPath,
		// ( [ <parameter_string> [, …] ] ) — possibly-empty arg list.
		v.consumeBalancedParens,
	)
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
	options := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("MAIN_FILE", v.parseString),
				v.option("COMPUTE_POOL", v.parseScalar),
				v.option("QUERY_WAREHOUSE", v.parseScalar),
				v.option("RUNTIME", v.parseString),
				v.option("ARGUMENTS", v.parseString),
				v.option("REQUIREMENTS_FILE", v.parseString),
				v.option("EXTERNAL_ACCESS_INTEGRATIONS", v.consumeBalancedParens),
				v.option("SECRETS", v.consumeBalancedParens),
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("NOTEBOOK") },
		func() bool { return v.MatchWord("PROJECT") },
		v.parseIdentPath,
		options,
	)
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
	tail := func() bool {
		return v.Optional(func() bool {
			return v.Choice(
				// USING CONFIG = <configuration_string>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("USING") },
						v.option("CONFIG", v.parseScalar),
					)
				},
				// RETRY LAST
				func() bool { return v.phrase("RETRY", "LAST") },
				// RETRY GRAPH RUN GROUP '<id>'
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("RETRY", "GRAPH", "RUN", "GROUP") },
						v.parseString,
					)
				},
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXECUTE") },
		func() bool { return v.MatchWord("TASK") },
		v.parseIdentPath,
		tail,
	)
}

// ParseExplain validates the Snowflake `EXPLAIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/explain
//
// Syntax:
//
//	EXPLAIN [ USING { TABULAR | JSON | TEXT } ] <statement>
func (v *Validator) ParseExplain() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("EXPLAIN") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("USING") },
					v.wordsValue("TABULAR", "JSON", "TEXT"),
				)
			})
		},
		// the inner statement is free-form; require at least one token then accept
		// the remainder.
		func() bool {
			if v.AtEnd() {
				v.expect("statement")
				return false
			}
			return v.consumeRest()
		},
	)
}
