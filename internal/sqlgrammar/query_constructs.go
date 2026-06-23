package sqlgrammar

import (
	"strings"

	"thaw/internal/sqltok"
)

// Query syntax constructs (SELECT sub-clauses) — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseWith validates the Snowflake `WITH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/with
//
// Syntax:
//
//	[ WITH
//	       <cte_name1> [ ( <cte_column_list> ) ] AS ( SELECT ...  )
//	   [ , <cte_name2> [ ( <cte_column_list> ) ] AS ( SELECT ...  ) ]
//	   [ , <cte_nameN> [ ( <cte_column_list> ) ] AS ( SELECT ...  ) ]
//	]
//	SELECT ...
//
//	[ WITH [ RECURSIVE ]
//	       <cte_name1> ( <cte_column_list> ) AS ( anchorClause UNION ALL recursiveClause )
//	   [ , <cte_name2> ( <cte_column_list> ) AS ( anchorClause UNION ALL recursiveClause ) ]
//	   [ , <cte_nameN> ( <cte_column_list> ) AS ( anchorClause UNION ALL recursiveClause ) ]
//	]
//	SELECT ...
//
//	anchorClause ::=
//	    SELECT <anchor_column_list> FROM ...
//
//	recursiveClause ::=
//	    SELECT <recursive_column_list> FROM ... [ JOIN ... ]
func (v *Validator) ParseWith() bool {
	// WITH [ RECURSIVE ] cte [ , cte ]* <query>
	cte := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool { return v.Optional(v.consumeBalancedParens) }, // ( col_list )
			func() bool { return v.MatchWord("AS") },
			v.consumeBalancedParens, // ( SELECT ... )
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("WITH") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("RECURSIVE") }) },
		cte,
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, cte)
			})
		},
		v.consumeRest, // trailing SELECT ...
	)
}

// ParseTopN validates the Snowflake `TOP_N` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/top_n
//
// Syntax:
//
//	SELECT
//	  [ TOP <n> ]
//	    ...
//	FROM ...
//	[ ORDER BY ... ]
//	[ ... ]
func (v *Validator) ParseTopN() bool {
	// SELECT [ TOP <n> ] ... — model the leading SELECT [ TOP <n> ] skeleton.
	return v.Sequence(
		func() bool { return v.MatchKeyword("SELECT") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("TOP") },
					func() bool { return v.Match(sqltok.NumberLit) },
				)
			})
		},
		v.consumeRest,
	)
}

// ParseInto validates the Snowflake `INTO` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/into
//
// Syntax:
//
//	SELECT <expression1>
//	   [ , <expression2> ]
//	   [ , <expressionN> ]
//	[ INTO :<variable1> ]
//	   [ , :<variable2> ]
//	   [ , :<variableN> ]
//	FROM ...
//	WHERE ...
//	[ ... ]
func (v *Validator) ParseInto() bool {
	// SELECT <expr> [, ...] INTO :<var> [, :<var> ]* FROM ...
	// Model: SELECT <something> INTO :var [, :var]* then free-form FROM/WHERE tail.
	// We require the SELECT lead, a non-empty select list (consumed leniently up to
	// INTO), the INTO keyword, and at least one :variable.
	intoVar := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.Colon) },
			v.parseIdentPath,
		)
	}
	// Consume select-list tokens up to (but not including) the INTO keyword.
	selectItem := func() bool {
		if v.AtEnd() {
			return false
		}
		t := v.Peek()
		if t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), "INTO") {
			return false
		}
		switch t.Kind {
		case sqltok.LParen:
			return v.consumeBalancedParens()
		case sqltok.RParen:
			return false
		}
		v.advance()
		return true
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("SELECT") },
		func() bool { return v.ZeroOrMore(selectItem) },
		func() bool { return v.MatchWord("INTO") },
		intoVar,
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, intoVar)
			})
		},
		v.consumeRest,
	)
}

// ParseFrom validates the Snowflake `FROM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/from
//
// Syntax:
//
//	SELECT ...
//	FROM objectReference [ JOIN objectReference [ ... ] ]
//	[ ... ]
//
//	objectReference ::=
//	   {
//	      [<namespace>.]<object_name>
//	           [ AT | BEFORE ( <object_state> ) ]
//	           [ CHANGES ( <change_tracking_type> ) ]
//	           [ MATCH_RECOGNIZE ]
//	           [ PIVOT | UNPIVOT ]
//	           [ [ AS ] <alias_name> ]
//	           [ SAMPLE ]
//	      | <table_function>
//	           [ PIVOT | UNPIVOT ]
//	           [ [ AS ] <alias_name> ]
//	           [ SAMPLE ]
//	      | ( VALUES (...) )
//	           [ SAMPLE ]
//	      | [ LATERAL ] ( <subquery> )
//	           [ [ AS ] <alias_name> ]
//	      | @[<namespace>.]<stage_name>[/<path>]
//	           [ ( FILE_FORMAT => <format_name>, PATTERN => '<regex_pattern>' ) ]
//	           [ [ AS ] <alias_name> ]
//	      | DIRECTORY( @<stage_name> )
//	      | SEMANTIC_VIEW( ... )
//	      | ERROR_TABLE( <base_table_name> )
//	      | DYNAMIC_TABLE_REFRESH_BOUNDARY( <object_name> )
//	           [ AT | BEFORE ( <object_state> ) ]
//	           [ CHANGES ( <change_tracking_type> ) ]
//	           [ [ AS ] <alias_name> ]
//	   }
func (v *Validator) ParseFrom() bool {
	// FROM <objectReference> [ , <objectReference> | JOIN ... ]* — model the FROM
	// keyword followed by at least one object reference, then a permissive tail
	// for joins / additional refs / clauses.
	objectRef := func() bool {
		return v.Choice(
			// LATERAL ( subquery ) | ( VALUES ... ) | ( subquery )
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("LATERAL") }) },
					v.consumeBalancedParens,
				)
			},
			// @stage[/path] [ ( ... ) ]
			func() bool {
				return v.Sequence(
					func() bool { return v.Match(sqltok.At) },
					v.parseIdentPath,
					func() bool { return v.Optional(v.consumeBalancedParens) },
				)
			},
			// table function / DIRECTORY(...) / SEMANTIC_VIEW(...) / <name>
			func() bool {
				return v.Sequence(
					v.parseIdentPath,
					func() bool { return v.Optional(v.consumeBalancedParens) },
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("FROM") },
		objectRef,
		v.consumeRest,
	)
}

// ParseAtBefore validates the Snowflake `AT_BEFORE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/at-before
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	  { AT | BEFORE }
//	  (
//	    { TIMESTAMP => <timestamp> |
//	      OFFSET => <time_difference> |
//	      STATEMENT => <id> |
//	      STREAM => '<name>' }
//	  )
//	[ ... ]
func (v *Validator) ParseAtBefore() bool {
	// { AT | BEFORE } ( { TIMESTAMP => .. | OFFSET => .. | STATEMENT => .. | STREAM => '..' } )
	return v.Sequence(
		v.wordsValue("AT", "BEFORE"),
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseChanges validates the Snowflake `CHANGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/changes
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	   CHANGES ( INFORMATION => { DEFAULT | APPEND_ONLY } )
//	   AT ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> | STREAM => '<name>' } ) | BEFORE ( STATEMENT => <id> )
//	   [ END( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	[ ... ]
func (v *Validator) ParseChanges() bool {
	// CHANGES ( INFORMATION => { DEFAULT | APPEND_ONLY } )
	//   { AT ( ... ) | BEFORE ( ... ) }
	//   [ END ( ... ) ]
	return v.Sequence(
		func() bool { return v.MatchWord("CHANGES") },
		v.consumeBalancedParens, // ( INFORMATION => ... )
		v.wordsValue("AT", "BEFORE"),
		v.consumeBalancedParens, // ( STATEMENT => ... etc )
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("END") },
					v.consumeBalancedParens,
				)
			})
		},
		v.consumeRest,
	)
}

// ParseConnectBy validates the Snowflake `CONNECT_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/connect-by
//
// Syntax:
//
//	SELECT <column_list> [ , <level_expression> ]
//	  FROM <data_source>
//	    START WITH <predicate>
//	    CONNECT BY [ PRIOR ] <col1_identifier> = [ PRIOR ] <col2_identifier>
//	           [ , [ PRIOR ] <col3_identifier> = [ PRIOR ] <col4_identifier> ]
//	           ...
//	  ...
func (v *Validator) ParseConnectBy() bool {
	// SELECT ... FROM <src> START WITH <pred> CONNECT BY [PRIOR] <c> = [PRIOR] <c> ...
	// Consume tokens up to START WITH, then model the START WITH / CONNECT BY skeleton.
	upToStart := func() bool {
		if v.AtEnd() {
			return false
		}
		t := v.Peek()
		if t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), "START") {
			return false
		}
		if t.Kind == sqltok.LParen {
			return v.consumeBalancedParens()
		}
		v.advance()
		return true
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("SELECT") },
		func() bool { return v.ZeroOrMore(upToStart) },
		func() bool { return v.phrase("START", "WITH") },
		v.consumeRest, // <predicate> CONNECT BY ...
	)
}

// ParseJoin validates the Snowflake `JOIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/join
//
// Syntax:
//
//	SELECT ...
//	FROM <object_ref1> [
//	                     {
//	                       INNER
//	                       | { LEFT | RIGHT | FULL } [ OUTER ]
//	                     }
//	                     [ DIRECTED ]
//	                   ]
//	                   JOIN <object_ref2>
//	  [ ON <condition> ]
//	[ ... ]
//
//	SELECT *
//	FROM <object_ref1> [
//	                     {
//	                       INNER
//	                       | { LEFT | RIGHT | FULL } [ OUTER ]
//	                     }
//	                     [ DIRECTED ]
//	                   ]
//	                   JOIN <object_ref2>
//	  [ USING( <column_list> ) ]
//	[ ... ]
//
//	SELECT ...
//	FROM <object_ref1> [
//	                     {
//	                       NATURAL [
//	                                 {
//	                                   INNER
//	                                   | { LEFT | RIGHT | FULL } [ OUTER ]
//	                                 }
//	                                 [ DIRECTED ]
//	                               }
//	                       | CROSS  [ DIRECTED ]
//	                     }
//	                   ]
//	                   JOIN <object_ref2>
//	[ ... ]
func (v *Validator) ParseJoin() bool {
	// FROM <ref1> [ <join-type> ] JOIN <ref2> [ ON <cond> | USING ( <cols> ) ]
	ref := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool { return v.Optional(v.consumeBalancedParens) },
			func() bool { return v.Optional(func() bool { return v.MatchWord("AS") }) },
			func() bool {
				return v.Optional(func() bool {
					// optional alias, but not the join keywords
					t := v.Peek()
					if t.Kind.IsIdentLike() {
						w := t.Text(v.src)
						for _, kw := range []string{"INNER", "LEFT", "RIGHT", "FULL", "NATURAL", "CROSS", "JOIN", "ON", "USING"} {
							if strings.EqualFold(w, kw) {
								return false
							}
						}
					}
					return v.parseIdentPath()
				})
			},
		)
	}
	joinType := func() bool {
		return v.Optional(func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("INNER") },
				func() bool {
					return v.Sequence(
						v.wordsValue("LEFT", "RIGHT", "FULL"),
						func() bool { return v.Optional(func() bool { return v.MatchWord("OUTER") }) },
					)
				},
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("NATURAL") },
						func() bool {
							return v.Optional(func() bool {
								return v.Choice(
									func() bool { return v.MatchWord("INNER") },
									func() bool {
										return v.Sequence(
											v.wordsValue("LEFT", "RIGHT", "FULL"),
											func() bool { return v.Optional(func() bool { return v.MatchWord("OUTER") }) },
										)
									},
								)
							})
						},
					)
				},
				func() bool { return v.MatchWord("CROSS") },
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("FROM") },
		ref,
		joinType,
		func() bool { return v.Optional(func() bool { return v.MatchWord("DIRECTED") }) },
		func() bool { return v.MatchWord("JOIN") },
		ref,
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("ON") }, v.consumeRest)
					},
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("USING") }, v.consumeBalancedParens)
					},
				)
			})
		},
		v.consumeRest,
	)
}

// ParseAsofJoin validates the Snowflake `ASOF_JOIN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/asof-join
//
// Syntax:
//
//	FROM <left_table> ASOF JOIN <right_table>
//	  MATCH_CONDITION ( <left_table.timecol> <comparison_operator> <right_table.timecol> )
//	  [ ON <table.col> = <table.col> [ AND ... ] | USING ( <column_list> ) ]
func (v *Validator) ParseAsofJoin() bool {
	// FROM <left> ASOF JOIN <right> MATCH_CONDITION ( ... ) [ ON ... | USING (...) ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("FROM") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("ASOF") },
		func() bool { return v.MatchWord("JOIN") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("MATCH_CONDITION") },
		v.consumeBalancedParens,
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("ON") }, v.consumeRest)
					},
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("USING") }, v.consumeBalancedParens)
					},
				)
			})
		},
		v.consumeRest,
	)
}

// ParseLateral validates the Snowflake `LATERAL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/join-lateral
//
// Syntax:
//
//	SELECT ...
//	  FROM <left_hand_table_expression>, LATERAL ( <inline_view> )
//	...
func (v *Validator) ParseLateral() bool {
	// FROM <left_expr>, LATERAL ( <inline_view> ) — model FROM <ref> , LATERAL ( ... ).
	return v.Sequence(
		func() bool { return v.MatchKeyword("FROM") },
		v.parseIdentPath,
		func() bool { return v.Optional(v.consumeBalancedParens) },
		func() bool { return v.Optional(func() bool { return v.MatchWord("AS") }) },
		func() bool {
			return v.Optional(func() bool {
				if v.Peek().Kind == sqltok.Comma {
					return false
				}
				return v.parseIdentPath()
			})
		},
		func() bool { return v.Match(sqltok.Comma) },
		func() bool { return v.MatchWord("LATERAL") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseMatchRecognize validates the Snowflake `MATCH_RECOGNIZE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/match_recognize
//
// Syntax:
//
//	MATCH_RECOGNIZE (
//	    [ PARTITION BY <expr> [, ... ] ]
//	    [ ORDER BY <expr> [, ... ] ]
//	    [ MEASURES <expr> [AS] <alias> [, ... ] ]
//	    [ ONE ROW PER MATCH |
//	      ALL ROWS PER MATCH [ { SHOW EMPTY MATCHES | OMIT EMPTY MATCHES | WITH UNMATCHED ROWS } ]
//	      ]
//	    [ AFTER MATCH SKIP
//	          {
//	          PAST LAST ROW   |
//	          TO NEXT ROW   |
//	          TO [ { FIRST | LAST} ] <symbol>
//	          }
//	      ]
//	    PATTERN ( <pattern> )
//	    DEFINE <symbol> AS <expr> [, ... ]
//	)
func (v *Validator) ParseMatchRecognize() bool {
	// MATCH_RECOGNIZE ( ... ) — the body is too detailed to model token-for-token.
	return v.Sequence(
		func() bool { return v.MatchWord("MATCH_RECOGNIZE") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParsePivot validates the Snowflake `PIVOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/pivot
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	   PIVOT ( <aggregate_function> ( <pivot_column> ) [ [ AS ] <alias> ]
//	            FOR <value_column> IN (
//	              <pivot_value_1> [ [ AS ] <alias> ] [ , <pivot_value_2> [ [ AS ] <alias> ] ... ]
//	              | ANY [ ORDER BY ... ]
//	              | <subquery>
//	            )
//	            [ DEFAULT ON NULL (<value>) ]
//	         )
//
//	[ ... ]
func (v *Validator) ParsePivot() bool {
	// PIVOT ( <agg>(<col>) FOR <col> IN ( ... ) [ DEFAULT ON NULL (<v>) ] )
	return v.Sequence(
		func() bool { return v.MatchWord("PIVOT") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseUnpivot validates the Snowflake `UNPIVOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/unpivot
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	    UNPIVOT [ { INCLUDE | EXCLUDE } NULLS ]
//	      ( <value_column>
//	        FOR <name_column> IN (
//	          <col> [ [ AS ] <col_alias> ],
//	          ...
//	        )
//	      )
//
//	[ ... ]
func (v *Validator) ParseUnpivot() bool {
	// UNPIVOT [ { INCLUDE | EXCLUDE } NULLS ] ( <value_col> FOR <name_col> IN ( ... ) )
	return v.Sequence(
		func() bool { return v.MatchWord("UNPIVOT") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					v.wordsValue("INCLUDE", "EXCLUDE"),
					func() bool { return v.MatchWord("NULLS") },
				)
			})
		},
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseValues validates the Snowflake `VALUES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/values
//
// Syntax:
//
//	SELECT ...
//	FROM ( VALUES ( <expr> [ , <expr> [ , ... ] ] ) [ , ( ... ) ] )
//	  [ [ AS ] <table_alias> [ ( <column_alias> [ , ... ] ) ] ]
//	[ ... ]
func (v *Validator) ParseValues() bool {
	// FROM ( VALUES ( <expr> [, ...] ) [ , ( ... ) ]* ) [ [AS] alias [(cols)] ]
	return v.Sequence(
		func() bool { return v.MatchKeyword("FROM") },
		func() bool { return v.Match(sqltok.LParen) },
		func() bool { return v.MatchWord("VALUES") },
		v.consumeBalancedParens, // ( <expr> [, ...] )
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, v.consumeBalancedParens)
			})
		},
		func() bool { return v.Match(sqltok.RParen) },
		v.consumeRest, // optional alias / column list
	)
}

// ParseSample validates the Snowflake `SAMPLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/sample
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	  { SAMPLE | TABLESAMPLE } [ samplingMethod ]
//	[ ... ]
//
//	samplingMethod ::= { { BERNOULLI | ROW } ( { <probability> | <num> ROWS } ) |
//	                     { SYSTEM | BLOCK } ( <probability> ) [ { REPEATABLE | SEED } ( <seed> ) ] }
func (v *Validator) ParseSample() bool {
	// { SAMPLE | TABLESAMPLE } [ { BERNOULLI | ROW | SYSTEM | BLOCK } ] ( ... )
	//   [ { REPEATABLE | SEED } ( <seed> ) ]
	return v.Sequence(
		v.wordsValue("SAMPLE", "TABLESAMPLE"),
		func() bool {
			return v.Optional(v.wordsValue("BERNOULLI", "ROW", "SYSTEM", "BLOCK"))
		},
		v.consumeBalancedParens, // ( <probability> | <num> ROWS )
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					v.wordsValue("REPEATABLE", "SEED"),
					v.consumeBalancedParens,
				)
			})
		},
		v.consumeRest,
	)
}

// ParseResample validates the Snowflake `RESAMPLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/resample
//
// Syntax:
//
//	FROM <object_reference> [ [ AS ] <alias_name> ]
//	  RESAMPLE(
//	    USING <time_series_column>
//	    INCREMENT BY <time_series_constant>
//	    [ PARTITION BY <partition_column> [ , ... ] ]
//	    [ METADATA_COLUMNS
//	        { IS_GENERATED() | BUCKET_START() } [ [ AS ] <alias_name> ] [ , ... ] ]
//	    )
func (v *Validator) ParseResample() bool {
	// FROM <ref> [ [AS] alias ] RESAMPLE ( USING ... INCREMENT BY ... [ ... ] )
	return v.Sequence(
		func() bool { return v.MatchKeyword("FROM") },
		v.parseIdentPath,
		func() bool { return v.Optional(func() bool { return v.MatchWord("AS") }) },
		func() bool {
			return v.Optional(func() bool {
				if v.Peek().Kind.IsIdentLike() && strings.EqualFold(v.Peek().Text(v.src), "RESAMPLE") {
					return false
				}
				return v.parseIdentPath()
			})
		},
		func() bool { return v.MatchWord("RESAMPLE") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseSemanticView validates the Snowflake `SEMANTIC_VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/semantic_view
//
// Syntax:
//
//	SEMANTIC_VIEW(
//	  [<namespace>.]<semantic_view_name>
//	  [
//	    {
//	      METRICS <metric_expr> [ [ AS ] <alias> ] [ , ... ] |
//	      FACTS <fact_expr>  [ , ... ]
//	    }
//	  ]
//	  [ DIMENSIONS <dimension_expr>  [ [ AS ] <alias> ] [ , ... ] ]
//	  [ WHERE <predicate> ]
//	)
func (v *Validator) ParseSemanticView() bool {
	// SEMANTIC_VIEW ( <name> [ METRICS .. | FACTS .. ] [ DIMENSIONS .. ] [ WHERE .. ] )
	return v.Sequence(
		func() bool { return v.MatchWord("SEMANTIC_VIEW") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseWhere validates the Snowflake `WHERE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/where
//
// Syntax:
//
//	...
//	WHERE <predicate>
//	[ ... ]
func (v *Validator) ParseWhere() bool {
	// WHERE <predicate>
	return v.Sequence(
		func() bool { return v.MatchKeyword("WHERE") },
		func() bool { return !v.AtEnd() }, // require at least one predicate token
		v.consumeRest,
	)
}

// ParseGroupBy validates the Snowflake `GROUP_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by
//
// Syntax:
//
//	SELECT ...
//	  FROM ...
//	  [ ... ]
//	  GROUP BY groupItem [ , groupItem [ , ... ] ]
//	  [ ... ]
//
//	SELECT ...
//	  FROM ...
//	  [ ... ]
//	  GROUP BY ALL
//	  [ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
func (v *Validator) ParseGroupBy() bool {
	// GROUP BY { ALL | groupItem [ , groupItem ]* }
	return v.Sequence(
		func() bool { return v.phrase("GROUP", "BY") },
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("ALL") },
				func() bool { return !v.AtEnd() }, // at least one group item token
			)
		},
		v.consumeRest,
	)
}

// ParseGroupByCube validates the Snowflake `GROUP_BY_CUBE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-cube
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ... ]
//	GROUP BY [ groupItem [ , groupItem [ , ... ] ] , ] CUBE ( groupItem [ , groupItem [ , ... ] ] )
//	[ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
func (v *Validator) ParseGroupByCube() bool {
	// GROUP BY [ groupItem [, ...] , ] CUBE ( groupItem [, ...] )
	upTo := func() bool {
		if v.AtEnd() {
			return false
		}
		t := v.Peek()
		if t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), "CUBE") {
			return false
		}
		if t.Kind == sqltok.LParen {
			return v.consumeBalancedParens()
		}
		v.advance()
		return true
	}
	return v.Sequence(
		func() bool { return v.phrase("GROUP", "BY") },
		func() bool { return v.ZeroOrMore(upTo) },
		func() bool { return v.MatchWord("CUBE") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseGroupByGroupingSets validates the Snowflake `GROUP_BY_GROUPING_SETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-grouping-sets
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ... ]
//	GROUP BY [ groupItem [ , groupItem [ , ... ] ] , ] GROUPING SETS ( groupSet [ , groupSet [ , ... ] ] )
//	[ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
//
//	groupSet ::= groupItem | ( groupItem [ , groupItem [ , ... ] ] )
func (v *Validator) ParseGroupByGroupingSets() bool {
	// GROUP BY [ groupItem [, ...] , ] GROUPING SETS ( groupSet [, ...] )
	upTo := func() bool {
		if v.AtEnd() {
			return false
		}
		t := v.Peek()
		if t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), "GROUPING") {
			return false
		}
		if t.Kind == sqltok.LParen {
			return v.consumeBalancedParens()
		}
		v.advance()
		return true
	}
	return v.Sequence(
		func() bool { return v.phrase("GROUP", "BY") },
		func() bool { return v.ZeroOrMore(upTo) },
		func() bool { return v.phrase("GROUPING", "SETS") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseGroupByRollup validates the Snowflake `GROUP_BY_ROLLUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/group-by-rollup
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ... ]
//	GROUP BY [ groupItem [ , groupItem [ , ... ] ] , ] ROLLUP ( groupItem [ , groupItem [ , ... ] ] )
//	[ ... ]
//
//	groupItem ::= { <column_alias> | <position> | <expr> }
func (v *Validator) ParseGroupByRollup() bool {
	// GROUP BY [ groupItem [, ...] , ] ROLLUP ( groupItem [, ...] )
	upTo := func() bool {
		if v.AtEnd() {
			return false
		}
		t := v.Peek()
		if t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), "ROLLUP") {
			return false
		}
		if t.Kind == sqltok.LParen {
			return v.consumeBalancedParens()
		}
		v.advance()
		return true
	}
	return v.Sequence(
		func() bool { return v.phrase("GROUP", "BY") },
		func() bool { return v.ZeroOrMore(upTo) },
		func() bool { return v.MatchWord("ROLLUP") },
		v.consumeBalancedParens,
		v.consumeRest,
	)
}

// ParseHaving validates the Snowflake `HAVING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/having
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	GROUP BY ...
//	HAVING <predicate>
//	[ ... ]
func (v *Validator) ParseHaving() bool {
	// HAVING <predicate>
	return v.Sequence(
		func() bool { return v.MatchKeyword("HAVING") },
		func() bool { return !v.AtEnd() },
		v.consumeRest,
	)
}

// ParseQualify validates the Snowflake `QUALIFY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/qualify
//
// Syntax:
//
//	QUALIFY <predicate>
//
//	SELECT <column_list>
//	  FROM <data_source>
//	  [GROUP BY ...]
//	  [HAVING ...]
//	  QUALIFY <predicate>
//	  [ ... ]
func (v *Validator) ParseQualify() bool {
	// QUALIFY <predicate>
	return v.Sequence(
		func() bool { return v.MatchKeyword("QUALIFY") },
		func() bool { return !v.AtEnd() },
		v.consumeRest,
	)
}

// ParseOrderBy validates the Snowflake `ORDER_BY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/order-by
//
// Syntax:
//
//	SELECT ...
//	  FROM ...
//	  ORDER BY orderItem [ , orderItem , ... ]
//	  [ ... ]
//
//	orderItem ::= { <column_alias> | <position> | <expr> } [ { ASC | DESC } ] [ NULLS { FIRST | LAST } ]
//
//	SELECT ...
//	  FROM ...
//	  ORDER BY ALL [ { ASC | DESC } ] [ NULLS { FIRST | LAST } ]
//	  [ ... ]
func (v *Validator) ParseOrderBy() bool {
	// ORDER BY { ALL | orderItem [ , orderItem ]* }
	//   orderItem ::= <expr> [ ASC | DESC ] [ NULLS { FIRST | LAST } ]
	return v.Sequence(
		func() bool { return v.phrase("ORDER", "BY") },
		func() bool { return !v.AtEnd() }, // ALL or first order item
		v.consumeRest,
	)
}

// ParseLimitFetch validates the Snowflake `LIMIT_FETCH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/limit
//
// Syntax:
//
//	SELECT ...
//	FROM ...
//	[ ORDER BY ... ]
//	LIMIT <count> [ OFFSET <start> ]
//	[ ... ]
//
//	SELECT ...
//	FROM ...
//	[ ORDER BY ... ]
//	[ OFFSET <start> ] [ { ROW | ROWS } ] FETCH [ { FIRST | NEXT } ] <count> [ { ROW | ROWS } ] [ ONLY ]
//	[ ... ]
func (v *Validator) ParseLimitFetch() bool {
	// Form 1: LIMIT <count> [ OFFSET <start> ]
	// Form 2: [ OFFSET <start> ] [ ROW | ROWS ] FETCH [ FIRST | NEXT ] <count>
	//         [ ROW | ROWS ] [ ONLY ]
	rowOrRows := func() bool { return v.Optional(v.wordsValue("ROW", "ROWS")) }
	limitForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("LIMIT") },
			func() bool { return v.Match(sqltok.NumberLit) },
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("OFFSET") },
						func() bool { return v.Match(sqltok.NumberLit) },
					)
				})
			},
		)
	}
	fetchForm := func() bool {
		return v.Sequence(
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("OFFSET") },
						func() bool { return v.Match(sqltok.NumberLit) },
						rowOrRows,
					)
				})
			},
			func() bool { return v.MatchWord("FETCH") },
			func() bool { return v.Optional(v.wordsValue("FIRST", "NEXT")) },
			func() bool { return v.Match(sqltok.NumberLit) },
			rowOrRows,
			func() bool { return v.Optional(func() bool { return v.MatchWord("ONLY") }) },
		)
	}
	return v.Sequence(
		func() bool { return v.Choice(limitForm, fetchForm) },
		v.consumeRest,
	)
}

// ParseForUpdate validates the Snowflake `FOR_UPDATE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/constructs/for-update
//
// Syntax:
//
//	SELECT ...
//	  FROM ...
//	  [ ... ]
//	  FOR UPDATE [ NOWAIT | WAIT <wait_time> ]
func (v *Validator) ParseForUpdate() bool {
	// FOR UPDATE [ NOWAIT | WAIT <wait_time> ]
	return v.Sequence(
		func() bool { return v.phrase("FOR", "UPDATE") },
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool { return v.MatchWord("NOWAIT") },
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("WAIT") },
							func() bool { return v.Match(sqltok.NumberLit) },
						)
					},
				)
			})
		},
	)
}
