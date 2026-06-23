package sqlgrammar

// Transactions — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseBegin validates the Snowflake `BEGIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/begin
//
// Syntax:
//
//	BEGIN [ { WORK | TRANSACTION } ] [ NAME <name> ]
//
//	START TRANSACTION [ NAME <name> ]
func (v *Validator) ParseBegin() bool {
	nameClause := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(
				func() bool { return v.MatchWord("NAME") },
				v.parseIdentPath,
			)
		})
	}
	return v.Choice(
		// BEGIN [ { WORK | TRANSACTION } ] [ NAME <name> ]
		func() bool {
			return v.Sequence(
				func() bool { return v.MatchKeyword("BEGIN") },
				func() bool { return v.Optional(v.wordsValue("WORK", "TRANSACTION")) },
				nameClause,
			)
		},
		// START TRANSACTION [ NAME <name> ]
		func() bool {
			return v.Sequence(
				func() bool { return v.MatchWord("START") },
				func() bool { return v.MatchWord("TRANSACTION") },
				nameClause,
			)
		},
	)
}

// ParseCommit validates the Snowflake `COMMIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/commit
//
// Syntax:
//
//	COMMIT [ WORK ]
func (v *Validator) ParseCommit() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("COMMIT") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("WORK") }) },
	)
}

// ParseRollback validates the Snowflake `ROLLBACK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/rollback
//
// Syntax:
//
//	ROLLBACK [ WORK ]
func (v *Validator) ParseRollback() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("ROLLBACK") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("WORK") }) },
	)
}
