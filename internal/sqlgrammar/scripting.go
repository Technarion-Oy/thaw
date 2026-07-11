package sqlgrammar

// Snowflake Scripting — grammar rules for the procedural constructs that appear
// inside a scripting block (`DECLARE … BEGIN … EXCEPTION … END`). These form a
// separate grammar layer from the standalone-statement grammars and are wired
// into the block-body statement `Choice` — the set of statements allowed inside
// `BEGIN … END` — rather than into top-level dispatch (dispatch.go).
//
// ponytail: the block-body `Choice` itself is not built yet (later issue in
// #556); until it lands ParseAwait is reachable only via direct calls and its
// table-driven tests. Add it to that Choice's candidate list when it exists.

// ParseAwait validates the Snowflake Scripting `AWAIT` construct.
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/await
//
// Syntax:
//
//	AWAIT { ALL | <result_set_name> }
func (v *Validator) ParseAwait() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("AWAIT") },
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("ALL") },
				v.parseIdentPath,
			)
		},
	)
}
