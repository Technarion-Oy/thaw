package sqlgrammar

import "thaw/internal/sqltok"

// Sessions & context (USE, SET) — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseSet validates the Snowflake `SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/set
//
// Syntax:
//
//	SET <var> = <expr>
//
//	SET ( <var> [ , <var> ... ] ) = ( <expr> [ , <expr> ... ] )
func (v *Validator) ParseSet() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("SET") },
		func() bool {
			return v.Choice(
				// SET ( <var>, ... ) = ( <expr>, ... )
				func() bool {
					return v.Sequence(
						func() bool { return v.parseParenList(v.parseIdentPath) },
						func() bool { return v.MatchOp("=") },
						v.consumeBalancedParens,
					)
				},
				// SET <var> = <expr>
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool { return v.MatchOp("=") },
						v.consumeRest,
					)
				},
			)
		},
	)
}

// ParseUnset validates the Snowflake `UNSET` command (drops a session variable).
// Reference: https://docs.snowflake.com/en/sql-reference/sql/unset
//
// Syntax:
//
//	UNSET <var>
//
//	UNSET ( <var> [ , <var> ... ] )
func (v *Validator) ParseUnset() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("UNSET") },
		func() bool {
			return v.Choice(
				// UNSET ( <var>, ... )
				func() bool { return v.parseParenList(v.parseIdentPath) },
				// UNSET <var>
				v.parseIdentPath,
			)
		},
	)
}

// ParseUse validates the Snowflake `USE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use
//
// Syntax:
//
//	USE <object>
func (v *Validator) ParseUse() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("USE") },
		func() bool {
			return v.Choice(
				// USE SECONDARY ROLES { ALL | NONE | <role> [, ...] }
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("SECONDARY") },
						func() bool { return v.MatchWord("ROLES") },
						func() bool {
							return v.Choice(
								func() bool { return v.MatchWord("ALL") },
								func() bool { return v.MatchWord("NONE") },
								func() bool {
									return v.Sequence(
										v.parseIdentPath,
										func() bool {
											return v.ZeroOrMore(func() bool {
												return v.Sequence(
													func() bool { return v.Match(sqltok.Comma) },
													v.parseIdentPath,
												)
											})
										},
									)
								},
							)
						},
					)
				},
				// USE { ROLE | WAREHOUSE | DATABASE | SCHEMA } <name>
				func() bool {
					return v.Sequence(
						v.wordsValue("ROLE", "WAREHOUSE", "DATABASE", "SCHEMA"),
						v.parseIdentPath,
					)
				},
				// USE <name>
				v.parseIdentPath,
			)
		},
	)
}

// ParseUseDatabase validates the Snowflake `USE DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-database
//
// Syntax:
//
//	USE [ DATABASE ] <name>
func (v *Validator) ParseUseDatabase() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("USE") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("DATABASE") }) },
		v.parseIdentPath,
	)
}

// ParseUseRole validates the Snowflake `USE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-role
//
// Syntax:
//
//	USE ROLE <name>
func (v *Validator) ParseUseRole() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("USE") },
		func() bool { return v.MatchWord("ROLE") },
		v.parseIdentPath,
	)
}

// ParseUseSchema validates the Snowflake `USE SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-schema
//
// Syntax:
//
//	USE [ SCHEMA ] [<db_name>.]<name>
func (v *Validator) ParseUseSchema() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("USE") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SCHEMA") }) },
		v.parseIdentPath,
	)
}

// ParseUseSecondaryRoles validates the Snowflake `USE SECONDARY ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-secondary-roles
//
// Syntax:
//
//	USE SECONDARY ROLES {
//	      ALL
//	    | NONE
//	    | <role_name> [ , <role_name> ... ]
//	  }
func (v *Validator) ParseUseSecondaryRoles() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("USE") },
		func() bool { return v.MatchWord("SECONDARY") },
		func() bool { return v.MatchWord("ROLES") },
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("ALL") },
				func() bool { return v.MatchWord("NONE") },
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Sequence(
									func() bool { return v.Match(sqltok.Comma) },
									v.parseIdentPath,
								)
							})
						},
					)
				},
			)
		},
	)
}

// ParseUseWarehouse validates the Snowflake `USE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/use-warehouse
//
// Syntax:
//
//	USE WAREHOUSE <name>
func (v *Validator) ParseUseWarehouse() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("USE") },
		func() bool { return v.MatchWord("WAREHOUSE") },
		v.parseIdentPath,
	)
}
